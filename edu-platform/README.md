# 🎓 Edu Platform — Go + Gin + SQLite 在线教育后端

基于 **Go 1.21 + Gin + GORM + SQLite** 的在线教育平台 RESTful API 后端，采用 Handler → Service → Repository 三层架构。

---

## 📁 目录结构

```
edu-platform/
├── main.go                    # 程序入口
├── go.mod / go.sum
├── config/
│   └── config.go              # 配置（支持环境变量覆盖）
├── database/
│   └── database.go            # 数据库连接 & AutoMigrate
├── model/
│   ├── user.go                # 用户模型（学生/教师/管理员）
│   ├── course.go              # 课程模型
│   ├── chapter.go             # 章节模型
│   └── progress.go            # 学习进度模型
├── handler/                   # HTTP 层（参数绑定 & 响应）
├── service/                   # 业务逻辑层
├── repository/                # 数据访问层（封装 GORM）
├── middleware/
│   ├── auth.go                # JWT 鉴权 & 角色权限
│   ├── cors.go                # 跨域
│   └── logger.go              # 请求日志
├── router/
│   └── router.go              # 路由注册（依赖注入入口）
└── utils/
    ├── response.go            # 统一响应结构
    └── jwt.go                 # JWT 工具
```

---

## 🚀 快速启动

### 前置条件

- Go 1.21+
- gcc（SQLite CGO 依赖，macOS/Linux 自带；Windows 安装 [MinGW](https://www.mingw-w64.org/)）

### 步骤

```bash
# 1. 进入项目目录
cd edu-platform

# 2. 下载依赖
go mod tidy

# 3. 启动服务（默认端口 8080）
go run main.go
```

服务启动后数据库文件 `edu_platform.db` 自动创建并完成迁移。

### 环境变量（可选）

| 变量名        | 默认值                              | 说明             |
|--------------|-------------------------------------|-----------------|
| `SERVER_PORT` | `8080`                              | 服务端口         |
| `GIN_MODE`    | `debug`                             | `debug/release` |
| `DB_DSN`      | `./edu_platform.db`                 | SQLite 文件路径  |
| `JWT_SECRET`  | `edu-platform-secret-key-change-...`| JWT 签名密钥     |

```bash
# 示例：生产模式启动
GIN_MODE=release JWT_SECRET=my-strong-secret go run main.go
```

---

## 📡 API 接口文档

### 公开接口

| 方法   | 路径                        | 说明           |
|-------|-----------------------------|---------------|
| POST  | `/api/v1/auth/register`      | 用户注册       |
| POST  | `/api/v1/auth/login`         | 用户登录       |
| GET   | `/api/v1/courses`            | 获取课程列表   |
| GET   | `/api/v1/courses/:id`        | 获取课程详情   |
| GET   | `/health`                    | 健康检查       |

### 需要 JWT 鉴权

在 `Authorization` Header 中携带 `Bearer <token>`

| 方法   | 路径                                    | 角色           | 说明               |
|-------|-----------------------------------------|---------------|--------------------|
| GET   | `/api/v1/users/me`                      | 所有登录用户   | 获取个人信息        |
| POST  | `/api/v1/courses`                       | teacher/admin | 创建课程            |
| PATCH | `/api/v1/courses/:id/publish`           | teacher/admin | 发布课程            |
| GET   | `/api/v1/teachers/me/courses`           | teacher/admin | 我发布的课程        |
| POST  | `/api/v1/progress`                      | student/admin | 更新章节学习进度    |
| GET   | `/api/v1/progress/courses/:course_id`   | student/admin | 查询课程整体进度    |

### 示例：注册用户

```bash
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"username":"alice","email":"alice@example.com","password":"123456","role":"student"}'
```

### 示例：登录并获取 Token

```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"alice@example.com","password":"123456"}'
```

### 示例：更新学习进度

```bash
curl -X POST http://localhost:8080/api/v1/progress \
  -H "Authorization: Bearer <your_token>" \
  -H "Content-Type: application/json" \
  -d '{"chapter_id":1,"course_id":1,"watched_secs":300,"is_finished":false}'
```

---

## 🗄️ 数据库模型关系

```
User (1) ──────── (N) Course      [teacher_id]
Course (1) ─────── (N) Chapter    [course_id]
User (1) ────────┐
Course (1) ──────┼─ (N) Progress  [student_id, course_id, chapter_id]
Chapter (1) ─────┘
```

---

## 🔧 技术要点

- **SQLite WAL 模式**：`PRAGMA journal_mode=WAL` 提升并发读性能
- **外键约束**：`PRAGMA foreign_keys = ON` 保证数据完整性
- **软删除**：所有核心表使用 `gorm.DeletedAt`，数据可追溯
- **Upsert**：进度表使用 `ON CONFLICT` 防止重复记录
- **角色中间件**：`RequireRole("teacher")` 细粒度权限控制
- **统一响应**：所有接口返回 `{code, message, data}` 格式
