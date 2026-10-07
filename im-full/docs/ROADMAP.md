# 路线图：上生产还差什么

本项目是**功能完整的单机版**。下面诚实列出它没有的东西，以及每一项该怎么补。

## 1. 已知限制

| 领域 | 现状 | 影响 |
|---|---|---|
| 规模 | 单进程、单数据目录；消息全量驻留内存 | 适合数万用户 / 数十万～百万条消息；超出后内存和启动回放时间会成为瓶颈 |
| 存储 | 追加写日志，无压缩/快照/归档 | 文件只增不减；启动时间随日志长度线性增长 |
| 高可用 | 无主从、无多机房 | 机器挂了服务就停（数据可从备份恢复） |
| 传输安全 | 自身不做 TLS | 必须依赖反向代理；原生 TCP 端口目前是明文 |
| 编码 | 消息体是 JSON | 比 Protobuf 体积大；协议层已把编解码收敛在 `protocol.Marshal/Unmarshal` 两处，易替换 |
| 好友 | 添加即互为好友，无申请/同意/拉黑/删除 | |
| 群 | 群主拉人、成员退群；无踢人/转让/解散/群公告/禁言；上限 500 人（写扩散） | |
| 消息 | 仅文本；无撤回/编辑/删除/引用/@；无图片、语音、文件 | `msg_type` 字段已预留 |
| 状态 | 无在线状态、无"正在输入" | |
| 离线推送 | 无 APNs / FCM / 厂商通道 | App 被杀后收不到通知 |
| 账号 | 无找回密码、无手机/邮箱验证、无修改资料、令牌不能列出/单独踢设备 | |
| 治理 | 无敏感词/内容审核、无举报、无管理后台、无审计日志 | |
| 新成员历史 | 群新成员能看到加入前的历史 | 有些产品要求仅可见入群后 |
| 限流 | 登录限流按直连 IP；不识别 `X-Forwarded-For` | 代理后所有用户共用额度 |

## 2. 演进路径（按收益排序）

### A. 换存储（最先做）
`service` 只依赖 `store.Store` 的方法集：`CreateUser/Verify/IssueToken/ResolveToken`、`AddFriend/CreateGroup/…`、
`AppendMessage/History/Sync/MarkRead/Conversations`。把它抽成接口，实现 MySQL（或 TiDB / PostgreSQL）版本：

| 数据 | 表设计要点 |
|---|---|
| 消息 | `message(conv_id, seq, msg_id, from_uid, type, content, client_msg_id, time)`，主键 `(conv_id, seq)`，按 `conv_id` 分片；`(from_uid, client_msg_id)` 唯一索引实现幂等 |
| 收件箱 | `inbox(uid, user_seq, conv_id, seq)`，主键 `(uid, user_seq)`；`sync` 即范围查询 |
| 序号 | 会话 `seq` / 用户 `user_seq` 用 Redis `INCR` 或号段服务分配，**必须单调且无重复** |
| 已读 | `read_pos(uid, conv_id, seq)` |
| 其余 | 用户、好友、群、令牌常规建表 |

热数据留内存缓存，冷数据归档；这一步之后启动时间不再依赖日志长度。

### B. 拆网关、多机部署
- `gateway.RouteStore` 接口已存在：换成 Redis 实现（`HSET route:{uid} {device} {gatewayID|connID}` + `EXPIRE`，`Unbind` 用 Lua 做 compare-and-delete）。
- `service.Pusher` 接口：换成"查路由 → gRPC 调目标网关的 `PushToUser`"的实现。
- `gateway.Backend` 接口：换成调用 `service` 的 gRPC 客户端。网关即变成无业务逻辑的接入层，可水平扩展，发版时用现成的优雅停机平滑摘流。
- 引入消息队列（Kafka / RocketMQ）：落库后异步做推送、审核、统计、搜索索引。

### C. 大群
写扩散在万人群下写放大严重。改为**读扩散**：群消息只存一份，成员各自记录读到的 `group_seq`；在线用户用广播推送，并按"每个网关只收一次"分发；对点赞/入群提示等低优先级消息做合并与丢弃。

### D. 功能补全
消息撤回（一条引用原 `msg_id` 的控制消息，走正常链路）· 富媒体（客户端直传对象存储，消息体只放 URL + 元数据）· 在线状态/输入中（订阅制、防抖，不要广播给所有好友）· 离线推送（APNs/FCM，只当"通知"，不当可靠通道）· 好友申请流程 · 群管理角色 · 修改资料/找回密码。

### E. 工程化
压测与容量基线 · Prometheus 指标（发送成功率、到达率、端到端 P99、重连率、ACK 超时率）· 链路追踪（按 `msg_id`）· 灰度发布 · 内容审核 · 端到端加密（Signal 协议：X3DH + Double Ratchet；代价是服务端无法审核、多端与历史漫游变复杂）。

## 3. 设计上已经为扩展留好的口子

| 口子 | 位置 |
|---|---|
| 逻辑层可远程化 | `gateway.Backend` 接口 |
| 路由可外置到 Redis | `gateway.RouteStore` 接口 |
| 推送可跨机 | `service.Pusher` 接口 |
| 编码可换 Protobuf | `protocol.Marshal / Unmarshal` |
| 消息类型可扩展 | `msg_type` 字段 + `Push` / `SendReq` 结构 |
| 序号模型已就位 | 会话 `seq` + 用户 `user_seq`，换存储时语义不变 |
