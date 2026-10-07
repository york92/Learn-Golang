# HTTP 接口文档

基础地址 `http://localhost:8080`。除注册/登录外都需要请求头 `Authorization: Bearer <token>`。
请求与响应均为 JSON。响应统一格式：

```json
{"code":0,"msg":"","data":{...}}
```

`code=0` 成功；否则 `msg` 是错误说明。HTTP 状态码同时反映类别（401 未认证、403 无权限、404 不存在、409 冲突、429 限流）。

**错误码：** 1001 认证失败 · 1002 参数错误 · 1003 限流 · 1500 内部错误 · 2001 不存在 · 2002 非好友 · 2003 无权限 · 2004 内容过长 · 3001 用户名已存在

## 快速体验（curl）

```bash
H=http://localhost:8080
# 1. 注册 + 登录
curl -s $H/api/register -d '{"username":"dave","password":"secret1","nickname":"戴夫"}'
TOKEN=$(curl -s $H/api/login -d '{"username":"alice","password":"123456"}' | sed 's/.*"token":"\([^"]*\)".*/\1/')
AUTH="Authorization: Bearer $TOKEN"

# 2. 发消息（HTTP 通道，适合脚本/机器人/Webhook）
curl -s -H "$AUTH" $H/api/messages -d '{"to_username":"bob","content":"来自 curl","client_msg_id":"k-1"}'
# 用同样的 client_msg_id 再发一次：返回完全相同的 msg_id，对方只收到一次

# 3. 查看会话与历史
curl -s -H "$AUTH" $H/api/conversations
curl -s -H "$AUTH" "$H/api/messages?conv=s:1000:1001&limit=20"
```

> HTTP 发的消息同样会实时推送给在线的 WebSocket / TCP 客户端。

## 账号

| 方法 路径 | 说明 | 请求体 / 参数 | `data` |
|---|---|---|---|
| `POST /api/register` | 注册 | `{username,password,nickname?}` | `{uid,username,nickname,created}` |
| `POST /api/login` | 登录 | `{username,password}` | `{user:{…}, token, expires_at}` |
| `POST /api/logout` | 吊销当前 token | — | — |
| `GET /api/me` | 当前用户 | — | `{user, latest_user_seq}` |
| `GET /api/users/search?username=x` | 按用户名查 | — | `{uid,username,nickname}` |
| `GET /api/users?ids=1,2,3` | 批量查（≤100） | — | `[user…]` |

登录/注册限流：每个来源 IP 每分钟允许的「注册 + 登录**失败**」次数由 `auth_per_minute` 控制（成功登录不计数）。

## 好友

| 方法 路径 | 说明 | 请求体 |
|---|---|---|
| `GET /api/friends` | 好友列表 | — |
| `POST /api/friends` | 添加好友（演示版直接互为好友，幂等） | `{username}` 或 `{uid}` |

## 群

| 方法 路径 | 说明 | 请求体 |
|---|---|---|
| `GET /api/groups` | 我的群（含成员信息 `member_info`） | — |
| `POST /api/groups` | 建群（自己是群主，上限 500 人） | `{name, member_usernames?:[…], members?:[uid…]}` |
| `GET /api/groups/{id}` | 群详情（仅成员） | — |
| `POST /api/groups/{id}/members` | 拉人（仅群主） | `{username}` 或 `{uid}` |
| `POST /api/groups/{id}/leave` | 退群（群主不能退） | — |

## 会话与消息

| 方法 路径 | 说明 |
|---|---|
| `GET /api/conversations` | `{latest_user_seq, conversations:[{conv_id,type,title,peer_uid,group_id,last,read_seq,unread,peer_read_seq}]}`，按最后消息时间倒序。**`latest_user_seq` 与列表在同一时刻取得**，用作之后 sync 的起点 |
| `GET /api/messages?conv=&before_seq=&limit=` | 历史（`limit` 默认 50，最大 200）。`before_seq` 省略则取最新一页；返回按 `seq` 升序。仅会话成员可读 |
| `POST /api/messages` | 发消息：`{to_uid \| to_username \| group_id, content, client_msg_id?}` → `SendAck` |
| `GET /api/sync?since=&limit=` | 与 WebSocket 的 sync 等价（HTTP 轮询场景）→ `{msgs,has_more,latest}` |
| `POST /api/read` | 上报已读 `{conv_id, seq}`（会触发已读通知） |

## 运维

| 方法 路径 | 说明 |
|---|---|
| `GET /healthz` | 存活检查，返回 `ok` |
| `GET /api/stats` | `{gateway:{online,open_conns,accepted,…}, store:{users,groups,messages}}`（**无需认证**，对外暴露时请在反向代理处限制访问） |
| `GET /ws` | WebSocket 升级入口，见 [PROTOCOL.md](PROTOCOL.md) |
