package main

import (
	"encoding/binary"
	"math"
)

type triggerSignature struct {
	L2Kind, R2Kind                                triggerEffectKind
	L2Position, L2Start, L2End, L2Amplitude, L2Hz int
	R2Position, R2Start, R2End, R2Amplitude, R2Hz int
}

func signatureForTriggers(t telemetry) triggerSignature {
	pair := triggerPairFromTelemetry(t)
	qPos := func(v unitValue) int { return int(math.Round(v.Float64() * 1_000_000)) }
	qForce := func(v triggerForce) int { return v.Level() }
	return triggerSignature{
		L2Kind: pair.L2.Kind, R2Kind: pair.R2.Kind,
		L2Position: qPos(pair.L2.StartPosition), L2Start: qForce(pair.L2.StartForce), L2End: qForce(pair.L2.EndForce),
		L2Amplitude: qForce(pair.L2.Amplitude), L2Hz: int(math.Round(pair.L2.FrequencyHz * 1000)),
		R2Position: qPos(pair.R2.StartPosition), R2Start: qForce(pair.R2.StartForce), R2End: qForce(pair.R2.EndForce),
		R2Amplitude: qForce(pair.R2.Amplitude), R2Hz: int(math.Round(pair.R2.FrequencyHz * 1000)),
	}
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// DualSense trigger encoding lives exclusively in this hardware adapter. The
// rest of the Bridge works with normalized triggerEffect values.
func clearTrigger(dst []byte) {
	for i := range dst {
		dst[i] = 0
	}
	if len(dst) > 0 {
		dst[0] = 0x05
	}
}

func fillOfficialResistance(dst []byte, effect triggerEffect) {
	for i := range dst {
		dst[i] = 0
	}
	if len(dst) < 11 || effect.StartForce <= 0 || effect.EndForce <= 0 {
		clearTrigger(dst)
		return
	}
	startZone := clampInt(int(math.Round(effect.StartPosition.Float64()*9.0)), 0, 9)
	var active uint16
	var packed uint32
	span := 9 - startZone
	for zone := startZone; zone < 10; zone++ {
		force := effect.EndForce
		if span > 0 {
			x := float64(zone-startZone) / float64(span)
			force = force48(effect.StartForce.Float64() + (effect.EndForce.Float64()-effect.StartForce.Float64())*x)
		}
		strength := force.officialStep()
		if strength <= 0 {
			continue
		}
		active |= uint16(1 << zone)
		packed |= uint32((strength-1)&7) << (3 * zone)
	}
	if active == 0 {
		clearTrigger(dst)
		return
	}
	dst[0] = 0x21
	dst[1], dst[2] = byte(active), byte(active>>8)
	dst[3], dst[4], dst[5], dst[6] = byte(packed), byte(packed>>8), byte(packed>>16), byte(packed>>24)
}

func fillOfficialVibrationEffect(dst []byte, effect triggerEffect) {
	for i := range dst {
		dst[i] = 0
	}
	if len(dst) < 11 || effect.Amplitude <= 0 || effect.FrequencyHz <= 0 {
		clearTrigger(dst)
		return
	}
	startZone := clampInt(int(math.Round(effect.StartPosition.Float64()*9.0)), 0, 9)
	strength := effect.Amplitude.officialStep()
	if strength <= 0 {
		clearTrigger(dst)
		return
	}
	var active uint16
	var packed uint32
	value := uint32((strength - 1) & 7)
	for zone := startZone; zone < 10; zone++ {
		active |= uint16(1 << zone)
		packed |= value << (3 * zone)
	}
	dst[0] = 0x26
	dst[1], dst[2] = byte(active), byte(active>>8)
	dst[3], dst[4], dst[5], dst[6] = byte(packed), byte(packed>>8), byte(packed>>16), byte(packed>>24)
	dst[9] = byte(clampInt(int(math.Round(effect.FrequencyHz)), 0, 255))
}

func fillFineFeedbackEffect(dst []byte, effect triggerEffect) {
	for i := range dst {
		dst[i] = 0
	}
	if len(dst) < 11 || effect.StartForce <= 0 {
		clearTrigger(dst)
		return
	}
	dst[0] = 0x01
	dst[1] = byte(effect.StartPosition.positionByte())
	dst[2] = byte(effect.StartForce.Level())
}

func fillTriggerEffect(dst []byte, effect triggerEffect) {
	switch effect.Kind {
	case triggerResistance:
		fillOfficialResistance(dst, effect)
	case triggerVibration:
		fillOfficialVibrationEffect(dst, effect)
	case triggerFine:
		fillFineFeedbackEffect(dst, effect)
	default:
		clearTrigger(dst)
	}
}

// fillUSBTriggerStateCommon builds the 47-byte DualSense SetStateData body
// used by USB. The Bridge owns adaptive triggers only; BeamNG Device.setRGB()
// is the sole lightbar writer, so valid_flag1 and RGB bytes stay clear.
func fillUSBTriggerStateCommon(common []byte, t telemetry) {
	if len(common) < 47 {
		return
	}
	clear(common[:47])
	common[0] = 0x0C // R2 + L2 trigger enables
	pair := triggerPairFromTelemetry(t)
	fillTriggerEffect(common[10:21], pair.R2)
	fillTriggerEffect(common[21:32], pair.L2)
}

// fillBluetoothSetStateData63 builds the SetStateData block embedded in the
// validated Bluetooth 0x36 all-in-one transport. The block remains present for
// audio/trigger state, but every secondary LED-valid field stays clear so
// BeamNG Device.setRGB() remains the only lightbar writer.
func fillBluetoothSetStateData63(state []byte, t telemetry) {
	if len(state) < 63 {
		return
	}
	clear(state[:63])
	state[0] = 0xFD
	state[1] = 0x00 // no lightbar/player/power-save ownership
	state[4], state[5], state[6] = 100, 100, 0x40
	state[7] = 0x09
	pair := triggerPairFromTelemetry(t)
	fillTriggerEffect(state[10:21], pair.R2)
	fillTriggerEffect(state[21:32], pair.L2)
	state[36], state[37] = 0x00, 0x01
	state[42], state[43] = 0x01, 0x00
	state[44], state[45], state[46] = 0, 0, 0
}

func buildBluetoothSetStateData63(t telemetry) []byte {
	state := make([]byte, 63)
	fillBluetoothSetStateData63(state, t)
	return state
}

func buildBluetoothControlBase(sequence, enable1, enable2 byte) []byte {
	report := make([]byte, bluetoothControlReportSize)
	report[0] = 0x31
	// Standard DualSense Bluetooth output header (SDL + hid-playstation):
	// report id, seq_tag, tag=0x10, then the 47-byte common effect state.
	report[1] = (sequence & 0x0F) << 4
	report[2] = 0x10
	common := report[3:50]
	common[0] = enable1
	common[1] = enable2
	common[38] = 0
	return report
}

func finalizeBluetoothControlReport(report []byte) []byte {
	binary.LittleEndian.PutUint32(report[74:78], sonyBluetoothCRC(report, bluetoothControlReportSize))
	return report
}

func buildBluetoothInitializationReport(enable1, enable2, _ byte) []byte {
	return finalizeBluetoothControlReport(buildBluetoothControlBase(0, enable1, enable2))
}

func buildBluetoothTriggerControlReport(sequence byte, t telemetry, _ int) []byte {
	// Diagnostic 0x31 path: update adaptive triggers only. Compatible rumble,
	// audio routing and all LED-valid fields remain untouched.
	report := buildBluetoothControlBase(sequence, 0x0C, 0x00)
	common := report[3:50]
	pair := triggerPairFromTelemetry(t)
	fillTriggerEffect(common[10:21], pair.R2)
	fillTriggerEffect(common[21:32], pair.L2)
	return finalizeBluetoothControlReport(report)
}
