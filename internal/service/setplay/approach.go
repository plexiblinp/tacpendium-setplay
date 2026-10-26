package setplay

import (
	"github.com/plexiblinp/tacpendium-setplay/internal/model"
	setplayrepo "github.com/plexiblinp/tacpendium-setplay/internal/repository/setplay"
)

// characterApproachMoves はキャラ固有の接近技の辞書である(M47-05・CHANGE-254 a)。
// キーはキャラの code(characters.code。例 "ken")、値はそのキャラの技の code(moves.code)の一覧。
//
// 提案の絞り込み「接近を含む提案だけ」は、共通の接近の手(前ステップ・前ジャンプ・ラッシュから
// 出す技＝isApproachMove)に加えて、ここに載っている技も接近の手として数える。
//
// # 足し方(開発者向け)
//
// キャラ 1 人を 1 行で足す。既にキャラの行があれば、その一覧へ code を足す(例は下)。
//
//   - code は character_data/<キャラ>.csv の move_code 列と同じ綴りで書く。
//   - 書き間違えた code は、internal/infra/migration/canary_setplay_approach_test.go の
//     TestCanary_ApproachMovesExistInSeed が赤で知らせる(辞書の全 code が moves に在ることを見る)。
//   - 一覧の無いキャラは、共通の接近の手だけで判定する(一部のキャラだけ書けばよい)。
//
// 書き方の例(★例であり、開発者の一覧ではない):
//
//	var characterApproachMoves = map[string][]string{
//		"ken": {"quick_dash"},
//		"aki": {"snake_step_light", "sinister_slide"},
//	}
//
// ★中身は開発者が書いた一覧である(指示書 §9.4-1。2026-09-28 に開発者が技名で渡し、製造が
// character_data/*.csv の name_ja と突き合わせて move_code へ直した)。各キャラの行の上のコメントが
// 開発者の一覧の要旨である。技名から code への直し方で製造が決めた点(強度の範囲・状態違いの版・派生)は
// M47-05 完了報告の追補に一覧がある。
// ★全体フレーム(total)が空欄の技・単独で出せない派生は繋ぎにならないため、載っていても提案の行には
// 現れない(当てる技になる場合を除く)。
// ★moves に「前へ進む技か」の列を持たせるかは、一覧を書いてみてから開発者が決める(D-1007)。
//
// テストで差し替えるため var とする。
var characterApproachMoves = map[string][]string{
	// 全強度 蛇軽功 ／ 悪鬼蛇行 ／ 猛毒牙 ／ 蛇連咬
	"aki": {"snake_step_light", "snake_step_medium", "snake_step_heavy", "snake_step_od", "snake_step_side_switch_heavy", "snake_step_side_switch_od", "sinister_slide", "venomous_fang", "heel_strike"},
	// 天魔空刃脚 ／ 全強度 斬空波動拳 ／ 弱中強 竜巻斬空脚(OD は除く＝開発者の指定 2026-09-28) ／ 全強度 百鬼襲とその派生(百鬼豪螺旋は除く) ／ 阿修羅閃空(前方) ／ 朧 ／ 瞬獄殺
	"akuma": {"tenmaku_blade_kick", "zanku_hadoken_light", "zanku_hadoken_medium", "zanku_hadoken_heavy", "zanku_hadoken_od", "tatsumaki_zanku_kyaku_light", "tatsumaki_zanku_kyaku_medium", "tatsumaki_zanku_kyaku_heavy", "demon_raid_light", "demon_raid_medium", "demon_raid_heavy", "demon_raid_od", "demon_low_slash", "demon_low_slash_od", "demon_guillotine", "demon_guillotine_od", "demon_guillotine_side_switch", "demon_blade_kick", "demon_blade_kick_od", "demon_blade_kick_side_switch", "demon_swoop", "demon_swoop_od", "demon_swoop_side_switch", "demon_swoop_side_switch_od", "demon_gou_zanku", "ashura_senku_forward", "oboro_throw", "ca_shun_goku_satsu"},
	// ステップイン ／ 全強度 スラッシュエルボー ／ タクティカルリープ ／ エアスタンピート ／ デンジャラスアプローチ ／ フライングクロスチョップ ／ 弱中 フラッシュアックス
	"alex": {"low_rush", "light_slashing_elbow", "medium_slashing_elbow", "heavy_slashing_elbow", "tactical_hop", "air_stampede", "dangerous_armbar", "flying_cross_chop", "flash_axe_light", "flash_axe_medium"},
	// しゃがみ強K ／ 全強度 大義の巨腕 ／ 仁義の一刀 ／ 全強度 厳酷な岩肩とその派生の過剰な正義(追撃は除く＝開発者の選択 2026-10-17)
	"arjun": {"crouching_heavy_kick", "blade_of_duty", "arm_of_justice_light", "arm_of_justice_medium", "arm_of_justice_heavy", "arm_of_justice_od", "cruel_shoulder_light", "cruel_shoulder_medium", "cruel_shoulder_heavy", "cruel_shoulder_od", "justice_beyond_light", "justice_beyond_medium", "justice_beyond_heavy", "justice_beyond_od"},
	// アマゾンリバーラン ／ サプライズフォワード ／ 全強度 ローリングアタック ・バックステップローリング ・エリアルローリング(【ライトニングビースト】版を含む)
	"blanka": {"amazon_river_run", "surprise_forward_hop", "rolling_attack_light", "rolling_attack_medium", "rolling_attack_heavy", "rolling_attack_od", "lightning_beast_light_rolling_attack", "lightning_beast_medium_rolling_attack", "lightning_beast_heavy_rolling_attack", "lightning_beast_od_rolling_attack", "backstep_rolling_attack_light", "backstep_rolling_attack_medium", "backstep_rolling_attack_heavy", "backstep_rolling_attack_od", "lightning_beast_light_backstep_rolling_attack", "lightning_beast_medium_backstep_rolling_attack", "lightning_beast_heavy_backstep_rolling_attack", "lightning_beast_od_backstep_rolling_attack", "aerial_rolling_attack_light", "aerial_rolling_attack_medium", "aerial_rolling_attack_heavy", "aerial_rolling_attack_od", "lightning_beast_light_aerial_rolling_attack", "lightning_beast_medium_aerial_rolling_attack", "lightning_beast_heavy_aerial_rolling_attack", "lightning_beast_od_aerial_rolling_attack"},
	// 全強度 バーニングキック ・空中バーニングキック(【ハイジャンプ】版を含む) ／ セービングフォースの前方ステップ ／ 弱中 サンダースラップ
	"c_viper": {"burning_kick_light", "burning_kick_medium", "burning_kick_heavy", "burning_kick_od", "aerial_burning_kick_light", "aerial_burning_kick_medium", "aerial_burning_kick_heavy", "aerial_burning_kick_od", "high_jump_light_aerial_burning_kick", "high_jump_medium_aerial_burning_kick", "high_jump_heavy_aerial_burning_kick", "high_jump_od_aerial_burning_kick", "focus_force_forward_step", "focus_force_forward_step_od", "thunder_dash_light", "thunder_dash_medium"},
	// キャノンスパイク以外の必殺技は全て
	"cammy": {"spiral_arrow_light", "spiral_arrow_medium", "spiral_arrow_heavy", "spiral_arrow_holding_heavy", "spiral_arrow_od", "quick_spin_knuckle_light", "quick_spin_knuckle_medium", "quick_spin_knuckle_heavy", "quick_spin_knuckle_od", "cannon_strike_light", "cannon_strike_medium", "cannon_strike_heavy", "cannon_strike_od", "hooligan_combination_light", "hooligan_combination_medium", "hooligan_combination_heavy", "hooligan_combination_holding_heavy", "hooligan_combination_od", "razors_edge_slicer", "razors_edge_slicer_od", "razors_edge_slicer_holding", "cannon_strike_light_hooligan_combination", "cannon_strike_medium_hooligan_combination", "cannon_strike_heavy_hooligan_combination", "cannon_strike_heavy_hooligan_combination_holding", "cannon_strike_light_od_hooligan_combination_od", "cannon_strike_medium_od_hooligan_combination_od", "cannon_strike_heavy_od_hooligan_combination_od", "reverse_edge_light_hooligan_combination", "reverse_edge_medium_hooligan_combination", "reverse_edge_heavy_hooligan_combination", "reverse_edge_heavy_hooligan_combination_holding", "reverse_edge_od", "fatal_leg_twister_light", "fatal_leg_twister_medium", "fatal_leg_twister_heavy", "fatal_leg_twister_heavy_hooligan_combination_holding", "fatal_leg_twister_light_od_hooligan_combination", "fatal_leg_twister_medium_od_hooligan_combination", "fatal_leg_twister_heavy_od_hooligan_combination", "silent_step_light", "silent_step_medium", "silent_step_heavy", "silent_step_light_od_hooligan_combination_od", "silent_step_medium_od_hooligan_combination_od", "silent_step_heavy_od_hooligan_combination_od"},
	// 弱中強 スピニングバードキック ／ 全強度 覇山蹴
	"chun_li": {"spinning_bird_kick_light", "spinning_bird_kick_medium", "spinning_bird_kick_heavy", "hazanshu_light", "hazanshu_medium", "hazanshu_heavy", "hazanshu_od"},
	// しゃがみ強K(開発者の訂正 2026-09-28: 立ち強K ではない)
	"dee_jay": {"crouching_heavy_kick"},
	// 弱中強 ドリルキック ／ ヨガテレポート(前方。P ・K の地上版)
	"dhalsim": {"drill_kick", "medium_drill_kick", "heavy_drill_kick", "yoga_teleport", "k_yoga_teleport_forward"},
	// 全強度 スーパー頭突き ・スーパー百貫落とし ・相撲ステップ
	"e_honda": {"sumo_headbutt_light", "sumo_headbutt_medium", "sumo_headbutt_heavy", "sumo_headbutt_od", "sumo_smash_light", "sumo_smash_medium", "sumo_smash_heavy", "sumo_smash_od", "sumo_dash", "sumo_dash_od"},
	// キルステップ(前方) ／ キルスイッチ 2 種 ／ 前強P(コブラパンチ)
	"ed": {"kill_rush_forward", "kill_switch_break", "kill_switch_chaser", "cobra_punch"},
	// スライディング ／ 全強度 ライノホーン ・リンクシング ・リンクスワール(スピンサイズ派生を含む)
	"elena": {"slide", "rhino_horn_light", "rhino_horn_medium", "rhino_horn_heavy", "rhino_horn_od", "lynx_song_light", "lynx_song_medium", "lynx_song_heavy", "lynx_song_od", "lynx_whirl_light", "lynx_whirl_medium", "lynx_whirl_heavy", "lynx_whirl_od", "lynx_whirl_spinning_scythe_light", "lynx_whirl_spinning_scythe_medium", "lynx_whirl_spinning_scythe_heavy", "lynx_whirl_od_spinning_scythe_light", "lynx_whirl_od_spinning_scythe_medium", "lynx_whirl_od_spinning_scythe_heavy"},
	// ニーバズーカ
	"guile": {"knee_bazooka"},
	// 全強度 ソーラーフレア(前方) ／ 中強OD サンライズ
	"ingrid": {"solar_burst_light_forward", "solar_burst_lv1_forward", "solar_burst_lv2_forward", "solar_burst_lv3_forward", "solar_burst_lv1_forward_od", "solar_burst_lv2_forward_od", "solar_burst_lv3_forward_od", "sun_rise_medium", "sun_rise_heavy", "sun_rise_od"},
	// 全強度 酔疾歩 ・無影蹴
	"jamie": {"swagger_step_light", "swagger_step_medium", "swagger_step_heavy", "swagger_step_od", "luminous_dive_kick_light", "luminous_dive_kick_medium", "luminous_dive_kick_heavy", "luminous_dive_kick_od"},
	// ヴィーハト・アクノ(OD 二連続ワープを含む)
	"jp": {"departure_window", "departure_window_double_warp_od"},
	// 疾空閃 ／ OD疾空閃 ／ SA2 突進版
	"juri": {"shiku_sen", "shiku_sen_od", "sa2_feng_shui_engine_dash"},
	// 奮迅脚 ／ 急停止 ／ 紫電カカト落とし ／ 踏み込み前蹴り ／ 全強度 竜巻旋風脚 ・龍尾脚
	"ken": {"quick_dash", "emergency_stop", "thunder_kick", "forward_step_kick", "tatsumaki_senpu_kyaku_light", "tatsumaki_senpu_kyaku_medium", "tatsumaki_senpu_kyaku_heavy", "tatsumaki_senpu_kyaku_od", "dragonlash_kick_light", "dragonlash_kick_medium", "dragonlash_kick_heavy", "dragonlash_kick_od"},
	// 飛箭蹴 ／ 疾駆け ／ OD疾駆け ／ 急停止 ・胴刎ね ・影すくい ・首狩り ・弧空(各 OD を含む)
	"kimberly": {"hisen_kick", "sprint", "sprint_od", "emergency_stop", "emergency_stop_od", "torso_cleaver", "torso_cleaver_od", "shadow_slide", "shadow_slide_od", "neck_hunter", "neck_hunter_od", "arc_step", "arc_step_od"},
	// 全強度 コンドルスパイア ・コンドルダイブ([風纏い]版・派生を含む)
	"lily": {"condor_spire_light", "condor_spire_medium", "condor_spire_heavy", "condor_spire_od", "windclad_light_condor_spire", "windclad_medium_condor_spire", "windclad_heavy_condor_spire", "windclad_od_condor_spire", "condor_dive", "condor_dive_od", "windclad_condor_dive", "windclad_od_condor_dive", "condor_dive_follow_up", "windclad_od_condor_dive_follow_up"},
	// 強フラッシュナックル(【ホールド】【ジャスト】を含む) ／ アベンジャー ・ノーチェイサー ・インパラー(各 OD を含む)
	"luke": {"flash_knuckle_heavy", "flash_knuckle_holding_heavy", "flash_knuckle_perfect_heavy", "avenger", "avenger_od", "no_chaser", "no_chaser_od", "impaler", "impaler_od"},
	// ホバーキック ／ 全強度 サイコクラッシャーアタック([サイコマイン付着中]版を含む) ・ダブルニープレス ・シャドウライズとその派生
	"m_bison": {"hover_kick", "psycho_crusher_attack_light", "psycho_crusher_attack_medium", "psycho_crusher_attack_heavy", "psycho_crusher_attack_od", "mine_set_light_psycho_crusher_attack", "mine_set_medium_psycho_crusher_attack", "mine_set_heavy_psycho_crusher_attack", "mine_set_od_psycho_crusher_attack", "double_knee_press_light", "double_knee_press_medium", "double_knee_press_heavy", "double_knee_press_od", "shadow_rise_light", "shadow_rise_medium", "shadow_rise_heavy", "shadow_rise_od", "devil_reverse", "devil_reverse_od", "head_press", "head_press_od"},
	// 全強度 必殺忍蜂 ・ムササビの舞([焔版]を含む)
	"mai": {"hissatsu_shinobi_bachi_light", "hissatsu_shinobi_bachi_medium", "hissatsu_shinobi_bachi_heavy", "hissatsu_shinobi_bachi_od", "flame_light_hissatsu_shinobi_bachi", "flame_medium_hissatsu_shinobi_bachi", "flame_heavy_hissatsu_shinobi_bachi", "flame_od_hissatsu_shinobi_bachi", "musasabi_no_mai", "musasabi_no_mai_od", "flame_musasabi_no_mai", "flame_od_musasabi_no_mai"},
	// 全強度 デガジェ ・ランヴェルセ(フェイントを含む)
	"manon": {"degage_light", "degage_medium", "degage_heavy", "degage_od", "renverse_light", "renverse_medium", "renverse_heavy", "renverse_od", "renverse_feint_light", "renverse_feint_medium", "renverse_feint_heavy", "renverse_feint_od"},
	// 全強度 ファランクス ・クアドリガ
	"marisa": {"phalanx_light", "phalanx_medium", "phalanx_heavy", "phalanx_od", "quadriga_light", "quadriga_medium", "quadriga_heavy", "quadriga_od"},
	// テンペスト・ムーン ／ サイドフリップ ／ フロントフリップ ／ 弱スピニング・ミキサー(強化版を含む) ／ 全強度 イーグル・スパイク ／ ウイング・ストローク ／ アサルト・ロール ／ アサルト・ネイル ／ アラビアン・スカイハイ
	"rashid": {"tempest_moon", "buffed_tempest_moon", "side_flip", "front_flip", "spinning_mixer_light", "buffed_light_spinning_mixer", "eagle_spike_light", "eagle_spike_medium", "eagle_spike_heavy", "eagle_spike_od", "buffed_light_eagle_spike", "buffed_medium_eagle_spike", "buffed_heavy_eagle_spike", "buffed_od_eagle_spike", "wing_stroke", "rolling_assault", "nail_assault", "arabian_skyhigh_light", "arabian_skyhigh_medium", "arabian_skyhigh_heavy", "arabian_skyhigh_od"},
	// 弱中強 竜巻旋風脚
	"ryu": {"tatsumaki_senpu_kyaku_light", "tatsumaki_senpu_kyaku_medium", "tatsumaki_senpu_kyaku_heavy"},
	// 全強度 タイガーニークラッシュ ／ ステップローキック ／ ステップハイキック
	"sagat": {"tiger_knee_crush_light", "tiger_knee_crush_medium", "tiger_knee_crush_heavy", "tiger_knee_crush_od", "low_step_kick", "high_step_kick"},
	// 中強OD バーンナックル ／ 全強度 クラックシュート ・ライジングタックル
	"terry": {"burning_knuckle_medium", "burning_knuckle_heavy", "burning_knuckle_od", "crack_shoot_light", "crack_shoot_medium", "crack_shoot_heavy", "crack_shoot_od", "rising_tackle_light", "rising_tackle_medium", "rising_tackle_heavy", "rising_tackle_od"},
	// 全強度 タリム・ン・ハンギン ・ムカ・ン・ランギット ・ウラン ・クロッグ
	"yasmine": {"talim_ng_hangin_light", "talim_ng_hangin_medium", "talim_ng_hangin_heavy", "talim_ng_hangin_od", "mukha_ng_langit_light", "mukha_ng_langit_medium", "mukha_ng_langit_heavy", "mukha_ng_langit_od", "ulan_light", "ulan_medium", "ulan_heavy", "ulan_od", "kulog_light", "kulog_medium", "kulog_heavy", "kulog_od"},
	// ニーバット ／ サイクロンニールキック
	"zangief": {"knee_hammer", "cyclone_wheel_kick"},
}

// approachCodesFor はキャラ code の固有の接近技を集合で返す。一覧が無ければ空の集合。
func approachCodesFor(charCode string) map[string]bool {
	codes := characterApproachMoves[charCode]
	out := make(map[string]bool, len(codes))
	for _, c := range codes {
		out[c] = true
	}
	return out
}

// isApproachMove は、その技が接近の手かを返す(M47-05)。
//
// 接近の手＝前ステップ(dash_forward)／前ジャンプ(jump_forward)／ラッシュから出す技
// (category＝rush_variant)／そのキャラの固有の接近技(extra)。開発者の回答＝D-1005 ・D-1007。
//
// ★前歩き・後ろステップ・後ろジャンプ・垂直ジャンプ・ドライブパリィ等は接近に数えない。
// ただし繋ぎの候補からは外さない(D-1007＝繋ぎは今のまま)。
// ★ラッシュの技は code の接頭辞 "rush_" ではなく category で見る(rush_variant の行は
// code も rush_<元技> だが、判定の根拠は category である)。
func isApproachMove(m setplayrepo.MoveCandidate, extra map[string]bool) bool {
	switch {
	case m.Code == model.MoveCodeDashForward, m.Code == model.MoveCodeJumpForward:
		return true
	case m.Category == model.MoveCategoryRushVariant:
		return true
	default:
		return extra[m.Code]
	}
}

// CharacterApproachMoves は characterApproachMoves の写しを返す(検査用。M47-05)。
// 辞書の code が moves に在ることを実 DB で確かめるテスト(internal/infra/migration の
// TestCanary_ApproachMovesExistInSeed)が使う。写しを返すため、呼び出し側が変えても辞書は変わらない。
func CharacterApproachMoves() map[string][]string {
	out := make(map[string][]string, len(characterApproachMoves))
	for ch, codes := range characterApproachMoves {
		out[ch] = append([]string(nil), codes...)
	}
	return out
}
