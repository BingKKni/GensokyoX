package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/hoshinonyaruko/gensokyo/callapi"
	"github.com/hoshinonyaruko/gensokyo/config"
	"github.com/hoshinonyaruko/gensokyo/echo"
	"github.com/hoshinonyaruko/gensokyo/idmap"
	"github.com/hoshinonyaruko/gensokyo/mylog"
	"github.com/tencent-connect/botgo/openapi"
)

type DeleteMsgResponse struct {
	Message string      `json:"message"`
	RetCode int         `json:"retcode"`
	Status  string      `json:"status"`
	Echo    interface{} `json:"echo"`
}

func init() {
	callapi.RegisterHandler("delete_msg", DeleteMsg)
}

func DeleteMsg(client callapi.Client, api openapi.OpenAPI, apiv2 openapi.OpenAPI, message callapi.ActionMessage) (string, error) {
	var RealMsgID string
	var err error

	RealMsgID = paramString(message.Params.RealMessageID)
	if RealMsgID == "" {
		messageID := paramString(message.Params.MessageID)
		if isVirtualMessageID(messageID) {
			// 如果从内存取
			if config.GetMemoryMsgid() {
				//还原msgid
				RealMsgID, _ = echo.GetCacheIDFromMemoryByRowID(messageID)
			} else {
				//还原msgid
				RealMsgID, err = idmap.RetrieveRowByCachev2(messageID)
				if err != nil {
					mylog.Printf("error retrieving real message ID: %v", err)
					return sendDeleteMsgResponse(client, message, failedDeleteMsgResponse(err, api.TraceID()))
				}
			}
		} else {
			RealMsgID = messageID
		}
	}
	if RealMsgID == "" {
		return sendDeleteMsgResponse(client, message, failedDeleteMsgResponse(fmt.Errorf("message_id is empty"), api.TraceID()))
	}

	//重新赋值
	message.Params.MessageID = RealMsgID
	called := false
	var retractErr error

	//撤回频道信息
	if paramString(message.Params.ChannelID) != "" {
		called = true
		var RChannelID string
		var err error
		// 使用RetrieveRowByIDv2还原真实的ChannelID
		RChannelID, err = idmap.RetrieveRowByIDv2(message.Params.ChannelID.(string))
		if err != nil {
			mylog.Printf("error retrieving real RChannelID: %v", err)
			return sendDeleteMsgResponse(client, message, failedDeleteMsgResponse(err, api.TraceID()))
		}
		message.Params.ChannelID = RChannelID
		err = api.RetractMessage(context.TODO(), message.Params.ChannelID.(string), message.Params.MessageID.(string), openapi.RetractMessageOptionHidetip)
		if err != nil {
			fmt.Println("Error retracting channel message:", err)
			retractErr = err
		}

	}

	//撤回频道私信
	if paramString(message.Params.GuildID) != "" {
		called = true
		//这里很复杂 要取的话需要调用internal-api 根据情况还原，虚拟成群就用群（channel-id）还原完整channel-id，
		//然后internal-api读配置获取guild-id ，虚拟成私信就用userid还原完整userid，然后读channel-id然后读guild-id
		//因为GuildID本身不直接出现在ob11事件里。
		err := api.RetractDMMessage(context.TODO(), message.Params.GuildID.(string), message.Params.MessageID.(string), openapi.RetractMessageOptionHidetip)
		if err != nil {
			fmt.Println("Error retracting DM message:", err)
			retractErr = err
		}

	}

	//撤回群信息
	if paramString(message.Params.GroupID) != "" {
		called = true
		var originalGroupID string
		originalGroupID, err := idmap.RetrieveRowByIDv2(message.Params.GroupID.(string))
		if err != nil {
			mylog.Printf("Error retrieving original GroupID: %v", err)
			return sendDeleteMsgResponse(client, message, failedDeleteMsgResponse(err, api.TraceID()))
		}
		message.Params.GroupID = originalGroupID
		err = api.RetractGroupMessage(context.TODO(), message.Params.GroupID.(string), message.Params.MessageID.(string), openapi.RetractMessageOptionHidetip)
		if err != nil {
			fmt.Println("Error retracting group message:", err)
			retractErr = err
		}

	}

	//撤回C2C私信消息列表
	if paramString(message.Params.UserID) != "" {
		called = true
		var UserID string
		//还原真实的userid
		UserID, err := idmap.RetrieveRowByIDv2(message.Params.UserID.(string))
		if err != nil {
			mylog.Printf("Error reading config: %v", err)
			return sendDeleteMsgResponse(client, message, failedDeleteMsgResponse(err, api.TraceID()))
		}
		message.Params.UserID = UserID
		err = api.RetractC2CMessage(context.TODO(), message.Params.UserID.(string), message.Params.MessageID.(string), openapi.RetractMessageOptionHidetip)
		if err != nil {
			fmt.Println("Error retracting C2C message:", err)
			retractErr = err
		}

	}

	if !called {
		return sendDeleteMsgResponse(client, message, failedDeleteMsgResponse(fmt.Errorf("delete_msg requires group_id, user_id, channel_id, or guild_id"), api.TraceID()))
	}

	return sendDeleteMsgResponse(client, message, deleteMsgResponse(retractErr, api.TraceID()))
}

func sendDeleteMsgResponse(client callapi.Client, message callapi.ActionMessage, response map[string]interface{}) (string, error) {
	if message.Echo != nil {
		response["echo"] = message.Echo
	}

	mylog.Printf("delete_msg: %+v\n", response)

	err := client.SendMessage(response)
	if err != nil {
		mylog.Printf("Error sending message via client: %v", err)
	}
	//把结果从struct转换为json
	result, err := json.Marshal(response)
	if err != nil {
		mylog.Printf("Error marshaling data: %v", err)
		//todo 符合onebotv11 ws返回的错误码
		return "", nil
	}
	return string(result), nil
}

func deleteMsgResponse(err error, traceID string) map[string]interface{} {
	if err == nil {
		return map[string]interface{}{
			"message": "",
			"retcode": 0,
			"status":  "ok",
			"traceID": traceID,
		}
	}

	return failedDeleteMsgResponse(err, traceID)
}

func failedDeleteMsgResponse(err error, traceID string) map[string]interface{} {
	type sdkErr interface {
		Code() int
		Text() string
		Trace() string
	}

	if e, ok := err.(sdkErr); ok {
		if e.Text() != "" {
			var platformResponse map[string]interface{}
			if json.Unmarshal([]byte(e.Text()), &platformResponse) == nil {
				if e.Trace() != "" {
					if _, ok := platformResponse["traceID"]; !ok {
						platformResponse["traceID"] = e.Trace()
					}
				}
				return platformResponse
			}
		}

		return map[string]interface{}{
			"message":    e.Text(),
			"error_code": e.Code(),
			"retcode":    e.Code(),
			"status":     "failed",
			"traceID":    e.Trace(),
		}
	}

	return map[string]interface{}{
		"message":    err.Error(),
		"error_code": 0,
		"retcode":    -1,
		"status":     "failed",
		"traceID":    traceID,
	}
}

func paramString(value interface{}) string {
	if value == nil {
		return ""
	}
	if s, ok := value.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", value)
}

func isVirtualMessageID(messageID string) bool {
	if messageID == "" {
		return false
	}
	_, err := strconv.ParseInt(messageID, 10, 64)
	return err == nil
}
