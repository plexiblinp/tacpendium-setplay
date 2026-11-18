# 2026-11-18 security: Node 22.23.5 と Dependabot #31・#32・#33 の取り込み

- 種類: `security`（実行環境と依存の版上げ）。開始コミット `dda510e`（main。PR #30 のマージ）。ブランチ `claude/beautiful-babbage-jm17hf`。
- 開発者の指示（2026-11-16）: 「11-18 に Node の更新と Dependabot 3 本の取り込みを始めてよい。報告PRは不要、次のものにまとめる」。
  - 11-16 の報告（`2026-11-16-security-wfx-loot-check-report.md`。コミット `8b378e0`）も、この PR に入れた。

## やったこと

### Node 22.23.4 → 22.23.5

- Node 22.23.5 は 2026-11-11 に公開された（TLS・HTTP/2・https.Agent の修正）。11-18 で公開 7 日の決まりを満たす。
- 手元の Node は公式の tarball を SHASUMS256 で照合して使った。
- 変えた場所:
  - `ci.yml` の `node-version` 3 か所と `local-release.yml` の 1 か所。
  - `web/package.json` の `engines.node` を `>=22.23.5` に。
  - AGENTS §5 と `docs/maintenance.md` の「依存の操作は Node ≥ 22.23.5 で」。
- 古い版の残り（AGENTS §8-8）: `22.23.4` を、`docs/progress/`（過去の記録）と lockfile を除いて検索した。残りは 1 件で、AGENTS §5 の「凍結の解除 2026-10-22」の行。これは当時の記録なので残した。

### Dependabot の 3 本（cherry-pick。作者は Dependabot のまま）

| PR | コミット | 中身 |
|---|---|---|
| [#31](https://github.com/plexiblinp/tacpendium-setplay/pull/31) | `ee9d150` | `modernc.org/sqlite` 1.61.1 → 1.62.0（`modernc.org/libc` 1.79.0 → 1.80.1）。ローカル版の票の保存で使う |
| [#32](https://github.com/plexiblinp/tacpendium-setplay/pull/32) | `805265a` | `pnpm/action-setup` → v6.1.1（`73e46992…`）。ci.yml 2 か所・local-release.yml 1 か所。SHA は公式のタグと照合した |
| [#33](https://github.com/plexiblinp/tacpendium-setplay/pull/33) | `74a99fd` | `@playwright/test` 1.64.1 → 1.65.0・`vite` 8.3.5 → 8.3.6・`wrangler` 4.156.2 → 4.158.0（miniflare 5.20261104.0-beta・workerd 1.20261104.1） |

- 3 本とも依存のファイルだけを変える PR なので、必須チェック `Analyze (go)` が出ない。そのため前回と同じく、作業ブランチに取り込んで出す（2026-11-08 の報告）。
- verify-plan: go.mod が変わったが、記録との一致を確かめた（`見張るコード 6 か所: 記録と一致`）。

## 検査（AGENTS §8。Node 22.23.5・pnpm 10.34.7。出力はファイルへ書き、`EXIT=` と件数で判定）

```
$ node --version                                    v22.23.5
$ python3 -I scripts/ci/verify-plan.py check        見張るコード 6 か所: 記録と一致 / 違反: 0 件
$ python3 -I scripts/check-deploy-workflow.py       違反: 0 件
$ bash scripts/check-actions-pinned.sh              違反: 0 件
$ go mod verify                                     GOMODVERIFY_EXIT=0
$ GOVULNCHECK_DB=… bash scripts/govulncheck.sh      GOVULN_EXIT=0  対象: 5 件 / 違反: 0 件
$ go vet ./...                                      GOVET_EXIT=0
$ go test -count=1 -v ./...                         GOTEST_EXIT=0  PASS 399 / FAIL 0 / SKIP 0（突き合わせた条件 5376・不一致 0）
$ pnpm install --frozen-lockfile                    FROZEN_EXIT=0
$ pnpm audit                                        AUDIT_EXIT=1  1 vulnerabilities found / Severity: 1 high（下）
$ npx tsc --noEmit -p .                             TSC_EXIT=0
$ pnpm test                                         VITEST_EXIT=0  Test Files  22 passed (22) / Tests  250 passed (250)
$ pnpm build                                        BUILD_EXIT=0
$ npx wrangler deploy --dry-run                     WRANGLER_EXIT=0
$ bash scripts/wasm/build.sh                        WASMBUILD_EXIT=0
$ node --test scripts/wasm/wasm.test.mjs            NODETEST_EXIT=0  tests 6 / pass 6 / fail 0
$ SKIP_ASSETS=1 bash scripts/local/build.sh         LOCALBUILD_EXIT=0
$ PW_CHROMIUM_PATH=… pnpm e2e                       E2E_EXIT=0  36 passed (1.0m)
```

- 試験の件数は前回と同じ（Go PASS 399・単体 250・E2E 36・WASM 6）。11-14 の報告の単体 249 に、同じ PR #30 で足した集計の試験 1 本を加えると 250。
- 計測（時間）は判断に使っていない。

## `pnpm audit` の 1 件: picomatch（GHSA-fc5q-39xc-vjpf）

- 内容: picomatch ≤ 4.0.7 で、繰り返した extglob のパターンが ReDoS になる（high、CVSS 7.5）。
- **この PR で入ったものではない**: main（`dda510e`）の lockfile も同じ picomatch 4.0.7。新しく公表された advisory。
- **直せる版がまだ無い**: npm の最新が 4.0.7（2026-08-24 公開）。advisory の修正版の欄も「無し」（`<0.0.0`）。override で逃げる先が無い。
- **配信物には入っていない**:
  - 経路は `vite`・`vitest` → `tinyglobby` → `fdir` → `picomatch`。どれも devDependencies で、ビルドと試験のときだけ動く。
  - `web/dist` を `picomatch` で検索して 0 件（`grep -rl picomatch dist`、`EXIT=1`）。同じ grep が `node_modules` の picomatch では 1 件当たることも確かめた。
  - `web/src`・`web/edge` からの import も 0 件。
- 受ける入力はビルドの設定にある glob（自分で書いたもの）だけ。外部から入るパターンは無い。
- **判断: いまは何もしない**。修正版が出たら Dependabot が PR を作るので、それを取り込む。
  - CI は `pnpm audit` を回していないので、CI は赤にならない。

## 配信・PR・タグ

- **配信物が変わる**（vite 8.3.6 でビルドし直す。wrangler 4.158.0 でデプロイする）。マージ後の「Cloudflare へ出す」を Approve する。画面の見た目は変わらない見込みなので、更新履歴には足さない。
- マージの後で、#31・#32・#33 を閉じる。
- **ローカル版のタグ `v1.1.1` がまだ出ていない**（Go 1.26.11 の修正を入れるには作り直しが要る。2026-11-14 の報告）。この PR の sqlite 1.62.0 も同じタグに入れられる。
  - この PR をマージした後に、main へタグを打つとよい。
- 11-16 の報告に書いたとおり、GitHub の Settings → Security log の確認は開発者が行う。

```
$ git diff --stat dda510e   # この報告を含む
 .github/workflows/ci.yml                           |  10 +-
 .github/workflows/local-release.yml                |   4 +-
 AGENTS.md                                          |   2 +-
 docs/maintenance.md                                |   2 +-
 .../2026-11-16-security-wfx-loot-check-report.md   |  48 ++++
 .../2026-11-18-security-node-deps-report.md        |  90 +++++++
 go.mod                                             |   4 +-
 go.sum                                             |  16 +-
 web/package.json                                   |   8 +-
 web/pnpm-lock.yaml                                 | 288 ++++++++++-----------
 10 files changed, 305 insertions(+), 167 deletions(-)
```
