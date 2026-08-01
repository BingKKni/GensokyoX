package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/hoshinonyaruko/gensokyo/callapi"
	"github.com/hoshinonyaruko/gensokyo/config"
	"github.com/hoshinonyaruko/gensokyo/idmap"
	"github.com/hoshinonyaruko/gensokyo/mylog"
	"github.com/hoshinonyaruko/gensokyo/structs"
	"github.com/tencent-connect/botgo/dto"
	"github.com/tencent-connect/botgo/openapi"
)

var streamState = struct {
	sync.RWMutex
	indexByMessageID   map[string]int
	relatedByMessageID map[string]string
}{
	indexByMessageID:   make(map[string]int),
	relatedByMessageID: make(map[string]string),
}

func init() {
	callapi.RegisterHandler("send_private_msg_sse", HandleSendPrivateMsgSSE)
}

func incrementIndex(msgID string) int {
	streamState.Lock()
	defer streamState.Unlock()
	index, loaded := streamState.indexByMessageID[msgID]
	if !loaded {
		streamState.indexByMessageID[msgID] = 0
		return 0
	}
	newVal := index + 1
	streamState.indexByMessageID[msgID] = newVal
	return newVal
}

func clearStreamState(msgID string) {
	streamState.Lock()
	defer streamState.Unlock()
	delete(streamState.indexByMessageID, msgID)
	delete(streamState.relatedByMessageID, msgID)
}

func rollbackIndex(msgID string, failedIndex int) {
	streamState.Lock()
	defer streamState.Unlock()
	if streamState.indexByMessageID[msgID] != failedIndex {
		return
	}
	if failedIndex == 0 {
		delete(streamState.indexByMessageID, msgID)
		return
	}
	streamState.indexByMessageID[msgID] = failedIndex - 1
}

func UpdateRelatedID(MessageID, ID string) {
	streamState.Lock()
	defer streamState.Unlock()
	streamState.relatedByMessageID[MessageID] = ID
}

func GetRelatedID(MessageID string) string {
	streamState.RLock()
	defer streamState.RUnlock()
	return streamState.relatedByMessageID[MessageID]
}

func HandleSendPrivateMsgSSE(client callapi.Client, api openapi.OpenAPI, apiv2 openapi.OpenAPI, message callapi.ActionMessage) (string, error) {
	// 使用 message.Echo 作为key来获取消息类型
	var retmsg string

	// 检查UserID是否为0
	checkZeroUserID := func(id interface{}) bool {
		switch v := id.(type) {
		case int:
			return v != 0
		case int64:
			return v != 0
		case string:
			return v != "" && v != "0"
		default:
			return false
		}
	}

	// New checks for UserID and GroupID being nil or 0
	if message.Params.UserID == nil || !checkZeroUserID(message.Params.UserID) {
		return "", fmt.Errorf("send_private_msg_sse requires a valid user_id")
	}

	var err error

	var resp *dto.C2CMessageResponse

	var UserID string

	if message.Params.UserID != nil && len(message.Params.UserID.(string)) != 32 {
		//私聊信息
		if config.GetIdmapPro() {
			//还原真实的userid
			//mylog.Printf("group_private:%v", message.Params.UserID.(string))
			_, UserID, err = idmap.RetrieveRowByIDv2Pro("690426430", message.Params.UserID.(string))
			if err != nil {
				mylog.Printf("Error reading config: %v", err)
				return "", nil
			}
			mylog.Printf("测试,通过Proid获取的UserID:%v", UserID)
		} else {
			//还原真实的userid
			UserID, err = idmap.RetrieveRowByIDv2(message.Params.UserID.(string))
			if err != nil {
				mylog.Printf("Error reading config: %v", err)
				return "", nil
			}
		}
	} else {
		UserID = message.Params.UserID.(string)
	}

	// 首先，将message.Params.Message序列化成JSON字符串
	messageJSON, err := json.Marshal(message.Params.Message)
	if err != nil {
		mylog.Errorf("Error marshalling stream message: %v", err)
		return "", nil
	}

	// 然后，将这个JSON字符串反序列化到InterfaceBody类型的对象中
	var messageBody structs.InterfaceBody
	err = json.Unmarshal(messageJSON, &messageBody)
	if err != nil {
		mylog.Errorf("Error unmarshalling stream message: %v", err)
		return "", nil
	}
	if messageBody.State != 1 && messageBody.State != 10 && messageBody.State != 11 && messageBody.State != 20 {
		return "", fmt.Errorf("send_private_msg_sse received invalid state %d", messageBody.State)
	}

	// 显式平台消息 ID 可避开全局 lazy context，并保证并发流互不串线。
	messageID := paramString(message.Params.MessageID)
	if messageID != "" {
		messageID, err = resolveExplicitMessageID(messageID)
		if err != nil {
			return "", err
		}
	}

	// 如果messageID仍然为空，尝试使用config.GetAppID和UserID的组合来获取messageID
	if messageID == "" {
		messageID = GetMessageIDByUseridOrGroupid(config.GetAppIDStr(), UserID)
	}
	if messageID == "" {
		return "", fmt.Errorf("send_private_msg_sse requires a valid message_id")
	}

	// 获取并打印相关ID
	relatedID := GetRelatedID(messageID)
	isFirstFragment := relatedID == ""

	dtoSSE := generateMessageSSE(messageBody, messageID, relatedID)

	resp, err = apiv2.PostC2CMessageSSE(context.TODO(), UserID, dtoSSE)
	if err != nil {
		rollbackIndex(messageID, dtoSSE.Stream.Index)
		mylog.Errorf("发送文本私聊信息失败: %v", err)
		return "", err
	}

	// 更新或刷新映射关系
	UpdateRelatedID(messageID, resp.Message.ID)

	// 一个流只产生一条用户可见消息，因此只在首片记录一次统计。
	retmsg, _ = SendC2CStreamResponse(client, err, &message, resp, apiv2, isFirstFragment)
	if messageBody.State == 10 || messageBody.State == 20 {
		clearStreamState(messageID)
	}

	return retmsg, nil
}

func generateMessageSSE(body structs.InterfaceBody, msgID, ID string) *dto.MessageSSE {
	index := incrementIndex(msgID) // 获取并递增Index

	// 将InterfaceBody的PromptKeyboard转换为MessageSSE的结构
	var rows []dto.RowSSE
	for _, label := range body.PromptKeyboard {
		row := dto.RowSSE{
			Buttons: []dto.ButtonSSE{
				{
					RenderData: dto.RenderDataSSE{Label: label, Style: 2},
					Action:     dto.ActionSSE{Type: 2},
				},
			},
		}
		rows = append(rows, row)
	}

	var msgsse dto.MessageSSE

	if body.Content != "" {
		// 确保Markdown已经初始化
		msgsse.Markdown = &dto.MarkdownSSE{}
		msgsse.Markdown.Content = body.Content
	}

	if len(rows) > 0 {
		// 确保PromptKeyboard及其嵌套结构已经初始化
		msgsse.PromptKeyboard = &dto.KeyboardSSE{
			KeyboardContentSSE: dto.KeyboardContentSSE{
				Content: dto.ContentSSE{
					Rows: []dto.RowSSE{}, // 初始化空切片，避免nil切片赋值
				},
			},
		}
		msgsse.PromptKeyboard.KeyboardContentSSE.Content.Rows = rows
	}

	// 剩余字段赋值
	msgsse.MsgType = 2
	msgsse.MsgSeq = index + 10 // 防止seq重复被去重 预留10条信息供上下文发送
	msgsse.Stream = &dto.StreamSSE{
		State: body.State,
		Index: index,
		Reset: body.Reset,
	}

	if ID != "" {
		msgsse.Stream.ID = ID
	}
	if msgID != "" {
		msgsse.MsgID = msgID
	}

	// 初始化ActionButtonSSE，如果CallbackData有值
	if body.CallbackData != "" {
		msgsse.ActionButton = &dto.ActionButtonSSE{
			TemplateID:   body.ActionButton,
			CallbackData: body.CallbackData,
		}
	}

	return &msgsse

}
