package main

import (
	"encoding/binary"
	"encoding/json"
	"testing"
)

func TestV150ResistanceEndZoneBounds(t *testing.T) {
	samples := []struct {
		start, end int
		want       uint16
	}{
		{0, 10, 0x03ff}, {2, 7, 0x007c}, {9, 10, 0x0200}, {4, 4, 0x0010},
	}
	for _, tc := range samples {
		b := make([]byte, 11)
		fillOfficialFeedback(b, tc.start, 80, 190, tc.end)
		if b[0] != 0x21 || binary.LittleEndian.Uint16(b[1:3]) != tc.want {
			t.Fatalf("start=%d end=%d: got mode=%x mask=%04x, want %04x", tc.start, tc.end, b[0], binary.LittleEndian.Uint16(b[1:3]), tc.want)
		}
	}
	b := make([]byte, 11)
	fillOfficialFeedback(b, 2, 80, 190) // Legacy protocol: no upper bound
	if got := binary.LittleEndian.Uint16(b[1:3]); got != 0x03fc {
		t.Fatalf("legacy mask %04x", got)
	}
}

func TestV150DecodeSemanticTriggerEndZone(t *testing.T) {
	var x telemetry
	if err := json.Unmarshal([]byte(`{"l2Effect":{"kind":"resistance","startPosition":0.2,"endPosition":0.7,"startForce":0.25,"endForce":0.75},"r2Effect":{"kind":"resistance","startPosition":0.4,"startForce":0.2,"endForce":0.3}}`), &x); err != nil {
		t.Fatal(err)
	}
	normalizeSemanticTriggerEffects(&x)
	if x.L2StartZone != 2 || x.L2EndZone != 7 {
		t.Fatalf("L2: %+v", x)
	}
	if x.R2StartZone != 4 || x.R2EndZone != 10 {
		t.Fatalf("legacy R2 end: %+v", x)
	}
	data := make([]byte, 11)
	fillTrigger(data, x.L2Mode, x.L2StartZone, x.L2StartStrength, x.L2EndStrength, x.L2Amplitude, x.L2Hz, x.L2EndZone)
	if got := binary.LittleEndian.Uint16(data[1:3]); got != 0x007c {
		t.Fatalf("L2 output mask %04x", got)
	}
}
