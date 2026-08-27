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

	var absActive bool
	var absPhase string
	control, absActive, absPhase, _, _, _, _ = applyABSHybridPulse(control, now, &e.absState)
	shiftCut := !e.shiftTorqueCutUntil.IsZero() && now.Before(e.shiftTorqueCutUntil)
	control, _, _, _, _ = applyAdaptiveThrottleTrigger(control, now, &e.r2State, shiftCut)
	control = applyUserTriggerPreferences(control, absActive)
	e.mixer.setSharedDynamics(absActive && absPhase == "kick")

	rgb := e.led.update(control, now)
	samples, status := e.mixer.render(frames, now)
	applyUserHapticMaster(samples)
	return sharedFeelFrame{Control: control, RGB: rgb, LEDStatus: e.led.status(), Samples: samples, Status: status}
}
