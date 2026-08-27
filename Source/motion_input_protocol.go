package main

import (
	"encoding/binary"
	"errors"
)

const (
	motionInputProtocolVersion = 2
	motionInputPacketSize      = 56
)

var motionInputMagic = [4]byte{'D', 'S', 'M', 'O'}

type motionInputWireState struct {
	Bluetooth       bool
	Calibrated      bool
	Seq             byte
	Axes            [9]int16 // legacy: gyro3, accel3, Euler3
	CorrectedGyro   [3]int16 // residual-bias corrected gyro for mapped motion
	Gravity         [3]int16 // local gravity vector, GamepadMotionHelpers sign
	LinearAccel     [3]int16 // accelerometer with gravity removed
	Quaternion      [4]int16 // fused orientation quaternion in filter frame, Q15 w/x/y/z
	SensorTimestamp uint32
}

func encodeMotionInputPacket(s motionInputWireState, dst []byte) ([]byte, error) {
	if len(dst) < motionInputPacketSize {
		return nil, errors.New("motion packet buffer too small")
	}
	p := dst[:motionInputPacketSize]
	copy(p[:4], motionInputMagic[:])
	p[4] = motionInputProtocolVersion
	var flags byte
	if s.Bluetooth {
		flags |= 1
	}
	if s.Calibrated {
		flags |= 2
	}
	p[5] = flags
	p[6] = s.Seq
	p[7] = 0
	off := 8
	for _, set := range [][3]int16{
		{s.Axes[0], s.Axes[1], s.Axes[2]},
		{s.Axes[3], s.Axes[4], s.Axes[5]},
		{s.Axes[6], s.Axes[7], s.Axes[8]},
		s.CorrectedGyro,
		s.Gravity,
		s.LinearAccel,
	} {
		for _, v := range set {
			binary.LittleEndian.PutUint16(p[off:off+2], uint16(v))
			off += 2
		}
	}
	for _, v := range s.Quaternion {
		binary.LittleEndian.PutUint16(p[off:off+2], uint16(v))
		off += 2
	}
	binary.LittleEndian.PutUint32(p[52:56], s.SensorTimestamp)
	return p, nil
}

func decodeMotionInputPacket(p []byte) (motionInputWireState, error) {
	if len(p) != motionInputPacketSize {
		return motionInputWireState{}, errors.New("invalid motion packet size")
	}
	if p[0] != motionInputMagic[0] || p[1] != motionInputMagic[1] || p[2] != motionInputMagic[2] || p[3] != motionInputMagic[3] {
		return motionInputWireState{}, errors.New("invalid motion magic")
	}
	if p[4] != motionInputProtocolVersion {
		return motionInputWireState{}, errors.New("unsupported motion version")
	}
	var s motionInputWireState
	s.Bluetooth = p[5]&1 != 0
	s.Calibrated = p[5]&2 != 0
	s.Seq = p[6]
	off := 8
	for i := 0; i < 9; i++ {
		s.Axes[i] = int16(binary.LittleEndian.Uint16(p[off : off+2]))
		off += 2
	}
	for i := 0; i < 3; i++ {
		s.CorrectedGyro[i] = int16(binary.LittleEndian.Uint16(p[off : off+2]))
		off += 2
	}
	for i := 0; i < 3; i++ {
		s.Gravity[i] = int16(binary.LittleEndian.Uint16(p[off : off+2]))
		off += 2
	}
	for i := 0; i < 3; i++ {
		s.LinearAccel[i] = int16(binary.LittleEndian.Uint16(p[off : off+2]))
		off += 2
	}
	for i := 0; i < 4; i++ {
		s.Quaternion[i] = int16(binary.LittleEndian.Uint16(p[off : off+2]))
		off += 2
	}
	s.SensorTimestamp = binary.LittleEndian.Uint32(p[52:56])
	return s, nil
}
