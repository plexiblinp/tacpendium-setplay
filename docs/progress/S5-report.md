# S5 フロント — 完了報告

- 段階開始コミット: `5f1f7ffa9b3b4cbe687ed36dbeec9616a7a0e2b1`
- ブランチ: `claude/pensive-brown-2tbyfb`

## 計画

### Context

HANDOVER §9 S5「フロント（画面・絞り込み・表記切替・5 言語・Worker・URL 共有・コピー(D16)）」。**完了条件: 主要フローの E2E が緑**。

前提（読んだもの）: HANDOVER §0・§1・§2.2・§4.4・§5・§6・§7.2〜§7.4・§9、S4 報告（とくに「次の段階への入力」と「S4 後の開発者決定」）、`_reference/combomgr/web/`（本家の画面・文言）。

- S4 後の開発者決定 1: **計算方式は S5 でブラウザ（WASM）の実際の所要を測ってから決める**（C: 表をやめ WASM のエンジンだけ／D: よく使う条件だけ最終結果を前もって作る）。HANDOVER §12 の「案 A/B の選択」の続きなので、**測った結果と推奨を添えて開発者に聞く**（AGENTS §3 止めるもの 2）。それまでは今の形（moves + 繋ぎ表を WASM に読ませる。結果は表なしと同じ）で作り、表の読み込みを設定 1 つで外せるようにしておく。
- D17（グッド/バッド）は S6。S5 では作らない（行のボタン置き場だけ考えておく）。
- デプロイ（Cloudflare）は公開に関わるので S5 ではしない。配信ヘッダは `_headers`（Cloudflare の形式）に書き、E2E のサーバが同じファイルを読んで同じヘッダで配る（§7.4「clipboard-write は S5 で実ブラウザで確かめる」をここで確かめる）。

### 作るもの

| 場所 | 中身 |
|---|---|
| `web/`（Vite + React + TypeScript、pnpm） | 画面一式。`package.json` の置き場所は CI が決めているとおり `web/package.json` |
| `web/src/lib/` の純粋関数 | 条件 ⇄ URL クエリ（zod で検証）、条件 → エンジンのクエリ文字列、表記の解決・レシピの文字列化（コピーの中身）、結果のメモリキャッシュ（LRU）、配信上限の数え上げ |
| `web/src/worker/` | Web Worker。WASM（`setplay.wasm` + Go 付属の `wasm_exec.js`）を起動し、manifest の `moves` / `table` を読んで `suggest` を呼ぶ。中断は Worker ごと `terminate` して作り直す |
| `web/src/locales/` | `ja` / `en` / `zh-Hans` / `ko` を手で書き、`zh-Hant` は OpenCC（`s2twp`）で `zh-Hans` から生成してコミット（再生成して一致を試験で見る。moves DB と同じ流儀） |
| `web/public/_headers` | §7.4 のヘッダ（CSP・nosniff・Referrer-Policy・Permissions-Policy（`clipboard-write=(self)` だけ許可））と、`/data/moves/*`・`/data/sp/*` の immutable キャッシュ |
| `web/scripts/` | サイトデータの用意（`go run ./cmd/precompute` と `scripts/wasm/build.sh` を `web/public/` の gitignore 下へ）、zh-Hant の生成、配信上限の検査（フロントのバンドル + wasm + data 全体。S4 レビュー低-3）、E2E 用の静的サーバ（`_headers` を読む） |
| `web/e2e/` | Playwright（`retries: 0`）。主要フロー |
| CI | Web job に生成・tsc・vitest・ビルド・配信上限・E2E を足す |
| docs | `docs/frontend.md`（新規）、`.github/dependabot.yml` に npm |

### 画面（本家 `SetplaySuggestionSection` の作り直し）

- キャラ選択 / KA 入力（0〜100 の整数）/ モード（重ねる・汚連携・有利調整）/ 当てる技の種別・ダメージ 0・特定の技 / N・G・調整後の有利 / 安全飛び 3 種 / 並び順 / 絞り込み 3 つ / 表記切替（4 種。既定は ja UI = 日本語技名、それ以外 = SRK）/ 言語切替（5 言語。`navigator.language` で自動選択）/ 結果一覧（件数・さらに表示・打ち切りの表示・理由）/ 各行のコピー / 制約の説明 / データ版の表示。
- 本家から外すもの: 採用・不採用・採用名・確認できた条件（ユーザーデータ依存。HANDOVER §2.2）。「採用」は「コピー」に置き換える。
- 計算中はボタンを無効化（連打。§7.2）、同じ条件はメモリキャッシュから即表示、「中断」で Worker を止める。
- 状態はメモリと URL クエリだけ（cookie・ストレージを使わない。§7.3）。URL に条件があれば読み込み時に検証して自動で計算する。

### 回す検査

- `pnpm exec tsc --noEmit`、`pnpm exec vitest --run`（純粋関数の単体・i18n のキー集合一致と非日本語ロケールへの日本語混入・zh-Hant の再生成一致・コンポーネントの主要な振る舞い）
- `pnpm exec playwright test`（`retries: 0`。Chromium。実 WASM・実データ・`_headers` どおりのヘッダで）
- Go 側に足すもの（あれば）: `go test`・`go vet`・gofmt。`watched` を触ったら `verify-plan.py ack`
- 配信上限（ファイル数 20,000・1 ファイル 25 MiB）を dist 全体で
- §8-3: 新しい検査ごとに、守りたい不具合を一度わざと起こして赤を確かめる
- §8-5: 配線の通し（Go → WASM → Worker → 画面）は E2E で、配線を 1 行消すと赤になることを確かめる

### 未決事項と扱い

| 事項 | 扱い |
|---|---|
| 計算方式 C / D（S4 後の開発者決定 1） | ブラウザで測って、推奨を添えて**聞く** |
| 「さらに表示」の作り | 自分で決める（推測: 本家の `limit` は「並べ替えの後で先頭を切る」だけなので、上限 3000 で 1 回計算して画面で 200 件ずつ出す。前置きの一致は試験で確かめる） |
| URL に言語・表記を入れるか | 自分で決める |
| zh-Hans・ko の文言 | 自分で書く（仮訳。画面に「機械翻訳です。改善提案歓迎」と Issue へのリンク） |
| コピーの中身 | 自分で決める（推測: 画面の行と同じ「レシピ（選択中の表記）＋ フレームの説明」） |
| デプロイ・公開 | しない（S7 以降・開発者） |

## 開発者決定（2026-10-11、S5 の途中）

**計算方式 C ＋ メモリ解放**（S4 後の開発者決定 1 の結論）。ブラウザでの実測（下の「計測」）を示して確認した。

- 繋ぎ表（`sp/`）は配らず、ブラウザ内の WASM のエンジンだけで計算する。表は速くしない（表あり・なしで同程度）。配信が約 26 MB・31 ファイル減る。
- D11「他ユーザーが実行済みの計算はしない」は、「クラウドで再計算しない」の意味で満たす（計算は利用者のブラウザの中。クラウドは静的ファイルを配るだけで、費用・無料枠に影響しない）。同じ条件でも別の利用者のブラウザでは計算し直す。
- 計算の後に WASM の線形メモリが 256 MiB を超えていたら、Worker を捨ててメモリを返す（Go の WASM は広げたメモリを返さないため）。計算中の最大（KA 100 で約 0.8 GiB）は残る。
- D（よく使う条件の事前計算）は、S6 の計測で重い KA（80 以上）がよく使われると分かったら足す。
- 確認の途中で開発者から「要求ごとに計算するのはホスト先的に駄目だったのでは」と質問があり、「計算は利用者のブラウザの中で、ホスト先は配るだけ」と説明した。「ユーザー環境のメモリへの影響は皆無か」には「皆無ではない（KA 100 で最大約 0.8 GiB、KA 60 以下は 0.1 GiB 未満）」と実測で答え、そのうえで上の形に決まった。

## やったこと

成果物（基準 `5f1f7ff` からの追加。全体の diff は末尾）:

- **フロント `web/`**（Vite 8 + React 19 + TypeScript 7 strict、pnpm 10）。画面: キャラ選択・KA 入力（0〜100）・モード（重ねる・汚連携・有利調整）・当てる技の種別・ダメージ 0・特定の技・N/G/調整後の有利・安全飛び 3 種・並び順・絞り込み 3 つ・表記切替 4 種・言語切替 5 種・結果一覧（件数・さらに表示・打ち切り・理由）・行ごとのコピー（D16）・制約の説明・データ版・フッター（ソース・データのクレジット・非公式）。仕様は `docs/frontend.md`。
- **Worker と WASM**: `web/src/worker/`（WASM の起動・moves の読み込み・`suggest`）と `web/src/lib/engineClient.ts`（1 度に 1 計算・中断は Worker ごと止める・古い返事を捨てる・WASM が落ちたら捨てる・計算後にメモリが 256 MiB を超えたら捨てる）。
- **URL 共有**: 条件 ⇄ URL クエリ（zod）。共有 URL を開くと自動で計算する。
- **5 言語**: ja・en・zh-Hans・ko を手で書き、zh-Hant は OpenCC（s2twp）で生成してコミット。
- **配信ヘッダ** `web/public/_headers`（§7.4）と、それを読む E2E 用の静的サーバ `web/scripts/serve.mjs`。
- **配信の上限の検査**を dist 全体で（`web/scripts/check-dist-limits.mjs`。S4 レビュー低-3）。
- **`cmd/precompute -tables=false`**（表を作らない。サイトのデータ用）と試験 `TestRun_WithoutTables`。`verify-plan.py ack --none` で記録（既定の出力・表・結合は変えていない）。
- **CI**: Web job に、サイトデータと WASM の用意・ビルド（配信の上限を含む）・Playwright の E2E を足した。Dependabot に npm（`/web`）を足した。
- **docs**: `docs/frontend.md`（新規）、`docs/precompute.md`（サイトは表を配らない）、`README.md`（しくみ）。
- **計測**: `web/scripts/measure/browser-timing.mjs` と結果 `docs/progress/S5-measurements/`。

### 計測（§8-6: Chromium 141・4 コア、同じ環境で背中合わせに取り、ほかの重い処理と並行させていない）

ブラウザ（WASM・Worker）で、画面から「提案を出す」を押してから結果が出るまで（wall）。5 キャラ（ryu・kimberly・cammy・zangief・dhalsim）の最大。単位は秒。各セルは「表あり / 表なし」。

| KA | 重ねる | 汚連携 | 有利調整 |
|---|---|---|---|
| 20 | 0.22 / 0.14 | 0.03 / 0.01 | 0.01 / 0.01 |
| 40 | 0.05 / 0.03 | 0.02 / 0.05 | 0.01 / 0.01 |
| 60 | 0.32 / 0.37 | 0.28 / 0.37 | 0.02 / 0.01 |
| 80 | 4.20 / 3.52 | 3.97 / 3.86 | 0.08 / 0.08 |
| 100 | 6.15 / 5.80 | 7.90 / 6.90 | 0.15 / 0.15 |

- KA 20 の重ねるは、キャラの最初の計算なので WASM の起動と moves の読み込み（0.09〜0.19 秒）を含む。
- **表ありの値は `8b5f740`（表を読む Worker）で取った**。今のコードは表を読まないので、同じ測り方では再現できない（レビュー低-4）。
- 全 31 キャラ（表なし・KA 80/100・重ねると汚連携、124 条件）: wall の中央値は KA 80 で 1.8 秒・KA 100 で 2.9 秒。最大は 8.2 秒（marisa KA 100 汚連携）。誤りは 0 件。
- WASM の線形メモリ（計算の後）: KA 100 は中央値 415 MiB・最大 833 MiB（ingrid 汚連携）、KA 80 は最大 537 MiB、KA 60 以下は最大 97 MiB（5 キャラ）。chromium のプロセス 1 つの RSS は最大 1.06 GB（0.5 秒ごとに取った）。有利調整のメモリは測っていない。
- 計測は 1 回ずつで、ばらつきは取っていない。

### 検査の結果（最終の値は末尾の「最終確認」）

- vitest: 10 ファイル・94 件 pass。
- E2E: 16 本 pass。内訳は、本家の正解と画面の結果を sha256 で突き合わせる 9 本（ryu KA 43 の 7 条件・ken KA 43 全種別・zangief KA 79）と、主要フロー 7 本。
- `go test ./cmd/precompute/`: ok。
- CI run 38164794212（`1346893`）: 6 job すべて success（Web: tsc・vitest・用意・ビルド・E2E）。

### §8-3 わざと壊して赤を確かめた

| 壊し方 | 赤になった検査 |
|---|---|
| `_headers` で `clipboard-write=()`（拒否） | E2E のコピー（ボタンが「Could not copy」）。§7.4 の注記（拒否するとコピーが失敗する）を実ブラウザで確かめた |
| `_headers` から `'wasm-unsafe-eval'` を外す | E2E の共有 URL（結果が出ない） |
| Worker がクエリを渡さない（配線を 1 行壊す。§8-5） | E2E の本家突き合わせ 6 本 |
| エンジンの limit を 200 にする | E2E の本家突き合わせ 2 本（200 件を超える条件） |
| 結果をキャッシュに入れない | 画面の試験「同じ条件は計算せず…」 |
| メモリ解放を外す | `engineClient` の試験 |
| fatal（WASM が落ちた）で Worker を捨てない／落ちた後も同じ api を使う／undefined を素通しする | `engineClient`・`engineHost` の試験 |
| 送っていない絞り込みでも「絞り込みを外すと」を出す | 画面の試験（汚連携＋ガード時 0F 以上） |
| zh-Hant を手で直す | 再生成一致の試験 |
| ko にかなを混ぜる／en のキーを 1 つ変える | i18n の試験（かな混入／キー集合・差し込み名） |
| 安全飛びでも接近を送る／共有 URL の安全飛びで URL の remain を信じる | `conditions`・画面の試験 |
| 表記の既定を全言語 SRK にする | 表記・画面の試験 4 本 |
| `-tables=false` でも sp/ を書く | `TestRun_WithoutTables` |
| 配信の上限（1 件だけ超える入力） | `checkLimits` の試験（1 件と出る・ちょうどは 0 件） |

どれも戻した後に緑。

## 判断と理由

1. **「さらに表示」は計算し直さない**（上限 3000 件で 1 回数え、画面で 200 件ずつ出す）。本家の `limit` は並べ替えの後で先頭を切るだけ（`service.go` の `proposals[:limit]`）なので、結果は同じ。KA 80 以上で数秒の計算を繰り返さずに済む。画面の先頭 200 件 = 本家の limit 200 の結果であることを E2E で確かめた（ryu KA 43 既定・zangief KA 79 既定）。
2. **当てる技の種別が 0 個のときは押せなくした（本家からの変更）**。本家は空の `target_types` を送り、エンジンが既定の種別に戻すため、押した条件と違う結果が出ていた。
3. **URL に言語・表記を入れない**。共有した相手の言語・表記を上書きしないため。ストレージを使えない（§7.3）ので、再読み込みすると言語は自動選択に戻る（推測: 不便なら URL に任意で足す）。
4. **コピーの中身は「レシピ（選択中の表記）(フレームの説明)」**（推測: 貼った先で条件が分かるように、画面の行と同じ情報にした）。
5. **zh-Hant は生成物をコミットする**（ビルド時に毎回作るのでなく）。tsc・vitest が生成を待たずに回り、試験が再生成と一致を見る（moves DB と同じ流儀）。HANDOVER §6 の「ビルド時に自動生成」からは形を変えたが、zh-Hant を手で書かない点は同じ。
6. **中国語・韓国語も全キーを訳した**（D5 は「ラベル・操作説明のみ」）。§8-9 のキー集合の一致を保つため、制約の説明まで仮訳した。機械翻訳の注記を出している。
7. **wasm_exec.js は Worker から完全な URL で動的 import する**。Vite の開発サーバは、相対・絶対パスの動的 import に `?import` を付け、public/ のファイルを読ませないため。ビルドでも開発サーバでも動くことを確かめた。
8. **ストレージの不使用を E2E で確かめる**（localStorage・sessionStorage・IndexedDB・cookie がすべて空）。
9. **依存**: すべて HANDOVER に名前があるか、許可ライセンス。推移的な依存に MPL-2.0（`lightningcss`。Vite が CSS の処理に使うビルド時だけの依存で、配信物には入らない）、MIT-0・BlueOak-1.0.0（許可的）がある。強いコピーレフトは無い（`pnpm licenses list` で確認）。`pnpm-workspace.yaml` の `minimumReleaseAge: 10080`（公開 7 日未満の版を入れない）で、最新より少し古い版になっているものがある。解除条件は無し（サプライチェーン対策として残す）。

## 残課題・既知の問題

- **KA 80 以上は数秒かかり、計算中のメモリは最大約 0.8 GiB**（KA 100 の重い条件）。メモリの少ない端末（スマホ）では Worker が落ちうる。落ちたときは誤り（`worker_failed` / `wasm_crashed`）を出して作り直す（固まらない）。実機のスマホでは測っていない。D（よく使う条件の事前計算）を足すかは S6 の計測で決める（開発者決定）。
- **D11 は「クラウドで再計算しない」の意味で満たしている**（開発者決定）。同じ条件でも、別の利用者のブラウザでは計算し直す。
- **結果が 3000 件を超えると、3000 件より後は出せない**（本家と同じ。理由も出ない。レビュー低-5）。
- **E2E は Chromium だけ**。Firefox・Safari（WebKit）では回していない。
- **言語は再読み込みで自動選択に戻る**（ストレージを使わないため。判断 3）。
- **中国語・韓国語は機械翻訳の質**（注記と Issue へのリンクあり）。
- **`_headers` の同じ名前のヘッダの連結は推測**（Cloudflare の説明による）。実配信での確認は S7 のヘッダ診断。
- `cmd/precompute` の manifest で、表の項目は `omitempty` なので「units・rows が 0 の表」と「表なし」を区別しない。表を作るとき units・rows は 1 以上なので、実害は無い（レビュー低-3 は不採用）。
- **繋ぎ表の仕組み（S4）は CI の検査ごと残している**。サイトでは使わない。→ S5 後の開発者決定 2 で消すと決まった（S6 の冒頭）。
- Worker の最上位の `.catch`（`handle` が万一 reject したときの返事）を通る試験が無い。実 WASM を落として Worker → 画面まで通す検査も無い（単体試験で undefined・例外の経路を押さえた。再レビュー新-低-2・新-低-3）。
- Dependabot の gomod は未登録（S5 で npm だけ足した）。
- デプロイ（Cloudflare）は未着手（S7 以降・開発者）。

## 次の段階への入力（S6 計測と投票）

- **計測（beacon）の材料**: 計算の結果には、Worker 側の所要（`boot`・`load`・`compute`）、メインスレッドの経過時間（`wallMs`）、WASM のメモリ量（`wasmMemoryBytes`）が入っている（`SuggestOutcome`、画面では件数の横の `data-testid="timing"` の `data-timings`）。キャッシュから出したかも `fromCache` で分かる。HANDOVER §7.1 の `cacheHit`・`ms` に使える。送る場所は `App.tsx` の `execute`（計算の後）。
- **D の判断材料**: S6 の計測で、KA 80 以上の実行の割合と所要を見る（S5 後の開発者決定 1）。
- **最初にやること**: 繋ぎ表の仕組みを消す（S5 後の開発者決定 2。対象は上の節）。
- **CSP**: `connect-src 'self'` なので、beacon・投票の受け口は同じオリジンに置く（Cloudflare の Worker の経路）。
- **D17（グッド/バッド）**: 行は `Results.tsx` の `Row`。並べ替えは「票 → 本家の順位」で、画面は 3000 件を持っている（判断 1）ので、並べ替えの後で 200 件ずつ出せる（「並べ替えの後で切る」を守れる）。行の鍵は `itemKey`（手順の code 列＋N）。
- **配信物**: `web/dist`（`pnpm assets && pnpm build`）。ファイル 39・合計約 6.7 MB（最大は `setplay.wasm` 3.9 MB）。CI は検査だけで、成果物として残していない。
- 計測スクリプト `web/scripts/measure/browser-timing.mjs` は、画面の `data-testid` に依存する。

## S5 後の開発者決定（2026-10-12）

1. **D（重い条件の結果を前もって作って配る）は、S6 の計測を見て決める。** S6 で KA 80 以上の実行の割合と待ち時間を計測し、多ければ作る。
2. **S4 の繋ぎ表の仕組みは消す。** 消す対象は、表のコード（`internal/service/setplay/table.go`・`engine` の `LoadTable`・WASM の `loadTable`・`cmd/precompute` の表の生成と `-tables`）と、CI の全 KA の突き合わせ（sweep）と、WASM・Node の表あり経路。
   - 理由: サイトは表を使わず（計算方式 C）、D も最終結果を配る方式なので、表は要らない。
   - ★本報告の「残課題」の「D を足すときに再利用できる」は誤りだった。D は最終結果を配る方式で、作るのはエンジンそのもの。
   - 時期: S6 の冒頭（または S6 の前の小さな片付け）。`watched` のコードを消すので、`verify-plan.py ack` で記録する。`docs/precompute.md`・`docs/engine.md`・`docs/upstream-diff.md` の表の記述も消す（AGENTS §8-8 の 3 か所 grep）。

本節は報告の後に追記した（docs だけの変更）。

## 最終確認（レビュー 2 往復の取り込み後、HEAD `99e9b7d` で回した結果）

出力はファイルへ書き、合否は `EXIT=` と件数で見た。`.upstream` は `bash scripts/fetch-upstream.sh` で取得済み。go test は `3c4bc1e` の後に回した（その後 Go のコードは変えていない: `git diff --stat 3c4bc1e HEAD -- '*.go'` は空）。

```
$ go test -count=1 -v -timeout 35m ./... > /tmp/s5-go.txt 2>&1; echo EXIT=$?
EXIT=0
PASS(全階層 ^\s*--- PASS)=384 FAIL=0 SKIP=0
    conformance_test.go:340: 突き合わせた条件: 10416(キャラ 31 ・エンジンと繋ぎ表の 2 経路 ・不一致 0)
    table_test.go:227: 突き合わせた呼び出し: 133064(キャラ 31 ・KA 9 通り ・不一致 0 ・6000 件上限 9712 ・単位上限に達した (キャラ,KA) 54)
$ go vet ./...                       → EXIT=0
$ gofmt -l(追跡中の .go、_reference 除く) → 出力なし
$ go mod tidy -diff                  → EXIT=0
$ cd web && pnpm exec tsc --noEmit   → EXIT=0
$ pnpm exec vitest --run > /tmp/f-vitest.txt 2>&1; echo EXIT=$?
EXIT=0
 Test Files  10 passed (10)
      Tests  97 passed (97)
$ bash scripts/prepare-assets.sh > /tmp/f-assets.txt 2>&1; echo EXIT=$?   (public/data・public/wasm を消してから)
EXIT=0
キャラ: 31 / ファイル: 32(上限 20000) / 最大: moves/jamie.246608900f96eda9.json 102287 bytes / 合計: 2257870 bytes / gzip 198432 bytes / 違反: 0 件
setplay.wasm: 3947021 bytes / gzip -9: 1088610 bytes
$ pnpm build > /tmp/f-build.txt 2>&1; echo EXIT=$?
EXIT=0
ファイル: 39(上限 20000) / 最大: wasm/setplay.wasm 3947021 bytes(上限 26214400) / 合計: 6653506 bytes / 違反: 0 件
$ PW_CHROMIUM_PATH=/opt/pw-browsers/chromium-1194/chrome-linux/chrome pnpm exec playwright test > /tmp/f-e2e.txt 2>&1; echo EXIT=$?
EXIT=0
  16 passed (25.7s)
$ python3 scripts/check-licenses.py         → 違反: 0 件
$ bash scripts/check-actions-pinned.sh      → 違反: 0 件
$ python3 -I scripts/ci/verify-plan.py check → 違反: 0 件
$ python3 -I scripts/ci/verify_plan_test.py → EXIT=0 OK
$ bash -n(hooks・scripts・upstream・web/scripts の .sh) → 誤りなし
```

- **完了条件（主要フローの E2E が緑）**: 満たした。E2E 16 本（本家の正解との突き合わせ 9 本＋主要フロー 7 本）が手元と CI の両方で緑。
- 手元の E2E は Playwright 1.63 に、版の合う Chromium が無いため Chromium 141（`/opt/pw-browsers/chromium-1194`）を `PW_CHROMIUM_PATH` で使った。CI は `playwright install` の版を使う。
- 削除されたファイル: 0（`git diff --diff-filter=D --name-only 5f1f7ffa HEAD`）。消した試験は無い。変えた既存の試験は `cmd/precompute/main_test.go`（`run` の引数を 1 つ足した）だけ。

### CI

- run 38164794212（`1346893`）: 6 job すべて success。Web job で tsc・vitest・用意・ビルド・E2E が通った。
- run 38165453253（`3c4bc1e`）: 6 job すべて success。
- run 38166068419（`d23b3bd`、再レビューの取り込み）: success。
- 最初の push の run 38164263435（`8b5f740`）はリポジトリ検査で赤だった（`cmd/` を変えたのに `verify-plan.py ack` が無かった）。次のコミットで ack を記録して直した。

### `git diff --stat 5f1f7ffa9b3b4cbe687ed36dbeec9616a7a0e2b1 HEAD`（HEAD = 99e9b7d）

```
 .github/dependabot.yml                            |   14 +-
 .github/workflows/ci.yml                          |   58 +-
 .gitignore                                        |    4 +
 README.md                                         |    6 +-
 cmd/precompute/main.go                            |   50 +-
 cmd/precompute/main_test.go                       |   45 +-
 docs/frontend.md                                  |  107 ++
 docs/precompute.md                                |   12 +-
 docs/progress/S5-measurements/mem-all.json        | 1618 +++++++++++++++++++++
 docs/progress/S5-measurements/mem-low.json        |  396 +++++
 docs/progress/S5-measurements/rss-all.txt         |    1 +
 docs/progress/S5-measurements/timing-notable.json |  890 ++++++++++++
 docs/progress/S5-measurements/timing-table.json   |  890 ++++++++++++
 docs/progress/S5-report.md                        |  163 +++
 docs/progress/S5-review.md                        |  344 +++++
 scripts/check-licenses.py                         |    1 +
 scripts/ci/logic-version.json                     |   12 +-
 web/e2e/conformance.spec.ts                       |   68 +
 web/e2e/setplay.spec.ts                           |  153 ++
 web/index.html                                    |   14 +
 web/package.json                                  |   42 +
 web/playwright.config.ts                          |   29 +
 web/pnpm-lock.yaml                                | 1563 ++++++++++++++++++++
 web/pnpm-workspace.yaml                           |    2 +
 web/public/_headers                               |   23 +
 web/public/favicon.svg                            |    1 +
 web/scripts/check-dist-limits.mjs                 |   22 +
 web/scripts/dist-limits.mjs                       |   31 +
 web/scripts/gen-zh-hant.mjs                       |   12 +
 web/scripts/headers.mjs                           |   44 +
 web/scripts/measure/browser-timing.mjs            |   46 +
 web/scripts/prepare-assets.sh                     |   12 +
 web/scripts/scripts.test.mjs                      |   74 +
 web/scripts/serve.mjs                             |   40 +
 web/scripts/zh-hant.mjs                           |   26 +
 web/scripts/zh-hant.test.mjs                      |   20 +
 web/src/components/App.test.tsx                   |  327 +++++
 web/src/components/App.tsx                        |  203 +++
 web/src/components/ConditionsPanel.tsx            |  395 +++++
 web/src/components/Footer.tsx                     |   30 +
 web/src/components/LimitationNotice.tsx           |   29 +
 web/src/components/Results.tsx                    |  187 +++
 web/src/components/SettingsBar.tsx                |   53 +
 web/src/components/TargetPicker.tsx               |   73 +
 web/src/lib/conditions.test.ts                    |  182 +++
 web/src/lib/conditions.ts                         |  271 ++++
 web/src/lib/constants.ts                          |   53 +
 web/src/lib/data.ts                               |   77 +
 web/src/lib/engineClient.test.ts                  |  123 ++
 web/src/lib/engineClient.ts                       |  108 ++
 web/src/lib/engineTypes.ts                        |   34 +
 web/src/lib/formState.test.ts                     |   50 +
 web/src/lib/formState.ts                          |  147 ++
 web/src/lib/i18n.test.ts                          |   74 +
 web/src/lib/i18n.ts                               |   63 +
 web/src/lib/loader.ts                             |   35 +
 web/src/lib/lru.test.ts                           |   21 +
 web/src/lib/lru.ts                                |   33 +
 web/src/lib/notation.test.ts                      |   42 +
 web/src/lib/notation.ts                           |   37 +
 web/src/locales/en.json                           |  153 ++
 web/src/locales/ja.json                           |  153 ++
 web/src/locales/ko.json                           |  153 ++
 web/src/locales/zh-Hans.json                      |  153 ++
 web/src/locales/zh-Hant.json                      |  153 ++
 web/src/main.tsx                                  |   33 +
 web/src/styles.css                                |   87 ++
 web/src/test/setup.ts                             |    1 +
 web/src/worker/engineHost.test.ts                 |  109 ++
 web/src/worker/engineHost.ts                      |  122 ++
 web/src/worker/protocol.ts                        |   26 +
 web/src/worker/setplay.worker.ts                  |   73 +
 web/tsconfig.json                                 |   21 +
 web/vite.config.ts                                |   20 +
 74 files changed, 10703 insertions(+), 34 deletions(-)
```

## 目視確認での追加（2026-10-12）

開発者の手元（devcontainer）で `pnpm install && pnpm assets && pnpm dev` の後、画面が真っ白だった（`pnpm assets` は成功）。原因は未特定で、環境の調査は開発者が行う。
こちらでは、次に止まったときに理由が画面に出るようにした。

- `index.html` の `#root` に「読み込み中… / Loading…」を置いた（React が描画を始めると消える）。
- `public/boot.js`（CSP の `script-src 'self'` で読める普通のスクリプト）を追加。描画の前の例外と、スクリプトの読み込みの失敗を `#boot-status` に書く。
- `main.tsx` で React 19 の `onUncaughtError` を使い、描画中の例外で画面ごと消えたときも理由を出す。
- E2E `web/e2e/boot.spec.ts`（3 本）。§8-3: `boot.js` を外すと 2 本が赤。全体は E2E 19 passed・vitest 97 passed（どちらも EXIT=0）。

## 目視確認での開発者要望（2026-10-12。後の段階の課題）

開発者が実物を確認し、観点（本家との結果の一致・各機能）は問題なしとした。見た目・使い勝手として次の 2 件の要望があり、後の段階の課題として扱う（開発者: 最後の方の課題でもよい）。

1. **キャラの入力で絞り込める選択**（本家の挙動を移植する）。
   - 本家では、`r` と打てばリュウ、`リ` と打てばリュウが候補に出る。キャラが 31 人いて、プルダウンから選ぶのは大変。
   - **実装は本家に揃える**（開発者の指示）。後続の作業で本家の実装を参照してよい（開発者の許可。AGENTS §0 の「combomgr は読みに行かない」の例外として、このキャラ選択の部品に限る。データの取込元は公開版 tacpendium のまま＝D2 は不変）。
   - `_reference/combomgr/` のスナップショットには、本家のキャラ選択の実装は入っていない（`web/src` を `character` で検索して、セットプレイの 3 ファイルだけ）。
   - 推測: 照合には、キャラの日本語名・英語名・code に加え、読み（かな）の対応表が要る。今の moves DB とマニフェストには読みが無い。本家がどこから読みを取っているかを、参照時に確かめる。
   - 5 言語の画面での表示と照合の扱い（中・韓の画面でどの名前で引けるか）を決める。
2. **初期状態へ戻すボタン**（リセット）。
   - 条件の欄・結果・URL を初期状態に戻す。
   - 言語と表記は戻すか戻さないかを決める（推測: 戻さない。利用者の設定に当たるため）。
