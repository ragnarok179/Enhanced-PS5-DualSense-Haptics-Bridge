package main

import (
	"fmt"
	"math"
	"sync"
	"time"
)

const (
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
	transport                              outputTransport
}

func newHapticMixer() *hapticMixer          { return newCanonicalHapticMixer() }
func newCanonicalHapticMixer() *hapticMixer { return newHapticMixerAtRate(canonicalHapticSampleRate) }
func newCanonicalHapticMixerForTransport(transport outputTransport) *hapticMixer {
	return newHapticMixerAtRateForTransport(canonicalHapticSampleRate, transport)
}
func newHapticMixerAtRate(sampleRate int) *hapticMixer {
	return newHapticMixerAtRateForTransport(sampleRate, transportReference)
}
func newHapticMixerAtRateForTransport(sampleRate int, transport outputTransport) *hapticMixer {
	if sampleRate < 1000 {
		sampleRate = canonicalHapticSampleRate
	}
	return &hapticMixer{sampleRate: float64(sampleRate), transport: transport}
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
	lane.target = math.Max(0.0, math.Min(32.0, strength))
	lane.speed = math.Max(0, speed)
	if lane.noise == 0 {
		lane.noise = seed
	}
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
		if isSuspensionBumpProfile(profile) && runtimeDiagnosticsEnabled() {
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

func isImpactVoiceProfile(profile string) bool {
	switch profile {
	case "collision", "landing", "suspension_bump", "suspension_secondary", "suspension_rebound":
		return true
	default:
		return false
	}
}

func splitSurfaceRawComponents(profile string, roughness, speed, rollingExcitation, legacyExcitation, slip float64) (rollingRaw, slipRaw float64) {
	// New BeamNG mods send a rolling-only excitation channel. The legacy
	// RoadExcitation channel is kept exactly as before and can contain a small
	// slip-derived texture term. Compute the stock total with that legacy value,
	// then assign every incremental slip-driven contribution to the Slip side.
	rollingRaw, _ = surfaceStrengthComponents(profile, roughness, speed, rollingExcitation, 0)
	totalRollingRaw, directSlipRaw := surfaceStrengthComponents(profile, roughness, speed, legacyExcitation, slip)
	slipRaw = math.Max(0, totalRollingRaw-rollingRaw) + math.Max(0, directSlipRaw)
	return
}

func splitCalibratedSurfaceStrength(profile string, rollingRaw, slidingRaw, sceneLow, sceneHigh float64) (rollingStrength, slipStrength, rollingBase float64) {
	// Reconstruct the original calibrated total exactly, then decompose it into
	// rolling and incremental-slip contributions. Because audibleSurfaceStrength
	// is linear in its base argument for a fixed scene envelope, the sum below is
	// exactly the old total at gain 1.0.
	rollingBase = tactileSurfaceStrength(profile, math.Min(0.44, math.Max(0, rollingRaw)))
	totalBase := tactileSurfaceStrength(profile, math.Min(0.44, math.Max(0, rollingRaw+slidingRaw)))
	rollingStrength = audibleSurfaceStrength(profile, rollingBase, sceneLow, sceneHigh)
	totalStrength := audibleSurfaceStrength(profile, totalBase, sceneLow, sceneHigh)
	slipStrength = math.Max(0, totalStrength-rollingStrength)
	return
}

func applyBoostOnlyAsphaltBed(profile string, rollingStrength, rollingBase, rollingGain float64) float64 {
	if profile != "asphalt" || rollingGain <= 1.000001 || rollingBase <= 0 || rollingStrength > 0.0015 {
		return rollingStrength
	}
	ref := referenceFor(profile, surfaceRollingReferencePercent, 100)
	maxGain := 100.0 / math.Max(ref, 1)
	if maxGain <= 1.000001 {
		return rollingStrength
	}
	progress := clamp01((rollingGain - 1) / (maxGain - 1))
	// Stock asphalt intentionally has no continuous bed during scheduler gaps.
	// Preserve that exactly at the calibrated reference. Above the reference, the
	// user's explicit boost fades in the same asphalt carrier so "Rolling" also
	// changes normal smooth-road feedback instead of only occasional asperities.
	return math.Max(rollingStrength, rollingBase*progress)
}

func scaleSurfaceAsperityCue(cue, lowSpeedScale, rollingGain float64) float64 {
	return math.Min(0.99, math.Max(0, cue)*math.Max(0, lowSpeedScale)*math.Max(0, rollingGain))
}

func (m *hapticMixer) renderInto(frames int, now time.Time, out, surfaceOut []int8, collectStats bool) ([]int8, mixerStatus) {
	m.mu.Lock()
	defer m.mu.Unlock()
	required := maxInt(frames, 0) * 2
	if cap(out) < required {
		out = make([]int8, required)
	} else {
		out = out[:required]
		clear(out)
	}
	collectSurface := surfaceOut != nil
	if collectSurface {
		if cap(surfaceOut) < required {
			surfaceOut = make([]int8, required)
		} else {
			surfaceOut = surfaceOut[:required]
			clear(surfaceOut)
		}
	}
	status := mixerStatus{}
	user := currentUserSettings()
	surfaceUserGain := surfaceMasterGain(user)
	impactUserGain := feedbackGain(user.Haptics.ImpactEnabled, user.Haptics.ImpactStrength)
	masterUserGain := hapticMasterGain(user.Haptics.MasterEnabled, user.Haptics.MasterStrength)
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
			rollingGainL, slipGainL := surfaceRollingGain(user, sourceProfileL), surfaceSlipGain(user, sourceProfileL)
			rollingGainR, slipGainR := surfaceRollingGain(user, sourceProfileR), surfaceSlipGain(user, sourceProfileR)

			// Split the calibrated surface AFTER tactile shaping. This is the key to
			// making Rolling and Slip genuinely independent while preserving the old
			// default exactly: rollingStock + slipStock == the legacy total. User
			// absolute-power gains are applied only after that decomposition.
			rollingExcitationL, rollingExcitationR := raw.RoadExcitationL, raw.RoadExcitationR
			if raw.RoadRollingExcitationValid {
				rollingExcitationL = raw.RoadRollingExcitationL
				rollingExcitationR = raw.RoadRollingExcitationR
			}
			rollingRawL, slidingRawL := splitSurfaceRawComponents(sourceProfileL, raw.SurfaceRoughnessL, raw.Speed, rollingExcitationL, raw.RoadExcitationL, raw.RoadSlipL)
			rollingRawR, slidingRawR := splitSurfaceRawComponents(sourceProfileR, raw.SurfaceRoughnessR, raw.Speed, rollingExcitationR, raw.RoadExcitationR, raw.RoadSlipR)
			sceneLowL, sceneHighL := continuousSurfaceScene(sourceProfileL, 1.0, raw.Speed, now, &m.surfacePatternL)
			sceneLowR, sceneHighR := continuousSurfaceScene(sourceProfileR, 1.0, raw.Speed, now, &m.surfacePatternR)
			rollingStockL, slipStockL, rollingBaseL := splitCalibratedSurfaceStrength(sourceProfileL, rollingRawL, slidingRawL, sceneLowL, sceneHighL)
			rollingStockR, slipStockR, rollingBaseR := splitCalibratedSurfaceStrength(sourceProfileR, rollingRawR, slidingRawR, sceneLowR, sceneHighR)
			rollingStockL = applyBoostOnlyAsphaltBed(sourceProfileL, rollingStockL, rollingBaseL, rollingGainL)
			rollingStockR = applyBoostOnlyAsphaltBed(sourceProfileR, rollingStockR, rollingBaseR, rollingGainR)
			lowSpeedScale := lowSpeedSurfaceScale(raw.Speed)
			rollingStrengthL := rollingStockL * rollingGainL * lowSpeedScale
			rollingStrengthR := rollingStockR * rollingGainR * lowSpeedScale
			slipStrengthL := slipStockL * slipGainL * lowSpeedScale
			slipStrengthR := slipStockR * slipGainR * lowSpeedScale
			strengthL := rollingStrengthL + slipStrengthL
			strengthR := rollingStrengthR + slipStrengthR
			activeL := sourceProfileL != "none" && raw.Speed >= 0.25 && strengthL > 0.0015
			activeR := sourceProfileR != "none" && raw.Speed >= 0.25 && strengthR > 0.0015

			// Synthetic asperities are derived from the physical
			// material/scheduler state before inactive continuous lanes are normalized
			// to "none".
			if m.surfacePatternL.serial != m.lastSurfaceSerialL {
				m.lastSurfaceSerialL = m.surfacePatternL.serial
				if cue, dur, ok := syntheticAsperityCue(sourceProfileL, &m.surfacePatternL); ok &&
					(m.lastSyntheticCueL.IsZero() || now.Sub(m.lastSyntheticCueL) >= surfaceCueCooldownAt(sourceProfileL, raw.Speed)) {
					cue = scaleSurfaceAsperityCue(cue, lowSpeedSurfaceScale(raw.Speed), rollingGainL)
					m.enqueueVoice(sourceProfileL, cue, 0, dur, now,
						uint32(m.surfacePatternL.serial)*2654435761+0xA341316C)
					m.lastSyntheticCueL = now
				}
			}
			if m.surfacePatternR.serial != m.lastSurfaceSerialR {
				m.lastSurfaceSerialR = m.surfacePatternR.serial
				if cue, dur, ok := syntheticAsperityCue(sourceProfileR, &m.surfacePatternR); ok &&
					(m.lastSyntheticCueR.IsZero() || now.Sub(m.lastSyntheticCueR) >= surfaceCueCooldownAt(sourceProfileR, raw.Speed)) {
					cue = scaleSurfaceAsperityCue(cue, lowSpeedSurfaceScale(raw.Speed), rollingGainR)
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
			voiceGain := 1.0
			if isSurfaceVoiceProfile(v.profile) {
				voiceGain *= surfaceUserGain
			}
			if isImpactVoiceProfile(v.profile) {
				voiceGain *= impactUserGain
			}
			voiceL, voiceR := wave*v.left*voiceGain, wave*v.right*voiceGain
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
				m.left.current * surfaceDuck * surfaceDrive(m.left.profile) * surfaceUserGain
			l = clamp(l+surface, -0.99, 0.99)
			surfaceL = clamp(surfaceL+surface, -0.99, 0.99)
		}
		if m.right.current > 0.0005 {
			surface := surfaceWave(m.right.profile, m.right.t, m.right.speed, &m.right.noise) *
				m.right.current * surfaceDuck * surfaceDrive(m.right.profile) * surfaceUserGain
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

		l *= masterUserGain
		r *= masterUserGain
		surfaceL *= masterUserGain
		surfaceR *= masterUserGain
		out[i*2] = int8(math.Round(clamp(l, -0.99, 0.99) * 127))
		out[i*2+1] = int8(math.Round(clamp(r, -0.99, 0.99) * 127))
		if collectSurface {
			surfaceOut[i*2] = int8(math.Round(clamp(surfaceL, -0.99, 0.99) * 127))
			surfaceOut[i*2+1] = int8(math.Round(clamp(surfaceR, -0.99, 0.99) * 127))
		}
	}
	for _, v := range m.voices {
		if v.pos < v.durationSamples {
			nextVoices = append(nextVoices, v)
		}
	}
	m.voices = nextVoices
	if collectSurface {
		status.surfacePCM = surfaceOut
	}
	if collectStats {
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
	}
	return out, status
}

// render is the allocation-friendly test/analysis path. Runtime rendering uses
// renderInto with buffers owned by sharedFeelEngine.
func (m *hapticMixer) render(frames int, now time.Time) ([]int8, mixerStatus) {
	return m.renderInto(frames, now, make([]int8, maxInt(frames, 0)*2), make([]int8, maxInt(frames, 0)*2), true)
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
