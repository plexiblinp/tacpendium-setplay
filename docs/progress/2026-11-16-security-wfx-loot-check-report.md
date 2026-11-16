# 2026-11-16 security: CI 秘密情報窃取キャンペーン「wfx-loot」の痕跡の確認ほか

- 種類: `security`。開始コミット `dda510e`（main。PR #30 のマージ）。ブランチ `claude/beautiful-babbage-jm17hf`。
- 開発者の依頼（2026-11-15）の 3 件: (1) wfx-loot の痕跡の確認【優先】、(2) Node 22.23.5、(3) Go 1.26.11。
- 記事（aikido.dev）は開いていない。痕跡の項目は、開発者の依頼の文に書かれたものをそのまま使った。

## 1. wfx-loot の痕跡: **見つからなかった**

確かめた日時: 2026-11-16。GitHub の API（このセッションの GitHub の接続）と、全ブランチを取得した手元の git で見た。

| 痕跡 | 見た範囲と方法 | 結果 |
|---|---|---|
| PR「docs: dependency audit notes」 | 全 PR #1〜#33（open・closed）の題と作者 | 無い。番号は #1〜#33 で抜けが無い。作者は開発者と dependabot[bot] だけ |
| ブランチ `wfx-docs`・`chore/<英数8桁>`・`ci-check-<数字4桁>` | GitHub の全ブランチ 12 本の名前 | 無い（`main`・`claude/…` 6 本・`dependabot/…` 5 本） |
| コミット「ci: refresh build cache」「chore: add ci helper」「chore: add dependency audit workflow」「ci: restore」「ci: cleanup」 | 全ブランチの全履歴の件名（`git log --all`） | 無い |
| `.github/workflows/build-audit-*.yml`・`.ci-*.sh` | 全ブランチの先端のファイルと、全履歴で一度でも存在したファイル | 無い |
| `web/package.json` への `postinstall` | 全ブランチの `web/package.json` の `postinstall`・`preinstall`・`install`・`prepare` | 無い（当たったのは既存の `"assets": "bash scripts/prepare-assets.sh"` だけ） |
| Actions の実行番号の欠番 | 全実行 434 件（API で全ページ）を workflow ごとに並べた | 欠番は 0（CI #1〜#177、Local release #1〜#119、Upstream watch #1〜#29、govulncheck #1〜#6、CodeQL #1〜#71、Dependabot #1〜#27・#1〜#5） |

ほかに見たこと:

- 登録されている workflow は 7 つ（`ci.yml`・`govulncheck.yml`・`local-release.yml`・`upstream-watch.yml` と、GitHub の動的な 3 つ）だけ。知らない workflow は無い。push して消された workflow も、登録は残るはずなので、これも痕跡が無い根拠の 1 つ。
- 全ブランチの workflow のファイルは、既知の 5 つ（`analytics.yml` は S9 で消す前の古いブランチ 3 本にだけ残る）。
- コミットの作者は 3 つだけ（Claude〈このセッションたち〉・開発者・dependabot[bot]）。
- 実行を起動したのは、開発者・dependabot[bot]・github-advanced-security[bot] だけ。
- 直近 14 日に workflow を変えたコミットは、Dependabot の #32（`pnpm/action-setup` を v6.1.0 → v6.1.1）だけ。差分は SHA の付け替え 3 か所だけ。新しい SHA `73e46992e05b19abd31f2f3c20e0e7370d762115` は、pnpm/action-setup の公式のタグ `v6.1.1` が指すコミットと一致した（`git ls-remote --tags https://github.com/pnpm/action-setup`）。

⇒ 秘密情報（Cloudflare の API トークン・Environment `production` の secrets）の再発行は、この確認の範囲では要らない。

限界:

- 見たのは GitHub 上に残っているもの（ブランチ・PR・履歴・実行の記録）。消されたブランチ・force push で消された履歴の中身は見えない。ただし、実行番号の欠番と知らない workflow の登録が無いので、workflow が動いた形跡は無い。
- 開発者の GitHub アカウント自体の安全（2FA・セッション・個人用アクセストークン・SSH 鍵）は見ていない。開発者が Settings → Security log で、見覚えの無いログインやトークンの作成が無いかを確かめると、なお確か。

## 2. Node 22.23.5（2026-11-11 公開）

- nodejs.org の `dist/index.json` で、`v22.23.5`（2026-11-11・security: true）を確かめた。
- **公開から 7 日（2026-11-18）たっていない**（`docs/maintenance.md`「Dependabot が上げないもの」の決まり）。この報告の時点では上げていない。
- 影響はビルドと `wrangler deploy` の経路（CI のランナーがレジストリ・Cloudflare と TLS で通信する所）。本番の Worker は Node を使わない。

## 3. Go 1.26.11

- **対応済み**: PR #30（2026-11-14 マージ）で、`go.mod` の `go` の行を 1.26.10 → 1.26.11 にした。CI の Go の govulncheck が GO-2026-6859（`net/http`。依頼の CVE-2026-97054 に当たると推測）を検出したため。報告は `docs/progress/2026-11-14-review-fable-audit-report.md`。
- 公開日: proxy.golang.org の toolchain の記録では 2026-11-02T20:36:21Z。依頼の文では 2026-11-10。golang-announce は、このセッションから開けないので確かめていない（go.dev も 403）。
  - 11-10 の公開なら、11-14 の時点では 7 日の決まりを満たしていなかったことになる。脆弱性の修正なので、結果として上げたことは妥当と考える（決まりの「急ぐ修正」）。
  - ただし、上げたときの報告に書いた「7 日を満たす」の根拠は proxy の時刻だけだった。
- cgo の 11 件（悪意あるモジュールをビルドしたときのビルド時のコードの実行）も 1.26.11 で直る。本リポジトリは cgo を使わない（`modernc.org/sqlite` は純 Go）。
- **残り: ローカル版の作り直し**。タグ `v1.1.1` はまだ無い（2026-11-16 の `git ls-remote --tags`）。開発者が main（`dda510e`）にタグを付けて push すると、draft の Release ができる。
