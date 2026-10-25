package seedgen

import (
	"strings"
	"testing"
)

// M20-06 生成器の単体テスト。層 C-3(何も押さずに派生する技・P-34)の規則を固定する。
//
// ★実データとの突合(13 件の name_ja が CSV と一致するか)は
// internal/infra/migration/migrate_m2006_test.go が実 DB で持つ。本ファイルは
// 「規則が何を返すか」を小さな入力で固定する。

// TestM2006_HumanizeMoveCode は §4.2 の規則 2 を固定する。
//
//	_ を半角スペースへ / トークン od は OD へ / 他のトークンは小文字のまま
//
// ★意訳しない。move_code に無い語を足した時点で新しい情報である(D-289)。
func TestM2006_HumanizeMoveCode(t *testing.T) {
	for _, tc := range []struct {
		moveCode, want, why string
	}{
		{"arc_step", "arc step", "od を含まない基本形"},
		{"scutum_counterattack", "scutum counterattack", "同上(2 トークン)"},
		{"psycho_mine_auto_detonation", "psycho mine auto detonation", "4 トークン"},
		{"buffed_jump_neutral", "buffed jump neutral", "第 3 枠も同じ規則"},

		// ★od → OD は末尾 3 件 + 語中 1 件の計 4 か所ある。同じ規則で処理されること。
		{"arc_step_od", "arc step OD", "末尾の od"},
		{"departure_shadow_od", "departure shadow OD", "末尾の od"},
		{"scutum_counterattack_od", "scutum counterattack OD", "末尾の od"},
		{"windclad_od_condor_dive_follow_up", "windclad OD condor dive follow up", "★語中の od も同じ規則"},

		// ★od の判定はトークン完全一致である。部分一致にすると下記が壊れる。
		{"condor_dive_follow_up", "condor dive follow up", "★condor の od を大文字化しない"},
		{"product_code", "product code", "★product の od を大文字化しない"},
		{"odd_step", "odd step", "★odd は od トークンではない"},
	} {
		if got := humanizeMoveCode(tc.moveCode); got != tc.want {
			t.Errorf("humanizeMoveCode(%q) = %q, want %q(%s)", tc.moveCode, got, tc.want, tc.why)
		}
	}
}

// TestM2006_HumanizeAddsNoNewWords は「新しい情報を作らない」を機械で守る。
//
// ★これが規則の本体である(§4.2)。整形結果のトークン列は、move_code のトークン列を
// 大文字小文字だけ変えたものでなければならない。語を足す・落とす・並べ替えるのは意訳である。
func TestM2006_HumanizeAddsNoNewWords(t *testing.T) {
	for charCode, codes := range noInputDerivedTargets {
		for moveCode := range codes {
			src := strings.Split(moveCode, "_")
			got := strings.Split(humanizeMoveCode(moveCode), " ")
			if len(src) != len(got) {
				t.Errorf("%s/%s: トークン数 %d → %d(語の増減がある)", charCode, moveCode, len(src), len(got))
				continue
			}
			for i := range src {
				if !strings.EqualFold(src[i], got[i]) {
					t.Errorf("%s/%s: トークン %d = %q, want %q を大文字小文字だけ変えた形",
						charCode, moveCode, i, got[i], src[i])
				}
			}
		}
	}
}

// TestM2006_TargetSetIsEighteen は対象集合が 22 件であることを固定する(関数名は 18 件の時代のまま)。
//
// ★内訳は 既 seed 19 キャラの 13 件(D-367)＋ 第四波の 5 件(M14-03f・2026-09-02 開発者判断)
// ＋ アルジュンの 4 件(2026-10-17 開発者判断)。
// ★19 キャラ側は 27 件のうち 13 件だけが値の決まった集合である。とくに
// jp/departure_window_double_warp_od は「エッジケースでありフォールバックでよい」として
// 意図的に外してある。★第四波側は 11 件のうち 5 件である(外した 6 件は下で対象外を主張する)。
//
// ★本テストは「足した瞬間に赤くなる」ためのものである。増やすときは、どの判断で
// 増やしたのかを noInputDerivedTargets の注記へ書いてから、ここの数を動かすこと。
func TestM2006_TargetSetIsEighteen(t *testing.T) {
	if got := noInputDerivedTargetCount(); got != 22 {
		t.Errorf("層 C-3 の対象 move_code = %d 件, want 22(D-367 の 13 件 + M14-03f の 5 件 + アルジュンの 4 件)", got)
	}
	if isNoInputDerivedTarget("jp", "departure_window_double_warp_od") {
		t.Error("jp/departure_window_double_warp_od が対象に入っている(D-367 で対象外)")
	}
	// 値が未決のまま残る代表例も対象外であること。
	for _, tc := range []struct{ charCode, moveCode string }{
		{"guile", "sonic_break"},
		{"manon", "renverse_feint_light"},
		{"marisa", "tonitrus_od"},
		{"rashid", "wall_jump"},
	} {
		if isNoInputDerivedTarget(tc.charCode, tc.moveCode) {
			t.Errorf("%s/%s が対象に入っている(P-34 で値が未決)", tc.charCode, tc.moveCode)
		}
	}

	// ★第四波で足した 5 件(M14-03f)。
	for _, tc := range []struct{ charCode, moveCode string }{
		{"cammy", "razors_edge_slicer"},
		{"cammy", "razors_edge_slicer_od"},
		{"cammy", "razors_edge_slicer_holding"},
		{"akuma", "demon_low_slash"},
		{"akuma", "demon_low_slash_od"},
	} {
		if !isNoInputDerivedTarget(tc.charCode, tc.moveCode) {
			t.Errorf("%s/%s が対象に入っていない(M14-03f で足した 5 件)", tc.charCode, tc.moveCode)
		}
	}

	// ★アルジュンで足した 4 件(2026-10-17)。
	for _, code := range []string{"justice_beyond_light", "justice_beyond_medium", "justice_beyond_heavy", "justice_beyond_od"} {
		if !isNoInputDerivedTarget("arjun", code) {
			t.Errorf("arjun/%s が対象に入っていない(2026-10-17 に足した 4 件)", code)
		}
	}

	// ★第四波で「同分類だが外した」6 件。command 空欄 かつ is_derived=true という点では
	//   上の 5 件と見分けが付かない。⇒ 足した瞬間にここが赤くなる。
	for _, tc := range []struct{ charCode, moveCode, why string }{
		{"akuma", "gou_hadoken_max_holding_light", "ボタンを押し続けている"},
		{"akuma", "gou_hadoken_max_holding_medium", "同上"},
		{"akuma", "gou_hadoken_max_holding_heavy", "同上"},
		{"aki", "sinister_slide_cancel", "上入力が要る"},
		{"chun_li", "serenity_stream_cancel", "上入力が要る"},
		{"alex", "prowler_stance_cancel", "解除操作・入力の記述が無い"},
	} {
		if isNoInputDerivedTarget(tc.charCode, tc.moveCode) {
			t.Errorf("%s/%s が対象に入っている(M14-03f で外した: %s)", tc.charCode, tc.moveCode, tc.why)
		}
	}
}

// TestM2006_NoInputDerivedIsFilled は層 C-3 が値を作る経路を固定する。
//
// alias_text = name_ja の複製 / alias_text_en = move_code の整形。
// ★対象外の「command 空欄の派生」は従来どおり derived-no-command のまま残る。
func TestM2006_NoInputDerivedIsFilled(t *testing.T) {
	const char = "kimberly"
	rows := []MoveRow{
		mkRow(char, "arc_step", "special", "弧空", "", true),
		mkRow(char, "arc_step_od", "special", "OD弧空", "", true),
		// 対象外(P-34 で値が未決)。同じ形なのに投入されないことを対で固定する。
		mkRow(char, "some_other_follow_up", "special", "別の派生", "", true),
	}
	res := genOne(t, NumericRule, char, rows)

	for _, tc := range []struct{ moveCode, text, textEn string }{
		{"arc_step", "弧空", "arc step"},
		{"arc_step_od", "OD弧空", "arc step OD"},
	} {
		row, ok := aliasOf(res, tc.moveCode)
		if !ok {
			t.Errorf("%s: 投入されていない", tc.moveCode)
			continue
		}
		if row.Layer != LayerCNoInput {
			t.Errorf("%s: layer = %s, want %s", tc.moveCode, row.Layer, LayerCNoInput)
		}
		if row.AliasText != tc.text {
			t.Errorf("%s: alias_text = %q, want %q(name_ja の複製)", tc.moveCode, row.AliasText, tc.text)
		}
		if row.AliasTextEn != tc.textEn {
			t.Errorf("%s: alias_text_en = %q, want %q", tc.moveCode, row.AliasTextEn, tc.textEn)
		}
	}

	u, ok := unfilledOf(res, "some_other_follow_up")
	if !ok {
		t.Fatal("some_other_follow_up: 投入しない行として記録されていない")
	}
	if u.Reason != ReasonDerivedNoCommand {
		t.Errorf("some_other_follow_up: reason = %s, want %s", u.Reason, ReasonDerivedNoCommand)
	}
}

// TestM2006_AliasTextEnAppearsInSQL は生成 SQL が alias_text_en を出すことを固定する。
//
// ★層 C-3 が入るまで、キャラ別の塊は 1 行も英語表記を持たなかった(値を持つのは移動系の
// 2 code だけ)。列を出し忘れると、値は生成結果に載るのに DB へ入らない——
// ★行数も表示も変わらないため、テストが無いと気づけない。
func TestM2006_AliasTextEnAppearsInSQL(t *testing.T) {
	const char = "kimberly"
	res := genOne(t, NumericRule, char, []MoveRow{
		mkRow(char, "arc_step", "special", "弧空", "", true),
	})
	if !strings.Contains(res.UpSQL, "alias_text_en") {
		t.Error("up SQL に alias_text_en 列が無い(値が DB へ入らない)")
	}
	if !strings.Contains(res.UpSQL, "'arc step' AS alias_text_en") {
		t.Errorf("up SQL に整形後の英語表記が無い:\n%s", res.UpSQL)
	}

	// ★英語表記を持たない塊では列を出さない(既存の全キャラ分へ NULL だけの列を増やさない)。
	//   ★移動系(層 C-1)は micro_forward / micro_back のために常に同列を出すので、
	//     キャラ別の塊だけを切り出して見る。
	res2 := genOne(t, NumericRule, char, []MoveRow{
		mkRow(char, "standing_light_punch", "normal", "立ち弱P", "p_l", false),
	})
	if block := charBlockOf(t, res2.UpSQL, char); strings.Contains(block, "alias_text_en") {
		t.Errorf("英語表記を持たない塊に alias_text_en 列が出ている:\n%s", block)
	}
	// 対照——層 C-3 を含む塊では出ていること(上の主張が空振りでないことの担保)。
	if block := charBlockOf(t, res.UpSQL, char); !strings.Contains(block, "alias_text_en") {
		t.Errorf("層 C-3 を含む塊に alias_text_en 列が無い:\n%s", block)
	}
}

// charBlockOf は生成 SQL から当該キャラの INSERT 文だけを切り出す
// (移動系 9 code の塊を混ぜずに主張するため)。
func charBlockOf(t *testing.T, sql, charCode string) string {
	t.Helper()
	marker := "-- ===== " + charCode + " ("
	i := strings.Index(sql, marker)
	if i < 0 {
		t.Fatalf("キャラ %s の INSERT 塊が見つからない:\n%s", charCode, sql)
	}
	return sql[i:]
}

// TestM2006_RuleReappliesOnNewSeedWave は §5 (g)——再適用責務——を固定する。
//
// ★層 C-3 は「1 回きりの backfill」ではなく「規則」である(D-181)。seed 波ごとに
// -mode aliases を回す既存手順(character_data/seed-progress.md)がそのまま再適用経路に
// なっていること、すなわち -noinput-only を付けない通常の生成でも層 C-3 が出ることを見る。
//
// ★ここが壊れる形は「-noinput-only のときだけ層 C-3 を当てる」実装である。
// その形だと次のキャラ波で層 C-3 が黙って落ち、当該技だけ日本語技名のまま残る
// (画面は壊れずテストも緑のまま = DES-004 §5.4 の症状)。
func TestM2006_RuleReappliesOnNewSeedWave(t *testing.T) {
	const char = "kimberly"
	rows := []MoveRow{
		mkRow(char, "standing_light_punch", "normal", "立ち弱P", "p_l", false),
		mkRow(char, "arc_step", "special", "弧空", "", true),
	}

	// 通常の -mode aliases(全層)でも層 C-3 が出ること。
	full := genOne(t, NumericRule, char, rows)
	row, ok := aliasOf(full, "arc_step")
	if !ok || row.Layer != LayerCNoInput {
		t.Fatalf("通常生成で層 C-3 が出ていない(1 回きりの backfill になっている): %+v", full.Rows)
	}
	if !strings.Contains(full.UpSQL, "'arc_step'") {
		t.Error("通常生成の SQL に層 C-3 の行が無い")
	}
	// 同じ塊に層 B の行も出ていること(層 C-3 が他層を押しのけていない)。
	if !strings.Contains(full.UpSQL, "'standing_light_punch'") {
		t.Error("通常生成の SQL から層 B の行が消えている")
	}
}

// TestM2006_NoInputOnlyEmitsOnlyLayerC3 は -noinput-only の出力範囲を固定する。
//
// ★衝突判定は全層で回し、絞るのは出力だけであること。層 C-3 だけを解決すると
// 衝突判定の母数から他層の表記が落ち、UNIQUE(preset_id, character_id, alias_text) に
// 触れる行を「衝突なし」と誤判定する。
//
// ★down が層 C-3 の move_code だけを消すことも見る。既存の -mode aliases をそのまま
// 再実行すると、up は NOT EXISTS で skip する一方 down は無条件 DELETE であるため、
// 前の波が入れた行まで消す down ができる(M20-02 レビュー H-2 と同型)。
func TestM2006_NoInputOnlyEmitsOnlyLayerC3(t *testing.T) {
	const char = "kimberly"
	rows := []MoveRow{
		mkRow(char, "standing_light_punch", "normal", "立ち弱P", "p_l", false),
		mkRow(char, "crouching_heavy_kick", "normal", "しゃがみ強K", "d plus k_h", false),
		mkRow(char, "arc_step", "special", "弧空", "", true),
	}
	res, err := GenerateAliases(NumericRule, []string{char}, map[string][]MoveRow{char: rows},
		[]string{char}, AliasHeader("t", "t", WithNoInputOnly()), WithNoInputOnly())
	if err != nil {
		t.Fatalf("GenerateAliases: %v", err)
	}

	// (1) 解決は全層で回っている(衝突判定の母数が保たれている)。
	if len(res.Rows) != 3 {
		t.Errorf("解決した行 = %d, want 3(層 C-3 だけを解決している)", len(res.Rows))
	}
	// (2) SQL へ出たのは層 C-3 だけ。
	if len(res.Emitted) != 1 || res.Emitted[0].MoveCode != "arc_step" {
		t.Errorf("SQL へ出した行 = %+v, want arc_step のみ", res.Emitted)
	}
	for _, code := range []string{"standing_light_punch", "crouching_heavy_kick"} {
		if strings.Contains(res.UpSQL, "'"+code+"'") {
			t.Errorf("up SQL に層 C-3 以外の行 %s が出ている", code)
		}
		if strings.Contains(res.DownSQL, "'"+code+"'") {
			t.Errorf("★down SQL に層 C-3 以外の行 %s が出ている(既存の投入分を消す)", code)
		}
	}
	// (3) 移動系 9 code のブロックを出さない(同じ理由で down が前の投入分を消す)。
	if strings.Contains(res.UpSQL, "層 C-1") || strings.Contains(res.DownSQL, "層 C-1") {
		t.Error("★-noinput-only なのに移動系 9 code のブロックが出ている")
	}
	// (4) 再適用ガードは残っていること(冪等性)。
	if !strings.Contains(res.UpSQL, "NOT EXISTS") {
		t.Error("up SQL に NOT EXISTS ガードが無い(再適用で落ちる)")
	}
}

// TestM2006_NoInputDerivedJoinsCollisionPool は層 C-3 が衝突判定の母数に入ることを固定する。
//
// ★入れ忘れると、name_ja が同一キャラの別技の表記と一致したときに両方投入され、
// UNIQUE(preset_id, character_id, alias_text) でマイグレ適用が落ちる——
// ★生成時ではなく適用時に落ちるため、原因が遠い。
//
// ★実データでは衝突 0 件である(13 件の name_ja は numeric / srk の表記と交わらない)。
// ここは「起きたときに検出できる」ことの担保である。
//
// ★層 C-3 同士の衝突は本経路では起きない——同一キャラ内で name_ja が重複する CSV は
// csv.go の validate が先に fail させる(「別の機構が代わりに守っている」)。
// ⇒ 層 C-3 に固有の危険は「name_ja が同キャラの別技の *表記* と一致する」形であり、
// これは name_ja の重複ではないため validate を素通りする。ここで見るのはその形である。
func TestM2006_NoInputDerivedJoinsCollisionPool(t *testing.T) {
	const char = "kimberly"
	// ★層 B が作る表記(5LP)と、層 C-3 の name_ja がぶつかる形。
	//   name_ja としては重複していないので CSV の validate は通る。
	rows := []MoveRow{
		mkRow(char, "standing_light_punch", "normal", "立ち弱P", "p_l", false),
		mkRow(char, "arc_step", "special", "5LP", "", true),
	}
	rows[1].RowIndex = 1

	// ★M48-08 以降(FormatCurrent)は優先の 1 つだけ残る。層 C-3 の行は旧規則でも作っていた行
	//   であり、派生技の層 A の行と違って後ろへ回さない ⇒ CSV の行順で先の standing_light_punch が残る。
	//   ★母数に入っていること(本テストの主張)は、arc_step が collision として落ちることで示される。
	cur := genOne(t, NumericRule, char, rows)
	if got, ok := aliasOf(cur, "standing_light_punch"); !ok || got.AliasText != "5LP" {
		t.Errorf("FormatCurrent: standing_light_punch が残っていない(%+v)", cur.Rows)
	}
	if u, ok := unfilledOf(cur, "arc_step"); !ok || u.Reason != ReasonCollision {
		t.Errorf("FormatCurrent: arc_step が衝突として記録されていない(%+v)", u)
	}

	// ★旧規則(FormatPreM4808)では両方落ちる(当時の §9.3-5)。
	res := genOne(t, NumericRule, char, rows, WithFormat(FormatPreM4808))
	if len(res.Rows) != 0 {
		t.Errorf("衝突した行が投入された = %+v, want 両方落ちる", res.Rows)
	}
	for _, code := range []string{"standing_light_punch", "arc_step"} {
		u, ok := unfilledOf(res, code)
		if !ok || u.Reason != ReasonCollision {
			t.Errorf("%s: 衝突として記録されていない(%+v)", code, u)
		}
	}
}

// TestM2006_PreM2003FormatOmitsLayerC3 は旧形式で層 C-3 を出さないことを固定する。
//
// ★適用済みの 6 stem(000026 / 000030 / 000045 / 000055 / 000072 / 000073)を再生成すると
// きに層 C-3 が出ると、golden の byte 一致が崩れる。golden は手編集ドリフトの唯一の
// 検出経路であり、恒常的に赤くなると「赤いのが普通」になって実際のドリフトを見逃す。
func TestM2006_PreM2003FormatOmitsLayerC3(t *testing.T) {
	const char = "kimberly"
	rows := []MoveRow{mkRow(char, "arc_step", "special", "弧空", "", true)}
	res, err := GenerateAliases(NumericRule, []string{char}, map[string][]MoveRow{char: rows},
		[]string{char}, AliasHeader("t", "t", WithFormat(FormatPreM2003)), WithFormat(FormatPreM2003))
	if err != nil {
		t.Fatalf("GenerateAliases: %v", err)
	}
	if _, ok := aliasOf(res, "arc_step"); ok {
		t.Error("旧形式で層 C-3 が出ている(適用済みマイグレの byte 一致が崩れる)")
	}
	u, ok := unfilledOf(res, "arc_step")
	if !ok || u.Reason != ReasonDerivedNoCommand {
		t.Errorf("旧形式では derived-no-command のまま残るべき(%+v)", u)
	}
}
