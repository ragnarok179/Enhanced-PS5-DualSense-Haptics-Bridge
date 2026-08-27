package main

import (
	"fmt"
	"path/filepath"
	"testing"
)

func TestCompatibilityEnvelopeOfficialProtocols(t *testing.T) {
	for _, version := range []int{40, 41, 42} {
		packet := []byte(fmt.Sprintf(`{"protocolId":"DPH","modVersion":"V1.2","v":%d}`, version))
		env, ok := inspectTelemetryEnvelope(packet)
		if !ok || env.Version != version || !gameplayProtocolSupported(env.Version) {
			t.Fatalf("official protocol %d not accepted: %+v ok=%t", version, env, ok)
		}
	}
}

func TestCompatibilityEnvelopeFutureProtocolRequiresUpdate(t *testing.T) {
	env, ok := inspectTelemetryEnvelope([]byte(`{"protocolId":"DPH","modVersion":"V1.3","protocolMin":43,"protocolMax":43,"v":43}`))
	if !ok || env.Version != 43 {
		t.Fatalf("bad envelope: %+v ok=%t", env, ok)
	}
	if gameplayProtocolSupported(env.Version) {
		t.Fatal("Bridge V1.4 must not claim future protocol 43 support")
	}
}

func TestPendingCompatibilityUsesNewLayout(t *testing.T) {
	root := filepath.Join("root", "bridge")
	want := filepath.Join(root, "Config", "pending_bridge_compatibility.json")
	if got := pendingCompatibilityPath(root); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
