package setplay

import (
	"context"
	"strings"
	"testing"

	"github.com/plexiblinp/tacpendium-setplay/internal/model"
	setplayrepo "github.com/plexiblinp/tacpendium-setplay/internal/repository/setplay"
)

// M47-05(CHANGE-254): 結果の行の絞り込み(接近 ／ ガード時 0F 以上 ／ 連打キャンセル)・
// 型 B の重複をまとめる・型 A の隣り合い を固定する。
//
// ★どのテストも「在る条件」と「無い条件」の両方を置く(playbook §4.75)。
// ★J の式の境目(ちょうど 0 と −1)と、重複で残す側(費用の小さい方)は破壊確認の対象である。

// codesOf は提案の表記順の code 列を ">" で連ねる。
func codesOf(p Proposal) string {
	codes := make([]string, len(p.Steps))
	for i, st := range p.Steps {
		codes[i] = st.Code
	}
	return strings.Join(codes, ">")
}

// suggestAll は limit で切らずに全行を返す(maxLimit まで)。
func suggestAll(t *testing.T, svc testService, params SuggestParams) *SuggestResult {
	t.Helper()
	params.Limit = maxLimit
	res, err := svc.SuggestForCombo(context.Background(), 1, params)
	if err != nil {
		t.Fatal(err)
	}
	if res.TotalFound != len(res.Proposals) {
		t.Fatalf("全行を取れていない: totalFound=%d rows=%d", res.TotalFound, len(res.Proposals))
	}
	return res
}

func countRecipe(res *SuggestResult, recipe string) int {
	n := 0
	for _, p := range res.Proposals {
		if codesOf(p) == recipe {
			n++
		}
	}
	return n
}

// withApproachDict は接近技の辞書をテストの間だけ差し替える。
func withApproachDict(t *testing.T, dict map[string][]string) {
	t.Helper()
	orig := characterApproachMoves
	characterApproachMoves = dict
	t.Cleanup(func() { characterApproachMoves = orig })
}

// --- 型 B の重複(常に) ---

// TestRefine_DedupeTypeB_KeepsCheaperCost は、同じレシピ「立弱P > 立弱P > 当てる技」が
// 合成単位経由(9+13=22)と単発 2 つ分(13+13=26)の 2 行で出る条件で、22 の 1 行だけが残ることを固定する
// (M19-DESIGN-07 §5-2 の実例・D-1003)。
func TestRefine_DedupeTypeB_KeepsCheaperCost(t *testing.T) {
	// 当てる技 startup 20 ・active 6 → 受理帯の幅 5。KA=45 → 予算 45+2-1-20=26。
	// Σ=22(残り 4)も Σ=26(残り 0)も受理される＝エンジンからは 2 行出る。
	const ka = 45
	moves := append(ryuChainMoves(),
		setplayrepo.MoveCandidate{ID: 9, Code: "tgt", Category: model.MoveCategoryUnique, Startup: ptr(20), Active: ptr(6), Total: ptr(60), Damage: ptr(600), StartupBasis: model.MoveStartupBasisStandalone})
	// 前提: エンジンは同じ表記を 2 通りで出す(まとめる対象が実在する)。
	raw := 0
	for _, sg := range mustEngine(t, ka, moves, "tgt") {
		if fillerNamesJoined(sg) == "standing_light_punch+standing_light_punch" || fillerNamesJoined(sg) == "standing_light_punch>standing_light_punch" {
			raw++
		}
	}
	if raw != 2 {
		t.Fatalf("前提: エンジンが 立弱P×2 を 2 通りで出すこと。got %d", raw)
	}

	svc := newService(newCombo(ptr(ka)), moves, nil)
	// 当てる技は tgt だけにする(チェーン員は通常技なので unique だけを対象)。
	res := suggestAll(t, svc, SuggestParams{NMin: 1, TargetTypes: []string{TargetTypeUnique}})
	recipe := "standing_light_punch>standing_light_punch>tgt"
	if got := countRecipe(res, recipe); got != 1 {
		t.Fatalf("同じレシピは 1 行にまとまること: %s が %d 行", recipe, got)
	}
	p := findProposal(res, "standing_light_punch", "standing_light_punch", "tgt")
	// 費用の小さい方(合成単位 22)が残る: S = 22 + 20 = 42。26 側なら 46。
	assertIdentities(t, p, ka, 9+13+20)

	// 無い条件: 別のレシピ(立弱P>屈弱P＝合成単位 9+14=23)はまとめられずに残る。
	if countRecipe(res, "standing_light_punch>crouching_light_punch>tgt") != 1 {
		t.Errorf("別のレシピまでまとめてはならない: %v", recipesOf(res))
	}
}

// TestDedupeByRecipe_Direct は、まとめる規則を関数単位で固定する(費用が同じなら先の行・位置は組の最初)。
func TestDedupeByRecipe_Direct(t *testing.T) {
	in := []Proposal{
		{recipeHash: "a", sumFiller: 26, sortName: "a26"},
		{recipeHash: "b", sumFiller: 10, sortName: "b10"},
		{recipeHash: "a", sumFiller: 22, sortName: "a22"},
		{recipeHash: "b", sumFiller: 10, sortName: "b10-second"},
		{recipeHash: "c", sumFiller: 5, sortName: "c5"},
	}
	out := dedupeByRecipe(in)
	var got []string
	for _, p := range out {
		got = append(got, p.sortName)
	}
	if want := "a22,b10,c5"; strings.Join(got, ",") != want {
		t.Errorf("dedupe = %v, want %s", got, want)
	}
}

// --- J: ガード時 0F 以上(meaty だけ) ---

// blockFixture は、繋ぎ 0 個・N=2 の行だけが出る 3 つの当てる技を返す。
// KA=5 ・startup 5 → 予算 1 → N = 1 + 1 = 2。total 30 は予算を超えるため繋ぎにならない。
// on_block: −1(−1+2−1=0 ＝境目の在る側)／ −2(−1 ＝境目の外)／ NULL。
func blockFixture() []setplayrepo.MoveCandidate {
	return []setplayrepo.MoveCandidate{
		{ID: 1, Code: "ob_minus1", Category: model.MoveCategoryUnique, Startup: ptr(5), Active: ptr(5), Total: ptr(30), Damage: ptr(500), OnBlock: ptr(-1)},
		{ID: 2, Code: "ob_minus2", Category: model.MoveCategoryUnique, Startup: ptr(5), Active: ptr(5), Total: ptr(30), Damage: ptr(500), OnBlock: ptr(-2)},
		{ID: 3, Code: "ob_null", Category: model.MoveCategoryUnique, Startup: ptr(5), Active: ptr(5), Total: ptr(30), Damage: ptr(500)},
	}
}

func TestRefine_BlockSafeOnly_BoundaryAndNull(t *testing.T) {
	svc := newService(newCombo(ptr(5)), blockFixture(), nil)

	off := suggestAll(t, svc, SuggestParams{NMin: 1, TargetTypes: allTypes()})
	if got := recipesOf(off); got != "ob_minus1,ob_minus2,ob_null" {
		t.Fatalf("前提: OFF では 3 行とも出る(N=2)。got %s", got)
	}
	for _, p := range off.Proposals {
		if p.N != 2 {
			t.Fatalf("前提: N=2。got %d(%s)", p.N, codesOf(p))
		}
	}

	on := suggestAll(t, svc, SuggestParams{NMin: 1, TargetTypes: allTypes(), BlockSafeOnly: true})
	// −1 + 2 − 1 = 0 は残る(境目を含む)。−2 + 2 − 1 = −1 は消える。NULL は消える。
	if got := recipesOf(on); got != "ob_minus1" {
		t.Errorf("ガード時 0F 以上だけ = %s, want ob_minus1(境目 0 を含み、−1 と NULL を除く)", got)
	}
	if on.TotalFound != 1 {
		t.Errorf("totalFound は絞った後の数であること: %d", on.TotalFound)
	}
}

// TestRefine_BlockSafeOnly_IgnoredOutsideMeaty は、汚連携 ・繋ぎだけでは J が効かないことを固定する。
func TestRefine_BlockSafeOnly_IgnoredOutsideMeaty(t *testing.T) {
	// 汚連携: KA=12 ・startup 5 ・active 5 → 予算 8 → N=9 ＝ G=4(1〜13 の範囲)。3 行とも出る。
	gapSvc := newService(newCombo(ptr(12)), blockFixture(), nil)
	gapOff := suggestAll(t, gapSvc, SuggestParams{Mode: ModeGap, TargetTypes: allTypes()})
	gapOn := suggestAll(t, gapSvc, SuggestParams{Mode: ModeGap, TargetTypes: allTypes(), BlockSafeOnly: true})
	if len(gapOff.Proposals) == 0 || recipesOf(gapOff) != recipesOf(gapOn) {
		t.Errorf("汚連携では J は効かない: off=%s on=%s", recipesOf(gapOff), recipesOf(gapOn))
	}

	// 繋ぎだけ: 当てる技が無い(targetOnBlock が nil)行も、J では消えない。
	fSvc := newService(newCombo(ptr(40)), gc1Moves(), nil)
	fOff := suggestAll(t, fSvc, SuggestParams{Mode: ModeFiller, Remain: 4})
	fOn := suggestAll(t, fSvc, SuggestParams{Mode: ModeFiller, Remain: 4, BlockSafeOnly: true})
	if len(fOff.Proposals) == 0 || recipesOf(fOff) != recipesOf(fOn) {
		t.Errorf("繋ぎだけでは J は効かない: off=%s on=%s", recipesOf(fOff), recipesOf(fOn))
	}
}

// --- B: 接近を含む提案だけ ---

// approachFixture: 前ステップ(19)・後ろステップ(23)・固有の接近技 quick_dash(20)・前歩き(total NULL)
// と、当てる技 tgt(startup 5 ・active 30＝受理帯が広い)。
func approachFixture() []setplayrepo.MoveCandidate {
	return []setplayrepo.MoveCandidate{
		{ID: 1, Code: model.MoveCodeDashForward, Category: model.MoveCategorySystem, Total: ptr(19)},
		{ID: 2, Code: "dash_back", Category: model.MoveCategorySystem, Total: ptr(23)},
		{ID: 3, Code: "forward", Category: model.MoveCategorySystem},
		{ID: 4, Code: "quick_dash", Category: model.MoveCategoryUnique, Startup: ptr(20), Active: ptr(1), Total: ptr(20)},
		{ID: 9, Code: "tgt", Category: model.MoveCategoryUnique, Startup: ptr(5), Active: ptr(30), Total: ptr(60), Damage: ptr(500)},
	}
}

func TestRefine_ApproachOnly_CommonAndCharacterMoves(t *testing.T) {
	// KA=30 → 予算 26。繋ぎ 1 つの行(19 / 20 / 23)と繋ぎ 0 個の行が出る。
	svc := newService(newCombo(ptr(30)), approachFixture(), nil)

	// 辞書に載っていないとき: 前ステップの行だけが残る。後ろステップ・繋ぎ 0 個は消える。
	withApproachDict(t, map[string][]string{})
	off := suggestAll(t, svc, SuggestParams{NMin: 1, TargetTypes: allTypes()})
	for _, r := range []string{"dash_forward>tgt", "dash_back>tgt", "quick_dash>tgt", "tgt"} {
		if countRecipe(off, r) != 1 {
			t.Fatalf("前提: OFF では %s が出る。got %s", r, recipesOf(off))
		}
	}
	on := suggestAll(t, svc, SuggestParams{NMin: 1, TargetTypes: allTypes(), ApproachOnly: true})
	if got := recipesOf(on); got != "dash_forward>tgt" {
		t.Errorf("辞書なし: 接近を含む提案だけ = %s, want dash_forward>tgt(後ろステップ・前歩き・固有技でない技は数えない)", got)
	}

	// 辞書に載せたとき: キャラ固有の接近技の行も残る(キャラの code＝fakeMoveReader の "test_char")。
	withApproachDict(t, map[string][]string{"test_char": {"quick_dash"}})
	on2 := suggestAll(t, svc, SuggestParams{NMin: 1, TargetTypes: allTypes(), ApproachOnly: true})
	if got := recipesOf(on2); got != "dash_forward>tgt,quick_dash>tgt" {
		t.Errorf("辞書あり: 接近を含む提案だけ = %s", got)
	}
	// 別のキャラの一覧は効かない。
	withApproachDict(t, map[string][]string{"other_char": {"quick_dash"}})
	on3 := suggestAll(t, svc, SuggestParams{NMin: 1, TargetTypes: allTypes(), ApproachOnly: true})
	if got := recipesOf(on3); got != "dash_forward>tgt" {
		t.Errorf("別キャラの辞書が効いている: %s", got)
	}
}

// TestRefine_ApproachOnly_RushFillerAndRushTarget は、ラッシュの技(rush_variant)が繋ぎにあっても
// 当てる技であっても「接近を含む」に数えることを固定する(当てる技は開発者の回答 2026-09-28)。
func TestRefine_ApproachOnly_RushFillerAndRushTarget(t *testing.T) {
	base := int64(20)
	moves := []setplayrepo.MoveCandidate{
		{ID: 20, Code: "standing_medium_punch", Category: model.MoveCategoryNormal, Startup: ptr(6), Active: ptr(3), Total: ptr(25), Damage: ptr(600)},
		{ID: 21, Code: "rush_standing_medium_punch", Category: model.MoveCategoryRushVariant, OriginalMoveID: &base, Startup: ptr(15), Active: ptr(3), Total: ptr(34), Damage: ptr(600), IsDerived: true, StartupBasis: model.MoveStartupBasisThrough},
	}
	// ラッシュの技が当てる技(normal_rush だけを対象)。繋ぎに接近の手は無い。
	svc := newService(newCombo(ptr(15)), moves, nil)
	off := suggestAll(t, svc, SuggestParams{NMin: 1, TargetTypes: []string{TargetTypeNormalRush}})
	on := suggestAll(t, svc, SuggestParams{NMin: 1, TargetTypes: []string{TargetTypeNormalRush}, ApproachOnly: true})
	if len(off.Proposals) == 0 || recipesOf(off) != recipesOf(on) {
		t.Errorf("当てる技がラッシュの技の行は接近を含む: off=%s on=%s", recipesOf(off), recipesOf(on))
	}

	// ラッシュの技が繋ぎ(当てる技は通常技)。KA=41 → 予算 41+2-1-6=34 ＝ラッシュの技の 34 がちょうど入る。
	svc2 := newService(newCombo(ptr(41)), moves, nil)
	on2 := suggestAll(t, svc2, SuggestParams{NMin: 1, TargetTypes: []string{TargetTypeNormal}, ApproachOnly: true})
	if countRecipe(on2, "rush_standing_medium_punch>standing_medium_punch") != 1 {
		t.Errorf("繋ぎにラッシュの技を含む行が残ること: %s", recipesOf(on2))
	}
	for _, p := range on2.Proposals {
		if !strings.Contains(codesOf(p), "rush_") {
			t.Errorf("接近を含まない行が残っている: %s", codesOf(p))
		}
	}
}

// --- G: 連打キャンセルを含む提案だけ ---

func TestRefine_ChainCancelOnly_TypeB(t *testing.T) {
	// KA=45 ・当てる技 startup 20 ・active 6 → Σ ∈ [21, 26]。チェーン員ではない繋ぎ(24)を 1 つ足す。
	const ka = 45
	moves := append(ryuChainMoves(),
		setplayrepo.MoveCandidate{ID: 8, Code: "plain_whiff", Category: model.MoveCategoryNormal, Startup: ptr(10), Active: ptr(2), Total: ptr(24)},
		setplayrepo.MoveCandidate{ID: 9, Code: "tgt", Category: model.MoveCategoryUnique, Startup: ptr(20), Active: ptr(6), Total: ptr(60), Damage: ptr(600), StartupBasis: model.MoveStartupBasisStandalone})
	svc := newService(newCombo(ptr(ka)), moves, nil)
	types := []string{TargetTypeUnique}
	off := suggestAll(t, svc, SuggestParams{NMin: 1, TargetTypes: types})
	on := suggestAll(t, svc, SuggestParams{NMin: 1, TargetTypes: types, ChainCancelOnly: true})
	if countRecipe(off, "plain_whiff>tgt") != 1 {
		t.Fatalf("前提: 単発の繋ぎの行が OFF で出る: %s", recipesOf(off))
	}
	if countRecipe(on, "plain_whiff>tgt") != 0 {
		t.Errorf("連打キャンセルを含まない行が残っている")
	}
	if countRecipe(on, "standing_light_punch>standing_light_punch>tgt") != 1 {
		t.Errorf("連打キャンセル(合成単位)を含む行が残ること: %s", recipesOf(on))
	}
	for _, p := range on.Proposals {
		if !p.hasChainCancel {
			t.Errorf("連打キャンセルを含まない行: %s", codesOf(p))
		}
	}
	if on.TotalFound >= off.TotalFound {
		t.Errorf("絞った後の totalFound が減っていない: on=%d off=%d", on.TotalFound, off.TotalFound)
	}
}

// TestRefine_ChainCancelOnly_TypeARapid は、型 A の連打版の技(単独で出せない行)を含む行が残ることを固定する。
func TestRefine_ChainCancelOnly_TypeARapid(t *testing.T) {
	moves, derivs := zangiefChainMoves()
	moves = append(moves, setplayrepo.MoveCandidate{ID: 9, Code: "tgt", Category: model.MoveCategoryUnique, Startup: ptr(20), Active: ptr(40), Total: ptr(80), Damage: ptr(600)})
	svc := newServiceWithDerivations(newCombo(ptr(70)), moves, derivs, nil)
	on := suggestAll(t, svc, SuggestParams{NMin: 1, TargetTypes: allTypes(), ChainCancelOnly: true})
	if countRecipe(on, "standing_light_punch>standing_light_punch_rapid>tgt") != 1 {
		t.Errorf("連打版を含む行が残ること: %s", recipesOf(on))
	}
	for _, p := range on.Proposals {
		if !strings.Contains(codesOf(p), "_rapid") {
			t.Errorf("型 A で連打版を含まない行が残っている: %s", codesOf(p))
		}
	}
}

// --- 型 A の隣り合い ---

// TestRefine_TypeA_Adjacency は開発者の回答(2026-09-28)の 4 つの形をザンギエフで固定する。
//
//	(a) 弱P>連打版弱P>連打版弱P(合成単位の内側)→ 残る
//	(b) 弱P ＋ 弱P(単発のチェーン員が隣り合う・どう並べても隣り合う)→ 落ちる
//	(c) 屈弱P ＋ 前ステップ ＋ 弱P(間に別の技)→ 残る
//	(d) 屈弱P ＋ 弱P ＋ 前ステップ(隣り合う)→ (c) の並びへ並べ替えて出す(S は変わらない)
func TestRefine_TypeA_Adjacency(t *testing.T) {
	moves, derivs := zangiefChainMoves()
	moves = append(moves,
		setplayrepo.MoveCandidate{ID: 7, Code: model.MoveCodeDashForward, Category: model.MoveCategorySystem, Total: ptr(19)},
		setplayrepo.MoveCandidate{ID: 9, Code: "tgt", Category: model.MoveCategoryUnique, Startup: ptr(20), Active: ptr(40), Total: ptr(80), Damage: ptr(600)})
	// KA=80 → 予算 61。受理帯の幅 39 → Σ ∈ [22, 61] が受理される。
	const ka = 80
	svc := newServiceWithDerivations(newCombo(ptr(ka)), moves, derivs, nil)
	res := suggestAll(t, svc, SuggestParams{NMin: 1, TargetTypes: allTypes()})

	// (a) 合成単位(15+12+15=42)は残る。
	if countRecipe(res, "standing_light_punch>standing_light_punch_rapid>standing_light_punch_rapid>tgt") != 1 {
		t.Errorf("(a) 合成単位の行が残ること")
	}
	// (b) 単発の 弱P ＋ 弱P(18+18=36)は落ちる。
	if countRecipe(res, "standing_light_punch>standing_light_punch>tgt") != 0 {
		t.Errorf("(b) 単発のチェーン員が隣り合う行は落ちること")
	}
	// (d) → (c): 屈弱P(15)・弱P(18)・前ステップ(19)＝52。エンジンは全体フレームの昇順
	// [屈弱P, 弱P, 前ステップ] で出す(隣り合う)。隣り合わない並びへ並べ替える。
	if countRecipe(res, "crouching_light_punch>standing_light_punch>dash_forward>tgt") != 0 {
		t.Errorf("(d) 隣り合う並びのまま出ている")
	}
	p := findProposal(res, "crouching_light_punch", "dash_forward", "standing_light_punch", "tgt")
	if p == nil {
		t.Fatalf("(c) 並べ替えた並びで出ること: %s", recipesOf(res))
	}
	assertIdentities(t, p, ka, 15+18+19+20) // 並べ替えても S は変わらない
	// どの行にも「連打キャンセルになる」隣り合いが残っていない。
	for _, q := range res.Proposals {
		if hasChainAdjacency(q.parts, q.targetPart) {
			t.Errorf("隣り合いが残っている: %s", codesOf(q))
		}
	}
}

// parentsOf は move_derivations の行を「子 → 親の ID 列」へ直す(テスト用)。
func parentsOf(derivs []setplayrepo.MoveDerivation) map[int64][]int64 {
	out := map[int64][]int64{}
	for _, d := range derivs {
		out[d.ChildMoveID] = append(out[d.ChildMoveID], d.ParentMoveID)
	}
	return out
}

// singlePart は技 1 つの区切りを作る(テスト用)。
func singlePart(m setplayrepo.MoveCandidate, role string) proposalPart {
	return proposalPart{steps: []ProposalStep{{MoveID: m.ID, Code: m.Code, Role: role, Counted: true}}, first: m, last: m}
}

// TestAdjustChainAdjacency_Target は、最後の繋ぎと当てる技の隣り合いも同じ規則であることを固定する(ザンギエフ)。
func TestAdjustChainAdjacency_Target(t *testing.T) {
	moves, _ := zangiefChainMoves()
	slp, clp := moves[0], moves[1]
	dash := setplayrepo.MoveCandidate{ID: 7, Code: model.MoveCodeDashForward, Category: model.MoveCategorySystem, Total: ptr(19)}
	target := singlePart(clp, RoleTarget) // 当てる技＝しゃがみ弱P(連打版を持つ)

	// [前ステップ, 立ち弱P] → しゃがみ弱P: 立ち弱P の直後のしゃがみ弱P は連打版に化ける → [立ち弱P, 前ステップ] へ並べ替える。
	p1 := Proposal{parts: []proposalPart{singlePart(dash, RoleFiller), singlePart(slp, RoleFiller)}, targetPart: &target}
	p1.assemble()
	// [立ち弱P] → しゃがみ弱P: 並べ替えられない → 落ちる。
	p2 := Proposal{parts: []proposalPart{singlePart(slp, RoleFiller)}, targetPart: &target}
	p2.assemble()
	out := adjustChainAdjacency([]Proposal{p1, p2})
	if len(out) != 1 {
		t.Fatalf("並べ替えられない行は落ちること: %d 行", len(out))
	}
	if got := codesOf(out[0]); got != "standing_light_punch>dash_forward>crouching_light_punch" {
		t.Errorf("並べ替え = %s, want standing_light_punch>dash_forward>crouching_light_punch", got)
	}
	if out[0].recipeHash == p1.recipeHash {
		t.Errorf("並べ替えたら recipeHash も作り直すこと")
	}
}

// TestRefine_NormalCharacter_ChainAdjacency は、連打版を持たない通常のキャラにも同じ規則を掛けることを固定する
// (M47-05 追補④・M19-DESIGN-07 §9-3 の最速原則)。単発のチェーン員が隣り合う行は、最速で押すと
// 連打キャンセルになり表記どおりのフレームにならない ⇒ 並べ替えるか落とす。
func TestRefine_NormalCharacter_ChainAdjacency(t *testing.T) {
	ryu := ryuChainMoves()
	clp, clk := ryu[1], ryu[2]
	dash := setplayrepo.MoveCandidate{ID: 7, Code: model.MoveCodeDashForward, Category: model.MoveCategorySystem, Total: ptr(19)}

	// しゃがみ弱P ＋ しゃがみ弱K: 並べ替えられない ⇒ 落ちる。
	drop := Proposal{parts: []proposalPart{singlePart(clp, RoleFiller), singlePart(clk, RoleFiller)}}
	drop.assemble()
	// しゃがみ弱P ＋ しゃがみ弱K ＋ 前ステップ: 隣り合わない並びへ並べ替える。
	fix := Proposal{parts: []proposalPart{singlePart(clp, RoleFiller), singlePart(clk, RoleFiller), singlePart(dash, RoleFiller)}}
	fix.assemble()
	// しゃがみ弱P ＋ 前ステップ: 隣り合いが無い ⇒ そのまま。
	keep := Proposal{parts: []proposalPart{singlePart(clp, RoleFiller), singlePart(dash, RoleFiller)}}
	keep.assemble()
	out := refineProposals([]Proposal{drop, fix, keep}, SuggestParams{}, ModeMeaty)
	var got []string
	for _, p := range out {
		got = append(got, codesOf(p))
	}
	want := "crouching_light_punch>dash_forward>crouching_light_kick,crouching_light_punch>dash_forward"
	if strings.Join(got, ",") != want {
		t.Errorf("通常のキャラの隣り合い = %s, want %s", strings.Join(got, ","), want)
	}
}

// --- ダルシム(連打版しゃがみ弱P は、直前が立ち弱P ・アジャイルキックのときだけ) ---

// dhalsimChainMoves はダルシムのチェーングループ(実 seed の値・ドメイン知識集成 §3-5)。
//
//	立弱P 4/3/14 cct 12 ／ 屈弱P 素 5/3/16 cct 11 ／ アジャイルキック 5/3/17 cct 11 ／ 屈弱P 連打版 4/3/15 cct 10
//	連打版の親＝立弱P ・アジャイルキック(★屈弱P は親ではない＝同技連打では素が出る)。
func dhalsimChainMoves() ([]setplayrepo.MoveCandidate, []setplayrepo.MoveDerivation) {
	moves := []setplayrepo.MoveCandidate{
		{ID: 1, Code: "standing_light_punch", Category: model.MoveCategoryNormal, Startup: ptr(4), Active: ptr(3), Total: ptr(14), Damage: ptr(300), StartupBasis: model.MoveStartupBasisStandalone, ChainCancelTotal: cctPtr(12)},
		{ID: 2, Code: "crouching_light_punch", Category: model.MoveCategoryNormal, Startup: ptr(5), Active: ptr(3), Total: ptr(16), Damage: ptr(300), StartupBasis: model.MoveStartupBasisStandalone, ChainCancelTotal: cctPtr(11)},
		{ID: 3, Code: "agile_kick", Category: model.MoveCategoryUnique, Startup: ptr(5), Active: ptr(3), Total: ptr(17), Damage: ptr(200), StartupBasis: model.MoveStartupBasisStandalone, ChainCancelTotal: cctPtr(11)},
		{ID: 4, Code: "crouching_light_punch_rapid", Category: model.MoveCategoryNormal, Startup: ptr(4), Active: ptr(3), Total: ptr(15), IsDerived: true, StartupBasis: model.MoveStartupBasisStandalone, ChainCancelTotal: cctPtr(10)},
	}
	derivs := []setplayrepo.MoveDerivation{{ChildMoveID: 4, ParentMoveID: 1}, {ChildMoveID: 4, ParentMoveID: 3}}
	return moves, derivs
}

// TestBuildFillerUnits_Dhalsim は、連打キャンセルの単位が「直前の技」で決まることを固定する(M47-05 追補②)。
func TestBuildFillerUnits_Dhalsim(t *testing.T) {
	moves, derivs := dhalsimChainMoves()
	parents := parentsOf(derivs)
	units, _ := buildFillerUnits(collectFillerSingles(moves, parents), moves, parents, 200, newCancelProbe(nil))
	byName := map[string]int{}
	for _, u := range units {
		byName[u.Name] = u.Total
	}
	want := map[string]int{
		// 直前が立ち弱P ・アジャイルキックなら、しゃがみ弱P は連打版になる。
		"standing_light_punch>crouching_light_punch_rapid": 12 + 15,
		"agile_kick>crouching_light_punch_rapid":           11 + 15,
		// ふつうの連打キャンセル(連打版に化けない組)。
		"standing_light_punch>standing_light_punch":   12 + 14,
		"crouching_light_punch>crouching_light_punch": 11 + 16, // 同技連打は素のしゃがみ弱P
		"crouching_light_punch>standing_light_punch":  11 + 14,
		"standing_light_punch>agile_kick":             12 + 17,
		// 連打版の後のしゃがみ弱P は素(連打キャンセルは成立する)。
		"standing_light_punch>crouching_light_punch_rapid>crouching_light_punch": 12 + 10 + 16,
	}
	for name, total := range want {
		got, ok := byName[name]
		if !ok {
			t.Errorf("単位 %q が作られていない", name)
			continue
		}
		if got != total {
			t.Errorf("単位 %q の total = %d, want %d", name, got, total)
		}
	}
	for _, name := range []string{
		"crouching_light_punch>crouching_light_punch_rapid",                            // 屈弱P は連打版の親ではない
		"standing_light_punch>crouching_light_punch",                                   // 立ち弱P の後は連打版になる
		"standing_light_punch>crouching_light_punch_rapid>crouching_light_punch_rapid", // 連打版の後は素
	} {
		if _, ok := byName[name]; ok {
			t.Errorf("実機で起きない単位 %q が作られている", name)
		}
	}
}

// TestRefine_Dhalsim_Adjacency は、ダルシムでも連打版に化けるかどうかに関わらず、単発のチェーン員の
// 隣り合いを並べ替えるか落とすことを固定する(追補②で「化ける組だけ」へ狭めたのは誤り＝追補④で是正)。
// 実機(KA 58): 「屈弱P > 屈弱P > 屈強K」(単発 2 つ)は、最速で連打キャンセルになり 5F 早く持続が切れた。
func TestRefine_Dhalsim_Adjacency(t *testing.T) {
	moves, _ := dhalsimChainMoves()
	slp, clp := moves[0], moves[1]
	dash := setplayrepo.MoveCandidate{ID: 7, Code: model.MoveCodeDashForward, Category: model.MoveCategorySystem, Total: ptr(19)}

	// しゃがみ弱P ＋ しゃがみ弱P: 化けないが連打キャンセルになる ⇒ 落ちる。
	same := Proposal{parts: []proposalPart{singlePart(clp, RoleFiller), singlePart(clp, RoleFiller)}}
	same.assemble()
	// しゃがみ弱P ＋ 立ち弱P ＋ 前ステップ: 化けない組 ⇒ 並べ替える。
	plain := Proposal{parts: []proposalPart{singlePart(clp, RoleFiller), singlePart(slp, RoleFiller), singlePart(dash, RoleFiller)}}
	plain.assemble()
	// 立ち弱P ＋ しゃがみ弱P ＋ 前ステップ: 化ける組 ⇒ 並べ替える。
	rapid := Proposal{parts: []proposalPart{singlePart(slp, RoleFiller), singlePart(clp, RoleFiller), singlePart(dash, RoleFiller)}}
	rapid.assemble()
	out := refineProposals([]Proposal{same, plain, rapid}, SuggestParams{}, ModeMeaty)
	var got []string
	for _, p := range out {
		got = append(got, codesOf(p))
	}
	want := "crouching_light_punch>dash_forward>standing_light_punch,standing_light_punch>dash_forward>crouching_light_punch"
	if strings.Join(got, ",") != want {
		t.Errorf("ダルシムの隣り合い = %s, want %s", strings.Join(got, ","), want)
	}
}

// --- 組み合わせ ・温存 ---

// TestRefine_FiltersCombineWithAND は 3 つの切り替えが AND で組み合わさることを固定する。
func TestRefine_FiltersCombineWithAND(t *testing.T) {
	mk := func(name string, approach, chain bool, ob *int) Proposal {
		return Proposal{sortName: name, recipeHash: name, N: 1, hasApproach: approach, hasChainCancel: chain, targetOnBlock: ob}
	}
	in := func() []Proposal {
		return []Proposal{
			mk("all", true, true, ptr(0)),
			mk("noApproach", false, true, ptr(0)),
			mk("noChain", true, false, ptr(0)),
			mk("minusBlock", true, true, ptr(-1)),
		}
	}
	names := func(ps []Proposal) string {
		var s []string
		for _, p := range ps {
			s = append(s, p.sortName)
		}
		return strings.Join(s, ",")
	}
	all := SuggestParams{ApproachOnly: true, ChainCancelOnly: true, BlockSafeOnly: true}
	if got := names(refineProposals(in(), all, ModeMeaty)); got != "all" {
		t.Errorf("AND(meaty) = %s, want all", got)
	}
	if got := names(refineProposals(in(), all, ModeGap)); got != "all,minusBlock" {
		t.Errorf("AND(gap。J は効かない) = %s, want all,minusBlock", got)
	}
	if got := names(refineProposals(in(), SuggestParams{}, ModeMeaty)); got != "all,noApproach,noChain,minusBlock" {
		t.Errorf("すべて OFF で行が変わった: %s", got)
	}
}

// TestRefine_AllOff_PreservesRowsWithoutDuplicates は、切り替えがすべて OFF で重複も型 A も無い
// 条件では、結果の行と順が変わらないことを固定する(指示書 §2.2 ・§7.1-2 の「無い条件」)。
// 「在る条件」(重複がある)は TestRefine_DedupeTypeB_KeepsCheaperCost が見る。
func TestRefine_AllOff_PreservesRowsWithoutDuplicates(t *testing.T) {
	svc := newService(newCombo(ptr(40)), gc1Moves(), nil)
	res := suggestAll(t, svc, SuggestParams{NMin: 1, TargetTypes: allTypes()})
	raw := mustEngine(t, 40, gc1Moves(), "collarbone_breaker")
	rawStd := mustEngine(t, 40, gc1Moves(), "standing_light_kick")
	if got, want := len(res.Proposals), len(raw)+len(rawStd); got != want {
		t.Errorf("OFF の行数 = %d, want エンジンの解の数 %d", got, want)
	}
	if countRecipe(res, "standing_light_kick>collarbone_breaker") != 1 {
		t.Errorf("GC-1 の行が消えた: %s", recipesOf(res))
	}
}

// --- ヘルパ ---

func recipesOf(res *SuggestResult) string {
	var rs []string
	for _, p := range res.Proposals {
		rs = append(rs, codesOf(p))
	}
	sortStrings(rs)
	return strings.Join(rs, ",")
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// mustEngine はサービスと同じ繋ぎの単位でエンジンを 1 つの当てる技について直接呼ぶ(前提の確認用)。
func mustEngine(t *testing.T, ka int, moves []setplayrepo.MoveCandidate, targetCode string) []Suggestion {
	t.Helper()
	parents := map[int64][]int64{}
	singles := collectFillerSingles(moves, parents)
	units, _ := buildFillerUnits(singles, moves, parents, ka+2, newCancelProbe(nil))
	fillers := make([]Move, 0, len(units))
	for _, u := range units {
		fillers = append(fillers, Move{Name: u.Name, TotalFrames: u.Total})
	}
	var tc *setplayrepo.MoveCandidate
	for i := range moves {
		if moves[i].Code == targetCode {
			tc = &moves[i]
		}
	}
	opt := DefaultOptions()
	opt.MaxResults = engineSafetyCap
	sgs, err := Suggest(ka, fillers, Target{Name: tc.Code, Startup: *tc.Startup, Active: *tc.Active}, opt)
	if err != nil {
		t.Fatal(err)
	}
	return sgs
}

// fillerNamesJoined は解の繋ぎの単位名を "+" で連ねる。
func fillerNamesJoined(sg Suggestion) string {
	names := make([]string, len(sg.Fillers))
	for i, f := range sg.Fillers {
		names[i] = f.Name
	}
	return strings.Join(names, "+")
}
