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
	// 设备初始化，创建
	deviceOption := arenal.NewDeviceOption()
	deviceOption.WithMQTTEndpoint("SAMPLE_MQTT_HOST")
	deviceOption.WithMQTTPort(8883)
	deviceOption.WithEnableTls(true)
	deviceOption.WithCloudHTTPEndpoint("SAMPLE_HTTP_HOST")
	deviceOption.WithInstanceID("SAMPLE_INSTANCE_ID")
	devices := make([]*arenal.Device, 0)
	// 创建10个设备
	for i := 1; i <= 10; i++ {
		deviceOption.WithDeviceInfo(
			"SAMPLE_PRODUCT_KEY",
			"SAMPLE_PRODUCT_SECRET",
			"device-"+strconv.Itoa(i),
			"",
			arenal.VerifyModeDynamicNoPreRegistered,
		)
		d := arenal.CreateDeviceWithConfig(*deviceOption)

		// 子设备上线（可选，网关设备支持子设备属性设置，需要子设备上线）
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

		// 设置属性设置handler
		d.SetPropertySetHandler(func(message arenal.PropertySetMessage) arenal.ErrorCode {
			messageStr, _ := json.Marshal(message)
			fmt.Println(fmt.Sprintf("Receive property set success!message = %v", string(messageStr)))
			return arenal.SuccessCode
		})

		// 设置服务调用handler
		d.SetServiceCallHandler(func(message arenal.ServiceCallMessage) (map[string]interface{}, arenal.ErrorCode) {
			messageStr, _ := json.Marshal(message)
			fmt.Println(fmt.Sprintf("Receive service call Success!message = %v", string(messageStr)))
			// 构建outputData：data_identifier:value的map，用于应答平台的服务调用，需与服务定义对应
			outputData := make(map[string]interface{})
			outputData["data_identifier"] = "value"
			return outputData, arenal.SuccessCode
		})

		// 设置设备影子更新handler
		d.SetShadowSetHandler(func(message arenal.ShadowSetMessage) arenal.ErrorCode {
			messageStr, _ := json.Marshal(message)
			fmt.Println(fmt.Sprintf("Receive shadow set Success!message = %v", string(messageStr)))
			return arenal.SuccessCode
		})

		// 设置自定义任务handler
		d.SetCustomJobHandler(func(message arenal.CustomJobMessage) (arenal.ErrorCode, string) {
			messageStr, _ := json.Marshal(message)
			fmt.Println(fmt.Sprintf("Receive custom job Success!message = %v", string(messageStr)))
			return arenal.SuccessCode, ""
		})

		devices = append(devices, &d)
	}

	for _, d := range devices {
		if ok := d.Init(); !ok {
			log.Error("device init failed")
		}
	}

	time.Sleep(30 * time.Minute)
}
