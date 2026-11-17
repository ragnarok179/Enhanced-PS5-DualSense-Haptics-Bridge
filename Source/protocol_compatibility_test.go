package main

import (
	"path/filepath"
	"testing"
)

func TestRuntimeProtocol43Only(t *testing.T) {
	for _, protocol := range []int{41, 42, 44} {
		if gameplayProtocolSupported(protocol) {
			t.Fatalf("unsupported wire generation %d accepted", protocol)
		}
	}
	if !gameplayProtocolSupported(43) {
		t.Fatal("V1.43+ wire generation rejected")
	}
}

func TestFutureProtocolTriggersUpdate(t *testing.T) {
	guard := protocolCompatibilityGuard{}
	stopped := false
	packet := []byte(`{"protocolId":"DPH","modVersion":"V1.52","protocolMin":44,"protocolMax":44,"v":44}`)
	if !guard.handlePacket(packet, false, func() { stopped = true }) || !stopped {
		t.Fatal("future wire protocol must request an update")
	}
}

func TestPendingCompatibilityPath(t *testing.T) {
	want := filepath.Join("root", "Config", "pending_bridge_compatibility.json")
	if got := pendingCompatibilityPath("root"); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestV143ModCanConnectToV151Bridge(t *testing.T) {
	guard := protocolCompatibilityGuard{}
	stopped := false
	packet := []byte(`{"protocolId":"DPH","modVersion":"V1.43","protocolMin":43,"protocolMax":43,"v":43}`)
	if guard.handlePacket(packet, false, func() { stopped = true }) || stopped {
		t.Fatal("V1.43 must still be readable by V1.51 Bridge for updater guidance")
	}
}

func TestV151SynchronizedBridge(t *testing.T) {
	guard := protocolCompatibilityGuard{}
	stopped := false
	if guard.handlePacket([]byte(`{"protocolId":"DPH","modVersion":"V1.51","protocolMin":43,"protocolMax":43,"v":43}`), false, func() { stopped = true }) || stopped {
		t.Fatal("V1.51 failed to synchronize")
	}
}
