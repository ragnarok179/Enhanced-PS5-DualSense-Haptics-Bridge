package main

import (
	"fmt"
	"path/filepath"
	"testing"
)

func TestCompatibilityEnvelopeSupportedProtocols(t *testing.T) {
	for _, version := range []int{41, 42, 43} {
		packet := []byte(fmt.Sprintf(`{"protocolId":"DPH","modVersion":"V1.3","v":%d}`, version))
		env, ok := inspectTelemetryEnvelope(packet)
		if !ok || env.Version != version || !gameplayProtocolSupported(env.Version) {
			t.Fatalf("supported protocol %d not accepted: %+v ok=%t", version, env, ok)
		}
	}
	if gameplayProtocolSupported(40) {
		t.Fatal("protocol 40 must remain outside the V1.3+ support floor")
	}
}

func TestCompatibilityEnvelopeFutureProtocolRequiresUpdate(t *testing.T) {
	env, ok := inspectTelemetryEnvelope([]byte(`{"protocolId":"DPH","modVersion":"V1.43","protocolMin":44,"protocolMax":44,"v":44}`))
	if !ok || env.Version != 44 {
		t.Fatalf("bad envelope: %+v ok=%t", env, ok)
	}
	if gameplayProtocolSupported(env.Version) {
		t.Fatal("Bridge V1.42 must not claim future wire generation 44 support")
	}
}

func TestPendingCompatibilityUsesNewLayout(t *testing.T) {
	root := filepath.Join("root", "bridge")
	want := filepath.Join(root, "Config", "pending_bridge_compatibility.json")
	if got := pendingCompatibilityPath(root); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestV142GuardAllowsV13ForOneTimeMigration(t *testing.T) {
	guard := protocolCompatibilityGuard{}
	stopped := false
	packet := []byte(`{"protocolId":"DPH","modVersion":"V1.3","protocolMin":41,"protocolMax":41,"v":41}`)
	if consumed := guard.handlePacket(packet, false, func() { stopped = true }); consumed {
		t.Fatal("supported V1.3 packet should continue through telemetry decoding")
	}
	if stopped {
		t.Fatal("V1.3 migration must not stop Bridge V1.42")
	}
}

func TestV142GuardAcceptsSynchronizedRelease(t *testing.T) {
	guard := protocolCompatibilityGuard{}
	stopped := false
	packet := []byte(`{"protocolId":"DPH","modVersion":"V1.42","protocolMin":43,"protocolMax":43,"v":43}`)
	if consumed := guard.handlePacket(packet, false, func() { stopped = true }); consumed {
		t.Fatal("synchronized V1.42 packet should continue through telemetry decoding")
	}
	if stopped {
		t.Fatal("synchronized V1.42 release must not stop")
	}
}

func TestV142GuardStopsNewerProjectRelease(t *testing.T) {
	guard := protocolCompatibilityGuard{}
	stopped := false
	packet := []byte(`{"protocolId":"DPH","modVersion":"V1.43","protocolMin":43,"protocolMax":43,"v":43}`)
	if consumed := guard.handlePacket(packet, false, func() { stopped = true }); !consumed {
		t.Fatal("newer mod release must be consumed by version guard")
	}
	if !stopped {
		t.Fatal("newer mod release must stop the older Bridge until manual update")
	}
}
