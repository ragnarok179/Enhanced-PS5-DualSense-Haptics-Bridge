package main

import (
	"math"
	"time"
)

type lightbarController struct {
	stage         int // 0=off, 1=progressive, 2=solid red
	lastColor     [3]byte
	enabled       bool
	rpmActive     bool
	belowOffSince time.Time
	limiterUntil  time.Time
	blinkActive   bool
}

func quantizeLED(v float64) byte {
	v = clamp(v, 0, 255)
	q := int(math.Round(v/8.0)) * 8
	if q > 255 {
		q = 255
	}
	return byte(q)
}

func steadyRPMRGB(ratio float64) [3]byte {
	cfg := feelProfile().LED
	if ratio >= cfg.RedRatio {
		return [3]byte{byte(clampInt(cfg.MaxBrightness, 0, 255)), 0, 0}
	}
	x := clamp01((ratio - cfg.FirstRatio) / math.Max(0.001, cfg.RedRatio-cfg.FirstRatio))
	hue := 120 * (1 - x)
	brightness := float64(cfg.MinBrightness) + float64(cfg.MaxBrightness-cfg.MinBrightness)*clamp01(x*1.8)
	r, g, b := hsvToRGB(hue, 1.0, brightness/255.0)
	return [3]byte{quantizeLED(float64(r)), quantizeLED(float64(g)), quantizeLED(float64(b))}
}

func (c *lightbarController) update(t telemetry, now time.Time) [3]byte {
	var black [3]byte
	engineActive := t.Active && t.Raw != nil && t.Raw.EngineRunning && t.Raw.MaxRPM > 1
	if !engineActive {
		c.stage = 0
		c.rpmActive = false
		c.belowOffSince = time.Time{}
		c.enabled = false
		c.limiterUntil = time.Time{}
		c.blinkActive = false
		c.lastColor = black
		return black
	}

	if t.ShiftLEDsInUse && !c.enabled {
		c.enabled = true
	}
	if !c.enabled {
		c.blinkActive = false
		c.lastColor = black
		return black
	}

	cfg := feelProfile().LED
	ratio := clamp01(t.Raw.RPM / math.Max(t.Raw.MaxRPM, 1))
	if !c.rpmActive {
		if ratio >= cfg.FirstRatio {
			c.rpmActive = true
			c.stage = 1
			c.belowOffSince = time.Time{}
		} else {
			c.blinkActive = false
			c.lastColor = black
			return black
		}
	} else if ratio < cfg.OffRatio {
		if c.belowOffSince.IsZero() {
			c.belowOffSince = now
		}
		if now.Sub(c.belowOffSince) >= time.Duration(cfg.OffHoldMS)*time.Millisecond {
			c.rpmActive = false
			c.stage = 0
			c.blinkActive = false
			c.lastColor = black
			return black
		}
	} else {
		c.belowOffSince = time.Time{}
	}

	if c.stage == 2 {
		if ratio < cfg.RedExitRatio {
			c.stage = 1
		}
	} else if ratio >= cfg.RedRatio {
		c.stage = 2
	}

	if t.Raw.RevLimiter && ratio >= cfg.BlinkMinRatio {
		c.limiterUntil = now.Add(time.Duration(cfg.BlinkHoldMS) * time.Millisecond)
	}
	blink := !c.limiterUntil.IsZero() && now.Before(c.limiterUntil) && ratio >= cfg.RedExitRatio
	if !cfg.BlinkOnlyOnRevLimiter && ratio >= 0.985 {
		blink = true
	}
	if blink {
		c.blinkActive = true
		hz := math.Max(1, cfg.BlinkHz)
		on := (int(math.Floor(float64(now.UnixNano())/1e9*hz*2.0)) % 2) == 0
		if !on {
			c.lastColor = black
			return black
		}
		red := [3]byte{byte(clampInt(cfg.MaxBrightness, 0, 255)), 0, 0}
		c.lastColor = red
		return red
	}
	c.blinkActive = false

	if ratio < cfg.FirstRatio && c.lastColor != black {
		return c.lastColor
	}
	rgb := steadyRPMRGB(math.Max(ratio, cfg.FirstRatio))
	c.lastColor = rgb
	return rgb
}

func (c *lightbarController) status() string {
	if c.blinkActive {
		return "blink"
	}
	if !c.rpmActive {
		return "off"
	}
	if c.stage >= 2 {
		return "red"
	}
	return "progress"
}

func hsvToRGB(h, s, v float64) (byte, byte, byte) {
	h = math.Mod(h, 360)
	if h < 0 {
		h += 360
	}
	c := v * s
	x := c * (1 - math.Abs(math.Mod(h/60, 2)-1))
	m := v - c
	r1, g1, b1 := 0.0, 0.0, 0.0
	switch {
	case h < 60:
		r1, g1 = c, x
	case h < 120:
		r1, g1 = x, c
	case h < 180:
		g1, b1 = c, x
	case h < 240:
		g1, b1 = x, c
	case h < 300:
		r1, b1 = x, c
	default:
		r1, b1 = c, x
	}
	roundByte := func(x float64) byte { return byte(clampInt(int(math.Floor(x*255+0.5)), 0, 255)) }
	return roundByte(r1 + m), roundByte(g1 + m), roundByte(b1 + m)
}
