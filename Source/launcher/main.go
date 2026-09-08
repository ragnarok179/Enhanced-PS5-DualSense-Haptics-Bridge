package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Ragnarok179/enhanced-ps5-dualsense-haptics-bridge/internal/buildinfo"
)

const beamNGSteamURL = "steam://rungameid/284160"

func main() {
	code := run()
	if code != 0 {
		fmt.Println()
		fmt.Print("Press Enter to close...")
		_, _ = fmt.Scanln()
	}
	os.Exit(code)
}

func run() int {
	executable, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "ERROR: the Bridge launcher could not start.")
		return 2
	}

	root := filepath.Dir(executable)
	launcherName := strings.ToUpper(filepath.Base(executable))
	launchBeamNG := strings.Contains(launcherName, "AND_BEAMNG")

	if launchBeamNG {
		fmt.Println("Enhanced PS5 DualSense Bridge", buildinfo.DisplayVersion, "- starting BeamNG")
		if err := exec.Command("explorer.exe", beamNGSteamURL).Start(); err != nil {
			fmt.Fprintln(os.Stderr, "ERROR: BeamNG could not be started through Steam.")
			return 3
		}
		time.Sleep(time.Second)
	}

	bridgeDir := filepath.Join(root, "Bridge")
	usbPath := filepath.Join(bridgeDir, "EnhancedPS5DualSenseHapticsUSB.exe")
	bluetoothPath := filepath.Join(bridgeDir, "EnhancedPS5DualSenseHapticsBluetooth.exe")

	if !fileExists(usbPath) {
		fmt.Fprintln(os.Stderr, "ERROR: Bridge files are incomplete. Reinstall or update the Bridge.")
		return 2
	}
	if !fileExists(bluetoothPath) {
		fmt.Fprintln(os.Stderr, "ERROR: Bridge files are incomplete. Reinstall or update the Bridge.")
		return 2
	}

	// Start each transport at most once. Older launchers spawned a probe process
	// and then restarted the selected Bridge, which made cold starts noticeably
	// slower on Windows/Defender. --launcher-auto exits 1 silently only when the
	// requested transport is absent; otherwise that same process becomes runtime.
	usbCode := runBridge(usbPath, bridgeDir, "--launcher-auto")
	if usbCode != 1 {
		return usbCode
	}
	btCode := runBridge(bluetoothPath, bridgeDir, "--launcher-auto")
	if btCode != 1 {
		return btCode
	}

	fmt.Fprintln(os.Stderr, "ERROR: no compatible DualSense controller was detected over USB or Bluetooth.")

	diagnosticPath := filepath.Join(root, "Diagnostics", "LIST_HARDWARE.bat")
	if fileExists(diagnosticPath) {
		fmt.Fprintf(os.Stderr, "Hardware diagnostics: %s\n", diagnosticPath)
	} else {
		fmt.Fprintln(os.Stderr, "ERROR: the hardware diagnostics file is missing. Reinstall or update the Bridge.")
	}
	return 1
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

type probeResult struct {
	detected bool
	exitCode int
	output   string
	err      error
}

func probe(path, workingDirectory string) probeResult {
	cmd := exec.Command(path, "--probe")
	cmd.Dir = workingDirectory
	output, err := cmd.CombinedOutput()
	result := probeResult{output: strings.TrimSpace(string(output)), err: err}
	if err == nil {
		result.detected = true
		return result
	}
	if exitError, ok := err.(*exec.ExitError); ok {
		result.exitCode = exitError.ExitCode()
	} else {
		result.exitCode = -1
	}
	return result
}

func printProbeFailure(name string, result probeResult) {
	if result.err == nil {
		return
	}
	if result.output != "" {
		fmt.Fprintf(os.Stderr, "%s probe: %s\n", name, result.output)
	}
	// Exit code 1 is the normal "controller not present on this transport" result.
	// Other failures need to stay visible instead of being misreported as no controller.
	if result.exitCode != 1 {
		fmt.Fprintf(os.Stderr, "%s probe failed (exit %d): %v\n", name, result.exitCode, result.err)
	}
}

func runBridge(path, workingDirectory string, args ...string) int {
	cmd := exec.Command(path, args...)
	cmd.Dir = workingDirectory
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err == nil {
		return 0
	}
	if exitError, ok := err.(*exec.ExitError); ok {
		code := exitError.ExitCode()
		if code == 1 {
			for _, arg := range args {
				if arg == "--launcher-auto" {
					return 1
				}
			}
		}
		fmt.Fprintln(os.Stderr, "\nThe Bridge stopped unexpectedly. Run the hardware diagnostics if the problem continues.")
		return code
	}

	fmt.Fprintln(os.Stderr, "ERROR: the Bridge could not be started.")
	return 2
}
