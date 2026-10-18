package movesdb

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

// upstreamSrc は公開版 tacpendium のチェックアウト(TACPENDIUM_SRC か scripts/fetch-upstream.sh の既定の置き場)。
// 無ければ skip せずに落とす(取得し忘れで緑にしない)。
func upstreamSrc(t *testing.T) string {
	t.Helper()
	if dir := os.Getenv("TACPENDIUM_SRC"); dir != "" {
		return dir
	}
	src := filepath.Join(repoRoot(t), ".upstream", "tacpendium")
	if _, err := os.Stat(filepath.Join(src, "migrations")); err != nil {
		t.Fatalf("公開版 tacpendium が無い(%s)。bash scripts/fetch-upstream.sh で取得すること: %v", src, err)
	}
	return src
}

// TestPipeline_RegeneratesCommittedData は公開版から作り直した moves DB が、コミット済みの data/moves と
// 1 バイトも違わないことを確かめる(手元の生成物を CI の生成と一致させる。AGENTS §8-12)。
// あわせて、オラクル突合と CSV/DB の一致に違反が無いこと。
func TestPipeline_RegeneratesCommittedData(t *testing.T) {
	root := repoRoot(t)
	sha, err := ReadLock(filepath.Join(root, "upstream", "tacpendium.lock"))
	if err != nil {
		t.Fatal(err)
	}
	ds, err := Load(context.Background(), upstreamSrc(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range CheckCSVAgainstDB(ds) {
		t.Error(e)
	}
	ov, err := LoadOverrides(filepath.Join(root, "data", "overrides"))
	if err != nil {
		t.Fatal(err)
	}
	files, reports, err := Build(ds, ov, sha)
	if err != nil {
		t.Fatal(err)
	}
	stats, errs := CheckOracle(ds, reports)
	for _, e := range errs {
		t.Error(e)
	}
	for _, s := range stats {
		t.Logf("オラクル %+v", s)
	}

	committed := filepath.Join(root, "data", "moves")
	if _, err := CarryDataVersions(files, committed); err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	if err := WriteAll(out, files, sha); err != nil {
		t.Fatal(err)
	}
	gen, err := filepath.Glob(filepath.Join(out, "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	have, err := filepath.Glob(filepath.Join(committed, "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(gen) != len(have) {
		t.Errorf("ファイル数: 生成 %d / コミット済み %d", len(gen), len(have))
	}
	for _, p := range gen {
		name := filepath.Base(p)
		a, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(filepath.Join(committed, name))
		if err != nil {
			t.Errorf("コミット済みに無い: %s", name)
			continue
		}
		if !bytes.Equal(a, b) {
			t.Errorf("%s が生成と違う。go run ./cmd/movesdb で作り直してコミットすること", name)
		}
	}
}
