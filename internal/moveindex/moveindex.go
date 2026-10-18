// Package moveindex は「クリーンなトークン(正準 command 表記) → キャラ別 move_code」の
// 索引を構築する独立モジュール(IF)である(M14-03b seed 契約 (f))。
//
// 設計意図(指示書 M14-03b §4.8・M17-02 §4.2・command-index-resolution-design v2.1.0):
//   - 索引に載せるのは非派生技(is_derived=false)のみ。派生技は載せない(段階2 で非対応)。
//   - Lookup は非派生候補から move.id 最小でタイブレークして単一返却する。
//   - LookupAll は候補一覧(デバッグ/人レビュー用)を返す。
//   - command 空欄・未知トークン(raw{...} および語彙外トークン)・条件残留 cond{...} の行は
//     索引に載せず記録する(fail しない。RESEARCH T8/T9/T10)。
//   - 索引キーは numpad 正規化済みキー(M17-02 §4.2 で確定。例 '2MP'・'236LP'・'[4]6LP')。
//     語彙は TOOL-002 §9.9(20260709 レポート §1.7 転記)が正であり、本パッケージは写像のみを行い
//     語彙を再定義しない。Add(構築)と Lookup(クエリ)の両方が同一の正規化を通る。
//
// 消費者は seedgen(変換系。moves / move_commands の seed SQL 生成)と本体ランタイム
// (repository/movecommand が move_commands テーブルから AddIndexed で再構築。M17-02 §2.3)。
// 段階2 解決サービス(inputresolve)と M17-04 他から引っ越しの共通基盤(「二度作らない」)。
package moveindex

import (
	"sort"
	"strings"
)

// SkipReason は行が索引に載らなかった理由を表す。
type SkipReason string

const (
	// SkipEmptyCommand は command 列が空欄だった行。
	SkipEmptyCommand SkipReason = "empty-command"
	// SkipDerived は派生技(is_derived=true)だった行。
	SkipDerived SkipReason = "derived"
	// SkipUnknownToken は未知トークン(raw{...} または §9.9 語彙外トークン)を含む行。
	SkipUnknownToken SkipReason = "unknown-token"
	// SkipConditionResidual は条件残留 cond{...} を含む行。
	SkipConditionResidual SkipReason = "condition-residual"
)

// entry は索引に載る 1 候補。
type entry struct {
	moveCode string
	id       int // タイブレーク用の安定順序キー(投入順の行 index または moves.id)
}

// Skipped は索引非搭載として記録された行。
type Skipped struct {
	CharKey  string
	MoveCode string
	Command  string
	Reason   SkipReason
}

// Entry は索引に載った 1 候補の外部公開形(seed 生成の列挙用。Entries が返す)。
type Entry struct {
	CharKey  string
	TokenKey string // 正規化済み索引キー(numpad 形)
	MoveCode string
	ID       int
}

// Index はキャラ別の「正規化済み索引キー → move_code」索引。
// ゼロ値は使用不可。New で生成する。
type Index struct {
	// charKey -> token -> []entry(id 昇順)
	m       map[string]map[string][]entry
	skipped []Skipped
}

// New は空の索引を返す。
func New() *Index {
	return &Index{m: make(map[string]map[string][]entry)}
}

// ---- numpad 正規化層(M17-02 §4.2・M14-03b が確保した後付け位置) ----

// tokenFragments は TOOL-002 §9.9 トークン → numpad 断片の写像(語彙の再定義はしない)。
// 方向はテンキー数字(右向き基準)、ボタンは強度別の慣用表記。plus は写像せず連結規則で表現する。
// テキスト由来トークンのうち opt_open/opt_close(オプション入力の括弧)は意図的に写像しない
// (現 CSV 出現 0 件・出現時は語彙外として索引非搭載+記録=fail しない。写像方針は第二波 seed で確定)。
var tokenFragments = map[string]string{
	// 方向(u/d/l/r と斜め・ニュートラル)
	"u": "8", "d": "2", "l": "4", "r": "6",
	"ul": "7", "ur": "9", "dl": "1", "dr": "3",
	"n": "5",
	// ため・一回転
	"charge_d": "[2]", "charge_l": "[4]", "charge_r": "[6]",
	"circle": "360",
	// 接続子
	"or": "/", "chain": ">", "alt_sep": "|", "hold": "(hold)",
	// ボタン(任意強度/弱/中/強)
	"p": "P", "p_l": "LP", "p_m": "MP", "p_h": "HP",
	"k": "K", "k_l": "LK", "k_m": "MK", "k_h": "HK",
}

// buttonFragments はボタン断片(隣接時に '+' を挿入する対象)。
var buttonFragments = map[string]bool{
	"P": true, "LP": true, "MP": true, "HP": true,
	"K": true, "LK": true, "MK": true, "HK": true,
}

// normalizeCommand はトークン列(空白区切り)を numpad 索引キーへ正規化する。
// plus トークンは削除し(方向→ボタンは単純連結)、ボタン断片同士が隣接する場合のみ '+' を
// 挿入する(同時押し。plus の有無に依らず表現が一意になる)。
// 語彙外トークンは verbatim 通過しつつ unknown として返す(Add では索引非搭載として記録・
// クエリでは自然ミスとなる防御的通過)。
//
// 例: "d plus p_m"→"2MP" / "d dr r plus p_l"→"236LP" / "charge_l r plus p_l"→"[4]6LP" /
// "circle plus p"→"360P" / "n or r plus p_l k_l"→"5/6LP+LK" / "p_m chain p_h"→"MP>HP"。
func normalizeCommand(command string) (key string, unknown []string) {
	var b strings.Builder
	prevButton := false
	for tok := range strings.FieldsSeq(command) {
		if tok == "plus" {
			continue
		}
		frag, ok := tokenFragments[tok]
		if !ok {
			unknown = append(unknown, tok)
			frag = tok
		}
		if prevButton && buttonFragments[frag] {
			b.WriteString("+")
		}
		b.WriteString(frag)
		prevButton = buttonFragments[frag]
	}
	return b.String(), unknown
}

// normalizeToken は正規化キーのみを返すクエリ用ヘルパ(語彙外は verbatim 通過)。
func normalizeToken(command string) string {
	key, _ := normalizeCommand(command)
	return key
}

// Add は 1 行を索引へ追加する。索引条件を満たさない行は記録のみ行い載せない(fail しない)。
// id は同一トークンに複数該当が残った際の move.id 最小タイブレーク用の安定順序キーで、
// 投入順(CSV 行 index 等)を渡すこと。
func (ix *Index) Add(charKey, moveCode, command string, isDerived bool, id int) {
	if reason, ok := skipReasonFor(command, isDerived); ok {
		ix.skipped = append(ix.skipped, Skipped{
			CharKey:  charKey,
			MoveCode: moveCode,
			Command:  command,
			Reason:   reason,
		})
		return
	}
	token, unknown := normalizeCommand(command)
	if len(unknown) > 0 {
		// 語彙外トークン(正規化層の導入で raw{...} 以外も機械検出できるようになった。M17-02 §4.1)。
		ix.skipped = append(ix.skipped, Skipped{
			CharKey:  charKey,
			MoveCode: moveCode,
			Command:  command,
			Reason:   SkipUnknownToken,
		})
		return
	}
	ix.insert(charKey, moveCode, token, id)
}

// AddIndexed は正規化済みの索引キーを直接投入する(move_commands テーブルからの
// ランタイム再構築用。正規化・skip 判定は通さない。M17-02 §2.3)。
// id には moves.id を渡すこと(Lookup の id 最小タイブレークが実 id で決まる)。
func (ix *Index) AddIndexed(charKey, moveCode, tokenKey string, id int) {
	ix.insert(charKey, moveCode, tokenKey, id)
}

// insert は正規化済みキーで 1 候補を索引へ載せる。
func (ix *Index) insert(charKey, moveCode, token string, id int) {
	byToken := ix.m[charKey]
	if byToken == nil {
		byToken = make(map[string][]entry)
		ix.m[charKey] = byToken
	}
	list := byToken[token]
	list = append(list, entry{moveCode: moveCode, id: id})
	// id 昇順を維持(Lookup の最小タイブレーク・LookupAll の決定論順序)。
	sort.SliceStable(list, func(i, j int) bool { return list[i].id < list[j].id })
	byToken[token] = list
}

// skipReasonFor は索引非搭載理由を判定する。載せるなら ok=false。
// raw{/cond{ はトークン分割前の部分文字列判定を維持する(cond{...} 内に空白が入り得るため。
// 語彙外トークンの検出は正規化後に Add 側で行う二段構え。M17-02 §4.1)。
func skipReasonFor(command string, isDerived bool) (SkipReason, bool) {
	if isDerived {
		return SkipDerived, true
	}
	if strings.TrimSpace(command) == "" {
		return SkipEmptyCommand, true
	}
	if strings.Contains(command, "raw{") {
		return SkipUnknownToken, true
	}
	if strings.Contains(command, "cond{") {
		return SkipConditionResidual, true
	}
	return "", false
}

// Lookup は charKey/token に対応する move_code を単一返却する。
// token は正準トークン列(例 "d plus p_m")・正規化済みキー(例 "2MP")のどちらでもよい
// (正規化は語彙外断片を verbatim 通過するため冪等)。
// 複数該当が残る場合は move.id 最小(投入順が最も若い)を返す。該当なしは ok=false。
func (ix *Index) Lookup(charKey, token string) (moveCode string, ok bool) {
	list := ix.m[charKey][normalizeToken(token)]
	if len(list) == 0 {
		return "", false
	}
	return list[0].moveCode, true // insert で id 昇順ソート済み
}

// LookupAll は charKey/token に対応する move_code 候補を id 昇順で返す(候補一覧・デバッグ用)。
func (ix *Index) LookupAll(charKey, token string) []string {
	list := ix.m[charKey][normalizeToken(token)]
	if len(list) == 0 {
		return nil
	}
	out := make([]string, len(list))
	for i, e := range list {
		out[i] = e.moveCode
	}
	return out
}

// Entries は索引に載った全候補を決定論順序(CharKey→TokenKey→ID 昇順)で返す
// (seedgen の move_commands seed 生成用。M17-02 §4.1)。
func (ix *Index) Entries() []Entry {
	var out []Entry
	for charKey, byToken := range ix.m {
		for token, list := range byToken {
			for _, e := range list {
				out = append(out, Entry{CharKey: charKey, TokenKey: token, MoveCode: e.moveCode, ID: e.id})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.CharKey != b.CharKey {
			return a.CharKey < b.CharKey
		}
		if a.TokenKey != b.TokenKey {
			return a.TokenKey < b.TokenKey
		}
		return a.ID < b.ID
	})
	return out
}

// Skipped は索引非搭載として記録された行の一覧を返す(記録順)。
func (ix *Index) Skipped() []Skipped {
	return ix.skipped
}

// CharKeys は索引に載ったキャラキー一覧を返す(昇順)。
func (ix *Index) CharKeys() []string {
	keys := make([]string, 0, len(ix.m))
	for k := range ix.m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
