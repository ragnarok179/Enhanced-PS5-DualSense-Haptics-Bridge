package main

import (
	"encoding/binary"
	"fmt"
)

const (
	dualSenseTouchpadWidth  = 1920
	dualSenseTouchpadHeight = 1080
)

type dualSenseTouchPoint struct {
	Active bool
	ID     byte
	X      int
	Y      int
}

type dualSenseExtendedInput struct {
	ReportID        byte
	ReportSize      int
	Transport       string
	Create          bool
	PS              bool
	Mute            bool
	TouchClick      bool
	EdgeFn1         bool
	EdgeFn2         bool
	EdgeLeft        bool
	EdgeRight       bool
	Touch           [2]dualSenseTouchPoint
	GyroRaw         [3]int16
	AccelRaw        [3]int16
	SensorTimestamp uint32
}

func decodeDualSenseTouchPoint(p []byte) (dualSenseTouchPoint, error) {
	if len(p) < 4 {
		return dualSenseTouchPoint{}, fmt.Errorf("touch point requires 4 bytes, got %d", len(p))
	}
	contact := p[0]
	return dualSenseTouchPoint{
		Active: contact&0x80 == 0,
		ID:     contact & 0x7f,
		X:      int(p[1]) | (int(p[2]&0x0f) << 8),
		Y:      int(p[2]>>4) | (int(p[3]) << 4),
	}, nil
}

// decodeDualSenseExtendedInput decodes only the Sony fields BeamNG currently
// does not expose reliably: Create, PS, Mute, multitouch and DualSense Edge
// rear/Fn buttons. The normal gamepad controls remain owned by BeamNG.
//
// Sony's public Linux driver defines a 63-byte common input body. USB report
// 0x01 prefixes it with one byte; enhanced Bluetooth report 0x31 prefixes it
// with two bytes. Compact Bluetooth report 0x01 does not contain the extended
// touch/sensor body and is intentionally rejected here.
func decodeDualSenseExtendedInput(report []byte) (dualSenseExtendedInput, error) {
	if len(report) == 0 {
		return dualSenseExtendedInput{}, fmt.Errorf("empty DualSense input report")
	}

	commonStart := -1
	transport := ""
	switch report[0] {
	case 0x01:
		if len(report) >= 64 {
			commonStart = 1
			transport = "USB"
		} else {
			return dualSenseExtendedInput{ReportID: report[0], ReportSize: len(report), Transport: "Bluetooth compact"},
				fmt.Errorf("compact Bluetooth report 0x01 (%d bytes) has no extended DualSense payload", len(report))
		}
	case 0x31:
		if len(report) < 78 {
			return dualSenseExtendedInput{ReportID: report[0], ReportSize: len(report), Transport: "Bluetooth"},
				fmt.Errorf("short enhanced Bluetooth report 0x31: %d bytes", len(report))
		}
		commonStart = 2
		transport = "Bluetooth enhanced"
	default:
		return dualSenseExtendedInput{ReportID: report[0], ReportSize: len(report)},
			fmt.Errorf("unsupported DualSense input report id 0x%02X (%d bytes)", report[0], len(report))
	}

	// Common-body offsets are from struct dualsense_input_report in Sony's
	// hid-playstation driver: buttons[1]=8, buttons[2]=9, gyro=15..20,
	// accel=21..26, sensor timestamp=27..30, touch points=32..39.
	if len(report) < commonStart+40 {
		return dualSenseExtendedInput{}, fmt.Errorf("DualSense report too short for extended fields: %d", len(report))
	}
	buttons1 := report[commonStart+8]
	buttons2 := report[commonStart+9]
	var gyroRaw, accelRaw [3]int16
	for i := 0; i < 3; i++ {
		gyroRaw[i] = int16(binary.LittleEndian.Uint16(report[commonStart+15+i*2 : commonStart+17+i*2]))
		accelRaw[i] = int16(binary.LittleEndian.Uint16(report[commonStart+21+i*2 : commonStart+23+i*2]))
	}
	sensorTimestamp := binary.LittleEndian.Uint32(report[commonStart+27 : commonStart+31])
	p1, err := decodeDualSenseTouchPoint(report[commonStart+32 : commonStart+36])
	if err != nil {
		return dualSenseExtendedInput{}, err
	}
	p2, err := decodeDualSenseTouchPoint(report[commonStart+36 : commonStart+40])
	if err != nil {
		return dualSenseExtendedInput{}, err
	}

	return dualSenseExtendedInput{
		ReportID:        report[0],
		ReportSize:      len(report),
		Transport:       transport,
		Create:          buttons1&0x10 != 0,
		PS:              buttons2&0x01 != 0,
		TouchClick:      buttons2&0x02 != 0,
		Mute:            buttons2&0x04 != 0,
		EdgeFn1:         buttons2&0x10 != 0,
		EdgeFn2:         buttons2&0x20 != 0,
		EdgeLeft:        buttons2&0x40 != 0,
		EdgeRight:       buttons2&0x80 != 0,
		Touch:           [2]dualSenseTouchPoint{p1, p2},
		GyroRaw:         gyroRaw,
		AccelRaw:        accelRaw,
		SensorTimestamp: sensorTimestamp,
	}, nil
}

func touchNormX(x int) float64 {
	if x < 0 {
		x = 0
	}
	if x >= dualSenseTouchpadWidth {
		x = dualSenseTouchpadWidth - 1
	}
	return float64(x) / float64(dualSenseTouchpadWidth-1)
}

func touchNormY(y int) float64 {
	if y < 0 {
		y = 0
	}
	if y >= dualSenseTouchpadHeight {
		y = dualSenseTouchpadHeight - 1
	}
	return float64(y) / float64(dualSenseTouchpadHeight-1)
}
