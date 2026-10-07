# 部署与运维

## 1. 端口与资源

| 端口 | 用途 | 对外？ |
|---|---|---|
| 8080 | Web 页面、HTTP API、WebSocket(`/ws`) | 通过 HTTPS 反向代理对外 |
| 9000 | 原生 TCP 长连接（App / CLI） | 仅当有原生客户端时开放 |

资源：每个二进制约 5MB（含内嵌的 Web 页面）；内存 = 全部用户/消息（驻留内存）+ 每个连接约 10~15KB；磁盘 = `wal.jsonl`（只增不减）。
长连接服务需要足够的文件描述符：`ulimit -n 65535` 以上（systemd 单元已设 `LimitNOFILE=1048576`）。

## 2. 三种部署方式

### systemd（Linux，推荐）
```bash
sudo ./scripts/install.sh
sudo vim /etc/im/config.json && sudo systemctl restart imserver
journalctl -u imserver -f
```
- 二进制 `/usr/local/bin/imserver`，数据 `/var/lib/im`（属主 `im`，权限 750），配置 `/etc/im/config.json`。
- 升级：替换二进制后 `systemctl restart imserver`。重启会向在线客户端发 `Kick` 并附带随机重连延迟，客户端自动错峰重连并补齐消息。

### Docker
```bash
docker compose -f deploy/docker-compose.yml up -d --build
```
配置用环境变量（见 USAGE 的配置表）。数据在命名卷 `im-data`。镜像以非 root（uid 10001）运行并带健康检查。

### 直接运行
```bash
./imserver -config config.json
```

## 3. HTTPS 与反向代理（对外开放必做）

服务本身不做 TLS，请在前面放 Nginx / Caddy。示例见 `deploy/nginx.conf.example`。三个必须注意：

1. **转发 WebSocket**：`Upgrade` / `Connection: upgrade` 头。
2. **透传 `Host`**：网关默认校验 WebSocket 的 `Origin` 必须与 `Host` 同源；Host 被改写会得到 403。
   （实在无法透传时设 `ws_allow_any_origin=true`，代价是失去跨站防护。）
3. **读超时 > 心跳间隔 × 2.5**（默认心跳 30s → 至少 75s，示例用 120s），否则代理会把空闲的长连接掐掉。

Caddy 等价配置（自动 HTTPS，自动处理 WebSocket）：
```
im.example.com {
    reverse_proxy 127.0.0.1:8080
}
```

原生 TCP（9000）要加密的话，用 Nginx `stream` + `ssl`，或在客户端侧走隧道/VPN；当前协议本身不含加密。

## 4. 安全检查清单

- [ ] 前面有 HTTPS（令牌与密码在明文 HTTP 上会被窃听）
- [ ] 不需要公开注册 → `allow_register=false`
- [ ] `/api/stats` 无需认证，在反向代理处限制来源（或仅内网可访问）
- [ ] 数据目录权限仅服务账号可读（含密码哈希与登录令牌；程序已将 WAL 设为 0600）
- [ ] 经过反向代理时，登录限流按**代理的 IP** 计数，所有用户共用额度；按需调大 `auth_per_minute`
- [ ] 9000 端口只在需要时对外开放；防火墙限制来源
- [ ] 定期备份 `wal.jsonl`

## 5. 备份与恢复

- **备份**：复制 `data/wal.jsonl`（追加写文件，直接复制即可；复制到的最后一行若不完整，恢复时自动丢弃）。
- **恢复**：停服务 → 把备份放回数据目录 → 启动。
- **迁移机器**：停服务 → 拷贝数据目录 → 新机器启动。
- **注意**：同一个数据目录**不能**被两个进程同时使用。

## 6. 监控与排障

- 存活：`GET /healthz`
- 指标：`GET /api/stats`
  - `online` 在线用户设备数；`open_conns` 含未认证连接
  - `slow_kick` 持续增长 → 有客户端消费太慢被踢；`push_retry` / `push_giveup` 高 → 网络差或客户端没回 ACK
  - `rate_limited` → 有客户端发送过快
- 日志：`log_level=debug` 可看到每个连接的关闭原因与 HTTP 请求。`log_format=json` 方便采集。
- 现象速查：

| 现象 | 常见原因 |
|---|---|
| Web 左上角红点 | 代理没开 WS 转发 / Host 被改写(403) / 代理读超时过短 |
| `accept error: too many open files` | fd 上限太低，调高 `ulimit -n` |
| 启动报 `wal corrupted at offset …` | WAL 中间行损坏（只有**尾行**不完整才会被自动容忍）。从备份恢复 |
| 注册/登录 429 | 触发 `auth_per_minute` |

## 7. 容量参考（仅量级，请以你自己的压测为准）

早期网关版本在本机一次压测：1.5 万条空闲长连接约 **14.5KB/连接**。消息存储每条约 200~300 字节内存（含索引）。
据此：单机 10 万连接约需 1.5GB 内存给连接，另加数据本身。这些数字未在生产负载下验证。
