# 2026-11-08 security: Dependabot #21・#22 の取り込みと、undici の固定の解除

- 種類: `security`。開始コミット `81d4420`（main。PR #20 のマージ）。ブランチ `claude/beautiful-babbage-jm17hf`。
- 開発者の指示（2026-11-08）: 前回（`2026-11-07-security-deps-report.md`）と同じ形で進める。

## やったこと

1. Dependabot の 2 本のコミットを、そのまま cherry-pick した（作者は Dependabot のまま）。
   - [#21](https://github.com/plexiblinp/tacpendium-setplay/pull/21) `6327d70` → `dc137ed`: modernc.org/sqlite 1.61.0 → 1.61.1（prepared statement の作り直しによる性能の後退の修正）
   - [#22](https://github.com/plexiblinp/tacpendium-setplay/pull/22) `c269245` → `bc0ef91`: i18next 26.6.0・react-i18next 17.1.10・@types/node 26.6.6・wrangler 4.156.0
   - 2 本とも、ほかのチェック（Go・Web・ローカル版のビルド・3 OS の起動確認）は緑だった。
   - 必須チェックの `Analyze (go)`（CodeQL）は走っていなかった（前回と同じ。「CodeQL」が neutral の 1 行だけ）。
2. **undici の固定（`web/pnpm-workspace.yaml` の `overrides`）を外した**（`932e41a`）。
   - 解除条件（`2026-10-31-security-undici-report.md`）は「wrangler 4.153.0 以上で、外しても `pnpm why undici` の 7 系が 7.30.1 以上、`pnpm audit` が 0 件」。
   - wrangler 4.156.0 の miniflare（5.20261023.3-beta）は、もともと undici 7.30.1 を使う。
   - 外した後: `pnpm why undici` は `undici@7.30.1`（miniflare）と `undici@8.12.2`（jsdom）。`pnpm audit` は `AUDIT_EXIT=0`・No known vulnerabilities found。
   - lockfile の差分は `overrides:` の 3 行が消えただけ（解決した版は変わらない）。
   - 古い記述の確認: `grep -rn "overrides\|7\.30\.1"`（md・yaml・json。`node_modules`・`.upstream`・lockfile・`docs/progress/` を除く）。当たったのは無関係の `data/overrides/`（英語技名・表記の上書き表）と AGENTS §5 の一般則だけで、固定を前提にした記述は 0 件。

## CodeQL が Dependabot の PR で走らないこと（2 回目）

- 開発者が設定を確かめた（2026-11-08）。
  - Languages: 4 つすべて
  - Query suites: Default
  - Runner: Standard GitHub runner
  - Scan events: main と保護したブランチへの push と PR、毎週の定期実行
  - 設定に問題は見当たらない。
- 推測: 原因は、GitHub が「解析する言語のソースを変えていない PR」で CodeQL の解析を省くことではないか（未確認）。
  - 今回の 4 本（#18・#19・#21・#22）は、どれも `go.mod`・`go.sum`・`package.json`・lockfile だけを変えている。
  - 省いたときは「CodeQL」の check が neutral になり、`Analyze (go)` の job は作られないと考えると、表示と合う。
  - 開発者が neutral の「CodeQL」の行を開くと、理由の文が出ているはず。
- そうだとすると、ruleset の必須チェックに `Analyze (go)` を名前で入れている限り、依存だけの PR はいつも止まる。
- 直し方の候補（どれも GitHub の設定の変更なので開発者が決める。AGENTS §6）:
  - (a) ruleset の必須チェックから `Analyze (go)` を外し、代わりに ruleset の「Require code scanning results」（CodeQL）を使う。推測: 解析を省いた PR では止めない。CodeQL の守りは残る。
  - (b) いまのまま、依存の PR は毎回このセッションで取り込む（今回の形）。

## 検査

（下の「全体」に逐語で貼る）

## 配信・PR・タグ

- 配信物が変わる（i18next・react-i18next は画面で使う）。マージ後の「Cloudflare へ出す」を Approve する。挙動は変わらない見込み（試験は件数も含めて前回と同じ）。
- ローカル版のタグは急がない。
- #21・#22 は、この PR のマージの後に閉じる。
