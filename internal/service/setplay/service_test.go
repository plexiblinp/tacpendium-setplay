package setplay

import (
	"context"
	"testing"

	"github.com/plexiblinp/tacpendium-setplay/internal/model"
	setplayrepo "github.com/plexiblinp/tacpendium-setplay/internal/repository/setplay"
)

// --- フェイク ---
//
// 単体版: 本家のテストは「コンボ(KA の入れ物)」を作り SuggestForCombo を呼ぶ。単体版にコンボは無いが、
// 本家のテストの本文をそのまま移植できるよう(本家の差分を同じ場所に当てるため)、ヘルパだけを差し替える。
// テストの本文が使う newCombo / newService / svc.SuggestForCombo は、下の testService が
// 単体版の入口 SuggestForKnockdown へ渡す。docs/upstream-diff.md。

// testCharCode は fixture のキャラ code(接近技の辞書に載らない code。本家の fakeMoveReader.CharacterCode の既定値)。
const testCharCode = "test_char"

type fakeMoveReader struct {
	moves       []setplayrepo.MoveCandidate
	derivations []setplayrepo.MoveDerivation
}

func (f *fakeMoveReader) ListCharacterMoveCandidates(ctx context.Context, characterCode string) ([]setplayrepo.MoveCandidate, error) {
	return f.moves, nil
}

func (f *fakeMoveReader) ListCharacterMoveDerivations(ctx context.Context, characterCode string) ([]setplayrepo.MoveDerivation, error) {
	return f.derivations, nil
}

// fakeDup は本家の alreadyAdopted 用のフェイクの名残である。単体版は alreadyAdopted を持たないので中身は無く、
// 本家のテストの呼び出しの形(newService(..., nil))を保つためだけに残す。常に nil を渡す。
type fakeDup struct{}

func ptr(n int) *int { return &n }

// testCombo は本家の model.Combo のうち、テストが使う KA だけを持つ入れ物。
type testCombo struct {
	KnockdownAdvantage *int
}

func newCombo(ka *int) *testCombo {
	return &testCombo{KnockdownAdvantage: ka}
}

// testService は本家のテストの呼び出し(svc.SuggestForCombo(ctx, comboID, params))を単体版の入口へ渡す。
type testService struct {
	svc   Service
	combo *testCombo
}

// SuggestForCombo は combo の KA で SuggestForKnockdown を呼ぶ。comboID は使わない。
func (s testService) SuggestForCombo(ctx context.Context, _ int64, params SuggestParams) (*SuggestResult, error) {
	return s.svc.SuggestForKnockdown(ctx, testCharCode, *s.combo.KnockdownAdvantage, params)
}

// allTypes は全対象種別(テストで種別に依らず候補を出したいとき用)。
func allTypes() []string {
	return []string{
		TargetTypeNormal, TargetTypeUnique, TargetTypeSpecial,
		TargetTypeSpecialProjectile, TargetTypeThrow,
	}
}

// GC-1 相当の最小データ: KA=40, filler 立ち弱K(normal,total18), target 鎖骨割り(unique,startup20,active4,dmg600)。
func gc1Moves() []setplayrepo.MoveCandidate {
	return []setplayrepo.MoveCandidate{
		{ID: 10, Code: "standing_light_kick", Category: model.MoveCategoryNormal, Startup: ptr(5), Active: ptr(3), Total: ptr(18), Damage: ptr(300)},
		{ID: 11, Code: "collarbone_breaker", Category: model.MoveCategoryUnique, Startup: ptr(20), Active: ptr(4), Total: ptr(42), Damage: ptr(600)},
	}
}

func newService(combo *testCombo, moves []setplayrepo.MoveCandidate, dup *fakeDup) testService {
	return newServiceWithDerivations(combo, moves, nil, dup)
}

// newServiceWithDerivations は move_derivations(表記用親・単独入力不可の判定に使う)を与える版。
func newServiceWithDerivations(combo *testCombo, moves []setplayrepo.MoveCandidate, derivations []setplayrepo.MoveDerivation, _ *fakeDup) testService {
	return testService{svc: NewService(&fakeMoveReader{moves: moves, derivations: derivations}), combo: combo}
}

// --- テスト ---

func TestService_GC1_ProducesExpectedRecipe(t *testing.T) {
	svc := newService(newCombo(ptr(40)), gc1Moves(), nil)
	res, err := svc.SuggestForCombo(context.Background(), 1, SuggestParams{TargetTypes: allTypes()})
	if err != nil {
		t.Fatal(err)
	}
	var found *Proposal
	for i := range res.Proposals {
		p := &res.Proposals[i]
		if len(p.Steps) == 2 && p.Steps[0].Code == "standing_light_kick" && p.Steps[1].Code == "collarbone_breaker" {
			found = p
		}
	}
	if found == nil {
		t.Fatalf("expected 立ち弱K>鎖骨割り recipe, got %+v", res.Proposals)
	}
	if found.N != 4 || found.Landing != 41 || found.S != 38 {
		t.Errorf("want N=4 landing=41 S=38, got N=%d landing=%d S=%d", found.N, found.Landing, found.S)
	}
	if found.Steps[0].Role != RoleFiller || found.Steps[1].Role != RoleTarget {
		t.Errorf("roles wrong: %+v", found.Steps)
	}
}

func TestService_TargetMoveID_OnlyThatTarget(t *testing.T) {
	moves := gc1Moves()
	tid := int64(11) // collarbone_breaker のみ target
	svc := newService(newCombo(ptr(40)), moves, nil)
	// 明示指定は TargetTypes を無視する(空でも当該技のみ target になる)。
	res, err := svc.SuggestForCombo(context.Background(), 1, SuggestParams{TargetMoveID: &tid})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range res.Proposals {
		last := p.Steps[len(p.Steps)-1]
		if last.MoveID != 11 {
			t.Errorf("target must be move 11, got %d", last.MoveID)
		}
	}
	if len(res.Proposals) == 0 {
		t.Error("expected at least one proposal for explicit target")
	}
}

func TestService_TargetMoveID_BypassesTypeDamageDerivedAerial(t *testing.T) {
	// is_derived=true・is_aerial=true・damage=0・種別 rush の move を明示指定 → 全ゲートを外して target 化。
	moves := []setplayrepo.MoveCandidate{
		{ID: 10, Code: "filler", Category: model.MoveCategoryNormal, Startup: ptr(5), Active: ptr(3), Total: ptr(18), Damage: ptr(300)},
		{ID: 12, Code: "weird", Category: model.MoveCategoryRushVariant, Startup: ptr(20), Active: ptr(4), Total: ptr(42), Damage: ptr(0), IsDerived: true, IsAerial: true},
	}
	tid := int64(12)
	svc := newService(newCombo(ptr(40)), moves, nil)
	res, err := svc.SuggestForCombo(context.Background(), 1, SuggestParams{TargetMoveID: &tid})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Proposals) == 0 {
		t.Fatal("explicit target must bypass type/damage/is_derived/is_aerial gates")
	}
}

// M19-05(§2.6・M19-DESIGN-07 §3): target ゲートを is_derived / is_aerial 依存から
// basis ＋ 親参照 ＋ air フラグ導出へ切り替えた。解禁側と非解禁側を対で固定する。
func TestService_AutoEnumeration_TargetGateByBasisAndFastestUnreachable(t *testing.T) {
	moves := []setplayrepo.MoveCandidate{
		{ID: 10, Code: "filler", Category: model.MoveCategoryNormal, Startup: ptr(5), Active: ptr(3), Total: ptr(18), Damage: ptr(300), StartupBasis: model.MoveStartupBasisStandalone},
		// 非解禁 1: 状態変種(is_derived=1 かつ through でない)は引き続き除外(DESIGN-07 §9-4)。
		{ID: 12, Code: "state_variant", Category: model.MoveCategoryUnique, Startup: ptr(20), Active: ptr(4), Total: ptr(42), Damage: ptr(600), IsDerived: true, StartupBasis: model.MoveStartupBasisStandalone},
		// 非解禁 2: 単独最速では地上に当たらない技は target にできない。
		{ID: 13, Code: "fastest_unreachable", Category: model.MoveCategoryUnique, Startup: ptr(20), Active: ptr(4), Total: ptr(42), Damage: ptr(600), StartupBasis: model.MoveStartupBasisStandalone, FastestUnreachable: true},
		// 非解禁 3: 単独入力不可(親参照あり × standalone)。
		{ID: 14, Code: "solo_unavailable", Category: model.MoveCategoryUnique, Startup: ptr(20), Active: ptr(4), Total: ptr(42), Damage: ptr(600), IsDerived: true, StartupBasis: model.MoveStartupBasisStandalone},
		// 解禁 1: through の派生技は通し値で S の意味が確定するので target になる。
		{ID: 15, Code: "through_target", Category: model.MoveCategoryUnique, Startup: ptr(20), Active: ptr(4), Total: ptr(42), Damage: ptr(600), IsDerived: true, StartupBasis: model.MoveStartupBasisThrough},
		// 解禁 2: is_aerial=1 でも fastest_unreachable=0 なら target になる(kimberly elbow_drop 型)。
		{ID: 16, Code: "aerial_reachable", Category: model.MoveCategoryUnique, Startup: ptr(20), Active: ptr(4), Total: ptr(42), Damage: ptr(600), IsAerial: true, StartupBasis: model.MoveStartupBasisThrough},
	}
	derivations := []setplayrepo.MoveDerivation{{ChildMoveID: 14, ParentMoveID: 10}}
	svc := newServiceWithDerivations(newCombo(ptr(40)), moves, derivations, nil)
	res, err := svc.SuggestForCombo(context.Background(), 1, SuggestParams{TargetTypes: allTypes()})
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, p := range res.Proposals {
		for _, st := range p.Steps {
			if st.Role == RoleTarget {
				seen[st.Code] = true
			}
		}
	}
	for _, code := range []string{"state_variant", "fastest_unreachable", "solo_unavailable"} {
		if seen[code] {
			t.Errorf("target にしてはならない技が target になった: %s", code)
		}
	}
	for _, code := range []string{"through_target", "aerial_reachable"} {
		if !seen[code] {
			t.Errorf("解禁されるべき技が target になっていない: %s", code)
		}
	}
}

func TestService_TargetTypeFilter(t *testing.T) {
	// 通常技 target(t_normal)と 必殺技(弾) target(t_proj)を用意し、種別選択で切り分ける。
	moves := []setplayrepo.MoveCandidate{
		{ID: 10, Code: "f18", Category: model.MoveCategoryNormal, Startup: ptr(5), Active: ptr(3), Total: ptr(18), Damage: ptr(300)},
		{ID: 11, Code: "t_normal", Category: model.MoveCategoryNormal, Startup: ptr(20), Active: ptr(4), Total: ptr(42), Damage: ptr(600)},
		{ID: 12, Code: "t_proj", Category: model.MoveCategorySpecial, Startup: ptr(20), Active: ptr(4), Total: ptr(42), Damage: ptr(600), IsProjectile: true},
	}
	svc := newService(newCombo(ptr(40)), moves, nil)
	targetOf := func(p Proposal) string { return p.Steps[len(p.Steps)-1].Code }

	// normal のみ → t_proj(必殺技弾)は target に出ない(f18/t_normal は normal で出得る)。
	resN, _ := svc.SuggestForCombo(context.Background(), 1, SuggestParams{TargetTypes: []string{TargetTypeNormal}})
	for _, p := range resN.Proposals {
		if targetOf(p) == "t_proj" {
			t.Errorf("normal-only must not target projectile special t_proj")
		}
	}
	// special_projectile のみ → target は t_proj のみ(normal の f18/t_normal は出ない)。
	resP, _ := svc.SuggestForCombo(context.Background(), 1, SuggestParams{TargetTypes: []string{TargetTypeSpecialProjectile}})
	for _, p := range resP.Proposals {
		if targetOf(p) != "t_proj" {
			t.Errorf("projectile-only: unexpected target %s", targetOf(p))
		}
	}
	if len(resN.Proposals) == 0 || len(resP.Proposals) == 0 {
		t.Error("both type filters should yield at least one proposal")
	}
	// special(非弾) のみ → t_proj は special_projectile なので出ない(空)。
	resS, _ := svc.SuggestForCombo(context.Background(), 1, SuggestParams{TargetTypes: []string{TargetTypeSpecial}})
	if len(resS.Proposals) != 0 {
		t.Errorf("non-projectile special filter should not match projectile special, got %d", len(resS.Proposals))
	}
}

func TestService_ZeroDamageExcludedByDefault(t *testing.T) {
	// ダメージ 0 の通常技 target(ドライブパリィ相当だが category=normal で type は通る)。
	moves := []setplayrepo.MoveCandidate{
		{ID: 10, Code: "f18", Category: model.MoveCategoryNormal, Startup: ptr(5), Active: ptr(3), Total: ptr(18), Damage: ptr(300)},
		{ID: 11, Code: "zero_dmg", Category: model.MoveCategoryNormal, Startup: ptr(20), Active: ptr(4), Total: ptr(42), Damage: ptr(0)},
	}
	svc := newService(newCombo(ptr(40)), moves, nil)

	// 既定: damage 0 の target は出ない。
	resDefault, _ := svc.SuggestForCombo(context.Background(), 1, SuggestParams{TargetTypes: []string{TargetTypeNormal}})
	for _, p := range resDefault.Proposals {
		if last := p.Steps[len(p.Steps)-1]; last.Code == "zero_dmg" {
			t.Error("damage=0 target must be excluded by default")
		}
	}
	// IncludeZeroDamage: damage 0 でも target になる。
	resInclude, _ := svc.SuggestForCombo(context.Background(), 1, SuggestParams{TargetTypes: []string{TargetTypeNormal}, IncludeZeroDamage: true})
	var sawZero bool
	for _, p := range resInclude.Proposals {
		if last := p.Steps[len(p.Steps)-1]; last.Code == "zero_dmg" {
			sawZero = true
		}
	}
	if !sawZero {
		t.Error("IncludeZeroDamage should allow damage=0 target")
	}
}

func TestService_SystemNeverTargeted(t *testing.T) {
	// category=system(ドライブパリィ)は type="" のため IncludeZeroDamage でも自動 target にならない。
	moves := []setplayrepo.MoveCandidate{
		{ID: 10, Code: "f18", Category: model.MoveCategoryNormal, Startup: ptr(5), Active: ptr(3), Total: ptr(18), Damage: ptr(300)},
		{ID: 11, Code: "drive_parry", Category: model.MoveCategorySystem, Startup: ptr(1), Active: ptr(12), Total: ptr(45), Damage: ptr(0)},
	}
	svc := newService(newCombo(ptr(40)), moves, nil)
	res, _ := svc.SuggestForCombo(context.Background(), 1, SuggestParams{TargetTypes: allTypes(), IncludeZeroDamage: true})
	for _, p := range res.Proposals {
		if last := p.Steps[len(p.Steps)-1]; last.Code == "drive_parry" {
			t.Error("system move must never be an auto target")
		}
	}
}

func TestService_RushTargetTypes(t *testing.T) {
	// filler と、通常技のラッシュ版(元 id=10 normal→normal_rush)・特殊技のラッシュ版(元 id=11 unique→unique_rush)。
	moves := []setplayrepo.MoveCandidate{
		{ID: 10, Code: "standing_light_kick", Category: model.MoveCategoryNormal, Startup: ptr(5), Active: ptr(3), Total: ptr(18), Damage: ptr(300)},
		{ID: 11, Code: "collarbone_breaker", Category: model.MoveCategoryUnique, Startup: ptr(20), Active: ptr(4), Total: ptr(42), Damage: ptr(600)},
		// filler(total=24): rush 立ち弱K(startup16) の帯 sumF∈[23,25] を埋める。
		{ID: 30, Code: "f24", Category: model.MoveCategoryNormal, Startup: ptr(3), Active: ptr(2), Total: ptr(24), Damage: ptr(100)},
		{ID: 20, Code: "rush_standing_light_kick", Category: model.MoveCategoryRushVariant, OriginalMoveID: ptr64(10), Startup: ptr(16), Active: ptr(3), Total: ptr(29), Damage: ptr(300), IsDerived: true},
		{ID: 21, Code: "rush_collarbone_breaker", Category: model.MoveCategoryRushVariant, OriginalMoveID: ptr64(11), Startup: ptr(20), Active: ptr(4), Total: ptr(53), Damage: ptr(600), IsDerived: true},
	}
	svc := newService(newCombo(ptr(40)), moves, nil)
	targetOf := func(p Proposal) string { return p.Steps[len(p.Steps)-1].Code }

	// rush 種別を選ばない(既定) → ラッシュ版は target に出ない(is_derived で除外)。
	resDefault, _ := svc.SuggestForCombo(context.Background(), 1, SuggestParams{TargetTypes: []string{TargetTypeNormal, TargetTypeUnique}})
	for _, p := range resDefault.Proposals {
		if c := targetOf(p); c == "rush_standing_light_kick" || c == "rush_collarbone_breaker" {
			t.Errorf("rush target must be excluded when rush type not selected: %s", c)
		}
	}

	// normal_rush を選ぶ → rush_standing_light_kick が target になり得る、rush_collarbone_breaker は出ない。
	resNR, _ := svc.SuggestForCombo(context.Background(), 1, SuggestParams{TargetTypes: []string{TargetTypeNormalRush}})
	var sawNR bool
	for _, p := range resNR.Proposals {
		c := targetOf(p)
		if c == "rush_standing_light_kick" {
			sawNR = true
		}
		if c == "rush_collarbone_breaker" {
			t.Error("unique-rush must not appear under normal_rush filter")
		}
	}
	if !sawNR {
		t.Error("normal_rush filter should surface the rush-of-normal target")
	}

	// unique_rush を選ぶ → rush_collarbone_breaker が出る。
	resUR, _ := svc.SuggestForCombo(context.Background(), 1, SuggestParams{TargetTypes: []string{TargetTypeUniqueRush}})
	var sawUR bool
	for _, p := range resUR.Proposals {
		if targetOf(p) == "rush_collarbone_breaker" {
			sawUR = true
		}
	}
	if !sawUR {
		t.Error("unique_rush filter should surface the rush-of-unique target")
	}
}

// TestService_RushTargetRequiresResolvedOriginal は M35-03 の実害そのものを固定する。
//
// ★★moveTargetType(service.go:144)は rush_variant の種別を元技の category で決めるため、
//
//	OriginalMoveID == nil のとき "" を返す。collectTargets(:641)は ttype == "" を continue で
//	落とす。⇒ 元技を解決できないラッシュ版は、どの種別フィルタを選んでも target にならない。
//
// ★★★これはエラーにならず警告も出ない。"" は system / SA / 元 special のラッシュと同じ
//
//	バケツであり、**データ欠陥と意図的除外が区別できない**。⇒ 人が読む以外に見つける経路が
//	無かった。実際 ingrid 2 / lily 1 / mai 1 の 4 技が、CSV の original_move_code の
//	dangling によってこの経路で静かに脱落していた(M35-03 / DES-004 §3.2.1)。
//
// ★対で主張する(SUPP-001 §5.5.2 (3))。⇒ 解決できる側が出ることだけを見ると、
//
//	「落ちる側も出てしまっている」形を取り落とす。逆も同じである。
//
// ★実装は正しい。本テストが守るのはデータ側の前提である——
//
//	「original_move_id が解けていないと候補にならない」。⇒ ここが緑のまま
//	ラッシュ版が提案に出ないときは、疑うのは規則ではなく seed である。
func TestService_RushTargetRequiresResolvedOriginal(t *testing.T) {
	// 同じ形の 2 本を並べる。違いは OriginalMoveID が解けているかどうかだけである。
	moves := []setplayrepo.MoveCandidate{
		{ID: 10, Code: "base_unique", Category: model.MoveCategoryUnique, Startup: ptr(20), Active: ptr(4), Total: ptr(42), Damage: ptr(600)},
		// filler: meaty KA=40 / target startup=20 active=4 → budgetMax=21・sumF∈[18,21]。
		{ID: 30, Code: "f18", Category: model.MoveCategoryNormal, Startup: ptr(5), Active: ptr(3), Total: ptr(18), Damage: ptr(100)},
		{ID: 20, Code: "rush_resolved", Category: model.MoveCategoryRushVariant, OriginalMoveID: ptr64(10), Startup: ptr(20), Active: ptr(4), Total: ptr(53), Damage: ptr(600), IsDerived: true},
		// ★元技を解決できない側。★他のフィールドは解決側と 1 つも違わない。
		{ID: 21, Code: "rush_dangling", Category: model.MoveCategoryRushVariant, OriginalMoveID: nil, Startup: ptr(20), Active: ptr(4), Total: ptr(53), Damage: ptr(600), IsDerived: true},
	}
	svc := newService(newCombo(ptr(40)), moves, nil)
	targetOf := func(p Proposal) string { return p.Steps[len(p.Steps)-1].Code }

	// ★★unique_rush を名指しで選ぶ——解決側は出る／未解決側は出ない。これが対である。
	//   ⇒ 2 本は OriginalMoveID 以外のフィールドが 1 つも違わないので、差が出る原因は
	//     service.go:144 の分岐しかない。
	resUR, err := svc.SuggestForCombo(context.Background(), 1, SuggestParams{TargetTypes: []string{TargetTypeUniqueRush}})
	if err != nil {
		t.Fatal(err)
	}
	var sawResolved, sawDangling bool
	for _, p := range resUR.Proposals {
		switch targetOf(p) {
		case "rush_resolved":
			sawResolved = true
		case "rush_dangling":
			sawDangling = true
		}
	}
	if !sawResolved {
		t.Error("unique_rush フィルタで解決側が target に出ない(維持側が壊れている)")
	}
	if sawDangling {
		t.Error("unique_rush フィルタで未解決側が target に出た(service.go:144 の分岐が効いていない)")
	}

	// ★全種別を選んでも未解決側は出ない。
	//   ⇒ 上の 1 本だけだと「uniqueRush という種別に当たらなかっただけで、
	//     別の種別でなら出る」という説明が残る。★"" はどの種別集合にも入らない。
	resAll, err := svc.SuggestForCombo(context.Background(), 1, SuggestParams{TargetTypes: allTypes()})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range resAll.Proposals {
		if targetOf(p) == "rush_dangling" {
			t.Error("全種別でも未解決側は target になってはならない")
		}
	}
}

func TestService_SetupOnlyIncludedAsFiller(t *testing.T) {
	moves := []setplayrepo.MoveCandidate{
		{ID: 20, Code: "so_filler", Category: model.MoveCategoryNormal, Startup: ptr(5), Active: ptr(3), Total: ptr(18), Damage: ptr(300), SetupOnly: true},
		{ID: 21, Code: "tgt", Category: model.MoveCategoryUnique, Startup: ptr(20), Active: ptr(4), Total: ptr(42), Damage: ptr(600)},
	}
	svc := newService(newCombo(ptr(40)), moves, nil)
	res, err := svc.SuggestForCombo(context.Background(), 1, SuggestParams{TargetTypes: allTypes()})
	if err != nil {
		t.Fatal(err)
	}
	var usedSetupOnlyFiller bool
	for _, p := range res.Proposals {
		for _, st := range p.Steps {
			if st.Role == RoleFiller && st.Code == "so_filler" {
				usedSetupOnlyFiller = true
			}
		}
	}
	if !usedSetupOnlyFiller {
		t.Errorf("setup_only=true move must be usable as filler, got %+v", res.Proposals)
	}
}

func TestService_Ranking_NDesc(t *testing.T) {
	// 2 つの target を持たせ、N の異なる解を作りランキング(sort=n)を確認する。
	moves := []setplayrepo.MoveCandidate{
		{ID: 10, Code: "f18", Category: model.MoveCategoryNormal, Startup: ptr(5), Active: ptr(3), Total: ptr(18), Damage: ptr(300)},
		{ID: 11, Code: "tgtA", Category: model.MoveCategoryNormal, Startup: ptr(20), Active: ptr(4), Total: ptr(42), Damage: ptr(600)},
		{ID: 12, Code: "tgtB", Category: model.MoveCategoryNormal, Startup: ptr(22), Active: ptr(2), Total: ptr(50), Damage: ptr(600)},
	}
	svc := newService(newCombo(ptr(40)), moves, nil)
	res, err := svc.SuggestForCombo(context.Background(), 1, SuggestParams{TargetTypes: []string{TargetTypeNormal}, Sort: SortByN})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Proposals) < 2 {
		t.Fatalf("need >=2 proposals, got %d", len(res.Proposals))
	}
	for i := 1; i < len(res.Proposals); i++ {
		if res.Proposals[i-1].N < res.Proposals[i].N {
			t.Errorf("sort=n: N must be descending, got %d before %d", res.Proposals[i-1].N, res.Proposals[i].N)
		}
	}
}

func TestService_SortByTarget(t *testing.T) {
	// sort=target は重ねる技(target code)昇順。
	moves := []setplayrepo.MoveCandidate{
		{ID: 10, Code: "f18", Category: model.MoveCategoryNormal, Startup: ptr(5), Active: ptr(3), Total: ptr(18), Damage: ptr(300)},
		{ID: 11, Code: "zzz_target", Category: model.MoveCategoryNormal, Startup: ptr(20), Active: ptr(4), Total: ptr(42), Damage: ptr(600)},
		{ID: 12, Code: "aaa_target", Category: model.MoveCategoryNormal, Startup: ptr(20), Active: ptr(4), Total: ptr(42), Damage: ptr(600)},
	}
	svc := newService(newCombo(ptr(40)), moves, nil)
	res, err := svc.SuggestForCombo(context.Background(), 1, SuggestParams{TargetTypes: []string{TargetTypeNormal}, Sort: SortByTarget})
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i < len(res.Proposals); i++ {
		prev := res.Proposals[i-1].Steps[len(res.Proposals[i-1].Steps)-1].Code
		cur := res.Proposals[i].Steps[len(res.Proposals[i].Steps)-1].Code
		if prev > cur {
			t.Errorf("sort=target: target code must be ascending, got %s before %s", prev, cur)
		}
	}
}

func ptr64(n int64) *int64 { return &n }
