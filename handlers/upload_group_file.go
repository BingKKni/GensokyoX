package handlers

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"net/http"
	"os"
	"strconv"
	"sync"
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
	callapi.RegisterHandler("upload_group_file", HandleUploadGroupFile)
}

func HandleUploadGroupFile(client callapi.Client, api openapi.OpenAPI, apiv2 openapi.OpenAPI, message callapi.ActionMessage) (string, error) {
	ctx := context.Background()

	groupID, ok := message.Params.GroupID.(string)
	if !ok || groupID == "" {
		return marshalUploadError("group_id is required", 40001), nil
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
		messageID = echo.GetLazyMessagesId(groupID)
		if message.Params.UserID != nil {
			uid, _ := message.Params.UserID.(string)
			if uid != "" && uid != "0" {
				messageID = echo.GetLazyMessagesIdv2(groupID, uid)
			}
		}
	}
	if messageID == "" {
		if echoStr, ok := message.Echo.(string); ok {
			messageID = echo.GetMsgIDByKey(echoStr)
		}
	}
	if messageID == "" {
		if message.Params.UserID != nil {
			uid, _ := message.Params.UserID.(string)
			if uid != "" && uid != "0" {
				messageID = GetMessageIDByUseridAndGroupid(config.GetAppIDStr(), message.Params.UserID, message.Params.GroupID)
			}
		}
		if messageID == "" {
			messageID = GetMessageIDByUseridOrGroupid(config.GetAppIDStr(), message.Params.GroupID)
		}
	}
	if messageID == "2000" {
		messageID = ""
		mylog.Println("upload_group_file: lazy 模式返回 2000，尝试获取 eventID")
		if eventID == "" {
			eventID = GetEventIDByUseridOrGroupid(config.GetAppIDStr(), message.Params.GroupID)
		}
	}
	if eventID == "" {
		eventID = GetEventIDByUseridOrGroupid(config.GetAppIDStr(), message.Params.GroupID)
	}

	// --- idmap 转换 ---
	originalGroupID := groupID
	if len(groupID) != 32 {
		if message.Params.UserID != nil && config.GetIdmapPro() {
			uid, _ := message.Params.UserID.(string)
			if uid != "" && uid != "0" {
				resolved, _, err := idmap.RetrieveRowByIDv2Pro(groupID, uid)
				if err == nil && resolved != "" {
					originalGroupID = resolved
				}
			}
		}
		if originalGroupID == groupID {
			resolved, err := idmap.RetrieveRowByIDv2(groupID)
			if err == nil && resolved != "" {
				originalGroupID = resolved
			}
		}
	}

	// --- 下载文件到临时文件并计算哈希 ---
	httpClient := &http.Client{Timeout: 10 * time.Minute}
	tmpPath, fileSize, md5Hex, sha1Hex, md510mHex, err := downloadAndComputeHashes(fileURL, httpClient)
	if err != nil {
		return marshalUploadError(fmt.Sprintf("download/hash failed: %v", err), 40010), nil
	}
	defer os.Remove(tmpPath)

	mylog.Printf("upload_group_file: group=%s file=%s size=%d type=%d md5=%s", originalGroupID, fileName, fileSize, fileType, md5Hex)

	// --- 缓存检查：相同文件+目标+类型 在 TTL 内可直接复用 file_info ---
	if cachedInfo, cachedUUID, hit := GetCachedFileInfo(md5Hex, "group", originalGroupID, fileType); hit {
		mylog.Printf("upload_group_file: cache HIT for md5=%s, file_uuid=%s, skipping upload", md5Hex, cachedUUID)
		return buildUploadResponse(client, api, apiv2, ctx, cachedInfo, cachedUUID, 0,
			originalGroupID, "", messageID, eventID, fileName, fileType, message, true,
			md5Hex, "group")
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
		return apiv2.GroupUploadPrepare(ctx, originalGroupID, prepReq)
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

		// 并发数由 API 返回值控制，默认 1，上限 10
		maxConcurrent := prepResp.Concurrency
		if maxConcurrent <= 0 {
			maxConcurrent = 1
		}
		if maxConcurrent > 10 {
			maxConcurrent = 10
		}

		// retry_timeout 用于 part_finish 40093001 持续重试
		var retryTimeoutMs int64
		if prepResp.RetryTimeout > 0 {
			retryTimeoutMs = int64(prepResp.RetryTimeout) * 1000
			if retryTimeoutMs > maxPartFinishRetryTimeoutMs {
				retryTimeoutMs = maxPartFinishRetryTimeoutMs
			}
		}

		pfCaller := func(ctx context.Context, req *dto.UploadPartFinishRequest) (*dto.UploadPartFinishResponse, error) {
			return apiv2.GroupUploadPartFinish(ctx, originalGroupID, req)
		}

		if err := uploadPartsWithConcurrency(ctx, httpClient, tmpFile, prepResp, blockSize, fileSize, maxConcurrent, retryTimeoutMs, pfCaller); err != nil {
			return marshalUploadError(err.Error(), 50002), nil
		}
	} else {
		mylog.Printf("upload_group_file: 0 parts returned (fast upload / 秒传), skipping to complete")
	}

	// --- Step 4: 合并完成（带重试） ---
	postFileReq := &dto.PostFileRequest{
		UploadID: prepResp.UploadID,
	}
	var mediaResp *dto.MediaResponse
	for attempt := 0; attempt <= 2; attempt++ {
		mediaResp, err = apiv2.GroupPostFile(ctx, originalGroupID, postFileReq)
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

	mylog.Printf("upload_group_file complete: file_uuid=%s ttl=%d", mediaResp.FileUUID, mediaResp.TTL)

	return buildUploadResponse(client, api, apiv2, ctx, mediaResp.FileInfo, mediaResp.FileUUID, mediaResp.TTL,
		originalGroupID, "", messageID, eventID, fileName, fileType, message, true,
		md5Hex, "group")
}

func marshalUploadError(msg string, code int) string {
	resp := ServerResponse{
		Message:   msg,
		ErrorCode: code,
	}
	b, _ := json.Marshal(structToMap(resp))
	return string(b)
}

// buildUploadResponse 构造上传响应（群和 C2C 共用）。
// isGroup=true 时用 GroupID + PostGroupMessage，否则用 UserID + PostC2CMessage。
// contentMD5 和 cacheScope 用于控制缓存：发送成功或无需发送时写入缓存，发送失败时清除缓存。
func buildUploadResponse(
	client callapi.Client,
	api openapi.OpenAPI, apiv2 openapi.OpenAPI, ctx context.Context,
	fileInfo, fileUUID string, ttl int,
	targetID string, userID string,
	messageID, eventID, fileName string, fileType int,
	message callapi.ActionMessage, isGroup bool,
	contentMD5, cacheScope string,
) (string, error) {
	response := ServerResponse{
		Message: "",
		TraceID: api.TraceID(),
	}
	response.Data.FileUUID = fileUUID
	response.Data.FileInfo = fileInfo
	response.Data.TTL = ttl

	if isGroup {
		if config.GetIdmapPro() && message.Params.UserID != nil {
			uid, _ := message.Params.UserID.(string)
			gid64, _, _ := idmap.StoreIDv2Pro(targetID, uid)
			response.GroupID = gid64
		} else {
			gid64, _ := idmap.StoreIDv2(targetID)
			response.GroupID = gid64
		}
	} else {
		uid64, _ := idmap.StoreIDv2(targetID)
		response.UserID = uid64
	}

	sendFailed := false

	if messageID != "" {
		msgseq := echo.GetMappingSeq(messageID)
		echo.AddMappingSeq(messageID, msgseq+1)
		msg := &dto.MessageToCreate{
			Content: fileName,
			Media: dto.Media{
				FileInfo: fileInfo,
			},
			MsgID:   messageID,
			EventID: eventID,
			MsgType: 7,
			MsgSeq:  msgseq + 1,
		}

		if isGroup {
			msgResp, sendErr := apiv2.PostGroupMessage(ctx, targetID, msg)
			if sendErr != nil {
				mylog.Printf("upload: file uploaded but send_msg failed: %v", sendErr)
				response.Message = fmt.Sprintf("file uploaded but send failed: %v", sendErr)
				sendFailed = true
			} else if msgResp != nil && msgResp.Message != nil {
				mid64, _ := strconv.ParseInt(msgResp.Message.ID, 10, 64)
				response.Data.MessageID = int(mid64)
				response.Data.RealMessageID = msgResp.Message.ID
			}
		} else {
			msgResp, sendErr := apiv2.PostC2CMessage(ctx, targetID, msg)
			if sendErr != nil {
				mylog.Printf("upload: file uploaded but send_msg failed: %v", sendErr)
				response.Message = fmt.Sprintf("file uploaded but send failed: %v", sendErr)
				sendFailed = true
			} else if msgResp != nil && msgResp.Message != nil {
				mid64, _ := strconv.ParseInt(msgResp.Message.ID, 10, 64)
				response.Data.MessageID = int(mid64)
				response.Data.RealMessageID = msgResp.Message.ID
			}
		}
	}

	// 缓存策略：发送成功或无需发送时写入缓存；发送失败时清除已有缓存
	if contentMD5 != "" {
		if sendFailed {
			InvalidateCachedFileInfo(contentMD5, cacheScope, targetID, fileType)
		} else if ttl > 0 {
			SetCachedFileInfo(contentMD5, cacheScope, targetID, fileType, fileInfo, fileUUID, ttl)
		}
	}

	outputMap := structToMap(response)
	jsonResponse, jsonErr := json.Marshal(outputMap)
	if jsonErr != nil {
		return "", jsonErr
	}
	_ = client.SendMessage(outputMap)
	return string(jsonResponse), nil
}

// 分片上传常量

const (
	partFinishRetryableCode       = 40093001 // part_finish 需要持续重试的业务错误码
	uploadPrepareDailyLimitCode   = 40093002 // upload_prepare 日累计上传限制错误码
	maxPartFinishRetryTimeoutMs   = 10 * 60 * 1000
	defaultPartFinishRetryTimeout = 2 * 60 * 1000
	partFinishRetryInterval       = 1 * time.Second
	partFinishMaxNormalRetries    = 2
	uploadPrepareMaxRetries       = 2
	uploadPrepareBaseDelay        = 1 * time.Second
)

// 分片上传公共辅助函数（群 + C2C 共用）

// uploadPrepareWithRetry 对 upload_prepare 做重试：
// 4xx 错误（客户端错误/业务限制）不重试，5xx 错误（平台瞬时故障如 850012）最多重试 2 次。
func uploadPrepareWithRetry(ctx context.Context, caller func(ctx context.Context) (*dto.UploadPrepareResponse, error)) (*dto.UploadPrepareResponse, error) {
	var lastErr error
	for attempt := 0; attempt <= uploadPrepareMaxRetries; attempt++ {
		resp, err := caller(ctx)
		if err == nil {
			return resp, nil
		}
		lastErr = err

		bizCode := extractBizCode(err)
		if bizCode == uploadPrepareDailyLimitCode {
			return nil, err
		}

		httpCode := extractHTTPCode(err)
		if httpCode >= 400 && httpCode < 500 {
			return nil, err
		}

		if attempt < uploadPrepareMaxRetries {
			delay := uploadPrepareBaseDelay * time.Duration(1<<uint(attempt))
			mylog.Printf("upload_prepare attempt %d failed (httpCode=%d, bizCode=%d), retrying in %v: %v", attempt+1, httpCode, bizCode, delay, err)
			time.Sleep(delay)
		}
	}
	return nil, lastErr
}

// extractHTTPCode 从 botgo SDK 错误中提取 HTTP 状态码
func extractHTTPCode(err error) int {
	if err == nil {
		return 0
	}
	type errWithCode interface {
		Code() int
	}
	if e, ok := err.(errWithCode); ok {
		return e.Code()
	}
	return 0
}

type partFinishCaller func(ctx context.Context, req *dto.UploadPartFinishRequest) (*dto.UploadPartFinishResponse, error)

// extractBizCode 从 botgo SDK 返回的错误中提取 QQ 平台业务错误码
func extractBizCode(err error) int {
	if err == nil {
		return 0
	}
	type errWithText interface {
		Text() string
	}
	e, ok := err.(errWithText)
	if !ok {
		return 0
	}
	var resp struct {
		Code    int `json:"code"`
		ErrCode int `json:"err_code"`
	}
	if json.Unmarshal([]byte(e.Text()), &resp) == nil {
		if resp.Code != 0 {
			return resp.Code
		}
		return resp.ErrCode
	}
	return 0
}

// partFinishWithSmartRetry 实现 OpenClaw 的 part_finish 重试策略：
//   - 40093001: 以 1s 间隔持续重试直到超时
//   - 其他错误: 最多 2 次指数退避重试
func partFinishWithSmartRetry(ctx context.Context, pfCaller partFinishCaller, req *dto.UploadPartFinishRequest, retryTimeoutMs int64) error {
	var lastErr error

	for attempt := 0; attempt <= partFinishMaxNormalRetries; attempt++ {
		_, lastErr = pfCaller(ctx, req)
		if lastErr == nil {
			return nil
		}

		bizCode := extractBizCode(lastErr)
		if bizCode == partFinishRetryableCode {
			timeoutMs := retryTimeoutMs
			if timeoutMs <= 0 {
				timeoutMs = defaultPartFinishRetryTimeout
			}
			mylog.Printf("part_finish[%d] hit retryable code %d, persistent retry (timeout=%ds)", req.PartIndex, bizCode, timeoutMs/1000)
			return partFinishPersistentRetry(ctx, pfCaller, req, timeoutMs)
		}

		if attempt < partFinishMaxNormalRetries {
			delay := time.Duration(1<<uint(attempt)) * time.Second
			mylog.Printf("part_finish[%d] attempt %d failed (bizCode=%d), retrying in %v: %v", req.PartIndex, attempt+1, bizCode, delay, lastErr)
			time.Sleep(delay)
		}
	}
	return lastErr
}

func partFinishPersistentRetry(ctx context.Context, pfCaller partFinishCaller, req *dto.UploadPartFinishRequest, timeoutMs int64) error {
	deadline := time.Now().Add(time.Duration(timeoutMs) * time.Millisecond)
	var attempt int
	var lastErr error

	for time.Now().Before(deadline) {
		_, lastErr = pfCaller(ctx, req)
		if lastErr == nil {
			mylog.Printf("part_finish[%d] persistent retry succeeded after %d retries", req.PartIndex, attempt)
			return nil
		}

		bizCode := extractBizCode(lastErr)
		if bizCode != partFinishRetryableCode {
			mylog.Printf("part_finish[%d] persistent retry: error no longer retryable (bizCode=%d), aborting", req.PartIndex, bizCode)
			return lastErr
		}

		attempt++
		remaining := time.Until(deadline)
		if remaining <= 0 {
			break
		}
		sleepDur := partFinishRetryInterval
		if remaining < sleepDur {
			sleepDur = remaining
		}
		time.Sleep(sleepDur)
	}
	return fmt.Errorf("part_finish[%d] persistent retry timed out (%ds, %d attempts): %v", req.PartIndex, timeoutMs/1000, attempt, lastErr)
}

// uploadPartsWithConcurrency 批次并发上传分片
func uploadPartsWithConcurrency(
	ctx context.Context,
	httpClient *http.Client,
	tmpFile *os.File,
	prepResp *dto.UploadPrepareResponse,
	blockSize int64,
	fileSize int64,
	maxConcurrent int,
	retryTimeoutMs int64,
	pfCaller partFinishCaller,
) error {
	parts := prepResp.Parts
	totalParts := len(parts)

	for batchStart := 0; batchStart < totalParts; batchStart += maxConcurrent {
		batchEnd := batchStart + maxConcurrent
		if batchEnd > totalParts {
			batchEnd = totalParts
		}
		batch := parts[batchStart:batchEnd]

		partErrors := make([]error, len(batch))
		var wg sync.WaitGroup

		for j, part := range batch {
			wg.Add(1)
			go func(idx int, p dto.UploadPart) {
				defer wg.Done()
				partErrors[idx] = uploadOnePart(ctx, httpClient, tmpFile, prepResp.UploadID, p, blockSize, fileSize, retryTimeoutMs, pfCaller)
			}(j, part)
		}
		wg.Wait()

		for _, e := range partErrors {
			if e != nil {
				return e
			}
		}
		mylog.Printf("upload batch %d-%d/%d complete", batchStart+1, batchEnd, totalParts)
	}
	return nil
}

func uploadOnePart(
	ctx context.Context,
	httpClient *http.Client,
	tmpFile *os.File,
	uploadID string,
	part dto.UploadPart,
	blockSize int64,
	fileSize int64,
	retryTimeoutMs int64,
	pfCaller partFinishCaller,
) error {
	offset := int64(part.Index-1) * blockSize
	length := blockSize
	if offset+length > fileSize {
		length = fileSize - offset
	}

	chunk := make([]byte, length)
	n, readErr := tmpFile.ReadAt(chunk, offset)
	if readErr != nil && readErr != io.EOF {
		return fmt.Errorf("chunk %d: read failed: %v", part.Index, readErr)
	}
	chunk = chunk[:n]

	chunkMD5 := computeMD5Hex(chunk)

	if err := putChunkWithRetry(ctx, httpClient, part.PresignedURL, chunk, 2); err != nil {
		return fmt.Errorf("chunk %d: PUT failed: %v", part.Index, err)
	}

	partFinishReq := &dto.UploadPartFinishRequest{
		UploadID:  uploadID,
		PartIndex: part.Index,
		BlockSize: int64(n),
		MD5:       chunkMD5,
	}
	if err := partFinishWithSmartRetry(ctx, pfCaller, partFinishReq, retryTimeoutMs); err != nil {
		return fmt.Errorf("chunk %d: part_finish failed: %v", part.Index, err)
	}

	mylog.Printf("chunk %d uploaded (%d bytes)", part.Index, n)
	return nil
}

// downloadAndComputeHashes 下载 URL 到临时文件，同时单次遍历计算 md5、sha1、md5_10m
func downloadAndComputeHashes(fileURL string, httpClient *http.Client) (tmpPath string, fileSize int64, md5Hex, sha1Hex, md510mHex string, err error) {
	resp, err := httpClient.Get(fileURL)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("HTTP %d %s", resp.StatusCode, resp.Status)
		return
	}

	tmpFile, err := os.CreateTemp("", "gsk-upload-*")
	if err != nil {
		return
	}
	tmpPath = tmpFile.Name()

	md5Hash := md5.New()
	sha1Hash := sha1.New()
	var md510mHash hash.Hash = md5.New()

	multiWriter := io.MultiWriter(tmpFile, md5Hash, sha1Hash)

	var bytesWritten int64
	buf := make([]byte, 64*1024)
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, wErr := multiWriter.Write(buf[:n]); wErr != nil {
				tmpFile.Close()
				os.Remove(tmpPath)
				err = wErr
				return
			}
			if bytesWritten < dto.MD510MSize {
				remaining := dto.MD510MSize - bytesWritten
				if int64(n) <= remaining {
					md510mHash.Write(buf[:n])
				} else {
					md510mHash.Write(buf[:remaining])
				}
			}
			bytesWritten += int64(n)
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			tmpFile.Close()
			os.Remove(tmpPath)
			err = readErr
			return
		}
	}

	tmpFile.Close()
	fileSize = bytesWritten

	md5Hex = fmt.Sprintf("%x", md5Hash.Sum(nil))
	sha1Hex = fmt.Sprintf("%x", sha1Hash.Sum(nil))
	if bytesWritten <= dto.MD510MSize {
		md510mHex = md5Hex
	} else {
		md510mHex = fmt.Sprintf("%x", md510mHash.Sum(nil))
	}
	return
}

// computeMD5Hex 计算数据的 MD5 十六进制字符串
func computeMD5Hex(data []byte) string {
	h := md5.Sum(data)
	return fmt.Sprintf("%x", h)
}

// putChunkWithRetry PUT 分片数据到预签名 URL，失败后指数退避重试
func putChunkWithRetry(ctx context.Context, httpClient *http.Client, presignedURL string, data []byte, maxRetries int) error {
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, "PUT", presignedURL, bytes.NewReader(data))
		if err != nil {
			return fmt.Errorf("create PUT request failed: %w", err)
		}
		req.Header.Set("Content-Type", "application/octet-stream")
		req.ContentLength = int64(len(data))

		resp, err := httpClient.Do(req)
		if err != nil {
			lastErr = err
			if attempt < maxRetries {
				time.Sleep(time.Duration(1<<uint(attempt)) * time.Second)
				continue
			}
			return lastErr
		}
		resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}
		lastErr = fmt.Errorf("COS PUT returned %d %s", resp.StatusCode, resp.Status)
		if attempt < maxRetries {
			time.Sleep(time.Duration(1<<uint(attempt)) * time.Second)
		}
	}
	return lastErr
}
