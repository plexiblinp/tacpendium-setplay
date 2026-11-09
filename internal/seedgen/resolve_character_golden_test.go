package seedgen

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// キャラ 1 人分の表記の解決を、入力ごと凍結した golden で守る。
//
// ★既存の凍結 golden(TestGolden_*)は比較先だけが凍結で、入力は現役の character_data/*.csv を読む。
//   データの追従で CSV が変われば赤になる——それは正しい合図であり、本テストはそれを置き換えない。
//   本テストは入力も testdata/ へ写して凍結し、「表記の規則が変わっていない」ことだけを主張する。
//
// ★入力の写し(resolveGoldenInputDir)は character_data/ のファイルを手を加えずに写したもの。
//   写した元のコミットは 46e5654(Merge pull request #425)。★写しは手で直さないこと。
//
// ★期待値(resolveGoldenFile)は、分ける前の実装で作った。更新するのは表記の規則を
//   意図して変えたときだけであり、差分を読んでから
//   TACPENDIUM_UPDATE_GOLDEN=1 go test ./internal/seedgen/ -run TestResolveCharacterGolden
//   で書き直す。

const (
	resolveGoldenInputDir = "m52-14-resolve-input"
	resolveGoldenFile     = "m52-14-resolve-golden.json"
	resolveSyntheticChar  = "zz_synthetic"
)

// resolveGoldenFormats は凍結する形式の全部(format.go の 4 つ)。
var resolveGoldenFormats = []struct {
	name   string
	format SQLFormat
}{
	{"pre-m2003", FormatPreM2003},
	{"pre-m4808", FormatPreM4808},
	{"pre-m4305", FormatPreM4305},
	{"current", FormatCurrent},
}

// resolveGoldenCase は 1 ケース分の出力全体。
type resolveGoldenCase struct {
	Name    string
	Err     string        `json:",omitempty"`
	Result  *AliasResult  `json:",omitempty"`
	Report  string        `json:",omitempty"`
	Rewrite *AliasRewrite `json:",omitempty"`
}

// readResolveGoldenInput は凍結した写しを読む(キャラは辞書順。行番号は通しで振る)。
func readResolveGoldenInput(t *testing.T) ([]string, map[string][]MoveRow) {
	t.Helper()
	dir := filepath.Join(repoRoot(t), "internal", "seedgen", goldenDir, resolveGoldenInputDir)
	paths, err := filepath.Glob(filepath.Join(dir, "*.csv"))
	if err != nil {
		t.Fatalf("glob %s: %v", dir, err)
	}
	if len(paths) == 0 {
		t.Fatalf("凍結した入力が無い: %s", dir)
	}
	sort.Strings(paths)
	var order []string
	byChar := map[string][]MoveRow{}
	next := 0
	for _, p := range paths {
		code := strings.TrimSuffix(filepath.Base(p), ".csv")
		rows, err := ReadFile(p, next)
		if err != nil {
			t.Fatalf("ReadFile %s: %v", p, err)
		}
		order = append(order, code)
		byChar[code] = rows
		next += len(rows)
	}
	return order, byChar
}

// resolveSyntheticRows は実データでは通らない分岐を通すための合成の行(写しではない)。
func resolveSyntheticRows() []MoveRow {
	c := resolveSyntheticChar
	rows := []MoveRow{
		// 移動系の行は解決の対象外(CSV の読み込みでは落ちないので、ここで混ぜる)。
		mkRow(c, "forward", "system", "前歩き", "r", false),
		// 層 B と層 A の両方。層 B が勝ち、捨てた層 A の値が記録される。
		mkRow(c, "standing_light_punch", "normal", "立ち弱P", "p_l", false),
		mkRow(c, "crouching_light_punch", "normal", "しゃがみ弱P", "d plus p_l", false),
		// 表記が被る組を 2 つ(被りの並べ替えに 2 件以上が要る)。
		mkRow(c, "crouching_heavy_punch", "normal", "しゃがみ強P", "d plus p_h", false),
		mkRow(c, "flying_body_press", "unique", "空中の特殊技", "d plus p_h", false),
		mkRow(c, "crouching_light_kick", "normal", "しゃがみ弱K", "d plus k_l", false),
		mkRow(c, "sliding_kick", "unique", "特殊技", "d plus k_l", false),
		// 派生技どうし ・派生でない技との被り(優先の規則で残る側が決まる)。
		mkRow(c, "chain_strike", "special", "連撃", "d dr r plus p_m", false),
		mkRow(c, "chain_strike_follow", "special", "連撃の派生", "d dr r plus p_m", true),
		mkRow(c, "chain_finisher", "special", "派生の締め", "d dl l plus k_h", true),
		// move_code が層 B の形の派生技(層 B が派生の判定より先に当たる。旧規則で順を入れ替えると穴に変わる)。
		mkRow(c, "standing_heavy_kick", "normal", "立ち強K(派生)", "k_h", true),
		// command を持たない派生技(層 C-3 の対象外)と、command を持たない技。
		mkRow(c, "auto_follow", "special", "自動の派生", "", true),
		mkRow(c, "taunt_pose", "unique", "挑発", "", false),
		// 移動系の表記と被る技(移動系が予約している)。
		mkRow(c, "step_forward", "unique", "前進", "r", false),
		// 索引に載らない command。
		mkRow(c, "conditional_move", "special", "条件付き", "raw{条件} plus p", false),
		mkRow(c, "unknown_token_move", "special", "語彙外", "zz plus p", false),
		// SA / CA で番号を取れないもの(注記なし)と、取れるもの。
		mkRow(c, "super_art_mystery", "super_art", "謎の SA", "d dr r d dr r plus k", false),
		mkRow(c, "sa2_burst", "super_art", "SA2", "d dl l d dl l plus p", false),
		mkRow(c, "ca_burst", "critical_art", "CA", "d dr r d dr r plus p", false),
		// ターゲットコンボ(描けるもの ・描けないもの)。
		mkRow(c, "tc_ok", "target_combo", "TC", "p_m chain p_h", false),
		mkRow(c, "tc_odd", "target_combo", "変な TC", "charge_l r plus p_l chain p_h", false),
		// rush: 合成できるもの ・元技が空 ・元技が実在しない ・元技に表記が無い ・rush どうしの被り。
		mkRush(c, "rush_crouching_light_punch", "ラッシュ 2LP", "crouching_light_punch"),
		mkRush(c, "rush_crouching_light_punch_b", "ラッシュ 2LP その 2", "crouching_light_punch"),
		mkRush(c, "rush_no_original", "ラッシュ 元技なし", ""),
		mkRush(c, "rush_ghost", "ラッシュ 幽霊", "no_such_move"),
		mkRush(c, "rush_taunt", "ラッシュ 挑発", "taunt_pose"),
		mkRush(c, "rush_heavy", "ラッシュ 2HP", "crouching_heavy_punch"),
	}
	return withIndex(rows...)
}

// runResolveGoldenCases は 1 つの入力について、表記の解決を通る生成関数を全部回す。
func runResolveGoldenCases(prefix string, order []string, byChar map[string][]MoveRow) []resolveGoldenCase {
	var out []resolveGoldenCase
	for _, rule := range []AliasPresetRule{NumericRule, SRKRule} {
		base := prefix + "/" + rule.PresetCode
		for _, f := range resolveGoldenFormats {
			opts := []Option{WithFormat(f.format)}
			out = append(out, resolveAliasesCase(base+"/aliases/"+f.name, rule, order, byChar, opts))
		}
		out = append(out, resolveAliasesCase(base+"/aliases/current-noinput-only", rule, order, byChar,
			[]Option{WithFormat(FormatCurrent), WithNoInputOnly()}))

		delta := resolveGoldenCase{Name: base + "/delta"}
		if res, err := GenerateAliasDelta(rule, order, byChar, AliasHeader("golden_delta", "golden")); err != nil {
			delta.Err = err.Error()
		} else {
			delta.Result = res
			delta.Report = RenderAliasReport(rule, res)
		}
		out = append(out, delta)

		rewrite := resolveGoldenCase{Name: base + "/rewrite"}
		if rw, err := GenerateAliasRewrite(rule, order, byChar, AliasHeader("golden_rewrite", "golden")); err != nil {
			rewrite.Err = err.Error()
		} else {
			rewrite.Rewrite = rw
		}
		out = append(out, rewrite)
	}
	return out
}

func resolveAliasesCase(name string, rule AliasPresetRule, order []string, byChar map[string][]MoveRow,
	opts []Option) resolveGoldenCase {
	c := resolveGoldenCase{Name: name}
	res, err := GenerateAliases(rule, order, byChar, order, AliasHeader("golden_aliases", "golden", opts...), opts...)
	if err != nil {
		c.Err = err.Error()
		return c
	}
	c.Result = res
	c.Report = RenderAliasReport(rule, res)
	return c
}

// buildResolveGoldenCases は凍結するケースの全部を作る。
func buildResolveGoldenCases(t *testing.T) []resolveGoldenCase {
	t.Helper()
	order, byChar := readResolveGoldenInput(t)
	cases := runResolveGoldenCases("frozen", order, byChar)

	syn := map[string][]MoveRow{resolveSyntheticChar: resolveSyntheticRows()}
	cases = append(cases, runResolveGoldenCases("synthetic", []string{resolveSyntheticChar}, syn)...)

	// 誤りの経路(解決の前に止まるもの)。
	missing := resolveGoldenCase{Name: "error/missing-character"}
	if _, err := GenerateAliases(NumericRule, []string{"no_such_char"}, syn, []string{resolveSyntheticChar},
		AliasHeader("golden_aliases", "golden")); err != nil {
		missing.Err = err.Error()
	}
	noMovement := resolveGoldenCase{Name: "error/empty-movement-chars"}
	if _, err := GenerateAliases(NumericRule, []string{resolveSyntheticChar}, syn, nil,
		AliasHeader("golden_aliases", "golden")); err != nil {
		noMovement.Err = err.Error()
	}
	return append(cases, missing, noMovement)
}

// TestResolveCharacterGolden は表記の解決を通る生成関数の出力全体を、凍結した入力で守る。
func TestResolveCharacterGolden(t *testing.T) {
	cases := buildResolveGoldenCases(t)
	got, err := json.MarshalIndent(cases, "", " ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got = append(got, '\n')

	path := filepath.Join(repoRoot(t), "internal", "seedgen", goldenDir, resolveGoldenFile)
	if os.Getenv(goldenUpdateEnv) != "" {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
		t.Logf("golden 更新: %s(%d ケース)", resolveGoldenFile, len(cases))
		return
	}
	wantBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v (★%s=1 で生成できる)", path, err, goldenUpdateEnv)
	}
	if string(wantBytes) == string(got) {
		return
	}
	// ★どのケースが違うかを名前で出す(本文は大きいため丸ごとは出さない)。
	var want []json.RawMessage
	if err := json.Unmarshal(wantBytes, &want); err != nil {
		t.Fatalf("unmarshal %s: %v", path, err)
	}
	if len(want) != len(cases) {
		t.Errorf("ケースの数が違う: got %d, want %d", len(cases), len(want))
	}
	for i, c := range cases {
		g, _ := json.MarshalIndent(c, " ", " ")
		if i >= len(want) {
			t.Errorf("余分なケース: %s", c.Name)
			continue
		}
		var w resolveGoldenCase
		if err := json.Unmarshal(want[i], &w); err != nil {
			t.Fatalf("unmarshal case %d: %v", i, err)
		}
		wb, _ := json.MarshalIndent(w, " ", " ")
		if string(g) != string(wb) {
			t.Errorf("ケース %s が凍結 golden と不一致(表記の規則が変わった)", c.Name)
		}
	}
	t.Errorf("%s が凍結 golden と不一致。意図した変更なら差分を読んでから %s=1 で更新すること",
		resolveGoldenFile, goldenUpdateEnv)
}
