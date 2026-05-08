package handlers

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hoshinonyaruko/gensokyo/callapi"
	"github.com/hoshinonyaruko/gensokyo/config"
	"github.com/hoshinonyaruko/gensokyo/echo"
	"github.com/hoshinonyaruko/gensokyo/idmap"
	"github.com/hoshinonyaruko/gensokyo/interactionwait"
	"github.com/hoshinonyaruko/gensokyo/mylog"
	"github.com/tencent-connect/botgo/dto"
	"github.com/tencent-connect/botgo/openapi"
)

func init() {
	callapi.RegisterHandler("send_private_msg", HandleSendPrivateMsg)
}

func HandleSendPrivateMsg(client callapi.Client, api openapi.OpenAPI, apiv2 openapi.OpenAPI, message callapi.ActionMessage) (string, error) {
	// 处理交互回应（按钮回调手动回应）
	if message.Params.InteractionID != nil && message.Params.InteractionID != "" {
		interactionID := fmt.Sprintf("%v", message.Params.InteractionID)

		// 获取回应代码，默认为0（成功）
		code := 0
		if message.Params.InteractionCode != nil {
			if intCode, ok := message.Params.InteractionCode.(int); ok {
				code = intCode
			}
		}

		// 优先把 code 投递给 webhook 等待中的 pending 槽位（webhook 模式下生效）；
		// 槽位不存在（websocket 模式 / 已超时 / 不在等待窗口内）则回退到平台 PUT API。
		if interactionwait.TryFill(interactionID, code) {
			mylog.Printf("发送私聊消息时通过webhook槽位回应交互: ID=%s, Code=%d", interactionID, code)
		} else {
			// 构造请求体
			requestBody := fmt.Sprintf(`{"code": %d}`, code)

			// 调用 PutInteraction API 进行手动回应
			ctx := context.Background()
			err := api.PutInteraction(ctx, interactionID, requestBody)
			if err != nil {
				mylog.Printf("发送私聊消息时手动回应交互失败: %v", err)
			} else {
				mylog.Printf("发送私聊消息时成功手动回应交互: ID=%s, Code=%d", interactionID, code)
			}
		}
	}

	// 使用 message.Echo 作为key来获取消息类型
	var msgType string
	var retmsg string
	if echoStr, ok := message.Echo.(string); ok {
		// 当 message.Echo 是字符串类型时执行此块
		msgType = echo.GetMsgTypeByKey(echoStr)
	}
	// 检查GroupID是否为0
	checkZeroGroupID := func(id interface{}) bool {
		switch v := id.(type) {
		case int:
			return v != 0
		case int64:
			return v != 0
		case string:
			return v != "0" // 检查字符串形式的0
		default:
			return true // 如果不是int、int64或string，假定它不为0
		}
	}

	// 检查UserID是否为0
	checkZeroUserID := func(id interface{}) bool {
		switch v := id.(type) {
		case int:
			return v != 0
		case int64:
			return v != 0
		case string:
			return v != "0" // 同样检查字符串形式的0
		default:
			return true // 如果不是int、int64或string，假定它不为0
		}
	}

	if message.Params.UserID != nil && len(message.Params.UserID.(string)) != 32 {
		if msgType == "" && message.Params.UserID != nil && checkZeroUserID(message.Params.UserID) {
			msgType = GetMessageTypeByUserid(config.GetAppIDStr(), message.Params.UserID)
		}
		if msgType == "" && message.Params.GroupID != nil && checkZeroGroupID(message.Params.GroupID) {
			msgType = GetMessageTypeByGroupid(config.GetAppIDStr(), message.Params.GroupID)
		}
		if msgType == "" && message.Params.UserID != nil && checkZeroUserID(message.Params.UserID) {
			msgType = GetMessageTypeByUseridV2(message.Params.UserID)
		}
		if msgType == "" && message.Params.GroupID != nil && checkZeroGroupID(message.Params.GroupID) {
			msgType = GetMessageTypeByGroupidV2(message.Params.GroupID)
		}
	}

	// New checks for UserID and GroupID being nil or 0
	if (message.Params.UserID == nil || !checkZeroUserID(message.Params.UserID)) &&
		(message.Params.GroupID == nil || !checkZeroGroupID(message.Params.GroupID)) {
		mylog.Printf("send_group_msgs接收到错误action: %v", message)
		return "", nil
	}

	var idInt64 int64
	var err error

	if message.Params.UserID != nil && len(message.Params.UserID.(string)) == 32 {
		idInt64, err = idmap.GenerateRowID(message.Params.UserID.(string), 9)
		// 临时的
		msgType = "group_private"
	} else {
		if message.Params.GroupID != "" {
			idInt64, err = ConvertToInt64(message.Params.GroupID)
		} else if message.Params.UserID != "" {
			idInt64, err = ConvertToInt64(message.Params.UserID)
		}
	}

	//设置递归 对直接向gsk发送action时有效果
	if msgType == "" {
		messageCopy := message
		if err != nil {
			mylog.Printf("错误：无法转换 ID %v\n", err)
		} else {
			// 递归3次
			echo.AddMapping(idInt64, 4)
			// 递归调用handleSendPrivateMsg，使用设置的消息类型
			echo.AddMsgType(config.GetAppIDStr(), idInt64, "group_private")
			HandleSendPrivateMsg(client, api, apiv2, messageCopy)
		}
	} else if echo.GetMapping(idInt64) <= 0 {
		// 特殊值代表不递归
		echo.AddMapping(idInt64, 10)
	}

	var resp *dto.C2CMessageResponse
	switch msgType {
	//这里是pr上来的,我也不明白为什么私聊会出现group类型 猜测是为了匹配包含了groupid的私聊?
	case "group_private", "group":
		//私聊信息
		var UserID string
		if len(message.Params.UserID.(string)) != 32 {
			if config.GetIdmapPro() {
				//还原真实的userid
				_, UserID, err = idmap.RetrieveRowByIDv2Pro("690426430", message.Params.UserID.(string))
				if err != nil {
					mylog.Printf("Error reading config: %v", err)
					return "", nil
				}
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

		// 解析消息内容
		messageText, foundItems := parseMessageContent(message.Params, message, client, api, apiv2)

		// 使用 echo 获取消息ID
		var messageID string
		// EventID
		var eventID string
		// is_wakeup 字段
		isWakeup := message.Params.IsWakeup

		if config.GetLazyMessageId() {
			//由于实现了Params的自定义unmarshell 所以可以类型安全的断言为string
			messageID = echo.GetLazyMessagesId(UserID)
		}
		if messageID == "" {
			if echoStr, ok := message.Echo.(string); ok {
				messageID = echo.GetMsgIDByKey(echoStr)
			}
		}
		// 如果messageID仍然为空，尝试使用config.GetAppID和UserID的组合来获取messageID
		// 如果messageID为空，通过函数获取
		if messageID == "" {
			messageID = GetMessageIDByUseridOrGroupid(config.GetAppIDStr(), UserID)
		}
		if messageID == "2000" {
			messageID = ""
			mylog.Println("通过lazymsgid发送群私聊主动信息,每月可发送1次")
			if len(message.Params.UserID.(string)) != 32 {
				eventID = GetEventIDByUseridOrGroupid(config.GetAppIDStr(), message.Params.UserID)
			} else {
				eventID = GetEventIDByUseridOrGroupidv2(config.GetAppIDStr(), message.Params.UserID)
			}
		}

		// 当 is_wakeup 为 true 时，清空 messageID 和 eventID
		if isWakeup {
			mylog.Printf("检测到 is_wakeup 被设置为真，该消息应为互动召回消息")
			messageID = ""
			eventID = ""
		}

		//开发环境用 私聊不可用1000
		// if config.GetDevMsgID() {
		// 	messageID = "1000"
		// }
		mylog.Println("私聊发信息messageText:", messageText)
		//mylog.Println("foundItems:", foundItems)

		var singleItem = make(map[string][]string)
		var imageType, imageUrl string
		imageCount := 0

		// 检查不同类型的图片并计算数量
		if imageURLs, ok := foundItems["local_image"]; ok && len(imageURLs) == 1 {
			imageType = "local_image"
			imageUrl = imageURLs[0]
			imageCount++
		} else if imageURLs, ok := foundItems["url_image"]; ok && len(imageURLs) == 1 {
			imageType = "url_image"
			imageUrl = imageURLs[0]
			imageCount++
		} else if base64Images, ok := foundItems["base64_image"]; ok && len(base64Images) == 1 {
			imageType = "base64_image"
			imageUrl = base64Images[0]
			imageCount++
		}

		if imageCount == 1 && messageText != "" {
			// 创建包含单个图片的 singleItem
			singleItem[imageType] = []string{imageUrl}
			msgseq := echo.GetMappingSeq(messageID)
			echo.AddMappingSeq(messageID, msgseq+1)
			groupReply := generatePrivateMessage(messageID, eventID, singleItem, "", msgseq+1, apiv2, UserID, isWakeup)
			// 进行类型断言
			richMediaMessage, ok := groupReply.(*dto.RichMediaMessage)
			if !ok {
				// mylog.Printf("Error: Expected RichMediaMessage type for key ")
				return "", nil
			}
			// 上传图片并获取FileInfo
			fileInfo, err := uploadMediaPrivate(context.TODO(), UserID, richMediaMessage, apiv2)
			if err != nil {
				mylog.Printf("上传图片失败: %v", err)
				return "", nil // 或其他错误处理
			}
			// 创建包含文本和图像信息的消息
			msgseq = echo.GetMappingSeq(messageID)
			echo.AddMappingSeq(messageID, msgseq+1)
			groupMessage := &dto.MessageToCreate{
				Content: messageText, // 添加文本内容
				Media: dto.Media{
					FileInfo: fileInfo, // 添加图像信息
				},
				MsgID:    messageID,
				EventID:  eventID,
				MsgSeq:   msgseq,
				MsgType:  7,        // 假设7是组合消息类型
				IsWakeup: isWakeup, // 设置召回消息标识
			}
			groupMessage.Timestamp = time.Now().Unix() // 设置时间戳

			// 发送组合消息（带 token 过期重试）
			resp, err = postC2CMessageWithRetry(apiv2, UserID, groupMessage)
			if err != nil {
				mylog.Printf("发送组合消息失败: %v", err)
				return "", nil // 或其他错误处理
			}

			// 发送成功回执
			retmsg, _ = SendC2CResponse(client, err, &message, resp, apiv2)

			delete(foundItems, imageType) // 从foundItems中删除已处理的图片项
			messageText = ""
		}

		// 优先发送文本信息
		if messageText != "" {
			msgseq := echo.GetMappingSeq(messageID)
			echo.AddMappingSeq(messageID, msgseq+1)
			groupReply := generatePrivateMessage(messageID, eventID, nil, messageText, msgseq+1, apiv2, UserID, isWakeup)

			// 进行类型断言
			groupMessage, ok := groupReply.(*dto.MessageToCreate)
			if !ok {
				mylog.Println("Error: Expected MessageToCreate type.")
				return "", nil
			}

			groupMessage.Timestamp = time.Now().Unix() // 设置时间戳
			// 带 token 过期重试的文本私聊发送
			resp, err := postC2CMessageWithRetry(apiv2, UserID, groupMessage)
			if err != nil {
				mylog.Printf("发送文本私聊信息失败: %v", err)
				//如果失败 防止进入递归
				return "", nil
			}
			//发送成功回执
			retmsg, _ = SendC2CResponse(client, err, &message, resp, apiv2)
		}

		// 遍历foundItems并发送每种信息
		for key, urls := range foundItems {
			for _, url := range urls {
				var singleItem = make(map[string][]string)
				singleItem[key] = []string{url} // 创建一个只包含一个 URL 的 singleItem
				//mylog.Println("singleItem:", singleItem)
				msgseq := echo.GetMappingSeq(messageID)
				echo.AddMappingSeq(messageID, msgseq+1)
				groupReply := generatePrivateMessage(messageID, eventID, singleItem, "", msgseq+1, apiv2, UserID, isWakeup)
				// 进行类型断言
				richMediaMessage, ok := groupReply.(*dto.RichMediaMessage)
				if !ok {
					// mylog.Printf("Error: Expected RichMediaMessage type for key %s.", key)
					// 定义一个map来存储关键字
					keyMap := map[string]bool{
						"markdown":      true,
						"qqmusic":       true,
						"local_image":   true,
						"local_record":  true,
						"url_image":     true,
						"url_images":    true,
						"base64_record": true,
						"base64_image":  true,
					}
					// key是 for key, urls := range foundItems { 这里的key
					if _, exists := keyMap[key]; exists {
						// 进行类型断言
						groupMessage, ok := groupReply.(*dto.MessageToCreate)
						if !ok {
							mylog.Println("Error: Expected MessageToCreate type.")
							return "", nil // 或其他错误处理
						}
						//重新为err赋值
						resp, err = apiv2.PostC2CMessage(context.TODO(), UserID, groupMessage)
						if err != nil {
							mylog.Printf("发送 MessageToCreate 私聊信息失败: %v", err)
							// 错误保存到本地
							if config.GetSaveError() {
								mylog.ErrLogToFile("type", "PostGroupMessage")
								mylog.ErrInterfaceToFile("request", groupMessage)
								mylog.ErrLogToFile("error", err.Error())
							}
						}
						if err != nil && strings.Contains(err.Error(), `"code":22009`) {
							mylog.Printf("私信主动转被动待实现")
							// var pair echo.MessageGroupPair
							// pair.Group = message.Params.GroupID.(string)
							// pair.GroupMessage = groupMessage
							// echo.PushGlobalStack(pair)
						} else if err != nil && strings.Contains(err.Error(), `"code":40034025`) {
							//请求参数event_id无效 重试
							groupMessage.EventID = ""
							//重新为err赋值
							resp, err = apiv2.PostC2CMessage(context.TODO(), UserID, groupMessage)
							if err != nil {
								mylog.Printf("发送 MessageToCreate 私聊信息失败 on code 40034025: %v", err)
								// 错误保存到本地
								if config.GetSaveError() {
									mylog.ErrLogToFile("type", "PostGroupMessage")
									mylog.ErrInterfaceToFile("request", groupMessage)
									mylog.ErrLogToFile("error", err.Error())
								}
							}
						}
						//发送成功回执
						retmsg, _ = SendC2CResponse(client, err, &message, resp, apiv2)
					}
					continue // 跳过这个项，继续下一个
				}
				message_return, err := apiv2.PostC2CMessage(context.TODO(), UserID, richMediaMessage)
				if err != nil {
					mylog.Printf("发送 %s 信息失败_send_private_msg: %v", key, err)
					if config.GetSendError() { //把报错当作文本发出去
						msgseq := echo.GetMappingSeq(messageID)
						echo.AddMappingSeq(messageID, msgseq+1)
						groupReply := generatePrivateMessage(messageID, eventID, nil, err.Error(), msgseq+1, apiv2, UserID, isWakeup)
						// 进行类型断言
						groupMessage, ok := groupReply.(*dto.MessageToCreate)
						if !ok {
							mylog.Println("Error: Expected MessageToCreate type.")
							return "", nil // 或其他错误处理
						}
						groupMessage.Timestamp = time.Now().Unix() // 设置时间戳
						//重新为err赋值
						_, err = apiv2.PostC2CMessage(context.TODO(), UserID, groupMessage)
						if err != nil {
							mylog.Printf("发送 %s 私聊信息失败: %v", key, err)
						}
					}
				}
				if message_return != nil && message_return.MediaResponse != nil && message_return.MediaResponse.FileInfo != "" {
					msgseq := echo.GetMappingSeq(messageID)
					echo.AddMappingSeq(messageID, msgseq+1)
					media := dto.Media{
						FileInfo: message_return.MediaResponse.FileInfo,
					}
					groupMessage := &dto.MessageToCreate{
						Content:  " ",
						MsgID:    messageID,
						EventID:  eventID,
						MsgSeq:   msgseq,
						MsgType:  7, // 默认文本类型
						Media:    media,
						IsWakeup: isWakeup, // 设置召回消息标识
					}
					groupMessage.Timestamp = time.Now().Unix() // 设置时间戳
					//重新为err赋值
					resp, err = apiv2.PostC2CMessage(context.TODO(), UserID, groupMessage)
					if err != nil {
						mylog.Printf("发送 %s 私聊信息失败: %v", key, err)
					}
				}
				//发送成功回执
				retmsg, _ = SendC2CResponse(client, err, &message, resp, apiv2)
			}
		}
		//这里是pr上来的,我也不明白为什么私聊会出现guild类型
	case "guild_private", "guild":
		//当收到发私信调用 并且来源是频道
		retmsg, _ = HandleSendGuildChannelPrivateMsg(client, api, apiv2, message, nil, nil)
	default:
		mylog.Printf("Unknown message type: %s", msgType)
	}

	// 如果递归id不是10(不递归特殊值)
	if echo.GetMapping(idInt64) != 10 {
		//重置递归类型
		if echo.GetMapping(idInt64) <= 0 {
			echo.AddMsgType(config.GetAppIDStr(), idInt64, "")
		}
		echo.AddMapping(idInt64, echo.GetMapping(idInt64)-1)

		//递归3次枚举类型
		if echo.GetMapping(idInt64) > 0 {
			tryMessageTypes := []string{"group", "guild", "guild_private"}
			messageCopy := message // 创建message的副本
			echo.AddMsgType(config.GetAppIDStr(), idInt64, tryMessageTypes[echo.GetMapping(idInt64)-1])
			delay := config.GetSendDelay()
			time.Sleep(time.Duration(delay) * time.Millisecond)
			retmsg, _ = HandleSendPrivateMsg(client, api, apiv2, messageCopy)
		}
	}

	return retmsg, nil
}

// 这个函数可以通过int类型的虚拟userid反推真实的guild_id和channel_id
func getGuildIDFromMessage(message callapi.ActionMessage) (string, string, error) {
	var userID string

	// 判断UserID的类型，并将其转换为string
	switch v := message.Params.UserID.(type) {
	case int:
		userID = strconv.Itoa(v)
	case float64:
		userID = strconv.FormatInt(int64(v), 10) // 将float64先转为int64，然后再转为string
	case string:
		userID = v
	default:
		return "", "", fmt.Errorf("unexpected type for UserID: %T", v) // 使用%T来打印具体的类型
	}
	var realUserID string
	var err error
	// 使用RetrieveRowByIDv2还原真实的UserID
	realUserID, err = idmap.RetrieveRowByIDv2(userID)
	if err != nil {
		return "", "", fmt.Errorf("error retrieving real UserID: %v", err)
	}
	// 使用realUserID作为sectionName从数据库中获取channel_id
	channelID, err := idmap.ReadConfigv2(realUserID, "channel_id")
	if err != nil {
		return "", "", fmt.Errorf("error reading channel_id: %v", err)
	}
	//使用channelID作为sectionName从数据库中获取guild_id
	guildID, err := idmap.ReadConfigv2(channelID, "guild_id")
	if err != nil {
		return "", "", fmt.Errorf("error reading guild_id: %v", err)
	}

	return guildID, channelID, nil
}

// 这个函数可以通过int类型的虚拟groupid反推真实的guild_id和channel_id
func getGuildIDFromMessagev2(message callapi.ActionMessage) (string, string, error) {
	var GroupID string
	//groupID此时是转换后的channelid

	// 判断UserID的类型，并将其转换为string
	switch v := message.Params.GroupID.(type) {
	case int:
		GroupID = strconv.Itoa(v)
	case float64:
		GroupID = strconv.FormatInt(int64(v), 10) // 将float64先转为int64，然后再转为string
	case string:
		GroupID = v
	default:
		return "", "", fmt.Errorf("unexpected type for UserID: %T", v) // 使用%T来打印具体的类型
	}

	var err error
	//使用channelID作为sectionName从数据库中获取guild_id
	guildID, err := idmap.ReadConfigv2(GroupID, "guild_id")
	if err != nil {
		return "", "", fmt.Errorf("error reading guild_id: %v", err)
	}

	return guildID, GroupID, nil
}

// uploadMedia 上传媒体并返回FileInfo
func uploadMediaPrivate(ctx context.Context, UserID string, richMediaMessage *dto.RichMediaMessage, apiv2 openapi.OpenAPI) (string, error) {
	// 调用API来上传媒体
	messageReturn, err := apiv2.PostC2CMessage(ctx, UserID, richMediaMessage)
	if err != nil {
		return "", err
	}
	// 返回上传后的FileInfo
	return messageReturn.MediaResponse.FileInfo, nil
}

// postC2CMessageWithRetry 带重试的私聊消息发送，接受 dto.APIMessage 接口
// 在 token 过期（err_code:11244）时等待 3 秒后重试，超时时等待 1 秒后重试
func postC2CMessageWithRetry(apiv2 openapi.OpenAPI, userID string, message dto.APIMessage) (resp *dto.C2CMessageResponse, err error) {
	const maxRetry = 3
	for i := 0; i < maxRetry; i++ {
		resp, err = apiv2.PostC2CMessage(context.TODO(), userID, message)
		if err == nil {
			return resp, nil
		}
		if isTokenExpireError(err) {
			// token 正在后台刷新，等待3秒后重试
			mylog.Printf("对私聊: %v 发送消息失败：token not exist or expire. 尝试重试...", userID)
			time.Sleep(3 * time.Second)
			continue
		}
		if strings.Contains(err.Error(), "context deadline exceeded") {
			// 请求超时，等待1秒后重试
			mylog.Printf("对私聊: %v 发送消息超时，重试第 %d 次", userID, i+1)
			time.Sleep(1 * time.Second)
			continue
		}
		// 其他错误（如 22009 权限不足）不重试，直接返回
		break
	}
	return resp, err
}
