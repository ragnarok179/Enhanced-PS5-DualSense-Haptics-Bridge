package main

import (
	"fmt"
	"math"
	"sync"
	"time"
)

// rawBumpRenderer is deliberately separate from hapticMixer. It is an A/B
// renderer for suspension bumps only: one accepted Lua bodyEvent -> one fixed,
// deterministic stereo pulse. No road texture, no collision/landing layer,
// no body isolation, no crossfeed, no voice merge and no strength remapping.
//
// It exists to answer one question cleanly: is the remaining bump problem in
// BeamNG event detection, or in the bridge mixer/rendering stage?
type rawBumpPulse struct {
	event     int
	side      int
	source    float64
	queuedAt  time.Time
	startedAt time.Time
}

type rawBumpRenderer struct {
	mu sync.Mutex

	synced    bool
	lastEvent int

	queue   []rawBumpPulse
	current *rawBumpPulse
	pos     int
	total   int

	accepted int
	played   int
	dropped  int

	lastStartedEvent int
	lastStartedSide  int
	lastQueueDelay   time.Duration
}

func newRawBumpRenderer() *rawBumpRenderer { return &rawBumpRenderer{} }

func rawBumpSideName(side int) string {
	switch {
	case side < 0:
		return "LEFT"
	case side > 0:
		return "RIGHT"
	default:
		return "CENTER"
	}
}

func isRawBumpEvent(t telemetry) bool {
	return t.BodyProfile == "suspension_bump" || t.BodyKind == "wheel"
}

// observe must be called for every decoded UDP packet, not from the haptic
// render tick. That way a short-lived serialized event cannot disappear merely
// because several telemetry packets arrived between two 10.67 ms audio slots.
func (r *rawBumpRenderer) observe(t telemetry, now time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.synced || t.BodyEvent < r.lastEvent {
		r.lastEvent = t.BodyEvent
		r.synced = true
		return
	}
	if t.BodyEvent <= 0 || t.BodyEvent == r.lastEvent {
		return
	}
	r.lastEvent = t.BodyEvent
	if !t.Active || !isRawBumpEvent(t) {
		return
	}

	source := math.Max(t.BodyStrength, math.Max(t.BodyLeftStrength, t.BodyRightStrength))
	p := rawBumpPulse{event: t.BodyEvent, side: t.BodySide, source: source, queuedAt: now}

	// A queue is intentional here. The diagnostic must never merge two events
	// into a louder pulse or silently overwrite the previous one. If Lua emits
	// duplicates, the user should feel distinct duplicate taps; that is useful
	// evidence that the problem is upstream of the mixer.
	const maxQueued = 24
	if len(r.queue) >= maxQueued {
		r.dropped++
		fmt.Printf("RAW_BUMP DROP event=%d side=%s src=%.4f queue=%d\n", p.event, rawBumpSideName(p.side), p.source, len(r.queue))
		return
	}
	r.queue = append(r.queue, p)
	r.accepted++
	fmt.Printf("RAW_BUMP QUEUE event=%d side=%s src=%.4f queue=%d\n", p.event, rawBumpSideName(p.side), p.source, len(r.queue))
}

func (r *rawBumpRenderer) startNextLocked(sampleRate int, now time.Time) {
	if r.current != nil || len(r.queue) == 0 {
		return
	}
	p := r.queue[0]
	r.queue = r.queue[1:]
	p.startedAt = now
	r.current = &p
	r.pos = 0
	// 52 ms is deliberately fixed. It is shorter than most bumper spacing in
	// the captures and independent of bodyStrength/vertical chassis motion.
	r.total = int(math.Round(float64(sampleRate) * 0.052))
	if r.total < 24 {
		r.total = 24
	}
	r.played++
	r.lastStartedEvent = p.event
	r.lastStartedSide = p.side
	r.lastQueueDelay = now.Sub(p.queuedAt)
	fmt.Printf("RAW_BUMP PLAY event=%d side=%s src=%.4f delay=%.1fms pending=%d\n",
		p.event, rawBumpSideName(p.side), p.source,
		float64(r.lastQueueDelay)/float64(time.Millisecond), len(r.queue))
}

// render returns signed stereo PCM. Its amplitude and duration do not depend on
// the source strength. LEFT and RIGHT are hard-panned; CENTER is equal-energy on
// both grips. This intentionally removes every mixer variable from the test.
func (r *rawBumpRenderer) render(frames, sampleRate int, now time.Time) []int8 {
	if frames <= 0 {
		return nil
	}
	if sampleRate < 1000 {
		sampleRate = canonicalHapticSampleRate
	}
	out := make([]int8, frames*2)

	r.mu.Lock()
	defer r.mu.Unlock()

	for i := 0; i < frames; i++ {
		r.startNextLocked(sampleRate, now)
		if r.current == nil || r.total <= 0 {
			continue
		}

		p := float64(r.pos) / float64(r.total)
		sec := float64(r.pos) / float64(sampleRate)

		// Fast attack + compact decay. The carrier is deterministic and has no
		// random/noise component, so identical events generate identical PCM.
		attack := 1.0
		attackSamples := int(math.Round(float64(sampleRate) * 0.0025))
		if attackSamples < 1 {
			attackSamples = 1
		}
		if r.pos < attackSamples {
			attack = float64(r.pos+1) / float64(attackSamples)
		}
		envelope := attack * math.Pow(math.Max(0, 1-p), 1.65)
		wave := 0.76*math.Sin(2*math.Pi*88*sec) + 0.24*math.Sin(2*math.Pi*176*sec)
		mono := 0.62 * envelope * wave

		l, rr := 0.0, 0.0
		switch {
		case r.current.side < 0:
			l = mono
		case r.current.side > 0:
			rr = mono
		default:
			// 1/sqrt(2) keeps roughly the same total energy when centered.
			l = mono * 0.70710678118
			rr = mono * 0.70710678118
		}
		out[i*2] = int8(math.Round(clamp(l*127.0, -127, 127)))
		out[i*2+1] = int8(math.Round(clamp(rr*127.0, -127, 127)))

		r.pos++
		if r.pos >= r.total {
			r.current = nil
			r.pos, r.total = 0, 0
		}
	}
	return out
}

type rawBumpStats struct {
	Accepted   int
	Played     int
	Dropped    int
	Pending    int
	Active     bool
	Event      int
	Side       int
	QueueDelay time.Duration
}

func (r *rawBumpRenderer) stats() rawBumpStats {
	r.mu.Lock()
	defer r.mu.Unlock()
	return rawBumpStats{
		Accepted:   r.accepted,
		Played:     r.played,
		Dropped:    r.dropped,
		Pending:    len(r.queue),
		Active:     r.current != nil,
		Event:      r.lastStartedEvent,
		Side:       r.lastStartedSide,
		QueueDelay: r.lastQueueDelay,
	}
}
