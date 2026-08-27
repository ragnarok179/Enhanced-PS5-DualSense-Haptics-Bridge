package main

import (
	"math"
	"time"
)

// Touchpad processing follows the same model used by modern multitouch APIs:
// keep stable contact identities and absolute 0..1 coordinates, then derive
// signed motion deltas from consecutive samples. The raw touch surface remains
// lossless; gameplay-friendly motion axes are derived separately.
const (
	touchMotionGain       = 16.0
	touchMotionNoiseFloor = 0.0015
	touchTapMinDuration   = 20 * time.Millisecond
	touchTapMaxDuration   = 450 * time.Millisecond
	touchTapMaxTravel     = 0.035 // fraction of full pad diagonal
	touchTapPulse         = 120 * time.Millisecond
)

type logicalTouchFrame struct {
	Count uint8

	PrimaryActive bool
	PrimaryID     byte
	PrimaryX      int
	PrimaryY      int

	SecondaryActive bool
	SecondaryID     byte
	SecondaryX      int
	SecondaryY      int

	// Raw normalized absolute positions (0..1, origin top-left).
	Abs1X float64
	Abs1Y float64
	Abs2X float64
	Abs2Y float64

	// Derived signed motion (-1..1). These are gameplay-friendly motion
	// signals; 0 means no movement.
	OneDX float64
	OneDY float64
	TwoDX float64
	TwoDY float64
	Pinch float64

	// Raw normalized deltas before gameplay gain. Used only by the Windows
	// mouse fallback and packet accumulation so sub-frame motion is not lost.
	RawOneDX float64
	RawOneDY float64
	RawTwoDX float64
	RawTwoDY float64

	OneTap bool
	TwoTap bool
}

type trackedContact struct {
	valid bool
	id    byte
	x     int
	y     int
}

type touchSession struct {
	active      bool
	startedAt   time.Time
	maxContacts int
	moved       bool
	starts      map[byte][2]int
}

type touchGestureTracker struct {
	primary   trackedContact
	secondary trackedContact

	session touchSession

	oneTapUntil time.Time
	twoTapUntil time.Time

	previousCount uint8
	prevPrimary   trackedContact
	prevSecondary trackedContact
	prevCentroidX float64
	prevCentroidY float64
	prevDistance  float64
}

func activeTouchPoints(points [2]dualSenseTouchPoint) []dualSenseTouchPoint {
	out := make([]dualSenseTouchPoint, 0, 2)
	for _, p := range points {
		if p.Active {
			out = append(out, p)
		}
	}
	return out
}

func touchPointByID(points []dualSenseTouchPoint, id byte) (dualSenseTouchPoint, bool) {
	for _, p := range points {
		if p.ID == id {
			return p, true
		}
	}
	return dualSenseTouchPoint{}, false
}

func unusedTouchPoint(points []dualSenseTouchPoint, usedID byte, hasUsed bool) (dualSenseTouchPoint, bool) {
	for _, p := range points {
		if !hasUsed || p.ID != usedID {
			return p, true
		}
	}
	return dualSenseTouchPoint{}, false
}

func (t *touchGestureTracker) assignContacts(points []dualSenseTouchPoint) (dualSenseTouchPoint, bool, dualSenseTouchPoint, bool) {
	var p1, p2 dualSenseTouchPoint
	p1ok, p2ok := false, false

	if t.primary.valid {
		if p, ok := touchPointByID(points, t.primary.id); ok {
			p1, p1ok = p, true
		}
	}
	if t.secondary.valid {
		if p, ok := touchPointByID(points, t.secondary.id); ok {
			p2, p2ok = p, true
		}
	}

	// Promote the still-present second contact when the first one is lifted.
	if !p1ok && p2ok {
		p1, p1ok = p2, true
		p2ok = false
		t.primary = trackedContact{valid: true, id: p1.ID, x: p1.X, y: p1.Y}
		t.secondary = trackedContact{}
	}

	if !p1ok {
		if p, ok := unusedTouchPoint(points, 0, false); ok {
			p1, p1ok = p, true
			t.primary = trackedContact{valid: true, id: p.ID, x: p.X, y: p.Y}
		}
	}
	if !p2ok {
		if p, ok := unusedTouchPoint(points, p1.ID, p1ok); ok {
			p2, p2ok = p, true
			t.secondary = trackedContact{valid: true, id: p.ID, x: p.X, y: p.Y}
		}
	}

	if !p1ok {
		t.primary = trackedContact{}
	}
	if !p2ok {
		t.secondary = trackedContact{}
	}
	return p1, p1ok, p2, p2ok
}

func normalizedDistance(aX, aY, bX, bY int) float64 {
	dx := float64(aX-bX) / float64(dualSenseTouchpadWidth-1)
	dy := float64(aY-bY) / float64(dualSenseTouchpadHeight-1)
	return math.Hypot(dx, dy)
}

func motionAxis(delta float64) float64 {
	if math.Abs(delta) < touchMotionNoiseFloor {
		return 0
	}
	v := delta * touchMotionGain
	if v > 1 {
		return 1
	}
	if v < -1 {
		return -1
	}
	return v
}

func (t *touchGestureTracker) updateTapSession(now time.Time, pts []dualSenseTouchPoint) {
	if len(pts) > 0 && !t.session.active {
		t.session = touchSession{
			active:      true,
			startedAt:   now,
			maxContacts: len(pts),
			starts:      make(map[byte][2]int, 2),
		}
		for _, p := range pts {
			t.session.starts[p.ID] = [2]int{p.X, p.Y}
		}
	}
	if !t.session.active {
		return
	}
	if len(pts) > t.session.maxContacts {
		t.session.maxContacts = len(pts)
	}
	for _, p := range pts {
		start, ok := t.session.starts[p.ID]
		if !ok {
			t.session.starts[p.ID] = [2]int{p.X, p.Y}
			continue
		}
		if normalizedDistance(p.X, p.Y, start[0], start[1]) > touchTapMaxTravel {
			t.session.moved = true
		}
	}
	if len(pts) != 0 {
		return
	}

	duration := now.Sub(t.session.startedAt)
	if !t.session.moved && duration >= touchTapMinDuration && duration <= touchTapMaxDuration {
		if t.session.maxContacts >= 2 {
			t.twoTapUntil = now.Add(touchTapPulse)
		} else if t.session.maxContacts == 1 {
			t.oneTapUntil = now.Add(touchTapPulse)
		}
	}
	t.session = touchSession{}
}

func (t *touchGestureTracker) Update(points [2]dualSenseTouchPoint, now time.Time) logicalTouchFrame {
	pts := activeTouchPoints(points)
	t.updateTapSession(now, pts)
	p1, p1ok, p2, p2ok := t.assignContacts(pts)

	frame := logicalTouchFrame{
		OneTap: !t.oneTapUntil.IsZero() && now.Before(t.oneTapUntil),
		TwoTap: !t.twoTapUntil.IsZero() && now.Before(t.twoTapUntil),
	}
	if p1ok {
		frame.Count++
		frame.PrimaryActive = true
		frame.PrimaryID = p1.ID
		frame.PrimaryX = p1.X
		frame.PrimaryY = p1.Y
		frame.Abs1X = touchNormX(p1.X)
		frame.Abs1Y = touchNormY(p1.Y)
	}
	if p2ok {
		frame.Count++
		frame.SecondaryActive = true
		frame.SecondaryID = p2.ID
		frame.SecondaryX = p2.X
		frame.SecondaryY = p2.Y
		frame.Abs2X = touchNormX(p2.X)
		frame.Abs2Y = touchNormY(p2.Y)
	}

	// One-finger motion: follow the same stable contact ID. Do not synthesize
	// a delta on contact-down, contact promotion, or 2->1 transitions.
	if frame.Count == 1 && t.previousCount == 1 && t.prevPrimary.valid && t.prevPrimary.id == p1.ID {
		dx := touchNormX(p1.X) - touchNormX(t.prevPrimary.x)
		dy := touchNormY(p1.Y) - touchNormY(t.prevPrimary.y)
		frame.RawOneDX, frame.RawOneDY = dx, dy
		frame.OneDX = motionAxis(dx)
		frame.OneDY = motionAxis(dy)
	}

	// Two-finger movement follows the centroid, not "finger slot 2". This is
	// stable if Sony swaps report slots and matches common two-finger pan
	// semantics. Pinch is the change in normalized inter-finger distance.
	if frame.Count == 2 {
		cx := (touchNormX(p1.X) + touchNormX(p2.X)) * 0.5
		cy := (touchNormY(p1.Y) + touchNormY(p2.Y)) * 0.5
		dist := normalizedDistance(p1.X, p1.Y, p2.X, p2.Y)
		if t.previousCount == 2 && t.prevPrimary.valid && t.prevSecondary.valid &&
			t.prevPrimary.id == p1.ID && t.prevSecondary.id == p2.ID {
			dx, dy := cx-t.prevCentroidX, cy-t.prevCentroidY
			frame.RawTwoDX, frame.RawTwoDY = dx, dy
			frame.TwoDX = motionAxis(dx)
			frame.TwoDY = motionAxis(dy)
			frame.Pinch = motionAxis(dist - t.prevDistance)
		}
		t.prevCentroidX = cx
		t.prevCentroidY = cy
		t.prevDistance = dist
	}

	t.previousCount = frame.Count
	if p1ok {
		t.prevPrimary = trackedContact{valid: true, id: p1.ID, x: p1.X, y: p1.Y}
	} else {
		t.prevPrimary = trackedContact{}
	}
	if p2ok {
		t.prevSecondary = trackedContact{valid: true, id: p2.ID, x: p2.X, y: p2.Y}
	} else {
		t.prevSecondary = trackedContact{}
	}

	return frame
}
