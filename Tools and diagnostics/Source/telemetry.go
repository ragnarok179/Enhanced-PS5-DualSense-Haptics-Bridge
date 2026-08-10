package main

import "encoding/json"

// BeamNG telemetry is intentionally transport-neutral. USB and Bluetooth consume
// the same decoded state through the Common Feel Engine.

type rawTelemetry struct {
	Speed                      float64 `json:"speed"`
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

type telemetry struct {
	Version        int  `json:"v"`
	Active         bool `json:"active"`
	Seq            int  `json:"seq"`
	ShiftLEDsInUse bool `json:"shiftLEDsInUse"`

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
	if json.Unmarshal(data, &t) != nil || t.Version != protocolVersion {
		return telemetry{}, false
	}
	return t, true
}
