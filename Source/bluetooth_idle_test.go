package main

import (
	"testing"
	"time"
)

func neutralDualSenseInputReport(reportID byte) []byte {
	size, start := 64, 1
	if reportID == 0x31 {
		size, start = 78, 2
	}
	p := make([]byte, size)
	p[0] = reportID
	p[start], p[start+1], p[start+2], p[start+3] = 128, 128, 128, 128
	p[start+7] = 8 // neutral dpad
	p[start+32] = 0x80
	p[start+36] = 0x80
	return p
}

func TestBluetoothIdleDefault(t *testing.T) {
	if bluetoothIdleTimeoutDefaultMinutes != 10 {
		t.Fatalf("default=%d", bluetoothIdleTimeoutDefaultMinutes)
	}
}

func TestBluetoothIdleTimeoutClamp(t *testing.T) {
	if got := normalizeBluetoothIdleTimeoutMinutes(-1); got != 0 {
		t.Fatalf("negative=%d", got)
	}
	if got := normalizeBluetoothIdleTimeoutMinutes(17); got != 17 {
		t.Fatalf("normal=%d", got)
	}
	if got := normalizeBluetoothIdleTimeoutMinutes(999); got != 240 {
		t.Fatalf("high=%d", got)
	}
}

func TestDualSenseReportUserActivityUsesButtonsTriggersSticksTouch(t *testing.T) {
	p := neutralDualSenseInputReport(0x31)
	start := 2
	p[start+15] = 0xff // gyro noise must not count
	p[start+21] = 0xff // accel noise must not count
	if dualSenseReportHasUserActivity(p) {
		t.Fatal("sensor-only report counted as user activity")
	}

	p[start] = 200 // stick command must count
	if !dualSenseReportHasUserActivity(p) {
		t.Fatal("stick input was not detected")
	}
	p[start] = 128
	p[start+8] = 1
	if !dualSenseReportHasUserActivity(p) {
		t.Fatal("button press was not detected")
	}
	p[start+8] = 0
	p[start+4] = bluetoothIdleTriggerDeadzone + 1
	if !dualSenseReportHasUserActivity(p) {
		t.Fatal("trigger input was not detected")
	}
	p[start+4] = 0
	p[start+32] = 0x01
	if !dualSenseReportHasUserActivity(p) {
		t.Fatal("touch contact was not detected")
	}
}

func TestBluetoothIdleSettingChangeRestartsCountdown(t *testing.T) {
	oldActivity := time.Now().Add(-time.Hour)
	noteControllerInputActivityReason(oldActivity, bluetoothIdleActivityButton)
	before := bluetoothIdleGeneration()
	now := time.Now()
	setBluetoothIdleTimeoutMinutesAt(13, now)
	if bluetoothIdleTimeoutMinutes() != 13 {
		t.Fatal("setting not stored")
	}
	if bluetoothIdleGeneration() == before {
		t.Fatal("generation not advanced")
	}
	if bluetoothLastInputActivity().UnixNano() != now.UnixNano() {
		t.Fatal("setting change did not restart countdown")
	}
	if bluetoothIdleLastActivityReason() != "baseline" {
		t.Fatal("setting reset must stay diagnostic baseline")
	}
	setBluetoothIdleTimeoutMinutesAt(0, now)
}

func TestBluetoothIdleMotionConfig(t *testing.T) {
	now := time.Now()
	bluetoothIdleState.configSequenceSeen.Store(false)
	bluetoothIdleState.activitySequenceSeen.Store(false)
	applyBluetoothIdleConfig(10, true, 7, now)
	if !bluetoothIdleMotionResetEnabled() {
		t.Fatal("motion reset not enabled")
	}
	if !noteControllerMotionActivity(1, now.Add(time.Second)) {
		t.Fatal("motion activity rejected")
	}
	if bluetoothIdleLastActivityReason() != "motion" {
		t.Fatal("motion reason missing")
	}
	last := bluetoothLastInputActivity()
	if noteControllerMotionActivity(1, now.Add(2*time.Second)) {
		t.Fatal("duplicate sequence accepted")
	}
	if bluetoothLastInputActivity() != last {
		t.Fatal("duplicate changed timer")
	}
	if !noteControllerMotionActivity(2, now.Add(3*time.Second)) {
		t.Fatal("new sequence rejected")
	}
}

func TestBluetoothIdleRequiresFreshInputStream(t *testing.T) {
	now := time.Now()
	bluetoothIdleState.lastReportNS.Store(0)
	if bluetoothInputStreamFresh(now) {
		t.Fatal("missing input stream reported fresh")
	}
	noteControllerReportSeen(now)
	if !bluetoothInputStreamFresh(now) {
		t.Fatal("live input stream reported stale")
	}
	bluetoothIdleState.lastReportNS.Store(now.Add(-2 * time.Second).UnixNano())
	if bluetoothInputStreamFresh(now) {
		t.Fatal("stale input stream reported fresh")
	}
}

func TestBluetoothIdleTrackerIgnoresStaticTriggerAndSensorNoise(t *testing.T) {
	p := neutralDualSenseInputReport(0x31)
	start := 2
	var tracker bluetoothIdleInputTracker
	if got := tracker.activity(p); got != bluetoothIdleActivityNone {
		t.Fatalf("neutral initial activity=%d", got)
	}

	// A real trigger pull is activity once.
	p[start+4] = 80
	if got := tracker.activity(p); got != bluetoothIdleActivityTrigger {
		t.Fatalf("trigger pull activity=%d", got)
	}
	// Static trigger level plus arbitrary IMU changes must not renew idle.
	p[start+15] = 0xff
	p[start+21] = 0xee
	for i := 0; i < 20; i++ {
		if got := tracker.activity(p); got != bluetoothIdleActivityNone {
			t.Fatalf("static trigger/sensor report renewed idle: %d", got)
		}
	}

	// A stick outside the deadzone is an intentional controller command and
	// must keep the controller awake, even while the trigger remains static.
	p[start] = 240
	if got := tracker.activity(p); got != bluetoothIdleActivityStick {
		t.Fatalf("stick activity=%d", got)
	}
}

func TestBluetoothIdleStickDeadzoneRejectsNoise(t *testing.T) {
	p := neutralDualSenseInputReport(0x31)
	start := 2
	var tracker bluetoothIdleInputTracker
	_ = tracker.activity(p)
	p[start] = byte(128 + bluetoothIdleStickDeadzone)
	if got := tracker.activity(p); got != bluetoothIdleActivityNone {
		t.Fatalf("stick noise at deadzone counted: %d", got)
	}
	p[start]++
	if got := tracker.activity(p); got != bluetoothIdleActivityStick {
		t.Fatalf("stick beyond deadzone not counted: %d", got)
	}
}

func TestBluetoothIdleTrackerMasksDigitalTriggerBits(t *testing.T) {
	p := neutralDualSenseInputReport(0x31)
	start := 2
	var tracker bluetoothIdleInputTracker
	_ = tracker.activity(p)
	p[start+8] = 0x0c // L2/R2 digital threshold bits only
	if got := tracker.activity(p); got != bluetoothIdleActivityNone {
		t.Fatalf("digital trigger bits counted as button activity: %d", got)
	}
	p[start+8] = 0x10 // Create is a real button
	if got := tracker.activity(p); got != bluetoothIdleActivityButton {
		t.Fatalf("Create button activity=%d", got)
	}
}
