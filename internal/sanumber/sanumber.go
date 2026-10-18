// Package sanumber は move_code から SA / CA の番号を機械抽出する規則を持つ(★設計卓裁定 D-5)。
//
// # なぜ独立したパッケージなのか
//
// 本規則の消費者は 2 つある(M22-07b)。
//
//	internal/seedgen  … 表記プリセットの層 A が付ける注記 " (SA1)" を作る(M20-02 以来)
//	internal/aliasindex … 逆引きの第 3 段が「SA 番号 → move.code」の索引を組む(M22-07b)
//
// ★規則を 2 つ持たないための置き場である。指示書 M22-07b §4.2 が「既存の導出規則を
// そのまま使う。再実装しない」と定めており、片方だけ直したときに静かにずれる形を禁じている。
// 元は internal/seedgen の非公開変数 saPattern だったが、逆引き側(実行時)から seed 生成の
// パッケージへ依存するのは層が逆であるため、規則だけを下位へ出した。
//
// ★挙動は移設前と 1 ビットも変えていない。対照は 2 つある。
//
//	seedgen の TestAliases_SAAnnotation … 注記の形を直接主張する。★接頭辞つきの分岐を
//	                                      守っているのはこちらである
//	seedgen の golden テスト             … 生成 SQL がコミット済みマイグレと byte 一致すること
//
// ★golden は接頭辞つきの分岐を守っていない(M22-07b の破壊確認 C で実測)。接頭辞つきの
// move は is_derived=true であり、層 A へ届く前に除外されるため、注記を落としても
// 生成 SQL が変わらない。⇒ 「golden が緑だから安全」と読まないこと。
package sanumber

import (
	"regexp"
	"strings"
)

// pattern は move_code から SA / CA の番号を機械抽出する。
//
// ★技名(name_ja)からは推測しない。move_code だけを源にする(D-5)。
// 接頭辞を持つ行(fuha_sa1_… / windclad_sa2_… / flame_sa1_… / denjin_charge_sa2_…)も
// 拾えることを super_art + critical_art の 96 行全数で実査確認済み(2026-08-13 / 2026-08-18)。
//
// ★接頭辞つきを落としてはならない。落とすと、同じ番号を持つ組が多義でないように見え、
// 「確定してはならないものを確定する」という最も悪い形になる(指示書 M22-07b §4.2)。
var pattern = regexp.MustCompile(`(^|_)(sa[123]|ca)(_|$)`)

// Extract は move_code から SA / CA 番号を大文字で返す(例 "SA1" / "CA")。
// 当たらなければ空文字と false を返す。
//
// ★1 つの move_code が 2 つ以上の番号を含む場合は先頭だけを採る。実データ 96 行に該当は無い
// (2026-08-18 実測)。★該当が出たら「先頭を黙って採る」のは危ういため、そのときは
// 呼び出し側の要求（注記 1 つ / 索引キー 1 つ）ごと見直すこと。
func Extract(moveCode string) (string, bool) {
	m := pattern.FindStringSubmatch(moveCode)
	if m == nil {
		return "", false
	}
	return strings.ToUpper(m[2]), true
}
