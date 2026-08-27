//go:build windows && (usb || bluetooth)

package main

import (
	"fmt"
	"math"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

var (
	modUser32             = syscall.NewLazyDLL("user32.dll")
	procSendInputTouchpad = modUser32.NewProc("SendInput")
)

const (
	inputMouse                 = 0
	mouseeventfMove            = 0x0001
	mouseeventfLeftDown        = 0x0002
	mouseeventfLeftUp          = 0x0004
	mouseeventfRightDown       = 0x0008
	mouseeventfRightUp         = 0x0010
	mouseeventfWheel           = 0x0800
	mouseeventfHWheel          = 0x1000
	touchMousePixelsPerSurface = 1450.0
	touchMouseTapDelay         = 180 * time.Millisecond
)

type mouseInputData struct {
	Dx        int32
	Dy        int32
	MouseData uint32
	Flags     uint32
	Time      uint32
	ExtraInfo uintptr
}

// Win32 INPUT on Windows amd64 is 40 bytes: 4-byte type, 4-byte alignment
// padding, then the 32-byte union (MOUSEINPUT here). SendInput rejects an
// incorrect cbSize.
type inputRecord struct {
	Type uint32
	_    uint32
	Mi   mouseInputData
}

var touchpadSendInputFailureLogged atomic.Bool

func sendMouse(flags uint32, dx, dy int32, data uint32) bool {
	in := inputRecord{Type: inputMouse, Mi: mouseInputData{Dx: dx, Dy: dy, MouseData: data, Flags: flags}}
	size := unsafe.Sizeof(in)
	inserted, _, callErr := procSendInputTouchpad.Call(1, uintptr(unsafe.Pointer(&in)), size)
	if inserted != 1 {
		if touchpadSendInputFailureLogged.CompareAndSwap(false, true) {
			fmt.Printf("Touchpad mouse: SendInput FAILED (INPUT=%d bytes, err=%v). If BeamNG is running as administrator, run the Bridge at the same integrity level.\n", size, callErr)
		}
		return false
	}
	return true
}

func sendMouseClick(left bool) {
	if left {
		sendMouse(mouseeventfLeftDown, 0, 0, 0)
		sendMouse(mouseeventfLeftUp, 0, 0, 0)
	} else {
		sendMouse(mouseeventfRightDown, 0, 0, 0)
		sendMouse(mouseeventfRightUp, 0, 0, 0)
	}
}

type pendingMouseTap struct {
	due    time.Time
	left   bool
	active bool
}

func (m *touchpadMouseController) cancelReservedTwoFingerTap() {
	if m.pending.active && !m.pending.left && !touchpadMouseTwoFingerTapAllowed() {
		if touchpadBindingDiagnosticsEnabled() {
			fmt.Println("Touchpad gesture: pending RIGHT TAP cancelled (reserved for Mouse-mode toggle)")
		}
		m.pending = pendingMouseTap{}
	}
}

type touchpadMouseController struct {
	prevOneTap, prevTwoTap bool
	pending                pendingMouseTap

	moveAccumX, moveAccumY float64
	moveContactID          byte
	moveContactValid       bool
	moveArmed              bool
	lastMoveAt             time.Time
	moveVelocity           touchPointerVelocityFilter

	contactActive           bool
	contactStartedAt        time.Time
	contactStartX           int
	contactStartY           int
	contactStartValid       bool
	contactMaxTravel        float64
	contactInjectedTravel   float64
	contactDeliberateMotion bool

	wheelAccum, hWheelAccum    float64
	scrollActive               bool
	scrollXArmed, scrollYArmed bool
	scrollStartX, scrollStartY float64
	lastScrollAt               time.Time
	scrollVelocity             touchPointerVelocityFilter
}

func (m *touchpadMouseController) beginContact(frame logicalTouchFrame, now time.Time) {
	m.contactActive = true
	m.contactStartedAt = now
	m.contactMaxTravel = 0
	m.contactInjectedTravel = 0
	m.contactDeliberateMotion = false
	m.contactStartValid = frame.PrimaryActive
	if frame.PrimaryActive {
		m.contactStartX, m.contactStartY = frame.PrimaryX, frame.PrimaryY
	}
	m.moveArmed = false
	m.moveVelocity.Reset()
	m.lastMoveAt = now
}

func (m *touchpadMouseController) endContact() {
	m.contactActive = false
	m.contactStartedAt = time.Time{}
	m.contactStartValid = false
	m.contactMaxTravel = 0
	m.contactInjectedTravel = 0
	m.contactDeliberateMotion = false
	m.moveArmed = false
	m.moveVelocity.Reset()
	m.lastMoveAt = time.Time{}
}

func (m *touchpadMouseController) cancel() {
	m.pending = pendingMouseTap{}
	m.moveAccumX, m.moveAccumY = 0, 0
	m.moveContactValid = false
	m.wheelAccum, m.hWheelAccum = 0, 0
	m.scrollActive = false
	m.scrollXArmed, m.scrollYArmed = false, false
	m.scrollStartX, m.scrollStartY = 0, 0
	m.lastScrollAt = time.Time{}
	m.scrollVelocity.Reset()
	m.endContact()
}

func (m *touchpadMouseController) updateContactTravel(frame logicalTouchFrame) {
	if !m.contactActive || !m.contactStartValid || !frame.PrimaryActive {
		return
	}
	travel := normalizedDistance(frame.PrimaryX, frame.PrimaryY, m.contactStartX, m.contactStartY)
	if travel > m.contactMaxTravel {
		m.contactMaxTravel = travel
	}
	if m.contactMaxTravel > touchMouseEarlyMoveSlop {
		m.contactDeliberateMotion = true
	}
}

func (m *touchpadMouseController) Update(frame logicalTouchFrame, now time.Time) {
	m.cancelReservedTwoFingerTap()
	allowed := touchpadMouseAllowed()
	if !allowed {
		m.cancel()
		m.prevOneTap = frame.OneTap
		m.prevTwoTap = frame.TwoTap
		return
	}

	if frame.Count > 0 && !m.contactActive {
		m.beginContact(frame, now)
	}
	m.updateContactTravel(frame)

	if frame.Count == 1 {
		if !m.moveContactValid || m.moveContactID != frame.PrimaryID {
			m.moveAccumX, m.moveAccumY = 0, 0
			m.moveContactID = frame.PrimaryID
			m.moveContactValid = true
			m.moveArmed = false
			m.moveVelocity.Reset()
			m.lastMoveAt = now
		}

		// Treat the beginning of a contact like a desktop touchpad's tap/jitter
		// hysteresis: keep small early motion out of Windows while tap intent is
		// still plausible. A clearly deliberate move escapes immediately; a slow
		// precision move arms after the short settle interval. Tap eligibility is
		// evaluated separately on release, so a few natural pixels no longer make
		// the click disappear.
		if !m.moveArmed {
			age := now.Sub(m.contactStartedAt)
			arm, deliberate := touchMouseShouldArmMotion(age, m.contactMaxTravel)
			if arm {
				m.moveArmed = true
				m.contactDeliberateMotion = deliberate
				m.moveAccumX, m.moveAccumY = 0, 0
			}
			m.lastMoveAt = now
		} else {
			dt := now.Sub(m.lastMoveAt).Seconds()
			m.lastMoveAt = now
			speed := m.moveVelocity.Update(frame.RawOneDX, frame.RawOneDY, dt)
			gain := touchPointerGainForSpeed(speed)
			dx := accumulateWholePixels(&m.moveAccumX, frame.RawOneDX*touchMousePixelsPerSurface*gain)
			dy := accumulateWholePixels(&m.moveAccumY, frame.RawOneDY*touchMousePixelsPerSurface*gain)
			if dx != 0 || dy != 0 {
				if sendMouse(mouseeventfMove, dx, dy, 0) {
					m.contactInjectedTravel += math.Hypot(float64(dx), float64(dy))
				}
			}
		}
	} else {
		m.moveAccumX, m.moveAccumY = 0, 0
		m.moveContactValid = false
		m.moveArmed = false
		m.moveVelocity.Reset()
		m.lastMoveAt = now
	}

	if frame.Count == 2 {
		// Two-finger scrolling follows modern touchpad semantics: require a small
		// per-axis movement threshold to enter scrolling, then preserve tiny motion
		// with high-resolution wheel deltas (< WHEEL_DELTA) and a speed-adaptive
		// gain. The established direction mapping is intentionally preserved.
		if !m.scrollActive {
			m.scrollActive = true
			m.scrollXArmed, m.scrollYArmed = false, false
			m.scrollStartX, m.scrollStartY = 0, 0
			m.wheelAccum, m.hWheelAccum = 0, 0
			m.scrollVelocity.Reset()
			m.lastScrollAt = now
		} else {
			dt := now.Sub(m.lastScrollAt).Seconds()
			m.lastScrollAt = now
			speed := m.scrollVelocity.Update(frame.RawTwoDX, frame.RawTwoDY, dt)
			gain := touchScrollGainForSpeed(speed)

			justArmedX, justArmedY := false, false
			if !m.scrollXArmed {
				// Use net displacement for the activation threshold so stationary
				// two-finger jitter cannot eventually accumulate into a scroll.
				m.scrollStartX += frame.RawTwoDX
				if math.Abs(m.scrollStartX) >= touchScrollStartSlop {
					m.scrollXArmed = true
					m.hWheelAccum = 0
					justArmedX = true
				}
			}
			if !m.scrollYArmed {
				m.scrollStartY += frame.RawTwoDY
				if math.Abs(m.scrollStartY) >= touchScrollStartSlop {
					m.scrollYArmed = true
					m.wheelAccum = 0
					justArmedY = true
				}
			}

			if m.scrollYArmed && !justArmedY {
				m.wheelAccum += frame.RawTwoDY * touchScrollWheelUnitsPerSurface * gain
				delta := int32(m.wheelAccum)
				if delta != 0 {
					sendMouse(mouseeventfWheel, 0, 0, uint32(delta))
					m.wheelAccum -= float64(delta)
				}
			}
			if m.scrollXArmed && !justArmedX {
				m.hWheelAccum += -frame.RawTwoDX * touchScrollWheelUnitsPerSurface * gain
				delta := int32(m.hWheelAccum)
				if delta != 0 {
					sendMouse(mouseeventfHWheel, 0, 0, uint32(delta))
					m.hWheelAccum -= float64(delta)
				}
			}
		}
	} else if m.scrollActive {
		m.scrollActive = false
		m.scrollXArmed, m.scrollYArmed = false, false
		m.scrollStartX, m.scrollStartY = 0, 0
		m.wheelAccum, m.hWheelAccum = 0, 0
		m.lastScrollAt = time.Time{}
		m.scrollVelocity.Reset()
	}

	if frame.OneTap && !m.prevOneTap && touchMouseTapEligible(m.contactMaxTravel, m.contactInjectedTravel, m.contactDeliberateMotion) {
		m.pending = pendingMouseTap{due: now.Add(touchMouseTapDelay), left: true, active: true}
		if touchpadBindingDiagnosticsEnabled() {
			fmt.Printf("Touchpad gesture: LEFT TAP accepted travel=%.4f injected=%.1fpx deliberate=%t\n", m.contactMaxTravel, m.contactInjectedTravel, m.contactDeliberateMotion)
		}
	} else if frame.OneTap && !m.prevOneTap && touchpadBindingDiagnosticsEnabled() {
		fmt.Printf("Touchpad gesture: LEFT TAP rejected travel=%.4f injected=%.1fpx deliberate=%t\n", m.contactMaxTravel, m.contactInjectedTravel, m.contactDeliberateMotion)
	}
	if frame.TwoTap && !m.prevTwoTap && touchpadMouseTwoFingerTapAllowed() && touchMouseTapEligible(m.contactMaxTravel, m.contactInjectedTravel, m.contactDeliberateMotion) {
		m.pending = pendingMouseTap{due: now.Add(touchMouseTapDelay), left: false, active: true}
		if touchpadBindingDiagnosticsEnabled() {
			fmt.Printf("Touchpad gesture: RIGHT TAP accepted travel=%.4f injected=%.1fpx deliberate=%t\n", m.contactMaxTravel, m.contactInjectedTravel, m.contactDeliberateMotion)
		}
	} else if frame.TwoTap && !m.prevTwoTap && touchpadBindingDiagnosticsEnabled() {
		reason := "gesture rejected"
		if !touchpadMouseTwoFingerTapAllowed() {
			reason = "reserved for Mouse-mode toggle"
		}
		fmt.Printf("Touchpad gesture: RIGHT TAP not injected (%s) travel=%.4f injected=%.1fpx deliberate=%t\n", reason, m.contactMaxTravel, m.contactInjectedTravel, m.contactDeliberateMotion)
	}
	m.prevOneTap, m.prevTwoTap = frame.OneTap, frame.TwoTap

	if frame.Count == 0 && m.contactActive {
		if touchpadBindingDiagnosticsEnabled() {
			fmt.Printf("Touchpad gesture: CONTACT END age=%dms travel=%.4f injected=%.1fpx armed=%t deliberate=%t tap1=%t tap2=%t\n",
				now.Sub(m.contactStartedAt).Milliseconds(), m.contactMaxTravel, m.contactInjectedTravel, m.moveArmed, m.contactDeliberateMotion, frame.OneTap, frame.TwoTap)
		}
		m.endContact()
	}

	if m.pending.active && !now.Before(m.pending.due) {
		if !m.pending.left && !touchpadMouseTwoFingerTapAllowed() {
			if touchpadBindingDiagnosticsEnabled() {
				fmt.Println("Touchpad gesture: RIGHT TAP suppressed at injection (reserved for Mouse-mode toggle)")
			}
		} else if touchpadMouseAllowed() {
			sendMouseClick(m.pending.left)
		}
		m.pending = pendingMouseTap{}
	}
}
