//go:build windows && (usb || bluetooth)

package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func runTouchpadMouseDiagnostic(d *device) int {
	if d == nil {
		return 2
	}

	fmt.Println("DualSense Touchpad Windows Mouse diagnostic")
	fmt.Printf("Controller: %s | PID=0x%04X | input=%d output=%d\n", d.product, d.productID, d.inputLen, d.outputLen)
	fmt.Println("BeamNG is not required. This test injects real Windows mouse events with SendInput.")
	fmt.Println("Test: 1 finger = cursor, 1-finger tap = left click, 2 fingers = vertical/horizontal scroll, 2-finger tap = right click.")
	fmt.Println("Move/tap/scroll should affect Windows immediately. Only SendInput failures are printed.")
	fmt.Println("Press Ctrl+C to stop.")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(stop)

	var tracker touchGestureTracker
	var mouse touchpadMouseController
	identified := false
	for {
		select {
		case <-stop:
			mouse.cancel()
			return 0
		default:
		}

		r, err := d.readReportOnce()
		if err != nil {
			fmt.Println("Read error:", err)
			return 3
		}
		decoded, err := decodeDualSenseExtendedInput(r)
		if err != nil {
			continue
		}
		if !identified {
			fmt.Printf("DualSense input detected: report=0x%02X len=%d transport=%s\n", decoded.ReportID, decoded.ReportSize, decoded.Transport)
			identified = true
		}
		now := time.Now()
		frame := tracker.Update(decoded.Touch, now)
		mouse.Update(frame, now)
	}
}
