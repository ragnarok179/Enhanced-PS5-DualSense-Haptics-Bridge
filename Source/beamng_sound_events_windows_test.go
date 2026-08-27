//go:build windows

package main

import (
	"strings"
	"testing"
	"time"
)

func TestParseExactCollisionPacket(t *testing.T) {
	packet := []byte("DSE1\t1\toneshot\tcollision\t42\tveh=1;node=2\t0.75\t1\t0.2\t0\tevent:>Destruction>Vehicle>vehicle_part_impact")
	e, ok := parseBeamNGSoundEventPacket(packet)
	if !ok {
		t.Fatal("valid collision packet rejected")
	}
	if e.Kind != "collision" || e.Op != "oneshot" || e.Sequence != 42 || e.Volume != .75 {
		t.Fatalf("unexpected event: %+v", e)
	}
}

func TestParseCollisionResetPacket(t *testing.T) {
	packet := []byte("DSE1\t1\treset\tcollision\t43\tveh=17;reason=reset\t0\t1\t0\t0\t-")
	e, ok := parseBeamNGSoundEventPacket(packet)
	if !ok || e.Op != "reset" || e.Kind != "collision" || e.Sequence != 43 {
		t.Fatalf("valid reset packet rejected: %+v ok=%t", e, ok)
	}
	if got := beamNGVehicleIDFromSource(e.Source); got != 17 {
		t.Fatalf("vehicle id=%d want=17", got)
	}
}

func TestCollisionResetFlushesOnlyTargetVehicle(t *testing.T) {
	s := newControllerSpeakerEngine()
	s.mu.Lock()
	s.enqueuePCMVoiceLocked("collision", "event:a", 17, []float32{.2, .2})
	s.enqueuePCMVoiceLocked("collision", "event:b", 18, []float32{.2, .2})
	s.mu.Unlock()
	s.handleExactBeamNGSoundEvent(beamNGSoundEvent{Op: "reset", Kind: "collision", Sequence: 2, Source: "veh=17;reason=reset"}, time.Now())
	if len(s.voices) != 1 || s.voices[0].vehicleID != 18 {
		t.Fatalf("targeted reset left voices=%+v", s.voices)
	}
}

func TestRejectsExperimentalSpeakerKinds(t *testing.T) {
	for _, packet := range []string{
		"DSE1\t1\tloop_start\tsuspension\t1\tveh=1\t0.5\t1\t0\t0\tevent:>Vehicle>Suspension>x",
		"DSE1\t1\toneshot\tshift\t1\tveh=1\t1\t1\t0\t0\tevent:>Vehicle>Interior>Gearshift>x",
		"DSE1\t1\toneshot\ttire\t1\tveh=1\t1\t1\t0\t0\tevent:>Vehicle>Failures>tire_burst",
	} {
		if _, ok := parseBeamNGSoundEventPacket([]byte(packet)); ok {
			t.Fatalf("experimental event accepted: %s", packet)
		}
	}
}

func TestCollisionExactEventRespectsSettings(t *testing.T) {
	old := currentSpeakerSettings()
	defer applySpeakerSettings(old.enabled, old.volume, old.categories, old.output)

	s := newControllerSpeakerEngine()
	e := beamNGSoundEvent{Op: "oneshot", Kind: "collision", Sequence: 1, Source: "veh=1;node=2", Volume: .8, Pitch: 1, EventPath: "event:>Destruction>Vehicle>vehicle_part_impact"}
	applySpeakerSettings(false, 80, speakerCategoryCollision, speakerOutputController)
	s.handleExactBeamNGSoundEvent(e, time.Now())
	if s.eventsPlayed != 0 || s.eventsFiltered != 1 {
		t.Fatalf("disabled speaker played event: played=%d filtered=%d", s.eventsPlayed, s.eventsFiltered)
	}

	applySpeakerSettings(true, 80, 0, speakerOutputController)
	s.handleExactBeamNGSoundEvent(e, time.Now())
	if s.eventsPlayed != 0 || s.eventsFiltered != 2 {
		t.Fatalf("disabled collision category played event")
	}

	applySpeakerSettings(true, 80, speakerCategoryCollision, speakerOutputController)
	s.handleExactBeamNGSoundEvent(e, time.Now())
	if s.eventsPlayed != 1 {
		t.Fatalf("enabled collision did not play: %d", s.eventsPlayed)
	}
}

func TestDiagnosticNamesCollisionOnlyProfile(t *testing.T) {
	s := newControllerSpeakerEngine()
	line := s.diagnosticStatus("USB")
	if !strings.Contains(line, "profile=collision-only") || !strings.Contains(line, "collision=") {
		t.Fatalf("unexpected status: %s", line)
	}
}
