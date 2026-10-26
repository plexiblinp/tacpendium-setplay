package seedgen

import (
	"fmt"
	"sort"
	"strings"
)

// 本ファイルは M43-08(アルジュンの投入・マイグレ 000015)の手書き分を出す。
//
//   - M4308MigrationPreamble … 000015 の冒頭のコメント(「何を戻さないか」を含む)
//   - ArjunCharacterSQL      … characters の 1 行(custom_states の states ＋ safe_jump を同じ行に持つ)
//   - ArjunSystemMovesSQL    … CSV に無い system move 10 code(移動 9 code ＋ drive_reversal)と official_ja_move の別名
//   - ArjunSeedColumnsSQL    … seedgen が SQL へ出さない列(is_projectile ・startup_basis ・fastest_unreachable ・
//     chain_cancel_total ・first_hit_startup)を CSV から当てる UPDATE
//   - ArjunDerivationsSQL    … move_derivations の組(13 組)
//
// ★生成器の規則(FormatCurrent 等)は変えない。CSV から出せる分(moves ・is_derived ・move_commands ・numeric / srk の
// 別名)は既存の生成器をそのまま呼ぶ(internal/seedgen の generate_m4308_test.go が 1 本へ束ねる。000014 と同じ型)。
// ★値の出どころ: 移動系の total ・連キャン ・安全飛びは開発者の実測(character_data/seed-progress.md の
// §アルジュン投入波の申し送り)。drive_reversal は 000004 の 31 行がすべて同じ値(damage 500 ・on_block -6 ・recovery 27)。

// M4308Stem は M43-08 のマイグレの stem である。
const M4308Stem = "000015_data_seed_arjun"

// arjunCode はアルジュンの character_code である。
const arjunCode = "arjun"

// arjunCharacter は characters 行の値である(表示名はインゲームの表記)。
//
// ★custom_states は開発者の Plan の回答(2026-10-18)＝1 状態＋選択肢(ダルシムの yoga_arch_is_set と同じ型)。
// 状態の名前と選択肢の日本語は開発者の言葉のまま。★code ・英名は推測(開発者の指定が無い)。
// ★safe_jump は 000010 の規則(true＝jump_forward の total ・f5＝−1 ・f6＝−2)。
const (
	arjunNameJA       = "アルジュン"
	arjunNameEN       = "Arjun"
	arjunCustomStates = `{"states":[{"code":"divine_pillar_is_set","name_ja":"神気の巨巌接地","name_en":"Divine Pillar is set",` +
		`"subject":"self","scope":"persistent","type":"level","value_definition":{"kind":"integer","min":0,"max":3,"single_value":true,` +
		`"options":[{"value":0,"label_ja":"なし","label_en":"None"},` +
		`{"value":1,"label_ja":"神気の巨巌接地中（発動前）","label_en":"Set (before activation)"},` +
		`{"value":2,"label_ja":"OD神気の巨巌接地中（発動前）","label_en":"OD set (before activation)"},` +
		`{"value":3,"label_ja":"神気の巨巌接地中","label_en":"Set"}]}}],` +
		`"safe_jump":{"true":43,"f5":42,"f6":41}}`
)

// arjunSystemMove は CSV に無い system move の 1 行である。
type arjunSystemMove struct {
	code     string
	nameJA   string // official_ja_move の別名(000008 の既存キャラと同じ文言)
	damage   *int
	total    *int
	onBlock  *int
	recovery *int
}

func m4308IntPtr(n int) *int { return &n }

// arjunSystemMoves は移動 9 code ＋ drive_reversal である(000004 の並び・値の形に合わせる)。
//
// ★移動 5 code の total は開発者の実測(2026-10-17)。micro_* ・forward ・back は既存のキャラと同じく値なし。
var arjunSystemMoves = []arjunSystemMove{
	{code: "forward", nameJA: "前入力"},
	{code: "back", nameJA: "後ろ入力"},
	{code: "micro_forward", nameJA: "微歩き(前)"},
	{code: "micro_back", nameJA: "微歩き(後)"},
	{code: "dash_forward", nameJA: "前方ステップ", total: m4308IntPtr(22)},
	{code: "dash_back", nameJA: "後方ステップ", total: m4308IntPtr(25)},
	{code: "jump_neutral", nameJA: "垂直ジャンプ", total: m4308IntPtr(43)},
	{code: "jump_forward", nameJA: "前ジャンプ", total: m4308IntPtr(43)},
	{code: "jump_back", nameJA: "後ろジャンプ", total: m4308IntPtr(43)},
	{code: "drive_reversal", nameJA: "ドライブリバーサル", damage: m4308IntPtr(500), onBlock: m4308IntPtr(-6), recovery: m4308IntPtr(27)},
}

// arjunDerivations は move_derivations の組(子 → 親)。出どころは character_data/seed-progress.md の申し送りの表
// (notes_tool と公式の条件が一致・開発者確認済み)。
var arjunDerivations = [][2]string{
	{"justice_beyond_light", "cruel_shoulder_light"},
	{"justice_beyond_medium", "cruel_shoulder_medium"},
	{"justice_beyond_heavy", "cruel_shoulder_heavy"},
	{"justice_beyond_od", "cruel_shoulder_od"},
	{"breathing_shockwave_light", "breathing_ring"},
	{"breathing_shockwave_medium", "breathing_ring"},
	{"breathing_shockwave_heavy", "breathing_ring"},
	{"breathing_shockwave_od", "breathing_ring_od"},
	{"moral_breakdown", "indomitable_defense"},
	{"follow_up", "arm_of_justice_light"},
	{"follow_up", "arm_of_justice_medium"},
	{"follow_up", "arm_of_justice_heavy"},
	{"follow_up_od", "arm_of_justice_od"},
}

// ArjunDerivationPairs は move_derivations の組の写しを返す(テストが CSV と DB を照らすため)。
func ArjunDerivationPairs() [][2]string {
	out := make([][2]string, len(arjunDerivations))
	copy(out, arjunDerivations)
	return out
}

// ArjunSystemMoveCodes は CSV に無い system move の code を 000004 の並びで返す。
func ArjunSystemMoveCodes() []string {
	out := make([]string, len(arjunSystemMoves))
	for i, s := range arjunSystemMoves {
		out[i] = s.code
	}
	return out
}

// ArjunCustomStates は characters.custom_states に入れる JSON である。
func ArjunCustomStates() string { return arjunCustomStates }

// M4308SectionHeader は 000015 の塊の見出しを返す(生成器の header 引数に渡す)。
func M4308SectionHeader(title string) func(dir string) string {
	return func(string) string { return "-- ===== " + title + " =====\n" }
}

// M4308MigrationPreamble は 000015 の冒頭のコメントである。
//
// ★「何を戻さないか」をここへ書く(playbook §4.17)。
func M4308MigrationPreamble(dir string) string {
	h := "-- " + M4308Stem + "." + dir + ".sql\n" +
		"-- M43-08: SF6 の 2026-10-13 の更新で追加された新キャラ アルジュン(arjun)を配布物へ入れる。\n" +
		"-- ★層 B(CC-BY-SA-4.0)。SF6 の事実であり GAME_TABLES へ書くため `_data_` を持つ。\n" +
		"-- ★スキーマ変更なし。★既存のマイグレ(000001〜000014)と既存のキャラの行は書き換えない(アルジュンの行だけを足す)。\n" +
		"-- 本ファイルは internal/seedgen が character_data/arjun.csv から生成した成果物(手編集しない)。\n" +
		"--   CSV から出す分: moves(＋official_ja_move の別名 ・rush の original_move_id)・is_derived ・move_commands ・\n" +
		"--   numeric / srk の別名(既存の生成器をそのまま呼ぶ)。\n" +
		"--   手書きの分(generate_m4308.go の表): characters 行 ・移動 9 code ＋ drive_reversal とその別名 ・\n" +
		"--   seedgen が SQL へ出さない列(is_projectile ・startup_basis ・fastest_unreachable ・chain_cancel_total ・\n" +
		"--   first_hit_startup。値は CSV)・move_derivations。\n" +
		"--   再生成: TACPENDIUM_UPDATE_GOLDEN=1 go test ./internal/seedgen/ -run TestM4308_MigrationIsGenerated\n" +
		"--   ★適用済み(internal/migrationpin/applied.sha256 に載った後)は再生成しない(character_data/seed-progress.md)。\n" +
		"-- FK 依存順 characters → moves → move_commands / preset_aliases / move_derivations。\n"
	if dir == "down" {
		h += "-- down は up の逆である(FK の逆順に、アルジュンの行だけを消す)。\n" +
			"-- ★忠実に戻せないもの:\n" +
			"--   (1) sqlite_sequence(characters ・moves 等の AUTOINCREMENT の採番の位置)は戻さない(次の採番が飛ぶだけで、行は戻る)。\n" +
			"--   (2) 利用者が up の後にアルジュンで作った行(コンボ ・セットプレイ ・メインキャラの設定等)は消さない。\n" +
			"--       マイグレは FK=OFF の接続で走るため down は止まらず、それらの行の character_id ／ move_id は\n" +
			"--       宙吊りになる(既存の生成器の down と同じ扱い)。カスタムプリセットの別名だけは、アルジュンの技を指す行を消す。\n"
	}
	return h + "\n"
}

// ArjunCharacterSQL は characters の 1 行を足す(up)／消す(down)。★moves より前に置く
// (後だと以降の INSERT ... SELECT が 0 行のまま成功する)。
func ArjunCharacterSQL(dir string) string {
	var b strings.Builder
	b.WriteString("-- ===== characters: arjun =====\n")
	if dir == "up" {
		b.WriteString("INSERT INTO characters (game_id, code, name_ja, name_en, custom_states)\n")
		fmt.Fprintf(&b, "SELECT g.id, %s, %s, %s, %s\nFROM games g WHERE g.code = 'sf6';\n\n",
			sqlStr(arjunCode), sqlStr(arjunNameJA), sqlStr(arjunNameEN), sqlStr(arjunCustomStates))
	} else {
		fmt.Fprintf(&b, "DELETE FROM characters WHERE %s;\n\n", charFilterBare(arjunCode))
	}
	return b.String()
}

// charFilterBare は表名を付けない charFilter である(DELETE FROM characters の WHERE 用)。
func charFilterBare(code string) string {
	return fmt.Sprintf("code = %s AND game_id IN (SELECT id FROM games WHERE code = 'sf6')", sqlStr(code))
}

// ArjunSystemMovesSQL は CSV に無い system move 10 code と official_ja_move の別名を足す(up)／消す(down)。
//
// ★numeric / srk の移動系の別名は GenerateAliases(-movement-chars arjun)が出す。drive_reversal の別名は
// 000004 / 000008 の既存キャラと同じく official_ja_move だけである。
func ArjunSystemMovesSQL(dir string) string {
	var b strings.Builder
	b.WriteString("-- ===== moves: arjun の system move 10 code(移動 9 code ＋ drive_reversal。CSV に無い) =====\n")
	codes := make([]string, len(arjunSystemMoves))
	for i, s := range arjunSystemMoves {
		codes[i] = sqlStr(s.code)
	}
	codeList := strings.Join(codes, ", ")
	if dir == "down" {
		b.WriteString("DELETE FROM preset_aliases WHERE move_id IN (\n")
		fmt.Fprintf(&b, "    SELECT m.id FROM moves m JOIN characters c ON c.id = m.character_id WHERE %s AND m.code IN (%s));\n",
			charFilter("c", arjunCode), codeList)
		fmt.Fprintf(&b, "DELETE FROM moves WHERE character_id IN (SELECT c.id FROM characters c WHERE %s)\n  AND code IN (%s);\n\n",
			charFilter("c", arjunCode), codeList)
		return b.String()
	}
	b.WriteString("INSERT INTO moves (character_id, code, category, damage, total, on_block, recovery)\n")
	b.WriteString("SELECT c.id, v.code, 'system', v.damage, v.total, v.on_block, v.recovery\n")
	b.WriteString("FROM characters c\nCROSS JOIN (\n")
	for i, s := range arjunSystemMoves {
		prefix := "  UNION ALL SELECT "
		if i == 0 {
			prefix = "        SELECT "
		}
		b.WriteString(prefix)
		b.WriteString(sqlStr(s.code))
		if i == 0 {
			b.WriteString(" AS code")
		}
		writeCol(&b, i, intSQL(s.damage), "damage")
		writeCol(&b, i, intSQL(s.total), "total")
		writeCol(&b, i, intSQL(s.onBlock), "on_block")
		writeCol(&b, i, intSQL(s.recovery), "recovery")
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, ") AS v\nWHERE %s;\n\n", charFilter("c", arjunCode))

	b.WriteString("INSERT INTO preset_aliases (preset_id, move_id, character_id, alias_text)\n")
	b.WriteString("SELECT (SELECT id FROM presets WHERE code = 'official_ja_move'), m.id, m.character_id, v.alias_text\n")
	b.WriteString("FROM moves m\nJOIN characters c ON c.id = m.character_id\nCROSS JOIN (\n")
	for i, s := range arjunSystemMoves {
		prefix := "  UNION ALL SELECT "
		if i == 0 {
			prefix = "        SELECT "
		}
		b.WriteString(prefix)
		b.WriteString(sqlStr(s.code))
		if i == 0 {
			b.WriteString(" AS code")
		}
		b.WriteString(", ")
		b.WriteString(sqlStr(s.nameJA))
		if i == 0 {
			b.WriteString(" AS alias_text")
		}
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, ") AS v\nWHERE %s AND m.code = v.code;\n\n", charFilter("c", arjunCode))
	return b.String()
}

// ArjunSeedColumnsSQL は seedgen が SQL へ出さない 5 列を CSV の値で当てる UPDATE を返す(up だけ。
// down は行ごと消すので要らない)。
//
// ★値は CSV が一次源(seed-progress.md の #6 小節＝両列が埋まっている波は CSV を使う。機械規則は使わない)。
// ★既定値(is_projectile=0 ・startup_basis='unknown' ・fastest_unreachable=0 ・NULL)と同じ値の行は出さない。
// ★1 行ずつの UPDATE は `WHERE <列> IS NULL` を付ける(SUPP-001 §5.5.2 (4))。
func ArjunSeedColumnsSQL(rows []MoveRow) string {
	var b strings.Builder
	b.WriteString("-- ===== moves: arjun の seedgen が SQL へ出さない列(値は CSV) =====\n")
	where := fmt.Sprintf("WHERE character_id IN (SELECT c.id FROM characters c WHERE %s)", charFilter("c", arjunCode))

	group := func(set, cond string, pick func(MoveRow) bool) {
		var codes []string
		for _, r := range rows {
			if !r.isMovementSystem() && pick(r) {
				codes = append(codes, sqlStr(r.MoveCode))
			}
		}
		if len(codes) == 0 {
			return
		}
		fmt.Fprintf(&b, "UPDATE moves SET %s\n%s\n  AND %s AND code IN (%s);\n", set, where, cond, strings.Join(codes, ", "))
	}
	group("is_projectile = 1", "is_projectile = 0", func(r MoveRow) bool { return r.IsProjectile })
	bases := map[string]bool{}
	for _, r := range rows {
		if r.StartupBasis != "" && r.StartupBasis != "unknown" {
			bases[r.StartupBasis] = true
		}
	}
	var basisOrder []string
	for k := range bases {
		basisOrder = append(basisOrder, k)
	}
	sort.Strings(basisOrder)
	for _, basis := range basisOrder {
		basis := basis
		group("startup_basis = "+sqlStr(basis), "startup_basis = 'unknown'", func(r MoveRow) bool { return r.StartupBasis == basis })
	}
	group("fastest_unreachable = 1", "fastest_unreachable = 0", func(r MoveRow) bool { return r.FastestUnreachable })
	for _, r := range rows {
		if r.ChainCancelTotal != nil {
			fmt.Fprintf(&b, "UPDATE moves SET chain_cancel_total = %d\n%s\n  AND code = %s AND chain_cancel_total IS NULL;\n",
				*r.ChainCancelTotal, where, sqlStr(r.MoveCode))
		}
	}
	for _, r := range rows {
		if r.FirstHitStartup != nil {
			fmt.Fprintf(&b, "UPDATE moves SET first_hit_startup = %d\n%s\n  AND code = %s AND first_hit_startup IS NULL;\n",
				*r.FirstHitStartup, where, sqlStr(r.MoveCode))
		}
	}
	b.WriteString("\n")
	return b.String()
}

// ArjunDerivationsSQL は move_derivations の組(13 組)を足す(up)／その組だけを消す(down)。
//
// ★KimberlySprintDerivationsSQL(000014)と同じ形。★down はキャラ単位で消さない(組で限定する)。
func ArjunDerivationsSQL(dir string) string {
	pairs := make([]string, len(arjunDerivations))
	for i, p := range arjunDerivations {
		pairs[i] = fmt.Sprintf("        (%s, %s)", sqlStr(p[0]), sqlStr(p[1]))
	}
	values := strings.Join(pairs, ",\n")
	var b strings.Builder
	fmt.Fprintf(&b, "-- ===== move_derivations: arjun の %d 組 =====\n", len(arjunDerivations))
	sel := "SELECT child.id, parent.id\n" +
		"FROM moves child\n" +
		"JOIN moves parent ON parent.character_id = child.character_id\n" +
		"JOIN characters c ON c.id = child.character_id\n" +
		"WHERE " + charFilter("c", arjunCode) + "\n" +
		"  AND (child.code, parent.code) IN (VALUES\n" + values + "\n      )"
	if dir == "up" {
		b.WriteString("INSERT INTO move_derivations (child_move_id, parent_move_id)\n")
		b.WriteString(sel + "\n")
		b.WriteString("  AND NOT EXISTS (SELECT 1 FROM move_derivations d\n" +
			"      WHERE d.child_move_id = child.id AND d.parent_move_id = parent.id);\n\n")
	} else {
		b.WriteString("DELETE FROM move_derivations WHERE (child_move_id, parent_move_id) IN (\n")
		b.WriteString(sel + ");\n\n")
	}
	return b.String()
}
