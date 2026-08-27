package main

import (
	"encoding/binary"
	"errors"
	"math"
)

const (
	dualSenseAccelResPerG  = 8192.0
	dualSenseAccelRangeG   = 4.0
	dualSenseGyroResPerDPS = 1024.0
	dualSenseGyroRangeDPS  = 2048.0
)

type motionAxisCalibration struct {
	bias  float64
	numer float64
	denom float64
}

type dualSenseMotionCalibration struct {
	valid bool
	gyro  [3]motionAxisCalibration
	accel [3]motionAxisCalibration
}

type dualSenseMotionSample struct {
	GyroDPS    [3]float64 // Sony order: pitch, yaw, roll
	AccelG     [3]float64 // x, y, z
	Timestamp  uint32
	Calibrated bool
}

func leI16(b []byte, off int) int16 {
	return int16(binary.LittleEndian.Uint16(b[off : off+2]))
}

// parseDualSenseMotionCalibration follows Sony's hid-playstation calibration
// layout and normalization equations. The feature report is 0x05 / 41 bytes.
func parseDualSenseMotionCalibration(buf []byte) (dualSenseMotionCalibration, error) {
	if len(buf) < 35 || buf[0] != 0x05 {
		return dualSenseMotionCalibration{}, errors.New("invalid DualSense calibration feature report")
	}
	pitchBias := float64(leI16(buf, 1))
	yawBias := float64(leI16(buf, 3))
	rollBias := float64(leI16(buf, 5))
	plus := [3]float64{float64(leI16(buf, 7)), float64(leI16(buf, 11)), float64(leI16(buf, 15))}
	minus := [3]float64{float64(leI16(buf, 9)), float64(leI16(buf, 13)), float64(leI16(buf, 17))}
	bias := [3]float64{pitchBias, yawBias, rollBias}
	speed2x := float64(leI16(buf, 19)) + float64(leI16(buf, 21))

	var c dualSenseMotionCalibration
	c.valid = true
	for i := 0; i < 3; i++ {
		denom := math.Abs(plus[i]-bias[i]) + math.Abs(minus[i]-bias[i])
		if denom == 0 || speed2x == 0 {
			c.valid = false
			break
		}
		// Linux normalizes to 1/1024 degree/s. We keep physical deg/s.
		c.gyro[i] = motionAxisCalibration{bias: 0, numer: speed2x, denom: denom}
	}

	accelPlus := [3]float64{float64(leI16(buf, 23)), float64(leI16(buf, 27)), float64(leI16(buf, 31))}
	accelMinus := [3]float64{float64(leI16(buf, 25)), float64(leI16(buf, 29)), float64(leI16(buf, 33))}
	for i := 0; i < 3; i++ {
		range2g := accelPlus[i] - accelMinus[i]
		if range2g == 0 {
			c.valid = false
			break
		}
		accBias := accelPlus[i] - range2g/2
		// Linux output divided by 8192 gives g, reducing to 2/range2g here.
		c.accel[i] = motionAxisCalibration{bias: accBias, numer: 2.0, denom: range2g}
	}
	if !c.valid {
		return dualSenseMotionCalibration{}, errors.New("invalid DualSense motion calibration denominators")
	}
	return c, nil
}

func (c dualSenseMotionCalibration) apply(raw dualSenseExtendedInput) dualSenseMotionSample {
	out := dualSenseMotionSample{Timestamp: raw.SensorTimestamp, Calibrated: c.valid}
	for i := 0; i < 3; i++ {
		rv := float64(raw.GyroRaw[i])
		if c.valid {
			out.GyroDPS[i] = (rv - c.gyro[i].bias) * c.gyro[i].numer / c.gyro[i].denom
		} else {
			out.GyroDPS[i] = rv / dualSenseGyroResPerDPS
		}
		av := float64(raw.AccelRaw[i])
		if c.valid {
			out.AccelG[i] = (av - c.accel[i].bias) * c.accel[i].numer / c.accel[i].denom
		} else {
			out.AccelG[i] = av / dualSenseAccelResPerG
		}
	}
	return out
}

type dualSenseQuaternion struct {
	w, x, y, z float64
}

type dualSenseOrientation struct {
	q                         dualSenseQuaternion
	RollDeg, PitchDeg, YawDeg float64
	rawYawDeg                 float64
	yawOffsetDeg              float64
	initialized               bool
	prevTimestamp             uint32

	// Application-level zero-rate compensation. Sony/Linux calibration scales
	// the gyro correctly but intentionally keeps gyro bias at zero. A real
	// DualSense can still sit a few tenths of a degree/s away from zero, which
	// is enough to drift absolute yaw over a long session. We estimate that
	// residual only while the controller is demonstrably stationary and apply
	// it to orientation fusion only; the physical GyroDPS axes remain untouched.
	gyroBiasDPS       [3]float64
	biasCandidateSum  [3]float64
	biasCandidateTime float64
	biasInitialized   bool
}

const (
	dualSenseMadgwickBeta             = 0.10
	dualSenseBiasStillGyroMaxDPS      = 0.80
	dualSenseBiasStillAccelToleranceG = 0.04
	dualSenseBiasAcquireSeconds       = 0.50
	dualSenseBiasAdaptTauSeconds      = 8.0
)

func wrapDegrees(v float64) float64 {
	for v > 180 {
		v -= 360
	}
	for v < -180 {
		v += 360
	}
	return v
}

func sensorDeltaSeconds(prev, cur uint32) float64 {
	if prev == 0 || prev == cur {
		return 0
	}
	var delta uint32
	if cur >= prev {
		delta = cur - prev
	} else {
		delta = ^prev + cur + 1
	}
	// Sony's driver documents DualSense sensor ticks as 0.33 us (~3 MHz).
	dt := float64(delta) / 3000000.0
	if dt <= 0 || dt > 0.1 {
		return 0
	}
	return dt
}

const dualSenseBeamNGMotionMinIntervalSeconds = 0.0075

func motionSensorDue(lastSentTimestamp, currentTimestamp uint32) bool {
	if lastSentTimestamp == 0 {
		return true
	}
	if currentTimestamp == lastSentTimestamp {
		return false
	}
	dt := sensorDeltaSeconds(lastSentTimestamp, currentTimestamp)
	// sensorDeltaSeconds rejects implausible gaps (>100 ms). For streaming we
	// must resync after such a gap rather than permanently waiting on the stale
	// timestamp.
	if dt <= 0 {
		return true
	}
	return dt >= dualSenseBeamNGMotionMinIntervalSeconds
}

func normalizeQuaternion(q dualSenseQuaternion) dualSenseQuaternion {
	n := math.Sqrt(q.w*q.w + q.x*q.x + q.y*q.y + q.z*q.z)
	if n <= 1e-12 {
		return dualSenseQuaternion{w: 1}
	}
	inv := 1 / n
	return dualSenseQuaternion{w: q.w * inv, x: q.x * inv, y: q.y * inv, z: q.z * inv}
}

// quaternionFromFilterTilt initializes the Z-up filter directly from gravity.
// DualSense uses X=right, Y=up, Z=toward player. The filter frame is
// X=right, Y=away from player, Z=up: filter(X,Y,Z)=DS(X,-Z,Y).
func quaternionFromFilterTilt(ax, ay, az float64) dualSenseQuaternion {
	rollX := math.Atan2(ay, az)
	pitchY := math.Atan2(-ax, math.Hypot(ay, az))
	hx, hy := rollX*0.5, pitchY*0.5
	cx, sx := math.Cos(hx), math.Sin(hx)
	cy, sy := math.Cos(hy), math.Sin(hy)
	return normalizeQuaternion(dualSenseQuaternion{
		w: cx * cy,
		x: sx * cy,
		y: cx * sy,
		z: -sx * sy,
	})
}

func madgwickIMUUpdate(q dualSenseQuaternion, gx, gy, gz, ax, ay, az, dt float64) dualSenseQuaternion {
	if dt <= 0 {
		return q
	}
	q = normalizeQuaternion(q)
	q0, q1, q2, q3 := q.w, q.x, q.y, q.z

	// Quaternion derivative from the gyroscope. Gyro values are rad/s.
	qDot0 := 0.5 * (-q1*gx - q2*gy - q3*gz)
	qDot1 := 0.5 * (q0*gx + q2*gz - q3*gy)
	qDot2 := 0.5 * (q0*gy - q1*gz + q3*gx)
	qDot3 := 0.5 * (q0*gz + q1*gy - q2*gx)

	// Use accelerometer correction only while its magnitude is plausibly gravity.
	// During a hard transient, gyro-only integration avoids interpreting linear
	// acceleration as a controller tilt.
	amag := math.Sqrt(ax*ax + ay*ay + az*az)
	if amag > 0.65 && amag < 1.35 {
		invA := 1 / amag
		ax, ay, az = ax*invA, ay*invA, az*invA

		_2q0, _2q1, _2q2, _2q3 := 2*q0, 2*q1, 2*q2, 2*q3
		_4q0, _4q1, _4q2 := 4*q0, 4*q1, 4*q2
		_8q1, _8q2 := 8*q1, 8*q2
		q0q0, q1q1, q2q2, q3q3 := q0*q0, q1*q1, q2*q2, q3*q3

		s0 := _4q0*q2q2 + _2q2*ax + _4q0*q1q1 - _2q1*ay
		s1 := _4q1*q3q3 - _2q3*ax + 4*q0q0*q1 - _2q0*ay - _4q1 + _8q1*q1q1 + _8q1*q2q2 + _4q1*az
		s2 := 4*q0q0*q2 + _2q0*ax + _4q2*q3q3 - _2q3*ay - _4q2 + _8q2*q1q1 + _8q2*q2q2 + _4q2*az
		s3 := 4*q1q1*q3 - _2q1*ax + 4*q2q2*q3 - _2q2*ay
		snorm2 := s0*s0 + s1*s1 + s2*s2 + s3*s3
		if snorm2 > 1e-18 {
			invS := 1 / math.Sqrt(snorm2)
			s0, s1, s2, s3 = s0*invS, s1*invS, s2*invS, s3*invS
			qDot0 -= dualSenseMadgwickBeta * s0
			qDot1 -= dualSenseMadgwickBeta * s1
			qDot2 -= dualSenseMadgwickBeta * s2
			qDot3 -= dualSenseMadgwickBeta * s3
		}
	}

	q0 += qDot0 * dt
	q1 += qDot1 * dt
	q2 += qDot2 * dt
	q3 += qDot3 * dt
	return normalizeQuaternion(dualSenseQuaternion{w: q0, x: q1, y: q2, z: q3})
}

func filterEuler(q dualSenseQuaternion) (xRot, yRot, zRot float64) {
	q = normalizeQuaternion(q)
	// X rotation.
	sinx := 2 * (q.w*q.x - q.y*q.z)
	if sinx >= 1 {
		xRot = math.Pi / 2
	} else if sinx <= -1 {
		xRot = -math.Pi / 2
	} else {
		xRot = math.Asin(sinx)
	}
	// Y rotation.
	siny := 2 * (q.w*q.y + q.z*q.x)
	cosy := 1 - 2*(q.x*q.x+q.y*q.y)
	yRot = math.Atan2(siny, cosy)
	// Z rotation.
	sinz := 2 * (q.w*q.z + q.x*q.y)
	cosz := 1 - 2*(q.y*q.y+q.z*q.z)
	zRot = math.Atan2(sinz, cosz)
	return
}

func (o *dualSenseOrientation) updateEuler() {
	xRot, yRot, zRot := filterEuler(o.q)
	// Map filter Z-up axes back to DualSense semantics.
	o.PitchDeg = xRot * 180 / math.Pi
	o.RollDeg = -yRot * 180 / math.Pi
	o.rawYawDeg = zRot * 180 / math.Pi
	o.YawDeg = wrapDegrees(o.rawYawDeg - o.yawOffsetDeg)
}

func (o *dualSenseOrientation) updateResidualGyroBias(s dualSenseMotionSample, dt float64) {
	if dt <= 0 {
		return
	}
	gm := math.Sqrt(s.GyroDPS[0]*s.GyroDPS[0] + s.GyroDPS[1]*s.GyroDPS[1] + s.GyroDPS[2]*s.GyroDPS[2])
	am := math.Sqrt(s.AccelG[0]*s.AccelG[0] + s.AccelG[1]*s.AccelG[1] + s.AccelG[2]*s.AccelG[2])
	still := gm <= dualSenseBiasStillGyroMaxDPS && math.Abs(am-1.0) <= dualSenseBiasStillAccelToleranceG
	if !still {
		o.biasCandidateTime = 0
		o.biasCandidateSum = [3]float64{}
		return
	}

	if !o.biasInitialized {
		o.biasCandidateTime += dt
		for i := 0; i < 3; i++ {
			o.biasCandidateSum[i] += s.GyroDPS[i] * dt
		}
		if o.biasCandidateTime >= dualSenseBiasAcquireSeconds {
			for i := 0; i < 3; i++ {
				o.gyroBiasDPS[i] = o.biasCandidateSum[i] / o.biasCandidateTime
			}
			o.biasInitialized = true
			o.biasCandidateTime = 0
			o.biasCandidateSum = [3]float64{}
		}
		return
	}

	// Very slow adaptation follows temperature drift without learning normal
	// deliberate controller motion as a new zero point.
	alpha := 1 - math.Exp(-dt/dualSenseBiasAdaptTauSeconds)
	for i := 0; i < 3; i++ {
		o.gyroBiasDPS[i] += alpha * (s.GyroDPS[i] - o.gyroBiasDPS[i])
	}
}

func (o *dualSenseOrientation) GyroBiasDPS() ([3]float64, bool) {
	return o.gyroBiasDPS, o.biasInitialized
}

func (o *dualSenseOrientation) Update(s dualSenseMotionSample) {
	// DualSense/SDL frame: +X right, +Y up, +Z toward player.
	// Madgwick Z-up frame: X=DS X, Y=-DS Z, Z=DS Y.
	fax, fay, faz := s.AccelG[0], -s.AccelG[2], s.AccelG[1]

	if !o.initialized {
		amag := math.Sqrt(fax*fax + fay*fay + faz*faz)
		if amag > 1e-9 {
			o.q = quaternionFromFilterTilt(fax, fay, faz)
		} else {
			o.q = dualSenseQuaternion{w: 1}
		}
		o.prevTimestamp = s.Timestamp
		o.initialized = true
		o.updateEuler()
		return
	}

	dt := sensorDeltaSeconds(o.prevTimestamp, s.Timestamp)
	o.prevTimestamp = s.Timestamp
	if dt <= 0 {
		return
	}
	o.updateResidualGyroBias(s, dt)

	corrected := s.GyroDPS
	if o.biasInitialized {
		for i := 0; i < 3; i++ {
			corrected[i] -= o.gyroBiasDPS[i]
		}
	}
	const degToRad = math.Pi / 180
	fgx := corrected[0] * degToRad
	fgy := -corrected[2] * degToRad
	fgz := corrected[1] * degToRad
	o.q = madgwickIMUUpdate(o.q, fgx, fgy, fgz, fax, fay, faz, dt)
	o.updateEuler()
}

func (o *dualSenseOrientation) CorrectedGyroDPS(s dualSenseMotionSample) [3]float64 {
	corrected := s.GyroDPS
	if o.biasInitialized {
		for i := 0; i < 3; i++ {
			corrected[i] -= o.gyroBiasDPS[i]
		}
	}
	return corrected
}

// GravityLocal returns the estimated gravity acceleration vector in the native
// DualSense/SDL local coordinate frame (+X right, +Y up, +Z toward player).
// The sign matches GamepadMotionHelpers: a controller resting naturally flat is
// approximately (0,-1,0). This makes rawAccel + gravity the linear acceleration
// with gravity removed.
func (o *dualSenseOrientation) GravityLocal() [3]float64 {
	q := normalizeQuaternion(o.q)
	// Madgwick's predicted accelerometer direction in the filter frame.
	fx := 2 * (q.x*q.z - q.w*q.y)
	fy := 2 * (q.w*q.x + q.y*q.z)
	fz := 1 - 2*(q.x*q.x+q.y*q.y)
	// filter(X,Y,Z) = DS(X,-Z,Y). The accelerometer at rest reports the
	// opposite of gravity, so negate the predicted proper-acceleration vector.
	return [3]float64{-fx, -fz, fy}
}

func (o *dualSenseOrientation) LinearAccelG(s dualSenseMotionSample) [3]float64 {
	g := o.GravityLocal()
	return [3]float64{s.AccelG[0] + g[0], s.AccelG[1] + g[1], s.AccelG[2] + g[2]}
}

// QuaternionQ15 returns the fused orientation quaternion in the Madgwick filter
// frame (X=DS X, Y=-DS Z, Z=DS Y). Sending the quaternion lets BeamNG derive
// relative swing/twist rotations without converting through Euler angles, so a
// deliberate rotation around one learned axis is not contaminated by simultaneous
// pitch/roll/yaw motion on the other axes.
func (o *dualSenseOrientation) QuaternionQ15() [4]int16 {
	q := normalizeQuaternion(o.q)
	return [4]int16{motionQ15(q.w), motionQ15(q.x), motionQ15(q.y), motionQ15(q.z)}
}

func (o *dualSenseOrientation) RecenterYaw() {
	o.yawOffsetDeg = o.rawYawDeg
	o.YawDeg = 0
}

func motionNormalize(v, limit float64) float64 {
	if limit <= 0 {
		return 0
	}
	v /= limit
	if v > 1 {
		return 1
	}
	if v < -1 {
		return -1
	}
	return v
}

func motionQ15(v float64) int16 {
	v = math.Max(-1, math.Min(1, v))
	return int16(math.Round(v * 32767))
}

func motionFromSample(s dualSenseMotionSample, o dualSenseOrientation) [9]int16 {
	return [9]int16{
		motionQ15(motionNormalize(s.GyroDPS[0], dualSenseGyroRangeDPS)),
		motionQ15(motionNormalize(s.GyroDPS[1], dualSenseGyroRangeDPS)),
		motionQ15(motionNormalize(s.GyroDPS[2], dualSenseGyroRangeDPS)),
		motionQ15(motionNormalize(s.AccelG[0], dualSenseAccelRangeG)),
		motionQ15(motionNormalize(s.AccelG[1], dualSenseAccelRangeG)),
		motionQ15(motionNormalize(s.AccelG[2], dualSenseAccelRangeG)),
		motionQ15(motionNormalize(o.RollDeg, 90)),
		motionQ15(motionNormalize(o.PitchDeg, 90)),
		motionQ15(motionNormalize(o.YawDeg, 180)),
	}
}

func correctedGyroWire(s dualSenseMotionSample, o dualSenseOrientation) [3]int16 {
	g := o.CorrectedGyroDPS(s)
	return [3]int16{
		motionQ15(motionNormalize(g[0], dualSenseGyroRangeDPS)),
		motionQ15(motionNormalize(g[1], dualSenseGyroRangeDPS)),
		motionQ15(motionNormalize(g[2], dualSenseGyroRangeDPS)),
	}
}

func gravityWire(o dualSenseOrientation) [3]int16 {
	g := o.GravityLocal()
	return [3]int16{motionQ15(g[0]), motionQ15(g[1]), motionQ15(g[2])}
}

func linearAccelWire(s dualSenseMotionSample, o dualSenseOrientation) [3]int16 {
	a := o.LinearAccelG(s)
	return [3]int16{
		motionQ15(motionNormalize(a[0], dualSenseAccelRangeG)),
		motionQ15(motionNormalize(a[1], dualSenseAccelRangeG)),
		motionQ15(motionNormalize(a[2], dualSenseAccelRangeG)),
	}
}
