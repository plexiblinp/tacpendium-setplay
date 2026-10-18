package movesdb

import "testing"

func TestGenerateNameEN(t *testing.T) {
	states := []State{
		{Code: "denjin_charge", Name: LocalizedName{JA: "電刃錬気", EN: "Denjin Charge"}},
		{Code: "drink_level", Name: LocalizedName{JA: "酔いレベル", EN: "Drink Level"}},
		{Code: "order_of_the_sun", Name: LocalizedName{JA: "サンオーダー発動中", EN: "Order of the Sun"}},
		{Code: "order_of_the_sun_strength", Name: LocalizedName{JA: "サンオーダーの強度", EN: "Order of the Sun strength"}},
	}
	cases := []struct{ code, want string }{
		{"hadoken_light", "Light Hadoken"},
		{"hadoken_od", "OD Hadoken"},
		{"aerial_tatsumaki_senpu_kyaku_od", "OD Aerial Tatsumaki Senpu Kyaku"},
		{"crouching_light_punch", "Crouching Light Punch"},
		{"rush_standing_heavy_punch", "Drive Rush Standing Heavy Punch"},
		{"throw_forward", "Forward Throw"},
		{"jump_neutral", "Neutral Jump"},
		{"denjin_charge_od_hadoken", "[Denjin Charge] OD Hadoken"},
		{"drink_level_4_senei_kick", "[Drink Level 4] Senei Kick"},
		{"bitter_strikes_2hits", "Bitter Strikes (2 hits)"},
		{"freeflow_kicks_2hit_light", "Light Freeflow Kicks (2 hits)"},
		{"psycho_flicker_holding_heavy", "Heavy Psycho Flicker (Hold)"},
		{"gou_hadoken_max_holding_light", "Light Gou Hadoken (Max Hold)"},
		{"sa2_the_devil_s_song", "SA2 The Devil's Song"},
		{"sa1_kagerou_no_mai", "SA1 Kagerou no Mai"},
		{"sa2_shin_hashogeki_lv1", "SA2 Shin Hashogeki Lv1"},
		{"the_devil_inside_up2", "The Devil Inside (+2)"},
		{"kasai_thrust_kick_during_od_gorai_axe_kick", "Kasai Thrust Kick During OD Gorai Axe Kick"},
		{"rush_drink_level_3_hermits_elbow", "Drive Rush [Drink Level 3] Hermits Elbow"},
		// 接頭辞の関係にある状態 code は長いほう
		{"order_of_the_sun_strength_kick", "[Order of the Sun strength] Kick"},
		{"order_of_the_sun_kick", "[Order of the Sun] Kick"},
	}
	for _, c := range cases {
		if got := GenerateNameEN(c.code, states); got != c.want {
			t.Errorf("GenerateNameEN(%q) = %q, want %q", c.code, got, c.want)
		}
	}
}

func TestStateOf(t *testing.T) {
	states := []State{{Code: "order_of_the_sun"}, {Code: "order_of_the_sun_strength"}, {Code: "drink_level"}}
	cases := map[string]string{
		"order_of_the_sun_strength_kick":   "order_of_the_sun_strength",
		"order_of_the_sun_kick":            "order_of_the_sun",
		"rush_drink_level_3_hermits_elbow": "drink_level",
		"hermits_elbow":                    "",
	}
	for code, want := range cases {
		got := ""
		if p := stateOf(code, states); p != nil {
			got = *p
		}
		if got != want {
			t.Errorf("stateOf(%q) = %q, want %q", code, got, want)
		}
	}
}
