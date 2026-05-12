# API 接口文档

## 基础信息

**Base URL**: `http://localhost:8080/api`

**认证方式**: Bearer Token (在请求头中添加 `Authorization: Bearer {token}`)

**响应格式**: JSON

---

## 通用响应格式

### 成功响应
```json
{
  "success": true,
  "message": "操作成功消息（可选）",
  "data": { } // 返回数据（可选）
}
```

### 失败响应
```json
{
  "success": false,
  "message": "错误消息"
}
```

---

## 🔐 用户认证接口

### 1. 用户注册
**POST** `/register`

**请求体**:
```json
{
  "username": "testuser",
  "email": "test@example.com",
  "password": "password123"
}
```

**响应**:
```json
{
  "success": true,
  "message": "注册成功",
  "data": {
    "user_id": 1
  }
}
```

---

### 2. 用户登录
**POST** `/login`

**请求体**:
```json
{
  "username": "testuser",  // 用户名或邮箱
  "password": "password123"
}
```

**响应**:
```json
{
  "success": true,
  "message": "登录成功",
  "data": {
    "token": "abc123...",
    "user": {
      "id": 1,
      "username": "testuser",
      "email": "test@example.com",
      "role": "reader",
      "avatar": "",
      "created_at": "2024-01-29T10:00:00Z"
    }
  }
}
```

---

### 3. 用户登出
**POST** `/logout`

**需要认证**: ✅

**响应**:
```json
{
  "success": true,
  "message": "登出成功"
}
```

---

### 4. 获取用户资料
**GET** `/profile`

**需要认证**: ✅

**响应**:
```json
{
  "success": true,
  "data": {
    "id": 1,
    "username": "testuser",
    "email": "test@example.com",
    "role": "reader",
    "avatar": "",
    "created_at": "2024-01-29T10:00:00Z"
  }
}
```

---

### 5. 更新用户资料
**PUT** `/profile`

**需要认证**: ✅

**请求体**:
```json
{
  "email": "newemail@example.com",
  "avatar": "https://example.com/avatar.jpg"
}
```

**响应**:
```json
{
  "success": true,
  "message": "更新成功"
}
```

---

## 📝 文章管理接口

### 6. 获取文章列表
**GET** `/articles`

**查询参数**:
- `category` (可选): 按分类筛选
- `tag` (可选): 按标签筛选
- `page` (可选): 页码，默认1

**示例**: `/articles?category=技术&page=1`

**响应**:
```json
{
  "success": true,
  "data": [
    {
      "id": 1,
      "title": "文章标题",
      "content": "文章摘要...",
      "author_id": 1,
      "author_name": "作者名",
      "category": "技术",
      "cover_image": "https://example.com/cover.jpg",
      "tags": ["Go", "Web"],
      "views": 100,
      "likes": 10,
      "status": "published",
      "created_at": "2024-01-29T10:00:00Z",
      "updated_at": "2024-01-29T10:00:00Z"
    }
  ]
}
```

---

### 7. 获取文章详情
**GET** `/articles/{id}`

**路径参数**:
- `id`: 文章ID

**响应**:
```json
{
  "success": true,
  "data": {
    "id": 1,
    "title": "完整文章标题",
    "content": "完整文章内容...",
    "author_id": 1,
    "author_name": "作者名",
    "category": "技术",
    "cover_image": "https://example.com/cover.jpg",
    "tags": ["Go", "Web"],
    "views": 101,
    "likes": 10,
    "status": "published",
    "created_at": "2024-01-29T10:00:00Z",
    "updated_at": "2024-01-29T10:00:00Z"
  }
}
```

---

### 8. 创建文章
**POST** `/articles`

**需要认证**: ✅ (作者或管理员)

**请求体**:
```json
{
  "title": "新文章标题",
  "content": "文章内容...",
  "category": "技术",
  "cover_image": "https://example.com/cover.jpg",
  "tags": ["Go", "Web", "开发"]
}
```

**响应**:
```json
{
  "success": true,
  "message": "创建成功",
  "data": {
    "article_id": 1
  }
}
```

---

### 9. 更新文章
**PUT** `/articles/{id}`

**需要认证**: ✅ (作者本人或管理员)

**路径参数**:
- `id`: 文章ID

**请求体**: (同创建文章)

**响应**:
```json
{
  "success": true,
  "message": "更新成功"
}
```

---

### 10. 删除文章
**DELETE** `/articles/{id}`

**需要认证**: ✅ (作者本人或管理员)

**路径参数**:
- `id`: 文章ID

**响应**:
```json
{
  "success": true,
  "message": "删除成功"
}
```

---

### 11. 点赞文章
**POST** `/articles/{id}/like`

**需要认证**: ✅

**路径参数**:
- `id`: 文章ID

**说明**: 重复调用可取消点赞

**响应**:
```json
{
  "success": true,
  "message": "操作成功"
}
```

---

## 💬 评论管理接口

### 12. 获取文章评论
**GET** `/articles/{id}/comments`

**路径参数**:
- `id`: 文章ID

**响应**:
```json
{
  "success": true,
  "data": [
    {
      "id": 1,
      "article_id": 1,
      "user_id": 2,
      "username": "评论者",
      "user_avatar": "",
      "parent_id": null,
      "content": "这是一条评论",
      "likes": 5,
      "status": "approved",
      "created_at": "2024-01-29T10:00:00Z",
      "replies": [
        {
          "id": 2,
          "parent_id": 1,
          "content": "这是一条回复",
          ...
        }
      ]
    }
  ]
}
```

---

### 13. 发表评论
**POST** `/articles/{id}/comments`

**需要认证**: ✅

**路径参数**:
- `id`: 文章ID

**请求体**:
```json
{
  "content": "评论内容",
  "parent_id": null  // 回复时填写父评论ID
}
```

**响应**:
```json
{
  "success": true,
  "message": "评论成功",
  "data": {
    "comment_id": 1
  }
}
```

---

### 14. 更新评论
**PUT** `/comments/{id}`

**需要认证**: ✅ (评论者本人或管理员)

**路径参数**:
- `id`: 评论ID

**请求体**:
```json
{
  "content": "修改后的评论内容"
}
```

**响应**:
```json
{
  "success": true,
  "message": "更新成功"
}
```

---

### 15. 删除评论
**DELETE** `/comments/{id}`

**需要认证**: ✅ (评论者本人、文章作者或管理员)

**路径参数**:
- `id`: 评论ID

**响应**:
```json
{
  "success": true,
  "message": "删除成功"
}
```

---

### 16. 点赞评论
**POST** `/comments/{id}/like`

**需要认证**: ✅

**路径参数**:
- `id`: 评论ID

**说明**: 重复调用可取消点赞

**响应**:
```json
{
  "success": true,
  "message": "操作成功"
}
```

---

## 🔖 分类和标签接口

### 17. 获取分类列表
**GET** `/categories`

**响应**:
```json
{
  "success": true,
  "data": ["技术", "生活", "旅行"]
}
```

---

### 18. 获取标签列表
**GET** `/tags`

**响应**:
```json
{
  "success": true,
  "data": [
    {
      "id": 1,
      "name": "Go"
    },
    {
      "id": 2,
      "name": "Web开发"
    }
  ]
}
```

---

## 🔍 搜索接口

### 19. 搜索文章
**GET** `/search`

**查询参数**:
- `q` (必填): 搜索关键词

**示例**: `/search?q=Go语言`

**响应**: (同文章列表格式)

---

## 🔔 通知接口

### 20. 获取通知列表
**GET** `/notifications`

**需要认证**: ✅

**响应**:
```json
{
  "success": true,
  "data": [
    {
      "id": 1,
      "user_id": 1,
      "type": "comment",
      "content": "用户A评论了你的文章",
      "related_id": 10,
      "is_read": false,
      "created_at": "2024-01-29T10:00:00Z"
    }
  ]
}
```

---

### 21. 标记通知已读
**PUT** `/notifications/{id}/read`

**需要认证**: ✅

**路径参数**:
- `id`: 通知ID

**响应**:
```json
{
  "success": true,
  "message": "标记成功"
}
```

---

## 👨‍💼 管理员接口

### 22. 获取用户列表
**GET** `/admin/users`

**需要认证**: ✅ (仅管理员)

**响应**:
```json
{
  "success": true,
  "data": [
    {
      "id": 1,
      "username": "admin",
      "email": "admin@blog.com",
      "role": "admin",
      "avatar": "",
      "created_at": "2024-01-29T10:00:00Z"
    }
  ]
}
```

---

### 23. 更新用户角色
**PUT** `/admin/users/{id}`

**需要认证**: ✅ (仅管理员)

**路径参数**:
- `id`: 用户ID

**请求体**:
```json
{
  "role": "author"  // admin/author/reader
}
```

**响应**:
```json
{
  "success": true,
  "message": "更新成功"
}
```

---

### 24. 获取待审核评论
**GET** `/admin/comments/pending`

**需要认证**: ✅ (仅管理员)

**响应**: (同评论列表格式)

---

### 25. 审核评论
**PUT** `/admin/comments/{id}/approve`

**需要认证**: ✅ (仅管理员)

**路径参数**:
- `id`: 评论ID

**请求体**:
```json
{
  "status": "approved"  // approved/rejected
}
```

**响应**:
```json
{
  "success": true,
  "message": "操作成功"
}
```

---

## 📊 状态码说明

| 状态码 | 说明 |
|--------|------|
| 200 | 请求成功 |
| 201 | 创建成功 |
| 400 | 请求参数错误 |
| 401 | 未授权（未登录或token无效） |
| 403 | 禁止访问（权限不足） |
| 404 | 资源不存在 |
| 409 | 资源冲突（如用户名已存在） |
| 500 | 服务器内部错误 |

---

## 🔒 权限说明

| 角色 | 权限 |
|------|------|
| Guest | 浏览公开文章 |
| Reader | Guest权限 + 评论、点赞 |
| Author | Reader权限 + 创建文章、管理自己的文章 |
| Admin | 所有权限 + 用户管理、评论审核 |

---

## 📝 使用示例

### JavaScript Fetch 示例

```javascript
// 登录
const loginResponse = await fetch('http://localhost:8080/api/login', {
  method: 'POST',
  headers: {
    'Content-Type': 'application/json'
  },
  body: JSON.stringify({
    username: 'admin',
    password: 'admin123'
  })
});
const loginData = await loginResponse.json();
const token = loginData.data.token;

// 创建文章（需要token）
const createArticleResponse = await fetch('http://localhost:8080/api/articles', {
  method: 'POST',
  headers: {
    'Content-Type': 'application/json',
    'Authorization': `Bearer ${token}`
  },
  body: JSON.stringify({
    title: '我的第一篇文章',
    content: '这是文章内容...',
    category: '技术',
    tags: ['Go', 'Web']
  })
});
```

### cURL 示例

```bash
# 登录
curl -X POST http://localhost:8080/api/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}'

# 获取文章列表
curl http://localhost:8080/api/articles

# 创建文章（需要token）
curl -X POST http://localhost:8080/api/articles \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{"title":"标题","content":"内容","category":"技术","tags":["Go"]}'
```

---

## 🚀 调试技巧

### 1. 使用浏览器开发者工具
- 打开Network标签查看API请求
- 查看请求头、响应体、状态码

### 2. 使用Postman
- 导入API接口进行测试
- 设置环境变量存储token

### 3. 查看服务器日志
- 终端中会显示所有API请求日志
- 包含错误信息和堆栈跟踪

---

## 📚 相关文档
- [README.md](README.md) - 项目说明
- [QUICKSTART.md](QUICKSTART.md) - 快速开始
- [DATABASE_DESIGN.md](DATABASE_DESIGN.md) - 数据库设计
- [TEST_CHECKLIST.md](TEST_CHECKLIST.md) - 测试清单
