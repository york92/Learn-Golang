# 🏥 Hospital Scheduler — 医院内部排班系统

> Go 1.21 + SQLite · REST API · 规则引擎 · 自动排班 · 应急调度

---

## 快速启动

```bash
# 依赖：需要 gcc（用于 CGO/SQLite）
# macOS:  xcode-select --install
# Ubuntu: apt install gcc

git clone <repo> && cd hospital-scheduler

go mod tidy                       # 下载依赖
make run                          # 编译 + 启动（默认 :8080）
# 或直接运行
CGO_ENABLED=1 go run ./cmd/server
```

启动后自动：
- 创建 SQLite 数据库（`./data/hospital.db`）
- 执行 Schema 迁移
- 注入演示数据（急诊科 + 6名医护 + 3种班次）

```
✓ Listening  http://localhost:8080
  API base   http://localhost:8080/api/v1
  Health     http://localhost:8080/health
```

---

## 环境变量

| 变量 | 默认 | 说明 |
|------|------|------|
| `PORT` | `8080` | HTTP 端口 |
| `DB_PATH` | `./data/hospital.db` | SQLite 路径 |
| `RULE_MAX_CONSECUTIVE_HOURS` | `24` | 连续工作上限（硬规则） |
| `RULE_MIN_REST_HOURS` | `8` | 班次间最低休息（硬规则） |
| `RULE_MAX_CONSECUTIVE_SHIFTS` | `5` | 连续排班天数建议上限（软规则） |
| `RULE_MAX_WEEKLY_HOURS` | `48` | 每周工时建议上限（软规则） |
| `RULE_MAX_NIGHT_SHIFTS_MONTH` | `8` | 每月夜班建议上限（软规则） |

---

## API 文档

### 健康检查
```
GET /health
```

### 科室
```
GET  /api/v1/departments          列出所有科室
POST /api/v1/departments          创建科室
```

### 班次类型
```
GET /api/v1/shift-types           早班(08-16) / 晚班(16-00) / 夜班(00-08)
```

### 员工
```
GET  /api/v1/staff                列出员工（?department_id=1 可过滤）
POST /api/v1/staff                创建员工
GET  /api/v1/staff/:id            获取单个员工
```

### 排班格 (Slot)
```
GET  /api/v1/slots                查询（?department_id=1&from=2024-01-01&to=2024-01-07）
POST /api/v1/slots                创建排班格
```

### 排班记录 (Assignment)
```
POST   /api/v1/assignments        手动排班（规则引擎校验）
DELETE /api/v1/assignments/:id    取消排班
```

### 自动排班
```
POST /api/v1/schedule/auto        贪心算法批量排班
```

### 工时报表
```
GET /api/v1/workload?department_id=1
```

### 应急调度
```
GET  /api/v1/emergency/candidates/:slotId   查找可替班人员
POST /api/v1/emergency/assign               应急分配
```

### 换班申请
```
POST /api/v1/swaps                提交换班申请
GET  /api/v1/swaps/pending        待审列表
POST /api/v1/swaps/:id/review     审批（action: approve / reject）
```

---

## 完整请求示例

```bash
BASE=http://localhost:8080/api/v1

# 创建员工
curl -X POST $BASE/staff \
  -H "Content-Type: application/json" \
  -d '{"employee_no":"D010","name":"孙医生","role":"DOCTOR",
       "department_id":1,"qualifications":["EMERGENCY","SURGERY"]}'

# 创建排班格
curl -X POST $BASE/slots \
  -H "Content-Type: application/json" \
  -d '{"department_id":1,"shift_type_id":1,"date":"2024-12-20",
       "required_staff":2,"required_role":"NURSE","required_quals":["ICU"]}'

# 手动排班（规则不通过 → 422 + 违规详情）
curl -X POST $BASE/assignments \
  -H "Content-Type: application/json" \
  -d '{"staff_id":3,"slot_id":1,"created_by":1}'

# 自动排班（整周）
curl -X POST $BASE/schedule/auto \
  -H "Content-Type: application/json" \
  -d '{"department_id":1,"from":"2024-12-20","to":"2024-12-27"}'

# 应急候选人
curl $BASE/emergency/candidates/1

# 换班申请 → 审批
curl -X POST $BASE/swaps \
  -H "Content-Type: application/json" \
  -d '{"requester_id":3,"slot_id":1,"target_staff_id":4,"reason":"家庭急事"}'

curl -X POST $BASE/swaps/1/review \
  -H "Content-Type: application/json" \
  -d '{"action":"approve","reviewer_id":1,"note":"同意换班"}'
```

---

## 规则引擎

### 硬规则（违反 → 422，排班被拒）

| Code | 规则 |
|------|------|
| H001 | 禁止双重排班（同时段重复分配） |
| H002 | 角色 + 资质必须完全匹配岗位要求 |
| H003 | 连续工作时长不超过 24h |
| H004 | 两班次间必须有 ≥8h 休息 |

### 软规则（违反 → 201 + warnings 字段警告）

| Code | 规则 |
|------|------|
| S001 | 连续排班建议不超过 5 天 |
| S002 | 每周工时建议不超过 48h |
| S003 | 每月夜班建议不超过 8 次 |
| S004 | 科室内工时公平性检查 |

---

## 项目结构

```
hospital-scheduler/
├── cmd/server/main.go              # 入口：启动 + seed
├── internal/
│   ├── domain/models.go            # 领域模型（JSON tags）
│   ├── config/config.go            # 配置（env vars）
│   ├── rules/engine.go             # 规则引擎（硬/软规则，可插拔）
│   ├── scheduler/auto.go           # 自动排班算法（贪心 + 工时均衡）
│   ├── service/schedule_service.go # 业务逻辑编排
│   ├── repository/
│   │   ├── db.go                   # SQLite 连接 + 迁移 + 事务
│   │   ├── driver.go               # 自定义驱动（per-conn PRAGMA FK=ON）
│   │   ├── migrate.go              # embed schema.sql
│   │   ├── schema.sql              # DDL + 索引
│   │   ├── staff_repo.go
│   │   ├── slot_repo.go
│   │   ├── assignment_repo.go
│   │   ├── workload_repo.go
│   │   ├── dept_repo.go
│   │   ├── swap_repo.go
│   │   └── audit_repo.go
│   └── api/
│       ├── router.go               # chi 路由 + CORS
│       ├── handlers.go             # HTTP handlers
│       └── response.go             # JSON 响应工具
├── db/migrations/001_init.sql      # 原始 SQL（参考用）
├── Makefile
└── go.mod
```

---

## 技术选型说明

| 组件 | 选择 | 理由 |
|------|------|------|
| 数据库 | SQLite + WAL | 零部署，单机内部系统，WAL 提升并发读 |
| HTTP | chi | 轻量，无反射，路由表达力强 |
| 规则引擎 | 接口+插件 | 每条规则独立文件，增删不影响其他规则 |
| 排班算法 | 贪心+工时排序 | 实用够用，比最优解更快，人工审核兜底 |
| 事务 | WithTx 封装 | 排班写入+计数更新原子完成，审计日志同步 |
