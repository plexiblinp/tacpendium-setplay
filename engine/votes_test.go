package engine

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// 票(S6・HANDOVER D17)の試験。票は本家の並べ替えの後にスコアの降順で安定に並べ直し、その後で limit を切る。

func loadedEngine(t *testing.T, codes ...string) *Engine {
	t.Helper()
	e := New()
	for _, c := range codes {
		b, err := os.ReadFile(filepath.Join(movesDir, c+".json"))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := e.Load(b); err != nil {
			t.Fatal(err)
		}
	}
	return e
}

func suggestItems(t *testing.T, e *Engine, code string, ka int, query, votes string) *Response {
	t.Helper()
	r := decodeSuggest(t, e.SuggestWithVotesJSON(code, ka, query, votes))
	if r.Error != nil || r.Result == nil {
		t.Fatalf("%s KA %d %q: %+v", code, ka, query, r.Error)
	}
	return r.Result
}

func keysOf(items []Item) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.RecipeKey
	}
	return out
}

// 票なし(省略 ・空文字 ・{})は、票の無い SuggestJSON とバイト列まで同じ(＝本家と同じ並び。golden ・本家との突き合わせが見る経路)。
func TestVotes_NoneKeepsUpstreamOrder(t *testing.T) {
	e := loadedEngine(t, "ryu")
	for _, q := range []string{"", "mode=gap&g_min=2", "mode=filler&remain=5", "sort=target&limit=3000"} {
		want := e.SuggestJSON("ryu", 43, q)
		for _, v := range []string{"", "{}", "null"} {
			if got := e.SuggestWithVotesJSON("ryu", 43, q, v); !bytes.Equal(got, want) {
				t.Errorf("%q votes=%q: 票なしと違う", q, v)
			}
		}
		// 結果のどれにも当たらない票も並びを変えない。
		if got := e.SuggestWithVotesJSON("ryu", 43, q, `{"no_such|move":5}`); !bytes.Equal(got, want) {
			t.Errorf("%q: 当たらない票で並びが変わった", q)
		}
	}
}

// RecipeKey は 1 つの結果の中で重ならない(重なると 1 票が複数の行に効く)。
func TestVotes_RecipeKeyUniqueWithinResult(t *testing.T) {
	e := loadedEngine(t, "ryu", "zangief", "jamie")
	for _, c := range []string{"ryu", "zangief", "jamie"} {
		for _, q := range []string{"limit=3000", "mode=gap&limit=3000", "mode=filler&remain=6&limit=3000",
			"target_types=normal,unique,special,special_projectile,throw&limit=3000"} {
			res := suggestItems(t, e, c, 50, q, "")
			seen := map[string]int{}
			for i, it := range res.Items {
				if it.RecipeKey == "" || !strings.Contains(it.RecipeKey, "|") {
					t.Fatalf("%s %q: 鍵の形 %q", c, q, it.RecipeKey)
				}
				if j, ok := seen[it.RecipeKey]; ok {
					t.Fatalf("%s %q: 鍵 %q が %d 番目と %d 番目で重なる", c, q, it.RecipeKey, j, i)
				}
				seen[it.RecipeKey] = i
			}
		}
	}
}

// 票は第一キー: 正の票の行は先頭へ、負の票の行は末尾へ。ほかの行は本家の順のまま。
func TestVotes_FirstSortKey(t *testing.T) {
	e := loadedEngine(t, "ryu")
	const q = "limit=3000"
	base := suggestItems(t, e, "ryu", 43, q, "")
	if len(base.Items) < 10 {
		t.Fatalf("件数が少なすぎる: %d", len(base.Items))
	}
	keys := keysOf(base.Items)
	up1, up2, down := keys[7], keys[3], keys[0]
	votes := fmt.Sprintf(`{%q:2,%q:1,%q:-1}`, up1, up2, down)
	got := keysOf(suggestItems(t, e, "ryu", 43, q, votes).Items)

	var rest []string
	for _, k := range keys {
		if k != up1 && k != up2 && k != down {
			rest = append(rest, k)
		}
	}
	want := append(append([]string{up1, up2}, rest...), down)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("票で並べ直した順が違う\n先頭 %v\n期待 %v", head(got), head(want))
	}
}

// 票は limit で切る前に効く(AGENTS §4「並べ替えの後で切る」): limit の外にある行も、票があれば先頭に入る。
func TestVotes_AppliedBeforeLimit(t *testing.T) {
	e := loadedEngine(t, "ryu")
	all := suggestItems(t, e, "ryu", 43, "limit=3000", "")
	if len(all.Items) <= 200 {
		t.Fatalf("limit 200 を超える条件が要る: %d 件", len(all.Items))
	}
	far := all.Items[len(all.Items)-1].RecipeKey
	top := suggestItems(t, e, "ryu", 43, "limit=200", "")
	for _, it := range top.Items {
		if it.RecipeKey == far {
			t.Fatal("前提: 最後の行は票なしの limit 200 に入らない")
		}
	}
	voted := suggestItems(t, e, "ryu", 43, "limit=200", fmt.Sprintf(`{%q:1}`, far))
	if len(voted.Items) != 200 || voted.Items[0].RecipeKey != far {
		t.Fatalf("票の付いた limit の外の行が先頭に来ない: %d 件 先頭 %q", len(voted.Items), voted.Items[0].RecipeKey)
	}
	if voted.TotalFound != top.TotalFound {
		t.Fatalf("totalFound が票で変わった: %d / %d", voted.TotalFound, top.TotalFound)
	}
	if !reflect.DeepEqual(keysOf(voted.Items[1:]), keysOf(top.Items[:199])) {
		t.Fatal("票の無い行の順が本家と違う")
	}
}

// 繋ぎだけ(安全飛び)の経路も票が効く。
func TestVotes_FillerOnly(t *testing.T) {
	e := loadedEngine(t, "ryu")
	const q = "mode=filler&remain=6&limit=3000"
	base := suggestItems(t, e, "ryu", 60, q, "")
	if len(base.Items) < 3 {
		t.Fatalf("件数が少なすぎる: %d", len(base.Items))
	}
	last := base.Items[len(base.Items)-1].RecipeKey
	if !strings.HasSuffix(last, "|") {
		t.Fatalf("繋ぎだけの鍵は当てる技が空: %q", last)
	}
	got := suggestItems(t, e, "ryu", 60, q, fmt.Sprintf(`{%q:3}`, last))
	if got.Items[0].RecipeKey != last {
		t.Fatalf("繋ぎだけで票が効かない: 先頭 %q", got.Items[0].RecipeKey)
	}
}

func TestParseVotes(t *testing.T) {
	ok, err := ParseVotes([]byte(`{"a,b|c":3,"d|":-2}`))
	if err != nil || !reflect.DeepEqual(ok, map[string]int{"a,b|c": 3, "d|": -2}) {
		t.Fatalf("%v %v", ok, err)
	}
	for _, bad := range []string{`[1]`, `{"a":1.5}`, `{"a":"1"}`, `{"":1}`, `{"a":1}{}`, `{"a":99999999999}`,
		`{"` + strings.Repeat("x", MaxRecipeKeyLen+1) + `":1}`} {
		if _, err := ParseVotes([]byte(bad)); err == nil {
			t.Errorf("%.40s: 誤りにならない", bad)
		}
	}
	var many strings.Builder
	many.WriteString("{")
	for i := 0; i <= MaxVoteEntries; i++ {
		if i > 0 {
			many.WriteString(",")
		}
		fmt.Fprintf(&many, `"k%d":1`, i)
	}
	many.WriteString("}")
	if _, err := ParseVotes([]byte(many.String())); err == nil {
		t.Error("件数の上限を超えても誤りにならない")
	}
	e := loadedEngine(t, "ryu")
	r := decodeSuggest(t, e.SuggestWithVotesJSON("ryu", 40, "", `{"a":"x"}`))
	if r.Error == nil || r.Error.Code != ErrCodeInvalidArguments {
		t.Fatalf("票の形の誤りは invalid_arguments: %+v", r)
	}
}

func head(s []string) []string {
	if len(s) > 12 {
		return s[:12]
	}
	return s
}
