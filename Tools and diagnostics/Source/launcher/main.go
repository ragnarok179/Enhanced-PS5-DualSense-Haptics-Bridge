package main

import (
	"fmt"
	"io"
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

	if probe(usbPath, bridgeDir) {
		fmt.Println("Enhanced PS5 DualSense Haptics - USB")
		return runBridge(usbPath, bridgeDir)
	}

	if probe(bluetoothPath, bridgeDir) {
		fmt.Println("Enhanced PS5 DualSense Haptics - Bluetooth")
		return runBridge(bluetoothPath, bridgeDir, "--protocol-36", "--rgb-via-beamng")
	}

	fmt.Fprintln(os.Stderr, "ERROR: no compatible DualSense controller was detected over USB or Bluetooth.")
	fmt.Fprintln(os.Stderr, `Open "Tools and diagnostics\Diagnostics\LIST_HARDWARE.bat" for more information.`)
	return 1
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func probe(path, workingDirectory string) bool {
	cmd := exec.Command(path, "--probe")
	cmd.Dir = workingDirectory
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run() == nil
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
