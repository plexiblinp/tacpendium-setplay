// Package movesdb は公開版 tacpendium のデータから、単体版の moves DB(キャラ別 JSON)を作る。
//
// 流れ: 公開版の migrations(*.up.sql)を番号順に一時の SQLite へ流す → 本家のエンジンが読むのと
// 同じ列・同じ順(moves.id 昇順)で読む → 4 表記を付ける → 検査する → 決定的な JSON に書く。
//
// ★SQL を手で解析しない。setup_only・派生関係・安全飛び・データ修正(UPDATE)は SQL でしか
// 表されておらず、本家のエンジンが見る値と同じにするには、本家と同じく migrations を流すのが確実である。
// ★本パッケージは取込(事前の変換)専用で、依存(modernc.org/sqlite)を持つ。
// エンジン(engine/)は本パッケージを import しない(標準ライブラリのみ・AGENTS §4)。
package movesdb

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/plexiblinp/tacpendium-setplay/internal/seedgen"

	_ "modernc.org/sqlite" // migrations を流す一時 DB
)

// Preset codes(本家 000007 の presets.code)。
const (
	PresetOfficialJA = "official_ja_move"
	PresetNumeric    = "numeric"
	PresetSRK        = "srk"
)

// DBCharacter は characters の 1 行。
type DBCharacter struct {
	ID           int64
	Code         string
	NameJA       string
	NameEN       string
	CustomStates sql.NullString // JSON。NULL 可
}

// DBMove は moves の 1 行(単体版が使う列)。NULL 可の列はポインタで持つ(NULL と 0 を区別する)。
type DBMove struct {
	ID                 int64
	CharacterID        int64
	Code               string
	Category           string
	OriginalMoveID     *int64
	Startup            *int
	Active             *int
	Recovery           *int
	Total              *int
	OnHit              *int
	OnBlock            *int
	Damage             *int
	IsDerived          bool
	IsAerial           bool
	IsProjectile       bool
	SetupOnly          bool
	StartupBasis       string
	ChainCancelTotal   *int
	FastestUnreachable bool
}

// Alias は preset_aliases の 1 行。
type Alias struct {
	Text   string
	TextEn string // "" は NULL
}

// Dataset は取込んだ全データ。
type Dataset struct {
	// Characters は characters.id 昇順。
	Characters []DBCharacter
	// Moves はキャラ id → moves(id 昇順)。
	Moves map[int64][]DBMove
	// Parents は子の move id → 親の move id(昇順)。
	Parents map[int64][]int64
	// Aliases は preset code → move id → 表記。
	Aliases map[string]map[int64]Alias
	// CSV はキャラ code → character_data/<code>.csv の行(CSV の行順)。
	CSV map[string][]seedgen.MoveRow
}

// Load は公開版 tacpendium のチェックアウト srcDir から Dataset を作る。
func Load(ctx context.Context, srcDir string) (*Dataset, error) {
	db, err := openMigrated(ctx, filepath.Join(srcDir, "migrations"))
	if err != nil {
		return nil, err
	}
	defer func() { _ = db.Close() }()

	ds := &Dataset{
		Moves:   map[int64][]DBMove{},
		Parents: map[int64][]int64{},
		Aliases: map[string]map[int64]Alias{},
		CSV:     map[string][]seedgen.MoveRow{},
	}
	if ds.Characters, err = loadCharacters(ctx, db); err != nil {
		return nil, err
	}
	if err := loadMoves(ctx, db, ds); err != nil {
		return nil, err
	}
	if err := loadDerivations(ctx, db, ds); err != nil {
		return nil, err
	}
	if err := loadAliases(ctx, db, ds); err != nil {
		return nil, err
	}
	for _, c := range ds.Characters {
		rows, err := seedgen.ReadFile(filepath.Join(srcDir, "character_data", c.Code+".csv"), 0)
		if err != nil {
			return nil, fmt.Errorf("read csv %s: %w", c.Code, err)
		}
		ds.CSV[c.Code] = rows
	}
	return ds, nil
}

// openMigrated はメモリ上の SQLite に dir の *.up.sql を番号順に流す。
func openMigrated(ctx context.Context, dir string) (*sql.DB, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.up.sql"))
	if err != nil {
		return nil, fmt.Errorf("glob migrations: %w", err)
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("migrations が見つからない: %s", dir)
	}
	sort.Strings(files)

	db, err := sql.Open("sqlite", "file::memory:?_pragma=foreign_keys(1)")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// メモリ DB は接続ごとに別物になる。1 本に固定する。
	db.SetMaxOpenConns(1)
	for _, f := range files {
		body, err := os.ReadFile(f)
		if err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("read %s: %w", f, err)
		}
		if _, err := db.ExecContext(ctx, string(body)); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("apply %s: %w", filepath.Base(f), err)
		}
	}
	return db, nil
}

func loadCharacters(ctx context.Context, db *sql.DB) ([]DBCharacter, error) {
	rows, err := db.QueryContext(ctx, `SELECT id, code, name_ja, name_en, custom_states FROM characters ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("query characters: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []DBCharacter
	for rows.Next() {
		var c DBCharacter
		if err := rows.Scan(&c.ID, &c.Code, &c.NameJA, &c.NameEN, &c.CustomStates); err != nil {
			return nil, fmt.Errorf("scan character: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate characters: %w", err)
	}
	return out, nil
}

// movesSQL は本家 repository/setplay の listCandidatesSQL と同じ列に、表示用の
// recovery / on_hit を足したもの。★順序は本家と同じ moves.id 昇順(エンジンの決定性に効く)。
const movesSQL = `
SELECT m.id, m.character_id, m.code, m.category, m.original_move_id,
       m.startup, m.active, m.recovery, m.total, m.on_hit, m.on_block, m.damage,
       m.is_derived, m.is_aerial, m.is_projectile, m.setup_only,
       m.startup_basis, m.chain_cancel_total, m.fastest_unreachable
FROM moves m
ORDER BY m.character_id, m.id`

func loadMoves(ctx context.Context, db *sql.DB, ds *Dataset) error {
	rows, err := db.QueryContext(ctx, movesSQL)
	if err != nil {
		return fmt.Errorf("query moves: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var m DBMove
		if err := rows.Scan(&m.ID, &m.CharacterID, &m.Code, &m.Category, &m.OriginalMoveID,
			&m.Startup, &m.Active, &m.Recovery, &m.Total, &m.OnHit, &m.OnBlock, &m.Damage,
			&m.IsDerived, &m.IsAerial, &m.IsProjectile, &m.SetupOnly,
			&m.StartupBasis, &m.ChainCancelTotal, &m.FastestUnreachable); err != nil {
			return fmt.Errorf("scan move: %w", err)
		}
		ds.Moves[m.CharacterID] = append(ds.Moves[m.CharacterID], m)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate moves: %w", err)
	}
	return nil
}

func loadDerivations(ctx context.Context, db *sql.DB, ds *Dataset) error {
	rows, err := db.QueryContext(ctx, `SELECT child_move_id, parent_move_id FROM move_derivations ORDER BY child_move_id, parent_move_id`)
	if err != nil {
		return fmt.Errorf("query derivations: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var c, p int64
		if err := rows.Scan(&c, &p); err != nil {
			return fmt.Errorf("scan derivation: %w", err)
		}
		ds.Parents[c] = append(ds.Parents[c], p)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate derivations: %w", err)
	}
	return nil
}

func loadAliases(ctx context.Context, db *sql.DB, ds *Dataset) error {
	rows, err := db.QueryContext(ctx, `
SELECT p.code, a.move_id, a.alias_text, COALESCE(a.alias_text_en, '')
FROM preset_aliases a JOIN presets p ON p.id = a.preset_id
WHERE p.is_builtin = 1
ORDER BY p.code, a.move_id`)
	if err != nil {
		return fmt.Errorf("query aliases: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var code string
		var id int64
		var a Alias
		if err := rows.Scan(&code, &id, &a.Text, &a.TextEn); err != nil {
			return fmt.Errorf("scan alias: %w", err)
		}
		if ds.Aliases[code] == nil {
			ds.Aliases[code] = map[int64]Alias{}
		}
		ds.Aliases[code][id] = a
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate aliases: %w", err)
	}
	return nil
}

// customStates は characters.custom_states の JSON(単体版が使う部分)。
type customStates struct {
	States []struct {
		Code   string `json:"code"`
		NameJA string `json:"name_ja"`
		NameEN string `json:"name_en"`
	} `json:"states"`
	SafeJump *struct {
		True int `json:"true"`
		F5   int `json:"f5"`
		F6   int `json:"f6"`
	} `json:"safe_jump"`
}

func parseCustomStates(s sql.NullString) (customStates, error) {
	var cs customStates
	if !s.Valid || strings.TrimSpace(s.String) == "" {
		return cs, nil
	}
	if err := json.Unmarshal([]byte(s.String), &cs); err != nil {
		return cs, fmt.Errorf("parse custom_states: %w", err)
	}
	return cs, nil
}
