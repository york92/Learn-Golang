#!/bin/bash

# 博客系统启动脚本

echo "========================================"
echo "  博客与评论系统 - 启动脚本"
echo "========================================"
echo ""

# 检查Go是否安装
if ! command -v go &> /dev/null; then
    echo "错误: 未检测到Go环境，请先安装Go 1.21或更高版本"
    exit 1
fi

echo "✓ Go版本: $(go version)"
echo ""

# 进入后端目录
cd backend

# 检查go.mod是否存在
if [ ! -f "go.mod" ]; then
    echo "错误: go.mod文件不存在"
    exit 1
fi

# 下载依赖
echo "正在下载依赖..."
go mod download
if [ $? -ne 0 ]; then
    echo "错误: 依赖下载失败"
    exit 1
fi
echo "✓ 依赖下载完成"
echo ""

# 创建数据库目录
mkdir -p ../database

# 启动服务器
echo "正在启动服务器..."
echo "服务器地址: http://localhost:8080"
echo "默认管理员账户: admin / admin123"
echo ""
echo "按 Ctrl+C 停止服务器"
echo "========================================"
echo ""

go run *.go
