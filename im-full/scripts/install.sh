#!/usr/bin/env bash
# Linux 一键安装为 systemd 服务（需要 root）。
#   sudo ./scripts/install.sh              安装并启动
#   sudo ./scripts/install.sh --uninstall  卸载（保留数据目录）
set -euo pipefail
cd "$(dirname "$0")/.."

PREFIX=/usr/local/bin; CONF=/etc/im; DATA=/var/lib/im; USER_NAME=im

if [ "$(id -u)" -ne 0 ]; then echo "请用 root 运行：sudo $0" >&2; exit 1; fi

if [ "${1:-}" = "--uninstall" ]; then
  systemctl disable --now imserver 2>/dev/null || true
  rm -f /etc/systemd/system/imserver.service "$PREFIX/imserver" "$PREFIX/imcli"
  systemctl daemon-reload 2>/dev/null || true
  echo "已卸载。数据与配置保留在 $DATA 和 $CONF，如不需要请手动删除。"; exit 0
fi

case "$(uname -m)" in x86_64|amd64) arch=amd64;; aarch64|arm64) arch=arm64;; *) echo "不支持的架构 $(uname -m)" >&2; exit 1;; esac
if   [ -x bin/imserver ];                 then SRC=bin
elif [ -x "dist/linux-${arch}/imserver" ]; then SRC="dist/linux-${arch}"
elif command -v go >/dev/null 2>&1;      then make -s build; SRC=bin
else echo "找不到二进制：请先 make build 或使用 dist/ 里的预编译包" >&2; exit 1; fi

id -u "$USER_NAME" >/dev/null 2>&1 || useradd --system --home "$DATA" --shell /usr/sbin/nologin "$USER_NAME"
install -d -o "$USER_NAME" -m 750 "$DATA"
install -d -m 755 "$CONF"
install -m 755 "$SRC/imserver" "$SRC/imcli" "$PREFIX/"
[ -f "$CONF/config.json" ] || sed "s#\"./data\"#\"$DATA\"#" configs/config.example.json > "$CONF/config.json"
install -m 644 deploy/imserver.service /etc/systemd/system/imserver.service
systemctl daemon-reload
systemctl enable --now imserver
sleep 1
systemctl --no-pager --lines=5 status imserver || true
echo
echo "安装完成。  Web: http://<服务器IP>:8080   TCP: :9000"
echo "配置文件: $CONF/config.json   数据目录: $DATA   日志: journalctl -u imserver -f"
