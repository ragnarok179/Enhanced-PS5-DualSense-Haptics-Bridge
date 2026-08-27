//go:build windows && bluetooth

package main

import (
	"fmt"
	"math"
	"path/filepath"
	"runtime"
	"syscall"
	"unsafe"
)

// Bluetooth controller-speaker packets use Opus 48 kHz stereo CBR 160 kb/s.
// At the 10 ms cadence of the 0x36 writer that is exactly 200 bytes/packet.
const (
	btOpusSampleRate          = 48000
	btOpusChannels            = 2
	btOpusFrameMono           = 480
	btSpeakerSourceTickMono   = 512
	btOpusPacketSize          = 200
	avSampleFmtFLT            = 3
	avMediaTypeAudio          = 1
	avChannelNative           = 1
	avOptSearchChildren       = 1
	loadWithAlteredSearchPath = 0x00000008
)

type ffAVChannelLayout struct {
	Order      int32
	NBChannels int32
	Mask       uint64
	Opaque     uintptr
}

type ffAVCodecDescriptorPrefix struct {
	ID        int32
	MediaType int32
}

type ffAVCodecParameters struct {
	CodecType          int32
	CodecID            int32
	CodecTag           uint32
	_pad0              uint32
	Extradata          uintptr
	ExtradataSize      int32
	_pad1              int32
	CodedSideData      uintptr
	NBCodedSideData    int32
	Format             int32
	BitRate            int64
	BitsPerCodedSample int32
	BitsPerRawSample   int32
	Profile            int32
	Level              int32
	Width              int32
	Height             int32
	SampleAspectRatio  [2]int32
	FrameRate          [2]int32
	FieldOrder         int32
	ColorRange         int32
	ColorPrimaries     int32
	ColorTRC           int32
	ColorSpace         int32
	ChromaLocation     int32
	VideoDelay         int32
	ChLayout           ffAVChannelLayout
	SampleRate         int32
	BlockAlign         int32
	FrameSize          int32
	InitialPadding     int32
	TrailingPadding    int32
	SeekPreroll        int32
}

// FFmpeg 8 AVFrame prefix/fields used by the audio encoder. Explicit padding
// makes the offsets self-checking on Windows/amd64 before any DLL call.
type ffAVFrame struct {
	Data                [8]uintptr
	LineSize            [8]int32
	ExtendedData        uintptr
	Width               int32
	Height              int32
	NBSamples           int32
	Format              int32
	PictType            int32
	SampleAspectRatio   [2]int32
	_pad0               int32
	PTS                 int64
	PktDTS              int64
	TimeBase            [2]int32
	Quality             int32
	_pad1               int32
	Opaque              uintptr
	RepeatPict          int32
	SampleRate          int32
	Buf                 [8]uintptr
	ExtendedBuf         uintptr
	NBExtendedBuf       int32
	_pad2               int32
	SideData            uintptr
	NBSideData          int32
	Flags               int32
	ColorRange          int32
	ColorPrimaries      int32
	ColorTRC            int32
	ColorSpace          int32
	ChromaLocation      int32
	_pad3               int32
	BestEffortTimestamp int64
	Metadata            uintptr
	DecodeErrorFlags    int32
	_pad4               int32
	HWFramesCtx         uintptr
	OpaqueRef           uintptr
	CropTop             uintptr
	CropBottom          uintptr
	CropLeft            uintptr
	CropRight           uintptr
	PrivateRef          uintptr
	ChLayout            ffAVChannelLayout
	Duration            int64
}

type ffAVPacketPrefix struct {
	Buf  uintptr
	PTS  int64
	DTS  int64
	Data uintptr
	Size int32
}

type ffmpegOpusAPI struct {
	avcodec, avutil     *syscall.DLL
	findEncoderByName   *syscall.Proc
	descriptorByName    *syscall.Proc
	allocContext3       *syscall.Proc
	freeContext         *syscall.Proc
	parametersAlloc     *syscall.Proc
	parametersFree      *syscall.Proc
	parametersToContext *syscall.Proc
	open2               *syscall.Proc
	sendFrame           *syscall.Proc
	receivePacket       *syscall.Proc
	packetAlloc         *syscall.Proc
	packetFree          *syscall.Proc
	packetUnref         *syscall.Proc
	fillAudioFrame      *syscall.Proc
	frameAlloc          *syscall.Proc
	frameFree           *syscall.Proc
	optSet              *syscall.Proc
	optSetInt           *syscall.Proc
}

func ffmpegOpusABISupported() bool {
	if unsafe.Offsetof(ffAVFrame{}.NBSamples) != 112 || unsafe.Offsetof(ffAVFrame{}.Format) != 116 ||
		unsafe.Offsetof(ffAVFrame{}.SampleRate) != 180 || unsafe.Offsetof(ffAVFrame{}.ChLayout) != 384 {
		return false
	}
	if unsafe.Offsetof(ffAVPacketPrefix{}.Data) != 24 || unsafe.Offsetof(ffAVPacketPrefix{}.Size) != 32 {
		return false
	}
	if unsafe.Offsetof(ffAVCodecParameters{}.ChLayout) != 128 || unsafe.Offsetof(ffAVCodecParameters{}.SampleRate) != 152 {
		return false
	}
	return true
}

var kernel32LoadLibraryExW = syscall.NewLazyDLL("kernel32.dll").NewProc("LoadLibraryExW")

// BeamNG ships FFmpeg as a dependency set in Bin64/VideoStream. Loading the
// absolute avcodec path alone does not make Windows search that folder for the
// DLL's dependencies, so use LOAD_WITH_ALTERED_SEARCH_PATH for this local set.
func loadBeamNGFFmpegDLL(path string) (*syscall.DLL, error) {
	wide, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, fmt.Errorf("invalid DLL path %q: %w", path, err)
	}
	h, _, callErr := kernel32LoadLibraryExW.Call(
		uintptr(unsafe.Pointer(wide)),
		0,
		loadWithAlteredSearchPath,
	)
	if h == 0 {
		if callErr == nil || callErr == syscall.Errno(0) {
			callErr = syscall.EINVAL
		}
		return nil, fmt.Errorf("Failed to load %s: %w", path, callErr)
	}
	return &syscall.DLL{Name: path, Handle: syscall.Handle(h)}, nil
}

func loadFFmpegOpusAPI(beamngPath string) (*ffmpegOpusAPI, error) {
	if !ffmpegOpusABISupported() {
		return nil, fmt.Errorf("FFmpeg ABI layout check failed")
	}
	dir := filepath.Join(beamngPath, "Bin64", "VideoStream")
	avutilPath := filepath.Join(dir, "avutil-60.dll")
	avcodecPath := filepath.Join(dir, "avcodec-62.dll")
	avutil, err := loadBeamNGFFmpegDLL(avutilPath)
	if err != nil {
		return nil, fmt.Errorf("load avutil-60.dll: %w", err)
	}
	avcodec, err := loadBeamNGFFmpegDLL(avcodecPath)
	if err != nil {
		_ = avutil.Release()
		return nil, fmt.Errorf("load avcodec-62.dll: %w", err)
	}
	p := func(d *syscall.DLL, name string) (*syscall.Proc, error) {
		pr, e := d.FindProc(name)
		if e != nil {
			return nil, fmt.Errorf("%s: %w", name, e)
		}
		return pr, nil
	}
	api := &ffmpegOpusAPI{avcodec: avcodec, avutil: avutil}
	var e error
	if api.findEncoderByName, e = p(avcodec, "avcodec_find_encoder_by_name"); e != nil {
		api.close()
		return nil, e
	}
	if api.descriptorByName, e = p(avcodec, "avcodec_descriptor_get_by_name"); e != nil {
		api.close()
		return nil, e
	}
	if api.allocContext3, e = p(avcodec, "avcodec_alloc_context3"); e != nil {
		api.close()
		return nil, e
	}
	if api.freeContext, e = p(avcodec, "avcodec_free_context"); e != nil {
		api.close()
		return nil, e
	}
	if api.parametersAlloc, e = p(avcodec, "avcodec_parameters_alloc"); e != nil {
		api.close()
		return nil, e
	}
	if api.parametersFree, e = p(avcodec, "avcodec_parameters_free"); e != nil {
		api.close()
		return nil, e
	}
	if api.parametersToContext, e = p(avcodec, "avcodec_parameters_to_context"); e != nil {
		api.close()
		return nil, e
	}
	if api.open2, e = p(avcodec, "avcodec_open2"); e != nil {
		api.close()
		return nil, e
	}
	if api.sendFrame, e = p(avcodec, "avcodec_send_frame"); e != nil {
		api.close()
		return nil, e
	}
	if api.receivePacket, e = p(avcodec, "avcodec_receive_packet"); e != nil {
		api.close()
		return nil, e
	}
	if api.packetAlloc, e = p(avcodec, "av_packet_alloc"); e != nil {
		api.close()
		return nil, e
	}
	if api.packetFree, e = p(avcodec, "av_packet_free"); e != nil {
		api.close()
		return nil, e
	}
	if api.packetUnref, e = p(avcodec, "av_packet_unref"); e != nil {
		api.close()
		return nil, e
	}
	if api.fillAudioFrame, e = p(avcodec, "avcodec_fill_audio_frame"); e != nil {
		api.close()
		return nil, e
	}
	if api.frameAlloc, e = p(avutil, "av_frame_alloc"); e != nil {
		api.close()
		return nil, e
	}
	if api.frameFree, e = p(avutil, "av_frame_free"); e != nil {
		api.close()
		return nil, e
	}
	if api.optSet, e = p(avutil, "av_opt_set"); e != nil {
		api.close()
		return nil, e
	}
	if api.optSetInt, e = p(avutil, "av_opt_set_int"); e != nil {
		api.close()
		return nil, e
	}
	return api, nil
}

func (a *ffmpegOpusAPI) close() {
	if a == nil {
		return
	}
	if a.avcodec != nil {
		_ = a.avcodec.Release()
		a.avcodec = nil
	}
	if a.avutil != nil {
		_ = a.avutil.Release()
		a.avutil = nil
	}
}

func cString(s string) *byte { p, _ := syscall.BytePtrFromString(s); return p }

func callInt(p *syscall.Proc, args ...uintptr) int32 {
	r, _, _ := p.Call(args...)
	return int32(r)
}

func (a *ffmpegOpusAPI) setOptChecked(ctx uintptr, key, value string, flags uintptr) error {
	rc := callInt(a.optSet, ctx, uintptr(unsafe.Pointer(cString(key))), uintptr(unsafe.Pointer(cString(value))), flags)
	if rc < 0 {
		return fmt.Errorf("av_opt_set %s=%s: %d", key, value, rc)
	}
	return nil
}

func (a *ffmpegOpusAPI) setOptIntChecked(ctx uintptr, key string, value int64, flags uintptr) error {
	rc := callInt(a.optSetInt, ctx, uintptr(unsafe.Pointer(cString(key))), uintptr(value), flags)
	if rc < 0 {
		return fmt.Errorf("av_opt_set_int %s=%d: %d", key, value, rc)
	}
	return nil
}

type bluetoothOpusStreamEncoder struct {
	api       *ffmpegOpusAPI
	codecName string
	ctx       uintptr
	frame     uintptr
	packet    uintptr
	stereo    []float32
}

func newBluetoothOpusStreamEncoder(beamngPath string) (*bluetoothOpusStreamEncoder, string, error) {
	api, err := loadFFmpegOpusAPI(beamngPath)
	if err != nil {
		return nil, "", err
	}
	enc := &bluetoothOpusStreamEncoder{api: api, stereo: make([]float32, btOpusFrameMono*2)}
	fail := func(err error) (*bluetoothOpusStreamEncoder, string, error) {
		name := enc.codecName
		enc.Close()
		return nil, name, err
	}

	codecName := "libopus"
	codec, _, _ := api.findEncoderByName.Call(uintptr(unsafe.Pointer(cString(codecName))))
	if codec == 0 {
		codecName = "opus"
		codec, _, _ = api.findEncoderByName.Call(uintptr(unsafe.Pointer(cString(codecName))))
	}
	enc.codecName = codecName
	if codec == 0 {
		return fail(fmt.Errorf("FFmpeg Opus encoder not available"))
	}

	enc.ctx, _, _ = api.allocContext3.Call(codec)
	if enc.ctx == 0 {
		return fail(fmt.Errorf("avcodec_alloc_context3 failed"))
	}

	desc, _, _ := api.descriptorByName.Call(uintptr(unsafe.Pointer(cString("opus"))))
	if desc == 0 {
		return fail(fmt.Errorf("FFmpeg Opus descriptor not available"))
	}
	dp := (*ffAVCodecDescriptorPrefix)(unsafe.Pointer(desc))
	if dp.ID <= 0 || dp.MediaType != avMediaTypeAudio {
		return fail(fmt.Errorf("invalid FFmpeg Opus descriptor id=%d type=%d", dp.ID, dp.MediaType))
	}

	par, _, _ := api.parametersAlloc.Call()
	if par == 0 {
		return fail(fmt.Errorf("avcodec_parameters_alloc failed"))
	}
	pp := (*ffAVCodecParameters)(unsafe.Pointer(par))
	pp.CodecType = avMediaTypeAudio
	pp.CodecID = dp.ID
	pp.Format = avSampleFmtFLT
	pp.BitRate = 160000
	pp.ChLayout = ffAVChannelLayout{Order: avChannelNative, NBChannels: btOpusChannels, Mask: 3}
	pp.SampleRate = btOpusSampleRate
	rc := callInt(api.parametersToContext, enc.ctx, par)
	q := par
	api.parametersFree.Call(uintptr(unsafe.Pointer(&q)))
	if rc < 0 {
		return fail(fmt.Errorf("avcodec_parameters_to_context=%d", rc))
	}

	if err := api.setOptChecked(enc.ctx, "frame_duration", "10", avOptSearchChildren); err != nil {
		return fail(err)
	}
	if err := api.setOptChecked(enc.ctx, "vbr", "off", avOptSearchChildren); err != nil {
		return fail(err)
	}
	if err := api.setOptChecked(enc.ctx, "application", "audio", avOptSearchChildren); err != nil {
		return fail(err)
	}
	_ = api.setOptIntChecked(enc.ctx, "strict", -2, 0)
	if rc := callInt(api.open2, enc.ctx, codec, 0); rc < 0 {
		return fail(fmt.Errorf("avcodec_open2=%d", rc))
	}

	enc.frame, _, _ = api.frameAlloc.Call()
	if enc.frame == 0 {
		return fail(fmt.Errorf("av_frame_alloc failed"))
	}
	fp := (*ffAVFrame)(unsafe.Pointer(enc.frame))
	fp.NBSamples = btOpusFrameMono
	fp.Format = avSampleFmtFLT
	fp.SampleRate = btOpusSampleRate
	fp.ChLayout = ffAVChannelLayout{Order: avChannelNative, NBChannels: btOpusChannels, Mask: 3}

	enc.packet, _, _ = api.packetAlloc.Call()
	if enc.packet == 0 {
		return fail(fmt.Errorf("av_packet_alloc failed"))
	}
	return enc, codecName, nil
}

func (e *bluetoothOpusStreamEncoder) Close() {
	if e == nil || e.api == nil {
		return
	}
	if e.packet != 0 {
		p := e.packet
		e.api.packetFree.Call(uintptr(unsafe.Pointer(&p)))
		e.packet = 0
	}
	if e.frame != 0 {
		p := e.frame
		e.api.frameFree.Call(uintptr(unsafe.Pointer(&p)))
		e.frame = 0
	}
	if e.ctx != 0 {
		p := e.ctx
		e.api.freeContext.Call(uintptr(unsafe.Pointer(&p)))
		e.ctx = 0
	}
	e.api.close()
	e.api = nil
}

// EncodeSourceTick encodes exactly one Bluetooth speaker clock tick. The
// 0x36 writer advances every ~10.667 ms (512 source samples at 48 kHz), while
// the controller expects one 10 ms / 480-sample Opus packet in each report.
func (e *bluetoothOpusStreamEncoder) EncodeSourceTick(source []float32) ([]byte, error) {
	if e == nil || e.api == nil || e.ctx == 0 || e.frame == 0 || e.packet == 0 {
		return nil, fmt.Errorf("Bluetooth Opus stream encoder is closed")
	}
	for i := range e.stereo {
		e.stereo[i] = 0
	}
	n := minInt(len(source), btSpeakerSourceTickMono)
	if n > 0 {
		for i := 0; i < btOpusFrameMono; i++ {
			pos := float64(i) * float64(btSpeakerSourceTickMono-1) / float64(btOpusFrameMono-1)
			lo := int(pos)
			hi := lo + 1
			if hi >= btSpeakerSourceTickMono {
				hi = btSpeakerSourceTickMono - 1
			}
			var a, b float32
			if lo < n {
				a = source[lo]
			}
			if hi < n {
				b = source[hi]
			}
			frac := float32(pos - float64(lo))
			v := a*(1-frac) + b*frac
			if float32(math.Abs(float64(v))) > .99 {
				if v > 0 {
					v = .99
				} else {
					v = -.99
				}
			}
			e.stereo[i*2], e.stereo[i*2+1] = v, v
		}
	}

	rc := callInt(e.api.fillAudioFrame, e.frame, btOpusChannels, avSampleFmtFLT, uintptr(unsafe.Pointer(&e.stereo[0])), uintptr(len(e.stereo)*4), 1)
	if rc < 0 {
		return nil, fmt.Errorf("avcodec_fill_audio_frame=%d", rc)
	}
	if rc = callInt(e.api.sendFrame, e.ctx, e.frame); rc < 0 {
		return nil, fmt.Errorf("avcodec_send_frame=%d", rc)
	}
	for {
		rc = callInt(e.api.receivePacket, e.ctx, e.packet)
		if rc == -11 {
			return nil, fmt.Errorf("Opus encoder produced no packet for source tick")
		}
		if rc < 0 {
			return nil, fmt.Errorf("avcodec_receive_packet=%d", rc)
		}
		pk := (*ffAVPacketPrefix)(unsafe.Pointer(e.packet))
		if pk.Data == 0 || pk.Size != btOpusPacketSize {
			e.api.packetUnref.Call(e.packet)
			return nil, fmt.Errorf("unexpected Opus packet size=%d want=%d", pk.Size, btOpusPacketSize)
		}
		raw := unsafe.Slice((*byte)(unsafe.Pointer(pk.Data)), int(pk.Size))
		out := append([]byte(nil), raw...)
		e.api.packetUnref.Call(e.packet)
		if len(out) != btOpusPacketSize || out[0] != 0xF4 {
			return nil, fmt.Errorf("unexpected Opus packet format size=%d toc=0x%02X", len(out), out[0])
		}
		runtime.KeepAlive(e.stereo)
		return out, nil
	}
}

// Kept as a diagnostic/test helper. Production Bluetooth speaker playback uses
// the persistent stream encoder so simultaneous BeamNG one-shots can be mixed
// before Opus encoding instead of being serialized as whole pre-encoded cues.
func encodeBluetoothOpusCollisionPCM(pcm []float32, beamngPath string) ([][]byte, string, error) {
	if len(pcm) == 0 {
		return nil, "", fmt.Errorf("empty PCM")
	}
	enc, backend, err := newBluetoothOpusStreamEncoder(beamngPath)
	if err != nil {
		return nil, backend, err
	}
	defer enc.Close()
	frames := make([][]byte, 0, (len(pcm)+btSpeakerSourceTickMono-1)/btSpeakerSourceTickMono)
	for start := 0; start < len(pcm); start += btSpeakerSourceTickMono {
		tick := make([]float32, btSpeakerSourceTickMono)
		copy(tick, pcm[start:minInt(start+btSpeakerSourceTickMono, len(pcm))])
		packet, err := enc.EncodeSourceTick(tick)
		if err != nil {
			return nil, backend, err
		}
		frames = append(frames, packet)
	}
	return frames, backend, nil
}
