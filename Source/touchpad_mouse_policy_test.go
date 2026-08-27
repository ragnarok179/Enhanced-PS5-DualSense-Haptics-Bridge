package main

import (
	"testing"
	"time"
)

func TestTouchMouseQuickTapJitterDoesNotArmMotion(t *testing.T) {
	arm, deliberate := touchMouseShouldArmMotion(70*time.Millisecond, 0.012)
	if arm || deliberate {
		t.Fatalf("quick tap jitter armed motion: arm=%t deliberate=%t", arm, deliberate)
	}
}

func TestTouchMouseQuickDeliberateMoveEscapesSettleWindow(t *testing.T) {
	arm, deliberate := touchMouseShouldArmMotion(40*time.Millisecond, 0.026)
	if !arm || !deliberate {
		t.Fatalf("deliberate early move not detected: arm=%t deliberate=%t", arm, deliberate)
	}
}

func TestTouchMouseSlowPrecisionMoveArmsAfterSettleWindow(t *testing.T) {
	arm, deliberate := touchMouseShouldArmMotion(140*time.Millisecond, 0.008)
	if !arm || deliberate {
		t.Fatalf("precision movement policy wrong: arm=%t deliberate=%t", arm, deliberate)
	}
}

func TestTouchMouseSmallMovedTapRemainsClickable(t *testing.T) {
	if !touchMouseTapEligible(0.018, 3.0, false) {
		t.Fatal("small natural tap movement should still click")
	}
}

func TestTouchMouseLargeTapTravelRejected(t *testing.T) {
	if touchMouseTapEligible(touchMouseTapMaxTravel+0.001, 0, false) {
		t.Fatal("tap over gesture travel threshold should not click")
	}
}

func TestTouchMouseDeliberateMoveRejectedAsTap(t *testing.T) {
	if touchMouseTapEligible(0.026, 2.0, true) {
		t.Fatal("deliberate pointer movement should not click")
	}
}

func TestTouchMouseTooMuchInjectedMotionRejectedAsTap(t *testing.T) {
	if touchMouseTapEligible(0.020, touchMouseTapMaxInjectedPixel+1, false) {
		t.Fatal("tap after substantial cursor motion should not click")
	}
}
