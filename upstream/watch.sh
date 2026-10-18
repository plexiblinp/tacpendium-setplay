#!/usr/bin/env bash
# 公開版 tacpendium の「見張るパス」が、基準(upstream/tacpendium.lock)から変わったかを調べる。
# 使い方: bash upstream/watch.sh [比較先の ref(既定: origin/main)]
# 終了コード: 0 = 変化なし / 1 = 変化あり / 2 = 実行エラー
# 書き込むのは一時ディレクトリと $GITHUB_STEP_SUMMARY(あれば)だけ。リポジトリは変更しない。
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
UPSTREAM_URL="${UPSTREAM_URL:-https://github.com/plexiblinp/tacpendium}"
TARGET="${1:-origin/main}"
BASE="$(tr -d '[:space:]' < "$HERE/tacpendium.lock")"
WATCH=(
  internal/service/setplay
  internal/repository/setplay
  internal/api/setplay
  internal/infra/migration/canary_setplay_approach_test.go
  internal/infra/migration/canary_setplay_rapid_test.go
  internal/model
  internal/service/setup/duplicate_keys.go
  internal/seedgen
  internal/moveindex
  internal/sanumber
  internal/gameversion
  web/src/features/setplay
  web/src/locales
  character_data
  migrations
)
OUT="${GITHUB_STEP_SUMMARY:-/dev/stdout}"

WORK="$(mktemp -d)"; trap 'rm -rf "$WORK"' EXIT
if ! git clone -q --filter=blob:none --no-checkout "$UPSTREAM_URL" "$WORK/tp" 2>"$WORK/err"; then
  echo "ERROR: 公開版 tacpendium を取得できませんでした" >&2; cat "$WORK/err" >&2; exit 2
fi
cd "$WORK/tp" || exit 2
git cat-file -e "$BASE^{commit}" 2>/dev/null || { echo "ERROR: 基準 $BASE が公開版に見つかりません" >&2; exit 2; }
HEAD_SHA="$(git rev-parse --verify -q "$TARGET^{commit}")" || { echo "ERROR: 比較先 $TARGET が見つかりません" >&2; exit 2; }

FILES="$(git diff --name-status "$BASE" "$HEAD_SHA" -- "${WATCH[@]}")"
{
  echo "### 上流(公開版 tacpendium)の見張り"
  echo ""
  echo "| 項目 | 値 |"
  echo "| --- | --- |"
  echo "| 基準 | \`$BASE\` |"
  echo "| 比較先 | \`$HEAD_SHA\` ($TARGET) |"
  echo "| 見張るパス | ${WATCH[*]} |"
  if [ -z "$FILES" ]; then
    echo "| 結果 | **変化なし** |"
  else
    echo "| 結果 | **変化あり: $(printf '%s\n' "$FILES" | wc -l) ファイル** |"
    echo ""
    echo '```'
    printf '%s\n' "$FILES"
    echo ""
    git diff --stat "$BASE" "$HEAD_SHA" -- "${WATCH[@]}" | tail -n 1
    echo '```'
    echo ""
    echo "取り込みは Claude Code で \`/sync_upstream $HEAD_SHA\`。"
  fi
} >> "$OUT"
[ -z "$FILES" ] && exit 0 || exit 1
