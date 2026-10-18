package movesdb

import (
	"testing"

	"github.com/plexiblinp/tacpendium-setplay/internal/seedgen"
)

// 合成データで区別の段を確かめる。★実データの形(CSV 行 + DB 行)に揃えてあるが、合成である。
// 実データ全件は data_test.go と pipeline_test.go が見る。

func synthRow(code, category, nameJA, command string, idx int) seedgen.MoveRow {
	return seedgen.MoveRow{CharacterCode: "test", MoveCode: code, Category: category, NameJA: nameJA, Command: command, RowIndex: idx}
}

func runNotation(t *testing.T, kind notationKind, rows []seedgen.MoveRow, isAerial map[string]bool, states []State) map[string]string {
	t.Helper()
	var moves []DBMove
	names := map[string]Notation{}
	stateByMove := map[string][]State{}
	for i, r := range rows {
		moves = append(moves, DBMove{ID: int64(i + 1), Code: r.MoveCode, Category: r.Category, IsAerial: isAerial[r.MoveCode]})
		ms := moveStates(r.MoveCode, r.NameJA, states)
		stateByMove[r.MoveCode] = ms
		names[r.MoveCode] = Notation{NameJA: r.NameJA, NameEN: GenerateNameEN(r.MoveCode, ms)}
	}
	res, err := buildNotation(kind, DBCharacter{Code: "test"}, stateByMove, moves, rows, nil, names, nil)
	if err != nil {
		t.Fatal(err)
	}
	return res.Text
}

func TestNotation_DistinctMarks(t *testing.T) {
	states := []State{{Code: "denjin_charge", Name: LocalizedName{JA: "電刃錬気", EN: "Denjin Charge"}}}
	rows := []seedgen.MoveRow{
		synthRow("hadoken_od", "special", "OD波動拳", "d dr r plus p p", 0),
		synthRow("denjin_charge_od_hadoken", "special", "[電刃錬気]OD波動拳", "d dr r plus p p", 1),
		synthRow("tatsumaki_senpu_kyaku_light", "special", "弱竜巻旋風脚", "d dl l plus k_l", 2),
		synthRow("aerial_tatsumaki_senpu_kyaku_light", "special", "弱空中竜巻旋風脚", "d dl l plus k_l", 3),
		synthRow("solo", "special", "単独", "d dr r plus k_h", 4),
		// is_aerial は空中技の印ではない(ドメイン知識集成 §7-2)。is_aerial=true でも空中印は付かない
		synthRow("burning_kick_light", "special", "弱バーニングキック", "d dr r plus k_l", 5),
		synthRow("flagged_kick_light", "special", "弱フラグキック", "d dr r plus k_l", 6),
		synthRow("high_jump_light_aerial_burning_kick", "special", "【ハイジャンプ】弱空中バーニングキック", "d dr r plus k_l", 7),
	}
	// 空中印は move_code の語から決める。is_aerial はわざと逆に立てる
	isAerial := map[string]bool{"tatsumaki_senpu_kyaku_light": true, "flagged_kick_light": true}

	num := runNotation(t, kindNumeric, rows, isAerial, states)
	wantNum := map[string]string{
		"hadoken_od":                          "236P+P",
		"denjin_charge_od_hadoken":            "[電刃錬気] 236P+P",
		"tatsumaki_senpu_kyaku_light":         "214LK",
		"aerial_tatsumaki_senpu_kyaku_light":  "j.214LK",
		"solo":                                "236HK",
		"burning_kick_light":                  "236LK (弱バーニングキック)",
		"flagged_kick_light":                  "236LK (弱フラグキック)",
		"high_jump_light_aerial_burning_kick": "hj.236LK",
	}
	srk := runNotation(t, kindSRK, rows, isAerial, states)
	wantSRK := map[string]string{
		"hadoken_od":                          "236P+P",
		"denjin_charge_od_hadoken":            "[Denjin Charge] 236P+P",
		"tatsumaki_senpu_kyaku_light":         "214LK",
		"aerial_tatsumaki_senpu_kyaku_light":  "j.214LK",
		"solo":                                "236HK",
		"burning_kick_light":                  "236LK (burning)",
		"flagged_kick_light":                  "236LK (flagged)",
		"high_jump_light_aerial_burning_kick": "hj.236LK",
	}
	for code, want := range wantNum {
		if num[code] != want {
			t.Errorf("numeric %s = %q, want %q", code, num[code], want)
		}
	}
	for code, want := range wantSRK {
		if srk[code] != want {
			t.Errorf("srk %s = %q, want %q", code, srk[code], want)
		}
	}
}

func TestNotation_ODAndNameFallback(t *testing.T) {
	rows := []seedgen.MoveRow{
		// 同じ入力で OD だけが違う組(kimberly emergency_stop と同じ形)
		synthRow("emergency_stop", "special", "急停止", "p", 0),
		synthRow("emergency_stop_od", "special", "OD急停止", "p", 1),
		// ラベルで区別できない組 → 名前を添える
		synthRow("yoga_flame_light", "special", "弱ヨガフレイム", "r dr d dl l plus p_l", 2),
		synthRow("yoga_comet_light", "special", "弱ヨガコメット", "r dr d dl l plus p_l", 3),
		// command が空の技 → 名前フォールバック
		synthRow("no_command", "special", "入力なし", "", 4),
	}
	num := runNotation(t, kindNumeric, rows, nil, nil)
	want := map[string]string{
		"emergency_stop":    "P",
		"emergency_stop_od": "P (OD)",
		"yoga_flame_light":  "63214LP (弱ヨガフレイム)",
		"yoga_comet_light":  "63214LP (弱ヨガコメット)",
		"no_command":        "入力なし",
	}
	for code, w := range want {
		if num[code] != w {
			t.Errorf("numeric %s = %q, want %q", code, num[code], w)
		}
	}
	srk := runNotation(t, kindSRK, rows, nil, nil)
	if srk["yoga_flame_light"] != "63214LP (flame)" || srk["no_command"] != "No Command" {
		t.Errorf("srk = %q / %q", srk["yoga_flame_light"], srk["no_command"])
	}
}

func TestNotation_Override(t *testing.T) {
	rows := []seedgen.MoveRow{synthRow("hadoken_light", "special", "弱波動拳", "d dr r plus p_l", 0)}
	moves := []DBMove{{ID: 1, Code: "hadoken_light", Category: "special"}}
	names := map[string]Notation{"hadoken_light": {NameJA: "弱波動拳", NameEN: "Light Hadoken"}}
	ov := map[string]NotationOverride{"hadoken_light": {Numeric: "X", Reason: "test"}}
	res, err := buildNotation(kindNumeric, DBCharacter{Code: "test"}, nil, moves, rows, nil, names, ov)
	if err != nil {
		t.Fatal(err)
	}
	if res.Text["hadoken_light"] != "X" || res.Source["hadoken_light"] != SourceOverride {
		t.Errorf("override not applied: %q %q", res.Text["hadoken_light"], res.Source["hadoken_light"])
	}
	// SRK は空なので上書きしない
	res, err = buildNotation(kindSRK, DBCharacter{Code: "test"}, nil, moves, rows, nil, names, ov)
	if err != nil {
		t.Fatal(err)
	}
	if res.Text["hadoken_light"] != "236LP" {
		t.Errorf("srk = %q", res.Text["hadoken_light"])
	}
}

// 状態は move_code の接頭辞だけでは決めない。name_ja の括弧ラベルに状態の名前があるときだけ
// (舞の kachousen_* は状態「花蝶扇バウンド中」を作る技で、前提ではない)。
func TestMoveStates_RequiresBracketLabel(t *testing.T) {
	states := []State{
		{Code: "kachousen", Name: LocalizedName{JA: "花蝶扇バウンド中", EN: "Kachousen bouncing"}},
		{Code: "drink_level", Name: LocalizedName{JA: "酔いレベル", EN: "Drink Level"}},
		{Code: "sumo_spirit", Name: LocalizedName{JA: "肩屋入り", EN: "Sumo Spirit"}},
	}
	cases := []struct{ code, nameJA, want string }{
		{"kachousen_light", "弱花蝶扇", ""},
		{"drink_level_4_senei_kick", "[酔いレベル4]旋影脚", "drink_level"},
		{"sumo_spirit_light_hundred_hand_slap", "[肩屋入り版]弱百裂張り手", "sumo_spirit"},
		{"rush_drink_level_3_hermits_elbow", "[酔いレベル3]仙姑肘(ラッシュ)", "drink_level"},
	}
	for _, c := range cases {
		got := ""
		if p := stateOf(c.code, moveStates(c.code, c.nameJA, states)); p != nil {
			got = *p
		}
		if got != c.want {
			t.Errorf("state(%s) = %q, want %q", c.code, got, c.want)
		}
	}
	if n := GenerateNameEN("kachousen_light", moveStates("kachousen_light", "弱花蝶扇", states)); n != "Light Kachousen" {
		t.Errorf("nameEn = %q", n)
	}
}
