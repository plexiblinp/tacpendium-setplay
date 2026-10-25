package setplay

import (
	"context"
	"errors"
	"testing"
)

// SuggestForCombo の、出力の golden(suggest_for_combo_golden_test.go)では通らない分岐を固定する。
// 取り消し(要求の ctx)を見る 4 か所と、エンジンが取り消し以外の誤りを返したときの読み飛ばしである。
// どれも今の振る舞い(誤りの文言 ・エンジンを呼んだ回数)を期待値にしている。

// 繋ぎの単位を作る途中で取り消すと、単位の構築の誤りとして返し、エンジンを呼ばない。
func TestSuggestForCombo_CanceledWhileBuildingFillerUnits(t *testing.T) {
	engineCalls := stubEngineCounter(t)
	svc := newServiceWithDerivations(newCombo(ptr(heavyKA)), heavyMoves(), nil, nil)

	// 1 回目の確認で取り消す。heavyKA では単位の構築の繰り返しが確認の間隔を超えるため、
	// 最初の確認は単位の構築の中で起きる。
	res, err := svc.SuggestForCombo(newCountdownCtx(1), 1, SuggestParams{TargetTypes: allTypes()})
	if !errors.Is(err, context.Canceled) || res != nil {
		t.Fatalf("err=%v res=%v, want context.Canceled と nil", err, res)
	}
	if got, want := err.Error(), "build filler units: context canceled"; got != want {
		t.Errorf("err = %q, want %q", got, want)
	}
	if *engineCalls != 0 {
		t.Errorf("取り消した後にエンジンを %d 回呼んだ", *engineCalls)
	}
}

// 当てる技 1 つごとの確認で取り消すと、エンジンを呼ぶ前に止まる。
func TestSuggestForCombo_CanceledBeforeFirstTarget(t *testing.T) {
	engineCalls := stubEngineCounter(t)
	moves, derivations := goldenMoves()
	svc := newServiceWithDerivations(newCombo(ptr(40)), moves, derivations, nil)

	// 小さい fixture では単位の構築は確認の間隔に届かない。⇒ 1 回目の確認は当てる技のループの先頭である。
	res, err := svc.SuggestForCombo(newCountdownCtx(1), 1, SuggestParams{TargetTypes: allTypes()})
	if !errors.Is(err, context.Canceled) || res != nil {
		t.Fatalf("err=%v res=%v, want context.Canceled と nil", err, res)
	}
	if got, want := err.Error(), "suggest: context canceled"; got != want {
		t.Errorf("err = %q, want %q", got, want)
	}
	if *engineCalls != 0 {
		t.Errorf("取り消した後にエンジンを %d 回呼んだ", *engineCalls)
	}
}

// 1 つの当てる技の提案を行にするループの中(確認の間隔ごと)で取り消すと止まる。
func TestSuggestForCombo_CanceledWhileBuildingProposals(t *testing.T) {
	orig := runEngine
	t.Cleanup(func() { runEngine = orig })
	calls := 0
	runEngine = func(ctx context.Context, ka int, moves []Move, target Target, opt Options) ([]Suggestion, error) {
		calls++
		// 確認の間隔を超える数の提案を返す(中身は繋ぎ無しの当て 1 つ)。
		out := make([]Suggestion, cancelCheckInterval+10)
		for i := range out {
			out[i] = Suggestion{Target: target, TotalFrames: target.Startup, HitActiveFrame: 1}
		}
		return out, nil
	}
	moves, derivations := goldenMoves()
	svc := newServiceWithDerivations(newCombo(ptr(40)), moves, derivations, nil)

	// 1 回目(当てる技のループの先頭)は通し、2 回目(行を作るループの中)で取り消す。
	ctx := newCountdownCtx(2)
	res, err := svc.SuggestForCombo(ctx, 1, SuggestParams{TargetTypes: allTypes()})
	if !errors.Is(err, context.Canceled) || res != nil {
		t.Fatalf("err=%v res=%v, want context.Canceled と nil", err, res)
	}
	if got, want := err.Error(), "suggest: context canceled"; got != want {
		t.Errorf("err = %q, want %q", got, want)
	}
	if calls != 1 {
		t.Errorf("エンジンを %d 回呼んだ(1 つ目の当てる技の途中で止まるはず)", calls)
	}
	if got := ctx.calls.Load(); got != 2 {
		t.Errorf("確認 %d 回で止まった(2 回目で止まるはず)", got)
	}
}

// 当てる技が 1 つも無いときは、ループの後の確認で取り消しを返す。
func TestSuggestForCombo_CanceledAfterTargetLoop(t *testing.T) {
	engineCalls := stubEngineCounter(t)
	moves, derivations := goldenMoves()
	svc := newServiceWithDerivations(newCombo(ptr(40)), moves, derivations, nil)

	res, err := svc.SuggestForCombo(newCountdownCtx(1), 1, SuggestParams{}) // 種別なし＝当てる技 0 件
	if !errors.Is(err, context.Canceled) || res != nil {
		t.Fatalf("err=%v res=%v, want context.Canceled と nil", err, res)
	}
	if got, want := err.Error(), "suggest: context canceled"; got != want {
		t.Errorf("err = %q, want %q", got, want)
	}
	if *engineCalls != 0 {
		t.Errorf("当てる技が無いのにエンジンを %d 回呼んだ", *engineCalls)
	}
}

// エンジンが取り消し以外の誤りを返した当てる技は読み飛ばし、他の当てる技の提案は返す。
// (当てる技は事前に絞ってあるため本番では通常到達しない防御の分岐である。)
func TestSuggestForCombo_EngineErrorSkipsTarget(t *testing.T) {
	orig := runEngine
	t.Cleanup(func() { runEngine = orig })
	const skipped = "hadoken_light"
	calls := 0
	runEngine = func(ctx context.Context, ka int, moves []Move, target Target, opt Options) ([]Suggestion, error) {
		calls++
		if target.Name == skipped {
			return nil, errors.New("engine: invalid target")
		}
		return orig(ctx, ka, moves, target, opt)
	}
	moves, derivations := goldenMoves()
	svc := newServiceWithDerivations(newCombo(ptr(40)), moves, derivations, nil)
	params := SuggestParams{TargetTypes: []string{TargetTypeNormal, TargetTypeUnique, TargetTypeSpecialProjectile}}

	res, err := svc.SuggestForCombo(context.Background(), 1, params)
	if err != nil {
		t.Fatalf("エンジンの誤り(取り消し以外)を返した: %v", err)
	}
	if calls < 2 {
		t.Fatalf("エンジンの呼び出しが %d 回(読み飛ばした後も続くはず)", calls)
	}
	if len(res.Proposals) == 0 {
		t.Fatal("他の当てる技の提案まで消えた")
	}
	for _, p := range res.Proposals {
		if p.targetCode == skipped {
			t.Errorf("誤りを返した当てる技の行が残った: %s", p.sortName)
		}
	}
	if res.TotalFound != len(res.Proposals) || res.Truncated {
		t.Errorf("totalFound=%d proposals=%d truncated=%v", res.TotalFound, len(res.Proposals), res.Truncated)
	}
}

// stubEngineCounter はエンジンを、呼んだ回数を数えてから本物へ渡すものに差し替える。
func stubEngineCounter(t *testing.T) *int {
	t.Helper()
	orig := runEngine
	t.Cleanup(func() { runEngine = orig })
	calls := 0
	runEngine = func(ctx context.Context, ka int, moves []Move, target Target, opt Options) ([]Suggestion, error) {
		calls++
		return orig(ctx, ka, moves, target, opt)
	}
	return &calls
}
