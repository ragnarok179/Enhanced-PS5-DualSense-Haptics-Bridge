//go:build windows && (usb || bluetooth)

package main

import (
	"testing"
	"unsafe"
)

func TestTouchpadSendInputABI64(t *testing.T) {
	if unsafe.Sizeof(uintptr(0)) != 8 {
		t.Skip("release bridge targets Windows amd64")
	}
	if got := unsafe.Sizeof(mouseInputData{}); got != 32 {
		t.Fatalf("MOUSEINPUT size=%d, want 32", got)
	}
	if got := unsafe.Sizeof(inputRecord{}); got != 40 {
		t.Fatalf("INPUT size=%d, want 40", got)
	}
	var in inputRecord
	if got := unsafe.Offsetof(in.Mi); got != 8 {
		t.Fatalf("INPUT.mi offset=%d, want 8", got)
	}
}

func TestReservedTwoFingerTapCancelsPendingRightClick(t *testing.T) {
	m := &touchpadMouseController{pending: pendingMouseTap{active: true, left: false}}
	updateTouchpadBindingState(true, false, true, true)
	m.cancelReservedTwoFingerTap()
	if m.pending.active {
		t.Fatal("reserved 2F tap left a pending right click")
	}
}
