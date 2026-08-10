//go:build windows && bluetooth

package main

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"net"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"
)

const bridgeVersion = "V1"
const telemetryAddress = "127.0.0.1:6974"
const ownerBeaconGEAddress = "127.0.0.1:6975"
const ownerBeaconVehicleAddress = "127.0.0.1:6976"

const (
	dualSenseBluetoothLEDConnectionComplete = uint32(10200000)
	dualSenseSensorTicksPerSecond           = 3000000.0
	dualSenseBluetoothLEDFallbackDelay      = 3500 * time.Millisecond
	dualSenseBluetoothLEDSafetyMargin       = 50 * time.Millisecond
)

func bluetoothSensorTimestamp(report []byte) (uint32, bool) {
	// Enhanced Bluetooth input report: byte 0 = 0x31, common state begins
	// at byte 2, and the 32-bit sensor timestamp is offset 27 in that state.
	if len(report) < 33 || report[0] != 0x31 {
		return 0, false
	}
	return binary.LittleEndian.Uint32(report[29:33]), true
}

func bluetoothLEDReleaseDelay(timestamp uint32, valid bool) time.Duration {
	if !valid {
		return dualSenseBluetoothLEDFallbackDelay
	}
	if timestamp >= dualSenseBluetoothLEDConnectionComplete {
		return 0
	}
	remainingTicks := float64(dualSenseBluetoothLEDConnectionComplete - timestamp)
	remaining := time.Duration(remainingTicks / dualSenseSensorTicksPerSecond * float64(time.Second))
	return remaining + dualSenseBluetoothLEDSafetyMargin
}

func scheduleBluetoothLEDRelease(d *device, now time.Time) (time.Time, uint32, bool, error) {
	report, err := d.readReportOnce()
	if err != nil {
		return now.Add(dualSenseBluetoothLEDFallbackDelay), 0, false, err
	}
	timestamp, valid := bluetoothSensorTimestamp(report)
	delay := bluetoothLEDReleaseDelay(timestamp, valid)
	if !valid {
		return now.Add(delay), 0, false, fmt.Errorf("unexpected Bluetooth input report (len=%d id=0x%02X)", len(report), report[0])
	}
	return now.Add(delay), timestamp, true, nil
}

type btProtocol int

const (
	protocol32 btProtocol = 32
	protocol36 btProtocol = 36
	protocol39 btProtocol = 39
)

func (p btProtocol) String() string {
	switch p {
	case protocol36:
		return "0x36 DS5Dongle all-in-one state+haptics"
	case protocol39:
		return "0x39 legacy diagnostic"
	default:
		return "0x32 SAxense haptics-only"
	}
}
func framesForProtocol(p btProtocol) int {
	if p == protocol39 {
		return hapticFramesPerReport39
	}
	if p == protocol36 {
		return hapticFramesPerReport36
	}
	return hapticFramesPerReport32
}
func makeHapticReport(p btProtocol, seq, counter byte, samples []int8, outputLen int) []byte {
	if p == protocol39 {
		return buildBluetoothHapticReport39(seq, counter, samples, outputLen)
	}
	if p == protocol36 {
		return buildBluetoothAllInOneReport36(seq, counter, samples, buildBluetoothSetStateData63(telemetry{}, [3]byte{}))
	}
	return buildBluetoothHapticReport32(seq, counter, samples, outputLen)
}
func advanceCounter(p btProtocol, counter byte) byte { return counter + 1 }

func main() {
	runtime.LockOSThread()
	endRealtime := enableRealtimeScheduling()
	defer endRealtime()
	probe, test, testBumpCarrier, testAll, testControl, list, hapticsOnly, restoreOnly, showProfile, rgbViaBeamNG, rawBumpsOnly := false, false, false, false, false, false, false, false, false, true, false
	protocol := protocol36
	stopFile := ""
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch a {
		case "--probe":
			probe = true
		case "--test":
			test = true
		case "--test-bump-carrier":
			testBumpCarrier = true
		case "--test-all":
			testAll = true
		case "--test-control-interference":
			testControl = true
		case "--haptics-only":
			hapticsOnly = true
		case "--rgb-via-beamng":
			rgbViaBeamNG = true
		case "--rgb-via-bridge":
			rgbViaBeamNG = false
		case "--raw-bumps-only":
			rawBumpsOnly = true
		case "--restore-audio-haptics":
			restoreOnly = true
		case "--list":
			list = true
		case "--show-feel-profile":
			showProfile = true
		case "--protocol-32":
			protocol = protocol32
		case "--protocol-36":
			protocol = protocol36
		case "--protocol-39":
			protocol = protocol39
		case "--stop-file":
			if i+1 < len(args) {
				i++
				stopFile = args[i]
			}
		}
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
		if !probe {
			fmt.Println("No compatible Bluetooth DualSense detected.")
		}
		os.Exit(1)
	}
	defer d.close()
	if probe {
		fmt.Printf("Bluetooth|%s|input=%d|output=%d\n", d.product, d.inputLen, d.outputLen)
		return
	}
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

	fmt.Println("DualSense Bluetooth Spatial Haptics", bridgeVersion)
	fmt.Printf("Connected: %s - Windows HID capacity %d bytes\n", d.product, d.outputLen)
	fmt.Println("Transport: unchanged 0x36 path for L2/R2 + haptics; RGB is owned by BeamNG custom Device.setRGB() by default.")
	profileVersion, profilePath, profileHash := feelProfileInfo()
	if profileHash != "" {
		fmt.Printf("Common Feel Engine: profile %s - %s - SHA-256 %.12s...\n", profileVersion, profilePath, profileHash)
	} else {
		fmt.Printf("Common Feel Engine: profile %s - built-in fallback values.\n", profileVersion)
	}
	transportProfile := feelProfile().Transport
	fmt.Printf("Transport calibration: Bluetooth output gain %.2f (USB reference %.2f).\n", transportProfile.BluetoothOutputGain, transportProfile.USBOutputGain)
	if testAll {
		fmt.Println("Comparative transport test. Close the main bridge before running it.")
		fmt.Println("=== FORMAT 0x32 SAxense ===")
		runHardwareTest(d, protocol32)
		time.Sleep(900 * time.Millisecond)
		fmt.Println("=== CORRECTED DS5Dongle 0x39 FORMAT ===")
		runHardwareTest(d, protocol39)
		return
	}
	if testControl {
		fmt.Println("0x31/0x32 interference test. Close all other bridges.")
		runControlInterferenceTest(d, protocol)
		return
	}
	if test {
		fmt.Println("Tested format:", protocol.String())
		runHardwareTest(d, protocol)
		return
	}
	if testBumpCarrier {
		fmt.Println("Surface-carrier bump test:", protocol.String())
		runBumpCarrierTest(d, protocol)
		return
	}

	fmt.Println("Active transport:", protocol.String())
	if rgbViaBeamNG {
		fmt.Println("Bluetooth RGB: owned by the BeamNG mod through Device.setRGB(); the bridge sends no runtime LED command.")
		fmt.Println("The bridge owns only L2/R2 + 0x36 haptics and publishes no RGB ownership beacons.")
	} else {
		fmt.Println("Single Bluetooth HID owner: one thread serializes 0x31 LED and 0x36 haptic traffic.")
		fmt.Println("Bluetooth LED: RELEASE_LEDS and RGB use only the standard 0x31/78-byte report; the continuous 0x36 stream carries no LED flags.")
	}
	fmt.Println("L2/R2 and PCM remain strictly on the 0x36 path.")
	fmt.Println("No second bridge may write to the controller over Bluetooth.")
	if hapticsOnly {
		fmt.Println("Diagnostic mode: exact 0x32 stream with no L2/R2/LED updates after initialization.")
	}
	conn, err := net.ListenUDP("udp", mustUDPAddr(telemetryAddress))
	if err != nil {
		fmt.Println("UDP:", err)
		os.Exit(2)
	}
	defer conn.Close()
	fmt.Println("Bluetooth telemetry: UDP", telemetryAddress)

	var ownerConnGE, ownerConnVehicle *net.UDPConn
	if !rgbViaBeamNG {
		ownerConnGE, _ = net.DialUDP("udp", nil, mustUDPAddr(ownerBeaconGEAddress))
		ownerConnVehicle, _ = net.DialUDP("udp", nil, mustUDPAddr(ownerBeaconVehicleAddress))
		if ownerConnGE != nil {
			defer ownerConnGE.Close()
			_, _ = ownerConnGE.Write([]byte("DPH_BT_OWNER_ACTIVE"))
		}
		if ownerConnVehicle != nil {
			defer ownerConnVehicle.Close()
			_, _ = ownerConnVehicle.Write([]byte("DPH_BT_OWNER_ACTIVE"))
		}
	}

	mixer := newCanonicalHapticMixer()
	var rawBumps *rawBumpRenderer
	if rawBumpsOnly {
		rawBumps = newRawBumpRenderer()
		fmt.Println("RAW BUMP A/B ACTIVE: suspension_bump only, fixed pulse, no mixer/texture/collision/landing.")
		fmt.Println("BeamNG native vibration must remain DISABLED for this test.")
	}
	done := make(chan struct{})
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
			if t, ok := decodeTelemetry(buf[:n]); ok {
				packetNow := time.Now()
				mixer.update(t, packetNow)
				if rawBumps != nil {
					rawBumps.observe(t, packetNow)
				}
			}
		}
	}()

	frames := framesForProtocol(protocol)
	interval := time.Second * time.Duration(frames) / bluetoothHapticSampleRate
	controlSeq, hapticSeq, audioCounter := startupControlSeq, byte(0), byte(0)
	lastStatus, lastOwnerBeacon := time.Time{}, time.Time{}
	feelEngine := newSharedFeelEngine(mixer)
	pcmStream := newCanonicalPCMStream()
	writeErrors, writes := 0, 0
	firstHeaderLogged := false
	var lastSentRGB [3]byte
	ledSynced := false
	lastLEDWrite := time.Time{}
	ledUpdateCount := 0
	ledReleased := protocol != protocol36 || rgbViaBeamNG
	ledReleaseAt := time.Now()
	ledColorReadyAt := time.Time{}
	if protocol == protocol36 && !rgbViaBeamNG {
		var sensorTimestamp uint32
		var sensorValid bool
		var scheduleErr error
		ledReleaseAt, sensorTimestamp, sensorValid, scheduleErr = scheduleBluetoothLEDRelease(d, time.Now())
		if sensorValid {
			delay := time.Until(ledReleaseAt)
			if delay < 0 {
				delay = 0
			}
			fmt.Printf("Bluetooth LED sequence: sensor timestamp=%d, RELEASE_LEDS in %.0f ms.\n", sensorTimestamp, float64(delay)/float64(time.Millisecond))
		} else {
			fmt.Printf("Bluetooth LED sequence: timestamp unavailable (%v), safe fallback delay %.0f ms.\n", scheduleErr, float64(dualSenseBluetoothLEDFallbackDelay)/float64(time.Millisecond))
		}
	}

	// Runtime timing metrics. A successful WriteFile only confirms that Windows
	// accepted the buffer; these counters reveal missed haptic deadlines.
	var hapticWriteTotal time.Duration
	var hapticWriteMax time.Duration
	lateTicks, severeLateTicks, nonSilentReports, silentReports := 0, 0, 0, 0
	var maxLateness time.Duration
	statusBaseWrites := 0
	statusBaseLate, statusBaseSevere := 0, 0

	deadline := time.Now().Add(interval)
	for {
		if !waitUntil(deadline, done) {
			neutral := telemetry{Version: protocolVersion, Active: false}
			if protocol == protocol36 {
				for i := 0; i < 6; i++ {
					report := buildBluetoothAllInOneReport36(hapticSeq, audioCounter, make([]int8, 64), buildBluetoothSetStateData63WithFlags(neutral, [3]byte{}, false, false))
					_ = d.writeReportExact(report)
					hapticSeq = (hapticSeq + 1) & 0x0F
					audioCounter++
					time.Sleep(interval)
				}
			} else {
				sendSilence(d, protocol, &hapticSeq, &audioCounter, 6)
			}
			if ownerConnGE != nil {
				_, _ = ownerConnGE.Write([]byte("DPH_BT_OWNER_OFF"))
			}
			if ownerConnVehicle != nil {
				_, _ = ownerConnVehicle.Write([]byte("DPH_BT_OWNER_OFF"))
			}
			if stopFile != "" {
				_ = os.Remove(stopFile)
			}
			return
		}
		now := time.Now()
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

		if !rgbViaBeamNG && (lastOwnerBeacon.IsZero() || now.Sub(lastOwnerBeacon) >= 200*time.Millisecond) {
			if ownerConnGE != nil {
				_, _ = ownerConnGE.Write([]byte("DPH_BT_OWNER_ACTIVE"))
			}
			if ownerConnVehicle != nil {
				_, _ = ownerConnVehicle.Write([]byte("DPH_BT_OWNER_ACTIVE"))
			}
			lastOwnerBeacon = now
		}

		latest, lastPacket := mixer.snapshot()
		canonicalFrames := canonicalFramesForBluetoothFrames(frames)
		frame := feelEngine.step(latest, lastPacket, now, canonicalFrames)
		controlState := frame.Control
		rgb := frame.RGB
		canonicalSamples := frame.Samples
		status := frame.Status
		if rawBumps != nil {
			canonicalSamples = rawBumps.render(canonicalFrames, canonicalHapticSampleRate, now)
		}
		samples := pcmStream.process(canonicalSamples).Bluetooth3k

		if countNonSilent(samples) > 0 {
			nonSilentReports++
		} else {
			silentReports++
		}
		var report []byte
		if protocol == protocol36 {
			if !rgbViaBeamNG {
				// The three hardware probes showed that a fixed 0x36 stream is stable,
				// and that a standard 0x31 lightbar command can coexist with that stream.
				// Keep the proven haptic/trigger packet unchanged except that it
				// never owns LED flags. Serialize the short 0x31 immediately before the
				// next 0x36 so there is still only one HID writer.
				if !ledReleased && !now.Before(ledReleaseAt) {
					release := finalizeBluetoothControlReport(buildBluetoothControlBase(controlSeq, 0, 0x08))
					if ledErr := d.writeReportExact(release); ledErr != nil {
						fmt.Println("RELEASE_LEDS 0x31:", ledErr)
					} else {
						controlSeq = (controlSeq + 1) & 0x0F
						ledReleased = true
						ledSynced = false
						ledColorReadyAt = now.Add(30 * time.Millisecond)
					}
				}
				if ledReleased && !now.Before(ledColorReadyAt) &&
					bluetoothLightbarUpdateDue(ledSynced, lastSentRGB, rgb, lastLEDWrite, now, frame.LEDStatus == "blink") {
					color := buildBluetoothControlReportMasked(controlSeq, controlState, rgb, d.outputLen, false, true)
					if ledErr := d.writeReportExact(color); ledErr != nil {
						fmt.Println("RGB 0x31:", ledErr)
					} else {
						controlSeq = (controlSeq + 1) & 0x0F
						lastSentRGB, lastLEDWrite, ledSynced = rgb, now, true
						ledUpdateCount++
					}
				}
			}
			var state63 []byte
			if rgbViaBeamNG {
				state63 = buildBluetoothSetStateData63ExternalRGB(controlState)
			} else {
				state63 = buildBluetoothSetStateData63WithFlags(controlState, rgb, false, false)
			}
			report = buildBluetoothAllInOneReport36(hapticSeq, audioCounter, samples, state63)
		} else {
			report = makeHapticReport(protocol, hapticSeq, audioCounter, samples, d.outputLen)
		}
		if !firstHeaderLogged {
			headerLen := 16
			if len(report) < headerLen {
				headerLen = len(report)
			}
			fmt.Println("First haptic header:", hex.EncodeToString(report[:headerLen]))
			firstHeaderLogged = true
		}
		hapticSeq = (hapticSeq + 1) & 0x0F
		audioCounter = advanceCounter(protocol, audioCounter)
		writeStarted := time.Now()
		var err error
		if protocol == protocol36 {
			err = d.writeReportExact(report)
		} else {
			err = d.writeReport(report)
		}
		writeDuration := time.Since(writeStarted)
		hapticWriteTotal += writeDuration
		if writeDuration > hapticWriteMax {
			hapticWriteMax = writeDuration
		}
		if err != nil {
			writeErrors++
			if writeErrors < 5 || writeErrors%50 == 0 {
				fmt.Println("Bluetooth haptic write:", err)
			}
		} else {
			writes++
			writeErrors = 0
		}

		if lastStatus.IsZero() || now.Sub(lastStatus) >= time.Second {
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
			fmt.Printf("%s all=%d(+%d/s) late=%d severe=%d max=%.2fms write=%.0fus max=%.2fms — LED=%s RGB=%02X%02X%02X ledWrites=%d L2=%d R2=%d L:%s %.2f R:%s %.2f slip %.2f/%.2f phys=%t/%d %.2f/%.2f corr=%s %.0fms/%.2f %.0fms/%.2f src=%.2f c=%.2f/%.2f j=%.2f/%.2f s=%.2f/%.2f p=%.2f/%.2f pending=%t nz=%d voices=%d rms=%.2f peak=%.2f\n",
				state, writes, dw, dl, ds, float64(maxLateness)/float64(time.Millisecond),
				avgHapticUS, float64(hapticWriteMax)/float64(time.Millisecond), frame.LEDStatus, rgb[0], rgb[1], rgb[2], ledUpdateCount, controlState.L2Mode, controlState.R2Mode,
				status.profileL, status.surfaceL, status.profileR, status.surfaceR, status.slipL, status.slipR, physicsHook, physicsSamples, physicsL, physicsR,
				corrReason, corrLMS, corrLCue, corrRMS, corrRCue,
				corrConfidence, corrLContact, corrRContact, corrLJolt, corrRJolt, corrLStress, corrRStress, corrLPeak, corrRPeak,
				corrPending, status.nonSilent, mixer.voiceCount(), status.blockRMS, status.blockPeak)
			if rawBumps != nil {
				rs := rawBumps.stats()
				fmt.Printf("RAW_BUMP status accepted=%d played=%d dropped=%d pending=%d active=%t last=%d/%s delay=%.1fms\n",
					rs.Accepted, rs.Played, rs.Dropped, rs.Pending, rs.Active, rs.Event, rawBumpSideName(rs.Side), float64(rs.QueueDelay)/float64(time.Millisecond))
			}
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

func runControlInterferenceTest(d *device, protocol btProtocol) {
	var hseq, cseq byte
	counter := byte(0)
	sendSilence(d, protocol, &hseq, &counter, 12)
	fmt.Println("Phase A: continuous haptic stream, no 0x31 report...")
	playContinuousTest(d, protocol, &hseq, &counter, nil, 5000)
	time.Sleep(600 * time.Millisecond)
	fmt.Println("Phase B: same 0x31 reports at 20 Hz, always immediately BEFORE 0x32...")
	neutral := telemetry{Version: protocolVersion, Active: true}
	playContinuousTest(d, protocol, &hseq, &counter, func() {
		_ = d.writeReport(buildBluetoothControlReport(cseq, neutral, [3]byte{0, 40, 0}, d.outputLen))
		cseq = (cseq + 1) & 0x0F
	}, 5000)
	sendSilence(d, protocol, &hseq, &counter, 10)
	fmt.Println("Test complete. Compare continuity between phases A and B.")
}

func playContinuousTest(d *device, protocol btProtocol, seq, counter *byte, control func(), durationMS int) {
	frames := framesForProtocol(protocol)
	interval := time.Second * time.Duration(frames) / bluetoothHapticSampleRate
	deadline := time.Now().Add(interval)
	end := time.Now().Add(time.Duration(durationMS) * time.Millisecond)
	lastControl := time.Time{}
	phase := 0.0
	pcmStream := newCanonicalPCMStream()
	neverStop := make(chan struct{})
	for time.Now().Before(end) {
		waitUntil(deadline, neverStop)
		deadline = deadline.Add(interval)
		canonicalFrames := canonicalFramesForBluetoothFrames(frames)
		canonical := make([]int8, canonicalFrames*2)
		for i := 0; i < canonicalFrames; i++ {
			phase += 2 * math.Pi * 92 / canonicalHapticSampleRate
			v := int8(math.Round(math.Sin(phase) * 78))
			canonical[i*2], canonical[i*2+1] = v, v
		}
		if control != nil && (lastControl.IsZero() || time.Since(lastControl) >= 50*time.Millisecond) {
			control()
			lastControl = time.Now()
		}
		samples := pcmStream.process(canonical).Bluetooth3k
		report := makeHapticReport(protocol, *seq, *counter, samples, d.outputLen)
		if protocol == protocol36 {
			_ = d.writeReportExact(report)
		} else {
			_ = d.writeReport(report)
		}
		*seq = (*seq + 1) & 0x0F
		*counter = advanceCounter(protocol, *counter)
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

func sendSilence(d *device, protocol btProtocol, outputSeq, audioCounter *byte, count int) {
	frames := framesForProtocol(protocol)
	interval := time.Second * time.Duration(frames) / bluetoothHapticSampleRate
	for i := 0; i < count; i++ {
		samples := make([]int8, frames*2)
		report := makeHapticReport(protocol, *outputSeq, *audioCounter, samples, d.outputLen)
		if protocol == protocol36 {
			_ = d.writeReportExact(report)
		} else {
			_ = d.writeReport(report)
		}
		*outputSeq = (*outputSeq + 1) & 0x0F
		*audioCounter = advanceCounter(protocol, *audioCounter)
		time.Sleep(interval)
	}
}

func runHardwareTest(d *device, protocol btProtocol) {
	m := newCanonicalHapticMixer()
	pcmStream := newCanonicalPCMStream()
	// Prime the controller with valid silence before the first transient.
	var seq byte
	counter := byte(0)
	sendSilence(d, protocol, &seq, &counter, 12)
	fmt.Println("Left test...")
	playTestVoice(d, m, pcmStream, protocol, &seq, &counter, "collision", 1.0, 0, 650)
	time.Sleep(300 * time.Millisecond)
	fmt.Println("Test droit...")
	playTestVoice(d, m, pcmStream, protocol, &seq, &counter, "collision", 0, 1.0, 650)
	time.Sleep(300 * time.Millisecond)
	fmt.Println("Center test...")
	playTestVoice(d, m, pcmStream, protocol, &seq, &counter, "landing", 1.0, 1.0, 650)
	sendSilence(d, protocol, &seq, &counter, 10)
	fmt.Println("Test complete.")
}

func runBumpCarrierTest(d *device, protocol btProtocol) {
	m := newCanonicalHapticMixer()
	pcmStream := newCanonicalPCMStream()
	var seq byte
	counter := byte(0)
	sendSilence(d, protocol, &seq, &counter, 12)
	fmt.Println("5 strong LEFT bumps, one every 320 ms...")
	for i := 0; i < 5; i++ {
		playTestVoice(d, m, pcmStream, protocol, &seq, &counter, "suspension_bump", 0.98, 0, 110)
		time.Sleep(220 * time.Millisecond)
	}
	time.Sleep(500 * time.Millisecond)
	fmt.Println("5 strong RIGHT bumps, one every 320 ms...")
	for i := 0; i < 5; i++ {
		playTestVoice(d, m, pcmStream, protocol, &seq, &counter, "suspension_bump", 0, 0.98, 110)
		time.Sleep(220 * time.Millisecond)
	}
	time.Sleep(500 * time.Millisecond)
	fmt.Println("3 CENTER bumps with equivalent energy...")
	const center = 0.608 // 0.86 / sqrt(2)
	for i := 0; i < 3; i++ {
		playTestVoice(d, m, pcmStream, protocol, &seq, &counter, "suspension_bump", center, center, 110)
		time.Sleep(220 * time.Millisecond)
	}
	sendSilence(d, protocol, &seq, &counter, 10)
	fmt.Println("Surface-carrier bump test complete.")
}

func playTestVoice(d *device, m *hapticMixer, pcmStream *canonicalPCMStream, protocol btProtocol, seq, counter *byte, profile string, l, r float64, duration int) {
	m.mu.Lock()
	m.voices = append(m.voices, &voice{profile: profile, left: l, right: r, durationSamples: duration * canonicalHapticSampleRate / 1000, noise: 0x12345678})
	m.lastPacket = time.Now()
	m.latest = telemetry{Active: true, Raw: &rawTelemetry{GroundedWheels: 4}}
	m.mu.Unlock()
	frames := framesForProtocol(protocol)
	interval := time.Second * time.Duration(frames) / bluetoothHapticSampleRate
	end := time.Now().Add(time.Duration(duration+180) * time.Millisecond)
	for time.Now().Before(end) {
		canonicalFrames := canonicalFramesForBluetoothFrames(frames)
		canonical, _ := m.render(canonicalFrames, time.Now())
		samples := pcmStream.process(canonical).Bluetooth3k
		report := makeHapticReport(protocol, *seq, *counter, samples, d.outputLen)
		var writeErr error
		if protocol == protocol36 {
			writeErr = d.writeReportExact(report)
		} else {
			writeErr = d.writeReport(report)
		}
		if writeErr != nil {
			fmt.Println("Test write:", writeErr)
		}
		*seq = (*seq + 1) & 15
		*counter = advanceCounter(protocol, *counter)
		time.Sleep(interval)
	}
}
