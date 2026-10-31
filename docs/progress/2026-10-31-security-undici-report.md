# 2026-10-31 security: undici の脆弱性（Dependabot #6・GHSA-xjj6-f33q-929c）

- 種類: `security`。開始コミット `2692ae5`（main。PR #16 のマージ）。ブランチ `claude/beautiful-babbage-jm17hf`
- 利用者に見える変更は無い（開発・デプロイの道具の依存だけ）。更新履歴には足さない。

## 何だったか

- Dependabot #6: **GHSA-xjj6-f33q-929c**（CVE-2026-109869、high、CVSS 7.5）。undici の `fetch()` が、6 つ以上の content-coding を返す悪意のサーバの応答で、処理されない stream の誤りによって Node のプロセスごと落ちる（DoS）。影響 `>=7.18.0 <7.30.1` ほか、修正 **7.30.1**（GitHub Advisory Database の頁。WebFetch で読んだ）。
- 経路（`pnpm why undici`）: `wrangler@4.151.1 > miniflare@5.20261012.1-beta > undici@7.29.1` だけ。jsdom の `undici@8.12.2` は 8 系の修正版で対象外。
- `pnpm audit` は同じ経路で **8 件**（high 4・moderate 4）を出した: GHSA-xjj6-f33q-929c・GHSA-4w7w-5xrc-9h26・GHSA-2m5v-pch6-74hx・GHSA-qrxm-fqhp-jwvm（high）、GHSA-4qv8-2p9j-2685・GHSA-f72v-6w45-q472・GHSA-9fm3-97px-rjf6・GHSA-55wc-3pg8-jfw6（moderate）。8 件とも修正は `>=7.30.1`。
- 影響の範囲: wrangler は devDependency（手元の `wrangler dev` と CI の `wrangler deploy`）。サイト・Worker・ローカル版の配信物には入らない。推測: 実害は、開発者か CI が悪意のサーバに `fetch` したときにその道具が落ちること（相手は Cloudflare の API で、届く見込みは小さい）。

## やったこと

`web/pnpm-workspace.yaml` に `overrides` を足した（理由と解除条件はファイルのコメントにも書いた）:

```yaml
overrides:
  "undici@>=7.0.0 <7.30.1": "7.30.1"
```

- lockfile の差分は `undici@7.29.1` → `7.30.1` の 1 パッケージだけ（integrity は registry の `dist.integrity` と一致。7.30.1 は 2026-10-15 公開・MIT）。
- 依存の操作は Node 22.23.4 で行った（AGENTS §5）。このコンテナの Node は 22.22.0 なので、nodejs.org の `node-v22.23.4-linux-x64.tar.xz` を scratchpad に取り、`SHASUMS256.txt` で照合して使った（`sha256sum -c` が `OK`）。pnpm は 10.34.7。

## 判断と理由

| 案 | 中身 | 選ばなかった／選んだ理由 |
|---|---|---|
| (a) wrangler を上げる | 4.153.0（2026-10-26 公開）から miniflare が undici 7.30.1 を使う（npm の registry で確認） | `minimumReleaseAge`（7 日）で入れられるのは 2026-11-02 から。2 日待つ間アラートが開いたままになる |
| **(b) overrides で固定（選んだ）** | undici の 7.30.1 未満だけを 7.30.1 に | 今日閉じられる。範囲を `<7.30.1` に限ったので、jsdom の 8 系にも、7.30.1 以上を持つ将来の miniflare にも効かない（固定が古い版へ引き戻すことは起きない）。7.29.1 → 7.30.1 は小版の上げ |

**版の固定の解除条件**（AGENTS §5）: wrangler を 4.153.0 以上に上げたあと、この `overrides` を消して `pnpm install` しても、`pnpm why undici` の 7 系が 7.30.1 以上で、`pnpm audit` が 0 件であること。Dependabot が wrangler を上げる PR を出したら、その PR か次の `security` の作業で消す。

## 検査（AGENTS §8）

赤（直す前。§8-3 の「1 件ある入力で 1 件と出る」の確認を兼ねる）:

```
$ pnpm audit
EXIT=1
8 vulnerabilities found
Severity: 4 moderate | 4 high
```

直した後（出力はファイルへ書き、`EXIT=` と件数で判定。Node 22.23.4・pnpm 10.34.7）:

```
$ pnpm install --frozen-lockfile        FROZEN_EXIT=0
$ pnpm audit                            AUDIT_EXIT=0  No known vulnerabilities found
$ pnpm why undici                       undici@7.30.1 / undici@8.12.2
$ node -e '<wrangler → miniflare から解決した undici の package.json>'   7.30.1
$ pnpm build                            BUILD_EXIT=0（違反: 0 件。束ねた: dist-edge/worker.mjs, local.mjs）
$ pnpm test                             VITEST_EXIT=0  Test Files  21 passed (21) / Tests  233 passed (233)
$ npx wrangler deploy --dry-run --outdir <scratchpad>   WRANGLER_EXIT=0（束縛 DB・TELEMETRY・ASSETS を読み、--dry-run: exiting now.）
$ bash scripts/local/build.sh           LOCALBUILD_EXIT=0（E2E の project local が使うバイナリ）
$ PW_CHROMIUM_PATH=/opt/pw-browsers/chromium-1243/chrome-linux64/chrome pnpm e2e
E2E_EXIT=0  32 passed (1.7m)
```

- E2E の手元の回し方の注意（このコンテナで踏んだ）: (1) Playwright が求める Chromium（headless shell 1248）がコンテナに無いので `PW_CHROMIUM_PATH` で差し替えた（`playwright.config.ts` が用意している口）。(2) `pnpm build` をデータの準備（`pnpm assets`。`scripts/local/build.sh` が中で回す）より先に回すと `dist` にデータが入らず、site の E2E が落ちる。CI と同じ順（assets → build → local の build → E2E）で回した。どちらも依存の変更とは関係が無い。
- `wrangler dev`（miniflare が undici を実際に使う経路）は回していない。CI でも回していない。

```
$ git diff --stat 2692ae5   # 報告を書いた後に pnpm audit（AUDIT_EXIT=0）と pnpm test（233 passed (233)）を回し直して一致
 docs/progress/2026-10-31-security-undici-report.md | 79 ++++++++++++++++++++++
 web/pnpm-lock.yaml                                 | 11 +--
 web/pnpm-workspace.yaml                            |  7 ++
 3 files changed, 93 insertions(+), 4 deletions(-)
```

試験は変えていない（Web 233 件・E2E 32 件）。

## 残課題

- `overrides` の解除（上の条件）。2026-11-02 以降に wrangler を上げるとき。
- Dependabot #6 は、この変更が main に入ると閉じる見込み（推測: Dependabot は main の lockfile を読み直して閉じる）。

## 配信・PR・タグ

- 配信物は変わらない。**デプロイは要らない**（マージ後の「Cloudflare へ出す」の承認待ちは Cancel でよい）。
- ローカル版のタグは要らない（undici はローカル版に入らない）。
- PR は頼まれたら作る。
