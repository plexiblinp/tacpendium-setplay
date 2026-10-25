# moves DB（`data/moves/`）

単体版の技データ。公開版 Tacpendium（`upstream/tacpendium.lock` の SHA）から `cmd/movesdb` が作る。
形の正本は `internal/movesdb/model.go`（`SchemaVersion`）。ライセンスは層 B（`CC-BY-SA-4.0`。`DATA-LICENSE.md`）。

## 作り方

```sh
bash scripts/fetch-upstream.sh            # 公開版を .upstream/tacpendium へ（lock の SHA）
go run ./cmd/movesdb                       # 検査して data/moves/ へ書く。違反があれば何も書かずに exit 1
go test -count=1 ./...                     # 検査・再生成一致（公開版の取得が要る）
```

- 公開版の `migrations/*.up.sql`（000001〜）を番号順にメモリ上の SQLite へ流し、本家のエンジンと同じ列を読む。
  `setup_only`・派生関係・安全飛び・データ修正（000011・000013 の UPDATE）は SQL にしか無いため、CSV だけからは作らない。
- 生成物はコミットする。CI（`.github/workflows/ci.yml` の Go job）が作り直し、1 バイトでも違えば赤になる
  （`internal/movesdb` の `TestPipeline_RegeneratesCommittedData` と CI の「moves DB の再生成と一致」）。
  **手で編集しない**。直すときは上書き表か生成器を直して作り直す。

## ファイル

| ファイル | 中身 |
|---|---|
| `data/moves/<キャラ code>.json` | キャラ 1 人分（下の形） |
| `data/moves/index.json` | 全体の版 `dataVersion`（= lock）と、キャラ一覧（本家の characters.id 順）・各キャラの版 `dataVersion`・各ファイルの `fileSha256`・技数・`calcFingerprint` |
| `data/overrides/name_en.json` | 英語技名の上書き表（キャラ → move_code → `{nameEn, reason}`） |
| `data/overrides/notation.json` | numeric / SRK の上書き表（キャラ → move_code → `{numeric?, srk?, reason}`） |

上書き表は**公式サイト等から転記しない**（`NOTICE` §2）。`reason` は必須（空なら生成が止まる）。未知のキャラ・技を書いても止まる。

## キャラ 1 人分の形

| キー | 意味 |
|---|---|
| `schemaVersion` | 形の版 |
| `dataVersion` | **キャラごとの版**: このキャラの**計算に効く列**（`calcFingerprint`）が最後に変わった取り込みの、公開版 Tacpendium の SHA。前の moves DB と `calcFingerprint` が同じなら前の版を引き継ぐ（`cmd/movesdb -prev`。既定は出力先そのもの）。表記・名前・メモ・ヒット時の有利など表示だけの変更では上げない。全体の版（= lock）は `index.json` の先頭の `dataVersion`（S4 後の開発者決定 2・2026-10-11 の見直し） |
| `character` | `code`・`name.ja`・`name.en`（本家 characters） |
| `states` | キャラ固有の状態（本家 `custom_states.states` の `code`・名前）。無ければ `[]` |
| `safeJump` | 安全飛びの値 `{true, f5, f6}`（本家 `custom_states.safe_jump`。「繋ぎの後に残すフレーム」）。無ければ `null` |
| `calcFingerprint` | キャラ単位の指紋（全技の `calcFingerprint` を並び順に連ね、`safeJump` を足した sha256） |
| `moves` | 技の配列。**本家の moves.id 昇順**（エンジンの決定性に効く。並べ替えない） |

## 技 1 件の形

数値の `null` は「データに値が無い」で、0 とは別（AGENTS §8-7）。真偽値は常に出す。
**列の意味は名前から推測しない**（ドメイン知識集成 §7。下の注記がそれ）。

| キー | 本家の列 | 注記 |
|---|---|---|
| `code` | `moves.code` | キャラ内で一意。キーは数値 id ではなくこれ |
| `category` | `moves.category` | normal / special / unique / super_art / critical_art / throw / system / target_combo / rush_variant / drive_impact |
| `startup` `active` `recovery` `total` | 同名 | `startup` の起点は `startupBasis` で決まる（通し値か単発値か） |
| `onHit` `onBlock` `damage` | `on_hit` `on_block` `damage` | `onBlock` は持続 1F 目を当てたときの値 |
| `isDerived` | `is_derived` | **派生技の印ではない**。「入力単独では技を解決できない（コマンド索引から外す）」印。非派生でも true がある |
| `isAerial` | `is_aerial` | **空中技の印ではない**。「ラッシュ版の自動生成から外す」印 |
| `isProjectile` | `is_projectile` | 飛び道具 |
| `setupOnly` | `setup_only` | 本家でも意味が照会中（ドメイン知識集成 §7-3）。値はそのまま運ぶ |
| `startupBasis` | `startup_basis` | standalone / through / unknown |
| `chainCancelTotal` | `chain_cancel_total` | 非 null であること自体がチェーングループ員の印。値はチェーン時の実消費 |
| `fastestUnreachable` | `fastest_unreachable` | 最速で出しても地上の相手に当たらない（target にできない） |
| `originalMove` | `original_move_id` を code に | ラッシュ版の元技。それ以外は `null` |
| `parents` | `move_derivations` を code に | 表記用の親（派生元）。無ければ `[]`。code 昇順（本家のサービス層が先頭を採るため） |
| `state` | — | この技が前提とするキャラ固有の状態（`states[].code`）。`move_code` が状態 code で始まり（ラッシュ版は `rush_` の後ろ。状態 code どうしが接頭辞の関係なら長いほう）、**かつ** `name_ja` の `[..]`・`【..】` のラベルに状態の日本語名が含まれる技に付く。無ければ `null`。★接頭辞だけでは決めない（舞の `kachousen_*` は状態「花蝶扇バウンド中」を作る技で、前提ではない）。★`custom_states` に無い印（`【ジャスト】`・`[焔版]` 等）は状態ではないので付かない |
| `notation` | — | 4 表記（下） |
| `calcFingerprint` | — | 計算に効く列だけの指紋（sha256 の先頭 16 桁）。`recovery`・`onHit`・表記は含まない |

CSV に無く本家 DB が seed する技（移動系 9 code・`drive_reversal`・ダルシム/ザンギエフの連打版）も含む。

★`originalMove`・`parents` が同じキャラの技を指さない行があると、生成は止まる。本家のサービス層は解決できない親を
黙って飛ばす（表記に前置しない）ので挙動が違うが、取込の時点で上流のデータの変化に気づくため、単体版は止める。

## 4 表記（`notation`）

| キー | 作り方 |
|---|---|
| `nameJa` | CSV の `name_ja`。CSV に無い技は本家 DB の `official_ja_move`（現データでは CSV と全件一致） |
| `nameEn` | `move_code` から規則で生成（`internal/movesdb/name_en.go`）＋ `data/overrides/name_en.json` |
| `numeric` | 本家の生成器（`internal/seedgen`。本家 preset `numeric` と同じ規則）＋区別印 |
| `srk` | 同（preset `srk`）＋区別印。英語の値がある技（移動系の微歩き等）は英語の値を使う |

本家の生成器が表記を作れない技（command が空の派生・語彙外・CSV に無い技等）は、`numeric` = `nameJa`、`srk` = `nameEn` にする。

公開版 v0.3.0（本家の M43-05）から、生成器そのものが**ターゲットコンボを段ごとに `~` でつなぎ末尾に `(TC)`**（numeric `5MP~5HP(TC)`・SRK `st.MP~st.HP (TC)`。以前は `MP>HP`）、**空中から出す技に `j.`（垂直ジャンプ `nj.`・ハイジャンプ `hj.`）**を付ける。そのため、以前は被って区別印で分けていた組の一部は、印が要らなくなった（例 `2HP (しゃがみ強P)` → `2HP`。空中の版が `j.2HP` になったため）。

### 区別印（D9・HANDOVER §4.3）

本家は表記の逆引きのため、同じキャラで同じ表記になる技の 1 つだけに表記を残す。単体版は逆引きしないので全技に表記を付け、
**被った組の中でだけ**、違う部分を印にする（被らない技は本家と同じ文字列）。段は上から順に、まだ被っている技にだけ当てる。

1. 空中印: `move_code` の語（`aerial`・`air`・`jumping_`・`neutral_jump`・`high_jump`）で空中の版と分かる技に `j.`（`nj.`・`hj.`）
2. OD 印: `move_code` の主部（`during` より前）に `od` がある技に ` (OD)`
3. ラベル: numeric は `name_ja` の `[..]`・`【..】`（前へ）と `(..)`・`（..）`（後ろへ）のうち組で共有しないもの。
   SRK は状態の英語名（`[Denjin Charge] 236P+P`）と、`move_code` の語のうち組で共有しないもの（後ろへ括弧書き。`lv1` は `Lv1`）
4. 名前: それでも被る技に、その表記の言語の技名を括弧で添える

### 検査（違反があれば生成も CI も赤）

- 32 キャラ（`ExpectedCharacterCount`。2026-10 のアルジュンで 31 → 32）・形どおり（未知のキーは読込で誤り。型が持つキーの欠けも、入れ子・配列の要素・index.json まで含めて誤り）・`state` が `states` にある・`dataVersion` が 40 桁の 16 進（キャラごとの版。中身が変わったキャラが lock になることは `TestCarryDataVersions` が見る）・`index.json` の先頭の `dataVersion` = lock・各キャラの版・sha256・技数・指紋が中身と一致
- **全技が 4 表記を持つ**（空・空白だけは不可）
- **同一キャラ × 同一表記種で、表示文字列が完全一致する技が 0 件**
- `originalMove`・`parents` が同じキャラの実在の技を指す
- CSV と DB の計算列が一致する（データ修正の migration が片方だけを直していない）
- **オラクル突合**: 区別印を付ける前の numeric / SRK を本家 DB の preset と比べ、説明のつかない差を違反にする。
  説明のつく差は「本家の規則で回し直すと被り（移動系の予約表記を含む）で落ちる」「本家の規則で回し直すと元技の表記が無くて落ちるラッシュ版」
  「本家にも生成器にも無い（名前フォールバック）」「上書き表」の 4 つ。「他の技と被っている」だけでは通さない
