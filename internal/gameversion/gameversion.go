// Package gameversion はゲームバージョン文字列 `YYYY.MM.DD.NN` の唯一の正本である。
//
// 形式(M28-02a・D-728):
//   - 前半 `YYYY.MM.DD` は SF6 のバランス調整の日付と一致させる
//     (プレイヤー目線で見るゲームバランスの変更という意味でのバージョンはここだけ)。
//   - 後半 `NN` は 2 桁のアプリ固有の版
//     (入力ミスの訂正・サイレント調整の見逃し・機能追加で列が増える等をここで表す)。
//
// ★★本形式の要は「全フィールドが固定幅のゼロ埋めである」ことである。
// そのため TEXT のまま辞書順で正しく並び、順序用の追加列が要らない
// (2026.08.03.01 < 2026.08.03.09 < 2026.08.03.10 < 2026.09.01.00)。
//
// ★★固定幅が崩れた瞬間に順序が壊れる。しかも壊れてもエラーにはならない。
// `2026.8.3.1` のような値が 1 つ入るだけで比較が狂う。
// ⇒ 入口を全部塞ぐこと。本パッケージはアプリ側の入口であり、他に
// DB 側の CHECK 制約(000104)・seedgen の値域検証・マイグレへ直書きする値そのもの がある。
package gameversion

import (
	"fmt"
	"regexp"
)

// Pattern は `YYYY.MM.DD.NN` の書式。月 01〜12・日 01〜31 まで見る
// (実在しない日付 2026.02.31 は通す。暦の妥当性はここでは見ない)。
var Pattern = regexp.MustCompile(`^\d{4}\.(0[1-9]|1[0-2])\.(0[1-9]|[12]\d|3[01])\.\d{2}$`)

// Valid は s が `YYYY.MM.DD.NN` の書式に合致するかを返す。
func Valid(s string) bool {
	return Pattern.MatchString(s)
}

// Validate は書式に合致しない場合にエラーを返す。
func Validate(s string) error {
	if !Valid(s) {
		return fmt.Errorf("game version %q is not in YYYY.MM.DD.NN form (zero-padded fixed width)", s)
	}
	return nil
}

// Newer は a が b より新しいかを返す。
//
// ★★書式が正しい限り、比較は単なる文字列比較でよい(固定幅ゼロ埋めのため)。
// 専用の比較関数を置くのは、呼び出し側が「なぜ文字列比較でよいのか」を
// 毎回考えずに済むようにするためである。SQL 側も同じ理由で単純な `>` を使う。
func Newer(a, b string) bool { return a > b }
