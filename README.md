# Tacpendium-Setplay

**非公式**のファンメイドツール。キャラクターと「ダウン後の有利フレーム（KA）」を入れると、
セットプレイのレシピ（空振りの繋ぎ → 重ねる技）を出す Web サービスです。
Tacpendium のセットプレイ自動提案機能を、単体で使えるように切り出したものです。

## しくみ

サーバでは計算しません。配信は静的ファイル（画面・技データ・計算エンジンの WebAssembly）だけで、
計算は利用者のブラウザの中（Web Worker）で行います。KA 60 以下はほぼ一瞬、KA 80〜100 の重い条件は数秒かかります。
KA 100 を超える計算には、本リポジトリから配布するローカル実行版を使う予定です。

動くコードは、実行回数の計測とグッド/バッドの受け口（Cloudflare Worker）だけです。止まっても提案は止まりません。
計測と投票で記録するものは、画面のプライバシーポリシーと [`docs/telemetry.md`](docs/telemetry.md) に書いています（cookie・ブラウザのストレージは使いません）。

## データの出所

技・フレームデータは公開版 Tacpendium（https://github.com/plexiblinp/tacpendium）から取り込みます。
開発者とコミュニティの手入力・観測による事実情報であり、公式サイト等から取得したデータ・画像は含みません。
取り込んだ版は `upstream/tacpendium.lock` で追えます。

## 新キャラクターへの対応

新しいキャラクターには、**ゲームでの配信開始日から 2 週間たってから**対応します。出たばかりのキャラのセットプレイを自分で見つける楽しみを残すためです。既存キャラクターの調整（バランスパッチ）は対象外で、データの更新が入りしだい反映します。

## ライセンス

| 対象 | ライセンス |
|---|---|
| アプリ本体（コード・利用者向けの文書） | [AGPL-3.0-or-later](LICENSE) |
| ゲームデータと事前計算の結果 | [CC-BY-SA-4.0](LICENSES/CC-BY-SA-4.0.txt)（データ © plexiblinp and the Tacpendium contributors。改変あり） |
| 開発運用の文書・道具 | [MIT](LICENSES/MIT.txt) |

パス単位の割当は [`REUSE.toml`](REUSE.toml)、データについては [`DATA-LICENSE.md`](DATA-LICENSE.md)、
非公式であることと撤退ポリシーは [`NOTICE`](NOTICE) を見てください。
ゲーム名・キャラクター名・技名は各権利者の商標またはそれに準ずるものです。

## 開発者向け

- 作業規約: [`AGENTS.md`](AGENTS.md)（Claude Code は `CLAUDE.md` 経由で読む）
- フロント（`web/`）: [`docs/frontend.md`](docs/frontend.md)
- 計画の正本: [`docs/handover/HANDOVER-001-setplay-standalone.md`](docs/handover/HANDOVER-001-setplay-standalone.md)
- 段階ごとの報告: `docs/progress/`
- 検査: `python3 scripts/check-licenses.py`（ライセンス宣言）、`bash scripts/check-actions-pinned.sh`（Action の SHA 固定）、`python3 scripts/check-private-refs.py`（公開してはいけない語）
