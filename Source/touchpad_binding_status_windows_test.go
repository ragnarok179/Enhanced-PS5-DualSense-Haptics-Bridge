//go:build windows && (usb || bluetooth)

package main

import (
	"io"
	"os"
	"strings"
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
		{"stale BeamNG heartbeat", true, true, false, false, false},
		{"explicit mouse overrides bound touchpad", true, false, true, true, true},
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

func TestTouchpadMouseExplicitPolicyAndBindingPriority(t *testing.T) {
	cases := []struct {
		name           string
		bound, capture bool
		policy         int32
		fresh, want    bool
	}{
		{"automatic fallback when no touch binding", false, false, 0, true, true},
		{"automatic fallback disabled when bound", true, false, 0, true, false},
		{"manual on without binding", false, false, 1, true, true},
		{"manual on overrides saved touch binding", true, false, 1, true, true},
		{"manual off unbound", false, false, 2, true, false},
		{"manual off with bindings", true, false, 2, true, false},
		{"binding editor always wins", false, true, 1, true, false},
		{"no new packets never re-enables Windows cursor", false, false, 1, false, false},
		{"invalid policy remains disabled", false, false, 99, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := touchpadMouseAllowedForPolicy(tc.bound, tc.capture, tc.policy, tc.fresh); got != tc.want {
				t.Fatalf("allowed=%t want=%t", got, tc.want)
			}
		})
	}
	updateTouchpadBindingPolicy(false, false, 2, false)
	if touchpadMouseAllowed() {
		t.Fatal("manual OFF still injected Windows mouse")
	}
	updateTouchpadBindingPolicy(false, false, 1, false)
	if !touchpadMouseAllowed() {
		t.Fatal("manual ON failed")
	}
	updateTouchpadBindingPolicy(true, false, 1, false)
	if !touchpadMouseAllowed() {
		t.Fatal("explicit mouse ON should override saved binding")
	}
	updateTouchpadBindingPolicy(true, true, 1, false)
	if touchpadMouseAllowed() {
		t.Fatal("binding editor capture must override mouse ON")
	}
}

// The ordinary Bridge window should display controller and BeamNG connection
// messages, never technical touchpad ownership changes.
func TestTouchpadStateTransitionsStayOutOfPublicConsole(t *testing.T) {
	setTouchpadBindingDiagnosticsEnabled(false)
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	previous := os.Stdout
	os.Stdout = writer
	updateTouchpadBindingPolicy(false, false, 1, false)
	updateTouchpadBindingPolicy(true, false, 1, false)
	updateTouchpadBindingPolicy(true, false, 2, false)
	updateTouchpadBindingPolicy(true, true, 2, false)
	_ = writer.Close()
	os.Stdout = previous
	defer reader.Close()
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) > 0 {
		t.Fatalf("public console leaked internal touchpad state: %s", data)
	}
}

func TestTouchpadTransitionsRemainAvailableInExplicitDiagnostics(t *testing.T) {
	setTouchpadBindingDiagnosticsEnabled(true)
	defer setTouchpadBindingDiagnosticsEnabled(false)
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	previous := os.Stdout
	os.Stdout = writer
	updateTouchpadBindingPolicy(false, false, 1, false)
	updateTouchpadBindingPolicy(true, false, 2, false)
	_ = writer.Close()
	os.Stdout = previous
	defer reader.Close()
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "Touchpad ownership:") {
		t.Fatalf("explicit diagnostic lost touchpad details: %q", data)
	}
}
