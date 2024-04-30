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
	}

	// 2.进行事件/属性上报需要先配置应答Handler
	d.SetEventReportReplyHandler(func(reply arenal.ReplyParam) {
		replyStr, _ := json.Marshal(reply)
		fmt.Println(fmt.Sprintf("receive message success!message = %v", string(replyStr)))
	})

	// 3.调用EventReport方法，传递对应事件所属的ModuleKey，自身的Identifier以及参数列表
	// 构建outputData:identifier：value的map，用于事件上报
	outputData := make(map[string]interface{})
	outputData["data_identifier"] = "value"
	param := arenal.EventReportParam{
		ModuleKey:  "moduleKey",
		Identifier: "event_identifier",
		Values:     outputData,
	}
	code := d.EventReport(param)
	if code != arenal.SuccessCode {
		// 错误处理
		log.Error("event report failed")
	}
	log.Infof("event report success,%#v", param)

	/*********************************************** 网关支持子设备事件上报 ***********************************************/
	// 4.子设备上线（网关设备支持子设备事件上报，需要子设备上线）
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

	// 5.子设备事件上报
	d.SetSubDeviceLoginReplyHandler(func(subDevices []arenal.DeviceIdentification) arenal.ErrorCode {
		log.Infof("sub devices login success, %#v", subDevices)
		outputData = make(map[string]interface{})
		outputData["data_identifier"] = "sub-value"
		param = arenal.EventReportParam{
			ModuleKey:  "moduleKey",
			Identifier: "event_identifier",
			Values:     outputData,
		}
		for i := 1; i <= 2; i++ {
			subDevice := arenal.DeviceIdentification{
				ProductKey: "xxxxxxxxx",
				DeviceName: "device2-" + strconv.Itoa(i),
			}
			// 网关设备代子设备进行事件上报，需要指定子设备的ProductKey以及DeviceName
			code = d.EventReport(param, subDevice)
			if code != arenal.SuccessCode {
				// 错误处理
				log.Error("sub device event report failed")
				return code
			}
			log.Infof("sub device event report success,sub device is %#v,report param is %#v", subDevice, param)
		}
		return arenal.SuccessCode
	})

	time.Sleep(100 * time.Second)
}
