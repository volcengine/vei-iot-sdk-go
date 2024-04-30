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

import "fmt"

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
