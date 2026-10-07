# 通信协议规范

适用于 **TCP（:9000）** 和 **WebSocket（`/ws`）**。想写自己的客户端（Android / iOS / 小程序 / 其他语言）看这一篇即可。
参考实现：`internal/protocol`（Go）、`web/index.html`（JavaScript）、`internal/client`（Go SDK）。

## 1. 帧格式

所有数据都是**帧**，大端序：

```
 0       4      6       10
 +-------+------+-------+----------------+
 | length| cmd  | seq   | body (JSON)    |
 | 4B    | 2B   | 4B    | length-6 字节   |
 +-------+------+-------+----------------+
```

- `length` = `cmd`(2) + `seq`(4) + `body` 的字节数，即 `6 + len(body)`（**不含 length 自己的 4 字节**）。
- `body` 是 UTF-8 JSON（可为空）。生产环境可整体替换为 Protobuf，只需改编解码函数。
- 单帧 body 上限 64KB。超限、`length < 6` 都会导致服务端断开连接。
- **TCP 是字节流**：必须按 `length` 自己切包，处理粘包/半包。
- **WebSocket**：必须用 **binary** 消息。一个 WS 消息里放一个或多个完整帧都行（建议一消息一帧）；
  服务端每个帧单独发一个 WS 消息。客户端 → 服务端的 WS 帧必须带掩码（浏览器自动满足）。

### `seq` 的含义

- 客户端发起的**请求**：客户端自增 `seq`；服务端的**响应**原样带回同一个 `seq`，客户端据此匹配。
- 服务端发起的**推送**（`Push`）：`seq` 是服务端在该连接上自增的，客户端回 `PushAck` 时原样带回。
- 两个方向的 `seq` 互相独立。

## 2. 命令字

| cmd | 名称 | 方向 | 说明 |
|---:|---|---|---|
| 1 | AuthReq | C→S | 建连后**第一帧必须是它**（10 秒内） |
| 2 | AuthResp | S→C | 认证结果，含心跳间隔 |
| 3 | Heartbeat | C→S | 心跳，body 为空 |
| 4 | HeartbeatAck | S→C | 心跳应答 |
| 10 | SendReq | C→S | 发消息 |
| 11 | SendAck | S→C | 落库确认（ServerAck） |
| 20 | Push | S→C | 推送一条消息 |
| 21 | PushAck | C→S | 收到推送的确认，`seq` 为被确认 Push 的 `seq` |
| 30 | Kick | S→C | 服务端要求断开（被顶号 / 服务重启） |
| 40 | SyncReq | C→S | 增量同步 |
| 41 | SyncResp | S→C | 同步结果 |
| 50 | ReadReq | C→S | 上报已读 |
| 51 | ReadAck | S→C | 已读应答 |
| 52 | ReadNotify | S→C | 已读通知（对方读了 / 我的其他设备读了） |
| 99 | Err | S→C | 通用错误，`seq` 同触发它的请求 |

## 3. 消息体（JSON）

```jsonc
// AuthReq (1)
{"token":"<登录接口返回的 token>","device_id":"phone-1","platform":"android"}
// AuthResp (2)
{"code":0,"msg":"","heartbeat_sec":30,"server_time":1730000000000}

// SendReq (10)  to_uid 与 group_id 二选一
{"client_msg_id":"uuid-或随机串","to_uid":1001,"group_id":0,"msg_type":1,"content":"你好"}
// SendAck (11)
{"client_msg_id":"…","code":0,"msg":"","msg_id":42,"seq":7,"conv_id":"s:1000:1001","server_time":1730000000000}

// Push (20) —— 同时也是 SyncResp.msgs[] 与历史接口里的消息结构
{"msg_id":42,"user_seq":13,"seq":7,"conv_id":"s:1000:1001","from_uid":1000,"from_name":"alice",
 "msg_type":1,"content":"你好","send_time":1730000000000,"client_msg_id":"仅发送者的回显里有"}
// PushAck (21)
{"msg_id":42}

// SyncReq (40) / SyncResp (41)
{"since":12,"limit":100}
{"code":0,"msgs":[ /* Push, 按 user_seq 升序 */ ],"has_more":false,"latest":13}

// ReadReq (50) / ReadAck (51) / ReadNotify (52)
{"conv_id":"s:1000:1001","seq":7}
{"code":0,"conv_id":"s:1000:1001","seq":7}
{"conv_id":"s:1000:1001","uid":1001,"seq":7}      // uid = 谁读的

// Kick (30)
{"reason":"server_restart","retry_after_ms":3200}
// Err (99)
{"code":1003,"msg":"rate limited"}
```

**会话 ID 约定：** 单聊 `s:<较小uid>:<较大uid>`，群聊 `g:<群id>`。

## 4. 业务码

| code | 含义 |
|---:|---|
| 0 | 成功 |
| 1001 | 认证失败（token 无效/过期） |
| 1002 | 请求格式错误 / 内容为空 |
| 1003 | 触发限流（SendReq/SyncReq/ReadReq 默认每连接 20 次/秒，突发 40） |
| 1500 | 服务端内部错误（**可重试**，保持相同 `client_msg_id`） |
| 2001 | 用户 / 群不存在 |
| 2002 | 非好友（仅 `require_friend=true`） |
| 2003 | 不是群成员 / 无权限 |
| 2004 | 内容过长（默认 > 4096 字节） |

`SendAck.code` 为 1003 / 1500 时客户端应**重试**；其他非 0 码是业务拒绝，**不要重试**。

## 5. 典型时序

**连接与同步**
```
C: AuthReq{token,device_id}                S: AuthResp{code:0, heartbeat_sec:30}
C: SyncReq{since:<本地游标>}               S: SyncResp{msgs:[...], has_more, latest}   （has_more 为 true 就继续拉）
C: Heartbeat  ───每 heartbeat_sec 秒───►   S: HeartbeatAck
```

**发消息**
```
C: SendReq{client_msg_id:X,...}  ──►  S: SendAck{code:0,msg_id,seq}
                                      S: Push{...,client_msg_id:X}   ← 发送者所有设备都会收到回显
（3 秒内没收到 SendAck：用同一个 X 重发，最多 3 次）
```

**收消息**
```
S: Push{user_seq:N,...}   ──►  C: PushAck（seq 取自这条 Push 帧）
客户端游标处理：  N <= 游标 → 丢弃(重复)；N == 游标+1 → 游标=N；N > 游标+1 → 暂存并 SyncReq 补洞
（服务端 3 秒内没收到 PushAck 会重传，最多 3 次）
```

## 6. 客户端必须遵守的规则

1. 建连后立刻发 `AuthReq`，**不认证不能发其他命令**。
2. 每个 `heartbeat_sec` 发一次 `Heartbeat`；超过 **2.5 × heartbeat_sec** 没收到服务端任何数据，视为连接已死，主动断开重连。
3. 重连使用**指数退避 + 随机抖动**（建议 500ms 起、上限 30s）；收到 `Kick` 时优先使用其 `retry_after_ms`。
4. 重连成功后立即 `SyncReq`。
5. 收到每个 `Push` 都要回 `PushAck`（包括重复的）。
6. 消息去重依据：`user_seq <= 本地游标` 或已见过的 `msg_id`。
7. 展示排序依据：`seq`（会话内），**不要用本地时间或 `send_time`**。
8. 同一账号同一 `device_id` 重复登录，旧连接会收到 `Kick{reason:"login_elsewhere"}`；多设备请使用**不同的** `device_id`。
