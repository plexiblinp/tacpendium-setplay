#!/usr/bin/env bash
# govulncheck(Go の既知の脆弱性の検査)を、配るものすべてに回す(S8。CI の Go job と .github/workflows/govulncheck.yml)。
#   ./...                          … 手元の OS(CI は linux/amd64)で全パッケージ
#   GOOS=js GOARCH=wasm ./cmd/wasm/ … ブラウザで動くエンジン(サイトとローカル版)
#   ./cmd/local/ を windows・darwin・linux で … ローカル版の単一バイナリ(OS ごとに標準ライブラリの経路が違う。S7 報告の GO-2026-6604 は Windows の os.Root)
# 呼ばれている脆弱な関数が 1 つでもあれば赤(govulncheck の終了コード 3)。全部の対象を回してから結果を返す。
# 版は固定(GOVULNCHECK_VERSION)。脆弱性 DB は既定の vuln.go.dev。手元の proxy で読めないときは
#   GOVULNCHECK_DB=https://storage.googleapis.com/go-vulndb を渡す(S7 報告)。
# 使い方: bash scripts/govulncheck.sh
# 終了コード: 0 = 検出なし / 1 = 検出あり・実行エラー
set -uo pipefail
GOVULNCHECK_VERSION="v1.9.0"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TOOLS="$(mktemp -d)"
trap 'rm -rf "$TOOLS"' EXIT

# 道具は手元の OS 向けに作る(GOOS=js のまま go run すると道具そのものが js 向けになる)。
if ! (cd "$ROOT" && GOOS= GOARCH= GOBIN="$TOOLS" go install "golang.org/x/vuln/cmd/govulncheck@$GOVULNCHECK_VERSION"); then
  echo "ERROR: govulncheck $GOVULNCHECK_VERSION を入れられない"
  echo "違反: 読めなかった"
  exit 1
fi
db=()
if [ -n "${GOVULNCHECK_DB:-}" ]; then db=(-db "$GOVULNCHECK_DB"); fi
echo "govulncheck $GOVULNCHECK_VERSION / $(go version)"

failed=0
total=0
run() { # run <説明> <GOOS> <GOARCH> <パターン>
  local label="$1" goos="$2" goarch="$3" pattern="$4"
  total=$((total + 1))
  echo "=== $label(GOOS=$goos GOARCH=$goarch $pattern)"
  if (cd "$ROOT" && GOOS="$goos" GOARCH="$goarch" "$TOOLS/govulncheck" "${db[@]}" "$pattern"); then
    echo "--- ok: $label"
  else
    echo "--- 検出または誤り: $label"
    failed=$((failed + 1))
  fi
}
run "全パッケージ" "$(go env GOHOSTOS)" "$(go env GOHOSTARCH)" ./...
run "WASM" js wasm ./cmd/wasm/
run "ローカル版 windows" windows amd64 ./cmd/local/
run "ローカル版 darwin" darwin arm64 ./cmd/local/
run "ローカル版 linux" linux amd64 ./cmd/local/

echo "対象: $total 件"
echo "違反: $failed 件"
[ "$failed" -eq 0 ]
