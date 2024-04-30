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
		return
	}

	// 2.属性上报需要设置ProeprtyReportReplyHandler
	d.SetPropertyReportReplyHandler(func(reply arenal.ReplyParam) {
		replyStr, _ := json.Marshal(reply)
		fmt.Printf(fmt.Sprintf("receive message success!message = %v\n", string(replyStr)))
	})

	// 3.1 构建uniqueName：value的map，用于属性上报，uniqueName可以调用FormatUniqueFieldName方法，传递所属的Modulekey和自身的Identifier
	propertyMap := make(map[string]arenal.PropertyValue)
	propertyMap[arenal.FormatUniqueFieldName("moduleKey", "identifier")] = arenal.PropertyValue{
		Value:    "value",
		TimeMill: arenal.GetTimeMill(),
	}

	// 3.2 调用PropertyReport方法进行属性上报，传递上面构建的propertyMap
	code := d.PropertyReport(propertyMap)
	if code != arenal.SuccessCode {
		log.Error("property report failed")
	}
	log.Infof("property report success,%#v", propertyMap)

	/*********************************************** 网关支持子设备属性上报 ***********************************************/
	// 4.子设备上线（网关设备支持子设备属性上报，需要子设备上线）
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

	// 5.子设备属性上报
	d.SetSubDeviceLoginReplyHandler(func(subDevices []arenal.DeviceIdentification) arenal.ErrorCode {
		log.Infof("sub devices login success, %#v", subDevices)
		propertyMap = make(map[string]arenal.PropertyValue)
		propertyMap[arenal.FormatUniqueFieldName("moduleKey", "identifier")] = arenal.PropertyValue{
			Value:    "sub-value",
			TimeMill: arenal.GetTimeMill(),
		}
		for i := 1; i <= 2; i++ {
			subDevice := arenal.DeviceIdentification{
				ProductKey: "xxxxxxxxx",
				DeviceName: "device2-" + strconv.Itoa(i),
			}
			// 网关设备代子设备进行属性上报，需要指定子设备的ProductKey以及DeviceName
			code = d.PropertyReport(propertyMap, subDevice)
			if code != arenal.SuccessCode {
				// 错误处理
				log.Error("sub device property report failed")
				return code
			}
			log.Infof("sub device property report success,sub device is %#v,report param is %#v", subDevice, propertyMap)
		}

		return arenal.SuccessCode
	})

	time.Sleep(100 * time.Second)
}
