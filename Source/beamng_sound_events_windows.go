//go:build windows

package main

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

// DSE1 is the localhost-only exact BeamNG sound-event protocol. In the
// collision-only profile, the Bridge accepts exactly one public event shape:
// oneshot/collision. Non-public sound categories are rejected at parse time.
type beamNGSoundEvent struct {
	Op        string
	Kind      string
	Sequence  uint64
	Source    string
	Volume    float64
	Pitch     float64
	Color     float64
	Texture   float64
	EventPath string
}

func parseBeamNGSoundEventPacket(data []byte) (beamNGSoundEvent, bool) {
	if !strings.HasPrefix(string(data), "DSE1\t") {
		return beamNGSoundEvent{}, false
	}
	parts := strings.SplitN(string(data), "\t", 11)
	if len(parts) != 11 || parts[0] != "DSE1" || parts[1] != "1" {
		return beamNGSoundEvent{}, false
	}
	seq, err := strconv.ParseUint(parts[4], 10, 64)
	if err != nil {
		return beamNGSoundEvent{}, false
	}
	volume, err1 := strconv.ParseFloat(parts[6], 64)
	pitch, err2 := strconv.ParseFloat(parts[7], 64)
	color, err3 := strconv.ParseFloat(parts[8], 64)
	texture, err4 := strconv.ParseFloat(parts[9], 64)
	if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
		return beamNGSoundEvent{}, false
	}
	if parts[3] != "collision" {
		return beamNGSoundEvent{}, false
	}
	if parts[2] == "reset" {
		return beamNGSoundEvent{
			Op:        "reset",
			Kind:      "collision",
			Sequence:  seq,
			Source:    parts[5],
			Volume:    0,
			Pitch:     1,
			Color:     0,
			Texture:   0,
			EventPath: parts[10],
		}, true
	}
	if parts[2] != "oneshot" || parts[10] == "" {
		return beamNGSoundEvent{}, false
	}
	return beamNGSoundEvent{
		Op:        "oneshot",
		Kind:      "collision",
		Sequence:  seq,
		Source:    parts[5],
		Volume:    volume,
		Pitch:     pitch,
		Color:     color,
		Texture:   texture,
		EventPath: parts[10],
	}, true
}

var exactSpeakerRoute struct {
	sync.RWMutex
	engine *controllerSpeakerEngine
}

func registerExactBeamNGSpeaker(s *controllerSpeakerEngine) func() {
	exactSpeakerRoute.Lock()
	exactSpeakerRoute.engine = s
	exactSpeakerRoute.Unlock()
	return func() {
		exactSpeakerRoute.Lock()
		if exactSpeakerRoute.engine == s {
			exactSpeakerRoute.engine = nil
		}
		exactSpeakerRoute.Unlock()
	}
}

func dispatchBeamNGSoundEventPacket(data []byte, now time.Time) bool {
	e, ok := parseBeamNGSoundEventPacket(data)
	if !ok {
		return false
	}
	exactSpeakerRoute.RLock()
	s := exactSpeakerRoute.engine
	exactSpeakerRoute.RUnlock()
	if s != nil {
		s.handleExactBeamNGSoundEvent(e, now)
	}
	return true
}

func beamNGVehicleIDFromSource(source string) int {
	for _, field := range strings.Split(source, ";") {
		field = strings.TrimSpace(field)
		if !strings.HasPrefix(field, "veh=") {
			continue
		}
		id, err := strconv.Atoi(strings.TrimPrefix(field, "veh="))
		if err == nil {
			return id
		}
	}
	return -1
}

func (s *controllerSpeakerEngine) handleExactBeamNGSoundEvent(e beamNGSoundEvent, now time.Time) {
	if s == nil {
		return
	}
	vehicleID := beamNGVehicleIDFromSource(e.Source)
	if e.Op == "reset" {
		s.flushVehicle(vehicleID, "beamng_reset")
		if runtimeDiagnosticsEnabled() {
			fmt.Printf("NATIVE_EXACT_RESET seq=%d kind=collision source=%q vehicle=%d decision=flush\n", e.Sequence, e.Source, vehicleID)
		}
		return
	}
	cfg := currentSpeakerSettings()
	s.mu.Lock()
	s.eventsSeen++
	decision := "play"
	categoryOn := cfg.categories&speakerCategoryCollision != 0
	if !cfg.enabled {
		decision = "speaker_off"
		s.eventsFiltered++
	} else if !categoryOn {
		decision = "category_off"
		s.eventsFiltered++
	} else if cfg.volume <= 0 || e.Volume <= 0 {
		decision = "silent"
		s.eventsFiltered++
	}

	if runtimeDiagnosticsEnabled() {
		fmt.Printf("NATIVE_EXACT_EVENT seq=%d op=oneshot kind=collision source=%q event=%q volume=%.4f pitch=%.4f color=%.4f texture=%.4f enabled=%t categoryOn=%t decision=%s\n",
			e.Sequence, e.Source, e.EventPath, e.Volume, e.Pitch, e.Color, e.Texture, cfg.enabled, categoryOn, decision)
	}
	s.mu.Unlock()
	if decision != "play" {
		return
	}
	s.triggerExact("collision", e.EventPath, vehicleID, clamp01(e.Volume), cfg, now)
}
