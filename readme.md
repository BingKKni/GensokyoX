<p align="center">
  <a href="https://www.github.com/hoshinonyaruko/gensokyo">
    <img src="images/head.gif" width="200" height="200" alt="gensokyo">
  </a>
</p>

<div align="center">

# gensokyo

_✨ 基于 [OneBot](https://github.com/howmanybots/onebot/blob/master/README.md) QQ官方机器人Api Golang 原生实现 ✨_  


</div>

<p align="center">
  <a href="https://raw.githubusercontent.com/hoshinonyaruko/gensokyo/main/LICENSE">
    <img src="https://img.shields.io/github/license/hoshinonyaruko/gensokyo" alt="license">
  </a>
  <a href="https://github.com/hoshinonyaruko/gensokyo/releases">
    <img src="https://img.shields.io/github/v/release/hoshinonyaruko/gensokyo?color=blueviolet&include_prereleases" alt="release">
  </a>
  <a href="https://github.com/howmanybots/onebot/blob/master/README.md">
    <img src="https://img.shields.io/badge/OneBot-v11-blue?style=flat&logo=data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAEAAAABABAMAAABYR2ztAAAAIVBMVEUAAAAAAAADAwMHBwceHh4UFBQNDQ0ZGRkoKCgvLy8iIiLWSdWYAAAAAXRSTlMAQObYZgAAAQVJREFUSMftlM0RgjAQhV+0ATYK6i1Xb+iMd0qgBEqgBEuwBOxU2QDKsjvojQPvkJ/ZL5sXkgWrFirK4MibYUdE3OR2nEpuKz1/q8CdNxNQgthZCXYVLjyoDQftaKuniHHWRnPh2GCUetR2/9HsMAXyUT4/3UHwtQT2AggSCGKeSAsFnxBIOuAggdh3AKTL7pDuCyABcMb0aQP7aM4AnAbc/wHwA5D2wDHTTe56gIIOUA/4YYV2e1sg713PXdZJAuncdZMAGkAukU9OAn40O849+0ornPwT93rphWF0mgAbauUrEOthlX8Zu7P5A6kZyKCJy75hhw1Mgr9RAUvX7A3csGqZegEdniCx30c3agAAAABJRU5ErkJggg==" alt="gensokyo">
  </a>
  <a href="https://github.com/hoshinonyaruko/gensokyo/actions">
    <img src="images/badge.svg" alt="action">
  </a>
  <a href="https://goreportcard.com/report/github.com/hoshinonyaruko/gensokyo">
  <img src="https://goreportcard.com/badge/github.com/hoshinonyaruko/gensokyo" alt="GoReportCard">
  </a>
</p>

<p align="center">
  <a href="https://github.com/howmanybots/onebot/blob/master/README.md">文档</a>
  ·
  <a href="https://github.com/hoshinonyaruko/gensokyo/releases">下载</a>
  ·
  <a href="https://github.com/hoshinonyaruko/gensokyo/releases">开始使用</a>
  ·
  <a href="https://github.com/hoshinonyaruko/gensokyo/blob/master/CONTRIBUTING.md">参与贡献</a>
</p>
<p align="center">
  <a href="https://gensokyo.bot">项目主页:gensokyo.bot</a>
</p>

## 引用
- [`tencent-connect/botgo`](https://github.com/tencent-connect/botgo): 本项目引用了此项目,并做了一些改动.

## 介绍
gensokyo兼容 [OneBot-v11](https://github.com/botuniverse/onebot-11) ，并在其基础上做了一些扩展，详情请看 OneBot 的文档。

Gensokyo文档(施工中):[起步](/docs/起步-注册QQ开放平台&启动gensokyo.md)

可将官方的websocket和api转换至onebotv11格式,

实现插件开发和用户开发者无需重新开发,复用过往生态的插件和使用体验.

持续完善中.....

## 特别鸣谢

- [`tencent-connect/botgo`](https://github.com/tencent-connect/botgo): QQ 官方 SDK，本项目在其基础上进行了重构与扩展.

### 接口

- [x] HTTP API
- [x] 反向 HTTP POST
- [x] 正向 WebSocket
- [x] 反向 WebSocket

### 拓展支持

> 拓展 API 可前往 [文档](docs/api介绍.md) 查看

- [x] 连接多个ws地址
- [x] 将频道虚拟成群事件
- [x] 将私信虚拟成频道或群事件
- [x] 可编辑的数据库
- [x] 文字,图片,语音,视频,MD,支持多种类型发送
- [x] 支持全域,频道,频道私聊,群,群私聊
- [x] 主动信息失败自动转被动,提高信息传达可靠性
- [x] 提前于官方支持群列表 群成员 api
- [x] 完善的重连,健壮的连接能力.
- [x] 支持 [CQ:markdown,data=] Markdown 发送
- [x] 自动 url 转换(短链服务, 需自备域名)
- [x] 可自定义图片压缩 / 腾讯云COS / 阿里云OSS / 百度云 图床服务
- [x] 支持 OneBot v11 的 array / message segment 两种上报格式
- [x] webhook 模式 (可迁移到纯 HTTP 推送, 不依赖 QQ 官方 WebSocket)
- [x] 按钮回调交互(INTERACTION_CREATE) 手动回应
- [x] 持续更新~


### 实现

<details>
<summary>已实现 CQ 码</summary>

> 仅列出代码中真实有处理逻辑的 CQ 码，其它 OneBot 标准 CQ 码受 QQ 官方 API 能力限制暂未支持。

| CQ 码         | 功能                                  |
| ------------- | ------------------------------------- |
| [CQ:at]       | @某人                                 |
| [CQ:image]    | 图片(本地路径 / http(s) / base64)     |
| [CQ:record]   | 语音(本地路径 / http(s) / base64)     |
| [CQ:video]    | 短视频(http(s))                       |
| [CQ:music]    | QQ 音乐分享(`type=qq,id=`)            |
| [CQ:reply]    | 回复(收到时移除标记，发送时不支持)    |
| [CQ:markdown] | Markdown 卡片(`data=base64://`)       |
| [CQ:avatar]   | 头像(拓展 CQ 码，将 QQ 号渲染成头像)  |

</details>

<details>
<summary>已实现 API</summary>

> 仅列出代码中通过 `callapi.RegisterHandler` 真实注册的 action。

#### 消息发送

| API                          | 功能                          |
| ---------------------------- | ----------------------------- |
| /send_private_msg            | 发送私聊消息                  |
| /send_private_msg_async      | 异步发送私聊消息              |
| /send_private_msg_sse        | SSE 流式私聊消息              |
| /send_group_msg              | 发送群消息                    |
| /send_to_group               | 同 /send_group_msg 的别名     |
| /send_group_msg_async        | 异步发送群消息                |
| /send_group_msg_raw          | 直接发送原始群消息(不走转换)  |
| /send_msg                    | 通用发送消息                  |
| /send_msg_async              | 异步通用发送消息              |
| /send_guild_channel_msg      | 发送频道消息                  |
| /send_guild_channel_forum    | 发送频道帖子                  |
| /send_group_forward_msg      | 发送合并转发(群)              |

#### 消息管理

| API                          | 功能                          |
| ---------------------------- | ----------------------------- |
| /delete_msg                  | 撤回信息                      |
| /put_interaction             | 手动回应按钮回调              |
| /.handle_quick_operation     | 对事件执行快速操作            |
| /.handle_quick_operation_async | 异步快速操作                |

#### 信息查询

| API                          | 功能                          |
| ---------------------------- | ----------------------------- |
| /get_avatar                  | 获取头像 url                  |
| /get_friend_list             | 获取好友列表                  |
| /get_group_info              | 获取群/频道信息               |
| /get_group_list              | 获取群列表                    |
| /get_group_member_info       | 获取群成员信息                |
| /get_group_member_list       | 获取群成员列表                |
| /get_guild_list              | 获取频道列表                  |
| /get_guild_channel_list      | 获取子频道列表                |

#### 群管理

| API                          | 功能                          |
| ---------------------------- | ----------------------------- |
| /set_group_ban               | 群组单人禁言(仅频道)          |
| /set_group_whole_ban         | 群组全员禁言(仅频道)          |

#### 文件上传

| API                          | 功能                          |
| ---------------------------- | ----------------------------- |
| /upload_group_file           | 群文件分片上传                |
| /upload_private_file         | C2C 文件分片上传              |

</details>

<details>
<summary>已实现 Event</summary>

> 仅列出代码中真实会上报的事件类型。

#### 消息事件

| message_type | 来源                       |
| ------------ | -------------------------- |
| private      | C2C 私聊 / 频道私信        |
| group        | QQ 群消息 / 频道虚拟成群   |
| guild        | 频道消息 / 频道帖子        |

#### 通知事件

| notice_type     | 触发场景                                       |
| --------------- | ---------------------------------------------- |
| group_increase  | 机器人被加入新群(SubType=invite)               |
| group_decrease  | 机器人被踢出群(SubType=kick_me)                |
| interaction     | 按钮回调(INTERACTION_CREATE)                   |
| group_msg_reject  | 用户关闭机器人主动消息推送(可选转为消息事件) |
| group_msg_receive | 用户开启机器人主动消息推送(可选转为消息事件) |

</details>

## 关于 ISSUE

以下 ISSUE 会被直接关闭

- 提交 BUG 不使用 Template
- 询问已知问题
- 提问找不到重点
- 重复提问

> 请注意, 开发者并没有义务回复您的问题. 您应该具备基本的提问技巧。  
> 有关如何提问，请阅读[《提问的智慧》](https://github.com/ryanhanwu/How-To-Ask-Questions-The-Smart-Way/blob/main/README-zh_CN.md)

## 性能

10mb内存占用 端口错开可多开 稳定运行无报错