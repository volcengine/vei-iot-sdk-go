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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"
)

type OTA interface {
	SetOTANotifyHandler(handler OTANotifyHandler)
	SetOTAReplyHandler(handler OTAReplyHandler)
	SetDiffAlgorithm(diffAlgorithm DiffAlgorithm)
	SetInstallAlgorithm(installAlgorithm InstallAlgorithm)
	SetSubDeviceCommonInstallAlgorithm(subDeviceCommonInstallAlgorithm SubDeviceCommonInstallAlgorithm)
	SetSubDeviceCustomInstallAlgorithm(subDeviceCustomInstallAlgorithm map[string]SubDeviceCustomInstallAlgorithm)
	SetQueryUpgradeConfig(config QueryUpgradeConfig)
	OTAVersionReport(versionMap map[string]string, deviceIdentification ...DeviceIdentification) ErrorCode
	OTAUpgradeRequest(module, otaJobID string, deviceIdentification ...DeviceIdentification) ErrorCode
	InitOTAJob(deviceIdentification DeviceIdentification, data OTAReplyData) (ErrorCode, OTAMetadata)
	OTADownload(module string, deviceIdentification ...DeviceIdentification) ErrorCode
	OTAInstall(module string, deviceIdentification ...DeviceIdentification) ErrorCode
	GetOTAInfo(otaModuleIdentification []OTAModuleIdentification) map[string]JobInfo
}

// GetOTAInfo 查询设备的OTA升级任务相关信息
func (d *Device) GetOTAInfo(otaModuleIdentification []OTAModuleIdentification) map[string]JobInfo {
	otaInfo := make(map[string]JobInfo, 0)
	for _, v := range otaModuleIdentification {
		otaModuleKey := formatOTAModuleKey(v.ProductKey, v.DeviceName, v.ModuleName)
		if value, ok := d.otaCurrentJob.Load(otaModuleKey); ok {
			job := value.(OTACurrentJobInfo)
			otaInfo[otaModuleKey] = job.Info
		}
	}
	return otaInfo
}

// SetOTANotifyHandler 配置处理OTA升级通知消息的Handler
// 传递 OTANotifyHandler
func (d *Device) SetOTANotifyHandler(handler OTANotifyHandler) {
	d.otaNotifyHandler = handler
}

// SetOTAReplyHandler 配置处理OTA请求回复消息的Handler
// 传递 OTAReplyHandler
func (d *Device) SetOTAReplyHandler(handler OTAReplyHandler) {
	d.otaReplyHandler = handler
}

// SetDiffAlgorithm 配置差分还原算法，用于安装前还原差分包，如果安装包为差分包，必须配置
// 传递 DiffAlgorithm
func (d *Device) SetDiffAlgorithm(diffAlgorithm DiffAlgorithm) {
	d.diffAlgorithm = diffAlgorithm
}

// SetInstallAlgorithm 配置安装算法，必须配置，用于安装升级包
// 传递 InstallAlgorithm
func (d *Device) SetInstallAlgorithm(installAlgorithm InstallAlgorithm) {
	d.installAlgorithm = installAlgorithm
}

// SetSubDeviceCommonInstallAlgorithm 配置子设备通用安装算法，网关设备必须配置，用于网关设备处理子设备安装相关流程
// 传递 SubDeviceCommonInstallAlgorithm
func (d *Device) SetSubDeviceCommonInstallAlgorithm(subDeviceCommonInstallAlgorithm SubDeviceCommonInstallAlgorithm) {
	d.subDeviceCommonInstallAlgorithm = subDeviceCommonInstallAlgorithm
}

// SetSubDeviceCustomInstallAlgorithm 配置子设备定制化安装算法，即针对不同产品的子设备，采用不同的定制化安装方法，不配置则采用默认的通用安装算法
// 传递 SubDeviceCustomInstallAlgorithm
func (d *Device) SetSubDeviceCustomInstallAlgorithm(subDeviceCustomInstallAlgorithm map[string]SubDeviceCustomInstallAlgorithm) {
	d.subDeviceCustomInstallAlgorithm = subDeviceCustomInstallAlgorithm
}

// SetQueryUpgradeConfig 配置定时轮询OTA升级任务配置
// 传递 QueryUpgradeConfig
func (d *Device) SetQueryUpgradeConfig(config QueryUpgradeConfig) {
	d.queryUpgradeConfig = &config
}

// OTAVersionReport 主动调用，进行OTA模块版本信息上报
// 传递 versionMap key为OTA模块标识 value为对应模块的版本号; 为子设备进行OTA模块版本信息上报还需要指定子设备标识 DeviceIdentification
// 返回 成功标识
func (d *Device) OTAVersionReport(versionMap map[string]string, deviceIdentification ...DeviceIdentification) ErrorCode {
	// 为子设备进行OTA模块版本信息上报需要指定子设备标识，否则默认为设备自身进行OTA模块版本信息上报
	productKey := d.ProductKey
	deviceName := d.Name
	if len(deviceIdentification) == 1 {
		productKey = deviceIdentification[0].ProductKey
		deviceName = deviceIdentification[0].DeviceName
	}
	if len(deviceIdentification) > 1 {
		return ErrorCodeParamNotValid
	}
	// 组装消息
	payload := OTAVersionReportPayload{
		ID:      getRandomStringWithTimeSuffix(8),
		Version: "1.0",
		Params:  versionMap,
	}

	payloadBytes, _ := json.Marshal(payload)
	if token := d.client.Publish(fmt.Sprintf(OTAVersionReportTopic, productKey, deviceName), 0, false, payloadBytes); token.Wait() && token.Error() != nil {
		d.sdkLogf(LogLevelError, "pub OTAVersion failed,clientId = %s,reply = %s, err: %s", formatMqttUserName(d.ProductKey, d.Name), string(payloadBytes), token.Error())
		return ErrorCodePubFailed
	}
	return SuccessCode
}

// OTAUpgradeRequest 主动调用，进行OTA升级任务信息请求
// 传递 module模块标识; 若otaJobID为空，表示请求最新的升级任务，建议不指定；为子设备请求升级还需要指定子设备标识 DeviceIdentification
// 返回 成功标识
func (d *Device) OTAUpgradeRequest(module, otaJobID string, deviceIdentification ...DeviceIdentification) ErrorCode {
	// 为子设备请求升级需要指定子设备标识，否则默认为设备自身请求升级
	productKey := d.ProductKey
	deviceName := d.Name
	if len(deviceIdentification) == 1 {
		productKey = deviceIdentification[0].ProductKey
		deviceName = deviceIdentification[0].DeviceName
	}
	if len(deviceIdentification) > 1 || module == "" {
		return ErrorCodeParamNotValid
	}
	// 组装消息
	payload := OTAUpgradeRequestPayload{
		ID:      getRandomStringWithTimeSuffix(8),
		Version: "1.0",
		Params: UpgradeRequestParam{
			OTAJobID:   otaJobID,
			SrcVersion: d.ModuleConfig[module].Version,
			Module:     module,
		},
	}

	if productKey != d.ProductKey || deviceName != d.Name {
		payload.Params.SrcVersion = d.SubDeviceModuleConfig[formatOTAModuleKey(productKey, deviceName, module)].Version
	}

	payloadBytes, _ := json.Marshal(payload)
	if token := d.client.Publish(fmt.Sprintf(OTARequestUpgradeTopic, productKey, deviceName), 1, false, payloadBytes); token.Wait() && token.Error() != nil {
		d.sdkLogf(LogLevelError, "pub OTAUpgradeRequest failed,clientId = %s,reply = %s, err: %s", formatMqttUserName(d.ProductKey, d.Name), string(payloadBytes), token.Error())
		return ErrorCodePubFailed
	}
	return SuccessCode
}

// otaUpgradeProcessReport 主动调用，进行OTA升级任务进程信息上报
// 传递 otaJobID OTA升级任务ID，param ProcessReportParam，deviceIdentification 设备/子设备标识
// 返回 成功标识
func (d *Device) otaUpgradeProcessReport(otaJobID string, param ProcessReportParam, deviceIdentification DeviceIdentification) ErrorCode {
	param.Time = currentTimeMill()
	payload := OTAUpgradeProcessReportPayload{
		ID:      getRandomStringWithTimeSuffix(8),
		Version: "1.0",
		Params:  param,
	}
	payloadBytes, _ := json.Marshal(payload)
	if token := d.client.Publish(fmt.Sprintf(OTAUpgradeProgressReportTopic, deviceIdentification.ProductKey, deviceIdentification.DeviceName, otaJobID), 1, false, payloadBytes); token.Wait() && token.Error() != nil {
		d.sdkLogf(LogLevelError, "pub otaUpgradeProcessReport failed,clientId = %s,reply = %s, err: %s", formatMqttUserName(d.ProductKey, d.Name), string(payloadBytes), token.Error())
		return ErrorCodePubFailed
	}
	return SuccessCode
}

// InitOTAJob 主动调用，初始化OTA升级任务
// 传递 DeviceIdentification 为子设备升级需要指定子设备身份标识；OTAReplyData 升级任务相关参数
func (d *Device) InitOTAJob(deviceIdentification DeviceIdentification, data OTAReplyData) (ErrorCode, OTAMetadata) {
	otaMetadata := OTAMetadata{
		ProductKey:    deviceIdentification.ProductKey,
		DeviceName:    deviceIdentification.DeviceName,
		OTAModuleName: data.Module,
		OTAJobID:      data.OTAJobID,
	}

	if data.OTAJobID == "" {
		return ErrorCodeAvailableOTAJobNotFound, otaMetadata
	}

	otaModuleKey := formatOTAModuleKey(deviceIdentification.ProductKey, deviceIdentification.DeviceName, data.Module)

	// 检查该模块是否有正在升级的任务
	if _, ok := d.otaCurrentJob.Load(otaModuleKey); ok {
		return ErrorCodeOTAJobLocked, otaMetadata
	}

	// 更新历史任务文件
	defer func() {
		err := d.updateJobHistoryFile()
		if err != nil {
			d.sdkLogf(LogLevelError, "Update OTA Job History error:%s", err)
		}
	}()

	// 如果设备请求升级时指定了历史任务，但云端未返回该任务，说明该历史任务已不可用，需要及时删除，否则会影响新任务
	if data.OTAJobID != "" && data.Url == "" {
		d.otaHistoryJob.Delete(otaModuleKey)
		return ErrorCodeAvailableOTAJobNotFound, otaMetadata
	}

	jobInfo, ok := d.otaHistoryJob.Load(otaModuleKey)

	historyJob := JobInfo{}
	if ok {
		historyJob = jobInfo.(JobInfo)
		// 1.该任务是历史任务/该任务与历史任务冲突但配置为历史任务优先，则恢复历史任务至otaCurrentJob
		if historyJob.OTAJobID == data.OTAJobID || (historyJob.OTAJobID != data.OTAJobID && d.OTAHistoryJobReloadMode == OTAReloadModeHistoryJobPriority) {
			otaMetadata.RetryTime = historyJob.RetryTime
			otaMetadata.Step = historyJob.Step
			// 重试超过5次，则删除
			if historyJob.RetryTime >= 5 {
				d.otaHistoryJob.Delete(otaModuleKey)
				return ErrorCodeAvailableOTAJobNotFound, otaMetadata
			}

			historyJob.DurationInMinutes = 0
			historyJob.RetryTime += 1
			historyJob.OTAReplyData = data
			d.otaCurrentJob.Store(otaModuleKey, OTACurrentJobInfo{
				Info:             historyJob,
				JobCancelChannel: make(chan struct{}),
			})
			d.sdkLogf(LogLevelInfo, "OTA history job restart,jobID:%s", historyJob.OTAJobID)
			return SuccessCode, otaMetadata
		}
	}

	// 2.该模块没有历史任务/该任务与历史任务冲突但配置为新任务覆盖，则初始化新任务至otaCurrentJob
	if !ok || (historyJob.OTAJobID != data.OTAJobID && d.OTAHistoryJobReloadMode == OTAReloadModeNewJobCoverage) {
		d.otaHistoryJob.Delete(otaModuleKey)
		// 初始化新任务
		newJob := OTACurrentJobInfo{
			Info: JobInfo{
				DeviceIdentification: deviceIdentification,
				OTAJobID:             data.OTAJobID,
				DurationInMinutes:    0,
				Step:                 string(UpgradeStepEnumToUpgrade),
				RetryTime:            0,
				OTAReplyData:         data,
			},
			JobCancelChannel: make(chan struct{}),
		}

		// 判断设备自身升级 or 子设备升级
		isUpgradeForSubDevice := false
		if deviceIdentification.ProductKey != d.ProductKey || deviceIdentification.DeviceName != d.Name {
			isUpgradeForSubDevice = true
		}

		// 查询自定义OTA模块/创建default模块
		var module *ModuleInfo
		tmpModuleConfig := d.ModuleConfig
		tmpModule := data.Module
		tmpDefaultModule := OTADefaultModule
		if isUpgradeForSubDevice {
			tmpModuleConfig = d.SubDeviceModuleConfig
			tmpModule = formatOTAModuleKey(newJob.Info.ProductKey, newJob.Info.DeviceName, newJob.Info.Module)
			tmpDefaultModule = formatOTAModuleKey(newJob.Info.ProductKey, newJob.Info.DeviceName, OTADefaultModule)
		}
		for _, moduleInfo := range tmpModuleConfig {
			if moduleInfo.ModuleKey == tmpModule {
				module = &moduleInfo
			}
		}
		// 默认创建default模块
		if data.Module == OTADefaultModule && module == nil {
			module = &ModuleInfo{
				ModuleKey: tmpDefaultModule,
			}
			if isUpgradeForSubDevice {
				d.SubDeviceModuleConfig[tmpDefaultModule] = *module
			} else {
				d.ModuleConfig[tmpDefaultModule] = *module
			}
		}

		if module == nil {
			d.sdkLogf(LogLevelError, "upgrade module not exist; module:%s", data.Module)
			param := ProcessReportParam{
				Status:     UpgradeDeviceStatusFailed,
				ResultCode: OTAErrorCodeModuleNotExist,
				ResultDesc: fmt.Sprintf("The upgrade module %s not exist.", data.Module),
			}
			code := d.otaUpgradeProcessReport(newJob.Info.OTAJobID, param, newJob.Info.DeviceIdentification)
			if code != SuccessCode {
				d.sdkLogf(LogLevelError, "[InitOTAJob] OTA process report failed,job:%s, code: %v", data.OTAJobID, code)
			}
			return ErrorCodeModuleNotExist, otaMetadata
		}
		d.otaCurrentJob.Store(otaModuleKey, newJob)
		otaMetadata.Step = newJob.Info.Step
		return SuccessCode, otaMetadata
	}

	return ErrorCodeAvailableOTAJobNotFound, otaMetadata
}

// OTADownload 下载升级包
// 传递 module OTA模块；为子设备下载安装包还需要指定子设备标识 DeviceIdentification
func (d *Device) OTADownload(module string, deviceIdentification ...DeviceIdentification) ErrorCode {
	productKey := d.ProductKey
	deviceName := d.Name
	if len(deviceIdentification) == 1 {
		productKey = deviceIdentification[0].ProductKey
		deviceName = deviceIdentification[0].DeviceName
	}
	if len(deviceIdentification) > 1 {
		return ErrorCodeParamNotValid
	}

	startTime := time.Now()
	otaModuleKey := formatOTAModuleKey(productKey, deviceName, module)
	value, ok := d.otaCurrentJob.Load(otaModuleKey)
	if !ok {
		return ErrorCodeAvailableOTAJobNotFound
	}
	job := value.(OTACurrentJobInfo)

	// 进入下载流程，先上报升级进度为"下载中"
	param := ProcessReportParam{
		Status:     UpgradeDeviceStatusDownloading,
		ResultCode: SuccessCode,
	}
	code := d.otaUpgradeProcessReport(job.Info.OTAJobID, param, job.Info.DeviceIdentification)
	if code != SuccessCode {
		d.sdkLogf(LogLevelError, "[OTADownload] OTA process report failed,job:%s, code: %v", job.Info.OTAJobID, code)
	}
	job.Info.Step = string(UpgradeStepEnumDownloading)

	defer func() {
		job.Info.DurationInMinutes += int64(time.Now().Minute() - startTime.Minute())

		// 处理otaHistoryJob：任务取消，需要删除；否则进行断点保存
		if job.Info.StepErrorCode == ErrorCodeUpgradeCanceled {
			d.otaHistoryJob.Delete(otaModuleKey)
		} else {
			d.otaHistoryJob.Store(otaModuleKey, job.Info)
		}

		// 处理otaCurrentJob：下载成功，更新otaCurrentJob；否则需要删除
		if job.Info.StepErrorCode == SuccessCode {
			d.otaCurrentJob.Store(otaModuleKey, job)
		} else {
			d.otaCurrentJob.Delete(otaModuleKey)
		}

		// 更新历史任务文件
		err := d.updateJobHistoryFile()
		if err != nil {
			d.sdkLogf(LogLevelError, "[OTADownload] Update OTA Job History error:%s", err)
		}

		// 上报下载结果
		if job.Info.StepErrorCode != ErrorCodeUpgradeCanceled {
			code = d.otaUpgradeProcessReport(job.Info.OTAJobID, param, job.Info.DeviceIdentification)
			if code != SuccessCode {
				d.sdkLogf(LogLevelError, "[OTADownload] OTA process report failed,job: %s, code: %v", job.Info.OTAJobID, code)
			}
		}
	}()

	// 获取下载url
	cryptedURL, err := base64.StdEncoding.DecodeString(job.Info.Url)
	if err != nil {
		d.sdkLogf(LogLevelError, "url decode error:%s", err)
		return ErrorCodeDecodeFailed
	}
	secret := d.Secret
	if job.Info.ProductKey != d.ProductKey || job.Info.DeviceName != d.Name {
		v, ok := d.subDeviceSecretMap.Load(formatMqttUserName(job.Info.ProductKey, job.Info.DeviceName))
		if !ok {
			d.sdkLogf(LogLevelError, "subDevice secret get failed,subDevice:%+v", job.Info.DeviceIdentification)
			return GatewayErrorCodeTopoNotExist
		}
		secret = v.(string)
	}
	url, err := aesDecrypt(cryptedURL, []byte(secret)[:16])
	if err != nil {
		d.sdkLogf(LogLevelError, "url decode error:%s", err)
		return ErrorCodeDecodeFailed
	}
	job.Info.Url = string(url)
	job.Info.OTAPackageFile = formatDownloadFilePath(d.FilePath, job.Info.Url)

	// 如果设置了超时时间，启动协程，计时发送timeout信号
	jobTimeoutChannel := make(chan struct{})
	if job.Info.TimeoutInMinutes != 0 {
		go func() {
			duration := time.Minute * time.Duration(job.Info.TimeoutInMinutes-job.Info.DurationInMinutes)
			select {
			case <-time.After(duration):
			case <-d.ctx.Done():
			}
			jobTimeoutChannel <- struct{}{}
		}()
	}

	fileInfo, err := os.Stat(job.Info.OTAPackageFile)
	var offset int64
	if err == nil {
		offset = fileInfo.Size()
	}
	downloader := NewFileDownloader(job.Info.Url, job.Info.OTAPackageFile, offset, job.Info.Size)
	doneChannel := make(chan error, 1)

	// 启动协程下载，并传递chan做超时处理
	go func() {
		err = downloader.Download()
		doneChannel <- err
	}()
	select {
	case err = <-doneChannel:
		if err != nil {
			d.sdkLogf(LogLevelError, "Download Error:%s", err)
			param = ProcessReportParam{
				Status:     UpgradeDeviceStatusFailed,
				ResultCode: OTAErrorCodeDownloadFailed,
			}
			job.Info.StepErrorCode = param.ResultCode
			return job.Info.StepErrorCode
		}
		d.sdkLogf(LogLevelInfo, "Download Complete, Time: %f second", time.Now().Sub(startTime).Seconds())

		// 对文件做签名校验
		ok = fileCheck(job.Info.OTAPackageFile, job.Info.Sign)
		if !ok {
			d.sdkLog(LogLevelError, "MD5 Check Error")
			param = ProcessReportParam{
				Status:     UpgradeDeviceStatusFailed,
				ResultCode: OTAErrorCodeCheckFailed,
			}
			job.Info.StepErrorCode = param.ResultCode
			return job.Info.StepErrorCode
		}

		param = ProcessReportParam{
			Status:     UpgradeDeviceStatusDownloaded,
			ResultCode: SuccessCode,
		}
		job.Info.Step = string(UpgradeStepEnumDownloaded)
		job.Info.StepErrorCode = param.ResultCode
		return job.Info.StepErrorCode

	case <-job.JobCancelChannel:
		d.sdkLog(LogLevelInfo, "Update Canceled")
		job.Info.StepErrorCode = ErrorCodeUpgradeCanceled
		return job.Info.StepErrorCode

	case <-jobTimeoutChannel:
		d.sdkLog(LogLevelInfo, "Update Timeout")
		param = ProcessReportParam{
			Status:     UpgradeDeviceStatusFailed,
			ResultCode: OTAErrorCodeTimeout,
		}
		job.Info.StepErrorCode = param.ResultCode
		return job.Info.StepErrorCode

	}
}

// OTADownload 安装升级包
// 传递 module OTA模块；为子设备进行安装还需要指定子设备标识 DeviceIdentification
func (d *Device) OTAInstall(module string, deviceIdentification ...DeviceIdentification) ErrorCode {
	productKey := d.ProductKey
	deviceName := d.Name
	if len(deviceIdentification) == 1 {
		productKey = deviceIdentification[0].ProductKey
		deviceName = deviceIdentification[0].DeviceName
	}
	if len(deviceIdentification) > 1 {
		return ErrorCodeParamNotValid
	}

	startTime := time.Now()
	otaModuleKey := formatOTAModuleKey(productKey, deviceName, module)
	value, ok := d.otaCurrentJob.Load(otaModuleKey)
	if !ok {
		return ErrorCodeAvailableOTAJobNotFound
	}
	job := value.(OTACurrentJobInfo)

	// 进入安装流程，先上报升级进度为"安装中"
	param := ProcessReportParam{
		Status:     UpgradeDeviceStatusInstalling,
		ResultCode: SuccessCode,
	}
	code := d.otaUpgradeProcessReport(job.Info.OTAJobID, param, job.Info.DeviceIdentification)
	if code != SuccessCode {
		d.sdkLogf(LogLevelError, "[OTAInstall] OTA process report failed,job:%s", job.Info.OTAJobID)
	}
	job.Info.Step = string(UpgradeStepEnumInstalling)

	// 判断设备自身升级 or 子设备升级
	isUpgradeForSubDevice := false
	if job.Info.ProductKey != d.ProductKey || job.Info.DeviceName != d.Name {
		isUpgradeForSubDevice = true
	}

	defer func() {
		job.Info.DurationInMinutes += int64(time.Now().Minute() - startTime.Minute())

		// 处理otaHistoryJob：任务取消/升级成功，删除；否则进行断点保存
		if job.Info.StepErrorCode == ErrorCodeUpgradeCanceled || job.Info.Step == string(UpgradeStepEnumSuccess) {
			d.otaHistoryJob.Delete(otaModuleKey)
		} else {
			d.otaHistoryJob.Store(otaModuleKey, job.Info)
		}

		// 处理otaCurrentJob：安装流程结束，删除otaCurrentJob，但是对于安装完成后需要重启生效的设备，需要重启才能结束该任务
		if job.Info.Step != string(UpgradeStepEnumInstalled) {
			d.otaCurrentJob.Delete(otaModuleKey)
		}

		// 更新历史任务文件
		err := d.updateJobHistoryFile()
		if err != nil {
			d.sdkLogf(LogLevelError, "[OTAInstall] Update OTA Job History error:%s", err)
		}

		// 对于升级成功的任务，需要更新配置文件的版本号信息
		if job.Info.Step == string(UpgradeStepEnumSuccess) {
			if isUpgradeForSubDevice {
				moduleKey := formatOTAModuleKey(job.Info.ProductKey, job.Info.DeviceName, job.Info.Module)
				moduleInfo := d.SubDeviceModuleConfig[moduleKey]
				moduleInfo.LastFileName = moduleInfo.FileName
				moduleInfo.LastVersion = moduleInfo.Version
				moduleInfo.FileName = job.Info.InstallPackageFile
				moduleInfo.Version = job.Info.DestVersion
				d.SubDeviceModuleConfig[moduleKey] = moduleInfo
			} else {
				moduleInfo := d.ModuleConfig[job.Info.Module]
				moduleInfo.LastFileName = moduleInfo.FileName
				moduleInfo.LastVersion = moduleInfo.Version
				moduleInfo.FileName = job.Info.InstallPackageFile
				moduleInfo.Version = job.Info.DestVersion
				d.ModuleConfig[job.Info.Module] = moduleInfo
			}
			code = d.updateConfigFile()
			if code != SuccessCode {
				d.sdkLogf(LogLevelError, "[OTAInstall] update config file failed:%d", code)
			}
		}

		// 上报安装结果
		if job.Info.StepErrorCode != ErrorCodeUpgradeCanceled {
			code = d.otaUpgradeProcessReport(job.Info.OTAJobID, param, job.Info.DeviceIdentification)
			if code != SuccessCode {
				d.sdkLogf(LogLevelError, "[OTAInstall] OTA process report failed,job:%s", job.Info.OTAJobID)
			}
		}
	}()

	// 如果设置了超时时间，启动协程，计时发送timeout信号
	jobTimeoutChannel := make(chan struct{})
	if job.Info.TimeoutInMinutes != 0 {
		go func() {
			duration := time.Minute * time.Duration(job.Info.TimeoutInMinutes-job.Info.DurationInMinutes)
			select {
			case <-time.After(duration):
			case <-d.ctx.Done():
			}
			jobTimeoutChannel <- struct{}{}
		}()
	}

	// 启动协程安装，安装完毕传递chan
	doneChannel := make(chan InstallAlgorithmResult)
	res := InstallAlgorithmResult{}
	go func() {
		// 子设备升级安装流程
		if isUpgradeForSubDevice {
			job.Info.InstallPackageFile = job.Info.OTAPackageFile
			algorithmParam := SubDeviceInstallParam{
				ProductKey: job.Info.ProductKey,
				DeviceName: job.Info.DeviceName,
				OTAModule:  job.Info.Module,
				FileName:   job.Info.OTAPackageFile,
			}
			var err error
			if algorithm, ok := d.subDeviceCustomInstallAlgorithm[job.Info.ProductKey]; ok {
				err = d.panicToError("OTAInstall", func() {
					res = algorithm(algorithmParam)
				})
			} else {
				err = d.panicToError("OTAInstall", func() {
					res = d.subDeviceCommonInstallAlgorithm(algorithmParam)
				})
			}
			if err != nil {
				res = InstallAlgorithmResult{
					IsSuccess: false,
					ResDesc:   "function panic",
				}
			}
			doneChannel <- res
		}
		// 设备自身升级安装流程
		if !isUpgradeForSubDevice {
			// 差分还原
			if job.Info.IsDiff && job.Info.Step != string(UpgradeStepEnumDiffRecovered) {
				job.Info.Step = string(UpgradeStepEnumDiffRecovering)
				var diffAlgorithmResult DiffAlgorithmResult
				err := d.panicToError("OTAInstall", func() {
					diffAlgorithmResult = d.diffAlgorithm(job.Info.OTAPackageFile)
				})
				if err != nil {
					diffAlgorithmResult = DiffAlgorithmResult{
						IsSuccess: false,
						ResDesc:   "function panic",
					}
				}
				if !diffAlgorithmResult.IsSuccess || diffAlgorithmResult.TargetFilePath == "" {
					d.sdkLog(LogLevelError, "差分还原失败")
					param = ProcessReportParam{
						Status:     UpgradeDeviceStatusFailed,
						ResultCode: OTAErrorCodeDiffRecoverFailed,
						ResultDesc: diffAlgorithmResult.ResDesc,
					}
					job.Info.StepErrorCode = param.ResultCode
					doneChannel <- res
					return
				}

				d.sdkLogf(LogLevelInfo, "文件差分还原完成耗时: %f second", time.Now().Sub(startTime).Seconds())
				job.Info.InstallPackageFile = diffAlgorithmResult.TargetFilePath
				job.Info.Step = string(UpgradeStepEnumDiffRecovered)
				job.Info.StepErrorCode = SuccessCode
			}
			if job.Info.InstallPackageFile == "" {
				job.Info.InstallPackageFile = job.Info.OTAPackageFile
			}
			// 安装
			err := d.panicToError("OTAInstall", func() {
				res = d.installAlgorithm(job.Info.Module, job.Info.InstallPackageFile)
			})
			if err != nil {
				res = InstallAlgorithmResult{
					IsSuccess: false,
					ResDesc:   "function panic",
				}
			}
			doneChannel <- res
		}
	}()

	select {
	case <-job.JobCancelChannel:
		d.sdkLog(LogLevelInfo, "Update Canceled")
		job.Info.StepErrorCode = ErrorCodeUpgradeCanceled
		return job.Info.StepErrorCode

	case <-jobTimeoutChannel:
		d.sdkLog(LogLevelInfo, "Update Timeout")
		param = ProcessReportParam{
			Status:     UpgradeDeviceStatusFailed,
			ResultCode: OTAErrorCodeTimeout,
		}
		job.Info.StepErrorCode = param.ResultCode
		return job.Info.StepErrorCode

	case <-doneChannel:
		if job.Info.Step == string(UpgradeStepEnumDiffRecovering) {
			return job.Info.StepErrorCode
		}
		if !(res.IsSuccess && res.DeviceVersion == job.Info.DestVersion) {
			d.sdkLog(LogLevelError, "Installing OTA package error")
			param = ProcessReportParam{
				Status:     UpgradeDeviceStatusFailed,
				ResultCode: OTAErrorCodeInstallFailed,
				ResultDesc: res.ResDesc,
			}
			job.Info.StepErrorCode = param.ResultCode
			return job.Info.StepErrorCode
		}
		param = ProcessReportParam{
			Status:     UpgradeDeviceStatusInstalled,
			ResultCode: SuccessCode,
			ResultDesc: res.ResDesc,
		}
		d.sdkLogf(LogLevelInfo, "文件安装完成耗时: %f second", time.Now().Sub(startTime).Seconds())
		job.Info.Step = string(UpgradeStepEnumInstalled)
		job.Info.StepErrorCode = param.ResultCode

		// 子设备升级或者是需要重启的设备自身升级，结束安装流程，否则还需要上报升级成功并更新版本号信息
		if res.NeedReboot && !isUpgradeForSubDevice {
			return job.Info.StepErrorCode
		}

		param = ProcessReportParam{
			Status:     UpgradeDeviceStatusSuccess,
			ResultCode: SuccessCode,
		}
		job.Info.Step = string(UpgradeStepEnumSuccess)
		job.Info.StepErrorCode = param.ResultCode
		return job.Info.StepErrorCode
	}
}

func (d *Device) queryUpgradeJob() {
	// 先判断用户是否配置了轮训配置，如果没有配置，则配置为默认，0点执行，24小时执行一次
	if d.queryUpgradeConfig == nil {
		d.queryUpgradeConfig = &QueryUpgradeConfig{
			TimeInterval: 24,
			StartTime:    0,
			EndTime:      24,
		}
	}
	// 先找到最近的开始时间
	now := time.Now()
	var start time.Time
	if now.Hour() < d.queryUpgradeConfig.StartTime {
		start = time.Date(now.Year(), now.Month(), now.Day(), d.queryUpgradeConfig.StartTime, 0, 0, 0, now.Location())
	} else {
		start = time.Date(now.Year(), now.Month(), now.Day()+1, d.queryUpgradeConfig.StartTime, 0, 0, 0, now.Location())
	}
	t := time.NewTimer(start.Sub(now))
	select {
	case <-t.C:
	case <-d.ctx.Done():
		return
	}
	for {
		now = time.Now()
		// 如果不在区间之中，不执行
		if now.Hour() > d.queryUpgradeConfig.EndTime || now.Hour() < d.queryUpgradeConfig.StartTime {
			continue
		}
		for _, info := range d.ModuleConfig {
			d.OTAUpgradeRequest(info.ModuleKey, "")
		}
		// 定时查询每个模块是否有更新任务
		now = time.Now()                                                                               // 获取当前时间，放到now里面，要给next用
		next := now.Add(time.Hour * time.Duration(d.queryUpgradeConfig.TimeInterval))                  // 通过now偏移interval小时
		next = time.Date(next.Year(), next.Month(), next.Day(), next.Hour(), 0, 0, 0, next.Location()) // 获取下一个运行时刻
		t = time.NewTimer(next.Sub(now))                                                               // 计算当前时间到下一个运行时刻的时间间隔，设置一个定时器
		select {
		case <-t.C:
		case <-d.ctx.Done():
			return
		}
	}
}

func (d *Device) reloadHistoryOTAJob() {
	historyBytes, err := readFile(filepath.Join(d.FilePath, otaJobHistoryPath), filepath.Join(d.FilePath, md5CodeStorageFile))
	if err != nil {
		d.sdkLogf(LogLevelError, "Read OTA Job History Failed! Error:%s", err)
		return
	}

	if len(historyBytes) == 0 {
		return
	}
	var otaJobHistory map[string]JobInfo
	err = json.Unmarshal(historyBytes, &otaJobHistory)
	if err != nil {
		d.sdkLogf(LogLevelError, "Unmarshal OTA Job History Content Failed! Error:%s", err)
		return
	}

	for k, job := range otaJobHistory {
		// 1.设备重启后，对状态为安装完成的升级任务，需要上报升级成功，并更新配置文件中相应的OTA模块版本信息
		if job.Step == string(UpgradeDeviceStatusInstalled) {
			param := ProcessReportParam{
				Status:     UpgradeDeviceStatusSuccess,
				ResultCode: SuccessCode,
			}
			code := d.otaUpgradeProcessReport(job.OTAJobID, param, job.DeviceIdentification)
			if code != SuccessCode {
				d.sdkLogf(LogLevelError, "[ReloadHistoryOTAJob] OTA process report failed, code: %v", code)
			} else {
				d.sdkLogf(LogLevelInfo, "[ReloadHistoryOTAJob] OTA process report success,job:%s", job.OTAJobID)
			}
			if job.ProductKey != d.ProductKey || job.DeviceName != d.Name {
				moduleKey := formatOTAModuleKey(job.ProductKey, job.DeviceName, job.Module)
				moduleInfo := d.SubDeviceModuleConfig[moduleKey]
				moduleInfo.LastFileName = moduleInfo.FileName
				moduleInfo.LastVersion = moduleInfo.Version
				moduleInfo.FileName = job.InstallPackageFile
				moduleInfo.Version = job.DestVersion
				d.SubDeviceModuleConfig[moduleKey] = moduleInfo
			} else {
				moduleInfo := d.ModuleConfig[job.Module]
				moduleInfo.LastFileName = moduleInfo.FileName
				moduleInfo.LastVersion = moduleInfo.Version
				moduleInfo.FileName = job.InstallPackageFile
				moduleInfo.Version = job.DestVersion
				d.ModuleConfig[job.Module] = moduleInfo
			}
			code = d.updateConfigFile()
			if code != SuccessCode {
				d.sdkLogf(LogLevelError, "[ReloadHistoryOTAJob] update config file failed, code: %d", code)
			}
		}

		// 2.若配置为历史任务优先，需要向云端发起指定该历史任务的升级请求
		if d.OTAHistoryJobReloadMode == OTAReloadModeHistoryJobPriority {
			// 向云端发起对该历史任务的升级请求
			code := d.OTAUpgradeRequest(job.Module, job.OTAJobID)
			if code != SuccessCode {
				d.sdkLogf(LogLevelError, "[ReloadHistoryOTAJob] OTA upgrade request failed, code: %d", code)
			} else {
				d.sdkLogf(LogLevelInfo, "[ReloadHistoryOTAJob] request OTA job with HistoryJobPriority,the history job is:%s", job.OTAJobID)
			}
			d.otaHistoryJob.Store(k, job)
		}

		// 3.若配置为新任务覆盖，需要向云端发起升级请求，无需指定升级任务，平台返回可用的最新升级任务
		if d.OTAHistoryJobReloadMode == OTAReloadModeNewJobCoverage {
			// 向云端发起升级请求，不指定升级任务
			code := d.OTAUpgradeRequest(job.Module, "")
			if code != SuccessCode {
				d.sdkLogf(LogLevelError, "[ReloadHistoryOTAJob] OTA upgrade request failed, code: %d", code)
			} else {
				d.sdkLogf(LogLevelInfo, "[ReloadHistoryOTAJob] request OTA job with NewJobCoverage,the history job is:%s", job.OTAJobID)
			}
			d.otaHistoryJob.Store(k, job)
		}
	}

	err = d.updateJobHistoryFile()
	if err != nil {
		d.sdkLogf(LogLevelError, "Update OTA Job History Content Failed! Error:%s", err)
		return
	}
}

func (d *Device) updateConfigFile() ErrorCode {
	moduleList := make([]ModuleInfo, 0)
	for _, moduleInfo := range d.ModuleConfig {
		moduleList = append(moduleList, moduleInfo)
	}
	subDeviceModuleList := make([]ModuleInfo, 0)
	for _, subModuleInfo := range d.SubDeviceModuleConfig {
		subDeviceModuleList = append(subDeviceModuleList, subModuleInfo)
	}
	qos0 := make([]string, 0)
	qos1 := make([]string, 0)
	for topic, qos := range d.UserDefineTopicQosMap {
		if qos == 0 {
			qos0 = append(qos0, topic)
		} else if qos == 1 {
			qos1 = append(qos1, topic)
		}
	}
	deviceConfig := DeviceConfig{
		InstanceID:              d.InstanceID,
		DeviceName:              d.Name,
		ProductKey:              d.ProductKey,
		ProductSecret:           d.ProductSecret,
		DeviceSecret:            d.Secret,
		MQTTHost:                d.MQTTHost,
		MQTTPort:                d.MQTTPort,
		CloudHTTPHost:           d.CloudHTTPHost,
		EnableTls:               d.EnableTls,
		VerifyMode:              string(d.VerifyMode),
		IsGateway:               d.IsGateway,
		FilePath:                d.FilePath,
		OTAHistoryJobReloadMode: string(d.OTAHistoryJobReloadMode),
		OTAModule:               moduleList,
		SubDeviceOTAModule:      subDeviceModuleList,
		TopicConfig: UserDefineTopicConfig{
			Qos0: qos0,
			Qos1: qos1,
		},
		LogReportConfig:  d.LogReportConfig.LogReportCommonConfig,
		WebshellExecutor: d.WebshellExecutor,
	}

	configBytes, err := yaml.Marshal(deviceConfig)
	if err != nil {
		d.sdkLogf(LogLevelError, "Update config Failed!,err :%s", err)
		return ErrorCodeFileOperateFailed
	}
	err = ioutil.WriteFile(ConfigFilePath+".tmp", configBytes, 0664)
	if err == nil {
		os.Rename(ConfigFilePath+".tmp", ConfigFilePath)
	} else {
		os.Remove(ConfigFilePath + ".tmp")
		d.sdkLogf(LogLevelError, "Write Config File Error:%s", err)
		return ErrorCodeFileOperateFailed
	}
	return SuccessCode
}

func (d *Device) updateJobHistoryFile() error {
	// 更新ota_job_history.json文件
	content := make(map[string]JobInfo)
	d.otaHistoryJob.Range(func(k, v interface{}) bool {
		content[k.(string)] = v.(JobInfo)
		return true
	})

	bytes, err := json.Marshal(content)
	if err != nil {
		return err
	}

	return saveFile(filepath.Join(d.FilePath, otaJobHistoryPath), filepath.Join(d.FilePath, md5CodeStorageFile), bytes)
}
