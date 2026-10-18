#!/usr/bin/env bash
# GOOS=js GOARCH=wasm の go test を Node で回すための -exec ラッパ。
# 使い方: GOOS=js GOARCH=wasm go test -exec="$PWD/scripts/wasm/go-test-exec.sh" ./engine/ ...
#
# Go 付属の go_js_wasm_exec は環境変数をすべて WASM へ渡すが、wasm_exec.js は引数と環境変数の合計を
# 12 KB 程度に制限している(超えると "total length of command line and environment variables exceeds limit")。
# CI や開発環境の環境変数はこれを超えうるので、PATH と HOME だけを残して起動する。
#
# GOMEMLIMIT: WASM(wasm32)のメモリは 4 GiB が上限で、Go は一度広げたメモリを返せない。S4 で engine の試験が
# out of memory(4 GiB 使用中)で落ちた。gctrace で見ると Go のヒープは最大 1.2 GB(生きている量は最大 885 MB)で、
# 上限まで広がったのは大きなスライスが置ける連続領域を再利用できない断片化による(GC は動かさない)。
# 柔らかい上限 1500 MiB でヒープを小さく保つと落ちにくくなる(3000 MiB では CI ・手元とも落ちた)が、1500 MiB でも
# 手元で 1 回落ちたので、これだけでは足りない。engine の本家突き合わせは WASM では KA 79 以下に絞った
# (engine/conformance_test.go の wasmConformanceMaxKA)。
# 外から GOMEMLIMIT が渡されればそれを使う。
set -euo pipefail
EXEC="$(go env GOROOT)/lib/wasm/go_js_wasm_exec"
exec env -i PATH="$PATH" HOME="${HOME:-/tmp}" GOMEMLIMIT="${GOMEMLIMIT:-1500MiB}" "$EXEC" "$@"
