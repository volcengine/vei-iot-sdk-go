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

type ErrorCode int

const (
	// 成功
	SuccessCode ErrorCode = 0
	// 参数解析失败
	ErrorCodeParamUnmarshalFailed ErrorCode = 100001
	// 执行函数异常panic
	ErrorCodeFuncExecPanic ErrorCode = 100002
	// 消息Pub失败
	ErrorCodePubFailed ErrorCode = 100003
	// 文件操作失败
	ErrorCodeFileOperateFailed ErrorCode = 100004
	// 暂无可用升级任务
	ErrorCodeAvailableOTAJobNotFound ErrorCode = 100005
	// 当前设备的该OTA模块已经处于升级流程中
	ErrorCodeOTAJobLocked ErrorCode = 100006
	// 模块不存在
	ErrorCodeModuleNotExist ErrorCode = 100007
	// Handler不存在
	ErrorCodeHandlerNotExist ErrorCode = 100008
	// Decode失败
	ErrorCodeDecodeFailed ErrorCode = 100009
	// 升级取消
	ErrorCodeUpgradeCanceled ErrorCode = 100010
	// 订阅失败
	ErrorCodeSubFailed ErrorCode = 100011
	// 参数不合法
	ErrorCodeParamNotValid ErrorCode = 100012
	// 设备名称不合法
	ErrorCodeDeviceNameInvalid     ErrorCode = 100013
	ErrorCodeInternalError         ErrorCode = 100014
	ErrorCodeDeviceNameInvalidDesc           = "The device name only supports English letters, numbers, and special characters _-.:@, and no more than 128 characters."
	// 加密失败
	ErrorCodeEncryptFailed ErrorCode = 100015
	// 同步调用超时
	ErrorCodeSyncCallTimeout ErrorCode = 100016
	// OTA ResultCode
	OTAErrorCodeModuleNotExist    ErrorCode = 100017
	OTAErrorCodeDownloadFailed    ErrorCode = 100018
	OTAErrorCodeCheckFailed       ErrorCode = 100019
	OTAErrorCodeInstallFailed     ErrorCode = 100020
	OTAErrorCodeTimeout           ErrorCode = 100021
	OTAErrorCodeDiffRecoverFailed ErrorCode = 100022
	// 网关错误码
	GatewayErrorCodeTopoNotExist ErrorCode = 100023
	// Task ResultCode
	JobErrorCodeDownloadFailed      ErrorCode = 100024
	JobErrorCodeFileSignCheckFailed ErrorCode = 100025
	JobErrorCodeCanceled            ErrorCode = 100026
	JobErrorCodeTimeout             ErrorCode = 100027
	JobErrorCodeJobNotFound         ErrorCode = 100028

	// common
	CommonErrorCodeResourceNotFound   ErrorCode = 100029
	CommonErrorCodeCloudInternalError           = 100030 // 内部错误

	// http access
	HTTPErrorCodeEncrypt                ErrorCode = 100031
	HTTPErrorCodeDoRequest              ErrorCode = 100032
	HTTPErrorCodeUnmarshalBody          ErrorCode = 100033
	HTTPErrorCodeParamInvalid           ErrorCode = 100034
	HTTPErrorCodeSignInvalid            ErrorCode = 100035
	HTTPErrorCodeTokenInvalid           ErrorCode = 100036
	HTTPErrorCodeTokenExpired           ErrorCode = 100037
	HTTPErrorCodeTopicNotSupport        ErrorCode = 100038
	HTTPErrorCodeMessageTPSLimitReached ErrorCode = 100039 // TPS达到阈值

	//  物模型
	ThingModelErrorCodeReportPropertyLimitReached ErrorCode = 100040 // 单次上报属性数量限制500

	// QoS当前仅支持0和1
	ErrorCodeQoSNotSupport ErrorCode = 100041
)

// 服务端接口错误码
const (
	cloudCommonErrorCodeSuccess                          = 0
	cloudCommonErrorCodeNotFound                         = 10000050 // 资源不存在
	cloudCommonErrorCodeInternalError                    = 10000010 // 内部错误
	cloudDeviceHTTPAccessErrorCodeInvalidSign            = 27000010 // 签名错误
	cloudDeviceHTTPAccessErrorCodeTokenExpired           = 27000020 // token过期
	cloudDeviceHTTPAccessErrorCodeTokenInvalid           = 27000030 // token不正确
	cloudDeviceHTTPAccessErrorCodeTopicNotSupport        = 27000040 // 不支持的topic
	cloudDeviceHTTPAccessErrorCodeMessageTPSLimitReached = 27000050 // TPS达到阈值
)

var errorCodeMap = map[int]ErrorCode{
	cloudCommonErrorCodeSuccess:                          SuccessCode,
	cloudCommonErrorCodeNotFound:                         CommonErrorCodeResourceNotFound,
	cloudCommonErrorCodeInternalError:                    CommonErrorCodeCloudInternalError,
	cloudDeviceHTTPAccessErrorCodeInvalidSign:            HTTPErrorCodeSignInvalid,
	cloudDeviceHTTPAccessErrorCodeTokenExpired:           HTTPErrorCodeTokenExpired,
	cloudDeviceHTTPAccessErrorCodeTokenInvalid:           HTTPErrorCodeTokenInvalid,
	cloudDeviceHTTPAccessErrorCodeTopicNotSupport:        HTTPErrorCodeTopicNotSupport,
	cloudDeviceHTTPAccessErrorCodeMessageTPSLimitReached: HTTPErrorCodeMessageTPSLimitReached,
}

func convertCloudCodeToDeviceErrorCode(cloudCode int) ErrorCode {
	code, ok := errorCodeMap[cloudCode]
	if ok {
		return code
	}
	return ErrorCodeInternalError
}
