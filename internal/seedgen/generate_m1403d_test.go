package seedgen

import (
	"path/filepath"
	"testing"

	"github.com/plexiblinp/tacpendium-setplay/internal/moveindex"
)

// M14-03d(第二波 seed = manon 単独)で生成した 3 本のマイグレに対する golden。
// 「生成マイグレ 1 本につき golden 1 本」は M14-03c で確立した運用(指示書 §5.1)。
//
// manon は単独波のため RowIndex は 0 起点(000030 の ryu と同じ)。前提となる characters 行と
// 移動 system move 9 種は手書きマイグレ 000043/000044 で投入する(manon は M14-03b の 000024/000025
// 適用時点で characters に存在せず、移動 9 種が 1 行も入っていなかったため)。

// 000045/000046/000047 生成時に cmd/seedgen の -note へ渡した説明行(再生成コマンドは各 golden のコメント)。
const (
	manonMovesMigrationNote        = "M14-03d: manon の moves + official_ja_move alias + recovery を手入力 CSV 由来で投入する(第二波)。"
	manonBackfillMigrationNote     = "M14-03d: manon の is_derived backfill(CSV に載る code のみ UPDATE)"
	manonMoveCommandsMigrationNote = "M14-03d: manon の command 索引 move_commands の seed(非派生のみ)"
)

// readManonRows は manon.csv を生成時と同一条件(単独キャラ・RowIndex 0 起点)で読む。
func readManonRows(t *testing.T) map[string][]MoveRow {
	t.Helper()
	rows, err := ReadFile(filepath.Join(repoRoot(t), "character_data", "manon.csv"), 0)
	if err != nil {
		t.Fatalf("ReadFile manon: %v", err)
	}
	return map[string][]MoveRow{"manon": rows}
}

// TestGolden_ManonMovesMatchesRegeneration は manon.csv から再生成した SQL がコミット済みの
// 000045 と一致することを保証する(手編集ドリフト検出)。再生成は TACPENDIUM_UPDATE_GOLDEN=1 go test ./internal/seedgen/ で行う
// (★M33-03。下の CLI は生成に使った引数の記録であり、出力先は migrations/ ではなく
//
//	 internal/seedgen/testdata/ である):
//
//		go run ./cmd/seedgen -chars manon -out 000045_seed_moves_manon -note "<manonMovesMigrationNote の文字列>"
func TestGolden_ManonMovesMatchesRegeneration(t *testing.T) {
	// ★旧形式を指定する(format.go の FormatPreM2003)。000045 は M20-03 より前に
	//   生成・適用済みであり、character_id を含まない。
	res, err := GenerateWithHeader([]string{"manon"}, readManonRows(t),
		CustomHeader("000045_seed_moves_manon", manonMovesMigrationNote), WithFormat(FormatPreM2003))
	if err != nil {
		t.Fatalf("GenerateWithHeader: %v", err)
	}
	assertGolden(t, "000045_seed_moves_manon", res.UpSQL, res.DownSQL)
}

// TestGolden_ManonDerivedBackfillMatchesRegeneration は manon.csv から再生成した SQL が
// コミット済みの 000046 と一致することを保証する。再生成は TACPENDIUM_UPDATE_GOLDEN=1 go test ./internal/seedgen/ で行う
// (★M33-03。下の CLI は生成に使った引数の記録であり、出力先は migrations/ ではなく
//
//	 internal/seedgen/testdata/ である):
//
//		go run ./cmd/seedgen -mode derived-backfill -chars manon \
//		  -out 000046_backfill_moves_is_derived_manon -note "<manonBackfillMigrationNote の文字列>"
func TestGolden_ManonDerivedBackfillMatchesRegeneration(t *testing.T) {
	res, err := GenerateDerivedBackfill([]string{"manon"}, readManonRows(t),
		BackfillHeader("000046_backfill_moves_is_derived_manon", manonBackfillMigrationNote))
	if err != nil {
		t.Fatalf("GenerateDerivedBackfill: %v", err)
	}
	assertGolden(t, "000046_backfill_moves_is_derived_manon", res.UpSQL, res.DownSQL)
}

// TestGolden_ManonMoveCommandsMatchesRegeneration は manon.csv から再生成した SQL が
// コミット済みの 000047 と一致することを保証する。既存 000035 は非改変(新規連番で追加)。再生成は TACPENDIUM_UPDATE_GOLDEN=1 go test ./internal/seedgen/ で行う
// (★M33-03。下の CLI は生成に使った引数の記録であり、出力先は migrations/ ではなく
//
//	 internal/seedgen/testdata/ である):
//
//		go run ./cmd/seedgen -mode move-commands -chars manon \
//		  -out 000047_seed_move_commands_manon -note "<manonMoveCommandsMigrationNote の文字列>"
func TestGolden_ManonMoveCommandsMatchesRegeneration(t *testing.T) {
	res, err := GenerateMoveCommands([]string{"manon"}, readManonRows(t),
		MoveCommandsHeader("000047_seed_move_commands_manon", manonMoveCommandsMigrationNote))
	if err != nil {
		t.Fatalf("GenerateMoveCommands: %v", err)
	}
	assertGolden(t, "000047_seed_move_commands_manon", res.UpSQL, res.DownSQL)
}

// TestManon_ConversionInvariants は manon 投入が M14-03d の非改変条件を満たすことを固定する。
// (a) 移動 9 種の drop が 0 件＝CSV に移動行が無く 000044 が唯一の投入元であること
// (b) 索引非搭載が derived のみ＝未知トークン(opt_open/opt_close 等)・command 空・条件残留が 0 件
func TestManon_ConversionInvariants(t *testing.T) {
	res, err := GenerateWithHeader([]string{"manon"}, readManonRows(t),
		CustomHeader("000045_seed_moves_manon", manonMovesMigrationNote))
	if err != nil {
		t.Fatalf("GenerateWithHeader: %v", err)
	}
	if len(res.Dropped) != 0 {
		t.Errorf("drop 移動move = %d, want 0(manon.csv の category=system は drive_parry のみ)", len(res.Dropped))
	}
	for _, s := range res.Index.Skipped() {
		if s.Reason != moveindex.SkipDerived {
			t.Errorf("索引非搭載の理由が derived 以外: move=%s reason=%s", s.MoveCode, s.Reason)
		}
	}
}
