package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Ragnarok179/enhanced-ps5-dualsense-haptics-bridge/internal/compatibility"
)

func TestInspectTelemetryEnvelope(t *testing.T) {
	envelope, ok := inspectTelemetryEnvelope([]byte(`{"protocolId":"DPH","v":42,"modVersion":"V1.6.0","protocolMin":41,"protocolMax":42}`))
	if !ok || envelope.Version != 42 || envelope.ModVersion != "V1.6.0" || !isProjectTelemetryEnvelope(envelope) {
		t.Fatalf("unexpected envelope: %+v ok=%v", envelope, ok)
	}
}

func TestCompiledProtocolCompatibility(t *testing.T) {
	guard := &protocolCompatibilityGuard{}
	if !guard.supportsEnvelope(telemetryEnvelope{Project: true, Version: 40}) {
		t.Fatal("legacy protocol 40 should remain supported")
	}
	if !guard.supportsEnvelope(telemetryEnvelope{Project: true, Version: 41}) {
		t.Fatal("current protocol 41 should be supported")
	}
	if guard.supportsEnvelope(telemetryEnvelope{Project: true, Version: 42}) {
		t.Fatal("future protocol 42 must require a compatible Bridge")
	}
}

func TestUnrelatedJSONDoesNotTriggerCompatibilityPrompt(t *testing.T) {
	envelope, ok := inspectTelemetryEnvelope([]byte(`{"v":999,"foo":"bar"}`))
	if !ok {
		t.Fatal("expected envelope parse")
	}
	if isProjectTelemetryEnvelope(envelope) {
		t.Fatal("arbitrary localhost JSON must not be treated as BeamNG mod telemetry")
	}
}

func TestPackagedCompatibilityIndexMatchesCompiledBridge(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "BRIDGE_COMPATIBILITY.json"))
	if err != nil {
		t.Fatal(err)
	}
	var index compatibility.Index
	if err := json.Unmarshal(data, &index); err != nil {
		t.Fatal(err)
	}
	if err := compatibility.Validate(index); err != nil {
		t.Fatal(err)
	}
	release, ok := compatibility.FindRelease(index, currentBridgeVersion)
	if !ok {
		t.Fatalf("compatibility index does not contain %s", currentBridgeVersion)
	}
	for _, protocol := range compatibility.SupportedProtocols() {
		if !release.Supports(compatibility.Target{Protocol: protocol}) {
			t.Fatalf("%s does not declare compiled protocol %d", currentBridgeVersion, protocol)
		}
	}
}

func TestSupportedEnvelopeInspectionDoesNotAllocate(t *testing.T) {
	packet := []byte(`{"protocolId":"DPH","modVersion":"V1.5.4","protocolMin":41,"protocolMax":41,"v":41,"active":true}`)
	allocs := testing.AllocsPerRun(1000, func() {
		envelope, ok := inspectTelemetryEnvelope(packet)
		if !ok || !envelope.Project || envelope.Version != 41 {
			panic("unexpected envelope")
		}
	})
	if allocs != 0 {
		t.Fatalf("supported compatibility envelope inspection allocated %.2f objects/run", allocs)
	}
}

func TestTransitionTelemetryLaneSelection(t *testing.T) {
	legacyV10 := telemetry{Version: legacyProtocolVersion, Active: true}
	if !shouldConsumeTelemetry(legacyV10) {
		t.Fatal("real V1.0/v40 telemetry must remain consumable")
	}

	legacyMirror := telemetry{Version: legacyProtocolVersion, Active: true, LegacyCompat: true, ModVersion: "V1.1"}
	if shouldConsumeTelemetry(legacyMirror) {
		t.Fatal("V1.1 legacy mirror must be ignored by Bridge V1.3 to avoid duplicate effects")
	}

	current := telemetry{Version: protocolVersion, Active: true, ModVersion: "V1.1"}
	if !shouldConsumeTelemetry(current) {
		t.Fatal("current v41 telemetry must be consumed")
	}
}

func TestTransitionLegacyMirrorDecodesButIsNotConsumed(t *testing.T) {
	packet := []byte(`{"protocolId":"DPH","modVersion":"V1.1","protocolMin":40,"protocolMax":41,"legacyCompat":true,"v":40,"active":true,"l2Mode":1,"l2StartZone":0,"l2StartStrength":1,"l2EndStrength":4}`)
	decoded, ok := decodeTelemetry(packet)
	if !ok {
		t.Fatal("transition v40 mirror must still decode for compatibility inspection")
	}
	if shouldConsumeTelemetry(decoded) {
		t.Fatal("transition v40 mirror must not reach the V1.3 runtime")
	}
}

func TestRealV10PacketRemainsCompatibleWithV13(t *testing.T) {
	packet := []byte(`{"v":40,"active":true,"seq":17,"shiftLEDsInUse":true,"l2Mode":1,"l2StartZone":0,"l2StartStrength":2,"l2EndStrength":6,"r2Mode":3,"r2StartZone":0,"r2StartStrength":1,"r2EndStrength":1,"raw":{"rpm":4200,"maxRPM":7000,"engineRunning":true,"surfaceMaterialL":19,"surfaceRoughnessL":0.48,"surfaceMaterialR":10,"surfaceRoughnessR":0.03}}`)
	decoded, ok := decodeTelemetry(packet)
	if !ok {
		t.Fatal("real Mod V1.0 protocol-v40 packet must decode in Bridge V1.3")
	}
	if !shouldConsumeTelemetry(decoded) {
		t.Fatal("real Mod V1.0 packet must not be mistaken for the V1.1 migration mirror")
	}
	pair := triggerPairFromTelemetry(decoded)
	if pair.L2.Kind != triggerResistance || pair.L2.StartForce.Level() != 12 || pair.L2.EndForce.Level() != 36 {
		t.Fatalf("unexpected V1.0 L2 conversion: %+v", pair.L2)
	}
	if pair.R2.Kind != triggerFine || pair.R2.StartForce.Level() != 1 {
		t.Fatalf("unexpected V1.0 R2 conversion: %+v", pair.R2)
	}
}
