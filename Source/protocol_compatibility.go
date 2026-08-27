package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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
	once           sync.Once
	compatibleOnce sync.Once
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
	// Marked migration mirrors never establish compatibility by themselves.
	// This prevents a legacy packet emitted after an unsupported canonical packet
	// from clearing the pending update request.
	if env.LegacyCompat {
		return true
	}
	if gameplayProtocolSupported(env.Version) {
		g.markCompatible()
		return false
	}
	g.once.Do(func() {
		_ = writePendingCompatibilityRequest(env)
		mod := strings.TrimSpace(env.ModVersion)
		if mod == "" {
			mod = "unknown"
		}
		fmt.Printf("UPDATE REQUIRED: BeamNG mod %s uses gameplay protocol %d; Bridge %s supports protocols %d-%d.\n", mod, env.Version, buildinfo.DisplayVersion, protocolMinVersion, protocolMaxVersion)
		if env.ProtocolMin > 0 && env.ProtocolMax >= env.ProtocolMin {
			fmt.Printf("Mod protocol range: %d-%d.\n", env.ProtocolMin, env.ProtocolMax)
		}
		if env.Version < protocolMinVersion {
			fmt.Println("This mod protocol is older than the minimum supported by this Bridge. Install a Bridge release that declares support for that protocol.")
			requestStop()
			return
		}
		if diagnostics {
			fmt.Println("Compatibility updater is not launched automatically in diagnostic mode. Run UPDATE_BRIDGE.exe after closing this log session.")
			requestStop()
			return
		}
		fmt.Print("Install the newest compatible Bridge release now? [Y/N]: ")
		answer, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		answer = strings.ToLower(strings.TrimSpace(answer))
		if answer != "y" && answer != "yes" {
			fmt.Println("Bridge stopped. Run UPDATE_BRIDGE.exe later to retry the compatibility update.")
			requestStop()
			return
		}
		if err := launchCompatibilityUpdater(env); err != nil {
			fmt.Printf("Unable to start UPDATE_BRIDGE.exe: %v\n", err)
			fmt.Println("Run UPDATE_BRIDGE.exe manually after closing the Bridge.")
		} else {
			fmt.Println("Updater started. The Bridge will close while a compatible stable release is installed.")
		}
		requestStop()
	})
	return true
}

func (g *protocolCompatibilityGuard) markCompatible() {
	g.compatibleOnce.Do(clearPendingCompatibilityRequest)
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
func launchCompatibilityUpdater(env telemetryEnvelope) error {
	root, err := bridgeInstallRoot()
	if err != nil {
		return err
	}
	updater := filepath.Join(root, "UPDATE_BRIDGE.exe")
	if !regularFileExists(updater) {
		return fmt.Errorf("missing %s", updater)
	}
	args := []string{"--compatibility-update", "--protocol", strconv.Itoa(env.Version), "--wait-pid", strconv.Itoa(os.Getpid()), "--relaunch"}
	if m := strings.TrimSpace(env.ModVersion); m != "" {
		args = append(args, "--mod-version", m)
	}
	cmd := exec.Command(updater, args...)
	cmd.Dir = root
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Start()
}
func regularFileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
