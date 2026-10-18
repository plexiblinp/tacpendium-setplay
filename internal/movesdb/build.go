package movesdb

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// fingerprintHexLen は技単位の指紋の長さ(16 進の桁数)。
const fingerprintHexLen = 16

// CharacterReport は 1 キャラ分の生成の記録(報告・検査用。JSON には出さない)。
type CharacterReport struct {
	Code     string
	Numeric  *NotationResult
	SRK      *NotationResult
	NameENBy map[string]string // move_code → "rule" | "override"
}

// Build は Dataset と上書き表から、キャラ別の moves DB を作る(キャラは characters.id 昇順)。
func Build(ds *Dataset, ov *Overrides, dataVersion string) ([]CharacterFile, []CharacterReport, error) {
	var files []CharacterFile
	var reports []CharacterReport
	knownChars := map[string]bool{}
	for _, c := range ds.Characters {
		knownChars[c.Code] = true
	}
	for ch := range ov.NameEN {
		if !knownChars[ch] {
			return nil, nil, fmt.Errorf("%s: 未知のキャラ %q", NameENOverrideFile, ch)
		}
	}
	for ch := range ov.Notation {
		if !knownChars[ch] {
			return nil, nil, fmt.Errorf("%s: 未知のキャラ %q", NotationOverrideFile, ch)
		}
	}

	for _, c := range ds.Characters {
		f, r, err := buildCharacter(ds, ov, c, dataVersion)
		if err != nil {
			return nil, nil, fmt.Errorf("build %s: %w", c.Code, err)
		}
		files = append(files, f)
		reports = append(reports, r)
	}
	return files, reports, nil
}

func buildCharacter(ds *Dataset, ov *Overrides, c DBCharacter, dataVersion string) (CharacterFile, CharacterReport, error) {
	cs, err := parseCustomStates(c.CustomStates)
	if err != nil {
		return CharacterFile{}, CharacterReport{}, err
	}
	states := make([]State, 0, len(cs.States))
	for _, s := range cs.States {
		states = append(states, State{Code: s.Code, Name: LocalizedName{JA: s.NameJA, EN: s.NameEN}})
	}
	var sj *SafeJump
	if cs.SafeJump != nil {
		sj = &SafeJump{True: cs.SafeJump.True, F5: cs.SafeJump.F5, F6: cs.SafeJump.F6}
	}

	moves := ds.Moves[c.ID]
	// ★本家のサービス層は、同じキャラに見つからない親を黙って飛ばす(service.go の表記展開)。
	// 単体版は取込の時点で止める(上流のデータの変化に気づくため)。挙動の違いは docs/data/moves-db.md。
	codeByID := map[int64]string{}
	known := map[string]bool{}
	for _, m := range moves {
		codeByID[m.ID] = m.Code
		known[m.Code] = true
	}
	for code := range ov.NameEN[c.Code] {
		if !known[code] {
			return CharacterFile{}, CharacterReport{}, fmt.Errorf("%s: 未知の技 %s/%s", NameENOverrideFile, c.Code, code)
		}
	}
	for code := range ov.Notation[c.Code] {
		if !known[code] {
			return CharacterFile{}, CharacterReport{}, fmt.Errorf("%s: 未知の技 %s/%s", NotationOverrideFile, c.Code, code)
		}
	}

	// 日本語技名: CSV の name_ja。CSV に無い技(移動系・drive_reversal・連打版)は本家 DB の
	// official_ja_move(公式表記・日本語技名)の表記。
	csvName := map[string]string{}
	for _, r := range ds.CSV[c.Code] {
		csvName[r.MoveCode] = r.NameJA
	}
	names := map[string]Notation{}
	nameENBy := map[string]string{}
	stateByMove := map[string][]State{} // 技が前提とする状態(0 か 1 件)
	for _, m := range moves {
		ja := csvName[m.Code]
		if ja == "" {
			ja = ds.Aliases[PresetOfficialJA][m.ID].Text
		}
		ms := moveStates(m.Code, ja, states)
		stateByMove[m.Code] = ms
		en := GenerateNameEN(m.Code, ms)
		nameENBy[m.Code] = "rule"
		if o, ok := ov.NameEN[c.Code][m.Code]; ok {
			en = o.NameEN
			nameENBy[m.Code] = "override"
		} else if m.OriginalMoveID != nil && m.Category == "rush_variant" {
			// ラッシュ版は元技の上書きを引き継ぐ(元技 code の前に rush_ が付く形のときだけ)。
			if oc := codeByID[*m.OriginalMoveID]; m.Code == "rush_"+oc {
				if o, ok := ov.NameEN[c.Code][oc]; ok {
					en = rushWord + " " + o.NameEN
					nameENBy[m.Code] = "override-of-original"
				}
			}
		}
		names[m.Code] = Notation{NameJA: ja, NameEN: en}
	}

	num, err := buildNotation(kindNumeric, c, stateByMove, moves, ds.CSV[c.Code], ds.Aliases[PresetNumeric], names, ov.Notation[c.Code])
	if err != nil {
		return CharacterFile{}, CharacterReport{}, err
	}
	srk, err := buildNotation(kindSRK, c, stateByMove, moves, ds.CSV[c.Code], ds.Aliases[PresetSRK], names, ov.Notation[c.Code])
	if err != nil {
		return CharacterFile{}, CharacterReport{}, err
	}

	f := CharacterFile{
		SchemaVersion: SchemaVersion,
		DataVersion:   dataVersion,
		Character:     CharacterRef{Code: c.Code, Name: LocalizedName{JA: c.NameJA, EN: c.NameEN}},
		States:        states,
		SafeJump:      sj,
		Moves:         make([]Move, 0, len(moves)),
	}
	for _, m := range moves {
		var orig *string
		if m.OriginalMoveID != nil {
			code, ok := codeByID[*m.OriginalMoveID]
			if !ok {
				return CharacterFile{}, CharacterReport{}, fmt.Errorf("%s: original_move_id %d が同じキャラに無い", m.Code, *m.OriginalMoveID)
			}
			orig = &code
		}
		parents := []string{}
		for _, pid := range ds.Parents[m.ID] {
			code, ok := codeByID[pid]
			if !ok {
				return CharacterFile{}, CharacterReport{}, fmt.Errorf("%s: 親 %d が同じキャラに無い", m.Code, pid)
			}
			parents = append(parents, code)
		}
		// 本家のサービス層は親が複数のとき move_code 昇順の先頭を採る(repository の注記)。並びを固定する。
		sort.Strings(parents)
		mv := Move{
			Code: m.Code, Category: m.Category,
			Startup: m.Startup, Active: m.Active, Recovery: m.Recovery, Total: m.Total,
			OnHit: m.OnHit, OnBlock: m.OnBlock, Damage: m.Damage,
			IsDerived: m.IsDerived, IsAerial: m.IsAerial, IsProjectile: m.IsProjectile, SetupOnly: m.SetupOnly,
			StartupBasis: m.StartupBasis, ChainCancelTotal: m.ChainCancelTotal, FastestUnreachable: m.FastestUnreachable,
			OriginalMove: orig, Parents: parents, State: stateOf(m.Code, stateByMove[m.Code]),
			Notation: Notation{
				NameJA: names[m.Code].NameJA, NameEN: names[m.Code].NameEN,
				SRK: srk.Text[m.Code], Numeric: num.Text[m.Code],
			},
		}
		fp, err := moveFingerprint(mv)
		if err != nil {
			return CharacterFile{}, CharacterReport{}, err
		}
		mv.CalcFingerprint = fp
		f.Moves = append(f.Moves, mv)
	}
	f.CalcFingerprint, err = characterFingerprint(f)
	if err != nil {
		return CharacterFile{}, CharacterReport{}, err
	}
	return f, CharacterReport{Code: c.Code, Numeric: num, SRK: srk, NameENBy: nameENBy}, nil
}

// calcFields は指紋に入れる列(計算に効くもの)。表記・名前・表示だけの列(recovery・onHit)は入れない。
// ★onBlock は絞り込み「ガード時 0F 以上だけ」に効くので入れる。
type calcFields struct {
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

func moveFingerprint(m Move) (string, error) {
	b, err := json.Marshal(calcFields{
		Code: m.Code, Category: m.Category, Startup: m.Startup, Active: m.Active, Total: m.Total,
		OnBlock: m.OnBlock, Damage: m.Damage, IsDerived: m.IsDerived, IsAerial: m.IsAerial,
		IsProjectile: m.IsProjectile, SetupOnly: m.SetupOnly, StartupBasis: m.StartupBasis,
		ChainCancelTotal: m.ChainCancelTotal, FastestUnreachable: m.FastestUnreachable,
		OriginalMove: m.OriginalMove, Parents: m.Parents,
	})
	if err != nil {
		return "", fmt.Errorf("marshal calc fields %s: %w", m.Code, err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])[:fingerprintHexLen], nil
}

// characterFingerprint はキャラ単位の指紋。技の並び(id 順)も計算に効くので、各技の指紋を並び順に連ねる。
func characterFingerprint(f CharacterFile) (string, error) {
	h := sha256.New()
	b, err := json.Marshal(f.SafeJump)
	if err != nil {
		return "", fmt.Errorf("marshal safe jump: %w", err)
	}
	h.Write(b)
	for _, m := range f.Moves {
		// ★保存済みの値ではなく中身から計算する(技の指紋だけ古いまま、を見逃さない)。
		fp, err := moveFingerprint(m)
		if err != nil {
			return "", err
		}
		h.Write([]byte("\n" + fp))
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// moveStates は技が前提とする状態を返す(0 か 1 件)。move_code が状態 code で始まり(matchState)、かつ
// name_ja の [..]・【..】 のラベルに状態の name_ja が含まれるときだけ当てる。
// ★move_code だけでは決めない。舞の kachousen_*(花蝶扇)は状態「花蝶扇バウンド中」(kachousen)を作る技で、
// 前提ではない。name_ja に状態の印が無いことで見分ける(再レビュー 高-1)。
func moveStates(code, nameJA string, states []State) []State {
	st, _, ok := matchState(code, states)
	if !ok || st.Name.JA == "" {
		return nil
	}
	for _, m := range bracketLabel.FindAllStringSubmatch(nameJA, -1) {
		if strings.Contains(m[1]+m[2], st.Name.JA) {
			return []State{st}
		}
	}
	return nil
}

// stateOf は move_code から技の状態 code を返す(matchState。無ければ nil)。
// states には moveStates の結果(その技が前提とする状態だけ)を渡す。
func stateOf(code string, states []State) *string {
	st, _, ok := matchState(code, states)
	if !ok {
		return nil
	}
	c := st.Code
	return &c
}
