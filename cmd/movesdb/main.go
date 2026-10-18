// Command movesdb は公開版 tacpendium のチェックアウトから moves DB(data/moves/*.json)を作る。
//
//	bash scripts/fetch-upstream.sh
//	go run ./cmd/movesdb -src .upstream/tacpendium
//
// 検査(HANDOVER §4.3・オラクル突合・CSV と DB の一致)に違反があれば何も書かずに exit 1。
// 標準出力に件数の要約を出す(CI の job summary に貼る)。
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/plexiblinp/tacpendium-setplay/internal/movesdb"
)

func main() {
	src := flag.String("src", ".upstream/tacpendium", "公開版 tacpendium のチェックアウト")
	out := flag.String("out", "data/moves", "出力先")
	ovDir := flag.String("overrides", "data/overrides", "上書き表の置き場")
	lock := flag.String("lock", "upstream/tacpendium.lock", "取込む版(公開版 tacpendium の SHA)")
	prev := flag.String("prev", "", "前の moves DB(キャラごとの版の引き継ぎ元。空は -out と同じ)")
	flag.Parse()
	if *prev == "" {
		*prev = *out
	}
	if err := run(context.Background(), os.Stdout, *src, *out, *ovDir, *lock, *prev); err != nil {
		fmt.Fprintln(os.Stderr, "movesdb:", err)
		os.Exit(1)
	}
}

// prev は前の moves DB の置き場。中身の変わらないキャラは、そこの dataVersion を引き継ぐ(movesdb.CarryDataVersions)。
func run(ctx context.Context, w io.Writer, src, out, ovDir, lock, prev string) error {
	sha, err := movesdb.ReadLock(lock)
	if err != nil {
		return err
	}
	ds, err := movesdb.Load(ctx, src)
	if err != nil {
		return err
	}
	ov, err := movesdb.LoadOverrides(ovDir)
	if err != nil {
		return err
	}
	files, reports, err := movesdb.Build(ds, ov, sha)
	if err != nil {
		return err
	}
	changed, err := movesdb.CarryDataVersions(files, prev)
	if err != nil {
		return err
	}

	var violations []string
	violations = append(violations, movesdb.CheckCSVAgainstDB(ds)...)
	if len(files) != movesdb.ExpectedCharacterCount {
		violations = append(violations, fmt.Sprintf("キャラ数 %d != %d", len(files), movesdb.ExpectedCharacterCount))
	}
	for _, f := range files {
		violations = append(violations, movesdb.CheckFile(f)...)
	}
	stats, oracleErrs := movesdb.CheckOracle(ds, reports)
	violations = append(violations, oracleErrs...)
	nameJADiffs := movesdb.CheckNameJA(ds)

	printSummary(w, sha, files, reports, stats, nameJADiffs, ov)
	fmt.Fprintf(w, "版が変わったキャラ(dataVersion = %s): %d %v\n", sha, len(changed), changed)
	if len(violations) > 0 {
		fmt.Fprintf(w, "\n違反: %d 件\n", len(violations))
		for _, v := range violations {
			fmt.Fprintln(w, "  "+v)
		}
		return fmt.Errorf("検査違反 %d 件(何も書いていない)", len(violations))
	}
	if err := movesdb.WriteAll(out, files, sha); err != nil {
		return err
	}
	fmt.Fprintf(w, "\n違反: 0 件\n書き出し: %s(%d キャラ + %s)\n", out, len(files), movesdb.IndexFile)
	return nil
}

func printSummary(w io.Writer, sha string, files []movesdb.CharacterFile, reports []movesdb.CharacterReport,
	stats []movesdb.OracleStats, nameJADiffs []string, ov *movesdb.Overrides) {
	moves := 0
	for _, f := range files {
		moves += len(f.Moves)
	}
	fmt.Fprintf(w, "dataVersion: %s\nキャラ: %d / 技: %d\n", sha, len(files), moves)
	for _, k := range []string{"numeric", "srk"} {
		src := map[movesdb.NotationSource]int{}
		marks := map[movesdb.Mark]int{}
		marked := 0
		unfilled := map[string]int{}
		for _, r := range reports {
			res := r.Numeric
			if k == "srk" {
				res = r.SRK
			}
			for _, s := range res.Source {
				src[s]++
			}
			for _, ms := range res.Marks {
				marked++
				for _, m := range ms {
					marks[m]++
				}
			}
			for _, u := range res.Unfilled {
				unfilled[u]++
			}
		}
		fmt.Fprintf(w, "%s: 出所 %s / 区別印の付いた技 %d(段ごと %s)/ 名前フォールバックの理由 %s\n",
			k, fmtMap(src), marked, fmtMap(marks), fmtMap(unfilled))
	}
	for _, s := range stats {
		fmt.Fprintf(w, "オラクル %s: 本家と同じ %d(うち区別印つき %d)/ 本家が被りで落とした %d / 本家が元技の表記なしで落としたラッシュ版 %d / 上書き %d / 本家にも生成器にも無い %d\n",
			s.Kind, s.Same, s.Marked, s.DroppedByUpstream, s.RushOriginalDropped, s.Override, s.NoUpstreamAlias)
	}
	byRule := map[string]int{}
	for _, r := range reports {
		for _, v := range r.NameENBy {
			byRule[v]++
		}
	}
	nNotation := 0
	for _, m := range ov.Notation {
		nNotation += len(m)
	}
	fmt.Fprintf(w, "英語技名: %s / 表記の上書き: %d\n", fmtMap(byRule), nNotation)
	fmt.Fprintf(w, "name_ja と本家 official_ja_move の差: %d\n", len(nameJADiffs))
	for _, d := range nameJADiffs {
		fmt.Fprintln(w, "  "+d)
	}
}

func fmtMap[K ~string](m map[K]int) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, string(k))
	}
	sort.Strings(keys)
	s := "{"
	for i, k := range keys {
		if i > 0 {
			s += " "
		}
		s += fmt.Sprintf("%s:%d", k, m[K(k)])
	}
	return s + "}"
}
