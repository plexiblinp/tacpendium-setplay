package movesdb

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestCarryDataVersions は、計算に効く列(calcFingerprint)の変わらないキャラは前の版を引き継ぎ
// (表記だけの変更でも引き継ぐ)、計算に効く列が変わったキャラ ・新しいキャラだけが新しい版(lock)になることを
// 確かめる(S4 後の開発者決定 2。2026-10-11 の見直しで「計算に効く変更だけ」に)。
func TestCarryDataVersions(t *testing.T) {
	const oldV, newV = testDataVersion, "abcdefabcdefabcdefabcdefabcdefabcdefabcd"
	same := validFile(t)
	renamed := validFile(t)
	renamed.Character.Code = "renamed"
	edited := validFile(t)
	edited.Character.Code = "edited"
	broken := validFile(t)
	broken.Character.Code = "broken"
	prevDir := t.TempDir()
	if err := WriteAll(prevDir, []CharacterFile{same, renamed, edited, broken}, oldV); err != nil {
		t.Fatal(err)
	}
	// 前の版の dataVersion が SHA の形でないファイルは引き継がない。
	b, err := os.ReadFile(filepath.Join(prevDir, "broken.json"))
	if err != nil {
		t.Fatal(err)
	}
	b = []byte(replaceOnce(string(b), oldV, "not-a-sha"))
	if err := os.WriteFile(filepath.Join(prevDir, "broken.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}

	next := func(f CharacterFile) CharacterFile { f.DataVersion = newV; return f }
	renamed2 := next(renamed)
	renamed2.Moves = append([]Move{}, renamed2.Moves...)
	renamed2.Moves[0].Notation.NameEN = "A2" // 表示だけの変更: 版は引き継ぐ
	refreshFingerprints(t, &renamed2)
	edited2 := next(edited)
	edited2.Moves = append([]Move{}, edited2.Moves...)
	startup := 5
	edited2.Moves[0].Startup = &startup // 計算に効く変更: 版が上がる
	refreshFingerprints(t, &edited2)
	added := next(same)
	added.Character.Code = "added"
	files := []CharacterFile{next(same), renamed2, edited2, next(broken), added}

	changed, err := CarryDataVersions(files, prevDir)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"edited", "broken", "added"}; !reflect.DeepEqual(changed, want) {
		t.Fatalf("版が変わったキャラ %v、期待 %v", changed, want)
	}
	got := map[string]string{}
	for _, f := range files {
		got[f.Character.Code] = f.DataVersion
	}
	if want := map[string]string{"test": oldV, "renamed": oldV, "edited": newV, "broken": newV, "added": newV}; !reflect.DeepEqual(got, want) {
		t.Fatalf("版 %v、期待 %v", got, want)
	}
	if renamed2.CalcFingerprint != renamed.CalcFingerprint || edited2.CalcFingerprint == edited.CalcFingerprint {
		t.Fatal("試験の前提: 表記の変更は指紋を変えず、発生の変更は指紋を変える")
	}
	// 何も変わらないキャラは、書き直しても前のファイルと 1 バイトも違わない(配信のファイル名が変わらない)。
	out := t.TempDir()
	if err := WriteAll(out, files, newV); err != nil {
		t.Fatal(err)
	}
	a, _ := os.ReadFile(filepath.Join(prevDir, "test.json"))
	c, _ := os.ReadFile(filepath.Join(out, "test.json"))
	if string(a) != string(c) {
		t.Fatal("中身の変わらないキャラのファイルが変わった")
	}
	_, _, errs, err := ReadAll(out)
	if err != nil || len(errs) != 0 {
		t.Fatalf("ReadAll: %v %v", err, errs)
	}
}

func replaceOnce(s, old, repl string) string {
	for i := 0; i+len(old) <= len(s); i++ {
		if s[i:i+len(old)] == old {
			return s[:i] + repl + s[i+len(old):]
		}
	}
	return s
}
