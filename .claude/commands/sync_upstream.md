---
description: 公開版 tacpendium の変更（セットプレイのロジック・データ・文言）を単体版へ取り込む
argument-hint: [取り込む公開版のコミット SHA（省略時は main の先頭）]
---

公開版 tacpendium（`plexiblinp/tacpendium`）の変更を単体版へ取り込む。取り込む版: `$ARGUMENTS`（空なら公開版 main の先頭）。
規約は `AGENTS.md`（とくに §4・§8）。**本家 combomgr は読まない**（取込元は公開版だけ。AGENTS §7）。

## 1. 差分を出す
- 基準は `upstream/tacpendium.lock`。見張るパスは `upstream/watch.sh` の `WATCH` と同じ。
- まず `bash upstream/watch.sh` で概要を見る（exit 0 なら変化なし。そう報告して終わる）。
- 公開版を一時ディレクトリへ `git clone --filter=blob:none --no-checkout` し、`git diff <基準> <新 SHA> -- <見張るパス>` で中身を読む。テストファイルの差分も読む。

- **新キャラクターを含む版は、そのキャラのゲームでの配信開始日から 2 週間たつまで取り込まない**（S6 後の開発者決定。README「新キャラクターへの対応」）。既存キャラの調整（バランスパッチ）は対象外で、すぐ取り込んでよい。新キャラと既存キャラの調整が同じ版に入っていて分けられないときは、開発者に聞く。新キャラかどうかは `character_data/` に増えたファイルで見る。

## 2. 分類する
変更ファイルを次に分け、報告の冒頭に表で書く:

| 分類 | 例 | 単体版での扱い |
|---|---|---|
| エンジン・絞り込み | `internal/service/setplay/*.go`（`setplay.go`・`refine.go`・`rapid.go`・`approach.go` など） | エンジン（`engine/`）へ移植。WASM 経路も確認 |
| 本家テスト | `*_test.go`（とくに `golden_test.go`） | 適合性テストへ反映。期待値が変わったなら理由を確かめる |
| データ | `character_data/*.csv`・`migrations/*_data_*.sql` | 取込みパイプラインの再生成で済むかを先に確かめる。済むならコード変更しない |
| 文言 | `web/src/locales/*.json` の `setplay.*` | 単体版の全ロケールへ反映（AGENTS §8-9） |
| 定数・モデル | `internal/model/*.go` | 単体版の対応する定数を同期 |
| 単体版に関係ない | DB・API・本家の画面固有 | 取り込まない。理由を一言書く |

## 3. 取り込む
- 単体版の対応箇所を特定して移植する。本家にしか無い依存（DB・ctx・ユーザーデータ）は、単体版の切り出し方に合わせて落とす。
- 本家で直ったバグは、単体版にも同じテストを足して、直す前に赤くなることを確かめる（AGENTS §8-3）。
- テスト・事前計算・検査を AGENTS §8 の形で回す（`EXIT=` と件数）。
- **計算に効くコード（`scripts/ci/logic-version.json` の `watched`）を変えたら、影響を判断して記録する**（記録しないと CI のリポジトリ検査が止まる）。判断は報告にも書く。
  - 影響なし（単体版の挙動が変わらない・単体版で使わない部分）: `python3 scripts/ci/verify-plan.py ack --none --reason "…"`
  - 一部のキャラ・検査だけ回す: `python3 scripts/ci/verify-plan.py ack --kinds oracle,wasm --chars ryu,ken --reason "…"`
  - 全部回す: `python3 scripts/ci/verify-plan.py ack --all --reason "…"`（迷ったらこれ）
  - 技データ（data/moves）だけの変更は記録不要（`calcFingerprint` が変わったキャラだけ自動で回る）。

## 4. 締める
- `upstream/tacpendium.lock` を取り込んだ SHA に更新する。
- `docs/progress/upstream-<短縮SHA>-report.md` に書く: 分類表／取り込んだ・取り込まなかった変更と理由／テスト結果の逐語／単体版の挙動が変わった点（利用者に見える変化）。
- コミット（例 `feat(upstream): 公開版 <短縮SHA> の setplay 変更を取り込み`）・push。
