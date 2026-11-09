# 公開版 tacpendium v0.4.0（`71c8d7a`）の取り込み — 本家の整理と多言語化（2026-11-09）

- 取り込む版: `71c8d7a28432191f3542c2e9ee27edd240aca3a5`（公開版のタグ `v0.4.0`）。基準（前の lock）: `ebd2768d6f19287e36ab68f3f1b8e64f806ec1bf`（`v0.3.0`）。
- 開始コミット `601e18a`（main。PR #23 のマージ）。ブランチ `claude/beautiful-babbage-jm17hf`。手順は `/sync_upstream`。
- **新キャラ・データ・エンジンの変更は無い版**。見張るパスのうち `character_data/`・`migrations/`・`internal/service/setplay/`・`internal/repository/setplay/` の差は 0 ファイル（`git diff --stat ebd2768 71c8d7a -- <見張るパス>`）。2 週間の待ちは要らない。
- 同じブランチで、本家の対応言語の案内（`guide.upstreamLanguages`）も直した（開発者の依頼。下の「同じ PR の ux の修正」）。

## 分類（`bash upstream/watch.sh` の 40 ファイル）

| 分類 | 本家のファイル | 単体版での扱い |
|---|---|---|
| 表記の生成器 | `internal/seedgen/generate_m2002.go`（`resolveCharacter` を取り出した）、新規 `resolve_character.go`（`resolveCharacter` を手順ごとの関数〔`charResolver`・`indexTokens`・`resolveBaseRows`・`reservedAliasTexts`・`synthesizeRushRows`・`sortResolved` ほか〕へ分けた。本家は振る舞いを変えないと凍結 golden で固定） | 移植。単体版の変更（`resolveCharacterWith`・`collisionResolver`）を新しいファイルへ当て直した。`generate_m2002.go` は本家と同じになった（import のパスだけ） |
| 本家テスト | `resolve_character_golden_test.go`（入力ごと凍結した golden。4 形式 × 写した 2 キャラ＋合成の行）、`resolve_character_test.go`（分岐 2 本） | 移植（変更なし）。`testdata/`（golden 41,643 行・入力の CSV 2 つ）は移さず、公開版のチェックアウトの同じパスを読む（既存の seedgen の試験と同じ。`docs/upstream-diff.md`） |
| 定数・モデル | `internal/model/move.go`（`MoveCodeJumpForward`・`DashForward`・`JumpNeutral` のコメントに、本家の確定反撃サーチの用途を追記） | 当てた（コメントだけ）。`Move` 構造体へのコメント（hunk 3）は、単体版がこの構造体を持たないので当てない |
| 単体版に関係ない | `internal/model/api_error.go`（パスワードの照合の一時拒否）・`combo.go`（コンボのダメージの値域）・`combo_damage_sync_test.go` | 取り込まない（本家のアカウント・コンボの機能） |
| 単体版に関係ない（ただし生成器に影響） | `internal/api/setplay/`（`handler.go`・`routes.go`・試験 3 つ）: echo v4 → v5（`echo.Context` → `*echo.Context`、`SetParamNames`/`SetParamValues` → `SetPathValues`） | ハンドラは取り込まない（本家の API）。**ただし本家の正解を作る生成器（`scripts/upstream-oracle/standalone_oracle_test.go.txt`）は本家のこのパッケージに入れて回すので、echo v5 に合わせて直した**（直さないとコンパイルできない。本家の試験と同じ書き換え） |
| 単体版に関係ない | `web/src/features/setplay/`（本家の画面の部品分け・色のトークン化） | 取り込まない（本家の画面。単体版の画面は別に作っている） |
| 文言 | `web/src/locales/*.json`（ja・en の整理、ko・zh-Hans・zh-Hant を新規）、試験 4 つ | `setplay.*` は ja・en とも追加 0・削除 0・変更 0（キー単位で突き合わせた）。単体版の文言は変えない。ほかは本家の画面の文言 |

## 単体版の挙動が変わった点（利用者に見える変化）

- **取り込みによる変化は無い**: moves DB は 32 キャラとも変わらない（下の検証）。提案の計算も変わらない（エンジン・データの差が無い。本家の正解の作り直しも一致＝下）。
- `data/moves/index.json` の `dataVersion` が lock の SHA（`71c8d7a…`）になる（前回の lock 更新と同じ扱い）。

## 同じ PR の ux の修正（開発者の依頼 2026-11-09）

- `guide.upstreamLanguages`（5 言語）: 「Tacpendium の画面は日本語のみ対応です(英語は一部、ほかの言語は未対応)。」→「Tacpendium の画面は日本語・英語・中国語(簡体字・繁体字)・韓国語に対応しています。」（en・ko・zh-Hans は手で、zh-Hant は `pnpm gen:locales`）。公開版 v0.4.0 も 5 言語のロケールを持つ（`web/src/locales/` の `ko.json`・`zh-Hans.json`・`zh-Hant.json`）。
- 試験の文（`App.test.tsx`）を直した。古い文に戻すと赤（`EXIT=1`・`1 failed | 1 passed | 54 skipped (56)`。`-t "本家"`）。
- 古い記述の検索（AGENTS §8-8）: 「日本語のみ」「英語は一部」「only partly」「仅支持日语」「僅支援日語」「일본어만」を、`node_modules`・`.upstream`・`.git`・ビルドの出力（`dist`・`dist-local`・`site`）・`docs/progress/`（過去の記録）を除いて検索し、残り 1 件。その 1 件は追跡していないローカル版のバイナリ（`build/local/`。作り直すと消える）。
- 更新履歴に 1 行（`upstreamLanguages`、2026-11-09。5 言語）。

## 判断と理由

- **`resolveCharacter` の分割も移植する**: 本家と同じパス・関数名に保つ方針（S1 後の開発者決定 2）。2026-10-31 の health の観測で、単体版の Go の最大の複雑度（gocyclo 45）だったのがこの関数で、上流由来なので直さず本家で直すのを待つとしていた。その本家の整理が入る。
- **正解の生成器を echo v5 に直す**: lock を上げると CI が正解を作り直す（`scripts/upstream-oracle/changed.sh` の入力に lock がある）。直さないと本家のパッケージでコンパイルできない。書き換えは本家の試験と同じ 2 か所。
- **verify-plan は `--all`**: lock と生成器（seedgen）が変わったので全部回す（迷ったら全部）。
- **本家の ko・zh の文言は取り込まない**: 単体版の `setplay.*` に当たるキーは本家の ja・en で変わっていない。本家が ko・zh を持ったので、単体版の機械翻訳の文（ko・zh）を本家の訳に寄せる余地はある（残課題）。

## 検証（手元。Node 22.23.4・pnpm 10.34.7。出力はファイルへ書き、`EXIT=` と件数で判定）

```
$ bash upstream/watch.sh                         → 取り込み前 EXIT=1(変化あり 40 ファイル)/ lock 更新後 EXIT=0(変化なし)
$ go run ./cmd/movesdb -out <tmp> -prev data/moves → EXIT=0 違反 0 件・版が変わったキャラ 0 []
                                                    オラクル numeric・srk: 本家と同じ 2790
                                                    diff -r <tmp> data/moves: 差は index.json だけ(dataVersion の SHA)
$ bash scripts/upstream-oracle/run.sh <tmp>      → EXIT=0 cells 5376(本家 v0.4.0・echo v5 で 407 秒)
                                                    cmp <tmp> engine/testdata/upstream_oracle.json → 一致(CMP_EXIT=0)
$ gofmt -l internal engine cmd                   → 出力 0 行
$ go vet ./...                                   → EXIT=0
$ go test -count=1 -v ./...                      → EXIT=0 PASS 399(サブテスト込み。v0.3.0 の取り込み時 396 ＋ 移植した 3)/ FAIL 0 / SKIP 0
                                                    突き合わせた条件: 5376(キャラ 32 ・不一致 0)
$ go test -count=1 -v ./internal/seedgen/        → EXIT=0 PASS 118 / FAIL 0 / SKIP 0
$ python3 -I scripts/ci/verify-plan.py check     → 違反 0 件(ack --all を記録)
$ pnpm assets                                    → EXIT=0
$ npx tsc --noEmit -p .                          → EXIT=0
$ pnpm test                                      → EXIT=0 Test Files 22 passed (22) / Tests 247 passed (247)
$ pnpm build                                     → EXIT=0 合計 6807646 bytes / 違反 0 件
$ bash scripts/local/build.sh                    → EXIT=0
$ PW_CHROMIUM_PATH=… pnpm e2e                    → EXIT=0 35 passed (1.0m)
```

- seedgen の試験が黙って減っていないこと（AGENTS §8-11）: 取り込み前の main（`601e18a`）の seedgen を、公開版 v0.3.0 のチェックアウト（`TACPENDIUM_SRC`）で回すと PASS 115（サブテスト込み）。取り込み後は 118。名前で突き合わせて、消えた試験 0、増えた試験 3（`TestResolveCharacterGolden`・`TestResolveLayerARow_MissingTokenIsIndexSkip`・`TestSortResolved_UnfilledTieBreaksByReason`）。v0.3.0 の報告の「seedgen PASS 126」は数え方が違う（移植した `internal/migrationpin` を含めた等。推測）。同じ数え方での比較は上の 115 → 118。
- 本家の正解の作り直しは、ほかの重い処理と同時には回していない（時間は合否の判断に使っていない）。

```
$ git diff --stat 601e18a   # この報告と ux の修正を含む
 data/moves/index.json                              |   2 +-
 docs/progress/upstream-71c8d7a-report.md           |  46 +++
 docs/upstream-diff.md                              |   2 +-
 internal/model/move.go                             |   8 +-
 internal/seedgen/generate_m2002.go                 | 255 +---------------
 internal/seedgen/resolve_character.go              | 333 +++++++++++++++++++++
 internal/seedgen/resolve_character_golden_test.go  | 243 +++++++++++++++
 internal/seedgen/resolve_character_test.go         |  71 +++++
 scripts/ci/logic-version.json                      |  22 +-
 .../upstream-oracle/standalone_oracle_test.go.txt  |   5 +-
 upstream/tacpendium.lock                           |   2 +-
 web/src/components/App.test.tsx                    |   2 +-
 web/src/lib/changelog.ts                           |   1 +
 web/src/locales/en.json                            |   3 +-
 web/src/locales/ja.json                            |   3 +-
 web/src/locales/ko.json                            |   3 +-
 web/src/locales/zh-Hans.json                       |   3 +-
 web/src/locales/zh-Hant.json                       |   3 +-
 18 files changed, 736 insertions(+), 271 deletions(-)
```

## 残課題

- 本家の ko・zh-Hans・zh-Hant の `setplay.*` の訳と、単体版の ko・zh の文を見比べ、寄せるかを決める（単体版の ko・zh は機械翻訳の注記を出している）。
- 次回の `/audit_scale`: `seedgen.resolveCharacterWith`（45）が分割で下がる見込み。
