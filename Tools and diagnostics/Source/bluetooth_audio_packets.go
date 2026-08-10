package main

import (
	"encoding/binary"
	"hash/crc32"
)

// Bluetooth audio-haptic packet builders live here so transport framing stays
// separate from haptic synthesis and gameplay interpretation.

func sonyBluetoothCRC(report []byte, protocolLength int) uint32 {
	if protocolLength < 4 || protocolLength > len(report) {
		return 0
	}
	// Sony output CRC is the IEEE CRC32 of the HIDP output prefix 0xA2
	// followed by the report up to, but excluding, its final CRC field.
	input := make([]byte, 1+protocolLength-4)
	input[0] = 0xA2
	copy(input[1:], report[:protocolLength-4])
	return crc32.ChecksumIEEE(input)
}

// buildBluetoothHapticReport32 follows SAxense's direct HID haptics-only
// report and the Gamepad-Core Windows policy. The Windows HID descriptor may
// advertise a 547-byte maximum report, but ProcessAudioHaptic writes this
// protocol with its exact 142-byte length.
func buildBluetoothHapticReport32(_ byte, audioCounter byte, samples []int8, _ int) []byte {
	report := make([]byte, bluetoothHaptic32ProtoSize)
	report[0] = 0x32
	// SAxense and Gamepad-Core do not use an outer HID sequence for this audio
	// stream. The continuously incremented audio counter is report[10].
	report[1] = 0x00

	// Sized packet 0x11, seven bytes of audio state/counters.
	report[2] = 0x91
	report[3] = 7
	report[4] = 0xFE
	report[5] = 0
	report[6] = 0
	report[7] = 0
	report[8] = 0
	report[9] = 0xFF
	report[10] = audioCounter

	// Sized packet 0x12, 64 bytes of interleaved signed stereo haptics.
	report[11] = 0x92
	report[12] = 64
	limit := minInt(len(samples), 64)
	for i := 0; i < limit; i++ {
		report[13+i] = byte(samples[i])
	}
	binary.LittleEndian.PutUint32(report[138:142], sonyBluetoothCRC(report, bluetoothHaptic32ProtoSize))
	return report
}

// buildBluetoothAllInOneReport36 follows DS5Dongle's current wireless
// transport. A single report carries audio state, a 63-byte SetStateData
// snapshot and the 64-byte stereo haptic block. There is no USB report ID
// inside SetStateData: byte 13 of the outer packet is SetStateData byte 0.
func buildBluetoothAllInOneReport36(sequence, audioCounter byte, samples []int8, state63 []byte) []byte {
	report := make([]byte, bluetoothHaptic36ProtoSize)
	report[0] = 0x36
	report[1] = (sequence & 0x0F) << 4
	report[2] = 0x91 // sized packet: audio state
	report[3] = 7
	report[4] = 0xFE // haptics/speaker path enabled, mic streaming disabled
	// DS5Dongle defaults audio_buffer_length to 64 and writes the same value
	// to all five buffer-length fields.
	report[5], report[6], report[7], report[8], report[9] = 64, 64, 64, 64, 64
	report[10] = audioCounter

	report[11] = 0x90 // sized packet 0x10 with the high "present" bit
	report[12] = 63
	if len(state63) > 63 {
		state63 = state63[:63]
	}
	copy(report[13:76], state63)

	report[76] = 0x92 // sized packet 0x12 with the high "present" bit
	report[77] = 64
	limit := minInt(len(samples), 64)
	for i := 0; i < limit; i++ {
		report[78+i] = byte(samples[i])
	}

	binary.LittleEndian.PutUint32(report[394:398], sonyBluetoothCRC(report, bluetoothHaptic36ProtoSize))
	return report
}

// buildBluetoothHapticReport39 follows DS5Dongle's combined Bluetooth audio
// report. It remains a diagnostic path and uses its exact 547-byte protocol.
func buildBluetoothHapticReport39(sequence, audioCounter byte, samples []int8, _ int) []byte {
	report := make([]byte, bluetoothHaptic39ProtoSize)
	report[0] = 0x39
	report[1] = (sequence & 0x0F) << 4
	report[2] = 0x91
	report[3] = 0x06
	report[4] = 0x7E
	// DS5Dongle sends four separate one-byte buffer-length fields.
	report[5], report[6], report[7], report[8] = 64, 64, 64, 64
	report[9] = audioCounter
	report[10] = 0xD2
	report[11] = 64
	limit := minInt(len(samples), 128)
	for i := 0; i < limit; i++ {
		report[12+i] = byte(samples[i])
	}
	binary.LittleEndian.PutUint32(report[543:547], sonyBluetoothCRC(report, bluetoothHaptic39ProtoSize))
	return report
}
