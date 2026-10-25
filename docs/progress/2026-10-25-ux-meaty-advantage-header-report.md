# ux: 重ねたときの有利の表示・ヘッダの中央寄せ・サイト名でリセット（2026-10-25）

公開後の最初の `ux` の作業（AGENTS §3「公開後の進め方」）。開発者の友人のフィードバックと、開発者の追加の依頼。
開始コミット `7406536`（作業ブランチ `claude/s9-web-deployment-t0w876`。S9 の締めの文書のコミットの上）。

## やったこと

### 1. 重ねたときの有利（友人の要望「打撃重ねのレシピのときに有利 F を出してほしい」）

- 重ねる提案（`mode=meaty`）の行の「持続 N F目ヒット」の後ろに `· ガード ±0 / ヒット +6` を出す。コピーの文にも入る。
- 計算は `web/src/lib/advantage.ts`: moves DB の `onBlock`・`onHit`（持続 1F 目を当てたときの値）＋（N − 1）。表示は符号つき、0 は `±0`、値が無い側は `—`。
- 出さない行: 重ねない提案（`gap`・`filler`）、当てる技が投げ、`onBlock`・`onHit` が両方 null。
- 「注意と対策」の先頭に 1 項（`guide.advantage`／`guide.advantageFix`）: 計算上の値で、多段技・持続のどこで当てても有利が変わらない技・飛び道具の距離による違いは考えない → ゲーム内で確かめる。
- moves DB はもともと `onHit`・`onBlock` を配っていて、画面の schema（`moveSchema`）が読んでいなかっただけ。**エンジン・データ・票の鍵・事前計算は変えていない**。
- 文言は 5 言語（ja・en・ko・zh-Hans は手で、zh-Hant は `pnpm gen:locales`）。

### 2. ヘッダ

- 中身（サイト名・設定）を `.page` と同じ幅で中央に置いた（`.site-header-inner`。背景と下線は画面の幅のまま）。
- サイト名をリンクにし、押すと「条件をリセット」と同じ処理（計算中なら止める・条件を初期化・結果を消す・URL のクエリを消す）。修飾キー付き・中クリックは普通のリンクとして初期の画面（`./`）を開く。

## 判断と理由（開発者決定 2026-10-25 を含む）

- **飛び道具にも出す**（開発者決定）: 空振りしていない＝重なったときの値が出ればよい。
- **多段技などは正確な値にしない**（開発者決定）: 行に「目安」と付けず、「注意と対策」の先頭に書く。持続のどこで当てても有利が変わらない技も、計算上の値のまま。
- **投げは出さない**（Claude の判断）: 依頼は打撃の重ね。投げの `onBlock` は全キャラで値が無く（`data/moves/*.json` の category=throw 79 件すべて null）、`onHit` は投げの後の有利で、重ねの深さで変わる値ではない。
- **ラッシュ版**: moves DB の値がすでにラッシュの +4 込み（リュウの立ち弱P 4/−1 → ラッシュ 8/+3 で確かめた）なので、同じ式。
- **ヘッダの「中央寄せ」の解釈**（推測）: 文字を中央揃えにするのではなく、中身の塊を本体と同じ中央の列に置き、本体と左端を揃えた。本体が左揃えの列なので、こちらのほうが揃って見える。文字の中央揃えがよければ、`.site-header-inner` に `text-align: center` と、設定の行に `justify-content: center` を足すだけで済む。
- **サイト名はリンク**（`<a href="./">`）: ボタンにしないのは、サイト名＝トップへのリンクという慣習と、新しいタブで開けるため。

## 検証（手元。Node v22.23.4・pnpm 10.34.7。出力はファイルへ書き、終了コードと件数で判定）

```
$ pnpm assets                                  → EXIT=0
$ pnpm exec tsc --noEmit -p .                  → EXIT=0
$ pnpm exec vitest --run                       → EXIT=0  Test Files 20 passed (20) / Tests 225 passed (225)
$ pnpm build                                   → EXIT=0  合計: 6717736 bytes / 違反: 0 件
$ PW_CHROMIUM_PATH=/opt/pw-browsers/chromium-1194/chrome-linux/chrome pnpm exec playwright test   → EXIT=0  31 passed  (retries: 0)
$ python3 -I scripts/check-private-refs.py     → EXIT=0 違反: 0 件
```

作業の前は vitest 218・E2E 29。増えた試験: `advantage.test.ts`（6 件）、`App.test.tsx`（有利の表示 1 件。コピーの期待値は有利入りへ変えた）、E2E 2 件（重ねたときの有利の通し・ヘッダ）と、既存の E2E に「重ねない提案には有利が出ない」の 1 行。

わざと壊して赤を確かめた（AGENTS §8-3。型とビルドが通る壊し方）:

- `advantage.ts` の `n - 1` を `n` に → vitest 5 件失敗。投げの除外を外す → 1 件失敗。
- `Results.tsx` の有利の表示の条件を反転、`.site-header-inner` の幅の指定を消す → ビルドは EXIT=0、新しい E2E 2 件とも失敗。
- 注: サイト名の `reset()` を消しても E2E は緑のまま（リンクの `./` へ移ると、読み込み直しで同じ初期の画面になるため）。利用者に見える結果は同じなので、試験は結果（URL・キャラ・KA・行が空）だけを見る。
- 途中で 1 度、ビルドが型の誤り（`App` の `location` は注入された `{ href, search }` で `pathname` が無い）で止まり、古い `dist` のまま E2E が回って赤になった。`href="./"` に直した。

E2E の「重ねたときの有利」は、配信している manifest → moves DB から当てる技の値を読み、画面の行の値と突き合わせる（実データ・実 WASM の通し）。

### `git diff --stat 7406536`（この報告を除く）

```
 docs/frontend.md                | 12 ++++++++-
 web/e2e/setplay.spec.ts         | 48 +++++++++++++++++++++++++++++++++++
 web/src/components/App.test.tsx | 17 +++++++++++--
 web/src/components/App.tsx      | 44 ++++++++++++++++++++++-----------
 web/src/components/Guide.tsx    |  1 +
 web/src/components/Results.tsx  | 22 +++++++++++++++--
 web/src/lib/advantage.test.ts   | 55 +++++++++++++++++++++++++++++++++++++++++
 web/src/lib/advantage.ts        | 37 +++++++++++++++++++++++++++
 web/src/lib/data.ts             |  3 +++
 web/src/lib/notation.test.ts    |  2 ++
 web/src/locales/en.json         |  3 +++
 web/src/locales/ja.json         |  3 +++
 web/src/locales/ko.json         |  3 +++
 web/src/locales/zh-Hans.json    |  3 +++
 web/src/locales/zh-Hant.json    |  3 +++
 web/src/styles.css              |  4 +++
 16 files changed, 241 insertions(+), 19 deletions(-)
```

試験のファイルは追加・増加のみ（`notation.test.ts`・`App.test.tsx` は fixture に `onHit`・`onBlock` を足した）。

## 残課題・次への入力

- 配信物が変わるので、マージの後に「Cloudflare へ出す」の承認が要る。
- ローカル版も画面を埋め込むので、次のローカル版のタグで反映される（急ぎではない。データの更新のときにまとめてよい）。
- 有利の値は計算上の値（上の「判断と理由」）。実際と違う技の報告が来たら、技ごとの例外をデータに持つかを開発者と決める（いまは持たない）。
