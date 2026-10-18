package seedgen

import (
	"fmt"
	"strings"

	"github.com/plexiblinp/tacpendium-setplay/internal/moveindex"
)

// 本ファイルは M17-02(G-k/CHANGE-069)の生成系 2 本を提供する。
//   - GenerateDerivedBackfill: 投入済みキャラの moves.is_derived backfill マイグレ。
//     ★凍結 golden は 000034_backfill_moves_is_derived(testdata/)。
//   - GenerateMoveCommands:    command 索引 move_commands の seed マイグレ。
//     ★凍結 golden は 000035_seed_move_commands(testdata/)。
// いずれも「生成物は手編集しない・golden テストで drift 検出」(M14-03c の定石)に従う。

// BackfillHeader は is_derived backfill マイグレ用の生成 SQL ヘッダを組み立てる
// (cmd/seedgen の -mode derived-backfill と golden テストが共用する)。
func BackfillHeader(stem, note string) func(dir string) string {
	return func(dir string) string {
		h := "-- " + stem + "." + dir + ".sql\n"
		if note != "" {
			h += "-- " + note + "\n"
		}
		h += "-- 本ファイルは cmd/seedgen が character_data/*.csv から生成した成果物(手編集しない)。\n" +
			"-- CSV で is_derived=true の code のみ UPDATE する(触れない行は既定値 false のまま正しく振る舞う\n" +
			"-- ＝ユーザー生成行〔画面18 追加・rush-variant 生成〕に触れない。CHANGE-069 §2.1-d/-e)。\n" +
			"-- 既存マイグレは非改変(新規連番で追加)。\n\n"
		return h
	}
}

// MoveCommandsHeader は move_commands seed マイグレ用の生成 SQL ヘッダを組み立てる
// (cmd/seedgen の -mode move-commands と golden テストが共用する)。
func MoveCommandsHeader(stem, note string) func(dir string) string {
	return func(dir string) string {
		h := "-- " + stem + "." + dir + ".sql\n"
		if note != "" {
			h += "-- " + note + "\n"
		}
		h += "-- 本ファイルは cmd/seedgen が character_data/*.csv から生成した成果物(手編集しない)。\n" +
			"-- 非派生技(is_derived=false)のみ搭載。token_key は numpad 正規化済み(internal/moveindex が一元管理)。\n" +
			"-- 索引は広く持つ(必殺技・空中技も搭載)＝段階2 スコープへの絞り込みは消費者側(CHANGE-069 §1.5-6)。\n" +
			"-- 既存マイグレは非改変(新規連番で追加)。FK 依存順 characters→moves→move_commands。\n\n"
		return h
	}
}

// GenerateDerivedBackfill は CSV の is_derived=true 行から moves.is_derived の backfill マイグレ
// (up: =1 / down: =0)を生成する。UPDATE は CSV に載る code の列挙のみで行う(削除系と非対称に、
// 列挙で正しい＝触らない行が既定値のまま正しく振る舞う。CHANGE-069 §4-#1-d)。
func GenerateDerivedBackfill(charOrder []string, rowsByChar map[string][]MoveRow, header func(dir string) string) (*Result, error) {
	if err := validate(charOrder, rowsByChar); err != nil {
		return nil, err
	}

	res := &Result{
		Index: moveindex.New(), // backfill では索引は構築しない(空のまま)
		Stats: map[string]int{},
	}

	var up strings.Builder
	up.WriteString(header("up"))
	var downBlocks []string

	for _, code := range charOrder {
		var derived []MoveRow
		for _, r := range rowsByChar[code] {
			if r.isMovementSystem() {
				res.Dropped = append(res.Dropped, r)
				continue
			}
			if r.IsDerived {
				derived = append(derived, r)
			}
		}
		if len(derived) == 0 {
			continue
		}
		res.Stats[code] = len(derived)

		fmt.Fprintf(&up, "-- ===== %s (%d moves) =====\n", code, len(derived))
		up.WriteString(buildDerivedUpdate(code, derived, 1))
		downBlocks = append(downBlocks,
			fmt.Sprintf("-- ===== %s =====\n", code)+buildDerivedUpdate(code, derived, 0))
	}

	var down strings.Builder
	down.WriteString(header("down"))
	// down はキャラ逆順で結合(投入と逆順・buildCharDown と同じ運び)。
	for i := len(downBlocks) - 1; i >= 0; i-- {
		down.WriteString(downBlocks[i])
	}

	res.UpSQL = up.String()
	res.DownSQL = down.String()
	return res, nil
}

// buildDerivedUpdate は 1 キャラ分の is_derived UPDATE 文を組み立てる。
func buildDerivedUpdate(code string, rows []MoveRow, value int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "UPDATE moves SET is_derived = %d\n", value)
	fmt.Fprintf(&b, "WHERE character_id IN (SELECT c.id FROM characters c WHERE %s)\n", charFilter("c", code))
	b.WriteString("  AND code IN (")
	writeCodeList(&b, rows)
	b.WriteString(");\n\n")
	return b.String()
}

// GenerateMoveCommands は CSV の command 列から move_commands の seed マイグレを生成する。
// 索引の構築主体は internal/moveindex(M14-03b IF)＝skip 規則(派生/空/raw{/cond{/語彙外)と
// numpad 正規化をここで再実装しない(「二度作らない」)。
func GenerateMoveCommands(charOrder []string, rowsByChar map[string][]MoveRow, header func(dir string) string) (*Result, error) {
	if err := validate(charOrder, rowsByChar); err != nil {
		return nil, err
	}

	res := &Result{
		Index: moveindex.New(),
		Stats: map[string]int{},
	}

	for _, code := range charOrder {
		for _, r := range rowsByChar[code] {
			if r.isMovementSystem() {
				res.Dropped = append(res.Dropped, r)
				continue
			}
			res.Index.Add(r.CharacterCode, r.MoveCode, r.Command, r.IsDerived, r.RowIndex)
		}
	}

	// Entries は CharKey→TokenKey→ID 昇順の決定論順序。キャラごとに分配して charOrder 順に出力する。
	byChar := map[string][]moveindex.Entry{}
	for _, e := range res.Index.Entries() {
		byChar[e.CharKey] = append(byChar[e.CharKey], e)
	}

	var up strings.Builder
	up.WriteString(header("up"))
	var downBlocks []string

	for _, code := range charOrder {
		entries := byChar[code]
		if len(entries) == 0 {
			continue
		}
		res.Stats[code] = len(entries)

		fmt.Fprintf(&up, "-- ===== %s (%d commands) =====\n", code, len(entries))
		writeMoveCommandsInsert(&up, code, entries)
		downBlocks = append(downBlocks, buildMoveCommandsDown(code, entries))
	}

	var down strings.Builder
	down.WriteString(header("down"))
	for i := len(downBlocks) - 1; i >= 0; i-- {
		down.WriteString(downBlocks[i])
	}

	res.UpSQL = up.String()
	res.DownSQL = down.String()
	return res, nil
}

// writeMoveCommandsInsert は 1 キャラ分の move_commands INSERT 文を書く(writeAliasInsert と同型)。
func writeMoveCommandsInsert(b *strings.Builder, code string, entries []moveindex.Entry) {
	b.WriteString("INSERT INTO move_commands (move_id, character_id, token_key)\n")
	b.WriteString("SELECT m.id, m.character_id, v.token_key\n")
	b.WriteString("FROM moves m\nJOIN characters c ON c.id = m.character_id\n")
	b.WriteString("CROSS JOIN (\n")
	for i, e := range entries {
		prefix := "  UNION ALL SELECT "
		if i == 0 {
			prefix = "        SELECT "
		}
		b.WriteString(prefix)
		b.WriteString(sqlStr(e.MoveCode))
		if i == 0 {
			b.WriteString(" AS code")
		}
		b.WriteString(", ")
		b.WriteString(sqlStr(e.TokenKey))
		if i == 0 {
			b.WriteString(" AS token_key")
		}
		b.WriteString("\n")
	}
	fmt.Fprintf(b, ") AS v\nWHERE %s AND m.code = v.code;\n\n", charFilter("c", code))
}

// buildMoveCommandsDown は 1 キャラ分の down(投入 code のみの精密 DELETE)を組み立てる。
func buildMoveCommandsDown(code string, entries []moveindex.Entry) string {
	// 同一 move が複数 token を持つ将来形に備えて code を重複排除する(現行は 1 move 1 command)。
	seen := map[string]bool{}
	var codes []string
	for _, e := range entries {
		if !seen[e.MoveCode] {
			seen[e.MoveCode] = true
			codes = append(codes, e.MoveCode)
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "-- ===== %s =====\n", code)
	b.WriteString("DELETE FROM move_commands WHERE move_id IN (\n")
	fmt.Fprintf(&b, "    SELECT m.id FROM moves m JOIN characters c ON c.id = m.character_id WHERE %s AND m.code IN (", charFilter("c", code))
	for i, c := range codes {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(sqlStr(c))
	}
	b.WriteString("));\n\n")
	return b.String()
}
