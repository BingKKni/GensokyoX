// 处理收到的信息事件
package Processor

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/hoshinonyaruko/gensokyo/callapi"
	"github.com/hoshinonyaruko/gensokyo/config"
	"github.com/hoshinonyaruko/gensokyo/echo"
	"github.com/hoshinonyaruko/gensokyo/handlers"
	"github.com/hoshinonyaruko/gensokyo/idmap"
	"github.com/hoshinonyaruko/gensokyo/mylog"
	"github.com/hoshinonyaruko/gensokyo/structs"
	"github.com/hoshinonyaruko/gensokyo/wsclient"
	"github.com/tencent-connect/botgo/dto"
	"github.com/tencent-connect/botgo/dto/keyboard"
	"github.com/tencent-connect/botgo/openapi"
)

// Processor 结构体用于处理消息
type Processors struct {
	Api             openapi.OpenAPI                   // API 类型
	Apiv2           openapi.OpenAPI                   //群的API
	Settings        *structs.Settings                 // 使用指针
	Wsclient        []*wsclient.WebSocketClient       // 指针的切片
	WsServerClients []callapi.WebSocketServerClienter //ws server被连接的客户端
}

// 频道信息事件
type OnebotChannelMessage struct {
	ChannelID       string      `json:"channel_id"`
	GuildID         string      `json:"guild_id"`
	Message         interface{} `json:"message"`
	MessageID       string      `json:"message_id"`
	MessageType     string      `json:"message_type"`
	PostType        string      `json:"post_type"`
	SelfTinyID      string      `json:"self_tiny_id"`
	SubType         string      `json:"sub_type"`
	UserID          int64       `json:"user_id"`
	Original        interface{} `json:"original"`
	RealMessageType string      `json:"real_message_type,omitempty"` //当前信息的真实类型 表情表态
	InteractionID   string      `json:"interaction_id,omitempty"`    //QQ平台下发的原始交互ID，当启用global_interaction_to_message时设置
}

// 群信息事件
type OnebotGroupMessage struct {
	MessageID       int         `json:"message_id"`
	GroupID         int64       `json:"group_id"` // Can be either string or int depending on p.Settings.CompleteFields
	MessageType     string      `json:"message_type"`
	PostType        string      `json:"post_type"`
	SubType         string      `json:"sub_type"`
	Message         interface{} `json:"message"` // For array format
	UserID          int64       `json:"user_id"`
	Original        interface{} `json:"original"`
	RealMessageType string      `json:"real_message_type,omitempty"` //当前信息的真实类型 group group_private guild guild_private
	RealUserID      string      `json:"real_user_id,omitempty"`      //当前真实uid
	RealGroupID     string      `json:"real_group_id,omitempty"`     //当前真实gid
	InteractionID   string      `json:"interaction_id,omitempty"`    //QQ平台下发的原始交互ID，当启用global_interaction_to_message时设置
}

// 私聊信息事件
type OnebotPrivateMessage struct {
	MessageID       int         `json:"message_id"` // Can be either string or int depending on logic
	MessageType     string      `json:"message_type"`
	PostType        string      `json:"post_type"`
	SubType         string      `json:"sub_type"`
	Message         interface{} `json:"message"` // For array format
	UserID          int64       `json:"user_id"` // Can be either string or int depending on logic
	Original        interface{} `json:"original"`
	RealMessageType string      `json:"real_message_type,omitempty"` //当前信息的真实类型 group group_private guild guild_private
	RealUserID      string      `json:"real_user_id,omitempty"`      //当前真实uid
	InteractionID   string      `json:"interaction_id,omitempty"`    //QQ平台下发的原始交互ID，当启用global_interaction_to_message时设置
}

// onebotv11标准扩展
type OnebotInteractionNotice struct {
	GroupID     int64                  `json:"group_id,omitempty"`
	NoticeType  string                 `json:"notice_type,omitempty"`
	PostType    string                 `json:"post_type,omitempty"`
	SubType     string                 `json:"sub_type,omitempty"`
	UserID      int64                  `json:"user_id,omitempty"`
	Data        *dto.WSInteractionData `json:"data,omitempty"`
	Original    interface{}            `json:"original"`
	RealUserID  string                 `json:"real_user_id,omitempty"`  //当前真实uid
	RealGroupID string                 `json:"real_group_id,omitempty"` //当前真实gid
}

// onebotv11标准扩展
type OnebotGroupRejectNotice struct {
	GroupID    int64                    `json:"group_id,omitempty"`
	NoticeType string                   `json:"notice_type,omitempty"`
	PostType   string                   `json:"post_type,omitempty"`
	SubType    string                   `json:"sub_type,omitempty"`
	UserID     int64                    `json:"user_id,omitempty"`
	Data       *dto.GroupMsgRejectEvent `json:"data,omitempty"`
	Original   interface{}              `json:"original"`
}

// onebotv11标准扩展
type OnebotGroupReceiveNotice struct {
	GroupID    int64                     `json:"group_id,omitempty"`
	NoticeType string                    `json:"notice_type,omitempty"`
	PostType   string                    `json:"post_type,omitempty"`
	SubType    string                    `json:"sub_type,omitempty"`
	UserID     int64                     `json:"user_id,omitempty"`
	Data       *dto.GroupMsgReceiveEvent `json:"data,omitempty"`
	Original   interface{}               `json:"original"`
}

// 打印结构体的函数
func PrintStructWithFieldNames(v interface{}) {
	val := reflect.ValueOf(v)

	// 如果是指针，获取其指向的元素
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	typ := val.Type()

	// 确保我们传入的是一个结构体
	if typ.Kind() != reflect.Struct {
		mylog.Println("Input is not a struct")
		return
	}

	// 单行 JSON 打印结构体
	if data, jerr := json.Marshal(val.Interface()); jerr == nil {
		mylog.Printf("%s: %s", typ.Name(), string(data))
	} else {
		mylog.Printf("%s: %+v", typ.Name(), val.Interface())
	}
}

// 将结构体转换为 map[string]interface{}
func structToMap(obj interface{}) map[string]interface{} {
	out := make(map[string]interface{})
	j, _ := json.Marshal(obj)
	json.Unmarshal(j, &out)
	return out
}

func parseOriginalPayload(raw []byte) interface{} {
	if len(raw) == 0 {
		return nil
	}

	var payload interface{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return string(raw)
	}

	return payload
}

// 修改函数的返回类型为 *Processor
func NewProcessor(api openapi.OpenAPI, apiv2 openapi.OpenAPI, settings *structs.Settings, wsclient []*wsclient.WebSocketClient) *Processors {
	return &Processors{
		Api:      api,
		Apiv2:    apiv2,
		Settings: settings,
		Wsclient: wsclient,
	}
}

// 修改函数的返回类型为 *Processor
func NewProcessorV2(api openapi.OpenAPI, apiv2 openapi.OpenAPI, settings *structs.Settings) *Processors {
	return &Processors{
		Api:      api,
		Apiv2:    apiv2,
		Settings: settings,
	}
}

// 发信息给所有连接正向ws的客户端
func (p *Processors) SendMessageToAllClients(message map[string]interface{}) error {
	var result *multierror.Error

	for _, client := range p.WsServerClients {
		// 使用接口的方法
		err := client.SendMessage(message)
		if err != nil {
			// Append the error to our result
			result = multierror.Append(result, fmt.Errorf("failed to send to client: %w", err))
		}
	}

	// This will return nil if no errors were added
	return result.ErrorOrNil()
}

// 方便快捷的发信息函数
func (p *Processors) BroadcastMessageToAllFAF(message map[string]interface{}, api openapi.MessageAPI, data interface{}) error {
	// 并发发送到我们作为客户端的Wsclient
	for _, client := range p.Wsclient {
		go func(c callapi.WebSocketServerClienter) {
			_ = c.SendMessage(message) // 忽略错误
		}(client)
	}

	// 并发发送到我们作为服务器连接到我们的WsServerClients
	for _, serverClient := range p.WsServerClients {
		go func(sc callapi.WebSocketServerClienter) {
			_ = sc.SendMessage(message) // 忽略错误
		}(serverClient)
	}

	// 不再等待所有 goroutine 完成，直接返回
	return nil
}

// 方便快捷的发信息函数
func (p *Processors) BroadcastMessageToAll(message map[string]interface{}, api openapi.MessageAPI, data interface{}) error {
	var wg sync.WaitGroup
	errorCh := make(chan string, len(p.Wsclient)+len(p.WsServerClients))
	defer close(errorCh)

	// 并发发送到我们作为客户端的Wsclient
	for _, client := range p.Wsclient {
		wg.Add(1)
		go func(c callapi.WebSocketServerClienter) {
			defer wg.Done()
			if err := c.SendMessage(message); err != nil {
				errorCh <- fmt.Sprintf("error sending message via wsclient: %v", err)
			}
		}(client)
	}

	// 并发发送到我们作为服务器连接到我们的WsServerClients
	for _, serverClient := range p.WsServerClients {
		wg.Add(1)
		go func(sc callapi.WebSocketServerClienter) {
			defer wg.Done()
			if err := sc.SendMessage(message); err != nil {
				errorCh <- fmt.Sprintf("error sending message via WsServerClient: %v", err)
			}
		}(serverClient)
	}

	wg.Wait() // 等待所有goroutine完成

	var errors []string
	failed := 0
	for len(errorCh) > 0 {
		err := <-errorCh
		errors = append(errors, err)
		failed++
	}

	// 仅对连接正反ws的bot应用这个判断
	if !p.Settings.HttpOnlyBot {
		// 检查是否所有尝试都失败了
		if failed == len(p.Wsclient)+len(p.WsServerClients) {
			// 处理全部失败的情况
			fmt.Println("All ws event sending attempts failed.")
			downtimemessgae := config.GetDowntimeMessage()
			switch v := data.(type) {
			case *dto.WSGroupATMessageData:
				msgtocreate := &dto.MessageToCreate{
					Content: downtimemessgae,
					MsgID:   v.ID,
					MsgSeq:  1,
					MsgType: 0, // 默认文本类型
				}
				api.PostGroupMessage(context.Background(), v.GroupID, msgtocreate)
			case *dto.WSATMessageData:
				msgtocreate := &dto.MessageToCreate{
					Content: downtimemessgae,
					MsgID:   v.ID,
					MsgSeq:  1,
					MsgType: 0, // 默认文本类型
				}
				api.PostMessage(context.Background(), v.ChannelID, msgtocreate)
			case *dto.WSMessageData:
				msgtocreate := &dto.MessageToCreate{
					Content: downtimemessgae,
					MsgID:   v.ID,
					MsgSeq:  1,
					MsgType: 0, // 默认文本类型
				}
				api.PostMessage(context.Background(), v.ChannelID, msgtocreate)
			case *dto.WSDirectMessageData:
				msgtocreate := &dto.MessageToCreate{
					Content: downtimemessgae,
					MsgID:   v.ID,
					MsgSeq:  1,
					MsgType: 0, // 默认文本类型
				}
				api.PostMessage(context.Background(), v.GuildID, msgtocreate)
			case *dto.WSC2CMessageData:
				msgtocreate := &dto.MessageToCreate{
					Content: downtimemessgae,
					MsgID:   v.ID,
					MsgSeq:  1,
					MsgType: 0, // 默认文本类型
				}
				api.PostC2CMessage(context.Background(), v.Author.ID, msgtocreate)
			}
		}
	}

	// 判断是否填写了反向post地址
	if !allEmpty(config.GetPostUrl()) {
		go PostMessageToUrls(message)
	}

	if len(errors) > 0 {
		return fmt.Errorf(strings.Join(errors, "; "))
	}

	return nil
}

// allEmpty checks if all the strings in the slice are empty.
func allEmpty(addresses []string) bool {
	for _, addr := range addresses {
		if addr != "" {
			return false
		}
	}
	return true
}

// PostMessageToUrls 使用并发 goroutines 上报信息给多个反向 HTTP URL
func PostMessageToUrls(message map[string]interface{}) {
	// 获取上报 URL 列表
	postUrls := config.GetPostUrl()

	// 检查 postUrls 是否为空
	if len(postUrls) == 0 {
		return
	}

	// 转换 message 为 JSON 字符串
	jsonString, err := handlers.ConvertMapToJSONString(message)
	if err != nil {
		mylog.Printf("Error converting message to JSON: %v", err)
		return
	}

	// 使用 WaitGroup 等待所有 goroutines 完成
	var wg sync.WaitGroup
	for _, url := range postUrls {
		wg.Add(1)
		// 启动一个 goroutine
		go func(url string) {
			defer wg.Done() // 确保减少 WaitGroup 的计数器
			sendPostRequest(jsonString, url)
		}(url)
	}
	wg.Wait() // 等待所有 goroutine 完成
}

// sendPostRequest 发送单个 POST 请求
func sendPostRequest(jsonString, url string) {
	// 创建请求体
	reqBody := bytes.NewBufferString(jsonString)

	// 创建 POST 请求
	req, err := http.NewRequest("POST", url, reqBody)
	if err != nil {
		mylog.Printf("Error creating POST request to %s: %v", url, err)
		return
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	// 设置 X-Self-ID
	var selfid string
	if config.GetUseUin() {
		selfid = config.GetUinStr()
	} else {
		selfid = config.GetAppIDStr()
	}
	req.Header.Set("X-Self-ID", selfid)

	// 发送请求
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		mylog.Printf("Error sending POST request to %s: %v", url, err)
		return
	}
	defer resp.Body.Close() // 确保释放网络资源

	// 反向 HTTP 上报成功的情况下不再打印日志，仅在前面的 NewRequest/Do 失败时记录错误
}

func (p *Processors) HandleFrameworkCommand(messageText string, data interface{}, Type string) error {
	// 正则表达式匹配转换后的 CQ 码
	cqRegex := regexp.MustCompile(`\[CQ:at,qq=\d+\]`)

	// 使用正则表达式替换所有的 CQ 码为 ""
	cleanedMessage := cqRegex.ReplaceAllString(messageText, "")

	// 去除字符串前后的空格
	cleanedMessage = strings.TrimSpace(cleanedMessage)
	if cleanedMessage == "t" {
		// 生成临时指令
		tempCmd := handleNoPermission()
		mylog.Printf("临时bind指令: %s 可忽略权限检查1次,或将masterid设置为空数组", tempCmd)
	}
	var err error
	var now, new, newpro1, newpro2 string
	var nowgroup, newgroup string
	var realid, realid2 string
	var guildid, guilduserid string

	// 避免未使用变量警告
	_ = err
	_ = now
	_ = new
	_ = newpro1
	_ = newpro2
	_ = nowgroup
	_ = newgroup
	_ = realid
	_ = realid2
	_ = guildid
	_ = guilduserid
	switch v := data.(type) {
	case *dto.WSGroupATMessageData:
		realid = v.Author.ID
	case *dto.WSATMessageData:
		realid = v.Author.ID
		guildid = v.GuildID
		guilduserid = v.Author.ID
	case *dto.WSMessageData:
		realid = v.Author.ID
		guildid = v.GuildID
		guilduserid = v.Author.ID
	case *dto.WSDirectMessageData:
		realid = v.Author.ID
	case *dto.WSC2CMessageData:
		realid = v.Author.ID
	}

	switch v := data.(type) {
	case *dto.WSGroupATMessageData:
		realid2 = v.GroupID
	case *dto.WSATMessageData:
		realid2 = v.ChannelID
	case *dto.WSMessageData:
		realid2 = v.ChannelID
	case *dto.WSDirectMessageData:
		realid2 = v.ChannelID
	case *dto.WSC2CMessageData:
		realid2 = "group_private"
	}

	// 获取MasterID数组
	masterIDs := config.GetMasterID()
	_ = masterIDs // 避免未使用变量警告

	return nil
}

// 生成由两个英文字母构成的唯一临时指令
func generateTemporaryCommand() (string, error) {
	bytes := make([]byte, 1) // 生成1字节的随机数，足以表示2个十六进制字符
	if _, err := rand.Read(bytes); err != nil {
		return "", err // 处理随机数生成错误
	}
	command := hex.EncodeToString(bytes)[:2] // 将1字节转换为2个十六进制字符
	return command, nil
}

// 生成并添加一个新的临时指令
func handleNoPermission() string {
	idmap.MutexT.Lock()
	defer idmap.MutexT.Unlock()

	cmd, _ := generateTemporaryCommand()
	idmap.TemporaryCommands = append(idmap.TemporaryCommands, cmd)
	return cmd
}

// SendMessage 发送消息根据不同的类型
func SendMessage(messageText string, data interface{}, messageType string, api openapi.OpenAPI, apiv2 openapi.OpenAPI) error {
	// 强制类型转换，获取Message结构
	var msg *dto.Message
	switch v := data.(type) {
	case *dto.WSGroupATMessageData:
		msg = (*dto.Message)(v)
	case *dto.WSATMessageData:
		msg = (*dto.Message)(v)
	case *dto.WSMessageData:
		msg = (*dto.Message)(v)
	case *dto.WSDirectMessageData:
		msg = (*dto.Message)(v)
	case *dto.WSC2CMessageData:
		msg = (*dto.Message)(v)
	default:
		return nil
	}
	switch messageType {
	case "guild":
		// 处理公会消息
		msgseq := echo.GetMappingSeq(msg.ID)
		echo.AddMappingSeq(msg.ID, msgseq+1)
		textMsg, _ := handlers.GenerateReplyMessage(msg.ID, nil, messageText, msgseq+1, "")
		if _, err := api.PostMessage(context.TODO(), msg.ChannelID, textMsg); err != nil {
			mylog.Printf("发送文本信息失败: %v", err)
			return err
		}

	case "group":
		// 处理群组消息
		msgseq := echo.GetMappingSeq(msg.ID)
		echo.AddMappingSeq(msg.ID, msgseq+1)
		textMsg, _ := handlers.GenerateReplyMessage(msg.ID, nil, messageText, msgseq+1, "")
		_, err := apiv2.PostGroupMessage(context.TODO(), msg.GroupID, textMsg)
		if err != nil {
			mylog.Printf("发送文本群组信息失败: %v", err)
			return err
		}

	case "guild_private":
		// 处理私信
		timestamp := time.Now().Unix()
		timestampStr := fmt.Sprintf("%d", timestamp)
		dm := &dto.DirectMessage{
			GuildID:    msg.GuildID,
			ChannelID:  msg.ChannelID,
			CreateTime: timestampStr,
		}
		msgseq := echo.GetMappingSeq(msg.ID)
		echo.AddMappingSeq(msg.ID, msgseq+1)
		textMsg, _ := handlers.GenerateReplyMessage(msg.ID, nil, messageText, msgseq+1, "")
		if _, err := apiv2.PostDirectMessage(context.TODO(), dm, textMsg); err != nil {
			mylog.Printf("发送文本信息失败: %v", err)
			return err
		}

	case "group_private":
		// 处理群组私聊消息
		msgseq := echo.GetMappingSeq(msg.ID)
		echo.AddMappingSeq(msg.ID, msgseq+1)
		textMsg, _ := handlers.GenerateReplyMessage(msg.ID, nil, messageText, msgseq+1, "")
		_, err := apiv2.PostC2CMessage(context.TODO(), msg.Author.ID, textMsg)
		if err != nil {
			mylog.Printf("发送文本私聊信息失败: %v", err)
			return err
		}

	default:
		return errors.New("未知的消息类型")
	}

	return nil
}

// SendMessageMd  发送Md消息根据不同的类型
func SendMessageMd(md *dto.Markdown, kb *keyboard.MessageKeyboard, data interface{}, messageType string, api openapi.OpenAPI, apiv2 openapi.OpenAPI) error {
	// 强制类型转换，获取Message结构
	var msg *dto.Message
	switch v := data.(type) {
	case *dto.WSGroupATMessageData:
		msg = (*dto.Message)(v)
	case *dto.WSATMessageData:
		msg = (*dto.Message)(v)
	case *dto.WSMessageData:
		msg = (*dto.Message)(v)
	case *dto.WSDirectMessageData:
		msg = (*dto.Message)(v)
	case *dto.WSC2CMessageData:
		msg = (*dto.Message)(v)
	default:
		return nil
	}
	switch messageType {
	case "guild":
		// 处理公会消息
		msgseq := echo.GetMappingSeq(msg.ID)
		echo.AddMappingSeq(msg.ID, msgseq+1)
		Message := &dto.MessageToCreate{
			MsgID:    msg.ID,
			MsgSeq:   msgseq,
			Markdown: md,
			Keyboard: kb,
			MsgType:  2, //md信息
		}
		Message.Timestamp = time.Now().Unix() // 设置时间戳
		if _, err := api.PostMessage(context.TODO(), msg.ChannelID, Message); err != nil {
			mylog.Printf("发送文本信息失败: %v", err)
			return err
		}

	case "group":
		// 处理群组消息
		msgseq := echo.GetMappingSeq(msg.ID)
		echo.AddMappingSeq(msg.ID, msgseq+1)
		Message := &dto.MessageToCreate{
			Content:  "markdown",
			MsgID:    msg.ID,
			MsgSeq:   msgseq,
			Markdown: md,
			Keyboard: kb,
			MsgType:  2, //md信息
		}
		Message.Timestamp = time.Now().Unix() // 设置时间戳
		_, err := apiv2.PostGroupMessage(context.TODO(), msg.GroupID, Message)
		if err != nil {
			mylog.Printf("发送文本群组信息失败: %v", err)
			return err
		}

	case "guild_private":
		// 处理私信
		timestamp := time.Now().Unix()
		timestampStr := fmt.Sprintf("%d", timestamp)
		dm := &dto.DirectMessage{
			GuildID:    msg.GuildID,
			ChannelID:  msg.ChannelID,
			CreateTime: timestampStr,
		}
		msgseq := echo.GetMappingSeq(msg.ID)
		echo.AddMappingSeq(msg.ID, msgseq+1)
		Message := &dto.MessageToCreate{
			MsgID:    msg.ID,
			MsgSeq:   msgseq,
			Markdown: md,
			Keyboard: kb,
			MsgType:  2, //md信息
		}
		Message.Timestamp = time.Now().Unix() // 设置时间戳
		if _, err := apiv2.PostDirectMessage(context.TODO(), dm, Message); err != nil {
			mylog.Printf("发送文本信息失败: %v", err)
			return err
		}

	case "group_private":
		// 处理群组私聊消息
		msgseq := echo.GetMappingSeq(msg.ID)
		echo.AddMappingSeq(msg.ID, msgseq+1)
		Message := &dto.MessageToCreate{
			Content:  "markdown",
			MsgID:    msg.ID,
			MsgSeq:   msgseq,
			Markdown: md,
			Keyboard: kb,
			MsgType:  2, //md信息
		}
		Message.Timestamp = time.Now().Unix() // 设置时间戳
		_, err := apiv2.PostC2CMessage(context.TODO(), msg.Author.ID, Message)
		if err != nil {
			mylog.Printf("发送文本私聊信息失败: %v", err)
			return err
		}

	default:
		return errors.New("未知的消息类型")
	}

	return nil
}

// cleanEventID 清理EventID，去除可能的前缀（如 "GROUP_ADD_ROBOT:"）
func cleanEventID(eventID string) string {
	if eventID == "" {
		return eventID
	}
	// 如果EventID包含 ":" 分隔符，取最后一部分（UUID部分）
	if idx := strings.LastIndex(eventID, ":"); idx >= 0 && idx < len(eventID)-1 {
		return eventID[idx+1:]
	}
	return eventID
}

// SendMessageMdAddBot  发送Md消息在AddBot事件
func SendMessageMdAddBot(md *dto.Markdown, kb *keyboard.MessageKeyboard, data *dto.GroupAddBotEvent, api openapi.OpenAPI, apiv2 openapi.OpenAPI) error {

	// 清理EventID，去除可能的前缀
	cleanedEventID := cleanEventID(data.EventID)

	// 处理群组消息
	msgseq := echo.GetMappingSeq(data.EventID)
	echo.AddMappingSeq(data.EventID, msgseq+1)
	Message := &dto.MessageToCreate{
		Content:  "markdown",
		EventID:  cleanedEventID, // 使用清理后的EventID
		MsgSeq:   msgseq,
		Markdown: md,
		Keyboard: kb,
		MsgType:  2, //md信息
	}

	Message.Timestamp = time.Now().Unix() // 设置时间戳
	_, err := apiv2.PostGroupMessage(context.TODO(), data.GroupOpenID, Message)
	if err != nil {
		mylog.Printf("发送文本群组信息失败: %v", err)

		// 如果发送失败，检查是否是因为事件ID问题（包括40034025和11255错误码）
		if strings.Contains(err.Error(), `"code":40034025`) || strings.Contains(err.Error(), `"code":11255`) || strings.Contains(err.Error(), `"err_code":11255`) {
			mylog.Printf("EventID无效（错误码40034025或11255），尝试不使用EventID重新发送")
			Message.EventID = "" // 清空EventID
			_, err = apiv2.PostGroupMessage(context.TODO(), data.GroupOpenID, Message)
			if err != nil {
				mylog.Printf("再次发送失败: %v", err)
				return err
			}
			mylog.Printf("不使用EventID重新发送成功")
		} else {
			return err
		}
	}

	return nil
}

// autobind 函数接受 interface{} 类型的数据
// commit by 紫夜 2023-11-19
func (p *Processors) Autobind(data interface{}) error {
	var realID string
	var groupID string
	var attachmentURL string

	// 群at
	switch v := data.(type) {
	case *dto.WSGroupATMessageData:
		realID = v.Author.ID
		groupID = v.GroupID
		attachmentURL = v.Attachments[0].URL
		//群私聊
	case *dto.WSC2CMessageData:
		realID = v.Author.ID
		groupID = v.GroupID
		attachmentURL = v.Attachments[0].URL
	default:
		return fmt.Errorf("未知的数据类型")
	}

	// 从 URL 中提取 newRowValue (vuin)
	vuinRegex := regexp.MustCompile(`vuin=(\d+)`)
	vuinMatches := vuinRegex.FindStringSubmatch(attachmentURL)
	if len(vuinMatches) < 2 {
		mylog.Errorf("无法从 URL 中提取 vuin")
		return nil
	}
	vuinstr := vuinMatches[1]
	vuinValue, err := strconv.ParseInt(vuinMatches[1], 10, 64)
	if err != nil {
		return err
	}
	// 从 URL 中提取第二个 newRowValue (群号)
	idRegex := regexp.MustCompile(`gchatpic_new/(\d+)/`)
	idMatches := idRegex.FindStringSubmatch(attachmentURL)
	if len(idMatches) < 2 {
		mylog.Errorf("无法从 URL 中提取 ID")
		return nil
	}
	idValuestr := idMatches[1]
	idValue, err := strconv.ParseInt(idMatches[1], 10, 64)
	if err != nil {
		return err
	}
	var GroupID64, userid64 int64
	//获取虚拟值
	// 映射str的GroupID到int
	GroupID64, err = idmap.StoreIDv2(groupID)
	if err != nil {
		mylog.Errorf("failed to convert ChannelID to int: %v", err)
		return nil
	}
	// 映射str的userid到int
	userid64, err = idmap.StoreIDv2(realID)
	if err != nil {
		mylog.Printf("Error storing ID: %v", err)
		return nil
	}
	//覆盖赋值
	if config.GetIdmapPro() {
		//转换idmap-pro 虚拟值
		//将真实id转为int userid64
		GroupID64, userid64, err = idmap.StoreIDv2Pro(groupID, realID)
		if err != nil {
			mylog.Errorf("Error storing ID689: %v", err)
		}
	}
	// 单独检查vuin和gid的绑定状态
	vuinBound := strconv.FormatInt(userid64, 10) == vuinstr
	gidBound := strconv.FormatInt(GroupID64, 10) == idValuestr
	// 根据不同情况进行处理
	if !vuinBound && !gidBound {
		// 两者都未绑定，更新两个映射
		if err := updateMappings(userid64, vuinValue, GroupID64, idValue); err != nil {
			mylog.Printf("Error updateMappings for both: %v", err)
			//return err
		}
		// idmaps pro也更新
		err = idmap.UpdateVirtualValuev2Pro(GroupID64, idValue, userid64, vuinValue)
		if err != nil {
			mylog.Errorf("Error storing ID703: %v", err)
		}
	} else if !vuinBound {
		// 只有vuin未绑定，更新vuin映射
		if err := idmap.UpdateVirtualValuev2(userid64, vuinValue); err != nil {
			mylog.Printf("Error UpdateVirtualValuev2 for vuin: %v", err)
			//return err
		}
		// idmaps pro也更新,但只更新vuin
		idmap.UpdateVirtualValuev2Pro(GroupID64, idValue, userid64, vuinValue)
	} else if !gidBound {
		// 只有gid未绑定，更新gid映射
		if err := idmap.UpdateVirtualValuev2(GroupID64, idValue); err != nil {
			mylog.Printf("Error UpdateVirtualValuev2 for gid: %v", err)
			//return err
		}
		// idmaps pro也更新,但只更新gid
		idmap.UpdateVirtualValuev2Pro(GroupID64, idValue, userid64, vuinValue)
	} else {
		// 两者都已绑定，不执行任何操作
		mylog.Errorf("Both vuin and gid are already binded")
	}

	return nil
}

// 更新映射的辅助函数
func updateMappings(userid64, vuinValue, GroupID64, idValue int64) error {
	if err := idmap.UpdateVirtualValuev2(userid64, vuinValue); err != nil {
		mylog.Printf("Error UpdateVirtualValuev2 for vuin: %v", err)
		return err
	}
	if err := idmap.UpdateVirtualValuev2(GroupID64, idValue); err != nil {
		mylog.Printf("Error UpdateVirtualValuev2 for gid: %v", err)
		return err
	}
	return nil
}
