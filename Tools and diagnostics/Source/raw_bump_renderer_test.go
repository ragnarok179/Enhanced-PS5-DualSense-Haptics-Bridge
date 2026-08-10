package main

import (
	"testing"
	"time"
)

func TestRawBumpRendererHardPanAndFixedPulse(t *testing.T) {
	r := newRawBumpRenderer()
	now := time.Now()
	r.observe(telemetry{Active: true, BodyEvent: 10}, now) // synchronize
	r.observe(telemetry{Active: true, BodyEvent: 11, BodyKind: "wheel", BodyProfile: "suspension_bump", BodySide: -1, BodyStrength: .03}, now.Add(time.Millisecond))
	out := r.render(200, canonicalHapticSampleRate, now.Add(2*time.Millisecond))
	var leftNZ, rightNZ int
	for i := 0; i+1 < len(out); i += 2 {
		if out[i] != 0 {
			leftNZ++
		}
		if out[i+1] != 0 {
			rightNZ++
		}
	}
	if leftNZ == 0 {
		t.Fatal("left bump produced no PCM")
	}
	if rightNZ != 0 {
		t.Fatalf("left bump leaked to right channel: %d samples", rightNZ)
	}
}

func TestRawBumpRendererDoesNotAcceptLandingOrCollision(t *testing.T) {
	r := newRawBumpRenderer()
	now := time.Now()
	r.observe(telemetry{Active: true, BodyEvent: 1}, now)
	r.observe(telemetry{Active: true, BodyEvent: 2, BodyKind: "landing", BodyProfile: "landing", BodySide: 0}, now.Add(time.Millisecond))
	r.observe(telemetry{Active: true, BodyEvent: 3, BodyKind: "collision", BodyProfile: "collision", BodySide: 1}, now.Add(2*time.Millisecond))
	st := r.stats()
	if st.Accepted != 0 {
		t.Fatalf("accepted non-bump events: %d", st.Accepted)
	}
}

func TestRawBumpRendererQueuesInsteadOfMerging(t *testing.T) {
	r := newRawBumpRenderer()
	now := time.Now()
	r.observe(telemetry{Active: true, BodyEvent: 5}, now)
	r.observe(telemetry{Active: true, BodyEvent: 6, BodyKind: "wheel", BodyProfile: "suspension_bump", BodySide: -1}, now.Add(time.Millisecond))
	r.observe(telemetry{Active: true, BodyEvent: 7, BodyKind: "wheel", BodyProfile: "suspension_bump", BodySide: 1}, now.Add(2*time.Millisecond))
	st := r.stats()
	if st.Accepted != 2 || st.Pending != 2 {
		t.Fatalf("expected 2 queued bumps, got accepted=%d pending=%d", st.Accepted, st.Pending)
	}
	_ = r.render(16, canonicalHapticSampleRate, now.Add(3*time.Millisecond))
	st = r.stats()
	if st.Played != 1 || st.Pending != 1 || st.Side != -1 {
		t.Fatalf("unexpected first playback state: %+v", st)
	}
}
