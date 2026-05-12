@echo off
chcp 65001 >nul
echo ========================================
echo   博客与评论系统 - 启动脚本
echo ========================================
echo.

REM 检查Go是否安装
go version >nul 2>&1
if errorlevel 1 (
    echo 错误: 未检测到Go环境，请先安装Go 1.21或更高版本
    pause
    exit /b 1
)

echo ✓ Go环境已安装
echo.

REM 进入后端目录
cd backend

REM 下载依赖
echo 正在下载依赖...
go mod download
if errorlevel 1 (
    echo 错误: 依赖下载失败
    pause
    exit /b 1
)
echo ✓ 依赖下载完成
echo.

REM 创建数据库目录
if not exist "..\database" mkdir "..\database"

REM 启动服务器
echo 正在启动服务器...
echo 服务器地址: http://localhost:8080
echo 默认管理员账户: admin / admin123
echo.
echo 按 Ctrl+C 停止服务器
echo ========================================
echo.

go run *.go
pause
