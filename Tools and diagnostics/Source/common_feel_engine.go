package main

import "time"

// sharedFeelFrame is the transport-independent output of the Common Feel Engine.
// A USB adapter consumes Control/RGB plus stereo Samples through HID + WASAPI;
// the Bluetooth adapter packs the same frame into the all-in-one 0x36 report.
type sharedFeelFrame struct {
	Control   telemetry
	RGB       [3]byte
	LEDStatus string
	Samples   []int8
	Status    mixerStatus
}

type sharedFeelEngine struct {
	mixer *hapticMixer
	led   *lightbarController

	absState absPulseState
	r2State  adaptiveThrottleTriggerState

	shiftTorqueCutUntil time.Time
	lastShiftEvent      int
	eventsSynced        bool

	// Reused canonical PCM buffers. Tests can still call hapticMixer.render when
	// they need independent retained blocks; the live engine is allocation-free.
	pcmScratch        []int8
	surfacePCMScratch []int8
}

func newSharedFeelEngine(m *hapticMixer) *sharedFeelEngine {
	if m == nil {
		m = newHapticMixer()
	}
	return &sharedFeelEngine{mixer: m, led: &lightbarController{}}
}

func (e *sharedFeelEngine) step(latest telemetry, lastPacket, now time.Time, frames int) sharedFeelFrame {
	fresh := latest.Active && !lastPacket.IsZero() && now.Sub(lastPacket) <= 1200*time.Millisecond
	control := latest
	if !fresh {
		control = telemetry{Version: protocolVersion, Active: false}
	}

	if control.Active {
		if !e.eventsSynced {
			e.lastShiftEvent = control.ShiftEvent
			e.eventsSynced = true
		} else if control.ShiftEvent > 0 && control.ShiftEvent != e.lastShiftEvent {
			e.lastShiftEvent = control.ShiftEvent
			e.shiftTorqueCutUntil = now.Add(time.Duration(feelProfile().Triggers.R2.ShiftDurationMS) * time.Millisecond)
		}
	} else {
		e.eventsSynced = false
		e.shiftTorqueCutUntil = time.Time{}
	}

	pair := triggerPairFromTelemetry(control)
	var absActive bool
	var absPhase string
	pair, absActive, absPhase, _, _, _, _ = applyABSHybridPulsePair(control, pair, now, &e.absState)
	shiftCut := !e.shiftTorqueCutUntil.IsZero() && now.Before(e.shiftTorqueCutUntil)
	pair, _, _, _, _ = applyAdaptiveThrottleTriggerPair(control, pair, now, &e.r2State, shiftCut)
	pair = applyUserTriggerPreferencesPair(control, pair, absActive)
	writeTriggerState(&control, pair)
	e.mixer.setSharedDynamics(absActive && absPhase == "kick")

	rgb := applyUserLEDSettings(e.led.update(control, now))
	required := maxInt(frames, 0) * 2
	if cap(e.pcmScratch) < required {
		e.pcmScratch = make([]int8, required)
	} else {
		e.pcmScratch = e.pcmScratch[:required]
	}
	diagnostics := runtimeDiagnosticsEnabled()
	var surfaceScratch []int8
	if diagnostics {
		if cap(e.surfacePCMScratch) < required {
			e.surfacePCMScratch = make([]int8, required)
		} else {
			e.surfacePCMScratch = e.surfacePCMScratch[:required]
		}
		surfaceScratch = e.surfacePCMScratch
	}
	samples, status := e.mixer.renderInto(frames, now, e.pcmScratch, surfaceScratch, diagnostics)
	return sharedFeelFrame{Control: control, RGB: rgb, LEDStatus: e.led.status(), Samples: samples, Status: status}
}
