#!/usr/bin/env bash
# 交叉编译：在 ./dist/<os>-<arch>/ 下生成 imserver 与 imcli（纯 Go、无 CGO，Web 页面已内嵌）。
set -euo pipefail
cd "$(dirname "$0")/.."
VERSION="${VERSION:-0.1.0}"
rm -rf dist && mkdir -p dist
for target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64; do
  os="${target%/*}"; arch="${target#*/}"; ext=""; [ "$os" = windows ] && ext=".exe"
  out="dist/${os}-${arch}"; mkdir -p "$out"
  for cmd in imserver imcli; do
    CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -trimpath \
      -ldflags "-s -w -X main.version=${VERSION}" -o "${out}/${cmd}${ext}" "./cmd/${cmd}"
  done
  cp configs/config.example.json "$out/config.json"
  echo "built $out"
done
(cd dist && (sha256sum */* 2>/dev/null || shasum -a 256 */*) > SHA256SUMS)
echo "完成：dist/ （校验和见 dist/SHA256SUMS）"
