package handlers

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/hoshinonyaruko/gensokyo/callapi"
	"github.com/hoshinonyaruko/gensokyo/config"
	"github.com/hoshinonyaruko/gensokyo/echo"
	"github.com/hoshinonyaruko/gensokyo/idmap"
	"github.com/hoshinonyaruko/gensokyo/mylog"
	"github.com/tencent-connect/botgo/dto"
	"github.com/tencent-connect/botgo/openapi"
)

func init() {
	callapi.RegisterHandler("upload_private_file", HandleUploadPrivateFile)
}

func HandleUploadPrivateFile(client callapi.Client, api openapi.OpenAPI, apiv2 openapi.OpenAPI, message callapi.ActionMessage) (string, error) {
	ctx := context.Background()

	userID, ok := message.Params.UserID.(string)
	if !ok || userID == "" {
		return marshalUploadError("user_id is required", 40001), nil
	}
	fileURL := message.Params.URL
	if fileURL == "" {
		return marshalUploadError("url is required", 40002), nil
	}
	fileName := message.Params.FileName
	if fileName == "" {
		fileName = "file"
	}
	fileType := 4
	if ft, ok := message.Params.FileType.(int); ok && ft > 0 {
		fileType = ft
	}

	// --- msg_id / event_id ---
	var messageID string
	var eventID string

	// 优先使用调用方显式传入的 msg_id
	if mid, ok := message.Params.MessageID.(string); ok && mid != "" && mid != "0" {
		messageID = mid
	}

	if message.Params.EventID != nil {
		if eid, ok := message.Params.EventID.(string); ok && eid != "" {
			eventID = eid
		}
	}

	// lazy_message_id 回退链
	if messageID == "" && config.GetLazyMessageId() {
		messageID = echo.GetLazyMessagesId(userID)
	}
	if messageID == "" {
		if echoStr, ok := message.Echo.(string); ok {
			messageID = echo.GetMsgIDByKey(echoStr)
		}
	}
	if messageID == "" {
		messageID = GetMessageIDByUseridOrGroupid(config.GetAppIDStr(), message.Params.UserID)
	}
	if messageID == "2000" {
		messageID = ""
		mylog.Println("upload_private_file: lazy 模式返回 2000，尝试获取 eventID")
		if eventID == "" {
			if len(userID) != 32 {
				eventID = GetEventIDByUseridOrGroupid(config.GetAppIDStr(), message.Params.UserID)
			} else {
				eventID = GetEventIDByUseridOrGroupidv2(config.GetAppIDStr(), message.Params.UserID)
			}
		}
	}
	if eventID == "" {
		if len(userID) != 32 {
			eventID = GetEventIDByUseridOrGroupid(config.GetAppIDStr(), message.Params.UserID)
		} else {
			eventID = GetEventIDByUseridOrGroupidv2(config.GetAppIDStr(), message.Params.UserID)
		}
	}

	// --- idmap 转换 ---
	originalUserID := userID
	if len(userID) != 32 {
		resolved, err := idmap.RetrieveRowByIDv2(userID)
		if err == nil && resolved != "" {
			originalUserID = resolved
		}
	}

	// --- 下载文件到临时文件并计算哈希 ---
	httpClient := &http.Client{Timeout: 10 * time.Minute}
	tmpPath, fileSize, md5Hex, sha1Hex, md510mHex, err := downloadAndComputeHashes(fileURL, httpClient)
	if err != nil {
		return marshalUploadError(fmt.Sprintf("download/hash failed: %v", err), 40010), nil
	}
	defer os.Remove(tmpPath)

	mylog.Printf("upload_private_file: user=%s file=%s size=%d type=%d md5=%s", originalUserID, fileName, fileSize, fileType, md5Hex)

	// --- 缓存检查：相同文件+目标+类型 在 TTL 内可直接复用 file_info ---
	if cachedInfo, cachedUUID, hit := GetCachedFileInfo(md5Hex, "c2c", originalUserID, fileType); hit {
		mylog.Printf("upload_private_file: cache HIT for md5=%s, file_uuid=%s, skipping upload", md5Hex, cachedUUID)
		return buildUploadResponse(client, api, apiv2, ctx, cachedInfo, cachedUUID, 0,
			originalUserID, "", messageID, eventID, fileName, fileType, message, false,
			md5Hex, "c2c")
	}

	// --- Step 1: upload_prepare（带重试，5xx 级别瞬时错误可恢复） ---
	prepReq := &dto.UploadPrepareRequest{
		FileType: fileType,
		FileName: fileName,
		FileSize: fileSize,
		MD5:      md5Hex,
		SHA1:     sha1Hex,
		MD510M:   md510mHex,
	}
	prepResp, err := uploadPrepareWithRetry(ctx, func(ctx context.Context) (*dto.UploadPrepareResponse, error) {
		return apiv2.C2CUploadPrepare(ctx, originalUserID, prepReq)
	})
	if err != nil {
		if extractBizCode(err) == uploadPrepareDailyLimitCode {
			return marshalUploadError("QQ 平台每日累计上传文件已达上限（2GB），请明天再试", 40093), nil
		}
		return marshalUploadError(fmt.Sprintf("upload_prepare failed: %v", err), 50001), nil
	}

	blockSize := prepResp.BlockSize
	if blockSize <= 0 {
		blockSize = 5 * 1024 * 1024
	}

	mylog.Printf("upload_prepare ok: upload_id=%s block_size=%d parts=%d concurrency=%d retry_timeout=%d",
		prepResp.UploadID, blockSize, len(prepResp.Parts), prepResp.Concurrency, prepResp.RetryTimeout)

	// Parts 为空 = 秒传（文件已在服务端），跳过分片直接完成
	if len(prepResp.Parts) > 0 {
		// --- Step 2 & 3: 并发分片上传 ---
		tmpFile, err := os.Open(tmpPath)
		if err != nil {
			return marshalUploadError(fmt.Sprintf("open temp file failed: %v", err), 40013), nil
		}
		defer tmpFile.Close()

		maxConcurrent := prepResp.Concurrency
		if maxConcurrent <= 0 {
			maxConcurrent = 1
		}
		if maxConcurrent > 10 {
			maxConcurrent = 10
		}

		var retryTimeoutMs int64
		if prepResp.RetryTimeout > 0 {
			retryTimeoutMs = int64(prepResp.RetryTimeout) * 1000
			if retryTimeoutMs > maxPartFinishRetryTimeoutMs {
				retryTimeoutMs = maxPartFinishRetryTimeoutMs
			}
		}

		pfCaller := func(ctx context.Context, req *dto.UploadPartFinishRequest) (*dto.UploadPartFinishResponse, error) {
			return apiv2.C2CUploadPartFinish(ctx, originalUserID, req)
		}

		if err := uploadPartsWithConcurrency(ctx, httpClient, tmpFile, prepResp, blockSize, fileSize, maxConcurrent, retryTimeoutMs, pfCaller); err != nil {
			return marshalUploadError(err.Error(), 50002), nil
		}
	} else {
		mylog.Printf("upload_private_file: 0 parts returned (fast upload / 秒传), skipping to complete")
	}

	// --- Step 4: 合并完成（带重试） ---
	postFileReq := &dto.PostFileRequest{
		UploadID: prepResp.UploadID,
	}
	var mediaResp *dto.MediaResponse
	for attempt := 0; attempt <= 2; attempt++ {
		mediaResp, err = apiv2.C2CPostFile(ctx, originalUserID, postFileReq)
		if err == nil {
			break
		}
		if attempt < 2 {
			delay := time.Duration(2*(attempt+1)) * time.Second
			mylog.Printf("complete upload attempt %d failed, retrying in %v: %v", attempt+1, delay, err)
			time.Sleep(delay)
		}
	}
	if err != nil {
		return marshalUploadError(fmt.Sprintf("post_file (merge) failed: %v", err), 50004), nil
	}

	mylog.Printf("upload_private_file complete: file_uuid=%s ttl=%d", mediaResp.FileUUID, mediaResp.TTL)

	return buildUploadResponse(client, api, apiv2, ctx, mediaResp.FileInfo, mediaResp.FileUUID, mediaResp.TTL,
		originalUserID, "", messageID, eventID, fileName, fileType, message, false,
		md5Hex, "c2c")
}
