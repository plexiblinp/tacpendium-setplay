# 2026-11-07 security: Dependabot #18・#19 の版上げを PR #20 に取り込む

- 種類: `security`（依存の版上げ）。開始コミット `3cb3ad3`（PR #20 のブランチ）。ブランチ `claude/beautiful-babbage-jm17hf`。

## 起きたこと

Dependabot の PR 2 本（[#18](https://github.com/plexiblinp/tacpendium-setplay/pull/18) Go、[#19](https://github.com/plexiblinp/tacpendium-setplay/pull/19) web）は、どちらもマージできなかった。

- 必須チェックの `Analyze (go)`（CodeQL）が「Expected — Waiting for status to be reported」のまま。
- ほかのチェックは緑で、main にも追いついていた。
- 開発者が `@dependabot rebase` をコメントしたが反応しなかった（開発者の連絡）。
- Actions の CodeQL の実行は「PR #17」「PR #20」にはあり、「PR #18」「PR #19」には無い。
  - #18・#19 に付いた「CodeQL」は、neutral の 1 行だけ。
  - 推測: CodeQL の既定の設定が Dependabot の PR で実行されなかった。理由は未確認。

## やったこと

- Dependabot の 2 つのコミットを、そのまま PR #20 のブランチに cherry-pick した（作者は Dependabot のまま）。
  - `7f23063` → `c5b0d8b`: modernc.org/sqlite 1.60.1 → 1.61.0（libc 1.79.0 ほか間接の依存）
  - `3c96f39` → `b150ffc`: i18next 26.5.10・react-i18next 17.1.8・vite 8.3.5・wrangler 4.152.0
- 新しい PR は作らなかった。セッションで使ってよいブランチが 1 本だけのため。
- 今日のデプロイを 1 回にまとめた。
- #18・#19 は、#20 のマージの後に開発者が閉じる。Dependabot は main の版を見て、上げ済みとして扱う見込み（推測）。

## 判断と理由

- **ruleset から `Analyze (go)` を外さなかった**: CodeQL の守りが消えるため。
- **wrangler 4.152.0 では undici の固定（`overrides`）は外せない**: 4.152.0 の miniflare は undici 7.29.1 を求める。lockfile では固定のおかげで 7.30.1 になっている（`pnpm why undici`）。外すのは wrangler 4.153.0 以上に上げたとき（`docs/progress/2026-10-31-security-undici-report.md` の解除条件）。
- **modernc.org/sqlite は v1.61.1 が出ているが、Dependabot が選んだ 1.61.0 のままにした**: 公開 7 日の待ち（cooldown）を Dependabot がすでに当てた版のため。
  - 1.61.1 は、prepared statement が毎回作り直される性能の後退を直している（Dependabot の PR の changelog）。
  - Go 側の sqlite は事前計算と試験の道具で使う。推測: 配信物には入らない。
  - 次の Dependabot の PR で上がる。

## 検査（AGENTS §8。Node 22.23.4・pnpm 10.34.7。出力はファイルへ書き、`EXIT=` と件数で判定）

```
$ go mod verify                          GOMODVERIFY_EXIT=0（all modules verified）
$ pnpm install --frozen-lockfile         FROZEN_EXIT=0
$ pnpm audit                             AUDIT_EXIT=0（No known vulnerabilities found）
$ pnpm why undici                        undici@7.30.1 / undici@8.12.2
$ go vet ./...                           GOVET_EXIT=0（出力はモジュールの取得 4 行だけ）
$ go test -count=1 ./...                 GOTEST_EXIT=0（ok 13 パッケージ・FAIL 0・no test files 1）
$ npx tsc --noEmit -p .                  TSC_EXIT=0
$ pnpm test                              VITEST_EXIT=0  Test Files  22 passed (22) / Tests  247 passed (247)
$ pnpm build                             BUILD_EXIT=0（違反: 0 件）
$ bash scripts/local/build.sh            LOCALBUILD_EXIT=0
$ PW_CHROMIUM_PATH=… pnpm e2e            E2E_EXIT=0  35 passed (56.9s)
```

試験の件数は版上げの前と同じ（単体 247・E2E 35）。

## 残課題

- #18・#19 を閉じる（#20 のマージの後に開発者が行う）。
- Dependabot の PR で CodeQL が走らない原因は未特定。次の Dependabot の PR でも同じなら、Settings → Code security → CodeQL の設定を開発者が確かめる。
- undici の固定の解除（wrangler 4.153.0 以上のとき）。
