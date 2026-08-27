package main

import "testing"

func TestParseDualSensePairingBluetoothAddress(t *testing.T) {
	report := make([]byte, 20)
	report[0] = 0x09
	// Conventional MAC AA:BB:CC:DD:EE:FF is reported FF EE DD CC BB AA.
	copy(report[1:7], []byte{0xFF, 0xEE, 0xDD, 0xCC, 0xBB, 0xAA})
	addr, err := parseDualSensePairingBluetoothAddress(report)
	if err != nil {
		t.Fatal(err)
	}
	if got := formatBluetoothAddress(addr); got != "AA:BB:CC:DD:EE:FF" {
		t.Fatalf("address=%s", got)
	}
}

func TestParseDualSensePairingBluetoothAddressRejectsEmpty(t *testing.T) {
	report := make([]byte, 20)
	report[0] = 0x09
	if _, err := parseDualSensePairingBluetoothAddress(report); err == nil {
		t.Fatal("zero Bluetooth address accepted")
	}
}

func TestParseBluetoothAddressString(t *testing.T) {
	for _, value := range []string{"AA:BB:CC:DD:EE:FF", "aa-bb-cc-dd-ee-ff", "AABBCCDDEEFF"} {
		addr, err := parseBluetoothAddressString(value)
		if err != nil {
			t.Fatalf("%q: %v", value, err)
		}
		if got := formatBluetoothAddress(addr); got != "AA:BB:CC:DD:EE:FF" {
			t.Fatalf("%q -> %s", value, got)
		}
	}
}
