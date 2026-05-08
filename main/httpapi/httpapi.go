package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	"github.com/hoshinonyaruko/gensokyo/config"
	"github.com/hoshinonyaruko/gensokyo/handlers"
	"github.com/hoshinonyaruko/gensokyo/structs"

	"github.com/gin-gonic/gin"
	"github.com/hoshinonyaruko/gensokyo/callapi"
	"github.com/tencent-connect/botgo/openapi"
)

// CombinedMiddleware 创建并返回一个带有依赖的中间件闭包
func CombinedMiddleware(api openapi.OpenAPI, apiV2 openapi.OpenAPI) gin.HandlerFunc {
	return func(c *gin.Context) {
		accessToken := config.GetHTTPAccessToken()
		if accessToken != "" {
			tokenHeader := strings.Replace(c.GetHeader("Authorization"), "Bearer ", "", 1)
			tokenQuery, _ := c.GetQuery("access_token")
			if (tokenHeader == "" || tokenHeader != accessToken) && (tokenQuery == "" || tokenQuery != accessToken) {
				c.JSON(http.StatusForbidden, gin.H{"error": "鉴权失败"})
				return
			}
		}

		// 检查路径和处理对应的请求
		if c.Request.URL.Path == "/send_group_msg" {
			handleSendGroupMessage(c, api, apiV2)
			return
		}
		if c.Request.URL.Path == "/send_group_msg_raw" {
			handleSendGroupMessageRaw(c, api, apiV2)
			return
		}
		if c.Request.URL.Path == "/send_private_msg" {
			handleSendPrivateMessage(c, api, apiV2)
			return
		}
		if c.Request.URL.Path == "/send_private_msg_sse" {
			handleSendPrivateMessageSSE(c, api, apiV2)
			return
		}
		if c.Request.URL.Path == "/send_guild_channel_msg" {
			handleSendGuildChannelMessage(c, api, apiV2)
			return
		}
		if c.Request.URL.Path == "/get_group_list" {
			handleGetGroupList(c, api, apiV2)
			return
		}
		if c.Request.URL.Path == "/put_interaction" {
			handlePutInteraction(c, api, apiV2)
			return
		}
		if c.Request.URL.Path == "/delete_msg" {
			handleDeleteMsg(c, api, apiV2)
			return
		}
		if c.Request.URL.Path == "/get_avatar" {
			handleGetAvatar(c, api, apiV2)
			return
		}
		if c.Request.URL.Path == "/get_friend_list" {
			handleGetFriendList(c, api, apiV2)
			return
		}
		if c.Request.URL.Path == "/upload_group_file" {
			handleUploadGroupFile(c, api, apiV2)
			return
		}
		if c.Request.URL.Path == "/upload_private_file" {
			handleUploadPrivateFile(c, api, apiV2)
			return
		}

		// 调用c.Next()以继续处理请求链
		c.Next()
	}
}

// handleSendGroupMessage 处理发送群聊消息的请求
func handleSendGroupMessage(c *gin.Context, api openapi.OpenAPI, apiV2 openapi.OpenAPI) {
	var retmsg string
	var req struct {
		GroupID    int64  `json:"group_id" form:"group_id"`
		UserID     *int64 `json:"user_id,omitempty" form:"user_id"`
		Message    string `json:"message" form:"message"`
		AutoEscape bool   `json:"auto_escape" form:"auto_escape"`
	}

	// 根据请求方法解析参数
	if c.Request.Method == http.MethodGet {
		// 从URL查询参数解析
		if err := c.ShouldBindQuery(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	} else {
		// 从JSON或表单数据解析
		if err := c.ShouldBind(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}

	// 使用解析后的参数处理请求
	client := &HttpAPIClient{}
	// 创建 ActionMessage 实例
	message := callapi.ActionMessage{
		Action: "send_group_msg",
		Params: callapi.ParamsContent{
			GroupID: strconv.FormatInt(req.GroupID, 10), // 注意这里需要转换类型，因为 GroupID 是 int64
			Message: req.Message,
		},
	}
	// 如果 UserID 存在，则加入到参数中
	if req.UserID != nil {
		message.Params.UserID = strconv.FormatInt(*req.UserID, 10)
	}
	// 调用处理函数
	retmsg, err := handlers.HandleSendGroupMsg(client, api, apiV2, message)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 返回处理结果
	c.Header("Content-Type", "application/json")
	c.String(http.StatusOK, retmsg)
}

// handleSendGroupMessageRaw 处理发送群聊消息的请求
func handleSendGroupMessageRaw(c *gin.Context, api openapi.OpenAPI, apiV2 openapi.OpenAPI) {
	var retmsg string
	var req struct {
		GroupID    int64  `json:"group_id" form:"group_id"`
		MessageID  string `json:"message_id" form:"message_id"`
		UserID     *int64 `json:"user_id,omitempty" form:"user_id"`
		Message    string `json:"message" form:"message"`
		AutoEscape bool   `json:"auto_escape" form:"auto_escape"`
	}

	// 根据请求方法解析参数
	if c.Request.Method == http.MethodGet {
		// 从URL查询参数解析
		if err := c.ShouldBindQuery(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	} else {
		// 从JSON或表单数据解析
		if err := c.ShouldBind(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}

	// 使用解析后的参数处理请求
	client := &HttpAPIClient{}
	// 创建 ActionMessage 实例
	message := callapi.ActionMessage{
		Action: "send_group_msg_raw",
		Params: callapi.ParamsContent{
			GroupID:   strconv.FormatInt(req.GroupID, 10), // 注意这里需要转换类型，因为 GroupID 是 int64
			MessageID: req.MessageID,
			Message:   req.Message,
		},
	}
	// 如果 UserID 存在，则加入到参数中
	if req.UserID != nil {
		message.Params.UserID = strconv.FormatInt(*req.UserID, 10)
	}
	// 调用处理函数
	retmsg, err := handlers.HandleSendGroupMsgRaw(client, api, apiV2, message)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 返回处理结果
	c.Header("Content-Type", "application/json")
	c.String(http.StatusOK, retmsg)
}

// handleSendPrivateMessage 处理发送私聊消息的请求
func handleSendPrivateMessage(c *gin.Context, api openapi.OpenAPI, apiV2 openapi.OpenAPI) {
	var retmsg string
	var req struct {
		GroupID    int64  `json:"group_id" form:"group_id"`
		UserID     int64  `json:"user_id" form:"user_id"`
		Message    string `json:"message" form:"message"`
		AutoEscape bool   `json:"auto_escape" form:"auto_escape"`
		IsWakeup   bool   `json:"is_wakeup" form:"is_wakeup"`
	}

	// 根据请求方法解析参数
	if c.Request.Method == http.MethodGet {
		// 从URL查询参数解析
		if err := c.ShouldBindQuery(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	} else {
		// 从JSON或表单数据解析
		if err := c.ShouldBind(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}

	// 使用解析后的参数处理请求
	// TODO: 添加请求处理逻辑
	// 例如：api.SendGroupMessage(req.GroupID, req.Message, req.AutoEscape)
	client := &HttpAPIClient{}
	// 创建 ActionMessage 实例
	message := callapi.ActionMessage{
		Action: "send_private_msg",
		Params: callapi.ParamsContent{
			GroupID:  strconv.FormatInt(req.GroupID, 10), // 注意这里需要转换类型，因为 GroupID 是 int64
			UserID:   strconv.FormatInt(req.UserID, 10),
			Message:  req.Message,
			IsWakeup: req.IsWakeup,
		},
	}
	// 调用处理函数
	retmsg, err := handlers.HandleSendPrivateMsg(client, api, apiV2, message)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 返回处理结果
	c.Header("Content-Type", "application/json")
	c.String(http.StatusOK, retmsg)
}

// handleSendGuildChannelMessage 处理发送消频道息的请求
func handleSendGuildChannelMessage(c *gin.Context, api openapi.OpenAPI, apiV2 openapi.OpenAPI) {
	var retmsg string
	var req struct {
		GuildID    string `json:"guild_id" form:"guild_id"`
		ChannelID  string `json:"channel_id" form:"channel_id"`
		Message    string `json:"message" form:"message"`
		AutoEscape bool   `json:"auto_escape" form:"auto_escape"`
	}

	// 根据请求方法解析参数
	if c.Request.Method == http.MethodGet {
		// 从URL查询参数解析
		if err := c.ShouldBindQuery(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	} else {
		// 从JSON或表单数据解析
		if err := c.ShouldBind(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}

	// 使用解析后的参数处理请求
	// TODO: 添加请求处理逻辑
	// 例如：api.SendGroupMessage(req.GroupID, req.Message, req.AutoEscape)
	client := &HttpAPIClient{}
	// 创建 ActionMessage 实例
	message := callapi.ActionMessage{
		Action: "send_guild_channel_msg",
		Params: callapi.ParamsContent{
			GuildID:   req.GuildID,
			ChannelID: req.ChannelID, // 注意这里需要转换类型，因为 GroupID 是 int64
			Message:   req.Message,
		},
	}
	// 调用处理函数
	retmsg, err := handlers.HandleSendGuildChannelMsg(client, api, apiV2, message)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 返回处理结果
	c.Header("Content-Type", "application/json")
	c.String(http.StatusOK, retmsg)
}

// 定义了一个符合 Client 接口的 HttpAPIClient 结构体
type HttpAPIClient struct {
	// 可添加所需字段
}

// 实现 Client 接口的 SendMessage 方法
// 假client中不执行任何操作，只是返回 nil 来符合接口要求
func (c *HttpAPIClient) SendMessage(message map[string]interface{}) error {
	// 不实际发送消息
	// log.Printf("SendMessage called with: %v", message)

	// 返回nil占位符
	return nil
}

// handleSendPrivateMessageSSE 处理发送私聊SSE消息的请求
func handleSendPrivateMessageSSE(c *gin.Context, api openapi.OpenAPI, apiV2 openapi.OpenAPI) {
	// 根据请求方法解析参数
	if c.Request.Method == http.MethodGet {
		var req struct {
			GroupID    int64  `json:"group_id" form:"group_id"`
			UserID     int64  `json:"user_id" form:"user_id"`
			Message    string `json:"message" form:"message"`
			AutoEscape bool   `json:"auto_escape" form:"auto_escape"`
		}
		// 从URL查询参数解析
		if err := c.ShouldBindQuery(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		var InterfaceBody structs.InterfaceBody
		if err := json.Unmarshal([]byte(req.Message), &InterfaceBody); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid message format"})
			return
		}

		client := &HttpAPIClient{}
		// 创建 ActionMessage 实例
		message := callapi.ActionMessage{
			Action: "send_private_msg_sse",
			Params: callapi.ParamsContent{
				GroupID: strconv.FormatInt(req.GroupID, 10), // 注意这里需要转换类型，因为 GroupID 是 int64
				UserID:  strconv.FormatInt(req.UserID, 10),
				Message: InterfaceBody,
			},
		}
		// 调用处理函数
		retmsg, err := handlers.HandleSendPrivateMsgSSE(client, api, apiV2, message)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// 返回处理结果
		c.Header("Content-Type", "application/json")
		c.String(http.StatusOK, retmsg)
	} else {
		var req struct {
			GroupID    int64       `json:"group_id" form:"group_id"`
			UserID     int64       `json:"user_id" form:"user_id"`
			Message    interface{} `json:"message" form:"message"`
			AutoEscape bool        `json:"auto_escape" form:"auto_escape"`
		}
		// 从JSON或表单数据解析
		if err := c.ShouldBind(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		client := &HttpAPIClient{}
		// 创建 ActionMessage 实例
		message := callapi.ActionMessage{
			Action: "send_private_msg_sse",
			Params: callapi.ParamsContent{
				GroupID: strconv.FormatInt(req.GroupID, 10), // 注意这里需要转换类型，因为 GroupID 是 int64
				UserID:  strconv.FormatInt(req.UserID, 10),
				Message: req.Message,
			},
		}
		// 调用处理函数
		retmsg, err := handlers.HandleSendPrivateMsgSSE(client, api, apiV2, message)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// 返回处理结果
		c.Header("Content-Type", "application/json")
		c.String(http.StatusOK, retmsg)
	}

}

// handleGetGroupList 处理获取群列表
func handleGetGroupList(c *gin.Context, api openapi.OpenAPI, apiV2 openapi.OpenAPI) {
	var retmsg string

	// 使用解析后的参数处理请求
	client := &HttpAPIClient{}
	// 创建 ActionMessage 实例
	message := callapi.ActionMessage{
		Action: "get_group_list",
	}

	// 调用处理函数
	retmsg, err := handlers.GetGroupList(client, api, apiV2, message)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 返回处理结果
	c.Header("Content-Type", "application/json")
	c.String(http.StatusOK, retmsg)
}

// handlePutInteraction 处理put_interaction的请求
//
// 入参（interaction_id 与 message_id 二选一，至少提供其一）：
//   - interaction_id : 平台原始 d.id，不做转换，直接用于回应
//   - message_id     : 框架转换后的数字短 ID,自动通过 idmap/memory_msgid 还原为真实 d.id
//   - code           : 回应代码 0~5，默认 0=成功
//     0=成功 1=操作失败 2=操作频繁 3=重复操作 4=没有权限 5=仅管理员操作
func handlePutInteraction(c *gin.Context, api openapi.OpenAPI, apiV2 openapi.OpenAPI) {
	var req struct {
		InteractionID string `json:"interaction_id" form:"interaction_id"` // 平台原始 d.id
		MessageID     string `json:"message_id" form:"message_id"`         // 转换后的数字短ID
		Code          int    `json:"code" form:"code"`                     // 回应代码 0~5，默认 0
	}

	// 根据请求方法解析参数
	if c.Request.Method == http.MethodGet {
		// 从URL查询参数解析
		if err := c.ShouldBindQuery(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	} else {
		// 从JSON或表单数据解析
		if err := c.ShouldBind(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}

	// 必须至少提供 interaction_id 或 message_id 之一
	if req.InteractionID == "" && req.MessageID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "interaction_id 或 message_id 至少需要一个。interaction_id为原始d.id，message_id为转换后的数字短ID"})
		return
	}

	// 创建 ActionMessage 实例
	message := callapi.ActionMessage{
		Action: "put_interaction",
		Params: callapi.ParamsContent{
			InteractionID:   req.InteractionID,
			MessageID:       req.MessageID,
			InteractionCode: req.Code,
		},
	}

	// 调用处理函数
	client := &HttpAPIClient{} // 假设HttpAPIClient实现了callapi.Client接口
	retmsg, err := handlers.HandlePutInteraction(client, api, apiV2, message)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 返回处理结果
	c.Header("Content-Type", "application/json")
	c.String(http.StatusOK, retmsg)
}

// 类型转换函数，将interface{}转换为string
func convertToString(value interface{}) string {
	switch v := value.(type) {
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case string:
		return v
	default:
		fmt.Println("Unexpected type:", reflect.TypeOf(value))
		return ""
	}
}

// convertToFileType 将 interface{} 转换为 int（兼容 JSON 整数、浮点数、字符串）
func convertToFileType(value interface{}) int {
	switch v := value.(type) {
	case float64:
		return int(v)
	case int:
		return v
	case string:
		if iv, err := strconv.Atoi(v); err == nil {
			return iv
		}
	}
	return 0
}

func handleDeleteMsg(c *gin.Context, api openapi.OpenAPI, apiV2 openapi.OpenAPI) {
	// 根据请求方法解析参数 GET
	if c.Request.Method == http.MethodGet {
		var req struct {
			UserID    string `json:"user_id,omitempty" form:"user_id"`
			GroupID   string `json:"group_id,omitempty" form:"group_id"`
			ChannelID string `json:"channel_id,omitempty" form:"channel_id"`
			GuildID   string `json:"guild_id,omitempty" form:"guild_id"`
			MessageID string `json:"message_id" form:"message_id"`
		}
		// 从URL查询参数解析
		if err := c.ShouldBindQuery(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		// 构造参数内容，只包括实际有值的字段
		params := callapi.ParamsContent{}

		if req.UserID != "" {
			params.UserID = req.UserID
		}
		if req.GroupID != "" {
			params.GroupID = req.GroupID
		}
		if req.ChannelID != "" {
			params.ChannelID = req.ChannelID
		}
		if req.GuildID != "" {
			params.GuildID = req.GuildID
		}
		if req.MessageID != "" {
			params.MessageID = req.MessageID
		}

		// 创建 ActionMessage 实例
		message := callapi.ActionMessage{
			Action: "delete_msg",
			Params: params,
		}

		// 调用处理函数，假设 handlers.DeleteMsg 已经实现并且适合处理消息删除的操作
		client := &HttpAPIClient{}
		retmsg, err := handlers.DeleteMsg(client, api, apiV2, message)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// 返回处理结果
		c.JSON(http.StatusOK, gin.H{"message": retmsg})
	} else {
		// 根据请求方法解析参数 POST
		// 使用interface{}以适应不同类型的输入，接受动态参数类型
		var req struct {
			UserID    interface{} `json:"user_id,omitempty" form:"user_id"`
			GroupID   interface{} `json:"group_id,omitempty" form:"group_id"`
			ChannelID interface{} `json:"channel_id,omitempty" form:"channel_id"`
			GuildID   interface{} `json:"guild_id,omitempty" form:"guild_id"`
			MessageID interface{} `json:"message_id" form:"message_id"`
		}
		// 从JSON或表单数据解析
		if err := c.ShouldBind(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		// 构造参数内容，只包括实际有值的字段
		params := callapi.ParamsContent{}

		if req.UserID != nil {
			params.UserID = convertToString(req.UserID)
		}
		if req.GroupID != nil {
			params.GroupID = convertToString(req.GroupID)
		}
		if req.ChannelID != nil {
			params.ChannelID = convertToString(req.ChannelID)
		}
		if req.GuildID != nil {
			params.GuildID = convertToString(req.GuildID)
		}
		if req.MessageID != nil {
			params.MessageID = convertToString(req.MessageID)
		}

		// 创建 ActionMessage 实例
		message := callapi.ActionMessage{
			Action: "delete_msg",
			Params: params,
		}

		// 调用处理函数，假设 handlers.DeleteMsg 已经实现并且适合处理消息删除的操作
		client := &HttpAPIClient{}
		retmsg, err := handlers.DeleteMsg(client, api, apiV2, message)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// 返回处理结果
		c.JSON(http.StatusOK, gin.H{"message": retmsg})
	}

}

// handleGetAvatar 处理get_avatar的请求
func handleGetAvatar(c *gin.Context, api openapi.OpenAPI, apiV2 openapi.OpenAPI) {
	var req struct {
		Echo    string `json:"echo" form:"echo"` // Echo值用于标识消息
		GroupID int64  `json:"group_id" form:"group_id"`
		UserID  int64  `json:"user_id" form:"user_id"`
	}

	// 根据请求方法解析参数
	if c.Request.Method == http.MethodGet {
		// 从URL查询参数解析
		if err := c.ShouldBindQuery(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	} else {
		// 从JSON或表单数据解析
		if err := c.ShouldBind(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}

	// 创建 ActionMessage 实例
	message := callapi.ActionMessage{
		Action: "get_avatar",
		Echo:   req.Echo,
		Params: callapi.ParamsContent{
			GroupID: strconv.FormatInt(req.GroupID, 10), // 注意这里需要转换类型，因为 GroupID 是 int64
			UserID:  strconv.FormatInt(req.UserID, 10),
		},
	}

	// 调用处理函数
	client := &HttpAPIClient{} // 假设HttpAPIClient实现了callapi.Client接口
	retmsg, err := handlers.GetAvatar(client, api, apiV2, message)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 返回处理结果
	c.Header("Content-Type", "application/json")
	c.String(http.StatusOK, retmsg)
}

// handleGetFriendList 处理获取好友列表
func handleGetFriendList(c *gin.Context, api openapi.OpenAPI, apiV2 openapi.OpenAPI) {
	var retmsg string
	var err error

	// 使用解析后的参数处理请求
	client := &HttpAPIClient{}
	// 创建 ActionMessage 实例
	message := callapi.ActionMessage{
		Action: "get_friend_list",
	}

	// 调用处理函数
	retmsg, err = handlers.GetFriendList(client, api, apiV2, message)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 返回处理结果
	c.Header("Content-Type", "application/json")
	c.String(http.StatusOK, retmsg)
}

// handleUploadGroupFile 处理群文件分片上传
func handleUploadGroupFile(c *gin.Context, api openapi.OpenAPI, apiV2 openapi.OpenAPI) {
	var req struct {
		GroupID  string      `json:"group_id" form:"group_id"`
		UserID   string      `json:"user_id" form:"user_id"`
		URL      string      `json:"url" form:"url"`
		FileName string      `json:"file_name" form:"file_name"`
		FileType interface{} `json:"file_type" form:"file_type"`
		MsgID    string      `json:"msg_id" form:"msg_id"`
		EventID  string      `json:"event_id" form:"event_id"`
	}

	if c.Request.Method == http.MethodGet {
		if err := c.ShouldBindQuery(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	} else {
		if err := c.ShouldBind(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}

	params := callapi.ParamsContent{
		URL:      req.URL,
		FileName: req.FileName,
	}
	if req.GroupID != "" {
		params.GroupID = req.GroupID
	}
	if req.UserID != "" {
		params.UserID = req.UserID
	}
	if req.EventID != "" {
		params.EventID = req.EventID
	}
	if req.MsgID != "" {
		params.MessageID = req.MsgID
	}
	if req.FileType != nil {
		params.FileType = convertToFileType(req.FileType)
	}

	client := &HttpAPIClient{}
	message := callapi.ActionMessage{
		Action: "upload_group_file",
		Params: params,
	}

	retmsg, err := handlers.HandleUploadGroupFile(client, api, apiV2, message)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Type", "application/json")
	c.String(http.StatusOK, retmsg)
}

// handleUploadPrivateFile 处理 C2C 文件分片上传
func handleUploadPrivateFile(c *gin.Context, api openapi.OpenAPI, apiV2 openapi.OpenAPI) {
	var req struct {
		UserID   string      `json:"user_id" form:"user_id"`
		URL      string      `json:"url" form:"url"`
		FileName string      `json:"file_name" form:"file_name"`
		FileType interface{} `json:"file_type" form:"file_type"`
		MsgID    string      `json:"msg_id" form:"msg_id"`
		EventID  string      `json:"event_id" form:"event_id"`
	}

	if c.Request.Method == http.MethodGet {
		if err := c.ShouldBindQuery(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	} else {
		if err := c.ShouldBind(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}

	params := callapi.ParamsContent{
		URL:      req.URL,
		FileName: req.FileName,
	}
	if req.UserID != "" {
		params.UserID = req.UserID
	}
	if req.EventID != "" {
		params.EventID = req.EventID
	}
	if req.MsgID != "" {
		params.MessageID = req.MsgID
	}
	if req.FileType != nil {
		params.FileType = convertToFileType(req.FileType)
	}

	client := &HttpAPIClient{}
	message := callapi.ActionMessage{
		Action: "upload_private_file",
		Params: params,
	}

	retmsg, err := handlers.HandleUploadPrivateFile(client, api, apiV2, message)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Type", "application/json")
	c.String(http.StatusOK, retmsg)
}
