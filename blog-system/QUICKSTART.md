# 快速开始指南

## 安装Go环境

### Windows
1. 访问 https://go.dev/dl/
2. 下载 Windows 安装包（go1.21.x.windows-amd64.msi）
3. 运行安装程序
4. 打开命令提示符，输入 `go version` 验证安装

### macOS
```bash
# 使用Homebrew
brew install go

# 或下载安装包
# 访问 https://go.dev/dl/
```

### Linux (Ubuntu/Debian)
```bash
# 下载并安装
wget https://go.dev/dl/go1.21.5.linux-amd64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.21.5.linux-amd64.tar.gz

# 添加到PATH
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc

# 验证安装
go version
```

## 运行项目

### 方法1: 使用启动脚本（推荐）

**Windows:**
双击 `start.bat` 或在命令提示符中运行：
```cmd
start.bat
```

**Linux/macOS:**
```bash
chmod +x start.sh
./start.sh
```

### 方法2: 手动启动

```bash
# 1. 进入后端目录
cd backend

# 2. 下载依赖
go mod download

# 3. 创建数据库目录
mkdir -p ../database

# 4. 运行程序
go run *.go
```

## 访问应用

启动成功后，打开浏览器访问:
```
http://localhost:8080
```

## 默认账户

- **用户名**: admin
- **密码**: admin123
- **角色**: 管理员

## 使用流程

### 1. 注册普通用户
- 点击"注册"按钮
- 填写用户名、邮箱、密码
- 提交注册

### 2. 登录系统
- 使用用户名或邮箱登录
- 输入密码

### 3. 创建文章（需要作者或管理员权限）
- 登录后点击"写文章"
- 填写标题、内容、分类、标签
- 发布文章

### 4. 浏览和评论
- 点击文章标题查看详情
- 在文章下方发表评论
- 可以回复他人评论
- 可以点赞文章和评论

### 5. 管理功能（仅管理员）
- 点击"管理"进入管理中心
- 用户管理：修改用户角色
- 评论审核：审核待审核的评论

## 常见问题

### Q: 端口8080被占用怎么办？
A: 修改 `backend/main.go` 文件中的端口号：
```go
port := "8081"  // 改为其他端口
```

### Q: 如何修改管理员密码？
A: 
1. 登录管理员账户
2. 点击用户名进入个人资料
3. 修改密码

### Q: 数据库在哪里？
A: `database/blog.db` 文件

### Q: 如何备份数据？
A: 复制 `database/blog.db` 文件即可

### Q: 如何重置数据库？
A: 删除 `database/blog.db` 文件，重启服务器会自动重建

## 开发提示

### 修改后端代码后需要重启服务器
按 `Ctrl+C` 停止服务器，然后重新运行启动脚本

### 修改前端代码后刷新浏览器即可
前端代码修改后无需重启服务器，直接刷新浏览器页面

### 查看日志
服务器日志会在终端中显示，包括：
- 请求日志
- 错误信息
- 数据库操作

## 性能优化建议

1. **生产环境建议**:
   - 使用 MySQL 或 PostgreSQL 替代 SQLite
   - 配置 HTTPS
   - 启用 Gzip 压缩
   - 使用 Nginx 反向代理

2. **安全建议**:
   - 修改默认管理员密码
   - 启用评论审核
   - 定期备份数据库
   - 配置防火墙规则

## 故障排除

### 服务器无法启动
1. 检查Go是否正确安装: `go version`
2. 检查端口是否被占用
3. 检查是否有权限创建数据库文件

### 无法连接数据库
1. 检查 `database` 目录是否存在
2. 检查数据库文件权限
3. 删除数据库文件重新初始化

### 页面显示异常
1. 清除浏览器缓存
2. 检查浏览器控制台错误信息
3. 确认API请求地址正确

## 技术支持

如遇到问题：
1. 查看终端日志输出
2. 检查浏览器控制台
3. 查阅README.md文档
4. 提交GitHub Issue
