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
		"SAMPLE_DEVICE_SECRET",
		arenal.VerifyModeDeviceSecret,
	)
	deviceOption.WithInstanceID("SAMPLE_INSTANCE_ID")
	deviceOption.WithMQTTEndpoint("SAMPLE_MQTT_HOST")
	deviceOption.WithMQTTPort(8883)
	deviceOption.WithEnableTls(true)
	deviceOption.WithCloudHTTPEndpoint("SAMPLE_HTTP_HOST")
	d := arenal.CreateDeviceWithConfig(*deviceOption)

	// 2.订阅自定义消息，并将消息打印出来
	d.SetCustomTopicHandler("sub", func(param arenal.CustomTopicParam) arenal.ErrorCode {
		fmt.Println(fmt.Sprintf("Receive custom topic = %s ,payload = %s", param.Topic, param.Payload))
		return arenal.SuccessCode
	})

	if ok := d.Init(); !ok {
		log.Error("device init failed")
	}

	// 3.发送自定义消息
	d.SendCustomTopicMessage("pub", "自定义测试消息", 0)

	time.Sleep(100 * time.Second)
}
