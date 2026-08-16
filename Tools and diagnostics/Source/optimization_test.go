package main

import (
	"bytes"
	"hash/crc32"
	"testing"
	"time"
)

func TestSonyBluetoothCRCMatchesReferenceConcatenation(t *testing.T) {
	report := make([]byte, bluetoothHaptic36ProtoSize)
	for i := range report {
		report[i] = byte((i*37 + 11) & 0xFF)
	}
	input := make([]byte, 1+bluetoothHaptic36ProtoSize-4)
	input[0] = 0xA2
	copy(input[1:], report[:bluetoothHaptic36ProtoSize-4])
	want := crc32.ChecksumIEEE(input)
	if got := sonyBluetoothCRC(report, bluetoothHaptic36ProtoSize); got != want {
		t.Fatalf("incremental Sony CRC mismatch: got %08X want %08X", got, want)
	}
}

func TestBluetoothReport36ReusableBuilderMatchesReference(t *testing.T) {
	control := telemetry{Version: protocolVersion, Active: true}
	pair := triggerPair{
		L2: resistanceTrigger(0.2, 6.0/48.0, 24.0/48.0),
		R2: fineTrigger(0.1, 1.0/48.0),
	}
	writeTriggerState(&control, pair)
	samples := make([]int8, 64)
	for i := range samples {
		samples[i] = int8((i % 31) - 15)
	}
	state := buildBluetoothSetStateData63(control)
	want := buildBluetoothAllInOneReport36(7, 91, samples, state)
	builder := &bluetoothReport36Builder{}
	got := append([]byte(nil), builder.build(7, 91, samples, control)...)
	if !bytes.Equal(got, want) {
		t.Fatal("reusable 0x36 builder changed the validated Bluetooth report")
	}
}

func TestUSBTransportPathDoesNotAdvanceBluetoothFilter(t *testing.T) {
	input := make([]int8, canonicalFramesForBluetoothFrames(32)*2)
	for i := 0; i < len(input); i += 2 {
		input[i] = int8((i/2)%101 - 50)
		input[i+1] = int8(50 - (i/2)%101)
	}
	withUSB := newCanonicalPCMStream()
	withoutUSB := newCanonicalPCMStream()
	_ = withUSB.processUSB(input)
	got := append([]int8(nil), withUSB.processBluetooth(input)...)
	want := append([]int8(nil), withoutUSB.processBluetooth(input)...)
	if !bytes.Equal(int8Bytes(got), int8Bytes(want)) {
		t.Fatal("USB conversion advanced or modified Bluetooth filter state")
	}
}

func int8Bytes(in []int8) []byte {
	out := make([]byte, len(in))
	for i, v := range in {
		out[i] = byte(v)
	}
	return out
}

func TestSharedFeelEngineReusesRuntimePCMBuffer(t *testing.T) {
	setRuntimeDiagnosticsEnabled(false)
	m := newHapticMixer()
	e := newSharedFeelEngine(m)
	now := time.Now()
	state := telemetry{Version: protocolVersion, Active: true, Raw: &rawTelemetry{GroundedWheels: 4, Speed: 10}}
	m.update(state, now)
	first := e.step(state, now, now, 512).Samples
	if len(first) == 0 {
		t.Fatal("first runtime PCM block is empty")
	}
	firstPtr := &first[0]
	second := e.step(state, now.Add(10*time.Millisecond), now.Add(10*time.Millisecond), 512).Samples
	if len(second) == 0 || firstPtr != &second[0] {
		t.Fatal("shared feel engine did not reuse its runtime PCM buffer")
	}
}

func TestSharedPCMQueueKeepsFixedRingStorage(t *testing.T) {
	var q sharedPCMQueue
	block := make([]int8, 1024)
	q.pushAtRate(block, canonicalHapticSampleRate)
	if len(q.data) == 0 {
		t.Fatal("ring storage was not configured")
	}
	storageLen := len(q.data)
	storagePtr := &q.data[0]
	left := make([]float64, 512)
	right := make([]float64, 512)
	for i := 0; i < 200; i++ {
		q.pushAtRate(block, canonicalHapticSampleRate)
		q.renderInto(left, right, canonicalHapticSampleRate)
	}
	if len(q.data) != storageLen || &q.data[0] != storagePtr {
		t.Fatal("USB PCM queue reallocated instead of keeping fixed ring storage")
	}
	if q.availableSourceFrames() > canonicalHapticSampleRate/20 {
		t.Fatal("USB PCM ring exceeded the 50 ms safety cap")
	}
}
