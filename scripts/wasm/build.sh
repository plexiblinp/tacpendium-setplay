#!/usr/bin/env bash
# エンジンの WASM(cmd/wasm)をビルドし、Go 付属の wasm_exec.js と並べて置く。
# 使い方: bash scripts/wasm/build.sh [置き場(既定: build/wasm)]
# 出力: <置き場>/setplay.wasm ・<置き場>/wasm_exec.js。最後に wasm の生と gzip 後のバイト数を出す。
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
OUT="${1:-$ROOT/build/wasm}"
mkdir -p "$OUT"
(cd "$ROOT" && GOOS=js GOARCH=wasm go build -trimpath -ldflags="-s -w" -o "$OUT/setplay.wasm" ./cmd/wasm)
cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" "$OUT/wasm_exec.js"
raw="$(wc -c < "$OUT/setplay.wasm")"
gz="$(gzip -9 -c "$OUT/setplay.wasm" | wc -c)"
echo "setplay.wasm: ${raw} bytes / gzip -9: ${gz} bytes"
