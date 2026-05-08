package template

const ConfigTemplate = `
version: 1
settings:
  #反向ws设置
  ws_address: ["ws://<YOUR_WS_ADDRESS>:<YOUR_WS_PORT>"] # WebSocket服务的地址 支持多个["","",""]
  ws_token: ["","",""]              #连接wss地址时服务器所需的token,按顺序一一对应,如果是ws地址,没有密钥,请留空.
  reconnect_times : 100             #反向ws连接失败后的重试次数,希望一直重试,可设置9999
  heart_beat_interval : 5          #反向ws心跳间隔 单位秒 推荐5-10
  launch_reconnect_times : 1        #启动时尝试反向ws连接次数,建议先打开应用端再开启gensokyo,因为启动时连接会阻塞webui启动,默认只连接一次,可自行增大

  #基础设置
  app_id: 12345                                      # 你的应用ID
  uin : 0                                            # 你的机器人QQ号,点击机器人资料卡查看  
  use_uin : false                                    # false=使用appid作为机器人id,true=使用机器人QQ号,需设置正确的uin
  token: "<YOUR_APP_TOKEN>"                          # 你的应用令牌
  client_secret: "<YOUR_CLIENT_SECRET>"              # 你的客户端密钥
  shard_count: 1                    #分片数量 默认1
  shard_id: 0                       #当前分片id 默认从0开始,详细请看 https://bot.q.qq.com/wiki/develop/api/gateway/reference.html
  shard_num: 1                      #接口调用超过频率限制时,如果不想要多开gsk,尝试调大.gsk会尝试连接到n个分片处理信息. n为你所配置的值.与 shard_count和shard_id互不相干.

  #事件订阅
  webhook_only: false                                # true=纯Webhook模式,跳过QQ Gateway WebSocket连接(官方WS已于24年底逐步下线,推荐迁移至Webhook). text_intent中的处理器仍会注册,确保QQ推送的HTTP事件可被正常处理. 需同时在QQ开放平台管理端配置Webhook回调地址.
  text_intent:                                       # 请根据公域 私域来选择intent,错误的intent将连接失败
    - "ATMessageEventHandler"                        # 频道at信息
    - "DirectMessageHandler"                         # 私域频道私信(dms)
    # - "ReadyHandler"                               # 连接成功
    # - "ErrorNotifyHandler"                         # 连接关闭
    # - "GuildEventHandler"                          # 频道事件
    # - "MemberEventHandler"                         # 频道成员新增
    # - "ChannelEventHandler"                        # 频道事件
    # - "CreateMessageHandler"                       # 频道不at信息 私域机器人需要开启 公域机器人开启会连接失败
    # - "InteractionHandler"                         # 添加频道互动回应 卡片按钮data回调事件
    # - "GroupATMessageEventHandler"                 # 群at信息 仅频道机器人时候需要注释
    # - "C2CMessageEventHandler"                     # 群私聊 仅频道机器人时候需要注释
    # - "ThreadEventHandler"                         # 频道发帖事件 仅频道私域机器人可用

  #转换类
  global_channel_to_group: true                      # 是否将频道转换成群 默认true
  global_private_to_channel: false                   # 是否将私聊转换成频道 如果是群场景 会将私聊转为群(方便提审\测试)
  global_forum_to_channel: false                     # 是否将频道帖子信息转化为频道 子频道信息 如果开启global_channel_to_group会进一步转换为群信息
  global_interaction_to_message : false              # 是否将按钮和表态回调转化为消息 仅在设置了按钮回调中的message时有效
  global_group_msg_rre_to_message : false            # 是否将用户开关机器人资料页的机器人推送开关 产生的事件转换为文本信息并发送给应用端.false将使用onebotv11的notice类型上报.
  global_group_msg_reject_message : "机器人主动消息被关闭"  # 当开启 global_group_msg_rre_to_message 时,机器人主动信息被关闭将上报的信息. 自行添加intent - GroupMsgRejectHandler
  global_group_msg_receive_message : "机器人主动消息被开启" # 建议设置为无规则复杂随机内容,避免用户指令内容碰撞. 自行添加 intent - GroupMsgReceiveHandler
  hash_id : true                                    # 使用hash来进行idmaps转换,可以让user_id不是123开始的递增值
  idmap_pro : false                                  # 需开启hash_id配合,高级id转换增强,可以多个真实值bind到同一个虚拟值,对于每个用户,每个群\私聊\判断私聊\频道,都会产生新的虚拟值,但可以多次bind,bind到同一个数字.数据库负担会变大.

  #Gensokyo互联类
  server_dir: "<YOUR_SERVER_DIR>"                    # Lotus地址.不带http头的域名或ip,提供图片上传服务的服务器(图床)需要带端口号. 如果需要发base64图,需为公网ip,且开放对应端口
  port: "15630"                                      # Lotus端口.idmaps和图床对外开放的端口号,若要连接到另一个gensokyo,也是链接端口
  backup_port : "5200"                               # 当totus为ture时,port值不再是本地webui的端口,使用lotus_Port来访问webui
  lotus: false                                       # lotus特性默认为false,当为true时,将会连接到另一个lotus为false的gensokyo。使用它提供的图床和idmaps服务(场景:同一个机器人在不同服务器运行,或内网需要发送base64图)。如果需要发送base64图片,需要设置正确的公网server_dir和开放对应的port, lotus鉴权 设置后,从gsk需要保持相同密码来访问主gsk
  lotus_password : "" 
  lotus_without_idmaps: false       #lotus只通过url,图片上传,语音,不通过id转换,在本地当前gsk维护idmaps转换.
  lotus_without_uploadpic : false   #lotus只转换id,不进行图片上传.
  lotus_grpc : false                #实验特性,使用grpc进行lotus连接.提高性能.
  lotus_grpc_port : 50051           #grpc的端口,连接与被连接需保持一致.并且在防火墙放通此端口.

  #增强配置项                                           
  master_id : ["1","2"]             #群场景尚未开放获取管理员和列表能力,手动从日志中获取需要设置为管理,的user_id并填入(适用插件有权限判断场景)
  record_sampleRate : 24000         #语音文件的采样率 最高48000 默认24000 单位Khz
  record_bitRate : 24000            #语音文件的比特率 默认25000 代表 25 kbps 最高无限 请根据带宽 您发送的实际码率调整
  card_nick : ""                    #默认为空,连接mirai-overflow时,请设置为非空,这里是机器人对用户称谓,为空为插件获取,mirai不支持
  auto_bind : true                  #测试功能,后期会移除

  #发图相关
  oss_type : 0                      #请完善后方具体配置 完成#腾讯云配置...,0代表配置server dir port服务器自行上传(省钱),1,腾讯cos存储桶 2,百度oss存储桶 3,阿里oss存储桶
  image_sizelimit : 0               #代表kb 腾讯api要求图片1500ms完成传输 如果图片发不出 请提升上行或设置此值 默认为0 不压缩
  image_limit : 100                 #每分钟上传的最大图片数量,可自行增加
  guild_url_image_to_base64 : false #解决频道发不了某些url图片,报错40003问题
  url_pic_transfer : false          #把图片url(任意来源图链)变成你备案的白名单url 需要较高上下行+ssl+自备案域名+设置白名单域名(暂时不需要)
  uploadpicv2_b64: true             #uploadpicv2接口使用base64直接上传 https://www.yuque.com/km57bt/hlhnxg/ig2nk88fccykn6dm
  direct_record_url: false          #语音使用URL时是否直接使用原始URL,而不进行下载、转码和重新上传,避免复杂工序,但可能遇到URL链接无法播放的问题
  global_server_temp_qqguild : false                     #需设置server_temp_qqguild,公域私域均可用,以频道为底层发图,速度快,该接口为进阶接口,使用有一定难度.
  server_temp_qqguild : "0"            #在v3图片接口采用固定的子频道号,可以是帖子子频道 https://www.yuque.com/km57bt/hlhnxg/uqmnsno3vx1ytp2q
  server_temp_qqguild_pool : []      #填写v3发图接口的endpoint http://127.0.0.1:12345/uploadpicv3 当填写多个时采用循环方式负载均衡,注,不包括自身,如需要自身也要填写

  #正向ws设置
  ws_server_path : "ws"             #默认监听0.0.0.0:port/ws_server_path 若有安全需求,可不放通port到公网,或设置ws_server_token 若想监听/ 可改为"",若想监听到不带/地址请写nil
  enable_ws_server: true            #是否启用正向ws服务器 监听server_dir:port/ws_server_path
  ws_server_token : "12345"         #正向ws的token 不启动正向ws可忽略 可为空

  #SSL配置类 和 白名单域名自动验证
  identify_file : true               #自动生成域名校验文件,在q.qq.com配置信息URL,在server_dir填入自己已备案域名,正确解析到机器人所在服务器ip地址,机器人即可发送链接
  identify_appids : []               #默认不需要设置,完成SSL配置类+server_dir设置为域名+完成备案+ssl全套设置后,若有多个机器人需要过域名校验(自己名下)可设置,格式为,整数appid,组成的数组
  crt : ""                           #证书路径 从你的域名服务商或云服务商申请签发SSL证书(qq要求SSL) 
  key : ""                           #密钥路径 Apache（crt文件、key文件）示例: "C:\\123.key" \需要双写成\\
  webhook_path : "webhook"           #webhook监听的地址,默认\webhook
  force_ssl : false                  #默认当port设置为443时启用ssl,true可以在其他port设置下强制启用ssl.
  http_port_after_ssl : "444"       # 指定启动SSL之后的备用HTTP服务器的端口号，默认为444
  
  #日志类
  developer_log : false             #开启开发者日志 默认关闭
  log_level : 1                     # 0=debug 1=info 2=warning 3=error 默认1
  save_logs : false                 #自动储存日志
  log_suffix_per_mins : 0           #默认0,代表不切分日志文件,设置60代表每60分钟储存一个日志文件,如果你的日志文件太大打不开,可以设置这个到合适的时间范围.

  #webui设置
  disable_webui: false              #禁用webui
  server_user_name : "useradmin"    #默认网页面板用户名
  server_user_password : "admin"    #默认网页面板密码

  #指令魔法类
  remove_prefix : false             #是否忽略公域机器人指令前第一个/
  remove_at : false                 #是否忽略公域机器人指令前第一个[CQ:aq,qq=机器人] 场景(公域机器人,但插件未适配at开头)
  remove_bot_at_group : true        #因为群聊机器人不支持发at,开启本开关会自动隐藏群机器人发出的at(不影响频道场景)
  add_at_group : false              #自动在群聊指令前加上at,某些机器人写法特别,必须有at才反应时,请打开,默认请关闭(如果需要at,不需要at指令混杂,请优化代码适配群场景,群场景目前没有at概念)

  #开发增强类
  develop_access_token_dir : ""     #开发者测试环境access_token自定义获取地址 默认留空 请留空忽略
  develop_bot_id : "1234"           #开发者环境需自行获取botid 填入 用户请不要设置这两行...开发者调试用
  sandbox_mode : false              #默认false 如果你只希望沙箱频道使用,请改为true
  dev_message_id : false            #在沙盒和测试环境使用无限制msg_id 仅沙盒有效,正式环境请关闭,内测结束后,tx侧未来会移除
  send_error : true                 #将报错用文本发出,避免机器人被审核报无响应
  save_error : false                #将保存保存在log文件夹,方便开发者定位发送错误.
  downtime_message : "我正在维护中~请不要担心,维护结束就回来~维护时间:(1小时)"
  memory_msgid : false              #当你的机器人单日信息量超过100万,就需要高性能SSD或者开启这个选项了.部分依赖msgid的功能可能会受影响(如delete_msg)
  threads_ret_msg : false           #异步,并发发送回执信息 仅ws可用.
  no_ret_msg : false                #当你的信息量达到1000万/天的时候,并且你的业务不需要获取回调信息,此时直接屏蔽是最好的选择,可以提升50%收发性能. 需应用端适配!!!

  #增长营销类(推荐gensokyo-broadcast项目)
  self_introduce : ["",""]          #自我介绍,可设置多个随机发送,当不为空时,机器人被邀入群会发送自定义自我介绍 需手动添加新textintent   - "GroupAddRobotEventHandler"   - "GroupDelRobotEventHandler"


  #API修改
  get_g_list_all_guilds : false     #在获取群列表api时,轮询获取全部的频道列表(api一次只能获取100个),建议仅在广播公告通知等特别场景时开启.
  get_g_list_delay : 500            #轮询时的延迟时间,毫秒数.
  get_g_list_guilds_type : 0        #0=全部返回,1=获取第1个子频道.以此类推.可以缩减返回值的大小.
  get_g_list_guilds : "10"          #在获取群列表api时,一次返回的频道数量.这里是string,不要去掉引号.最大100(5分钟内连续请求=翻页),获取全部请开启get_g_list_return_guilds.
  get_g_list_return_guilds : true   #获取群列表时是否返回频道列表.
  forward_msg_limit : 3             #发送折叠转发信息时的最大限制条数 若要发转发信息 请设置lazy_message_id为true
  custom_bot_name : "Gensokyo全域机器人"   #自定义api返回的机器人名字,会在api调用中返回,默认Gensokyo全域机器人
  transform_api_ids : true          #对get_group_menmber_list\get_group_member_info\get_group_list生效,是否在其中返回转换后的值(默认转换,不转换请自行处理插件逻辑,比如调用gsk的http api转换)
  auto_put_interaction : false      #自动回应按钮回调的/interactions/{interaction_id} 注本api需要邮件申请,详细方法参考群公告:196173384
  put_interaction_delay : 0         #单位毫秒 表示回应已收到回调类型的按钮的毫秒数 会按用户进行区分 非全局delay
  put_interaction_except : []       #自动回复按钮的例外,当你想要自己用api回复,回复特殊状态时,将指令前缀填入进去(根据按钮的data字段判断的)
  webhook_resp_code : 0             #webhook模式下,在收到INTERACTION_CREATE时,把code写进200 OK响应体: {"op":12,"code":N},用于覆盖按钮回应提示,无需邮件申请白名单. 作为应用端无覆盖时的兜底code. 0=兜底走code 0操作成功 1=操作失败 2=操作频繁 3=重复操作 4=没有权限 5=仅管理员操作
  webhook_resp_wait_ms : 1000       #webhook模式下,等待应用端通过send_group_msg/send_private_msg/put_interaction提供code的最大毫秒数. 期间应用端指定的code覆盖兜底webhook_resp_code. 0=不等待立即用兜底code回复(避免重复按按钮的"已重复操作"提示刷屏)

  #Onebot修改
  disable_error_chan : false        #禁用ws断开时候将信息放入补发频道,当信息非常多时可能导致冲垮应用端,可以设置本选项为true.
  string_action : false             #开启后将兼容action调用中使用string形式的user_id和group_id.

  #URL相关
  visible_ip : false                #转换url时,如果server_dir是ip true将以ip形式发出url 默认隐藏url 将server_dir配置为自己域名可以转换url
  url_to_qrimage : false            #将信息中的url转换为二维码单独作为图片发出,需要同时设置  #SSL配置类 机器人发送URL设置 的 transfer_url 为 true visible_ip也需要为true
  qr_size : 200                     #二维码尺寸,单位像素
  transfer_url : true                #默认开启,关闭后自理url发送,配置server_dir为你的域名,配置crt和key后,将域名/url和/image在q.qq.com后台通过校验,自动使用302跳转处理机器人发出的所有域名.

  #框架修改
  title : "Gensokyo © 2023 - Hoshinonyaruko"              #程序的标题 如果多个机器人 可根据标题区分
  frp_port : "0"                    #不使用请保持为0,frp的端口,frp有内外端口,请在frp软件设置gensokyo的port,并将frp显示的对外端口填入这里
 
  #MD相关
  custom_template_id : ""           #自动转换图文信息到md所需要的id *需要应用端支持双方向echo
  keyboard_id : ""                  #自动转换图文信息到md所需要的按钮id *需要应用端支持双方向echo
  native_md : false                 #自动转换图文信息到md,使用原生markdown能力.
  enters_as_block : false           #自动转换图文信息到md,\r \r\n \n 替换为空格.

  #发送行为修改
  lazy_message_id : false           #false=message_id 条条准确对应 true=message_id 按时间范围随机对应(适合主动推送bot)前提,有足够多的活跃信息刷新id池
  ramdom_seq : false                #当多开gensokyo时,如果遇到群信息只能发出一条,请开启每个gsk的此项.(建议使用一个gsk连接多个应用)
  bot_forum_title : "机器人帖子"                      # 机器人发帖子回复默认标题 
  AMsgRetryAsPMsg_Count : 30        #当主动信息发送失败时,自动转为后续的被动信息发送,需要开启Lazy message id,该配置项为所有群、频道的主动转被动消息队列最大长度,建议30-100,无上限
  send_delay : 300                  #单位 毫秒 默认300ms 可以视情况减少到100或者50

  #错误临时修复类
  fix_11300: false                  #修复11300报错,需要在develop_bot_id填入自己机器人的appid. 11300原因暂时未知,临时修复方案.
  http_only_bot : false             #这个配置项会自动配置,请不要修改,保持false.
  do_not_replace_appid : false      #在频道内机器人尝试at自己回at不到,保持false.群内机器人有发送用户头像url的需求时,true(因为用户头像url包含了appid,如果false就会出错.)
  
  #HTTP API配置-正向http
  http_address: ""                  #http监听地址 与websocket独立 示例:0.0.0.0:5700 为空代表不开启
  http_access_token: ""             #http访问令牌
  http_version : 11                 #暂时只支持11
  http_timeout: 5                   #反向 HTTP 超时时间, 单位秒，<5 时将被忽略

  #HTTP API配置-反向http
  post_url: [""]                    #反向HTTP POST地址列表 为空代表不开启 示例:http://192.168.0.100:5789
  post_secret: [""]                 #密钥
  post_max_retries: [3]             #最大重试,0 时禁用
  post_retries_interval: [1500]     #重试时间,单位毫秒,0 时立即

  #腾讯云配置
  t_COS_BUCKETNAME : ""             #存储桶名称
  t_COS_REGION : ""                 #COS_REGION 所属地域()内的复制进来 可以在控制台查看 https://console.cloud.tencent.com/cos5/bucket, 关于地域的详情见 https://cloud.tencent.com/document/product/436/6224
  t_COS_SECRETID : ""               #用户的 SecretId,建议使用子账号密钥,授权遵循最小权限指引，降低使用风险。子账号密钥获取可参考 https://cloud.tencent.com/document/product/598/37140
  t_COS_SECRETKEY : ""              #用户的 SECRETKEY 请腾讯云搜索 api密钥管理 生成并填写.妥善保存 避免泄露
  t_audit : false                   #是否审核内容 请先到控制台开启

  #百度云配置
  b_BOS_BUCKETNAME : ""             #百度智能云-BOS控制台-Bucket列表-需要选择的存储桶-域名发布信息-完整官方域名-填入 形如 hellow.gz.bcebos.com
  b_BCE_AK : ""                     #百度 BCE的 AK 获取方法 https://cloud.baidu.com/doc/BOS/s/Tjwvyrw7a 
  b_BCE_SK : ""                     #百度 BCE的 SK 
  b_audit : 0                       #0 不审核 仅使用oss, 1 使用oss+审核, 2 不使用oss 仅审核

  #阿里云配置
  a_OSS_EndPoint : ""               #形如 https://oss-cn-hangzhou.aliyuncs.com 这里获取 https://oss.console.aliyun.com/bucket/oss-cn-shenzhen/sanaee/overview
  a_OSS_BucketName : ""             #要使用的桶名称,上方EndPoint不包含这个名称,如果有,请填在这里
  a_OSS_AccessKeyId : ""            #阿里云控制台-最右上角点击自己头像-AccessKey管理-然后管理和生成
  a_OSS_AccessKeySecret : ""
  a_audit : false                   #是否审核图片 请先开通阿里云内容安全需企业认证。具体操作 请参见https://help.aliyun.com/document_detail/69806.html

`
const Logo = `
'                                                                                                      
'    ,hakurei,                                                      ka                                  
'   ho"'     iki                                                    gu                                  
'  ra'                                                              ya                                  
'  is              ,kochiya,    ,sanae,    ,Remilia,   ,Scarlet,    fl   and  yu        ya   ,Flandre,   
'  an      Reimu  'Dai   sei  yas     aka  Rei    sen  Ten     shi  re  sca    yu      ku'  ta"     "ko  
'  Jun        ko  Kirisame""  ka       na    Izayoi,   sa       ig  Koishi       ko   mo'   ta       ga  
'   you.     rei  sui   riya  ko       hi  Ina    baI  'ran   you   ka  rlet      komei'    "ra,   ,sa"  
'     "Marisa"      Suwako    ji       na   "Sakuya"'   "Cirno"'    bu     sen     yu''        Satori  
'                                                                                ka'                   
'                                                                               ri'                    
`
