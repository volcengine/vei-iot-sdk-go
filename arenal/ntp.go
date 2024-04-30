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
	"math/rand"
	"net/url"
	"time"
)

// SendNTPRequest 获取服务端当前时间戳，无需init，单位ms
// 获取服务端事件
func (d *Device) GetServerTime() (int64, ErrorCode) {
	basicParam := DynamicRegisterBasicParam{
		InstanceID: d.InstanceID,
		ProductKey: d.ProductKey,
		DeviceName: d.Name,
		RandomNum:  rand.Int63(),
		Timestamp:  time.Now().Unix(),
		AuthType:   AuthTypePreRegistration,
	}

	param := GetServerTimeParam{
		DynamicRegisterParam: DynamicRegisterParam{
			DynamicRegisterBasicParam: basicParam,
		},
	}
	// 优先用设备密钥
	if d.Secret != "" {
		var err error
		param.Signature, err = hmacSHA256Encrypt(basicParam, d.Secret)
		if err != nil {
			return 0, ErrorCodeEncryptFailed
		}
		param.SecretType = SecretTypeDevice
	} else if d.ProductSecret != "" {
		var err error
		param.Signature, err = hmacSHA256Encrypt(basicParam, d.ProductSecret)
		if err != nil {
			return 0, ErrorCodeEncryptFailed
		}
		param.SecretType = SecretTypeProduct
	} else {
		return 0, ErrorCodeParamNotValid
	}

	param.DeviceSendTime = currentTimeMill()
	v := url.Values{}
	v.Set(ActionKey, DynamicRegisterAction)
	v.Set(VersionKey, APIVerion)
	query := v.Encode()
	u := fmt.Sprintf("%s/2021-12-14/GetServerTime", d.formatCloudHTTPUrl()) + "?" + query
	body, err := d.sendHTTPPostRequest(u, nil, param)
	if err != nil {
		d.sdkLogf(LogLevelError, "sendHTTPPostRequest failed, %s", err.Error())
		return 0, ErrorCodeInternalError
	}
	var response GetServerTimeHttpResponse
	err = json.Unmarshal(body, &response)
	if err != nil {
		return 0, ErrorCodeInternalError
	}
	if response.ResponseMetadata.Error != nil {
		return 0, ErrorCode(response.ResponseMetadata.Error.CodeN)
	}

	deviceRecvTime := GetTimeMill()
	serverTime := (response.Result.ServerRecvTime + response.Result.ServerSendTime + deviceRecvTime - response.Result.DeviceSendTime) / 2
	return serverTime, SuccessCode
}

// SendNTPRequest 发送NTP请求到服务端
// 获取服务端事件
func (d *Device) SendNTPRequest() ErrorCode {
	payload := NTPRequest{
		CommonRequest: CommonRequest{
			ID:      getRandomStringWithTimeSuffix(8),
			Version: "1.0",
		},
		DeviceSendTime: GetTimeMill(),
	}
	payloadBytes, _ := json.Marshal(payload)
	if token := d.client.Publish(fmt.Sprintf(NTPRequestTopic, d.ProductKey, d.Name), 0, false, payloadBytes); token.Wait() && token.Error() != nil {
		d.sdkLogf(LogLevelError, "pub ntp request failed,clientId = %s,payload = %s, err: %s", formatMqttUserName(d.ProductKey, d.Name), string(payloadBytes), token.Error().Error())
		return ErrorCodePubFailed
	}
	return SuccessCode
}

// SetNTPResponseHandler 配置NTP响应的Handler
// serverTimeMS 服务端当前时间戳（单位ms）
func (d *Device) SetNTPResponseHandler(handler NTPResponseHandler) {
	d.ntpResponseHandler = handler
}

func (d *Device) createNTPResponseMQTTHandler() func(client paho.Client, message paho.Message) {
	return func(client paho.Client, message paho.Message) {
		go func() {
			var response NTPResponse
			err := json.Unmarshal(message.Payload(), &response)
			if err != nil {
				d.sdkLogf(LogLevelError, "ntp response parse payload failed, payload is: %s, err is : %s", message.Payload(), err.Error())
				return
			}

			deviceRecvTime := GetTimeMill()
			serverTime := (response.Data.ServerRecvTime + response.Data.ServerSendTime + deviceRecvTime - response.Data.DeviceSendTime) / 2
			d.panicToError("createNTPResponseMQTTHandler", func() {
				d.ntpResponseHandler(serverTime)
			})
		}()
	}
}
