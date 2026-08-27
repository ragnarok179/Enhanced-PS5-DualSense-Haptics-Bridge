//go:build windows

package main

import (
	"encoding/binary"
	"math"
	"strings"
	"testing"
)

func TestBluetoothSpeakerPacketSharesReport36(t *testing.T) {
	frame := make([]byte, 200)
	for i := range frame {
		frame[i] = byte(i)
	}
	state := buildBluetoothSetStateData63ExternalRGB(telemetry{})
	r := buildBluetoothAllInOneReport36WithSpeaker(3, 9, make([]int8, 64), state, frame)
	if len(r) != 398 {
		t.Fatalf("len=%d", len(r))
	}
	if r[142] != 0x93 || r[143] != 200 {
		t.Fatalf("speaker header=%02x/%d", r[142], r[143])
	}
	for i := 0; i < 200; i++ {
		if r[144+i] != byte(i) {
			t.Fatalf("payload mismatch %d", i)
		}
	}
	if r[4] != 0xFE {
		t.Fatalf("mic-safe config mask changed: %02x", r[4])
	}
}

func TestBluetoothSpeakerSetupDoesNotSelectCompatibleRumble(t *testing.T) {
	r := buildBluetoothSpeakerSetupReport(2, 70, true)
	if len(r) != 78 {
		t.Fatalf("len=%d", len(r))
	}
	common := r[3:50]
	if common[0]&0x03 != 0 {
		t.Fatalf("compatible rumble selected: %02x", common[0])
	}
	if common[0]&0xA0 != 0xA0 || common[1]&0x80 == 0 {
		t.Fatal("speaker flags missing")
	}
	if common[5] != 80 || common[7] != 0x30 || common[37] != 0x02 {
		t.Fatal("speaker route fields incorrect")
	}
}

func TestSpeakerDefaultsSafeOffCollisionOnly(t *testing.T) {
	applySpeakerSettings(false, 70, speakerCategoryCollision, speakerOutputController)
	c := currentSpeakerSettings()
	if c.enabled {
		t.Fatal("speaker should be off")
	}
	if c.volume != 70 || c.categories != speakerCategoryCollision {
		t.Fatalf("unexpected defaults: %+v", c)
	}
}

func TestSpeakerMasksRemovedCategories(t *testing.T) {
	applySpeakerSettings(true, 80, 0xFFFF, speakerOutputController)
	if got := currentSpeakerSettings().categories; got != speakerCategoryCollision {
		t.Fatalf("removed categories leaked through mask: 0x%04X", got)
	}
}

func TestSpeakerDiagnosticTracksBeamNGSettingsHeartbeat(t *testing.T) {
	speakerBeamNGSettingsState.packets.Store(0)
	speakerBeamNGSettingsState.lastSeen.Store(0)
	applyBeamNGSpeakerSettings(true, 65, speakerCategoryCollision, speakerOutputController)
	defer applySpeakerSettings(false, 70, speakerCategoryCollision, speakerOutputController)
	s := newControllerSpeakerEngine()
	status := s.diagnosticStatus("USB")
	for _, want := range []string{"SPEAKER_STATUS transport=USB", "profile=collision-only", "enabled=true", "volume=65%", "settingsRx=1"} {
		if !strings.Contains(status, want) {
			t.Fatalf("status %q missing %q", status, want)
		}
	}
}

func TestUSBVoiceMixerLetsOverlappingCollisionsFinish(t *testing.T) {
	s := newControllerSpeakerEngine()
	s.usbOutput = true
	s.mu.Lock()
	s.enqueuePCMVoiceLocked("collision", "event:a", 1, []float32{0, .4, .4, 0})
	s.enqueuePCMVoiceLocked("collision", "event:b", 1, []float32{0, .2, .2, .2, .2, 0})
	s.mu.Unlock()
	buf := make([]byte, 6*2*4)
	if !s.renderUSB(buf, 6, 2, 8, true) {
		t.Fatal("mixer wrote no audio")
	}
	for i := 0; i < 6; i++ {
		bits := binary.LittleEndian.Uint32(buf[i*8+4:])
		v := math.Float32frombits(bits)
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			t.Fatalf("invalid sample at frame %d: %v", i, v)
		}
	}
	if len(s.voices) != 0 {
		t.Fatalf("completed voices retained: %d", len(s.voices))
	}
}

func TestBluetoothVoiceMixerOverlapsInsteadOfQueueing(t *testing.T) {
	s := newControllerSpeakerEngine()
	s.usbOutput = false
	s.mu.Lock()
	s.enqueuePCMVoiceLocked("collision", "event:a", 1, make([]float32, 700))
	s.enqueuePCMVoiceLocked("collision", "event:b", 1, make([]float32, 900))
	for i := range s.voices[0].pcm {
		s.voices[0].pcm[i] = .4
	}
	for i := range s.voices[1].pcm {
		s.voices[1].pcm[i] = .2
	}
	tick := make([]float32, 512)
	if !s.mixBluetoothSourceTickLocked(tick) {
		s.mu.Unlock()
		t.Fatal("Bluetooth overlap mixer wrote no PCM")
	}
	remainingVoices := len(s.voices)
	s.mu.Unlock()
	if remainingVoices != 2 {
		t.Fatalf("voices were serialized/dropped: remaining=%d", remainingVoices)
	}
	want := (.4 + .2) / math.Sqrt(2)
	if math.Abs(float64(tick[100])-want) > 1e-5 {
		t.Fatalf("mixed sample=%f want=%f", tick[100], want)
	}
}

func TestBluetoothSpeakerHardwareOutputGainIsPointEight(t *testing.T) {
	if math.Abs(bluetoothSpeakerHardwareOutputGain-0.80) > 1e-12 {
		t.Fatalf("bluetooth speaker hardware gain=%f", bluetoothSpeakerHardwareOutputGain)
	}
	r := buildBluetoothSpeakerSetupReport(2, 100, true)
	if got := r[3+5]; got != 80 {
		t.Fatalf("Bluetooth speaker hardware level=%d want=80", got)
	}
}

func TestBluetoothContinuousStateKeepsSpeakerHardwareLevelAtEighty(t *testing.T) {
	state := buildBluetoothSetStateData63ExternalRGB(telemetry{})
	if got := state[5]; got != 80 {
		t.Fatalf("continuous 0x36 SetStateData speaker level=%d want=80", got)
	}
	report := buildBluetoothAllInOneReport36WithSpeaker(1, 1, make([]int8, 64), state, speakerOpusSilence[:])
	if got := report[13+5]; got != 80 {
		t.Fatalf("0x36 wire speaker level=%d want=80", got)
	}
}
