# 2026-10-31 health: 規模・複雑度の観測を入れ、増えた App.tsx をリファクタリング

- 種類: `health`（この作業で新しく作った種類。AGENTS §3 の表に足した）。開始コミット `2692ae5`。ブランチ `claude/beautiful-babbage-jm17hf`（同じ日の `security` の作業〔undici〕の後に積んだ）。
- 開発者の指示（2026-10-31）: 本家の「規模をチェックするカスタムコマンド」（`/audit_scale_health`）を本リポジトリにも入れ、複雑度が増えているコードはリファクタリングする。
- 開発者の決定（AskUserQuestion）: **軽量版を新しく作る**／直す対象は**単体版だけのコード**／比べる基準は**公開時点**。
- 利用者に見える変更は無い（挙動を変えないリファクタリング）。更新履歴には足さない。

## やったこと

### 1. 観測の仕組み（軽量版）

| ファイル | 中身 |
|---|---|
| `scripts/observe-scale.sh` | 機械部。`[<commit>]` を渡すと git worktree に出して測る（作業ツリーには触らない。終わったら worktree を消す）。行数・gocyclo・scc・試験の数（静的）を出し、末尾の `== COUNTS ==` に key=value。測れないものは `-1` |
| `docs/audits/scale-baseline.md` | 系列表（1 回 1 列。所見は書かない）。公開時点・観測 1 回目・リファクタ後の 3 列 |
| `.claude/commands/audit_scale.md` | `/audit_scale [前回の commit]`。調査だけ（コードは直さない） |
| `docs/maintenance.md`「health」 | 手順・道具の版・対象の選び方・守り |
| `AGENTS.md` §3・`.claude/commands/maint.md` | 作業の種類に `health` を足した |

本家から写さなかったもの（公開版 tacpendium の `docs/audits/scale-health-runbook.md`〔790 行〕・`scale-health-baseline.md`・`scripts/observe-scale-scan.sh` を読んで判断）: board・followup の記録、エンドポイント・テーブル・マイグレーションの数、`make release-archives` の容量、ペース（コミット・著者比率）、eslint の数、`go test` の時間。理由: 単体版にその仕組みが無いか、AGENTS が軽く保つと決めているため（§1「本家の強い規約は移さない」）。配信物の容量は `pnpm build` の検査（`check-dist-limits.mjs`）がすでに見ている。**本家側から追加でもらうファイルは要らない**（参考は公開版にそろっていた）。

### 2. 公開時点との比較（系列表の 1・2 列目）

| 項目 | 公開時点 `65672aa` | いま `2692ae5` |
|---|---|---|
| Go prod 行 | 8,587 | 9,783 |
| gocyclo 平均 / 最大 | 5.76 / 41 | 5.47 / 45 |
| gocyclo 15 超え / 20 超え | 15 / 11 | 15 / 11 |
| scc TypeScript Complexity | 775 | 818 |

関数・ファイルごとに突き合わせた結果（`cyclo-sorted.txt` をファイルの位置 + 関数名で、`scc.csv` をファイルで）:

- **Go: 単体版だけのコードで複雑度が増えた関数は 0 個**。増えたのは上流由来だけ。`seedgen.resolveCharacterWith` 41→45（`internal/seedgen/generate_m2002.go`）、新しく入った `generate_m4305.go`・`generate_m4308.go`・`suggest_for_combo.go`・`migrationpin.go`。v0.3.0 の取り込み（アルジュン）で入った。
- **TS: 増えた単体版のコードは `web/src/components/App.tsx`（scc Complexity 83→89）と `Results.tsx`（39→43）**。新しいファイル `web/src/lib/advantage.ts`（18）も単体版。
- 「上流由来」の判定: 同じパスのファイルが取得済みの公開版 tacpendium（`upstream/tacpendium.lock` の `ebd2768`）にあるか。`scripts/observe-scale.sh` の上位の表の 5 列目。

### 3. リファクタリング（App.tsx）

挙動は変えずに、React に依らない部分を純関数へ、起動時の読み込みを Hooks へ切り出した。

- `web/src/lib/appRun.ts`（新規）:
  - `myVoteKey`: 自分の票の鍵。`App` の 2 か所と `Results` の 1 か所で同じ文字列を別々に組み立てていたので 1 つにした。
  - `forgetVoteCounts`: 票を読み直したとき、そのキャラ・KA の数だけを忘れる処理。
  - `resultMetrics`・`NO_RESULT_METRICS`: 計測に載せる結果の数。同じ 3 項目を 4 か所で書いていた。
  - `withSharedSafeJump`: 共有 URL の安全飛びの引き直し。元は引数の条件を書き換えていたので、新しい条件を返す形にした（値は同じ）。
  - `isPlainLeftClick`: サイト名のリンクの押し方の判定。
- `web/src/components/useAppData.ts`（新規）: `useManifest`・`useSampleRate`（`App` の `useEffect` 2 つ）。
- 結果: `App.tsx` は scc Complexity **89 → 74**（公開時点の 83 より下）、Code 366 → 333 行。`Results.tsx` は 43 のまま（鍵を関数に替えただけ）。TS 全体は 818 → 817（切り出した先の 13 が足されるので、総量はほぼ同じ。狙いは 1 ファイルの集中を減らし、純関数を試験できるようにすること）。

検討した案: (a) `execute`（計算の実行。キャッシュ・票・計測を順に行う）を丸ごと Hook に移す。中断のトークン・pending・キャッシュの ref を共有するので、移すと境界が増えて読みにくくなると判断し、今回は数の組み立てだけを外へ出した。(b) ヘッダの JSX を部品に分ける。複雑度への効きが小さいので見送った。

## 判断と理由

- **`health` を 4 つ目の種類にした**: 観測（定期）とリファクタリング（挙動を変えない）は、`security`・`ux`・`data` のどれにも当たらないため。推測: AGENTS の規約の変更にあたるが、開発者の指示で入れた仕組みの置き場所なので聞かずに進めた。不要なら `ux` に寄せる。
- **試験の数を静的に数える**: 過去のコミットも同じ物差しで測るため（過去の木では依存を入れてテストを回さない）。実行した件数とは一致しない（`it.each` の展開、E2E の site と local の 2 つの project）。系列表に書いた。
- **8 列を超えたら古い列から消す**: 本家は 4 列＋退避ファイル。ここでは退避ファイルを作らず、git の履歴に任せた（軽くするため）。
- **リファクタリングの候補として残したもの**（高いが公開後に増えていない単体版のコード。直すかは開発者が決める）: `internal/movesdb/name_en.go` `GenerateNameEN`（37）、`internal/movesdb/notation.go` `buildNotation`（29）、`internal/movesdb/check.go` `CheckFile`（29）、`internal/movesdb/build.go` `buildCharacter`（26）、`cmd/precompute/main.go` `run`（24）、`engine/query.go` `ParseQuery`（19。エンジンなので golden と WASM の経路の試験が要る）、TS の `web/src/lib/conditions.ts`（scc 108。最大）・`ConditionsPanel.tsx`（58）。

## 検査（AGENTS §8）

新しい試験 `web/src/lib/appRun.test.ts`（6 件）を、1 つずつ壊して赤くなることを確かめた（型・ビルドが通る壊し方。壊したら元に戻した＝`diff -q` で一致）:

| 壊し方 | 結果 |
|---|---|
| 前方一致の区切り `\|` を外す（KA 3 を忘れると KA 30 も消える） | 最初の試験は**緑のまま**だった（KA 30 を忘れて KA 3 が残るか、の向きしか見ていなかった）。逆向きの確認を足して `EXIT=1`・`1 failed \| 5 passed (6)` |
| `items` を件数（totalFound）にする | `EXIT=1`・`1 failed \| 56 passed (57)` |
| 安全飛びで元の条件を書き換える | `EXIT=1`・`1 failed \| 56 passed (57)` |
| `altKey` を見ない | `EXIT=1`・`1 failed \| 56 passed (57)` |
| 数を忘れるときに押した状態も消す | `EXIT=1`・`2 failed \| 55 passed (57)`（App の試験も赤） |

`scripts/observe-scale.sh` の値を道具の直接の結果と突き合わせた（「1 件ある入力で 1 件と出る」）: gocyclo `-over 20` を直に回して 11 件（スクリプト 11）、`-over 15` は 15 件（同 15）、`scc` の `App.tsx` は 74（同 74）、`appRun.test.ts` の `it(` は 6（vitest 6）。

全体（出力はファイルへ書き、`EXIT=` と件数で判定。Node 22.23.4）:

```
$ npx tsc --noEmit -p .                 TSC_EXIT=0
$ pnpm build                            BUILD_EXIT=0（違反: 0 件）
$ bash scripts/local/build.sh           LOCALBUILD_EXIT=0
$ pnpm test                             VITEST_EXIT=0  Test Files  22 passed (22) / Tests  239 passed (239)
$ PW_CHROMIUM_PATH=… pnpm e2e           E2E_EXIT=0  32 passed (1.8m)
$ python3 -I scripts/check-private-refs.py   PRIVATE_REFS_EXIT=0（走査: 336 ファイル・違反: 0 件）
$ python3 scripts/check-licenses.py          LICENSES_EXIT=0（違反: 0 件）
$ bash -n（CI と同じ対象のシェルスクリプト）   構文の誤り 0
$ bash scripts/observe-scale.sh 65672aa / 2692ae5 / 3840023   EXIT=0（3 回。COUNTS は系列表のとおり）
```

Go は変えていないので `go test` は回していない（観測のスクリプトは Go のソースを読むだけ）。

```
$ git diff --stat 2692ae5   # 同じ日の security（undici）の変更を含む。報告を書いた後に vitest（239 passed (239)）と observe-scale.sh 3840023（COUNTS が一致）を回し直した
 .claude/commands/audit_scale.md                    |  22 ++++
 .claude/commands/maint.md                          |   7 +-
 AGENTS.md                                          |   3 +-
 docs/audits/scale-baseline.md                      |  33 ++++++
 docs/maintenance.md                                |  22 +++-
 docs/progress/2026-10-31-health-scale-report.md    | 105 +++++++++++++++++
 docs/progress/2026-10-31-security-undici-report.md |  82 +++++++++++++
 scripts/observe-scale.sh                           | 130 +++++++++++++++++++++
 web/pnpm-lock.yaml                                 |  11 +-
 web/pnpm-workspace.yaml                            |   7 ++
 web/src/components/App.tsx                         |  72 +++---------
 web/src/components/Results.tsx                     |   3 +-
 web/src/components/useAppData.ts                   |  37 ++++++
 web/src/lib/appRun.test.ts                         |  57 +++++++++
 web/src/lib/appRun.ts                              |  49 ++++++++
 15 files changed, 575 insertions(+), 65 deletions(-)
```

試験は消していない（Web 233 → 239 件。増えた 6 件は `appRun.test.ts`）。

## 残課題

- 上の「リファクタリングの候補」。とくに `movesdb` の 4 関数（gocyclo 26〜37）と `conditions.ts`。
- 上流由来の `seedgen.resolveCharacterWith`（45）は直さない。必要なら本家で。
- 観測は月 1 回。次回は `/audit_scale 3840023`（系列表の最新列）。

## 配信・PR・タグ

- 配信物（`web/dist`）は App の JS が変わる（挙動は同じ）。**デプロイはしてよいが急がない**。security の undici の変更（配信物は変わらない）と同じ PR になるので、マージ後の「Cloudflare へ出す」を 1 回承認すれば足りる。
- ローカル版のタグは要らない（画面の挙動が変わらないため。次のタグのときに一緒に入る）。
- PR は頼まれたら作る。
