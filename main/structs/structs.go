package structs

type FriendData struct {
	Nickname string `json:"nickname"`
	Remark   string `json:"remark"`
	UserID   string `json:"user_id"`
}

type Settings struct {
	//反向ws设置
	WsAddress           []string `yaml:"ws_address"`
	WsToken             []string `yaml:"ws_token"`
	ReconnecTimes       int      `yaml:"reconnect_times"`
	HeartBeatInterval   int      `yaml:"heart_beat_interval"`
	LaunchReconectTimes int      `yaml:"launch_reconnect_times"`
	//基础配置
	AppID        uint64 `yaml:"app_id"`
	Uin          int64  `yaml:"uin"`
	Token        string `yaml:"token"`
	ClientSecret string `yaml:"client_secret"`
	ShardCount   int    `yaml:"shard_count"`
	ShardID      int    `yaml:"shard_id"`
	UseUin       bool   `yaml:"use_uin"`
	ShardNum     int    `yaml:"shard_num"`
	//事件订阅类
	TextIntent  []string `yaml:"text_intent"`
	WebhookOnly bool     `yaml:"webhook_only"` // true=纯Webhook模式,跳过QQ Gateway WebSocket连接,但仍注册text_intent中的事件处理器
	//转换类
	GlobalChannelToGroup                     bool   `yaml:"global_channel_to_group"`
	GlobalPrivateToChannel                   bool   `yaml:"global_private_to_channel"`
	GlobalForumToChannel                     bool   `yaml:"global_forum_to_channel"`
	GlobalInteractionToMessage               bool   `yaml:"global_interaction_to_message"`
	GlobalGroupMsgRejectReciveEventToMessage bool   `yaml:"global_group_msg_rre_to_message"`
	GlobalGroupMsgRejectMessage              string `yaml:"global_group_msg_reject_message"`
	GlobalGroupMsgReceiveMessage             string `yaml:"global_group_msg_receive_message"`
	HashID                                   bool   `yaml:"hash_id"`
	IdmapPro                                 bool   `yaml:"idmap_pro"`
	//gensokyo互联类
	Server_dir            string `yaml:"server_dir"`
	Port                  string `yaml:"port"`
	BackupPort            string `yaml:"backup_port"`
	Lotus                 bool   `yaml:"lotus"`
	LotusPassword         string `yaml:"lotus_password"`
	LotusWithoutIdmaps    bool   `yaml:"lotus_without_idmaps"`
	LotusWithoutUploadPic bool   `yaml:"lotus_without_uploadpic"`
	LotusGrpc             bool   `yaml:"lotus_grpc"`
	LotusGrpcPort         int    `yaml:"lotus_grpc_port"`
	//增强配置
	MasterID         []string `yaml:"master_id"`
	RecordSampleRate int      `yaml:"record_sampleRate"`
	RecordBitRate    int      `yaml:"record_bitRate"`
	CardAndNick      string   `yaml:"card_nick"`
	AutoBind         bool     `yaml:"auto_bind"`
	//发图相关
	OssType                 int      `yaml:"oss_type"`
	ImageLimit              int      `yaml:"image_limit"`
	GuildUrlImageToBase64   bool     `yaml:"guild_url_image_to_base64"`
	DirectRecordURL         bool     `yaml:"direct_record_url"`
	GlobalServerTempQQguild bool     `yaml:"global_server_temp_qqguild"`
	ServerTempQQguild       string   `yaml:"server_temp_qqguild"`
	ServerTempQQguildPool   []string `yaml:"server_temp_qqguild_pool"`
	//正向ws设置
	WsServerPath   string `yaml:"ws_server_path"`
	EnableWsServer bool   `yaml:"enable_ws_server"`
	WsServerToken  string `yaml:"ws_server_token"`
	//ssl和链接转换类
	IdentifyFile     bool     `yaml:"identify_file"`
	IdentifyAppids   []int64  `yaml:"identify_appids"`
	Crt              string   `yaml:"crt"`
	Key              string   `yaml:"key"`
	UseSelfCrt       bool     `yaml:"use_self_crt"`
	WebhookPath      string   `yaml:"webhook_path"`
	WebhookPrefixIp  []string `yaml:"webhook_prefix_ip"`
	ForceSSL         bool     `yaml:"force_ssl"`
	HttpPortAfterSSL string   `yaml:"http_port_after_ssl"`
	//日志类
	DeveloperLog     bool `yaml:"developer_log"`
	LogLevel         int  `yaml:"log_level"`
	SaveLogs         bool `yaml:"save_logs"`
	LogSuffixPerMins int  `yaml:"log_suffix_per_mins"`

	//指令魔法类
	RemovePrefix     bool `yaml:"remove_prefix"`
	RemoveAt         bool `yaml:"remove_at"`
	RemoveBotAtGroup bool `yaml:"remove_bot_at_group"`
	AddAtGroup       bool `yaml:"add_at_group"`

	//开发增强类
	SandBoxMode     bool   `yaml:"sandbox_mode"`
	DevMessgeID     bool   `yaml:"dev_message_id"`
	SendError       bool   `yaml:"send_error"`
	SaveError       bool   `yaml:"save_error"`
	DowntimeMessage string `yaml:"downtime_message"`
	MemoryMsgid     bool   `yaml:"memory_msgid"`
	ThreadsRetMsg   bool   `yaml:"threads_ret_msg"`
	NoRetMsg        bool   `yaml:"no_ret_msg"`
	//增长营销类
	SelfIntroduce []string `yaml:"self_introduce"`
	//api修改
	GetGroupListAllGuilds    bool     `yaml:"get_g_list_all_guilds"`
	GetGroupListGuilds       string   `yaml:"get_g_list_guilds"`
	GetGroupListReturnGuilds bool     `yaml:"get_g_list_return_guilds"`
	GetGroupListGuidsType    int      `yaml:"get_g_list_guilds_type"`
	GetGroupListDelay        int      `yaml:"get_g_list_delay"`
	ForwardMsgLimit          int      `yaml:"forward_msg_limit"`
	CustomBotName            string   `yaml:"custom_bot_name"`
	TransFormApiIds          bool     `yaml:"transform_api_ids"`
	AutoPutInteraction       bool     `yaml:"auto_put_interaction"`
	PutInteractionDelay      int      `yaml:"put_interaction_delay"`
	PutInteractionExcept     []string `yaml:"put_interaction_except"`
	//webhook 模式下，收到 INTERACTION_CREATE 时把 code 写进 200 OK 响应体: {"op":12,"code":N}
	//在 webhook_resp_wait_ms 倒计时内若应用端通过 send_group_msg/send_private_msg/put_interaction
	//指定了 interaction_id+code，则使用应用端给定的 code；超时则使用此 webhook_resp_code 作为兜底。
	//0=兜底走 code 0 操作成功(平台默认) 1=操作失败 2=操作频繁 3=重复操作 4=没有权限 5=仅管理员操作
	WebhookRespCode int `yaml:"webhook_resp_code"`
	//webhook 模式下等待应用端覆盖 code 的最大毫秒数。
	//0=立即用 webhook_resp_code 兜底回 200 OK；>0=注册 pending 槽位，最多等 N 毫秒，
	//期间应用端可通过 HTTP API 投递 code 覆盖兜底（避免重复按按钮时刷屏文字提示）。
	WebhookRespWaitMs int `yaml:"webhook_resp_wait_ms"`
	//onebot修改
	DisableErrorChan bool `yaml:"disable_error_chan"`
	StringAction     bool `yaml:"string_action"`
	//url相关
	VisibleIp    bool `yaml:"visible_ip"`
	UrlToQrimage bool `yaml:"url_to_qrimage"`
	QrSize       int  `yaml:"qr_size"`
	TransferUrl  bool `yaml:"transfer_url"`
	//框架修改
	Title   string `yaml:"title"`
	FrpPort string `yaml:"frp_port"`
	//MD相关
	CustomTemplateID string `yaml:"custom_template_id"`
	KeyBoardID       string `yaml:"keyboard_id"`
	NativeMD         bool   `yaml:"native_md"`
	EntersAsBlock    bool   `yaml:"enters_as_block"`
	//发送行为修改
	LazyMessageId bool   `yaml:"lazy_message_id"`
	RamDomSeq     bool   `yaml:"ramdom_seq"`
	BotForumTitle string `yaml:"bot_forum_title"`
	AtoPCount     int    `yaml:"AMsgRetryAsPMsg_Count"`
	SendDelay     int    `yaml:"send_delay"`
	//错误临时修复类
	Fix11300          bool `yaml:"fix_11300"`
	HttpOnlyBot       bool `yaml:"http_only_bot"`
	DoNotReplaceAppid bool `yaml:"do_not_replace_appid"`

	//HTTP API配置
	HttpAddress         string   `yaml:"http_address"`
	AccessToken         string   `yaml:"http_access_token"`
	HttpVersion         int      `yaml:"http_version"`
	HttpTimeOut         int      `yaml:"http_timeout"`
	PostUrl             []string `yaml:"post_url"`
	PostSecret          []string `yaml:"post_secret"`
	PostMaxRetries      []int    `yaml:"post_max_retries"`
	PostRetriesInterval []int    `yaml:"post_retries_interval"`
	//腾讯云
	TencentBucketName   string `yaml:"t_COS_BUCKETNAME"`
	TencentBucketRegion string `yaml:"t_COS_REGION"`
	TencentCosSecretid  string `yaml:"t_COS_SECRETID"`
	TencentSecretKey    string `yaml:"t_COS_SECRETKEY"`
	TencentAudit        bool   `yaml:"t_audit"`
	//百度云
	BaiduBOSBucketName string `yaml:"b_BOS_BUCKETNAME"`
	BaiduBCEAK         string `yaml:"b_BCE_AK"`
	BaiduBCESK         string `yaml:"b_BCE_SK"`
	BaiduAudit         int    `yaml:"b_audit"`
	//阿里云
	AliyunEndpoint        string `yaml:"a_OSS_EndPoint"`
	AliyunAccessKeyId     string `yaml:"a_OSS_AccessKeyId"`
	AliyunAccessKeySecret string `yaml:"a_OSS_AccessKeySecret"`
	AliyunBucketName      string `yaml:"a_OSS_BucketName"`
	AliyunAudit           bool   `yaml:"a_audit"`
}

type InterfaceBody struct {
	Content        string   `json:"content"`
	State          int      `json:"state"`
	PromptKeyboard []string `json:"prompt_keyboard,omitempty"`
	ActionButton   int      `json:"action_button,omitempty"`
	CallbackData   string   `json:"callback_data,omitempty"`
}
