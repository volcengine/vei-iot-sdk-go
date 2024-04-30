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

package main

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/volcengine/vei-iot-sdk-go/arenal"
	"os"
)

func main() {
	// 设备初始化
	// 1. 设备已预先注册
	// 使用yaml文件初始化设备信息（建议）
	deviceOption := arenal.NewDeviceOption()
	deviceOption.WithDeviceInfo(
		"SAMPLE_PRODUCT_KEY",
		"SAMPLE_PRODUCT_SECRET",
		"SAMPLE_DEVICE_NAME",
		"SAMPLE_DEVICE_SECRET",
		arenal.VerifyModeDeviceSecret,
	)
	deviceOption.WithMQTTEndpoint("SAMPLE_MQTT_HOST")
	deviceOption.WithMQTTPort(8883)
	deviceOption.WithEnableTls(true)
	deviceOption.WithCloudHTTPEndpoint("SAMPLE_HTTP_HOST")
	deviceOption.WithIsGateway(true)
	deviceOption.WithInstanceID("SAMPLE_INSTANCE_ID")
	d := arenal.CreateDeviceWithConfig(*deviceOption)

	if ok := d.Init(); !ok {
		// 设备自身日志
		fmt.Println("Init Failed...")
	}

	// 创建测试文件
	file, err := os.OpenFile("testfile", os.O_CREATE|os.O_RDWR, 0755)
	if err != nil {
		log.Errorf("open file failed: %s", err.Error())
		return
	}

	toBeWritten := make([]byte, 1024*1024)
	_, err = file.Write(toBeWritten)
	if err != nil {
		log.Errorf("write file failed: %s", err.Error())
		return
	}
	file.Close()

	file, err = os.Open("testfile")
	log.Info("start upload file")
	stat, _ := file.Stat()
	// 上传到平台或者用户默认tos bucket
	fileID, errCode := d.UploadFile(arenal.UploadFileParam{
		Filename:   "test",
		Filesize:   stat.Size(),
		FileReader: file,
	})
	log.Infof("fileID: %s, errCode: %d", fileID, errCode)
}
