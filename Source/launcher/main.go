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

const (
	beamNGSteamURL                     = "steam://rungameid/284160"
	launcherExitControllerDisconnected = 10
	launcherExitTransportNotReady      = 11
	controllerSearchInterval           = time.Second
)

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

	// START_BRIDGE owns controller discovery for the whole session. Every child
	// receives --launcher-auto plus any optional diagnostic/test arguments.
	// A disconnect or a transport that is still coming online is a retry state,
	// not a fatal error. This keeps one terminal alive across USB/Bluetooth swaps.
	childArgs := make([]string, 0, len(os.Args))
	childArgs = append(childArgs, "--launcher-auto")
	for _, arg := range os.Args[1:] {
		if arg != "--launcher-auto" {
			childArgs = append(childArgs, arg)
		}
	}

	waitingPrinted := false
	for {
		usbCode := runBridge(usbPath, bridgeDir, childArgs...)
		switch usbCode {
		case 0:
			return 0
		case launcherExitControllerDisconnected:
			waitingPrinted = false
		case 1, launcherExitTransportNotReady:
			// USB is absent or Windows has not finished exposing all of its
			// controller endpoints yet. Try Bluetooth, then scan again.
		default:
			return usbCode
		}

		btCode := runBridge(bluetoothPath, bridgeDir, childArgs...)
		switch btCode {
		case 0:
			return 0
		case launcherExitControllerDisconnected:
			waitingPrinted = false
		case 1, launcherExitTransportNotReady:
			// Bluetooth is absent or still initializing; wait and scan again.
		default:
			return btCode
		}

		if !waitingPrinted {
			fmt.Println("Waiting for a DualSense controller...")
			waitingPrinted = true
		}
		time.Sleep(controllerSearchInterval)
	}
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
		launcherAuto := false
		for _, arg := range args {
			if arg == "--launcher-auto" {
				launcherAuto = true
				break
			}
		}
		if launcherAuto && (code == 1 || code == launcherExitControllerDisconnected || code == launcherExitTransportNotReady) {
			return code
		}
		fmt.Fprintln(os.Stderr, "\nThe Bridge stopped unexpectedly. Run Diagnostics\\RUN_DIAGNOSTIC.bat if the problem continues.")
		return code
	}

	fmt.Fprintln(os.Stderr, "ERROR: the Bridge could not be started.")
	return 2
}
