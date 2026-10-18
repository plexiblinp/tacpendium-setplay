package movesdb

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/plexiblinp/tacpendium-setplay/internal/seedgen"
)

// 本ファイルは numeric / SRK の表記を作り、同一キャラ内の被りを区別印で解く(HANDOVER §2.6・§4.3・D9)。
//
// 基底の表記は本家の生成器(internal/seedgen の規則 = 本家 preset_aliases と同じ)で作る。
// 単体版は逆引きしないので、本家が被りで落とした技にも表記を付け、被った組の中で
// 「違う部分」だけを印にして区別する。印は被った組の技にだけ付く(被らない技は本家と同じ文字列)。
//
// 区別の段(上から順に、まだ被っている技にだけ当てる。組の全員が同じ値の段は使わない):
//  1. 空中印: move_code の語(aerial・air・jumping_・neutral_jump・high_jump)が組の中で違う → 空中の版に
//     j.(垂直ジャンプは nj.・ハイジャンプは hj.)。★is_aerial は空中技の印ではないので使わない(ドメイン知識集成 §7-2)
//  2. OD 印: move_code の od トークンが組の中で違う → OD の技に " (OD)"
//  3. ラベル: numeric は name_ja の [..]/【..】(前へ)と (..)/（..）(後ろへ)のうち組で共有しないもの。
//     SRK は状態の英語名(状態 code で始まる技に [..])と、move_code の語のうち組で共有しないもの(後ろへ)
//  4. 名前: それでも被る技に、その表記の言語の技名を括弧で添える(numeric = name_ja、SRK = 英語技名)
//
// 本家の表記が付かない技(command が空の派生など)は、numeric = name_ja、SRK = 英語技名を基底にする
// (本家の層 C-3 と同じ考え方: 新しい入力情報を作らない)。

// NotationSource は基底の表記の出所。報告と検査に使う(JSON には出さない)。
type NotationSource string

const (
	// SourceGenerator は seedgen の規則(本家の preset_aliases と同じ規則)。
	SourceGenerator NotationSource = "generator"
	// SourceMovement は移動系 9 code の固定表(seedgen の NumericRule / SRKRule)。
	SourceMovement NotationSource = "movement"
	// SourceDB は CSV に無い技(drive_reversal・連打版)の本家 DB の preset_aliases。
	SourceDB NotationSource = "db"
	// SourceName は規則が当たらない技の名前フォールバック。
	SourceName NotationSource = "name"
	// SourceOverride は data/overrides/notation.json。
	SourceOverride NotationSource = "override"
)

// Mark は区別の段の名前(報告用)。
type Mark string

const (
	MarkAerial Mark = "aerial"
	MarkOD     Mark = "od"
	MarkLabel  Mark = "label"
	MarkName   Mark = "name"
)

// notationKind は numeric / SRK のどちらか。
type notationKind string

const (
	kindNumeric notationKind = "numeric"
	kindSRK     notationKind = "srk"
)

// moveInfo は表記づくりに要る技の情報。
type moveInfo struct {
	move     DBMove
	nameJA   string
	nameEN   string
	base     string
	source   NotationSource
	text     string
	marks    []Mark
	unfilled string  // 本家の生成器で埋まらなかった理由(SourceName のとき)
	states   []State // この技が前提とする状態(moveStates。0 か 1 件)
}

// NotationResult は 1 キャラ・1 表記種の結果。
type NotationResult struct {
	Kind   string
	Text   map[string]string // move_code → 表示文字列
	Base   map[string]string // move_code → 区別印を付ける前の文字列
	Source map[string]NotationSource
	Marks  map[string][]Mark
	// Unfilled は本家の生成器が表記を作れなかった技と理由(名前フォールバックの内訳)。
	Unfilled map[string]string
}

// buildNotation は 1 キャラ分の numeric または SRK を作る。
func buildNotation(kind notationKind, char DBCharacter, stateByMove map[string][]State, moves []DBMove, csvRows []seedgen.MoveRow,
	dbAliases map[int64]Alias, names map[string]Notation, overrides map[string]NotationOverride) (*NotationResult, error) {

	rule := seedgen.NumericRule
	if kind == kindSRK {
		rule = seedgen.SRKRule
	}
	pick := func(text, textEn string) string {
		if kind == kindSRK && textEn != "" {
			return textEn
		}
		return text
	}

	resolved, unfilled := seedgen.ResolveAllAliases(rule, char.Code, csvRows)
	gen := map[string]string{}
	for _, r := range resolved {
		gen[r.MoveCode] = pick(r.AliasText, r.AliasTextEn)
	}
	unfilledReason := map[string]string{}
	for _, u := range unfilled {
		unfilledReason[u.MoveCode] = string(u.Reason)
	}
	inCSV := map[string]bool{}
	for _, r := range csvRows {
		inCSV[r.MoveCode] = true
	}

	infos := make([]*moveInfo, 0, len(moves))
	for _, m := range moves {
		mi := &moveInfo{move: m, nameJA: names[m.Code].NameJA, nameEN: names[m.Code].NameEN, states: stateByMove[m.Code]}
		switch {
		case gen[m.Code] != "":
			mi.base, mi.source = gen[m.Code], SourceGenerator
		case !inCSV[m.Code]:
			if t, te, ok := seedgen.MovementAlias(rule, m.Code); ok {
				mi.base, mi.source = pick(t, te), SourceMovement
			} else if a, ok := dbAliases[m.ID]; ok {
				mi.base, mi.source = pick(a.Text, a.TextEn), SourceDB
			}
		}
		if mi.base == "" {
			mi.source = SourceName
			mi.unfilled = unfilledReason[m.Code]
			if mi.unfilled == "" {
				mi.unfilled = "not-in-csv"
			}
			if kind == kindSRK {
				mi.base = mi.nameEN
			} else {
				mi.base = mi.nameJA
			}
		}
		if ov, ok := overrides[m.Code]; ok {
			v := ov.Numeric
			if kind == kindSRK {
				v = ov.SRK
			}
			if v != "" {
				mi.base, mi.source = v, SourceOverride
			}
		}
		mi.text = mi.base
		infos = append(infos, mi)
	}

	labelLayer := nameJALabelAffix
	if kind == kindSRK {
		labelLayer = func(g []*moveInfo) []affix { return codeLabelAffix(g) }
	}
	layers := []struct {
		mark Mark
		fn   func([]*moveInfo) []affix
	}{
		{MarkAerial, aerialAffix},
		{MarkOD, odAffix},
		{MarkLabel, labelLayer},
		{MarkName, func(g []*moveInfo) []affix { return nameAffix(g, kind) }},
	}

	for _, l := range layers {
		for _, g := range collisionGroups(infos) {
			affixes := l.fn(g)
			if affixes == nil {
				continue
			}
			for i, mi := range g {
				if affixes[i].prefix == "" && affixes[i].suffix == "" {
					continue
				}
				mi.text = affixes[i].prefix + mi.text + affixes[i].suffix
				mi.marks = append(mi.marks, l.mark)
			}
		}
	}

	res := &NotationResult{
		Kind: string(kind), Text: map[string]string{}, Base: map[string]string{},
		Source: map[string]NotationSource{}, Marks: map[string][]Mark{}, Unfilled: map[string]string{},
	}
	for _, mi := range infos {
		if mi.text == "" {
			return nil, fmt.Errorf("%s/%s: %s が空", char.Code, mi.move.Code, kind)
		}
		res.Text[mi.move.Code] = mi.text
		res.Base[mi.move.Code] = mi.base
		res.Source[mi.move.Code] = mi.source
		if len(mi.marks) > 0 {
			res.Marks[mi.move.Code] = mi.marks
		}
		if mi.source == SourceName {
			res.Unfilled[mi.move.Code] = mi.unfilled
		}
	}
	return res, nil
}

// affix は区別印(前に付ける文字列・後ろに付ける文字列)。
type affix struct{ prefix, suffix string }

// collisionGroups は表示文字列が同じ技の組(2 件以上)を返す。決定性のため文字列順・moves の並び順。
func collisionGroups(infos []*moveInfo) [][]*moveInfo {
	by := map[string][]*moveInfo{}
	for _, mi := range infos {
		by[mi.text] = append(by[mi.text], mi)
	}
	keys := make([]string, 0, len(by))
	for k, g := range by {
		if len(g) > 1 {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	out := make([][]*moveInfo, 0, len(keys))
	for _, k := range keys {
		out = append(out, by[k])
	}
	return out
}

// distinct は組の全員が同じ値なら false。
func distinct(vals []string) bool {
	for _, v := range vals[1:] {
		if v != vals[0] {
			return true
		}
	}
	return false
}

// aerialAffix は空中印。move_code の語で空中の版と分かる技に j.(垂直ジャンプは nj.・ハイジャンプは hj.)。
// ★is_aerial は使わない。is_aerial は「空中技」ではなく「ラッシュ版の自動生成から外す」印である
// (ドメイン知識集成 §7-2)。★既に j. 等で始まる表記には付けない。
func aerialAffix(g []*moveInfo) []affix {
	vals := make([]string, len(g))
	for i, mi := range g {
		vals[i] = aerialPrefix(mi.move.Code)
	}
	if !distinct(vals) {
		return nil
	}
	out := make([]affix, len(g))
	for i, mi := range g {
		if vals[i] == "" || hasAerialPrefix(mi.text) {
			continue
		}
		out[i].prefix = vals[i]
	}
	return out
}

// aerialPrefix は move_code の主部の語から空中印を決める(該当しなければ空)。
func aerialPrefix(code string) string {
	main := strings.Join(mainTokens(code), "_")
	switch {
	case strings.Contains(main, "high_jump"):
		return "hj."
	case strings.Contains(main, "neutral_jump"):
		return "nj."
	case hasToken(code, "aerial") || hasToken(code, "air") || strings.HasPrefix(code, "jumping_"):
		return "j."
	}
	return ""
}

func hasAerialPrefix(s string) bool {
	return strings.HasPrefix(s, "j.") || strings.HasPrefix(s, "nj.") || strings.HasPrefix(s, "hj.")
}

// hasToken は move_code の主部(during より前)に語 tok があるか。
func hasToken(code, tok string) bool {
	for _, t := range mainTokens(code) {
		if t == tok {
			return true
		}
	}
	return false
}

// odAffix は OD 印。
func odAffix(g []*moveInfo) []affix {
	vals := make([]string, len(g))
	for i, mi := range g {
		if hasToken(mi.move.Code, "od") {
			vals[i] = "od"
		}
	}
	if !distinct(vals) {
		return nil
	}
	out := make([]affix, len(g))
	for i := range g {
		if vals[i] != "" {
			out[i].suffix = " (OD)"
		}
	}
	return out
}

var (
	bracketLabel = regexp.MustCompile(`\[([^\]]+)\]|【([^】]+)】`)
	parenLabel   = regexp.MustCompile(`[（(]([^）)]+)[）)]`)
)

// nameJALabelAffix は numeric のラベル印。name_ja の [..]/【..】 と (..)/（..）のうち、組で共有しないもの。
func nameJALabelAffix(g []*moveInfo) []affix {
	type labels struct{ pre, post []string }
	ls := make([]labels, len(g))
	count := map[string]int{}
	for i, mi := range g {
		for _, m := range bracketLabel.FindAllStringSubmatch(mi.nameJA, -1) {
			ls[i].pre = append(ls[i].pre, m[1]+m[2])
		}
		for _, m := range parenLabel.FindAllStringSubmatch(mi.nameJA, -1) {
			ls[i].post = append(ls[i].post, m[1])
		}
		seen := map[string]bool{}
		for _, l := range append(append([]string(nil), ls[i].pre...), ls[i].post...) {
			if !seen[l] {
				count[l]++
				seen[l] = true
			}
		}
	}
	out := make([]affix, len(g))
	vals := make([]string, len(g))
	for i := range g {
		var pre, post []string
		for _, l := range ls[i].pre {
			if count[l] < len(g) {
				pre = append(pre, "["+l+"]")
			}
		}
		for _, l := range ls[i].post {
			if count[l] < len(g) {
				post = append(post, "("+l+")")
			}
		}
		if len(pre) > 0 {
			out[i].prefix = strings.Join(pre, "") + " "
		}
		if len(post) > 0 {
			out[i].suffix = " " + strings.Join(post, " ")
		}
		vals[i] = out[i].prefix + "|" + out[i].suffix
	}
	if !distinct(vals) {
		return nil
	}
	return out
}

// codeLabelTokensSkip は SRK のラベルに使わない語(前の段や基底の表記が既に表しているもの)。
var codeLabelTokensSkip = map[string]bool{"od": true, "aerial": true, "air": true}

// codeLabelAffix は SRK のラベル印。状態 code で始まる技に [状態の英語名](段階つきは数字も)、
// move_code の語のうち組で共有しないものを後ろへ括弧書き。
func codeLabelAffix(g []*moveInfo) []affix {
	toks := make([][]string, len(g))
	pre := make([]string, len(g))
	count := map[string]int{}
	for i, mi := range g {
		label, rest := stateLabel(mi.move.Code, mi.states)
		pre[i] = label
		seen := map[string]bool{}
		for _, t := range strings.Split(rest, "_") {
			if t == "" || codeLabelTokensSkip[t] || seen[t] {
				continue
			}
			if strings.HasPrefix(t, "lv") && isDigits(t[2:]) {
				t = "Lv" + t[2:] // numeric(name_ja の Lv1)と表記を揃える
			}
			seen[t] = true
			toks[i] = append(toks[i], t)
			count[t]++
		}
	}
	out := make([]affix, len(g))
	vals := make([]string, len(g))
	preDistinct := distinct(pre)
	for i := range g {
		var words []string
		for _, t := range toks[i] {
			if count[t] < len(g) {
				words = append(words, t)
			}
		}
		if preDistinct && pre[i] != "" {
			out[i].prefix = "[" + pre[i] + "] "
		}
		if len(words) > 0 {
			out[i].suffix = " (" + strings.Join(words, " ") + ")"
		}
		vals[i] = out[i].prefix + "|" + out[i].suffix
	}
	if !distinct(vals) {
		return nil
	}
	return out
}

// nameAffix は最後の段。表記の言語の技名を括弧で添える。
func nameAffix(g []*moveInfo, kind notationKind) []affix {
	out := make([]affix, len(g))
	for i, mi := range g {
		n := mi.nameJA
		if kind == kindSRK {
			n = mi.nameEN
		}
		out[i].suffix = " (" + n + ")"
	}
	return out
}
