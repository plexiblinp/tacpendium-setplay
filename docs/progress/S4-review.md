# S4 レビュー（事前計算 CLI と CI 組込み）

- 対象: ブランチ `claude/stage-4-HwJ9s` の HEAD `abd231bb1cfdf46a1e2d99c7f4f6a781a1c42e02`
- 基準コミット: `c842beab1d18e58a40a26c62f4fa2cdf593e6bec`（呼び出し時の指定。報告の計画節の記録とも一致）
- 手順: `.claude/commands/review.md`。読み取り専用。このファイル以外は変えていない。試すコードは `go test -overlay`（スクラッチ領域のファイルを仮想的に重ねる）で回し、リポジトリのファイルは触っていない

## 1. 要約

- 繋ぎ表の結合 `(*FillerTable).Suggest` は、サービスが渡しうる入力（nLo ≥ 1・発生 ≥ 1 ⇒ budgetMax ≤ KA ≤ MaxKA、AllowRepeat=true、MaxFillers=8、MaxResults=6000）では `SuggestContext` と同じ結果になる。順序・打ち切り・k=0 の行・窓の端をコードで確かめ、全 KA の突き合わせ（luke・lily）を自分でも回して不一致 0 だった。完了条件「生成物が上限内」も自分で回して確かめた（63 ファイル・最大 2,090,725 bytes・違反 0 件）。
- **ただし HEAD の CI は赤**。(a) Go job: `engine` パッケージが `go test` の既定 10 分の timeout を超えた（本家の正解を 2 経路で回すようにしたため）。(b) WASM job: GOMEMLIMIT を入れても `TestConformance_UpstreamOracle/characters/ryu` が out of memory で落ちた。報告にはこの CI の結果が無く、「CI」節を後で書くとしたまま。
- **docs と報告の主張がコードと食い違う**: 「表記・名前だけの変更では `sp` は 1 バイトも変わらない」「変わらないキャラは同じ名前でキャッシュが効く」は誤り。表の見出しに `dataVersion`（取込元の SHA）が入るので、上流を取り込むたびに全キャラの `sp`・`moves` の名前が変わる（実際に試して確かめた）。
- パーサは壊れた入力で panic する・メモリを使い切って落ちる（WASM の「例外を投げない」約束が破れる）。繋ぎだけモードの配線は、外しても検査が赤にならない。

## 2. 完了条件への適合

| 項目 | 判定 | 根拠 |
|---|---|---|
| HANDOVER §9 S4「生成物が上限内」 | **済** | 自分で `go run ./cmd/precompute` を回して EXIT=0・`違反: 0 件`・ファイル 63・最大 2,090,725 bytes（§5）。2 回出力して `diff -r` も一致。上限の検査は配信するデータだけが対象で、フロントのバンドルと WASM は含まない（S5 で見直す。低-3） |
| 事前計算 CLI | 済 | `cmd/precompute`。決定的・上限の検査・manifest がある |
| CI 組込み | **一部** | job は足してある。ただし HEAD の CI run 38118829815 は Go と WASM の job が赤（高-1・高-2）。事前計算 job はレビュー時点ではまだ回っていた（結果未確認） |
| データ版ハッシュ付きの出力 | 一部 | 中身の sha256 を名前にし、manifest に `dataVersion` を入れている。ただし「変わらないキャラは同じ名前」（開発者決定 3）は上流を取り込むたびに崩れる（高-3） |
| 計画: 全 KA の突き合わせ（S3 からの引継ぎ） | 済（手元）／未確認（CI） | 報告に逐語がある（1,514,495 呼び出し・不一致 0）。自分では luke・lily の全 KA を回して不一致 0。CI では未完了 |
| 計画: 表あり = 本家の正解（Go・WASM・Node） | Go・Node は済、WASM は CI で赤 | 自分で回した結果: Go は 10,416 条件・不一致 0、Node は 7/7 pass。WASM は CI で OOM（高-2） |
| 計画: 「最初の push で oracle の省略を確かめる」 | 済（CI の画面）／報告には書いていない | run 38116617101 で「本家の正解の再生成」が 8 秒の ✓。報告に summary の逐語は無い |
| 計画: changed.sh の入力（engine・cmd/precompute を含める） | 変えた（報告に書いていない） | 実装は `engine`・`cmd/precompute` を外し、`internal/model` を足した。突き合わせの試験はこの 2 つに依存しないので、中身は妥当。ただし計画からの変更を報告していない（低-1） |
| 報告の形（§8-1・§8-11） | **未** | `go test ./...` の EXIT と件数、WASM go test・node の逐語、`git diff --stat` が無い。「下の最終確認」「下の『CI』」は参照先の節が無い（高-4） |

## 3. 指摘

### 高

**高-1. Go job が `engine` の既定 10 分 timeout で落ちる** — `.github/workflows/ci.yml:98`
- 内容: `go test -count=1 -v ./...` に `-timeout` が無い。S4 で `TestConformance_UpstreamOracle` をエンジン・繋ぎ表の 2 経路にしたので、所要がおよそ 2 倍になった（手元 4 コアで `engine` 376 秒）。CI run 38118829815 の Go job では `panic: test timed out after 10m0s`（実行中 `TestConformance_UpstreamOracle`）、`FAIL github.com/plexiblinp/tacpendium-setplay/engine 600.044s`。job の `timeout-minutes` は 45 に上げたが、go test の timeout は上げていない。
- なぜ問題か: HEAD の CI が赤。報告にも書かれていない。
- 直し方の案: `go test -count=1 -v -timeout 40m ./...`。あわせて、ネイティブでは表あり経路の条件を絞る（表あり = 表なしは `TestTable_MatchesEngine` が全 KA で見ている）ことも考えられる。

**高-2. WASM job は GOMEMLIMIT を入れても out of memory で落ちる** — `scripts/wasm/go-test-exec.sh:15`、`engine/conformance_test.go:256-290`
- 内容: CI run 38118829815 の WASM job（GOMEMLIMIT と WASM での逐次化が入った後のコード）で `TestConformance_UpstreamOracle/characters/ryu` が `runtime: out of memory: cannot allocate 524288-byte block (4207345664 in use)` で落ちた。スタックは表なしの経路（`SuggestContext` → `searchDepth`、KA 0x64=100）。報告の判断 6 は「手元で再現 → 逐次＋GOMEMLIMIT で緑（1,680 条件）」としているが、CI では緑になっていない。
- なぜ問題か: CI が赤で、報告の判断 6 の結論（直った）を CI が裏付けていない。推測: GOMEMLIMIT は柔らかい上限なので、生きている量そのものが 4 GiB 近くまで伸びると効かない。原因はまだ特定できていない。
- 直し方の案: CI と同じ条件で、生きている量を測る（`runtime.ReadMemStats`）。KA 100 の重い条件を WASM で絞る（例: WASM では KA ≤ 80 の条件だけ、または 1 条件ごとに `runtime.GC()` / `debug.FreeOSMemory`）。表あり経路を WASM の conformance から外し、表は `TestTable_MatchesEngine` の WASM 版と Node に任せる。

**高-3. 「`sp` は 1 バイトも変わらない」「変わらないキャラは同じ名前（キャッシュが効く）」は誤り** — `internal/service/setplay/table.go:130`、`docs/precompute.md:20-23`、`docs/progress/S4-report.md:41`・判断 2
- 内容: 表の見出しに `DataVersion: c.DataVersion`（上流の SHA）が入る。moves DB は全ファイルに同じ `dataVersion` を持つ（`internal/movesdb/check.go:41` が全ファイルの一致を強制）。このため上流を取り込むと、データが変わらないキャラも含めて全キャラの `sp/*.bin` と `moves/*.json` の中身が変わり、名前も変わる。
- 確かめた方法: `data/moves` を 2 つ写し、一方の `dataVersion` だけを書き換えて `go run ./cmd/precompute -chars ryu,luke` を回した。`sp/ryu.9b3f2242bfd7fa74.bin` → `sp/ryu.8f756cadd62bb775.bin`、`sp/luke.d9edcbc759f5d928.bin` → `sp/luke.d0d76c21b8d00084.bin`（moves も 2 つとも名前が変わった）。
- なぜ問題か: コードと docs が食い違う。報告は開発者決定 3（変わらないキャラはキャッシュが効く）を満たしたとしているが、実際には取り込みのたびに全キャラを取り直す。
- 直し方の案: 表の見出しから `dataVersion` を外す（manifest にあるので記録としては足りる）。moves も、配信用に `dataVersion` を除いたバイト列にするか、名前のハッシュを `calcFingerprint`＋表示に効く列から取る。変えたら「1 キャラの dataVersion だけを変えても sp の名前が変わらない」試験を足し、`DataVersion` を見出しに戻すと赤になることを確かめる（§8-3）。

**高-4. 報告の「緑」に証拠が無い・参照先の節が無い** — `docs/progress/S4-report.md:126`・`:170`
- 内容: `go test -count=1 -v ./...` の EXIT と件数、WASM の go test・`node --test` の逐語、`gofmt`・`go vet`・`go mod tidy -diff`・ライセンス・Action 固定の検査結果、`git diff --stat c842bea` が無い（§8-1・§8-11）。「本家の正解…不一致 0（下の最終確認）」の「最終確認」節と、「結果は下の『CI』に追記する」の「CI」節が存在しない。§8-3 の表の「どれも戻した後に緑」にもコマンドと EXIT が無い。性能の表（エンジン vs 繋ぎ表）は、測り方のコード（ベンチマーク等）がコミットされておらず再現できない。
- なぜ問題か: 「緑」と書いた主張のうち、CI では実際に赤のもの（高-1・高-2）がある。
- 直し方の案: 完了報告で各コマンドの逐語を貼る。CI の run 番号と job ごとの結果を書く。測った方法（ベンチマークの名前、またはスクリプト）を残す。

### 中

**中-1. 壊れた表で panic・メモリ切れになる（WASM の「例外を投げない」が破れる）** — `internal/service/setplay/table.go:328`・`:315`・`:354`
- 内容（overlay の試験で確かめた）:
  - 技 code の数が 0 で、構成技が 1 つ以上の単位がある → `nCodes-1` が uint64 で桁あふれし、上限の検査が効かない。`t.codes[m]` で `panic: runtime error: index out of range [0] with length 0`。
  - 見出しの `maxKA` を 1,000,000,000 にする → `make([]tableBucket, (MaxKA+1)*(MaxK+1))` で `fatal error: runtime: out of memory`（recover できない）。`maxKA` を 1<<62 にすると積が桁あふれして小さくなる。今回はデータが足りず誤りで済んだが、バケット数が合う細工をすれば `Suggest` の `bucket()` が範囲外を読む。
- なぜ問題か: `loadTable` で Go が panic すると、WASM のプログラムごと終わる（以後の呼び出しも使えない）。表は CI で作るので、普段の危険は低い（推測）。それでも `ParseFillerTable` は「壊れた表を誤りにする」を約束している。
- 直し方の案: 見出しの検査に上限を置く（`MaxKA ≤ 1000`、`MaxK ≤ 64` 等）。各バケットは最低 2 バイトを使うので、`(MaxKA+1)*(MaxK+1) ≤ len(data)/2` も検査する。`nCodes == 0 && nm > 0` は誤りにする。`TestParseFillerTable_Broken` にこの 3 つを足す。行が辞書順に厳密に増えることの検査も、展開時（`decode`）にあると安全（低い優先度）。

**中-2. 繋ぎだけモードで表を使う配線は、外しても赤にならない** — `internal/service/setplay/service.go:366`
- 内容: `suggestFillerOnly(..., s.engineFor(characterCode, ka))` を `runEngine` に戻す変更を overlay で入れた。`engine` の `TestLoadTable*` と `cmd/precompute` の `TestRun_*` を回すと EXIT=0 で緑のままだった。`TestLoadTable_UsedBySuggest` は既定のクエリ（meaty）でしか表の誤りを見ていない。
- なぜ問題か: §8-5「配線を 1 行消すと赤になることを確かめる」に足りない。上流の差分を当てるときに、この差し替えが外れても気づけない。
- 直し方の案: `TestLoadTable_UsedBySuggest` に `mode=filler&remain=…` でも `ErrFillerTable` になることを足す（gap も同じように足す）。

**中-3. 取り消し済みの ctx でエンジンと結果が違う** — `internal/service/setplay/table.go:478` 付近（`Suggest`）
- 内容: エンジンは k ごとに `probe.check()` で ctx を見る。結合は `probe.tick()`（1024 回に 1 回）だけなので、取り消し済みの ctx でも結果の少ない呼び出しは結果を返す。overlay で確かめた: `pre-cancelled KA5: engine 0 context canceled / table 1 <nil>`。
- なぜ問題か: godoc の「SuggestContext と同じ入力を受け、同じ結果を返す」と食い違う。サービスは当てる技ごとに `probe.check()` を呼ぶので、画面の結果は変わらない（コードで確かめた）。`TestTable_Cancel` は当てる技を回し、どれか 1 つが誤りを返せば合格にしているので、この差を見落とす。
- 直し方の案: k のループの先頭で `probe.check()` を呼ぶ（エンジンと同じ形）。試験は 1 回の呼び出しで、エンジンと結合の誤りが一致することを見る。

**中-4. 繋ぎ表で速くならない（S4 の目的に関わる。開発者の判断が要る）**
- 内容: 報告自身が書いているとおり、結合はエンジンの 0.9〜1.6 倍で、配信が 26 MB（gzip 5.9 MB）増える。案 B を選んだ前提（S1 の「約 0.4 倍」）が masked では成り立たなかった。
- なぜ問題か: 完了条件（上限内）は満たすが、案 B を採る理由が無くなっている。報告は「開発者に判断してほしい」と書いており、それは適切。ただし測り方が残っていない（高-4）。
- 直し方の案: 測り方をベンチマークとしてコミットする。WASM でも 1 点測る。そのうえで、案 B を続けるか（表を配らず、エンジンだけにするか）を開発者に聞く（AGENTS §3 の「止めて聞く」2 に当たる）。

**中-5. CI の「全 KA の突き合わせ」は、入力が変わったブランチでは push のたびに 15 分以上回る** — `.github/workflows/ci.yml` の事前計算 job・`scripts/precompute/changed.sh`
- 内容: main 以外の基準は「origin/main との分岐点」なので、`internal/service/setplay` を触ったブランチでは、以後の push（docs だけの push も含む）で毎回全 KA を回す。リポジトリが private の間は Actions の分数が限られる（AGENTS §9）。
- 直し方の案: 当面はこのままでもよいが、所要を CI で測ってから決める（報告の残課題どおり）。推測: 基準を `github.event.before` にする手もあるが、取りこぼしの危険があるので、選ぶなら理由を書く。

### 低

- **低-1.** `scripts/precompute/changed.sh:12-18` の入力が計画と違う（`engine`・`cmd/precompute` を外し、`internal/model` を足した）。中身は妥当だが、報告に書いていない。`docs/precompute.md` にも入力の一覧が無い（oracle は `docs/engine.md` に一覧がある）。
- **低-2.** `cmd/precompute/main_test.go:41` の godoc は「かつ実際に表で数えていることを確かめる」としているが、`FillerTable(code) != nil` を見ているだけ（表が使われたことの証拠は `engine` の試験が持つ）。文言を合わせる。
- **低-3.** 上限の検査（`checkLimits`）は配信するデータだけを数える。S5 でフロントのバンドルと `setplay.wasm`（3,944,611 bytes）を同じ配信に載せるときは、合計で数え直す必要がある。S5 への引継ぎに書くとよい。
- **低-4.** `TestRun_OutputServesSameResults` は `c.SafeJump.True` を nil の確認なしで読む（テストのキャラは値を持つので今は落ちない）。
- **低-5.** 計画節は試験名を `TestTable_MatchesEngine_AllKA`、標本を「KA 12 点程度」としているが、実装は `TestTable_MatchesEngine`・9 点。完了報告の本文は実装と合っているので、計画節のままでよい（記録として）。
- **低-6.** `go test ./...`（手元）の `cmd/movesdb`・`internal/movesdb`・`internal/seedgen` の失敗 35 件は、すべて `.upstream/tacpendium` が無いことによる（`公開版 tacpendium が無い … bash scripts/fetch-upstream.sh で取得すること`）。S4 の変更とは関係ない。CI の Go job ではこの 3 パッケージは ok。

### 観点ごとの「指摘なし」

- `_reference/` の変更: 指摘なし（`git diff --stat c842beab HEAD -- _reference` が空）。
- 禁止事項（AGENTS §7）: 指摘なし。秘密情報・公式データの追加なし、combomgr の参照なし（差分 19 ファイルを読んだ。新しく入れた依存は標準ライブラリだけ）。
- i18n・文言: 対象外（S4 はフロントの文字列を変えていない。`web/` は差分に無い）。
- テストの減少: 指摘なし。差分は 2,272 行追加・36 行削除で、削除は `engine/conformance_test.go` の 1 経路のループを 2 経路のループにした部分と、置き換えた行だけ。消えた試験は無い。
- 結合の正しさ（順序・打ち切り・k=0・窓）: 指摘なし（中-3 の取り消しを除く）。確かめた点:
  - 単位は表とエンジンで同じ `(total, name)` 順（`sanitize`）。名前は表の中で一意なので、F_KA から表への写しは順を保つ。だから k ごとの辞書順は一致する。
  - 窓は `[max(0, budgetMax−band), budgetMax]` で、エンジンの `0 ≤ remaining ≤ band` と同じ。
  - k=0 の行は `lo == 0`（⇔ budgetMax ≤ band）のときだけ出る。
  - `MaxResults` は追加の前に見る（エンジンは追加の後に見て止まる。結果は同じ）。
  - `maxDepth` は同じ関数で求め、表の `MaxK` を超えれば誤り。
  - サービスでは nLo ≥ 1（`meatyBand` は 1 以上に丸め、`gapBand` は active+1 以上）・発生 ≥ 1 なので budgetMax ≤ KA。表の範囲外の誤りには届かない。
  - 表が組み違っていても、F_KA ⊆ U で total が一致すれば結果は正しく、一致しなければ `ErrFillerTable` になる。したがって、黙って行を欠く道は無い（指紋の照合は早めに止めるための保険）。

## 4. 良かった点

- U を「KA 0〜100 で本家の `buildFillerUnits` が作る単位の和集合」とし、F_KA は実行時に作る設計にした。これで本家の関数を変えず、表も小さく（最大 2.0 MiB）、masked の意味を保てた。単位上限に達する 705 セルが S1 と一致したことまで確かめている。
- 表で受けられない入力は `ErrFillerTable` で止め、黙ってエンジンへ戻さない。サービスもこの誤りを飛ばさない。
- 全 KA の突き合わせは、当てる技を種別・ダメージで絞らず、受理帯 5 通りと繋ぎだけの全 remain まで含む。`ran == 0` なら赤になる。§8-3 のわざと壊す確認も多い。自分でも k=0 の行を消す壊し方を試し、不一致 238 で赤になることを確かめた。
- CLI は決定的で、上限の検査は「ちょうどは 0 件・超過は 1 件」まで試験してある。出力先が空でないと止まる。
- WASM の OOM を最初の push で見つけ、変更前のコードの問題だと切り分けた（CI では直り切っていない＝高-2）。

## 5. 自分で回したコマンドと結果

出力はすべて `/tmp/claude-0/s4rev/` に書き、そのファイルから読んだ。

```
$ go test -count=1 -v ./... > gotest.txt 2>&1; echo EXIT=$?
EXIT=1
ok   cmd/precompute 41.503s / engine 376.130s / internal/repository/setplay 0.008s / internal/service/setplay 176.001s ほか
FAIL cmd/movesdb・internal/movesdb・internal/seedgen（35 件。すべて .upstream/tacpendium が無いため。低-6）
--- PASS 337 / --- FAIL 35 / --- SKIP 0
conformance_test.go:301: 突き合わせた条件: 10416(キャラ 31 ・エンジンと繋ぎ表の 2 経路 ・不一致 0)
table_test.go:197: 突き合わせた呼び出し: 133064(キャラ 31 ・KA 9 通り ・不一致 0 ・6000 件上限 9712 ・単位上限に達した (キャラ,KA) 54)
（並行して回したため、TestTable_MatchesEngine は 173.95s。報告の「約 60 秒」は単独で回した値と思われる＝推測）

$ SETPLAY_TABLE_SWEEP=all go test -count=1 -v -run 'TestTable_MatchesEngine/characters/(luke|lily)$' -timeout 30m ./internal/service/setplay/ > sweep2.txt 2>&1; echo EXIT=$?
EXIT=0
table_test.go:197: 突き合わせた呼び出し: 92415(キャラ 2 ・KA 101 通り ・不一致 0 ・6000 件上限 4766 ・単位上限に達した (キャラ,KA) 42)

$ go run ./cmd/precompute -out /tmp/claude-0/s4rev/site > pre.txt 2>&1; echo EXIT=$?
EXIT=0
キャラ: 31 / ファイル: 63(上限 20000) / 最大: sp/cammy.43a131ec10be29da.bin 2090725 bytes(上限 26214400)
合計: 26155797 bytes / gzip 5871109 bytes / 違反: 0 件
（2 回目の出力と diff -r: DIFF_EXIT=0）

$ bash scripts/wasm/build.sh > wbuild.txt 2>&1; echo BUILD_EXIT=$?
BUILD_EXIT=0   setplay.wasm: 3944611 bytes / gzip -9: 1087962 bytes
$ node --test scripts/wasm/wasm.test.mjs > node.txt 2>&1; echo EXIT=$?
EXIT=0   # tests 7 / # pass 7 / # fail 0 / # skipped 0
WASM 経由で突き合わせた条件: 504(不一致 0) / WASM 経由(繋ぎ表あり)で突き合わせた条件: 504(不一致 0)

$ gofmt -l（_reference を除く全 .go）→ EXIT=0・0 行 / go vet ./... → EXIT=0 / go mod tidy -diff → EXIT=0
$ python3 scripts/check-licenses.py → EXIT=0「違反: 0 件」 / bash scripts/check-actions-pinned.sh → EXIT=0「違反: 0 件」
$ bash scripts/precompute/changed.sh c842beab → EXIT=0・changed / HEAD → unchanged / deadbeef → changed

# わざと壊す（go test -overlay。リポジトリのファイルは変えていない）
k=0 の行を出さない（table.go の if を false &&）→ TestTable_MatchesEngine(luke|lily) EXIT=1・不一致 238 / 8113
繋ぎだけモードを runEngine に戻す（service.go:366）→ engine TestLoadTable* と cmd/precompute TestRun_* は EXIT=0（赤にならない＝中-2）

# 調べるための試験（overlay）
nCodes=0 の単位 → PANIC index out of range [0] with length 0
見出しの maxKA=1e9 → fatal error: runtime: out of memory（中-1）
取り消し済みの ctx・KA 5 → engine 0 件 context canceled / table 1 件 <nil>（中-3）
dataVersion だけを書き換えて -chars ryu,luke → sp・moves の 4 ファイルとも名前が変わった（高-3）

# CI（gh run view / GitHub の job ログ）
run 38118829815（HEAD abd231b）: Go ✗（engine: panic: test timed out after 10m0s・FAIL 600.044s）
  WASM ✗（internal/service/setplay ok 133.335s・TestTable_MatchesEngine 12720 呼び出し 不一致 0。
          engine: runtime: out of memory … (4207345664 in use)、TestConformance_UpstreamOracle/characters/ryu）
  事前計算: レビュー時点で実行中（結果未確認） / リポジトリ検査・本家の正解の再生成・Web ✓
run 38116617101（計画だけのコミット）: 本家の正解の再生成 ✓ 8s・WASM ✗（報告の判断 6 の OOM）
S4 のコードを含む他の run はすべて cancelled（concurrency による打ち切り）。S4 のコードで緑になった CI run はまだ無い。
```

WASM の `go test`（GOOS=js）と `SETPLAY_TABLE_SWEEP=all` の全キャラは、時間の都合で自分では回していない（WASM は CI のログで確認、全 KA は luke・lily だけ）。

---

## 取り込み（実装担当が追記）

| 指摘 | 採否 | 理由・対応 |
|---|---|---|
| 高-1 go test の 10 分 timeout | 採用 | `ci.yml` の go test に `-timeout 35m`。 |
| 高-2 WASM の out of memory | 採用 | 原因を `GODEBUG=gctrace=1` で確かめた: WASM の Go ヒープは最大 1.2 GB（生きている量は最大 885 MB）で、4 GiB まで広がったのは大きなスライスの連続領域を再利用できない**断片化**（推測にあった「生きている量が 4 GiB 近く」ではない）。CI と同じ呼び出し（`engine` パッケージ全体）を手元で再現 → `GOMEMLIMIT=3000MiB` でも落ち、`1500MiB` で EXIT=0（547 秒）。`scripts/wasm/go-test-exec.sh` の既定を 1500MiB にした。**追記**: 最終確認で 1500MiB でも 1 回落ちたので、WASM の本家突き合わせを KA 79 以下に絞った（KA 100 はネイティブと Node が見る。逸脱として報告に書いた）。手元で 2 回続けて緑。 |
| 高-3 dataVersion で全キャラの名前が変わる | 採用（表）・一部は残課題（moves） | 表の見出しから `dataVersion` を外した。`TestTable_RoundTrip` に「dataVersion だけ違う moves DB から同じバイト列の表」を足し、見出しに戻すと赤（`dataVersion だけが違う moves DB から違う表ができた`）を確かめた。moves は S2 の形が `dataVersion` を持ち、バイト列のまま配る設計なので取り込みのたびに名前が変わる（1 キャラ 60〜100 KB）。docs と報告を事実に合わせ、S2 の形を変えるかは残課題として開発者に回す。 |
| 高-4 証拠が無い | 採用 | 完了報告に最終確認（逐語・EXIT・件数・`git diff --stat`・CI の run）を足す。時間の測り方を `BenchmarkTableVsEngine` としてコミットした。 |
| 中-1 壊れた表で panic・OOM | 採用 | 見出しの `maxK ≤ 64`、バケット数 × 2 バイト ≤ ファイルの大きさ（確保の前）、構成技 0 個・技 code 0 個の単位を誤りに。`TestParseFillerTable_Hostile`（4 通り）を足し、検査を外すと `fatal error: runtime: out of memory` で赤を確かめた。 |
| 中-2 filler の配線が試験されない | 採用 | `TestLoadTable_UsedBySuggest` に `mode=filler` を足し、`suggestFillerOnly` に `runEngine` を渡すと赤（`mode=filler: … <nil>`）を確かめた。gap は meaty と同じループの同じ `run` を通るので足していない。 |
| 中-3 取り消し済み ctx | 採用 | k ごとに `probe.check()`（エンジンと同じ形）。`TestTable_Cancel` を 1 回の呼び出しでエンジンと比べる形にし、`MaxResults 5` を足した（間引いた確認より前に止まる件数）。外すと赤（`engine 0 件 context canceled / table 5 件 <nil>`）。 |
| 中-4 速くならない | 不採用（開発者の判断） | 案 B は開発者決定（S1 後 1）。続けるかは開発者が決める。完了報告の残課題と最終応答に書く。測り方は高-4 のベンチマーク。 |
| 中-5 全 KA が push ごとに回る | 不採用（今は） | CI での所要を測ってから決める（S3 で `before` を基準にしない理由＝打ち切りでの取りこぼし、と同じ事情）。完了報告の残課題に書く。 |
| 低-1 changed.sh の入力が計画と違う | 採用 | 報告に書く（突き合わせの結果を決めるのは表・結合・エンジンのコードとデータで、`engine/`・`cmd/precompute` は関係しない。`internal/model` は計算が使う定数）。 |
| 低-2 試験の godoc | 採用 | 文言を「表が読み込まれ、表なしと同じ結果」に直し、表で数えている証拠は engine の試験だと書いた。 |
| 低-3 上限は data だけ | 採用 | S5 への引継ぎに書く。 |
| 低-4 SafeJump の nil | 採用 | nil なら Fatal。 |
| 低-5 計画節の名前 | そのまま | 記録として残す（指摘のとおり）。 |
| 低-6 手元の movesdb 等の失敗 | 対応不要 | 最終確認は `scripts/fetch-upstream.sh` で取得してから回す。 |

## 再レビュー（2 往復目）

対象: `abd231b..HEAD`（87e81ea・6049ad4・bfa049e）。取り込み表の各行を、コード・docs・完了報告と照らした。重い検査（全体の go test・WASM・node・sweep）は回していない（機械が別の長い試験で使用中のため）。下の「回したコマンド」は軽い対象だけ。

### 高

**高-1（前回の高-4 の残り）. 完了報告に「最終確認」「CI」の節がまだ無い** — `docs/progress/S4-report.md:126`・`:173`
- 内容: 本文は「逐語は末尾の『最終確認』」「CI での所要は末尾の『CI』」と参照しているが、報告は 184 行で終わり、見出しは「次の段階への入力」が最後（`grep -n '^## \|^### '` で確かめた）。`go test ./...` 全体・WASM・`node --test` の逐語と EXIT、`gofmt`/`go vet`/`go mod tidy -diff`/ライセンス/Action 固定、`git diff --stat c842beab`、CI の run 番号が無い。`GOMEMLIMIT=1500MiB` が CI（`engine` パッケージ全体）で通ったこと（前回の高-2 の本当の確認）もまだ書かれていない（判断 6 の EXIT=0 は手元の値）。
- なぜ問題か: 前回の高-4 は「採用」になっているが、まだ果たされていない。§8-1・§8-11。推測: 今回の長い試験の実行がこの節のためのもので、これから書く予定と思われる。
- 直し方の案: 最終確認と CI の節を書き、そのあと検査をもう一度回して報告と一致することを確かめる（§8-11）。CI の WASM job の結果（1500MiB で engine が通ったか）を必ず入れる。

### 中

**中-1. 壊れた表で、まだ panic と巨大な確保が起きる（前回の中-1 の取りこぼし。場所はバケットの展開）** — `internal/service/setplay/table.go:392-399`・`:387`
- 内容:
  - `decode` で先頭行の共通接頭長 `lcp` が 0 でないと `r.err` を立てるが、そのまま `for j := 0; j < lcp; j++ { sum += t.units[prev[j]].total }` を回す。単位が 0 個の表（`units:0` は `ParseFillerTable` を通る）で `T=0,k=1` のバケットを「行数 1・長さ 1・中身 [1]」にすると、`panic: runtime error: index out of range [0] with length 0`。overlay で足した試験で確かめた（下の o1）。単位が 1 個以上あっても、先頭行の lcp≠0 は `prev` の 0 を読んで誤りへ進むだけなので、panic は単位 0 個のときに限る。
  - `rows := make([]uint32, 0, b.count*k)` の `b.count` は `uvarint(size)`（ファイル全体の大きさ）までしか縛っていない。バケットの中身が数バイトでも、行数を 200 万・k=64 にすると約 512 MB を確保する（WASM では 4 GiB の上限に近づき、recover できない out of memory になりうる）。推測: 実際には確かめていない（確保が大きいので回していない）。
  - `ParseFillerTable` 側の `names := make([]string, 0, nm)` も `nm ≤ size` なので、ファイルの大きさ × 16 バイトまでは確保しうる（表は最大 2 MiB なので 32 MB 程度。低い）。
- なぜ問題か: 取り込み表の中-1 は「壊れた表で panic・OOM しない」として採用したが、見出しと単位表だけを直し、`decode`（Suggest から初回に呼ばれる＝WASM ではプログラムごと落ちる）は直っていない。`TestParseFillerTable_Hostile` は `ParseFillerTable` しか見ないので、この道を通らない。リポジトリに `recover()` は 1 つも無い（`grep -rn "recover()" --include=*.go`、`_reference` 除く）。表は CI で作るので普段の危険は低い（推測）。
- 直し方の案: `decode` で `lcp` の誤りを立てたら `break` する（または `if r.err != nil { break }` を lcp の加算の前に置く）。`ParseFillerTable` のバケットの読み込みで `k ≥ 1 なら count ≤ len(bucket data)`（1 行は最低 1 バイト）、`k = 0 なら count ≤ 1` を検査する。`nm` は残りのバイト数以下を検査する。`TestParseFillerTable_Hostile` に「読めたあと `decode` で誤り（panic しない）」の 2 通りを足し、lcp の break を外すと panic で赤になることを確かめる。

**中-2. `TestTable_Cancel` の「途中の取り消し」は、行の内側の確認（`probe.tick()`）をもう守っていない** — `internal/service/setplay/table_test.go:351`・`:355`
- 内容: 今回 k ごとの `probe.check()` を足したので、`cancelAfter{after: 3}` は 4 回目の `Err()`＝k=3 の先頭の確認で取り消され、そこで止まる。コメントの「結果の行の内側で取り消しを見ている」はもう成り立たない。overlay で `if probe.tick() {` を `if false && probe.tick() {` に変えても、`TestTable_Cancel` は `--- PASS`・EXIT=0 のまま（下の o2）。
- なぜ問題か: 行の内側の確認が外れても赤にならない（§8-3）。WASM で重い条件（KA 100 で 1 回 1 秒前後）を中断ボタンで止める S5 の前提が、試験で守られていない。前回の中-3 の直しが、既存の試験の狙いを黙って弱めた（新しい問題）。
- 直し方の案: `after` を「k の先頭の確認の回数（maxDepth+1）＋ 1」より大きくする、または k の先頭では数えず tick 経由だけ数える ctx にする（例: `Err()` の呼び出しが k の先頭の回数を超えたら取り消す）。`if false && probe.tick()` の壊し方で赤になることを確かめる。

### 低

- **低-1.** `docs/progress/S4-report.md:32`（計画節）の表の見出しに `dataVersion` が残っている。計画節は記録として残す方針（前回の低-5）なので直さなくてよいが、§8-8 の grep で引っかかる。`docs/precompute.md:56`・`table.go` の godoc・判断 2 は実装と合っている（`dataVersion` の grep で確かめた。`cmd/precompute/main.go` の `dataVersion` は manifest のもので正しい）。
- **低-2.** 完了報告の「§8-3 わざと壊して赤を確かめた」表（`S4-report.md:128-141`）に、今回の 4 つの赤（`TestParseFillerTable_Hostile` の OOM、`mode=filler` の配線、取り消し、`dataVersion` を見出しに戻す）が無い。取り込み表にしか書かれていない。完了報告を読む次のセッションのため、表に足すとよい。
- **低-3.** 完了報告に go test の `-timeout 35m`（前回の高-1 の直し）が出てこない（`grep -n '35m'` で 0 件）。「やったこと」の CI の行（`:95`）は job の timeout 45 分だけを書いている。
- **低-4.** `BenchmarkTableVsEngine`（`table_test.go:432`）は `t := &testing.T{}` を `loadTableTestChars` に渡す。データが読めないときの `t.Fatal` はゼロ値の T で `runtime.Goexit` を呼ぶので、ベンチマークが理由を出さずに終わる（推測: 回していない）。`loadTableTestChars` を `testing.TB` を受ける形にして `b` を渡すとよい。
- **低-5.** `GOMEMLIMIT=1500MiB` は生きている量の最大 885 MB に対して 1.7 倍の余裕。データが増えて生きている量が 1500MiB を超えると、OOM ではなく GC の空回り（CPU の上限 50%）で遅くなる（推測）。WASM job が急に遅くなったときの見当として、`go-test-exec.sh` のコメントか報告の判断 6 に一言あるとよい。

### 取り込み表の行ごとの確認

| 前回の指摘 | 確認結果 |
|---|---|
| 高-1 timeout | 正しい。`ci.yml:99` に `-timeout 35m`、job は `timeout-minutes: 45`（`:59`）で順序も合う。証拠は CI 待ち（高-1）。 |
| 高-2 WASM OOM | 既定値の変更は正しい（`go-test-exec.sh:16`。`ci.yml` 側で `GOMEMLIMIT` を上書きしていない＝grep 0 件、`build/wasm/setplay.wasm` のバイナリ一致を除く）。古い 3000MiB の記述は「経緯」としてだけ残る。CI での確認はまだ（高-1）。 |
| 高-3 dataVersion | 表は正しい。`calcFingerprint` は movesdb で「計算に効く列だけ」（`internal/movesdb/model.go:67`・`:78`）なので `dataVersion` を間接にも含まない。`TestTable_RoundTrip` の追加は、`BuildFillerTable` が `c.DataVersion` を見出しに戻すと赤になる形で妥当。moves を残課題にしたことは docs・報告で一致。 |
| 高-4 証拠 | 未了（高-1）。ベンチマークのコミットは確かめた。比 0.8〜1.7 倍は表の値（12/19 … 1062/875）から 0.82〜1.67 で合う。 |
| 中-1 壊れた表 | 見出し・単位表は正しい（`MaxKA+1 ≤ len` を先に見るので、積は 64 ビットで桁あふれしない）。`decode` が残る（中-1）。 |
| 中-2 filler の配線 | 正しい。`engine/table_test.go` の追加は `suggestFillerOnly` へ表を渡す 1 行を守る。gap を足さない理由（同じ `run`）も妥当。 |
| 中-3 取り消し | 直しは正しい。k の先頭の確認を外すと `取り消し済み MaxResults 5: engine 0 件 context canceled / table 5 件 <nil>` で赤（o3。取り込み表の逐語と一致）。ただし途中の取り消しの試験が弱くなった（中-2）。 |
| 中-4・中-5 | 不採用の扱いと残課題の記述は一致。 |
| 低-1〜低-6 | 低-1 は判断 7、低-3 は次の段階への入力の最後、低-2・低-4 はコードで確かめた。 |

### 回したコマンド（出力は scratchpad の `r2/` に書き、そのファイルから読んだ）

```
$ go test -count=1 -run 'TestTable_(Cancel|RoundTrip)$|TestParseFillerTable' -v ./internal/service/setplay/ > o0.txt 2>&1; echo EXIT=$?
--- PASS: TestTable_RoundTrip (0.05s) / TestTable_Cancel (0.20s) / TestParseFillerTable_Hostile (0.00s) / TestParseFillerTable_Broken (0.03s)
EXIT=0
$ gofmt -l internal/service/setplay engine cmd/precompute → 出力なし EXIT=0
$ go vet ./internal/service/setplay/ ./engine/ ./cmd/precompute/ → EXIT=0
# o1: overlay で試験を 1 本足す（単位 0 個・T=0,k=1 に lcp=1 の 1 行）→ decode を呼ぶ
r2probe_test.go:21: PANIC in decode: runtime error: index out of range [0] with length 0
EXIT=1
# o2: overlay で table.go の `if probe.tick() {` → `if false && probe.tick() {`
--- PASS: TestTable_Cancel (0.18s)
EXIT=0
# o3: overlay で k の先頭の `probe.check()` を外す
table_test.go:348: 取り消し済み MaxResults 5: engine 0 件 context canceled / table 5 件 <nil>
--- FAIL: TestTable_Cancel (0.24s)
EXIT=1
```
（overlay は `go test -overlay` で、リポジトリのファイルは変えていない）

## 取り込み（2 往復目・実装担当が追記）

| 指摘 | 採否 | 理由・対応 |
|---|---|---|
| 高（最終確認・CI の節が無い） | 採用 | 完了報告の末尾に「最終確認」「CI」を足す（検査を回し直してから）。 |
| 中（decode の panic・巨大な確保） | 採用 | 先頭行の共通接頭の誤り・varint の誤りで `break`。行数 > バケットのバイト数を展開の前に誤りにした。`TestFillerTable_DecodeHostile`（4 通り）を足し、守りを外すと `panic: runtime error: index out of range [0] with length 0` で赤を確かめた。 |
| 中（途中の取り消しの試験が効いていない） | 採用 | 途中の取り消しの試験を外し、`TestTable_CancelChecksInsideRows`（行の多い 1 回の呼び出しで、`Err` の呼び出しが手数ごとの回数より多い）にした。行の内側の確認を外すと赤（`Err の呼び出しが 6 回(手数ごとの 9 回より多いことを期待)`）。 |
| 低-1 計画節の dataVersion | そのまま | 計画の記録として残す（実装と違う点は本文の判断 2 に書いた）。 |
| 低-2 §8-3 の表に新しい赤が無い | 採用 | 完了報告の §8-3 の表に足した。 |
| 低-3 `-timeout 35m` が報告に無い | 採用 | やったことの CI に書いた。 |
| 低-4 ベンチマークの `&testing.T{}` | 採用 | `loadTableTestChars` を `testing.TB` にし、ベンチマークは `b` を渡す。 |
| 低-5 GOMEMLIMIT の余裕 | 採用 | 判断 6 に書いた（生きている量 885 MB に対し 1.7 倍。超えると OOM より先に GC が増えて遅くなる見込み＝推測）。 |
