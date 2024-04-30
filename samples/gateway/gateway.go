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
	log "github.com/sirupsen/logrus"
	"github.com/volcengine/vei-iot-sdk-go/arenal"
	"strconv"
	"time"
)

func main() {
	// 1、网关设备初始化
	deviceOption := arenal.NewDeviceOption()
	deviceOption.WithDeviceInfo(
		"SAMPLE_PRODUCT_KEY",
		"SAMPLE_PRODUCT_SECRET",
		"SAMPLE_DEVICE_NAME",
		"SAMPLE_DEVICE_SECRET",
		arenal.VerifyModeDeviceSecret,
	)
	deviceOption.WithMQTTEndpoint("SAMPLE_MQTT_HOST")
	deviceOption.WithMQTTPort(8883)
	deviceOption.WithEnableTls(true)
	deviceOption.WithCloudHTTPEndpoint("SAMPLE_HTTP_HOST")
	deviceOption.WithIsGateway(true)
	deviceOption.WithInstanceID("SAMPLE_INSTANCE_ID")
	d := arenal.CreateDeviceWithConfig(*deviceOption)

	subDeviceProductSecret := "xxxx"
	subDevicesWithProductSecret := make([]arenal.DeviceAuthWithProductSecret, 0)
	subDevices := make([]arenal.DeviceIdentification, 0)
	for i := 1; i <= 2; i++ {
		subDevice := arenal.DeviceIdentification{
			ProductKey: "xxxxx",
			DeviceName: "sub-" + strconv.Itoa(i),
		}
		subDevices = append(subDevices, subDevice)
		subDevicesWithProductSecret = append(subDevicesWithProductSecret, arenal.DeviceAuthWithProductSecret{
			DeviceIdentification: subDevice,
			ProductSecret:        subDeviceProductSecret,
		})
	}

	// 2、通过注册handler把流程串起来：网关发现子设备->平台确认->添加拓扑关系->获取拓扑关系->子设备动态注册->子设备上线->子设备下线->删除拓扑关系
	// 收到通知添加子设备的请求，发起添加拓扑关系请求
	d.SetNotifyAddSubDeviceHandler(func(subDevices []arenal.DeviceIdentification) arenal.ErrorCode {
		d.DeviceLogf(arenal.LogLevelInfo, "receive notify add sub device request, %#v", subDevices)
		// 添加拓扑关系
		_, errCode := d.AddTopologies(subDevicesWithProductSecret)
		if errCode != arenal.SuccessCode {
			log.Error("add sub devices failed, %d", errCode)
			return errCode
		}
		return arenal.SuccessCode
	})
	// 收到拓扑关系添加reply，拉取当前拓扑关系
	d.SetAddTopologiesReplyHandler(func(subDevices []arenal.DeviceIdentification) arenal.ErrorCode {
		d.DeviceLogf(arenal.LogLevelInfo, "add sub devices success, %#v", subDevices)
		// 确认添加子设备成功，获取子设备列表
		errCode := d.GetTopologies()
		if errCode != arenal.SuccessCode {
			log.Error("get sub devices failed, %d", errCode)
		}
		return arenal.SuccessCode
	})
	d.SetDeleteTopologiesReplyHandler(func(subDevices []arenal.DeviceIdentification) arenal.ErrorCode {
		d.DeviceLogf(arenal.LogLevelInfo, "delete sub devices success, %#v", subDevices)
		return arenal.SuccessCode
	})
	// 收到获取拓扑关系reply，对获取到的子设备发起上线请求
	d.SetGetTopologiesReplyHandler(func(subDevices []arenal.DeviceIdentification) arenal.ErrorCode {
		d.DeviceLogf(arenal.LogLevelInfo, "get sub devices success, %#v", subDevices)
		// 子设备上线
		_, errCode := d.SubDeviceLogin(subDevices)
		if errCode != arenal.SuccessCode {
			log.Errorf("sub devices login failed, %d", errCode)
			return errCode
		}
		return arenal.SuccessCode
	})
	// 子设备上线成功，sleep一分钟后发起下线请求
	d.SetSubDeviceLoginReplyHandler(func(subDevices []arenal.DeviceIdentification) arenal.ErrorCode {
		d.DeviceLogf(arenal.LogLevelInfo, "sub devices login success, %#v", subDevices)
		time.Sleep(1 * time.Minute)
		// 子设备下线
		errCode := d.SubDeviceLogout(subDevices)
		if errCode != arenal.SuccessCode {
			log.Error("sub devices logout failed, %d", errCode)
		}
		return arenal.SuccessCode
	})
	// 子设备下线成功后，sleep一分钟后删除拓扑关系
	d.SetSubDeviceLogoutReplyHandler(func(subDevices []arenal.DeviceIdentification) arenal.ErrorCode {
		d.DeviceLogf(arenal.LogLevelInfo, "sub devices logout success, %#v", subDevices)
		time.Sleep(1 * time.Minute)
		// 删除拓扑关系
		errCode := d.DeleteTopologies(subDevices)
		if errCode != arenal.SuccessCode {
			log.Error("delete sub devices failed, %d", errCode)
		}
		return arenal.SuccessCode
	})
	d.SetSubDeviceChangeHandler(func(param arenal.SubDeviceChangeParam) arenal.ErrorCode {
		d.DeviceLogf(arenal.LogLevelInfo, "received sub device change request, %#v", param)
		return arenal.SuccessCode
	})
	d.SetTopologyChangeHandler(func(param arenal.TopoChangeParam) arenal.ErrorCode {
		d.DeviceLogf(arenal.LogLevelInfo, "received topology change request, %#v", param)
		return arenal.SuccessCode
	})

	if !d.Init() {
		d.DeviceLog(arenal.LogLevelError, "init gateway failed")
		return
	}
	// 上报发现的设备列表
	errCode := d.ReportDiscoveryList(subDevices)
	if errCode != arenal.SuccessCode {
		d.DeviceLogf(arenal.LogLevelError, "report discovery list failed, %d", errCode)
		return
	}
	time.Sleep(1000 * time.Minute)
}
