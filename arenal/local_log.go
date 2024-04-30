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
	"bufio"
	"encoding/json"
	"fmt"
	paho "github.com/eclipse/paho.mqtt.golang"
	"io"
	"io/ioutil"
	"os"
	"path"
	"regexp"
	"strings"
	"time"
)

func (d *Device) createLocalLogConfigMqttHandler() func(client paho.Client, message paho.Message) {
	return func(client paho.Client, message paho.Message) {
		go func() {
			var payload LocalLogConfigPayload
			if err := json.Unmarshal(message.Payload(), &payload); err != nil {
				d.sdkLogf(LogLevelError, "stream log config parse payload failed, payload is: %s, err is : %s", message.Payload(), err.Error())
				return
			}
			topicFields := strings.Split(message.Topic(), "/")
			traceID := topicFields[len(topicFields)-1]
			d.sendLocalLogs(traceID, payload.LocalLogCondition)
		}()
	}
}

func (d *Device) sendLocalLogs(traceID string, cond LocalLogCondition) {
	logDir := path.Join(d.FilePath, "log")
	files, err := ioutil.ReadDir(logDir)
	if err != nil {
		d.sdkLogf(LogLevelError, "list log dir %v failed, err = %s", logDir, err.Error())
		return
	}
	if cond.Count == 0 {
		cond.Count = localLogSendLimit
	}
	localLogs := make([]Log, 0)
	var totalOffset int64
	nextOffset := cond.Offset
	for _, file := range files {
		// 跳过日期不符的文件
		if !d.isFileNeedReport(file.Name(), cond.StartTime, 0) {
			continue
		}
		// 跳过offset不符的文件
		if file.Size() <= nextOffset {
			totalOffset += file.Size()
			nextOffset -= file.Size()
			continue
		}
		fileReader, err := os.Open(path.Join(logDir, file.Name()))
		if err != nil {
			d.sdkLogf(LogLevelError, "open log file %s failed, err = %s", path.Join(logDir, file.Name()), err.Error())
			return
		}
		var tmpLogs []Log
		tmpLogs, nextOffset, err = d.getLocalLogs(fileReader, LocalLogCondition{
			CommonLogCondition: cond.CommonLogCondition,
			Offset:             nextOffset,
			Count:              cond.Count - len(localLogs),
		})
		fileReader.Close()
		if err != nil {
			d.sdkLogf(LogLevelError, "close fileReader %v failed,err: %s", path.Join(logDir, file.Name()), err.Error())
			return
		}
		totalOffset += nextOffset
		for _, log := range tmpLogs {
			localLogs = append(localLogs, log)
		}
		if len(localLogs) == cond.Count {
			break
		}
		// 如果一个文件没有读到足够的数据，从头开始读下一个文件
		nextOffset = 0
	}
	// 如果读到的日志小于指定count，说明读完了
	if len(localLogs) < cond.Count {
		totalOffset = 0
	}
	payload := LocalLogReportPayload{
		ID:      getRandomStringWithTimeSuffix(8),
		Version: "1.0",
		Data: LocalLogResult{
			Logs:   localLogs,
			Offset: totalOffset,
		},
	}
	bytes, _ := json.Marshal(payload)
	if token := d.client.Publish(fmt.Sprintf(DeviceLocalLogReportTopic, d.ProductKey, d.Name, traceID), 0, false, bytes); token.Wait() && token.Error() != nil {
		d.sdkLogf(LogLevelInfo, "pub local log failed,clientId = %s,payload = %+v,err = %s", formatMqttUserName(d.ProductKey, d.Name), payload, token.Error())
	}
}

func (d *Device) getLocalLogs(file *os.File, cond LocalLogCondition) (logs []Log, offset int64, err error) {
	buf := bufio.NewReader(file)
	// 读取log
	var line []byte
	if _, err = file.Seek(cond.Offset, 0); err != nil {
		d.sdkLogf(LogLevelError, "seek log file %s failed, err = %s", file.Name(), err.Error())
		return
	}
	offset = cond.Offset
	for {
		line, err = buf.ReadBytes('\n')
		offset += int64(len(line))
		if err == io.EOF {
			err = nil
			break
		}
		if err != nil {
			return
		}
		var log Log
		if err = json.Unmarshal(line, &log); err != nil {
			continue
		}
		if skipLog(log, cond.CommonLogCondition) {
			continue
		}
		logs = append(logs, Log{
			CreateTime: log.CreateTime,
			LogLevel:   log.LogLevel,
			Content:    log.Content,
			Type:       log.Type,
		})
		if len(logs) == cond.Count {
			break
		}
	}
	return
}

func skipLog(log Log, cond CommonLogCondition) bool {
	// 按照条件过滤
	if cond.Type != "" && !strings.Contains(cond.Type, log.Type) {
		return true
	}
	if cond.StartTime != 0 && log.CreateTime < cond.StartTime {
		return true
	}
	if cond.LogLevel != "" && !strings.Contains(cond.LogLevel, log.LogLevel) {
		return true
	}
	if cond.ContentKeyWord != "" && !strings.Contains(log.Content, cond.ContentKeyWord) {
		return true
	}
	return false
}

func (d *Device) isFileNeedReport(filename string, startTime, endTime int64) bool {
	re, _ := regexp.Compile(`log_(.*).log`)
	matches := re.FindStringSubmatch(filename)
	if len(matches) < 2 {
		return false
	}
	fileStartTime, parseErr := time.Parse("2006-01-02", matches[1])
	if parseErr != nil {
		d.sdkLogf(LogLevelError, "parse filename failed, err = %s", parseErr.Error())
		return false
	}
	fileEndTime := fileStartTime.AddDate(0, 0, 1)
	// 过滤较老的文件
	if startTime != 0 && fileEndTime.Before(time.Unix(-1, startTime*1000*1000)) {
		return false
	}
	// 过滤比结束更新的文件
	if endTime != 0 && fileStartTime.After(time.Unix(-1, endTime*1000*1000)) {
		return false
	}
	return true
}
