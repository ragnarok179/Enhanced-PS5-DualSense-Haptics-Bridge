package main

// sharedPCMQueue is the final USB audio buffer. The Common Feel Engine always
// produces one canonical 48 kHz stream. This queue only adapts that stream if a
// Windows audio endpoint exposes a non-48-kHz format; normal DualSense USB is a
// direct 48-kHz pass-through.
type sharedPCMQueue struct {
	data       []int8
	pos        float64
	sourceRate int
}

func (q *sharedPCMQueue) reset()              { q.data = q.data[:0]; q.pos = 0; q.sourceRate = 0 }
func (q *sharedPCMQueue) push(samples []int8) { q.pushAtRate(samples, canonicalHapticSampleRate) }
func (q *sharedPCMQueue) pushAtRate(samples []int8, sourceRate int) {
	if len(samples) < 2 {
		return
	}
	if len(samples)%2 != 0 {
		samples = samples[:len(samples)-1]
	}
	if sourceRate < 1000 {
		sourceRate = canonicalHapticSampleRate
	}
	if q.sourceRate != 0 && q.sourceRate != sourceRate {
		q.reset()
	}
	q.sourceRate = sourceRate
	q.data = append(q.data, samples...)
	// Keep the USB queue short. A long software queue turns stale PCM into
	// perceptible controller latency even if WASAPI itself is configured well.
	maxFrames := sourceRate / 20 // 50 ms hard safety cap
	frames := len(q.data) / 2
	if frames > maxFrames {
		drop := frames - maxFrames
		q.data = append([]int8(nil), q.data[drop*2:]...)
		q.pos -= float64(drop)
		if q.pos < 0 {
			q.pos = 0
		}
	}
}
func (q *sharedPCMQueue) availableSourceFrames() int {
	frames := len(q.data) / 2
	remaining := float64(frames) - q.pos
	if remaining <= 0 {
		return 0
	}
	return int(remaining)
}

func (q *sharedPCMQueue) availableOutputFrames(outputRate int) int {
	if outputRate <= 0 {
		return 0
	}
	rate := q.sourceRate
	if rate < 1000 {
		rate = canonicalHapticSampleRate
	}
	remaining := float64(len(q.data)/2) - q.pos
	if remaining <= 0 {
		return 0
	}
	return int(remaining * float64(outputRate) / float64(rate))
}
func (q *sharedPCMQueue) renderHold(dstFrames, outputRate int) (left, right []float64) {
	return q.render(dstFrames, outputRate)
}

func (q *sharedPCMQueue) render(dstFrames, outputRate int) (left, right []float64) {
	if dstFrames <= 0 || outputRate <= 0 {
		return nil, nil
	}
	left = make([]float64, dstFrames)
	right = make([]float64, dstFrames)
	q.renderInto(left, right, outputRate)
	return
}

// renderInto is the allocation-free runtime path. The USB engine owns reusable
// scratch slices and passes them here on every audio tick; tests can keep using
// render/renderHold for convenience.
func (q *sharedPCMQueue) renderInto(left, right []float64, outputRate int) int {
	dstFrames := len(left)
	if len(right) < dstFrames {
		dstFrames = len(right)
	}
	if dstFrames <= 0 || outputRate <= 0 {
		return 0
	}
	clear(left[:dstFrames])
	clear(right[:dstFrames])
	rate := q.sourceRate
	if rate < 1000 {
		rate = canonicalHapticSampleRate
	}
	step := float64(rate) / float64(outputRate)
	frames := len(q.data) / 2
	rendered := 0
	for i := 0; i < dstFrames; i++ {
		idx := int(q.pos)
		if idx < 0 || idx >= frames {
			break
		}
		frac := q.pos - float64(idx)
		next := idx + 1
		if next >= frames {
			next = idx
		}
		l0, l1 := float64(q.data[idx*2])/127.0, float64(q.data[next*2])/127.0
		r0, r1 := float64(q.data[idx*2+1])/127.0, float64(q.data[next*2+1])/127.0
		left[i] = l0 + (l1-l0)*frac
		right[i] = r0 + (r1-r0)*frac
		q.pos += step
		rendered++
	}
	consumed := int(q.pos)
	if consumed > 0 {
		if consumed > frames {
			consumed = frames
		}
		q.data = q.data[consumed*2:]
		q.pos -= float64(consumed)
		if q.pos < 0 {
			q.pos = 0
		}
	}
	return rendered
}
