# 本家（公開版 tacpendium）から移植したコードと、単体版での変更点

開発者決定（S1 報告「S1 後の開発者決定」2）により、移植したコードは**本家と同じパス・ファイル名・関数名**に置き、
本家の差分を同じ場所へ当てられるようにする。本ファイルは「本家のファイルから変えた箇所」の一覧の正本である。
取込元は公開版 tacpendium（`upstream/tacpendium.lock` の SHA）。見張りは `upstream/watch.sh`、取り込みは `/sync_upstream`。

## 移植したパッケージ（S2）

| 単体版のパス | 本家のパス | 移植の版 |
|---|---|---|
| `internal/seedgen/`（テスト含む・`testdata/` 除く） | 同じ | lock SHA |
| `internal/moveindex/`（テスト含む） | 同じ | lock SHA |
| `internal/sanumber/`（テスト含む） | 同じ | lock SHA |
| `internal/gameversion/`（テスト含む） | 同じ | lock SHA |
| `internal/service/setplay/`（テスト含む）（S3） | 同じ | lock SHA |
| `internal/repository/setplay/repository.go`（S3。型だけ移植し、読み口は単体版で書き直し） | 同じ | lock SHA |
| `internal/model/move.go`（S3。使う定数だけ） | 同じ | lock SHA |
| `internal/migrationpin/`（テスト・`applied.sha256` 含む）（公開版 v0.3.0 の取り込みで追加。`internal/seedgen` のテストが使う） | 同じ | lock SHA |

本家に当たるものが無い単体版だけのもの（S3）: `engine/`（本家の API 層 `internal/api/setplay` の handler・dto に当たる入口）、`cmd/wasm/`、
`scripts/upstream-oracle/`（本家の実装で適合性テストの正解を作る）、`scripts/wasm/`。

共通の機械的な変更: import パス `github.com/plexiblinp/tacpendium/internal/...` → `github.com/plexiblinp/tacpendium-setplay/internal/...`。

## ファイルごとの変更

| ファイル | 変更 | 理由 |
|---|---|---|
| `internal/seedgen/generate_test.go` | `repoRoot` が go.mod のあるディレクトリではなく、公開版のチェックアウト（`TACPENDIUM_SRC` か `.upstream/tacpendium`）を返す。無ければ `t.Fatal`（skip しない） | 単体版は `character_data/`・`migrations/`・`internal/seedgen/testdata/` を持たない。テストは公開版の同じファイルと突き合わせる |
| `internal/seedgen/testdata/` | 移植しない | 上と同じ。比較先は公開版のチェックアウトの同じパス |
| `internal/seedgen/generate_m2002.go` | `resolveCharacter` の本体を `resolveCharacterWith(…, resolve collisionResolver)` へ切り出し、`collisionResolver` 型を追加。`resolveCharacter` は `resolve = nil` で呼ぶだけ（nil なら本家と同じ挙動）。衝突の解き方を選ぶ `resolveCollisions` の宣言を `collisionResolver` 型にし、`resolve != nil` なら差し替える | 単体版は逆引きしないので被りを落とさない（HANDOVER §2.6）。本家の差分を当てるときは、`resolveCharacter` への変更を `resolveCharacterWith` へ当てること |
| `internal/seedgen/standalone.go`（新規・本家に無い） | `ResolveAllAliases`（被りを落とさない解決）・`UpstreamUnfilled`（本家の規則で落ちる技と理由。オラクル突合用）・`MovementAlias`・`keepAllCollisions` | 単体版だけの追加。本家ファイルを変えずに済むよう別ファイルに置く |

### S3（セットプレイのエンジン）

| ファイル | 変更 | 理由 |
|---|---|---|
| `internal/service/setplay/service.go` | 入口 `SuggestForCombo(ctx, comboID, params)` を `SuggestForKnockdown(ctx, characterCode, ka, params)` に。`ComboReader`・`DuplicateChecker`・`NewService` の引数 2 つ・`ErrComboNotFound`・`ErrKnockdownNotSet`・`Proposal.AlreadyAdopted` と、返却分の alreadyAdopted の判定（2 か所）を削除。`MoveCandidateReader` のキャラを `characterID int64` → `characterCode string` にし、`CharacterCode` を削除（接近技の辞書は入力の code で引く）。`suggestFillerOnly` の引数から comboID・characterID を削除。`assemble` の `recipeHash` を `setupsvc.CalcSetupRecipeHash`（sha256）から「move ID 列の連結」に | HANDOVER §2.2（comboId 不要・alreadyAdopted 削除）。`recipeHash` の単体版での用途は `dedupeByRecipe` の等しさだけで、本家の正準形は modifiers が常に null なので「ID 列が同じ ⇔ ハッシュが同じ」。`service/setup`・`internal/recipehash` を持ち込まないため |
| `internal/service/setplay/service_test.go` | フェイクだけを差し替え: `fakeComboReader`・`fakeDup` の中身・`CharacterCode` を削除。`newCombo` は KA の入れ物 `testCombo` を返し、`newService` は本家と同じ呼び出しの形のまま `testService`（`SuggestForCombo` を `SuggestForKnockdown` へ渡す）を返す | テストの本文を本家と同じに保ち、本家の差分を同じ場所へ当てられるようにするため |
| `internal/service/setplay/*_test.go` | 移植しないテスト 5 本: `TestService_KnockdownNull_ReturnsErr`・`TestService_ComboNotFound`・`TestService_AlreadyAdopted`（service_test.go）、`TestRefine_AlreadyAdoptedOnReturnedOnly`（refine_test.go）、`TestFillerOnly_AlreadyAdoptedByFillerRecipe`（filler_only_test.go）。`refine_m4705_test.go` の `suggestAll` の引数型を `testService` に。`filler_only_test.go` の `setupsvc` の import を削除 | 対象の機能（コンボ・KA 未入力・alreadyAdopted）が単体版に無い |
| `internal/repository/setplay/repository.go` | `MoveCandidate`・`MoveDerivation` はそのまま。SQL の実装（`Repository`・`New`・SQL 3 本）を削除し、moves DB（JSON）を読む `ParseCharacter`・`Store`・`Character`・`SafeJump`・`SchemaVersion`・`ErrCharacterNotFound` を追加 | HANDOVER §2.1（DB 読込 → JSON 読込）。数値 ID は配列の位置 + 1（並びは本家の id 昇順なので順序の意味は同じ）。本家の読み口との同一性は `engine` の `TestConformance_CandidatesMatchUpstreamRepository` が見る |
| `internal/repository/setplay/repository_test.go`（新規・本家に無い） | `ParseCharacter`・`Store` の単体テスト | 単体版で書いた読み口のテスト |
| `internal/model/move.go` | 技カテゴリ・名指しの技 code・startup_basis の定数だけ（`Move` 構造体・技プロパティ等は無い）。`doc.go` は単体版で書いた | 計算が使う定数だけを持ち込む |
| 移植しない本家ファイル | `internal/service/setup`・`internal/recipehash`・`internal/repository/combo`・`internal/api/setplay`（engine が同じ役を持つ）・`internal/infra/migration` の canary（`engine/canary_test.go` が moves DB に対して同じことを見る。計測専用の `canary_setplay_projection_test.go`・`canary_setplay_perf_test.go` は移植しない） | |

### 公開版 v0.3.0（取り込み時 `5840f6e`、メールの書き換え後 `ebd2768`。中身は同じ）の取り込み（2026-10-25。報告 `docs/progress/upstream-5840f6e-report.md`）

| ファイル | 変更 | 理由 |
|---|---|---|
| `internal/service/setplay/suggest_for_combo.go`（本家で新規） | 本家は `SuggestForCombo` の本体を service.go からこのファイルへ移し、手順ごとの関数（`checkSuggestPreconditions`・`loadSuggestMaterial`・`suggestTargets`・`targetOptions`・`limitProposals`）に分けた。単体版は同じファイル・同じ関数名で入口を `SuggestForKnockdown` のまま移した。`findComboForSuggest`・`markAlreadyAdopted` の呼び出しは持たない。`loadSuggestMaterial` はキャラを code で受け、`CharacterCode` を読まない。並べ替えの後・件数で切る前に `applyVotes` を掛ける（S6 と同じ位置） | 上の S3 の変更をこの形へ当てた |
| `internal/service/setplay/service.go` | 本家の `withoutParentsHash`（親行を除いた並びのレシピハッシュ）と `markAlreadyAdopted` は移さない | 採用済みの印（alreadyAdopted）のためだけの値で、単体版に無い |
| `internal/service/setplay/suggest_for_combo_golden_test.go` | fixture の `goldenMoves` だけを移した。`TestSuggestForComboGolden` と `testdata/suggest_for_combo_golden.json` は移さない | golden の出力に採用済みの印と重複判定の呼び出し順を含む。関数を分けた後も同じ振る舞いであることは engine の適合性テスト（本家の新しい実装で作り直した正解との全件の突き合わせ）が見る |
| `internal/service/setplay/service_m4305_test.go` | 移さない | 採用済みの印の試験 |
| `internal/service/setplay/suggest_for_combo_branches_test.go` | そのまま移した（`SuggestForCombo` は試験用の `testService` 経由） | |

### S4（事前計算）

| ファイル | 変更 | 理由 |
|---|---|---|
| `internal/repository/setplay/repository.go` | `Character.CalcFingerprint` を足す（moves DB の `calcFingerprint` を読む） | 事前計算 CLI が空でないことを確かめる・CI の差分実行の鍵（`scripts/ci/verify-plan.py` は index.json の同じ値を使う） |

本家に当たるものが無い単体版だけのもの（S4）: `cmd/precompute/`。
S4 で足した繋ぎ表（`internal/service/setplay/table.go`・`service.go` の `engineFor`/`tables`・`engine.LoadTable`・WASM の `loadTable`）は **S6 で消し、`service.go` の呼び出しを本家と同じ `runEngine(...)` に戻した**（S5 後の開発者決定 2）。

本家の API 層との対応（`engine/`）: `ParseQuery` は本家 `GetSuggestions` のクエリ解釈（名前・既定・誤りの文言・検査の順）と同じ。違いは `target_move_id`（数値 ID）→ `target_move`（技の code）だけ。結果の形 `Response` は本家の `SuggestionsResponse` から `moveId`・`alreadyAdopted` を除いたもの。誤りの写し方（`SuggestJSON`）も本家ハンドラに揃える: クエリの誤りと `ErrInvalidRemain` は `invalid_query_parameter`（本家の 400）。本家に無いもの: `character_not_loaded`・`invalid_moves_data`・`invalid_arguments`（WASM の引数。KA は整数だけ受け、NaN・Infinity・小数は誤り）・`internal_error`（本家の 500）。

一覧の漏れの見つけ方: import パスを揃えて公開版と比べる（差のあるファイルが上の表と一致すること）。

```sh
for f in internal/{seedgen,moveindex,sanumber,gameversion,model,repository/setplay,service/setplay}/*.go; do
  u=".upstream/tacpendium/$f"
  [ -f "$u" ] || { echo "単体版のみ: $f"; continue; }
  sed 's#tacpendium-setplay/internal#tacpendium/internal#' "$f" | cmp -s - "$u" || echo "変更あり: $f"
done
```
