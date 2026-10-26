package seedgen

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/plexiblinp/tacpendium-setplay/internal/migrationpin"
)

// M43-07 の生成物テストの分け方(syncGeneratedMigration)を、一時ディレクトリで固定する(破壊確認の常設)。
//
// ★固定済み＝render を呼ばず、出し直しでも書かない ／ 未固定＝照らし合わせ、出し直しで書く。

const m4307Stem = "000099_wip_delta"

func m4307Setup(t *testing.T, up, down string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range map[string]string{m4307Stem + ".up.sql": up, m4307Stem + ".down.sql": down} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func m4307PinsOf(t *testing.T, dir string) migrationpin.Manifest {
	t.Helper()
	m := migrationpin.Manifest{}
	for _, d := range []string{"up", "down"} {
		name := m4307Stem + "." + d + ".sql"
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(b)
		m[name] = hex.EncodeToString(sum[:])
	}
	return m
}

func TestM4307_SyncGeneratedMigration(t *testing.T) {
	render := func(called *bool) func() (string, string) {
		return func() (string, string) { *called = true; return "UP-NEW\n", "DOWN-NEW\n" }
	}
	readUp := func(t *testing.T, dir string) string {
		t.Helper()
		b, err := os.ReadFile(filepath.Join(dir, m4307Stem+".up.sql"))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}

	t.Run("固定済み: 生成器の出力が変わっても赤にならず、render も呼ばない", func(t *testing.T) {
		dir := m4307Setup(t, "UP-OLD\n", "DOWN-OLD\n")
		called := false
		res := syncGeneratedMigration(dir, m4307Stem, m4307PinsOf(t, dir), false, render(&called))
		if !res.pinned || called || len(res.problems) != 0 {
			t.Errorf("pinned=%v called=%v problems=%v, want true/false/[]", res.pinned, called, res.problems)
		}
	})
	t.Run("固定済み: 出し直し(update)でも書かない", func(t *testing.T) {
		dir := m4307Setup(t, "UP-OLD\n", "DOWN-OLD\n")
		called := false
		res := syncGeneratedMigration(dir, m4307Stem, m4307PinsOf(t, dir), true, render(&called))
		if called || len(res.wrote) != 0 || readUp(t, dir) != "UP-OLD\n" {
			t.Errorf("固定済みを書いた: called=%v wrote=%v up=%q", called, res.wrote, readUp(t, dir))
		}
	})
	t.Run("固定済み: ファイルが無ければ赤", func(t *testing.T) {
		dir := m4307Setup(t, "UP-OLD\n", "DOWN-OLD\n")
		pins := m4307PinsOf(t, dir)
		if err := os.Remove(filepath.Join(dir, m4307Stem+".down.sql")); err != nil {
			t.Fatal(err)
		}
		res := syncGeneratedMigration(dir, m4307Stem, pins, false, func() (string, string) { return "", "" })
		if len(res.problems) != 1 || !strings.Contains(res.problems[0], "ファイルが無い") {
			t.Errorf("problems=%v, want ファイルが無い 1 件", res.problems)
		}
	})
	t.Run("未固定: 生成器の出力と違えば赤(照らし合わせる)", func(t *testing.T) {
		dir := m4307Setup(t, "UP-OLD\n", "DOWN-OLD\n")
		called := false
		res := syncGeneratedMigration(dir, m4307Stem, migrationpin.Manifest{}, false, render(&called))
		if res.pinned || !called || len(res.problems) != 2 {
			t.Errorf("pinned=%v called=%v problems=%v, want false/true/2 件", res.pinned, called, res.problems)
		}
	})
	t.Run("未固定: 一致すれば緑", func(t *testing.T) {
		dir := m4307Setup(t, "UP-NEW\n", "DOWN-NEW\n")
		called := false
		if res := syncGeneratedMigration(dir, m4307Stem, migrationpin.Manifest{}, false, render(&called)); len(res.problems) != 0 {
			t.Errorf("problems=%v", res.problems)
		}
	})
	t.Run("未固定: 出し直し(update)で書く", func(t *testing.T) {
		dir := m4307Setup(t, "UP-OLD\n", "DOWN-OLD\n")
		called := false
		res := syncGeneratedMigration(dir, m4307Stem, migrationpin.Manifest{}, true, render(&called))
		if len(res.wrote) != 2 || readUp(t, dir) != "UP-NEW\n" {
			t.Errorf("wrote=%v up=%q", res.wrote, readUp(t, dir))
		}
	})
}

// TestM4307_AppliedDeltaMigrationsArePinned は、今の差分マイグレ 000012 ・000014 が固定済みであることを固定する
// (⇒ 生成物テストは在ることだけを確かめる側に入る)。
func TestM4307_AppliedDeltaMigrationsArePinned(t *testing.T) {
	pins := loadPins(t)
	for _, stem := range []string{m4808Stem, m4305Stem} {
		if !pins.Pinned(stem) {
			t.Errorf("%s が固定の一覧に無い", stem)
		}
	}
}
