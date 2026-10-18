# S4 事前計算 CLI と CI 組込み — 完了報告

- 段階開始コミット: `c842beab1d18e58a40a26c62f4fa2cdf593e6bec`
- ブランチ: `claude/stage-4-HwJ9s`（セッション開始時は detached HEAD だったため作成）

## 計画

### Context

HANDOVER §9 S4「事前計算 CLI と CI 組込み、データ版ハッシュ付きの出力」。**完了条件: 生成物が上限内**（Cloudflare 無料: 静的アセット 20,000 ファイル・1 ファイル 25 MiB。S1 ⑥）。

前提（読んだもの）:
- S1 後の開発者決定: **案 B（繋ぎ表）を masked で採る**（uncapped の表 ＋ 本家が単位上限に達する KA の単位集合）。本家を正とし追従（本家のファイル・関数名に合わせ、変更点は `docs/upstream-diff.md`）。再計算は必要な分だけ（技単位の指紋＝S2 で `calcFingerprint` として実装済み、キャラ単位で作り直し、出力のファイル名はキャラ単位の指紋）。
- S3 の引継ぎ: 計算の入口は `engine`。**本家一致は KA 8 点だけ → S4 で全 KA 0〜100 の突き合わせを置く**。最初の push で CI の「本家の正解の再生成」が「入力の変更なし: 作り直しを省いた」と出ることを確かめる（S3 の未確認事項）。
- 開始コミット: `c842beab1d18e58a40a26c62f4fa2cdf593e6bec`（detached HEAD → `claude/stage-4-HwJ9s` を作る）。

### 設計（自分で決めるもの。報告に「検討した案と理由」を書く）

#### 1. 表の中身（S1 案 B の簡素化。意味は masked と同じ）
- 単位の全体集合 **U = ∪_{KA=0..100} buildFillerUnits(KA+2)**（本家の関数そのまま・上限つき）。S1 の「上限を全部外した表」より小さく、どの KA の単位集合 F_KA も U に含まれる。
- 表 = 「U の単位の多重集合で、合計 T ちょうど・個数 k」の行（T 0〜100、k 0〜8 = `DefaultOptions().MaxFillers`）。(T,k) ごとに、U を (total, name) 昇順に並べた索引の辞書順。
- **KA ごとの単位集合は配らない**。実行時にサービスが今どおり `buildFillerUnits(ka+2)` で F_KA と `unitsTruncated` を作り（数 ms）、結合はその F_KA に含まれる単位だけの行に絞る。→ masked と同じ結果で、配るものが表だけになる。
- 結合（`runEngine` の代わり）: `SuggestContext` と同じ前処理（誤り判定・nMin・帯・budgetMax・`sanitize`・`computeMaxDepth`）→ k 昇順に、窓 T ∈ [max(0, budgetMax−band), budgetMax] のバケットを F_KA で絞ってマージ（辞書順）→ `MaxResults`(6000) で止める。F_KA の単位が U に無い／total が違うときは誤りを返す（黙って engine に戻さない）。AllowRepeat=false・k>8・budgetMax>表の上限も誤り。ctx は `cancelProbe` で見る。

#### 2. 置き場所と本家からの変更（最小）
- 新規 `internal/service/setplay/table.go`（単体版だけ。未公開関数 `sanitize`・`computeMaxDepth`・`buildFillerUnits`・`collectFillerSingles` を使うため同じパッケージ）: `BuildFillerTable(cands, derivations, maxKA)`・`MarshalBinary`・`ParseFillerTable`・結合 `(*FillerTable).Suggest(ctx, ka, fillers, target, opt)`。
- `service.go` の変更: `runEngine(...)` の 2 か所（meaty/gap のループ・`suggestFillerOnly`）を「キャラの表があり ka ≤ 表の上限なら表の結合、なければ runEngine」に差し替える。`service` に表の引き口（`TableSource` interface）を足し、`NewServiceWithTables` を追加（`NewService` は不変）。`docs/upstream-diff.md` に追記。
- `engine`: `LoadTable(data) (code, error)`・`LoadTableJSON`。表の見出しの `calcFingerprint` が読み込み済みの moves と違えば誤り（組み違いを読み込み時に止める）。`Suggest` の形は不変。
- `cmd/wasm`: `loadTable(bytes: Uint8Array)` を追加（`js.CopyBytesToGo`）。

#### 3. 表のファイル形式（バイナリ。読むのは Go/WASM だけなので TS に形を持たせない）
- `magic "TSPT"`・形式版・見出し JSON（character・calcFingerprint・dataVersion・maxKA・maxK・件数）・技 code の文字列表・単位表（total と構成技の索引）・バケット（T,k ごとに行数、行は直前の行との共通接頭長 ＋ 残りの索引を uvarint）。`encoding/binary` のみ。
- バケットは初回使用時に展開してキャッシュ（WASM のメモリを抑える）。

#### 4. 事前計算 CLI `cmd/precompute`
- `-moves data/moves -out build/site-data [-chars ryu,ken] [-max-ka 100]`。キャラごとに並行、出力は決定的（時刻を入れない）。
- 出力:
  - `manifest.json`（唯一の可変ファイル）: schemaVersion・dataVersion（上流 SHA）・maxKnockdownAdvantage・tableFormatVersion・characters[{code, name, calcFingerprint, safeJump, moves, movesBytes, table, tableBytes, units, rows}]
  - `moves/<code>.<sha256先頭16>.json`（data/moves のバイト列そのまま）
  - `sp/<code>.<sha256先頭16>.bin`（表）
- ファイル名は**中身の sha256**（キャラ単位の指紋）。判断: 入力の `calcFingerprint` を名前にすると、生成器（本家 `buildFillerUnits` の追従など）が変わったときに同名で中身が変わり immutable キャッシュが古いまま残る。中身のハッシュなら変わらないキャラは同名（決定的出力）＝開発者決定 3 の「変わらないキャラはキャッシュが効く」を満たし、生成器の変更でも壊れない。表の見出しに `calcFingerprint` を入れるので、表記・名前だけの変更（calcFingerprint 不変）では表は 1 バイトも変わらない。HANDOVER §3 の `/data/<dataHash>/` は使わない（decision 3 の見直し）。
- 全キャラの作り直し時間を測る。決定的で同名になるため「変わったキャラだけ作り直す」仕組みは、時間が CI で問題になる大きさ（目安 1 分超）でなければ作らず、理由を報告に書く（逸脱なら逸脱と書く）。
- **上限検査**（定数 `maxAssetFiles = 20000`・`maxAssetBytes = 25 << 20`）: 書いた後に出力を走査し、ファイル数・最大サイズ・合計・gzip 後の合計を出し、超えたら exit≠0。要約は `違反: N 件` の行で出す（CI summary が読む）。

#### 5. 検査（AGENTS §8）
- `table_test.go`: 符号化の往復、結合の境界（budgetMax=0・band<0・k=0 行・6000 件ちょうど・AllowRepeat=false の誤り・U に無い単位の誤り・取り消し）。
- **全 KA の突き合わせ**（S3 の引継ぎ）`TestTable_MatchesEngine_AllKA`: 31 キャラ × KA 0〜100 × 全 target（全種別・ダメージ 0 含む）× 帯 {meaty 最大・meaty N2〜4・gap G1〜13・gap G2〜5} ＋ filler 全 remain で、結合と `runEngine` の `[]Suggestion` を順序まで・`truncated` 判定まで比べる。通常の `go test` では標本（全キャラ × KA {0,1,25,50,63,64,67,70,80,90,99,100} 程度）、`SETPLAY_TABLE_SWEEP=all` で全 KA。
- **通し（本家の正解）**: `engine/conformance_test.go` を、表なし・表ありの両方で `upstream_oracle.json`（5,208 条件）と突き合わせる。WASM でも同じ（5 キャラ、表は試験内で作る）。
- **配線**: (a) CLI の出力 → manifest → `engine.LoadTable` → 表なしと一致（`cmd/precompute` のテスト）。(b) 表が実際に使われている証拠: 行を 1 つ抜いた表を読ませると結果が変わる（配線を消すと赤）。(c) calcFingerprint 違いの表は読込で誤り。(d) Node（`scripts/wasm/wasm.test.mjs`）: `go run ./cmd/precompute -chars` で 2〜3 キャラを出し、WASM の `loadTable` 後の結果 = 本家の正解。
- 決定性: CLI を 2 回回してバイト一致（テスト）。
- 上限検査: 上限を下げた設定で 1 ファイル超過 → 1 件と出る（§8-3「1 件ある入力で 1 件」）。
- §8-3 わざと壊す: マージ順を逆／絞り（F_KA）を外す／窓の端を 1 ずらす／service の差し替えを外す／Node で loadTable を外す → それぞれ赤を確認して戻す。

#### 6. CI（`.github/workflows/ci.yml`）
- 新 job `precompute`（事前計算）: CLI を回し、上限検査、summary に件数・容量・時間・違反件数。生成物はまだ配信しない（デプロイは S5 以降。公開物は CI からだけ＝§8-12）。
- 全 KA の突き合わせ（重い）は同じ job で、入力が変わったときだけ回す（判定 `scripts/precompute/changed.sh`、`scripts/upstream-oracle/changed.sh` と同じ作法: 入力 = data/moves・internal/service/setplay・internal/repository/setplay・engine・cmd/precompute・go.mod、基準は main への push なら before、他は origin/main との分岐点、手動は常に）。所要を測り、重ければデータだけの変更では `calcFingerprint` が変わったキャラに絞る。
- WASM job: node テストの前に `go` があるので precompute は node テスト内で回す。
- Action は既存の SHA 固定のものだけ使う（upload-artifact は入れない）。

#### 7. docs
- 新 `docs/precompute.md`（出力の形・ファイル名の規則・表の形式・CLI・検査）。`docs/engine.md`（表・`LoadTable`・WASM `loadTable`）。`docs/upstream-diff.md`（service.go の差し替え）。HANDOVER §3 の `/data/<dataHash>/` 記述との差は report に書く（HANDOVER は編集しない）。

### 未決事項と扱い
- 開発者に聞く必要のあるもの: なし（§1 決定の変更・費用・公開・秘密・ライセンスに当たらない）。
- 自分で決める: 上記 1〜6（U の取り方・KA ごとの単位集合を実行時に作る・中身ハッシュの名前・バイナリ形式・CI のゲート）。推測: 表は全キャラ合計で数十 MB 以内・最大キャラ 25 MiB 未満（S1 の上限つき表が JSON 103 MiB・最大 9 MiB だったため）。超えたら T 範囲で分割する。

### 進め方（/stage の手順）
1. ブランチ作成 → `docs/progress/S4-report.md` に計画節 → commit・push → CI で oracle の省略を確認。
2. table.go ＋ テスト → service/engine 差し替え → CLI → WASM/Node → CI → docs。節目ごとに commit・push（`feat(S4): ...`）。
3. 検査はファイルへ出力し `EXIT=` を取る:
   - `go test -count=1 -v ./... > /tmp/s4-go.txt 2>&1; echo EXIT=$?`
   - `SETPLAY_TABLE_SWEEP=all go test -count=1 -run TestTable_MatchesEngine_AllKA -timeout 60m ./internal/service/setplay/ > /tmp/s4-sweep.txt 2>&1; echo EXIT=$?`
   - `GOOS=js GOARCH=wasm go test ... -exec=scripts/wasm/go-test-exec.sh ./internal/service/setplay/ ./internal/repository/setplay/ ./engine/`
   - `bash scripts/wasm/build.sh && node --test scripts/wasm/wasm.test.mjs`
   - `go run ./cmd/precompute -out build/site-data`（時間・容量・違反 0）、`go vet`・`gofmt -l`・`go mod tidy -diff`・`check-licenses.py`・`check-actions-pinned.sh`
   - 計測は重い処理と並行させない（§8-6）。
4. 新しいサブエージェント（general-purpose, fork しない）で `.claude/commands/review.md` に従うレビュー → `S4-review.md` に取り込み節 → 直す（最大 2 往復）。
5. 完了報告 4 項目 ＋ `git diff --stat c842bea` ＋ 検査の再実行 → commit・push → ローカルとリモートの SHA 一致を確認して停止。

### 検証（完了条件の判定）
- CLI 出力のファイル数 ≤ 20,000・各ファイル ≤ 25 MiB（CLI の上限検査 `違反: 0 件` と CI の summary）。
- 結合 = エンジン（全 KA）・表あり = 本家の正解（Go/WASM/Node）がすべて緑。

---

## やったこと

成果物（基準 `c842bea` からの追加。全体の diff は末尾）:

- **繋ぎ表**（`internal/service/setplay/table.go`。単体版だけ）: 作成 `BuildFillerTable`・ファイルの形 `MarshalBinary`/`ParseFillerTable`・結合 `(*FillerTable).Suggest`（`SuggestContext` と同じ入力・同じ出力）。形と意味は `docs/precompute.md`。
- **サービスへの組込み**（`service.go` の最小の変更。`docs/upstream-diff.md` の S4 節）: `runEngine` の 2 か所を `s.engineFor(characterCode, ka)` に。表が無い・KA が表の範囲外ならエンジン（本家と同じ）。表の誤り（`ErrFillerTable`）は飛ばさず返す。
- **engine**: `LoadTable`・`LoadTableJSON`（`calcFingerprint` が moves DB と違えば `table_mismatch`。moves DB を読み直すと表は外れる）。`repository` の `Character` に `CalcFingerprint`。
- **WASM**: `loadTable(Uint8Array)`。
- **事前計算 CLI** `cmd/precompute`: `manifest.json`・`moves/<code>.<hash>.json`・`sp/<code>.<hash>.bin`（hash は中身の sha256 先頭 16 桁）。配信上限の検査（`違反: N 件`、超えたら exit 1）。
- **CI**: `事前計算` job（生成と上限の検査は毎回。全 KA の突き合わせは入力が変わったときだけ＝`scripts/precompute/changed.sh`）。WASM の go test に `GOMEMLIMIT`（下の「判断」）。go / WASM job の timeout を 45 分に、go test に `-timeout 35m`（engine が既定の 10 分を超えた。レビュー高-1）、WASM の go test を `-timeout 40m` に。
- **docs**: `docs/precompute.md`（新規）・`docs/engine.md`・`docs/upstream-diff.md`。

### 生成物（完了条件: 上限内）

```
$ go run ./cmd/precompute -out build/site-data > /tmp/s4-pre.txt 2>&1; echo EXIT=$?
EXIT=0
（キャラごとの明細 31 行は略。最大は cammy: units=2366 rows=796305 table=2090725）
キャラ: 31
ファイル: 63(上限 20000)
最大: sp/cammy.43a131ec10be29da.bin 2090725 bytes(上限 26214400)
合計: 26155797 bytes / gzip 5871109 bytes
生成: 3.19s(GOMAXPROCS 4)
違反: 0 件
```

- 2 回生成して `diff -r` で一致（決定的）。単位の全体集合 U は 1 キャラ 419〜2,987、行は 2.5 万〜79.6 万。
- S1 の見積もり（uncapped の表 282 MiB・最大 28.8 MiB で 25 MiB 超え）より大幅に小さい。U を「全上限を外した単位」ではなく「KA 0〜100 で本家が作る単位の和集合」にしたこと（最大 2,987 単位。S1 の uncapped は最大 119,466）と、バイナリ・接頭共有の符号化による。

### 突き合わせ（逐語）

```
# 全 KA（S3 の引継ぎ: 本家一致は KA 8 点だけ → 全 KA を S4 で担保）
$ SETPLAY_TABLE_SWEEP=all go test -count=1 -v -run TestTable_MatchesEngine -timeout 120m ./internal/service/setplay/ > /tmp/s4-sweep.txt 2>&1; echo EXIT=$?
    table_test.go:189: 突き合わせた呼び出し: 1514495(キャラ 31 ・KA 101 通り ・不一致 0 ・6000 件上限 104016 ・単位上限に達した (キャラ,KA) 705)
--- PASS: TestTable_MatchesEngine (930.56s)
EXIT=0
```
- 範囲: 31 キャラ × KA 0〜100 × 当てる技になりうる全技（種別・ダメージで絞らない）× 受理帯 5 通り（meaty 最大・meaty NMin 3・meaty N2〜4・gap G1〜13・gap G2〜5）＋ 繋ぎだけの全 remain（0〜KA−1）。`[]Suggestion` を順序まで、誤りまで比べた。
- 単位上限に達する (キャラ, KA) **705** は S1 ②-2 の 705 と一致。エンジンの 6000 件上限で切れる呼び出し 104,016 を含む。
- 本家の正解（5,208 条件）をエンジン・繋ぎ表の 2 経路で: 不一致 0。WASM（5 キャラ）・Node（3 キャラ）も表あり・なしで不一致 0（逐語は末尾の「最終確認」）。

### §8-3 わざと壊して赤を確かめた

| 壊し方 | 赤になった検査 | 結果 |
|---|---|---|
| マージの辞書順を逆（`lessRow` の `<` → `>`） | `TestTable_MatchesEngine`（luke・lily） | 不一致 3,641 / 8,113 |
| 窓の下端を 1 ずらす（`lo := budgetMax − band + 1`） | 同 | 不一致 4,306 |
| U を KA 100 の単位だけから作る（和集合をやめる） | 同 | 不一致 911（KA の低い単位が表に無く `ErrFillerTable`） |
| KA ごとの単位集合での絞りを外す（表の全行を使う） | 同 | 不一致 630 |
| サービスの差し替えを外す（`engineFor` が常に `runEngine`） | `engine` の `TestLoadTable_UsedBySuggest` | 「ErrFillerTable を期待したが <nil>」 |
| WASM の `loadTable` の登録を消す | `node --test`（loadTable の 2 本） | fail 2 |
| CLI の `-chars` の絞り（実装時に実際に起きたバグ: 絞りの集合を回しながら消していた） | `cmd/precompute` の `TestRun_OutputServesSameResults` | 赤（指定外のキャラまで出た）→ 直した |
| 上限検査（1 ファイルだけ超える入力） | `TestCheckLimits` | 1 件と出る（ちょうどは 0 件・ファイル数超過も 1 件） |
| （レビュー後）壊れた見出しの守りを外す | `TestParseFillerTable_Hostile` | `fatal error: runtime: out of memory` |
| （レビュー後）行の展開の守りを外す | `TestFillerTable_DecodeHostile` | `panic: runtime error: index out of range [0] with length 0` |
| （レビュー後）繋ぎだけモードに `runEngine` を渡す | `TestLoadTable_UsedBySuggest` | `mode=filler: 表に無い単位で ErrFillerTable を期待したが <nil>` |
| （レビュー後）手数ごとの取り消しの確認を外す | `TestTable_Cancel` | `取り消し済み MaxResults 5: engine 0 件 context canceled / table 5 件 <nil>` |
| （レビュー後）行の内側の取り消しの確認を外す | `TestTable_CancelChecksInsideRows` | `Err の呼び出しが 6 回(手数ごとの 9 回より多いことを期待)` |
| （レビュー後）表の見出しに dataVersion を戻す | `TestTable_RoundTrip` | `dataVersion だけが違う moves DB から違う表ができた` |

どれも戻した後に緑。

### 時間（§8-6: 他の重い処理と並行させず、同じプロセスで背中合わせ。4 コア・ネイティブ）

測り方は `BenchmarkTableVsEngine`（`go test -run '^$' -bench TableVsEngine -benchtime 3x ./internal/service/setplay/`）。当てる技になりうる全技 × 受理帯 5 通り、エンジン段だけ（refine・rank は含まない）。1 回あたり:

| キャラ | KA 40（エンジン / 表） | KA 80 | KA 100 |
|---|---|---|---|
| ryu | 12 ms / 19 ms | 715 ms / 682 ms | 1,236 ms / 1,076 ms |
| kimberly | 18 ms / 30 ms | 814 ms / 807 ms | 1,233 ms / 1,021 ms |
| cammy | 18 ms / 24 ms | 721 ms / 732 ms | 1,062 ms / 875 ms |

**繋ぎ表の結合はエンジンと同じくらいの時間で、速くならない**（0.8〜1.7 倍）。S1 の「結合はエンジンの約 0.4 倍」は絞らない結合（uncapped）の値で、masked では成り立たなかった。CPU プロファイル（cammy KA 100）では、時間の多くは結果の組み立ての割り当て・GC と `sanitize`（両経路で同じ）で、マージそのものではない。

## 判断と理由

1. **U を「KA 0〜100 で本家が作る単位の和集合」にし、KA ごとの単位集合は配らない。**
   - 検討した案: (a) S1 のとおり単位上限を全部外した表 ＋ 上限に達する KA の単位集合を配る、(b) 和集合 U の表だけを配り、単位集合は実行時にサービスが今までどおり作る。
   - (b) を選んだ。どの KA の単位集合 F_KA も U に含まれるので、F_KA で絞った結合はエンジンと一致する（S1 ②-2 の masked と同じ意味。全 KA で不一致 0・705 セルも一致）。配るものが表だけになり、本家の `buildFillerUnits` をそのまま使うので本家の関数を変えない。表も小さい（最大 2.0 MiB）。S1 の推測「上限に達しない KA の単位集合は表の単位から決まる」に頼らずに済む。
2. **ファイル名は中身のハッシュ**（`calcFingerprint` ではなく）。理由は `docs/precompute.md`。表の見出しには `calcFingerprint` だけを入れ、取込元の版（`dataVersion`）は入れない（レビュー高-3 で、最初は入れていて上流の取り込みのたびに全キャラの表の名前が変わることが分かり、外した）。これで表（配信の大半）は、計算に効く列が変わったキャラだけ名前が変わる＝開発者決定 3 の「変わらないキャラはキャッシュが効く」を満たす。生成器が変わったときに同名で中身が変わる事故も起きない。**moves のファイルは取り込みのたびに全キャラの名前が変わる**（S2 の形が各ファイルに `dataVersion` を持ち、バイト列のまま配るため。1 キャラ 60〜100 KB）。HANDOVER §3 の `/data/<dataHash>/` は使わない（決定 3 の見直しのとおり）。
3. **「変わったキャラだけ作り直す」仕組みは作らなかった**（開発者決定 3 の「表の作り直しはキャラ単位」との関係）。全キャラの生成が 3.2 秒で、出力は決定的なので、全部作り直しても変わらないキャラは同じ名前・同じバイト列になる（キャッシュの効き方は同じ）。重いのは全 KA の突き合わせ（15 分）の方なので、こちらを入力が変わったときだけ回す（`scripts/precompute/changed.sh`）。推測: データだけが変わったときに `calcFingerprint` の変わったキャラに絞る余地はあるが、CI の所要を見てから決める。
4. **表はバイナリ**（JSON にしない）。読むのは Go/WASM だけで TS に形を持たせる必要が無く、`encoding/binary` だけで済む。読み込みは見出し・単位表だけで、行のバケットは初めて使うときに展開する。
5. **表の誤りは黙ってエンジンに戻さない。** 組み違い（指紋違い）は読み込みで、表に無い単位は結合で `ErrFillerTable`。KA が表の範囲外（ローカル版の KA 100 超）だけは正規にエンジンで数える。
6. **WASM の go test に `GOMEMLIMIT=1500MiB`**（`scripts/wasm/go-test-exec.sh`）と、WASM ではキャラごとの試験を並行にしない。本ブランチ最初の push（計画だけの docs コミット 679cf94、run 38116617101）で、**変更前のコード**の WASM の `TestConformance_UpstreamOracle` が `fatal error: out of memory`（4 GiB 使用中）で落ちた。S3 の CI では通っていたので境目にあった。
   - 経緯: 最初は 3000MiB にし、手元で conformance だけを回して緑にしたが、CI（`engine` パッケージ全体）では落ちた（レビュー高-2）。CI と同じ呼び出しを手元で再現し、`GODEBUG=gctrace=1` で見ると、Go のヒープは最大 1.2 GB（生きている量は最大 885 MB）で、4 GiB まで広がったのは大きなスライスの連続領域を再利用できない断片化だった（wasm32 は 4 GiB が上限、Go は広げたメモリを返せず、GC はオブジェクトを動かさない）。1500MiB でヒープを小さく保つと、同じ呼び出しが EXIT=0（547 秒）。
   - **1500MiB でも足りなかった**: 最終確認で WASM の `engine` がもう一度 OOM で落ちた（ryu、4,207,083,520 bytes 使用中）。GC の時刻で結果が揺れる。そこで WASM の本家突き合わせを **KA 79 以下に絞った**（`engine/conformance_test.go` の `wasmConformanceMaxKA`）。KA 100 はネイティブ（全 31 キャラ・2 経路）と Node の通し（WASM 経由・3 キャラ・表あり／なし）が見る。絞った後、手元で 2 回続けて EXIT=0（1,470 条件・不一致 0・約 3 分）。これは**逸脱**（S3 からの WASM の検査範囲を狭めた）。GOMEMLIMIT は残した。
   - 推測: ブラウザ（S5）でも、KA 100 の重い条件 1 回で数百 MB〜1 GB を使い、断片化で落ちうる。S5 で 1 条件ごとに Worker を作り直す等の対策が要るかもしれない。
7. **`scripts/precompute/changed.sh` の入力は計画と変えた**（計画: data/moves・internal/service/setplay・internal/repository/setplay・engine・cmd/precompute・go.mod → 実装: `engine`・`cmd/precompute` を外し `internal/model` を足した）。全 KA の突き合わせ（`TestTable_MatchesEngine`）の結果を決めるのは、データと、表・結合・エンジンのコード（と計算が使う定数）だけで、`engine/`・`cmd/precompute` は関係しない。
8. **既定の go test の突き合わせは KA 9 点**（0,1,2,25,50,64,67,80,100。約 60 秒）。全 KA は 15 分かかるので CI の事前計算 job に分けた。WASM は KA 5 点・5 キャラ。

## 残課題・既知の問題

- **繋ぎ表で計算は速くならない**（上の「時間」）。正しさ（本家と同じ結果）は担保したが、ブラウザでの所要は表なしとほぼ同じ見込み（推測: WASM でも同じ比。未計測）。S1 が指摘したとおり、本家の時間の大部分はエンジンの後（提案の組み立て・refine・rank。KA 100 で 1 モード最大 7 秒）で、表はそこに効かない。**開発者に判断してほしい点**（下の最終応答にも書く）。
- 全 KA の突き合わせの CI での所要は末尾の「CI」。入力（`internal/service/setplay` 等）を触ったブランチでは、マージまで push ごとに回る（レビュー中-5。基準を「前回の push」にすると打ち切りで検査が抜けるので変えていない。負担が大きければ開発者が判断）。
- **moves のファイル名は上流の取り込みのたびに全キャラ変わる**（上の判断 2）。キャッシュを効かせるなら S2 の moves DB の形から `dataVersion` を外す（S2 の形の変更なので開発者の判断）。
- WASM job が長くなった（手元: setplay 84 秒・engine 548 秒・node 319 秒）。表あり・なしの 2 経路にしたため。WASM のメモリの余裕も小さい（判断 6）。
- Node の WASM 試験で、`loadTable` が `LoadTable` を呼ばずに成功だけ返す壊し方は試していない（登録を消す壊し方だけ）。`character_not_loaded` の検査はこの壊し方を捕まえる設計だが、赤を確かめていない。

## 次の段階への入力（S5 フロント）

- データの読み方: `manifest.json`（可変・短いキャッシュ）→ キャラごとに `moves`（JSON。表記・名前の表示と WASM の `load`）と `table`（`Uint8Array` で WASM の `loadTable`）。`moves`・`table` は immutable で配ってよい。`dataVersion` を画面のデータ版に使う。形は `docs/precompute.md`。
- WASM の手順: `load(moves)` → `loadTable(table)` → `suggest(code, ka, query)`。KA 0〜100 は表で、それより上（ローカル版）はエンジンで数える。表を読まなくても結果は同じ。
- 生成物は CI からだけ出す。デプロイ（Cloudflare）は未着手。CI の事前計算 job は `$RUNNER_TEMP/site-data` に作って検査するだけで、成果物として残していない。
- 計算時間の問題（上）は S5 で Worker・中断ボタンを作るときに効く。WASM のメモリ（判断 6）も同じ。
- 配信の上限検査（`cmd/precompute`）は data だけを数える。S5 でフロントのバンドルと `setplay.wasm`（約 3.9 MB）を同じ配信に載せるときは、全体で数え直す（レビュー低-3）。

## 最終確認（レビュー 2 往復の取り込み後に回し直した結果）

HEAD `9832c65` で回した（この節を足すコミットは本報告だけを変える）。出力はファイルへ書き、合否は `EXIT=` と件数で見た。`.upstream` は `bash scripts/fetch-upstream.sh` で取得済み。

```
$ go test -count=1 -v -timeout 35m ./... > /tmp/f-go.txt 2>&1; echo EXIT=$?
EXIT=0
PASS=381 FAIL=0 SKIP=0
    conformance_test.go:306: 突き合わせた条件: 10416(キャラ 31 ・エンジンと繋ぎ表の 2 経路 ・不一致 0)
    table_test.go:198: 突き合わせた呼び出し: 133064(キャラ 31 ・KA 9 通り ・不一致 0 ・6000 件上限 9712 ・単位上限に達した (キャラ,KA) 54)
$ GOOS=js GOARCH=wasm go test -count=1 -v -timeout 60m -exec="$PWD/scripts/wasm/go-test-exec.sh" ./internal/service/setplay/ ./internal/repository/setplay/ ./engine/ > /tmp/f-wasm.txt 2>&1; echo EXIT=$?
EXIT=0
PASS=173 FAIL=0 SKIP=0
    table_test.go:198: 突き合わせた呼び出し: 12720(キャラ 5 ・KA 5 通り ・不一致 0 ・6000 件上限 1602 ・単位上限に達した (キャラ,KA) 6)
    conformance_test.go:306: 突き合わせた条件: 1470(キャラ 5 ・エンジンと繋ぎ表の 2 経路 ・不一致 0)
$ bash scripts/wasm/build.sh
setplay.wasm: 3947021 bytes / gzip -9: 1088606 bytes
$ node --test scripts/wasm/wasm.test.mjs > /tmp/f-node.txt 2>&1; echo EXIT=$?
EXIT=0
# WASM 経由で突き合わせた条件: 504(不一致 0)
# WASM 経由(繋ぎ表あり)で突き合わせた条件: 504(不一致 0)
# tests 7 / # pass 7 / # fail 0
$ go run ./cmd/precompute -out /tmp/f-site > /tmp/f-pre.txt 2>&1; echo EXIT=$?
EXIT=0
キャラ: 31
ファイル: 63(上限 20000)
最大: sp/cammy.f12869ab92ac4780.bin 2090668 bytes(上限 26214400)
合計: 26154030 bytes / gzip 5869108 bytes
生成: 3.634s(GOMAXPROCS 4)
違反: 0 件
$ go vet ./...                                   → EXIT=0
$ gofmt -l（追跡中の .go、_reference 除く）          → 出力なし
$ go mod tidy -diff                              → EXIT=0
$ python3 scripts/check-licenses.py              → EXIT=0 違反: 0 件
$ bash scripts/check-actions-pinned.sh           → EXIT=0 違反: 0 件
$ bash -n（hooks・scripts・upstream の .sh）      → 誤りなし
```

全 KA の突き合わせ（`SETPLAY_TABLE_SWEEP=all`、1,514,495 呼び出し・不一致 0）は「やったこと」の逐語のとおり（`a46a183` 以降は結合のコードを変えていない: 変えたのは取り消しの確認の位置と壊れた表の守りだけで、CI の事前計算 job が `9832c65` でも全 KA を回した。下の CI）。

表の中身のハッシュは、見出しから `dataVersion` を外したので「やったこと」の最初の生成（`sp/cammy.43a131ec10be29da.bin`）から変わった（`sp/cammy.f12869ab92ac4780.bin`、2,090,725 → 2,090,668 bytes）。

削除されたファイル: 0（`git diff --diff-filter=D --name-only c842beab HEAD`）。テストファイルは追加と、既存の `engine/conformance_test.go` の 2 経路化・WASM の絞りだけ（消えた試験は無い）。

### CI

- 本ブランチ最初の push（679cf94・run 38116617101）: 本家の正解の再生成が「入力の変更なし」で省かれた（S3 の未確認事項を確認）。同じ run で、変更前のコードの WASM が out of memory（判断 6 の発端）。
- run 38118829815（abd231b）: 事前計算 job success（生成 8 秒、全 KA の突き合わせ 24 分 18 秒）。Go（10 分 timeout）・WASM（OOM）は失敗 → レビュー高-1・高-2 で直した。
- run 38122644135（9832c65）: **6 job すべて success**。リポジトリ検査・Go（go test 7 分 15 秒）・WASM（go test 3 分 52 秒・node 4 分 24 秒）・本家の正解の再生成（入力の変更なしで省略）・Web・事前計算（生成と上限の検査 9 秒、全 KA の突き合わせ 19 分 12 秒）。

### `git diff --stat c842beab1d18e58a40a26c62f4fa2cdf593e6bec HEAD`（HEAD = 9832c65）

```
 .github/workflows/ci.yml                  |  75 +++-
 cmd/precompute/main.go                    | 372 ++++++++++++++++++
 cmd/precompute/main_test.go               | 189 +++++++++
 cmd/wasm/main.go                          |  12 +
 docs/engine.md                            |  10 +-
 docs/precompute.md                        |  75 ++++
 docs/progress/S4-report.md                | 191 +++++++++
 docs/progress/S4-review.md                | 270 +++++++++++++
 docs/upstream-diff.md                     |  11 +
 engine/api.go                             |  23 ++
 engine/conformance_test.go                |  72 +++-
 engine/engine.go                          |  42 +-
 engine/table_test.go                      | 169 ++++++++
 internal/repository/setplay/repository.go |  18 +-
 internal/service/setplay/service.go       |  17 +-
 internal/service/setplay/table.go         | 633 ++++++++++++++++++++++++++++++
 internal/service/setplay/table_test.go    | 489 +++++++++++++++++++++++
 scripts/precompute/changed.sh             |  38 ++
 scripts/wasm/go-test-exec.sh              |  13 +-
 scripts/wasm/wasm.test.mjs                |  55 ++-
 20 files changed, 2735 insertions(+), 39 deletions(-)
```

## S4 後の開発者決定（2026-10-11）

報告後の対話で、開発者が次を決めた。

1. **計算方式は S5 で測ってから決める。** 繋ぎ表（案 B）は計算を速くしない（上の「時間」）。案 A（結果ファイル）は容量 5.7 GiB と、絞り込みで本家と食い違う（S1）ので採らない。S5 でブラウザ（WASM）の実際の所要を測り、C（表をやめ、WASM のエンジンだけで数える）か D（よく使う条件だけ、キャラ × KA ごとに最終結果を前もって作る）を決める。D は容量・ファイル数・票（D17）との整合を測る。それまで案 B は今のまま残す（本家と同じ結果は担保済み）。
2. **キャラごとの版。** 各キャラの moves の `dataVersion` を「そのキャラのデータが最後に変わった取り込みの SHA」にした。全体の版（lock）は `index.json` の先頭と manifest に置く。
3. **全 KA の突き合わせは、セットプレイに効く列の指紋（`calcFingerprint`）が変わったキャラだけ回す。** 表・エンジンのコードが変わったときは全キャラ。検査済みの組は CI のキャッシュに残し、同じ組は二度と回さない。
4. **重い検査も同じ仕組みで差分だけ回す**（Go の本家の正解との突き合わせ、WASM の突き合わせと Node の通し）。

### やったこと

- **キャラごとの版**（決定 2）: `movesdb.CarryDataVersions`（前の moves DB と、`dataVersion` を除いた中身が 1 バイトも違わなければ前の版を引き継ぐ。→ 開発者の確認で「計算に効く変更だけで上げる」に変えた。下の「目視レビュー後の変更」）。`cmd/movesdb -prev`（既定は出力先そのもの）。`index.json` の各キャラに `dataVersion`。検査は「40 桁の 16 進」に（前は「= lock」）。CI の再生成と `TestPipeline_RegeneratesCommittedData`・`TestRun_MatchesCommittedData` は前の版（コミット済み）から引き継いで比べる。manifest の各キャラに `dataVersion`。作り直した data/moves は `index.json` に各キャラの版が足されただけ（各キャラのファイルは 1 バイトも変わらない。今のデータは全キャラが lock の版）。
- **差分実行**（決定 3・4）: `scripts/ci/verify-plan.py`（鍵 = sha256(種類, キャラ, `calcFingerprint`, 関わるコードの git の木の SHA)）と試験 `scripts/ci/verify_plan_test.py`（10 本。リポジトリ検査 job で回す）。試験に `SETPLAY_TABLE_CHARS`・`SETPLAY_ORACLE_CHARS`（未設定は全キャラ、知らない code は誤り）。CI の Go・WASM・事前計算 job で、`actions/cache/restore`・`save`（v6.1.0、SHA 固定）を使い、通ったときだけ記録を保存する。`scripts/precompute/changed.sh` は消した（置き換え）。仕組みと限界は `docs/precompute.md` の「CI での差分実行」。

### 検証（逐語）

```
$ go test -count=1 -run 'TestCarryDataVersions|TestCheck' -v ./internal/movesdb/
--- PASS: TestCarryDataVersions (0.00s)   （ほか 4 本 PASS）
$ go run ./cmd/movesdb > /tmp/s4v-movesdb.txt 2>&1; echo EXIT=$?
EXIT=0
版が変わったキャラ(dataVersion = ffdb69e468a3f5c0b0e16255e68b6baa3266796d): 0 []
違反: 0 件
$ python3 -I scripts/ci/verify_plan_test.py
Ran 10 tests ... OK
# 実データでの判定（一覧ファイルを手元の一時ディレクトリに置いて）
sweep: 対象 31 キャラ / 検査済みの鍵 0 件 / 回す 31 キャラ
sweep: 記録 31 キャラ / 鍵 31 件
sweep: 対象 31 キャラ / 検査済みの鍵 31 件 / 回す 0 キャラ
（ken の calcFingerprint だけを変えた index で）sweep: … / 回す 1 キャラ  chars=ken
（同じ一覧で種類を oracle に）oracle: … / 回す 31 キャラ
$ SETPLAY_TABLE_CHARS=luke,ed go test -run 'TestTable_MatchesEngine$' -v ./internal/service/setplay/
    突き合わせた呼び出し: 7888(キャラ 2 ・KA 9 通り ・不一致 0 …)
$ SETPLAY_TABLE_CHARS=nobody …  → 「知らないキャラ map[nobody:true]」で赤
$ SETPLAY_ORACLE_CHARS=luke go test -run 'TestConformance_UpstreamOracle$' -v ./engine/
    突き合わせた条件: 336(キャラ 1 ・エンジンと繋ぎ表の 2 経路 ・不一致 0)
```

§8-3:
- 版の引き継ぎを外す（常に lock）→ `TestCarryDataVersions` が赤（`版が変わったキャラ [test edited broken added]、期待 [edited broken added]`）。
- 鍵から指紋を外す → `verify_plan_test.py` の 2 本が赤（`test_changed_fingerprint_runs_only_that_character`・`test_wasm_runs_all_its_characters_if_any_changed`）。

### CI での確認

| run | コミット | キャッシュ | Go | WASM | 事前計算 |
|---|---|---|---|---|---|
| 38126932809（1 回目） | 545f1c5（差分実行を入れたコミット） | 空 → 全部回した | success 9 分 42 秒（go test 9 分 19 秒。本家の正解との突き合わせを 31 キャラ） | success 5 分 50 秒（go test 2 分 20 秒・Node 3 分 14 秒） | success 18 分 21 秒（全 KA の突き合わせ 18 分 7 秒、31 キャラ） |
| 38128271353（2 回目） | a05fc94（docs だけ） | 検査済みの記録を復元 → 回す 0 キャラ | success 35 秒（go test 21 秒。突き合わせは省略） | success 55 秒（go test 35 秒。Node の通しの step は skipped） | success 12 秒（全 KA の突き合わせ・記録・保存の step は skipped） |

- どちらの run も 6 job すべて success。1 回目は 3 job とも、通った後に検査済みの記録を保存した（`検査済みの記録` と `actions/cache/save` の step が success）。2 回目は記録を復元し、3 job とも重い step が skipped（保存の step も skipped＝新しく記録するものが無い）。
- 2 回目の合計（Go・WASM・事前計算）は約 1 分 40 秒で、1 回目の約 34 分から減った。リポジトリ検査・本家の正解の再生成（入力の変更なしで省略）・Web は 1 回目と同じ。

## 目視レビュー後の変更（2026-10-11、開発者の指摘）

開発者の目視レビュー（PR #5）の指摘で、次を変えた。

1. **差分実行のきっかけからコードの変更を外した（案 A）。** 計算に影響しないロジック変更、特定の範囲だけ再計算すればよい修正、単体版には影響しない変更がありうるため（開発者の指摘）。
   - 鍵 = sha256(種類, キャラ, `calcFingerprint`, その種類の計算の版, そのキャラの追加の版)。コードの git の木の SHA は鍵から外した。
   - 計算に効くコード（`scripts/ci/logic-version.json` の `watched`）が変わると、リポジトリ検査 job の `verify-plan.py check` が赤になる。変更した人が `verify-plan.py ack` で影響を理由つきで記録する（`--none`／`--kinds … --chars …`／`--all`）。手順は `.claude/commands/sync_upstream.md`。
   - 安全網: Go job は本家の正解との突き合わせを KA 61 以下だけ全キャラで毎回回す（`SETPLAY_ORACLE_SMOKE_MAX_KA=61`）。手元 4 コアで 14 秒・7,812 回（KA 25 以下は 4 秒・5,208 回、KA 43 以下は 5 秒・6,510 回。61 にしたのは、6000 件の上限・単位の上限が効き始める KA 64・67 の直前まで毎回見るため）。
   - 鍵の形が変わったので、この変更の後の最初の CI は全部を 1 回回す（`logic-version.json` の log の初期化の行）。
2. **見張る範囲を広げた（指摘 2「未来のことは判断しにくいので何か残す」への対応）。** `watched` を `internal/`・`engine/`・`cmd/`・`go.mod`・`upstream/tacpendium.lock`・`scripts/wasm/` にした。計算に効くコードを新しい場所に置いた場合も、この範囲の中なら `check` が止める。範囲の外（新しいトップレベルのディレクトリ）に置いたときは `watched` に足す必要があり、`docs/precompute.md` と `logic-version.json` の `_comment` に書いた。
3. **キャラごとの版は「計算に効く変更」だけで上げる。** `CarryDataVersions` の比較を、ファイルの中身の一致から `calcFingerprint` の一致に変えた。表記・名前だけの変更では版を上げない（ファイルの中身は変わるので、配信のファイル名は変わる）。
4. 指摘 3 は問題なし。指摘 5（S5 で使う形）は開発者が確認中。

§8-3:
- 版: 指紋を見ない（常に引き継ぐ）→ `TestCarryDataVersions` が赤（`版が変わったキャラ [broken added]、期待 [edited broken added]`）。常に lock → 赤（`[test renamed edited broken added]`）。
- ack: 記録を更新しない → `test_none_records_trees_and_runs_nothing` が赤。キャラの版を上げない → `test_chars_reruns_only_those_characters_and_kinds` が赤。

### 目視レビュー後の変更の CI での確認

| run | コミット | Go | WASM | 事前計算 |
|---|---|---|---|---|
| 38134299436（1 回目） | 3408846（鍵の形を変えたので全部回る） | success 約 6 分（go test 5 分 33 秒。全キャラ） | success 約 9 分（go test 3 分 51 秒・Node 4 分 34 秒） | success 約 25 分（全 KA の突き合わせ 24 分 19 秒） |
| 38138331343（2 回目） | f4f149e（docs だけ） | success 約 1 分（go test 42 秒。全 KA の突き合わせは省き、安全網だけ: `oracle: … / 回す 0 キャラ`、`突き合わせた条件: 7812(キャラ 31 ・エンジンと繋ぎ表の 2 経路 ・不一致 0)`、TestConformance_UpstreamOracle 39.15 秒） | success 約 40 秒（go test 21 秒。Node の通しの step は skipped） | success 約 20 秒（全 KA の突き合わせ・記録・保存の step は skipped） |

- 1 回目は 6 job すべて success。リポジトリ検査の新しい step「計算に効くコードの変更の記録」も success（違反 0 件）。3 job とも通った後に検査済みの記録を保存した。本家の正解の再生成は入力の変更なしで省略。
- 2 回目も 6 job すべて success。3 job とも重い検査は「回す 0 キャラ」で省かれ、Go の安全網（KA 61 以下・全 31 キャラ・7,812 回）は毎回どおり回った。
