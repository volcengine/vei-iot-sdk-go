//go:build windows
// +build windows

/* This file may have been modified by Beijing Volcano Engine Technology Ltd. */

package conpty

import (
	"context"
	"errors"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modKernel32                        = windows.NewLazySystemDLL("kernel32.dll")
	fCreatePseudoConsole               = modKernel32.NewProc("CreatePseudoConsole")
	fResizePseudoConsole               = modKernel32.NewProc("ResizePseudoConsole")
	fClosePseudoConsole                = modKernel32.NewProc("ClosePseudoConsole")
	fInitializeProcThreadAttributeList = modKernel32.NewProc("InitializeProcThreadAttributeList")
	fUpdateProcThreadAttribute         = modKernel32.NewProc("UpdateProcThreadAttribute")
	fmultiByteToWideChar               = modKernel32.NewProc("MultiByteToWideChar")
	fwideCharToMultiByte               = modKernel32.NewProc("WideCharToMultiByte")
	ErrConPtyUnsupported               = errors.New("ConPty is not available on this version of Windows")
	bRunningCompitible                 = false
)

func CurrentToUtf8(s []byte) ([]byte, error) {
	if s == nil || len(s) <= 0 {
		return []byte(""), nil
	}
	size, _, _ := fmultiByteToWideChar.Call(
		0, 0,
		uintptr(unsafe.Pointer(&s[0])),
		uintptr(len(s)),
		uintptr(0), 0)
	if size <= 0 {
		return []byte(""), windows.GetLastError()
	}
	utf16 := make([]uint16, size)
	rc, _, _ := fmultiByteToWideChar.Call(
		0, 0,
		uintptr(unsafe.Pointer(&s[0])), uintptr(len(s)),
		uintptr(unsafe.Pointer(&utf16[0])), size)
	if rc == 0 {
		return []byte(""), windows.GetLastError()
	}

	size, _, _ = fwideCharToMultiByte.Call(
		65001, 0,
		uintptr(unsafe.Pointer(&utf16[0])),
		uintptr(len(utf16)),
		uintptr(0), 0, uintptr(0), uintptr(0))
	if size <= 0 {
		return nil, windows.GetLastError()
	}
	mbcs := make([]byte, size)
	rc, _, _ = fwideCharToMultiByte.Call(
		65001, 0,
		uintptr(unsafe.Pointer(&utf16[0])),
		uintptr(len(utf16)),
		uintptr(unsafe.Pointer(&mbcs[0])), size, uintptr(0), uintptr(0))
	if rc == 0 {
		return nil, windows.GetLastError()
	}
	if mbcs[size-1] == 0 {
		mbcs = mbcs[:size-1]
	}
	return mbcs, nil
}

func Utf8ToCurrent(s []byte) ([]byte, error) {
	if s == nil || len(s) <= 0 {
		return []byte(""), nil
	}
	size, _, _ := fmultiByteToWideChar.Call(
		65001, 0,
		uintptr(unsafe.Pointer(&s[0])),
		uintptr(len(s)),
		uintptr(0), 0)
	if size <= 0 {
		return []byte(""), windows.GetLastError()
	}
	utf16 := make([]uint16, size)
	rc, _, _ := fmultiByteToWideChar.Call(
		65001, 0,
		uintptr(unsafe.Pointer(&s[0])), uintptr(len(s)),
		uintptr(unsafe.Pointer(&utf16[0])), size)
	if rc == 0 {
		return []byte(""), windows.GetLastError()
	}

	size, _, _ = fwideCharToMultiByte.Call(
		0, 0,
		uintptr(unsafe.Pointer(&utf16[0])),
		uintptr(len(utf16)),
		uintptr(0), 0, uintptr(0), uintptr(0))
	if size <= 0 {
		return nil, windows.GetLastError()
	}
	mbcs := make([]byte, size)
	rc, _, _ = fwideCharToMultiByte.Call(
		0, 0,
		uintptr(unsafe.Pointer(&utf16[0])),
		uintptr(len(utf16)),
		uintptr(unsafe.Pointer(&mbcs[0])), size, uintptr(0), uintptr(0))
	if rc == 0 {
		return nil, windows.GetLastError()
	}
	if mbcs[size-1] == 0 {
		mbcs = mbcs[:size-1]
	}
	return mbcs, nil
}

func IsConPtyAvailable() bool {
	return fCreatePseudoConsole.Find() == nil &&
		fResizePseudoConsole.Find() == nil &&
		fClosePseudoConsole.Find() == nil &&
		fInitializeProcThreadAttributeList.Find() == nil &&
		fUpdateProcThreadAttribute.Find() == nil
}

const (
	_STILL_ACTIVE                        uint32  = 259
	_S_OK                                uintptr = 0
	_PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE uintptr = 0x20016
	defaultConsoleWidth                          = 156 // in characters
	defaultConsoleHeight                         = 40  // in characters
)

type _COORD struct {
	X, Y int16
}

func (c *_COORD) Pack() uintptr {
	return uintptr((int32(c.Y) << 16) | int32(c.X))
}

type _HPCON windows.Handle

type handleIO struct {
	handle windows.Handle
}

func (h *handleIO) Read(p []byte) (int, error) {
	var numRead uint32 = 0
	err := windows.ReadFile(h.handle, p, &numRead, nil)
	return int(numRead), err
}

func (h *handleIO) Write(p []byte) (int, error) {
	var numWritten uint32 = 0
	err := windows.WriteFile(h.handle, p, &numWritten, nil)
	return int(numWritten), err
}

func (h *handleIO) Close() error {
	return windows.CloseHandle(h.handle)
}

type ConPty struct {
	hpc                          _HPCON
	pi                           *windows.ProcessInformation
	ptyIn, ptyOut, cmdIn, cmdOut *handleIO
}

func win32ClosePseudoConsole(hPc _HPCON) {
	if fClosePseudoConsole.Find() != nil {
		return
	}
	// this kills the attached process. there is no return value.
	fClosePseudoConsole.Call(uintptr(hPc))
}

func win32ResizePseudoConsole(hPc _HPCON, coord *_COORD) error {
	if fResizePseudoConsole.Find() != nil {
		return fmt.Errorf("ResizePseudoConsole not found")
	}
	ret, _, _ := fResizePseudoConsole.Call(uintptr(hPc), coord.Pack())
	if ret != _S_OK {
		return fmt.Errorf("ResizePseudoConsole failed with status 0x%x", ret)
	}
	return nil
}

func win32CreatePseudoConsole(c *_COORD, hIn, hOut windows.Handle) (_HPCON, error) {
	if fCreatePseudoConsole.Find() != nil {
		return 0, fmt.Errorf("CreatePseudoConsole not found")
	}
	var hPc _HPCON
	ret, _, _ := fCreatePseudoConsole.Call(
		c.Pack(),
		uintptr(hIn),
		uintptr(hOut),
		0,
		uintptr(unsafe.Pointer(&hPc)))
	if ret != _S_OK {
		return 0, fmt.Errorf("CreatePseudoConsole() failed with status 0x%x", ret)
	}
	return hPc, nil
}

type _StartupInfoEx struct {
	startupInfo   windows.StartupInfo
	attributeList []byte
}

func getStartupInfoExForPTY(hpc _HPCON) (*_StartupInfoEx, error) {
	if fInitializeProcThreadAttributeList.Find() != nil {
		return nil, fmt.Errorf("InitializeProcThreadAttributeList not found")
	}
	if fUpdateProcThreadAttribute.Find() != nil {
		return nil, fmt.Errorf("UpdateProcThreadAttribute not found")
	}
	var siEx _StartupInfoEx
	siEx.startupInfo.Cb = uint32(unsafe.Sizeof(windows.StartupInfo{}) + unsafe.Sizeof(&siEx.attributeList[0]))
	var size uintptr

	// first call is to get required size. this should return false.
	ret, _, _ := fInitializeProcThreadAttributeList.Call(0, 1, 0, uintptr(unsafe.Pointer(&size)))
	siEx.attributeList = make([]byte, size, size)
	ret, _, err := fInitializeProcThreadAttributeList.Call(
		uintptr(unsafe.Pointer(&siEx.attributeList[0])),
		1,
		0,
		uintptr(unsafe.Pointer(&size)))
	if ret != 1 {
		return nil, fmt.Errorf("InitializeProcThreadAttributeList: %v", err)
	}

	ret, _, err = fUpdateProcThreadAttribute.Call(
		uintptr(unsafe.Pointer(&siEx.attributeList[0])),
		0,
		_PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE,
		uintptr(hpc),
		unsafe.Sizeof(hpc),
		0,
		0)
	if ret != 1 {
		return nil, fmt.Errorf("InitializeProcThreadAttributeList: %v", err)
	}
	return &siEx, nil
}

func createConsoleProcessAttachedToPTY(hpc _HPCON, commandLine string) (*windows.ProcessInformation, error) {
	cmdLine, err := windows.UTF16PtrFromString(commandLine)
	if err != nil {
		return nil, err
	}
	siEx, err := getStartupInfoExForPTY(hpc)
	if err != nil {
		return nil, err
	}
	var pi windows.ProcessInformation
	err = windows.CreateProcess(
		nil, // use this if no args
		cmdLine,
		nil,
		nil,
		false, // inheritHandle
		windows.EXTENDED_STARTUPINFO_PRESENT,
		nil,
		nil,
		&siEx.startupInfo,
		&pi)
	if err != nil {
		return nil, err
	}
	return &pi, nil
}

// This will only return the first error.
func closeHandles(handles ...windows.Handle) error {
	var err error
	for _, h := range handles {
		if h != windows.InvalidHandle {
			if err == nil {
				err = windows.CloseHandle(h)
			} else {
				windows.CloseHandle(h)
			}
		}
	}
	return err
}

// Close all open handles and terminate the process.
func (cpty *ConPty) Close() error {
	// there is no return code
	if cpty.hpc != 0 {
		win32ClosePseudoConsole(cpty.hpc)

	}
	return closeHandles(
		cpty.pi.Process,
		cpty.pi.Thread,
		cpty.ptyIn.handle,
		cpty.ptyOut.handle,
		cpty.cmdIn.handle,
		cpty.cmdOut.handle)
}

// Wait for the process to exit and return the exit code. If context is canceled,
// Wait() will return STILL_ACTIVE and an error indicating the context was canceled.
func (cpty *ConPty) Wait(ctx context.Context) (uint32, error) {
	var exitCode uint32 = _STILL_ACTIVE
	for {
		if err := ctx.Err(); err != nil {
			return _STILL_ACTIVE, fmt.Errorf("wait canceled: %v", err)
		}
		ret, _ := windows.WaitForSingleObject(cpty.pi.Process, 1000)
		if ret != uint32(windows.WAIT_TIMEOUT) {
			err := windows.GetExitCodeProcess(cpty.pi.Process, &exitCode)
			return exitCode, err
		}
	}
}

func (cpty *ConPty) Resize(width, height int) error {
	coords := _COORD{
		int16(width),
		int16(height),
	}

	if cpty.hpc != 0 {
		return win32ResizePseudoConsole(cpty.hpc, &coords)
	} else {
		return nil
	}
}

func (cpty *ConPty) Read(p []byte) (int, error) {
	var numRead int
	var err error
	if bRunningCompitible {
		readBuf := make([]byte, len(p)/3)
		numRead, err = cpty.cmdOut.Read(readBuf)
		copy(p, readBuf)
	} else {
		numRead, err = cpty.cmdOut.Read(p)
	}

	if err == nil && bRunningCompitible {
		if result, incomplete, expected := IsUtf8(p[0:numRead]); !result {
			bufstr, err := CurrentToUtf8(p[0:numRead])
			if err == nil {
				pNL := ConvertToWindowsNewlines(bufstr)
				copy(p, pNL)
				numRead = len(pNL)
			}
		} else {
			if incomplete {
				readBufExpected := make([]byte, expected)
				numReadExpected, err := cpty.cmdOut.Read(readBufExpected)
				if err == nil && numReadExpected == expected {
					copy(p[numRead:], readBufExpected)
					numRead += expected
				}
			}
			pNL := ConvertToWindowsNewlines(p[0:numRead])
			copy(p, pNL)
			numRead = len(pNL)
		}
	}
	return numRead, err
}

func (cpty *ConPty) Write(p []byte) (int, error) {
	if len(p) > 0 && p[len(p)-1] == '\r' {
		p = append(p, '\n')
	}

	if bRunningCompitible {
		current, err := Utf8ToCurrent(p)
		if err == nil {
			p = current
		}
		cpty.ptyOut.Write(p)
	}
	return cpty.cmdIn.Write(p)
}

type conPtyArgs struct {
	coords _COORD
}

type ConPtyOption func(args *conPtyArgs)

func ConPtyDimensions(width, height int) ConPtyOption {
	return func(args *conPtyArgs) {
		args.coords.X = int16(width)
		args.coords.Y = int16(height)
	}
}

func StartWinLowVersion(commandString string) (*ConPty, error) {

	var cmdIn, cmdOut, ptyIn, ptyOut windows.Handle
	if err := windows.CreatePipe(&ptyIn, &cmdIn, nil, 0); err != nil {
		return nil, fmt.Errorf("CreatePipe: %v", err)
	}
	if err := windows.CreatePipe(&cmdOut, &ptyOut, nil, 0); err != nil {
		closeHandles(ptyIn, cmdIn)
		return nil, fmt.Errorf("CreatePipe: %v", err)
	}

	var pi windows.ProcessInformation
	var startupInfo windows.StartupInfo
	startupInfo.Cb = uint32(unsafe.Sizeof(windows.StartupInfo{}))
	windows.GetStartupInfo(&startupInfo)
	windows.SetHandleInformation(ptyIn, 1, 1)
	windows.SetHandleInformation(ptyOut, 1, 1)
	startupInfo.StdInput = ptyIn
	startupInfo.StdOutput = ptyOut
	startupInfo.StdErr = ptyOut
	startupInfo.ShowWindow = windows.SW_HIDE
	startupInfo.Flags = windows.STARTF_USESTDHANDLES
	processnameptr := windows.StringToUTF16Ptr(commandString)
	err := windows.CreateProcess(
		processnameptr, // use this if no args
		processnameptr,
		nil,
		nil,
		true, // inheritHandle
		0,
		nil,
		nil,
		&startupInfo,
		&pi)
	if err != nil {
		return nil, err
	}

	cpty := &ConPty{
		pi:     &windows.ProcessInformation{Process: pi.Process, Thread: pi.Thread, ProcessId: pi.ProcessId, ThreadId: pi.ThreadId},
		ptyIn:  &handleIO{ptyIn},
		ptyOut: &handleIO{ptyOut},
		cmdIn:  &handleIO{cmdIn},
		cmdOut: &handleIO{cmdOut},
	}
	bRunningCompitible = true
	return cpty, nil
}

// Start a new process specified in `commandLine` and attach a pseudo console using the Windows
// ConPty API. If ConPty is not available, ErrConPtyUnsupported will be returned.
//
// On successful return, an instance of ConPty is returned. You must call Close() on this to release
// any resources associated with the process. To get the exit code of the process, you can call Wait().
func Start(commandLine string, options ...ConPtyOption) (*ConPty, error) {
	if !IsConPtyAvailable() {
		return StartWinLowVersion(commandLine)
	}
	args := &conPtyArgs{
		coords: _COORD{defaultConsoleWidth, defaultConsoleHeight},
	}
	for _, opt := range options {
		opt(args)
	}

	var cmdIn, cmdOut, ptyIn, ptyOut windows.Handle
	if err := windows.CreatePipe(&ptyIn, &cmdIn, nil, 0); err != nil {
		return nil, fmt.Errorf("CreatePipe: %v", err)
	}
	if err := windows.CreatePipe(&cmdOut, &ptyOut, nil, 0); err != nil {
		closeHandles(ptyIn, cmdIn)
		return nil, fmt.Errorf("CreatePipe: %v", err)
	}

	hPc, err := win32CreatePseudoConsole(&args.coords, ptyIn, ptyOut)
	if err != nil {
		closeHandles(ptyIn, ptyOut, cmdIn, cmdOut)
		return nil, err
	}

	pi, err := createConsoleProcessAttachedToPTY(hPc, commandLine)
	if err != nil {
		closeHandles(ptyIn, ptyOut, cmdIn, cmdOut)
		win32ClosePseudoConsole(hPc)
		return nil, fmt.Errorf("Failed to create console process: %v", err)
	}

	cpty := &ConPty{
		hpc:    hPc,
		pi:     pi,
		ptyIn:  &handleIO{ptyIn},
		ptyOut: &handleIO{ptyOut},
		cmdIn:  &handleIO{cmdIn},
		cmdOut: &handleIO{cmdOut},
	}
	return cpty, nil
}
