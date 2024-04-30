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
	paho "github.com/eclipse/paho.mqtt.golang"
	"os"
	"path/filepath"
	"time"
)

// SetCustomJobHandler 配置自定义任务Handler
// 传递 CustomJobHandler 根据参数中的身份标识区分设备自身或子设备，根据Param和FilePath执行相应的任务
func (d *Device) SetCustomJobHandler(handler CustomJobHandler) {
	d.customJobHandler = handler
}

func (d *Device) runTask(device DeviceIdentification, param TaskPayloadParams, initialStatus JobStatus) {
	status := initialStatus
	var errCode ErrorCode
	var errDesc string

	jobKey := fmt.Sprintf("%s|%s|%s", device.ProductKey, device.DeviceName, param.JobID)
	value, ok := d.executingJobMap.Load(jobKey)
	if ok && value.(*ExecutingJob).Status != JobStatusCreated {
		// 任务已经在执行，上报当前状态即可
		reportParam := TaskProgressReportParam{
			JobID:      param.JobID,
			Status:     value.(*ExecutingJob).Status,
			ResultCode: errCode,
			ResultDesc: errDesc,
			Time:       GetTimeMill(),
		}
		code := d.reportTaskProcess(device, reportParam)
		if code != SuccessCode {
			d.sdkLogf(LogLevelError, "reportTaskProcess failed, ret code: %d, report param: %#v", code, reportParam)
		}
		return
	}

	executingJob := ExecutingJob{
		ExecutingJobMeta: ExecutingJobMeta{
			TaskPayloadParams: param,
			Status:            initialStatus,
		},
		jobCancelChannel:  make(chan struct{}),
		jobTimeoutChannel: make(chan struct{}),
	}
	d.executingJobMap.Store(jobKey, &executingJob)

	// 如果设置了超时时间，启动协程，计时发送timeout信号
	if param.TimeoutInMinutes != 0 {
		go func() {
			duration := time.Minute * time.Duration(param.TimeoutInMinutes)
			<-time.After(duration)
			if value, ok := d.executingJobMap.Load(param.JobID); ok {
				value.(*ExecutingJob).jobTimeoutChannel <- struct{}{}
			}
		}()
	}

	// 状态机
	for status != JobStatusSuccess && status != JobStatusFailed {
		switch status {
		case JobStatusCreated:
			if param.Type == JobTypeCustom && param.Param.CustomParams.URL != "" {
				status = JobStatusDownloading
			} else {
				status = JobStatusInProgress
			}
		case JobStatusDownloading:
			executingJob.Status = JobStatusDownloading
			d.saveExecutingJob()
			errCode = d.downloadTaskFile(device, param)
			status = JobStatusInProgress
		case JobStatusInProgress:
			executingJob.Status = JobStatusInProgress
			d.saveExecutingJob()
			errCode, errDesc = d.execTask(device, param)
			status = JobStatusSuccess
		}

		if errCode != SuccessCode {
			// 如果是取消，则不上报进度
			if errCode == JobErrorCodeCanceled {
				return
			}
			status = JobStatusFailed
		}
		reportParam := TaskProgressReportParam{
			JobID:      param.JobID,
			Status:     status,
			ResultCode: errCode,
			ResultDesc: errDesc,
			Time:       GetTimeMill(),
		}
		code := d.reportTaskProcess(device, reportParam)
		if code != SuccessCode {
			d.sdkLogf(LogLevelError, "reportTaskProcess failed, ret code: %d, report param: %#v", code, reportParam)
		}
	}
}

func (d *Device) saveExecutingJob() {
	content := make(map[string]ExecutingJobMeta)
	d.executingJobMap.Range(func(k, v interface{}) bool {
		content[k.(string)] = v.(*ExecutingJob).ExecutingJobMeta
		return true
	})

	bytes, err := json.Marshal(content)
	if err != nil {
		d.sdkLogf(LogLevelError, "marshal executing job failed, error: %s", err.Error())
		return
	}

	err = saveFile(filepath.Join(d.FilePath, executingJobPath), filepath.Join(d.FilePath, executingJobMD5File), bytes)
	if err != nil {
		d.sdkLogf(LogLevelError, "saveFile failed, error: %s", err.Error())
	}
}

func (d *Device) loadExecutingJob() {
	bytes, err := readFile(filepath.Join(d.FilePath, executingJobPath), filepath.Join(d.FilePath, executingJobMD5File))
	if err != nil {
		d.sdkLogf(LogLevelError, "Read executing job file Failed! Error:%s", err)
		return
	}

	if len(bytes) == 0 {
		return
	}

	var executingJobMap map[string]ExecutingJobMeta
	err = json.Unmarshal(bytes, &executingJobMap)
	if err != nil {
		d.sdkLogf(LogLevelError, "Unmarshal executing job Failed! Error:%s", err)
		return
	}

	for k, job := range executingJobMap {
		productKey, deviceName, _ := getElemFromJobKey(k)
		// 对于进行中的任务，根据策略决定状态
		if job.Status == JobStatusDownloading || job.Status == JobStatusInProgress {
			if job.ActionOnInterrupt == ActionOnInterruptFail {
				job.Status = JobStatusFailed
			} else {
				job.Status = JobStatusCreated
			}
		}
		d.executingJobMap.Store(k, &ExecutingJob{
			ExecutingJobMeta:  job,
			jobCancelChannel:  make(chan struct{}),
			jobTimeoutChannel: make(chan struct{}),
		})

		d.runTask(DeviceIdentification{
			ProductKey: productKey,
			DeviceName: deviceName,
		}, job.TaskPayloadParams, job.Status)
	}
}

func (d *Device) execTask(device DeviceIdentification, param TaskPayloadParams) (errCode ErrorCode, errDesc string) {
	var function func()
	switch param.Type {
	case JobTypePropertySet:
		function = func() {
			errCode = d.propertySetHandler(PropertySetMessage{
				DeviceIdentification: device,
				Params:               param.Param.PropertySetParams,
			})
			if errCode != SuccessCode {
				errDesc = "属性设置失败"
			}
		}
	case JobTypeServiceCall:
		function = func() {
			_, errCode = d.serviceCallHandler(ServiceCallMessage{
				DeviceIdentification: device,
				ServiceCallParam:     *param.Param.ServiceCallParams,
			})
			if errCode != SuccessCode {
				errDesc = "服务调用失败"
			}
		}
	case JobTypeConfigUpdate:
		d.LogReportConfig = *param.Param.ConfigUpdateParams
		d.sdkLogf(LogLevelInfo, "Log Config Change Success, new config: %#v", d.LogReportConfig)
		// 修改配置文件，如果设备重启保留配置
		errCode = d.updateConfigFile()
		if errCode != SuccessCode {
			errDesc = "更新配置文件失败"
		}
		return
	case JobTypeShadowUpdate:
		function = func() {
			errCode = d.shadowSetHandler(ShadowSetMessage{
				DeviceIdentification: device,
				ShadowDesiredParam:   *param.Param.ShadowUpdateParams,
			})
			if errCode != SuccessCode {
				errDesc = "更新设备影子失败"
			}
		}
	case JobTypeCustom:
		function = func() {
			errCode, errDesc = d.customJobHandler(CustomJobMessage{
				DeviceIdentification: device,
				Param:                param.Param.CustomParams.Param,
				FilePath:             formatDownloadFilePath(d.FilePath, param.Param.CustomParams.URL),
			})
		}
	}

	err := d.panicToError("execTask", function)
	if err != nil {
		errCode = ErrorCodeFuncExecPanic
		errDesc = "方法执行panic"
	}
	return
}

func (d *Device) downloadTaskFile(device DeviceIdentification, param TaskPayloadParams) ErrorCode {
	customParam := param.Param.CustomParams
	downloadPath := formatDownloadFilePath(d.FilePath, customParam.URL)
	doneChannel := make(chan error, 1)
	jobKey := fmt.Sprintf("%s|%s|%s", device.ProductKey, device.DeviceName, param.JobID)
	value, ok := d.executingJobMap.Load(jobKey)
	if !ok {
		d.sdkLogf(LogLevelError, "job %s not found in executing job", jobKey)
		return JobErrorCodeJobNotFound
	}
	executingJob := value.(*ExecutingJob)

	var err error
	// 启动协程下载，并传递chan做超时处理
	fileInfo, err := os.Stat(downloadPath)
	var offset int64
	if err == nil {
		offset = fileInfo.Size()
	}
	downloader := NewFileDownloader(customParam.URL, downloadPath, offset, param.Param.CustomParams.FileSize)
	go func() {
		err = downloader.Download()
		doneChannel <- err
	}()
	select {
	case err = <-doneChannel:
		if err != nil {
			d.sdkLogf(LogLevelError, "Download Error:%s", err.Error())
			return JobErrorCodeDownloadFailed
		}
		// 对文件做签名校验
		ok := fileCheck(downloadPath, customParam.Sign)
		if !ok {
			d.sdkLogf(LogLevelError, "MD5 not match, filepath: %s, sign: %s", downloadPath, customParam.Sign)
			return JobErrorCodeFileSignCheckFailed
		}
	case <-executingJob.jobCancelChannel:
		d.sdkLog(LogLevelInfo, "Task Canceled")
		return JobErrorCodeCanceled
	case <-executingJob.jobTimeoutChannel:
		d.sdkLog(LogLevelInfo, "Task Timeout")
		return JobErrorCodeTimeout
	}
	return SuccessCode
}

func (d *Device) reportTaskProcess(device DeviceIdentification, param TaskProgressReportParam) ErrorCode {
	payload := TaskProgressReportPayload{
		ID:      getRandomStringWithTimeSuffix(8),
		Version: "1.0",
		Params:  param,
	}
	payloadBytes, _ := json.Marshal(payload)
	if token := d.client.Publish(fmt.Sprintf(TaskProgressTopic, device.ProductKey, device.DeviceName), 1, false, payloadBytes); token.Wait() && token.Error() != nil {
		d.sdkLogf(LogLevelError, "pub otaUpgradeProcessReport failed,clientId = %s,reply = %s, err: %s", formatMqttUserName(device.ProductKey, device.DeviceName), string(payloadBytes), token.Error().Error())
		return ErrorCodePubFailed
	}
	return SuccessCode
}

func (d *Device) createTaskNotifyMqttHandler(productKey, deviceName string) func(client paho.Client, message paho.Message) {
	return func(client paho.Client, message paho.Message) {
		go func() {
			var payload TaskNotifyPayload
			err := json.Unmarshal(message.Payload(), &payload)
			if err != nil {
				d.sdkLogf(LogLevelError, "notify task parse payload failed, payload is: %s, err is : %s", message.Payload(), err.Error())
				return
			}

			if payload.Params.Action == NotifyActionExecute {
				d.runTask(DeviceIdentification{
					ProductKey: productKey,
					DeviceName: deviceName,
				}, payload.Params, JobStatusCreated)
			} else if payload.Params.Action == NotifyActionCancel {
				// 只有当取消ID和正在进行的ID相同时，向通道中发送消息
				k := fmt.Sprintf("%s|%s|%s", productKey, deviceName, payload.Params.JobID)
				if value, ok := d.executingJobMap.Load(k); ok {
					value.(ExecutingJob).jobCancelChannel <- struct{}{}
				}
			}
		}()
	}
}

func (d *Device) createTaskGetMqttHandler(productKey, deviceName string) func(client paho.Client, message paho.Message) {
	return func(client paho.Client, message paho.Message) {
		go func() {
			var reply TaskGetReply
			err := json.Unmarshal(message.Payload(), &reply)
			if err != nil {
				d.sdkLogf(LogLevelError, "get task reply parse payload failed, payload is: %s, err is : %s", message.Payload(), err.Error())
				return
			}

			if reply.Code != SuccessCode {
				d.sdkLogf(LogLevelError, "get task failed, payload is: %s, code is %v", message.Payload(), reply.Code)
				// 拉取失败继续拉取
				d.getDeviceTask(productKey, deviceName)
				return
			}

			for _, task := range reply.Data {
				d.runTask(DeviceIdentification{
					ProductKey: productKey,
					DeviceName: deviceName,
				}, task, JobStatusCreated)
			}
		}()
	}
}

func (d *Device) createTaskProgressReplyMqttHandler(productKey, deviceName string) func(client paho.Client, message paho.Message) {
	return func(client paho.Client, message paho.Message) {
		go func() {
			var reply TaskProgressReportReply
			err := json.Unmarshal(message.Payload(), &reply)
			if err != nil {
				d.sdkLogf(LogLevelError, "report task progress parse payload failed, payload is: %s, err is : %s", message.Payload(), err.Error())
				return
			}

			if reply.Code != SuccessCode {
				d.sdkLogf(LogLevelError, "report task progress failed, payload is: %s, code is %v", message.Payload(), reply.Code)
				return
			}

			jobKey := fmt.Sprintf("%s|%s|%s", productKey, deviceName, reply.JobID)
			d.executingJobMap.Delete(jobKey)
			d.saveExecutingJob()
		}()
	}
}

func (d *Device) getDeviceTask(productKey, deviceName string) {
	payload := TaskGetPayload{}
	bytes, _ := json.Marshal(payload)
	if token := d.client.Publish(fmt.Sprintf(TaskGetTopic, productKey, deviceName), 0, false, bytes); token.Wait() && token.Error() != nil {
		d.sdkLogf(LogLevelInfo, "pub delay reply failed,clientId = %s,err = %s", formatMqttUserName(productKey, deviceName), token.Error().Error())
	}
}
