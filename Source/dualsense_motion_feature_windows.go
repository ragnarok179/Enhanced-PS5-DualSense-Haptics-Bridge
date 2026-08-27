//go:build windows && (usb || bluetooth)

package main

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"
)

const dualSenseCalibrationFeatureSize = 41

func readDualSenseMotionCalibration(handle syscall.Handle) (dualSenseMotionCalibration, error) {
	if !validHandle(handle) {
		return dualSenseMotionCalibration{}, fmt.Errorf("invalid HID handle")
	}
	buf := make([]byte, dualSenseCalibrationFeatureSize)
	buf[0] = 0x05
	r, _, callErr := procGetFeature.Call(
		uintptr(handle),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
	)
	runtime.KeepAlive(buf)
	if r == 0 {
		return dualSenseMotionCalibration{}, fmt.Errorf("HidD_GetFeature(0x05): %v", callErr)
	}
	return parseDualSenseMotionCalibration(buf)
}

// Diagnostic-only raw calibration fetch. Keeping it separate from
// readDualSenseMotionCalibration leaves the normal runtime path unchanged while
// allowing the hardware log to include the exact 0x05 bytes for later review.
func readDualSenseMotionCalibrationFeature(handle syscall.Handle) ([]byte, error) {
	if !validHandle(handle) {
		return nil, fmt.Errorf("invalid HID handle")
	}
	buf := make([]byte, dualSenseCalibrationFeatureSize)
	buf[0] = 0x05
	r, _, callErr := procGetFeature.Call(
		uintptr(handle),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
	)
	runtime.KeepAlive(buf)
	if r == 0 {
		return nil, fmt.Errorf("HidD_GetFeature(0x05): %v", callErr)
	}
	return buf, nil
}
