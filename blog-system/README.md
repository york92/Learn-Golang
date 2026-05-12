# 博客与评论系统

一个功能完整的博客与评论系统，使用Go后端、SQLite3数据库和原生HTML/CSS/JavaScript前端开发。

## 功能特性

### 用户系统
- ✅ 用户注册与登录
- ✅ 多角色权限管理（管理员/作者/读者/访客）
- ✅ 用户个人资料管理
- ✅ 基于Session的身份认证

### 文章管理
- ✅ 创建、编辑、删除文章
- ✅ 文章分类和标签
- ✅ 文章浏览量统计
- ✅ 文章点赞功能
- ✅ 文章列表展示和分页
- ✅ 文章详情页

### 评论系统
- ✅ 发表评论
- ✅ 多级评论（回复功能）
- ✅ 评论点赞
- ✅ 评论审核（管理员）
- ✅ 删除评论

### 搜索与筛选
- ✅ 全文搜索（标题、内容、作者）
- ✅ 按分类筛选
- ✅ 按标签筛选

### 通知系统
- ✅ 评论通知
- ✅ 回复通知
- ✅ 未读消息提醒

### 管理功能
- ✅ 用户管理（管理员）
- ✅ 角色权限管理
- ✅ 评论审核

## 技术栈

### 后端
- **语言**: Go 1.21+
- **框架**: Gorilla Mux (路由)
- **数据库**: SQLite3
- **认证**: bcrypt密码加密 + Session Token

### 前端
- **HTML5**: 页面结构
- **CSS3**: 样式设计（响应式布局）
- **JavaScript**: 原生JS（无框架）

## 项目结构

```
blog-system/
├── backend/                # Go后端
│   ├── main.go            # 主程序入口
│   ├── database.go        # 数据库初始化
│   ├── models.go          # 数据模型定义
│   ├── auth.go            # 认证中间件
│   ├── handlers_user.go   # 用户处理函数
│   ├── handlers_article.go # 文章处理函数
│   ├── handlers_comment.go # 评论处理函数
│   ├── handlers_other.go  # 其他处理函数
│   └── go.mod             # Go依赖管理
├── frontend/              # 前端文件
│   ├── index.html         # 主页面
│   ├── styles.css         # 样式文件
│   └── app.js             # JavaScript逻辑
├── database/              # 数据库目录
│   └── blog.db            # SQLite数据库（运行时生成）
├── README.md              # 项目文档
└── start.sh               # 启动脚本
```

## 快速开始

### 前置要求
- Go 1.21 或更高版本
- 现代Web浏览器

### 安装步骤

1. **初始化Go模块并下载依赖**
   ```bash
   cd backend
   go mod download
   ```

2. **启动服务器**
   ```bash
   go run *.go
   ```
   
   或使用启动脚本（Linux/Mac）：
   ```bash
   chmod +x start.sh
   ./start.sh
   ```

3. **访问应用**
   打开浏览器访问: `http://localhost:8080`

## 默认账户

系统会自动创建一个管理员账户：
- **用户名**: admin
- **密码**: admin123

建议首次登录后立即修改密码。

## API接口文档

### 用户相关
- `POST /api/register` - 用户注册
- `POST /api/login` - 用户登录
- `POST /api/logout` - 用户登出
- `GET /api/profile` - 获取用户资料
- `PUT /api/profile` - 更新用户资料

### 文章相关
- `GET /api/articles` - 获取文章列表
- `POST /api/articles` - 创建文章
- `GET /api/articles/{id}` - 获取文章详情
- `PUT /api/articles/{id}` - 更新文章
- `DELETE /api/articles/{id}` - 删除文章
- `POST /api/articles/{id}/like` - 点赞文章

### 评论相关
- `GET /api/articles/{id}/comments` - 获取文章评论
- `POST /api/articles/{id}/comments` - 发表评论
- `PUT /api/comments/{id}` - 更新评论
- `DELETE /api/comments/{id}` - 删除评论
- `POST /api/comments/{id}/like` - 点赞评论

### 其他接口
- `GET /api/categories` - 获取分类列表
- `GET /api/tags` - 获取标签列表
- `GET /api/search?q=keyword` - 搜索文章
- `GET /api/notifications` - 获取通知列表
- `PUT /api/notifications/{id}/read` - 标记通知已读

### 管理员接口
- `GET /api/admin/users` - 获取用户列表
- `PUT /api/admin/users/{id}` - 更新用户角色
- `GET /api/admin/comments/pending` - 获取待审核评论
- `PUT /api/admin/comments/{id}/approve` - 审核评论

## 用户角色说明

### 管理员 (Admin)
- 全部权限
- 管理所有文章和评论
- 管理用户角色
- 审核评论

### 作者 (Author)
- 创建、编辑、删除自己的文章
- 管理自己文章的评论
- 发表评论

### 读者 (Reader)
- 浏览文章
- 发表评论
- 点赞文章和评论

### 访客 (Guest)
- 仅浏览公开文章
- 需要登录才能互动

## 开发说明

### 数据库表结构

**users** - 用户表
- id, username, email, password, role, avatar, created_at, updated_at

**articles** - 文章表
- id, title, content, author_id, category, cover_image, views, likes, status, created_at, updated_at

**comments** - 评论表
- id, article_id, user_id, parent_id, content, likes, status, created_at

**tags** - 标签表
- id, name

**article_tags** - 文章标签关联表
- article_id, tag_id

**likes** - 点赞表
- id, user_id, target_type, target_id, created_at

**notifications** - 通知表
- id, user_id, type, content, related_id, is_read, created_at

**sessions** - 会话表
- id, user_id, token, expires_at, created_at

## 安全性

- 密码使用bcrypt加密存储
- Session token随机生成
- API接口身份验证
- SQL注入防护
- XSS防护

## 注意事项

1. 本项目使用SQLite作为数据库，适合中小型应用
2. 生产环境建议更换为MySQL/PostgreSQL
3. 建议配置HTTPS
4. 定期备份数据库文件

## 许可证

MIT License

## 贡献

欢迎提交Issue和Pull Request！

## 联系方式

如有问题，请通过GitHub Issues联系。
