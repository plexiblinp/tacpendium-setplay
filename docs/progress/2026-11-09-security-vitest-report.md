# 2026-11-09 security: Dependabot #27 の取り込み（vitest 5.1.0）

- 種類: `security`（依存の版上げ）。開始コミット `f633685`（main。PR #26 のマージ）。ブランチ `claude/beautiful-babbage-jm17hf`。
- 開発者の指示（2026-11-09）: #27 を進める（前回までと同じ形）。

## やったこと

- [#27](https://github.com/plexiblinp/tacpendium-setplay/pull/27) のコミット `c67295f` を、そのまま cherry-pick した（`5908735`。作者は Dependabot のまま）。
  - 依存のファイルだけを変える PR なので、必須チェック `Analyze (go)` が出ない見込み（2026-11-08 の報告）。
- 中身: vitest 5.0.5 → 5.1.0（minor）。lockfile で変わるのは、試験の道具だけ。
  - `vitest`・`@vitest/mocker`・`@vitest/spy`
  - `magic-string` 1.4.3 → 1.4.5・`tinybench` 6.2.2 → 6.2.3・`@types/estree` 1.0.9 → 1.0.10
  - vite（ビルド）は変わらない。
- **配信物は変わらない**: ビルドしたスクリプトの名前（中身のハッシュ）が、main（#26）のビルドと同じ `index-59CWM6X6.js`。**デプロイは要らない**。

## 検査（AGENTS §8。Node 22.23.4・pnpm 10.34.7。出力はファイルへ書き、`EXIT=` と件数で判定）

```
$ pnpm install --frozen-lockfile         FROZEN_EXIT=0
$ pnpm audit                             AUDIT_EXIT=0（No known vulnerabilities found）
$ npx tsc --noEmit -p .                  TSC_EXIT=0
$ pnpm test                              VITEST_EXIT=0  RUN v5.1.0 / Test Files  22 passed (22) / Tests  248 passed (248)
$ pnpm build                             BUILD_EXIT=0（違反: 0 件。index-59CWM6X6.js＝main と同じ）
$ bash scripts/local/build.sh            LOCALBUILD_EXIT=0
$ PW_CHROMIUM_PATH=… pnpm e2e            E2E_EXIT=0  35 passed (58.6s)
```

試験の件数は前回と同じ（単体 248・E2E 35）。Go は変えていない。

## 配信・PR・タグ

- デプロイは要らない（配信物が同じ）。マージ後の「Cloudflare へ出す」の承認待ちは Cancel でよい（`docs/maintenance.md`「デプロイ」）。
- #27 は、この PR のマージの後に閉じる。
- ローカル版のタグは要らない。
