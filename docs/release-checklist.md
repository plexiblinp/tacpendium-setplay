# 公開の手順とチェックリスト（S7 で作成。S9 で開発者が使う）

HANDOVER §13 の照合表と、公開の手順。**公開・デプロイ・GitHub と Cloudflare の設定は開発者が行う**（AGENTS §3）。Claude は手順の更新と、走らせる前の確認まで。
GitHub 設定の項目は、本家（非公開）の公開手順書を S7 で読み、このリポジトリに当てはまるものだけを書き直した（中身は写していない）。

状態: **済**＝S7 でリポジトリ内を直して確かめた ／ **開発者**＝開発者の作業 ／ **デプロイ後**＝実環境でないと確かめられない。

## 1. HANDOVER §13 の照合

| # | 項目 | 状態 | 証拠・やること |
|---|---|---|---|
| 1 | `_reference/` を削除した | 済 | S7 で削除（`git ls-files \| grep -c _reference` = 0）。REUSE の配布しない範囲・LicenseRef・検査の代表パスも外した |
| 2 | 本家の履歴を含めていない | 済（2026-10-18） | このリポジトリ自身の履歴に `_reference/`（本家の非公開資料）が残る。**公開は最終ツリーを 1 コミットにまとめた新しいリポジトリで行う**（開発者決定 2026-10-17）。手順は §2 |
| 3 | 非公開の言及が無い | 済 | 本家の名前・文書 ID は公開版 tacpendium で既に公開されているため対象外（開発者決定 2026-10-18）。公開されていない本家の SHA とメモ名は伏せ、`python3 scripts/check-private-refs.py`（CI でも回す）で 0 件。語はハッシュで持つ。§13 の語ごとの扱いは下の表 |
| 4 | LICENSE・DATA-LICENSE・NOTICE・クレジット・非公式表記 | 済 | `python3 scripts/check-licenses.py` 違反 0。画面のフッターにソース（AGPL）・データ（CC BY-SA 4.0）・非公式の表記。NOTICE §4 の公開 URL と README 冒頭の帯は公開時に直した（§4） |
| 5 | プライバシーポリシー・計測の説明 | 済（照合は開発者） | 画面のプライバシーポリシー（5 言語）と `docs/telemetry.md`。法令との照合の材料は `docs/progress/S7-privacy-research.md`。`PRIVACY_LAST_UPDATED`（`web/src/components/PrivacyPolicy.tsx`）は公開日の 2026-10-18 |
| 6 | セキュリティヘッダと CSP（外部診断） | 済（手元）／デプロイ後 | 手元の本物の配信（`_headers`＋Worker）で E2E `web/e2e/headers.spec.ts` が静的アセットと `/api` の両方を検査。外部診断は §3 |
| 7 | Cloudflare にカード未登録・Worker の回数制限 | 開発者 | カードを登録しない（無料プランは超過で止まるだけ＝HANDOVER §7.5）。回数制限は Worker のコードの中（isolate ごと。`web/edge/rate.ts`）で、Cloudflare 側の設定は要らない。D1 の `database_id` を `web/wrangler.toml` に入れる（`docs/telemetry.md`「デプロイの手順」） |
| 8 | Actions がコミット SHA 固定・権限は最小 | 済 | `bash scripts/check-actions-pinned.sh` 違反 0（workflow 3 本・uses 14 件）。全 workflow が `permissions: contents: read`、checkout は `persist-credentials: false` |
| 9 | GitHub のリポジトリ設定 | 開発者 | §5 |
| 10 | KA 0〜100 で全キャラの生成物が存在し、ゴールデンテストが緑 | 済 | 計算方式 C（サイトはエンジンを WASM で回す。S5 後の開発者決定）なので、KA ごとの生成物は無い。代わりに「31 キャラの moves DB と manifest（KA 上限 100）が生成される」（`go run ./cmd/precompute` キャラ 31・違反 0）と「本家の正解との突き合わせ 5,208 条件・不一致 0、golden 緑」（`go test -count=1 ./...`）で見る。数値は `docs/progress/S7-report.md` |

### §13 の語ごとの扱い（S7 時点の木。`.git`・`node_modules`・`.upstream` を除く）

| §13 の語 | 残っている場所 | 扱い |
|---|---|---|
| `private`（非公開リポジトリの意味） | `AGENTS.md`・`ci.yml` のコメント・HANDOVER・`docs/progress/` | 残す。「private の間は Actions の分数が有限」等、このリポジトリ自身の状態の説明で、本家の非公開資料ではない |
| `autopilot-combomgr` | `internal/service/setplay/setplay.go:4`、HANDOVER §2.1・§13、`docs/progress/S0-report.md`（S0 の記録） | 残す。公開版 tacpendium の同じファイル・docs に同じ記述がある（公開済み） |
| 友人限定リポジトリ・公式 dist | HANDOVER §13 の照合項目の文そのものだけ | 残す（本チェックリストの出所の記録） |
| 本家のコミット SHA・公開されていないメモ名 | なし | `check-private-refs.py` で 0 件 |

## 2. 公開の手順（1 コミットにまとめる）

**このリポジトリをそのまま public にしない。** 履歴に本家の非公開資料（`_reference/`）が残っているうえ、GitHub は PR の head（`refs/pull/N/head`）を消さないので、main を書き換えても古いコミットは PR から読める。

推奨の形（推測: いちばん単純で漏れが無い。開発者が決める）:

1. このリポジトリの名前を変えて非公開のまま残す（例 `tacpendium-setplay-dev`。Claude のセッションの設定も新しい名前に）。
2. 同じ名前 `tacpendium-setplay` で**新しいリポジトリを private で**作る（画面のフッターのソースへのリンクがこの名前を指している＝`web/src/lib/constants.ts` の `SOURCE_URL`）。
3. 公開する版を 1 コミットにする（例。手元で。`<ref>` は PR をマージした後の main）:

   ```
   git clone --branch main https://github.com/plexiblinp/tacpendium-setplay-dev.git pub && cd pub
   python3 scripts/check-private-refs.py        # 違反: 0 件 であること
   python3 scripts/check-licenses.py            # 違反: 0 件 であること
   git checkout --orphan release
   git commit -m "Initial public release"       # 本家の SHA をメッセージに書かない
   git remote add public https://github.com/plexiblinp/tacpendium-setplay.git
   git push public release:main
   ```
4. private のまま中身と設定（§5）を確かめ、CI が緑になってから public にする。**public にした直後に main の保護（ruleset）を確かめ直す**（可視性を変えると無効になることがある）。
5. **以後の開発は公開リポジトリで直接行う**（開発者決定 2026-10-18。スピード重視。本家のような開発用・公開用の 2 リポジトリには分けない）。このリポジトリ（名前を変えたもの）はほぼ凍結し、記録として非公開で残す。Claude のセッションは公開リポジトリを対象にする。

## 3. 外部のヘッダ診断（デプロイ後）

対象: サイトの `/` と `/api/votes?char=ryu&ka=1`（Worker の応答）。

- https://securityheaders.com/ — 期待: **A 以上**。`Permissions-Policy` は `clipboard-write=(self)` だけ許可（コピー D16 のため。減点されても外さない）。
- https://developer.mozilla.org/observatory — 期待: CSP に `unsafe-inline`・`unsafe-eval` が無いこと（`wasm-unsafe-eval` は WASM のため）、HSTS・nosniff・Referrer-Policy・X-Frame-Options（または frame-ancestors）が通ること。
- `workers.dev` は HTTPS 固定。HSTS に `preload` は付けていない（自分のドメインでないため）。
- 存在しないパス（例 `/no-such-page`）も 1 つ見る。アセットに無い要求は Worker の最後の `env.ASSETS.fetch` が返すので、`_headers` が掛かるかは手元で確かめていない（S7 レビュー低-9）。
- 結果（評価と指摘）を S9 の報告に貼る。直すものがあれば `web/public/_headers` と `web/edge/index.ts` の `API_SECURITY_HEADERS` を同時に直す（E2E `headers.spec.ts` が両方を見る）。

## 4. 公開時に書き換える箇所

| 場所 | 中身 |
|---|---|
| `NOTICE` §4 | **済**（2026-10-18）: 公開リポジトリの URL |
| `README.md` 冒頭 | **済**（2026-10-18）: 「まだ公開していない・サイト名は未定」の帯を外し、名前 **Tacpendium-Setplay**（2026-10-18 に開発者が決定）を書く |
| `web/src/components/PrivacyPolicy.tsx` | **済**: `PRIVACY_LAST_UPDATED` は公開日の 2026-10-18（S7 で入れた値のまま） |
| `.github/workflows/upstream-watch.yml` | **済**（2026-10-18）: **定期実行（日 1 回）を戻す**（`schedule` のコメントを外す。公開後は Actions が無料。開発者決定 2026-10-18）。AGENTS §9 の「手動起動のみ」の行も直す |
| `.github/workflows/ci.yml` | **済**（2026-10-18）: **`claude/**` への push での起動を戻す**（`push: branches: [main, 'claude/**']`。PR・手動は残す。job は 2 つのまま。開発者決定 2026-10-18）。AGENTS §9 の「push では走らない」の行も直す |
| `web/src/lib/constants.ts` | **済**（2026-10-18）: `FEEDBACK_FORM_URL` に Google フォームの URL を入れた（空にすると Issues だけを案内する）。フォームは本家用の Google アカウントで、日英併記の 1 つ。「内容」は回答の検証で URL を含めない（正規表現 `https?://|www\.` を含まない）。メールアドレスは収集しない |
| `web/wrangler.toml` | D1 の `database_id`（デプロイ時。秘密情報ではない） |
| `.github/workflows/deploy.yml` | S9 で追加。CI が main で緑 → 承認 → Cloudflare へ。`analytics.yml` は S9 で削除（集計は手元） |

## 5. GitHub のリポジトリ設定（公開する新しいリポジトリで。開発者）

「忘れても何も赤くならない」項目なので、1 つずつ済を付ける。

| # | 設定 | 備考 |
|---|---|---|
| G-1 | Private vulnerability reporting を有効 | public でも既定は off |
| G-2 | Dependency graph・Dependabot alerts を有効 | 同上 |
| G-3 | Dependabot security updates を有効 | version updates は `.github/dependabot.yml`（actions・npm・gomod、週 1・まとめて 1 本） |
| G-4 | Secret scanning と Push protection を有効 | public なら無料 |
| G-5 | Code scanning（CodeQL default setup。Go・JavaScript/TypeScript） | public なら無料。Actions を使うが public は無料 |
| G-6 | main の ruleset: 削除と force push を禁止、PR 必須、必須 check に `Go・WASM・リポジトリ検査` と `Web` | 必須 check を登録したら `paths` フィルタを入れない（AGENTS §9）。public にした直後に有効か確かめ直す |
| G-7 | Actions: Workflow permissions を「Read repository contents」、「Allow GitHub Actions to create and approve pull requests」を off、外部からの PR の workflow は承認を必須（初回の協力者） | 全 workflow が `contents: read` を明示しているが、既定も絞る。例外は `local-release.yml` の `release` job だけ（タグ v* の push のとき draft の Release を作るので job に `contents: write`。S8） |
| G-8 | アカウントの 2FA | HANDOVER §7.4 |
| G-9 | Environment `production` を作る: Required reviewers に開発者、Prevent self-review は**付けない**、Deployment branches は main だけ、管理者の迂回は許さない。secrets `CLOUDFLARE_API_TOKEN`・`CLOUDFLARE_ACCOUNT_ID` は**ここにだけ**置く（Repository secret・Variables に置かない） | S9 で変更（2026-10-20）。集計は手元に移したので集計用の Secrets は置かない。`docs/telemetry.md`「デプロイ用の API トークン」 |
| G-10 | Issues を有効にする（意見・不具合・翻訳の改善とプライバシーの問い合わせの窓口。`CONTRIBUTING.md`）。Pull requests の機能は Dependabot が使うので有効のまま、外からの PR は `CONTRIBUTING.md` のとおり閉じる。Discussions・Wiki・Projects は無効 | 開発者決定 2026-10-18（Google フォーム＋Issues） |
| G-11 | **About の Website** に、Cloudflare にデプロイしたサイトの URL を入れる（リポジトリの画面の右上 About の歯車 → Website） | **S9 の手番。デプロイが済んでから**（開発者指示 2026-10-18。S8 で記載） |

## 6. S9 の手番（S8 で追加）

| # | 手番 | 備考 |
|---|---|---|
| S9-1 | Cloudflare へのデプロイ後、GitHub の About の **Website** にサイトの URL を入れる（§5 G-11） | 開発者指示 2026-10-18 |
| S9-2 | **済**（2026-10-20）: main の 9880c88 に `v1.0.0` のタグ → Local release の run 39142810181 が全 job 成功（ビルド・ubuntu / macos / windows の起動確認 12 項目 ok・失敗 0・draft の作成）→ 開発者が中身を確かめて公開（https://github.com/plexiblinp/tacpendium-setplay/releases/tag/v1.0.0 、2026-10-20T15:12:39Z）。**v1.0.1**（2026-10-21）: Go 1.26.10 で作り直し（GO-2026-6809 / CVE-2026-111943。eMudhra の中間 CA 鍵の危殆化）。main の 999925d に `v1.0.1` → run 39627631145 が全 5 job 成功 → 開発者が照合して公開（2026-10-21T15:45:32Z。添付 6）。v1.0.0 は残し、本文で v1.0.1 へ案内。以下は手順の記録。ローカル版の最初の Release: main で `v<版>`（例 `v1.0.0`）のタグを push → `local-release.yml` が 3 OS の起動確認の後に **draft** の Release を作る → 中身（5 種の圧縮・`SHA256SUMS`）を確かめて公開する | 手順は `docs/local.md`「配布」。サイトの KA の説明は Releases（`RELEASES_URL`）へ案内するので、**サイトを公開する前か同時に**出す（無いと空の Releases を指す） |
| S9-3 | **済**（2026-10-21）: main の 999925d で手動起動 → run 39626692726 成功。`govulncheck.yml` の週 1 回の定期実行は default branch（main）に入ってから動く。マージ後に Actions の画面で 1 度手動起動し、緑を確かめる | S8 |
| S9-4 | ヘッダ診断（§3）・§5 の「済」付け・S9 の報告 | 開発者指示 2026-10-18（S8 ではやらない） |
