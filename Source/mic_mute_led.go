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
	toggle              bool
	on                  bool
	generation          uint64
	lastStatus          time.Time
	focusKnown          bool
	gameFocused         bool
	processSeen         bool
	processAlive        bool
	processForeground   bool
	processChecked      time.Time
	focusLossGeneration byte
}

// The Bridge mirrors BeamNG's DMB status; it must never predict a LED toggle
// from the physical button before the game action has changed.
func updateMicMuteLED(toggle, on bool) {
	updateMicMuteLEDState(toggle, on, false, false)
}

// DMB2 adds program focus without changing the LED or the physical Mic audio.
func updateMicMuteLEDWithFocus(toggle, on, focused bool) {
	updateMicMuteLEDState(toggle, on, true, focused)
}

func updateMicMuteLEDState(toggle, on, focusKnown, focused bool) {
	if !toggle {
		on = false
	}
	muteLED.Lock()
	defer muteLED.Unlock()
	now := time.Now()
	muteLED.lastStatus = now
	if focusKnown {
		muteLED.focusKnown = true
		muteLED.gameFocused = focused
	}
	// A stale status report must not illuminate the LED while BeamNG is in the background.
	if toggle && ((focusKnown && !focused) || (muteLED.processSeen && muteLED.processAlive && !muteLED.processForeground)) {
		on = false
	}
	if !toggle {
		muteLED.focusKnown = false
		muteLED.processSeen = false
		muteLED.processChecked = time.Time{}
	}
	if muteLED.toggle != toggle || muteLED.on != on {
		muteLED.toggle = toggle
		muteLED.on = on
		muteLED.generation++
	}
}

// A low-frequency Windows process scan runs outside the HID output loop.
// If BeamNG exits while unfocused, release the otherwise preserved LED.
// The OS samples focus independently of BeamNG's Lua update frequency. A two-
// bit loss epoch travels with every DSE4 input frame, so a frozen Lua VM still
// notices an intervening Alt+Tab when it resumes. No virtual press is invented.
func noteBeamNGProcessFocusSample(alive, foreground bool, at time.Time) {
	muteLED.Lock()
	defer muteLED.Unlock()
	wasForeground := muteLED.processSeen && muteLED.processAlive && muteLED.processForeground
	if wasForeground && (!alive || !foreground) {
		muteLED.focusLossGeneration = (muteLED.focusLossGeneration + 1) & 0x03
	}
	muteLED.processSeen = true
	muteLED.processAlive = alive
	muteLED.processForeground = foreground
	muteLED.processChecked = at
	if muteLED.toggle && (!alive || !foreground) && muteLED.on {
		muteLED.on = false
		muteLED.generation++
	}
}

func micMuteFocusLossGeneration() byte {
	muteLED.RLock()
	generation := muteLED.focusLossGeneration
	muteLED.RUnlock()
	return generation
}

// Recognize legacy and focus-aware status packets at one validated boundary.
func handleMicMuteStatusPacket(packet []byte) bool {
	if len(packet) == 7 && string(packet[:4]) == "DMB1" && packet[4] == 1 && packet[5] <= 1 && packet[6] <= 1 {
		updateMicMuteLED(packet[5] == 1, packet[6] == 1)
		return true
	}
	if len(packet) == 8 && string(packet[:4]) == "DMB2" && packet[4] == 1 && packet[5] <= 1 && packet[6] <= 1 && packet[7] <= 1 {
		updateMicMuteLEDWithFocus(packet[5] == 1, packet[6] == 1, packet[7] == 1)
		return true
	}
	return false
}

// Send on change; refresh at most once a second while toggle mode is active.
// The background exception requires the OS to confirm a running, unfocused
// BeamNG process, or a short fallback for an explicit Lua focus-loss report.
func micMuteLEDUpdateDue(lastGeneration uint64, lastSent time.Time, now time.Time) (apply, on bool, generation uint64) {
	muteLED.Lock()
	if muteLED.toggle && !muteLED.lastStatus.IsZero() && now.Sub(muteLED.lastStatus) > 3*time.Second {
		backgroundReported := muteLED.focusKnown && !muteLED.gameFocused
		processRecent := muteLED.processSeen && now.Sub(muteLED.processChecked) <= 5*time.Second
		// A Windows foreground check also covers games which suspend Lua before
		// it can send the first "unfocused" packet. A dead or foreground game
		// with a silent status stream releases LED ownership.
		backgroundObserved := processRecent && muteLED.processAlive && !muteLED.processForeground
		// If the OS process snapshot cannot be sampled, keep an explicitly
		// reported background latch for at most 30 s, never indefinitely.
		waitingForWatcher := backgroundReported && !processRecent && now.Sub(muteLED.lastStatus) <= 30*time.Second
		keep := backgroundObserved || waitingForWatcher
		if !keep {
			muteLED.toggle, muteLED.on = false, false
			muteLED.focusKnown = false
			muteLED.generation++
		}
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
