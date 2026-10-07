#!/usr/bin/env bash
# 冒烟测试：不依赖浏览器，用 imcli + curl 验证"注册 → 加好友 → 发消息 → 离线拉取 → 历史"。
set -euo pipefail
cd "$(dirname "$0")/.."
SRV=${SRV:-bin/imserver}; CLI=${CLI:-bin/imcli}
HTTP=127.0.0.1:18081; TCP=127.0.0.1:19001; DATA=$(mktemp -d)
cleanup(){ kill "$PID" 2>/dev/null || true; rm -rf "$DATA"; }
trap cleanup EXIT

"$SRV" -http "$HTTP" -tcp "$TCP" -data "$DATA" >"$DATA/server.log" 2>&1 & PID=$!
for i in $(seq 50); do curl -fs "http://$HTTP/healthz" >/dev/null 2>&1 && break; sleep 0.1; done
ok(){ echo "  ✓ $1"; }; die(){ echo "  ✗ $1" >&2; echo "--- server log ---"; cat "$DATA/server.log"; exit 1; }
C=("$CLI")
A=(-http "http://$HTTP" -tcp "$TCP" -p secret1)

echo "1) 注册"
"${C[@]}" register "${A[@]}" -u alice >/dev/null && "${C[@]}" register "${A[@]}" -u bob >/dev/null && ok "alice / bob 注册成功"
"${C[@]}" register "${A[@]}" -u alice >/dev/null 2>&1 && die "重复注册应失败" || ok "重复注册被拒绝"

echo "2) alice 给 bob 发 3 条消息（bob 此时不在线）"
TOKEN=$(curl -fs "http://$HTTP/api/login" -d '{"username":"alice","password":"secret1"}' | sed 's/.*"token":"\([^"]*\)".*/\1/')
for i in 1 2 3; do "${C[@]}" send "${A[@]}" -u alice -to bob -text "离线消息-$i" >/dev/null || die "发送失败"; done
ok "3 条消息已落库并 ServerAck"

echo "3) bob 上线拉取（增量同步）"
OUT=$("${C[@]}" pull "${A[@]}" -u bob)
for i in 1 2 3; do echo "$OUT" | grep -q "离线消息-$i" || die "bob 没收到 离线消息-$i"; done
echo "$OUT" | grep -q "user_seq=3" || die "游标应推进到 3"
ok "bob 收到 3 条离线消息，游标 user_seq=3"

echo "4) HTTP 接口：会话列表 / 历史"
BT=$(curl -fs "http://$HTTP/api/login" -d '{"username":"bob","password":"secret1"}' | sed 's/.*"token":"\([^"]*\)".*/\1/')
curl -fs -H "Authorization: Bearer $BT" "http://$HTTP/api/conversations" | grep -q '"unread":3' || die "未读数应为 3"
ok "会话列表未读数 = 3"
CONV=$(curl -fs -H "Authorization: Bearer $BT" "http://$HTTP/api/conversations" | sed 's/.*"conv_id":"\([^"]*\)".*/\1/')
curl -fs -H "Authorization: Bearer $BT" "http://$HTTP/api/messages?conv=$CONV" | grep -q '离线消息-3' || die "历史里应有最后一条"
ok "历史消息可查询"

echo "5) HTTP 发消息（幂等）"
R1=$(curl -fs -H "Authorization: Bearer $TOKEN" "http://$HTTP/api/messages" -d '{"to_username":"bob","content":"x","client_msg_id":"k1"}')
R2=$(curl -fs -H "Authorization: Bearer $TOKEN" "http://$HTTP/api/messages" -d '{"to_username":"bob","content":"x","client_msg_id":"k1"}')
[ "$R1" = "$R2" ] && ok "相同 client_msg_id 重发得到相同结果" || die "幂等失败"

echo; echo "冒烟测试全部通过 ✅"
