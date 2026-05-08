package handlers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hoshinonyaruko/gensokyo/callapi"
	"github.com/hoshinonyaruko/gensokyo/config"
	"github.com/hoshinonyaruko/gensokyo/echo"
	"github.com/hoshinonyaruko/gensokyo/idmap"
	"github.com/hoshinonyaruko/gensokyo/interactionwait"
	"github.com/hoshinonyaruko/gensokyo/mylog"
	"github.com/tencent-connect/botgo/openapi"
)

type InteractionResponse struct {
	Data    string      `json:"data"`
	Message string      `json:"message"`
	RetCode int         `json:"retcode"`
	Status  string      `json:"status"`
	Echo    interface{} `json:"echo"`
}

func init() {
	callapi.RegisterHandler("put_interaction", HandlePutInteraction)
}

// HandlePutInteraction 手动回应按钮回调（INTERACTION_CREATE）。
//
// 入参（来自 message.Params）：
//   - InteractionID  : 平台原始 d.id，直接用于回应，不做任何转换；
//   - MessageID      : 框架转换后的数字短 ID，自动通过 idmap/memory_msgid 还原为真实 d.id；
//   - InteractionCode: 回应代码（int），合法范围 0~5，默认 0=成功。
//
// 二选一：InteractionID 与 MessageID 至少提供其一，前者优先。
func HandlePutInteraction(client callapi.Client, api openapi.OpenAPI, apiv2 openapi.OpenAPI, message callapi.ActionMessage) (string, error) {

	// 解析交互ID：interaction_id（原始 d.id）优先，否则用 message_id 还原
	var interactionID string

	// 1) 直接给出原始 interaction_id
	if message.Params.InteractionID != nil {
		if s := fmt.Sprintf("%v", message.Params.InteractionID); s != "" && s != "0" {
			interactionID = s
		}
	}

	// 2) 框架转换后的 message_id，通过 idmap/memory 还原为真实 d.id
	if interactionID == "" && message.Params.MessageID != nil {
		msgID := fmt.Sprintf("%v", message.Params.MessageID)
		if msgID != "" && msgID != "0" {
			var realID string
			var err error
			if config.GetMemoryMsgid() {
				// 从内存取
				realID, _ = echo.GetCacheIDFromMemoryByRowID(msgID)
			} else {
				realID, err = idmap.RetrieveRowByCachev2(msgID)
			}
			if err != nil {
				mylog.Printf("put_interaction message_id 还原失败: %v", err)
			}
			interactionID = realID
		}
	}

	if interactionID == "" {
		return "", fmt.Errorf("put_interaction 需要 interaction_id 或 message_id")
	}

	// 解析 code：从 Params.InteractionCode 读取，默认 0=成功
	var code int
	if message.Params.InteractionCode != nil {
		switch v := message.Params.InteractionCode.(type) {
		case int:
			code = v
		case float64: // 经过 JSON Unmarshal 时可能为 float64
			code = int(v)
		}
	}

	// 校验 code 范围（0=成功 1=操作失败 2=操作频繁 3=重复操作 4=没有权限 5=仅管理员操作）
	if code < 0 || code > 5 {
		return "", fmt.Errorf("invalid code: %d (expected 0-5)", code)
	}

	// 优先把 code 投递给 webhook 等待中的 pending 槽位（webhook 模式下生效）；
	// 槽位不存在则回退到平台 PUT API。
	if interactionwait.TryFill(interactionID, code) {
		mylog.Printf("put_interaction 通过webhook槽位回应交互: ID=%s, Code=%d", interactionID, code)
	} else {
		// 构造请求体，包括 code
		requestBody := fmt.Sprintf(`{"code": %d}`, code)

		// 调用 PutInteraction API
		ctx := context.Background()
		err := api.PutInteraction(ctx, interactionID, requestBody)
		if err != nil {
			return "", err
		}
	}

	var response InteractionResponse

	response.Data = ""
	response.Message = ""
	response.RetCode = 0
	response.Status = "ok"
	response.Echo = message.Echo

	// Convert the members slice to a map
	outputMap := structToMap(response)

	err := client.SendMessage(outputMap)
	if err != nil {
		mylog.Printf("Error sending message via client: %v", err)
	} else {
		mylog.Printf("put_interaction 响应: %+v", outputMap)
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
