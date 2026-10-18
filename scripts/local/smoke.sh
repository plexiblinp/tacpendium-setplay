#!/usr/bin/env bash
# ローカル版のバイナリの起動確認(3 OS の CI で回す。.github/workflows/local-release.yml)。
# 実バイナリを起動し、画面・データ・WASM が正しいヘッダで配られることを HTTP で確かめる(画面の操作は E2E が Linux で見る)。
# 使い方: bash scripts/local/smoke.sh <バイナリ> [ポート(既定 18787)]
# 終了コード: 0 = 通った / 1 = 失敗
set -uo pipefail
BIN="${1:?バイナリを渡す}"
PORT="${2:-18787}"
BASE="http://127.0.0.1:$PORT"
WORK="$(mktemp -d)"
fails=0
check() { # check <説明> <条件の結果(0/1)>
  if [ "$2" -eq 0 ]; then echo "ok   $1"; else echo "FAIL $1"; fails=$((fails + 1)); fi
}
header() { # header <ファイル> <名前> → 値(改行・CR を落とす)
  grep -i "^$2:" "$1" | head -n 1 | sed -E 's/^[^:]+:[[:space:]]*//' | tr -d '\r\n'
}
fetch() { # fetch <パス> <名前> [curl の引数...] → $WORK/<名前>.h(ヘッダ)・.b(本文)。標準出力に状態コード
  local p="$1" n="$2"; shift 2
  curl -s -D "$WORK/$n.h" -o "$WORK/$n.b" -w '%{http_code}' "$@" "$BASE$p"
}

ver="$("$BIN" -version)"
check "-version が版を出す($ver)" "$([ -n "$ver" ]; echo $?)"

"$BIN" -open=false -port "$PORT" > "$WORK/server.log" 2>&1 &
pid=$!
up=1
for _ in $(seq 1 100); do
  if curl -s -o /dev/null "$BASE/"; then up=0; break; fi
  sleep 0.1
done
check "起動して $BASE/ に応答する" "$up"

code="$(fetch / index)"
check "/ が 200 で HTML" "$([ "$code" = 200 ] && header "$WORK/index.h" Content-Type | grep -q '^text/html'; echo $?)"
csp="$(header "$WORK/index.h" Content-Security-Policy)"
check "/ に CSP(wasm-unsafe-eval・connect-src 'self')" "$(printf '%s' "$csp" | grep -q "'wasm-unsafe-eval'" && printf '%s' "$csp" | grep -q "connect-src 'self'"; echo $?)"
check "/ に nosniff" "$([ "$(header "$WORK/index.h" X-Content-Type-Options)" = nosniff ]; echo $?)"

code="$(fetch /wasm/setplay.wasm wasm)"
size="$(wc -c < "$WORK/wasm.b" | tr -d ' ')"
check "WASM が 200・application/wasm・1 MB 超($size bytes)" "$([ "$code" = 200 ] && [ "$(header "$WORK/wasm.h" Content-Type)" = application/wasm ] && [ "$size" -gt 1000000 ]; echo $?)"
code="$(fetch /wasm/wasm_exec.js wasmexec)"
check "wasm_exec.js が text/javascript" "$([ "$code" = 200 ] && header "$WORK/wasmexec.h" Content-Type | grep -q '^text/javascript'; echo $?)"

code="$(fetch /data/manifest.json manifest)"
chars="$(grep -o '"moves": *"[^"]*"' "$WORK/manifest.b" | wc -l | tr -d ' ')"
check "manifest が 200 で キャラ $chars 人(1 人以上)" "$([ "$code" = 200 ] && [ "$chars" -gt 0 ]; echo $?)"
first="$(grep -o '"moves": *"[^"]*"' "$WORK/manifest.b" | head -n 1 | sed -E 's/.*"([^"]*)"$/\1/')"
code="$(fetch "/data/$first" moves)"
check "moves($first)が 200 で JSON" "$([ "$code" = 200 ] && header "$WORK/moves.h" Content-Type | grep -q '^application/json'; echo $?)"

# index.html が読むスクリプトがすべて配られる(埋め込みの漏れ)。
missing=0
for src in $(grep -oE '(src|href)="/[^"]+"' "$WORK/index.b" | sed -E 's/^[a-z]+="//; s/"$//'); do
  c="$(curl -s -o /dev/null -w '%{http_code}' "$BASE$src")"
  if [ "$c" != 200 ]; then echo "     $src → $c"; missing=$((missing + 1)); fi
done
check "index.html が読むファイルがすべて 200" "$missing"

check "_headers は配らない(404)" "$([ "$(fetch /_headers h)" = 404 ]; echo $?)"
check "別の Host は 403(DNS rebinding)" "$([ "$(fetch / evil -H "Host: evil.example:$PORT")" = 403 ]; echo $?)"

kill "$pid" 2>/dev/null || true
wait "$pid" 2>/dev/null || true
echo "--- server.log"
cat "$WORK/server.log"
echo "失敗: $fails 件"
[ "$fails" -eq 0 ]
