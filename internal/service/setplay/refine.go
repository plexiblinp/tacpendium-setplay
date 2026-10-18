package setplay

// refineProposals は、行を全部作った後・並べ替え(rank / rankFillerOnly)と totalFound の前に掛ける
// 仕上げである(M47-05・CHANGE-254 d ・e)。次の順で掛ける。
//
//  1. 連打キャンセルになる隣り合い(常に・全キャラ)——別々の区切りに置いたチェーン員どうしが隣り合う行は、
//     最速で押すと連打キャンセルになり、表記(単発ずつ)どおりのフレームにならない。隣り合わない順へ
//     並べ替える。どう並べても隣り合う行は落とす(adjustChainAdjacency)。
//     ★M47-05 追補④で全キャラへ広げた(M19-DESIGN-07 §9-3 の最速原則。開発者の実機検証＝ダルシム KA 58)。
//  2. 同じレシピの重複をまとめる(常に)——同じレシピ(recipeHash)が 2 行以上出たら、
//     費用(sumFiller)の小さい 1 行だけを残す(dedupeByRecipe)。費用が同じ 2 行(同じ表記・同じ S)もまとめる。
//  3. 3 つの絞り込み(params の true のものを AND)——接近 ／ ガード時 0F 以上(meaty だけ) ／ 連打キャンセル。
//
// ★totalFound を「絞った後の総数」にするため、必ず rank と totalFound の前で呼ぶこと。
// 画面の件数表示「全 {totalFound} 件中 {n} 件」と「さらに表示」(items < totalFound)が、
// これで絞り込みと食い違わない。
// ★切り替えがすべて false のとき、結果から消えるのは 1 と 2 の分だけである(指示書 §2.2 ・§7.1-2)。
//
// M19-LIMITATION-NOTICE (h): 1 と 2 の扱い(隣り合い・重複のまとめ)を変更したら、
// ヒント文面(web i18n `setplay.limitations.weakChain`)を同時に更新すること。逆も同様。
func refineProposals(props []Proposal, params SuggestParams, mode string) []Proposal {
	props = adjustChainAdjacency(props)
	props = dedupeByRecipe(props)
	if !params.ApproachOnly && !params.ChainCancelOnly && !(params.BlockSafeOnly && mode == ModeMeaty) {
		return props
	}
	out := props[:0]
	for _, p := range props {
		if params.ApproachOnly && !p.hasApproach {
			continue
		}
		if params.ChainCancelOnly && !p.hasChainCancel {
			continue
		}
		if params.BlockSafeOnly && mode == ModeMeaty && !blockSafe(p) {
			continue
		}
		out = append(out, p)
	}
	return out
}

// blockSafe は、持続当ての結果ガード時の有利フレームが 0F 以上か(J・開発者の式＝D-1005)。
//
//	on_block + N − 1 ≥ 0
//
// on_block は持続の 1F 目を当てたときの値であり、持続当てが 1 深くなれば 1 増える。
// N は起き上がりに重なる持続の位置(1 始まり)。★ちょうど 0 を含む。
// on_block が NULL の技(当てる技を持たない行を含む)は false(出さない＝D-1007)。
func blockSafe(p Proposal) bool {
	if p.targetOnBlock == nil {
		return false
	}
	return *p.targetOnBlock+p.N-1 >= 0
}

// dedupeByRecipe は同じレシピの重複をまとめる(M47-05・D-1003)。主な出どころは型 B の費用違いの重複だが、
// 費用が同じ 2 行もまとめ、型 A のキャラ・全モードにも掛ける(下の ★)。
//
// 型 B(連打版の行が無いキャラ)では、チェーングループ員が単発の繋ぎとしても残るため、
// 同じ表記(例 リュウ 立弱P > 立弱P > 当てる技)が、合成単位経由の費用(連打キャンセル・22F)と
// 単発 2 つ分の費用(26F)の 2 行で出る。表記どおりに入力すれば実機ではどちらか一方にしかならず、
// 最速で入力すれば連打キャンセルになる(最速原則＝M19-DESIGN-07 §9-3)。
// ⇒ 同じレシピ(recipeHash＝展開後のステップの move ID の並び。当てる技を含む)の行のうち、
// 費用(sumFiller)が最も小さい 1 行だけを残す。費用も同じなら先に在る行を残す(決定論)。
// ★費用が同じ 2 行も実在する——単位の切り方が違うだけで同じ費用になる組(例 [a>b]+[c] と [a]+[b>c])であり、
// 画面では表記も S も同じ行が 2 つ並ぶ。同じレシピなので同じ規則でまとめる(M47-05 の実測＝ryu KA 60 で 450 行)。
// 残る行の位置は、その組の最初の行の位置である(組の中の順だけが変わる)。
//
// ★モードに関係なく掛ける。繋ぎだけ(filler)では Σ繋ぎが KA − Remain で固定のため、
// 費用違いの 2 行は原理的に出ない。**ただし費用が同じ 2 行(単位の切り方違い)は出るので、それはまとまる**
// (実測は M47-05 完了報告の繋ぎだけモードの表)。
func dedupeByRecipe(props []Proposal) []Proposal {
	first := make(map[string]int, len(props)) // recipeHash → out での位置
	out := props[:0]
	for _, p := range props {
		if i, ok := first[p.recipeHash]; ok {
			if p.sumFiller < out[i].sumFiller {
				out[i] = p
			}
			continue
		}
		first[p.recipeHash] = len(out)
		out = append(out, p)
	}
	return out
}

// adjustChainAdjacency は「最速で押すと連打キャンセルになる」隣り合いを持つ提案を扱う
// (M47-05 ・指示書 §9.4-2 ／ 追補④・M19-DESIGN-07 §9-3 の最速原則)。
//
// 別々の区切り(繋ぎの単位 ・当てる技)に置いたチェーン員 P ・X が隣り合うと、実機で最速に入力すれば
// 連打キャンセルになる(開発者＝「硬直が解けるまで待ってから次を押す入力は人間には不可能」)。
// そのとき P の消費は全体フレームではなく連打キャンセルの値になり、X が連打版に化けることもある。
// ⇒ 表記どおり(単発ずつ)のフレームにならない。
// 実測(ダルシム KA 58・2026-09-28): 「屈弱P > 屈弱P > 屈強K」(単発 2 つ＝16+16F)は、
// 実機では持続が起き上がりの 5F 前に切れた(16 − 11 ＝ 5F 早い)。
//
// ★連打キャンセルした形は、合成の繋ぎ単位(buildFillerUnits)として別の行に在る(KA に収まれば)。
// ★合成単位の内側は 1 つの区切りなので対象外である(＝残る)。
// ★【追補②→④】追補②では「連打版に化ける組だけ」へ狭めたが、化けない組(例 ダルシム 屈弱P ＋ 屈弱P)も
// 連打キャンセルになるため誤りだった。追補④で「チェーン員どうし」へ戻し、全キャラへ広げた。
//
// 扱い(Plan の ■ 未決事項 A の推奨案):
//   - 今の並びで隣り合いが無い行は、そのまま残す。
//   - 隣り合う行は、繋ぎの単位の順を入れ替えて隣り合わない並びを探し、見つかればその並びで出す。
//     順を入れ替えても単位の中身 ・合計フレーム(S ・N ・G)は変わらない。★エンジンは同じ技の組を
//     1 つの並び(全体フレームの昇順)でしか出さないため、落とすと隣り合わない並びの提案まで消える。
//   - どう並べても隣り合う行(例 弱P ＋ 弱P ＋ 弱P)は落とす。
//
// 当てる技は常に最後に置く(動かさない)。並べ替えた結果が別の行と同じレシピになったら、
// 呼び出し側の dedupeByRecipe がまとめる。
func adjustChainAdjacency(props []Proposal) []Proposal {
	out := props[:0]
	for _, p := range props {
		if !hasChainAdjacency(p.parts, p.targetPart) {
			out = append(out, p)
			continue
		}
		order, ok := findNonAdjacentOrder(p.parts, p.targetPart)
		if !ok {
			continue // どう並べても隣り合う＝表記どおりに入力できない
		}
		reordered := make([]proposalPart, len(order))
		for i, idx := range order {
			reordered[i] = p.parts[idx]
		}
		p.parts = reordered
		p.assemble()
		out = append(out, p)
	}
	return out
}

// chainAdjacent は、区切り a の直後に区切り b を置いたとき、最速で押すと連打キャンセルになるか
// (a の最後の技と b の最初の技が、どちらもチェーングループ員)。
func chainAdjacent(a, b proposalPart) bool {
	return a.last.ChainCancelTotal != nil && b.first.ChainCancelTotal != nil
}

// hasChainAdjacency は、区切りの並びに「連打キャンセルになる」隣り合いが在るかを返す。
func hasChainAdjacency(parts []proposalPart, target *proposalPart) bool {
	for i := 0; i+1 < len(parts); i++ {
		if chainAdjacent(parts[i], parts[i+1]) {
			return true
		}
	}
	if target != nil && len(parts) > 0 && chainAdjacent(parts[len(parts)-1], *target) {
		return true
	}
	return false
}

// findNonAdjacentOrder は、連打キャンセルになる隣り合いの無い繋ぎの単位の並び(parts の添字列)を深さ優先で探す。
// 候補は元の順で試すため、見つかる並びは元の並びに近いもの(辞書順で最小)になり、決定論である。
// 繋ぎの数は MaxFillers(8)以下であり、隣り合う枝はその場で切るため、探索は小さい。
func findNonAdjacentOrder(parts []proposalPart, target *proposalPart) ([]int, bool) {
	n := len(parts)
	order := make([]int, 0, n)
	used := make([]bool, n)
	var dfs func() bool
	dfs = func() bool {
		if len(order) == n {
			if target != nil && n > 0 && chainAdjacent(parts[order[n-1]], *target) {
				return false
			}
			return true
		}
		for i := 0; i < n; i++ {
			if used[i] {
				continue
			}
			if len(order) > 0 && chainAdjacent(parts[order[len(order)-1]], parts[i]) {
				continue
			}
			used[i] = true
			order = append(order, i)
			if dfs() {
				return true
			}
			order = order[:len(order)-1]
			used[i] = false
		}
		return false
	}
	if !dfs() {
		return nil, false
	}
	return order, true
}
