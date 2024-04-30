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

	// 2.子设备上线（可选，网关设备支持子设备属性设置，需要子设备上线）
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

	// 3.平台能够对属性进行设置，需要事先构建PropertySetHandler，用于接收平台下发的属性设置消息
	d.SetPropertySetHandler(func(message arenal.PropertySetMessage) arenal.ErrorCode {
		messageStr, _ := json.Marshal(message)
		fmt.Println(fmt.Sprintf("receive message success!message = %v", string(messageStr)))
		return arenal.SuccessCode
	})

	if ok := d.Init(); !ok {
		log.Error("device init failed")
	}

	time.Sleep(30 * time.Minute)
}
