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

	// 2.支持设备影子功能，需要对ShadowSetHandler进行设置，其中编写接收到Desired消息（期望字段）之后进行的处理
	d.SetShadowSetHandler(func(message arenal.ShadowSetMessage) arenal.ErrorCode {
		// 此处建议对属性进行相应的设置
		messageStr, _ := json.Marshal(message)
		fmt.Println(fmt.Sprintf("receive message success!message = %v\n", string(messageStr)))
		return arenal.SuccessCode
	})

	if ok := d.Init(); !ok {
		log.Error("device init failed")
	}

	// 3.设备主动获取设备影子期望值，返回的消息会发送至ShadowSetHandler中进行处理
	code := d.GetShadowDesired()
	if code != arenal.SuccessCode {
		log.Error("get shadow desired failed")
	}

	/*********************************************** 网关支持子设备期望值更新 ***********************************************/
	// 4.子设备上线（可选，网关设备支持子设备期望值更新，需要子设备上线）
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

	// 5.获取子设备影子期望值
	d.SetSubDeviceLoginReplyHandler(func(subDevices []arenal.DeviceIdentification) arenal.ErrorCode {
		log.Infof("sub devices login success, %#v", subDevices)
		for i := 1; i <= 2; i++ {
			subDevice := arenal.DeviceIdentification{
				ProductKey: "xxxxxxxxx",
				DeviceName: "device2-" + strconv.Itoa(i),
			}
			// 网关设备获取子设备影子期望值，需要指定子设备的ProductKey以及DeviceName
			code = d.GetShadowDesired(subDevice)
			if code != arenal.SuccessCode {
				log.Error("get sub device shadow desired failed,sub device is %#v", subDevice)
				return code
			}
		}
		return arenal.SuccessCode
	})

	time.Sleep(30 * time.Minute)
}
