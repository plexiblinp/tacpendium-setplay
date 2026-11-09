package seedgen

import (
	"fmt"
	"sort"
	"strings"

	"github.com/plexiblinp/tacpendium-setplay/internal/model"
	"github.com/plexiblinp/tacpendium-setplay/internal/moveindex"
)

// resolveCharacter は 1 キャラ分の表記を解決する。
//
// 手順:
//  1. 層 B / 層 A で基底の表記を作る(rush_variant は対象外。派生技は旧規則でだけ対象外)。
//     FormatCurrent では、ここでターゲットコンボを `~`/`(TC)` の形へ写し、空中の技に `j.` を付ける
//     (M43-05。★衝突判定より前に当てる——新しい表記どうしで被りを判定するため)。
//  2. 移動系 9 code を衝突判定の母数へ入れる(投入は別経路だが、同一キャラ内で
//     ぶつかれば逆引きが壊れるため判定には含める)。
//  3. 同一キャラ内で表記が衝突した組を解く。FormatCurrent では優先の 1 つだけ残し
//     (keepPriorCollisions)、旧規則では全部落とす(dropCollisions)。
//  4. 生き残った基底の表記から rush_variant を合成する。
//  5. rush 同士の衝突も同じ関数で解く。
//
// format は規則の版を決める(format.go)。層 C-3(P-34 の 13 件)は旧形式
// FormatPreM2003 では当てない(理由は includesNoInputDerived)。
func resolveCharacter(rule AliasPresetRule, charCode string, rows []MoveRow, format SQLFormat) (
	[]AliasRow, []UnfilledRow, []DiscardedLayerAValue, []UnfilledRow, []AliasCollision) {
	return resolveCharacterWith(rule, charCode, rows, format, nil)
}

// collisionResolver は同一キャラ内の表記の衝突を解く関数(dropCollisions / keepPriorCollisions の形)。
type collisionResolver func(map[string]AliasRow, map[string]string) (map[string]AliasRow, []collision, []AliasCollision)

// resolveCharacterWith は resolveCharacter の本体である。resolve が nil なら format が決める
// 本家の規則で衝突を解き、nil でなければそれを使う。
//
// ★単体版での変更(docs/upstream-diff.md): 本家の resolveCharacter をこの関数へ切り出し、
// 衝突の解き方だけを差し替えられるようにした。単体版は逆引きしないため被りを落とさない(HANDOVER §2.6)。
func resolveCharacterWith(rule AliasPresetRule, charCode string, rows []MoveRow, format SQLFormat, resolve collisionResolver) (
	[]AliasRow, []UnfilledRow, []DiscardedLayerAValue, []UnfilledRow, []AliasCollision) {

	priority := format.resolvesDerivedWithPriority()
	cr := &charResolver{
		rule:     rule,
		charCode: charCode,
		noInput:  format.includesNoInputDerived(),
		priority: priority,
		notate:   format.notatesTargetComboAndAerial(),
		base:     map[string]AliasRow{}, // move_code -> 表記
		byMove:   map[string]MoveRow{},
	}
	cr.tokenOf, cr.indexSkipped = indexTokens(rows)
	cr.resolveBaseRows(rows)

	// ── 衝突: 移動系 9 code も母数に入れて判定する ──────────────────────
	reserved := reservedAliasTexts(rule, priority)
	var resolveCollisions collisionResolver = dropCollisions
	if priority {
		resolveCollisions = keepPriorCollisions(cr.byMove)
	}
	if resolve != nil {
		resolveCollisions = resolve
	}
	var collisions []AliasCollision
	unfilled := cr.unfilled
	base, collided, kept := resolveCollisions(cr.base, reserved)
	collisions = append(collisions, kept...)
	for _, c := range collided {
		unfilled = append(unfilled, mkUnfilled(cr.byMove[c.moveCode], ReasonCollision, c.detail))
	}

	// ── 層 C-2: rush_variant を合成する ────────────────────────────────
	rushBase, unfilled := synthesizeRushRows(charCode, cr.rushRows, cr.byMove, base, unfilled)
	// rush の表記が基底・移動系とぶつかっていないかも見る
	// (reserved は移動系 9 code で初期化済み。そこへ生き残った基底の表記を足す)。
	for code, row := range base {
		reserved[row.AliasText] = code
	}
	rushBase, rushCollided, rushKept := resolveCollisions(rushBase, reserved)
	collisions = append(collisions, rushKept...)
	for _, c := range rushCollided {
		unfilled = append(unfilled, mkUnfilled(cr.byMove[c.moveCode], ReasonCollision, c.detail))
	}

	out := sortResolved(base, rushBase, unfilled, collisions)
	return out, unfilled, cr.discarded, cr.saNoAnnotation, collisions
}

// charResolver は 1 キャラ分の基底の表記を作る間の状態(規則 ・形式から決まる切り替え ・積み上げる結果)。
type charResolver struct {
	rule     AliasPresetRule
	charCode string
	noInput  bool // 層 C-3 を当てるか
	priority bool // 派生技も層 A にし、被ったら優先の 1 つだけ残すか
	notate   bool // ターゲットコンボ ・空中の技の表記を当てるか

	tokenOf      map[string]string // move_code -> 正規化キー
	indexSkipped map[string]bool   // 索引に載らなかった move_code

	base           map[string]AliasRow
	byMove         map[string]MoveRow
	rushRows       []MoveRow
	unfilled       []UnfilledRow
	discarded      []DiscardedLayerAValue
	saNoAnnotation []UnfilledRow
}

// indexTokens は各行の command を正規化し、move_code ごとの正規化キーと、索引に載らなかった
// move_code の集合を返す。
func indexTokens(rows []MoveRow) (map[string]string, map[string]bool) {
	// 正規化キーを得るための索引。★派生行も正規化できるよう isDerived=false を渡す
	// (層 A の対象にするかどうかの判定は resolveBaseRows で別に行う。ここでは「捨てた層 A の値」を
	//  報告するために全行の正規化結果が要る = D-2 (b))。
	ix := moveindex.New()
	for i, r := range rows {
		if r.isMovementSystem() || strings.TrimSpace(r.Command) == "" {
			continue
		}
		ix.Add(r.CharacterCode, r.MoveCode, r.Command, false, i)
	}
	tokenOf := map[string]string{}
	for _, e := range ix.Entries() {
		tokenOf[e.MoveCode] = e.TokenKey
	}
	indexSkipped := map[string]bool{}
	for _, s := range ix.Skipped() {
		indexSkipped[s.MoveCode] = true
	}
	return tokenOf, indexSkipped
}

// resolveBaseRows は rush_variant 以外の行に基底の表記を当てる(層 B → 派生 → 層 A の順)。
// rush_variant の行は後で合成するため、取り分けておくだけである。
func (cr *charResolver) resolveBaseRows(rows []MoveRow) {
	for _, r := range rows {
		cr.byMove[r.MoveCode] = r
		if r.isMovementSystem() {
			continue // 移動系は別経路(そもそも CSV から drop される)
		}
		if r.Category == "rush_variant" {
			cr.rushRows = append(cr.rushRows, r)
			continue
		}
		if cr.resolveLayerBRow(r) {
			continue
		}

		// ── 層 A(command が表記を決める行) ──────────────────────────────
		// ★旧規則では is_derived=true は対象外(U-2)。M48-08(D-1037)で U-2 を撤回し、
		//   FormatCurrent では command を持つ派生技も層 A の対象にする。
		//   ★command が空の派生技(層 C-3 ・derived-no-command)の扱いは変えていない。
		if r.IsDerived && (!cr.priority || strings.TrimSpace(r.Command) == "") {
			cr.resolveDerivedRow(r)
			continue
		}
		cr.resolveLayerARow(r)
	}
}

// resolveLayerBRow は move_code の構造が表記を決める行なら表記を当てて true を返す。
func (cr *charResolver) resolveLayerBRow(r MoveRow) bool {
	// ── 層 B(move_code の構造が表記を決める行) ───────────────────────
	text, ok := layerB(cr.rule, r.MoveCode)
	if !ok {
		return false
	}
	// ★層 B の `jumping_*` は既に `j.` で始まるため、空中の接頭辞は二重に付かない(M43-05)。
	if cr.notate {
		text = aerialNotation(r, text)
	}
	cr.base[r.MoveCode] = AliasRow{CharCode: cr.charCode, MoveCode: r.MoveCode, AliasText: text, Layer: LayerB}
	// ★捨てた層 A の値を記録する(D-2 (b)「黙って捨てない」)。
	if a, has := cr.tokenOf[r.MoveCode]; has && a != text {
		cr.discarded = append(cr.discarded, DiscardedLayerAValue{
			CharCode: cr.charCode, MoveCode: r.MoveCode,
			LayerAKey: a, LayerBKey: text, LostInfo: lostInfo(a, text),
		})
	}
	return true
}

// resolveDerivedRow は層 A の対象にしない派生行を扱う(値の決まった行だけ層 C-3 で当て、残りは穴として記録する)。
func (cr *charResolver) resolveDerivedRow(r MoveRow) {
	// ★command が無い派生行(D-315 の 28 件)は別立てで記録する。
	//   規則をどう広げても埋まらない類であり、値は設計卓が決めた(P-34 → D-367)。
	if strings.TrimSpace(r.Command) == "" {
		// ── 層 C-3: 値が決まった 13 件だけを投入する(M20-06) ─────────
		if cr.noInput && isNoInputDerivedTarget(cr.charCode, r.MoveCode) {
			cr.base[r.MoveCode] = AliasRow{
				CharCode: cr.charCode, MoveCode: r.MoveCode,
				AliasText:   r.NameJA,
				AliasTextEn: humanizeMoveCode(r.MoveCode),
				Layer:       LayerCNoInput,
			}
			return
		}
		cr.unfilled = append(cr.unfilled, mkUnfilled(r, ReasonDerivedNoCommand, ""))
		return
	}
	cr.unfilled = append(cr.unfilled, mkUnfilled(r, ReasonDerived, ""))
}

// resolveLayerARow は command の正規化キーから表記を作る。作れない行は穴として記録する。
func (cr *charResolver) resolveLayerARow(r MoveRow) {
	if strings.TrimSpace(r.Command) == "" {
		cr.unfilled = append(cr.unfilled, mkUnfilled(r, ReasonNoCommand, ""))
		return
	}
	if cr.indexSkipped[r.MoveCode] {
		cr.unfilled = append(cr.unfilled, mkUnfilled(r, ReasonIndexSkip, r.Command))
		return
	}
	text, ok := cr.tokenOf[r.MoveCode]
	if !ok {
		cr.unfilled = append(cr.unfilled, mkUnfilled(r, ReasonIndexSkip, r.Command))
		return
	}
	ann := saAnnotation(r.Category, r.MoveCode)
	if ann == "" && (r.Category == "super_art" || r.Category == "critical_art") {
		// ★注記が付かなかった SA / CA を記録する(M20-05)。投入は止めない——
		//   表記自体は正しく、止めると穴が増えるだけである。報告で気づけるようにする。
		cr.saNoAnnotation = append(cr.saNoAnnotation, mkUnfilled(r, "sa-no-annotation",
			"move_code から SA/CA 番号を抽出できない"))
	}
	text += ann
	if cr.notate {
		// ── M43-05: ターゲットコンボの `~`/`(TC)` ・空中の技の `j.` ──────────────
		if r.Category == model.MoveCategoryTargetCombo {
			tc, ok := renderTargetCombo(cr.rule, r, text)
			if !ok {
				// ★旧い `>` の形のまま黙って投入しない(新旧の表記が混ざる)。穴として報告する。
				cr.unfilled = append(cr.unfilled, mkUnfilled(r, ReasonTargetComboUnrenderable, r.Command))
				return
			}
			text = tc
		} else {
			text = aerialNotation(r, text)
		}
	}
	cr.base[r.MoveCode] = AliasRow{
		CharCode: cr.charCode, MoveCode: r.MoveCode,
		AliasText: text, Layer: LayerA,
	}
}

// reservedAliasTexts は衝突判定で先に押さえる表記と、その出所を返す。押さえるのは移動系 9 code の表記で、
// priority のときは他のプリセットの移動系の表記も押さえる。
func reservedAliasTexts(rule AliasPresetRule, priority bool) map[string]string {
	reserved := map[string]string{} // 表記 -> 出所(報告用)
	for _, code := range movementCodeOrder {
		reserved[rule.Movement[code].text] = code
	}
	if priority {
		// ★M48-08(Plan 未決 5 の開発者決定): 他のプリセットの移動系の表記も予約する。
		//   逆引きの辞書は全プリセット横断である(DES-004 §5.5 の注記)。予約しないと、
		//   srk の派生技(alex/low_retreat 等)が numeric の移動系の表記 `4` を取り、
		//   取込で `4` が「歩き」と「その技」の 2 候補に割れる。
		for _, other := range []AliasPresetRule{NumericRule, SRKRule} {
			for _, code := range movementCodeOrder {
				if _, taken := reserved[other.Movement[code].text]; !taken {
					reserved[other.Movement[code].text] = other.PresetCode + ":" + code
				}
			}
		}
	}
	return reserved
}

// synthesizeRushRows は生き残った基底の表記から rush_variant の表記を合成する。
// 合成できない行は unfilled へ足して返す。
func synthesizeRushRows(charCode string, rushRows []MoveRow, byMove map[string]MoveRow,
	base map[string]AliasRow, unfilled []UnfilledRow) (map[string]AliasRow, []UnfilledRow) {
	rushBase := map[string]AliasRow{}
	for _, r := range rushRows {
		orig := strings.TrimSpace(r.OriginalMoveCode)
		if orig == "" {
			unfilled = append(unfilled, mkUnfilled(r, ReasonRushNoOriginal, "original_move_code が空"))
			continue
		}
		// ★元技が CSV に実在するかを確かめる。実在しない場合は当てない(D-305)。
		//
		// ★★【2026-09-11 更新 = M35-03】実データの食い違いは 0 件になった。
		// かつて original_move_code が glowing_touch_1 のように実体(glowing_touch_1hits)と
		// 食い違う入力ミスが 4 件あり(ingrid 2 / lily 1 / mai 1)、DB 側では
		// original_move_id が NULL になっていた。⇒ M35-03 が CSV 正本を是正し、
		// golden 000026 / 000072 / 000073 を再生成して解消した(実測 0 件)。
		//
		// ★★★それでも本ガードは残す。⇒ D-305 の「推測で元技を当てない」は 1 文字も
		// 変わっていない。変わったのは「推測しなくても解決できるようになった」ことだけである。
		// ★同型の入力ミスは CSV の手入力でいつでも再発しうる。再発を検出する床は 2 本:
		//   - internal/seedgen/generate_m2002_golden_test.go の rushNoOriginal: 0
		//   - internal/infra/migration/migrate_m2002_test.go の
		//     「v73 時点で original_move_id が NULL の rush_variant が 0 行」(母数付き)
		// ⇒ 本ガードを外すと、その 2 本は緑のまま「推測で当てた別名」が投入される。
		if _, exists := byMove[orig]; !exists {
			unfilled = append(unfilled, mkUnfilled(r, ReasonRushNoOriginal,
				fmt.Sprintf("original_move_code=%q が実在しない", orig)))
			continue
		}
		ob, ok := base[orig]
		if !ok {
			unfilled = append(unfilled, mkUnfilled(r, ReasonRushOriginalUnfilled,
				fmt.Sprintf("元技 %s の表記が無い", orig)))
			continue
		}
		rushBase[r.MoveCode] = AliasRow{
			CharCode: charCode, MoveCode: r.MoveCode,
			AliasText: rushPrefix + rushJoiner + ob.AliasText, Layer: LayerCRush,
		}
	}
	return rushBase, unfilled
}

// sortResolved は基底と rush の表記を 1 本にまとめて move_code 順に並べ、unfilled と
// collisions もその場で並べる。
func sortResolved(base, rushBase map[string]AliasRow, unfilled []UnfilledRow, collisions []AliasCollision) []AliasRow {
	out := make([]AliasRow, 0, len(base)+len(rushBase))
	for _, m := range []map[string]AliasRow{base, rushBase} {
		for _, row := range m {
			out = append(out, row)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].MoveCode < out[j].MoveCode })
	sort.Slice(unfilled, func(i, j int) bool {
		if unfilled[i].MoveCode != unfilled[j].MoveCode {
			return unfilled[i].MoveCode < unfilled[j].MoveCode
		}
		return unfilled[i].Reason < unfilled[j].Reason
	})
	sort.Slice(collisions, func(i, j int) bool { return collisions[i].AliasText < collisions[j].AliasText })
	return out
}
