#!/usr/bin/env bash
# 規模・複雑度の定点観測の機械部(/audit_scale から呼ぶ。手順は docs/maintenance.md「health」、系列は docs/audits/scale-baseline.md)。
# 公開版 tacpendium の scripts/observe-scale-scan.sh を参考に、単体版で測るものだけに絞って作り直した。
#
# 使い方: bash scripts/observe-scale.sh [<commit>]
#   <commit> を渡すと、その木を git worktree で一時ディレクトリに出して測る(作業ツリーには触らない)。省略時は作業ツリー。
#   環境変数: OBSERVE_OUT=<dir>(生データの保存先。既定は mktemp -d)
#   要るもの: gocyclo・scc(リポジトリ外の $(go env GOPATH)/bin。入れ方は docs/maintenance.md「health」)
# 出力: 節ごとの生データと、末尾の「== COUNTS ==」(key=value。系列表と突き合わせる値)。
#   COUNTS の -1 は「測れなかった」(道具が無い等)。0 件と混ぜない(AGENTS §8-7)。
#   ソースだけを読み、ビルド・テストは回さない(過去のコミットも同じ物差しで測るため)。
# 終了コード: 測れた・測れなかったに関わらず 0(判定は系列表との突き合わせで行う)。引数の誤りだけ 2。
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT="${OBSERVE_OUT:-$(mktemp -d)}"
mkdir -p "$OUT"
GOBIN="$(go env GOPATH 2>/dev/null)/bin"
PATH="$GOBIN:$PATH"

# 複雑度の閾値(gocyclo)。15 超え・20 超えの数を系列に置く。
CYCLO_WARN=15
CYCLO_HIGH=20
# 上位の表に出す数。
TOP_N=20
# 測らない場所: 取得した上流・依存・ビルド成果物・生成物(データ・配信用の資産)。
EXCLUDE_RE='(^|/)(\.upstream|node_modules|dist|dist-edge|build|\.git|test-results|playwright-report)/|^web/public/'
SCC_EXCLUDE='.upstream,node_modules,dist,dist-edge,build,.git,test-results,playwright-report,public'

TARGET="$ROOT"
WT=""
if [ $# -ge 1 ]; then
  if ! git -C "$ROOT" rev-parse --verify --quiet "$1^{commit}" >/dev/null; then
    echo "commit が無い: $1" >&2
    exit 2
  fi
  WT="$(mktemp -d)"
  git -C "$ROOT" worktree add --detach --quiet "$WT" "$1"
  trap 'git -C "$ROOT" worktree remove --force "$WT"' EXIT
  TARGET="$WT"
fi
cd "$TARGET"

declare -a KEYS=()
declare -A C=()
put() { KEYS+=("$1"); C[$1]="$2"; }
section() { printf '\n== %s ==\n' "$1"; }
# 一覧(1 行 1 件)の行数。0 件も 0 と数える。
lines() { grep -c . "$1" || true; }

section "対象"
put commit "$(git rev-parse --short HEAD)"
put commit_date "$(git log -1 --format=%cs)"
put dirty "$([ -z "$(git status --porcelain)" ] && echo 0 || echo 1)"
put gocyclo_version "$(command -v gocyclo >/dev/null && go version -m "$(command -v gocyclo)" 2>/dev/null | awk '$1=="mod"{print $3}' || echo -1)"
put scc_version "$(command -v scc >/dev/null && scc --version | awk '{print $NF}' || echo -1)"
printf '%s\n' "commit=${C[commit]} (${C[commit_date]}) dirty=${C[dirty]}"

# git が管理するファイルだけを対象にする(未追跡のゴミを数えない)。
git ls-files | grep -Ev "$EXCLUDE_RE" > "$OUT/files.txt" || true
grep -E '\.go$' "$OUT/files.txt" | grep -Ev '_test\.go$' > "$OUT/go-prod.txt" || true
grep -E '_test\.go$' "$OUT/files.txt" > "$OUT/go-test.txt" || true
grep -E '\.(ts|tsx|mjs|js)$' "$OUT/files.txt" | grep -Ev '\.(test|spec)\.(ts|tsx|mjs|js)$|\.d\.ts$' > "$OUT/web-prod.txt" || true
grep -E '\.(test|spec)\.(ts|tsx|mjs|js)$' "$OUT/files.txt" > "$OUT/web-test.txt" || true

section "行数(prod / test。空行を含む wc -l)"
sumlines() { if [ -s "$1" ]; then tr '\n' '\0' < "$1" | xargs -0 cat | wc -l; else echo 0; fi; }
put go_prod_files "$(lines "$OUT/go-prod.txt")"
put go_prod_lines "$(sumlines "$OUT/go-prod.txt")"
put go_test_lines "$(sumlines "$OUT/go-test.txt")"
put web_prod_files "$(lines "$OUT/web-prod.txt")"
put web_prod_lines "$(sumlines "$OUT/web-prod.txt")"
put web_test_lines "$(sumlines "$OUT/web-test.txt")"
for k in go_prod_files go_prod_lines go_test_lines web_prod_files web_prod_lines web_test_lines; do echo "$k=${C[$k]}"; done

section "Go の複雑度(gocyclo。テストを除く)"
if command -v gocyclo >/dev/null && [ -s "$OUT/go-prod.txt" ]; then
  tr '\n' '\0' < "$OUT/go-prod.txt" | xargs -0 gocyclo > "$OUT/cyclo-all.txt"
  sort -k1,1nr -k4 "$OUT/cyclo-all.txt" > "$OUT/cyclo-sorted.txt"
  put cyclo_funcs "$(lines "$OUT/cyclo-sorted.txt")"
  put cyclo_avg "$(awk '{s+=$1; n++} END {if (n) printf "%.2f", s/n; else print -1}' "$OUT/cyclo-sorted.txt")"
  put cyclo_max "$(awk 'NR==1{print $1}' "$OUT/cyclo-sorted.txt")"
  put cyclo_over_warn "$(awk -v t="$CYCLO_WARN" '$1>t' "$OUT/cyclo-sorted.txt" | grep -c . || true)"
  put cyclo_over_high "$(awk -v t="$CYCLO_HIGH" '$1>t' "$OUT/cyclo-sorted.txt" | grep -c . || true)"
  # 上位。4 列目は「上流由来か」(同じパスのファイルが取得済みの公開版 tacpendium にあるか。未取得なら ?)。
  echo "複雑度 パッケージ 関数 位置 上流由来"
  awk -v n="$TOP_N" 'NR<=n' "$OUT/cyclo-sorted.txt" | while read -r score pkg fn pos; do
    file="${pos%%:*}"
    if [ -d "$ROOT/.upstream/tacpendium" ]; then
      [ -f "$ROOT/.upstream/tacpendium/$file" ] && up=yes || up=no
    else
      up="?"
    fi
    echo "$score $pkg $fn $pos $up"
  done | tee "$OUT/cyclo-top.txt"
else
  echo "SKIP: gocyclo が無い。cyclo 系は -1"
  for k in cyclo_funcs cyclo_avg cyclo_max cyclo_over_warn cyclo_over_high; do put "$k" -1; done
fi

section "scc(言語別の Code と Complexity。上の除外を同じに適用)"
if command -v scc >/dev/null; then
  scc --no-cocomo --exclude-dir "$SCC_EXCLUDE" . | tee "$OUT/scc.txt"
  scc --by-file -f csv --exclude-dir "$SCC_EXCLUDE" . > "$OUT/scc.csv"
  # scc.csv の列: Language,Provider,Filename,Lines,Code,Comments,Blanks,Complexity,Bytes,ULOC
  for lang in Go TypeScript JavaScript; do
    key="$(echo "$lang" | tr '[:upper:]' '[:lower:]')"
    put "scc_${key}_code" "$(awk -F, -v l="$lang" '$1==l{s+=$5} END{print s+0}' "$OUT/scc.csv")"
    put "scc_${key}_complexity" "$(awk -F, -v l="$lang" '$1==l{s+=$8} END{print s+0}' "$OUT/scc.csv")"
  done
  echo "複雑度の高い TS/JS のファイル(テストを除く。上位 10)"
  awk -F, '($1=="TypeScript"||$1=="JavaScript") && $2 !~ /\.(test|spec)\./ {print $8, $5, $2}' "$OUT/scc.csv" \
    | sort -k1,1nr -k3 | awk 'NR<=10' | tee "$OUT/ts-top.txt"
else
  echo "SKIP: scc が無い。scc 系は -1"
  for k in scc_go_code scc_go_complexity scc_typescript_code scc_typescript_complexity scc_javascript_code scc_javascript_complexity; do put "$k" -1; done
fi

section "試験の数(静的に数えた関数・ケースの数。実行はしない)"
countre() { if [ -s "$1" ]; then tr '\n' '\0' < "$1" | xargs -0 grep -hE "$2" | grep -c . || true; else echo 0; fi; }
put go_test_funcs "$(countre "$OUT/go-test.txt" '^func (Test|Fuzz)[A-Z_]')"
grep -E '^web/e2e/' "$OUT/web-test.txt" > "$OUT/e2e.txt" || true
grep -Ev '^web/e2e/' "$OUT/web-test.txt" > "$OUT/unit.txt" || true
put web_unit_cases "$(countre "$OUT/unit.txt" '^\s*(it|test)(\.each\([^)]*\))?\(')"
put web_e2e_cases "$(countre "$OUT/e2e.txt" '^\s*test(\.each\([^)]*\))?\(')"
for k in go_test_funcs web_unit_cases web_e2e_cases; do echo "$k=${C[$k]}"; done

section "COUNTS"
for k in "${KEYS[@]}"; do echo "$k=${C[$k]}"; done
echo "raw=$OUT"
