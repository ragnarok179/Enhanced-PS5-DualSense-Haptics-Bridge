package main

import (
	"encoding/json"

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
	RoadRollingExcitationValid bool    `json:"roadRollingExcitationValid"`
	RoadRollingExcitationL     float64 `json:"roadRollingExcitationL"`
	RoadRollingExcitationR     float64 `json:"roadRollingExcitationR"`
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
	Schema                  int            `json:"schema"`
	TriggerForceScale       int            `json:"triggerForceScale"`
	SurfaceProfileStrengths map[string]int `json:"surfaceProfileStrengths"`
	SurfaceRollingStrengths map[string]int `json:"surfaceRollingStrengths"`
	SurfaceSlipStrengths    map[string]int `json:"surfaceSlipStrengths"`

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
	LightingEnabled         bool `json:"lightingEnabled"`
	LightingBrightness      int  `json:"lightingBrightness"`

	// Historical percentage fields are decoded only for old v40/settings-schema
	// migrations. Current protocol v41 does not emit them.
	MasterPercent     int `json:"masterPercent"`
	SurfacePercent    int `json:"surfacePercent"`
	ImpactPercent     int `json:"impactPercent"`
	L2BrakePercent    int `json:"l2BrakePercent"`
	ABSPercent        int `json:"absPercent"`
	R2ThrottlePercent int `json:"r2ThrottlePercent"`
	R2EffectsPercent  int `json:"r2EffectsPercent"`
}

type telemetry struct {
	ProtocolID     string                 `json:"protocolId"`
	Version        int                    `json:"v"`
	ModVersion     string                 `json:"modVersion"`
	ProtocolMin    int                    `json:"protocolMin"`
	ProtocolMax    int                    `json:"protocolMax"`
	LegacyCompat   bool                   `json:"legacyCompat"`
	Active         bool                   `json:"active"`
	Seq            int                    `json:"seq"`
	ShiftLEDsInUse bool                   `json:"shiftLEDsInUse"`
	UserSettings   *telemetryUserSettings `json:"userSettings"`

	L2Effect *wireTriggerEffect `json:"l2Effect"`
	R2Effect *wireTriggerEffect `json:"r2Effect"`

	// Legacy v40 trigger fields are decode/debug compatibility only. Protocol v41
	// sends L2Effect/R2Effect using normalized values; active force is canonical
	// 0..48 inside the Bridge.
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
	if json.Unmarshal(data, &t) != nil || !compatibility.ProtocolSupported(t.Version) {
		return telemetry{}, false
	}

	// Trigger encodings are normalized later by triggerPairFromTelemetry().
	// Keeping the legacy wire bytes untouched here makes the protocol adapter
	// explicit and prevents hardware units from leaking into gameplay logic.
	if t.UserSettings != nil {
		applyBeamNGUserSettings(*t.UserSettings)
	}
	return t, true
}

// shouldConsumeTelemetry prevents the temporary V1.1 legacy-v40 mirror from
// being processed twice by Bridge V1.3. Real V1.0 packets do not carry the
// legacyCompat marker and remain fully supported through the v40 adapter.
func shouldConsumeTelemetry(t telemetry) bool {
	return !(t.Version == legacyProtocolVersion && t.LegacyCompat)
}
