package setplay

import (
	"errors"
	"reflect"
	"testing"
)

// fillerNames は提案を空振り技名の列に圧縮する(順序付き比較用)。
func fillerNames(s Suggestion) []string {
	out := make([]string, len(s.Fillers))
	for i, f := range s.Fillers {
		out[i] = f.Name
	}
	return out
}

// suggestionShapes は提案群を [][]string(各提案の空振り技名列)に変換する。
func suggestionShapes(ss []Suggestion) [][]string {
	out := make([][]string, len(ss))
	for i, s := range ss {
		out[i] = fillerNames(s)
	}
	return out
}

func mustSuggest(t *testing.T, ka int, moves []Move, target Target, opt Options) []Suggestion {
	t.Helper()
	got, err := Suggest(ka, moves, target, opt)
	if err != nil {
		t.Fatalf("Suggest returned unexpected error: %v", err)
	}
	return got
}

// --- 移植テスト(18 件・探索機構の回帰) ---
//
// 移植元 setplay-suggestion は完全一致エンジン(Σfiller.total + startup == KA)だった。
// 受理帯方式では Active = NMin = 2 とすると band = Active − NMin = 0 かつ
// budgetMax = KA + 2 − NMin − startup = KA − startup となり、移植元の予算・解集合・
// TotalFrames(S = KA) を厳密に再現する。よって移植 18 テストは Target に Active:2 を
// 足し、Options の NMin を 2 にするだけで期待値を変えずに通る(§4.1.6 の再計算は
// この構成では不要=0 件。窓方式そのものは後半の新規 10 観点で検証する)。

// portedOpts は移植元(完全一致)を受理帯方式で再現する設定(band=0, budget=KA−startup)。
func portedOpts() Options {
	o := DefaultOptions()
	o.NMin = 2
	return o
}

func TestExactMatchNoFiller(t *testing.T) {
	// KA == 発生 → 空振りなしで target をそのまま重ねる 1 件。S == KA。
	target := Target{Name: "236P", Startup: 13, Active: 2}
	got := mustSuggest(t, 13, []Move{{Name: "5LP", TotalFrames: 20}}, target, portedOpts())

	if len(got) != 1 {
		t.Fatalf("want 1 suggestion, got %d: %+v", len(got), got)
	}
	if len(got[0].Fillers) != 0 {
		t.Errorf("want 0 fillers, got %+v", got[0].Fillers)
	}
	if got[0].Target != target {
		t.Errorf("target mismatch: %+v", got[0].Target)
	}
	if got[0].TotalFrames != 13 {
		t.Errorf("TotalFrames(S) want 13, got %d", got[0].TotalFrames)
	}
}

func TestSingleFiller(t *testing.T) {
	// budget = 33。33F ちょうどの技 1 つで成立。S == KA == 46。
	target := Target{Name: "T", Startup: 13, Active: 2}
	moves := []Move{
		{Name: "A", TotalFrames: 10},
		{Name: "B", TotalFrames: 20},
		{Name: "C", TotalFrames: 33},
	}
	got := mustSuggest(t, 46, moves, target, portedOpts())

	want := [][]string{{"C"}}
	if shapes := suggestionShapes(got); !reflect.DeepEqual(shapes, want) {
		t.Errorf("shapes = %v, want %v", shapes, want)
	}
	if got[0].TotalFrames != 46 {
		t.Errorf("TotalFrames(S) want 46, got %d", got[0].TotalFrames)
	}
}

func TestRepeatAllowedFewestFirst(t *testing.T) {
	target := Target{Name: "T", Startup: 10, Active: 2}
	moves := []Move{
		{Name: "A", TotalFrames: 10},
		{Name: "B", TotalFrames: 15},
	}
	got := mustSuggest(t, 40, moves, target, portedOpts())

	want := [][]string{{"B", "B"}, {"A", "A", "A"}}
	if shapes := suggestionShapes(got); !reflect.DeepEqual(shapes, want) {
		t.Errorf("shapes = %v, want %v", shapes, want)
	}
}

func TestRepeatDisallowed(t *testing.T) {
	target := Target{Name: "T", Startup: 10, Active: 2}
	moves := []Move{
		{Name: "A", TotalFrames: 10},
		{Name: "B", TotalFrames: 15},
	}
	opt := portedOpts()
	opt.AllowRepeat = false
	got := mustSuggest(t, 40, moves, target, opt)

	if len(got) != 0 {
		t.Errorf("want no suggestions without repeat, got %v", suggestionShapes(got))
	}
}

func TestDistinctMovesSameFrames(t *testing.T) {
	target := Target{Name: "T", Startup: 10, Active: 2}
	moves := []Move{
		{Name: "A", TotalFrames: 20},
		{Name: "B", TotalFrames: 20},
	}
	opt := portedOpts()
	opt.AllowRepeat = false
	got := mustSuggest(t, 50, moves, target, opt) // budget = 40

	want := [][]string{{"A", "B"}}
	if shapes := suggestionShapes(got); !reflect.DeepEqual(shapes, want) {
		t.Errorf("shapes = %v, want %v", shapes, want)
	}
}

func TestNoSolution(t *testing.T) {
	target := Target{Name: "T", Startup: 13, Active: 2}
	moves := []Move{{Name: "A", TotalFrames: 10}, {Name: "B", TotalFrames: 20}}
	got := mustSuggest(t, 20, moves, target, portedOpts())
	if len(got) != 0 {
		t.Errorf("want empty, got %v", suggestionShapes(got))
	}
}

func TestKnockdownLessThanStartup(t *testing.T) {
	target := Target{Name: "T", Startup: 13, Active: 2}
	got := mustSuggest(t, 10, []Move{{Name: "A", TotalFrames: 5}}, target, portedOpts())
	if len(got) != 0 {
		t.Errorf("want empty, got %v", suggestionShapes(got))
	}
}

func TestFilterNonPositiveTotal(t *testing.T) {
	target := Target{Name: "T", Startup: 10, Active: 2}
	moves := []Move{
		{Name: "Unset", TotalFrames: 0},
		{Name: "Bad", TotalFrames: -5},
		{Name: "Good", TotalFrames: 30},
	}
	got := mustSuggest(t, 40, moves, target, portedOpts()) // budget = 30
	want := [][]string{{"Good"}}
	if shapes := suggestionShapes(got); !reflect.DeepEqual(shapes, want) {
		t.Errorf("shapes = %v, want %v", shapes, want)
	}
}

func TestDeduplicateIdenticalMoves(t *testing.T) {
	target := Target{Name: "T", Startup: 10, Active: 2}
	moves := []Move{
		{Name: "A", TotalFrames: 30},
		{Name: "A", TotalFrames: 30},
		{Name: "A", TotalFrames: 30},
	}
	opt := portedOpts()
	opt.AllowRepeat = false
	got := mustSuggest(t, 40, moves, target, opt) // budget = 30
	want := [][]string{{"A"}}
	if shapes := suggestionShapes(got); !reflect.DeepEqual(shapes, want) {
		t.Errorf("shapes = %v, want %v", shapes, want)
	}
}

func TestMaxFillersCap(t *testing.T) {
	target := Target{Name: "T", Startup: 10, Active: 2}
	moves := []Move{{Name: "A", TotalFrames: 10}}

	full := mustSuggest(t, 40, moves, target, portedOpts())
	if want := [][]string{{"A", "A", "A"}}; !reflect.DeepEqual(suggestionShapes(full), want) {
		t.Fatalf("uncapped shapes = %v, want %v", suggestionShapes(full), want)
	}

	opt := portedOpts()
	opt.MaxFillers = 2
	capped := mustSuggest(t, 40, moves, target, opt)
	if len(capped) != 0 {
		t.Errorf("want empty with MaxFillers=2, got %v", suggestionShapes(capped))
	}
}

func TestMaxResultsCap(t *testing.T) {
	target := Target{Name: "T", Startup: 10, Active: 2}
	moves := []Move{
		{Name: "A", TotalFrames: 10},
		{Name: "B", TotalFrames: 20},
	}

	all := mustSuggest(t, 50, moves, target, portedOpts())
	wantAll := [][]string{{"B", "B"}, {"A", "A", "B"}, {"A", "A", "A", "A"}}
	if !reflect.DeepEqual(suggestionShapes(all), wantAll) {
		t.Fatalf("uncapped shapes = %v, want %v", suggestionShapes(all), wantAll)
	}

	opt := portedOpts()
	opt.MaxResults = 2
	capped := mustSuggest(t, 50, moves, target, opt)
	wantCapped := [][]string{{"B", "B"}, {"A", "A", "B"}}
	if !reflect.DeepEqual(suggestionShapes(capped), wantCapped) {
		t.Errorf("capped shapes = %v, want %v", suggestionShapes(capped), wantCapped)
	}
}

func TestFillersCanonicalAscending(t *testing.T) {
	target := Target{Name: "T", Startup: 10, Active: 2}
	moves := []Move{
		{Name: "Big", TotalFrames: 20},
		{Name: "Small", TotalFrames: 10},
	}
	got := mustSuggest(t, 50, moves, target, portedOpts()) // budget = 40
	found := false
	for _, s := range got {
		if reflect.DeepEqual(fillerNames(s), []string{"Small", "Small", "Big"}) {
			found = true
		}
		for i := 1; i < len(s.Fillers); i++ {
			if s.Fillers[i-1].TotalFrames > s.Fillers[i].TotalFrames {
				t.Errorf("fillers not ascending: %v", fillerNames(s))
			}
		}
	}
	if !found {
		t.Errorf("expected a [Small,Small,Big] suggestion, got %v", suggestionShapes(got))
	}
}

func TestNoCandidatesBudgetPositive(t *testing.T) {
	target := Target{Name: "T", Startup: 10, Active: 2}
	got := mustSuggest(t, 40, nil, target, portedOpts())
	if len(got) != 0 {
		t.Errorf("want empty with no candidates and positive budget, got %v", suggestionShapes(got))
	}
}

func TestNoCandidatesBudgetZero(t *testing.T) {
	target := Target{Name: "T", Startup: 13, Active: 2}
	got := mustSuggest(t, 13, nil, target, portedOpts())
	if len(got) != 1 || len(got[0].Fillers) != 0 {
		t.Errorf("want single no-filler suggestion, got %v", suggestionShapes(got))
	}
}

func TestErrors(t *testing.T) {
	target := Target{Name: "T", Startup: 13, Active: 2}
	if _, err := Suggest(-1, nil, target, portedOpts()); !errors.Is(err, ErrNegativeAdvantage) {
		t.Errorf("want ErrNegativeAdvantage, got %v", err)
	}
	if _, err := Suggest(20, nil, Target{Name: "T", Startup: 0, Active: 2}, portedOpts()); !errors.Is(err, ErrInvalidTargetStartup) {
		t.Errorf("want ErrInvalidTargetStartup for startup=0, got %v", err)
	}
	if _, err := Suggest(20, nil, Target{Name: "T", Startup: -3, Active: 2}, portedOpts()); !errors.Is(err, ErrInvalidTargetStartup) {
		t.Errorf("want ErrInvalidTargetStartup for negative startup, got %v", err)
	}
}

func TestMaxResultsPartialWithinDepth(t *testing.T) {
	target := Target{Name: "T", Startup: 10, Active: 2}
	moves := []Move{
		{Name: "A", TotalFrames: 20},
		{Name: "B", TotalFrames: 20},
	}

	all := mustSuggest(t, 30, moves, target, portedOpts()) // budget = 20
	if want := [][]string{{"A"}, {"B"}}; !reflect.DeepEqual(suggestionShapes(all), want) {
		t.Fatalf("uncapped shapes = %v, want %v", suggestionShapes(all), want)
	}

	opt := portedOpts()
	opt.MaxResults = 1
	capped := mustSuggest(t, 30, moves, target, opt)
	if want := [][]string{{"A"}}; !reflect.DeepEqual(suggestionShapes(capped), want) {
		t.Errorf("capped shapes = %v, want %v", suggestionShapes(capped), want)
	}
}

func TestLowerBoundPruningMixed(t *testing.T) {
	target := Target{Name: "T", Startup: 5, Active: 2}
	moves := []Move{
		{Name: "f7", TotalFrames: 7},
		{Name: "f3", TotalFrames: 3},
		{Name: "f5", TotalFrames: 5},
	}
	got := mustSuggest(t, 15, moves, target, portedOpts()) // budget = 10
	want := [][]string{{"f3", "f7"}, {"f5", "f5"}}
	if shapes := suggestionShapes(got); !reflect.DeepEqual(shapes, want) {
		t.Errorf("shapes = %v, want %v", shapes, want)
	}
}

func TestZeroOptionsDisallowsRepeat(t *testing.T) {
	// ゼロ値 Options は AllowRepeat=false(NMin=0 は 1 に丸められる)。
	// A(10) を 3 回必要だが repeat 不可 → 解なし。
	target := Target{Name: "T", Startup: 10, Active: 2}
	moves := []Move{{Name: "A", TotalFrames: 10}}
	got := mustSuggest(t, 40, moves, target, Options{})
	if len(got) != 0 {
		t.Errorf("zero Options should disallow repeat → empty, got %v", suggestionShapes(got))
	}
}

// --- 窓化の新規テスト(§5.1 の 10 観点) ---

// #1 帯の下限で成立: S = KA+2−Active の入力で解が返り、N == Active。
func TestWindowLowerBound(t *testing.T) {
	// KA=20, startup=5, Active=4, NMin=1 → budgetMax=16, band=3。
	// filler total=13 → remaining=3=band → N=4=Active, S=18=KA+2−Active。
	target := Target{Name: "T", Startup: 5, Active: 4}
	got := mustSuggest(t, 20, []Move{{Name: "X", TotalFrames: 13}}, target, DefaultOptions())
	if len(got) != 1 {
		t.Fatalf("want 1 suggestion at lower bound, got %d: %v", len(got), suggestionShapes(got))
	}
	if got[0].HitActiveFrame != target.Active {
		t.Errorf("N want %d(=Active), got %d", target.Active, got[0].HitActiveFrame)
	}
	if want := 20 + 2 - target.Active; got[0].TotalFrames != want {
		t.Errorf("S want %d(=KA+2−Active), got %d", want, got[0].TotalFrames)
	}
}

// #2 帯の上限で成立: S = KA+2−NMin の入力で解が返り、N == NMin。
func TestWindowUpperBound(t *testing.T) {
	// KA=20, startup=5, Active=4, NMin=1 → budgetMax=16。
	// filler total=16 → remaining=0 → N=1=NMin, S=21=KA+2−NMin。
	target := Target{Name: "T", Startup: 5, Active: 4}
	got := mustSuggest(t, 20, []Move{{Name: "Y", TotalFrames: 16}}, target, DefaultOptions())
	if len(got) != 1 {
		t.Fatalf("want 1 suggestion at upper bound, got %d: %v", len(got), suggestionShapes(got))
	}
	nMin := DefaultOptions().NMin
	if got[0].HitActiveFrame != nMin {
		t.Errorf("N want %d(=NMin), got %d", nMin, got[0].HitActiveFrame)
	}
	if want := 20 + 2 - nMin; got[0].TotalFrames != want {
		t.Errorf("S want %d(=KA+2−NMin), got %d", want, got[0].TotalFrames)
	}
}

// #3 帯の外(下限−1・上限+1)では解が返らない。
func TestWindowOutOfBand(t *testing.T) {
	// budgetMax=16, band=3。
	// Lo total=12 → remaining=4 → N=5>Active(4) 却下。
	// Hi total=17 → sumF>budgetMax → break で却下。
	target := Target{Name: "T", Startup: 5, Active: 4}
	got := mustSuggest(t, 20, []Move{{Name: "Lo", TotalFrames: 12}, {Name: "Hi", TotalFrames: 17}}, target, DefaultOptions())
	if len(got) != 0 {
		t.Errorf("want empty outside band, got %v", suggestionShapes(got))
	}
}

// #4 恒等式: 全解で N == KA+2−(Σfiller.total+Startup) かつ N == remaining+NMin。
func TestWindowIdentity(t *testing.T) {
	ka := 34
	nMin := DefaultOptions().NMin
	target := Target{Name: "T", Startup: 6, Active: 6}
	moves := []Move{{Name: "a", TotalFrames: 7}, {Name: "b", TotalFrames: 11}, {Name: "c", TotalFrames: 13}}
	got := mustSuggest(t, ka, moves, target, DefaultOptions())
	if len(got) == 0 {
		t.Fatal("expected at least one suggestion")
	}
	for _, s := range got {
		sumF := 0
		for _, f := range s.Fillers {
			sumF += f.TotalFrames
		}
		wantN := ka + 2 - (sumF + target.Startup)
		if s.HitActiveFrame != wantN {
			t.Errorf("N=%d, want KA+2−S=%d (fillers=%v)", s.HitActiveFrame, wantN, fillerNames(s))
		}
		if s.TotalFrames != sumF+target.Startup {
			t.Errorf("S=%d, want Σfiller+startup=%d", s.TotalFrames, sumF+target.Startup)
		}
		// remaining = budgetMax − sumF; N == remaining + NMin。
		budgetMax := ka + 2 - nMin - target.Startup
		remaining := budgetMax - sumF
		if s.HitActiveFrame != remaining+nMin {
			t.Errorf("N=%d, want remaining+NMin=%d", s.HitActiveFrame, remaining+nMin)
		}
		if s.HitActiveFrame < nMin || s.HitActiveFrame > target.Active {
			t.Errorf("N=%d out of [%d,%d]", s.HitActiveFrame, nMin, target.Active)
		}
	}
}

// #5 NMin > Active はエラーではなく空結果。
func TestWindowNMinGreaterThanActive(t *testing.T) {
	target := Target{Name: "T", Startup: 5, Active: 2}
	opt := DefaultOptions()
	opt.NMin = 5
	got, err := Suggest(30, []Move{{Name: "A", TotalFrames: 10}}, target, opt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("want empty when NMin>Active, got %v", suggestionShapes(got))
	}
}

// #6 NMin < 1 は 1 に丸められる(NMin=0 と NMin=1 が同一結果)。
func TestWindowNMinRoundedToOne(t *testing.T) {
	target := Target{Name: "T", Startup: 5, Active: 4}
	moves := []Move{{Name: "X", TotalFrames: 13}, {Name: "Y", TotalFrames: 16}}
	opt0 := DefaultOptions()
	opt0.NMin = 0
	opt1 := DefaultOptions()
	opt1.NMin = 1
	got0 := mustSuggest(t, 20, moves, target, opt0)
	got1 := mustSuggest(t, 20, moves, target, opt1)
	if !reflect.DeepEqual(got0, got1) {
		t.Errorf("NMin=0 should equal NMin=1: got0=%+v got1=%+v", got0, got1)
	}
	// 負の NMin も同様。
	optNeg := DefaultOptions()
	optNeg.NMin = -3
	gotNeg := mustSuggest(t, 20, moves, target, optNeg)
	if !reflect.DeepEqual(gotNeg, got1) {
		t.Errorf("negative NMin should equal NMin=1")
	}
}

// #7 Active < 1 は ErrInvalidTargetActive。
func TestWindowInvalidActive(t *testing.T) {
	if _, err := Suggest(30, nil, Target{Name: "T", Startup: 5, Active: 0}, DefaultOptions()); !errors.Is(err, ErrInvalidTargetActive) {
		t.Errorf("want ErrInvalidTargetActive for active=0, got %v", err)
	}
	if _, err := Suggest(30, nil, Target{Name: "T", Startup: 5, Active: -2}, DefaultOptions()); !errors.Is(err, ErrInvalidTargetActive) {
		t.Errorf("want ErrInvalidTargetActive for negative active, got %v", err)
	}
}

// #8 枝刈り健全性: 帯内に下限側・上限側の解が存在する入力で両方が列挙される。
func TestWindowPruningEnumeratesBothEnds(t *testing.T) {
	// KA=20, startup=5, Active=5, NMin=1 → budgetMax=16, band=4。
	// filler 12 → remaining=4 → N=5(下限側=最遅持続)。
	// filler 16 → remaining=0 → N=1(上限側=最速重ね)。
	target := Target{Name: "T", Startup: 5, Active: 5}
	got := mustSuggest(t, 20, []Move{{Name: "big", TotalFrames: 16}, {Name: "small", TotalFrames: 12}}, target, DefaultOptions())
	var sawLow, sawHigh bool
	for _, s := range got {
		if s.HitActiveFrame == target.Active { // N==5 下限側
			sawLow = true
		}
		if s.HitActiveFrame == DefaultOptions().NMin { // N==1 上限側
			sawHigh = true
		}
	}
	if !sawLow || !sawHigh {
		t.Errorf("pruning dropped a band-end solution: sawLow=%v sawHigh=%v (got %+v)", sawLow, sawHigh, got)
	}
}

// #9 重複列挙なし: 同一 filler 多重集合が 2 回以上返らない。
func TestWindowNoDuplicateEnumeration(t *testing.T) {
	target := Target{Name: "T", Startup: 5, Active: 6}
	moves := []Move{{Name: "a", TotalFrames: 6}, {Name: "b", TotalFrames: 9}, {Name: "c", TotalFrames: 6}}
	got := mustSuggest(t, 40, moves, target, DefaultOptions())
	seen := map[string]bool{}
	for _, s := range got {
		key := ""
		for _, f := range s.Fillers {
			key += f.Name + "|"
		}
		if seen[key] {
			t.Errorf("duplicate multiset enumerated: %v", fillerNames(s))
		}
		seen[key] = true
	}
}

// #10 現行互換(帯幅0への退化): NMin=1 かつ Active=1 のとき band=0 となり、
// 受理は remaining==0(= S==KA+1, N==1)の完全一致に退化する。
// ※ 補足: 移植元の "S==KA" は起き上がり=KA(誤解釈)前提だった。訂正後の意味論
// (起き上がり=KA+1)では、単一 active(Active=1)が起き上がりに載る唯一の解は S=KA+1
// (N=1)。本テストは「帯幅0で完全一致エンジンに退化する」性質を検証する(§5.1 #10)。
func TestWindowCollapsesToExactMatch(t *testing.T) {
	ka := 40
	target := Target{Name: "T", Startup: 10, Active: 1}
	opt := DefaultOptions() // NMin=1
	// budgetMax = 40+2−1−10 = 31, band = 0 → sumF は 31 ちょうど。
	// filler 31 → remaining=0 → N=1, S=KA+1=41。31 以外(30/32)は帯外。
	moves := []Move{{Name: "exact", TotalFrames: 31}, {Name: "under", TotalFrames: 30}, {Name: "over", TotalFrames: 32}}
	got := mustSuggest(t, ka, moves, target, opt)
	if want := [][]string{{"exact"}}; !reflect.DeepEqual(suggestionShapes(got), want) {
		t.Fatalf("collapse-to-exact shapes = %v, want %v", suggestionShapes(got), want)
	}
	for _, s := range got {
		if s.HitActiveFrame != 1 {
			t.Errorf("band=0 must yield N==1, got %d", s.HitActiveFrame)
		}
		if s.TotalFrames != ka+1 {
			t.Errorf("band=0 must yield S==KA+1(%d), got %d", ka+1, s.TotalFrames)
		}
	}
}

// --- 受理帯一般化(Band 機構)の engine テスト(mode 非依存の 11 観点) ---
//
// engine は受理帯 [nLo, nHi] を Options.Band で差し替えられる。ここでは band 機構
// そのもの(負の nLo を含む任意の帯で正しく列挙・枝刈りされること)を検証する。
// 実際のモードは service 層が band を構築して渡す(meaty=[nMin, Active]、
// 汚連携 gap=[active+Gmin, active+Gmax])。**モード固有のセマンティクス
// (gap の G=N−active・完全空振り等)の検証は service_test / refine_test 側**。
//
// 共通シナリオ: KA=40, target startup=20 active=4。band=[-12, 0](幅12。負 nLo を
// 意図的に含め、engine が負の下限でも健全なことを示す)。
//
//	budgetMax = 40+2−(−12)−20 = 34。受理 sumF∈[22,34]、N=remaining+nLo∈[−12,0]。
//
// filler total 集合 {11,17,22,34} を用いる。
func bandTarget() Target { return Target{Name: "tgt", Startup: 20, Active: 4} }

func bandMoves() []Move {
	return []Move{{Name: "m11", TotalFrames: 11}, {Name: "m17", TotalFrames: 17}, {Name: "m22", TotalFrames: 22}, {Name: "m34", TotalFrames: 34}}
}

func bandOpts(lo, hi int) Options {
	o := DefaultOptions()
	o.Band = &NBand{Lo: lo, Hi: hi}
	return o
}

// #1 meaty 現行互換: Band=nil と Band{NMin,Active} が同一解集合。
func TestBand_MeatyEquivalence(t *testing.T) {
	ka := 40
	target := Target{Name: "tgt", Startup: 20, Active: 5}
	moves := bandMoves()
	base := DefaultOptions()
	base.NMin = 2
	withNil := mustSuggest(t, ka, moves, target, base)
	explicit := base
	explicit.Band = &NBand{Lo: 2, Hi: 5} // NMin=2, Active=5
	withBand := mustSuggest(t, ka, moves, target, explicit)
	if !reflect.DeepEqual(suggestionShapes(withNil), suggestionShapes(withBand)) {
		t.Errorf("Band{NMin,Active} must equal nil(meaty): nil=%v band=%v", suggestionShapes(withNil), suggestionShapes(withBand))
	}
}

// #2 一般化後の恒等式: 全解で N==KA+2−(Σfiller+Startup) かつ N==remaining+nLo。
func TestBand_Identity(t *testing.T) {
	ka, nLo := 40, -12
	got := mustSuggest(t, ka, bandMoves(), bandTarget(), bandOpts(nLo, 0))
	if len(got) == 0 {
		t.Fatal("expected solutions")
	}
	for _, s := range got {
		sumF := 0
		for _, f := range s.Fillers {
			sumF += f.TotalFrames
		}
		wantN := ka + 2 - (sumF + bandTarget().Startup)
		if s.HitActiveFrame != wantN {
			t.Errorf("N=%d, want KA+2-S=%d (S=%d)", s.HitActiveFrame, wantN, sumF+bandTarget().Startup)
		}
		remaining := (ka + 2 - nLo - bandTarget().Startup) - sumF
		if s.HitActiveFrame != remaining+nLo {
			t.Errorf("N=%d, want remaining+nLo=%d", s.HitActiveFrame, remaining+nLo)
		}
	}
}

// #3 帯の下限で成立: 入力 band の nLo で解が返る。
func TestBand_LowerBound(t *testing.T) {
	nLo := -12
	got := mustSuggest(t, 40, bandMoves(), bandTarget(), bandOpts(nLo, 0))
	var found bool
	for _, s := range got {
		if s.HitActiveFrame == nLo { // sumF=34 の解({34} or {17,17})
			found = true
		}
	}
	if !found {
		t.Errorf("expected a solution at N==nLo(%d)", nLo)
	}
}

// #4 帯の上限で成立: 入力 band の nHi で解が返る。
func TestBand_UpperBound(t *testing.T) {
	nHi := 0
	got := mustSuggest(t, 40, bandMoves(), bandTarget(), bandOpts(-12, nHi))
	var found bool
	for _, s := range got {
		if s.HitActiveFrame == nHi { // sumF=22 の解({22} or {11,11})
			found = true
		}
	}
	if !found {
		t.Errorf("expected a solution at N==nHi(%d)", nHi)
	}
}

// #5 帯の外は解が返らない(全解の N が [nLo,nHi] 内)。
func TestBand_NoOutOfBand(t *testing.T) {
	nLo, nHi := -12, 0
	got := mustSuggest(t, 40, bandMoves(), bandTarget(), bandOpts(nLo, nHi))
	for _, s := range got {
		if s.HitActiveFrame < nLo || s.HitActiveFrame > nHi {
			t.Errorf("N=%d out of band [%d,%d] must not be returned", s.HitActiveFrame, nLo, nHi)
		}
	}
}

// #6 別の帯(内側にシフト)でも下限・上限とも成立し、帯の外に漏れない。
func TestBand_ShiftedBandWithinBounds(t *testing.T) {
	nLo, nHi := -7, -1 // [-7,-1]: sumF∈[27,33] → {11,22}=33(N=-7)/{11,11,11}=33/… を含む
	got := mustSuggest(t, 40, bandMoves(), bandTarget(), bandOpts(nLo, nHi))
	if len(got) == 0 {
		t.Fatal("expected solutions for shifted band")
	}
	for _, s := range got {
		if s.HitActiveFrame < nLo || s.HitActiveFrame > nHi {
			t.Errorf("shifted band leaked N=%d outside [%d,%d]", s.HitActiveFrame, nLo, nHi)
		}
	}
}

// #7 nLo>nHi はエラーではなく空結果。
func TestBand_LoGreaterThanHi(t *testing.T) {
	got := mustSuggest(t, 40, bandMoves(), bandTarget(), bandOpts(0, -5))
	if len(got) != 0 {
		t.Errorf("nLo>nHi must yield empty, got %d", len(got))
	}
}

// #8 Active<1 は band 指定でも ErrInvalidTargetActive。
func TestBand_InvalidActive(t *testing.T) {
	tgt := Target{Name: "tgt", Startup: 20, Active: 0}
	_, err := Suggest(40, bandMoves(), tgt, bandOpts(-12, 0))
	if !errors.Is(err, ErrInvalidTargetActive) {
		t.Errorf("Active<1 must return ErrInvalidTargetActive, got %v", err)
	}
}

// #9 枝刈り健全性: nLo が負で budgetMax が大きい入力でも、帯の下限側・上限側の両方が列挙される。
func TestBand_PruneEnumeratesBothEnds(t *testing.T) {
	got := mustSuggest(t, 40, bandMoves(), bandTarget(), bandOpts(-12, 0))
	var sawLo, sawHi bool
	for _, s := range got {
		if s.HitActiveFrame == -12 { // 下限
			sawLo = true
		}
		if s.HitActiveFrame == 0 { // 上限
			sawHi = true
		}
	}
	if !sawLo || !sawHi {
		t.Errorf("pruning dropped a band end: sawLo=%v sawHi=%v", sawLo, sawHi)
	}
}

// #10 重複列挙なし: 同一 filler 多重集合が 2 回以上返らない。
func TestBand_NoDuplicateEnumeration(t *testing.T) {
	got := mustSuggest(t, 40, bandMoves(), bandTarget(), bandOpts(-12, 0))
	seen := map[string]bool{}
	for _, sh := range suggestionShapes(got) {
		key := ""
		for _, n := range sh {
			key += n + ">"
		}
		if seen[key] {
			t.Errorf("duplicate filler multiset enumerated: %v", sh)
		}
		seen[key] = true
	}
}

// #11 budgetMax<0 は空結果(エラーにしない)。
func TestBand_NegativeBudget(t *testing.T) {
	// startup を大きくし budgetMax = KA+2−nLo−startup < 0 にする。
	tgt := Target{Name: "tgt", Startup: 100, Active: 4}
	got := mustSuggest(t, 5, bandMoves(), tgt, bandOpts(-12, 0))
	if len(got) != 0 {
		t.Errorf("budgetMax<0 must yield empty, got %d", len(got))
	}
}
