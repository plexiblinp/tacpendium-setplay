package setplay

import (
	"context"
	"errors"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/plexiblinp/tacpendium-setplay/internal/model"
	setplayrepo "github.com/plexiblinp/tacpendium-setplay/internal/repository/setplay"
)

// M47-03(A2): 要求が取り消されたら、計算の繰り返しの途中で止まる。
// 取り消さない要求の結果は今と同じ(playbook §4.75＝在る条件と無い条件の両側)。

// countdownCtx は Err() を呼ばれた回数を数え、after 回目以降だけ context.Canceled を返す。
// after ≤ 0 なら取り消さない(回数だけ数える)。計算の「途中」で取り消しを起こすために使う。
type countdownCtx struct {
	context.Context
	after int64
	calls atomic.Int64
}

func newCountdownCtx(after int64) *countdownCtx {
	return &countdownCtx{Context: context.Background(), after: after}
}

func (c *countdownCtx) Err() error {
	n := c.calls.Add(1)
	if c.after > 0 && n >= c.after {
		return context.Canceled
	}
	return nil
}

// heavyMoves は、KA を大きくすると探索が長くなる候補(チェーン員 3 行 ＋ 当てる技 1 行)。
func heavyMoves() []setplayrepo.MoveCandidate {
	return append(ryuChainMoves(),
		setplayrepo.MoveCandidate{ID: 9, Code: "tgt", Category: model.MoveCategoryUnique, Startup: ptr(20), Active: ptr(4), Total: ptr(42), Damage: ptr(600), StartupBasis: model.MoveStartupBasisStandalone})
}

// heavyKA は heavyMoves で ctx の確認が何十回も起きる大きさの KA。
const heavyKA = 150

func engineHeavyInput() ([]Move, Target, Options) {
	moves := []Move{{Name: "a", TotalFrames: 3}, {Name: "b", TotalFrames: 4}, {Name: "c", TotalFrames: 5}, {Name: "d", TotalFrames: 7}}
	opt := DefaultOptions()
	opt.MaxResults = 0 // 件数で止めない(取り消しだけで止まることを見る)
	return moves, Target{Name: "t", Startup: 5, Active: 3}, opt
}

func TestSuggestContext_PreCanceled_ReturnsCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	moves, target, opt := engineHeavyInput()
	res, err := SuggestContext(ctx, 40, moves, target, opt)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
	if res != nil {
		t.Errorf("取り消したときは結果を返さない: %d 件", len(res))
	}
}

// 探索の再帰の内側で取り消しを見ていることを固定する。
// 取り消さない走行の ctx 確認回数 T を数え、T の半分で取り消すと、そこで止まる。
func TestSuggestContext_StopsInsideSearchLoop(t *testing.T) {
	moves, target, opt := engineHeavyInput()
	const ka = 80

	full := newCountdownCtx(0)
	want, err := SuggestContext(full, ka, moves, target, opt)
	if err != nil {
		t.Fatal(err)
	}
	total := full.calls.Load()
	// 反復深化の各回の確認(深さの数)だけでなく、再帰の内側の確認が走っていること。
	depths := int64(computeMaxDepth(ka+2-1-target.Startup, sanitize(moves), opt.MaxFillers) + 1)
	if total <= depths {
		t.Fatalf("再帰の内側で ctx を見ていない(確認 %d 回 ≤ 深さの数 %d)", total, depths)
	}

	half := newCountdownCtx(total / 2)
	res, err := SuggestContext(half, ka, moves, target, opt)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("途中で取り消したのに止まらなかった: err=%v, 件数=%d(取り消さなければ %d)", err, len(res), len(want))
	}
	if got := half.calls.Load(); got != total/2 {
		t.Errorf("取り消しを観測した後も確認が続いた: %d 回(取り消しは %d 回目)", got, total/2)
	}
}

func TestSuggestContext_NotCanceled_SameAsSuggest(t *testing.T) {
	moves, target, opt := engineHeavyInput()
	opt.MaxResults = 5000
	want, err := Suggest(40, moves, target, opt)
	if err != nil {
		t.Fatal(err)
	}
	got, err := SuggestContext(newCountdownCtx(0), 40, moves, target, opt)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("取り消さない要求の結果が変わった: got %d 件, want %d 件", len(got), len(want))
	}
}

// サービスの 3 つの経路(meaty ／ gap ／ filler＝M47-04 の繋ぎだけ・安全飛び)のそれぞれで、
// 計算の途中で取り消すと context.Canceled で止まる。
func TestService_CancelMidway_AllPaths(t *testing.T) {
	cases := []struct {
		name   string
		params SuggestParams
	}{
		{"meaty", SuggestParams{Mode: ModeMeaty, TargetTypes: allTypes()}},
		{"gap", SuggestParams{Mode: ModeGap, TargetTypes: allTypes()}},
		{"filler", SuggestParams{Mode: ModeFiller, Remain: 4}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := newServiceWithDerivations(newCombo(ptr(heavyKA)), heavyMoves(), nil, nil)

			full := newCountdownCtx(0)
			if _, err := svc.SuggestForCombo(full, 1, tc.params); err != nil {
				t.Fatal(err)
			}
			total := full.calls.Load()
			if total < 4 {
				t.Fatalf("確認回数が少なすぎてテスト条件が弱い: %d", total)
			}

			half := newCountdownCtx(total / 2)
			res, err := svc.SuggestForCombo(half, 1, tc.params)
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("途中で取り消したのに止まらなかった: err=%v res=%v", err, res != nil)
			}
			if got := half.calls.Load(); got != total/2 {
				t.Errorf("取り消しを観測した後も計算が続いた: 確認 %d 回(取り消しは %d 回目)", got, total/2)
			}
		})
	}
}

// 取り消さない要求は、ctx の確認が入っても結果(行・順・件数・打ち切り)が同じ。
// ★KA は heavyKA ではなく 60 にしている(M47-05 追補④)。heavyMoves はチェーン員と当てる技だけであり、
// KA 150 ではエンジンの安全上限までに出る行がすべて「単発のチェーン員が隣り合う」行になって落ち、0 件になる。
// KA 60 なら合成単位 1 つ ＋ 当てる技の行が残る。
func TestService_NotCanceled_SameResult_AllPaths(t *testing.T) {
	const ka = 60
	for _, params := range []SuggestParams{
		{Mode: ModeMeaty, TargetTypes: allTypes()},
		{Mode: ModeGap, TargetTypes: allTypes()},
		{Mode: ModeFiller, Remain: 4},
	} {
		svc := newServiceWithDerivations(newCombo(ptr(ka)), heavyMoves(), nil, nil)
		want, err := svc.SuggestForCombo(context.Background(), 1, params)
		if err != nil {
			t.Fatal(err)
		}
		got, err := svc.SuggestForCombo(newCountdownCtx(0), 1, params)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("mode=%s: 取り消さない要求の結果が変わった", params.Mode)
		}
		if len(want.Proposals) == 0 {
			t.Errorf("mode=%s: 結果が 0 件でテスト条件が弱い", params.Mode)
		}
	}
}

// M47-03 ＝ D-1003: 合成の繋ぎ単位の上限(maxChainUnits)に達したら truncated を立てる。
// 達しないときは立たない。meaty と filler の両方で見る。
func TestService_ChainUnitCapSetsTruncated(t *testing.T) {
	const ka = 43 // budgetCap 45。既定の上限では単位は数個で、エンジンの安全上限にも届かない。
	for _, params := range []SuggestParams{
		{Mode: ModeMeaty, TargetTypes: allTypes()},
		{Mode: ModeFiller, Remain: 4},
	} {
		svc := newServiceWithDerivations(newCombo(ptr(ka)), heavyMoves(), nil, nil)
		res, err := svc.SuggestForCombo(context.Background(), 1, params)
		if err != nil {
			t.Fatal(err)
		}
		if res.Truncated {
			t.Errorf("mode=%s: 上限に達していないのに truncated", params.Mode)
		}

		func() {
			orig := maxChainUnits
			maxChainUnits = 4 // 単発 3 ＋ 合成 1 で到達する
			defer func() { maxChainUnits = orig }()
			res, err := svc.SuggestForCombo(context.Background(), 1, params)
			if err != nil {
				t.Fatal(err)
			}
			if !res.Truncated {
				t.Errorf("mode=%s: 合成の繋ぎ単位の上限に達したのに truncated が立たない", params.Mode)
			}
		}()
	}
}

// M47-03: 連鎖の長さの上限(maxChainLen)まで伸ばした後、もう 1 段伸ばせる単位が残っていたら
// truncated。残っていなければ立たない(境目の両側)。
func TestBuildFillerUnits_ChainLenCapTruncated(t *testing.T) {
	orig := maxChainLen
	maxChainLen = 2
	defer func() { maxChainLen = orig }()

	moves := ryuChainMoves() // 立弱P の chain_cancel_total 9 / total 13 が最小
	// 長さ 3 の最小＝9+9+13=31。budgetCap 31 なら 3 段目が置ける → truncated。
	if _, tr := buildFillerUnits(moves, moves, map[int64][]int64{}, 31, newCancelProbe(nil)); !tr {
		t.Error("長さの上限の先に置ける単位が残っているのに truncated が立たない")
	}
	// budgetCap 30 なら長さ 3 はどれも置けない(長さ 2 の 9+13=22 は置ける) → 立たない。
	if _, tr := buildFillerUnits(moves, moves, map[int64][]int64{}, 30, newCancelProbe(nil)); tr {
		t.Error("長さの上限の先に置ける単位が無いのに truncated が立った")
	}
}

func TestBuildFillerUnits_UnitCapTruncated(t *testing.T) {
	moves := ryuChainMoves()
	if _, tr := buildFillerUnits(moves, moves, map[int64][]int64{}, 45, newCancelProbe(nil)); tr {
		t.Error("上限に達していないのに truncated")
	}
	orig := maxChainUnits
	maxChainUnits = 4
	defer func() { maxChainUnits = orig }()
	if _, tr := buildFillerUnits(moves, moves, map[int64][]int64{}, 45, newCancelProbe(nil)); !tr {
		t.Error("単位の個数の上限に達したのに truncated が立たない")
	}
}

// 単位の構築の繰り返しの内側でも取り消しを見る(KA が極端なときの前段の重い経路)。
func TestBuildFillerUnits_StopsWhenCanceled(t *testing.T) {
	// 個数の上限を外し、長さの上限(8)まで幅優先で伸ばし切る重い条件にする。
	orig := maxChainUnits
	maxChainUnits = 1 << 30
	defer func() { maxChainUnits = orig }()
	moves := ryuChainMoves()
	full := newCountdownCtx(0)
	allUnits, _ := buildFillerUnits(moves, moves, map[int64][]int64{}, 100000, newCancelProbe(full))
	total := full.calls.Load()
	if total < 2 {
		t.Fatalf("単位の構築で ctx を見ていない、または条件が弱い: 確認 %d 回", total)
	}
	half := newCountdownCtx(total / 2)
	probe := newCancelProbe(half)
	units, _ := buildFillerUnits(moves, moves, map[int64][]int64{}, 100000, probe)
	if !errors.Is(probe.err, context.Canceled) {
		t.Fatalf("取り消しを観測していない: %v", probe.err)
	}
	if len(units) >= len(allUnits) {
		t.Errorf("取り消したのに最後まで作った: %d 個(取り消さなければ %d 個)", len(units), len(allUnits))
	}
}

// すべての経路(meaty ／ gap ／ filler)が、要求の ctx を計算の本体(エンジン)へ渡している。
// ★エンジンの中で取り消しを見ること(TestSuggestContext_StopsInsideSearchLoop)と対である——
//
//	経路が ctx を渡し忘れると、取り消してもエンジンは最後まで計算する。後段の確認で
//	context.Canceled は返るため、TestService_CancelMidway_AllPaths だけでは見逃す(M47-03 の破壊確認で実測)。
func TestService_AllPathsPassRequestCtxToEngine(t *testing.T) {
	type ctxKey struct{}
	for _, params := range []SuggestParams{
		{Mode: ModeMeaty, TargetTypes: allTypes()},
		{Mode: ModeGap, TargetTypes: allTypes()},
		{Mode: ModeFiller, Remain: 4},
	} {
		t.Run(params.Mode, func(t *testing.T) {
			orig := runEngine
			defer func() { runEngine = orig }()
			calls, withReqCtx := 0, 0
			runEngine = func(ctx context.Context, ka int, moves []Move, target Target, opt Options) ([]Suggestion, error) {
				calls++
				if ctx.Value(ctxKey{}) == "req" {
					withReqCtx++
				}
				return orig(ctx, ka, moves, target, opt)
			}
			reqCtx := context.WithValue(context.Background(), ctxKey{}, "req")
			svc := newServiceWithDerivations(newCombo(ptr(43)), heavyMoves(), nil, nil)
			if _, err := svc.SuggestForCombo(reqCtx, 1, params); err != nil {
				t.Fatal(err)
			}
			if calls == 0 {
				t.Fatal("エンジンが runEngine を通して呼ばれていない(取り消しを見ない経路が残る)")
			}
			if withReqCtx != calls {
				t.Errorf("要求の ctx をエンジンへ渡していない呼び出しがある: %d / %d", calls-withReqCtx, calls)
			}
		})
	}
}
