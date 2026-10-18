package setplay

import "testing"

func TestRecipeKey(t *testing.T) {
	steps := []ProposalStep{
		{Code: "parent_x", Role: RoleParent},
		{Code: "a", Role: RoleFiller, Counted: true},
		{Code: "b", Role: RoleFiller, Counted: true},
		{Code: "t", Role: RoleTarget, Counted: true},
	}
	if got := RecipeKey(steps); got != "parent_x,a,b|t" {
		t.Fatalf("%q", got)
	}
	// 繋ぎだけ(当てる技なし)と、同じ技を当てる技にした行は別の鍵。
	fillerOnly := []ProposalStep{{Code: "a", Role: RoleFiller}, {Code: "t", Role: RoleFiller}}
	meaty := []ProposalStep{{Code: "a", Role: RoleFiller}, {Code: "t", Role: RoleTarget}}
	if RecipeKey(fillerOnly) == RecipeKey(meaty) {
		t.Fatalf("繋ぎだけと当てる技の行が同じ鍵: %q", RecipeKey(meaty))
	}
}

func TestApplyVotes_StableAndNoopWhenEmpty(t *testing.T) {
	mk := func(codes ...string) []Proposal {
		out := make([]Proposal, len(codes))
		for i, c := range codes {
			out[i] = Proposal{Steps: []ProposalStep{{Code: c, Role: RoleTarget}}}
		}
		return out
	}
	order := func(ps []Proposal) string {
		s := ""
		for _, p := range ps {
			s += p.Steps[0].Code
		}
		return s
	}
	ps := mk("a", "b", "c", "d", "e")
	applyVotes(ps, nil)
	if order(ps) != "abcde" {
		t.Fatal(order(ps))
	}
	applyVotes(ps, map[string]int{"|d": 1, "|b": 1, "|a": -1})
	if order(ps) != "bdcea" {
		t.Fatalf("票の降順・同点は元の順: %s", order(ps))
	}
}
