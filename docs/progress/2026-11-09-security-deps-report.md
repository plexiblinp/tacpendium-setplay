# 2026-11-09 security: Dependabot #24 の取り込み

- 種類: `security`（依存の版上げ）。開始コミット `7b26763`（main。PR #25 のマージ）。ブランチ `claude/beautiful-babbage-jm17hf`。
- 開発者の指示（2026-11-09）: #24 を進める（前回〔`2026-11-08-security-deps-report.md`〕と同じ形）。

## やったこと

- [#24](https://github.com/plexiblinp/tacpendium-setplay/pull/24) のコミット `120cd60` を、そのまま cherry-pick した（`2d5dd20`。作者は Dependabot のまま）。
- 中身: i18next 26.6.0 → 26.7.1・react-i18next 17.1.10 → 17.1.11・@vitejs/plugin-react 6.1.2 → 6.2.1・wrangler 4.156.0 → 4.156.2（miniflare 5.20261102.0-beta・workerd 1.20261102.1）。
- #24 は古い main（#25 の前）の上にあったが、#25 は `web/package.json`・lockfile を変えていないので、ぶつからずに当たった。
- 前回と同じく、依存のファイルだけを変える PR なので、#24 には必須チェック `Analyze (go)` が出ない見込み（原因の推測と直し方の案は 2026-11-08 の報告）。
- undici: miniflare 5.20261102.0-beta も undici 7.30.1 を使う（固定は外したままでよい）。`pnpm audit` 0 件。

## 検査

（下に逐語で貼る）

## 配信・PR・タグ

- 配信物が変わる（i18next・react-i18next は画面で使う）。マージ後の「Cloudflare へ出す」を Approve する。挙動は変わらない見込み。
- **#25 の「Cloudflare へ出す」が承認待ちのうちは、この PR をマージしない**（承認待ちを残して次をマージすると、後の実行が止まる＝`docs/maintenance.md`「デプロイ」）。
- #24 は、この PR のマージの後に閉じる。
- ローカル版のタグは急がない。
