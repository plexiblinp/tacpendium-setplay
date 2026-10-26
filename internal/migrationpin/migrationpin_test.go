package migrationpin

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
)

// fakeSet は 000001〜000003 の up / down を持つ仮のマイグレ群と、その全部を固定した一覧を返す。
func fakeSet() (map[string][]byte, Manifest) {
	files := map[string][]byte{}
	m := Manifest{}
	for v := 1; v <= 3; v++ {
		for _, dir := range []string{"up", "down"} {
			name := fmt.Sprintf("%06d_fake.%s.sql", v, dir)
			body := []byte(fmt.Sprintf("SELECT %d; -- %s\n", v, dir))
			files[name] = body
			sum := sha256.Sum256(body)
			m[name] = hex.EncodeToString(sum[:])
		}
	}
	return files, m
}

func addFake(files map[string][]byte, stem string) {
	files[stem+".up.sql"] = []byte("SELECT 'up';\n")
	files[stem+".down.sql"] = []byte("SELECT 'down';\n")
}

func pin(m Manifest, files map[string][]byte, stem string) {
	for _, dir := range []string{"up", "down"} {
		name := stem + "." + dir + ".sql"
		sum := sha256.Sum256(files[name])
		m[name] = hex.EncodeToString(sum[:])
	}
}

func assertProblems(t *testing.T, label string, got []string, wantSub ...string) {
	t.Helper()
	if len(wantSub) == 0 {
		if len(got) != 0 {
			t.Errorf("%s: 赤になった(緑のはず):\n  %s", label, strings.Join(got, "\n  "))
		}
		return
	}
	joined := strings.Join(got, "\n")
	for _, w := range wantSub {
		if !strings.Contains(joined, w) {
			t.Errorf("%s: %q を含む赤が無い。got:\n  %s", label, w, strings.Join(got, "\n  "))
		}
	}
}

// TestCheck は固定と漏れの判定を在る条件 ・無い条件の両方で固定する(M43-07 指示書 §5・破壊確認の常設)。
func TestCheck(t *testing.T) {
	t.Run("対照: 全部固定済みなら緑", func(t *testing.T) {
		files, m := fakeSet()
		assertProblems(t, "対照", Check(files, m))
	})
	t.Run("1 バイト変えると赤", func(t *testing.T) {
		files, m := fakeSet()
		files["000002_fake.up.sql"] = append([]byte{}, files["000002_fake.up.sql"]...)
		files["000002_fake.up.sql"][0] ^= 1
		assertProblems(t, "改変", Check(files, m), "000002_fake.up.sql: 固定済みなのに中身が変わった")
	})
	t.Run("一覧に在るファイルが消えると赤", func(t *testing.T) {
		files, m := fakeSet()
		delete(files, "000003_fake.down.sql")
		assertProblems(t, "消失", Check(files, m), "000003_fake.down.sql: 固定の一覧に在るのに")
	})
	t.Run("(a) 一覧から途中の 1 本を外すと赤", func(t *testing.T) {
		files, m := fakeSet()
		delete(m, "000002_fake.up.sql")
		delete(m, "000002_fake.down.sql")
		assertProblems(t, "(a)", Check(files, m), "000002_fake.up.sql: 固定の一覧に無いのに", "000002_fake.down.sql: 固定の一覧に無いのに")
	})
	t.Run("(a') up だけ固定し down を外しても赤", func(t *testing.T) {
		files, m := fakeSet()
		delete(m, "000003_fake.down.sql")
		assertProblems(t, "(a')", Check(files, m), "000003_fake.down.sql: 固定の一覧に無いのに")
	})
	t.Run("(b) 末尾に仮の 2 本を足しても赤にならない", func(t *testing.T) {
		files, m := fakeSet()
		addFake(files, "000004_wip_generated")
		addFake(files, "000005_wip_handwritten")
		assertProblems(t, "(b)", Check(files, m))
	})
	t.Run("(c) 仮の 2 本のうち大きい方だけ固定すると、小さい方が赤", func(t *testing.T) {
		files, m := fakeSet()
		addFake(files, "000004_wip_generated")
		addFake(files, "000005_wip_handwritten")
		pin(m, files, "000005_wip_handwritten")
		assertProblems(t, "(c)", Check(files, m), "000004_wip_generated.up.sql: 固定の一覧に無いのに", "000004_wip_generated.down.sql: 固定の一覧に無いのに")
	})
	t.Run("(c) の対照: 2 本とも固定すれば緑", func(t *testing.T) {
		files, m := fakeSet()
		addFake(files, "000004_wip_generated")
		addFake(files, "000005_wip_handwritten")
		pin(m, files, "000004_wip_generated")
		pin(m, files, "000005_wip_handwritten")
		assertProblems(t, "(c) 対照", Check(files, m))
	})
}

func TestParse(t *testing.T) {
	sum := strings.Repeat("a", 64)
	m, err := Parse([]byte(sum + "  000001_x.up.sql\n\n" + sum + " *000001_x.down.sql\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !m.Pinned("000001_x") || m.Pinned("000002_x") {
		t.Errorf("Pinned の判定が違う: %v", m)
	}
	for _, bad := range []string{"zz  000001_x.up.sql", sum + "000001_x.up.sql", sum + "  a\n" + sum + "  a"} {
		if _, err := Parse([]byte(bad)); err == nil {
			t.Errorf("不正な一覧 %q を受け付けた", bad)
		}
	}
}

// TestLoad は埋め込みの一覧が読めて、空でないことを確かめる(実ファイルとの突き合わせは
// internal/infra/migration の TestAppliedMigrations_SHA256Pinned)。
func TestLoad(t *testing.T) {
	m, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(m) == 0 {
		t.Fatal("固定の一覧が空")
	}
}
