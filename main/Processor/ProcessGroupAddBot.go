// 处理收到的信息事件
package Processor

import (
	"math/rand"
	"strconv"

	"github.com/hoshinonyaruko/gensokyo/callapi"
	"github.com/hoshinonyaruko/gensokyo/config"
	"github.com/hoshinonyaruko/gensokyo/echo"
	"github.com/hoshinonyaruko/gensokyo/handlers"
	"github.com/hoshinonyaruko/gensokyo/idmap"
	"github.com/hoshinonyaruko/gensokyo/mylog"
	"github.com/tencent-connect/botgo/dto"
)

// GroupRequestEvent 表示群组请求事件的数据结构
type GroupRequestEvent struct {
	Comment     string `json:"comment"`
	Flag        string `json:"flag"`
	GroupID     int64  `json:"group_id"`
	PostType    string `json:"post_type"`
	RequestType string `json:"request_type"`
	SubType     string `json:"sub_type"`
	UserID      int64  `json:"user_id"`
	Original    interface{} `json:"original"`
	RealUserID  string `json:"real_user_id,omitempty"`  //当前真实uid
	RealGroupID string `json:"real_group_id,omitempty"` //当前真实gid
}

// GroupNoticeEvent 表示群通知事件的数据结构
type GroupNoticeEvent struct {
	GroupID     int64  `json:"group_id"`
	NoticeType  string `json:"notice_type"`
	OperatorID  int64  `json:"operator_id"`
	PostType    string `json:"post_type"`
	SubType     string `json:"sub_type"`
	UserID      int64  `json:"user_id"`
	Original    interface{} `json:"original"`
	RealUserID  string `json:"real_user_id,omitempty"`  //当前真实uid
	RealGroupID string `json:"real_group_id,omitempty"` //当前真实gid
}

// 定义了一个符合 Client 接口的 SelfIntroduceClient 结构体
type SelfIntroduceClient struct {
	// 可添加所需字段
}

// 实现 Client 接口的 SendMessage 方法
// 假client中不执行任何操作，只是返回 nil 来符合接口要求
func (c *SelfIntroduceClient) SendMessage(message map[string]interface{}) error {
	// 不实际发送消息
	// log.Printf("SendMessage called with: %v", message)

	// 返回nil占位符
	return nil
}

// ProcessGroupAddBot 处理机器人增加
func (p *Processors) ProcessGroupAddBot(data *dto.GroupAddBotEvent, originalRaw []byte) error {
	var userid64 int64
	var GroupID64 int64
	var err error
	var Request GroupRequestEvent
	var Notice GroupNoticeEvent
	originalPayload := parseOriginalPayload(originalRaw)
	if config.GetIdmapPro() {
		GroupID64, userid64, err = idmap.StoreIDv2Pro(data.GroupOpenID, data.OpMemberOpenID)
		if err != nil {
			mylog.Errorf("Error storing ID: %v", err)
		}
	} else {
		GroupID64, err = idmap.StoreIDv2(data.GroupOpenID)
		if err != nil {
			mylog.Errorf("failed to convert ChannelID to int: %v", err)
			return nil
		}
		userid64, err = idmap.StoreIDv2(data.OpMemberOpenID)
		if err != nil {
			mylog.Printf("Error storing ID: %v", err)
			return nil
		}
	}
	Request = GroupRequestEvent{
		Comment:     "",
		Flag:        "",
		GroupID:     GroupID64,
		PostType:    "request",
		RequestType: "group",
		SubType:     "invite",
		UserID:      userid64,
		Original:    originalPayload,
	}
	//enhanced config
	Request.RealUserID = data.OpMemberOpenID
	Request.RealGroupID = data.GroupOpenID

	Notice = GroupNoticeEvent{
		GroupID:    GroupID64,
		NoticeType: "group_increase",
		OperatorID: 0,
		PostType:   "notice",
		SubType:    "invite",
		UserID:     userid64,
		Original:   originalPayload,
	}
	//enhanced config
	Notice.RealUserID = data.OpMemberOpenID
	Notice.RealGroupID = data.GroupOpenID

	groupMsgMap := structToMap(Request)
	//上报信息到onebotv11应用端(正反ws)
	go p.BroadcastMessageToAll(groupMsgMap, p.Apiv2, data)

	groupMsgMap = structToMap(Notice)
	//上报信息到onebotv11应用端(正反ws)
	go p.BroadcastMessageToAll(groupMsgMap, p.Apiv2, data)

	// 转换appid
	AppIDString := strconv.FormatUint(p.Settings.AppID, 10)

	// 储存和群号相关的eventid
	echo.AddEvnetID(AppIDString, GroupID64, data.EventID)
	// 确保也存储了32位字符串格式的eventID映射
	echo.AddEvnetIDv2(AppIDString, data.GroupOpenID, data.EventID)

	mylog.Printf("Bot被[%v]邀请进入群[%v]eventid[%v]", userid64, GroupID64, data.EventID)

	// 调用GetSelfIntroduce函数
	intros := config.GetSelfIntroduce()

	// 检查intros是否为空或只包含空字符串
	var validIntros []string
	for _, intro := range intros {
		if intro != "" {
			validIntros = append(validIntros, intro)
		}
	}

	// 如果设置了自我介绍
	if len(validIntros) != 0 {
		// 从validIntros中随机选择一个
		selectedIntro := validIntros[rand.Intn(len(validIntros))]

		// 创建 ActionMessage 实例
		message := callapi.ActionMessage{
			Action: "send_group_msg_group",
			Params: callapi.ParamsContent{
				GroupID: strconv.FormatInt(GroupID64, 10), // 转换 GroupID 类型
				UserID:  strconv.FormatInt(userid64, 10),
				Message: selectedIntro,
				EventID: data.EventID, // 使用原始EventID
			},
		}
		// clinet是发回值用的 这里相当于和http一样 不发回值所以建立一个假的client
		client := &SelfIntroduceClient{}
		// 调用处理函数
		_, err = handlers.HandleSendGroupMsg(client, p.Api, p.Apiv2, message)
		if err != nil {
			mylog.Printf("自我介绍发送失败%v", err)
		}
	}

	return nil
}
