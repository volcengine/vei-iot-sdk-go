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
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/volcengine/vei-iot-sdk-go/arenal/common/webshell/bashexec"
	"github.com/volcengine/vei-iot-sdk-go/arenal/common/webshell/webtty"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"sync"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

var webttyMap sync.Map

type BaseDevice interface {
	Init() bool
	DisConnect()
	Close()
	IsConnected() bool
	SetPropertySetHandler(handler PropertySetHandler)
	SetServiceCallHandler(handler ServiceCallHandler)
	SetShadowSetHandler(handler ShadowSetHandler)
	SetEventReportReplyHandler(handler EventReportReplyHandler)
	SetPropertyReportReplyHandler(handler PropertyReportReplyHandler)
	SetCustomTopicHandler(topic string, handler CustomTopicHandler)
	PropertyReport(properties map[string]PropertyValue, deviceIdentification ...DeviceIdentification) ErrorCode
	PropertyReportAsync(properties map[string]PropertyValue, deviceIdentification ...DeviceIdentification) (paho.Token, ErrorCode)
	ShadowReport(reported Reported, deviceIdentification ...DeviceIdentification) ErrorCode
	GetShadowDesired(deviceIdentification ...DeviceIdentification) ErrorCode
	EventReport(param EventReportParam, deviceIdentification ...DeviceIdentification) ErrorCode
	SendCustomTopicMessage(topicSuffix, payload string, qos int) ErrorCode
	SetCustomJobHandler(handler CustomJobHandler)
	SendNTPRequest() ErrorCode
	SetNTPResponseHandler(handler NTPResponseHandler)
	GetServerTime() (int64, ErrorCode)
	SetOnConnectHandler(handler OnConnectHandler)
	UploadFile(param UploadFileParam) (string, ErrorCode)
	GetDeviceToken() (*GetDeviceTokenResult, ErrorCode)
	PublishMessageHTTP(param *PublishMessageParam) (*PublishMessageHTTPResult, ErrorCode)
}

func CreateDeviceWithConfig(config DeviceOption) Device {
	return Device{
		DeviceOption:      config,
		subDeviceLoginMap: make(map[string]struct{}),
	}
}

func (d *Device) fillOption() {
	if d.AccessType == "" {
		d.AccessType = DeviceAccessTypeMQTT
	}
	// log report option
	d.DeviceOption.LogReportConfig.logSendMinInterval = time.Minute * 5
	d.DeviceOption.LogReportConfig.logSendChannel = make(chan Log, 40)
	// map init
	if d.DeviceOption.ModuleConfig == nil {
		d.DeviceOption.ModuleConfig = make(map[string]ModuleInfo)
	}
	if d.DeviceOption.SubDeviceModuleConfig == nil {
		d.DeviceOption.SubDeviceModuleConfig = make(map[string]ModuleInfo)
	}
	if d.DeviceOption.UserDefineTopicQosMap == nil {
		d.DeviceOption.UserDefineTopicQosMap = make(map[string]byte)
	}
}

func (d *Device) validateOption() bool {
	if d.AccessType != DeviceAccessTypeHTTP && d.AccessType != DeviceAccessTypeMQTT {
		d.sdkLogf(LogLevelError, "AccessType %v not supported", d.AccessType)
		return false
	}
	if d.DeviceOption.ProductKey == "" {
		d.sdkLogf(LogLevelError, "ProductKey can not be empty")
		return false
	}
	if d.DeviceOption.Name == "" {
		d.sdkLogf(LogLevelError, "DeviceName can not be empty")
		return false
	}
	switch d.DeviceOption.VerifyMode {
	case VerifyModeDeviceSecret:
		if d.DeviceOption.Secret == "" {
			d.sdkLogf(LogLevelError, "DeviceSecret can not be empty when verifyMode is %v", VerifyModeDeviceSecret)
			return false
		}
	case VerifyModeDynamicNoPreRegistered, VerifyModeDynamicPreRegistered:
		if d.DeviceOption.ProductSecret == "" {
			d.sdkLogf(LogLevelError, "ProductSecret can not be empty when using dynamicRegister")
			return false
		}
	default:
		d.sdkLogf(LogLevelError, "VerifyMode invalid, support %v, %v, %v", VerifyModeDeviceSecret, VerifyModeDynamicPreRegistered, VerifyModeDynamicNoPreRegistered)
		return false
	}
	if d.DeviceOption.CloudHTTPHost == "" {
		d.sdkLogf(LogLevelError, "CloudHTTPHost can not be empty ")
		return false
	}
	if d.DeviceOption.InstanceID == "" {
		d.sdkLogf(LogLevelError, "InstanceID can not be empty ")
		return false
	}
	// 检查接入方式相关的参数
	switch d.AccessType {
	case DeviceAccessTypeHTTP:
		return d.checkHTTPOption()
	case DeviceAccessTypeMQTT:
		return d.checkMQTTOption()
	}
	return true
}

func (d *Device) checkMQTTOption() bool {
	if d.DeviceOption.MQTTHost == "" {
		d.sdkLogf(LogLevelError, "MQTTHost can not be empty ")
		return false
	}
	return true
}

func (d *Device) checkHTTPOption() bool {
	return true
}

func (d *Device) formatBrokerUrl() string {
	port := d.MQTTPort
	if port == 0 {
		port = DefaultMQTTPort
	}
	if d.EnableTls {
		return fmt.Sprintf("mqtts://%v:%v", d.MQTTHost, port)
	}
	return fmt.Sprintf("mqtt://%v:%v", d.MQTTHost, port)
}

func (d *Device) formatCloudHTTPUrl() string {
	return fmt.Sprintf("https://%v", d.CloudHTTPHost)
}

func (d *Device) initMQTT() bool {
	d.setMQTTDebugLogger()

	// 初始化mqtt client
	opts := paho.NewClientOptions()
	opts.AddBroker(d.formatBrokerUrl())
	clientID := formatMqttUserName(d.ProductKey, d.Name)
	if d.AllowMultiple {
		clientID = fmt.Sprintf("%s|%s", clientID, tmpSuffix)
	}

	if d.EnableTls {
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM([]byte(emqxCA))
		opts.SetTLSConfig(&tls.Config{
			RootCAs: caCertPool,
		})
	}
	opts.SetClientID(clientID)
	opts.SetAutoReconnect(true)
	if d.KeepAlive != nil {
		opts.SetKeepAlive(time.Duration(*d.KeepAlive) * time.Second)
	} else {
		opts.SetKeepAlive(defaultKeepAlive * time.Second)
	}
	opts.SetCleanSession(false)
	opts.SetWriteTimeout(3 * time.Second)
	opts.SetStore(paho.NewFileStore(path.Join(d.FilePath, "qos1")))
	opts.SetConnectTimeout(5 * time.Second)
	opts.SetCredentialsProvider(func() (username string, password string) {
		return formatMqttUserName(d.ProductKey, d.Name), generateMqttPassword(d.authType, d.ProductKey, d.Name, d.Secret)
	})
	d.firstConnect.Store(true)

	opts.OnConnect = func(client paho.Client) {
		if !d.firstConnect.Load() {
			retryCount := 0
			subSuccess := false
			for retryCount < subscriptionMaxRetryCount {
				ok := d.subDefaultTopics(d.ProductKey, d.Name)
				if ok {
					subSuccess = true
					break
				}
				time.Sleep(subscriptionRetryIntv)
				retryCount++
			}
			if !subSuccess {
				d.sdkLogf(LogLevelError, "Subscribe device default topic failed!")
				return
			}
			if d.IsGateway {
				retryCount = 0
				subSuccess = false
				for retryCount < subscriptionMaxRetryCount {
					ok := d.gatewayRestore()
					if ok {
						subSuccess = true
						break
					}
					time.Sleep(subscriptionRetryIntv)
					retryCount++
				}
				if !subSuccess {
					d.sdkLogf(LogLevelError, "Subscribe gateway default topic failed!")
					return
				}
			}
		}
		d.firstConnect.Store(false)
		d.sdkLogf(LogLevelInfo, "Device Connect Success!productKey=%s,deviceName=%s,deviceSecret=%s,clientId=%s", d.ProductKey, d.Name, d.Secret, formatMqttUserName(d.ProductKey, d.Name))
		if d.onConnectHandler != nil {
			d.panicToError("onConnectHandler", func() {
				d.onConnectHandler()
			})
		}
	}

	opts.OnConnectionLost = func(client paho.Client, err error) {
		d.sdkLogf(LogLevelError, "Device Connect Loss!productKey=%s,deviceName=%s,deviceSecret=%s,clientId=%s, err: %s", d.ProductKey, d.Name, d.Secret, formatMqttUserName(d.ProductKey, d.Name), err)
	}

	// 为所有Topic设置默认Handler
	d.setDefaultHandler()

	if d.WebshellExecutor == "" {
		d.WebshellExecutor = "sh"
	}

	if d.OTAHistoryJobReloadMode == "" {
		d.OTAHistoryJobReloadMode = OTAReloadModeIgnore
	}

	d.client = paho.NewClient(opts)
	if token := d.client.Connect(); token.Wait() && token.Error() != nil {
		d.sdkLogf(LogLevelError, "Device Connect Failed!err=%s,productKey=%s,deviceName=%s,deviceSecret=%s,clientId=%s", token.Error(), d.ProductKey, d.Name, d.Secret, formatMqttUserName(d.ProductKey, d.Name))
		return false
	}

	// 订阅设备相关topic
	ok := d.subDefaultTopics(d.ProductKey, d.Name)
	if !ok {
		d.sdkLogf(LogLevelError, "Device Sub Topic Failed!")
		return false
	}

	// 拉取未完成任务
	d.getDeviceTask(d.ProductKey, d.Name)

	// 网关相关初始化
	if d.IsGateway {
		ok = d.gatewayInit()
		if !ok {
			d.sdkLogf(LogLevelError, "Gateway Init Failed!")
			return false
		}
	}

	// 重启OTA历史任务
	d.reloadHistoryOTAJob()

	// 加载未完成的任务
	d.loadExecutingJob()

	// 开启协程，每天0点查询是否有升级任务
	go d.queryUpgradeJob()

	return true
}

// Init 设备初始化方法，设备必须经过初始化才能够使用
// 必须对设备所属产品、产品密钥、设备名称、服务质量QoS、认证方式、所属区域进行配置
// 文件下载路径：FilePath默认配置为当前工作目录，密钥Secret可以通过配置认证方式为VerifyModeDynamicPreRegistered、VerifyModeDynamicNoPreRegistered动态获取
// Warning: 在Init函数失败或者调用Device.Close()之后不要复用Device，否则可能会造成内存泄露
func (d *Device) Init() bool {
	d.initLock.Lock()
	defer d.initLock.Unlock()
	if d.initSuccess {
		return true
	}
	d.ctx, d.cancel = context.WithCancel(context.Background())
	defer func() {
		if d.initSuccess {
			return
		}
		d.Close()
	}()
	d.fillOption()

	if !d.validateOption() {
		return false
	}
	d.httpClient = &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 50,
			MaxConnsPerHost:     50,
			IdleConnTimeout:     time.Minute,
		},
		Timeout: time.Second * 3,
	}

	ok := d.initLogConfig()
	if !ok {
		return false
	}

	// 认证
	switch d.VerifyMode {
	case VerifyModeDeviceSecret:
		d.authType = AuthTypePreRegistration
	case VerifyModeDynamicPreRegistered:
		d.authType = AuthTypePreRegistration
		secret, err := d.dynamicRegister(d.authType)
		if err != nil {
			d.sdkLogf(LogLevelError, "DynamicRegister Failed,err:%s", err)
			return false
		}
		d.Secret = secret
	case VerifyModeDynamicNoPreRegistered:
		if !CheckDeviceName(d.Name) {
			d.sdkLogf(LogLevelError, "设备名称 %s 非法, %s", d.Name, ErrorCodeDeviceNameInvalidDesc)
			return false
		}

		d.authType = AuthTypeNoPreRegistration
		secret, err := d.dynamicRegister(d.authType)
		if err != nil {
			d.sdkLogf(LogLevelError, "DynamicRegister Failed,err:%s", err)
			return false
		}
		d.Secret = secret
	}
	switch d.AccessType {
	case DeviceAccessTypeMQTT:
		d.initSuccess = d.initMQTT()
	case DeviceAccessTypeHTTP:
		// http方式不需要特殊初始化
		d.initSuccess = true
	default:
		d.sdkLogf(LogLevelError, "access_type %v not supported", d.AccessType)
		return false
	}
	return d.initSuccess
}

func (d *Device) setMQTTDebugLogger() {
	if !d.MQTTDebugLogEnabled {
		return
	}
	paho.DEBUG = NewConsoleLogger("[Debug] ")
	paho.CRITICAL = NewConsoleLogger("[CRITICAL] ")
	paho.WARN = NewConsoleLogger("[WARN] ")
	paho.ERROR = NewConsoleLogger("[Error] ")
}

func (d *Device) SetOnConnectHandler(handler OnConnectHandler) {
	d.onConnectHandler = handler
}

// Close()方法用于清理Device后台worker，包括日志、任务查询等
// Warning: 调用Close()之后不会等待所有worker都退出，所以调用Close()函数之后不要复用Device
func (d *Device) Close() {
	if d.cancel != nil {
		d.cancel()
	}
	d.DisConnect()
	d.initSuccess = false
}

// DisConnect 设备断开链接
// 调用方法可以断开设备的MQTT连接
func (d *Device) DisConnect() {
	if d.client == nil {
		return
	}
	d.client.Disconnect(0)
}

// IsConnected 判断设备是否连接
// 连接中返回true，未连接返回false
func (d *Device) IsConnected() bool {
	if d.client == nil {
		return false
	}

	return d.client.IsConnected()
}

// SetPropertySetHandler 配置处理属性设置消息的Handler
// 传递 PropertySetHandler 根据参数中的身份标识区分设备自身或子设备
func (d *Device) SetPropertySetHandler(handler PropertySetHandler) {
	d.propertySetHandler = handler
}

// SetServiceCallHandler 配置处理服务调用消息的Handler
// 传递 ServiceCallHandler 根据参数中的身份标识区分设备自身或子设备
func (d *Device) SetServiceCallHandler(handler ServiceCallHandler) {
	d.serviceCallHandler = handler
}

// SetEventReportReplyHandler 配置处理事件报告返回消息的Handler
// 传递 EventReportReplyHandler 根据参数中的身份标识区分设备自身或子设备
func (d *Device) SetEventReportReplyHandler(handler EventReportReplyHandler) {
	d.eventReportReplyHandler = handler
}

// SetPropertyReportReplyHandler 配置处理属性上报返回消息的Handler
// 传递 PropertyReportReplyHandler 根据参数中的身份标识区分设备自身或子设备
func (d *Device) SetPropertyReportReplyHandler(handler PropertyReportReplyHandler) {
	d.propertyReportReplyHandler = handler
}

// SetCustomTopicHandler 配置自定义topic的Handler，需要预先在平台创建自定义topic
// 自定义格式为 sys/{ProductKey}/{DeviceName}/custom/{TopicSuffix}
func (d *Device) SetCustomTopicHandler(topicSuffix string, handler CustomTopicHandler) {
	if len(d.customTopicHandlerMap) == 0 {
		d.customTopicHandlerMap = make(map[string]CustomTopicHandler)
	}
	d.customTopicHandlerMap[formatCustomTopic(d.ProductKey, d.Name, topicSuffix)] = handler
}

// PropertyReportAsync 主动调用，进行属性上报，该方法会立即返回，若需要感知发送状态，可以通过操作返回结果中的token获取发送状态， token.wait()，判断token.Error()
// 传递 map[string]PropertyValue key为 FormatUniqueFieldName 方法创建的唯一key，value为 PropertyValue; DeviceIdentification 为子设备进行属性上报还需要指定子设备标识
// 返回 成功标识
func (d *Device) PropertyReportAsync(propertyValues map[string]PropertyValue, deviceIdentification ...DeviceIdentification) (paho.Token, ErrorCode) {
	// 一个设备单次最多可上报属性500个
	if len(propertyValues) > reportPropertyLimit {
		return nil, ThingModelErrorCodeReportPropertyLimitReached
	}
	// 为子设备进行属性上报需要指定子设备标识，否则默认为设备自身进行属性上报
	productKey := d.ProductKey
	deviceName := d.Name
	if len(deviceIdentification) == 1 {
		productKey = deviceIdentification[0].ProductKey
		deviceName = deviceIdentification[0].DeviceName
	}
	if len(deviceIdentification) > 1 {
		return nil, ErrorCodeParamNotValid
	}
	// 组装消息
	payload := PropertyPayload{
		ID:      getRandomStringWithTimeSuffix(8),
		Version: "1.0",
		Params:  propertyValues,
	}

	payloadBytes, _ := json.Marshal(payload)
	if d.client == nil || !d.client.IsConnectionOpen() {
		d.sdkLogf(LogLevelError, "pub PropertyReport failed,clientId = %s,payload = %s,err = %s", formatMqttUserName(d.ProductKey, d.Name), string(payloadBytes), "client is reconnecting")
		go d.writePropertiesToFile(string(payloadBytes))
		return nil, ErrorCodePubFailed
	}
	token := d.client.Publish(fmt.Sprintf(PropertyReportTopic, productKey, deviceName), 1, false, payloadBytes)
	go d.handlerPropertyPublishToken(token, string(payloadBytes))
	return token, SuccessCode
}

func (d *Device) writePropertiesToFile(payloads string) {
	d.uploadPropertyFileLock.Lock()
	defer d.uploadPropertyFileLock.Unlock()
	dirPath := path.Join(d.FilePath, "device_property_history")
	if !fileExist(dirPath) {
		err := os.Mkdir(dirPath, 0750)
		if err != nil {
			d.sdkLogf(LogLevelError, "create device_property_history dir failed, %s", err.Error())
			return
		}
	}
	filePath := path.Join(dirPath, fmt.Sprintf("property_%s", time.Now().Format("2006-01-02-15")))
	err := writeContentToFile(fmt.Sprintf("%s\n", payloads), filePath)
	if err != nil {
		d.sdkLogf(LogLevelError, "write device property history to file failed, %s", err.Error())
	}
}

// PropertyReport 主动调用，进行属性上报，该方法会阻塞直到获取发送结果状态，在断连情况下若未收到ACK则会一直阻塞住
// 传递 map[string]PropertyValue key为 FormatUniqueFieldName 方法创建的唯一key，value为 PropertyValue; DeviceIdentification 为子设备进行属性上报还需要指定子设备标识
// 返回 成功标识
func (d *Device) PropertyReport(propertyValues map[string]PropertyValue, deviceIdentification ...DeviceIdentification) ErrorCode {
	// 一个设备单次最多可上报属性500个
	if len(propertyValues) > reportPropertyLimit {
		return ThingModelErrorCodeReportPropertyLimitReached
	}
	// 为子设备进行属性上报需要指定子设备标识，否则默认为设备自身进行属性上报
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
	payload := PropertyPayload{
		ID:      getRandomStringWithTimeSuffix(8),
		Version: "1.0",
		Params:  propertyValues,
	}

	payloadBytes, _ := json.Marshal(payload)
	if d.client == nil || !d.client.IsConnectionOpen() {
		d.sdkLogf(LogLevelError, "pub PropertyReport failed,clientId = %s,payload = %s,err = %s", formatMqttUserName(d.ProductKey, d.Name), string(payloadBytes), "client is reconnecting")
		go d.writePropertiesToFile(string(payloadBytes))
		return ErrorCodePubFailed
	}
	token := d.client.Publish(fmt.Sprintf(PropertyReportTopic, productKey, deviceName), 1, false, payloadBytes)

	if token.Wait() && token.Error() != nil {
		d.sdkLogf(LogLevelError, "pub PropertyReport failed,clientId = %s,payload = %+v,err = %s", formatMqttUserName(d.ProductKey, d.Name), propertyValues, token.Error())
		return ErrorCodePubFailed
	}

	return SuccessCode
}

func (d *Device) handlerPropertyPublishToken(token paho.Token, payload string) {
	if token.Wait(); token.Error() != nil {
		d.sdkLogf(LogLevelError, "pub PropertyReport failed,clientId = %s,payload = %s,err = %s", formatMqttUserName(d.ProductKey, d.Name), payload, token.Error())
		go d.writePropertiesToFile(payload)
		return
	}
	go func() {
		dirPath := path.Join(d.FilePath, "device_property_history")
		d.uploadPropertyFileLock.Lock()
		defer d.uploadPropertyFileLock.Unlock()
		if !fileExist(dirPath) {
			return
		}
		files, err := os.ReadDir(dirPath)
		if err != nil {
			d.sdkLogf(LogLevelError, "ls device_property_history dir failed, %s", err.Error())
			return
		}
	fileLoop:
		for _, file := range files {
			p := path.Join(dirPath, file.Name())
			f, err := os.Open(p)
			if err != nil {
				d.sdkLogf(LogLevelError, "open device_property_history file %s failed, %s", file.Name(), err.Error())
				continue
			}
			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				line := scanner.Text()
				var tmp PropertyPayload
				bytes := []byte(line)
				err = json.Unmarshal(bytes, &tmp)
				if err != nil {
					d.sdkLogf(LogLevelError, "unmarshal device_property_history failed, %s", err.Error())
					return
				}
				_, code := d.PropertyReportAsync(tmp.Params)
				interval := time.Second
				if d.UploadHistoryPropertyIntv != nil {
					interval = *d.UploadHistoryPropertyIntv
				}
				if interval > 0 {
					time.Sleep(interval)
				}
				if code != SuccessCode {
					continue fileLoop
				}
			}
			err = os.Remove(p)
			if err == nil {
				d.sdkLogf(LogLevelInfo, "upload PropertyReport file %s success,file delete success", p)
			} else {
				d.sdkLogf(LogLevelError, "upload PropertyReport file %s success,file delete failed,err=%s", p, err.Error())
			}
		}
	}()
}

// EventReport 主动调用，进行事件上报
// 传递  ModuleKey 事件所属模块标识; Identifier 事件标识; Values 输出参数列表:key为标识符,value为具体值; DeviceIdentification 为子设备进行事件上报还需要指定子设备标识
// 返回 成功标识
func (d *Device) EventReport(param EventReportParam, deviceIdentification ...DeviceIdentification) ErrorCode {
	// 为子设备进行事件上报需要指定子设备标识，否则默认为设备自身进行事件上报
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
	timeMill := GetTimeMill()
	payload := EventPayload{
		ID:      getRandomStringWithTimeSuffix(8),
		Version: "1.0",
		Params: EventParam{
			Values:   param.Values,
			TimeMill: timeMill,
		},
	}

	payloadBytes, _ := json.Marshal(payload)
	if token := d.client.Publish(fmt.Sprintf(EventReportTopic, productKey, deviceName, param.ModuleKey, param.Identifier), 1, false, payloadBytes); token.Wait() && token.Error() != nil {
		d.sdkLogf(LogLevelError, "pub EventReport failed,clientId = %s,reply = %s, err: %s", formatMqttUserName(d.ProductKey, d.Name), string(payloadBytes), token.Error())
		return ErrorCodePubFailed
	}
	return SuccessCode
}

// SendCustomTopicMessage 发送消息到自定义topic，需要预先在平台创建自定义topic
// 传递 topicSuffix 自定义topic后缀；payload 消息体；qos 服务质量，0: 最多传递一次，1: 至少传递一次
func (d *Device) SendCustomTopicMessage(topicSuffix, payload string, qos int) ErrorCode {
	if qos != 0 && qos != 1 {
		return ErrorCodeQoSNotSupport
	}
	if token := d.client.Publish(formatCustomTopic(d.ProductKey, d.Name, topicSuffix), byte(qos), false, payload); token.Wait() && token.Error() != nil {
		d.sdkLogf(LogLevelError, "pub CustomTopicMessage failed,clientId = %s,reply = %s, err: %s", formatMqttUserName(d.ProductKey, d.Name), payload, token.Error())
		return ErrorCodePubFailed
	}
	return SuccessCode
}

func (d *Device) subDefaultTopics(productKey, deviceName string) bool {
	// 订阅自定义Topic
	for t, h := range d.customTopicHandlerMap {
		if token := d.client.Subscribe(t, 1, d.createCustomTopicHandler(h)); token.Wait() && token.Error() != nil {
			d.sdkLogf(LogLevelError, "Device Sub Custom Topic [%s] Failed! clientId = %s,err: %s", t, formatMqttUserName(d.ProductKey, d.Name), token.Error())
			return false
		}
	}
	if d.AllowMultiple {
		return true
	}

	// 订阅平台服务调用的topic
	topic := fmt.Sprintf(ServiceCallTopicForSub, productKey, deviceName)
	if token := d.client.Subscribe(topic, 1, d.createServiceCallMqttHandler(productKey, deviceName)); token.Wait() && token.Error() != nil {
		d.sdkLogf(LogLevelError, "Device Sub Topic [%s] Failed! clientId = %s,err: %s", topic, formatMqttUserName(d.ProductKey, d.Name), token.Error())
		return false
	}

	// 订阅平台属性设置的topic
	topic = fmt.Sprintf(PropertySetTopic, productKey, deviceName)
	if token := d.client.Subscribe(topic, 1, d.createPropertySetMqttHandler(productKey, deviceName)); token.Wait() && token.Error() != nil {
		d.sdkLogf(LogLevelError, "Device Sub Topic [%s] Failed! clientId = %s,err: %s", topic, formatMqttUserName(d.ProductKey, d.Name), token.Error())
		return false
	}

	// 订阅设备影子下发的topic
	topic = fmt.Sprintf(ShadowDesiredSetTopic, productKey, deviceName)
	if token := d.client.Subscribe(topic, 1, d.createShadowDesiredSetMqttHandler(productKey, deviceName)); token.Wait() && token.Error() != nil {
		d.sdkLogf(LogLevelError, "Device Sub Topic [%s] Failed! clientId = %s,err: %s", topic, formatMqttUserName(d.ProductKey, d.Name), token.Error())
		return false
	}

	// 订阅获取设备影子reply的topic
	topic = fmt.Sprintf(ShadowDesiredGetReplyTopic, productKey, deviceName)
	if token := d.client.Subscribe(topic, 1, d.createShadowGetReplyMqttHandler(productKey, deviceName)); token.Wait() && token.Error() != nil {
		d.sdkLogf(LogLevelError, "Device Sub Topic [%s] Failed! clientId = %s,err: %s", topic, formatMqttUserName(d.ProductKey, d.Name), token.Error())
		return false
	}

	// 订阅事件上报应答的topic
	topic = fmt.Sprintf(EventReportReplyTopicForSub, productKey, deviceName)
	if token := d.client.Subscribe(topic, 1, d.createEventReportReplyMqttHandler(productKey, deviceName)); token.Wait() && token.Error() != nil {
		d.sdkLogf(LogLevelError, "Device Sub Topic [%s] Failed! clientId = %s,err: %s", topic, formatMqttUserName(d.ProductKey, d.Name), token.Error())
		return false
	}

	// 订阅属性上报应答的topic
	topic = fmt.Sprintf(PropertyReportReplyTopic, productKey, deviceName)
	if token := d.client.Subscribe(topic, 1, d.createPropertyReportReplyMqttHandler(productKey, deviceName)); token.Wait() && token.Error() != nil {
		d.sdkLogf(LogLevelError, "Device Sub Topic [%s] Failed! clientId = %s,err: %s", topic, formatMqttUserName(d.ProductKey, d.Name), token.Error())
		return false
	}

	// 订阅OTA推送升级通知的topic
	topic = fmt.Sprintf(OTAUpgradeNotifyTopic, productKey, deviceName, "+")
	if token := d.client.Subscribe(topic, 1, d.createOTAUpgradeNotifyMqttHandler(productKey, deviceName)); token.Wait() && token.Error() != nil {
		d.sdkLogf(LogLevelError, "Device Sub Topic [%s] Failed! clientId = %s,err: %s", topic, formatMqttUserName(d.ProductKey, d.Name), token.Error())
		return false
	}

	// 订阅OTA升级请求响应的topic
	topic = fmt.Sprintf(OTARequestUpgradeReplyTopic, productKey, deviceName)
	if token := d.client.Subscribe(topic, 1, d.createOTAUpgradeReplyMqttHandler(productKey, deviceName)); token.Wait() && token.Error() != nil {
		d.sdkLogf(LogLevelError, "Device Sub Topic [%s] Failed! clientId = %s,err: %s", topic, formatMqttUserName(d.ProductKey, d.Name), token.Error())
		return false
	}

	// 订阅webshell指令执行的的topic
	topic = fmt.Sprintf(WebShellCmdPostTopic, productKey, deviceName)
	if token := d.client.Subscribe(topic, 1, d.createWebshellCmdMqttHandler()); token.Wait() && token.Error() != nil {
		d.sdkLogf(LogLevelError, "Device Sub Topic [%s] Failed! clientId = %s,err: %s", topic, formatMqttUserName(d.ProductKey, d.Name), token.Error())
		return false
	}

	// 订阅延迟探测指令执行的的topic
	topic = fmt.Sprintf(DeviceDelayPostTopic, productKey, deviceName)
	if token := d.client.Subscribe(topic, 1, d.createDelayDetectHandler(productKey, deviceName)); token.Wait() && token.Error() != nil {
		d.sdkLogf(LogLevelError, "Device Sub Topic [%s] Failed! clientId = %s,err: %s", topic, formatMqttUserName(d.ProductKey, d.Name), token.Error())
		return false
	}

	// 订阅设备定时定量日志配置下发的topic
	topic = fmt.Sprintf(DeviceLogReportConfigTopic, productKey, deviceName)
	if token := d.client.Subscribe(topic, 1, d.createDeviceLogReportConfigHandler()); token.Wait() && token.Error() != nil {
		d.sdkLogf(LogLevelError, "Device Sub Topic [%s] Failed! clientId = %s,err: %s", topic, formatMqttUserName(d.ProductKey, d.Name), token.Error())
		return false
	}

	// 订阅实时流日志配置下发的topic
	topic = fmt.Sprintf(DeviceStreamLogConfigTopic, productKey, deviceName, "+")
	if token := d.client.Subscribe(topic, 1, d.createStreamLogConfigMqttHandler()); token.Wait() && token.Error() != nil {
		d.sdkLogf(LogLevelError, "Device Sub Topic [%s] Failed! clientId = %s,err: %s", topic, formatMqttUserName(d.ProductKey, d.Name), token.Error())
		return false
	}

	// 订阅本地日志配置下发的topic
	topic = fmt.Sprintf(DeviceLocalLogConfigTopic, productKey, deviceName, "+")
	if token := d.client.Subscribe(topic, 1, d.createLocalLogConfigMqttHandler()); token.Wait() && token.Error() != nil {
		d.sdkLogf(LogLevelError, "Device Sub Topic [%s] Failed! clientId = %s,err: %s", topic, formatMqttUserName(d.ProductKey, d.Name), token.Error())
		return false
	}

	// 订阅文件上传通知topic
	topic = fmt.Sprintf(DeviceNotifyUploadFileTopic, productKey, deviceName, "+")
	if token := d.client.Subscribe(topic, 1, d.createNotifyUploadFileMqttHandler()); token.Wait() && token.Error() != nil {
		d.sdkLogf(LogLevelError, "Device Sub Topic [%s] Failed! clientId = %s,err: %s", topic, formatMqttUserName(d.ProductKey, d.Name), token.Error())
		return false
	}

	// 订阅批量任务通知topic
	topic = fmt.Sprintf(TaskNotifyTopic, productKey, deviceName)
	if token := d.client.Subscribe(topic, 1, d.createTaskNotifyMqttHandler(productKey, deviceName)); token.Wait() && token.Error() != nil {
		d.sdkLogf(LogLevelError, "Device Sub Topic [%s] Failed! clientId = %s,err: %s", topic, formatMqttUserName(d.ProductKey, d.Name), token.Error())
		return false
	}

	// 订阅获取待执行任务reply topic
	topic = fmt.Sprintf(TaskGetReplyTopic, productKey, deviceName)
	if token := d.client.Subscribe(topic, 1, d.createTaskGetMqttHandler(productKey, deviceName)); token.Wait() && token.Error() != nil {
		d.sdkLogf(LogLevelError, "Device Sub Topic [%s] Failed! clientId = %s,err: %s", topic, formatMqttUserName(d.ProductKey, d.Name), token.Error())
		return false
	}

	// 订阅获取待执行任务reply topic
	topic = fmt.Sprintf(TaskProgressReplyTopic, productKey, deviceName)
	if token := d.client.Subscribe(topic, 1, d.createTaskProgressReplyMqttHandler(productKey, deviceName)); token.Wait() && token.Error() != nil {
		d.sdkLogf(LogLevelError, "Device Sub Topic [%s] Failed! clientId = %s,err: %s", topic, formatMqttUserName(d.ProductKey, d.Name), token.Error())
		return false
	}

	// 订阅ntp服务response topic
	topic = fmt.Sprintf(NTPResponseTopic, productKey, deviceName)
	if token := d.client.Subscribe(topic, 1, d.createNTPResponseMQTTHandler()); token.Wait() && token.Error() != nil {
		d.sdkLogf(LogLevelError, "Device Sub Topic [%s] Failed! clientId = %s,err: %s", topic, formatMqttUserName(d.ProductKey, d.Name), token.Error())
		return false
	}

	return true
}

func (d *Device) createDeviceLogReportConfigHandler() func(client paho.Client, message paho.Message) {
	return func(client paho.Client, message paho.Message) {
		reportConfig := LogReportConfigPayload{}
		err := json.Unmarshal(message.Payload(), &reportConfig)
		if err != nil {
			d.sdkLogf(LogLevelError, "Log Config Change Unmarshal Failed, err is : %s", err.Error())
			return
		}
		d.LogReportConfig.LogReportCommonConfig = reportConfig.Data
		d.sdkLogf(LogLevelInfo, "Log Config Change Success, new config: %#v", d.LogReportConfig.LogReportCommonConfig)
		// 修改配置文件，如果设备重启保留配置
		d.updateConfigFile()
	}
}

func (d *Device) createCustomTopicHandler(handler CustomTopicHandler) func(client paho.Client, message paho.Message) {
	return func(client paho.Client, message paho.Message) {
		go func() {
			d.panicToError("createCustomTopicHandler", func() {
				handler(CustomTopicParam{
					Topic:   message.Topic(),
					Payload: string(message.Payload()),
				})
			})
		}()
	}
}

func (d *Device) createWebshellCmdMqttHandler() func(client paho.Client, message paho.Message) {
	return func(client paho.Client, message paho.Message) {
		var cmdResult []byte

		args := strings.Split(message.Topic(), "/")
		uid := args[5]

		defer func() {
			if len(cmdResult) == 0 {
				return
			}
			if token := d.client.Publish(fmt.Sprintf(WebShellCmdReplyTopic, d.ProductKey, d.Name, uid), 0, false, cmdResult); token.Wait() && token.Error() != nil {
				d.sdkLogf(LogLevelError, "pub webshell cmd result failed,clientId = %s,reply = %s,err = %s", formatMqttUserName(d.ProductKey, d.Name), cmdResult, token.Error())
			}
		}()

		if _, ok := webttyMap.Load(uid); !ok {
			bashExec, err := bashexec.New(d.WebshellExecutor, nil)
			if err != nil {
				d.sdkLog(LogLevelError, fmt.Sprintf("设备命令执行器初始化异常!err=%s", err.Error()))
				cmdResult = []byte("设备命令执行器初始化异常")
				return
			}
			wt, err := webtty.New(client, fmt.Sprintf(WebShellCmdReplyTopic, d.ProductKey, d.Name, uid), bashExec)
			if err != nil {
				d.sdkLog(LogLevelError, fmt.Sprintf("设备命令执行器初始化异常!err=%s", err.Error()))
				cmdResult = []byte("设备命令执行器初始化异常")
				return
			}
			go wt.Run()
			webttyMap.Store(uid, wt)
		}

		if wt, ok := webttyMap.Load(uid); ok {
			err := wt.(*webtty.WebTTY).HandleMasterReadEvent(message.Payload())
			if err != nil {
				d.sdkLog(LogLevelError, fmt.Sprintf("设备命令执行异常!err=%s", err.Error()))
				cmdResult = []byte("设备命令执行异常")
			}
		}
	}
}

func (d *Device) createDelayDetectHandler(productKey, deviceName string) func(client paho.Client, message paho.Message) {
	return func(client paho.Client, message paho.Message) {
		args := strings.Split(message.Topic(), "/")
		uid := args[4]

		if token := d.client.Publish(fmt.Sprintf(DeviceDelayReplyTopic, productKey, deviceName, uid), 0, false, message.Payload()); token.Wait() && token.Error() != nil {
			d.sdkLogf(LogLevelError, "pub delay reply failed,clientId = %s,err = %s", formatMqttUserName(d.ProductKey, d.Name), token.Error())
		}
	}
}

func (d *Device) createServiceCallMqttHandler(productKey, deviceName string) func(client paho.Client, message paho.Message) {
	serviceCallHandler := func(client paho.Client, message paho.Message) {
		go func() {
			replyPayload := ReplyPayload{
				Code: SuccessCode,
				Data: make(map[string]interface{}, 0),
			}
			topicFields := strings.Split(message.Topic(), "/")
			moduleKey := topicFields[5]
			identifier := topicFields[6]
			uuid := topicFields[8]

			defer func() {
				payloadBytes, _ := json.Marshal(replyPayload)
				if token := d.client.Publish(fmt.Sprintf(ServiceCallReplyTopic, productKey, deviceName, moduleKey, identifier, uuid), 0, false, payloadBytes); token.Wait() && token.Error() != nil {
					d.sdkLogf(LogLevelError, "pub serviceCall reply failed,clientId = %s,reply = %s, err: %s", formatMqttUserName(d.ProductKey, d.Name), string(payloadBytes), token.Error())
				}
			}()

			payload := ServiceCallPayload{}
			if err := json.Unmarshal(message.Payload(), &payload); err != nil {
				replyPayload.Code = ErrorCodeParamUnmarshalFailed
				d.sdkLogf(LogLevelError, "unmarshal propertySet message payload failed,clientId = %s,message = %s,err: %s", formatMqttUserName(d.ProductKey, d.Name), message.Payload(), err)
				return
			}

			serviceCallMessage := ServiceCallMessage{
				DeviceIdentification: DeviceIdentification{
					ProductKey: productKey,
					DeviceName: deviceName,
				},
				ServiceCallParam: ServiceCallParam{
					ModuleKey:         moduleKey,
					ElementIdentifier: identifier,
					Params:            payload.Params,
				},
			}
			replyPayload.ID = payload.ID
			// 正常调用服务调用handler
			err := d.panicToError("createServiceCallMqttHandler", func() {
				replyPayload.Data, replyPayload.Code = d.serviceCallHandler(serviceCallMessage)
			})
			if err != nil {
				replyPayload.Code = ErrorCodeFuncExecPanic
			}
		}()
	}

	return serviceCallHandler
}

func (d *Device) createPropertySetMqttHandler(productKey, deviceName string) func(client paho.Client, message paho.Message) {
	return func(client paho.Client, message paho.Message) {
		go func() {
			replyParam := ReplyPayload{
				Code: SuccessCode,
				Data: make(map[string]interface{}, 0),
			}

			defer func() {
				payloadBytes, _ := json.Marshal(replyParam)
				if token := d.client.Publish(fmt.Sprintf(PropertySetReplyTopic, productKey, deviceName), 0, false, payloadBytes); token.Wait() && token.Error() != nil {
					d.sdkLogf(LogLevelError, "pub property set reply failed,clientId = %s,reply = %s, err: %s", formatMqttUserName(d.ProductKey, d.Name), string(payloadBytes), token.Error())
				}
			}()

			var payload ServiceCallPayload
			if err := json.Unmarshal(message.Payload(), &payload); err != nil {
				replyParam.Code = ErrorCodeParamUnmarshalFailed
				d.sdkLogf(LogLevelError, "unmarshal propertySet message payload failed,clientId = %s,message = %s, err: %s", formatMqttUserName(d.ProductKey, d.Name), message.Payload(), err)
				return
			}

			propertySetMessage := PropertySetMessage{
				DeviceIdentification: DeviceIdentification{
					ProductKey: productKey,
					DeviceName: deviceName,
				},
				Params: payload.Params,
			}
			replyParam.ID = payload.ID
			err := d.panicToError("createPropertySetMqttHandler", func() {
				replyParam.Code = d.propertySetHandler(propertySetMessage)
			})
			if err != nil {
				replyParam.Code = ErrorCodeFuncExecPanic
			}
		}()
	}
}

func (d *Device) createEventReportReplyMqttHandler(productKey, deviceName string) func(client paho.Client, message paho.Message) {
	return func(client paho.Client, message paho.Message) {
		go func() {
			reply := ReplyPayload{}
			err := json.Unmarshal(message.Payload(), &reply)
			if err != nil {
				d.sdkLogf(LogLevelError, "unmarshal eventReportReply message payload failed,clientId = %s,message = %s, err: %s", formatMqttUserName(d.ProductKey, d.Name), message.Payload(), err)
				return
			}
			d.panicToError("createEventReportReplyMqttHandler", func() {
				d.eventReportReplyHandler(ReplyParam{
					DeviceIdentification: DeviceIdentification{
						ProductKey: productKey,
						DeviceName: deviceName,
					},
					ReplyPayload: reply,
				})
			})
		}()
	}
}

func (d *Device) createPropertyReportReplyMqttHandler(productKey, deviceName string) func(client paho.Client, message paho.Message) {
	return func(client paho.Client, message paho.Message) {
		go func() {
			reply := ReplyPayload{}
			err := json.Unmarshal(message.Payload(), &reply)
			if err != nil {
				d.sdkLogf(LogLevelError, "unmarshal propertyReportReply message payload failed,clientId = %s,message = %s, err: %s", formatMqttUserName(d.ProductKey, d.Name), message.Payload(), err)
				return
			}
			_ = d.panicToError("createPropertyReportReplyMqttHandler", func() {
				d.propertyReportReplyHandler(ReplyParam{
					DeviceIdentification: DeviceIdentification{
						ProductKey: productKey,
						DeviceName: deviceName,
					},
					ReplyPayload: reply,
				})
			})
		}()
	}
}

func (d *Device) createOTAUpgradeNotifyMqttHandler(productKey, deviceName string) func(client paho.Client, message paho.Message) {
	otaUpgradeInformHandler := func(client paho.Client, message paho.Message) {
		go func() {
			payload := OTANotifyPayload{}
			split := strings.Split(message.Topic(), "/")
			otaJobID := split[5]
			err := json.Unmarshal(message.Payload(), &payload)
			if err != nil {
				d.sdkLogf(LogLevelError, "unmarshal upgrade inform message payload failed,clientId = %s,message = %s, err: %s", formatMqttUserName(d.ProductKey, d.Name), message.Payload(), err.Error())
				return
			}
			payload.ProductKey = productKey
			payload.DeviceName = deviceName
			payload.OTAJobID = otaJobID
			if payload.Data.Type == string(NotifyTypeUpgrade) {
				d.panicToError("createOTAUpgradeNotifyMqttHandler", func() {
					d.otaNotifyHandler(payload)
				})
				return
			}
			if payload.Data.Type == string(NotifyTypeCancel) {
				// 只有当取消ID和正在进行的ID相同时，向通道中发送消息
				if value, ok := d.otaCurrentJob.Load(formatOTAModuleKey(productKey, deviceName, payload.Data.Module)); ok {
					value.(OTACurrentJobInfo).JobCancelChannel <- struct{}{}
				}
			}
		}()
	}
	return otaUpgradeInformHandler
}

func (d *Device) createOTAUpgradeReplyMqttHandler(productKey, deviceName string) func(client paho.Client, message paho.Message) {
	otaUpgradeReplyHandler := func(client paho.Client, message paho.Message) {
		go func() {
			payload := OTAReplyPayload{}
			err := json.Unmarshal(message.Payload(), &payload)
			if err != nil {
				d.sdkLogf(LogLevelError, "unmarshal propertySet message payload failed,clientId = %s,message = %s, err: %s", formatMqttUserName(d.ProductKey, d.Name), message.Payload(), err.Error())
				return
			}
			if payload.Code != int(SuccessCode) {
				return
			}
			payload.ProductKey = productKey
			payload.DeviceName = deviceName
			if payload.Data.OTAJobID != "" {
				d.panicToError("createOTAUpgradeReplyMqttHandler", func() {
					d.otaReplyHandler(payload)
				})
			}
		}()
	}
	return otaUpgradeReplyHandler
}

func (d *Device) initLogConfig() bool {
	err := os.MkdirAll(path.Join(d.FilePath, "log"), 0777)
	if err != nil {
		d.sdkLogf(LogLevelError, "Log Directory Create Failed,err: %s", err)
		return false
	}
	// 启动协程，定时定量上传log
	go d.logReport()
	go d.logLocalStorage()
	return true
}

func (d *Device) dynamicRegister(authType AuthType) (string, error) {
	basicParam := DynamicRegisterBasicParam{
		InstanceID: d.InstanceID,
		ProductKey: d.ProductKey,
		DeviceName: d.Name,
		RandomNum:  rand.Int63(),
		Timestamp:  time.Now().Unix(),
		AuthType:   authType,
	}
	signature, err := hmacSHA256Encrypt(basicParam, d.ProductSecret)
	if err != nil {
		return "", fmt.Errorf("Encrypt failed,err: %s", err.Error())
	}
	param := DynamicRegisterParam{
		DynamicRegisterBasicParam: basicParam,
		Signature:                 signature,
	}

	v := url.Values{}
	v.Set(ActionKey, DynamicRegisterAction)
	v.Set(VersionKey, APIVerion)
	query := v.Encode()
	u := fmt.Sprintf("%s/2021-12-14/DynamicRegister", d.formatCloudHTTPUrl()) + "?" + query
	body, err := d.sendHTTPPostRequest(u, nil, param)
	if err != nil {
		d.sdkLogf(LogLevelError, "sendHTTPPostRequest failed, %s", err.Error())
		return "", err
	}
	var response DynamicRegisterHttpResponse
	err = json.Unmarshal(body, &response)
	if err != nil {
		return "", fmt.Errorf("unmarshal response body failed,err: %s", err.Error())
	}
	if response.ResponseMetadata.Error != nil {
		return "", errors.New(response.ResponseMetadata.Error.Message)
	}
	secret := getSecret(response.Result.Payload, d.ProductSecret)
	return secret, nil
}

func (d *Device) setDefaultHandler() {
	if d.propertySetHandler == nil {
		d.propertySetHandler = func(message PropertySetMessage) ErrorCode {
			messageStr, _ := json.Marshal(message)
			d.sdkLogf(LogLevelInfo, "propertySetHandler not set,receive message success!message = %v", string(messageStr))
			return ErrorCodeHandlerNotExist
		}
	}

	if d.serviceCallHandler == nil {
		d.serviceCallHandler = func(message ServiceCallMessage) (map[string]interface{}, ErrorCode) {
			messageStr, _ := json.Marshal(message)
			d.sdkLogf(LogLevelInfo, "serviceCallHandler not set,receive message success!message = %v", string(messageStr))
			return nil, ErrorCodeHandlerNotExist
		}
	}

	if d.shadowSetHandler == nil {
		d.shadowSetHandler = func(message ShadowSetMessage) ErrorCode {
			messageStr, _ := json.Marshal(message)
			d.sdkLogf(LogLevelInfo, "shadowSetHandler not set,receive message success!message = %v", string(messageStr))
			return ErrorCodeHandlerNotExist
		}
	}

	if d.otaNotifyHandler == nil {
		d.otaNotifyHandler = func(message OTANotifyPayload) {
			messageStr, _ := json.Marshal(message)
			d.sdkLogf(LogLevelInfo, "otaNotifyHandler not set,receive message success!message = %v", string(messageStr))
			code := d.OTAUpgradeRequest(message.Data.Module, "", message.DeviceIdentification)
			if code != SuccessCode {
				// 错误处理
				d.sdkLogf(LogLevelError, "upgrade request failed,code : %d", code)
			}
		}
	}

	if d.otaReplyHandler == nil {
		d.otaReplyHandler = func(message OTAReplyPayload) {
			messageStr, _ := json.Marshal(message)
			d.sdkLogf(LogLevelInfo, "otaReplyHandler not set,receive message success!message = %v", string(messageStr))
			code, otaMetadata := d.InitOTAJob(message.DeviceIdentification, message.Data)
			if code != SuccessCode {
				// 错误处理
				d.sdkLogf(LogLevelError, "no available OTA job was found for the OTA reply message,code:%d,otaMetadata:%v", code, otaMetadata)
				return
			}
			code = d.OTADownload(otaMetadata.OTAModuleName)
			if code != SuccessCode {
				// 错误处理
				d.sdkLogf(LogLevelError, "OTA download failed,code:%d,otaMetadata:%v", code, otaMetadata)
				return
			}

			code = d.OTAInstall(otaMetadata.OTAModuleName)
			if code != SuccessCode {
				// 错误处理
				d.sdkLogf(LogLevelError, "OTA install failed,code:%d,otaMetadata:%v", code, otaMetadata)
			}
		}
	}

	if d.eventReportReplyHandler == nil {
		d.eventReportReplyHandler = func(message ReplyParam) {
			messageStr, _ := json.Marshal(message)
			d.sdkLogf(LogLevelInfo, "eventReportReplyHandler not set,receive message success!message = %v", string(messageStr))
		}
	}

	if d.propertyReportReplyHandler == nil {
		d.propertyReportReplyHandler = func(message ReplyParam) {
			messageStr, _ := json.Marshal(message)
			d.sdkLogf(LogLevelInfo, "propertyReportReplyHandler not set,receive message success!message = %v", string(messageStr))
		}
	}

	if d.customJobHandler == nil {
		d.customJobHandler = func(message CustomJobMessage) (ErrorCode, string) {
			messageStr, _ := json.Marshal(message)
			d.sdkLogf(LogLevelInfo, "customJobHandler not set,receive message success!message = %v", string(messageStr))
			return ErrorCodeHandlerNotExist, "handler not exist"
		}
	}
}
