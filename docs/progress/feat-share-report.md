# 単発作業: シェアボタン（この条件を共有）— 完了報告

- 開発者の決定: 2026-10-20（HANDOVER D1 の機能範囲の外の追加機能。段階 S<n> ではない）
- 開始コミット: `9880c88`（main の最新と同じ）
- ブランチ: `claude/awesome-turing-rrl0yj`
- レビュー: `docs/progress/feat-share-review.md`（新しい general-purpose のサブエージェント。2 往復）

## 開発者に聞いて決めたこと（2026-10-20）

| 問い | 決定 |
|---|---|
| X 以外の SNS（Bluesky・LINE 等）のリンク | **足さない（X のみ）**。共有メニューがある端末はそこから、無い端末は URL のコピーで足りる |
| OGP の `og:url`・`og:image` | **後回し**。サイトの URL が決まったら、画像（自作 1 枚）と一緒に足す。`docs/release-checklist.md` §4 に 1 行足した |
| ローカル版での扱い | **出さない**（127.0.0.1 の URL は相手が開けない） |

## やったこと

1. **共有の操作**: `web/src/components/Share.tsx`（新規）。結果が 1 件以上あるとき、件数の行の下に出す（`Results` の `share` に App から渡す）。
   - `navigator.share` があるとき: ボタン「この条件を共有」で端末の共有メニューを開く（`title`=アプリ名、`text`=共有の文、`url`）。閉じただけ（`AbortError`）なら何も出さない。ほかの失敗では「共有できませんでした」を出す。
   - 無いとき: 「この条件を共有:」のラベル、「URL をコピー」（結果を 2 秒出す）、「X に投稿」（`https://x.com/intent/post?text=…&url=…`。`target="_blank" rel="noopener noreferrer"`）。
   - `role="group"` と `aria-labelledby` で、まとまりに名前を付けた（コロンは読ませない）。`key` に共有 URL を渡し、条件が変わったら作り直す（前の条件の「コピーしました」を残さない）。
2. **純粋な関数**: `web/src/lib/share.ts`（新規）
   - `shareUrl`（origin＋path＋`conditionsToSearch`）
   - `shareText`（「… | Tacpendium-Setplay」）
   - `xIntentUrl`（`URLSearchParams`）
   - `characterDisplayName`（ja だけ日本語名。`CharacterSelect` の同じ規則もこれを使うように変えた）
3. **OGP**: `web/index.html` に `og:type=website`・`og:title`・`og:description`（日本語の固定文）・`twitter:card=summary`。
4. **i18n**: `share.*` の 8 キーを ja・en・ko・zh-Hans に足し、zh-Hant は `pnpm gen:locales` で生成した。
5. **docs**: `docs/frontend.md` に「この条件を共有」の節を足し、検査の表を直した。`docs/release-checklist.md` §4 に OGP の残りを書いた。

`_headers`・プライバシーポリシー・依存（`package.json`・lockfile）・Go は変えていない（下の diff --stat）。

## 判断と理由

- **共有 URL は `location.href` をそのまま使わず、`shareUrl(location.href, 結果の条件)` で組んだ。** App の `location` prop は起動時の写し（`main.tsx`）で、`replaceState` の後も古いまま。組んだ URL は、その時点の `location.href` と同じ文字列になる。E2E の `data-share-url === page.url()` と、X の `url` パラメータ `=== page.url()` で確かめた。
  - 検討した案: クリックの時に `window.location.href` を読む。→ App の試験で注入できず、URL を書き換えた後の順序にも依存するので採らなかった。
- **置き場所は件数の行の下**（結果が 1 件以上のときだけ）。0 件・負の KA・誤り・計算中・中断では出さない。0 件の条件を共有する意味は薄い。
- **共有の文にはキャラと KA だけを入れた**（モード・絞り込みは URL に入る）。文を短く保つため。
- `navigator.share` の `title` はアプリ名にした。`text` と同じ文を渡すと、共有先によっては 2 回入る（レビュー低-1。推測）。
- **en の文は「setplays」**（既存の訳の綴りに揃えた）。**zh-Hans は「分享到 X」「在新窗口中打开」**。最初は「发布到 X」「标签页」にしていたが、OpenCC がそれぞれ「釋出」（ソフトの「リリース」の意味）・「標籤頁」（台湾では「分頁」）に変えて不自然になったため（レビュー中-1）。ko・zh は仮訳。
- **`_headers` は変えずに済む。**
  - Permissions-Policy に `web-share` は無く、既定の allowlist（self）で使える。
  - X へは、利用者が押したリンクで移るだけ。CSP の `connect-src`・`script-src` に掛からない。
  - COOP の same-origin と `noopener` は両立する。
  - E2E（実際の `_headers` で配信）で、CSP 違反・コンソールのエラーが 0 だった。
- **OGP は文字だけの `summary` にした。** 条件ごとのカード（キャラ名入り）は、配信側（Worker）で HTML を書き換えないと作れない。範囲の外なので作っていない。

## 検査（AGENTS §8-1。出力はファイルへ書き、`EXIT=` と件数で判定）

`web/` で、最後のコミット前に回した結果:

```
pnpm typecheck                                   EXIT=0
pnpm test (vitest --run)                         Test Files  19 passed (19) / Tests  186 passed (186) / EXIT=0
pnpm build                                       違反: 0 件 / EXIT=0
bash scripts/local/build.sh (リポジトリ直下)       EXIT=0
PW_CHROMIUM_PATH=/opt/pw-browsers/chromium-1194/chrome-linux/chrome pnpm e2e
                                                 29 passed (1.1m) / EXIT=0（project site と local）
python3 scripts/check-licenses.py                違反: 0 件 / EXIT=0
python3 scripts/check-private-refs.py            走査: 295 ファイル / 違反: 0 件 / EXIT=0
```

- 試験の件数は、vitest が 174 → 186（+12）、E2E が 28 → 29（+1）。
  - 単体 `share.test.ts`: 5 件
  - App: 7 件（PC の形・スマホの形・条件入りの URL・ローカル版では出さない・0 件では出さない・group の名前・条件を変えたときのコピー表示）
  - E2E: `setplay.spec.ts` に 1 件（結果を出す → 共有の操作が出る → URL・X のリンク・クリップボードに条件入りの URL・OGP の 4 タグ）。`local.spec.ts` の既存の試験に「共有の操作が無い」の 1 行を足した。

### わざと壊して赤になるのを確かめた（AGENTS §8-3。どれも `tsc --noEmit` は EXIT=0 の形で壊し、確認後に戻した）

| 壊し方 | 結果 |
|---|---|
| X の URL を符号化しない（`?text=${text}&url=${url}`） | share.test・App.test が赤（2 failed） |
| ローカル版でも出す（`online &&` を外す） | App.test「ローカル版では出さない」が赤 |
| 共有 URL を起動時の `location.href` にする（`location.href \|\| shareUrl(...)`） | App.test 2 failed。**E2E も赤**（Expected `…/?c=ryu&ka=40&mode=gap`、Received `…/`） |
| `AbortError` でも失敗を出す | App.test「スマホの形」が赤 |
| `key={sharedUrl}` を消す | App.test「条件を変えたとき」が赤。**最初は部分一致（`toHaveTextContent("URL をコピー")` が「URL をコピーしました」にも一致）のせいで緑のままだったので、完全一致に直してから赤を確かめた** |
| `role="group"`・`aria-labelledby` を消す／名前にコロンを戻す | App.test「group の名前」が赤 |

レビュー側でも、ほかに「パスを `/` に固定する」「0 件でも出す」の 2 通りで赤になることを確かめた（`feat-share-review.md`）。

`local.spec.ts` に足した 1 行は、壊して赤になるのを確かめていない（ローカル版のバイナリを壊した形で作り直す必要があるため）。同じ性質は App.test の「ローカル版では出さない」で赤になるのを確かめてある。

### git diff --stat 9880c88（報告と review を除く）

```
 docs/frontend.md                       | 17 ++++++-
 docs/release-checklist.md              |  1 +
 web/e2e/local.spec.ts                  |  2 +
 web/e2e/setplay.spec.ts                | 31 ++++++++++++
 web/index.html                         |  5 ++
 web/src/components/App.test.tsx        | 113 +++++++++++++++
 web/src/components/App.tsx             | 15 +++++
 web/src/components/CharacterSelect.tsx |  3 +-
 web/src/components/Results.tsx         |  9 ++-
 web/src/components/Share.tsx           | 88 ++++++++++++
 web/src/lib/share.test.ts              | 40 ++++++
 web/src/lib/share.ts                   | 32 ++++++
 web/src/locales/*.json (5 言語)         | 各 10 行の追加
 web/src/styles.css                     |  2 +
```

（正確な数はコミット後に `git diff --stat 9880c88` で取れる。上は要約。）試験の行の削除は 0（`git diff 9880c88 -- web/src web/e2e` に `-  it(`・`-  test(` は無い）。

## 残課題・既知の問題

1. **OGP の `og:url`・`og:image`**: サイトの URL が決まったら足す（`docs/release-checklist.md` §4）。画像は自作の 1200×630 を 1 枚にして、`twitter:card` を `summary_large_image` に変える。それまではカードが文字だけになる。
2. `og:title`・`og:description` は日本語の固定文。言語ごとのカードは、配信側で書き換えないと出せない。
3. `navigator.share` がある PC（macOS の Safari、Windows の Chrome・Edge 等）では、X のリンクとコピーのボタンは出ず、OS の共有メニューだけになる（要求どおり。docs に書いた）。
4. iOS・Android の実機では確かめていない（E2E は Linux の Chromium で、共有メニューが無い形に固定した）。スマホの形は App.test（偽の `navigator.share`）でだけ見ている。
5. ko・zh の文言は仮訳。

## 次への入力

- S9 のセッションと触りうるファイル: `web/index.html`（head に OGP の 5 行）、`web/src/locales/*.json`（`share` の節を `setplay` の後に足した）、`docs/release-checklist.md` §4（表の末尾に 1 行）、`docs/frontend.md`。取り込みでコンフリクトが出ても、どれも追加だけなので両方を残せる。
- サイトの URL が決まったら、OGP の残りをやる（残課題 1）。共有の機能自体は、URL が決まらなくても動く（共有 URL はページの origin から組む）。
- PR はまだ作っていない（開発者が頼んだときだけ作る）。
