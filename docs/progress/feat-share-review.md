# feat-share レビュー（この条件を共有）

- 対象: ブランチ `claude/awesome-turing-rrl0yj`、基準 `9880c88`（main）〜 `7e4d6f5`（`feat(share): …`。1 コミット）
- 観点と型: `.claude/commands/review.md`（「段階 S<n>」→ feat-share。完了条件は開発者の要求 1〜6）
- 完了報告 `docs/progress/feat-share-report.md` はまだ無い（レビュー後に書く予定）。そのため「報告の証拠」は実装者の申告と、自分で回した結果で照合した。

## 1. 要約

- 要求 1〜6 はすべて満たしている。共有 URL は `replaceState` 後の URL と一致し（パスは起動時の `pathname` を使い、`replaceState` もパスを変えずハッシュを落とすので一致する）、ローカル版では出さず、X の URL は `URLSearchParams` で組んでいる。
- テストは効いている。実装者が挙げた 4 つの壊し方に「パスを `/` に固定する」を足した 5 つを、作業ツリーの写しで実際に起こし、すべて赤になった（§5）。
- 高の指摘は無い。中は 2 件（zh-Hant の「釋出到 X」が不自然、en の綴りが既存の訳と揃っていない）。低は 7 件（共有データの title の重複、キャッシュから即座に結果が出たとき「コピーしました」が残る、0 件・キャッシュの経路のテスト不足 等）。
- typecheck・unit・build・E2E を自分で回し、実装者の申告（183 件・29 件）と一致した。

## 2. 完了条件への適合

| # | 要求 | 判定 | 根拠 |
|---|---|---|---|
| 1 | 結果が出ているときに共有の操作。`navigator.share` があれば開き、無ければ「URL をコピー」と「X に投稿」(`x.com/intent/post`、`_blank`、`noopener noreferrer`)。X 以外は無し。URL は今のページの URL、文は選んでいる言語でキャラ名と KA | 済 | `web/src/components/Share.tsx:30-80`・`web/src/lib/share.ts:20-32`・`web/src/components/App.tsx:307-315`。`Results.tsx:134` で件数行の下（items>0 の分岐の中）。ja の文は「リュウ KA 40 のセットプレイ \| Tacpendium-Setplay」（App.test で逐語確認） |
| 2 | `index.html` に og:title・og:description・og:type・twitter:card。og:url・og:image は入れず残課題に | 済 | `web/index.html:9-13`。`docs/release-checklist.md:75`（§4）に未として記載。推測の URL は書いていない（`grep -n "og:url\|og:image" web/index.html` はコメント行のみ） |
| 3 | ローカル版では出さない | 済 | `App.tsx:310` の `online &&`（`online = flavor === "site"`）。App.test・E2E `local.spec.ts:54-55` |
| 4 | 外部 JS・iframe・ウィジェット無し、`_headers`・プライバシーポリシー不変、計測無し、依存不変、`dangerouslySetInnerHTML` 無し、`URLSearchParams`、i18n 5 言語同時 | 済 | `git diff 9880c88 --stat -- web/public web/package.json web/pnpm-lock.yaml web/src/components/PrivacyPolicy.tsx` が空。`grep -rn dangerouslySetInnerHTML web/src`（test 以外）は `i18n.ts:60` のコメント 1 件のみ。`report`/`sendBeacon` を Share から呼ぶ箇所なし。5 ロケールに `share.*` 8 キー、zh-Hant 再生成一致テスト（`scripts/*.test.mjs`）が緑 |
| 5 | 純粋関数の単体・App.test（あり／なし／ローカル）・E2E 1 本、わざと壊して赤 | 済 | `share.test.ts`（4 件）、App.test の describe 4 件、`setplay.spec.ts:99`。壊して赤は §5 で再現 |
| 6 | `docs/frontend.md` の該当箇所 | 済 | `docs/frontend.md:64-74`・`124`・`129`。コードとの食い違いは見つからなかった（§3 低-5 は根拠の書き方の粗さ） |

## 3. 指摘

### 高

指摘なし（見たもの: `git diff 9880c88` 全体、共有 URL の組み立てと `main.tsx:39-42` の `replaceState`、ローカル版の分岐、X の URL の符号化、`_headers` の CSP・Permissions-Policy・COOP との関係、docs とコードの突き合わせ、テストの効き目を壊して確認）。

### 中

**中-1 zh-Hant の「釋出到 X」「新標籤頁」が台湾の言い方として不自然**
- 場所: `web/src/locales/zh-Hant.json:172-173`（元は `web/src/locales/zh-Hans.json:172-173` の「发布到 X」「在新标签页中打开」）
- 内容: OpenCC s2twp が「发布」を「釋出」（ソフトの「リリース」の意味の台湾語彙）に変える。投稿の意味では台湾では「發布／發文／分享到 X」。「標籤頁」も台湾では「分頁」が普通。
- なぜ問題か: 利用者に見える文字列（ボタンと aria-label）。生成物なので zh-Hant を手で直すと再生成一致テストが赤になる。
- 直し方の案: zh-Hans 側を変換で崩れない語にする（例「分享到 X」「(在新页面中打开)」等）→ `pnpm gen:locales`。再生成後の zh-Hant を目で確かめる。

**中-2 en の「set plays」が既存の訳の綴り（setplay / setplays）と揃っていない**
- 場所: `web/src/locales/en.json:168`
- 内容: `grep -oiE "set ?-?plays?" en.json` の結果: `setplay` 3・`setplays` 2・`Setplay` 1・`set plays` 1（今回の 1 件だけ 2 語）。
- なぜ問題か: 共有文は外へ出る文で、アプリ名（Tacpendium-Setplay）とも表記が揺れる。
- 直し方の案: `"{{character}} KA {{ka}} setplays"` に揃える（docs の例文は ja なので変更不要）。

### 低

**低-1 `navigator.share` に `title` と `text` へ同じ文を渡している**
- 場所: `web/src/components/Share.tsx:33`
- 内容: `{ title: text, text, url }`。推測: Android の一部の共有先（件名と本文をつなげて貼るメモ・チャット系）で同じ文が 2 回入る。iOS Safari は title をほぼ使わないので実害は小さい。text と url の重複（text に URL を含める形）は起きていない。
- 直し方の案: `title` を `APP_NAME` にするか省く。App.test の期待値も合わせる。

**低-2 キャッシュから即座に結果が出ると「URL をコピーしました」が新しい条件の上に残る**
- 場所: `web/src/components/Share.tsx:23-28`、`App.tsx:171`（キャッシュ命中は `running` を経ずに `done`）
- 内容: 計算する検索では `running` の間に Share が外れて状態が消えるが、キャッシュ命中では Share が付いたまま `url` だけ変わり、最大 2 秒「コピーしました」が残る（まだコピーしていない新しい URL について）。
- 直し方の案: `<Share key={url} … />` にする。

**低-3 テストの不足（0 件の結果・キャッシュ命中）**
- 場所: `web/src/components/App.test.tsx:750-814`
- 内容: 「結果 0 件・負の KA では出さない」（`Results.tsx` の分岐位置に依存）と、キャッシュ命中で `url` が新しい条件になることのテストが無い。`{share}` を `items.length === 0` の分岐の外へ動かしても緑のまま（推測。壊しては試していない）。
- 直し方の案: 0 件の偽エンジン結果で `share` が無いことを 1 件足す。

**低-4 「X に投稿」と「URL をコピー」のまとまりにラベルが付いていない**
- 場所: `web/src/components/Share.tsx:52-60`
- 内容: 「この条件を共有:」は `span` で、2 つの操作とは結び付いていない。ボタン・リンク自体の名前は十分（`postXAria` は見える文字列を先頭に含み WCAG 2.5.3 を満たす）。コピー結果はボタンの文字を変えるだけだが、`Results` の `section aria-live="polite"` の中にあるので読み上げはされる（推測: 読み上げソフトで差がある）。
- 直し方の案: `div` に `role="group"` と `aria-labelledby`（span に id）。

**低-5 docs の根拠「PC は URL のコピーで足りる」は、PC でも `navigator.share` がある環境（macOS Safari、Windows/ChromeOS の Chrome・Edge）では当たらない**
- 場所: `docs/frontend.md:69`（「主にスマホ」）・`docs/frontend.md:71`
- 内容: コードは要求どおり「share があればそれだけ」で、これらの PC ではコピーと X のリンクは出ず OS の共有シートだけになる。docs は「主に」と書いており虚偽ではないが、読み手は PC では必ずコピーが出ると読みうる。E2E は share を消して「PC の形」に固定しているので、この差はテストに出ない。
- 直し方の案: 「share がある PC（macOS Safari・Windows の Chrome/Edge 等）でも OS の共有シートになる」と 1 文足す。挙動を変えるなら開発者に確認（要求 1 の文言どおりなので現状で可）。

**低-6 `.share`・`.share-x` の CSS が無い**
- 場所: `web/src/components/Share.tsx:52`・`64`、`web/src/styles.css`（`grep -n "\.share" styles.css` は 0 件）
- 内容: 見た目は `.inline` と既定のリンクの形。意図どおりならクラス名は不要、または意図があるなら CSS が抜けている。

**低-7 OGP の E2E は og:title と twitter:card だけ**
- 場所: `web/e2e/setplay.spec.ts:105-106`
- 内容: og:description・og:type は検査していない。消しても緑。重要度は低い。

### 観点ごとの補足（指摘なし）

- 共有 URL の正しさ: 指摘なし。`shareUrl` は起動時 `href` の `origin + pathname` に `conditionsToSearch(run.conditions)`。`main.tsx:40` の `replaceState` は `window.location.pathname` + 同じ `conditionsToSearch(c)`（`App.tsx:167`）でハッシュを落とすので、パスが `/` 以外でも・`#privacy` 付きで開いても一致する。自動計算時の安全飛びの補正（`App.tsx:287-291`）も補正後の `c` を `execute` に渡すので一致。中断は `canceled` で Share を消すので、URL と古い結果の食い違いは出ない。
- `navigator.share`: 指摘なし（低-1 以外）。クリックの中で同期に呼ぶのでユーザー操作の条件を満たす。`AbortError` は黙り、それ以外は表示。
- CSP/COOP: 指摘なし。CSP はナビゲーションを縛らず（`navigate-to` 無し、`form-action 'none'` はリンクに無関係）、COOP same-origin と `noopener` は両立。`web-share` は Permissions-Policy に無く既定の `self` で使える（docs `frontend.md:73` の記述と一致）。`Referrer-Policy` と `noreferrer` で X へリファラも渡らない。
- 禁止事項（AGENTS §7）: 指摘なし（秘密情報・公式データ・combomgr の混入は diff に無い）。
- テストが黙って減っていないか: 指摘なし。`git diff 9880c88 --stat -- '*.test.*' '*.spec.*'` は 4 ファイル +137 行、削除 0 行。
- 成果物の鮮度: `web/dist` を写しで作り直した `index.html`（資産のハッシュ名込み）が手元の `web/dist/index.html` と一致。ローカル版バイナリ（15:37 作成）は `x.com/intent/post`・`Share these conditions` を含む。`Share.tsx` の mtime（15:40）がバイナリより新しいが、`dist` が現行ソースと一致することから内容の変更ではないと推測（壊して戻した作業の跡）。完了報告の前にバイナリを作り直しておくと確実。
- 完了報告: `docs/release-checklist.md:75` が未作成の `docs/progress/feat-share-report.md` を参照している。報告を書けば解消。

## 4. 良かった点

- `App` の `location` が起動時の写しで古くなることを見抜き、`location.href` をそのまま使わず条件から組み直した（docs にも理由を残した）。E2E で `page.url()` と突き合わせ、配線まで押さえている。
- 純粋関数（`shareUrl`・`shareText`・`xIntentUrl`・`characterDisplayName`）に切り出し、`CharacterSelect` と名前の規則を共有した。
- X の URL のテストが「共有 URL の `&` がクエリを割らない」（キーが text・url の 2 つだけ）まで見ている。
- E2E でクリップボードを別オリジンのページから読み戻す工夫（アプリ側は `clipboard-read=()`）で、実際にコピーされた文字列を確かめている。
- `_headers`・プライバシーポリシー・依存に手を付けず、外部への送信を増やしていない。

## 5. 自分で回したコマンドと結果

出力はすべて `/tmp/claude-0/-home-user-tacpendium-setplay/d05e6106-f447-5445-95e3-837b7f5d5fd2/scratchpad/` へ書き、`EXIT=` と件数の行で判定した（ANSI を落としてから grep）。

`web/` で（作業ツリーのまま）:

```
pnpm typecheck                         → EXIT=0
pnpm test                              → Test Files  19 passed (19) / Tests  183 passed (183) / EXIT=0
PW_CHROMIUM_PATH=/opt/pw-browsers/chromium-1194/chrome-linux/chrome pnpm e2e
                                       → 29 passed (1.1m) / EXIT=0
  （うち ✓ [site] setplay.spec.ts:99 この条件を共有…、✓ [local] local.spec.ts:48 共有 URL の KA 100 超え…）
```

`pnpm build` はリポジトリの `dist` を書き換えないよう、`web/` の写し（`src` は `diff -r` で同一を確認、`node_modules`・`public` は元へのリンク）で回した:

```
pnpm build → 違反: 0 件 / EXIT=0。生成した dist/index.html は手元の web/dist/index.html と一致
```

壊して赤になるかの確認（同じ写しで 1 つずつ壊し、`vitest --run src/components/App.test.tsx src/lib/share.test.ts`、毎回元に戻す）:

| 壊し方 | 結果 |
|---|---|
| 壊さない（基準） | Tests 46 passed (46) / EXIT=0 |
| `AbortError` でも「共有できませんでした」を出す | 1 failed（スマホの形のテスト）/ EXIT=1 |
| `online &&` を外す（ローカル版でも出す） | 1 failed（ローカル版では出さない）/ EXIT=1 |
| X の URL を `?text=${text}&url=${url}`（符号化しない） | 2 failed（`xIntentUrl`・PC の形）/ EXIT=1 |
| 共有 URL を起動時の `location.href` にする | 2 failed（PC の形・スマホの形）/ EXIT=1 |
| `shareUrl` のパスを `/` に固定する | 1 failed（origin と path…）/ EXIT=1 |

E2E での「起動時の `location.href`」の赤は再現していない（実装者の申告のみ）。リポジトリの作業ツリーは最後まで変更なし（`git status --short` は本ファイルを書く前に空）。
