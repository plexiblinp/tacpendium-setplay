package setplay

import (
	"context"
	"strings"
	"testing"

	"github.com/plexiblinp/tacpendium-setplay/internal/model"
	setplayrepo "github.com/plexiblinp/tacpendium-setplay/internal/repository/setplay"
)

// golden-case 実測(§5.3)。character_data/*.csv の実フレーム値を用い、N_min=1 で
// 期待レシピが提案に含まれ、N・着弾(landing)が期待表と一致することを検証する。
//
// 【M19-05】GC-2(ケン・派生技)を追加して 5/5 とした。through target の解禁と表記展開
// (費用規則 分類 1 = 表記用親の計上ゼロ)が入ったため通るようになった。
//
// | #    | キャラ | KA | 期待レシピ                          | N | 着弾 | S |
// |------|--------|----|-------------------------------------|---|------|---|
// | GC-1 | リュウ | 40 | 立ち弱K > 鎖骨割り                  | 4 | 41 | 38 |
// | GC-2 | ケン   | 43 | 立ち弱P > 奮迅脚 > 紫電カカト落とし | 3 | 44 | 42 |
// | GC-3 | ジュリ | 27 | 立ち中P > 前投げ                    | 3 | 28 | 26 |
// | GC-4 | テリー | 27 | しゃがみ弱K > 中K(立ち中K)          | 3 | 28 | 26 |
// | GC-5 | ガイル | 34 | ニーバズーカ > 立ち中P              | 2 | 35 | 34 |
//
// ★GC-2 の既知の限界(M19-05 完了報告にも記載):
//
//	(1) 本テストは fixture 側で thunder_kick に親参照(quick_dash)を与えている。
//	(2) 実データの ken/thunder_kick には move_derivations の親参照が無い
//	    (移動 system move / quick_dash を親に取れるかが D-233 の未裁定事項のため)。
//	(3) ⇒ 本ケースの green は、実データで奮迅脚が前置されることを保証していない。
//	    実データでは「立ち弱P > 紫電カカト落とし」と表記される(S・N は同じ)。
//	    followup gc2-thunder-kick-parent-reference。

type goldenCase struct {
	name        string
	ka          int
	moves       []setplayrepo.MoveCandidate
	derivations []setplayrepo.MoveDerivation
	types       []string
	// wantSteps は期待レシピの表記順(表記用親を含む)。
	wantSteps   []string
	wantN       int
	wantLanding int
	wantS       int
}

func TestGoldenCases(t *testing.T) {
	cases := []goldenCase{
		{
			name: "GC-1 Ryu 立ち弱K>鎖骨割り", ka: 40,
			moves: []setplayrepo.MoveCandidate{
				{ID: 1, Code: "standing_light_kick", Category: model.MoveCategoryNormal, Startup: ptr(5), Active: ptr(3), Total: ptr(18), Damage: ptr(300)},
				{ID: 2, Code: "collarbone_breaker", Category: model.MoveCategoryUnique, Startup: ptr(20), Active: ptr(4), Total: ptr(42), Damage: ptr(600)},
			},
			types:     []string{TargetTypeUnique}, // 鎖骨割り=特殊技(既定 ON)
			wantSteps: []string{"standing_light_kick", "collarbone_breaker"},
			wantN:     4, wantLanding: 41, wantS: 38,
		},
		{
			// GC-2 ケン: 奮迅脚(quick_dash)からの通し値 29F を持つ紫電カカト落とし(thunder_kick)。
			// 立ち弱P 空振り 13F → 紫電カカトが奮迅脚入力起点 29F 目に第 1 active。
			// S = 13 + 29 = 42。44F 目(=KA+1)に持続 3F 目が命中(N = 3 <= active 3)。
			// ★奮迅脚(total 45)は表記用親として前置されるが計上ゼロ(費用規則 分類 1)。
			//   45 を足すと S = 87 になり成立しない——「親計上ゼロ」の規約が効いていることの検算。
			name: "GC-2 Ken 立ち弱P>奮迅脚>紫電カカト落とし", ka: 43,
			moves: []setplayrepo.MoveCandidate{
				{ID: 1, Code: "standing_light_punch", Category: model.MoveCategoryNormal, Startup: ptr(4), Active: ptr(3), Total: ptr(13), Damage: ptr(300), StartupBasis: model.MoveStartupBasisStandalone},
				{ID: 2, Code: "quick_dash", Category: model.MoveCategoryUnique, Startup: ptr(1), Active: ptr(45), Total: ptr(45), Damage: ptr(0), StartupBasis: model.MoveStartupBasisStandalone},
				{ID: 3, Code: "thunder_kick", Category: model.MoveCategoryUnique, Startup: ptr(29), Active: ptr(3), Total: ptr(51), Damage: ptr(1000), IsDerived: true, StartupBasis: model.MoveStartupBasisThrough},
			},
			derivations: []setplayrepo.MoveDerivation{{ChildMoveID: 3, ParentMoveID: 2}},
			types:       []string{TargetTypeUnique}, // 紫電カカト落とし=特殊技(既定 ON)
			wantSteps:   []string{"standing_light_punch", "quick_dash", "thunder_kick"},
			wantN:       3, wantLanding: 44, wantS: 42,
		},
		{
			name: "GC-3 Juri 立ち中P>前投げ", ka: 27,
			moves: []setplayrepo.MoveCandidate{
				{ID: 1, Code: "standing_medium_punch", Category: model.MoveCategoryNormal, Startup: ptr(6), Active: ptr(4), Total: ptr(21), Damage: ptr(600)},
				{ID: 2, Code: "throw_forward", Category: model.MoveCategoryThrow, Startup: ptr(5), Active: ptr(3), Total: ptr(30), Damage: ptr(1200)},
			},
			types:     []string{TargetTypeThrow}, // 前投げ=投げ(既定 OFF・明示 ON で再現)
			wantSteps: []string{"standing_medium_punch", "throw_forward"},
			wantN:     3, wantLanding: 28, wantS: 26,
		},
		{
			name: "GC-4 Terry しゃがみ弱K>立ち中K", ka: 27,
			moves: []setplayrepo.MoveCandidate{
				{ID: 1, Code: "crouching_light_kick", Category: model.MoveCategoryNormal, Startup: ptr(5), Active: ptr(2), Total: ptr(17), Damage: ptr(200)},
				{ID: 2, Code: "standing_medium_kick", Category: model.MoveCategoryNormal, Startup: ptr(9), Active: ptr(3), Total: ptr(29), Damage: ptr(700)},
			},
			types:     []string{TargetTypeNormal}, // 立ち中K=通常技(既定 ON)
			wantSteps: []string{"crouching_light_kick", "standing_medium_kick"},
			wantN:     3, wantLanding: 28, wantS: 26,
		},
		{
			name: "GC-5 Guile ニーバズーカ>立ち中P", ka: 34,
			moves: []setplayrepo.MoveCandidate{
				{ID: 1, Code: "knee_bazooka", Category: model.MoveCategoryUnique, Startup: ptr(8), Active: ptr(5), Total: ptr(27), Damage: ptr(500)},
				{ID: 2, Code: "standing_medium_punch", Category: model.MoveCategoryNormal, Startup: ptr(7), Active: ptr(3), Total: ptr(24), Damage: ptr(600)},
			},
			types:     []string{TargetTypeNormal}, // 立ち中P=通常技(既定 ON)。filler ニーバズーカ(特殊技)
			wantSteps: []string{"knee_bazooka", "standing_medium_punch"},
			wantN:     2, wantLanding: 35, wantS: 34,
		},
	}

	for _, gc := range cases {
		t.Run(gc.name, func(t *testing.T) {
			svc := newServiceWithDerivations(newCombo(ptr(gc.ka)), gc.moves, gc.derivations, nil)
			res, err := svc.SuggestForCombo(context.Background(), 1, SuggestParams{NMin: 1, TargetTypes: gc.types})
			if err != nil {
				t.Fatal(err)
			}
			found := findProposal(res, gc.wantSteps...)
			if found == nil {
				t.Fatalf("expected recipe %s not found in %d proposals",
					strings.Join(gc.wantSteps, ">"), len(res.Proposals))
			}
			if found.N != gc.wantN {
				t.Errorf("N = %d, want %d", found.N, gc.wantN)
			}
			if found.Landing != gc.wantLanding {
				t.Errorf("landing = %d, want %d", found.Landing, gc.wantLanding)
			}
			if found.S != gc.wantS {
				t.Errorf("S = %d, want %d", found.S, gc.wantS)
			}
			if res.Truncated {
				t.Errorf("golden case should not truncate")
			}
		})
	}
}
