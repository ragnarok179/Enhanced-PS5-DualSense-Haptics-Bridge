//go:build windows && usb

package main

import (
	"fmt"
	"runtime"
	"strings"
	"syscall"
	"unsafe"
)

const (
	digcfPresent         = 0x00000002
	digcfDeviceInterface = 0x00000010
	genericRead          = 0x80000000
	genericWrite         = 0x40000000
	fileShareRead        = 0x00000001
	fileShareWrite       = 0x00000002
	openExisting         = 3
	invalidHandle        = ^uintptr(0)
	sonyVID              = 0x054C
	dualSensePID         = 0x0CE6
	dualSenseEdgePID     = 0x0DF2
)

var (
	modHID                = syscall.NewLazyDLL("hid.dll")
	modSetupAPI           = syscall.NewLazyDLL("setupapi.dll")
	modKernel32           = syscall.NewLazyDLL("kernel32.dll")
	procGetGuid           = modHID.NewProc("HidD_GetHidGuid")
	procGetAttrs          = modHID.NewProc("HidD_GetAttributes")
	procGetPPD            = modHID.NewProc("HidD_GetPreparsedData")
	procFreePPD           = modHID.NewProc("HidD_FreePreparsedData")
	procGetCaps           = modHID.NewProc("HidP_GetCaps")
	procGetProd           = modHID.NewProc("HidD_GetProductString")
	procGetFeature        = modHID.NewProc("HidD_GetFeature")
	procGetClassDevs      = modSetupAPI.NewProc("SetupDiGetClassDevsW")
	procEnumIface         = modSetupAPI.NewProc("SetupDiEnumDeviceInterfaces")
	procGetDetail         = modSetupAPI.NewProc("SetupDiGetDeviceInterfaceDetailW")
	procDestroyInfo       = modSetupAPI.NewProc("SetupDiDestroyDeviceInfoList")
	procCreateFile        = modKernel32.NewProc("CreateFileW")
	procWriteFile         = modKernel32.NewProc("WriteFile")
	procReadFile          = modKernel32.NewProc("ReadFile")
	procClose             = modKernel32.NewProc("CloseHandle")
	procGetCurrentProcess = modKernel32.NewProc("GetCurrentProcess")
	procGetCurrentThread  = modKernel32.NewProc("GetCurrentThread")
	procSetPriorityClass  = modKernel32.NewProc("SetPriorityClass")
	procSetThreadPriority = modKernel32.NewProc("SetThreadPriority")
	modWinMM              = syscall.NewLazyDLL("winmm.dll")
	procTimeBeginPeriod   = modWinMM.NewProc("timeBeginPeriod")
	procTimeEndPeriod     = modWinMM.NewProc("timeEndPeriod")
)

type guid struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}
type spDeviceInterfaceData struct {
	CbSize             uint32
	InterfaceClassGuid guid
	Flags              uint32
	Reserved           uintptr
}
type hiddAttributes struct {
	Size                               uint32
	VendorID, ProductID, VersionNumber uint16
}
type hidpCaps struct {
	Usage, UsagePage, InputReportByteLength, OutputReportByteLength, FeatureReportByteLength                                                                                                                                                          uint16
	Reserved                                                                                                                                                                                                                                          [17]uint16
	NumberLinkCollectionNodes, NumberInputButtonCaps, NumberInputValueCaps, NumberInputDataIndices, NumberOutputButtonCaps, NumberOutputValueCaps, NumberOutputDataIndices, NumberFeatureButtonCaps, NumberFeatureValueCaps, NumberFeatureDataIndices uint16
}
type device struct {
	handle                   syscall.Handle
	path, product            string
	productID                uint16
	outputLen, inputLen      int
	writeNormalizationLogged bool
}

func validHandle(h syscall.Handle) bool { return h != 0 && uintptr(h) != invalidHandle }
func (d *device) close() {
	if d != nil && validHandle(d.handle) {
		procClose.Call(uintptr(d.handle))
		d.handle = 0
	}
}
func utf16String(buf []uint16) string {
	n := 0
	for n < len(buf) && buf[n] != 0 {
		n++
	}
	return syscall.UTF16ToString(buf[:n])
}
func productString(h syscall.Handle) string {
	buf := make([]uint16, 128)
	r, _, _ := procGetProd.Call(uintptr(h), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)*2))
	if r == 0 {
		return "DualSense"
	}
	s := strings.TrimSpace(utf16String(buf))
	if s == "" {
		return "DualSense"
	}
	return s
}
func getHIDCaps(h syscall.Handle) (hidpCaps, bool) {
	var pp uintptr
	r, _, _ := procGetPPD.Call(uintptr(h), uintptr(unsafe.Pointer(&pp)))
	if r == 0 || pp == 0 {
		return hidpCaps{}, false
	}
	defer procFreePPD.Call(pp)
	var caps hidpCaps
	status, _, _ := procGetCaps.Call(pp, uintptr(unsafe.Pointer(&caps)))
	return caps, int32(status) >= 0
}
func openPath(path string, access uintptr) syscall.Handle {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return syscall.Handle(invalidHandle)
	}
	h, _, _ := procCreateFile.Call(uintptr(unsafe.Pointer(p)), access, uintptr(fileShareRead|fileShareWrite), 0, openExisting, 0, 0)
	return syscall.Handle(h)
}
func enumerateUSBDualSense() ([]*device, error) {
	var hg guid
	procGetGuid.Call(uintptr(unsafe.Pointer(&hg)))
	hi, _, e := procGetClassDevs.Call(uintptr(unsafe.Pointer(&hg)), 0, 0, digcfPresent|digcfDeviceInterface)
	if hi == invalidHandle {
		return nil, fmt.Errorf("SetupDiGetClassDevsW: %v", e)
	}
	defer procDestroyInfo.Call(hi)
	var out []*device
	for index := uint32(0); ; index++ {
		iface := spDeviceInterfaceData{CbSize: uint32(unsafe.Sizeof(spDeviceInterfaceData{}))}
		r, _, enumErr := procEnumIface.Call(hi, 0, uintptr(unsafe.Pointer(&hg)), uintptr(index), uintptr(unsafe.Pointer(&iface)))
		if r == 0 {
			if errno, ok := enumErr.(syscall.Errno); ok && errno == 259 {
				break
			}
			break
		}
		var required uint32
		procGetDetail.Call(hi, uintptr(unsafe.Pointer(&iface)), 0, 0, uintptr(unsafe.Pointer(&required)), 0)
		if required < 8 {
			continue
		}
		detail := make([]byte, required)
		if unsafe.Sizeof(uintptr(0)) == 8 {
			*(*uint32)(unsafe.Pointer(&detail[0])) = 8
		} else {
			*(*uint32)(unsafe.Pointer(&detail[0])) = 6
		}
		r, _, _ = procGetDetail.Call(hi, uintptr(unsafe.Pointer(&iface)), uintptr(unsafe.Pointer(&detail[0])), uintptr(required), uintptr(unsafe.Pointer(&required)), 0)
		if r == 0 {
			continue
		}
		path := syscall.UTF16ToString((*[1 << 15]uint16)(unsafe.Pointer(&detail[4]))[:])
		h := openPath(path, genericRead|genericWrite)
		if !validHandle(h) {
			continue
		}
		attrs := hiddAttributes{Size: uint32(unsafe.Sizeof(hiddAttributes{}))}
		ok, _, _ := procGetAttrs.Call(uintptr(h), uintptr(unsafe.Pointer(&attrs)))
		if ok == 0 || attrs.VendorID != sonyVID || (attrs.ProductID != dualSensePID && attrs.ProductID != dualSenseEdgePID) {
			procClose.Call(uintptr(h))
			continue
		}
		caps, capsOK := getHIDCaps(h)
		if capsOK && (caps.UsagePage != 1 || caps.Usage != 5) {
			procClose.Call(uintptr(h))
			continue
		}
		outLen, inLen := 547, 78
		if capsOK {
			if caps.OutputReportByteLength > 0 {
				outLen = int(caps.OutputReportByteLength)
			}
			if caps.InputReportByteLength > 0 {
				inLen = int(caps.InputReportByteLength)
			}
		}
		if inLen >= 78 || outLen < 48 {
			procClose.Call(uintptr(h))
			continue
		}
		out = append(out, &device{handle: h, path: path, product: productString(h), productID: attrs.ProductID, outputLen: outLen, inputLen: inLen})
	}
	return out, nil
}
func findUSBDualSense() (*device, error) {
	ds, err := enumerateUSBDualSense()
	if err != nil {
		return nil, err
	}
	if len(ds) == 0 {
		return nil, nil
	}
	d := ds[0]
	for _, other := range ds[1:] {
		other.close()
	}
	return d, nil
}
func (d *device) readReportOnce() ([]byte, error) {
	if d == nil || !validHandle(d.handle) {
		return nil, fmt.Errorf("controller is closed")
	}
	if d.inputLen <= 0 {
		return nil, fmt.Errorf("invalid HID input report size: %d", d.inputLen)
	}
	buf := make([]byte, d.inputLen)
	var read uint32
	r, _, callErr := procReadFile.Call(uintptr(d.handle), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), uintptr(unsafe.Pointer(&read)), 0)
	runtime.KeepAlive(buf)
	if r == 0 {
		if errno, ok := callErr.(syscall.Errno); ok && errno == 0 {
			return nil, fmt.Errorf("ReadFile failed without a Win32 error code")
		}
		return nil, fmt.Errorf("ReadFile: %v", callErr)
	}
	if read == 0 {
		return nil, fmt.Errorf("ReadFile returned an empty report")
	}
	return buf[:read], nil
}

func (d *device) writeReport(report []byte) error {
	if d == nil || !validHandle(d.handle) {
		return fmt.Errorf("controller is closed")
	}
	if len(report) == 0 {
		return fmt.Errorf("empty HID report")
	}

	// USB writes use the HID collection OutputReportByteLength. The logical 0x02
	// payload is placed at the start and any descriptor padding stays zero.
	wire := report
	if d.outputLen > len(report) {
		wire = make([]byte, d.outputLen)
		copy(wire, report)
		if !d.writeNormalizationLogged {
			if runtimeDiagnosticsEnabled() {
				fmt.Printf("Windows HID route validated: logical %d-byte report padded to %d bytes.\n", len(report), len(wire))
			}
			d.writeNormalizationLogged = true
		}
	}

	var written uint32
	r, _, callErr := procWriteFile.Call(
		uintptr(d.handle),
		uintptr(unsafe.Pointer(&wire[0])),
		uintptr(len(wire)),
		uintptr(unsafe.Pointer(&written)),
		0,
	)
	runtime.KeepAlive(report)
	runtime.KeepAlive(wire)
	if r == 0 {
		if errno, ok := callErr.(syscall.Errno); ok && errno == 0 {
			return fmt.Errorf("WriteFile failed without a Win32 error code (%d/%d)", written, len(wire))
		}
		return fmt.Errorf("WriteFile: %v (%d/%d)", callErr, written, len(wire))
	}
	return nil
}

func (d *device) writeReportExact(report []byte) error {
	if d == nil || !validHandle(d.handle) {
		return fmt.Errorf("controller is closed")
	}
	if len(report) == 0 {
		return fmt.Errorf("empty HID report")
	}
	var written uint32
	r, _, callErr := procWriteFile.Call(
		uintptr(d.handle), uintptr(unsafe.Pointer(&report[0])), uintptr(len(report)),
		uintptr(unsafe.Pointer(&written)), 0,
	)
	runtime.KeepAlive(report)
	if r == 0 {
		if errno, ok := callErr.(syscall.Errno); ok && errno == 0 {
			return fmt.Errorf("exact WriteFile failed without a Win32 error code (%d/%d)", written, len(report))
		}
		return fmt.Errorf("WriteFile exact: %v (%d/%d)", callErr, written, len(report))
	}
	return nil
}

func enableRealtimeScheduling() func() {
	// The production haptic cadence is about 10.67 ms. Request 1 ms timer
	// resolution and raise priority modestly so BeamNG rendering does not starve
	// the haptic sender. Avoid REALTIME_PRIORITY_CLASS, which can harm the OS.
	procTimeBeginPeriod.Call(1)
	process, _, _ := procGetCurrentProcess.Call()
	thread, _, _ := procGetCurrentThread.Call()
	const aboveNormalPriorityClass = 0x00008000
	const threadPriorityHighest = 2
	procSetPriorityClass.Call(process, aboveNormalPriorityClass)
	procSetThreadPriority.Call(thread, threadPriorityHighest)
	return func() { procTimeEndPeriod.Call(1) }
}
