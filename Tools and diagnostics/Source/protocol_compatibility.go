package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/Ragnarok179/enhanced-ps5-dualsense-haptics-bridge/internal/compatibility"
)

const (
	protocolVersion       = compatibility.CurrentProtocol
	legacyProtocolVersion = compatibility.LegacyProtocol
	currentBridgeVersion  = compatibility.CurrentBridgeVersion
)

var (
	jsonKeyVersion     = []byte(`"v"`)
	jsonKeyProtocolID  = []byte(`"protocolId"`)
	jsonKeyModVersion  = []byte(`"modVersion"`)
	jsonKeyProtocolMin = []byte(`"protocolMin"`)
	jsonKeyProtocolMax = []byte(`"protocolMax"`)
	protocolIDBytes    = []byte(compatibility.ProtocolID)
)

type telemetryEnvelope struct {
	Project     bool
	Version     int
	ModVersion  string
	ProtocolMin int
	ProtocolMax int
}

type protocolCompatibilityGuard struct {
	once           sync.Once
	compatibleOnce sync.Once
}

func inspectTelemetryEnvelope(data []byte) (telemetryEnvelope, bool) {
	version, ok := jsonIntField(data, jsonKeyVersion)
	if !ok || version <= 0 {
		return telemetryEnvelope{}, false
	}
	envelope := telemetryEnvelope{
		Project: jsonStringFieldEquals(data, jsonKeyProtocolID, protocolIDBytes),
		Version: version,
	}
	// Current/supported traffic stays allocation-free here. Extra display/range
	// metadata is needed only when the Bridge cannot decode the packet.
	if envelope.Project && !compatibility.ProtocolSupported(version) {
		envelope.ModVersion, _ = jsonStringField(data, jsonKeyModVersion)
		envelope.ProtocolMin, _ = jsonIntField(data, jsonKeyProtocolMin)
		envelope.ProtocolMax, _ = jsonIntField(data, jsonKeyProtocolMax)
	}
	return envelope, true
}

func jsonFieldValueStart(data, key []byte) (int, bool) {
	index := bytes.Index(data, key)
	if index < 0 {
		return 0, false
	}
	index += len(key)
	for index < len(data) && (data[index] == ' ' || data[index] == '\t' || data[index] == '\r' || data[index] == '\n') {
		index++
	}
	if index >= len(data) || data[index] != ':' {
		return 0, false
	}
	index++
	for index < len(data) && (data[index] == ' ' || data[index] == '\t' || data[index] == '\r' || data[index] == '\n') {
		index++
	}
	return index, index < len(data)
}

func jsonIntField(data, key []byte) (int, bool) {
	index, ok := jsonFieldValueStart(data, key)
	if !ok {
		return 0, false
	}
	sign := 1
	if data[index] == '-' {
		sign = -1
		index++
	}
	if index >= len(data) || data[index] < '0' || data[index] > '9' {
		return 0, false
	}
	value := 0
	for index < len(data) && data[index] >= '0' && data[index] <= '9' {
		value = value*10 + int(data[index]-'0')
		index++
	}
	return value * sign, true
}

func jsonStringFieldEquals(data, key, expected []byte) bool {
	index, ok := jsonFieldValueStart(data, key)
	if !ok || data[index] != '"' {
		return false
	}
	index++
	if len(data)-index < len(expected)+1 {
		return false
	}
	if !bytes.Equal(data[index:index+len(expected)], expected) {
		return false
	}
	return data[index+len(expected)] == '"'
}

func jsonStringField(data, key []byte) (string, bool) {
	index, ok := jsonFieldValueStart(data, key)
	if !ok || data[index] != '"' {
		return "", false
	}
	index++
	start := index
	for index < len(data) {
		switch data[index] {
		case '\\':
			return "", false
		case '"':
			return string(data[start:index]), true
		default:
			index++
		}
	}
	return "", false
}

func isProjectTelemetryEnvelope(envelope telemetryEnvelope) bool {
	return envelope.Project
}

func (g *protocolCompatibilityGuard) supportsEnvelope(envelope telemetryEnvelope) bool {
	// The compiled decoder support set is the runtime source of truth. If the wire
	// contract changes incompatibly, the protocol number must increase.
	return compatibility.ProtocolSupported(envelope.Version)
}

// handlePacket returns true when the packet belongs to the project but cannot
// be consumed by this Bridge generation. The prompt is local-only: GitHub is
// contacted only after the user explicitly chooses Y and UPDATE_BRIDGE.exe runs.
func (g *protocolCompatibilityGuard) handlePacket(data []byte, diagnostics bool, requestStop func()) bool {
	envelope, ok := inspectTelemetryEnvelope(data)
	if !ok || !isProjectTelemetryEnvelope(envelope) {
		return false
	}
	if g.supportsEnvelope(envelope) {
		g.markCompatible()
		return false
	}

	g.once.Do(func() {
		_ = writePendingCompatibilityRequest(envelope)
		modLabel := strings.TrimSpace(envelope.ModVersion)
		if modLabel == "" {
			modLabel = "unknown"
		}
		fmt.Printf("BeamNG mod %s uses protocol %d, which is not compatible with Bridge %s.\n", modLabel, envelope.Version, currentBridgeVersion)
		if envelope.ProtocolMin > 0 && envelope.ProtocolMax >= envelope.ProtocolMin {
			fmt.Printf("Mod protocol range: %d-%d. Bridge protocols: %s.\n", envelope.ProtocolMin, envelope.ProtocolMax, supportedProtocolText())
		}

		if diagnostics {
			fmt.Println("Compatibility update was not started from diagnostic mode. Run UPDATE_BRIDGE.exe from the Bridge folder.")
			requestStop()
			return
		}

		fmt.Print("Install the newest compatible Bridge release now? [Y/N]: ")
		answer, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		answer = strings.ToLower(strings.TrimSpace(answer))
		if answer != "y" && answer != "yes" {
			fmt.Println("Bridge stopped. You can run UPDATE_BRIDGE.exe later.")
			requestStop()
			return
		}

		if err := launchCompatibilityUpdater(envelope); err != nil {
			fmt.Printf("Unable to start UPDATE_BRIDGE.exe: %v\n", err)
			fmt.Println("Please run UPDATE_BRIDGE.exe manually and try again later.")
		} else {
			fmt.Println("Updater started. The Bridge will close while the compatible release is installed.")
		}
		requestStop()
	})
	return true
}

func (g *protocolCompatibilityGuard) markCompatible() {
	g.compatibleOnce.Do(clearPendingCompatibilityRequest)
}

func beamNGConnectionMessage(t telemetry) string {
	modVersion := strings.TrimSpace(t.ModVersion)
	if modVersion != "" {
		return fmt.Sprintf("BeamNG.drive: connected. Mod %s / protocol %d.", modVersion, t.Version)
	}
	return fmt.Sprintf("BeamNG.drive: connected. Protocol %d.", t.Version)
}

func supportedProtocolText() string {
	protocols := compatibility.SupportedProtocols()
	parts := make([]string, 0, len(protocols))
	for _, protocol := range protocols {
		parts = append(parts, strconv.Itoa(protocol))
	}
	return strings.Join(parts, ", ")
}

func bridgeInstallRoot() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", err
	}
	// Runtime executables live in <root>/Tools and diagnostics/Bridge/.
	return filepath.Clean(filepath.Join(filepath.Dir(executable), "..", "..")), nil
}

func launchCompatibilityUpdater(envelope telemetryEnvelope) error {
	root, err := bridgeInstallRoot()
	if err != nil {
		return err
	}
	updater := filepath.Join(root, "UPDATE_BRIDGE.exe")
	if !regularFileExists(updater) {
		return fmt.Errorf("missing %s", updater)
	}
	args := []string{
		"--compatibility-update",
		"--protocol", strconv.Itoa(envelope.Version),
		"--wait-pid", strconv.Itoa(os.Getpid()),
		"--relaunch",
	}
	if modVersion := strings.TrimSpace(envelope.ModVersion); modVersion != "" {
		args = append(args, "--mod-version", modVersion)
	}
	cmd := exec.Command(updater, args...)
	cmd.Dir = root
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Start()
}

func pendingCompatibilityPath(root string) string {
	return filepath.Join(root, "Tools and diagnostics", "Config", compatibility.PendingFileName)
}

func writePendingCompatibilityRequest(envelope telemetryEnvelope) error {
	root, err := bridgeInstallRoot()
	if err != nil {
		return err
	}
	path := pendingCompatibilityPath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(compatibility.PendingTarget{ModVersion: strings.TrimSpace(envelope.ModVersion), Protocol: envelope.Version}, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func clearPendingCompatibilityRequest() {
	root, err := bridgeInstallRoot()
	if err != nil {
		return
	}
	_ = os.Remove(pendingCompatibilityPath(root))
}

func regularFileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
