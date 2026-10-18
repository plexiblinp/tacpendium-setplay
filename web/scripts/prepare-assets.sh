#!/usr/bin/env bash
# サイトが読むデータと WASM を web/public/ の下に用意する(どちらも gitignore。公開物は CI からだけ出す＝AGENTS §8-12)。
#   public/data/  … 事前計算の出力(go run ./cmd/precompute。manifest.json・moves/。
#                   計算はブラウザの WASM＝S5 の開発者決定: 計算方式 C)
#   public/wasm/  … エンジンの WASM と Go 付属の wasm_exec.js(scripts/wasm/build.sh。同じ Go で作るので版が揃う)
# 使い方: bash scripts/prepare-assets.sh [-chars ryu,ken]   (引数は cmd/precompute へそのまま渡す)
set -euo pipefail
WEB="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ROOT="$(cd "$WEB/.." && pwd)"
rm -rf "$WEB/public/data" "$WEB/public/wasm"
(cd "$ROOT" && go run ./cmd/precompute -out "$WEB/public/data" "$@")
bash "$ROOT/scripts/wasm/build.sh" "$WEB/public/wasm"
