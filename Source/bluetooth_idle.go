package main

import (
	"math"
	"sync/atomic"
	"time"
)

const (
	bluetoothIdleTimeoutDefaultMinutes = 10
	bluetoothIdleTimeoutMaxMinutes     = 240
	// Trigger input is treated separately from digital buttons so an adaptive-
	// trigger effect cannot keep the controller awake through the L2/R2 digital
	// bits. A real user pull must cross this input deadzone and then move by a
	// meaningful amount to refresh the timer.
	bluetoothIdleTriggerDeadzone = 16
	bluetoothIdleTriggerDelta    = 4
	bluetoothIdleStickDeadzone   = 18
)

const (
	bluetoothIdleActivityNone int32 = iota
	bluetoothIdleActivityButton
	bluetoothIdleActivityTrigger
	bluetoothIdleActivityStick
	bluetoothIdleActivityTouchpad
	bluetoothIdleActivityMotion
)

var bluetoothIdleState struct {
	timeoutMinutes       atomic.Int32
	lastActivityNS       atomic.Int64
	lastReportNS         atomic.Int64
	lastReason           atomic.Int32
	generation           atomic.Uint32
	motionResetEnabled   atomic.Bool
	configResetSequence  atomic.Uint32
	configSequenceSeen   atomic.Bool
	activitySequence     atomic.Uint32
	activitySequenceSeen atomic.Bool
}

func init() {
	bluetoothIdleState.timeoutMinutes.Store(bluetoothIdleTimeoutDefaultMinutes)
	seedBluetoothIdleBaseline(time.Now())
}

func normalizeBluetoothIdleTimeoutMinutes(minutes int) int {
	if minutes < 0 {
		return 0
	}
	if minutes > bluetoothIdleTimeoutMaxMinutes {
		return bluetoothIdleTimeoutMaxMinutes
	}
	return minutes
}

func setBluetoothIdleTimeoutMinutesAt(minutes int, now time.Time) int {
	minutes = normalizeBluetoothIdleTimeoutMinutes(minutes)
	old := bluetoothIdleState.timeoutMinutes.Swap(int32(minutes))
	if int(old) != minutes {
		seedBluetoothIdleBaseline(now)
		bluetoothIdleState.generation.Add(1)
	}
	return minutes
}
func setBluetoothIdleTimeoutMinutes(minutes int) int {
	return setBluetoothIdleTimeoutMinutesAt(minutes, time.Now())
}
func bluetoothIdleMotionResetEnabled() bool { return bluetoothIdleState.motionResetEnabled.Load() }
func applyBluetoothIdleConfig(minutes int, motionReset bool, resetSequence uint16, now time.Time) {
	minutes = normalizeBluetoothIdleTimeoutMinutes(minutes)
	oldMinutes := int(bluetoothIdleState.timeoutMinutes.Swap(int32(minutes)))
	oldMotion := bluetoothIdleState.motionResetEnabled.Swap(motionReset)
	resetChanged := false
	if !bluetoothIdleState.configSequenceSeen.Swap(true) {
		bluetoothIdleState.configResetSequence.Store(uint32(resetSequence))
	} else {
		old := uint16(bluetoothIdleState.configResetSequence.Swap(uint32(resetSequence)))
		resetChanged = old != resetSequence
	}
	if oldMinutes != minutes || oldMotion != motionReset || resetChanged {
		seedBluetoothIdleBaseline(now)
		bluetoothIdleState.activitySequenceSeen.Store(false)
		bluetoothIdleState.generation.Add(1)
	}
}
func sequence16IsNewer(current, previous uint16) bool {
	d := uint16(current - previous)
	return d != 0 && d < 0x8000
}
func noteControllerMotionActivity(sequence uint16, now time.Time) bool {
	if !bluetoothIdleMotionResetEnabled() {
		return false
	}
	if !bluetoothIdleState.activitySequenceSeen.Swap(true) {
		bluetoothIdleState.activitySequence.Store(uint32(sequence))
	} else {
		prev := uint16(bluetoothIdleState.activitySequence.Load())
		if !sequence16IsNewer(sequence, prev) {
			return false
		}
		bluetoothIdleState.activitySequence.Store(uint32(sequence))
	}
	noteControllerInputActivityReason(now, bluetoothIdleActivityMotion)
	return true
}

func bluetoothIdleTimeoutMinutes() int { return int(bluetoothIdleState.timeoutMinutes.Load()) }
func bluetoothIdleGeneration() uint32  { return bluetoothIdleState.generation.Load() }

func seedBluetoothIdleBaseline(now time.Time) {
	bluetoothIdleState.lastActivityNS.Store(now.UnixNano())
	bluetoothIdleState.lastReason.Store(bluetoothIdleActivityNone)
}

func noteControllerInputActivity(now time.Time) {
	bluetoothIdleState.lastActivityNS.Store(now.UnixNano())
}

func noteControllerInputActivityReason(now time.Time, reason int32) {
	noteControllerInputActivity(now)
	bluetoothIdleState.lastReason.Store(reason)
}

func noteControllerReportSeen(now time.Time) { bluetoothIdleState.lastReportNS.Store(now.UnixNano()) }

func bluetoothInputStreamFresh(now time.Time) bool {
	ns := bluetoothIdleState.lastReportNS.Load()
	if ns == 0 {
		return false
	}
	age := now.Sub(time.Unix(0, ns))
	return age >= 0 && age <= time.Second
}

func bluetoothLastInputActivity() time.Time {
	ns := bluetoothIdleState.lastActivityNS.Load()
	if ns == 0 {
		return time.Time{}
	}
	return time.Unix(0, ns)
}

func bluetoothIdleLastActivityReason() string {
	switch bluetoothIdleState.lastReason.Load() {
	case bluetoothIdleActivityButton:
		return "button"
	case bluetoothIdleActivityTrigger:
		return "trigger"
	case bluetoothIdleActivityStick:
		return "stick"
	case bluetoothIdleActivityTouchpad:
		return "touchpad"
	case bluetoothIdleActivityMotion:
		return "motion"
	default:
		return "baseline"
	}
}

func dualSenseCommonInputStart(report []byte) (int, bool) {
	if len(report) >= 64 && report[0] == 0x01 {
		return 1, true
	}
	if len(report) >= 78 && report[0] == 0x31 {
		return 2, true
	}
	return 0, false
}

type bluetoothIdleInputSnapshot struct {
	lx, ly, rx, ry byte
	l2, r2         byte
	digitalDown    bool
	touchDown      bool
}

func dualSenseIdleInputSnapshot(report []byte) (bluetoothIdleInputSnapshot, bool) {
	start, ok := dualSenseCommonInputStart(report)
	if !ok || len(report) < start+40 {
		return bluetoothIdleInputSnapshot{}, false
	}

	buttons0 := report[start+7]
	dpad := buttons0 & 0x0f
	// buttons[1] bits 2/3 are the digital L2/R2 thresholds. Mask them out so
	// adaptive-trigger mechanics can never refresh idle through this path; the
	// analog trigger positions below use their own movement detector.
	buttons1NoTriggers := report[start+8] &^ byte(0x0c)
	buttons2 := report[start+9]

	return bluetoothIdleInputSnapshot{
		lx:          report[start+0],
		ly:          report[start+1],
		rx:          report[start+2],
		ry:          report[start+3],
		l2:          report[start+4],
		r2:          report[start+5],
		digitalDown: buttons0&0xf0 != 0 || dpad <= 7 || buttons1NoTriggers != 0 || buttons2 != 0,
		touchDown:   report[start+32]&0x80 == 0 || report[start+36]&0x80 == 0,
	}, true
}

// bluetoothIdleInputTracker is intentionally stateful. Buttons, stick movement
// outside a hardware-noise deadzone, and touchpad contact are physical user
// inputs and keep the controller awake. Trigger activity requires meaningful
// input-position movement, so a static pull or output-side adaptive-trigger
// effect cannot renew the timer forever. IMU/audio/LED/haptics are never read.
type bluetoothIdleInputTracker struct {
	initialized bool
	l2, r2      byte
}

func stickOutsideIdleDeadzone(v byte) bool {
	return math.Abs(float64(int(v)-128)) > bluetoothIdleStickDeadzone
}

func stickActive(s bluetoothIdleInputSnapshot) bool {
	return stickOutsideIdleDeadzone(s.lx) || stickOutsideIdleDeadzone(s.ly) ||
		stickOutsideIdleDeadzone(s.rx) || stickOutsideIdleDeadzone(s.ry)
}

func triggerMovedByUser(previous, current byte) bool {
	prev, cur := int(previous), int(current)
	if prev <= bluetoothIdleTriggerDeadzone && cur <= bluetoothIdleTriggerDeadzone {
		return false
	}
	if (prev <= bluetoothIdleTriggerDeadzone) != (cur <= bluetoothIdleTriggerDeadzone) {
		return true
	}
	return math.Abs(float64(cur-prev)) >= bluetoothIdleTriggerDelta
}

func (t *bluetoothIdleInputTracker) activity(report []byte) int32 {
	s, ok := dualSenseIdleInputSnapshot(report)
	if !ok {
		return bluetoothIdleActivityNone
	}

	triggerActivity := false
	if !t.initialized {
		// If the Bridge attaches while the user is already holding a trigger,
		// treat that as the initial physical input once, not on every HID report.
		triggerActivity = int(s.l2) > bluetoothIdleTriggerDeadzone || int(s.r2) > bluetoothIdleTriggerDeadzone
		t.initialized = true
	} else {
		triggerActivity = triggerMovedByUser(t.l2, s.l2) || triggerMovedByUser(t.r2, s.r2)
	}
	t.l2, t.r2 = s.l2, s.r2

	if s.touchDown {
		return bluetoothIdleActivityTouchpad
	}
	if s.digitalDown {
		return bluetoothIdleActivityButton
	}
	if stickActive(s) {
		return bluetoothIdleActivityStick
	}
	if triggerActivity {
		return bluetoothIdleActivityTrigger
	}
	return bluetoothIdleActivityNone
}

// Compatibility helper used by diagnostics/tests. Production idle tracking uses
// bluetoothIdleInputTracker.activity() above to distinguish real trigger motion
// from static/output-induced state.
func dualSenseReportHasUserActivity(report []byte) bool {
	s, ok := dualSenseIdleInputSnapshot(report)
	if !ok {
		return false
	}
	return s.digitalDown || s.touchDown || stickActive(s) ||
		int(s.l2) > bluetoothIdleTriggerDeadzone || int(s.r2) > bluetoothIdleTriggerDeadzone
}
