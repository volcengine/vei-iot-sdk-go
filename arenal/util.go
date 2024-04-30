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
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"go.uber.org/atomic"
	"gopkg.in/yaml.v3"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func generateMqttPassword(authType AuthType, productKey, deviceName, deviceSecret string) string {
	values := make([]string, 0)
	values = append(values, strconv.Itoa(int(authType)))
	randomNum := getRandomNum()
	values = append(values, strconv.FormatInt(randomNum, 10))
	timestamp := time.Now().Unix()
	values = append(values, strconv.FormatInt(timestamp, 10))
	param := DynamicRegisterBasicParam{
		ProductKey: productKey,
		DeviceName: deviceName,
		AuthType:   authType,
		Timestamp:  timestamp,
		RandomNum:  randomNum,
	}
	sign, err := hmacSHA256Encrypt(param, deviceSecret)
	if err != nil {
		return ""
	}
	values = append(values, sign)
	return strings.Join(values, "|")
}

func hmacSHA256Encrypt(param DynamicRegisterBasicParam, secret string) (string, error) {
	// 对参数（auth_type,device_name、random_num、product_key、timestamp）按字典序升序排序
	content := fmt.Sprintf("auth_type=%d&device_name=%s&random_num=%d&product_key=%s&timestamp=%d", param.AuthType, param.DeviceName, param.RandomNum, param.ProductKey, param.Timestamp)
	h := hmac.New(sha256.New, []byte(secret))
	_, err := h.Write([]byte(content))
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(h.Sum(nil)), nil
}

func getRandomNum() int64 {
	return rand.Int63()
}

func formatMqttUserName(productKey, deviceName string) string {
	return fmt.Sprintf("%s|%s", productKey, deviceName)
}

func getDeviceInfoFromClientId(clientId string) (string, string) {
	tmpStrs := strings.Split(clientId, "|")
	if len(tmpStrs) != 2 {
		return "", ""
	}
	return tmpStrs[0], tmpStrs[1]
}

func formatOTAModuleKey(productKey, deviceName, module string) string {
	return fmt.Sprintf("%s|%s|%s", productKey, deviceName, module)
}

func getRandomStringWithTimeSuffix(n int) string {
	randBytes := make([]byte, n/2)
	rand.Read(randBytes)
	return fmt.Sprintf("%x%s", randBytes, time.Now().Format("060102150405"))
}

// FormatUniqueFieldName 构建唯一标识的方法
// 传递moduleKey，identifier
func FormatUniqueFieldName(moduleKey, identifier string) string {
	return fmt.Sprintf("%s:%s", moduleKey, identifier)
}

func fileCheck(filename, sign string) bool {
	return md5Check(filename, sign)
}

func md5Check(filename string, sign string) bool {
	fileMd5 := getFileMd5(filename)
	if sign != fileMd5 {
		return false
	}
	return true
}

// GetTimeMill 获取时间戳
// 返回 int64 类型时间戳
func GetTimeMill() int64 {
	return time.Now().UnixNano() / 1000 / 1000
}

func getFileMd5(filename string) string {
	// 文件全路径名
	pFile, err := os.Open(filename)
	if err != nil {
		return ""
	}
	defer pFile.Close()
	md5h := md5.New()
	_, err = io.Copy(md5h, pFile)
	if err != nil {
		return ""
	}
	return hex.EncodeToString(md5h.Sum(nil))
}

func getMD5FromReader(reader io.Reader) (string, error) {
	md5h := md5.New()
	_, err := io.Copy(md5h, reader)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(md5h.Sum(nil)), nil
}

func formatDownloadFilePath(dirPath string, url string) string {
	split := strings.Split(url, "/")
	filename := strings.Split(split[len(split)-1], "?")[0]
	return filepath.Join(dirPath, filename)
}

func getSecret(payload, productSecret string) string {
	decoded, _ := base64.StdEncoding.DecodeString(payload)
	rs, _ := aesDecrypt(decoded, []byte(productSecret[:16]))
	return string(rs)
}

func aesDecrypt(crypted, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	// AES分组长度为128位，所以blockSize=16，单位字节
	blockSize := block.BlockSize()
	blockMode := cipher.NewCBCDecrypter(block, key[:blockSize]) // 初始向量的长度必须等于块block的长度16字节
	origData := make([]byte, len(crypted))
	blockMode.CryptBlocks(origData, crypted)
	origData = PKCS5UnPadding(origData)
	return origData, nil
}

func PKCS5UnPadding(origData []byte) []byte {
	length := len(origData)
	unpadding := int(origData[length-1])
	return origData[:(length - unpadding)]
}

func initModuleConfig(moduleList []ModuleInfo) map[string]ModuleInfo {
	moduleConfig := make(map[string]ModuleInfo)
	for _, module := range moduleList {
		moduleConfig[module.ModuleKey] = module
	}
	return moduleConfig
}

func initUserDefineTopicList(userDefineTopics UserDefineTopicConfig) map[string]byte {
	userDefineTopicQosMap := make(map[string]byte)
	for _, qos0Topic := range userDefineTopics.Qos0 {
		userDefineTopicQosMap[qos0Topic] = 0
	}

	for _, qos1Topic := range userDefineTopics.Qos1 {
		userDefineTopicQosMap[qos1Topic] = 1
	}
	return userDefineTopicQosMap
}

// InitOptionWithConfigFile 根据指定的配置文件生成对应的 DeviceOption
// 输入： 配置文件所在的位置（路径）
// 输出： *DeviceOption，error
func InitOptionWithConfigFile(configPath string) (*DeviceOption, error) {
	content, err := ioutil.ReadFile(configPath)
	if err != nil {
		log.Errorf("config init failed, err is %s", err.Error())
		return nil, err
	}
	ConfigFilePath = configPath
	var deviceConfig DeviceConfig
	err = yaml.Unmarshal(content, &deviceConfig)
	if err != nil {
		log.Errorf("config init failed, err is %s", err.Error())
		return nil, err
	}
	moduleConfig := initModuleConfig(deviceConfig.OTAModule)
	subDeviceModuleConfig := initModuleConfig(deviceConfig.SubDeviceOTAModule)
	userDefineTopicList := initUserDefineTopicList(deviceConfig.TopicConfig)

	deviceOption := DeviceOption{
		InstanceID:              deviceConfig.InstanceID,
		Name:                    deviceConfig.DeviceName,
		Secret:                  deviceConfig.DeviceSecret,
		ProductKey:              deviceConfig.ProductKey,
		ProductSecret:           deviceConfig.ProductSecret,
		MQTTHost:                deviceConfig.MQTTHost,
		MQTTPort:                deviceConfig.MQTTPort,
		CloudHTTPHost:           deviceConfig.CloudHTTPHost,
		EnableTls:               deviceConfig.EnableTls,
		VerifyMode:              DeviceVerifyMode(deviceConfig.VerifyMode),
		IsGateway:               deviceConfig.IsGateway,
		FilePath:                deviceConfig.FilePath,
		OTAHistoryJobReloadMode: OTAHistoryJobReloadMode(deviceConfig.OTAHistoryJobReloadMode),
		ModuleConfig:            moduleConfig,
		SubDeviceModuleConfig:   subDeviceModuleConfig,
		UserDefineTopicQosMap:   userDefineTopicList,
		LogReportConfig: LogReportConfig{
			LogReportCommonConfig: deviceConfig.LogReportConfig,
		},
		WebshellExecutor: deviceConfig.WebshellExecutor,
		KeepAlive:        deviceConfig.KeepAlive,
	}
	return &deviceOption, err
}

func currentTimeMill() int64 {
	return time.Now().UnixNano() / 1000 / 1000
}

func syncCall(client paho.Client, param SyncCallParam) ([]byte, ErrorCode) {
	if param.TimeoutMs == 0 {
		param.TimeoutMs = 3000
	}
	if token := client.Publish(param.RequestTopic, param.RequestQos, false, param.Payload); token.Wait() && token.Error() != nil {
		return nil, ErrorCodePubFailed
	}
	var success atomic.Bool
	success.Store(false)
	var payLoad []byte
	var handler paho.MessageHandler = func(client paho.Client, msg paho.Message) {
		success.Store(true)
		payLoad = msg.Payload()
	}
	if token := client.Subscribe(param.ReplyTopic, param.ReplyQos, handler); token.Wait() && token.Error() != nil {
		return nil, ErrorCodeSubFailed
	}
	start := currentTimeMill()
	for currentTimeMill()-start < param.TimeoutMs {
		if success.Load() {
			return payLoad, SuccessCode
		}
		time.Sleep(10 * time.Millisecond)
	}
	if success.Load() {
		return payLoad, SuccessCode
	}
	return nil, ErrorCodeSyncCallTimeout
}

func formatCustomTopic(productKey, deviceName, topicSuffix string) string {
	return fmt.Sprintf("sys/%s/%s/custom/%s", productKey, deviceName, topicSuffix)
}

func writeContentToFile(content string, filepath string) error {
	file, err := os.OpenFile(filepath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return fmt.Errorf("open file %s Failed,err is: %s", filepath, err.Error())
	}
	defer file.Close()
	_, err = file.Write([]byte(content))
	return err
}

func fileExist(path string) bool {
	_, err := os.Lstat(path)
	return !os.IsNotExist(err)
}

func (d *Device) panicToError(funcName string, f func()) (err error) {
	defer func() {
		if e := recover(); e != nil {
			err = fmt.Errorf(panicTrace(e))
			d.sdkLogf(LogLevelError, "%s exit occurs due to an exception goroutine panic, %s", funcName, err.Error())
		}
	}()
	f()
	return
}

func panicTrace(err interface{}) string {
	stackBuf := make([]byte, 4096)
	n := runtime.Stack(stackBuf, false)

	return fmt.Sprintf("panic: %v %s", err, stackBuf[:n])
}

func saveFile(filepath, md5Filepath string, bytes []byte) error {
	tmpFilePath := filepath + ".tmp"
	file, err := os.OpenFile(tmpFilePath, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0664)
	if err != nil {
		return err
	}

	_, err = file.Write(bytes)
	file.Close()
	if err == nil {
		os.Rename(tmpFilePath, filepath)
	} else {
		os.Remove(tmpFilePath)
		return err
	}

	// 更新校验文件
	code := getFileMd5(filepath)
	tmpFilePath = md5Filepath + ".tmp"
	err = ioutil.WriteFile(tmpFilePath, []byte(code), 0664)
	if err == nil {
		os.Rename(tmpFilePath, md5Filepath)
	} else {
		os.Remove(tmpFilePath)
	}
	return err
}

func readFile(filepath, md5Filepath string) (bytes []byte, err error) {
	_, err1 := os.Stat(filepath)
	_, err2 := os.Stat(md5Filepath)
	// 两个文件都不存在，返回空，不报错
	if err1 != nil && os.IsNotExist(err1) && err2 != nil && os.IsNotExist(err2) {
		return bytes, nil
	}

	if err1 != nil {
		return bytes, err1
	}
	if err2 != nil {
		return bytes, err2
	}

	// 检验md5
	md5File, err := os.Open(md5Filepath)
	if err != nil {
		return
	}
	defer md5File.Close()

	code, err := ioutil.ReadAll(md5File)
	if err != nil {
		return
	}
	ok := md5Check(filepath, string(code))
	if !ok {
		return bytes, errors.New(fmt.Sprintf("md5 check failed, filepath: %s, md5 in file: %s", filepath, string(code)))
	}

	// 读取文件
	file, err := os.Open(filepath)
	if err != nil {
		return bytes, err
	}
	return ioutil.ReadAll(file)
}

func getElemFromJobKey(jobKey string) (productKey, deviceName, jobID string) {
	splitRes := strings.Split(jobKey, "|")
	if len(splitRes) < 3 {
		return
	}
	return splitRes[0], splitRes[1], splitRes[2]
}

func CheckDeviceName(name string) bool {
	match, _ := regexp.MatchString("^[a-zA-Z0-9_\\-.:@]{1,128}$", name)
	return match
}

func (d *Device) sendHTTPPostRequest(url string, headers map[string]string, param interface{}) ([]byte, error) {
	paramBytes, err := json.Marshal(param)
	if err != nil {
		return nil, fmt.Errorf("marshal para failed,err: %s", err)
	}
	request, err := http.NewRequest("POST", url, bytes.NewBuffer(paramBytes))
	if err != nil {
		return nil, fmt.Errorf("new request failed,err: %s", err.Error())
	}
	request.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	resp, err := d.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("do request failed,err: %s", err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode > 500 {
		return nil, fmt.Errorf("can't process, response is %v", resp.StatusCode)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read resp body failed,err: %s", err.Error())
	}

	return body, nil
}
