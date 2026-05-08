package server

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hoshinonyaruko/gensokyo/config"
	"github.com/hoshinonyaruko/gensokyo/interactionwait"
	"github.com/hoshinonyaruko/gensokyo/mylog"
	"github.com/tencent-connect/botgo/dto"
	"github.com/tencent-connect/botgo/event"
	"github.com/tencent-connect/botgo/openapi"
	"github.com/tencent-connect/botgo/websocket/client"
)

// Payload 定义请求载荷结构
type Payload struct {
	D  ValidationRequest `json:"d"`
	Op int               `json:"op"`
}

// ValidationRequest 定义鉴权请求结构
type ValidationRequest struct {
	PlainToken string `json:"plain_token"`
	EventTs    string `json:"event_ts"`
}

// WebhookPayload 定义Webhook消息结构
type WebhookPayload struct {
	PlainToken string `json:"plain_token"`
	EventTs    string `json:"event_ts"`
	RawMessage []byte // 保存原始消息内容
}

// WebhookHandler 负责处理 Webhook 的接收和消息处理
type WebhookHandler struct {
	messageQueue chan *WebhookPayload
	closeChan    chan error
}

// NewWebhookHandler 创建新的 WebhookHandler 实例
func NewWebhookHandler(queueSize int) *WebhookHandler {
	return &WebhookHandler{
		messageQueue: make(chan *WebhookPayload, queueSize),
		closeChan:    make(chan error),
	}
}

// 在启动时生成私钥
var privateKey ed25519.PrivateKey
var publicKey ed25519.PublicKey

func InitPrivateKey(botSecret string) {
	seed := botSecret
	for len(seed) < ed25519.SeedSize {
		seed = strings.Repeat(seed, 2)
	}
	seed = seed[:ed25519.SeedSize]
	reader := strings.NewReader(seed)

	pkey, key, err := ed25519.GenerateKey(reader)
	if err != nil {
		log.Fatalf("Failed to generate ed25519 private key: %v", err)
	}
	privateKey = key
	publicKey = pkey
}

func CreateHandleValidationSafe(wh *WebhookHandler, apiV2 openapi.OpenAPI) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 读取 HTTP Body
		httpBody, err := io.ReadAll(c.Request.Body)
		if err != nil {
			log.Println("Failed to read HTTP body:", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
			return
		}

		// 恢复 HTTP Body，确保多次读取
		c.Request.Body = io.NopCloser(bytes.NewReader(httpBody))

		// 签名校验
		if err := validateSignature(c.Request, publicKey); err != nil {
			log.Printf("Signature validation failed: %v", err)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid signature"})
			return
		}

		// 解析请求数据
		var payload Payload
		if err := json.Unmarshal(httpBody, &payload); err != nil {
			log.Println("Failed to parse HTTP payload:", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse payload"})
			return
		}

		// 判断 Op 类型
		switch payload.Op {
		case 13:
			// 处理签名验证请求
			var msg bytes.Buffer
			msg.WriteString(payload.D.EventTs)
			msg.WriteString(payload.D.PlainToken)
			signature := hex.EncodeToString(ed25519.Sign(privateKey, msg.Bytes()))

			// 返回签名验证响应
			c.JSON(http.StatusOK, gin.H{
				"plain_token": payload.D.PlainToken,
				"signature":   signature,
			})

		default:
			// 解一次 t / d.id，决定是否走 "等待应用端覆盖 code" 路径
			eventT, interactionID := parseInteractionProbe(httpBody)
			isInteraction := eventT == string(dto.EventInteractionCreate) && interactionID != ""

			// 仅 INTERACTION_CREATE 且配置了等待时长时注册 pending 槽位。
			// 必须在入队前注册，确保应用端处理事件时能找到槽位。
			var waitFn func(time.Duration) (int, bool)
			if isInteraction {
				if config.GetWebhookRespWaitMs() > 0 {
					waitFn = interactionwait.Register(interactionID)
				}
			}

			// 异步推送消息到队列
			go func(httpBody []byte, payload Payload) {
				webhookPayload := &WebhookPayload{
					PlainToken: payload.D.PlainToken,
					EventTs:    payload.D.EventTs,
					RawMessage: httpBody,
				}

				// 尝试写入队列
				select {
				case wh.messageQueue <- webhookPayload:
					mylog.Println("Message enqueued successfully")
				default:
					log.Println("Message queue is full, dropping message")
				}
			}(httpBody, payload)

			// 返回 HTTP Callback ACK；对 INTERACTION_CREATE 事件可附带 code 覆盖按钮回应提示
			c.JSON(http.StatusOK, buildWebhookAckBody(isInteraction, interactionID, waitFn))
		}
	}
}

// parseInteractionProbe 从 webhook 原始 body 中提取 t 和 d.id。解析失败时返回空串。
func parseInteractionProbe(httpBody []byte) (eventT, interactionID string) {
	var probe struct {
		T string `json:"t"`
		D struct {
			ID string `json:"id"`
		} `json:"d"`
	}
	if err := json.Unmarshal(httpBody, &probe); err != nil {
		return "", ""
	}
	return probe.T, probe.D.ID
}

// buildWebhookAckBody 构造 webhook 200 OK 的响应体。
//
//   - 非 INTERACTION_CREATE 事件：固定返回 {"op":12}。
//   - INTERACTION_CREATE 事件：
//   - 若 waitFn 非空，最多等 webhook_resp_wait_ms 毫秒，期间应用端可通过
//     send_group_msg / send_private_msg / put_interaction 提供 code 覆盖兜底。
//   - 超时或 waitFn 为空时，使用 webhook_resp_code 兜底（0 时返回纯 {"op":12}）。
func buildWebhookAckBody(isInteraction bool, interactionID string, waitFn func(time.Duration) (int, bool)) gin.H {
	if !isInteraction {
		return gin.H{"op": 12}
	}

	fallback := config.GetWebhookRespCode()

	if waitFn != nil {
		wait := time.Duration(config.GetWebhookRespWaitMs()) * time.Millisecond
		if code, ok := waitFn(wait); ok {
			mylog.Printf("[webhook] interaction %s 应用端 code=%d", interactionID, code)
			return gin.H{"op": 12, "code": code}
		}
		// 超时走兜底
	}

	if fallback == 0 {
		return gin.H{"op": 12}
	}
	mylog.Printf("[webhook] interaction %s 兜底 code=%d", interactionID, fallback)
	return gin.H{"op": 12, "code": fallback}
}

// 签名验证逻辑
func validateSignature(req *http.Request, publicKey ed25519.PublicKey) error {
	// 获取 X-Signature-Ed25519 Header
	signature := req.Header.Get("X-Signature-Ed25519")
	if signature == "" {
		return errors.New("missing X-Signature-Ed25519 header")
	}

	// 解码 Signature
	sig, err := hex.DecodeString(signature)
	if err != nil {
		return errors.New("invalid hex encoding in signature")
	}
	if len(sig) != ed25519.SignatureSize || sig[63]&224 != 0 {
		return errors.New("invalid signature size or format")
	}

	// 获取 X-Signature-Timestamp Header
	timestamp := req.Header.Get("X-Signature-Timestamp")
	if timestamp == "" {
		return errors.New("missing X-Signature-Timestamp header")
	}

	// 读取 HTTP Body
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return errors.New("failed to read HTTP body")
	}
	req.Body = io.NopCloser(bytes.NewReader(body)) // 恢复 Body 以供后续使用

	// 组合签名体: timestamp + body
	var msg bytes.Buffer
	msg.WriteString(timestamp)
	msg.Write(body)

	// 使用 Ed25519 验证签名
	if !ed25519.Verify(publicKey, msg.Bytes(), sig) {
		return errors.New("signature verification failed")
	}

	return nil
}

// listenAndProcessMessages 启动协程处理队列中的消息
func (wh *WebhookHandler) ListenAndProcessMessages() {
	for payload := range wh.messageQueue {
		go func(p *WebhookPayload) {
			mylog.Printf("Processing Webhook event with token: %s", p.PlainToken)
			// 业务逻辑处理的地方
			payload := &dto.WSPayload{}
			if err := json.Unmarshal(p.RawMessage, payload); err != nil {
				log.Printf("%s json failed, %v", p.EventTs, err)
				return
			}
			// 更新 global_s 的值
			atomic.StoreInt64(&client.Global_s, payload.S)

			payload.RawMessage = p.RawMessage
			mylog.Printf("%s receive %s message, %s", p.EventTs, dto.OPMeans(payload.OPCode), string(p.RawMessage))

			// 性能不够 报错也没用 就扬了
			go event.ParseAndHandle(payload)
		}(payload)
	}
	log.Println("Message queue is closed")
}

// Close 关闭消息队列
func (wh *WebhookHandler) Close() {
	close(wh.messageQueue)
}
