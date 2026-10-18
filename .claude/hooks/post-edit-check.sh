#!/usr/bin/env bash
# PostToolUse(Write|Edit|MultiEdit): 編集したファイルへの高速チェック(非ブロック)。
# .go     → gofmt -l(書き換えない)+ 属するモジュールで go vet ./<pkg>
# .ts/.tsx → 上方の tsconfig.json のディレクトリで tsc --noEmit(node_modules があるときだけ)
# 指摘があれば additionalContext で Claude に返す。常に exit 0。
# 読み取り系の検査のみ。ソース改変・git 書込・ネットワークアクセスを行わない。
set -uo pipefail
command -v jq >/dev/null 2>&1 || exit 0
ROOT="${CLAUDE_PROJECT_DIR:-$(pwd)}"
input="$(cat)"
file="$(printf '%s' "$input" | jq -r '.tool_input.file_path // empty')"
[ -n "$file" ] && [ -f "$file" ] || exit 0
case "$file" in */node_modules/*) exit 0 ;; esac
rel="${file#"$ROOT"/}"

up_find() { # $1 = 開始ディレクトリ, $2 = 目印のファイル名。ROOT より上へは行かない
  local d="$1"
  while :; do
    [ -f "$d/$2" ] && { printf '%s' "$d"; return 0; }
    [ "$d" = "$ROOT" ] || [ "$d" = "/" ] && return 1
    d="$(dirname "$d")"
  done
}

findings=""
case "$file" in
  *.go)
    command -v gofmt >/dev/null 2>&1 && [ -n "$(gofmt -l "$file" 2>&1)" ] \
      && findings+="gofmt: 未フォーマット ($rel)。gofmt -w で整形すること。"$'\n'
    if mod="$(up_find "$(dirname "$file")" go.mod)"; then
      pkg="$(dirname "${file#"$mod"/}")"
      vet_out="$(cd "$mod" && go vet "./$pkg" 2>&1)" || findings+="go vet (./$pkg):"$'\n'"$vet_out"$'\n'
    fi
    ;;
  *.ts|*.tsx)
    if proj="$(up_find "$(dirname "$file")" tsconfig.json)" && [ -d "$proj/node_modules" ]; then
      tsc_out="$(cd "$proj" && pnpm -s exec tsc --noEmit 2>&1)" || findings+="tsc --noEmit (${proj#"$ROOT"/}):"$'\n'"$tsc_out"$'\n'
    fi
    ;;
esac

if [ -n "$findings" ]; then
  jq -n --arg ctx "[品質フック] $rel の編集後チェックで指摘:"$'\n'"$findings" \
    '{hookSpecificOutput: {hookEventName: "PostToolUse", additionalContext: $ctx}}'
fi
exit 0
