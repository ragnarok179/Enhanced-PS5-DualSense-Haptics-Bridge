//go:build windows && (usb || bluetooth)

package main

import (
	"fmt"
	"math"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type motionDiagScalar struct {
	n           int64
	sum         float64
	sumSq       float64
	min         float64
	max         float64
	initialized bool
}

func (s *motionDiagScalar) Add(v float64) {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return
	}
	if !s.initialized {
		s.min, s.max, s.initialized = v, v, true
	} else {
		if v < s.min {
			s.min = v
		}
		if v > s.max {
			s.max = v
		}
	}
	s.n++
	s.sum += v
	s.sumSq += v * v
}

func (s motionDiagScalar) Mean() float64 {
	if s.n == 0 {
		return 0
	}
	return s.sum / float64(s.n)
}

func (s motionDiagScalar) StdDev() float64 {
	if s.n < 2 {
		return 0
	}
	m := s.Mean()
	v := s.sumSq/float64(s.n) - m*m
	if v < 0 {
		v = 0
	}
	return math.Sqrt(v)
}

func (s motionDiagScalar) Range() (float64, float64) {
	if !s.initialized {
		return 0, 0
	}
	return s.min, s.max
}

type motionDiagStats struct {
	start                time.Time
	lastHost             time.Time
	lastSensorTimestamp  uint32
	reports              uint64
	decoded              uint64
	decodeErrors         uint64
	duplicateSensorTS    uint64
	invalidSensorDelta   uint64
	hostGapsOver20ms     uint64
	sensorGapsOver20ms   uint64
	calibratedSamples    uint64
	fallbackSamples      uint64
	nonFiniteSamples     uint64
	rawGyroSaturation    [3]uint64
	rawAccelSaturation   [3]uint64
	hostDtMS             motionDiagScalar
	sensorDtMS           motionDiagScalar
	gyro                 [3]motionDiagScalar
	accel                [3]motionDiagScalar
	orientation          [3]motionDiagScalar
	gyroMagnitude        motionDiagScalar
	accelMagnitude       motionDiagScalar
	quietSamples         uint64
	quietGyro            [3]motionDiagScalar
	quietAccel           [3]motionDiagScalar
	quietOrientation     [3]motionDiagScalar
	quietAccelMagnitude  motionDiagScalar
	orientationGyroBias  [3]float64
	orientationBiasValid bool
	firstTransport       string
	firstReportID        byte
	firstReportSize      int
	firstSensorTimestamp uint32
}

func absI16(v int16) int {
	if v < 0 {
		return -int(v)
	}
	return int(v)
}

func (s *motionDiagStats) Add(now time.Time, decoded dualSenseExtendedInput, sample dualSenseMotionSample, orientation dualSenseOrientation) (hostDtMS, sensorDtMS float64) {
	s.decoded++
	if s.start.IsZero() {
		s.start = now
	}
	if s.firstReportSize == 0 {
		s.firstTransport = decoded.Transport
		s.firstReportID = decoded.ReportID
		s.firstReportSize = decoded.ReportSize
		s.firstSensorTimestamp = decoded.SensorTimestamp
	}

	if !s.lastHost.IsZero() {
		hostDtMS = now.Sub(s.lastHost).Seconds() * 1000
		s.hostDtMS.Add(hostDtMS)
		if hostDtMS > 20 {
			s.hostGapsOver20ms++
		}
	}
	s.lastHost = now

	if s.lastSensorTimestamp != 0 {
		if decoded.SensorTimestamp == s.lastSensorTimestamp {
			s.duplicateSensorTS++
		} else {
			dt := sensorDeltaSeconds(s.lastSensorTimestamp, decoded.SensorTimestamp)
			if dt <= 0 {
				s.invalidSensorDelta++
			} else {
				sensorDtMS = dt * 1000
				s.sensorDtMS.Add(sensorDtMS)
				if sensorDtMS > 20 {
					s.sensorGapsOver20ms++
				}
			}
		}
	}
	s.lastSensorTimestamp = decoded.SensorTimestamp

	if sample.Calibrated {
		s.calibratedSamples++
	} else {
		s.fallbackSamples++
	}

	finite := true
	for i := 0; i < 3; i++ {
		if math.IsNaN(sample.GyroDPS[i]) || math.IsInf(sample.GyroDPS[i], 0) || math.IsNaN(sample.AccelG[i]) || math.IsInf(sample.AccelG[i], 0) {
			finite = false
		}
		s.gyro[i].Add(sample.GyroDPS[i])
		s.accel[i].Add(sample.AccelG[i])
		if absI16(decoded.GyroRaw[i]) >= 32700 {
			s.rawGyroSaturation[i]++
		}
		if absI16(decoded.AccelRaw[i]) >= 32700 {
			s.rawAccelSaturation[i]++
		}
	}
	ori := [3]float64{orientation.RollDeg, orientation.PitchDeg, orientation.YawDeg}
	s.orientationGyroBias, s.orientationBiasValid = orientation.GyroBiasDPS()
	for i := 0; i < 3; i++ {
		if math.IsNaN(ori[i]) || math.IsInf(ori[i], 0) {
			finite = false
		}
		s.orientation[i].Add(ori[i])
	}
	if !finite {
		s.nonFiniteSamples++
	}

	gm := math.Sqrt(sample.GyroDPS[0]*sample.GyroDPS[0] + sample.GyroDPS[1]*sample.GyroDPS[1] + sample.GyroDPS[2]*sample.GyroDPS[2])
	am := math.Sqrt(sample.AccelG[0]*sample.AccelG[0] + sample.AccelG[1]*sample.AccelG[1] + sample.AccelG[2]*sample.AccelG[2])
	s.gyroMagnitude.Add(gm)
	s.accelMagnitude.Add(am)

	// Automatic quiet-window candidate. This is intentionally permissive: the
	// first 5 seconds with the controller flat should provide thousands of useful
	// samples, while normal deliberate motion is excluded.
	if gm <= 5.0 && am >= 0.90 && am <= 1.10 {
		s.quietSamples++
		for i := 0; i < 3; i++ {
			s.quietGyro[i].Add(sample.GyroDPS[i])
			s.quietAccel[i].Add(sample.AccelG[i])
			s.quietOrientation[i].Add(ori[i])
		}
		s.quietAccelMagnitude.Add(am)
	}
	return hostDtMS, sensorDtMS
}

func printMotionCalibration(cal dualSenseMotionCalibration, err error) {
	if err != nil {
		fmt.Printf("MOTION_CAL status=fallback reason=%q; nominal Sony scales will be used\n", err)
		fmt.Printf("MOTION_CAL nominal gyroRes=%.1f_raw_per_dps gyroRange=+/-%.0f_dps accelRes=%.1f_raw_per_g accelRange=+/-%.1f_g\n",
			dualSenseGyroResPerDPS, dualSenseGyroRangeDPS, dualSenseAccelResPerG, dualSenseAccelRangeG)
		return
	}
	fmt.Println("MOTION_CAL status=loaded feature=0x05 valid=true")
	for i, name := range []string{"Pitch/X", "Yaw/Y", "Roll/Z"} {
		fmt.Printf("MOTION_CAL_GYRO axis=%s bias=%+.6f numer=%.6f denom=%.6f scale=%.9f_dps_per_raw\n",
			name, cal.gyro[i].bias, cal.gyro[i].numer, cal.gyro[i].denom, cal.gyro[i].numer/cal.gyro[i].denom)
	}
	for i, name := range []string{"X", "Y", "Z"} {
		fmt.Printf("MOTION_CAL_ACCEL axis=%s bias=%+.6f numer=%.6f denom=%.6f scale=%.9f_g_per_raw\n",
			name, cal.accel[i].bias, cal.accel[i].numer, cal.accel[i].denom, cal.accel[i].numer/cal.accel[i].denom)
	}
}

func printMotionDiagSummary(s *motionDiagStats, calibrationOK bool, reason string) {
	if s == nil {
		return
	}
	elapsed := time.Since(s.start)
	if s.start.IsZero() {
		elapsed = 0
	}
	rate := 0.0
	if elapsed > 0 {
		rate = float64(s.decoded) / elapsed.Seconds()
	}
	hMin, hMax := s.hostDtMS.Range()
	tsMin, tsMax := s.sensorDtMS.Range()
	gmMin, gmMax := s.gyroMagnitude.Range()
	amMin, amMax := s.accelMagnitude.Range()
	fmt.Println()
	fmt.Println("================ MOTION_DIAG_SUMMARY ================")
	fmt.Printf("MOTION_SUMMARY reason=%s elapsed=%.3fs reports=%d decoded=%d decodeErrors=%d rate=%.3fHz calibration=%t calibratedSamples=%d fallbackSamples=%d\n",
		reason, elapsed.Seconds(), s.reports, s.decoded, s.decodeErrors, rate, calibrationOK, s.calibratedSamples, s.fallbackSamples)
	fmt.Printf("MOTION_TIMING hostDt_ms mean=%.4f std=%.4f min=%.4f max=%.4f gaps>20ms=%d | sensorDt_ms mean=%.4f std=%.4f min=%.4f max=%.4f gaps>20ms=%d duplicateTS=%d invalidDt=%d\n",
		s.hostDtMS.Mean(), s.hostDtMS.StdDev(), hMin, hMax, s.hostGapsOver20ms,
		s.sensorDtMS.Mean(), s.sensorDtMS.StdDev(), tsMin, tsMax, s.sensorGapsOver20ms, s.duplicateSensorTS, s.invalidSensorDelta)
	fmt.Printf("MOTION_SOURCE transport=%q reportID=0x%02X reportSize=%d firstSensorTS=%d lastSensorTS=%d nonFinite=%d\n",
		s.firstTransport, s.firstReportID, s.firstReportSize, s.firstSensorTimestamp, s.lastSensorTimestamp, s.nonFiniteSamples)
	for i, name := range []string{"Pitch", "Yaw", "Roll"} {
		mn, mx := s.gyro[i].Range()
		fmt.Printf("MOTION_GYRO axis=%s mean=%+.6f_dps std=%.6f min=%+.6f max=%+.6f rawSaturation=%d\n",
			name, s.gyro[i].Mean(), s.gyro[i].StdDev(), mn, mx, s.rawGyroSaturation[i])
	}
	for i, name := range []string{"X", "Y", "Z"} {
		mn, mx := s.accel[i].Range()
		fmt.Printf("MOTION_ACCEL axis=%s mean=%+.6f_g std=%.6f min=%+.6f max=%+.6f rawSaturation=%d\n",
			name, s.accel[i].Mean(), s.accel[i].StdDev(), mn, mx, s.rawAccelSaturation[i])
	}
	for i, name := range []string{"Roll", "Pitch", "Yaw"} {
		mn, mx := s.orientation[i].Range()
		fmt.Printf("MOTION_ORIENTATION axis=%s mean=%+.5f_deg std=%.5f min=%+.5f max=%+.5f\n",
			name, s.orientation[i].Mean(), s.orientation[i].StdDev(), mn, mx)
	}
	fmt.Printf("MOTION_MAG gyro_dps mean=%.5f std=%.5f min=%.5f max=%.5f | accel_g mean=%.6f std=%.6f min=%.6f max=%.6f\n",
		s.gyroMagnitude.Mean(), s.gyroMagnitude.StdDev(), gmMin, gmMax,
		s.accelMagnitude.Mean(), s.accelMagnitude.StdDev(), amMin, amMax)
	fmt.Printf("MOTION_ORIENTATION_GYRO_BIAS valid=%t Pitch=%+.7f_dps Yaw=%+.7f_dps Roll=%+.7f_dps\n",
		s.orientationBiasValid, s.orientationGyroBias[0], s.orientationGyroBias[1], s.orientationGyroBias[2])
	fmt.Printf("MOTION_QUIET candidates=%d (criteria gyroMag<=5dps and 0.90<=|accel|<=1.10g)\n", s.quietSamples)
	if s.quietSamples > 0 {
		for i, name := range []string{"Pitch", "Yaw", "Roll"} {
			mn, mx := s.quietGyro[i].Range()
			fmt.Printf("MOTION_QUIET_GYRO axis=%s mean=%+.7f_dps std=%.7f min=%+.7f max=%+.7f\n", name, s.quietGyro[i].Mean(), s.quietGyro[i].StdDev(), mn, mx)
		}
		for i, name := range []string{"X", "Y", "Z"} {
			mn, mx := s.quietAccel[i].Range()
			fmt.Printf("MOTION_QUIET_ACCEL axis=%s mean=%+.7f_g std=%.7f min=%+.7f max=%+.7f\n", name, s.quietAccel[i].Mean(), s.quietAccel[i].StdDev(), mn, mx)
		}
		for i, name := range []string{"Roll", "Pitch", "Yaw"} {
			mn, mx := s.quietOrientation[i].Range()
			fmt.Printf("MOTION_QUIET_ORIENTATION axis=%s mean=%+.6f_deg std=%.6f min=%+.6f max=%+.6f\n", name, s.quietOrientation[i].Mean(), s.quietOrientation[i].StdDev(), mn, mx)
		}
		mn, mx := s.quietAccelMagnitude.Range()
		fmt.Printf("MOTION_QUIET_GRAVITY mean=%.7f_g std=%.7f min=%.7f max=%.7f\n", s.quietAccelMagnitude.Mean(), s.quietAccelMagnitude.StdDev(), mn, mx)
	}
	fmt.Println("=====================================================")
}

func runMotionInputDiagnostic(d *device) int {
	if d == nil {
		return 2
	}
	fmt.Println("DualSense Motion Sensor diagnostic")
	featureLen := 0
	if caps, ok := getHIDCaps(d.handle); ok {
		featureLen = int(caps.FeatureReportByteLength)
	}
	fmt.Printf("MOTION_DEVICE product=%q VID=0x%04X PID=0x%04X inputLen=%d outputLen=%d featureLen=%d path=%q\n",
		d.product, sonyVID, d.productID, d.inputLen, d.outputLen, featureLen, d.path)
	calRaw, calRawErr := readDualSenseMotionCalibrationFeature(d.handle)
	if calRawErr == nil {
		fmt.Printf("MOTION_CAL_RAW size=%d bytes=% X\n", len(calRaw), calRaw)
	} else {
		fmt.Printf("MOTION_CAL_RAW unavailable err=%q\n", calRawErr)
	}
	calibration, calErr := dualSenseMotionCalibration{}, calRawErr
	if calRawErr == nil {
		calibration, calErr = parseDualSenseMotionCalibration(calRaw)
	}
	printMotionCalibration(calibration, calErr)
	const autoDiagnosticDuration = 80 * time.Second
	fmt.Println("MOTION_INSTRUCTIONS 1=keep controller flat/still for 5s 2=pitch +/- slow+fast 3=yaw +/- slow+fast 4=roll +/- slow+fast 5=still 5s")
	fmt.Println("MOTION_NOTE sample lines are throttled to ~20Hz; timing/statistics use every HID report.")
	fmt.Printf("MOTION_NOTE diagnostic auto-stops after %.0fs and always prints the final summary; Ctrl+C is optional when running the EXE directly.\n", autoDiagnosticDuration.Seconds())

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(stop)
	var orientation dualSenseOrientation
	var stats motionDiagStats
	stats.start = time.Now()
	deadline := stats.start.Add(autoDiagnosticDuration)
	lastPrint := time.Time{}
	reason := "signal"
	defer func() { printMotionDiagSummary(&stats, calErr == nil, reason) }()
	decodeErrorPrints := 0
	firstValidReportLogged := false

	for {
		select {
		case <-stop:
			reason = "Ctrl+C"
			return 0
		default:
		}
		if time.Now().After(deadline) {
			reason = "auto_80s"
			return 0
		}
		r, err := d.readReportOnce()
		if err != nil {
			fmt.Println("MOTION_READ_ERROR", err)
			reason = "read_error"
			return 3
		}
		stats.reports++
		decoded, err := decodeDualSenseExtendedInput(r)
		if err != nil {
			stats.decodeErrors++
			if decodeErrorPrints < 20 {
				decodeErrorPrints++
				prefix := r
				if len(prefix) > 32 {
					prefix = prefix[:32]
				}
				fmt.Printf("MOTION_DECODE_ERROR count=%d size=%d first32=% X err=%q\n", stats.decodeErrors, len(r), prefix, err)
			}
			continue
		}
		if !firstValidReportLogged {
			firstValidReportLogged = true
			fmt.Printf("MOTION_FIRST_REPORT size=%d bytes=% X\n", len(r), r)
		}
		sample := calibration.apply(decoded)
		orientation.Update(sample)
		now := time.Now()
		hostDtMS, sensorDtMS := stats.Add(now, decoded, sample, orientation)
		if now.Sub(lastPrint) < 50*time.Millisecond {
			continue
		}
		lastPrint = now
		gm := math.Sqrt(sample.GyroDPS[0]*sample.GyroDPS[0] + sample.GyroDPS[1]*sample.GyroDPS[1] + sample.GyroDPS[2]*sample.GyroDPS[2])
		am := math.Sqrt(sample.AccelG[0]*sample.AccelG[0] + sample.AccelG[1]*sample.AccelG[1] + sample.AccelG[2]*sample.AccelG[2])
		bias, biasOK := orientation.GyroBiasDPS()
		corrected := orientation.CorrectedGyroDPS(sample)
		gravity := orientation.GravityLocal()
		linear := orientation.LinearAccelG(sample)
		fmt.Printf("MOTION_SAMPLE n=%d t=%.3fs hostDt=%.3fms sensorTs=%d sensorDt=%.3fms cal=%t rawGyro=(%+6d,%+6d,%+6d) rawAccel=(%+6d,%+6d,%+6d) gyroDPS=(%+9.3f,%+9.3f,%+9.3f) corrected=(%+9.3f,%+9.3f,%+9.3f) gyroMag=%.3f accelG=(%+8.4f,%+8.4f,%+8.4f) gravity=(%+8.4f,%+8.4f,%+8.4f) linear=(%+8.4f,%+8.4f,%+8.4f) accelMag=%.5f orientDeg=(R%+8.3f,P%+8.3f,Y%+8.3f) bias=%t:(%+.4f,%+.4f,%+.4f) quat=(%.6f,%.6f,%.6f,%.6f)\n",
			stats.decoded, now.Sub(stats.start).Seconds(), hostDtMS, decoded.SensorTimestamp, sensorDtMS, sample.Calibrated,
			decoded.GyroRaw[0], decoded.GyroRaw[1], decoded.GyroRaw[2], decoded.AccelRaw[0], decoded.AccelRaw[1], decoded.AccelRaw[2],
			sample.GyroDPS[0], sample.GyroDPS[1], sample.GyroDPS[2],
			corrected[0], corrected[1], corrected[2], gm,
			sample.AccelG[0], sample.AccelG[1], sample.AccelG[2],
			gravity[0], gravity[1], gravity[2], linear[0], linear[1], linear[2], am,
			orientation.RollDeg, orientation.PitchDeg, orientation.YawDeg,
			biasOK, bias[0], bias[1], bias[2],
			orientation.q.w, orientation.q.x, orientation.q.y, orientation.q.z)
	}
}
