//go:build windows && !bluetooth

package main

import "fmt"

const btOpusPacketSize = 200
const btSpeakerSourceTickMono = 512

type bluetoothOpusStreamEncoder struct{}

func newBluetoothOpusStreamEncoder(_ string) (*bluetoothOpusStreamEncoder, string, error) {
	return nil, "", fmt.Errorf("Bluetooth Opus encoder is not part of this build")
}

func (*bluetoothOpusStreamEncoder) EncodeSourceTick(_ []float32) ([]byte, error) {
	return nil, fmt.Errorf("Bluetooth Opus encoder is not part of this build")
}

func (*bluetoothOpusStreamEncoder) Close() {}

func encodeBluetoothOpusCollisionPCM(_ []float32, _ string) ([][]byte, string, error) {
	return nil, "", fmt.Errorf("Bluetooth Opus encoder is not part of this build")
}
