# 公開版 tacpendium v0.3.0（`5840f6e`）の取り込み — アルジュンほか（2026-10-25）

- 取り込む版: `5840f6ee0092fa6fcf3be55ca0c94844f3eec015`（公開版のタグ `v0.3.0`）。基準（前の lock）: `ffdb69e468a3f5c0b0e16255e68b6baa3266796d`（`v0.2.0`）。
- 開始コミット `5cafce6`（作業ブランチ `claude/s9-web-deployment-t0w876`）。手順は `/sync_upstream`。
- **新キャラ（アルジュン）を含む版**。ゲームでの配信開始は 2026-10-13（火。開発者の連絡）で、2 週間後の **2026-10-27（火）にマージ・デプロイする**（開発者の予定）。それまで PR をマージしない。既存キャラの変更（キンバリーの派生・表記の規則）も同じ版にあるが、既存キャラの調整はいつ取り込んでもよいので、版ごと取り込む。
- ★公開版のコミットのメールの書き換え（開発者。2026-10-25）で、v0.3.0 は **`ebd2768d6f19287e36ab68f3f1b8e64f806ec1bf`** になった（中身は同じ。下の「公開版の SHA の書き換え」）。lock はこの SHA。報告のファイル名の `5840f6e` は取り込んだ時点の SHA のまま残す。

## 分類（`bash upstream/watch.sh` の 41 ファイル）

| 分類 | 本家のファイル | 単体版での扱い |
|---|---|---|
| データ | `character_data/arjun.csv`（新規・85 行）、`migrations/000015_data_seed_arjun`、`migrations/000014_data_preset_aliases_tc_aerial_and_kimberly_derivations`（キンバリーの疾駆けの派生 6 技の親・表記の書き換え） | 再生成で取り込み（`go run ./cmd/movesdb`）。コードの変更はキャラ数の検査（31 → 32）だけ |
| データ（記録） | `character_data/chain-cancel-measurements.md`・`command-correction-history.md`・`seed-progress.md` | 本家の作業記録。取り込まない |
| 表記の生成器 | `internal/seedgen/`（`format.go`・`generate_m2002.go`・`generate_m4808.go`、新規 `generate_m4305.go`・`generate_m4308.go` とテスト） | 移植（本家の差分をそのまま当てた。当たらない箇所は無かった）。テストが使う `internal/migrationpin/` も移植した（見張るパスに追加） |
| エンジン | `internal/service/setplay/approach.go`（アルジュンの接近技 14 個） | 移植 |
| エンジン（整理） | `internal/service/setplay/service.go` → 新規 `suggest_for_combo.go`（`SuggestForCombo` の本体を手順ごとの関数へ分けた。本家は振る舞いを変えないと golden で固定）、`withoutParentsHash`・`markAlreadyAdopted`（採用済みの印） | 関数分けは `SuggestForKnockdown` のまま同じファイル・同じ関数名で移植（3 方向のマージ。`docs/upstream-diff.md`）。採用済みの印は単体版に無いので移さない |
| 本家テスト | `suggest_for_combo_branches_test.go` | 移植（取り消し・エンジンの誤りの分岐 5 本） |
| 本家テスト | `suggest_for_combo_golden_test.go`・`testdata/suggest_for_combo_golden.json`、`service_m4305_test.go` | 移さない（fixture の `goldenMoves` だけ移した）。golden の出力と m4305 は採用済みの印・重複判定の呼び出し順を見るため。関数分けの後も同じ振る舞いであることは、下の「本家の正解との突き合わせ」が全件で見る |
| 定数・モデル | `internal/model/move.go`（`MoveCodeJumpNeutral`） | 移植（使わないが本家と同じに保つ） |
| 単体版に関係ない | `internal/model/`（`api_error.go`・`combo.go`・`position.go`・`punish.go` とテスト）、`internal/api/setplay/`（ハンドラと golden）、`web/src/locales/*.json`（更新の確認・コンボの手順の画面。`setplay.limitations._marker` の注記にファイル名が 1 つ増えただけ）、`web/src/features/setplay/components/SetplayLimitationNotice.tsx`（同じ注記） | 取り込まない（コンボ・確定反撃・本家の画面・API の機能） |

## 単体版の挙動が変わった点（利用者に見える変化）

1. **アルジュンを選べる**（32 キャラ。技 94、状態「神気の巨巌接地」、安全飛び 真 43・5F 42・6F 41）。画面の名前は ja「アルジュン」・en「Arjun」（ko・zh は英語名。ほかのキャラと同じ）。
2. **表記（ナンバリング・SRK）**: ターゲットコンボが `LP>LP` → `5LP~5LP(TC)`（SRK `st.LP~st.LP (TC)`）、空中から出す技に `j.`（例 アキの蚣蝮 `2HP (蚣蝮)` → `j.2HP`）。そのため、被りを分けていた区別印の一部が要らなくなった（例 アキのしゃがみ強P `2HP (しゃがみ強P)` → `2HP`）。既存 31 キャラで numeric 236 技・SRK 228 技の表記が変わった。日本語技名・英語技名は変わらない。
3. **キンバリー**: 疾駆け（OD 疾駆け）からの派生 6 技（急停止・首刈り・影すべり と OD 版）に親が付き、レシピの先頭に「疾駆け >」が付く。計算の結果が変わったのはキンバリーの 53 条件だけ（本家の正解の差。ほかの 30 キャラの 5,040 条件は 1 件も変わっていない）。
   - ★票の鍵（RecipeKey）は親の手を含むので、この 6 技を含むレシピの鍵が変わる。公開から 2 日の間にそれらへ入った票があれば、新しい鍵には引き継がれない（D1 に残るが画面には出ない）。開発者が D1 の Console で `SELECT recipe, ka, good, bad FROM votes WHERE char = 'kimberly' AND (recipe LIKE '%emergency_stop%' OR recipe LIKE '%torso_cleaver%' OR recipe LIKE '%shadow_slide%');` を回し、**0 行**（2026-10-26）。引き継ぎの要る票は無い。
4. **更新履歴**に 2026-10-27 の 1 行（アルジュン・表記の変更。5 言語）。

## 判断と理由

- **版ごと取り込む**: アルジュンの解禁日（10-27）にマージする前提なら、既存キャラの変更と分ける必要がない（`/sync_upstream` の規則で、既存キャラの調整は即時でよい）。
- **関数分けも移植する**: 本家と同じパス・関数名に保つ方針（S1 後の開発者決定 2）。そのままにすると、次からの本家の差分が `suggest_for_combo.go` に当たり、移植の手間が毎回増える。振る舞いが変わらないことは、本家の新しい実装で作り直した正解と全件で一致することで確かめた。
- **golden を移さない**: 出力の半分が採用済みの印と重複判定の呼び出し順で、単体版に無い機能。ほかの部分だけを抜き出して凍結し直すより、本家の実装との全件突き合わせのほうが強い。
- **`internal/migrationpin` を移植する**: seedgen のテスト（m4305・m4307・m4808）が使う。標準ライブラリだけで、本家と同じパスに置けば次の差分も当たる。
- **verify-plan は `--all`**: 計算に効くコード（エンジン・生成器・lock）が変わったので全部回す。

## 検証（手元。Node v22.23.4・pnpm 10.34.7。出力はファイルへ書き、終了コードと件数で判定）

```
$ bash upstream/watch.sh                         → EXIT=1(取り込み前: 変化あり 41 ファイル)
$ go run ./cmd/movesdb -out <tmp> -prev data/moves → EXIT=0 キャラ 32 / 技 3151 / 違反 0 件
                                                    版が変わったキャラ 2 [kimberly arjun]
                                                    オラクル numeric・srk: 本家と同じ 2790 / 説明のつかない差 0
$ bash scripts/upstream-oracle/run.sh            → EXIT=0 cells 5376(本家 v0.3.0 のハンドラ・サービス・SQL を migrations 15 本の DB で)
$ go vet ./...                                   → EXIT=0
$ gofmt -l internal engine cmd                   → 出力なし
$ go test -count=1 -v ./...                      → EXIT=0 PASS 396 / FAIL 0 / SKIP 0
                                                    突き合わせた条件: 5376(キャラ 32 ・不一致 0)
$ python3 -I scripts/ci/verify-plan.py check     → 違反 0 件(ack --all を記録)
$ pnpm assets                                    → EXIT=0
$ pnpm exec tsc --noEmit -p .                    → EXIT=0
$ pnpm exec vitest --run                         → EXIT=0 Test Files 21 passed (21) / Tests 230 passed (230)
$ pnpm build                                     → EXIT=0 ファイル 43 / 合計 6795794 bytes / 違反 0 件
$ PW_CHROMIUM_PATH=/opt/pw-browsers/chromium-1194/chrome-linux/chrome pnpm exec playwright test
                                                 → EXIT=0 32 passed(retries: 0。アルジュンの 1 本を追加)
```

- seedgen は単独でも `go test -v`: PASS 126 / FAIL 0 / SKIP 0（移植したテストは公開版のチェックアウトと突き合わせる）。
- setplay は単独で PASS 133（移植した分岐のテスト 5 本を含む）。
- E2E は本家の正解の作り直し（重い処理）と同時に回した。合否だけを見ていて、時間は見ていない（AGENTS §8-6）。
- 正解の差: 既存キャラはキンバリー 53 条件だけが変わり、アルジュンの 168 条件（8 KA × 21 クエリ）が増えた（`git show HEAD~:engine/testdata/upstream_oracle.json` と比べた）。

### `git diff --stat 5cafce6`（生成物 `data/moves`・`engine/testdata` とこの報告を除く）

```
 docs/data/moves-db.md                              |   4 +-
 docs/engine.md                                     |   2 +-
 docs/precompute.md                                 |   2 +-
 docs/upstream-diff.md                              |  11 +
 internal/migrationpin/applied.sha256               |  30 ++
 internal/migrationpin/migrationpin.go              | 152 ++++++
 internal/migrationpin/migrationpin_test.go         | 133 +++++
 internal/model/move.go                             |   7 +
 internal/movesdb/check.go                          |   4 +-
 internal/seedgen/format.go                         |  27 +-
 internal/seedgen/generate_m2002.go                 |  48 +-
 internal/seedgen/generate_m2002_test.go            |   2 +
 internal/seedgen/generate_m2006_test.go            |  16 +-
 internal/seedgen/generate_m4305.go                 | 562 +++++++++++++++++++++
 internal/seedgen/generate_m4305_test.go            | 329 ++++++++++++
 internal/seedgen/generate_m4307_pin_test.go        | 120 +++++
 internal/seedgen/generate_m4308.go                 | 312 ++++++++++++
 internal/seedgen/generate_m4308_test.go            | 159 ++++++
 internal/seedgen/generate_m4808.go                 |  10 +-
 internal/seedgen/generate_m4808_test.go            | 143 ++++--
 internal/service/setplay/approach.go               |   2 +
 internal/service/setplay/service.go                | 139 -----
 internal/service/setplay/suggest_for_combo.go      | 225 +++++++++
 .../setplay/suggest_for_combo_branches_test.go     | 152 ++++++
 .../setplay/suggest_for_combo_golden_test.go       |  37 ++
 scripts/ci/logic-version.json                      |  25 +-
 upstream/tacpendium.lock                           |   2 +-
 upstream/watch.sh                                  |   1 +
 web/e2e/setplay.spec.ts                            |  10 +
 web/src/lib/changelog.ts                           |   5 +-
 web/src/locales/en.json                            |   1 +
 web/src/locales/ja.json                            |   1 +
 web/src/locales/ko.json                            |   1 +
 web/src/locales/zh-Hans.json                       |   1 +
 web/src/locales/zh-Hant.json                       |   1 +
 35 files changed, 2482 insertions(+), 194 deletions(-)
```

生成物: `data/moves`（32 キャラ + index、新規 `arjun.json`）と `engine/testdata/upstream_oracle.json`（+1823 / −55）。試験は追加・増加のみ（`generate_m2006_test.go`・`generate_m4808_test.go` の書き換えは本家の差分そのもの）。

## 公開版の SHA の書き換え（2026-10-25）

開発者が公開版 tacpendium のコミットのメール（作者・コミッター）を書き換え、全コミットの SHA が変わった（中身は変えない）。

- 確かめたこと（公開版を clone し直して）: `main` と `v0.3.0` の先は `ebd2768`。**tree は `d96e7978bd1f2dba67a360ff9fb5b839c36ef44a` で、書き換え前の `5840f6e` と同じ**。`v0.2.0` も旧 `ffdb69e` と新 `acab5f9` で tree が同じ（`50d93a50…`）。作者・コミッターのメールは GitHub の noreply の 1 種類だけ。旧 `5840f6e` はどのブランチ・タグからもたどれない（GitHub はまだ渡すが、いつ消えるか分からないので lock を移した）。
- `upstream/tacpendium.lock` → `ebd2768`。
- moves DB: main のデータ（取り込み前）を `-prev` にして作り直し、版が変わったキャラ（キンバリー・アルジュン）の `dataVersion` を `ebd2768` にした。差分は `index.json`・`kimberly.json`・`arjun.json` の版の文字列だけ（7 行）。
  - ほかの 30 キャラの `dataVersion` は前の値 `ffdb69e…`（旧 v0.2.0）のまま残る。キャラごとの版は計算に効く中身が変わったときだけ上がる決まり（`docs/data/moves-db.md`）で、中身は変わっていないため。**`ffdb69e` は書き換え後の `acab5f9`（v0.2.0）と同じ中身**。そのキャラのデータが次に変わったときに新しい SHA になる。
- 本家の正解: `ebd2768` で作り直し、コミット済みのものと 1 バイトも違わないことを確かめた。
  `bash scripts/upstream-oracle/run.sh <tmp>` → EXIT=0（`TestStandaloneOracle` PASS）、`cmp engine/testdata/upstream_oracle.json <tmp>` → 0（一致）。
  その後 `go test -count=1 ./engine/ ./internal/movesdb/ ./cmd/movesdb/` → EXIT=0（3 パッケージ ok）、`pnpm assets`・`pnpm build` → EXIT=0・違反 0 件。
- verify-plan: 中身の同じ lock の付け替えなので `ack --none`。

## 残課題・次への入力

- （済）公開版のコミットのメールの書き換えへの追従: 下の「公開版の SHA の書き換え」。
- **火曜（2026-10-27）**: PR をマージ → 「Cloudflare へ出す」を承認 → サイトでアルジュンを選んで提案が出るか確かめる。更新履歴の日付が 10-27 なので、ずれたら直す。
- **ローカル版**: データを埋め込んでいるので、新しいタグ（例 `v1.1.0`）で出し直す（`docs/local.md`「配布」）。サイトの更新（有利の表示・ヘッダ・更新履歴）も入る。
- 本家の golden（`suggest_for_combo_golden.json`）は移していない。次に本家が `suggest_for_combo.go` を変えたら、単体版でも本家の正解との突き合わせで確かめる。
