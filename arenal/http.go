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
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
)

// 获取http接入token
// 有过期时间，需要自己处理token过期
func (d *Device) GetDeviceToken() (*GetDeviceTokenResult, ErrorCode) {
	param := GetDeviceTokenParam{
		DeviceIdentification: DeviceIdentification{
			ProductKey: d.ProductKey,
			DeviceName: d.Name,
		},
		ClientID:   formatMqttUserName(d.ProductKey, d.Name),
		Timestamp:  GetTimeMill(),
		InstanceID: d.InstanceID,
	}
	sign, err := hmacSHA256EncryptGetTokenParam(param, d.Secret)
	if err != nil {
		d.sdkLogf(LogLevelError, "Encrypt failed,err: %s", err.Error())
		return nil, HTTPErrorCodeEncrypt
	}
	param.Sign = sign
	v := url.Values{}
	v.Set(ActionKey, GetDeviceTokenAction)
	v.Set(VersionKey, APIVerion)
	query := v.Encode()
	u := fmt.Sprintf("%s/%v/%v", d.formatCloudHTTPUrl(), APIVerion, GetDeviceTokenAction) + "?" + query
	body, err := d.sendHTTPPostRequest(u, nil, param)
	if err != nil {
		d.sdkLogf(LogLevelError, "sendHTTPPostRequest failed, %s", err.Error())
		return nil, HTTPErrorCodeDoRequest
	}
	var response GetDeviceTokenResponse
	err = json.Unmarshal(body, &response)
	if err != nil {
		d.sdkLogf(LogLevelError, "unmarshal response body failed,body: %s, err: %s", string(body), err.Error())
		return nil, HTTPErrorCodeUnmarshalBody
	}
	if response.ResponseMetadata.Error != nil {
		d.sdkLogf(LogLevelError, "GetDeviceToken failed, err: %+v", response.ResponseMetadata)
		return nil, convertCloudCodeToDeviceErrorCode(response.ResponseMetadata.Error.CodeN)
	}
	return &response.Result, SuccessCode
}

func hmacSHA256EncryptGetTokenParam(param GetDeviceTokenParam, secret string) (string, error) {
	// 对参数（client_id、device_name、product_key、timestamp）按字典序升序排序
	content := fmt.Sprintf("client_id=%v&device_name=%v&product_key=%s&timestamp=%d", param.ClientID, param.DeviceName, param.ProductKey, param.Timestamp)
	h := hmac.New(sha256.New, []byte(secret))
	_, err := h.Write([]byte(content))
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(h.Sum(nil)), nil
}

// 发布消息
func (d *Device) PublishMessageHTTP(param *PublishMessageParam) (*PublishMessageHTTPResult, ErrorCode) {
	if param.AccessToken == "" || param.Payload == nil || param.Topic == "" {
		return nil, HTTPErrorCodeParamInvalid
	}
	publishParam := PublishMessageHTTPParam{
		Topic:      param.Topic,
		Payload:    param.Payload,
		InstanceID: d.InstanceID,
	}
	v := url.Values{}
	v.Set(ActionKey, DeviceMessagePublishHTTPAction)
	v.Set(VersionKey, APIVerion)
	query := v.Encode()
	u := fmt.Sprintf("%s/%v/%v/%v/%v", d.formatCloudHTTPUrl(), APIVerion, DeviceMessagePublishHTTPAction, d.ProductKey, d.Name) + "?" + query
	headers := map[string]string{
		IOTHTTPAccessHeaderKey: param.AccessToken,
	}
	body, err := d.sendHTTPPostRequest(u, headers, publishParam)
	if err != nil {
		d.sdkLogf(LogLevelError, "sendHTTPPostRequest failed, %s", err.Error())
		return nil, HTTPErrorCodeDoRequest
	}
	var response PublishMessageHTTPResponse
	err = json.Unmarshal(body, &response)
	if err != nil {
		d.sdkLogf(LogLevelError, "unmarshal response body failed,body: %s, err: %s", string(body), err.Error())
		return nil, HTTPErrorCodeUnmarshalBody
	}
	if response.ResponseMetadata.Error != nil {
		d.sdkLogf(LogLevelError, "PublishMessageHTTP failed, err: %+v", response.ResponseMetadata)
		return nil, convertCloudCodeToDeviceErrorCode(response.ResponseMetadata.Error.CodeN)
	}
	return &response.Result, SuccessCode
}
