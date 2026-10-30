# 2026-10-30 ux: 集計スクリプト report.mjs が本物の Cloudflare で失敗する（SQL の型の誤り）

- 種類: `ux`（開発者の指定）。直したのは開発者が手元で回す集計の道具で、**利用者に見える変更は無い**（サイト・ローカル版の配信物は変わらない。更新履歴には足さない＝`docs/maintenance.md`「更新履歴」4）。
- 開始コミット: `6534c29`（main。PR #15 のマージ）
- ブランチ: `claude/beautiful-babbage-jm17hf`

## 起きたこと

開発者が 2026-10-30 に手元の Windows で `node scripts/analytics/report.mjs 7` を回し、次で止まった（本物の Analytics Engine で集計を流したのは初めて）。

```
Error: SQL API: HTTP 422: Input was invalid: the 2nd and 3rd arguments to IF() function must have the same type but instead had Double and Integer: if("double2" >= 80, "_sample_interval" / "double19", 0)
```

原因: `buildQueries` の `SUM(if(条件, _sample_interval / double19, 0))`。`_sample_interval`（integer）÷ `double19`（double）は Double、`0` は整数のリテラルで、Analytics Engine の SQL は `if` の 2・3 番目の型が揃うことを求める。同じ形は「KA 80 以上の割合」（`if` 2 つ）と「結果が 0 件の割合」（`if` 1 つ）の 2 つの問い合わせ・3 か所。さらに `report()` が 1 つの失敗で全体を投げるので、通ったはずの 1 つ目の表も出なかった。

## やったこと

1. **条件つきの和を `sumIf(重み, 条件)` に書き換えた**（`web/scripts/analytics/report.mjs`）。`if` は SQL から無くなった。
2. **1 つの問い合わせが失敗しても残りの表を出す**。失敗した表には `失敗: SQL API: HTTP 422: …`（応答の先頭 300 字。既存の `query` の切り方）を出し、最後に「失敗した表: n 件」を標準エラーに出して終了コード 1。
   - 本体を `run(env, argv, fetch, out, err)` に切り出して終了コードを返すようにした（試験から終了コードを確かめるため）。終了コード: 0 全部出た／1 失敗した表がある・日数の誤り／2 環境変数が無い（2 と日数の誤りの 1 は従来どおり）。
   - `report()` の戻り値は文字列から `{ markdown, failures }` に変えた。呼び出し元はこのスクリプトと試験だけ（`grep -rn "analytics/report\|report.mjs"`、`node_modules`・`.upstream`・`.git` を除くリポジトリ全体: report.mjs・report.test.mjs・docs/telemetry.md の 3 ファイル）。
3. 試験（`web/scripts/analytics/report.test.mjs`、5 件 → 8 件）:
   - 「条件つきの和は sumIf で書き、`if(` を使わない」（全問い合わせの SQL に、名前の途中でない `if(` が無いこと。2 つの問い合わせが `sumIf(重み, 条件)` の形であること）
   - 「1 つの表が失敗しても残りの表は出す」（`sumIf` を含む問い合わせだけ 422 を返す偽の fetch。表の数＝問い合わせの数、`失敗:` が 2 つ、残り 8 つは表、誤りの文は 300 字で切れる、トークンが出ない）
   - 「コマンドは失敗した表があれば表を出したうえで終了コード 1」（`run` の 0・1・2 と日数の誤り、トークンが出ない）
4. `docs/telemetry.md`:
   - 「限界」の「本物の Cloudflare では動かしていない」を実態に直した（試験は偽物、サイトは S9 から本物で動いている、集計は初回で 422 が出て直した、直した後の問い合わせは実アカウントで未確認、試験が守れるもの・守れないもの）。
   - 集計用トークンの作り方を、**Account API Tokens（範囲は Entire Account でよい）** と **My Profile → API Tokens（Account Resources: Include → 対象のアカウント）** に分けて書き直した。権限は Account Analytics: Read の 1 つだけ（読み取りのみ）。
   - §集計に PowerShell 7 の例（`Read-Host -MaskInput`、使い終わったら `Remove-Item Env:CLOUDFLARE_API_TOKEN`）と、失敗した表の出方・終了コードを足した。ps1 のファイルは置かない（開発者の手元の運用。docs の例をそのまま写せば足りる）。

## 判断と理由

- **`0` → `0.0` ではなく `sumIf` にした**。案は 2 つ: (a) `if(条件, 重み, 0.0)`（Literals の頁で `4.2` は double）、(b) `sumIf(重み, 条件)`（Aggregate functions の頁に載っている）。(b) は値の枝が 1 つしか無く、2・3 番目の型を揃える規則そのものに当たらない。読みやすさも上がる。推測: どちらも通る。どちらも実アカウントでは未検証。(b) が 422 で落ちたら (a) に替える（下の「手元で回す手順」）。
- **1 つの失敗で止めず、表ごとに失敗を出す**: 本物でしか分からない誤りが 1 回の実行で全部見える（今回は 1 つ目で止まり、ほかの 9 つの成否が分からなかった）。失敗があれば終了コードは 1 のまま。
- **関数の確認**: cloudflare-docs のサイト（developers.cloudflare.com）にはこのセッションから届かなかった（プロキシで拒否）。代わりに GitHub の `cloudflare/cloudflare-docs` を手元に取り（2026-10-30 のコミット `a5ad2e3`）、`src/content/docs/analytics/analytics-engine/sql-reference/` を読んだ。問い合わせで使う関数・構文と、載っている頁:

  | 使っているもの | 頁 | 結果 |
  |---|---|---|
  | `sumIf(expr, cond)` | aggregate-functions | 載っている（New） |
  | `SUM(x)` | aggregate-functions（`sum`） | 載っている |
  | `count(DISTINCT col)` | aggregate-functions | 載っている |
  | `quantileExactWeighted(q)(col, weight)` | aggregate-functions | 載っている。例の重みは `_sample_interval`（integer）。重みの型の決まりは書いていない |
  | `max(x)` | aggregate-functions | 載っている |
  | `toUInt32(x)` | type-conversion-functions | 載っている（負の数は未定義。重みは正） |
  | `round(x)`・`round(x, 1)` | mathematical-functions | 載っている（New） |
  | `intDiv(a, b)` | mathematical-functions | 載っている。引数の型の決まりは書いていない（`ka` は double、`10` は整数） |
  | `toStartOfDay(ts)` | date-time-functions | 載っている（New） |
  | `NOW()`・`INTERVAL '7' DAY` | date-time-functions（`now`）・literals | 載っている。`NOW() - INTERVAL '1' DAY` の例が sql-api ほかにある |
  | `if` | conditional-functions | 載っている。型の規則は docs に書いていない（今回の 422 で分かった）。もう使っていない |
  | 四則・比較（`100 * x`、`= 0`、`>= 80`） | operators | 載っている |

  docs から分からない残りの危険（推測）: `intDiv(double, 整数)`、`quantileExactWeighted` の重みに `UInt32` を渡すこと、`round(100 * … / …, 1)` の整数と Double の掛け算。どれも `if` の枝のような「型を揃える」場面ではないので、通る見込み。

## 検査（AGENTS §8）

出力はファイルへ書き、`EXIT=` と件数で判定した（ANSI を落として grep）。

赤の確認（§8-3。直す前に足した試験で、修正前のコードのまま）:

```
$ npx vitest --run scripts/analytics/      # sumIf の試験だけ足した状態
EXIT=1
     × 条件つきの和は sumIf で書き、if を使わない(本物の SQL API は if の 2・3 番目の型が揃わないと HTTP 422。2026-10-30 に実際に出た)
      Tests  1 failed | 5 passed (6)
```

修正の後に、1 つずつ壊して赤を確かめ、戻した（型・ビルドが通る壊し方）:

| 壊し方 | 結果 |
|---|---|
| 2 つの問い合わせを `SUM(if(条件, 重み, 0))` に戻す | `EXIT=1`・`Tests  3 failed \| 5 passed (8)`（sumIf の試験・部分失敗・終了コード） |
| 失敗した表で `throw e`（以前の挙動） | `EXIT=1`・`Tests  2 failed \| 6 passed (8)`（部分失敗・終了コード） |
| `failures > 0` を `failures > 99`（失敗でも 0 で終わる） | `EXIT=1`・`Tests  1 failed \| 7 passed (8)`（終了コード） |

通しの確認（§8-5。本物の入口 `node scripts/analytics/report.mjs` を回す。このセッションからは api.cloudflare.com に届かないので、全部の表が「失敗」になる形で配線を見る。トークンは偽の値）:

```
$ node scripts/analytics/report.mjs 7                         # 環境変数なし
EXIT=2
$ CLOUDFLARE_ACCOUNT_ID=dummyacc CLOUDFLARE_API_TOKEN=dummy-token-xyz node scripts/analytics/report.mjs 7
EXIT=1
### の数: 10 / 「失敗: 」の数: 10（失敗: fetch failed）/ 標準エラー: 失敗した表: 10 件(表の「失敗:」を見る)
偽のトークンの出現: 標準出力 0・標準エラー 0
```

全体（Web。スクリプトしか変えていないので Go・E2E・`pnpm build` は回していない）:

```
$ npx vitest --run
EXIT=0
 Test Files  21 passed (21)
      Tests  233 passed (233)
$ npx tsc --noEmit -p .
TSC_EXIT=0（出力 0 行）
```

```
$ git diff --stat 6534c29   # この報告を含む。報告を書いた後に vitest を回し直して EXIT=0・233 passed (233) で一致
 .../progress/2026-10-30-ux-analytics-sql-report.md | 141 +++++++++++++++++++++
 docs/telemetry.md                                  |  21 ++-
 web/scripts/analytics/report.mjs                   |  58 ++++++---
 web/scripts/analytics/report.test.mjs              |  45 ++++++-
 4 files changed, 245 insertions(+), 20 deletions(-)
```

試験は消していない（report.test.mjs は 5 件 → 8 件。Web 全体は 233 件。ほかの試験ファイルは変えていないので、main では 233 − 3 = 230 件の計算〔main で回してはいない〕）。

## 偽物で守れるもの・守れないもの

- 守れる: `if(` が SQL に戻ること（今回の型の誤りの形）、1 つの失敗で残りの表が消えること、失敗しても終了コード 0 で終わること、誤りの文へのトークンの混入。
- 守れない: Analytics Engine の SQL の型の厳しさそのもの（偽の fetch は SQL を解釈しない）。`if` 以外の関数の引数の型、未対応の関数・構文、結果の数値の正しさ。これは実アカウントで回すまで分からない。

## 手元で回す手順（開発者）

1. このブランチ（またはマージ後の main）を手元に取る。
2. PowerShell 7（`docs/telemetry.md` §集計 の例）:

   ```powershell
   cd web
   $env:CLOUDFLARE_ACCOUNT_ID = Read-Host
   $env:CLOUDFLARE_API_TOKEN = Read-Host -MaskInput
   node scripts/analytics/report.mjs 7
   $LASTEXITCODE
   Remove-Item Env:CLOUDFLARE_API_TOKEN
   ```

3. 成功の判定: **`$LASTEXITCODE` が 0** で、`###` の表が 10 個あり、どれにも `失敗:` が無いこと（データが無い表は「該当なし(0 行)」で、それは成功）。表は次の 10 個:
   日ごとの実行数と推定利用者数／KA 80 以上の実行の割合／KA の帯の実行数／KA の帯の待ち時間／キャラ別／モード別／言語・表記別／結果と誤りの code／結果が 0 件の割合／票の読み込み。
4. 1 つでも `失敗:` が出たら、**その表の題と `失敗:` の行だけ**を次のセッションに貼る（集計の数値・トークン・アカウント ID は貼らない。誤りの文は SQL と型の名前だけで、計測の中身は入らない）。とくに「KA 80 以上の実行の割合」「結果が 0 件だった実行の割合」が `sumIf` で落ちたら、`if(条件, 重み, 0.0)` に替える。

## 残課題・既知の問題

- 直した後の 10 の問い合わせが実アカウントで通るかは**未確認**（このセッションから api.cloudflare.com に届かない・トークンが無い）。開発者が上の手順で確かめる。
- cloudflare-docs のサイトには届かず、GitHub の原稿（2026-10-30 版）で確かめた。公開の頁と原稿がずれている可能性は残る（推測: 小さい）。
- 1 回の実行で 10 回問い合わせるのは従来どおり（無料枠の読み取り 1 日 1 万回に対して小さい）。

## 配信・PR・タグ

- 配信物（`web/dist`・Worker）は変わらない。**デプロイは要らない**（main の CI の「Cloudflare へ出す」が承認待ちになったら Cancel でよい＝`docs/maintenance.md`「デプロイ」）。
- ローカル版のタグは要らない（ローカル版に集計スクリプトは入らない）。
- PR は頼まれたら作る。
