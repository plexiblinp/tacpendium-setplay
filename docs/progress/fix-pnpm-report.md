# pnpm 本体の脆弱性への対応（2026-10-24）

S9 の途中の別件。開発者が手元でシェアの修正をビルドしようとして、`web/` の pnpm が 10.28.0 だと気づいた（ほかのプロジェクト向けも含む連絡メモから、本リポジトリの節だけを扱った）。

## やったこと

- `web/package.json`: `packageManager` を `pnpm@10.28.0` → **`pnpm@10.34.7`**。`engines.pnpm: ">=10.34.7"` を足した。
- `.github/workflows/ci.yml`（web job）: `pnpm install` の前に「pnpm の版」の step。実際に動く `pnpm --version` が `packageManager` と違えば落とし、版を job summary に出す。
- `AGENTS.md` §5: pnpm 本体の版の扱いを 1 項足した。
- lockfile は変わらない（`pnpm install --frozen-lockfile` が lockfile を書き換えずに通った）。

## 判断と理由

- **版の選定**: pnpm の公式アドバイザリ（`github.com/pnpm/pnpm/security/advisories`、4 ページ・31 件を 2026-10-24 に確認）のうち、10 系の修正の下限がいちばん高いのは GHSA-vq4v-j7r6-jq4m（依存 tarball の manifest の `name` で node_modules の外へ書ける。`--ignore-scripts` でも起きる。影響 `< 10.34.5`、修正 `>= 10.34.5 <11`）。GHSA-hwx4-2j3j-g496 の修正は 10.34.0、GHSA-c59q-g84q-2gj5・GHSA-vx52-2968-3vc6 も 10.34.5 の CHANGELOG に入っている。
  - 10 系の最新は 10.34.8（2026-10-23 公開）。CHANGELOG（タグ v10.34.8 の `pnpm/CHANGELOG.md`）で 10.34.6〜10.34.8 を読んだ。10.34.6 は `self-update` の強化・`adm-zip` の更新（Windows で Node をダウンロードするときだけ使う）、10.34.7 はミラー経由の切り替えの修正、10.34.8 はスタンドアロン実行ファイルに埋め込む Node の更新と hoist の修正。CI はスタンドアロン版ではなく setup-node の Node 22.23.4 で pnpm を動かすので、10.34.8 の Node 更新は本リポジトリには効かない。
  - **公開から 7 日たった最新の 10.34.7**（2026-10-12 公開）を選んだ。依存の `minimumReleaseAge`（7 日）と同じ考え方で、公開直後の版を避けた。11 系（11.28.8）へはメジャーの移行になるので上げなかった。
- **`engines.pnpm`**: pnpm は既定で `packageManager` の版へ自動で切り替える。ただし `manage-package-manager-versions=false` の環境では古い pnpm のまま動くので、下限を `engines.pnpm` で強制した（`engines.node` は警告だけだが、`engines.pnpm` は pnpm がエラーにする）。
- **CI の step**: `pnpm/action-setup` が `packageManager` を読む前提を、実際の版で確かめる。job は増やしていない（§9）。

## 検証（Node 22.23.4〔公式の SHASUMS256.txt と照合〕・pnpm 10.34.7、手元）

| 検査 | コマンド | 結果 |
|---|---|---|
| pnpm の版 | `cd web && pnpm --version` | `10.34.7`、EXIT=0 |
| install | `rm -rf node_modules && pnpm install --frozen-lockfile` | EXIT=0（`Done … using pnpm v10.34.7`）、lockfile は変化なし |
| assets | `pnpm assets` | EXIT=0 |
| 型 | `pnpm exec tsc --noEmit -p .` | EXIT=0 |
| 単体 | `pnpm exec vitest --run` | EXIT=0、Test Files 19 passed (19)、Tests 209 passed (209) |
| build | `pnpm build` | EXIT=0（合計 6712804 bytes、違反 0 件） |
| E2E（`node scripts/serve.mjs dist` とローカル版の配信） | `pnpm exec playwright test` | EXIT=0、29 passed |
| デプロイの守り | `python3 scripts/check-deploy-workflow.py`／`python3 -m unittest scripts/check_deploy_workflow_test.py` | EXIT=0 違反 0 件／EXIT=0 Ran 10 tests OK |

守りが赤くなることの確認（§8-3）:

- `engines.pnpm`: pnpm 10.28.0 で切り替えを止めて（`npm_config_manage_package_manager_versions=false`）`pnpm install --frozen-lockfile --offline` → EXIT=1、`ERR_PNPM_UNSUPPORTED_ENGINE … Expected version: >=10.34.7`。切り替えが有効なら、10.28.0 は `web/` の中で 10.34.7 を動かす（`web/` の外では 10.28.0 のまま）。
- CI の「pnpm の版」の step（ci.yml から抜き出して bash で実行）: 10.34.7 → EXIT=0、10.28.0 → EXIT=1（`::error::pnpm 10.28.0 が packageManager の 10.34.7 と違う`）。

`git diff --stat origin/main`（報告を除く）:

```
 .github/workflows/ci.yml | 11 +++++++++++
 AGENTS.md                |  1 +
 web/package.json         |  5 +++--
 3 files changed, 15 insertions(+), 2 deletions(-)
```

テストは減っていない。

## 依存の監査（pnpm 本体とは別）

`pnpm audit --json`（2026-10-24）: high 3・moderate 4。

- **zod `<4.6.6`**（GHSA-mrhx-c7xc-m3w5、high。制約つきの大きな集まりを検査するとメモリを使い切りうる）。直接依存で、画面と Worker（票の受け口）の両方で動く。Worker は本文を `API_BODY_MAX_BYTES`（1024 バイト）までしか読まないので、外から大きな入力は入らない見込み（推測: 1024 バイトの中で起こせるかは試していない）。Dependabot の PR（zod 4.6.6、#9）で直し、main に入った。main を取り込んだ後の監査では high 2・moderate 4（undici だけ）。
- **undici**（GHSA-4qv8-2p9j-2685・GHSA-f72v-6w45-q472・GHSA-xjj6-f33q-929c。2026-10-23 公開）: 経路は `jsdom>undici`（テスト）と `wrangler>miniflare>undici`（手元の開発用）だけ。公開の Worker・ローカル版には入らない。修正版（8.12.2・7.30.1）はまだ 7 日たっていないので、Dependabot の PR か次の段階で上げる。

## 残課題・次への入力

- 手元（開発者の PC）: Node を 22.23.4 以上にしてから、`web/` で `pnpm --version` が 10.34.7 になるのを確かめ、`pnpm install --frozen-lockfile` する。この変更がマージされるまでは、10.28.0 で install しない。すでに 10.28.0 で install した `node_modules` は、消して入れ直す。
- クラウドのセッションの hook（`session-start.sh`）は環境の Node 22.22.0 で `pnpm install` する。pnpm は 10.34.7 に切り替わるが、Node は古い（`engines.node` は警告だけ）。依存を足す・上げるときは AGENTS §5 のとおり Node ≥ 22.23.4 を別に用意する（今回は公式の tarball を SHASUMS256.txt と照合して使った）。
- シェアの PR（#10）・zod の PR（#9）のマージ後に main を取り込み、検査を回し直した（下の「main 取り込み後」）。本変更は配信物を変えない（ビルドの道具だけ）。

## main 取り込み後（3bdbc0d を merge）

| 検査 | 結果 |
|---|---|
| `pnpm --version` | 10.34.7 |
| `rm -rf node_modules && pnpm install --frozen-lockfile` | EXIT=0 |
| `pnpm assets` / `tsc --noEmit` | EXIT=0 / EXIT=0 |
| `vitest --run` | EXIT=0、Test Files 19 passed (19)、Tests 218 passed (218)（シェアの PR で増えた） |
| `pnpm build` | EXIT=0 |
| `playwright test` | EXIT=0、29 passed |
| `check-deploy-workflow.py` | EXIT=0、違反 0 件 |
| `pnpm audit --json` | EXIT=1、high 2・moderate 4（すべて undici。zod は解消） |
