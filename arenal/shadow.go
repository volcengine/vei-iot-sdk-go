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

package arenal

import (
	"encoding/json"
	"fmt"
	paho "github.com/eclipse/paho.mqtt.golang"
)

// SetShadowSetHandler 配置处理设备影子设置期望值消息的Handler
// 传递 ShadowSetHandler
func (d *Device) SetShadowSetHandler(handler ShadowSetHandler) {
	d.shadowSetHandler = handler
}

// ShadowReport 主动调用，进行设备影子上报
// 传递 Reported 要上报的属性信息; DeviceIdentification 为子设备进行设备影子上报还需要指定子设备标识
// 返回 成功标识
func (d *Device) ShadowReport(reported Reported, deviceIdentification ...DeviceIdentification) ErrorCode {
	// 为子设备进行设备影子上报需要指定子设备标识，否则默认为设备自身进行设备影子上报
	productKey := d.ProductKey
	deviceName := d.Name
	if len(deviceIdentification) == 1 {
		productKey = deviceIdentification[0].ProductKey
		deviceName = deviceIdentification[0].DeviceName
	}
	if len(deviceIdentification) > 1 {
		return ErrorCodeParamNotValid
	}
	// 组装消息
	payload := ShadowReportPayload{
		CommonRequest: CommonRequest{
			ID:      getRandomStringWithTimeSuffix(8),
			Version: "1.0",
		},
		Params: ShadowReportParam{
			Version:  GetTimeMill(),
			Reported: reported.Reported,
		},
	}

	payloadBytes, _ := json.Marshal(payload)
	if token := d.client.Publish(fmt.Sprintf(ShadowReportTopic, productKey, deviceName), 0, false, payloadBytes); token.Wait() && token.Error() != nil {
		d.sdkLogf(LogLevelError, "pub ShadowUpdateReport failed,clientId = %s,reply = %s, err: %s", formatMqttUserName(d.ProductKey, d.Name), string(payloadBytes), token.Error())
		return ErrorCodePubFailed
	}
	return SuccessCode
}

// GetShadowDesired 主动调用，主动获取设备影子期望值，返回的消息会发送至ShadowSetHandler中进行处理
// 传递 DeviceIdentification 为子设备获取设备影子期望值需要指定子设备标识
// 返回 成功标识
func (d *Device) GetShadowDesired(deviceIdentification ...DeviceIdentification) ErrorCode {
	// 为子设备获取设备影子期望值需要指定子设备标识，否则默认获取设备自身的影子期望值
	productKey := d.ProductKey
	deviceName := d.Name
	if len(deviceIdentification) == 1 {
		productKey = deviceIdentification[0].ProductKey
		deviceName = deviceIdentification[0].DeviceName
	}
	if len(deviceIdentification) > 1 {
		return ErrorCodeParamNotValid
	}
	// 组装消息
	payload := CommonRequest{
		ID:      getRandomStringWithTimeSuffix(8),
		Version: "1.0",
	}
	payloadBytes, _ := json.Marshal(payload)
	if token := d.client.Publish(fmt.Sprintf(ShadowDesiredGetTopic, productKey, deviceName), 0, false, payloadBytes); token.Wait() && token.Error() != nil {
		d.sdkLogf(LogLevelError, "pub GetShadowDesired failed,clientId = %s,reply = %s, err: %s", formatMqttUserName(d.ProductKey, d.Name), string(payloadBytes), token.Error())
		return ErrorCodePubFailed
	}
	return SuccessCode
}

func (d *Device) createShadowDesiredSetMqttHandler(productKey, deviceName string) func(client paho.Client, message paho.Message) {
	shadowSetHandler := func(client paho.Client, message paho.Message) {
		go func() {
			var payload ShadowDesiredSetPayload
			err := json.Unmarshal(message.Payload(), &payload)
			if err != nil {
				d.sdkLogf(LogLevelError, "unmarshal shadowSet message payload failed,clientId = %s,message = %s", formatMqttUserName(d.ProductKey, d.Name), message.Payload())
				return
			}

			errorCode := d.shadowSetHandler(ShadowSetMessage{
				DeviceIdentification: DeviceIdentification{
					ProductKey: productKey,
					DeviceName: deviceName,
				},
				ShadowDesiredParam: payload.Params,
			})
			if errorCode != SuccessCode {
				return
			}
			// 影子数据处理完毕后，发布clear消息
			payloadBytes, err := json.Marshal(ClearPayload{
				CommonRequest: CommonRequest{
					ID:      getRandomStringWithTimeSuffix(8),
					Version: "1.0",
				},
				ShadowVersion: GetTimeMill(),
			})
			if err != nil {
				d.sdkLogf(LogLevelError, "marshal shadowClear message payload failed,clientId = %s,message = %s", formatMqttUserName(d.ProductKey, d.Name), message.Payload())
				return
			}
			if token := d.client.Publish(fmt.Sprintf(ShadowDesiredClearTopic, productKey, deviceName), 0, false, payloadBytes); token.Wait() && token.Error() != nil {
				d.sdkLogf(LogLevelError, "pub shadowClear failed,clientId = %s,reply = %s, err: %s", formatMqttUserName(d.ProductKey, d.Name), string(payloadBytes), token.Error())
			}
		}()
	}
	return shadowSetHandler
}

func (d *Device) createShadowGetReplyMqttHandler(productKey, deviceName string) func(client paho.Client, message paho.Message) {
	shadowSetHandler := func(client paho.Client, message paho.Message) {
		go func() {
			var payload ShadowGetReplyPayload
			err := json.Unmarshal(message.Payload(), &payload)
			if err != nil {
				d.sdkLogf(LogLevelError, "unmarshal shadowSet message payload failed,clientId = %s,message = %s", formatMqttUserName(d.ProductKey, d.Name), message.Payload())
				return
			}

			var errorCode ErrorCode
			err = d.panicToError("createShadowGetReplyMqttHandler", func() {
				errorCode = d.shadowSetHandler(ShadowSetMessage{
					DeviceIdentification: DeviceIdentification{
						ProductKey: productKey,
						DeviceName: deviceName,
					},
					ShadowDesiredParam: payload.Data,
				})
			})
			if err != nil {
				return
			}
			if errorCode != SuccessCode {
				return
			}
			// 影子数据处理完毕后，发布clear消息
			payloadBytes, err := json.Marshal(ClearPayload{
				CommonRequest: CommonRequest{
					ID:      getRandomStringWithTimeSuffix(8),
					Version: "1.0",
				},
				ShadowVersion: GetTimeMill(),
			})
			if err != nil {
				d.sdkLogf(LogLevelError, "marshal shadowClear message payload failed,clientId = %s,message = %s, err: %s", formatMqttUserName(d.ProductKey, d.Name), message.Payload(), err)
				return
			}
			if token := d.client.Publish(fmt.Sprintf(ShadowDesiredClearTopic, productKey, deviceName), 0, false, payloadBytes); token.Wait() && token.Error() != nil {
				d.sdkLogf(LogLevelError, "pub shadowClear failed,clientId = %s,reply = %s, err: %s", formatMqttUserName(d.ProductKey, d.Name), string(payloadBytes), token.Error())
			}
		}()
	}
	return shadowSetHandler
}
