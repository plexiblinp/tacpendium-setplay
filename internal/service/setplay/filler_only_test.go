package setplay

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"testing"

	"github.com/plexiblinp/tacpendium-setplay/internal/model"
	setplayrepo "github.com/plexiblinp/tacpendium-setplay/internal/repository/setplay"
)

// M47-04: 繋ぎだけモード(ModeFiller)のテスト。
//
// 見本の繋ぎ＝全体 13F / 18F / 26F の 3 つ(＋当てる技にしかならない技 1 つ)。
// 繋ぎの全体フレームの合計＝KA − Remain の組だけが返ることを、独立に数えた期待値
// (fillerOnlyOracle)と突き合わせる。★当たる組(Σ が一致)と当たらない組(Σ が 1 ずれる)を
// 両方見本に入れる(playbook §4.75)。

func fillerOnlyMoves() []setplayrepo.MoveCandidate {
	return []setplayrepo.MoveCandidate{
		{ID: 20, Code: "standing_light_punch", Category: model.MoveCategoryNormal, Startup: ptr(4), Active: ptr(3), Total: ptr(13), Damage: ptr(300)},
		{ID: 21, Code: "standing_light_kick", Category: model.MoveCategoryNormal, Startup: ptr(5), Active: ptr(3), Total: ptr(18), Damage: ptr(300)},
		{ID: 22, Code: "throw_forward", Category: model.MoveCategoryThrow, Startup: ptr(5), Active: ptr(3), Total: ptr(26), Damage: ptr(1200)},
	}
}

// fillerOnlyOracle は totals の多重集合(繰り返し可・最大 8 個)で和が sum になる組を、
// 昇順の全体フレーム列として返す。エンジンとは独立に数えた期待値である。
func fillerOnlyOracle(totals []int, sum int) [][]int {
	sort.Ints(totals)
	var out [][]int
	var rec func(start, rem int, cur []int)
	rec = func(start, rem int, cur []int) {
		if rem == 0 && len(cur) > 0 {
			out = append(out, append([]int(nil), cur...))
			return
		}
		if len(cur) == 8 {
			return
		}
		for i := start; i < len(totals); i++ {
			if totals[i] > rem {
				break
			}
			rec(i, rem-totals[i], append(cur, totals[i]))
		}
	}
	rec(0, sum, nil)
	return out
}

func proposalTotals(t *testing.T, p Proposal, byID map[int64]setplayrepo.MoveCandidate) []int {
	t.Helper()
	var ts []int
	for _, st := range p.Steps {
		if !st.Counted {
			continue
		}
		ts = append(ts, *byID[st.MoveID].Total)
	}
	sort.Ints(ts)
	return ts
}

func key(ts []int) string { return fmt.Sprint(ts) }

func TestFillerOnly_SumEqualsKAMinusRemain(t *testing.T) {
	moves := fillerOnlyMoves()
	byID := map[int64]setplayrepo.MoveCandidate{}
	for _, m := range moves {
		byID[m.ID] = m
	}
	cases := []struct {
		name       string
		ka, remain int
		wantRows   int
	}{
		{"当たる組: 13+18=31", 40, 9, 1},
		{"当たる組: 26 ／ 13+13", 40, 14, 2},
		{"当たらない組: Σ=35 は作れない", 40, 5, 0},
		{"X=0: 13×3 ／ 13+26", 39, 0, 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc := newService(newCombo(ptr(c.ka)), moves, nil)
			res, err := svc.SuggestForCombo(context.Background(), 1, SuggestParams{Mode: ModeFiller, Remain: c.remain})
			if err != nil {
				t.Fatal(err)
			}
			want := fillerOnlyOracle([]int{13, 18, 26}, c.ka-c.remain)
			if len(want) != c.wantRows {
				t.Fatalf("oracle sanity: want %d rows, oracle gives %d", c.wantRows, len(want))
			}
			if len(res.Proposals) != len(want) || res.TotalFound != len(want) {
				t.Fatalf("rows: want %d, got %d (totalFound %d)", len(want), len(res.Proposals), res.TotalFound)
			}
			wantSet := map[string]bool{}
			for _, w := range want {
				wantSet[key(w)] = true
			}
			for _, p := range res.Proposals {
				ts := proposalTotals(t, p, byID)
				sum := 0
				for _, v := range ts {
					sum += v
				}
				if sum != c.ka-c.remain {
					t.Errorf("Σ繋ぎ = %d, want KA−X = %d (%v)", sum, c.ka-c.remain, ts)
				}
				if !wantSet[key(ts)] {
					t.Errorf("unexpected recipe %v", ts)
				}
				for _, st := range p.Steps {
					if st.Role == RoleTarget {
						t.Errorf("filler-only row must not carry a target step: %+v", p.Steps)
					}
				}
				if p.Mode != ModeFiller || p.Remain != c.remain || p.N != 0 || p.G != 0 || p.TargetActive != 0 {
					t.Errorf("row fields: mode=%q remain=%d n=%d g=%d targetActive=%d", p.Mode, p.Remain, p.N, p.G, p.TargetActive)
				}
				if p.Landing != c.ka+1 {
					t.Errorf("landing want %d, got %d", c.ka+1, p.Landing)
				}
			}
		})
	}
}

// Remain ≥ KA は繋ぎを置けないため 0 件＋理由。Remain = KA のときエンジンが返す「繋ぎ 0 個」の
// 行を出さないこと、Remain > KA で予算が負になっても落ちないことを見る。
func TestFillerOnly_RemainNotBelowKA(t *testing.T) {
	for _, remain := range []int{40, 41, 100} {
		svc := newService(newCombo(ptr(40)), fillerOnlyMoves(), nil)
		res, err := svc.SuggestForCombo(context.Background(), 1, SuggestParams{Mode: ModeFiller, Remain: remain})
		if err != nil {
			t.Fatalf("remain=%d: %v", remain, err)
		}
		if len(res.Proposals) != 0 || res.Reason != ReasonRemainNotBelowKnockdown {
			t.Errorf("remain=%d: want 0 rows + reason, got %d rows reason=%q", remain, len(res.Proposals), res.Reason)
		}
	}
}

func TestFillerOnly_NegativeRemainIsError(t *testing.T) {
	svc := newService(newCombo(ptr(40)), fillerOnlyMoves(), nil)
	_, err := svc.SuggestForCombo(context.Background(), 1, SuggestParams{Mode: ModeFiller, Remain: -1})
	if !errors.Is(err, ErrInvalidRemain) {
		t.Fatalf("want ErrInvalidRemain, got %v", err)
	}
}

// 負 KA は既存どおりの理由(繋ぎだけモードでも変えない)。
func TestFillerOnly_NegativeKAKeepsExistingReason(t *testing.T) {
	svc := newService(newCombo(ptr(-3)), fillerOnlyMoves(), nil)
	res, err := svc.SuggestForCombo(context.Background(), 1, SuggestParams{Mode: ModeFiller, Remain: 0})
	if err != nil {
		t.Fatal(err)
	}
	if res.Reason != ReasonKnockdownNegative {
		t.Errorf("want %q, got %q", ReasonKnockdownNegative, res.Reason)
	}
}

// 繋ぎだけモードは当てる技の条件(種別・特定の技)を見ない。TargetTypes が空でも行が出る。
func TestFillerOnly_IgnoresTargetParams(t *testing.T) {
	svc := newService(newCombo(ptr(40)), fillerOnlyMoves(), nil)
	id := int64(22)
	res, err := svc.SuggestForCombo(context.Background(), 1, SuggestParams{Mode: ModeFiller, Remain: 9, TargetMoveID: &id})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Proposals) != 1 {
		t.Fatalf("want 1 row, got %d", len(res.Proposals))
	}
}

// 並びは手数の少ない順(推測)→ 表記の昇順。
func TestFillerOnly_RankFewerStepsFirst(t *testing.T) {
	svc := newService(newCombo(ptr(39)), fillerOnlyMoves(), nil)
	res, err := svc.SuggestForCombo(context.Background(), 1, SuggestParams{Mode: ModeFiller, Remain: 0})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Proposals) != 2 {
		t.Fatalf("want 2 rows, got %d", len(res.Proposals))
	}
	if len(res.Proposals[0].Steps) != 2 || len(res.Proposals[1].Steps) != 3 {
		t.Errorf("want 2-step row first, got %d then %d", len(res.Proposals[0].Steps), len(res.Proposals[1].Steps))
	}
}

// 合成の繋ぎ単位(連打キャンセル)と through の表記用親は、既存の提案と同じ規則で展開する。
func TestFillerOnly_ExpandsChainUnitsAndParents(t *testing.T) {
	moves := []setplayrepo.MoveCandidate{
		// 連打キャンセル: 非末端は chain_cancel_total(9)、末端は total(14)。単位 = 9 + 14 = 23。
		{ID: 30, Code: "crouching_light_punch", Category: model.MoveCategoryNormal, Startup: ptr(4), Active: ptr(2), Total: ptr(14), ChainCancelTotal: ptr(9), Damage: ptr(300)},
		// through の技(親 30 から出す)。単発の全体 17。親は計上ゼロで前置される。
		{ID: 31, Code: "follow_up", Category: model.MoveCategoryUnique, Startup: ptr(10), Active: ptr(2), Total: ptr(17), Damage: ptr(500), IsDerived: true, StartupBasis: model.MoveStartupBasisThrough},
	}
	der := []setplayrepo.MoveDerivation{{ChildMoveID: 31, ParentMoveID: 30}}
	svc := newServiceWithDerivations(newCombo(ptr(40)), moves, der, nil)

	// Σ = 23 → 連打キャンセルの 1 単位(14 と 14 の単発 2 つ＝28 は一致しない)
	res, err := svc.SuggestForCombo(context.Background(), 1, SuggestParams{Mode: ModeFiller, Remain: 17})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, p := range res.Proposals {
		if p.sortName == "crouching_light_punch>crouching_light_punch" && p.sumFiller == 23 {
			found = true
		}
	}
	if !found {
		t.Errorf("chain unit (9+14=23) not found: %+v", res.Proposals)
	}

	// Σ = 17 → through の技。表記用親(計上ゼロ)を前置する。
	res, err = svc.SuggestForCombo(context.Background(), 1, SuggestParams{Mode: ModeFiller, Remain: 23})
	if err != nil {
		t.Fatal(err)
	}
	var through *Proposal
	for i := range res.Proposals {
		if res.Proposals[i].sortName == "crouching_light_punch>follow_up" {
			through = &res.Proposals[i]
		}
	}
	if through == nil {
		t.Fatalf("through row not found: %+v", res.Proposals)
	}
	if through.Steps[0].Role != RoleParent || through.Steps[0].Counted || through.Steps[1].Role != RoleFiller {
		t.Errorf("parent step must be prepended uncounted: %+v", through.Steps)
	}
}

// 既存の提案(重ねる・汚連携)は繋ぎだけモードを足しても変わらない。Mode を指定しない呼び出しは
// 当てる技を持つ行だけを返す(M47-04 の最重要ゲート (1) の補助。本体は既存テストが緑のまま)。
func TestFillerOnly_DoesNotLeakIntoMeatyOrGap(t *testing.T) {
	for _, mode := range []string{"", ModeMeaty, ModeGap} {
		svc := newService(newCombo(ptr(40)), gc1Moves(), nil)
		res, err := svc.SuggestForCombo(context.Background(), 1, SuggestParams{Mode: mode, TargetTypes: allTypes(), Remain: 9})
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range res.Proposals {
			last := p.Steps[len(p.Steps)-1]
			if last.Role != RoleTarget || p.Mode == ModeFiller || p.Remain != 0 {
				t.Errorf("mode=%q: row must end with target and carry no remain: %+v", mode, p)
			}
		}
	}
}
