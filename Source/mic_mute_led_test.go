package main

import (
	"testing"
	"time"
)

// This inspection helper exists only for white-box unit tests; runtime code
// publishes changes through micMuteLEDUpdateDue instead.
func micMuteLEDSnapshot() (toggle, on bool, generation uint64) {
	muteLED.RLock()
	toggle, on, generation = muteLED.toggle, muteLED.on, muteLED.generation
	muteLED.RUnlock()
	return
}

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

func TestMicMuteLEDChangesImmediatelyFromBeamNG(t *testing.T) {
	updateMicMuteLED(false, false)
	defer updateMicMuteLED(false, false)
	updateMicMuteLED(true, false)
	_, _, previous := micMuteLEDSnapshot()
	for _, expected := range []bool{true, false, true, false} {
		// Consecutive confirmed statuses must never be delayed by a HID press guard.
		updateMicMuteLED(true, expected)
		due, on, next := micMuteLEDUpdateDue(previous, time.Now(), time.Now())
		if !due || on != expected || next == previous {
			t.Fatalf("immediate status failed: expected=%t due=%t on=%t", expected, due, on)
		}
		previous = next
	}
}

func TestMicMuteBackgroundWatchdogKeepsLEDNeutralUntilGameExits(t *testing.T) {
	updateMicMuteLED(false, false)
	defer updateMicMuteLED(false, false)
	now := time.Now()
	if !handleMicMuteStatusPacket([]byte{'D', 'M', 'B', '2', 1, 1, 1, 0}) {
		t.Fatal("DMB2 background status not accepted")
	}
	_, _, baseline := micMuteLEDSnapshot()
	noteBeamNGProcessFocusSample(true, false, now)
	due, on, _ := micMuteLEDUpdateDue(baseline, now, now.Add(4*time.Second))
	if !due || on {
		t.Fatal("Alt+Tab must not preserve an uncertain orange LED")
	}
	noteBeamNGProcessFocusSample(false, false, now.Add(4*time.Second))
	due, on, _ = micMuteLEDUpdateDue(baseline, now, now.Add(5*time.Second))
	if !due || on {
		t.Fatal("BeamNG process exit must clear stale background LED")
	}
}

func TestMicMuteBackgroundWithoutFinalFocusPacket(t *testing.T) {
	updateMicMuteLED(false, false)
	defer updateMicMuteLED(false, false)
	updateMicMuteLED(true, true) // last status was sent before Alt+Tab
	_, _, baseline := micMuteLEDSnapshot()
	now := time.Now()
	noteBeamNGProcessFocusSample(true, false, now) // OS sees background game
	_, on, _ := micMuteLEDUpdateDue(baseline, now, now.Add(4*time.Second))
	if on {
		t.Fatal("background BeamNG must release stale LED even without a Lua focus report")
	}
	noteBeamNGProcessFocusSample(true, true, now.Add(4*time.Second))
	_, on, _ = micMuteLEDUpdateDue(baseline, now, now.Add(5*time.Second))
	if on {
		t.Fatal("foreground BeamNG with dead status stream must clear LED")
	}
}

func TestMicMuteForegroundWatchdogStillClears(t *testing.T) {
	updateMicMuteLED(false, false)
	defer updateMicMuteLED(false, false)
	now := time.Now()
	if !handleMicMuteStatusPacket([]byte{'D', 'M', 'B', '2', 1, 1, 1, 1}) {
		t.Fatal("DMB2 foreground status not accepted")
	}
	_, _, gen := micMuteLEDSnapshot()
	noteBeamNGProcessFocusSample(true, true, now)
	_, on, _ := micMuteLEDUpdateDue(gen, now, now.Add(4*time.Second))
	if on {
		t.Fatal("foreground stream loss must clear LED despite running game")
	}
}

func TestMicMuteStatusPacketValidationAndLegacy(t *testing.T) {
	updateMicMuteLED(false, false)
	defer updateMicMuteLED(false, false)
	for _, packet := range [][]byte{
		{}, []byte("DMB2"), []byte{'D', 'M', 'B', '2', 1, 1, 1, 2},
		[]byte{'D', 'M', 'B', '2', 2, 1, 1, 0}, []byte{'D', 'M', 'B', '1', 1, 1, 0, 1},
	} {
		if handleMicMuteStatusPacket(packet) {
			t.Fatalf("malformed Mic Mute packet accepted: %v", packet)
		}
	}
	if !handleMicMuteStatusPacket([]byte{'D', 'M', 'B', '1', 1, 1, 1}) {
		t.Fatal("older DMB1 status rejected")
	}
	_, on, _ := micMuteLEDSnapshot()
	if !on {
		t.Fatal("older DMB1 LED status not applied")
	}
}

func TestMicMuteFocusLossClearsLEDAndExposesEpoch(t *testing.T) {
	updateMicMuteLED(false, false)
	defer updateMicMuteLED(false, false)
	now := time.Now()
	noteBeamNGProcessFocusSample(true, true, now)
	before := micMuteFocusLossGeneration()
	updateMicMuteLED(true, true)
	noteBeamNGProcessFocusSample(true, false, now.Add(250*time.Millisecond))
	_, on, _ := micMuteLEDSnapshot()
	if on {
		t.Fatal("Alt+Tab must extinguish stale LED when BeamNG releases its input")
	}
	if (int(micMuteFocusLossGeneration())-int(before)+4)%4 != 1 {
		t.Fatal("missed focus-loss epoch")
	}
	// Stale Lua DMB1/DMB2 must not immediately re-light the LED in background.
	updateMicMuteLEDWithFocus(true, true, false)
	_, on, _ = micMuteLEDSnapshot()
	if on {
		t.Fatal("stale background DMB2 re-lit LED")
	}
	noteBeamNGProcessFocusSample(true, true, now.Add(500*time.Millisecond))
	_, on, _ = micMuteLEDSnapshot()
	if on {
		t.Fatal("focus return must not restore the old latch automatically")
	}
}
