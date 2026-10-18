# S7 公開前監査 — レビュー

- 対象: ブランチ `claude/relaxed-euler-t2z8gs`、基準 `52af7a19cd599460d5483278a8e7df6b3a08a285` → HEAD `e591341`（13 コミット）
- 読んだもの: `AGENTS.md`（§4・§8・§9）、HANDOVER §9 の S7 の行と §13、`docs/progress/S7-report.md`（「レビュー前」の版）、`git diff 52af7a19`（`_reference/` 以外は全部）、`docs/release-checklist.md`、`docs/progress/S7-privacy-research.md`（拾い読み）
- 読み取りのみ。書いたのは本ファイルだけ。一時出力は `/tmp/claude-0/review/`

## 1. 要約

- 計画したリポジトリ内の作業（`_reference/` の削除、ヘッダ、依存、CI の整理、公開してはいけない語の検査、画面 A 案、新キャラの方針、公開の手順書）は入っている。手元で回した Go・vitest・ビルド・E2E・リポジトリ検査はすべて EXIT=0。
- CI は 6 job を 2 job にまとめたが、前の検査は全部残っている。検査済みの記録（cache）は `success()` のときだけ保存され、PR での本家の正解の判定の基準も正しく動く（§3「良かった点」）。
- **高 1 件**: 完了報告に検査結果の逐語（`EXIT=`・件数）と `git diff --stat` が無い（「レビュー前」の版なので、取り込みのときに貼れば足りる）。
- **中 3 件**: 「1 つの技を指定」で技を選ばないまま押すと、見えていない種類の条件で計算される ／ 公開してはいけない語の検査が、拡張子やパスの付いた語を見逃す ／ HANDOVER §2.1 に `_reference/` を置いているという記述が残っている。
- 完了条件「外部ヘッダ診断で問題なし」はデプロイ待ちで未。報告はこれを逸脱として正しく書いている。

## 2. 完了条件への適合

| 項目（HANDOVER §9 S7・報告の計画） | 判定 | 根拠 |
|---|---|---|
| ヘッダ/CSP | 済（手元） | `_headers` に CORP・XFO・HSTS を追加。`/api` には `API_SECURITY_HEADERS`。E2E `headers.spec.ts` の 2 本が通った（下の §5） |
| 依存 | 済 | `go.mod` の toolchain を go1.26.9 に、dependabot に gomod を追加。govulncheck・pnpm audit の結果は報告の表にある（自分では回していない） |
| Actions 固定 | 済 | `check-actions-pinned.sh` で違反 0（workflow 3 本・uses 14 件） |
| `_reference/` 削除 | 済（docs が 1 か所古い＝中-3） | `git ls-files` の中に `_reference` は無い。REUSE・LicenseRef・EXPECT・hook も外した。HANDOVER §2.1 に古い記述が残る |
| private 由来の記述の除去 | 済（開発者決定で範囲を縮めた） | `check-private-refs.py` で違反 0（走査 270 ファイル）。基準コミットの木で、`_reference` 以外に 2 件見つかることを自分で確かめた（報告の「2 件」と一致）。照合の穴は中-2 |
| 外部ヘッダ診断で問題なし | 未（デプロイ後） | 報告の「逸脱」に書いてある。手順と期待値は `release-checklist.md` §3 にある |
| §13 チェックリスト完 | 一部 | `release-checklist.md` §1 に全 10 項目がある。開発者の作業とデプロイ後の項目が残る。項目 3 の書き方は低-3 |
| public 化 | 開発者（S9） | 開発者決定（1 コミットにまとめて公開）。手順は `release-checklist.md` §2 |
| 画面 Q4（重ねる技・検索・リセット・新キャラ） | 済（不具合 1 件＝中-1） | 5 言語のキー集合は一致（各 158 キー）。vitest と E2E が通った |
| 完了報告に検査の逐語 | **未**（高-1） | |

## 3. 指摘

### 高

**高-1 完了報告に検査の逐語と `git diff --stat` が無い**
- 場所: `docs/progress/S7-report.md`（全体。`EXIT` という語は 47 行目の計画の見出しにしか出てこない）
- 内容: 「やったこと」の表には「違反 0」「赤を確かめた」などの言葉はあるが、`go test`・`vitest`・`playwright`・`tsc`・`go vet`・`gofmt` などの `EXIT=` と pass/fail の件数の逐語も、`git diff --stat 52af7a1` も貼っていない。
- なぜ問題か: AGENTS §3「完了報告」・§8-1・§8-11 の必須項目で、証拠の無い「緑」になる。報告は「レビュー前」の版だが、このまま完了にはできない。
- 直し方: 取り込みの後に全検査を回し直し、§5 の形で逐語を貼る。`git diff --stat 52af7a19`（`_reference/` 141 ファイルの削除が大半を占めるので、`-- . ':(exclude)_reference/**'` の stat も並べて貼る）と、試験の数が減っていないこと（下の §5 の数え方）も貼る。

### 中

**中-1 「1 つの技を指定」で技を選ばないまま押すと、見えていない種類の条件で計算される**
- 場所: `web/src/components/ConditionsPanel.tsx:90-95`（`showMove`／`chooseByMove`）、`:410-414`（「提案を出す」の `disabled`）、`web/src/lib/formState.ts:114`
- 内容: 「1 つの技を指定」を押すと種類のチェックボックスは隠れるが、`form.targetTypes` は前のまま残る。技の選択が空（`targetMove == null`）でも `validateForm` は通るので、「提案を出す」は押せる。押すと、画面には出ていない種類（既定の 3 種類）で計算される。逆に、種類を 0 個にしてから切り替えると、隠れた欄の誤り「重ねる技の種類を 1 つ以上選んでください」が出てボタンが押せず、直す欄が画面に無い。
- なぜ問題か: 利用者が選んだ「選び方」と、実際に計算する条件が違う。S5 で直した本家の不具合（押した条件と違う結果が出る＝`docs/frontend.md`「本家の画面から変えたところ」）と同じ種類の問題。試験は「技を選んでから押す」と「種類に戻す」しか見ていない。
- 直し方: 選び方が「1 つの技を指定」（`showMove`）で `targetMove == null` のときは「提案を出す」を押せないようにし、「技を選んでください」と出す（5 言語）。種類 0 個の誤りは、種類で選んでいるときだけ出す。vitest を 1 本足し、§8-3 のとおり壊して赤になることを確かめる。

**中-2 公開してはいけない語の検査が、拡張子・パス・`_` の付いた形を見逃す**
- 場所: `scripts/check-private-refs.py:36`（`WORD_RE`）・`:49`（`.strip(".-")`）
- 内容: 語は `[A-Za-z0-9_.-]+` でひとかたまりに切り出し、端の `.` と `-` だけを落としてから、全体のハッシュで照合している。そのため、禁止語の後ろに `.md` が付いた形（例 `` `<語>.md` ``、`docs/<語>.md`）や、`<語>_v2` は 1 つの別の語になって当たらない。DENY のうち実在の語のハッシュで確かめた（語そのものはここに書かない）。素の語 → 1 件、大文字 → 1 件、`` `<語>.md` `` → 0 件、`docs/<語>.md` → 0 件、`<語>_v2` → 0 件、`-` を `_` に替えた形 → 0 件。
- なぜ問題か: 伏せた語は文書の名前なので、いちばん書かれやすいのはファイル名（拡張子付き）の形。今の木には無い（違反 0）が、検査は「今後に入るのを止める」ためにある。試験（`check_private_refs_test.py`）も末尾の `.` しか見ていない。
- 直し方: 切り出した語を `.`／`_`／`/` でもう一度分け、元の語と分けた後の連なりの両方を照合する（例: 区切りで分けた部分のすべての連続部分列）。あるいは、素の語に加えて拡張子付きの形もハッシュにして DENY に入れる。試験に `secret-note.md`・`docs/secret-note.md` を足し、今のコードで赤になることを確かめる。

**中-3 HANDOVER §2.1 に「`_reference/combomgr/` にスナップショットを置く」が残っている**
- 場所: `docs/handover/HANDOVER-001-setplay-standalone.md:45`（見出し「本リポジトリでは `_reference/combomgr/` にスナップショットを置く。§10」）、`:375`（「`_reference/` は編集しない」）
- 内容: §10 の木（352 行目）には「S7 で削除」と書き足してあるが、§2.1 の見出しと 375 行目は今もそこにあるかのような書き方のまま。
- なぜ問題か: HANDOVER は正本で、S8 以降も §2 を読む。消したディレクトリを指す記述はレビューの依頼でも「古い参照」として名前が挙がっている（AGENTS §8-8）。
- 直し方: 2 か所に「（S7 で削除。移植元は公開版 tacpendium＝`.upstream/tacpendium`）」を添える。AGENTS §1 の「HANDOVER を覆さない」は D1〜D15 についての決まりなので、この注記は書いてよい。

### 低

- **低-1** `docs/telemetry.md:60` が「S7 報告『開発者に決めてほしいこと』」を指しているが、S7 報告にその見出しは無い（中身は「残課題・既知の問題」の 2 つ目）。見出しの名前を合わせる。
- **低-2** `scripts/check-private-refs.py:23` のコメントは「本家のコミット SHA 3 件」だが、報告（判断の 1 つ目）は「公開されていないのは本家の SHA 2 つとメモ名 1 つだけ」。基準の木で `_reference/` の中に当たったのが 1 件あったので、3 つ目はそれだと思われる（推測）。どちらかに「3 つ目は `_reference/` の中だけで、履歴に残る」と書けば、食い違いには見えなくなる。
- **低-3** `docs/release-checklist.md` §1 項目 3 は「済」だが、HANDOVER §13 の grep の語のうち `autopilot-combomgr` は `internal/service/setplay/setplay.go:4` と HANDOVER:49 に残っている。公開版 tacpendium の同じファイルにも同じ文があるので、開発者決定（公開済みのものは伏せない）の範囲内ではある。§13 の語ごとに grep の結果（何件、どこ、なぜ残すのか）を照合表に書いておくと、S9 で迷わない。
- **低-4** zh-Hant の `limitations.newCharacters` が「上線 2 周後」になっている。繁体字なら「2 週後」が自然（OpenCC の s2t は「周」を変換しない）。`web/src/locales/zh-Hant.json`。
- **低-5** `ci.yml` の Web job では、互いに独立な `tsc` と `vitest` に `if: ${{ !cancelled() }}` が付いていないので、`tsc` が落ちると `vitest` の結果が出ない。AGENTS §9 で新しく足した「独立な検査は `!cancelled()`」と揃っていない（Go job だけに当てた）。
- **低-6** URL から技付きで開いた後（`byMove=false` で、`showMove` は `form.targetMove` から真になる）に、技の欄で空の選択肢を選ぶか、キャラを替えると、技の欄がその場で消えて種類の欄に変わる。`ConditionsPanel.tsx:90`。`targetMove` が入ったときに `setByMove(true)` しておけば、この揺れは無くなる。
- **低-7** `.dimmed`（`web/src/styles.css:54`）は、どこからも使われなくなった。
- **低-8** `web/edge/index.test.ts` の「セキュリティのヘッダ」の試験は `API_SECURITY_HEADERS` の中身を回して比べているので、定数から 1 項目を消しても緑のまま。E2E の `headers.spec.ts` は値を別に持っているのでそこで赤になるが、単体でも期待値を直接書いておくと、Vitest だけで気づける。
- **低-9**（推測）Worker の最後の `env.ASSETS.fetch(req)`（`web/edge/index.ts:157`。アセットに無い `/api` 以外の道）が返す応答に `_headers` が掛かるかは確かめていない。404 のページだけなので影響は小さい。外部診断のときに、存在しないパスも 1 つ見ておくとよい。

### 指摘なし（見たもの）

- **CI の統合**: 前の 6 job（checks・go・wasm・upstream-oracle・precompute・web）の段を突き合わせた。ライセンス・SHA 固定・置き場所・verify_plan の試験・ack の検査・`bash -n`・tidy・取得・gofmt・vet・oracle の plan/test/record/save・moves DB の再生成・事前計算・正解の判定/再生成/cmp・WASM の plan/test/build/node/record/save・Web の全段は、どれも残っている。消えたのは `go.mod`／`package.json` の有無の判定だけで、どちらも「置き場所」の段で守られている。`continue-on-error` は無い。`!cancelled()` の段が落ちれば job は失敗になるので、失敗が緑として通る `if:` は見つからなかった。記録の段（oracle・wasm の record/save）は `success()` で、前の段のどれかが落ちていれば保存しない。plan の段が落ちて `chars` が空になったときは安全網だけを回すが、その場合も plan の段の失敗で job は赤になる。
- **PR での本家の正解の基準**: `pull_request` では `github.ref` が `refs/pull/N/merge` なので、`git merge-base origin/main HEAD` を通る。`fetch-depth: 0` で `origin/*` を取り、マージコミットは main の先端を親に持つので、分岐点は main の先端になり、差分は PR の変更だけになる。正しく動く。S7 は `go.mod` を変えたので、PR では作り直しが走る（想定どおり）。main への最初の push で `before` が 0 のときは `changed.sh` が作り直す側に倒す。
- **job の上限時間**: 90 分。S4 の実測（全部回したとき go 約 6〜10 分・WASM 約 6〜9 分・事前計算 約 25 分。事前計算の全 KA の突き合わせは S6 で削除済み）と正解の再生成（約 5 分）を足しても収まる。
- **i18n**: 5 言語でキー集合が一致（158）。消したキー（`pickTargetToggle`・`clearSelectedMove`）と古い文言（「特定の技から探す」「当てたい技の種別」「種別フィルタ」「選択技をクリア」「Search from a specific move」「Meaty move type」）は、`web/src`・`docs`（S0〜S6 の報告を除く）に 0 件。Go のコメントの「当てたい技」はエンジンの用語なので、そのままでよい。
- **リセット**: 計算中なら `cancel()`（計測も送る）→ form・run・URL を戻す。`autoRunDone` は真のままなので、URL を空にしても自動の計算は走り直さない。言語と表記は戻さない。
- **キャラ検索**: 公開版の `text-filter.ts` の規則（NFKC・小文字・語の先頭一致）と合っている。選んでいるキャラは絞り込みで外れても一覧に残る。
- **試験の数**: `_reference/` 以外では、試験は減っていない（Go の `func Test` は基準 256 → HEAD 256。基準の 407 との差は `_reference/` の中の 151。E2E の `test(` 13 → 16、vitest の `it/test(` 108 → 119）。
- **公開してはいけない語**: 現在の木で違反 0。7〜40 桁の 16 進で数字と英字が混ざるもの（md/go/ts/sh/py/yml/toml/mjs の 131 種）のうち、このリポジトリの commit でも公開版 tacpendium の commit でもないもの 38 件を全部見た。Action の SHA、セッション ID（UUID）、データのハッシュ、Cloudflare docs の commit、試験用のダミーだけで、本家の SHA らしいものは無かった（推測: 公開版の取得は shallow なので、古い公開 commit もこの 38 件に入っている）。

## 4. 良かった点

- 禁止語をハッシュで持ち、検査スクリプト自体が「何を伏せたか」の一覧にならない作り。16 進は先頭 7 文字で比べるので、長い SHA も当たる。
- `_headers` と Worker の両方の経路を、本物の配信で 1 本ずつ見る E2E を置いた（AGENTS §8-5）。
- CI をまとめた後も、記録の段は `success()` のまま残し、「失敗の後に記録しない」を守っている。
- 公開の手順に、PR の head（`refs/pull/N/head`）から古いコミットが読めることまで書いてあり、新しいリポジトリで公開する理由がはっきりしている。

## 5. 自分で回したコマンドと結果

出力はすべて `/tmp/claude-0/review/` に書き、ファイルから読んだ。

```
$ go test -count=1 ./...            > go.txt          (go1.26.9 は GOTOOLCHAIN で取得)
EXIT=0   ok のパッケージ 10・FAIL 0
$ cd web && pnpm exec vitest --run  > vitest.txt
 Test Files  15 passed (15)
      Tests  158 passed (158)
EXIT=0
$ cd web && pnpm build              > build.txt
合計: 6659557 bytes / 違反: 0 件 / 束ねた: dist-edge/worker.mjs, local.mjs
EXIT=0
$ cd web && PW_CHROMIUM_PATH=/opt/pw-browsers/chromium-1194/chrome-linux/chrome pnpm exec playwright test > e2e.txt
  24 passed (42.3s)
EXIT=0
$ python3 scripts/check-licenses.py              → MIT: 55 件 / 違反: 0 件  EXIT=0
$ bash scripts/check-actions-pinned.sh           → workflow: 3 本 / uses 参照: 14 件 / 違反: 0 件  EXIT=0
$ python3 -I scripts/check-private-refs.py       → 走査: 270 ファイル / 違反: 0 件  EXIT=0
$ python3 -I scripts/check_private_refs_test.py  → Ran 5 tests / OK  EXIT=0
$ python3 -I scripts/ci/verify-plan.py check     → 見張るコード 6 か所: 記録と一致 / 違反: 0 件  EXIT=0
$ python3 -I scripts/ci/verify_plan_test.py      → OK  EXIT=0
$ gofmt -l（git ls-files の *.go）               → 出力は空  EXIT=0
$ go vet ./...                                   → EXIT=0
$ go mod tidy -diff                              → EXIT=0
$ cd web && pnpm exec tsc --noEmit               → EXIT=0
$ ci.yml・dependabot.yml の YAML 読み込み          → ok  EXIT=0
$ bash -n（hooks・scripts・upstream・web/scripts） → EXIT=0
基準の木（52af7a19）に check-private-refs の find() を当てる → _reference 以外で 2 件（HANDOVER:5・:362）、_reference 内で 1 件
```

回していないもの: `node --test scripts/wasm/wasm.test.mjs`、WASM の go test、govulncheck、`pnpm audit`、本家の正解の再生成（報告の値を信じた）。

## 取り込み（実装担当。2026-10-18）

| 指摘 | 判断 | 理由・やったこと |
|---|---|---|
| 高-1 報告に逐語の証拠と `git diff --stat` が無い | 採用 | 取り込みの後に全検査を回し直し、完了報告の「最終確認」に `EXIT=`・件数・`git diff --stat 52af7a1` を貼る |
| 中-1 技が未選択でも押せる／隠れた種類の誤り | 採用 | 「1 つの技を指定」で技が未選択なら「提案を出す」を押せなくし、「技を選んでください」（5 言語）を出す。種類の誤りは種類で選んでいるときだけ出す。vitest 1 本。§8-3: 押せなくする条件を外すと赤・隠れた誤りを出すと赤（同じ試験の前半・後半）。最初の版の試験は種類を 0 個にしてから見ていたため別の理由で押せず、条件を外しても緑だった。既定の種類のまま見る形に書き直した |
| 中-2 拡張子・パス・`_` の形を見逃す | 採用 | 語を `_`・`.`・`-` で部分に分け、連続する部分の並びを `-` でつないだ形すべてを照合。試験に拡張子・パス・`_`・前後に語が付いた形・`.` 区切りを足し、直す前のコードで赤（§8-3）。本物の禁止語の拡張子付き・`_` 版を一時ファイルで当て、2 行とも検出（ファイルは消した。語はここに書かない） |
| 中-3 HANDOVER §2.1・375 行目 | 採用 | 「S7 で削除。移植元は公開版 tacpendium」を添えた |
| 低-1 telemetry の見出しの参照 | 採用 | 「残課題・既知の問題」に直した |
| 低-2 SHA の件数の食い違い | 採用 | 検査のコメントと報告の両方に、4 つの禁止語の内訳（木にあった 2 つ＋木に無い SHA 2 つ）を書いた |
| 低-3 §13 の語ごとの扱い | 採用 | `docs/release-checklist.md` に語ごとの表（どこに残り、なぜ残すか） |
| 低-4 zh-Hant「2 周後」 | 採用 | zh-Hans を「2 个星期后」にし、zh-Hant を生成し直した（「2 個星期後」）。zh-Hant は手で直さない規則のため |
| 低-5 Web job の tsc・vitest | 採用 | `if: ${{ !cancelled() }}` を付けた |
| 低-6 URL から技付きで開いた後の揺れ | 採用 | 技が入ったら選び方を「1 つの技を指定」に固定（`useEffect`）。vitest（URL で開いて技を空にしても欄が残る）。§8-3: 固定を外すと赤 |
| 低-7 `.dimmed` が未使用 | 採用 | 消した |
| 低-8 単体の試験が定数を回している | 採用 | 期待値を試験に直接書き、定数とも一致を見る。§8-3: 定数から `x-frame-options` を消すと赤 |
| 低-9 アセットに無いパスのヘッダ | 採用（記録） | 外部診断の手順（`docs/release-checklist.md` §3）に「存在しないパスも 1 つ見る」を足した。手元では確かめていない（Cloudflare の挙動のため） |

## 再レビュー（1 回目）

対象: 取り込みのコミット `0ed6aee`（親 `e591341` との差分）と、依頼された各ファイル。読み取りのみ（本節の追記だけ）。

### 取り込みの照合

| 指摘 | 照合 | 根拠 |
|---|---|---|
| 高-1 | **未（予定のまま）** | 「最終確認に貼る」は未来形のとおりで、`docs/progress/S7-report.md` に「最終確認」の節・`EXIT=` の逐語・`git diff --stat 52af7a1` はまだ無い（`grep -n '最終確認\|diff --stat'` は計画節の 48 行目だけ）。「やったこと」の試験の本数（`App.test.tsx` に 5 本 等）も取り込みの 2 本を反映していない。貼るまで高-1 は閉じない |
| 中-1 | 済 | `ConditionsPanel.tsx:79-89`・`:428`。`validateForm` は `targetMove != null` なら種類を問わない（`formState.ts:114`）ので、`showMove` のときに隠す `targetTypesRequired` は、技が未選択＝`moveRequired` のときにしか出ず、代わりに `target-move-required` が出る（文が消えて押せないだけ、にはならない）。試験は既定の 3 種類のまま見ているので、`|| moveRequired` を外せば前半が赤、`&& showMove` を外せば後半が赤（コードを読んでの判断。書き換えての確認はしていない） |
| 中-2 | 済 | `_word_forms` は部分の連続した並びを `-` でつなぐ。基準の木の HANDOVER（`git show 52af7a1:...`）に新しい `find` を当て、2 行（5・362）を検出＝既存の DENY の語の形と食い違っていない（`_`/`.` 入りの形でハッシュしていたら当たらなくなるところだった）。今の木で違反 0（271 ファイル）＝誤検出なし |
| 中-3 | 済 | HANDOVER:45・375。`git grep _reference`（`docs/progress` を除く）の残りは、どれも「削除した／削除する」の文脈 |
| 低-1〜4・7・8 | 済 | telemetry:59、報告 89 行目と検査のコメントが「4 つ」で一致、checklist に語ごとの表、zh-Hant は生成し直しと一致（`renderLocale(toTraditional(zh-Hans))` と同一＝true）、`.dimmed` は `git grep` で 0、edge の試験は期待値を直書き |
| 低-5 | 一部 | 下の低-R2 |
| 低-6 | 済 | `useEffect`（`ConditionsPanel.tsx:76-78`）。リセットは `setByMove(false)` を先に呼ぶ（:439）ので、E2E の「リセットで種類で選ぶに戻る」も緑 |
| 低-9 | 済（記録） | checklist §3 |

### 新しい指摘

**高**: なし（高-1 は上のとおり未了のまま）。

**中**

- **中-R1 `docs/frontend.md` に「技が未選択なら押せない」と「技が入ったら選び方を固定」が無い**（`docs/frontend.md:67`・`:70`）。67 行目は押せない条件を列挙しているが、取り込みで足した条件（「1 つの技を指定」で技が未選択）が無い。70 行目にも固定の挙動が無い。食い違いではなく欠落だが、AGENTS §8-8 の「仕様を変えたら docs も」に当たり、S8 以降が押せない理由を docs から辿れない。67・70 行目に 1 文ずつ足す。

**低**

- **低-R1 `_word_forms` は部分の数の 2 乗**: 区切りの多い 1 語（`a-a-…` 2000 部分）で `find` が 15 秒（手元で計測）。今の木の最大は 12 部分で全体 1.7 秒なので実害は無い。DENY の語の部分の数の上限（例: 8）で `j - i` を打ち切れば線形になる。
- **低-R2 Web job の `!cancelled()` が tsc・vitest だけ**: `prepare-assets`・`pnpm build`・E2E には付いていないので、vitest が落ちると E2E の結果が出ない（`ci.yml:364-` 以降）。build と E2E は前段の成功に依るので今の形でもよいが、そうなら低-5 の「独立な段」の範囲を報告に書く。
- **低-R3 `target-move-required` がキャラ未選択でも出る**: キャラを選ぶ前に「1 つの技を指定」を押すと、技の一覧が空のまま「技を選んでください」が出る。他の欄は「開いた直後から赤くしない」方針（`characterRequired` は出さない）なので、キャラが未選択のときは出さない方が揃う。
- **低-R4 checklist の語ごとの表の「残っている場所」が一部抜ける**: `autopilot-combomgr` は `docs/progress/S0-report.md:14` にもある（`git grep -n autopilot-combomgr`）。表に `docs/progress/` を足す。

### 回したコマンドと結果（出力は `/tmp/claude-0/rereview/`）

```
cd web && pnpm exec tsc --noEmit                      → EXIT=0
cd web && pnpm exec vitest --run                      → Test Files 15 passed (15) / Tests 159 passed (159) / EXIT=0
python3 -I scripts/check_private_refs_test.py         → Ran 6 tests / OK / EXIT=0
python3 -I scripts/check-private-refs.py              → 走査: 271 ファイル / 違反: 0 件 / EXIT=0（real 1.686s）
cd web && pnpm build                                  → EXIT=0
cd web && PW_CHROMIUM_PATH=... pnpm exec playwright test → 24 passed (41.7s) / EXIT=0
新しい find を基準の木の HANDOVER に当てる           → hits [5, 362]
zh-Hant の再生成と一致                                → match true。5 言語とも 159 キー、targetMoveRequired あり
```

件数: 高 0（未了の高-1 が 1）・中 1・低 4。

## 取り込み（再レビュー 1 回目の分。実装担当）

| 指摘 | 判断 | 理由・やったこと |
|---|---|---|
| 高-1（未了） | 採用 | 完了報告の「最終確認」に、取り込みの後に回した全検査の逐語と `git diff --stat` を貼る。「やったこと」の試験の本数も直す |
| 中-R1 `docs/frontend.md` に 2 つの挙動が無い | 採用 | 「技が未選択なら押せない」と「技が入ったら選び方を固定」を書いた |
| 低 `_word_forms` が部分の数の 2 乗 | 不採用（記録） | 今の木で全体 1.7 秒。異常に長い語（数千の部分）は追跡ファイルに無い。増えたら上限を設ける（完了報告の残課題） |
| 低 Web job で vitest が落ちると E2E が出ない | 採用 | Web job の用意・ビルド・Chromium・E2E にも `!cancelled()` を付けた（落ちた段の後の段は、前提が欠けていれば自分も落ちる＝緑にはならない） |
| 低「技を選んでください」がキャラ未選択でも出る | 採用 | キャラを選んでからだけ出す（押せない条件は変えない） |
| 低 checklist の語ごとの表に S0 報告が抜け | 採用 | 足した |
