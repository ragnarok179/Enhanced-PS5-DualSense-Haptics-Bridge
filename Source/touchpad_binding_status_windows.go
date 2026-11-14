//go:build windows && (usb || bluetooth)

package main

import (
	"fmt"
	"net"
	"sync/atomic"
	"time"
)

const touchpadBindingStatusAddress = "127.0.0.1:6978"

var touchpadBindingState struct {
	bound               atomic.Bool
	capture             atomic.Bool
	mouseForced         atomic.Bool
	mousePolicy         atomic.Int32 // 0=auto, 1=on, 2=off
	suppressTwoTapMouse atomic.Bool
	lastSeen            atomic.Int64
}

func touchpadMouseAllowedForPolicy(bound, capture bool, policy int32, statusFresh bool) bool {
	// In the absence of a current BeamNG status, never inject unexpected
	// cursor movement or clicks. Explicit OFF must be authoritative.
	if !statusFresh || capture {
		return false
	}
	if policy == 1 {
		return true
	} // explicit Windows mouse overrides saved bindings
	return policy == 0 && !bound
}

// Compatibility helper for the legacy DSM2 v2 boolean 'mouseForced'.
func touchpadMouseAllowedForState(bound, capture, mouseForced, statusFresh bool) bool {
	policy := int32(0)
	if mouseForced {
		policy = 1
	}
	return touchpadMouseAllowedForPolicy(bound, capture, policy, statusFresh)
}

func touchpadMouseAllowed() bool {
	// BeamNG refreshes ownership every 500 ms. Never inject mouse input
	// after the status becomes stale, even if it was explicitly enabled.
	last := touchpadBindingState.lastSeen.Load()
	fresh := last != 0 && time.Since(time.Unix(0, last)) <= 2*time.Second
	return touchpadMouseAllowedForPolicy(
		touchpadBindingState.bound.Load(),
		touchpadBindingState.capture.Load(),
		touchpadBindingState.mousePolicy.Load(),
		fresh,
	)
}

func updateTouchpadBindingState(bound, capture, mouseForced, suppressTwoTapMouse bool) {
	policy := int32(0)
	if mouseForced {
		policy = 1
	}
	updateTouchpadBindingPolicy(bound, capture, policy, suppressTwoTapMouse)
}

func updateTouchpadBindingPolicy(bound, capture bool, policy int32, suppressTwoTapMouse bool) {
	if policy < 0 || policy > 2 {
		policy = 2
	}
	oldPolicy := touchpadBindingState.mousePolicy.Swap(policy)
	mouseForced := policy == 1
	oldBound := touchpadBindingState.bound.Swap(bound)
	oldCapture := touchpadBindingState.capture.Swap(capture)
	oldMouseForced := touchpadBindingState.mouseForced.Swap(mouseForced)
	oldSuppress := touchpadBindingState.suppressTwoTapMouse.Swap(suppressTwoTapMouse)
	touchpadBindingState.lastSeen.Store(time.Now().UnixNano())
	// This is high-frequency internal state, not a public connection status.
	// Keep detailed ownership transitions in explicit diagnostics only; the
	// normal V1.43-style console stays focused on controller and BeamNG status.
	if !touchpadBindingDiagnosticsEnabled() ||
		(oldBound == bound && oldCapture == capture && oldMouseForced == mouseForced && oldSuppress == suppressTwoTapMouse && oldPolicy == policy) {
		return
	}
	owner := "automatic mouse fallback"
	if capture {
		owner = "BeamNG binding capture"
	} else if policy == 1 {
		owner = "Windows mouse ON"
	} else if bound {
		owner = "BeamNG touchpad binding"
	} else if policy == 2 {
		owner = "Windows mouse OFF"
	}
	// Log only actual ownership transitions (never the 500-ms heartbeat).
	// This makes broken UI/hotkey/binding handoffs diagnosable from the
	// regular Bridge log without requiring a separate debug build.
	fmt.Printf("Touchpad ownership: %s (bound=%t capture=%t policy=%d 2Freserved=%t)\n", owner, bound, capture, policy, suppressTwoTapMouse)
}

func touchpadMouseTwoFingerTapAllowed() bool {
	// Reservation is different from ownership. If BeamNG status briefly goes
	// stale, keep the last explicit 2F reservation instead of re-enabling a
	// Windows right-click behind the user's back. A new DSM2 status or Bridge
	// restart clears it normally.
	return !touchpadBindingState.suppressTwoTapMouse.Load()
}

func startTouchpadBindingStatusReceiver(done <-chan struct{}) {
	addr, err := net.ResolveUDPAddr("udp", touchpadBindingStatusAddress)
	if err != nil {
		return
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		if touchpadBindingDiagnosticsEnabled() {
			fmt.Printf("Touchpad diagnostic receiver error: %v\n", err)
		}
		return
	}
	go func() {
		defer conn.Close()
		buf := make([]byte, 4096)
		_ = conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
		for {
			select {
			case <-done:
				return
			default:
			}
			_ = conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
			n, _, err := conn.ReadFromUDP(buf)
			if err != nil {
				continue
			}
			if n == 7 && string(buf[:4]) == "DMB1" && buf[4] == 1 && buf[5] <= 1 && buf[6] <= 1 {
				updateMicMuteLED(buf[5] == 1, buf[6] == 1)
			} else if n >= 9 && string(buf[:4]) == "DSM2" && buf[4] == 3 {
				updateTouchpadBindingPolicy(buf[5] != 0, buf[6] != 0, int32(buf[7]), buf[8] != 0)
			} else if n >= 9 && string(buf[:4]) == "DSM2" && buf[4] == 2 {
				updateTouchpadBindingState(buf[5] != 0, buf[6] != 0, buf[7] != 0, buf[8] != 0)
			} else if n >= 7 && string(buf[:4]) == "DSM2" && buf[4] == 1 {
				updateTouchpadBindingState(buf[5] != 0, buf[6] != 0, false, false)
			} else if n >= 5 && string(buf[:5]) == "DSE1\t" {
				_ = dispatchBeamNGSoundEventPacket(buf[:n], time.Now())
			} else if n >= 10 && string(buf[:4]) == "DSS1" && buf[4] == 2 {
				flags := uint16(buf[7]) | uint16(buf[8])<<8
				applyBeamNGSpeakerSettings(buf[5] != 0, int(buf[6]), flags, buf[9])
			} else if n >= 9 && string(buf[:4]) == "DSS1" && buf[4] == 1 {
				applyBeamNGSpeakerSettings(buf[5] != 0, int(buf[6]), uint16(buf[7]), buf[8])
			} else if n >= 10 && string(buf[:4]) == "DSC2" && buf[4] == 1 {
				applyBluetoothIdleConfig(int(buf[5])|int(buf[6])<<8, buf[7]&1 != 0, uint16(buf[8])|uint16(buf[9])<<8, time.Now())
			} else if n >= 7 && string(buf[:4]) == "DSA1" && buf[4] == 1 {
				_ = noteControllerMotionActivity(uint16(buf[5])|uint16(buf[6])<<8, time.Now())
			} else if n >= 7 && string(buf[:4]) == "DSC1" && buf[4] == 1 {
				setBluetoothIdleTimeoutMinutes(int(buf[5]) | int(buf[6])<<8)
			} else if n >= 6 && string(buf[:4]) == "DSM1" && buf[4] == 1 {
				updateTouchpadBindingState(buf[5] != 0, false, false, false)
			} else if n >= 5 && string(buf[:5]) == "DSMM1" && touchpadBindingDiagnosticsEnabled() {
				// Diagnostic-only Motion messages share this already-existing UDP
				// receiver. They never alter Bridge runtime/input policy.
				fmt.Printf("BeamNG motion debug: %s\n", string(buf[:n]))
			} else if n >= 5 && string(buf[:4]) == "DSMD" && touchpadBindingDiagnosticsEnabled() {
				fmt.Printf("BeamNG touchpad debug: %s\n", string(buf[:n]))
			}
		}
	}()
}
