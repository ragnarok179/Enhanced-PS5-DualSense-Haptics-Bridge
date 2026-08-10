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
	"syscall"
	"time"
)

const usbSharedBridgeVersion = "V1"
const usbTelemetryAddress = "127.0.0.1:6972"

// The DualSense audio-haptic endpoint is rendered through WASAPI, but the
// controller also has a HID state path for triggers/lightbar. Keeping that HID
// path alive independently of LED changes prevents the audio-haptic route from
// becoming perceptually dormant while RGB and triggers stay constant.
const usbHIDKeepAliveInterval = 25 * time.Millisecond

func buildUSBSharedStateReport(d *device, t telemetry, rgb [3]byte) []byte {
	length := 48
	if d != nil && d.outputLen > length {
		length = d.outputLen
	}
	report := make([]byte, length)
	report[0] = 0x02
	common := report[1:48]
	// Same rule as the validated USB HD bridge: update only R2/L2 and lightbar.
	// Never set HAPTICS_SELECT/COMPATIBLE_VIBRATION while WASAPI owns the grips.
	common[0] = 0x0C
	common[1] = 0x04
	common[2], common[3] = 0, 0
	fillTrigger(common[10:21], t.R2Mode, t.R2StartZone, t.R2StartStrength, t.R2EndStrength, t.R2Amplitude, t.R2Hz)
	fillTrigger(common[21:32], t.L2Mode, t.L2StartZone, t.L2StartStrength, t.L2EndStrength, t.L2Amplitude, t.L2Hz)
	common[44], common[45], common[46] = rgb[0], rgb[1], rgb[2]
	return report
}

func mustUSBAddr(s string) *net.UDPAddr {
	a, err := net.ResolveUDPAddr("udp", s)
	if err != nil {
		panic(err)
	}
	return a
}

func runUSBSharedStereoTest(audio *hapticAudioEngine, d *device) {
	if audio == nil {
		return
	}
	audio.EnableSharedPCM()
	neutral := telemetry{Version: protocolVersion, Active: false}
	lastKeepAlive := time.Time{}
	keepAlive := func() {
		now := time.Now()
		if lastKeepAlive.IsZero() || now.Sub(lastKeepAlive) >= usbHIDKeepAliveInterval {
			_ = d.writeReport(buildUSBSharedStateReport(d, neutral, [3]byte{}))
			lastKeepAlive = now
		}
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
		keepAlive()
		audio.PushSharedSamples(mk(72, 0), bluetoothHapticSampleRate)
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(300 * time.Millisecond)
	fmt.Println("Right grip test...")
	for i := 0; i < 85; i++ {
		keepAlive()
		audio.PushSharedSamples(mk(0, 72), bluetoothHapticSampleRate)
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(300 * time.Millisecond)
	fmt.Println("Center test...")
	for i := 0; i < 85; i++ {
		keepAlive()
		audio.PushSharedSamples(mk(56, 56), bluetoothHapticSampleRate)
		time.Sleep(10 * time.Millisecond)
	}
	fmt.Println("Test complete.")
}

func main() {
	runtime.LockOSThread()
	endRealtime := enableRealtimeScheduling()
	defer endRealtime()
	probe, listAudio, showProfile, testStereo, rawBumpsOnly := false, false, false, false, false
	stopFile := ""
	telemetryAddress := usbTelemetryAddress
	preferredAudio := -1
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--probe":
			probe = true
		case "--list-audio":
			listAudio = true
		case "--show-feel-profile":
			showProfile = true
		case "--test-stereo":
			testStereo = true
		case "--raw-bumps-only":
			rawBumpsOnly = true
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
		fmt.Println("USB error:", err)
		os.Exit(2)
	}
	if d == nil {
		if !probe {
			fmt.Println("No compatible USB DualSense detected.")
		}
		os.Exit(1)
	}
	defer d.close()
	if probe {
		fmt.Printf("USB|%s|input=%d|output=%d\n", d.product, d.inputLen, d.outputLen)
		return
	}

	audio, details, err := openHapticAudioEngine(preferredAudio)
	if err != nil || audio == nil {
		fmt.Println("Unable to open the USB WASAPI haptic endpoint:", err)
		for _, line := range details {
			fmt.Println(line)
		}
		os.Exit(3)
	}
	defer audio.Close()
	audio.EnableSharedPCM()

	fmt.Println("Enhanced PS5 DualSense Haptics")
	fmt.Println("Version:", usbSharedBridgeVersion)
	fmt.Printf("Connected: %s - USB HID %d/%d\n", d.product, d.inputLen, d.outputLen)
	fmt.Printf("WASAPI: %s - %s - grip channels %d/%d\n", audio.deviceName, audio.formatName, audio.leftChannel, audio.rightChannel)
	v, p, h := feelProfileInfo()
	fmt.Printf("Common Feel Engine: %s - %s - SHA-256 %.12s...\n", v, p, h)
	transportProfile := feelProfile().Transport
	fmt.Printf("Transport calibration: USB output gain %.2f (Bluetooth %.2f).\n", transportProfile.USBOutputGain, transportProfile.BluetoothOutputGain)
	fmt.Println("Single 48 kHz Common Feel stream: USB direct; Bluetooth derived at 3 kHz. HID L2/R2/LED state is independent from PCM.")

	neutral := telemetry{Version: protocolVersion, Active: false}
	_ = d.writeReport(buildUSBSharedStateReport(d, neutral, [3]byte{}))
	fmt.Printf("USB HID keep-alive: %d Hz, independent from LEDs/triggers.\n", int(time.Second/usbHIDKeepAliveInterval))
	if testStereo {
		fmt.Println("USB stereo test: LEDs kept off, HID keep-alive active.")
		runUSBSharedStereoTest(audio, d)
		return
	}

	conn, err := net.ListenUDP("udp", mustUSBAddr(telemetryAddress))
	if err != nil {
		fmt.Println("UDP:", err)
		os.Exit(4)
	}
	defer conn.Close()
	fmt.Println("USB telemetry: UDP", telemetryAddress)

	mixer := newCanonicalHapticMixer()
	engine := newSharedFeelEngine(mixer)
	var rawBumps *rawBumpRenderer
	if rawBumpsOnly {
		rawBumps = newRawBumpRenderer()
		fmt.Println("RAW BUMP A/B ACTIVE: suspension_bump only, fixed pulse, no mixer/texture/collision/landing.")
		fmt.Println("BeamNG native vibration must remain DISABLED for this test.")
	}
	done := make(chan struct{})
	stopRequested := make(chan struct{}, 1)
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

	// Both transports advance the exact same 48-kHz Common Feel block every
	// Bluetooth haptic period. USB then hands that canonical block to WASAPI.
	interval := time.Second * hapticFramesPerReport32 / bluetoothHapticSampleRate
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
		if rawBumps != nil {
			canonicalSamples = rawBumps.render(canonicalFramesPerStep, canonicalHapticSampleRate, now)
		}
		// USB consumes the canonical 48-kHz reference directly. Bluetooth-only filtering happens inside the transport adapter.
		usbSamples := pcmStream.process(canonicalSamples).USB48k
		audio.PushSharedSamples(usbSamples, canonicalHapticSampleRate)
		trig := signatureForTriggers(frame.Control)
		stateChanged := !stateSynced || trig != lastTrigger || rgbDistance(frame.RGB, lastRGB) >= 8
		keepAliveDue := stateSynced && now.Sub(lastHIDWrite) >= usbHIDKeepAliveInterval
		if stateChanged || keepAliveDue {
			if err := d.writeReport(buildUSBSharedStateReport(d, frame.Control, frame.RGB)); err != nil {
				fmt.Println("HID USB:", err)
			} else {
				hidWrites++
				if keepAliveDue && !stateChanged {
					hidKeepAliveWrites++
				}
				lastHIDWrite = now
			}
			lastTrigger, lastRGB, stateSynced = trig, frame.RGB, true
		}
		if lastStatus.IsZero() || now.Sub(lastStatus) >= time.Second {
			fmt.Printf("USB shared - LED=%s L2=%d R2=%d L:%s %.3f R:%s %.3f nz=%d rms=%.3f pic=%.3f queue=%d hid=%d keep=%d age=%dms\n",
				frame.LEDStatus, frame.Control.L2Mode, frame.Control.R2Mode, frame.Status.profileL, frame.Status.surfaceL, frame.Status.profileR, frame.Status.surfaceR, frame.Status.nonSilent, frame.Status.blockRMS, frame.Status.blockPeak, audio.sharedPCM.availableSourceFrames(), hidWrites, hidKeepAliveWrites, now.Sub(lastHIDWrite).Milliseconds())
			if rawBumps != nil {
				rs := rawBumps.stats()
				fmt.Printf("RAW_BUMP status accepted=%d played=%d dropped=%d pending=%d active=%t last=%d/%s delay=%.1fms\n",
					rs.Accepted, rs.Played, rs.Dropped, rs.Pending, rs.Active, rs.Event, rawBumpSideName(rs.Side), float64(rs.QueueDelay)/float64(time.Millisecond))
			}
			lastStatus = now
		}
	}
}
