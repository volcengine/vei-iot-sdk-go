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
)

func main() {
	// 1.设备初始化
	deviceOption := arenal.NewDeviceOption()
	productKey := "SAMPLE_PRODUCT_KEY"
	productSecret := "SAMPLE_PRODUCT_SECRET"
	deviceName := "SAMPLE_DEVICE_NAME"
	deviceSecret := "SAMPLE_DEVICE_SECRET"
	deviceOption.WithDeviceInfo(
		productKey,
		productSecret,
		deviceName,
		deviceSecret,
		arenal.VerifyModeDynamicNoPreRegistered,
	)
	deviceOption.AccessType = arenal.DeviceAccessTypeHTTP
	deviceOption.WithInstanceID("SAMPLE_INSTANCE_ID")
	deviceOption.WithCloudHTTPEndpoint("SAMPLE_HTTP_HOST")
	d := arenal.CreateDeviceWithConfig(*deviceOption)

	if ok := d.Init(); !ok {
		log.Error("device init failed")
		return
	}

	res, code := d.GetDeviceToken()
	if code != arenal.SuccessCode {
		fmt.Printf("GetDeviceToken unexpected code: %v\n", code)
		return
	}

	fmt.Println(res.Token)

	moduleKey := "xxxx"
	payload := arenal.PropertyPayload{
		Params: map[string]arenal.PropertyValue{
			arenal.FormatUniqueFieldName(moduleKey, "xxxxx"): {
				Value:    16,
				TimeMill: arenal.GetTimeMill(),
			},
		},
	}
	bs, _ := json.Marshal(payload)
	param := arenal.PublishMessageParam{
		AccessToken: res.Token,
		Topic:       fmt.Sprintf(arenal.PropertyReportTopic, productKey, deviceName),
		Payload:     bs,
	}
	publishRes, code := d.PublishMessageHTTP(&param)
	if code != arenal.SuccessCode {
		fmt.Printf("PublishMessageHTTP unexpected code: %v\n", code)
		return
	}
	fmt.Printf("%+v\n", publishRes)
}
