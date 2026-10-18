package movesdb

import (
	"os"
	"path/filepath"
	"testing"
)

// repoRoot は go.mod のあるディレクトリ(本リポジトリのルート)を返す。
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod が見つからない")
		}
		dir = parent
	}
}

// TestCommittedMovesDB はコミット済みの data/moves を検査する(HANDOVER §4.3。公開版の取得は要らない)。
func TestCommittedMovesDB(t *testing.T) {
	root := repoRoot(t)
	sha, err := ReadLock(filepath.Join(root, "upstream", "tacpendium.lock"))
	if err != nil {
		t.Fatal(err)
	}
	idx, files, idxErrs, err := ReadAll(filepath.Join(root, "data", "moves"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range idxErrs {
		t.Error(e)
	}
	if idx.DataVersion != sha {
		t.Errorf("index の dataVersion %q != lock %q", idx.DataVersion, sha)
	}
	if idx.SchemaVersion != SchemaVersion {
		t.Errorf("index の schemaVersion %d != %d", idx.SchemaVersion, SchemaVersion)
	}
	if len(files) != ExpectedCharacterCount {
		t.Errorf("キャラ数 %d != %d", len(files), ExpectedCharacterCount)
	}
	moves := 0
	for _, f := range files {
		for _, e := range CheckFile(f) {
			t.Error(e)
		}
		moves += len(f.Moves)
	}
	t.Logf("キャラ %d / 技 %d", len(files), moves)
}
