package seedgen

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/plexiblinp/tacpendium-setplay/internal/migrationpin"
)

// M48-08(CHANGE-256)の生成器テスト。
//
//   - 規則: 派生技も層 A ／ 被ったら優先の 1 つだけ残す(派生でない技 → 派生技 → CSV の行順)
//   - 不変条件: 旧規則で作っていた行は、新しい規則でも 1 行も変わらない(今出ている表記が消えない)
//   - 生成物: 差分マイグレ 000012 がリポジトリに在る(★M43-07 以降、固定済みなので今の CSV と照らさない＝
//     assertGeneratedMigration)。被りの表 internal/aliascollision/table_gen.go は TestM4305_CollisionTableIsGenerated

const m4808Stem = "000012_data_preset_aliases_derived_priority"

// readAllCharacterRows は character_data/*.csv の全キャラを辞書順に読む(RowIndex は通し番号)。
//
// ★キャラ内の RowIndex の順は CSV の行の順であり、000004 が振った moves.id の順と同じである
// (M48-08 §3.3 の実査。31 キャラ全数で逆転 0)。
func readAllCharacterRows(t *testing.T) ([]string, map[string][]MoveRow) {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(repoRoot(t), "character_data", "*.csv"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	sort.Strings(files)
	var order []string
	rowsByChar := map[string][]MoveRow{}
	next := 0
	for _, f := range files {
		rows, err := ReadFile(f, next)
		if err != nil {
			t.Fatalf("ReadFile %s: %v", f, err)
		}
		if len(rows) == 0 {
			continue
		}
		next += len(rows)
		code := rows[0].CharacterCode
		order = append(order, code)
		rowsByChar[code] = rows
	}
	if len(order) != 32 {
		t.Fatalf("CSV のキャラ数 = %d, want 32", len(order))
	}
	return order, rowsByChar
}

// withIndex は fixture の行に CSV の行順(RowIndex)を振る。
func withIndex(rows ...MoveRow) []MoveRow {
	for i := range rows {
		rows[i].RowIndex = i
	}
	return rows
}

// TestM4808_DerivedMoveGetsLayerA は command を持つ派生技にも層 A の表記が付くことを固定する(U-2 の撤回)。
func TestM4808_DerivedMoveGetsLayerA(t *testing.T) {
	rows := withIndex(
		mkRow("guile", "sonic_boom_light", "special", "弱ソニックブーム", "charge_l r plus p_l", false),
		mkRow("guile", "sonic_boom_perfect_light", "special", "【ジャスト】弱ソニックブーム", "charge_l r plus p_m", true),
		// ★command が空の派生技は今までどおり埋めない(層 C-3 の対象でもない)。
		mkRow("guile", "no_input_follow_up", "special", "何もしない派生", "", true),
	)
	res := genOne(t, NumericRule, "guile", rows)
	got, ok := aliasOf(res, "sonic_boom_perfect_light")
	if !ok || got.Layer != LayerA || got.AliasText != "[4]6MP" {
		t.Errorf("派生技の層 A = %+v (ok=%v), want [4]6MP / 層 A", got, ok)
	}
	if u, ok := unfilledOf(res, "no_input_follow_up"); !ok || u.Reason != ReasonDerivedNoCommand {
		t.Errorf("command が空の派生技 = %+v, want derived-no-command", u)
	}
	for _, u := range res.Unfilled {
		if u.Reason == ReasonDerived {
			t.Errorf("FormatCurrent で ReasonDerived が出た: %+v", u)
		}
	}
}

// TestM4808_CollisionKeepsPrior は被ったら優先の 1 つだけ残ることを固定する(開発者決定 D-1037 ／ Plan §9.4-1)。
func TestM4808_CollisionKeepsPrior(t *testing.T) {
	t.Run("非派生どうしは CSV の行順で先が残る", func(t *testing.T) {
		// ★合成の fixture(is_aerial を立てていない)。実データのフライングボディプレスは is_aerial=1 であり、
		//   M43-05 以降は `j.2HP` になってしゃがみ強P と被らない。本テストは優先の規則だけを見る。
		rows := withIndex(
			mkRow("zangief", "crouching_heavy_punch", "normal", "しゃがみ強P", "d plus p_h", false),
			mkRow("zangief", "flying_body_press", "unique", "フライングボディプレス", "d plus p_h", false),
		)
		res := genOne(t, NumericRule, "zangief", rows)
		if got, ok := aliasOf(res, "crouching_heavy_punch"); !ok || got.AliasText != "2HP" {
			t.Errorf("先の crouching_heavy_punch が残っていない: %+v", res.Rows)
		}
		u, ok := unfilledOf(res, "flying_body_press")
		if !ok || u.Reason != ReasonCollision || !strings.Contains(u.Detail, "残る側: crouching_heavy_punch") {
			t.Errorf("flying_body_press = %+v (ok=%v), want collision ＋ 残る側", u, ok)
		}
		if len(res.Collisions) != 1 || res.Collisions[0].Kept != "crouching_heavy_punch" ||
			strings.Join(res.Collisions[0].Dropped, ",") != "flying_body_press" || res.Collisions[0].AliasText != "2HP" {
			t.Errorf("Collisions = %+v", res.Collisions)
		}
	})

	t.Run("派生技は行順が先でも非派生の後ろ", func(t *testing.T) {
		// ★実データの alex `6MP`(中スラッシュエルボー id2150 ＜ チョップ id2162)と同じ形。
		rows := withIndex(
			mkRow("alex", "medium_slashing_elbow", "unique", "中スラッシュエルボー", "r plus p_m", true),
			mkRow("alex", "chop", "unique", "チョップ", "r plus p_m", false),
		)
		res := genOne(t, NumericRule, "alex", rows)
		if got, ok := aliasOf(res, "chop"); !ok || got.AliasText != "6MP" {
			t.Errorf("非派生の chop が残っていない: %+v", res.Rows)
		}
		if _, ok := aliasOf(res, "medium_slashing_elbow"); ok {
			t.Errorf("派生技が残った: %+v", res.Rows)
		}
	})

	t.Run("派生どうしは行順で先が残る", func(t *testing.T) {
		rows := withIndex(
			mkRow("dee_jay", "juggling_sway_od", "special", "ODジャグリングスウェイ", "l plus p", true),
			mkRow("dee_jay", "juggling_sway", "special", "ジャグリングスウェイ", "l plus p", true),
		)
		res := genOne(t, NumericRule, "dee_jay", rows)
		if _, ok := aliasOf(res, "juggling_sway_od"); !ok {
			t.Errorf("先の juggling_sway_od が残っていない: %+v", res.Rows)
		}
		if _, ok := aliasOf(res, "juggling_sway"); ok {
			t.Errorf("後の juggling_sway が残った: %+v", res.Rows)
		}
	})

	t.Run("移動系の表記と被ったら移動系が残り、被りの組には入れない", func(t *testing.T) {
		rows := withIndex(mkRow("alex", "low_retreat", "unique", "ロウリトリート", "l", true))
		for _, rule := range []AliasPresetRule{NumericRule, SRKRule} {
			// ★srk では `4` は移動系の表記ではないが、numeric の移動系の表記として予約する
			//   (Plan 未決 5 の開発者決定)。
			res := genOne(t, rule, "alex", rows)
			if _, ok := aliasOf(res, "low_retreat"); ok {
				t.Errorf("%s: 移動系の表記 4 を技が取った: %+v", rule.PresetCode, res.Rows)
			}
			if u, ok := unfilledOf(res, "low_retreat"); !ok || u.Reason != ReasonCollision {
				t.Errorf("%s: low_retreat = %+v, want collision", rule.PresetCode, u)
			}
			if len(res.Collisions) != 0 {
				t.Errorf("%s: 移動系との被りが Collisions に入った: %+v", rule.PresetCode, res.Collisions)
			}
		}
	})

	t.Run("旧規則(FormatPreM4808)は他のプリセットの移動系を予約しない", func(t *testing.T) {
		rows := withIndex(mkRow("alex", "low_retreat_x", "unique", "非派生の 4", "l", false))
		res := genOne(t, SRKRule, "alex", rows, WithFormat(FormatPreM4808))
		if _, ok := aliasOf(res, "low_retreat_x"); !ok {
			t.Errorf("旧規則の srk で 4 が作られていない: %+v", res.Unfilled)
		}
	})
}

// TestM4808_ExistingRowsSurvive は「旧規則で作っていた行は、新しい規則でも同じ表記で残る」を
// 実データの全 32 キャラ × numeric ・srk で固定する(M48-08 §3.3-1 (c)＝今出ている表記が消えるボタン 0 の根拠)。
func TestM4808_ExistingRowsSurvive(t *testing.T) {
	order, rows := readAllCharacterRows(t)
	for _, rule := range []AliasPresetRule{NumericRule, SRKRule} {
		oldRes, err := GenerateAliases(rule, order, rows, order, AliasHeader("m", "m"), WithFormat(FormatPreM4808))
		if err != nil {
			t.Fatalf("旧規則: %v", err)
		}
		// ★M43-05 以降、「新しい規則」は M48-08 の規則を凍結した FormatPreM4305 を指す
		//   (FormatCurrent はターゲットコンボ ・空中の技の表記を変える＝TestM4305_*)。
		newRes, err := GenerateAliases(rule, order, rows, order, AliasHeader("m", "m"), WithFormat(FormatPreM4305))
		if err != nil {
			t.Fatalf("新しい規則: %v", err)
		}
		got := map[string]string{}
		for _, r := range newRes.Rows {
			got[r.CharCode+"/"+r.MoveCode] = r.AliasText
		}
		lost := 0
		for _, r := range oldRes.Rows {
			if t2, ok := got[r.CharCode+"/"+r.MoveCode]; !ok || t2 != r.AliasText {
				lost++
				t.Errorf("%s: %s/%s の表記 %q が新しい規則で %q になった", rule.PresetCode, r.CharCode, r.MoveCode, r.AliasText, t2)
			}
		}
		if lost == 0 && len(newRes.Rows) <= len(oldRes.Rows) {
			t.Errorf("%s: 行が増えていない(旧 %d ・新 %d)", rule.PresetCode, len(oldRes.Rows), len(newRes.Rows))
		}
	}
}

// TestM4808_GenerationMeasurements は新しい規則の実測値(全 32 キャラ)を固定する。
//
// ★投入行と非投入行を対で固定する(SUPP-001 §5.5.2 (3))。旧規則の実測は
// TestM2002_GenerationMeasurements ・TestFourthWave_GenerationMeasurements(FormatPreM4808)。
func TestM4808_GenerationMeasurements(t *testing.T) {
	order, rows := readAllCharacterRows(t)
	for _, tc := range []struct {
		rule                                   AliasPresetRule
		oldRows, newRows, added                int
		collision, collisionGroups, droppedSum int
		derivedNoCommand, noCommand, indexSkip int
		rushOrigUnfilled                       int
	}{
		{rule: NumericRule, oldRows: 2065, newRows: 2429, added: 364,
			collision: 347, collisionGroups: 245, droppedSum: 338,
			derivedNoCommand: 20, noCommand: 14, indexSkip: 5, rushOrigUnfilled: 12},
		// ★srk は numeric の移動系の表記(4 ・6 ・66 ・8 等)を予約するため、派生技 7 行と
		//   その DR 版 3 行が作られない(Plan 未決 5 の開発者決定)。
		{rule: SRKRule, oldRows: 2093, newRows: 2444, added: 351,
			collision: 333, collisionGroups: 231, droppedSum: 324,
			derivedNoCommand: 20, noCommand: 14, indexSkip: 5, rushOrigUnfilled: 11},
	} {
		t.Run(tc.rule.PresetCode, func(t *testing.T) {
			oldRes, err := GenerateAliases(tc.rule, order, rows, order, AliasHeader("m", "m"), WithFormat(FormatPreM4808))
			if err != nil {
				t.Fatal(err)
			}
			res, err := GenerateAliases(tc.rule, order, rows, order, AliasHeader("m", "m"), WithFormat(FormatPreM4305))
			if err != nil {
				t.Fatal(err)
			}
			delta, err := GenerateAliasDelta(tc.rule, order, rows, AliasDeltaHeader("m", tc.rule.PresetCode))
			if err != nil {
				t.Fatal(err)
			}
			byReason := map[unfilledReason]int{}
			for _, u := range res.Unfilled {
				byReason[u.Reason]++
			}
			dropped := 0
			for _, c := range res.Collisions {
				dropped += len(c.Dropped)
			}
			for _, c := range []struct {
				name      string
				got, want int
			}{
				{"旧規則の行", len(oldRes.Rows), tc.oldRows},
				{"新しい規則の行", len(res.Rows), tc.newRows},
				{"差分マイグレの行", len(delta.Emitted), tc.added},
				{"collision(落ちた行。移動系との被りを含む)", byReason[ReasonCollision], tc.collision},
				{"被りの組(技が残った組)", len(res.Collisions), tc.collisionGroups},
				{"被りの組で落ちた技", dropped, tc.droppedSum},
				{"derived", byReason[ReasonDerived], 0},
				{"derived-no-command", byReason[ReasonDerivedNoCommand], tc.derivedNoCommand},
				{"no-command", byReason[ReasonNoCommand], tc.noCommand},
				{"index-skip", byReason[ReasonIndexSkip], tc.indexSkip},
				{"rush-no-original", byReason[ReasonRushNoOriginal], 0},
				{"rush-original-unfilled", byReason[ReasonRushOriginalUnfilled], tc.rushOrigUnfilled},
			} {
				if c.got != c.want {
					t.Errorf("%s = %d, want %d", c.name, c.got, c.want)
				}
			}
			if len(res.Rows) != len(oldRes.Rows)+len(delta.Emitted) {
				t.Errorf("新しい規則の行 %d ≠ 旧 %d ＋ 差分 %d", len(res.Rows), len(oldRes.Rows), len(delta.Emitted))
			}
		})
	}
}

// renderDeltaMigration は差分マイグレ 000012 の up / down の全文を作る(numeric → srk の順)。
func renderDeltaMigration(t *testing.T) (string, string) {
	t.Helper()
	order, rows := readAllCharacterRows(t)
	up := DeltaMigrationPreamble(m4808Stem, "up")
	down := DeltaMigrationPreamble(m4808Stem, "down")
	var downs []string
	for _, rule := range []AliasPresetRule{NumericRule, SRKRule} {
		res, err := GenerateAliasDelta(rule, order, rows, AliasDeltaHeader(m4808Stem, rule.PresetCode))
		if err != nil {
			t.Fatalf("GenerateAliasDelta(%s): %v", rule.PresetCode, err)
		}
		up += res.UpSQL
		downs = append(downs, res.DownSQL)
	}
	// down は up の逆順(srk → numeric)。
	for i := len(downs) - 1; i >= 0; i-- {
		down += downs[i]
	}
	return up, down
}

// assertGenerated は生成物とコミット済みのファイルの一致を見る(TACPENDIUM_UPDATE_GOLDEN=1 で書く)。
//
// ★★書き先が固定の一覧(internal/migrationpin/applied.sha256)に載るファイルなら、照らさず書かずに止める(M43-07)。
// ⇒ 差分マイグレは assertGeneratedMigration を通すこと。本関数を直接使うのは被りの表のようなマイグレでない生成物だけ。
func assertGenerated(t *testing.T, path, want string) {
	t.Helper()
	if loadPins(t).Has(filepath.Base(path)) {
		t.Fatalf("%s は固定済みのマイグレである。照らし合わせも出し直しもしない。\n%s", path, migrationpin.Guidance)
	}
	if os.Getenv(goldenUpdateEnv) != "" {
		if err := os.WriteFile(path, []byte(want), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
		t.Logf("生成物を更新: %s", path)
		return
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v (★%s=1 で生成できる)", path, err, goldenUpdateEnv)
	}
	if string(got) != want {
		t.Errorf("%s が生成器の出力と不一致。CSV 正本と規則のどちらが変わったかを特定してから %s=1 で更新すること",
			path, goldenUpdateEnv)
	}
}

func loadPins(t *testing.T) migrationpin.Manifest {
	t.Helper()
	m, err := migrationpin.Load()
	if err != nil {
		t.Fatalf("固定の一覧を読めない: %v", err)
	}
	return m
}

// generatedMigrationResult は syncGeneratedMigration の結果。
type generatedMigrationResult struct {
	pinned   bool     // 固定済みだったか(＝今の CSV と照らさなかったか)
	wrote    []string // 書いたファイル
	problems []string // 赤にする理由
}

// syncGeneratedMigration は差分マイグレ(stem の up / down)を、固定の一覧に載っているかで分けて扱う(M43-07 指示書 §1.2)。
//
//	固定済み … 在ることだけを確かめる。render(今の CSV の読み込み)を呼ばない。update でも書かない。
//	未固定   … render の出力と byte 一致で照らす。update なら書く(作業中のサブのものだけが出し直せる)。
func syncGeneratedMigration(dir, stem string, pins migrationpin.Manifest, update bool,
	render func() (up, down string)) generatedMigrationResult {
	var res generatedMigrationResult
	paths := [2]string{filepath.Join(dir, stem+".up.sql"), filepath.Join(dir, stem+".down.sql")}
	if pins.Has(stem+".up.sql") || pins.Has(stem+".down.sql") {
		res.pinned = true
		if !pins.Pinned(stem) {
			res.problems = append(res.problems, fmt.Sprintf("%s: up と down の片方だけが固定の一覧に載っている", stem))
		}
		for _, p := range paths {
			if _, err := os.Stat(p); err != nil {
				res.problems = append(res.problems, fmt.Sprintf("%s: 固定済みなのにファイルが無い: %v", p, err))
			}
		}
		return res
	}
	up, down := render()
	for i, want := range [2]string{up, down} {
		p := paths[i]
		if update {
			if err := os.WriteFile(p, []byte(want), 0o644); err != nil {
				res.problems = append(res.problems, fmt.Sprintf("write %s: %v", p, err))
				continue
			}
			res.wrote = append(res.wrote, p)
			continue
		}
		got, err := os.ReadFile(p)
		if err != nil {
			res.problems = append(res.problems, fmt.Sprintf("read %s: %v (★未固定なので %s=1 で生成できる)", p, err, goldenUpdateEnv))
			continue
		}
		if string(got) != want {
			res.problems = append(res.problems, fmt.Sprintf("%s が生成器の出力と不一致。CSV 正本と規則のどちらが変わったかを特定してから "+
				"%s=1 go test ./internal/seedgen/ -run <このテスト> で出し直すこと(未固定＝作業中のサブのものだけ)", p, goldenUpdateEnv))
		}
	}
	return res
}

// assertGeneratedMigration は syncGeneratedMigration を migrations/ と埋め込みの固定の一覧で回す。
func assertGeneratedMigration(t *testing.T, stem string, render func() (up, down string)) {
	t.Helper()
	res := syncGeneratedMigration(filepath.Join(repoRoot(t), "migrations"), stem, loadPins(t),
		os.Getenv(goldenUpdateEnv) != "", render)
	for _, p := range res.problems {
		t.Error(p)
	}
	switch {
	case res.pinned:
		t.Logf("%s は固定済み(適用済み)。在ることだけを確かめ、今の CSV と照らさない・出し直さない(中身は sha256 が守る)", stem)
	case len(res.wrote) > 0:
		t.Logf("生成物を更新(未固定): %s ／ ★完了報告の前に %s へ sha256 を足すこと", strings.Join(res.wrote, ", "), migrationpin.ManifestPath)
	}
}

// TestM4808_DeltaMigrationIsGenerated は差分マイグレ 000012 を固定の一覧の基準で扱う(M43-07)。
//
// ★★000012 は固定済み(適用済み)である。⇒ 在ることだけを確かめ、今の CSV から作り直して照らさない。
// 中身は internal/migrationpin の sha256 が守る。CSV を直して生成器の出力が変わっても、000012 は書き換えず、
// 新しい連番の差分マイグレを起こすこと(character_data/seed-progress.md)。
// ★失われる主張＝「000012 が今の CSV ＋ FormatPreM4305 で再現できる」(M43-07 完了報告 §失われる主張)。
func TestM4808_DeltaMigrationIsGenerated(t *testing.T) {
	assertGeneratedMigration(t, m4808Stem, func() (string, string) { return renderDeltaMigration(t) })
}

// ★【M43-05】取込の被りの表(internal/aliascollision/table_gen.go)は今の規則(FormatCurrent)の出力であり、
// TestM4305_CollisionTableIsGenerated が生成 ・固定する。M48-08 の規則で出した表は 000012 と一緒に凍結しない
// (取込が引くのは今の組み込みの行と一致する表だけである)。
