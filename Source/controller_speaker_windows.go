//go:build windows

package main

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

const (
	speakerCategoryCollision uint16 = 1
	speakerCategoryAll              = speakerCategoryCollision
)

const (
	speakerOutputController byte = iota
	speakerOutputControllerAndPC
)

type speakerSettings struct {
	enabled    bool
	volume     int
	categories uint16
	output     byte
}

var speakerSettingsState struct {
	enabled    atomic.Bool
	volume     atomic.Int32
	categories atomic.Uint32
	output     atomic.Uint32
	generation atomic.Uint32
}

func init() {
	speakerSettingsState.volume.Store(70)
	speakerSettingsState.categories.Store(uint32(speakerCategoryCollision))
	speakerSettingsState.output.Store(uint32(speakerOutputController))
}

func currentSpeakerSettings() speakerSettings {
	return speakerSettings{
		enabled:    speakerSettingsState.enabled.Load(),
		volume:     clampInt(int(speakerSettingsState.volume.Load()), 0, 100),
		categories: uint16(speakerSettingsState.categories.Load()) & speakerCategoryAll,
		output:     byte(speakerSettingsState.output.Load()),
	}
}

func applySpeakerSettings(enabled bool, volume int, categories uint16, output byte) bool {
	volume = clampInt(volume, 0, 100)
	if output > speakerOutputControllerAndPC {
		output = speakerOutputController
	}
	old := currentSpeakerSettings()
	speakerSettingsState.enabled.Store(enabled)
	speakerSettingsState.volume.Store(int32(volume))
	speakerSettingsState.categories.Store(uint32(categories & speakerCategoryAll))
	speakerSettingsState.output.Store(uint32(output))
	now := currentSpeakerSettings()
	changed := old != now
	if changed {
		speakerSettingsState.generation.Add(1)
	}
	return changed
}

func speakerSettingsGeneration() uint32 { return speakerSettingsState.generation.Load() }

var speakerBeamNGSettingsState struct {
	packets  atomic.Uint64
	lastSeen atomic.Int64
}

func applyBeamNGSpeakerSettings(enabled bool, volume int, categories uint16, output byte) {
	changed := applySpeakerSettings(enabled, volume, categories, output)
	count := speakerBeamNGSettingsState.packets.Add(1)
	speakerBeamNGSettingsState.lastSeen.Store(time.Now().UnixNano())
	if runtimeDiagnosticsEnabled() && (count == 1 || changed) {
		cfg := currentSpeakerSettings()
		fmt.Printf("SPEAKER_SETTINGS_RX packet=%d profile=collision-only enabled=%t volume=%d%% categories=0x%04X output=%s changed=%t\n",
			count, cfg.enabled, cfg.volume, cfg.categories, speakerOutputName(cfg.output), changed)
	}
}

func speakerOutputName(output byte) string {
	if output == speakerOutputControllerAndPC {
		return "controller+PC"
	}
	return "controller"
}

type speakerPCMVoice struct {
	id        uint64
	kind      string
	eventPath string
	vehicleID int
	pcm       []float32
	pos       int
}

const maxControllerSpeakerVoices = 32

type controllerSpeakerEngine struct {
	mu             sync.Mutex
	voices         []speakerPCMVoice
	nextVoiceID    uint64
	usbOutput      bool
	pcm            []float32 // latest cue retained for diagnostics/tests
	pcmPos         int
	currentKind    string
	eventsSeen     uint64
	eventsPlayed   uint64
	eventsFiltered uint64
	lastStrength   float64
	lastEventAt    time.Time
	native         *nativeBeamNGSampleEngine

	btEncoderMu      sync.Mutex
	btEncoder        *bluetoothOpusStreamEncoder
	btEncoderBackend string
}

func newControllerSpeakerEngine() *controllerSpeakerEngine { return &controllerSpeakerEngine{} }

func (s *controllerSpeakerEngine) enableNativeUSB() {
	if s == nil {
		return
	}
	s.usbOutput = true
	if s.native == nil {
		s.native = newNativeBeamNGSampleEngine("USB")
	}
}

func (s *controllerSpeakerEngine) enableNativeBluetooth() {
	if s == nil {
		return
	}
	s.usbOutput = false
	if s.native == nil {
		s.native = newNativeBeamNGSampleEngine("Bluetooth")
	}
}

func (s *controllerSpeakerEngine) Close() {
	if s == nil {
		return
	}
	if s.native != nil {
		s.native.close()
	}
	s.btEncoderMu.Lock()
	if s.btEncoder != nil {
		s.btEncoder.Close()
		s.btEncoder = nil
	}
	s.btEncoderBackend = ""
	s.btEncoderMu.Unlock()
}

func (s *controllerSpeakerEngine) nativeDiagnosticStatus(transport string) string {
	if s != nil && s.native != nil {
		return s.native.statusLine()
	}
	return "NATIVE_AUDIO_STATUS transport=" + transport + " profile=collision-only backend=BeamNG-FSB5 ready=false reason=native_engine_not_initialized"
}

func (s *controllerSpeakerEngine) ensureBluetoothEncoder(beamngPath string) (string, error) {
	s.btEncoderMu.Lock()
	defer s.btEncoderMu.Unlock()
	if s.btEncoder != nil {
		return s.btEncoderBackend, nil
	}
	enc, backend, err := newBluetoothOpusStreamEncoder(beamngPath)
	if err != nil {
		return backend, err
	}
	s.btEncoder = enc
	s.btEncoderBackend = backend
	return backend, nil
}

func (s *controllerSpeakerEngine) triggerExact(kind, eventPath string, vehicleID int, strength float64, cfg speakerSettings, now time.Time) {
	if kind != "collision" {
		return
	}

	// BeamNG's real one-shot call is the only trigger. The Bridge only resolves
	// the exact event to BeamNG's own sample and mirrors that sample to the
	// controller. No synthetic collision is generated here.
	if s.native != nil {
		if native, ok := s.native.renderExact("collision", eventPath, strength, cfg.volume); ok && len(native.PCM) > 0 {
			pcm := make([]float32, len(native.PCM))
			for i, v := range native.PCM {
				if math.IsNaN(v) || math.IsInf(v, 0) {
					v = 0
				}
				pcm[i] = float32(clamp(v, -.99, .99))
			}

			backend := "BeamNG-native-sample"
			if !s.usbOutput {
				opusBackend, encErr := s.ensureBluetoothEncoder(s.native.beamngPath)
				if encErr != nil {
					if runtimeDiagnosticsEnabled() {
						fmt.Printf("SPEAKER_BT_NATIVE_ENCODER status=error event=%q error=%q\n", eventPath, encErr.Error())
					}
					return
				}
				backend = "BeamNG-native-sample+FFmpeg-" + opusBackend
				if runtimeDiagnosticsEnabled() {
					fmt.Printf("SPEAKER_BT_NATIVE_ENCODER status=ready backend=%s mode=stream opusBytes=%d hardwareGain=%.2f\n",
						opusBackend, btOpusPacketSize, bluetoothSpeakerHardwareOutputGain)
				}
			}

			s.mu.Lock()
			s.pcm, s.pcmPos = pcm, 0
			queued := s.enqueuePCMVoiceLocked("collision", eventPath, vehicleID, pcm)
			if queued {
				s.currentKind = "collision"
				s.eventsPlayed++
				s.lastStrength = strength
				s.lastEventAt = now
			}
			s.mu.Unlock()
			if !queued {
				return
			}

			if runtimeDiagnosticsEnabled() {
				fmt.Printf("SPEAKER_TRIGGER kind=collision strength=%.3f volume=%d%% output=%s backend=%s samples=%q duration=%dms\n",
					strength, cfg.volume, speakerOutputName(cfg.output), backend, strings.Join(native.Samples, "+"), len(pcm)*1000/48000)
			}
			return
		}
	}

	if runtimeDiagnosticsEnabled() {
		fmt.Printf("SPEAKER_NATIVE_UNAVAILABLE transport=%s event=%q reason=native_pcm_not_ready_or_decode_failed\n", map[bool]string{true: "USB", false: "Bluetooth"}[s.usbOutput], eventPath)
	}
}

func (s *controllerSpeakerEngine) enqueuePCMVoiceLocked(kind, eventPath string, vehicleID int, pcm []float32) bool {
	if len(pcm) == 0 {
		return false
	}
	if len(s.voices) >= maxControllerSpeakerVoices {
		if runtimeDiagnosticsEnabled() {
			fmt.Printf("SPEAKER_VOICE_REJECT kind=collision event=%q reason=voice_limit active=%d\n", eventPath, len(s.voices))
		}
		return false
	}
	s.nextVoiceID++
	s.voices = append(s.voices, speakerPCMVoice{id: s.nextVoiceID, kind: "collision", eventPath: eventPath, vehicleID: vehicleID, pcm: pcm})
	if runtimeDiagnosticsEnabled() {
		transport := "Bluetooth"
		if s.usbOutput {
			transport = "USB"
		}
		fmt.Printf("SPEAKER_VOICE_START transport=%s id=%d kind=collision vehicle=%d event=%q frames=%d active=%d\n",
			transport, s.nextVoiceID, vehicleID, eventPath, len(pcm), len(s.voices))
	}
	return true
}

func (s *controllerSpeakerEngine) flushVehicle(vehicleID int, reason string) int {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	before := len(s.voices)
	kept := s.voices[:0]
	for _, voice := range s.voices {
		if vehicleID >= 0 && voice.vehicleID != vehicleID {
			kept = append(kept, voice)
		}
	}
	if vehicleID < 0 {
		kept = kept[:0]
	}
	s.voices = kept
	dropped := before - len(kept)
	if len(s.voices) == 0 {
		s.currentKind = ""
		s.pcm = nil
		s.pcmPos = 0
	}
	s.mu.Unlock()
	if runtimeDiagnosticsEnabled() {
		transport := "Bluetooth"
		if s.usbOutput {
			transport = "USB"
		}
		fmt.Printf("SPEAKER_FLUSH transport=%s reason=%s vehicle=%d dropped=%d\n", transport, reason, vehicleID, dropped)
	}
	return dropped
}

func (s *controllerSpeakerEngine) hasUSBPCM() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.voices) > 0
}

func (s *controllerSpeakerEngine) renderUSB(dst []byte, frames int, channels, blockAlign int, floatPCM bool) bool {
	if s == nil || frames <= 0 || channels < 2 {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.voices) == 0 {
		return false
	}

	wrote := false
	speakerChannel := 1
	started := make(map[uint64]speakerPCMVoice)
	for f := 0; f < frames; f++ {
		active := 0
		sum := 0.0
		for i := range s.voices {
			v := &s.voices[i]
			if v.pos >= len(v.pcm) {
				continue
			}
			if v.pos == 0 {
				started[v.id] = *v
			}
			sample := float64(v.pcm[v.pos])
			if math.IsNaN(sample) || math.IsInf(sample, 0) {
				sample = 0
			}
			sum += sample
			v.pos++
			active++
		}
		if active == 0 {
			break
		}
		if active > 1 {
			sum /= math.Sqrt(float64(active))
		}
		v := clamp(sum, -0.99, 0.99)
		if math.Abs(v) > .000001 {
			wrote = true
		}
		if floatPCM {
			off := f*blockAlign + speakerChannel*4
			if off+4 <= len(dst) {
				*(*uint32)(unsafe.Pointer(&dst[off])) = math.Float32bits(float32(v))
			}
		} else {
			off := f*blockAlign + speakerChannel*2
			if off+2 <= len(dst) {
				q := int16(v * 32767)
				dst[off], dst[off+1] = byte(q), byte(uint16(q)>>8)
			}
		}
	}

	kept := s.voices[:0]
	for _, v := range s.voices {
		if v.pos < len(v.pcm) {
			kept = append(kept, v)
		} else if runtimeDiagnosticsEnabled() {
			fmt.Printf("SPEAKER_VOICE_END transport=USB id=%d kind=collision event=%q frames=%d\n", v.id, v.eventPath, len(v.pcm))
		}
	}
	s.voices = kept
	if runtimeDiagnosticsEnabled() {
		for _, v := range started {
			fmt.Printf("SPEAKER_USB_RENDER id=%d kind=collision event=%q channel=1 frames=%d sampleRate=48000\n", v.id, v.eventPath, frames)
		}
	}
	return wrote
}

// mixBluetoothSourceTickLocked mirrors the USB voice mixer before transport
// encoding: BeamNG one-shots overlap in PCM time, then one mixed source tick is
// encoded into the next 0x36 speaker packet. No collision FIFO exists here.
func (s *controllerSpeakerEngine) mixBluetoothSourceTickLocked(dst []float32) bool {
	if len(dst) == 0 || len(s.voices) == 0 {
		return false
	}
	wrote := false
	for f := range dst {
		active := 0
		sum := 0.0
		for i := range s.voices {
			v := &s.voices[i]
			if v.pos >= len(v.pcm) {
				continue
			}
			sample := float64(v.pcm[v.pos])
			if math.IsNaN(sample) || math.IsInf(sample, 0) {
				sample = 0
			}
			sum += sample
			v.pos++
			active++
		}
		if active == 0 {
			break
		}
		if active > 1 {
			sum /= math.Sqrt(float64(active))
		}
		dst[f] = float32(clamp(sum, -0.99, 0.99))
		if math.Abs(float64(dst[f])) > .000001 {
			wrote = true
		}
	}

	kept := s.voices[:0]
	for _, v := range s.voices {
		if v.pos < len(v.pcm) {
			kept = append(kept, v)
		} else if runtimeDiagnosticsEnabled() {
			fmt.Printf("SPEAKER_VOICE_END transport=Bluetooth id=%d kind=collision event=%q frames=%d\n", v.id, v.eventPath, len(v.pcm))
		}
	}
	s.voices = kept
	if len(s.voices) == 0 {
		s.currentKind = ""
	}
	return wrote
}

func (s *controllerSpeakerEngine) nextBluetoothFrame() []byte {
	cfg := currentSpeakerSettings()
	if !cfg.enabled || cfg.volume <= 0 || cfg.categories&speakerCategoryCollision == 0 {
		return nil
	}

	tick := make([]float32, btSpeakerSourceTickMono)
	s.mu.Lock()
	voiceCount := len(s.voices)
	if voiceCount == 0 {
		s.mu.Unlock()
		return speakerOpusSilence[:]
	}
	_ = s.mixBluetoothSourceTickLocked(tick)
	remainingVoices := len(s.voices)
	s.mu.Unlock()

	s.btEncoderMu.Lock()
	enc := s.btEncoder
	backend := s.btEncoderBackend
	if enc == nil {
		s.btEncoderMu.Unlock()
		return speakerOpusSilence[:]
	}
	packet, err := enc.EncodeSourceTick(tick)
	if err != nil {
		enc.Close()
		s.btEncoder = nil
		s.btEncoderBackend = ""
		s.btEncoderMu.Unlock()
		s.flushVehicle(-1, "opus_encoder_error")
		if runtimeDiagnosticsEnabled() {
			fmt.Printf("SPEAKER_BT_STREAM_ERROR backend=%s error=%q\n", backend, err.Error())
		}
		return speakerOpusSilence[:]
	}
	s.btEncoderMu.Unlock()
	if runtimeDiagnosticsEnabled() && voiceCount > 1 {
		fmt.Printf("SPEAKER_BT_MIX voices=%d remaining=%d sourceFrames=%d opusBytes=%d\n", voiceCount, remainingVoices, btSpeakerSourceTickMono, len(packet))
	}
	return packet
}

func (s *controllerSpeakerEngine) diagnosticStatus(transport string) string {
	cfg := currentSpeakerSettings()
	packets := speakerBeamNGSettingsState.packets.Load()
	lastSeen := speakerBeamNGSettingsState.lastSeen.Load()
	settingsAge := "never"
	if lastSeen != 0 {
		age := time.Since(time.Unix(0, lastSeen))
		if age < 0 {
			age = 0
		}
		settingsAge = fmt.Sprintf("%dms", age.Milliseconds())
	}
	if s == nil {
		return fmt.Sprintf("SPEAKER_STATUS transport=%s profile=collision-only enabled=%t volume=%d%% collision=%t output=%s settingsRx=%d settingsAge=%s engine=nil",
			transport, cfg.enabled, cfg.volume, cfg.categories&speakerCategoryCollision != 0, speakerOutputName(cfg.output), packets, settingsAge)
	}

	s.mu.Lock()
	remaining := 0
	active := "none"
	voiceCount := len(s.voices)
	for i := range s.voices {
		if active == "none" {
			active = "collision"
		}
		r := len(s.voices[i].pcm) - s.voices[i].pos
		if r > remaining {
			remaining = r
		}
	}
	seen, played, filtered := s.eventsSeen, s.eventsPlayed, s.eventsFiltered
	lastStrength := s.lastStrength
	lastEventAt := s.lastEventAt
	s.mu.Unlock()
	lastAge := "never"
	if !lastEventAt.IsZero() {
		lastAge = fmt.Sprintf("%dms", time.Since(lastEventAt).Milliseconds())
	}
	return fmt.Sprintf("SPEAKER_STATUS transport=%s profile=collision-only enabled=%t volume=%d%% collision=%t output=%s settingsRx=%d settingsAge=%s active=%s remaining=%d remainingUnit=pcmSamples voices=%d seen=%d played=%d filtered=%d lastStrength=%.3f lastAge=%s",
		transport, cfg.enabled, cfg.volume, cfg.categories&speakerCategoryCollision != 0, speakerOutputName(cfg.output), packets, settingsAge,
		active, remaining, voiceCount, seen, played, filtered, lastStrength, lastAge)
}
