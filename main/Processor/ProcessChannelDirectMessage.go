// 处理收到的信息事件
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

// ProcessChannelDirectMessage 处理频道私信消息 这里我们是被动收到
func (p *Processors) ProcessChannelDirectMessage(data *dto.WSDirectMessageData, originalRaw []byte) error {
	// 打印data结构体
	//PrintStructWithFieldNames(data)
	originalPayload := parseOriginalPayload(originalRaw)

	// 从私信中提取必要的信息 这是测试回复需要用到
	//recipientID := data.Author.ID
	//ChannelID := data.ChannelID
	//sourece是源头频道
	//GuildID := data.GuildID

	//获取当前的s值 当前ws连接所收到的信息条数
	s := client.GetGlobalS()
	if !p.Settings.GlobalPrivateToChannel {
		// 把频道类型的私信转换成普通ob11的私信

		//转换appidstring
		AppIDString := strconv.FormatUint(p.Settings.AppID, 10)
		// 获取当前时间的13位毫秒级时间戳
		currentTimeMillis := time.Now().UnixNano() / 1e6
		// 构造echostr，包括AppID，原始的s变量和当前时间戳
		echostr := fmt.Sprintf("%s_%d_%d", AppIDString, s, currentTimeMillis)
		_ = echostr

		var userid64 int64
		var ChannelID64 int64
		var err error
		if config.GetIdmapPro() {
			//将真实id转为int userid64
			_, _, err = idmap.StoreIDv2Pro(data.ChannelID, data.Author.ID)
			if err != nil {
				mylog.Errorf("Error storing ID: %v", err)
			}
			//将真实id转为int userid64
			userid64, err = idmap.StoreIDv2(data.Author.ID)
			if err != nil {
				mylog.Errorf("Error storing ID: %v", err)
			}
			ChannelID64, err = idmap.StoreIDv2(data.ChannelID)
			if err != nil {
				mylog.Printf("Error storing ID: %v", err)
				return nil
			}
			if !config.GetHashIDValue() {
				mylog.Fatalf("避坑日志:你开启了高级id转换,请设置hash_id为true,并且删除idmaps并重启")
			}
			//补救措施
			idmap.SimplifiedStoreID(data.Author.ID)
			//补救措施
			idmap.SimplifiedStoreID(data.ChannelID)
			//补救措施
			echo.AddMsgIDv3(AppIDString, data.ChannelID, data.ID)
			//补救措施
			echo.AddMsgIDv3(AppIDString, data.Author.ID, data.ID)
		} else {
			//将真实id转为int userid64
			userid64, err = idmap.StoreIDv2(data.Author.ID)
			if err != nil {
				mylog.Errorf("Error storing ID: %v", err)
			}
			//将channelid写入数据库,可取出guild_id
			ChannelID64, err = idmap.StoreIDv2(data.ChannelID)
			if err != nil {
				mylog.Printf("Error storing ID: %v", err)
				return nil
			}
		}
		//将真实id写入数据库,可取出ChannelID
		idmap.WriteConfigv2(data.Author.ID, "channel_id", data.ChannelID)
		//转成int再互转
		idmap.WriteConfigv2(fmt.Sprint(ChannelID64), "guild_id", data.GuildID)
		//直接储存 适用于私信场景私聊
		idmap.WriteConfigv2(data.ChannelID, "guild_id", data.GuildID)
		//收到私聊信息调用的具体还原步骤
		//1,idmap还原真实userid,
		//2,通过idmap获取channelid,
		//3,通过idmap用channelid获取guildid,
		//发信息使用的是guildid
		//todo 优化数据库读写次数
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
		//转换at
		messageText := handlers.RevertTransformedText(data, "guild_private", p.Api, p.Apiv2, userid64, userid64, true)
		if messageText == "" {
			mylog.Printf("信息被自定义黑白名单拦截")
			return nil
		}
		//框架内指令
		p.HandleFrameworkCommand(messageText, data, "guild_private")
		// 如果在Array模式下, 则处理Message为Segment格式
		var segmentedMessages interface{} = messageText

		privateMsg := OnebotPrivateMessage{
			Message:     segmentedMessages,
			MessageID:   messageID,
			MessageType: "private",
			PostType:    "message",
			UserID:      userid64,
			SubType:     "friend",
			Original:    originalPayload,
		}
		//enhanced config
		privateMsg.RealMessageType = "guild_private"
		// 将当前s和appid和message进行映射
		echo.AddMsgID(AppIDString, s, data.ID)
		echo.AddMsgType(AppIDString, s, "guild_private")
		//其实不需要用AppIDString,因为gensokyo是单机器人框架
		echo.AddMsgID(AppIDString, userid64, data.ID)
		echo.AddMsgType(AppIDString, userid64, "guild_private")
		//储存当前群或频道号的类型
		idmap.WriteConfigv2(fmt.Sprint(userid64), "type", "guild_private")
		//懒message_id池
		echo.AddLazyMessageId(strconv.FormatInt(userid64, 10), data.ID, time.Now())

		// 调试
		PrintStructWithFieldNames(privateMsg)

		// Convert OnebotGroupMessage to map and send
		privateMsgMap := structToMap(privateMsg)
		//上报信息到onebotv11应用端(正反ws)
		go p.BroadcastMessageToAll(privateMsgMap, p.Apiv2, data)
	} else {
		if !p.Settings.GlobalChannelToGroup {
			//将频道私信作为普通频道信息

			//获取s
			s := client.GetGlobalS()
			//转换at
			messageText := handlers.RevertTransformedText(data, "guild_private", p.Api, p.Apiv2, 10000, 10000, true) //todo 这里未转换
			if messageText == "" {
				mylog.Printf("信息被自定义黑白名单拦截")
				return nil
			}
			//框架内指令
			p.HandleFrameworkCommand(messageText, data, "guild_private")
			//转换appid
			AppIDString := strconv.FormatUint(p.Settings.AppID, 10)
			// 获取当前时间的13位毫秒级时间戳
			currentTimeMillis := time.Now().UnixNano() / 1e6
			// 构造echostr，包括AppID，原始的s变量和当前时间戳
			echostr := fmt.Sprintf("%s_%d_%d", AppIDString, s, currentTimeMillis)
			_ = echostr
			//映射str的userid到int
			userid64, err := idmap.StoreIDv2(data.Author.ID)
			if err != nil {
				mylog.Printf("Error storing ID: %v", err)
				return nil
			}
			//OnebotChannelMessage
			onebotMsg := OnebotChannelMessage{
				ChannelID:   data.ChannelID,
				GuildID:     data.GuildID,
				Message:     messageText,
				MessageID:   data.ID,
				MessageType: "guild",
				PostType:    "message",
				UserID:      userid64,
				SelfTinyID:  "",
				SubType:     "channel",
				Original:    originalPayload,
			}
			//将当前s和appid和message进行映射
			echo.AddMsgID(AppIDString, s, data.ID)
			//通过echo始终得知真实的事件类型,来对应调用正确的api
			echo.AddMsgType(AppIDString, s, "guild_private")
			//为不支持双向echo的ob服务端映射
			echo.AddMsgID(AppIDString, userid64, data.ID)
			echo.AddMsgType(AppIDString, userid64, "guild_private")
			//储存当前群或频道号的类型
			idmap.WriteConfigv2(data.ChannelID, "type", "guild_private")
			//储存当前群或频道号的类型
			idmap.WriteConfigv2(fmt.Sprint(userid64), "type", "guild_private")
			//todo 完善频道类型信息转换
			//懒message_id池
			echo.AddLazyMessageId(strconv.FormatInt(userid64, 10), data.ID, time.Now())

			//调试
			PrintStructWithFieldNames(onebotMsg)

			// 将 onebotMsg 结构体转换为 map[string]interface{}
			msgMap := structToMap(onebotMsg)
			//上报信息到onebotv11应用端(正反ws)
			go p.BroadcastMessageToAll(msgMap, p.Apiv2, data)
		} else {
			//将频道信息转化为群信息(特殊需求情况下)
			//将channelid写入bolt,可取出guild_id
			var userid64 int64
			var ChannelID64 int64
			var err error
			if config.GetIdmapPro() {
				//将真实id转为int userid64
				ChannelID64, userid64, err = idmap.StoreIDv2Pro(data.ChannelID, data.Author.ID)
				if err != nil {
					mylog.Errorf("Error storing ID: %v", err)
				}
				//将真实id转为int userid64
				_, err = idmap.StoreIDv2(data.Author.ID)
				if err != nil {
					mylog.Errorf("Error storing ID: %v", err)
				}
				_, err = idmap.StoreIDv2(data.ChannelID)
				if err != nil {
					mylog.Printf("Error storing ID: %v", err)
					return nil
				}
				if !config.GetHashIDValue() {
					mylog.Fatalf("避坑日志:你开启了高级id转换,请设置hash_id为true,并且删除idmaps并重启")
				}
				//补救措施
				idmap.SimplifiedStoreID(data.Author.ID)
				//补救措施
				idmap.SimplifiedStoreID(data.ChannelID)
			} else {
				//将真实id转为int userid64
				userid64, err = idmap.StoreIDv2(data.Author.ID)
				if err != nil {
					mylog.Errorf("Error storing ID: %v", err)
				}
				//将真实channelid和虚拟做映射
				ChannelID64, err = idmap.StoreIDv2(data.ChannelID)
				if err != nil {
					mylog.Printf("Error storing ID: %v", err)
					return nil
				}
			}
			//转成int再互转 适用于群场景私聊
			idmap.WriteConfigv2(fmt.Sprint(ChannelID64), "guild_id", data.GuildID)
			//直接储存 适用于私信场景私聊
			idmap.WriteConfigv2(data.ChannelID, "guild_id", data.GuildID)
			//转换at
			messageText := handlers.RevertTransformedText(data, "guild_private", p.Api, p.Apiv2, userid64, userid64, true)
			if messageText == "" {
				mylog.Printf("信息被自定义黑白名单拦截")
				return nil
			}
			//框架内指令
			p.HandleFrameworkCommand(messageText, data, "guild_private")
			//转换appid
			AppIDString := strconv.FormatUint(p.Settings.AppID, 10)
			// 获取当前时间的13位毫秒级时间戳
			currentTimeMillis := time.Now().UnixNano() / 1e6
			// 构造echostr，包括AppID，原始的s变量和当前时间戳
			echostr := fmt.Sprintf("%s_%d_%d", AppIDString, s, currentTimeMillis)
			_ = echostr

			//userid := int(userid64)
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
			// 如果在Array模式下, 则处理Message为Segment格式
			var segmentedMessages interface{} = messageText
			if false { // 精简版：不使用Array模式
				segmentedMessages = handlers.ConvertToSegmentedMessage(data)
			}

			groupMsg := OnebotGroupMessage{
				Message:     segmentedMessages,
				MessageID:   messageID,
				GroupID:     ChannelID64,
				MessageType: "group",
				PostType:    "message",
				UserID:      userid64,
				SubType:     "normal",
				Original:    originalPayload,
			}
			//enhanced config
			groupMsg.RealMessageType = "guild_private"
			//将当前s和appid和message进行映射
			echo.AddMsgID(AppIDString, s, data.ID)
			echo.AddMsgType(AppIDString, s, "guild_private")
			//为不支持双向echo的ob服务端映射
			echo.AddMsgID(AppIDString, userid64, data.ID)
			//为频道私聊转群聊映射
			echo.AddMsgID(AppIDString, ChannelID64, data.ID)
			//将当前的userid和groupid和msgid进行一个更稳妥的映射
			echo.AddMsgIDv2(AppIDString, ChannelID64, userid64, data.ID)
			//映射类型
			echo.AddMsgType(AppIDString, userid64, "guild_private")
			//储存当前群或频道号的类型
			idmap.WriteConfigv2(fmt.Sprint(ChannelID64), "type", "guild_private")
			echo.AddMsgType(AppIDString, ChannelID64, "guild_private")
			//懒message_id池
			echo.AddLazyMessageId(strconv.FormatInt(userid64, 10), data.ID, time.Now())

			//调试
			PrintStructWithFieldNames(groupMsg)

			// Convert OnebotGroupMessage to map and send
			groupMsgMap := structToMap(groupMsg)
			//上报信息到onebotv11应用端(正反ws)
			go p.BroadcastMessageToAll(groupMsgMap, p.Apiv2, data)
		}

	}
	return nil
}
