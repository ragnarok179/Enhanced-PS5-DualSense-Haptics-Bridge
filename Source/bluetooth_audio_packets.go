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

// buildBluetoothAllInOneReport36 follows DS5Dongle's current wireless
// transport. A single report carries audio state, a 63-byte SetStateData
// snapshot and the 64-byte stereo haptic block. There is no USB report ID
// inside SetStateData: byte 13 of the outer packet is SetStateData byte 0.
func buildBluetoothAllInOneReport36WithSpeaker(sequence, audioCounter byte, samples []int8, state63 []byte, speakerFrame []byte) []byte {
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

	// Sized packet 0x13: internal membrane speaker, Opus 48 kHz stereo CBR
	// 160 kb/s = exactly 200 bytes/frame. It shares this 0x36 report so the
	// existing scheduler remains the only Bluetooth HID writer.
	if len(speakerFrame) >= 200 {
		report[142] = 0x93
		report[143] = 200
		copy(report[144:344], speakerFrame[:200])
	}

	binary.LittleEndian.PutUint32(report[394:398], sonyBluetoothCRC(report, bluetoothHaptic36ProtoSize))
	return report
}

func buildBluetoothAllInOneReport36(sequence, audioCounter byte, samples []int8, state63 []byte) []byte {
	return buildBluetoothAllInOneReport36WithSpeaker(sequence, audioCounter, samples, state63, nil)
}
