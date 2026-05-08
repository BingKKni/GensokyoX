// 处理收到的回调事件
package Processor

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/hoshinonyaruko/gensokyo/config"
	"github.com/hoshinonyaruko/gensokyo/echo"
	"github.com/hoshinonyaruko/gensokyo/handlers"
	"github.com/hoshinonyaruko/gensokyo/idmap"
	"github.com/hoshinonyaruko/gensokyo/mylog"
	"github.com/tencent-connect/botgo/dto"
	"github.com/tencent-connect/botgo/websocket/client"
)

// ProcessInlineSearch 处理内联查询
func (p *Processors) ProcessInlineSearch(data *dto.WSInteractionData, originalRaw []byte) error {
	// 转换appid
	var userid64 int64
	var GroupID64 int64
	var LongGroupID64 int64
	var LongUserID64 int64
	var err error
	var fromgid, fromuid string
	if data.GroupOpenID != "" {
		// 群聊场景
		fromgid = data.GroupOpenID
		fromuid = data.GroupMemberOpenID
	} else if data.UserOpenID != "" {
		// 私信场景（c2c）
		fromgid = ""              // 私信没有群 ID
		fromuid = data.UserOpenID // ← 关键修复：使用 UserOpenID
	} else {
		// 频道场景
		fromgid = data.ChannelID
		fromuid = data.GuildID
	}

	// 获取s
	s := client.GetGlobalS()
	originalPayload := parseOriginalPayload(originalRaw)
	// 转换appid
	AppIDString := strconv.FormatUint(p.Settings.AppID, 10)

	// 获取当前时间的13位毫秒级时间戳
	currentTimeMillis := time.Now().UnixNano() / 1e6

	// 构造echostr，包括AppID，原始的s变量和当前时间戳
	echostr := fmt.Sprintf("%s_%d_%d", AppIDString, s, currentTimeMillis)
	_ = echostr

	// 注释掉自动回应功能，改为手动回应
	// // 这里处理自动handle回调回应
	// if config.GetAutoPutInteraction() {
	// 	exceptions := config.GetPutInteractionExcept() // 会返回一个string[]，即例外列表
	//
	// 	shouldCall := true // 默认应该调用DelayedPutInteraction，除非下面的条件匹配
	//
	// 	// 判断，data.Data.Resolved.ButtonData 是否以返回的string[]中的任意成员开头
	// 	for _, prefix := range exceptions {
	// 		if strings.HasPrefix(data.Data.Resolved.ButtonData, prefix) {
	// 			shouldCall = false // 如果匹配到任何一个前缀，设置shouldCall为false
	// 			break              // 找到匹配项，无需继续检查
	// 		}
	// 	}
	//
	// 	// 如果data.Data.Resolved.ButtonData不以返回的string[]中的任意成员开头，
	// 	// 则调用DelayedPutInteraction，否则不调用
	// 	if shouldCall {
	// 		DelayedPutInteraction(p.Api, data.ID, fromuid, fromgid)
	// 	}
	// }

	if config.GetIdmapPro() {
		//将真实id转为int userid64
		GroupID64, userid64, err = idmap.StoreIDv2Pro(fromgid, fromuid)
		if err != nil {
			mylog.Errorf("Error storing ID: %v", err)
		}
		// 当哈希碰撞 因为获取时候是用的非idmap的get函数
		LongGroupID64, _ = idmap.StoreIDv2(fromgid)
		LongUserID64, _ = idmap.StoreIDv2(fromuid)
		if !config.GetHashIDValue() {
			mylog.Fatalf("避坑日志:你开启了高级id转换,请设置hash_id为true,并且删除idmaps并重启")
		}
	} else {
		// 映射str的GroupID到int
		GroupID64, err = idmap.StoreIDv2(fromgid)
		if err != nil {
			mylog.Errorf("failed to convert ChannelID to int: %v", err)
			return nil
		}
		// 映射str的userid到int
		userid64, err = idmap.StoreIDv2(fromuid)
		if err != nil {
			mylog.Printf("Error storing ID: %v", err)
			return nil
		}
		// 在非idmap-pro模式下，LongGroupID64和LongUserID64应该等于GroupID64和userid64
		LongGroupID64 = GroupID64
		LongUserID64 = userid64
	}
	if !config.GetGlobalInteractionToMessage() {
		notice := &OnebotInteractionNotice{
			GroupID:    GroupID64,
			NoticeType: "interaction",
			PostType:   "notice",
			SubType:    "create",
			UserID:     userid64,
			Data:       data,
			Original:   originalPayload,
		}
		//enhanced config
		notice.RealUserID = fromuid
		notice.RealGroupID = fromgid
		//debug
		PrintStructWithFieldNames(notice)

		// Convert OnebotGroupMessage to map and send
		noticeMap := structToMap(notice)

		//上报信息到onebotv11应用端(正反ws)
		go p.BroadcastMessageToAll(noticeMap, p.Apiv2, data)

		// 转换appid
		AppIDString := strconv.FormatUint(p.Settings.AppID, 10)

		// 储存和群号相关的eventid
		// idmap-pro的设计其实是有问题的,和idmap冲突,并且也还是会哈希碰撞 需要用一个不会碰撞的id去存
		echo.AddEvnetID(AppIDString, LongGroupID64, data.EventID)
	} else {
		if data.GroupOpenID != "" {
			//群回调
			newdata := ConvertInteractionToMessage(data)
			//mylog.Printf("回调测试111-newdata:%v\n", newdata)

			// 如果在Array模式下, 则处理Message为Segment格式
			var segmentedMessages interface{} = data.Data.Resolved.ButtonData
			if false { // 精简版：不使用Array模式
				segmentedMessages = handlers.ConvertToSegmentedMessage(newdata)
			}

			//映射str的messageID到int
			var messageID64 int64
			if config.GetMemoryMsgid() {
				messageID64, err = echo.StoreCacheInMemory(data.ID)
				if err != nil {
					log.Fatalf("Error storing ID: %v", err)
				}
			} else {
				messageID64, err = idmap.StoreCachev2(data.ID)
				if err != nil {
					log.Fatalf("Error storing ID: %v", err)
				}
			}

			messageID := int(messageID64)

			//mylog.Printf("回调测试-interaction:%v\n", segmentedMessages)
			groupMsg := OnebotGroupMessage{
				Message:       segmentedMessages,
				MessageID:     messageID,
				GroupID:       GroupID64,
				MessageType:   "group",
				PostType:      "message",
				UserID:        userid64,
				SubType:       "normal",
				Original:      originalPayload,
				InteractionID: data.ID, // 设置QQ平台下发的原始交互ID
			}
			//enhanced config
			groupMsg.RealMessageType = "interaction"
			groupMsg.RealGroupID = data.GroupOpenID
			groupMsg.RealUserID = data.GroupMemberOpenID

			// 添加msgID映射，确保应用端回复时能正确获取messageID
			// 将当前s和appid和message进行映射
			echo.AddMsgID(AppIDString, s, data.ID)
			// 映射消息类型
			echo.AddMsgType(AppIDString, s, "group")
			//为不支持双向echo的ob服务端映射
			echo.AddMsgID(AppIDString, GroupID64, data.ID)
			//将当前的userid和groupid和msgid进行一个更稳妥的映射
			echo.AddMsgIDv2(AppIDString, GroupID64, userid64, data.ID)

			//储存当前群或频道号的类型
			idmap.WriteConfigv2(fmt.Sprint(GroupID64), "type", "group")

			//映射类型
			echo.AddMsgType(AppIDString, GroupID64, "group")

			// 调试
			PrintStructWithFieldNames(groupMsg)

			// Convert OnebotGroupMessage to map and send
			groupMsgMap := structToMap(groupMsg)
			//上报信息到onebotv11应用端(正反ws)
			go p.BroadcastMessageToAll(groupMsgMap, p.Apiv2, data)

			// 转换appid
			AppIDString := strconv.FormatUint(p.Settings.AppID, 10)

			// 储存和群号相关的eventid
			fmt.Printf("测试:储存eventid:[%v]LongGroupID64[%v]\n", data.EventID, LongGroupID64)
			echo.AddEvnetID(AppIDString, LongGroupID64, data.EventID)

			// 上报事件
			notice := &OnebotInteractionNotice{
				GroupID:    GroupID64,
				NoticeType: "interaction",
				PostType:   "notice",
				SubType:    "create",
				UserID:     userid64,
				Data:       data,
				Original:   originalPayload,
			}
			//enhanced config
			notice.RealUserID = fromuid
			notice.RealGroupID = fromgid
			//debug
			PrintStructWithFieldNames(notice)

			// Convert OnebotGroupMessage to map and send
			noticeMap := structToMap(notice)

			//上报信息到onebotv11应用端(正反ws)
			go p.BroadcastMessageToAll(noticeMap, p.Apiv2, data)
		} else if data.UserOpenID != "" {
			//私聊回调
			newdata := ConvertInteractionToMessage(data)

			// 如果在Array模式下, 则处理Message为Segment格式
			var segmentedMessages interface{} = data.Data.Resolved.ButtonData
			if false { // 精简版：不使用Array模式
				segmentedMessages = handlers.ConvertToSegmentedMessage(newdata)
			}

			//平台事件,不是真实信息,无需messageID
			messageID64 := 123

			messageID := int(messageID64)
			privateMsg := OnebotPrivateMessage{
				Message:       segmentedMessages,
				MessageID:     messageID,
				MessageType:   "private",
				PostType:      "message",
				UserID:        userid64,
				SubType:       "friend",
				Original:      originalPayload,
				InteractionID: data.ID, // 设置QQ平台下发的原始交互ID
			}
			//enhanced config
			privateMsg.RealMessageType = "interaction"
			// 添加msgID映射，确保应用端回复时能正确获取messageID
			// 将当前s和appid和message进行映射
			echo.AddMsgID(AppIDString, s, data.ID)
			// 映射类型 对S映射
			echo.AddMsgType(AppIDString, s, "group_private")
			//为不支持双向echo的ob服务端映射
			echo.AddMsgID(AppIDString, userid64, data.ID)
			// 映射类型 对userid64映射
			echo.AddMsgType(AppIDString, userid64, "group_private")

			// 持久化储存当前用户的类型
			idmap.WriteConfigv2(fmt.Sprint(userid64), "type", "group_private")

			// 调试
			PrintStructWithFieldNames(privateMsg)

			// Convert OnebotGroupMessage to map and send
			privateMsgMap := structToMap(privateMsg)

			if data.Data.Resolved.ButtonData != "" {
				//上报信息到onebotv11应用端(正反ws)
				go p.BroadcastMessageToAll(privateMsgMap, p.Apiv2, data)
			}

			// 转换appid
			AppIDString := strconv.FormatUint(p.Settings.AppID, 10)

			// 储存和用户ID相关的eventid
			echo.AddEvnetID(AppIDString, LongUserID64, data.EventID)

			// 上报事件
			notice := &OnebotInteractionNotice{
				GroupID:    GroupID64,
				NoticeType: "interaction",
				PostType:   "notice",
				SubType:    "create",
				UserID:     userid64,
				Data:       data,
				Original:   originalPayload,
			}
			//enhanced config
			notice.RealUserID = fromuid
			notice.RealGroupID = fromgid
			//debug
			PrintStructWithFieldNames(notice)

			// Convert OnebotGroupMessage to map and send
			noticeMap := structToMap(notice)

			//上报信息到onebotv11应用端(正反ws)
			go p.BroadcastMessageToAll(noticeMap, p.Apiv2, data)
		} else {
			// TODO: 区分频道和频道私信 如果有人提需求
			// 频道回调
			// 处理onebot_channel_message逻辑
			newdata := ConvertInteractionToMessage(data)

			// 如果在Array模式下, 则处理Message为Segment格式
			var segmentedMessages interface{} = data.Data.Resolved.ButtonData
			if false { // 精简版：不使用Array模式
				segmentedMessages = handlers.ConvertToSegmentedMessage(newdata)
			}

			onebotMsg := OnebotChannelMessage{
				ChannelID:     data.ChannelID,
				GuildID:       data.GuildID,
				Message:       segmentedMessages,
				MessageID:     data.ID,
				MessageType:   "guild",
				PostType:      "message",
				UserID:        userid64,
				SelfTinyID:    "0",
				SubType:       "channel",
				Original:      originalPayload,
				InteractionID: data.ID, // 设置QQ平台下发的原始交互ID
			}
			//enhanced config
			onebotMsg.RealMessageType = "interaction"
			//debug
			PrintStructWithFieldNames(onebotMsg)

			// 将 onebotMsg 结构体转换为 map[string]interface{}
			msgMap := structToMap(onebotMsg)

			//上报信息到onebotv11应用端(正反ws)
			go p.BroadcastMessageToAll(msgMap, p.Apiv2, data)

			// TODO: 实现eventid
		}
	}

	return nil
}

// ConvertInteractionToMessage 转换 Interaction 到 Message
func ConvertInteractionToMessage(interaction *dto.WSInteractionData) *dto.Message {
	var message dto.Message

	// 直接映射的字段
	message.ID = interaction.ID
	message.ChannelID = interaction.ChannelID
	message.GuildID = interaction.GuildID
	message.GroupID = interaction.GroupOpenID

	// 特殊处理的字段
	message.Content = interaction.Data.Resolved.ButtonData
	message.DirectMessage = interaction.ChatType == 2

	return &message
}
