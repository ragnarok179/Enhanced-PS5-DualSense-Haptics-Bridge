package main

// sharedPCMQueue is the final USB audio buffer. It is a fixed-size stereo ring
// buffer capped at 50 ms, so the steady-state WASAPI path performs no slice
// growth/compaction allocations and cannot accumulate stale haptics.
type sharedPCMQueue struct {
	data       []int8 // fixed interleaved stereo storage once a source rate is known
	head       int    // oldest source frame
	size       int    // queued source frames
	pos        float64
	sourceRate int
}

func (q *sharedPCMQueue) reset() {
	q.head = 0
	q.size = 0
	q.pos = 0
	q.sourceRate = 0
}

func (q *sharedPCMQueue) configure(sourceRate int) {
	if sourceRate < 1000 {
		sourceRate = canonicalHapticSampleRate
	}
	capacityFrames := maxInt(1, sourceRate/20) // 50 ms hard safety cap
	if q.sourceRate == sourceRate && len(q.data) == capacityFrames*2 {
		return
	}
	q.sourceRate = sourceRate
	q.data = make([]int8, capacityFrames*2)
	q.head, q.size, q.pos = 0, 0, 0
}

func (q *sharedPCMQueue) capacityFrames() int {
	return len(q.data) / 2
}

func (q *sharedPCMQueue) frameIndex(relative int) int {
	capFrames := q.capacityFrames()
	if capFrames <= 0 {
		return 0
	}
	return (q.head + relative) % capFrames
}

func (q *sharedPCMQueue) copyFramesIn(samples []int8, frames int) {
	if frames <= 0 {
		return
	}
	capFrames := q.capacityFrames()
	tail := (q.head + q.size) % capFrames
	first := minInt(frames, capFrames-tail)
	copy(q.data[tail*2:(tail+first)*2], samples[:first*2])
	if first < frames {
		copy(q.data[:(frames-first)*2], samples[first*2:frames*2])
	}
	q.size += frames
}

func (q *sharedPCMQueue) dropOldest(frames int) {
	if frames <= 0 || q.size <= 0 {
		return
	}
	if frames >= q.size {
		q.head, q.size, q.pos = 0, 0, 0
		return
	}
	q.head = q.frameIndex(frames)
	q.size -= frames
	// Overflow is a latency safety event. Discard any fractional interpolation
	// position together with the stale whole frames rather than carrying phase
	// into unrelated newer samples.
	q.pos = 0
}

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
	q.configure(sourceRate)
	frames := len(samples) / 2
	capFrames := q.capacityFrames()
	if frames >= capFrames {
		// Keep only the newest 50 ms when a producer overruns the safety cap.
		samples = samples[(frames-capFrames)*2:]
		q.head, q.size, q.pos = 0, 0, 0
		q.copyFramesIn(samples, capFrames)
		return
	}
	if overflow := q.size + frames - capFrames; overflow > 0 {
		q.dropOldest(overflow)
	}
	q.copyFramesIn(samples, frames)
}

func (q *sharedPCMQueue) availableSourceFrames() int {
	remaining := float64(q.size) - q.pos
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
	remaining := float64(q.size) - q.pos
	if remaining <= 0 {
		return 0
	}
	return int(remaining * float64(outputRate) / float64(rate))
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
// scratch slices and passes them here on every audio tick.
func (q *sharedPCMQueue) renderInto(left, right []float64, outputRate int) int {
	dstFrames := minInt(len(left), len(right))
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
	rendered := 0
	for i := 0; i < dstFrames; i++ {
		idx := int(q.pos)
		if idx < 0 || idx >= q.size {
			break
		}
		frac := q.pos - float64(idx)
		next := idx + 1
		if next >= q.size {
			next = idx
		}
		idx0 := q.frameIndex(idx)
		idx1 := q.frameIndex(next)
		l0, l1 := float64(q.data[idx0*2])/127.0, float64(q.data[idx1*2])/127.0
		r0, r1 := float64(q.data[idx0*2+1])/127.0, float64(q.data[idx1*2+1])/127.0
		left[i] = l0 + (l1-l0)*frac
		right[i] = r0 + (r1-r0)*frac
		q.pos += step
		rendered++
	}
	consumed := int(q.pos)
	if consumed > 0 {
		if consumed > q.size {
			consumed = q.size
		}
		q.head = q.frameIndex(consumed)
		q.size -= consumed
		q.pos -= float64(consumed)
		if q.pos < 0 || q.size == 0 {
			q.pos = 0
		}
	}
	return rendered
}
