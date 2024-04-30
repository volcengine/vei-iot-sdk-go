/* This file may have been modified by Beijing Volcano Engine Technology Ltd. */

package webtty

import (
	"encoding/base64"
	"encoding/json"
	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/pkg/errors"
	"github.com/volcengine/vei-iot-sdk-go/arenal/common/webshell/bashexec"
	"sync"
)

// WebTTY bridges a PTY slave and its PTY master.
type WebTTY struct {
	bashExec    *bashexec.BashExec
	client      paho.Client
	topic       string
	windowTitle []byte
	permitWrite bool
	columns     int
	rows        int
	masterPrefs []byte

	bufferSize int
	writeMutex sync.Mutex
}

// New creates a new instance of WebTTY.
func New(client paho.Client, topic string, bashExec *bashexec.BashExec) (*WebTTY, error) {
	wt := &WebTTY{
		bashExec:    bashExec,
		client:      client,
		topic:       topic,
		permitWrite: true,
		columns:     0,
		rows:        0,
		bufferSize:  1024,
	}

	return wt, nil
}

// Run starts the main process of the WebTTY.
func (wt *WebTTY) Run() error {
	err := wt.SendInitializeMessage()
	if err != nil {
		return errors.Wrapf(err, "failed to send initializing message")
	}

	errs := make(chan error, 2)

	go func() {
		errs <- func() error {
			buffer := make([]byte, wt.bufferSize)
			for {
				n, err := wt.bashExec.Read(buffer)
				if err != nil {
					return errors.New("web browser close webshell connect!!!!!")
				}

				err = wt.HandleSlaveReadEvent(buffer[:n])
				if err != nil {
					return err
				}
			}
		}()
	}()

	select {
	case err = <-errs:
	}

	return err
}

func (wt *WebTTY) SendInitializeMessage() error {
	err := wt.MasterWrite(append([]byte{SetWindowTitle}, wt.windowTitle...))
	if err != nil {
		return errors.Wrapf(err, "failed to send window title")
	}

	if wt.masterPrefs != nil {
		err := wt.MasterWrite(append([]byte{SetPreferences}, wt.masterPrefs...))
		if err != nil {
			return errors.Wrapf(err, "failed to set preferences")
		}
	}

	return nil
}

func (wt *WebTTY) HandleSlaveReadEvent(data []byte) error {
	safeMessage := base64.StdEncoding.EncodeToString(data)
	err := wt.MasterWrite(append([]byte{Output}, []byte(safeMessage)...))
	if err != nil {
		return errors.Wrapf(err, "failed to send message to master")
	}

	return nil
}

func (wt *WebTTY) MasterWrite(data []byte) error {
	wt.writeMutex.Lock()
	defer wt.writeMutex.Unlock()

	if token := wt.client.Publish(wt.topic, 1, false, data); token.Error() != nil {
		return errors.Wrapf(token.Error(), "failed to write to master")
	}

	return nil
}

func (wt *WebTTY) HandleMasterReadEvent(data []byte) error {
	if len(data) == 0 {
		return errors.New("unexpected zero length read from master")
	}

	switch data[0] {
	case Input:
		if !wt.permitWrite {
			return nil
		}

		if len(data) <= 1 {
			return nil
		}

		_, err := wt.bashExec.Write(data[1:])
		if err != nil {
			return errors.Wrapf(err, "failed to write received data to slave")
		}

	case Ping:
		err := wt.MasterWrite([]byte{Pong})
		if err != nil {
			return errors.Wrapf(err, "failed to return Pong message to master")
		}

	case ResizeTerminal:
		if wt.columns != 0 && wt.rows != 0 {
			break
		}

		if len(data) <= 1 {
			return errors.New("received malformed remote command for terminal resize: empty payload")
		}

		var args argResizeTerminal
		err := json.Unmarshal(data[1:], &args)
		if err != nil {
			return errors.Wrapf(err, "received malformed data for terminal resize")
		}
		rows := wt.rows
		if rows == 0 {
			rows = int(args.Rows)
		}

		columns := wt.columns
		if columns == 0 {
			columns = int(args.Columns)
		}

		_ = wt.bashExec.ResizeTerminal(columns, rows)
	default:
		return nil
	}

	return nil
}

type argResizeTerminal struct {
	Columns float64
	Rows    float64
}
