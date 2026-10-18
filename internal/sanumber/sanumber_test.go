package sanumber_test

import (
	"testing"

	"github.com/plexiblinp/tacpendium-setplay/internal/sanumber"
)

// 本ファイルは M22-07b で seedgen から移設した導出規則を、移設前の挙動のまま固定する。
//
// ★消費者は 2 つある（seedgen の層 A 注記 / aliasindex の第 3 段）。★規則を 2 つ持たない
// ための置き場であり、ここが唯一の正本である（指示書 M22-07b §4.2）。

func TestExtract(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
		ok   bool
	}{
		// 素の形。
		{"SA1", "sa1_shinku_hadoken", "SA1", true},
		{"SA2", "sa2_shin_shoryuken", "SA2", true},
		{"SA3", "sa3_denjin_hadoken", "SA3", true},
		{"CA", "ca_shinryu_reppa", "CA", true},

		// ★状態接頭辞つき。落とすと、同じ番号の組が多義でないように見え、
		// 「確定してはならないものを確定する」形になる（指示書 §4.2）。
		{"電刃ため", "denjin_charge_sa1_shinku_hadoken", "SA1", true},
		{"炎まとい", "flame_sa2_chou_hissatsu_shinobi_bachi", "SA2", true},
		{"風まとい", "windclad_sa2_thunderbird", "SA2", true},
		{"風破", "fuha_sa1_sakkai_fuhazan", "SA1", true},

		// 末尾に来る形（区切りが行末）。
		{"末尾の ca", "something_ca", "CA", true},

		// ★当たってはならないもの。
		{"通常技", "standing_light_punch", "", false},
		{"語中の ca", "caelum_arc", "", false},
		{"語中の sa1 相当", "kaisan", "", false},
		{"sa4 は無い", "sa4_future_move", "", false},
		{"空", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := sanumber.Extract(tt.in)
			if ok != tt.ok {
				t.Fatalf("Extract(%q) の ok = %v, want %v（got=%q）", tt.in, ok, tt.ok, got)
			}
			if got != tt.want {
				t.Errorf("Extract(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestExtract_IsUpperCase は返り値が常に大文字であることを固定する。
// seedgen の注記（" (SA1)"）と aliasindex の索引キーの両方がこれに依存している。
func TestExtract_IsUpperCase(t *testing.T) {
	for _, in := range []string{"sa1_x", "SA1_X", "Ca_y"} {
		got, ok := sanumber.Extract(in)
		if !ok {
			continue
		}
		for _, r := range got {
			if r >= 'a' && r <= 'z' {
				t.Errorf("Extract(%q) = %q に小文字が混じっている", in, got)
				break
			}
		}
	}
}

// TestExtract_TakesFirstMatchOnly は「2 つ以上の番号を含むときは先頭を採る」挙動を固定する。
//
// ★実データ 96 行に該当は無い（2026-08-18 実測）。★将来そういう move_code が現れたときに
// 「黙って先頭を採った」ことに気づけるよう、挙動を明示的に固定しておく。
func TestExtract_TakesFirstMatchOnly(t *testing.T) {
	got, ok := sanumber.Extract("sa1_foo_sa2_bar")
	if !ok {
		t.Fatal("2 つ含む move_code で当たらなかった")
	}
	if got != "SA1" {
		t.Errorf("Extract(\"sa1_foo_sa2_bar\") = %q, want \"SA1\"（先頭を採る）", got)
	}
}
