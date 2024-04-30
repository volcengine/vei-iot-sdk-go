//go:build !windows
// +build !windows

/*
 * Copyright 2022-2024 Beijing Volcano Engine Technology Ltd.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *    http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package bashexec

import (
	"github.com/creack/pty"
	"github.com/pkg/errors"
	"os"
	"os/exec"
	"syscall"
	"time"
	"unsafe"
)

const (
	DefaultCloseSignal  = syscall.SIGINT
	DefaultCloseTimeout = 10 * time.Second
)

type BashExec struct {
	command string
	argv    []string

	closeSignal  syscall.Signal
	closeTimeout time.Duration

	cmd       *exec.Cmd
	pty       *os.File
	ptyClosed chan struct{}
}

func New(command string, argv []string) (*BashExec, error) {
	cmd := exec.Command(command, argv...)

	pty, err := pty.Start(cmd)
	if err != nil {
		// todo close cmd?
		return nil, errors.Wrapf(err, "failed to start command `%s`,err is `%s`", command, err.Error())
	}
	ptyClosed := make(chan struct{})

	lcmd := &BashExec{
		command: command,
		argv:    argv,

		closeSignal:  DefaultCloseSignal,
		closeTimeout: DefaultCloseTimeout,

		cmd:       cmd,
		pty:       pty,
		ptyClosed: ptyClosed,
	}

	// When the process is closed by the user,
	// close pty so that Read() on the pty breaks with an EOF.
	go func() {
		defer func() {
			lcmd.pty.Close()
			close(lcmd.ptyClosed)
		}()

		_ = lcmd.cmd.Wait()
	}()

	return lcmd, nil
}

func (e *BashExec) Read(p []byte) (n int, err error) {
	return e.pty.Read(p)
}

func (e *BashExec) Write(p []byte) (n int, err error) {
	return e.pty.Write(p)
}

func (e *BashExec) Close() error {
	if e.cmd != nil && e.cmd.Process != nil {
		e.cmd.Process.Signal(e.closeSignal)
	}
	for {
		select {
		case <-e.ptyClosed:
			return nil
		case <-e.closeTimeoutC():
			e.cmd.Process.Signal(syscall.SIGKILL)
		}
	}
}

func (e *BashExec) WindowTitleVariables() map[string]interface{} {
	return map[string]interface{}{
		"command": e.command,
		"argv":    e.argv,
		"pid":     e.cmd.Process.Pid,
	}
}

func (e *BashExec) ResizeTerminal(width int, height int) error {
	window := struct {
		row uint16
		col uint16
		x   uint16
		y   uint16
	}{
		uint16(height),
		uint16(width),
		0,
		0,
	}
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		e.pty.Fd(),
		syscall.TIOCSWINSZ,
		uintptr(unsafe.Pointer(&window)),
	)
	if errno != 0 {
		return errno
	} else {
		return nil
	}
}

func (e *BashExec) closeTimeoutC() <-chan time.Time {
	if e.closeTimeout >= 0 {
		return time.After(e.closeTimeout)
	}

	return make(chan time.Time)
}
