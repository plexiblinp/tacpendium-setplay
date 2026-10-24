# 単発作業: シェアの改善（色付きのボタン・文にモード・このサイトを共有）— 完了報告

- 開発者の要望: 2026-10-24（PR #8 のデプロイを見て）
- 開始コミット: `b941855`（main。PR #8 のマージ）。作業ブランチ `claude/awesome-turing-rrl0yj` を main へ早送り（fast-forward）してから始めた（force push はしていない）
- 前の作業の報告: `docs/progress/feat-share-report.md`

## 要望と対応

| 要望 | 対応 |
|---|---|
| 色付きのボタンで目立たせる | 共有メニューのボタン・「URL をコピー」を `button.primary`（アクセント色）。「X に投稿」は枠線付きのボタンの形（`a.button-like`。hover で塗る） |
| 1. 結果の共有に文（キャラ名・KA・モード・アプリ名）、選んでいる言語で | 「リュウ KA 43 の重ね — Tacpendium-Setplay」。`share.text` に `{{mode}}` を足し、`share.mode.{meaty,gap,filler}` を新設。安全飛びの検索では、モードの代わりに安全飛びの種類（`setplay.safeJumpKind.*`。例「真安全飛び」）。区切りは ` | ` から ` — ` に変えた（`SHARE_TEXT_SEPARATOR`） |
| Web Share は title / text / url に分ける | title = アプリ名、text = 共有の文、url = 共有の URL（前から分けていた形のまま。text と同じ文を title に渡すと、共有先によって 2 回入るため） |
| X は text と url に分ける（文字数に注意） | intent の `text` と `url` に分けて渡す（前から）。X の数え方（URL は 23、CJK は 2）で、5 言語・長いキャラ名（20 文字）・KA 100・全モードの文が 280 以内であることを単体試験で見る |
| URL のコピーは URL だけか、文付きか（提案） | **URL だけのままにした（提案）**。理由: ブラウザのアドレス欄にもそのまま貼れる。SNS・チャットに貼れば OGP のカードが出るので、文を足すと重なる。文付きが要る場面は、共有メニュー（スマホ）と X のリンクで足りる。文付きに変えるなら `Share.tsx` の `onCopy` の 1 行と文言 1 キー |
| 2. アプリを紹介する共有ボタン（フッター） | フッターの先頭に「このサイトを共有」。結果の有無にかかわらず出す。URL はサイトのトップ（`siteUrl`＝ページの origin と path。条件・ハッシュなし）、文は紹介の一言（`share.appText`）＋アプリ名。部品は結果の共有と同じ `Share`（testid だけ `app-share-*`） |
| 3. 変えないこと | OGP のカードはアプリ共通のまま。外部 JS なし、`_headers`・プライバシーポリシーは変更なし、`dangerouslySetInnerHTML` なし、ストレージなし |

### 判断（聞かずに決めたこと）
- **ローカル版では「このサイトを共有」も出さない。** 前の決定（結果の共有を出さない。URL が 127.0.0.1）と揃えた。
  - ローカル版だけサイトの URL（定数）を共有する案もあった。採らなかったのは、URL の決め方が版で分かれるうえ、ローカル版の利用者は PC で使っていてサイトの URL を自分で貼れるため。
- en の文は「Ryu KA 43 meaty setplays」のように名詞句にした（en には「の」に当たる語を入れない）。ko・zh は仮訳。
- 紹介の一言はガイドの冒頭（`guide.lead1`）を短くしたもの。「非公式」を必ず入れた（HANDOVER §8）。

## 検査（AGENTS §8-1。`web/` で。出力はファイルへ）

```
pnpm assets                                      EXIT=0
pnpm typecheck                                   EXIT=0
pnpm test (vitest --run)                         Test Files  19 passed (19) / Tests  218 passed (218) / EXIT=0
pnpm build                                       違反: 0 件 / EXIT=0
bash scripts/local/build.sh (リポジトリ直下)       EXIT=0
PW_CHROMIUM_PATH=/opt/pw-browsers/chromium-1194/chrome-linux/chrome pnpm e2e
                                                 29 passed (1.2m) / EXIT=0（project site と local）
```

- vitest は 209 → 218（+9）。
  - `share.test.ts`: `shareModeKey`・`siteUrl`・X の文字数の数え方の確かめ・全ロケールで 280 以内、の 4 件
  - App: 安全飛びと有利調整の文・ko と zh-Hans の文・フッターの共有（PC の形と共有メニューの形）・ローカル版で出さない・色付き、の 5 件
- 既存の試験の期待値（文の区切り・モード）は新しい文に合わせて直した。削除した試験は 0（`git diff b941855 -- web/src web/e2e` で、削除行の `it(`・`test(` は 0、追加は 9）。
- E2E: 既存の共有の試験に、フッターの共有（URL がトップ、X の url と文）と、文のモード（汚連携）の検査を足した。`local.spec.ts` に「フッターの共有も無い」を 1 行足した。

### わざと壊して赤になるのを確かめた（AGENTS §8-3。どれも `tsc --noEmit` は EXIT=0 の形。確認後に戻した）

| 壊し方 | 結果 |
|---|---|
| `shareModeKey` が安全飛びを見ない | 2 failed（単体・画面） |
| 文のモードを常に「重ね」にする | 2 failed |
| ローカル版でもフッターの共有を出す | 1 failed（ローカル版） |
| `siteUrl` が条件付きの URL を返す | 2 failed（単体・フッター） |
| コピーのボタンを `secondary` に戻す | 1 failed（色付き） |
| 区切りを ` | ` に戻す | 7 failed |
| ko の `share.appText` を消す | i18n の試験 2 failed（キー集合・差し込み） |
| en の `share.mode.meaty` を日本語にする | i18n の試験 1 failed（日本語の混入） |
| フッターに共有を渡さない（`share={online ? undefined : appShare}`）→ ビルド → E2E | **E2E 1 failed**（`footer` の `app-share-x` が見つからない）。戻してビルドし直し、1 passed |

### git diff --stat b941855（報告を除く）

```
 docs/frontend.md                | 18 ++++++---
 web/e2e/local.spec.ts           |  1 +
 web/e2e/setplay.spec.ts         | 13 ++++++-
 web/src/components/App.test.tsx | 82 ++++++++++++++++++++++++++++++++++++++++-
 web/src/components/App.tsx      | 18 +++++++--
 web/src/components/Footer.tsx   | 21 +++++++++--
 web/src/components/Share.tsx    | 29 +++++++++------
 web/src/lib/share.test.ts       | 46 ++++++++++++++++++++++-
 web/src/lib/share.ts            | 18 ++++++++-
 web/src/locales/en.json         | 11 +++++-
 web/src/locales/ja.json         | 11 +++++-
 web/src/locales/ko.json         | 11 +++++-
 web/src/locales/zh-Hans.json    | 11 +++++-
 web/src/locales/zh-Hant.json    | 11 +++++-
 web/src/styles.css              |  4 +-
 15 files changed, 263 insertions(+), 42 deletions(-)
```

### 古い記述の残り（AGENTS §8-8）
- 検索語: `| Tacpendium-Setplay`・`のセットプレイ | `・`KA n のセットプレイ`・`setplays |`・`モード・絞り込みは文に入れない`
- 走査範囲: `docs/frontend.md`・`docs/release-checklist.md`・`web/src`（`.md`・`.ts`・`.tsx`）と 5 言語の locales
- 結果: 共有の文としての残りは 0。
  - `document.title` の「Tacpendium-Setplay | セットプレイ自動提案」は画面の題で、共有の文ではないので対象外。
  - `share.test.ts` の `xIntentUrl` の試験にある ` | ` は、符号化を見るための任意の文なので残した。
- `docs/progress/` の過去の報告は、当時の記録なので直していない。

## 残課題
1. ko・zh の文言は仮訳。
2. iOS・Android の実機では確かめていない（共有メニューの形は画面の試験の偽物でだけ見ている）。
3. サブエージェントのレビューは回していない（この回の指示に無い）。

## 次への入力
- 共有の文を変えるときは `share.text`・`share.mode.*`・`share.appText`（5 言語。zh-Hant は `pnpm gen:locales`）と、X の文字数の試験（`share.test.ts`）を見る。
- URL のコピーを文付きにするなら `Share.tsx` の `onCopy` と、文言 `share.copyUrl`・`share.copied`。
