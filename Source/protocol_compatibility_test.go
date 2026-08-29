package main

import (
	"fmt"
	"path/filepath"
	"testing"
)

func TestCompatibilityEnvelopeOfficialProtocols(t *testing.T) {
	for _, version := range []int{40, 41, 42, 43} {
		packet := []byte(fmt.Sprintf(`{"protocolId":"DPH","modVersion":"V1.2","v":%d}`, version))
		env, ok := inspectTelemetryEnvelope(packet)
		if !ok || env.Version != version || !gameplayProtocolSupported(env.Version) {
			t.Fatalf("official protocol %d not accepted: %+v ok=%t", version, env, ok)
		}
	}
}

func TestCompatibilityEnvelopeFutureProtocolRequiresUpdate(t *testing.T) {
	env, ok := inspectTelemetryEnvelope([]byte(`{"protocolId":"DPH","modVersion":"V1.42","protocolMin":44,"protocolMax":44,"v":44}`))
	if !ok || env.Version != 44 {
		t.Fatalf("bad envelope: %+v ok=%t", env, ok)
	}
	if gameplayProtocolSupported(env.Version) {
		t.Fatal("Bridge V1.41 must not claim future wire generation 44 support")
	}
}

func TestPendingCompatibilityUsesNewLayout(t *testing.T) {
	root := filepath.Join("root", "bridge")
	want := filepath.Join(root, "Config", "pending_bridge_compatibility.json")
	if got := pendingCompatibilityPath(root); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestV141GuardAllowsLegacyModForOneTimeMigration(t *testing.T) {
	guard := protocolCompatibilityGuard{}
	stopped := false
	packet := []byte(`{"protocolId":"DPH","modVersion":"V1.2","protocolMin":42,"protocolMax":42,"v":42}`)
	if consumed := guard.handlePacket(packet, false, func() { stopped = true }); consumed {
		t.Fatal("supported legacy V1.2 packet should continue through telemetry decoding")
	}
	if stopped {
		t.Fatal("legacy mod migration must not stop Bridge V1.41")
	}
}

func TestV141GuardAcceptsSynchronizedRelease(t *testing.T) {
	guard := protocolCompatibilityGuard{}
	stopped := false
	packet := []byte(`{"protocolId":"DPH","modVersion":"V1.41","protocolMin":43,"protocolMax":43,"v":43}`)
	if consumed := guard.handlePacket(packet, false, func() { stopped = true }); consumed {
		t.Fatal("synchronized V1.41 packet should continue through telemetry decoding")
	}
	if stopped {
		t.Fatal("synchronized V1.41 release must not stop")
	}
}

func TestV141GuardStopsNewerProjectRelease(t *testing.T) {
	guard := protocolCompatibilityGuard{}
	stopped := false
	packet := []byte(`{"protocolId":"DPH","modVersion":"V1.42","protocolMin":43,"protocolMax":43,"v":43}`)
	if consumed := guard.handlePacket(packet, false, func() { stopped = true }); !consumed {
		t.Fatal("newer mod release must be consumed by version guard")
	}
	if !stopped {
		t.Fatal("newer mod release must stop the older Bridge until manual update")
	}
}
