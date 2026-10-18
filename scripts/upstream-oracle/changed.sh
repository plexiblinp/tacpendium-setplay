#!/usr/bin/env bash
# 本家の正解(engine/testdata/upstream_oracle.json)の作り直しが要るかを判定する。
# 使い方: bash scripts/upstream-oracle/changed.sh <比較の基準(コミット)>
# 出力(標準出力の最後の行): changed = 作り直す / unchanged = 省いてよい
# 終了コード: 0 = 判定できた / 2 = 実行エラー
#
# 正解は「入力」だけで決まる(本家の版は lock、条件は oracle_conditions.json、作り方は生成器と Go の版)。
# 基準から HEAD までに入力のどれかが変わっていれば changed。正解そのものも入力に含め、
# 正解だけを手で書き換えた場合も作り直しで見つかるようにする。基準が見つからなければ changed(安全側)。
# ★.github/workflows/ci.yml は入力にしない(正解の中身を決めないため。入れると CI を触るブランチで毎回走る)。
set -uo pipefail
INPUTS=(
  upstream/tacpendium.lock
  engine/testdata/oracle_conditions.json
  engine/testdata/upstream_oracle.json
  scripts/upstream-oracle
  scripts/fetch-upstream.sh
  go.mod
)
BASE="${1:-}"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT" || exit 2
if [ -z "$BASE" ] || ! git cat-file -e "${BASE}^{commit}" 2>/dev/null; then
  echo "基準のコミット '${BASE}' が見つからない: 作り直す"
  echo changed
  exit 0
fi
if ! files="$(git diff --name-only "$BASE" HEAD -- "${INPUTS[@]}")"; then
  echo "ERROR: git diff に失敗しました" >&2
  exit 2
fi
if [ -n "$files" ]; then
  echo "入力の変更:"
  printf '  %s\n' $files
  echo changed
else
  echo "入力の変更なし(基準 ${BASE})"
  echo unchanged
fi
