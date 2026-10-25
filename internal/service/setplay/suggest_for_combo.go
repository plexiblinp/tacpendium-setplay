package setplay

import (
	"context"
	"errors"
	"fmt"

	setplayrepo "github.com/plexiblinp/tacpendium-setplay/internal/repository/setplay"
)

// SuggestForKnockdown は次の順で提案を作る。前提を確かめる(早期に 0 件や誤りで返す)→
// 候補 ・派生を読んで材料を作る → 当てる技ごとにエンジンを回して行を作る →
// 仕上げ(隣り合い ・重複 ・絞り込み)→ 並べ替え → 票 → 総数を数える → 件数で切る → 返す。
// 繋ぎだけモードは材料を作った後で別の経路(suggestFillerOnly)へ分かれる。
//
// 単体版: 本家の SuggestForCombo(comboID)に当たる。コンボを読まず、キャラの code と KA を入力で受ける
// (HANDOVER §2.2)。本家の findComboForSuggest ・markAlreadyAdopted(採用済みの印)は持たない。
func (s *service) SuggestForKnockdown(ctx context.Context, characterCode string, ka int, params SuggestParams) (*SuggestResult, error) {
	landing := ka + 1

	mode, early, err := checkSuggestPreconditions(ka, params)
	if early != nil || err != nil {
		return early, err
	}

	mat, err := s.loadSuggestMaterial(ctx, characterCode, ka)
	if err != nil {
		return nil, err
	}

	if mode == ModeFiller {
		return s.suggestFillerOnly(ctx, ka, params, mat.fillers, mat.unitByName, mat.parentOf, mat.marks, mat.unitsTruncated)
	}

	proposals, truncated, err := suggestTargets(ctx, ka, landing, mode, params, mat)
	if err != nil {
		return nil, err
	}

	// M47-05: 連打キャンセルになる隣り合い ・同じレシピの重複 ・3 つの絞り込みを、並べ替えと totalFound の前に掛ける
	//   (totalFound を「絞った後の総数」にし、画面の件数表示と「さらに表示」を食い違わせないため)。
	proposals = refineProposals(proposals, params, mode)

	// M19-LIMITATION-NOTICE (g): 打ち切りはここ(ランキング後)で行う。エンジンの安全上限
	//   より前ではない。この扱いを変更したら、ヒント文面を同時に更新すること。
	rank(proposals, params.Sort, mode == ModeGap)
	applyVotes(proposals, params.Votes) // 単体版(S6): 票を第一キーに。limit で切る前に掛ける
	totalFound := len(proposals)

	proposals = limitProposals(proposals, params.Limit)

	return &SuggestResult{Proposals: proposals, Truncated: truncated, TotalFound: totalFound}, nil
}

// checkSuggestPreconditions は候補を読む前に決まる前提を確かめ、モード(未指定は重ねる)を返す。
// 提案 0 件を理由つきで返す場合は early を、誤りで返す場合は err を返す。どちらも nil なら続ける。
func checkSuggestPreconditions(ka int, params SuggestParams) (mode string, early *SuggestResult, err error) {
	// M19-LIMITATION-NOTICE (e): 負 KA(有利フレームがマイナス)のコンボは提案不可。
	//   NULL(400)とは別で、保存が許される正当値(VAL-C10)のため 200 + 専用理由コードで返す。
	//   この扱いを変更したら、ヒント文面(web i18n `setplay.limitations.*`)を同時に更新すること。
	if ka < 0 {
		return "", &SuggestResult{Proposals: []Proposal{}, Reason: ReasonKnockdownNegative}, nil
	}

	mode = params.Mode
	if mode == "" {
		mode = ModeMeaty
	}
	if mode == ModeFiller {
		if params.Remain < 0 {
			return "", nil, ErrInvalidRemain
		}
		// Remain ≥ KA は繋ぎを 1 つも置けない(= KA は繋ぎ 0 個の行だけ、> KA は予算が負)。
		if params.Remain >= ka {
			return "", &SuggestResult{Proposals: []Proposal{}, Reason: ReasonRemainNotBelowKnockdown}, nil
		}
	}
	return mode, nil, nil
}

// suggestMaterial は、キャラの候補から作る提案の材料である(どのモードでも同じものを使う)。
type suggestMaterial struct {
	cands          []setplayrepo.MoveCandidate
	parentsByChild map[int64][]int64 // 子の技 → 親の技(表記用親 ・単独で出せない技の判定に使う)
	marks          proposalMarks
	parentOf       parentResolver
	// probe は要求の取り消しを見る部品である。繋ぎの単位の構築と当てる技のループで同じものを使う。
	probe          *cancelProbe
	fillers        []Move                // エンジンへ渡す繋ぎ(単位ごとに 1 つ)
	unitByName     map[string]fillerUnit // 繋ぎの名前 → 単位(行へ展開するときに引く)
	unitsTruncated bool                  // 繋ぎの単位の構築で上限に達した
}

// loadSuggestMaterial はキャラの候補 ・派生を読み、提案の材料を作る。
// 読む順は 候補 → 派生であり、途中で誤りが出たらそこで返す。
//
// 単体版: キャラは数値 ID ではなく code で指す。本家の CharacterCode(ID → code)の読み出しは要らない。
// 繋ぎの単位を作る途中で要求が取り消されたら、取り消しの誤りを返す。
func (s *service) loadSuggestMaterial(ctx context.Context, characterCode string, ka int) (*suggestMaterial, error) {
	cands, err := s.moveReader.ListCharacterMoveCandidates(ctx, characterCode)
	if err != nil {
		return nil, fmt.Errorf("list move candidates: %w", err)
	}

	derivations, err := s.moveReader.ListCharacterMoveDerivations(ctx, characterCode)
	if err != nil {
		return nil, fmt.Errorf("list move derivations: %w", err)
	}
	parentsByChild := make(map[int64][]int64, len(derivations))
	for _, d := range derivations {
		parentsByChild[d.ChildMoveID] = append(parentsByChild[d.ChildMoveID], d.ParentMoveID)
	}

	byID := make(map[int64]setplayrepo.MoveCandidate, len(cands))
	for _, m := range cands {
		byID[m.ID] = m
	}

	// M47-05: 接近の判定に使うキャラ固有の接近技(辞書はキャラの code で引く)。
	marks := proposalMarks{approach: approachCodesFor(characterCode)}
	// 表記用親の解決器。through の技の直前に前置する親を決める(費用規則 分類 1 = 計上ゼロ)。
	parentOf := newParentResolver(byID, parentsByChild)

	singles := collectFillerSingles(cands, parentsByChild)
	// 要求の取り消しを、以降の計算の繰り返しの内側で見る(M47-03・A2)。
	probe := newCancelProbe(ctx)
	units, unitsTruncated := buildFillerUnits(singles, cands, parentsByChild, ka+2, probe)
	if probe.err != nil {
		return nil, fmt.Errorf("build filler units: %w", probe.err)
	}
	unitByName := make(map[string]fillerUnit, len(units))
	fillers := make([]Move, 0, len(units))
	for _, u := range units {
		unitByName[u.Name] = u
		fillers = append(fillers, Move{Name: u.Name, TotalFrames: u.Total})
	}

	return &suggestMaterial{
		cands:          cands,
		parentsByChild: parentsByChild,
		marks:          marks,
		parentOf:       parentOf,
		probe:          probe,
		fillers:        fillers,
		unitByName:     unitByName,
		unitsTruncated: unitsTruncated,
	}, nil
}

// suggestTargets は重ねる ・汚連携のモードで、当てる技ごとにエンジンを回して行を作る。
// 返すのは仕上げ ・並べ替えの前の行(当てる技の列挙順 → エンジンの返した順)と、数え切れていないか。
// 要求が取り消されたら、そこで止めて取り消しの誤りを返す。
func suggestTargets(ctx context.Context, ka, landing int, mode string, params SuggestParams, mat *suggestMaterial) ([]Proposal, bool, error) {
	targets := collectTargets(mat.cands, mat.parentsByChild, params)

	opt := DefaultOptions()
	if params.NMin != 0 {
		opt.NMin = params.NMin // NMin < 1 はエンジンが 1 へ丸める。gap では帯に用いず無視される。
	}
	// エンジンは安全上限まで全列挙する(打ち切りはランキング後にグルーで行う=§4.3.2)。
	opt.MaxResults = engineSafetyCap

	// 安全上限に到達した(=数え切れていない)ときのみ truncated=true。
	// 繋ぎの単位の構築で上限(maxChainUnits / maxChainLen)に達したときも同じ(M47-03)。
	truncated := mat.unitsTruncated
	probe := mat.probe
	proposals := []Proposal{}
	for _, tc := range targets {
		if probe.check() {
			return nil, false, fmt.Errorf("suggest: %w", probe.err)
		}
		target := Target{Name: tc.Code, Startup: *tc.Startup, Active: *tc.Active}
		optT := targetOptions(opt, mode, params, target.Active)
		suggestions, serr := runEngine(ctx, ka, mat.fillers, target, optT)
		if serr != nil {
			if errors.Is(serr, context.Canceled) || errors.Is(serr, context.DeadlineExceeded) {
				return nil, false, fmt.Errorf("suggest: %w", serr)
			}
			// target は事前フィルタ済みのため通常到達しないが、防御的にスキップ。
			continue
		}
		if opt.MaxResults > 0 && len(suggestions) >= opt.MaxResults {
			truncated = true
		}
		for _, sg := range suggestions {
			if probe.tick() {
				return nil, false, fmt.Errorf("suggest: %w", probe.err)
			}
			proposals = append(proposals, buildProposal(landing, mode, tc, sg, mat.unitByName, mat.parentOf, mat.marks))
		}
	}
	if probe.check() {
		return nil, false, fmt.Errorf("suggest: %w", probe.err)
	}
	return proposals, truncated, nil
}

// targetOptions は 1 つの当てる技に渡すエンジンの設定を返す。opt を写し、受理帯だけをモードに応じて差し替える。
func targetOptions(opt Options, mode string, params SuggestParams, active int) Options {
	// M19-LIMITATION-NOTICE (f): 受理帯の差し替えだけでモード/上限を実現する
	//   (探索構造・恒等式は不変)。この扱いを変更したら、ヒント文面を同時に更新すること。
	optT := opt
	if mode == ModeGap {
		// M19-02-GAP-DEF: gap は N を [active+Gmin, active+Gmax] へ(完全空振り)。ロールバック点。
		optT.Band = gapBand(params.GMin, params.GMax, active)
	} else {
		optT.Band = meatyBand(opt.NMin, params.NMax, active) // meaty は n_max 指定時のみ上限を絞る。
	}
	return optT
}

// limitProposals は並べ替えた後の行を返却件数の上限で切る。
// limit が 0 以下なら既定の件数、上限を超えたら上限に丸める。切っても打ち切り(truncated)にはしない。
func limitProposals(proposals []Proposal, limit int) []Proposal {
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit // 超過指定は丸める(エラーにしない=n_min の流儀)。
	}
	if len(proposals) > limit {
		proposals = proposals[:limit] // limit 切りは truncated にしない(§4.3.2-3)。
	}
	return proposals
}
