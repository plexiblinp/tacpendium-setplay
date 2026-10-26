// Package migrationpin は「固定の一覧」(applied.sha256)を持ち、マイグレが一覧と食い違っていないかを判定する(M43-07)。
//
// ★★判定の基準は「固定の一覧に載っているか」である(番号の大小 ・手書きか生成かは問わない)。
//
//	一覧に載っている(固定済み) … 適用済み(または完了したサブのもの)。中身は sha256 が守る。
//	                               生成物 ・グラウンドトゥルースのテストは「在る」ことだけを確かめ、
//	                               出し直し(TACPENDIUM_UPDATE_GOLDEN=1 ・cmd/seedgen)でも書かない。
//	一覧に載っていない(未固定) … 作っている最中のサブのもの。今の CSV で照らし合わせ、出し直してよい。
//
// ★マイグレを作ったサブは、完了報告の前に自分のマイグレを一覧へ足す(⇒ マージの時点で全部が固定済み)。
//
//	(cd migrations && sha256sum 000015_xxx.up.sql 000015_xxx.down.sql) >> internal/migrationpin/applied.sha256
//
// ★レビューで未マージの自分のマイグレを直すときは、一覧の行を外し、出し直し、もう一度足す。
// 人が確かめるとき: (cd migrations && sha256sum -c ../internal/migrationpin/applied.sha256)
//
// ★本パッケージは dev ツール(cmd/seedgen)とテストからだけ引く。本体ランタイムからは引かない。
//
// 推測: 一覧の持ち方(別ファイル ・sha256sum 形式 ・本パッケージへの埋め込み)は M43-07 指示書 §9.2 で
// 製造に委ねられた点であり、3 か所(infra/migration のテスト ・seedgen のテスト ・cmd/seedgen)から
// 同じ一覧を引くためにこの形にした。
package migrationpin

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// ManifestPath は固定の一覧のリポジトリルート相対パス(案内の文面に使う)。
const ManifestPath = "internal/migrationpin/applied.sha256"

// Guidance は固定に関する赤 ・拒否のときに必ず添える文面。
const Guidance = "★固定済みのマイグレは書き換えない。直すなら新しい連番のマイグレで。" +
	"自分のマイグレを固定し忘れていないか(完了報告の前に " + ManifestPath + " へ sha256 を足す。" +
	"レビューで未マージの自分のマイグレを直すときは、行を外し・出し直し・もう一度足す)"

//go:embed applied.sha256
var manifestData []byte

// Manifest はファイル名 → sha256(16 進小文字)。
type Manifest map[string]string

// Load は埋め込みの固定の一覧を読む。
func Load() (Manifest, error) {
	return Parse(manifestData)
}

// Parse は sha256sum 形式(`<64 桁の 16 進>  <ファイル名>`)を読む。空行は読み飛ばす。
func Parse(data []byte) (Manifest, error) {
	m := Manifest{}
	sc := bufio.NewScanner(bytes.NewReader(data))
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		sum, name, ok := strings.Cut(line, " ")
		name = strings.TrimLeft(strings.TrimSpace(name), "*")
		if !ok || len(sum) != sha256.Size*2 || name == "" {
			return nil, fmt.Errorf("固定の一覧の %d 行目が sha256sum 形式でない: %q", n, line)
		}
		if _, err := hex.DecodeString(sum); err != nil {
			return nil, fmt.Errorf("固定の一覧の %d 行目の sha256 が 16 進でない: %w", n, err)
		}
		if _, dup := m[name]; dup {
			return nil, fmt.Errorf("固定の一覧に %s が 2 回ある", name)
		}
		m[name] = strings.ToLower(sum)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("固定の一覧を読めない: %w", err)
	}
	return m, nil
}

// Has はファイル名(例 000012_xxx.up.sql)が一覧に載っているかを返す。
func (m Manifest) Has(file string) bool {
	_, ok := m[file]
	return ok
}

// Pinned は stem(拡張子なし)の up と down の両方が一覧に載っているかを返す。
func (m Manifest) Pinned(stem string) bool {
	return m.Has(stem+".up.sql") && m.Has(stem+".down.sql")
}

// Version はマイグレのファイル名の先頭の連番を返す。
func Version(name string) (uint64, bool) {
	head, _, ok := strings.Cut(name, "_")
	if !ok {
		return 0, false
	}
	v, err := strconv.ParseUint(head, 10, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

// Check はマイグレのファイル群(ファイル名 → 中身)を一覧と突き合わせ、問題を返す(空なら緑)。
//
//	(1) 一覧に載っているのに sha256 が違う ⇒ 書き換えられた
//	(2) 一覧に載っているのにファイルが無い ⇒ 消えた
//	(3) 一覧に載っていないファイルの番号が「固定済みの最大の番号」より小さい ⇒ 固定漏れ
//
// ★未固定は末尾にまとまっている限り何本でも許す(1 サブで 2 本以上作る場合を含む)。
// ★固定済みの最大の番号と同じ番号の未固定(up だけ固定 等)も漏れとして扱う。
func Check(files map[string][]byte, m Manifest) []string {
	var problems []string
	var maxPinned uint64
	for name := range m {
		if v, ok := Version(name); ok && v > maxPinned {
			maxPinned = v
		}
	}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		want, pinned := m[name]
		if pinned {
			sum := sha256.Sum256(files[name])
			if got := hex.EncodeToString(sum[:]); got != want {
				problems = append(problems, fmt.Sprintf("%s: 固定済みなのに中身が変わった(sha256 = %s, want %s)", name, got, want))
			}
			continue
		}
		if v, ok := Version(name); ok && v <= maxPinned {
			problems = append(problems, fmt.Sprintf("%s: 固定の一覧に無いのに、固定済みの最大の番号(%06d)以下である(固定漏れ)", name, maxPinned))
		}
	}
	listed := make([]string, 0, len(m))
	for name := range m {
		listed = append(listed, name)
	}
	sort.Strings(listed)
	for _, name := range listed {
		if _, ok := files[name]; !ok {
			problems = append(problems, fmt.Sprintf("%s: 固定の一覧に在るのに、マイグレのファイルが無い", name))
		}
	}
	return problems
}
