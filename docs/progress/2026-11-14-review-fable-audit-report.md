# 2026-11-14 独立監査（2026-11-10）の取り込み

- 種類: `ux`（SP-01）と `security`（SP-02）。開始コミット `d9c620b`（main。PR #28 のマージ）。ブランチ `claude/beautiful-babbage-jm17hf`。
- 入力: 開発者が抜き打ちで行わせた上位モデルによる独立監査（読み取り専用。対象 `f633685`）。開発者の指示「クオリティアップのため必要なものは取り込んで」。
- 監査の結論: 重い問題は無し。発見は低 2 件（SP-01・SP-02）と、人が 5 分見る価値のある仮説 2 つ。

## 取り込みの判断

| 項目 | 監査の内容 | 判断 |
|---|---|---|
| SP-01（低・バグ） | 画面の欄は N・残すフレームに上限が無いが、beacon の形は N ≤ 999・remain ≤ 100。超えた実行は黙って計測に載らない | **直した**（下） |
| SP-02（低・品質） | `check-deploy-workflow.py` が文字列の一致だけで見るので、条件の `&&`→`||`・`secrets['…']`/`toJSON(secrets)`・1 行の `on: […]` を見逃す | **直した**（下） |
| 仮説 1 | D1 の無料枠はアカウント単位なので、同じアカウントの別サービス（開発者の comboauction）と共有される | **docs に書いた**（`docs/telemetry.md` の「無料枠を超えたとき」）。アカウントを分けるかは開発者の判断 |
| 仮説 2 | 利用者のハッシュ（ソルトの日）と票の日を別々に `now()` で取るので、UTC 0 時をまたぐ 1 回だけ前日のソルトで当日の票が入る | **直さない**。害はほぼ無い（同じ人が 0 時の前後に 1 回ずつ押した場合に、重複防止が 1 回すり抜けるだけ。設計上「日をまたげばもう 1 票」なので、範囲は同じ）。記録だけ |
| §5・§6 | Go のテスト 43 件が `.upstream/` 無しで失敗 | 既知の設計（`docs/upstream-diff.md`: 公開版のチェックアウトが無ければ `t.Fatal`、skip しない）。直さない |

## SP-01: 大きな N・残すフレームも計測に載せる

- **案**:
  - (a) beacon を作るときに受け口の上限へ丸める。
  - (b) 画面の欄に上限を付ける。
  - (c) 丸めて「丸めた」印を足す。
- **選んだのは (a)**。理由:
  - (b) は利用者に見える仕様の変更になる（AGENTS §3）。
  - (c) は beacon の形の変更（列の追加・プライバシーポリシー）になる。
  - N が 999 を超える・残すフレームが 100 を超える、はどちらも実在の技・サイトの KA を超える「事実上の上限なし」。上限ちょうどの値と区別しなくても集計の意味は変わらない（AGENTS §8-7 を確かめたうえでの判断）。
- 変更: `web/src/lib/telemetry.ts` の `buildBeacon` で `nMin`・`nMax` を `N_MAX_LIMIT`、`remain` を `KA_MAX_SITE` に丸める。`nMax` の 0（上限なし）はそのまま。
- 試験: `web/src/lib/telemetry.test.ts` に 1 本足した。
  - nMin 1000・nMax 5000・remain 150 でも送られ、上限の値になる。
  - 小さい値と nMax 0 は変わらない。
  - 直す前に赤: `EXIT=1`・`1 failed | 4 passed (5)`。直した後: `EXIT=0`・`5 passed (5)`。
- docs: `docs/telemetry.md` の beacon の中身に、丸めることを書いた。

## SP-02: デプロイの workflow の検査を形で見る

`scripts/check-deploy-workflow.py`:

- **起動の条件**: 「2 つの文字列を含むか」から、「許した形のどれかと完全に一致するか」に変えた（空白を除き、`${{ }}` を外して比べる）。
  - 許す形は、`main` かつ（`push` または `workflow_dispatch`）の並び替えだけ。
  - 成果物（`site-dist`）の段は、その前に `success() &&` が付いた形。
  - `pull_request` が入る・`&&` が `||` になる・どちらかが欠ける、はどれも 1 件の違反になる。
- **secrets**: `secrets.` の数から、「`${{ }}` の式のうち語 `secrets` を含むものの数」に変えた。`secrets['X']`・`toJSON(secrets)`・`secrets [ 'X' ]` も数える。
- **禁止の起動**（`workflow_run`・`pull_request_target`）: 字下げした `xxx:` の行から、コメントを除いた全文の語に変えた。`on: [push, pull_request_target]` の 1 行の形も見つかる。
- **コメント**: YAML のコメント（説明の文）の中の語は数えない（誤検知しない）。
- 試験（`scripts/check_deploy_workflow_test.py`）に 4 本足した。監査の 3 つの書き換え（サブテストで 7 通り）と、コメントで誤検知しないこと。
  - 直す前: `EXIT=1`・`Ran 14 tests`・`FAILED (failures=7)`（監査の書き換えが全部すり抜けた）。
  - 直した後: `EXIT=0`・`Ran 14 tests`・`OK`。いまの `ci.yml` は違反 0 件。
- 限界: これは YAML を解釈しない行の検査で、「うっかりの書き換え」を拾う補助。本当の関門は Environment `production` の承認者と main だけの制限（監査も同じ見立て）。

## 検査（AGENTS §8。Node 22.23.4。出力はファイルへ書き、`EXIT=` と件数で判定）

```
$ python3 -I scripts/check_deploy_workflow_test.py   CDW_TEST_EXIT=0  Ran 14 tests / OK
$ python3 -I scripts/check-deploy-workflow.py        CDW_EXIT=0  違反: 0 件
$ python3 -I scripts/check-private-refs.py           PRIVATE_REFS_EXIT=0  違反: 0 件
$ npx tsc --noEmit -p .                              TSC_EXIT=0
$ pnpm test                                          VITEST_EXIT=0  Test Files  22 passed (22) / Tests  249 passed (249)
$ pnpm build                                         BUILD_EXIT=0（違反: 0 件）
$ bash scripts/local/build.sh                        LOCALBUILD_EXIT=0
$ PW_CHROMIUM_PATH=… pnpm e2e                        E2E_EXIT=0  36 passed (1.2m)
```

試験は単体 248 → 249（SP-01 の 1 本）、workflow の検査の試験 10 → 14（SP-02 の 4 本）。Go は変えていない。

```
$ git diff --stat d9c620b   # この報告を含む
 .../2026-11-14-review-fable-audit-report.md        | 59 ++++++++++++++++++++++
 docs/telemetry.md                                  |  4 +-
 scripts/check-deploy-workflow.py                   | 57 ++++++++++++++++-----
 scripts/check_deploy_workflow_test.py              | 27 ++++++++++
 web/src/lib/telemetry.test.ts                      | 13 ++++-
 web/src/lib/telemetry.ts                           | 15 ++++--
 6 files changed, 155 insertions(+), 20 deletions(-)
```

## 追加: 集計の表を 2 つ（開発者の依頼 2026-11-14。同じ PR #30）

- 背景（開発者の問い）: 「キャラ別」でリュウとヴァイパーが多い。いまの表は**実行の回数**で、少人数が何回も回したのか、大勢が少しずつなのか、お試し（リュウ）なのかを見分けられない。
  - 推測: キャラの一覧は manifest の順で、リュウが 1 番目・C.ヴァイパーが 4 番目。お試しの例もリュウ。
- `web/scripts/analytics/report.mjs` に 2 つ足した（画面・計測・プライバシーポリシーは変えない。開発者の手元の集計だけ）。
  - **キャラ別の推定利用者数**:
    - `visitorDays`＝期間の `count(DISTINCT visitor)`。利用者のハッシュは日替わりなので、日ごとの利用者数の合計（延べ人日）になる。
    - `runs`。
    - `runsPerVisitor`＝`runs / visitorDays`。大きいほど少人数が何回も回している。
    - `if` は使わない（2026-10-30 の HTTP 422 の型の規則）。
  - **キャラ別×始め方**: `trigger` で分ける。`form` だけを見ればお試し（`trial`）を除ける。
- 使った関数（`count(DISTINCT …)`・割り算・`round(x, 1)`・2 列の `GROUP BY`）は、cloudflare-docs の SQL reference（2026-10-30 版を手元で確かめた）に載っている。実アカウントでは未確認で、開発者が手元で回して確かめる。
- 試験: `report.test.mjs` に 1 本足した。
  - 実装前: `EXIT=1`・`1 failed | 9 passed (10)`。
  - 実装後: `EXIT=0`・`10 passed (10)`。
  - 入口のスクリプトを偽のトークンで回すと、表の数は 13（通信できないので全部「失敗」）。トークンの出現は 0。
- docs: `docs/telemetry.md` の「出す表」に 2 つ足した。

## 追加: Go 1.26.11（GO-2026-6859。同じ PR #30）

- PR #30 の CI で、「Go・WASM・リポジトリ検査」の govulncheck の段だけが赤になった。ほかの段はすべて緑で、#30 の変更とは関係が無い。
- 原因: 新しく公表された標準ライブラリの脆弱性 **GO-2026-6859**。
  - 内容: `net/http` の HTTP/1 サーバが、MaxBytesReader とヘッダの競合で落ちうる。
  - 該当: `net/http@go1.26.10`。修正: `go1.26.11`。
  - 手元の govulncheck（`GOVULNCHECK_DB=https://storage.googleapis.com/go-vulndb`。既定の vuln.go.dev はこのコンテナの proxy で 403）で再現した。
  - 検出されたのは、全パッケージとローカル版（windows・darwin・linux。`cmd/local` の `http.Server.Serve` など）の 4 件。WASM（サイト）は検出なし。
- 対応: `go.mod` の `go` の行を 1.26.10 → **1.26.11** に上げた。CI は `go-version-file: go.mod`。
  - go1.26.11 は 2026-11-02 公開（proxy.golang.org の toolchain の時刻）。公開 7 日の決まりを満たす。
  - 前回の Go の修正（GO-2026-6809。go1.26.9 → 1.26.10）と同じやり方。
- 検査（go1.26.11）:

```
$ GOVULNCHECK_DB=… bash scripts/govulncheck.sh   GOVULN_EXIT=0  対象: 5 件 / 違反: 0 件（上げる前は 違反: 4 件）
$ gofmt -l internal engine cmd                    出力 0 行
$ go vet ./...                                    GOVET_EXIT=0
$ go test -count=1 -v ./...                       GOTEST_EXIT=0  PASS 399 / FAIL 0 / SKIP 0（本家との突き合わせ 5376 条件・不一致 0）
$ bash scripts/wasm/build.sh                      WASMBUILD_EXIT=0  setplay.wasm 3926564 bytes
$ node --test scripts/wasm/wasm.test.mjs          NODETEST_EXIT=0  tests 6 / pass 6 / fail 0
$ bash scripts/local/build.sh                     LOCALBUILD_EXIT=0
```

- verify-plan は `--all` を記録した（Go の版が変わったので、CI で本家の正解と WASM を全部回し直す）。
- **ローカル版は作り直さないと直らない**（標準ライブラリを埋め込んだ単一バイナリ。`docs/maintenance.md`「Go の修正のとき」）。マージ後に main へタグ `v1.1.1` → draft の Release → 開発者が照合して公開する。サイトは WASM に該当が無いが、デプロイで Worker 以外も作り直される。

## 配信・PR・タグ

- 配信物が変わる（計測のコード）。マージ後の「Cloudflare へ出す」を Approve する。画面の見た目は変わらない（更新履歴には足さない）。
- ローカル版は計測を送らないので影響しない。タグは要らない。
