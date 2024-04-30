//go:build windows
// +build windows

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
	"syscall"
	"time"
	"unicode/utf16"

	log "github.com/sirupsen/logrus"
	"github.com/volcengine/vei-iot-sdk-go/arenal/common/webshell/conpty"
	"golang.org/x/sys/windows"
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

	pty       *conpty.ConPty
	ptyClosed chan struct{}
}

func New(command string, argv []string) (*BashExec, error) {
	cmdString := "%windir%\\System32\\cmd.exe"
	cmdExpandString := make([]uint16, 1000)
	cmdStringPtr, _ := windows.UTF16PtrFromString(cmdString)
	nchars, err := windows.ExpandEnvironmentStrings(cmdStringPtr, &cmdExpandString[0], 1000)

	if err != nil && nchars < 2 {
		log.Errorf("failed to get windows system directory path: %s nchars: %d", err.Error(), nchars)
		return nil, err
	}

	cmdExpandStringrunrue := utf16.Decode(cmdExpandString[0 : nchars-1])
	commandLine := string(cmdExpandStringrunrue)
	// commandLine := `c:\windows\system32\cmd.exe`
	pty, err := conpty.Start(commandLine)
	if err != nil {
		log.Errorf("failed to spawn a pty: %s", err.Error())
		return nil, err
	}

	ptyClosed := make(chan struct{})
	bashExec := &BashExec{
		command:      command,
		argv:         argv,
		closeSignal:  DefaultCloseSignal,
		closeTimeout: DefaultCloseTimeout,
		pty:          pty,
		ptyClosed:    ptyClosed,
	}
	return bashExec, nil
}

func (e *BashExec) Read(p []byte) (n int, err error) {
	return e.pty.Read(p)
}

func (e *BashExec) Write(p []byte) (n int, err error) {
	return e.pty.Write(p)
}

func (e *BashExec) Close() error {
	return e.pty.Close()
}

func (e *BashExec) WindowTitleVariables() map[string]interface{} {
	return nil
}

func (e *BashExec) ResizeTerminal(width int, height int) error {
	return e.pty.Resize(width, height)
}

func (e *BashExec) closeTimeoutC() <-chan time.Time {
	if e.closeTimeout >= 0 {
		return time.After(e.closeTimeout)
	}

	return make(chan time.Time)
}
