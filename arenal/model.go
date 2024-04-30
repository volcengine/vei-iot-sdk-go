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
	"context"
	paho "github.com/eclipse/paho.mqtt.golang"
	"go.uber.org/atomic"
	"io"
	"net/http"
	"sync"
	"time"
)

const (
	otaJobHistoryPath         = "ota_job_history.json"
	md5CodeStorageFile        = "check.txt"
	OTADefaultModule          = "default"
	tokenHeader               = "arenal-token"
	tmpSuffix                 = "tmp"
	executingJobPath          = "executing_job.json"
	executingJobMD5File       = "executing_job.md5"
	ServiceIDKey              = "ServiceID"
	md5Key                    = "MD5"
	fileSourceKey             = "Source"
	defaultKeepAlive          = 180
	subscriptionRetryIntv     = time.Second
	subscriptionMaxRetryCount = 100
)

var ConfigFilePath = "config.yaml"

const (
	DefaultMQTTPort = 1883
)

const (
	emqxCA = `-----BEGIN CERTIFICATE-----
MIID7TCCAtWgAwIBAgIEY6qvdTANBgkqhkiG9w0BAQsFADBUMQswCQYDVQQGEwJD
TjERMA8GA1UECBMIWmhlamlhbmcxETAPBgNVBAcTCEhhbmd6aG91MQ0wCwYDVQQK
EwRFTVFYMRAwDgYDVQQDEwdSb290IENBMCAXDTIyMTIyNzA4NDAyMVoYDzIxMjIx
MjI3MDg0MDIxWjBfMQswCQYDVQQGEwJDTjERMA8GA1UECBMIWmhlamlhbmcxETAP
BgNVBAcTCEhhbmd6aG91MQ0wCwYDVQQKEwRFTVFYMRswGQYDVQQDExJTZXJ2ZXIg
Y2VydGlmaWNhdGUwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQDP/t64
uarS5CEdCZoQYfmVKWnO1SHbfxvw82aKNjh1sHoQAA5PRwNaLUNsw8ZW1du1O9H1
pD0dJSKWQEuHWrFcvkqiQ9m0zgx34+D9JYIFXZs0QWzx8SqK+kXlAMpD7xdYDO6M
nLsLRk1cHSZYfsceCxY5gNU5IqKjjr09Hj5FecZUWGPrZYdybyv6zxF/PWbTl1WG
jDFV67Hd1v5+vxepWOpGY+7LvYGNiWZ2eBbddATxEn2fOxzSpiyT2ItkuLvBOeES
ASWRMVSF5E2KQbLBs4OGGyY5Lv82sbodr5JlwGhhoSBfXpdik6ysc3rXyLxCLSeS
Nkv26uZ4f/Qqnr7TAgMBAAGjgbkwgbYwDgYDVR0PAQH/BAQDAgWgMBMGA1UdJQQM
MAoGCCsGAQUFBwMBMAwGA1UdEwEB/wQCMAAwHwYDVR0jBBgwFoAURD1nlV5jHsEA
oSuXsQk+DxqWIrowYAYDVR0RBFkwV4IFKi5pb3SCHCouY24tc2hhbmdoYWkuaW90
LnZvbGNlcy5jb22CHiouY24tZ3VpbGluLWJvZS5pb3Qudm9sY2VzLmNvbYIQKi5p
b3Qudm9sY2VzLmNvbTANBgkqhkiG9w0BAQsFAAOCAQEABocjnt/0CjUUWh3nJLR/
V2AcCAWDPSs3btJ6bRcZW1OE4UTQ5Bm/2DpHlKTXbkqgOOHyDHS+Vm36vl9drRWT
XTwYeOUgLxTOnoZEdhjuxgGk9gFK1ujGATIQRoQ/SfefPyQ6VUGpowj/xxLUe3tb
ON5QbuHTwXa2UjBzZhXA9CiI9fiF/KBx8aPC19HxGJhEgtPHFHFsTXLXFKYV7C84
+z+V+ffUrl94Vm8rEFnzQDpNrMo+/8P1IQMztI3u/YvrLdxoNueNS1TuxsLFc1yl
rPg2MJ788MGf9PoCzy+IlEHE76pLIrP20rVo5bePubDTZk4B+GToDHJ6YGpQfZdl
Ow==
-----END CERTIFICATE-----`
)

const (
	InstanceIDFieldKey = "InstanceID"
)

const (
	IOTHTTPAccessHeaderKey = "X-IoT-AccessToken"
)

const (
	ActionKey  = "Action"
	VersionKey = "Version"

	UploadFileAction               = "UploadFile"
	DynamicRegisterAction          = "DynamicRegister"
	GetDeviceTokenAction           = "GetDeviceToken"
	DeviceMessagePublishHTTPAction = "DeviceMessagePublishHTTP"
)

const APIVerion = "2021-12-14"

type DynamicRegisterParam struct {
	DynamicRegisterBasicParam
	// 签名
	// Required: true
	Signature string `json:"signature" `
}

type DynamicRegisterHttpResponse struct {
	Result           DynamicRegisterResponse `json:"Result"`
	ResponseMetadata ResponseMetadata        `json:"ResponseMetadata"`
}

type ResponseMetadata struct {
	Error *ErrorObj `json:",omitempty"`
}

type ErrorObj struct {
	CodeN   int    `json:",omitempty"`
	Code    string `json:",omitempty"`
	Message string `json:",omitempty"`
}

type DynamicRegisterResponse struct {
	// 有效字节数
	// Required: true
	Len int `json:"len" `
	// 加密后的数据
	// Required: true
	Payload string `json:"payload" `
}

type DynamicRegisterBasicParam struct {
	// 实例ID
	// Required: true
	InstanceID string `json:"InstanceID"`
	// 产品标识
	// Required: true
	ProductKey string `json:"product_key" `
	// 设备名称
	// Required: true
	DeviceName string `json:"device_name" `
	// 随机数
	// Required: true
	RandomNum int64 `json:"random_num" `
	// 时间戳
	// Required: true
	Timestamp int64 `json:"timestamp" `
	// 认证类型，0-预认证；1-免认证
	// swagger:model
	// Required: true
	AuthType AuthType `json:"auth_type" `
}

// internal communicate with arenal
type GetDeviceTokenParam struct {
	DeviceIdentification
	ClientID   string `json:"ClientID"`
	Timestamp  int64  `json:"Timestamp"`
	Sign       string `json:"Sign"`
	InstanceID string `json:"InstanceID"`
}

type GetDeviceTokenResult struct {
	Token string `json:"Token"`
}

type GetDeviceTokenResponse struct {
	Result           GetDeviceTokenResult `json:"Result"`
	ResponseMetadata ResponseMetadata     `json:"ResponseMetadata"`
}

type PublishMessageParam struct {
	AccessToken string `json:"AccessToken,omitempty"`
	Topic       string `json:"Topic"`
	Payload     []byte `json:"Payload"`
}

type PublishMessageHTTPParam struct {
	AccessToken string `json:"AccessToken,omitempty"` // header
	Topic       string `json:"Topic"`                 // body
	Payload     []byte `json:"Payload"`               // body
	InstanceID  string `json:"InstanceID"`            // body
	ProductKey  string `json:"ProductKey,omitempty"`  // path
	DeviceName  string `json:"DeviceName,omitempty"`  // path
}

type PublishMessageHTTPResult struct {
}

type PublishMessageHTTPResponse struct {
	Result           PublishMessageHTTPResult `json:"Result"`
	ResponseMetadata ResponseMetadata         `json:"ResponseMetadata"`
}

type Device struct {
	DeviceOption
	ctx                        context.Context
	cancel                     context.CancelFunc
	client                     paho.Client
	initSuccess                bool
	firstConnect               atomic.Bool
	initLock                   sync.Mutex
	propertySetHandler         PropertySetHandler
	serviceCallHandler         ServiceCallHandler
	shadowSetHandler           ShadowSetHandler
	eventReportReplyHandler    EventReportReplyHandler
	propertyReportReplyHandler PropertyReportReplyHandler
	streamLogCancelFuncMap     sync.Map
	customTopicHandlerMap      map[string]CustomTopicHandler
	ntpResponseHandler         NTPResponseHandler
	onConnectHandler           OnConnectHandler
	authType                   AuthType
	httpClient                 *http.Client
	uploadPropertyFileLock     sync.Mutex
	/***********************    OTA相关     *************************/
	otaNotifyHandler                OTANotifyHandler
	otaReplyHandler                 OTAReplyHandler
	diffAlgorithm                   DiffAlgorithm
	installAlgorithm                InstallAlgorithm
	subDeviceCommonInstallAlgorithm SubDeviceCommonInstallAlgorithm
	subDeviceCustomInstallAlgorithm map[string]SubDeviceCustomInstallAlgorithm
	otaHistoryJob                   sync.Map // key为ProductKey|DeviceName|OTAModuleName
	otaCurrentJob                   sync.Map // key为ProductKey|DeviceName|OTAModuleName
	queryUpgradeConfig              *QueryUpgradeConfig
	/***********************    网关相关     *************************/
	subDeviceSecretMap             sync.Map
	subDeviceLoginMap              map[string]struct{}
	subDeviceLoginMapMtx           sync.RWMutex
	addTopologiesReplyHandler      GatewayCommonHandler
	deleteTopologiesReplyHandler   GatewayCommonHandler
	getTopologiesReplyHandler      GatewayCommonHandler
	subDeviceLoginReplyHandler     GatewayCommonHandler
	subDeviceLogoutReplyHandler    GatewayCommonHandler
	notifyAddSubDeviceHandler      GatewayCommonHandler
	topologyChangeHandler          TopologyChangeHandler
	subDeviceChangeHandler         SubDeviceChangeHandler
	subDeviceSecretGetReplyHandler SubDeviceSecretGetReplyHandler
	/***********************    任务相关     *************************/
	customJobHandler CustomJobHandler
	executingJobMap  sync.Map // key为ProductKey|DeviceName|JobID
}

type OTACurrentJobInfo struct {
	Info             JobInfo
	JobCancelChannel chan struct{}
}

type DeviceConfig struct {
	InstanceID              string                `yaml:"instance_id" json:"instance_id"`
	DeviceName              string                `yaml:"device_name" json:"device_name"`
	ProductKey              string                `yaml:"product_key" json:"product_key"`
	ProductSecret           string                `yaml:"product_secret" json:"product_secret"`
	DeviceSecret            string                `yaml:"device_secret" json:"device_secret"`
	VerifyMode              string                `yaml:"verify_mode" json:"verify_mode"`
	IsGateway               bool                  `yaml:"is_gateway" json:"is_gateway"`
	CloudHTTPHost           string                `yaml:"cloud_http_host" json:"cloud_http_host"`
	MQTTHost                string                `yaml:"mqtt_host" json:"mqtt_host"`
	MQTTPort                int                   `yaml:"mqtt_port" json:"mqtt_port"`
	Region                  string                `yaml:"region" json:"region"`
	EnableTls               bool                  `yaml:"enable_tls" json:"enable_tls"`
	FilePath                string                `yaml:"file_path" json:"file_path"`
	OTAHistoryJobReloadMode string                `yaml:"ota_history_job_reload_mode" json:"ota_history_job_reload_mode"`
	OTAModule               []ModuleInfo          `yaml:"ota_module" json:"ota_module"`
	SubDeviceOTAModule      []ModuleInfo          `yaml:"sub_device_ota_module" json:"sub_device_ota_module"`
	TopicConfig             UserDefineTopicConfig `yaml:"user_define_topic" json:"user_define_topic"`
	LogReportConfig         LogReportCommonConfig `yaml:"log_report_config" json:"log_report_config"`
	WebshellExecutor        string                `yaml:"webshell_executor" json:"webshell_executor"`
	KeepAlive               *int64                `yaml:"keep_alive" json:"keep_alive"`
}

type LogReportConfigPayload struct {
	ID      string                `json:"id"`
	Version string                `json:"version"`
	Data    LogReportCommonConfig `json:"data"`
}

type LogReportCommonConfig struct {
	Switch      bool   `json:"Switch" yaml:"switch"`
	LowestLevel string `json:"LowestLevel" yaml:"lowest_level"`
}

type StreamLogConfig struct {
	// 是否开启实时流日志上报
	Switch bool `json:"Switch"`
	// 内容关键字
	// Required: false
	ContentKeyWord string `json:"ContentKeyword"`
	// 日志类型(sdk、device)
	// Required: false
	Type string `json:"Type"`
	// 日志等级（debug、info、warn、error、fatal）
	// Required: false
	LogLevel string `json:"LogLevel"`
	// 开始时间
	// Required: true
	StartTime int64 `json:"StartTime"`
}

type StreamLogConfigPayload struct {
	ID              string          `json:"ID"`
	Version         string          `json:"Version"`
	StreamLogConfig StreamLogConfig `json:"Data"`
}

type fileOp uint32

const (
	fileOpCreate fileOp = 1 << iota
	fileOpWrite
)

type CommonLogCondition struct {
	// 内容关键字
	// Required: false
	ContentKeyWord string `json:"ContentKeyword"`
	// 日志类型(sdk、device)
	// Required: false
	Type string `json:"Type"`
	// 日志等级（debug、info、warn、error、fatal）
	// Required: false
	LogLevel string `json:"LogLevel"`
	// 开始时间
	// Required: false
	StartTime int64 `json:"StartTime"`
	// 结束时间
	// Required: false
	EndTime int64 `json:"EndTime"`
}

type LocalLogCondition struct {
	CommonLogCondition
	// 日志偏移量，表明从这个偏移量开始读取。默认为0
	// Required: false
	Offset int64 `json:"Offset"`
	// 每次读取的日志数。默认为40
	// Required: false
	Count int `json:"Count"`
}

type LocalLogConfigPayload struct {
	ID                string            `json:"ID"`
	Version           string            `json:"Version"`
	LocalLogCondition LocalLogCondition `json:"Data"`
}

type LocalLogResult struct {
	// 设备日志列表
	// swagger:model
	// Required: true
	Logs []Log `json:"List"`
	// 日志偏移量，表明下次从这个偏移量开始读取。如果为0说明查询结束
	// Required: true
	Offset int64 `form:"Offset"`
}

type LocalLogReportPayload struct {
	ID      string         `json:"Id"`
	Version string         `json:"Version"`
	Data    LocalLogResult `json:"Data"`
}

type UserDefineTopicConfig struct {
	Qos0 []string `yaml:"qos_0" json:"qos_0"`
	Qos1 []string `yaml:"qos_1" json:"qos_1"`
}

type ModuleInfo struct {
	ModuleKey    string `yaml:"module_key" json:"module_key"`
	Version      string `yaml:"version" json:"version"`
	FileName     string `yaml:"file_name" json:"file_name"`
	LastVersion  string `yaml:"last_version" json:"last_version,omitempty"`
	LastFileName string `yaml:"last_file_name" json:"last_file_name,omitempty"`
}

type JobInfo struct {
	DeviceIdentification
	OTAJobID           string    `json:"ota_job_id"`
	InstallPackageFile string    `json:"install_package_file"` // 用于安装的安装包文件，整包则为下载的升级包文件，差分包则为差分还原后的完整升级包文件
	OTAPackageFile     string    `json:"ota_package_file"`     // 下载的升级包文件
	DurationInMinutes  int64     `json:"duration_in_minutes"`  // 升级持续时长，用于计算升级是否超时，从下载升级包开始算起，对于重试的升级任务则从重试时间开始算起
	Step               string    `json:"step"`                 // 断点保存，用于历史任务中断恢复
	StepErrorCode      ErrorCode `json:"step_error_code"`      // 当前阶段的状态
	RetryTime          int64     `json:"retry_time"`           // 重试次数，若为0，表示该任务当前是第一次执行，重试超过5次将从历史任务中删除
	OTAReplyData
}

type PropertyPayload struct {
	ID      string                   `json:"ID"`
	Version string                   `json:"Version"`
	Params  map[string]PropertyValue `json:"Params"`
}

type PropertyValue struct {
	// 属性具体值
	Value interface{} `json:"Value"`
	// 属性值获取时间
	TimeMill int64 `json:"Time"`
}

type PropertySetHandler func(message PropertySetMessage) ErrorCode

// DeviceIdentification 设备标识
type DeviceIdentification struct {
	// 设备产品标识
	// Required: true
	ProductKey string `json:"ProductKey"`
	// 设备名称
	// Required: true
	DeviceName string `json:"DeviceName"`
}

type PropertySetMessage struct {
	DeviceIdentification
	Params map[string]interface{} `json:"Params"`
}

type ServiceCallHandler func(message ServiceCallMessage) (map[string]interface{}, ErrorCode)

type ServiceCallMessage struct {
	DeviceIdentification
	ServiceCallParam
}

type ServiceCallPayload struct {
	ID      string                 `json:"ID"`
	Version string                 `json:"Version"`
	Params  map[string]interface{} `json:"Params"`
}

type ServiceCallParam struct {
	// 模块标示
	// Required: true
	ModuleKey string `json:"ModuleKey"`
	// 功能标示ID
	// Required: true
	ElementIdentifier string `json:"ElementIdentifier"`
	// 输入参数
	// Required: false
	Params map[string]interface{} `json:"Params"`
}

type ReplyParam struct {
	DeviceIdentification
	ReplyPayload
}

type ReplyPayload struct {
	ID   string                 `json:"ID"`
	Code ErrorCode              `json:"Code"`
	Data map[string]interface{} `json:"Data"`
}

type ShadowSetHandler func(message ShadowSetMessage) ErrorCode

type ShadowSetMessage struct {
	DeviceIdentification
	ShadowDesiredParam
}

type ShadowDesiredSetPayload struct {
	CommonRequest
	Params ShadowDesiredParam `json:"Params"`
}

type ShadowGetReplyPayload struct {
	CommonReply
	Data ShadowDesiredParam `json:"Data"`
}

type ShadowDesiredParam struct {
	Desired map[string]interface{} `json:"Desired"`
	Version int64                  `json:"Version"`
}

type ShadowReportPayload struct {
	CommonRequest
	Params ShadowReportParam `json:"Params"`
}

type ShadowReportParam struct {
	// 设备上报的版本
	Version int64 `json:"Version"`
	// 上报的各个属性
	Reported map[string]interface{} `json:"Reported"`
}

type ClearPayload struct {
	CommonRequest
	ShadowVersion int64 `json:"Params"`
}

type Reported struct {
	// 设备影子内容，key为 FormatUniqueFieldName 构建的唯一key，value为对应属性的值
	Reported map[string]interface{} `json:"reported"`
}

type EventReportReplyHandler func(message ReplyParam)

type PropertyReportReplyHandler func(message ReplyParam)

type EventPayload struct {
	ID      string     `json:"ID"`
	Version string     `json:"Version"`
	Params  EventParam `json:"Params"`
}

type EventParam struct {
	Values   map[string]interface{} `json:"Value"`
	TimeMill int64                  `json:"Time"`
}

type OTANotifyHandler func(message OTANotifyPayload)

type OTAReplyHandler func(message OTAReplyPayload)

type OTAVersionReportPayload struct {
	ID      string            `json:"id"`
	Version string            `json:"version"`
	Params  map[string]string `json:"params"`
}

type OTANotifyPayload struct {
	DeviceIdentification
	OTAJobID string     `json:"ota_job_id"`
	ID       string     `json:"id"`
	Code     int        `json:"code"`
	Data     NotifyData `json:"data"`
}

type NotifyType string

const (
	NotifyTypeUpgrade NotifyType = "Upgrade"
	NotifyTypeCancel  NotifyType = "Cancel"
)

type NotifyData struct {
	Type          string `json:"type"`
	DestVersion   string `json:"dest_version"`
	Module        string `json:"module"`
	CustomMessage string `json:"custom_message"`
}

type OTAUpgradeRequestPayload struct {
	ID      string              `json:"id"`
	Version string              `json:"version"`
	Params  UpgradeRequestParam `json:"params"`
}

type UpgradeRequestParam struct {
	OTAJobID   string `json:"ota_job_id,omitempty"`
	SrcVersion string `json:"src_version"`
	Module     string `json:"module"`
}

type OTAReplyPayload struct {
	DeviceIdentification
	ID   string       `json:"id"`
	Code int          `json:"code"`
	Data OTAReplyData `json:"data"`
}

type OTAReplyData struct {
	// OTA任务ID
	OTAJobID string `json:"ota_job_id"`
	// 安装包大小
	Size int64 `json:"size,omitempty"`
	// 目标版本
	DestVersion string `json:"dest_version,omitempty"`
	// 模块标识
	Module string `json:"module,omitempty"`
	// 是否为差分包
	IsDiff bool `json:"is_diff,omitempty"`
	// 下载地址url
	Url string `json:"url,omitempty"`
	// 签名值，用于校验完整性
	Sign string `json:"sign,omitempty"`
	// 单设备超时时间（单位：分钟）
	TimeoutInMinutes int64 `json:"timeout_in_minutes,omitempty"`
}

// OTAModuleIdentification 设备OTA模块标识
type OTAModuleIdentification struct {
	// 设备产品标识
	ProductKey string
	// 设备名称
	DeviceName string
	// OTA模块
	ModuleName string
}

type OTAUpgradeProcessReportPayload struct {
	ID      string             `json:"id"`
	Version string             `json:"version"`
	Params  ProcessReportParam `json:"params"`
}

type ProcessReportParam struct {
	// 目前状态，下载中/下载完成/安装中/安装完成/升级成功/升级失败
	Status UpgradeDeviceStatusEnum `json:"status"`
	// 结果码 OTAProcessReportResultCode
	ResultCode ErrorCode `json:"result_code"`
	// 结果说明
	ResultDesc string `json:"result_desc,omitempty"`
	// 时间戳，毫秒
	Time int64 `json:"time"`
}

type UpgradeDeviceStatusEnum string

const (
	UpgradeDeviceStatusDownloading UpgradeDeviceStatusEnum = "Downloading" // 下载中
	UpgradeDeviceStatusDownloaded  UpgradeDeviceStatusEnum = "Downloaded"  // 下载完成
	UpgradeDeviceStatusInstalling  UpgradeDeviceStatusEnum = "Installing"  // 安装中
	UpgradeDeviceStatusInstalled   UpgradeDeviceStatusEnum = "Installed"   // 安装完成
	UpgradeDeviceStatusSuccess     UpgradeDeviceStatusEnum = "Success"     // 升级成功
	UpgradeDeviceStatusFailed      UpgradeDeviceStatusEnum = "Failed"      // 升级失败
)

type UpgradeStepEnum string

const (
	UpgradeStepEnumToUpgrade      UpgradeStepEnum = "ToUpgrade"      // 待升级
	UpgradeStepEnumDownloading    UpgradeStepEnum = "Downloading"    // 下载中
	UpgradeStepEnumDownloaded     UpgradeStepEnum = "Downloaded"     // 下载完成
	UpgradeStepEnumDiffRecovering UpgradeStepEnum = "DiffRecovering" // 差分还原中
	UpgradeStepEnumDiffRecovered  UpgradeStepEnum = "DiffRecovered"  // 差分还原完成
	UpgradeStepEnumInstalling     UpgradeStepEnum = "Installing"     // 安装中
	UpgradeStepEnumInstalled      UpgradeStepEnum = "Installed"      // 安装完成
	UpgradeStepEnumSuccess        UpgradeStepEnum = "Success"        // 升级成功
)

// DiffAlgorithm 差分还原算法方法定义
// 传递 diffFileName 差分包源文件
// 返回 差分还原结果
type DiffAlgorithm func(diffFileName string) DiffAlgorithmResult

type DiffAlgorithmResult struct {
	// 差分还原结果
	IsSuccess bool
	// 结果描述信息
	ResDesc string
	// 完整安装包路径
	TargetFilePath string
}

type DiffFilePathConfig struct {
	// 差分包所在路径
	DiffFilePath string
}

// InstallAlgorithm 安装算法方法定义
// 传递 module:模块标识符，installFile:安装包
// 返回 安装结果
type InstallAlgorithm func(module, installFile string) InstallAlgorithmResult

type InstallAlgorithmResult struct {
	// 安装结果
	IsSuccess bool
	// 结果描述信息
	ResDesc string
	// 安装后版本号
	DeviceVersion string
	// 安装完成后是否需要重启，子设备升级时可不填
	NeedReboot bool
}

// SubDeviceCommonInstallAlgorithm 子设备通用安装算法方法定义
// 传递 SubDeviceInstallParam
// 返回 子设备安装结果
type SubDeviceCommonInstallAlgorithm func(SubDeviceCommonInstallParam SubDeviceInstallParam) InstallAlgorithmResult

type SubDeviceInstallParam struct {
	// 子设备所属产品标识
	ProductKey string
	// 子设备名称
	DeviceName string
	// OTA模块标识符
	OTAModule string
	// 安装包路径
	FileName string
	// 是否是差分包
	IsDiff bool
}

// SubDeviceCustomInstallAlgorithm 子设备定制化安装算法方法定义
// 传递 SubDeviceCustomInstallParam
// 返回 子设备安装结果
type SubDeviceCustomInstallAlgorithm func(SubDeviceCustomInstallParam SubDeviceInstallParam) InstallAlgorithmResult

// QueryUpgradeConfig 查询OTA升级频次配置
type QueryUpgradeConfig struct {
	// 间隔时间（单位：小时），默认为24小时
	TimeInterval int `json:"time_interval"`
	// 时间段起始时间（单位：小时），默认为每天0点
	StartTime int `json:"start_time"`
	// 时间段结束时间（单位：小时），默认为每天24点
	EndTime int `json:"end_time"`
}

type OTAMetadata struct {
	// 升级任务唯一标识
	OTAJobID string
	// 产品唯一标识
	ProductKey string
	// 设备名称
	DeviceName string
	// OTA模块名称
	OTAModuleName string
	// 重试次数
	RetryTime int64
	// 历史任务断点
	Step string
}

type Log struct {
	CreateTime int64  `json:"CreateTime"`
	LogLevel   string `json:"LogLevel"`
	Content    string `json:"Content"`
	Type       string `json:"Type"`
}

type LogReportConfig struct {
	LogReportCommonConfig
	logSendChannel     chan Log
	logSendMinInterval time.Duration
}

type LogReportPayload struct {
	ID      string      `json:"id"`
	Version string      `json:"version"`
	Data    interface{} `json:"data"`
}

type SyncCallParam struct {
	RequestTopic string
	RequestQos   byte
	ReplyTopic   string
	ReplyQos     byte
	Payload      string
	TimeoutMs    int64
}

type FileUploader interface {
	uploadFile(filepath string)
}

type UploadType string

const UploadTypeArenal UploadType = "Arenal"

type sourceType string

const sourceTypeDevice sourceType = "Device"
const sourceTypeArenal sourceType = "Arenal"

type UploadFileParam struct {
	// 文件名称，必传
	Filename string
	// 文件大小，必传
	Filesize int64
	// 分片大小，非必传，默认50MiB，必须大于等于5MiB，并且保证总分片数小于等于100
	PartSize int64
	// 业务ID，非必传，上传到用户TOS时可以传递，如果有默认的业务ID bucket可不传
	ServiceID string
	// 文件read seeker，必传
	FileReader io.ReadSeeker
}

type uploadFileParamInternal struct {
	reader     io.ReadSeeker
	filename   string
	filesize   int64
	partSize   int64
	uploadType UploadType
	// 触发源，平台/设备
	source    sourceType
	serviceID string
	filepath  string
}

type UploadFilePayload struct {
	ID      string `json:"ID"`
	Version string `json:"Version"`
	Params  struct {
		// 文件在设备上的路径
		// Required: true
		FilePath string `json:"FilePath"`
		// 上传类型
		// Required: true
		UploadType UploadType `json:"UploadType"`
		// 业务ID
		// Required: false
		ServiceID string `json:"ServiceID"`
	} `json:"Params"`
}

type UploadFileStatus string

type CommonHttpResponse struct {
	// 请求元信息
	ResponseMetadata ResponseMetadata `json:"ResponseMetadata"`
}

const (
	UploadFileStatusUploading UploadFileStatus = "Uploading"
	UploadFileStatusSuccess   UploadFileStatus = "Success"
	UploadFileStatusFailed    UploadFileStatus = "Failed"
)

type UploadFileReport struct {
	Status      UploadFileStatus `json:"Status"`
	Description string           `json:"Description"`
}

type UploadFileProgressPayload struct {
	ID      string           `json:"ID"`
	Version string           `json:"Version"`
	Params  UploadFileReport `json:"Params"`
}

type InitUploadFileParams struct {
	FileName  string     `json:"FileName"`
	FileSize  int64      `json:"FileSize"`
	PartSize  int64      `json:"PartSize"`
	ServiceID string     `json:"ServiceID"`
	Source    sourceType `json:"Source"`
}

type InitUploadFileRequest struct {
	ID      string               `json:"ID"`
	Version string               `json:"Version"`
	Params  InitUploadFileParams `json:"Params"`
}

type UploadFileURL struct {
	PartNum   int    `json:"PartNum"`
	UploadURL string `json:"UploadURL"`
}

type InitUploadFileResponseData struct {
	UploadID       string          `json:"UploadID"`
	FileID         string          `json:"FileID"`
	UploadFileURLs []UploadFileURL `json:"UploadFileURLs"`
}

type InitUploadFileResponse struct {
	CommonReply
	Data InitUploadFileResponseData `json:"Data"`
}

type EndUploadFileParams struct {
	UploadID      string         `json:"UploadID"`
	Success       bool           `json:"Success"`
	CRC64         uint64         `json:"CRC64"`
	UploadedParts []UploadedPart `json:"UploadedParts"`
}

type UploadedPart struct {
	PartNumber int    `json:"PartNumber"`
	ETag       string `json:"ETag"`
}

type EndUploadFileRequest struct {
	ID      string              `json:"ID"`
	Version string              `json:"Version"`
	Params  EndUploadFileParams `json:"Params"`
}

type NotifyAction string

const (
	NotifyActionExecute NotifyAction = "Execute"
	NotifyActionCancel  NotifyAction = "Cancel"
)

type TaskNotifyPayload struct {
	ID      string            `json:"ID"`
	Version string            `json:"Version"`
	Params  TaskPayloadParams `json:"Params"`
}

type ActionOnInterrupt string

const (
	ActionOnInterruptRetry = "Retry"
	ActionOnInterruptFail  = "Fail"
)

type TaskPayloadParams struct {
	// 任务ID，任务的唯一标识
	JobID string `json:"JobID"`
	// 任务类型
	Type JobType `json:"Type"`
	// 通知操作，有 Execute/Cancel 执行/取消
	Action NotifyAction `json:"Action"`
	// 任务超时时间
	TimeoutInMinutes int64 `json:"TimeoutInMinutes"`
	// 任务参数
	Param TaskParam `json:"Param"`
	// 任务异常中断处理，可选 重试/失败 Retry/Failed，默认失败
	ActionOnInterrupt string `json:"ActionOnInterrupt"`
}

type TaskParam struct {
	// 属性设置参数
	PropertySetParams map[string]interface{} `json:"PropertySetParam,omitempty"`
	// 服务调用参数
	ServiceCallParams *ServiceCallParam `json:"ServiceCallParams,omitempty"`
	// 配置下发参数
	ConfigUpdateParams *LogReportConfig `json:"ConfigUpdateParams,omitempty"`
	// 设备影子更新参数
	ShadowUpdateParams *ShadowDesiredParam `json:"ShadowUpdateParams,omitempty"`
	// 自定义任务参数，格式和内容自定义
	CustomParams *CustomTaskPayload `json:"CustomParams,omitempty"`
}

type CustomTaskPayload struct {
	// 自定义任务参数
	Param string `json:"Param"`
	// 文件URL
	URL string `json:"URL"`
	// 文件签名
	Sign string `json:"Sign"`
	// 文件大小
	FileSize int64 `json:"FileSize"`
}

type JobType string

const (
	JobTypePropertySet  JobType = "PropertySet"
	JobTypeServiceCall  JobType = "ServiceCall"
	JobTypeConfigUpdate JobType = "ConfigUpdate"
	JobTypeShadowUpdate JobType = "ShadowUpdate"
	JobTypeCustom       JobType = "Custom"
)

type CustomJobHandler func(message CustomJobMessage) (ErrorCode, string)

type CustomJobMessage struct {
	DeviceIdentification
	Param string `json:"Param"`
	// 文件路径，如果没有携带文件则为空
	FilePath string `json:"FilePath"`
}

type ExecutingJobMeta struct {
	TaskPayloadParams
	// 状态
	Status JobStatus `json:"Status"`
}

type ExecutingJob struct {
	ExecutingJobMeta
	jobCancelChannel  chan struct{}
	jobTimeoutChannel chan struct{}
}

type TaskProgressReportPayload struct {
	ID      string                  `json:"ID"`
	Version string                  `json:"Version"`
	Params  TaskProgressReportParam `json:"Params"`
}

type TaskProgressReportReply struct {
	ID    string    `json:"ID"`
	Code  ErrorCode `json:"Code"`
	JobID string    `json:"Data"`
}

type TaskGetPayload struct {
	ID      string `json:"ID"`
	Version string `json:"Version"`
}

type JobStatus string

const (
	JobStatusCreated     JobStatus = "Created"
	JobStatusDownloading JobStatus = "Downloading"
	JobStatusInProgress  JobStatus = "InProgress"
	JobStatusSuccess     JobStatus = "Success"
	JobStatusFailed      JobStatus = "Failed"
)

type TaskProgressReportParam struct {
	// 任务ID
	JobID string `json:"JobID"`
	// 状态
	Status JobStatus `json:"Status"`
	// 时间
	Time int64 `json:"Time"`
	// 错误吗
	ResultCode ErrorCode `json:"ResultCode"`
	// 错误描述
	ResultDesc string `json:"ResultDesc"`
}

type TaskGetReply struct {
	ID   string              `json:"ID"`
	Code ErrorCode           `json:"Code"`
	Data []TaskPayloadParams `json:"Data"`
}

type NTPRequest struct {
	CommonRequest
	DeviceSendTime int64 `json:"Params"`
}

// 密钥类型，支持Product/Device
type SecretType string

const (
	SecretTypeProduct SecretType = "Product"
	SecretTypeDevice  SecretType = "Device"
)

type GetServerTimeParam struct {
	DynamicRegisterParam
	DeviceSendTime int64      `json:"DeviceSendTime"`
	SecretType     SecretType `json:"SecretType"`
}

type NTPResponseData struct {
	DeviceSendTime int64 `json:"DeviceSendTime"`
	ServerRecvTime int64 `json:"ServerRecvTime"`
	ServerSendTime int64 `json:"ServerSendTime"`
}

type GetServerTimeHttpResponse struct {
	Result           NTPResponseData  `json:"Result"`
	ResponseMetadata ResponseMetadata `json:"ResponseMetadata"`
}

type NTPResponse struct {
	CommonReply
	Data NTPResponseData `json:"Data"`
}

type NTPResponseHandler func(serverTimeMS int64) ErrorCode

type OnConnectHandler func() ErrorCode

/***********************    网关相关模型     *************************/
type IoTGateway struct {
	base                           *Device
	subDeviceSecretMap             sync.Map
	addTopologiesReplyHandler      GatewayCommonHandler
	deleteTopologiesReplyHandler   GatewayCommonHandler
	getTopologiesReplyHandler      GatewayCommonHandler
	subDeviceLoginReplyHandler     GatewayCommonHandler
	subDeviceLogoutReplyHandler    GatewayCommonHandler
	notifyAddSubDeviceHandler      GatewayCommonHandler
	topologyChangeHandler          TopologyChangeHandler
	subDeviceChangeHandler         SubDeviceChangeHandler
	subDeviceSecretGetReplyHandler SubDeviceSecretGetReplyHandler
}

// DeviceAuthWithProductSecret 带产品密钥的设备认证信息
type DeviceAuthWithProductSecret struct {
	DeviceIdentification
	// 产品密钥
	// Required: true
	ProductSecret string `json:"product_secret"`
}

// DeviceAuthWithProductSecret 带设备密钥的设备认证信息
type DeviceAuthWithDeviceSecret struct {
	DeviceIdentification
	// 设备密钥
	// Required: true
	DeviceSecret string `json:"device_secret"`
}

type GatewayCommonHandler func(subDevices []DeviceIdentification) ErrorCode

type TopologyOperateType string

const (
	TopologyOperateTypeCreate  TopologyOperateType = "create"
	TopologyOperateTypeDelete  TopologyOperateType = "delete"
	TopologyOperateTypeEnable  TopologyOperateType = "enable"
	TopologyOperateTypeDisable TopologyOperateType = "disable"
)

type TopoChangeParam struct {
	OperateType TopologyOperateType    `json:"operate_type"`
	SubDevices  []DeviceIdentification `json:"sub_devices"`
}
type TopoChangeParamWithSecret struct {
	OperateType TopologyOperateType `json:"operate_type"`
	SubDevices  []SubDeviceSecret   `json:"sub_devices"`
}
type TopologyChangeHandler func(param TopoChangeParam) ErrorCode

type DeviceOperateType string
type SubDeviceChangeHandler func(param SubDeviceChangeParam) ErrorCode

type SubDeviceSecret struct {
	DeviceIdentification
	DeviceSecret DynamicRegisterResponse `json:"device_secret"`
}

type SubDeviceSecretReply struct {
	CommonReply
	Data []SubDeviceSecret `json:"data"`
}

type SubDeviceSecretGetReplyHandler func(subDeviceSecrets []DeviceAuthWithDeviceSecret) ErrorCode

type CommonRequest struct {
	ID      string `json:"ID"`
	Version string `json:"Version"`
}

type TopologyRequest struct {
	CommonRequest
	SubDevices []DeviceIdentification `json:"params"`
}

type SubDeviceParamWithAuth struct {
	DeviceIdentification
	// 随机数
	// Required: true
	RandomNum int64 `json:"random_num" `
	// 时间戳
	// Required: true
	Timestamp int64 `json:"timestamp"`
	// 签名
	// Required: true
	Signature string `json:"signature"`
}

type TopologyRequestWithAuth struct {
	CommonRequest
	SubDevices []SubDeviceParamWithAuth `json:"params"`
}

type CommonReply struct {
	ID   string    `json:"ID"`
	Code ErrorCode `json:"Code"`
}

type TopoReplyParam struct {
	CommonReply
	Data []DeviceIdentification `json:"data"`
}

type TopoChangePayload struct {
	CommonRequest
	Params TopoChangeParamWithSecret `json:"params"`
}

type SubDeviceChangeParam struct {
	OperateType DeviceOperateType      `json:"operate_type"`
	SubDevices  []DeviceIdentification `json:"sub_devices"`
}

type SubDeviceChangePayload struct {
	CommonRequest
	Params SubDeviceChangeParam `json:"params"`
}

type NotifyAddTopoRequest struct {
	TopologyRequest
	Timestamp int64 `json:"timestamp"`
}

type NotifyAddTopoReply struct {
	CommonReply
	Timestamp int64 `json:"timestamp"`
}

type SubDeviceLoginRequest struct {
	CommonRequest
	TriggerTime int64                    `json:"trigger_time"`
	SubDevices  []SubDeviceParamWithAuth `json:"params"`
}

type SubDeviceLogoutRequest struct {
	CommonRequest
	TriggerTime int64                  `json:"trigger_time"`
	SubDevices  []DeviceIdentification `json:"params"`
}

type SubDeviceLoginReportRequest struct {
	CommonRequest
	Params SubDeviceLoginReportParams `json:"params"`
}

type SubDeviceLoginReportParams struct {
	TriggerTime int64                  `json:"trigger_time"`
	SubDevices  []DeviceIdentification `json:"sub_devices"`
}

type EventReportParam struct {
	// 事件所属模块标识
	ModuleKey string
	// 事件标识
	Identifier string
	// 输出参数列表:key为标识符,value为具体值
	Values map[string]interface{}
}

type CustomTopicHandler func(param CustomTopicParam) ErrorCode

type CustomTopicParam struct {
	Topic   string
	Payload string
}
