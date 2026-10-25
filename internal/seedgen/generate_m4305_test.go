package seedgen

import (
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// M43-05(CHANGE-257)の生成器テスト。
//
//   - 規則: ターゲットコンボは段ごとに通常技の表記を `~` でつなぎ末尾に `(TC)`(numeric `5MP~5HP(TC)` ／
//     srk `st.MP~st.HP (TC)`)。空中から出す技には `j.`(垂直ジャンプ `nj.` ／ ハイジャンプ `hj.`)
//   - 範囲: ターゲットコンボ ・空中の技(とその rush 版)の外の行は変わらない(生成器が止まる)
//   - 生成物: 差分マイグレ 000014 がリポジトリに在る(★M43-07 以降、固定済みなので今の CSV と照らさない)／
//     被りの表 internal/aliascollision/table_gen.go が生成器の出力と一致する(マイグレではないので今の CSV に追従する)

const m4305Stem = "000014_data_preset_aliases_tc_aerial_and_kimberly_derivations"

// TestM4305_TargetComboNotation はターゲットコンボの表記の形を固定する(Plan §0 §9.4-1 の開発者決定)。
func TestM4305_TargetComboNotation(t *testing.T) {
	tc := func(code, command string) MoveRow {
		return mkRow("ryu", code, "target_combo", code, command, true)
	}
	aerial := mkRow("chun_li", "soaring_eagle_punches", "target_combo", "鷹嘴連拳", "p_h chain p_h", true)
	// ★対照(開発者決定・追補①): 名前に jumping を含むが地上のターゲットコンボ。`j.` を付けない。
	notAerial := mkRow("terry", "jumping_knee", "target_combo", "ジャンプニーアタック", "p_m chain k_m chain k_m", true)
	cases := []struct {
		row          MoveRow
		numeric, srk string
	}{
		{tc("plain", "p_h chain k_h"), "5HP~5HK(TC)", "st.HP~st.HK (TC)"},
		{tc("four_hits", "p_l chain p_m chain d plus p_h chain k_h"), "5LP~5MP~2HP~5HK(TC)", "st.LP~st.MP~cr.HP~st.HK (TC)"},
		{tc("crouching", "d plus k_l chain d plus k_h"), "2LK~2HK(TC)", "cr.LK~cr.HK (TC)"},
		{tc("forward", "r plus p_h chain r plus p_h chain k_h"), "6HP~6HP~5HK(TC)", "6HP~6HP~st.HK (TC)"},
		{tc("back", "l plus p_m chain k_h"), "4MP~5HK(TC)", "4MP~st.HK (TC)"},
		{tc("diagonal", "p_m chain dr plus k_h chain d plus k_h"), "5MP~3HK~2HK(TC)", "st.MP~3HK~cr.HK (TC)"},
		{tc("double_down", "d d plus k_m chain k_m"), "22MK~5MK(TC)", "22MK~st.MK (TC)"},
		{tc("any_strength", "r plus k_h chain l plus k_h chain p"), "6HK~4HK~5P(TC)", "6HK~4HK~st.P (TC)"},
		// ★空中のターゲットコンボは全段に `j.`(開発者決定)。
		{aerial, "j.HP~j.HP(TC)", "j.HP~j.HP (TC)"},
		{notAerial, "5MP~5MK~5MK(TC)", "st.MP~st.MK~st.MK (TC)"},
	}
	for _, c := range cases {
		for _, p := range []struct {
			rule AliasPresetRule
			want string
		}{{NumericRule, c.numeric}, {SRKRule, c.srk}} {
			res := genOne(t, p.rule, c.row.CharacterCode, withIndex(c.row))
			got, ok := aliasOf(res, c.row.MoveCode)
			if !ok || got.AliasText != p.want {
				t.Errorf("%s %s = %q (ok=%v), want %q", p.rule.PresetCode, c.row.MoveCode, got.AliasText, ok, p.want)
			}
		}
	}

	// ★対照: ターゲットコンボでない `chain` の技(必殺技の派生入力)は今の `>` の形のまま(範囲はカテゴリで決める)。
	sp := mkRow("jamie", "freeflow_strikes_2hits_light", "special", "流酔拳", "d dr r plus p_l chain r plus p", false)
	res := genOne(t, NumericRule, "jamie", withIndex(sp))
	if got, ok := aliasOf(res, sp.MoveCode); !ok || got.AliasText != "236LP>6P" {
		t.Errorf("ターゲットコンボでない chain の技 = %q (ok=%v), want 236LP>6P", got.AliasText, ok)
	}
}

// TestM4305_TargetComboUnrenderableIsReported は段を写せないターゲットコンボを投入せず報告することを固定する。
func TestM4305_TargetComboUnrenderableIsReported(t *testing.T) {
	// charge_l は `[4]` へ正規化される。ターゲットコンボの段の語彙(方向の数字 ＋ ボタン)の外である。
	row := mkRow("ryu", "odd_tc", "target_combo", "変な TC", "charge_l r plus p_l chain p_h", true)
	res := genOne(t, NumericRule, "ryu", withIndex(row))
	if got, ok := aliasOf(res, "odd_tc"); ok {
		t.Errorf("写せないターゲットコンボに表記が付いた: %q", got.AliasText)
	}
	if u, ok := unfilledOf(res, "odd_tc"); !ok || u.Reason != ReasonTargetComboUnrenderable {
		t.Errorf("odd_tc = %+v (ok=%v), want %s", u, ok, ReasonTargetComboUnrenderable)
	}
}

// TestM4305_AerialPrefix は空中の技の接頭辞を固定する(Plan §0 §9.4-2)。
//
// ★在る条件と無い条件の両方を見る(playbook §4.75)——地上の技には付かない ・二重にならない。
func TestM4305_AerialPrefix(t *testing.T) {
	aerialRow := func(char, code, category, command string) MoveRow {
		r := mkRow(char, code, category, code, command, false)
		r.IsAerial = true
		return r
	}
	sa := mkRow("chun_li", "sa1_aerial_kikosho", "super_art", "SA1 空中気功掌", "d dr r d dr r plus p", false)
	cases := []struct {
		row  MoveRow
		want string
	}{
		// 固定表(is_aerial=false の必殺技 ・SA)
		{mkRow("cammy", "cannon_strike_light", "special", "弱キャノンストライク", "d dl l plus k_l", false), "j.214LK"},
		{sa, "j.236236P (SA1)"},
		{mkRow("c_viper", "high_jump_light_aerial_burning_kick", "special", "【ハイジャンプ】弱空中バーニングキック", "d dr r plus k_l", true), "hj.236LK"},
		// is_aerial=true の通常技 ・特殊技
		{aerialRow("zangief", "flying_body_press", "unique", "d plus p_h"), "j.2HP"},
		{aerialRow("marisa", "jumping_heavy_punch_holding", "normal", "p_h hold"), "j.HP(hold)"},
		{aerialRow("ken", "neutral_jumping_heavy_kick", "normal", "k_h"), "nj.HK"},
		{aerialRow("c_viper", "high_jumping_light_punch", "unique", "p_l"), "hj.LP"},
		{aerialRow("c_viper", "high_jumping_neutral_heavy_kick", "unique", "k_h"), "hj.nj.HK"},
		// ★二重にしない: 層 B の jumping_* は既に `j.`。
		{aerialRow("ryu", "jumping_heavy_punch", "normal", "p_h"), "j.HP"},
		// ★地上の技には付かない(対照)。
		{mkRow("ryu", "hadoken_light", "special", "弱波動拳", "d dr r plus p_l", false), "236LP"},
		{mkRow("alex", "aerial_knee_smash_light", "special", "弱エアニースマッシュ", "r d dr plus k_l", false), "623LK"},
		// ★is_aerial=true でも除外表の技には付けない(rashid front_flip＝開発者決定)。
		{aerialRow("rashid", "front_flip", "unique", "l or n or r plus k k"), "4/5/6K+K"},
	}
	for _, c := range cases {
		for _, rule := range []AliasPresetRule{NumericRule, SRKRule} {
			res := genOne(t, rule, c.row.CharacterCode, withIndex(c.row))
			got, ok := aliasOf(res, c.row.MoveCode)
			if !ok || got.AliasText != c.want {
				t.Errorf("%s %s/%s = %q (ok=%v), want %q", rule.PresetCode, c.row.CharacterCode, c.row.MoveCode, got.AliasText, ok, c.want)
			}
		}
	}

	// rush 版は元技の表記から合成される(`DR > j.2MK`)。
	orig := mkRow("e_honda", "flying_sumo_press", "unique", "フライングスモウプレス", "d plus k_m", false)
	res := genOne(t, NumericRule, "e_honda", withIndex(orig, mkRush("e_honda", "rush_flying_sumo_press", "フライングスモウプレス(ラッシュ)", "flying_sumo_press")))
	if got, ok := aliasOf(res, "rush_flying_sumo_press"); !ok || got.AliasText != "DR > j.2MK" {
		t.Errorf("rush 版 = %q (ok=%v), want DR > j.2MK", got.AliasText, ok)
	}
}

// aerialEvidence は「空中技かもしれない」印を持つ行を拾う(分類の漏れの検査用)。
//
// 印は 3 つ: move_code の語 ／ 技名の語 ／ notes_tool の「ジャンプから」。
var (
	aerialCodeWord = regexp.MustCompile(`(^|_)(aerial|air|zanku|soaring|flying|dive|diving|jumping|jump|falling|skyhigh)(_|$)`)
	aerialNameWord = regexp.MustCompile(`空中|斬空|ジャンプ`)
)

func hasAerialEvidence(r MoveRow) bool {
	return r.IsAerial || aerialCodeWord.MatchString(r.MoveCode) || aerialNameWord.MatchString(r.NameJA) ||
		strings.Contains(r.NotesTool, "ジャンプから")
}

// TestM4305_AerialCandidatesAreClassified は、空中技の印を持つ行が固定表 ・除外表 ・is_aerial の
// どれかで分類済みであることを実データ全数で主張する。
//
// ★新しいキャラの CSV を足したときに、空中の必殺技が `j.` なしで静かに投入されるのを止めるための検査である
// (is_aerial は必殺技 ・SA の空中版で false)。赤になったら aerialTargets か aerialExcluded へ足すこと
// (character_data/seed-progress.md)。
// ★層 B の jumping_* ・投げ ・システム技 ・rush 版(元技に従う)・層 C-3 の技名の行は見ない。
func TestM4305_AerialCandidatesAreClassified(t *testing.T) {
	order, rows := readAllCharacterRows(t)
	var missing []string
	scanned := 0
	for _, c := range order {
		for _, r := range rows[c] {
			if r.Category == "throw" || r.Category == "system" || r.isRush() {
				continue
			}
			if _, ok := layerB(NumericRule, r.MoveCode); ok {
				continue
			}
			if !hasAerialEvidence(r) {
				continue
			}
			scanned++
			_, excluded := aerialExcluded[c][r.MoveCode]
			if r.IsAerial || aerialTargets[c][r.MoveCode] || excluded || isNoInputDerivedTarget(c, r.MoveCode) {
				continue
			}
			missing = append(missing, c+"/"+r.MoveCode+"("+r.NameJA+")")
		}
	}
	sort.Strings(missing)
	for _, m := range missing {
		t.Errorf("空中技の印があるのに分類されていない: %s ⇒ aerialTargets か aerialExcluded へ足すこと", m)
	}
	if scanned == 0 {
		t.Fatal("印を持つ行が 0 件(検査が対象を見ていない)")
	}
	// ★表に書いた技が実データに実在すること(綴りの誤りで静かに効かなくなるのを防ぐ)。
	exists := map[string]bool{}
	for _, c := range order {
		for _, r := range rows[c] {
			exists[c+"/"+r.MoveCode] = true
		}
	}
	for c, codes := range aerialTargets {
		for code := range codes {
			if !exists[c+"/"+code] {
				t.Errorf("aerialTargets の %s/%s が実データに無い", c, code)
			}
		}
	}
	for c, codes := range aerialExcluded {
		for code := range codes {
			if !exists[c+"/"+code] {
				t.Errorf("aerialExcluded の %s/%s が実データに無い", c, code)
			}
		}
	}
}

// TestM4305_GenerationMeasurements は新しい規則の実測値(全 32 キャラ)を固定する。
//
// ★投入行と非投入行を対で固定する(SUPP-001 §5.5.2 (3))。旧規則の実測は TestM4808_GenerationMeasurements。
func TestM4305_GenerationMeasurements(t *testing.T) {
	order, rows := readAllCharacterRows(t)
	for _, tc := range []struct {
		rule                         AliasPresetRule
		oldRows, newRows             int
		changed, added, lost, groups int
		tcRows                       int
	}{
		{rule: NumericRule, oldRows: 2429, newRows: 2502, changed: 159, added: 73, lost: 0, groups: 200, tcRows: 127},
		{rule: SRKRule, oldRows: 2444, newRows: 2502, changed: 174, added: 58, lost: 0, groups: 200, tcRows: 127},
	} {
		t.Run(tc.rule.PresetCode, func(t *testing.T) {
			oldRes, err := GenerateAliases(tc.rule, order, rows, order, AliasHeader("m", "m"), WithFormat(FormatPreM4305))
			if err != nil {
				t.Fatal(err)
			}
			rw, err := GenerateAliasRewrite(tc.rule, order, rows, RewriteHeader("m", tc.rule.PresetCode))
			if err != nil {
				t.Fatal(err)
			}
			res := rw.Result
			tcRows, doubled := 0, 0
			for _, r := range res.Rows {
				if strings.HasSuffix(r.AliasText, "(TC)") {
					tcRows++
				}
				if strings.Contains(r.AliasText, "j.j.") || strings.HasPrefix(r.AliasText, "j.nj.") || strings.HasPrefix(r.AliasText, "j.hj.") {
					doubled++
				}
			}
			byReason := map[unfilledReason]int{}
			for _, u := range res.Unfilled {
				byReason[u.Reason]++
			}
			for _, c := range []struct {
				name      string
				got, want int
			}{
				{"旧規則の行", len(oldRes.Rows), tc.oldRows},
				{"新しい規則の行", len(res.Rows), tc.newRows},
				{"書き換わる行", len(rw.Changed), tc.changed},
				{"新たに付く行", len(rw.Added), tc.added},
				{"消える行", len(rw.Lost), tc.lost},
				{"被りの組", len(res.Collisions), tc.groups},
				{"(TC) の行", tcRows, tc.tcRows},
				{"接頭辞の二重", doubled, 0},
				{"target-combo-unrenderable", byReason[ReasonTargetComboUnrenderable], 0},
			} {
				if c.got != c.want {
					t.Errorf("%s = %d, want %d", c.name, c.got, c.want)
				}
			}
			if len(res.Rows) != len(oldRes.Rows)+len(rw.Added)-len(rw.Lost) {
				t.Errorf("新しい規則の行 %d ≠ 旧 %d ＋ 追加 %d − 削除 %d", len(res.Rows), len(oldRes.Rows), len(rw.Added), len(rw.Lost))
			}
		})
	}
}

// TestM4305_OutOfScopeChangeStopsGeneration は範囲の外の行が変わるなら生成が止まることを固定する(M48-08 の型)。
//
// ★範囲の判定を「何も範囲に入らない」へ差し替え、実データの差分(ターゲットコンボ ・空中の技)を
// 範囲の外の変化として見せる。⇒ 生成が error で止まること。対照として、差し替えない場合は止まらない。
func TestM4305_OutOfScopeChangeStopsGeneration(t *testing.T) {
	order, rows := readAllCharacterRows(t)
	if _, err := GenerateAliasRewrite(NumericRule, order, rows, RewriteHeader("m", "numeric")); err != nil {
		t.Fatalf("対照(範囲の判定そのまま)で止まった: %v", err)
	}
	saved := rewriteScope
	defer func() { rewriteScope = saved }()
	rewriteScope = func(MoveRow, map[string]MoveRow) bool { return false }
	_, err := GenerateAliasRewrite(NumericRule, order, rows, RewriteHeader("m", "numeric"))
	if err == nil || !strings.Contains(err.Error(), "範囲の外") {
		t.Errorf("範囲の外の変化で止まらなかった: err=%v", err)
	}
}

// renderM4305Migration は差分マイグレ 000014 の up / down の全文を作る。
//
// up＝numeric → srk → キンバリー ／ down＝キンバリー → srk → numeric(up の逆順)。
func renderM4305Migration(t *testing.T) (string, string, map[string]*AliasResult) {
	t.Helper()
	order, rows := readAllCharacterRows(t)
	up := M4305MigrationPreamble(m4305Stem, "up")
	down := M4305MigrationPreamble(m4305Stem, "down") + KimberlySprintDerivationsSQL("down")
	results := map[string]*AliasResult{}
	var downs []string
	for _, rule := range []AliasPresetRule{NumericRule, SRKRule} {
		rw, err := GenerateAliasRewrite(rule, order, rows, RewriteHeader(m4305Stem, rule.PresetCode))
		if err != nil {
			t.Fatalf("GenerateAliasRewrite(%s): %v", rule.PresetCode, err)
		}
		up += rw.UpSQL
		downs = append(downs, rw.DownSQL)
		results[rule.PresetCode] = rw.Result
	}
	up += KimberlySprintDerivationsSQL("up")
	for i := len(downs) - 1; i >= 0; i-- {
		down += downs[i]
	}
	return up, down, results
}

// TestM4305_MigrationIsGenerated は差分マイグレ 000014 を固定の一覧の基準で扱う(M43-07)。
//
// ★★000014 は固定済み(適用済み)である。⇒ 在ることだけを確かめ、今の CSV から作り直して照らさない。
// 中身は internal/migrationpin の sha256 が守る。CSV を直して生成器の出力が変わっても、000014 は書き換えず、
// 今の規則を名前付きで凍結して新しい連番の差分マイグレを起こすこと(M48-08 ・M43-05 と同じ型。character_data/seed-progress.md)。
// ★失われる主張＝「000014 が今の CSV ＋ FormatCurrent で再現できる」(M43-07 完了報告 §失われる主張)。
func TestM4305_MigrationIsGenerated(t *testing.T) {
	assertGeneratedMigration(t, m4305Stem, func() (string, string) {
		up, down, _ := renderM4305Migration(t)
		return up, down
	})
}

// TestM4305_CollisionTableIsGenerated は取込の被りの表が今の規則の出力と一致することを固定する。
func TestM4305_CollisionTableIsGenerated(t *testing.T) {
	_, _, results := renderM4305Migration(t)
	src, err := RenderCollisionTable(results)
	if err != nil {
		t.Fatal(err)
	}
	assertGenerated(t, filepath.Join(repoRoot(t), "internal", "aliascollision", "table_gen.go"), src)
}
