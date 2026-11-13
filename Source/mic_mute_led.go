package main

import (
	"sync"
	"time"
)

// Mic Mute is a momentary input, not a hardware latching switch. BeamNG owns
// the virtual latch, the Bridge mirrors it to the orange LED. This never mutes
// the physical microphone or writes power-save/mic-audio control flags.
var muteLED struct {
	sync.RWMutex
	toggle         bool
	on             bool
	generation     uint64
	lastStatus     time.Time
	lastLocalPress time.Time
}

const muteLEDPressConfirmationWindow = 90 * time.Millisecond

// The input stream sees the physical Mic press before BeamNG can forward its
// DMB1 latch state. Update the indicator immediately; BeamNG will reconcile
// the result after its action has executed.
func notePhysicalMicMutePress(now time.Time) {
	muteLED.Lock()
	defer muteLED.Unlock()
	if !muteLED.toggle || muteLED.lastStatus.IsZero() || now.Sub(muteLED.lastStatus) > 3*time.Second {
		return
	}
	muteLED.on = !muteLED.on
	muteLED.generation++
	muteLED.lastLocalPress = now
}

func updateMicMuteLED(toggle, on bool) {
	if !toggle {
		on = false
	}
	muteLED.Lock()
	now := time.Now()
	muteLED.lastStatus = now
	// A delayed DMB1 frame can still describe the pre-press state. Hold the
	// optimistic LED briefly to avoid an OFF/ON flicker. Later authoritative
	// state reports, including a rejected vehicle action, always win.
	if toggle && on != muteLED.on && now.Sub(muteLED.lastLocalPress) < muteLEDPressConfirmationWindow {
		muteLED.Unlock()
		return
	}
	if muteLED.toggle != toggle || muteLED.on != on {
		muteLED.toggle = toggle
		muteLED.on = on
		muteLED.generation++
	}
	muteLED.Unlock()
}

func micMuteLEDSnapshot() (toggle, on bool, generation uint64) {
	muteLED.RLock()
	toggle, on, generation = muteLED.toggle, muteLED.on, muteLED.generation
	muteLED.RUnlock()
	return
}

// Send on change; refresh at most once a second while toggle mode is active
// because Windows/other software could overwrite the visible LED state.
func micMuteLEDUpdateDue(lastGeneration uint64, lastSent time.Time, now time.Time) (apply, on bool, generation uint64) {
	// Lua refreshes the state every 500 ms. If BeamNG or the mod disappears,
	// clear its orange LED rather than leaving a stale gameplay indicator on.
	muteLED.Lock()
	if muteLED.toggle && !muteLED.lastStatus.IsZero() && now.Sub(muteLED.lastStatus) > 3*time.Second {
		muteLED.toggle, muteLED.on = false, false
		muteLED.generation++
	}
	toggle, on, generation := muteLED.toggle, muteLED.on, muteLED.generation
	muteLED.Unlock()
	apply = generation != lastGeneration || (toggle && (lastSent.IsZero() || now.Sub(lastSent) >= time.Second))
	return
}

func applyMicMuteLEDReport(common []byte, apply, on bool) {
	if !apply || len(common) < 9 {
		return
	}
	common[1] |= 0x01 // MIC_MUTE_LED_CONTROL_ENABLE only
	if on {
		common[8] = 1
	} else {
		common[8] = 0
	}
}
