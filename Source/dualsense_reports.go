package main

import (
	"encoding/binary"
	"math"
	"time"
)

type triggerSignature struct {
	L2Mode, L2Zone, L2Start, L2End, L2Amp, L2Hz int
	R2Mode, R2Zone, R2Start, R2End, R2Amp, R2Hz int
}

func signatureForTriggers(t telemetry) triggerSignature {
	return triggerSignature{
		t.L2Mode, t.L2StartZone, t.L2StartStrength, t.L2EndStrength, t.L2Amplitude, t.L2Hz,
		t.R2Mode, t.R2StartZone, t.R2StartStrength, t.R2EndStrength, t.R2Amplitude, t.R2Hz,
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

// Internal trigger strength is normalized to 0..255. The DualSense official
// feedback/vibration effects still expose eight hardware strength steps per
// trigger zone, so quantization happens only here at the final HID boundary.
// Keeping the rest of the bridge in 0..255 gives user settings and effect math
// enough headroom without pretending that the hardware has 255 resistance steps.
func strength8To255(v int) int {
	if v <= 0 {
		return 0
	}
	return clampInt(int(math.Round(float64(clampInt(v, 0, 8))*255.0/8.0)), 0, 255)
}

func strength255To8(v int) int {
	if v <= 0 {
		return 0
	}
	q := int(math.Round(float64(clampInt(v, 0, 255)) * 8.0 / 255.0))
	return clampInt(q, 1, 8)
}

func fillOfficialFeedback(dst []byte, startZone, startStrength, endStrength int) {
	for i := range dst {
		dst[i] = 0
	}
	if len(dst) < 11 {
		return
	}
	startZone = clampInt(startZone, 0, 9)
	startStrength = clampInt(startStrength, 0, 255)
	endStrength = clampInt(endStrength, 0, 255)
	if startStrength == 0 || endStrength == 0 {
		dst[0] = 0x05
		return
	}
	var active uint16
	var packed uint32
	span := 9 - startZone
	for zone := startZone; zone < 10; zone++ {
		normalized := endStrength
		if span > 0 {
			x := float64(zone-startZone) / float64(span)
			normalized = int(math.Round(float64(startStrength) + float64(endStrength-startStrength)*x))
		}
		strength := strength255To8(normalized)
		active |= uint16(1 << zone)
		packed |= uint32((strength-1)&7) << (3 * zone)
	}
	dst[0] = 0x21
	dst[1], dst[2] = byte(active), byte(active>>8)
	dst[3], dst[4], dst[5], dst[6] = byte(packed), byte(packed>>8), byte(packed>>16), byte(packed>>24)
}

func fillOfficialVibration(dst []byte, startZone, amplitude, hz int) {
	for i := range dst {
		dst[i] = 0
	}
	if len(dst) < 11 {
		return
	}
	startZone = clampInt(startZone, 0, 9)
	amplitude = clampInt(amplitude, 0, 255)
	hz = clampInt(hz, 0, 255)
	if amplitude == 0 || hz == 0 {
		dst[0] = 0x05
		return
	}
	var active uint16
	var packed uint32
	strength := strength255To8(amplitude)
	value := uint32((strength - 1) & 7)
	for zone := startZone; zone < 10; zone++ {
		active |= uint16(1 << zone)
		packed |= value << (3 * zone)
	}
	dst[0] = 0x26
	dst[1], dst[2] = byte(active), byte(active>>8)
	dst[3], dst[4], dst[5], dst[6] = byte(packed), byte(packed>>8), byte(packed>>16), byte(packed>>24)
	dst[9] = byte(hz)
}

func fillFineFeedback(dst []byte, position, strength int) {
	for i := range dst {
		dst[i] = 0
	}
	if len(dst) < 11 {
		return
	}
	position = clampInt(position, 0, 255)
	strength = clampInt(strength, 0, 48)
	if strength == 0 {
		dst[0] = 0x05
		return
	}
	dst[0], dst[1], dst[2] = 0x01, byte(position), byte(strength)
}

func fillTrigger(dst []byte, mode, startZone, startStrength, endStrength, amplitude, hz int) {
	switch mode {
	case 1:
		fillOfficialFeedback(dst, startZone, startStrength, endStrength)
	case 2:
		fillOfficialVibration(dst, startZone, amplitude, hz)
	case 3:
		fillFineFeedback(dst, startZone, startStrength)
	default:
		for i := range dst {
			dst[i] = 0
		}
		if len(dst) > 0 {
			dst[0] = 0x05
		}
	}
}

func rgbDistance(a, b [3]byte) int {
	distance := 0
	for i := 0; i < 3; i++ {
		delta := int(a[i]) - int(b[i])
		if delta < 0 {
			delta = -delta
		}
		distance += delta
	}
	return distance
}

type lightbarController struct {
	stage         int // 0=off, 1=progressive, 2=solid red
	lastColor     [3]byte
	enabled       bool
	rpmActive     bool
	belowOffSince time.Time
	limiterUntil  time.Time
	blinkActive   bool
}

func quantizeLED(v float64) byte {
	v = clamp(v, 0, 255)
	q := int(math.Round(v/8.0)) * 8
	if q > 255 {
		q = 255
	}
	return byte(q)
}

func steadyRPMRGB(ratio float64) [3]byte {
	cfg := feelProfile().LED
	if ratio >= cfg.RedRatio {
		return [3]byte{byte(clampInt(cfg.MaxBrightness, 0, 255)), 0, 0}
	}
	x := clamp01((ratio - cfg.FirstRatio) / math.Max(0.001, cfg.RedRatio-cfg.FirstRatio))
	hue := 120 * (1 - x)
	brightness := float64(cfg.MinBrightness) + float64(cfg.MaxBrightness-cfg.MinBrightness)*clamp01(x*1.8)
	r, g, b := hsvToRGB(hue, 1.0, brightness/255.0)
	return [3]byte{quantizeLED(float64(r)), quantizeLED(float64(g)), quantizeLED(float64(b))}
}

func (c *lightbarController) update(t telemetry, now time.Time) [3]byte {
	var black [3]byte
	engineActive := t.Active && t.Raw != nil && t.Raw.EngineRunning && t.Raw.MaxRPM > 1
	if !engineActive {
		c.stage = 0
		c.rpmActive = false
		c.belowOffSince = time.Time{}
		c.enabled = false
		c.limiterUntil = time.Time{}
		c.blinkActive = false
		c.lastColor = black
		return black
	}

	if t.ShiftLEDsInUse && !c.enabled {
		c.enabled = true
	}
	if !c.enabled {
		c.blinkActive = false
		c.lastColor = black
		return black
	}

	cfg := feelProfile().LED
	ratio := clamp01(t.Raw.RPM / math.Max(t.Raw.MaxRPM, 1))
	if !c.rpmActive {
		if ratio >= cfg.FirstRatio {
			c.rpmActive = true
			c.stage = 1
			c.belowOffSince = time.Time{}
		} else {
			c.blinkActive = false
			c.lastColor = black
			return black
		}
	} else if ratio < cfg.OffRatio {
		if c.belowOffSince.IsZero() {
			c.belowOffSince = now
		}
		if now.Sub(c.belowOffSince) >= time.Duration(cfg.OffHoldMS)*time.Millisecond {
			c.rpmActive = false
			c.stage = 0
			c.blinkActive = false
			c.lastColor = black
			return black
		}
	} else {
		c.belowOffSince = time.Time{}
	}

	if c.stage == 2 {
		if ratio < cfg.RedExitRatio {
			c.stage = 1
		}
	} else if ratio >= cfg.RedRatio {
		c.stage = 2
	}

	if t.Raw.RevLimiter && ratio >= cfg.BlinkMinRatio {
		c.limiterUntil = now.Add(time.Duration(cfg.BlinkHoldMS) * time.Millisecond)
	}
	blink := !c.limiterUntil.IsZero() && now.Before(c.limiterUntil) && ratio >= cfg.RedExitRatio
	if !cfg.BlinkOnlyOnRevLimiter && ratio >= 0.985 {
		blink = true
	}
	if blink {
		c.blinkActive = true
		hz := math.Max(1, cfg.BlinkHz)
		on := (int(math.Floor(float64(now.UnixNano())/1e9*hz*2.0)) % 2) == 0
		if !on {
			c.lastColor = black
			return black
		}
		red := [3]byte{byte(clampInt(cfg.MaxBrightness, 0, 255)), 0, 0}
		c.lastColor = red
		return red
	}
	c.blinkActive = false

	if ratio < cfg.FirstRatio && c.lastColor != black {
		return c.lastColor
	}
	rgb := steadyRPMRGB(math.Max(ratio, cfg.FirstRatio))
	c.lastColor = rgb
	return rgb
}

func (c *lightbarController) isBlinking() bool {
	return c.blinkActive
}

func (c *lightbarController) status() string {
	if c.blinkActive {
		return "blink"
	}
	if !c.rpmActive {
		return "off"
	}
	if c.stage >= 2 {
		return "red"
	}
	return "progress"
}

func bluetoothLightbarUpdateDue(ledSynced bool, lastRGB, rgb [3]byte, lastWrite, now time.Time, blinking bool) bool {
	if ledSynced && rgbDistance(rgb, lastRGB) < 8 {
		return false
	}
	interval := 120 * time.Millisecond
	if blinking {
		interval = 50 * time.Millisecond
	}
	return lastWrite.IsZero() || now.Sub(lastWrite) >= interval
}

// buildBluetoothSetStateData63 returns the payload used by DS5Dongle's
// SetStateData sized packet. It is 63 bytes long, but only bytes 0..46 are the
// documented SetStateData structure; bytes 47..62 are zero padding. Do NOT put
// USB report ID 0x02 at byte 0 here: Bluetooth SetStateData begins at offset 0.
func buildBluetoothSetStateData63WithFlags(t telemetry, rgb [3]byte, updateLightbar, releaseLEDs bool) []byte {
	state := make([]byte, 63)
	// DS5Dongle's 0x36 audio-haptic transport requires the SetStateData block
	// to remain present, so keep the validated audio/trigger state in every
	// frame. valid_flag1, however, must remain transactional; an older implementation used 0xF3,
	// which revalidated microphone LED, power-save, player indicators and
	// audio-control2 roughly 94 times/s. SDL and hid-playstation only validate
	// LED-related fields when those fields actually change.
	state[0] = 0xFD
	state[1] = 0x80 // AUDIO_CONTROL2 only; no steady LED/player/power-save ownership
	if updateLightbar {
		state[1] |= 0x04 // LIGHTBAR_CONTROL_ENABLE, one frame only
	}
	if releaseLEDs {
		state[1] |= 0x08 // RELEASE_LEDS, one frame only
	}
	state[2], state[3] = 0, 0
	state[4], state[5], state[6] = 100, byte(math.Round(100*bluetoothSpeakerHardwareOutputGain)), 0x40
	state[7], state[8], state[9] = 0x09, 0x00, 0x00
	fillTrigger(state[10:21], t.R2Mode, t.R2StartZone, t.R2StartStrength, t.R2EndStrength, t.R2Amplitude, t.R2Hz)
	fillTrigger(state[21:32], t.L2Mode, t.L2StartZone, t.L2StartStrength, t.L2EndStrength, t.L2Amplitude, t.L2Hz)
	state[36], state[37] = 0x00, 0x01
	state[38], state[39], state[40] = 0, 0, 0
	// Do not combine RELEASE_LEDS with LIGHTBAR_SETUP/LIGHT_OUT. SDL uses the
	// release bit by itself after the Bluetooth connection animation; Linux's
	// LIGHT_OUT setup is a separate initialization mechanism. Combining both
	// can deliberately fade the bar while we are trying to hand it to RGB.
	state[41] = 0x00
	state[42], state[43] = 0x01, 0x00
	state[44], state[45], state[46] = rgb[0], rgb[1], rgb[2]
	return state
}

func buildBluetoothSetStateData63(t telemetry, rgb [3]byte) []byte {
	return buildBluetoothSetStateData63WithFlags(t, rgb, true, false)
}

// When RGB is owned by BeamNG Device.setRGB(), the continuous 0x36 stream must
// not validate any secondary-output group that could cause the controller to
// re-apply a stale lightbar state. Keep the trigger/audio primary state, but
// clear valid_flag1 and RGB bytes completely. This is intentionally stricter
// than merely clearing LIGHTBAR_CONTROL_ENABLE.
func buildBluetoothSetStateData63ExternalRGB(t telemetry) []byte {
	state := buildBluetoothSetStateData63WithFlags(t, [3]byte{}, false, false)
	state[1] = 0x00
	state[44], state[45], state[46] = 0, 0, 0
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

func buildBluetoothControlReportMasked(sequence byte, t telemetry, rgb [3]byte, _ int, updateTriggers, updateLED bool) []byte {
	// Only mark fields that this packet intentionally changes. Bits 0/1 remain
	// clear so compatible rumble is never selected and audio haptics stay active.
	// 0x04/0x08 are the documented R2/L2 trigger enables;
	// valid_flag1 bit 0x04 is the documented lightbar enable.
	var enable1, enable2 byte
	if updateTriggers {
		enable1 = 0x0C
	}
	if updateLED {
		enable2 = 0x04
	}
	report := buildBluetoothControlBase(sequence, enable1, enable2)
	common := report[3:50]
	if updateTriggers {
		fillTrigger(common[10:21], t.R2Mode, t.R2StartZone, t.R2StartStrength, t.R2EndStrength, t.R2Amplitude, t.R2Hz)
		fillTrigger(common[21:32], t.L2Mode, t.L2StartZone, t.L2StartStrength, t.L2EndStrength, t.L2Amplitude, t.L2Hz)
	}
	if updateLED {
		common[44], common[45], common[46] = rgb[0], rgb[1], rgb[2]
	}
	return finalizeBluetoothControlReport(report)
}

const bluetoothSpeakerHardwareOutputGain = 0.80

// buildBluetoothSpeakerSetupReport enables the internal membrane speaker without
// selecting compatible rumble or microphone duplex. User volume is already part
// of the BeamNG-derived PCM; the 0.80 factor below is transport calibration only.
func buildBluetoothSpeakerSetupReport(sequence byte, volume int, enabled bool) []byte {
	_ = volume
	var enable1, enable2 byte
	if enabled {
		enable1 = 0xA0 // AUDIO_CONTROL_ENABLE | SPEAKER_VOLUME_ENABLE
		enable2 = 0x80 // AUDIO_CONTROL2_ENABLE
	}
	report := buildBluetoothControlBase(sequence, enable1, enable2)
	common := report[3:50]
	if enabled {
		common[5] = byte(math.Round(100 * bluetoothSpeakerHardwareOutputGain)) // Bluetooth speaker transport calibration
		common[7] = 0x30                                                       // PATH_SPEAKER
		common[37] = 0x02                                                      // speaker preamp
	}
	return finalizeBluetoothControlReport(report)
}

func buildBluetoothControlReport(sequence byte, t telemetry, rgb [3]byte, outputLen int) []byte {
	return buildBluetoothControlReportMasked(sequence, t, rgb, outputLen, true, true)
}
