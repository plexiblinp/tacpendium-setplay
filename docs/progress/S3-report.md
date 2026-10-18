# S3 エンジン切り出しと WASM 化 — 完了報告

- 段階開始コミット: `daec548ca9f2244bc24db68e2a34c8e4a1a7df8e`
- ブランチ: `claude/stage-3-6rQ0V`（セッション開始時は detached HEAD だったため作成）

## 計画

### 完了条件（HANDOVER §9 S3）
「エンジン切り出しと WASM 化、ゴールデンテスト移植」。**完了条件: Go と WASM の両方でテストが緑**。

### 前提（読んだもの）
- S1 後の開発者決定 2: **本家を正とし追従する。本家のファイル構成・関数名に合わせて移植し、単体版で変える箇所は一覧にする。本家のテストは移植する。**
- S2 報告「次の段階への入力」: moves DB は `data/moves/<code>.json`（技は本家 moves.id 昇順、`parents` は code 昇順、数値 id は無い）。エンジンは `internal/movesdb`（sqlite 依存）を import しない。`approach.go`・`rapid.go` はコードのまま移植。
- 移植元は公開版 tacpendium の lock SHA（`ffdb69e4…`）。`_reference/combomgr/internal/service/setplay/` と比べて違うのは `service.go` の alreadyAdopted 周り（M43-05 の withoutParentsHash）だけで、単体版ではどちらにしても消す部分。`service_m4305_test.go` は公開版に無い（同じく alreadyAdopted の試験）ので移植しない。

### 方針（自分で決めたもの）
1. **本家と同じパスへ移植**: `internal/service/setplay/`（setplay.go・service.go・refine.go・rapid.go・approach.go・cancel.go とテスト全部）、`internal/repository/setplay/repository.go`（型＋読み口）、`internal/model/move.go`（使う定数だけ）。
2. **単体版で変える箇所（最小）**:
   - `service.go`: コンボ・alreadyAdopted を除く。入口を `SuggestForCombo(ctx, comboID, params)` → `SuggestForKnockdown(ctx, characterCode, ka, params)` に。`ComboReader`・`DuplicateChecker`・`ErrComboNotFound`・`ErrKnockdownNotSet`・`Proposal.AlreadyAdopted` を削除。`MoveCandidateReader` は `characterID int64` → `characterCode string`、`CharacterCode` は不要になるので削除。
   - `recipeHash`: 本家は `service/setup` → `internal/recipehash`（sha256・model.Modifiers）へ依存する。単体版での用途は重複の判定（同じ move ID 列か）だけなので、ID 列を連結した鍵を同じフィールドに入れる（等しさの意味は同じ）。
   - `repository.go`: SQL を外し、moves DB（JSON）を読む実装に置き換える。数値 ID は配列の位置 + 1（並びは本家 id 昇順なので順序の意味は保たれる）。
   - テスト: 本文はそのまま。ヘルパ（`service_test.go` の fake）だけを単体版の入口に合わせる。**alreadyAdopted・コンボ未発見・KA 未入力の試験は、機能ごと無いので移植しない**（件数と名前を報告と `docs/upstream-diff.md` に書く）。
3. **`engine/`（新規・単体版だけ）**: AGENTS §4 の「エンジン（engine/）」に当たる公開の入口。本家の API 層（`internal/api/setplay` の handler・dto）に当たる部分を標準ライブラリだけで持つ: `Load`（moves DB の JSON → キャラ）、`ParseQuery`（本家ハンドラと同じクエリ名・既定・誤り）、`Suggest`（結果を本家 DTO と同じ形の JSON に）。`target_move_id` は数値 id が無いので `target_move`（code）にする。
4. **WASM**: `cmd/wasm`（`GOOS=js GOARCH=wasm`）。JS から `load(code, json)`・`suggest(code, ka, query)` を呼べる。中断は Worker ごと止める（HANDOVER §2.1。S5）。TinyGo は入れない（推測: HANDOVER の名前のある依存でなく、まず標準 Go のサイズを計測して報告する）。
5. **適合性テスト（本家を正解に）**: 公開版のチェックアウトの中で本家の `SuggestForCombo` を本物の DB（migrations）で回し、条件ごとの結果の要約（件数・truncated・行の正準形の sha256）を `engine/testdata/upstream_oracle.json` に書く生成器を置く（`scripts/upstream-oracle/`）。単体版は同じ条件を data/moves で回して全件突き合わせる。生成物はコミットし、CI で作り直して一致を見る（S2 の data/moves と同じ扱い）。
6. **WASM 経路の検査**:
   - (a) 移植したテストと engine のテストを `GOOS=js GOARCH=wasm go test -exec go_js_wasm_exec` で回す（同じテストを WASM で）。
   - (b) 配線の通し: `cmd/wasm` をビルドし、Node から wasm_exec.js で読み、同じ oracle と突き合わせる（Go → WASM → JS）。`node --test`（依存なし）。

### 作るファイル
- `internal/model/move.go`・`internal/repository/setplay/repository.go`・`internal/service/setplay/*.go`（テスト込み）
- `engine/engine.go`・`engine/query.go`・`engine/*_test.go`・`engine/testdata/upstream_oracle.json`
- `scripts/upstream-oracle/`（本家側で回す生成器と実行スクリプト）
- `cmd/wasm/main.go`・`web-wasm-test/`（推測: 置き場は `scripts/wasm/` とし、Node の検査 1 本）
- `.github/workflows/ci.yml`（oracle の再生成と一致・WASM テスト・wasm の通し・サイズの summary）
- `docs/upstream-diff.md` 追記・`upstream/watch.sh`（`internal/api/setplay` を見張りに足す）・`docs/engine.md`（入口の使い方）

### 回す検査
- `go test -count=1 ./...`（移植テスト・engine・oracle 突合）
- `GOOS=js GOARCH=wasm go test -count=1 -exec <go_js_wasm_exec> ./internal/service/setplay/ ./engine/`
- `node --test`（wasm の通し）
- oracle の作り直し → `git diff --exit-code`
- §8-3: 移植テスト（費用規則を壊す）・oracle 突合（並べ替えのキーを 1 つ変える）・wasm の通し（関数の登録を 1 行消す）がそれぞれ赤になることを確かめる。

### 未決事項と扱い
- 本家ハンドラの数値 id（`target_move_id`）・`moveId` は単体版に無い → code で置き換える（自分で決める。報告に書く）。
- KA の上限 100（D10）はエンジンでは掛けない（ローカル版は無制限＝D10）。サイト側の上限は S4/S5 で掛ける。定数だけ engine に置く（推測）。
- §3「止めて聞くもの」に当たるものは今のところ無い。

---

## やったこと

成果物（基準 `daec548` からの追加。全体の diff は末尾）:

- **移植**（本家と同じパス・名前。変更点は `docs/upstream-diff.md` の「S3」節）: `internal/service/setplay/`（setplay.go・service.go・refine.go・rapid.go・approach.go・cancel.go とテスト 8 ファイル）、`internal/repository/setplay/repository.go`（型はそのまま、読み口を SQL → moves DB の JSON に）、`internal/model/move.go`（計算が使う定数だけ）。
  - service.go の変更: 入口を `SuggestForKnockdown(ctx, characterCode, ka, params)` に、コンボ・alreadyAdopted・`CharacterCode` を削除、`recipeHash` を move ID 列の連結に。
  - テスト: 本文は本家のまま。フェイク（`service_test.go`）だけを差し替え。**移植しなかったテスト 5 本**（`TestService_KnockdownNull_ReturnsErr`・`TestService_ComboNotFound`・`TestService_AlreadyAdopted`・`TestRefine_AlreadyAdoptedOnReturnedOnly`・`TestFillerOnly_AlreadyAdoptedByFillerRecipe`。対象の機能が単体版に無い）。
- **`engine/`**（単体版だけ。本家の API 層に当たる）: `New`・`Load`・`ParseQuery`（本家ハンドラと同じクエリ名・既定・誤りの文言・検査の順。`target_move_id` → `target_move`（code））・`Suggest`・`Response`（本家 DTO から `moveId`・`alreadyAdopted` を除いた形）・`LoadJSON`/`SuggestJSON`（WASM の中身。誤りも JSON）。標準ライブラリのみ。
- **`cmd/wasm/`**: `globalThis.tacpendiumSetplay.load(json)`・`.suggest(code, ka, query)`。
- **本家を正解にした適合性テスト**: `scripts/upstream-oracle/`（`run.sh` が本家のチェックアウトへ一時的にテストを 1 本写し、本家の HTTP ハンドラ → サービス → SQL のリポジトリを migrations を流した本物の DB で回して `engine/testdata/upstream_oracle.json` を作る）。条件は `engine/testdata/oracle_conditions.json`（31 キャラ × KA 8 通り {-1,0,7,25,43,61,79,100} × クエリ 18 通り = **4,464 条件**。meaty/gap/filler・全種別・並べ替え・N/G 帯・3 つの絞り込み・target_move・limit 切り・誤り 7 通り）。あわせてキャラごとの候補行と親子関係（本家 repository が DB から読むもの）の要約 31 件。
- **テスト（単体版で書いたもの）**: `engine/conformance_test.go`（上の正解と全条件・全キャラを突き合わせ。キャラごとに並行）、`engine/golden_test.go`（GC-1〜5 を moves DB の実データで）、`engine/canary_test.go`（本家 canary 2 本を moves DB 向けに移植: 接近技の辞書の code が在る・連打版の命名）、`engine/query_test.go`・`engine/api_test.go`、`internal/repository/setplay/repository_test.go`、`internal/movesdb/engine_compat_test.go`（読み口の版 = moves DB の版）。
- **WASM の検査**: `scripts/wasm/go-test-exec.sh`（`GOOS=js GOARCH=wasm go test` 用の `-exec`。環境変数を PATH・HOME だけにして起動）、`scripts/wasm/build.sh`、`scripts/wasm/wasm.test.mjs`（`node --test`。Go → WASM → JS の通し）。
- **CI**: `wasm` job（WASM で go test → ビルド → node --test、サイズを summary へ）と `upstream-oracle` job（正解を本家で作り直して `cmp`）を追加。Go job の timeout 15 → 25 分、シェル構文検査に `scripts/*/*.sh`。
- **その他**: `docs/engine.md`、`docs/upstream-diff.md` 追記、`upstream/watch.sh`（見張りに `internal/api/setplay` と canary 2 本）、`REUSE.toml`（`engine/testdata/upstream_oracle.json` を層 B）・`scripts/check-licenses.py`（EXPECT 3 本）・`DATA-LICENSE.md`・`NOTICE`（移植元と Go ランタイム/wasm_exec.js の BSD-3）、`.gitignore`（`/build/`）。

### 結果（逐語）

ネイティブ（全パッケージ）:

```
$ go test -count=1 -v ./... > /tmp/s3-go-v.txt 2>&1; echo EXIT=$?
EXIT=0
PASS=329 FAIL=0 SKIP=0     （grep -cE '^\s*--- PASS' / '--- FAIL' / '--- SKIP'。サブテストを含む）
ok  	github.com/plexiblinp/tacpendium-setplay/cmd/movesdb	0.387s
ok  	github.com/plexiblinp/tacpendium-setplay/engine	289.517s
ok  	github.com/plexiblinp/tacpendium-setplay/internal/gameversion	0.005s
?   	github.com/plexiblinp/tacpendium-setplay/internal/model	[no test files]
ok  	github.com/plexiblinp/tacpendium-setplay/internal/moveindex	0.003s
ok  	github.com/plexiblinp/tacpendium-setplay/internal/movesdb	0.657s
ok  	github.com/plexiblinp/tacpendium-setplay/internal/repository/setplay	0.008s
ok  	github.com/plexiblinp/tacpendium-setplay/internal/sanumber	0.010s
ok  	github.com/plexiblinp/tacpendium-setplay/internal/seedgen	0.911s
ok  	github.com/plexiblinp/tacpendium-setplay/internal/service/setplay	0.779s
```

（engine の 289 秒は、レビューのサブエージェントが同じコンテナで並行して重い検査を回していた時間を含む。単独では 120 秒＝下）

本家との突き合わせ（ネイティブ・単独）:

```
$ go test -count=1 -v -run 'Conformance' ./engine/ > /tmp/s3-conf.txt 2>&1; echo EXIT=$?
EXIT=0
--- PASS: TestConformance_CandidatesMatchUpstreamRepository (0.04s)
    conformance_test.go:265: 突き合わせた条件: 4464(キャラ 31 ・不一致 0)
--- PASS: TestConformance_UpstreamOracle (119.66s)
```

正解の内訳（`upstream_oracle.json` の集計）: 4,464 条件のうち 200 応答 2,728・400 応答 1,736（誤りのクエリ 7 通り × 248）。`truncated=true` 555、行のある条件 1,874、limit で切られた条件（totalFound > 返却件数）758、3,000 件ちょうど返す条件 488。reason は `knockdown_advantage_negative` 341・`remain_not_below_knockdown` 62。候補行 3,057・親子関係 476（31 キャラ）。

移植テストの件数: 本家（公開版の同じパッケージを回した件数）PASS 131・`func Test` 116 → 単体版 PASS 126・`func Test` 111。**差 5 は移植しなかった 5 本と一致**（上記）。

WASM（`GOOS=js GOARCH=wasm`、Node 22）:

```
$ GOOS=js GOARCH=wasm go test -count=1 -v -timeout 30m -exec="$PWD/scripts/wasm/go-test-exec.sh" ./internal/service/setplay/ ./internal/repository/setplay/ ./engine/ > /tmp/s3-wasm.txt 2>&1; echo EXIT=$?
EXIT=0
ok  	github.com/plexiblinp/tacpendium-setplay/internal/service/setplay	2.156s
ok  	github.com/plexiblinp/tacpendium-setplay/internal/repository/setplay	0.213s
    conformance_test.go:265: 突き合わせた条件: 720(キャラ 5 ・不一致 0)
ok  	github.com/plexiblinp/tacpendium-setplay/engine	173.816s
PASS=152 FAIL=0 SKIP=0
```

```
$ bash scripts/wasm/build.sh
setplay.wasm: 3882385 bytes / gzip -9: 1070766 bytes
$ node --test scripts/wasm/wasm.test.mjs > /tmp/s3-node.txt 2>&1; echo EXIT=$?
EXIT=0
# WASM 経由で突き合わせた条件: 432(不一致 0)
# tests 5
# pass 5
# fail 0
```

正解の作り直し（本家の実装）: `bash scripts/upstream-oracle/run.sh` → `cells: 4464`・`--- PASS: TestStandaloneOracle (325.44s)`・`real 5m26s`・EXIT=0。

計測（同じコンテナ・4 コア、参考値）: 単体版のネイティブで KA 100・全種別・limit 3000 のリュウ 1 条件 3.8 秒（totalFound 155,133）、既定のクエリ 0.49 秒。WASM は単一スレッドで、5 キャラ 720 条件に 174 秒。

### §8-3 わざと壊して赤を確かめた

| 検査 | 壊し方 | 結果 |
|---|---|---|
| 本家との突き合わせ（読み口） | `ParseCharacter` の `OnBlock` を常に nil に | `TestConformance_CandidatesMatchUpstreamRepository` 赤、`UpstreamOracle` も各キャラ「不一致 5 / 144」（block_safe_only の条件）。EXIT=1 |
| 本家との突き合わせ（入口） | `DefaultTargetTypes` から special_projectile を外す | 不一致 542 / 4,464、EXIT=1。このとき `TestParseQuery_Defaults` が**緑のまま**だった（`DefaultTargetTypes()` と比べる同語反復）→ リテラルと比べるよう直し、同じ壊し方で赤を確認 |
| 移植テスト（ネイティブ・WASM） | `newParentResolver` が常に nil（表記用親を前置しない） | ネイティブ `TestCost_Class1_NotationParentIsFree`・`TestFillerOnly_ExpandsChainUnitsAndParents`・`TestGoldenCases` 赤。WASM でも 3 件赤 |
| WASM で本当に回っているか | `-exec=/bin/false`／`-exec` なし | どちらも FAIL（EXIT=1）。また WASM 実行では突き合わせが 5 キャラ 720 件になる（`runtime.GOARCH == "wasm"` の枝を通っている） |
| Go → WASM → JS の配線 | `cmd/wasm` の `js.Global().Set(globalName, api)` を消す | `globalThis.tacpendiumSetplay が無い` で赤 |
| 同上（値の受け渡し） | `suggest` が KA に +1 して渡す | `不一致 216 / 432` で赤 |
| ライセンス宣言 | REUSE.toml の層 B から `engine/testdata/upstream_oracle.json` を外す | `EXPECT: … は CC-BY-SA-4.0 のはずが AGPL-3.0-or-later` 違反 1 件、EXIT=1 |

どれも戻して緑を確認した。

## 判断と理由

- **移植は本家と同じパス（`internal/service/setplay` 等）に置き、`engine/` は薄い公開の入口にした。** 開発者決定 2（本家のファイル構成に合わせ差分を同じ場所へ当てる）と AGENTS §4（`engine/` は標準ライブラリのみ・純関数）の両立のため。案: (b) 全部を `engine/` へ集める → 本家の差分を当てる場所が変わる。採らない。`engine/` は本家の API 層（handler・dto）に当たる部分だけを持つ。
- **入口は `SuggestForKnockdown(ctx, characterCode, ka, params)`。** 案: (b) `SuggestForCombo` とコンボの読み口を残し、KA を入れた偽のコンボを渡す → service.go の差分は最小だが、単体版に無い概念（コンボ ID）を API に残す。採らない。代わりに**テストの本文は本家のまま**にし、フェイクだけで吸収した（`testService.SuggestForCombo`）。本家のテスト差分はそのまま当てられる。
- **`recipeHash` は move ID 列の連結にした。** 用途は重複の判定（等しさ）だけ。本家の sha256 は modifiers が常に null の正準形なので、等しさの意味は同じ。`service/setup`・`internal/recipehash`（と model の Modifiers・ComboStep）を持ち込まずに済む。等しさが変わっていないことは適合性テスト（dedupe の結果を含む全行の一致）で確かめた。
- **数値 ID は moves DB の配列の位置 + 1。** moves DB は本家の id 昇順なので、本家が ID 順に依存する箇所（候補の列挙順・親子の並び・`dedupeByRecipe` の先勝ち）は同じ順になる。読み口の同一性（並び順を含む）を本家 DB と直接比べる検査を置いた。`target_move_id` は code（`target_move`）にした（逸脱ではなく単体版の入力形式の変更。`docs/upstream-diff.md`）。
- **適合性テストの正解は本家の実装そのもので作る（oracle）。** 本家の golden は 5 件・合成の fixture で、配線（DB → 候補 → 絞り込み → 並べ替え → limit）を保証しない（AGENTS §8-5）。本家のハンドラから回すので、クエリの解釈・誤りの文言も同時に比べられる。案: (b) 期待値を手で書いた golden を増やす → 量が足りず、本家の変更に追従できない。(c) 本家のコードを単体版のテストから直接呼ぶ → 別モジュールの internal なので不可。正解はコミットし、CI が本家で作り直して `cmp` する（S2 の data/moves と同じ扱い）。
- **正解は件数と行の正準形の sha256 だけを持つ（行そのものは持たない）。** 全行を持つと数百 MB になる。不一致のときは同じ条件を手元で回して差を見る。
- **条件の選び方**: KA は {-1, 0, 7, 25, 43, 61, 79, 100}（負・0・低〜高、単位上限に達する 67 以上を 2 点、6000 件上限に達する高 KA を含む）。クエリは本家の API の全キーを 1 回以上通る 18 通り。推測: 全 KA（0〜100）は S4 の事前計算の検査で回す（S4 は全 KA を出すので、そこで本家との一致を見るほうが自然）。
- **WASM では本家との突き合わせを 5 キャラに絞った**（ネイティブは全 31 キャラ）。WASM は単一スレッドで 5 キャラ 174 秒。全キャラは 18 分前後になり CI の時間に見合わない。WASM とネイティブで違いうるのは言語処理系の差（整数・ソートの安定性・map の順など）で、5 キャラ 720 条件（型 A の連打版・状態の多いキャラ・単位上限に早く達するキャラを含む）と、移植テスト・engine の単体テスト全部を WASM で回すことで足りると判断した。
- **`go_js_wasm_exec` をそのまま使わずラッパを置いた。** Go 付属のものは環境変数をすべて渡し、wasm_exec.js の上限（引数 + 環境変数で約 12 KB）を超えて起動に失敗した（この環境で再現）。テストは環境変数を読まないので PATH・HOME だけにする。
- **TinyGo は使わない。** 標準 Go の WASM は gzip -9 で 1,070,766 バイト（HANDOVER §5 の見込み「約 1MB」どおり）。TinyGo は HANDOVER に名前があるが、`encoding/json`・`reflect` の対応差の検証が要り、まずは標準 Go で動くものを置いた。サイズを詰めるかは S5（読み込み時間を見て）で決める。
- **KA の上限 100 はエンジンで掛けない**（ローカル版 D10 のため）。定数 `engine.SiteMaxKnockdownAdvantage` だけ置いた（推測: S4 の事前計算と S5 の入力検査が使う）。
- **`engine/testdata/upstream_oracle.json` は層 B（CC-BY-SA-4.0）にした。** ゲームデータで本家の計算を回した結果の要約で、HANDOVER §8 の「派生データ」に当たると判断（§8 の範囲内の当てはめ）。
- **CI に job を 2 つ足した**（`wasm` 約 5 分、`upstream-oracle` 約 6〜7 分。どちらも Go job と並行）。push ごとの Actions の分数が増える（下の残課題）。

## 残課題・既知の問題

- ~~**CI の分数が増えた。**~~ → 報告後の開発者決定で「入力が変わったときだけ作り直す」にした（末尾「S3 後の開発者決定」）。当時の記述: push ごとに Go 約 4 分 + WASM 約 5 分 + 本家の正解の再生成 約 6〜7 分。正解の再生成は入力（lock・条件・生成器）が変わらなければ結果も変わらないので、負担が大きければ「入力のハッシュが変わったときだけ回す」に緩められる（開発者決定 4 の「想定より大きければ緩和」に当たる。判断は開発者）。actions/cache を使う案は、本リポジトリのセッションから SHA を確かめられなかったので入れていない。
- **WASM の本家突き合わせは 5 キャラ**（上記）。
- **GC-2（ケン）の親が実データに無い**のは本家の既知の限界（本家 golden_test.go の注記・followup `gc2-thunder-kick-parent-reference`）。実データ版の golden は「立ち弱P > 紫電カカト落とし」で固定した。本家で親が足されたら `engine/golden_test.go` の期待を直す（適合性テストは自動で追従する）。
- **000014（キンバリーの疾駆け派生の親）は公開版に無い**（S2 から継続）。入れば適合性テストの正解を作り直す。
- WASM の中断は Worker ごと止める前提で、`cmd/wasm` は取り消しを受けない（S5 で Worker と中断ボタンを作る）。
- **移植した本家のテストには穴がある**: 並べ替えの最後のタイブレーク（`sortName` の比較）を逆にしても本家のテストは緑のまま（レビューの追試）。単体版では本家との突き合わせ（1,198 件不一致で赤）が捕まえる。本家側のテストの不足であり、単体版では足していない（本家に追従する方針のため。本家へ伝えるなら開発者経由）。
- 本家の `ProjectCandidates`（計測用）は移植したが単体版では使っていない（本家と同じファイルに保つため残した）。

## 次の段階への入力（S4 事前計算）

- 計算の入口は `engine`（`New`・`Load`・`ParseQuery`・`Suggest`）。並行に回すときはキャラごとに `Engine` を作る（共有状態は読むだけ）。本家と同じ結果になることは `engine/conformance_test.go` が 4,464 条件で保証している。
- 案 B（繋ぎ表）を作るなら、表の結合結果を `engine.Suggest` の出力と突き合わせる検査を置くこと（S1 の masked 方式。単位上限に達する KA 67 以上に注意）。`internal/service/setplay` の `buildFillerUnits`・`collectTargets`・`refineProposals`・`rank` が本家と同じ規則の正本。
- **本家との一致は KA 8 点（{-1,0,7,25,43,61,79,100}）でしか見ていない。** S4 は KA 0〜100 の全点を出すので、事前計算の出力（案 B の結合結果）を全 KA で `engine.Suggest` と突き合わせる検査を S4 に置くこと（engine 自体の本家一致は 8 点、全 KA は S4 で担保する分担）。
- KA の上限 `engine.SiteMaxKnockdownAdvantage = 100`。`limit` の上限 3000 はサービス内（`maxLimit`）。
- 本家の正解（oracle）の作り直し: `bash scripts/upstream-oracle/run.sh`（約 5〜6 分）。上流の取り込み（`/sync_upstream`）のときは data/moves と一緒に作り直してコミットする。
- WASM: `bash scripts/wasm/build.sh` → `build/wasm/setplay.wasm`（gzip 約 1.07 MB）と `wasm_exec.js`。JS API は `docs/engine.md`。

## 最終確認（レビュー取り込み後に回し直した結果）

HEAD `08194b5` で回した（この節を足すコミットは本報告だけを変える）。上の「結果」節の件数は取り込み前のもので、取り込みでテストが 2 本（`TestKnockdownFromFloat`・`TestApiErrorOf_InvalidRemain`）増えた。

```
$ go test -count=1 -v ./... > /tmp/f-go.txt 2>&1; echo EXIT=$?
EXIT=0
PASS=331 FAIL=0 SKIP=0
    conformance_test.go:265: 突き合わせた条件: 4464(キャラ 31 ・不一致 0)
$ GOOS=js GOARCH=wasm go test -count=1 -v -timeout 30m -exec="$PWD/scripts/wasm/go-test-exec.sh" ./internal/service/setplay/ ./internal/repository/setplay/ ./engine/ > /tmp/f-wasm.txt 2>&1; echo EXIT=$?
EXIT=0
PASS=154 FAIL=0 SKIP=0
    conformance_test.go:265: 突き合わせた条件: 720(キャラ 5 ・不一致 0)
$ bash scripts/wasm/build.sh
setplay.wasm: 3884297 bytes / gzip -9: 1070918 bytes
$ node --test scripts/wasm/wasm.test.mjs > /tmp/f-node.txt 2>&1; echo EXIT=$?
EXIT=0
# WASM 経由で突き合わせた条件: 432(不一致 0)
# tests 5
# pass 5
# fail 0
$ go vet ./...                          → EXIT=0
$ gofmt -l（追跡中の .go、_reference 除く）→ 出力なし
$ go mod tidy -diff                     → EXIT=0
$ python3 scripts/check-licenses.py     → 違反: 0 件
$ bash scripts/check-actions-pinned.sh  → 違反: 0 件
```

（gzip 後のサイズはビルドごとに数バイト揺れた: 1,070,915〜1,070,918。生のサイズは同じ）

CI（GitHub Actions run 37870504851、`08194b5`）: リポジトリ検査・Go・WASM・本家の正解の再生成・Web の 5 job がすべて success。本家の正解の再生成は「正解の再生成」約 5 分 → 「コミット済みと一致」（`cmp`）success（レビュー中-4 で指摘された未完走はこれで解消）。

削除されたファイル: 0（`git diff --diff-filter=D --name-only daec548ca9f2244bc24db68e2a34c8e4a1a7df8e HEAD`）。テストファイルは追加のみ（本家から移植しなかった 5 本は、本家のファイルを写した時点で外したもので、このリポジトリに一度も入っていない）。

### `git diff --stat daec548ca9f2244bc24db68e2a34c8e4a1a7df8e HEAD`

```
 .github/workflows/ci.yml                           |    83 +-
 .gitignore                                         |     3 +
 DATA-LICENSE.md                                    |     2 +-
 NOTICE                                             |     6 +-
 REUSE.toml                                         |     4 +-
 cmd/wasm/main.go                                   |    46 +
 docs/engine.md                                     |    77 +
 docs/progress/S3-report.md                         |   177 +
 docs/progress/S3-review.md                         |   195 +
 docs/upstream-diff.md                              |    22 +-
 engine/api.go                                      |   113 +
 engine/api_test.go                                 |   133 +
 engine/canary_test.go                              |   120 +
 engine/conformance_test.go                         |   266 +
 engine/engine.go                                   |   128 +
 engine/golden_test.go                              |    91 +
 engine/query.go                                    |   172 +
 engine/query_test.go                               |    83 +
 engine/testdata/oracle_conditions.json             |    24 +
 engine/testdata/upstream_oracle.json               | 46972 +++++++++++++++++++
 internal/model/doc.go                              |     2 +
 internal/model/move.go                             |    71 +
 internal/movesdb/engine_compat_test.go             |    15 +
 internal/repository/setplay/repository.go          |   240 +
 internal/repository/setplay/repository_test.go     |    98 +
 internal/service/setplay/approach.go               |   143 +
 internal/service/setplay/cancel.go                 |    55 +
 internal/service/setplay/cancel_test.go            |   298 +
 internal/service/setplay/cost_test.go              |   411 +
 internal/service/setplay/filler_only_test.go       |   255 +
 internal/service/setplay/golden_test.go            |   135 +
 internal/service/setplay/rapid.go                  |    93 +
 internal/service/setplay/refine.go                 |   190 +
 internal/service/setplay/refine_m4705_test.go      |   592 +
 internal/service/setplay/refine_test.go            |   389 +
 internal/service/setplay/service.go                |  1110 +
 internal/service/setplay/service_test.go           |   454 +
 internal/service/setplay/setplay.go                |   319 +
 internal/service/setplay/setplay_test.go           |   690 +
 scripts/check-licenses.py                          |     3 +
 scripts/upstream-oracle/run.sh                     |    29 +
 .../upstream-oracle/standalone_oracle_test.go.txt  |   259 +
 scripts/wasm/build.sh                              |    13 +
 scripts/wasm/go-test-exec.sh                       |    10 +
 scripts/wasm/wasm.test.mjs                         |   126 +
 upstream/watch.sh                                  |     3 +
 46 files changed, 54714 insertions(+), 6 deletions(-)
```

## S3 後の開発者決定（2026-10-10）

報告後の対話で、開発者が次を決めた。

1. **本家の正解の再生成（CI の `upstream-oracle` job）は、入力が変わったときだけ回す。**
   - 入力: `upstream/tacpendium.lock`・`engine/testdata/oracle_conditions.json`・`engine/testdata/upstream_oracle.json`（正解そのもの。手で書き換えても作り直しで見つかる）・`scripts/upstream-oracle/`・`scripts/fetch-upstream.sh`・`go.mod`（Go の版）。`ci.yml` は正解の中身を決めないので入れない（当初は入れていたが、CI を触るブランチで毎回走るので外した）。
   - 判定: `scripts/upstream-oracle/changed.sh <基準>`。基準は main への push なら `github.event.before`、ほかのブランチなら `origin/main` との分岐点、手動実行は常に作り直す。基準のコミットが見つからなければ作り直す（安全側）。
   - 省いたときも job は success で終わり、summary に「入力の変更なし: 作り直しを省いた（基準 …）」と出す。
   - 単体版の突き合わせ（`engine/conformance_test.go`）と WASM の検査は、これまでどおり push ごとに回る（変わるのは正解を本家で作り直す部分だけ）。
   - §8-3: 一時 worktree の使い捨てコミットで、正解だけを 1 バイト変える → `changed`、lock を変える → `changed`、docs だけ → `unchanged`、ci.yml だけ → `unchanged`、生成器（run.sh）→ `changed`。基準が全 0 や空 → `changed`。CI では、ci.yml を変えたコミットで作り直しが走り、入力に触れないコミットで省かれることを確かめた（結果は下に追記）。
   - CI の結果: 作り直す側 = run 38057155922（`b872540`。ci.yml を変えたので `changed`）で「正解の再生成」約 8 分 → 「コミット済みと一致」success、5 job すべて success。省く側 = 本追記のコミット（報告だけを変える）の run で確かめる→ run 38057782212 は `changed`。判定の基準がブランチでは「origin/main との分岐点」なので、正解を足した S3 ブランチではマージまで毎回作り直す（判定どおり）。**省く側は CI ではまだ確かめていない**（手元の判定スクリプトの検査だけ）。マージ後に入力に触れないブランチの最初の push で summary に「入力の変更なし: 作り直しを省いた」と出ることを確かめる（S4 の最初の確認事項）。
   - 判定の基準を「前回の push（github.event.before）」にしなかった理由: CI は同じ ref の古い実行を打ち切る（concurrency）ので、入力を変えた push の実行が次の push で打ち切られると、次の実行は「変更なし」と判定して検査が抜ける（本セッションで実際に打ち切りが起きた）。main への push だけは `before` を使う（マージの頻度が低い。打ち切られた場合は手動実行で作り直す）。

2. **本家と比べる条件に、キャラごとの safe jump の remain を足す。**（PR #4 のレビューでの依頼）
   - `engine/testdata/oracle_conditions.json` のクエリに `mode=filler&remain={safe_jump.true}`・`{safe_jump.f5}`・`{safe_jump.f6}` の 3 本を足した（18 → 21 通り、条件 4,464 → **5,208**）。
   - `{safe_jump.*}` はキャラごとの安全飛びの値に置き換えて回す。本家側（`scripts/upstream-oracle`）は本家 DB の `characters.custom_states` の `safe_jump` から、単体版側は moves DB の `safeJump`（`engine.Engine.SafeJump` を新設）から、Node 側は moves DB の JSON から置き換える。値は本家の 000010 から取り込み済み（例 リュウ 43/42/41・ダルシム 73/72/71・春麗 47/46/45）。
   - 結果: ネイティブ 5,208 条件・不一致 0（safe jump の 744 条件のうち行のある条件 234、`remain_not_below_knockdown` 415）。WASM 840 条件・不一致 0。Node 経由 504 条件・不一致 0（5/5）。WASM の go test PASS 155・FAIL 0。正解の作り直しは本家側で 502 秒。
   - §8-3: Go 側の置き換えで f5 に f6 の値を入れる → 不一致 87（赤）。Node 側で同じ壊し方 → 不一致 8 / 504（赤）。戻して緑。
3. **変数名 `ka`・`KAs` はそのままにする。** 本家から移植したコード（`internal/service/setplay`）が全体で `ka` を使っており、本家の差分を当てるために名前を揃えている。「KA」は AGENTS.md §0 の用語表と HANDOVER で定義済みで、毎セッション読まれる。単体版のコードだけ改名すると、本家のコードと呼び方が分かれる。代わりに `engine` のパッケージ説明と `Suggest` の説明に「KA ＝ knockdown advantage（ダウン後の有利フレーム）」を明記した。
