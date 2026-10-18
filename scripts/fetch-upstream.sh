#!/usr/bin/env bash
# 公開版 tacpendium を upstream/tacpendium.lock の SHA で取得する(データの唯一の取込元＝HANDOVER D2)。
# 使い方: bash scripts/fetch-upstream.sh [置き場(既定: .upstream/tacpendium)]
# 置き場に同じ SHA が既にあれば何もしない。最後の行に置き場の絶対パスを出す。
# 終了コード: 0 = 成功 / 2 = 実行エラー
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
UPSTREAM_URL="${UPSTREAM_URL:-https://github.com/plexiblinp/tacpendium}"
SHA="$(tr -d '[:space:]' < "$ROOT/upstream/tacpendium.lock")"
DEST="${1:-$ROOT/.upstream/tacpendium}"

if [ -d "$DEST/.git" ] && [ "$(git -C "$DEST" rev-parse HEAD 2>/dev/null)" = "$SHA" ]; then
  (cd "$DEST" && pwd)
  exit 0
fi
rm -rf "$DEST"
mkdir -p "$DEST"
git -C "$DEST" init -q
git -C "$DEST" remote add origin "$UPSTREAM_URL"
if ! git -C "$DEST" fetch -q --depth 1 origin "$SHA"; then
  echo "ERROR: 公開版 tacpendium の $SHA を取得できませんでした" >&2
  exit 2
fi
git -C "$DEST" -c advice.detachedHead=false checkout -q FETCH_HEAD
got="$(git -C "$DEST" rev-parse HEAD)"
if [ "$got" != "$SHA" ]; then
  echo "ERROR: 取得した SHA が lock と違います: $got != $SHA" >&2
  exit 2
fi
(cd "$DEST" && pwd)
