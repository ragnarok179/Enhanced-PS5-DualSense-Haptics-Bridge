//go:build windows

package main

import (
	"fmt"
	"syscall"
	"time"
)

var (
	msvcrtDLL = syscall.NewLazyDLL("msvcrt.dll")
	procKBHit = msvcrtDLL.NewProc("_kbhit")
	procGetCh = msvcrtDLL.NewProc("_getch")
)

func consoleKeyAvailable() bool {
	r, _, _ := procKBHit.Call()
	return r != 0
}

func consoleGetKey() int {
	r, _, _ := procGetCh.Call()
	return int(r & 0xff)
}

// This legacy terminal editor remains available when the Bridge console has
// focus, but the normal user-facing configuration lives in BeamNG's Mods menu.
func startConsoleSettingsMenu(done <-chan struct{}) {
	ensureUserSettings()
	go func() {
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				if !consoleKeyAvailable() {
					continue
				}
				key := consoleGetKey()
				if key == 'p' || key == 'P' {
					runConsoleSettingsMenu(done)
				}
			}
		}
	}()
}

func printConsoleSettings(selected int) {
	s := currentUserSettings()
	fmt.Println()
	fmt.Println("================ Controller settings ================")
	fmt.Println("Up/Down: select | Left/Right: -/+ 1 | Space: on/off | R: reset | P/Esc: close")
	fmt.Println("Values are shown as percentages. Disabling a layer preserves its stored value.")
	for i := 0; i < userSettingCount; i++ {
		marker := "  "
		if i == selected {
			marker = "> "
		}
		state := "off"
		if userSettingEnabled(i, s) {
			state = "on "
		}
		fmt.Printf("%s%-31s %s %5.1f%%\n", marker, userSettingName(i), state, userSettingPercent(i, userSettingValue(i, s)))
	}
	fmt.Println("=====================================================")
}

func runConsoleSettingsMenu(done <-chan struct{}) {
	selected := 0
	printConsoleSettings(selected)
	for {
		select {
		case <-done:
			return
		default:
		}
		if !consoleKeyAvailable() {
			time.Sleep(20 * time.Millisecond)
			continue
		}
		key := consoleGetKey()
		if key == 0 || key == 224 {
			for !consoleKeyAvailable() {
				time.Sleep(time.Millisecond)
			}
			extended := consoleGetKey()
			switch extended {
			case 72: // up
				selected = (selected + userSettingCount - 1) % userSettingCount
				printConsoleSettings(selected)
			case 80: // down
				selected = (selected + 1) % userSettingCount
				printConsoleSettings(selected)
			case 75: // left
				value := adjustUserSetting(selected, -1)
				_ = saveUserSettings()
				fmt.Printf("[SETTINGS] %s = %.1f%%\n", userSettingName(selected), userSettingPercent(selected, value))
			case 77: // right
				value := adjustUserSetting(selected, 1)
				_ = saveUserSettings()
				fmt.Printf("[SETTINGS] %s = %.1f%%\n", userSettingName(selected), userSettingPercent(selected, value))
			}
			continue
		}
		switch key {
		case 'p', 'P', 27:
			if err := saveUserSettings(); err != nil {
				fmt.Println("[SETTINGS] Unable to save settings:", err)
			}
			fmt.Println("[SETTINGS] Closed. Press P to reopen.")
			return
		case 'r', 'R':
			resetUserSettings()
			_ = saveUserSettings()
			fmt.Println("[SETTINGS] Defaults restored.")
			printConsoleSettings(selected)
		case ' ':
			enabled := toggleUserSetting(selected)
			_ = saveUserSettings()
			fmt.Printf("[SETTINGS] %s = %t\n", userSettingName(selected), enabled)
			printConsoleSettings(selected)
		case '+', '=':
			value := adjustUserSetting(selected, 1)
			_ = saveUserSettings()
			fmt.Printf("[SETTINGS] %s = %.1f%%\n", userSettingName(selected), userSettingPercent(selected, value))
		case '-', '_':
			value := adjustUserSetting(selected, -1)
			_ = saveUserSettings()
			fmt.Printf("[SETTINGS] %s = %.1f%%\n", userSettingName(selected), userSettingPercent(selected, value))
		}
	}
}
