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
	timestamp, errCode := d.GetServerTime()
	fmt.Printf("timestamp: %d, errcode: %d\n", timestamp, errCode)

	// 设置NTP服务response handler
	d.SetNTPResponseHandler(func(serverTimeMS int64) arenal.ErrorCode {
		d.DeviceLogf(arenal.LogLevelInfo, "get server time success!time = %d\n", serverTimeMS)
		return arenal.SuccessCode
	})

	if ok := d.Init(); !ok {
		log.Error("device init failed")
	}

	errCode = d.SendNTPRequest()
	if errCode != arenal.SuccessCode {
		d.DeviceLogf(arenal.LogLevelError, "send ntp request failed, %d", errCode)
	}

	time.Sleep(time.Minute * 10)
}
