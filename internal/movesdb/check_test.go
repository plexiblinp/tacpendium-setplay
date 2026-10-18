package movesdb

import (
	"strings"
	"testing"

	"github.com/plexiblinp/tacpendium-setplay/internal/seedgen"
)

const testDataVersion = "0123456789012345678901234567890123456789"

func validFile(t *testing.T) CharacterFile {
	t.Helper()
	parent := "a"
	f := CharacterFile{
		SchemaVersion: SchemaVersion, DataVersion: testDataVersion,
		Character: CharacterRef{Code: "test", Name: LocalizedName{JA: "テスト", EN: "Test"}},
		States:    []State{},
		Moves: []Move{
			{Code: "a", Category: "special", Parents: []string{}, Notation: Notation{NameJA: "あ", NameEN: "A", SRK: "236P", Numeric: "236P"}},
			{Code: "b", Category: "special", Parents: []string{"a"}, OriginalMove: &parent, Notation: Notation{NameJA: "い", NameEN: "B", SRK: "236K", Numeric: "236K"}},
		},
	}
	refreshFingerprints(t, &f)
	if errs := CheckFile(f); len(errs) != 0 {
		t.Fatalf("valid file has errors: %v", errs)
	}
	return f
}

func refreshFingerprints(t *testing.T, f *CharacterFile) {
	t.Helper()
	for i := range f.Moves {
		fp, err := moveFingerprint(f.Moves[i])
		if err != nil {
			t.Fatal(err)
		}
		f.Moves[i].CalcFingerprint = fp
	}
	fp, err := characterFingerprint(*f)
	if err != nil {
		t.Fatal(err)
	}
	f.CalcFingerprint = fp
}

// 各検査が、守りたい不具合を 1 つ入れた入力で狙いどおり赤くなること。
func TestCheckFile_DetectsEachViolation(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*CharacterFile)
		want   string
	}{
		{"空の表記", func(f *CharacterFile) { f.Moves[1].Notation.SRK = " " }, "srk が空"},
		{"同じ表記", func(f *CharacterFile) { f.Moves[1].Notation.NameEN = "A" }, "nameEn \"A\" が 2 技で同じ"},
		{"同じ numeric", func(f *CharacterFile) { f.Moves[1].Notation.Numeric = "236P" }, "numeric \"236P\" が 2 技で同じ"},
		{"無い親", func(f *CharacterFile) { f.Moves[1].Parents = []string{"zz"} }, "親 zz が無い"},
		{"null の parents", func(f *CharacterFile) { f.Moves[0].Parents = nil }, "parents が null"},
		{"無い元技", func(f *CharacterFile) { s := "zz"; f.Moves[1].OriginalMove = &s }, "originalMove zz が無い"},
		{"技 code の重複", func(f *CharacterFile) { f.Moves[1].Code = "a" }, "技 code が重複"},
		{"dataVersion", func(f *CharacterFile) { f.DataVersion = "x" }, "dataVersion"},
		{"dataVersion 大文字", func(f *CharacterFile) { f.DataVersion = strings.ToUpper(testDataVersion[:39]) + "A" }, "dataVersion"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := validFile(t)
			c.mutate(&f)
			refreshFingerprints(t, &f)
			errs := CheckFile(f)
			if !containsSubstr(errs, c.want) {
				t.Errorf("want violation containing %q, got %v", c.want, errs)
			}
		})
	}
}

func TestCheckFile_DetectsStaleFingerprint(t *testing.T) {
	f := validFile(t)
	v := 5
	f.Moves[0].Startup = &v // 指紋を作り直さない
	errs := CheckFile(f)
	if !containsSubstr(errs, "a の calcFingerprint が中身と合わない") || !containsSubstr(errs, "test: calcFingerprint が中身と合わない") {
		t.Errorf("stale fingerprint not detected: %v", errs)
	}
}

func TestFingerprint_IgnoresNotation(t *testing.T) {
	f := validFile(t)
	before := f.Moves[0].CalcFingerprint
	f.Moves[0].Notation.NameEN = "Changed"
	v := 3
	f.Moves[0].Recovery = &v
	fp, err := moveFingerprint(f.Moves[0])
	if err != nil {
		t.Fatal(err)
	}
	if fp != before {
		t.Errorf("表記・recovery の変更で指紋が変わった")
	}
	f.Moves[0].Total = &v
	if fp, _ := moveFingerprint(f.Moves[0]); fp == before {
		t.Errorf("total の変更で指紋が変わらない")
	}
}

func containsSubstr(list []string, sub string) bool {
	for _, s := range list {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func TestCheckCSVAgainstDB_DetectsMismatch(t *testing.T) {
	v, w := 10, 11
	ds := &Dataset{
		Characters: []DBCharacter{{ID: 1, Code: "test"}},
		Moves: map[int64][]DBMove{1: {
			{ID: 1, CharacterID: 1, Code: "a", Category: "special", Total: &v, StartupBasis: "unknown"},
		}},
		CSV: map[string][]seedgen.MoveRow{"test": {
			{CharacterCode: "test", MoveCode: "a", Category: "special", Total: &v},
		}},
	}
	if errs := CheckCSVAgainstDB(ds); len(errs) != 0 {
		t.Fatalf("一致しているのに違反: %v", errs)
	}
	ds.CSV["test"][0].Total = &w
	ds.CSV["test"][0].IsProjectile = true
	errs := CheckCSVAgainstDB(ds)
	if !containsSubstr(errs, "total,is_projectile") {
		t.Errorf("total・is_projectile の食い違いを検出しない: %v", errs)
	}
	ds.CSV["test"] = append(ds.CSV["test"], seedgen.MoveRow{MoveCode: "zz"})
	if !containsSubstr(CheckCSVAgainstDB(ds), "CSV の技が DB に無い") {
		t.Errorf("DB に無い技を検出しない")
	}
}

// 本家 DB に表記が無い技は、本家の規則で回し直して実際に落ちる技でなければ違反にする
// (「他の技と被っている」だけでは通さない)。
func TestCheckOracle_RequiresUpstreamToActuallyDrop(t *testing.T) {
	rows := []seedgen.MoveRow{
		{CharacterCode: "test", MoveCode: "a", Category: "special", NameJA: "あ", Command: "d dr r plus p_l", RowIndex: 0},
		{CharacterCode: "test", MoveCode: "b", Category: "special", NameJA: "い", Command: "d dr r plus k_l", RowIndex: 1},
	}
	ds := &Dataset{
		Characters: []DBCharacter{{ID: 1, Code: "test"}},
		Moves:      map[int64][]DBMove{1: {{ID: 1, CharacterID: 1, Code: "a"}, {ID: 2, CharacterID: 1, Code: "b"}}},
		CSV:        map[string][]seedgen.MoveRow{"test": rows},
		Aliases: map[string]map[int64]Alias{
			PresetNumeric: {1: {Text: "236LP"}},
			PresetSRK:     {1: {Text: "236LP"}, 2: {Text: "236LK"}},
		},
	}
	res := func() *NotationResult {
		// b の基底が(生成器の不具合で)a と同じになった、という状態
		return &NotationResult{
			Text:   map[string]string{"a": "236LP (あ)", "b": "236LP (い)"},
			Base:   map[string]string{"a": "236LP", "b": "236LP"},
			Source: map[string]NotationSource{"a": SourceGenerator, "b": SourceGenerator},
		}
	}
	_, errs := CheckOracle(ds, []CharacterReport{{Code: "test", Numeric: res(), SRK: res()}})
	if !containsSubstr(errs, "numeric/test/b: 本家 DB に表記が無いのに、本家の規則では落ちない") {
		t.Errorf("被りがあるだけで通してしまう: %v", errs)
	}
	if !containsSubstr(errs, "srk/test/b: 基底 \"236LP\" が本家 \"236LK\" と違う") {
		t.Errorf("本家と違う基底を検出しない: %v", errs)
	}
}

func TestMissingKeys(t *testing.T) {
	f := validFile(t)
	f.SafeJump = &SafeJump{True: 43, F5: 42, F6: 41}
	f.States = []State{{Code: "s", Name: LocalizedName{JA: "状態", EN: "State"}}}
	b, err := Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	if miss := missingKeys[CharacterFile](b); len(miss) != 0 {
		t.Fatalf("全キーあるのに欠けと判定: %v", miss)
	}
	// 入れ子の各所からキーを 1 つずつ消す
	broken := string(b)
	for _, del := range []string{"\"startup\": null,\n", "\"srk\": \"236P\",\n", "\"f5\": 42,\n", "\"en\": \"State\"\n", "\"state\": null,\n"} {
		if !strings.Contains(broken, del) {
			t.Fatalf("消す対象 %q が無い", del)
		}
		broken = strings.Replace(broken, del, "", 1)
	}
	broken = strings.Replace(broken, "\"ja\": \"状態\",", "\"ja\": \"状態\"", 1) // 消した en の前のカンマ
	miss := missingKeys[CharacterFile]([]byte(broken))
	for _, want := range []string{"moves[0].startup", "moves[0].notation.srk", "safeJump.f5", "states[0].name.en", "moves[0].state"} {
		if !containsSubstr(miss, want) {
			t.Errorf("欠け %s を検出しない: %v", want, miss)
		}
	}
	// index.json の形
	ib := []byte(`{"schemaVersion": 1, "dataVersion": "x", "characters": [{"code": "a", "name": {"ja": "あ"}, "file": "a.json", "moveCount": 1, "calcFingerprint": "c"}]}`)
	miss = missingKeys[Index](ib)
	for _, want := range []string{"characters[0].name.en", "characters[0].fileSha256"} {
		if !containsSubstr(miss, want) {
			t.Errorf("index の欠け %s を検出しない: %v", want, miss)
		}
	}
}
