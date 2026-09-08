//go:build windows && usb

package main

import (
	"fmt"
	"math"
	"net"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"sync"
	"syscall"
	"time"
)

const usbTelemetryAddress = "127.0.0.1:6972"

// The DualSense audio-haptic endpoint is rendered through WASAPI, but the
// controller also has a HID state path for adaptive triggers and speaker state.
// Keep that path alive independently of trigger changes while BeamNG owns RGB.
const usbHIDKeepAliveInterval = 25 * time.Millisecond

func buildUSBSharedStateReport(d *device, t telemetry, _ [3]byte) []byte {
	length := 48
	if d != nil && d.outputLen > length {
		length = d.outputLen
	}
	report := make([]byte, length)
	report[0] = 0x02
	common := report[1:48]
	// Update only R2/L2 and speaker state. BeamNG owns the lightbar.
	// Never set HAPTICS_SELECT/COMPATIBLE_VIBRATION while WASAPI owns the grips.
	common[0] = 0x0C
	common[1] = 0x00
	cfg := currentSpeakerSettings()
	if cfg.enabled {
		common[0] |= 0xA0 // audio path + speaker volume; never compatible rumble
		common[1] |= 0x80 // AUDIO_CONTROL2
		common[5] = 100   // hardware speaker volume; user volume is applied once in generated PCM
		common[7] = 0x30
		common[37] = 0x02
	}
	common[2], common[3] = 0, 0
	fillTrigger(common[10:21], t.R2Mode, t.R2StartZone, t.R2StartStrength, t.R2EndStrength, t.R2Amplitude, t.R2Hz)
	fillTrigger(common[21:32], t.L2Mode, t.L2StartZone, t.L2StartStrength, t.L2EndStrength, t.L2Amplitude, t.L2Hz)
	common[44], common[45], common[46] = 0, 0, 0
	return report
}

func mustUSBAddr(s string) *net.UDPAddr {
	a, err := net.ResolveUDPAddr("udp", s)
	if err != nil {
		panic(err)
	}
	return a
}

func runUSBSharedStereoTest(audio *hapticAudioEngine, d *device) error {
	if audio == nil {
		return fmt.Errorf("USB haptic audio is unavailable")
	}
	audio.EnableSharedPCM()
	neutral := telemetry{Version: protocolVersion, Active: false}
	lastKeepAlive := time.Time{}
	keepAlive := func() error {
		now := time.Now()
		if lastKeepAlive.IsZero() || now.Sub(lastKeepAlive) >= usbHIDKeepAliveInterval {
			if err := d.writeReport(buildUSBSharedStateReport(d, neutral, [3]byte{})); err != nil {
				return err
			}
			lastKeepAlive = now
		}
		return nil
	}
	mk := func(l, r int8) []int8 {
		out := make([]int8, 64)
		for i := 0; i < len(out); i += 2 {
			out[i], out[i+1] = l, r
		}
		return out
	}
	fmt.Println("Left grip test...")
	for i := 0; i < 85; i++ {
		if err := keepAlive(); err != nil {
			return err
		}
		audio.PushSharedSamples(mk(72, 0), bluetoothHapticSampleRate)
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(300 * time.Millisecond)
	fmt.Println("Right grip test...")
	for i := 0; i < 85; i++ {
		if err := keepAlive(); err != nil {
			return err
		}
		audio.PushSharedSamples(mk(0, 72), bluetoothHapticSampleRate)
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(300 * time.Millisecond)
	fmt.Println("Center test...")
	for i := 0; i < 85; i++ {
		if err := keepAlive(); err != nil {
			return err
		}
		audio.PushSharedSamples(mk(56, 56), bluetoothHapticSampleRate)
		time.Sleep(10 * time.Millisecond)
	}
	fmt.Println("Test complete.")
	return nil
}

func main() {
	runtime.LockOSThread()
	launcherExitCode := 0
	defer func() {
		if launcherExitCode != 0 {
			os.Exit(launcherExitCode)
		}
	}()
	endRealtime := enableRealtimeScheduling()
	defer endRealtime()
	probe, launcherAuto, listAudio, showProfile, testStereo, testExtendedInputs, testMotionInputs, testTouchpadMouse, diagnosticStatus, diagnosticTouchpadBinding := false, false, false, false, false, false, false, false, false, false
	stopFile := ""
	telemetryAddress := usbTelemetryAddress
	preferredAudio := -1
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--probe":
			probe = true
		case "--launcher-auto":
			launcherAuto = true
		case "--list-audio":
			listAudio = true
		case "--show-feel-profile":
			showProfile = true
		case "--test-stereo":
			testStereo = true
		case "--test-extended-inputs":
			testExtendedInputs = true
		case "--test-motion-inputs":
			testMotionInputs = true
		case "--test-touchpad-mouse":
			testTouchpadMouse = true
		case "--diagnostic-status":
			diagnosticStatus = true
		case "--diagnostic-touchpad-binding":
			diagnosticTouchpadBinding = true
			diagnosticStatus = true
		case "--audio-device-index":
			if i+1 < len(args) {
				i++
				if n, err := strconv.Atoi(args[i]); err == nil {
					preferredAudio = n
				}
			}
		case "--stop-file":
			if i+1 < len(args) {
				i++
				stopFile = args[i]
			}
		case "--telemetry-address":
			if i+1 < len(args) {
				i++
				telemetryAddress = args[i]
			}
		}
	}
	setRuntimeDiagnosticsEnabled(diagnosticStatus)
	setTouchpadBindingDiagnosticsEnabled(diagnosticTouchpadBinding)
	if diagnosticTouchpadBinding {
		fmt.Println("Touchpad/binding diagnostics: ENABLED (BeamNG ownership + gesture decisions will be logged).")
	}
	if showProfile {
		v, p, h := feelProfileInfo()
		fmt.Printf("Common Feel Engine|version=%s|path=%s|sha256=%s\n", v, p, h)
		return
	}
	if listAudio {
		for _, line := range audioDeviceDiagnostics() {
			fmt.Println(line)
		}
		return
	}
	d, err := findUSBDualSense()
	if err != nil {
		printRuntimeError("Unable to detect the DualSense controller.", err)
		os.Exit(2)
	}
	if d == nil {
		if !probe && !launcherAuto {
			fmt.Println("No compatible USB DualSense detected.")
		}
		os.Exit(1)
	}
	defer d.close()
	if probe {
		fmt.Printf("USB|%s|input=%d|output=%d\n", d.product, d.inputLen, d.outputLen)
		return
	}
	if testExtendedInputs {
		code := runExtendedInputDiagnostic(d)
		if launcherAuto && code == 3 {
			os.Exit(launcherExitControllerDisconnected)
		}
		os.Exit(code)
	}
	if testMotionInputs {
		code := runMotionInputDiagnostic(d)
		if launcherAuto && code == 3 {
			os.Exit(launcherExitControllerDisconnected)
		}
		os.Exit(code)
	}
	if testTouchpadMouse {
		code := runTouchpadMouseDiagnostic(d)
		if launcherAuto && code == 3 {
			os.Exit(launcherExitControllerDisconnected)
		}
		os.Exit(code)
	}

	ensureUserSettings()
	audio, details, err := openHapticAudioEngine(preferredAudio)
	if err != nil || audio == nil {
		// Immediately after an USB reconnect, Windows can expose the HID
		// interface before the GameInput/WASAPI haptic endpoint is ready.
		// Under START_BRIDGE this is a transient discovery state: return to
		// the launcher so it can keep scanning USB and Bluetooth instead of
		// stopping the user-facing terminal.
		if launcherAuto {
			os.Exit(launcherExitTransportNotReady)
		}
		printRuntimeError("Unable to initialize USB haptics.", err)
		if diagnosticStatus {
			for _, line := range details {
				fmt.Println("DIAG", line)
			}
		}
		os.Exit(3)
	}
	defer audio.Close()
	audio.EnableSharedPCM()
	speaker := newControllerSpeakerEngine()
	speaker.enableNativeUSB()
	defer speaker.Close()
	unregisterExactSpeaker := registerExactBeamNGSpeaker(speaker)
	defer unregisterExactSpeaker()
	audio.speaker = speaker

	fmt.Println("Enhanced PS5 DualSense Bridge", bridgeDisplayVersion, "- USB")
	fmt.Printf("Controller connected: %s\n", d.product)
	if diagnosticStatus {
		actualBufferMS := float64(audio.bufferFrames) * 1000.0 / float64(audio.sampleRate)
		v, profilePath, profileHash := feelProfileInfo()
		fmt.Printf("DIAG WASAPI device=%q format=%q channels=%d/%d buffer=%.1fms requested=%.1fms prime=%.1fms\n", audio.deviceName, audio.formatName, audio.leftChannel, audio.rightChannel, actualBufferMS, audio.requestedBufferMS, float64(audio.primeFrames)*1000.0/float64(audio.sampleRate))
		fmt.Printf("DIAG profile=%s path=%q sha256=%.12s settings=%s\n", v, profilePath, profileHash, userSettingsSummary())
	}

	neutral := telemetry{Version: protocolVersion, Active: false}
	_ = d.writeReport(buildUSBSharedStateReport(d, neutral, [3]byte{}))
	if diagnosticStatus {
		fmt.Printf("DIAG USB HID keep-alive=%dHz\n", int(time.Second/usbHIDKeepAliveInterval))
	}
	if testStereo {
		fmt.Println("USB stereo test: LEDs kept off, HID keep-alive active.")
		if err := runUSBSharedStereoTest(audio, d); err != nil {
			if launcherAuto {
				os.Exit(launcherExitControllerDisconnected)
			}
			printRuntimeError("Controller disconnected.", err)
			os.Exit(3)
		}
		return
	}

	conn, err := net.ListenUDP("udp", mustUSBAddr(telemetryAddress))
	if err != nil {
		printRuntimeError("Unable to connect the Bridge to BeamNG.", err)
		os.Exit(4)
	}
	defer conn.Close()
	if diagnosticStatus {
		fmt.Println("DIAG telemetry UDP", telemetryAddress)
	}
	fmt.Println("Waiting for BeamNG...")

	mixer := newCanonicalHapticMixer()
	engine := newSharedFeelEngine(mixer)
	done := make(chan struct{})
	startExtendedInputStream(d, done)
	stopRequested := make(chan struct{}, 1)
	requestStop := func() {
		select {
		case stopRequested <- struct{}{}:
		default:
		}
	}
	sig := make(chan os.Signal, 2)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		select {
		case <-sig:
		case <-stopRequested:
		}
		close(done)
		_ = conn.Close()
	}()
	if stopFile != "" {
		_ = os.Remove(stopFile)
		go func() {
			ticker := time.NewTicker(100 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-done:
					return
				case <-ticker.C:
					if _, err := os.Stat(stopFile); err == nil {
						select {
						case stopRequested <- struct{}{}:
						default:
						}
						return
					}
				}
			}
		}()
	}
	var beamNGConnectedOnce sync.Once
	compatibilityGuard := &protocolCompatibilityGuard{}
	go func() {
		buf := make([]byte, 65535)
		for {
			n, _, e := conn.ReadFromUDP(buf)
			if e != nil {
				select {
				case <-done:
					return
				default:
					continue
				}
			}
			if compatibilityGuard.handlePacket(buf[:n], diagnosticStatus, requestStop) {
				continue
			}
			if t, ok := decodeTelemetry(buf[:n]); ok {
				beamNGConnectedOnce.Do(func() { fmt.Println(telemetryConnectionSummary(t)) })
				packetNow := time.Now()
				mixer.update(t, packetNow)
			}
		}
	}()

	// Both transports advance the exact same 48-kHz Common Feel block every
	// Bluetooth haptic period. USB then hands that canonical block to WASAPI.
	interval := time.Second * hapticFramesPerReport36 / bluetoothHapticSampleRate
	canonicalFramesPerStep := int(math.Round(float64(canonicalHapticSampleRate) * interval.Seconds()))
	pcmStream := newCanonicalPCMStream()
	deadline := time.Now().Add(interval)
	var lastTrigger triggerSignature
	var lastRGB [3]byte
	stateSynced := false
	lastHIDWrite := time.Now()
	var hidWrites uint64
	var hidKeepAliveWrites uint64
	lastStatus := time.Time{}
	for {
		select {
		case <-done:
			for i := 0; i < 6; i++ {
				audio.PushSharedSamples(make([]int8, canonicalFramesPerStep*2), canonicalHapticSampleRate)
				time.Sleep(interval)
			}
			_ = d.writeReport(buildUSBSharedStateReport(d, neutral, [3]byte{}))
			if stopFile != "" {
				_ = os.Remove(stopFile)
			}
			return
		default:
		}
		now := time.Now()
		if sleep := time.Until(deadline); sleep > 0 {
			time.Sleep(sleep)
		}
		now = time.Now()
		deadline = deadline.Add(interval)
		if now.Sub(deadline) > interval {
			deadline = now.Add(interval)
		}

		latest, lastPacket := mixer.snapshot()
		frame := engine.step(latest, lastPacket, now, canonicalFramesPerStep)
		canonicalSamples := frame.Samples
		// USB consumes the canonical 48-kHz reference directly. Bluetooth-only filtering happens inside the transport adapter.
		usbSamples := pcmStream.processUSB(canonicalSamples)
		audio.PushSharedSamples(usbSamples, canonicalHapticSampleRate)
		trig := signatureForTriggers(frame.Control)
		stateChanged := !stateSynced || trig != lastTrigger || rgbDistance(frame.RGB, lastRGB) >= 8
		keepAliveDue := stateSynced && now.Sub(lastHIDWrite) >= usbHIDKeepAliveInterval
		if stateChanged || keepAliveDue {
			if err := d.writeReport(buildUSBSharedStateReport(d, frame.Control, frame.RGB)); err != nil {
				printRuntimeError("Controller disconnected.", err)
				if launcherAuto {
					launcherExitCode = launcherExitControllerDisconnected
				}
				requestStop()
				return
			} else {
				hidWrites++
				if keepAliveDue && !stateChanged {
					hidKeepAliveWrites++
				}
				lastHIDWrite = now
			}
			lastTrigger, lastRGB, stateSynced = trig, frame.RGB, true
		}
		if diagnosticStatus && (lastStatus.IsZero() || now.Sub(lastStatus) >= time.Second) {
			settings := currentUserSettings()
			fmt.Printf("USB shared - LED=%s L2=%d[%d>%d a%d] R2=%d[%d>%d a%d] gain H/S/I=%d/%d/%d ABS=%d L:%s %.3f R:%s %.3f nz=%d rms=%.3f pic=%.3f queue=%d hid=%d keep=%d age=%dms\n",
				frame.LEDStatus, frame.Control.L2Mode, frame.Control.L2StartStrength, frame.Control.L2EndStrength, frame.Control.L2Amplitude,
				frame.Control.R2Mode, frame.Control.R2StartStrength, frame.Control.R2EndStrength, frame.Control.R2Amplitude,
				settings.Haptics.MasterPercent, settings.Haptics.SurfacePercent, settings.Haptics.ImpactPercent, settings.AdaptiveTriggers.ABSPercent,
				frame.Status.profileL, frame.Status.surfaceL, frame.Status.profileR, frame.Status.surfaceR, frame.Status.nonSilent, frame.Status.blockRMS, frame.Status.blockPeak, audio.sharedPCM.availableSourceFrames(), hidWrites, hidKeepAliveWrites, now.Sub(lastHIDWrite).Milliseconds())
			fmt.Println(speaker.diagnosticStatus("USB"))
			fmt.Println(speaker.nativeDiagnosticStatus("USB"))
			lastStatus = now
		}
	}
}
