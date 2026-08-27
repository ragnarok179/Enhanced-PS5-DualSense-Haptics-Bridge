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
	suppressTwoTapMouse atomic.Bool
	lastSeen            atomic.Int64
}

func touchpadMouseAllowedForState(bound, capture, mouseForced, statusFresh bool) bool {
	if !statusFresh {
		// Fail safe: if BeamNG disappears, do not strand the touchpad in a stale
		// virtual-input ownership state.
		return true
	}
	if capture {
		return false
	}
	if mouseForced {
		return true
	}
	return !bound
}

func touchpadMouseAllowed() bool {
	// BeamNG refreshes ownership every 500 ms. A fresh status owns the decision;
	// stale/missing status falls back to mouse so the pad never becomes dead.
	last := touchpadBindingState.lastSeen.Load()
	fresh := last != 0 && time.Since(time.Unix(0, last)) <= 2*time.Second
	return touchpadMouseAllowedForState(
		touchpadBindingState.bound.Load(),
		touchpadBindingState.capture.Load(),
		touchpadBindingState.mouseForced.Load(),
		fresh,
	)
}

func updateTouchpadBindingState(bound, capture, mouseForced, suppressTwoTapMouse bool) {
	oldBound := touchpadBindingState.bound.Swap(bound)
	oldCapture := touchpadBindingState.capture.Swap(capture)
	oldMouseForced := touchpadBindingState.mouseForced.Swap(mouseForced)
	oldSuppress := touchpadBindingState.suppressTwoTapMouse.Swap(suppressTwoTapMouse)
	touchpadBindingState.lastSeen.Store(time.Now().UnixNano())
	if oldBound == bound && oldCapture == capture && oldMouseForced == mouseForced && oldSuppress == suppressTwoTapMouse {
		return
	}
	owner := "automatic mouse fallback"
	if capture {
		owner = "BeamNG binding capture"
	} else if mouseForced {
		owner = "forced Windows mouse mode"
	} else if bound {
		owner = "BeamNG touchpad binding"
	}
	if touchpadBindingDiagnosticsEnabled() {
		fmt.Printf("Touchpad ownership: %s (bound=%t capture=%t mouse=%t 2Freserved=%t)\n", owner, bound, capture, mouseForced, suppressTwoTapMouse)
	}
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
			if n >= 9 && string(buf[:4]) == "DSM2" && buf[4] == 2 {
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
