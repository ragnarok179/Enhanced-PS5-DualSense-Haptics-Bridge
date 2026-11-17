//go:build windows && (usb || bluetooth)

package main

import (
	"strings"
	"syscall"
	"time"
	"unsafe"
)

// Only inspect process names and the foreground-window PID; never open or
// modify another process. Run outside the high-frequency HID writer.
var (
	user32Foreground    = syscall.NewLazyDLL("user32.dll").NewProc("GetForegroundWindow")
	user32ForegroundPID = syscall.NewLazyDLL("user32.dll").NewProc("GetWindowThreadProcessId")
)

func beamNGGameProcessStatus() (present, foreground, valid bool) {
	handle, err := syscall.CreateToolhelp32Snapshot(syscall.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return false, false, false
	}
	defer syscall.CloseHandle(handle)
	entry := syscall.ProcessEntry32{Size: uint32(unsafe.Sizeof(syscall.ProcessEntry32{}))}
	if err = syscall.Process32First(handle, &entry); err != nil {
		return false, false, false
	}
	for {
		name := strings.ToLower(syscall.UTF16ToString(entry.ExeFile[:]))
		switch name {
		case "beamng.drive.x64.exe", "beamng.drive.x86.exe", "beamng.drive.exe":
			present = true
			window, _, _ := user32Foreground.Call()
			if window != 0 {
				var currentPID uint32
				user32ForegroundPID.Call(window, uintptr(unsafe.Pointer(&currentPID)))
				if currentPID == entry.ProcessID {
					foreground = true
				}
			}
		}
		if err = syscall.Process32Next(handle, &entry); err != nil {
			break
		}
	}
	return present, foreground, true
}

func startMicMuteGameProcessMonitor(done <-chan struct{}) {
	go func() {
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				alive, focused, valid := beamNGGameProcessStatus()
				if valid {
					noteBeamNGProcessFocusSample(alive, focused, time.Now())
				}
			}
		}
	}()
}
