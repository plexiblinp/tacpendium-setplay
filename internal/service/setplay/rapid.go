package setplay

import (
	"strings"

	setplayrepo "github.com/plexiblinp/tacpendium-setplay/internal/repository/setplay"
)

// rapidSuffix は連打版の行の code の末尾である(例 crouching_light_punch_rapid)。
// 連打版 R とその元技 X の対応はデータに明示の列が無いため、この命名で結ぶ(M47-05 追補②)。
// ★命名が崩れると黙って効かなくなるため、internal/infra/migration の
// TestCanary_RapidVariantsNamedAfterBaseMove が実 seed で「連打版は必ず <元技>_rapid で、元技が在る」を見る。
const rapidSuffix = "_rapid"

// rapidTable はキャラの「連打版」の表である(M47-05 追補②・M19-DESIGN-07 §9 項目 8 の親集合駆動の差し込み規則)。
//
// 連打版＝チェーングループ員のうち単独で出せない行(isSoloUnavailable)。連打キャンセルで
// 直前の技に続けて元技を押したときにだけ出る。どの直前の技から出るかは move_derivations の親集合で決まる。
//
//   - ザンギエフ: 連打版 3 行の親は 立ち弱P ・しゃがみ弱P ・しゃがみ弱K ＝ 2 打目以降は常に連打版。
//   - ダルシム: 連打版しゃがみ弱P の親は 立ち弱P ・アジャイルキック だけ ＝ 直前が しゃがみ弱P(素・連打版)なら素が出る
//     (ドメイン知識集成 §3-5 ・開発者の回答 2026-09-28)。
//   - 通常のキャラ: 連打版が無い ＝ 表は空。**⇒ 本表を引く処理は、通常のキャラでは何も変えない。**
type rapidTable struct {
	// rapidOf は元技の code → その連打版の行。
	rapidOf map[string]setplayrepo.MoveCandidate
	// parents は連打版の行の ID → 親(直前に出せる技)の code の集合。
	parents map[int64]map[string]bool
	// baseOf は連打版の code → 元技の code。直前の技が連打版のとき、その元技で親を引く
	// (ザンギエフの「3 打目以降も連打版」＝連打版の後も元技が親として働く)。
	baseOf map[string]string
}

// newRapidTable は候補とその親子関係から連打版の表を作る。
// 連打版として数えるのは「チェーングループ員 ・単独で出せない ・code が <元技>_rapid ・元技が同キャラのチェーン員」の行だけ。
func newRapidTable(cands []setplayrepo.MoveCandidate, parentsByChild map[int64][]int64) rapidTable {
	t := rapidTable{
		rapidOf: map[string]setplayrepo.MoveCandidate{},
		parents: map[int64]map[string]bool{},
		baseOf:  map[string]string{},
	}
	codeByID := make(map[int64]string, len(cands))
	chainMember := make(map[string]bool, len(cands))
	for _, m := range cands {
		codeByID[m.ID] = m.Code
		if m.ChainCancelTotal != nil {
			chainMember[m.Code] = true
		}
	}
	for _, m := range cands {
		if m.ChainCancelTotal == nil || m.Total == nil || *m.Total < 1 {
			continue
		}
		if !isSoloUnavailable(m, len(parentsByChild[m.ID]) > 0) || !strings.HasSuffix(m.Code, rapidSuffix) {
			continue
		}
		base := strings.TrimSuffix(m.Code, rapidSuffix)
		if !chainMember[base] {
			continue
		}
		ps := make(map[string]bool, len(parentsByChild[m.ID]))
		for _, pid := range parentsByChild[m.ID] {
			if c, ok := codeByID[pid]; ok {
				ps[c] = true
			}
		}
		t.rapidOf[base] = m
		t.parents[m.ID] = ps
		t.baseOf[m.Code] = base
	}
	return t
}

// empty は連打版を持たないキャラ(通常のキャラ)か。
func (t rapidTable) empty() bool { return len(t.rapidOf) == 0 }

// next は、直前の技 p の後に技 x を最速で押したとき(連打キャンセル)に実際に出る技を返す。
// x に連打版 R が在り、p の元技(p が連打版ならその元技)が R の親なら R、そうでなければ x。
// 2 つ目の戻り値は連打版に化けたか。
func (t rapidTable) next(p, x setplayrepo.MoveCandidate) (setplayrepo.MoveCandidate, bool) {
	r, ok := t.rapidOf[x.Code]
	if !ok {
		return x, false
	}
	base := p.Code
	if b, ok := t.baseOf[p.Code]; ok {
		base = b
	}
	if t.parents[r.ID][base] {
		return r, true
	}
	return x, false
}
