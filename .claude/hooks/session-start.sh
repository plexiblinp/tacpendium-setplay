#!/usr/bin/env bash
# SessionStart(startup|resume): 依存を冪等に揃える(非ブロック)。
# - pnpm-lock.yaml があるディレクトリで node_modules が無ければ pnpm install --frozen-lockfile
# - go.mod があるディレクトリで go mod download
# 対象がまだ無ければ何もしない。.upstream/(公開版の取得先)・node_modules/ は対象外。
# 失敗は警告に留め、必ず exit 0(環境整備が転けてセッションが始まらないのが最悪)。
# 出力は stderr のみ。ソース改変・git 書込は行わない(取得先は公式のパッケージレジストリのみ)。
set -uo pipefail
ROOT="${CLAUDE_PROJECT_DIR:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)}"
cd "$ROOT" 2>/dev/null || exit 0

warn() { printf 'session-start: %s\n' "$*" >&2; }
run_capped() {
  local secs="$1"; shift
  if command -v timeout >/dev/null 2>&1; then timeout "$secs" "$@"; else "$@"; fi
}
find_dirs() { # $1 = 目印のファイル名
  find . \( -path ./.upstream -o -name node_modules -o -path ./.git \) -prune -o -name "$1" -print 2>/dev/null \
    | xargs -r -n1 dirname | sort -u
}

if command -v pnpm >/dev/null 2>&1; then
  for d in $(find_dirs pnpm-lock.yaml); do
    [ -d "$d/node_modules" ] && continue
    if out="$(cd "$d" && run_capped 240 pnpm install --frozen-lockfile 2>&1)"; then
      warn "installed $d/node_modules"
    else
      warn "WARN: pnpm install に失敗しました ($d)。テストは回りません。"
      printf '%s\n' "$out" | tail -5 >&2
    fi
  done
fi

if command -v go >/dev/null 2>&1; then
  for d in $(find_dirs go.mod); do
    out="$(cd "$d" && run_capped 180 go mod download 2>&1)" \
      || { warn "WARN: go mod download に失敗しました ($d)。"; printf '%s\n' "$out" | tail -5 >&2; }
  done
fi
exit 0
