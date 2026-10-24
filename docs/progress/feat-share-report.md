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

### git diff --stat 9880c88（報告と review を除く。`-- . ':!docs/progress'`）

```
 docs/frontend.md                       |  17 ++++-
 docs/release-checklist.md              |   1 +
 web/e2e/local.spec.ts                  |   2 +
 web/e2e/setplay.spec.ts                |  31 ++++++++++
 web/index.html                         |   5 ++
 web/src/components/App.test.tsx        | 110 +++++++++++++++++++++++++++++++++
 web/src/components/App.tsx             |  16 +++++
 web/src/components/CharacterSelect.tsx |   3 +-
 web/src/components/Results.tsx         |   9 ++-
 web/src/components/Share.tsx           |  89 ++++++++++++++++++++++++++
 web/src/lib/share.test.ts              |  40 ++++++++++++
 web/src/lib/share.ts                   |  32 ++++++++++
 web/src/locales/en.json                |  10 +++
 web/src/locales/ja.json                |  10 +++
 web/src/locales/ko.json                |  10 +++
 web/src/locales/zh-Hans.json           |  10 +++
 web/src/locales/zh-Hant.json           |  10 +++
 web/src/styles.css                     |   2 +
 18 files changed, 401 insertions(+), 6 deletions(-)
```

試験の行の削除は 0（`git diff 9880c88 -- web/src web/e2e` に `-  it(`・`-  test(` は無い）。

## 残課題・既知の問題

1. ~~OGP の `og:url`・`og:image`~~ → 2026-10-24 に済（下の「S9 からの連絡の取り込み」）。
2. `og:title`・`og:description` は日本語の固定文。言語ごとのカードは、配信側で書き換えないと出せない。
3. `navigator.share` がある PC（macOS の Safari、Windows の Chrome・Edge 等）では、X のリンクとコピーのボタンは出ず、OS の共有メニューだけになる（要求どおり。docs に書いた）。
4. iOS・Android の実機では確かめていない（E2E は Linux の Chromium で、共有メニューが無い形に固定した）。スマホの形は App.test（偽の `navigator.share`）でだけ見ている。
5. ko・zh の文言は仮訳。

## 次への入力

- S9 のセッションと触りうるファイル: `web/index.html`（head に OGP の 5 行）、`web/src/locales/*.json`（`share` の節を `setplay` の後に足した）、`docs/release-checklist.md` §4（表の末尾に 1 行）、`docs/frontend.md`。取り込みでコンフリクトが出ても、どれも追加だけなので両方を残せる。
- サイトの URL が決まったら、OGP の残りをやる（残課題 1）。共有の機能自体は、URL が決まらなくても動く（共有 URL はページの origin から組む）。
- PR はまだ作っていない（開発者が頼んだときだけ作る）。

## S9 からの連絡の取り込み（2026-10-24）

S9（Web 公開）の側からの連絡（サイトの URL・main の取り込み・CORP の確認・PR）を受けて、次をやった。

### main の取り込み
- `git merge origin/main`（08fa3ed。PR #5・#6・#7）→ マージコミット `9e1166f`。
- コンフリクトは `docs/release-checklist.md` §4 の表の末尾 1 か所だけ。両方の行を残した（main の `wrangler.toml` の「済」と `deploy` job の行、こちらの `web/index.html` の行）。
- locales（`privacy.hashBody` の 1 行）は自動で合わさった。`pnpm gen:locales` を回しても zh-Hant に差分が出ないことを確かめた。

### OGP の残り
- `web/index.html` に足したタグ:
  - `og:url` = `https://tacpendium-setplay.plexiblinp.workers.dev/`
  - `og:image` = `https://tacpendium-setplay.plexiblinp.workers.dev/og.png`
  - `og:image:width`・`og:image:height`・`og:image:alt`
  - `twitter:card` は `summary_large_image` に変えた
- 画像 `web/public/og.png`（1200×630、PNG、39,276 bytes）。`web/scripts/og-image.mjs` が、自作の HTML（favicon と同じ形・画面の色・英語の文字だけ）を Playwright の Chromium で撮って作る（依存は増やしていない）。
  - ゲームのロゴ・画像は使っていない。
  - 英語だけにしたのは、この環境に日本語のフォントが無く、字形が環境で変わるため。
  - ライセンスは REUSE の既定（層 A。favicon.svg と同じ）。`check-licenses.py` は違反 0。
- `web/scripts/serve.mjs`（E2E の静的サーバ）の Content-Type の表に `.png` を足した。本番は Cloudflare が付けるので関係ない。ローカル版の Go のサーバ（`internal/localserver`）は `.png` を `application/octet-stream` で返すが、ローカル版で og.png を読む場面は無いので、Go は触っていない。
- E2E（`setplay.spec.ts` の共有の試験）で次を確かめる:
  - `og:url` の値
  - `og:image` が `og:url` と同じ host であること
  - その path をこの配信が 200・`image/png` で返すこと
- わざと壊した確認: `og:image` を `/og-missing.png` にすると E2E が赤になった（Expected 200 / Received 404。1 failed、EXIT=1）。戻して緑。

### CORP（`Cross-Origin-Resource-Policy: same-origin`）の確認
本番（`tacpendium-setplay.plexiblinp.workers.dev`）には、この環境のネットワークの制限で繋がらなかった（プロキシが 403）。そのため、`_headers` どおりにヘッダを付ける手元の配信（`scripts/serve.mjs`）で確かめた。

| 取り方 | 結果 |
|---|---|
| ブラウザの外から取得（Node の fetch。UA は Twitterbot/1.0。SNS のサーバが取りに来る形） | **200・`image/png`・39,276 bytes**。CORP は付いているが効かない |
| ブラウザで、CSP も CORP も無い別オリジンのページから `<img>` で読む（CORP なしの配信） | load（幅 1200） |
| 同じページから `<img>` で読む（CORP `same-origin` の配信） | **error**（読めない） |

- 最初の試し方（Playwright の route で作ったページから読む形）は、CORP の有無にかかわらず両方とも timeout になり、何も測れていなかった。CORP なしの対照を置いて、試し方を作り直した。
- 結論:
  - X・Discord・Slack・LINE・Facebook 等は、カードの画像を自分のサーバで取って自分の CDN から出す（推測。各社の一般的な作り。本番への実際の取得は確かめていない）。そのため CORP は効かない見込み。
  - 一方で、**og:image をブラウザの中で直接読む作りの読み手（プロキシを通さない一部の SNS のクライアント等）では、画像が出ない**。
  - 直すには `_headers` に `/og.png` だけ CORP を `cross-origin` にする規則を足す必要がある。Cloudflare の `_headers` は、同じヘッダを 2 つの規則で付けると値をカンマでつなぐ。そのため `! Cross-Origin-Resource-Policy` で外してから付け直す形になる。
  - **`_headers` は変えない約束なので、変えていない（開発者に聞く）。**
- デプロイ後に開発者が確かめること: X の投稿欄にサイトの URL を貼り、カードに画像が出るか（`docs/release-checklist.md` §4）。

### 検査（この回。`web/` で。出力はファイルへ）

```
pnpm assets                                      EXIT=0
pnpm typecheck                                   EXIT=0
pnpm test (vitest --run)                         Test Files  19 passed (19) / Tests  209 passed (209) / EXIT=0
pnpm build                                       違反: 0 件 / EXIT=0（dist/og.png 39,276 bytes）
bash scripts/local/build.sh                      EXIT=0
pnpm e2e (PW_CHROMIUM_PATH=…chromium-1194…)      29 passed (1.2m) / EXIT=0
python3 scripts/check-licenses.py                違反: 0 件 / EXIT=0
python3 scripts/check-private-refs.py            違反: 0 件 / EXIT=0
```

vitest の 186 → 209 は main 側（S9）で増えた試験。この回で足した vitest の試験は無い。E2E は既存の試験に検査を足しただけで、件数は 29 のまま。

この回のレビュー: サブエージェントのレビューは回していない（指示に無い。差分は OGP のタグ・画像・E2E の数行）。

### この回の残課題
1. `og.png` の CORP（上）。**開発者決定 2026-10-24: 今は `_headers` を変えない。** デプロイ後に X のカードを見て、画像が出なければ `/og.png` だけ CORP を `cross-origin` にする規則を足す。
2. 本番での OGP の見え方（X 等のカード）は、デプロイ後に開発者が確かめる。
3. `og:title`・`og:description` は日本語の固定文、画像の文字は英語だけ。
