# S3 レビュー（エンジン切り出しと WASM 化、ゴールデンテスト移植）

- 基準コミット: `daec548ca9f2244bc24db68e2a34c8e4a1a7df8e`（S3-report.md 計画節の記録と一致）
- 対象: `claude/stage-3-6rQ0V`。レビュー開始時の HEAD は `36ae878`。レビュー中に `6fce13b docs(S3): 完了報告の下書き`（`docs/progress/S3-report.md` だけ、+127 行）が積まれたので、それも読んだ。
- 読み取り専用で行った。書いたのは本ファイルだけ。重い検査（ネイティブ・WASM・node・本家の正解の再生成）はすべて自分で回し、出力は scratchpad に置いた。リポジトリの作業ツリーは clean のまま（`git status --short` が空）。

## 1. 要約

- 完了条件「Go と WASM の両方でテストが緑」は**満たしている**。手元で回して、ネイティブ `go test ./...` EXIT=0（PASS 329）、WASM の go test EXIT=0（PASS 152）、Go → WASM → JS の `node --test` EXIT=0（5/5）、本家の正解の再生成は `cmp` で**完全に一致**（4,464 条件）。
- 移植の差分は `docs/upstream-diff.md` の表と一致する（公開版のチェックアウトと全ファイルを比べて確かめた）。テストは 116 → 111 本で、減った 5 本は文書に書かれた 5 本と名前まで一致する。`_reference/` は変更されていない。
- §8-3 の壊し方を 3 つ自分で試した（WASM の関数登録を外す・並べ替えの最後のタイブレークを逆にする・親子の並べ直しを外す）。どれも狙いどおり赤になった。適合性テストはよく効いている。
- 高の指摘は 1 件で、コードと食い違うコメント（テスト名の書き間違い。直すのは 1 行）。中は 4 件: WASM の KA の型検査、`ErrInvalidRemain` の誤りの対応付け、完了報告の `git diff --stat` 欠落、CI の本家の正解の再生成 job がまだ一度も最後まで走っていないこと。

## 2. 完了条件への適合

| 項目 | 判定 | 根拠 |
|---|---|---|
| HANDOVER §9 S3「Go と WASM の両方でテストが緑」 | 済 | §5 のコマンド。ネイティブ EXIT=0、WASM の go test EXIT=0、node --test EXIT=0 |
| 計画 1: 本家と同じパスへ移植 | 済 | `internal/service/setplay/`（本家のテストファイル 8 本すべて）・`internal/repository/setplay/repository.go`・`internal/model/move.go`。`upstream-diff.md` 末尾の比較ループを回した結果、差のあるファイルは表の行と一致した（§5） |
| 計画 2: 単体版で変える箇所を最小に・一覧化 | 済 | service.go と公開版の diff を読んだ。変わったのはコンボ・alreadyAdopted・`CharacterCode` の削除、入口、`recipeHash`、コメント 2 行の削除だけ。`recipeHash` の置き換えは本家 `recipehash.calc`（`<move_id>:<modifiers JSON>` を `\n` でつないだ sha256。modifiers は常に null）と比べて、等しさの意味が同じであることを確かめた |
| 計画 2: 移植しないテストの件数と名前を報告と diff 一覧に書く | 済 | `func Test` の数: 本家 116 本、単体版 111 本（filler_only −1・refine −1・service −3）。名前は upstream-diff.md・報告と一致 |
| 計画 3: `engine/`（Load・ParseQuery・Suggest・JSON） | 済 | `go list -deps` の結果、標準ライブラリ以外の依存は自リポジトリのパッケージだけ（cmd/wasm を含む）。`ParseQuery` を本家 `GetSuggestions` と 1 行ずつ比べ、キー・既定・文言・検査の順が一致した |
| 計画 4: WASM（`cmd/wasm`） | 済 | build.sh EXIT=0。`setplay.wasm: 3882385 bytes / gzip -9: 1070774 bytes` |
| 計画 5: 本家を正解にした適合性テスト（生成器・コミット・CI で作り直し） | 一部 | 生成器・突き合わせ・手元での再生成と一致は済。**CI の `upstream-oracle` job は、36ae878 では新しい push に打ち切られて（concurrency による cancelled）一度も完走していない**。6fce13b の実行はレビューを書いている時点で in_progress（中-4） |
| 計画 6(a): 同じテストを WASM で | 済 | PASS 152・FAIL 0。WASM では突き合わせが 5 キャラ 720 条件になる |
| 計画 6(b): 配線の通し（Go → WASM → JS） | 済 | node --test 5/5。suggest の登録を外すと 3 件赤になった（§5） |
| 計画「回す検査」: §8-3 で 3 種を壊して赤を確認 | 済（報告 + 自分で追試） | 報告の表に 7 行ある。自分でも 3 つ試してすべて赤 |
| 完了報告（AGENTS §3・§8-11） | 一部 | 下書きに「やったこと／判断と理由／残課題／次の段階への入力」と逐語の結果がある。`git diff --stat <基準>` が無い（中-3）。テストファイル数の記述に誤り（低-1） |

## 3. 指摘

### 高

**高-1 コメントが存在しないテスト名を指している**
- 場所: `internal/repository/setplay/repository.go:65-66`
  「一致はテスト TestSchemaVersionMatchesMovesDB が見る」
- 実際のテストは `internal/movesdb/engine_compat_test.go:11` の `TestSchemaVersionMatchesEngineReader`。`grep -rn "TestSchemaVersionMatches"` の当たりはこの 2 か所だけ（`.upstream`・`_reference` は除いた）。
- なぜ問題か: 版の一致を担保するテストを探す人がその名前で検索すると見つからない。テストは緑のままで、読み手しか気づけない種類の食い違いである（review.md の「高」の定義）。
- 直し方: コメントを `TestSchemaVersionMatchesEngineReader`（internal/movesdb/engine_compat_test.go）に直す。

### 中

**中-1 WASM の `suggest` が整数でない KA を黙って受け付ける**
- 場所: `cmd/wasm/main.go:35-38`（型の検査が `js.TypeNumber` だけで、`args[1].Int()` で整数にしている）
- 再現: `build.sh` で作った wasm を Node で起動して `suggest("ryu", ka, "limit=1")` を呼んだ（scratchpad の probe.mjs）。
  - `40.9` → KA 40 として結果が返る（誤りにならない）
  - `NaN` → `{"items":[],"truncated":false,"totalFound":0}`（reason も誤りも無い）
  - `-0.5` → KA 0 として扱われる（負の KA の reason は付かない）
- なぜ問題か: 本家の KA は DB の整数列なので、整数でない値は入ってこない。単体版はここが外からの入力の境目になり、「未入力・不正」と「該当なし（0 件）」が同じ形で返ってしまう（AGENTS §8-7）。S5 の画面やローカル版（D10・上限なし）で入力を取り違えても気づけない。
- 直し方: `f := args[1].Float()` を取り、`math.IsNaN(f) || math.IsInf(f, 0) || f != math.Trunc(f)` なら `InvalidArgumentsJSON` を返す。wasm.test.mjs にも `40.5`・`NaN` の検査を足す。

**中-2 `setplay.ErrInvalidRemain` が `internal_error` になる（本家は 400 の invalid_query_parameter）**
- 場所: `engine/api.go:76-82`（`SuggestJSON` の誤りの対応付け）。本家は `.upstream/tacpendium/internal/api/setplay/handler.go` の `case errors.Is(err, setplaysvc.ErrInvalidRemain)` で 400 にしている（本家のテスト `TestHandler_ServiceInvalidRemain_Returns400`）。
- 現状は `ParseQuery` が先に負の remain を弾くので、`SuggestJSON` からはこの経路に入らない。入るのは Go の API `Engine.Suggest` に手で組んだ `Query`（`Mode: filler, Remain: -1`）を渡したときで、そのときは wrap された誤りがそのまま返る。S4 の事前計算 CLI はこの経路を使う。
- なぜ問題か: `docs/engine.md`・`upstream-diff.md` は「本家の API 層と同じ」と書いているが、本家ハンドラにある対応付けのうちこの 1 つだけが移されておらず、一覧にも無い。本家のハンドラのテストは移植しておらず（upstream-diff の「移植しない本家ファイル」に `internal/api/setplay`）、代わりの検査も無い。
- 直し方: `SuggestJSON` で `errors.Is(err, setplay.ErrInvalidRemain)` を `ErrCodeInvalidQuery`（Param `remain`）に対応付け、api_test に 1 件足す。直さない場合は upstream-diff の S3 節に「対応付けを移していない」と書く。

**中-3 完了報告に `git diff --stat <基準>` が無い**
- 場所: `docs/progress/S3-report.md`（6fce13b の下書き）。「全体の diff は末尾」と書いてあるが、末尾にも本文中にも無い（`grep -n "diff --stat\|files changed"` の当たりは 0 件。全 175 行）。
- なぜ問題か: AGENTS §8-11 の必須項目。テストが黙って減っていないことを読み手が確かめる手段になる。
- 直し方: `git diff --stat daec548` を貼る。参考までに 36ae878 時点では `45 files changed, 54322 insertions(+), 6 deletions(-)` で、削除 6 行はすべて既存の設定・docs の書き換え（ci.yml 2・DATA-LICENSE 1・NOTICE 1・REUSE 1・upstream-diff 1）だった。

**中-4 CI の「本家の正解の再生成」job が一度も完走していない**
- 根拠: `gh run view 37868846501`（36ae878）では `本家の正解の再生成: completed cancelled`。手順「正解の再生成: cancelled」「コミット済みと一致: skipped」。原因は 6fce13b の push で concurrency が古い実行を打ち切ったこと。6fce13b の実行 `37869412072` は、本ファイルを書いた時点で Go・WASM・Web・リポジトリ検査が success、正解の再生成だけが in_progress だった。
- 手元では同じ `run.sh` を回して `cmp` が一致した（§5）。ただし CI 上の緑はまだ観測されていない。
- なぜ問題か: 完了報告で「CI が作り直して一致を見る」を緑の根拠にするなら、その実行の結果が要る（AGENTS §8-1・§8-2「回していない検査を過去形で書かない」）。
- 直し方: 37869412072 の完了を待ち、job の結果を報告に書く。報告の push でまた打ち切られないよう、報告の最後のコミットの実行の結果を確かめてから止まる。

### 低

**低-1 報告のテストファイル数の誤り**: `S3-report.md` の「やったこと」に「テスト 9 ファイル」とあるが、`internal/service/setplay/*_test.go` は 8 本（`ls | wc -l` = 8。本家も 8 本）。

**低-2 適合性テストの KA は 8 点だけ**: `oracle_conditions.json` の KA は {-1,0,7,25,43,61,79,100}。S4 は 0〜100 の全 KA を出し、S6 の完了条件は「票なしの並びは本家と一致」なので、本家との一致は S4 で全 KA について見る必要がある。報告の「推測: 全 KA は S4 で回す」は妥当なので、S4 の計画の入力として明示しておくとよい（例: 既定のクエリだけを全 KA で回す条件セットを足す）。

**低-3 移植した本家テストは、並べ替えの最後のタイブレークを逆にしても緑のまま**: `service.go` の `rank` の `a.sortName < b.sortName` を `>` に変えると、`internal/service/setplay` のテストは EXIT=0（FAIL 0）だった。一方 `TestConformance_UpstreamOracle` は「不一致 1198 / 4464」で赤になった。単体版の欠陥ではなく本家のテストの穴で、適合性テストが埋めている。本家のテストを単体版で書き換えない方針なので、記録だけにする（docs/engine.md の検査表に「並べ替えの細部は適合性テストだけが見る」と一言あるとよい）。

**低-4 CI の分数**: push ごとに本家の正解の再生成（手元で 5〜6 分）が走る。報告の残課題に書かれていて、判断を開発者へ回しているので、それでよい。

### 観点別に指摘なしのもの
- 禁止事項（§7）: 指摘なし。差分に秘密情報の形（`ghp_`・`AKIA`・`PRIVATE KEY`・api_key）は 0 件。`_reference/` の差分は 0 行。combomgr への言及は、計画節（_reference との比較の記述）・本家由来のコメント（公開版にも同じ行がある）・check-licenses の既存の行の 3 か所だけで、取込元にしてはいない。
- 文言・i18n: S3 には画面の文言が無いので対象外。誤りの文言は本家ハンドラの日本語をそのまま使い、engine.md に「S5 で i18n のキーに当てる」とある。
- 打ち切りの順: `SuggestForKnockdown` は refine → rank → `totalFound` → limit の順で、本家と同じ（並べ替えの後で切る）。
- 未計測／該当なし: `Response.Items` は 0 件でも `[]`（`TestSuggestJSON` が null でないことを検査している）。`Reason` は本家と同じく omitempty。
- 決定性・依存: engine と cmd/wasm は標準ライブラリだけに依存する。`gofmt -l` は空、`go vet ./...` は EXIT=0、WASM の vet も EXIT=0。
- ライセンス: `check-licenses.py` EXIT=0（違反 0 件）、`check-actions-pinned.sh` は違反 0 件（uses 12 件）。setup-node も SHA で固定されている。

## 4. 良かった点

- **本家の実装そのものを正解にした適合性テスト**。本家のハンドラ → サービス → SQL を本物の DB で回すので、クエリの解釈・誤りの文言・並び・打ち切り・truncated まで 4,464 条件で比べている。手元の再生成が 1 バイトまで一致した。
- 読み口の同一性（候補行・親子関係の並びを含む）を別の検査として置いている。親の並べ直しを外すと赤になることを確かめた（code の順と ID の順が違う技は実データに 84 件あり、検査が実際に効く入力がある）。
- テストの本文を本家のまま残し、フェイクの `testService.SuggestForCombo` で差を吸収しているので、本家の差分をそのまま当てられる。
- `go_js_wasm_exec` の環境変数の上限の問題を、再現したうえでラッパで回避し、理由をスクリプトに書いている。
- 正解の条件ファイルと正解の並びが合わないと赤にする検査（作り直し忘れの検出）が入っている。

## 5. 自分で回したコマンドと結果（逐語）

出力先は `S=/tmp/claude-0/-home-user-tacpendium-setplay/bd5573e4-3ec7-5481-b148-e5f9a03c458c/scratchpad`。

```
$ go test -count=1 ./... > $S/gotest.txt 2>&1; echo EXIT=$?
ok  	github.com/plexiblinp/tacpendium-setplay/cmd/movesdb	0.745s
ok  	github.com/plexiblinp/tacpendium-setplay/engine	288.119s
ok  	github.com/plexiblinp/tacpendium-setplay/internal/gameversion	0.014s
?   	github.com/plexiblinp/tacpendium-setplay/internal/model	[no test files]
ok  	github.com/plexiblinp/tacpendium-setplay/internal/moveindex	0.003s
ok  	github.com/plexiblinp/tacpendium-setplay/internal/movesdb	0.923s
ok  	github.com/plexiblinp/tacpendium-setplay/internal/repository/setplay	0.005s
ok  	github.com/plexiblinp/tacpendium-setplay/internal/sanumber	0.002s
ok  	github.com/plexiblinp/tacpendium-setplay/internal/seedgen	1.444s
ok  	github.com/plexiblinp/tacpendium-setplay/internal/service/setplay	1.609s
EXIT=0

$ go test -count=1 -v ./... > $S/gotest_v.txt 2>&1; echo EXIT=$?
EXIT=0
PASS=329 FAIL=0 SKIP=0   （grep -cE '^\s*--- PASS|FAIL|SKIP'）
    conformance_test.go:265: 突き合わせた条件: 4464(キャラ 31 ・不一致 0)

$ GOOS=js GOARCH=wasm go test -count=1 -v -timeout 30m -exec="$PWD/scripts/wasm/go-test-exec.sh" \
    ./internal/service/setplay/ ./internal/repository/setplay/ ./engine/ > $S/wasmtest.txt 2>&1; echo EXIT=$?
ok  	github.com/plexiblinp/tacpendium-setplay/internal/service/setplay	4.218s
ok  	github.com/plexiblinp/tacpendium-setplay/internal/repository/setplay	1.110s
    conformance_test.go:265: 突き合わせた条件: 720(キャラ 5 ・不一致 0)
ok  	github.com/plexiblinp/tacpendium-setplay/engine	190.623s
EXIT=0
PASS 152 / FAIL 0 / SKIP 0

$ bash scripts/wasm/build.sh $S/wasm; echo EXIT=$?
EXIT=0
setplay.wasm: 3882385 bytes / gzip -9: 1070774 bytes

$ WASM_DIR=$S/wasm node --test scripts/wasm/wasm.test.mjs > $S/node.txt 2>&1; echo EXIT=$?
EXIT=0
# WASM 経由で突き合わせた条件: 432(不一致 0)
# tests 5
# pass 5
# fail 0

$ bash scripts/upstream-oracle/run.sh $S/oracle_regen.json; echo EXIT=$?; cmp engine/testdata/upstream_oracle.json $S/oracle_regen.json; echo CMP_EXIT=$?
    zz_standalone_oracle_test.go:227: cells: 4464
ok  	github.com/plexiblinp/tacpendium/internal/api/setplay	356.969s
EXIT=0
CMP_EXIT=0
（作業後も git status --short は空、.upstream/tacpendium の status --porcelain も空）

$ go vet ./... ; echo EXIT=$?                                  → EXIT=0
$ GOOS=js GOARCH=wasm go vet ./engine/ ./cmd/wasm/ ./internal/service/setplay/ ./internal/repository/setplay/ → EXIT=0
$ gofmt -l engine cmd internal scripts                         → 出力なし
$ python3 scripts/check-licenses.py; echo EXIT=$?              → EXIT=0 / 違反: 0 件
$ bash scripts/check-actions-pinned.sh                         → workflow: 2 本 / uses 参照: 12 件 / 違反: 0 件
```

upstream-diff の比較ループ（docs/upstream-diff.md 末尾。公開版 `.upstream/tacpendium` は lock と同じ `ffdb69e4…`）:
```
変更あり: internal/seedgen/generate_m2002.go
変更あり: internal/seedgen/generate_test.go
単体版のみ: internal/seedgen/standalone.go
変更あり: internal/model/doc.go
変更あり: internal/model/move.go
変更あり: internal/repository/setplay/repository.go
単体版のみ: internal/repository/setplay/repository_test.go
変更あり: internal/service/setplay/filler_only_test.go
変更あり: internal/service/setplay/refine_m4705_test.go
変更あり: internal/service/setplay/refine_test.go
変更あり: internal/service/setplay/service.go
変更あり: internal/service/setplay/service_test.go
```
→ upstream-diff.md の表（S2・S3）と一致する。`_reference/combomgr/internal/service/setplay` と公開版の違いは `service.go`（withoutParentsHash / markAlreadyAdopted）と `service_m4305_test.go` だけで、計画節の記述と一致する。

§8-3 の追試（`git archive HEAD` を scratchpad の `mut/` に展開した写しの上で壊した。リポジトリは変えていない）:

| 壊し方 | 結果 |
|---|---|
| `cmd/wasm/main.go` の `api.Set("suggest", …)` を外す | node --test EXIT=1、`# pass 2 / # fail 3` |
| `service.go` の `rank` の最後のタイブレーク `a.sortName < b.sortName` → `>` | `TestConformance_UpstreamOracle` EXIT=1、「突き合わせた条件: 4464(キャラ 31 ・不一致 1198)」。`internal/service/setplay` のテストは EXIT=0（低-3） |
| `repository.go` の親 ID の `sort.Slice` を外す | `TestConformance_CandidatesMatchUpstreamRepository` EXIT=1（ken・c_viper・dhalsim ほか「本家の読み口と違う」） |

WASM の KA の入力の確かめ（中-1）: scratchpad の `js/probe.mjs` で `suggest("ryu", ka, "limit=1")` を呼んだ。`40` と `40.9` は同じ結果、`NaN` と `-0.5` は `{"items":[],"truncated":false,"totalFound":0}`。

CI（`gh run view`）: 36ae878 の実行 37868846501 は cancelled（Go・WASM・Web・リポジトリ検査は success、本家の正解の再生成は cancelled）。6fce13b の実行 37869412072 は、本ファイルを書いた時点で正解の再生成以外が success、正解の再生成は in_progress。

---

## 取り込み（実装担当）

| 指摘 | 採否 | 対応・理由 |
|---|---|---|
| 高-1 存在しないテスト名をコメントが指す | 採用 | `repository.go` のコメントを `internal/movesdb の TestSchemaVersionMatchesEngineReader` に直した |
| 中-1 WASM の KA を数値かしか見ない | 採用 | `engine.KnockdownFromFloat`（NaN・±Infinity・小数・int32 範囲外を拒否）を足し、`cmd/wasm` は `invalid_arguments` を返す。Go の単体テストと node のテスト（40.9・-0.5・NaN・Infinity）を追加。§8-3: 小数の判定を外す → `TestKnockdownFromFloat` 赤、WASM を `args[1].Int()` に戻す → node 1 件赤 |
| 中-2 `ErrInvalidRemain` が internal_error | 採用 | `apiErrorOf` で本家ハンドラと同じ `invalid_query_parameter`（param `remain`）へ写す。`TestApiErrorOf_InvalidRemain` を追加（写しを外す → 赤）。`docs/upstream-diff.md` に誤りの写し方を追記 |
| 中-3 `git diff --stat` が無い | 採用 | 完了報告の末尾に貼る（最終確認節） |
| 中-4 正解の再生成 job が未完走 | 採用 | 取り込み後の push の CI で完走と結果を確かめ、完了報告に書く |
| 低: テスト 9 ファイル → 8 | 採用 | 報告を直した |
| 低: 全 KA の一致は S4 で | 採用 | 報告「次の段階への入力」に明記 |
| 低: 本家テストのタイブレークの穴 | 採用（記録のみ） | 報告の残課題に記録。本家追従の方針なので単体版ではテストを足さない（本家との突き合わせが捕まえる） |
| 低: CI 時間 | 既に残課題 | 開発者判断（変更なし） |
