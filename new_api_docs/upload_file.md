# 大文件分片上传 API

Gensokyo 新增了群聊和私聊的大文件分片上传能力，对应 QQ 开放平台的 `upload_prepare` / `upload_part_finish` / `files(upload_id)` 三步流程。框架内部自动处理分片逻辑，调用方只需提供一个可下载的文件 URL。

---

## 端点一览

| 端点 | action 名 | 说明 |
|------|-----------|------|
| `/upload_group_file` | `upload_group_file` | 群文件分片上传 |
| `/upload_private_file` | `upload_private_file` | C2C（私聊）文件分片上传 |

HTTP 和 WebSocket 两种调用路径均可使用，支持 GET（query params）和 POST（JSON body）。

---

## 群文件上传 `/upload_group_file`

### 请求参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `group_id` | string/int | 是 | 目标群号 |
| `url` | string | 是 | 文件下载地址（Gensokyo 会从此 URL 流式下载并分片上传） |
| `file_name` | string | 否 | 文件名，默认 `"file"` |
| `file_type` | int | 否 | 1=图片 2=视频 3=语音 4=文件，默认 `4` |
| `user_id` | string/int | 否 | 用户 ID，用于 `lazy_message_id` 匹配更精准的 msg_id |
| `msg_id` | string | 否 | 被动消息 ID。HTTP API 和 WS 路径均可传入。若不传且开启了 `lazy_message_id`，框架自动获取 |
| `event_id` | string | 否 | 事件 ID，与 msg_id 二选一 |

### HTTP 调用示例

**GET 方式**（适配 HuanmengX 现有风格）：

```
GET http://127.0.0.1:5700/upload_group_file?group_id=123456&url=https%3A%2F%2Fyour-server.com%2Farchive.zip&file_name=archive.zip&file_type=4
```

**POST 方式**：

```json
POST http://127.0.0.1:5700/upload_group_file
Content-Type: application/json

{
  "group_id": "123456",
  "url": "https://your-server.com/archive.zip",
  "file_name": "archive.zip",
  "file_type": 4,
  "user_id": "654321",
  "msg_id": "可选的被动消息ID"
}
```

### WebSocket 调用示例

通过反向/正向 WS 发送 action JSON，框架自动分发到 handler：

```json
{
  "action": "upload_group_file",
  "params": {
    "group_id": "123456",
    "url": "https://your-server.com/archive.zip",
    "file_name": "archive.zip",
    "file_type": 4,
    "user_id": "654321"
  }
}
```

---

## 私聊文件上传 `/upload_private_file`

### 请求参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `user_id` | string/int | 是 | 目标用户 ID |
| `url` | string | 是 | 文件下载地址 |
| `file_name` | string | 否 | 文件名，默认 `"file"` |
| `file_type` | int | 否 | 1=图片 2=视频 3=语音 4=文件，默认 `4` |
| `msg_id` | string | 否 | 被动消息 ID |
| `event_id` | string | 否 | 事件 ID |

### 调用示例

```json
POST http://127.0.0.1:5700/upload_private_file
Content-Type: application/json

{
  "user_id": "654321",
  "url": "https://your-server.com/archive.zip",
  "file_name": "archive.zip",
  "file_type": 4,
  "msg_id": "可选的被动消息ID"
}
```

---

## 响应格式

### 成功 — 仅上传（无 msg_id）

文件已上传到 QQ 服务器，返回 `file_info` 供后续发消息使用：

```json
{
  "data": {
    "file_uuid": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
    "file_info": "不透明字符串，发消息时放在 media.file_info 中",
    "ttl": 7200
  },
  "message": "",
  "error_code": 0,
  "group_id": 123456,
  "traceID": "xxx"
}
```

### 成功 — 上传并发送（有 msg_id）

文件上传完成后自动作为被动消息发送到群/私聊：

```json
{
  "data": {
    "message_id": 789,
    "real_message_id": "QQ返回的消息ID",
    "file_uuid": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
    "file_info": "不透明字符串",
    "ttl": 7200
  },
  "message": "",
  "error_code": 0,
  "group_id": 123456,
  "traceID": "xxx"
}
```

### 失败

```json
{
  "data": {},
  "message": "upload_prepare failed: ...",
  "error_code": 50001,
  "traceID": ""
}
```

### 错误码

| error_code | 说明 |
|------------|------|
| 40001 | 缺少 `group_id` 或 `user_id` |
| 40002 | 缺少 `url` |
| 40010 | 文件下载失败（URL 不可达或哈希计算异常） |
| 40013 | 打开临时文件失败 |
| 40093 | QQ 平台每日累计上传文件已达上限（2GB） |
| 50001 | `upload_prepare` 调用失败 |
| 50002 | 分片上传失败（PUT 或 `upload_part_finish`） |
| 50004 | `post_file`（合并）调用失败 |

---

## lazy_message_id 适配

当配置文件中 `lazy_message_id: true` 时：

1. 框架自动从最近 4 分钟内收到的群消息中获取一个可用的 `msg_id`
2. 如果传了 `user_id`，会优先匹配该用户在该群的最近消息（更精准）
3. 获取到 `msg_id` 后，上传完成会**自动发送**文件消息（被动消息）
4. 如果没有可用的 `msg_id`（群内 4 分钟内无人说话），则只完成上传，返回 `file_info`

也可以通过 HTTP API / WS 直接传入 `msg_id` 参数，此时显式传入的 `msg_id` 优先级最高。

---

## 内部流程

```
调用方                       Gensokyo                        QQ Open Platform
  │                            │                                  │
  │─── url + group_id ────────►│                                  │
  │                            │── GET url ──► 文件服务器            │
  │                            │◄── 文件 + 计算 md5/sha1/md5_10m ──│
  │                            │                                  │
  │                            │── upload_prepare ───────────────►│
  │                            │◄── upload_id + parts + block_size│
  │                            │                                  │
  │                            │    ┌── 批次并发上传(concurrency) ──┐│
  │                            │    ├── PUT chunk[0] ───────────►││
  │                            │    │── part_finish(0) ─────────►││
  │                            │    ├── PUT chunk[1] ───────────►││
  │                            │    │── part_finish(1) ─────────►││
  │                            │    └── ...                      ││
  │                            │    └───────────────────────────┘│
  │                            │                                  │
  │                            │── POST /files(upload_id) ──────►│
  │                            │◄── file_info ──────────────────│
  │                            │                                  │
  │                            │── PostGroupMessage(file_info) ─►│  (如果有 msg_id)
  │                            │◄── message_id ─────────────────│
  │                            │                                  │
  │◄── JSON 响应 ──────────────│                                  │
```

### 秒传

如果文件已在 QQ 服务端（相同 MD5+SHA1），`upload_prepare` 会返回空的 `parts` 列表。此时框架跳过分片上传步骤，直接调用合并完成接口，实现零传输秒传。

### 并发上传

框架根据 `upload_prepare` 返回的 `concurrency` 字段控制并发度（默认 1，上限 10）。分片以批次模式并发上传：每批同时上传 N 个分片，全部完成后启动下一批。

### 重试策略（与 OpenClaw 官方实现对齐）

| 步骤 | 策略 |
|------|------|
| PUT 分片到 COS | 最多 2 次重试，指数退避（1s, 2s） |
| `upload_part_finish` | 普通错误最多 2 次指数退避重试；错误码 `40093001` 进入持续重试模式（1s 间隔，超时由 `retry_timeout` 控制，上限 10 分钟） |
| `POST /files`（合并） | 最多 2 次重试，递增延迟（2s, 4s） |

### file_info 缓存（与 OpenClaw upload-cache 对齐）

框架内置了基于内存的 `file_info` 缓存：

- **缓存 key** = 文件 MD5 + scope（group/c2c）+ 目标 ID + file_type
- 上传成功后自动缓存 `file_info`，TTL 比 API 返回值提前 60 秒失效
- 下次相同文件上传到相同目标时，跳过 `upload_prepare` 及所有分片步骤，直接复用缓存的 `file_info`
- 最大缓存 500 条，超限时惰性清理过期条目

注意：缓存命中仍需先下载文件计算 MD5（用于匹配 key）。如果 QQ 服务端的 `file_info` 已过期，用缓存值发送消息会失败，此时需重新上传。

---

## 注意事项

- QQ 平台当前对分片上传的文件大小有限制（参考 openclaw-qqbot 文档），框架侧**不做大小检测**，由 QQ 后端返回错误
- QQ 平台每日累计上传文件有 2GB 限制，触发时返回 `error_code: 40093`
- `file_info` 有 TTL（通常数小时），过期后需要重新上传
- 大文件上传耗时较长，HTTP 调用方应设置足够的超时时间（建议 10 分钟以上）
- `lazy_message_id` 获取的 `msg_id` 有 5 分钟有效期，大文件上传可能超时。建议使用"预上传"模式：先不带 `msg_id` 上传拿到 `file_info`，等用户下次触发时再通过 `send_group_msg` 发送

---

## 变更文件清单

| 操作 | 文件 |
|------|------|
| 新建 | `botgo/dto/file_upload.go` |
| 新建 | `botgo/openapi/v2/file_upload.go` |
| 新建 | `botgo/openapi/v1/file_upload.go` |
| 新建 | `handlers/upload_group_file.go` |
| 新建 | `handlers/upload_private_file.go` |
| 新建 | `handlers/upload_cache.go` |
| 修改 | `botgo/openapi/v2/resource.go` |
| 修改 | `botgo/openapi/v1/resource.go` |
| 修改 | `botgo/openapi/iface.go` |
| 修改 | `callapi/callapi.go` |
| 修改 | `handlers/message_parser.go` |
| 修改 | `httpapi/httpapi.go` |
