# フロント（`web/`）

S5 で作った。Vite + React + TypeScript（strict）、パッケージ管理は pnpm（版は `web/package.json` の `packageManager`）。
計画の正本は HANDOVER §5・§6・§7.2〜§7.4。

## 動かし方

```sh
cd web
pnpm install --frozen-lockfile
pnpm assets          # public/data(manifest・moves)と public/wasm(setplay.wasm・wasm_exec.js)を作る。Go が要る。gitignore
pnpm dev             # 開発サーバ
pnpm test            # vitest(単体・画面)
pnpm build           # tsc → vite build → 配信の上限の検査(dist 全体)
pnpm e2e             # Playwright。dist を public/_headers どおりのヘッダで配って回す(先に assets と build)
```

手元の Chromium が Playwright の版と合わないときは `PW_CHROMIUM_PATH=<chrome の実行ファイル>` を渡す（CI は `playwright install`）。

## しくみ

```
画面(React) ──postMessage──▶ Web Worker ──▶ setplay.wasm(cmd/wasm。engine パッケージ)
   │  条件 ⇄ URL クエリ(zod)          │ moves/<code>.<hash>.json を fetch して load
   │  結果のメモリキャッシュ(LRU 30)  │ suggest(キャラ, KA, クエリ) → JSON
   └ manifest.json・moves を fetch    └ 結果と所要(boot・load・compute)・WASM のメモリ量を返す
```

- **計算はブラウザの中だけ**で行う（サーバでは計算しない）。配るのは静的ファイルだけ。
- **計算方式 C**（S5 の開発者決定、2026-10-11）: S4 の繋ぎ表は配らず（S6 で仕組みごと消した）、WASM のエンジンで数える。理由は `docs/precompute.md` の「計算はブラウザの WASM」と `docs/progress/S5-report.md`。
- **中断**は Worker ごと `terminate` して、次の計算で作り直す（`cmd/wasm` は取り消しを受けない）。
- **メモリ**: Go の WASM は広げたメモリを返さない。計算の後に線形メモリが 256 MiB（`WASM_MEMORY_RELEASE_BYTES`）を超えていたら、Worker を捨ててメモリを返す（次の計算で作り直す。起動は 0.1〜0.2 秒）。計算中の最大は KA 100 の重い条件で約 0.8 GiB（実測）。WASM が落ちたら（panic。Go のメモリ不足も同じ経路になる見込み＝推測。`cmd/wasm` は recover しないので、その呼び出しは `undefined` を返し、以後は例外を投げる）`wasm_crashed`、Worker 自体が落ちたら `worker_failed` の誤りを出し、どちらも Worker を捨てて次の計算で作り直す（固まらない）。
- **「さらに表示」**: エンジンの `limit` は「並べ替えの後で先頭を切る」だけなので、上限 3000 で 1 回だけ数え、画面で 200 件ずつ出す（計算し直さない）。画面の先頭 200 件が本家の `limit` 200 の結果と同じことは E2E（`e2e/conformance.spec.ts`）が見る。
- **同じ条件**はメモリキャッシュから即表示する（鍵はキャラ・KA・エンジンのクエリ）。計算中は「提案を出す」を押せない（HANDOVER §7.2）。
- **状態**はメモリと URL クエリだけ。cookie・localStorage・sessionStorage・IndexedDB は使わない（HANDOVER §7.3。E2E が確かめる）。

## 計測・グッド/バッド・プライバシーポリシー（S6）

`docs/telemetry.md`。要点: 「提案を出す」ごとに beacon を 1 回（抽出率は `public/config.json`）。計算の前に票を読み（1.5 秒で打ち切り、読めなければ票なし）、エンジンに渡す。各行に 👍 / 👎（押した票は次の検索から並びに効く）。フッターの「プライバシーポリシー」（`#privacy` で直接開く）。
`/api/*` は同じオリジンの Cloudflare Worker（`web/edge/`）。手元の `pnpm dev` では `/api` が無いので、票は `unavailable`、計測は送っても捨てられる（画面は動く）。`pnpm build` の後の `node scripts/serve.mjs dist` は Worker の本物のコードで `/api` を受ける。

## 条件と URL（共有）

`web/src/lib/conditions.ts`。「提案を出す」で確定した条件を URL に書く（`history.replaceState`）。URL に条件があれば、読み込み後に 1 度だけ自動で計算する。

| URL のキー | 条件 | 既定 |
|---|---|---|
| `c` | キャラ（manifest の code） | 必須 |
| `ka` | KA（0〜100 の整数。ローカル版は 0 以上で上限なし＝`docs/local.md`） | 必須 |
| `mode` | `meaty` / `gap` / `filler` | `meaty` |
| `n_min` / `n_max` | 重ねる持続の位置（`n_max` 0 は上限なし） | 1 / 0 |
| `g_min` / `g_max` | 汚連携の空振りの位置（1〜13） | 1 / 13 |
| `remain` | 有利調整の「調整後の有利」 | 4 |
| `sj` | 安全飛びの種類（`f5` / `f6` / `true`。有利調整のときだけ。remain はキャラの値で引き直す） | なし |
| `types` | 当てる技の種別（カンマ区切り） | `normal,unique,special_projectile` |
| `zero` / `approach` / `block_safe` / `chain` | ダメージ 0 を含める / 絞り込み 3 つ（`1` で ON） | OFF |
| `move` | 「重ねる技」を「1 つの技を指定」で選んだ技（code） | なし |
| `sort` | `n` / `target` | `n` |

- 既定値と同じキーは書かない。**言語・表記は URL に入れない**（受け手の設定を上書きしない）。
- キャラ・KA が無いか誤りなら自動計算しない。ほかのキーの誤りはそのキーだけ既定値に戻し、画面に「読めなかった」と出す。いまのモードで押せない組（重ねるの n、汚連携の g の最大 < 最小）・モードに合わないキー（有利調整でない `sj`、重ねるでない `block_safe`）・安全な整数を超える数も同じ扱い。数の規則は画面の欄と同じで、画面で押せる条件は共有しても同じ条件になる（往復の試験）。
- エンジンに渡すクエリは本家の画面と同じ規則で組む（`toEngineQuery`。`n_max` は重ねるだけ、`g_*` は汚連携だけ、`block_safe_only` は重ねるだけ、安全飛びでは `approach_only` を掛けない）。

### この条件を共有・このサイトを共有（開発者決定 2026-10-20・要望 2026-10-24）

`web/src/components/Share.tsx`（2 か所で使う部品）・`web/src/lib/share.ts`。

- **この条件を共有**: 結果が 1 件以上出ているとき、件数の行の下に出す。
- **このサイトを共有**: フッターの先頭（結果の共有と取り違えないよう離して置く）。結果の有無にかかわらず出す。共有する URL はサイトのトップ（`siteUrl`＝ページの origin と path。条件・ハッシュは付けない）、文は紹介の一言（`share.appText`）。
- ボタンは色付き（`button.primary`。X のリンクは枠線付きのボタンの形 `a.button-like`）。

- 共有する URL は、ページの origin と path に結果の条件のクエリ（`conditionsToSearch`）を付けたもの（`shareUrl`。ハッシュは付けない）。画面が `replaceState` で書いた後の `location.href` と同じ文字列になる（E2E が確かめる）。App の `location` は起動時の写しで、`replaceState` の後も古いままなので、そのまま使わない。
- 端末の共有メニュー（`navigator.share`）があればボタン 1 つでそれを開く（主にスマホ。macOS の Safari、Windows の Chrome・Edge など PC でもあるものはこちらになり、OS の共有メニューにコピー・X 等が並ぶ）。渡すのは title = アプリ名・text = 共有の文・url。メニューを閉じただけ（`AbortError`）なら何も出さず、ほかの失敗は「共有できませんでした」。無ければ「URL をコピー」と「X に投稿」（`https://x.com/intent/post?text=…&url=…` を新しいタブで開くリンク。`rel="noopener noreferrer"`）。
- 共有の文は選んでいる言語で「キャラ名 KA n の〈モード〉 — Tacpendium-Setplay」（例「リュウ KA 43 の重ね — Tacpendium-Setplay」。`share.text`・`share.mode.*`。安全飛びの検索ではモードの代わりに安全飛びの種類＝`setplay.safeJumpKind.*`。キャラ名は ja だけ日本語名、ほかは英語名＝キャラ選択と同じ規則）。絞り込みは文に入れない（URL に入る）。区切りは `SHARE_TEXT_SEPARATOR`。
- 文の長さ: X の数え方（URL は 23、CJK は 2）で、全ロケール・長いキャラ名・KA 100 でも 280 以内（`share.test.ts`）。
- 「URL をコピー」は URL だけを写す（文は付けない）。アドレス欄にもそのまま貼れ、SNS・チャットに貼れば OGP のカードが出るため（開発者への提案 2026-10-24）。
- X 以外の SNS のリンクは置かない（共有メニューがある端末はそこから、無い端末は URL のコピーで足りる。開発者決定 2026-10-20）。
- 操作のまとまりは `role="group"` で「この条件を共有」「このサイトを共有」の名前を付ける。条件が変わると作り直す（前の条件の「コピーしました」を残さない）。
- **ローカル版ではどちらも出さない**（URL が 127.0.0.1 で相手が開けない。開発者決定 2026-10-20）。
- 外部の JS・SNS のウィジェットは使わず、押した回数も計測しない。`_headers`（CSP・Permissions-Policy）とプライバシーポリシーは変えていない（`web-share` は Permissions-Policy に無く、既定の self で使える。X へは利用者が押したリンクで移るだけ）。
- OGP: `web/index.html` に `og:title`・`og:description`（日本語の固定文）・`og:type`・`og:url`（サイトの URL）・`og:image`（同じ host の `/og.png`。幅・高さ・alt も）・`twitter:card=summary_large_image`。**カードはアプリ共通**（キャラ・条件ごとには変えない。変えるには Worker で HTML を書き換える仕組みが要る。開発者決定 2026-10-24）。画像は `web/public/og.png`（自作の 1200×630。ゲームのロゴ・画像は使わない）で、`node web/scripts/og-image.mjs` で作り直してコミットする（字形が環境で変わらないよう英語だけ）。サイトの URL を変えたら `og:url`・`og:image` と E2E（`setplay.spec.ts`）を直す。ローカル版の HTML にも同じタグが入るが、ローカル版は共有しないので害はない。条件ごとのカード（キャラ名入り）は、配信側で HTML を書き換えないと出せないので作っていない。
- `og.png` にも `_headers` の `/*` の `Cross-Origin-Resource-Policy: same-origin` が付く。SNS のサーバが取りに来る形（ブラウザの外）には効かないが、**別のサイトのページがブラウザで直接 `<img>` に読む形では読めない**（手元で確かめた。`docs/progress/feat-share-report.md`）。

### 本家の画面から変えたところ

- 採用・不採用・採用名・確認できた条件は無い（ユーザーデータに依存するため。HANDOVER §2.2）。代わりに行ごとの**コピー**（D16）。
- **重ねる技の種類が 0 個のときは押せない**（「1 つの技を指定」で技を選んでいるとき・有利調整では問わない）。**「1 つの技を指定」で技が未選択のときも押せない**（「技を選んでください」。隠れた種類の条件で計算しないため。種類の誤りは種類で選んでいるときだけ出す。S7）。本家は種別 0 個で `target_types=` を送り、エンジンが既定の種別に戻すため、押した条件と違う結果が出ていた。
- 安全飛びの採用（ジャンプから出す技の選択）は無い。安全飛びの行は繋ぎだけを出し、種類と残りフレームを添える（本家の行の表示と同じ）。
- **重ねる技**（S7・A 案）: 本家の「当てたい技の種別」と「特定の技から探す」を 1 つの枠「重ねる技」にまとめ、「選び方: 種類で選ぶ／1 つの技を指定」で切り替える。「種類で選ぶ」に戻すと選んでいた技を外す。URL の `move` があれば「1 つの技を指定」で開き、技が一度入ったら選び方を固定する（技の欄を空にしても種類の欄へ勝手に変わらない）。条件と URL の形は変えていない。
- **キャラの名前検索**（S7）: 本家と同じ規則（日本語名・英語名・code の語の先頭一致。NFKC・小文字。`web/src/lib/characterSearch.ts`）。**見た目と動きも本家に揃えた**（開発者の要望 2026-10-18）: 箱を押すと検索欄と候補の一覧が開き、打つと絞られ、↑↓ で移って Enter かクリックで選ぶ。検索欄で Enter は先頭を選ぶ。Esc か外側のクリックで閉じ、閉じたら検索語を消す。本家は Radix の Popover を使うが、依存を増やさないよう自前の部品にした（`web/src/components/CharacterSelect.tsx`）。
- **条件をリセット**（S7）: 条件・結果・URL を最初の状態に戻す。計算中なら中断する。言語と表記は利用者の設定なので戻さない。

## 使い方と注意（S7）

画面冒頭の説明と、折りたたみ 3 つ（使い方・注意と対策・レシピのルール）。`web/src/components/Guide.tsx`、文言は `guide.*`（5 言語）。開発者の案（2026-10-18）を要約したもので、旧「提案の制約について」（`limitations.*`）を置き換えた。

- エンジンのコメント（`internal/service/setplay/*.go` の「M19-LIMITATION-NOTICE」「ヒント文面（web i18n `setplay.limitations.*`）」）は公開版 tacpendium から移したままで、本家の画面の文言を指す。**このリポジトリで対応する文は `guide.*`**。連打キャンセル（`refine.go`）・ラッシュ・target の列挙・filler の規則を変えたら `guide.ruleChain`・`guide.ruleExcluded`・`guide.rush` 等を見直す。
- 旧項目のうち、要約の方針（長いと読まれない）で**載せなかったもの**: 有利フレームが 100 を超える場合（入力欄が 0〜100 しか受け付けない。S8 で KA の説明にローカル版への案内を足した）・持続未入力の技・単独で最速でも当たらない技・ダメージ 0 の技（既定）・派生技の扱い（親を前に置く表記）・候補が多いときは上位のみ（「さらに表示」がある）・安全飛びは通常の前ジャンプのみ・汚連携は相手次第・状態を前提とする技が繋ぎに混じる・設置技の硬直を持続として扱う・提案が無くても無いとは限らない。計算の挙動は変えていない。
- 本家への導線は公開版 tacpendium（`DATA_SOURCE_URL`）。

## コピー（D16）

選択中の表記で `レシピ (フレームの説明)` を `navigator.clipboard.writeText` で書く（例: `2LP > 2LP > 236HP (Hits on active frame 3)`）。
失敗したら（権限の拒否・安全でない接続）ボタンに「コピーできませんでした」と出す。
**`Permissions-Policy` で `clipboard-write` を拒否すると、実ブラウザ（Chromium）で書き込みが失敗する**ことを S5 で確かめた（E2E の壊し方で「コピーできませんでした」になる）。そのため `_headers` は `clipboard-write=(self)` だけを許可している（HANDOVER §7.4）。

## 多言語（HANDOVER §6）

- `web/src/locales/` の `ja` / `en` / `zh-Hans` / `ko` を手で書き、**`zh-Hant.json` は `pnpm gen:locales`（OpenCC `s2twp`）で `zh-Hans.json` から作ってコミットする**。手で直さない（試験が再生成と一致を見る）。
- 言語は `navigator.languages` から選ぶ（`zh-CN`/`zh-SG` → 簡体、`zh-TW`/`zh-HK`/`zh-MO` → 繁体。文字体系 `Hans`/`Hant` が書いてあれば地域より優先）。画面上で切り替えられる。
- 中国語・韓国語には「機械翻訳です。改善の提案を歓迎します」と Issue へのリンクを出す。
- 技の表記は 4 種（日本語技名・英語技名・SRK・ナンバリング）を切り替える。既定は ja UI = 日本語技名、それ以外 = SRK。技名は翻訳しない。
- 文言・キーを変えたら全ロケールに同時に足す。試験（`src/lib/i18n.test.ts`）が、キー集合の一致・差し込み名の一致・非日本語ロケールへのかなの混入（en・ko は漢字も）を見る。

## 配信のヘッダ（HANDOVER §7.4）

`web/public/_headers`（Cloudflare の形式。ビルドで `dist/` に入る）。E2E の静的サーバ（`web/scripts/serve.mjs`）が同じファイルを読んで同じヘッダで配る。

- CSP: `default-src 'self'; script-src 'self' 'wasm-unsafe-eval'; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'none'; object-src 'none'`
- `X-Content-Type-Options: nosniff`・`Referrer-Policy: strict-origin-when-cross-origin`・`Cross-Origin-Opener-Policy: same-origin`
- S7 で足した: `Cross-Origin-Resource-Policy: same-origin`・`X-Frame-Options: DENY`（CSP の `frame-ancestors` を解さない古いブラウザ向け）・`Strict-Transport-Security: max-age=31536000`（preload は付けない＝`workers.dev` は自分のドメインでない）
- `/api`（Worker の応答）には `_headers` が掛からないので、`web/edge/index.ts` の `API_SECURITY_HEADERS` が同じ値と `default-src 'none'` の CSP を付ける
- `Permissions-Policy`: `clipboard-write=(self)` 以外を拒否
- キャッシュ: 名前にハッシュがあるもの（`/data/moves/*`・`/assets/*`）は immutable。`manifest.json` と `/wasm/*`（名前にハッシュが無い）は `no-cache`（再検証）。

外部の CSS・フォント・スクリプトは読まない。`dangerouslySetInnerHTML` は使わない。

## 検査

| 検査 | どこで |
|---|---|
| 条件 ⇄ URL・エンジンのクエリ・欄の検査・表記・LRU・言語の選択・ロケール・共有の URL と文 | `web/src/lib/*.test.ts` |
| Worker の中身（起動・読み込み・誤り）・Worker の窓口（中断・古い返事・落ちたとき・メモリ解放） | `web/src/worker/engineHost.test.ts`・`web/src/lib/engineClient.test.ts` |
| 画面の主要な振る舞い（偽のエンジンで） | `web/src/components/App.test.tsx` |
| zh-Hant の再生成一致・`_headers`・配信の上限 | `web/scripts/*.test.mjs` |
| **画面の結果 = 本家の正解（sha256・順序まで）**（Go → WASM → Worker → 画面の通し） | `web/e2e/conformance.spec.ts`（9 条件） |
| 主要フロー（提案・表記・さらに表示・モード・安全飛び・絞り込み・共有 URL・この条件を共有・OGP・コピー・中断・5 言語・ストレージ不使用） | `web/e2e/setplay.spec.ts` |
| 計測と投票の通し（S6。画面 → beacon → Worker → Analytics Engine／画面 → D1 → WASM → 画面） | `web/e2e/telemetry.spec.ts`（`docs/telemetry.md`） |
| 実際に配ったときのセキュリティのヘッダ（S7。静的アセットと `/api` の両方） | `web/e2e/headers.spec.ts` |

E2E は `retries: 0`・直列（AGENTS §8-10）。CI の Web job が vitest → サイトデータと WASM の用意 → ビルド（配信の上限）→ ローカル版のビルド → E2E の順に回す。E2E は project `site`（`serve.mjs` の dist）と `local`（ローカル版の実バイナリ。`e2e/local.spec.ts`。`docs/local.md`）の 2 つで、両方のサーバを起動するので、先に `bash scripts/local/build.sh` でこの OS 向けのバイナリを作っておく。

## 計測（参考）

`web/scripts/measure/browser-timing.mjs` は、画面を通して条件ごとの所要と WASM のメモリ量を測る（件数の横の `data-testid="timing"` の `data-timings`）。
S5 の結果は `docs/progress/S5-report.md` と `docs/progress/S5-measurements/`。
