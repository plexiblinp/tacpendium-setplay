# S8 レビュー（ローカル版・govulncheck の常設）

- 対象: `c6d5ded15d9784c860425ed41dcf7c77da3193bf`（段階開始。依頼と `S8-report.md` の記載が一致）〜 `85a6163`（HEAD）
- 手順: `.claude/commands/review.md`（読み取り専用。書いたのは本ファイルだけ）
- 読んだもの: AGENTS.md §4・§8・§9、HANDOVER §5・§9 の S8 行、`docs/progress/S8-report.md`、`git diff c6d5ded HEAD` の全ファイル（42 files changed, 1694 insertions(+), 105 deletions(-)）

## 1. 要約

完了条件（ローカル版・単一バイナリ・Releases の workflow・3 OS での起動確認）と開発者の追加指示（govulncheck の常設・週 1＋手動の workflow・わざと赤・S9 の手番の記載）は、どれも実装されている。手元で回した単体・tsc・vitest・smoke・E2E（project local）・govulncheck・リポジトリ検査は報告の値と一致した。サイト側の KA 100・計測・票の挙動は、vitest と Worker の検査（`KA_MAX_SITE` 固定）で保たれている。ローカル版で票・計測を 0 件や unavailable に寄せず `null`（＝使わない）で持つ形も §8-7 に合っている。
一方で、**コメント・docs とコードの食い違いが 3 件**ある（高）。(1) 「edge/ は flavor.ts を読まない」と書いてあるが、実際は `shared/api.ts → lib/conditions.ts → lib/flavor.ts` の経路で読んでいる（`?.` のおかげで動いている）。(2) 古い mode 名 `local` が 2 か所に残っている。(3) `-port 8787` を指定したときの挙動が docs と違う。中は、報告の `git diff --stat` 全文がまだ無いこと、KA を大きくしたときの境界（WASM のメモリの壁）の未確認、AGENTS §9「job を増やさない」との関係、計測の記述の食い違いの 4 件。

## 2. 完了条件への適合

| 項目 | 判定 | 根拠 |
|---|---|---|
| ローカル版（KA 無制限） | 済 | `KA_MAX_LOCAL = 2147483647`（`web/src/lib/constants.ts:9`。`engine.KnockdownFromFloat` の int32 の範囲と一致）。Go 側の `int` は js/wasm で 64 bit なので `ka+2` は溢れない。vitest「ローカル版」5 本・E2E local 3 本（KA 120・URL の KA 110・KA 400 の中断）を自分でも回して緑 |
| 単一バイナリ | 済 | `cmd/local`（`go:embed all:site`・標準ライブラリのみ）＋`internal/localserver`。smoke 12 項目を自分でも回して ok |
| Releases | 済（workflow まで） | `local-release.yml`: タグ `v*` で draft を作るところまで（公開は開発者＝AGENTS §3-3）。最初のタグの push は S9-2 として `docs/release-checklist.md` §6 に書いてある |
| 3 OS で起動確認 | 済（HTTP の段まで） | 報告では run 39117681817 で ubuntu・macos・windows が success。自分では GitHub の run を開いていない（このレビューは手元だけ）。画面の操作は Linux の E2E だけで、報告の残課題にもそう書いてある |
| govulncheck を CI の Go job に 1 step（job を増やさない・3 対象・版固定） | 済 | `ci.yml` の Go job に 1 段・summary 1 行。`scripts/govulncheck.sh` は v1.9.0 固定で `./...`・`GOOS=js GOARCH=wasm ./cmd/wasm/`・`./cmd/local/` を windows／darwin／linux で回す。手元で EXIT=0・対象 5 件・違反 0 件 |
| 週 1＋手動の小さな workflow | 済 | `.github/workflows/govulncheck.yml`（cron `41 0 * * 1`・`workflow_dispatch`・`contents: read`・SHA 固定） |
| 検出で赤・わざと起こして赤 | 済（報告のとおり） | スクリプトは失敗した対象を数えて非 0 で終わる。CI の段は `set -o pipefail` 付き。わざと赤は `GOTOOLCHAIN=go1.26.4` で EXIT=1・違反 4 件（報告。自分では再現していない） |
| About の Website を S9 の手番として書く | 済 | `docs/release-checklist.md` G-11・§6 S9-1 |
| デプロイ・ヘッダ診断・§5 の「済」付けをしない | 済 | §6 S9-4 に回してある。§5 に「済」は付いていない |

## 3. 指摘

### 高

**高-1. 「edge/（Worker）は flavor.ts を読まない」というコメントと報告がコードと食い違う**
- 場所: `web/src/lib/flavor.ts:3`、`web/src/lib/constants.ts:2`、`docs/progress/S8-report.md:77`（判断 3）。実際の経路は `web/edge/index.ts:7`（と `edge/beacon.ts:2`・`edge/votes.ts:5`）→ `web/src/shared/api.ts:6`（`SAFE_JUMP_KINDS`・`TARGET_TYPE_ORDER` を `../lib/conditions` から読む）→ `web/src/lib/conditions.ts:14`（`import { KA_MAX } from "./flavor"`）→ `flavor.ts:18`（`import.meta.env?.MODE`）。
- なぜ問題か: Worker は flavor.ts を実際に読んでいる。動いているのは `?.` のおかげと、Vite の束ねで tree-shake されているから（自分で `edge/index.ts` を build-edge.mjs と同じ設定でスクラッチに束ね、`MODE`・`import.meta`・`flavorFromMode` が出力に 0 件なのを確かめた）。本番は wrangler（esbuild）が束ねる。推測: esbuild は `flavorFromMode(...)` の呼び出しを副作用ありとして残すので、`import.meta.env?.MODE` は Workers の実行時に `undefined` になり、サイトとして動く。ただ、コメントを信じて `?.` を外したり、flavor.ts に Vite 専用のもの（`import.meta.env.X` の直接参照・`import.meta.glob` 等）を足したりすると、Worker だけが壊れる。その壊れ方は vitest にも Vite の束ねにも出ない（tree-shake で消えるため）。依頼の「edge Worker が import.meta.env に依存しないか」の答えは「依存はしていないが、読んではいる」で、記述が実態と違う。
- 直し方の案: (a) `conditions.ts`・`formState.ts` は flavor を読まないようにする（`kaMax` を必須の引数にして、既定値はコンポーネント側で `kaMaxFor(FLAVOR)` を渡す）。または `SAFE_JUMP_KINDS`・`TARGET_TYPE_ORDER` を flavor に依存しない小さなモジュールへ移し、`shared/api.ts` からはそれを読む。(b) 守りの検査を置く。例: `build-edge.mjs` の後か vitest で、edge の import グラフ（または束ねた worker.mjs）に `flavor` も `import.meta` も無いことを確かめる。§8-3 に従い、一度わざと読ませて赤くなるのを確かめる。(c) コメントと報告の判断 3 を実態に合わせる。

**高-2. 古い mode 名 `local` がコメントに残っている（§8-8）**
- 場所: `internal/localserver/localserver.go:3`（「web の `vite build --mode local`」）、`web/src/lib/flavor.ts:12`（「local 以外はサイト」。実際は `standalone` 以外がサイト）。
- なぜ問題か: 実装は `standalone` に変えた（Vite が `local` を受け付けないため）。このコメントのとおりに `--mode local` でビルドすると、Vite が拒否するか、サイト版がビルドされる（flavorFromMode が site を返す）。その結果、KA 100 で止まり、計測・票の通信をするバイナリができる。§8-8 の grep（`mode local`）で拾えるのに残っている。
- 直し方の案: 2 か所を `standalone`（または `LOCAL_BUILD_MODE`）に直す。`grep -rn "mode local"` の残数が、計画の記録（S8-report.md:22）の 1 件だけになることを確かめる。

**高-3. `-port 8787` を明示したときの挙動が docs と違う**
- 場所: `docs/local.md:28`（「指定したポートが使われていれば誤りで終わる」）と `cmd/local/main.go:70`（`listen(*port, *port == defaultPort)`）。
- なぜ問題か: 判定は「値が既定と同じか」なので、利用者が `-port 8787` と明示しても、8787 が使われていれば黙って空きポートに移る。自分で、1 つ目のバイナリが 8787 を使っている間に `-port 8787` で 2 つ目を起動すると、誤りにならず `http://127.0.0.1:43127/` で起動した（再現済み）。docs を信じた人は、固定ポートでブックマークした URL が別インスタンスを指すことに気づけない。`TestListen_Fallback` は `listen()` を直接呼ぶので、この判定の誤りは試験に出ない。
- 直し方の案: `fl.Visit` で `port` が明示されたかを見て、明示なら `fallback=false` にする。試験は `run()` 越しに「使用中のポートを `-port` で渡すと誤り」を足す。もしくは docs の方を「8787 を指定した場合は既定と同じ扱い」に直す。どちらにするかを決めて、フラグの説明文（main.go:58）とも揃える。

### 中

**中-1. 報告に `git diff --stat` の全文がまだ無い（§8-11）**
- 場所: `docs/progress/S8-report.md:143-145`（「全文は下の『最終確認』に貼る」とあるが、その節が無い）。
- なぜ問題か: 「テストのファイルは追加のみ」の証拠がまだ報告に無い。自分で見た範囲では、テストのファイルは追加と追記だけで、消えた試験は無い（`App.test.tsx +74/-1`・`conditions.test.ts +13`・`formState.test.ts +9`・新規 `flavor.test.ts`・`localserver_test.go`・`main_test.go`・`e2e/local.spec.ts`。`-1` は `renderApp` のシグネチャの 1 行）。
- 直し方の案: レビューの取り込み後に「最終確認」節を足し、`git diff --stat c6d5ded` の全文と、検査をもう一度回した `EXIT=` と件数を貼る。

**中-2. KA を大きくしたときの境界（WASM のメモリの壁）が未確認のまま「無制限」にしている**
- 場所: `web/src/lib/constants.ts:9`（`KA_MAX_LOCAL`）、`docs/local.md:41-43`、報告の残課題。
- なぜ問題か: 入力は 2147483647 まで通る。一方、確かめた最大は KA 400 の中断だけで、計算の完了は KA 200 まで。4 GiB の壁に当たったとき、Go の WASM は `fatal error: out of memory` で exit する。推測: そのとき Worker は例外を投げずに止まるので、`engineClient` の `onerror` が呼ばれず、画面が「計算中」のまま（中断を押すまで）になりうる。「誤り」と「計算中」が区別できない状態になる（§8-7 の近縁）。報告の残課題に書いてあるので逸脱ではないが、利用者が最初に試しそうな境界（KA 1000 等）の挙動を誰も見ていない。
- 直し方の案: 一度、手元で KA 1000 程度を中断せずに放置し、完了／誤り表示／固まるのどれになるかを記録する。固まるなら、Go の exit（wasm_exec の `exited`）を Worker から `error` として返す。docs の目安（「1 GB 近い」）も、その実測で書き直す。

**中-3. `local-release.yml` は push ごとに job を 4〜5 本足している。AGENTS §9「job を増やさない」との関係が報告に無い**
- 場所: `.github/workflows/local-release.yml:13-45`（`claude/**` への push と main 向けの PR で、`web/**`・`engine/**` 等が変わると build＋smoke×3 が走る）、`AGENTS.md:159`。
- なぜ問題か: public 化で Actions は無料になったが、§9 の文はそのまま残っている。報告の判断 11 は paths フィルタの話だけで、「CI の job は増やさない」という規約に対する例外の理由（3 OS の起動確認は完了条件で、別 OS の runner が要る）を書いていない。範囲を広げた事実の記録（§8-2）が足りない。
- 直し方の案: AGENTS §9 の追記行（:157）か報告の判断に「3 OS の runner が要るので別 workflow に分けた。§9 の『job を増やさない』は ci.yml の話」と書く。回数を絞るなら、`claude/**` への push を外し、PR・タグ・手動起動だけにする手もある。

**中-4. 計測の記述が計画・実績・docs の間で食い違う（§8-2・§8-6）**
- 場所: `docs/progress/S8-report.md:19`（KA 400 は「77 秒で打ち切り」・「画面の条件での計測は実装後にやり直す」）と `:71`（「110 秒の打ち切り」・やり直していない）、`docs/local.md:43`（「1 GB 近いメモリ」。実測の最大は 646 MiB）。
- なぜ問題か: 計画で約束した「画面の条件での計測のやり直し」をしていないのに、逸脱の欄は「無し」になっている（`S8-report.md:89`）。KA 150（5.7 秒）が KA 100（8.0 秒）より速いという逆転にも触れていない（原因未特定と書くべきところ）。docs の「1 GB 近い」は測っていない数字。
- 直し方の案: やり直さないなら逸脱として書く。77 秒と 110 秒は別の回であることを明記する。逆転は「原因未特定」と書く。docs の値は実測に揃える（中-2 と一緒に測るとよい）。

### 低

**低-1. ヘッダの規則を、解決後の名前ではなく生の URL パスに当てている**
- 場所: `internal/localserver/localserver.go:175`（`HeadersFor(h.rules, r.URL.Path)`）。
- 内容: `curl --path-as-is /assets/../index.html` では index.html が返り、`/assets/*` の `Cache-Control: public, max-age=31536000, immutable` が付いた（再現済み）。ブラウザは送る前にドットの区間を正規化するので実害は小さい。ただ、ファイルを決めたパスとヘッダを決めたパスがずれている。パスの安全性そのもの（外へ出ない・`_headers` とドットで始まる名前・ディレクトリを配らない・GET/HEAD のみ・Host の検査）は、試験と手元の curl（`//index.html`・`%2e%2e`・`%00`・OPTIONS→405）で問題が無かった。
- 直し方の案: `HeadersFor(h.rules, "/"+name)` にする（`/` のときは `/` のまま扱う）。試験に 1 件足す。

**低-2. govulncheck の版固定には更新の経路が無い**
- 場所: `scripts/govulncheck.sh:12`。
- 内容: Dependabot はシェルの中の版を見ない。道具の更新が止まると、新しい Go の版の解析に追いつかなくなる。AGENTS §5 は版を固定したら解除条件を書くよう求めているが、報告にはそれが無い。
- 直し方の案: 報告か docs に「更新の手順（`go list -m golang.org/x/vuln@latest` で確かめて書き換える）と頻度（例: 週 1 の run が道具の古さで落ちたとき）」を書く。または、別の `tools/go.mod` にして Dependabot に見せる。

**低-3. Release の job はどのブランチのタグでも走る**
- 場所: `.github/workflows/local-release.yml:177`。
- 内容: docs（`docs/local.md` 配布 1・release-checklist S9-2）は「main で」としているが、workflow は確かめていない。draft なので害は小さい。
- 直し方の案: release job で `git merge-base --is-ancestor "$GITHUB_SHA" origin/main` を確かめる（checkout を足す）。または docs に「main 以外のタグでも draft はできる」と書く。

**低-4. smoke の「index.html が読むファイルがすべて 200」は、0 件でも ok になる**
- 場所: `scripts/local/smoke.sh:58-63`。
- 内容: Vite の `base` が相対（`./`）に変わると、`(src|href)="/` に 1 件も当たらず、黙って通る（§8-3「1 件ある入力で 1 件と出る」）。いまは 4 件当たっている。
- 直し方の案: 見つけた件数を数え、0 件なら FAIL にする。

**低-5. `-port`・SIGTERM 周りの小さな点**
- 場所: `cmd/local/main.go:84`。
- 内容: `os.Interrupt` しか受けないので、SIGTERM（Linux のサービス管理・`kill`）では Shutdown を通らずに終わる。配るのは静的ファイルだけなので実害はほぼ無い。
- 直し方の案: `syscall.SIGTERM` も足す（Windows ではビルドタグか `os.Interrupt` だけにする）。

**低-6. `validateForm` の JSDoc が 2 つ並んでいる**
- 場所: `web/src/lib/formState.ts:82-87`。
- 内容: 新しい `/** kaMax は… */` が関数に付き、もとの説明（使わない欄は既定値・safeJump は null）が宙に浮く。エディタの表示では新しい方しか出ない。
- 直し方の案: 1 つの JSDoc にまとめ、`@param kaMax` にする。

**低-7. 5 言語の文言の小さな揺れ**
- 場所: `web/src/locales/ko.json`（`kaHelp` の「다운시킨 후의」に対して `kaHelpLocal` は「다운시킨 뒤의」）、`zh-Hans.json`・`zh-Hant.json`（`kaHelp` の「连段最后击倒对手后的」に対して `kaHelpLocal` は「连段收尾击倒后的」）。
- 内容: 同じ文の上限の部分だけを変えたいのに、前半の言い回しまで違っている。キー集合は 5 言語で一致し、非日本語ロケールへの日本語の混入は無い（vitest の既存の検査が緑）。en・ja は揃っている。
- 直し方の案: `kaHelpLocal` の前半を各言語の `kaHelp` と同じ言い回しにする。

**低-8. E2E の site だけを回すときも、ローカル版のバイナリが要る**
- 場所: `web/playwright.config.ts:41-54`。
- 内容: webServer は project に関係なく両方とも起動する。`--project site` でも、`build/local/` が無いと起動待ちで落ちる。docs（frontend.md）に書いてあるので低。
- 直し方の案: `LOCAL_BIN` が無く既定のパスにも無いときは、local の webServer と project を外す（そのとき「local を回していない」と出す）。または今のまま docs で足りるとする。

### 指摘なし（見たもの）

- **サイト側の挙動**: `KA_MAX_SITE` は beacon・票の schema（`shared/api.ts`）と Worker（`edge/index.ts:138`）で固定のまま。サイトの `validateForm`・`parseSearch` の既定は `FLAVOR=site` で 100（`flavor.test.ts` が mode test／production／undefined をサイトと確かめる）。サイトでの中断（票を待つ間）の計測は、S6 からの `unavailable` を保っている（`App.tsx` の cancel）。
- **§8-7**: ローカル版の `votes` は `null`（使わない）で、`unavailable` にも `{counts:{}}` にも寄せていない。`report` は `!online || votes == null` で送らず、`votesStatus` を作らない。Results は `votes != null` のときだけ票の案内とボタンを出す。
- **workflow**: すべて `contents: read` が既定で、`release` job だけが `contents: write`。Action は SHA 固定（`check-actions-pinned.sh` EXIT=0・違反 0 件）。`run:` に入る値は env 経由（`matrix.os` は静的な値）。タグの push には paths フィルタが効かない（GitHub の仕様）ので、`tags: v*` と paths の併記は意図どおり動く。Windows／macOS でも `shell: bash` で、`sha256sum`／`shasum` の切り替えがある。
- **禁止事項（§7）**: 秘密情報・公式データ・combomgr を取込元にすること、いずれも見当たらない（`check-private-refs.py` EXIT=0・違反 0 件・走査 290 ファイル）。
- **配線（§8-5）**: E2E local は、実バイナリ → 埋め込み → `_headers` → Worker の WASM → 画面を通す。サイト版の dist を埋め込むと 3 本とも赤になることを報告で確かめている。

## 4. 良かった点

- 計測・票を「無い（null）」として型で区別し、送らない分岐を `online` と `votes == null` の二重にしている。0 件や unavailable に寄せていない。
- Content-Type を固定の表にした（Windows の registry の MIME の罠）うえで、windows の runner で `text/javascript` を実際に確かめている。
- smoke が「利用者と同じ手順」（配る圧縮をダウンロード → SHA256SUMS → 展開 → 起動）で 3 OS を見ている。
- govulncheck は `go install` してから GOOS を変える（道具が js 向けになる罠）。古い toolchain でわざと赤にして、OS ごとの差（WASM は 0 件）まで記録している。
- `flavor`・`kaMax` を引数で受ける形にしたので、1 つのビルドの試験でサイトとローカル版の両方を回せる。

## 5. 自分で回したコマンドと結果

出力はスクラッチのファイルへ書き、合否は EXIT と件数で見た。バイナリは手元にあった `build/local/tacpendium-setplay-local_linux_amd64`（`-version` は `12de659-dirty`。12de659 から HEAD までの web/src の変更は `flavor.ts` の mode 名とコメント・テストだけで、中身の JS に `standalone` が入っているのを確かめた）。

```
$ go test -count=1 -v ./cmd/local/ ./internal/localserver/ > go.txt 2>&1; echo EXIT=$?
EXIT=0
--- PASS(全階層)=10 / --- FAIL=0
$ go vet ./cmd/local/ ./internal/localserver/ → VET=0
$ gofmt -l cmd internal engine → 出力 0 行
$ cd web && pnpm exec tsc --noEmit → TSC=0
$ pnpm exec vitest --run > vitest.txt 2>&1; echo EXIT=$?
EXIT=0
 Test Files  17 passed (17)
      Tests  172 passed (172)
$ bash scripts/local/smoke.sh build/local/tacpendium-setplay-local_linux_amd64 > smoke.txt 2>&1; echo EXIT=$?
EXIT=0   (ok 12 行)  失敗: 0 件
$ PW_CHROMIUM_PATH=/opt/pw-browsers/chromium-1194/chrome-linux/chrome pnpm exec playwright test --project local > e2e-local.txt 2>&1; echo EXIT=$?
EXIT=0   3 passed (12.3s)
$ GOVULNCHECK_DB=https://storage.googleapis.com/go-vulndb bash scripts/govulncheck.sh > vuln.txt 2>&1; echo EXIT=$?
EXIT=0   govulncheck v1.9.0 / go version go1.26.9 linux/amd64 / 対象: 5 件 / 違反: 0 件
$ bash scripts/check-actions-pinned.sh → EXIT=0  workflow: 5 本 / uses 参照: 24 件 / 違反: 0 件
$ python3 -I scripts/check-private-refs.py → EXIT=0  走査: 290 ファイル / 違反: 0 件
$ python3 -I scripts/ci/verify-plan.py check → EXIT=0  見張るコード 6 か所: 記録と一致 / 違反: 0 件
$ git diff --stat c6d5ded HEAD | tail -1
 42 files changed, 1694 insertions(+), 105 deletions(-)
```

再現の確認（指摘の根拠）:

```
# 高-1: edge/index.ts を build-edge.mjs と同じ設定でスクラッチに束ねた → EXIT=0。
#       出力の worker.mjs に MODE / import.meta / standalone / flavorFromMode は 0 件(tree-shake で消える)
#       import の経路: edge/index.ts:7 → src/shared/api.ts:6 → src/lib/conditions.ts:14 → src/lib/flavor.ts
# 高-3: 1 つ目を -port 8787 で起動中に、2 つ目を -port 8787 で起動
#       → 2 つ目は誤りにならず "http://127.0.0.1:43127/ を開いてください" で起動したまま
# 低-1: curl --path-as-is http://127.0.0.1:18799/assets/../index.html
#       → Content-Type: text/html; charset=utf-8 / Cache-Control: public, max-age=31536000, immutable
#       (/index.html では Cache-Control は付かない)
# パスの安全性: //index.html→200、/%2e%2e/index.html→200(中身は index)、/index.html%00→404、/data/→404、OPTIONS /→405
```

回していないもの: `go test ./...` の全体（engine の突き合わせ）、E2E の project site、`GOTOOLCHAIN=go1.26.4` でのわざと赤の再現、GitHub Actions の run の中身の確認。

## 取り込み（実装担当が追記）

| 指摘 | 採否 | 理由・直したこと |
|---|---|---|
| 高-1 edge が flavor.ts を読む経路がある | 採用 | `lib/conditions.ts`・`lib/formState.ts` は flavor を読まない形にした（`kaMax` の既定はサイトの `KA_MAX_SITE`、ローカル版は画面が `kaMaxFor(flavor)` を渡す。もともと画面は全部渡していた）。`flavor.ts` の `KA_MAX` を消し、コメントを実態に合わせた。試験 `web/edge/imports.test.ts`（Worker の入口から flavor.ts に届かないこと。§8-3: conditions.ts に flavor の import を足すと赤） |
| 高-2 古い mode 名 `local` | 採用 | `internal/localserver/localserver.go:3`・`flavor.ts` の注記を `standalone` に。全体を grep して残りは計画節の記録（意図的）だけ |
| 高-3 `-port 8787` を明示しても空きポートへ移る | 採用（コードを docs に合わせた） | `fl.Visit` で明示を判定し、明示したポートが使えなければ誤り。試験 `TestRun_ExplicitPortBusy`（§8-3: 常に移る形に戻すと FAIL） |
| 中-1 diff --stat 全文と最終確認が無い | 採用 | 完了報告の「最終確認」に貼る |
| 中-2 KA の上限（4 GiB）の挙動を見ていない・「1 GB 近い」は実測でない | 一部採用 | `docs/local.md` の文を S5 の実測（1.06 GB）と「KA 100 超は未測定」に直した。4 GiB の壁の実測は残課題（KA 400 でも 2 分を超え、壁に届く KA を探すのに時間がかかる） |
| 中-3 local-release.yml が job を増やす理由が無い | 採用 | AGENTS §9 の行に例外の理由（3 OS は job を分けないとできない・public は無料・paths で絞る）を書いた |
| 中-4 計測の記述の食い違い・やり直しの未実施が逸脱に無い | 採用 | 報告の KA 400 を「110 秒の打ち切り」に揃え（計画節の 77 秒は誤記）、KA 150 が KA 100 より速い逆転（件数が 55347 → 25466 に減る）を書いた。画面の条件での計測のやり直しをしなかったことを逸脱に書いた |
| 低-1 ヘッダの規則を生のパスに当てる | 採用 | 規則は返すファイルのパスに当てる（`/` で終わる要求はディレクトリの URL のまま）。試験を足した |
| 低-2 govulncheck の版の更新経路・解除条件 | 採用 | `scripts/govulncheck.sh` の注記と報告の「版の固定」に書いた |
| 低-3 Release が main 以外のタグでも走る | 採用 | release job でタグのコミットが `origin/main` の祖先かを確かめ、違えば落とす |
| 低-4 smoke が 0 件でも ok | 採用 | 読むファイルが 1 件以上あることも条件にした |
| 低-5 SIGTERM を受けない | 採用 | `signal.NotifyContext` に `syscall.SIGTERM` を足した（Windows でも定義がある。`GOOS=windows go vet` で確認） |
| 低-6 JSDoc が 2 つ並ぶ | 採用 | 1 つにまとめた |
| 低-7 ko・zh の言い回しの揺れ | 採用 | `kaHelp` と同じ前半に揃えた（zh-Hant は再生成） |
| 低-8 site の E2E だけでもバイナリが要る | 不採用 | CI は両方を回すので常に作る。手元の手順は `docs/frontend.md` に書いた。片方だけ起動する分岐は設定を複雑にする割に得が小さい |
