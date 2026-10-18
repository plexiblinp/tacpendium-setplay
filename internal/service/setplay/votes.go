package setplay

import (
	"sort"
	"strings"
)

// 単体版だけ(S6・HANDOVER D17 グッド/バッド)。本家に無い。
//
// 票は「キャラ・KA・レシピ」ごとに集め(開発者決定 2026-10-12)、スコアは good − bad。
// サービスは本家の並べ替え(rank / rankFillerOnly)の後に、スコアの降順で**安定に**並べ直す。
// だから票の無い行どうし・同点の行どうしは本家の順のまま、票が 1 つも無ければ並びは本家と同じ。
// limit はこの後で切る(AGENTS §4「並べ替えの後で切る」)。エンジンの安全上限(engineSafetyCap 等)は
// これより前に掛かるので、上限で落ちた行は票があっても出ない(docs/telemetry.md の限界)。

// recipeKeyStepSep ・recipeKeyTargetSep はレシピの鍵の区切り。技 code は英小文字 ・数字 ・_ だけなので混ざらない。
const (
	recipeKeyStepSep   = ","
	recipeKeyTargetSep = "|"
)

// RecipeKey はレシピの同一性の鍵を返す。「当てる技以外の手(繋ぎ ・表記用親)の技 code を順に , で連結」+ "|" +
// 「当てる技の code」(繋ぎだけモードは空)。KA ・N ・G は入れない(票は KA ごとに別に集める)。
// 技 code で作るので、データ版が変わっても同じ技の同じ手順なら同じ鍵になる。
func RecipeKey(steps []ProposalStep) string {
	var setup []string
	target := ""
	for _, st := range steps {
		if st.Role == RoleTarget {
			target = st.Code
			continue
		}
		setup = append(setup, st.Code)
	}
	return strings.Join(setup, recipeKeyStepSep) + recipeKeyTargetSep + target
}

// applyVotes は props をスコア(votes[RecipeKey])の降順で安定に並べ直す。votes が空なら何もしない。
func applyVotes(props []Proposal, votes map[string]int) {
	if len(votes) == 0 {
		return
	}
	type scored struct {
		p     Proposal
		score int
	}
	tmp := make([]scored, len(props))
	for i, p := range props {
		tmp[i] = scored{p: p, score: votes[RecipeKey(p.Steps)]}
	}
	sort.SliceStable(tmp, func(i, j int) bool { return tmp[i].score > tmp[j].score })
	for i := range tmp {
		props[i] = tmp[i].p
	}
}
