#!/usr/bin/env bash
# Web 客户端 UI 测试：自动起一个临时服务端 → 运行 jsdom 测试 → 清理。需要 Node.js 18+。
set -euo pipefail
cd "$(dirname "$0")/.."
command -v node >/dev/null || { echo "需要 Node.js" >&2; exit 1; }
DATA=$(mktemp -d); trap 'kill $PID 2>/dev/null || true; rm -rf "$DATA"' EXIT
bin/imserver -http 127.0.0.1:18080 -tcp 127.0.0.1:19000 -data "$DATA" >"$DATA/server.log" 2>&1 & PID=$!
for i in $(seq 50); do curl -fs http://127.0.0.1:18080/healthz >/dev/null 2>&1 && break; sleep 0.1; done
(cd tests/ui && [ -d node_modules ] || npm install --silent)
(cd tests/ui && npm test --silent)
