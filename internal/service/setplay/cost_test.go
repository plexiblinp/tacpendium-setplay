package setplay

import (
	"context"
	"strings"
	"testing"

	"github.com/plexiblinp/tacpendium-setplay/internal/model"
	setplayrepo "github.com/plexiblinp/tacpendium-setplay/internal/repository/setplay"
)

// M19-05 段階 B: 費用規則 4 分類(M19-DESIGN-07 §2-1)と合成 filler 単位(同 §2-2)。
//
// | # | 条件                                            | cost(i)                |
// |---|-------------------------------------------------|------------------------|
// | 1 | i が target 直前の表記用親                       | 0                      |
// | 2 | i と i+1 が同一チェーングループ員として隣接      | chain_cancel_total(i)  |
// | 3 | i が target(最終ステップ)                        | startup(i)             |
// | 4 | その他(単発 filler・チェーン末端を含む)          | total(i)               |
//
// 恒等式 S = Σcost(i) / N = KA + 2 − S / G = N − active は不変。

func cctPtr(n int) *int { return &n }

// findProposal は表記順の code 列が recipe と一致する提案を返す。
func findProposal(res *SuggestResult, recipe ...string) *Proposal {
	want := strings.Join(recipe, ">")
	for i := range res.Proposals {
		got := make([]string, 0, len(res.Proposals[i].Steps))
		for _, st := range res.Proposals[i].Steps {
			got = append(got, st.Code)
		}
		if strings.Join(got, ">") == want {
			return &res.Proposals[i]
		}
	}
	return nil
}

// assertIdentities は恒等式が成立していることを確かめる。
// S は「計上される」ステップ費用の総和であり、Counted=false の親は寄与しない。
func assertIdentities(t *testing.T, p *Proposal, ka, wantS int) {
	t.Helper()
	if p.S != wantS {
		t.Errorf("S = %d, want %d", p.S, wantS)
	}
	if want := ka + 2 - p.S; p.N != want {
		t.Errorf("恒等式 N = KA + 2 − S を満たさない: N=%d, KA+2-S=%d", p.N, want)
	}
	if p.Mode == ModeGap {
		if want := p.N - p.TargetActive; p.G != want {
			t.Errorf("恒等式 G = N − active を満たさない: G=%d, N-active=%d", p.G, want)
		}
	}
	if want := ka + 1; p.Landing != want {
		t.Errorf("landing = %d, want KA+1 = %d", p.Landing, want)
	}
}

// TestCost_Class1_NotationParentIsFree は分類 1(表記用親の cost 0)を固定する。
//
// リュウ立弱P(total 13)を空振り → ケン相当の through target(startup 29・親 = 奮迅脚 total 45)。
// 親の total 45 は S に一切寄与せず、S = 13 + 29 = 42 のままであること。
func TestCost_Class1_NotationParentIsFree(t *testing.T) {
	const ka = 43
	moves := []setplayrepo.MoveCandidate{
		{ID: 1, Code: "standing_light_punch", Category: model.MoveCategoryNormal, Startup: ptr(4), Active: ptr(3), Total: ptr(13), Damage: ptr(300), StartupBasis: model.MoveStartupBasisStandalone},
		{ID: 2, Code: "quick_dash", Category: model.MoveCategoryUnique, Startup: ptr(1), Active: ptr(45), Total: ptr(45), Damage: ptr(0), StartupBasis: model.MoveStartupBasisStandalone},
		{ID: 3, Code: "thunder_kick", Category: model.MoveCategoryUnique, Startup: ptr(29), Active: ptr(3), Total: ptr(51), Damage: ptr(1000), IsDerived: true, StartupBasis: model.MoveStartupBasisThrough},
	}
	derivations := []setplayrepo.MoveDerivation{{ChildMoveID: 3, ParentMoveID: 2}}
	svc := newServiceWithDerivations(newCombo(ptr(ka)), moves, derivations, nil)
	res, err := svc.SuggestForCombo(context.Background(), 1, SuggestParams{NMin: 1, TargetTypes: allTypes()})
	if err != nil {
		t.Fatal(err)
	}
	p := findProposal(res, "standing_light_punch", "quick_dash", "thunder_kick")
	if p == nil {
		t.Fatalf("表記展開された解が無い(親が前置されていない): %+v", res.Proposals)
	}
	// 分類 1: 親は計上ゼロ。
	if p.Steps[1].Role != RoleParent {
		t.Errorf("前置ステップの role = %q, want %q", p.Steps[1].Role, RoleParent)
	}
	if p.Steps[1].Counted {
		t.Error("表記用親は計上ゼロでなければならない(counted=false)")
	}
	// 分類 4(立弱P total 13)＋ 分類 3(target startup 29)＝ 42。親の total 45 は入らない。
	assertIdentities(t, p, ka, 13+29)
	if p.N != 3 {
		t.Errorf("N = %d, want 3", p.N)
	}
}

// TestCost_Class1_NotAppliedToStandalone は standalone の技に親を前置しないことを固定する。
// through 以外に前置すると通し値でない分が二重計上になる。
func TestCost_Class1_NotAppliedToStandalone(t *testing.T) {
	moves := []setplayrepo.MoveCandidate{
		{ID: 1, Code: "f18", Category: model.MoveCategoryNormal, Startup: ptr(5), Active: ptr(3), Total: ptr(18), Damage: ptr(300), StartupBasis: model.MoveStartupBasisStandalone},
		{ID: 2, Code: "parent", Category: model.MoveCategoryNormal, Startup: ptr(5), Active: ptr(3), Total: ptr(20), Damage: ptr(300), StartupBasis: model.MoveStartupBasisStandalone},
		// through ではない子。親参照は持つが standalone なので単独入力不可 → filler からも落ちる。
		{ID: 3, Code: "tgt", Category: model.MoveCategoryUnique, Startup: ptr(20), Active: ptr(4), Total: ptr(42), Damage: ptr(600), StartupBasis: model.MoveStartupBasisStandalone},
	}
	svc := newServiceWithDerivations(newCombo(ptr(40)), moves, nil, nil)
	res, err := svc.SuggestForCombo(context.Background(), 1, SuggestParams{NMin: 1, TargetTypes: allTypes()})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range res.Proposals {
		for _, st := range p.Steps {
			if st.Role == RoleParent {
				t.Errorf("standalone の技に表記用親を前置してはならない: %+v", p.Steps)
			}
		}
	}
}

// ryuChainMoves はリュウのチェーングループ(実測値・M19-DESIGN-07 §1-3)。
//
//	立弱P 4/3/7/13 → チェーン時実消費 9
//	屈弱P 4/2/9/14 → 10
//	屈弱K 5/2/10/16 → 12
func ryuChainMoves() []setplayrepo.MoveCandidate {
	return []setplayrepo.MoveCandidate{
		{ID: 1, Code: "standing_light_punch", Category: model.MoveCategoryNormal, Startup: ptr(4), Active: ptr(3), Total: ptr(13), Damage: ptr(300), StartupBasis: model.MoveStartupBasisStandalone, ChainCancelTotal: cctPtr(9)},
		{ID: 2, Code: "crouching_light_punch", Category: model.MoveCategoryNormal, Startup: ptr(4), Active: ptr(2), Total: ptr(14), Damage: ptr(300), StartupBasis: model.MoveStartupBasisStandalone, ChainCancelTotal: cctPtr(10)},
		{ID: 3, Code: "crouching_light_kick", Category: model.MoveCategoryNormal, Startup: ptr(5), Active: ptr(2), Total: ptr(16), Damage: ptr(300), StartupBasis: model.MoveStartupBasisStandalone, ChainCancelTotal: cctPtr(12)},
	}
}

// TestCost_Class2_ChainAdjacencyUsesChainCancelTotal は分類 2 を固定する。
//
// ★M19-DESIGN-07 §5-2 の実例: 「リュウ立弱P ×2 は 26F でなく 22F」。
// 現行エンジンは 13+13=26 と計上していた(物理的に入力できないレシピ)。
// 正しくは 9(チェーン時実消費) + 13(末端の total) = 22 である。
func TestCost_Class2_ChainAdjacencyUsesChainCancelTotal(t *testing.T) {
	// KA=43・target startup 20 → budgetMax = 43+2-1-20 = 24。
	// 合成単位 22(=9+13)は収まるが、チェーンを使わない 13+13=26 は収まらない。
	const ka = 43
	moves := append(ryuChainMoves(),
		setplayrepo.MoveCandidate{ID: 9, Code: "tgt", Category: model.MoveCategoryUnique, Startup: ptr(20), Active: ptr(4), Total: ptr(42), Damage: ptr(600), StartupBasis: model.MoveStartupBasisStandalone})
	svc := newServiceWithDerivations(newCombo(ptr(ka)), moves, nil, nil)
	res, err := svc.SuggestForCombo(context.Background(), 1, SuggestParams{NMin: 1, TargetTypes: allTypes()})
	if err != nil {
		t.Fatal(err)
	}
	p := findProposal(res, "standing_light_punch", "standing_light_punch", "tgt")
	if p == nil {
		t.Fatalf("立弱P ×2 の合成単位を使う解が無い: %+v", res.Proposals)
	}
	// 分類 2(9) ＋ 分類 4(13) ＋ 分類 3(20) = 42。26+20=46 ではない。
	assertIdentities(t, p, ka, 9+13+20)
	for _, st := range p.Steps {
		if !st.Counted {
			t.Errorf("チェーン構成技は計上される(counted=true)こと: %+v", st)
		}
	}
}

// TestCost_CompositeUnitTotalFormula は合成単位 total の式を長さ 2 と 3 で固定する。
// 単位 total ＝ Σ(非末端の chain_cancel_total) ＋ 末端の total。
func TestCost_CompositeUnitTotalFormula(t *testing.T) {
	moves := ryuChainMoves()
	units, _ := buildFillerUnits(moves, moves, map[int64][]int64{}, 200, newCancelProbe(nil))
	byName := map[string]int{}
	for _, u := range units {
		byName[u.Name] = u.Total
	}
	cases := []struct {
		name string
		want int
	}{
		// 単発(長さ 1)は total そのもの。
		{"standing_light_punch", 13},
		{"crouching_light_kick", 16},
		// 長さ 2: cct(先頭) + total(末端)。
		{"standing_light_punch>standing_light_punch", 9 + 13},
		{"standing_light_punch>crouching_light_kick", 9 + 16},
		{"crouching_light_kick>crouching_light_punch", 12 + 14},
		// 長さ 3: cct + cct + total。
		{"standing_light_punch>crouching_light_punch>crouching_light_kick", 9 + 10 + 16},
	}
	for _, c := range cases {
		got, ok := byName[c.name]
		if !ok {
			t.Errorf("合成単位 %q が作られていない", c.name)
			continue
		}
		if got != c.want {
			t.Errorf("単位 %q の total = %d, want %d", c.name, got, c.want)
		}
	}
}

// TestCost_CompositeUnit_BudgetCap は budgetCap を超える単位を作らないことを固定する。
// k は KA 上限で自然に有限になる(M19-DESIGN-07 §2-2)。
func TestCost_CompositeUnit_BudgetCap(t *testing.T) {
	moves := ryuChainMoves()
	units, _ := buildFillerUnits(moves, moves, map[int64][]int64{}, 25, newCancelProbe(nil))
	for _, u := range units {
		if u.Total > 25 {
			t.Errorf("budgetCap 超過の単位が作られた: %s total=%d", u.Name, u.Total)
		}
	}
	// 予算 25 なら 9+13=22 は作られるが 9+10+16=35 は作られない。
	var has22, has35 bool
	for _, u := range units {
		switch u.Name {
		case "standing_light_punch>standing_light_punch":
			has22 = true
		case "standing_light_punch>crouching_light_punch>crouching_light_kick":
			has35 = true
		}
	}
	if !has22 {
		t.Error("予算内の長さ 2 の単位が作られていない")
	}
	if has35 {
		t.Error("予算外の長さ 3 の単位が作られている")
	}
}

// zangiefChainMoves はザンギエフのチェーングループ(実測値・M19-DESIGN-07 §1-3・§9-1)。
// 通常版 3 技(単独入力可)と連打版 3 技(単独入力不可・moves に別行として登録済み)。
func zangiefChainMoves() ([]setplayrepo.MoveCandidate, []setplayrepo.MoveDerivation) {
	moves := []setplayrepo.MoveCandidate{
		{ID: 1, Code: "standing_light_punch", Category: model.MoveCategoryNormal, Startup: ptr(7), Active: ptr(3), Total: ptr(18), Damage: ptr(300), StartupBasis: model.MoveStartupBasisStandalone, ChainCancelTotal: cctPtr(15)},
		{ID: 2, Code: "crouching_light_punch", Category: model.MoveCategoryNormal, Startup: ptr(6), Active: ptr(2), Total: ptr(15), Damage: ptr(300), StartupBasis: model.MoveStartupBasisStandalone, ChainCancelTotal: cctPtr(13)},
		{ID: 3, Code: "crouching_light_kick", Category: model.MoveCategoryNormal, Startup: ptr(4), Active: ptr(3), Total: ptr(18), Damage: ptr(300), StartupBasis: model.MoveStartupBasisStandalone, ChainCancelTotal: cctPtr(12)},
		{ID: 4, Code: "standing_light_punch_rapid", Category: model.MoveCategoryNormal, Startup: ptr(4), Active: ptr(3), Total: ptr(15), IsDerived: true, StartupBasis: model.MoveStartupBasisStandalone, ChainCancelTotal: cctPtr(12)},
		{ID: 5, Code: "crouching_light_punch_rapid", Category: model.MoveCategoryNormal, Startup: ptr(3), Active: ptr(2), Total: ptr(12), IsDerived: true, StartupBasis: model.MoveStartupBasisStandalone, ChainCancelTotal: cctPtr(10)},
		{ID: 6, Code: "crouching_light_kick_rapid", Category: model.MoveCategoryNormal, Startup: ptr(3), Active: ptr(3), Total: ptr(17), IsDerived: true, StartupBasis: model.MoveStartupBasisStandalone, ChainCancelTotal: cctPtr(11)},
	}
	var derivs []setplayrepo.MoveDerivation
	for _, child := range []int64{4, 5, 6} {
		for _, parent := range []int64{1, 2, 3} {
			derivs = append(derivs, setplayrepo.MoveDerivation{ChildMoveID: child, ParentMoveID: parent})
		}
	}
	return moves, derivs
}

// TestCost_CompositeUnit_ZangiefRapidGoesSecondOrLater は連打版行が 2 番目以降にだけ現れること、
// および差し込んだ行自身の値を費用規則が読むことを固定する(M19-DESIGN-07 §9-1)。
//
// ★キャラ名ではなく性質で一般化している——「単独入力不可のチェーングループ員があれば、
// 2 番目以降はその集合から採る」。ザンギエフ特例は要らない。
func TestCost_CompositeUnit_ZangiefRapidGoesSecondOrLater(t *testing.T) {
	moves, derivs := zangiefChainMoves()
	parents := map[int64][]int64{}
	for _, d := range derivs {
		parents[d.ChildMoveID] = append(parents[d.ChildMoveID], d.ParentMoveID)
	}
	singles := collectFillerSingles(moves, parents)
	units, _ := buildFillerUnits(singles, moves, parents, 200, newCancelProbe(nil))

	rapid := map[string]bool{
		"standing_light_punch_rapid": true, "crouching_light_punch_rapid": true, "crouching_light_kick_rapid": true,
	}
	var sawRapidContinuation, sawNormalContinuation bool
	for _, u := range units {
		codes := strings.Split(u.Name, ">")
		if rapid[codes[0]] {
			t.Errorf("連打版が先頭に来ている(単独入力不可なので連携を始められない): %s", u.Name)
		}
		for _, c := range codes[1:] {
			if rapid[c] {
				sawRapidContinuation = true
			} else {
				sawNormalContinuation = true
			}
		}
	}
	if !sawRapidContinuation {
		t.Error("2 番目以降に連打版が差し込まれていない")
	}
	if sawNormalContinuation {
		t.Error("単独入力不可の員があるキャラでは、2 番目以降に通常版を置いてはならない")
	}

	// 差し込んだ行自身の値が読まれること: 立弱P(cct 15) > 屈弱P連打版(total 12) = 27。
	// 通常版の屈弱P total 15 を読むと 30 になり、実機と合わない。
	var got int
	for _, u := range units {
		if u.Name == "standing_light_punch>crouching_light_punch_rapid" {
			got = u.Total
		}
	}
	if want := 15 + 12; got != want {
		t.Errorf("連打版を末端に置いた単位の total = %d, want %d(連打版自身の total を読むこと)", got, want)
	}
}

// TestCost_CompositeUnit_NoSoloUnavailableMember は単独入力不可の員が居ないキャラでは
// グループ全員が 2 番目以降に来ることを固定する(分岐の対を押さえる)。
func TestCost_CompositeUnit_NoSoloUnavailableMember(t *testing.T) {
	moves := ryuChainMoves()
	units, _ := buildFillerUnits(moves, moves, map[int64][]int64{}, 200, newCancelProbe(nil))
	seen := map[string]bool{}
	for _, u := range units {
		codes := strings.Split(u.Name, ">")
		for _, c := range codes[1:] {
			seen[c] = true
		}
	}
	for _, m := range moves {
		if !seen[m.Code] {
			t.Errorf("単独入力不可の員が居ないキャラでは全員が 2 番目以降に来ること: %s", m.Code)
		}
	}
}

// TestCost_CompositeUnit_UnitCountCap は合成単位の個数上限が効くことを固定する。
//
// ★budgetCap = KA + 2 は KA に比例して緩み、maxChainLen は長さしか縛らない。
// KA は VAL-C10 が WARNING のみで −600〜+600 が警告なしに保存できるため、
// 極端な KA では「エンジンの枝刈りが効く前に確定的に大量の単位を作る」経路になりうる。
// engineSafetyCap と同じ役割の後段防御を単位構築側にも置いた。
func TestCost_CompositeUnit_UnitCountCap(t *testing.T) {
	orig := maxChainUnits
	maxChainUnits = 20
	defer func() { maxChainUnits = orig }()

	// budgetCap を十分大きく取り、上限が無ければ 3^k で膨らむ条件にする。
	units, _ := buildFillerUnits(ryuChainMoves(), ryuChainMoves(), map[int64][]int64{}, 100000, newCancelProbe(nil))
	if len(units) > maxChainUnits {
		t.Errorf("合成単位が上限を超えた: %d > %d", len(units), maxChainUnits)
	}
	// 上限に達していること自体を確かめる(条件が緩くて素通りしていないか)。
	if len(units) < maxChainUnits {
		t.Errorf("上限に到達していない(テスト条件が弱い): %d < %d", len(units), maxChainUnits)
	}
	// ★短い連鎖から作るため、上限で切れても長さ 2 の単位は残る。
	var hasLen2 bool
	for _, u := range units {
		if len(u.Members) == 2 {
			hasLen2 = true
		}
	}
	if !hasLen2 {
		t.Error("上限で切ったあとに長さ 2 の単位が残っていない(幅優先になっていない疑い)")
	}
}

// TestCost_CompositeUnit_PruneKeepsExtendablePath は枝刈りが保守的であることを固定する。
//
// ★「末端として置けるか(total)」と「非末端として更に伸ばせるか(chain_cancel_total)」は
// 別の判定である。chain_cancel_total < total なので、末端にすると予算超過だが
// 非末端として繋げば収まる組が原理的にありうる。2 つを 1 つの判定にまとめると
// その経路ごと落ちる。実データでは起きないが、コード上の根拠として固定する。
func TestCost_CompositeUnit_PruneKeepsExtendablePath(t *testing.T) {
	// big: total 40(末端にすると予算超過)だが chain_cancel_total 5(繋ぐだけなら安い)。
	// small: total 6。big を非末端にすれば 5 + 5 + 6 = 16 <= 20 で収まる。
	moves := []setplayrepo.MoveCandidate{
		{ID: 1, Code: "head", Category: model.MoveCategoryNormal, Startup: ptr(4), Active: ptr(3), Total: ptr(10), Damage: ptr(300), StartupBasis: model.MoveStartupBasisStandalone, ChainCancelTotal: cctPtr(5)},
		{ID: 2, Code: "big", Category: model.MoveCategoryNormal, Startup: ptr(4), Active: ptr(3), Total: ptr(40), Damage: ptr(300), StartupBasis: model.MoveStartupBasisStandalone, ChainCancelTotal: cctPtr(5)},
		{ID: 3, Code: "small", Category: model.MoveCategoryNormal, Startup: ptr(4), Active: ptr(3), Total: ptr(6), Damage: ptr(300), StartupBasis: model.MoveStartupBasisStandalone, ChainCancelTotal: cctPtr(5)},
	}
	units, _ := buildFillerUnits(moves, moves, map[int64][]int64{}, 20, newCancelProbe(nil))
	var found bool
	for _, u := range units {
		if u.Name == "head>big>small" {
			found = true
			if want := 5 + 5 + 6; u.Total != want {
				t.Errorf("head>big>small の total = %d, want %d", u.Total, want)
			}
		}
		if u.Name == "head>big" {
			t.Errorf("末端にすると予算超過の単位が作られている: %s total=%d", u.Name, u.Total)
		}
	}
	if !found {
		t.Error("末端では予算超過だが非末端としては繋げる経路が枝刈りされている")
	}
}

// TestCost_GapMode_WithCompositeUnit は gap モードでも恒等式 G = N − active が
// 新しい費用規則(合成単位)の下で成立することを固定する。
func TestCost_GapMode_WithCompositeUnit(t *testing.T) {
	const ka = 40
	moves := append(ryuChainMoves(),
		setplayrepo.MoveCandidate{ID: 9, Code: "tgt", Category: model.MoveCategoryUnique, Startup: ptr(10), Active: ptr(4), Total: ptr(42), Damage: ptr(600), StartupBasis: model.MoveStartupBasisStandalone})
	svc := newServiceWithDerivations(newCombo(ptr(ka)), moves, nil, nil)
	res, err := svc.SuggestForCombo(context.Background(), 1, SuggestParams{
		Mode: ModeGap, TargetTypes: allTypes(),
	})
	if err != nil {
		t.Fatal(err)
	}
	var checked int
	for i := range res.Proposals {
		p := &res.Proposals[i]
		// 恒等式は gap でも不変。
		if want := ka + 2 - p.S; p.N != want {
			t.Errorf("N = %d, want KA+2-S = %d", p.N, want)
		}
		if want := p.N - p.TargetActive; p.G != want {
			t.Errorf("G = %d, want N-active = %d", p.G, want)
		}
		// 合成単位を使った解が 1 件以上あること(gap 経路でも単位が渡っている)。
		if len(p.Steps) >= 3 {
			checked++
		}
	}
	if len(res.Proposals) == 0 {
		t.Fatal("gap モードで提案が 0 件")
	}
	if checked == 0 {
		t.Error("gap モードで合成単位を使った解が 1 件も無い")
	}
}
