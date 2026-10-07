#!/usr/bin/env bash
# 一键启动（无需安装）：自动选择可用的二进制，并默认创建演示账号。
#   ./scripts/start.sh            # 使用 ./data
#   ./scripts/start.sh -http :9090 -data /tmp/im
set -euo pipefail
cd "$(dirname "$0")/.."

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$(uname -m)" in x86_64|amd64) arch=amd64;; aarch64|arm64) arch=arm64;; *) arch="$(uname -m)";; esac

if   [ -x bin/imserver ];                    then BIN=bin/imserver
elif [ -x "dist/${os}-${arch}/imserver" ];   then BIN="dist/${os}-${arch}/imserver"
elif command -v go >/dev/null 2>&1;          then echo "编译中…"; make -s build; BIN=bin/imserver
else echo "找不到可执行文件：请先 'make build'（需要 Go 1.22+）或使用 dist/ 里的预编译包" >&2; exit 1; fi

CFG=()
[ -f config.json ] && CFG=(-config config.json)
exec "$BIN" "${CFG[@]}" -seed "$@"
