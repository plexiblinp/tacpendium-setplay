# S9 公開 — 報告（限定公開の前まで）

- 期間: 2026-10-20〜2026-10-24
- 開始: main `9880c88`（S8 まで）。作業ブランチ `claude/s9-web-deployment-t0w876`（PR #6 をマージした後、規約どおり main から作り直して続けた）
- 状態（2026-10-24）: **サイトはデプロイ済み・URL はまだ誰にも渡していない**。限定公開（URL を知り合いだけに渡す）→ シェアのマージ → 一般公開（告知）は、これからの開発者の手番。HANDOVER §9 の S9 の完了（一般公開）までは、この報告に追記していく。

## 公開先

| 項目 | 値 |
|---|---|
| URL | **https://tacpendium-setplay.plexiblinp.workers.dev** |
| Worker | `tacpendium-setplay`（`workers.dev` のサブドメイン `plexiblinp`。アカウントで 1 つ。将来の別サービスも同じサブドメインに載る＝開発者決定 2026-10-21） |
| 最初のデプロイ | 2026-10-22。CI run 39801993741（3 回目の試行）。版 `49dd6fb1-7995-4dc6-b981-7fa6e0290607` |
| いまの版 | 2026-10-24。CI run 40068301125（手動起動。権限を絞ったトークンの確認）。版 `9b97804c-013e-4570-87b7-8774db9fb36f`。中身は main `ada2522` |
| 束縛 | `env.DB`（D1 `tacpendium-setplay`、`database_id = a0ec73f7-c24f-490a-a8be-c0fcedf81f77`）・`env.TELEMETRY`（Analytics Engine `setplay_runs`）・`env.ASSETS` |

## 開発者がやること（残り）

1. **この報告の PR をマージしてデプロイ**（`/api` の Permissions-Policy の修正を含む）→ securityheaders.com で `/api/votes?char=ryu&ka=1` を診断し直す（期待 A+）。
2. 画面の確認の残り: 結果の行の 👍 を押した後、もう一度「提案を出す」でその行が上に来るか。
3. **GitHub の About の Website**（G-11・S9-1）: 限定公開のうちに入れるか、一般公開のときに入れるかを決める（入れるとリポジトリを見た人に URL が見える）。
4. **限定公開**: URL を知り合いだけに渡す。
5. **シェアの PR をマージ**してから一般公開する（開発者の希望の順）。シェアの作業に OGP の URL を渡す（下の「次への入力」）。
6. **一般公開**（告知）。
7. デプロイ用トークンの期限 **2027-01-22** の 1 週間前をカレンダーに入れる（SF6 の大きな更新の対応のたびに作り直す）。
8. （リリース後）Google・Apple アカウントの強化（案内は Claude が出す約束）。

## やったこと

### 1. デプロイの方式と CI（リポジトリ）

- **方式の決定**（開発者決定 2026-10-20）: (a) GitHub Actions ＋ スコープを絞った API トークン ＋ 承認者付きの Environment。(b) Cloudflare の Git 連携（Workers Builds）は採らなかった（理由は「判断と理由」）。
- **デプロイの job**: 最初は別の workflow（`deploy.yml`。CI の完了で起動する `workflow_run`）で作ったが、CodeQL が **actions/cache-poisoning（high ×3）** と指摘したので、`ci.yml` の `deploy` job（「Cloudflare へ出す」）に作り直した（`920a91d`）。main への push と手動起動（main）のときだけ、Go・Web の job の成功を待って、Web の job で作り E2E を通した `dist` を出す。Environment `production` を通り、トークンは `wrangler deploy` の 1 段だけに渡す。
- **守りの検査**: `scripts/check-deploy-workflow.py`（と試験）を CI のリポジトリ検査に足した。main・push の条件・`needs`・Environment・secrets の置き場所・成果物の条件が消えたら赤。`workflow_run`・`pull_request_target` も禁止。
- `wrangler` 4.150.0 を devDependency に（lockfile 固定）。sharp の native（LGPL-3.0 の libvips）は `ignoredOptionalDependencies` で入れない（AGENTS §5）。
- **集計の workflow（`analytics.yml`）を削除**: public の Actions のログ・summary は誰でも読めるため（開発者決定 2026-10-20）。集計は手元で `web/scripts/analytics/report.mjs`。
- `docs/incident.md`（事故のときの手順）を新しく作った。
- `web/wrangler.toml`: D1 の `database_id`、`preview_urls = false`（F-06）。

### 2. 緊急対応（2026-10-21〜22。eMudhra の中間 CA 鍵の危殆化）

- **Go**: `go 1.26.10`（toolchain の行は消えた）。go1.26.10 の `crypto/x509` に 8 件の SPKI の拒否リストがあることを差分で確かめ、govulncheck が go1.26.9 で GO-2026-6809 を検出・go1.26.10 で 0 件になることを確かめた。修正だけのブランチ `claude/go-1.26.10-hotfix` で PR #5（開発者の許可 2026-10-21）→ マージ。CVE-2026-111943 との対応は外部 AI の調査（`docs/research/2026-10-22-go-6809-cve-alias.md`）。
- **ローカル版 v1.0.1**: main `999925d` に `v1.0.1` → Local release の run 39627631145 が全 5 job 成功 → 開発者が照合して公開（2026-10-21T15:45:32Z。添付 6）。v1.0.0 は残し、本文で v1.0.1 へ案内。
- **Node**: CI・local-release の setup-node を `22` → `22.23.4`、`engines` を `>=22.23.4`（ルート証明書が NSS 3.131.1 に。emSign のルートが 4 → 1 件）。
- **依存の凍結**: AGENTS §5 に一時凍結を書き（2026-10-21）、Node の修正版で Node・Go を解除（2026-10-22。Python は pip ≥ 26.2.2 の環境でだけ）。凍結の前日に足した wrangler（と間接の 59 件）は PR #5 から外し、凍結の解除の後に PR #6 で main に入れた。

### 3. セキュリティ監査の指摘（F-01〜F-08）

報告は `docs/progress/fix-security-report.md`。F-01〜F-04・F-06・F-07 を直し、F-08 は確かめ、F-05（npm の脆弱性検査）は案だけ。PR #6 に入れて最初のデプロイから効いている。

### 4. GitHub の設定（開発者。2026-10-21 に点検。`docs/release-checklist.md` §5）

| # | 結果 |
|---|---|
| G-1〜G-3・G-5 | 有効（CodeQL の対象は GitHub Actions・Go・JavaScript/TypeScript・Python） |
| G-4 | Secret scanning は有効。**Push protection は無効だったので有効にした** |
| G-6 | ruleset は既存（Active・bypass なし・必須 check は Go・Web と CodeQL の `Analyze (go)`）。**Require branches to be up to date は無効だったので有効にした** |
| G-7 | 想定どおり。「Require actions to be pinned to a full-length commit SHA」は最初から有効 |
| G-8 | 2FA（GitHub Mobile）。パスキーを追加した（2026-10-21） |
| G-9 | Environment `production`: Required reviewers＝開発者・Prevent self-review なし・管理者の迂回なし・main だけ。secrets は `CLOUDFLARE_API_TOKEN`・`CLOUDFLARE_ACCOUNT_ID` だけ |
| G-10 | Issues 有効、Wikis・Discussions・Projects・Sponsorships 無効、Allow merge commits 有効。Preserve this repository は既定の有効のまま |
| G-11 | **未**（上の「開発者がやること」3） |

### 5. Cloudflare（開発者）

| 項目 | 結果 |
|---|---|
| アカウント | メール＋パスワードで作成（GitHub・Google で作ると入口が 1 つにまとまるため避けた）。2FA（認証アプリ）と回復コードの保管。**支払い方法なし**（Billing で確認） |
| サブドメイン | 自動で付いた名前がメールアドレスの一部だったので `plexiblinp` に変えた |
| D1 | 画面で `tacpendium-setplay` を作り、Console で `schema.sql` の表（`ballots`・`salts`・`votes`、後から `quotas`）を作った |
| Analytics Engine | 初回のデプロイが「You need to enable Analytics Engine」（code 10089）で止まったので、画面で **Create Blank Dataset**（`setplay_runs`・Binding `TELEMETRY`）を作って有効にした |
| デプロイ用トークン | `github-actions-deploy-tacpendium-setplay`（アカウント所有・期限 2026-10-22〜2027-01-22・IP 制限なし）。権限の変遷は下 |
| Worker の設定（2026-10-24） | Domains の Worker URL: Production（`tacpendium-setplay.plexiblinp.workers.dev`）有効・**Preview（`*-tacpendium-setplay…`）無効**。Build: Git は**未接続**。Observability: Logs・Traces は空で Sampling は操作できない（Workers Logs は無効の表示と判断。推測） |

**トークンの権限の変遷**（Legacy の「Workers Scripts」は使わない＝開発者決定 2026-10-22）:

1. `Workers: Editor`（Entire Account）→ 初回のデプロイが `assets-upload-session` で「No access to the specified resource」。
2. 外部 AI の調査（`docs/research/2026-10-22-cloudflare-token-permissions.md`）と cloudflare-docs のソースで、「Editor は既存の Worker の更新だけで作成できない。作成には Workers 全体の Admin」を確かめた。
3. `Workers: Admin`（Entire Account）→ アップロードは通り、上の Analytics Engine の誤りに進んだ。有効にした後、**デプロイ成功**（2026-10-22）。
4. Worker ができたので **`Individual Workers: Editor`（Specified Workers → tacpendium-setplay だけ）** に絞った（2026-10-24）→ CI の手動起動で**デプロイ成功**（run 40068301125）。ただしアセットの変更が無かった（「No updated asset files to upload」）ので、**ファイルの上げ直しを Editor で通したことはまだ無い**（残課題）。

### 6. 外部のヘッダ診断（2026-10-24。版 `9b97804c`）

| 対象 | securityheaders.com | MDN Observatory |
|---|---|---|
| `/` | **A+**（指摘なし） | **A+**（140/100） |
| `/no-such-page` | **A+**（404 にもヘッダが全部付く＝F-07 が本番で効いている。「Response is not HTML」「Status code indicates error」は注意だけ） | 404 で採点されない |
| `/api/votes?char=ryu&ka=1` | **A**（**Permissions-Policy が無い**。「Response is not HTML」は注意だけ） | 404 で採点されない（推測: Observatory はホスト名のトップを調べる道具） |

→ `/api` の応答（`API_SECURITY_HEADERS`）に Permissions-Policy を足した（`5ac63b1`。JSON の応答は画面の機能を使わないので clipboard-write も含めて全部拒否）。単体試験と E2E は修正前に赤を確かめた。**本番にはこの報告の PR のデプロイで出る**。

### 7. 画面の確認（開発者。2026-10-24）

PC・スマホで表示と提案は問題なし（「ざっと見て」）。👍 の反映は未確認（上の「開発者がやること」2）。`/no-such-page` が 404 になることは診断で確かめた。

## 判断と理由

- **Actions ＋ トークン（(a)）にした理由**: テストに通った CI と同じ環境・同じコミットで作った `dist` をそのまま出せる（§8-12）。Workers Builds は Go・Node・pnpm の既定の版が CI とずれ、CI の結果と連動しない。トークンを GitHub に置く弱さは、Environment の承認者・main だけ・期限・権限の絞り込みで抑えた。
- **別の workflow にしなかった理由**: 上の CodeQL の指摘。`workflow_run` はフォークの PR の完了でも起動し main の扱いで動く。CI の中の job にすれば、その心配が無く、E2E を通した `dist` を出せる。
- **集計を手元に移した理由**: public のリポジトリでは Actions の summary を誰でも読める。プライバシーポリシーの「第三者に渡さない」と利用者の期待からずれる。集計用のトークンも GitHub に置かずに済む。
- **トークンを Legacy にしなかった理由**: 開発者の希望。調べると、新しい方針（Workers の Admin／Individual Workers の Editor）で同じことができ、範囲を 1 つの Worker に絞れる分、Legacy（アカウント全体）より狭い。
- **D1 の権限を付けなかった理由**: docs（束縛先の権限は要らない）と実際のデプロイの両方で要らなかった。
- 問い合わせの窓口を Google フォームのままにした理由（開発者の質問 2026-10-22 への回答）: アプリに持たせると、自由記述の保存（個人情報を持つサイトになる）・スパム対策・読む画面と認証・通知・削除の手順を自前で抱える。

## 残課題・既知の問題

- **絞ったトークンでのアセットの上げ直し**: 次に画面が変わるデプロイ（シェアのマージ等）で、`Individual Workers: Editor` でファイルの上げ直しが通るかを確かめる。通らなければ、そのときに権限を見直す（候補: 一時的に Workers: Admin）。
- **無料プランの CPU 時間（1 要求 10 ms）**: 票の受け口の目録（manifest・moves DB）の初回の読み込みは未計測（`fix-security-report.md`）。
- **F-05**（npm の脆弱性検査）は案のまま（`docs/release-checklist.md` §7）。凍結は解けたので、頼まれれば足す。
- **公開後に弱点が見つかったとき**は、public のブランチで直すと修正が先に見えるので、GitHub の Security Advisory の非公開の作業場所で直す（開発者への回答 2026-10-22。`docs/incident.md` への追記は未）。
- S8 の残課題のうち、macOS の実ブラウザでのローカル版の確認は未（Windows は開発者が S8 の過程で確認済み。KA 100 超えも計算できた）。
- ローカル版のウイルス対策ソフトの誤検知（ノートン → アバストで「ひとまず問題なし」）。`docs/local.md` に照合と報告の手順を足した。
- Analytics Engine の超過時の挙動は未確認（S1 ⑥ のまま）。

## 次への入力

- **シェアの作業へ（OGP）**: `og:url` は `https://tacpendium-setplay.plexiblinp.workers.dev/`。`og:image` は同じ host の絶対 URL（例 `https://tacpendium-setplay.plexiblinp.workers.dev/og.png`）。★`public/_headers` の `/*` の規則で、画像にも `Cross-Origin-Resource-Policy: same-origin` が付く。X などが OGP の画像を読めるか（CORP は通常ブラウザの埋め込みにだけ効き、サーバ側の取得には効かない見込み＝推測）を、シェアの作業側で確かめること。
- 一般公開の前に、シェアの PR のマージの有無を開発者に確かめる（開発者の希望の順）。
- デプロイは CI の「Cloudflare へ出す」の承認で行う。手順は `docs/telemetry.md`「デプロイの手順」。

## 検証（手元。Node v22.23.4。この報告のコミットの直前の木で回した。出力はファイルへ書き、終了コードと件数で判定）

```
$ pnpm exec tsc --noEmit                         → EXIT=0
$ pnpm test                                      → EXIT=0  Test Files 18 passed (18) / Tests 197 passed (197)
$ pnpm build                                     → EXIT=0  ファイル: 41(上限 20000) / 違反: 0 件
$ SKIP_ASSETS=1 bash scripts/local/build.sh      → EXIT=0
$ PW_CHROMIUM_PATH=/opt/pw-browsers/chromium-1194/chrome-linux/chrome pnpm e2e   → EXIT=0  28 passed (1.7m)  (retries: 0)
$ python3 scripts/check-licenses.py              → EXIT=0 違反: 0 件
$ python3 -I scripts/check-private-refs.py       → EXIT=0 違反: 0 件
$ python3 -I scripts/check-deploy-workflow.py    → EXIT=0 違反: 0 件
$ bash scripts/check-actions-pinned.sh           → EXIT=0 違反: 0 件
```

Go は、S9 の Go の変更（go.mod の版の行）の後に手元で `go test -count=1 ./...` EXIT=0・PASS 359・FAIL 0・本家との突き合わせ 5208 条件・不一致 0（`fix-security-report.md`）。PR #5・#6 と main の CI の Go の job も緑。その後 Go のファイルは変えていない（`git diff --name-only ada2522 -- '*.go' go.mod go.sum` は 0 件）。

E2E は、このコンテナに入っている Chromium の版に合わせて `PW_CHROMIUM_PATH` を付けた（S8 と同じ）。

### `git diff --stat 9880c88`（lockfile を除く。この報告のファイルを除く）

```
 .github/workflows/analytics.yml       |  49 -------
 .github/workflows/ci.yml              |  87 +++++++++++-
 .github/workflows/local-release.yml   |   2 +-
 AGENTS.md                             |   6 +-
 docs/incident.md                      |  46 ++++++
 docs/local.md                         |   3 +
 docs/progress/fix-security-report.md  | 203 ++++++++++++++++++++++++++
 docs/release-checklist.md             |  56 +++++---
 docs/telemetry.md                     |  80 +++++++----
 go.mod                                |   4 +-
 scripts/check-deploy-workflow.py      | 135 ++++++++++++++++++
 scripts/check_deploy_workflow_test.py |  67 +++++++++
 scripts/ci/logic-version.json         |  22 ++-
 web/e2e/headers.spec.ts               |   1 +
 web/edge/catalog.ts                   |  86 +++++++++++
 web/edge/index.test.ts                | 259 ++++++++++++++++++++++++++++++++--
 web/edge/index.ts                     | 137 ++++++++++++++----
 web/edge/schema.sql                   |   9 ++
 web/edge/testing/local.ts             |  11 +-
 web/edge/visitor.ts                   |  56 +++++++-
 web/edge/votes.ts                     |  23 ++-
 web/package.json                      |   5 +-
 web/pnpm-workspace.yaml               |   6 +
 web/scripts/analytics/report.mjs      |   2 +-
 web/scripts/serve.mjs                 |  11 +-
 web/src/components/App.test.tsx       |   2 +-
 web/src/components/PrivacyPolicy.tsx  |   2 +-
 web/src/locales/en.json               |   2 +-
 web/src/locales/ja.json               |   2 +-
 web/src/locales/ko.json               |   2 +-
 web/src/locales/zh-Hans.json          |   2 +-
 web/src/locales/zh-Hant.json          |   2 +-
 web/wrangler.toml                     |   9 +-
 33 files changed, 1220 insertions(+), 169 deletions(-)
```

試験のファイルは追加・増加のみ（`web/edge/index.test.ts`・`web/e2e/headers.spec.ts`・`scripts/check_deploy_workflow_test.py`〔新規〕。`App.test.tsx` はプライバシーポリシーの日付の 1 行の置き換え）。
