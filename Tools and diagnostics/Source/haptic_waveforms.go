package main

import "math"

// Pure waveform and profile calibration helpers. Runtime scheduling/mixing lives
// in haptic_mixer.go; no transport-specific behavior belongs here.

func deterministicNoise(state *uint32) float64 {
	*state = *state*1664525 + 1013904223
	return float64((*state>>8)&0xFFFFFF)/8388607.5 - 1.0
}

func surfaceProfileFromMaterial(material int) string {
	switch material {
	case 10:
		return "asphalt"
	case 11:
		return "asphalt_wet"
	case 12:
		return "slippery"
	case 13:
		return "rock"
	case 14:
		return "dusty_dirt"
	case 15:
		return "dirt"
	case 16:
		return "sand"
	case 17:
		return "sandy_road"
	case 18:
		return "mud"
	case 19:
		return "gravel"
	case 20:
		return "grass"
	case 21:
		return "ice"
	case 22:
		return "snow"
	case 29:
		return "rumble_strip"
	case 30:
		return "cobblestone"
	default:
		return "none"
	}
}

func tactileSurfaceStrength(profile string, raw float64) float64 {
	if raw <= 0 || profile == "" || profile == "none" {
		return 0
	}
	raw = clamp01(raw)
	minimum, maximum := 0.20, 0.42
	switch profile {
	case "asphalt":
		minimum, maximum = 0.11, 0.22
	case "asphalt_wet":
		minimum, maximum = 0.13, 0.26
	case "slippery", "ice":
		minimum, maximum = 0.10, 0.20
	case "sand":
		minimum, maximum = 0.18, 0.28
	case "mud":
		minimum, maximum = 0.20, 0.32
	case "grass", "snow":
		minimum, maximum = 0.22, 0.36
	case "dirt", "dusty_dirt", "sandy_road":
		minimum, maximum = 0.30, 0.50
	case "gravel":
		minimum, maximum = 0.38, 0.62
	case "rock":
		minimum, maximum = 0.48, 0.78
	case "cobblestone":
		minimum, maximum = 0.46, 0.74
	case "rumble_strip":
		minimum, maximum = 0.58, 0.88
	}
	// The USB voice-coil actuators require much more PCM headroom than the
	// compatibility motors. Keep material ordering, but lift active windows
	// above the threshold demonstrated by the physical controller.
	normalized := math.Min(1, raw/0.24)
	return minimum + (maximum-minimum)*math.Sqrt(normalized)
}
func surfaceWave(profile string, t, speed float64, noise *uint32) float64 {
	cadenceScale, carrierScale := lowSpeedCadenceScales(speed)
	carrierSine := func(hz float64) float64 { return math.Sin(2 * math.Pi * hz * carrierScale * t) }
	ct := t * cadenceScale
	n := deterministicNoise(noise)
	switch profile {
	case "asphalt":
		grain := 0.16 + 0.34*math.Pow(math.Max(0, n), 2)
		mod := 0.62 + 0.30*math.Sin(2*math.Pi*(7+speed*0.32)*ct)
		return mod*(0.62*carrierSine(88)+0.52*carrierSine(172)) + grain*n
	case "asphalt_wet":
		mod := 0.58 + 0.22*math.Sin(2*math.Pi*(5+speed*0.24)*ct)
		return mod*(0.48*carrierSine(64)+0.26*carrierSine(112)) + 0.12*n
	case "slippery":
		return 0.34*carrierSine(92) + 0.42*carrierSine(168) + 0.10*n
	case "ice":
		return 0.24*carrierSine(118) + 0.52*carrierSine(196) + 0.08*n
	case "sand":
		mod := 0.68 + 0.20*math.Sin(2*math.Pi*2.2*ct)
		return mod * (0.94*carrierSine(68) + 0.18*carrierSine(112) + 0.03*n)
	case "mud":
		pulse := 0.25 + 0.75*math.Pow(math.Max(0, math.Sin(2*math.Pi*3.5*ct)), 2)
		return pulse * (0.98*carrierSine(58) + 0.28*carrierSine(96))
	case "dirt", "dusty_dirt":
		step := 0.60 + 0.30*math.Sin(2*math.Pi*7.0*ct) + 0.12*n
		return step * (0.72*carrierSine(68) + 0.28*carrierSine(126))
	case "sandy_road":
		return (0.72+0.18*math.Sin(2*math.Pi*5*ct))*(0.72*carrierSine(60)+0.32*carrierSine(112)) + 0.05*n
	case "gravel":
		click := math.Pow(math.Max(0, n), 3)
		return 0.46*carrierSine(82) + 0.82*carrierSine(176) + 0.85*click
	case "grass":
		mod := 0.52 + 0.34*math.Sin(2*math.Pi*5.3*ct+0.7) + 0.10*n
		return mod * (0.42*carrierSine(62) + 0.62*carrierSine(118))
	case "snow":
		return (0.46 + 0.16*n) * (0.26*carrierSine(78) + 0.70*carrierSine(154))
	case "rock":
		gate := 0.25
		if math.Mod(ct*11, 1) < 0.18 {
			gate = 1
		}
		return gate * (0.46*carrierSine(112) + 0.86*carrierSine(228))
	case "rumble_strip":
		gate := 0.12
		if math.Mod(ct*(18+speed*1.5), 1) < 0.46 {
			gate = 1
		}
		return gate * (0.96*carrierSine(76) + 0.86*carrierSine(152))
	case "cobblestone":
		mod := 0.35 + 0.65*math.Pow(math.Max(0, math.Sin(2*math.Pi*(7+speed*0.18)*ct)), 2)
		return mod * (0.98*carrierSine(72) + 0.86*carrierSine(144))
	default:
		return 0
	}
}

func isSuspensionBumpProfile(profile string) bool {
	return profile == "suspension_bump" || profile == "suspension_secondary" || profile == "suspension_rebound"
}

func isPrimarySuspensionBumpProfile(profile string) bool {
	return profile == "suspension_bump" || profile == "suspension_secondary"
}

func profileGain(profile string) float64 {
	switch profile {
	case "collision":
		return 1.28
	case "landing":
		return 1.18
	case "suspension_bump":
		// Vehicle Lua sends calibrated dynamic primary severity.
		return 1.00
	case "suspension_secondary":
		// Rear/front axle crossing the same obstacle: real, but subordinate to
		// the primary impact so the episode remains perceptually readable. Lua
		// already applies an 0.80 severity scale, so keep bridge gain close to 1.
		return 0.94
	case "suspension_rebound":
		return 0.82
	case "shift":
		return 1.08
	case "abs_pulse":
		return 0.72
	case "rumble_strip":
		return 1.30
	case "rock", "cobblestone":
		return 1.15
	case "gravel":
		return 1.25
	case "dirt", "dusty_dirt", "sandy_road":
		return 1.05
	case "grass", "snow":
		return 0.95
	case "sand":
		return 0.90
	case "mud":
		return 1.05
	default:
		return 0.90
	}
}

func profileWave(profile string, t, duration float64, noise *uint32) float64 {
	if duration <= 0 {
		return 0
	}
	p := t / duration
	if p < 0 || p > 1 {
		return 0
	}
	attack := math.Min(1, t/0.002)
	sine := func(hz float64) float64 { return math.Sin(2 * math.Pi * hz * t) }
	n := deterministicNoise(noise)
	switch profile {
	case "collision":
		// Renderer-only severity signatures. Vehicle Lua already decides when/where
		// the collision happened and supplies its strength/duration; changing the
		// spectral envelope here cannot affect telemetry, event timing or stereo.
		// Shorter impacts read as a sharp body hit, while long severe impacts shift
		// energy downwards and add a delayed chassis compression.
		switch {
		case duration <= 0.155: // light / glancing contact
			second := 0.0
			if p > 0.22 {
				q := p - 0.22
				second = (0.22*sine(122) + 0.16*sine(238)) * math.Exp(-10.0*q)
			}
			return attack*(0.68*sine(92)+0.42*sine(184)+0.18*sine(276)+0.10*n)*math.Exp(-5.0*p) + second
		case duration <= 0.205: // medium body impact
			second := 0.0
			if p > 0.18 {
				q := p - 0.18
				second = (0.40*sine(82) + 0.24*sine(164) + 0.08*n) * math.Exp(-7.0*q)
			}
			return attack*(0.78*sine(68)+0.48*sine(136)+0.24*sine(218)+0.12*n)*math.Exp(-3.4*p) + second
		default: // severe crash / structural hit
			second := 0.0
			if p > 0.14 {
				q := p - 0.14
				second = (0.52*sine(62) + 0.30*sine(124) + 0.10*n) * math.Exp(-5.0*q)
			}
			third := 0.0
			if p > 0.36 {
				q := p - 0.36
				third = (0.22*sine(48) + 0.12*sine(96)) * math.Exp(-7.0*q)
			}
			return attack*(0.82*sine(50)+0.50*sine(100)+0.24*sine(158)+0.12*n)*math.Exp(-2.5*p) + second + third
		}
	case "landing":
		// Landings are intentionally more vertical and rounded than collisions.
		// Duration already tracks severity, so use it only to choose a tactile
		// signature; no detection or strength mapping is changed.
		switch {
		case duration <= 0.140: // light touchdown
			return attack * (0.70*sine(82) + 0.34*sine(164)) * math.Exp(-4.8*p)
		case duration <= 0.165: // normal landing / compression
			second := 0.0
			if p > 0.20 {
				second = (0.28*sine(94) + 0.14*sine(188)) * math.Exp(-6.5*(p-0.20))
			}
			return attack*(0.82*sine(64)+0.44*sine(128))*math.Exp(-3.6*p) + second
		default: // hard landing / suspension bottoming
			second := 0.0
			if p > 0.17 {
				second = (0.38*sine(76) + 0.20*sine(152)) * math.Exp(-5.5*(p-0.17))
			}
			return attack*(0.88*sine(52)+0.48*sine(104))*math.Exp(-3.0*p) + second
		}
	case "suspension_bump":
		// One tactile family, three severity signatures selected by duration.
		// All retain the validated stereo surface-carrier idea so side remains
		// easy to identify; larger impacts add progressively more low-frequency
		// weight instead of merely clipping harder.
		release := 1.0
		if p > 0.72 {
			release = math.Max(0, (1-p)/0.28)
		}
		switch {
		case duration <= 0.070: // small/sharp seam or ~5 cm bump
			carrier := 0.88*sine(118) + 0.70*sine(218) + 0.020*n
			kick := 0.14 * sine(82) * math.Exp(-11.0*p)
			return attack * release * (carrier + kick)
		case duration <= 0.094: // medium road bump
			carrier := 0.94*sine(92) + 0.78*sine(178) + 0.025*n
			kick := 0.36 * sine(64) * math.Exp(-9.0*p)
			return attack * release * (carrier + kick)
		default: // large 10-20 cm obstacle / hard suspension compression
			carrier := 0.92*sine(74) + 0.76*sine(148) + 0.030*n
			kick := 0.68 * sine(48) * math.Exp(-7.5*p)
			return attack * release * (carrier + kick)
		}
	case "suspension_secondary":
		// The other axle crossing the same obstacle. Keep the validated surface
		// carrier stereo signature, slightly shorter/brighter than the primary so
		// it reads as wheelbase consequence rather than suspension bounce.
		release := 1.0
		if p > 0.70 {
			release = math.Max(0, (1-p)/0.30)
		}
		carrier := 0.92*sine(94) + 0.74*sine(184) + 0.025*n
		kick := 0.30 * sine(66) * math.Exp(-9.5*p)
		return attack * release * (carrier + kick)
	case "suspension_rebound":
		// A rebound is a consequence, not a second obstacle. Keep it short,
		// low-frequency and deliberately weak so it reads as suspension return
		// on the owner side rather than a new impact on the opposite grip.
		return attack * (0.74*sine(50) + 0.30*sine(92)) * math.Exp(-5.8*p)
	case "shift":
		second := 0.0
		if p > 0.34 {
			second = 0.62 * math.Sin(2*math.Pi*92*(t-duration*0.34)) * math.Exp(-8.0*(p-0.34))
		}
		return attack*(0.92*sine(68)+0.62*sine(132))*math.Exp(-6.2*p) + second
	case "abs_pulse":
		return attack * (0.96*sine(62) + 0.48*sine(118)) * math.Exp(-5.0*p)
	case "rock":
		return attack * (0.48*sine(115) + 0.82*sine(235) + 0.22*n) * math.Exp(-5.0*p)
	case "dusty_dirt":
		return attack * (0.68*sine(76) + 0.30*sine(138) + 0.16*n) * math.Exp(-3.5*p)
	case "dirt":
		return attack * (0.76*sine(70) + 0.28*sine(128) + 0.14*n) * math.Exp(-3.3*p)
	case "sand":
		return attack * (0.88*sine(58) + 0.18*sine(92) + 0.08*n) * math.Exp(-2.3*p)
	case "sandy_road":
		return attack * (0.72*sine(64) + 0.34*sine(116) + 0.12*n) * math.Exp(-2.8*p)
	case "mud":
		return attack * (0.92*sine(46) + 0.30*sine(78)) * math.Exp(-2.2*p)
	case "gravel":
		return attack * (0.34*sine(105) + 0.86*sine(205) + 0.34*n) * math.Exp(-4.6*p)
	case "grass":
		return attack * (0.34*sine(66) + 0.66*sine(122) + 0.20*n) * math.Exp(-3.6*p)
	case "snow":
		return attack * (0.22*sine(82) + 0.72*sine(158) + 0.16*n) * math.Exp(-4.5*p)
	case "rumble_strip":
		gate := 1.0
		if math.Mod(t*72, 1) > 0.50 {
			gate = 0.12
		}
		return attack * gate * (0.72*sine(92) + 0.76*sine(178)) * math.Exp(-1.8*p)
	case "cobblestone":
		return attack * (0.86*sine(62) + 0.48*sine(122)) * math.Exp(-2.8*p)
	default:
		return attack * (0.62*sine(76) + 0.48*sine(148)) * math.Exp(-4*p)
	}
}
