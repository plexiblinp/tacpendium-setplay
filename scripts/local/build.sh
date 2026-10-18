#!/usr/bin/env bash
# ローカル版(HANDOVER D10・§5)の単一バイナリを作る。
#   1. サイトのデータと WASM を web/public/ に用意する(web/scripts/prepare-assets.sh。SKIP_ASSETS=1 なら今あるものを使う)
#   2. フロントをローカル版でビルドする(vite build --mode standalone → web/dist-local。KA の上限なし・通信なし)
#   3. web/dist-local を cmd/local/site/ に置き(埋め込みの中身)、go build する
# 使い方: bash scripts/local/build.sh [置き場(既定: build/local)] [GOOS/GOARCH ...(既定: 手元の OS)]
#   例: bash scripts/local/build.sh build/local linux/amd64 darwin/arm64 windows/amd64
# 出力: <置き場>/tacpendium-setplay-local_<os>_<arch>[.exe]。版は VERSION(既定: git describe)。
# ★公開物(Releases)は CI(.github/workflows/local-release.yml)からだけ出す(AGENTS §8-12)。
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
OUT="${1:-$ROOT/build/local}"
shift || true
TARGETS=("$@")
if [ "${#TARGETS[@]}" -eq 0 ]; then TARGETS=("$(go env GOOS)/$(go env GOARCH)"); fi
VERSION="${VERSION:-$(git -C "$ROOT" describe --tags --always --dirty 2>/dev/null || echo dev)}"
SITE="$ROOT/cmd/local/site"
NAME="tacpendium-setplay-local"

if [ "${SKIP_ASSETS:-}" != "1" ]; then
  bash "$ROOT/web/scripts/prepare-assets.sh"
fi
for f in "$ROOT/web/public/data/manifest.json" "$ROOT/web/public/wasm/setplay.wasm"; do
  [ -f "$f" ] || { echo "ERROR: $f がありません(web/scripts/prepare-assets.sh を回す)" >&2; exit 1; }
done
(cd "$ROOT/web" && pnpm exec tsc --noEmit && pnpm exec vite build --mode standalone --outDir dist-local --emptyOutDir)

# 埋め込みの中身を入れ替える(プレースホルダ .gitkeep は残す)。
find "$SITE" -mindepth 1 ! -name .gitkeep -exec rm -rf {} +
cp -R "$ROOT/web/dist-local/." "$SITE/"
# config.json はサイトの計測の設定(ローカル版は読まない)。配らない。
rm -f "$SITE/config.json"
[ -f "$SITE/index.html" ] && [ -f "$SITE/_headers" ] || { echo "ERROR: $SITE に index.html と _headers が要る" >&2; exit 1; }

mkdir -p "$OUT"
for t in "${TARGETS[@]}"; do
  os="${t%/*}"; arch="${t#*/}"
  ext=""; [ "$os" = "windows" ] && ext=".exe"
  bin="$OUT/${NAME}_${os}_${arch}${ext}"
  (cd "$ROOT" && CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -trimpath -ldflags="-s -w -X main.version=$VERSION" -o "$bin" ./cmd/local)
  echo "$(basename "$bin"): $(wc -c < "$bin") bytes"
done
echo "版: $VERSION"
