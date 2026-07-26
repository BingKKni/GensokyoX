package handlers

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hoshinonyaruko/gensokyo/callapi"
	"github.com/hoshinonyaruko/gensokyo/config"
	"github.com/hoshinonyaruko/gensokyo/echo"
	"github.com/hoshinonyaruko/gensokyo/idmap"
	"github.com/hoshinonyaruko/gensokyo/mylog"
	"github.com/tencent-connect/botgo/openapi"
)

func init() {
	callapi.RegisterHandler("send_msg", HandleSendMsg)
}

// resolveEchoVirtualID resolves an already-known real ID without creating a
// mapping. Numeric values from OneBot applications are virtual IDs and must
// never be fed back into StoreID, which would consume another virtual ID.
func resolveEchoVirtualID(id string) string {
	if id == "" {
		return ""
	}
	if row, err := strconv.ParseInt(id, 10, 64); err == nil {
		if realID, err := idmap.RetrieveRowByIDv2(id); err == nil {
			// 违规虚拟ID在查询时会被内部重映射并删除旧反向键，
			// 此时原始数字ID已失效，需用真实值反查当前有效的虚拟ID。
			if idmap.IsViolateID(row) {
				if _, virtualID, err := idmap.RetrieveVirtualValuev2(realID); err == nil {
					return virtualID
				}
			}
			return id
		}
	}
	if _, virtualID, err := idmap.RetrieveVirtualValuev2(id); err == nil {
		return virtualID
	}
	return id
}

func resolveEchoVirtualIDInt64(id string) (int64, error) {
	virtualID := resolveEchoVirtualID(id)
	row, err := strconv.ParseInt(virtualID, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("ID %q has no existing virtual mapping", id)
	}
	return row, nil
}

func resolveEchoVirtualIDPair(groupID, userID string) (int64, int64, error) {
	if groupRow, groupErr := strconv.ParseInt(groupID, 10, 64); groupErr == nil {
		if userRow, userErr := strconv.ParseInt(userID, 10, 64); userErr == nil {
			return groupRow, userRow, nil
		}
	}
	virtualGroupID, virtualUserID, err := idmap.RetrieveVirtualValuev2Pro(groupID, userID)
	if err != nil {
		return 0, 0, err
	}
	groupRow, err := strconv.ParseInt(virtualGroupID, 10, 64)
	if err != nil {
		return 0, 0, err
	}
	userRow, err := strconv.ParseInt(virtualUserID, 10, 64)
	if err != nil {
		return 0, 0, err
	}
	return groupRow, userRow, nil
}

func resolveExplicitMessageID(messageID string) (string, error) {
	if messageID == "" || messageID == "0" || !isVirtualMessageID(messageID) {
		return messageID, nil
	}
	if config.GetMemoryMsgid() {
		if realID, ok := echo.GetCacheIDFromMemoryByRowID(messageID); ok {
			return realID, nil
		}
		return "", fmt.Errorf("message_id %s is not present in the in-memory cache", messageID)
	}
	realID, err := idmap.RetrieveRowByCachev2(messageID)
	if err != nil {
		return "", fmt.Errorf("resolve message_id %s: %w", messageID, err)
	}
	return realID, nil
}

func normalizePlatformEventID(eventID string) string {
	if index := strings.LastIndex(eventID, ":"); index >= 0 && index < len(eventID)-1 {
		return eventID[index+1:]
	}
	return eventID
}

func resolveExplicitEventID(eventID string) string {
	if eventID == "" || eventID == "0" || !isVirtualMessageID(eventID) {
		return normalizePlatformEventID(eventID)
	}
	if realID, ok := echo.GetCacheIDFromMemoryByRowID(eventID); ok {
		return normalizePlatformEventID(realID)
	}
	// Legacy versions stored event IDs in the stable entity bucket. Keep old
	// notices usable during migration without creating any new mapping.
	if realID, err := idmap.RetrieveRowByIDv2(eventID); err == nil {
		return normalizePlatformEventID(realID)
	}
	return eventID
}

// 平台因 event_id 本身不可用而拒收的错误码:
// 40034025/11255 为 event_id 无效, 40034026 为 event_id 已过期
// (群聊 event_id 仅5分钟有效期, 单聊为60分钟)。
// 这几种都应当清空 event_id 后转主动消息重发, 避免消息静默丢失。
func isEventIDRejected(err error) bool {
	if err == nil {
		return false
	}
	errText := err.Error()
	for _, code := range []string{"40034025", "40034026", "11255"} {
		if strings.Contains(errText, `"code":`+code) || strings.Contains(errText, `"err_code":`+code) {
			return true
		}
	}
	return false
}

func HandleSendMsg(client callapi.Client, api openapi.OpenAPI, apiv2 openapi.OpenAPI, message callapi.ActionMessage) (string, error) {
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

	if len(message.Params.GroupID.(string)) != 32 {
		if msgType == "" && message.Params.GroupID != nil && checkZeroGroupID(message.Params.GroupID) {
			msgType = GetMessageTypeByGroupid(config.GetAppIDStr(), message.Params.GroupID)
		}
		if msgType == "" && message.Params.UserID != nil && checkZeroUserID(message.Params.UserID) {
			msgType = GetMessageTypeByUserid(config.GetAppIDStr(), message.Params.UserID)
		}
		if msgType == "" && message.Params.GroupID != nil && checkZeroGroupID(message.Params.GroupID) {
			msgType = GetMessageTypeByGroupidV2(message.Params.GroupID)
		}
		if msgType == "" && message.Params.UserID != nil && checkZeroUserID(message.Params.UserID) {
			msgType = GetMessageTypeByUseridV2(message.Params.UserID)
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

	if len(message.Params.GroupID.(string)) == 32 {
		if message.Params.GroupID != "" {
			idInt64, err = idmap.GenerateRowID(message.Params.GroupID.(string), 9)
		} else if message.Params.UserID != "" {
			idInt64, err = idmap.GenerateRowID(message.Params.UserID.(string), 9)
		}
		// 临时的
		msgType = "group"
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
			// 递归调用handleSendMsg，使用设置的消息类型
			// 修复：如果有GroupID，优先尝试group类型；否则才使用group_private
			defaultType := "group_private"
			if message.Params.GroupID != nil && message.Params.GroupID.(string) != "" && message.Params.GroupID.(string) != "0" {
				defaultType = "group"
			}
			echo.AddMsgType(config.GetAppIDStr(), idInt64, defaultType)
			retmsg, _ = HandleSendMsg(client, api, apiv2, messageCopy)
		}
	} else if echo.GetMapping(idInt64) <= 0 {
		// 特殊值代表不递归
		echo.AddMapping(idInt64, 10)
	}

	switch msgType {
	case "group":
		//复用处理逻辑
		retmsg, _ = HandleSendGroupMsg(client, api, apiv2, message)
	case "guild":
		//用GroupID给ChannelID赋值,因为我们是把频道虚拟成了群
		message.Params.ChannelID = message.Params.GroupID.(string)
		var RChannelID string
		if message.Params.UserID != nil && config.GetIdmapPro() && message.Params.UserID.(string) != "" && message.Params.UserID.(string) != "0" {
			RChannelID, _, err = idmap.RetrieveRowByIDv2Pro(message.Params.ChannelID.(string), message.Params.UserID.(string))
		}
		if RChannelID == "" {
			// 使用RetrieveRowByIDv2还原真实的ChannelID
			RChannelID, err = idmap.RetrieveRowByIDv2(message.Params.ChannelID.(string))
		}
		if err != nil {
			mylog.Printf("error retrieving real RChannelID: %v", err)
		}
		message.Params.ChannelID = RChannelID
		retmsg, _ = HandleSendGuildChannelMsg(client, api, apiv2, message)
	case "guild_private":
		//send_msg比具体的send_xxx少一层,其包含的字段类型在虚拟化场景已经失去作用
		//根据userid绑定得到的具体真实事件类型,这里也有多种可能性
		//1,私聊(但虚拟成了群),这里用群号取得需要的id
		//2,频道私聊(但虚拟成了私聊)这里传递2个nil,用user_id去推测channel_id和guild_id
		retmsg, _ = HandleSendGuildChannelPrivateMsg(client, api, apiv2, message, nil, nil)
	case "group_private":
		//私聊信息
		retmsg, _ = HandleSendPrivateMsg(client, api, apiv2, message)
	case "forum":
		//用GroupID给ChannelID赋值,因为我们是把频道虚拟成了群
		message.Params.ChannelID = message.Params.GroupID.(string)
		var RChannelID string
		if message.Params.UserID != nil && config.GetIdmapPro() && message.Params.UserID.(string) != "" && message.Params.UserID.(string) != "0" {
			RChannelID, _, err = idmap.RetrieveRowByIDv2Pro(message.Params.ChannelID.(string), message.Params.UserID.(string))
		}
		if RChannelID == "" {
			// 使用RetrieveRowByIDv2还原真实的ChannelID
			RChannelID, err = idmap.RetrieveRowByIDv2(message.Params.ChannelID.(string))
		}
		if err != nil {
			mylog.Printf("error retrieving real RChannelID: %v", err)
		}
		message.Params.ChannelID = RChannelID
		retmsg, _ = HandleSendGuildChannelForum(client, api, apiv2, message)
	default:
		mylog.Printf("1Unknown message type: %s", msgType)
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
			retmsg, _ = HandleSendMsg(client, api, apiv2, messageCopy)
		}
	}

	return retmsg, nil
}

// 通过user_id获取messageID
func GetMessageIDByUseridOrGroupid(appID string, userID interface{}) string {
	// 从appID和userID生成key
	var userIDStr string
	switch u := userID.(type) {
	case int:
		userIDStr = strconv.Itoa(u)
	case int64:
		userIDStr = strconv.FormatInt(u, 10)
	case float64:
		userIDStr = strconv.FormatFloat(u, 'f', 0, 64)
	case string:
		userIDStr = u
	default:
		// 可能需要处理其他类型或报错
		return ""
	}
	virtualUserID := resolveEchoVirtualID(userIDStr)
	key := appID + "_" + virtualUserID
	messageid := echo.GetMsgIDByKey(key)
	if messageid == "" {
		key := appID + "_" + userIDStr
		messageid = echo.GetMsgIDByKey(key)
	}
	return messageid
}

// 通过user_id获取EventID 私聊,群,频道,通用 userID可以是三者之一 这是不需要区分群+用户的 只需要精准到群 私聊只需要精准到用户 idmap不开启的用户使用
func GetEventIDByUseridOrGroupid(appID string, userID interface{}) string {
	// 从appID和userID生成key
	var userIDStr string
	switch u := userID.(type) {
	case int:
		userIDStr = strconv.Itoa(u)
	case int64:
		userIDStr = strconv.FormatInt(u, 10)
	case float64:
		userIDStr = strconv.FormatFloat(u, 'f', 0, 64)
	case string:
		userIDStr = u
	default:
		// 可能需要处理其他类型或报错
		return ""
	}
	virtualUserID := resolveEchoVirtualID(userIDStr)
	key := appID + "_" + virtualUserID
	eventid := echo.GetEventIDByKey(key)
	if eventid == "" {
		// 用原始id获取,这个分支应该是没有用的.
		key := appID + "_" + userIDStr
		eventid = echo.GetEventIDByKey(key)
	}
	return eventid
}

// 通过user_id获取EventID 私聊,群,频道,通用 userID可以是三者之一 这是不需要区分群+用户的 只需要精准到群 私聊只需要精准到用户 idmap不开启的用户使用
func GetEventIDByUseridOrGroupidv2(appID string, userID interface{}) string {
	// 从appID和userID生成key
	var userIDStr string
	switch u := userID.(type) {
	case int:
		userIDStr = strconv.Itoa(u)
	case int64:
		userIDStr = strconv.FormatInt(u, 10)
	case float64:
		userIDStr = strconv.FormatFloat(u, 'f', 0, 64)
	case string:
		userIDStr = u
	default:
		// 可能需要处理其他类型或报错
		return ""
	}

	key := appID + "_" + userIDStr
	eventid := echo.GetEventIDByKey(key)

	return eventid
}

// 通过user_id获取messageID
func GetMessageIDByUseridAndGroupid(appID string, userID interface{}, groupID interface{}) string {
	// 从appID和userID生成key
	var userIDStr string
	switch u := userID.(type) {
	case int:
		userIDStr = strconv.Itoa(u)
	case int64:
		userIDStr = strconv.FormatInt(u, 10)
	case float64:
		userIDStr = strconv.FormatFloat(u, 'f', 0, 64)
	case string:
		userIDStr = u
	default:
		// 可能需要处理其他类型或报错
		return ""
	}
	// 从appID和userID生成key
	var GroupIDStr string
	switch u := groupID.(type) {
	case int:
		GroupIDStr = strconv.Itoa(u)
	case int64:
		GroupIDStr = strconv.FormatInt(u, 10)
	case float64:
		GroupIDStr = strconv.FormatFloat(u, 'f', 0, 64)
	case string:
		GroupIDStr = u
	default:
		// 可能需要处理其他类型或报错
		return ""
	}
	var virtualUserID, virtualGroupID string
	if config.GetIdmapPro() {
		var err error
		virtualGroupID, virtualUserID, err = idmap.RetrieveVirtualValuev2Pro(GroupIDStr, userIDStr)
		if err != nil {
			// Inputs may already be the Pro virtual pair.
			virtualGroupID, virtualUserID = GroupIDStr, userIDStr
		}
	} else {
		virtualUserID = resolveEchoVirtualID(userIDStr)
		virtualGroupID = resolveEchoVirtualID(GroupIDStr)
	}
	key := appID + "_" + virtualGroupID + "_" + virtualUserID
	return echo.GetMsgIDByKey(key)
}
