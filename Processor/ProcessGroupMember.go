// 处理收到的群成员变更事件
package Processor

import (
	"fmt"

	"github.com/hoshinonyaruko/gensokyo/config"
	"github.com/hoshinonyaruko/gensokyo/idmap"
	"github.com/hoshinonyaruko/gensokyo/mylog"
	"github.com/tencent-connect/botgo/dto"
)

// ProcessGroupMemberAdd 处理群成员进群事件
func (p *Processors) ProcessGroupMemberAdd(data *dto.GroupMemberEvent, originalRaw []byte) error {
	return p.processGroupMemberChange(data, originalRaw, "group_increase", "approve")
}

// ProcessGroupMemberRemove 处理群成员退群事件
func (p *Processors) ProcessGroupMemberRemove(data *dto.GroupMemberEvent, originalRaw []byte) error {
	return p.processGroupMemberChange(data, originalRaw, "group_decrease", "leave")
}

func (p *Processors) processGroupMemberChange(data *dto.GroupMemberEvent, originalRaw []byte, noticeType string, subType string) error {
	var groupID64 int64
	var userID64 int64
	var eventID64 int64
	var eventGroupID64 int64
	var err error

	if data.EventID != "" {
		eventID64, err = idmap.StoreIDv2(data.EventID)
		if err != nil {
			mylog.Errorf("Error storing group member event ID: %v", err)
		}
	}

	if config.GetIdmapPro() && data.GroupOpenID != "" && data.MemberOpenID != "" {
		groupID64, userID64, err = idmap.StoreIDv2Pro(data.GroupOpenID, data.MemberOpenID)
		if err != nil {
			mylog.Errorf("Error storing group member ID: %v", err)
		}
		eventGroupID64, err = idmap.StoreIDv2(data.GroupOpenID)
		if err != nil {
			mylog.Errorf("Error storing group openid for event ID: %v", err)
		}
		_, _ = idmap.StoreIDv2(data.MemberOpenID)
		if !config.GetHashIDValue() {
			mylog.Fatalf("避坑日志:你开启了高级id转换,请设置hash_id为true,并且删除idmaps并重启")
		}
	} else {
		if data.GroupOpenID != "" {
			groupID64, err = idmap.StoreIDv2(data.GroupOpenID)
			if err != nil {
				mylog.Errorf("failed to convert group openid to int: %v", err)
				return nil
			}
			eventGroupID64 = groupID64
		}
		if data.MemberOpenID != "" {
			userID64, err = idmap.StoreIDv2(data.MemberOpenID)
			if err != nil {
				mylog.Printf("Error storing member openid: %v", err)
				return nil
			}
		}
	}

	notice := GroupNoticeEvent{
		GroupID:    groupID64,
		NoticeType: noticeType,
		OperatorID: 0,
		PostType:   "notice",
		SubType:    subType,
		UserID:     userID64,
		EventID:    eventID64,
		Original:   parseOriginalPayload(originalRaw),
	}
	notice.RealUserID = data.MemberOpenID
	notice.RealGroupID = data.GroupOpenID

	// 注意: 群成员增减事件的 event_id 不能写入按群号索引的通用 event_id 槽。
	// 该槽会被普通群发送在 msg_id 过期时兜底读取 (send_group_msg.go 的
	// GetEventIDByUseridOrGroupid), 而成员事件的 event_id 是"单次可用"且
	// 退群事件根本不允许回复, 一旦污染该槽, 后续普通消息会取到废弃/越权的
	// event_id, 直接触发 40034025(event_id 无效)。
	// 如需入群欢迎语, 应由应用端消费上报 notice 中的 event_id 并显式回传,
	// 且仅允许发送一条, 而不是依赖这里写入共享槽。
	// eventID64 仍在上方转换并放入 notice.EventID 供应用端使用, 此处不再写入 echo 映射。
	_ = eventGroupID64

	noticeMap := structToMap(notice)
	go p.BroadcastMessageToAll(noticeMap, p.Apiv2, data)

	mylog.Printf("群成员变更事件[%v] group[%v] user[%v] event[%v]", noticeType, groupID64, userID64, eventID64)
	if groupID64 != 0 {
		idmap.WriteConfigv2(fmt.Sprint(groupID64), "type", "group")
	}
	return nil
}
