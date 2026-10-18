package engine

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 本家の golden_test.go(GC-1〜5)を、合成の fixture ではなく moves DB の実データで回す。
//
// 本家の GC は fixture に実データのフレーム値を書き写したものである。単体版では data/moves を読み、
// エンジンの入口(ParseQuery → Suggest → Response)を通す。期待値(N ・着弾 ・S)は本家の表と同じ。
//
// ★GC-2 の既知の限界(本家の golden_test.go の注記と同じ): 実データの ken/thunder_kick には
// move_derivations の親(quick_dash)が無いため、実データでは奮迅脚が前置されず
// 「立ち弱P > 紫電カカト落とし」と表記される(S ・N は同じ)。本家の fixture 版(親を与えたもの)は
// internal/service/setplay/golden_test.go がそのまま回す。

type realGoldenCase struct {
	name      string
	character string
	ka        int
	query     string
	wantSteps []string
	wantN     int
	wantLand  int
	wantS     int
}

func TestGoldenCases_RealData(t *testing.T) {
	cases := []realGoldenCase{
		{"GC-1 Ryu 立ち弱K>鎖骨割り", "ryu", 40, "target_types=unique&limit=3000",
			[]string{"standing_light_kick", "collarbone_breaker"}, 4, 41, 38},
		{"GC-2 Ken 立ち弱P>紫電カカト落とし(実データは親なし)", "ken", 43, "target_types=unique&limit=3000",
			[]string{"standing_light_punch", "thunder_kick"}, 3, 44, 42},
		{"GC-3 Juri 立ち中P>前投げ", "juri", 27, "target_types=throw&limit=3000",
			[]string{"standing_medium_punch", "throw_forward"}, 3, 28, 26},
		{"GC-4 Terry しゃがみ弱K>立ち中K", "terry", 27, "target_types=normal&limit=3000",
			[]string{"crouching_light_kick", "standing_medium_kick"}, 3, 28, 26},
		{"GC-5 Guile ニーバズーカ>立ち中P", "guile", 34, "target_types=normal&limit=3000",
			[]string{"knee_bazooka", "standing_medium_punch"}, 2, 35, 34},
	}
	for _, gc := range cases {
		t.Run(gc.name, func(t *testing.T) {
			e := New()
			b, err := os.ReadFile(filepath.Join(movesDir, gc.character+".json"))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := e.Load(b); err != nil {
				t.Fatal(err)
			}
			v, err := url.ParseQuery(gc.query)
			if err != nil {
				t.Fatal(err)
			}
			q, err := ParseQuery(v)
			if err != nil {
				t.Fatal(err)
			}
			res, err := e.Suggest(context.Background(), gc.character, gc.ka, q)
			if err != nil {
				t.Fatal(err)
			}
			want := strings.Join(gc.wantSteps, ">")
			var found *Item
			for i := range res.Items {
				codes := make([]string, len(res.Items[i].Steps))
				for j, st := range res.Items[i].Steps {
					codes[j] = st.Code
				}
				if strings.Join(codes, ">") == want {
					found = &res.Items[i]
					break
				}
			}
			if found == nil {
				t.Fatalf("期待のレシピ %s が %d 件の中に無い(totalFound %d)", want, len(res.Items), res.TotalFound)
			}
			if found.N != gc.wantN || found.Landing != gc.wantLand || found.S != gc.wantS {
				t.Errorf("N/着弾/S = %d/%d/%d, want %d/%d/%d", found.N, found.Landing, found.S, gc.wantN, gc.wantLand, gc.wantS)
			}
			if res.Truncated {
				t.Errorf("golden case は打ち切られないはず")
			}
		})
	}
}
