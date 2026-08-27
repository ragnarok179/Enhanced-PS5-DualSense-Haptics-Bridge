package main

import (
	"math"
	"testing"
)

func TestAccumulateWholePixelsPreservesSlowPositiveMotion(t *testing.T) {
	var accum float64
	if got := accumulateWholePixels(&accum, 0.4); got != 0 {
		t.Fatalf("first sample = %d, want 0", got)
	}
	if got := accumulateWholePixels(&accum, 0.4); got != 0 {
		t.Fatalf("second sample = %d, want 0", got)
	}
	if got := accumulateWholePixels(&accum, 0.4); got != 1 {
		t.Fatalf("third sample = %d, want 1", got)
	}
	if math.Abs(accum-0.2) > 1e-9 {
		t.Fatalf("remainder = %.12f, want 0.2", accum)
	}
}

func TestAccumulateWholePixelsPreservesSlowNegativeMotion(t *testing.T) {
	var accum float64
	for i := 0; i < 2; i++ {
		if got := accumulateWholePixels(&accum, -0.4); got != 0 {
			t.Fatalf("sample %d = %d, want 0", i+1, got)
		}
	}
	if got := accumulateWholePixels(&accum, -0.4); got != -1 {
		t.Fatalf("third sample = %d, want -1", got)
	}
	if math.Abs(accum+0.2) > 1e-9 {
		t.Fatalf("remainder = %.12f, want -0.2", accum)
	}
}
