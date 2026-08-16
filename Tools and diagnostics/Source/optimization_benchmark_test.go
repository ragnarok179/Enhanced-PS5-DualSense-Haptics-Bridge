package main

import "testing"

func BenchmarkBluetoothReport36Builder(b *testing.B) {
	builder := &bluetoothReport36Builder{}
	samples := make([]int8, 64)
	control := telemetry{Version: protocolVersion, Active: true}
	pair := triggerPair{
		L2: resistanceTrigger(0.2, 6.0/48.0, 24.0/48.0),
		R2: fineTrigger(0.1, 1.0/48.0),
	}
	writeTriggerState(&control, pair)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = builder.build(byte(i), byte(i>>8), samples, control)
	}
}

func BenchmarkUSBPCMQueueSteadyState(b *testing.B) {
	var q sharedPCMQueue
	block := make([]int8, 1024)
	left := make([]float64, 512)
	right := make([]float64, 512)
	q.pushAtRate(block, canonicalHapticSampleRate)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q.pushAtRate(block, canonicalHapticSampleRate)
		q.renderInto(left, right, canonicalHapticSampleRate)
	}
}

func BenchmarkTransportUSB(b *testing.B) {
	stream := newCanonicalPCMStream()
	block := make([]int8, 1024)
	_ = stream.processUSB(block)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = stream.processUSB(block)
	}
}

func BenchmarkTransportBluetooth(b *testing.B) {
	stream := newCanonicalPCMStream()
	block := make([]int8, canonicalFramesForBluetoothFrames(32)*2)
	_ = stream.processBluetooth(block)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = stream.processBluetooth(block)
	}
}
