package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync"
)

const userSettingsSchema = 11

type userSettings struct {
	Schema  int `json:"schema"`
	Haptics struct {
		MasterEnabled           bool           `json:"master_enabled"`
		MasterStrength          int            `json:"master_strength"`
		SurfaceEnabled          bool           `json:"surface_enabled"`
		SurfaceStrength         int            `json:"surface_strength"`
		ImpactEnabled           bool           `json:"impact_enabled"`
		ImpactStrength          int            `json:"impact_strength"`
		SurfaceProfileStrengths map[string]int `json:"surface_profile_strengths,omitempty"`
		SurfaceRollingStrengths map[string]int `json:"surface_rolling_strengths"`
		SurfaceSlipStrengths    map[string]int `json:"surface_slip_strengths"`
	} `json:"haptics"`
	AdaptiveTriggers struct {
		L2BrakeEnabled          bool `json:"l2_brake_enabled"`
		L2BrakeStartStrength    int  `json:"l2_brake_start_strength"`
		L2BrakeEndStrength      int  `json:"l2_brake_end_strength"`
		L2BrakeStrength         int  `json:"l2_brake_strength,omitempty"`
		ABSEnabled              bool `json:"abs_enabled"`
		ABSStrength             int  `json:"abs_strength"`
		R2ThrottleEnabled       bool `json:"r2_throttle_enabled"`
		R2ThrottleStartStrength int  `json:"r2_throttle_start_strength"`
		R2ThrottleEndStrength   int  `json:"r2_throttle_end_strength"`
		R2ThrottleStrength      int  `json:"r2_throttle_strength,omitempty"`
		R2EffectsEnabled        bool `json:"r2_effects_enabled"`
		R2EffectsStrength       int  `json:"r2_effects_strength"`
	} `json:"adaptive_triggers"`
	Lighting struct {
		Enabled    bool `json:"enabled"`
		Brightness int  `json:"brightness"`
	} `json:"lighting"`
}

type legacyUserSettings struct {
	Schema  int `json:"schema"`
	Haptics struct {
		MasterPercent  int `json:"master_percent"`
		SurfacePercent int `json:"surface_percent"`
		ImpactPercent  int `json:"impact_percent"`
	} `json:"haptics"`
	AdaptiveTriggers struct {
		L2BrakePercent    int `json:"l2_brake_percent"`
		ABSPercent        int `json:"abs_percent"`
		R2ThrottlePercent int `json:"r2_throttle_percent"`
		R2EffectsPercent  int `json:"r2_effects_percent"`
	} `json:"adaptive_triggers"`
}

var userSettingsState struct {
	once sync.Once
	mu   sync.RWMutex
	path string
	data userSettings
}

func hapticMasterGain(enabled bool, strength int) float64 {
	if !enabled {
		return 0
	}
	return absoluteGain(strength, hapticReferencePercent)
}

func surfaceMasterGain(settings userSettings) float64 {
	if !settings.Haptics.SurfaceEnabled {
		return 0
	}
	return absoluteGain(settings.Haptics.SurfaceStrength, surfaceMasterReferencePercent)
}

func surfaceRollingGain(settings userSettings, profile string) float64 {
	if settings.Haptics.SurfaceRollingStrengths == nil {
		return 1
	}
	value, ok := settings.Haptics.SurfaceRollingStrengths[profile]
	if !ok {
		return 1
	}
	ref := referenceFor(profile, surfaceRollingReferencePercent, 100)
	refStrength := percentToStrength255(ref)
	value = clampStrength255(value)
	baseGain := float64(value) / float64(refStrength)
	if value <= refStrength || refStrength >= 255 {
		return baseGain
	}
	progress := clamp(float64(value-refStrength)/float64(255-refStrength), 0, 1)
	boost := 1.0
	if v, ok := surfaceRollingFullScaleBoost[profile]; ok && v > 1 {
		boost = 1 + (v-1)*progress
	}
	return baseGain * boost
}

func surfaceSlipGain(settings userSettings, profile string) float64 {
	if settings.Haptics.SurfaceSlipStrengths == nil {
		return 1
	}
	value, ok := settings.Haptics.SurfaceSlipStrengths[profile]
	if !ok {
		return 1
	}
	ref := referenceFor(profile, surfaceSlipReferencePercent, 1)
	refStrength := percentToStrength255(ref)
	value = clampStrength255(value)
	baseGain := float64(value) / float64(refStrength)
	if value <= refStrength || refStrength >= 255 {
		return baseGain
	}
	progress := clamp(float64(value-refStrength)/float64(255-refStrength), 0, 1)
	boost := 1.0
	if v, ok := surfaceSlipFullScaleBoost[profile]; ok && v > 0 {
		boost = 1 + (v-1)*progress
	}
	return baseGain * boost
}

func userSettingsFilePath() string {
	if exe, err := os.Executable(); err == nil {
		bridgeDir := filepath.Dir(exe)
		return filepath.Clean(filepath.Join(bridgeDir, "..", "Config", "user_settings.json"))
	}
	if cwd, err := os.Getwd(); err == nil {
		return filepath.Join(cwd, "user_settings.json")
	}
	return "user_settings.json"
}

func ensureUserSettings() {
	userSettingsState.once.Do(func() {
		settings := defaultUserSettings()
		path := userSettingsFilePath()
		if data, err := os.ReadFile(path); err == nil {
			var header struct {
				Schema int `json:"schema"`
			}
			if json.Unmarshal(data, &header) == nil {
				switch header.Schema {
				case userSettingsSchema:
					loaded := settings
					if json.Unmarshal(data, &loaded) == nil {
						settings = normalizeUserSettings(loaded)
					}
				case 10:
					loaded := settings
					if json.Unmarshal(data, &loaded) == nil {
						settings = migrateSchema10UserSettings(loaded)
					}
				case 9:
					loaded := settings
					if json.Unmarshal(data, &loaded) == nil {
						settings = migrateSchema9UserSettings(loaded)
					}
				case 8:
					loaded := settings
					if json.Unmarshal(data, &loaded) == nil {
						settings = migrateSchema8UserSettings(loaded)
					}
				case 7:
					loaded := settings
					if json.Unmarshal(data, &loaded) == nil {
						settings = migrateSchema7UserSettings(loaded)
					}
				case 6:
					loaded := settings
					if json.Unmarshal(data, &loaded) == nil {
						settings = migrateSchema6UserSettings(loaded)
					}
				case 3, 4, 5:
					loaded := settings
					if json.Unmarshal(data, &loaded) == nil {
						settings = migratePre6UserSettings(loaded)
					}
				default:
					var old legacyUserSettings
					if json.Unmarshal(data, &old) == nil {
						settings = migrateLegacyUserSettings(old)
					}
				}
			}
		}
		userSettingsState.mu.Lock()
		userSettingsState.path = path
		userSettingsState.data = settings
		userSettingsState.mu.Unlock()
	})
}

func currentUserSettings() userSettings {
	ensureUserSettings()
	userSettingsState.mu.RLock()
	defer userSettingsState.mu.RUnlock()
	return userSettingsState.data
}

func saveUserSettings() error {
	ensureUserSettings()
	userSettingsState.mu.RLock()
	path := userSettingsState.path
	settings := userSettingsState.data
	userSettingsState.mu.RUnlock()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

const userSettingCount = 9

func userSettingName(index int) string {
	switch index {
	case 0:
		return "Haptics - master"
	case 1:
		return "Haptics - road surfaces"
	case 2:
		return "Haptics - bumps / impacts"
	case 3:
		return "L2 - start resistance"
	case 4:
		return "L2 - end resistance"
	case 5:
		return "L2 - ABS pulse"
	case 6:
		return "R2 - start resistance"
	case 7:
		return "R2 - end resistance"
	case 8:
		return "R2 - dynamic effects"
	default:
		return "Unknown"
	}
}

func userSettingValue(index int, s userSettings) int {
	switch index {
	case 0:
		return s.Haptics.MasterStrength
	case 1:
		return s.Haptics.SurfaceStrength
	case 2:
		return s.Haptics.ImpactStrength
	case 3:
		return s.AdaptiveTriggers.L2BrakeStartStrength
	case 4:
		return s.AdaptiveTriggers.L2BrakeEndStrength
	case 5:
		return s.AdaptiveTriggers.ABSStrength
	case 6:
		return s.AdaptiveTriggers.R2ThrottleStartStrength
	case 7:
		return s.AdaptiveTriggers.R2ThrottleEndStrength
	case 8:
		return s.AdaptiveTriggers.R2EffectsStrength
	default:
		return 1
	}
}

func userSettingMax(index int) int {
	if index >= 3 && index <= 8 {
		return triggerForceMax
	}
	return 255
}

func clampUserSettingValue(index, value int) int {
	if index >= 3 && index <= 8 {
		return clampTriggerStrength48(value)
	}
	return clampStrength255(value)
}

func userSettingPercent(index, value int) float64 {
	return float64(clampUserSettingValue(index, value)) * 100.0 / float64(userSettingMax(index))
}

func userSettingEnabled(index int, s userSettings) bool {
	switch index {
	case 0:
		return s.Haptics.MasterEnabled
	case 1:
		return s.Haptics.SurfaceEnabled
	case 2:
		return s.Haptics.ImpactEnabled
	case 3, 4:
		return s.AdaptiveTriggers.L2BrakeEnabled
	case 5:
		return s.AdaptiveTriggers.ABSEnabled
	case 6, 7:
		return s.AdaptiveTriggers.R2ThrottleEnabled
	case 8:
		return s.AdaptiveTriggers.R2EffectsEnabled
	default:
		return false
	}
}

func setUserSettingValue(index, value int) {
	ensureUserSettings()
	value = clampUserSettingValue(index, value)
	userSettingsState.mu.Lock()
	s := userSettingsState.data
	switch index {
	case 0:
		s.Haptics.MasterStrength = value
	case 1:
		s.Haptics.SurfaceStrength = value
	case 2:
		s.Haptics.ImpactStrength = value
	case 3:
		s.AdaptiveTriggers.L2BrakeStartStrength = value
	case 4:
		s.AdaptiveTriggers.L2BrakeEndStrength = value
	case 5:
		s.AdaptiveTriggers.ABSStrength = value
	case 6:
		s.AdaptiveTriggers.R2ThrottleStartStrength = value
	case 7:
		s.AdaptiveTriggers.R2ThrottleEndStrength = value
	case 8:
		s.AdaptiveTriggers.R2EffectsStrength = value
	}
	s.AdaptiveTriggers.L2BrakeStrength = s.AdaptiveTriggers.L2BrakeEndStrength
	s.AdaptiveTriggers.R2ThrottleStrength = s.AdaptiveTriggers.R2ThrottleStartStrength
	userSettingsState.data = s
	userSettingsState.mu.Unlock()
}

func setUserSettingEnabled(index int, enabled bool) {
	ensureUserSettings()
	userSettingsState.mu.Lock()
	s := userSettingsState.data
	switch index {
	case 0:
		s.Haptics.MasterEnabled = enabled
	case 1:
		s.Haptics.SurfaceEnabled = enabled
	case 2:
		s.Haptics.ImpactEnabled = enabled
	case 3, 4:
		s.AdaptiveTriggers.L2BrakeEnabled = enabled
	case 5:
		s.AdaptiveTriggers.ABSEnabled = enabled
	case 6, 7:
		s.AdaptiveTriggers.R2ThrottleEnabled = enabled
	case 8:
		s.AdaptiveTriggers.R2EffectsEnabled = enabled
	}
	userSettingsState.data = s
	userSettingsState.mu.Unlock()
}

func adjustUserSetting(index, delta int) int {
	s := currentUserSettings()
	value := clampUserSettingValue(index, userSettingValue(index, s)+delta)
	setUserSettingValue(index, value)
	return value
}

func toggleUserSetting(index int) bool {
	s := currentUserSettings()
	next := !userSettingEnabled(index, s)
	setUserSettingEnabled(index, next)
	return next
}

func resetUserSettings() {
	ensureUserSettings()
	userSettingsState.mu.Lock()
	userSettingsState.data = defaultUserSettings()
	userSettingsState.mu.Unlock()
}

func feedbackGain(enabled bool, strength int) float64 {
	if !enabled {
		return 0
	}
	return float64(clampStrength255(strength)) / 255.0
}

func applyUserLEDSettings(rgb [3]byte) [3]byte {
	settings := currentUserSettings()
	if !settings.Lighting.Enabled {
		return [3]byte{}
	}
	gain := absoluteGain(settings.Lighting.Brightness, ledReferencePercent)
	for i := range rgb {
		rgb[i] = byte(clampInt(int(math.Round(float64(rgb[i])*gain)), 0, 255))
	}
	return rgb
}

func settingForce(enabled bool, strength int) triggerForce {
	if !enabled {
		return 0
	}
	return force48FromLevel(clampTriggerStrength48(strength))
}

func configuredL2StartForce(settings userSettings) triggerForce {
	if !settings.AdaptiveTriggers.L2BrakeEnabled {
		return 0
	}
	return settingForce(true, settings.AdaptiveTriggers.L2BrakeStartStrength)
}

func configuredL2EndForce(settings userSettings) triggerForce {
	if !settings.AdaptiveTriggers.L2BrakeEnabled {
		return 0
	}
	return settingForce(true, settings.AdaptiveTriggers.L2BrakeEndStrength)
}

func configuredR2StartForce(settings userSettings) triggerForce {
	if !settings.AdaptiveTriggers.R2ThrottleEnabled {
		return 0
	}
	return settingForce(true, settings.AdaptiveTriggers.R2ThrottleStartStrength)
}

func configuredR2EndForce(settings userSettings) triggerForce {
	if !settings.AdaptiveTriggers.R2ThrottleEnabled {
		return 0
	}
	return settingForce(true, settings.AdaptiveTriggers.R2ThrottleEndStrength)
}

func normalL2Effect(t telemetry, settings userSettings) triggerEffect {
	if !settings.AdaptiveTriggers.L2BrakeEnabled {
		return offTrigger()
	}
	start := configuredL2StartForce(settings)
	end := configuredL2EndForce(settings)
	if t.Raw == nil || t.Raw.Brake < 0.005 {
		end = start
	}
	return resistanceTrigger(0, start.Float64(), end.Float64())
}

func normalR2Effect(settings userSettings) triggerEffect {
	if !settings.AdaptiveTriggers.R2ThrottleEnabled {
		return offTrigger()
	}
	return resistanceTrigger(0, configuredR2StartForce(settings).Float64(), configuredR2EndForce(settings).Float64())
}

// Compatibility wrappers for tests/diagnostics. They route through the normalized
// trigger model and only mirror hardware-like integers at the boundary.
func r2IsDynamic(t telemetry, effect triggerEffect) bool {
	if effect.Kind == triggerVibration || effect.Kind == triggerFine {
		return true
	}
	if t.Raw == nil {
		return false
	}
	return t.Raw.RevLimiter || t.Raw.TCS || t.Raw.TCSRaw || t.Raw.Wheelspin || t.Raw.Shifting || t.Raw.Airborne
}

func applyUserTriggerPreferencesPair(t telemetry, pair triggerPair, absActive bool) triggerPair {
	settings := currentUserSettings()

	if !absActive {
		switch pair.L2.Kind {
		case triggerResistance, triggerVibration:
			pair.L2 = normalL2Effect(t, settings)
		case triggerFine:
			if !settings.AdaptiveTriggers.L2BrakeEnabled {
				pair.L2 = offTrigger()
			} else {
				baseline := unit(feelProfile().Triggers.L2.NormalEndForce)
				scale := 1.0
				if baseline > 0 {
					scale = configuredL2EndForce(settings).Float64() / baseline.Float64()
				}
				pair.L2.StartForce = force48(pair.L2.StartForce.Float64() * scale)
				pair.L2.EndForce = pair.L2.StartForce
			}
		}
	}

	if !settings.AdaptiveTriggers.R2EffectsEnabled && r2IsDynamic(t, pair.R2) {
		pair.R2 = normalR2Effect(settings)
	} else if pair.R2.Kind == triggerResistance {
		pair.R2 = normalR2Effect(settings)
	}
	return pair
}

// Compatibility wrapper used by existing diagnostics/tests. Runtime gameplay
// uses the normalized triggerPair and mirrors it into telemetry only at the
// boundary.
// applyBeamNGUserSettings mirrors the persistent in-game settings sent by the
// BeamNG mod. Telemetry updates memory only; the BeamNG settings file is the
// source of truth while a vehicle is connected.
func applyBeamNGUserSettings(remote telemetryUserSettings) {
	ensureUserSettings()
	next := defaultUserSettings()

	if remote.Schema >= 11 {
		next.Haptics.MasterEnabled = remote.MasterEnabled
		next.Haptics.MasterStrength = clampStrength255(remote.MasterStrength)
		next.Haptics.SurfaceEnabled = remote.SurfaceEnabled
		next.Haptics.SurfaceStrength = clampStrength255(remote.SurfaceStrength)
		next.Haptics.ImpactEnabled = remote.ImpactEnabled
		next.Haptics.ImpactStrength = clampStrength255(remote.ImpactStrength)
		next.Haptics.SurfaceRollingStrengths = normalizeSurfaceStrengths(remote.SurfaceRollingStrengths, surfaceRollingReferencePercent)
		next.Haptics.SurfaceSlipStrengths = normalizeSurfaceStrengths(remote.SurfaceSlipStrengths, surfaceSlipReferencePercent)
		next.AdaptiveTriggers.L2BrakeEnabled = remote.L2BrakeEnabled
		next.AdaptiveTriggers.L2BrakeStartStrength = clampTriggerStrength48(remote.L2BrakeStartStrength)
		next.AdaptiveTriggers.L2BrakeEndStrength = clampTriggerStrength48(remote.L2BrakeEndStrength)
		next.AdaptiveTriggers.L2BrakeStrength = next.AdaptiveTriggers.L2BrakeEndStrength
		next.AdaptiveTriggers.ABSEnabled = remote.ABSEnabled
		next.AdaptiveTriggers.ABSStrength = clampTriggerStrength48(remote.ABSStrength)
		next.AdaptiveTriggers.R2ThrottleEnabled = remote.R2ThrottleEnabled
		next.AdaptiveTriggers.R2ThrottleStartStrength = clampTriggerStrength48(remote.R2ThrottleStartStrength)
		next.AdaptiveTriggers.R2ThrottleEndStrength = clampTriggerStrength48(remote.R2ThrottleEndStrength)
		next.AdaptiveTriggers.R2ThrottleStrength = next.AdaptiveTriggers.R2ThrottleStartStrength
		next.AdaptiveTriggers.R2EffectsEnabled = remote.R2EffectsEnabled
		next.AdaptiveTriggers.R2EffectsStrength = clampTriggerStrength48(remote.R2EffectsStrength)
		next.Lighting.Enabled = remote.LightingEnabled
		next.Lighting.Brightness = clampStrength255(remote.LightingBrightness)
	} else if remote.Schema == 10 {
		stage := defaultUserSettings()
		stage.Schema = 10
		stage.Haptics.MasterEnabled, stage.Haptics.MasterStrength = remote.MasterEnabled, remote.MasterStrength
		stage.Haptics.SurfaceEnabled, stage.Haptics.SurfaceStrength = remote.SurfaceEnabled, remote.SurfaceStrength
		stage.Haptics.ImpactEnabled, stage.Haptics.ImpactStrength = remote.ImpactEnabled, remote.ImpactStrength
		stage.Haptics.SurfaceRollingStrengths = remote.SurfaceRollingStrengths
		stage.Haptics.SurfaceSlipStrengths = remote.SurfaceSlipStrengths
		stage.AdaptiveTriggers.L2BrakeEnabled = remote.L2BrakeEnabled
		stage.AdaptiveTriggers.L2BrakeStartStrength = remote.L2BrakeStartStrength
		stage.AdaptiveTriggers.L2BrakeEndStrength = remote.L2BrakeEndStrength
		stage.AdaptiveTriggers.L2BrakeStrength = remote.L2BrakeStrength
		stage.AdaptiveTriggers.ABSEnabled, stage.AdaptiveTriggers.ABSStrength = remote.ABSEnabled, remote.ABSStrength
		stage.AdaptiveTriggers.R2ThrottleEnabled = remote.R2ThrottleEnabled
		stage.AdaptiveTriggers.R2ThrottleStartStrength = remote.R2ThrottleStartStrength
		stage.AdaptiveTriggers.R2ThrottleEndStrength = remote.R2ThrottleEndStrength
		stage.AdaptiveTriggers.R2ThrottleStrength = remote.R2ThrottleStrength
		stage.AdaptiveTriggers.R2EffectsEnabled, stage.AdaptiveTriggers.R2EffectsStrength = remote.R2EffectsEnabled, remote.R2EffectsStrength
		stage.Lighting.Enabled, stage.Lighting.Brightness = remote.LightingEnabled, remote.LightingBrightness
		next = migrateSchema10UserSettings(stage)
	} else if remote.Schema == 9 {
		stage := defaultUserSettings()
		stage.Schema = 9
		stage.Haptics.MasterEnabled, stage.Haptics.MasterStrength = remote.MasterEnabled, remote.MasterStrength
		stage.Haptics.SurfaceEnabled, stage.Haptics.SurfaceStrength = remote.SurfaceEnabled, remote.SurfaceStrength
		stage.Haptics.ImpactEnabled, stage.Haptics.ImpactStrength = remote.ImpactEnabled, remote.ImpactStrength
		stage.Haptics.SurfaceRollingStrengths = remote.SurfaceRollingStrengths
		stage.Haptics.SurfaceSlipStrengths = remote.SurfaceSlipStrengths
		stage.AdaptiveTriggers.L2BrakeEnabled, stage.AdaptiveTriggers.L2BrakeStrength = remote.L2BrakeEnabled, remote.L2BrakeStrength
		stage.AdaptiveTriggers.ABSEnabled, stage.AdaptiveTriggers.ABSStrength = remote.ABSEnabled, remote.ABSStrength
		stage.AdaptiveTriggers.R2ThrottleEnabled, stage.AdaptiveTriggers.R2ThrottleStrength = remote.R2ThrottleEnabled, remote.R2ThrottleStrength
		stage.AdaptiveTriggers.R2EffectsEnabled, stage.AdaptiveTriggers.R2EffectsStrength = remote.R2EffectsEnabled, remote.R2EffectsStrength
		stage.Lighting.Enabled, stage.Lighting.Brightness = remote.LightingEnabled, remote.LightingBrightness
		next = migrateSchema9UserSettings(stage)
	} else if remote.Schema == 8 {
		stage := defaultUserSettings()
		stage.Schema = 8
		stage.Haptics.MasterEnabled, stage.Haptics.MasterStrength = remote.MasterEnabled, remote.MasterStrength
		stage.Haptics.SurfaceEnabled, stage.Haptics.SurfaceStrength = remote.SurfaceEnabled, remote.SurfaceStrength
		stage.Haptics.ImpactEnabled, stage.Haptics.ImpactStrength = remote.ImpactEnabled, remote.ImpactStrength
		stage.Haptics.SurfaceRollingStrengths = remote.SurfaceRollingStrengths
		stage.Haptics.SurfaceSlipStrengths = remote.SurfaceSlipStrengths
		stage.AdaptiveTriggers.L2BrakeEnabled, stage.AdaptiveTriggers.L2BrakeStrength = remote.L2BrakeEnabled, remote.L2BrakeStrength
		stage.AdaptiveTriggers.ABSEnabled, stage.AdaptiveTriggers.ABSStrength = remote.ABSEnabled, remote.ABSStrength
		stage.AdaptiveTriggers.R2ThrottleEnabled, stage.AdaptiveTriggers.R2ThrottleStrength = remote.R2ThrottleEnabled, remote.R2ThrottleStrength
		stage.AdaptiveTriggers.R2EffectsEnabled, stage.AdaptiveTriggers.R2EffectsStrength = remote.R2EffectsEnabled, remote.R2EffectsStrength
		stage.Lighting.Enabled, stage.Lighting.Brightness = remote.LightingEnabled, remote.LightingBrightness
		next = migrateSchema8UserSettings(stage)
	} else if remote.Schema == 7 {
		stage := defaultUserSettings()
		stage.Schema = 7
		stage.Haptics.MasterEnabled, stage.Haptics.MasterStrength = remote.MasterEnabled, remote.MasterStrength
		stage.Haptics.SurfaceEnabled, stage.Haptics.SurfaceStrength = remote.SurfaceEnabled, remote.SurfaceStrength
		stage.Haptics.ImpactEnabled, stage.Haptics.ImpactStrength = remote.ImpactEnabled, remote.ImpactStrength
		stage.Haptics.SurfaceRollingStrengths = remote.SurfaceRollingStrengths
		stage.Haptics.SurfaceSlipStrengths = remote.SurfaceSlipStrengths
		stage.AdaptiveTriggers.L2BrakeEnabled, stage.AdaptiveTriggers.L2BrakeStrength = remote.L2BrakeEnabled, remote.L2BrakeStrength
		stage.AdaptiveTriggers.ABSEnabled, stage.AdaptiveTriggers.ABSStrength = remote.ABSEnabled, remote.ABSStrength
		stage.AdaptiveTriggers.R2ThrottleEnabled, stage.AdaptiveTriggers.R2ThrottleStrength = remote.R2ThrottleEnabled, remote.R2ThrottleStrength
		stage.AdaptiveTriggers.R2EffectsEnabled, stage.AdaptiveTriggers.R2EffectsStrength = remote.R2EffectsEnabled, remote.R2EffectsStrength
		stage.Lighting.Enabled, stage.Lighting.Brightness = remote.LightingEnabled, remote.LightingBrightness
		next = migrateSchema7UserSettings(stage)
	} else if remote.Schema == 6 {
		// V1.4.6 wire values were relative gains for haptics/surfaces/LEDs.
		next.Haptics.MasterEnabled = remote.MasterEnabled
		next.Haptics.MasterStrength = scaleOldGainToAbsolute(remote.MasterStrength, hapticReferencePercent)
		next.Haptics.SurfaceEnabled = remote.SurfaceEnabled
		next.Haptics.SurfaceStrength = scaleOldGainToAbsolute(remote.SurfaceStrength, surfaceMasterReferencePercent)
		next.Haptics.ImpactEnabled = remote.ImpactEnabled
		next.Haptics.ImpactStrength = clampStrength255(remote.ImpactStrength)
		next.Haptics.SurfaceRollingStrengths = convertOldSurfaceGainMap(remote.SurfaceRollingStrengths, surfaceRollingReferencePercent)
		next.Haptics.SurfaceSlipStrengths = convertOldSurfaceGainMap(remote.SurfaceSlipStrengths, surfaceSlipReferencePercent)
		next.AdaptiveTriggers.L2BrakeEnabled = remote.L2BrakeEnabled
		next.AdaptiveTriggers.L2BrakeStrength = clampStrength255(remote.L2BrakeStrength)
		next.AdaptiveTriggers.ABSEnabled = remote.ABSEnabled
		next.AdaptiveTriggers.ABSStrength = clampStrength255(remote.ABSStrength)
		next.AdaptiveTriggers.R2ThrottleEnabled = remote.R2ThrottleEnabled
		next.AdaptiveTriggers.R2ThrottleStrength = clampStrength255(remote.R2ThrottleStrength)
		next.AdaptiveTriggers.R2EffectsEnabled = remote.R2EffectsEnabled
		next.AdaptiveTriggers.R2EffectsStrength = clampStrength255(remote.R2EffectsStrength)
		next.Lighting.Enabled = remote.LightingEnabled
		next.Lighting.Brightness = scaleOldGainToAbsolute(remote.LightingBrightness, ledReferencePercent)
	} else if remote.Schema >= 5 {
		// Compatibility with the short-lived pre-V1.4.6 absolute-reference wire format.
		stage := defaultUserSettings()
		stage.Schema = 6
		stage.Haptics.MasterEnabled = remote.MasterEnabled
		stage.Haptics.MasterStrength = remote.MasterStrength
		stage.Haptics.SurfaceEnabled = remote.SurfaceEnabled
		stage.Haptics.SurfaceStrength = remote.SurfaceStrength
		stage.Haptics.ImpactEnabled = remote.ImpactEnabled
		stage.Haptics.ImpactStrength = remote.ImpactStrength
		stage.Haptics.SurfaceProfileStrengths = remote.SurfaceProfileStrengths
		stage.Haptics.SurfaceRollingStrengths = remote.SurfaceRollingStrengths
		stage.Haptics.SurfaceSlipStrengths = remote.SurfaceSlipStrengths
		stage.AdaptiveTriggers.L2BrakeEnabled, stage.AdaptiveTriggers.L2BrakeStrength = remote.L2BrakeEnabled, remote.L2BrakeStrength
		stage.AdaptiveTriggers.ABSEnabled, stage.AdaptiveTriggers.ABSStrength = remote.ABSEnabled, remote.ABSStrength
		stage.AdaptiveTriggers.R2ThrottleEnabled, stage.AdaptiveTriggers.R2ThrottleStrength = remote.R2ThrottleEnabled, remote.R2ThrottleStrength
		stage.AdaptiveTriggers.R2EffectsEnabled, stage.AdaptiveTriggers.R2EffectsStrength = remote.R2EffectsEnabled, remote.R2EffectsStrength
		stage.Lighting.Enabled, stage.Lighting.Brightness = remote.LightingEnabled, remote.LightingBrightness
		next = migratePre6UserSettings(stage)
	} else if remote.Schema >= 2 {
		// Older 0..255 values were multipliers around calibrated defaults.
		next.Haptics.MasterEnabled = remote.MasterEnabled
		next.Haptics.MasterStrength = scaleOldGainToAbsolute(remote.MasterStrength, hapticReferencePercent)
		next.Haptics.SurfaceEnabled = remote.SurfaceEnabled
		next.Haptics.SurfaceStrength = scaleOldGainToAbsolute(remote.SurfaceStrength, surfaceMasterReferencePercent)
		next.Haptics.ImpactEnabled, next.Haptics.ImpactStrength = remote.ImpactEnabled, clampStrength255(remote.ImpactStrength)
		next.AdaptiveTriggers.L2BrakeEnabled, next.AdaptiveTriggers.L2BrakeStrength = remote.L2BrakeEnabled, clampStrength255(remote.L2BrakeStrength)
		next.AdaptiveTriggers.ABSEnabled, next.AdaptiveTriggers.ABSStrength = remote.ABSEnabled, clampStrength255(remote.ABSStrength)
		next.AdaptiveTriggers.R2ThrottleEnabled, next.AdaptiveTriggers.R2ThrottleStrength = remote.R2ThrottleEnabled, clampStrength255(remote.R2ThrottleStrength)
		next.AdaptiveTriggers.R2EffectsEnabled, next.AdaptiveTriggers.R2EffectsStrength = remote.R2EffectsEnabled, clampStrength255(remote.R2EffectsStrength)
	} else {
		legacy := legacyUserSettings{}
		legacy.Haptics.MasterPercent = remote.MasterPercent
		legacy.Haptics.SurfacePercent = remote.SurfacePercent
		legacy.Haptics.ImpactPercent = remote.ImpactPercent
		legacy.AdaptiveTriggers.L2BrakePercent = remote.L2BrakePercent
		legacy.AdaptiveTriggers.ABSPercent = remote.ABSPercent
		legacy.AdaptiveTriggers.R2ThrottlePercent = remote.R2ThrottlePercent
		legacy.AdaptiveTriggers.R2EffectsPercent = remote.R2EffectsPercent
		next = migrateLegacyUserSettings(legacy)
	}

	if remote.Schema == 6 || (remote.Schema >= 2 && remote.Schema < 5) {
		applyLegacyTriggerRanges255(&next, next.AdaptiveTriggers.L2BrakeStrength, next.AdaptiveTriggers.R2ThrottleStrength)
		next.Schema = 10
		next = migrateSchema10UserSettings(next)
	}
	userSettingsState.mu.Lock()
	userSettingsState.data = normalizeUserSettings(next)
	userSettingsState.mu.Unlock()
}

func onOff(v bool) string {
	if v {
		return "on"
	}
	return "off"
}

func userSettingsSummary() string {
	s := currentUserSettings()
	percent255 := func(v int) int { return int(math.Round(float64(clampStrength255(v)) * 100.0 / 255.0)) }
	percent48 := func(v int) int {
		return int(math.Round(float64(clampTriggerStrength48(v)) * 100.0 / float64(triggerForceMax)))
	}
	return fmt.Sprintf("haptics=%s/%d%% surface=%s/%d%% impacts=%s/%d%% L2=%s/%d%% ABS=%s/%d%% R2=%s/%d%% effects=%s/%d%%",
		onOff(s.Haptics.MasterEnabled), percent255(s.Haptics.MasterStrength),
		onOff(s.Haptics.SurfaceEnabled), percent255(s.Haptics.SurfaceStrength),
		onOff(s.Haptics.ImpactEnabled), percent255(s.Haptics.ImpactStrength),
		onOff(s.AdaptiveTriggers.L2BrakeEnabled), percent48(s.AdaptiveTriggers.L2BrakeStrength),
		onOff(s.AdaptiveTriggers.ABSEnabled), percent48(s.AdaptiveTriggers.ABSStrength),
		onOff(s.AdaptiveTriggers.R2ThrottleEnabled), percent48(s.AdaptiveTriggers.R2ThrottleStrength),
		onOff(s.AdaptiveTriggers.R2EffectsEnabled), percent48(s.AdaptiveTriggers.R2EffectsStrength))
}
