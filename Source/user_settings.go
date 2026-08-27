package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync"
)

const userSettingsSchema = 3

type userSettings struct {
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

func defaultUserSettings() userSettings {
	var s userSettings
	s.Schema = userSettingsSchema
	s.Haptics.MasterPercent = 100
	s.Haptics.SurfacePercent = 100
	s.Haptics.ImpactPercent = 100
	s.AdaptiveTriggers.L2BrakePercent = 100
	s.AdaptiveTriggers.ABSPercent = 100
	s.AdaptiveTriggers.R2ThrottlePercent = 100
	s.AdaptiveTriggers.R2EffectsPercent = 100
	return s
}

func clampPercent(v int) int {
	if v < 0 {
		return 0
	}
	if v > 200 {
		return 200
	}
	return v
}

func normalizeUserSettings(s userSettings) userSettings {
	if s.Schema != userSettingsSchema {
		return defaultUserSettings()
	}
	s.Haptics.MasterPercent = clampPercent(s.Haptics.MasterPercent)
	s.Haptics.SurfacePercent = clampPercent(s.Haptics.SurfacePercent)
	s.Haptics.ImpactPercent = clampPercent(s.Haptics.ImpactPercent)
	s.AdaptiveTriggers.L2BrakePercent = clampPercent(s.AdaptiveTriggers.L2BrakePercent)
	s.AdaptiveTriggers.ABSPercent = clampPercent(s.AdaptiveTriggers.ABSPercent)
	s.AdaptiveTriggers.R2ThrottlePercent = clampPercent(s.AdaptiveTriggers.R2ThrottlePercent)
	s.AdaptiveTriggers.R2EffectsPercent = clampPercent(s.AdaptiveTriggers.R2EffectsPercent)
	return s
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
			loaded := settings
			if json.Unmarshal(data, &loaded) == nil && (loaded.Schema == 1 || loaded.Schema == 2 || loaded.Schema == userSettingsSchema) {
				loaded.Schema = userSettingsSchema
				settings = normalizeUserSettings(loaded)
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

func userSettingsPath() string {
	ensureUserSettings()
	userSettingsState.mu.RLock()
	defer userSettingsState.mu.RUnlock()
	return userSettingsState.path
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
	// Settings are tiny and written only from the interactive menu. Write the
	// final file directly so repeated saves also work on Windows, where os.Rename
	// does not replace an existing destination atomically.
	return os.WriteFile(path, data, 0o644)
}

const userSettingCount = 7

func userSettingName(index int) string {
	switch index {
	case 0:
		return "Haptics - master"
	case 1:
		return "Haptics - road surfaces"
	case 2:
		return "Haptics - bumps / impacts"
	case 3:
		return "L2 - brake resistance"
	case 4:
		return "L2 - ABS pulse"
	case 5:
		return "R2 - throttle resistance"
	case 6:
		return "R2 - dynamic effects"
	default:
		return "Unknown"
	}
}

func userSettingValue(index int, s userSettings) int {
	switch index {
	case 0:
		return s.Haptics.MasterPercent
	case 1:
		return s.Haptics.SurfacePercent
	case 2:
		return s.Haptics.ImpactPercent
	case 3:
		return s.AdaptiveTriggers.L2BrakePercent
	case 4:
		return s.AdaptiveTriggers.ABSPercent
	case 5:
		return s.AdaptiveTriggers.R2ThrottlePercent
	case 6:
		return s.AdaptiveTriggers.R2EffectsPercent
	default:
		return 0
	}
}

func setUserSettingValue(index, value int) {
	ensureUserSettings()
	value = clampPercent(value)
	userSettingsState.mu.Lock()
	s := userSettingsState.data
	switch index {
	case 0:
		s.Haptics.MasterPercent = value
	case 1:
		s.Haptics.SurfacePercent = value
	case 2:
		s.Haptics.ImpactPercent = value
	case 3:
		s.AdaptiveTriggers.L2BrakePercent = value
	case 4:
		s.AdaptiveTriggers.ABSPercent = value
	case 5:
		s.AdaptiveTriggers.R2ThrottlePercent = value
	case 6:
		s.AdaptiveTriggers.R2EffectsPercent = value
	}
	userSettingsState.data = s
	userSettingsState.mu.Unlock()
}

func adjustUserSetting(index, delta int) int {
	s := currentUserSettings()
	value := userSettingValue(index, s) + delta
	value = clampPercent(value)
	setUserSettingValue(index, value)
	return value
}

func resetUserSettings() {
	ensureUserSettings()
	userSettingsState.mu.Lock()
	userSettingsState.data = defaultUserSettings()
	userSettingsState.mu.Unlock()
}

func percentGain(v int) float64 {
	return float64(clampPercent(v)) / 100.0
}

func scaleNormalized255(v, percent int) int {
	if v <= 0 || percent <= 0 {
		return 0
	}
	return clampInt(int(math.Round(float64(v)*percentGain(percent))), 0, 255)
}

func scaleFineStrength(v, percent int) int {
	if v <= 0 || percent <= 0 {
		return 0
	}
	return clampInt(int(math.Round(float64(v)*percentGain(percent))), 0, 48)
}

func applyUserHapticMaster(samples []int8) {
	if len(samples) == 0 {
		return
	}
	gain := percentGain(currentUserSettings().Haptics.MasterPercent)
	if math.Abs(gain-1) < 0.000001 {
		return
	}
	for i, sample := range samples {
		v := float64(sample) / 127.0
		samples[i] = int8(math.Round(clamp(v*gain, -0.99, 0.99) * 127))
	}
}

func applyUserTriggerPreferences(t telemetry, absActive bool) telemetry {
	settings := currentUserSettings()

	// ABS builds its own pulse pattern and already applies both the
	// brake and ABS percentages inside applyABSHybridPulse.
	if !absActive && t.L2Mode == 1 {
		t.L2StartStrength = scaleNormalized255(t.L2StartStrength, settings.AdaptiveTriggers.L2BrakePercent)
		t.L2EndStrength = scaleNormalized255(t.L2EndStrength, settings.AdaptiveTriggers.L2BrakePercent)
		if t.L2StartStrength == 0 || t.L2EndStrength == 0 {
			t.L2Mode = 0
		}
	}

	// Normal throttle uses the official progressive-feedback mode. Fine/vibration
	// R2 effects are adjustable separately, but the exact airborne 1-byte release
	// remains untouched so the validated airborne behavior cannot drift.
	if t.R2Mode == 1 {
		t.R2StartStrength = scaleNormalized255(t.R2StartStrength, settings.AdaptiveTriggers.R2ThrottlePercent)
		t.R2EndStrength = scaleNormalized255(t.R2EndStrength, settings.AdaptiveTriggers.R2ThrottlePercent)
		if t.R2StartStrength == 0 || t.R2EndStrength == 0 {
			t.R2Mode = 0
		}
	} else if t.Raw == nil || !t.Raw.Airborne {
		if t.R2Mode == 2 {
			t.R2Amplitude = scaleNormalized255(t.R2Amplitude, settings.AdaptiveTriggers.R2EffectsPercent)
			if t.R2Amplitude == 0 {
				t.R2Mode = 0
			}
		} else if t.R2Mode == 3 {
			t.R2StartStrength = scaleFineStrength(t.R2StartStrength, settings.AdaptiveTriggers.R2EffectsPercent)
			t.R2EndStrength = t.R2StartStrength
			if t.R2StartStrength == 0 {
				t.R2Mode = 0
			}
		}
	}
	return t
}

// applyBeamNGUserSettings mirrors the persistent in-game settings sent by the
// BeamNG mod. It intentionally updates memory only: telemetry arrives many
// times per second, so writing user_settings.json here would create needless
// disk I/O. The BeamNG settings file is the source of truth while a vehicle is
// connected; a local file, when present, is treated only as migration/support state.
func strength255ToPercent(v int) int {
	v = clampInt(v, 0, 255)
	return clampPercent(int(math.Round(float64(v) * 100.0 / 255.0)))
}

func force48ToPercent(v int) int {
	v = clampInt(v, 0, 48)
	return clampPercent(int(math.Round(float64(v) * 100.0 / 48.0)))
}

func applyBeamNGUserSettings(remote telemetryUserSettings) {
	ensureUserSettings()
	next := currentUserSettings()

	if remote.Schema >= 11 {
		// V1.1/V1.2 settings packets use absolute 0..255 haptic strengths and
		// a 0..48 trigger-force lattice. The semantic l2Effect/r2Effect packet
		// already contains the chosen normal trigger forces, so the legacy
		// percentage multipliers must stay at unity to avoid scaling them twice.
		if remote.MasterEnabled {
			next.Haptics.MasterPercent = strength255ToPercent(remote.MasterStrength)
		} else {
			next.Haptics.MasterPercent = 0
		}
		if remote.SurfaceEnabled {
			next.Haptics.SurfacePercent = strength255ToPercent(remote.SurfaceStrength)
		} else {
			next.Haptics.SurfacePercent = 0
		}
		if remote.ImpactEnabled {
			next.Haptics.ImpactPercent = strength255ToPercent(remote.ImpactStrength)
		} else {
			next.Haptics.ImpactPercent = 0
		}

		if remote.L2BrakeEnabled {
			next.AdaptiveTriggers.L2BrakePercent = 100
		} else {
			next.AdaptiveTriggers.L2BrakePercent = 0
		}
		if remote.ABSEnabled {
			next.AdaptiveTriggers.ABSPercent = force48ToPercent(remote.ABSStrength)
		} else {
			next.AdaptiveTriggers.ABSPercent = 0
		}
		if remote.R2ThrottleEnabled {
			next.AdaptiveTriggers.R2ThrottlePercent = 100
		} else {
			next.AdaptiveTriggers.R2ThrottlePercent = 0
		}
		if remote.R2EffectsEnabled {
			next.AdaptiveTriggers.R2EffectsPercent = 100
		} else {
			next.AdaptiveTriggers.R2EffectsPercent = 0
		}
	} else {
		// Real legacy packets use the historical percentage fields directly.
		next.Haptics.MasterPercent = clampPercent(remote.MasterPercent)
		next.Haptics.SurfacePercent = clampPercent(remote.SurfacePercent)
		next.Haptics.ImpactPercent = clampPercent(remote.ImpactPercent)
		next.AdaptiveTriggers.L2BrakePercent = clampPercent(remote.L2BrakePercent)
		next.AdaptiveTriggers.ABSPercent = clampPercent(remote.ABSPercent)
		next.AdaptiveTriggers.R2ThrottlePercent = clampPercent(remote.R2ThrottlePercent)
		next.AdaptiveTriggers.R2EffectsPercent = clampPercent(remote.R2EffectsPercent)
	}

	userSettingsState.mu.Lock()
	userSettingsState.data = next
	userSettingsState.mu.Unlock()
}

func userSettingsSummary() string {
	s := currentUserSettings()
	return fmt.Sprintf("haptics=%d%% surface=%d%% impacts=%d%% L2=%d%% ABS=%d%% R2=%d%% effects=%d%%",
		s.Haptics.MasterPercent, s.Haptics.SurfacePercent, s.Haptics.ImpactPercent,
		s.AdaptiveTriggers.L2BrakePercent, s.AdaptiveTriggers.ABSPercent,
		s.AdaptiveTriggers.R2ThrottlePercent, s.AdaptiveTriggers.R2EffectsPercent)
}
