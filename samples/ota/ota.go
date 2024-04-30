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
	var tmpVersion string
	// 1.设备初始化
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
	d := arenal.CreateDeviceWithConfig(*deviceOption)

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
	var InstallAlgorithm arenal.InstallAlgorithm
	InstallAlgorithm = func(module, filename string) arenal.InstallAlgorithmResult {
		// 这里传递的参数为最终的整包路径，如果不是差分，则为下载路径，是差分则为反差分算法形成的整包路径
		log.Infof("installing：filename:%s", filename)
		time.Sleep(10 * time.Second)
		return arenal.InstallAlgorithmResult{
			IsSuccess:     true,
			ResDesc:       "",
			DeviceVersion: tmpVersion,
			NeedReboot:    false,
		}
	}
	d.SetInstallAlgorithm(InstallAlgorithm)

	// 4.配置设备收到OTA升级通知后需要进行的操作，建议收到升级通知后向平台请求升级，若未配置，默认立即调用OTAUpgradeRequest请求升级
	d.SetOTANotifyHandler(func(message arenal.OTANotifyPayload) {
		// 此处演示接收到升级通知时，立即调用OTAUpgradeRequest请求升级
		messageStr, _ := json.Marshal(message)
		fmt.Println(fmt.Sprintf("receive message success!message = %v", string(messageStr)))
		// 若OTAUpgradeRequest参数otaJobID为空，表示请求最新的升级任务，建议不指定；
		code := d.OTAUpgradeRequest(message.Data.Module, "", message.DeviceIdentification)
		if code != arenal.SuccessCode {
			// 错误处理
			log.Error("upgrade request failed")
		}
	})

	// 5.配置设备请求升级后，接收到平台响应消息时需要进行的操作，建议初始化升级任务->下载->安装，也可以自定义升级流程，例如稍后下载/达到某个外界条件时安装，但初始化升级任务是必要步骤
	d.SetOTAReplyHandler(func(message arenal.OTAReplyPayload) {
		messageStr, _ := json.Marshal(message)
		fmt.Println(fmt.Sprintf("receive message success!message = %v", string(messageStr)))
		tmpVersion = message.Data.DestVersion
		// 5.1 升级任务初始化
		code, otaMetadata := d.InitOTAJob(message.DeviceIdentification, message.Data)
		if code != arenal.SuccessCode {
			// 错误处理
			log.Infof("no available OTA job was found for the OTA reply message,code:%d,otaMetadata:%v", code, otaMetadata)
			return
		}

		// 5.2 下载流程
		code = d.OTADownload(otaMetadata.OTAModuleName)
		if code != arenal.SuccessCode {
			// 错误处理
			log.Errorf("OTA download failed,code:%d,otaMetadata:%v", code, otaMetadata)
			return
		}

		// 5.3 安装流程
		code = d.OTAInstall(otaMetadata.OTAModuleName)
		if code != arenal.SuccessCode {
			// 错误处理
			log.Errorf("OTA install failed,code:%d,otaMetadata:%v", code, otaMetadata)
		}
	})

	if ok := d.Init(); !ok {
		log.Error("device init failed")
	}

	// 6.可选，设备获取升级信息有两种方式，方式一是通知触发升级，即设备接收到平台下发的升级通知后，调用OTAUpgradeRequest申请升级信息，见步骤4；
	// 方式二是设备主动请求升级，即主动调用OTAUpgradeRequest申请升级信息，此时需要主动填写设备需要升级的OTA模块，如果有可用更新，则平台会通过Reply返回升级信息，否则说明当前无可用更新
	// 系统默认配置方式一，方式一需要平台下发升级通知触发，可在平台升级任务列表页或升级任务详情页以任务/设备为粒度触发通知操作
	// 这里采用方式二，此时需要主动填写设备需要升级的OTA模块，这里以default模块为例主动请求升级，建议不指定otaJobID，即请求最新升级任务
	code := d.OTAUpgradeRequest("default", "")
	if code != arenal.SuccessCode {
		// 错误处理
		log.Error("upgrade request failed")
	}

	time.Sleep(30 * time.Minute)
}
