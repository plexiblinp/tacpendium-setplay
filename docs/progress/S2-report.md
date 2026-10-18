# S2 データパイプライン — 完了報告

- 段階開始コミット: `eea270c3eaba39b416ff8a7ff6bb359a56a79d7f`
- ブランチ: `claude/stage-2-08rivo`

## 計画

### Context
HANDOVER §9 S2: 「tacpendium 取得 → moves DB → 4 表記 → CI 検査(§4.3)」。**完了条件: 31 キャラ分の JSON と検査が緑**。
S1 の結論と開発者決定（2026-10-07）を前提にする: 案 B 採用・本家を正として追従（移植コードは本家のファイル構成・関数名に合わせる）・変更検知は技単位の指紋・取込元は公開版 tacpendium のみ（D2、lock = `ffdb69e4…`、000013 まで。000014 は公開版に無い）。
開始コミット: `eea270c3eaba39b416ff8a7ff6bb359a56a79d7f`（ブランチ `claude/stage-2-08rivo`）。

### 調べて分かった事実（計画の根拠）
- エンジンが読む列（`_reference/.../repository/setplay/repository.go`）: code, category, original_move_id, startup, active, total, damage, is_derived, is_aerial, is_projectile, **setup_only**, startup_basis, chain_cancel_total, fastest_unreachable, on_block ＋ move_derivations。**`setup_only` は CSV に無く DB（000004）にしか無い**。並びは **id 昇順**（決定性に効く）。
- 派生（000006）・安全飛び（000010, `json_set`）・`custom_states`（000003）・データ修正（000011/000013, UPDATE）は SQL でしか取れず、派生は数値 id 参照。
- 公開版に表記生成器 `internal/seedgen`（`GenerateAliases`、被りは落とす）と `internal/moveindex` がある（AGPL、移植可）。公開版の `go.mod` は go 1.26・`modernc.org/sqlite`（BSD-3、pure Go）。
- 既存 CI は root `go.mod` だけを見る。`REUSE.toml` 層 B = `data/**`、`scripts/check-licenses.py` の PENDING に `data/**: S2`。
- `upstream/watch.sh` の見張りに `internal/seedgen`・`internal/moveindex` が入っていない。

### 方針（自分で決めたもの。報告の「判断と理由」に書く）
1. **DB 経由で読む**: 公開版の `migrations/*.up.sql`（000001〜000013）を番号順に `modernc.org/sqlite` の一時 DB へ流し、本家の repository と同じ列・同じ順で読む（SQL を手で解析しない。本家のエンジンが見る値と同一にするため）。依存は取込 CLI 側だけ。将来の `engine/` は標準ライブラリのみのまま。
2. **取得**: `scripts/fetch-upstream.sh` が `upstream/tacpendium.lock` の SHA で公開版を shallow 取得（`git init`+`fetch --depth 1 <sha>`）し、パスを出す。combomgr・`_reference` はデータ源にしない。
3. **移植**: `internal/seedgen`（`csv.go`・`model.go`・`format.go`・`generate_m2002.go` と依存分）と `internal/moveindex` を**本家と同じパス・名前**で移植し、該当テストも移植。被りを落とす前の値を取り出すための最小の追加は別ファイル（例 `seedgen/standalone.go`）に置き、本家ファイルの変更点は `docs/upstream-diff.md` に一覧化（S3 も追記していく）。`upstream/watch.sh` の WATCH に 2 パスを足す。
4. **moves DB の形**（HANDOVER §4.1 案をベースに）: `data/moves/<char>.json`
   - `dataVersion`（lock SHA）、`character{code,name{ja,en}}`、`states[{code,name{ja,en}}]`、`safeJump{true,f5,f6}`
   - `moves[]` は **DB id 昇順**（id 自体は出さない）。各技: code, category, startup, active, recovery, total, onHit, onBlock, damage, isDerived, isAerial, isProjectile, setupOnly, startupBasis, chainCancelTotal, fastestUnreachable, originalMove(code|null), parents[code…], state(code|null), notation{nameJa,nameEn,srk,numeric}
   - NULL と 0/false を区別（AGENTS §8-7）: 数値は `null` 可、bool は常に出す。
   - `calcFingerprint`（計算に効く列だけの sha256、キャラ単位。開発者決定 3 の土台）と `data/moves/index.json`（キャラ一覧・各ファイルの指紋・dataVersion）。
   - **逸脱候補として報告**: §4.1 の `approachMoves`/`rapidCancel`（approach.go/rapid.go のデータ化）は入れない。開発者決定 2（本家の構成に合わせ差分を同じ場所へ当てる）と衝突するため。S3 でコードのまま移植。
5. **4 表記**:
   - nameJa = CSV `name_ja`、numeric/SRK = seedgen の被り除去前の値、nameEn = `move_code` からの規則（強度・OD・Aerial・Drive Rush・段数・throw_forward 等）＋上書き表 `data/overrides/name_en.json`。
   - 状態印: `condition_ja/en` と `custom_states` から合成（例 `[Denjin] 236P`）。被りが残れば空中印・ホールド/Lv・派生（親 > 子）の区別規則を追加し、規則で表せない分は `data/overrides/notation.json`（理由つき）。上書き表は公式サイト等から転記しない（NOTICE §2）。
   - 表記が生成できない行（numeric 43・SRK 42）は規則（派生は親表記＋`>`等）か上書きで埋める。
6. **生成物のコミット**: `data/moves/*.json` はコミットする（S4 の入力・上流更新時の差分レビューのため）。ただし「生成物は CI から」（§8-12）を担保するため、**CI に再生成→`git diff --exit-code` の一致検査**を置く（手元の生成物が CI の生成と 1 バイトでも違えば赤）。推測: moves DB は公開物ではなく中間データなので、この形で §8-12 の趣旨を満たす。

### 作るファイル
- `go.mod`/`go.sum`（module `github.com/plexiblinp/tacpendium-setplay`、go 1.26 系に合わせる）
- `internal/seedgen/…`・`internal/moveindex/…`（移植＋テスト）
- `internal/movesdb/`: `load.go`（migrations→SQLite→行）、`notation.go`（4 表記・状態印・区別規則）、`name_en.go`、`model.go`（JSON の型＝スキーマ、godoc）、`write.go`（決定的な JSON 出力）、`check.go`（§4.3 検査）＋各 `_test.go`
- `cmd/movesdb/main.go`（`-src <公開版 checkout> -out data/moves`、検査も実行し違反で exit 1）
- `scripts/fetch-upstream.sh`
- `data/moves/*.json`（31）・`data/moves/index.json`・`data/overrides/name_en.json`・`data/overrides/notation.json`
- `docs/data/moves-db.md`（形・各列の意味・生成規則・再生成手順）、`docs/upstream-diff.md`
- `.github/workflows/ci.yml`: go job を `go-version-file: go.mod` に、**data job**（fetch → 再生成 → 検査 → `git diff --exit-code data/`、件数を job summary へ）を追加。Action は SHA 固定のまま。
- `scripts/check-licenses.py` の PENDING から `data/**` を外し EXPECT に `data/moves/ryu.json` を追加。`NOTICE` §5 に直接依存（modernc.org/sqlite 等）を記載。`upstream/watch.sh` WATCH 追加。
- `docs/progress/S2-report.md`（冒頭に計画節 → 最後に 4 項目）、`S2-review.md`

### 検査（CI で緑にするもの）
`go test ./internal/movesdb/...`（コミット済み `data/moves` を読む。ネット不要）:
1. 31 キャラ・各ファイルがスキーマどおり（未知キー無し・必須キー有り・dataVersion = lock）
2. **全技が 4 表記を持つ**（空文字なし）
3. **同一キャラ × 同一表記種で表示文字列の完全一致 0 件**
4. CSV と DB の計算列一致（取込時）、parents/originalMove が同キャラの実在 code
5. オラクル突合: 生成 numeric/SRK と公開版 DB の preset 3/5 の差を分類（DB 側が被りで欠落／生成値＝DB 値＋区別印／上書き表に理由つき）。**説明のつかない差は赤**。件数を報告へ。
6. `index.json` の指紋がファイル内容と一致
加えて data job の再生成一致検査。

各検査は §8-3 どおり一度わざと壊して赤を確認（例: 1 技の nameEn を他技と同じにする→検査 3 が赤、notation を空に→2 が赤、ファイルを 1 バイト変える→再生成一致が赤、override の理由を消す→5 が赤）。§8-5: CLI→JSON の通し（fetch→生成→読込）を data job に置き、配線を 1 行消すと赤になることを確かめる。

### 進め方（/stage の手順）
1. `S2-report.md` 計画節を書いてコミット・push
2. go.mod＋seedgen/moveindex 移植（テスト緑）→ commit
3. load（SQLite）→ JSON 出力 → commit
4. 表記・状態印・区別規則・nameEn・上書き表 → 検査 2/3/5 を緑に → commit
5. CI data job・ライセンス・NOTICE・docs → commit・push、CI の結果を確認
6. 新規 general-purpose サブエージェントで `/review` 手順のレビュー（`S2-review.md`）→ 取り込み（最大 2 往復）
7. 完了報告（`go test -count=1 ./... > file; echo EXIT=`、件数逐語、`git diff --stat eea270c`）→ 検査を再実行して一致確認 → commit・push、ローカル/リモート SHA 一致を確認して停止

### 未決事項と扱い
- 区別規則で被りが消えず、上書き表が大量（目安 100 件超）になる場合 → D9/§4.3 の運用に関わるため、件数と選択肢を添えて聞く。それ以外は聞かずに進める。

---

## やったこと

成果物（基準 `eea270c` からの追加。全体の diff は末尾）:

- **取得**: `scripts/fetch-upstream.sh` — 公開版 tacpendium を lock の SHA（`ffdb69e4…`）で `.upstream/tacpendium` へ shallow 取得し、取得 SHA が lock と一致することを確かめる。`.upstream/` は `.gitignore`。
- **移植**（本家と同じパス・名前。変更点は `docs/upstream-diff.md`）: `internal/seedgen`・`internal/moveindex`・`internal/sanumber`・`internal/gameversion`（テスト込み）。本家ファイルへの変更は 2 つだけ: `generate_test.go` の `repoRoot`（比較先を公開版のチェックアウトへ）と、`generate_m2002.go` の `resolveCharacter` を `resolveCharacterWith` へ切り出し（衝突の解き方の差し替え口）。単体版の追加は `seedgen/standalone.go`（被りを落とさない `ResolveAllAliases`・`MovementAlias`）。
- **moves DB の生成**: `internal/movesdb`（`load.go` SQLite 取込 / `build.go` 組み立てと指紋 / `notation.go` numeric・SRK と区別印 / `name_en.go` 英語技名 / `overrides.go` 上書き表 / `check.go` 検査 / `write.go` 決定的な JSON 出力）と CLI `cmd/movesdb`。
- **データ**: `data/moves/<31 キャラ>.json` と `data/moves/index.json`、`data/overrides/name_en.json`（7 件）・`data/overrides/notation.json`（0 件）。
- **CI**（`.github/workflows/ci.yml` の Go job）: `go-version-file: go.mod`・公開版の取得・「moves DB の再生成と一致」（CLI で作り直して `diff -r data/moves`）・要約を job summary へ。Summary 段の既存の不具合（`grep -c` が 0 件で exit 1 → `bash -e` で段が赤）を直した。
- **その他**: `REUSE.toml`（`docs/upstream-diff.md` を層 C へ、層 B の注記）、`scripts/check-licenses.py`（PENDING を空に・EXPECT に代表パス 4 本）、`NOTICE` §2・§5（依存と移植元）、`DATA-LICENSE.md` §4、`upstream/watch.sh`（見張りに移植 4 パッケージ）、フック 2 本（`.upstream/` を走査対象から外す）、`docs/data/moves-db.md`（形・列の意味・生成規則・検査）。

生成の要約（`go run ./cmd/movesdb` の標準出力・逐語）:

```
dataVersion: ffdb69e468a3f5c0b0e16255e68b6baa3266796d
キャラ: 31 / 技: 3057
numeric: 出所 {generator:2709 movement:279 name:69} / 区別印の付いた技 436(段ごと {aerial:44 label:235 name:138 od:45})/ 名前フォールバックの理由 {derived-no-command:20 index-skip:5 no-command:6 not-in-csv:35 rush-original-unfilled:3}
srk: 出所 {generator:2709 movement:279 name:69} / 区別印の付いた技 390(段ごと {aerial:44 label:319 od:45})/ 名前フォールバックの理由 {derived-no-command:20 index-skip:5 no-command:6 not-in-csv:35 rush-original-unfilled:3}
オラクル numeric: 本家と同じ 2635(うち区別印つき 118)/ 本家が被りで落とした 344 / 本家が元技の表記なしで落としたラッシュ版 9 / 上書き 0 / 本家にも生成器にも無い 69
オラクル srk: 本家と同じ 2650(うち区別印つき 91)/ 本家が被りで落とした 330 / 本家が元技の表記なしで落としたラッシュ版 8 / 上書き 0 / 本家にも生成器にも無い 69
英語技名: {override:7 override-of-original:2 rule:3048} / 表記の上書き: 0
name_ja と本家 official_ja_move の差: 0

違反: 0 件
書き出し: data/moves(31 キャラ + index.json)
```

読み方:
- 技 3,057 = CSV 2,743 ＋ CSV に無く本家 DB が seed する 314（移動系 9 code × 31 = 279、`drive_reversal` 31、連打版 4）。
- 「本家が被りで落とした」「…ラッシュ版」は、本家の規則（seedgen の `resolveCharacter`・FormatCurrent）で回し直したときに、その技が実際に理由 collision / rush-original-unfilled で落ちることを確かめた数（レビュー中-2 で締め直した。それまでは「他の技と被っている」だけで数えていた）。
- 「本家と同じ」は、区別印を付ける前の表記が本家 DB の preset（numeric 2,635 行・SRK 2,650 行＝DB の全行）と完全一致した数。**DB にある行は全件一致**（ただし S1 ③ と同じく、DB は同じ生成器の出力なので独立の検証ではない。移植の忠実さの確認である）。
- 名前フォールバック 69 = 本家の生成器が表記を作れない 34（command 空の派生 20・command 空 6・語彙外 5・元技の表記が無いラッシュ 3）＋ CSV に無く DB にも preset が無い 35（`drive_reversal` 31・連打版 4）。S1 ③ の「表記なし numeric 43・SRK 42」との差は、S1 の値が被り落ちと合算で数えた別の切り口のため（本 S2 の 34 は生成器の `UnfilledRow` から被りを除いた数）。推測: 差の内訳は確かめていない。

検査（§4.3 ほか）は `internal/movesdb` と `cmd/movesdb` のテスト、CI の再生成一致で回る:

| 検査 | どこで | わざと壊して赤を確かめた方法（§8-3） |
|---|---|---|
| 全技が 4 表記を持つ | `CheckFile`（生成時・`TestCommittedMovesDB`） | 合成: SRK を空白に → 赤（`TestCheckFile_DetectsEachViolation/空の表記`）。実データ: `Build` で SRK を "" にする → `TestPipeline` が 32 ファイル不一致で赤 |
| 同一キャラ × 同一表記種で完全一致 0 | 同上 | 合成: nameEn・numeric を他技と同じに → 赤。実データ: ryu.json の nameEn を隣の技と同じにして sha を合わせる → `TestCommittedMovesDB` が `nameEn "Forward Dash" が 2 技で同じ` で赤 |
| 必須キー（欠けたキーが黙って既定値にならない） | `ReadAll` の `missingKeys` | 合成: `moves[0].startup`・`notation.srk`・`safeJump.f5`・`states[0].name.en`・`moves[0].state`・index の `name.en`・`fileSha256` を消す → 検出（`TestMissingKeys`）。入れ子への再帰を外す → そのテストが赤 |
| `state` が `states` にあり、状態 code の最長一致、name_ja の印で確かめる | `CheckFile`・`TestStateOf`・`TestMoveStates_RequiresBracketLabel` | 最長一致を「最初に当たったもの」に戻す → `TestStateOf`・`TestGenerateNameEN` が赤。name_ja の確認を外す → `kachousen_light` に state が付き赤 |
| 形・index・指紋 | `ReadAll`・`CheckFile` | ryu.json を 1 バイト変える → `fileSha256 が中身と合わない` で赤。指紋を作り直さずに startup を変える → 技・キャラ両方の指紋で赤（この検査がキャラ単位の指紋の欠陥を見つけ、直した） |
| オラクル突合 | `CheckOracle`（`TestPipeline`・CLI） | seedgen の `StandingPrefix` を `5`→`N` → `基底 "NLP" が本家 "5LP" と違う` で赤。合成: 本家に表記が無く、本家の規則では落ちない技（基底だけ他の技と被る）→ 違反（`TestCheckOracle_RequiresUpstreamToActuallyDrop`）。判定を「何でも説明がつく」に緩める → そのテストが赤 |
| 再生成一致（手元の生成物 = CI の生成） | `TestPipeline_RegeneratesCommittedData`・CLI の `TestRun_MatchesCommittedData`・CI の `diff -r` | 上の 1 バイト変更・`Build` の配線外し。CLI は書き出し先をずらす → `index.json` が無く赤 |
| 空中印が is_aerial を使わない | `TestNotation_DistinctMarks` | 空中印を is_aerial から作る形に戻す → 3 件赤 |
| CSV と DB の計算列一致 | `CheckCSVAgainstDB`（生成時・`TestPipeline`） | 合成: CSV の total・is_projectile を変える → 検出（`TestCheckCSVAgainstDB_DetectsMismatch`）。total の比較を外す → そのテストが赤 |
| 移植した本家テスト | `internal/seedgen` 等 | 公開版の写しの ryu.csv で `hadoken_light` の command を変える → seedgen 11 件赤。`TACPENDIUM_SRC` を存在しない場所に → 赤（skip しない） |

テスト（逐語）:

```
$ go test -count=1 -v ./... > /tmp/s2-go-v.txt 2>&1; echo EXIT=$?
EXIT=0
PASS=150 FAIL=0 SKIP=0     （grep -cE '^\s*--- PASS' / '--- FAIL' / '--- SKIP'。サブテストを含む）
ok  	github.com/plexiblinp/tacpendium-setplay/cmd/movesdb
ok  	github.com/plexiblinp/tacpendium-setplay/internal/gameversion
ok  	github.com/plexiblinp/tacpendium-setplay/internal/moveindex
ok  	github.com/plexiblinp/tacpendium-setplay/internal/movesdb
ok  	github.com/plexiblinp/tacpendium-setplay/internal/sanumber
ok  	github.com/plexiblinp/tacpendium-setplay/internal/seedgen
```

パッケージ別の PASS 件数: seedgen 95・moveindex 12・sanumber 17・gameversion 3（**公開版で同じパッケージを回した件数と同じ**＝移植でテストは減っていない。`func Test` の数も seedgen 78 = 78）・movesdb 22・cmd/movesdb 1。

その他の検査: `go mod tidy -diff` EXIT=0（CI にも段を足した）、`go vet ./...` EXIT=0、`gofmt -l`（cmd・internal）出力なし、`python3 scripts/check-licenses.py` EXIT=0「違反: 0 件」、`bash scripts/check-actions-pinned.sh` EXIT=0「違反: 0 件」、フック・スクリプトの `bash -n` 全件 OK。

## 判断と理由

- **SQLite に migrations を流して読む（CSV を直接読まない）。** `setup_only` は CSV に無く DB（000004）にしか無い。派生（000006）は数値 id 参照、安全飛び（000010）は `json_set`。SQL を手で解析すると本家のエンジンが見る値とずれうる。依存 `modernc.org/sqlite`（BSD-3・pure Go。本家と同じ版 v1.58.0）は取込 CLI 側だけで、将来の `engine/` は標準ライブラリのみのまま。案: (b) SQL の手解析 → UPDATE・json_set の再現が要り、壊れ方が静か。採らない。
- **本家の生成器を移植し、被り落としだけを差し替えた。** 開発者決定 2（本家に合わせ差分を同じ場所へ当てる）。生成器を書き直すと、本家の規則変更（M48-08 のような）を追えない。差し替えは関数の切り出し 1 か所に留めた。
- **区別印は「被った組の中でだけ、違う部分を印にする」汎用規則にした（上書き表ではなく）。** 被りは numeric 250 組・SRK 235 組（約 550 技）あり、上書き表では保守できない。被らない技は本家と同じ文字列のまま（オラクルで確認できる）。段の順は、言語に依らない印（空中・OD）→ ラベル → 名前。案: (b) 全技に状態印を付ける → 本家との差が増え、HANDOVER の例（`[Denjin] 236P`）以上の情報が無い。(c) 翻訳表でラベルを英訳 → 35 種の状態ラベル・71 種の括弧ラベルの英訳を人が持つことになる。SRK は `move_code` の語（英語）から作れるので不要と判断。
- **空中印に `is_aerial` を使わない。** 最初は `is_aerial` で作っていたが、ドメイン知識集成 §7-2 で `is_aerial` は「ラッシュ版の自動生成から外す」印と確認し、`move_code` の語（aerial・air・jumping_・neutral_jump・high_jump）へ直した（コミット `3a39d2b`）。テストで is_aerial をわざと逆に立てて固定している。
- **numeric のラベルは日本語（name_ja の括弧）、SRK は英語（move_code の語・状態の英語名）。** HANDOVER §4.1 の例（numeric `[電刃] 236P`・SRK `[Denjin] 236P`）に合わせた。ただし印は略さず状態名をそのまま使う（`[電刃錬気] 236P+P`・`[Denjin Charge] 236P+P`）。推測: 略称の表は持たないほうが保守が軽い。
- **名前フォールバックは numeric = nameJa、SRK = nameEn。** 本家の層 C-3（入力の無い派生は name_ja と humanize）と同じ考え方で、新しい入力情報を作らない。本家の層 C-3 に当たる 13 技は本家の値（SRK は `alias_text_en` の humanize＝小文字。例 `razors edge slicer OD`）をそのまま使う（本家と同じ文字列を優先。見た目は残課題）。
- **英語技名は規則＋上書き 7 件（ジェイミーのみ）。** S1 の推定 10〜30 件より少ない。規則を足して吸収した: 状態の段階（`[Drink Level 4]`）・`throw_<向き>`・移動系 9 code の固定名・`rush`→Drive Rush・ホールド・段数・`up<n>`→`(+n)`・所有格・`during` 以降は語順のまま。**全 3,057 件の目視はしていない**（ジェイミー全件と、無作為 60 件・怪しい語の機械抽出 33 件を見た）。
- **生成物（`data/moves`）をコミットし、CI の再生成と 1 バイト一致を検査する。** S4 の入力であり、上流更新時の差分をレビューできる。AGENTS §8-12「生成物は CI からだけ出す」は公開物（事前計算・サイト）を対象と読み、中間データは「手元の生成物が CI の生成と一致すること」を CI が保証する形で趣旨を満たすと判断した。この扱いは報告後に開発者決定で確定した（末尾「S2 後の開発者決定」）。
- **指紋**: 技単位（計算に効く列だけの sha256 先頭 16 桁）とキャラ単位（全技の指紋＋安全飛び）。開発者決定 3（技単位の変更検知・キャラ単位の作り直し）の土台。`recovery`・`onHit`・表記は入れない。`onBlock` は絞り込みに効くので入れた。
- **再生成一致の CI 段は計画の「data job」ではなく Go job の 1 段にした（計画からの変更）。** 公開版の取得とテストを共有でき、分数を節約できる。同じ検査を go test（`TestPipeline_RegeneratesCommittedData`）にも置いたので、CI の段は要約を残す役。
- **`approachMoves`・`rapidCancel`（HANDOVER §4.1 の案）を JSON に入れなかった（逸脱）。** §4.1 は「形（案）」で、`approach.go`・`rapid.go` のハードコード表をデータ化する案だった。開発者決定 2（本家の構成に合わせ、本家の差分を同じ場所へ当てる）と衝突するため、S3 でコードのまま移植する。
- **CSV に無い技（移動系・`drive_reversal`・連打版）も moves DB に入れた。** 本家のエンジンは `moves` の全行を見る（repository の SQL に絞りが無い）ため。nameJa は本家 DB の `official_ja_move`。
- **`go.mod` は本家と同じ go 1.26.4 / toolchain go1.26.8。** modernc.org/sqlite の要求と本家に合わせた。
- **技ごとの `state` は「`move_code` が `custom_states` の状態 code で始まり、かつ `name_ja` の括弧ラベルに状態の日本語名が含まれる」で決める（計画の「`condition_ja/en` と `custom_states` から合成」からの逸脱）。** `condition_ja/en` に値があるのは公開版 CSV 全行で 2 行だけ（ingrid `ca_cosmic_ray`・lily `double_arrow`。どちらも状態ではなく発動条件）で、状態の材料にならない。接頭辞だけで決めると、舞の `kachousen_*`（状態「花蝶扇バウンド中」を**作る**技）やブランカの `blanka_chan_bomb_activated`（人形の数の状態）にも付いてしまう（再レビュー高-1・中）ので、`name_ja` の印で確かめる。状態 code どうしが接頭辞の関係（ingrid `order_of_the_sun`／`_strength`、blanka `blanka_chan_bomb`／`_is_set`）にあるので長いほうを採る。付いたのは 5 キャラ 68 技（ryu 8・jamie 23・lily 12・blanka 21・e_honda 4）。英語技名と SRK の状態印も同じ判定を使う。numeric の状態印は `name_ja` の括弧（`custom_states` に無い `【ジャスト】` 等も含む）から作るので、`state` とは別物（`state` は絞り込み用、印は区別用）。なお `state` は当初の実装で漏れており、レビュー中-1 で足し、再レビューで判定を直した。
- **`SchemaVersion` は 1 のまま。** `state` を足したが、moves DB の利用者はまだ無い（S3 から）。最初に使われる形を 1 とする。
- **必須キーの検査**（計画の検査 1）も当初漏れており、レビュー中-1 で `ReadAll` に足し、再レビュー高-2 で入れ子（`states[]`・`character.name`・`safeJump`・各技・`notation`）と `index.json` まで再帰で見る形にした（`encoding/json` は欠けたキーを黙って既定値にするので、キーの有無を別に見る）。
- **同じキャラに見つからない親・元技で生成を止める（本家は黙って飛ばす）。** 取込の時点で上流の変化に気づくため。`build.go` のコメントと `docs/data/moves-db.md` に書いた（レビュー低-5）。

## 残課題・既知の問題

- **`state` が付かない状態つきの技**: name_ja に状態の印が無い技（例 lily `windclad_od_condor_dive_follow_up`「ODコンドルダイブ(派生)」）は、move_code が状態で始まっても `state` が付かない（誤って付けるより、付けないほうに倒した）。逆に、状態の前提が name_ja にも move_code にも出ない技は拾えない。S5 で状態の絞り込みを作るときに見直す。
- **表記の見た目**: ラッシュ版の状態印は `[酔いレベル3] DR > 4HP` のように DR の前に付く（区別はできる）。numeric の名前フォールバック 138 技（例 `236LK (弱空中バーニングキック)` は空中印で解けたが、`63214LP (弱ヨガフレイム)` / `(弱ヨガコメット)` のように別技が同じ入力のものは名前を添えている）。SRK の層 C-3 13 技は本家の小文字 humanize（`razors edge slicer OD`）。ラベル段の SRK は `move_code` の語をそのまま括弧に入れる（`6K (during gorai axe)`）。どれも区別はできているが、整える余地がある（上書き表で個別に直せる）。
- **英語技名は全件目視していない**（上記）。コード命名の不整合はジェイミー以外にもありうる（`razors` のような所有格の落ち等）。
- **000014 が公開版に無い**（S1 ⑤）。キンバリーの疾駆け派生 8 技の親（`move_derivations`）が moves DB の `parents` に入っていない。公開版が出したら `/sync_upstream` で取り込む。
- 名前フォールバック 69 と S1 ③ の「表記なし 43/42」の対応は確かめていない（上記「読み方」）。
- `setup_only` は本家でも意味が照会中（ドメイン知識集成 §7-3）。値はそのまま運んでいる。
- オラクル突合は本家 DB と同じ生成器どうしの比較なので、表記が正しいことの独立の検証ではない（S1 ③ と同じ限界）。

## 次の段階への入力（S3 エンジン）

- moves DB の読み方: `internal/movesdb.ReadAll("data/moves")`（未知キーは誤り）か、`data/moves/<code>.json` を直接。型の正本は `internal/movesdb/model.go`。**ただしエンジン（`engine/`）は `internal/movesdb` を import しない**（sqlite 依存を持ち込まないため）。エンジン側に JSON の読み口を別に作るか、型だけを依存の無いパッケージへ分けること。
- `moves` は本家の moves.id 昇順。本家の repository は `ORDER BY m.id` で読むので、配列の順がそれに当たる。数値 id は出していない（`originalMove`・`parents` は code で引く）。`parents` は code 昇順（本家のサービス層が「move_code 昇順の先頭」を採る）。
- 本家のエンジンが読む列は全部入っている（`repository/setplay/repository.go` の `MoveCandidate` の全列）。列の意味は `docs/data/moves-db.md`（ドメイン知識集成 §7 の注記つき）。`isDerived`・`isAerial` を名前どおりに使わないこと。
- 安全飛びは `safeJump`（本家 `custom_states.safe_jump`）。キャラ固有の接近技（`approach.go`）と連打（`rapid.go`）はコードのまま移植する（上の逸脱）。
- golden の fixture を作るときは、`data/moves` の実データを使える（合成データの限界を減らせる）。
- 移植の作法: 本家と同じパス・名前に置き、本家から変えた箇所は `docs/upstream-diff.md` に追記する。`upstream/watch.sh` の見張りに足す。

## 最終確認（報告を書いた後に回し直した結果）

HEAD `4981961` で回した（この節を足すコミットは本報告だけを変える）。

```
$ go test -count=1 -v ./... > /tmp/f-go.txt 2>&1; echo EXIT=$?
EXIT=0
PASS=150 FAIL=0 SKIP=0
$ go vet ./...                     → EXIT=0
$ gofmt -l（追跡中の .go、_reference 除く）→ 出力なし
$ go mod tidy -diff                → EXIT=0
$ python3 scripts/check-licenses.py → EXIT=0 違反: 0 件
$ bash scripts/check-actions-pinned.sh → EXIT=0 違反: 0 件
$ go run ./cmd/movesdb -out <tmp>  → EXIT=0 違反: 0 件
$ diff -r data/moves <tmp>         → EXIT=0（差なし）
```

削除されたファイル: 0（`git diff --diff-filter=D --name-only eea270c3eaba39b416ff8a7ff6bb359a56a79d7f HEAD`）。テストファイルの差分は追加のみ（`-- '*_test.go'`: 21 files changed, 4343 insertions(+)。うち移植した本家のテストが大半）。

### `git diff --stat eea270c3eaba39b416ff8a7ff6bb359a56a79d7f HEAD`

```
 .claude/hooks/session-start.sh                 |    4 +-
 .claude/hooks/stop-test.sh                     |    4 +-
 .github/workflows/ci.yml                       |   40 +-
 .gitignore                                     |    3 +
 DATA-LICENSE.md                                |    2 +-
 NOTICE                                         |   15 +-
 REUSE.toml                                     |    3 +-
 cmd/movesdb/main.go                            |  146 ++++++
 cmd/movesdb/main_test.go                       |   44 ++
 data/moves/aki.json                            | 2336 ++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
 data/moves/akuma.json                          | 3347 +++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
 data/moves/alex.json                           | 3078 +++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
 data/moves/blanka.json                         | 3648 ++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
 data/moves/c_viper.json                        | 2871 +++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
 data/moves/cammy.json                          | 3123 ++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
 data/moves/chun_li.json                        | 2750 +++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
 data/moves/dee_jay.json                        | 2680 ++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
 data/moves/dhalsim.json                        | 3121 ++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
 data/moves/e_honda.json                        | 2504 ++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
 data/moves/ed.json                             | 2404 ++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
 data/moves/elena.json                          | 3162 ++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
 data/moves/guile.json                          | 2792 ++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
 data/moves/index.json                          |  347 +++++++++++++
 data/moves/ingrid.json                         | 2898 ++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
 data/moves/jamie.json                          | 3919 ++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
 data/moves/jp.json                             | 2415 +++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
 data/moves/juri.json                           | 2370 +++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
 data/moves/ken.json                            | 2568 ++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
 data/moves/kimberly.json                       | 2989 ++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
 data/moves/lily.json                           | 2588 +++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
 data/moves/luke.json                           | 2643 +++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
 data/moves/m_bison.json                        | 2538 +++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
 data/moves/mai.json                            | 3016 +++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
 data/moves/manon.json                          | 2316 +++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
 data/moves/marisa.json                         | 3336 ++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
 data/moves/rashid.json                         | 3090 +++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
 data/moves/ryu.json                            | 2664 ++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
 data/moves/sagat.json                          | 2477 +++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
 data/moves/terry.json                          | 2260 +++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
 data/moves/yasmine.json                        | 2720 ++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
 data/moves/zangief.json                        | 2588 +++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
 data/overrides/name_en.json                    |   32 ++
 data/overrides/notation.json                   |    1 +
 docs/data/moves-db.md                          |  103 ++++
 docs/progress/S2-report.md                     |  177 +++++++
 docs/progress/S2-review.md                     |  265 ++++++++++
 docs/upstream-diff.md                          |   35 ++
 go.mod                                         |   19 +
 go.sum                                         |   50 ++
 internal/gameversion/gameversion.go            |   46 ++
 internal/gameversion/gameversion_test.go       |   97 ++++
 internal/moveindex/moveindex.go                |  264 ++++++++++
 internal/moveindex/moveindex_test.go           |  217 ++++++++
 internal/movesdb/build.go                      |  263 ++++++++++
 internal/movesdb/check.go                      |  288 +++++++++++
 internal/movesdb/check_test.go                 |  207 ++++++++
 internal/movesdb/data_test.go                  |   59 +++
 internal/movesdb/load.go                       |  275 +++++++++++
 internal/movesdb/model.go                      |  100 ++++
 internal/movesdb/name_en.go                    |  191 +++++++
 internal/movesdb/name_en_test.go               |   60 +++
 internal/movesdb/notation.go                   |  420 ++++++++++++++++
 internal/movesdb/notation_test.go              |  162 ++++++
 internal/movesdb/overrides.go                  |   77 +++
 internal/movesdb/pipeline_test.go              |   88 ++++
 internal/movesdb/write.go                      |  197 ++++++++
 internal/sanumber/sanumber.go                  |   53 ++
 internal/sanumber/sanumber_test.go             |   87 ++++
 internal/seedgen/classify_m2207_test.go        |  359 ++++++++++++++
 internal/seedgen/csv.go                        |  183 +++++++
 internal/seedgen/format.go                     |  115 +++++
 internal/seedgen/generate.go                   |  383 ++++++++++++++
 internal/seedgen/generate_m1403d_test.go       |  103 ++++
 internal/seedgen/generate_m1403e_test.go       |  164 ++++++
 internal/seedgen/generate_m1403f_test.go       |  349 +++++++++++++
 internal/seedgen/generate_m1702.go             |  217 ++++++++
 internal/seedgen/generate_m1702_test.go        |  219 ++++++++
 internal/seedgen/generate_m2002.go             | 1264 +++++++++++++++++++++++++++++++++++++++++++++++
 internal/seedgen/generate_m2002_golden_test.go |  282 +++++++++++
 internal/seedgen/generate_m2002_test.go        |  602 ++++++++++++++++++++++
 internal/seedgen/generate_m2003_test.go        |  124 +++++
 internal/seedgen/generate_m2006_test.go        |  358 ++++++++++++++
 internal/seedgen/generate_m2802a.go            |  132 +++++
 internal/seedgen/generate_m2802a_test.go       |  148 ++++++
 internal/seedgen/generate_m4808.go             |  182 +++++++
 internal/seedgen/generate_m4808_test.go        |  330 +++++++++++++
 internal/seedgen/generate_test.go              |  284 +++++++++++
 internal/seedgen/model.go                      |  129 +++++
 internal/seedgen/standalone.go                 |   40 ++
 scripts/check-licenses.py                      |    8 +-
 scripts/fetch-upstream.sh                      |   30 ++
 upstream/watch.sh                              |    4 +
 92 files changed, 97640 insertions(+), 21 deletions(-)
```

## S2 後の開発者決定（2026-10-07）

報告後の対話で、開発者が次を決めた（推奨案のとおり）。S3 以降はこれを前提にする。

1. **`data/moves`（moves DB）と `data/overrides`（上書き表）はコミットする。**
   - CI（Go job の「moves DB の再生成と一致」と `internal/movesdb` の `TestPipeline_RegeneratesCommittedData`）が公開版から作り直し、1 バイトでも違えば赤にする。手で編集しない。
   - AGENTS §8-12「生成物は CI からだけ出す」は公開物（S4 の事前計算結果・公開サイト）に当てる。中間データは上の一致検査で担保する。
   - 上流の取り込み（`/sync_upstream`）や上書き表・生成器の変更のときは、`go run ./cmd/movesdb` で作り直してコミットする。
