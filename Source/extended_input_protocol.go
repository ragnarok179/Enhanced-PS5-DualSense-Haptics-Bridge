package main

import (
	"encoding/binary"
	"errors"
	"math"
)

const (
	extendedInputProtocolVersion = 4
	extendedInputPacketSize      = 30
	extendedInputEdgePID         = 0x0DF2
)

var extendedInputMagic = [4]byte{'D', 'S', 'E', '4'}

type extendedInputWireState struct {
	Create    bool
	PS        bool
	Mute      bool
	EdgeFn1   bool
	EdgeFn2   bool
	EdgeLeft  bool
	EdgeRight bool

	OneTap bool
	TwoTap bool

	Touch1     bool
	Touch2     bool
	EdgeDevice bool
	Bluetooth  bool
	TouchMode  byte
	Seq        byte
	Count      byte
	ID1        byte
	ID2        byte

	X1, Y1 uint16
	X2, Y2 uint16

	OneDX, OneDY int16
	TwoDX, TwoDY int16
	Pinch        int16
}

func signedUnitToQ15(v float64) int16 {
	if v > 1 {
		v = 1
	}
	if v < -1 {
		v = -1
	}
	return int16(math.Round(v * 32767.0))
}

func q15ToSignedUnit(v int16) float64 {
	return float64(v) / 32767.0
}

func wireStateFromDualSense(s dualSenseExtendedInput, frame logicalTouchFrame, productID uint16, seq byte) extendedInputWireState {
	w := extendedInputWireState{
		Create: s.Create, PS: s.PS, Mute: s.Mute,
		EdgeFn1: s.EdgeFn1, EdgeFn2: s.EdgeFn2, EdgeLeft: s.EdgeLeft, EdgeRight: s.EdgeRight,
		OneTap: frame.OneTap, TwoTap: frame.TwoTap,
		Touch1: frame.PrimaryActive, Touch2: frame.SecondaryActive,
		EdgeDevice: productID == extendedInputEdgePID,
		Bluetooth:  s.ReportID == 0x31,
		TouchMode:  0, // BeamNG owns the touchpad representation.
		Seq:        seq, Count: frame.Count,
		ID1: frame.PrimaryID, ID2: frame.SecondaryID,
		X1: uint16(frame.PrimaryX), Y1: uint16(frame.PrimaryY),
		X2: uint16(frame.SecondaryX), Y2: uint16(frame.SecondaryY),
		OneDX: signedUnitToQ15(frame.OneDX), OneDY: signedUnitToQ15(frame.OneDY),
		TwoDX: signedUnitToQ15(frame.TwoDX), TwoDY: signedUnitToQ15(frame.TwoDY),
		Pinch: signedUnitToQ15(frame.Pinch),
	}
	return w
}

func neutralizeExtendedTouch(s *extendedInputWireState) {
	if s == nil {
		return
	}
	s.OneTap = false
	s.TwoTap = false
	s.Touch1 = false
	s.Touch2 = false
	s.Count = 0
	s.ID1 = 0
	s.ID2 = 0
	s.X1, s.Y1, s.X2, s.Y2 = 0, 0, 0, 0
	s.OneDX, s.OneDY = 0, 0
	s.TwoDX, s.TwoDY = 0, 0
	s.Pinch = 0
}

func encodeExtendedInputPacket(s extendedInputWireState, dst []byte) ([]byte, error) {
	if len(dst) < extendedInputPacketSize {
		return nil, errors.New("extended input packet buffer too small")
	}
	p := dst[:extendedInputPacketSize]
	copy(p[0:4], extendedInputMagic[:])
	p[4] = extendedInputProtocolVersion

	var buttons byte
	if s.Create {
		buttons |= 1 << 0
	}
	if s.PS {
		buttons |= 1 << 1
	}
	if s.Mute {
		buttons |= 1 << 2
	}
	if s.EdgeFn1 {
		buttons |= 1 << 3
	}
	if s.EdgeFn2 {
		buttons |= 1 << 4
	}
	if s.EdgeLeft {
		buttons |= 1 << 5
	}
	if s.EdgeRight {
		buttons |= 1 << 6
	}
	p[5] = buttons

	var gestures byte
	if s.OneTap {
		gestures |= 1 << 0
	}
	if s.TwoTap {
		gestures |= 1 << 1
	}
	p[6] = gestures

	var touchFlags byte
	if s.Touch1 {
		touchFlags |= 1 << 0
	}
	if s.Touch2 {
		touchFlags |= 1 << 1
	}
	if s.EdgeDevice {
		touchFlags |= 1 << 2
	}
	if s.Bluetooth {
		touchFlags |= 1 << 3
	}
	touchFlags |= (s.TouchMode & 0x03) << 4
	p[7] = touchFlags
	p[8] = s.Seq
	p[9] = s.Count
	p[10] = s.ID1
	p[11] = s.ID2

	binary.LittleEndian.PutUint16(p[12:14], s.X1)
	binary.LittleEndian.PutUint16(p[14:16], s.Y1)
	binary.LittleEndian.PutUint16(p[16:18], s.X2)
	binary.LittleEndian.PutUint16(p[18:20], s.Y2)
	binary.LittleEndian.PutUint16(p[20:22], uint16(s.OneDX))
	binary.LittleEndian.PutUint16(p[22:24], uint16(s.OneDY))
	binary.LittleEndian.PutUint16(p[24:26], uint16(s.TwoDX))
	binary.LittleEndian.PutUint16(p[26:28], uint16(s.TwoDY))
	binary.LittleEndian.PutUint16(p[28:30], uint16(s.Pinch))
	return p, nil
}

func decodeExtendedInputPacket(p []byte) (extendedInputWireState, error) {
	if len(p) != extendedInputPacketSize {
		return extendedInputWireState{}, errors.New("invalid extended input packet size")
	}
	if p[0] != extendedInputMagic[0] || p[1] != extendedInputMagic[1] || p[2] != extendedInputMagic[2] || p[3] != extendedInputMagic[3] {
		return extendedInputWireState{}, errors.New("invalid extended input packet magic")
	}
	if p[4] != extendedInputProtocolVersion {
		return extendedInputWireState{}, errors.New("unsupported extended input packet version")
	}
	buttons, gestures, flags := p[5], p[6], p[7]
	return extendedInputWireState{
		Create: buttons&(1<<0) != 0, PS: buttons&(1<<1) != 0, Mute: buttons&(1<<2) != 0,
		EdgeFn1: buttons&(1<<3) != 0, EdgeFn2: buttons&(1<<4) != 0,
		EdgeLeft: buttons&(1<<5) != 0, EdgeRight: buttons&(1<<6) != 0,
		OneTap: gestures&(1<<0) != 0, TwoTap: gestures&(1<<1) != 0,
		Touch1: flags&(1<<0) != 0, Touch2: flags&(1<<1) != 0,
		EdgeDevice: flags&(1<<2) != 0, Bluetooth: flags&(1<<3) != 0, TouchMode: (flags >> 4) & 0x03,
		Seq: p[8], Count: p[9], ID1: p[10], ID2: p[11],
		X1: binary.LittleEndian.Uint16(p[12:14]), Y1: binary.LittleEndian.Uint16(p[14:16]),
		X2: binary.LittleEndian.Uint16(p[16:18]), Y2: binary.LittleEndian.Uint16(p[18:20]),
		OneDX: int16(binary.LittleEndian.Uint16(p[20:22])), OneDY: int16(binary.LittleEndian.Uint16(p[22:24])),
		TwoDX: int16(binary.LittleEndian.Uint16(p[24:26])), TwoDY: int16(binary.LittleEndian.Uint16(p[26:28])),
		Pinch: int16(binary.LittleEndian.Uint16(p[28:30])),
	}, nil
}

func extendedButtonsChanged(a, b extendedInputWireState) bool {
	return a.Create != b.Create || a.PS != b.PS || a.Mute != b.Mute ||
		a.EdgeFn1 != b.EdgeFn1 || a.EdgeFn2 != b.EdgeFn2 ||
		a.EdgeLeft != b.EdgeLeft || a.EdgeRight != b.EdgeRight ||
		a.OneTap != b.OneTap || a.TwoTap != b.TwoTap ||
		a.Touch1 != b.Touch1 || a.Touch2 != b.Touch2 || a.Count != b.Count ||
		a.ID1 != b.ID1 || a.ID2 != b.ID2 || a.EdgeDevice != b.EdgeDevice || a.TouchMode != b.TouchMode
}

func extendedTouchChanged(a, b extendedInputWireState) bool {
	return a.X1 != b.X1 || a.Y1 != b.Y1 || a.X2 != b.X2 || a.Y2 != b.Y2 ||
		a.OneDX != b.OneDX || a.OneDY != b.OneDY || a.TwoDX != b.TwoDX ||
		a.TwoDY != b.TwoDY || a.Pinch != b.Pinch
}
