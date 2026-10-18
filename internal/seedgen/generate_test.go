package seedgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testHeader = "character_code,move_code,category,name_ja,startup,active,recovery,total,on_hit,on_block,damage,is_aerial,is_projectile,is_derived,notes,notes_tool,original_move_code,command,condition_ja,condition_en,startup_basis,chain_cancel_total,fastest_unreachable,last_changed_game_version,first_hit_startup\n"

// gen は 1 キャラ分の CSV 本文(ヘッダ抜き)から Generate を回す小道具。
func gen(t *testing.T, code, body string) (*Result, error) {
	t.Helper()
	rows, err := Read(strings.NewReader(testHeader+body), 0)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	return Generate([]string{code}, map[string][]MoveRow{code: rows})
}

func TestGenerate_RemapAndRawData(t *testing.T) {
	body := "terry,standing_light_punch,normal,立ち弱P,4,3,7,13,4,-1,300,false,false,false,,,,p_l,,,,,,,\n" +
		"terry,jumping_lariat,target_combo,ジャンプラリアット,24,2,18,43,4,3,1000,false,false,true,地上では繋がらない,,,p_m chain k_m,,,,,,,\n"
	res, err := gen(t, "terry", body)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	up := res.UpSQL
	// 通常技の remap(recovery/is_aerial/damage・raw_data NULL)。
	if !strings.Contains(up, "'standing_light_punch' AS code, 'normal' AS category, 300 AS damage, 4 AS startup, 3 AS active, 13 AS total, 4 AS on_hit, -1 AS on_block, 7 AS recovery, 0 AS is_aerial, NULL AS raw_data") {
		t.Errorf("standing_light_punch の remap が期待形でない:\n%s", up)
	}
	// notes → raw_data JSON。
	if !strings.Contains(up, `'{"notes":"地上では繋がらない"}'`) {
		t.Errorf("notes → raw_data JSON が期待形でない:\n%s", up)
	}
	// target_combo passthrough(category 無改変)。
	if !strings.Contains(up, "'jumping_lariat', 'target_combo'") {
		t.Errorf("target_combo passthrough が崩れている:\n%s", up)
	}
	// alias 対(name_ja)。
	if !strings.Contains(up, "INSERT INTO preset_aliases") || !strings.Contains(up, "'立ち弱P'") {
		t.Errorf("alias 投入が無い:\n%s", up)
	}
}

// TestGenerate_IsProjectileNotEmitted は M18-01 の設計確定(Option A・backfill 専用)を守るため、
// seedgen が is_projectile を生成 SQL へ出力しないことを検証する。列は 000039 で追加・初期値は
// 同マイグレの backfill で投入するため、seed INSERT に is_projectile が現れると
// 000026/000030 は列追加前に実行され no such column で壊れる。
func TestGenerate_IsProjectileNotEmitted(t *testing.T) {
	// is_projectile=true の飛び道具行(startup13+active3-1+recovery20=total35)。
	body := "terry,power_wave_light,special,弱パワーウェイブ,13,3,20,35,,,600,false,true,false,,,,d dr r p_l,,,,,,,\n"
	res, err := gen(t, "terry", body)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if strings.Contains(res.UpSQL, "is_projectile") {
		t.Errorf("生成 up SQL に is_projectile が出力されている(backfill 専用のはず):\n%s", res.UpSQL)
	}
	if strings.Contains(res.DownSQL, "is_projectile") {
		t.Errorf("生成 down SQL に is_projectile が出力されている:\n%s", res.DownSQL)
	}
	// 飛び道具技自体は(is_projectile 列なしで)seed される。
	if !strings.Contains(res.UpSQL, "'power_wave_light'") {
		t.Errorf("飛び道具技が seed されていない:\n%s", res.UpSQL)
	}
}

// TestGenerate_FrameCostColumnsNotEmitted は M19-04 の設計確定(CHANGE-091 §3.1・backfill 専用)を
// 守るため、seedgen が新列 3 つを生成 SQL へ出力しないことを検証する。列は 000049 で追加・機械で
// 決まる分の初期値は 000050 の backfill で投入するため、seed INSERT にこれらが現れると
// 000026/000030/000045 は列追加前に実行され no such column で壊れる(is_projectile と同型)。
func TestGenerate_FrameCostColumnsNotEmitted(t *testing.T) {
	// 新列 3 つに値が入っている行(startup4+active3-1+recovery7=total13)。
	body := "terry,standing_light_punch,normal,立ち弱P,4,3,7,13,4,-1,300,false,false,false,,,,p_l,,,standalone,9,true,,6\n"
	res, err := gen(t, "terry", body)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	// ★M37-04: first_hit_startup も同じ「保全のみ・SQL 非投入」である(列は 000114)。
	for _, col := range []string{"startup_basis", "chain_cancel_total", "fastest_unreachable", "first_hit_startup"} {
		if strings.Contains(res.UpSQL, col) {
			t.Errorf("生成 up SQL に %s が出力されている(backfill 専用のはず):\n%s", col, res.UpSQL)
		}
		if strings.Contains(res.DownSQL, col) {
			t.Errorf("生成 down SQL に %s が出力されている:\n%s", col, res.DownSQL)
		}
	}
	// 値そのもの(standalone)も漏れていないこと。
	if strings.Contains(res.UpSQL, "standalone") {
		t.Errorf("生成 up SQL に startup_basis の値が出力されている:\n%s", res.UpSQL)
	}
	// 技自体は(新列なしで)seed される。
	if !strings.Contains(res.UpSQL, "'standing_light_punch'") {
		t.Errorf("技が seed されていない:\n%s", res.UpSQL)
	}
}

// TestRead_UnknownStartupBasisFails は startup_basis の値域検証(validate 側)を固定する。
// category と同じ分業＝parseRow は型エラーのみ、意味検証は validate。
func TestGenerate_UnknownStartupBasisFails(t *testing.T) {
	body := "terry,standing_light_punch,normal,立ち弱P,4,3,7,13,4,-1,300,false,false,false,,,,p_l,,,bogus,,,,\n"
	if _, err := gen(t, "terry", body); err == nil {
		t.Fatal("unknown startup_basis で fail していない")
	} else if !strings.Contains(err.Error(), "unknown startup_basis") {
		t.Errorf("エラー文言が期待形でない: %v", err)
	}
}

func TestGenerate_TotalMismatchFails(t *testing.T) {
	// startup+active-1+recovery = 4+3-1+7 = 13 だが total=99。
	body := "terry,standing_light_punch,normal,立ち弱P,4,3,7,99,4,-1,300,false,false,false,,,,p_l,,,,,,,\n"
	_, err := gen(t, "terry", body)
	if err == nil || !strings.Contains(err.Error(), "total mismatch") {
		t.Fatalf("total 不一致で fail するはず: %v", err)
	}
}

func TestGenerate_DupMoveCodeFails(t *testing.T) {
	body := "terry,dup_code,normal,技A,1,1,1,2,,,100,false,false,false,,,,p_l,,,,,,,\n" +
		"terry,dup_code,normal,技B,1,1,1,2,,,100,false,false,false,,,,p_m,,,,,,,\n"
	_, err := gen(t, "terry", body)
	if err == nil || !strings.Contains(err.Error(), "duplicate move_code") {
		t.Fatalf("code 重複で fail するはず: %v", err)
	}
}

func TestGenerate_DupAliasTextFails(t *testing.T) {
	body := "terry,code_a,super_art,同名,1,1,1,2,,,100,false,false,false,,,,p_l,,,,,,,\n" +
		"terry,code_b,critical_art,同名,1,1,1,2,,,100,false,false,false,,,,p_m,,,,,,,\n"
	_, err := gen(t, "terry", body)
	if err == nil || !strings.Contains(err.Error(), "duplicate alias_text") {
		t.Fatalf("alias_text 重複で fail するはず: %v", err)
	}
}

func TestGenerate_NonCanonicalMoveCodeFails(t *testing.T) {
	// 大文字/記号を含む move_code は DES-004 §2.1 正準形に反するため fail。
	body := "terry,Standing-LP,normal,立ち弱P,4,3,7,13,4,-1,300,false,false,false,,,,p_l,,,,,,,\n"
	_, err := gen(t, "terry", body)
	if err == nil || !strings.Contains(err.Error(), "not canonical form") {
		t.Fatalf("非正準 move_code で fail するはず: %v", err)
	}
}

func TestGenerate_UnknownCategoryFails(t *testing.T) {
	body := "terry,x,bogus,技,1,1,1,2,,,100,false,false,false,,,,p_l,,,,,,,\n"
	_, err := gen(t, "terry", body)
	if err == nil || !strings.Contains(err.Error(), "unknown category") {
		t.Fatalf("未知 category で fail するはず: %v", err)
	}
}

func TestGenerate_DropsMovementSystemMove(t *testing.T) {
	body := "terry,dash_forward,system,前ダッシュ,,,,,,,,false,false,false,,,,,,,,,,,\n" +
		"terry,drive_parry,system,ドライブパリィ,1,12,33,45,,,0,false,false,false,,,,p_m k_m,,,,,,,\n"
	res, err := gen(t, "terry", body)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(res.Dropped) != 1 || res.Dropped[0].MoveCode != "dash_forward" {
		t.Fatalf("移動 system move が drop されていない: %+v", res.Dropped)
	}
	if strings.Contains(res.UpSQL, "'dash_forward'") {
		t.Errorf("dash_forward が投入されている(drop 漏れ):\n%s", res.UpSQL)
	}
	if !strings.Contains(res.UpSQL, "'drive_parry'") {
		t.Errorf("drive_parry が通過していない:\n%s", res.UpSQL)
	}
}

func TestGenerate_RushOriginalMoveID(t *testing.T) {
	body := "terry,standing_medium_punch,normal,立ち中P,7,3,16,25,2,-3,700,false,false,false,,,,p_m,,,,,,,\n" +
		"terry,rush_standing_medium_punch,rush_variant,立ち中P(ラッシュ),18,3,16,36,6,1,700,false,false,false,,,standing_medium_punch,,,,,,,,\n"
	res, err := gen(t, "terry", body)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !strings.Contains(res.UpSQL, "WHEN 'rush_standing_medium_punch' THEN 'standing_medium_punch'") {
		t.Errorf("rush の original_move_id 解決が無い:\n%s", res.UpSQL)
	}
}

func TestGenerate_IndexExcludesDerivedAndEmpty(t *testing.T) {
	body := "terry,power_wave_light,special,弱パワーウェイブ,14,36,0,49,-3,-9,600,false,true,false,,,,d dr r plus p_l,,,,,,,\n" +
		"terry,power_drive,target_combo,パワードライブ,15,3,22,39,,-5,1470,false,false,true,,,,p_m chain p_h,,,,,,,\n"
	res, err := gen(t, "terry", body)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if got, ok := res.Index.Lookup("terry", "d dr r plus p_l"); !ok || got != "power_wave_light" {
		t.Errorf("非派生技が索引化されていない: %q %v", got, ok)
	}
	if _, ok := res.Index.Lookup("terry", "p_m chain p_h"); ok {
		t.Errorf("派生技(is_derived=true)が索引化されている")
	}
}

// TestGolden_CommittedMigrationMatchesRegeneration は character_data から再生成した SQL が
// コミット済みの 000026 と一致することを保証する(手編集ドリフト検出)。
func TestGolden_CommittedMigrationMatchesRegeneration(t *testing.T) {
	root := repoRoot(t)
	rowsByChar := map[string][]MoveRow{}
	next := 0
	for _, code := range FirstWaveOrder {
		rows, err := ReadFile(filepath.Join(root, "character_data", code+".csv"), next)
		if err != nil {
			t.Fatalf("ReadFile %s: %v", code, err)
		}
		rowsByChar[code] = rows
		next += len(rows)
	}
	// ★旧形式を指定する(format.go の FormatPreM2003)。000026 は M20-03 より前に
	//   生成・適用済みであり、character_id を含まない。改変できないため形式を固定する。
	res, err := Generate(FirstWaveOrder, rowsByChar, WithFormat(FormatPreM2003))
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	// ★★M33-03: 比較先を migrations/ から testdata/ へ移し、他の 15 本と同じ
	//   assertGolden へ統一した（着手時点は本テストだけインラインで比較していた）。
	assertGolden(t, FirstWaveStem, res.UpSQL, res.DownSQL)
}

// ryuMigrationNote は 000030 生成時に cmd/seedgen の -note へ渡した説明行(再生成は TACPENDIUM_UPDATE_GOLDEN=1 go test ./internal/seedgen/ で行う(★M33-03))。
const ryuMigrationNote = "M14-03c: ryu の moves + official_ja_move alias + recovery を手入力 CSV 由来で投入する(旧 seed は 000029 で削除済み)。"

// ryuStem は ryu 単独波の生成物の stem（★M33-03 以降は testdata/ の golden 名）。
const ryuStem = "000030_seed_moves_ryu"

// TestGolden_RyuMigrationMatchesRegeneration は ryu.csv から再生成した SQL がコミット済みの
// 000030 と一致することを保証する(手編集ドリフト検出。M14-03c §4.3.1)。再生成は TACPENDIUM_UPDATE_GOLDEN=1 go test ./internal/seedgen/ で行う
// (★M33-03。下の CLI は生成に使った引数の記録であり、出力先は migrations/ ではなく
//
//	 internal/seedgen/testdata/ である):
//
//		go run ./cmd/seedgen -chars ryu -out 000030_seed_moves_ryu -note "<ryuMigrationNote の文字列>"
func TestGolden_RyuMigrationMatchesRegeneration(t *testing.T) {
	root := repoRoot(t)
	rows, err := ReadFile(filepath.Join(root, "character_data", "ryu.csv"), 0)
	if err != nil {
		t.Fatalf("ReadFile ryu: %v", err)
	}
	// ★旧形式を指定する(000026 と同じ理由)。
	res, err := GenerateWithHeader([]string{"ryu"}, map[string][]MoveRow{"ryu": rows},
		CustomHeader(ryuStem, ryuMigrationNote), WithFormat(FormatPreM2003))
	if err != nil {
		t.Fatalf("GenerateWithHeader: %v", err)
	}
	assertGolden(t, ryuStem, res.UpSQL, res.DownSQL)
}

// repoRoot は比較の基準となる公開版 tacpendium のチェックアウトを返す。
//
// ★単体版での変更(docs/upstream-diff.md): 本家では go.mod のあるリポジトリルートを探していた。
// 単体版は character_data・migrations・golden(testdata)を持たず、公開版 tacpendium
// (upstream/tacpendium.lock の SHA)を取込元とする(HANDOVER D2)。⇒ 比較先はそのチェックアウトである。
// 環境変数 TACPENDIUM_SRC が無ければ、scripts/fetch-upstream.sh の既定の置き場を使う。
// 無ければ skip せずに落とす(取得し忘れで緑にしない)。
func repoRoot(t *testing.T) string {
	t.Helper()
	if dir := os.Getenv("TACPENDIUM_SRC"); dir != "" {
		return dir
	}
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			src := filepath.Join(dir, ".upstream", "tacpendium")
			if _, err := os.Stat(filepath.Join(src, "character_data")); err != nil {
				t.Fatalf("公開版 tacpendium が無い(%s)。bash scripts/fetch-upstream.sh で取得すること: %v", src, err)
			}
			return src
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod が見つからない")
		}
		dir = parent
	}
}
