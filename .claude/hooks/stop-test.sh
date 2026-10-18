#!/usr/bin/env bash
# Stop(応答完了時): 変更した側のテストだけを回す(非ブロック)。
# 未コミットの差分＋未追跡ファイルに .go があれば各 go.mod で go test ./...、
# .ts/.tsx があれば各 package.json(test スクリプトと node_modules があるもの)で CI=1 pnpm -s test。
# 失敗は systemMessage で警告するだけで、常に exit 0。.upstream/ は対象外。
# git の読み取りとテスト実行のみ。ソース改変・git 書込・ネットワークアクセスを行わない。
set -uo pipefail
command -v jq >/dev/null 2>&1 || exit 0
ROOT="${CLAUDE_PROJECT_DIR:-$(pwd)}"
input="$(cat)"
[ "$(printf '%s' "$input" | jq -r '.stop_hook_active // false')" = "true" ] && exit 0
cd "$ROOT" 2>/dev/null || exit 0

changed="$( { git diff --name-only HEAD; git ls-files --others --exclude-standard; } 2>/dev/null)"
[ -z "$changed" ] && exit 0

find_dirs() {
  find . \( -path ./.upstream -o -name node_modules -o -path ./.git \) -prune -o -name "$1" -print 2>/dev/null \
    | xargs -r -n1 dirname | sort -u
}

summary=""
if printf '%s\n' "$changed" | grep -Eq '\.go$' && command -v go >/dev/null 2>&1; then
  for d in $(find_dirs go.mod); do
    out="$(cd "$d" && go test ./... 2>&1)" || summary+="go test 失敗 ($d):"$'\n'"$(printf '%s' "$out" | grep -E -- '^(--- FAIL|FAIL|panic|.*\.go:[0-9]+)' | head -n 40)"$'\n'
  done
fi
if printf '%s\n' "$changed" | grep -Eq '\.(ts|tsx)$' && command -v pnpm >/dev/null 2>&1; then
  for d in $(find_dirs package.json); do
    [ -d "$d/node_modules" ] || continue
    jq -e '.scripts.test' "$d/package.json" >/dev/null 2>&1 || continue
    out="$(cd "$d" && CI=1 pnpm -s test 2>&1)" || summary+="pnpm test 失敗 ($d):"$'\n'"$(printf '%s' "$out" | tail -n 40)"$'\n'
  done
fi

if [ -n "$summary" ]; then
  printf '%s\n' "$summary" >&2
  jq -n --arg m "[品質フック] テスト失敗を検知(非ブロック):"$'\n'"$summary" '{systemMessage: $m}'
fi
exit 0
