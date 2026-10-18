# S6 計測と投票 — レビュー

- 対象: `git diff b386ea0cff0f1c449007b27c38a4e312f9e98bae HEAD`（HEAD = `586d91e`、5 コミット、77 files changed, 3014 insertions(+), 1851 deletions(-)）
- 基準コミットは `docs/progress/S6-report.md` の計画節に記載のものと一致。
- レビュアーは実装の文脈を持たない（意図的）。読み取り専用で、書いたのは本ファイルだけ。

## 1. 要約

- エンジンの票（`applyVotes`）は本家の並べ替えの**後**・`limit` の**前**に安定ソートで掛かり、票なしは `SuggestJSON` とバイト列まで同じ。Go の全テスト・web の tsc / vitest / build / E2E は手元で全部緑だった（§5）。わざと壊すと票の試験が赤くなることも 2 通り確かめた。
- `RecipeKey` が 1 つの結果の中で重ならないことを、試験の範囲（3 キャラ × KA 50）より広く、全キャラ × KA 5 通り × クエリ 5 通り（776,549 行）で確かめた。重なりは 0 件。
- 完了条件の「実行ごとのイベントが見える」「票が保存され第一キーに効く」「票なしは本家と一致」は、偽の D1 / Analytics Engine の上で E2E まで通っている。本物の Cloudflare では動かしていない（docs に限界として書いてある。デプロイは段階の範囲外）。
- 指摘は **高 0・中 3・低 6**。中は (1) beacon が D1 の障害に引きずられて落ちる、(2) プライバシーポリシーの計測項目が実際に送る項目より少ない、(3)「乱数は翌日に捨てる」の書き方が実装・基盤より強い、の 3 件。
- 完了報告 `S6-report.md` はレビュー時点で計画節しか無い（やったこと・証拠・`git diff --stat` が未記入）。/stage の流れではレビューの後に書くものなので指摘には数えないが、締める前に必ず埋めること。

## 2. 完了条件への適合

| 項目（HANDOVER §9 S6・報告の計画節） | 判定 | 根拠 |
|---|---|---|
| 実行ごとのイベントが見える | 済（偽の AE の上で） | `web/e2e/telemetry.spec.ts:23-46`。画面 → sendBeacon → Worker の本物のコード（`dist-edge/local.mjs`）→ メモリの AE に 1 点、キャッシュ時も 1 点。手元の E2E 21 passed |
| 票が保存され、並べ替えの第一キーに効く | 済（偽の D1 = `node:sqlite` の上で） | `web/e2e/telemetry.spec.ts:48-75`（251 行目の行に票 → D1 に保存 → 再検索で先頭、2 番目は票なしの並びの先頭）。Go 側は `engine/votes_test.go` |
| 票なしの並びは本家と一致（ゴールデンが緑のまま） | 済 | `go test -count=1 ./...` EXIT=0（engine 162 s。本家突き合わせ・golden を含む）。`TestVotes_NoneKeepsUpstreamOrder` が票なし（省略・`""`・`{}`・`null`・当たらない票）で `SuggestJSON` とバイト列一致を確認 |
| 並べ替えの後で limit を切る（AGENTS §4） | 済 | `internal/service/setplay/service.go:427`・`:488`。`limit` の切り取りより前。mutation で確認（§5） |
| 繋ぎ表の仕組みの削除（S5 後の開発者決定 2） | 済 | `table.go`・`table_test.go`・`LoadTable`・`loadTable`・`-tables`・CI の sweep を削除。`grep -rn "loadTable\|LoadTable\|FillerTable\|繋ぎ表\|-tables\|tableFormatVersion\|TableSource\|engineFor\|ErrFillerTable"`（`_reference`・`node_modules`・`.upstream`・`dist`・`.git`・S0〜S5 の報告を除く）で残りは「消した」と書いた履歴の記述だけ。`service.go` と本家の差分は、票の 2 行とフィールドのほかは S3 からの既知の差分だけ |
| beacon Worker・Analytics Engine | 済（コードと雛形。デプロイは範囲外） | `web/edge/index.ts`・`beacon.ts`・`beacon-layout.json`・`wrangler.toml` |
| ダッシュボード（§12-4 = 集計スクリプト） | 済（実アカウントでは未実行） | `web/scripts/analytics/report.mjs`・`.github/workflows/analytics.yml`（`workflow_dispatch` のみ・`permissions: contents: read`・Action は SHA 固定）。SQL は偽の API でしか試していない（`docs/telemetry.md` の限界に書いてある） |
| プライバシーポリシー（5 言語） | 一部 | 画面・5 言語・キー集合一致は済。ただし計測項目の書き漏れがある（指摘 中-2）、ソルトの破棄の書き方（指摘 中-3） |
| グッド/バッドの受け口と保存（D17・§12-6 の決定） | 済 | `web/edge/votes.ts`・`schema.sql`。重複防止（hash × レシピ × 日）・1 日 200・1 分 30 回 |
| 計画の「`worker/`」 | 逸脱（場所だけ） | 実装は `web/edge/`。報告で場所の変更と理由を書くこと（指摘 低-6） |
| 完了報告 | 未（レビュー時点） | `S6-report.md` は計画節のみ |

## 3. 指摘

### 高

指摘なし。見たもの: エンジン・サービスの票の配線と並べ替えの位置、本家 `service.go` との差分、golden・本家突き合わせの結果、i18n キー集合、`_reference/` の変更（`git diff --stat b386ea0 HEAD -- _reference` は空）、秘密情報（`wrangler.toml` の `database_id` はゼロの雛形。トークンはコード・docs に無い）、公式データの混入（新しいデータファイルは無い）。

### 中

**中-1 beacon（計測）が D1 の障害・無料枠超過で落ちる**

- 場所: `web/edge/index.ts:67-71`（`visitorOf`）・`:73-84`（`handleBeacon`）・`:121-124`（catch で 503）、`web/edge/visitor.ts:39-52`（`SaltStore.today`）
- 内容: beacon の受け口は利用者のハッシュを作るために、毎回 `SaltStore.today(env.DB)` を通る。isolate がその日のソルトをまだ覚えていなければ、D1 に書き込み（`INSERT OR IGNORE`＋`DELETE` 2 本の batch）と読み込みをする。D1 が誤りを返すと（無料枠を超えて UTC 0 時まで誤りになる場合。S1 ⑦）例外になり、外側の catch で **503 になって点は書かれない**。
- なぜ問題か: 計測（Analytics Engine）と票（D1）は無料枠が別なのに、D1 が止まると、新しい isolate に来た beacon が全部落ちる。`env.ts:37` のコメント「無ければ…計測は利用者のハッシュなしで書く」は D1 の束縛が**無い**ときだけで、D1 が**誤りを返す**ときは当てはまらない。§7.1 の「止まるのは計測だけ」とは別の向きで、票の障害が計測を止める。`docs/telemetry.md` の「無料枠を超えたとき」にもこの連鎖は書いていない。
- 直し方の案: `handleBeacon` の中で `visitorOf` を try/catch で囲み、失敗したら `visitor = ""`（または「不明」を表す値。§8-7 に従い、ハッシュが空の理由を区別できる値）で点を書く。Worker の試験に「D1 が誤りを返しても beacon は 204 で 1 点書く」を足す（今ある `index.test.ts:229` の「D1 が誤りを返したら 503」は票だけを見ている）。

**中-2 プライバシーポリシーの「送る内容」が、実際に送る beacon の項目より少ない**

- 場所: `web/src/locales/ja.json` の `privacy.measureBody`（ほか 4 言語も同じ内容）と `web/src/shared/api.ts:47-90`（`beaconSchema`）
- 内容: 文面に挙がっているのは「キャラ・KA・モードと絞り込みの条件・表記と言語・結果の件数・計算にかかった時間・データ版」。実際には次も送って保存している: 実行の結果の種別（結果／誤り／中断）と誤りの code（`outcome`・`errorCode`）、キャッシュから出したか（`cacheHit`）、受け取った件数と打ち切り（`items`・`truncated`）、票の読み込みの状態と票の付いたレシピの数（`votesStatus`・`votesApplied`）、抽出率（`sampleRate`）。利用者のハッシュは「利用者の区別」の節で触れているが、**計測の点にも入る**こと（`beacon-layout.json` の `visitor`）は書いていない（投票の節と混ざって読める）。
- なぜ問題か: HANDOVER §7.3 は「計測項目を明記する」。今の文面は実装より狭く、テスト（`App.test.tsx:494` は「IP アドレスそのものは保存しません」しか見ない）では気づけない。
- 直し方の案: `measureBody` に上の項目を足す（5 言語同時。§8-9）。ハッシュの節に「実行の計測と投票の重複防止の両方に使う」と書く。可能なら、beacon の項目一覧と文面の対応を docs（`docs/telemetry.md`）に表で持ち、項目を足すときの確認先にする。

**中-3「乱数は翌日に捨てる」の書き方が、実装と基盤の性質より強い**

- 場所: `web/src/locales/*.json` の `privacy.hashBody`、`docs/telemetry.md`「利用者のハッシュと日替わりソルト」、`web/edge/visitor.ts:1-3,35-52`
- 内容（実装）: 前日のソルト・`ballots` を消すのは、**翌日になって最初に D1 を使う要求が来たとき**（`SaltStore.today` の `DELETE`）。要求が来なければ残り続ける。
- 内容（基盤。推測: Cloudflare D1 の Time Travel は常に有効で、無料プランでも過去 7 日程度の任意時点に戻せる、という公開情報に基づく。実アカウントで未確認）: `DELETE` した後も、アカウントの持ち主は Time Travel でソルトの入った時点に D1 を戻せる。ソルトが戻れば、IPv4 全体 × よくある UA の総当たりで、Analytics Engine に 3 か月残る `visitor`（`beacon-layout.json`）から IP を当てうる。
- なぜ問題か: §7.3・D14 は「ソルトは翌日破棄」。利用者に見える文面（5 言語）が「捨てるため…結び付けることはできません」と言い切っており、D14 の法的照合（S7）の前提になる。テストは緑のままで、読み手しか気づけない種類の食い違い（優先度の基準では高に近いが、基盤の性質の部分が推測なので中とした）。
- 直し方の案: (a) 文面を実装どおりに弱める（「翌日以降に消します」「消した後もサービス基盤のバックアップに一定期間残ることがあります」等）、(b) ソルトを D1 に置かない（例: 日付と Worker の秘密値から導く → 秘密情報が要るので開発者判断、または isolate のメモリだけに置き、isolate ごとに別のソルトになるのを受け入れる → 利用者数の推定が粗くなる）、(c) S7 の法的照合の論点に Time Travel を加える。少なくとも (c) を報告の「残課題」に書くこと。

### 低

**低-1 自分の票の数が、後で読み直した票の数より優先され続ける**

- 場所: `web/src/components/Results.tsx:131-139`（`good={mine?.good ?? counted?.[0] ?? 0}`）、`web/src/components/App.tsx`（`myVotes` は消さない）
- 内容: 一度押した行は、`myVotes` に投票時点の good/bad が残り、次の検索で `/api/votes` から新しい数を読んでも、表示は古い `mine` の数のまま（ページを開いている間ずっと）。並び（エンジンに渡す票）は新しい数で決まるので、表示の数と並びが合わないことがある。
- 直し方の案: 新しい計算の結果を出すとき（`votes` を読み直したとき）に、そのキャラ・KA の `myVotes` の `good`/`bad` を null に戻す（`value` は残す）。または表示を `counted` 優先にし、`mine` は投票直後から次の計算までだけ使う。

**低-2 `postVote` が受け口の返した `value` を捨てる**

- 場所: `web/src/lib/votes.ts:59-61`
- 内容: 応答の `value` を読まずに、送った `value` を返す。今の Worker は `req.value` をそのまま返すので実害は無いが、`VoteReply` の注記（`shared/api.ts`「value はこの人の今日の票」）と、`castVote` が将来「既に同じ票」などで別の値を返した場合にずれる。
- 直し方の案: 応答の `value` を `VOTE_VALUES` で検証して使う（zod の形を `shared/api.ts` に置き、Worker の応答もそれで作る）。

**低-3 事前計算 CLI の試験が、表の削除と関係なく狭くなった**

- 場所: `cmd/precompute/main_test.go`（`TestRun_OutputServesSameResults`）
- 内容: 表あり／なしの比較をやめたのは当然だが、同時にクエリ `n_min=2&n_max=4&approach_only=true&target_types=…` を消し、KA を `{0, 30, 67, 100}` から `{80}` だけにした。この試験の目的（CLI → ファイル → engine の配線）には大きく効かないが、範囲を狭めた事実が報告に無い（§8-2）。
- 直し方の案: 報告に「狭めた」と書くか、KA の標本を戻す（表が無いので速い）。

**低-4 E2E の beacon の回数制限の余裕が見えない**

- 場所: `web/edge/index.ts:23`（`BEACON_PER_MINUTE = 30`）、`web/edge/testing/local.ts`、`web/playwright.config.ts`
- 内容: E2E は全テストで 1 つのサーバ（同じ IP 空・同じ UA ＝ 同じ `visitor`）を共有し、固定の 1 分窓で 30 回を数える。`conformance`（9 条件）＋`setplay`（`generate`/`goto` が 18 か所）＋`telemetry` で、1 分窓に 30 回近く送る見込み（推測: grep からの概算）。31 回目以降は 429 で点が書かれず、`telemetry.spec.ts:28` の `before + 1` が赤くなる。手元では、分の境目に合わせて始めた実行（全体 38 秒が 1 つの窓に収まる）でも緑だったので、今は 30 未満に収まっている。ただ余裕は測っていない。また D1 の束縛が無いときは全員の鍵が `"anonymous"` になり、全利用者で 1 分 30 回を共有する（`index.ts:81`）。
- 直し方の案: `createLocalApi` の回数制限を E2E 用に緩めるか、telemetry spec の冒頭で窓をまたぐのを待つのではなく、点を数える前後で 429 が出ていないことを `/__e2e/` の口で返す。`"anonymous"` のときは回数制限を掛けないか、別の上限にする。

**低-5 集計の待ち時間の分位点が、画面の抽出率で重み付けされていない**

- 場所: `web/scripts/analytics/report.mjs`（`quantileExactWeighted(…)(${ms}, _sample_interval)`）
- 内容: 件数は `_sample_interval / sampleRate` で割り戻すのに、分位点の重みは `_sample_interval` だけ。期間の途中で `beaconSampleRate` を変えると、抽出率が高かった期間の値に寄る。日ごとの推定利用者数（`count(DISTINCT visitor)`）も抽出率 < 1 では小さく出る（表の題に注記が無い）。
- 直し方の案: 重みを `_sample_interval / sampleRate` に揃える（整数が要るなら丸める）。利用者数の表に「抽出率 1 未満の期間は過小」と書く。

**低-6 計画の置き場（`worker/`）と実装（`web/edge/`）が違う**

- 場所: `docs/progress/S6-report.md` の「作るもの」表、実装 `web/edge/`
- 内容: 置き場を変えたこと自体は妥当（`web/src/shared/api.ts` の zod の形を画面と共有できる）だが、計画のまま。報告の「判断と理由」に書くこと。

## 4. 良かった点

- 票の並べ替えを Go の 1 か所（`applyVotes`・`RecipeKey`）に置き、画面は `recipeKey` を受け取るだけにしている。鍵の作り方が TS と Go に二重にならない。
- 票なしのとき `SuggestJSON` と**バイト列まで同じ**ことを、票の形 4 通りで試験している。本家突き合わせ・golden の経路を票が汚さないことが明確。
- `TestVotes_AppliedBeforeLimit` は「limit の外の行が票で入る」「totalFound が変わらない」「残りは本家の順」を同時に見ていて、AGENTS §4 の打ち切りの順序の不具合を確実に捕まえる（mutation で確認）。
- null と 0 を分けている（beacon のモード外の項目は null、AE では -1・空文字。`nMax` の 0 は「上限なし」と区別）。§8-7 に沿う。
- 票の取得に失敗・時間切れでも本家の順で出し、その状態を画面（`votes-note`）と beacon（`votesStatus`）の両方に出す。HANDOVER §3 の「票の受け口が止まっても提案は止まらない」が試験されている。
- `/api/votes` の `no-store` は E2E で見つけた実際の不具合への対処で、理由がコメントと docs にある。
- 集計用トークンのリスクの説明（権限・漏れた場合・Roll）が具体的で、開発者の質問に答えている。

## 5. 自分で回したコマンドと結果

出力はすべて scratchpad（`/tmp/claude-0/.../scratchpad/`）のファイルに書き、合否は `EXIT=` で判定した。

```
$ go test -count=1 ./... > go.txt 2>&1; echo "EXIT=$?"
ok  	github.com/plexiblinp/tacpendium-setplay/cmd/movesdb	0.500s
ok  	github.com/plexiblinp/tacpendium-setplay/cmd/precompute	4.500s
ok  	github.com/plexiblinp/tacpendium-setplay/engine	162.259s
ok  	github.com/plexiblinp/tacpendium-setplay/internal/gameversion	0.002s
?   	github.com/plexiblinp/tacpendium-setplay/internal/model	[no test files]
ok  	github.com/plexiblinp/tacpendium-setplay/internal/moveindex	0.006s
ok  	github.com/plexiblinp/tacpendium-setplay/internal/movesdb	0.793s
ok  	github.com/plexiblinp/tacpendium-setplay/internal/repository/setplay	0.007s
ok  	github.com/plexiblinp/tacpendium-setplay/internal/sanumber	0.005s
ok  	github.com/plexiblinp/tacpendium-setplay/internal/seedgen	1.244s
ok  	github.com/plexiblinp/tacpendium-setplay/internal/service/setplay	1.159s
EXIT=0
```

```
$ cd web && pnpm exec tsc --noEmit            → EXIT=0
$ pnpm exec vitest --run                      → Test Files  14 passed (14) / Tests  144 passed (144) / EXIT=0
$ pnpm assets && pnpm build                   → 違反: 0 件 / 束ねた: dist-edge/worker.mjs, local.mjs / EXIT=0
$ PW_CHROMIUM_PATH=/opt/pw-browsers/chromium-1194/chrome-linux/chrome pnpm exec playwright test
                                              → 21 passed (38.9s) / EXIT=0
```

（vitest の件数は ANSI エスケープを落としてから grep した。）

回数制限の確認（低-4）: 分の 58 秒に合わせて E2E を始め、全体を 1 つの 1 分窓に収めて再実行 → `21 passed (38.2s)`、`EXIT=0`（10:52:58 開始、10:53:37 終了）。

mutation（AGENTS §8-3。リポジトリの外の写しで実施。リポジトリは変更していない）:

```
# 1) suggestFillerOnly の applyVotes を消す
$ go test -count=1 -run 'TestVotes' ./engine/
--- FAIL: TestVotes_FillerOnly (0.01s)
FAIL	github.com/plexiblinp/tacpendium-setplay/engine	0.347s
EXIT=1

# 2) meaty/gap の applyVotes を limit の切り取りの後へ動かす
$ go test -count=1 -run 'TestVotes' ./engine/
--- FAIL: TestVotes_AppliedBeforeLimit (0.02s)
FAIL	github.com/plexiblinp/tacpendium-setplay/engine	0.459s
EXIT=1
```

RecipeKey の重なりの確認（リポジトリの外の小さな Go プログラム。`engine` を `replace` で参照）: `data/moves/*.json` の全キャラ × KA {15, 30, 45, 60, 75} × クエリ {`limit=3000`, `mode=gap&limit=3000`, `mode=filler&remain=4&limit=3000`, `target_types=normal,unique,special,special_projectile,throw&limit=3000`, `chain_cancel_only=true&limit=3000`} で、結果の中の `recipeKey` の重なりを数えた。

```
checked 776549 dups 0
EXIT=0
```

技 code の形（`RECIPE_KEY_RE`・`CODE_RE` との整合）: `data/moves/*.json` の `code` 3,151 件を `^[a-z0-9_]{1,64}$` で照合 → 不一致 0 件。各キャラ内の `code` の重複も 0 件。

その他の確認: `git diff --stat b386ea0 HEAD -- _reference` は空（`_reference/` の変更なし）。i18n のキー集合は ja と en / ko / zh-Hans / zh-Hant で差 0（Python で照合）。

## 取り込み（実装担当が追記）

| 指摘 | 採否 | 理由・対応 |
|---|---|---|
| 中-1 beacon が D1 の障害で落ちる | 採用 | `handleBeacon` で `visitorOf` の失敗を捕まえ、`visitor = "unavailable"`（束縛が無いときの空文字と区別）で書く。ハッシュが無いときは別の回数制限（isolate の全員で 1 分 600 回）。試験「D1 が誤りを返しても beacon は 204 で 1 点書く」を追加し、try/catch を外すと赤になることを確かめた |
| 中-2 ポリシーの送る項目が足りない | 採用 | 5 言語の `privacy.measureBody` に outcome・誤りの種類・件数と打ち切り・計算済みの表示・票の読み込みと数・送る割合と、ハッシュを付けて保存することを足した。`hashBody` に計測と投票の両方に使うと書いた。`docs/telemetry.md` に項目 ↔ 文面の対応を置いた |
| 中-3「翌日に捨てる」が強すぎる | 採用 | 文面を「日付が変わった後の最初のアクセスで消す」「基盤の復旧用の記録に一定期間残ることがある」に弱めた（5 言語）。Time Travel は推測のまま `docs/telemetry.md` と完了報告の残課題（S7 の法的照合の論点）に書いた。ソルトの置き場の変更（秘密情報が要る案）はしない |
| 低-1 自分の票の数が残り続ける | 採用 | 票を読み直したとき、そのキャラ・KA の `myVotes` の good/bad を null に戻す（押した状態は残す）。画面の試験を足し、戻しを外すと赤になることを確かめた |
| 低-2 `postVote` が応答の value を捨てる | 採用 | `voteReplySchema`（shared/api.ts）で応答を検証して使う |
| 低-3 precompute の試験が狭くなった | 採用 | KA {0, 30, 67, 100} と絞り込みのクエリを戻した（全体で 0 件なら赤） |
| 低-4 E2E の回数制限の余裕 | 採用 | `createLocalApi`（E2E・手元用）の回数制限を緩めた（本番の値は変えない）。`anonymous` の扱いは中-1 の別の上限で対応 |
| 低-5 分位点の重み | 採用 | `toUInt32(round(_sample_interval / sampleRate))` で重み付け。利用者数の表の題に「抽出率 1 未満の日は過小」 |
| 低-6 計画の置き場と違う | 採用 | 完了報告の「判断と理由」に書く |

## 再レビュー（2 回目）

- 対象: 取り込み `git diff 586d91e HEAD`（コミット `0c50045`、18 files changed, 293 insertions(+), 52 deletions(-)。うち 179 行は本ファイル）。段階全体は `git diff --stat b386ea0 HEAD` で 78 files changed, 3234 insertions(+), 1830 deletions(-)。`_reference/` の変更は無し（`git diff --stat b386ea0 HEAD -- _reference` が空）。
- レビュアーは実装の文脈を持たない。書いたのは本節だけ（読み取り専用。mutation はリポジトリの外の写しで実施）。

### 1. 要約

- 1 回目の指摘 9 件（中 3・低 6）は、中-3 の一部と低-6（報告に書く約束）を除いて直っている。中-1（beacon を D1 の障害から切り離す）と低-1（自分の票の数の残り）は、直し方を外すと試験が赤になることを自分でも確かめた。
- 手元の検査は全部緑（Go EXIT=0、tsc EXIT=0、vitest 145 passed、build EXIT=0、E2E 21 passed、verify-plan 違反 0 件）。vitest は 1 回目の 144 から 1 増（中-1 の試験）。
- 新しい指摘は **高 0・中 1・低 5**。中は、中-3 の文面の弱め方が、利用者向けの 5 言語では直ったのに、docs とコードのコメントに強い言い方が 2 か所残っていること（AGENTS §8-8 の grep 漏れ）。
- 低は、低-1 の直し方の端（票の読み込みに失敗したとき押した行の数が 0 になる・投票と読み直しの競合）、直したのに試験が無い箇所（応答の `value`・分位点の重み・ハッシュなしの回数制限）、新しく入った `visitor = "unavailable"` が利用者数に 1 人として数えられること、取り込み表の過去形（完了報告にまだ書いていない）。

### 2. 1 回目の指摘ごとの確認

| 指摘 | 判定 | 根拠 |
|---|---|---|
| 中-1 beacon が D1 の障害で落ちる | 済 | `web/edge/index.ts:84-93`。`visitorOf` の例外を捕まえ `VISITOR_UNAVAILABLE` で 1 点書く。束縛なし（`""`）と区別（§8-7）。試験 `web/edge/index.test.ts:152-158`。mutation: try/catch を外すと `1 failed | 26 passed (27)`、EXIT=1（§5）。票の受け口（`handleVote`）は従来どおり D1 の誤りで 503（`index.ts:112`、外側の catch）で、意図どおり |
| 中-2 ポリシーの項目不足 | 済 | 5 言語の `privacy.measureBody` に outcome・誤りの種類・件数と打ち切り・計算済みの表示・票の読み込みと数・送る割合・ハッシュの付与を追加。`beaconSchema`（`web/src/shared/api.ts:47-90`）と突き合わせて漏れは見つからなかった（N/G/remain・安全飛び・種別・特定の技・ダメージ 0・並び順は「モードと絞り込みの条件」に含む）。参照先の見出し名（「利用者の区別」/ Telling visitors apart / 이용자 구별 / 区分访问者 / 區分訪問者）は各ロケールの `hashTitle` と一致。`docs/telemetry.md:40` に項目 ↔ 文面の対応 |
| 中-3「翌日に捨てる」が強い | 一部 | 5 言語の `hashBody` は遅延削除と復旧用の記録を書いた形に直り、`docs/telemetry.md:58-59` も直った。ただし同じ docs の 98 行目と `web/edge/visitor.ts:1-3` に強い言い方が残る（新しい指摘 中-1）。完了報告の残課題への記載はまだ無い（低-5） |
| 低-1 自分の票の数が残る | 済（端に問題あり） | `web/src/components/App.tsx:183-189`。試験 `App.test.tsx:490-493`。mutation: 戻しを外すと `1 failed | 24 passed (25)`、EXIT=1。鍵の接頭辞 `char|ka|` は末尾の `|` があるので `ryu|4|` と `ryu|40|` は混ざらない。端の問題は低-1 |
| 低-2 `postVote` が応答の value を捨てる | 済（試験なし） | `web/src/lib/votes.ts:66-67` で `voteReplySchema`（`shared/api.ts:115-121`）を通す。Worker の応答（`web/edge/votes.ts:59`）は good/bad/value の 3 項目だけなので `strictObject` に通る（E2E の投票が緑）。応答の value を使うことの試験は無い（低-2） |
| 低-3 precompute の試験が狭い | 済 | `cmd/precompute/main_test.go`。KA {0, 30, 67, 100} × 4 クエリに戻し、キャラごとに合計 0 件なら Fatal。`scripts/ci/logic-version.json` に試験のみの記録（decision none）を足し、`verify-plan.py check` は違反 0 件 |
| 低-4 E2E の回数制限の余裕 | 済 | `web/edge/testing/local.ts:10,21-23` で E2E・手元だけ 1 分 10,000 回。本番の値（`index.ts:23-25`）は不変。`anonymous` は中-1 の別の上限（1 分 600 回）で扱う。ハッシュなしの上限の試験は無い（低-3） |
| 低-5 分位点の重み | 済（試験なし） | `web/scripts/analytics/report.mjs:35,59`。利用者数の表の題に「抽出率 1 未満の日は過小」。重みの試験は無い（低-3） |
| 低-6 計画の置き場 | 未（約束どおり後で） | `S6-report.md:36,44` はまだ `worker/`。取り込み表は「書く」（未来形）なので、締める前に書けばよい |

### 3. 新しい指摘

#### 高

指摘なし。見たもの: 取り込みの全差分、beacon の新しい分岐（ハッシュあり／束縛なし／D1 の誤り）と回数制限の鍵、`voteReplySchema` と Worker の応答の形、`myVotes` の戻しとキャッシュ（投票でキャッシュを全部捨てるので、キャッシュから出す結果が `myVotes` より古い数を出すことは無い）、i18n のキー集合（ja と他 4 言語で差 0）と非日本語ロケールへのかなの混入（0 件。Python で `[぀-ヿ]` を照合）。

#### 中

**中-1 中-3 の弱め方が docs とコードのコメントに行き渡っていない（§8-8 の grep 漏れ）**

- 場所: `docs/telemetry.md:98`（「生の IP なし・ハッシュは翌日に意味を失う」）、`web/edge/visitor.ts:1-3`（「翌日になったら前日以前のソルトと投票の記録(ballots)を消す（ソルトを捨てると、前日のハッシュから IP を当てることも…できなくなる）」）
- 内容: 利用者向けの 5 言語と `docs/telemetry.md:58-59` は「日付が変わった後の最初の要求で消す（来なければ残る）」「基盤の復旧用の記録に残りうる（Time Travel。推測）」に直したが、同じ docs の 98 行目（集計用トークンのリスクの説明）は「翌日に意味を失う」と言い切ったまま。`visitor.ts` の冒頭コメントも「翌日になったら消す」で、遅延削除（同じファイルの 31 行目「日が変わって初めて引いたときに」は正確）と Time Travel の注意が無い。
- なぜ問題か: 98 行目は、トークンが漏れた場合の影響を開発者が判断する材料で、58-59 行目と食い違う（同じ文書の中で、ハッシュから IP を当てうるかの結論が逆）。テストでは気づけない種類。検索語 `捨て|翌日|破棄` を `web/src`・`web/edge`・`web/scripts`・`docs/telemetry.md`・`docs/progress/S6-report.md` で走査して、残りはこの 2 か所（ほかの「捨てる」はキャッシュ・Worker 等の別の話）。英語の `next day` 等は locale の `hashBody` 以外に該当なし（locale は直っている）。
- 直し方の案: 98 行目を「生の IP なし。ハッシュはソルトを消した後は IP に戻せない（ただし D1 の Time Travel でソルトを戻せる可能性がある。上の『利用者のハッシュと日替わりソルト』）」に。`visitor.ts:1-3` を「日が変わった後の最初の要求で前日以前を消す（遅延削除）。消した後も D1 の復旧機能で戻せる可能性は docs/telemetry.md」に。

#### 低

**低-1 低-1 の直し方の端: 読み直しに失敗したとき、押した行の数が 0 になる／投票と読み直しが重なると古い数に戻る**

- 場所: `web/src/components/App.tsx:181-189`（戻し）、`web/src/components/Results.tsx:131-139`（`good={mine?.good ?? counted?.[0] ?? 0}`）、`web/src/lib/votes.ts:34-41`（失敗時は `counts: {}`）
- 内容: (a) `fetchVotes` が失敗・時間切れ（`status: "unavailable"`）でも戻しは走るので、自分がグッドを押した行（`aria-pressed=true`）の数が 0/0 になる。上限（`VOTES_MAX_ENTRIES`）の外のレシピでも同じ。直す前は投票の応答の数を出していた。(b) 投票の送信中に「提案を出す」を押し、`/api/votes` の読み取りが投票の書き込みより前に D1 を読むと、応答の後で戻しが走って、正しい応答の数を古い数で上書きしうる（推測: 順序の分析のみ。再現はしていない）。
- なぜ問題か: 表示の数と押した状態が食い違う。並び（エンジン）には影響しない。
- 直し方の案: 戻しを `votes.status !== "unavailable"` のときに限る。(b) は戻しを「送信中（`state: "sending"`）の鍵には掛けない」で大半を避けられる。

**低-2 応答の `value` を使うことに試験が無い**

- 場所: `web/src/lib/votes.test.ts:31-36`
- 内容: 試験の応答の `value` はどれも送った値と同じ（`value: 1` を送って `value: 1`）。`postVote` を 1 回目の形（送った `value` を返す）に戻しても緑のまま（推測: 試験の入力からの判断。mutation は回していない）。範囲外の値（例 `value: 2`）で null になる試験も無い。
- 直し方の案: 送った値と違う `value`（例: 1 を送って応答 0）を返す fetch で、応答の値が返ることを見る。`value: 2` で null。

**低-3 取り込みで足した分岐・式のうち 2 つに試験が無い**

- 場所: `web/scripts/analytics/report.test.mjs:20`（`qs[0]` の件数の式だけ見る）、`web/edge/index.test.ts:72`（`anonBeaconLimiter` は 1000 回で作るだけ）
- 内容: 分位点の重み（`WI`）を `_sample_interval` に戻しても、ハッシュなしの beacon を `beaconLimiter` に回しても（または `anonymous` 鍵を使わなくしても）緑のまま（推測: 試験の内容からの判断。mutation は回していない）。また `toUInt32(round(_sample_interval / sampleRate))` は丸めるので、`_sample_interval = 1`・抽出率 0.4 の点は重み 2（真は 2.5）になる。抽出率を混ぜた期間でだけ効く小さな偏り。
- 直し方の案: 分位点の SQL が `WI` の式を含むことの試験を 1 本。「D1 の束縛が無い beacon は `anonBeaconLimiter` で数え、上限を超えたら 429」の試験を 1 本（小さい上限の limiter を渡す）。丸めは報告の限界に 1 行。

**低-4 `visitor = "unavailable"` と空文字が「推定利用者数」に 1 人ずつ数えられる**

- 場所: `web/scripts/analytics/report.mjs:47`（`count(DISTINCT visitor)`）、`web/edge/index.ts:27`
- 内容: ハッシュの無い点（空文字・`unavailable`）も DISTINCT の 1 値として数える。D1 が止まった日は利用者が実際より 1 多く、ハッシュの付いた点が少ない分だけ少なく出る。空文字は 1 回目からあったが、取り込みで値が 1 つ増えた。
- 直し方の案: `countIf(DISTINCT …)` 相当で `visitor NOT IN ('', 'unavailable')` に絞り、別の列に「ハッシュなしの実行数」を出す（D1 の障害の見張りにもなる）。

**低-5 取り込み表の過去形が、まだ無い成果物を指す（§8-2）**

- 場所: 本ファイル「取り込み」の中-3 の行（「完了報告の残課題（S7 の法的照合の論点）に書いた」）、`docs/progress/S6-report.md`（58 行・計画節のみ。`grep -n "Time Travel"` で 0 件）
- 内容: 完了報告にはまだ書かれていない。/stage の流れでは報告はこの後に書くので、書き方の問題。
- 直し方の案: 完了報告を書くときに、残課題（Time Travel・S7 の論点）、低-6（置き場の変更と理由）、1 回目の §1 で求めた「やったこと・証拠・`git diff --stat`」を入れ、取り込み表の記述と合わせる。

### 4. 良かった点

- 中-1 で、束縛なし（空文字）と D1 の誤り（`unavailable`）を別の値にしたのは §8-7 に沿う。ハッシュなしの beacon を別の緩い上限に回したので、1 回目の「全利用者で 1 分 30 回を共有する」問題も同時に解けている。
- E2E 用の回数制限を `testing/local.ts` だけで緩め、本番の定数に触れていない。
- `voteReplySchema` を `shared/api.ts` に置き、型 `VoteReply` をそこから導いた（Worker と画面が同じ形を共有する）。
- precompute の試験に「キャラごとに合計 0 件なら Fatal」を足し、比べた意味の無い緑を防いでいる。試験だけの変更でも `logic-version.json` に decision none で記録し、CI の検査と整合している。

### 5. 自分で回したコマンドと結果

出力は scratchpad のファイルに書き、合否は `EXIT=` で判定した（vitest・playwright の件数は ANSI エスケープを落としてから grep）。

```
$ go test -count=1 ./cmd/... ./engine/ ./internal/... > go2.txt 2>&1; echo "EXIT=$?"
ok  	github.com/plexiblinp/tacpendium-setplay/cmd/movesdb	0.547s
ok  	github.com/plexiblinp/tacpendium-setplay/cmd/precompute	28.047s
ok  	github.com/plexiblinp/tacpendium-setplay/engine	165.537s
ok  	github.com/plexiblinp/tacpendium-setplay/internal/gameversion	0.005s
?   	github.com/plexiblinp/tacpendium-setplay/internal/model	[no test files]
ok  	github.com/plexiblinp/tacpendium-setplay/internal/moveindex	0.010s
ok  	github.com/plexiblinp/tacpendium-setplay/internal/movesdb	0.955s
ok  	github.com/plexiblinp/tacpendium-setplay/internal/repository/setplay	0.009s
ok  	github.com/plexiblinp/tacpendium-setplay/internal/sanumber	0.015s
ok  	github.com/plexiblinp/tacpendium-setplay/internal/seedgen	1.400s
ok  	github.com/plexiblinp/tacpendium-setplay/internal/service/setplay	1.813s
EXIT=0
```

（precompute の 28 秒は vitest と並行して回したため。時間の計測ではない。）

```
$ cd web && pnpm exec tsc --noEmit             → EXIT=0
$ pnpm exec vitest --run                       → Test Files  14 passed (14) / Tests  145 passed (145) / EXIT=0
$ pnpm build                                   → 違反: 0 件 / 束ねた: dist-edge/worker.mjs, local.mjs / EXIT=0
$ PW_CHROMIUM_PATH=/opt/pw-browsers/chromium-1194/chrome-linux/chrome pnpm exec playwright test
                                               → 21 passed (37.9s) / EXIT=0
$ python3 -I scripts/ci/verify-plan.py check   → 見張るコード 6 か所: 記録と一致 / 違反: 0 件 / EXIT=0
```

mutation（`web/` を scratchpad に写し、`node_modules` はリンク。リポジトリは変更していない）:

```
# 1) handleBeacon の try/catch を外す(visitor = await visitorOf(...) のみ)
$ pnpm exec vitest --run edge/index.test.ts
 × D1 が誤りを返しても beacon は 204 で 1 点書く(visitor は unavailable。票の障害で計測を止めない)
 Tests  1 failed | 26 passed (27)
EXIT=1

# 2) App.tsx の myVotes の戻しを外す(next[k] = v)
$ pnpm exec vitest --run src/components/App.test.tsx
 × 投票の後は覚えた結果を捨て、同じ条件でも票を読み直して計算する
 Tests  1 failed | 24 passed (25)
EXIT=1
```

その他: i18n のキー集合は ja と en / ko / zh-Hans / zh-Hant で差 0、非日本語 4 ロケールのかな混入 0 件（Python）。`git diff --stat b386ea0 HEAD -- _reference` は空。

## 取り込み（2 回目。実装担当が追記）

| 指摘 | 採否 | 理由・対応 |
|---|---|---|
| 中: 強い言い方が 2 か所に残る | 採用 | `docs/telemetry.md`（トークンの節）と `web/edge/visitor.ts` の冒頭を、遅延削除・復旧用の記録に残りうる書き方に揃えた |
| 低 1: 票を読めないと自分の行が 0/0・投票と読み直しの競合 | 一部採用 | 読めなかったときは押した時点の数を残す。投票と読み直しが重なったときの順序の問題は再現しておらず、表示だけの問題なので残課題 |
| 低 2: 応答の value を使う試験が無い | 採用 | `votes.test.ts` に、送った値と違う value を返す応答・形の違う応答の試験を足した |
| 低 3: 分位点の重み・ハッシュなしの回数制限に試験が無い | 採用 | `report.test.mjs`・`edge/index.test.ts` に足した |
| 低 4: 空文字・unavailable が利用者 1 人ずつに数えられる | 不採用（注記） | AE の SQL で条件つきの種類数を確かめていないため、表の題に注記した。誤差は最大 2 人/日 |
| 低 5: 完了報告が未記入 | 採用 | 完了報告に記入する（中-3 の残課題・低-6 の判断を含む） |

再レビューは 2 往復で打ち切る（/stage の規約）。残りは完了報告の「残課題」へ。
