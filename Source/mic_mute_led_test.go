package main

import (
	"testing"
	"time"
)

func TestMuteLEDOnlyControlsTheLEDFlag(t *testing.T) {
	common := make([]byte, 47)
	common[1] = 0x80
	applyMicMuteLEDReport(common, true, true)
	if common[1] != 0x81 || common[8] != 1 || common[9] != 0 || common[44] != 0 {
		t.Fatalf("unexpected HID side-effects: flag1=%02x led=%d power=%d rgb=%d", common[1], common[8], common[9], common[44])
	}
	other := make([]byte, 47)
	applyMicMuteLEDReport(other, false, true)
	if other[1] != 0 || other[8] != 0 {
		t.Fatal("LED ownership written outside a valid update")
	}
}

func TestMuteLEDStateChangesAndRefresh(t *testing.T) {
	// Restore global state to normal on exit to avoid leaking state to tests.
	updateMicMuteLED(false, false)
	defer updateMicMuteLED(false, false)
	_, _, baseline := micMuteLEDSnapshot()
	updateMicMuteLED(true, false)
	due, on, gen := micMuteLEDUpdateDue(baseline, time.Now(), time.Now())
	if !due || on || gen == baseline {
		t.Fatalf("enter toggle did not request LED off: %t %t %d", due, on, gen)
	}
	updateMicMuteLED(true, true)
	due, on, gen2 := micMuteLEDUpdateDue(gen, time.Now(), time.Now())
	if !due || !on || gen2 == gen {
		t.Fatal("latched ON did not request LED on")
	}
	now := time.Now()
	due, _, _ = micMuteLEDUpdateDue(gen2, now, now.Add(100*time.Millisecond))
	if due {
		t.Fatal("LED writes must not occur every HID frame")
	}
	due, _, _ = micMuteLEDUpdateDue(gen2, now, now.Add(1100*time.Millisecond))
	if !due {
		t.Fatal("missing slow LED reassertion")
	}
	updateMicMuteLED(false, false)
	due, on, _ = micMuteLEDUpdateDue(gen2, now, now)
	if !due || on {
		t.Fatal("leaving toggle must clear orange LED once")
	}
}

func TestMicMuteBluetoothStatePreservesRGBAndMicAudio(t *testing.T) {
	s := buildBluetoothStateWithMuteLED(telemetry{}, true, true)
	if len(s) != 63 || s[1] != 0x01 || s[8] != 1 || s[9] != 0 {
		t.Fatalf("wrong LED-only secondary state: len=%d flags=%02x led=%d micPower=%02x", len(s), s[1], s[8], s[9])
	}
	for _, offset := range []int{44, 45, 46} {
		if s[offset] != 0 {
			t.Fatalf("LED must not own RGB at offset %d", offset)
		}
	}
	s = buildBluetoothStateWithMuteLED(telemetry{}, false, true)
	if s[1] != 0 || s[8] != 0 {
		t.Fatal("invalid transient LED ownership when unchanged")
	}

}

func TestMicMuteWatchdogClearsLostBeamNGToggle(t *testing.T) {
	updateMicMuteLED(false, false)
	defer updateMicMuteLED(false, false)
	updateMicMuteLED(true, true)
	_, _, gen := micMuteLEDSnapshot()
	due, on, next := micMuteLEDUpdateDue(gen, time.Now(), time.Now().Add(4*time.Second))
	if !due || on || next == gen {
		t.Fatal("stale BeamNG LED ownership should be relinquished and cleared")
	}
}

func TestPhysicalMicPressUpdatesLEDBeforeBeamNGStatus(t *testing.T) {
	updateMicMuteLED(false, false)
	defer updateMicMuteLED(false, false)
	updateMicMuteLED(true, false)
	_, _, baseline := micMuteLEDSnapshot()
	notePhysicalMicMutePress(time.Now())
	due, on, generation := micMuteLEDUpdateDue(baseline, time.Now(), time.Now())
	if !due || !on || generation == baseline {
		t.Fatal("physical Mic down must light LED without waiting for Lua")
	}
	// A queued old DMB1 status must not flash the LED off immediately.
	updateMicMuteLED(true, false)
	_, on, _ = micMuteLEDUpdateDue(generation, time.Now(), time.Now())
	if !on {
		t.Fatal("delayed pre-press state caused a spurious flash")
	}
	// A newer vehicle state remains authoritative once the short guard expires.
	muteLED.Lock()
	muteLED.lastLocalPress = time.Now().Add(-time.Second)
	muteLED.Unlock()
	updateMicMuteLED(true, false)
	_, on, _ = micMuteLEDUpdateDue(generation, time.Now(), time.Now())
	if on {
		t.Fatal("vehicle rejection was not reflected after guard")
	}
	// Button mode does not latch an orange LED.
	updateMicMuteLED(false, false)
	_, _, before := micMuteLEDSnapshot()
	notePhysicalMicMutePress(time.Now())
	_, on, after := micMuteLEDSnapshot()
	if on || after != before {
		t.Fatal("button mode must not toggle LED locally")
	}
}
