package dto

import (
	"encoding/json"
	"strconv"
)

// MD510MSize 计算 md5_10m 使用的前缀字节数（与 QQ 开放平台协议一致）
const MD510MSize = 10002432

// UploadPrepareRequest 分片上传 - 创建上传任务请求
type UploadPrepareRequest struct {
	FileType int    `json:"file_type"`
	FileName string `json:"file_name"`
	FileSize int64  `json:"file_size"`
	MD5      string `json:"md5"`
	SHA1     string `json:"sha1"`
	MD510M   string `json:"md5_10m"`
}

// UploadPart 分片信息（服务端返回，含 1-based 索引和预签名链接）
type UploadPart struct {
	Index        int    `json:"index"`
	PresignedURL string `json:"presigned_url"`
}

// UploadPrepareResponse 分片上传 - 创建上传任务响应
type UploadPrepareResponse struct {
	UploadID     string       `json:"upload_id"`
	BlockSize    int64        `json:"block_size"`
	Parts        []UploadPart `json:"parts"`
	Concurrency  int          `json:"concurrency,omitempty"`
	RetryTimeout int          `json:"retry_timeout,omitempty"`
}

// UnmarshalJSON QQ 开放平台返回的 block_size / concurrency / retry_timeout 可能是字符串
func (r *UploadPrepareResponse) UnmarshalJSON(data []byte) error {
	type Alias UploadPrepareResponse
	aux := &struct {
		BlockSize    interface{} `json:"block_size"`
		Concurrency  interface{} `json:"concurrency"`
		RetryTimeout interface{} `json:"retry_timeout"`
		*Alias
	}{
		Alias: (*Alias)(r),
	}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	r.BlockSize = flexInt64(aux.BlockSize)
	r.Concurrency = int(flexInt64(aux.Concurrency))
	r.RetryTimeout = int(flexInt64(aux.RetryTimeout))
	return nil
}

func flexInt64(v interface{}) int64 {
	switch val := v.(type) {
	case float64:
		return int64(val)
	case string:
		if n, err := strconv.ParseInt(val, 10, 64); err == nil {
			return n
		}
	}
	return 0
}

// UploadPartFinishRequest 分片上传 - 上报分片完成请求
type UploadPartFinishRequest struct {
	UploadID  string `json:"upload_id"`
	PartIndex int    `json:"part_index"`
	BlockSize int64  `json:"block_size"`
	MD5       string `json:"md5"`
}

// UploadPartFinishResponse 分片上传 - 上报分片完成响应
type UploadPartFinishResponse struct {
	Status int    `json:"status,omitempty"`
	Msg    string `json:"msg,omitempty"`
}

// PostFileRequest 分片上传 - 合并完成上传请求
type PostFileRequest struct {
	UploadID string `json:"upload_id"`
}
