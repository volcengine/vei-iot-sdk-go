# vei-iot-sdk-go
vei-iot-sdk-go是火山引擎边缘智能产品提供的设备接入SDK的Go语言版本，提供设备和边缘智能平台之间的通讯能力，包括属性上报、属性设置、服务调用、事件上报、设备影子等基础服务，以及OTA升级等高级服务。开发者可以使用SDK快速便捷的将边缘智能设备接入火山引擎边缘智能平台。

[边缘智能产品主页](https://www.volcengine.com/product/vei/mainpage)

## 功能支持
* [设备连接鉴权](#设备连接鉴权)

* [设备属性上报](#设备属性上报)

* [设备属性设置](#设备属性设置)

* [设备服务调用](#设备服务调用)

* [设备事件上报](#设备事件上报)

* [设备影子上报](#设备影子上报)

* [设备影子设置](#设备影子设置)

* [自定义Topic](#自定义topic)

* [OTA相关能力](#ota相关能力)
  
* [WebShell](#webshell)


## 设备连接鉴权
详细Sample请见[`samples/thingmodel/dynamic_register.go`](samples/thingmodel/dynamic_register.go)

## 设备属性上报
详细Sample请见[`samples/thingmodel/property_report.go`](samples/thingmodel/property_report.go)

## 设备属性设置
详细Sample请见[`samples/thingmodel/property_set.go`](samples/thingmodel/property_set.go)

## 设备服务调用
详细Sample请见[`samples/thingmodel/service_call.go`](samples/thingmodel/service_call.go)

## 设备事件上报
详细Sample请见[`samples/thingmodel/event_report.go`](samples/thingmodel/event_report.go)

## 设备影子上报
详细Sample请见[`samples/thingmodel/shadow_report.go`](samples/thingmodel/shadow_report.go)

## 设备影子设置
详细Sample请见[`samples/thingmodel/shadow_set.go`](samples/thingmodel/shadow_set.go)

## 自定义Topic
详细Sample请见[`samples/thingmodel/custom_topic.go`](samples/thingmodel/custom_topic.go)

## OTA相关能力
OTA相关能力包括升级通知的接收、升级进度上报、升级请求以及升级应答信息的接受。
下面将对流程以及各功能配置进行讲解。
1. 设备初始化
首先，我们需要对设备进行初始化。调用`arenal.CreateDeviceWithConfig`方法，传递`DeviceOption`即可对设备进行初始化。
`DeviceOption`有两种创建方式：
- 一种方式是自行填写内容：
```go
d := arenal.CreateDeviceWithConfig(arenal.DeviceOption{
	Name:       "xxx",
	ProductKey: "xxxxxxxxx",
	Region:     arenal.RegionBoe,
	VerifyMode: arenal.VerifyModeDynamicNoPreRegistered,
	FilePath: "",
	ModuleConfig: map[string]arenal.ModuleInfo{
		"default":arenal.ModuleInfo{
			ModuleKey: "default",
			Version: "1.0",
		},
	},
	UserDefineTopicQosMap: map[string]byte{
		"sys/topic/demo":1,
	},
})
if ok := d.Init(); !ok {
	log.Error("device init failed")
}
```
- 另一种方式是使用yaml文件配置，文件结构详见sample下的config_template.yaml；使用`arenal.InitOptionWithConfigFile`方法，传递yaml文件所在路径即可生成对应的`DeviceOption`：
```go
deviceOption,err := arenal.InitOptionWithConfigFile("config/config.yaml")
if err != nil{
	fmt.Printf("error is : %s",err.Error())
	return
}
d := arenal.CreateDeviceWithConfig(*deviceOption)
if ok := d.Init(); !ok {
    log.Error("device init failed")
}
```
2. 配置升级通知接收器（可选）
调用设备提供的`SetOTANotifyHandler`方法，传递固定结构的function即可。每当通知消息到来时，就会进入该function。默认情况下，会直接发起升级请求，对目标模块进行升级：
```go
// 配置OTA升级通知Handler，设备支持OTA功能必须进行实现
d.SetOTANotifyHandler(func(message arenal.OTANotifyPayload) {
	// 此处实现接收到升级通知时，需要进行的操作 
	messageStr, _ := json.Marshal(message)
	fmt.Println(fmt.Sprintf("Receive Message Success!message = %v", string(messageStr)))
	// 进行某项OTA升级前，需要先调用OTAUpgradeRequest申请更新包url，平台返回会直接进入OTAReplyPayload进行处理
	// 需要主动填写需要更新的module，如果有可用更新，则会通过Reply返回更新信息；若OTAUpgradeRequest参数otaJobID为空，表示请求最新的升级任务，建议不指定 
	code := d.OTAUpgradeRequest(message.Data.Module, "", message.DeviceIdentification)
	if code != arenal.SuccessCode {
        // 错误处理
        log.Error("upgrade Request failed")
    }
})
```
3. 配置差分还原算法（可选，如果存在差分包必须进行配置），调用设备提供的`SetDiffAlgorithm`方法，传递固定结构的function：
```go
// 如果存在差分包的情况，必须准备好对应的反差分算法，否则可能会导致安装失败
var diffAlgorithm arenal.DiffAlgorithm
diffAlgorithm = func(diffFileName string) arenal.DiffAlgorithmResult {
//具体的生成整包流程
    return arenal.DiffAlgorithmResult{
        IsSuccess:      true,
        ResDesc:        "",
        TargetFilePath: diffFileName,
    }
}
d.SetDiffAlgorithm(diffAlgorithm)
```
4. 配置安装包安装流程（支持OTA升级的设备，必须配置），调用设备提供的`SetInstallAlgorithm`方法，传递固定结构的function：
```go
// 需要提前准备好安装包烧写流程
var InstallAlgorithm arenal.InstallAlgorithm
InstallAlgorithm = func(module, filename string) arenal.InstallAlgorithmResult {
    // 这里传递的参数为最终的整包路径，如果不是差分，则为下载路径，是差分则为反差分算法形成的整包路径
    log.Infof("installing：filename:%s", filename)
    time.Sleep(10 * time.Second)
    return arenal.InstallAlgorithmResult{
        IsSuccess:     true,
        ResDesc:       "",
        DeviceVersion: tmpVersion,
        NeedReboot:    false,
    }
}
d.SetInstallAlgorithm(InstallAlgorithm)
```
5. 配置OTA升级请求应答消息接收器（可选）
   调用设备提供的`SetOTAReplyHandler`方法，传递固定结构的function，平台下发的升级任务信息将在这一接收器中进行处理。默认情况下，会直接触发设备升级，根据传递的模块信息、安装包信息等内容进行OTA升级：
```go
// 建议初始化升级任务->下载->安装，也可以自定义升级流程，例如稍后下载/达到某个外界条件时安装，但初始化升级任务是必要步骤
d.SetOTAReplyHandler(func(message arenal.OTAReplyPayload) {
    messageStr, _ := json.Marshal(message)
    fmt.Println(fmt.Sprintf("receive message success!message = %v", string(messageStr)))
    tmpVersion = message.Data.DestVersion
    // 5.1 升级任务初始化
    code, otaMetadata := d.InitOTAJob(message.DeviceIdentification, message.Data)
    if code != arenal.SuccessCode {
        // 错误处理
        log.Infof("no available OTA job was found for the OTA reply message,code:%d,otaMetadata:%v", code, otaMetadata)
        return
    }

    // 5.2 下载流程
    code = d.OTADownload(otaMetadata.OTAModuleName)
    if code != arenal.SuccessCode {
        // 错误处理
        log.Errorf("OTA download failed,code:%d,otaMetadata:%v", code, otaMetadata)
    }

    // 5.3 安装流程
    code = d.OTAInstall(otaMetadata.OTAModuleName)
    if code != arenal.SuccessCode {
        // 错误处理
        log.Errorf("OTA install failed,code:%d,otaMetadata:%v", code, otaMetadata)
    }
})
```
6. 主动请求升级包信息，调用设备提供的`OTAUpgradeRequest`方法：
```go
code := d.OTAUpgradeRequest("default", "")
if code != arenal.SuccessCode {
	// 错误处理
	log.Error("upgrade Request failed")
}
```
完整的Sample详见[`samples/ota/ota.go`](samples/ota/ota.go)。

## Webshell
详细Sample请见[`samples/webshell/webshell.go`](samples/webshell/webshell.go)。

## 许可证
[Apache-2.0 License](LICENSE).
