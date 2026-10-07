@echo off
REM Windows 一键启动（使用 dist\windows-amd64 里的预编译文件，或同目录下的 imserver.exe）
cd /d "%~dp0\.."
if exist dist\windows-amd64\imserver.exe (
  dist\windows-amd64\imserver.exe -seed %*
) else if exist imserver.exe (
  imserver.exe -seed %*
) else (
  echo 找不到 imserver.exe
  exit /b 1
)
