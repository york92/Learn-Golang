# 博客系统 - 项目目录结构

```
blog-system/
│
├── 📄 README.md                      # 项目说明文档
├── 📄 QUICKSTART.md                  # 快速开始指南
├── 📄 API_DOCUMENTATION.md           # API接口文档（25个接口）
├── 📄 DATABASE_DESIGN.md             # 数据库设计文档（8张表）
├── 📄 TEST_CHECKLIST.md              # 功能测试清单（60+测试项）
├── 📄 PROJECT_SUMMARY.md             # 项目总结文档
│
├── 🚀 start.sh                       # Linux/Mac启动脚本
├── 🚀 start.bat                      # Windows启动脚本
│
├── 💻 backend/                       # Go后端代码
│   ├── main.go                      # 主程序（路由定义）
│   ├── database.go                  # 数据库初始化和表创建
│   ├── models.go                    # 数据模型定义
│   ├── auth.go                      # 认证中间件和工具函数
│   ├── handlers_user.go             # 用户相关API处理
│   ├── handlers_article.go          # 文章相关API处理
│   ├── handlers_comment.go          # 评论相关API处理
│   ├── handlers_other.go            # 其他API处理（搜索、通知、管理）
│   └── go.mod                       # Go模块依赖管理
│
├── 🎨 frontend/                      # 前端代码
│   ├── index.html                   # 单页应用HTML（300行）
│   ├── styles.css                   # 样式文件（600行）
│   └── app.js                       # JavaScript逻辑（800行）
│
└── 🗄️ database/                      # 数据库目录
    └── blog.db                      # SQLite数据库（运行时自动生成）
```

---

## 📊 文件详情

### 后端文件

#### `main.go` (约150行)
- HTTP服务器启动
- 路由配置
- 中间件注册
- 静态文件服务

#### `database.go` (约150行)
- 数据库连接初始化
- 8个数据表创建
- 默认管理员账户创建
- 外键约束配置

#### `models.go` (约120行)
- 用户模型 (User)
- 文章模型 (Article)
- 评论模型 (Comment)
- 通知模型 (Notification)
- 请求/响应结构体

#### `auth.go` (约150行)
- 密码加密/验证
- Token生成/验证
- Session管理
- 认证中间件
- 权限中间件

#### `handlers_user.go` (约200行)
- 用户注册接口
- 用户登录接口
- 用户登出接口
- 获取用户资料
- 更新用户资料

#### `handlers_article.go` (约400行)
- 获取文章列表
- 获取文章详情
- 创建文章
- 更新文章
- 删除文章
- 点赞文章
- 标签管理辅助函数

#### `handlers_comment.go` (约250行)
- 获取评论列表
- 创建评论
- 更新评论
- 删除评论
- 点赞评论
- 获取评论回复

#### `handlers_other.go` (约350行)
- 获取分类列表
- 获取标签列表
- 搜索文章
- 获取通知列表
- 标记通知已读
- 获取用户列表（管理员）
- 更新用户角色（管理员）
- 获取待审核评论（管理员）
- 审核评论（管理员）

### 前端文件

#### `index.html` (约300行)
页面结构：
- 导航栏
- 首页（文章列表）
- 文章详情页
- 文章编辑页
- 登录/注册页
- 通知页
- 管理页
- Toast提示组件

#### `styles.css` (约600行)
样式模块：
- 全局样式
- 导航栏样式
- 按钮样式
- 主内容区样式
- 搜索栏样式
- 文章列表样式
- 文章详情样式
- 评论区样式
- 表单样式
- 认证页面样式
- 通知列表样式
- 管理页面样式
- Toast提示样式
- 响应式设计

#### `app.js` (约800行)
功能模块：
- 应用初始化
- 页面切换逻辑
- 用户认证（注册、登录、登出）
- 文章管理（创建、编辑、删除、点赞）
- 评论管理（发表、回复、删除、点赞）
- 搜索与筛选
- 通知系统
- 管理功能
- 工具函数

### 文档文件

#### `README.md` (约300行)
- 项目介绍
- 功能特性
- 技术栈
- 项目结构
- 快速开始
- 默认账户
- API接口列表
- 用户角色说明
- 数据库表结构
- 安全性说明

#### `QUICKSTART.md` (约200行)
- Go环境安装指南
- 项目运行方法
- 使用流程说明
- 常见问题解答
- 开发提示
- 性能优化建议
- 故障排除

#### `API_DOCUMENTATION.md` (约600行)
- 25个API接口完整文档
- 请求/响应示例
- 状态码说明
- 权限说明
- 使用示例（JavaScript、cURL）
- 调试技巧

#### `DATABASE_DESIGN.md` (约400行)
- 8个数据表详细设计
- 表关系图（ERD）
- 业务逻辑说明
- 安全性设计
- 性能优化建议
- 数据库迁移指南
- 数据统计查询
- 备份恢复方法
- 维护建议

#### `TEST_CHECKLIST.md` (约300行)
- 60+功能测试项
- 用户认证测试
- 文章管理测试
- 评论系统测试
- 搜索筛选测试
- 通知系统测试
- 管理功能测试
- 界面交互测试
- 性能稳定性测试

#### `PROJECT_SUMMARY.md` (约400行)
- 项目概述
- 功能完成情况
- 技术架构
- 设计模式
- 代码统计
- 功能亮点
- 部署建议
- 性能指标
- 学习价值
- 扩展建议

---

## 📦 文件大小统计

### 源代码
```
Go后端:    约2000行  (8个文件)
前端HTML:   约300行  (1个文件)
前端CSS:    约600行  (1个文件)
前端JS:     约800行  (1个文件)
-----------------------------------
代码总计:   约3700行
```

### 文档
```
README.md:              约300行
QUICKSTART.md:          约200行
API_DOCUMENTATION.md:   约600行
DATABASE_DESIGN.md:     约400行
TEST_CHECKLIST.md:      约300行
PROJECT_SUMMARY.md:     约400行
-----------------------------------
文档总计:              约2200行
```

### 压缩包
```
blog-system.zip:     约36KB
blog-system.tar.gz:  约32KB
```

---

## 🎯 核心模块说明

### 1. 用户认证模块
**文件**: `auth.go`, `handlers_user.go`  
**功能**: 注册、登录、权限控制  
**安全**: bcrypt加密、Session Token

### 2. 文章管理模块
**文件**: `handlers_article.go`  
**功能**: CRUD操作、标签管理、点赞  
**特点**: 权限控制、分页支持

### 3. 评论系统模块
**文件**: `handlers_comment.go`  
**功能**: 多级评论、回复、审核  
**特点**: 嵌套结构、实时互动

### 4. 搜索筛选模块
**文件**: `handlers_other.go`  
**功能**: 全文搜索、分类筛选、标签筛选  
**特点**: 快速响应、多维度查询

### 5. 通知系统模块
**文件**: `handlers_other.go`  
**功能**: 实时通知、未读提醒  
**特点**: 自动触发、状态管理

### 6. 管理功能模块
**文件**: `handlers_other.go`  
**功能**: 用户管理、评论审核  
**特点**: 仅管理员可访问

---

## 🔧 技术依赖

### Go依赖包
```go
github.com/gorilla/mux v1.8.1        // 路由
github.com/mattn/go-sqlite3 v1.14.18 // SQLite驱动
golang.org/x/crypto v0.17.0          // bcrypt加密
```

### 前端依赖
```
无外部依赖 - 纯原生实现
```

---

## 🚀 快速导航

### 开始使用
1. 阅读 `README.md` 了解项目
2. 查看 `QUICKSTART.md` 快速上手
3. 运行 `start.sh` 或 `start.bat`
4. 访问 `http://localhost:8080`

### 开发参考
1. 查看 `API_DOCUMENTATION.md` 了解接口
2. 阅读 `DATABASE_DESIGN.md` 了解数据结构
3. 参考 `TEST_CHECKLIST.md` 进行测试

### 项目理解
1. 阅读 `PROJECT_SUMMARY.md` 了解全貌
2. 查看源代码注释
3. 运行项目实际体验

---

## 📝 文件命名规范

### Go文件
- `*_handler.go`: API处理函数
- `*.go`: 核心功能模块

### 文档文件
- `*.md`: Markdown格式文档
- 全大写: 重要文档（README等）

### 脚本文件
- `*.sh`: Linux/Mac脚本
- `*.bat`: Windows脚本

---

**项目已完整打包，可直接下载使用！** 🎉
