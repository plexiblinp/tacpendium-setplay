package seedgen

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/plexiblinp/tacpendium-setplay/internal/moveindex"
)

// M14-03e(第三波)で seedgen 生成する 3 本のマイグレーションに対する golden。
// characters・移動 9 種・is_projectile は手書きの 000053/000054/000058 で補う。
var thirdWaveOrder = []string{"m_bison", "rashid", "jamie", "luke", "marisa", "jp"}

const (
	thirdWaveMovesNote        = "M14-03e: m_bison / rashid / jamie / luke / marisa / jp の moves + official_ja_move alias + recovery を手入力 CSV 由来で投入する(第三波)。"
	thirdWaveDerivedNote      = "M14-03e: 第三波 6 キャラの is_derived backfill(CSV に載る code のみ UPDATE)"
	thirdWaveMoveCommandsNote = "M14-03e: 第三波 6 キャラの command 索引 move_commands の seed(非派生のみ)"
)

func readThirdWaveRows(t *testing.T) map[string][]MoveRow {
	t.Helper()
	rowsByChar := make(map[string][]MoveRow, len(thirdWaveOrder))
	next := 0
	for _, code := range thirdWaveOrder {
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
//		go run ./cmd/seedgen -chars m_bison,rashid,jamie,luke,marisa,jp -out 000055_seed_moves_third_wave -note "<thirdWaveMovesNote>"
func TestGolden_ThirdWaveMovesMatchesRegeneration(t *testing.T) {
	// ★旧形式を指定する(format.go の FormatPreM2003)。000055 は M20-03 より前に
	//   生成・適用済みであり、character_id を含まない。
	res, err := GenerateWithHeader(thirdWaveOrder, readThirdWaveRows(t),
		CustomHeader("000055_seed_moves_third_wave", thirdWaveMovesNote), WithFormat(FormatPreM2003))
	if err != nil {
		t.Fatalf("GenerateWithHeader: %v", err)
	}
	assertGolden(t, "000055_seed_moves_third_wave", res.UpSQL, res.DownSQL)
}

// 再生成は TACPENDIUM_UPDATE_GOLDEN=1 go test ./internal/seedgen/ で行う
// (★M33-03。下の CLI は生成に使った引数の記録であり、出力先は migrations/ ではなく
//
//	 internal/seedgen/testdata/ である):
//
//		go run ./cmd/seedgen -mode derived-backfill -chars m_bison,rashid,jamie,luke,marisa,jp -out 000056_backfill_moves_is_derived_third_wave -note "<thirdWaveDerivedNote>"
func TestGolden_ThirdWaveDerivedBackfillMatchesRegeneration(t *testing.T) {
	res, err := GenerateDerivedBackfill(thirdWaveOrder, readThirdWaveRows(t),
		BackfillHeader("000056_backfill_moves_is_derived_third_wave", thirdWaveDerivedNote))
	if err != nil {
		t.Fatalf("GenerateDerivedBackfill: %v", err)
	}
	assertGolden(t, "000056_backfill_moves_is_derived_third_wave", res.UpSQL, res.DownSQL)
}

// 再生成は TACPENDIUM_UPDATE_GOLDEN=1 go test ./internal/seedgen/ で行う
// (★M33-03。下の CLI は生成に使った引数の記録であり、出力先は migrations/ ではなく
//
//	 internal/seedgen/testdata/ である):
//
//		go run ./cmd/seedgen -mode move-commands -chars m_bison,rashid,jamie,luke,marisa,jp -out 000057_seed_move_commands_third_wave -note "<thirdWaveMoveCommandsNote>"
func TestGolden_ThirdWaveMoveCommandsMatchesRegeneration(t *testing.T) {
	res, err := GenerateMoveCommands(thirdWaveOrder, readThirdWaveRows(t),
		MoveCommandsHeader("000057_seed_move_commands_third_wave", thirdWaveMoveCommandsNote))
	if err != nil {
		t.Fatalf("GenerateMoveCommands: %v", err)
	}
	assertGolden(t, "000057_seed_move_commands_third_wave", res.UpSQL, res.DownSQL)
}

func TestThirdWave_ConversionInvariants(t *testing.T) {
	rowsByChar := readThirdWaveRows(t)
	res, err := GenerateWithHeader(thirdWaveOrder, rowsByChar,
		CustomHeader("000055_seed_moves_third_wave", thirdWaveMovesNote))
	if err != nil {
		t.Fatalf("GenerateWithHeader: %v", err)
	}
	if len(res.Dropped) != 0 {
		t.Errorf("drop 移動 move = %d, want 0(CSV の system 行は drive_parry のみ)", len(res.Dropped))
	}

	// ★第三波に非派生で command が空の行はもう無い(M20-02 / 2026-08-13)。
	// 旧主張は {jp/triglav_od, jp/departure_od} の 2 件だった——強度 3 分割を 1 行へ
	// 統合した OD 技で、コマンドの一次源がゲーム内表示にしか無く repo のどこにも
	// 無かったため空欄で出荷されていた。M20-02(D-318)で開発者から確定値を受領し
	// CSV へ補記したため、2 件とも索引に載るようになった(triglav_od=22P+P /
	// departure_od=214P+P)。⇒ 期待値を「空 map」へ更新するのが正しい追随である
	// (「既知の失敗」として流さない = D-308)。
	emptyCommand := map[string]bool{}
	for _, skipped := range res.Index.Skipped() {
		switch skipped.Reason {
		case moveindex.SkipDerived:
		case moveindex.SkipEmptyCommand:
			emptyCommand[skipped.CharKey+"/"+skipped.MoveCode] = true
		default:
			t.Errorf("索引非搭載の想定外理由: %s/%s reason=%s", skipped.CharKey, skipped.MoveCode, skipped.Reason)
		}
	}
	if len(emptyCommand) != 0 {
		t.Fatalf("empty-command = %v, want {}(M20-02 で OD 技 4 件を補記済み)", emptyCommand)
	}

	// importer の公式英語名誤りは手入力 CSV に持ち込まれていない。
	lukeCodes := map[string]bool{}
	for _, row := range rowsByChar["luke"] {
		lukeCodes[row.MoveCode] = true
	}
	if !lukeCodes["no_chaser_od"] || lukeCodes["chaser_od"] {
		t.Errorf("luke ODノーチェイサー code: no_chaser_od=%v chaser_od=%v", lukeCodes["no_chaser_od"], lukeCodes["chaser_od"])
	}
}

func TestThirdWave_MoveCodeQualityMeasurements(t *testing.T) {
	want := map[string]struct {
		rows        int
		longestCode string
		longest     int
		over40      int
	}{
		"m_bison": {79, "mine_set_medium_psycho_crusher_attack", 37, 0},
		"rashid":  {99, "whirlwind_shot_max_holding_medium", 33, 0},
		"jamie":   {127, "drink_level_4_ransui_haze_3_drink_while_retreating", 50, 7},
		"luke":    {83, "flash_knuckle_holding_medium", 28, 0},
		"marisa":  {108, "sa1_javelin_of_marisa_counterattack", 35, 0},
		"jp":      {75, "departure_window_double_warp_od", 31, 0},
	}

	for code, rows := range readThirdWaveRows(t) {
		gotLongest, gotLongestCode, gotOver40 := 0, "", 0
		for _, row := range rows {
			if n := len(row.MoveCode); n > gotLongest {
				gotLongest, gotLongestCode = n, row.MoveCode
			}
			if len(row.MoveCode) > 40 {
				gotOver40++
			}
			if strings.HasPrefix(row.NameJA, "CA ") && !strings.HasPrefix(row.MoveCode, "ca_") {
				t.Errorf("%s/%s: CA 接頭語不整合", code, row.MoveCode)
			}
			for _, sa := range []string{"1", "2", "3"} {
				if strings.HasPrefix(row.NameJA, "SA"+sa+" ") && !strings.HasPrefix(row.MoveCode, "sa"+sa+"_") {
					t.Errorf("%s/%s: SA%s 接頭語不整合", code, row.MoveCode, sa)
				}
			}
		}
		w := want[code]
		if len(rows) != w.rows || gotLongestCode != w.longestCode || gotLongest != w.longest || gotOver40 != w.over40 {
			t.Errorf("%s quality = rows:%d longest:%s(%d) over40:%d, want rows:%d longest:%s(%d) over40:%d",
				code, len(rows), gotLongestCode, gotLongest, gotOver40, w.rows, w.longestCode, w.longest, w.over40)
		}
	}
}
