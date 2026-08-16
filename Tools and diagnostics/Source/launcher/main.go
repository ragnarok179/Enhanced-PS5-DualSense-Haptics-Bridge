package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
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
		fmt.Fprintf(os.Stderr, "ERROR: unable to locate the launcher: %v\n", err)
		return 2
	}

	root := filepath.Dir(executable)
	launcherName := strings.ToUpper(filepath.Base(executable))
	launchBeamNG := strings.Contains(launcherName, "AND_BEAMNG")

	if launchBeamNG {
		fmt.Println("Enhanced PS5 DualSense Haptics - Starting BeamNG.drive")
		if err := exec.Command("explorer.exe", beamNGSteamURL).Start(); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: unable to launch BeamNG.drive through Steam: %v\n", err)
			return 3
		}
		time.Sleep(time.Second)
	}

	bridgeDir := filepath.Join(root, "Tools and diagnostics", "Bridge")
	usbPath := filepath.Join(bridgeDir, "EnhancedPS5DualSenseHapticsUSB.exe")
	bluetoothPath := filepath.Join(bridgeDir, "EnhancedPS5DualSenseHapticsBluetooth.exe")

	if !fileExists(usbPath) {
		fmt.Fprintf(os.Stderr, "ERROR: missing %q.\n", usbPath)
		return 2
	}
	if !fileExists(bluetoothPath) {
		fmt.Fprintf(os.Stderr, "ERROR: missing %q.\n", bluetoothPath)
		return 2
	}

	usbProbe := probe(usbPath, bridgeDir)
	if usbProbe.detected {
		fmt.Println("Enhanced PS5 DualSense Haptics - USB")
		return runBridge(usbPath, bridgeDir)
	}

	btProbe := probe(bluetoothPath, bridgeDir)
	if btProbe.detected {
		fmt.Println("Enhanced PS5 DualSense Haptics - Bluetooth")
		return runBridge(bluetoothPath, bridgeDir, "--protocol-36")
	}

	fmt.Fprintln(os.Stderr, "ERROR: no compatible DualSense controller was detected over USB or Bluetooth.")
	printProbeFailure("USB", usbProbe)
	printProbeFailure("Bluetooth", btProbe)

	diagnosticPath := filepath.Join(root, "Tools and diagnostics", "Diagnostics", "LIST_HARDWARE.bat")
	if fileExists(diagnosticPath) {
		fmt.Fprintf(os.Stderr, "Hardware diagnostics: %s\n", diagnosticPath)
	} else {
		fmt.Fprintf(os.Stderr, "ERROR: hardware diagnostics file is missing: %s\n", diagnosticPath)
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
		fmt.Fprintf(os.Stderr, "\nThe bridge exited with code %d.\n", exitError.ExitCode())
		return exitError.ExitCode()
	}

	fmt.Fprintf(os.Stderr, "ERROR: unable to start the bridge: %v\n", err)
	return 2
}
