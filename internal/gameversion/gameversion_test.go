package gameversion

import "testing"

func TestValid(t *testing.T) {
	good := []string{
		"2026.08.03.01",
		"2026.01.01.00",
		"2026.12.31.99",
		"1999.10.09.10",
	}
	for _, s := range good {
		if !Valid(s) {
			t.Errorf("Valid(%q) = false, want true", s)
		}
	}

	// ★★本パッケージが存在する理由そのもの。ゼロ埋めが崩れた値を通すと
	// 辞書順が静かに壊れる(エラーにならない)。
	bad := []string{
		"2026.8.3.1",   // ★指示書 §2.2-3-b が名指しする形。ゼロ埋めなし
		"2026.8.03.01", // 月だけゼロ埋めなし
		"2026.08.3.01", // 日だけゼロ埋めなし
		"2026.08.03.1", // アプリ版だけ 1 桁
		"2026.08.03",   // アプリ版なし
		"2026.08.03.01.02",
		"2026.13.01.01", // 月が範囲外
		"2026.00.01.01",
		"2026.08.32.01", // 日が範囲外
		"2026.08.00.01",
		"26.08.03.01", // 年が 2 桁
		"2026-08-03-01",
		"v2026.08.03.01",
		"2026.08.03.01 ",
		"",
		"1.9",
		"1.10",
	}
	for _, s := range bad {
		if Valid(s) {
			t.Errorf("Valid(%q) = true, want false", s)
		}
		if Validate(s) == nil {
			t.Errorf("Validate(%q) = nil, want error", s)
		}
	}
}

// TestLexicographicOrderMatchesChronologicalOrder は本形式を採った理由を固定する
// (指示書 §2.2-3-a)。順序用の追加列を持たない根拠がこれである。
func TestLexicographicOrderMatchesChronologicalOrder(t *testing.T) {
	// ★時系列に並べた列。隣接ペアがすべて「文字列比較で昇順」であることを見る。
	ordered := []string{
		"2026.08.03.00",
		"2026.08.03.01",
		"2026.08.03.09",
		"2026.08.03.10", // ★ "1.9" < "1.10" 型の誤りが起きる箇所。ゼロ埋めで解けている
		"2026.08.03.99",
		"2026.08.09.00",
		"2026.08.10.00", // ★日でも同じ
		"2026.09.01.00",
		"2026.10.01.00", // ★月でも同じ
		"2027.01.01.00",
	}
	for i, s := range ordered {
		if !Valid(s) {
			t.Fatalf("test data %q is itself invalid", s)
		}
		if i == 0 {
			continue
		}
		prev := ordered[i-1]
		if !Newer(s, prev) {
			t.Errorf("Newer(%q, %q) = false, want true (辞書順が時系列順と一致していない)", s, prev)
		}
		if Newer(prev, s) {
			t.Errorf("Newer(%q, %q) = true, want false", prev, s)
		}
	}
	// 同値は「新しい」ではない(判定は基準より新しいものだけを出す)。
	if Newer("2026.08.03.01", "2026.08.03.01") {
		t.Error("Newer(x, x) = true, want false")
	}
}

// TestNaiveDottedVersionWouldBreak は「ゼロ埋めしない形式なら壊れる」ことを示し、
// 本形式の必要性を記録に残す。
func TestNaiveDottedVersionWouldBreak(t *testing.T) {
	// ゼロ埋めなしの版数だと辞書順が時系列順と食い違う。
	if !("1.10" < "1.9") {
		t.Error(`"1.10" < "1.9" が偽になった(前提が変わった)`)
	}
	// 同じ内容をゼロ埋め形式で書けば逆転しない。
	if !("2026.08.03.09" < "2026.08.03.10") {
		t.Error("ゼロ埋め形式で辞書順が逆転した")
	}
}
