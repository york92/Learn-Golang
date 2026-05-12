# 公司内部库存管理系统

基于 Go + SQLite 的库存管理系统，支持防超卖、流水追踪、库存预警。
本版本使用纯 Go 实现的 SQLite 驱动，**无需安装 GCC，开箱即用**。

---

## 环境要求

| 工具 | 版本要求 | 下载地址 |
|------|---------|---------|
| Go   | 1.22 +  | https://go.dev/dl/ |

> 不需要 GCC，不需要 MinGW，不需要任何 C 编译器。

---

## 启动步骤

### 第一步：下载安装 Go

1. 打开 https://go.dev/dl/
2. 下载 `go1.22.x.windows-amd64.msi`
3. 双击安装，全部默认，安装完成后**重新打开一个新的 CMD 窗口**
4. 验证安装成功：
   ```bat
   go version
   ```
   看到类似 `go version go1.22.x windows/amd64` 即为成功

---

### 第二步：解压项目

将 `inventory.tar.gz` 解压到任意目录，例如 `D:\inventory`

```bat
cd D:\inventory
```

> 如果没有解压工具，可用 Windows 11 自带的右键 → "解压到此处"，
> 或在 PowerShell 中执行：
> ```powershell
> tar -xzf inventory.tar.gz
> ```

---

### 第三步：修改驱动（从 mattn 换为 modernc，无需 GCC）

**修改 `go.mod`**，将文件内容替换为：

```
module inventory

go 1.22.2

require modernc.org/sqlite v1.29.9
```

> 注意：删除原来的 `require github.com/mattn/go-sqlite3` 和 `replace` 那两行。

**修改 `db/db.go`**，找到以下两处并修改：

第 1 处，import 中替换驱动包名：
```go
// 改前
_ "github.com/mattn/go-sqlite3"

// 改后
_ "modernc.org/sqlite"
```

第 2 处，`sql.Open` 的驱动名称：
```go
// 改前
db, err := sql.Open("sqlite3", fmt.Sprintf("%s?_foreign_keys=on", dbPath))

// 改后
db, err := sql.Open("sqlite", fmt.Sprintf("%s?_foreign_keys=on", dbPath))
```

---

### 第四步：拉取依赖

```bat
go mod tidy
```

正常输出示例：
```
go: finding module for package modernc.org/sqlite
go: downloading modernc.org/sqlite v1.29.9
...
```

> 如果下载慢，设置国内代理再重试：
> ```bat
> go env -w GOPROXY=https://goproxy.cn,direct
> go mod tidy
> ```

---

### 第五步：编译

```bat
go build -o inventory_server.exe .
```

编译成功后目录下会出现 `inventory_server.exe`，无任何报错即为成功。

---

### 第六步：启动服务

```bat
inventory_server.exe
```

看到以下输出说明启动成功：

```
2024/01/01 10:00:00 [DB] SQLite initialized, WAL mode active
2024/01/01 10:00:00 🚀 库存管理系统启动，监听端口 :8080
2024/01/01 10:00:00    数据库路径：./inventory.db
```

浏览器访问验证：
```
http://localhost:8080/health
```
返回 `{"status":"ok"}` 即为正常。

按 `Ctrl + C` 可安全停止服务。

---

## 自定义配置

启动时通过环境变量修改默认配置：

```bat
:: 修改端口
set SERVER_PORT=9090
inventory_server.exe

:: 修改数据库路径
set DB_PATH=D:\data\inventory.db
inventory_server.exe

:: 同时修改多项
set SERVER_PORT=9090 && set DB_PATH=D:\data\inv.db && inventory_server.exe
```

| 环境变量 | 默认值 | 说明 |
|---------|--------|------|
| SERVER_PORT | 8080 | HTTP 监听端口 |
| DB_PATH | ./inventory.db | 数据库文件路径，首次启动自动创建 |

---

## API 快速参考

服务启动后，使用任意 HTTP 工具（Postman、Apifox、curl）调用以下接口。

### 健康检查
```
GET http://localhost:8080/health
```

### 商品管理

| 方法 | 地址 | 说明 |
|------|------|------|
| POST | /api/products | 创建商品 |
| GET | /api/products | 商品列表（支持 keyword/page/page_size） |
| GET | /api/products/{id} | 商品详情 |
| PUT | /api/products/{id} | 更新商品信息 |
| DELETE | /api/products/{id} | 软删除商品 |

**创建商品示例：**
```json
POST /api/products
{
  "name": "农夫山泉矿泉水",
  "spec": "550ml/瓶",
  "price": 2.5,
  "initial_stock": 100,
  "warning_threshold": 10
}
```

### 库存操作

| 方法 | 地址 | 说明 |
|------|------|------|
| POST | /api/stock/in | 入库 |
| POST | /api/stock/out | 出库 |
| POST | /api/stock/loss | 损耗登记 |

**出库示例：**
```json
POST /api/stock/out
{
  "product_id": 1,
  "quantity": 10,
  "remark": "销售订单 SO-001",
  "operator": "张三"
}
```

### 流水查询

```
GET /api/logs?product_id=1&change_type=OUT&page=1&page_size=20
```

| 参数 | 说明 |
|------|------|
| product_id | 商品ID，不传则查全部 |
| change_type | IN / OUT / LOSS，不传则查全部 |
| start_time | 开始时间，格式 `2024-01-01` |
| end_time | 结束时间 |

### 预警管理

```
GET  /api/warnings?only_unresolved=true
PUT  /api/warnings/{id}/resolve
```

---

## 常见问题

**Q：`go mod tidy` 提示 replacement directory does not exist**
删除 `go.mod` 中的 `replace github.com/mattn/go-sqlite3 => ...` 这一行，保存后重试。

**Q：下载依赖超时或失败**
```bat
go env -w GOPROXY=https://goproxy.cn,direct
go mod tidy
```

**Q：8080 端口被占用**
```bat
set SERVER_PORT=9090
inventory_server.exe
```

**Q：数据库文件在哪里**
默认在 exe 同级目录下的 `inventory.db`，SQLite 单文件，直接备份该文件即可。

**Q：如何开机自启**
将以下内容保存为 `start.bat`，放入 `shell:startup` 文件夹（Win+R 输入该命令打开）：
```bat
@echo off
cd /d D:\inventory
start inventory_server.exe
```
