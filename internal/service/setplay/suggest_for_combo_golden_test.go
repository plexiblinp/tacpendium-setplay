package setplay

import (
	"github.com/plexiblinp/tacpendium-setplay/internal/model"
	setplayrepo "github.com/plexiblinp/tacpendium-setplay/internal/repository/setplay"
)

// 単体版: 本家の suggest_for_combo_golden_test.go のうち、fixture の goldenMoves だけを移した
// (suggest_for_combo_branches_test.go が使う)。golden の本体(TestSuggestForComboGolden と
// testdata/suggest_for_combo_golden.json)は、出力に採用済みの印(alreadyAdopted)と重複判定の呼び出し順を
// 含むため移さない(単体版は採用済みを持たない。docs/upstream-diff.md)。
// 関数を分けた後も振る舞いが同じことは、engine の適合性テスト(本家の実装で作った正解との全件の突き合わせ)が見る。

func goldenMoves() ([]setplayrepo.MoveCandidate, []setplayrepo.MoveDerivation) {
	moves := []setplayrepo.MoveCandidate{
		{ID: 1, Code: "standing_light_punch", Category: model.MoveCategoryNormal, Startup: ptr(4), Active: ptr(3), Total: ptr(13), Damage: ptr(300), StartupBasis: model.MoveStartupBasisStandalone, ChainCancelTotal: ptr(9), OnBlock: ptr(-1)},
		{ID: 2, Code: "crouching_light_punch", Category: model.MoveCategoryNormal, Startup: ptr(4), Active: ptr(2), Total: ptr(14), Damage: ptr(300), StartupBasis: model.MoveStartupBasisStandalone, ChainCancelTotal: ptr(10), OnBlock: ptr(0)},
		{ID: 3, Code: "crouching_light_kick", Category: model.MoveCategoryNormal, Startup: ptr(5), Active: ptr(2), Total: ptr(16), Damage: ptr(300), StartupBasis: model.MoveStartupBasisStandalone, ChainCancelTotal: ptr(12), OnBlock: ptr(-3)},
		{ID: 4, Code: model.MoveCodeDashForward, Category: model.MoveCategorySystem, Total: ptr(19), StartupBasis: model.MoveStartupBasisStandalone},
		{ID: 5, Code: "quick_dash", Category: model.MoveCategoryUnique, Startup: ptr(1), Active: ptr(20), Total: ptr(24), Damage: ptr(0), StartupBasis: model.MoveStartupBasisStandalone},
		{ID: 6, Code: "thunder_kick", Category: model.MoveCategoryUnique, Startup: ptr(18), Active: ptr(3), Total: ptr(40), Damage: ptr(1000), IsDerived: true, StartupBasis: model.MoveStartupBasisThrough, OnBlock: ptr(-6)},
		{ID: 7, Code: "collarbone_breaker", Category: model.MoveCategoryUnique, Startup: ptr(20), Active: ptr(4), Total: ptr(42), Damage: ptr(600), StartupBasis: model.MoveStartupBasisStandalone, OnBlock: ptr(-1)},
		{ID: 8, Code: "hadoken_light", Category: model.MoveCategorySpecial, Startup: ptr(12), Active: ptr(6), Total: ptr(40), Damage: ptr(600), IsProjectile: true, StartupBasis: model.MoveStartupBasisStandalone, OnBlock: ptr(-4)},
		{ID: 9, Code: "throw_forward", Category: model.MoveCategoryThrow, Startup: ptr(5), Active: ptr(3), Total: ptr(30), Damage: ptr(1200), StartupBasis: model.MoveStartupBasisStandalone},
		{ID: 11, Code: "standing_medium_punch", Category: model.MoveCategoryNormal, Startup: ptr(6), Active: ptr(4), Total: ptr(21), Damage: ptr(600), StartupBasis: model.MoveStartupBasisStandalone, OnBlock: ptr(1)},
		{ID: 12, Code: "drive_rush_standing_medium_punch", Category: model.MoveCategoryRushVariant, OriginalMoveID: ptr64(11), Startup: ptr(14), Active: ptr(4), Total: ptr(29), Damage: ptr(600), IsDerived: true, StartupBasis: model.MoveStartupBasisStandalone, OnBlock: ptr(4)},
		{ID: 13, Code: "standing_heavy_kick", Category: model.MoveCategoryNormal, Startup: ptr(12), Active: ptr(4), Total: ptr(35), Damage: ptr(900), StartupBasis: model.MoveStartupBasisStandalone, FastestUnreachable: true},
		{ID: 14, Code: "collarbone_follow_up", Category: model.MoveCategoryUnique, Startup: ptr(8), Active: ptr(2), Total: ptr(20), Damage: ptr(400), IsDerived: true, StartupBasis: model.MoveStartupBasisStandalone},
		{ID: 15, Code: "drive_parry", Category: model.MoveCategoryNormal, Startup: ptr(1), Active: ptr(12), Total: ptr(33), StartupBasis: model.MoveStartupBasisStandalone},
		{ID: 16, Code: "no_total_move", Category: model.MoveCategoryNormal, Startup: ptr(7), Active: ptr(2), Damage: ptr(500), StartupBasis: model.MoveStartupBasisStandalone},
	}
	derivations := []setplayrepo.MoveDerivation{
		{ChildMoveID: 6, ParentMoveID: 5},
		{ChildMoveID: 14, ParentMoveID: 7},
	}
	return moves, derivations
}
