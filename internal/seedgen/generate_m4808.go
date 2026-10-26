package seedgen

import (
	"fmt"
	"go/format"
	"sort"
	"strings"
)

// 本ファイルは M48-08(CHANGE-256)の生成系を提供する。
//
//   - GenerateAliasDelta … 規則を変えたことで増える numeric / srk の行だけを出す差分マイグレ
//   - RenderCollisionTable … 被った表記の表(internal/aliascollision/table_gen.go)
//
// ★差分にする理由——既に適用済みのマイグレ(000008)は書き換えない(sha256 固定＝M42-04)。
// ⇒ 新しい規則の出力から旧規則の出力(＝000008 の numeric / srk の行)を引いた行だけを足す。
// ★旧規則の行はすべて新しい規則でも同じ表記で残る(優先＝派生でない技 → 派生技 の帰結)。
// それが崩れたら生成を止める——崩れたまま出すと、足すだけの差分では既存の行の変化を表せない。

// GenerateAliasDelta は 1 プリセット分の差分(FormatPreM4305 − FormatPreM4808)の SQL を作る。
//
// ★【M43-05】000012 は適用済みである。⇒ 新しい側は FormatCurrent ではなく
// M48-08 の規則を凍結した FormatPreM4305 である(M43-05 の差分は GenerateAliasRewrite ＝000014)。
// ★【M43-07】000012 は固定済み(internal/migrationpin)であり、生成物テストは今の CSV で作り直して照らさない。
//
// ★移動系 9 code は出さない(規則を変えても変わらず、000008 が投入済み)。
// ★up は既存の行を書き換えない(NOT EXISTS)。down は差分の行だけを当該プリセットの中で消す
// (カスタムプリセットの行には触れない＝M48-08 Plan §9.4-3 の開発者決定「足さない」)。
func GenerateAliasDelta(rule AliasPresetRule, charOrder []string, rowsByChar map[string][]MoveRow,
	header func(dir string) string) (*AliasResult, error) {
	if err := validate(charOrder, rowsByChar); err != nil {
		return nil, err
	}
	res := &AliasResult{LayerStats: map[aliasLayer]int{}}
	var up, down strings.Builder
	up.WriteString(header("up"))
	down.WriteString(header("down"))
	var downBlocks []string
	for _, charCode := range charOrder {
		rows := rowsByChar[charCode]
		oldRows, _, _, _, _ := resolveCharacter(rule, charCode, rows, FormatPreM4808)
		newRows, unfilled, discarded, saNoAnn, collisions := resolveCharacter(rule, charCode, rows, FormatPreM4305)
		res.Unfilled = append(res.Unfilled, unfilled...)
		res.DiscardedLayerA = append(res.DiscardedLayerA, discarded...)
		res.SAWithoutAnnotation = append(res.SAWithoutAnnotation, saNoAnn...)
		res.Collisions = append(res.Collisions, collisions...)
		res.Rows = append(res.Rows, newRows...)

		byMove := map[string]AliasRow{}
		for _, r := range newRows {
			byMove[r.MoveCode] = r
		}
		for _, o := range oldRows {
			n, ok := byMove[o.MoveCode]
			if !ok || n.AliasText != o.AliasText || n.AliasTextEn != o.AliasTextEn {
				return nil, fmt.Errorf("preset %q: %s/%s の既存の表記 %q が新しい規則で変わる(差分マイグレは追加だけを表せる)",
					rule.PresetCode, charCode, o.MoveCode, o.AliasText)
			}
			delete(byMove, o.MoveCode)
		}
		var emit []AliasRow
		for _, r := range newRows { // newRows は MoveCode 昇順
			if _, add := byMove[r.MoveCode]; add {
				emit = append(emit, r)
				res.LayerStats[r.Layer]++
			}
		}
		res.Emitted = append(res.Emitted, emit...)
		if len(emit) == 0 {
			continue
		}
		writeCharAliasInsert(&up, rule, charCode, emit, FormatPreM4305)
		downBlocks = append(downBlocks, buildCharAliasDeltaDown(rule, charCode, emit))
	}
	for i := len(downBlocks) - 1; i >= 0; i-- {
		down.WriteString(downBlocks[i])
	}
	res.UpSQL = up.String()
	res.DownSQL = down.String()
	return res, nil
}

// buildCharAliasDeltaDown は差分の 1 キャラ分の down を作る。
//
// ★(技, 表記) の組で消す。up は (プリセット, 技) の NOT EXISTS で skip するため、技だけで消すと
// 「up の時点で既に在った行」まで消しうる(GenerateAliases の godoc が警告する非対称)。組み込みは
// 編集できず差分は 000008 の行を含まないため実際には起きないが、その不変条件に依存しない形にする
// (M48-08 レビュー 低-3)。
func buildCharAliasDeltaDown(rule AliasPresetRule, charCode string, rows []AliasRow) string {
	var b strings.Builder
	fmt.Fprintf(&b, "-- ===== %s =====\n", charCode)
	fmt.Fprintf(&b, "DELETE FROM preset_aliases WHERE preset_id = %s AND EXISTS (\n", presetIDSubquery(rule))
	fmt.Fprintf(&b, "    SELECT 1 FROM moves m JOIN characters c ON c.id = m.character_id\n")
	fmt.Fprintf(&b, "    WHERE m.id = preset_aliases.move_id AND %s\n", charFilter("c", charCode))
	b.WriteString("      AND (m.code, preset_aliases.alias_text) IN (VALUES\n")
	for i, r := range rows {
		sep := ","
		if i == len(rows)-1 {
			sep = ""
		}
		fmt.Fprintf(&b, "        (%s, %s)%s\n", sqlStr(r.MoveCode), sqlStr(r.AliasText), sep)
	}
	b.WriteString("      ));\n\n")
	return b.String()
}

// AliasDeltaHeader は GenerateAliasDelta 用の生成 SQL ヘッダを組み立てる。
func AliasDeltaHeader(stem, presetCode string) func(dir string) string {
	return func(dir string) string {
		return "-- ===== preset " + presetCode + " =====\n" +
			"-- " + stem + "." + dir + ".sql の " + presetCode + " の塊(cmd/seedgen ではなく生成器のテストが生成する。手編集しない)。\n\n"
	}
}

// DeltaMigrationPreamble は差分マイグレ(000012)の冒頭のコメントである。
//
// ★「何を戻さないか」をここへ書く(playbook §4.17)。
func DeltaMigrationPreamble(stem, dir string) string {
	h := "-- " + stem + "." + dir + ".sql\n" +
		"-- M48-08(CHANGE-256): 組み込みプリセット numeric / srk の表記を、派生技にも付け、被ったら優先の 1 つだけ残す規則で足す。\n" +
		"-- 本ファイルは internal/seedgen の GenerateAliasDelta が character_data/*.csv から生成した成果物(手編集しない)。\n" +
		"--   再生成: TACPENDIUM_UPDATE_GOLDEN=1 go test ./internal/seedgen/ -run TestM4808_\n" +
		"-- 足すのは「新しい規則の出力 − 旧規則の出力(＝000008 の numeric / srk の行)」だけである。既存の行は 1 行も変えない。\n" +
		"-- ★カスタムプリセットには足さない(開発者決定)。表示は base_preset_code の組み込みへ戻って引くため新しい表記は出る。\n"
	if dir == "down" {
		h += "-- down は本ファイルの up が足した (プリセット, 技, 表記) の行だけを numeric / srk の中で消す。\n" +
			"-- ★戻さないもの: なし(カスタムプリセットの行・利用者の行には up も down も触れない)。\n"
	}
	return h + "\n"
}

// RenderCollisionTable は被った表記の表を Go のソースとして出す(internal/aliascollision)。
//
// results は preset code → そのプリセットを全キャラで生成した結果。★取込の辞書は
// プリセットを跨いで 1 つであるため、表もプリセットを跨いで (キャラ, 表記) → 落ちた技の集合に畳む。
func RenderCollisionTable(results map[string]*AliasResult) (string, error) {
	table := map[string]map[string]map[string]bool{}
	for _, res := range results {
		for _, c := range res.Collisions {
			if table[c.CharCode] == nil {
				table[c.CharCode] = map[string]map[string]bool{}
			}
			if table[c.CharCode][c.AliasText] == nil {
				table[c.CharCode][c.AliasText] = map[string]bool{}
			}
			for _, d := range c.Dropped {
				table[c.CharCode][c.AliasText][d] = true
			}
		}
	}
	var b strings.Builder
	b.WriteString("// Code generated by internal/seedgen (TestM4808_CollisionTableIsGenerated). DO NOT EDIT.\n")
	b.WriteString("//\n// 再生成: TACPENDIUM_UPDATE_GOLDEN=1 go test ./internal/seedgen/ -run TestM4808_\n\n")
	b.WriteString("package aliascollision\n\n")
	b.WriteString("// dropped はキャラ code → 被った表記 → 表記を作らなかった技の move_code(昇順)。\n")
	b.WriteString("var dropped = map[string]map[string][]string{\n")
	chars := sortedKeys(table)
	for _, c := range chars {
		fmt.Fprintf(&b, "\t%q: {\n", c)
		for _, t := range sortedKeys(table[c]) {
			codes := sortedKeys(table[c][t])
			quoted := make([]string, len(codes))
			for i, code := range codes {
				quoted[i] = fmt.Sprintf("%q", code)
			}
			fmt.Fprintf(&b, "\t\t%q: {%s},\n", t, strings.Join(quoted, ", "))
		}
		b.WriteString("\t},\n")
	}
	b.WriteString("}\n")
	// ★gofmt 済みの形で出す(生成物も CI の gofmt 検査の対象になるため)。
	src, err := format.Source([]byte(b.String()))
	if err != nil {
		return "", fmt.Errorf("被りの表の Go ソースを整形できない: %w", err)
	}
	return string(src), nil
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
