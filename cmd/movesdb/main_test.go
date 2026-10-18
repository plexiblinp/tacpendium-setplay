package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRun_MatchesCommittedData は CLI(run)を通しで回し、出力がコミット済みの data/moves と同じことを確かめる
// (CLI → JSON の配線。AGENTS §8-5)。公開版の取得が要る(scripts/fetch-upstream.sh)。
func TestRun_MatchesCommittedData(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	src := os.Getenv("TACPENDIUM_SRC")
	if src == "" {
		src = filepath.Join(root, ".upstream", "tacpendium")
	}
	out := t.TempDir()
	var log bytes.Buffer
	if err := run(context.Background(), &log, src, out,
		filepath.Join(root, "data", "overrides"), filepath.Join(root, "upstream", "tacpendium.lock"),
		filepath.Join(root, "data", "moves")); err != nil {
		t.Fatalf("run: %v\n%s", err, log.String())
	}
	if !strings.Contains(log.String(), "違反: 0 件") {
		t.Errorf("要約に「違反: 0 件」が無い:\n%s", log.String())
	}
	committed, err := os.ReadFile(filepath.Join(root, "data", "moves", "index.json"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(out, "index.json"))
	if err != nil {
		t.Fatal(err)
	}
	// index.json は全ファイルの sha256 を持つので、一致すれば全ファイルが一致する。
	if !bytes.Equal(got, committed) {
		t.Errorf("CLI の出力の index.json がコミット済みと違う")
	}
}
