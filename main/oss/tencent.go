package oss

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sync"

	"github.com/hoshinonyaruko/gensokyo/config"
	"github.com/hoshinonyaruko/gensokyo/mylog"
	"github.com/tencentyun/cos-go-sdk-v5"
)

var (
	once   sync.Once
	client *cos.Client
)

// 初始化oss单例
func initClient() {
	once.Do(func() {
		bucketURL, _ := url.Parse(config.GetTencentBucketURL())
		b := &cos.BaseURL{BucketURL: bucketURL}
		c := cos.NewClient(b, &http.Client{
			Transport: &cos.AuthorizationTransport{
				SecretID:  config.GetTencentCosSecretid(),
				SecretKey: config.GetTencentSecretKey(),
			},
		})
		client = c
	})
}

// 上传并审核
func UploadAndAuditImage(base64Data string) (string, error) {
	initClient()

	// Decode base64 data
	decodedData, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return "", err
	}

	// Create a temporary file to save decoded data
	tmpFile, err := os.CreateTemp("", "upload-*.jpg")
	if err != nil {
		return "", err
	}
	defer func() {
		tmpFile.Close()
		os.Remove(tmpFile.Name()) // 清理临时文件
	}()

	if _, err = tmpFile.Write(decodedData); err != nil {
		return "", err
	}

	// 计算解码数据的 MD5
	h := md5.New()
	if _, err := h.Write(decodedData); err != nil {
		return "", err
	}
	md5Hash := fmt.Sprintf("%x", h.Sum(nil))

	// 使用 MD5 值作为对象键
	objectKey := md5Hash + ".jpg"

	// 上传文件到 COS
	_, err = client.Object.PutFromFile(context.Background(), objectKey, tmpFile.Name(), nil)
	if err != nil {
		return "", err
	}

	if config.GetTencentAudit() {
		// Call ImageRecognition
		res, _, err := client.CI.ImageRecognition(context.Background(), objectKey, "")
		if err != nil {
			return "", err
		}

		// 检查图片审核结果：仅在不正常时输出完整识别信息
		if res.Result != 0 {
			mylog.Printf("Image not normal. Result code: %d, Details: %+v", res.Result, res)
			return "", nil
		}
	}

	// 图片正常，返回图片 URL
	bucketURL, err := url.Parse(config.GetTencentBucketURL())
	if err != nil {
		return "", fmt.Errorf("解析存储桶URL失败: %w", err)
	}
	imageURL := bucketURL.String() + "/" + objectKey

	// 验证生成的URL是否可访问 (可选的验证步骤)
	if config.GetTencentAudit() { // 复用审核配置来决定是否启用URL验证，仅在验证异常时输出日志
		resp, err := http.Head(imageURL)
		if err != nil {
			mylog.Printf("警告: 图片URL验证失败: %v, URL: %s", err, imageURL)
		} else {
			resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				mylog.Printf("警告: 图片URL返回状态码: %d, URL: %s", resp.StatusCode, imageURL)
			}
		}
	}

	return imageURL, nil
}

// 上传语音
func UploadAndAuditRecord(base64Data string) (string, error) {
	initClient()

	// Decode base64 data
	decodedData, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return "", err
	}

	// Create a temporary file to save decoded data
	tmpFile, err := os.CreateTemp("", "upload-*.amr")
	if err != nil {
		return "", err
	}
	defer func() {
		tmpFile.Close()
		os.Remove(tmpFile.Name()) // 清理临时文件
	}()

	if _, err = tmpFile.Write(decodedData); err != nil {
		return "", err
	}

	// 计算解码数据的 MD5
	h := md5.New()
	if _, err := h.Write(decodedData); err != nil {
		return "", err
	}
	md5Hash := fmt.Sprintf("%x", h.Sum(nil))

	// 使用 MD5 值作为对象键
	objectKey := md5Hash + ".amr"

	// 上传文件到 COS
	_, err = client.Object.PutFromFile(context.Background(), objectKey, tmpFile.Name(), nil)
	if err != nil {
		return "", err
	}

	// 语音正常，返回语音 URL
	bucketURL, err := url.Parse(config.GetTencentBucketURL())
	if err != nil {
		return "", fmt.Errorf("解析存储桶URL失败: %w", err)
	}
	recordURL := bucketURL.String() + "/" + objectKey
	return recordURL, nil
}
