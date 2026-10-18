package seedgen

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// ReadFile は 1 キャラの CSV を読み、行を返す。startRowIndex は RowIndex の開始通し番号。
// ヘッダは csvColumns と厳密一致を要求する(列契約のズレを早期に検出)。
func ReadFile(path string, startRowIndex int) ([]MoveRow, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()
	rows, err := Read(f, startRowIndex)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return rows, nil
}

// Read は CSV を r から読み取り MoveRow 列へ変換する。
func Read(r io.Reader, startRowIndex int) ([]MoveRow, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = len(csvColumns) // 列数の厳密一致を強制

	header, err := cr.Read()
	if err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}
	if err := checkHeader(header); err != nil {
		return nil, err
	}

	var out []MoveRow
	line := 1 // ヘッダ = 1 行目
	for {
		rec, err := cr.Read()
		if err == io.EOF {
			break
		}
		line++
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}
		row, err := parseRow(rec, startRowIndex+len(out))
		if err != nil {
			return nil, fmt.Errorf("line %d (%s): %w", line, rec[1], err)
		}
		out = append(out, row)
	}
	return out, nil
}

func checkHeader(header []string) error {
	if len(header) != len(csvColumns) {
		return fmt.Errorf("header has %d columns, want %d", len(header), len(csvColumns))
	}
	for i, want := range csvColumns {
		if strings.TrimSpace(header[i]) != want {
			return fmt.Errorf("header column %d = %q, want %q", i+1, header[i], want)
		}
	}
	return nil
}

func parseRow(rec []string, rowIndex int) (MoveRow, error) {
	g := func(i int) string { return strings.TrimSpace(rec[i]) }

	startup, err := parseNullableInt(g(4))
	if err != nil {
		return MoveRow{}, fmt.Errorf("startup: %w", err)
	}
	active, err := parseNullableInt(g(5))
	if err != nil {
		return MoveRow{}, fmt.Errorf("active: %w", err)
	}
	recovery, err := parseNullableInt(g(6))
	if err != nil {
		// 非整数 recovery は手入力ミス扱い(指示書 §4.1・確定 B)。
		return MoveRow{}, fmt.Errorf("recovery must be an integer (got %q): %w", g(6), err)
	}
	total, err := parseNullableInt(g(7))
	if err != nil {
		return MoveRow{}, fmt.Errorf("total: %w", err)
	}
	onHit, err := parseNullableInt(g(8))
	if err != nil {
		return MoveRow{}, fmt.Errorf("on_hit: %w", err)
	}
	onBlock, err := parseNullableInt(g(9))
	if err != nil {
		return MoveRow{}, fmt.Errorf("on_block: %w", err)
	}
	damage, err := parseNullableInt(g(10))
	if err != nil {
		return MoveRow{}, fmt.Errorf("damage: %w", err)
	}
	isAerial, err := parseBool(g(11))
	if err != nil {
		return MoveRow{}, fmt.Errorf("is_aerial: %w", err)
	}
	isProjectile, err := parseBool(g(12))
	if err != nil {
		return MoveRow{}, fmt.Errorf("is_projectile: %w", err)
	}
	isDerived, err := parseBool(g(13))
	if err != nil {
		return MoveRow{}, fmt.Errorf("is_derived: %w", err)
	}
	chainCancelTotal, err := parseNullableInt(g(21))
	if err != nil {
		return MoveRow{}, fmt.Errorf("chain_cancel_total: %w", err)
	}
	fastestUnreachable, err := parseBool(g(22))
	if err != nil {
		return MoveRow{}, fmt.Errorf("fastest_unreachable: %w", err)
	}
	firstHitStartup, err := parseNullableInt(g(24))
	if err != nil {
		return MoveRow{}, fmt.Errorf("first_hit_startup: %w", err)
	}

	return MoveRow{
		CharacterCode:    g(0),
		MoveCode:         g(1),
		Category:         g(2),
		NameJA:           g(3),
		Startup:          startup,
		Active:           active,
		Recovery:         recovery,
		Total:            total,
		OnHit:            onHit,
		OnBlock:          onBlock,
		Damage:           damage,
		IsAerial:         isAerial,
		IsProjectile:     isProjectile,
		IsDerived:        isDerived,
		Notes:            g(14),
		NotesTool:        g(15),
		OriginalMoveCode: g(16),
		Command:          g(17),
		ConditionJA:      g(18),
		ConditionEN:      g(19),
		// 新列 3 つ(旧 000049)。保全のみ・SQL 非投入。startup_basis の値域検証は
		// category と同じく validate 側で行う(parseRow は型エラーのみを見る分業)。
		StartupBasis:       g(20),
		ChainCancelTotal:   chainCancelTotal,
		FastestUnreachable: fastestUnreachable,
		// M28-02a: FR702 のマーカー。★こちらは SQL へ投入する(§2.2-4)。
		LastChangedGameVersion: g(23),
		// M37-04(B06): 初段の発生。保全のみ・SQL 非投入(上の旧 000049 の 3 列と同じ流儀)。
		FirstHitStartup: firstHitStartup,
		RowIndex:        rowIndex,
	}, nil
}

func parseNullableInt(s string) (*int, error) {
	if s == "" {
		return nil, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return nil, err
	}
	return &n, nil
}

func parseBool(s string) (bool, error) {
	switch s {
	case "true":
		return true, nil
	case "false", "":
		return false, nil
	default:
		return false, fmt.Errorf("want true/false, got %q", s)
	}
}
