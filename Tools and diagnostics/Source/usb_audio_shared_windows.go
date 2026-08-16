//go:build windows && usb

package main

import (
	"errors"
	"fmt"
	"math"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

const (
	coinitMultithreaded = 0x0
	clsctxAll           = 0x17

	eRender           = 0
	deviceStateActive = 0x1
	deviceStateAll    = 0xF
	stgmRead          = 0

	audclntSharemodeShared      = 0
	audclntStreamflagsNoPersist = 0x00080000
	audclntBufferflagsSilent    = 0x2

	waveFormatPCM           = 0x0001
	waveFormatIEEEFloat     = 0x0003
	waveFormatExtensibleTag = 0xFFFE

	gameInputKindController        = 0x0000000E
	gameInputKindGamepad           = 0x00040000
	gameInputDeviceConnected       = 0x00000001
	gameInputDeviceHapticInfoReady = 0x00200000
	gameInputBlockingEnumeration   = 2
)

var (
	modOle32             = syscall.NewLazyDLL("ole32.dll")
	procCoInitializeEx   = modOle32.NewProc("CoInitializeEx")
	procCoUninitialize   = modOle32.NewProc("CoUninitialize")
	procCoCreateInstance = modOle32.NewProc("CoCreateInstance")
	procCoTaskMemFree    = modOle32.NewProc("CoTaskMemFree")
	procPropVariantClear = modOle32.NewProc("PropVariantClear")

	modGameInput            = syscall.NewLazyDLL("gameinput.dll")
	procGameInputInitialize = modGameInput.NewProc("GameInputInitialize")
	procGameInputCreate     = modGameInput.NewProc("GameInputCreate")
)

var (
	clsidMMDeviceEnumerator = guid{0xBCDE0395, 0xE52F, 0x467C, [8]byte{0x8E, 0x3D, 0xC4, 0x57, 0x92, 0x91, 0x69, 0x2E}}
	iidIMMDeviceEnumerator  = guid{0xA95664D2, 0x9614, 0x4F35, [8]byte{0xA7, 0x46, 0xDE, 0x8D, 0xB6, 0x36, 0x17, 0xE6}}
	iidIAudioClient         = guid{0x1CB9AD4C, 0xDBFA, 0x4C32, [8]byte{0xB1, 0x78, 0xC2, 0xF5, 0x68, 0xA7, 0x03, 0xB2}}
	iidIAudioRenderClient   = guid{0xF294ACFC, 0x3146, 0x4483, [8]byte{0xA7, 0xBF, 0xAD, 0xDC, 0xA7, 0xC2, 0x60, 0xE2}}
	iidIGameInput           = guid{0x20EFC1C7, 0x5D9A, 0x43BA, [8]byte{0xB2, 0x6F, 0xB8, 0x07, 0xFA, 0x48, 0x60, 0x9C}}

	pkeyDeviceFriendlyName = propertyKey{guid{0xA45C254E, 0xDF1C, 0x4EFD, [8]byte{0x80, 0x20, 0x67, 0xD1, 0x46, 0xA8, 0x50, 0xE0}}, 14}

	hapticGripLeft  = guid{0x08C707C2, 0x66BB, 0x406C, [8]byte{0xA8, 0x4A, 0xDF, 0xE0, 0x85, 0x12, 0x0A, 0x92}}
	hapticGripRight = guid{0x155A0B77, 0x8BB2, 0x40DB, [8]byte{0x86, 0x90, 0xB6, 0xD4, 0x11, 0x26, 0xDF, 0xC1}}
)

type propertyKey struct {
	Fmtid guid
	Pid   uint32
}

type propVariant struct {
	VT    uint16
	R1    uint16
	R2    uint16
	R3    uint16
	Val   uintptr
	Extra [8]byte
}

type waveFormatEx struct {
	FormatTag      uint16
	Channels       uint16
	SamplesPerSec  uint32
	AvgBytesPerSec uint32
	BlockAlign     uint16
	BitsPerSample  uint16
	CbSize         uint16
}

type waveFormatExtensible struct {
	Format      waveFormatEx
	ValidBits   uint16
	ChannelMask uint32
	SubFormat   guid
}

type gameInputHapticInfo struct {
	AudioEndpointID [256]uint16
	LocationCount   uint32
	Locations       [8]guid
}

type gameInputEndpoint struct {
	ID           string
	LeftChannel  int
	RightChannel int
	Details      []string
}

type wasapiCandidate struct {
	Index      int
	ID         string
	Name       string
	State      uint32
	Channels   int
	SampleRate int
	Bits       int
	Float      bool
	Score      int
}

type hapticAudioEngine struct {
	mu                sync.Mutex
	enumerator        uintptr
	device            uintptr
	audioClient       uintptr
	renderClient      uintptr
	endpointID        string
	deviceName        string
	formatName        string
	channels          int
	sampleRate        int
	bits              int
	blockAlign        int
	floatPCM          bool
	leftChannel       int
	rightChannel      int
	bufferFrames      uint32
	requestedBufferMS float64
	defaultPeriodMS   float64
	primeFrames       uint32
	closed            bool
	renderTicks       uint64
	releasedBuffers   uint64
	releasedFrames    uint64
	nonSilentBuffers  uint64
	nonSilentFrames   uint64
	silentFrames      uint64
	paddingErrors     uint64
	getBufferErrors   uint64
	releaseErrors     uint64
	lastHRESULT       string
	stop              chan struct{}
	done              chan struct{}
	comInitialized    bool

	sharedMode   bool
	sharedPCM    sharedPCMQueue
	scratchLeft  []float64
	scratchRight []float64
}

func hresultFailed(v uintptr) bool { return int32(uint32(v)) < 0 }
func hresultText(v uintptr) string { return fmt.Sprintf("0x%08X", uint32(v)) }

func comMethod(obj uintptr, index int) uintptr {
	if obj == 0 {
		return 0
	}
	vtbl := *(*uintptr)(unsafe.Pointer(obj))
	return *(*uintptr)(unsafe.Pointer(vtbl + uintptr(index)*unsafe.Sizeof(uintptr(0))))
}

func comCall(obj uintptr, index int, args ...uintptr) (uintptr, uintptr, syscall.Errno) {
	fn := comMethod(obj, index)
	all := make([]uintptr, 0, len(args)+1)
	all = append(all, obj)
	all = append(all, args...)
	return syscall.SyscallN(fn, all...)
}

func comRelease(obj uintptr) {
	if obj != 0 {
		comCall(obj, 2)
	}
}

func utf16PtrString(p uintptr) string {
	if p == 0 {
		return ""
	}
	a := make([]uint16, 0, 256)
	for i := uintptr(0); i < 4096; i += 2 {
		v := *(*uint16)(unsafe.Pointer(p + i))
		if v == 0 {
			break
		}
		a = append(a, v)
	}
	return syscall.UTF16ToString(a)
}

func utf16ArrayString(a []uint16) string {
	n := 0
	for n < len(a) && a[n] != 0 {
		n++
	}
	return syscall.UTF16ToString(a[:n])
}

func guidEqual(a, b guid) bool { return a == b }

var gameInputCollectMu sync.Mutex
var gameInputCollect *gameInputEndpoint

func gameInputDeviceCallback(token, context, device, timestamp, currentStatus, previousStatus uintptr) uintptr {
	if device == 0 {
		return 0
	}
	var infoPtr uintptr
	hr, _, _ := comCall(device, 3, uintptr(unsafe.Pointer(&infoPtr)))
	if hresultFailed(hr) || infoPtr == 0 {
		return 0
	}
	vendor := *(*uint16)(unsafe.Pointer(infoPtr))
	product := *(*uint16)(unsafe.Pointer(infoPtr + 2))
	if vendor != 0x054C {
		return 0
	}
	var hi gameInputHapticInfo
	hr, _, _ = comCall(device, 4, uintptr(unsafe.Pointer(&hi)))
	gameInputCollectMu.Lock()
	defer gameInputCollectMu.Unlock()
	if gameInputCollect == nil {
		gameInputCollect = &gameInputEndpoint{}
	}
	gameInputCollect.Details = append(gameInputCollect.Details,
		fmt.Sprintf("GameInput Sony VID=054C PID=%04X status=0x%08X GetHapticInfo=%s", product, uint32(currentStatus), hresultText(hr)))
	if hresultFailed(hr) {
		return 0
	}
	id := strings.TrimSpace(utf16ArrayString(hi.AudioEndpointID[:]))
	if id == "" || hi.LocationCount == 0 {
		return 0
	}
	left, right := -1, -1
	count := int(hi.LocationCount)
	if count > len(hi.Locations) {
		count = len(hi.Locations)
	}
	for i := 0; i < count; i++ {
		if guidEqual(hi.Locations[i], hapticGripLeft) {
			left = i
		}
		if guidEqual(hi.Locations[i], hapticGripRight) {
			right = i
		}
	}
	if left < 0 && count >= 4 {
		left = 2
	}
	if right < 0 && count >= 4 {
		right = 3
	}
	gameInputCollect.ID = id
	gameInputCollect.LeftChannel = left
	gameInputCollect.RightChannel = right
	gameInputCollect.Details = append(gameInputCollect.Details,
		fmt.Sprintf("GameInput endpoint=%s channels-map left=%d right=%d count=%d", id, left, right, count))
	return 0
}

func discoverGameInputEndpoint(wait time.Duration) (*gameInputEndpoint, []string, error) {
	details := []string{"GameInput v3: searching for DualSense and waiting for HapticInfoReady"}
	if err := modGameInput.Load(); err != nil {
		return nil, append(details, "gameinput.dll introuvable"), fmt.Errorf("GameInput unavailable: %w", err)
	}
	if err := procGameInputInitialize.Find(); err != nil {
		// GameInputInitialize was added with the v3 API. Older v0-v2 runtimes
		// legitimately export only GameInputCreate. The WASAPI fallback below is
		// sufficient for DualSense audio haptics and avoids calling an interface
		// with an unknown ABI version through a hand-written vtable.
		if createErr := procGameInputCreate.Find(); createErr == nil {
			details = append(details, "GameInputCreate exported (API v0-v2); direct WASAPI identification used")
			return nil, details, errors.New("GameInput API v3 indisponible ; repli WASAPI direct")
		}
		return nil, append(details, "no GameInput factory exported"), fmt.Errorf("GameInput unavailable: %w", err)
	}
	var gi uintptr
	hr, _, _ := procGameInputInitialize.Call(uintptr(unsafe.Pointer(&iidIGameInput)), uintptr(unsafe.Pointer(&gi)))
	if hresultFailed(hr) || gi == 0 {
		return nil, append(details, "GameInputInitialize="+hresultText(hr)), fmt.Errorf("GameInputInitialize failed: %s", hresultText(hr))
	}
	defer comRelease(gi)

	gameInputCollectMu.Lock()
	gameInputCollect = &gameInputEndpoint{}
	gameInputCollectMu.Unlock()

	cb := syscall.NewCallback(gameInputDeviceCallback)
	var token uint64
	filter := uintptr(gameInputDeviceConnected | gameInputDeviceHapticInfoReady)
	kind := uintptr(gameInputKindGamepad)
	hr, _, _ = comCall(gi, 8, 0, kind, filter, gameInputBlockingEnumeration, 0, cb, uintptr(unsafe.Pointer(&token)))
	if hresultFailed(hr) {
		return nil, append(details, "RegisterDeviceCallback="+hresultText(hr)), fmt.Errorf("RegisterDeviceCallback failed: %s", hresultText(hr))
	}

	deadline := time.Now().Add(wait)
	for time.Now().Before(deadline) {
		gameInputCollectMu.Lock()
		found := gameInputCollect != nil && gameInputCollect.ID != ""
		gameInputCollectMu.Unlock()
		if found {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	// StopCallback then UnregisterCallback (methods 11 and 12 in IGameInput v3).
	comCall(gi, 11, uintptr(token))
	comCall(gi, 12, uintptr(token))

	gameInputCollectMu.Lock()
	result := gameInputCollect
	gameInputCollect = nil
	gameInputCollectMu.Unlock()
	if result != nil {
		details = append(details, result.Details...)
	}
	if result == nil || result.ID == "" {
		return nil, details, errors.New("GameInput did not provide a ready DualSense haptic endpoint")
	}
	return result, details, nil
}

func initCOM() (bool, error) {
	hr, _, _ := procCoInitializeEx.Call(0, coinitMultithreaded)
	// S_OK, S_FALSE and RPC_E_CHANGED_MODE are acceptable for this process.
	if hresultFailed(hr) && uint32(hr) != 0x80010106 {
		return false, fmt.Errorf("CoInitializeEx: %s", hresultText(hr))
	}
	return uint32(hr) != 0x80010106, nil
}

func createEnumerator() (uintptr, error) {
	var e uintptr
	hr, _, _ := procCoCreateInstance.Call(
		uintptr(unsafe.Pointer(&clsidMMDeviceEnumerator)), 0, clsctxAll,
		uintptr(unsafe.Pointer(&iidIMMDeviceEnumerator)), uintptr(unsafe.Pointer(&e)))
	if hresultFailed(hr) || e == 0 {
		return 0, fmt.Errorf("MMDeviceEnumerator: %s", hresultText(hr))
	}
	return e, nil
}

func deviceID(dev uintptr) string {
	var p uintptr
	hr, _, _ := comCall(dev, 5, uintptr(unsafe.Pointer(&p)))
	if hresultFailed(hr) || p == 0 {
		return ""
	}
	s := utf16PtrString(p)
	procCoTaskMemFree.Call(p)
	return s
}

func deviceState(dev uintptr) uint32 {
	var s uint32
	comCall(dev, 6, uintptr(unsafe.Pointer(&s)))
	return s
}

func deviceFriendlyName(dev uintptr) string {
	var store uintptr
	hr, _, _ := comCall(dev, 4, stgmRead, uintptr(unsafe.Pointer(&store)))
	if hresultFailed(hr) || store == 0 {
		return ""
	}
	defer comRelease(store)
	var pv propVariant
	hr, _, _ = comCall(store, 5, uintptr(unsafe.Pointer(&pkeyDeviceFriendlyName)), uintptr(unsafe.Pointer(&pv)))
	if hresultFailed(hr) {
		return ""
	}
	defer procPropVariantClear.Call(uintptr(unsafe.Pointer(&pv)))
	// VT_LPWSTR = 31.
	if pv.VT == 31 {
		return utf16PtrString(pv.Val)
	}
	return ""
}

func activateAudioClient(dev uintptr) (uintptr, error) {
	var c uintptr
	hr, _, _ := comCall(dev, 3, uintptr(unsafe.Pointer(&iidIAudioClient)), clsctxAll, 0, uintptr(unsafe.Pointer(&c)))
	if hresultFailed(hr) || c == 0 {
		return 0, fmt.Errorf("Activate IAudioClient: %s", hresultText(hr))
	}
	return c, nil
}

func mixFormat(client uintptr) (*waveFormatEx, uintptr, error) {
	var p uintptr
	hr, _, _ := comCall(client, 8, uintptr(unsafe.Pointer(&p)))
	if hresultFailed(hr) || p == 0 {
		return nil, 0, fmt.Errorf("GetMixFormat: %s", hresultText(hr))
	}
	return (*waveFormatEx)(unsafe.Pointer(p)), p, nil
}

func formatSubTypeAt(wf *waveFormatEx) (validBits uint16, channelMask uint32, sub guid, ok bool) {
	if wf == nil || wf.FormatTag != waveFormatExtensibleTag || wf.CbSize < 22 {
		return 0, 0, guid{}, false
	}
	// WAVEFORMATEX is a packed 18-byte C structure. A Go struct has 2 bytes of
	// trailing padding (unsafe.Sizeof == 20), so embedding waveFormatEx inside a
	// Go waveFormatExtensible struct shifts every extended field by two bytes.
	// Read the native WAVEFORMATEXTENSIBLE fields at their documented byte
	// offsets instead: Samples=18, dwChannelMask=20, SubFormat=24.
	base := unsafe.Pointer(wf)
	validBits = *(*uint16)(unsafe.Add(base, 18))
	channelMask = *(*uint32)(unsafe.Add(base, 20))
	sub = *(*guid)(unsafe.Add(base, 24))
	return validBits, channelMask, sub, true
}

func guidText(g guid) string {
	return fmt.Sprintf("%08X-%04X-%04X-%02X%02X-%02X%02X%02X%02X%02X%02X",
		g.Data1, g.Data2, g.Data3,
		g.Data4[0], g.Data4[1], g.Data4[2], g.Data4[3],
		g.Data4[4], g.Data4[5], g.Data4[6], g.Data4[7])
}

func parseFormat(wf *waveFormatEx) (channels, sampleRate, bits, blockAlign int, floatPCM bool, description string, ok bool) {
	if wf == nil {
		return
	}
	channels = int(wf.Channels)
	sampleRate = int(wf.SamplesPerSec)
	bits = int(wf.BitsPerSample) // container size, not ValidBitsPerSample
	blockAlign = int(wf.BlockAlign)
	tag := wf.FormatTag
	validBits := uint16(0)
	channelMask := uint32(0)
	sub := guid{}
	subKnown := false
	encoding := "unknown"

	switch tag {
	case waveFormatIEEEFloat:
		floatPCM = true
		encoding = "float"
	case waveFormatPCM:
		floatPCM = false
		encoding = "PCM"
	case waveFormatExtensibleTag:
		if vb, mask, sg, extOK := formatSubTypeAt(wf); extOK {
			validBits, channelMask, sub, subKnown = vb, mask, sg, true
			switch sub.Data1 {
			case waveFormatIEEEFloat:
				floatPCM = true
				encoding = "float"
			case waveFormatPCM:
				floatPCM = false
				encoding = "PCM"
			}
		}
	}

	bytesPerSample := 0
	if channels > 0 && blockAlign > 0 && blockAlign%channels == 0 {
		bytesPerSample = blockAlign / channels
	}
	// GetMixFormat on Windows normally returns WAVE_FORMAT_EXTENSIBLE float32
	// for shared-mode endpoints. If an unusual driver omits the standard
	// sub-format GUID but still reports a coherent 4-byte sample container,
	// accept it as float32 rather than falsely rejecting a valid haptic endpoint.
	inferredFloat := false
	if encoding == "unknown" && bytesPerSample == 4 && bits == 32 {
		floatPCM = true
		encoding = "float"
		inferredFloat = true
	}

	containerCoherent := bytesPerSample > 0 && bits == bytesPerSample*8
	encodingSupported := (floatPCM && bits == 32) || (!floatPCM && bits == 16 && encoding == "PCM")
	ok = channels >= 4 && sampleRate > 0 && blockAlign > 0 && containerCoherent && encodingSupported

	details := fmt.Sprintf("WASAPI shared %dch %dHz %s%d block=%d tag=0x%04X",
		channels, sampleRate, encoding, bits, blockAlign, tag)
	if tag == waveFormatExtensibleTag {
		details += fmt.Sprintf(" valid=%d mask=0x%08X", validBits, channelMask)
		if subKnown {
			details += " sub=" + guidText(sub)
		}
	}
	if inferredFloat {
		details += " inferred"
	}
	description = details
	return
}

func endpointByID(enumerator uintptr, id string) (uintptr, error) {
	p, err := syscall.UTF16PtrFromString(id)
	if err != nil {
		return 0, err
	}
	var dev uintptr
	hr, _, _ := comCall(enumerator, 5, uintptr(unsafe.Pointer(p)), uintptr(unsafe.Pointer(&dev)))
	if hresultFailed(hr) || dev == 0 {
		return 0, fmt.Errorf("IMMDeviceEnumerator.GetDevice: %s", hresultText(hr))
	}
	return dev, nil
}

func enumerateWASAPICandidates(enumerator uintptr) ([]wasapiCandidate, []string) {
	var collection uintptr
	hr, _, _ := comCall(enumerator, 3, eRender, deviceStateAll, uintptr(unsafe.Pointer(&collection)))
	if hresultFailed(hr) || collection == 0 {
		return nil, []string{"EnumAudioEndpoints=" + hresultText(hr)}
	}
	defer comRelease(collection)
	var count uint32
	comCall(collection, 3, uintptr(unsafe.Pointer(&count)))
	out := make([]wasapiCandidate, 0, count)
	details := make([]string, 0, count)
	for i := uint32(0); i < count; i++ {
		var dev uintptr
		hr, _, _ := comCall(collection, 4, uintptr(i), uintptr(unsafe.Pointer(&dev)))
		if hresultFailed(hr) || dev == 0 {
			continue
		}
		name := deviceFriendlyName(dev)
		id := deviceID(dev)
		state := deviceState(dev)
		c := wasapiCandidate{Index: int(i), ID: id, Name: name, State: state}
		client, err := activateAudioClient(dev)
		if err == nil {
			wf, mem, ferr := mixFormat(client)
			if ferr == nil {
				c.Channels, c.SampleRate, c.Bits, _, c.Float, _, _ = parseFormat(wf)
				procCoTaskMemFree.Call(mem)
			}
			comRelease(client)
		}
		low := strings.ToLower(name + " " + id)
		if strings.Contains(low, "dualsense") || strings.Contains(low, "wireless controller") {
			c.Score += 100
		}
		if c.Channels >= 4 {
			c.Score += 220
		}
		if state == deviceStateActive {
			c.Score += 30
		}
		details = append(details, fmt.Sprintf("WASAPI [%d] state=0x%X %s | %dch %dHz %dbit | id=%s", i, state, name, c.Channels, c.SampleRate, c.Bits, id))
		out = append(out, c)
		comRelease(dev)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].Index < out[j].Index
	})
	return out, details
}

func openWASAPIEngine(enumerator uintptr, dev uintptr, endpoint string, leftChannel, rightChannel int) (*hapticAudioEngine, error) {
	client, err := activateAudioClient(dev)
	if err != nil {
		return nil, err
	}
	wf, mem, err := mixFormat(client)
	if err != nil {
		comRelease(client)
		return nil, err
	}
	channels, rate, bits, block, floatPCM, desc, ok := parseFormat(wf)
	if !ok {
		procCoTaskMemFree.Call(mem)
		comRelease(client)
		return nil, fmt.Errorf("format haptique non pris en charge: %s", desc)
	}
	if leftChannel < 0 || leftChannel >= channels {
		leftChannel = channels - 2
	}
	if rightChannel < 0 || rightChannel >= channels {
		rightChannel = channels - 1
	}
	// The previous public build requested a 100 ms shared-mode buffer and then
	// kept it full. For haptics that translates directly into perceptible input
	// latency. Request roughly two Windows engine periods instead (normally
	// around 20 ms) and, below, never queue silence just to keep that buffer full.
	var defaultPeriod100ns, minimumPeriod100ns int64
	periodHR, _, _ := comCall(client, 9, uintptr(unsafe.Pointer(&defaultPeriod100ns)), uintptr(unsafe.Pointer(&minimumPeriod100ns)))
	requestedDuration100ns := int64(200000) // 20 ms fallback; REFERENCE_TIME is 100 ns units.
	if !hresultFailed(periodHR) && defaultPeriod100ns > 0 {
		requestedDuration100ns = defaultPeriod100ns * 2
		if requestedDuration100ns < 100000 {
			requestedDuration100ns = 100000 // 10 ms floor for broad shared-mode compatibility.
		}
		if requestedDuration100ns > 300000 {
			requestedDuration100ns = 300000 // avoid reintroducing a large queued-latency window.
		}
	}
	hr, _, _ := comCall(client, 3, audclntSharemodeShared, audclntStreamflagsNoPersist, uintptr(requestedDuration100ns), 0, mem, 0)
	procCoTaskMemFree.Call(mem)
	if hresultFailed(hr) {
		comRelease(client)
		return nil, fmt.Errorf("IAudioClient.Initialize: %s", hresultText(hr))
	}
	var frames uint32
	hr, _, _ = comCall(client, 4, uintptr(unsafe.Pointer(&frames)))
	if hresultFailed(hr) || frames == 0 {
		comRelease(client)
		return nil, fmt.Errorf("GetBufferSize: %s", hresultText(hr))
	}
	var render uintptr
	hr, _, _ = comCall(client, 14, uintptr(unsafe.Pointer(&iidIAudioRenderClient)), uintptr(unsafe.Pointer(&render)))
	if hresultFailed(hr) || render == 0 {
		comRelease(client)
		return nil, fmt.Errorf("GetService IAudioRenderClient: %s", hresultText(hr))
	}

	name := deviceFriendlyName(dev)
	if strings.TrimSpace(name) == "" {
		name = "Endpoint haptique GameInput"
	}
	defaultPeriodMS := 0.0
	if defaultPeriod100ns > 0 {
		defaultPeriodMS = float64(defaultPeriod100ns) / 10000.0
	}
	primeFrames := uint32(0)
	if rate > 0 {
		primeMS := defaultPeriodMS
		if primeMS <= 0 {
			primeMS = 5.0
		}
		primeFrames = uint32(math.Round(float64(rate) * primeMS / 1000.0))
		if primeFrames > frames {
			primeFrames = frames
		}
	}
	h := &hapticAudioEngine{
		enumerator: enumerator, device: dev, audioClient: client, renderClient: render,
		endpointID: endpoint, deviceName: name, formatName: desc,
		channels: channels, sampleRate: rate, bits: bits, blockAlign: block, floatPCM: floatPCM,
		leftChannel: leftChannel, rightChannel: rightChannel, bufferFrames: frames,
		requestedBufferMS: float64(requestedDuration100ns) / 10000.0, defaultPeriodMS: defaultPeriodMS, primeFrames: primeFrames,
		stop: make(chan struct{}), done: make(chan struct{}),
	}
	// Prime only one engine period. The render loop never pads the rest of the
	// buffer with future silence, so new haptic PCM can reach the next available
	// audio period instead of waiting behind a persistent 100 ms queue.
	if primeFrames > 0 {
		var p uintptr
		hr, _, _ = comCall(render, 3, uintptr(primeFrames), uintptr(unsafe.Pointer(&p)))
		if !hresultFailed(hr) && p != 0 {
			comCall(render, 4, uintptr(primeFrames), audclntBufferflagsSilent)
		}
	}
	hr, _, _ = comCall(client, 10)
	if hresultFailed(hr) {
		comRelease(render)
		comRelease(client)
		return nil, fmt.Errorf("IAudioClient.Start: %s", hresultText(hr))
	}
	go h.renderLoop()
	return h, nil
}

func openHapticAudioEngine(preferredID int) (*hapticAudioEngine, []string, error) {
	comOwned, err := initCOM()
	if err != nil {
		return nil, nil, err
	}
	enumerator, err := createEnumerator()
	if err != nil {
		if comOwned {
			procCoUninitialize.Call()
		}
		return nil, nil, err
	}
	details := []string{}

	gi, giDetails, giErr := discoverGameInputEndpoint(10 * time.Second)
	details = append(details, giDetails...)
	if giErr == nil && gi != nil {
		dev, derr := endpointByID(enumerator, gi.ID)
		if derr == nil {
			engine, oerr := openWASAPIEngine(enumerator, dev, gi.ID, gi.LeftChannel, gi.RightChannel)
			if oerr == nil {
				engine.comInitialized = comOwned
				details = append(details, fmt.Sprintf("OPEN GameInput/WASAPI %s | %s | L=%d R=%d", engine.deviceName, engine.formatName, engine.leftChannel, engine.rightChannel))
				return engine, details, nil
			}
			details = append(details, "GameInput endpoint WASAPI open error: "+oerr.Error())
			comRelease(dev)
		} else {
			details = append(details, "GameInput endpoint lookup error: "+derr.Error())
		}
	} else if giErr != nil {
		details = append(details, "GameInput: "+giErr.Error())
	}

	// Fallback: unlike waveOut, MMDevice enumerates hidden render endpoints too.
	candidates, wasapiDetails := enumerateWASAPICandidates(enumerator)
	details = append(details, wasapiDetails...)
	if preferredID >= 0 {
		sort.SliceStable(candidates, func(i, j int) bool {
			if candidates[i].Index == preferredID {
				return true
			}
			if candidates[j].Index == preferredID {
				return false
			}
			return false
		})
		details = append(details, fmt.Sprintf("Manual WASAPI priority: index %d", preferredID))
	}
	for _, c := range candidates {
		nameKey := strings.ToLower(c.Name + " " + c.ID)
		looksDualSense := strings.Contains(nameKey, "dualsense") || strings.Contains(nameKey, "wireless controller")
		if c.Channels < 4 || c.State != deviceStateActive || !looksDualSense {
			continue
		}
		dev, derr := endpointByID(enumerator, c.ID)
		if derr != nil {
			continue
		}
		engine, oerr := openWASAPIEngine(enumerator, dev, c.ID, c.Channels-2, c.Channels-1)
		if oerr == nil {
			engine.comInitialized = comOwned
			details = append(details, fmt.Sprintf("OPEN WASAPI fallback [%d] %s | %s", c.Index, engine.deviceName, engine.formatName))
			return engine, details, nil
		}
		details = append(details, fmt.Sprintf("TRY WASAPI [%d] %s | %v", c.Index, c.Name, oerr))
		comRelease(dev)
	}
	comRelease(enumerator)
	if comOwned {
		procCoUninitialize.Call()
	}
	return nil, details, errors.New("no four-channel GameInput/WASAPI haptic endpoint could be opened")
}

func audioDeviceDiagnostics() []string {
	comOwned, err := initCOM()
	if err != nil {
		return []string{err.Error()}
	}
	defer func() {
		if comOwned {
			procCoUninitialize.Call()
		}
	}()
	enumerator, err := createEnumerator()
	if err != nil {
		return []string{err.Error()}
	}
	defer comRelease(enumerator)
	details := []string{}
	_, giDetails, giErr := discoverGameInputEndpoint(4 * time.Second)
	details = append(details, giDetails...)
	if giErr != nil {
		details = append(details, "GameInput result: "+giErr.Error())
	}
	_, w := enumerateWASAPICandidates(enumerator)
	return append(details, w...)
}

func clampAudioSample(v float64) float64 {
	return math.Max(-0.99, math.Min(0.99, v))
}

func (h *hapticAudioEngine) EnableSharedPCM() {
	if h == nil {
		return
	}
	h.mu.Lock()
	h.sharedMode = true
	h.sharedPCM.reset()
	h.mu.Unlock()
}

func (h *hapticAudioEngine) PushSharedSamples(samples []int8, sourceRate int) {
	if h == nil || len(samples) < 2 {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return
	}
	h.sharedMode = true
	h.sharedPCM.pushAtRate(samples, sourceRate)
}

func (h *hapticAudioEngine) mixSharedPCM(dst []byte, frames int) bool {
	if h == nil || !h.sharedMode || frames <= 0 {
		return false
	}
	if cap(h.scratchLeft) < frames {
		h.scratchLeft = make([]float64, frames)
		h.scratchRight = make([]float64, frames)
	}
	left := h.scratchLeft[:frames]
	right := h.scratchRight[:frames]
	rendered := h.sharedPCM.renderInto(left, right, h.sampleRate)
	wrote := false
	for frame := 0; frame < rendered; frame++ {
		lv, rv := clampAudioSample(left[frame]), clampAudioSample(right[frame])
		if math.Abs(lv) > 0.000001 || math.Abs(rv) > 0.000001 {
			wrote = true
		}
		if h.floatPCM {
			lo, ro := frame*h.blockAlign+h.leftChannel*4, frame*h.blockAlign+h.rightChannel*4
			if lo+4 <= len(dst) {
				*(*uint32)(unsafe.Pointer(&dst[lo])) = math.Float32bits(float32(lv))
			}
			if ro+4 <= len(dst) {
				*(*uint32)(unsafe.Pointer(&dst[ro])) = math.Float32bits(float32(rv))
			}
		} else {
			l, r := int16(lv*32767), int16(rv*32767)
			lo, ro := frame*h.blockAlign+h.leftChannel*2, frame*h.blockAlign+h.rightChannel*2
			if lo+2 <= len(dst) {
				dst[lo], dst[lo+1] = byte(l), byte(uint16(l)>>8)
			}
			if ro+2 <= len(dst) {
				dst[ro], dst[ro+1] = byte(r), byte(uint16(r)>>8)
			}
		}
	}
	return wrote
}

func (h *hapticAudioEngine) renderLoop() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	comOwned, _ := initCOM()
	if comOwned {
		defer procCoUninitialize.Call()
	}
	defer close(h.done)
	ticker := time.NewTicker(2 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-h.stop:
			return
		case <-ticker.C:
			h.mu.Lock()
			h.renderTicks++
			if h.closed {
				h.mu.Unlock()
				return
			}
			var padding uint32
			hr, _, _ := comCall(h.audioClient, 6, uintptr(unsafe.Pointer(&padding)))
			if hresultFailed(hr) {
				h.paddingErrors++
				h.lastHRESULT = "GetCurrentPadding " + hresultText(hr)
				h.mu.Unlock()
				continue
			}
			available := h.bufferFrames - padding
			if available == 0 {
				h.mu.Unlock()
				continue
			}
			// Write only PCM that actually exists in the Common Feel queue. Filling
			// every free WASAPI frame with silence keeps the hardware buffer full and
			// delays the next real haptic event by the entire buffer duration.
			queued := h.sharedPCM.availableOutputFrames(h.sampleRate)
			if queued <= 0 {
				h.mu.Unlock()
				continue
			}
			if uint32(queued) < available {
				available = uint32(queued)
			}
			if available == 0 {
				h.mu.Unlock()
				continue
			}
			var p uintptr
			hr, _, _ = comCall(h.renderClient, 3, uintptr(available), uintptr(unsafe.Pointer(&p)))
			if hresultFailed(hr) || p == 0 {
				h.getBufferErrors++
				h.lastHRESULT = "GetBuffer " + hresultText(hr)
				h.mu.Unlock()
				continue
			}
			total := int(available) * h.blockAlign
			dst := unsafe.Slice((*byte)(unsafe.Pointer(p)), total)
			for i := range dst {
				dst[i] = 0
			}
			silent := true
			if h.sharedMode {
				// All vehicle-feel math was already mixed by the Common Feel Engine.
				if h.mixSharedPCM(dst, int(available)) {
					silent = false
				}
			}
			flags := uintptr(0)
			if silent {
				flags = audclntBufferflagsSilent
			}
			hr, _, _ = comCall(h.renderClient, 4, uintptr(available), flags)
			if hresultFailed(hr) {
				h.releaseErrors++
				h.lastHRESULT = "ReleaseBuffer " + hresultText(hr)
			} else {
				h.releasedBuffers++
				h.releasedFrames += uint64(available)
				if silent {
					h.silentFrames += uint64(available)
				} else {
					h.nonSilentBuffers++
					h.nonSilentFrames += uint64(available)
				}
			}
			h.mu.Unlock()
		}
	}
}

func (h *hapticAudioEngine) Close() {
	if h == nil {
		return
	}
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return
	}
	h.closed = true
	close(h.stop)
	h.mu.Unlock()
	<-h.done
	if h.audioClient != 0 {
		comCall(h.audioClient, 11)
		comCall(h.audioClient, 12)
	}
	comRelease(h.renderClient)
	comRelease(h.audioClient)
	comRelease(h.device)
	comRelease(h.enumerator)
	if h.comInitialized {
		procCoUninitialize.Call()
	}
}
