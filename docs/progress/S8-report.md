# S8 ローカル版 — 完了報告

- 段階開始コミット: `c6d5ded15d9784c860425ed41dcf7c77da3193bf`
- ブランチ: `claude/optimistic-faraday-0kbp3t`

## 計画

### 完了条件（HANDOVER §9 S8）
「ローカル版(KA 無制限・単一バイナリ・Releases)」— **3 OS で起動確認**。
加えて開発者の追加指示（2026-10-18）:
- govulncheck の常設: CI の Go job に step を 1 つ（job は増やさない）。対象は `./...`・WASM（`GOOS=js GOARCH=wasm ./cmd/wasm/`）・ローカル版。`golang.org/x/vuln/cmd/govulncheck` は版を固定。週 1 回＋手動起動で govulncheck だけを回す小さな workflow。検出したら赤。わざと起こして赤を確かめる（AGENTS §8-3）。
- Cloudflare のデプロイ・ヘッダ診断・S9 の報告・チェックリスト §5 の「済」付けは S9（S8 ではやらない）。
- Cloudflare のデプロイ後に GitHub リポジトリの About の Website に URL を入れることを、S9 の手番として書いておく。

### 前提（読んで分かったこと）
- 計算は既にブラウザ内の WASM（S5 の計算方式 C）。サイトとローカル版の違いは「KA の上限」と「通信（計測 beacon・票）」だけ。
- KA の上限 100 は `web/src/lib/constants.ts` の `KA_MAX`（URL の検証・欄の検査・入力欄の max・文言）と、beacon の形（`web/src/shared/api.ts`）・Worker（`web/edge/index.ts`）で掛かる。エンジンは掛けない（`engine.KnockdownFromFloat` は int32 の範囲だけ）。
- 中断は既にある（Worker ごと terminate）。HANDOVER §5 の「Worker と中断ボタン付き」は満たしている。
- 予備計測（Node 上の WASM、ryu 重ねる・既定のクエリ）: KA 100 = 8.0 秒・428 MiB、KA 150 = 5.7 秒・646 MiB、KA 200 = 33.3 秒・605 MiB、KA 400 は 110 秒の打ち切りで未完了（レビュー中-4 で誤記を直した）。ingrid 汚連携 KA 100 は 3 分を超えて打ち切った（推測: 既定のクエリが画面と違う。画面の条件での計測は実装後にやり直す）。→ KA を上げると時間は大きく伸びるが、中断で止められる。

### 作るもの
1. **フロントのローカル版ビルド**: `vite build --mode local`（`web/src/lib/flavor.ts` で判定）。→ 実装では mode 名を `standalone` にした（Vite が `local` を `.env.local` と紛れるとして拒否するため）。
   - KA の上限を外す（推測: エンジンが受ける int32 の上限 2147483647 を `KA_MAX_LOCAL` にする。数で切らない＝「無制限」。重い条件は既存の「時間がかかる」注意と中断で扱う）。サイトは 100 のまま（`KA_MAX_SITE`。beacon と Worker は常に `KA_MAX_SITE`）。
   - 通信しない: 計測（config.json・beacon）と票（取得・投票ボタン・票の案内）を出さない。プライバシーポリシーの代わりに「ローカル版は計測・投票の通信をしない」と出す（5 言語）。
   - サイト側: KA の説明に「100 を超えるときはローカル版（Releases）」の案内を足す（D10）。
2. **Go の単一バイナリ** `cmd/local`（標準ライブラリのみ）: ローカル版の dist（データ・WASM 込み）を `embed` し、`127.0.0.1` だけで配る。`_headers` を同じ規則で読んでヘッダを付ける（CSP 下で WASM・Worker が動くこと）。Content-Type は固定の表（Windows の registry の MIME に頼らない）。起動したらブラウザを開く（`-open=false` で止める）。Host の検査・タイムアウト・Ctrl+C で終了。配信の処理は `internal/localserver`（`fstest.MapFS` で単体試験）。
   - 埋め込む中身は `scripts/local/build.sh` が `cmd/local/site/` に置く（gitignore。プレースホルダ 1 つだけコミット。中身が無いバイナリは起動時に理由を出して終わる）。
3. **Releases の workflow** `.github/workflows/local-release.yml`: ubuntu で 1 回ビルド（5 種: linux amd64/arm64・darwin amd64/arm64・windows amd64）→ ubuntu・macos・windows で**起動確認**（実バイナリを起動し、index・manifest・WASM・ヘッダを HTTP で確かめる）→ タグ（`v*`）の push のときだけ **draft の Release** を作る（公開は開発者が押す＝AGENTS §3-3）。PR・`claude/**` への push では関係するパスが変わったときだけ走る（必須 check ではないので paths フィルタ可）。
4. **govulncheck**: `scripts/govulncheck.sh`（版 v1.9.0 固定。対象は `./...`・WASM・ローカル版を windows/darwin/linux で）。CI の Go job に step 1 つ＋summary、`.github/workflows/govulncheck.yml`（週 1・手動）。
5. **E2E**: Playwright に project `local` を足し、実バイナリを起動して KA 120 で提案が出る・計測と票の通信が無いことを確かめる。CI の Web job にローカル版のビルドの段を足す（job は増やさない）。
6. docs: `docs/local.md`（使い方・ビルド・配布・署名なしの警告）、README、`docs/release-checklist.md`（S9 の手番: Release のタグ、About の Website）。

### 回す検査
- `go test -count=1 ./...`・`go vet`・`gofmt -l`、`GOOS=js GOARCH=wasm` のテスト（変更が無ければ省略しない範囲で）
- `pnpm exec tsc --noEmit`・`vitest --run`・`pnpm build`・ローカル版のビルド・Playwright（サイト＋local）
- `scripts/govulncheck.sh`（正常と、古い toolchain でわざと赤）
- `check-licenses.py`・`check-private-refs.py`・`check-actions-pinned.sh`
- 3 OS の起動確認は local-release.yml の結果（push 後に GitHub で確かめる）

### 未決事項と扱い
- KA の上限の値（無制限の表し方）: 自分で決める（上の推測）。
- Release の作成（タグの push）: 公開に関わるので開発者（S9 の手番として書く）。workflow は draft を作るまで。
- コード署名（macOS の公証・Windows の署名）: 費用がかかるのでやらない。警告の回避手順を docs に書く。

## やったこと

### 成果物
| 物 | 中身 |
|---|---|
| `cmd/local`（単一バイナリ） | ローカル版のフロント（データ・WASM 込み）を `go:embed` し、`127.0.0.1` だけで配る。既定ポート 8787（使われていれば空きポート）・`-port`・`-open=false`・`-version`・Ctrl+C で終了。中身の無いバイナリは理由を出して終わる |
| `internal/localserver` | 配信の本体。`_headers` を `web/scripts/headers.mjs` と同じ規則で読む・固定の Content-Type 表・Host の検査（DNS rebinding）・`_headers` とドットで始まる名前は配らない・GET/HEAD のみ |
| `web/src/lib/flavor.ts` ほか | `vite build --mode standalone` でローカル版。KA の上限なし（`KA_MAX_LOCAL` = 2147483647）・計測と票の通信なし・票のボタンと案内なし・フッターに「通信しない」・見出しに「ローカル版」。サイトは KA の説明にローカル版（Releases）への案内を足した。新しい文言 5 キー × 5 言語 |
| `scripts/local/build.sh`・`smoke.sh` | ビルド（データ・WASM → vite → 埋め込み → cross compile）と、実バイナリの起動確認（12 項目） |
| `.github/workflows/local-release.yml` | ubuntu で 5 種をビルド → ubuntu・macos・windows で配る圧縮を展開して起動確認 → タグ `v*` のときだけ draft の Release |
| `scripts/govulncheck.sh`・`.github/workflows/govulncheck.yml`・`ci.yml` | govulncheck v1.9.0 固定。対象は `./...`・WASM・ローカル版（windows/darwin/linux）。CI の Go job に 1 段、週 1 回（月曜 09:41 JST）＋手動の workflow |
| E2E `web/e2e/local.spec.ts` | Playwright の project `local`。実バイナリを起動し、KA 120・共有 URL の KA 110・KA 400 の中断を通す。外への要求が 0 件であることも見る。CI の Web job にローカル版のビルドの段を足した（job は 2 つのまま） |
| docs | `docs/local.md`（使い方・署名なしの警告・開発者向け。配る圧縮に README.md として同梱）、README、`docs/frontend.md`、`docs/release-checklist.md`（G-11 About の Website・§6 S9 の手番）、AGENTS §9 に workflow 2 本の 1 行 |

### 3 OS の起動確認（完了条件）
GitHub Actions の `Local release` run 39117681817（コミット `e96c14a`）。配る圧縮をダウンロード → SHA256SUMS で照合 → 展開 → 起動 → `scripts/local/smoke.sh`。

| OS（runner） | バイナリ | 結果 |
|---|---|---|
| ubuntu-latest | linux/amd64 | success |
| macos-latest | darwin/arm64 | success（ログ: 12 項目 ok・`失敗: 0 件`。WASM は `application/wasm`・キャラ 31 人） |
| windows-latest | windows/amd64 | success（ログ: 12 項目 ok・`失敗: 0 件`。WASM は `application/wasm`・wasm_exec.js は `text/javascript`） |

linux/arm64 と darwin/amd64 はビルドだけ（起動は確かめていない）。画面の操作（ブラウザでの計算）は Linux の E2E だけで見ている。

### 計測（予備。Node 上の WASM、ryu 重ねる・クエリ `mode=meaty` だけ）
KA 100 = 8.0 秒・428 MiB・55347 件 / KA 150 = 5.7 秒・646 MiB・25466 件 / KA 200 = 33.3 秒・605 MiB・106372 件 / KA 400 は 110 秒の打ち切りで未完了。KA 150 が KA 100 より速いのは件数が減るため（推測: 長い KA を埋める繋ぎの組み合わせが条件に合いにくくなる。原因未特定）。ingrid 汚連携 KA 100 は 3 分を超えたので止めた（原因未特定。S5 の画面での測定では最大 8.2 秒なので、画面と違うクエリのせいと推測）。測定はほかの処理と並行していない。

## 判断と理由

1. **ローカル版も計算はブラウザの WASM**（HANDOVER §5 のとおり）。案: Go のネイティブで計算する API をバイナリに持たせる（速い・WASM の 4 GiB の壁が無い）。→ 採らない。§5 に「計算はブラウザ内」とあり、サイトと同じ経路（同じ試験）で済む。KA 200 でも WASM のメモリは 605 MiB で、壁はまだ遠い。
2. **KA の上限 = エンジンが受ける最大（int32）**。案: 999 などの 3 桁で切る。→ D10・§9 の「KA 無制限」に合わせ、アプリでは数で切らない。重い条件は既存の注意（KA 80 以上）と中断で扱う（E2E で KA 400 を中断できることを確かめた）。推測: 実戦の KA が 100 を大きく超えることはまれ。
3. **ビルドの種類は Vite の mode（`standalone`）**。`local` は Vite が受け付けない。値はビルド時に決まるが、画面のコンポーネントと検査の関数は `flavor`・`kaMax` を引数で受けるようにした（試験で両方を回せる）。`edge/`（Worker）は `flavor.ts` を読まない（wrangler の束ねには `import.meta.env` が無い）。最初の実装では `shared/api.ts` → `lib/conditions.ts` 経由で読んでいた（レビュー高-1）ので、`lib/` の関数は flavor を読まない形に直し、`web/edge/imports.test.ts` で経路を検査する。念のため `import.meta.env?.MODE` にして、読んでもサイトとして動く。
4. **計測・票は「無い」として扱い、0 件や unavailable に寄せない**（AGENTS §8-7）。`RunState.votes` は `LoadedVotes | null`（null = 票を使わない版）。beacon は送らないので `votesStatus` を作らない。
5. **配信のヘッダはサイトの `_headers` をそのまま使う**。CSP・Permissions-Policy をサイトと揃えると、サイトの E2E で確かめた条件（WASM・Worker・コピー）がそのまま効く。HSTS は HTTP では無視される（RFC 6797 §8.1）ので外さない。
6. **Content-Type は固定の表**。Go の `mime` は Windows で registry を読み、`.js` が `text/plain` になる環境がある（module script と Worker が読めなくなる）。windows の起動確認で `text/javascript` を確かめた。
7. **Host の検査**: 配る中身は公開物だけなので漏れるものは無いが、別の名前から読ませない方が筋が良く、数行で済むので入れた。
8. **Release は draft まで**（公開は開発者＝AGENTS §3-3）。タグの push も開発者（S9 の手番として書いた）。
9. **コード署名はしない**（費用＝AGENTS §3-4）。OS の警告の回避手順を `docs/local.md` に書いた。
10. **govulncheck は `go install` で入れてから GOOS を変えて回す**（`go run pkg@v` に GOOS=js を渡すと道具そのものが js 向けになる）。`go.mod` の `tool` にはしない（依存が go.mod に入り、計算に効く部分の見張りと Dependabot の対象が増える）。版 v1.9.0 は 2026-10-11 公開（Dependabot の cooldown 7 日を満たす）。
11. **ローカル版の workflow は paths フィルタ付き**（必須 check ではない）。3 OS の runner を docs だけの変更で回さない。
12. 追加した Action: `actions/upload-artifact@cf430e0…`（v7.0.2）・`actions/download-artifact@9000827…`（v8.0.2）。どちらも 2026-10-07 のコミット（`git fetch` で日付を確かめた）。

### 逸脱
- 計画の「画面の条件での計測を実装後にやり直す」は**行わなかった**。数値は Node 上の予備計測（クエリが画面と違う）だけ。E2E で KA 120・110 が 90 秒以内に出ること、KA 400 を中断できることは確かめた。
- 計画の mode 名 `local` は `standalone` に変えた（Vite の制約。計画節に追記）。

### 版の固定と解除条件（AGENTS §5）
- `golang.org/x/vuln/cmd/govulncheck` v1.9.0（`scripts/govulncheck.sh`）。Dependabot はシェルの中の版を見ないので、上げるのは段階のセッションで手で（公開から 7 日以上の版）。解除条件は無い（道具の版が動くと同じコードで結果が変わりうる）。
- 追加した Action は SHA 固定（`upload-artifact` v7.0.2・`download-artifact` v8.0.2）。Dependabot（github-actions）が上げる。

## 残課題・既知の問題

- 3 OS の確認は HTTP まで。macOS・Windows の実ブラウザで画面を操作しての確認はしていない（開発者の手元で 1 度見てもらうとよい）。linux/arm64・darwin/amd64 は起動を確かめていない。
- 署名なしのため、macOS の Gatekeeper・Windows の SmartScreen の警告が出る（`docs/local.md`）。
- KA が大きいと計算は長い（KA 200 で 30 秒台。KA 400 は 2 分超）。中断はできる。メモリの上限（WASM の 4 GiB）に当たる KA は測っていない。
- manifest の `maxKnockdownAdvantage` は 100 のまま（ローカル版の画面は読まない。サイトの値の記録として残した）。
- サイトの KA の説明は Releases を指すので、最初の Release（S9-2）より前にサイトを出すと空のページを指す。
- 前のコミット `12de659` の CI は赤（`verify-plan` の ack 前。次の `e96c14a` で記録して緑）。

## 次の段階への入力（S9）

- `docs/release-checklist.md` の §5 G-11（About の Website。デプロイ後）と §6（S9-1〜4: Website・最初の Release のタグ・govulncheck.yml の手動起動・ヘッダ診断と「済」付け）。
- ローカル版の最初の Release は `v<版>` のタグを main で push → draft を確かめて公開（`docs/local.md`「配布」）。
- E2E は project が 2 つ（`site`・`local`）になり、手元で回すには先に `bash scripts/local/build.sh`（この OS 向けのバイナリ）が要る。
- govulncheck は `GOVULNCHECK_DB=https://storage.googleapis.com/go-vulndb bash scripts/govulncheck.sh`（このコンテナの proxy は vuln.go.dev を通さない）。

## 検査の結果（逐語。出力はファイルへ、合否は EXIT と件数で見た）

段階開始 `c6d5ded`、計測時の HEAD `9f22180`（作業木はその後 docs だけ）。

```
$ go test -count=1 -v -timeout 35m ./... > /tmp/s8-go.txt 2>&1; echo EXIT=$?
EXIT=0
PASS(全階層 ^\s*--- PASS)=358 FAIL=0 SKIP=0   (S7 の最終は 348)
$ go vet ./... → VET=0 / gofmt -l → 出力なし / go mod tidy -diff → TIDY=0
$ cd web && pnpm exec tsc --noEmit → EXIT=0
$ pnpm exec vitest --run → EXIT=0  Test Files 17 passed (17) / Tests 172 passed (172)   (S7 は 16 / 161)
$ pnpm build → EXIT=0(違反: 0 件)
$ bash scripts/local/build.sh build/local linux/amd64 windows/amd64 darwin/arm64 → EXIT=0
  tacpendium-setplay-local_linux_amd64: 12877986 bytes / _windows_amd64.exe: 13031424 bytes / _darwin_arm64: 12574626 bytes
$ bash scripts/local/smoke.sh build/local/tacpendium-setplay-local_linux_amd64 → EXIT=0  失敗: 0 件(12 項目 ok)
$ PW_CHROMIUM_PATH=/opt/pw-browsers/chromium-1194/chrome-linux/chrome pnpm exec playwright test → EXIT=0  28 passed (1.1m)   (site 25 + local 3)
$ GOVULNCHECK_DB=https://storage.googleapis.com/go-vulndb bash scripts/govulncheck.sh → EXIT=0  対象: 5 件 / 違反: 0 件
$ python3 scripts/check-licenses.py → EXIT=0 違反: 0 件 / python3 -I scripts/check-private-refs.py → EXIT=0 違反: 0 件(走査 283 ファイル)
$ bash scripts/check-actions-pinned.sh → EXIT=0 workflow: 5 本 / uses 参照: 24 件 / 違反: 0 件
$ python3 -I scripts/ci/verify-plan.py check → EXIT=0 違反: 0 件(ack --none を記録)
GitHub Actions: CI run 39117763196(9f22180)success / Local release run 39117681817(e96c14a)success
```

### わざと壊して赤を確かめた（AGENTS §8-3）

| 守るもの | 壊し方 | 結果 |
|---|---|---|
| ローカル版は通信しない（App） | `online = flavor === "site" \|\| true` | vitest「ローカル版」5 本のうち 2 本 × |
| ローカル版は KA の上限を外す | `kaMaxFor` が常に 100 を返す | 同 2 本 × |
| 実バイナリの通し（E2E local） | サイト版の dist を埋め込んだバイナリを `LOCAL_BIN` で渡す | 3 passed → 3 failed（EXIT=1） |
| 起動確認（smoke） | 中身を埋め込まないバイナリ | EXIT=1・失敗: 10 件 |
| govulncheck | `GOTOOLCHAIN=go1.26.4`（修正前の標準ライブラリ） | EXIT=1・違反: 4 件（./... とローカル版 3 OS。各 11 件の脆弱性。WASM は 0） |

### git diff --stat c6d5ded（報告を書く前の時点）

```
 42 files changed, 1592 insertions(+), 105 deletions(-)
```
（全文は下の「最終確認」に貼る。テストのファイルは追加のみで、消した試験は無い）

## レビュー

- 1 回目（`docs/progress/S8-review.md`）: 高 3・中 4・低 8。高はすべて採用して直した。中-2 は一部採用、低-8 は不採用（理由は review の「取り込み」）。
- 再レビュー: 高 1（高-3 の試験が元の不具合を捕まえない）・中 1・低 4。高・中・低-A〜C は採用、低-D は一部採用（「再レビューの取り込み」）。往復は 2 回で止めた。

## 最終確認（レビュー 2 往復の取り込み後、HEAD `b247ddb` で回した結果）

出力はファイルへ書き、合否は `EXIT=` と件数で見た。並行させずに順に回した。

```
$ SKIP_ASSETS=1 bash scripts/local/build.sh build/local linux/amd64 windows/amd64 darwin/arm64 → EXIT=0
  tacpendium-setplay-local_linux_amd64: 12877986 bytes / _windows_amd64.exe: 13033472 bytes / _darwin_arm64: 12574626 bytes / 版: b247ddb
$ cd web && pnpm build → EXIT=0(ファイル: 41(上限 20000)/ 違反: 0 件)
$ go test -count=1 -v -timeout 35m ./... → EXIT=0  PASS(全階層)=359 FAIL=0 SKIP=0   (S7 の最終 348)
$ go vet ./... → EXIT=0 / gofmt -l(追跡中の .go 全部)→ 出力なし / go mod tidy -diff → EXIT=0
$ pnpm exec tsc --noEmit → EXIT=0
$ pnpm exec vitest --run → EXIT=0  Test Files 18 passed (18) / Tests 174 passed (174)   (S7 の最終 16 / 161)
$ bash scripts/local/smoke.sh build/local/tacpendium-setplay-local_linux_amd64 → EXIT=0  失敗: 0 件
$ PW_CHROMIUM_PATH=/opt/pw-browsers/chromium-1194/chrome-linux/chrome pnpm exec playwright test → EXIT=0  28 passed (1.1m)   (site 25 + local 3)
$ GOVULNCHECK_DB=https://storage.googleapis.com/go-vulndb bash scripts/govulncheck.sh → EXIT=0  対象: 5 件 / 違反: 0 件
$ python3 scripts/check-licenses.py → EXIT=0 違反: 0 件
$ python3 -I scripts/check-private-refs.py → EXIT=0 走査: 292 ファイル / 違反: 0 件(試験 check_private_refs_test.py も EXIT=0)
$ bash scripts/check-actions-pinned.sh → EXIT=0 workflow: 5 本 / uses 参照: 25 件 / 違反: 0 件
$ python3 -I scripts/ci/verify-plan.py check → EXIT=0 違反: 0 件(試験 verify_plan_test.py も EXIT=0)
```

GitHub Actions（push 時点）: CI run 39117763196（`9f22180`）success、Local release run 39117681817（`e96c14a`。3 OS の起動確認）success。取り込み後のコミットの run は push 後に GitHub で確かめる（下の追記）。

### git diff --stat c6d5ded（報告のコミット前。テストのファイルは追加・増加のみ）

```
 .github/workflows/ci.yml                 |  19 +-
 .github/workflows/govulncheck.yml        |  40 ++++
 .github/workflows/local-release.yml      | 208 +++++++++++++++++++++
 .gitignore                               |   5 +
 AGENTS.md                                |   1 +
 README.md                                |   5 +-
 cmd/local/main.go                        | 152 +++++++++++++++
 cmd/local/main_test.go                   |  73 ++++++++
 cmd/local/site/.gitkeep                  |   0
 docs/frontend.md                         |   6 +-
 docs/local.md                            |  89 +++++++++
 docs/progress/S8-report.md               | 182 ++++++++++++++++++
 docs/progress/S8-review.md               | 307 +++++++++++++++++++++++++++++++
 docs/release-checklist.md                |  12 +-
 internal/localserver/localserver.go      | 220 ++++++++++++++++++++++
 internal/localserver/localserver_test.go | 187 +++++++++++++++++++
 scripts/ci/logic-version.json            |  36 +++-
 scripts/govulncheck.sh                   |  50 +++++
 scripts/local/build.sh                   |  43 +++++
 scripts/local/smoke.sh                   |  72 ++++++++
 web/e2e/local.spec.ts                    |  70 +++++++
 web/edge/imports.test.ts                 |  46 +++++
 web/edge/index.ts                        |   4 +-
 web/playwright.config.ts                 |  40 +++-
 web/src/components/App.test.tsx          |  74 +++++++-
 web/src/components/App.tsx               |  57 ++++--
 web/src/components/ConditionsPanel.tsx   |  35 +++-
 web/src/components/Footer.tsx            |  17 +-
 web/src/components/Guide.tsx             |   6 +-
 web/src/components/Results.tsx           |  66 ++++---
 web/src/lib/conditions.test.ts           |  13 ++
 web/src/lib/conditions.ts                |  11 +-
 web/src/lib/constants.ts                 |  19 +-
 web/src/lib/flavor.test.ts               |  24 +++
 web/src/lib/flavor.ts                    |  18 ++
 web/src/lib/formState.test.ts            |   9 +
 web/src/lib/formState.ts                 |  11 +-
 web/src/locales/en.json                  |   7 +-
 web/src/locales/ja.json                  |   7 +-
 web/src/locales/ko.json                  |   7 +-
 web/src/locales/zh-Hans.json             |   7 +-
 web/src/locales/zh-Hant.json             |   7 +-
 web/src/shared/api.ts                    |   8 +-
 web/src/styles.css                       |   1 +
 44 files changed, 2166 insertions(+), 105 deletions(-)
```
