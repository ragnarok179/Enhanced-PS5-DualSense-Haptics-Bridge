//go:build windows && bluetooth

package main

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"
)

const ioctlBthDisconnectDevice = 0x41000c

var (
	modBthProps                 = syscall.NewLazyDLL("bthprops.cpl")
	procBluetoothFindFirstRadio = modBthProps.NewProc("BluetoothFindFirstRadio")
	procBluetoothFindNextRadio  = modBthProps.NewProc("BluetoothFindNextRadio")
	procBluetoothFindRadioClose = modBthProps.NewProc("BluetoothFindRadioClose")
	procDeviceIoControl         = modKernel32.NewProc("DeviceIoControl")
	procGetSerialNumber         = modHID.NewProc("HidD_GetSerialNumberString")
)

type bluetoothFindRadioParams struct {
	DwSize uint32
}

func readDualSenseBluetoothAddress(d *device) (uint64, error) {
	if d == nil || !validHandle(d.handle) {
		return 0, fmt.Errorf("controller is closed")
	}
	// Linux hid-playstation/dualsensectl define report 0x09 as 20 bytes, while
	// RPCS3 observes 21 bytes from hidapi on some stacks. Direct HidD_GetFeature
	// behavior can also vary with the Windows collection. Try the report-sized
	// forms first, then conservative larger buffers; only bytes 1..6 are needed.
	lengths := []int{20, 21, 64}
	if d.featureLen > 0 {
		lengths = append(lengths, d.featureLen)
	}
	seen := map[int]bool{}
	var lastErr error
	for _, length := range lengths {
		if length < 7 || seen[length] {
			continue
		}
		seen[length] = true
		buf := make([]byte, length)
		buf[0] = dualSensePairingFeatureID
		r, _, callErr := procGetFeature.Call(
			uintptr(d.handle),
			uintptr(unsafe.Pointer(&buf[0])),
			uintptr(len(buf)),
		)
		runtime.KeepAlive(buf)
		if r == 0 {
			lastErr = fmt.Errorf("HidD_GetFeature(0x09, %d bytes): %v", length, callErr)
			continue
		}
		addr, err := parseDualSensePairingBluetoothAddress(buf)
		if err == nil {
			return addr, nil
		}
		lastErr = err
	}
	// Some Windows Bluetooth HID collections reject pairing feature 0x09 even
	// though HID enumeration exposes the controller MAC as its serial number.
	// This is also the identity source used by dualsensectl/hidapi when present.
	serialBuf := make([]uint16, 128)
	r, _, serialErr := procGetSerialNumber.Call(
		uintptr(d.handle),
		uintptr(unsafe.Pointer(&serialBuf[0])),
		uintptr(len(serialBuf)*2),
	)
	if r != 0 {
		serial := utf16String(serialBuf)
		if addr, err := parseBluetoothAddressString(serial); err == nil {
			fmt.Printf("Bluetooth power-off: controller address from HID serial %s.\n", formatBluetoothAddress(addr))
			return addr, nil
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("DualSense pairing feature 0x09 unavailable")
	}
	if r == 0 && serialErr != nil {
		return 0, fmt.Errorf("%v; HidD_GetSerialNumberString: %v", lastErr, serialErr)
	}
	return 0, lastErr
}

func disconnectBluetoothAddressViaWindows(addr uint64) error {
	params := bluetoothFindRadioParams{DwSize: uint32(unsafe.Sizeof(bluetoothFindRadioParams{}))}
	var radio syscall.Handle
	find, _, findErr := procBluetoothFindFirstRadio.Call(
		uintptr(unsafe.Pointer(&params)),
		uintptr(unsafe.Pointer(&radio)),
	)
	if find == 0 {
		return fmt.Errorf("BluetoothFindFirstRadio: %v", findErr)
	}
	defer procBluetoothFindRadioClose.Call(find)

	var lastErr error
	for validHandle(radio) {
		var returned uint32
		r, _, callErr := procDeviceIoControl.Call(
			uintptr(radio),
			uintptr(ioctlBthDisconnectDevice),
			uintptr(unsafe.Pointer(&addr)),
			uintptr(unsafe.Sizeof(addr)),
			0,
			0,
			uintptr(unsafe.Pointer(&returned)),
			0,
		)
		procClose.Call(uintptr(radio))
		radio = 0
		runtime.KeepAlive(&addr)
		if r != 0 {
			return nil
		}
		lastErr = fmt.Errorf("DeviceIoControl(IOCTL_BTH_DISCONNECT_DEVICE): %v", callErr)

		next, _, nextErr := procBluetoothFindNextRadio.Call(find, uintptr(unsafe.Pointer(&radio)))
		if next == 0 {
			if lastErr == nil {
				lastErr = fmt.Errorf("BluetoothFindNextRadio: %v", nextErr)
			}
			break
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no usable Bluetooth radio found")
	}
	return lastErr
}

func disconnectDualSenseBluetoothViaWindows(d *device) error {
	addr, err := readDualSenseBluetoothAddress(d)
	if err != nil {
		return err
	}
	if runtimeDiagnosticsEnabled() {
		fmt.Printf("Bluetooth power-off: Windows radio disconnect for DualSense %s.\n", formatBluetoothAddress(addr))
	}
	return disconnectBluetoothAddressViaWindows(addr)
}
