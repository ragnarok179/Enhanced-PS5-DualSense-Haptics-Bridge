package main

import (
	"fmt"
	"strconv"
	"strings"
)

const dualSensePairingFeatureID = 0x09

func parseDualSensePairingBluetoothAddress(report []byte) (uint64, error) {
	if len(report) < 7 || report[0] != dualSensePairingFeatureID {
		return 0, fmt.Errorf("invalid DualSense pairing feature report")
	}
	// Feature 0x09 stores the controller Bluetooth address least-significant
	// byte first. RPCS3 renders the conventional MAC from report[6]..report[1].
	var addr uint64
	allZero, allFF := true, true
	for i := 0; i < 6; i++ {
		b := report[1+i]
		addr |= uint64(b) << (8 * i)
		allZero = allZero && b == 0
		allFF = allFF && b == 0xff
	}
	if allZero || allFF {
		return 0, fmt.Errorf("invalid Bluetooth address in DualSense pairing feature")
	}
	return addr, nil
}

func formatBluetoothAddress(addr uint64) string {
	return fmt.Sprintf("%02X:%02X:%02X:%02X:%02X:%02X",
		byte(addr>>40), byte(addr>>32), byte(addr>>24), byte(addr>>16), byte(addr>>8), byte(addr))
}

func parseBluetoothAddressString(value string) (uint64, error) {
	compact := strings.NewReplacer(":", "", "-", "", " ", "").Replace(strings.TrimSpace(value))
	if len(compact) != 12 {
		return 0, fmt.Errorf("invalid Bluetooth address string")
	}
	var addr uint64
	for i := 0; i < 6; i++ {
		b, err := strconv.ParseUint(compact[i*2:i*2+2], 16, 8)
		if err != nil {
			return 0, fmt.Errorf("invalid Bluetooth address string")
		}
		addr = (addr << 8) | b
	}
	if addr == 0 || addr == 0xffffffffffff {
		return 0, fmt.Errorf("invalid Bluetooth address string")
	}
	return addr, nil
}
