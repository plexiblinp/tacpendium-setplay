package seedgen

import (
	"strings"
	"testing"
)

// M20-02 生成器の単体テスト。規則そのもの(層順・SA 注記・衝突除外・rush 合成・冪等性)を固定する。
//
// ★golden(実マイグレとの byte-identical)は generate_m2002_golden_test.go が持つ。
// 本ファイルは「規則が何を返すか」を小さな入力で固定する。

// mkRow は最小限のフィールドだけを埋めた MoveRow を作る。
func mkRow(char, code, category, nameJA, command string, derived bool) MoveRow {
	return MoveRow{
		CharacterCode: char, MoveCode: code, Category: category,
		NameJA: nameJA, Command: command, IsDerived: derived,
		StartupBasis: "", // validate は空文字を許す(validStartupBases に "" が入っている)
	}
}

// mkRush は rush_variant 行を作る。
func mkRush(char, code, nameJA, originalCode string) MoveRow {
	r := mkRow(char, code, "rush_variant", nameJA, "", true)
	r.OriginalMoveCode = originalCode
	return r
}

// genOne は 1 キャラ分を生成して結果を返す(opts は GenerateAliases へそのまま渡す)。
func genOne(t *testing.T, rule AliasPresetRule, char string, rows []MoveRow, opts ...Option) *AliasResult {
	t.Helper()
	res, err := GenerateAliases(rule, []string{char}, map[string][]MoveRow{char: rows},
		[]string{char}, AliasHeader("test_stem", "unit test", opts...), opts...)
	if err != nil {
		t.Fatalf("GenerateAliases: %v", err)
	}
	return res
}

// aliasOf は生成結果から当該 move の表記を引く。
func aliasOf(res *AliasResult, moveCode string) (AliasRow, bool) {
	for _, r := range res.Rows {
		if r.MoveCode == moveCode {
			return r, true
		}
	}
	return AliasRow{}, false
}

// unfilledOf は生成結果から当該 move の不投入理由を引く。
func unfilledOf(res *AliasResult, moveCode string) (UnfilledRow, bool) {
	for _, u := range res.Unfilled {
		if u.MoveCode == moveCode {
			return u, true
		}
	}
	return UnfilledRow{}, false
}

// TestAliases_DES004SampleTable は DES-004 §3.4 サンプル表を固定点として検査する。
//
// ★固定点は 4 行 x 2 プリセット = 8 件である(★設計卓裁定 D-4。7 行 14 件ではない)。
// 残る 3 行は原理的に生成できない:
//   - `sa1` は実在する move_code ではない(ryu の実体は sa1_shinku_hadoken)。さらに
//     numeric 期待値 236236HP に対し CSV の command は強度指定なしの p であり 236236P になる。
//     HP はデータから作れない。
//   - `parry_drive_rush` / `cancel_drive_rush` は modifiers.type の非技ステップであり
//     moves 行ではないため preset_aliases(move_id が鍵)に入らない(指示書 §4.4 が自認)。
//
// なお DES-004 §3.4 自身が「※上記は方針を示すためのサンプルであり、実装時の確定表記ではない」
// と明記している。
func TestAliases_DES004SampleTable(t *testing.T) {
	rows := []MoveRow{
		mkRow("ryu", "standing_light_punch", "normal", "立ち弱P", "p_l", false),
		mkRow("ryu", "crouching_light_kick", "normal", "しゃがみ弱K", "k_l", false),
		mkRow("ryu", "hadoken_light", "special", "弱波動拳", "d dr r plus p_l", false),
		mkRow("ryu", "shoryuken_heavy", "special", "強昇龍拳", "r d dr plus p_h", false),
	}

	for _, tc := range []struct {
		rule AliasPresetRule
		want map[string]string
	}{
		{NumericRule, map[string]string{
			"standing_light_punch": "5LP",
			"crouching_light_kick": "2LK",
			"hadoken_light":        "236LP",
			"shoryuken_heavy":      "623HP",
		}},
		{SRKRule, map[string]string{
			"standing_light_punch": "st.LP",
			"crouching_light_kick": "cr.LK",
			"hadoken_light":        "236LP",
			"shoryuken_heavy":      "623HP",
		}},
	} {
		t.Run(tc.rule.PresetCode, func(t *testing.T) {
			res := genOne(t, tc.rule, "ryu", rows)
			for code, want := range tc.want {
				got, ok := aliasOf(res, code)
				if !ok {
					t.Errorf("%s の表記が生成されていない", code)
					continue
				}
				if got.AliasText != want {
					t.Errorf("%s = %q, want %q", code, got.AliasText, want)
				}
			}
		})
	}
}

// TestAliases_LayerOrder は層順の原理(★D-1)を固定する。
//
// standing_light_punch と jumping_light_punch は CSV の command がどちらも "p_l" であり、
// 層 A(token_key)を先に当てるとどちらも "LP" になって衝突する。
// move_code の構造が表記を決める行なので層 B が先に当たること、
// その結果 2 つが別表記になって衝突しないことを固定する。
func TestAliases_LayerOrder_StandingVsJumping(t *testing.T) {
	rows := []MoveRow{
		mkRow("ryu", "standing_light_punch", "normal", "立ち弱P", "p_l", false),
		mkRow("ryu", "jumping_light_punch", "normal", "ジャンプ弱P", "p_l", false),
	}
	res := genOne(t, NumericRule, "ryu", rows)

	st, ok1 := aliasOf(res, "standing_light_punch")
	jp, ok2 := aliasOf(res, "jumping_light_punch")
	if !ok1 || !ok2 {
		t.Fatalf("両方が生成されていない: standing=%v jumping=%v", ok1, ok2)
	}
	if st.Layer != LayerB || jp.Layer != LayerB {
		t.Errorf("層 = standing:%s jumping:%s, want 両方 %s(層 A を先に当ててはならない)", st.Layer, jp.Layer, LayerB)
	}
	if st.AliasText != "5LP" || jp.AliasText != "j.LP" {
		t.Errorf("表記 = standing:%q jumping:%q, want 5LP / j.LP", st.AliasText, jp.AliasText)
	}
	if st.AliasText == jp.AliasText {
		t.Errorf("立ち技と J 攻撃が同表記になった(%q)。層 A を先に当てた形である", st.AliasText)
	}

	// ★捨てた層 A の値が記録されていること(D-2 (b)「黙って捨てない」)。
	if len(res.DiscardedLayerA) != 2 {
		t.Errorf("捨てた層 A の記録 = %d 件, want 2", len(res.DiscardedLayerA))
	}
	for _, d := range res.DiscardedLayerA {
		if d.LayerAKey != "LP" {
			t.Errorf("%s の層 A 値 = %q, want LP", d.MoveCode, d.LayerAKey)
		}
	}
}

// TestAliases_LayerB_IncompleteFormFallsThrough は層 B が接頭辞だけで判定しないことを固定する。
//
// 実データには接頭辞を持つが強度・ボタン語彙に当たらない行が実在する
// (terry/jumping_knee = target_combo / zangief/standing_light_punch_rapid = normal)。
// 接頭辞だけで判定すると、それらに誤った表記が付く。
func TestAliases_LayerB_IncompleteFormFallsThrough(t *testing.T) {
	rows := []MoveRow{
		// 接頭辞あり・強度/ボタン語彙外 → 層 B 不成立。command があるので層 A が拾う。
		mkRow("terry", "jumping_knee", "unique", "ジャンプ膝", "u plus k_m", false),
		// 接頭辞あり・パーツ数が 3 → 層 B 不成立。command が無く派生 → 埋めない。
		mkRow("zangief", "standing_light_punch_rapid", "normal", "立ち弱P連打", "", true),
	}
	// ★terry/jumping_knee は M43-05 の除外表に在る(名前に jumping を含むが地上の技＝開発者決定)ため、
	//   今の規則でも `j.` は付かない。
	res := genOne(t, NumericRule, "terry", rows[:1])
	got, ok := aliasOf(res, "jumping_knee")
	if !ok {
		t.Fatalf("jumping_knee が生成されていない")
	}
	if got.Layer != LayerA {
		t.Errorf("jumping_knee の層 = %s, want %s(層 B は完全形にのみ当てる)", got.Layer, LayerA)
	}
	if got.AliasText != "8MK" {
		t.Errorf("jumping_knee = %q, want 8MK", got.AliasText)
	}

	res2 := genOne(t, NumericRule, "zangief", rows[1:])
	if _, ok := aliasOf(res2, "standing_light_punch_rapid"); ok {
		t.Errorf("standing_light_punch_rapid に表記が付いた。層 B は完全形にのみ当てるべきで、"+
			"付けると元技 standing_light_punch と同表記になり衝突する。res=%v", res2.Rows)
	}
}

// TestAliases_DoNotGuess は「埋めずに返す」ことを固定する(★推測で埋めない)。
//
// ★M48-08 で U-2 を撤回したため、派生技の行(1 件目)が ReasonDerived になるのは旧規則
// (FormatPreM4808)だけである。⇒ 本テストは旧規則を凍結した形で回す。新しい規則で派生技が
// 層 A の対象になることは generate_m4808_test.go の TestM4808_DerivedMoveGetsLayerA が固定する。
func TestAliases_DoNotGuess(t *testing.T) {
	rows := []MoveRow{
		// 派生技は層 A の対象外(U-2。旧規則)。
		// ★★【2026-09-12 注記・M30-07】この code は seed から消えた(guile の【ジャスト】版は
		//   接尾形 `sonic_boom_perfect_light` へ揃った)。**本ファイルは合成 fixture であり、
		//   実在しない code を入力に使うこと自体が主張の趣旨である**(同じ配列の
		//   `mystery_move` / `sonic_cross_conditional` も実在しない)。⇒ 意図して触っていない。
		//   ★実在する code と読み違えないこと。
		mkRow("guile", "perfect_timing_light_sonic_boom", "special", "【ジャスト】弱ソニックブーム",
			"charge_l r plus p_l", true),
		// command 空欄・非派生 → 埋めない。
		mkRow("guile", "mystery_move", "special", "謎技", "", false),
		// cond{ を含む → 正規化できない → 埋めない。
		mkRow("guile", "sonic_cross_conditional", "special", "条件付き", "r plus p or cond{（条件）} r plus p p", false),
	}
	res := genOne(t, NumericRule, "guile", rows, WithFormat(FormatPreM4808))

	for _, c := range []struct {
		code string
		want unfilledReason
	}{
		{"perfect_timing_light_sonic_boom", ReasonDerived},
		{"mystery_move", ReasonNoCommand},
		{"sonic_cross_conditional", ReasonIndexSkip},
	} {
		if _, ok := aliasOf(res, c.code); ok {
			t.Errorf("%s に表記が付いた(推測で埋めてはならない)", c.code)
		}
		u, ok := unfilledOf(res, c.code)
		if !ok {
			t.Errorf("%s が Unfilled に記録されていない(穴は全件報告する)", c.code)
			continue
		}
		if u.Reason != c.want {
			t.Errorf("%s の理由 = %s, want %s", c.code, u.Reason, c.want)
		}
	}
}

// TestAliases_SAAnnotation は SA 注記(★D-5)を固定する。
//
// 書式は「半角スペース 1 個 + 半角丸括弧」。値は move_code から機械抽出し、技名から推測しない。
// 適用範囲は super_art / critical_art 一律である。
func TestAliases_SAAnnotation(t *testing.T) {
	rows := []MoveRow{
		mkRow("ryu", "sa1_shinku_hadoken", "super_art", "真空波動拳", "d dr r d dr r plus p", false),
		mkRow("ryu", "sa3_shin_shoryuken", "super_art", "真・昇龍拳", "d dr r d dr r plus k", false),
		mkRow("ryu", "ca_shin_shoryuken", "critical_art", "CA 真・昇龍拳", "d dr r d dr r plus k_h", false),
		// ★接頭辞を持つ形(denjin_charge_sa2_…)も拾えること。
		mkRow("ryu", "denjin_charge_sa2_shin_hashogeki_lv1", "super_art", "電刃 真・波掌撃", "d dl l d dl l plus p_l", false),
		// ★super_art / critical_art 以外には付けないこと。
		mkRow("ryu", "hadoken_light", "special", "弱波動拳", "d dr r plus p_l", false),
	}
	res := genOne(t, NumericRule, "ryu", rows)

	for _, c := range []struct{ code, want string }{
		{"sa1_shinku_hadoken", "236236P (SA1)"},
		{"sa3_shin_shoryuken", "236236K (SA3)"},
		{"ca_shin_shoryuken", "236236HK (CA)"},
		{"denjin_charge_sa2_shin_hashogeki_lv1", "214214LP (SA2)"},
		{"hadoken_light", "236LP"},
	} {
		got, ok := aliasOf(res, c.code)
		if !ok {
			t.Errorf("%s が生成されていない", c.code)
			continue
		}
		if got.AliasText != c.want {
			t.Errorf("%s = %q, want %q", c.code, got.AliasText, c.want)
		}
	}
}

// TestAliases_SAAnnotationResolvesSA3vsCA は注記が SA3 vs CA の衝突を解くことを固定する。
//
// 実データでは SA3 と CA が同一コマンドである組が全 17 キャラに存在する。
// 注記が無いと同表記になって両方が投入対象外になる(§9.3-5)。
func TestAliases_SAAnnotationResolvesSA3vsCA(t *testing.T) {
	rows := []MoveRow{
		mkRow("ryu", "sa3_shin_shoryuken", "super_art", "真・昇龍拳", "d dr r d dr r plus k", false),
		mkRow("ryu", "ca_shin_shoryuken", "critical_art", "CA 真・昇龍拳", "d dr r d dr r plus k", false),
	}
	res := genOne(t, NumericRule, "ryu", rows)
	if len(res.Rows) != 2 {
		t.Fatalf("投入行 = %d, want 2(注記が衝突を解くはず)。Unfilled=%v", len(res.Rows), res.Unfilled)
	}
	sa, _ := aliasOf(res, "sa3_shin_shoryuken")
	ca, _ := aliasOf(res, "ca_shin_shoryuken")
	if sa.AliasText == ca.AliasText {
		t.Errorf("SA3 と CA が同表記のまま(%q)", sa.AliasText)
	}
}

// TestAliases_CollisionDropsBothSides は旧規則で衝突した組を両方落とすことを固定する(§9.3-5)。
//
// ★★M48-08(開発者決定 D-1037)で規則は「優先の 1 つだけ残す」へ変わった
// (generate_m4808_test.go の TestM4808_CollisionKeepsPrior)。⇒ 本テストは旧規則を凍結した
// FormatPreM4808 で回し、凍結 golden が再現する挙動を守る。
func TestAliases_CollisionDropsBothSides(t *testing.T) {
	rows := []MoveRow{
		mkRow("zangief", "crouching_heavy_punch", "normal", "しゃがみ強P", "d plus p_h", false),
		// 空中版。numeric では層 B の 2HP と層 A の 2HP がぶつかる。
		mkRow("zangief", "flying_body_press", "unique", "フライングボディプレス", "d plus p_h", false),
	}
	res := genOne(t, NumericRule, "zangief", rows, WithFormat(FormatPreM4808))
	if len(res.Rows) != 0 {
		t.Errorf("投入行 = %d, want 0(衝突した組は両方落とす)。got=%v", len(res.Rows), res.Rows)
	}
	for _, code := range []string{"crouching_heavy_punch", "flying_body_press"} {
		u, ok := unfilledOf(res, code)
		if !ok || u.Reason != ReasonCollision {
			t.Errorf("%s が collision として記録されていない(got ok=%v reason=%s)", code, ok, u.Reason)
		}
		if !strings.Contains(u.Detail, "2HP") {
			t.Errorf("%s の補足に衝突した表記が入っていない: %q", code, u.Detail)
		}
	}

	// ★srk では cr. 接頭辞のおかげで衝突しない(地上/空中の曖昧が偶然解ける)。
	resSRK := genOne(t, SRKRule, "zangief", rows, WithFormat(FormatPreM4808))
	if len(resSRK.Rows) != 2 {
		t.Errorf("srk の投入行 = %d, want 2(cr.HP と 2HP で衝突しない)。Unfilled=%v",
			len(resSRK.Rows), resSRK.Unfilled)
	}
}

// TestAliases_RushComposition は rush_variant の合成(D-311)を固定する。
func TestAliases_RushComposition(t *testing.T) {
	rows := []MoveRow{
		mkRow("guile", "crouching_light_punch", "normal", "しゃがみ弱P", "d plus p_l", false),
		mkRush("guile", "rush_crouching_light_punch", "しゃがみ弱P(ラッシュ)", "crouching_light_punch"),
		// ★元技 code が実在しない場合は当てない(D-305)。
		// ★かつて実データに在った 4 行(ingrid 2 / lily 1 / mai 1)と同型である。
		//   ⇒ 実データ側は M35-03(2026-09-11)で 0 件になったが、本フィクスチャは残す——
		//     規則そのものを固定するものであり、実データの有無に依らない。
		mkRush("guile", "rush_ghost", "幽霊(ラッシュ)", "no_such_move"),
	}
	res := genOne(t, NumericRule, "guile", rows)

	got, ok := aliasOf(res, "rush_crouching_light_punch")
	if !ok {
		t.Fatalf("rush_crouching_light_punch が生成されていない。Unfilled=%v", res.Unfilled)
	}
	if got.AliasText != "DR > 2LP" {
		t.Errorf("rush 合成 = %q, want %q", got.AliasText, "DR > 2LP")
	}
	if got.Layer != LayerCRush {
		t.Errorf("rush の層 = %s, want %s", got.Layer, LayerCRush)
	}

	if _, ok := aliasOf(res, "rush_ghost"); ok {
		t.Errorf("元技が実在しない rush に表記が付いた(推測で元技を当ててはならない)")
	}
	if u, ok := unfilledOf(res, "rush_ghost"); !ok || u.Reason != ReasonRushNoOriginal {
		t.Errorf("rush_ghost が rush-no-original として記録されていない(ok=%v reason=%s)", ok, u.Reason)
	}
}

// TestAliases_Movement は移動系 9 code(§4.6)を固定する。
func TestAliases_Movement(t *testing.T) {
	for _, tc := range []struct {
		rule AliasPresetRule
		want map[string]string
	}{
		{NumericRule, map[string]string{
			"forward": "6", "back": "4", "dash_forward": "66", "dash_back": "44",
			"jump_neutral": "8", "jump_forward": "9", "jump_back": "7",
			"micro_forward": "微歩き", "micro_back": "微下がり",
		}},
		{SRKRule, map[string]string{
			"forward": "f", "back": "b", "dash_forward": "dash", "dash_back": "backdash",
			"jump_neutral": "nj", "jump_forward": "fj", "jump_back": "bj",
			"micro_forward": "微歩き", "micro_back": "微下がり",
		}},
	} {
		t.Run(tc.rule.PresetCode, func(t *testing.T) {
			res := genOne(t, tc.rule, "ryu", []MoveRow{
				mkRow("ryu", "hadoken_light", "special", "弱波動拳", "d dr r plus p_l", false),
			})
			if len(res.Movement) != 9 {
				t.Fatalf("移動系 = %d code, want 9", len(res.Movement))
			}
			byCode := map[string]AliasRow{}
			for _, m := range res.Movement {
				byCode[m.MoveCode] = m
			}
			for code, want := range tc.want {
				if byCode[code].AliasText != want {
					t.Errorf("%s = %q, want %q", code, byCode[code].AliasText, want)
				}
			}
			// ★移動系 9 code のうち alias_text_en を持つのは micro_forward / micro_back の
			//   2 code だけ(D-321)。★本主張は res.Movement の中の話である——キャラ別の行では
			//   層 C-3(P-34 の 13 code)も英語表記を持つ(M20-06 / D-373)。
			var withEn []string
			for _, m := range res.Movement {
				if m.AliasTextEn != "" {
					withEn = append(withEn, m.MoveCode)
				}
			}
			if len(withEn) != 2 {
				t.Errorf("alias_text_en を持つ code = %v, want [micro_forward micro_back] の 2 件", withEn)
			}
			if byCode["micro_forward"].AliasTextEn != "microwalk" ||
				byCode["micro_back"].AliasTextEn != "back microwalk" {
				t.Errorf("alias_text_en = micro_forward:%q micro_back:%q, want microwalk / back microwalk",
					byCode["micro_forward"].AliasTextEn, byCode["micro_back"].AliasTextEn)
			}
		})
	}
}

// TestAliases_Idempotent は生成器の冪等性(同じ入力から同じ出力)を固定する(§4.9)。
func TestAliases_Idempotent(t *testing.T) {
	rows := []MoveRow{
		mkRow("guile", "standing_light_punch", "normal", "立ち弱P", "p_l", false),
		mkRow("guile", "crouching_medium_kick", "normal", "しゃがみ中K", "d plus k_m", false),
		mkRow("guile", "sonic_boom_light", "special", "弱ソニックブーム", "charge_l r plus p_l", false),
		mkRow("guile", "sa2_solid_puncher", "super_art", "ソリッドパンチャー", "d dl l d dl l plus p", false),
		mkRush("guile", "rush_standing_light_punch", "立ち弱P(ラッシュ)", "standing_light_punch"),
	}
	for _, rule := range []AliasPresetRule{NumericRule, SRKRule} {
		t.Run(rule.PresetCode, func(t *testing.T) {
			a := genOne(t, rule, "guile", rows)
			b := genOne(t, rule, "guile", rows)
			if a.UpSQL != b.UpSQL {
				t.Errorf("up SQL が 2 回の生成で一致しない")
			}
			if a.DownSQL != b.DownSQL {
				t.Errorf("down SQL が 2 回の生成で一致しない")
			}
			if len(a.Rows) != len(b.Rows) {
				t.Errorf("投入行数が一致しない: %d vs %d", len(a.Rows), len(b.Rows))
			}
			for i := range a.Rows {
				if a.Rows[i] != b.Rows[i] {
					t.Errorf("行 %d が一致しない: %v vs %v", i, a.Rows[i], b.Rows[i])
				}
			}
		})
	}
}

// TestAliases_AllLayersExercised は層 A / B / C-rush / C-movement が各 1 件以上通ることを固定する。
func TestAliases_AllLayersExercised(t *testing.T) {
	rows := []MoveRow{
		mkRow("guile", "standing_light_punch", "normal", "立ち弱P", "p_l", false),                  // 層 B
		mkRow("guile", "sonic_boom_light", "special", "弱ソニックブーム", "charge_l r plus p_l", false), // 層 A
		mkRush("guile", "rush_standing_light_punch", "立ち弱P(ラッシュ)", "standing_light_punch"),      // 層 C-rush
	}
	res := genOne(t, NumericRule, "guile", rows)
	for _, l := range []aliasLayer{LayerA, LayerB, LayerCRush} {
		if res.LayerStats[l] < 1 {
			t.Errorf("層 %s の投入が 0 件(各層が 1 件以上通ること)", l)
		}
	}
	if len(res.Movement) != 9 {
		t.Errorf("層 C-movement = %d, want 9", len(res.Movement))
	}
	// 層 A の値が正しいこと(溜め記号はそのまま使う = §4.3 の処遇表)。
	if got, _ := aliasOf(res, "sonic_boom_light"); got.AliasText != "[4]6LP" {
		t.Errorf("sonic_boom_light = %q, want [4]6LP(溜めの角括弧はそのまま使う)", got.AliasText)
	}
}

// TestAliases_SQLShape は生成 SQL の要点(preset を code で引く・NOT EXISTS ガード・
// 移動系がキャラを列挙しない)を固定する。
func TestAliases_SQLShape(t *testing.T) {
	res := genOne(t, NumericRule, "guile", []MoveRow{
		mkRow("guile", "standing_light_punch", "normal", "立ち弱P", "p_l", false),
	})

	// ★preset は id リテラルではなく code で引くこと(id は投入順の産物)。
	if !strings.Contains(res.UpSQL, "SELECT id FROM presets WHERE code = 'numeric'") {
		t.Errorf("up SQL が presets を code で引いていない")
	}
	if strings.Contains(res.UpSQL, "preset_id, move_id, alias_text)\nSELECT 3,") {
		t.Errorf("up SQL が preset id をリテラルで埋め込んでいる")
	}
	// ★再適用時に skip するガードがあること(§4.9)。
	if strings.Count(res.UpSQL, "NOT EXISTS") < 2 {
		t.Errorf("up SQL の NOT EXISTS ガードが足りない(移動系 + キャラ別で 2 箇所以上)")
	}
	// ★移動系はキャラを列挙しない(CSV を持たないキャラにも当たるため)。
	movementBlock := res.UpSQL[strings.Index(res.UpSQL, "-- ===== 層 C-1"):strings.Index(res.UpSQL, "-- ===== guile")]
	if strings.Contains(movementBlock, "c.code = 'guile'") {
		t.Errorf("移動系の INSERT がキャラを限定している(全キャラへ当てること)")
	}
	if !strings.Contains(movementBlock, "m.category = 'system'") {
		t.Errorf("移動系の INSERT が category='system' で絞っていない")
	}
	// ★英語表記を持つ行が 1 つも無い塊では alias_text_en 列を出さない。
	//   ★【2026-08-14 更新 = M20-06】旧記述「alias_text_en は移動系のみ」は失効した——
	//     層 C-3(P-34 の 13 件)がキャラ別の塊にも英語表記を入れる(D-373)。
	//     本ケースの guile は層 C-3 の対象を持たないため、依然として 3 列である。
	//     ⇒ 主張は「移動系のみ」ではなく「値を持つ行が無ければ列を出さない」である。
	if strings.Contains(res.UpSQL[strings.Index(res.UpSQL, "-- ===== guile"):], "alias_text_en") {
		t.Errorf("英語表記を持たない塊に alias_text_en 列が出ている(NULL だけの列が増える)")
	}
	// down は投入した code のみを消すこと。
	if !strings.Contains(res.DownSQL, "'standing_light_punch'") {
		t.Errorf("down SQL が投入 code を列挙していない")
	}
	if !strings.Contains(res.DownSQL, "preset_id = (SELECT id FROM presets WHERE code = 'numeric')") {
		t.Errorf("down SQL が preset_id で絞っていない(他プリセットを巻き添えにする)")
	}
}

// TestAliases_MovementDownIsScopedToItsOwnChars は「次の seed 波の down が
// 前の波の移動系エイリアスを巻き添えにしない」ことを固定する（★M20-02 レビュー H-2 の回帰テスト）。
//
// ★これは規則そのものに埋まっていた罠である。
// 旧実装は移動系を「characters を CROSS JOIN して全キャラへ」投入し、down は preset_id と
// 9 code だけで DELETE していた。up は NOT EXISTS で既存キャラを skip する一方 down は
// 無条件に消すため、波 2 のマイグレを down すると波 1 が入れた全キャラ分まで消えた
// （`SUPP-001` §5.5.2 (5)「down のスコープ条件は up の反転ではない」の一形）。
//
// ★マイグレ側のテストでは検出できない——波を跨いだ down を回す経路が無いため。
// 生成器レベルで「波 2 の down SQL に波 1 のキャラが現れないこと」を見るのが唯一の守り方である。
func TestAliases_MovementDownIsScopedToItsOwnChars(t *testing.T) {
	const wave1Char, wave2Char = "ryu", "guile"
	rows := []MoveRow{mkRow(wave1Char, "standing_light_punch", "normal", "立ち弱P", "p_l", false)}

	// 波 1: ryu だけへ移動系を投入する。
	wave1, err := GenerateAliases(NumericRule, []string{wave1Char},
		map[string][]MoveRow{wave1Char: rows}, []string{wave1Char}, AliasHeader("wave1", "wave1"))
	if err != nil {
		t.Fatalf("wave1: %v", err)
	}
	// 波 2: guile だけへ移動系を投入する（ryu は波 1 で投入済み）。
	rows2 := []MoveRow{mkRow(wave2Char, "standing_light_punch", "normal", "立ち弱P", "p_l", false)}
	wave2, err := GenerateAliases(NumericRule, []string{wave2Char},
		map[string][]MoveRow{wave2Char: rows2}, []string{wave2Char}, AliasHeader("wave2", "wave2"))
	if err != nil {
		t.Fatalf("wave2: %v", err)
	}

	// ★核心: 移動系 down ブロックが「自分の波のキャラだけ」で絞られていること。
	//
	// ★ここは Contains だけで書いてはならない。スコープ句ごと外すと他の波のキャラは
	//   「現れない」ため、Contains による不在チェックは通ってしまう(実際に破壊テストで
	//   すり抜けを確認した)。⇒ スコープ句の存在そのものを先に主張する。
	w1mv := movementDownBlock(t, wave1.DownSQL)
	w2mv := movementDownBlock(t, wave2.DownSQL)

	for _, c := range []struct {
		name, block, want, notWant string
	}{
		{"波 1 の移動系 down", w1mv, wave1Char, wave2Char},
		{"波 2 の移動系 down", w2mv, wave2Char, wave1Char},
	} {
		if !strings.Contains(c.block, "c.code IN (") {
			t.Errorf("%s にキャラのスコープ句が無い。スコープが無い DELETE は"+
				"他の波が投入した移動系エイリアスまで消す\n%s", c.name, c.block)
			continue
		}
		if !strings.Contains(c.block, "'"+c.want+"'") {
			t.Errorf("%s に自分のキャラ %q が無い(down が何も消さない)", c.name, c.want)
		}
		if strings.Contains(c.block, "'"+c.notWant+"'") {
			t.Errorf("%s に他の波のキャラ %q が現れる。前の波の投入分を巻き添えにする",
				c.name, c.notWant)
		}
	}

	// ★up も同じ集合で絞られていること(up/down の対称性)。
	for _, c := range []struct {
		name, sql, want, notWant string
	}{
		{"wave2 up", wave2.UpSQL, wave2Char, wave1Char},
		{"wave1 up", wave1.UpSQL, wave1Char, wave2Char},
	} {
		if !strings.Contains(c.sql, "c.code IN (") {
			t.Errorf("%s の移動系にキャラのスコープ句が無い", c.name)
		}
		if !strings.Contains(c.sql, "'"+c.want+"'") {
			t.Errorf("%s に %q が無い", c.name, c.want)
		}
		if strings.Contains(c.sql, "'"+c.notWant+"'") {
			t.Errorf("%s に他の波のキャラ %q が現れる(up/down は同じ集合で絞ること)", c.name, c.notWant)
		}
	}
}

// movementDownBlock は down SQL から移動系ブロックだけを切り出す。
// ★ブロック単位で見ること——down 全体を Contains すると、キャラ別ブロックに現れる
// キャラ code を移動系のスコープと取り違える。
func movementDownBlock(t *testing.T, downSQL string) string {
	t.Helper()
	const marker = "-- ===== 層 C-1: 移動系"
	i := strings.Index(downSQL, marker)
	if i < 0 {
		t.Fatalf("down SQL に移動系ブロックが無い:\n%s", downSQL)
	}
	rest := downSQL[i:]
	if j := strings.Index(rest, "\n\n"); j >= 0 {
		return rest[:j]
	}
	return rest
}

// TestAliases_MovementCharsRequired は movementChars の指定漏れを黙って通さないことを固定する。
//
// ★空で通すと移動系ブロックが 0 キャラ相手の SQL になり、静かに何も投入しない。
func TestAliases_MovementCharsRequired(t *testing.T) {
	_, err := GenerateAliases(NumericRule, []string{"ryu"},
		map[string][]MoveRow{"ryu": {mkRow("ryu", "standing_light_punch", "normal", "立ち弱P", "p_l", false)}},
		nil, AliasHeader("x", "x"))
	if err == nil {
		t.Fatal("movementChars が空でもエラーにならない（静かに何も投入しない形になる）")
	}
	if !strings.Contains(err.Error(), "movementChars") {
		t.Errorf("エラーメッセージが原因を示していない: %v", err)
	}
}
