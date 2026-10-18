package seedgen

import (
	"strings"
	"testing"
)

// M20-03 §5.1 (l): 生成器 2 本が preset_aliases への INSERT に character_id を出すことを固定する。
//
// ★これが本サブで一番静かに壊れる箇所である。character_id は nullable であるため、
// 生成器が出さなくてもマイグレは通り、テストも lint も緑になる。そのまま NULL 行が入り、
// UNIQUE(preset_id, character_id, alias_text) はその行に当たらない——
// ★制約をすり抜けた行は、落ちないため気づけない(指示書 §4.5)。
// ⇒ 出力の形を機械で固定する以外に、これを見つける経路が無い。
//
// ★旧形式(FormatPreM2003)側も対で固定する。適用済み 6 ファイルとの byte 一致を
// 主張する golden がそちらに依存しており、既定値を取り違えると golden が
// 「再生成しろ」という誤ったメッセージで落ちるためである。

// m2003AliasBody は official_ja_move 側(generate.go)を回すための最小 CSV 本文。
const m2003AliasBody = "terry,standing_light_punch,normal,立ち弱P,4,3,7,13,4,-1,300,false,false,false,,,,p_l,,,,,,,\n"

// TestM2003_OfficialJaMoveGeneratorEmitsCharacterID は generate.go の writeAliasInsert を固定する。
func TestM2003_OfficialJaMoveGeneratorEmitsCharacterID(t *testing.T) {
	rows, err := Read(strings.NewReader(testHeader+m2003AliasBody), 0)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	src := map[string][]MoveRow{"terry": rows}

	// ── 既定(FormatCurrent): character_id を出す ─────────────────────────
	cur, err := Generate([]string{"terry"}, src)
	if err != nil {
		t.Fatalf("Generate(current): %v", err)
	}
	for _, want := range []string{
		"INSERT INTO preset_aliases (preset_id, move_id, character_id, alias_text)",
		"m.id, m.character_id, v.alias_text",
	} {
		if !strings.Contains(cur.UpSQL, want) {
			t.Errorf("既定形式の出力に %q が無い。character_id を出さないと NULL 行が入り、"+
				"UNIQUE をすり抜ける(落ちないため気づけない)\n%s", want, cur.UpSQL)
		}
	}

	// ── 旧形式(FormatPreM2003): 出さない ────────────────────────────────
	old, err := Generate([]string{"terry"}, src, WithFormat(FormatPreM2003))
	if err != nil {
		t.Fatalf("Generate(pre-M20-03): %v", err)
	}
	if !strings.Contains(old.UpSQL, "INSERT INTO preset_aliases (preset_id, move_id, alias_text)") {
		t.Errorf("旧形式の出力が M20-02 以前の形になっていない:\n%s", old.UpSQL)
	}
	if strings.Contains(old.UpSQL, "m.character_id, v.alias_text") {
		t.Errorf("旧形式の出力に character_id が混ざっている(適用済み 6 ファイルとの byte 一致が壊れる):\n%s",
			old.UpSQL)
	}
}

// TestM2003_AliasPresetGeneratorEmitsCharacterID は generate_m2002.go の 2 つの writer を固定する。
//
//	writeCharAliasInsert     … キャラ別(層 A / B / C-rush)
//	writeMovementAliasInsert … 移動系 9 code(alias_text_en を伴う)
func TestM2003_AliasPresetGeneratorEmitsCharacterID(t *testing.T) {
	rows := []MoveRow{
		mkRow("ryu", "standing_light_punch", "normal", "立ち弱P", "p_l", false),
	}
	src := map[string][]MoveRow{"ryu": rows}

	gen := func(t *testing.T, opts ...Option) string {
		t.Helper()
		res, err := GenerateAliases(NumericRule, []string{"ryu"}, src, []string{"ryu"},
			AliasHeader("test_stem", "unit test"), opts...)
		if err != nil {
			t.Fatalf("GenerateAliases: %v", err)
		}
		return res.UpSQL
	}

	// ── 既定(FormatCurrent) ─────────────────────────────────────────────
	cur := gen(t)
	for _, want := range []string{
		// 移動系(alias_text_en 付き)。
		"INSERT INTO preset_aliases (preset_id, move_id, character_id, alias_text, alias_text_en)",
		"m.id, m.character_id, v.alias_text, v.alias_text_en",
		// キャラ別。
		"INSERT INTO preset_aliases (preset_id, move_id, character_id, alias_text)",
		"m.id, m.character_id, v.alias_text\n",
	} {
		if !strings.Contains(cur, want) {
			t.Errorf("既定形式の出力に %q が無い:\n%s", want, cur)
		}
	}

	// ── 旧形式(FormatPreM2003) ─────────────────────────────────────────
	old := gen(t, WithFormat(FormatPreM2003))
	for _, want := range []string{
		"INSERT INTO preset_aliases (preset_id, move_id, alias_text, alias_text_en)",
		"INSERT INTO preset_aliases (preset_id, move_id, alias_text)",
	} {
		if !strings.Contains(old, want) {
			t.Errorf("旧形式の出力が M20-02 以前の形になっていない(%q が無い):\n%s", want, old)
		}
	}
	if strings.Contains(old, "m.character_id, v.alias_text") {
		t.Errorf("旧形式の出力に character_id が混ざっている:\n%s", old)
	}
}

// TestM2003_DefaultFormatIsCurrent は「オプション未指定なら新形式」を固定する。
//
// ★cmd/seedgen はオプションを渡さない。既定が旧形式へ倒れると、次の seed 波が
// character_id 無しで投入され、誰も気づかないまま NULL 行が積み上がる。
func TestM2003_DefaultFormatIsCurrent(t *testing.T) {
	if got := resolveOptions(nil).format; got != FormatCurrent {
		t.Errorf("オプション未指定時の形式 = %v, want FormatCurrent", got)
	}
	if !FormatCurrent.includesCharacterID() {
		t.Error("FormatCurrent が character_id を含まないことになっている")
	}
	if FormatPreM2003.includesCharacterID() {
		t.Error("FormatPreM2003 が character_id を含むことになっている")
	}
}
