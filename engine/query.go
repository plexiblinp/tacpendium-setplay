package engine

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/plexiblinp/tacpendium-setplay/internal/service/setplay"
)

// クエリのキー。本家の GET /api/combos/:comboId/setplay-suggestions と同じ名前である
// (internal/api/setplay の handler.go。単体版で変えたのは target_move_id → QueryTargetMove だけ)。
const (
	QueryNMin            = "n_min"
	QueryNMax            = "n_max"
	QuerySort            = "sort"
	QueryTargetTypes     = "target_types"
	QueryIncludeZeroDmg  = "include_zero_damage"
	QueryApproachOnly    = "approach_only"
	QueryBlockSafeOnly   = "block_safe_only"
	QueryChainCancelOnly = "chain_cancel_only"
	// QueryTargetMove は当てる技の明示指定(技の code)。本家の target_move_id(数値 ID)に当たる。
	// moves DB は数値 ID を持たないので code で指す。
	QueryTargetMove = "target_move"
	QueryMode       = "mode"
	QueryRemain     = "remain"
	QueryGMin       = "g_min"
	QueryGMax       = "g_max"
	QueryLimit      = "limit"
)

// ErrCodeInvalidQuery は QueryError の Code(本家の API 誤りの code と同じ値)。
const ErrCodeInvalidQuery = "invalid_query_parameter"

// QueryError はクエリの誤り(本家ハンドラが 400 invalid_query_parameter を返す場合に当たる)。
type QueryError struct {
	// Param は誤ったキー。
	Param string
	// Message は本家ハンドラと同じ文言(日本語)。画面に出す文言は S5 で i18n のキーに当てる。
	Message string
}

func (e *QueryError) Error() string {
	return fmt.Sprintf("%s: %s: %s", ErrCodeInvalidQuery, e.Param, e.Message)
}

// Query は解釈したクエリ。Params は本家の SuggestParams そのもの(TargetMoveID だけは Suggest がキャラを見て埋める)。
type Query struct {
	Params setplay.SuggestParams
	// TargetMove は当てる技の code(QueryTargetMove)。空なら自動列挙。
	TargetMove string
}

// DefaultTargetTypes は target_types 省略時の既定(通常技・特殊技・必殺技(弾))。本家の defaultTargetTypes と同じ。
func DefaultTargetTypes() []string {
	return []string{
		setplay.TargetTypeNormal,
		setplay.TargetTypeUnique,
		setplay.TargetTypeSpecialProjectile,
	}
}

// ParseQuery は本家ハンドラ(GetSuggestions)と同じ規則でクエリを解釈する。
//
//   - 整数・真偽値の形だけを検査し、範囲の丸めはサービスが行う(本家と同じ。n_min・g_min/g_max・limit)。
//   - mode は省略時 meaty。未知の値は誤り。mode=filler のとき remain は必須・0 以上。
//   - sort は省略時 n。target_types は省略時 DefaultTargetTypes。未知の種別は誤りにせず、どれにも当たらないだけ。
//
// 本家と同じく、検査の順は n_min → sort → target_types → include_zero_damage → 絞り込み 3 つ →
// target_move → mode → remain → n_max/g_min/g_max/limit。最初の誤りを返す。
func ParseQuery(q url.Values) (Query, error) {
	var out Query
	p := &out.Params

	if v := q.Get(QueryNMin); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return Query{}, &QueryError{Param: QueryNMin, Message: "n_min は整数"}
		}
		p.NMin = n
	}

	switch setplay.SuggestSort(q.Get(QuerySort)) {
	case "", setplay.SortByN:
		p.Sort = setplay.SortByN
	case setplay.SortByTarget:
		p.Sort = setplay.SortByTarget
	default:
		return Query{}, &QueryError{Param: QuerySort, Message: "sort は n または target"}
	}

	if v := q.Get(QueryTargetTypes); v != "" {
		p.TargetTypes = splitCSV(v)
	} else {
		p.TargetTypes = DefaultTargetTypes()
	}

	bools := []struct {
		key string
		dst *bool
	}{
		{QueryIncludeZeroDmg, &p.IncludeZeroDamage},
		{QueryApproachOnly, &p.ApproachOnly},
		{QueryBlockSafeOnly, &p.BlockSafeOnly},
		{QueryChainCancelOnly, &p.ChainCancelOnly},
	}
	for _, b := range bools {
		if v := q.Get(b.key); v != "" {
			x, err := strconv.ParseBool(v)
			if err != nil {
				return Query{}, &QueryError{Param: b.key, Message: b.key + " は真偽値"}
			}
			*b.dst = x
		}
	}

	out.TargetMove = q.Get(QueryTargetMove)

	switch v := q.Get(QueryMode); v {
	case "", setplay.ModeMeaty:
		p.Mode = setplay.ModeMeaty
	case setplay.ModeGap:
		p.Mode = setplay.ModeGap
	case setplay.ModeFiller:
		p.Mode = setplay.ModeFiller
	default:
		return Query{}, &QueryError{Param: QueryMode, Message: "mode は meaty / gap / filler のいずれか"}
	}

	if p.Mode == setplay.ModeFiller {
		v := q.Get(QueryRemain)
		n, err := strconv.Atoi(v)
		if v == "" || err != nil || n < 0 {
			return Query{}, &QueryError{Param: QueryRemain, Message: "remain は 0 以上の整数(mode=filler のとき必須)"}
		}
		p.Remain = n
	}

	ints := []struct {
		key string
		dst *int
	}{
		{QueryNMax, &p.NMax},
		{QueryGMin, &p.GMin},
		{QueryGMax, &p.GMax},
		{QueryLimit, &p.Limit},
	}
	for _, it := range ints {
		if v := q.Get(it.key); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil {
				return Query{}, &QueryError{Param: it.key, Message: it.key + " は整数"}
			}
			*it.dst = n
		}
	}
	return out, nil
}

// splitCSV はカンマ区切り文字列を trim 済み・空要素除去のスライスにする(本家と同じ)。
func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
