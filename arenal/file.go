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
	"bytes"
	"encoding/json"
	"fmt"
	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/pkg/errors"
	"github.com/volcengine/ve-tos-golang-sdk/v2/tos"
	"hash"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

const (
	defaultFilePartSize = 50 * 1024 * 1024
)

// UploadFile 上传文件到云端
// 可选择
// 1、上传到物联网平台，此时无需指定ServiceID。限制单实例文件数不超过1000个，单设备文件数不超过20个
// 2、直接上传到火山引擎TOS，此时需要提前在物联网平台产品详情中配置文件上传配置，文件数量和大小无限制
// 注意：如果在平台设置了上传到TOS的配置，并且没有设置默认，则上传时ServiceID为空将报错
// 返回 远端存储名称 成功标识
func (d *Device) UploadFile(param UploadFileParam) (fileID string, errCode ErrorCode) {
	if param.PartSize == 0 {
		param.PartSize = defaultFilePartSize
	}
	if param.Filename == "" || param.Filesize == 0 || param.FileReader == nil || param.PartSize < minFilePartSize {
		return "", ErrorCodeParamNotValid
	}
	if param.Filesize/param.PartSize > maxFilePartNum {
		return "", ErrorCodeParamNotValid
	}
	return d.uploadFileWithReader(uploadFileParamInternal{
		uploadType: UploadTypeArenal,
		serviceID:  param.ServiceID,
		source:     sourceTypeDevice,
		filename:   param.Filename,
		reader:     param.FileReader,
		filesize:   param.Filesize,
		partSize:   param.PartSize,
	})
}

func (d *Device) createNotifyUploadFileMqttHandler() func(client paho.Client, message paho.Message) {
	uploadFileHandler := func(client paho.Client, message paho.Message) {
		go func() {
			var payload UploadFilePayload
			err := json.Unmarshal(message.Payload(), &payload)
			if err != nil {
				d.sdkLogf(LogLevelError, "notify upload file parse payload failed, payload is: %s, err is : %s", message.Payload(), err.Error())
				return
			}
			topicFields := strings.Split(message.Topic(), "/")
			traceID := topicFields[len(topicFields)-1]
			go d.reportUploadProgress(UploadFileStatusUploading, "", traceID)
			err = d.uploadFile(uploadFileParamInternal{
				uploadType: payload.Params.UploadType,
				source:     sourceTypeArenal,
				serviceID:  payload.Params.ServiceID,
				filepath:   payload.Params.FilePath,
			})
			status := UploadFileStatusSuccess
			description := ""
			if err != nil {
				d.sdkLogf(LogLevelError, "notify upload file failed, payload is: %s, err is : %s", message.Payload(), err.Error())
				status = UploadFileStatusFailed
				description = err.Error()
			}
			go d.reportUploadProgress(status, description, traceID)
		}()
	}
	return uploadFileHandler
}

func (d *Device) uploadFile(param uploadFileParamInternal) error {
	// 判断文件大小和类型
	stat, err := os.Stat(param.filepath)
	if err != nil {
		return err
	}
	if stat.Size() > int64(uploadSizeLimit) {
		return errors.New("file size exceed limit")
	}
	if !stat.Mode().IsRegular() {
		return errors.New("only support regular file")
	}
	file, err := os.Open(filepath.Clean(param.filepath))
	if err != nil {
		return fmt.Errorf("open uploadFile %s failed,err: %s", param.filepath, err.Error())
	}
	defer file.Close()
	param.filename = path.Base(param.filepath)
	param.reader = file
	param.filesize = stat.Size()
	param.partSize = defaultFilePartSize
	_, errCode := d.uploadFileWithReader(param)
	if errCode != SuccessCode {
		return fmt.Errorf("upload file met error, error code: %d", errCode)
	}
	return nil
}

type crc64Reader struct {
	r io.Reader
	h hash.Hash64
}

func (cr *crc64Reader) Read(p []byte) (n int, err error) {
	n, err = cr.r.Read(p)
	_, _ = cr.h.Write(p)
	return n, err
}

func (d *Device) uploadFileWithReader(param uploadFileParamInternal) (string, ErrorCode) {
	if param.uploadType != UploadTypeArenal {
		return "", ErrorCodeParamNotValid
	}
	if param.partSize == 0 {
		param.partSize = defaultFilePartSize
	}
	initData, errCode := d.initUpload(param)
	if errCode != SuccessCode {
		return "", errCode
	}

	parts, crc64, errCode := d.uploadParts(param, *initData)

	endParam := EndUploadFileParams{
		UploadID: initData.UploadID,
		CRC64:    crc64,
	}
	if errCode != SuccessCode {
		endParam.Success = false
		d.endUpload(endParam)
		return "", errCode
	}
	endParam.Success = true
	endParam.UploadedParts = parts
	errCode = d.endUpload(endParam)
	if errCode != SuccessCode {
		return "", errCode
	}

	return initData.FileID, SuccessCode
}

func (d *Device) initUpload(param uploadFileParamInternal) (*InitUploadFileResponseData, ErrorCode) {
	request := InitUploadFileRequest{
		ID:      getRandomStringWithTimeSuffix(8),
		Version: "1.0",
		Params: InitUploadFileParams{
			FileName:  param.filename,
			FileSize:  param.filesize,
			PartSize:  param.partSize,
			ServiceID: param.serviceID,
			Source:    param.source,
		},
	}
	payloadBytes, _ := json.Marshal(request)
	traceID := getRandomStringWithTimeSuffix(8)
	resBytes, errCode := syncCall(d.client, SyncCallParam{
		RequestTopic: fmt.Sprintf(InitUploadFileRequestTopic, d.ProductKey, d.Name, traceID),
		ReplyTopic:   fmt.Sprintf(InitUploadFileReplyTopic, d.ProductKey, d.Name, traceID),
		TimeoutMs:    5000,
		Payload:      string(payloadBytes),
	})
	if errCode != SuccessCode {
		d.sdkLogf(LogLevelError, "syncCall met err: %d", errCode)
		return nil, errCode
	}
	var initReply InitUploadFileResponse
	if err := json.Unmarshal(resBytes, &initReply); err != nil {
		d.sdkLogf(LogLevelError, "unmarshal init file upload reply payload failed,clientId = %s,message = %s", formatMqttUserName(d.ProductKey, d.Name), payloadBytes)
		return nil, ErrorCodeDecodeFailed
	}
	if initReply.Code != SuccessCode {
		return nil, initReply.Code
	}
	return &initReply.Data, SuccessCode
}

func (d *Device) uploadParts(param uploadFileParamInternal, initData InitUploadFileResponseData) (parts []UploadedPart, crc64 uint64, errCode ErrorCode) {
	var wg sync.WaitGroup
	errCh := make(chan error, len(initData.UploadFileURLs))
	var crc64Final uint64
	parts = make([]UploadedPart, 0)
	var partMutex sync.Mutex
	urlMap := make(map[int]string)
	for _, url := range initData.UploadFileURLs {
		urlMap[url.PartNum] = url.UploadURL
	}

	for part := 1; part <= len(initData.UploadFileURLs); part++ {
		data := make([]byte, param.partSize)
		crc64Hash := tos.NewCRC(tos.DefaultCrcTable(), 0)
		cReader := crc64Reader{
			r: param.reader,
			h: crc64Hash,
		}
		_, err := cReader.Read(data)
		if err != nil {
			d.sdkLogf(LogLevelError, "read file met err: %s", err.Error())
			return nil, 0, ErrorCodeFileOperateFailed
		}
		crc64Once := crc64Hash.Sum64()
		crc64Final = tos.CRC64Combine(crc64Final, crc64Once, uint64(len(data)))
		wg.Add(1)
		go func(partNum int, url string, data []byte, crc64 uint64) {
			defer wg.Done()
			req, _ := http.NewRequest(http.MethodPut, url, bytes.NewReader(data))
			httpClient := &http.Client{}
			res, err := httpClient.Do(req)
			if err != nil {
				d.sdkLogf(LogLevelError, "http request met err: %s", err.Error())
				errCh <- err
				return
			}
			if res.StatusCode != http.StatusOK {
				d.sdkLogf(LogLevelError, "http code not 200, %d", res.StatusCode)
				errCh <- fmt.Errorf("http code not 200, %d", res.StatusCode)
				return
			}
			crcInServer, _ := strconv.ParseUint(res.Header.Get("X-Tos-Hash-Crc64ecma"), 10, 64)
			if crcInServer != crc64Once {
				d.sdkLogf(LogLevelError, "crc of part %d not matched, crc local: %d, crc in server %d", partNum, crc64Once, crcInServer)
				errCh <- fmt.Errorf("crc not matched")
				return
			}
			partMutex.Lock()
			parts = append(parts, UploadedPart{
				PartNumber: partNum,
				ETag:       res.Header.Get("ETag"),
			})
			partMutex.Unlock()
		}(part, urlMap[part], data, crc64Once)
	}
	wg.Wait()
	select {
	case <-errCh:
		return nil, 0, ErrorCodeFileOperateFailed
	default:
	}

	return parts, crc64Final, SuccessCode
}

func (d *Device) endUpload(params EndUploadFileParams) ErrorCode {
	endRequest := EndUploadFileRequest{
		ID:      getRandomStringWithTimeSuffix(8),
		Version: "1.0",
		Params:  params,
	}
	payloadBytes, _ := json.Marshal(endRequest)
	traceID := getRandomStringWithTimeSuffix(8)
	resBytes, errCode := syncCall(d.client, SyncCallParam{
		RequestTopic: fmt.Sprintf(EndUploadFileRequestTopic, d.ProductKey, d.Name, traceID),
		ReplyTopic:   fmt.Sprintf(EndUploadFileReplyTopic, d.ProductKey, d.Name, traceID),
		TimeoutMs:    5000,
		Payload:      string(payloadBytes),
	})
	if errCode != SuccessCode {
		d.sdkLogf(LogLevelError, "syncCall met err: %d", errCode)
		return errCode
	}
	var reply CommonReply
	if err := json.Unmarshal(resBytes, &reply); err != nil {
		d.sdkLogf(LogLevelError, "unmarshal end file upload reply payload failed,clientId = %s,message = %s", formatMqttUserName(d.ProductKey, d.Name), payloadBytes)
		return ErrorCodeDecodeFailed
	}
	if reply.Code != SuccessCode {
		return reply.Code
	}
	return SuccessCode
}

func (d *Device) reportUploadProgress(status UploadFileStatus, description, traceID string) {
	payload := UploadFileProgressPayload{
		ID:      getRandomStringWithTimeSuffix(8),
		Version: "1.0",
		Params: UploadFileReport{
			Status:      status,
			Description: description,
		},
	}
	payloadBytes, _ := json.Marshal(payload)
	if token := d.client.Publish(fmt.Sprintf(UploadFileProgressTopic, d.ProductKey, d.Name, traceID), 0, false, payloadBytes); token.Wait() && token.Error() != nil {
		d.sdkLogf(LogLevelError, "pub reportUploadProgress failed,clientId = %s,reply = %s, err: %s", formatMqttUserName(d.ProductKey, d.Name), string(payloadBytes), token.Error())
	}
}
