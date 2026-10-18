#!/usr/bin/env bash
# GitHub Actions の workflow を検査する(HANDOVER §7.4)。
#   1. `uses:` の参照がコミット SHA(40 桁)で固定されている(ローカルの `./` 参照と docker:// は除く)
#   2. 各 workflow がトップレベルに `permissions:` を持ち、`write-all` を使わない
#      (個別の `: write` は job に必要なことがあるので止めない。レビューで見る)
# 使い方: bash scripts/check-actions-pinned.sh [ディレクトリ(既定: .github/workflows)]
# 終了コード: 0 = 違反なし / 1 = 違反あり / 2 = 実行エラー
set -uo pipefail
DIR="${1:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/.github/workflows}"
[ -d "$DIR" ] || { echo "ERROR: $DIR がありません" >&2; exit 2; }

shopt -s nullglob
files=("$DIR"/*.yml "$DIR"/*.yaml)
[ "${#files[@]}" -gt 0 ] || { echo "ERROR: $DIR に workflow がありません" >&2; exit 2; }

violations=0
refs=0
for f in "${files[@]}"; do
  while IFS= read -r line; do
    lineno="${line%%:*}"
    ref="$(printf '%s' "${line#*:}" | sed -E 's/^[[:space:]]*-?[[:space:]]*uses:[[:space:]]*//; s/[[:space:]]*#.*$//; s/^["'\'']//; s/["'\'']$//')"
    case "$ref" in ./*|docker://*) continue ;; esac
    refs=$((refs + 1))
    if ! printf '%s' "$ref" | grep -Eq '^[^@[:space:]]+@[0-9a-f]{40}$'; then
      echo "ERROR: ${f##*/}:$lineno: SHA 固定でない参照: $ref"
      violations=$((violations + 1))
    fi
  done < <(grep -nE '^[[:space:]]*-?[[:space:]]*uses:' "$f")
  if ! grep -Eq '^permissions:' "$f"; then
    echo "ERROR: ${f##*/}: トップレベルの permissions: がありません"
    violations=$((violations + 1))
  fi
  if grep -qE '^[[:space:]]*permissions:[[:space:]]*["'\'']?write-all' "$f"; then
    echo "ERROR: ${f##*/}: permissions: write-all は使わない"
    violations=$((violations + 1))
  fi
done

echo "workflow: ${#files[@]} 本 / uses 参照: $refs 件"
echo "違反: $violations 件"
[ "$violations" -eq 0 ]
