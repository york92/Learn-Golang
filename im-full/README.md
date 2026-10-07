# IM · 即时通讯系统（可直接运行的完整基础版）

一个**零依赖、单文件部署**的即时通讯服务：账号体系、单聊、群聊、离线消息、多端同步、已读回执、
历史记录、断线重连、持久化存储，自带 Web 客户端和命令行客户端。
Go 1.22 标准库实现，**不需要数据库、不需要 Redis、不需要 Node**。

```
浏览器 ──WebSocket──┐
                    ├──► imserver ──► data/wal.jsonl（持久化）
App/CLI ───TCP──────┘     │
                          └─ HTTP API · Web 客户端（内嵌）
```

## 30 秒跑起来

**方式 A：用预编译包（无需 Go）**

```bash
# Linux / macOS：进入解压目录
./scripts/start.sh                 # 自动选择适合你系统的 dist/<os>-<arch>/imserver，并创建演示账号
# Windows：双击 scripts\start.bat
```

**方式 B：从源码（需要 Go 1.22+）**

```bash
make demo          # 编译并启动，创建演示账号 alice / bob / carol（密码都是 123456）
```

**方式 C：Docker**

```bash
docker compose -f deploy/docker-compose.yml up -d --build
```

启动后看到：

```
  Web 客户端 : http://localhost:8080
  WebSocket  : ws://localhost:8080/ws
  TCP 长连接 : localhost:9000
```

## 立刻验证它能用

1. 打开 <http://localhost:8080>，用 `alice / 123456` 登录。
2. **再开一个标签页**（每个标签页是独立登录），用 `bob / 123456` 登录。
3. alice 里点「好友 → bob」发一句话，bob 那边会实时收到、出现红点；bob 点开后 alice 这边显示「✓✓ 已读」。

更多测试场景（离线消息、群聊、断线重连、多设备）见 [docs/USAGE.md](docs/USAGE.md#测试清单)。

一条命令跑完自动化验证：

```bash
make race        # 全部 Go 测试（含竞态检测）
make smoke       # 命令行冒烟：注册 → 发消息 → 离线拉取 → 历史 → 幂等
make test-ui     # 用 jsdom 把 Web 客户端跑起来，两个标签页对聊（需要 Node.js）
```

## 功能清单

| 模块 | 已实现 |
|---|---|
| 账号 | 注册 / 登录 / 登出、PBKDF2-SHA256 加盐哈希、令牌（可过期、可吊销）、登录防暴力破解 |
| 接入 | 原生 TCP 二进制协议 + WebSocket（自实现 RFC 6455），同一套协议与逻辑 |
| 单聊 / 群聊 | 文本消息；群主拉人、成员退群；可选「必须先加好友才能聊」 |
| 可靠性 | ServerAck、推送 ACK + 超时重传、`client_msg_id` 幂等去重、会话内 `seq` 保序 |
| 离线 / 多端 | 每用户一条收件箱时间线（`user_seq`），上线按游标增量同步、自动检测并补齐空洞；同账号多设备同步 |
| 会话 | 会话列表、未读数、已读回执（单聊对方可见、本人多端同步）、分页历史 |
| 连接管理 | 心跳、空闲/假死检测、指数退避+抖动重连、慢消费者保护、限流、同设备重复登录踢旧、优雅停机 |
| 存储 | 追加写日志（WAL）+ 内存索引；崩溃恢复（自动截断半行）；后台组提交 fsync |
| 客户端 | 内嵌 Web 客户端、Go SDK、`imcli` 命令行（交互聊天 / 一次性发送 / 拉取） |
| 运维 | 配置文件 + 环境变量 + 命令行覆盖、`/healthz`、`/api/stats`、systemd / Docker / Nginx 示例 |

## 目录结构

```
im/
├── README.md                    本文件
├── Makefile                     make help 查看所有目标
├── go.mod                       零第三方依赖
├── cmd/
│   ├── imserver/main.go         服务端入口（TCP + WS + HTTP）
│   └── imcli/main.go            命令行客户端
├── internal/
│   ├── protocol/                帧格式（length|cmd|seq|body）、命令字、消息体
│   ├── gateway/                 长连接网关：连接管理、握手、心跳、限流、可靠推送、WebSocket、优雅停机
│   ├── service/                 业务逻辑：收发、同步、已读、好友、群、会话、历史
│   ├── store/                   持久化：WAL + 内存索引、密码哈希
│   ├── api/                     HTTP 接口层
│   ├── client/                  Go 客户端 SDK（游标同步、重连、重试）
│   ├── config/                  配置加载
│   └── e2e/                     全栈集成测试
├── web/                         Web 客户端（单文件，go:embed 进二进制）
├── configs/config.example.json  配置模板
├── scripts/                     start / install / smoke / test-ui / build-all
├── deploy/                      Dockerfile、docker-compose、systemd、nginx 示例
├── tests/ui/                    Web 客户端 UI 自动化测试（jsdom）
├── dist/                        预编译二进制（linux/darwin/windows，见 SHA256SUMS）
└── docs/
    ├── USAGE.md                 使用指南 + 测试清单 + 常见问题
    ├── ARCHITECTURE.md          架构与设计思路
    ├── PROTOCOL.md              通信协议规范（写自己的客户端看这个）
    ├── API.md                   HTTP 接口文档（含 curl 示例）
    ├── DEPLOY.md                部署与运维
    └── ROADMAP.md               上生产还差什么、怎么扩展
```

## 适用范围（请先读）

这是一个**功能完整、可以真实使用**的**单机**版本，适合：学习、内部小团队、原型验证、作为二次开发的起点。
消息全量驻留内存、单进程，规模参考：数万用户、数十万条消息量级。
想上到十万级以上并发或多机部署，请先读 [docs/ROADMAP.md](docs/ROADMAP.md)（里面写清了每一项该怎么换）。
