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

package arenal

import (
	"context"
	"encoding/json"
	"fmt"
	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/fsnotify/fsnotify"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"time"
)

func (d *Device) createStreamLogConfigMqttHandler() func(client paho.Client, message paho.Message) {
	return func(client paho.Client, message paho.Message) {
		go func() {
			var payload StreamLogConfigPayload
			if err := json.Unmarshal(message.Payload(), &payload); err != nil {
				d.sdkLogf(LogLevelError, "stream log config parse payload failed, payload is: %s, err is : %s", message.Payload(), err.Error())
				return
			}
			topicFields := strings.Split(message.Topic(), "/")
			traceID := topicFields[len(topicFields)-1]
			// 关闭当前上报任务
			if !payload.StreamLogConfig.Switch {
				cancelFunc, ok := d.streamLogCancelFuncMap.Load(traceID)
				if ok {
					cancelFunc.(context.CancelFunc)()
				}
				return
			}
			ctx, cancelFunc := context.WithCancel(context.Background())
			d.streamLogCancelFuncMap.Store(traceID, cancelFunc)
			d.sendStreamLog(ctx, traceID, payload.StreamLogConfig)
		}()
	}
}

func (d *Device) sendStreamLog(ctx context.Context, traceID string, config StreamLogConfig) {
	var err error
	defer func() {
		// 遇到错误，将错误输出到平台
		if err != nil {
			if token := d.client.Publish(fmt.Sprintf(DeviceStreamLogReportTopic, d.ProductKey, d.Name, traceID), 0, false, err.Error()); token.Wait() && token.Error() != nil {
				d.sdkLogf(LogLevelInfo, "pub stream log report error result failed,clientId = %s,err = %s", formatMqttUserName(d.ProductKey, d.Name), token.Error())
			}
		}
	}()
	logDir := path.Join(d.FilePath, "log")
	// 用于监听日志目录有新文件产生或者文件发生改变
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		d.sdkLogf(LogLevelError, "watch log dir failed, err = %s", err.Error())
		return
	}
	if err = watcher.Add(logDir + "/"); err != nil {
		d.sdkLogf(LogLevelError, "watch log dir failed, err = %s", err.Error())
		return
	}
	// 如果过了3个小时，文件还没上传完并且没收到取消的通知，就退出
	ticker := time.NewTicker(streamLogTimeout)

	// 先把已有数据发送完
	lastReadOffsetMap := make(map[string]int64)
	files, err := ioutil.ReadDir(logDir)
	if err != nil {
		d.sdkLogf(LogLevelError, "list log dir failed, err = %s", err.Error())
		return
	}
	for _, file := range files {
		if !d.isFileNeedReport(file.Name(), config.StartTime, 0) {
			continue
		}
		if err = d.sendStreamLogInFile(path.Join(logDir, file.Name()), traceID, config, 0); err != nil {
			return
		}
		lastReadOffsetMap[path.Join(logDir, file.Name())] = file.Size()
	}
	// 监听新产生的数据
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			return
		case ev, ok := <-watcher.Events:
			if !ok {
				return
			}
			// 有新文件或者新写入，读取文件
			if fileOp(ev.Op) == fileOpCreate || fileOp(ev.Op) == fileOpWrite {
				offset, ok := lastReadOffsetMap[ev.Name]
				if !ok {
					offset = 0
				}
				if err = d.sendStreamLogInFile(ev.Name, traceID, config, offset); err != nil {
					return
				}
				stat, err := os.Stat(ev.Name)
				if err != nil {
					d.sdkLogf(LogLevelError, "stat file %s failed, err = %s", ev.Name, err.Error())
					return
				}
				lastReadOffsetMap[ev.Name] = stat.Size()
			}
		}
	}
}

func (d *Device) sendStreamLogInFile(filepath, traceID string, config StreamLogConfig, offset int64) error {
	fileReader, err := os.Open(filepath)
	if err != nil {
		d.sdkLogf(LogLevelError, "open log file %s failed, err = %s", filepath, err.Error())
		return err
	}
	defer fileReader.Close()
	nextOffset := offset
	for {
		tmpLogs, offset, err := d.getLocalLogs(fileReader, LocalLogCondition{
			CommonLogCondition: CommonLogCondition{
				ContentKeyWord: config.ContentKeyWord,
				Type:           config.Type,
				LogLevel:       config.LogLevel,
				StartTime:      config.StartTime,
				EndTime:        0,
			},
			Offset: nextOffset,
			Count:  localLogSendLimit,
		})
		if err != nil {
			return err
		}
		if len(tmpLogs) == 0 {
			return nil
		}
		nextOffset = offset
		payload, _ := json.Marshal(tmpLogs)
		if token := d.client.Publish(fmt.Sprintf(DeviceStreamLogReportTopic, d.ProductKey, d.Name, traceID), 0, false, payload); token.Wait() && token.Error() != nil {
			d.sdkLogf(LogLevelInfo, "pub stream log failed,clientId = %s,reply = %s,err = %s", formatMqttUserName(d.ProductKey, d.Name), payload, token.Error())
		}
	}
}
