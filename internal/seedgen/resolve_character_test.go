package seedgen

import "testing"

// 表記の解決を分けた関数のうち、公開の生成関数からは届かない分岐を直接呼んで固定する。
// 出力全体は resolve_character_golden_test.go が守る。

// TestSortResolved_UnfilledTieBreaksByReason は同じ move_code の穴が理由の順に並ぶことを固定する。
//
// ★公開の生成関数では届かない(同じキャラの中で move_code が重なる入力は validate が止める)。
func TestSortResolved_UnfilledTieBreaksByReason(t *testing.T) {
	unfilled := []UnfilledRow{
		{MoveCode: "b_move", Reason: ReasonNoCommand},
		{MoveCode: "a_move", Reason: ReasonRushNoOriginal},
		{MoveCode: "a_move", Reason: ReasonCollision},
	}
	collisions := []AliasCollision{{AliasText: "6"}, {AliasText: "2LP"}}
	base := map[string]AliasRow{"z_move": {MoveCode: "z_move"}, "c_move": {MoveCode: "c_move"}}
	rush := map[string]AliasRow{"rush_a": {MoveCode: "rush_a"}}

	out := sortResolved(base, rush, unfilled, collisions)

	wantOut := []string{"c_move", "rush_a", "z_move"}
	if len(out) != len(wantOut) {
		t.Fatalf("out = %v, want move_code %v", out, wantOut)
	}
	for i, w := range wantOut {
		if out[i].MoveCode != w {
			t.Errorf("out[%d] = %s, want %s", i, out[i].MoveCode, w)
		}
	}
	wantUnfilled := []struct {
		code   string
		reason unfilledReason
	}{
		{"a_move", ReasonCollision}, {"a_move", ReasonRushNoOriginal}, {"b_move", ReasonNoCommand},
	}
	for i, w := range wantUnfilled {
		if unfilled[i].MoveCode != w.code || unfilled[i].Reason != w.reason {
			t.Errorf("unfilled[%d] = %s/%s, want %s/%s", i, unfilled[i].MoveCode, unfilled[i].Reason, w.code, w.reason)
		}
	}
	if collisions[0].AliasText != "2LP" || collisions[1].AliasText != "6" {
		t.Errorf("collisions = %v, want 表記の昇順", collisions)
	}
}

// TestResolveLayerARow_MissingTokenIsIndexSkip は、索引の外れの集合に無いのに正規化キーも無い行を
// index-skip の穴として記録することを固定する。
//
// ★公開の生成関数では届かない(command を持つ行は、索引に載るか外れの集合に入るかのどちらかである)。
func TestResolveLayerARow_MissingTokenIsIndexSkip(t *testing.T) {
	cr := &charResolver{
		rule: NumericRule, charCode: "ryu",
		tokenOf: map[string]string{}, indexSkipped: map[string]bool{},
		base: map[string]AliasRow{}, byMove: map[string]MoveRow{},
	}
	r := mkRow("ryu", "hadoken_light", "special", "弱波動拳", "d dr r plus p_l", false)
	cr.resolveLayerARow(r)

	if len(cr.base) != 0 {
		t.Errorf("表記が付いた: %v", cr.base)
	}
	if len(cr.unfilled) != 1 {
		t.Fatalf("unfilled = %v, want 1 件", cr.unfilled)
	}
	u := cr.unfilled[0]
	if u.MoveCode != "hadoken_light" || u.Reason != ReasonIndexSkip || u.Detail != r.Command {
		t.Errorf("unfilled = %+v, want hadoken_light/%s/%q", u, ReasonIndexSkip, r.Command)
	}
}
