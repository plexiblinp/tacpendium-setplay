package movesdb

import (
	"strings"
)

// 英語技名は move_code から規則で作る(HANDOVER D7・§4.2)。規則で表せない分は
// data/overrides/name_en.json で人が補う。★公式サイト等から転記しない(NOTICE §2)。

// strengthWords は強度の語。名前の先頭へ回す。
var strengthWords = map[string]string{"light": "Light", "medium": "Medium", "heavy": "Heavy"}

// upperTokens はそのまま大文字にする語。
var upperTokens = map[string]bool{"od": true, "sa1": true, "sa2": true, "sa3": true, "ca": true, "tc": true}

// smallWords は先頭以外で小文字のままにする語(no は日本語の「の」のローマ字)。
var smallWords = map[string]bool{"of": true, "the": true, "and": true, "in": true, "on": true, "to": true, "a": true, "from": true, "no": true}

// rushWord はラッシュ版の英語の接頭。
const rushWord = "Drive Rush"

// leadingWords は強度・OD より前に置く語(位置・経路を表す接頭)。
var leadingWords = map[string]string{"rush": rushWord, "standing": "Standing", "crouching": "Crouching", "jumping": "Jumping"}

// holdSuffix はホールドの語を末尾の括弧書きへ写す(max_holding は 2 語で 1 つ)。
const (
	holdSuffix    = "(Hold)"
	maxHoldSuffix = "(Max Hold)"
)

// movementNames は移動系 9 code の英語名(CSV に無く、本家 DB が seed する共通の技)。
var movementNames = map[string]string{
	"forward": "Walk Forward", "back": "Walk Back",
	"micro_forward": "Micro Walk Forward", "micro_back": "Micro Walk Back",
	"dash_forward": "Forward Dash", "dash_back": "Back Dash",
	"jump_neutral": "Neutral Jump", "jump_forward": "Forward Jump", "jump_back": "Back Jump",
}

// hitCountSuffix は段数の語を括弧書きへ写す(例 2hits → (2 hits))。
var hitCountSuffix = map[string]string{
	"1hit": "(1 hit)", "1hits": "(1 hit)", "2hit": "(2 hits)", "2hits": "(2 hits)", "3hits": "(3 hits)", "4hits": "(4 hits)", "5hits": "(5 hits)",
}

// GenerateNameEN は move_code から英語技名を作る。
//
// 規則:
//   - 状態の code(custom_states の states[].code)で始まる技は、その英語名を角括弧で前に付ける
//     (例 denjin_charge_hadoken → [Denjin Charge] Hadoken)
//   - 移動系 9 code は固定の名前(movementNames)
//   - throw_<向き> は <向き> Throw(例 throw_forward → Forward Throw)
//   - rush/standing/crouching/jumping は先頭に残し(rush は Drive Rush)、その後へ od と強度を回す
//     (例 hadoken_od → OD Hadoken、crouching_light_punch → Crouching Light Punch)
//   - ただし during より後ろの語(派生元の技)は語順のまま
//   - 段数(2hits 等)とホールド(holding・max_holding)は末尾の括弧書き
//   - 単独の s は直前の語の所有格(the_devil_s_song → The Devil's Song)
//   - 段階つきの状態は段階の数字も印に含める(drink_level_4_senei_kick → [Drink Level 4] Senei Kick)
//   - up<n> は末尾の (+n)、lv<n> は Lv<n>、sa1 等は大文字
//   - of/the 等は先頭と SA1 等の直後以外で小文字、ほかは先頭大文字
func GenerateNameEN(code string, states []State) string {
	if n, ok := movementNames[code]; ok {
		return n
	}
	var prefix string
	label, rest := stateLabel(code, states)
	if label != "" {
		prefix = "[" + label + "] "
		if r, ok := strings.CutPrefix(rest, "rush_"); ok {
			prefix = rushWord + " " + prefix
			rest = r
		}
	}
	tokens := strings.Split(rest, "_")
	if len(tokens) == 2 && tokens[0] == "throw" {
		tokens = []string{tokens[1], "throw"}
	}

	var lead, head, body, tail []string
	inPlace := false // during の後ろの語は別の技を指すので、強度・OD を先頭へ回さない
	for i, t := range tokens {
		switch {
		case t == "":
			continue
		case t == "during":
			inPlace = true
			body = append(body, t)
		case inPlace && t == "od":
			body = append(body, "OD")
		case inPlace && strengthWords[t] != "":
			body = append(body, strengthWords[t])
		case leadingWords[t] != "" && len(head) == 0 && len(body) == 0:
			lead = append(lead, leadingWords[t])
		case t == "max" && i+1 < len(tokens) && tokens[i+1] == "holding":
			tail = append(tail, maxHoldSuffix)
			tokens[i+1] = ""
		case t == "holding":
			tail = append(tail, holdSuffix)
		case t == "od":
			head = append([]string{"OD"}, head...)
		case strengthWords[t] != "":
			head = append(head, strengthWords[t])
		case hitCountSuffix[t] != "":
			tail = append(tail, hitCountSuffix[t])
		case t == "s" && i > 0 && len(body) > 0:
			body[len(body)-1] += "'s"
		case upperTokens[t]:
			body = append(body, strings.ToUpper(t))
		case strings.HasPrefix(t, "up") && len(t) > 2 && isDigits(t[2:]):
			tail = append(tail, "(+"+t[2:]+")")
		case strings.HasPrefix(t, "lv") && len(t) > 2 && isDigits(t[2:]):
			body = append(body, "Lv"+t[2:])
		default:
			body = append(body, t)
		}
	}
	words := append(append(lead, head...), body...)
	for i, w := range words {
		if i > 0 && smallWords[w] && !upperTokens[strings.ToLower(words[i-1])] {
			continue
		}
		words[i] = titleWord(w)
	}
	words = append(words, tail...)
	return prefix + strings.Join(words, " ")
}

func titleWord(w string) string {
	if w == "" || w[0] < 'a' || w[0] > 'z' {
		return w
	}
	return strings.ToUpper(w[:1]) + w[1:]
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// matchState は move_code が状態 code で始まるとき、その状態と残りの move_code を返す。
// 状態 code どうしが接頭辞の関係にあるとき(例 order_of_the_sun と order_of_the_sun_strength)は長いほうを採る。
// ラッシュ版(rush_ で始まる)は rush_ の後ろを見る。
func matchState(code string, states []State) (State, string, bool) {
	body := strings.TrimPrefix(code, "rush_")
	var best State
	var rest string
	found := false
	for _, st := range states {
		r, ok := strings.CutPrefix(body, st.Code+"_")
		if !ok || (found && len(st.Code) <= len(best.Code)) {
			continue
		}
		best, rest, found = st, r, true
	}
	return best, rest, found
}

// stateLabel は状態 code で始まる技の状態の英語名と、残りの move_code を返す。段階つきの状態
// (例 drink_level_4_...)は段階の数字も含める。状態で始まらなければ label は空、rest は code。
// ★ラッシュ版は rest の先頭に rush_ を残す(呼び出し側が Drive Rush を付ける)。
func stateLabel(code string, states []State) (label, rest string) {
	st, r, ok := matchState(code, states)
	if !ok || st.Name.EN == "" {
		return "", code
	}
	label = st.Name.EN
	if n, r2, found := strings.Cut(r, "_"); found && isDigits(n) {
		label += " " + n
		r = r2
	}
	if strings.HasPrefix(code, "rush_") {
		r = "rush_" + r
	}
	return label, r
}

// mainTokens は move_code の語のうち、during より前のもの(後ろは派生元の技を指す)。
func mainTokens(code string) []string {
	toks := strings.Split(code, "_")
	for i, t := range toks {
		if t == "during" {
			return toks[:i]
		}
	}
	return toks
}
