package moveindex

import (
	"reflect"
	"testing"
)

func TestLookup_SingleCandidate(t *testing.T) {
	ix := New()
	ix.Add("terry", "power_wave_light", "d dr r plus p_l", false, 0)

	got, ok := ix.Lookup("terry", "d dr r plus p_l")
	if !ok || got != "power_wave_light" {
		t.Fatalf("Lookup = %q, %v; want power_wave_light, true", got, ok)
	}
}

func TestLookup_WhitespaceNormalization(t *testing.T) {
	ix := New()
	ix.Add("terry", "power_wave_light", "d dr r plus p_l", false, 0)

	// 索引キー・クエリ双方の余分な空白を吸収する。
	if got, ok := ix.Lookup("terry", "  d   dr r   plus p_l "); !ok || got != "power_wave_light" {
		t.Fatalf("Lookup(normalized) = %q, %v; want power_wave_light, true", got, ok)
	}
}

func TestLookup_IDMinTiebreak(t *testing.T) {
	ix := New()
	// 同一 command に複数 move。id(投入順)が若いものを返す。挿入順は逆にしておく。
	ix.Add("zangief", "move_high_id", "circle plus p_l", false, 5)
	ix.Add("zangief", "move_low_id", "circle plus p_l", false, 2)

	got, ok := ix.Lookup("zangief", "circle plus p_l")
	if !ok || got != "move_low_id" {
		t.Fatalf("Lookup = %q; want move_low_id (id 最小タイブレーク)", got)
	}
	all := ix.LookupAll("zangief", "circle plus p_l")
	want := []string{"move_low_id", "move_high_id"}
	if !reflect.DeepEqual(all, want) {
		t.Fatalf("LookupAll = %v; want %v (id 昇順)", all, want)
	}
}

func TestLookup_Unresolved(t *testing.T) {
	ix := New()
	ix.Add("terry", "power_wave_light", "d dr r plus p_l", false, 0)

	if got, ok := ix.Lookup("terry", "no such token"); ok {
		t.Fatalf("Lookup(unknown) = %q, true; want unresolved", got)
	}
	if got, ok := ix.Lookup("unknown_char", "d dr r plus p_l"); ok {
		t.Fatalf("Lookup(unknown char) = %q, true; want unresolved", got)
	}
	if all := ix.LookupAll("terry", "no such token"); all != nil {
		t.Fatalf("LookupAll(unknown) = %v; want nil", all)
	}
}

func TestAdd_SkipsDerivedAndMarkersAndEmpty(t *testing.T) {
	ix := New()
	ix.Add("terry", "power_drive", "p_m chain p_h", true, 0)                            // 派生技
	ix.Add("terry", "rush_standing_light_punch", "", false, 1)                          // 空 command
	ix.Add("guile", "sonic_cross_od", "r plus p p or cond{（...中に）} r plus p", false, 2) // 条件残留
	ix.Add("mystery", "raw_move", "raw{-} plus p", false, 3)                            // 未知トークン

	// いずれも索引に載らない。
	for _, tc := range []struct{ char, token string }{
		{"terry", "p_m chain p_h"},
		{"terry", ""},
	} {
		if _, ok := ix.Lookup(tc.char, tc.token); ok {
			t.Fatalf("Lookup(%q,%q) resolved; want skipped", tc.char, tc.token)
		}
	}

	skipped := ix.Skipped()
	if len(skipped) != 4 {
		t.Fatalf("Skipped len = %d; want 4", len(skipped))
	}
	reasons := map[SkipReason]int{}
	for _, s := range skipped {
		reasons[s.Reason]++
	}
	for _, r := range []SkipReason{SkipDerived, SkipEmptyCommand, SkipConditionResidual, SkipUnknownToken} {
		if reasons[r] != 1 {
			t.Fatalf("reason %q count = %d; want 1 (all: %+v)", r, reasons[r], reasons)
		}
	}
}

// TestNormalizeCommand_Vocabulary は TOOL-002 §9.9 の 24 トークン+テキスト由来トークンの
// numpad 正規化(M17-02 §4.2 確定仕様)を固定する。
func TestNormalizeCommand_Vocabulary(t *testing.T) {
	cases := []struct {
		command string
		want    string
	}{
		// 方向 9 種(単独)
		{"u", "8"}, {"d", "2"}, {"l", "4"}, {"r", "6"},
		{"ul", "7"}, {"ur", "9"}, {"dl", "1"}, {"dr", "3"}, {"n", "5"},
		// ボタン 8 種(単独)
		{"p", "P"}, {"p_l", "LP"}, {"p_m", "MP"}, {"p_h", "HP"},
		{"k", "K"}, {"k_l", "LK"}, {"k_m", "MK"}, {"k_h", "HK"},
		// ため・一回転
		{"charge_d", "[2]"}, {"charge_l", "[4]"}, {"charge_r", "[6]"}, {"circle", "360"},
		// 単方向+ボタン(段階2 スコープ・plus は連結へ畳む)
		{"d plus p_m", "2MP"}, {"r plus k_h", "6HK"}, {"dr plus p_h", "3HP"},
		// 必殺技モーション(索引は広く持つ=M17-04 用)
		{"d dr r plus p_l", "236LP"}, {"d dl l plus k_m", "214MK"},
		// ため・一回転の複合
		{"charge_l r plus p_l", "[4]6LP"}, {"circle plus p", "360P"},
		// 同時押し(plus 有無に依らずボタン隣接は '+')
		{"p_l plus k_l", "LP+LK"}, {"r plus p_l k_l", "6LP+LK"},
		// 接続子(or/chain/alt_sep/hold)
		{"n or r plus p_l k_l", "5/6LP+LK"},
		{"p_m chain p_h", "MP>HP"},
		{"d plus k_m alt_sep d plus k_h", "2MK|2HK"},
		{"k_h hold", "HK(hold)"},
		// 空白正規化(M14-03b からの継承挙動)
		{"  d   plus  p_m ", "2MP"},
	}
	for _, tc := range cases {
		got, unknown := normalizeCommand(tc.command)
		if got != tc.want || len(unknown) != 0 {
			t.Errorf("normalizeCommand(%q) = %q (unknown=%v); want %q (unknown なし)", tc.command, got, unknown, tc.want)
		}
	}
}

// TestNormalizeCommand_UnknownToken は語彙外トークンの verbatim 通過+unknown 報告を固定する。
func TestNormalizeCommand_UnknownToken(t *testing.T) {
	got, unknown := normalizeCommand("d plus bogus_token")
	if got != "2bogus_token" {
		t.Fatalf("normalizeCommand = %q; want verbatim 通過 %q", got, "2bogus_token")
	}
	if len(unknown) != 1 || unknown[0] != "bogus_token" {
		t.Fatalf("unknown = %v; want [bogus_token]", unknown)
	}
	// 正規化済みキーの再正規化は冪等(runtime の AddIndexed 済みキーを Lookup が引ける前提)。
	again, _ := normalizeCommand(got)
	if again != got {
		t.Fatalf("再正規化 = %q; want 冪等 %q", again, got)
	}
}

// TestAdd_SkipsVocabularyUnknownToken は語彙外トークンを含む行が索引非搭載として
// 記録されること(M17-02 §4.1 のトークン単位判定の追加)を固定する。
func TestAdd_SkipsVocabularyUnknownToken(t *testing.T) {
	ix := New()
	ix.Add("terry", "typo_move", "d plus p_x", false, 0) // p_x は語彙外

	if _, ok := ix.Lookup("terry", "d plus p_x"); ok {
		t.Fatal("語彙外トークンを含む行が索引に載っている")
	}
	skipped := ix.Skipped()
	if len(skipped) != 1 || skipped[0].Reason != SkipUnknownToken {
		t.Fatalf("Skipped = %+v; want SkipUnknownToken 1 件", skipped)
	}
}

// TestLookup_AcceptsNormalizedKey は正規化済みキーでの直接クエリ(FE/M17-04 が numpad 形で
// 引くケース)が正準トークン列と同じ結果になることを固定する。
func TestLookup_AcceptsNormalizedKey(t *testing.T) {
	ix := New()
	ix.Add("ryu", "hadoken_light", "d dr r plus p_l", false, 0)

	if got, ok := ix.Lookup("ryu", "236LP"); !ok || got != "hadoken_light" {
		t.Fatalf("Lookup(236LP) = %q, %v; want hadoken_light, true", got, ok)
	}
}

func TestAddIndexed_RuntimeReconstruction(t *testing.T) {
	ix := New()
	// move_commands テーブル相当(正規化済みキー+moves.id)からの再構築。
	ix.AddIndexed("ryu", "high_id_move", "2MP", 42)
	ix.AddIndexed("ryu", "low_id_move", "2MP", 7)

	if got, ok := ix.Lookup("ryu", "d plus p_m"); !ok || got != "low_id_move" {
		t.Fatalf("Lookup = %q, %v; want low_id_move (moves.id 最小)", got, ok)
	}
	if len(ix.Skipped()) != 0 {
		t.Fatalf("AddIndexed は skip 判定を通さない; Skipped = %+v", ix.Skipped())
	}
}

func TestEntries_DeterministicOrder(t *testing.T) {
	ix := New()
	ix.Add("terry", "crouching_medium_punch", "d plus p_m", false, 3)
	ix.Add("terry", "standing_light_punch", "p_l", false, 1)
	ix.Add("guile", "sonic_boom_light", "charge_l r plus p_l", false, 2)
	ix.Add("terry", "same_key_high", "d plus p_m", false, 9)

	got := ix.Entries()
	want := []Entry{
		{CharKey: "guile", TokenKey: "[4]6LP", MoveCode: "sonic_boom_light", ID: 2},
		{CharKey: "terry", TokenKey: "2MP", MoveCode: "crouching_medium_punch", ID: 3},
		{CharKey: "terry", TokenKey: "2MP", MoveCode: "same_key_high", ID: 9},
		{CharKey: "terry", TokenKey: "LP", MoveCode: "standing_light_punch", ID: 1},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Entries = %+v; want %+v (CharKey→TokenKey→ID 昇順)", got, want)
	}
}

func TestCharScope_IsPerCharacter(t *testing.T) {
	ix := New()
	ix.Add("terry", "terry_move", "d plus p_l", false, 0)
	ix.Add("guile", "guile_move", "d plus p_l", false, 0)

	if got, _ := ix.Lookup("terry", "d plus p_l"); got != "terry_move" {
		t.Fatalf("terry scope leaked: got %q", got)
	}
	if got, _ := ix.Lookup("guile", "d plus p_l"); got != "guile_move" {
		t.Fatalf("guile scope leaked: got %q", got)
	}
}
