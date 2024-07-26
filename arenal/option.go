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

import "time"

type DeviceOption struct {
	// 接入方式 MQTT/HTTP 默认MQTT
	// Required:false
	AccessType AccessType `yaml:"access_type"`
	// 实例ID
	// Required: true
	InstanceID string `yaml:"instance_id" json:"instance_id"`
	// 设备名称
	// Required:true
	Name string
	// 设备密钥
	// Required:false 无Secret时必须修改设备认证方式，动态获取secret
	Secret string
	// 产品标示
	// Required:true
	ProductKey string
	// 产品密钥
	// Required:false 设备Secret未获取到时，必须填写
	ProductSecret string
	// MQTT接入域名
	// Required: true
	MQTTHost string
	// MQTT接入端口，默认1883
	// Required: true
	MQTTPort int
	// 云端API地址 todo 设备侧不与云端API交互
	// Required: true
	CloudHTTPHost string
	// 是否启动tls认证，默认为false
	// Required:false
	EnableTls bool
	// 设备认证方式，默认为 DeviceSecret
	// Required:true
	VerifyMode DeviceVerifyMode
	// 是否是网关设备
	// Required:true
	IsGateway bool
	// OTA历史任务重入模式，不重入/新任务抢占/历史任务优先，默认为不重入
	// Required:false
	OTAHistoryJobReloadMode OTAHistoryJobReloadMode
	// 默认的文件存放路径，包括安装包下载文件，日志文件以及OTA升级历史任务文件等，不填则默认为当前工作目录
	// Required:false
	FilePath string
	// 设备OTA模块配置
	// Required:false 不填写无法进行OTA升级，key为设备OTAModuleKey
	ModuleConfig map[string]ModuleInfo
	// 子设备OTA模块配置
	// Required:false 不填写子设备无法进行OTA升级，key为子设备ProductKey:DeviceName:OTAModuleKey
	SubDeviceModuleConfig map[string]ModuleInfo
	// topic qos配置
	// Required:false key为topic，value为qos等级
	UserDefineTopicQosMap map[string]byte
	// Webshell执行器，默认为sh，如果系统不支持sh，请配置合适的执行器，比如bash,zsh 等
	// Required:false
	WebshellExecutor string
	// log report 配置
	// Required: false 默认配置为关闭，最低为info
	LogReportConfig LogReportConfig
	// 是否允许多个设备同时在线（临时方案）
	// Required:false
	AllowMultiple bool
	// mqtt连接保活时间，单位s，超过keepalive*1.5时间未收到消息认为客户端掉线。默认3分钟
	// Required:false
	KeepAlive *int64
	// 尝试重连的最大间隔时间，单位s。默认10秒
	// Required:false
	MaxReconnectInterval *int64
	// 是否开启mqtt debug日志，定位问题使用，不要在生产环境中默认打开
	// Required: false
	MQTTDebugLogEnabled bool
	// mqtt debug日志配置
	// Required: false
	MQTTDebugLogConfig MQTTDebugLogConfig
	// 断点重传property时的时间间隔，默认1s
	// Required:false
	UploadHistoryPropertyIntv *time.Duration
	// 网关定期上报在线子设备的时间间隔，默认5min
	// Required:false
	ReportSubDeviceLoginIntv *time.Duration
}

type AccessType string

const (
	DeviceAccessTypeHTTP AccessType = "http"
	DeviceAccessTypeMQTT AccessType = "mqtt"
)

type LogType string

const (
	MQTTLogTypeConsole LogType = "console"
	MQTTLogTypeFile    LogType = "file"
)

func NewDeviceOption() *DeviceOption {
	option := &DeviceOption{
		InstanceID:              "please config your instanceID",
		Name:                    "please config deviceName",
		Secret:                  "please config deviceSecret",
		ProductKey:              "please config ProductKey",
		ProductSecret:           "please config ProductSecret",
		EnableTls:               false,
		VerifyMode:              VerifyModeDeviceSecret,
		IsGateway:               false,
		OTAHistoryJobReloadMode: OTAReloadModeIgnore,
		FilePath:                "./",
		ModuleConfig:            make(map[string]ModuleInfo),
		SubDeviceModuleConfig:   make(map[string]ModuleInfo),
		UserDefineTopicQosMap:   make(map[string]byte),
		WebshellExecutor:        "sh",
		LogReportConfig: LogReportConfig{
			LogReportCommonConfig: LogReportCommonConfig{
				Switch: false,
			},
		},
		AllowMultiple: false,
	}
	return option
}

func (o *DeviceOption) WithInstanceID(instanceID string) *DeviceOption {
	o.InstanceID = instanceID
	return o
}

func (o *DeviceOption) WithDeviceInfo(productKey, productSecret, deviceName, deviceSecret string, verifyMode DeviceVerifyMode) *DeviceOption {
	o.ProductKey = productKey
	o.ProductSecret = productSecret
	o.Name = deviceName
	o.Secret = deviceSecret
	o.VerifyMode = verifyMode
	return o
}

func (o *DeviceOption) WithCloudHTTPEndpoint(endpoint string) *DeviceOption {
	o.CloudHTTPHost = endpoint
	return o
}

func (o *DeviceOption) WithMQTTEndpoint(endpoint string) *DeviceOption {
	o.MQTTHost = endpoint
	return o
}
func (o *DeviceOption) WithMQTTPort(port int) *DeviceOption {
	o.MQTTPort = port
	return o
}

func (o *DeviceOption) WithEnableTls(enableTls bool) *DeviceOption {
	o.EnableTls = enableTls
	return o
}

func (o *DeviceOption) WithOTAHistoryJobReloadMode(mode OTAHistoryJobReloadMode) *DeviceOption {
	o.OTAHistoryJobReloadMode = mode
	return o
}

func (o *DeviceOption) WithIsGateway(isGateway bool) *DeviceOption {
	o.IsGateway = isGateway
	return o
}

func (o *DeviceOption) WithFilePath(path string) *DeviceOption {
	o.FilePath = path
	return o
}

func (o *DeviceOption) WithModuleConfig(moduleConfig map[string]ModuleInfo) *DeviceOption {
	o.ModuleConfig = moduleConfig
	return o
}

func (o *DeviceOption) WithSubDeviceModuleConfig(subDeviceModuleConfig map[string]ModuleInfo) *DeviceOption {
	o.SubDeviceModuleConfig = subDeviceModuleConfig
	return o
}

func (o *DeviceOption) WithUserDefineTopicQosMap(userDefineTopicQosMap map[string]byte) *DeviceOption {
	o.UserDefineTopicQosMap = userDefineTopicQosMap
	return o
}

func (o *DeviceOption) WithWebshellExecutor(webshellExecutor string) *DeviceOption {
	o.WebshellExecutor = webshellExecutor
	return o
}

func (o *DeviceOption) WithLogReportConfig(logReportConfig LogReportConfig) *DeviceOption {
	o.LogReportConfig = logReportConfig
	return o
}

func (o *DeviceOption) WithAllowMultiple(allowMultiple bool) *DeviceOption {
	o.AllowMultiple = allowMultiple
	return o
}

func (o *DeviceOption) WithKeepAlive(keepAlive int64) *DeviceOption {
	o.KeepAlive = &keepAlive
	return o
}

func (o *DeviceOption) WithMaxReconnectInterval(maxReconnectInterval int64) *DeviceOption {
	o.MaxReconnectInterval = &maxReconnectInterval
	return o
}

func (o *DeviceOption) WithMQTTDebugLogEnabled(enabled bool) *DeviceOption {
	o.MQTTDebugLogEnabled = enabled
	return o
}

func (o *DeviceOption) WithMQTTDebugLogConfig(mqttDebugLogConfig MQTTDebugLogConfig) *DeviceOption {
	mqttDebugLogConfig.setDefaults()
	o.MQTTDebugLogConfig = mqttDebugLogConfig
	return o
}

func (o *DeviceOption) WithUploadHistoryPropertyIntv(uploadHistoryPropertyIntv time.Duration) *DeviceOption {
	o.UploadHistoryPropertyIntv = &uploadHistoryPropertyIntv
	return o
}

func (o *DeviceOption) WithReportSubDeviceLoginIntv(reportSubDeviceLoginIntv time.Duration) *DeviceOption {
	o.ReportSubDeviceLoginIntv = &reportSubDeviceLoginIntv
	return o
}

type DeviceVerifyMode string

// 一机一密，需要提供ProductKey、Name、Secret
const VerifyModeDeviceSecret DeviceVerifyMode = "DeviceSecret"

// 一型一密预注册，需要提供ProductKey、ProductSecret、Name
const VerifyModeDynamicPreRegistered DeviceVerifyMode = "DynamicRegistered"

// 一型一密免预注册，需要提供ProductKey、ProductSecret、Name
const VerifyModeDynamicNoPreRegistered DeviceVerifyMode = "DynamicUnRegistered"

const ServicePropertySet string = "propertySet"

type OTAHistoryJobReloadMode string

const (
	OTAReloadModeIgnore             OTAHistoryJobReloadMode = "Ignore"
	OTAReloadModeNewJobCoverage     OTAHistoryJobReloadMode = "NewJobCoverage"
	OTAReloadModeHistoryJobPriority OTAHistoryJobReloadMode = "HistoryJobPriority"
)

type AuthType int

const (
	AuthTypePreRegistration AuthType = iota
	AuthTypeNoPreRegistration
)
