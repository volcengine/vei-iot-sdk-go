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

package conpty

import (
	"bytes"
	"fmt"
)

const Utf8InMultiBytes = 2

// IsUtf8 判断字节数组是否为UTF-8编码
func IsUtf8(data []byte) (result, incomplete bool, expected int) {
	i := 0
	for data[i]&0xC0 == 0x80 && i < len(data) { // 跳过开头的可能被之前的buffer截断的UTF8字符尾部
		i++
	}
	for i < len(data) {
		if data[i]&0x80 == 0x00 {
			i++
			continue
		} else if num := byteCount(data[i]); num >= Utf8InMultiBytes {
			i++
			j := 0
			for ; j < num-1 && i < len(data); j++ {
				if data[i]&0xc0 != 0x80 {
					return false, false, 0
				}
				i++
			}
			if j < num-1 {
				// 字节数组尾部存在不完整的UTF8字符,仍需 num-1-j 字节
				return true, true, num - 1 - j
			}
		} else {
			return false, false, 0
		}
	}
	return true, false, 0
}

func byteCount(data byte) int {
	str := fmt.Sprintf("%b", data)
	i := 0
	for i < len(str) {
		if str[i] != '1' {
			break
		}
		i++
	}
	return i
}

// ConvertToWindowsNewlines 统一转换换行符为windows \r\n
func ConvertToWindowsNewlines(d []byte) []byte {
	d = bytes.Replace(d, []byte{13, 10}, []byte{10}, -1)
	d = bytes.Replace(d, []byte{13}, []byte{10}, -1)
	d = bytes.Replace(d, []byte{10}, []byte{13, 10}, -1)
	return d
}
