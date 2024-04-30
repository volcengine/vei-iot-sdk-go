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
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"os"
	"path"
	"strings"
	"time"
)

var logLocalStoreChan chan Log
var levelToInt map[LogLevel]int

func init() {
	log.SetFormatter(&log.JSONFormatter{})
	log.SetOutput(os.Stdout)
	log.SetLevel(log.InfoLevel)
	logLocalStoreChan = make(chan Log, 100)
	levelToInt = map[LogLevel]int{
		LogLevelDebug: 1,
		LogLevelInfo:  2,
		LogLevelWarn:  3,
		LogLevelError: 4,
		LogLevelFatal: 5,
	}
}

type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
	LogLevelFatal LogLevel = "fatal"
)

const (
	LogTypeDevice = "device"
	LogTypeSDK    = "sdk"
)

func (d *Device) sdkLogf(level LogLevel, format string, args ...interface{}) {
	content := fmt.Sprintf(format, args...)
	go d.logProcess(LogTypeSDK, level, content)
}

// DeviceLogf 格式化打印设备日志
// 传递 level 日志级别；format 日志格式化字符串；args 参数
func (d *Device) DeviceLogf(level LogLevel, format string, args ...interface{}) {
	content := fmt.Sprintf(format, args...)
	go d.logProcess(LogTypeDevice, level, content)
}

func (d *Device) sdkLog(level LogLevel, content string) {
	go d.logProcess(LogTypeSDK, level, content)
}

// DeviceLog 打印设备日志
// 传递 level 日志级别；content 日志内容
func (d *Device) DeviceLog(level LogLevel, content string) {
	go d.logProcess(LogTypeDevice, level, content)
}

func (d *Device) logProcess(logType string, level LogLevel, content string) {
	// 输出到控制台
	switch level {
	case LogLevelDebug:
		log.Debug(content)
	case LogLevelInfo:
		log.Info(content)
	case LogLevelWarn:
		log.Warn(content)
	case LogLevelError:
		log.Error(content)
	case LogLevelFatal:
		log.Fatal(content)
	}

	logObj := Log{
		CreateTime: GetTimeMill(),
		LogLevel:   string(level),
		Content:    content,
		Type:       logType,
	}
	// 先将日志写入本地存储，在用户指定目录下新建log目录，在目录下以log_yyyy-mm-dd.log命名
	logLocalStoreChan <- logObj

	// 如果满足发送条件，将log消息推入chan，等待消费
	if d.LogReportConfig.Switch && levelToInt[LogLevel(d.LogReportConfig.LowestLevel)] <= levelToInt[level] {
		d.LogReportConfig.logSendChannel <- logObj
	}
	// TODO 根据实时流要求 实时pub
}

func (d *Device) logReport() {
	// 启动协程，轮训判断chan 是否满
	logSendFlagChan := make(chan bool, 1)
	go func() {
		// 每5秒发送一次消息
		ticker := time.NewTicker(time.Second * 5)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
			case <-d.ctx.Done():
				return
			}
			if !d.LogReportConfig.Switch {
				continue
			}
			// 如果chan满，触发发送
			if len(d.LogReportConfig.logSendChannel) == cap(d.LogReportConfig.logSendChannel) {
				select {
				case logSendFlagChan <- true:
				case <-d.ctx.Done():
					return
				}
			}
		}
	}()
	ticker := time.NewTicker(d.LogReportConfig.logSendMinInterval)
	defer ticker.Stop()
	for {
		// 通道已满/达到最大间隔时间，上报
		select {
		case <-ticker.C:
		case <-logSendFlagChan:
		case <-d.ctx.Done():
			return
		}
		if !d.LogReportConfig.Switch {
			continue
		}
		logList := make([]Log, 0)
		// 最多发送40条
		for i := 0; i < cap(d.LogReportConfig.logSendChannel); i++ {
			select {
			case l := <-d.LogReportConfig.logSendChannel:
				logList = append(logList, l)
			default:
			}
		}
		if len(logList) == 0 {
			// 如果没有log，不上报
			continue
		}
		payload := LogReportPayload{
			ID:      getRandomStringWithTimeSuffix(8),
			Version: "1.0",
			Data:    logList,
		}
		payloadBytes, _ := json.Marshal(payload)
		d.client.Publish(fmt.Sprintf(DeviceLogReportTopic, d.ProductKey, d.Name), 0, false, payloadBytes)
	}
}

func (d *Device) logLocalStorage() {
	for {
		// 最大间隔时间为5s
		// 每100条写一次文件
		logList := make([]string, 0, 100)
		for i := 0; i < 100; i++ {
			timeoutFlag := false
			c := time.After(time.Second * 5)
			select {
			case l := <-logLocalStoreChan:
				logBytes, _ := json.Marshal(l)
				logList = append(logList, string(logBytes))
			case <-c:
				timeoutFlag = true
			case <-d.ctx.Done():
				return
			}
			if timeoutFlag {
				break
			}
		}
		if len(logList) == 0 {
			continue
		}
		content := strings.Join(logList, "\n") + "\n"
		d.writeLogToLocalStorage(content)
	}
}

func (d *Device) writeLogToLocalStorage(content string) {
	logDir := path.Join(d.FilePath, "log")
	date := time.Now().Format("2006-01-02")
	logFilePath := path.Join(logDir, fmt.Sprintf("log_%s.log", date))
	logFile, err := os.OpenFile(logFilePath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Errorf("Open Log File Failed,err is: %s", err.Error())
		return
	}
	defer logFile.Close()
	logFile.Write([]byte(content))
}
