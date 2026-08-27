package main

import (
	"math"
	"testing"
	"time"
)

func tp(active bool, id byte, x, y int) dualSenseTouchPoint {
	return dualSenseTouchPoint{Active: active, ID: id, X: x, Y: y}
}

func closeEnough(a, b, eps float64) bool { return math.Abs(a-b) <= eps }

func TestTouchAbsoluteAndOneFingerMotion(t *testing.T) {
	var tr touchGestureTracker
	now := time.Unix(100, 0)
	f := tr.Update([2]dualSenseTouchPoint{tp(true, 3, 960, 540)}, now)
	if f.Count != 1 || !closeEnough(f.Abs1X, 960.0/1919.0, 1e-6) || f.OneDX != 0 || f.OneDY != 0 {
		t.Fatalf("initial frame=%+v", f)
	}
	f = tr.Update([2]dualSenseTouchPoint{tp(true, 3, 760, 640)}, now.Add(8*time.Millisecond))
	if f.OneDX >= 0 || f.OneDY <= 0 {
		t.Fatalf("expected left/down signed motion, got %+v", f)
	}
}

func TestTwoFingerPanUsesCentroid(t *testing.T) {
	var tr touchGestureTracker
	now := time.Unix(100, 0)
	tr.Update([2]dualSenseTouchPoint{tp(true, 1, 400, 300), tp(true, 9, 1400, 700)}, now)
	f := tr.Update([2]dualSenseTouchPoint{tp(true, 1, 300, 300), tp(true, 9, 1300, 700)}, now.Add(8*time.Millisecond))
	if f.TwoDX >= 0 || math.Abs(f.TwoDY) > 0.02 {
		t.Fatalf("expected two-finger left pan, got %+v", f)
	}
}

func TestSlotSwapKeepsFingerIdentity(t *testing.T) {
	var tr touchGestureTracker
	now := time.Unix(100, 0)
	a := [2]dualSenseTouchPoint{tp(true, 7, 300, 300), tp(true, 22, 1500, 700)}
	tr.Update(a, now)
	// Same contacts, reversed report slots and moved together to the right.
	b := [2]dualSenseTouchPoint{tp(true, 22, 1550, 700), tp(true, 7, 350, 300)}
	f := tr.Update(b, now.Add(8*time.Millisecond))
	if f.PrimaryID != 7 || f.SecondaryID != 22 || f.TwoDX <= 0 {
		t.Fatalf("slot swap broke identity: %+v", f)
	}
}

func TestOneAndTwoFingerTap(t *testing.T) {
	now := time.Unix(100, 0)
	var one touchGestureTracker
	one.Update([2]dualSenseTouchPoint{tp(true, 1, 800, 400)}, now)
	f := one.Update([2]dualSenseTouchPoint{}, now.Add(120*time.Millisecond))
	if !f.OneTap || f.TwoTap {
		t.Fatalf("one tap=%+v", f)
	}

	var two touchGestureTracker
	two.Update([2]dualSenseTouchPoint{tp(true, 1, 700, 400), tp(true, 2, 1200, 500)}, now)
	f = two.Update([2]dualSenseTouchPoint{}, now.Add(120*time.Millisecond))
	if !f.TwoTap || f.OneTap {
		t.Fatalf("two tap=%+v", f)
	}
}

func TestMovementCancelsTap(t *testing.T) {
	var tr touchGestureTracker
	now := time.Unix(100, 0)
	tr.Update([2]dualSenseTouchPoint{tp(true, 1, 300, 300)}, now)
	tr.Update([2]dualSenseTouchPoint{tp(true, 1, 900, 300)}, now.Add(80*time.Millisecond))
	f := tr.Update([2]dualSenseTouchPoint{}, now.Add(160*time.Millisecond))
	if f.OneTap || f.TwoTap {
		t.Fatalf("movement should cancel tap: %+v", f)
	}
}

func TestPinchSign(t *testing.T) {
	var tr touchGestureTracker
	now := time.Unix(100, 0)
	tr.Update([2]dualSenseTouchPoint{tp(true, 1, 700, 500), tp(true, 2, 1200, 500)}, now)
	f := tr.Update([2]dualSenseTouchPoint{tp(true, 1, 600, 500), tp(true, 2, 1300, 500)}, now.Add(8*time.Millisecond))
	if f.Pinch <= 0 {
		t.Fatalf("spread should be positive pinch, got %+v", f)
	}
}
