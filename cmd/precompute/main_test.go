package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/plexiblinp/tacpendium-setplay/engine"
)

const testMovesDir = "../../data/moves"

// testChars は通しで回すキャラ(既定のキャラ ・連打版の型 A ・単位上限に早く達するキャラ)。
var testChars = []string{"ryu", "lily", "zangief"} // index.json の順

func runCLI(t *testing.T, out string) (Manifest, string) {
	t.Helper()
	var w bytes.Buffer
	if err := run(&w, testMovesDir, out, strings.Join(testChars, ",")); err != nil {
		t.Fatalf("run: %v\n%s", err, w.String())
	}
	b, err := os.ReadFile(filepath.Join(out, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m, w.String()
}

// TestRun_OutputServesSameResults は CLI の出力(manifest → moves)を Engine に読ませると、
// data/moves を直接読んだ Engine と同じ結果になることを確かめる(CLI → ファイル → engine の配線)。
func TestRun_OutputServesSameResults(t *testing.T) {
	out := filepath.Join(t.TempDir(), "site")
	m, summary := runCLI(t, out)
	if !strings.Contains(summary, "違反: 0 件") {
		t.Fatalf("要約に違反 0 件が無い:\n%s", summary)
	}
	if m.MaxKnockdownAdvantage != engine.SiteMaxKnockdownAdvantage || len(m.Characters) != len(testChars) {
		t.Fatalf("manifest: maxKA %d ・キャラ %d 人(%d 人を期待)", m.MaxKnockdownAdvantage, len(m.Characters), len(testChars))
	}
	queries := []string{"", "mode=gap&g_min=2&g_max=5", "mode=filler&remain={safe_jump.true}", "n_min=2&n_max=4&approach_only=true&target_types=normal,unique,special,special_projectile,throw"}
	for i, c := range m.Characters {
		if c.Code != testChars[i] {
			t.Fatalf("%d 番目のキャラ %q(index.json の順でない)", i, c.Code)
		}
		moves, err := os.ReadFile(filepath.Join(out, filepath.FromSlash(c.Moves)))
		if err != nil {
			t.Fatal(err)
		}
		if len(moves) != c.MovesBytes {
			t.Fatalf("%s: manifest の大きさが違う", c.Code)
		}
		src, err := os.ReadFile(filepath.Join(testMovesDir, c.Code+".json"))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(src, moves) {
			t.Fatalf("%s: moves が data/moves と違う", c.Code)
		}
		var head struct {
			DataVersion string `json:"dataVersion"`
		}
		if err := json.Unmarshal(moves, &head); err != nil {
			t.Fatal(err)
		}
		if c.DataVersion == "" || c.DataVersion != head.DataVersion {
			t.Fatalf("%s: manifest のキャラの版 %q != moves の %q", c.Code, c.DataVersion, head.DataVersion)
		}
		served, direct := engine.New(), engine.New()
		if _, err := served.Load(moves); err != nil {
			t.Fatal(err)
		}
		if _, err := direct.Load(src); err != nil {
			t.Fatal(err)
		}
		total := 0
		for _, ka := range []int{0, 30, 67, 100} {
			for _, rq := range queries {
				if strings.Contains(rq, "{safe_jump.true}") {
					if c.SafeJump == nil {
						t.Fatalf("%s: manifest に safeJump が無い", c.Code)
					}
					rq = strings.ReplaceAll(rq, "{safe_jump.true}", strconv.Itoa(c.SafeJump.True))
				}
				v, _ := url.ParseQuery(rq)
				q, err := engine.ParseQuery(v)
				if err != nil {
					t.Fatal(err)
				}
				want, err := direct.Suggest(context.Background(), c.Code, ka, q)
				if err != nil {
					t.Fatal(err)
				}
				got, err := served.Suggest(context.Background(), c.Code, ka, q)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(want, got) {
					t.Errorf("%s KA %d %q: 配信 %d 件 / data/moves %d 件", c.Code, ka, rq, len(got.Items), len(want.Items))
				}
				total += len(got.Items)
			}
		}
		if total == 0 {
			t.Fatalf("%s: どの条件も 0 件(比べた意味が無い)", c.Code)
		}
	}
}

// TestRun_Deterministic は同じ入力から同じバイト列(同じファイル名)が出ることを確かめる。
func TestRun_Deterministic(t *testing.T) {
	a, b := filepath.Join(t.TempDir(), "a"), filepath.Join(t.TempDir(), "b")
	runCLI(t, a)
	runCLI(t, b)
	sa, err := scan(a)
	if err != nil {
		t.Fatal(err)
	}
	sb, err := scan(b)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(sa.sizes, sb.sizes) {
		t.Fatalf("ファイルの集合が違う: %v / %v", sa.sizes, sb.sizes)
	}
	for p := range sa.sizes {
		x, _ := os.ReadFile(filepath.Join(a, p))
		y, _ := os.ReadFile(filepath.Join(b, p))
		if !bytes.Equal(x, y) {
			t.Fatalf("%s: 中身が違う", p)
		}
	}
}

// TestCheckLimits は上限を超えるファイルが 1 つあれば 1 件、ファイル数の超過も 1 件として出ることを確かめる。
func TestCheckLimits(t *testing.T) {
	dir := t.TempDir()
	for name, n := range map[string]int{"a.bin": 10, "b.bin": 11, "sub/c.json": 3} {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, make([]byte, n), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	st, err := scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	if st.files != 3 || st.maxPath != "b.bin" || st.maxBytes != 11 {
		t.Fatalf("scan: %+v", st)
	}
	if v := checkLimits(st, 3, 11); len(v) != 0 {
		t.Fatalf("上限ちょうどは違反でない: %v", v)
	}
	if v := checkLimits(st, 3, 10); len(v) != 1 || !strings.HasPrefix(v[0], "b.bin:") {
		t.Fatalf("大きすぎるファイル 1 つで 1 件: %v", v)
	}
	if v := checkLimits(st, 2, 11); len(v) != 1 || !strings.HasPrefix(v[0], "ファイル数") {
		t.Fatalf("ファイル数の超過で 1 件: %v", v)
	}
}

// TestRun_RefusesNonEmptyOutput は前回の出力が混ざらないよう、空でない出力先を誤りにすることを確かめる。
func TestRun_RefusesNonEmptyOutput(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "stale.bin"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	var w bytes.Buffer
	if err := run(&w, testMovesDir, dir, "ryu"); err == nil {
		t.Fatal("空でない出力先で誤りにならない")
	}
}

// TestRun_UnknownCharacter は存在しないキャラの指定を誤りにすることを確かめる。
func TestRun_UnknownCharacter(t *testing.T) {
	var w bytes.Buffer
	if err := run(&w, testMovesDir, filepath.Join(t.TempDir(), "o"), "ryu,no_such"); err == nil {
		t.Fatal("存在しないキャラで誤りにならない")
	}
}

// TestRun_NoTables は、出力に繋ぎ表(sp/)と表の項目が無いことを確かめる(S6 で表の仕組みを消した。
// S5 後の開発者決定 2。表を配らない＝計算方式 C)。
func TestRun_NoTables(t *testing.T) {
	out := filepath.Join(t.TempDir(), "site")
	runCLI(t, out)
	if _, err := os.Stat(filepath.Join(out, "sp")); !os.IsNotExist(err) {
		t.Fatalf("sp/ ができている(err=%v)", err)
	}
	raw, err := os.ReadFile(filepath.Join(out, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"table"`, `"tableBytes"`, `"tableFormatVersion"`, `"units"`, `"rows"`} {
		if bytes.Contains(raw, []byte(key)) {
			t.Errorf("manifest に %s がある", key)
		}
	}
}
