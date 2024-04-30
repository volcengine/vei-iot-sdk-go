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

const (
	// PropertySetTopic 属性设置 sys/{productKey}/{deviceName}/thingmodel/service/preset/property/set
	PropertySetTopic string = "sys/%s/%s/thingmodel/service/preset/propertySet/post"

	// PropertySetReplyTopic 属性设置响应 sys/{productKey}/{deviceName}/thingmodel/service/preset/property/set_reply
	PropertySetReplyTopic string = "sys/%s/%s/thingmodel/service/preset/propertySet/post_reply"

	// ServiceCallTopicForSub 服务调用 sys/{productKey}/{deviceName}/thingmodel/service/{moduleKey}/{serviceIdentifier}/post/{uuid}
	ServiceCallTopicForSub string = "sys/%s/%s/thingmodel/service/+/+/post/+"

	// ServiceCallReplyTopic 服务调用响应 sys/{productKey}/{deviceName}/thingmodel/service/{moduleKey}/{serviceIdentifier}/post_reply/{uuid}
	ServiceCallReplyTopic string = "sys/%s/%s/thingmodel/service/%s/%s/post_reply/%s"

	// ShadowReportTopic 设备影子上报 sys/{productKey}/{deviceName}/shadow/report
	ShadowReportTopic string = "sys/%s/%s/shadow/report"
	// ShadowDesiredGetTopic 设备影子获取 sys/{productKey}/{deviceName}/shadow/desired/get
	ShadowDesiredGetTopic string = "sys/%s/%s/shadow/desired/get"
	// ShadowDesiredGetReplyTopic 设备影子获取reply sys/{productKey}/{deviceName}/shadow/desired/get_reply
	ShadowDesiredGetReplyTopic string = "sys/%s/%s/shadow/desired/get/reply"
	// ShadowDesiredClearTopic 设备期望状态清除 sys/{productKey}/{deviceName}/shadow/desired/clear
	ShadowDesiredClearTopic string = "sys/%s/%s/shadow/desired/clear"

	// ShadowReportReplyTopic 设备订阅，设备影子上报reply sys/{productKey}/{deviceName}/shadow/report_reply
	ShadowReportReplyTopic string = "sys/%s/%s/shadow/report_reply"

	// ShadowDesiredSetTopic 设备订阅，设备状态设置 sys/{productKey}/{deviceName}/shadow/set
	ShadowDesiredSetTopic string = "sys/%s/%s/shadow/desired/set"

	// EventReportTopic 事件上报 sys/{productKey}/{deviceName}/thingmodel/event/+/+/post
	EventReportTopic string = "sys/%s/%s/thingmodel/event/%s/%s/post"

	// PropertyReportTopic 属性上报 sys/{productKey}/{deviceName}/thingmodel/property/post
	PropertyReportTopic string = "sys/%s/%s/thingmodel/property/post"

	// PropertyReportReplyTopic 属性上报响应 sys/{productKey}/{deviceName}/thingmodel/property/post_reply
	PropertyReportReplyTopic string = "sys/%s/%s/thingmodel/property/post_reply"

	// EventReportReplyTopicForSub 事件上报响应 sys/{productKey}/{deviceName}/thingmodel/event/+/+/post_reply
	EventReportReplyTopicForSub string = "sys/%s/%s/thingmodel/event/+/+/post_reply"

	// OTAVersionReportTopic OTA设备上报版本信息 sys/{productKey}/{deviceName}/ota/version
	OTAVersionReportTopic string = "sys/%s/%s/ota/version"

	// OTAUpgradeNotifyTopic OTA平台推送升级通知 sys/{productKey}/{deviceName}/ota/notify/{otaJobID}
	OTAUpgradeNotifyTopic string = "sys/%s/%s/ota/notify/%s"

	// OTARequestUpgradeTopic OTA设备请求升级信息 sys/{productKey}/{deviceName}/ota/upgrade/post
	OTARequestUpgradeTopic string = "sys/%s/%s/ota/upgrade/post"

	// OTARequestUpgradeReplyTopic OTA设备请求升级信息相应 sys/{productKey}/{deviceName}/ota/upgrade/post_reply
	OTARequestUpgradeReplyTopic string = "sys/%s/%s/ota/upgrade/post_reply"

	// OTAUpgradeProgressReportTopic OTA设备上报升级进度 sys/{productKey}/{deviceName}/ota/progress/{otaJobID}
	OTAUpgradeProgressReportTopic string = "sys/%s/%s/ota/progress/%s"

	// WebShellCmdPostTopic webshell命令下发topic sys/{productKey}/{deviceName}/webshell/cmd/{uuid}/post
	WebShellCmdPostTopic string = "sys/%s/%s/webshell/cmd/+/post"

	// WebShellCmdReplyTopic webshell命令结果上报topic sys/{productKey}/{deviceName}/webshell/cmd/{uuid}/post_reply
	WebShellCmdReplyTopic string = "sys/%s/%s/webshell/cmd/%s/post_reply"

	// DeviceDelayPostTopic 设备延迟探测topic sys/{productKey}/{deviceName}/delay/{uuid}/post
	DeviceDelayPostTopic string = "sys/%s/%s/delay/+/post"

	// DeviceDelayReplyTopic 设备延迟探测结果上报topic sys/{productKey}/{deviceName}/delay/{uuid}/post
	DeviceDelayReplyTopic string = "sys/%s/%s/delay/%s/post_reply"

	// DeviceLogReportTopic 设备批量日志上报topic sys/{productKey}/{deviceName}/log/batch/report
	DeviceLogReportTopic string = "sys/%s/%s/log/batch/report"

	// DeviceLogReportConfigTopic 设备批量日志配置下发topic sys/{productKey}/{deviceName}/log/batch/config
	DeviceLogReportConfigTopic string = "sys/%s/%s/log/batch/config"

	// DeviceStreamLogConfigTopic 设备实时日志上报配置下发topic sys/{productKey}/{deviceName}/log/stream/config/{uuid}
	DeviceStreamLogConfigTopic string = "sys/%s/%s/log/stream/config/%s"

	// DeviceStreamLogReportTopic 设备实时日志上报topic sys/{productKey}/{deviceName}/log/stream/report/{uuid}
	DeviceStreamLogReportTopic string = "sys/%s/%s/log/stream/report/%s"

	// DeviceLocalLogConfigTopic 设备本地日志上报配置下发topic sys/{productKey}/{deviceName}/log/local/config/{uuid}
	DeviceLocalLogConfigTopic string = "sys/%s/%s/log/local/config/%s"

	// DeviceLocalLogReportTopic 设备本地日志上报topic sys/{productKey}/{deviceName}/log/local/report/{uuid}
	DeviceLocalLogReportTopic string = "sys/%s/%s/log/local/report/%s"

	// DeviceNotifyUploadFileTopic 通知设备上传文件topic sys/{productKey}/{deviceName}/file/upload/notify/{uuid}
	DeviceNotifyUploadFileTopic string = "sys/%s/%s/file/upload/notify/%s"

	// UploadFileProgressTopic 设备上传文件进度上报 sys/{productKey}/{deviceName}/file/upload/progress/{uuid}
	UploadFileProgressTopic string = "sys/%s/%s/file/upload/progress/%s"

	// TaskNotifyTopic 通知设备执行任务topic sys/{productKey}/{deviceName}/task/notify
	TaskNotifyTopic string = "sys/%s/%s/task/notify"

	// TaskProgressTopic 设备任务执行进度上报topic sys/{productKey}/{deviceName}/task/progress
	TaskProgressTopic string = "sys/%s/%s/task/progress"

	// TaskProgressReplyTopic 设备任务执行进度上报响应topic sys/{productKey}/{deviceName}/task/progress_reply
	TaskProgressReplyTopic string = "sys/%s/%s/task/progress_reply"

	// TaskGetTopic 设备获取待执行任务topic sys/{productKey}/{deviceName}/task/get
	TaskGetTopic string = "sys/%s/%s/task/get"

	// TaskGetReplyTopic 设备获取待执行任务响应 sys/{productKey}/{deviceName}/task/get_reply
	TaskGetReplyTopic string = "sys/%s/%s/task/get_reply"

	// NTPRequestTopic 设备NTP请求topic sys/{productKey}/{deviceName}/ntp/request
	NTPRequestTopic string = "sys/%s/%s/ntp/request"

	// NTPResponseTopic 设备NTP请求响应topic sys/{productKey}/{deviceName}/ntp/response
	NTPResponseTopic string = "sys/%s/%s/ntp/response"

	// InitUploadFileRequestTopic 初始化设备上传文件请求topic sys/{productKey}/{deviceName}/file/upload/init/{uuid}
	InitUploadFileRequestTopic string = "sys/%s/%s/file/upload/init/%s"

	// InitUploadFileReplyTopic 初始化设备文件上传响应topic sys/{productKey}/{deviceName}/file/upload/init_reply/{uuid}
	InitUploadFileReplyTopic string = "sys/%s/%s/file/upload/init_reply/%s"

	// EndUploadFileRequestTopic 结束设备上传文件请求topic sys/{productKey}/{deviceName}/file/upload/end/{uuid}
	EndUploadFileRequestTopic string = "sys/%s/%s/file/upload/end/%s"

	// EndUploadFileReplyTopic 结束设备文件上传响应topic sys/{productKey}/{deviceName}/file/upload/end_reply/{uuid}
	EndUploadFileReplyTopic string = "sys/%s/%s/file/upload/end_reply/%s"
)

// 网关相关topic
const (
	// GatewayTopoAddTopic 增加拓扑关系 sys/{gwProductKey}/{gwDeviceName}/gateway/topo/add
	GatewayTopoAddTopic = "sys/%s/%s/gateway/topo/add"
	// GatewayTopoAddReplyTopic 增加拓扑关系请求响应 sys/{gwProductKey}/{gwDeviceName}/gateway/topo/add_reply
	GatewayTopoAddReplyTopic = "sys/%s/%s/gateway/topo/add_reply"
	// GatewayTopoDeleteTopic 删除拓扑关系 sys/{gwProductKey}/{gwDeviceName}/gateway/topo/delete
	GatewayTopoDeleteTopic = "sys/%s/%s/gateway/topo/delete"
	// GatewayTopoDeleteReplyTopic 删除拓扑关系请求响应 sys/{gwProductKey}/{gwDeviceName}/gateway/topo/delete_reply
	GatewayTopoDeleteReplyTopic = "sys/%s/%s/gateway/topo/delete_reply"
	// GatewayTopoGetTopic 获取拓扑关系 sys/{gwProductKey}/{gwDeviceName}/gateway/topo/get
	GatewayTopoGetTopic = "sys/%s/%s/gateway/topo/get"
	// GatewayTopoGetReplyTopic 获取拓扑关系请求响应 sys/{gwProductKey}/{gwDeviceName}/gateway/topo/get_reply
	GatewayTopoGetReplyTopic = "sys/%s/%s/gateway/topo/get_reply"
	// GatewaySubDeviceSecretGetTopic 子设备密钥获取请求 sys/{gwProductKey}/{gwDeviceName}/gateway/sub/secret/get/{uuid}
	GatewaySubDeviceSecretGetTopic = "sys/%s/%s/gateway/sub/secret/get/%s"
	// GatewaySubDeviceSecretGetReplyTopic 子设备密钥获取请求响应 sys/{gwProductKey}/{gwDeviceName}/gateway/sub/secret/get_reply/{uuid}
	GatewaySubDeviceSecretGetReplyTopic = "sys/%s/%s/gateway/sub/secret/get_reply/%s"
	// GatewaySubDeviceLoginTopic 子设备上线请求 sys/{gwProductKey}/{gwDeviceName}/gateway/sub/login
	GatewaySubDeviceLoginTopic = "sys/%s/%s/gateway/sub/login"
	// GatewaySubDeviceLoginReplyTopic 子设备上线请求响应 sys/{gwProductKey}/{gwDeviceName}/gateway/sub/login_reply
	GatewaySubDeviceLoginReplyTopic = "sys/%s/%s/gateway/sub/login_reply"
	// GatewaySubDeviceLogoutTopic 子设备下线请求 sys/{gwProductKey}/{gwDeviceName}/gateway/sub/logout"
	GatewaySubDeviceLogoutTopic = "sys/%s/%s/gateway/sub/logout"
	// GatewaySubDeviceLogoutReplyTopic 子设备下线请求响应 sys/{gwProductKey}/{gwDeviceName}/gateway/sub/logout_reply
	GatewaySubDeviceLogoutReplyTopic = "sys/%s/%s/gateway/sub/logout_reply"
	// GatewaySubDeviceDiscoveryTopic 上报发现的子设备列表 sys/{gwProductKey}/{gwDeviceName}/gateway/sub/discovery
	GatewaySubDeviceDiscoveryTopic = "sys/%s/%s/gateway/sub/discovery"
	// GatewaySubDeviceDiscoveryReplyTopic 发现子设备列表请求响应 sys/{gwProductKey}/{gwDeviceName}/gateway/sub/discovery_reply
	GatewaySubDeviceDiscoveryReplyTopic = "sys/%s/%s/gateway/sub/discovery_reply"
	// GatewayTopoAddNotifyTopic 通知网关添加子设备 sys/{gwProductKey}/{gwDeviceName}/gateway/topo/add/notify/{uuid}
	GatewayTopoAddNotifyTopic = "sys/%s/%s/gateway/topo/notify/+"
	// GatewayTopoAddNotifyReplyTopic 通知网关添加子设备响应 sys/{gwProductKey}/{gwDeviceName}/gateway/topo/add/notify_reply/{uuid}
	GatewayTopoAddNotifyReplyTopic = "sys/%s/%s/gateway/topo/notify_reply/%s"
	// GatewayTopoChangeTopic 通知网关拓扑关系变化 sys/{gwProductKey}/{gwDeviceName}/gateway/topo/chang
	GatewayTopoChangeTopic = "sys/%s/%s/gateway/topo/change"
	// GatewayTopoChangeReplyTopic 通知网关拓扑关系变化响应 sys/{gwProductKey}/{gwDeviceName}/gateway/topo/change_reply
	GatewayTopoChangeReplyTopic = "sys/%s/%s/gateway/topo/change_reply"
	// GatewaySubDeviceChangeTopic 通知网关子设备状态变化 sys/{gwProductKey}/{gwDeviceName}/gateway/sub/change
	GatewaySubDeviceChangeTopic = "sys/%s/%s/gateway/sub/change"
	// GatewaySubDeviceChangeReplyTopic 通知网关子设备状态变化响应 sys/{gwProductKey}/{gwDeviceName}/gateway/sub/change_reply
	GatewaySubDeviceChangeReplyTopic = "sys/%s/%s/gateway/sub/change_reply"
	// GatewaySubDeviceLoginReport 网关定期上报在线子设备 sys/{gwProductKey}/{gwDeviceName}/gateway/sub/report
	GatewaySubDeviceLoginReport = "sys/%s/%s/gateway/sub/report"
)
