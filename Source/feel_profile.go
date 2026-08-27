package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type cooldownPair [2]int

type feelProfileConfig struct {
	Schema         int    `json:"schema"`
	ProfileVersion string `json:"profile_version"`
	Reference      string `json:"reference"`
	Surface        struct {
		LowSpeed struct {
			MinSpeedMS              float64 `json:"min_speed_ms"`
			FullSpeedMS             float64 `json:"full_speed_ms"`
			MinAmplitudeScale       float64 `json:"min_amplitude_scale"`
			AmplitudeExponent       float64 `json:"amplitude_exponent"`
			CadenceFullSpeedMS      float64 `json:"cadence_full_speed_ms"`
			CadenceMinScale         float64 `json:"cadence_min_scale"`
			CadenceSpan             float64 `json:"cadence_span"`
			CarrierMinScale         float64 `json:"carrier_min_scale"`
			CarrierSpan             float64 `json:"carrier_span"`
			HighSpeedCarrierBase    float64 `json:"high_speed_carrier_base"`
			HighSpeedCarrierDivisor float64 `json:"high_speed_carrier_divisor"`
			HighSpeedCarrierMax     float64 `json:"high_speed_carrier_max"`
		} `json:"low_speed"`
		SyntheticCooldownMS map[string]cooldownPair `json:"synthetic_cooldown_ms"`
	} `json:"surface"`
	Transport struct {
		USBOutputGain       float64 `json:"usb_output_gain"`
		BluetoothOutputGain float64 `json:"bluetooth_output_gain"`
		BluetoothGainBasis  string  `json:"bluetooth_gain_basis"`
	} `json:"transport"`
	Triggers struct {
		ABS struct {
			KickStrength255 int `json:"kick_strength_255"`
			BaseStrength255 int `json:"base_strength_255"`
		} `json:"abs"`
		R2 struct {
			NormalStartStrength255    int     `json:"normal_start_strength_255"`
			NormalEndStrength255      int     `json:"normal_end_strength_255"`
			WheelspinStartPosition255 float64 `json:"wheelspin_start_position_255"`
			WheelspinEndPosition255   float64 `json:"wheelspin_end_position_255"`
			WheelspinStartStrength255 float64 `json:"wheelspin_start_strength_255"`
			WheelspinEndStrength255   float64 `json:"wheelspin_end_strength_255"`
			ShiftPosition255          int     `json:"shift_position_255"`
			ShiftStrength255          int     `json:"shift_strength_255"`
			ShiftDurationMS           int     `json:"shift_duration_ms"`
		} `json:"r2"`
	} `json:"triggers"`
	ShiftHaptic struct {
		MinStrength   float64 `json:"min_strength"`
		MaxStrength   float64 `json:"max_strength"`
		MinDurationMS int     `json:"min_duration_ms"`
		MaxDurationMS int     `json:"max_duration_ms"`
		Priority      int     `json:"priority"`
		SurfaceDuck   float64 `json:"surface_duck"`
		GlobalDuck    float64 `json:"global_duck"`
	} `json:"shift_haptic"`
	BodyIsolation struct {
		OppositeMergeWindowMS int `json:"opposite_merge_window_ms"`
		Collision             struct {
			HardAttackMS int `json:"hard_attack_ms"`
			ReleaseEndMS int `json:"release_end_ms"`
		} `json:"collision"`
		Landing struct {
			HardAttackMS int `json:"hard_attack_ms"`
			ReleaseEndMS int `json:"release_end_ms"`
		} `json:"landing"`
		SuspensionBump struct {
			HardAttackMS          int `json:"hard_attack_ms"`
			ReleaseEndMS          int `json:"release_end_ms"`
			OppositeMergeWindowMS int `json:"opposite_merge_window_ms"`
		} `json:"suspension_bump"`
	} `json:"body_isolation"`
	LED struct {
		FirstRatio            float64 `json:"first_ratio"`
		OffRatio              float64 `json:"off_ratio"`
		OffHoldMS             int     `json:"off_hold_ms"`
		RedRatio              float64 `json:"red_ratio"`
		RedExitRatio          float64 `json:"red_exit_ratio"`
		BlinkOnlyOnRevLimiter bool    `json:"blink_only_on_rev_limiter"`
		BlinkMinRatio         float64 `json:"blink_min_ratio"`
		BlinkHoldMS           int     `json:"blink_hold_ms"`
		BlinkHz               float64 `json:"blink_hz"`
		MaxBrightness         int     `json:"max_brightness"`
		MinBrightness         int     `json:"min_brightness"`
	} `json:"led"`
}

var (
	sharedFeelProfile     feelProfileConfig
	sharedFeelProfilePath = "built-in defaults"
	sharedFeelProfileHash = ""
	feelProfileOnce       sync.Once
)

func defaultFeelProfile() feelProfileConfig {
	var p feelProfileConfig
	p.Schema = 1
	p.ProfileVersion = "V1.3"
	p.Reference = "V1.3 Common Feel Engine; official trigger strength normalized to 0..255 internally; USB low-latency WASAPI path; Bluetooth derived by 48 kHz to 3 kHz transport adapter"
	p.Surface.LowSpeed.MinSpeedMS = 0.25
	p.Surface.LowSpeed.FullSpeedMS = 6.0
	p.Surface.LowSpeed.MinAmplitudeScale = 0.16
	p.Surface.LowSpeed.AmplitudeExponent = 0.65
	p.Surface.LowSpeed.CadenceFullSpeedMS = 8.0
	p.Surface.LowSpeed.CadenceMinScale = 0.18
	p.Surface.LowSpeed.CadenceSpan = 0.82
	p.Surface.LowSpeed.CarrierMinScale = 0.34
	p.Surface.LowSpeed.CarrierSpan = 0.6957142857
	p.Surface.LowSpeed.HighSpeedCarrierBase = 0.75
	p.Surface.LowSpeed.HighSpeedCarrierDivisor = 28.0
	p.Surface.LowSpeed.HighSpeedCarrierMax = 1.8
	p.Transport.USBOutputGain = 1.0
	p.Transport.BluetoothOutputGain = 0.80
	p.Transport.BluetoothGainBasis = "Controller IMU comparison from paired USB/Bluetooth BeamNG logs; global transport compensation only."
	p.Surface.SyntheticCooldownMS = map[string]cooldownPair{
		"asphalt": {800, 140}, "asphalt_wet": {800, 140}, "slippery": {800, 140}, "ice": {800, 140},
		"sand": {520, 95}, "mud": {520, 95}, "grass": {460, 95}, "snow": {460, 95},
		"dirt": {420, 70}, "dusty_dirt": {420, 70}, "sandy_road": {420, 70}, "gravel": {320, 45},
		"rock": {460, 90}, "cobblestone": {300, 55}, "rumble_strip": {240, 35}, "default": {360, 80},
	}
	// Official feedback strength is kept on a normalized 0..255 scale until
	// final HID quantization. 32 ~= 1/8 and 128 ~= 4/8.
	p.Triggers.ABS.KickStrength255 = 224
	p.Triggers.ABS.BaseStrength255 = 32
	p.Triggers.R2.NormalStartStrength255 = 32
	p.Triggers.R2.NormalEndStrength255 = 32
	p.Triggers.R2.WheelspinStartPosition255 = 26
	p.Triggers.R2.WheelspinEndPosition255 = 144
	p.Triggers.R2.WheelspinStartStrength255 = 3
	p.Triggers.R2.WheelspinEndStrength255 = 1
	p.Triggers.R2.ShiftPosition255 = 0
	p.Triggers.R2.ShiftStrength255 = 1
	p.Triggers.R2.ShiftDurationMS = 150
	p.ShiftHaptic.MinStrength = 0.11
	p.ShiftHaptic.MaxStrength = 0.15
	p.ShiftHaptic.MinDurationMS = 68
	p.ShiftHaptic.MaxDurationMS = 82
	p.ShiftHaptic.Priority = 25
	p.ShiftHaptic.SurfaceDuck = 0.76
	p.ShiftHaptic.GlobalDuck = 0.76
	p.BodyIsolation.OppositeMergeWindowMS = 45
	p.BodyIsolation.Collision.HardAttackMS, p.BodyIsolation.Collision.ReleaseEndMS = 32, 112
	p.BodyIsolation.Landing.HardAttackMS, p.BodyIsolation.Landing.ReleaseEndMS = 28, 96
	p.BodyIsolation.SuspensionBump.HardAttackMS, p.BodyIsolation.SuspensionBump.ReleaseEndMS = 128, 150
	p.BodyIsolation.SuspensionBump.OppositeMergeWindowMS = 8
	p.LED.FirstRatio = 0.70
	p.LED.OffRatio = 0.65
	p.LED.OffHoldMS = 300
	p.LED.RedRatio = 0.95
	p.LED.RedExitRatio = 0.92
	p.LED.BlinkOnlyOnRevLimiter = true
	p.LED.BlinkMinRatio = 0.92
	p.LED.BlinkHoldMS = 180
	p.LED.BlinkHz = 8
	p.LED.MaxBrightness = 220
	p.LED.MinBrightness = 48
	return p
}

func profileCandidatePaths() []string {
	paths := make([]string, 0, 5)
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		paths = append(paths,
			filepath.Join(dir, "config", "feel_profile_v1.json"),
			filepath.Join(dir, "..", "config", "feel_profile_v1.json"),
			filepath.Join(dir, "..", "Common Feel Engine", "feel_profile_v1.json"),
		)
	}
	if cwd, err := os.Getwd(); err == nil {
		paths = append(paths,
			filepath.Join(cwd, "config", "feel_profile_v1.json"),
			filepath.Join(cwd, "feel_profile_v1.json"),
		)
	}
	return paths
}

func ensureFeelProfile() {
	feelProfileOnce.Do(func() {
		sharedFeelProfile = defaultFeelProfile()
		for _, candidate := range profileCandidatePaths() {
			path := filepath.Clean(candidate)
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			loaded := defaultFeelProfile()
			if json.Unmarshal(data, &loaded) != nil || loaded.Schema != 1 || loaded.ProfileVersion == "" {
				continue
			}
			sharedFeelProfile = loaded
			sharedFeelProfilePath = path
			sum := sha256.Sum256(data)
			sharedFeelProfileHash = hex.EncodeToString(sum[:])
			return
		}
	})
}

func feelProfile() *feelProfileConfig {
	ensureFeelProfile()
	return &sharedFeelProfile
}

func feelProfileInfo() (string, string, string) {
	p := feelProfile()
	return p.ProfileVersion, sharedFeelProfilePath, sharedFeelProfileHash
}
