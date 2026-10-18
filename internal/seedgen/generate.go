package seedgen

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/plexiblinp/tacpendium-setplay/internal/gameversion"
	"github.com/plexiblinp/tacpendium-setplay/internal/moveindex"
)

// moveCodePattern は DES-004 §2.1 の正準 move_code 書式(小文字英数・アンダースコア)。
var moveCodePattern = regexp.MustCompile(`^[a-z0-9_]+$`)

// FirstWaveStem は第一波の生成物の stem。
//
// ★★M33-03 までは「コミット済みマイグレ migrations/000026_seed_moves_first_wave」を
// 指していた。⇒ M33-02 が旧 111 本を新系列 9 本へ潰したため、その連番のマイグレは
// 存在しない。★いまは internal/seedgen/testdata/ に凍結した golden の名前である
// （由来を辿れるよう綴りは変えていない）。
const FirstWaveStem = "000026_seed_moves_first_wave"

// FirstWaveOrder は第一波キャラの固定順序。
// 決定論的な生成物(再生成で差分ゼロ)のため順序を固定する。ryu は本サブ対象外(M14-03c)。
var FirstWaveOrder = []string{
	"terry", "guile", "lily", "ingrid", "kimberly", "juri", "ken", "mai", "zangief",
}

// Result は生成の成果物。
type Result struct {
	UpSQL   string
	DownSQL string
	Index   *moveindex.Index
	Dropped []MoveRow // drop した移動 system move
	Stats   map[string]int
}

// Generate は キャラ別 CSV 行から seed SQL(up/down)と索引を生成する。
// 検証(total 検算・category enum・dup)に失敗した場合は SQL を生成せずエラーを返す。
// charOrder は投入するキャラ code の順序(通常 FirstWaveOrder)。rowsByChar は各 code の CSV 行。
// ヘッダは第一波(FirstWaveStem)固定文言。別 stem へ出力する場合は GenerateWithHeader を使う。
// opts は省略可(既定 = FormatCurrent。詳細は format.go)。
func Generate(charOrder []string, rowsByChar map[string][]MoveRow, opts ...Option) (*Result, error) {
	return GenerateWithHeader(charOrder, rowsByChar, defaultHeader, opts...)
}

// GenerateWithHeader は Generate のヘッダ差し替え版(M14-03c §4.3.1 の I/O 境界拡張)。
// header は方向("up"/"down")を受けて生成 SQL 冒頭のコメントヘッダを返す。
// 変換規則(validate・remap・alias 対・移動 9 種 drop・dup スキャン)は Generate と完全に同一。
// opts は省略可(既定 = FormatCurrent。詳細は format.go)。
func GenerateWithHeader(charOrder []string, rowsByChar map[string][]MoveRow, header func(dir string) string,
	opts ...Option) (*Result, error) {
	if err := validate(charOrder, rowsByChar); err != nil {
		return nil, err
	}
	opt := resolveOptions(opts)

	res := &Result{
		Index: moveindex.New(),
		Stats: map[string]int{},
	}

	var up, down strings.Builder
	up.WriteString(header("up"))
	down.WriteString(header("down"))

	// down は FK 逆順・キャラ逆順で安全に削除するため後で結合する。
	var downBlocks []string

	for _, code := range charOrder {
		rows := rowsByChar[code]
		var insertRows []MoveRow
		for _, r := range rows {
			if r.isMovementSystem() {
				res.Dropped = append(res.Dropped, r) // §4.1 移動 9 種 drop
				continue
			}
			insertRows = append(insertRows, r)
			// 索引構築(非派生のみ載る・派生/空/marker は Skipped 記録)。
			res.Index.Add(r.CharacterCode, r.MoveCode, r.Command, r.IsDerived, r.RowIndex)
		}
		if len(insertRows) == 0 {
			continue
		}
		res.Stats[code] = len(insertRows)

		writeCharComment(&up, code, len(insertRows))
		writeMovesInsert(&up, code, insertRows)
		writeOriginalMoveIDUpdate(&up, code, insertRows)
		writeAliasInsert(&up, code, insertRows, opt.format)

		downBlocks = append(downBlocks, buildCharDown(code, insertRows))
	}

	// down はキャラ逆順で結合(投入と逆順)。
	for i := len(downBlocks) - 1; i >= 0; i-- {
		down.WriteString(downBlocks[i])
	}

	res.UpSQL = up.String()
	res.DownSQL = down.String()
	return res, nil
}

// validate は投入前検証を行う(dup・total 検算・category enum)。1 件でも問題があれば全体を fail。
func validate(charOrder []string, rowsByChar map[string][]MoveRow) error {
	var problems []string

	for _, code := range charOrder {
		rows, ok := rowsByChar[code]
		if !ok {
			return fmt.Errorf("no CSV rows for character %q", code)
		}
		seenCode := map[string]string{}  // code -> first name(dup 検出)
		seenAlias := map[string]string{} // alias_text -> first code
		for _, r := range rows {
			if r.isMovementSystem() {
				continue // drop 対象は検証・投入いずれの対象外
			}
			if r.CharacterCode != code {
				problems = append(problems, fmt.Sprintf("%s: row %q has character_code %q (file/column mismatch)", code, r.MoveCode, r.CharacterCode))
			}
			if !validCategories[r.Category] {
				problems = append(problems, fmt.Sprintf("%s/%s: unknown category %q", code, r.MoveCode, r.Category))
			}
			if !validStartupBases[r.StartupBasis] {
				problems = append(problems, fmt.Sprintf("%s/%s: unknown startup_basis %q", code, r.MoveCode, r.StartupBasis))
			}
			// ★★FR702 のマーカーの値域(M28-02a §2.2-3-b)。空欄は未記入として許容する。
			//   ★ここが seedgen 側の入口である。ゼロ埋めが崩れた版数を 1 つ通すだけで
			//     辞書順が静かに壊れ、エラーにはならない。⇒ 生成そのものを fail させる。
			if r.LastChangedGameVersion != "" && !gameversion.Valid(r.LastChangedGameVersion) {
				problems = append(problems, fmt.Sprintf("%s/%s: invalid last_changed_game_version %q (want YYYY.MM.DD.NN)",
					code, r.MoveCode, r.LastChangedGameVersion))
			}
			if r.MoveCode == "" {
				problems = append(problems, fmt.Sprintf("%s: empty move_code", code))
			} else if !moveCodePattern.MatchString(r.MoveCode) {
				// DES-004 §2.1 正準形の書式検証(CSV を正とし再採番はしないが、書式逸脱は fail)。
				problems = append(problems, fmt.Sprintf("%s: move_code %q is not canonical form [a-z0-9_]+", code, r.MoveCode))
			}
			if r.NameJA == "" {
				problems = append(problems, fmt.Sprintf("%s/%s: empty name_ja (alias 源)", code, r.MoveCode))
			}
			// dup: 同一キャラ内 code 衝突。
			if prev, dup := seenCode[r.MoveCode]; dup {
				problems = append(problems, fmt.Sprintf("%s: duplicate move_code %q (also %q)", code, r.MoveCode, prev))
			} else {
				seenCode[r.MoveCode] = r.MoveCode
			}
			// dup: 同一キャラ内 alias_text(=name_ja)衝突。
			if prev, dup := seenAlias[r.NameJA]; dup {
				problems = append(problems, fmt.Sprintf("%s: duplicate alias_text %q (move %q and %q)", code, r.NameJA, prev, r.MoveCode))
			} else {
				seenAlias[r.NameJA] = r.MoveCode
			}
			// total 検算(startup/active/recovery が揃う行のみ)。
			if r.Startup != nil && r.Active != nil && r.Recovery != nil {
				want := *r.Startup + *r.Active - 1 + *r.Recovery
				if r.Total == nil || *r.Total != want {
					problems = append(problems, fmt.Sprintf("%s/%s: total mismatch (startup+active-1+recovery=%d, csv total=%s)", code, r.MoveCode, want, intStr(r.Total)))
				}
			}
		}
	}

	if len(problems) > 0 {
		sort.Strings(problems)
		return fmt.Errorf("seedgen validation failed (%d issue(s)); no SQL generated:\n  - %s",
			len(problems), strings.Join(problems, "\n  - "))
	}
	return nil
}

// ---- SQL 生成ヘルパ ----

// CustomHeader は stem(出力ファイル名)と note(説明行)から生成 SQL のヘッダを組み立てる
// (M14-03c §4.3.1 の I/O 境界。cmd/seedgen の -chars/-out 指定時と golden テストが共用する)。
func CustomHeader(stem, note string) func(dir string) string {
	return func(dir string) string {
		h := "-- " + stem + "." + dir + ".sql\n"
		if note != "" {
			h += "-- " + note + "\n"
		}
		h += "-- 本ファイルは cmd/seedgen が character_data/*.csv から生成した成果物(手編集しない)。\n" +
			"-- 移動 system move は CSV に無い(seed の投入元は 000004_data_seed_moves)。\n" +
			"-- FK 依存順 characters→moves→preset_aliases。\n\n"
		return h
	}
}

// defaultHeader は第一波 stem 用の既定ヘッダ。文言は byte-identical 維持(golden ゲート対象)。
//
// ★★M33-03 で失効した連番参照を落とした。⇒ ここは *文字列リテラルであり生成物の byte に出る*。
// 着手時点は「移動 system move 9 種は drop 済(投入元は 000025)」「既存マイグレ
// 000001〜000025 は非改変」と書いていたが、M33-02 が旧 111 本を新系列 9 本へ潰したため
// 000025 は存在しない。★「000001〜000025」は範囲表記なので、新 000001 が在ることを
// 理由に「偶然正しい」とは言えない。
func defaultHeader(dir string) string {
	return "-- " + FirstWaveStem + "." + dir + ".sql\n" +
		"-- M14-03b: 第一波キャラの moves + official_ja_move alias + recovery を投入する。\n" +
		"-- 本ファイルは cmd/seedgen が character_data/*.csv から生成した成果物(手編集しない)。\n" +
		"-- 移動 system move は CSV に無い(seed の投入元は 000004_data_seed_moves)。\n" +
		"-- ryu は本サブ対象外(M14-03c)。FK 依存順 characters→moves→preset_aliases。\n\n"
}

func writeCharComment(b *strings.Builder, code string, n int) {
	fmt.Fprintf(b, "-- ===== %s (%d moves) =====\n", code, n)
}

// charFilter は「code=? かつ sf6」の WHERE 断片。
func charFilter(alias, code string) string {
	return fmt.Sprintf("%s.code = %s AND %s.game_id IN (SELECT id FROM games WHERE code = 'sf6')",
		alias, sqlStr(code), alias)
}

func writeMovesInsert(b *strings.Builder, code string, rows []MoveRow) {
	b.WriteString("INSERT INTO moves (character_id, code, category, damage, startup, active, total, on_hit, on_block, recovery, is_aerial, raw_data)\n")
	b.WriteString("SELECT c.id, v.code, v.category, v.damage, v.startup, v.active, v.total, v.on_hit, v.on_block, v.recovery, v.is_aerial, v.raw_data\n")
	b.WriteString("FROM characters c\n")
	b.WriteString("CROSS JOIN (\n")
	for i, r := range rows {
		prefix := "  UNION ALL SELECT "
		if i == 0 {
			prefix = "        SELECT "
		}
		b.WriteString(prefix)
		b.WriteString(sqlStr(r.MoveCode))
		if i == 0 {
			b.WriteString(" AS code")
		}
		writeCol(b, i, sqlStr(r.Category), "category")
		writeCol(b, i, intSQL(r.Damage), "damage")
		writeCol(b, i, intSQL(r.Startup), "startup")
		writeCol(b, i, intSQL(r.Active), "active")
		writeCol(b, i, intSQL(r.Total), "total")
		writeCol(b, i, intSQL(r.OnHit), "on_hit")
		writeCol(b, i, intSQL(r.OnBlock), "on_block")
		writeCol(b, i, intSQL(r.Recovery), "recovery")
		writeCol(b, i, boolSQL(r.IsAerial), "is_aerial")
		writeCol(b, i, rawDataSQL(r), "raw_data")
		b.WriteString("\n")
	}
	fmt.Fprintf(b, ") AS v\nWHERE %s;\n\n", charFilter("c", code))
}

// writeCol は 1 列を書く。先頭行のみ AS 別名を付ける。
func writeCol(b *strings.Builder, rowIdx int, val, name string) {
	b.WriteString(", ")
	b.WriteString(val)
	if rowIdx == 0 {
		b.WriteString(" AS ")
		b.WriteString(name)
	}
}

func writeOriginalMoveIDUpdate(b *strings.Builder, code string, rows []MoveRow) {
	var rush []MoveRow
	for _, r := range rows {
		if r.isRush() {
			rush = append(rush, r)
		}
	}
	if len(rush) == 0 {
		return
	}
	b.WriteString("-- rush_variant の original_move_id を元技 code から解決\n")
	b.WriteString("UPDATE moves SET original_move_id = (\n")
	b.WriteString("    SELECT b.id FROM moves b WHERE b.character_id = moves.character_id AND b.code = CASE moves.code\n")
	for _, r := range rush {
		fmt.Fprintf(b, "        WHEN %s THEN %s\n", sqlStr(r.MoveCode), sqlStr(r.OriginalMoveCode))
	}
	b.WriteString("    END)\n")
	fmt.Fprintf(b, "WHERE character_id IN (SELECT c.id FROM characters c WHERE %s)\n", charFilter("c", code))
	b.WriteString("  AND code IN (")
	writeCodeList(b, rush)
	b.WriteString(");\n\n")
}

// writeAliasInsert は official_ja_move のエイリアス INSERT を書く。
//
// ★character_id を出すのは M20-03 以降の必須事項である(同 §4.5)。出さないと
// character_id が NULL の行が入り、UNIQUE(preset_id, character_id, alias_text) を
// すり抜ける——落ちないため気づけない。format が旧形式なのは、適用済み 6 ファイルとの
// byte 一致を主張する golden テストのときだけである(format.go の FormatPreM2003)。
func writeAliasInsert(b *strings.Builder, code string, rows []MoveRow, format SQLFormat) {
	if format.includesCharacterID() {
		b.WriteString("INSERT INTO preset_aliases (preset_id, move_id, character_id, alias_text)\n")
		b.WriteString("SELECT (SELECT id FROM presets WHERE code = 'official_ja_move'), m.id, m.character_id, v.alias_text\n")
	} else {
		b.WriteString("INSERT INTO preset_aliases (preset_id, move_id, alias_text)\n")
		b.WriteString("SELECT (SELECT id FROM presets WHERE code = 'official_ja_move'), m.id, v.alias_text\n")
	}
	b.WriteString("FROM moves m\nJOIN characters c ON c.id = m.character_id\n")
	b.WriteString("CROSS JOIN (\n")
	for i, r := range rows {
		prefix := "  UNION ALL SELECT "
		if i == 0 {
			prefix = "        SELECT "
		}
		b.WriteString(prefix)
		b.WriteString(sqlStr(r.MoveCode))
		if i == 0 {
			b.WriteString(" AS code")
		}
		b.WriteString(", ")
		b.WriteString(sqlStr(r.NameJA))
		if i == 0 {
			b.WriteString(" AS alias_text")
		}
		b.WriteString("\n")
	}
	fmt.Fprintf(b, ") AS v\nWHERE %s AND m.code = v.code;\n\n", charFilter("c", code))
}

func buildCharDown(code string, rows []MoveRow) string {
	var b strings.Builder
	fmt.Fprintf(&b, "-- ===== %s =====\n", code)
	// alias を先に削除(FK: preset_aliases.move_id → moves.id)。
	b.WriteString("DELETE FROM preset_aliases WHERE move_id IN (\n")
	fmt.Fprintf(&b, "    SELECT m.id FROM moves m JOIN characters c ON c.id = m.character_id WHERE %s AND m.code IN (", charFilter("c", code))
	writeCodeList(&b, rows)
	b.WriteString("));\n")
	// moves 削除(この波が投入した code のみ。移動 system move は CSV に無いので対象外)。
	fmt.Fprintf(&b, "DELETE FROM moves WHERE character_id IN (SELECT c.id FROM characters c WHERE %s)\n", charFilter("c", code))
	b.WriteString("  AND code IN (")
	writeCodeList(&b, rows)
	b.WriteString(");\n\n")
	return b.String()
}

func writeCodeList(b *strings.Builder, rows []MoveRow) {
	for i, r := range rows {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(sqlStr(r.MoveCode))
	}
}

// ---- 値フォーマット ----

// sqlStr は SQL 文字列リテラルへ変換する(単一引用符を二重化)。
func sqlStr(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

func intSQL(p *int) string {
	if p == nil {
		return "NULL"
	}
	return strconv.Itoa(*p)
}

func boolSQL(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

func intStr(p *int) string {
	if p == nil {
		return "NULL"
	}
	return strconv.Itoa(*p)
}

// rawDataSQL は notes/notes_tool から raw_data JSON リテラルを作る(空→NULL)。
func rawDataSQL(r MoveRow) string {
	rd := struct {
		Notes     string `json:"notes,omitempty"`
		NotesTool string `json:"notes_tool,omitempty"`
	}{Notes: r.Notes, NotesTool: r.NotesTool}
	buf, err := json.Marshal(rd)
	if err != nil || string(buf) == "{}" {
		return "NULL"
	}
	return sqlStr(string(buf))
}
