package seedgen

import (
	"path/filepath"
	"testing"

	"github.com/plexiblinp/tacpendium-setplay/internal/moveindex"
)

// M14-03f(第四波)で seedgen 生成する 5 本のマイグレーションに対する golden。
// characters・移動 9 種・is_projectile・chain_cancel_total・frame_cost は手書きの
// 000082 / 000083 / 000087 / 000088 / 000089 で補う。
//
// ★第三波(3 本)との差は表記プリセットのエイリアス 2 本である。M20-02 が
// 「seed 波ごとに再適用する規則」を足したため、波ごとに numeric / srk を起こす
// (character_data/seed-progress.md)。
//
// ★P-34(層 C-3)の -noinput-only マイグレは起こさない。000076 / 000077 は
// 000072 / 000073 が M20-06 より前に生成されたことの追補であり、M20-06 以降に
// 起こす波では層 C-3 が通常実行に含まれる(実測 = 下記 C-noinput 5 件)。

// fourthWaveOrder は第四波の投入順(未 seed 12 キャラの辞書順 + 仮登録 2 体)。
//
// ★c_viper / dhalsim は characters 行が既に在る仮登録である(000014 で投入・移動
// system move 9 種のみ・攻撃技 0)。⇒ 000082(characters 行)の対象からは外れるが、
// moves 以降の段では他の 12 キャラと同じに扱う。
var fourthWaveOrder = []string{
	"aki", "akuma", "alex", "blanka", "cammy", "chun_li", "dee_jay", "e_honda",
	"ed", "elena", "sagat", "yasmine", "c_viper", "dhalsim",
}

// fourthWaveMovementChars は移動系 9 code を投入する 12 キャラ(本波で characters 行が増えた分)。
//
// ★c_viper / dhalsim を混ぜてはならない。両者の移動 9 code は 000025 が、その
// numeric / srk エイリアスは 000072 / 000073 が既に投入している。混ぜると本波の
// down が前の波の投入分まで消す(生成器テスト TestAliases_MovementDownIsScopedToItsOwnChars)。
var fourthWaveMovementChars = []string{
	"aki", "akuma", "alex", "blanka", "cammy", "chun_li", "dee_jay", "e_honda",
	"ed", "elena", "sagat", "yasmine",
}

const (
	fourthWaveMovesNote        = "M14-03f: 第四波 14 キャラ(未 seed 12 + 仮登録 2)の moves + official_ja_move alias + recovery を手入力 CSV 由来で投入する(第四波)。"
	fourthWaveDerivedNote      = "M14-03f: 第四波 14 キャラの is_derived backfill(CSV に載る code のみ UPDATE)"
	fourthWaveMoveCommandsNote = "M14-03f: 第四波 14 キャラの command 索引 move_commands の seed(非派生のみ)"
	fourthWaveNumericAliasNote = "M14-03f: 第四波 14 キャラの表記プリセット numeric のエイリアス(移動系 9 code は新 12 キャラのみ)"
	fourthWaveSRKAliasNote     = "M14-03f: 第四波 14 キャラの表記プリセット srk のエイリアス(移動系 9 code は新 12 キャラのみ)"
)

func readFourthWaveRows(t *testing.T) map[string][]MoveRow {
	t.Helper()
	rowsByChar := make(map[string][]MoveRow, len(fourthWaveOrder))
	next := 0
	for _, code := range fourthWaveOrder {
		rows, err := ReadFile(filepath.Join(repoRoot(t), "character_data", code+".csv"), next)
		if err != nil {
			t.Fatalf("ReadFile %s: %v", code, err)
		}
		rowsByChar[code] = rows
		next += len(rows)
	}
	return rowsByChar
}

// 再生成は TACPENDIUM_UPDATE_GOLDEN=1 go test ./internal/seedgen/ で行う
// (★M33-03。下の CLI は生成に使った引数の記録であり、出力先は migrations/ ではなく
//
//	 internal/seedgen/testdata/ である):
//
//		go run ./cmd/seedgen -chars aki,akuma,alex,blanka,cammy,chun_li,dee_jay,e_honda,ed,elena,sagat,yasmine,c_viper,dhalsim \
//		  -out 000084_seed_moves_fourth_wave -note "<fourthWaveMovesNote>"
func TestGolden_FourthWaveMovesMatchesRegeneration(t *testing.T) {
	res, err := GenerateWithHeader(fourthWaveOrder, readFourthWaveRows(t),
		CustomHeader("000084_seed_moves_fourth_wave", fourthWaveMovesNote))
	if err != nil {
		t.Fatalf("GenerateWithHeader: %v", err)
	}
	assertGolden(t, "000084_seed_moves_fourth_wave", res.UpSQL, res.DownSQL)
}

// 再生成は TACPENDIUM_UPDATE_GOLDEN=1 go test ./internal/seedgen/ で行う
// (★M33-03。下の CLI は生成に使った引数の記録であり、出力先は migrations/ ではなく
//
//	 internal/seedgen/testdata/ である):
//
//		go run ./cmd/seedgen -mode derived-backfill -chars <14 キャラ> \
//		  -out 000085_backfill_moves_is_derived_fourth_wave -note "<fourthWaveDerivedNote>"
func TestGolden_FourthWaveDerivedBackfillMatchesRegeneration(t *testing.T) {
	res, err := GenerateDerivedBackfill(fourthWaveOrder, readFourthWaveRows(t),
		BackfillHeader("000085_backfill_moves_is_derived_fourth_wave", fourthWaveDerivedNote))
	if err != nil {
		t.Fatalf("GenerateDerivedBackfill: %v", err)
	}
	assertGolden(t, "000085_backfill_moves_is_derived_fourth_wave", res.UpSQL, res.DownSQL)
}

// 再生成は TACPENDIUM_UPDATE_GOLDEN=1 go test ./internal/seedgen/ で行う
// (★M33-03。下の CLI は生成に使った引数の記録であり、出力先は migrations/ ではなく
//
//	 internal/seedgen/testdata/ である):
//
//		go run ./cmd/seedgen -mode move-commands -chars <14 キャラ> \
//		  -out 000086_seed_move_commands_fourth_wave -note "<fourthWaveMoveCommandsNote>"
func TestGolden_FourthWaveMoveCommandsMatchesRegeneration(t *testing.T) {
	res, err := GenerateMoveCommands(fourthWaveOrder, readFourthWaveRows(t),
		MoveCommandsHeader("000086_seed_move_commands_fourth_wave", fourthWaveMoveCommandsNote))
	if err != nil {
		t.Fatalf("GenerateMoveCommands: %v", err)
	}
	assertGolden(t, "000086_seed_move_commands_fourth_wave", res.UpSQL, res.DownSQL)
}

// 再生成は TACPENDIUM_UPDATE_GOLDEN=1 go test ./internal/seedgen/ で行う
// (★M33-03。下の CLI は生成に使った引数の記録であり、出力先は migrations/ ではなく
//
//	 internal/seedgen/testdata/ である):
//
//		go run ./cmd/seedgen -mode aliases -preset numeric -chars <14 キャラ> \
//		  -movement-chars <新 12 キャラ> -out 000090_seed_aliases_numeric_fourth_wave \
//		  -note "<fourthWaveNumericAliasNote>"
func TestGolden_FourthWaveNumericAliasesMatchesRegeneration(t *testing.T) {
	// ★M48-08: 000090 は旧規則(派生は対象外・被ったら全部落とす)で生成された。⇒ FormatPreM4808 で凍結する。
	res, err := GenerateAliases(NumericRule, fourthWaveOrder, readFourthWaveRows(t), fourthWaveMovementChars,
		AliasHeader("000090_seed_aliases_numeric_fourth_wave", fourthWaveNumericAliasNote, WithFormat(FormatPreM4808)),
		WithFormat(FormatPreM4808))
	if err != nil {
		t.Fatalf("GenerateAliases(numeric): %v", err)
	}
	assertGolden(t, "000090_seed_aliases_numeric_fourth_wave", res.UpSQL, res.DownSQL)
}

// 再生成は TACPENDIUM_UPDATE_GOLDEN=1 go test ./internal/seedgen/ で行う
// (★M33-03。下の CLI は生成に使った引数の記録であり、出力先は migrations/ ではなく
//
//	 internal/seedgen/testdata/ である):
//
//		go run ./cmd/seedgen -mode aliases -preset srk -chars <14 キャラ> \
//		  -movement-chars <新 12 キャラ> -out 000091_seed_aliases_srk_fourth_wave \
//		  -note "<fourthWaveSRKAliasNote>"
func TestGolden_FourthWaveSRKAliasesMatchesRegeneration(t *testing.T) {
	// ★M48-08: 000091 も旧規則で凍結する(000090 と同じ)。
	res, err := GenerateAliases(SRKRule, fourthWaveOrder, readFourthWaveRows(t), fourthWaveMovementChars,
		AliasHeader("000091_seed_aliases_srk_fourth_wave", fourthWaveSRKAliasNote, WithFormat(FormatPreM4808)),
		WithFormat(FormatPreM4808))
	if err != nil {
		t.Fatalf("GenerateAliases(srk): %v", err)
	}
	assertGolden(t, "000091_seed_aliases_srk_fourth_wave", res.UpSQL, res.DownSQL)
}

// TestFourthWave_ConversionInvariants は変換規則が第四波でも同じ境界で働くことを固定する。
func TestFourthWave_ConversionInvariants(t *testing.T) {
	rowsByChar := readFourthWaveRows(t)
	res, err := GenerateWithHeader(fourthWaveOrder, rowsByChar,
		CustomHeader("000084_seed_moves_fourth_wave", fourthWaveMovesNote))
	if err != nil {
		t.Fatalf("GenerateWithHeader: %v", err)
	}

	// ★移動 9 種は 14 CSV すべてに 1 行も無い。⇒ drop は 0 件である。
	//   drop が 0 であることは「CSV に無い」の証明であり、移動 9 種の投入元が
	//   000083(手書き)だけであること = 二重投入にならないことの根拠になる。
	if len(res.Dropped) != 0 {
		t.Errorf("drop 移動 move = %d, want 0(CSV の system 行は drive_parry のみ)", len(res.Dropped))
	}

	// ★非派生で command が空の行(索引に載らない)。第四波は 6 件ある。
	//   alex/prowler_stance_cancel は dist に対応行が無く候補を作れないため埋めない
	//   (precheck 第7バッチ 追補・2026-08-25)。他 5 件も同バッチで確認済みである。
	//   ⇒ 「既知の失敗」として流さず、顔ぶれを固定する(D-308)。
	wantEmptyCommand := map[string]bool{
		"chun_li/yoso_kick":               true,
		"chun_li/wall_jump":               true,
		"ed/psycho_knuckle":               true,
		"ed/psycho_knuckle_max_holding":   true,
		"ed/psycho_shoot_od":              true,
		"elena/sa2_revival_dance_healing": true,
	}
	// ★語彙外トークンで索引に載らない行。第四波は blanka の 2 件だけである。
	//   command が `raw{...}` 等で moveindex の語彙に無いトークンを含むため正規化できない。
	//   ⇒ numeric / srk の層 A が当たらず、official_ja_move へフォールバックする
	//     (アラートレポートの index-skip 2 件と同じ 2 行)。
	wantUnknownToken := map[string]bool{
		"blanka/blanka_chan_bomb_activated":    true,
		"blanka/blanka_chan_bomb_activated_od": true,
	}
	gotUnknownToken := map[string]bool{}

	gotEmptyCommand := map[string]bool{}
	for _, skipped := range res.Index.Skipped() {
		switch skipped.Reason {
		case moveindex.SkipDerived:
		case moveindex.SkipEmptyCommand:
			gotEmptyCommand[skipped.CharKey+"/"+skipped.MoveCode] = true
		case moveindex.SkipUnknownToken:
			gotUnknownToken[skipped.CharKey+"/"+skipped.MoveCode] = true
		default:
			t.Errorf("索引非搭載の想定外理由: %s/%s reason=%s", skipped.CharKey, skipped.MoveCode, skipped.Reason)
		}
	}
	if len(gotUnknownToken) != len(wantUnknownToken) {
		t.Errorf("unknown-token = %d 件, want %d 件: got=%v", len(gotUnknownToken), len(wantUnknownToken), gotUnknownToken)
	}
	for k := range wantUnknownToken {
		if !gotUnknownToken[k] {
			t.Errorf("unknown-token に %s が無い", k)
		}
	}
	if len(gotEmptyCommand) != len(wantEmptyCommand) {
		t.Errorf("empty-command = %d 件, want %d 件: got=%v", len(gotEmptyCommand), len(wantEmptyCommand), gotEmptyCommand)
	}
	for k := range wantEmptyCommand {
		if !gotEmptyCommand[k] {
			t.Errorf("empty-command に %s が無い", k)
		}
	}
	for k := range gotEmptyCommand {
		if !wantEmptyCommand[k] {
			t.Errorf("empty-command に想定外の %s がある", k)
		}
	}
}

// TestFourthWave_GenerationMeasurements は生成の実測値を固定する。
//
// ★投入行と非投入行を対で固定する(SUPP-001 §5.5 (3))。「投入した件数」だけでは
// 規則を広く当てすぎても検出できない(件数が増えるだけでエラーにならない)。
func TestFourthWave_GenerationMeasurements(t *testing.T) {
	rows := readFourthWaveRows(t)

	total := 0
	for _, code := range fourthWaveOrder {
		total += len(rows[code])
	}
	if total != 1264 {
		t.Errorf("第四波の CSV 行数(= 投入 moves)= %d, want 1264", total)
	}

	derived, err := GenerateDerivedBackfill(fourthWaveOrder, rows,
		BackfillHeader("measure", "measure"))
	if err != nil {
		t.Fatalf("GenerateDerivedBackfill: %v", err)
	}
	// ★M37-04 追補2 で 519 -> 520(鷹嘴連拳が is_derived=true になった分)。
	if got := sumStats(derived.Stats); got != 520 {
		t.Errorf("is_derived backfill = %d, want 520", got)
	}

	cmds, err := GenerateMoveCommands(fourthWaveOrder, rows, MoveCommandsHeader("measure", "measure"))
	if err != nil {
		t.Fatalf("GenerateMoveCommands: %v", err)
	}
	// ★M37-04 追補2 で 737 -> 736(鷹嘴連拳が段階2 の解決索引から外れた分＝DES-004 §2.4)。
	if got := sumStats(cmds.Stats); got != 736 {
		t.Errorf("move_commands = %d, want 736", got)
	}

	for _, tc := range []struct {
		rule                             AliasPresetRule
		layerB, layerA, layerRush        int
		layerNoInput                     int
		derived, derivedNoCommand        int
		noCommand, indexSkip             int
		rushNoOriginal, rushOrigUnfilled int
		collision                        int
	}{
		// ★★【2026-09-13・M37-04 追補2 で更新】chun_li soaring_eagle_punches(鷹嘴連拳)を
		//   elena soaring_raid と同じ「空中限定ターゲットコンボ」の形へ揃えた(is_derived を
		//   false -> true、フレーム 4 列を空に)。⇒ 同行が 1 件ぶん「投入」から「非投入 derived」へ
		//   移り、下の実測値が ±1 ずつ動いた。★soaring_raid / satelite_leap は元から索引に居ない。
		{
			rule:   NumericRule,
			layerB: 248, layerA: 436, layerRush: 216,
			// ★層 C-3 の 5 件(cammy 3 / akuma 2)。M14-03f で固定表へ足した分である。
			//   ⇒ 000076 / 000077 と同型の -noinput-only マイグレは起こさない。
			layerNoInput: 5,
			// ★同分類は第四波に 11 件ある。5 件を投入し、6 件は値が未決のまま残る。
			derived: 267, derivedNoCommand: 6, noCommand: 6, indexSkip: 2,
			rushNoOriginal: 0, rushOrigUnfilled: 26,
			collision: 52,
		},
		{
			rule:   SRKRule,
			layerB: 252, layerA: 440, layerRush: 221,
			layerNoInput: 5,
			derived:      267, derivedNoCommand: 6, noCommand: 6, indexSkip: 2,
			rushNoOriginal: 0, rushOrigUnfilled: 21,
			collision: 44,
		},
	} {
		t.Run(tc.rule.PresetCode, func(t *testing.T) {
			// ★M48-08: 本表は第四波の as-built(旧規則)の実測である。⇒ FormatPreM4808 で測る。
			//   新しい規則の実測は generate_m4808_test.go の TestM4808_GenerationMeasurements。
			res, err := GenerateAliases(tc.rule, fourthWaveOrder, rows, fourthWaveMovementChars,
				AliasHeader("measure", "measure"), WithFormat(FormatPreM4808))
			if err != nil {
				t.Fatalf("GenerateAliases: %v", err)
			}
			byLayer := map[aliasLayer]int{}
			for _, r := range res.Rows {
				byLayer[r.Layer]++
			}
			for _, c := range []struct {
				name string
				got  int
				want int
			}{
				{"層 B", byLayer[LayerB], tc.layerB},
				{"層 A", byLayer[LayerA], tc.layerA},
				{"層 C-rush", byLayer[LayerCRush], tc.layerRush},
				{"層 C-noinput", byLayer[LayerCNoInput], tc.layerNoInput},
			} {
				if c.got != c.want {
					t.Errorf("%s = %d, want %d", c.name, c.got, c.want)
				}
			}
			byReason := map[unfilledReason]int{}
			for _, u := range res.Unfilled {
				byReason[u.Reason]++
			}
			for _, c := range []struct {
				name string
				got  int
				want int
			}{
				{"derived", byReason[ReasonDerived], tc.derived},
				{"derived-no-command", byReason[ReasonDerivedNoCommand], tc.derivedNoCommand},
				{"no-command", byReason[ReasonNoCommand], tc.noCommand},
				{"index-skip", byReason[ReasonIndexSkip], tc.indexSkip},
				{"rush-no-original", byReason[ReasonRushNoOriginal], tc.rushNoOriginal},
				{"rush-original-unfilled", byReason[ReasonRushOriginalUnfilled], tc.rushOrigUnfilled},
				{"collision", byReason[ReasonCollision], tc.collision},
			} {
				if c.got != c.want {
					t.Errorf("非投入 %s = %d, want %d", c.name, c.got, c.want)
				}
			}
		})
	}
}

// sumStats は Result.Stats(キャラ別件数)の合計を返す。
func sumStats(stats map[string]int) int {
	n := 0
	for _, v := range stats {
		n += v
	}
	return n
}
