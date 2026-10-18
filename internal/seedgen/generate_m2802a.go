package seedgen

import (
	"fmt"
	"sort"
	"strings"

	"github.com/plexiblinp/tacpendium-setplay/internal/moveindex"
)

// GameVersionHeader は -mode game-version の生成ヘッダ。
func GameVersionHeader(stem, note string) func(dir string) string {
	return func(dir string) string {
		h := "-- " + stem + "." + dir + ".sql\n"
		if note != "" {
			h += "-- " + note + "\n"
		}
		h += "-- 本ファイルは cmd/seedgen -mode game-version が character_data/*.csv から\n" +
			"-- 生成した成果物(手編集しない)。FR702 のマーカー(M28-02a)。\n" +
			"-- 既存マイグレは非改変(新規連番で追加)。\n\n"
		return h
	}
}

// GenerateGameVersion は CSV の last_changed_game_version 列から
// moves のマーカーを立てるマイグレを生成する(FR702・M28-02a §2.2-4)。
//
// ★★なぜ moves の seed 波(-mode moves)と別モードなのか。
//
//	golden テストは「現在の CSV から再生成した SQL」と「適用済みマイグレ」を byte 比較する。
//	⇒ マーカーを moves 波の出力に混ぜると、CSV へ値を 1 つ書いた瞬間に
//	  000026 等の golden が落ちる。しかも失敗メッセージは「凍結 golden を
//	  再生成してはならない」であり、直し方が無い赤になる。
//	★同じ壁に本プロジェクトは一度当たっている ——
//	  zangief / dhalsim の連打版を CSV に載せていないのは「載せると golden が壊れる」
//	  ためである(csv_db_sync_test.go 冒頭・D-94 / D-99)。
//	⇒ マーカーは別モードの成果物として独立させる。moves 波の生成物は
//	  マーカーの有無に関わらず不変であり、golden は永続的に安全になる。
//	★これは derived-backfill / move-commands と同じ流儀である(新しい流儀を作っていない)。
//
// ★★あわせて games.current_data_version を引き上げる。
//
//	マーカーだけを立てて現在版を据え置くと、以後に登録されるコンボの基準が
//	マーカーより古くなり、**登録した瞬間に「影響可能性あり」で出る**。
//	⇒ 版の引き上げを同じマイグレに含めて、その状態を作れないようにする。
//	★引き下げはしない(`current_data_version < ?` を条件に置く)。
func GenerateGameVersion(charOrder []string, rowsByChar map[string][]MoveRow, header func(dir string) string) (*Result, error) {
	if err := validate(charOrder, rowsByChar); err != nil {
		return nil, err
	}

	res := &Result{
		Index: moveindex.New(), // 本モードは索引を構築しない(空のまま)
		Stats: map[string]int{},
	}

	var up strings.Builder
	up.WriteString(header("up"))
	var downBlocks []string
	maxVersion := ""

	for _, code := range charOrder {
		// 版数ごとに move code をまとめる(同じ版で変わった技は 1 本の UPDATE で済む)。
		byVersion := map[string][]MoveRow{}
		var versions []string
		var marked []MoveRow
		for _, r := range rowsByChar[code] {
			if r.isMovementSystem() {
				res.Dropped = append(res.Dropped, r)
				continue
			}
			v := strings.TrimSpace(r.LastChangedGameVersion)
			if v == "" {
				continue
			}
			if _, seen := byVersion[v]; !seen {
				versions = append(versions, v)
			}
			byVersion[v] = append(byVersion[v], r)
			marked = append(marked, r)
			if v > maxVersion {
				maxVersion = v
			}
		}
		if len(versions) == 0 {
			continue
		}
		sort.Strings(versions) // 生成物を決定論にする(map の反復順に依存しない)
		res.Stats[code] = len(marked)

		fmt.Fprintf(&up, "-- ===== %s (%d moves) =====\n", code, len(marked))
		for _, v := range versions {
			up.WriteString(buildGameVersionUpdate(code, byVersion[v], sqlStr(v)))
		}
		// ★down は NULL へ戻す。
		//   ★★戻す先は「立てる前の値」ではない —— 前の値を記録していないためである。
		//     同じ move を 2 度マークした場合、down で 1 度目の版も消える。
		//     ⇒ 失われるものは完了報告に書くこと。
		downBlocks = append(downBlocks,
			fmt.Sprintf("-- ===== %s =====\n", code)+buildGameVersionUpdate(code, marked, "NULL"))
	}

	if maxVersion != "" {
		// ★現在のデータバージョンを引き上げる(引き下げはしない)。
		fmt.Fprintf(&up,
			"-- 現在のデータバージョンを引き上げる(★マーカーより古いままにしない)\n"+
				"UPDATE games SET current_data_version = %s\nWHERE code = 'sf6' AND current_data_version < %s;\n\n",
			sqlStr(maxVersion), sqlStr(maxVersion))
		// ★down で版を戻さない —— 戻す先を記録していない。完了報告に明記する。
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

// buildGameVersionUpdate は 1 版数分の UPDATE 文を組み立てる。value は SQL リテラル or NULL。
func buildGameVersionUpdate(code string, rows []MoveRow, value string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "UPDATE moves SET last_changed_game_version = %s\n", value)
	fmt.Fprintf(&b, "WHERE character_id IN (SELECT c.id FROM characters c WHERE %s)\n", charFilter("c", code))
	b.WriteString("  AND code IN (")
	writeCodeList(&b, rows)
	b.WriteString(");\n\n")
	return b.String()
}
