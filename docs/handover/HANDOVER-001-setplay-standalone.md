# HANDOVER-001: セットプレイ自動提案 単体版 — 引き継ぎ資料

- 作成日: 2026-10-05
- 作成元: combomgr(tacpendium 本家・非公開リポジトリ)での相談セッション(読み取り専用・実装なし)
- 根拠コミット: combomgr の 2026-10-05 時点のコミット(本資料中の行番号・数値はこの時点。SHA は公開前監査 S7 で伏せた)
- 宛先: 新リポジトリでの **次セッション以降の実装担当**
- 本資料の位置づけ: 新リポジトリの最初の正本。次セッションはまず本資料を読み、§9 の S0 から着手する

---

## 0. 一言で

**SF6 のキャラと「ダウン後の有利フレーム(KA)」を入れると、セットプレイのレシピ(空振りの繋ぎ → 重ねる技)が出る Web サービス**を、本家 tacpendium から切り出して **無料・課金事故なし** で公開する。

**構成の核**: サーバで計算しない。**CI がマスタデータから全パターンを事前計算し、静的ファイルとして Cloudflare から配る**。「誰かが計算済みなら再計算しない」の最も強い形(＝実行時には誰も計算しない)。

---

## 1. 開発者の決定事項(確定・再議論しない)

| # | 項目 | 決定 |
|---|---|---|
| D1 | 機能範囲 | 本家のセットプレイ提案を**フィルタ含めそのまま**(meaty / gap / filler(安全飛び)、全絞り込み) |
| D2 | データの正本 | **本家**。取込元は **公開版 tacpendium**(public)。combomgr(非公開)は取込元にしない |
| D3 | 計測 | **閲覧数ではなく実行数を細かく**。キャラ・KA・モード・絞り込み・言語・結果件数 等 |
| D4 | ドメイン | **無料**(`*.workers.dev` / `*.pages.dev`)で可 |
| D5 | UI 言語 | 日・英。**ラベル・操作説明のみ**中国語(簡体・繁体の両方)・韓国語も |
| D6 | 技の表記 | **日本語技名 / 英語技名 / SRK / ナンバリング を切替可能**。英語 UI の既定は **SRK** |
| D7 | 英語技名 | **`move_code` から規則で生成**(+少数の上書き表) |
| D8 | プリセット | 本家の preset 機構は使わず、**1 つの moves DB(キャラ別 JSON)に 4 表記を持たせる**。**カスタムプリセット非対応** |
| D9 | 強化系の表示 | 本家でプリセットの一意制約のせいで表示できなかった技(強化版等)を**こちらでは表示できるようにする** |
| D10 | KA 上限 | **0〜100**。超える人は本家 or **ローカル実行版**(本リポジトリから配布)へ |
| D11 | キャッシュ | 他ユーザーが実行済みの計算はしない(→ 事前計算で実現) |
| D12 | 濫用対策 | 連打・1 ユーザー大量実行・想定外経路の大量実行に備える |
| D13 | 費用 | **途中から課金・停止が起きない**こと |
| D14 | 利用者数推定 | IP + 日替わりソルトのハッシュ(生 IP 非保存)。**法律・規約上問題ない範囲で**(§7.3) |
| D15 | 中国語 | 簡体・繁体の両方(本資料 §6 の方式) |
| D16 | コピー(2026-10-06 追加) | 結果一覧の**各レシピにコピーボタン**。選択中の表記(D6)でテキストにしてクリップボードへ書く |
| D17 | グッド/バッド(2026-10-06 追加) | 結果一覧の各レシピに**グッド/バッドボタン**。票は**全ユーザー共通**で、提案の**並べ替えの第一キー**にする。票が無い・同点のときは本家の順位に従う。保存先・排他・スコア式などは §12-6 で後日決める |

---

## 2. 本家の現状(移植元の事実)

### 2.1 ファイル(本リポジトリでは `_reference/combomgr/` にスナップショットを置いた。§10。**S7 で削除**。移植元は公開版 tacpendium＝`.upstream/tacpendium`)

| 役割 | 本家パス | 備考 |
|---|---|---|
| エンジン | `internal/service/setplay/setplay.go` | **標準ライブラリのみ・純関数・決定的**。別 repo `autopilot-combomgr/projects/setplay-suggestion` から移植されたもの |
| グルー | `internal/service/setplay/service.go`(約 1,200 行) | `SuggestForCombo` / `suggestFillerOnly` / `collectTargets` / `buildFillerUnits` / `rank` |
| 絞り込み・重複除去 | `refine.go` | `refineProposals`(接近の手 / ガード有利 / 連打キャンセル / 重複 / 隣接並べ替え) |
| 連打キャンセル表 | `rapid.go` | |
| 接近の手 | `approach.go` | **キャラ別にコードへハードコード**(`characterApproachMoves`)→ データ化する |
| キャンセル | `cancel.go` | ctx キャンセル。単体版では Worker 内の中断に置換 |
| テスト | `*_test.go`(約 3,400 行。`golden_test.go` 含む) | **適合性テストの土台**にする |
| リポジトリ | `internal/repository/setplay/repository.go` | SQL 3 本。単体版では JSON 読込に置換 |
| API | `internal/api/setplay/`(`GET /api/combos/:comboId/setplay-suggestions`) | 単体版では不要(参考のみ) |
| UI | `web/src/features/setplay/`(`SetplaySuggestionSection.tsx` 46KB 他) | **作り直す**(参考のみ) |
| 文言 | `web/src/locales/{ja,en}.json` の `setplay.*` | 流用可 |

グルーの本家内依存: `internal/model`(定数)/ `internal/repository/combo`(KA 取得)/ `internal/repository/setplay`(型)/ `internal/service/setup`(`alreadyAdopted` 用ハッシュ)/ `internal/testutil/dbtest`(テスト)。**単体版では `model` の必要定数だけ残し、他は除去**。

### 2.2 入出力

- 入力(本家): comboId → そこから **KA**(`knockdown_advantage`)と `character_id` のみ使用。クエリ: `mode=meaty|gap|filler`, `n_min`, `n_max`, `g_min`/`g_max`(1–13), `remain`, `target_types`, `target_move_id`, `include_zero_damage`, `sort=n|target`, `limit`(既定 200・上限 3000), `approach_only`, `block_safe_only`, `chain_cancel_only`
- 単体版の入力: **キャラ + KA + 上記クエリ**(comboId 不要)
- 出力: `{items[], truncated, totalFound, reason}`。item = 順序付き `steps[]`(`moveId, code, role: filler|target|parent, counted`)+ `s, n, landing(=KA+1), targetActive, mode, g, remain`
- **`alreadyAdopted` は単体版で削除**(ユーザーデータ依存はこれだけ)。「採用」ボタンは「コピー」に置換
- エッジ: KA < 0 → `reason=knockdown_advantage_negative` / filler で remain ≥ KA → `reason=remain_not_below_knockdown`

### 2.3 アルゴリズムの要点(事前計算設計に効く事実)

- エンジン `Suggest(KA, fillers[], target, opt)` は**当てる技(target)1 つごとに独立**に走る(`service.go:420-450` のループ)
- 成立条件: `NMin ≤ N ≤ target.Active`、`N = KA + 2 − S`、`S = Σfiller.total + target.startup`
- **target 側の絞り込み**(`target_types` / `target_move_id` / `include_zero_damage`)は `collectTargets` で**エンジン前に target を選ぶだけ**
- **`approach_only` / `chain_cancel_only` / `block_safe_only` / 重複除去**は `refineProposals` で**エンジン後**
- `n_min/n_max`・`g_min/g_max` は**受理帯を狭めるだけ**(`meatyBand` / `gapBand`)
- **filler(安全飛び)モード**はダミー target(`startup = remain+1, active = 1`)で走らせる ⇒ **Σfiller = KA − remain ちょうど**の列挙
- 上限: `engineSafetyCap = 6000`(target あたり)/ `maxChainUnits = 2000` / `maxChainLen = 8` / MaxFillers 8。到達で `truncated=true`

⇒ **仮説(S1 で検証)**: どのモードも本質は「**繋ぎ(filler)の組み合わせで合計 T フレームちょうどになるもの**」の表引きに分解できる。
- meaty/gap: target t・N について `T = KA + 2 − N − t.startup`
- filler: `T = KA − remain`
- **T はキャラごとに 0〜約 101 しか取らず、KA に依存しない**

### 2.4 実測コスト(本家 `docs/progress/M47-RESEARCH-01-report.md` B-1)

| KA | 応答 |
|---|---|
| ≤ 40 | ≤ 20 ms |
| 60 | ≤ 131 ms |
| 80 | 最大 1.4 s(**31 キャラ中 18 で打ち切り発生**) |
| 120–150(全種別) | 4–4.7 s |

### 2.5 データ

- 正本: `character_data/*.csv`(31 キャラ・約 2,740 技行)。列: `character_code, move_code, category, name_ja, startup, active, recovery, total, on_hit, on_block, damage, is_aerial, is_projectile, is_derived, notes, notes_tool, original_move_code, command, condition_ja, condition_en, startup_basis, chain_cancel_total, fastest_unreachable, last_changed_game_version, first_hit_startup`
- 他に必要: 派生関係(本家 `migrations/000006_data_seed_move_derivations`)、安全飛び値(`000010_data_seed_safe_jump`)、キャラ名と状態名(`000003_data_seed_characters` の `name_ja/name_en/custom_states`)、データ修正(`000011` / `000013`)
- 表記の照合用: `000008` / `000012` / `000014` の `preset_aliases`(preset 3 = ナンバリング、preset 5 = SRK、preset 1 = 公式日本語)
- **英語技名は本家に無い**(`alias_text_en` は移動系の共通語 80 行のみ)

### 2.6 本家の表示問題(D9 の対象)

- 原因: `preset_aliases` の**一意制約** `UNIQUE(preset_id, character_id, alias_text)`。本家は「表記 → 技」の**逆引き**を行うため一意が必要
- 結果: 表記が被る技(空中版・【強化】版・ホールド段階・Lv 段階・別技の同入力)は、片方しか表記を持てない
- 本家記録: `followup-backlog` の `preset-alias-duplicate-display` / `seed-alias-collision-change-unfiled`
  - `M48-RESEARCH-01` 時点で被り 75
  - `M48-08` で「被ったら優先の 1 つだけ残す」方式になった
- **単体版は逆引きしない(表示専用)ため一意制約は不要。** 状態印を付けて区別する(§4.3)

---

## 3. アーキテクチャ

```
[公開版 tacpendium: character_data/*.csv + migrations の data_* SQL]   ← 正本(D2)
        │ GitHub Actions が取得(public なのでトークン不要)。コミット SHA を固定・記録
        ▼
[GitHub Actions]
  ① 変換:   CSV/SQL → moves DB(キャラ別 JSON・4 表記入り)+ 検査
  ② 事前計算: エンジン(Go)で KA 0〜100 の全パターン → 静的 JSON
  ③ 適合性:  本家と同じ入力で同じ結果になることをゴールデンテストで確認
  ④ ビルド:  フロント(Vite)
        ▼
[Cloudflare(Workers の静的アセット or Pages)・カード未登録の無料プラン]
  /                                  画面(5 言語)
  /data/<dataHash>/moves/<char>.json  技データ        ← immutable キャッシュ
  /data/<dataHash>/sp/<char>/...      事前計算結果     ← immutable キャッシュ
  /api/beacon                        実行計測の受け口(動くコード)
  /api/vote(D17・名前は仮)          グッド/バッドの受け口と票の配信(動くコード。保存先は §12-6、候補は D1)
        ▼
[Workers Analytics Engine] → 集計ダッシュボード
```

★2026-10-06(D17)で、実行時に書き込む処理が 1 つ増えた(全ユーザー共通の票)。**提案そのものは引き続き静的配信**で、票の受け口が止まっても提案は止まらない(票なしの並び＝本家の順位で出る)ようにする。
票の保存先は未決(§12-6)。KV は §3.1 のとおり書込 1,000 回/日で不採用のまま。候補は Cloudflare D1(SQLite)で、無料枠と超過時の挙動は S1 ⑦で確かめる。

### 3.1 採らない案と理由(再検討不要)

| 案 | 不採用理由 |
|---|---|
| サーバで実行時計算(Cloud Run 等) | 課金アカウント必須で課金事故の芽(D13) |
| Workers で実行時計算 + 共有キャッシュ | 無料プランの CPU 上限は **10ms/リクエスト**、KA 60 で 131ms のため不可 |
| ブラウザ計算結果を共有キャッシュへ投稿 | **送られた結果を検証できない(キャッシュ汚染)** |
| KV / R2 | KV は書込 1,000 回/日、R2 はカード登録必須 |
| ブラウザから毎回 GitHub raw を取得 | 回数制限・可用性依存・変換と事前計算ができない |
| GitHub Pages | 帯域に月 100GB の目安(停止リスク)、レスポンスヘッダ設定不可 |

### 3.2 事前計算の 2 案(S1 で実測して選ぶ)

| | 案 A: 結果ファイル方式 | 案 B: 繋ぎ表方式(§2.3 の仮説) |
|---|---|---|
| 置くもの | `(キャラ, KA)` ごとに全 target の結果(受理帯は最大)。31 × 101 ≒ 3,100 ファイル | キャラごとに「合計 T ちょうどの繋ぎ組み合わせ表」(T = 0〜約 101)。31 ファイル |
| ブラウザの仕事 | 絞り込み・並べ替え・件数制限 | target と N から T を引いて結合 → 絞り込み・並べ替え |
| 長所 | 本家とほぼ同じ処理系 | 容量・ファイル数が圧倒的に小さい見込み。KA 上限の拡張も容易 |
| 短所 | 容量が大きい可能性(高 KA で大量) | **打ち切り(cap)の意味が本家と一致しない可能性**。結合ロジックの移植が要る |

- 共通: 絞り込み・並べ替えのロジック(`refine.go` / `rank`)は Go のまま **WASM** で動かし(§5)、本家と同一挙動を保つ
- Cloudflare 無料プランの上限: 静的アセット 20,000 ファイル・1 ファイル 25MiB(**要確認**)

---

## 4. moves DB(D6〜D9)

### 4.1 形(案)

キャラ別 JSON:

```json
{
  "dataVersion": "<tacpendium SHA>",
  "character": { "code": "ryu", "name": { "ja": "リュウ", "en": "Ryu" } },
  "states": [ { "code": "denjin_charge", "name": { "ja": "電刃錬気", "en": "Denjin Charge" } } ],
  "moves": [
    {
      "code": "denjin_charge_hadoken", "category": "special",
      "startup": 16, "active": 1, "total": 46, "onBlock": -6, "damage": 800,
      "isDerived": false, "isAerial": false, "isProjectile": true,
      "chainCancelTotal": null, "fastestUnreachable": false, "parent": null,
      "state": "denjin_charge",
      "notation": { "nameJa": "[電刃錬気]波動拳", "nameEn": "Denjin Charge Hadoken",
                    "srk": "[Denjin] 236P", "numeric": "[電刃] 236P" }
    }
  ],
  "safeJump": [ ... ],
  "approachMoves": [ ... ],
  "rapidCancel": [ ... ]
}
```

- **数値は例**。キーは `move_code`(本家の数値 ID は使わない)
- `approach.go` / `rapid.go` のハードコード表もここへデータ化する

### 4.2 4 表記の生成

| 表記 | 生成元 |
|---|---|
| 日本語技名 | CSV `name_ja` そのまま |
| 英語技名 | `move_code` から規則生成 + 上書き表 `overrides/name_en.json` |
| SRK | CSV `command` 列から生成。normal は `code` の `standing_` / `crouching_` / `jumping_` から `st.` / `cr.` / `j.` |
| ナンバリング | CSV `command` 列から生成(例: `d dr r plus p_l` → `236LP`、`chain` → `>`) |

英語技名の規則例:
- `hadoken_light` → `Light Hadoken`
- `_od` → `OD …`
- `aerial_` → `Aerial …`
- `_2hits` → `(2 hits)`
- 単語は Title Case

SRK とナンバリングの取り扱い:
- 本家 `preset_aliases`(preset 5 / 3)は**照合用オラクル**として CI で突合し、食い違いを一覧化する
- 一致率は S1 で実測する
- 方針として「生成を正、本家と食い違ったら人が判断して上書き表へ」

### 4.3 強化系の区別(D9)

- 状態つきの技は CSV `condition_ja` / `condition_en` とキャラの `custom_states` から**状態印**を合成する(例: `[Denjin] 236P`)
- **CI 検査**:
  - 全技が 4 表記を持つこと
  - **同一キャラ × 同一表記種で、表示文字列が完全一致する技が 0 件**であること
  - 一致が残れば状態印・空中印などの区別規則を足す

### 4.4 表示

- 4 表記をトグルで切替。既定は ja UI = 日本語技名、それ以外 = SRK
- 中国語・韓国語 UI でも技表記は上の 4 択(技名は翻訳しない)

---

## 5. エンジンとフロント

- **エンジン**: Go のまま新リポジトリの `engine/` パッケージへ切り出す(DB・ctx・ユーザー依存を除去)
  - 用途 1: 事前計算 CLI(CI で実行)
  - 用途 2: **WASM**(ブラウザで絞り込み・並べ替え・案 B の結合、ローカル版では計算そのもの)
  - サイズ: 標準 Go は gzip 後 約 1MB、TinyGo なら数百 KB の見込み(要実測)
  - 本家ゴールデンテストを移植し、事前計算と WASM の両経路で通すこと
- **フロント**: React + TypeScript + Vite(本家と同じ系統)
  - 画面: キャラ選択 / KA 入力(0〜100)/ モード / 絞り込み / 表記切替 / 結果一覧 / コピー
    - コピー(D16): レシピごとのボタン。選択中の表記でテキスト化し `navigator.clipboard.writeText` で書く(§7.4 の Permissions-Policy の注記を参照)
    - グッド/バッド(D17): レシピごとのボタン。並べ替えは「票のスコア → 本家の順位」の順。票の取得に失敗しても一覧は本家の順位で出す
  - 処理: Web Worker で WASM を実行し、メインスレッドを固めない
  - 状態: 同一条件はメモリキャッシュから即表示。URL クエリで条件共有(読込時に zod で検証)
- **ローカル版(D10)**: 同じフロント + WASM で、**KA 上限を外したビルド**
  - 静的配信用の小さな Go 単一バイナリ(embed)として GitHub Releases で配布
  - 計算はブラウザ内(事前計算なしで都度計算)。Worker と中断ボタン付き

---

## 6. 多言語(D5・D15)

- react-i18next。ロケールは `ja` / `en` / `zh-Hans` / `zh-Hant` / `ko`
- 翻訳対象は UI ラベル・操作説明のみ
- **中国語**: `zh-Hans` を 1 本書き、`zh-Hant` は**ビルド時に OpenCC(`s2twp`:台湾の語彙にも変換)で自動生成**
- 自動選択: `navigator.language` が `zh-CN` / `zh-SG` なら簡体、`zh-TW` / `zh-HK` / `zh-MO` なら繁体。手動切替あり
- 中国語・韓国語には「機械翻訳です。改善提案歓迎」の表示と Issue へのリンクを付ける
- 本家 `ja.json` / `en.json` の `setplay.*` を出発点にする。本家にある言語テスト(`no-japanese-in-en.test.ts` 等)の考え方を流用する

---

## 7. 計測・セキュリティ・費用

### 7.1 実行計測(D3)

- 「提案」実行ごとに `POST /api/beacon` を送る(`navigator.sendBeacon`)
- 送る中身: `{char, ka, mode, nMin, nMax, gMin, gMax, remain, targetTypes, filters, notation, lang, resultCount, truncated, cacheHit, ms, dataVersion}`
- Worker は**厳格に検証してから**(列挙値・範囲・1KB 以下)Workers Analytics Engine に書く
- 集計は SQL API で取る。ダッシュボードは Grafana Cloud 無料 か、Cloudflare Access で保護した自作ページ
- 無料枠を超えても**止まるのは計測だけで、提案機能は静的配信なので無影響**
- 混雑時はクライアント側のサンプリング率(静的設定ファイル)を下げる

### 7.2 濫用対策(D12)

| 想定 | 対策 |
|---|---|
| 連打 | 実行中はボタン無効化。同一条件はメモリキャッシュ |
| 大量取得(スクリプト) | 静的配信なので費用ゼロ。中身は CC-BY-SA のデータ相当(データセットとして公開も可) |
| 計測の水増し・枯渇 | Worker で検証。**Workers の回数制限機能(要確認:無料プランで使えるか)**。集計側で外れ値除去 |
| 投票の水増し・連打(D17) | cookie・ストレージを使わない(§7.3)ため、重複は「`hash(IP + UA + 日替わりソルト)` × レシピ × 日」で 1 票に抑える案。Worker 側の回数制限。票は並びを変えるので、水増しの影響は計測より大きい(詳細は §12-6) |
| 想定外経路(直接 POST 等) | 実行時の計算が無いため、提案機能には攻撃対象が無い。受け口(beacon・投票 D17)は Worker で厳格に検証する |
| 無料ドメインの制約 | `*.workers.dev` / `*.pages.dev` は WAF ルールを置けない ⇒ 回数制限は Worker コード側で行う |

### 7.3 プライバシー(D14)

- 利用者数の推定: `hash(IP + UA + 日替わりソルト)` を記録し、ソルトは翌日破棄。生 IP は保存しない
- クッキー・ブラウザストレージは使わない
- **プライバシーポリシーのページを用意**し、計測項目を明記する。**投票(D17)で記録するもの**(重複防止のハッシュ等)も同じページに書く
- 法的助言ではないため、公開前に以下と照合する
  - 個人情報保護法の「個人関連情報」の扱い
  - GDPR の仮名化データの扱い
  - Cloudflare の規約

### 7.4 配信のセキュリティ

- レスポンスヘッダ:
  - CSP: `default-src 'self'; script-src 'self' 'wasm-unsafe-eval'; connect-src 'self'; frame-ancestors 'none'`
  - `X-Content-Type-Options: nosniff`
  - `Referrer-Policy: strict-origin-when-cross-origin`
  - `Permissions-Policy`(全拒否)。ただし `clipboard-write` は拒否するとコピー(D16)の `writeText` が失敗しうるため `clipboard-write=(self)` のみ許可する(S5 で実ブラウザで確かめる)
- `dangerouslySetInnerHTML` 禁止
- 入力の上限: KA 0〜100、`limit` ≤ 3000(本家と同じ)
- サプライチェーン:
  - lockfile と Dependabot/Renovate
  - GitHub Actions は `permissions: contents: read` を既定にし、Action は**コミット SHA 固定**
  - main を保護し、2FA を必須にする
- デプロイ: Cloudflare の GitHub 連携(秘密情報なし)、またはスコープを絞った API トークン

### 7.5 費用(D13)

- **Cloudflare にカードを登録しない**(無料プランのまま自動課金されない)
- 静的配信は量無制限。動く Worker は計測と投票(D17)だけ。どちらも無料枠を超えたら止まるだけで、提案は止まらないこと(投票の保存先の挙動は S1 ⑦で確認)
- GitHub Actions は public リポジトリなら無料・無制限
  - **リポジトリを private で始める期間は月 2,000 分の制限**があるので、事前計算の実行時間に注意

---

## 8. ライセンス・権利

- **コード**: 本家エンジンを流用するため **AGPL-3.0-or-later**。画面フッターにソースへのリンクを置く
- **データ**: **CC-BY-SA-4.0**。事前計算結果も派生データとして同ライセンス。クレジット表示を付け、本家 `NOTICE` の削除要請対応方針を引き継ぐ
- **公式データの混入禁止**: 本家の法務判断(手入力データのみ公開)を踏襲。取込元は公開版 tacpendium の `character_data/` と data SQL のみ
- **商標**: サイト名・ドメインに「Street Fighter」「SF6」を使うか、ロゴ・画像を使うかは Capcom のガイドラインを確認してから決める。「非公式」を明記する

---

## 9. 作業計画(次セッション以降)

| ID | 内容 | 完了条件 |
|---|---|---|
| **S0** | リポジトリ整備: `AGENTS.md`/`CLAUDE.md`(本家から必要分のみ移植・§11)、ライセンス一式、CI 骨格、`.gitignore` | CI が空で緑 |
| **S1 RESEARCH** | 実測のみ・実装しない: ①案 A の総容量と CI 時間(KA 0〜100)②案 B の表サイズと、本家結果との一致(打ち切り含む)③`command` から生成した SRK/ナンバリングと preset の一致率 ④英語技名の上書き件数 ⑤公開版 tacpendium に必要ファイルがそろっているか ⑥Cloudflare 無料枠の値(静的アセットのファイル数・Analytics Engine・回数制限)⑦(2026-10-06 追加・D17)票の保存先の候補 D1 の無料枠(読み書き回数・容量)と**超過時の挙動**(止まるだけか・課金に移るか＝D13)⑧(同)票を並べ替えの第一キーにしたときの**打ち切り(cap)への影響**。「並べ替えの後で切る」を守るには、票で上がりうる候補を事前計算で落とせない。案 A/B の評価に含める | レポートと、案 A/B の推奨 |
| S2 | データパイプライン: tacpendium 取得 → moves DB → 4 表記 → CI 検査(§4.3) | 31 キャラ分の JSON と検査が緑 |
| S3 | エンジン切り出しと WASM 化、ゴールデンテスト移植 | Go と WASM の両方でテストが緑 |
| S4 | 事前計算 CLI と CI 組込み、データ版ハッシュ付きの出力 | 生成物が上限内 |
| S5 | フロント(画面・絞り込み・表記切替・5 言語・Worker・URL 共有・コピー(D16)) | 主要フローの E2E が緑 |
| S6 | 計測と投票(beacon Worker・Analytics Engine・ダッシュボード・プライバシーポリシー・グッド/バッド(D17)の受け口と保存) | 実行ごとのイベントが見える。票が保存され並べ替えの第一キーに効く(票なしの並びは本家と一致＝ゴールデンテストが緑のまま) |
| S7 | 公開前監査: ヘッダ/CSP、依存、Actions 固定、**`_reference/` 削除**、private 由来の記述の除去 → public 化 | 外部ヘッダ診断で問題なし、チェックリスト完 |
| S8 | ローカル版(KA 無制限・単一バイナリ・Releases) | 3 OS で起動確認 |
| S9 | 限定公開 → 一般公開 | |

**データ更新の運用**:
- 定期(例: 日次)の Actions で tacpendium の HEAD を確認し、変化があれば再生成・再デプロイ
- データ版を画面に表示する
- 本家のバランスパッチ対応はそのまま流れてくる

---

## 10. 新リポジトリの初期構成

```
<repo>/
├── docs/handover/HANDOVER-001-setplay-standalone.md   ← 本資料
├── _reference/combomgr/            ← 本家スナップショット(読むだけ。Go は `_` 始まりのディレクトリをビルドしない)。S7 で削除
│   ├── internal/service/setplay/   (エンジン・グルー・テスト)
│   ├── internal/repository/setplay/
│   ├── internal/api/setplay/
│   ├── internal/model/
│   ├── internal/service/setup/duplicate_keys.go
│   ├── web/src/features/setplay/
│   ├── web/src/locales/{ja,en}.json
│   ├── character_data/             (S1 の実測用。本番の取込は tacpendium から)
│   ├── migrations/                 (data_* と 000001/000003/000007 のみ)
│   ├── docs/                       (M47-RESEARCH-01 報告・M47/M19 指示書・技の調査メモ)
│   └── claude/                     (本家の AGENTS.md・CLAUDE.md・.claude/settings.json・hooks。S0 で取捨)
├── LICENSE / LICENSES/ / DATA-LICENSE.md / NOTICE / REUSE.toml   (本家からコピー。S0 で内容を単体版向けに修正)
└── README.md                        (S0 で作成)
```

---

## 11. 新リポジトリでの運用ルール(S0 で `AGENTS.md` に起こす要点)

- 本家の Git 規律(push は `claude/` ブランチのみ、履歴改変禁止、取り込みの規律)を踏襲するかは S0 で開発者に確認する
  - 本家の `.claude/settings.json` は 8 本の hook を参照している。**hook を移さずに settings だけ移すと壊れる**
- 依存の追加は開発者に提案してから(本家 §6 と同じ)
- `_reference/` は**編集しない**(移植元の記録)。移植したコードは本リポジトリ側の正規の場所に置く(**S7 で `_reference/` を削除した**。以後の移植元は公開版 tacpendium)
- 本資料の §1 決定事項は覆さない。覆す必要が出たら開発者に確認する

---

## 12. 未決事項(開発者確認が要るもの)

1. サイト名・リポジトリ名・商標の扱い(§8)
2. 本家の Git/hook 運用をどこまで移すか(§11)
3. S1 の結果を受けた案 A/B の選択 → **2026-10-07 決定: 案 B**（繋ぎ表方式。詳細と付帯の方針は `docs/progress/S1-report.md`「S1 後の開発者決定」）
4. 計測ダッシュボードの方式(Grafana Cloud / 自作 + Access)
5. 公開版 tacpendium の更新を検知する方式(定期確認 / tacpendium 側からの通知)
6. グッド/バッド(D17)の詳細(2026-10-06 追加。開発者の方針は「おいおい決める」)
   - 票の保存先(D1 か、ほかの方式か)・同時書き込みの排他
   - スコアの式(good − bad / 比率 / Wilson 下限 など)
   - レシピの同一性キー(データ版が変わっても同じレシピを同じ票に結び付ける鍵。例: キャラ + 技コードの列)
   - 票の単位(KA ごとか、KA をまたいで同じレシピに集めるか)
   - 水増し対策の強さ(§7.2)

---

## 13. 公開前チェックリスト(S7)

- [ ] `_reference/` を削除した(**本家の非公開 docs 由来の記述が残らない**こと)
- [ ] 本家 combomgr の**履歴を含めていない**(本リポジトリは履歴なしで新規作成済み)
- [ ] `private` / `autopilot-combomgr` / 友人限定リポジトリ・公式 dist への言及が無い(grep)
- [ ] LICENSE(AGPL)・DATA-LICENSE(CC-BY-SA)・NOTICE・クレジット・非公式表記
- [ ] プライバシーポリシー・計測の説明
- [ ] セキュリティヘッダと CSP の確認(外部診断)
- [ ] Cloudflare にカード未登録・Worker の回数制限が有効
- [ ] Actions がコミット SHA 固定・権限は最小
- [ ] GitHub のリポジトリ設定(Dependabot alerts / security updates、secret scanning と push protection、main のブランチ保護と必須 check、Actions の権限、2FA)を公開に合わせて設定した。**本家で実施済みの作業**なので、S7 では開発者が combomgr か公開版 tacpendium をセッションに接続し、本家の公開時チェックリストを参考にする。設定の変更は開発者が行う(Claude は手順の確認と照合だけ)
- [ ] KA 0〜100 で全キャラの生成物が存在し、ゴールデンテストが緑

*以上*
