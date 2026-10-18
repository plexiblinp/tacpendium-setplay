package engine

import (
	"errors"
	"net/url"
	"reflect"
	"testing"

	"github.com/plexiblinp/tacpendium-setplay/internal/service/setplay"
)

func parse(t *testing.T, raw string) (Query, error) {
	t.Helper()
	v, err := url.ParseQuery(raw)
	if err != nil {
		t.Fatal(err)
	}
	return ParseQuery(v)
}

// 省略時の既定は本家ハンドラと同じ(mode=meaty ・sort=n ・既定の種別 ・絞り込みなし ・数値は 0＝サービスが丸める)。
func TestParseQuery_Defaults(t *testing.T) {
	q, err := parse(t, "")
	if err != nil {
		t.Fatal(err)
	}
	// 既定の種別は本家の defaultTargetTypes の値を書き写して比べる(DefaultTargetTypes と比べると同語反復になる)。
	want := setplay.SuggestParams{Mode: setplay.ModeMeaty, Sort: setplay.SortByN, TargetTypes: []string{"normal", "unique", "special_projectile"}}
	if !reflect.DeepEqual(q.Params, want) || q.TargetMove != "" {
		t.Errorf("既定 = %+v / %q, want %+v", q.Params, q.TargetMove, want)
	}
}

func TestParseQuery_AllKeys(t *testing.T) {
	q, err := parse(t, "n_min=2&n_max=5&sort=target&target_types=normal,+throw,,&include_zero_damage=true"+
		"&approach_only=1&block_safe_only=true&chain_cancel_only=T&target_move=hadoken_light&mode=filler&remain=3"+
		"&g_min=-4&g_max=99&limit=5000")
	if err != nil {
		t.Fatal(err)
	}
	want := setplay.SuggestParams{
		NMin: 2, NMax: 5, Sort: setplay.SortByTarget, TargetTypes: []string{"normal", "throw"},
		IncludeZeroDamage: true, ApproachOnly: true, BlockSafeOnly: true, ChainCancelOnly: true,
		Mode: setplay.ModeFiller, Remain: 3, GMin: -4, GMax: 99, Limit: 5000,
	}
	if !reflect.DeepEqual(q.Params, want) || q.TargetMove != "hadoken_light" {
		t.Errorf("got %+v / %q, want %+v", q.Params, q.TargetMove, want)
	}
}

// remain は filler のときだけ読む(本家と同じ。他のモードでは不正な値でも誤りにしない)。
func TestParseQuery_RemainOnlyForFiller(t *testing.T) {
	q, err := parse(t, "mode=gap&remain=abc")
	if err != nil || q.Params.Remain != 0 {
		t.Errorf("gap の remain は読まない: %+v %v", q.Params, err)
	}
}

func TestParseQuery_Errors(t *testing.T) {
	cases := map[string]string{
		"n_min=x":                 QueryNMin,
		"sort=zzz":                QuerySort,
		"include_zero_damage=yes": QueryIncludeZeroDmg,
		"approach_only=maybe":     QueryApproachOnly,
		"block_safe_only=2":       QueryBlockSafeOnly,
		"chain_cancel_only=no!":   QueryChainCancelOnly,
		"mode=xxx":                QueryMode,
		"mode=filler":             QueryRemain,
		"mode=filler&remain=-1":   QueryRemain,
		"mode=filler&remain=1.5":  QueryRemain,
		"n_max=a":                 QueryNMax,
		"g_min=a":                 QueryGMin,
		"g_max=a":                 QueryGMax,
		"limit=1.5":               QueryLimit,
	}
	for raw, param := range cases {
		_, err := parse(t, raw)
		var qe *QueryError
		if !errors.As(err, &qe) || qe.Param != param {
			t.Errorf("%q: want QueryError(%s), got %v", raw, param, err)
		}
	}
}
