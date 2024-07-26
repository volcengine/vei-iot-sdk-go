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
	"fmt"
	"io"

	"github.com/sirupsen/logrus"
)

const (
	MQTTLogFileMaxSize    = 500
	MQTTLogFileMaxBackups = 3
	MQTTLogFileMaxAge     = 28
	MQTTLogFileCompress   = true
)

type MQTTDebugLogConfig struct {
	LogType                   // 默认输出到console，可选值为console/file
	LogFileName       *string // 日志文件名
	LogFileMaxSize    *int    // 每个日志文件的最大大小（MB）
	LogFileMaxBackups *int    // 保留的旧日志文件的最大数量
	LogFileMaxAge     *int    // 保留的旧日志文件的最大天数
	LogFileCompress   *bool   // 是否压缩旧日志文件
}

func (c *MQTTDebugLogConfig) setDefaults() {
	if c.LogType == "" {
		c.LogType = MQTTLogTypeConsole
	}
	if c.LogFileName == nil {
		c.LogFileName = new(string)
		*c.LogFileName = "arenal.log"
	}
	if c.LogFileMaxSize == nil {
		c.LogFileMaxSize = new(int)
		*c.LogFileMaxSize = MQTTLogFileMaxSize
	}
	if c.LogFileMaxBackups == nil {
		c.LogFileMaxBackups = new(int)
		*c.LogFileMaxBackups = MQTTLogFileMaxBackups
	}
	if c.LogFileMaxAge == nil {
		c.LogFileMaxAge = new(int)
		*c.LogFileMaxAge = MQTTLogFileMaxAge
	}
	if c.LogFileCompress == nil {
		c.LogFileCompress = new(bool)
		*c.LogFileCompress = MQTTLogFileCompress
	}
}

type ConsoleLogger struct {
	Prefix string
}

func NewConsoleLogger(prefix string) *ConsoleLogger {
	return &ConsoleLogger{Prefix: prefix}
}

func (l *ConsoleLogger) Println(v ...interface{}) {
	fmt.Println(l.Prefix, v)
}

func (l *ConsoleLogger) Printf(format string, v ...interface{}) {
	fmt.Printf(l.Prefix+format, v)
}

type FileLogger struct {
	level  logrus.Level
	logger *logrus.Logger
}

func NewFileLogger(level logrus.Level, output io.Writer) *FileLogger {
	logger := logrus.New()
	logger.SetOutput(output)
	logger.SetFormatter(&logrus.JSONFormatter{})
	return &FileLogger{level: level, logger: logger}
}

func (l *FileLogger) Println(v ...interface{}) {
	l.logger.Log(l.level, v...)
}

func (l *FileLogger) Printf(format string, v ...interface{}) {
	l.logger.Logf(l.level, format, v...)
}
