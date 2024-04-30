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
	// 1.设备初始化
	deviceOption := arenal.NewDeviceOption()
	deviceOption.WithDeviceInfo(
		"SAMPLE_PRODUCT_KEY",
		"SAMPLE_PRODUCT_SECRET",
		"SAMPLE_DEVICE_NAME",
		"",
		arenal.VerifyModeDynamicNoPreRegistered,
	)
	deviceOption.WithInstanceID("SAMPLE_INSTANCE_ID")
	deviceOption.WithMQTTEndpoint("SAMPLE_MQTT_HOST")
	deviceOption.WithMQTTPort(8883)
	deviceOption.WithEnableTls(true)
	deviceOption.WithCloudHTTPEndpoint("SAMPLE_HTTP_HOST")
	d := arenal.CreateDeviceWithConfig(*deviceOption)

	if ok := d.Init(); !ok {
		log.Error("device init failed")
	}

	// 2.1 构建uniqueName：value的map，用于上报设备影子
	propertyMap := make(map[string]interface{})
	propertyMap[arenal.FormatUniqueFieldName("default", "Brightness")] = 30
	// 构建设备影子的reported字段内容，填写propertyMap（可不包含全部属性）
	reported := arenal.Reported{
		Reported: propertyMap,
	}

	// 2.2 调用ShadowReport方法，传递reported对象，将当前信息传递给平台设备影子
	code := d.ShadowReport(reported)
	if code != arenal.SuccessCode {
		log.Error("shadow report failed")
	}
	log.Infof("shadow report success,%#v", propertyMap)

	/*********************************************** 网关支持子设备影子上报 ***********************************************/
	// 3.子设备上线（网关设备支持子设备影子上报，需要子设备上线）
	d.SetSubDeviceSecretGetReplyHandler(func(subDeviceSecrets []arenal.DeviceAuthWithDeviceSecret) arenal.ErrorCode {
		log.Infof("get sub device secrets success, %#v", subDeviceSecrets)
		subDevices := make([]arenal.DeviceIdentification, 0)
		for _, v := range subDeviceSecrets {
			subDevices = append(subDevices, v.DeviceIdentification)
		}
		_, errCode := d.SubDeviceLogin(subDevices)
		if errCode != arenal.SuccessCode {
			log.Errorf("sub devices login failed, %d", errCode)
			return errCode
		}
		return arenal.SuccessCode
	})

	// 4.子设备影子上报
	d.SetSubDeviceLoginReplyHandler(func(subDevices []arenal.DeviceIdentification) arenal.ErrorCode {
		log.Infof("sub devices login success, %#v", subDevices)
		propertyMap = make(map[string]interface{})
		propertyMap[arenal.FormatUniqueFieldName("moduleKey", "identifier")] = "sub-value"
		reported = arenal.Reported{
			Reported: propertyMap,
		}
		for i := 1; i <= 2; i++ {
			subDevice := arenal.DeviceIdentification{
				ProductKey: "xxxxxxxxx",
				DeviceName: "device2-" + strconv.Itoa(i),
			}
			// 网关设备代子设备进行设备影子上报，需要指定子设备的ProductKey以及DeviceName
			code = d.ShadowReport(reported, subDevice)
			if code != arenal.SuccessCode {
				log.Error("sub device shadow report failed")
				return code
			}
			log.Infof("sub device shadow report success,sub device is %#v,report param is %#v", subDevice, propertyMap)
		}
		return arenal.SuccessCode
	})

	time.Sleep(100 * time.Second)
}
