package seedgen

// 本ファイルは単体版(tacpendium-setplay)だけが持つ追加である(本家に無い。docs/upstream-diff.md)。

// ResolveAllAliases は 1 キャラ分の表記を、同一キャラ内の被りを落とさずに解決する。
//
// 本家の GenerateAliases は逆引きのため被った組の 1 つだけを残す(M48-08)。単体版は表示専用で
// 逆引きしないので、被った技にも表記を付け、区別は呼び出し側(internal/movesdb)が状態印などで行う
// (HANDOVER §2.6・§4.3)。規則(層 A/B/C)は本家の FormatCurrent と同じ。
//
// 戻り値の AliasRow は move_code 昇順。UnfilledRow は表記を作れなかった行(理由つき)。
// ★移動系 9 code は CSV に無いので戻り値に含まれない(MovementAlias で引く)。
func ResolveAllAliases(rule AliasPresetRule, charCode string, rows []MoveRow) ([]AliasRow, []UnfilledRow) {
	resolved, unfilled, _, _, _ := resolveCharacterWith(rule, charCode, rows, FormatCurrent, keepAllCollisions)
	return resolved, unfilled
}

// MovementAlias は移動系 9 code の表記を返す(rule.Movement)。該当しなければ ok=false。
func MovementAlias(rule AliasPresetRule, moveCode string) (text, textEn string, ok bool) {
	ma, ok := rule.Movement[moveCode]
	return ma.text, ma.textEn, ok
}

// keepAllCollisions は衝突を解かず、全行を残す collisionResolver である。
// ★移動系 9 code(reserved)と被った行も残す(区別は呼び出し側)。
func keepAllCollisions(rows map[string]AliasRow, _ map[string]string) (map[string]AliasRow, []collision, []AliasCollision) {
	return rows, nil, nil
}

// UpstreamUnfilled は本家の規則(FormatCurrent。被りは優先の 1 つだけ残す)で表記が付かない技と、その理由を返す。
// キーは move_code、値は理由(collision・rush-original-unfilled 等)。オラクル突合で
// 「本家 DB に表記が無い技が、本家の生成器が実際に落とした技か」を確かめるために使う。
func UpstreamUnfilled(rule AliasPresetRule, charCode string, rows []MoveRow) map[string]string {
	_, unfilled, _, _, _ := resolveCharacter(rule, charCode, rows, FormatCurrent)
	out := make(map[string]string, len(unfilled))
	for _, u := range unfilled {
		out[u.MoveCode] = string(u.Reason)
	}
	return out
}
