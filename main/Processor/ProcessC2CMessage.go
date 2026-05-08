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
	"github.com/hoshinonyaruko/gensokyo/structs"
	"github.com/tencent-connect/botgo/dto"
	"github.com/tencent-connect/botgo/websocket/client"
)

// ProcessC2CMessage 处理C2C消息 群私聊
func (p *Processors) ProcessC2CMessage(data *dto.WSC2CMessageData, originalRaw []byte) error {
	originalPayload := parseOriginalPayload(originalRaw)

	// 从私信中提取必要的信息 这是测试回复需要用到
	//recipientID := data.Author.ID
	//ChannelID := data.ChannelID
	//sourece是源头频道
	//GuildID := data.GuildID

	if data.Author.ID == "" {
		mylog.Printf("出现ID为空未知错误.%v\n", data)
		return nil
	}

	//获取当前的s值 当前ws连接所收到的信息条数
	s := client.GetGlobalS()
	if !p.Settings.GlobalPrivateToChannel {
		// 直接转换成ob11私信

		//转换appidstring
		AppIDString := strconv.FormatUint(p.Settings.AppID, 10)
		// 获取当前时间的13位毫秒级时间戳
		currentTimeMillis := time.Now().UnixNano() / 1e6
		// 构造echostr，包括AppID，原始的s变量和当前时间戳
		echostr := fmt.Sprintf("%s_%d_%d", AppIDString, s, currentTimeMillis)
		_ = echostr
		var userid64 int64
		var err error
		if config.GetIdmapPro() {
			//将真实id转为int userid64
			_, userid64, err = idmap.StoreIDv2Pro("group_private", data.Author.ID)
			if err != nil {
				mylog.Errorf("Error storing ID: %v", err)
			}
			//当参数不全
			_, _ = idmap.StoreIDv2(data.Author.ID)
			if !config.GetHashIDValue() {
				mylog.Fatalf("避坑日志:你开启了高级id转换,请设置hash_id为true,并且删除idmaps并重启")
			}
			//补救措施
			idmap.SimplifiedStoreID(data.Author.ID)
			//补救措施
			echo.AddMsgIDv3(AppIDString, data.Author.ID, data.ID)
		} else {
			//将真实id转为int userid64
			userid64, err = idmap.StoreIDv2(data.Author.ID)
			if err != nil {
				mylog.Errorf("Error storing ID: %v", err)
			}
		}

		//收到私聊信息调用的具体还原步骤
		//1,idmap还原真实userid,
		//发信息使用的是userid
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
		if config.GetAutoBind() {
			if len(data.Attachments) > 0 && data.Attachments[0].URL != "" {
				p.Autobind(data)
			}
		}
		//转换at
		messageText := handlers.RevertTransformedText(data, "group_private", p.Api, p.Apiv2, userid64, userid64, true)
		if messageText == "" {
			mylog.Printf("信息被自定义黑白名单拦截")
			return nil
		}
		//框架内指令
		p.HandleFrameworkCommand(messageText, data, "group_private")
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
		privateMsg.RealMessageType = "group_private"
		privateMsg.RealUserID = data.Author.ID
		// 将当前s和appid和message进行映射
		echo.AddMsgID(AppIDString, s, data.ID)
		echo.AddMsgType(AppIDString, s, "group_private")
		//其实不需要用AppIDString,因为gensokyo是单机器人框架
		//可以试着开发一个,会很棒的
		echo.AddMsgID(AppIDString, userid64, data.ID)

		//懒message_id池
		echo.AddLazyMessageId(strconv.FormatInt(userid64, 10), data.ID, time.Now())

		//懒message_id池
		echo.AddLazyMessageId(data.Author.ID, data.ID, time.Now())

		//储存类型
		echo.AddMsgType(AppIDString, userid64, "group_private")
		//储存当前群或频道号的类型
		idmap.WriteConfigv2(fmt.Sprint(userid64), "type", "group_private")

		// 调试
		PrintStructWithFieldNames(privateMsg)

		// Convert OnebotGroupMessage to map and send
		privateMsgMap := structToMap(privateMsg)
		//上报信息到onebotv11应用端(正反ws)
		go p.BroadcastMessageToAll(privateMsgMap, p.Apiv2, data)
		//组合FriendData
		struserid := strconv.FormatInt(userid64, 10)
		userdata := structs.FriendData{
			Nickname: "",
			Remark:   "",
			UserID:   struserid,
		}
		//缓存私信好友列表
		idmap.StoreUserInfo(data.Author.ID, userdata)
	} else {
		//将私聊信息转化为群信息(特殊需求情况下)
		//转换appid
		AppIDString := strconv.FormatUint(p.Settings.AppID, 10)
		// 获取当前时间的13位毫秒级时间戳
		currentTimeMillis := time.Now().UnixNano() / 1e6
		// 构造echostr，包括AppID，原始的s变量和当前时间戳
		echostr := fmt.Sprintf("%s_%d_%d", AppIDString, s, currentTimeMillis)
		_ = echostr
		//把userid作为群号
		//映射str的userid到int
		var userid64 int64
		var err error
		var magic int64
		if config.GetIdmapPro() {
			//将真实id转为int userid64
			magic, userid64, err = idmap.StoreIDv2Pro("group_private", data.Author.ID)
			mylog.Printf("魔法数字:%v", magic) //690426430
			if err != nil {
				mylog.Errorf("Error storing ID: %v", err)
			}
			//当参数不全,降级时
			_, _ = idmap.StoreIDv2(data.Author.ID)
			//补救措施
			idmap.SimplifiedStoreID(data.Author.ID)
		} else {
			//将真实id转为int userid64
			userid64, err = idmap.StoreIDv2(data.Author.ID)
			if err != nil {
				mylog.Errorf("Error storing ID: %v", err)
			}
		}
		//转换at
		messageText := handlers.RevertTransformedText(data, "group_private", p.Api, p.Apiv2, userid64, userid64, true)
		if messageText == "" {
			mylog.Printf("信息被自定义黑白名单拦截")
			return nil
		}
		//框架内指令
		p.HandleFrameworkCommand(messageText, data, "group_private")
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

		groupMsg := OnebotGroupMessage{
			Message:     messageText,
			MessageID:   messageID,
			GroupID:     userid64,
			MessageType: "group",
			PostType:    "message",
			UserID:      userid64,
			SubType:     "normal",
			Original:    originalPayload,
		}
		//enhanced config
		groupMsg.RealMessageType = "group_private"
		groupMsg.RealUserID = data.Author.ID
		//将当前s和appid和message进行映射
		echo.AddMsgID(AppIDString, s, data.ID)
		echo.AddMsgType(AppIDString, s, "group_private")
		//为不支持双向echo的ob服务端映射
		echo.AddMsgID(AppIDString, userid64, data.ID)
		//映射类型
		echo.AddMsgType(AppIDString, userid64, "group_private")
		//储存当前群或频道号的类型
		idmap.WriteConfigv2(fmt.Sprint(userid64), "type", "group_private")

		//懒message_id池
		echo.AddLazyMessageId(strconv.FormatInt(userid64, 10), data.ID, time.Now())

		//懒message_id池
		echo.AddLazyMessageId(data.Author.ID, data.ID, time.Now())

		//调试
		PrintStructWithFieldNames(groupMsg)

		// Convert OnebotGroupMessage to map and send
		groupMsgMap := structToMap(groupMsg)
		//上报信息到onebotv11应用端(正反ws)
		go p.BroadcastMessageToAll(groupMsgMap, p.Apiv2, data)

		//组合FriendData
		struserid := strconv.FormatInt(userid64, 10)
		userdata := structs.FriendData{
			Nickname: "",
			Remark:   "",
			UserID:   struserid,
		}
		//缓存私信好友列表
		idmap.StoreUserInfo(data.Author.ID, userdata)
	}

	return nil
}
