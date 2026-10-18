package movesdb

// 本ファイルの型が moves DB(data/moves/<character>.json)の形の正本である。
// 説明は docs/data/moves-db.md。形を変えたら同ファイルと SchemaVersion を同時に直すこと。

// SchemaVersion は moves DB の形の版。形を変えたら上げる。
const SchemaVersion = 1

// LocalizedName は日本語・英語の名前。
type LocalizedName struct {
	JA string `json:"ja"`
	EN string `json:"en"`
}

// CharacterRef はキャラの識別と名前。
type CharacterRef struct {
	Code string        `json:"code"`
	Name LocalizedName `json:"name"`
}

// State はキャラ固有の状態(characters.custom_states の states)。
type State struct {
	Code string        `json:"code"`
	Name LocalizedName `json:"name"`
}

// SafeJump は安全飛びの値(characters.custom_states の safe_jump。値は「繋ぎの後に残すフレーム」)。
type SafeJump struct {
	True int `json:"true"`
	F5   int `json:"f5"`
	F6   int `json:"f6"`
}

// Notation は技の 4 表記(HANDOVER D6)。どれも空文字にならない(検査 CheckFile)。
type Notation struct {
	NameJA  string `json:"nameJa"`
	NameEN  string `json:"nameEn"`
	SRK     string `json:"srk"`
	Numeric string `json:"numeric"`
}

// Move は技 1 件。数値列の null は「データに値が無い」(0 とは別)。
// 列の意味は本家の moves と同じ(docs/data/moves-db.md。名前から推測しないこと)。
type Move struct {
	Code               string   `json:"code"`
	Category           string   `json:"category"`
	Startup            *int     `json:"startup"`
	Active             *int     `json:"active"`
	Recovery           *int     `json:"recovery"`
	Total              *int     `json:"total"`
	OnHit              *int     `json:"onHit"`
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
	// State はこの技が前提とするキャラ固有の状態(states[].code)。move_code が状態 code で始まる技
	// (ラッシュ版は rush_ の後ろ)に付く。無ければ null。
	State    *string  `json:"state"`
	Notation Notation `json:"notation"`
	// CalcFingerprint は計算に効く列だけの指紋(表記・名前は含まない)。技単位の変更検知に使う。
	CalcFingerprint string `json:"calcFingerprint"`
}

// CharacterFile は data/moves/<character>.json の全体。
type CharacterFile struct {
	SchemaVersion int          `json:"schemaVersion"`
	DataVersion   string       `json:"dataVersion"`
	Character     CharacterRef `json:"character"`
	States        []State      `json:"states"`
	SafeJump      *SafeJump    `json:"safeJump"`
	// CalcFingerprint はキャラ単位の指紋(全技の CalcFingerprint を並び順に連ね、安全飛びを足したもの)。
	CalcFingerprint string `json:"calcFingerprint"`
	// Moves は本家の moves.id 昇順(エンジンの決定性に効く。並べ替えないこと)。
	Moves []Move `json:"moves"`
}

// IndexEntry は index.json の 1 キャラ分。
type IndexEntry struct {
	Code      string        `json:"code"`
	Name      LocalizedName `json:"name"`
	File      string        `json:"file"`
	MoveCount int           `json:"moveCount"`
	// DataVersion はキャラごとの版(そのキャラのファイルの dataVersion と同じ。CarryDataVersions)。
	DataVersion     string `json:"dataVersion"`
	CalcFingerprint string `json:"calcFingerprint"`
	// FileSHA256 はファイルの中身の sha256(検査が中身と突き合わせる)。
	FileSHA256 string `json:"fileSha256"`
}

// Index は data/moves/index.json。キャラは本家の characters.id 昇順。
type Index struct {
	SchemaVersion int `json:"schemaVersion"`
	// DataVersion は全体の版(取込元の公開版 tacpendium の SHA = lock)。キャラごとの版は Characters[].DataVersion。
	DataVersion string       `json:"dataVersion"`
	Characters  []IndexEntry `json:"characters"`
}
