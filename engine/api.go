package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"strconv"

	setplayrepo "github.com/plexiblinp/tacpendium-setplay/internal/repository/setplay"
	"github.com/plexiblinp/tacpendium-setplay/internal/service/setplay"
)

// JS(WASM)との境目で返す誤りの code。本家の API 誤りの code に揃えたもの以外は単体版だけ。
const (
	// ErrCodeCharacterNotLoaded は Load していないキャラを指したとき。
	ErrCodeCharacterNotLoaded = "character_not_loaded"
	// ErrCodeInvalidData は moves DB の JSON が読めないとき。
	ErrCodeInvalidData = "invalid_moves_data"
	// ErrCodeInvalidArguments は JS から渡された引数の数 ・型が違うとき(cmd/wasm が使う)。
	ErrCodeInvalidArguments = "invalid_arguments"
	// ErrCodeInternal はそれ以外(計算の誤り)。
	ErrCodeInternal = "internal_error"
)

// APIError は JSON で返す誤り。
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	// Param は誤ったクエリのキー(ErrCodeInvalidQuery のときだけ)。
	Param string `json:"param,omitempty"`
}

// LoadResult は LoadJSON の結果。
type LoadResult struct {
	Character string    `json:"character,omitempty"`
	Error     *APIError `json:"error,omitempty"`
}

// SuggestResult は SuggestJSON の結果。成功なら Result、失敗なら Error のどちらか一方だけが入る。
type SuggestResult struct {
	Result *Response `json:"result,omitempty"`
	Error  *APIError `json:"error,omitempty"`
}

// LoadJSON は Load を呼び、結果を JSON で返す(WASM の load の中身。誤りも JSON で返し、例外にしない)。
func (e *Engine) LoadJSON(data []byte) []byte {
	code, err := e.Load(data)
	if err != nil {
		return mustMarshal(LoadResult{Error: &APIError{Code: ErrCodeInvalidData, Message: err.Error()}})
	}
	return mustMarshal(LoadResult{Character: code})
}

// SuggestJSON はクエリ文字列(本家のクエリと同じ名前。ParseQuery)を解釈して Suggest を呼び、結果を JSON で返す
// (WASM の suggest の中身)。誤りも JSON で返す。
func (e *Engine) SuggestJSON(characterCode string, ka int, rawQuery string) []byte {
	return e.suggestJSON(characterCode, ka, rawQuery, nil)
}

// SuggestWithVotesJSON は SuggestJSON に票(S6・HANDOVER D17)を足したもの。votesJSON は ParseVotes の形
// ({"<RecipeKey>": good − bad, ...})。票の形が誤りなら invalid_arguments(票を黙って捨てると並びが変わったことに気づけない)。
func (e *Engine) SuggestWithVotesJSON(characterCode string, ka int, rawQuery, votesJSON string) []byte {
	votes, err := ParseVotes([]byte(votesJSON))
	if err != nil {
		return InvalidArgumentsJSON(err.Error())
	}
	return e.suggestJSON(characterCode, ka, rawQuery, votes)
}

func (e *Engine) suggestJSON(characterCode string, ka int, rawQuery string, votes map[string]int) []byte {
	v, err := url.ParseQuery(rawQuery)
	if err != nil {
		return mustMarshal(SuggestResult{Error: &APIError{Code: ErrCodeInvalidQuery, Message: err.Error()}})
	}
	q, err := ParseQuery(v)
	if err != nil {
		var qe *QueryError
		if errors.As(err, &qe) {
			return mustMarshal(SuggestResult{Error: &APIError{Code: ErrCodeInvalidQuery, Message: qe.Message, Param: qe.Param}})
		}
		return mustMarshal(SuggestResult{Error: &APIError{Code: ErrCodeInternal, Message: err.Error()}})
	}
	q.Params.Votes = votes
	res, err := e.Suggest(context.Background(), characterCode, ka, q)
	if err != nil {
		return mustMarshal(SuggestResult{Error: apiErrorOf(err)})
	}
	return mustMarshal(SuggestResult{Result: res})
}

// apiErrorOf は Suggest の誤りを JSON の誤りへ写す(本家ハンドラの写し方に揃える)。
// ★ErrInvalidRemain は本家では 400 invalid_query_parameter("remain は 0 以上の整数")。ParseQuery を通せば
// 起きないが、Query を直接組んで Suggest を呼ぶ経路(事前計算など)でも同じ扱いにする。
func apiErrorOf(err error) *APIError {
	switch {
	case errors.Is(err, setplayrepo.ErrCharacterNotFound):
		return &APIError{Code: ErrCodeCharacterNotLoaded, Message: err.Error()}
	case errors.Is(err, setplay.ErrInvalidRemain):
		return &APIError{Code: ErrCodeInvalidQuery, Message: "remain は 0 以上の整数", Param: QueryRemain}
	default:
		return &APIError{Code: ErrCodeInternal, Message: err.Error()}
	}
}

// 票の上限(S6)。1 キャラ ・1 KA の票の付いたレシピの数と、鍵の長さ。超えたら誤り(Worker の配信も同じ上限で切る)。
const (
	MaxVoteEntries  = 10000
	MaxRecipeKeyLen = 1024
	maxAbsVoteScore = 1 << 30
)

// ParseVotes は票の JSON({"<RecipeKey>": スコア(整数), ...})を読む。空の入力 ・null は票なし(nil)。
// 鍵が空 ・長すぎる、スコアが整数でない ・大きすぎる、件数が多すぎるときは誤り。
func ParseVotes(data []byte) (map[string]int, error) {
	if len(data) == 0 {
		return nil, nil
	}
	var raw map[string]any // json.Number で受けると "1"(文字列)も通るため any で受けて型を見る
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("votes: %w", err)
	}
	if dec.More() {
		return nil, errors.New("votes: JSON の後に余分な入力がある")
	}
	if len(raw) > MaxVoteEntries {
		return nil, fmt.Errorf("votes: %d 件 > 上限 %d", len(raw), MaxVoteEntries)
	}
	out := make(map[string]int, len(raw))
	for k, x := range raw {
		if k == "" || len(k) > MaxRecipeKeyLen {
			return nil, fmt.Errorf("votes: 鍵の長さ %d", len(k))
		}
		n, isNum := x.(json.Number)
		v, err := strconv.ParseInt(string(n), 10, 64)
		if !isNum || err != nil || v > maxAbsVoteScore || v < -maxAbsVoteScore {
			return nil, fmt.Errorf("votes: %q のスコア %v は整数でないか大きすぎる", k, x)
		}
		out[k] = int(v)
	}
	return out, nil
}

// KnockdownFromFloat は JS の数値を KA(整数)にする。NaN ・±Infinity ・小数 ・int32 の範囲外は false。
// ★黙って切り捨てると「不正な入力」が「0 件(該当なし)」や別の KA の結果に化ける(区別できない)ため、誤りにする。
func KnockdownFromFloat(f float64) (int, bool) {
	if math.IsNaN(f) || math.IsInf(f, 0) || f != math.Trunc(f) || f < math.MinInt32 || f > math.MaxInt32 {
		return 0, false
	}
	return int(f), true
}

// InvalidArgumentsJSON は引数の誤りを JSON で返す(load と suggest のどちらの形でも error だけが入る)。
func InvalidArgumentsJSON(message string) []byte {
	return mustMarshal(SuggestResult{Error: &APIError{Code: ErrCodeInvalidArguments, Message: message}})
}

// mustMarshal は本パッケージの型(文字列 ・数値 ・真偽値 ・スライスだけ)を JSON にする。
// これらの型の Marshal は失敗しないため、失敗はプログラムの誤りとして panic する。
func mustMarshal(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic("engine: marshal: " + err.Error())
	}
	return b
}
