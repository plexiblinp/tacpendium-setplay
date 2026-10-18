# エンジン（セットプレイ提案の計算）

S3 で本家（公開版 tacpendium）のセットプレイ提案を移植した。本家との変更点の正本は `docs/upstream-diff.md`。

## 構成

| 場所 | 役割 | 本家との関係 |
|---|---|---|
| `internal/service/setplay/` | 計算の本体（探索エンジン `setplay.go`・グルー `service.go`・絞り込み `refine.go`・連打版 `rapid.go`・接近技 `approach.go`・中断 `cancel.go`） | 本家と同じパス・名前。変えたのはコンボ・alreadyAdopted の除去と入口だけ |
| `internal/repository/setplay/` | moves DB（`data/moves/<code>.json`）を本家と同じ型（`MoveCandidate`・`MoveDerivation`）で読む | 型は本家と同じ。読み口（SQL → JSON）は単体版で書いた |
| `internal/model/` | 計算が使う定数（技カテゴリ等） | 本家の一部 |
| `engine/` | 公開の入口。`New`・`Load`・`ParseQuery`・`Suggest`・`LoadJSON`・`SuggestJSON` | 本家の API 層（handler・dto）に当たる。単体版だけ |
| `cmd/wasm/` | `engine` を WASM（`GOOS=js GOARCH=wasm`）で JS に出す | 単体版だけ |
| `cmd/precompute/` | 配信する静的データ（manifest・moves。`docs/precompute.md`）。S4 の繋ぎ表は S6 で消した | 単体版だけ |

どれも標準ライブラリだけに依存する（AGENTS §4）。`internal/movesdb`（moves DB を作る側。sqlite 依存）は import しない。

## 使い方

### Go（事前計算など）

```go
e := engine.New()
code, err := e.Load(jsonBytes)              // data/moves/ryu.json の中身
q, err := engine.ParseQuery(url.Values{...}) // 本家の API と同じクエリ名
res, err := e.Suggest(ctx, code, ka, q)      // *engine.Response（本家の SuggestionsResponse と同じ形）
```

### WASM（ブラウザ・Node）

```sh
bash scripts/wasm/build.sh   # build/wasm/setplay.wasm と wasm_exec.js
```

```js
// wasm_exec.js を読み込んだ後
const go = new Go();
const { instance } = await WebAssembly.instantiate(bytes, go.importObject);
go.run(instance); // 待たない(main は関数を登録して待ち続ける)
const api = globalThis.tacpendiumSetplay;
api.load(movesJsonText);                    // → '{"character":"ryu"}' | '{"error":{...}}'
api.suggest("ryu", 40, "mode=gap&g_min=2"); // → '{"result":{...}}'  | '{"error":{...}}'
api.suggest("ryu", 40, "", '{"a,b|t":3}');  // 4 番目は任意: 票(S6)。{"<recipeKey>": good − bad}
```

どちらも JSON 文字列を返し、例外を投げない。`ka` は整数だけ受ける（小数・NaN・Infinity は `invalid_arguments`）。計算の中断は Web Worker ごと止める（S5）。

### クエリ（`engine.ParseQuery`）

本家の `GET /api/combos/:comboId/setplay-suggestions` と同じ（名前・既定・誤りの文言）。違いは `target_move_id`（数値 ID）の代わりに `target_move`（技の code）を使うことだけ。

| キー | 既定 | 意味 |
|---|---|---|
| `mode` | `meaty` | `meaty`（重ねる）/ `gap`（あえて重ねない）/ `filler`（繋ぎだけ） |
| `n_min` / `n_max` | 1 / なし | meaty の持続番号 N の範囲 |
| `g_min` / `g_max` | 1 / 13 | gap の隙間 G の範囲（範囲外は丸める） |
| `remain` | （filler で必須） | 繋ぎの後に残すフレーム（安全飛びはキャラごとの値。`Engine.SafeJump(code)` で取れる。moves DB の `safeJump`） |
| `target_types` | `normal,unique,special_projectile` | 当てる技の種別（カンマ区切り） |
| `target_move` | なし | 当てる技を 1 つに決める（技の code） |
| `include_zero_damage` | false | ダメージ 0 の技も当てる技にする |
| `approach_only` / `block_safe_only` / `chain_cancel_only` | false | 結果の絞り込み（AND） |
| `sort` | `n` | `n`（meaty は N 降順・gap は G 昇順）/ `target` |
| `limit` | 200 | 返す件数（上限 3000 に丸める） |

### 票（S6・HANDOVER D17。単体版だけ）

- 結果の各行に `recipeKey`（`internal/service/setplay/votes.go` の `RecipeKey`。当てる技以外の手の code を `,` で連結 `|` 当てる技の code）を付ける。
- `SuggestParams.Votes`（鍵 → スコア。Go は `Query.Params.Votes`、JSON は `SuggestWithVotesJSON`、WASM は `suggest` の 4 番目の引数）を渡すと、本家の並べ替えの後にスコアの降順で安定に並べ直し、その後で `limit` を切る。空なら並びも中身も本家と同じ。
- 票の JSON の誤り（整数でない・鍵が空や長すぎる・10,000 件超）は `invalid_arguments`。詳しくは `docs/telemetry.md`。

KA の上限（サイト 100 = HANDOVER D10）はエンジンでは掛けない（ローカル版は無制限）。定数は `engine.SiteMaxKnockdownAdvantage`。

## 検査

| 検査 | どこで |
|---|---|
| 票（第一キー・limit の前・票なしは不変） | `engine/votes_test.go`・`internal/service/setplay/votes_test.go` |
| 本家のテスト（golden を含む） | `internal/service/setplay/*_test.go`（本文は本家と同じ） |
| 本家の実装との一致（全 31 キャラ × KA 8 通り × クエリ 21 通り。クエリの `{safe_jump.true}`・`{safe_jump.f5}`・`{safe_jump.f6}` はキャラごとの安全飛びの値に置き換える） | `engine/conformance_test.go` が `engine/testdata/upstream_oracle.json` と突き合わせる。正解は `scripts/upstream-oracle/run.sh` が本家のハンドラ・サービス・SQL を本物の DB で回して作る |
| 読み口の一致（moves DB の候補行・親子関係 = 本家 DB の候補行・親子関係） | `engine` の `TestConformance_CandidatesMatchUpstreamRepository` |
| 実データの golden（GC-1〜5） | `engine/golden_test.go` |
| 本家の canary（接近技の辞書・連打版の命名） | `engine/canary_test.go` |
| WASM で同じテスト | `GOOS=js GOARCH=wasm go test -exec="$PWD/scripts/wasm/go-test-exec.sh" ./internal/service/setplay/ ./internal/repository/setplay/ ./engine/`（本家との一致は 5 キャラ・KA 79 以下に絞る。KA 100 はメモリの断片化で落ちることがあるため。WASM の KA 100 は Node の通しが見る） |
| Go → WASM → JS の通し | `bash scripts/wasm/build.sh && node --test scripts/wasm/wasm.test.mjs` |

CI では、本家との突き合わせ（ネイティブ・WASM）と Node の通しは、検査済みでないキャラ（指紋か、記録した計算の版が変わったキャラ）だけ回す（`docs/precompute.md` の「CI での差分実行」）。

正解の作り直し: 上流を取り込んだとき（`/sync_upstream`）や条件（`engine/testdata/oracle_conditions.json`）を変えたときは `bash scripts/upstream-oracle/run.sh` を回してコミットする。CI が作り直して一致を見る。CI が作り直すのは入力（`upstream/tacpendium.lock`・`engine/testdata/oracle_conditions.json`・`engine/testdata/upstream_oracle.json`・`scripts/upstream-oracle/`・`scripts/fetch-upstream.sh`・`go.mod`）が変わったときだけ（判定は `scripts/upstream-oracle/changed.sh`。S3 後の開発者決定）。
