// Package setplay(repository) はセットプレイ自動提案(M19-01)のための moves 読み取りを提供する。
//
// 提案の target 列挙(§4.2)には moves.is_derived が必要である。本リポジトリは提案生成のため
// BE 内で is_derived / is_projectile を参照する。
//
// ★★【2026-09-10 更新・M30-04】is_derived は「model.Move / GET /api/moves に露出していない
// DB 専用列」ではなくなった —— 必殺技ファミリー行の並び順に使うため、リスト DTO へ
// 露出した(model.Move.IsDerived / MoveResponse.isDerived)。
// ⇒ 本リポジトリが専用投影を持つ理由は「API 非露出だから」ではなく、提案生成が必要とする列
// (is_projectile / startup_basis / chain_cancel_total / fastest_unreachable)が
// リスト DTO に無いことである。is_projectile は引き続き API 非露出。
package setplay

// 単体版: 本家は moves を SQLite から SQL 3 本で読む。単体版は moves DB(data/moves/<code>.json。
// 形の正本は internal/movesdb/model.go)を読み、本家と同じ型(MoveCandidate ・MoveDerivation)に直す。
// ★本パッケージは標準ライブラリだけに依存する(WASM と事前計算の両方で使うため)。
// ★数値 ID は moves DB に無い。配列の位置 + 1 を ID にする。配列は本家の moves.id 昇順なので、
//   本家の「ID 昇順」の並び(listCandidatesSQL の ORDER BY m.id 等)はそのまま保たれる。
//   ID の値そのものは本家と違うので、外へ出す識別子には code を使うこと。

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
)

// MoveCandidate は提案生成に必要な moves の 1 行。
// フレーム列(Startup/Active/Total/Damage)は NULL 許容のためポインタ。
type MoveCandidate struct {
	ID             int64
	Code           string
	Category       string
	OriginalMoveID *int64 // rush_variant の元技(通常技/特殊技)を指す。rush 種別の判定に使う
	Startup        *int
	Active         *int
	Total          *int
	Damage         *int
	IsDerived      bool
	IsAerial       bool
	IsProjectile   bool
	SetupOnly      bool

	// StartupBasis は Startup の由来(model.MoveStartupBasis*)。DB では NOT NULL・既定 unknown。
	StartupBasis string
	// ChainCancelTotal はチェーン時の実消費フレーム(実測値)。
	// 非 NULL であること自体がチェーングループ員であることを兼ねる(DES-003 §3.3)。
	ChainCancelTotal *int
	// FastestUnreachable は「単独で最速入力しても地上の相手に当てられない」技(B/C 型)。
	// true = スカラー S の target にできない。
	FastestUnreachable bool
	// OnBlock はガード時の有利フレーム(符号付き・公式が空欄なら NULL＝DES-003 §3.3)。
	// 持続の 1F 目を当てたときの値である。提案の絞り込み「ガード時 0F 以上だけ」が
	// on_block + N − 1 ≥ 0 の判定に使う(M47-05・CHANGE-254 b)。
	OnBlock *int
}

// MoveDerivation は move_derivations の 1 行(表記用親の参照)。
type MoveDerivation struct {
	ChildMoveID  int64
	ParentMoveID int64
}

// SchemaVersion は読める moves DB の形の版(internal/movesdb.SchemaVersion と同じ値であること。
// 一致はテスト internal/movesdb の TestSchemaVersionMatchesEngineReader が見る)。
const SchemaVersion = 1

// SafeJump は安全飛びの値(本家 characters.custom_states の safe_jump。値は「繋ぎの後に残すフレーム」)。
// 本家ではサービスの外(画面)が読む値であり、計算は ModeFiller の Remain として渡す。
type SafeJump struct {
	True int `json:"true"`
	F5   int `json:"f5"`
	F6   int `json:"f6"`
}

// Character は moves DB 1 キャラ分を、提案生成が読む形に直したもの。
type Character struct {
	// Code はキャラの code(例 "ryu")。
	Code string
	// DataVersion は moves DB の取込元の版(公開版 tacpendium の SHA)。
	DataVersion string
	// SafeJump は安全飛びの値。moves DB に無ければ nil。
	SafeJump *SafeJump
	// CalcFingerprint は moves DB のキャラ単位の計算用の指紋(事前計算 CLI が空でないことを確かめる。S4)。
	CalcFingerprint string
	// Candidates は本家の ListCharacterMoveCandidates と同じく ID 昇順の全技。
	Candidates []MoveCandidate
	// Derivations は本家の listDerivationsSQL と同じく (子 ID, 親 ID) 昇順の親子関係。
	Derivations []MoveDerivation
}

// movesFile は moves DB のうち提案生成が読む列だけを受ける。表記・指紋などは読まない。
type movesFile struct {
	SchemaVersion int    `json:"schemaVersion"`
	DataVersion   string `json:"dataVersion"`
	Character     struct {
		Code string `json:"code"`
	} `json:"character"`
	SafeJump        *SafeJump  `json:"safeJump"`
	CalcFingerprint string     `json:"calcFingerprint"`
	Moves           []movesRow `json:"moves"`
}

type movesRow struct {
	Code               string   `json:"code"`
	Category           string   `json:"category"`
	Startup            *int     `json:"startup"`
	Active             *int     `json:"active"`
	Total              *int     `json:"total"`
	OnBlock            *int     `json:"onBlock"`
	Damage             *int     `json:"damage"`
	IsDerived          bool     `json:"isDerived"`
	IsAerial           bool     `json:"isAerial"`
	IsProjectile       bool     `json:"isProjectile"`
	SetupOnly          bool     `json:"setupOnly"`
	StartupBasis       string   `json:"startupBasis"`
	ChainCancelTotal   *int     `json:"chainCancelTotal"`
	FastestUnreachable bool     `json:"fastestUnreachable"`
	OriginalMove       *string  `json:"originalMove"`
	Parents            []string `json:"parents"`
}

// ParseCharacter は moves DB 1 キャラ分の JSON を読む。
//
// 形の誤り(版違い ・キャラ code が空 ・技 code が空か重複 ・startupBasis が空 ・元技や親が同じキャラに無い)は
// 誤りにする。★本家の repository は親が解決できない行を黙って飛ばすが、moves DB は取込時に
// 同じ検査を済ませている(internal/movesdb)ので、ここで見つかるのは壊れたファイルである。
func ParseCharacter(data []byte) (*Character, error) {
	var f movesFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse moves db: %w", err)
	}
	if f.SchemaVersion != SchemaVersion {
		return nil, fmt.Errorf("moves db schemaVersion %d: want %d", f.SchemaVersion, SchemaVersion)
	}
	if f.Character.Code == "" {
		return nil, fmt.Errorf("moves db: character.code is empty")
	}
	idByCode := make(map[string]int64, len(f.Moves))
	for i, m := range f.Moves {
		if m.Code == "" {
			return nil, fmt.Errorf("moves db %s: moves[%d].code is empty", f.Character.Code, i)
		}
		if _, dup := idByCode[m.Code]; dup {
			return nil, fmt.Errorf("moves db %s: duplicate move code %q", f.Character.Code, m.Code)
		}
		idByCode[m.Code] = int64(i + 1)
	}
	c := &Character{
		Code:            f.Character.Code,
		DataVersion:     f.DataVersion,
		SafeJump:        f.SafeJump,
		CalcFingerprint: f.CalcFingerprint,
		Candidates:      make([]MoveCandidate, 0, len(f.Moves)),
		Derivations:     []MoveDerivation{},
	}
	for i, m := range f.Moves {
		if m.StartupBasis == "" {
			return nil, fmt.Errorf("moves db %s: %s: startupBasis is empty", c.Code, m.Code)
		}
		mc := MoveCandidate{
			ID:                 int64(i + 1),
			Code:               m.Code,
			Category:           m.Category,
			Startup:            m.Startup,
			Active:             m.Active,
			Total:              m.Total,
			Damage:             m.Damage,
			IsDerived:          m.IsDerived,
			IsAerial:           m.IsAerial,
			IsProjectile:       m.IsProjectile,
			SetupOnly:          m.SetupOnly,
			StartupBasis:       m.StartupBasis,
			ChainCancelTotal:   m.ChainCancelTotal,
			FastestUnreachable: m.FastestUnreachable,
			OnBlock:            m.OnBlock,
		}
		if m.OriginalMove != nil {
			id, ok := idByCode[*m.OriginalMove]
			if !ok {
				return nil, fmt.Errorf("moves db %s: %s: originalMove %q not found", c.Code, m.Code, *m.OriginalMove)
			}
			mc.OriginalMoveID = &id
		}
		parentIDs := make([]int64, 0, len(m.Parents))
		for _, pc := range m.Parents {
			id, ok := idByCode[pc]
			if !ok {
				return nil, fmt.Errorf("moves db %s: %s: parent %q not found", c.Code, m.Code, pc)
			}
			parentIDs = append(parentIDs, id)
		}
		// 本家の listDerivationsSQL は ORDER BY child_move_id, parent_move_id。moves DB の parents は code 昇順なので ID で並べ直す。
		sort.Slice(parentIDs, func(a, b int) bool { return parentIDs[a] < parentIDs[b] })
		for _, pid := range parentIDs {
			c.Derivations = append(c.Derivations, MoveDerivation{ChildMoveID: mc.ID, ParentMoveID: pid})
		}
		c.Candidates = append(c.Candidates, mc)
	}
	return c, nil
}

// Store は読み込んだキャラを code で引く(本家の Repository に当たる。service/setplay の MoveCandidateReader を満たす)。
// 並行して読み書きしないこと(WASM は単一スレッド、事前計算はキャラごとに作る想定)。
type Store struct {
	chars map[string]*Character
}

// NewStore は空の Store を返す。
func NewStore() *Store { return &Store{chars: map[string]*Character{}} }

// Add はキャラを登録する。同じ code は置き換える。
func (s *Store) Add(c *Character) { s.chars[c.Code] = c }

// Character は code のキャラを返す。無ければ ErrCharacterNotFound を wrap して返す。
func (s *Store) Character(code string) (*Character, error) {
	c, ok := s.chars[code]
	if !ok {
		return nil, fmt.Errorf("character %q: %w", code, ErrCharacterNotFound)
	}
	return c, nil
}

// ErrCharacterNotFound は Store に無いキャラを指したときの誤り。
var ErrCharacterNotFound = errors.New("setplay: character not loaded")

// ListCharacterMoveCandidates は characterCode のキャラの全技を ID 昇順で返す(本家の同名メソッドに当たる)。
func (s *Store) ListCharacterMoveCandidates(ctx context.Context, characterCode string) ([]MoveCandidate, error) {
	c, err := s.Character(characterCode)
	if err != nil {
		return nil, err
	}
	return c.Candidates, nil
}

// ListCharacterMoveDerivations は characterCode のキャラの親子関係を返す(本家の同名メソッドに当たる)。
func (s *Store) ListCharacterMoveDerivations(ctx context.Context, characterCode string) ([]MoveDerivation, error) {
	c, err := s.Character(characterCode)
	if err != nil {
		return nil, err
	}
	return c.Derivations, nil
}
