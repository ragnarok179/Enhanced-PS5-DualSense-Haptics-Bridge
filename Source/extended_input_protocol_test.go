package main

import "testing"

func TestExtendedInputPacketRoundTrip(t *testing.T) {
	in := extendedInputWireState{
		Create: true, PS: true, Mute: true, EdgeFn1: true, EdgeRight: true,
		OneTap: true, TwoTap: false,
		Touch1: true, Touch2: true, EdgeDevice: true, Bluetooth: true,
		Seq: 233, Count: 2, ID1: 7, ID2: 19,
		X1: 1919, Y1: 1079, X2: 960, Y2: 540,
		OneDX: -12345, OneDY: 23456, TwoDX: -2222, TwoDY: 3333, Pinch: -4444,
	}
	buf := make([]byte, extendedInputPacketSize)
	p, err := encodeExtendedInputPacket(in, buf)
	if err != nil {
		t.Fatal(err)
	}
	out, err := decodeExtendedInputPacket(p)
	if err != nil {
		t.Fatal(err)
	}
	if out != in {
		t.Fatalf("round trip mismatch: got %+v want %+v", out, in)
	}
}

func TestExtendedInputPacketRejectsWrongVersion(t *testing.T) {
	p := make([]byte, extendedInputPacketSize)
	copy(p[:4], extendedInputMagic[:])
	p[4] = 99
	if _, err := decodeExtendedInputPacket(p); err == nil {
		t.Fatal("expected version error")
	}
}

func TestQ15SignedUnit(t *testing.T) {
	vals := []float64{-1, -0.5, 0, 0.5, 1}
	for _, v := range vals {
		got := q15ToSignedUnit(signedUnitToQ15(v))
		if got-v > 0.0001 || v-got > 0.0001 {
			t.Fatalf("%v -> %v", v, got)
		}
	}
}
