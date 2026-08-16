package main

// outputTransport identifies the physical DualSense output path. Gameplay
// synthesis stays shared; only the transport adapter may branch on this value.
//
// USB is the physical reference at 1.00. Bluetooth keeps the same shared
// gameplay synthesis, then applies one transport-wide physical calibration
// gain (0.80 in V1.3) before its mandatory anti-alias filtering,
// 48 kHz -> 3 kHz decimation and HID packet transport.
type outputTransport uint8

const (
	transportReference outputTransport = iota
	transportUSB
	transportBluetooth
)

func (t outputTransport) String() string {
	switch t {
	case transportUSB:
		return "USB"
	case transportBluetooth:
		return "Bluetooth"
	default:
		return "Reference"
	}
}
