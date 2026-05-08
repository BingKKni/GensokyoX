package wsclient

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/hoshinonyaruko/gensokyo/botstats"
	"github.com/hoshinonyaruko/gensokyo/callapi"
	"github.com/hoshinonyaruko/gensokyo/config"
	"github.com/hoshinonyaruko/gensokyo/mylog"
	"github.com/tencent-connect/botgo/openapi"
)

type WebSocketClient struct {
	conn   *websocket.Conn
	api    openapi.OpenAPI
	apiv2  openapi.OpenAPI
	botID  uint64
	urlStr string
	cancel context.CancelFunc
	// isReconnecting: 原子标志，保证同一时刻只有一个 Reconnect goroutine 在跑。
	// 调用点用 CompareAndSwap(false, true) 原子抢占抢到才能启动 Reconnect，
	// 避免 startWriter 和 handleIncomingMessages 同时检测到连接断开后各自启动一个 Reconnect。
	isReconnecting atomic.Bool
	// writeQueue: 无界写入队列。SendMessage 入队（lock+append），startWriter 出队（lock+取首）。
	// 跨 Reconnect 持久化，应用端短暂不可用期间 QQ 推送的事件不会因为缓冲耗尽被丢弃。
	writeQueue   []writeRequest
	writeQueueMu sync.Mutex
	// writeNotify: 容量为 1 的信号通道，作为 startWriter 在队列空时的唤醒源。
	// SendMessage 入队后非阻塞投放一次信号；多次入队期间信号合并到一个，
	// startWriter 拿到一次唤醒后会自旋取队列直到为空，无需逐条对应。
	writeNotify chan struct{}
	closeCh     chan struct{} // 用于关闭的通道
	writerDone  chan struct{}
}

type writeRequest struct {
	messageType int
	data        []byte
}

// SendMessage 入队待发送的消息。无界队列，永远入队成功，应用端短暂不可用期间不会丢消息。
// 真正写入由 startWriter 异步消费。
func (client *WebSocketClient) SendMessage(message map[string]interface{}) error {
	// 序列化消息
	msgBytes, err := json.Marshal(message)
	if err != nil {
		log.Println("Error marshalling message:", err)
		return err
	}

	req := writeRequest{
		messageType: websocket.TextMessage,
		data:        msgBytes,
	}

	client.writeQueueMu.Lock()
	client.writeQueue = append(client.writeQueue, req)
	client.writeQueueMu.Unlock()

	// 非阻塞唤醒 startWriter（writeNotify 容量 1，多余的信号合并到一个）
	select {
	case client.writeNotify <- struct{}{}:
	default:
	}
	return nil
}

// Close 关闭 WebSocketClient，停止写 Goroutine
func (client *WebSocketClient) Close() error {
	close(client.closeCh)
	client.conn.Close()
	return nil
}

// startWriter 专用的写 Goroutine，从无界队列 writeQueue 取消息并写入连接。通过 ctx 控制生命周期。
// done 与 conn 显式传入：每个 startWriter 实例只对自己启动时绑定的连接和完成信号通道负责，
// 避免在 Reconnect 复用 client 字段时与新一代 goroutine 共享/重复 close(writerDone) 或并发写同一个 conn。
func (client *WebSocketClient) startWriter(ctx context.Context, done chan struct{}, conn *websocket.Conn) {
	defer close(done)
	for {
		// 先快速检查停止信号，避免在队列非空时连续忙写错过 ctx/closeCh
		select {
		case <-ctx.Done():
			return
		case <-client.closeCh:
			return
		default:
		}

		// 取一条；队列空则阻塞等待 SendMessage 投递信号 / 停止信号
		client.writeQueueMu.Lock()
		if len(client.writeQueue) == 0 {
			client.writeQueueMu.Unlock()
			select {
			case <-client.writeNotify:
				continue
			case <-ctx.Done():
				return
			case <-client.closeCh:
				return
			}
		}
		req := client.writeQueue[0]
		client.writeQueue = client.writeQueue[1:]
		client.writeQueueMu.Unlock()

		// 设置单次写超时：防止 TCP 流控（应用端读取缓慢）导致永久阻塞。
		// 超时不代表消息丢失——会触发重连，本条消息会被放回队列头由新 startWriter 续发。
		conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
		err := conn.WriteMessage(req.messageType, req.data)
		conn.SetWriteDeadline(time.Time{}) // 重置，避免影响后续写入
		if err != nil {
			log.Printf("startWriter: 写入失败（TCP 可能阻塞），触发重连: %v", err)
			// 默认行为：把失败的这条放回队列头，重连后由新 startWriter 顺序续发。
			// 若配置 disable_error_chan: true，则丢弃，避免应用端长期不可用导致内存堆积。
			if !config.GetDisableErrorChan() {
				client.writeQueueMu.Lock()
				client.writeQueue = append([]writeRequest{req}, client.writeQueue...)
				client.writeQueueMu.Unlock()
			}
			// 连接已死，触发重连后退出；重连完成后会重启 startWriter
			// CAS(false, true) 原子抢占：抢到才 go Reconnect，避免与 handleIncomingMessages 双Reconnect
			if client.isReconnecting.CompareAndSwap(false, true) {
				go client.Reconnect()
			}
			return
		}
	}
}

// 处理onebotv11应用端发来的信息
// conn 显式传入：避免 Reconnect 替换 client.conn 后老 goroutine 误读新连接。
func (client *WebSocketClient) handleIncomingMessages(cancel context.CancelFunc, conn *websocket.Conn) {
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			mylog.Println("WebSocket connection closed:", err)
			cancel() // 取消心跳 goroutine
			// 应用端用 1012 (Service Restart) 主动关闭 = 它正在重启。
			// 这里先等 5 秒避开握手试探期的紧密 Dial-then-immediately-close。
			// 如果应用端启动慢（5s 不够），后续由 Reconnect() 里序的 0.5s 退避循环继续重试（受 reconnect_times 控制），
			// 唯一代价只是多打几行 connection refused 警告日志。
			if websocket.IsCloseError(err, websocket.CloseServiceRestart) {
				mylog.Println("收到 Service Restart (1012)，应用端正在重启，等待 5 秒再重连。")
				time.Sleep(5 * time.Second)
			}
			// CAS(false, true) 原子抢占：抢到才 go Reconnect，避免与 startWriter 双Reconnect
			if client.isReconnecting.CompareAndSwap(false, true) {
				go client.Reconnect()
			}
			return // 退出循环，不再尝试读取消息
		}

		go client.recvMessage(msg)
	}
}

// 断线重连
func (client *WebSocketClient) Reconnect() {
	// isReconnecting 已由调用点 CompareAndSwap(false, true) 原子抢占设为 true。
	// 提前注册 defer：保证不论从哪条路径 return（reconnect_times=-1不重连 / 达到重试上限 / 正常完成）都会把标志复位，
	// 避免被卡在 isReconnecting=true 导致后续任何重连触发都 CAS 失败、全部静默错过。
	defer client.isReconnecting.Store(false)

	client.cancel() // 先停止旧的所有 goroutine 并等待写 goroutine 完全退出，防止 concurrent write
	client.conn.Close()
	<-client.writerDone

	addresses := config.GetWsAddress()
	tokens := config.GetWsToken()

	var token string
	for index, address := range addresses {
		if address == client.urlStr && index < len(tokens) {
			token = tokens[index]
			break
		}
	}

	// 检查URL中是否有access_token参数
	mp := getParamsFromURI(client.urlStr)
	if val, ok := mp["access_token"]; ok {
		token = val
	}

	headers := http.Header{
		"User-Agent":    []string{"CQHttp/4.15.0"},
		"X-Client-Role": []string{"Universal"},
		"X-Self-ID":     []string{fmt.Sprintf("%d", client.botID)},
	}

	if token != "" {
		headers["Authorization"] = []string{"Token " + token}
	}
	mylog.Printf("准备使用token[%s]重新连接到[%s]\n", token, client.urlStr)
	dialer := websocket.Dialer{
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: 45 * time.Second,
	}

	var conn *websocket.Conn
	var err error

	// reconnect_times 语义：
	//   == -1: 不尝试重连（调试 / 临时场景，应用端掉线后不再发起任何重试）
	//   ==  0: 无限重连，永不放弃（应用端可能启动失败要长时间排查的场景，等多久都要接上）
	//   >   0: 重连最多这么多次，与旧语义一致
	maxRetryAttempts := config.GetReconnecTimes()
	if maxRetryAttempts < 0 {
		mylog.Printf("reconnect_times = %d 配置为不重连，放弃 [%v]", maxRetryAttempts, client.urlStr)
		return
	}
	retryCount := 0
	for {
		mylog.Println("Dialing URL:", client.urlStr)
		conn, _, err = dialer.Dial(client.urlStr, headers)
		if err != nil {
			retryCount++
			// maxRetryAttempts == 0 表示无限重连，不检查上限
			if maxRetryAttempts > 0 && retryCount > maxRetryAttempts {
				mylog.Printf("Exceeded maximum retry attempts for WebSocket[%v]: %v\n", client.urlStr, err)
				return
			}
			mylog.Printf("Failed to connect to WebSocket[%v]: %v, retrying in 0.5 second...\n", client.urlStr, err)
			time.Sleep(500 * time.Millisecond) // sleep for 0.5 second before retrying
		} else {
			mylog.Printf("Successfully connected to %s.\n", client.urlStr) // 输出连接成功提示
			break                                                          // successfully connected, break the loop
		}
	}
	// 复用现有的client完成重连
	client.conn = conn

	// 再次发送元事件
	message := map[string]interface{}{
		"meta_event_type": "lifecycle",
		"post_type":       "meta_event",
		"sub_type":        "connect",
		"original":        nil,
	}

	mylog.Printf("Message: %+v\n", message)

	err = client.SendMessage(message)
	if err != nil {
		// handle error
		mylog.Printf("Error sending message: %v\n", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	client.cancel = cancel
	done := make(chan struct{})
	client.writerDone = done
	heartbeatinterval := config.GetHeartBeatInterval()
	// 显式传入 done/conn：让本代 startWriter / handleIncomingMessages 只对自己绑定的对象负责，
	// 不会因为后续 Reconnect 替换 client.writerDone / client.conn 而互相干扰。
	go client.startWriter(ctx, done, conn)
	go client.sendHeartbeat(ctx, heartbeatinterval)
	go client.handleIncomingMessages(cancel, conn)

	mylog.Printf("Successfully reconnected to WebSocket.")

}

// 处理信息,调用腾讯api
func (client *WebSocketClient) recvMessage(msg []byte) {
	var message callapi.ActionMessage
	//mylog.Println("Received from onebotv11 server raw:", string(msg))
	err := json.Unmarshal(msg, &message)
	if err != nil {
		mylog.Printf("Error unmarshalling message: %v, Original message: %s", err, string(msg))
		return
	}
	mylog.Println("Received from onebotv11 server:", TruncateMessage(message, 800))
	// 调用callapi
	go callapi.CallAPIFromDict(client, client.api, client.apiv2, message)
}

// 截断信息
func TruncateMessage(message callapi.ActionMessage, maxLength int) string {
	paramsStr, err := json.Marshal(message.Params)
	if err != nil {
		return "Error marshalling Params for truncation."
	}

	// Truncate Params if its length exceeds maxLength
	truncatedParams := string(paramsStr)
	if len(truncatedParams) > maxLength {
		truncatedParams = truncatedParams[:maxLength] + "..."
	}

	return fmt.Sprintf("Action: %s, Params: %s, Echo: %v", message.Action, truncatedParams, message.Echo)
}

// 发送心跳包
func (client *WebSocketClient) sendHeartbeat(ctx context.Context, heartbeatinterval int) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Duration(heartbeatinterval) * time.Second):
			messageReceived, messageSent, lastMessageTime, err := botstats.GetStats()
			if err != nil {
				mylog.Printf("心跳错误,获取机器人发信状态错误:%v", err)
			}
			message := map[string]interface{}{
				"post_type":       "meta_event",
				"meta_event_type": "heartbeat",
				"original":        nil,
				"status": map[string]interface{}{
					"app_enabled":     true,
					"app_good":        true,
					"app_initialized": true,
					"good":            true,
					"online":          true,
					"plugins_good":    nil,
					"stat": map[string]int{
						"packet_received":   34933,
						"packet_sent":       8513,
						"packet_lost":       0,
						"message_received":  messageReceived,
						"message_sent":      messageSent,
						"disconnect_times":  0,
						"lost_times":        0,
						"last_message_time": int(lastMessageTime),
					},
				},
				"interval": 5000, // 以毫秒为单位
			}
			client.SendMessage(message)
		}
	}
}

// NewWebSocketClient 创建 WebSocketClient 实例，接受 WebSocket URL、botID 和 openapi.OpenAPI 实例
func NewWebSocketClient(urlStr string, botID uint64, api openapi.OpenAPI, apiv2 openapi.OpenAPI, maxRetryAttempts int) (*WebSocketClient, error) {
	addresses := config.GetWsAddress()
	tokens := config.GetWsToken()

	var token string
	for index, address := range addresses {
		if address == urlStr && index < len(tokens) {
			token = tokens[index]
			break
		}
	}

	// 检查URL中是否有access_token参数
	mp := getParamsFromURI(urlStr)
	if val, ok := mp["access_token"]; ok {
		token = val
	}

	headers := http.Header{
		"User-Agent":    []string{"CQHttp/4.15.0"},
		"X-Client-Role": []string{"Universal"},
		"X-Self-ID":     []string{fmt.Sprintf("%d", botID)},
	}

	if token != "" {
		headers["Authorization"] = []string{"Token " + token}
	}
	mylog.Printf("准备使用token[%s]连接到[%s]\n", token, urlStr)
	dialer := websocket.Dialer{
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: 45 * time.Second,
	}

	var conn *websocket.Conn
	var err error

	retryCount := 0
	for {
		mylog.Println("Dialing URL:", urlStr)
		conn, _, err = dialer.Dial(urlStr, headers)
		if err != nil {
			retryCount++
			if retryCount > maxRetryAttempts {
				mylog.Printf("Exceeded maximum retry attempts for WebSocket[%v]: %v\n", urlStr, err)
				return nil, err
			}
			mylog.Printf("Failed to connect to WebSocket[%v]: %v, retrying in 5 seconds...\n", urlStr, err)
			time.Sleep(5 * time.Second) // sleep for 5 seconds before retrying
		} else {
			mylog.Printf("Successfully connected to %s.\n", urlStr) // 输出连接成功提示
			break                                                   // successfully connected, break the loop
		}
	}
	client := &WebSocketClient{
		conn:        conn,
		api:         api,
		apiv2:       apiv2,
		botID:       botID,
		urlStr:      urlStr,
		writeQueue:  make([]writeRequest, 0, 64),
		writeNotify: make(chan struct{}, 1),
		closeCh:     make(chan struct{}),
		writerDone:  make(chan struct{}),
	}

	// Sending initial message similar to your setupB function
	message := map[string]interface{}{
		"meta_event_type": "lifecycle",
		"post_type":       "meta_event",
		"sub_type":        "connect",
		"original":        nil,
	}

	mylog.Printf("Message: %+v\n", message)

	err = client.SendMessage(message)
	if err != nil {
		// handle error
		mylog.Printf("Error sending message: %v\n", err)
	}

	// Starting goroutine for heartbeats and another for listening to messages
	ctx, cancel := context.WithCancel(context.Background())

	client.cancel = cancel
	heartbeatinterval := config.GetHeartBeatInterval()
	go client.startWriter(ctx, client.writerDone, conn)
	go client.sendHeartbeat(ctx, heartbeatinterval)
	go client.handleIncomingMessages(cancel, conn)

	return client, nil
}

// getParamsFromURI 解析给定URI中的查询参数，并返回一个映射（map）
func getParamsFromURI(uriStr string) map[string]string {
	params := make(map[string]string)

	u, err := url.Parse(uriStr)
	if err != nil {
		mylog.Printf("Error parsing the URL: %v\n", err)
		return params
	}

	// 遍历查询参数并将其添加到返回的映射中
	for key, values := range u.Query() {
		if len(values) > 0 {
			params[key] = values[0] // 如果一个参数有多个值，这里只选择第一个。可以根据需求进行调整。
		}
	}

	return params
}
