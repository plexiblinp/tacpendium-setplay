# S5 フロント — レビュー

- 基準コミット: `5f1f7ffa9b3b4cbe687ed36dbeec9616a7a0e2b1`（呼び出し元の指定。`S5-report.md` の記録と一致）
- 対象: `git diff 5f1f7ffa HEAD`（HEAD = `7866662`。73 files, +10055 / −33）
- レビュアーは実装の文脈を持たない。読み取り専用（書いたのはこのファイルだけ）。

## 1. 要約

- 完了条件「主要フローの E2E が緑」は、手元で回して **16 passed / EXIT=0**（本家の正解との sha256 一致 9 条件 + 主要フロー 7 本）。tsc・vitest（83 件）・`go test ./cmd/precompute/` も緑。
- ただし **`S5-report.md` は計画節と開発者決定だけで、「やったこと」「判断と理由」「残課題」「次の段階への入力」、検査の逐語（`EXIT=`・件数）、`git diff --stat`、§8-3 の「わざと壊して赤」の記録、docs が参照している「計測」の節が無い**。現状では「緑」に報告上の証拠が無い（呼び出し元の言うとおり途中であることは踏まえたうえで、未として挙げる）。
- コードの不具合として、**WASM（Go）が一度 panic / メモリ不足で終了すると、同じ Worker への次の要求が永遠に返らない**経路がある（高）。docs の「Worker が落ちたら `worker_failed`」とも食い違う。
- そのほか、0 件のときの「絞り込みを外すと…」の出し分けが本家と違う（中）、共有 URL で範囲の前後関係を検査しない（中）、`_headers` に配らない `sp/` の規則が残る（低）など。

## 2. 完了条件への適合

| 項目 | 判定 | 根拠 |
|---|---|---|
| HANDOVER §9 S5「主要フローの E2E が緑」 | 済（手元） | 下 §5 の逐語。16 passed、`retries: 0`・workers 1（`web/playwright.config.ts`） |
| 画面（キャラ・KA・3 モード・種別・ダメージ 0・特定の技・N/G/remain・安全飛び 3 種・並び順・絞り込み 3 つ） | 済 | `ConditionsPanel.tsx`・`formState.ts`。エンジンへのクエリは本家 `setplayApi.getSuggestions` と同じ規則（`conditions.ts` `toEngineQuery` を本家と突き合わせた） |
| 表記切替 4 種・既定（ja = 日本語技名、他 = SRK） | 済 | `notation.ts`、E2E「5 言語」 |
| 5 言語・zh-Hant は OpenCC 生成 | 済 | `locales/`、`i18n.test.ts`（キー集合・差し込み名・かなの混入）、`scripts/zh-hant.test.mjs`。コード中で使うキーが ja に全部あることを別途 grep で確かめた（欠け 0） |
| Worker・中断 | 済（ただし §3 高-1） | `engineClient.ts`・`setplay.worker.ts`、E2E「中断」 |
| URL 共有 | 一部 | 読み書き・自動計算は済。範囲の前後関係の検査が無い（§3 中-2） |
| コピー（D16）・`clipboard-write=(self)` の実ブラウザ確認 | 済（実行）／証拠未 | E2E「コピー」は緑。docs は「壊すと『コピーできませんでした』になることを確かめた」とあるが、報告に記録が無い |
| ストレージ不使用（§7.3） | 済 | E2E「ストレージ・cookie を使わない」 |
| CI の Web job（生成・tsc・vitest・ビルド・配信上限・E2E・summary） | 済（未実行） | `ci.yml` の差分。`pipefail`・ANSI 除去・「読めなかった」の既定あり。CI 上での実行結果は見ていない |
| 完了報告（AGENTS §3・§8-1・§8-11） | **未** | `S5-report.md` に結果・逐語・`git diff --stat`・判断・残課題が無い（§3 高-2） |
| 計算方式 C ＋ 256 MiB で Worker を捨てる（開発者決定） | 済 | `engineHost.ts` は表を読まない、`prepare-assets.sh` は `-tables=false`、`engineClient.ts:99` |

## 3. 指摘

### 高

**高-1. WASM が一度終了すると、以後その Worker の要求が返らず「計算中」のまま固まる（docs とも食い違う）**
- 場所: `web/src/worker/engineHost.ts:75`・`:86`、`web/src/worker/setplay.worker.ts:59-62`、`web/src/lib/engineClient.ts:73`・`:99`、`docs/frontend.md:32`
- 内容: `cmd/wasm` の `load`/`suggest` は `recover` していない。手元で確かめた（`GOOS=js GOARCH=wasm` で `js.FuncOf` の中で panic させ、Go 付属の `wasm_exec.js` から呼ぶ）:
  - 1 回目: panic を出して Go が exit code 2 で終わり、**JS 側の呼び出しは例外を投げずに `undefined` を返す**
  - 2 回目以降: **`Error: Go program has already exited` を投げる**
  - これを本実装に当てはめると、1 回目は `parseJson(undefined)` → null → `invalid_engine_response`（`worker_failed` ではない）。`this.api` は捨てないので、2 回目は `api.suggest` が投げて `handle` が reject → `queue` が reject のまま → **返事を送らない**。さらに `queue = queue.then(...)` は reject 済みの鎖に繋ぐので、**以後のすべての要求も実行されない**。`onerror` も発火しない（未処理の reject は Worker の `error` イベントにならない。推測: Chromium の挙動。少なくとも本実装は `unhandledrejection` を拾っていない）。画面は「計算中」のままで、利用者が「中断」を押すまで戻らない。
  - メモリ不足（Go の `fatal error: out of memory`）も同じ「Go が exit する」経路。この場合は多くが 256 MiB を超えているので `engineClient.ts:99` で偶然 Worker が捨てられ固まらないが、出る誤りは `invalid_engine_response` で、docs の「Worker が落ちたら（メモリ不足など）`worker_failed`」と合わない。256 MiB 未満で起きる panic（エンジンの不具合・壊れたデータ）では固まる。
- なぜ問題か: 誤った状態（無限の計算中）を出すバグ。docs がコードと食い違う。単体試験（`engineHost.test.ts`・`engineClient.test.ts`）は「API が例外を投げる」「undefined を返す」を試していないので緑のまま。
- 直し方の案:
  1. `engineHost.handle` で `api.load` / `api.suggest` を try/catch し、例外または JSON でない応答のときは `this.api = null; this.loaded.clear()` にして誤りを返す（次の要求で起動し直す）。
  2. または Worker 側で「Go が終わった」（`go.exited` か `go.run(...)` の Promise の解決）を検知したら、誤りの返事に「Worker を捨てよ」の印を付け、`EngineClient.onReply` で `dropWorker()` する（`worker_failed` として出すならここで code を揃える）。
  3. `setplay.worker.ts` の鎖は `queue = queue.then(run, run)` か `.catch` で切らないようにし、`handle` が投げても `{type:"error", id}` を返す。
  4. 試験: `engineHost.test.ts` に「1 回目 undefined・2 回目 throw」の偽 API、`engineClient.test.ts` に「誤りの返事の後も次の要求が返る」を足し、上の修正を外すと赤になることを確かめる（§8-3）。docs の文言も実装に合わせる。

**高-2. 完了報告に結果・証拠が無い。docs が報告の無い節を参照している**
- 場所: `docs/progress/S5-report.md`（全体。`:61` が「下の「計測」」を参照するが節が無い）、`docs/precompute.md:13`（「KA 100 の最大: 表あり 7.9 秒・表なし 6.9 秒。`docs/progress/S5-report.md`」）、`docs/frontend.md`「計測（参考）」・「コピー」節（「E2E の壊し方で…確かめた」）
- 内容: 報告に「やったこと」「判断と理由」「残課題」「次の段階への入力」、tsc/vitest/E2E/go test の逐語（`EXIT=`・件数）、`git diff --stat 5f1f7ffa`、§8-3（新しい検査をわざと壊して赤）・§8-5（配線を 1 行消すと E2E が赤）の記録が無い。数値（7.9 秒・6.9 秒・833 MiB・97 MiB）は `docs/progress/S5-measurements/*.json` から再集計すると一致した（下 §5）が、報告本文からは辿れない。
- なぜ問題か: AGENTS §3・§8-1・§8-2・§8-11。docs が存在しない記述を根拠に挙げている。
- 直し方の案: 報告を完成させる。最低限: 計測の表（KA ごとの最大所要・最大メモリ、計測した条件とキャラの範囲＝メモリの KA 20〜60 は 5 キャラ・meaty/gap だけ、KA 80/100 は 31 キャラ・meaty/gap だけ、filler は未計測）、検査の逐語、`git diff --stat`、壊して赤の記録（どの行を消し／変えたか、どの試験が赤になったか）、Permissions-Policy を拒否にしたときのコピー失敗の記録、表ありの計測を今の `browser-timing.mjs` では再現できないこと（`engineHost` が表を読まなくなったため。下 低-4）。

### 中

**中-1. 0 件のときの「絞り込みを外すと…」を、エンジンに送っていない絞り込みでも出す**
- 場所: `web/src/components/Results.tsx:77`
- 内容: `filtered = c.approachOnly || c.blockSafeOnly || c.chainCancelOnly`。しかし `toEngineQuery` は安全飛びでは `approach_only` を送らず、`block_safe_only` は meaty でしか送らない。本家は `buildParams` で `blockSafeOnly && mode==="meaty"`、安全飛びは `approachOnly: false` にして `appliedFiltered` を判定している（`_reference/combomgr/web/src/features/setplay/components/SetplaySuggestionSection.tsx` の `buildParams`・`runSafeJump`・`appliedFiltered`）。
- なぜ問題か: 汚連携・有利調整・安全飛びで、効いていない絞り込みを外すよう促す誤った案内が出る（本家と食い違う挙動）。
- 直し方の案: 「実際に送った絞り込み」で判定する（例: `toEngineQuery(c)` に `approach_only`/`block_safe_only`/`chain_cancel_only` が含まれるか、または同じ条件式を共有する関数を `conditions.ts` に置く）。`App.test.tsx` に gap + block_safe ON + 0 件で `setplay.empty` が出る試験を足す。

**中-2. 共有 URL は範囲の前後関係を検査せず、画面では押せない条件で自動計算する**
- 場所: `web/src/lib/conditions.ts:196-230`（`parseSearch`）、`web/src/components/App.tsx:128-145`
- 内容: `n_max < n_min`（n_max>0）や `g_max < g_min` は `validateForm` では押せない（本家 M47-04 追補④）が、URL からは通って自動計算され、必ず 0 件になる。`sj` が mode≠filler のとき黙って捨て、`rejected` にも挙げない。`n_min` などの上限が無く、`9999999999999999999999` のような値は `Number` で丸まった値（1e22 等）として通り、エンジンの誤りになる（推測: 文字列化で `1e+22` になりエンジンが整数として読めない）。
- なぜ問題か: 「同じ条件を画面で押すと押せない」のに URL だと計算される不整合。`rejected` の表示（「読めなかった」）とも合わない。
- 直し方の案: `parseSearch` の後で `validateForm(formFromConditions(c))` を通し、誤りがあれば自動計算しない（欄には入れて誤りを出す）か、該当キーを `rejected` に入れて既定値に戻す。数値は `Number.isSafeInteger` で上限を切る。試験を `conditions.test.ts` に足す。

**中-3. 単体試験が Worker 内の例外経路と URL の前後関係を押さえていない（上 2 件の再発防止）**
- 場所: `web/src/worker/engineHost.test.ts`、`web/src/lib/engineClient.test.ts`、`web/src/lib/conditions.test.ts`
- 内容・直し方: 高-1・中-2 の直し方 4 に同じ。

### 低

**低-1. `_headers` に配らない `sp/` の規則が残っている**
- 場所: `web/public/_headers:14-15`、`web/scripts/scripts.test.mjs:26-28,40,59`
- 内容: 計算方式 C で `sp/` は配らない。害は無いが、読み手が「表を配る」と誤解する。試験も `sp/` を例に使っている。
- 直し方: 規則を消し、試験の例を `moves/` に替える（または「D を足すときのために残す」とコメントする）。

**低-2. `headersFor` の「同じ名前は後の規則が勝つ」は Cloudflare の挙動と違う**
- 場所: `web/scripts/headers.mjs:28`
- 内容: Cloudflare Pages の `_headers` は、複数の規則に当たった同名ヘッダを**カンマで連結する**（推測: 公式 docs の記述に基づく記憶。確認していない）。いまの `_headers` に同名の重複は無いので実害なし。将来 `Cache-Control` を重ねると E2E と本番がずれる。
- 直し方: 連結に合わせるか、同名の重複を検出して誤りにする。

**低-3. `-tables=true` でも `units`/`rows` が 0 のときに manifest から消える（§8-7）**
- 場所: `cmd/precompute/main.go:73-76`
- 内容: `omitempty` は「表を作らない」と「表はあるが 0」を同じ「キーが無い」に寄せる。現データで 0 になるキャラがあるかは未確認（推測: 無い）。
- 直し方: ポインタ型にする、または `tables=false` 用に別の型を出す。

**低-4. 計測スクリプトで「表あり」の計測を再現できない**
- 場所: `web/scripts/measure/browser-timing.mjs`、`web/src/worker/engineHost.ts:42`
- 内容: `docs/precompute.md:13` の「表あり 7.9 秒」は `8b5f740` 時点の Worker（表を読む）で取った値で、今のコードでは再現できない。報告にその旨と、取ったコミットを書くとよい。

**低-5. 結果が 3000 件を超えるとき、3000 件まで出した後に理由の表示が無い**
- 場所: `web/src/components/Results.tsx:105-127`
- 内容: 例 `Showing 200 of 51073`（`S5-measurements/mem-all.json` の ryu KA 80）。「さらに表示」は 3000 件で止まり、残りが出せない理由が出ない。本家も同様（`canShowMore` は `SETPLAY_MAX_LIMIT` で止まるだけ）なので本家との差ではない。
- 直し方: `visible.length === items.length && items.length < totalFound` のとき「先頭 3000 件までを表示します」を出す（全ロケールに同時に足す）。

**低-6. `block_safe=1` が meaty 以外でも URL に残る**
- 場所: `web/src/lib/conditions.ts` `conditionsToSearch`
- 内容: 計算には効かない（`toEngineQuery` が落とす）が、共有 URL に効いていない条件が載る。中-1 と同じ判定関数で落とせる。

**低-7. 「モードと絞り込み」の E2E の最後の確認が弱い**
- 場所: `web/e2e/setplay.spec.ts:59-62`（`result-row` または `empty` が見えればよい）
- 内容: block_safe の絞り込みが効いたか（件数が減ったか・行がガード時 0F 以上か）を見ていない。正しさは `conformance.spec.ts` の `approach=1&block_safe=1` で sha256 一致を見ているので配線は担保されている。主要フローとしては許容。

## 4. 良かった点

- `conformance.spec.ts` が、画面に出た行の正準形（`data-canonical`）を本家の正解（`engine/testdata/upstream_oracle.json`）と sha256・順序・件数・`truncated` まで突き合わせており、Go → WASM → Worker → 画面の通しを数字で押さえている（§8-5）。「先頭 200 行 = 本家 limit 200」で「並べ替えの後で切る」も見ている。
- `toEngineQuery` が本家の `setplayApi.getSuggestions` と同じ規則（n_max・g_*・remain・block_safe_only の送り分け、安全飛びで approach_only を掛けない）になっている。種別 0 個で押せなくした変更は理由付きで docs に書いてある。
- E2E が `_headers` どおりのヘッダ（CSP・Permissions-Policy）で配った dist に対して回り、console error・pageerror を失敗にしている。`retries: 0`・直列。
- 結果の打ち切り表示（`truncated`）を「limit で切った」と区別している。
- `_reference/`・`engine/`・`internal/` に差分なし（`git diff --stat 5f1f7ffa HEAD -- _reference engine internal` が空）。`web/public/data`・`web/public/wasm`・`node_modules` はコミットされていない。
- 計測値（7.9 秒・6.9 秒・833 MiB・97 MiB）は `S5-measurements/*.json` から再集計して一致した。

### 指摘なしの観点
- 禁止事項（AGENTS §7）: 秘密情報・公式データ・combomgr を取込元にする記述を、差分の新規ファイル一覧と `_headers`・`constants.ts` の URL（取込元は `plexiblinp/tacpendium`）で確認。指摘なし。
- `dangerouslySetInnerHTML`・ストレージ: `web/src` を grep（`dangerouslySetInnerHTML|localStorage|sessionStorage|indexedDB|document.cookie`）、`web/src`・`web/e2e`・`web/scripts` で、該当は `src/lib/i18n.ts:59` のコメント 1 件と E2E の検査コードだけ。E2E でも実行時に 0 を確認。
- テストの減少: 差分は追加のみ。`cmd/precompute/main_test.go` は引数の追加と試験 1 本の追加で、既存の試験は消えていない。

## 5. 自分で回したコマンドと結果

出力はスクラッチのファイルへ書き、`EXIT=` で判定した。

```
$ cd web && pnpm exec tsc --noEmit
EXIT=0

$ cd web && pnpm exec vitest --run
 Test Files  10 passed (10)
      Tests  83 passed (83)
EXIT=0

$ go test -count=1 ./cmd/precompute/
ok  	github.com/plexiblinp/tacpendium-setplay/cmd/precompute	17.077s
EXIT=0

$ cd web && bash scripts/prepare-assets.sh && pnpm build && PW_CHROMIUM_PATH=/opt/pw-browsers/chromium-1194/chrome-linux/chrome pnpm exec playwright test
（prepare）違反: 0 件 / setplay.wasm: 3947021 bytes
（build）ファイル: 39(上限 20000) / 最大: wasm/setplay.wasm 3947021 bytes / 違反: 0 件
Running 16 tests using 1 worker
  ✓ conformance 9 本（ryu KA 43 ×7、ken KA 43、zangief KA 79）
  ✓ setplay.spec 7 本
  16 passed (28.4s)
EXIT=0
```

高-1 の裏付け（スクラッチで、リポジトリ外）: `js.FuncOf` の中で nil map に代入して panic させた最小の Go WASM を、Go 1.24.7 付属の `wasm_exec.js` で node から 2 回呼んだ。

```
panic: assignment to entry in nil map ... exit code: 2
returned undefined
2nd THREW: Go program has already exited
```

計測の再集計（`docs/progress/S5-measurements/`）: timing-table KA 100 最大 7903 ms、timing-notable KA 100 最大 6904 ms、mem-all（31 キャラ × meaty/gap）KA 100 最大 833 MiB・KA 80 最大 537 MiB（62 中 35 が 256 MiB 以下）、mem-low（cammy・dhalsim・ingrid・marisa・ryu × meaty/gap）KA 60 最大 97 MiB。filler はメモリ未計測。

## 取り込み（1 往復目）

| 指摘 | 採否 | 理由・対応 |
|---|---|---|
| 高-1 WASM の panic 後に Worker が固まる | 採用 | `EngineHost` が WASM の呼び出しを try/catch し、文字列でない応答・例外を `wasm_crashed`（`fatal: true`）で返す。起動と読み込みの記録を捨てる。Worker の鎖は reject を `.catch` して返事をし（`worker_failed`・fatal）、切らない。`EngineClient` は fatal の返事で Worker を捨てて作り直す。試験 3 本を追加（落ちた後も同じ api を使う・undefined を素通しする・fatal で捨てない、の 3 通りで壊して赤を確認）。docs/frontend.md を直した |
| 高-2 完了報告が未完 | 採用 | 完了報告を書く（この取り込みの後） |
| 中-1 送っていない絞り込みで「絞り込みを外すと」 | 採用 | `hasAppliedFilters`（`toEngineQuery` と同じ規則）で判定。`toEngineQuery` と全組み合わせで食い違わない試験、画面の試験（汚連携＋ガード時 0F 以上）を追加。古い判定に戻すと画面の試験が赤 |
| 中-2 共有 URL の検査の抜け | 採用 | 最大 < 最小は最大（gap は両方）を既定に戻して `rejected` に挙げる。有利調整でない `sj`・重ねるでない `block_safe` は捨てて挙げる。`n_*` は 999、`remain` は 100 を上限にした |
| 中-3 上の試験が無い | 採用 | 上の 2 件に試験を追加 |
| 低-1 `_headers` に `/data/sp/*` が残る | 採用 | 消した（試験の例も moves に） |
| 低-2 同じ名前のヘッダは Cloudflare では連結 | 採用 | `headersFor` を ", " 連結に変え、試験を追加。推測（Cloudflare の説明による）で、実配信での確認は S7 のヘッダ診断 |
| 低-3 `omitempty` で units/rows 0 と表なしが区別できない | 不採用 | 表を作るとき units・rows は 1 以上（KA 0 でも単位 0 個の行がある）。0 で表があるという状態は起きないので、区別の必要が無い。報告に記録 |
| 低-4 「表あり 7.9 秒」が今のコードで再現できない | 採用（記録） | 完了報告に「表ありの値は 8b5f740 の Worker（表を読む）で取った」と書く |
| 低-5 3000 件を超えた分を出せない理由が出ない | 不採用 | 本家と同じ挙動（D1）。残課題に記録 |
| 低-6 `block_safe` が重ねる以外でも URL に残る | 採用 | 重ねるのときだけ書く。試験を追加 |
| 低-7 E2E の絞り込みの確認が弱い | 不採用 | 絞り込みの正しさは E2E の本家突き合わせ（approach＋block_safe・chain の 2 条件）が担保している |

取り込み後: vitest 94 件 pass（EXIT=0）、E2E 16 passed（EXIT=0）。

## 再レビュー（2 往復目）

- 対象: `git diff ec973d7~1 HEAD`（`ec973d7` 取り込み・`3c4bc1e` 完了報告。HEAD = `3c4bc1e`）。基準は `5f1f7ffa`（全体 74 files, +10493 / −33。`_reference`・`engine`・`internal` の差分は空）。
- 範囲: 1 往復目の指摘が直ったか、直したことで出た新しい問題、完了報告と事実の食い違い。レビュアーは実装の文脈を持たない。書いたのはこの節だけ。

### 要約

- 高-1（WASM が落ちた後に固まる）・中-1（送っていない絞り込みで「絞り込みを外すと」）・低-1・低-2・低-6 は**直っている**。直した箇所を 1 つずつわざと元に戻し、対応する試験が赤になることを確かめた（下「わざと壊した結果」）。
- 中-2（共有 URL の検査）は直ったが、**URL 側だけに上限・前後関係の検査を足したため、画面で押せる条件を共有すると、開いた側で「読めなかった」になり別の条件で計算される**ようになった（新しい問題・中）。
- 高-2（完了報告）は大部分が書かれたが、**報告が「末尾の『最終確認』」「全体の diff は末尾」を参照しているのに、その節が無い**。`EXIT=` の逐語・`git diff --stat` も無い（AGENTS §8-1・§8-11。高のまま）。
- 手元で tsc・vitest（94 件）・E2E（16 本）を回して緑。HEAD の CI（run 38165453253）も 6 job success。

### 1 往復目の指摘の確認

| 指摘 | 判定 | 根拠 |
|---|---|---|
| 高-1 WASM の panic 後に固まる | 済 | `engineHost.ts:62-69` が例外・文字列でない応答を `wasm_crashed`（`fatal`）にして `api`・`loaded` を捨てる。`engineClient.ts:101-104` が fatal で Worker を捨てる。`setplay.worker.ts:62-71` の鎖は `.catch` で切れない。壊すと赤（下）。ただし docs の分類に食い違いが残る（新-中-2） |
| 高-2 完了報告が未完 | **一部** | やったこと・計測・判断・残課題・次の段階への入力・§8-3 の表はある。参照先の節が無い・逐語が無い（新-高-1） |
| 中-1 送っていない絞り込み | 済 | `conditions.ts:115-117` `hasAppliedFilters`、`Results.tsx:77`。`toEngineQuery` と全組み合わせで照合する試験あり。古い判定に戻すと `App.test.tsx` の新しい試験が赤 |
| 中-2 共有 URL の検査 | 済（ただし新-中-1） | `conditions.ts:151-155, 230-254`。各分岐を外すと `conditions.test.ts` が赤 |
| 中-3 試験の不足 | 済 | 上 2 件の試験が効くことを確かめた |
| 低-1 `_headers` の `sp/` | 一部 | `_headers` と headers の試験は直った。取り込み節は「試験の例も moves に」とあるが、`web/scripts/scripts.test.mjs:24-28`（`listFiles` の試験）は `data/sp/` のまま（新-低-1） |
| 低-2 同名ヘッダの連結 | 済 | `headers.mjs:30-39`。元に戻すと「同じ名前のヘッダは連結する」が赤。Cloudflare の挙動であることは推測のまま（報告の残課題に記録あり） |
| 低-3 不採用 | 了解 | 理由が報告の残課題にある |
| 低-4 表ありの再現 | 済 | 報告の計測節に「`8b5f740` で取った」とある |
| 低-5 不採用 | 了解 | 残課題に記録あり |
| 低-6 `block_safe` が URL に残る | 済 | `conditions.ts:188-189`。元に戻すと赤 |
| 低-7 不採用 | 了解 | 理由は妥当（本家突き合わせが担保） |

### 指摘

#### 高

**新-高-1. 完了報告が存在しない節を参照し、検査の逐語（`EXIT=`・件数）と `git diff --stat` が無い**
- 場所: `docs/progress/S5-report.md:71`（「全体の diff は末尾」）、`:102`（「最終の値は末尾の「最終確認」」）。報告の末尾は「次の段階への入力」で終わり、どちらの節も無い（`grep -n "最終確認\|diff --stat\|EXIT=" docs/progress/S5-report.md` の該当は `:102` の見出し 1 件だけ）。
- 内容: 「検査の結果」は「vitest: 10 ファイル・94 件 pass」「E2E: 16 本 pass」と要約だけで、回したコマンド・`EXIT=`・出力の逐語が無い。`git diff --stat 5f1f7ffa`（テストを含む）も無い。CI は `1346893`（取り込み前）の run 38164794212 を挙げており、取り込み後の HEAD の CI（run 38165453253、6 job success。手元で `gh run view` で確認）が書かれていない。
- なぜ問題か: AGENTS §8-1（緑の証拠の逐語）・§8-11（`git diff --stat` を貼り、報告を書いた後に検査を回し直して一致を確かめる）。1 往復目の高-2 は「採用」とされたが、この部分が満たされていない。数値自体は手元の結果（vitest 94・E2E 16）と一致した。
- 直し方の案: 報告の末尾に「最終確認」節を足し、HEAD で回した `tsc`・`vitest --run`・`playwright test`・`go test -count=1 ./cmd/precompute/` のコマンドと `EXIT=`・件数の逐語、`git diff --stat 5f1f7ffa HEAD`、HEAD の CI run を貼る。「全体の diff は末尾」の参照先も同じ節にする。

#### 中

**新-中-1. 画面で押せる条件を共有すると、開いた側で「読めなかった」になり、別の条件で自動計算される（中-2 の直しで生じた）**
- 場所: `web/src/lib/conditions.ts:151-155`（URL だけの上限 `URL_FRAME_MAX`=999・`remain` ≤ `KA_MAX`）、`:244-253`（前後関係の検査がモードを見ない）。対する画面側 `web/src/lib/formState.ts:94-99`（`n_min`・`n_max`・`remain` に上限なし）、`:101-110`（前後関係は今のモードの欄だけ検査）。
- 内容: 画面の `validateForm` → `conditionsToSearch` → `parseSearch` の往復を手元で回した（スクラッチの試験。リポジトリは変更していない）:
  ```
  meaty_nmin1000           q=c=ryu&ka=40&n_min=1000                 rejected=["n_min"]          same=false
  filler_remain150         q=c=ryu&ka=40&mode=filler&remain=150     rejected=["remain"]         same=false
  meaty_hidden_g_reversed  q=c=ryu&ka=40&g_min=5&g_max=3            rejected=["g_min","g_max"]  same=false
  gap_hidden_n_reversed    q=c=ryu&ka=40&mode=gap&n_min=5&n_max=3   rejected=["n_max"]          same=false
  ```
  - 上 2 つは画面で押せる（`errors: []`）。送り手は自分の条件で計算したのに、受け手は既定値に戻した**別の条件で自動計算**する（`remain=150` は送り手では理由つきの 0 件、受け手は既定 remain の結果）。
  - 下 2 つは、使っていないモードの欄に残った値（汚連携で G を 5〜3 にしてから重ねるに切り替える等）。計算は同じだが、受け手に誤った「読めなかった」が出る。
- なぜ問題か: 1 往復目の中-2 は「画面と URL の不整合」を直す指摘だったが、URL 側を画面より厳しくしたため、逆向きの不整合（画面 → URL → 画面で条件が変わる）が生じた。docs/frontend.md:56 の「画面の欄で押せない組…も同じ扱い」とも合わない（押せる組まで弾いている）。既存の往復試験（`conditions.test.ts:74`）は既定に近い条件しか通さないので気づけない。
- 直し方の案: 上限と前後関係の規則を 1 か所（`conditions.ts` か `constants.ts`）に置き、`validateForm` と `parseSearch` の両方で使う（例: 画面の `n_*` にも 999、`remain` にも上限を掛ける）。前後関係は今のモードの欄だけ検査する（`parseSearch` は `c.mode === "meaty"` のときだけ n、`"gap"` のときだけ g）か、`conditionsToSearch` で今のモードに効かない欄を書かない。試験として「`validateForm` が通した条件は `parseSearch(conditionsToSearch(c))` で `rejected: []`・同じ条件に戻る」を、上の 4 例を含めて足す。

**新-中-2. docs とコメントの「メモリ不足 → `worker_failed`」は今のコードの分類と合わない**
- 場所: `docs/frontend.md:32`（「Worker 自体が落ちたら（メモリ不足など）`worker_failed`」）、`web/src/lib/engineClient.ts:74`（`onerror` の注記「WASM が落ちた(メモリ不足など)」）
- 内容: 1 往復目の高-1 で確かめたとおり、Go の WASM のメモリ不足（`memory.grow` の失敗 → Go の `fatal error: out of memory` → exit）は panic と同じ「Go が終わる」経路で、今のコードでは `engineHost` の `call` が拾って `wasm_crashed` になる。`onerror`（`worker_failed`）が拾うのは Worker のスクリプトでの未処理の例外（`wasm_exec.js` の読み込み失敗など）。推測: ブラウザのプロセスごとのメモリ上限で落ちる場合はタブ全体が落ち、`onerror` も届かない。
- なぜ問題か: コードと食い違う docs・コメント（review.md の基準では高に当たるが、OOM の実挙動を実ブラウザで確かめていないので中とした）。S6 で誤り code を計測に使うと、メモリ不足の件数を `worker_failed` で数える誤りになる。
- 直し方の案: docs を「メモリ不足（Go が終わる）も `wasm_crashed`。`worker_failed` は Worker のスクリプトの例外」に直し、`engineClient.ts:74` の注記も合わせる。可能なら、KA 100 の重い条件を小さい `WebAssembly.Memory` の上限で動かして、実際の code を確かめる。

#### 低

**新-低-1. 取り込み節の「試験の例も moves に」が一部だけ**
- 場所: `web/scripts/scripts.test.mjs:24-28`（`listFiles` の試験が `data/sp/a.bin` を使う）
- 内容: 害は無い（汎用の数え上げの試験）が、取り込み節の記述と事実が一部違う。1 往復目の低-1 はこの行も挙げていた。
- 直し方: 例を `data/moves/` に替えるか、取り込み節を「headers の試験だけ」に直す。

**新-低-2. Worker の鎖の `.catch` に試験が無く、誤り code が文字列直書き**
- 場所: `web/src/worker/setplay.worker.ts:66-71`
- 内容: `handle` が reject したときの返事（`worker_failed`・fatal）は、どの試験からも通らない（`setplay.worker.ts` は単体試験の対象外。今の `handle` は WASM の例外を自分で拾うので、実際に通る経路も見当たらない）。`"worker_failed"` は `engineClient.ts:12` の `CLIENT_ERROR_WORKER` と同じ値の直書き（AGENTS §4 定数化）。
- 直し方: `CLIENT_ERROR_WORKER`（または protocol 側の定数）を使う。鎖の処理を `engineHost.ts` 側の小さな関数に出して「reject しても返事をし、次の要求も処理する」試験を足すか、報告に「防御のみ・試験なし」と書く。

**新-低-3. WASM が落ちる経路の通しの検査が無い（§8-5）**
- 場所: `web/src/worker/engineHost.test.ts`・`web/src/lib/engineClient.test.ts`（偽の API・偽の Worker だけ）
- 内容: `EngineHost`（fatal を返す）と `EngineClient`（fatal で捨てる）はそれぞれ単体で押さえたが、実 WASM → Worker → 画面で「落ちた後に次の計算が返る」は見ていない。単体の照合（`fatal` フラグの名前・形）は型で揃っているので、配線が切れる危険は小さい。
- 直し方（任意）: 試験用に panic させる入力を `cmd/wasm` が持たないなら、E2E で Worker 側の `globalThis.tacpendiumSetplay.suggest` を差し替えて例外を投げさせ、誤りの表示と次の計算の成功を見る。

**新-低-4. 報告の CI の記録が取り込み前のコミット**
- 場所: `docs/progress/S5-report.md:107`
- 内容: run 38164794212 は `1346893`（取り込み前）。取り込み後の `ec973d7` の run 38165373391 は cancelled（次の push で打ち切り）、HEAD `3c4bc1e` の run 38165453253 は 6 job success。
- 直し方: 新-高-1 の「最終確認」に HEAD の run を書く。

### 良かった点

- 高-1 の直しは、1 往復目の直し方の案 1・2・3 をすべて入れており、`call` で「例外」と「文字列でない応答」を同じ fatal に寄せた上で、起動と読み込みの記録を捨てて次の要求で起動し直す（試験で `boot`・`load` の呼び出し回数まで見ている）。
- `hasAppliedFilters` の試験が `toEngineQuery` と全組み合わせで照合する形なので、今後どちらかの規則だけを変えると赤になる。
- 直した箇所の試験は、どれも元に戻すと 1 本だけ狙いどおり赤になる（下）。

### わざと壊した結果（スクラッチのコピーで。リポジトリは変更していない）

`web/` を node_modules 抜きでスクラッチへ写し、1 か所ずつ元の形に戻して `pnpm exec vitest --run` を回した。戻さない状態は 94 passed / EXIT=0。

| 壊し方 | EXIT | 赤になった試験 |
|---|---|---|
| `engineClient` の `if (r.fatal) this.dropWorker();` を消す | 1 | engineClient「fatal の誤り…Worker を捨て」 |
| `Results` の判定を旧式（3 フラグの OR）に戻す | 1 | App「0 件のとき、エンジンに送っていない絞り込み…」 |
| `parseSearch` の n の前後関係の検査を外す | 1 | conditions「最大 < 最小は…」 |
| `parseSearch` の g の前後関係の検査を外す | 1 | 同上 |
| `parseSearch` の `sj` の `rejected` を外す | 1 | conditions「有利調整でない sj…」 |
| `parseSearch` の `block_safe` を重ねる以外でも受ける | 1 | 同上 |
| `remain` の上限を外す | 1 | conditions「巨大な整数は受けない」 |
| `conditionsToSearch` で `block_safe` をモードによらず書く | 1 | conditions「重ねるでないときは block_safe を URL に書かない」 |
| `headersFor` を「後が勝つ」に戻す | 1 | scripts「同じ名前のヘッダは連結する」 |
| `crashed` で `api`・`loaded` を捨てない | 1 | engineHost「WASM が落ちたら…起動し直して読み直す」 |
| `call` で文字列でない応答を素通しする | 1 | 同上 |

（いずれも 1 failed | 93 passed (94)。戻した後に元の 94 passed を確認。）

### 自分で回したコマンドと結果

出力はスクラッチのファイルへ書き、`EXIT=` で判定した（`go test` は回していない。別のプロセスが回している可能性があるため。計測の主張も検証していない）。

```
$ cd web && pnpm exec tsc --noEmit
EXIT=0

$ cd web && pnpm exec vitest --run
 Test Files  10 passed (10)
      Tests  94 passed (94)
EXIT=0

$ cd web && bash scripts/prepare-assets.sh
違反: 0 件 / setplay.wasm: 3947021 bytes / gzip -9: 1088611 bytes
EXIT=0

$ cd web && pnpm build
最大: wasm/setplay.wasm 3947021 bytes(上限 26214400) / 合計: 6653368 bytes / 違反: 0 件
EXIT=0

$ cd web && PW_CHROMIUM_PATH=/opt/pw-browsers/chromium-1194/chrome-linux/chrome pnpm exec playwright test
Running 16 tests using 1 worker
  16 passed (39.6s)
EXIT=0

$ gh run view 38165453253 --json jobs   # HEAD 3c4bc1e の CI
リポジトリ検査 success / 事前計算 success / 本家の正解の再生成 success / Web success / WASM success / Go success

$ git diff --stat 5f1f7ffa HEAD | tail -1
 74 files changed, 10493 insertions(+), 33 deletions(-)
$ git diff --stat 5f1f7ffa HEAD -- _reference engine internal
（空）
```

### 一覧

| 重要度 | 指摘 | 場所 |
|---|---|---|
| 高 | 新-高-1 報告が無い節（最終確認・末尾の diff）を参照、`EXIT=` 逐語・`git diff --stat`・HEAD の CI が無い | `docs/progress/S5-report.md:71,102,107` |
| 中 | 新-中-1 画面で押せる条件を共有すると受け手で「読めなかった」・別条件で計算（URL だけ上限、前後関係がモードを見ない） | `web/src/lib/conditions.ts:151-155,244-253`、`formState.ts:94-110` |
| 中 | 新-中-2 「メモリ不足 → worker_failed」の docs・注記が今の分類（wasm_crashed）と合わない | `docs/frontend.md:32`、`web/src/lib/engineClient.ts:74` |
| 低 | 新-低-1 `listFiles` の試験が `data/sp/` のまま（取り込み節と一部違う） | `web/scripts/scripts.test.mjs:24-28` |
| 低 | 新-低-2 Worker の `.catch` に試験なし・`"worker_failed"` 直書き | `web/src/worker/setplay.worker.ts:66-71` |
| 低 | 新-低-3 WASM が落ちる経路の通しの検査なし | `engineHost.test.ts`・`engineClient.test.ts` |
| 低 | 新-低-4 報告の CI が取り込み前のコミット | `docs/progress/S5-report.md:107` |

## 取り込み（2 往復目）

| 指摘 | 採否 | 理由・対応 |
|---|---|---|
| 新-高-1 完了報告に「最終確認」・`git diff --stat`・HEAD の CI が無い | 採用 | 完了報告の末尾に「最終確認」節を足す（HEAD で回し直した逐語・`git diff --stat`・HEAD の CI） |
| 新-中-1 画面で押せる条件が共有すると読めない | 採用 | URL の数の規則を画面の欄と同じに（0 以上の整数・安全な整数まで。999・100 の上限は撤回）。前後関係の検査は今のモードの欄だけ。種別 0 個は特定の技・有利調整のときだけ受ける（往復の試験で見つかった）。「画面で押せる条件は共有しても同じ条件」の往復試験（8 例）を追加。g の検査をモードによらず行うと往復試験が赤、上限を 999 に戻すと赤になることを確認 |
| 新-中-2 メモリ不足は `worker_failed` でなく `wasm_crashed` の経路 | 採用 | docs/frontend.md とコメントを「Go のメモリ不足は wasm_crashed の経路になる見込み（推測。実ブラウザの OOM は未確認）」に直した |
| 新-低-1 試験の例が `data/sp/` のまま | 採用 | `data/moves/` に変えた |
| 新-低-2 `.catch` の経路の試験が無い・`"worker_failed"` の直書き | 一部採用 | 定数 `CLIENT_ERROR_WORKER` を使うようにした。`.catch` の経路は Worker の最上位にあり単体で通せない（`handle` は reject しない作り）ので、試験は足さず残課題に記録 |
| 新-低-3 WASM が落ちる経路の実 WASM の通し検査が無い | 不採用 | 実 WASM を落とすには `cmd/wasm` に試験用の入口を足す必要があり、範囲を広げる。単体試験（undefined・例外）で経路を押さえた。残課題に記録 |
| 新-低-4 報告の CI が取り込み前 | 採用 | 新-高-1 と同じ |

再レビューは 2 往復で打ち切る（/stage の上限）。
