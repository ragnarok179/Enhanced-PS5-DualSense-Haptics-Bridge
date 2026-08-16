package main

import "math"

// unitValue is used for normalized non-force trigger quantities such as travel
// position. Trigger force has its own canonical 48-step type below.
type unitValue float64

func unit(v float64) unitValue            { return unitValue(clamp(v, 0, 1)) }
func unitFromPercent(v float64) unitValue { return unit(v / 100.0) }
func unitFromPositionByte(v int) unitValue {
	return unit(float64(clampInt(v, 0, 255)) / 255.0)
}
func (u unitValue) Float64() float64 { return float64(unit(float64(u))) }
func (u unitValue) positionByte() int {
	return clampInt(int(math.Round(u.Float64()*255.0)), 0, 255)
}

// triggerForce is the single canonical trigger-force lattice used throughout
// the Bridge. Fine Feedback is the highest-resolution force mode currently
// used by the DualSense backend, so every gameplay force/amplitude is reduced
// once to 0..48. Official Feedback/Vibration are derived from this value only
// inside the final HID adapter.
const triggerForceMax = 48

type triggerForce uint8

func force48(v float64) triggerForce {
	v = clamp(v, 0, 1)
	if v <= 0 {
		return 0
	}
	level := int(math.Round(v * triggerForceMax))
	if level < 1 {
		level = 1
	}
	return triggerForce(clampInt(level, 1, triggerForceMax))
}

func force48FromLevel(v int) triggerForce {
	return triggerForce(clampInt(v, 0, triggerForceMax))
}

func force48FromOfficialStep(v int) triggerForce {
	step := clampInt(v, 0, 8)
	return force48FromLevel(step * (triggerForceMax / 8))
}

func (f triggerForce) Level() int { return clampInt(int(f), 0, triggerForceMax) }
func (f triggerForce) Float64() float64 {
	return float64(f.Level()) / float64(triggerForceMax)
}

// officialStep is deliberately private to the hardware boundary. The
// DualSense Official Feedback/Vibration formats expose eight force levels.
func (f triggerForce) officialStep() int {
	if f.Level() <= 0 {
		return 0
	}
	step := int(math.Round(float64(f.Level()) * 8.0 / float64(triggerForceMax)))
	return clampInt(step, 1, 8)
}

type triggerEffectKind string

const (
	triggerOff        triggerEffectKind = "off"
	triggerResistance triggerEffectKind = "resistance"
	triggerVibration  triggerEffectKind = "vibration"
	triggerFine       triggerEffectKind = "fine"
)

// triggerEffect is transport-neutral. Force/amplitude always use the common
// 0..48 triggerForce lattice. Position remains normalized and frequency stays
// in Hz because neither is a force quantity.
type triggerEffect struct {
	Kind          triggerEffectKind
	StartPosition unitValue
	StartForce    triggerForce
	EndForce      triggerForce
	Amplitude     triggerForce
	FrequencyHz   float64
}

type triggerPair struct {
	L2 triggerEffect
	R2 triggerEffect
}

// On the BeamNG wire, forces remain normalized 0..1 so the protocol stays
// semantic and transport-neutral. The Bridge quantizes them immediately to the
// canonical 0..48 force lattice when decoding.
type wireTriggerEffect struct {
	Kind          string  `json:"kind"`
	StartPosition float64 `json:"startPosition"`
	StartForce    float64 `json:"startForce"`
	EndForce      float64 `json:"endForce"`
	Amplitude     float64 `json:"amplitude"`
	FrequencyHz   float64 `json:"frequencyHz"`
}

func offTrigger() triggerEffect { return triggerEffect{Kind: triggerOff} }

func resistanceTrigger(position, start, end float64) triggerEffect {
	return triggerEffect{Kind: triggerResistance, StartPosition: unit(position), StartForce: force48(start), EndForce: force48(end)}
}

func vibrationTrigger(position, amplitude, frequencyHz float64) triggerEffect {
	return triggerEffect{Kind: triggerVibration, StartPosition: unit(position), Amplitude: force48(amplitude), FrequencyHz: math.Max(0, frequencyHz)}
}

func fineTrigger(position, force float64) triggerEffect {
	f := force48(force)
	return triggerEffect{Kind: triggerFine, StartPosition: unit(position), StartForce: f, EndForce: f}
}

func wireEffect(e triggerEffect) *wireTriggerEffect {
	return &wireTriggerEffect{
		Kind: string(e.Kind), StartPosition: e.StartPosition.Float64(), StartForce: e.StartForce.Float64(),
		EndForce: e.EndForce.Float64(), Amplitude: e.Amplitude.Float64(), FrequencyHz: e.FrequencyHz,
	}
}

func effectFromWire(w *wireTriggerEffect) (triggerEffect, bool) {
	if w == nil {
		return triggerEffect{}, false
	}
	switch triggerEffectKind(w.Kind) {
	case triggerOff:
		return offTrigger(), true
	case triggerResistance:
		return resistanceTrigger(w.StartPosition, w.StartForce, w.EndForce), true
	case triggerVibration:
		return vibrationTrigger(w.StartPosition, w.Amplitude, w.FrequencyHz), true
	case triggerFine:
		return fineTrigger(w.StartPosition, w.StartForce), true
	default:
		return triggerEffect{}, false
	}
}

func triggerPairFromTelemetry(t telemetry) triggerPair {
	l2, okL := effectFromWire(t.L2Effect)
	r2, okR := effectFromWire(t.R2Effect)

	// Historical integer trigger fields belong exclusively to protocol v40.
	// Protocol v41 never falls back to them: current packets must carry the
	// semantic normalized effect objects, which are quantized once to force48.
	if t.Version == legacyProtocolVersion || t.Version == 0 {
		if !okL {
			l2 = legacyV40Effect(t.L2Mode, t.L2StartZone, t.L2StartStrength, t.L2EndStrength, t.L2Amplitude, t.L2Hz)
		}
		if !okR {
			r2 = legacyV40Effect(t.R2Mode, t.R2StartZone, t.R2StartStrength, t.R2EndStrength, t.R2Amplitude, t.R2Hz)
		}
	}
	if !okL && t.Version == protocolVersion {
		l2 = offTrigger()
	}
	if !okR && t.Version == protocolVersion {
		r2 = offTrigger()
	}
	return triggerPair{L2: l2, R2: r2}
}

// writeTriggerState stores normalized wire effects plus a compact diagnostic
// mirror. All force fields in that mirror are 0..48; no active trigger-force
// path uses a 0..255 scale anymore. Position/frequency retain their real HID
// units where useful for diagnostics.
func writeTriggerState(t *telemetry, pair triggerPair) {
	if t == nil {
		return
	}
	write := func(e triggerEffect) (mode, position, start, end, amp, hz int) {
		switch e.Kind {
		case triggerResistance:
			return 1, clampInt(int(math.Round(e.StartPosition.Float64()*9)), 0, 9), e.StartForce.Level(), e.EndForce.Level(), 0, 0
		case triggerVibration:
			return 2, clampInt(int(math.Round(e.StartPosition.Float64()*9)), 0, 9), 0, 0, e.Amplitude.Level(), clampInt(int(math.Round(e.FrequencyHz)), 0, 255)
		case triggerFine:
			force := e.StartForce.Level()
			return 3, e.StartPosition.positionByte(), force, force, 0, 0
		default:
			return 0, 0, 0, 0, 0, 0
		}
	}
	t.L2Mode, t.L2StartZone, t.L2StartStrength, t.L2EndStrength, t.L2Amplitude, t.L2Hz = write(pair.L2)
	t.R2Mode, t.R2StartZone, t.R2StartStrength, t.R2EndStrength, t.R2Amplitude, t.R2Hz = write(pair.R2)
	t.L2Effect, t.R2Effect = wireEffect(pair.L2), wireEffect(pair.R2)
}
