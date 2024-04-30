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
	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"
	"math/rand"
	"strings"
	"time"
)

type Gateway interface {
	AddTopologies(subDevices []DeviceAuthWithProductSecret) ([]DeviceIdentification, ErrorCode)
	DeleteTopologies(subDevices []DeviceIdentification) ErrorCode
	GetTopologies() ErrorCode
	GetSubDeviceSecrets(subDevices []DeviceIdentification) ErrorCode
	ReportDiscoveryList(subDevices []DeviceIdentification) ErrorCode
	SubDeviceLogin(subDevices []DeviceIdentification) ([]DeviceIdentification, ErrorCode)
	SubDeviceLogout(subDevices []DeviceIdentification) ErrorCode
	SetAddTopologiesReplyHandler(handler GatewayCommonHandler)
	SetDeleteTopologiesReplyHandler(handler GatewayCommonHandler)
	SetGetTopologiesReplyHandler(handler GatewayCommonHandler)
	SetNotifyAddSubDeviceHandler(handler GatewayCommonHandler)
	SetTopologyChangeHandler(handler TopologyChangeHandler)
	SetSubDeviceChangeHandler(handler SubDeviceChangeHandler)
	SetSubDeviceSecretGetReplyHandler(handler SubDeviceSecretGetReplyHandler)
	SetSubDeviceLoginReplyHandler(handler GatewayCommonHandler)
	SetSubDeviceLogoutReplyHandler(handler GatewayCommonHandler)
}

func (d *Device) gatewayInit() bool {
	d.setGatewayDefaultHandler()
	ok := d.subGatewayDefaultTopic()
	if !ok {
		return false
	}
	errCode := d.GetSubDeviceSecrets([]DeviceIdentification{})
	if errCode != SuccessCode {
		return false
	}
	go d.reportSubDeviceLoginLoop()
	return true
}

func (d *Device) gatewayRestore() bool {
	ok := d.subGatewayDefaultTopic()
	if !ok {
		d.sdkLogf(LogLevelError, "Subscribe gateway topic failed")
		return false
	}
	d.subDeviceLoginMapMtx.RLock()
	defer d.subDeviceLoginMapMtx.RUnlock()
	for k, _ := range d.subDeviceLoginMap {
		productKey, deviceName := getDeviceInfoFromClientId(k)
		subOK := d.subDefaultTopics(productKey, deviceName)
		if !subOK {
			d.sdkLogf(LogLevelError, "Subscribe default topic of subdevice %s %s failed", productKey, deviceName)
			return false
		}
	}
	return true
}

func (d *Device) subGatewayDefaultTopic() bool {
	// 订阅添加拓扑关系响应topic
	topic := fmt.Sprintf(GatewayTopoAddReplyTopic, d.ProductKey, d.Name)
	clientID := formatMqttUserName(d.ProductKey, d.Name)
	if token := d.client.Subscribe(topic, 1, d.createTopoAddReplyMqttHandler(d.addTopologiesReplyHandler)); token.Wait() && token.Error() != nil {
		d.sdkLogf(LogLevelError, "Device Sub Topic [%s] Failed! clientId = %s, err: %s", topic, clientID, token.Error())
		return false
	}
	// 订阅删除拓扑关系响应topic
	topic = fmt.Sprintf(GatewayTopoDeleteReplyTopic, d.ProductKey, d.Name)
	if token := d.client.Subscribe(topic, 1, d.createTopoDeleteReplyMqttHandler()); token.Wait() && token.Error() != nil {
		d.sdkLogf(LogLevelError, "Device Sub Topic [%s] Failed! clientId = %s, err: %s", topic, clientID, token.Error())
		return false
	}
	// 订阅获取拓扑关系响应topic
	topic = fmt.Sprintf(GatewayTopoGetReplyTopic, d.ProductKey, d.Name)
	if token := d.client.Subscribe(topic, 1, d.createTopoReplyMqttHandler(d.getTopologiesReplyHandler)); token.Wait() && token.Error() != nil {
		d.sdkLogf(LogLevelError, "Device Sub Topic [%s] Failed! clientId = %s, err: %s", topic, clientID, token.Error())
		return false
	}
	// 订阅子设备上线响应topic
	topic = fmt.Sprintf(GatewaySubDeviceLoginReplyTopic, d.ProductKey, d.Name)
	if token := d.client.Subscribe(topic, 1, d.createSubDeviceLoginReplyMqttHandler()); token.Wait() && token.Error() != nil {
		d.sdkLogf(LogLevelError, "Device Sub Topic [%s] Failed! clientId = %s, err: %s", topic, clientID, token.Error())
		return false
	}
	// 订阅子设备下线响应topic
	topic = fmt.Sprintf(GatewaySubDeviceLogoutReplyTopic, d.ProductKey, d.Name)
	if token := d.client.Subscribe(topic, 1, d.createSubDeviceLogoutReplyMqttHandler()); token.Wait() && token.Error() != nil {
		d.sdkLogf(LogLevelError, "Device Sub Topic [%s] Failed! clientId = %s, err: %s", topic, clientID, token.Error())
		return false
	}
	// 订阅通知添加拓扑关系topic
	topic = fmt.Sprintf(GatewayTopoAddNotifyTopic, d.ProductKey, d.Name)
	if token := d.client.Subscribe(topic, 1, d.createNotifyAddSubDeviceMqttHandler()); token.Wait() && token.Error() != nil {
		d.sdkLogf(LogLevelError, "Device Sub Topic [%s] Failed! clientId = %s, err: %s", topic, clientID, token.Error())
		return false
	}
	// 订阅拓扑关系变化topic
	topic = fmt.Sprintf(GatewayTopoChangeTopic, d.ProductKey, d.Name)
	if token := d.client.Subscribe(topic, 1, d.createTopoChangeMqttHandler()); token.Wait() && token.Error() != nil {
		d.sdkLogf(LogLevelError, "Device Sub Topic [%s] Failed! clientId = %s, err: %s", topic, clientID, token.Error())
		return false
	}
	// 订阅子设备状态变化topic
	topic = fmt.Sprintf(GatewaySubDeviceChangeTopic, d.ProductKey, d.Name)
	if token := d.client.Subscribe(topic, 1, d.createSubDeviceChangeMqttHandler()); token.Wait() && token.Error() != nil {
		d.sdkLogf(LogLevelError, "Device Sub Topic [%s] Failed! clientId = %s, err: %s", topic, clientID, token.Error())
		return false
	}
	// 订阅子设备密钥获取响应topic
	topic = fmt.Sprintf(GatewaySubDeviceSecretGetReplyTopic, d.ProductKey, d.Name, "+")
	if token := d.client.Subscribe(topic, 1, d.createSubDeviceSecretGetReplyMqttHandler()); token.Wait() && token.Error() != nil {
		d.sdkLogf(LogLevelError, "Device Sub Topic [%s] Failed! clientId = %s, err: %s", topic, clientID, token.Error())
		return false
	}
	return true
}

func (d *Device) setGatewayDefaultHandler() {
	if d.addTopologiesReplyHandler == nil {
		d.addTopologiesReplyHandler = func(subDevices []DeviceIdentification) ErrorCode {
			d.sdkLog(LogLevelInfo, "You Haven't Set The AddTopologiesReplyHandler,please use function SetAddTopologiesReplyHandler to set your handler.")
			return ErrorCodeHandlerNotExist
		}
	}

	if d.deleteTopologiesReplyHandler == nil {
		d.deleteTopologiesReplyHandler = func(subDevices []DeviceIdentification) ErrorCode {
			d.sdkLog(LogLevelInfo, "You Haven't Set The DeleteTopologiesReplyHandler,please use function SetDeleteTopologiesReplyHandler to set your handler.")
			return ErrorCodeHandlerNotExist
		}
	}

	if d.getTopologiesReplyHandler == nil {
		d.getTopologiesReplyHandler = func(subDevices []DeviceIdentification) ErrorCode {
			d.sdkLog(LogLevelInfo, "You Haven't Set The GetTopologiesReplyHandler,please use function SetGetTopologiesReplyHandler to set your handler.")
			return ErrorCodeHandlerNotExist
		}
	}

	if d.subDeviceLoginReplyHandler == nil {
		d.subDeviceLoginReplyHandler = func(subDevices []DeviceIdentification) ErrorCode {
			d.sdkLog(LogLevelInfo, "You Haven't Set The SubDeviceLoginReplyHandler,please use function SetSubDeviceLoginReplyHandler to set your handler.")
			return ErrorCodeHandlerNotExist
		}
	}

	if d.subDeviceLogoutReplyHandler == nil {
		d.subDeviceLogoutReplyHandler = func(subDevices []DeviceIdentification) ErrorCode {
			d.sdkLog(LogLevelInfo, "You Haven't Set The SubDeviceLogoutReplyHandler,please use function SetSubDeviceLogoutReplyHandler to set your handler.")
			return ErrorCodeHandlerNotExist
		}
	}

	if d.notifyAddSubDeviceHandler == nil {
		d.notifyAddSubDeviceHandler = func(subDevices []DeviceIdentification) ErrorCode {
			d.sdkLog(LogLevelInfo, "You Haven't Set The NotifyAddSubDeviceHandler,please use function SetNotifyAddSubDeviceHandler to set your handler.")
			return ErrorCodeHandlerNotExist
		}
	}

	if d.subDeviceSecretGetReplyHandler == nil {
		d.subDeviceSecretGetReplyHandler = func(subDeviceSecrets []DeviceAuthWithDeviceSecret) ErrorCode {
			d.sdkLog(LogLevelInfo, "You Haven't Set The SubDeviceSecretGetReplyHandler,please use function SetSubDeviceSecretGetReplyHandler to set your handler.")
			return ErrorCodeHandlerNotExist
		}
	}

	if d.topologyChangeHandler == nil {
		d.topologyChangeHandler = func(param TopoChangeParam) ErrorCode {
			d.sdkLog(LogLevelInfo, "You Haven't Set The TopologyChangeHandler,please use function SetTopologyChangeHandler to set your handler.")
			return ErrorCodeHandlerNotExist
		}
	}

	if d.subDeviceChangeHandler == nil {
		d.subDeviceChangeHandler = func(param SubDeviceChangeParam) ErrorCode {
			d.sdkLog(LogLevelInfo, "You Haven't Set The SubDeviceChangeHandler,please use function SetSubDeviceChangeHandler to set your handler.")
			return ErrorCodeHandlerNotExist
		}
	}
}

func (d *Device) createTopoReplyMqttHandler(handler GatewayCommonHandler) func(client paho.Client, message paho.Message) {
	return func(client paho.Client, message paho.Message) {
		go func() {
			var reply TopoReplyParam
			if json.Unmarshal(message.Payload(), &reply) != nil {
				d.sdkLogf(LogLevelError, "unmarshal topoAdd reply payload failed,clientId = %s,message = %s", formatMqttUserName(d.ProductKey, d.Name), message.Payload())
				return
			}
			if reply.Code != SuccessCode {
				d.sdkLogf(LogLevelError, "add subdevice failed,clientId = %s,message = %s, code = %v", formatMqttUserName(d.ProductKey, d.Name), message.Payload(), reply.Code)
				return
			}
			d.panicToError("createTopoReplyMqttHandler", func() {
				handler(reply.Data)
			})
		}()
	}
}

func (d *Device) createTopoAddReplyMqttHandler(handler GatewayCommonHandler) func(client paho.Client, message paho.Message) {
	return func(client paho.Client, message paho.Message) {
		go func() {
			var reply SubDeviceSecretReply
			if json.Unmarshal(message.Payload(), &reply) != nil {
				d.sdkLogf(LogLevelError, "unmarshal topoAdd reply payload failed,clientId = %s,message = %s", formatMqttUserName(d.ProductKey, d.Name), message.Payload())
				return
			}
			if reply.Code != SuccessCode {
				d.sdkLogf(LogLevelError, "add subdevice failed,clientId = %s,message = %s code = %v", formatMqttUserName(d.ProductKey, d.Name), message.Payload(), reply.Code)
				return
			}
			subDevices := make([]DeviceIdentification, 0)
			for _, subDevice := range reply.Data {
				subDevices = append(subDevices, DeviceIdentification{
					ProductKey: subDevice.ProductKey,
					DeviceName: subDevice.DeviceName,
				})
				secret := getSecret(subDevice.DeviceSecret.Payload, d.Secret)
				d.subDeviceSecretMap.Store(formatMqttUserName(subDevice.ProductKey, subDevice.DeviceName), secret)
			}
			d.panicToError("createTopoAddReplyMqttHandler", func() {
				handler(subDevices)
			})
		}()
	}
}

func (d *Device) createTopoDeleteReplyMqttHandler() func(client paho.Client, message paho.Message) {
	return func(client paho.Client, message paho.Message) {
		go func() {
			var reply TopoReplyParam
			if json.Unmarshal(message.Payload(), &reply) != nil {
				d.sdkLogf(LogLevelError, "unmarshal topoAdd reply payload failed,clientId = %s,message = %s", formatMqttUserName(d.ProductKey, d.Name), message.Payload())
				return
			}
			if reply.Code != SuccessCode {
				d.sdkLogf(LogLevelError, "delete subdevice failed,clientId = %s,message = %s, code = %v", formatMqttUserName(d.ProductKey, d.Name), message.Payload(), reply.Code)
				return
			}
			go d.unsubscribeDefaultTopics(reply.Data)

			d.panicToError("createTopoDeleteReplyMqttHandler", func() {
				d.deleteTopologiesReplyHandler(reply.Data)
			})
		}()
	}
}

func (d *Device) createSubDeviceLoginReplyMqttHandler() func(client paho.Client, message paho.Message) {
	return func(client paho.Client, message paho.Message) {
		go func() {
			var reply TopoReplyParam
			if json.Unmarshal(message.Payload(), &reply) != nil {
				d.sdkLogf(LogLevelError, "unmarshal topoAdd reply payload failed,clientId = %s,message = %s", formatMqttUserName(d.ProductKey, d.Name), message.Payload())
				return
			}
			if reply.Code != SuccessCode {
				d.sdkLogf(LogLevelError, "subdevice login failed,clientId = %s,message = %s, code = %v", formatMqttUserName(d.ProductKey, d.Name), message.Payload(), reply.Code)
				return
			}
			for _, subDevice := range reply.Data {
				d.subDefaultTopics(subDevice.ProductKey, subDevice.DeviceName)
				d.subDeviceLoginMapMtx.Lock()
				d.subDeviceLoginMap[formatMqttUserName(subDevice.ProductKey, subDevice.DeviceName)] = struct{}{}
				d.subDeviceLoginMapMtx.Unlock()
				d.getDeviceTask(subDevice.ProductKey, subDevice.DeviceName)
			}

			d.panicToError("createSubDeviceLoginReplyMqttHandler", func() {
				d.subDeviceLoginReplyHandler(reply.Data)
			})
			go d.reportSubDeviceLogin()
		}()
	}
}

func (d *Device) createSubDeviceLogoutReplyMqttHandler() func(client paho.Client, message paho.Message) {
	return func(client paho.Client, message paho.Message) {
		go func() {
			var reply TopoReplyParam
			if json.Unmarshal(message.Payload(), &reply) != nil {
				d.sdkLogf(LogLevelError, "unmarshal topoAdd reply payload failed,clientId = %s,message = %s", formatMqttUserName(d.ProductKey, d.Name), message.Payload())
				return
			}
			if reply.Code != SuccessCode {
				d.sdkLogf(LogLevelError, "subdevice logout failed,clientId = %s,message = %s, code = %v", formatMqttUserName(d.ProductKey, d.Name), message.Payload(), reply.Code)
				return
			}
			go d.unsubscribeDefaultTopics(reply.Data)

			d.panicToError("createSubDeviceLogoutReplyMqttHandler", func() {
				d.subDeviceLogoutReplyHandler(reply.Data)
			})
			go d.reportSubDeviceLogin()
		}()
	}
}

func (d *Device) unsubscribeDefaultTopics(subDevices []DeviceIdentification) {
	topics := make([]string, 0)
	for _, subDevice := range subDevices {
		topics = append(topics, fmt.Sprintf("sys/%s/%s/#", subDevice.ProductKey, subDevice.DeviceName))
		d.subDeviceLoginMapMtx.Lock()
		delete(d.subDeviceLoginMap, formatMqttUserName(subDevice.ProductKey, subDevice.DeviceName))
		d.subDeviceLoginMapMtx.Unlock()
	}
	if token := d.client.Unsubscribe(topics...); token.Wait() && token.Error() != nil {
		d.sdkLogf(LogLevelError, "Unsubscribe device default topic faield, err: %s", token.Error())
	}
}

func (d *Device) createNotifyAddSubDeviceMqttHandler() func(client paho.Client, message paho.Message) {
	return func(client paho.Client, message paho.Message) {
		go func() {
			topicFields := strings.Split(message.Topic(), "/")
			traceID := topicFields[len(topicFields)-1]
			var request NotifyAddTopoRequest
			if json.Unmarshal(message.Payload(), &request) != nil {
				d.sdkLogf(LogLevelError, "unmarshal notify add subdevice payload failed,clientId = %s,message = %s", formatMqttUserName(d.ProductKey, d.Name), message.Payload())
				return
			}
			topic := fmt.Sprintf(GatewayTopoAddNotifyReplyTopic, d.ProductKey, d.Name, traceID)

			reply := NotifyAddTopoReply{
				CommonReply: CommonReply{
					ID:   request.ID,
					Code: 0,
				},
				Timestamp: request.Timestamp,
			}

			defer func() {
				payloadBytes, _ := json.Marshal(reply)
				if token := d.client.Publish(topic, 1, false, payloadBytes); token.Wait() && token.Error() != nil {
					d.sdkLogf(LogLevelError, "pub DeleteTopologies failed,clientId = %s,reply = %s, err: %s", formatMqttUserName(d.ProductKey, d.Name), string(payloadBytes), token.Error())
				}
			}()

			err := d.panicToError("createNotifyAddSubDeviceMqttHandler", func() {
				reply.Code = d.notifyAddSubDeviceHandler(request.SubDevices)
			})
			if err != nil {
				reply.Code = ErrorCodeFuncExecPanic
			}
		}()
	}
}

func (d *Device) createTopoChangeMqttHandler() func(client paho.Client, message paho.Message) {
	return func(client paho.Client, message paho.Message) {
		go func() {
			var payload TopoChangePayload
			if json.Unmarshal(message.Payload(), &payload) != nil {
				d.sdkLogf(LogLevelError, "unmarshal topoAdd reply payload failed,clientId = %s,message = %s", formatMqttUserName(d.ProductKey, d.Name), message.Payload())
				return
			}

			reply := CommonReply{
				ID:   payload.ID,
				Code: 0,
			}

			defer func() {
				payloadBytes, _ := json.Marshal(reply)
				topic := fmt.Sprintf(GatewayTopoChangeReplyTopic, d.ProductKey, d.Name)
				if token := d.client.Publish(topic, 1, false, payloadBytes); token.Wait() && token.Error() != nil {
					d.sdkLogf(LogLevelError, "pub GatewayTopoChangeReply failed,clientId = %s,reply = %s, err: %s", formatMqttUserName(d.ProductKey, d.Name), string(payloadBytes), token.Error())
				}
			}()

			topoChangeParam := TopoChangeParam{
				OperateType: payload.Params.OperateType,
			}
			for _, subDevice := range payload.Params.SubDevices {
				topoChangeParam.SubDevices = append(topoChangeParam.SubDevices, DeviceIdentification{
					ProductKey: subDevice.ProductKey,
					DeviceName: subDevice.DeviceName,
				})
				if payload.Params.OperateType == TopologyOperateTypeCreate {
					d.subDeviceSecretMap.Store(formatMqttUserName(subDevice.ProductKey, subDevice.DeviceName),
						getSecret(subDevice.DeviceSecret.Payload, d.Secret))
				}
			}
			if payload.Params.OperateType == TopologyOperateTypeDelete || payload.Params.OperateType == TopologyOperateTypeDisable {
				go d.unsubscribeDefaultTopics(topoChangeParam.SubDevices)
			}

			err := d.panicToError("createTopoChangeMqttHandler", func() {
				reply.Code = d.topologyChangeHandler(topoChangeParam)
			})
			if err != nil {
				reply.Code = ErrorCodeFuncExecPanic
			}
		}()
	}
}

func (d *Device) createSubDeviceChangeMqttHandler() func(client paho.Client, message paho.Message) {
	return func(client paho.Client, message paho.Message) {
		go func() {
			var payload SubDeviceChangePayload
			if json.Unmarshal(message.Payload(), &payload) != nil {
				d.sdkLogf(LogLevelError, "unmarshal topoAdd reply payload failed,clientId = %s,message = %s", formatMqttUserName(d.ProductKey, d.Name), message.Payload())
				return
			}

			reply := CommonReply{
				ID:   payload.ID,
				Code: SuccessCode,
			}

			defer func() {
				payloadBytes, _ := json.Marshal(reply)
				topic := fmt.Sprintf(GatewaySubDeviceChangeReplyTopic, d.ProductKey, d.Name)
				if token := d.client.Publish(topic, 1, false, payloadBytes); token.Wait() && token.Error() != nil {
					d.sdkLogf(LogLevelError, "pub SubDeviceChangeReply failed,clientId = %s,reply = %s, err: %s", formatMqttUserName(d.ProductKey, d.Name), string(payloadBytes), token.Error())
				}
			}()

			err := d.panicToError("createSubDeviceChangeMqttHandler", func() {
				reply.Code = d.subDeviceChangeHandler(payload.Params)
			})
			if err != nil {
				reply.Code = ErrorCodeFuncExecPanic
			}
		}()
	}
}

func (d *Device) createSubDeviceSecretGetReplyMqttHandler() func(client paho.Client, message paho.Message) {
	return func(client paho.Client, message paho.Message) {
		d.sdkLog(LogLevelInfo, "receive sub device secret get reply")
		go func() {
			var reply SubDeviceSecretReply
			if json.Unmarshal(message.Payload(), &reply) != nil {
				d.sdkLogf(LogLevelError, "unmarshal secret get reply payload failed,clientId = %s,message = %s", formatMqttUserName(d.ProductKey, d.Name), message.Payload())
				return
			}
			if reply.Code != SuccessCode {
				d.sdkLogf(LogLevelError, "subdevice secret get failed,clientId = %s,message = %s, code = %v", formatMqttUserName(d.ProductKey, d.Name), message.Payload(), reply.Code)
				return
			}
			deviceSecret := make([]DeviceAuthWithDeviceSecret, 0)
			for _, subDevice := range reply.Data {
				secret := getSecret(subDevice.DeviceSecret.Payload, d.Secret)
				deviceSecret = append(deviceSecret, DeviceAuthWithDeviceSecret{
					DeviceIdentification: subDevice.DeviceIdentification,
					DeviceSecret:         secret,
				})
				d.subDeviceSecretMap.Store(formatMqttUserName(subDevice.ProductKey, subDevice.DeviceName), secret)
			}

			d.panicToError("createSubDeviceSecretGetReplyMqttHandler", func() {
				d.subDeviceSecretGetReplyHandler(deviceSecret)
			})
		}()
	}
}

// AddTopologies 将子设备添加到网关拓扑关系，遇到错误时返回失败的设备列表，添加的结果在AddTopologiesReplyHandler中处理
// 传递 subDevices 子设备身份标识和产品密钥
func (d *Device) AddTopologies(subDevices []DeviceAuthWithProductSecret) (failedDevices []DeviceIdentification, errCode ErrorCode) {
	errCode = SuccessCode
	request := TopologyRequestWithAuth{
		CommonRequest: CommonRequest{
			ID:      getRandomStringWithTimeSuffix(8),
			Version: "1.0",
		},
		SubDevices: make([]SubDeviceParamWithAuth, 0),
	}
	for _, device := range subDevices {
		if !CheckDeviceName(device.DeviceName) {
			d.sdkLogf(LogLevelError, "设备名称 %s 非法, %s", device.DeviceName, ErrorCodeDeviceNameInvalidDesc)
			failedDevices = append(failedDevices, device.DeviceIdentification)
			errCode = ErrorCodeDeviceNameInvalid
			continue
		}

		subDeviceParam := SubDeviceParamWithAuth{
			DeviceIdentification: device.DeviceIdentification,
			RandomNum:            rand.Int63(),
			Timestamp:            time.Now().Unix(),
		}
		var err error
		subDeviceParam.Signature, err = hmacSHA256EncryptSubDeviceParam(subDeviceParam, device.ProductSecret)
		if err != nil {
			failedDevices = append(failedDevices, device.DeviceIdentification)
			errCode = ErrorCodeEncryptFailed
		}
		request.SubDevices = append(request.SubDevices, subDeviceParam)
	}
	if len(failedDevices) != 0 {
		return failedDevices, errCode
	}
	payloadBytes, _ := json.Marshal(request)
	if token := d.client.Publish(fmt.Sprintf(GatewayTopoAddTopic, d.ProductKey, d.Name), 1, false, payloadBytes); token.Wait() && token.Error() != nil {
		d.sdkLogf(LogLevelError, "pub AddTopologies failed,clientId = %s,reply = %s, err: %s", formatMqttUserName(d.ProductKey, d.Name), string(payloadBytes), token.Error())
		for _, d := range subDevices {
			failedDevices = append(failedDevices, d.DeviceIdentification)
		}
		return failedDevices, ErrorCodePubFailed
	}
	return
}

// DeleteTopologies 将子设备从网关拓扑关系中删除，删除的结果在DeleteTopologiesReplyHandler中处理
// 传递 subDevices 子设备身份标识
func (d *Device) DeleteTopologies(subDevices []DeviceIdentification) ErrorCode {
	request := TopologyRequest{
		CommonRequest: CommonRequest{
			ID:      getRandomStringWithTimeSuffix(8),
			Version: "1.0",
		},
		SubDevices: subDevices,
	}
	payloadBytes, _ := json.Marshal(request)
	if token := d.client.Publish(fmt.Sprintf(GatewayTopoDeleteTopic, d.ProductKey, d.Name), 1, false, payloadBytes); token.Wait() && token.Error() != nil {
		d.sdkLogf(LogLevelError, "pub DeleteTopologies failed,clientId = %s,reply = %s, err: %s", formatMqttUserName(d.ProductKey, d.Name), string(payloadBytes), token.Error())
		return ErrorCodePubFailed
	}
	return SuccessCode
}

// GetTopologies 从平台拉取最新拓扑关系，异步方法，结果在GetTopologiesReplyHandler中处理
func (d *Device) GetTopologies() ErrorCode {
	request := CommonRequest{
		ID:      getRandomStringWithTimeSuffix(8),
		Version: "1.0",
	}
	payloadBytes, _ := json.Marshal(request)
	if token := d.client.Publish(fmt.Sprintf(GatewayTopoGetTopic, d.ProductKey, d.Name), 1, false, payloadBytes); token.Wait() && token.Error() != nil {
		d.sdkLogf(LogLevelError, "pub GetTopologies failed,clientId = %s,reply = %s, err: %s", formatMqttUserName(d.ProductKey, d.Name), string(payloadBytes), token.Error())
		return ErrorCodePubFailed
	}
	return SuccessCode
}

// GetSubDeviceSecrets 获取子设备密钥，异步方法，结果在SubDeviceSecretGetReplyHandler中处理
// 传递 subDevices 要获取密钥的子设备标识，为空则获取全部子设备密钥
func (d *Device) GetSubDeviceSecrets(subDevices []DeviceIdentification) ErrorCode {
	request := TopologyRequest{
		CommonRequest: CommonRequest{
			ID:      getRandomStringWithTimeSuffix(8),
			Version: "1.0",
		},
		SubDevices: subDevices,
	}
	payloadBytes, _ := json.Marshal(request)
	if token := d.client.Publish(fmt.Sprintf(GatewaySubDeviceSecretGetTopic, d.ProductKey, d.Name, uuid.New().String()), 1, false, payloadBytes); token.Wait() && token.Error() != nil {
		d.sdkLogf(LogLevelError, "pub GetSubDeviceSecrets failed,clientId = %s,reply = %s", formatMqttUserName(d.ProductKey, d.Name), string(payloadBytes))
		return ErrorCodePubFailed
	}
	return SuccessCode
}

func (d *Device) getSubDeviceSecretsSync(subDevices []DeviceIdentification) ([]DeviceAuthWithDeviceSecret, ErrorCode) {
	request := TopologyRequest{
		CommonRequest: CommonRequest{
			ID:      getRandomStringWithTimeSuffix(8),
			Version: "1.0",
		},
		SubDevices: subDevices,
	}
	payloadBytes, _ := json.Marshal(request)
	traceID := uuid.New().String()
	payloadBytes, errCode := syncCall(d.client, SyncCallParam{
		RequestTopic: fmt.Sprintf(GatewaySubDeviceSecretGetTopic, d.ProductKey, d.Name, traceID),
		ReplyTopic:   fmt.Sprintf(GatewaySubDeviceSecretGetReplyTopic, d.ProductKey, d.Name, traceID),
		Payload:      string(payloadBytes),
	})
	if errCode != SuccessCode {
		d.sdkLogf(LogLevelError, "syncCall failed, error code: %d", errCode)
		return nil, errCode
	}
	var reply SubDeviceSecretReply
	if json.Unmarshal(payloadBytes, &reply) != nil {
		d.sdkLogf(LogLevelError, "unmarshal secret get reply payload failed,clientId = %s,message = %s", formatMqttUserName(d.ProductKey, d.Name), payloadBytes)
		return nil, ErrorCodeDecodeFailed
	}
	if reply.Code != SuccessCode {
		d.sdkLogf(LogLevelError, "subdevice secret get failed,clientId = %s,message = %s, code = %v", formatMqttUserName(d.ProductKey, d.Name), payloadBytes, reply.Code)
		return nil, reply.Code
	}
	deviceSecrets := make([]DeviceAuthWithDeviceSecret, 0)
	for _, subDevice := range reply.Data {
		secret := getSecret(subDevice.DeviceSecret.Payload, d.Secret)
		deviceSecrets = append(deviceSecrets, DeviceAuthWithDeviceSecret{
			DeviceIdentification: subDevice.DeviceIdentification,
			DeviceSecret:         secret,
		})
		d.subDeviceSecretMap.Store(formatMqttUserName(subDevice.ProductKey, subDevice.DeviceName), secret)
	}
	return deviceSecrets, SuccessCode
}

// SubDeviceLogin 子设备登陆，遇到错误时返回失败的设备列表，异步方法，结果在SubDeviceLoginReplyHandler中处理
// 传递 subDevices 要登录的子设备
func (d *Device) SubDeviceLogin(subDevices []DeviceIdentification) (failedDevices []DeviceIdentification, errCode ErrorCode) {
	errCode = SuccessCode
	request := SubDeviceLoginRequest{
		CommonRequest: CommonRequest{
			ID:      getRandomStringWithTimeSuffix(8),
			Version: "1.0",
		},
		TriggerTime: currentTimeMill(),
		SubDevices:  make([]SubDeviceParamWithAuth, 0),
	}
	subDevicesToGetSecret := make([]DeviceIdentification, 0)
	for _, subDevice := range subDevices {
		subDeviceParam := SubDeviceParamWithAuth{
			DeviceIdentification: subDevice,
			RandomNum:            rand.Int63(),
			Timestamp:            time.Now().Unix(),
		}
		var err error
		secret, ok := d.subDeviceSecretMap.Load(formatMqttUserName(subDevice.ProductKey, subDevice.DeviceName))
		if !ok {
			subDevicesToGetSecret = append(subDevicesToGetSecret, subDevice)
			continue
		}
		subDeviceParam.Signature, err = hmacSHA256EncryptSubDeviceParam(subDeviceParam, secret.(string))
		if err != nil {
			failedDevices = append(failedDevices, subDevice)
		}
		request.SubDevices = append(request.SubDevices, subDeviceParam)
	}
	if len(failedDevices) != 0 {
		return failedDevices, ErrorCodeEncryptFailed
	}
	if len(subDevicesToGetSecret) != 0 {
		deviceSecrets, errCode := d.getSubDeviceSecretsSync(subDevicesToGetSecret)
		if errCode != SuccessCode {
			return subDevicesToGetSecret, errCode
		}
		for _, subDevice := range deviceSecrets {
			subDeviceParam := SubDeviceParamWithAuth{
				DeviceIdentification: subDevice.DeviceIdentification,
				RandomNum:            rand.Int63(),
				Timestamp:            time.Now().Unix(),
			}
			var err error
			subDeviceParam.Signature, err = hmacSHA256EncryptSubDeviceParam(subDeviceParam, subDevice.DeviceSecret)
			if err != nil {
				failedDevices = append(failedDevices, subDevice.DeviceIdentification)
			}
			request.SubDevices = append(request.SubDevices, subDeviceParam)
		}
		if len(failedDevices) != 0 {
			return failedDevices, ErrorCodeEncryptFailed
		}
	}
	payloadBytes, _ := json.Marshal(request)
	if token := d.client.Publish(fmt.Sprintf(GatewaySubDeviceLoginTopic, d.ProductKey, d.Name), 1, false, payloadBytes); token.Wait() && token.Error() != nil {
		d.sdkLogf(LogLevelError, "pub SubDeviceLogin failed,clientId = %s,reply = %s, err: %s", formatMqttUserName(d.ProductKey, d.Name), string(payloadBytes), token.Error())
		return subDevices, ErrorCodePubFailed
	}
	return
}

// SubDeviceLogout 子设备下线，异步方法，结果在SubDeviceLogoutReplyHandler中处理
// 传递 subDevices 要下线的子设备
func (d *Device) SubDeviceLogout(subDevices []DeviceIdentification) ErrorCode {
	request := SubDeviceLogoutRequest{
		CommonRequest: CommonRequest{
			ID:      getRandomStringWithTimeSuffix(8),
			Version: "1.0",
		},
		TriggerTime: currentTimeMill(),
		SubDevices:  subDevices,
	}
	payloadBytes, _ := json.Marshal(request)
	if token := d.client.Publish(fmt.Sprintf(GatewaySubDeviceLogoutTopic, d.ProductKey, d.Name), 1, false, payloadBytes); token.Wait() && token.Error() != nil {
		d.sdkLogf(LogLevelError, "pub SubDeviceLogout failed,clientId = %s,reply = %s, err: %s", formatMqttUserName(d.ProductKey, d.Name), string(payloadBytes), token.Error())
		return ErrorCodePubFailed
	}
	return SuccessCode
}

// ReportDiscoveryList 上报网关发现的子设备列表，需要在平台进行确认才会添加拓扑关系。无需确认时直接用AddTopologies方法
// 传递 subDevices 发现的子设备
func (d *Device) ReportDiscoveryList(subDevices []DeviceIdentification) ErrorCode {
	request := TopologyRequest{
		CommonRequest: CommonRequest{
			ID:      getRandomStringWithTimeSuffix(8),
			Version: "1.0",
		},
		SubDevices: subDevices,
	}
	payloadBytes, _ := json.Marshal(request)
	if token := d.client.Publish(fmt.Sprintf(GatewaySubDeviceDiscoveryTopic, d.ProductKey, d.Name), 1, false, payloadBytes); token.Wait() && token.Error() != nil {
		d.sdkLogf(LogLevelError, "pub ReportDiscoveryList failed,clientId = %s,reply = %s, err: %s", formatMqttUserName(d.ProductKey, d.Name), string(payloadBytes), token.Error())
		return ErrorCodePubFailed
	}
	return SuccessCode
}

// SetAddTopologiesReplyHandler 配置添加拓扑关系返回消息的Handler
func (d *Device) SetAddTopologiesReplyHandler(handler GatewayCommonHandler) {
	d.addTopologiesReplyHandler = handler
}

// SetAddTopologiesReplyHandler 配置删除拓扑关系返回消息的Handler
func (d *Device) SetDeleteTopologiesReplyHandler(handler GatewayCommonHandler) {
	d.deleteTopologiesReplyHandler = handler
}

// SetAddTopologiesReplyHandler 配置获取拓扑关系返回消息的Handler
func (d *Device) SetGetTopologiesReplyHandler(handler GatewayCommonHandler) {
	d.getTopologiesReplyHandler = handler
}

// SetNotifyAddSubDeviceHandler 设置通知添加子设备消息的Handler
// 调用ReportDiscoveryList上报后在平台确认添加时，设备会收到NotifyAddSubDevice消息
func (d *Device) SetNotifyAddSubDeviceHandler(handler GatewayCommonHandler) {
	d.notifyAddSubDeviceHandler = handler
}

// SetSubDeviceSecretGetReplyHandler 设置获取子设备密钥返回消息的Handler
func (d *Device) SetSubDeviceSecretGetReplyHandler(handler SubDeviceSecretGetReplyHandler) {
	d.subDeviceSecretGetReplyHandler = handler
}

// SetSubDeviceLoginReplyHandler 设置子设备登陆返回消息的Handler
func (d *Device) SetSubDeviceLoginReplyHandler(handler GatewayCommonHandler) {
	d.subDeviceLoginReplyHandler = handler
}

// SetSubDeviceLogoutReplyHandler 设置子设备下线返回消息的Handler
func (d *Device) SetSubDeviceLogoutReplyHandler(handler GatewayCommonHandler) {
	d.subDeviceLogoutReplyHandler = handler
}

// SetTopologyChangeHandler 设置拓扑关系变化消息的Handler，拓扑关系变化包括：创建/删除/启用/禁用
func (d *Device) SetTopologyChangeHandler(handler TopologyChangeHandler) {
	d.topologyChangeHandler = handler
}

// SetSubDeviceChangeHandler 设置子设备变化消息的Handler，子设备变化包括：删除/启用/禁用
func (d *Device) SetSubDeviceChangeHandler(handler SubDeviceChangeHandler) {
	d.subDeviceChangeHandler = handler
}

func hmacSHA256EncryptSubDeviceParam(param SubDeviceParamWithAuth, secret string) (string, error) {
	// 对参数（device_name、random_num、product_key、timestamp）按字典序升序排序
	content := fmt.Sprintf("device_name=%s&random_num=%d&product_key=%s&timestamp=%d", param.DeviceName, param.RandomNum, param.ProductKey, param.Timestamp)
	h := hmac.New(sha256.New, []byte(secret))
	_, err := h.Write([]byte(content))
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(h.Sum(nil)), nil
}

func (d *Device) reportSubDeviceLoginLoop() {
	reportSubDeviceLoginIntv := 5 * time.Minute
	if d.ReportSubDeviceLoginIntv != nil {
		reportSubDeviceLoginIntv = *d.ReportSubDeviceLoginIntv
	}
	ticker := time.NewTicker(reportSubDeviceLoginIntv)
	for {
		select {
		case <-ticker.C:
			d.reportSubDeviceLogin()
		case <-d.ctx.Done():
			return
		}
	}
}

func (d *Device) reportSubDeviceLogin() {
	if !d.client.IsConnected() {
		return
	}
	subDevices := make([]DeviceIdentification, 0)
	for k := range d.subDeviceLoginMap {
		productKey, deviceName := getDeviceInfoFromClientId(k)
		subDevices = append(subDevices, DeviceIdentification{
			ProductKey: productKey,
			DeviceName: deviceName,
		})
	}
	request := SubDeviceLoginReportRequest{
		CommonRequest: CommonRequest{
			ID:      getRandomStringWithTimeSuffix(8),
			Version: "1.0",
		},
		Params: SubDeviceLoginReportParams{
			TriggerTime: currentTimeMill(),
		},
	}
	request.Params.SubDevices = make([]DeviceIdentification, 0)
	d.subDeviceLoginMapMtx.RLock()
	for k := range d.subDeviceLoginMap {
		productKey, deviceName := getDeviceInfoFromClientId(k)
		request.Params.SubDevices = append(request.Params.SubDevices, DeviceIdentification{
			ProductKey: productKey,
			DeviceName: deviceName,
		})
	}
	d.subDeviceLoginMapMtx.RUnlock()
	payloadBytes, _ := json.Marshal(request)
	if token := d.client.Publish(fmt.Sprintf(GatewaySubDeviceLoginReport, d.ProductKey, d.Name), 1, false, payloadBytes); token.Wait() && token.Error() != nil {
		d.sdkLogf(LogLevelError, "pub SubDeviceLoginReport failed,clientId = %s,reply = %s, err: %s", formatMqttUserName(d.ProductKey, d.Name), string(payloadBytes), token.Error())
	}
}
