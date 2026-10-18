#!/usr/bin/env bash
# 本家(公開版 tacpendium)の実装で、単体版の適合性テストの正解(engine/testdata/upstream_oracle.json)を作る。
# 使い方: bash scripts/upstream-oracle/run.sh [書き出し先(既定: engine/testdata/upstream_oracle.json)]
# 前提: bash scripts/fetch-upstream.sh で .upstream/tacpendium が lock の SHA にあること(このスクリプトも呼ぶ)。
# 本家のチェックアウトへ一時的にテストファイルを 1 つ写して回し、終わったら消す(本家のファイルは変えない)。
# 終了コード: 0 = 成功 / それ以外 = 失敗
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
OUT="$(realpath -m "${1:-$ROOT/engine/testdata/upstream_oracle.json}")"
UP="$(bash "$ROOT/scripts/fetch-upstream.sh" | tail -n 1)"
DEST="$UP/internal/api/setplay/zz_standalone_oracle_test.go"
trap 'rm -f "$DEST"' EXIT
cp "$ROOT/scripts/upstream-oracle/standalone_oracle_test.go.txt" "$DEST"
if [ -n "$(gofmt -l "$DEST")" ]; then
  echo "ERROR: standalone_oracle_test.go.txt が gofmt されていません" >&2
  exit 1
fi
# 本家の他の変更が混ざっていないこと(写したファイル以外に差が無いこと)を確かめる。
if [ -n "$(git -C "$UP" status --porcelain --untracked-files=all | grep -v 'internal/api/setplay/zz_standalone_oracle_test.go$' || true)" ]; then
  echo "ERROR: 本家のチェックアウトに想定外の変更があります" >&2
  git -C "$UP" status --porcelain >&2
  exit 1
fi
(
  cd "$UP"
  ORACLE_CONDITIONS="$ROOT/engine/testdata/oracle_conditions.json" ORACLE_OUT="$OUT" \
    go test -count=1 -timeout 90m -run '^TestStandaloneOracle$' -v ./internal/api/setplay/
)
echo "書き出し: $OUT"
