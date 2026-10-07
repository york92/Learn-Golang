# 使用指南

## 目录
1. [安装与启动](#安装与启动)
2. [Web 客户端使用](#web-客户端使用)
3. [命令行客户端 imcli](#命令行客户端-imcli)
4. [配置](#配置)
5. [测试清单](#测试清单)
6. [常见问题](#常见问题)

---

## 安装与启动

### 1. 预编译包（推荐，无需任何环境）

`dist/` 下有 5 个平台的二进制，每个目录里有 `imserver`、`imcli` 和 `config.json`：

| 目录 | 平台 |
|---|---|
| `dist/linux-amd64` | Linux x86_64 |
| `dist/linux-arm64` | Linux ARM64（树莓派 4/5、ARM 云主机） |
| `dist/darwin-arm64` | macOS Apple Silicon |
| `dist/darwin-amd64` | macOS Intel |
| `dist/windows-amd64` | Windows 64 位 |

```bash
cd dist/linux-amd64
chmod +x imserver imcli          # macOS / Linux 首次需要
./imserver -seed                 # -seed：创建演示账号 alice/bob/carol（密码 123456）
```

macOS 若提示"无法验证开发者"：`xattr -d com.apple.quarantine imserver imcli`。
校验完整性：`cd dist && sha256sum -c SHA256SUMS`。

### 2. 从源码

需要 Go 1.22+（`go version` 检查）。

```bash
make build        # 生成 ./bin/imserver 和 ./bin/imcli
make demo         # 编译并以演示模式启动
make dist         # 交叉编译 5 个平台到 ./dist
```

### 3. 安装为系统服务（Linux）

```bash
sudo ./scripts/install.sh          # 安装到 /usr/local/bin，配置 /etc/im/config.json，数据 /var/lib/im
sudo systemctl status imserver
journalctl -u imserver -f          # 看日志
sudo ./scripts/install.sh --uninstall
```

### 4. Docker

```bash
docker compose -f deploy/docker-compose.yml up -d --build
docker compose -f deploy/docker-compose.yml logs -f
```
数据保存在 volume `im-data`。想要演示账号：编辑 compose 文件取消 `command: ["-seed"]` 的注释。

### 启动参数

```
imserver [-config config.json] [-tcp :9000] [-http :8080] [-data ./data] [-seed] [-version]
```

端口说明：**8080** 提供 Web 页面 + HTTP API + WebSocket(`/ws`)；**9000** 是原生 TCP 长连接（给 App / CLI 用）。
被占用时用 `-http :8081 -tcp :9001` 改端口。

---

## Web 客户端使用

浏览器打开 `http://localhost:8080`。

- **注册 / 登录**：用户名 3-32 位（字母数字下划线），密码至少 6 位。
- **每个标签页是独立登录**（用 sessionStorage 存令牌），所以在**同一个浏览器里开两个标签页就能模拟两个用户互聊**。关闭标签页即退出登录。
- **会话 / 好友 / 群** 三个标签：
  - 「+ 新聊天」输入对方用户名直接开聊；
  - 「好友」里「+ 添加好友」输入用户名（演示版无需对方确认，直接互为好友）；
  - 「群」里「+ 新建群」输入群名与成员用户名（逗号分隔）。
- **群聊**：群主可点「邀请」拉人；成员可「退群」（群主不能退群）。
- **左上角圆点**：绿色=已连接，红色=断线重连中。
- 消息状态：发送中 → 已送达（✓）→ 已读（✓✓，仅单聊）；发送失败会显示红字，点击即可重试（使用相同的 `client_msg_id`，不会重复）。
- 会话列表红点 = 未读数；打开会话（且页面可见）自动标记已读。

---

## 命令行客户端 imcli

```bash
imcli register -u carol -p 123456 [-nick 小红]     # 注册
imcli chat     -u alice -p 123456                  # 交互式聊天
imcli send     -u alice -p 123456 -to bob -text "hi"      # 发一条就退出
imcli send     -u alice -p 123456 -group 1 -text "大家好"  # 发群消息
imcli pull     -u bob   -p 123456                  # 拉取全部收件箱后退出（验证离线消息）
# 通用参数：-http http://127.0.0.1:8080  -tcp 127.0.0.1:9000
```

`imcli chat` 内的命令：

```
/to <用户名> <内容>          发单聊
/g <群id> <内容>             发群聊
/friends  /groups            列出好友 / 群
/addfriend <用户名>          加好友
/newgroup <名称> <用户名,用户名>   建群
/quit                        退出
```

---

## 配置

优先级：**默认值 < 配置文件 < 环境变量 < 命令行参数**。模板见 `configs/config.example.json`。

| 配置项 | 环境变量 | 默认 | 说明 |
|---|---|---|---|
| `tcp_addr` | `IM_TCP_ADDR` | `:9000` | 原生 TCP 监听；留空则不开 |
| `http_addr` | `IM_HTTP_ADDR` | `:8080` | HTTP/WS/Web 监听；留空则不开 |
| `data_dir` | `IM_DATA_DIR` | `./data` | 数据目录（`wal.jsonl`） |
| `allow_register` | `IM_ALLOW_REGISTER` | `true` | 是否开放自助注册 |
| `require_friend` | `IM_REQUIRE_FRIEND` | `false` | `true` 时单聊必须先是好友 |
| `token_ttl_hours` | — | `720` | 登录令牌有效期（30 天） |
| `auth_per_minute` | `IM_AUTH_PER_MINUTE` | `60` | 每个 IP 每分钟允许的「注册 + 登录**失败**」次数 |
| `heartbeat_sec` | `IM_HEARTBEAT_SEC` | `30` | 下发给客户端的心跳间隔；服务端空闲 3 倍间隔判死，客户端 2.5 倍判死 |
| `max_conns` | `IM_MAX_CONNS` | `100000` | 最大连接数 |
| `max_content_bytes` | — | `4096` | 单条消息内容上限 |
| `fsync_ms` | `IM_FSYNC_MS` | `200` | 落盘间隔；`0`=每次写都 fsync（最安全最慢） |
| `ws_allow_any_origin` | `IM_WS_ALLOW_ANY_ORIGIN` | `false` | `false`=仅允许与页面同源的 WebSocket |
| `log_level` / `log_format` | `IM_LOG_LEVEL` / `IM_LOG_FORMAT` | `info` / `text` | `debug\|info\|warn\|error`；`text\|json` |

**对外开放前请务必：** 前面放 HTTPS 反向代理（见 [DEPLOY.md](DEPLOY.md)）；不需要公开注册就设 `allow_register=false`（先用 `-seed` 或临时开放注册建好账号）。

---

## 测试清单

下面每一项都可以手动验证，而且**都已被自动化测试覆盖**（括号里是对应测试）。
先 `make demo` 启动，浏览器开两个标签页分别登录 alice、bob。

| # | 场景 | 操作 | 期望 | 自动化测试 |
|---|---|---|---|---|
| 1 | 实时单聊 | alice 给 bob 发消息 | bob 实时收到，会话列表红点 +1 | `TestFullFlow/direct_message` |
| 2 | 已读回执 | bob 打开该会话 | 红点消失；alice 看到「✓✓ 已读」 | `TestFullFlow/conversations_unread…` |
| 3 | 多设备 | 再开第三个标签页登录 bob | 新标签页自动补全历史；之后 bob 两处同时收到新消息 | `TestFullFlow/multi_device` |
| 4 | **离线消息** | 关掉 bob 的标签页；alice 发 3 条；重新登录 bob | 3 条按序出现，未读 3 | `TestFullFlow/offline_sync` |
| 5 | **断线重连** | 停掉服务端再启动（或断网） | 圆点变红后自动变绿，期间的消息自动补齐 | UI 测试「重连后自动补齐」 |
| 6 | 群聊 | alice 建群拉入 bob、carol 并发言 | 全员收到并显示发言人；非成员发不出 | `TestFullFlow/group_chat` |
| 7 | 退群 | bob 退群后 alice 再发言 | bob 不再收到 | `TestFullFlow/group_chat` |
| 8 | 幂等 | 同一 `client_msg_id` 发两次（见 API.md） | 返回相同 `msg_id`，对方只收到一次 | `TestFullFlow/idempotent_http_send` |
| 9 | 历史分页 | 会话里点「加载更早消息」 | 按页回溯 | `TestFullFlow/history_pagination` |
| 10 | **重启不丢** | `Ctrl+C` 停服务再启动 | 账号、令牌、会话、消息都在，序号接续 | `TestPersistenceAcrossRestart` |
| 11 | 崩溃恢复 | `kill -9` 服务；或手动往 `wal.jsonl` 末尾追加半行再启动 | 正常启动，半行被丢弃 | `TestReplayAndTornTail` |
| 12 | 好友限制 | 设 `IM_REQUIRE_FRIEND=true` | 非好友发消息被拒，加好友后可发 | `TestRequireFriend` |
| 13 | 登录防爆破 | 连续输错密码 | 一段时间后 429，成功登录不受限 | `TestLoginRateLimitOnlyCountsFailures` |
| 14 | WebSocket 协议 | — | 握手/分片/ping/跨站 Origin 拒绝 | `TestWebSocketTransport` |
| 15 | XSS | 发送 `<b>x</b>` | 按纯文本显示 | UI 测试「无 XSS」 |

---

## 常见问题

**打开页面后左上角一直是红点？**
WebSocket 没连上。常见原因：① 经过的反向代理没有开 WebSocket 转发（见 `deploy/nginx.conf.example`）；② 代理没有透传 `Host`，触发同源校验被拒（返回 403）——要么透传 Host，要么设 `ws_allow_any_origin=true`；③ 代理的读超时小于心跳间隔。

**注册时报 "too many attempts"？**
同一 IP 注册/登录失败过于频繁被限流（`auth_per_minute`）。注意：经反向代理时所有用户共用代理 IP，必要时调大。

**忘记密码 / 想重置所有数据？**
演示版没有找回密码功能。停服务后删除数据目录（默认 `./data`）即可清空重来。**不要在服务运行时手动改 `wal.jsonl`。**

**能同时用多个进程/多台机器共用一个数据目录吗？**
不能。WAL 是单进程独占写，当前版本是单机架构，见 ROADMAP。

**Windows 上 `scripts/*.sh` 不能运行？**
用 `scripts\start.bat`，或在 PowerShell 里直接 `dist\windows-amd64\imserver.exe -seed`。

**端口被占用？**
`imserver -http :8081 -tcp :9001`，或改配置文件。

**怎么备份？**
热备份：复制 `data/wal.jsonl` 即可（追加写，复制到的最后一行如果不完整，启动时会被自动丢弃）。恢复：放回数据目录再启动。

**消息会不会丢？**
消息写入 WAL 后才会向发送方返回 ServerAck。进程崩溃不丢；操作系统崩溃/断电最多丢最近 `fsync_ms`（默认 200ms）内的写入，设 `fsync_ms=0` 可彻底避免（吞吐会明显下降）。
