package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Ragnarok179/enhanced-ps5-dualsense-haptics-bridge/internal/buildinfo"
	"github.com/Ragnarok179/enhanced-ps5-dualsense-haptics-bridge/internal/compatibility"
)

type telemetryEnvelope struct {
	Project      bool
	Version      int
	ModVersion   string
	ProtocolMin  int
	ProtocolMax  int
	LegacyCompat bool
}

type protocolCompatibilityGuard struct {
	unsupportedOnce sync.Once
	migrationOnce   sync.Once
	futureOnce      sync.Once
	compatibleOnce  sync.Once
}

func inspectTelemetryEnvelope(data []byte) (telemetryEnvelope, bool) {
	var h struct {
		ProtocolID   string `json:"protocolId"`
		Version      int    `json:"v"`
		ModVersion   string `json:"modVersion"`
		ProtocolMin  int    `json:"protocolMin"`
		ProtocolMax  int    `json:"protocolMax"`
		LegacyCompat bool   `json:"legacyCompat"`
	}
	if json.Unmarshal(data, &h) != nil || h.Version <= 0 {
		return telemetryEnvelope{}, false
	}
	if h.ProtocolID != "" && h.ProtocolID != protocolID {
		return telemetryEnvelope{}, false
	}
	return telemetryEnvelope{Project: true, Version: h.Version, ModVersion: h.ModVersion, ProtocolMin: h.ProtocolMin, ProtocolMax: h.ProtocolMax, LegacyCompat: h.LegacyCompat}, true
}

func (g *protocolCompatibilityGuard) handlePacket(data []byte, diagnostics bool, requestStop func()) bool {
	env, ok := inspectTelemetryEnvelope(data)
	if !ok || !env.Project {
		return false
	}
	if env.LegacyCompat {
		return true
	}

	if !gameplayProtocolSupported(env.Version) {
		g.unsupportedOnce.Do(func() {
			_ = writePendingCompatibilityRequest(env)
			mod := fallbackTelemetryModVersion(env.ModVersion)
			fmt.Printf("UPDATE REQUIRED: BeamNG mod %s uses wire generation %d; Bridge %s supports %d-%d.\n", mod, env.Version, buildinfo.DisplayVersion, protocolMinVersion, protocolMaxVersion)
			fmt.Println("Run UPDATE_DUALSENSE.exe manually and follow the displayed steps.")
			requestStop()
		})
		return true
	}

	g.markCompatible()
	modVersion := strings.TrimSpace(env.ModVersion)
	if modVersion == "" {
		return false
	}
	cmp := compatibility.CompareVersions(modVersion, buildinfo.DisplayVersion)
	if cmp < 0 {
		g.migrationOnce.Do(func() {
			_ = writePendingCompatibilityRequest(env)
			fmt.Println()
			fmt.Println("============================================================")
			fmt.Println("ONE-TIME UPDATE MIGRATION")
			fmt.Println("============================================================")
			fmt.Println("1. Close this Bridge window.")
			fmt.Println("2. Run UPDATE_DUALSENSE.exe from the Bridge folder.")
			fmt.Println("3. Close BeamNG.drive if it is open.")
			fmt.Println("4. Confirm the update in UPDATE_DUALSENSE.exe.")
			fmt.Println("5. Follow the installation tutorial that opens after the update.")
			fmt.Println()
			fmt.Println("You can also open IMPORTANT_V1.41_-_RUN_UPDATE_DUALSENSE_TO_FINISH_UPDATE.txt for these steps.")
		})
		return false
	}
	if cmp > 0 {
		g.futureOnce.Do(func() {
			fmt.Printf("VERSION MISMATCH: BeamNG mod %s is newer than Bridge %s.\n", modVersion, buildinfo.DisplayVersion)
			fmt.Println("Run UPDATE_DUALSENSE.exe manually and follow the displayed steps.")
			requestStop()
		})
		return true
	}
	return false
}

func (g *protocolCompatibilityGuard) markCompatible() {
	g.compatibleOnce.Do(clearPendingCompatibilityRequest)
}

func fallbackTelemetryModVersion(v string) string {
	if strings.TrimSpace(v) == "" {
		return "unknown"
	}
	return strings.TrimSpace(v)
}

func bridgeInstallRoot() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Clean(filepath.Join(filepath.Dir(exe), "..")), nil
}

func pendingCompatibilityPath(root string) string {
	return filepath.Join(root, "Config", compatibility.PendingFileName)
}

func writePendingCompatibilityRequest(env telemetryEnvelope) error {
	root, err := bridgeInstallRoot()
	if err != nil {
		return err
	}
	path := pendingCompatibilityPath(root)
	if err = os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(compatibility.PendingTarget{ModVersion: strings.TrimSpace(env.ModVersion), Protocol: env.Version}, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func clearPendingCompatibilityRequest() {
	root, err := bridgeInstallRoot()
	if err == nil {
		_ = os.Remove(pendingCompatibilityPath(root))
	}
}
