# S1 RESEARCH — 完了報告

- 段階開始コミット: `469050731104d162f396ddfd0f40dbcb84880887`
- ブランチ: `claude/gifted-fermi-smqg4b`

## 計画

### Context
HANDOVER §9 S1: ①案 A の総容量と CI 時間（KA 0〜100）②案 B の表サイズと本家結果との一致（打ち切り含む）③`command` 生成の SRK/ナンバリングと preset の一致率 ④英語技名の上書き件数 ⑤公開版 tacpendium に必要ファイルがそろっているか ⑥Cloudflare 無料枠 ⑦D1 の無料枠と超過時の挙動 ⑧票を第一キーにしたときの打ち切りへの影響。
**完了条件: レポートと、案 A/B の推奨**。案の選択自体は §12-3 で開発者が決める（S1 では推奨まで。聞かない）。
- 開始コミット: `469050731104d162f396ddfd0f40dbcb84880887`、ブランチ `claude/gifted-fermi-smqg4b`
- 公開版 tacpendium の HEAD = `upstream/tacpendium.lock` = `ffdb69e4…`（ls-remote で確認済み）

### 成果物（リポジトリに入れるもの）
- `docs/progress/S1-report.md`（冒頭に計画節 → 実測 → 結論/推奨 → 完了報告 4 項目）。`/research` の型（結論・問いごとの節・不明・想定外・推奨）で書く。
- `docs/progress/S1-review.md`（レビュアー＋取り込み節）
- 計測ハーネスは **scratchpad に置きリポジトリへ入れない**（`.claude/commands/research.md`「計測用の一時ファイルはリポジトリへ入れない」）。パスと再現手順・要旨を報告の付録へ書く。レビュアーは同じコンテナなので実物を読める。

### 計測ハーネス（scratchpad `$SP/s1/`）
1. 公開版 tacpendium を lock SHA で clone（`--filter=blob:none`）。取込元は公開版のみ（D2）。
2. エンジン: 公開版の `internal/service/setplay` と `_reference/combomgr/internal/service/setplay` を diff し記録。**移植対象は `_reference` 版**なので、clone の写しに `_reference` 版を重ねる（コードの参照であってデータの取込ではない）。公開版の `testutil/dbtest` 等で migrations から使い捨て DB を作り、M47 §1.3 と同じく **clone 側にだけ計測用 `_test.go`** を置き、本物の `collectFillerSingles` / `buildFillerUnits` / `collectTargets` / `Suggest` / `refineProposals` / `rank` を呼ぶ。
   - 代替（Go モジュールが取れない等）: Python 標準 `sqlite3` で migrations を流して候補を JSON に書き出し、Go 側はスタブの Reader で読む。
3. 整合の確認: ハーネスの総数が M47 付録 B の `totalFound`（例: リュウ KA40=407・KA60=6169）と合うか（データ版差があれば差として記録）。

### 問いごとの測り方
- **①案 A**: 31 キャラ × KA 0〜100 × {meaty(n 帯最大)/gap(G 1〜13)/filler(remain 0〜KA−1)}、target 種別は全種＋ダメージ 0 含む（受理帯最大）。各 (キャラ,KA) の結果を仮の出力形（steps は技コードの索引）で JSON 化し、ファイル数・総容量・最大ファイル・gzip 後容量・`truncated` 件数・生成の壁時計時間（`nproc` 記載、他の重い処理と並行させない §8-6）。Cloudflare の 25MiB/20,000 ファイルとの比較。
- **②案 B**: キャラごとに「繋ぎ単位の多重集合で合計 T ちょうど」の表（T=0〜102、深さ k 別）を作り、件数・容量。エンジンは深さ k 昇順・候補 (total,name) 昇順の辞書順 DFS で 6000 件打ち切り → 表を (T,k) ごとに辞書順で持てば、窓内の T をマージして**打ち切り順まで再現できるか**を検証。**全キャラ × KA 0〜100 × 3 モード（既定パラメータ）で、案 B 結合 → 本物の refine/rank の出力を本家経路の出力とバイト比較**し一致率と不一致の原因を出す。要注意: `buildFillerUnits` の `budgetCap=KA+2` と `maxChainUnits=2000` の打ち切りは KA 依存（表を KA 100 で作ると低 KA で単位集合が変わりうる）→ 該当 (キャラ,KA) を数える。
- **⑧票と打ち切り**: 上限を外したときの真の件数を DP（深さ別・和の数え上げ）で数え、本家の 6000/2000/8 の各上限が効く (キャラ,KA,モード) と、上限で落ちる件数を出す。「並べ替えの後で切る」＝票で上がりうる候補を落とさないには事前計算で全件が要る → 案 A/B それぞれの容量・時間がどうなるか、本家順位と一致させたまま票を第一キーにできる条件を整理。
- **③表記**: Python で migrations を SQLite に流し、`preset_aliases`（preset 5=SRK, 3=ナンバリング）を `move_code` に結合。`command` 列のトークン文法（`d dr r plus p_l`、`chain`、`or`、`p p` 等）を列挙し、試作の生成器（scratchpad）で一致率をキャラ別・カテゴリ別に。不一致を型ごとに分類（状態印・空中・ホールド等）。生成同士の衝突件数（§4.3 の検査）も数える。
- **④英語技名**: `move_code` の規則生成（§4.2 の例）を試作し、上書きが要る技を機械的な型（ローマ字固有名詞、`during_`、状態接頭辞、数字段、略語等）で数える。2 キャラ程度を全件目視して型の当否を確かめ、件数は「推定」と明示。
- **⑤ファイル**: 公開版に `character_data/*.csv`（31）と migrations 000003/5/6/8/10/11/12/13/14 があるか、`_reference` との差（ファイル一覧・ハッシュ）、CSV 列の一致。
- **⑥⑦**: Cloudflare 公式 docs（Workers 静的アセットのファイル数・サイズ、Pages、Analytics Engine、Workers 無料のリクエスト/CPU、D1 の読み書き行数・容量、上限超過時の挙動＝エラーで止まるか課金か）を WebFetch で取得し、URL と取得日を書く。取れなければ「不明」と何を見れば分かるかを書く。

### 未決と扱い
- 案 A/B の選択（§12-3）: 推奨を書くだけ。開発者に聞かない（S1 完了条件が推奨まで）。
- 票の詳細（§12-6）: 決めない。⑧に必要な仮定は「推測:」で書く。
- AGENTS §3 の止める事項（費用・公開・秘密・ライセンス）: アカウント作成や課金設定は一切しない（docs を読むだけ）。

### 進め方
1. `S1-report.md` に計画節（上記）を書き、コミット `docs(S1): 計画` → push。
2. ⑤ → ③④ → ハーネス → ①②⑧ → ⑥⑦ の順。節目ごとに `docs(S1): …` でコミット・push。
3. 検査は出力をファイルへ書き `EXIT=` を出す。ハーネスの数え方は小さい入力で手計算と照合し、わざと壊して数が変わることを確かめる（§8-3、例: 打ち切りを 1 件にして一致検査が赤になる）。
4. レビュー: general-purpose サブエージェントを新規起動（fork しない）。段階 1・開始コミット・`.claude/commands/review.md` に従う・報告先 `docs/progress/S1-review.md`・読み取り専用・scratchpad のハーネスのパス。取り込み節を追記、最大 2 往復。
5. 完了報告 4 項目＋`git diff --stat 469050731104…`＋再実行で数値一致確認。push 後ローカル/リモート SHA 一致を確認して停止（S2 へ進まない）。

### 検証
- 案 B の一致検査が 100% 一致（または不一致の全件が原因つきで説明される）こと。
- 報告後にハーネスの主要集計を再実行し、報告の数値と diff で一致。

---

## 結論（先頭 7 行）

1. **推奨: 案 B（繋ぎ表方式）を、単位の上限を外した表（uncapped）＋「KA ごとに本家が作る単位の集合」で行を絞る結合（masked）で採る。** この方式は本家の結果と**全 3,131 セル（31 キャラ × KA 0〜100）で一致**した。エンジン段は受理帯 4 通り（meaty 最大・meaty N2〜4・gap G1〜13・gap G2〜5）と filler 全 remain の計 899,910 回、本家の単位上限に達する 705 セルと 6000 件上限に達する 850 セルを含めて、順序まで一致した。通し（refine・rank 後の全件）は 2,976 条件で比べて不一致 0（`truncated` は 6000 件上限による分だけが実質の比較。単位上限による分は両側が同じ印から作るので比べていない）。絞らない uncapped だけだと、本家が単位上限に達する KA 67 以上の 694 セルで食い違う。
2. **案 A は配信の上限内には収まる**（3,131 ファイル・最大 15.7 MiB < 25 MiB）が、総量 **5.70 GiB（gzip 0.77 GiB）**・生成 25 分（4 コア）で、しかも「最大の結果を絞る」方式は**条件を絞ったときに本家と食い違う**（1,085 セル中 162。うち 7 は本家が打ち切っていないセル）。
3. 案 B の表は全キャラで **282 MiB（gzip 32.9 MiB）**、最大はキャミィの 28.8 MiB で**1 ファイル 25 MiB を超える** → S4 で分割か符号化の見直しが要る（gzip では 3.2 MiB）。
4. **③表記**: 公開版 tacpendium に `command` からの生成器（`internal/seedgen` の `GenerateAliases`）がすでにあり、その出力は DB の preset を **numeric 2,325/2,325・SRK 2,340/2,340 で再現**する。ただし DB は同じ生成器の出力なので、これは**独立の検証ではない**。CSV 全 2,743 行のうち表記が付くのは numeric 2,356（85.9%）・SRK 2,371（86.4%）。本家は被りを落としていて、落とす前は**同一キャラ内で表記が被る技が numeric 578・SRK 550**。これが D9 の区別規則で扱う量になる。
5. **④英語技名**: `move_code` はほぼ英語で、規則生成で大半は読める。上書きは**推定 10〜30 件**（コード命名の不整合・所有格など）。ほかに状態印・段数などの**規則の追加**が要る（件数は確定させていない）。
6. **⑤公開版のファイル**: CSV 31 本と migrations 000003〜000013 は `_reference` と同一。**000014（TC/空中の表記書き換え＋キンバリーの派生 8 件）が公開版に無い**。
7. **⑥⑦ Cloudflare**: 静的アセットは無料・無制限（Workers Free はファイル 20,000・1 ファイル 25 MiB）。D1 Free は読み 500 万行/日・書き 10 万行/日・5 GB で、**超えると UTC 0 時までエラーで止まる（課金に移らない）**。

---

## 調べたこと

### 方法（再現手順）

- 計測ハーネスはリポジトリに入れていない（`.claude/commands/research.md`）。置き場所: scratchpad の `s1/`（`harness/*.go`・`py/*.py`・出力 `out/`）。レビュアーは同じコンテナで読める。
  - `SP=/tmp/claude-0/-home-user-tacpendium-setplay/e90b879c-d78a-5465-b917-802c9d645d0c/scratchpad/s1`
- **データとコード**: 公開版 tacpendium を `ffdb69e468a3f5c0b0e16255e68b6baa3266796d`（= `upstream/tacpendium.lock` = 公開版 HEAD）で clone（`$SP/tp`）。データの取込元は公開版のみ（D2）。
  - 公開版の `internal/service/setplay` と `_reference` 版の差は `service.go` の `alreadyAdopted` 関連（+46/−12 行、`withoutParentsHash`／`markAlreadyAdopted`。`$SP/service.diff`）と、`_reference` にだけある `service_m4305_test.go`。単体版で消す部分なので、**計算の意味は同じ**と判断し、公開版のコードをそのまま使った（計画では `_reference` 版を重ねるとしていた。変更点は「判断と理由」）。
- clone の `internal/service/setplay/` と `internal/seedgen/` に**ビルドタグ `s1probe` 付きの `_test.go` だけ**を置き、本物の `collectFillerSingles`／`buildFillerUnits`／`collectTargets`／`Suggest`／`refineProposals`／`rank` を呼んだ（M47 §1.3 と同じ方式）。DB は公開版の `dbtest.Setup`（migrations 000001〜000013 を適用）。
  - 実行例: `cd $SP/tp/internal/service/setplay && S1OUT=$SP/out go test -tags s1probe -count=1 -run TestS1Counts -v .`
  - 長い計測はバイナリを作って回した: `go test -tags s1probe -c -o $SP/probe.test .`。案 B は 4 シャード（`S1CHARS` にキャラを 7〜8 人ずつ渡す。例 `S1OUT=$SP/out S1CHARS=ryu,ken,ingrid,c_viper,dhalsim,terry,guile,lily $SP/probe.test -test.run TestS1PlanB -test.v -test.timeout 180m`）を並行で回した。
  - 注意: `TestS1PlanB`・`TestS1PlanBMasked`・`TestS1PlanAFilter` は不一致を**記録するだけ**で、不一致があっても PASS になる。以下の `PASS`・`EXIT=0` は「最後まで回った」証拠で、一致の証拠は集計スクリプトの出力である。
- 環境: Intel Xeon 2.10GHz × 4（`nproc`=4）、メモリ 15 GiB、Go 1.26.8（公開版 go.mod の toolchain）。
- 整合の確認: M47 §1.3 の 6 値と、ハーネスの meaty（既定の種別・エンジン段の件数）を比べた（`$SP/out/counts.json`）。リュウ KA 40 = 407・KA 60 = 6,169、ザンギエフ KA 40 = 177・KA 60 = 1,880、ガイル KA 40 = 851 は一致。**ダルシム KA 40 は 358（M47 は 188）で合わない**。推測: M47 は migrations 000001〜000009 で測っており、その後の 000011（`data_fix_dhalsim_projectile_akuma_jump`。ダルシムの飛び道具の訂正）で既定の種別に入る target が増えた。000011 を外した DB で数え直してはいない。

### 共通: 件数の数え上げ（DP）と検証

`TestS1Counts`: キャラ × KA 0〜100 で、繋ぎ単位の多重集合の数を DP（個数 k ≤ 8・合計 T）で数えた。
- 抜き取り検査: KA ∈ {0,20,40,60,80,100} × 全 target で、DP の `min(件数, 6000)` とエンジンの件数を比べた → **全 31 キャラで不一致 0**（キャラあたり 288〜468 件、計 11,040 件を検査。`$SP/out/counts.log`、各行 `dpMismatch=0/N`、`EXIT=0`）。

```
$ S1OUT=$SP/out go test -tags s1probe -count=1 -run TestS1Counts -v . > $SP/out/counts.log 2>&1; echo EXIT=$?
EXIT=0   （--- PASS: TestS1Counts。秒数は回すたびに 12.5〜12.6 秒）
$ grep -c 'dpMismatch=0/' $SP/out/counts.log
31
$ python3 -I $SP/py/counts_sum.py $SP/out/counts.json
unitsMatch102=false cells 362 chars 26 minKA 62
unitsTrunc cells 705 minKA 67
tableRows(capped) total 11230321
dpMismatch total 0 checked 11040
```

### ① 案 A: 総容量と CI 時間（KA 0〜100）

`TestS1PlanA`: 31 キャラ × KA 0〜101 通り × {meaty（N 1〜持続）・gap（G 1〜13）}、target は**全 7 種別＋ダメージ 0 を含む**（受理帯最大）、filler は remain 0〜KA−1 の全部を同じファイルに入れた。1 ファイル = (キャラ, KA)。出力の仮の形は `[[N か G か remain, 手, 手, …], …]`（手 = 技の索引×8 + 役割×2 + 計上）。refine・rank まで本物と同じ処理（limit で切らない）。

```
$ S1OUT=$SP/out $SP/probe.test -test.run TestS1PlanA -test.v -test.timeout 120m > $SP/out/plana.log 2>&1; echo EXIT=$?
--- PASS: TestS1PlanA (1507.18s)
EXIT=0
$ python3 -I $SP/py/plana_sum.py $SP/out/plana.json
cells 9393 copyMismatch 0 wall s 1507 workers 4
files(char,KA)=3131 totalRaw=5.70GiB totalGzip=0.77GiB props=247,527,022
max file ('blanka', 99) raw=15.7MiB gz=1.6MiB props=579,996
files >25MiB raw: 0 first KA per char min: None
cpu sum (ms->min): 83.7
meaty raw=2.79GiB gz=0.36GiB props=117,166,556 cpu_min=40.3 truncCells=856
gap raw=2.24GiB gz=0.31GiB props=99,872,955 cpu_min=33.3 truncCells=738
filler raw=0.66GiB gz=0.09GiB props=30,487,511 cpu_min=10.1 truncCells=705
KA<=40: raw=0.004GiB gz=0.001GiB maxfile=0.04MiB cpu_min=0.1
KA<=60: raw=0.085GiB gz=0.016GiB maxfile=0.90MiB cpu_min=1.2
KA<=80: raw=1.163GiB gz=0.179GiB maxfile=8.15MiB cpu_min=17.4
KA<=100: raw=5.696GiB gz=0.766GiB maxfile=15.67MiB cpu_min=83.7
```

- 壁時計 25 分 7 秒は 4 並列で、直列化と gzip を含む。計算（refine・rank まで）の合計は 83.7 CPU 分。GitHub Actions の標準ランナー（public は 4 vCPU、private は 2 vCPU）に置き換えた時間は**未計測**（推測: private の間は 2 倍前後）。
- 1 セルの計算時間の最大: KA 100 meaty で 6,956 ms（全 target・limit なし）。**大部分はエンジンの後**（提案の組み立て・refine・rank）で、KA 100 のエンジン段だけの最大は 718 ms（全 KA での最大は KA 98 の 880 ms。案 B の計測、下）。
- 抜き取り検査: ハーネスの写し（`fullSuggest`）を本物の `SuggestForCombo(limit=3000)` と KA ∈ {0,25,50,75,100} × 3 モードで比べ、**不一致 0**（`copyMismatch 0`）。
  - §8-3: 写しの並べ替えをわざと `SortByTarget` に変えると 124 件の不一致になった（`$SP/out/trial/break.log`、`EXIT=1`）。戻して 0。
- **絞り込みの忠実さ（追加の検査）** `TestS1PlanAFilter`: 案 A のブラウザの仕事（最大の結果から、種別・N 帯・G 帯・3 つの絞り込みで選んで並べ替え直す）を模し、本家（条件ごとに計算）と比べた。31 キャラ × KA {20,40,60,70,80,90,100} × 5 条件 = 1,085 セル。

```
$ S1OUT=$SP/out go test -tags s1probe -count=1 -run TestS1PlanAFilter -v -timeout 60m . > $SP/out/planafilter.log 2>&1; echo EXIT=$?
ok  	github.com/plexiblinp/tacpendium/internal/service/setplay	659.671s
EXIT=0
$ python3 -I $SP/py/planafilter_sum.py $SP/out/planafilter.json
cells 1085 match 923
mismatch by refTrunc Counter({True: 155, False: 7})
mismatch by KA [(70, 4), (80, 39), (90, 58), (100, 61)]
mismatch by variant [(1, 80), (2, 82)]
mismatch with got<want (rows missing) 162 got>want 0 equal len 0
nonTrunc mismatches [('ingrid', 70, 2, 10579, 10074), ('jamie', 80, 1, 38581, 26785), ('jamie', 80, 2, 35198, 31403), ('marisa', 100, 1, 43344, 35420), ('jp', 80, 2, 8587, 8308), ('elena', 90, 1, 21467, 20749), ('elena', 90, 2, 32950, 32217)]
```
  - 食い違いは条件 1（gap・G 2〜5）と条件 2（全種別・N 2〜4・接近のみ）、つまり**帯を狭める条件だけ**で、**KA 60 以下では 0**、162 セルの**すべてで案 A の行が足りない**（`got < want`）。**7 セルは本家が打ち切っていない**のに食い違う。
  - 推測（根拠は上の 3 点）: 受理帯を最大にして計算すると target あたり 6000 件の上限に先に当たり、狭い帯なら入る行が落ちる。target ごとに上限に当たったかは記録していない。
  - §8-3: `filterWide` の N 帯の絞り込みをわざと外すと、リュウ・ジェイミーの 70 セルで不一致が 12 → 20 になり、KA 20〜60 にも出た（`$SP/out/trial/filter_break.log`）。戻した。
- Cloudflare の上限との比較: ファイル数 3,131 ≤ 20,000、最大 15.7 MiB ≤ 25 MiB。**総容量の上限は docs に記載が無い**（⑥）。

### ② 案 B: 表のサイズと本家結果との一致

`TestS1PlanB`: キャラごとに「繋ぎ単位の多重集合で合計 T ちょうど・個数 k」の表（T 0〜102、k 0〜8、索引列はエンジンと同じ (total, name) 昇順の辞書順）を作り、**結合**（受理帯の T をマージし、k 昇順・辞書順・6000 件で切る）がエンジンと同じ `[]Suggestion` を**順序まで含めて**返すかを、31 キャラ × KA 0〜100 × 全 target × {meaty・gap} ＋ filler 全 remain で比べた（528,230 回）。表は 2 通り:
- **capped**: 本家の単位上限（`maxChainUnits=2000`・`maxChainLen=8`）のまま予算 102 で作る
- **uncapped**: 単位上限を外して作る

通し検査（e2e）: KA が 10 の倍数のセルで、6 条件（既定 meaty／gap G2〜5 全種別／N2〜4・接近のみ／ガード有利のみ・技順／filler／filler・連打のみ）について refine・rank 後の**全件**を比べた。

```
（4 並列で実行。各シャード --- PASS・EXIT=0）
$ python3 -I $SP/py/planb_sum.py $SP/out   （抜粋。全文は $SP/out/planb_summary.txt）
chars 31 cells 6262
[capped] cells=3131 calls=528,230 mismatchCalls=6,011 mismatchCells=310
  mismatch cells by (unitsTrunc, unitsMatch102): Counter({(True, False): 209, (False, False): 101})
  cells NOT unitsTrunc: 2426 of which mismatch: 101
  engTrunc & not unitsTrunc cells: 152 mismatch among them: 59
  e2e checked 341 fail 17 fail&notUnitsTrunc 2 fail where engine-level matched 0
  time join=153s engine=354s
  table: rows=11,230,321 raw=103.2MiB gz=25.0MiB maxraw=9.05MiB(cammy) maxgz=2.20MiB units max=2000 buildMs max=97
[uncapped] cells=3131 calls=528,230 mismatchCalls=47,190 mismatchCells=694
  mismatch cells by (unitsTrunc, unitsMatch102): Counter({(True, True): 488, (True, False): 206})
  cells NOT unitsTrunc: 2426 of which mismatch: 0
  engTrunc & not unitsTrunc cells: 152 mismatch among them: 0
  e2e checked 341 fail 77 fail&notUnitsTrunc 0 fail where engine-level matched 0
  time join=141s engine=362s
  table: rows=13,518,609 raw=282.3MiB gz=32.9MiB maxraw=28.76MiB(cammy) maxgz=3.17MiB units max=119466 buildMs max=219
```

読み方:
- **uncapped は、本家が単位の上限に達しないセル（2,426）で全件一致**。うち本家が 6000 件の上限に当たる 152 セルでも一致 ＝ **打ち切りの順序まで再現**できる。食い違いは本家が単位上限に達するセル（KA 67 以上、本家は `truncated=true`）だけ。
- **capped は低い KA でも食い違う**（本家が単位上限に達しないのに 101 セル）。予算 102 で作ると単位上限に早く当たり、KA ごとの単位集合と変わるため（`unitsMatch102=false` が 362 セル。KA 62〜）。⇒ 本家の上限をそのまま表に持ち込むのは誤り。
- 時間（参考。4 シャードを並行で走らせたため絶対値は §8-6 の条件を満たさない）: 同じプロセス内の比で、結合はエンジンの約 0.4 倍。セルあたり最大は KA 100 で結合 376 ms・エンジン 718 ms（全 target・meaty+gap+filler 全 remain）。
- §8-3: 結合のマージ順を逆にすると、ルークの uncapped で本家が単位上限に達しないセル 93（全 101 セル）のうち 75 が赤になった（`$SP/out/trial/planb-luke.json`）。戻して 0。
- 検査の範囲: エンジン段の比較は受理帯が最大のもの（meaty N1〜持続・gap G1〜13）と filler 全 remain。帯を狭めた入力は通し検査（KA 10 刻み 341 セル × 6 条件）だけで、`truncated` は比べていない。⇒ 下の masked で範囲を広げた。

#### ②-2 masked: uncapped の表を KA ごとの単位集合で絞る（レビュー中-3 の案を検証）

uncapped の表で食い違う 694 セルは、どれも本家が単位上限（2000 個・連鎖長 8）に達したセルだった（相関）。そこで、`TestS1PlanBMasked` で、結合のときに「その KA で本家の `buildFillerUnits(KA+2)` が作る単位だけでできた行」に絞った。部分集合に絞っても辞書順は保たれるので、絞った後に 6000 件で切る。比べる範囲は全 31 キャラ × KA 0〜100 で、エンジン段は受理帯 4 通り（meaty N1〜持続／meaty N2〜4／gap G1〜13／gap G2〜5）と filler 全 remain。通し検査は KA 10 刻みと KA 95〜100 で 6 条件。`truncated` も比べたが、ハーネスは両側とも本家の準備（`p.unitsTr`）から始めるので、単位上限による `truncated`（233 セル・1,398 条件）は比べになっていない。実質は 6000 件上限による分と提案の中身だけ。

```
（4 シャード並行。各 --- PASS・EXIT=0 は最後まで回った証拠）
$ python3 -I $SP/py/planbmask_sum.py $SP/out
chars 31 cells 3131 calls 899,910
mismatchCells 0 unitsTruncCells 705 engTruncCells 850
e2e 2976 e2eFail 0 truncFail 0
maskSize max 2000 KA with units trunc per char: 705
```
- **全セル一致**。本家が単位上限に達する 705 セル、6000 件上限に達する 850 セルも含む。⇒ 694 セルの食い違いの原因は単位集合の違い、と再現で確かめた。
- §8-3: 絞りを外す（`allow` を見ない）と、ルークで 6 セル・通し 3 条件が赤になった（`$SP/out/trial/mask_break.log`。壊した実行の JSON は戻した後の実行で上書きされたため、壊した結果はログと本報告の記録だけ）。戻して 0。
- 追加で配るもの: KA ごとの単位集合と、KA ごとの「単位上限に達した」印（`truncated` に使う）。推測: 本家が単位上限に達しない KA では、単位集合は「表の単位のうち予算以下の全部」になるので配らなくてよい。これは今のデータでは成り立った（`unitsMatch102` と uncapped の一致から）が、単位集合が表の単位に含まれることが前提で、データ次第で崩れうる。要るのは上限に達する 705 セル（KA 67 以上）分。1 セル最大 2000 単位。形と容量は未計測（推測: 単位の索引のビット列なら 1 セルあたり 15 KB 程度 × キャラあたり最大 34 KA）。
- 表の容量（仮の形 = 単位表〔名前は技コードの `>` 連結〕＋ T・k ごとの索引列の JSON）: uncapped は**キャミィ 28.76 MiB が 25 MiB を超える**。単位数 119,466（リリー）のように単位表の文字列が大半を占める（推測: 単位を技の索引列にすれば大きく縮む。未計測）。

### ⑧ 票を第一キーにしたときの打ち切りへの影響

`$SP/out/cap_summary.txt`（`TestS1Counts`・`TestS1Uncapped` の DP。エンジン段・refine 前の件数。下は抜粋）:

```
meaty/default: cells with engine cap hit=581/3131 firstKA=74 dropped=143,997,793 of uncapped 217,418,712
meaty/all: cells with engine cap hit=846/3131 firstKA=64 dropped=500,921,231 of uncapped 685,396,983
   KA100: chars with cap hit=31 max uncapped=9,623,464 max capped=431,272
gap/all: cells with engine cap hit=682/3131 firstKA=70 dropped=292,815,134 of uncapped 444,473,902
filler: cells with cap hit 347 firstKA 81
unitsTrunc cells 705 firstKA 67
meaty/all KA100 true count (units uncapped) max (12122195, 'cammy') sum 93,814,560
```

- 本家の上限（target あたり 6000・単位 2000・連鎖長 8）は **KA 64 以上で効き始め、KA 100 では 31 キャラ全部で効く**。KA 100 の meaty（全種別）は上限なしで最大 **962 万件**（単位上限も外すと 1,212 万件）、上限ありで 43 万件。
- 「並べ替えの後で切る」（AGENTS §4）を票で守るには、票で上がりうる行を事前計算や上限で落とせない。上限なしの全件を事前計算・配信するのは、案 A でも案 B の結合結果でも上の件数では非現実的（推測による見積もり: 案 A の 1 行あたり約 24 バイト〔5.70 GiB ÷ 2.48 億行。refine 後〕× 上限なしの件数〔エンジン段・refine 前。meaty 全種別 6.85 億＋gap 4.44 億＋filler 0.75 億＝約 12 億、KA 0〜100 の合計。単位上限は残した値〕≒ 約 29 GB。refine で減る分は過大側に、単位上限を外して増える分〔meaty 全種別は KA 100 だけで 0.94 億〕は過小側に働く）。
- **案 A/B と独立に成り立つ解き方（推奨・未検証）**: 推測: 票の付いたレシピは、上限の結果全体より桁違いに少ない（票の数の見込みは無い）。そこで**票の付いたレシピを別に配り、要求ごとに成立するかを判定して上限の結果に合流させる**。こうすると、票の付いた行は上限で落ちず、票の無い行は本家と同じ順・同じ上限のまま。合流させる行が満たすべき条件:
  - 成立する: S = Σ繋ぎ + 当てる技の発生、N = KA + 2 − S が受理帯に入る（n_min〜n_max・G 帯。filler は Σ = KA − remain）
  - その要求の絞り込み（target 種別・ダメージ 0・接近のみ・連打キャンセルのみ・ガード有利のみ）と、隣り合いの並べ替え・重複除去（`refineProposals`）を通る
  - 本家の単位上限の意味を保つなら、その KA の単位集合（②-2）に含まれる単位だけでできている
  - 上限の結果にすでにある行と重ならない。`totalFound` の数え方を決める
  - ⇒ 票は**レシピの同一性キー（技コードの列）で持つ**のが前提（§12-6 で決める項目）。推測: 案 B なら単位の展開と単位集合を共有できるので組み込みやすい。

### ③ `command` から生成した SRK/ナンバリングと preset の一致率

公開版には `command` 列から numeric/SRK を作る生成器（`internal/seedgen/generate_m2002.go` の `GenerateAliases`。M48-08 以降の規則）があり、DB の preset はその出力である。`TestS1Alias` で、全 CSV（31 キャラ・2,743 行）に生成器を通し、DB（公開版 migrations 適用後）の preset と突き合わせた。

```
$ S1OUT=$SP/out go test -tags s1probe -count=1 -run TestS1Alias -v . > $SP/out/alias.log 2>&1; echo EXIT=$?
numeric: csv=2743 gen=2356 pre=2691 db=2325 same=2325 differ=0 genOnly=31 dbOnly=0 coll=243/335 unfilled=map[collision:344 derived-no-command:20 index-skip:5 no-command:6 rush-original-unfilled:12] dupPre=243/578
srk: csv=2743 gen=2371 pre=2692 db=2340 same=2340 differ=0 genOnly=31 dbOnly=0 coll=229/321 unfilled=map[collision:330 derived-no-command:20 index-skip:5 no-command:6 rush-original-unfilled:11] dupPre=229/550
EXIT=0
```

- **DB にある行は 100% 一致**（numeric 2,325/2,325・SRK 2,340/2,340）。ただし DB の preset はこの生成器の出力なので、**生成器が DB を再現することの確認**であり、表記が正しいことの独立の検証ではない。本家の現行（000014 の書き換え後）との一致率は未計測。
- 計画との違い（逸脱）: 計画では試作の生成器を作り、キャラ別・カテゴリ別の一致率と不一致の型を出すとしていた。公開版に生成器があったので試作は作らず、キャラ別・カテゴリ別の集計もしていない（不一致が 0 なので型分けの対象は無い）。`genOnly=31` は全部 `drive_parry`（31 キャラ × 1。ハーネスが DB 側だけ `category='system'` を除いた非対称で、生成器の差ではない）を確認済み。
- **表記が付かない行**（被り以外）: numeric 43・SRK 42（command 空の派生 20、command 空 6、語彙外 5、ラッシュの元技が無い 12/11）。⇒ 単体版の「全技が 4 表記を持つ」（§4.3）には、この分の規則か上書きが要る。
- **被り**: 本家は被った組の 1 つだけ残す（M48-08）。落とす前は**同一キャラ内で同じ文字列になる技が numeric 578（243 組）・SRK 550（229 組）**。単体版は逆引きしないので落とさず、状態印・空中印などで区別する（D9・§4.3）。この量が S2 の区別規則の作業量になる。
- §8-3: 生成側のリュウ `hadoken_light` の表記をわざと変えると `same` が 1 減り `differ=1` になった（numeric・SRK とも。`$SP/out/trial/alias_break.log`）。
- 公開版に 000014（M43-05: ターゲットコンボを `5MP~5HP(TC)`、空中技に `j.`・`nj.`・`hj.` を付ける書き換え）が無いため、上の一致は 000014 **前**の規則での値。000014 の生成器（`GenerateAliasRewrite`）も公開版には無い。

### ④ 英語技名の上書き件数

`$SP/py/name_en.py`（試作）: `move_code` を `_` で分け、強度（light/medium/heavy/od → 先頭）・`rush_` → `Drive Rush`・`aerial_` → `Aerial`・`2hits` 等 → 括弧書き・Title Case（of/the 等は小文字）で生成。

- `move_code` は 775 種の語でほぼ英語（例: `freeflow_strikes` `the_devil_inside` `phantom_sway`）。日本語由来の固有名（`tatsumaki_senpu_kyaku` `hashogeki`）もローマ字表記のまま英語名として通用する形。
- 全件の目視は 2 キャラ: **リュウ 84 行は上書き 0**。ただしジェイミーと同じ基準で見ると、規則の追加は要る（`throw_forward` → `Forward Throw`、`denjin_charge_od_hadoken` → 状態印と OD の語順）。**ジェイミー 127 行**は、規則を足せば済むもの（`drink_level_4_` 等の状態接頭辞 → 状態印、`up2` → `(+2)`、`reach_drink_lv4`、`throw_forward` → `Forward Throw`）と、**上書きが要るもの約 5 件**（`phantom_sway`／`full_moon_kick`＝飲酒版なのにコードに印が無い、`intoxicated_assault_1hit`＝和名は 2 発止め、`sa2_the_devil_s_song`・`hermits_elbow`＝所有格）。
- 機械的な旗（和名の修飾語が英語名に無い・連結語 during/reach・数字語）は非ラッシュで 68 行・11 キャラ（ジェイミー 32・ルーク 13・アレックス 7 …）。ただし「ラッシュ」の検出が「フラッシュ」に誤反応するなど精度は低い（`$SP/out/name_en.json`）。
- **推定: 上書き 10〜30 件**（コード命名の不整合はジェイミーに集中。ドメイン知識集成 §8 の「code 命名の不整合」と整合）。**確定値ではない**。全件の確定は S2 で全 2,743 行を通すときに行う。

### ⑤ 公開版 tacpendium に必要ファイルがそろっているか

`$SP/files.txt`（sha1 比較）:
- `character_data/*.csv` 31 本: `_reference` と**全部同一**。列（25 列、HANDOVER §2.5 の列）も同じ。`seed-progress.md` だけ異なる（文書）。
- migrations: 000003（キャラ・`custom_states`）/000004（moves）/000005（command）/000006（派生）/000008・000012（preset_aliases）/000010（安全飛び）/000011・000013（データ修正）は**同一**（`_reference` にあるのは 000001・000003〜000008・000010〜000014 で、000002・000009 は比べていない）。
- **000014 が公開版に無い**（`_reference` には有る）。中身: preset numeric/SRK の書き換え（TC・空中印）と、**キンバリーの疾駆け派生 8 技の親（`move_derivations`）**。後者は計算に効く（`isSoloUnavailable`・表記用親）。⇒ 公開版が次のリリースで 000014 を出すまで、キンバリーの派生 8 技の扱いが本家と違う。
- 英語技名は公開版にも無い（HANDOVER §2.5 のとおり）。
- 生成器: 公開版に `internal/seedgen`（表記の生成器）・`internal/moveindex`（command の正規化）がある（AGPL。移植可）。

### ⑥ Cloudflare 無料枠

developers.cloudflare.com はこのコンテナのネットワーク設定で遮断されていたため、**同じ docs のソース**（`github.com/cloudflare/cloudflare-docs`、commit `55be7a604d5ecd63a12a3381299cd644b76095f3`・2026-10-06）を読んだ。

| 項目 | Workers Free | 出所 |
|---|---|---|
| 静的アセットのファイル数 | 20,000 / Worker 版 | `workers/platform/limits.mdx` L34・L371 |
| 1 ファイルの大きさ | 25 MiB | 同 L35・L372 |
| 静的アセットへのリクエスト | **無料・無制限** | `workers/platform/pricing.mdx` L42、`workers/static-assets/billing-and-limitations.mdx` L16 |
| Worker（動くコード）のリクエスト | 100,000/日、UTC 0 時にリセット。超えると **Error 1027**（route が fail open なら Worker を素通り） | `limits.mdx` L172〜L182 |
| CPU 時間 | 10 ms/リクエスト（超えると Error 1102） | 同 L73・L82 |
| `run_worker_first` を使うと | 無料枠超過時に 429 を返し、静的アセットに戻らない | `billing-and-limitations.mdx` L18 |
| Pages | 20,000 ファイル・25 MiB・ビルド 500 回/月 | `pages/platform/limits.mdx` L23〜L24・L40・L46 |
| Analytics Engine | `writeDataPoint` 1 回に blob 20・double 20・index 1、blob 合計 16 KB、1 呼び出し 250 点、保持 3 か月。Free は書込 10 万点/日・読取クエリ 1 万回/日 | `analytics/analytics-engine/limits.mdx` L16〜L23、`analytics/analytics-engine/pricing.mdx` L20 |
| Analytics Engine の課金 | 「現在は課金していない」（料金は予告） | `analytics/analytics-engine/pricing.mdx` L24 |

- **総容量（全ファイルの合計）の上限は docs に記載が無い**（検索語 `total`・`GB`・`size` で `limits.mdx`・`billing-and-limitations.mdx` を見た）。
- 不明: Analytics Engine の Free 枠を超えたときの挙動（書込が捨てられるか・エラーか）は docs に記載が無い。確かめるには実アカウントでの試験が要る（費用ではないが、アカウント作成は AGENTS §3-3 なので S1 では行わない）。

### ⑦ D1 の無料枠と超過時の挙動

| 項目 | Workers Free | 出所 |
|---|---|---|
| 読んだ行 | 5,000,000/日 | `partials/workers/d1-pricing.mdx` |
| 書いた行 | 100,000/日 | 同 |
| 容量 | 合計 5 GB、1 DB 500 MB、DB 10 個 | 同、`d1/platform/limits.mdx` L15〜L17 |
| 1 Worker 呼び出しのクエリ数 | 50 | `limits.mdx` L20 |
| リセット | 「Free limits reset daily at 00:00 UTC」 | `d1-pricing.mdx` 定義 8 |
| 超過時 | クエリが `Your account has exceeded D1's free tier daily row read/write limit. Upgrade to a paid plan or wait until tomorrow (midnight UTC) to continue.` で失敗 | `d1/observability/debug-d1.mdx` L72・L73 |

- ⇒ **超えると止まるだけで、自動で課金に移らない**（D13 に適合）。ただし「読んだ行」は走査した行で数える（索引の無い条件は全行を読む。定義 1）ので、票の配信を D1 から毎回読む形にすると読み行数が先に尽きうる（推測: 票は定期的に静的 JSON へ書き出して配るほうが安全。S6 で設計）。
- 票の書込 10 万行/日 は、索引を付けると 1 票で 2 行以上になる（定義 6）。

---

## 不明だった点

- GitHub Actions のランナー（private 2 vCPU）での案 A の生成時間（未計測。ここでは 4 コアの壁時計 25 分のみ）。
- 案 B の表・案 A のファイルの **brotli** での大きさ（Go 標準に brotli が無く未計測。Cloudflare は配信時に圧縮する）。
- **推奨する masked の結合の時間**（未計測）。上の 376 ms・0.4 倍は絞らない結合（6000 件で早く止まる）の値。ハーネスの `maskedJoin` は窓の中の全行を作ってから絞るので、上限なしで KA 100 meaty 全種別は最大 1,212 万行になる。S4 ではマージの中で絞り、上限で止める形にする。
- ブラウザ（WASM）での結合・refine・rank の時間（未計測。Go ネイティブで KA 100 のセル全体が絞らない結合 376 ms、案 A の refine・rank 込み 1 モード最大 7 秒）。
- Analytics Engine の無料枠超過時の挙動（⑥）。
- 英語技名の上書きの確定件数（④。2 キャラだけ全件目視）。

## 想定外の発見・矛盾

1. **公開版に migration 000014 が無い**（⑤）。キンバリーの派生 8 技の親が公開版では未登録。
2. **本家の上限を表に持ち込むと、低い KA でも食い違う**（capped の 101 セル）。`buildFillerUnits` の上限は予算（KA+2）に依存する。幅優先で作るので、予算が大きいほど、低い KA で使う単位を作る前に 2000 個に達する。
3. 本家の処理時間の大部分は**エンジンの後**（提案の組み立て・refine・rank）。KA 100 で 1 モード最大 7 秒に対し、KA 100 のエンジン段は全モード合わせて最大 0.7 秒。HANDOVER §2.4 の「エンジンが重い」前提とは違い、案 B でもブラウザの refine・rank が重くなりうる。
4. HANDOVER §2.3 の仮説「T は 0〜約 101 で KA に依存しない」は、**単位上限を外せば成立**する（uncapped の一致）。本家の結果を全セルで再現するには、KA ごとの単位集合が別に要る（②-2）。
5. 英語技名の生成では、ジェイミーの `phantom_sway`（飲酒版）と `phantom_sway_2hits`（素の幻酔舞）のように、**コードの接尾辞の意味が技ごとに逆**（ドメイン知識集成 §8 の既知事項）。
6. HANDOVER §2.5 は「`000008` / `000012` / `000014` の preset_aliases を照合用に使う」としているが、公開版では 000014 が使えない。
7. M47 のダルシム KA 40 の件数（188）が現在のデータ（358）と合わない（推測: 000011 のデータ修正）。

## 推奨（求められたもの）

**案 B を推奨する。表は uncapped で作り、本家が単位上限に達する KA では「その KA の単位集合」で行を絞る（masked）。** 理由:
- **本家と全 3,131 セルで一致**した（②-2）。順序と 6000 件の打ち切りまで一致し、受理帯を狭めた入力も含む（エンジン段 4 帯 + filler、通しは 2,976 条件）。単位上限による `truncated` は比べていない（KA ごとの印を配る前提）。⇒ D1「本家をそのまま」を KA 0〜100 の全域で満たせる見込み。
- 案 A は、受理帯を狭めると本家と食い違う（1,085 セル中 162。本家が打ち切っていないセルでも 7）。食い違いを避けるには条件ごとのファイルが要り、容量がさらに増える。
- 容量は表 282 MiB（gzip 33 MiB）で、案 A（5.7 GiB）の約 1/20。表を組む部分は 1 キャラ最大 0.22 秒（uncapped の単位を作る時間と KA ごとの単位集合を作る時間は含まない。未計測だが、`TestS1Counts` は単位構築 31×101 回を含めて 12.6 秒）。
- 票（D17）は、案と独立に「票の付いたレシピを合流する」形で解ける見込み（⑧。未検証）。

条件と残る作業:
- キャミィの表が 25 MiB を超えるので、S4 で**単位を技の索引列で持つ**か、**T の範囲でファイルを分ける**。
- KA ごとの単位集合（本家が単位上限に達する 705 セル分）の配り方と容量を S4 で決めて測る。
- KA 上限（D10）を 100 より上げると、uncapped の単位数は予算とともに組み合わせ的に増える（予算 102 で最大 119,466・リリー）。推測: 表は T の上限とともに速く大きくなるので、上げるなら測り直す。
- ブラウザの結合・refine・rank の時間は S5 で測る。重ければ工夫が要るが、「並べ替えの後で切る」は崩さない。

**選択は HANDOVER §12-3 のとおり開発者が決める。** S1 の範囲では推奨までとし、開発者には聞いていない。

---

## やったこと

- 公開版 tacpendium（`ffdb69e4`）を scratchpad に clone し、`s1probe` タグ付きのテストだけを置いた計測ハーネスで ①〜⑧ を実測した（ハーネスはリポジトリ外。§「方法」）。
- 実測の主な数値:
  - ① 案 A: 3,131 ファイル・5.70 GiB（gzip 0.77 GiB）・最大 15.7 MiB・4 コアで 25 分。絞り込みの忠実さは 923/1,085。
  - ② 案 B: 表 uncapped 282 MiB（gzip 33 MiB）・最大 28.8 MiB。masked で本家と全 3,131 セル一致（899,910 回 + 通し 2,976 条件）。
  - ③ 表記: 生成器が DB を 100% 再現（独立の検証ではない）。被り numeric 578・SRK 550。表記なし 43・42。
  - ④ 英語技名: 上書き推定 10〜30 件（2 キャラを全件目視）。
  - ⑤ 公開版に 000014 が無い。他の必要ファイルは `_reference` と同一。
  - ⑥⑦ Cloudflare・D1 の無料枠と超過時の挙動（docs のソースから）。
  - ⑧ 上限が効くのは KA 64 以上。票の合流案。
- 新規の検査は、すべてわざと壊して赤くなることを確かめた（写し・結合順・単位の絞り・案 A の絞り込み・表記の比較）。
- 成果物はこの報告と `S1-review.md` だけ（コードは入れていない）。

## 判断と理由

- **計測ハーネスをリポジトリに入れなかった。** `.claude/commands/research.md` の「計測用の一時ファイルはリポジトリへ入れない」に従った。再現手順と置き場所を書き、レビュアーは同じコンテナで実物を読んで回した。
- **公開版のエンジンをそのまま使った（計画からの変更）。** 計画では `_reference` 版を重ねるとしていた。差は `alreadyAdopted`（単体版で消す部分）だけだったので、公開版のまま使えば取込元が公開版だけで済み、D2 の説明が簡単になる。
- **③ で試作の生成器を作らなかった（計画からの逸脱）。** 公開版に生成器（`GenerateAliases`）があったため。その結果、③ の一致率は独立の検証ではない。キャラ別・カテゴリ別の集計もしていない。
- **④ の件数を推定にとどめた。** 正解の英語名が無い（公式データは使わない＝§8）ので、機械的な判定はできない。2 キャラだけ全件目視した。確定は S2。
- **masked 方式を追加で検証した（範囲を広げた）。** レビュー中-3 の案。D1（本家をそのまま）に直結し、推奨を左右するため。
- **Cloudflare の docs はソース（GitHub）から読んだ。** developers.cloudflare.com がこのコンテナでは遮断されていたため。内容は同じ docs の原稿で、commit を記録した。
- **案の選択と票の詳細は決めていない**（§12-3・§12-6。推奨と判断材料まで）。

## 残課題・既知の問題
- **レビュー 2 往復目で残ったもの（記録して止める）**: 単位上限による `truncated` の一致は未検証（中-1。印を配る形で S4 に検証）、masked の結合時間は未計測（中-2）、`planafilter.json` は今のハーネス（`S1CHARS` を足した版）より前の版で作った（中身の処理は同じ）。

- ③: 本家の現行（000014 適用後）の表記との一致率は未計測。公開版が 000014 を出したら測る。
- ④: 上書きの確定件数は未定（推定 10〜30 件）。
- 案 A の生成時間を GitHub Actions のランナーで測っていない。brotli の大きさも未計測。
- ブラウザ（WASM）での結合・refine・rank の時間は未計測（S5）。
- Analytics Engine の無料枠超過時の挙動は docs に記載が無い（実アカウントでの試験が要る＝AGENTS §3-3）。
- ダルシム KA 40 の M47 との差（188 → 358）の原因は推測のまま。
- masked で配る単位集合の形と容量は未計測（S4）。
- 票の合流案（⑧）は未検証。

## 次の段階への入力

- **S2（データ）**:
  - 公開版に **000014 が無い**。キンバリーの疾駆け派生 8 技の親（`move_derivations`）と、TC・空中印の表記の書き換えが入っていない。公開版が出すまでは 000013 までで作る。
  - 表記は公開版の `internal/seedgen`（`GenerateAliases`）と `internal/moveindex` を移植できる（AGPL）。表記が付かない行が numeric 43・SRK 42、同一キャラ内の被りが numeric 578（243 組）・SRK 550（229 組）。これが §4.3 の区別規則と「全技が 4 表記を持つ」検査の作業量になる。
  - 英語技名は状態接頭辞（`drink_level_4_` 等）・`up2`・`reach_drink_lv4`・`throw_forward` 等の規則を足し、残りを上書き表（推定 10〜30 件）にする。ジェイミーが最多。
- **S3（エンジン）**:
  - 公開版と `_reference` の `service.go` の差は `alreadyAdopted` 関連だけ。`_reference` の `service_m4305_test.go`（`TestM4305_RealData_KimberlySprintPrefix`）は 000014 を前提にしているので、公開版のデータでは通らない可能性がある（推測）。
  - golden（適合性テスト）には、受理帯を狭めた入力と `truncated` を比べる入力を含める。
- **S4（事前計算）**: 推奨どおり案 B なら、表は uncapped で作り、本家が単位上限に達する KA（67 以上）の単位集合を別に配る。キャミィの表（28.8 MiB）が 25 MiB を超えるので、形を見直すか分ける。ハーネスの `buildTable`・`joinEngine`・`maskedJoin`（scratchpad）が試作として使える。
- **S6（票）**: D1 Free は読み 500 万行/日・書き 10 万行/日で、超えると UTC 0 時までエラー（課金なし）。読みは走査行で数える。推測: 票は静的 JSON に書き出して配るほうが安全。
- ハーネスと出力の置き場所: `$SP` = `/tmp/claude-0/-home-user-tacpendium-setplay/e90b879c-d78a-5465-b917-802c9d645d0c/scratchpad/s1`。コンテナが消えると残らない。S4 で使う関数は報告の記述から作り直せる粒度で書いた。

## 最終確認（報告を書いた後に回し直した結果）

報告とレビューの取り込みを書き終えた後、軽い検査を回し直し、報告に貼った値と比べた（出力は `$SP/final/`）。

```
$ cd $SP/tp/internal/service/setplay && S1OUT=$SP/final go test -tags s1probe -count=1 -run TestS1Counts -v . > $SP/final/counts.log 2>&1; echo COUNTS_EXIT=$?
COUNTS_EXIT=0
$ cmp $SP/final/counts.json $SP/out/counts.json && echo counts.json IDENTICAL
counts.json IDENTICAL
$ cd $SP/tp/internal/seedgen && S1OUT=$SP/final go test -tags s1probe -count=1 -run TestS1Alias -v . > $SP/final/alias.log 2>&1; echo ALIAS_EXIT=$?
ALIAS_EXIT=0
$ cmp $SP/final/alias.json $SP/out/alias.json && echo alias.json IDENTICAL
alias.json IDENTICAL
（集計スクリプト 6 本を回し直し、保存済みの summary と diff）
counts_summary IDENTICAL / plana IDENTICAL / planb IDENTICAL / planbmask IDENTICAL / planafilter IDENTICAL / cap IDENTICAL（各 EXIT=0）
```

長い計測（`TestS1PlanA`・`TestS1PlanB`・`TestS1PlanBMasked`・`TestS1PlanAFilter`、各 10〜30 分）は回し直していない。数値はそれぞれの出力 JSON から集計し直したもの。

### `git diff --stat 469050731104d162f396ddfd0f40dbcb84880887`

```
 docs/progress/S1-report.md | 372 +++++++++++++++++++++++++++++++++++++++++++++
 docs/progress/S1-review.md | 298 ++++++++++++++++++++++++++++++++++++
 2 files changed, 670 insertions(+)
```
（この節を足す前の値。この節の行数ぶん S1-report.md が増える。）コード・テストの変更は無い（S1 は実装しない段階。テストは 0 本のまま減っていない）。`_reference/` の差分は 0 行（`git diff --stat 4690507 -- _reference` が空）。

---

## S1 後の開発者決定（2026-10-07）

報告後の対話で、開発者が次を決めた。S2 以降はこれを前提にする。

1. **案 B（繋ぎ表方式）を採る**（HANDOVER §12-3 を決定済みに更新）。本家との一致は、推奨どおり masked（uncapped の表＋本家が単位上限に達する KA の単位集合）で取る。
2. **本家を正とし、本アプリは追従する。**
   - 本家で非対応のフレーム消費（連打キャンセル・連打版・通し値・隣り合いの扱い等）は、本家で直ったらそれを参考にこちらも直す。こちらが独自に計算方式を変えたり、切り替え式の別実装を持ったりはしない。
   - そのため S3 は、本家のファイル構成・関数名にできるだけ合わせて移植する（本家の差分を同じ場所に当てられるように）。単体版で変える箇所（`alreadyAdopted` の削除、DB 読込 → JSON 読込 等）は一覧にして残す。本家のテスト（golden 等）は移植し、本家が足したら追従する。検知と取り込みは `upstream/watch.sh` と `/sync_upstream`。
3. **再計算は必要な分だけにする。**
   - 変更の検知は技（move）単位。計算に効く列（発生・持続・全体・連打キャンセル値・派生関係・各種フラグ等）だけの指紋を技ごとに取る。表記・名前・メモだけの変更では表を作り直さない。本家の更新が技単位であることに合わせる。
   - 表の作り直しはキャラ単位（1 つの技の変更が、その技を含む全組み合わせ・連打単位・単位上限の効き方に波及するため。作り直しは 1 キャラ数秒）。
   - 出力のファイル名はキャラ単位の指紋にし、変わらないキャラはブラウザのキャッシュが効くようにする（HANDOVER §3 の `/data/<dataHash>/` 全体ハッシュは S4 で見直す）。
4. **生成物の扱い**: 確認用はローカル PC で回してよい。公開に使う生成物は CI からだけ出す（AGENTS §8-12）。ただし GitHub Actions の枠は他のリポジトリと共有なので、**動かしてみて負担が想定より大きければ、この規約を緩和する**（その時点で開発者が判断）。
