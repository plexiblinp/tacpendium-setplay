// Package engine はセットプレイ提案の公開の入口である(単体版だけのパッケージ)。
//
// 計算そのものは本家から移植した internal/service/setplay(と internal/repository/setplay)が行う。
// 本パッケージは本家の API 層(internal/api/setplay の handler ・dto)に当たる部分を持つ:
// moves DB(data/moves/<code>.json)の読込、クエリの解釈(ParseQuery)、結果の DTO(Response)。
//
// 用語: KA ＝ knockdown advantage(ダウン後の有利フレーム。本サービスの主入力)。変数名 ka ・KAs もこの意味。
// 本家のコード(internal/service/setplay)と同じ略し方に揃えている(本家の差分を当てやすくするため。AGENTS §0)。
//
// ★標準ライブラリだけに依存し、純関数・決定的である(AGENTS §4)。WASM(cmd/wasm)と
// 事前計算(S4)の両方がここを通す。
// ★KA の上限(サイト 100 ＝ HANDOVER D10)はここでは掛けない。ローカル版は無制限のため、
// 上限は呼び出し側が SiteMaxKnockdownAdvantage で掛ける。
package engine

import (
	"context"
	"fmt"

	setplayrepo "github.com/plexiblinp/tacpendium-setplay/internal/repository/setplay"
	"github.com/plexiblinp/tacpendium-setplay/internal/service/setplay"
)

// SiteMaxKnockdownAdvantage は公開サイトで受け付ける KA の上限(HANDOVER D10)。エンジン自身は使わない。
const SiteMaxKnockdownAdvantage = 100

// Engine は読み込んだキャラと提案のサービスを持つ。ゼロ値ではなく New で作ること。
// 並行して呼ばないこと(WASM は単一スレッド。事前計算はキャラごとに Engine を作る想定)。
type Engine struct {
	store *setplayrepo.Store
	svc   setplay.Service
}

// New は空の Engine を返す。
func New() *Engine {
	e := &Engine{store: setplayrepo.NewStore()}
	e.svc = setplay.NewService(e.store)
	return e
}

// Load は moves DB 1 キャラ分の JSON を読み込み、キャラの code を返す。同じキャラは置き換える。
func (e *Engine) Load(data []byte) (string, error) {
	c, err := setplayrepo.ParseCharacter(data)
	if err != nil {
		return "", fmt.Errorf("load character: %w", err)
	}
	e.store.Add(c)
	return c.Code, nil
}

// SafeJump は安全飛びの値(本家 characters.custom_states の safe_jump。どれも「繋ぎの後に残すフレーム」で、
// mode=filler の remain に渡す)。キャラごとに違う。
type SafeJump struct {
	True int `json:"true"`
	F5   int `json:"f5"`
	F6   int `json:"f6"`
}

// SafeJump は読み込んだキャラの安全飛びの値を返す。moves DB に値が無いキャラは nil。
func (e *Engine) SafeJump(characterCode string) (*SafeJump, error) {
	c, err := e.store.Character(characterCode)
	if err != nil {
		return nil, err
	}
	if c.SafeJump == nil {
		return nil, nil
	}
	return &SafeJump{True: c.SafeJump.True, F5: c.SafeJump.F5, F6: c.SafeJump.F6}, nil
}

// Step は提案 1 ステップ(本家の SuggestionStepDTO から moveId を除いたもの。技は code で指す)。
type Step struct {
	Code    string `json:"code"`
	Role    string `json:"role"` // "filler" | "target" | "parent"
	Counted bool   `json:"counted"`
}

// Item は提案 1 件(本家の SuggestionDTO から alreadyAdopted を除いたもの)。
type Item struct {
	Steps        []Step `json:"steps"`
	S            int    `json:"s"`
	N            int    `json:"n"`
	Landing      int    `json:"landing"`
	TargetActive int    `json:"targetActive"`
	Mode         string `json:"mode"`
	G            int    `json:"g"`
	Remain       int    `json:"remain"`
	// RecipeKey はレシピの同一性の鍵(単体版だけ。S6・HANDOVER D17)。画面はこの鍵で票を送り、票を引く。
	RecipeKey string `json:"recipeKey"`
}

// Response は提案の結果(本家の SuggestionsResponse と同じ形)。
type Response struct {
	Items      []Item `json:"items"`
	Truncated  bool   `json:"truncated"`
	TotalFound int    `json:"totalFound"`
	Reason     string `json:"reason,omitempty"`
}

// noSuchMoveID は target_move がキャラに無いときに渡す ID。moves DB の ID は 1 から振るので、どの技にも当たらない。
// ★本家は他キャラの技の target_move_id を受けたとき、誤りにせず 0 件を返す。それと揃える。
const noSuchMoveID int64 = 0

// Suggest はキャラ characterCode ・KA(ダウン後の有利フレーム)ka ・クエリ q でセットプレイ提案を返す。
// ctx の取り消しは計算の内側で見る(本家と同じ。WASM では Worker ごと止めるので使わない)。
func (e *Engine) Suggest(ctx context.Context, characterCode string, ka int, q Query) (*Response, error) {
	c, err := e.store.Character(characterCode)
	if err != nil {
		return nil, err
	}
	params := q.Params
	if q.TargetMove != "" {
		id := noSuchMoveID
		for _, m := range c.Candidates {
			if m.Code == q.TargetMove {
				id = m.ID
				break
			}
		}
		params.TargetMoveID = &id
	}
	res, err := e.svc.SuggestForKnockdown(ctx, characterCode, ka, params)
	if err != nil {
		return nil, fmt.Errorf("suggest: %w", err)
	}
	return toResponse(res), nil
}

// toResponse はサービスの結果を DTO へ変換する(本家 dto.go の toResponse に当たる)。
func toResponse(res *setplay.SuggestResult) *Response {
	items := make([]Item, len(res.Proposals))
	for i, p := range res.Proposals {
		steps := make([]Step, len(p.Steps))
		for j, st := range p.Steps {
			steps[j] = Step{Code: st.Code, Role: st.Role, Counted: st.Counted}
		}
		items[i] = Item{
			Steps:        steps,
			S:            p.S,
			N:            p.N,
			Landing:      p.Landing,
			TargetActive: p.TargetActive,
			Mode:         p.Mode,
			G:            p.G,
			Remain:       p.Remain,
			RecipeKey:    setplay.RecipeKey(p.Steps),
		}
	}
	return &Response{
		Items:      items,
		Truncated:  res.Truncated,
		TotalFound: res.TotalFound,
		Reason:     res.Reason,
	}
}
