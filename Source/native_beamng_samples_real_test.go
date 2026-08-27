package main

import (
	"math"
	"os"
	"strings"
	"testing"
)

func TestNativeBeamNGRealCollisionBanksOptional(t *testing.T) {
	z := os.Getenv("NATIVE_BEAMNG_ART_SOUND")
	if z == "" {
		t.Skip("set NATIVE_BEAMNG_ART_SOUND for real-bank regression")
	}
	_, banks, err := extractNativeBeamNGBanks(z)
	if err != nil {
		t.Fatal(err)
	}
	var collision, shrapnel []nativeBeamNGSample
	for _, b := range banks {
		ss, scanErr := scanNativeFSB5Bank(b)
		if scanErr != nil {
			t.Logf("partial scan %s: %v", b, scanErr)
		}
		for _, s := range ss {
			low := strings.ToLower(s.Name)
			if s.Mode != fsbModeFADPCM || s.Channels != 1 {
				continue
			}
			switch {
			case strings.HasPrefix(low, "vehicle_impact_shrapnel_"):
				shrapnel = append(shrapnel, s)
			case strings.HasPrefix(low, "vehicle_impact_bump_"):
				// intentionally excluded
			case strings.HasPrefix(low, "vehicle_impact_"):
				collision = append(collision, s)
			}
		}
	}
	t.Logf("native collision pools collision=%d shrapnel=%d", len(collision), len(shrapnel))
	if len(collision) < 20 || len(shrapnel) < 5 {
		t.Fatal("unexpected collision pool counts")
	}
	pcm, err := decodeNativeBeamNGSample(collision[0])
	if err != nil {
		t.Fatalf("decode %s: %v", collision[0].Name, err)
	}
	if len(pcm) < 100 {
		t.Fatalf("short decode %s: %d", collision[0].Name, len(pcm))
	}
	var sum, peak float64
	for _, v := range pcm {
		sum += v * v
		if math.Abs(v) > peak {
			peak = math.Abs(v)
		}
	}
	rms := math.Sqrt(sum / float64(len(pcm)))
	if peak < .001 || rms < .0001 {
		t.Fatalf("silent decode peak=%.6f rms=%.6f", peak, rms)
	}
}
