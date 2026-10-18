# S6 計測と投票 — 完了報告

- 段階開始コミット: `b386ea0cff0f1c449007b27c38a4e312f9e98bae`
- ブランチ: `claude/stage-6-93v2ao`

## 計画

### Context

HANDOVER §9 S6「計測と投票（beacon Worker・Analytics Engine・ダッシュボード・プライバシーポリシー・グッド/バッド(D17)の受け口と保存）」。
**完了条件: 実行ごとのイベントが見える。票が保存され並べ替えの第一キーに効く（票なしの並びは本家と一致＝ゴールデンテストが緑のまま）。**

読んだもの: HANDOVER §0・§1・§3・§7.1〜§7.3・§9・§12、S5 報告（とくに「次の段階への入力」と「S5 後の開発者決定」）、S1 報告 ⑥⑦⑧（Cloudflare・D1 の無料枠、票と打ち切り）。

前提:
- デプロイ・Cloudflare のアカウント作成・本番公開は**しない**（AGENTS §3-3）。Worker のコードと設定（`wrangler.toml` の雛形）までを作り、手元と CI では Node の上で Worker の本物のコードを動かして試験する（D1 は `node:sqlite` で同じ SQL を流す。Analytics Engine は書いた点を覚える偽物）。
- S5 後の開発者決定 2: **最初に繋ぎ表の仕組みを消す**。

### 開発者決定（2026-10-12、S6 の冒頭。AskUserQuestion で確認）

| 事項（HANDOVER §12） | 決定 |
|---|---|
| §12-4 ダッシュボード | **集計スクリプト**（Analytics Engine の SQL API を叩き、表を出す。手元の 1 コマンドと、手動起動の Actions の job summary）。API トークンは開発者が GitHub の Secrets に置く予定（リスクの説明を求められた → 本報告と `docs/telemetry.md` に書く） |
| §12-6 票の保存先・配信 | **D1 ＋ Worker から配信**（`GET /api/votes?char=&ka=`。主キーの索引で、そのキャラ・KA の票の行だけ読む）。開発者注: 利用者は少なく、超正確な数値は要らない（→ 排他・水増し対策は簡素でよい） |
| §12-6 スコアの式 | **good − bad**。票なし＝0。同点・票なしは本家の順位 |
| §12-6 票の単位 | **キャラ＋KA ごと**。レシピの鍵はキャラ・KA の中で「繋ぎの技 code の列＋当てる技 code」（データ版が変わっても技 code が同じなら同じ票） |
| §12-6 水増し対策 | §7.2 の案: `hash(IP + UA + 日替わりソルト)` × レシピ × 日で 1 票（同じ日は付け替え・取り消し可）、1 人 1 日の票数に上限 |

### 作るもの

| 場所 | 中身 |
|---|---|
| 片付け | 繋ぎ表の仕組みを消す（`internal/service/setplay/table.go`・試験、`engine` の `LoadTable`、WASM の `loadTable`、`cmd/precompute` の表の生成と `-tables`、CI の sweep、WASM・Node の表あり経路、docs）。`verify-plan.py ack` で記録 |
| エンジン | `SuggestParams.Votes`（レシピの鍵 → スコア）。並べ替えの第一キーを票にし、**並べ替えの後で limit を切る**。各提案に `recipeKey` を出す（画面が投票に使う鍵の正本を Go の 1 か所にする）。票が無ければ並び・中身は本家と同じ |
| WASM | `suggest(code, ka, query, votesJSON?)` |
| `worker/`（Cloudflare Worker、TypeScript） | `POST /api/beacon`（厳格に検証して Analytics Engine に 1 点）、`POST /api/vote`（検証・重複防止・D1）、`GET /api/votes`。それ以外は静的アセット。`schema.sql`・`wrangler.toml`（雛形。ID は開発者が入れる） |
| 画面 | 実行ごとに `sendBeacon`（§7.1 の項目。抽出率は静的な `config.json`）。計算の前に票を取り（失敗・時間切れなら票なしで計算）、Worker に渡す。各行にグッド/バッド。プライバシーポリシー（5 言語） |
| 集計 | `scripts/analytics/`（SQL API → 表）と `.github/workflows/analytics.yml`（`workflow_dispatch` のみ） |
| docs | `docs/telemetry.md`（計測・投票・集計・トークンの扱い）、`docs/engine.md`・`docs/frontend.md`・`docs/precompute.md` の更新 |

### 回す検査

- Go: `go test -count=1 ./...`（golden・本家との突き合わせが票なしで緑のまま）、票の試験（票で上に来る・limit の外の行が票で入る・票なしは不変）、`go vet`、gofmt、WASM の試験と Node の通し
- `worker/`: `tsc --noEmit`・vitest（検証・重複防止・上限・D1 の SQL を `node:sqlite` で）
- `web/`: tsc・vitest（i18n のキー集合・日本語混入を含む）・E2E（画面 → beacon → Worker → 記録まで、投票 → 保存 → 再計算で第一キー、の通し。§8-5）
- 集計スクリプトの単体試験（偽の SQL API）
- §8-3: 新しい検査ごとに、守りたい不具合を一度わざと起こして赤を確かめる

### 未決事項と扱い

| 事項 | 扱い |
|---|---|
| 投票した直後に並びを変えるか | 自分で決める（推測: 変えない。押した行が動くと分かりにくい。次の計算から効く） |
| 票の取得が遅い・失敗したとき | 自分で決める（推測: 短い時間切れで票なしの並び＝本家の順位で出す。HANDOVER §3「票の受け口が止まっても提案は止まらない」） |
| beacon の回数制限 | 自分で決める（推測: Worker の検証＋isolate 内の簡易な回数制限＋抽出率。Workers の Rate Limiting は S1 で無料可否が不明なので使わない） |
| 日替わりソルトの置き場 | 自分で決める（推測: D1 に日付ごとの乱数を置き、前日以前は消す。秘密情報を要らなくする） |
| プライバシーポリシーの法的照合（§7.3） | 文案まで。照合は公開前（S7）に開発者 |
| デプロイ | しない（S7 以降・開発者） |

## やったこと

成果物（基準 `b386ea0` からの全体の diff は末尾）:

- **繋ぎ表の仕組みを削除**（S5 後の開発者決定 2）: `internal/service/setplay/table.go`・試験、`engine` の `LoadTable`、WASM の `loadTable`、`cmd/precompute` の表の生成と `-tables`、CI の sweep（事前計算 job・`verify-plan.py` の種類 `sweep`）、WASM・Node の表あり経路、docs（`precompute.md`・`engine.md`・`upstream-diff.md`・`frontend.md`）。`service.go` は本家と同じ `runEngine(...)` の呼び出しに戻った。`verify-plan.py ack --none` で記録。
- **エンジンの票**（D17）: `SuggestParams.Votes`・`RecipeKey`（`internal/service/setplay/votes.go`）。本家の並べ替えの後にスコア（good − bad）の降順で安定に並べ直し、**その後で limit を切る**。各行に `recipeKey`。WASM の `suggest` に 4 番目の引数（票の JSON）。票が空なら並びも中身も本家と同じ。
- **Cloudflare Worker**（`web/edge/`）: `POST /api/beacon`（zod で検証 → Analytics Engine に 1 点。列の順は `beacon-layout.json`）、`POST /api/vote`・`GET /api/votes`（D1。`schema.sql`）、日替わりソルトと利用者のハッシュ（D1 に置き、日付が変わった後の最初の要求で前日分を消す）、isolate の中の回数制限、Origin の検査。`web/wrangler.toml`（雛形。`database_id` はゼロ）。
- **画面**: 実行ごとに `sendBeacon`（結果・誤り・中断・キャッシュのどれも。抽出率は `public/config.json`）。計算の前に票を読み（1.5 秒で打ち切り、読めなければ票なし）WASM に渡す。各行に 👍 / 👎（押した票は次の検索から効く）。プライバシーポリシー（5 言語・フッターと `#privacy`）。
- **集計**（§12-4 の決定）: `web/scripts/analytics/report.mjs`（SQL API → Markdown の表。KA 80 以上の割合・KA 帯の待ち時間・キャラ・モード・言語/表記・誤り・0 件・票）と `.github/workflows/analytics.yml`（手動起動・Secrets が無ければ省く）。
- **通しの検査**: `web/scripts/serve.mjs` が Worker の本物のコード（`dist-edge/local.mjs`。D1 = `node:sqlite`、Analytics Engine = メモリ）で `/api` を受け、E2E `web/e2e/telemetry.spec.ts` が「画面 → beacon → Worker → Analytics Engine」「画面 → D1 → WASM → 画面」を通す。`pnpm build` の最後で Worker を webworker 向けに束ね（`dist-edge/worker.mjs` 202,891 bytes）、Node の組み込みに頼らないことを確かめる。
- **docs**: `docs/telemetry.md`（新規。構成・項目・null の扱い・票・ハッシュ・集計・デプロイ手順・トークンのリスク・限界）。

### §8-3 わざと壊して赤を確かめた

| 壊し方 | 赤になった検査 |
|---|---|
| 票を limit で切った後に掛ける | `TestVotes_AppliedBeforeLimit` |
| `applyVotes` を何もしない | `TestVotes_FirstSortKey`・`AppliedBeforeLimit`・`FillerOnly`・`TestApplyVotes_StableAndNoopWhenEmpty` |
| 付け替えで前の票を引かない（二重に数える） | Worker「付ける・付け替える・取り消す」ほか 1 本 |
| null の double を 0 にする | Worker「正しい beacon は…layout の順」 |
| 前日の投票の記録を消さない | Worker「日が変わると…」 |
| ハッシュをやめ IP をそのまま入れる | Worker の IP が残らない試験 2 本ほか 1 本 |
| D1 の誤りで beacon を落とす（レビュー中-1 の直しを外す） | Worker「D1 が誤りを返しても beacon は 204」 |
| 票をエンジンに渡さない／投票後にキャッシュを捨てない／キャッシュの beacon を cacheHit=false | 画面の試験（各 1 本） |
| 票を読み直しても自分の票の数を残す（レビュー低-1） | 画面「投票の後は覚えた結果を捨て…」 |
| main.tsx の sendBeacon を何もしない（配線を 1 か所壊す。§8-5） | E2E「1 回ごとに計測が 1 点届く」 |
| （壊したのではなく実際の不具合）票の配信に `max-age=30` | E2E「票が保存され…第一キー」が赤 → `no-store` に直した |

どれも戻した後に緑。

## 判断と理由

1. **票はエンジン（Go）の中で並べ替えの後・limit の前に掛ける**。案: (a) 画面で 3000 件を並べ直す、(b) エンジン。(a) だと 3000 件より後ろの票の付いた行が入らない（並べ替えの後で切る、に反する）。(b) は純関数・決定的のまま（票は入力）で、golden は票なしの経路なので不変。鍵（`RecipeKey`）もエンジンが作り、画面は受け取った鍵で送る（鍵の作り方を Go の 1 か所にした）。
2. **安定ソートで後から並べ直す**（本家の `rank` を変えない）。本家の差分を当てやすく、票なし・同点は本家の順のまま。
3. **Worker は `web/edge/`**（計画では `worker/`。レビュー低-6）。理由: CI の検査が `package.json` を `web/` だけに許しており、画面と同じ zod の形（`web/src/shared/api.ts`）を共有できるため。Web Worker（`web/src/worker/`）と名前がぶつからないよう edge とした。
4. **D1・Analytics Engine は試験で偽物を使う**（`node:sqlite` に同じ SQL を流す）。アカウントを作らない（AGENTS §3-3）ため。wrangler も足していない（デプロイ時に開発者が使う）。
5. **ソルトは D1 に置く**（秘密情報を要らなくする）。前日分の削除は遅延削除（日付が変わった後の最初の要求）。
6. **票の配信は `no-store`**（E2E で、投票直後の検索が古い票で並ぶ不具合を見つけた）。同じ条件の再表示は画面のメモリのキャッシュが受け、投票するとそのキャッシュを捨てる。
7. **投票してもその場では行を動かさない**（推測: 押した行が動くと分かりにくい）。数と押した状態だけ変える。
8. **beacon は D1 が止まっても書く**（visitor = `unavailable`。レビュー中-1）。null の扱い: double は -1、blob は空文字（§8-7）。
9. **プライバシーポリシーは画面の中の節**（ストレージを使わず `#privacy` で直接開ける）。文面は beacon の全項目を挙げ、削除は「日付が変わった後の最初のアクセス」「基盤の復旧用の記録に残ることがある」と書いた（レビュー中-2・中-3）。
10. **集計の API トークン**は開発者の質問に答える形で `docs/telemetry.md` に書いた（下の「開発者への回答」）。
11. 範囲を広げた点: `cmd/precompute` の試験を表なしの形に書き直した（表あり/なしの比較を「配信 = data/moves を直接読む」に置き換え。KA 標本とクエリはレビュー低-3 で元に戻した）。`web/src/lib/langs.ts` を切り出した（Worker が i18next を読まないため）。

### 開発者への回答: 集計用の API トークンを GitHub の Secrets に置くリスク

（詳細は `docs/telemetry.md`「集計用の API トークンの扱い」）

- 権限は **Account Analytics: Read だけ**にする。漏れても**読まれるだけ**（計測のデータ。生の IP は入っていない）。サイトの書き換え・票の読み書き・課金の変更はできない。起きうる実害は、読み取りの無料枠（1 日 1 万回）を使い切られてその日の集計が失敗すること。課金はされない（カード未登録）。
- Secrets は暗号化され、ログでは伏せられ、フォークからの PR には渡らない。ただしリポジトリに**書き込める人**はワークフローを書き換えて取り出せる（Secrets の安全 = 書き込み権限の管理の強さ）。気になるなら Environment ＋必須レビュアー。
- 漏れたら Cloudflare のダッシュボードでトークンを **Roll / Delete** すれば即座に無効になる。
- デプロイ用のトークン（もし要るなら）は別物で危険度が高い（サイトの差し替え・票の削除ができる）。集計用と分け、できれば Cloudflare の GitHub 連携を使う。

## 残課題・既知の問題

- **本物の Cloudflare で動かしていない**（アカウント無し）。D1・Analytics Engine は偽物で試験した。集計の SQL は docs の関数だけで書いたが実アカウントで流していない。デプロイ時に `report.mjs` を 1 回流して確かめる。
- **D1 の Time Travel で消したソルトを戻せる可能性**（推測・未確認。レビュー中-3）。戻せれば 3 か月残る `visitor` から IP を総当たりで当てうる。**S7 の法的照合（§7.3）の論点**にする。根本対策はソルトを D1 に置かない（秘密値から導く＝秘密情報が要る）ことで、開発者判断。
- プライバシーポリシーは法的助言ではない文案。公開前に個人情報保護法・GDPR・Cloudflare の規約と照合する（§7.3。S7）。
- 回数制限は isolate の中だけ（厳密ではない）。同じ人の同時の投票はずれうる（排他しない。開発者注）。受け口はレシピの実在を確かめない（存在しない鍵はどの行にも当たらない）。
- エンジンの安全上限（target あたり 6000 等）は票より前に掛かる。KA 64 以上で上限に落ちた行は票があっても出ない（S1 ⑧）。
- 投票と票の読み直しが重なったときに表示の数が古くなりうる（再レビュー低-1 の後半。順序の分析のみ・未再現。並びには影響しない）。
- 推定利用者数は、ハッシュを作れなかった実行（空文字・`unavailable`）もそれぞれ 1 人に数える（最大 2 人/日。表の題に注記）。
- D（重い条件の結果を前もって配る）の判断は、デプロイ後の集計（「KA 80 以上の実行の割合」と KA 帯の待ち時間）で行う（S5 後の開発者決定 1）。
- S5 からの持ち越し: キャラの入力で絞り込める選択・リセットボタン（S5 目視の要望）は S6 では扱っていない。
- 手元の E2E は Chromium 141（`PW_CHROMIUM_PATH`）。CI は `playwright install` の版。

## 次の段階への入力（S7 公開前監査）

- **デプロイに要る作業は `docs/telemetry.md`「デプロイの手順」**: D1 の作成（`database_id` を `web/wrangler.toml` へ）、`schema.sql` の適用、`wrangler deploy` か GitHub 連携、集計用トークンと Secrets。どれも開発者の作業（AGENTS §3 の 3〜5）。
- `_headers` は静的アセットだけに掛かる。Worker の応答（`/api/*`）は Worker が `content-type`・`cache-control`・`nosniff` を付ける。ヘッダ診断では `/api` も見る。
- CSP は変えていない（`connect-src 'self'` で beacon・票が通ることを E2E で確かめた）。
- 法的照合の論点: 計測項目（`docs/telemetry.md` の対応表）・日替わりハッシュ・遅延削除と D1 の Time Travel・Analytics Engine の 3 か月保存。
- `verify-plan.py` の種類は `oracle`・`wasm` の 2 つになった（`sweep` は削除）。

## 最終確認（レビュー 2 往復の取り込み後、HEAD `665e821` で回した結果）

出力はファイルへ書き、合否は `EXIT=` と件数で見た。`.upstream` は `bash scripts/fetch-upstream.sh` で取得済み。

```
$ go test -count=1 -v -timeout 35m ./... > /tmp/f-go.txt 2>&1; echo EXIT=$?
EXIT=0
PASS(全階層 ^\s*--- PASS)=348 FAIL=0 SKIP=0
    conformance_test.go:332: 突き合わせた条件: 5208(キャラ 31 ・不一致 0)
$ go vet ./...                         → EXIT=0
$ gofmt -l(追跡中の .go、_reference 除く) → 出力なし
$ go mod tidy -diff                    → EXIT=0
$ GOOS=js GOARCH=wasm go test -exec=scripts/wasm/go-test-exec.sh ./internal/service/setplay/ ./internal/repository/setplay/ ./engine/ → EXIT=0(3 パッケージ ok)
$ bash scripts/wasm/build.sh && node --test scripts/wasm/wasm.test.mjs → EXIT=0  # pass 6  # fail 0
$ cd web && pnpm exec tsc --noEmit     → EXIT=0
$ pnpm exec vitest --run > /tmp/f-vitest.txt 2>&1; echo EXIT=$?
EXIT=0
 Test Files  14 passed (14)
      Tests  146 passed (146)
$ bash scripts/prepare-assets.sh       → EXIT=0(public/data・public/wasm を消してから。キャラ 31)
$ pnpm build > /tmp/f-build.txt 2>&1; echo EXIT=$?
EXIT=0
ファイル: 41(上限 20000) / 最大: wasm/setplay.wasm 3920761 bytes(上限 26214400) / 合計: 6660421 bytes / 違反: 0 件
束ねた: dist-edge/worker.mjs, local.mjs
$ PW_CHROMIUM_PATH=/opt/pw-browsers/chromium-1194/chrome-linux/chrome pnpm exec playwright test > /tmp/f-e2e.txt 2>&1; echo EXIT=$?
EXIT=0
  21 passed (37.9s)
$ python3 scripts/check-licenses.py         → 違反: 0 件
$ bash scripts/check-actions-pinned.sh      → 違反: 0 件
$ python3 -I scripts/ci/verify-plan.py check → 違反: 0 件
$ python3 -I scripts/ci/verify_plan_test.py → EXIT=0
$ bash -n(hooks・scripts・upstream・web/scripts の .sh) → 誤りなし
```

- **完了条件**:
  - 「実行ごとのイベントが見える」: Worker の本物のコードで、実行 1 回ごとに Analytics Engine（偽物）へ 1 点届くことを E2E で確かめた。本番の Analytics Engine への書き込みと集計スクリプトの実行は、デプロイ後（開発者）でないと確かめられない。
  - 「票が保存され並べ替えの第一キーに効く」: D1（`node:sqlite`）に保存され、次の検索で票の付いた行（票なしでは 251 番目）が先頭に来ることを E2E で、limit の外の行が票で入ることを Go の試験で確かめた。
  - 「票なしの並びは本家と一致＝ゴールデンテストが緑のまま」: golden・本家との突き合わせ（5,208 条件・不一致 0）・E2E の本家突き合わせ 9 本が緑。
- **Go の PASS が S5 の 384 から 348 に減った理由**: 繋ぎ表の試験を消したため（開発者決定 2）。消したファイルは `internal/service/setplay/table_test.go`（トップレベルの試験 9 本＋キャラごとの副試験）と `engine/table_test.go`（3 本）。本家突き合わせは「エンジンと繋ぎ表の 2 経路」から 1 経路になり、条件の数が 10,416 → 5,208 になった（同じ条件の表の経路が無くなった分）。足した試験: `engine/votes_test.go`（6 本）・`internal/service/setplay/votes_test.go`（2 本）。それ以外に消した・skip した試験は無い。
- 削除したファイル（`git diff --diff-filter=D --name-only b386ea0 HEAD`）: `engine/table_test.go`・`internal/service/setplay/table.go`・`internal/service/setplay/table_test.go` の 3 つ（どれも開発者決定 2 の対象）。

### CI

- run 38236629157（`a8830e6`、表の削除＋エンジンの票）: success。
- run 38237111197（`08b7200`、Worker）: success。
- run 38238484920（`586d91e`、画面・E2E の通し・docs）: success（Web job の E2E を含む）。
- run 38238312563（`562dfb1`）は次の push で打ち切り（concurrency）。
- run 38239641359（`0c50045`、レビューの取り込み）: success。
- run 38240564775（`665e821`、再レビューの取り込み）: success。

### `git diff --stat b386ea0cff0f1c449007b27c38a4e312f9e98bae HEAD`（HEAD = 665e821。本報告の追記を含まない）

```
 .claude/commands/sync_upstream.md         |   2 +-
 .github/workflows/analytics.yml           |  49 +++
 .github/workflows/ci.yml                  |  62 +--
 .gitignore                                |   4 +
 README.md                                 |   3 +
 cmd/precompute/main.go                    |  84 +---
 cmd/precompute/main_test.go               |  71 ++--
 cmd/wasm/main.go                          |  29 +-
 docs/engine.md                            |  17 +-
 docs/frontend.md                          |  10 +-
 docs/precompute.md                        |  62 +--
 docs/progress/S6-report.md                |  58 +++
 docs/progress/S6-review.md                | 322 +++++++++++++++
 docs/telemetry.md                         | 124 ++++++
 docs/upstream-diff.md                     |  10 +-
 engine/api.go                             |  79 ++--
 engine/conformance_test.go                |  59 +--
 engine/engine.go                          |  44 +--
 engine/table_test.go                      | 169 --------
 engine/votes_test.go                      | 189 +++++++++
 internal/repository/setplay/repository.go |   2 +-
 internal/service/setplay/service.go       |  23 +-
 internal/service/setplay/table.go         | 633 ------------------------------
 internal/service/setplay/table_test.go    | 518 ------------------------
 internal/service/setplay/votes.go         |  55 +++
 internal/service/setplay/votes_test.go    |  47 +++
 scripts/ci/logic-version.json             |  35 +-
 scripts/ci/verify-plan.py                 |  10 +-
 scripts/ci/verify_plan_test.py            |  49 ++-
 scripts/wasm/go-test-exec.sh              |   5 +-
 scripts/wasm/wasm.test.mjs                |  59 +--
 web/e2e/telemetry.spec.ts                 |  71 ++++
 web/edge/beacon-layout.json               |  42 ++
 web/edge/beacon.ts                        |  46 +++
 web/edge/env.ts                           |  41 ++
 web/edge/index.test.ts                    | 282 +++++++++++++
 web/edge/index.ts                         | 159 ++++++++
 web/edge/rate.ts                          |  32 ++
 web/edge/schema.sql                       |  30 ++
 web/edge/testing/local.ts                 |  32 ++
 web/edge/testing/sqliteD1.ts              |  68 ++++
 web/edge/visitor.ts                       |  52 +++
 web/edge/votes.ts                         |  79 ++++
 web/package.json                          |   2 +-
 web/public/_headers                       |   6 +-
 web/public/config.json                    |   4 +
 web/scripts/analytics/report.mjs          | 134 +++++++
 web/scripts/analytics/report.test.mjs     |  50 +++
 web/scripts/build-edge.mjs                |  29 ++
 web/scripts/prepare-assets.sh             |   6 +-
 web/scripts/serve.mjs                     |  38 +-
 web/src/components/App.test.tsx           | 220 ++++++++++-
 web/src/components/App.tsx                | 161 +++++++-
 web/src/components/Footer.tsx             |   8 +-
 web/src/components/PrivacyPolicy.tsx      |  35 ++
 web/src/components/Results.tsx            |  92 ++++-
 web/src/lib/engineTypes.ts                |   2 +
 web/src/lib/i18n.ts                       |   5 +-
 web/src/lib/langs.ts                      |   3 +
 web/src/lib/lru.ts                        |   5 +
 web/src/lib/notation.test.ts              |   2 +-
 web/src/lib/telemetry.test.ts             |  60 +++
 web/src/lib/telemetry.ts                  | 112 ++++++
 web/src/lib/votes.test.ts                 |  40 ++
 web/src/lib/votes.ts                      |  71 ++++
 web/src/locales/en.json                   |  28 +-
 web/src/locales/ja.json                   |  28 +-
 web/src/locales/ko.json                   |  28 +-
 web/src/locales/zh-Hans.json              |  28 +-
 web/src/locales/zh-Hant.json              |  28 +-
 web/src/main.tsx                          |   6 +
 web/src/shared/api.ts                     | 137 +++++++
 web/src/styles.css                        |   5 +
 web/src/worker/engineHost.ts              |   6 +-
 web/src/worker/protocol.ts                |   2 +
 web/tsconfig.json                         |   2 +-
 web/vite.config.ts                        |   2 +-
 web/wrangler.toml                         |  23 ++
 78 files changed, 3395 insertions(+), 1830 deletions(-)
```

## S6 後の開発者要望（2026-10-16。後の段階の課題）

**新キャラクターへの対応は、そのキャラが出てから 2 週間後とするポリシーを、画面などに書く。**

- 意図（開発者）: 出たばかりのキャラの研究（セットプレイを自分で見つける楽しみ）を奪わないため。
- 入れる段階: **S7（公開前監査）の中で行う**（推測: 公開した時点でポリシーが画面に出ている必要があるため。S7 の計画で範囲を確定する）。
- 書く場所の案: 画面の「制約の説明」（5 言語）・README・`NOTICE` か利用者向けの文書。
- S7 の計画のときに開発者へ確認すること（未決）:
  1. 2 週間の起点: ゲームでの配信開始日か、公開版 tacpendium にデータが入った日か（推測: 配信開始日。利用者に分かりやすい）。
  2. 書くだけにするか、仕組みでも守るか: 公開版 tacpendium の取り込み（`/sync_upstream`・`upstream/watch.sh`）で新キャラを起点から 14 日間は moves DB に入れない（または画面に出さない）ようにするか。仕組みにするなら、キャラごとの起点の日付をどこに持つか（推測: 小さな表をリポジトリに置き、CI が日付で判定する）。
  3. 既存のキャラの調整（バランスパッチ）は対象外か（推測: 対象外。新キャラだけ）。
- S5 からの持ち越し（キャラの入力で絞り込める選択・リセットボタン）と同じく、置き場は S7 の計画で決める。

### 開発者の回答（2026-10-16）

1. 起点は**ゲームでの配信開始日**。
2. **書くだけ**（取り込みの仕組みでは止めない）。推測: 運用で守る＝新キャラを含む公開版 tacpendium の取り込み（`/sync_upstream`）は、配信開始日から 2 週間たってから行う。S7 で `.claude/commands/sync_upstream.md` にもこの運用を書く。
3. 既存キャラの調整（バランスパッチ）は**対象外**（新キャラだけ）。

本節は報告の後に追記した（docs だけの変更）。

## 目視確認での開発者の指摘（2026-10-17）

直したもの（このブランチ。5 言語同時。vitest 146 passed・E2E 21 passed、どちらも EXIT=0）:

1. フッターの文言: 「いかなるゲーム会社とも関係ありません」を外し、「個人が開発する非公式・非営利のツールです。ゲームのロゴ・画像は含みません。フレームデータは手作業で計測しています。」に（開発者: カプコンのゲームのファンツールであり、非営利を強調したい）。`NOTICE` の「提携・承認・…を受けていない」は法的な断り書きとして残した（画面の文言とは別）。
2. 「重ねる持続の位置」の説明文と、弾の注意書き（`nRangeHelp`・`projectileNote`）を消した（開発者: 冗長）。
3. プライバシーポリシーの「実行の計測」を要約にし、全項目の一覧は `docs/telemetry.md` を指すようにした（開発者: 長いと読まれない）。推測: 個人情報保護法・GDPR の説明で求められるのは主に目的と情報の種類で、項目の全列挙ではない。照合は下の S7 で行う。

後の段階に回したもの（S7 の計画で範囲を決める）:

- **提案の制約について**（`limitations.*`）を短くする（開発者: やや長い。時期は任せる）。判断: 後で行う。理由: 本家の説明（本家のコードのコメント「この扱いを変更したら文面を同時に更新」）と対応している注意書きで、短くするには項目の取捨（何を残すか）を開発者と決める必要がある。下の文言の見直しとまとめて行う。
- 「当てたい技の種別」と「特定の技から探す」の文言（開発者: 分かりにくい。別の文言を探りたい）。案を出して開発者に選んでもらう。
- プライバシーポリシーの法的照合（§7.3）: S7 の中で `/research` を回して論点と根拠（個人情報保護法の個人関連情報・GDPR の仮名化データと ePrivacy・Cloudflare の規約）をまとめ、開発者が判断する。法的助言ではない。

## 目視確認での開発者の要望（2026-10-17、2 件目）

1. **グッド/バッドのアイコンの色を分ける**: 絵文字（👍👎）は色を変えられないため、SVG のアイコン（`currentColor`）にし、グッドは緑・バッドは赤（ダーク表示用の色も用意）。押した状態の枠と数も同じ色。向きは CSS の class で回す（CSP が style 属性を許していないため）。
2. **提案を隠すボタン**: 各行に「隠す」。その回の結果からだけ消え（次の行が繰り上がる）、もう一度検索する（計算済みの結果の表示を含む）と元に戻る。隠している件数と「すべて表示に戻す」を出す。状態はメモリだけ（§7.3）。計測・票には影響しない。
   - 画面の試験を 1 本足した。§8-3: 検索で戻すのを外す／隠した後に繰り上げない、の 2 通りで赤を確かめた。vitest 147 passed・E2E 21 passed（どちらも EXIT=0）。

## プライバシーポリシーの差し替え（2026-10-17）

開発者がプライバシーポリシーのレビューを依頼し、結論は「法的には問題なし。信頼のための説明という位置づけで、読みやすく短くした方がよい」。その修正案の文面に差し替えた（5 言語。zh-Hant は生成）。

- 構成: 前書き（アカウントなし・cookie とストレージを使わない）／集める情報と目的（利用状況・グッド/バッド）／利用者の区別／保存期間／外部サービス（第三者に渡さない）／問い合わせ（Issue は公開・特定の人のデータは探して消せない）／最終更新日。
- 最終更新日は `PRIVACY_LAST_UPDATED`（`web/src/components/PrivacyPolicy.tsx`、今は 2026-10-17）を画面の言語の書式で出す。**公開時（S7〜S9）に公開日へ揃える。**
- 注記: 「乱数は毎日消すため、日をまたいで同じ人を結び付けることはできません」は、レビュー中-3 で弱めた書き方を開発者の判断で言い切りに戻したもの。実装は日付が変わった後の最初の要求で消す遅延削除で、D1 の Time Travel で戻せる可能性（推測・未確認）は残課題のまま（S7 で確かめる）。
- vitest 147 passed・E2E 21 passed（どちらも EXIT=0）。
