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

	if ok := d.Init(); !ok {
		log.Error("device init failed")
	}

	// 2.属性上报需要设置PropertyReportReplyHandler
	d.SetPropertyReportReplyHandler(func(reply arenal.ReplyParam) {
		replyStr, _ := json.Marshal(reply)
		fmt.Printf(fmt.Sprintf("receive message success!message = %v\n", string(replyStr)))
	})

	for {
		// 3.1 构建uniqueName：value的map，用于属性上报，uniqueName可以调用FormatUniqueFieldName方法，传递所属的Modulekey和自身的Identifier
		propertyMap := make(map[string]arenal.PropertyValue)
		propertyMap[arenal.FormatUniqueFieldName("default", "timestamp")] = arenal.PropertyValue{
			Value:    arenal.GetTimeMill(),
			TimeMill: arenal.GetTimeMill(),
		}

		// 3.2 调用PropertyReport方法进行属性上报，传递上面构建的propertyMap
		_, code := d.PropertyReportAsync(propertyMap)
		if code != arenal.SuccessCode {
			log.Error("property report failed")
		} else {
			log.Infof("property report success,%#v", propertyMap)
		}

		time.Sleep(1 * time.Second)
	}
}
