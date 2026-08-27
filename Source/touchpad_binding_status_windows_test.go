//go:build windows && (usb || bluetooth)

package main

import (
	"testing"
	"time"
)

func TestTouchpadMouseOwnershipMatrix(t *testing.T) {
	cases := []struct {
		name                               string
		bound, capture, mouseForced, fresh bool
		want                               bool
	}{
		{"unbound normal", false, false, false, true, true},
		{"binding editor", false, true, false, true, false},
		{"bound touchpad", true, false, false, true, false},
		{"bound and capture", true, true, false, true, false},
		{"stale BeamNG heartbeat", true, true, false, false, true},
		{"forced mouse overrides stored touch binding", true, false, true, true, true},
		{"binding capture still wins over forced mouse", true, true, true, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := touchpadMouseAllowedForState(tc.bound, tc.capture, tc.mouseForced, tc.fresh); got != tc.want {
				t.Fatalf("allowed=%v, want %v", got, tc.want)
			}
		})
	}
}

func TestTwoFingerReservationSurvivesStaleOwnershipStatus(t *testing.T) {
	updateTouchpadBindingState(true, false, true, true)
	touchpadBindingState.lastSeen.Store(time.Now().Add(-10 * time.Second).UnixNano())
	if touchpadMouseTwoFingerTapAllowed() {
		t.Fatal("stale ownership status re-enabled reserved 2F right-click")
	}
	updateTouchpadBindingState(false, false, false, false)
	if !touchpadMouseTwoFingerTapAllowed() {
		t.Fatal("fresh unreserved status did not restore 2F right-click")
	}
}
