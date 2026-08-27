package main

import (
	"fmt"
	"time"
)

// runtimeUserStatus keeps normal user-facing lifecycle messages shared by USB
// and Bluetooth. Detailed per-frame state remains diagnostic-only.
type runtimeUserStatus struct {
	startedAt      time.Time
	missingShown   bool
	connectedSeen  bool
	connectionLost bool
}

func newRuntimeUserStatus(now time.Time) *runtimeUserStatus {
	return &runtimeUserStatus{startedAt: now}
}

func (s *runtimeUserStatus) tick(lastPacket, now time.Time, diagnostics bool) {
	if s == nil || diagnostics {
		return
	}
	if lastPacket.IsZero() {
		if !s.missingShown && now.Sub(s.startedAt) >= 10*time.Second {
			fmt.Println("BeamNG.drive mod not detected yet. Waiting for Enhanced PS5 DualSense Haptics telemetry...")
			s.missingShown = true
		}
		return
	}

	age := now.Sub(lastPacket)
	if age <= 1200*time.Millisecond {
		s.connectedSeen = true
		if s.connectionLost {
			fmt.Println("BeamNG.drive connection restored.")
			s.connectionLost = false
		}
		return
	}

	if s.connectedSeen && !s.connectionLost && age >= 3*time.Second {
		fmt.Println("BeamNG.drive connection lost. Waiting for mod telemetry...")
		s.connectionLost = true
	}
}
