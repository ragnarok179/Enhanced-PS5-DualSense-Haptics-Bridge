//go:build windows && (usb || bluetooth)

package main

import (
	"fmt"
	"net"
	"runtime"
	"time"
	"unsafe"
)

const (
	extendedInputBeamNGAddress = "127.0.0.1:6977"
	motionInputBeamNGAddress   = "127.0.0.1:6979"
)

func dialLocalUDP(address string) *net.UDPConn {
	addr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return nil
	}
	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return nil
	}
	return conn
}

// startExtendedInputStream owns one dedicated read-only HID handle for every
// extra input: touch/buttons and IMU. It never opens a second sensor reader and
// never writes to the controller, so the validated haptics/trigger writer stays
// independent and race-free.
func startExtendedInputStream(d *device, done <-chan struct{}) {
	if d == nil || d.path == "" {
		return
	}

	readHandle := openPath(d.path, genericRead)
	if !validHandle(readHandle) {
		fmt.Println("Extended inputs: unable to open dedicated HID read handle; feature disabled.")
		return
	}

	touchConn := dialLocalUDP(extendedInputBeamNGAddress)
	if touchConn == nil {
		procClose.Call(uintptr(readHandle))
		fmt.Println("Extended inputs: unable to open BeamNG UDP output.")
		return
	}
	motionConn := dialLocalUDP(motionInputBeamNGAddress)

	calibration, calibrationErr := readDualSenseMotionCalibration(readHandle)
	if calibrationErr != nil {
		fmt.Println("DualSense motion calibration: unavailable; using Sony nominal scales.")
	}

	startTouchpadBindingStatusReceiver(done)

	go func() {
		defer touchConn.Close()
		if motionConn != nil {
			defer motionConn.Close()
		}
		defer procClose.Call(uintptr(readHandle))

		inputBuf := make([]byte, d.inputLen)
		touchPacketBuf := make([]byte, extendedInputPacketSize)
		motionPacketBuf := make([]byte, motionInputPacketSize)
		var tracker touchGestureTracker
		var idleInputTracker bluetoothIdleInputTracker
		var mouse touchpadMouseController
		var orientation dualSenseOrientation
		var lastSent extendedInputWireState
		haveLast := false
		var touchSeq, motionSeq byte
		lastTouchSend := time.Time{}
		var lastMotionSensorTS uint32
		const touchPeriod = time.Second / 120
		// The DualSense IMU arrives at ~250 Hz. A wall-clock 120 Hz gate checked
		// only on 4 ms HID reports can alias down to ~83 Hz. Gate on Sony's sensor
		// timestamp instead and emit every ~8 ms for a stable ~125 Hz stream.
		const heartbeat = 250 * time.Millisecond

		// Preserve every sub-sample between 120 Hz BeamNG packets. The old
		// implementation could retain only the last tiny HID delta.
		var accumOneX, accumOneY, accumTwoX, accumTwoY float64

		for {
			select {
			case <-done:
				return
			default:
			}

			var n uint32
			r, _, _ := procReadFile.Call(
				uintptr(readHandle), uintptr(unsafe.Pointer(&inputBuf[0])), uintptr(len(inputBuf)),
				uintptr(unsafe.Pointer(&n)), 0,
			)
			runtime.KeepAlive(inputBuf)
			if r == 0 || n == 0 {
				time.Sleep(2 * time.Millisecond)
				continue
			}

			decoded, err := decodeDualSenseExtendedInput(inputBuf[:n])
			if err != nil {
				continue
			}
			now := time.Now()
			noteControllerReportSeen(now)
			if reason := idleInputTracker.activity(inputBuf[:n]); reason != bluetoothIdleActivityNone {
				noteControllerInputActivityReason(now, reason)
			}
			frame := tracker.Update(decoded.Touch, now)

			// Default mouse fallback is derived directly from unamplified HID
			// deltas. BeamNG binding/capture status can cancel it immediately.
			mouse.Update(frame, now)

			accumOneX += frame.RawOneDX
			accumOneY += frame.RawOneDY
			accumTwoX += frame.RawTwoDX
			accumTwoY += frame.RawTwoDY

			candidate := wireStateFromDualSense(decoded, frame, d.productID, touchSeq)
			candidate.OneDX = signedUnitToQ15(motionAxis(accumOneX))
			candidate.OneDY = signedUnitToQ15(motionAxis(accumOneY))
			candidate.TwoDX = signedUnitToQ15(motionAxis(accumTwoX))
			candidate.TwoDY = signedUnitToQ15(motionAxis(accumTwoY))

			immediate := !haveLast || extendedButtonsChanged(candidate, lastSent)
			touchActive := candidate.Count > 0
			changed := !haveLast || extendedTouchChanged(candidate, lastSent)
			dueTouch := touchActive && (lastTouchSend.IsZero() || now.Sub(lastTouchSend) >= touchPeriod)
			dueTouchStop := changed && !touchActive
			dueHeartbeat := lastTouchSend.IsZero() || now.Sub(lastTouchSend) >= heartbeat
			if immediate || dueTouch || dueTouchStop || dueHeartbeat {
				touchSeq++
				candidate.Seq = touchSeq
				if packet, e := encodeExtendedInputPacket(candidate, touchPacketBuf); e == nil {
					if _, e = touchConn.Write(packet); e == nil {
						lastSent, haveLast, lastTouchSend = candidate, true, now
						accumOneX, accumOneY, accumTwoX, accumTwoY = 0, 0, 0, 0
					}
				}
			}

			if motionConn != nil {
				dueMotion := motionSensorDue(lastMotionSensorTS, decoded.SensorTimestamp)
				if dueMotion {
					sample := calibration.apply(decoded)
					orientation.Update(sample)
					motionSeq++
					ms := motionInputWireState{
						Bluetooth:       decoded.ReportID == 0x31,
						Calibrated:      sample.Calibrated,
						Seq:             motionSeq,
						Axes:            motionFromSample(sample, orientation),
						CorrectedGyro:   correctedGyroWire(sample, orientation),
						Gravity:         gravityWire(orientation),
						LinearAccel:     linearAccelWire(sample, orientation),
						Quaternion:      orientation.QuaternionQ15(),
						SensorTimestamp: decoded.SensorTimestamp,
					}
					if packet, e := encodeMotionInputPacket(ms, motionPacketBuf); e == nil {
						if _, e = motionConn.Write(packet); e == nil {
							lastMotionSensorTS = decoded.SensorTimestamp
						}
					}
				}
			}
		}
	}()
}
