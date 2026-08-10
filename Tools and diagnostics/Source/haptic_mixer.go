package main

import (
	"fmt"
	"math"
	"sync"
	"time"
)

const (
	protocolVersion            = 40
	canonicalHapticSampleRate  = 48000 // one transport-neutral waveform for USB and Bluetooth
	bluetoothHapticSampleRate  = 3000  // DualSense wireless haptic stream
	hapticFramesPerReport32    = 32    // 64 signed 8-bit stereo bytes
	hapticFramesPerReport36    = 32    // DS5Dongle all-in-one: state + 64-byte stereo haptics
	hapticFramesPerReport39    = 64    // legacy diagnostic path
	bluetoothAudioReportSize   = 142
	bluetoothHaptic32ProtoSize = 142
	bluetoothHaptic36ProtoSize = 398
	bluetoothControlReportSize = 78
	bluetoothHaptic39ProtoSize = 547
)

func clamp01(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) || v <= 0 {
		return 0
	}
	if v >= 1 {
		return 1
	}
	return v
}
func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

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

type surfaceLane struct {
	profile                   string
	target, current, speed, t float64
	noise                     uint32
}
type voice struct {
	profile              string
	left, right          float64
	durationSamples, pos int
	noise                uint32
	priority             int
	created              time.Time
}

type hardIsolationState struct {
	side                    int // -1 impact left => suppress right, +1 impact right => suppress left
	hardSamplesRemaining    int
	releaseSamplesRemaining int
	releaseSamplesTotal     int
	lastSide                int
	lastEventAt             time.Time
}

func bodyOppositeMergeWindow(kind, profile string) time.Duration {
	cfg := feelProfile().BodyIsolation
	if isSuspensionBumpProfile(profile) || (kind != "collision" && kind != "landing" && profile != "collision" && profile != "landing") {
		if cfg.SuspensionBump.OppositeMergeWindowMS > 0 {
			return time.Duration(cfg.SuspensionBump.OppositeMergeWindowMS) * time.Millisecond
		}
	}
	return time.Duration(cfg.OppositeMergeWindowMS) * time.Millisecond
}

func bodyIsolationWindow(kind, profile string) (hard, release time.Duration) {
	cfg := feelProfile().BodyIsolation
	switch kind {
	case "collision":
		return time.Duration(cfg.Collision.HardAttackMS) * time.Millisecond, time.Duration(cfg.Collision.ReleaseEndMS) * time.Millisecond
	case "landing":
		return time.Duration(cfg.Landing.HardAttackMS) * time.Millisecond, time.Duration(cfg.Landing.ReleaseEndMS) * time.Millisecond
	default:
		if profile == "collision" {
			return time.Duration(cfg.Collision.HardAttackMS) * time.Millisecond, time.Duration(cfg.Collision.ReleaseEndMS) * time.Millisecond
		}
		if profile == "landing" {
			return time.Duration(cfg.Landing.HardAttackMS) * time.Millisecond, time.Duration(cfg.Landing.ReleaseEndMS) * time.Millisecond
		}
		return time.Duration(cfg.SuspensionBump.HardAttackMS) * time.Millisecond, time.Duration(cfg.SuspensionBump.ReleaseEndMS) * time.Millisecond
	}
}

func (m *hapticMixer) updateBodyIsolation(t telemetry, now time.Time) {
	side := t.BodySide
	merge := bodyOppositeMergeWindow(t.BodyKind, t.BodyProfile)
	if side == 0 {
		if m.isolation.side != 0 && (m.isolation.hardSamplesRemaining > 0 || m.isolation.releaseSamplesRemaining > 0) {
			return
		}
		m.isolation.side = 0
		m.isolation.hardSamplesRemaining = 0
		m.isolation.releaseSamplesRemaining = 0
		m.isolation.releaseSamplesTotal = 0
		m.isolation.lastSide = 0
		m.isolation.lastEventAt = now
		return
	}
	if m.isolation.lastSide != 0 && m.isolation.lastSide == -side && !m.isolation.lastEventAt.IsZero() && now.Sub(m.isolation.lastEventAt) <= merge {
		m.isolation.side = 0
		m.isolation.hardSamplesRemaining = 0
		m.isolation.releaseSamplesRemaining = 0
		m.isolation.releaseSamplesTotal = 0
		m.isolation.lastSide = side
		m.isolation.lastEventAt = now
		return
	}
	hard, release := bodyIsolationWindow(t.BodyKind, t.BodyProfile)
	hardSamples := maxInt(1, int(math.Round(hard.Seconds()*m.rate())))
	releaseTotalSamples := maxInt(hardSamples+1, int(math.Round(release.Seconds()*m.rate())))
	releaseSpan := maxInt(1, releaseTotalSamples-hardSamples)
	m.isolation.side = side
	m.isolation.hardSamplesRemaining = hardSamples
	m.isolation.releaseSamplesRemaining = releaseSpan
	m.isolation.releaseSamplesTotal = releaseSpan
	m.isolation.lastSide = side
	m.isolation.lastEventAt = now
}

func (m *hapticMixer) bodyIsolationGains() (left, right float64) {
	left, right = 1, 1
	iso := &m.isolation
	if iso.side == 0 {
		return
	}
	wrong := 1.0
	if iso.hardSamplesRemaining > 0 {
		wrong = 0
		iso.hardSamplesRemaining--
	} else if iso.releaseSamplesRemaining > 0 {
		done := iso.releaseSamplesTotal - iso.releaseSamplesRemaining
		wrong = smooth01(float64(done) / float64(maxInt(1, iso.releaseSamplesTotal)))
		iso.releaseSamplesRemaining--
	} else {
		iso.side = 0
		return
	}
	if iso.side < 0 {
		right = wrong
	} else {
		left = wrong
	}
	return
}

type hapticMixer struct {
	mu                                     sync.Mutex
	left, right                            surfaceLane
	voices                                 []*voice
	latest                                 telemetry
	lastPacket                             time.Time
	eventSynced                            bool
	bodyEvent, shiftEvent                  int
	lastShiftRendered                      time.Time
	surfacePatternL, surfacePatternR       surfacePatternState
	lastSurfaceSerialL, lastSurfaceSerialR uint64
	lastSyntheticCueL, lastSyntheticCueR   time.Time
	globalLowPhase, globalHighPhase        float64
	globalLowCurrent, globalHighCurrent    float64
	isolation                              hardIsolationState
	absKick                                bool
	sampleRate                             float64
	lastAcceptedBumpSide                   int
	lastAcceptedBumpAt                     time.Time
	lastAcceptedBumpStrength               float64
}

func newHapticMixer() *hapticMixer          { return newCanonicalHapticMixer() }
func newCanonicalHapticMixer() *hapticMixer { return newHapticMixerAtRate(canonicalHapticSampleRate) }
func newHapticMixerAtRate(sampleRate int) *hapticMixer {
	if sampleRate < 1000 {
		sampleRate = canonicalHapticSampleRate
	}
	return &hapticMixer{sampleRate: float64(sampleRate)}
}
func (m *hapticMixer) rate() float64 {
	if m == nil || m.sampleRate < 1000 {
		return canonicalHapticSampleRate
	}
	return m.sampleRate
}

func profilePriority(profile string) int {
	switch profile {
	case "collision":
		return 50
	case "landing":
		return 40
	case "abs_pulse":
		return 35
	case "suspension_bump":
		return 30
	case "suspension_secondary":
		return 26
	case "suspension_rebound":
		return 18
	case "shift":
		return feelProfile().ShiftHaptic.Priority
	case "rumble_strip", "cobblestone", "rock":
		return 12
	default:
		return 10
	}
}

func stereoClass(left, right float64) int {
	left, right = math.Max(0, left), math.Max(0, right)
	peak := math.Max(left, right)
	if peak <= 0.0001 {
		return 0
	}
	if left > right*1.45 {
		return -1
	}
	if right > left*1.45 {
		return 1
	}
	return 0
}

func mergeWindowForProfile(profile string) time.Duration {
	switch profile {
	case "collision":
		return 150 * time.Millisecond
	case "landing":
		return 170 * time.Millisecond
	case "suspension_bump":
		// Only collapse duplicate packets from the same physical observation.
		return 28 * time.Millisecond
	case "suspension_secondary":
		return 34 * time.Millisecond
	case "suspension_rebound":
		// At most one rebound is emitted per episode in Lua; this is only a
		// serialization guard against duplicate packets.
		return 80 * time.Millisecond
	case "shift":
		return 70 * time.Millisecond
	default:
		return 0
	}
}

func ducksForPriority(priority int) (surface, global float64) {
	surface, global = 1.0, 1.0
	if priority >= 50 {
		return 0.50, 0.10
	}
	if priority >= 40 {
		return 0.68, 0.42
	}
	if priority >= 30 {
		// Keep the impacted-side road texture audible underneath a discrete
		// suspension hit. Directional hard-isolation still mutes the opposite
		// grip, so this no longer sacrifices bump localization.
		return 0.60, 0.78
	}
	if priority >= feelProfile().ShiftHaptic.Priority {
		return feelProfile().ShiftHaptic.SurfaceDuck, feelProfile().ShiftHaptic.GlobalDuck
	}
	return surface, global
}

func (m *hapticMixer) enqueueVoice(profile string, left, right float64, durationMS int, now time.Time, seed uint32) {
	left, right = clamp01(left), clamp01(right)
	durationMS = clampInt(durationMS, 16, 240)
	if left <= 0 && right <= 0 {
		return
	}
	newClass := stereoClass(left, right)
	window := mergeWindowForProfile(profile)
	if window > 0 {
		for _, v := range m.voices {
			if v == nil || v.profile != profile || now.Sub(v.created) >= window {
				continue
			}
			if stereoClass(v.left, v.right) != newClass {
				continue
			}
			v.left = math.Max(v.left, left)
			v.right = math.Max(v.right, right)
			v.durationSamples = maxInt(v.durationSamples, int(math.Round(float64(durationMS)*m.rate()/1000.0)))
			v.pos = 0
			v.created = now
			v.noise = seed
			return
		}
	}

	v := &voice{
		profile:         profile,
		left:            left,
		right:           right,
		durationSamples: int(math.Round(float64(durationMS) * m.rate() / 1000.0)),
		noise:           seed,
		priority:        profilePriority(profile),
		created:         now,
	}

	// Match the USB multivoice queue: 32 voices maximum; on saturation evict
	// the oldest voice among the lowest-priority class, but never sacrifice a
	// higher-priority event for a lower-priority one.
	const maxVoices = 32
	if len(m.voices) >= maxVoices {
		drop := -1
		lowestPriority := int(^uint(0) >> 1)
		oldest := now
		for i, existing := range m.voices {
			if existing == nil {
				drop = i
				break
			}
			if existing.priority < lowestPriority ||
				(existing.priority == lowestPriority && existing.created.Before(oldest)) {
				lowestPriority = existing.priority
				oldest = existing.created
				drop = i
			}
		}
		if drop >= 0 && (m.voices[drop] == nil || m.voices[drop].priority <= v.priority) {
			m.voices = append(m.voices[:drop], m.voices[drop+1:]...)
		} else {
			return
		}
	}
	m.voices = append(m.voices, v)
}

func setSurfaceLane(lane *surfaceLane, profile string, strength, speed float64, seed uint32) {
	if lane == nil {
		return
	}
	// Surface lane updates do not overwrite the profile during scheduler
	// gaps. It only moves target to zero, allowing the lane phase to continue.
	if profile == "" || profile == "none" || strength <= 0 {
		lane.target = 0
		return
	}
	if lane.profile != profile {
		lane.profile = profile
		lane.t = 0
	}
	lane.target = math.Max(0.0, math.Min(0.99, strength))
	lane.speed = math.Max(0, speed)
	if lane.noise == 0 {
		lane.noise = seed
	}
}

func bodyPeakStrength(t telemetry) float64 {
	return math.Max(t.BodyStrength, math.Max(t.BodyLeftStrength, t.BodyRightStrength))
}

func bumpTransientEvidence(t telemetry, side int) (newSide, oldSide float64) {
	if t.Raw == nil {
		return 0, 0
	}
	left := math.Max(clamp01(t.Raw.CandidateL), clamp01(t.Raw.PeakImpulseL))
	right := math.Max(clamp01(t.Raw.CandidateR), clamp01(t.Raw.PeakImpulseR))
	if side < 0 {
		return left, right
	}
	if side > 0 {
		return right, left
	}
	return 0, math.Max(left, right)
}

func (m *hapticMixer) suppressBumpEcho(t telemetry, now time.Time) bool {
	if !isSuspensionBumpProfile(t.BodyProfile) && t.BodyKind != "wheel" {
		return false
	}
	side, peak := t.BodySide, bodyPeakStrength(t)
	if m.lastAcceptedBumpAt.IsZero() || now.Sub(m.lastAcceptedBumpAt) > 180*time.Millisecond {
		if side != 0 {
			m.lastAcceptedBumpSide, m.lastAcceptedBumpAt, m.lastAcceptedBumpStrength = side, now, peak
		}
		return false
	}
	if m.lastAcceptedBumpSide != 0 {
		if side == 0 {
			return true
		}
		if side == -m.lastAcceptedBumpSide {
			// An opposite-side event inside the physical bump cluster is accepted
			// only when the new wheel has its own transient load/peak evidence.
			// Continuous road texture is deliberately excluded: it can alternate
			// left/right from one 30 Hz sample to the next on a single-wheel hit.
			newEvidence, oldEvidence := bumpTransientEvidence(t, side)
			clearlyIndependent := newEvidence >= 0.10 && newEvidence > oldEvidence*1.25
			if !clearlyIndependent {
				return true
			}
		}
	}
	if side != 0 {
		m.lastAcceptedBumpSide, m.lastAcceptedBumpAt, m.lastAcceptedBumpStrength = side, now, peak
	}
	return false
}

func (m *hapticMixer) update(t telemetry, now time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.latest = t
	m.lastPacket = now
	if !t.Active {
		m.left.target = 0
		m.right.target = 0
		return
	}
	if !m.eventSynced || t.BodyEvent < m.bodyEvent || t.ShiftEvent < m.shiftEvent {
		m.bodyEvent = t.BodyEvent
		m.shiftEvent = t.ShiftEvent
		m.eventSynced = true
		return
	}
	if t.BodyEvent > 0 && t.BodyEvent != m.bodyEvent {
		m.bodyEvent = t.BodyEvent
		// Vehicle Lua already performs the physical bump-cluster/deduplication before
		// queueing the event. Do not filter the serialized event a second time
		// using t.Raw: queued body events can be sent one or more telemetry frames
		// after detection, so CandidateL/R and PeakImpulseL/R may belong to a later
		// wheel sample. The bridge must trust the event payload that was accepted by
		// vehicle Lua instead of dropping a genuine opposite/center hit as an echo.
		profile, l, r, dur := bodyStereo(t)
		if isPrimarySuspensionBumpProfile(profile) {
			// Directional primary/second-axle impacts are physically unambiguous.  The
			// event voice itself is already hard-panned, but centered engine/tyre
			// bands and the opposite surface lane were still audible at the same
			// time.  On a DualSense those background components can be felt in the
			// opposite grip and mask the pan even though the bump PCM is correct.
			//
			// For the exact lifetime of a LEFT/RIGHT suspension-bump voice, mute
			// every contribution on the opposite output channel.  A CENTER event
			// remains symmetric.  A newly arriving opposite-side bump immediately
			// replaces the isolation side, so closely spaced left/right bumpers are
			// not merged or delayed.
			if t.BodySide != 0 {
				hardSamples := maxInt(1, int(math.Round(float64(dur)*m.rate()/1000.0)))
				m.isolation.side = t.BodySide
				m.isolation.hardSamplesRemaining = hardSamples
				m.isolation.releaseSamplesRemaining = 0
				m.isolation.releaseSamplesTotal = 0
				m.isolation.lastSide = t.BodySide
				m.isolation.lastEventAt = now
			} else {
				m.isolation.side = 0
				m.isolation.hardSamplesRemaining = 0
				m.isolation.releaseSamplesRemaining = 0
				m.isolation.releaseSamplesTotal = 0
				m.isolation.lastSide = 0
				m.isolation.lastEventAt = now
			}
		} else if profile == "suspension_rebound" {
			// Rebound is intentionally weak; do not mute an entire grip for it.
		} else {
			m.updateBodyIsolation(t, now)
		}
		if isSuspensionBumpProfile(profile) {
			fmt.Printf("BUMP_SOURCE event=%d profile=%s side=%d reason=%s conf=%.2f score=%.2f/%.2f contact=%.2f/%.2f energy=%.2f/%.2f peak=%.2f/%.2f jolt=%.2f/%.2f susp=%.2f/%.2f wheel=%d/%d axle=%d/%d\n",
				t.BodyEvent, profile, t.BodySide, t.BodySourceReason, t.BodySourceConfidence,
				t.BodySourceLeftScore, t.BodySourceRightScore,
				t.BodySourceLeftContact, t.BodySourceRightContact,
				t.BodySourceLeftEnergy, t.BodySourceRightEnergy,
				t.BodySourceLeftPeak, t.BodySourceRightPeak,
				t.BodySourceLeftJolt, t.BodySourceRightJolt,
				t.BodySourceLeftStress, t.BodySourceRightStress,
				t.BodySourceLeftWheel, t.BodySourceRightWheel,
				t.BodySourceLeftAxle, t.BodySourceRightAxle)
			fmt.Printf("BUMP_PAN event=%d profile=%s inputSide=%d pcmL=%.3f pcmR=%.3f durationMS=%d\n",
				t.BodyEvent, profile, t.BodySide, l, r, dur)
		}
		m.enqueueVoice(profile, l, r, dur, now, uint32(t.BodyEvent)*2654435761+0x9E3779B9)
	}
	if t.ShiftEvent > 0 && t.ShiftEvent != m.shiftEvent {
		m.shiftEvent = t.ShiftEvent
		// Same 45 ms safety gate as the validated USB bridge. Legitimate fast
		// sequential shifts still pass, while pathological duplicate counters do not.
		if m.lastShiftRendered.IsZero() || now.Sub(m.lastShiftRendered) >= 45*time.Millisecond {
			m.lastShiftRendered = now
			level, dur := shiftCue(t)
			m.enqueueVoice("shift", level, level, dur, now, uint32(t.ShiftEvent)*2246822519+0x85EBCA6B)
		}
	}
}

func (m *hapticMixer) snapshot() (telemetry, time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.latest, m.lastPacket
}

func (m *hapticMixer) setSharedDynamics(absKick bool) {
	m.mu.Lock()
	m.absKick = absKick
	m.mu.Unlock()
}

func isSurfaceVoiceProfile(profile string) bool {
	switch profile {
	case "asphalt", "asphalt_wet", "slippery", "ice", "sand", "mud", "dirt", "dusty_dirt", "sandy_road", "gravel", "grass", "snow", "rock", "cobblestone", "rumble_strip":
		return true
	default:
		return false
	}
}

func (m *hapticMixer) render(frames int, now time.Time) ([]int8, mixerStatus) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]int8, frames*2)
	surfaceOut := make([]int8, frames*2)
	status := mixerStatus{}
	packetAge := time.Duration(1<<63 - 1)
	if !m.lastPacket.IsZero() {
		packetAge = now.Sub(m.lastPacket)
	}
	hardStale := m.lastPacket.IsZero() || packetAge > 1200*time.Millisecond || !m.latest.Active || m.latest.Raw == nil
	status.packetAgeMS = float64(packetAge) / float64(time.Millisecond)
	status.stale = hardStale
	freshnessGain := 1.0 // Keep the last valid telemetry until the 1.2 s watchdog expires.
	if hardStale {
		m.left.target, m.right.target = 0, 0
		m.surfacePatternL.reset("none", now)
		m.surfacePatternR.reset("none", now)
	} else {
		raw := m.latest.Raw
		if raw.Airborne || raw.GroundedWheels <= 0 {
			m.left.target, m.right.target = 0, 0
			m.surfacePatternL.reset("none", now)
			m.surfacePatternR.reset("none", now)
		} else {
			sourceProfileL := surfaceProfileFromMaterial(raw.SurfaceMaterialL)
			sourceProfileR := surfaceProfileFromMaterial(raw.SurfaceMaterialR)
			rawStrengthL := continuousSurfaceStrength(sourceProfileL, raw.SurfaceRoughnessL, raw.Speed, raw.RoadExcitationL, raw.RoadSlipL)
			rawStrengthR := continuousSurfaceStrength(sourceProfileR, raw.SurfaceRoughnessR, raw.Speed, raw.RoadExcitationR, raw.RoadSlipR)
			baseStrengthL := tactileSurfaceStrength(sourceProfileL, rawStrengthL)
			baseStrengthR := tactileSurfaceStrength(sourceProfileR, rawStrengthR)
			sceneLowL, sceneHighL := continuousSurfaceScene(sourceProfileL, 1.0, raw.Speed, now, &m.surfacePatternL)
			sceneLowR, sceneHighR := continuousSurfaceScene(sourceProfileR, 1.0, raw.Speed, now, &m.surfacePatternR)
			strengthL := audibleSurfaceStrength(sourceProfileL, baseStrengthL, sceneLowL, sceneHighL)
			strengthR := audibleSurfaceStrength(sourceProfileR, baseStrengthR, sceneLowR, sceneHighR)
			lowSpeedScale := lowSpeedSurfaceScale(raw.Speed)
			strengthL *= lowSpeedScale
			strengthR *= lowSpeedScale
			activeL := sourceProfileL != "none" && raw.Speed >= 0.25 && strengthL > 0.0015
			activeR := sourceProfileR != "none" && raw.Speed >= 0.25 && strengthR > 0.0015

			// Synthetic asperities are derived from the physical
			// material/scheduler state before inactive continuous lanes are normalized
			// to "none".
			if m.surfacePatternL.serial != m.lastSurfaceSerialL {
				m.lastSurfaceSerialL = m.surfacePatternL.serial
				if cue, dur, ok := syntheticAsperityCue(sourceProfileL, &m.surfacePatternL); ok &&
					(m.lastSyntheticCueL.IsZero() || now.Sub(m.lastSyntheticCueL) >= surfaceCueCooldownAt(sourceProfileL, raw.Speed)) {
					cue = math.Min(0.99, cue*lowSpeedSurfaceScale(raw.Speed))
					m.enqueueVoice(sourceProfileL, cue, 0, dur, now,
						uint32(m.surfacePatternL.serial)*2654435761+0xA341316C)
					m.lastSyntheticCueL = now
				}
			}
			if m.surfacePatternR.serial != m.lastSurfaceSerialR {
				m.lastSurfaceSerialR = m.surfacePatternR.serial
				if cue, dur, ok := syntheticAsperityCue(sourceProfileR, &m.surfacePatternR); ok &&
					(m.lastSyntheticCueR.IsZero() || now.Sub(m.lastSyntheticCueR) >= surfaceCueCooldownAt(sourceProfileR, raw.Speed)) {
					cue = math.Min(0.99, cue*lowSpeedSurfaceScale(raw.Speed))
					m.enqueueVoice(sourceProfileR, 0, cue, dur, now,
						uint32(m.surfacePatternR.serial)*2246822519+0xC8013EA4)
					m.lastSyntheticCueR = now
				}
			}

			outputProfileL, outputProfileR := sourceProfileL, sourceProfileR
			if !activeL {
				outputProfileL, strengthL = "none", 0
			}
			if !activeR {
				outputProfileR, strengthR = "none", 0
			}
			setSurfaceLane(&m.left, outputProfileL, strengthL*freshnessGain, raw.Speed, 0xA341316C)
			setSurfaceLane(&m.right, outputProfileR, strengthR*freshnessGain, raw.Speed, 0xC8013EA4)

			status.sourceProfileL, status.sourceProfileR = sourceProfileL, sourceProfileR
			status.profileL, status.profileR = outputProfileL, outputProfileR
			status.surfaceL, status.surfaceR = m.left.target, m.right.target
			status.slipL, status.slipR = clamp01(raw.RoadSlipL), clamp01(raw.RoadSlipR)
		}
	}

	rate := m.rate()
	smooth := 1 - math.Exp(-1/(rate*0.010))
	maxPriority := 0
	for _, v := range m.voices {
		if v != nil && v.priority > maxPriority {
			maxPriority = v.priority
		}
	}
	status.maxPriority = maxPriority
	surfaceDuck, globalDuck := ducksForPriority(maxPriority)

	// The control loop updates the global target once per tick, while the 48 kHz
	// renderer smooths it sample-by-sample. Bluetooth is derived later from the same
	// canonical stream.
	targetLow, targetHigh := 0.0, 0.0
	if !hardStale && m.latest.Raw != nil {
		baseLow, baseHigh := tactileBeamNGBase(m.latest.Raw)
		spinLow, spinHigh, _ := wheelspinRumbleAt(m.latest, now)
		targetLow = mixBand(baseLow, spinLow)
		targetHigh = mixBand(baseHigh, spinHigh)
		if m.absKick {
			targetLow = math.Max(targetLow, 0.080)
			targetHigh = math.Max(targetHigh, 0.050)
		}
	}
	status.globalLevel = math.Max(math.Abs(targetLow), math.Abs(targetHigh))
	globalAlpha := 1 - math.Exp(-1/(rate*0.012))

	nextVoices := m.voices[:0]
	for i := 0; i < frames; i++ {
		l, r := 0.0, 0.0
		surfaceL, surfaceR := 0.0, 0.0
		bumpVoiceActive := false

		// USB render order: transient voices first. Clamp after each voice just as
		// the WASAPI engine clamps when adding PCM buffers.
		for _, v := range m.voices {
			if v == nil || v.pos >= v.durationSamples {
				continue
			}
			tt := float64(v.pos) / rate
			dur := float64(v.durationSamples) / rate
			wave := profileWave(v.profile, tt, dur, &v.noise) * profileGain(v.profile)
			voiceL, voiceR := wave*v.left, wave*v.right
			if isPrimarySuspensionBumpProfile(v.profile) {
				bumpVoiceActive = true
			}
			l = clamp(l+voiceL, -0.99, 0.99)
			r = clamp(r+voiceR, -0.99, 0.99)
			if isSurfaceVoiceProfile(v.profile) {
				surfaceL = clamp(surfaceL+voiceL, -0.99, 0.99)
				surfaceR = clamp(surfaceR+voiceR, -0.99, 0.99)
			}
			v.pos++
		}

		// Then USB's centered BeamNG/tyre/ABS bands with the same transient duck.
		m.globalLowCurrent += (targetLow - m.globalLowCurrent) * globalAlpha
		m.globalHighCurrent += (targetHigh - m.globalHighCurrent) * globalAlpha
		m.globalLowPhase += 2 * math.Pi * 58 / rate
		m.globalHighPhase += 2 * math.Pi * 165 / rate
		global := (math.Sin(m.globalLowPhase)*m.globalLowCurrent*0.72 +
			math.Sin(m.globalHighPhase)*m.globalHighCurrent*0.58) * globalDuck
		if !bumpVoiceActive {
			l = clamp(l+global, -0.99, 0.99)
			r = clamp(r+global, -0.99, 0.99)
		}

		// Finally the independent left/right continuous surface lanes.
		m.left.current += (m.left.target - m.left.current) * smooth
		m.right.current += (m.right.target - m.right.current) * smooth
		if m.left.current > 0.0005 {
			surface := surfaceWave(m.left.profile, m.left.t, m.left.speed, &m.left.noise) *
				m.left.current * surfaceDuck * surfaceDrive(m.left.profile)
			l = clamp(l+surface, -0.99, 0.99)
			surfaceL = clamp(surfaceL+surface, -0.99, 0.99)
		}
		if m.right.current > 0.0005 {
			surface := surfaceWave(m.right.profile, m.right.t, m.right.speed, &m.right.noise) *
				m.right.current * surfaceDuck * surfaceDrive(m.right.profile)
			r = clamp(r+surface, -0.99, 0.99)
			surfaceR = clamp(surfaceR+surface, -0.99, 0.99)
		}
		m.left.t += 1.0 / rate
		m.right.t += 1.0 / rate

		leftIso, rightIso := m.bodyIsolationGains()
		l *= leftIso
		r *= rightIso
		surfaceL *= leftIso
		surfaceR *= rightIso

		out[i*2] = int8(math.Round(clamp(l, -0.99, 0.99) * 127))
		out[i*2+1] = int8(math.Round(clamp(r, -0.99, 0.99) * 127))
		surfaceOut[i*2] = int8(math.Round(clamp(surfaceL, -0.99, 0.99) * 127))
		surfaceOut[i*2+1] = int8(math.Round(clamp(surfaceR, -0.99, 0.99) * 127))
	}
	for _, v := range m.voices {
		if v.pos < v.durationSamples {
			nextVoices = append(nextVoices, v)
		}
	}
	m.voices = nextVoices
	status.surfacePCM = surfaceOut
	status.nonSilent = countNonSilent(out)
	if len(out) > 0 {
		sumSquares := 0.0
		peak := 0.0
		for _, sample := range out {
			v := math.Abs(float64(sample)) / 127.0
			sumSquares += v * v
			if v > peak {
				peak = v
			}
		}
		status.blockRMS = math.Sqrt(sumSquares / float64(len(out)))
		status.blockPeak = peak
	}
	return out, status
}

func (m *hapticMixer) voiceCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.voices)
}

type mixerStatus struct {
	profileL, profileR               string
	sourceProfileL, sourceProfileR   string
	surfaceL, surfaceR, slipL, slipR float64
	surfacePCM                       []int8
	nonSilent                        int
	blockRMS, blockPeak              float64
	packetAgeMS                      float64
	stale                            bool
	maxPriority                      int
	globalLevel                      float64
}

func countNonSilent(v []int8) int {
	n := 0
	for _, x := range v {
		if x != 0 {
			n++
		}
	}
	return n
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
