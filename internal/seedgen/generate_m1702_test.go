package seedgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// genRows は 1 キャラ分の CSV 本文(ヘッダ抜き)を MoveRow 列へ読む小道具(M17-02 生成系用)。
func genRows(t *testing.T, body string) []MoveRow {
	t.Helper()
	rows, err := Read(strings.NewReader(testHeader+body), 0)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	return rows
}

func TestGenerateDerivedBackfill_UpdatesOnlyDerivedCodes(t *testing.T) {
	body := "terry,standing_light_punch,normal,立ち弱P,4,3,7,13,4,-1,300,false,false,false,,,,p_l,,,,,,,\n" +
		"terry,power_drive,target_combo,パワードライブ,15,3,22,39,,-5,1470,false,false,true,,,,p_m chain p_h,,,,,,,\n" +
		"terry,rush_standing_light_punch,rush_variant,立ち弱P(ラッシュ),15,3,7,24,8,3,300,false,false,true,,,standing_light_punch,,,,,,,,\n"
	res, err := GenerateDerivedBackfill([]string{"terry"}, map[string][]MoveRow{"terry": genRows(t, body)},
		BackfillHeader("000034_test", ""))
	if err != nil {
		t.Fatalf("GenerateDerivedBackfill: %v", err)
	}

	// up: is_derived=true の 2 code のみを =1 で UPDATE(非派生は列挙されない)。
	if !strings.Contains(res.UpSQL, "UPDATE moves SET is_derived = 1") ||
		!strings.Contains(res.UpSQL, "'power_drive', 'rush_standing_light_punch'") {
		t.Errorf("up が期待形でない:\n%s", res.UpSQL)
	}
	if strings.Contains(res.UpSQL, "'standing_light_punch'") {
		t.Errorf("非派生 code が backfill に列挙されている:\n%s", res.UpSQL)
	}
	// down: 同じ code 列挙を =0 へ戻す。
	if !strings.Contains(res.DownSQL, "UPDATE moves SET is_derived = 0") ||
		!strings.Contains(res.DownSQL, "'power_drive', 'rush_standing_light_punch'") {
		t.Errorf("down が期待形でない:\n%s", res.DownSQL)
	}
	if res.Stats["terry"] != 2 {
		t.Errorf("Stats[terry] = %d, want 2", res.Stats["terry"])
	}
}

func TestGenerateMoveCommands_NormalizedKeysAndSkips(t *testing.T) {
	body := "terry,crouching_medium_punch,normal,しゃがみ中P,5,3,11,18,5,-2,600,false,false,false,,,,d plus p_m,,,,,,,\n" +
		"terry,power_wave_light,special,弱パワーウェイブ,14,36,0,49,-3,-9,600,false,true,false,,,,d dr r plus p_l,,,,,,,\n" +
		"terry,buster_wolf,super_art,バスターウルフ,9,4,44,56,,-22,2000,false,false,false,,,,d dr r d dr r plus k,,,,,,,\n" +
		"terry,jumping_light_punch,normal,ジャンプ弱P,5,6,0,10,,,300,true,false,false,,,,p_l,,,,,,,\n" +
		"terry,power_drive,target_combo,パワードライブ,15,3,22,39,,-5,1470,false,false,true,,,,p_m chain p_h,,,,,,,\n" +
		"terry,rush_standing_light_punch,rush_variant,立ち弱P(ラッシュ),15,3,7,24,8,3,300,false,false,true,,,standing_light_punch,,,,,,,,\n"
	res, err := GenerateMoveCommands([]string{"terry"}, map[string][]MoveRow{"terry": genRows(t, body)},
		MoveCommandsHeader("000035_test", ""))
	if err != nil {
		t.Fatalf("GenerateMoveCommands: %v", err)
	}
	up := res.UpSQL

	// numpad 正規化済みキーで INSERT される(出力順は TokenKey 昇順=Entries の決定論順序)。
	for _, want := range []string{
		"'crouching_medium_punch', '2MP'",
		"'power_wave_light', '236LP'",
		// 索引は広く持つ: 必殺技(モーション)・空中技も搭載(M17-04 用・§1.5-6)。
		"'buster_wolf' AS code, '236236K' AS token_key",
		"'jumping_light_punch', 'LP'",
	} {
		if !strings.Contains(up, want) {
			t.Errorf("up に %q が無い:\n%s", want, up)
		}
	}
	// 派生技(target_combo の派生・rush_variant)は搭載しない。
	for _, ng := range []string{"power_drive", "rush_standing_light_punch"} {
		if strings.Contains(up, ng) {
			t.Errorf("派生技 %q が索引 seed に混入:\n%s", ng, up)
		}
	}
	// down は搭載 code のみの精密 DELETE。
	if !strings.Contains(res.DownSQL, "DELETE FROM move_commands WHERE move_id IN (") ||
		!strings.Contains(res.DownSQL, "'crouching_medium_punch'") {
		t.Errorf("down が期待形でない:\n%s", res.DownSQL)
	}
	if res.Stats["terry"] != 4 {
		t.Errorf("Stats[terry] = %d, want 4", res.Stats["terry"])
	}
	if got := len(res.Index.Skipped()); got != 2 {
		t.Errorf("Skipped = %d, want 2 (derived 2 件)", got)
	}
}

// m1702CharOrder は 000034/000035 生成時の対象キャラ順(第一波 9+ryu。manon は対象外=M17-02)。
var m1702CharOrder = append(append([]string{}, FirstWaveOrder...), "ryu")

// 000034/000035 生成時に cmd/seedgen の -note へ渡した説明行(再生成コマンドは各 golden のコメント)。
const (
	backfillMigrationNote     = "M17-02: 投入済み 10 キャラの is_derived backfill(CHANGE-069 §2.1-d。CSV に載る code のみ UPDATE)"
	moveCommandsMigrationNote = "M17-02: command 索引 move_commands の seed(非派生のみ・CHANGE-069 §2.1-b)"
)

// readM1702Rows は 10 キャラ分の CSV を生成時と同一順・通し RowIndex で読む。
func readM1702Rows(t *testing.T) map[string][]MoveRow {
	t.Helper()
	root := repoRoot(t)
	rowsByChar := map[string][]MoveRow{}
	next := 0
	for _, code := range m1702CharOrder {
		rows, err := ReadFile(filepath.Join(root, "character_data", code+".csv"), next)
		if err != nil {
			t.Fatalf("ReadFile %s: %v", code, err)
		}
		rowsByChar[code] = rows
		next += len(rows)
	}
	return rowsByChar
}

// goldenDir は凍結 golden の置き場。
//
// ★★★M33-03 で比較先を migrations/ から testdata/ へ移した。理由は構造的である——
// M33-02 が旧 111 本を新系列 9 本へ潰したため、**byte 一致の再建が原理的に不可能に
// なった。** seedgen は `INSERT ... SELECT ... CROSS JOIN` 形式・12 列を出すが、
// 新 000004_data_seed_moves は id 明示の VALUES タプル 22 列であり、さらに自己参照
// 514 行を「NULL で入れてから UPDATE」の 2 パスで解いている。⇒ seedgen にその生成能力は無い。
// ★さらに is_derived backfill 系 4 stem は、対応先の SQL ファイルそのものが存在しない
// （列値として 000004 に溶けた）。⇒ stem 単位の対応が原理的に付かない。
//
// ★★では何を守っているのか＝「CSV(正本) → SQL の変換規則がドリフトしていない」ことである。
// ⇒ 着手時点の golden も、実質その主張を「コミット済みマイグレ」を写しとして使って
// 見ていた。写しの置き場が変わっただけであり、主張は 1 つも減っていない。
//
// ★★★減った主張が 1 つある＝「配布されるシードが、この変換規則の出力と一致する」。
// ⇒ こちらは TestSeedMatchesCSV_MovesAndCommands（generate_csvdb_test.go）が
//
//	*意味* で受け持つ。byte ではなく行の集合で突き合わせる。
const goldenDir = "testdata"

// goldenUpdateEnv を立てて走らせると、golden を現在の生成結果で書き直す。
//
//	TACPENDIUM_UPDATE_GOLDEN=1 go test ./internal/seedgen/
//
// ★★再生成の手順を機構として置く理由(M33-03)——着手時点の再生成手順は各テストの
// doc コメントに `go run ./cmd/seedgen -mode ... -out ... -note "<定数の文字列>"` として
// 書かれており、**note を Go の定数から人が転記する**形だった。⇒ 転記を誤ると
// golden だけが動く。★ここを通せば、生成に使う引数はテスト本体と同一になる。
const goldenUpdateEnv = "TACPENDIUM_UPDATE_GOLDEN"

// assertGolden は凍結 golden との byte-identical を検証する。
//
// ★★歯止めの限界(M33-03 レビュー 低-2)——TACPENDIUM_UPDATE_GOLDEN は *無条件に上書きする*。
// ⇒ 「なぜ変わったか」を特定したかどうかを機械では見ていない。★守っているのは
//
//	「git diff に出る」ことだけである。⇒ レビューで差分を読むのが最後の砦である。
func assertGolden(t *testing.T, stem, upSQL, downSQL string) {
	t.Helper()
	root := repoRoot(t)
	dir := filepath.Join(root, "internal", "seedgen", goldenDir)
	for _, c := range []struct{ name, want string }{
		{stem + ".up.sql", upSQL},
		{stem + ".down.sql", downSQL},
	} {
		path := filepath.Join(dir, c.name)
		if os.Getenv(goldenUpdateEnv) != "" {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatalf("mkdir %s: %v", dir, err)
			}
			if err := os.WriteFile(path, []byte(c.want), 0o644); err != nil {
				t.Fatalf("write %s: %v", path, err)
			}
			t.Logf("golden 更新: %s", c.name)
			continue
		}
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v (★golden が無い。%s=1 で生成できる)", c.name, err, goldenUpdateEnv)
		}
		if string(got) != c.want {
			// ★落ちたら、CSV 正本と変換規則のどちらが変わったかを先に特定すること。
			//   ⇒ 「%s=1 で上書きする」は*特定した後*の手順である。
			t.Errorf("%s が凍結 golden と不一致。CSV 正本と変換規則のどちらが変わったかを"+
				"特定してから %s=1 で更新すること", c.name, goldenUpdateEnv)
		}
	}
}

// TestGolden_DerivedBackfillMatchesRegeneration は 10 キャラ CSV から再生成した SQL が
// コミット済みの 000034 と一致することを保証する(手編集ドリフト検出)。再生成は TACPENDIUM_UPDATE_GOLDEN=1 go test ./internal/seedgen/ で行う
// (★M33-03。下の CLI は生成に使った引数の記録であり、出力先は migrations/ ではなく
//
//	 internal/seedgen/testdata/ である):
//
//		go run ./cmd/seedgen -mode derived-backfill -chars terry,guile,lily,ingrid,kimberly,juri,ken,mai,zangief,ryu \
//		  -out 000034_backfill_moves_is_derived -note "<backfillMigrationNote の文字列>"
func TestGolden_DerivedBackfillMatchesRegeneration(t *testing.T) {
	res, err := GenerateDerivedBackfill(m1702CharOrder, readM1702Rows(t),
		BackfillHeader("000034_backfill_moves_is_derived", backfillMigrationNote))
	if err != nil {
		t.Fatalf("GenerateDerivedBackfill: %v", err)
	}
	assertGolden(t, "000034_backfill_moves_is_derived", res.UpSQL, res.DownSQL)
}

// TestGolden_MoveCommandsMatchesRegeneration は 10 キャラ CSV から再生成した SQL が
// コミット済みの 000035 と一致することを保証する(手編集ドリフト検出)。再生成は TACPENDIUM_UPDATE_GOLDEN=1 go test ./internal/seedgen/ で行う
// (★M33-03。下の CLI は生成に使った引数の記録であり、出力先は migrations/ ではなく
//
//	 internal/seedgen/testdata/ である):
//
//		go run ./cmd/seedgen -mode move-commands -chars terry,guile,lily,ingrid,kimberly,juri,ken,mai,zangief,ryu \
//		  -out 000035_seed_move_commands -note "<moveCommandsMigrationNote の文字列>"
func TestGolden_MoveCommandsMatchesRegeneration(t *testing.T) {
	res, err := GenerateMoveCommands(m1702CharOrder, readM1702Rows(t),
		MoveCommandsHeader("000035_seed_move_commands", moveCommandsMigrationNote))
	if err != nil {
		t.Fatalf("GenerateMoveCommands: %v", err)
	}
	assertGolden(t, "000035_seed_move_commands", res.UpSQL, res.DownSQL)
}
