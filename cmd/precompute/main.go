// Command precompute は moves DB(data/moves)から、配信する静的データ(HANDOVER §3 ②)を作る。
//
//	go run ./cmd/precompute -out build/site-data
//
// 出力(詳細は docs/precompute.md):
//
//	manifest.json             キャラ一覧と各ファイルの場所(唯一の可変ファイル)
//	moves/<code>.<hash>.json  moves DB(data/moves のバイト列そのまま)
//
// 計算はブラウザの WASM が行う(S5 の開発者決定: 計算方式 C)。S4 の繋ぎ表は S6 で消した(S5 後の開発者決定 2)。
//
// <hash> はファイルの中身の sha256 の先頭 16 桁。中身の変わらないキャラは同じ名前になる(出力は決定的)。
// 書いた後に配信の上限(Cloudflare 無料: 20,000 ファイル ・1 ファイル 25 MiB)を検査し、超えたら exit 1。
// 標準出力に件数の要約を出す(CI の job summary に貼る。2 字下げの行はキャラごとの明細)。
package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/plexiblinp/tacpendium-setplay/engine"
	setplayrepo "github.com/plexiblinp/tacpendium-setplay/internal/repository/setplay"
)

// 配信の上限(Cloudflare Workers Free の静的アセット。S1 ⑥)。
const (
	maxAssetFiles = 20000
	maxAssetBytes = 25 << 20
)

// ManifestSchemaVersion は manifest.json の形の版。
const ManifestSchemaVersion = 1

// hashLen はファイル名に入れる sha256 の桁数(16 進)。
const hashLen = 16

// Manifest は manifest.json の形。
type Manifest struct {
	SchemaVersion         int         `json:"schemaVersion"`
	DataVersion           string      `json:"dataVersion"`
	MaxKnockdownAdvantage int         `json:"maxKnockdownAdvantage"`
	Characters            []Character `json:"characters"`
}

// Character は manifest のキャラ 1 人分。
type Character struct {
	Code string          `json:"code"`
	Name json.RawMessage `json:"name"`
	// DataVersion はキャラごとの版(そのキャラのデータが最後に変わった上流の版。moves DB の dataVersion)。
	DataVersion     string                `json:"dataVersion"`
	CalcFingerprint string                `json:"calcFingerprint"`
	SafeJump        *setplayrepo.SafeJump `json:"safeJump"`
	Moves           string                `json:"moves"`
	MovesBytes      int                   `json:"movesBytes"`
}

// indexFile は data/moves/index.json のうち読む部分。
type indexFile struct {
	DataVersion string `json:"dataVersion"`
	Characters  []struct {
		Code        string          `json:"code"`
		Name        json.RawMessage `json:"name"`
		File        string          `json:"file"`
		DataVersion string          `json:"dataVersion"`
	} `json:"characters"`
}

func main() {
	movesDir := flag.String("moves", "data/moves", "moves DB の置き場")
	out := flag.String("out", "build/site-data", "出力先(無いか空のディレクトリ)")
	chars := flag.String("chars", "", "作るキャラの code(カンマ区切り。空は全キャラ)")
	flag.Parse()
	if err := run(os.Stdout, *movesDir, *out, *chars); err != nil {
		fmt.Fprintln(os.Stderr, "precompute:", err)
		os.Exit(1)
	}
}

// built はキャラ 1 人分の生成結果。
type built struct {
	moves []byte
	c     *setplayrepo.Character
	dur   time.Duration
	err   error
}

func run(w io.Writer, movesDir, out, chars string) error {
	start := time.Now()
	if err := ensureEmptyDir(out); err != nil {
		return err
	}
	var idx indexFile
	b, err := os.ReadFile(filepath.Join(movesDir, "index.json"))
	if err != nil {
		return fmt.Errorf("read index: %w", err)
	}
	if err := json.Unmarshal(b, &idx); err != nil {
		return fmt.Errorf("parse index: %w", err)
	}
	want := map[string]bool{}
	for _, c := range strings.Split(chars, ",") {
		if c != "" {
			want[c] = true
		}
	}
	type job struct {
		code, file, dataVersion string
		name                    json.RawMessage
	}
	var jobs []job
	seen := map[string]bool{}
	for _, c := range idx.Characters {
		if len(want) == 0 || want[c.Code] {
			jobs = append(jobs, job{code: c.Code, file: c.File, name: c.Name, dataVersion: c.DataVersion})
			seen[c.Code] = true
		}
	}
	var unknown []string
	for c := range want {
		if !seen[c] {
			unknown = append(unknown, c)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return fmt.Errorf("unknown characters: %s", strings.Join(unknown, ","))
	}
	if len(jobs) == 0 {
		return errors.New("no characters")
	}

	// キャラごとに並行に作る(出力の順は index.json の順に固定する)。
	results := make([]built, len(jobs))
	sem := make(chan struct{}, runtime.GOMAXPROCS(0))
	var wg sync.WaitGroup
	for i, j := range jobs {
		wg.Add(1)
		go func(i int, j job) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[i] = buildCharacter(filepath.Join(movesDir, j.file), j.code)
		}(i, j)
	}
	wg.Wait()

	m := Manifest{
		SchemaVersion:         ManifestSchemaVersion,
		DataVersion:           idx.DataVersion,
		MaxKnockdownAdvantage: engine.SiteMaxKnockdownAdvantage,
		Characters:            []Character{},
	}
	for i, r := range results {
		if r.err != nil {
			return fmt.Errorf("%s: %w", jobs[i].code, r.err)
		}
		if r.c.DataVersion == "" || r.c.DataVersion != jobs[i].dataVersion {
			return fmt.Errorf("%s: dataVersion %q != index のキャラの版 %q", jobs[i].code, r.c.DataVersion, jobs[i].dataVersion)
		}
		movesPath, err := writeHashed(out, "moves", jobs[i].code, ".json", r.moves)
		if err != nil {
			return err
		}
		m.Characters = append(m.Characters, Character{
			Code:            jobs[i].code,
			Name:            jobs[i].name,
			DataVersion:     r.c.DataVersion,
			CalcFingerprint: r.c.CalcFingerprint,
			SafeJump:        r.c.SafeJump,
			Moves:           movesPath,
			MovesBytes:      len(r.moves),
		})
		fmt.Fprintf(w, "  %-10s moves=%-7d %s\n", jobs[i].code, len(r.moves), r.dur.Round(time.Millisecond))
	}
	mb, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	if err := os.WriteFile(filepath.Join(out, "manifest.json"), append(mb, '\n'), 0o644); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}

	st, err := scan(out)
	if err != nil {
		return err
	}
	violations := checkLimits(st, maxAssetFiles, maxAssetBytes)
	fmt.Fprintf(w, "キャラ: %d\n", len(m.Characters))
	fmt.Fprintf(w, "ファイル: %d(上限 %d)\n", st.files, maxAssetFiles)
	fmt.Fprintf(w, "最大: %s %d bytes(上限 %d)\n", st.maxPath, st.maxBytes, maxAssetBytes)
	fmt.Fprintf(w, "合計: %d bytes / gzip %d bytes\n", st.total, st.gzipTotal)
	fmt.Fprintf(w, "生成: %s(GOMAXPROCS %d)\n", time.Since(start).Round(time.Millisecond), runtime.GOMAXPROCS(0))
	for _, v := range violations {
		fmt.Fprintf(w, "  違反: %s\n", v)
	}
	fmt.Fprintf(w, "違反: %d 件\n", len(violations))
	if len(violations) > 0 {
		return fmt.Errorf("%d violations", len(violations))
	}
	return nil
}

// buildCharacter はキャラ 1 人分の moves DB を読み、Engine が読めることまで確かめる。
func buildCharacter(path, code string) built {
	start := time.Now()
	moves, err := os.ReadFile(path)
	if err != nil {
		return built{err: fmt.Errorf("read moves: %w", err)}
	}
	c, err := setplayrepo.ParseCharacter(moves)
	if err != nil {
		return built{err: err}
	}
	if c.Code != code {
		return built{err: fmt.Errorf("character code %q in %s", c.Code, path)}
	}
	if c.CalcFingerprint == "" {
		return built{err: fmt.Errorf("%s: calcFingerprint is empty", path)}
	}
	if _, err := engine.New().Load(moves); err != nil {
		return built{err: err}
	}
	return built{moves: moves, c: c, dur: time.Since(start)}
}

// writeHashed は data を <out>/<dir>/<code>.<hash><ext> に書き、out からの相対パス(/ 区切り)を返す。
func writeHashed(out, dir, code, ext string, data []byte) (string, error) {
	sum := sha256.Sum256(data)
	rel := dir + "/" + code + "." + hex.EncodeToString(sum[:])[:hashLen] + ext
	p := filepath.Join(out, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return "", fmt.Errorf("mkdir: %w", err)
	}
	if err := os.WriteFile(p, data, 0o644); err != nil {
		return "", fmt.Errorf("write %s: %w", rel, err)
	}
	return rel, nil
}

// ensureEmptyDir は dir が無ければ作り、空でなければ誤りにする(前回の出力が混ざらないように)。
func ensureEmptyDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", dir, err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("read %s: %w", dir, err)
	}
	if len(entries) > 0 {
		return fmt.Errorf("output %s is not empty", dir)
	}
	return nil
}

// stats は出力の走査結果。
type stats struct {
	files     int
	total     int64
	gzipTotal int64
	maxPath   string
	maxBytes  int64
	sizes     map[string]int64
}

// scan は dir 以下の全ファイルを数える(上限の検査は書いたファイルそのものを見る)。
func scan(dir string) (stats, error) {
	st := stats{sizes: map[string]int64{}}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		n := int64(len(b))
		st.files++
		st.total += n
		st.sizes[rel] = n
		if n > st.maxBytes || st.maxPath == "" {
			st.maxPath, st.maxBytes = rel, n
		}
		var gz bytes.Buffer
		zw, err := gzip.NewWriterLevel(&gz, gzip.BestCompression)
		if err != nil {
			return err
		}
		if _, err := zw.Write(b); err != nil {
			return err
		}
		if err := zw.Close(); err != nil {
			return err
		}
		st.gzipTotal += int64(gz.Len())
		return nil
	})
	if err != nil {
		return st, fmt.Errorf("scan %s: %w", dir, err)
	}
	return st, nil
}

// checkLimits は配信の上限を超えるものを返す(ファイル数 1 件 ・大きすぎるファイルはファイルごとに 1 件)。
func checkLimits(st stats, maxFiles int, maxBytes int64) []string {
	var out []string
	if st.files > maxFiles {
		out = append(out, fmt.Sprintf("ファイル数 %d > %d", st.files, maxFiles))
	}
	paths := make([]string, 0, len(st.sizes))
	for p := range st.sizes {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		if st.sizes[p] > maxBytes {
			out = append(out, fmt.Sprintf("%s: %d bytes > %d", p, st.sizes[p], maxBytes))
		}
	}
	return out
}
