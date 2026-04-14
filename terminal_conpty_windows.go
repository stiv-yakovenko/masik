//go:build windows

package main

import (
	"fmt"
	"io"
	"os"
	"sync"
	"unsafe"

	"gioui.org/app"
	"golang.org/x/sys/windows"
)

type terminalSession struct {
	mu      sync.Mutex
	hpc     windows.Handle
	process windows.Handle
	thread  windows.Handle
	input   *os.File
	output  *os.File
	once    sync.Once
}

func startTerminalSession(currentDir string, cols, rows int) (*terminalSession, error) {
	if cols <= 0 {
		cols = defaultTerminalCols
	}
	if rows <= 0 {
		rows = defaultTerminalRows
	}

	conptyInRead, conptyInWrite, err := makeTerminalPipePair()
	if err != nil {
		return nil, err
	}
	cleanupInput := true
	defer func() {
		if cleanupInput {
			_ = conptyInRead.Close()
			_ = conptyInWrite.Close()
		}
	}()

	conptyOutRead, conptyOutWrite, err := makeTerminalPipePair()
	if err != nil {
		return nil, err
	}
	cleanupOutput := true
	defer func() {
		if cleanupOutput {
			_ = conptyOutRead.Close()
			_ = conptyOutWrite.Close()
		}
	}()

	var hpc windows.Handle
	size := windows.Coord{X: int16(cols), Y: int16(rows)}
	if err := windows.CreatePseudoConsole(size, windows.Handle(conptyInRead.Fd()), windows.Handle(conptyOutWrite.Fd()), 0, &hpc); err != nil {
		return nil, fmt.Errorf("CreatePseudoConsole: %w", err)
	}
	cleanupHPC := true
	defer func() {
		if cleanupHPC {
			windows.ClosePseudoConsole(hpc)
		}
	}()

	attrList, err := windows.NewProcThreadAttributeList(1)
	if err != nil {
		return nil, err
	}
	defer attrList.Delete()
	if err := attrList.Update(windows.PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE, unsafe.Pointer(hpc), unsafe.Sizeof(hpc)); err != nil {
		return nil, fmt.Errorf("update pseudoconsole attribute: %w", err)
	}

	shell := os.Getenv("COMSPEC")
	if shell == "" {
		shell = "C:\\Windows\\System32\\cmd.exe"
	}
	cmdLine := shell + " /Q /D"
	cmdLineBuf, err := windows.UTF16FromString(cmdLine)
	if err != nil {
		return nil, err
	}
	var currentDirPtr *uint16
	if currentDir != "" {
		currentDirPtr, err = windows.UTF16PtrFromString(currentDir)
		if err != nil {
			return nil, err
		}
	}

	si := windows.StartupInfoEx{
		StartupInfo:             windows.StartupInfo{Cb: uint32(unsafe.Sizeof(windows.StartupInfoEx{}))},
		ProcThreadAttributeList: attrList.List(),
	}
	var pi windows.ProcessInformation
	if err := windows.CreateProcess(nil, &cmdLineBuf[0], nil, nil, true, windows.EXTENDED_STARTUPINFO_PRESENT|windows.CREATE_UNICODE_ENVIRONMENT, nil, currentDirPtr, &si.StartupInfo, &pi); err != nil {
		return nil, fmt.Errorf("CreateProcess: %w", err)
	}

	session := &terminalSession{
		hpc:     hpc,
		process: pi.Process,
		thread:  pi.Thread,
		input:   conptyInWrite,
		output:  conptyOutRead,
	}

	_ = conptyInRead.Close()
	_ = conptyOutWrite.Close()
	cleanupInput = false
	cleanupOutput = false
	cleanupHPC = false
	return session, nil
}

func makeTerminalPipePair() (*os.File, *os.File, error) {
	var readHandle windows.Handle
	var writeHandle windows.Handle
	if err := windows.CreatePipe(&readHandle, &writeHandle, nil, 0); err != nil {
		return nil, nil, err
	}
	return os.NewFile(uintptr(readHandle), "conpty-read"), os.NewFile(uintptr(writeHandle), "conpty-write"), nil
}

func (s *terminalSession) Write(data []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.input == nil {
		return 0, io.ErrClosedPipe
	}
	return s.input.Write(data)
}

func (s *terminalSession) Resize(cols, rows int) error {
	if cols <= 0 || rows <= 0 {
		return nil
	}
	s.mu.Lock()
	hpc := s.hpc
	s.mu.Unlock()
	if hpc == 0 {
		return nil
	}
	return windows.ResizePseudoConsole(hpc, windows.Coord{X: int16(cols), Y: int16(rows)})
}

func (s *terminalSession) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.input != nil {
		_, _ = s.input.Write([]byte("exit\r"))
	}
	if s.process != 0 {
		_ = windows.TerminateProcess(s.process, 0)
	}
}

func (s *terminalSession) readOutput(tabID int, terminalEvents chan<- terminalProcessEvent, win *app.Window) {
	buf := make([]byte, 4096)
	for {
		n, err := s.output.Read(buf)
		if n > 0 {
			chunk := make([]byte, n)
			copy(chunk, buf[:n])
			terminalEvents <- terminalProcessEvent{TabID: tabID, Data: chunk}
			win.Invalidate()
		}
		if err != nil {
			if err != io.EOF {
				terminalEvents <- terminalProcessEvent{TabID: tabID, Data: []byte("\r\n[terminal stream error] " + err.Error() + "\r\n")}
				win.Invalidate()
			}
			return
		}
	}
}

func (s *terminalSession) wait(tabID int, terminalEvents chan<- terminalProcessEvent, win *app.Window) {
	_, waitErr := windows.WaitForSingleObject(s.process, windows.INFINITE)
	var exitCode uint32
	_ = windows.GetExitCodeProcess(s.process, &exitCode)
	s.Close()

	var err error
	if waitErr != nil {
		err = waitErr
	} else if exitCode != 0 {
		err = fmt.Errorf("exit code %d", exitCode)
	}
	terminalEvents <- terminalProcessEvent{TabID: tabID, Closed: true, Err: err}
	win.Invalidate()
}

func (s *terminalSession) Close() {
	s.once.Do(func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.input != nil {
			_ = s.input.Close()
			s.input = nil
		}
		if s.output != nil {
			_ = s.output.Close()
			s.output = nil
		}
		if s.thread != 0 {
			_ = windows.CloseHandle(s.thread)
			s.thread = 0
		}
		if s.process != 0 {
			_ = windows.CloseHandle(s.process)
			s.process = 0
		}
		if s.hpc != 0 {
			windows.ClosePseudoConsole(s.hpc)
			s.hpc = 0
		}
	})
}
