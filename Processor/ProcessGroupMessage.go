// 处理收到的信息事件
package Processor

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/hoshinonyaruko/gensokyo/config"
	"github.com/hoshinonyaruko/gensokyo/echo"
	"github.com/hoshinonyaruko/gensokyo/handlers"
	"github.com/hoshinonyaruko/gensokyo/idmap"
	"github.com/hoshinonyaruko/gensokyo/mylog"

	"github.com/tencent-connect/botgo/dto"
	"github.com/tencent-connect/botgo/websocket/client"
)

// ProcessGroupMessage 处理群组消息
func (p *Processors) ProcessGroupMessage(data *dto.WSGroupATMessageData, originalRaw []byte) error {
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

	var userid64 int64
	var GroupID64 int64
	var err error

	if data.Author.ID == "" {
		mylog.Printf("出现ID为空未知错误.%v\n", data)
		return nil
	}

	if config.GetIdmapPro() {
		//将真实id转为int userid64
		GroupID64, userid64, err = idmap.StoreIDv2Pro(data.GroupID, data.Author.ID)
		if err != nil {
			mylog.Errorf("Error storing ID: %v", err)
		}
		//当参数不全
		_, _ = idmap.StoreIDv2(data.GroupID)
		_, _ = idmap.StoreIDv2(data.Author.ID)
		if !config.GetHashIDValue() {
			mylog.Fatalf("避坑日志:你开启了高级id转换,请设置hash_id为true,并且删除idmaps并重启")
		}
		//补救措施
		idmap.SimplifiedStoreID(data.Author.ID)
		//补救措施
		idmap.SimplifiedStoreID(data.GroupID)
		//补救措施
		echo.AddMsgIDv3(AppIDString, data.GroupID, data.ID)
	} else {
		// 映射str的GroupID到int
		GroupID64, err = idmap.StoreIDv2(data.GroupID)
		if err != nil {
			mylog.Errorf("failed to convert GroupID64 to int: %v", err)
			return nil
		}
		// 映射str的userid到int
		userid64, err = idmap.StoreIDv2(data.Author.ID)
		if err != nil {
			mylog.Printf("Error storing ID: %v", err)
			return nil
		}
	}

	var messageText string
	GetDisableErrorChan := config.GetDisableErrorChan()

	//当屏蔽错误通道时候=性能模式 不解析at 不解析图片
	if !GetDisableErrorChan {
		// 转换at
		messageText = handlers.RevertTransformedText(data, "group", p.Api, p.Apiv2, GroupID64, userid64, true)
		if messageText == "" {
			mylog.Printf("信息被自定义黑白名单拦截")
			return nil
		}

		//框架内指令
		p.HandleFrameworkCommand(messageText, data, "group")
	} else {
		// 减少无用的性能开支
		messageText = data.Content

		if messageText == "/ " {
			messageText = " "
		}

		if messageText == " / " {
			messageText = " "
		}
		messageText = strings.TrimSpace(messageText)

		// 检查是否需要移除前缀
		if config.GetRemovePrefixValue() {
			// 移除消息内容中第一次出现的 "/"
			if idx := strings.Index(messageText, "/"); idx != -1 {
				messageText = messageText[:idx] + messageText[idx+1:]
			}
		}

	}

	//群没有at,但用户可以选择加一个
	if config.GetAddAtGroup() {
		messageText = "[CQ:at,qq=" + config.GetAppIDStr() + "] " + messageText
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

	if config.GetAutoBind() {
		if len(data.Attachments) > 0 && data.Attachments[0].URL != "" {
			p.Autobind(data)
		}
	}

	// 如果在Array模式下, 则处理Message为Segment格式
	var segmentedMessages interface{} = messageText

	//mylog.Printf("回调测试-群:%v\n", segmentedMessages)
	groupMsg := OnebotGroupMessage{
		Message:     segmentedMessages,
		MessageID:   messageID,
		GroupID:     GroupID64,
		MessageType: "group",
		PostType:    "message",
		UserID:      userid64,
		SubType:     "normal",
		Original:    originalPayload,
	}
	//enhanced config
	groupMsg.RealMessageType = "group"
	groupMsg.RealGroupID = data.GroupID
	groupMsg.RealUserID = data.Author.ID
	// 将当前s和appid和message进行映射
	echo.AddMsgID(AppIDString, s, data.ID)
	echo.AddMsgType(AppIDString, s, "group")
	//为不支持双向echo的ob服务端映射
	echo.AddMsgID(AppIDString, GroupID64, data.ID)
	//将当前的userid和groupid和msgid进行一个更稳妥的映射
	echo.AddMsgIDv2(AppIDString, GroupID64, userid64, data.ID)
	//储存当前群或频道号的类型
	idmap.WriteConfigv2(fmt.Sprint(GroupID64), "type", "group")
	//映射类型
	echo.AddMsgType(AppIDString, GroupID64, "group")
	//懒message_id池
	echo.AddLazyMessageId(strconv.FormatInt(GroupID64, 10), data.ID, time.Now())
	//懒message_id池
	echo.AddLazyMessageIdv2(strconv.FormatInt(GroupID64, 10), strconv.FormatInt(userid64, 10), data.ID, time.Now())
	// 如果要使用string参数action
	if config.GetStringAction() {
		//懒message_id池
		echo.AddLazyMessageId(data.GroupID, data.ID, time.Now())
		//懒message_id池
		echo.AddLazyMessageIdv2(data.GroupID, data.Author.ID, data.ID, time.Now())
	}
	// 调试
	PrintStructWithFieldNames(groupMsg)

	// Convert OnebotGroupMessage to map and send
	groupMsgMap := structToMap(groupMsg)

	// 如果不是性能模式
	if !GetDisableErrorChan {
		//上报信息到onebotv11应用端(正反ws) 并等待返回
		go p.BroadcastMessageToAll(groupMsgMap, p.Apiv2, data)
	} else {
		// FAF式
		go p.BroadcastMessageToAllFAF(groupMsgMap, p.Apiv2, data)
	}

	return nil
}
