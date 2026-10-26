package seedgen

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/plexiblinp/tacpendium-setplay/internal/model"
)

// 本ファイルは M43-05(CHANGE-257)の規則と生成系を提供する。
//
//   - renderTargetCombo … ターゲットコンボを段ごとに通常技の表記で書き `~` でつなぎ、末尾に `(TC)` を付ける
//   - aerialNotation    … 空中から出す技に `j.`(垂直ジャンプ `nj.` ／ ハイジャンプ `hj.`)を付ける
//   - GenerateAliasRewrite … 規則を変えたことで変わる numeric / srk の行を書き換える差分マイグレ
//   - KimberlySprintDerivationsSQL … キンバリーの疾駆け派生 8 技の親(move_derivations)
//
// ★形はすべて開発者が Plan で確かめたもの(M43-05 Plan §0。2026-09-30)。

// targetComboJoiner はターゲットコンボの段をつなぐ文字(開発者の逐語 `5MP~5HP(TC)`)。
//
// ★レシピの連結子 `" > "` とは別の文字にする——旧い `MP>HP` は連結子と同じ文字で読みにくかった(D-1051)。
const targetComboJoiner = "~"

// tcSegment は正規化キーの 1 段(例 "6HP" ・"HP" ・"22MK" ・"P" ・"HP(hold)")を方向とボタンに分ける。
//
// ★方向とボタンの語彙はここで再定義しない。moveindex が command から作った正規化キーを分けるだけである
// (「二度作らない」)。方向はテンキーの数字の並び、ボタンは moveindex の断片の `+` つなぎ。
var tcSegment = regexp.MustCompile(`^([1-9]*)((?:[LMH]?[PK])(?:\+[LMH]?[PK])*(?:\(hold\))?)$`)

// renderTargetCombo はターゲットコンボの表記を作る(M43-05 §4.1)。key は moveindex の正規化キー。
//
// 各段の前置き:
//
//	空中の技           … jumpPrefix(`j.` ／ `nj.` ／ `hj.`)＋ 方向の数字(全段。開発者決定)
//	方向なし           … rule.StandingPrefix(numeric `5` ／ srk `st.`)
//	方向が `2` だけ    … rule.CrouchingPrefix(numeric `2` ／ srk `cr.`)
//	それ以外の方向     … テンキーの数字のまま(`6HP` ・`3HK` ・`22MK`。今の srk の層 A と同じ流儀)
//
// ★段を写せないとき(語彙外の断片)は ok=false。旧い `>` の形で黙って出さない。
func renderTargetCombo(rule AliasPresetRule, r MoveRow, key string) (string, bool) {
	aerial := IsAerialMove(r)
	segs := strings.Split(key, ">")
	out := make([]string, 0, len(segs))
	for _, seg := range segs {
		m := tcSegment.FindStringSubmatch(seg)
		if m == nil {
			return "", false
		}
		dir, btn := m[1], m[2]
		var prefix string
		switch {
		case aerial:
			prefix = jumpPrefix(r.MoveCode) + dir
		case dir == "":
			prefix = rule.StandingPrefix
		case dir == "2":
			prefix = rule.CrouchingPrefix
		default:
			prefix = dir
		}
		out = append(out, prefix+btn)
	}
	return strings.Join(out, targetComboJoiner) + rule.TargetComboSuffix, true
}

// ---- 空中から出す技(M43-05 §4.2) ----------------------------------------------

// aerialTargets は is_aerial=false だが空中から出す技の集合(★Plan §0 で開発者が確かめた一覧)。
//
// ★必殺技 ・SA ・ターゲットコンボの空中版は CSV の is_aerial が false である(M48-RESEARCH-01 §5-4)。
// move_code の語(aerial / air / zanku 等)でも見分けられない——地上技(alex aerial_knee_smash、
// dee_jay air_slasher、zangief sa1_aerial_russian_slam)が当たり、空中技(無影蹴 ・疾空閃 ・
// キャノンストライク)が当たらない。⇒ noInputDerivedTargets と同じく集合を固定表で持つ。
//
// ★新しいキャラを足したら、TestM4305_AerialCandidatesAreClassified が分類の漏れを赤で知らせる。
// そのときは本表か aerialExcluded のどちらかへ足すこと(character_data/seed-progress.md)。
var aerialTargets = map[string]map[string]bool{
	"akuma": setOf("zanku_hadoken_light", "zanku_hadoken_medium", "zanku_hadoken_heavy", "zanku_hadoken_od",
		"aerial_tatsumaki_zanku_kyaku", "aerial_tatsumaki_zanku_kyaku_od", "demon_gou_zanku", "sa1_tenma_gozanku"),
	"blanka": setOf("aerial_rolling_attack_light", "aerial_rolling_attack_medium", "aerial_rolling_attack_heavy",
		"aerial_rolling_attack_od", "lightning_beast_light_aerial_rolling_attack",
		"lightning_beast_medium_aerial_rolling_attack", "lightning_beast_heavy_aerial_rolling_attack",
		"lightning_beast_od_aerial_rolling_attack"),
	// ★【ハイジャンプ】版は jumpPrefix で `hj.` になる(開発者決定)。
	"c_viper": setOf("aerial_burning_kick_light", "aerial_burning_kick_medium", "aerial_burning_kick_heavy",
		"aerial_burning_kick_od", "high_jump_light_aerial_burning_kick", "high_jump_medium_aerial_burning_kick",
		"high_jump_heavy_aerial_burning_kick", "high_jump_od_aerial_burning_kick"),
	"cammy": setOf("cannon_strike_light", "cannon_strike_medium", "cannon_strike_heavy", "cannon_strike_od",
		"sa2_aerial_killer_bee_spin"),
	"chun_li": setOf("aerial_hundred_lightning_kicks_light", "aerial_hundred_lightning_kicks_medium",
		"aerial_hundred_lightning_kicks_heavy", "aerial_hundred_lightning_kicks_od", "sa1_aerial_kikosho",
		"soaring_eagle_punches"),
	"dee_jay": setOf("party_in_the_air"),
	"dhalsim": setOf("yoga_comet_light", "yoga_comet_medium", "yoga_comet_heavy", "yoga_comet_od",
		"aerial_yoga_float", "aerial_yoga_teleport", "p_aerial_yoga_teleport_backward",
		"k_aerial_yoga_teleport_forward", "k_aerial_yoga_teleport_backward"),
	"e_honda": setOf("flying_sumo_press"),
	"elena":   setOf("soaring_raid", "raptor_range"),
	"ingrid": setOf("solar_burst_light_neutral", "solar_burst_light_forward",
		"solar_burst_lv1_neutral", "solar_burst_lv1_forward", "solar_burst_lv1_neutral_od", "solar_burst_lv1_forward_od",
		"solar_burst_lv2_neutral", "solar_burst_lv2_forward", "solar_burst_lv2_neutral_od", "solar_burst_lv2_forward_od",
		"solar_burst_lv3_neutral", "solar_burst_lv3_forward", "solar_burst_lv3_neutral_od", "solar_burst_lv3_forward_od"),
	"jamie":    setOf("luminous_dive_kick_light", "luminous_dive_kick_medium", "luminous_dive_kick_heavy", "luminous_dive_kick_od"),
	"juri":     setOf("shiku_sen", "shiku_sen_od"),
	"ken":      setOf("aerial_tatsumaki_senpu_kyaku", "aerial_tatsumaki_senpu_kyaku_od"),
	"kimberly": setOf("aerial_bushin_senpukyaku", "aerial_bushin_senpukyaku_od", "sa2_soaring_bushin_scramble"),
	"lily": setOf("condor_dive", "condor_dive_od", "windclad_condor_dive", "windclad_od_condor_dive",
		"sa2_soaring_thunderbird", "windclad_sa2_soaring_thunderbird"),
	"luke":   setOf("aerial_flash_knuckle", "aerial_flash_knuckle_od", "aerial_flash_knuckle_holding"),
	"mai":    setOf("sa2_air_chou_hissatsu_shinobi_bachi", "flame_sa2_air_chou_hissatsu_shinobi_bachi"),
	"rashid": setOf("arabian_skyhigh_light", "arabian_skyhigh_medium", "arabian_skyhigh_heavy", "arabian_skyhigh_od"),
	"ryu":    setOf("aerial_tatsumaki_senpu_kyaku", "aerial_tatsumaki_senpu_kyaku_od"),
}

// aerialExcluded は `j.` を付けない技と、その理由(★Plan §0 で開発者が確かめた一覧)。
//
// ★2 種類が入る。(1) is_aerial=true だが `j.` を付けない技(本当の除外)。(2) 空中の印になる語 ・
// ジャンプの親を持つが、開発者が空中技として扱わないと決めた技(分類の漏れの検査を通すための記録)。
var aerialExcluded = map[string]map[string]string{
	"kimberly": {
		"step_up_backward": "移動に近い(command が ul ＝ `j.7` の形になる)",
		"step_up_neutral":  "移動に近い(command が u)",
		"step_up_forward":  "移動に近い(command が ur)",
	},
	"rashid": {
		// ★開発者＝`j.` を付けたくない。is_aerial=1 がデータの誤りと思われる(設計卓へ申し送り)。
		"front_flip": "開発者決定(is_aerial=1 の是正は設計卓へ申し送り)",
	},
	"alex":    {"aerial_knee_smash_light": "地上技", "aerial_knee_smash_medium": "地上技", "aerial_knee_smash_heavy": "地上技", "aerial_knee_smash_od": "地上技", "air_stampede": "開発者決定(入れない)", "rush_air_stampede": "ラッシュ版(元技に従う)"},
	"dee_jay": {"air_slasher_light": "地上技", "air_slasher_medium": "地上技", "air_slasher_heavy": "地上技", "air_slasher_od": "地上技"},
	"zangief": {"sa1_aerial_russian_slam": "地上技", "borscht_dynamite": "開発者決定(入れない)", "borscht_dynamite_od": "開発者決定(入れない)"},
	"akuma":   {"tatsumaki_zanku_kyaku_light": "地上技", "tatsumaki_zanku_kyaku_medium": "地上技", "tatsumaki_zanku_kyaku_heavy": "地上技", "tatsumaki_zanku_kyaku_od": "地上技"},
	"ingrid":  {"pretty_heel_kick": "開発者決定(入れない)"},
	"blanka":  {"raid_jump": "開発者決定(入れない)", "rush_raid_jump": "ラッシュ版(元技に従う)"},
	"c_viper": {"high_jump": "移動(地上から跳ぶ)", "rush_high_jump": "ラッシュ版(元技に従う)"},
	"m_bison": {"head_press": "開発者決定(入れない)", "head_press_od": "開発者決定(入れない)"},
	"chun_li": {"falling_crane": "地上技", "rush_falling_crane": "ラッシュ版(元技に従う)", "wall_jump": "command が空(表記が作られない)"},
	"jamie":   {"falling_star_kick": "地上技", "rush_falling_star_kick": "ラッシュ版(元技に従う)"},
	"mai":     {"musasabi_no_mai": "開発者決定(入れない)", "flame_musasabi_no_mai": "開発者決定(入れない)", "flame_od_musasabi_no_mai": "開発者決定(入れない)"},
	"e_honda": {"rush_flying_sumo_press": "ラッシュ版(元技に従う)"},
	// ★開発者決定(2026-09-30・M43-05 追補①): 名前に jumping を含むが、ジャンプ攻撃ではない地上のターゲットコンボ。
	"terry": {"jumping_lariat": "開発者決定(地上のターゲットコンボ。名前が紛らわしい例外)", "jumping_knee": "開発者決定(地上のターゲットコンボ。名前が紛らわしい例外)"},
}

func setOf(codes ...string) map[string]bool {
	m := make(map[string]bool, len(codes))
	for _, c := range codes {
		m[c] = true
	}
	return m
}

// IsAerialMove は当該行が空中から出す技かを返す(is_aerial ∪ aerialTargets − aerialExcluded)。
//
// ★公開しているのは、マイグレのテストが move_derivations(親がジャンプの技)と突き合わせるためである
// (seedgen は move_derivations を作らないため、DB 側から分類の漏れを見る)。
func IsAerialMove(r MoveRow) bool {
	if _, excluded := aerialExcluded[r.CharacterCode][r.MoveCode]; excluded {
		return false
	}
	return r.IsAerial || aerialTargets[r.CharacterCode][r.MoveCode]
}

// AerialExclusionReason は当該技を空中技として扱わない理由を返す(除外表に無ければ ok=false)。
func AerialExclusionReason(charCode, moveCode string) (string, bool) {
	reason, ok := aerialExcluded[charCode][moveCode]
	return reason, ok
}

// jumpPrefix は空中の技の接頭辞を返す(開発者決定。両プリセット共通)。
//
//	high_jumping_neutral_* → `hj.nj.`(ハイジャンプの垂直)
//	high_jumping_* / high_jump_* → `hj.`(ハイジャンプ)
//	neutral_jumping_* → `nj.`(垂直ジャンプ)
//	それ以外 → `j.`
//
// ★`nj.` ・`hj.` を置いた理由——垂直ジャンプ ・ハイジャンプの通常技は、`j.` だけだと層 B の
// `jumping_*`(`j.HP` 等)と被って表記が消える(Plan §2 の 3.3-2「消える 11 行」)。
func jumpPrefix(moveCode string) string {
	switch {
	case strings.HasPrefix(moveCode, "high_jumping_neutral_"):
		return "hj.nj."
	case strings.HasPrefix(moveCode, "high_jumping_"), strings.HasPrefix(moveCode, "high_jump_"):
		return "hj."
	case strings.HasPrefix(moveCode, "neutral_jumping_"):
		return "nj."
	}
	return "j."
}

// aerialNotation は空中の技の表記に接頭辞を付ける(それ以外はそのまま返す)。
//
// ★既に `j.` で始まる表記(層 B の `jumping_*`)には付けない(二重にしない)。
func aerialNotation(r MoveRow, text string) string {
	if !IsAerialMove(r) || strings.HasPrefix(text, "j.") {
		return text
	}
	return jumpPrefix(r.MoveCode) + text
}

// inM4305Scope は当該行が M43-05 の規則で表記が変わりうる行かを返す(差分マイグレの範囲の検査)。
func inM4305Scope(r MoveRow, byMove map[string]MoveRow) bool {
	if r.Category == model.MoveCategoryTargetCombo || IsAerialMove(r) {
		return true
	}
	if r.isRush() {
		if orig, ok := byMove[strings.TrimSpace(r.OriginalMoveCode)]; ok {
			return inM4305Scope(orig, byMove)
		}
	}
	return false
}

// ---- 差分マイグレ(000014) ----------------------------------------------------

// rewriteScope は差分マイグレの範囲の判定(★テストが差し替えて、範囲の外の変化で生成が止まることを見る)。
var rewriteScope = inM4305Scope

// AliasRewriteRow は差分マイグレの 1 行(書き換え ・追加 ・削除のいずれか)。
type AliasRewriteRow struct {
	CharCode string
	MoveCode string
	OldText  string // 追加の行では ""
	NewText  string // 削除の行では ""
}

// AliasRewrite は GenerateAliasRewrite の成果物。
type AliasRewrite struct {
	UpSQL   string
	DownSQL string
	Changed []AliasRewriteRow
	Added   []AliasRewriteRow
	Lost    []AliasRewriteRow
	// Result は新しい規則で全キャラを生成した結果(被りの表の材料)。
	Result *AliasResult
}

// GenerateAliasRewrite は 1 プリセット分の差分(FormatCurrent − FormatPreM4305)の SQL を作る。
//
// ★範囲の外の行が変わるなら生成を止める(M48-08 の型)。範囲＝ターゲットコンボ ・空中の技
// (と、その rush 版)。それ以外で付く ・消える行は、旧い被りの組 ・新しい被りの組に範囲内の技が
// 居たもの(被りが解けた ・新たに被った)に限る。
//
// ★組み込み(numeric / srk)の行は元の値を条件に書き換える。カスタムプリセットは、値が旧い組み込みの値と
// 一致する行(＝書き換えていない複製)だけを書き換える(Plan §0 §9.4-4 (c))。新しい行は足さない(D-1050)。
func GenerateAliasRewrite(rule AliasPresetRule, charOrder []string, rowsByChar map[string][]MoveRow,
	header func(dir string) string) (*AliasRewrite, error) {
	if err := validate(charOrder, rowsByChar); err != nil {
		return nil, err
	}
	out := &AliasRewrite{Result: &AliasResult{LayerStats: map[aliasLayer]int{}}}
	var up, down strings.Builder
	up.WriteString(header("up"))
	down.WriteString(header("down"))
	var downBlocks []string
	for _, charCode := range charOrder {
		rows := rowsByChar[charCode]
		byMove := map[string]MoveRow{}
		for _, r := range rows {
			byMove[r.MoveCode] = r
		}
		oldRows, _, _, _, oldColl := resolveCharacter(rule, charCode, rows, FormatPreM4305)
		newRows, unfilled, discarded, saNoAnn, newColl := resolveCharacter(rule, charCode, rows, FormatCurrent)
		res := out.Result
		res.Unfilled = append(res.Unfilled, unfilled...)
		res.DiscardedLayerA = append(res.DiscardedLayerA, discarded...)
		res.SAWithoutAnnotation = append(res.SAWithoutAnnotation, saNoAnn...)
		res.Collisions = append(res.Collisions, newColl...)
		res.Rows = append(res.Rows, newRows...)
		for _, r := range newRows {
			res.LayerStats[r.Layer]++
		}

		oldText := map[string]string{}
		for _, r := range oldRows {
			oldText[r.MoveCode] = r.AliasText
		}
		newText := map[string]string{}
		for _, r := range newRows {
			newText[r.MoveCode] = r.AliasText
		}
		// 被りの組に範囲内の技が居たか(被りが解けた ・新たに被った技の検査)。
		scopedGroup := func(groups []AliasCollision, code string) bool {
			for _, g := range groups {
				members := append([]string{g.Kept}, g.Dropped...)
				hasCode, hasScoped := false, false
				for _, m := range members {
					if m == code {
						hasCode = true
					}
					if rewriteScope(byMove[m], byMove) {
						hasScoped = true
					}
				}
				if hasCode && hasScoped {
					return true
				}
			}
			return false
		}

		var changed, added, lost []AliasRewriteRow
		for _, code := range sortedKeys(oldText) {
			o := oldText[code]
			n, ok := newText[code]
			switch {
			case !ok:
				if !rewriteScope(byMove[code], byMove) && !scopedGroup(newColl, code) {
					return nil, fmt.Errorf("preset %q: %s/%s の表記 %q が範囲の外で消える", rule.PresetCode, charCode, code, o)
				}
				lost = append(lost, AliasRewriteRow{CharCode: charCode, MoveCode: code, OldText: o})
			case n != o:
				if !rewriteScope(byMove[code], byMove) {
					return nil, fmt.Errorf("preset %q: %s/%s の表記が範囲の外で変わる(%q → %q)", rule.PresetCode, charCode, code, o, n)
				}
				changed = append(changed, AliasRewriteRow{CharCode: charCode, MoveCode: code, OldText: o, NewText: n})
			}
		}
		for _, code := range sortedKeys(newText) {
			if _, ok := oldText[code]; ok {
				continue
			}
			if !rewriteScope(byMove[code], byMove) && !scopedGroup(oldColl, code) {
				return nil, fmt.Errorf("preset %q: %s/%s に範囲の外で表記 %q が付く", rule.PresetCode, charCode, code, newText[code])
			}
			added = append(added, AliasRewriteRow{CharCode: charCode, MoveCode: code, NewText: newText[code]})
		}
		if err := checkRewriteOrder(oldText, lost, changed, added); err != nil {
			return nil, fmt.Errorf("preset %q: %s: %w", rule.PresetCode, charCode, err)
		}
		if len(changed)+len(added)+len(lost) == 0 {
			continue
		}
		out.Changed = append(out.Changed, changed...)
		out.Added = append(out.Added, added...)
		out.Lost = append(out.Lost, lost...)
		up.WriteString(buildRewriteUp(rule, charCode, lost, changed, added))
		downBlocks = append(downBlocks, buildRewriteDown(rule, charCode, lost, changed, added))
	}
	for i := len(downBlocks) - 1; i >= 0; i-- {
		down.WriteString(downBlocks[i])
	}
	out.UpSQL = up.String()
	out.DownSQL = down.String()
	return out, nil
}

// checkRewriteOrder は up(削除 → 書き換え → 追加)と down(削除 → 書き換えを戻す → 戻す追加)の
// どの時点でも、同じキャラ ・プリセットで表記が重ならないことを確かめる。
//
// ★UNIQUE(preset_id, character_id, alias_text) は 1 行ずつ検査される。最終の状態が正しくても、
// 書き換えの途中で「別の技の旧い表記」と一致すると up が落ちる。
func checkRewriteOrder(oldText map[string]string, lost, changed, added []AliasRewriteRow) error {
	live := map[string]string{} // 表記 → 技
	for code, t := range oldText {
		live[t] = code
	}
	take := func(t, code, phase string) error {
		if other, taken := live[t]; taken && other != code {
			return fmt.Errorf("%s で表記 %q が %s と %s で重なる", phase, t, other, code)
		}
		live[t] = code
		return nil
	}
	for _, r := range lost {
		delete(live, r.OldText)
	}
	for _, r := range changed {
		delete(live, r.OldText)
		if err := take(r.NewText, r.MoveCode, "up の書き換え"); err != nil {
			return err
		}
	}
	for _, r := range added {
		if err := take(r.NewText, r.MoveCode, "up の追加"); err != nil {
			return err
		}
	}
	// down
	for _, r := range added {
		delete(live, r.NewText)
	}
	for i := len(changed) - 1; i >= 0; i-- {
		r := changed[i]
		delete(live, r.NewText)
		if err := take(r.OldText, r.MoveCode, "down の書き換え"); err != nil {
			return err
		}
	}
	for _, r := range lost {
		if err := take(r.OldText, r.MoveCode, "down の復元"); err != nil {
			return err
		}
	}
	return nil
}

// moveIDSubquery は (キャラ, 技) の moves.id を引く副問い合わせ。
func moveIDSubquery(charCode, moveCode string) string {
	return fmt.Sprintf("(SELECT m.id FROM moves m JOIN characters c ON c.id = m.character_id WHERE %s AND m.code = %s)",
		charFilter("c", charCode), sqlStr(moveCode))
}

// customPresetIDs はその組み込みを基にしたカスタムプリセットの id の集合。
func customPresetIDs(rule AliasPresetRule) string {
	return "(SELECT id FROM presets WHERE is_builtin = 0 AND base_preset_code = " + sqlStr(rule.PresetCode) + ")"
}

// updateBuiltin は組み込みの 1 行を from → to へ書き換える(★元の値を条件にする)。
func updateBuiltin(b *strings.Builder, rule AliasPresetRule, charCode, moveCode, from, to string) {
	fmt.Fprintf(b, "UPDATE preset_aliases SET alias_text = %s\n    WHERE preset_id = %s AND alias_text = %s\n      AND move_id = %s;\n",
		sqlStr(to), presetIDSubquery(rule), sqlStr(from), moveIDSubquery(charCode, moveCode))
}

// updateCustom はカスタムの行のうち、値が from の行(＝書き換えていない複製)だけを to へ書き換える。
//
// ★同じカスタム ・キャラで to が既に使われていたら書き換えない(利用者が別の技に同じ表記を付けていた場合。
// 書き換えると UNIQUE に触れて up が落ちる)。
func updateCustom(b *strings.Builder, rule AliasPresetRule, charCode, moveCode, from, to string) {
	fmt.Fprintf(b, "UPDATE preset_aliases SET alias_text = %s\n    WHERE preset_id IN %s AND alias_text = %s\n      AND move_id = %s\n"+
		"      AND NOT EXISTS (SELECT 1 FROM preset_aliases o WHERE o.preset_id = preset_aliases.preset_id\n"+
		"          AND o.character_id IS preset_aliases.character_id AND o.alias_text = %s);\n",
		sqlStr(to), customPresetIDs(rule), sqlStr(from), moveIDSubquery(charCode, moveCode), sqlStr(to))
}

// insertBuiltin は組み込みへ 1 行を足す(★既に行があれば足さない)。
func insertBuiltin(b *strings.Builder, rule AliasPresetRule, charCode, moveCode, text string) {
	fmt.Fprintf(b, "INSERT INTO preset_aliases (preset_id, move_id, character_id, alias_text)\n"+
		"SELECT %s, m.id, m.character_id, %s\n"+
		"FROM moves m JOIN characters c ON c.id = m.character_id\n"+
		"WHERE %s AND m.code = %s\n"+
		"  AND NOT EXISTS (SELECT 1 FROM preset_aliases pa WHERE pa.preset_id = %s AND pa.move_id = m.id);\n",
		presetIDSubquery(rule), sqlStr(text), charFilter("c", charCode), sqlStr(moveCode), presetIDSubquery(rule))
}

// deleteBuiltin は組み込みの 1 行を (技, 表記) で消す。
func deleteBuiltin(b *strings.Builder, rule AliasPresetRule, charCode, moveCode, text string) {
	fmt.Fprintf(b, "DELETE FROM preset_aliases\n    WHERE preset_id = %s AND alias_text = %s\n      AND move_id = %s;\n",
		presetIDSubquery(rule), sqlStr(text), moveIDSubquery(charCode, moveCode))
}

func buildRewriteUp(rule AliasPresetRule, charCode string, lost, changed, added []AliasRewriteRow) string {
	var b strings.Builder
	fmt.Fprintf(&b, "-- ===== %s (書き換え %d ・追加 %d ・削除 %d) =====\n", charCode, len(changed), len(added), len(lost))
	for _, r := range lost {
		deleteBuiltin(&b, rule, charCode, r.MoveCode, r.OldText)
	}
	for _, r := range changed {
		updateBuiltin(&b, rule, charCode, r.MoveCode, r.OldText, r.NewText)
	}
	for _, r := range added {
		insertBuiltin(&b, rule, charCode, r.MoveCode, r.NewText)
	}
	if len(changed) > 0 {
		fmt.Fprintf(&b, "-- %s: カスタムプリセットの書き換えていない複製\n", charCode)
		for _, r := range changed {
			updateCustom(&b, rule, charCode, r.MoveCode, r.OldText, r.NewText)
		}
	}
	b.WriteString("\n")
	return b.String()
}

func buildRewriteDown(rule AliasPresetRule, charCode string, lost, changed, added []AliasRewriteRow) string {
	var b strings.Builder
	fmt.Fprintf(&b, "-- ===== %s =====\n", charCode)
	if len(changed) > 0 {
		fmt.Fprintf(&b, "-- %s: カスタムプリセット(up が書き換えた値のままの行)\n", charCode)
		for i := len(changed) - 1; i >= 0; i-- {
			r := changed[i]
			updateCustom(&b, rule, charCode, r.MoveCode, r.NewText, r.OldText)
		}
	}
	for _, r := range added {
		deleteBuiltin(&b, rule, charCode, r.MoveCode, r.NewText)
	}
	for i := len(changed) - 1; i >= 0; i-- {
		r := changed[i]
		updateBuiltin(&b, rule, charCode, r.MoveCode, r.NewText, r.OldText)
	}
	for _, r := range lost {
		insertBuiltin(&b, rule, charCode, r.MoveCode, r.OldText)
	}
	b.WriteString("\n")
	return b.String()
}

// RewriteHeader は GenerateAliasRewrite 用の生成 SQL ヘッダを組み立てる。
func RewriteHeader(stem, presetCode string) func(dir string) string {
	return func(dir string) string {
		return "-- ===== preset " + presetCode + " =====\n" +
			"-- " + stem + "." + dir + ".sql の " + presetCode + " の塊(cmd/seedgen ではなく生成器のテストが生成する。手編集しない)。\n\n"
	}
}

// M4305MigrationPreamble は差分マイグレ(000014)の冒頭のコメントである。
//
// ★「何を戻さないか」をここへ書く(playbook §4.17)。
func M4305MigrationPreamble(stem, dir string) string {
	h := "-- " + stem + "." + dir + ".sql\n" +
		"-- M43-05(CHANGE-257): 組み込みプリセット numeric / srk の表記を、ターゲットコンボ(`5MP~5HP(TC)` ／ `st.MP~st.HP (TC)`)と\n" +
		"-- 空中の技の接頭辞(`j.` ・`nj.` ・`hj.`)の規則で書き換える。あわせてキンバリーの疾駆け派生 8 技の親を move_derivations へ足す。\n" +
		"-- 本ファイルは internal/seedgen の GenerateAliasRewrite / KimberlySprintDerivationsSQL が character_data/*.csv から生成した成果物(手編集しない)。\n" +
		"--   再生成: TACPENDIUM_UPDATE_GOLDEN=1 go test ./internal/seedgen/ -run TestM4305_MigrationIsGenerated\n" +
		"--   ★適用済みになった後は再生成しない(CSV を直すと生成器の出力が変わるため。character_data/seed-progress.md)。\n" +
		"-- ★組み込みの行は元の値を条件に書き換える。カスタムプリセットは、値が旧い組み込みの値と一致する行\n" +
		"--   (＝書き換えていない複製)だけを書き換え、新しい行は足さない(D-1050)。\n" +
		"-- ★recipe_cache は書き換えない(D-615)。表示は設定の「キャッシュ再構築」で新しい表記になる。\n"
	if dir == "down" {
		h += "-- down は up の逆である(足した行を消す ・書き換えた行を元の値へ戻す ・キンバリーの足した組だけを消す)。\n" +
			"-- ★忠実に戻せないもの(down は状態を持たないため、up が書き換えた行と区別できない):\n" +
			"--   (1) カスタムプリセットで「新しい表記と同じ文字列」を持つ行は、利用者が自分で入れた値でも旧い表記へ戻る\n" +
			"--       (up の前から入れていた行 ・up の後に書き換えた行の両方)。\n" +
			"--   (2) up の後に作ったカスタムプリセットが複製した「up が足した行」は消さない(組み込みの行だけを消す)。\n"
	}
	return h + "\n"
}

// ---- キンバリーの疾駆け派生(M43-05 §4.3) --------------------------------------

// kimberlySprintDerivations は疾駆け派生 8 技の親(子 → 親)。
//
// ★無印は sprint、OD 版は sprint_od(発生の値がそれぞれの疾駆けからの通し値＝Plan §2 の 3.3-3)。
// ★2 つとも登録しない——親が複数あると、セットプレイの解決器は move_code 昇順の先頭(sprint)を
// 常に選ぶ(internal/service/setplay の newParentResolver)。
var kimberlySprintDerivations = [][2]string{
	{"emergency_stop", "sprint"}, {"emergency_stop_od", "sprint_od"},
	{"torso_cleaver", "sprint"}, {"torso_cleaver_od", "sprint_od"},
	{"shadow_slide", "sprint"}, {"shadow_slide_od", "sprint_od"},
	{"neck_hunter", "sprint"}, {"neck_hunter_od", "sprint_od"},
}

// KimberlySprintDerivationsSQL はキンバリーの疾駆け派生の親を足す(up)／消す(down)SQL を返す。
//
// ★move_derivations は seedgen が作らない表である(000006 は手で書かれた)。本関数は差分マイグレを
// 1 本にまとめるために同じテストから出す(指示書 §4.4)。★down は足した組だけを消す(表を空にしない)。
func KimberlySprintDerivationsSQL(dir string) string {
	pairs := make([]string, len(kimberlySprintDerivations))
	for i, p := range kimberlySprintDerivations {
		pairs[i] = fmt.Sprintf("        (%s, %s)", sqlStr(p[0]), sqlStr(p[1]))
	}
	values := strings.Join(pairs, ",\n")
	var b strings.Builder
	b.WriteString("-- ===== move_derivations: kimberly の疾駆け派生 8 技の親 =====\n")
	sel := "SELECT child.id, parent.id\n" +
		"FROM moves child\n" +
		"JOIN moves parent ON parent.character_id = child.character_id\n" +
		"JOIN characters c ON c.id = child.character_id\n" +
		"WHERE " + charFilter("c", "kimberly") + "\n" +
		"  AND (child.code, parent.code) IN (VALUES\n" + values + "\n      )"
	if dir == "up" {
		b.WriteString("INSERT INTO move_derivations (child_move_id, parent_move_id)\n")
		b.WriteString(sel + "\n")
		b.WriteString("  AND NOT EXISTS (SELECT 1 FROM move_derivations d\n" +
			"      WHERE d.child_move_id = child.id AND d.parent_move_id = parent.id);\n\n")
	} else {
		b.WriteString("DELETE FROM move_derivations WHERE (child_move_id, parent_move_id) IN (\n")
		b.WriteString(sel + ");\n\n")
	}
	return b.String()
}
