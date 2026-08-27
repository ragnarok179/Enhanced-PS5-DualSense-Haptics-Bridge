package main

import (
	"encoding/binary"
	"math"
	"testing"
)

func putI16(b []byte, off int, v int16) { binary.LittleEndian.PutUint16(b[off:off+2], uint16(v)) }

func testCalibrationFeature() []byte {
	b := make([]byte, 41)
	b[0] = 0x05
	// gyro biases 0; +/-1000 reference; speed_2x=2000 -> 1 raw unit = 1 dps.
	for _, off := range []int{1, 3, 5} {
		putI16(b, off, 0)
	}
	putI16(b, 7, 1000)
	putI16(b, 9, -1000)
	putI16(b, 11, 1000)
	putI16(b, 13, -1000)
	putI16(b, 15, 1000)
	putI16(b, 17, -1000)
	putI16(b, 19, 1000)
	putI16(b, 21, 1000)
	// +/-1g references -> normal DualSense 8192 units/g.
	putI16(b, 23, 8192)
	putI16(b, 25, -8192)
	putI16(b, 27, 8192)
	putI16(b, 29, -8192)
	putI16(b, 31, 8192)
	putI16(b, 33, -8192)
	return b
}

func TestDualSenseMotionCalibration(t *testing.T) {
	c, err := parseDualSenseMotionCalibration(testCalibrationFeature())
	if err != nil {
		t.Fatal(err)
	}
	raw := dualSenseExtendedInput{GyroRaw: [3]int16{120, -30, 45}, AccelRaw: [3]int16{8192, 0, -8192}}
	s := c.apply(raw)
	wantG := [3]float64{1, 0, -1}
	for i := 0; i < 3; i++ {
		if math.Abs(s.AccelG[i]-wantG[i]) > 1e-6 {
			t.Fatalf("accel[%d]=%f", i, s.AccelG[i])
		}
	}
	if math.Abs(s.GyroDPS[0]-120) > 1e-6 || math.Abs(s.GyroDPS[1]+30) > 1e-6 || math.Abs(s.GyroDPS[2]-45) > 1e-6 {
		t.Fatalf("gyro=%v", s.GyroDPS)
	}
}

func TestMotionPacketRoundTrip(t *testing.T) {
	in := motionInputWireState{
		Bluetooth: true, Calibrated: true, Seq: 19, SensorTimestamp: 0x89abcdef,
		Axes:          [9]int16{-32767, -123, 0, 123, 32767, 42, -42, 999, -999},
		CorrectedGyro: [3]int16{101, -202, 303}, Gravity: [3]int16{11, -32767, 22},
		LinearAccel: [3]int16{-404, 505, -606}, Quaternion: [4]int16{32000, 1000, -2000, 3000},
	}
	buf := make([]byte, motionInputPacketSize)
	p, err := encodeMotionInputPacket(in, buf)
	if err != nil {
		t.Fatal(err)
	}
	out, err := decodeMotionInputPacket(p)
	if err != nil {
		t.Fatal(err)
	}
	if out != in {
		t.Fatalf("roundtrip mismatch: %#v %#v", in, out)
	}
}

func TestDualSenseInputMotionOffsetsUSB(t *testing.T) {
	r := make([]byte, 64)
	r[0] = 0x01
	common := 1
	valsG := [3]int16{111, -222, 333}
	valsA := [3]int16{-444, 555, -666}
	for i := 0; i < 3; i++ {
		putI16(r, common+15+i*2, valsG[i])
		putI16(r, common+21+i*2, valsA[i])
	}
	binary.LittleEndian.PutUint32(r[common+27:common+31], 0x12345678)
	// mark touch inactive
	r[common+32] = 0x80
	r[common+36] = 0x80
	d, err := decodeDualSenseExtendedInput(r)
	if err != nil {
		t.Fatal(err)
	}
	if d.GyroRaw != valsG || d.AccelRaw != valsA || d.SensorTimestamp != 0x12345678 {
		t.Fatalf("decoded=%+v", d)
	}
}

func TestDualSenseOrientationFlat(t *testing.T) {
	o := dualSenseOrientation{}
	o.Update(dualSenseMotionSample{AccelG: [3]float64{0, 1, 0}, Timestamp: 1000, Calibrated: true})
	if math.Abs(o.RollDeg) > 0.01 || math.Abs(o.PitchDeg) > 0.01 {
		t.Fatalf("flat orientation roll=%f pitch=%f", o.RollDeg, o.PitchDeg)
	}
}

func TestDualSenseOrientationInitialPitchAndRoll(t *testing.T) {
	const deg = math.Pi / 180
	{
		o := dualSenseOrientation{}
		// +30 degrees DS pitch: gravity shifts from +Y toward -Z.
		o.Update(dualSenseMotionSample{AccelG: [3]float64{0, math.Cos(30 * deg), -math.Sin(30 * deg)}, Timestamp: 1000})
		if math.Abs(o.PitchDeg-30) > 0.2 || math.Abs(o.RollDeg) > 0.2 {
			t.Fatalf("pitch init roll=%f pitch=%f", o.RollDeg, o.PitchDeg)
		}
	}
	{
		o := dualSenseOrientation{}
		// +30 degrees DS roll: gravity shifts from +Y toward +X.
		o.Update(dualSenseMotionSample{AccelG: [3]float64{math.Sin(30 * deg), math.Cos(30 * deg), 0}, Timestamp: 1000})
		if math.Abs(o.RollDeg-30) > 0.2 || math.Abs(o.PitchDeg) > 0.2 {
			t.Fatalf("roll init roll=%f pitch=%f", o.RollDeg, o.PitchDeg)
		}
	}
}

func TestDualSenseOrientationYawIntegration(t *testing.T) {
	o := dualSenseOrientation{}
	ts := uint32(1000)
	o.Update(dualSenseMotionSample{AccelG: [3]float64{0, 1, 0}, Timestamp: ts})
	for i := 0; i < 100; i++ {
		ts += 30000 // 10 ms at ~3 MHz.
		o.Update(dualSenseMotionSample{GyroDPS: [3]float64{0, 90, 0}, AccelG: [3]float64{0, 1, 0}, Timestamp: ts})
	}
	if math.Abs(o.YawDeg-90) > 2.0 {
		t.Fatalf("yaw=%f, want about 90", o.YawDeg)
	}
}

func TestOrientationResidualGyroBiasConvergesWhileStill(t *testing.T) {
	o := dualSenseOrientation{}
	const dtTicks = uint32(12000) // 4 ms at Sony's 3 MHz sensor clock
	var ts uint32 = 1000
	for i := 0; i < 15000; i++ { // 60 s
		ts += dtTicks
		o.Update(dualSenseMotionSample{
			GyroDPS:    [3]float64{0.26, 0.05, -0.10},
			AccelG:     [3]float64{0, 1, 0},
			Timestamp:  ts,
			Calibrated: true,
		})
	}
	bias, ok := o.GyroBiasDPS()
	if !ok {
		t.Fatal("residual gyro bias was not acquired")
	}
	want := [3]float64{0.26, 0.05, -0.10}
	for i := 0; i < 3; i++ {
		if math.Abs(bias[i]-want[i]) > 0.005 {
			t.Fatalf("bias[%d]=%f want %f", i, bias[i], want[i])
		}
	}
	if math.Abs(o.YawDeg) > 0.10 {
		t.Fatalf("stationary yaw drift too large after 60s: %f deg", o.YawDeg)
	}
}

func TestOrientationBiasDoesNotLearnDeliberateMotion(t *testing.T) {
	o := dualSenseOrientation{}
	const dtTicks = uint32(12000)
	var ts uint32 = 1000
	for i := 0; i < 1000; i++ {
		ts += dtTicks
		o.Update(dualSenseMotionSample{GyroDPS: [3]float64{0.25, 0.04, -0.10}, AccelG: [3]float64{0, 1, 0}, Timestamp: ts, Calibrated: true})
	}
	before, ok := o.GyroBiasDPS()
	if !ok {
		t.Fatal("bias not initialized")
	}
	for i := 0; i < 250; i++ { // 1 s real yaw rotation
		ts += dtTicks
		o.Update(dualSenseMotionSample{GyroDPS: [3]float64{0.25, 20.04, -0.10}, AccelG: [3]float64{0, 1, 0}, Timestamp: ts, Calibrated: true})
	}
	after, _ := o.GyroBiasDPS()
	if math.Abs(after[1]-before[1]) > 0.002 {
		t.Fatalf("deliberate yaw motion polluted bias: before=%f after=%f", before[1], after[1])
	}
}

func TestMotionSensorDueAtDualSenseCadence(t *testing.T) {
	base := uint32(100000)
	if !motionSensorDue(0, base) {
		t.Fatal("first motion packet must be due")
	}
	// One 4 ms HID frame is too soon; two frames (~8 ms) are due.
	if motionSensorDue(base, base+12000) {
		t.Fatal("4 ms frame should not be emitted")
	}
	if !motionSensorDue(base, base+24000) {
		t.Fatal("8 ms frame should be emitted")
	}
	if !motionSensorDue(base, base+600000) { // 200 ms: force a resync
		t.Fatal("large sensor gap must resync instead of freezing the stream")
	}
}

func TestMappedMotionDerivedSignalsFlat(t *testing.T) {
	o := dualSenseOrientation{}
	s := dualSenseMotionSample{
		GyroDPS:   [3]float64{0.25, -0.10, 0.05},
		AccelG:    [3]float64{0, 1, 0},
		Timestamp: 1000,
	}
	o.Update(s)
	g := o.GravityLocal()
	if math.Abs(g[0]) > 1e-5 || math.Abs(g[1]+1) > 1e-5 || math.Abs(g[2]) > 1e-5 {
		t.Fatalf("flat gravity=%v, want approximately [0 -1 0]", g)
	}
	la := o.LinearAccelG(s)
	for i := 0; i < 3; i++ {
		if math.Abs(la[i]) > 1e-5 {
			t.Fatalf("flat linear accel=%v, want ~0", la)
		}
	}
	q := o.QuaternionQ15()
	if q[0] < 32760 || math.Abs(float64(q[1])) > 2 || math.Abs(float64(q[2])) > 2 || math.Abs(float64(q[3])) > 2 {
		t.Fatalf("flat quaternion q15=%v", q)
	}
}

func TestMappedMotionCorrectedGyroUsesResidualBiasOnly(t *testing.T) {
	o := dualSenseOrientation{biasInitialized: true, gyroBiasDPS: [3]float64{0.25, -0.10, 0.05}}
	s := dualSenseMotionSample{GyroDPS: [3]float64{10.25, 19.90, -4.95}}
	got := o.CorrectedGyroDPS(s)
	want := [3]float64{10, 20, -5}
	for i := range want {
		if math.Abs(got[i]-want[i]) > 1e-9 {
			t.Fatalf("corrected gyro=%v want=%v", got, want)
		}
	}
	if s.GyroDPS != [3]float64{10.25, 19.90, -4.95} {
		t.Fatalf("physical gyro mutated: %v", s.GyroDPS)
	}
}
