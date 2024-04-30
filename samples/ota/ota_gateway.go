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

package main

import (
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/volcengine/vei-iot-sdk-go/arenal"
	"time"
)

func main() {
	// 子设备升级前提条件：子设备已关联到网关（两种方式：通过平台关联到网关/在网关sdk通过topic关联到网关，为简化流程，这里采用第一种方式）
	var tmpVersion string
	// 1、网关设备初始化
	deviceOption := arenal.NewDeviceOption()
	deviceOption.WithDeviceInfo(
		"SAMPLE_PRODUCT_KEY",
		"SAMPLE_PRODUCT_SECRET",
		"SAMPLE_DEVICE_NAME",
		"SAMPLE_DEVICE_SECRET",
		arenal.VerifyModeDynamicNoPreRegistered,
	)
	deviceOption.WithInstanceID("SAMPLE_INSTANCE_ID")
	deviceOption.WithMQTTEndpoint("SAMPLE_MQTT_HOST")
	deviceOption.WithMQTTPort(8883)
	deviceOption.WithEnableTls(true)
	deviceOption.WithCloudHTTPEndpoint("SAMPLE_HTTP_HOST")
	deviceOption.WithIsGateway(true)
	d := arenal.CreateDeviceWithConfig(*deviceOption)

	d.SetSubDeviceSecretGetReplyHandler(func(subDeviceSecrets []arenal.DeviceAuthWithDeviceSecret) arenal.ErrorCode {
		log.Infof("get sub device secrets success, %#v", subDeviceSecrets)
		subDevices := make([]arenal.DeviceIdentification, 0)
		for _, v := range subDeviceSecrets {
			subDevices = append(subDevices, v.DeviceIdentification)
		}
		// 子设备上线
		_, errCode := d.SubDeviceLogin(subDevices)
		if errCode != arenal.SuccessCode {
			log.Errorf("sub devices login failed, %d", errCode)
			return errCode
		}

		return arenal.SuccessCode
	})

	// （可选）子设备上线成功后，网关代子设备主动请求升级
	// 设备获取升级信息有两种方式，方式一是通知触发升级，即设备接收到平台下发的升级通知后，调用OTAUpgradeRequest申请升级信息；
	// 方式二是设备主动请求升级，即主动调用OTAUpgradeRequest申请升级信息，此时需要主动填写设备需要升级的OTA模块，如果有可用更新，则平台会通过Reply返回升级信息，否则说明当前无可用更新
	// 若此处SetSubDeviceLoginReplyHandler未配置，则系统默认配置方式一，方式一需要平台下发升级通知触发，可在平台升级任务列表页或升级任务详情页以任务/设备为粒度触发通知操作
	d.SetSubDeviceLoginReplyHandler(func(subDevices []arenal.DeviceIdentification) arenal.ErrorCode {
		log.Infof("sub devices login success, %#v", subDevices)
		// 这里采用方式二，子设备上线成功，网关代子设备主动向平台发起升级请求，需要主动填写设备需要升级的OTA模块，这里以default模块为例，建议不指定otaJobID，即请求最新升级任务
		for _, subDevice := range subDevices {
			code := d.OTAUpgradeRequest("default", "", subDevice)
			if code != arenal.SuccessCode {
				// 错误处理
				log.Error("upgrade request failed")
			}
		}
		return arenal.SuccessCode
	})

	// 2.(可选，若存在差分包则必须配置)配置差分还原算法，调用设备提供的SetDiffAlgorithm方法，传递固定结构的function
	// 如果存在差分包的情况，必须准备好对应的反差分算法，否则可能会导致安装失败
	var diffAlgorithm arenal.DiffAlgorithm
	diffAlgorithm = func(diffFileName string) arenal.DiffAlgorithmResult {
		// 具体的生成整包流程
		return arenal.DiffAlgorithmResult{
			IsSuccess:      true,
			ResDesc:        "",
			TargetFilePath: diffFileName,
		}
	}
	d.SetDiffAlgorithm(diffAlgorithm)

	// 3.配置安装包安装流程，调用设备提供的SetInstallAlgorithm方法，传递固定结构的function
	// 需要提前准备好安装包烧写流程
	var InstallAlgorithm arenal.InstallAlgorithm
	InstallAlgorithm = func(module, filename string) arenal.InstallAlgorithmResult {
		// 这里传递的参数为最终的整包路径，如果不是差分，则为下载路径，是差分则为反差分算法形成的整包路径
		log.Infof("installing：filename:%s", filename)
		time.Sleep(20 * time.Second)
		return arenal.InstallAlgorithmResult{
			IsSuccess:     true,
			DeviceVersion: "your dest version",
			ResDesc:       "",
		}
	}
	d.SetInstallAlgorithm(InstallAlgorithm)

	// 4.配置设备请求升级后，接收到平台响应消息时需要进行的操作，建议初始化升级任务->下载->安装，也可以自定义升级流程，例如稍后下载/达到某个外界条件时安装，但初始化升级任务是必要步骤
	d.SetOTAReplyHandler(func(message arenal.OTAReplyPayload) {
		messageStr, _ := json.Marshal(message)
		fmt.Println(fmt.Sprintf("receive message success!message = %v", string(messageStr)))
		tmpVersion = message.Data.DestVersion

		// 4.1 升级任务初始化
		code, otaMetadata := d.InitOTAJob(message.DeviceIdentification, message.Data)
		if code != arenal.SuccessCode {
			// 错误处理
			log.Infof("no available OTA job was found for the OTA reply message,code:%d,otaMetadata:%v", code, otaMetadata)
			return
		}

		// 4.2 下载流程
		code = d.OTADownload(otaMetadata.OTAModuleName, message.DeviceIdentification)
		if code != arenal.SuccessCode {
			// 错误处理
			log.Errorf("OTA download failed,code:%d,otaMetadata:%v", code, otaMetadata)
			return
		}

		// 4.3 安装流程
		code = d.OTAInstall(otaMetadata.OTAModuleName, message.DeviceIdentification)
		if code != arenal.SuccessCode {
			// 错误处理
			log.Errorf("OTA install failed,code:%d,otaMetadata:%v", code, otaMetadata)
		}

	})

	// 5.网关设备需要配置通用的子设备升级包安装流程，调用设备提供的SetSubDeviceInstallAlgorithm方法，传递固定结构的function
	var subDeviceCommonInstallAlgorithm arenal.SubDeviceCommonInstallAlgorithm
	subDeviceCommonInstallAlgorithm = func(SubDeviceCommonInstallParam arenal.SubDeviceInstallParam) arenal.InstallAlgorithmResult {
		// 完整的子设备升级包安装流程应该包括:向子设备下发升级包，获取子设备最新版本号
		// 只有当子设备版本号与升级包版本号一致时，网关代子设备上报升级成功，这里作演示用，默认子设备最新版本号为升级包版本号
		time.Sleep(20 * time.Second)
		log.Infof("install with common-install-algorithm:%s|%s", SubDeviceCommonInstallParam.ProductKey, SubDeviceCommonInstallParam.DeviceName)
		return arenal.InstallAlgorithmResult{
			IsSuccess:     true,
			ResDesc:       "",
			DeviceVersion: tmpVersion,
		}
	}
	d.SetSubDeviceCommonInstallAlgorithm(subDeviceCommonInstallAlgorithm)

	// 6.（可选）网关设备可以配置定制化的子设备升级包安装流程，即针对不同产品的设备，采用不同的定制化安装方法，对于未配置的子设备，默认采用步骤5配置的通用流程
	subDeviceCustomInstallAlgorithms := make(map[string]arenal.SubDeviceCustomInstallAlgorithm)
	var subDeviceCustomInstallAlgorithm arenal.SubDeviceCustomInstallAlgorithm
	subDeviceCustomInstallAlgorithm = func(SubDeviceCustomInstallParam arenal.SubDeviceInstallParam) arenal.InstallAlgorithmResult {
		// 针对某个产品下的所有子设备，采用定制化的安装流程
		log.Infof("install with custom-install-algorithm:%s|%s", SubDeviceCustomInstallParam.ProductKey, SubDeviceCustomInstallParam.DeviceName)
		time.Sleep(20 * time.Second)
		return arenal.InstallAlgorithmResult{
			IsSuccess:     true,
			ResDesc:       "",
			DeviceVersion: tmpVersion,
		}
	}
	// 这里的subDeviceCommonInstallAlgorithms的key为子设备的productKey
	subDeviceCustomInstallAlgorithms["61c070a8785cb9d8eb7bd84c"] = subDeviceCustomInstallAlgorithm
	d.SetSubDeviceCustomInstallAlgorithm(subDeviceCustomInstallAlgorithms)

	if !d.Init() {
		log.Error("init gateway failed")
		return
	}

	time.Sleep(30 * time.Minute)
}
