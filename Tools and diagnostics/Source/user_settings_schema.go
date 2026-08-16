package main

import "math"

var knownSurfaceProfiles = []string{
	"asphalt", "asphalt_wet", "slippery", "ice", "dirt", "dusty_dirt",
	"sandy_road", "sand", "mud", "gravel", "grass", "snow", "rock",
	"cobblestone", "rumble_strip",
}

const (
	// Haptic master is a global multiplier. The existing effect set already
	// contains full-scale suspension/collision peaks, so gain 1.0 is 100%.
	hapticReferencePercent = 100.0

	// Road-surface master is a pure global multiplier. 100% is gain 1.0.
	// Absolute per-material power is represented by the Rolling controls below.
	surfaceMasterReferencePercent = 100.0

	ledReferencePercent       = 86.0 // 220 / 255 calibrated lightbar ceiling
	r2DynamicReferencePercent = 25.0
	legacyR2DynamicPeakByte   = 64 // pre-normalized settings storage; migration only
)

// Previous UI reference tables are kept only for lossless migrations.
var v147SurfaceRollingReferencePercent = map[string]float64{
	"asphalt": 22, "asphalt_wet": 26, "slippery": 20, "ice": 20,
	"dirt": 50, "dusty_dirt": 50, "sandy_road": 50, "sand": 28,
	"mud": 32, "gravel": 62, "grass": 36, "snow": 36,
	"rock": 78, "cobblestone": 74, "rumble_strip": 88,
}

var v147SurfaceSlipReferencePercent = map[string]float64{
	"asphalt": 3, "asphalt_wet": 7, "slippery": 8, "ice": 7,
	"dirt": 8, "dusty_dirt": 8, "sandy_road": 5, "sand": 3,
	"mud": 4, "gravel": 10, "grass": 5, "snow": 4,
	"rock": 8, "cobblestone": 6, "rumble_strip": 4,
}

// V1.4.8 used final-peak estimates and a relative 100% slip control. Those
// values did preserve the default gain, but they did not describe perceived
// haptic power and the slip control had almost no useful upward range.
var v148SurfaceRollingReferencePercent = map[string]float64{
	"asphalt": 29, "asphalt_wet": 43, "slippery": 43, "ice": 43,
	"dirt": 70, "dusty_dirt": 70, "sandy_road": 65, "sand": 48,
	"mud": 58, "gravel": 100, "grass": 45, "snow": 39,
	"rock": 100, "cobblestone": 100, "rumble_strip": 100,
}

var v148SurfaceSlipReferencePercent = map[string]float64{
	"asphalt": 100, "asphalt_wet": 100, "slippery": 100, "ice": 100,
	"dirt": 100, "dusty_dirt": 100, "sandy_road": 100, "sand": 100,
	"mud": 100, "gravel": 100, "grass": 100, "snow": 100,
	"rock": 100, "cobblestone": 100, "rumble_strip": 100,
}

// Absolute haptic-power references for the unchanged calibrated renderer.
// The table was measured from the canonical 48-kHz surface stream with maximum
// rolling excitation (slip=0), using the active RMS of a full-strength primary
// suspension bump as the 100% haptic-power reference. These numbers are labels
// only: at each reference, runtime gain is exactly 1.0 and the old feel is
// preserved sample-for-sample. 100% is intentionally a large boost target.
var surfaceRollingReferencePercent = map[string]float64{
	"asphalt": 10, "asphalt_wet": 6, "slippery": 3, "ice": 3,
	"dirt": 23, "dusty_dirt": 22, "sandy_road": 34, "sand": 24,
	"mud": 22, "gravel": 62, "grass": 12, "snow": 8,
	"rock": 31, "cobblestone": 70, "rumble_strip": 84,
}

// Slip is isolated as the incremental output produced by RoadSlip over the
// rolling signal. It is extremely small in the stock calibration, which is why
// the previous relative 100% slider sounded ineffective. At the references
// below the stock feel is unchanged; increasing toward 100% boosts only that
// incremental sliding component.
// Per-material compensation used only ABOVE the calibrated reference. The
// normal/default renderer is untouched. At 100% this compensates spectral and
// scheduler duty-cycle differences so each rolling surface can approach the
// same full-power haptic reference as a strongest primary bump instead of only
// reaching the same instantaneous PCM peak.
var surfaceRollingFullScaleBoost = map[string]float64{
	"asphalt": 1.215, "asphalt_wet": 1.028, "slippery": 4.196, "ice": 3.606,
	"dirt": 1.774, "dusty_dirt": 1.867, "sandy_road": 1.152, "sand": 1.028,
	"mud": 1.463, "gravel": 1.183, "grass": 2.581, "snow": 1.867,
	"rock": 2.364, "cobblestone": 1.401, "rumble_strip": 1.929,
}

var surfaceSlipReferencePercent = map[string]float64{
	"asphalt": 1, "asphalt_wet": 1, "slippery": 1, "ice": 1,
	"dirt": 1, "dusty_dirt": 1, "sandy_road": 1, "sand": 1,
	"mud": 1, "gravel": 1, "grass": 1, "snow": 1,
	"rock": 1, "cobblestone": 1, "rumble_strip": 1,
}

// Slip is a much smaller stock component than rolling. The 1% defaults below
// preserve that stock contribution exactly. Above the reference, these
// per-material compensations map 100% toward the same full-power incremental
// haptic reference, so the control remains audible instead of topping out at a
// barely perceptible coefficient change.
var surfaceSlipFullScaleBoost = map[string]float64{
	"asphalt": 2.880, "asphalt_wet": 2.177, "slippery": 12.955, "ice": 11.666,
	"dirt": 3.114, "dusty_dirt": 3.114, "sandy_road": 1.357, "sand": 2.763,
	"mud": 4.052, "gravel": 0.420, "grass": 10.026, "snow": 9.089,
	"rock": 3.349, "cobblestone": 0.654, "rumble_strip": 3.817,
}

func percentToStrength255(percent float64) int {
	if percent < 1 {
		percent = 1
	}
	if percent > 100 {
		percent = 100
	}
	return clampStrength255(int(math.Round(percent * 255.0 / 100.0)))
}

func strength255ToPercent(strength int) float64 {
	return float64(clampStrength255(strength)) * 100.0 / 255.0
}

func percentToTriggerForce48(percent float64) int {
	if percent < 1 {
		percent = 1
	}
	if percent > 100 {
		percent = 100
	}
	return clampTriggerStrength48(int(math.Round(percent * float64(triggerForceMax) / 100.0)))
}

// legacyTriggerStrength255To48 exists only for settings-file/wire migration.
// Active trigger-force storage is 0..48 from schema 11 onward.
func legacyTriggerStrength255To48(strength int) int {
	strength = clampStrength255(strength)
	return clampTriggerStrength48(int(math.Round(float64(strength) * float64(triggerForceMax) / 255.0)))
}

func clampTriggerStrength48(v int) int {
	if v < 1 {
		return 1
	}
	if v > triggerForceMax {
		return triggerForceMax
	}
	return v
}

func referenceFor(profile string, refs map[string]float64, fallback float64) float64 {
	if v, ok := refs[profile]; ok && v > 0 {
		return v
	}
	return fallback
}

func defaultSurfaceStrengths(refs map[string]float64) map[string]int {
	m := make(map[string]int, len(knownSurfaceProfiles))
	for _, name := range knownSurfaceProfiles {
		m[name] = percentToStrength255(referenceFor(name, refs, 100))
	}
	return m
}

func normalizeSurfaceStrengths(src map[string]int, refs map[string]float64) map[string]int {
	next := defaultSurfaceStrengths(refs)
	for _, name := range knownSurfaceProfiles {
		if src != nil {
			if value, ok := src[name]; ok {
				next[name] = clampStrength255(value)
			}
		}
	}
	return next
}

func absoluteGain(strength int, referencePercent float64) float64 {
	if referencePercent <= 0 {
		referencePercent = 100
	}
	referenceStrength := percentToStrength255(referencePercent)
	return float64(clampStrength255(strength)) / float64(referenceStrength)
}

func scaleOldGainToAbsolute(oldStrength int, referencePercent float64) int {
	gain := float64(clampStrength255(oldStrength)) / 255.0
	return percentToStrength255(referencePercent * gain)
}

func convertOldSurfaceGainMap(old map[string]int, refs map[string]float64) map[string]int {
	next := defaultSurfaceStrengths(refs)
	for _, name := range knownSurfaceProfiles {
		oldStrength := 255
		if old != nil {
			if v, ok := old[name]; ok {
				oldStrength = clampStrength255(v)
			}
		}
		next[name] = scaleOldGainToAbsolute(oldStrength, referenceFor(name, refs, 100))
	}
	return next
}

// V1.4.6 stored master/surface/profile/LED values as relative gains where 100%
// meant "keep calibration". Convert those gains to the new absolute percentage
// labels so the physical feel remains unchanged after upgrade.
func rescaleAbsoluteReference(oldStrength int, oldReference, newReference float64) int {
	gain := absoluteGain(oldStrength, oldReference)
	return percentToStrength255(clamp(newReference*gain, 1, 100))
}

func convertSurfaceReferenceMap(old map[string]int, oldRefs, newRefs map[string]float64) map[string]int {
	next := defaultSurfaceStrengths(newRefs)
	for _, name := range knownSurfaceProfiles {
		oldRef := referenceFor(name, oldRefs, 100)
		newRef := referenceFor(name, newRefs, 100)
		oldValue := percentToStrength255(oldRef)
		if old != nil {
			if v, ok := old[name]; ok {
				oldValue = clampStrength255(v)
			}
		}
		next[name] = rescaleAbsoluteReference(oldValue, oldRef, newRef)
	}
	return next
}

func applyLegacyTriggerRanges255(s *userSettings, legacyL2, legacyR2 int) {
	if legacyL2 <= 0 {
		legacyL2 = 128
	}
	legacyL2 = clampStrength255(legacyL2)
	start := clampStrength255(int(math.Round(float64(legacyL2) * 32.0 / 128.0)))
	if start > legacyL2 {
		start = legacyL2
	}
	s.AdaptiveTriggers.L2BrakeStartStrength = start
	s.AdaptiveTriggers.L2BrakeEndStrength = legacyL2
	s.AdaptiveTriggers.L2BrakeStrength = legacyL2
	if legacyR2 <= 0 {
		legacyR2 = 32
	}
	legacyR2 = clampStrength255(legacyR2)
	s.AdaptiveTriggers.R2ThrottleStartStrength = legacyR2
	s.AdaptiveTriggers.R2ThrottleEndStrength = legacyR2
	s.AdaptiveTriggers.R2ThrottleStrength = legacyR2
}

// V1.4.9 used one normal-strength value per trigger. Preserve that feel while
// expanding it into explicit start/end resistance values.
func migrateSchema10UserSettings(old userSettings) userSettings {
	old.Schema = userSettingsSchema
	old.AdaptiveTriggers.L2BrakeStartStrength = legacyTriggerStrength255To48(old.AdaptiveTriggers.L2BrakeStartStrength)
	old.AdaptiveTriggers.L2BrakeEndStrength = legacyTriggerStrength255To48(old.AdaptiveTriggers.L2BrakeEndStrength)
	old.AdaptiveTriggers.L2BrakeStrength = legacyTriggerStrength255To48(old.AdaptiveTriggers.L2BrakeStrength)
	old.AdaptiveTriggers.ABSStrength = legacyTriggerStrength255To48(old.AdaptiveTriggers.ABSStrength)
	old.AdaptiveTriggers.R2ThrottleStartStrength = legacyTriggerStrength255To48(old.AdaptiveTriggers.R2ThrottleStartStrength)
	old.AdaptiveTriggers.R2ThrottleEndStrength = legacyTriggerStrength255To48(old.AdaptiveTriggers.R2ThrottleEndStrength)
	old.AdaptiveTriggers.R2ThrottleStrength = legacyTriggerStrength255To48(old.AdaptiveTriggers.R2ThrottleStrength)
	old.AdaptiveTriggers.R2EffectsStrength = legacyTriggerStrength255To48(old.AdaptiveTriggers.R2EffectsStrength)
	return normalizeUserSettings(old)
}

func migrateSchema9UserSettings(old userSettings) userSettings {
	legacyL2, legacyR2 := old.AdaptiveTriggers.L2BrakeStrength, old.AdaptiveTriggers.R2ThrottleStrength
	applyLegacyTriggerRanges255(&old, legacyL2, legacyR2)
	old.Schema = 10
	return migrateSchema10UserSettings(old)
}

// V1.4.8 stored the current calibrated gain under different display
// references. Convert every value through its old gain so upgrading to schema 9
// cannot change the physical feel, including custom user values.
func migrateSchema8UserSettings(old userSettings) userSettings {
	next := defaultUserSettings()
	next.Haptics.MasterEnabled = old.Haptics.MasterEnabled
	next.Haptics.MasterStrength = old.Haptics.MasterStrength // 100% was already gain 1.0
	next.Haptics.SurfaceEnabled = old.Haptics.SurfaceEnabled
	next.Haptics.SurfaceStrength = rescaleAbsoluteReference(old.Haptics.SurfaceStrength, 100, surfaceMasterReferencePercent)
	next.Haptics.ImpactEnabled = old.Haptics.ImpactEnabled
	next.Haptics.ImpactStrength = old.Haptics.ImpactStrength
	next.Haptics.SurfaceRollingStrengths = convertSurfaceReferenceMap(old.Haptics.SurfaceRollingStrengths, v148SurfaceRollingReferencePercent, surfaceRollingReferencePercent)
	next.Haptics.SurfaceSlipStrengths = convertSurfaceReferenceMap(old.Haptics.SurfaceSlipStrengths, v148SurfaceSlipReferencePercent, surfaceSlipReferencePercent)
	next.AdaptiveTriggers = old.AdaptiveTriggers
	applyLegacyTriggerRanges255(&next, old.AdaptiveTriggers.L2BrakeStrength, old.AdaptiveTriggers.R2ThrottleStrength)
	next.Lighting = old.Lighting
	next.Schema = 10
	return migrateSchema10UserSettings(next)
}

// V1.4.7 used absolute-looking labels, but master/surface references were not
// true final-output percentages and slip displayed an intermediate coefficient.
// Translate every stored value through its old gain so the physical default (and
// any user attenuation/boost) is preserved while the new labels are truthful.
func migrateSchema7UserSettings(old userSettings) userSettings {
	next := defaultUserSettings()
	next.Haptics.MasterEnabled = old.Haptics.MasterEnabled
	next.Haptics.MasterStrength = rescaleAbsoluteReference(old.Haptics.MasterStrength, 65, hapticReferencePercent)
	next.Haptics.SurfaceEnabled = old.Haptics.SurfaceEnabled
	next.Haptics.SurfaceStrength = rescaleAbsoluteReference(old.Haptics.SurfaceStrength, 88, surfaceMasterReferencePercent)
	next.Haptics.ImpactEnabled = old.Haptics.ImpactEnabled
	next.Haptics.ImpactStrength = clampStrength255(old.Haptics.ImpactStrength)
	next.Haptics.SurfaceRollingStrengths = convertSurfaceReferenceMap(old.Haptics.SurfaceRollingStrengths, v147SurfaceRollingReferencePercent, surfaceRollingReferencePercent)
	next.Haptics.SurfaceSlipStrengths = convertSurfaceReferenceMap(old.Haptics.SurfaceSlipStrengths, v147SurfaceSlipReferencePercent, surfaceSlipReferencePercent)
	next.AdaptiveTriggers = old.AdaptiveTriggers
	applyLegacyTriggerRanges255(&next, old.AdaptiveTriggers.L2BrakeStrength, old.AdaptiveTriggers.R2ThrottleStrength)
	next.Lighting = old.Lighting
	next.Schema = 10
	return migrateSchema10UserSettings(next)
}

// V1.4.6 stored 0..255 relative gains: 255 meant "keep calibration". Translate
// those gains directly into the current absolute/relative labels.
func migrateSchema6UserSettings(old userSettings) userSettings {
	next := defaultUserSettings()
	next.Haptics.MasterEnabled = old.Haptics.MasterEnabled
	next.Haptics.MasterStrength = scaleOldGainToAbsolute(old.Haptics.MasterStrength, hapticReferencePercent)
	next.Haptics.SurfaceEnabled = old.Haptics.SurfaceEnabled
	next.Haptics.SurfaceStrength = scaleOldGainToAbsolute(old.Haptics.SurfaceStrength, surfaceMasterReferencePercent)
	next.Haptics.ImpactEnabled = old.Haptics.ImpactEnabled
	next.Haptics.ImpactStrength = clampStrength255(old.Haptics.ImpactStrength)
	next.Haptics.SurfaceRollingStrengths = convertOldSurfaceGainMap(old.Haptics.SurfaceRollingStrengths, surfaceRollingReferencePercent)
	next.Haptics.SurfaceSlipStrengths = convertOldSurfaceGainMap(old.Haptics.SurfaceSlipStrengths, surfaceSlipReferencePercent)
	next.AdaptiveTriggers = old.AdaptiveTriggers
	applyLegacyTriggerRanges255(&next, old.AdaptiveTriggers.L2BrakeStrength, old.AdaptiveTriggers.R2ThrottleStrength)
	next.Lighting.Enabled = old.Lighting.Enabled
	next.Lighting.Brightness = scaleOldGainToAbsolute(old.Lighting.Brightness, ledReferencePercent)
	next.Schema = 10
	return migrateSchema10UserSettings(next)
}

// Reproduce the V1.4.6 relative-gain representation for older schema-3/4/5
// files, then translate it into the current schema.
func migratePre6UserSettings(old userSettings) userSettings {
	stage := old
	stage.Schema = 6
	// Older master: 65% was the calibrated output.
	p := strength255ToPercent(old.Haptics.MasterStrength)
	masterGain := 1.0
	if p <= 1.0 {
		masterGain = 0.05
	} else if p <= 65.0 {
		masterGain = 0.05 + (1.0-0.05)*((p-1.0)/64.0)
	}
	stage.Haptics.MasterStrength = clampStrength255(int(math.Round(clamp(masterGain, 1.0/255.0, 1.0) * 255.0)))
	// Older road master: 88% was calibrated gain 1.0.
	oldSurface := float64(clampStrength255(old.Haptics.SurfaceStrength)) / 255.0
	stage.Haptics.SurfaceStrength = clampStrength255(int(math.Round(clamp(oldSurface/0.88, 1.0/255.0, 1.0) * 255.0)))
	// Older per-material field used the V1.4.7 intermediate rolling references.
	legacy := make(map[string]int, len(knownSurfaceProfiles))
	for _, name := range knownSurfaceProfiles {
		ref := referenceFor(name, v147SurfaceRollingReferencePercent, 100)
		value := percentToStrength255(ref)
		if old.Haptics.SurfaceProfileStrengths != nil {
			if v, ok := old.Haptics.SurfaceProfileStrengths[name]; ok {
				value = clampStrength255(v)
			}
		}
		gain := (float64(value) / 255.0) / (ref / 100.0)
		legacy[name] = clampStrength255(int(math.Round(clamp(gain, 1.0/255.0, 1.0) * 255.0)))
	}
	stage.Haptics.SurfaceRollingStrengths = legacy
	stage.Haptics.SurfaceSlipStrengths = legacy
	stage.Lighting.Enabled, stage.Lighting.Brightness = true, 255
	applyLegacyTriggerRanges255(&stage, stage.AdaptiveTriggers.L2BrakeStrength, stage.AdaptiveTriggers.R2ThrottleStrength)
	return migrateSchema6UserSettings(stage)
}

func defaultUserSettings() userSettings {
	var s userSettings
	s.Schema = userSettingsSchema
	s.Haptics.MasterEnabled, s.Haptics.MasterStrength = true, percentToStrength255(hapticReferencePercent)
	s.Haptics.SurfaceEnabled, s.Haptics.SurfaceStrength = true, percentToStrength255(surfaceMasterReferencePercent)
	s.Haptics.ImpactEnabled, s.Haptics.ImpactStrength = true, 255
	s.Haptics.SurfaceProfileStrengths = nil
	s.Haptics.SurfaceRollingStrengths = defaultSurfaceStrengths(surfaceRollingReferencePercent)
	s.Haptics.SurfaceSlipStrengths = defaultSurfaceStrengths(surfaceSlipReferencePercent)
	s.AdaptiveTriggers.L2BrakeEnabled = true
	s.AdaptiveTriggers.L2BrakeStartStrength = percentToTriggerForce48(13)
	s.AdaptiveTriggers.L2BrakeEndStrength = percentToTriggerForce48(50)
	s.AdaptiveTriggers.L2BrakeStrength = s.AdaptiveTriggers.L2BrakeEndStrength
	s.AdaptiveTriggers.ABSEnabled, s.AdaptiveTriggers.ABSStrength = true, percentToTriggerForce48(75)
	s.AdaptiveTriggers.R2ThrottleEnabled = true
	s.AdaptiveTriggers.R2ThrottleStartStrength = percentToTriggerForce48(13)
	s.AdaptiveTriggers.R2ThrottleEndStrength = percentToTriggerForce48(13)
	s.AdaptiveTriggers.R2ThrottleStrength = s.AdaptiveTriggers.R2ThrottleStartStrength
	s.AdaptiveTriggers.R2EffectsEnabled, s.AdaptiveTriggers.R2EffectsStrength = true, percentToTriggerForce48(r2DynamicReferencePercent)
	s.Lighting.Enabled, s.Lighting.Brightness = true, percentToStrength255(ledReferencePercent)
	return s
}

func clampStrength255(v int) int {
	if v < 1 {
		return 1
	}
	if v > 255 {
		return 255
	}
	return v
}

func clampLegacyPercent(v int) int {
	if v < 0 {
		return 0
	}
	if v > 200 {
		return 200
	}
	return v
}

func migrateLegacySetting(percent, baseline int) (bool, int) {
	percent = clampLegacyPercent(percent)
	if percent <= 0 {
		return false, baseline
	}
	return true, clampStrength255(int(math.Round(float64(baseline) * float64(percent) / 100.0)))
}

func migrateLegacyUserSettings(old legacyUserSettings) userSettings {
	s := defaultUserSettings()
	// Earliest user percentages were multipliers around the calibrated defaults.
	s.Haptics.MasterEnabled = old.Haptics.MasterPercent > 0
	s.Haptics.MasterStrength = percentToStrength255(hapticReferencePercent * float64(clampLegacyPercent(old.Haptics.MasterPercent)) / 100.0)
	s.Haptics.SurfaceEnabled = old.Haptics.SurfacePercent > 0
	s.Haptics.SurfaceStrength = percentToStrength255(surfaceMasterReferencePercent * float64(clampLegacyPercent(old.Haptics.SurfacePercent)) / 100.0)
	s.Haptics.ImpactEnabled, s.Haptics.ImpactStrength = migrateLegacySetting(old.Haptics.ImpactPercent, 255)
	s.AdaptiveTriggers.L2BrakeEnabled, s.AdaptiveTriggers.L2BrakeStrength = migrateLegacySetting(old.AdaptiveTriggers.L2BrakePercent, 128)
	s.AdaptiveTriggers.ABSEnabled, s.AdaptiveTriggers.ABSStrength = migrateLegacySetting(old.AdaptiveTriggers.ABSPercent, 192)
	s.AdaptiveTriggers.R2ThrottleEnabled, s.AdaptiveTriggers.R2ThrottleStrength = migrateLegacySetting(old.AdaptiveTriggers.R2ThrottlePercent, 32)
	s.AdaptiveTriggers.R2EffectsEnabled, s.AdaptiveTriggers.R2EffectsStrength = migrateLegacySetting(old.AdaptiveTriggers.R2EffectsPercent, legacyR2DynamicPeakByte)
	applyLegacyTriggerRanges255(&s, s.AdaptiveTriggers.L2BrakeStrength, s.AdaptiveTriggers.R2ThrottleStrength)
	s.Schema = 10
	return migrateSchema10UserSettings(s)
}

func normalizeUserSettings(s userSettings) userSettings {
	if s.Schema != userSettingsSchema {
		return defaultUserSettings()
	}
	s.Haptics.MasterStrength = clampStrength255(s.Haptics.MasterStrength)
	s.Haptics.SurfaceStrength = clampStrength255(s.Haptics.SurfaceStrength)
	s.Haptics.ImpactStrength = clampStrength255(s.Haptics.ImpactStrength)
	s.Haptics.SurfaceRollingStrengths = normalizeSurfaceStrengths(s.Haptics.SurfaceRollingStrengths, surfaceRollingReferencePercent)
	s.Haptics.SurfaceSlipStrengths = normalizeSurfaceStrengths(s.Haptics.SurfaceSlipStrengths, surfaceSlipReferencePercent)
	if s.AdaptiveTriggers.L2BrakeEndStrength <= 0 {
		end := s.AdaptiveTriggers.L2BrakeStrength
		if end <= 0 {
			end = percentToTriggerForce48(50)
		}
		s.AdaptiveTriggers.L2BrakeEndStrength = clampTriggerStrength48(end)
		s.AdaptiveTriggers.L2BrakeStartStrength = clampTriggerStrength48(int(math.Round(float64(s.AdaptiveTriggers.L2BrakeEndStrength) * 0.25)))
	}
	s.AdaptiveTriggers.L2BrakeStartStrength = clampTriggerStrength48(s.AdaptiveTriggers.L2BrakeStartStrength)
	s.AdaptiveTriggers.L2BrakeEndStrength = clampTriggerStrength48(s.AdaptiveTriggers.L2BrakeEndStrength)
	s.AdaptiveTriggers.L2BrakeStrength = s.AdaptiveTriggers.L2BrakeEndStrength
	s.AdaptiveTriggers.ABSStrength = clampTriggerStrength48(s.AdaptiveTriggers.ABSStrength)
	if s.AdaptiveTriggers.R2ThrottleEndStrength <= 0 {
		base := s.AdaptiveTriggers.R2ThrottleStrength
		if base <= 0 {
			base = percentToTriggerForce48(13)
		}
		base = clampTriggerStrength48(base)
		s.AdaptiveTriggers.R2ThrottleStartStrength, s.AdaptiveTriggers.R2ThrottleEndStrength = base, base
	}
	s.AdaptiveTriggers.R2ThrottleStartStrength = clampTriggerStrength48(s.AdaptiveTriggers.R2ThrottleStartStrength)
	s.AdaptiveTriggers.R2ThrottleEndStrength = clampTriggerStrength48(s.AdaptiveTriggers.R2ThrottleEndStrength)
	s.AdaptiveTriggers.R2ThrottleStrength = s.AdaptiveTriggers.R2ThrottleStartStrength
	s.AdaptiveTriggers.R2EffectsStrength = clampTriggerStrength48(s.AdaptiveTriggers.R2EffectsStrength)
	s.Lighting.Brightness = clampStrength255(s.Lighting.Brightness)
	return s
}
