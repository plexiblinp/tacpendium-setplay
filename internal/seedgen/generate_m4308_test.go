package seedgen

import (
	"strings"
	"testing"
)

// M43-08(アルジュンの投入・マイグレ 000015)の生成器テスト。
//
//   - 生成物: 000015 の up / down が、今の CSV ＋ 既存の生成器 ＋ generate_m4308.go の手書きの表から作り直したものと
//     byte 一致する(★未固定のあいだ。固定済みになれば在ることだけを確かめる＝assertGeneratedMigration)
//   - 手書きの表の健全性: 移動系 ・派生の code が CSV に在る(INSERT ... SELECT は 0 行でも成功する)・自己参照が無い

// arjunRows は CSV の全キャラを読んだうえでアルジュンの行だけを返す(RowIndex は全キャラの通し番号のまま)。
func arjunRows(t *testing.T) ([]string, map[string][]MoveRow) {
	t.Helper()
	_, all := readAllCharacterRows(t)
	rows := all[arjunCode]
	if len(rows) == 0 {
		t.Fatalf("character_data/%s.csv の行が無い", arjunCode)
	}
	return []string{arjunCode}, map[string][]MoveRow{arjunCode: rows}
}

// renderM4308Migration は 000015 の up / down の全文を作る。
//
// up  ＝ characters → moves(＋official_ja_move ・rush)→ system move 10 code → is_derived → 手書きの列 →
//
//	move_commands → numeric → srk → move_derivations
//
// down＝ その逆順(FK の逆順。最後に characters)。
func renderM4308Migration(t *testing.T) (string, string, map[string]*AliasResult) {
	t.Helper()
	order, rows := arjunRows(t)
	moves, err := GenerateWithHeader(order, rows, M4308SectionHeader("moves: arjun(CSV。official_ja_move の別名 ・rush の original_move_id を含む)"))
	if err != nil {
		t.Fatalf("GenerateWithHeader: %v", err)
	}
	derived, err := GenerateDerivedBackfill(order, rows, M4308SectionHeader("moves.is_derived: arjun(CSV)"))
	if err != nil {
		t.Fatalf("GenerateDerivedBackfill: %v", err)
	}
	commands, err := GenerateMoveCommands(order, rows, M4308SectionHeader("move_commands: arjun(CSV)"))
	if err != nil {
		t.Fatalf("GenerateMoveCommands: %v", err)
	}
	aliases := map[string]*AliasResult{}
	for _, rule := range []AliasPresetRule{NumericRule, SRKRule} {
		res, err := GenerateAliases(rule, order, rows, order, M4308SectionHeader("preset "+rule.PresetCode+": arjun(移動系 9 code ＋ CSV)"))
		if err != nil {
			t.Fatalf("GenerateAliases(%s): %v", rule.PresetCode, err)
		}
		aliases[rule.PresetCode] = res
	}

	up := M4308MigrationPreamble("up") +
		ArjunCharacterSQL("up") +
		moves.UpSQL +
		ArjunSystemMovesSQL("up") +
		derived.UpSQL +
		ArjunSeedColumnsSQL(rows[arjunCode]) +
		commands.UpSQL +
		aliases["numeric"].UpSQL +
		aliases["srk"].UpSQL +
		ArjunDerivationsSQL("up")
	down := M4308MigrationPreamble("down") +
		ArjunDerivationsSQL("down") +
		aliases["srk"].DownSQL +
		aliases["numeric"].DownSQL +
		commands.DownSQL +
		ArjunSystemMovesSQL("down") +
		moves.DownSQL +
		ArjunCharacterSQL("down")
	return up, down, aliases
}

// TestM4308_MigrationIsGenerated は 000015 を固定の一覧の基準で扱う(M43-07 の型)。
//
// ★未固定(作業中)のあいだは今の CSV から作り直して byte 一致で照らす。固定済みになれば在ることだけを確かめる。
func TestM4308_MigrationIsGenerated(t *testing.T) {
	assertGeneratedMigration(t, M4308Stem, func() (string, string) {
		up, down, _ := renderM4308Migration(t)
		return up, down
	})
}

// TestM4308_HandwrittenTablesMatchCSV は手書きの表の code が CSV と矛盾しないことを固定する。
//
// ★INSERT ... SELECT は子や親が居なくても 0 行のまま成功する(サイレント no-op)。⇒ 生成の前に機械で照らす。
// ★今の CSV を読むため、固定後に arjun.csv を直すと赤になりうる(そのときは新しい連番のマイグレと本テストを直す)。
func TestM4308_HandwrittenTablesMatchCSV(t *testing.T) {
	_, rows := arjunRows(t)
	inCSV := map[string]MoveRow{}
	for _, r := range rows[arjunCode] {
		inCSV[r.MoveCode] = r
	}
	// system move 10 code は CSV に無いこと(在れば seedgen の moves と二重に入る)。
	for _, code := range ArjunSystemMoveCodes() {
		if _, ok := inCSV[code]; ok {
			t.Errorf("system move %s が CSV に在る(手書きと二重になる)", code)
		}
	}
	if len(ArjunSystemMoveCodes()) != len(movementSystemCodes)+1 {
		t.Errorf("system move = %d code, want 移動 %d ＋ drive_reversal", len(ArjunSystemMoveCodes()), len(movementSystemCodes))
	}
	// 派生の組: 子 ・親が CSV に在り、自己参照が無く、子は is_derived である。
	children := map[string]bool{}
	for _, p := range ArjunDerivationPairs() {
		child, okC := inCSV[p[0]]
		_, okP := inCSV[p[1]]
		if !okC || !okP {
			t.Errorf("派生の組 %s ← %s の code が CSV に無い(子 %v ・親 %v)", p[0], p[1], okC, okP)
			continue
		}
		if p[0] == p[1] {
			t.Errorf("派生の組 %s が自己参照である", p[0])
		}
		if !child.IsDerived {
			t.Errorf("派生の子 %s が CSV で is_derived=false", p[0])
		}
		children[p[0]] = true
	}
	// 子の集合＝CSV の「is_derived=true かつ rush_variant でない」行(申し送りの表が全数であること)。
	for code, r := range inCSV {
		if r.IsDerived && r.Category != "rush_variant" && !children[code] {
			t.Errorf("CSV で is_derived=true の %s に親の組が無い", code)
		}
	}
	if len(children) != 11 || len(ArjunDerivationPairs()) != 13 {
		t.Errorf("派生 = 子 %d ・組 %d, want 11 ・13(申し送りの表)", len(children), len(ArjunDerivationPairs()))
	}
}

// TestM4308_GenerationMeasurements は 000015 の生成の実測を固定する(完了報告の数と同じ)。
//
// ★本テストは固定の前後を問わず今の CSV から組み直す。⇒ 固定後に arjun.csv を直すと赤になる。そのときは
// 新しい連番のマイグレを起こしたうえで、本テストの期待値も直すこと(000015 は書き換えない)。
func TestM4308_GenerationMeasurements(t *testing.T) {
	up, _, aliases := renderM4308Migration(t)
	if !strings.Contains(up, "INSERT INTO characters") {
		t.Fatal("characters 行が出ていない")
	}
	// characters は moves より前(後だと以降の INSERT ... SELECT が 0 行のまま成功する)。
	if strings.Index(up, "INSERT INTO characters") > strings.Index(up, "INSERT INTO moves") {
		t.Error("characters 行が moves より後ろにある")
	}
	for preset, wantEmitted := range map[string]int{"numeric": 73, "srk": 73} {
		res := aliases[preset]
		if got := len(res.Emitted); got != wantEmitted {
			t.Errorf("%s の投入行 = %d, want %d", preset, got, wantEmitted)
		}
		if got := len(res.Movement); got != len(movementSystemCodes) {
			t.Errorf("%s の移動系 = %d, want %d", preset, got, len(movementSystemCodes))
		}
		if len(res.SAWithoutAnnotation) != 0 {
			t.Errorf("%s: SA 注記の欠落 %d 件", preset, len(res.SAWithoutAnnotation))
		}
	}
}
