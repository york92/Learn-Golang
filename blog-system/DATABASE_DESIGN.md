# 数据库设计文档

## 数据库类型
SQLite3 (可轻松迁移至 MySQL/PostgreSQL)

## 数据库文件位置
`database/blog.db`

---

## 📊 表结构设计

### 1. users - 用户表
存储用户基本信息和认证数据

| 字段名 | 类型 | 约束 | 说明 |
|--------|------|------|------|
| id | INTEGER | PRIMARY KEY, AUTOINCREMENT | 用户ID |
| username | TEXT | UNIQUE, NOT NULL | 用户名 |
| email | TEXT | UNIQUE, NOT NULL | 邮箱 |
| password | TEXT | NOT NULL | 密码（bcrypt加密） |
| role | TEXT | DEFAULT 'reader' | 角色：admin/author/reader |
| avatar | TEXT | | 头像URL |
| created_at | DATETIME | DEFAULT CURRENT_TIMESTAMP | 创建时间 |
| updated_at | DATETIME | DEFAULT CURRENT_TIMESTAMP | 更新时间 |

**索引**:
- UNIQUE INDEX on username
- UNIQUE INDEX on email
- INDEX on role

---

### 2. articles - 文章表
存储博客文章内容

| 字段名 | 类型 | 约束 | 说明 |
|--------|------|------|------|
| id | INTEGER | PRIMARY KEY, AUTOINCREMENT | 文章ID |
| title | TEXT | NOT NULL | 文章标题 |
| content | TEXT | NOT NULL | 文章内容 |
| author_id | INTEGER | NOT NULL, FOREIGN KEY | 作者ID（关联users.id） |
| category | TEXT | | 文章分类 |
| cover_image | TEXT | | 封面图片URL |
| views | INTEGER | DEFAULT 0 | 浏览量 |
| likes | INTEGER | DEFAULT 0 | 点赞数 |
| status | TEXT | DEFAULT 'published' | 状态：published/draft |
| created_at | DATETIME | DEFAULT CURRENT_TIMESTAMP | 创建时间 |
| updated_at | DATETIME | DEFAULT CURRENT_TIMESTAMP | 更新时间 |

**索引**:
- INDEX on author_id
- INDEX on category
- INDEX on status
- INDEX on created_at

**外键**:
- author_id REFERENCES users(id)

---

### 3. tags - 标签表
存储文章标签

| 字段名 | 类型 | 约束 | 说明 |
|--------|------|------|------|
| id | INTEGER | PRIMARY KEY, AUTOINCREMENT | 标签ID |
| name | TEXT | UNIQUE, NOT NULL | 标签名称 |

**索引**:
- UNIQUE INDEX on name

---

### 4. article_tags - 文章标签关联表
多对多关系：文章与标签

| 字段名 | 类型 | 约束 | 说明 |
|--------|------|------|------|
| article_id | INTEGER | NOT NULL, FOREIGN KEY | 文章ID |
| tag_id | INTEGER | NOT NULL, FOREIGN KEY | 标签ID |

**主键**: (article_id, tag_id)

**外键**:
- article_id REFERENCES articles(id) ON DELETE CASCADE
- tag_id REFERENCES tags(id) ON DELETE CASCADE

**索引**:
- INDEX on article_id
- INDEX on tag_id

---

### 5. comments - 评论表
存储文章评论和回复

| 字段名 | 类型 | 约束 | 说明 |
|--------|------|------|------|
| id | INTEGER | PRIMARY KEY, AUTOINCREMENT | 评论ID |
| article_id | INTEGER | NOT NULL, FOREIGN KEY | 文章ID |
| user_id | INTEGER | NOT NULL, FOREIGN KEY | 评论用户ID |
| parent_id | INTEGER | FOREIGN KEY | 父评论ID（回复） |
| content | TEXT | NOT NULL | 评论内容 |
| likes | INTEGER | DEFAULT 0 | 点赞数 |
| status | TEXT | DEFAULT 'pending' | 状态：pending/approved/rejected |
| created_at | DATETIME | DEFAULT CURRENT_TIMESTAMP | 创建时间 |

**索引**:
- INDEX on article_id
- INDEX on user_id
- INDEX on parent_id
- INDEX on status
- INDEX on created_at

**外键**:
- article_id REFERENCES articles(id) ON DELETE CASCADE
- user_id REFERENCES users(id)
- parent_id REFERENCES comments(id) ON DELETE CASCADE

---

### 6. likes - 点赞表
存储点赞记录（文章和评论）

| 字段名 | 类型 | 约束 | 说明 |
|--------|------|------|------|
| id | INTEGER | PRIMARY KEY, AUTOINCREMENT | 点赞ID |
| user_id | INTEGER | NOT NULL, FOREIGN KEY | 用户ID |
| target_type | TEXT | NOT NULL | 目标类型：article/comment |
| target_id | INTEGER | NOT NULL | 目标ID |
| created_at | DATETIME | DEFAULT CURRENT_TIMESTAMP | 创建时间 |

**唯一约束**: (user_id, target_type, target_id)

**索引**:
- UNIQUE INDEX on (user_id, target_type, target_id)
- INDEX on target_type, target_id

**外键**:
- user_id REFERENCES users(id)

---

### 7. notifications - 通知表
存储用户通知消息

| 字段名 | 类型 | 约束 | 说明 |
|--------|------|------|------|
| id | INTEGER | PRIMARY KEY, AUTOINCREMENT | 通知ID |
| user_id | INTEGER | NOT NULL, FOREIGN KEY | 接收用户ID |
| type | TEXT | NOT NULL | 通知类型：comment/reply/like |
| content | TEXT | NOT NULL | 通知内容 |
| related_id | INTEGER | | 关联对象ID |
| is_read | INTEGER | DEFAULT 0 | 是否已读：0/1 |
| created_at | DATETIME | DEFAULT CURRENT_TIMESTAMP | 创建时间 |

**索引**:
- INDEX on user_id
- INDEX on is_read
- INDEX on created_at

**外键**:
- user_id REFERENCES users(id)

---

### 8. sessions - 会话表
存储用户登录会话

| 字段名 | 类型 | 约束 | 说明 |
|--------|------|------|------|
| id | INTEGER | PRIMARY KEY, AUTOINCREMENT | 会话ID |
| user_id | INTEGER | NOT NULL, FOREIGN KEY | 用户ID |
| token | TEXT | UNIQUE, NOT NULL | 会话令牌 |
| expires_at | DATETIME | NOT NULL | 过期时间 |
| created_at | DATETIME | DEFAULT CURRENT_TIMESTAMP | 创建时间 |

**索引**:
- UNIQUE INDEX on token
- INDEX on user_id
- INDEX on expires_at

**外键**:
- user_id REFERENCES users(id) ON DELETE CASCADE

---

## 🔗 表关系图（ERD）

```
users (1) ─────────── (N) articles
  │                        │
  │                        │
  │                   (N) ─┴─ (N) tags
  │                   article_tags
  │
  ├─────────── (N) comments
  │                    │
  │                    └── (自关联) parent_id
  │
  ├─────────── (N) likes
  │
  ├─────────── (N) notifications
  │
  └─────────── (N) sessions
```

---

## 📝 业务逻辑说明

### 用户角色权限
```
admin     - 完全权限，管理所有内容
author    - 可创建文章，管理自己的文章和评论
reader    - 可评论、点赞
guest     - 仅浏览公开内容
```

### 文章状态
```
published - 已发布（公开可见）
draft     - 草稿（仅作者和管理员可见）
```

### 评论状态
```
pending   - 待审核
approved  - 已通过
rejected  - 已拒绝
```

### 评论层级
- 支持两级评论：主评论 + 回复
- parent_id 为 NULL 表示主评论
- parent_id 有值表示回复某条评论

---

## 🔐 安全性设计

### 密码安全
- 使用 bcrypt 加密存储
- 成本因子：14
- 永不明文存储或传输

### 会话管理
- Token 随机生成（64位十六进制）
- 有效期：7天
- 自动清理过期会话

### SQL注入防护
- 使用参数化查询
- 输入验证和转义

### XSS防护
- 前端输出转义
- Content-Type 正确设置

---

## 📈 性能优化建议

### 索引优化
已在关键字段创建索引：
- 外键字段
- 查询频繁的字段
- 排序字段

### 查询优化
- 使用 JOIN 减少查询次数
- 分页加载（LIMIT + OFFSET）
- 避免 SELECT *

### 缓存策略（生产环境）
- 文章列表缓存（5分钟）
- 分类标签缓存（1小时）
- 用户会话缓存（Redis）

---

## 🔄 迁移到其他数据库

### MySQL 迁移
```sql
-- 修改自增长语法
AUTOINCREMENT → AUTO_INCREMENT

-- 添加字符集
DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci

-- 修改布尔类型
INTEGER (0/1) → BOOLEAN
```

### PostgreSQL 迁移
```sql
-- 修改自增长
AUTOINCREMENT → SERIAL

-- 修改布尔类型
INTEGER (0/1) → BOOLEAN

-- 修改时间函数
CURRENT_TIMESTAMP → NOW()
```

---

## 📊 数据统计查询

### 常用统计
```sql
-- 文章总数
SELECT COUNT(*) FROM articles WHERE status = 'published';

-- 用户总数
SELECT COUNT(*) FROM users;

-- 评论总数
SELECT COUNT(*) FROM comments WHERE status = 'approved';

-- 热门文章（按浏览量）
SELECT * FROM articles 
ORDER BY views DESC 
LIMIT 10;

-- 热门文章（按点赞数）
SELECT * FROM articles 
ORDER BY likes DESC 
LIMIT 10;

-- 活跃用户（按文章数）
SELECT u.username, COUNT(a.id) as article_count
FROM users u
LEFT JOIN articles a ON u.id = a.author_id
GROUP BY u.id
ORDER BY article_count DESC
LIMIT 10;
```

---

## 🗄️ 数据备份与恢复

### 备份
```bash
# 复制数据库文件
cp database/blog.db database/blog_backup_$(date +%Y%m%d).db
```

### 恢复
```bash
# 还原数据库文件
cp database/blog_backup_20240129.db database/blog.db
```

### 导出SQL（使用 sqlite3 命令）
```bash
sqlite3 database/blog.db .dump > backup.sql
```

### 导入SQL
```bash
sqlite3 database/blog.db < backup.sql
```

---

## 🔧 维护建议

### 定期维护
1. **清理过期会话**（每日）
   ```sql
   DELETE FROM sessions WHERE expires_at < datetime('now');
   ```

2. **清理已删除文章的孤立数据**（每周）
   ```sql
   DELETE FROM likes WHERE target_type = 'article' 
   AND target_id NOT IN (SELECT id FROM articles);
   ```

3. **数据库优化**（每月）
   ```sql
   VACUUM;
   ANALYZE;
   ```

### 监控指标
- 数据库文件大小
- 表记录数增长
- 查询性能
- 错误日志

---

## 📚 参考资料
- SQLite 官方文档: https://www.sqlite.org/docs.html
- Go database/sql: https://pkg.go.dev/database/sql
- bcrypt: https://pkg.go.dev/golang.org/x/crypto/bcrypt
