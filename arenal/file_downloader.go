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
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
)

// FileDownloader 文件下载器
type FileDownloader struct {
	fileSize   int64
	url        string
	outputFile string
	offset     int64 // 开始byte
}

// NewFileDownloader 下载器工厂入口
func NewFileDownloader(url, outputFile string, offset, filesize int64) *FileDownloader {
	return &FileDownloader{
		fileSize:   filesize,
		url:        url,
		outputFile: outputFile,
		offset:     offset,
	}
}

// Download 开始下载任务
func (d *FileDownloader) Download() error {
	if d.offset == d.fileSize {
		return nil
	}

	r, err := d.getNewRequest("GET")
	if err != nil {
		return err
	}
	r.Header.Set("Range", fmt.Sprintf("bytes=%v-%v", d.offset, d.fileSize))
	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		return fmt.Errorf("do request failed, err: %s", err.Error())
	}
	if resp.StatusCode != 200 && resp.StatusCode != 206 {
		return errors.New(fmt.Sprintf("服务器错误状态码: %v", resp.StatusCode))
	}
	defer resp.Body.Close()

	out, err := os.OpenFile(d.outputFile, os.O_APPEND|os.O_RDWR|os.O_CREATE, 0666) // ignore_security_alert
	if err != nil {
		return err
	}

	_, err = io.Copy(out, resp.Body)
	out.Close()
	return err
}

// getNewRequest 创建一个request
func (d *FileDownloader) getNewRequest(method string) (*http.Request, error) {
	r, err := http.NewRequest(
		method,
		d.url,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("new request failed, err: %s", err.Error())
	}
	r.Header.Set("User-Agent", "mojocn")
	return r, nil
}
