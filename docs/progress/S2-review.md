# S2 データパイプライン — レビュー

- 対象: ブランチ `claude/stage-2-08rivo`、HEAD `35e764221a258ab9de24d70098132e14ee5bae6b`
- 基準コミット: `eea270c3eaba39b416ff8a7ff6bb359a56a79d7f`（呼び出し元の指定。`S2-report.md` 計画節の記録と一致）
- 手順: `.claude/commands/review.md`。読み取り専用（本ファイル以外は変更していない）

## 1. 要約

- 完了条件「31 キャラ分の JSON と検査が緑」は満たしている。手元で `go test`（146 PASS / 0 FAIL / 0 SKIP）・CLI の再生成（`diff -r` 一致）・ライセンス検査・Action 固定検査を回し、報告の数値と一致した。CI の最新の実行（run 37565401192）も 3 job とも成功。
- moves DB は、本家 `repository/setplay` の `listCandidatesSQL` の全列（＋表示用の `recovery`・`onHit`）を id 昇順で運んでいる。派生は 000006 の 459 行と 000011 の 17 行を合わせた 476 本で、JSON の `parents` の総数と一致した。
- **高 3 件**: `docs/upstream-diff.md` に `generate_m2002.go` の変更と `standalone.go` が載っていない（コードのコメントは「docs/upstream-diff.md に載せた」と言っている）／完了報告に `git diff --stat` が無い（本文は「末尾」と言っている）／CI 先頭のコメントが、`setup-go` の `go-version-file: go.mod` と食い違う。
- **中 2 件**: 計画にあったのに実装も報告もされていない項目がある（技ごとの `state`・必須キーの検査・`condition_ja/en` を使う状態印）／オラクル突合の「説明のつく差」の判定が緩い。
- **低 5 件**: go.mod が tidy でない・`stateLabel` が最長一致でない（今のデータでは発火しない）・英語技名と SRK のラベル表記が揃っていない・CI の要約に SKIP 件数が無い・別キャラの親を持つ行で生成が止まる（本家は黙って飛ばす）。

## 2. 完了条件への適合

| 項目 | 判定 | 根拠 |
|---|---|---|
| HANDOVER §9 S2「31 キャラ分の JSON と検査が緑」 | 済 | `data/moves/` に 31 ファイルと `index.json`。`TestCommittedMovesDB` と CLI が「キャラ: 31 / 技: 3057」「違反: 0 件」を出す。go test は EXIT=0（§5） |
| §4.3 全技が 4 表記を持つ | 済 | `CheckFile` が空・空白だけの値を違反にする（`check.go:57-66`）。合成データで赤になるテストもある |
| §4.3 同一キャラ × 同一表記種で完全一致 0 件 | 済 | 同上。4 表記すべて（nameJa・nameEn を含む）を見ている |
| 計画: tacpendium の取得（lock の SHA） | 済 | `scripts/fetch-upstream.sh` が SHA の一致まで確かめる |
| 計画: seedgen・moveindex 等の移植とテスト | 済 | 公開版の同じ 4 パッケージを公開版のツリーで回すと 127 PASS。単体版でも 127 PASS（§5） |
| 計画: 本家ファイルの変更を `docs/upstream-diff.md` に一覧化 | **一部** | `generate_m2002.go` の変更と `standalone.go` が一覧に無い（高-1） |
| 計画: moves DB の形（技ごとの `state(code\|null)` を含む） | **一部（未報告の逸脱）** | `model.go` の `Move` に `state` が無い（中-1） |
| 計画: 検査 1「未知キー無し・**必須キー有り**」 | **一部（未報告の逸脱）** | 未知キーは `DisallowUnknownFields` で誤りになる。必須キーの欠落は検出しない（中-1） |
| 計画: 状態印を `condition_ja/en` と `custom_states` から合成 | **変更（報告に明記なし）** | `condition_*` はコードで使っていない。numeric は name_ja の括弧、SRK は move_code の接頭辞で作る（中-1） |
| 計画: オラクル突合（説明のつかない差は赤） | 済（判定は緩い） | `CheckOracle`。緩さは中-2 |
| 計画: CI で再生成し 1 バイト一致を検査 | 済 | Go job の「moves DB の再生成と一致」と `TestPipeline_RegeneratesCommittedData`・`TestRun_MatchesCommittedData`。data job ではなく Go job の 1 段にした点は報告に「計画からの変更」として書かれている |
| 計画: ライセンス宣言・NOTICE・watch.sh | 済 | REUSE 層 B=`data/**`、層 C に `docs/upstream-diff.md`。公開版 REUSE.toml で `migrations/*_data_*.sql` は CC-BY-SA なので、そこから作る JSON を層 B にするのは整合する |
| 完了報告に `git diff --stat <基準>`（AGENTS §8-11・計画手順 7） | **未** | 高-2 |
| `_reference/` を変更していない | 済 | `git diff --stat eea270c HEAD -- _reference` の出力が空 |

## 3. 指摘

### 高

**高-1 `docs/upstream-diff.md` に、本家ファイルの変更 1 件と追加ファイル 1 件が載っていない（docs とコードの食い違い）**
- 場所: `docs/upstream-diff.md:19-24`（「ファイルごとの変更」の表）。関係するコードは `internal/seedgen/generate_m2002.go:546-558, 680-685`・`internal/seedgen/standalone.go`
- 内容: 表に載っているのは `generate_test.go` と `testdata/` の 2 行だけ。import パスを揃えて公開版と diff を取ると、本家から変えたファイルは `generate_m2002.go`（`resolveCharacter` の本体を `resolveCharacterWith` へ切り出し、`collisionResolver` 型を追加し、`resolve != nil` なら差し替える）と `generate_test.go` の 2 つある。新規の `standalone.go` も表に無い（`grep -n "resolveCharacterWith\|standalone.go" docs/upstream-diff.md` は 0 件・EXIT=1）。一方、`generate_m2002.go` のコメントは「★単体版での変更(docs/upstream-diff.md)」、`standalone.go` のコメントは「本家に無い。docs/upstream-diff.md」と、一覧に載っている前提で書かれている。完了報告の「本家ファイルへの変更は 2 つだけ」とも合わない。
- なぜ問題か: 開発者決定 2 で、このファイルは「本家の差分を同じ場所へ当てる」ときの正本になる。`/sync_upstream` で `generate_m2002.go` に本家の差分を当てるとき、切り出しの存在を見落として衝突したり、変更を黙って失ったりする。テストは緑のままなので、読み手しか気づけない。
- 直し方: 表に 2 行足す。`generate_m2002.go`: 「`resolveCharacter` の本体を `resolveCharacterWith(…, resolve collisionResolver)` へ切り出し、`collisionResolver` 型を追加。`resolve` が nil なら本家と同じ挙動」。`standalone.go`: 「単体版だけの追加（`ResolveAllAliases`・`MovementAlias`・`keepAllCollisions`）」。あわせて、import パスを揃えた diff で一覧の漏れを検出する小さな検査（例: `upstream/watch.sh` か `/sync_upstream` の手順に、差分のあるファイルと表の行の突き合わせを足す）を置くと再発を防げる。

**高-2 完了報告に `git diff --stat <基準>` が無い（本文は「末尾」にあると言っている）**
- 場所: `docs/progress/S2-report.md:76`「成果物（基準 `eea270c` からの追加。全体の diff は末尾）」。報告の末尾は「次の段階への入力」で終わり、diff --stat は無い。`grep -n "diff --stat" docs/progress/S2-report.md` が当たるのは計画節（34・44・67 行）だけ。
- なぜ問題か: AGENTS §8-11 の必須項目（テストが黙って減っていないことの証拠）。計画手順 7 にも挙がっている。無いものを「末尾」にあると書いているのは §8-2「まだ無い成果物を過去形で書かない」にも当たる。
- 直し方: `git diff --stat eea270c3eaba39b416ff8a7ff6bb359a56a79d7f HEAD` を逐語で末尾に貼る（91 files changed, 93992 insertions(+), 18 deletions(-)。削除 18 行は既存ファイルの書き換え分で、テストファイルの削除は無い）。貼った後に検査をもう一度回し、報告と一致することを確かめる。

**高-3 CI 冒頭のコメントが Go job の実装と食い違う**
- 場所: `.github/workflows/ci.yml:1`（「Go(go.mod)…は、まだ無ければ『スキップ』を job summary に出して緑で終わる」）と `:63-66`（`actions/setup-go` に `go-version-file: go.mod`。「go.mod の有無」の判定より前に動く）
- 内容: S2 で `go-version-file: go.mod` にしたため、go.mod が無ければ `setup-go` の段が判定より先に失敗する。「スキップして緑」というコメントはもう成り立たない。今は go.mod があるので実害は無いが、コードと食い違うコメントは rubric 上「高」に当たる。
- 直し方: どちらかに揃える。(a) 判定の段を `setup-go` より前へ移し、`setup-go` にも `if: steps.detect.outputs.present == 'true'` を付ける（コメントのとおりに戻す）。(b) Go はもう常にある前提として、判定の段とコメントの Go の部分を消す。(a) のほうが Web job と形が揃う。

### 中

**中-1 計画にあったのに実装も報告もされていない項目がある（「判断」でも「逸脱」でもなく黙って落ちている）**
- 場所: 計画 `S2-report.md:26`（技ごとの `state(code|null)`）、`:32`（状態印を `condition_ja/en` と `custom_states` から合成）、`:50`（検査 1「必須キー有り」）。実装は `internal/movesdb/model.go`（`Move` に `State` が無い）、`internal/movesdb/write.go:72-121`（`DisallowUnknownFields` だけで、欠けたキーは Go の既定値に黙ってなる）、`internal/movesdb/notation.go`・`name_en.go`（`condition` を `grep -rni condition internal/movesdb cmd` で探して 0 件）
- なぜ問題か: AGENTS §8-2「要求を実装しなかったら『判断』ではなく『逸脱』と書く」。とくに `state` は HANDOVER §4.1 の形（案）にもあり、S5 で状態ごとに絞ったり印を出したりするときに、技の状態を move_code の接頭辞から推測し直すことになる。`approachMoves`・`rapidCancel` を入れないことは逸脱として書かれているのに、こちらは書かれていない。必須キーは、コミット済みデータについては `fileSha256` と再生成一致で守られているので実害は小さい。ただ、検査一覧（報告の表・`docs/data/moves-db.md` の「形どおり」）の読み手は欠落も検出すると受け取りうる。
- 補足: `condition_ja/en` に値があるのは公開版 CSV の全行で 2 行だけ（ingrid `ca_cosmic_ray`・lily `double_arrow`）。使わない判断には理がある。報告に書かれていないことが問題。
- 直し方: 報告の「判断と理由」に 3 点を逸脱として書く（`state` を出さない理由／状態印の作り方を変えた理由と `condition_*` が 2 行しか無い事実／必須キーは sha と再生成一致で代替していること）。`state` を出すなら、`stateLabel` と同じ接頭辞の規則で `state: code|null` を足し、`SchemaVersion` を上げる。

**中-2 オラクル突合の「説明のつく差」の判定が緩く、本家と違う値の一部を見逃しうる**
- 場所: `internal/movesdb/check.go:181`（`case baseCount[base] > 1: DroppedByUpstream`）、`:183`（`m.OriginalMoveID != nil && !hasAlias(...)` で無条件に `ReservedByUpstream`）
- 内容: 本家 DB に表記が無い技について、(1) 基底の表記が同じキャラの他の技と 1 つでも同じなら、本家が実際にその組を被りで落としたかを確かめずに「本家が被りで落とした」とする。(2) ラッシュ版は、元技に本家の表記が無ければ基底の値を問わず「予約で落とした」とする。生成器（移植）の不具合で、本家が出していない表記をたまたま別の技と同じ値で作っても赤にならない。
- なぜ問題か: 報告は「説明のつかない差は赤」を検査の柱にしている（報告の検査表）。実際の判定は「被りがある」ことしか見ていない。今回の数（numeric 349・SRK 329 が DroppedByUpstream）は大きく、この区分が中身を確かめずに通っている。
- 直し方: (1) は、seedgen を本家と同じ `dropCollisions`（または `keepPriorCollisions`）で回した結果の `collision` 一覧に、その技が入っていることを条件にする（`resolveCharacterWith` に本家の解き方を渡せば得られる）。(2) は、seedgen が `rush-original-unfilled`・reserved として落とした一覧と突き合わせる。直したら、わざと 1 技の基底を他技の値に変えて赤になることを確かめる（§8-3）。

### 低

**低-1 go.mod が tidy でない**
- 場所: `go.mod:16`（`modernc.org/sqlite v1.58.0 // indirect`）
- 内容: `internal/movesdb/load.go` が直接 import しているのに `// indirect` になっている。`go mod tidy -diff` は EXIT=1 で、sqlite を直接依存の `require` へ移し、go.sum に数行足す差分を出す。NOTICE §5 は「直接依存」として sqlite を挙げており、宣言と go.mod が合わない。
- 直し方: `go mod tidy` を回してコミットする。CI に `go mod tidy -diff` の段を足すことも検討する。

**低-2 `stateLabel` が最初に当たった状態を採る（最長一致でない）**
- 場所: `internal/movesdb/name_en.go:140-154`
- 内容: ingrid には `order_of_the_sun` と `order_of_the_sun_strength`、blanka には `blanka_chan_bomb` と `blanka_chan_bomb_is_set` という、片方がもう片方の接頭辞になる状態がある。今のデータでは両キャラとも、これらで始まる move_code が無いので発火しない（`blanka_chan_bomb_activated` は短いほうに正しく当たる）。上流の更新で長いほうの状態で始まる技が入ると、短いほうのラベルと誤った残りを返す。
- 直し方: 状態 code の長い順に試す。両キャラの形を合成テストに入れて固定する。

**低-3 英語技名と SRK のラベル表記が揃っていない**
- 例: jamie `rush_drink_level_3_hermits_elbow` の nameEn は `Drive Rush Drink Level 3 Hermits Elbow`、SRK は `DR > 4HP (drink level 3)`。ラッシュでない版は `[Drink Level 3] Hermit's Elbow`（上書き）。`rush_` の接頭辞があると状態が角括弧にならず、所有格の上書きも効かない。ryu の SRK `214214P (SA2) (lv1)` と numeric `(Lv1)` の大小文字も揃っていない。
- 区別はできている（検査は緑）。報告の残課題「表記の見た目」に含まれているとみなせるが、ラッシュ版の状態印が抜けることは書かれていない。直すなら `stateLabel` を `rush_` の後ろにも当てる。

**低-4 CI の Go 要約に SKIP 件数が無い**
- 場所: `.github/workflows/ci.yml` Go job の Summary 段
- 内容: pass と fail は数えているが skip は数えていない。本リポジトリは「skip して緑にしない」（AGENTS §4）を方針にしており、`upstreamSrc` も skip せず Fatal にしている。要約で skip 0 を見せると方針の証拠になる。
- 直し方: `skip="$(grep -cE '^[[:space:]]*--- SKIP' … || true)"` を足す。

**低-5 別キャラの親や元技を持つ行で生成が止まる（本家は黙って飛ばす）**
- 場所: `internal/movesdb/build.go`（`original_move_id … が同じキャラに無い`・`親 … が同じキャラに無い` で error）。本家 `service.go:1079-1081` は「別キャラ等で解決できない親は前置しない」で continue する。
- 今のデータでは起きない。厳しくする判断はありうるが、本家と挙動が違うことはコメントか docs に書いておくとよい（上流の更新で生成が止まったとき、原因を追いやすい）。

## 4. 良かった点

- 本家の repository と同じ列・同じ並び（`ORDER BY m.id`）で、migrations を流した SQLite から読んでいる。`setup_only`・派生（000011 の 17 行の追加を含む）・安全飛び・000011/000013 のデータ修正を取りこぼしていない。SQL を手で解析しない判断が効いている。
- NULL と 0・false を区別している（数値はポインタで `null`、真偽値は常に出す）。§8-7 に沿っている。
- `is_aerial` を空中印に使わない修正（ドメイン知識集成 §7-2）と、わざと is_aerial を逆に立てて固定するテスト。`docs/data/moves-db.md` の列の注記（`isDerived`・`isAerial`・`setupOnly`・`chainCancelTotal`）がドメイン知識集成 §7 と一致している。
- 再生成一致を、go test（`TestPipeline_RegeneratesCommittedData`）・CLI の通しテスト・CI の `diff -r` の 3 か所で見ている。公開版の取得が無ければ skip せずに落とす。
- 移植したテストの件数が公開版と同じ（127 = 127）ことを確かめられる形で報告している。
- `index.json` の `fileSha256` と、保存値ではなく中身から計算し直すキャラ単位の指紋の照合で、手編集を検出できる。

## 5. 自分で回したコマンドと結果

```
$ go test -count=1 -v ./... > /tmp/s2r-go.txt 2>&1; echo EXIT=$?
EXIT=0
grep -cE '^\s*--- PASS' → 146 / '--- FAIL' → 0 / '--- SKIP' → 0
ok  github.com/plexiblinp/tacpendium-setplay/cmd/movesdb
ok  github.com/plexiblinp/tacpendium-setplay/internal/gameversion
ok  github.com/plexiblinp/tacpendium-setplay/internal/moveindex
ok  github.com/plexiblinp/tacpendium-setplay/internal/movesdb
ok  github.com/plexiblinp/tacpendium-setplay/internal/sanumber
ok  github.com/plexiblinp/tacpendium-setplay/internal/seedgen

$ (公開版のツリーで) go test -count=1 -v ./internal/{seedgen,moveindex,sanumber,gameversion}/ > /tmp/s2r-up.txt 2>&1; echo EXIT=$?
EXIT=0   --- PASS 127
$ (単体版で同じ 4 パッケージ) --- PASS 127

$ go vet ./... ; echo EXIT=$?                              → EXIT=0
$ git ls-files '*.go' ':!:_reference/**' | xargs gofmt -l → 出力なし
$ python3 scripts/check-licenses.py ; echo EXIT=$?         → EXIT=0「違反: 0 件」
$ bash scripts/check-actions-pinned.sh ; echo EXIT=$?      → EXIT=0「違反: 0 件」
$ go run ./cmd/movesdb -out /tmp/s2r-moves > /tmp/s2r-movesdb.txt 2>&1; echo EXIT=$?
EXIT=0（要約は報告の逐語と同一。キャラ: 31 / 技: 3057 … 違反: 0 件）
$ diff -r data/moves /tmp/s2r-moves; echo EXIT=$?          → EXIT=0
$ go mod tidy -diff; echo EXIT=$?                           → EXIT=1（低-1 の差分）
$ git diff --stat eea270c HEAD -- _reference               → 出力なし
$ gh run view 37565401192                                   → Web ✓ / Go ✓ / リポジトリ検査 ✓（HEAD 35e7642 の CI）
```

突き合わせ（Python で集計）: JSON の `parents` の総数 476 = 000006 の 459 行 + 000011 で足す 17 行。`originalMove` 非 null 514、`chainCancelTotal` 非 null 93、`setupOnly` true 0（000004 の値どおり）、`safeJump` 31/31 キャラ。データ合計 2,190,555 バイト、jamie.json 99,195 バイト（報告と一致）。

移植の差分: import パスを `tacpendium-setplay`→`tacpendium` に置き換えて公開版と diff を取った。差があるのは `internal/seedgen/generate_m2002.go` と `internal/seedgen/generate_test.go` の 2 ファイルだけ（加えて `standalone.go` が単体版だけにあり、`testdata/` が公開版だけにある）。moveindex・sanumber・gameversion は import パス以外一致。

推測で書いたことは無い。ただし、中-2 の「生成器の不具合を見逃しうる」は判定式から導いたもので、実際に不具合を入れて通ることは確かめていない。

---

## 取り込み（1 往復目）

| 指摘 | 採否 | 理由・対応 |
|---|---|---|
| 高-1 upstream-diff の漏れ | 採用 | `generate_m2002.go`（`resolveCharacterWith` への切り出し）と `standalone.go` を表に追加。漏れの見つけ方（import パスを揃えた比較のスクリプト）も同ファイルに書いた |
| 高-2 diff --stat が無い | 採用 | 完了報告の末尾に `git diff --stat` を逐語で貼る（報告の最終確認節） |
| 高-3 CI コメントと setup-go の順 | 採用 | 案 (a)。「go.mod の有無」を `setup-go` より前へ移し、`setup-go` に `if` を付けた |
| 中-1 計画にあり報告に無い 3 項目 | 採用 | `state` を足した（`move_code` が状態 code で始まる技。7 キャラ 79 技）。必須キーの検査を `ReadAll` に足した（`missingKeys`・`TestMissingKeys`）。`condition_ja/en` を使わないことは逸脱として報告の「判断と理由」に書いた。`SchemaVersion` は利用者がまだ無いので 1 のまま（同節） |
| 中-2 オラクルの判定が緩い | 採用 | 本家の規則（`resolveCharacter`・FormatCurrent）で回し直した結果の理由（collision / rush-original-unfilled）と突き合わせる形に変えた（`seedgen.UpstreamUnfilled`）。合成テストで、旧来の緩い判定なら素通りする入力が違反になること、判定を緩めるとテストが赤になることを確かめた |
| 低-1 go.mod が tidy でない | 採用 | `go mod tidy`。CI に `go mod tidy -diff` の段を足した |
| 低-2 stateLabel が最長一致でない | 採用 | `matchState` で長いほうを採る。ingrid の形を合成テストに入れ、短いほうを採る形に戻すと赤になることを確かめた |
| 低-3 ラッシュ版の状態印・所有格、lv の大小 | 採用 | ラッシュ版も `rush_` の後ろで状態を見る（`Drive Rush [Drink Level 3] Hermit's Elbow`）。元技の英語名の上書きをラッシュ版へ引き継ぐ（2 件）。SRK のラベルの `lv1` を `Lv1` に |
| 低-4 CI 要約に SKIP が無い | 採用 | skip 件数を足した |
| 低-5 親が見つからないと止まる（本家は飛ばす） | 採用（記述のみ） | 止める挙動は変えない（上流の変化に気づくため）。`build.go` のコメントと `docs/data/moves-db.md` に違いを書いた |

---

## 再レビュー（2 往復目）

- 対象: HEAD `4264d8d7245dd554e38ca603a9a74136c11dc7d4`。取り込みの差分 `35e7642..4264d8d` と、基準 `eea270c` からの S2 全体
- 手順: `.claude/commands/review.md`。読み取り専用（本節の追記以外は変更していない。変異テストは scratchpad へ写したコピーで行った）

### 1. 要約

- 1 往復目で採用とした 10 件は、すべてコードと docs に入っていることを確かめた（高-2 の `git diff --stat` は呼び出し元の指示どおり最終段で貼る前提として扱い、再指摘しない）。手元の go test は 149 PASS / 0 FAIL / 0 SKIP（報告と一致）、再生成は `diff -r` 一致、HEAD の CI（run 37566110344）も成功。新しいテスト 3 本は、守る対象を壊すと赤になることを変異で確かめた。
- **高 2 件**: (高-1) 取り込みで足した `state` が、舞の通常の花蝶扇 8 技に「花蝶扇バウンド中」を付けている（誤った値。英語技名 `[Kachousen bouncing] Light` も同じ原因で 1 往復目から誤り）。(高-2) `docs/data/moves-db.md` と報告は「型が持つキーの欠けも誤り」と書くが、`missingKeys` は `states[]`・`character.name`・`safeJump` の中身・`index.json` を見ない（docs とコードの食い違い）。
- **中 1 件**: ブランカの `blanka_chan_bomb_activated`（2 技）の `state` が、在庫（stock）の `blanka_chan_bomb` になっている（推測: 前提は設置フラグ `blanka_chan_bomb_is_set`）。
- **低 1 件**: 報告の「7 キャラ 79 技」は実データでは 6 キャラ 79 技。

### 2. 1 往復目の採用項目の確認

| 指摘 | 判定 | 確かめたこと |
|---|---|---|
| 高-1 upstream-diff の漏れ | 済 | `docs/upstream-diff.md` に `generate_m2002.go`・`standalone.go`（`UpstreamUnfilled` 含む）の行がある。同ファイルの比較スクリプトを自分で回し、出力（変更あり 2・単体版のみ 1）が表と一致 |
| 高-2 diff --stat | 対象外 | 呼び出し元の指示どおり最終段で追加予定。再指摘しない |
| 高-3 CI コメントと setup-go | 済 | `ci.yml` で「go.mod の有無」が `setup-go` より前に移り、`setup-go`・`go mod tidy` に `if` が付いた。冒頭コメント（無ければスキップして緑）と一致 |
| 中-1 state・必須キー・condition | 一部 | 3 点とも実装・報告済み。ただし `state` の値に誤り（新・高-1、中-1）、必須キーの検査範囲が docs より狭い（新・高-2） |
| 中-2 オラクルの判定 | 済 | `CheckOracle` が `seedgen.UpstreamUnfilled`（本家と同じ `resolveCharacter`・`dropCollisions`・FormatCurrent）の理由 collision / rush-original-unfilled と突き合わせる形になった。区分の合計は numeric 349+4 = 344+9 = 353、SRK 329+9 = 330+8 = 338 で前後一致。変異: `collision` の条件を `true` にすると `TestCheckOracle_RequiresUpstreamToActuallyDrop` が赤（FAIL 1） |
| 低-1 go.mod | 済 | `go mod tidy -diff` EXIT=0。CI に段がある |
| 低-2 最長一致 | 済 | `matchState`。変異: 「最初に当たったもの」に戻すと `TestStateOf`・`TestGenerateNameEN` が赤（FAIL 2） |
| 低-3 ラッシュ版・Lv | 済 | ジェイミーのラッシュ版が `Drive Rush [Drink Level 3] …`、上書きの引継ぎ 2 件（`override-of-original:2`）、SRK `(Lv1)` |
| 低-4 SKIP 件数 | 済 | Summary 段に `skip=` |
| 低-5 親が無いと止まる | 済（記述） | `build.go` のコメントと `docs/data/moves-db.md` に記述 |

### 3. 指摘

#### 高

**高-1 `state`（と英語技名の状態印）が、状態を前提としない技に付いている（舞の通常の花蝶扇 8 技）**
- 場所: `internal/movesdb/name_en.go:148`（`matchState`）、`internal/movesdb/build.go:158, 234`（`stateOf`）、`docs/data/moves-db.md:63`（「この技が前提とするキャラ固有の状態」）。データ: `data/moves/mai.json` の `kachousen_light`・`kachousen_holding_light`・`_medium`・`_holding_medium`・`_heavy`・`_holding_heavy`・`_od`・`_holding_od`
- 内容: 舞の状態 `kachousen` は「花蝶扇バウンド中」（公開版 `000003_data_seed_characters.up.sql` の custom_states。type=level、選択肢は「ホールド弱…乱れ花蝶扇」＝**投げた扇が跳ねている状態**）。通常の花蝶扇（name_ja `弱花蝶扇` 等、numeric `236LP` で状態印なし）は、その状態を**作る**技であって前提にする技ではない。`move_code` が `kachousen_` で始まるので、接頭辞の規則で `state: "kachousen"` が付き、英語技名も `[Kachousen bouncing] Light` になっている。英語技名のほうは 1 往復目の時点（`35e7642` の mai.json 1061 行）から同じ誤りで、1 往復目のレビューも見落としていた。同じ技の 4 表記のうち nameJa・numeric・SRK には状態印が無く、nameEn だけが状態つきという食い違いにもなっている。
- なぜ問題か: 誤った値を出すバグ。報告は `state` を「絞り込み用」と位置づけており、S5 で状態ごとに絞ると、舞の基本の飛び道具が「バウンド中」でしか出ない技として扱われる。nameEn は利用者に見える文字列で、存在しない状態名を冠した技名になる。検査（`CheckFile` は `state` が `states` にあることだけを見る）もテスト（合成の `TestStateOf`）も緑のまま。
- 直し方: 接頭辞だけで決めない。案 (a) `name_ja` の `[..]`・`【..】` のラベルが状態の `name_ja` を含む技だけに付ける（実データで、ryu `[電刃錬気]`・lily `[風纏い]`・blanka `【ライトニングビースト】`・e_honda `[肩屋入り版]`・jamie の酔いレベルは残り、舞の 8 技とブランカの起動 2 技が外れる。jamie の name_ja の形は推測: 未確認なので、適用前に 23 技を確かめる）。案 (b) 状態 code の接頭辞に加え、状態の `type`（stock / level / flag）や上書き表で例外を持つ。どちらでも、舞の `kachousen_light` が `state: null`・nameEn `Light Kachousen` になることを実データのテスト（`TestCommittedMovesDB` 相当か合成）で固定し、規則を接頭辞だけに戻すと赤になることを確かめる。直した後、報告の「付いたのは … 79 技」も数え直す。

**高-2 必須キーの検査の範囲が、docs と報告の書き方より狭い（docs とコードの食い違い）**
- 場所: `internal/movesdb/write.go:143-180`（`missingKeys` が見るのは全体・`character`・各技・`notation` の 4 層だけ。関数コメントはその範囲を正しく書いている）、`docs/data/moves-db.md:96`（「形どおり（未知のキーは読込で誤り、**型が持つキーの欠けも誤り**）」）、`docs/progress/S2-report.md:113`（「必須キー（欠けたキーが黙って既定値にならない）」）
- 内容: `states[].code`・`states[].name.{ja,en}`・`character.name.{ja,en}`・`safeJump.{true,f5,f6}`、`index.json` のどのキーも欠けを検出しない。たとえば `safeJump` から `f5` を消すと、`encoding/json` は黙って 0 にする（`SafeJump.F5` は `int`）。これは検査を足した理由そのもの（§8-7、「null と既定値を取り違えない」）に当たる。コミット済みデータは `fileSha256` と再生成一致で守られているので実害は今は無いが、docs と報告の読み手は「型のどこが欠けても誤り」と受け取る。
- 直し方: `checkKeys` を構造体・ポインタ・スライスの要素へ再帰させ（`reflect` で `Struct` / `Ptr` / `Slice` を辿る）、`index.json` にも当てる。`TestMissingKeys` に `safeJump.f5`・`states[0].name.en`・`character.name.ja` の欠けを足し、再帰を外すと赤になることを確かめる。あるいは docs と報告の文言を「全体・character・各技・notation のキー」に狭める（その場合は `safeJump` の中身が 0 に化けうることを既知の問題に書く）。

#### 中

**中-1 ブランカ `blanka_chan_bomb_activated`・`_od` の `state` が在庫の状態になっている**
- 場所: `data/moves/blanka.json`（2 技）、規則は高-1 と同じ `matchState`
- 内容: 状態 `blanka_chan_bomb` は「ブランカちゃん人形」の **stock（0〜3 の在庫）**、`blanka_chan_bomb_is_set` は「設置」の **flag**。起動（`_activated`）は `blanka_chan_bomb_is_set_` では始まらないので、最長一致でも短いほう（在庫）に当たる。推測: 起動の前提は「設置されていること」で、在庫ではない。name_ja `ブランカちゃん爆弾（起動）` にも状態印は無い。
- なぜ問題か: 高-1 と同じく絞り込みの値が意味とずれる。在庫型の状態を「前提とする状態」とみなすかどうかを、規則として決めていない。
- 直し方: 高-1 の案 (a) なら外れる。前提を表すなら上書き表で `blanka_chan_bomb_is_set` を指定する。どちらにするかを報告の「判断と理由」に書く。

#### 低

**低-1 報告の「7 キャラ 79 技」が実データと合わない**
- 場所: `docs/progress/S2-report.md:155`
- 内容: `state` が付いたのは blanka 23（`lightning_beast` 21・`blanka_chan_bomb` 2）・e_honda 4・jamie 23・lily 13・mai 8・ryu 8 の **6 キャラ** 79 技（Python で `data/moves/*.json` の `state` 非 null を数えた）。ブランカを状態の数で 2 回数えたと推測する。高-1 を直すと数が変わるので、直した後の数で書き直す。

### 4. 良かった点

- オラクルの締め直しが、本家の生成器そのもの（`resolveCharacter`）で回し直して理由を突き合わせる形になり、「被りがある」だけで通す余地が無くなった。区分の付け替え（予約→collision、ラッシュ版の分離）の前後で合計が一致している。
- 新しいテスト 3 本（オラクル・必須キー・最長一致）は、どれも変異で赤になることを確かめられた。
- `docs/upstream-diff.md` に漏れの見つけ方（比較スクリプト）を置き、実際に回すと表と一致する。

### 5. 自分で回したコマンドと結果

```
$ go test -count=1 -v ./... > /tmp/s2r2-go.txt 2>&1; echo EXIT=$?
EXIT=0
grep -cE '^\s*--- PASS' → 149 / '--- FAIL' → 0 / '--- SKIP' → 0
ok  github.com/plexiblinp/tacpendium-setplay/cmd/movesdb
ok  github.com/plexiblinp/tacpendium-setplay/internal/gameversion
ok  github.com/plexiblinp/tacpendium-setplay/internal/moveindex
ok  github.com/plexiblinp/tacpendium-setplay/internal/movesdb
ok  github.com/plexiblinp/tacpendium-setplay/internal/sanumber
ok  github.com/plexiblinp/tacpendium-setplay/internal/seedgen
（TestPipeline_RegeneratesCommittedData・TestCommittedMovesDB・TestRun_MatchesCommittedData・
  TestCheckOracle_RequiresUpstreamToActuallyDrop・TestMissingKeys・TestStateOf がいずれも --- PASS）

$ go vet ./... ; echo EXIT=$?                               → EXIT=0
$ gofmt -l cmd internal                                     → 出力 0 行
$ go mod tidy -diff ; echo EXIT=$?                          → EXIT=0
$ go run ./cmd/movesdb -out /tmp/s2r2-moves > /tmp/s2r2-movesdb.txt 2>&1; echo EXIT=$?
EXIT=0（要約は報告の逐語と同一。オラクル numeric 2635/118/344/9/0/69・srk 2650/99/330/8/0/69、英語技名 {override:7 override-of-original:2 rule:3048}、違反: 0 件）
$ diff -r data/moves /tmp/s2r2-moves ; echo EXIT=$?         → EXIT=0
$ python3 scripts/check-licenses.py ; echo EXIT=$?          → EXIT=0「違反: 0 件」
$ bash scripts/check-actions-pinned.sh ; echo EXIT=$?       → EXIT=0「違反: 0 件」
$ git diff --stat eea270c HEAD -- _reference | wc -l        → 0
$ (docs/upstream-diff.md の比較スクリプト)                   → 変更あり: generate_m2002.go / generate_test.go、単体版のみ: standalone.go
$ gh run list --branch claude/stage-2-08rivo --limit 3      → 37566110344（HEAD 4264d8d）completed success

変異（scratchpad へ写したコピーで実施。リポジトリは変更していない）:
- check.go の `case upstreamUnfilled[m.Code] == "collision":` を `case true:` に
  → go test -run TestCheckOracle_RequiresUpstreamToActuallyDrop: EXIT=1、--- FAIL 1
- write.go の checkKeys で欠けを報告しない形に
  → go test -run TestMissingKeys: EXIT=1、--- FAIL 1
- name_en.go の matchState を最初に当たったものを採る形に
  → go test -run 'TestStateOf|TestGenerateNameEN': EXIT=1、--- FAIL 2
```

`state` の点検（Python で `data/moves/*.json` を走査）: `state` 非 null は 6 キャラ 79 技。状態の `name_ja` が技の `name_ja` に含まれない技は 11 件（mai `kachousen_*` 8・blanka `blanka_chan_bomb_activated*` 2・lily `windclad_od_condor_dive_follow_up` 1）。lily の 1 件は「風纏い中のコンドルダイブの派生」で `state` は妥当と判断した（推測）。舞の状態の意味は公開版 `migrations/000003_data_seed_characters.up.sql` の custom_states で確かめた。

## 取り込み（2 往復目）

| 指摘 | 採否 | 理由・対応 |
|---|---|---|
| 高-1 舞の `kachousen_*` に誤った `state` | 採用 | `state` を「move_code が状態 code で始まり、かつ name_ja の [..]・【..】 に状態の日本語名が含まれる」で決める（`moveStates`）。英語技名・SRK の状態印も同じ判定を使う（`Light Kachousen` に直った）。`TestMoveStates_RequiresBracketLabel` で固定し、name_ja の確認を外すと赤になることを確かめた。付く技は 5 キャラ 68 技に |
| 高-2 必須キーの検査が docs より狭い | 採用 | `missingKeys` を型から再帰で見る形にし（入れ子の構造体・配列の要素・null でないポインタの先）、`index.json` にも当てた。`TestMissingKeys` に `safeJump.f5`・`states[0].name.en`・`moves[0].state`・index の欠けを足し、再帰を外すと赤になることを確かめた |
| 中 ブランカの `blanka_chan_bomb_activated` の state | 採用 | 高-1 の修正で付かなくなった（name_ja「ブランカちゃん爆弾（起動）」に状態の印が無い） |
| 低 「7 キャラ 79 技」 | 採用 | 高-1 の修正後の数（5 キャラ 68 技）に書き直した |

再レビューは 2 往復で打ち切る（/stage の手順）。残った懸念は完了報告の「残課題」に書いた（name_ja に印が無い状態つきの技に `state` が付かないこと）。
