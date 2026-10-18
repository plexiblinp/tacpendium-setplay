package setplay

import (
	"context"
	"reflect"
	"testing"

	"github.com/plexiblinp/tacpendium-setplay/internal/model"
	setplayrepo "github.com/plexiblinp/tacpendium-setplay/internal/repository/setplay"
)

// M19-02 グルー/API テスト(§5.2)。

// filler として使える技を並べた共通セット。target は unique の tgt。
// meaty KA=40 target startup=20 active=4 → budgetMax=21・sumF∈[18,21] に total を合わせる。
func fillerProbeMoves() []setplayrepo.MoveCandidate {
	return []setplayrepo.MoveCandidate{
		{ID: 1, Code: "normal18", Category: model.MoveCategoryNormal, Startup: ptr(5), Active: ptr(3), Total: ptr(18), Damage: ptr(300)},
		{ID: 2, Code: "derived19", Category: model.MoveCategoryUnique, Startup: ptr(6), Active: ptr(3), Total: ptr(19), Damage: ptr(300), IsDerived: true},
		{ID: 3, Code: "rush20", Category: model.MoveCategoryRushVariant, OriginalMoveID: ptr64(1), Startup: ptr(7), Active: ptr(3), Total: ptr(20), Damage: ptr(300), IsDerived: true},
		{ID: 4, Code: "setuponly21", Category: model.MoveCategoryNormal, Startup: ptr(8), Active: ptr(3), Total: ptr(21), Damage: ptr(300), SetupOnly: true},
		{ID: 5, Code: "tc20", Category: model.MoveCategoryTargetCombo, Startup: ptr(9), Active: ptr(3), Total: ptr(20), Damage: ptr(300)},
		{ID: 6, Code: "tgt", Category: model.MoveCategoryUnique, Startup: ptr(20), Active: ptr(4), Total: ptr(42), Damage: ptr(600)},
		// M19-05 ゲート 1: target_combo でも is_derived=1(前段のヒット/ガードが要る)なら除外され続ける。
		// tc20(is_derived=0 ＝ 空振りでも出る)と対で固定するために置く。
		{ID: 7, Code: "tc_derived21", Category: model.MoveCategoryTargetCombo, Startup: ptr(9), Active: ptr(3), Total: ptr(21), Damage: ptr(300), IsDerived: true},
	}
}

func fillerCodesUsed(res *SuggestResult) map[string]bool {
	used := map[string]bool{}
	for _, p := range res.Proposals {
		for _, st := range p.Steps {
			if st.Role == RoleFiller {
				used[st.Code] = true
			}
		}
	}
	return used
}

// M19-05 ゲート 1(§2.1): filler の除外条件を category 単独から
// 「category = 'target_combo' AND is_derived = 1」へ精密化した。
// ★「category を外す」ではなく「category を is_derived へ置き換える」——緩和側と維持側を対で固定する
// (SUPP-001 §5.5.2 (3)。維持側だけを見ていると、条件を広く当てすぎても検出できない)。
func TestRefine_FillerGate1_TargetComboSplitByIsDerived(t *testing.T) {
	svc := newService(newCombo(ptr(40)), fillerProbeMoves(), nil)
	res, err := svc.SuggestForCombo(context.Background(), 1, SuggestParams{TargetTypes: allTypes()})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Proposals) == 0 {
		t.Fatal("expected proposals")
	}
	used := fillerCodesUsed(res)
	// 維持: 前段のヒット/ガードが要る行(パターン 1/2)は引き続き除外される。
	if used["tc_derived21"] {
		t.Error("target_combo かつ is_derived=1 の技は filler にしてはならない")
	}
	// 緩和: 空振りでも出る行(パターン 3)は候補に入る。
	if !used["tc20"] {
		t.Error("target_combo でも is_derived=0 なら filler に使えること(M19-DESIGN-08 §1)")
	}
}

// M19-05 ゲート 2(§2.1): 単独入力不可(startup_basis IN ('standalone','unknown') かつ親参照あり)を除外する。
// ★親参照を持たない同型の行は除外されないことを対で固定する。
func TestRefine_FillerGate2_SoloUnavailableExcluded(t *testing.T) {
	moves := []setplayrepo.MoveCandidate{
		{ID: 1, Code: "plain18", Category: model.MoveCategoryNormal, Startup: ptr(5), Active: ptr(3), Total: ptr(18), Damage: ptr(300), StartupBasis: model.MoveStartupBasisStandalone},
		// 親参照あり × standalone → 除外。
		{ID: 2, Code: "solo_standalone19", Category: model.MoveCategorySpecial, Startup: ptr(6), Active: ptr(3), Total: ptr(19), Damage: ptr(300), StartupBasis: model.MoveStartupBasisStandalone},
		// 親参照あり × unknown → 除外(CHANGE-093／D-227 で 'standalone' のみから拡張された)。
		{ID: 3, Code: "solo_unknown20", Category: model.MoveCategorySpecial, Startup: ptr(6), Active: ptr(3), Total: ptr(20), Damage: ptr(300), StartupBasis: model.MoveStartupBasisUnknown},
		// 親参照あり × through → 除外しない(通し値なので意味が確定している)。
		{ID: 4, Code: "child_through21", Category: model.MoveCategorySpecial, Startup: ptr(6), Active: ptr(3), Total: ptr(21), Damage: ptr(300), StartupBasis: model.MoveStartupBasisThrough},
		// 親参照なし × standalone → 除外しない(述語の片輪だけでは落ちない)。
		{ID: 5, Code: "noparent_standalone21", Category: model.MoveCategorySpecial, Startup: ptr(7), Active: ptr(3), Total: ptr(21), Damage: ptr(300), StartupBasis: model.MoveStartupBasisStandalone},
		{ID: 9, Code: "tgt", Category: model.MoveCategoryUnique, Startup: ptr(20), Active: ptr(4), Total: ptr(42), Damage: ptr(600), StartupBasis: model.MoveStartupBasisStandalone},
	}
	derivations := []setplayrepo.MoveDerivation{
		{ChildMoveID: 2, ParentMoveID: 1},
		{ChildMoveID: 3, ParentMoveID: 1},
		{ChildMoveID: 4, ParentMoveID: 1},
	}
	svc := newServiceWithDerivations(newCombo(ptr(40)), moves, derivations, nil)
	res, err := svc.SuggestForCombo(context.Background(), 1, SuggestParams{TargetTypes: allTypes()})
	if err != nil {
		t.Fatal(err)
	}
	used := fillerCodesUsed(res)
	for _, code := range []string{"solo_standalone19", "solo_unknown20"} {
		if used[code] {
			t.Errorf("単独入力不可の技を単独 filler にしてはならない: %s", code)
		}
	}
	for _, code := range []string{"plain18", "child_through21", "noparent_standalone21"} {
		if !used[code] {
			t.Errorf("述語に該当しない技は filler のまま残ること: %s", code)
		}
	}
}

// §4.1「除外しないもの」: is_derived / rush_variant / setup_only は filler に含まれる。
func TestRefine_FillerIncludesDerivedRushSetupOnly(t *testing.T) {
	svc := newService(newCombo(ptr(40)), fillerProbeMoves(), nil)
	res, err := svc.SuggestForCombo(context.Background(), 1, SuggestParams{TargetTypes: allTypes()})
	if err != nil {
		t.Fatal(err)
	}
	used := fillerCodesUsed(res)
	for _, code := range []string{"normal18", "derived19", "rush20", "setuponly21"} {
		if !used[code] {
			t.Errorf("filler %s must be usable (is_derived/rush/setup_only are not excluded)", code)
		}
	}
}

// §4.2: 負 KA は 200 相当(エラーなし)+ 専用理由コード + items 空。
func TestRefine_NegativeKA(t *testing.T) {
	svc := newService(newCombo(ptr(-5)), fillerProbeMoves(), nil)
	res, err := svc.SuggestForCombo(context.Background(), 1, SuggestParams{TargetTypes: allTypes()})
	if err != nil {
		t.Fatalf("negative KA must not error (200 semantics), got %v", err)
	}
	if res.Reason != ReasonKnockdownNegative {
		t.Errorf("want Reason=%q, got %q", ReasonKnockdownNegative, res.Reason)
	}
	if len(res.Proposals) != 0 {
		t.Errorf("negative KA must yield 0 proposals, got %d", len(res.Proposals))
	}
}

// §4.3: totalFound が返り、limit で切ると totalFound>len(items) かつ truncated=false。
func TestRefine_TotalFoundAndLimitCut(t *testing.T) {
	svc := newService(newCombo(ptr(40)), fillerProbeMoves(), nil)
	res, err := svc.SuggestForCombo(context.Background(), 1, SuggestParams{TargetTypes: allTypes(), Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if res.TotalFound <= 1 {
		t.Fatalf("need TotalFound>1 to test cut, got %d", res.TotalFound)
	}
	if len(res.Proposals) != 1 {
		t.Errorf("limit=1 must return 1 item, got %d", len(res.Proposals))
	}
	if res.TotalFound <= len(res.Proposals) {
		t.Errorf("TotalFound(%d) must exceed returned(%d)", res.TotalFound, len(res.Proposals))
	}
	if res.Truncated {
		t.Error("limit cut must NOT set truncated (only safety cap does)")
	}
}

// §4.3: 安全上限に到達したときのみ truncated=true。
func TestRefine_SafetyCapTruncates(t *testing.T) {
	saved := engineSafetyCap
	engineSafetyCap = 2 // 意図的に極小化して打ち切りを起こす。
	defer func() { engineSafetyCap = saved }()
	svc := newService(newCombo(ptr(40)), fillerProbeMoves(), nil)
	res, err := svc.SuggestForCombo(context.Background(), 1, SuggestParams{TargetTypes: allTypes()})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Truncated {
		t.Error("reaching engine safety cap must set truncated=true")
	}
}

// §4.3 中核: グルーが全解をランキングしてから limit で切っている
// (少手数順ではなく、rank 全体の上位 limit 件と一致する)。
func TestRefine_RankThenCut(t *testing.T) {
	moves := fillerProbeMoves()
	svc := newService(newCombo(ptr(40)), moves, nil)
	full, err := svc.SuggestForCombo(context.Background(), 1, SuggestParams{TargetTypes: allTypes(), Sort: SortByN, Limit: maxLimit})
	if err != nil {
		t.Fatal(err)
	}
	if len(full.Proposals) < 3 {
		t.Fatalf("need >=3 proposals, got %d", len(full.Proposals))
	}
	// full は N 降順であること。
	for i := 1; i < len(full.Proposals); i++ {
		if full.Proposals[i-1].N < full.Proposals[i].N {
			t.Fatalf("full result must be N-desc")
		}
	}
	small, err := svc.SuggestForCombo(context.Background(), 1, SuggestParams{TargetTypes: allTypes(), Sort: SortByN, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	// 返却分は full の上位 limit 件と一致(=ランキング後に切っている)。
	shape := func(ps []Proposal) [][]string {
		out := make([][]string, len(ps))
		for i, p := range ps {
			codes := make([]string, len(p.Steps))
			for j, s := range p.Steps {
				codes[j] = s.Code
			}
			out[i] = codes
		}
		return out
	}
	if !reflect.DeepEqual(shape(small.Proposals), shape(full.Proposals[:2])) {
		t.Errorf("limited result must equal top-2 of ranked full:\n small=%v\n full[:2]=%v", shape(small.Proposals), shape(full.Proposals[:2]))
	}
}

// §4.4: gap モードで n_min が無視され、G が付与される(G=N−active・完全空振り)。sort=n は G 昇順。
func TestRefine_GapMode(t *testing.T) {
	svc := newService(newCombo(ptr(40)), fillerProbeMoves(), nil)
	res, err := svc.SuggestForCombo(context.Background(), 1, SuggestParams{TargetTypes: allTypes(), Mode: ModeGap, GMin: 1, GMax: 13, Sort: SortByN})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Proposals) == 0 {
		t.Fatal("expected gap proposals")
	}
	for _, p := range res.Proposals {
		if p.Mode != ModeGap {
			t.Errorf("mode must be gap, got %q", p.Mode)
		}
		// G = N − active（最終 active が起き上がりの G フレーム前＝完全空振り）。
		if p.G != p.N-p.TargetActive {
			t.Errorf("G(%d) must equal N-active(%d)", p.G, p.N-p.TargetActive)
		}
		if p.G < 1 || p.G > 13 {
			t.Errorf("G=%d out of [1,13]", p.G)
		}
		// meaty(N ≤ active)と排他＝N > active。
		if p.N <= p.TargetActive {
			t.Errorf("gap must be disjoint from meaty: N(%d) must exceed active(%d)", p.N, p.TargetActive)
		}
		// 完全空振り: 最終 active(S+active−1)が起き上がり(Landing=KA+1)より前。
		if p.S+p.TargetActive-1 >= p.Landing {
			t.Errorf("gap must fully whiff before wake-up: lastActive(%d) < landing(%d)", p.S+p.TargetActive-1, p.Landing)
		}
	}
	// sort=n = G 昇順（隙間が小さい順。target ごとに active が異なるため G を直接比較）。
	for i := 1; i < len(res.Proposals); i++ {
		if res.Proposals[i-1].G > res.Proposals[i].G {
			t.Errorf("gap sort=n must be G-ascending, got %d before %d", res.Proposals[i-1].G, res.Proposals[i].G)
		}
	}
}

// §4.4: g_max>13→13・g_min<1→1・g_min>g_max→空。
func TestRefine_GapRounding(t *testing.T) {
	svc := newService(newCombo(ptr(40)), fillerProbeMoves(), nil)
	// g_max=999 は 13 に丸め、g_min=0 は 1 に丸め → G∈[1,13]。
	res, _ := svc.SuggestForCombo(context.Background(), 1, SuggestParams{TargetTypes: allTypes(), Mode: ModeGap, GMin: 0, GMax: 999})
	for _, p := range res.Proposals {
		if p.G < 1 || p.G > 13 {
			t.Errorf("rounded G must lie in [1,13], got %d", p.G)
		}
	}
	// g_min>g_max → 空。
	empty, _ := svc.SuggestForCombo(context.Background(), 1, SuggestParams{TargetTypes: allTypes(), Mode: ModeGap, GMin: 10, GMax: 3})
	if len(empty.Proposals) != 0 {
		t.Errorf("g_min>g_max must yield empty, got %d", len(empty.Proposals))
	}
}

// §4.4: meaty モードで g_min/g_max が無視される(挙動が既定と同じ)。
func TestRefine_MeatyIgnoresGParams(t *testing.T) {
	svc := newService(newCombo(ptr(40)), fillerProbeMoves(), nil)
	base, _ := svc.SuggestForCombo(context.Background(), 1, SuggestParams{TargetTypes: allTypes()})
	withG, _ := svc.SuggestForCombo(context.Background(), 1, SuggestParams{TargetTypes: allTypes(), GMin: 5, GMax: 9})
	if base.TotalFound != withG.TotalFound {
		t.Errorf("meaty must ignore g params: base=%d withG=%d", base.TotalFound, withG.TotalFound)
	}
	for _, p := range withG.Proposals {
		if p.Mode != ModeMeaty {
			t.Errorf("mode must default to meaty, got %q", p.Mode)
		}
	}
}

// §4.4: mode 省略時は meaty。
func TestRefine_ModeOmittedIsMeaty(t *testing.T) {
	svc := newService(newCombo(ptr(40)), fillerProbeMoves(), nil)
	res, err := svc.SuggestForCombo(context.Background(), 1, SuggestParams{TargetTypes: allTypes()})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Proposals) == 0 {
		t.Fatal("expected proposals")
	}
	for _, p := range res.Proposals {
		if p.Mode != ModeMeaty || p.G != 0 {
			t.Errorf("omitted mode must be meaty with G=0, got mode=%q G=%d", p.Mode, p.G)
		}
	}
}

// §4.6-2: 明示 target_move_id 指定でも category=system は BE で除外される。
func TestRefine_ExplicitTargetExcludesSystem(t *testing.T) {
	moves := []setplayrepo.MoveCandidate{
		{ID: 1, Code: "f18", Category: model.MoveCategoryNormal, Startup: ptr(5), Active: ptr(3), Total: ptr(18), Damage: ptr(300)},
		{ID: 2, Code: "drive_parry", Category: model.MoveCategorySystem, Startup: ptr(1), Active: ptr(12), Total: ptr(45), Damage: ptr(0)},
	}
	sysID := int64(2)
	svc := newService(newCombo(ptr(40)), moves, nil)
	res, err := svc.SuggestForCombo(context.Background(), 1, SuggestParams{TargetMoveID: &sysID})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Proposals) != 0 {
		t.Errorf("explicit system target must be excluded in BE, got %d proposals", len(res.Proposals))
	}
}

// n_max(N の上限・meaty のみ)で持続の長い技の深い N を切り落とす。
func TestRefine_NMaxCapsActiveFrame(t *testing.T) {
	// 持続の長い target（active=10。波動拳的）＋ N=1..10 を各 1 filler で作れる
	// total=30..39 の filler 群（startup=nil で target にはならない）。
	// KA=40・target startup=2 → N = 40 − sumF、sumF∈[30,39] で N∈[1,10]。
	moves := []setplayrepo.MoveCandidate{
		{ID: 100, Code: "long_active", Category: model.MoveCategoryNormal, Startup: ptr(2), Active: ptr(10), Total: ptr(200), Damage: ptr(600)},
	}
	for i := 0; i <= 9; i++ {
		moves = append(moves, setplayrepo.MoveCandidate{
			ID: int64(i), Code: "f" + string(rune('a'+i)), Category: model.MoveCategoryNormal,
			Total: ptr(30 + i), Damage: ptr(0), // startup/active=nil → filler 専用（target にならない）
		})
	}
	svc := newService(newCombo(ptr(40)), moves, nil)

	// 上限なし: N は 1..10 まで出る。
	full, err := svc.SuggestForCombo(context.Background(), 1, SuggestParams{TargetTypes: []string{TargetTypeNormal}, Limit: maxLimit})
	if err != nil {
		t.Fatal(err)
	}
	// n_max=3: どの提案も N<=3。
	capped, err := svc.SuggestForCombo(context.Background(), 1, SuggestParams{TargetTypes: []string{TargetTypeNormal}, NMax: 3, Limit: maxLimit})
	if err != nil {
		t.Fatal(err)
	}
	if capped.TotalFound >= full.TotalFound {
		t.Errorf("n_max=3 should reduce candidates: full=%d capped=%d", full.TotalFound, capped.TotalFound)
	}
	for _, p := range capped.Proposals {
		if p.N > 3 {
			t.Errorf("n_max=3 must exclude N>3, got N=%d", p.N)
		}
	}
	if len(capped.Proposals) == 0 {
		t.Error("n_max=3 should still yield some proposals")
	}
}

// n_max < n_min は空結果（エラーにしない）。
func TestRefine_NMaxBelowNMinEmpty(t *testing.T) {
	svc := newService(newCombo(ptr(40)), fillerProbeMoves(), nil)
	res, err := svc.SuggestForCombo(context.Background(), 1, SuggestParams{TargetTypes: allTypes(), NMin: 5, NMax: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Proposals) != 0 {
		t.Errorf("n_max<n_min must yield empty, got %d", len(res.Proposals))
	}
}

// n_max はエンジンの meaty 恒等式を壊さない（N=KA+2−S のまま）。
func TestRefine_NMaxKeepsIdentity(t *testing.T) {
	svc := newService(newCombo(ptr(40)), fillerProbeMoves(), nil)
	res, err := svc.SuggestForCombo(context.Background(), 1, SuggestParams{TargetTypes: allTypes(), NMax: 4})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range res.Proposals {
		if p.Landing != 41 { // KA+1
			t.Errorf("landing must remain KA+1=41, got %d", p.Landing)
		}
		if p.Mode != ModeMeaty || p.G != 0 {
			t.Errorf("n_max is meaty-only; mode/G unexpected: %q/%d", p.Mode, p.G)
		}
	}
}

// gap モードでは n_max は無視される（帯は G で決まる）。
func TestRefine_GapIgnoresNMax(t *testing.T) {
	svc := newService(newCombo(ptr(40)), fillerProbeMoves(), nil)
	base, _ := svc.SuggestForCombo(context.Background(), 1, SuggestParams{TargetTypes: allTypes(), Mode: ModeGap, GMin: 1, GMax: 13})
	withNMax, _ := svc.SuggestForCombo(context.Background(), 1, SuggestParams{TargetTypes: allTypes(), Mode: ModeGap, GMin: 1, GMax: 13, NMax: 2})
	if base.TotalFound != withNMax.TotalFound {
		t.Errorf("gap must ignore n_max: base=%d withNMax=%d", base.TotalFound, withNMax.TotalFound)
	}
}
