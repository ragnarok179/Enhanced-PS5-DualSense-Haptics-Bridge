package main

import (
	"archive/zip"
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const fsbModeFADPCM = 16

type nativeBeamNGSample struct {
	Name       string
	BankPath   string
	Mode       uint32
	Frequency  int
	Channels   int
	Samples    int
	DataOffset int64
	DataSize   int64
	SizeClass  int
	FSBOffset  int64
	FSBSize    int64
	Subsong    int
}

type nativeBeamNGSampleEngine struct {
	transport  string
	mu         sync.Mutex
	ready      atomic.Bool
	closed     atomic.Bool
	initErr    string
	beamngPath string
	cacheDir   string
	collision  []nativeBeamNGSample
	shrapnel   []nativeBeamNGSample
	rng        *rand.Rand
	lastSample string
	lastName   string
	decodes    atomic.Uint64
	failures   atomic.Uint64
}

type nativePCMResult struct {
	PCM     []float64
	Samples []string
	Backend string
}

func newNativeBeamNGSampleEngine(transport string) *nativeBeamNGSampleEngine {
	if transport == "" {
		transport = "USB"
	}
	n := &nativeBeamNGSampleEngine{transport: transport, rng: rand.New(rand.NewSource(time.Now().UnixNano()))}
	if runtimeDiagnosticsEnabled() {
		fmt.Printf("NATIVE_AUDIO_INIT transport=%s profile=collision-only backend=BeamNG-FSB5 status=starting\n", transport)
	}
	go n.initialize()
	return n
}

func (n *nativeBeamNGSampleEngine) initialize() {
	beamng := locateBeamNGForNativeSamples()
	if beamng == "" {
		n.failInit("BeamNG.drive installation not found")
		return
	}
	n.beamngPath = beamng
	if runtimeDiagnosticsEnabled() {
		fmt.Printf("NATIVE_BEAMNG_FOUND path=%q\n", beamng)
	}
	zpath := filepath.Join(beamng, "content", "art_sound.zip")
	cache, banks, err := extractNativeBeamNGBanks(zpath)
	if err != nil {
		n.failInit("art_sound extraction: " + err.Error())
		return
	}
	n.cacheDir = cache
	if runtimeDiagnosticsEnabled() {
		fmt.Printf("NATIVE_BANK_FOUND profile=collision-only backend=FSB5 artSound=%q cache=%q files=%d\n", zpath, cache, len(banks))
	}

	n.mu.Lock()
	for _, bank := range banks {
		samples, scanErr := scanNativeFSB5Bank(bank)
		if scanErr != nil && runtimeDiagnosticsEnabled() {
			fmt.Printf("NATIVE_FSB5_SCAN file=%q status=partial error=%q\n", filepath.Base(bank), scanErr.Error())
		}
		if runtimeDiagnosticsEnabled() {
			fmt.Printf("NATIVE_FSB5_FOUND file=%q samples=%d\n", filepath.Base(bank), len(samples))
		}
		for _, sample := range samples {
			low := strings.ToLower(sample.Name)
			if sample.Mode != fsbModeFADPCM || sample.Frequency <= 0 || sample.Channels != 1 {
				continue
			}
			switch {
			case strings.HasPrefix(low, "vehicle_impact_shrapnel_"):
				n.shrapnel = append(n.shrapnel, sample)
			case strings.HasPrefix(low, "vehicle_impact_bump_"):
				// Suspension/bump assets are intentionally excluded from the public speaker profile.
			case strings.HasPrefix(low, "vehicle_impact_"):
				n.collision = append(n.collision, sample)
			}
		}
	}
	collisionCount, shrapnelCount := len(n.collision), len(n.shrapnel)
	n.mu.Unlock()
	if collisionCount == 0 {
		n.failInit("native collision sample pool is empty")
		return
	}
	n.ready.Store(true)
	if runtimeDiagnosticsEnabled() {
		fmt.Printf("NATIVE_AUDIO_READY transport=%s profile=collision-only backend=BeamNG-FSB5 collision=%d shrapnel=%d\n", n.transport, collisionCount, shrapnelCount)
	}
}

func (n *nativeBeamNGSampleEngine) failInit(msg string) {
	n.mu.Lock()
	n.initErr = msg
	n.mu.Unlock()
	if runtimeDiagnosticsEnabled() {
		fmt.Printf("NATIVE_AUDIO_INIT transport=%s profile=collision-only backend=BeamNG-FSB5 status=unavailable error=%q\n", n.transport, msg)
	} else {
		fmt.Println("Controller speaker unavailable. Run a diagnostic log for details.")
	}
}
func (n *nativeBeamNGSampleEngine) close() {
	if n != nil {
		n.closed.Store(true)
	}
}

func (n *nativeBeamNGSampleEngine) render(kind string, strength float64, uiVolume int) (nativePCMResult, bool) {
	return n.renderExact(kind, "", strength, uiVolume)
}

func (n *nativeBeamNGSampleEngine) renderExact(kind, eventPath string, strength float64, uiVolume int) (nativePCMResult, bool) {
	if n == nil || kind != "collision" || !n.ready.Load() || n.closed.Load() || uiVolume <= 0 {
		return nativePCMResult{}, false
	}
	n.mu.Lock()
	if n.rng == nil {
		n.rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	main, ok := chooseNativeSampleAvoid(n.collision, strength, n.rng, n.lastName)
	if ok {
		n.lastName = main.Name
	}
	var layer nativeBeamNGSample
	addLayer := false
	if strength >= 0.78 && len(n.shrapnel) > 0 {
		layer, addLayer = chooseNativeSample(n.shrapnel, strength, n.rng)
	}
	n.mu.Unlock()
	if !ok {
		return nativePCMResult{}, false
	}

	pcm, err := decodeNativeBeamNGSample(main)
	if err != nil || len(pcm) == 0 {
		n.failures.Add(1)
		if runtimeDiagnosticsEnabled() {
			fmt.Printf("NATIVE_FALLBACK kind=collision reason=decode sample=%q error=%q\n", main.Name, fmt.Sprint(err))
		}
		return nativePCMResult{}, false
	}
	names := []string{main.Name}
	if addLayer {
		if lp, layerErr := decodeNativeBeamNGSample(layer); layerErr == nil && len(lp) > 0 {
			mixNativePCM(pcm, lp, 0.26+0.18*clamp01(strength))
			names = append(names, layer.Name)
		}
	}
	applyNativeGainAndFade(pcm, nativeKindGain("collision", strength, uiVolume))
	n.decodes.Add(1)
	n.mu.Lock()
	n.lastSample = strings.Join(names, "+")
	n.mu.Unlock()
	if runtimeDiagnosticsEnabled() {
		fmt.Printf("NATIVE_AUDIO_EVENT kind=collision event=%q family=collision-impact sample=%q strength=%.3f volume=%d%%\n", eventPath, strings.Join(names, "+"), strength, uiVolume)
		fmt.Printf("NATIVE_PCM_DECODE kind=collision event=%q frames=%d sampleRate=48000 sourceRate=%d codec=FADPCM\n", eventPath, len(pcm), main.Frequency)
		fmt.Printf("NATIVE_SPEAKER_RENDER transport=%s profile=collision-only backend=BeamNG-FSB5 kind=collision\n", n.transport)
	}
	return nativePCMResult{PCM: pcm, Samples: names, Backend: "BeamNG-FSB5"}, true
}

func nativeKindGain(kind string, strength float64, uiVolume int) float64 {
	if kind != "collision" {
		return 0
	}
	return float64(uiVolume) / 100.0 * (0.48 + 0.52*clamp01(strength))
}

func (n *nativeBeamNGSampleEngine) statusLine() string {
	if n == nil {
		return "NATIVE_AUDIO_STATUS transport=unknown profile=collision-only backend=BeamNG-FSB5 ready=false reason=engine_nil"
	}
	n.mu.Lock()
	errText, beamng, cache, last := n.initErr, n.beamngPath, n.cacheDir, n.lastSample
	c, sh := len(n.collision), len(n.shrapnel)
	n.mu.Unlock()
	return fmt.Sprintf("NATIVE_AUDIO_STATUS transport=%s profile=collision-only backend=BeamNG-FSB5 ready=%t decodes=%d failures=%d collision=%d shrapnel=%d lastSample=%q beamng=%q cache=%q error=%q", n.transport, n.ready.Load(), n.decodes.Load(), n.failures.Load(), c, sh, last, beamng, cache, errText)
}

func nativeCodecName(mode uint32) string {
	if mode == fsbModeFADPCM {
		return "FADPCM"
	}
	return fmt.Sprintf("mode_%d", mode)
}
func chooseNativeSampleAvoid(pool []nativeBeamNGSample, strength float64, rng *rand.Rand, avoid string) (nativeBeamNGSample, bool) {
	if len(pool) == 0 {
		return nativeBeamNGSample{}, false
	}
	if len(pool) == 1 || avoid == "" {
		return chooseNativeSample(pool, strength, rng)
	}
	for i := 0; i < 6; i++ {
		s, ok := chooseNativeSample(pool, strength, rng)
		if !ok {
			return nativeBeamNGSample{}, false
		}
		if s.Name != avoid {
			return s, true
		}
	}
	for _, s := range pool {
		if s.Name != avoid {
			return s, true
		}
	}
	return chooseNativeSample(pool, strength, rng)
}

func chooseNativeSample(pool []nativeBeamNGSample, strength float64, rng *rand.Rand) (nativeBeamNGSample, bool) {
	if len(pool) == 0 {
		return nativeBeamNGSample{}, false
	}
	wanted := 1
	if strength < 0.42 {
		wanted = 0
	} else if strength > 0.76 {
		wanted = 2
	}
	candidates := make([]nativeBeamNGSample, 0, len(pool))
	for _, s := range pool {
		if s.SizeClass == wanted {
			candidates = append(candidates, s)
		}
	}
	if len(candidates) == 0 {
		candidates = pool
	}
	return candidates[rng.Intn(len(candidates))], true
}

func nativeSizeClass(name string) int {
	low := strings.ToLower(name)
	if strings.Contains(low, "_sml") || strings.Contains(low, "small") {
		return 0
	}
	if strings.Contains(low, "_lrg") || strings.Contains(low, "large") || strings.Contains(low, "hood") {
		return 2
	}
	return 1
}

func decodeNativeBeamNGSample(s nativeBeamNGSample) ([]float64, error) {
	if s.Mode != fsbModeFADPCM {
		return nil, fmt.Errorf("unsupported codec mode %d", s.Mode)
	}
	f, err := os.Open(s.BankPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if s.DataSize <= 0 || s.DataSize > 64<<20 {
		return nil, fmt.Errorf("invalid data size %d", s.DataSize)
	}
	data := make([]byte, s.DataSize)
	if _, err = io.ReadFull(io.NewSectionReader(f, s.DataOffset, s.DataSize), data); err != nil {
		return nil, err
	}
	pcm16, err := decodeFMODFADPCMMono(data, s.Samples)
	if err != nil {
		return nil, err
	}
	mono := make([]float64, len(pcm16))
	for i, v := range pcm16 {
		mono[i] = float64(v) / 32768.0
	}
	if s.Frequency != 48000 && s.Frequency > 0 {
		mono = resampleNativePCM(mono, s.Frequency, 48000)
	}
	return mono, nil
}

var fmodFADPCMCoefs = [8][2]int32{{0, 0}, {60, 0}, {122, 60}, {115, 52}, {98, 55}, {0, 0}, {0, 0}, {0, 0}}

func decodeFMODFADPCMMono(data []byte, targetSamples int) ([]int16, error) {
	if targetSamples <= 0 {
		targetSamples = (len(data) / 140) * 256
	}
	out := make([]int16, 0, targetSamples)
	pos := 0
	for len(out) < targetSamples && pos+12 <= len(data) {
		co := binary.LittleEndian.Uint32(data[pos:])
		sh := binary.LittleEndian.Uint32(data[pos+4:])
		hist1 := int32(int16(binary.LittleEndian.Uint16(data[pos+8:])))
		hist2 := int32(int16(binary.LittleEndian.Uint16(data[pos+10:])))
		pos += 12
		for i := 0; i < 8 && len(out) < targetSamples; i++ {
			idx := int((co>>uint(i*4))&0x0f) % 7
			shift := 22 - int((sh>>uint(i*4))&0x0f)
			c1, c2 := fmodFADPCMCoefs[idx][0], fmodFADPCMCoefs[idx][1]
			for j := 0; j < 4 && len(out) < targetSamples; j++ {
				if pos+4 > len(data) {
					break
				}
				nib := binary.LittleEndian.Uint32(data[pos:])
				pos += 4
				for k := 0; k < 8 && len(out) < targetSamples; k++ {
					n := int32((nib >> uint(k*4)) & 0x0f)
					// Sign-extend the 4-bit nibble through the same 32-bit path
					// used by FMOD/vgmstream-derived decoders.
					sample := (n << 28) >> uint(shift)
					sample = (sample - hist2*c2 + hist1*c1) >> 6
					if sample > 32767 {
						sample = 32767
					} else if sample < -32768 {
						sample = -32768
					}
					out = append(out, int16(sample))
					hist2 = hist1
					hist1 = sample
				}
			}
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no FADPCM frames decoded")
	}
	return out, nil
}

func resampleNativePCM(in []float64, src, dst int) []float64 {
	if len(in) == 0 || src <= 0 || dst <= 0 || src == dst {
		return in
	}
	outN := int(math.Round(float64(len(in)) * float64(dst) / float64(src)))
	if outN < 1 {
		outN = 1
	}
	out := make([]float64, outN)
	ratio := float64(src) / float64(dst)
	for i := range out {
		x := float64(i) * ratio
		j := int(x)
		frac := x - float64(j)
		if j >= len(in)-1 {
			out[i] = in[len(in)-1]
		} else {
			out[i] = in[j]*(1-frac) + in[j+1]*frac
		}
	}
	return out
}
func mixNativePCM(dst, src []float64, gain float64) {
	for i := 0; i < len(dst) && i < len(src); i++ {
		dst[i] = clampNativeAudio(dst[i] + src[i]*gain)
	}
}
func applyNativeGainAndFade(pcm []float64, gain float64) {
	if len(pcm) == 0 {
		return
	}
	fade := 240
	if fade*2 > len(pcm) {
		fade = len(pcm) / 2
	}
	for i := range pcm {
		g := gain
		if i < fade {
			g *= float64(i) / float64(fade)
		}
		if i >= len(pcm)-fade {
			g *= float64(len(pcm)-1-i) / float64(fade)
		}
		pcm[i] = clampNativeAudio(pcm[i] * g)
	}
}
func clampNativeAudio(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	if v > 1 {
		return 1
	}
	if v < -1 {
		return -1
	}
	return v
}

// ---- FSB5 parser ----

type fsb5Header struct {
	Offset                                                                int64
	Version, NumSamples, SampleHeadersSize, NameTableSize, DataSize, Mode uint32
	HeaderSize                                                            int64
}
type fsb5SampleHeader struct {
	Name                         string
	Frequency, Channels, Samples int
	DataOffset                   int64
}

func scanNativeFSB5Bank(path string) ([]nativeBeamNGSample, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	offsets, err := findFSB5Offsets(f)
	if err != nil {
		return nil, err
	}
	var out []nativeBeamNGSample
	var firstErr error
	for _, off := range offsets {
		ss, e := parseFSB5At(f, path, off)
		if e != nil {
			if firstErr == nil {
				firstErr = e
			}
			continue
		}
		out = append(out, ss...)
	}
	return out, firstErr
}
func findFSB5Offsets(f *os.File) ([]int64, error) {
	if _, err := f.Seek(0, 0); err != nil {
		return nil, err
	}
	buf := make([]byte, 1<<20)
	carry := []byte{}
	var base int64
	var out []int64
	for {
		n, err := f.Read(buf)
		if n > 0 {
			data := append(carry, buf[:n]...)
			start := 0
			for {
				i := bytes.Index(data[start:], []byte("FSB5"))
				if i < 0 {
					break
				}
				idx := start + i
				abs := base - int64(len(carry)) + int64(idx)
				if len(out) == 0 || out[len(out)-1] != abs {
					out = append(out, abs)
				}
				start = idx + 4
			}
			if len(data) >= 3 {
				carry = append(carry[:0], data[len(data)-3:]...)
			}
			base += int64(n)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}
func parseFSB5At(f *os.File, path string, off int64) ([]nativeBeamNGSample, error) {
	hbuf := make([]byte, 64)
	if _, err := f.ReadAt(hbuf, off); err != nil && err != io.EOF {
		return nil, err
	}
	if string(hbuf[:4]) != "FSB5" {
		return nil, fmt.Errorf("bad magic at %d", off)
	}
	h := fsb5Header{Offset: off, Version: binary.LittleEndian.Uint32(hbuf[4:8]), NumSamples: binary.LittleEndian.Uint32(hbuf[8:12]), SampleHeadersSize: binary.LittleEndian.Uint32(hbuf[12:16]), NameTableSize: binary.LittleEndian.Uint32(hbuf[16:20]), DataSize: binary.LittleEndian.Uint32(hbuf[20:24]), Mode: binary.LittleEndian.Uint32(hbuf[24:28]), HeaderSize: 60}
	if h.Version == 0 {
		h.HeaderSize = 64
	}
	if h.NumSamples == 0 || h.NumSamples > 200000 || h.SampleHeadersSize > 64<<20 || h.NameTableSize > 64<<20 {
		return nil, fmt.Errorf("implausible FSB5 header at %d", off)
	}
	sh := make([]byte, h.SampleHeadersSize)
	if _, err := f.ReadAt(sh, off+h.HeaderSize); err != nil {
		return nil, err
	}
	pos := 0
	headers := make([]fsb5SampleHeader, 0, h.NumSamples)
	for i := 0; i < int(h.NumSamples); i++ {
		if pos+8 > len(sh) {
			return nil, fmt.Errorf("truncated sample header")
		}
		raw := binary.LittleEndian.Uint64(sh[pos:])
		pos += 8
		nxt := raw & 1
		freqCode := (raw >> 1) & 0xf
		channels := int((raw>>5)&1) + 1
		dataOff := int64((raw>>6)&0x0fffffff) * 16
		samples := int((raw >> 34) & 0x3fffffff)
		freq := fsbFrequency(int(freqCode))
		for nxt != 0 {
			if pos+4 > len(sh) {
				return nil, fmt.Errorf("truncated metadata")
			}
			cr := binary.LittleEndian.Uint32(sh[pos:])
			pos += 4
			nxt = uint64(cr & 1)
			sz := int((cr >> 1) & 0x00ffffff)
			typ := (cr >> 25) & 0x7f
			if pos+sz > len(sh) {
				return nil, fmt.Errorf("bad metadata size")
			}
			dat := sh[pos : pos+sz]
			pos += sz
			if typ == 1 && sz >= 1 {
				channels = int(dat[0])
			}
			if typ == 2 && sz >= 4 {
				freq = int(binary.LittleEndian.Uint32(dat))
			}
		}
		headers = append(headers, fsb5SampleHeader{Name: fmt.Sprintf("%04d", i), Frequency: freq, Channels: channels, Samples: samples, DataOffset: dataOff})
	}
	if h.NameTableSize > 0 {
		nt := make([]byte, h.NameTableSize)
		ntOff := off + h.HeaderSize + int64(h.SampleHeadersSize)
		if _, err := f.ReadAt(nt, ntOff); err != nil {
			return nil, err
		}
		need := int(h.NumSamples) * 4
		if need <= len(nt) {
			for i := range headers {
				o := int(binary.LittleEndian.Uint32(nt[i*4:]))
				if o < 0 || o >= len(nt) {
					continue
				}
				end := o
				for end < len(nt) && nt[end] != 0 {
					end++
				}
				if end > o {
					headers[i].Name = string(nt[o:end])
				}
			}
		}
	}
	dataBase := off + h.HeaderSize + int64(h.SampleHeadersSize) + int64(h.NameTableSize)
	out := make([]nativeBeamNGSample, 0, len(headers))
	for i, s := range headers {
		end := int64(h.DataSize)
		if i+1 < len(headers) {
			end = headers[i+1].DataOffset
		}
		size := end - s.DataOffset
		if size <= 0 {
			continue
		}
		out = append(out, nativeBeamNGSample{Name: s.Name, BankPath: path, Mode: h.Mode, Frequency: s.Frequency, Channels: s.Channels, Samples: s.Samples, DataOffset: dataBase + s.DataOffset, DataSize: size, SizeClass: nativeSizeClass(s.Name), FSBOffset: off, FSBSize: h.HeaderSize + int64(h.SampleHeadersSize) + int64(h.NameTableSize) + int64(h.DataSize), Subsong: i + 1})
	}
	return out, nil
}
func fsbFrequency(code int) int {
	switch code {
	case 1:
		return 8000
	case 2:
		return 11000
	case 3:
		return 11025
	case 4:
		return 16000
	case 5:
		return 22050
	case 6:
		return 24000
	case 7:
		return 32000
	case 8:
		return 44100
	case 9:
		return 48000
	}
	return 0
}

func extractNativeBeamNGBanks(zpath string) (string, []string, error) {
	zr, err := zip.OpenReader(zpath)
	if err != nil {
		return "", nil, err
	}
	defer zr.Close()
	st, _ := os.Stat(zpath)
	fingerprint := "unknown"
	if st != nil {
		fingerprint = fmt.Sprintf("%d_%d", st.Size(), st.ModTime().Unix())
	}
	cache := filepath.Join(os.TempDir(), "DualSenseBeamNGCollisionSamples", fingerprint)
	if err := os.MkdirAll(cache, 0755); err != nil {
		return "", nil, err
	}
	wanted := map[string]bool{"vehicle_preload.assets.bank": true, "vehicle.assets.bank": true}
	prefix := "art/sound/fmod/desktop/"
	var out []string
	for _, zf := range zr.File {
		name := strings.ReplaceAll(zf.Name, "\\", "/")
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		base := filepath.Base(name)
		if !wanted[base] {
			continue
		}
		dst := filepath.Join(cache, base)
		if fi, e := os.Stat(dst); e == nil && uint64(fi.Size()) == zf.UncompressedSize64 {
			out = append(out, dst)
			continue
		}
		r, e := zf.Open()
		if e != nil {
			return "", nil, e
		}
		tmp := dst + ".tmp"
		w, e := os.Create(tmp)
		if e != nil {
			r.Close()
			return "", nil, e
		}
		_, ce := io.Copy(w, r)
		we := w.Close()
		r.Close()
		if ce != nil {
			return "", nil, ce
		}
		if we != nil {
			return "", nil, we
		}
		_ = os.Remove(dst)
		if e = os.Rename(tmp, dst); e != nil {
			return "", nil, e
		}
		out = append(out, dst)
		if runtimeDiagnosticsEnabled() {
			fmt.Printf("NATIVE_BANK_EXTRACT file=%q bytes=%d\\n", base, zf.UncompressedSize64)
		}
	}
	sort.Strings(out)
	if len(out) < 2 {
		return "", out, fmt.Errorf("required vehicle asset banks missing (%d/2)", len(out))
	}
	return cache, out, nil
}

func locateBeamNGForNativeSamples() string {
	var candidates []string
	for _, env := range []string{"BEAMNG_HOME", "BEAMNG_PATH"} {
		if p := strings.Trim(strings.TrimSpace(os.Getenv(env)), "\""); p != "" {
			candidates = append(candidates, p)
		}
	}
	if exe, err := os.Executable(); err == nil {
		base := filepath.Dir(exe)
		for _, rel := range []string{filepath.Join("..", "Config", "beamng_path.txt"), filepath.Join("..", "..", "Config", "beamng_path.txt")} {
			if data, e := os.ReadFile(filepath.Clean(filepath.Join(base, rel))); e == nil {
				if p := strings.Trim(strings.TrimSpace(string(data)), "\""); p != "" {
					candidates = append(candidates, p)
				}
			}
		}
	}
	for _, root := range nativeSampleSteamRoots() {
		for _, lib := range nativeSampleSteamLibraries(root) {
			candidates = append(candidates, filepath.Join(lib, "steamapps", "common", "BeamNG.drive"))
		}
	}
	for _, d := range []string{"C", "D", "E", "F", "G", "H"} {
		candidates = append(candidates, fmt.Sprintf(`%s:\\SteamLibrary\\steamapps\\common\\BeamNG.drive`, d))
	}
	seen := map[string]bool{}
	for _, p := range candidates {
		p = filepath.Clean(p)
		k := strings.ToLower(p)
		if seen[k] {
			continue
		}
		seen[k] = true
		if fileExistsNative(filepath.Join(p, "content", "art_sound.zip")) {
			return p
		}
	}
	return ""
}
func nativeSampleSteamRoots() []string {
	var roots []string
	if x := os.Getenv("ProgramFiles(x86)"); x != "" {
		roots = append(roots, filepath.Join(x, "Steam"))
	}
	if x := os.Getenv("ProgramFiles"); x != "" {
		roots = append(roots, filepath.Join(x, "Steam"))
	}
	for _, d := range []string{"C", "D", "E", "F", "G", "H"} {
		roots = append(roots, fmt.Sprintf(`%s:\\Steam`, d), fmt.Sprintf(`%s:\\SteamLibrary`, d))
	}
	return roots
}
func nativeSampleSteamLibraries(root string) []string {
	libs := []string{root}
	f, err := os.Open(filepath.Join(root, "steamapps", "libraryfolders.vdf"))
	if err != nil {
		return libs
	}
	defer f.Close()
	re := regexp.MustCompile(`(?i)"path"\s+"([^"]+)"`)
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if m := re.FindStringSubmatch(sc.Text()); len(m) == 2 {
			libs = append(libs, strings.ReplaceAll(m[1], `\\`, `\`))
		}
	}
	return libs
}
func fileExistsNative(p string) bool { st, e := os.Stat(p); return e == nil && !st.IsDir() }
