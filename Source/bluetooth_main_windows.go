//go:build windows && bluetooth

package main

import (
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"
)

const telemetryAddress = "127.0.0.1:6974"

func main() {
	runtime.LockOSThread()
	endRealtime := enableRealtimeScheduling()
	defer endRealtime()
	probe, launcherAuto, testStereo, testExtendedInputs, testMotionInputs, testTouchpadMouse, list, restoreOnly, showProfile, diagnosticStatus, diagnosticTouchpadBinding := false, false, false, false, false, false, false, false, false, false, false
	stopFile := ""
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--probe":
			probe = true
		case "--launcher-auto":
			launcherAuto = true
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
		case "--restore-audio-haptics":
			restoreOnly = true
		case "--list":
			list = true
		case "--show-feel-profile":
			showProfile = true
		case "--stop-file":
			if i+1 < len(args) {
				i++
				stopFile = args[i]
			}
		}
	}
	setRuntimeDiagnosticsEnabled(diagnosticStatus)
	setTouchpadBindingDiagnosticsEnabled(diagnosticTouchpadBinding)
	if diagnosticTouchpadBinding {
		fmt.Println("Touchpad/binding diagnostics: ENABLED (BeamNG ownership + gesture decisions will be logged).")
	}
	if showProfile {
		version, path, hash := feelProfileInfo()
		fmt.Printf("Common Feel Engine|version=%s|path=%s|sha256=%s\n", version, path, hash)
		return
	}
	d, err := findBluetoothDualSense()
	if err != nil {
		fmt.Println("Detection error:", err)
		os.Exit(2)
	}
	if d == nil {
		if !probe && !launcherAuto {
			fmt.Println("No compatible Bluetooth DualSense detected.")
		}
		os.Exit(1)
	}
	defer d.close()
	if probe {
		fmt.Printf("Bluetooth|%s|input=%d|output=%d\n", d.product, d.inputLen, d.outputLen)
		return
	}
	if testExtendedInputs {
		os.Exit(runExtendedInputDiagnostic(d))
	}
	if testMotionInputs {
		os.Exit(runMotionInputDiagnostic(d))
	}
	if testTouchpadMouse {
		os.Exit(runTouchpadMouseDiagnostic(d))
	}
	ensureUserSettings()
	if list {
		fmt.Printf("%s — input %d / output Windows %d\n", d.product, d.inputLen, d.outputLen)
		return
	}

	// Reproduce the public Gamepad-Core Windows initialization and then select
	// audio haptics with the exact 78-byte control report.
	var startupControlSeq byte
	if err := initializeGamepadCoreBluetooth(d, &startupControlSeq); err != nil {
		fmt.Println("Unable to initialize the Bluetooth transport:", err)
		os.Exit(3)
	}
	if restoreOnly {
		fmt.Println("Bluetooth audio-haptics mode restored.")
		return
	}
	speaker := newControllerSpeakerEngine()
	speaker.enableNativeBluetooth()
	defer speaker.Close()
	unregisterExactSpeaker := registerExactBeamNGSpeaker(speaker)
	defer unregisterExactSpeaker()
	lastSpeakerGeneration := uint32(0xFFFFFFFF)

	fmt.Println("Enhanced PS5 DualSense Bridge", bridgeDisplayVersion)
	fmt.Printf("Controller: %s (Bluetooth)\n", d.product)
	if diagnosticStatus {
		profileVersion, profilePath, profileHash := feelProfileInfo()
		fmt.Printf("DIAG transport=Bluetooth output=%d feature=%d profile=%s path=%q sha256=%.12s\n", d.outputLen, d.featureLen, profileVersion, profilePath, profileHash)
		fmt.Println("DIAG runtime status enabled; RGB remains owned by the BeamNG mod and Bluetooth output uses the single 0x36 writer.")
	}
	if testStereo {
		fmt.Println("Bluetooth stereo hardware test: left, right, center.")
		runHardwareTest(d)
		return
	}

	conn, err := net.ListenUDP("udp", mustUDPAddr(telemetryAddress))
	if err != nil {
		fmt.Println("Unable to open BeamNG telemetry port:", err)
		os.Exit(2)
	}
	defer conn.Close()
	fmt.Println("Waiting for BeamNG.drive mod...")

	mixer := newCanonicalHapticMixer()
	done := make(chan struct{})
	startExtendedInputStream(d, done)
	var stopOnce sync.Once
	requestStop := func() {
		stopOnce.Do(func() {
			close(done)
			_ = conn.Close()
		})
	}
	sig := make(chan os.Signal, 2)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() { <-sig; requestStop() }()
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
						fmt.Println("Diagnostic stop requested.")
						requestStop()
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

	frames := hapticFramesPerReport36
	interval := time.Second * time.Duration(frames) / bluetoothHapticSampleRate
	controlSeq, hapticSeq, audioCounter := startupControlSeq, byte(0), byte(0)
	lastStatus := time.Time{}
	feelEngine := newSharedFeelEngine(mixer)
	pcmStream := newCanonicalPCMStream()
	writeErrors, writes := 0, 0
	controllerIssueShown := false
	userStatus := newRuntimeUserStatus(time.Now())
	firstHeaderLogged := false

	// Runtime timing metrics. A successful WriteFile only confirms that Windows
	// accepted the buffer; these counters reveal missed haptic deadlines.
	var hapticWriteTotal time.Duration
	var hapticWriteMax time.Duration
	lateTicks, severeLateTicks, nonSilentReports, silentReports := 0, 0, 0, 0
	var maxLateness time.Duration
	statusBaseWrites := 0
	statusBaseLate, statusBaseSevere := 0, 0
	lastIdleGeneration := ^uint32(0)
	nextIdleCheck := time.Time{}
	nextIdleDiagnostic := time.Time{}
	seedBluetoothIdleBaseline(time.Now())

	deadline := time.Now().Add(interval)
	for {
		if !waitUntil(deadline, done) {
			neutral := telemetry{Version: protocolVersion, Active: false}
			for i := 0; i < 6; i++ {
				report := buildBluetoothAllInOneReport36(hapticSeq, audioCounter, make([]int8, 64), buildBluetoothSetStateData63ExternalRGB(neutral))
				_ = d.writeReportExact(report)
				hapticSeq = (hapticSeq + 1) & 0x0F
				audioCounter++
				time.Sleep(interval)
			}
			if stopFile != "" {
				_ = os.Remove(stopFile)
			}
			return
		}
		now := time.Now()
		idleGeneration := bluetoothIdleGeneration()
		if idleGeneration != lastIdleGeneration {
			minutes := bluetoothIdleTimeoutMinutes()
			if runtimeDiagnosticsEnabled() {
				if minutes == 0 {
					fmt.Println("Bluetooth idle power-off: disabled.")
				} else {
					fmt.Printf("Bluetooth idle power-off: %d min without controller input.\n", minutes)
				}
			}
			lastIdleGeneration = idleGeneration
			nextIdleCheck = now
			nextIdleDiagnostic = now
		}
		if nextIdleCheck.IsZero() || !now.Before(nextIdleCheck) {
			nextIdleCheck = now.Add(250 * time.Millisecond)
			minutes := bluetoothIdleTimeoutMinutes()
			lastInput := bluetoothLastInputActivity()
			streamFresh := bluetoothInputStreamFresh(now)
			if minutes > 0 && touchpadBindingDiagnosticsEnabled() && !lastInput.IsZero() && (nextIdleDiagnostic.IsZero() || !now.Before(nextIdleDiagnostic)) {
				timeout := time.Duration(minutes) * time.Minute
				age := now.Sub(lastInput)
				remaining := timeout - age
				if remaining < 0 {
					remaining = 0
				}
				seconds := int((remaining + time.Second - 1) / time.Second)
				fmt.Printf("Bluetooth idle: %ds remaining (last=%s inputAge=%ds streamFresh=%t).\n", seconds, bluetoothIdleLastActivityReason(), int(age/time.Second), streamFresh)
				nextIdleDiagnostic = now.Add(5 * time.Second)
			}
			if minutes > 0 && streamFresh && !lastInput.IsZero() && now.Sub(lastInput) >= time.Duration(minutes)*time.Minute {
				fmt.Printf("Bluetooth idle timeout reached (%d min); powering off controller.\n", minutes)
				if err := disconnectDualSenseBluetoothViaWindows(d); err != nil {
					fmt.Println("Unable to power off Bluetooth controller:", err)
					nextIdleCheck = now.Add(5 * time.Second) // retry backoff, not fake user activity
				} else {
					fmt.Println("Bluetooth controller powered off after idle timeout.")
					requestStop()
					return
				}
			}
		}
		lateness := now.Sub(deadline)
		if lateness > 1500*time.Microsecond {
			lateTicks++
			if lateness > maxLateness {
				maxLateness = lateness
			}
		}
		if lateness > interval {
			severeLateTicks++
		}
		// Keep an absolute schedule. If Windows paused us for several periods,
		// resume from the current time instead of sending a burst of stale frames.
		deadline = deadline.Add(interval)
		if now.Sub(deadline) > interval {
			deadline = now.Add(interval)
		}

		latest, lastPacket := mixer.snapshot()
		userStatus.tick(lastPacket, now, diagnosticStatus)
		canonicalFrames := canonicalFramesForBluetoothFrames(frames)
		frame := feelEngine.step(latest, lastPacket, now, canonicalFrames)
		controlState := frame.Control
		rgb := frame.RGB
		canonicalSamples := frame.Samples
		status := frame.Status
		samples := pcmStream.processBluetooth(canonicalSamples)

		if countNonSilent(samples) > 0 {
			nonSilentReports++
		} else {
			silentReports++
		}
		gen := speakerSettingsGeneration()
		if gen != lastSpeakerGeneration {
			cfg := currentSpeakerSettings()
			setupSeq := controlSeq
			setup := buildBluetoothSpeakerSetupReport(controlSeq, cfg.volume, cfg.enabled)
			if err := d.writeReportExact(setup); err == nil {
				controlSeq = (controlSeq + 1) & 0x0F
				lastSpeakerGeneration = gen
				if diagnosticStatus {
					fmt.Printf("SPEAKER_BT_SETUP seq=%d enabled=%t volume=%d%% result=ok\n", setupSeq, cfg.enabled, cfg.volume)
				}
			} else if diagnosticStatus {
				fmt.Printf("SPEAKER_BT_SETUP seq=%d enabled=%t volume=%d%% result=error err=%v\n", setupSeq, cfg.enabled, cfg.volume, err)
			}
		}
		state63 := buildBluetoothSetStateData63ExternalRGB(controlState)
		report := buildBluetoothAllInOneReport36WithSpeaker(hapticSeq, audioCounter, samples, state63, speaker.nextBluetoothFrame())
		if diagnosticStatus && !firstHeaderLogged {
			headerLen := 16
			if len(report) < headerLen {
				headerLen = len(report)
			}
			fmt.Println("First haptic header:", hex.EncodeToString(report[:headerLen]))
			firstHeaderLogged = true
		}
		hapticSeq = (hapticSeq + 1) & 0x0F
		audioCounter++
		writeStarted := time.Now()
		err := d.writeReportExact(report)
		writeDuration := time.Since(writeStarted)
		hapticWriteTotal += writeDuration
		if writeDuration > hapticWriteMax {
			hapticWriteMax = writeDuration
		}
		if err != nil {
			writeErrors++
			if diagnosticStatus {
				if writeErrors < 5 || writeErrors%50 == 0 {
					fmt.Println("Bluetooth haptic write:", err)
				}
			} else if !controllerIssueShown {
				fmt.Println("Controller connection issue (Bluetooth). Retrying...")
				controllerIssueShown = true
			}
			if writeErrors >= 12 {
				fmt.Println("Controller disconnected. Bridge stopped.")
				requestStop()
			}
		} else {
			if controllerIssueShown && !diagnosticStatus {
				fmt.Println("Controller connection restored (Bluetooth).")
			}
			controllerIssueShown = false
			writes++
			writeErrors = 0
		}

		if diagnosticStatus && (lastStatus.IsZero() || now.Sub(lastStatus) >= time.Second) {
			state := "packets sent"
			if status.stale {
				state = "waiting for BeamNG"
			}
			dw := writes - statusBaseWrites
			dl := lateTicks - statusBaseLate
			ds := severeLateTicks - statusBaseSevere
			avgHapticUS := 0.0
			if writes > 0 {
				avgHapticUS = float64(hapticWriteTotal.Microseconds()) / float64(writes)
			}
			physicsHook, physicsSamples, physicsL, physicsR := false, 0, 0.0, 0.0
			corrReason := ""
			corrLMS, corrRMS, corrLCue, corrRCue := 999.0, 999.0, 0.0, 0.0
			corrLContact, corrRContact, corrLJolt, corrRJolt := 0.0, 0.0, 0.0, 0.0
			corrLStress, corrRStress, corrLPeak, corrRPeak, corrConfidence := 0.0, 0.0, 0.0, 0.0, 0.0
			corrPending := false
			if latest.Raw != nil {
				physicsHook = latest.Raw.PhysicsHookEnabled
				physicsSamples = latest.Raw.PhysicsSamples
				physicsL, physicsR = latest.Raw.PhysicsImpulseL, latest.Raw.PhysicsImpulseR
				corrReason = latest.Raw.NativeBumpCorrReason
				corrLMS, corrRMS = latest.Raw.NativeBumpCorrLeftMS, latest.Raw.NativeBumpCorrRightMS
				corrLCue, corrRCue = latest.Raw.NativeBumpCorrLeftCue, latest.Raw.NativeBumpCorrRightCue
				corrLContact, corrRContact = latest.Raw.NativeBumpCorrLeftContact, latest.Raw.NativeBumpCorrRightContact
				corrLJolt, corrRJolt = latest.Raw.NativeBumpCorrLeftJolt, latest.Raw.NativeBumpCorrRightJolt
				corrLStress, corrRStress = latest.Raw.NativeBumpCorrLeftStress, latest.Raw.NativeBumpCorrRightStress
				corrLPeak, corrRPeak = latest.Raw.NativeBumpCorrLeftPeak, latest.Raw.NativeBumpCorrRightPeak
				corrConfidence = latest.Raw.NativeBumpSourceConfidence
				corrPending = latest.Raw.NativeBumpPending
			}
			settings := currentUserSettings()
			fmt.Printf("%s all=%d(+%d/s) late=%d severe=%d max=%.2fms write=%.0fus max=%.2fms — LED=%s RGB=%02X%02X%02X L2=%d[%d>%d a%d] R2=%d[%d>%d a%d] gain H/S/I=%d/%d/%d ABS=%d L:%s %.2f R:%s %.2f slip %.2f/%.2f phys=%t/%d %.2f/%.2f corr=%s %.0fms/%.2f %.0fms/%.2f src=%.2f c=%.2f/%.2f j=%.2f/%.2f s=%.2f/%.2f p=%.2f/%.2f pending=%t nz=%d voices=%d rms=%.2f peak=%.2f\n",
				state, writes, dw, dl, ds, float64(maxLateness)/float64(time.Millisecond),
				avgHapticUS, float64(hapticWriteMax)/float64(time.Millisecond), frame.LEDStatus, rgb[0], rgb[1], rgb[2],
				controlState.L2Mode, controlState.L2StartStrength, controlState.L2EndStrength, controlState.L2Amplitude,
				controlState.R2Mode, controlState.R2StartStrength, controlState.R2EndStrength, controlState.R2Amplitude,
				settings.Haptics.MasterPercent, settings.Haptics.SurfacePercent, settings.Haptics.ImpactPercent, settings.AdaptiveTriggers.ABSPercent,
				status.profileL, status.surfaceL, status.profileR, status.surfaceR, status.slipL, status.slipR, physicsHook, physicsSamples, physicsL, physicsR,
				corrReason, corrLMS, corrLCue, corrRMS, corrRCue,
				corrConfidence, corrLContact, corrRContact, corrLJolt, corrRJolt, corrLStress, corrRStress, corrLPeak, corrRPeak,
				corrPending, status.nonSilent, mixer.voiceCount(), status.blockRMS, status.blockPeak)
			fmt.Println(speaker.diagnosticStatus("Bluetooth"))
			fmt.Println(speaker.nativeDiagnosticStatus("Bluetooth"))
			lastStatus = now
			statusBaseWrites = writes
			statusBaseLate, statusBaseSevere = lateTicks, severeLateTicks
		}
	}
}

func waitUntil(deadline time.Time, done <-chan struct{}) bool {
	for {
		select {
		case <-done:
			return false
		default:
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return true
		}
		if remaining > 1200*time.Microsecond {
			time.Sleep(remaining - 600*time.Microsecond)
			continue
		}
		// Do not yield the locked high-priority thread inside the final sub-ms
		// window. A scheduler yield here can miss an entire 10.67 ms haptic slot.
		// The spin is short and consumes only a small fraction of one core.
	}
}

func initializeGamepadCoreBluetooth(d *device, seq *byte) error {
	// FDualSenseLibrary::Initialize first sends FF/FF, then FF/57. The
	// integration test selects FC/57 before starting Bluetooth audio haptics.
	stages := [][2]byte{{0xFF, 0xFF}, {0xFF, 0x57}, {0xFC, 0x57}}
	for _, stage := range stages {
		report := buildBluetoothInitializationReport(stage[0], stage[1], *seq)
		if err := d.writeReportExact(report); err != nil {
			return err
		}
		*seq = (*seq + 1) & 0x0F
		time.Sleep(12 * time.Millisecond)
	}
	return nil
}

func restoreAudioHapticsMode(d *device, seq *byte) error {
	neutral := telemetry{Version: protocolVersion, Active: false}
	report := buildBluetoothControlReport(*seq, neutral, [3]byte{}, d.outputLen)
	if err := d.writeReport(report); err != nil {
		return err
	}
	*seq = (*seq + 1) & 0x0F
	time.Sleep(12 * time.Millisecond)
	return nil
}

func mustUDPAddr(s string) *net.UDPAddr {
	a, err := net.ResolveUDPAddr("udp", s)
	if err != nil {
		panic(err)
	}
	return a
}

func sendSilence(d *device, outputSeq, audioCounter *byte, count int) {
	frames := hapticFramesPerReport36
	interval := time.Second * time.Duration(frames) / bluetoothHapticSampleRate
	neutral := buildBluetoothSetStateData63ExternalRGB(telemetry{})
	for i := 0; i < count; i++ {
		samples := make([]int8, frames*2)
		report := buildBluetoothAllInOneReport36(*outputSeq, *audioCounter, samples, neutral)
		_ = d.writeReportExact(report)
		*outputSeq = (*outputSeq + 1) & 0x0F
		*audioCounter = *audioCounter + 1
		time.Sleep(interval)
	}
}

func runHardwareTest(d *device) {
	m := newCanonicalHapticMixer()
	pcmStream := newCanonicalPCMStream()
	var seq byte
	counter := byte(0)
	sendSilence(d, &seq, &counter, 12)
	fmt.Println("Left test...")
	playTestVoice(d, m, pcmStream, &seq, &counter, "collision", 1.0, 0, 650)
	time.Sleep(300 * time.Millisecond)
	fmt.Println("Right test...")
	playTestVoice(d, m, pcmStream, &seq, &counter, "collision", 0, 1.0, 650)
	time.Sleep(300 * time.Millisecond)
	fmt.Println("Center test...")
	playTestVoice(d, m, pcmStream, &seq, &counter, "landing", 1.0, 1.0, 650)
	sendSilence(d, &seq, &counter, 10)
	fmt.Println("Test complete.")
}

func playTestVoice(d *device, m *hapticMixer, pcmStream *canonicalPCMStream, seq, counter *byte, profile string, l, r float64, duration int) {
	m.mu.Lock()
	m.voices = append(m.voices, &voice{profile: profile, left: l, right: r, durationSamples: duration * canonicalHapticSampleRate / 1000, noise: 0x12345678})
	m.lastPacket = time.Now()
	m.latest = telemetry{Active: true, Raw: &rawTelemetry{GroundedWheels: 4}}
	m.mu.Unlock()
	frames := hapticFramesPerReport36
	interval := time.Second * time.Duration(frames) / bluetoothHapticSampleRate
	end := time.Now().Add(time.Duration(duration+180) * time.Millisecond)
	neutral := buildBluetoothSetStateData63ExternalRGB(telemetry{})
	for time.Now().Before(end) {
		canonicalFrames := canonicalFramesForBluetoothFrames(frames)
		canonical, _ := m.render(canonicalFrames, time.Now())
		samples := pcmStream.processBluetooth(canonical)
		report := buildBluetoothAllInOneReport36(*seq, *counter, samples, neutral)
		if err := d.writeReportExact(report); err != nil {
			fmt.Println("Test write:", err)
		}
		*seq = (*seq + 1) & 15
		*counter = *counter + 1
		time.Sleep(interval)
	}
}
