//go:build windows && usb

package main

import "testing"

func TestUSBSharedReportDoesNotOwnLightbar(t *testing.T) {
	d := &device{outputLen: 48}
	r := buildUSBSharedStateReport(d, telemetry{}, [3]byte{0xAA, 0x55, 0x11})
	if len(r) < 48 {
		t.Fatalf("report too short: %d", len(r))
	}
	common := r[1:48]
	if common[1]&0x04 != 0 {
		t.Fatalf("USB bridge unexpectedly claims lightbar ownership: valid_flag1=%02x", common[1])
	}
	if common[44] != 0 || common[45] != 0 || common[46] != 0 {
		t.Fatalf("USB bridge unexpectedly writes RGB: %02x%02x%02x", common[44], common[45], common[46])
	}
	if common[0]&0x0C != 0x0C {
		t.Fatalf("adaptive trigger enables missing: valid_flag0=%02x", common[0])
	}
}
