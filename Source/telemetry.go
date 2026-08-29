package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Ragnarok179/enhanced-ps5-dualsense-haptics-bridge/internal/compatibility"
)

// BeamNG telemetry is intentionally transport-neutral. USB and Bluetooth consume
// the same decoded state through the Common Feel Engine.

type rawTelemetry struct {
	Speed                      float64 `json:"speed"`
	Brake                      float64 `json:"brake"`
	Throttle                   float64 `json:"throttle"`
	RPM                        float64 `json:"rpm"`
	MaxRPM                     float64 `json:"maxRPM"`
	EngineRunning              bool    `json:"engineRunning"`
	RevLimiter                 bool    `json:"revLimiter"`
	ABS                        bool    `json:"abs"`
	ABSRaw                     bool    `json:"absRaw"`
	ABSSeverity                float64 `json:"absSeverity"`
	ABSWheelCount              int     `json:"absWheelCount"`
	ABSControlHz               float64 `json:"absControlHz"`
	Lock                       bool    `json:"lock"`
	LockHaptic                 bool    `json:"lockHaptic"`
	LockedWheelCount           int     `json:"lockedWheelCount"`
	Shifting                   bool    `json:"shifting"`
	TCS                        bool    `json:"tcs"`
	TCSRaw                     bool    `json:"tcsRaw"`
	Wheelspin                  bool    `json:"wheelspin"`
	DrivenSlip                 float64 `json:"drivenSlip"`
	Airborne                   bool    `json:"airborne"`
	GroundedWheels             int     `json:"groundedWheels"`
	GroundedLeftWheels         int     `json:"groundedLeftWheels"`
	GroundedRightWheels        int     `json:"groundedRightWheels"`
	SurfaceMaterialL           int     `json:"surfaceMaterialL"`
	SurfaceRoughnessL          float64 `json:"surfaceRoughnessL"`
	SurfaceMaterialR           int     `json:"surfaceMaterialR"`
	SurfaceRoughnessR          float64 `json:"surfaceRoughnessR"`
	RoadExcitationL            float64 `json:"roadExcitationL"`
	RoadExcitationR            float64 `json:"roadExcitationR"`
	RoadSlipL                  float64 `json:"roadSlipL"`
	RoadSlipR                  float64 `json:"roadSlipR"`
	CandidateL                 float64 `json:"candidateL"`
	CandidateR                 float64 `json:"candidateR"`
	PeakImpulseL               float64 `json:"peakImpulseL"`
	PeakImpulseR               float64 `json:"peakImpulseR"`
	PhysicsImpulseL            float64 `json:"physicsImpulseL"`
	PhysicsImpulseR            float64 `json:"physicsImpulseR"`
	PhysicsSamples             int     `json:"physicsSamples"`
	PhysicsHookEnabled         bool    `json:"physicsHookEnabled"`
	NativeBumpCorrReason       string  `json:"nativeBumpCorrelationReason"`
	NativeBumpCorrLeftMS       float64 `json:"nativeBumpCorrLeftMS"`
	NativeBumpCorrRightMS      float64 `json:"nativeBumpCorrRightMS"`
	NativeBumpCorrLeftCue      float64 `json:"nativeBumpCorrLeftCue"`
	NativeBumpCorrRightCue     float64 `json:"nativeBumpCorrRightCue"`
	NativeBumpCorrLeftContact  float64 `json:"nativeBumpCorrLeftContact"`
	NativeBumpCorrRightContact float64 `json:"nativeBumpCorrRightContact"`
	NativeBumpCorrLeftJolt     float64 `json:"nativeBumpCorrLeftJolt"`
	NativeBumpCorrRightJolt    float64 `json:"nativeBumpCorrRightJolt"`
	NativeBumpCorrLeftStress   float64 `json:"nativeBumpCorrLeftStress"`
	NativeBumpCorrRightStress  float64 `json:"nativeBumpCorrRightStress"`
	NativeBumpCorrLeftPeak     float64 `json:"nativeBumpCorrLeftPeak"`
	NativeBumpCorrRightPeak    float64 `json:"nativeBumpCorrRightPeak"`
	NativeBumpSourceConfidence float64 `json:"nativeBumpSourceConfidence"`
	NativeBumpPending          bool    `json:"nativeBumpPending"`
	NativeRumbleBaseForce      float64 `json:"nativeRumbleBaseForce"`
}

type telemetryUserSettings struct {
	Schema            int `json:"schema"`
	TriggerForceScale int `json:"triggerForceScale"`

	MasterEnabled   bool `json:"masterEnabled"`
	MasterStrength  int  `json:"masterStrength"`
	SurfaceEnabled  bool `json:"surfaceEnabled"`
	SurfaceStrength int  `json:"surfaceStrength"`
	ImpactEnabled   bool `json:"impactEnabled"`
	ImpactStrength  int  `json:"impactStrength"`

	L2BrakeEnabled          bool `json:"l2BrakeEnabled"`
	L2BrakeStartStrength    int  `json:"l2BrakeStartStrength"`
	L2BrakeEndStrength      int  `json:"l2BrakeEndStrength"`
	L2BrakeStrength         int  `json:"l2BrakeStrength"`
	ABSEnabled              bool `json:"absEnabled"`
	ABSStrength             int  `json:"absStrength"`
	R2ThrottleEnabled       bool `json:"r2ThrottleEnabled"`
	R2ThrottleStartStrength int  `json:"r2ThrottleStartStrength"`
	R2ThrottleEndStrength   int  `json:"r2ThrottleEndStrength"`
	R2ThrottleStrength      int  `json:"r2ThrottleStrength"`
	R2EffectsEnabled        bool `json:"r2EffectsEnabled"`
	R2EffectsStrength       int  `json:"r2EffectsStrength"`

	LightingEnabled    bool `json:"lightingEnabled"`
	LightingBrightness int  `json:"lightingBrightness"`

	SurfaceRollingStrengths map[string]int `json:"surfaceRollingStrengths"`
	SurfaceSlipStrengths    map[string]int `json:"surfaceSlipStrengths"`

	// Legacy percentage fields are retained for real protocol-40/settings
	// migrations only. V1.2 sends the schema-12 fields above.
	MasterPercent     int `json:"masterPercent"`
	SurfacePercent    int `json:"surfacePercent"`
	ImpactPercent     int `json:"impactPercent"`
	L2BrakePercent    int `json:"l2BrakePercent"`
	ABSPercent        int `json:"absPercent"`
	R2ThrottlePercent int `json:"r2ThrottlePercent"`
	R2EffectsPercent  int `json:"r2EffectsPercent"`
}

type triggerEffectWire struct {
	Kind          string  `json:"kind"`
	StartPosition float64 `json:"startPosition"`
	StartForce    float64 `json:"startForce"`
	EndForce      float64 `json:"endForce"`
	Amplitude     float64 `json:"amplitude"`
	FrequencyHz   float64 `json:"frequencyHz"`
}

type telemetry struct {
	Version        int                    `json:"v"`
	ProtocolID     string                 `json:"protocolId"`
	ModVersion     string                 `json:"modVersion"`
	ProtocolMin    int                    `json:"protocolMin"`
	ProtocolMax    int                    `json:"protocolMax"`
	LegacyCompat   bool                   `json:"legacyCompat"`
	Active         bool                   `json:"active"`
	Seq            int                    `json:"seq"`
	ShiftLEDsInUse bool                   `json:"shiftLEDsInUse"`
	UserSettings   *telemetryUserSettings `json:"userSettings"`
	L2Effect       *triggerEffectWire     `json:"l2Effect"`
	R2Effect       *triggerEffectWire     `json:"r2Effect"`

	L2Mode          int `json:"l2Mode"`
	L2StartZone     int `json:"l2StartZone"`
	L2StartStrength int `json:"l2StartStrength"`
	L2EndStrength   int `json:"l2EndStrength"`
	L2Amplitude     int `json:"l2Amplitude"`
	L2Hz            int `json:"l2Hz"`

	R2Mode                 int           `json:"r2Mode"`
	R2StartZone            int           `json:"r2StartZone"`
	R2StartStrength        int           `json:"r2StartStrength"`
	R2EndStrength          int           `json:"r2EndStrength"`
	R2Amplitude            int           `json:"r2Amplitude"`
	R2Hz                   int           `json:"r2Hz"`
	BodyEvent              int           `json:"bodyEvent"`
	BodyStrength           float64       `json:"bodyStrength"`
	BodyLeftStrength       float64       `json:"bodyLeftStrength"`
	BodyRightStrength      float64       `json:"bodyRightStrength"`
	BodyDurationMS         int           `json:"bodyDurationMS"`
	BodyKind               string        `json:"bodyKind"`
	BodySide               int           `json:"bodySide"`
	BodyProfile            string        `json:"bodyProfile"`
	BodySourceReason       string        `json:"bodySourceReason"`
	BodySourceConfidence   float64       `json:"bodySourceConfidence"`
	BodySourceLeftScore    float64       `json:"bodySourceLeftScore"`
	BodySourceRightScore   float64       `json:"bodySourceRightScore"`
	BodySourceLeftContact  float64       `json:"bodySourceLeftContact"`
	BodySourceRightContact float64       `json:"bodySourceRightContact"`
	BodySourceLeftJolt     float64       `json:"bodySourceLeftJolt"`
	BodySourceRightJolt    float64       `json:"bodySourceRightJolt"`
	BodySourceLeftStress   float64       `json:"bodySourceLeftStress"`
	BodySourceRightStress  float64       `json:"bodySourceRightStress"`
	BodySourceLeftPeak     float64       `json:"bodySourceLeftPeak"`
	BodySourceRightPeak    float64       `json:"bodySourceRightPeak"`
	BodySourceLeftEnergy   float64       `json:"bodySourceLeftEnergy"`
	BodySourceRightEnergy  float64       `json:"bodySourceRightEnergy"`
	BodySourceLeftWheel    int           `json:"bodySourceLeftWheel"`
	BodySourceRightWheel   int           `json:"bodySourceRightWheel"`
	BodySourceLeftAxle     int           `json:"bodySourceLeftAxle"`
	BodySourceRightAxle    int           `json:"bodySourceRightAxle"`
	ShiftEvent             int           `json:"shiftEvent"`
	ShiftStrength          float64       `json:"shiftStrength"`
	ShiftDurationMS        int           `json:"shiftDurationMS"`
	Raw                    *rawTelemetry `json:"raw"`
}

func decodeTelemetry(data []byte) (telemetry, bool) {
	var t telemetry
	if json.Unmarshal(data, &t) != nil {
		return telemetry{}, false
	}
	if t.ProtocolID != "" && t.ProtocolID != protocolID {
		return telemetry{}, false
	}

	switch t.Version {
	case 40:
		// Marked protocol-40 packets are the V1.1 migration mirror that was
		// emitted for old Bridges. V1.4 understands the semantic generations
		// directly, so the mirror must be ignored to prevent duplicated events
		// and state rollback. Mod V1.2/protocol 42 emits no legacy mirror.
		if t.LegacyCompat {
			return telemetry{}, false
		}
		normalizeLegacyOfficialTriggerStrengths(&t)
	case 41:
		// Protocol 41 is the semantic l2Effect/r2Effect generation used by Mod V1.1.
		normalizeSemanticTriggerEffects(&t)
	case 42:
		// Legacy V1.2 semantic generation.
		normalizeSemanticTriggerEffects(&t)
	case 43:
		// V1.41 migration generation. The wire payload remains semantic; from
		// this release onward user-facing compatibility is release-version based.
		normalizeSemanticTriggerEffects(&t)
	default:
		return telemetry{}, false
	}

	if t.UserSettings != nil {
		applyBeamNGUserSettings(*t.UserSettings)
	}
	return t, true
}

func clampUnit(v float64) float64 {
	if v <= 0 {
		return 0
	}
	if v >= 1 {
		return 1
	}
	return v
}

func unitTo255(v float64) int {
	return int(clampUnit(v)*255.0 + 0.5)
}

func unitTo48(v float64) int {
	if v <= 0 {
		return 0
	}
	return clampInt(int(clampUnit(v)*48.0+0.5), 1, 48)
}

func effectToLegacyFields(effect *triggerEffectWire) (mode, zone, start, end, amplitude, hz int) {
	if effect == nil {
		return 0, 0, 0, 0, 0, 0
	}
	switch effect.Kind {
	case "resistance":
		return 1,
			clampInt(int(clampUnit(effect.StartPosition)*9.0+0.5), 0, 9),
			unitTo255(effect.StartForce), unitTo255(effect.EndForce), 0, 0
	case "vibration":
		return 2,
			clampInt(int(clampUnit(effect.StartPosition)*9.0+0.5), 0, 9),
			0, 0, unitTo255(effect.Amplitude), clampInt(int(effect.FrequencyHz+0.5), 0, 255)
	case "fine":
		force := unitTo48(effect.StartForce)
		return 3,
			clampInt(int(clampUnit(effect.StartPosition)*255.0+0.5), 0, 255),
			force, force, 0, 0
	default:
		return 0, 0, 0, 0, 0, 0
	}
}

func normalizeSemanticTriggerEffects(t *telemetry) {
	if t == nil {
		return
	}
	t.L2Mode, t.L2StartZone, t.L2StartStrength, t.L2EndStrength, t.L2Amplitude, t.L2Hz = effectToLegacyFields(t.L2Effect)
	t.R2Mode, t.R2StartZone, t.R2StartStrength, t.R2EndStrength, t.R2Amplitude, t.R2Hz = effectToLegacyFields(t.R2Effect)
}

func normalizeLegacyOfficialTriggerStrengths(t *telemetry) {
	if t == nil {
		return
	}
	if t.L2Mode == 1 {
		t.L2StartStrength = strength8To255(t.L2StartStrength)
		t.L2EndStrength = strength8To255(t.L2EndStrength)
	} else if t.L2Mode == 2 {
		t.L2Amplitude = strength8To255(t.L2Amplitude)
	}
	if t.R2Mode == 1 {
		t.R2StartStrength = strength8To255(t.R2StartStrength)
		t.R2EndStrength = strength8To255(t.R2EndStrength)
	} else if t.R2Mode == 2 {
		t.R2Amplitude = strength8To255(t.R2Amplitude)
	}
}

type telemetryHeader struct {
	Version      int    `json:"v"`
	ProtocolID   string `json:"protocolId"`
	ModVersion   string `json:"modVersion"`
	ProtocolMin  int    `json:"protocolMin"`
	ProtocolMax  int    `json:"protocolMax"`
	LegacyCompat bool   `json:"legacyCompat"`
}

func inspectTelemetryHeader(data []byte) (telemetryHeader, bool) {
	var h telemetryHeader
	if json.Unmarshal(data, &h) != nil || h.Version == 0 {
		return telemetryHeader{}, false
	}
	if h.ProtocolID != "" && h.ProtocolID != protocolID {
		return telemetryHeader{}, false
	}
	return h, true
}

func telemetryCompatibilityError(data []byte) string {
	h, ok := inspectTelemetryHeader(data)
	if !ok || h.LegacyCompat || gameplayProtocolSupported(h.Version) {
		return ""
	}
	mod := h.ModVersion
	if mod == "" {
		mod = "unknown"
	}
	return fmt.Sprintf("BeamNG mod %s uses gameplay protocol %d; Bridge %s supports official protocols %d-%d. Update the mod or Bridge so their protocol ranges overlap.",
		mod, h.Version, bridgeDisplayVersion, protocolMinVersion, protocolMaxVersion)
}

func telemetryConnectionSummary(t telemetry) string {
	mod := strings.TrimSpace(t.ModVersion)
	if mod == "" {
		return fmt.Sprintf("BeamNG.drive connected - legacy mod - wire generation %d.", t.Version)
	}
	if compatibility.CompareVersions(mod, bridgeDisplayVersion) == 0 {
		return fmt.Sprintf("BeamNG.drive connected - release %s synchronized.", bridgeDisplayVersion)
	}
	return fmt.Sprintf("BeamNG.drive connected - mod %s / Bridge %s.", mod, bridgeDisplayVersion)
}
