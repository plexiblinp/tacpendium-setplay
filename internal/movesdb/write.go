package movesdb

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
)

// IndexFile は data/moves/ の目次ファイル名。
const IndexFile = "index.json"

// Marshal は決定的な JSON(2 空白の字下げ・末尾改行・HTML エスケープなし)を返す。
func Marshal(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return nil, fmt.Errorf("encode json: %w", err)
	}
	return buf.Bytes(), nil
}

// WriteAll は dir にキャラ別ファイルと index.json を書く。dir にある他の *.json は消す
// (キャラが減ったときに古いファイルが残らないように)。
func WriteAll(dir string, files []CharacterFile, dataVersion string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	old, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return fmt.Errorf("glob %s: %w", dir, err)
	}
	for _, p := range old {
		if err := os.Remove(p); err != nil {
			return fmt.Errorf("remove %s: %w", p, err)
		}
	}
	idx := Index{SchemaVersion: SchemaVersion, DataVersion: dataVersion}
	for _, f := range files {
		b, err := Marshal(f)
		if err != nil {
			return err
		}
		name := f.Character.Code + ".json"
		if err := os.WriteFile(filepath.Join(dir, name), b, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
		sum := sha256.Sum256(b)
		idx.Characters = append(idx.Characters, IndexEntry{
			Code: f.Character.Code, Name: f.Character.Name, File: name,
			MoveCount: len(f.Moves), DataVersion: f.DataVersion, CalcFingerprint: f.CalcFingerprint, FileSHA256: hex.EncodeToString(sum[:]),
		})
	}
	b, err := Marshal(idx)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, IndexFile), b, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", IndexFile, err)
	}
	return nil
}

// ReadAll は dir の index.json とキャラ別ファイルを読む。未知のキーは誤りにする。
// 戻り値の違反は、index とファイルの食い違い(件数・sha256・指紋・余分なファイル)。
func ReadAll(dir string) (*Index, []CharacterFile, []string, error) {
	var idx Index
	if err := readStrictJSON(filepath.Join(dir, IndexFile), &idx); err != nil {
		return nil, nil, nil, err
	}
	var files []CharacterFile
	var errs []string
	ib, err := os.ReadFile(filepath.Join(dir, IndexFile))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("read %s: %w", IndexFile, err)
	}
	for _, miss := range missingKeys[Index](ib) {
		errs = append(errs, fmt.Sprintf("%s: キーが無い: %s", IndexFile, miss))
	}
	listed := map[string]bool{IndexFile: true}
	for _, e := range idx.Characters {
		listed[e.File] = true
		path := filepath.Join(dir, e.File)
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("read %s: %w", path, err)
		}
		var f CharacterFile
		dec := json.NewDecoder(bytes.NewReader(b))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&f); err != nil {
			return nil, nil, nil, fmt.Errorf("decode %s: %w", path, err)
		}
		for _, miss := range missingKeys[CharacterFile](b) {
			errs = append(errs, fmt.Sprintf("%s: キーが無い: %s", e.File, miss))
		}
		sum := sha256.Sum256(b)
		if hex.EncodeToString(sum[:]) != e.FileSHA256 {
			errs = append(errs, fmt.Sprintf("index: %s の fileSha256 が中身と合わない", e.File))
		}
		if f.Character.Code != e.Code || e.File != e.Code+".json" {
			errs = append(errs, fmt.Sprintf("index: %s のキャラ code が合わない", e.File))
		}
		if len(f.Moves) != e.MoveCount {
			errs = append(errs, fmt.Sprintf("index: %s の moveCount %d != %d", e.File, e.MoveCount, len(f.Moves)))
		}
		if f.CalcFingerprint != e.CalcFingerprint {
			errs = append(errs, fmt.Sprintf("index: %s の calcFingerprint が合わない", e.File))
		}
		if f.DataVersion != e.DataVersion {
			errs = append(errs, fmt.Sprintf("index: %s の dataVersion が index のキャラの版と違う", e.File))
		}
		files = append(files, f)
	}
	all, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("glob %s: %w", dir, err)
	}
	for _, p := range all {
		if name := filepath.Base(p); !listed[name] {
			errs = append(errs, "index に無いファイル: "+name)
		}
	}
	return &idx, files, errs, nil
}

// CarryDataVersions はキャラごとの版を決める(S4 後の開発者決定 2・2026-10-11 の見直し)。files は Build が作った
// 直後のもの(dataVersion = lock)。prevDir に同じキャラの前のファイルがあり、計算用の指紋(calcFingerprint。
// セットプレイの計算に効く列と安全飛びだけから作る)が同じなら、前の dataVersion を引き継ぐ。違えば(前が
// 無ければ)lock のまま。
// ★表記 ・名前 ・メモ ・ヒット時の有利などの表示だけの変更では版を上げない(開発者の指定)。そのときファイルの
// 中身は変わるので、配信のファイル名(中身のハッシュ)は変わる。戻り値は版が lock になった(＝計算に効く
// 変更があった ・新しい)キャラの code。
func CarryDataVersions(files []CharacterFile, prevDir string) ([]string, error) {
	var changed []string
	for i := range files {
		f := &files[i]
		prev, err := os.ReadFile(filepath.Join(prevDir, f.Character.Code+".json"))
		if errors.Is(err, fs.ErrNotExist) {
			changed = append(changed, f.Character.Code)
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("read previous %s: %w", f.Character.Code, err)
		}
		var head struct {
			DataVersion     string `json:"dataVersion"`
			CalcFingerprint string `json:"calcFingerprint"`
		}
		if err := json.Unmarshal(prev, &head); err != nil {
			return nil, fmt.Errorf("decode previous %s: %w", f.Character.Code, err)
		}
		if isSHA(head.DataVersion) && head.CalcFingerprint != "" && head.CalcFingerprint == f.CalcFingerprint {
			f.DataVersion = head.DataVersion
			continue
		}
		changed = append(changed, f.Character.Code)
	}
	return changed, nil
}

// ReadLock は upstream/tacpendium.lock の SHA を読む。
func ReadLock(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read lock: %w", err)
	}
	sha := strings.TrimSpace(string(b))
	if len(sha) != 40 {
		return "", fmt.Errorf("lock の SHA が 40 桁でない: %q", sha)
	}
	return sha, nil
}

// missingKeys は JSON に、型 T が持つキーの欠けが無いかを返す(入れ子の構造体・配列の要素・null でないポインタの先まで)。
// ★encoding/json は欠けたキーを黙って既定値にする。null と既定値(0・false・"")を取り違えないよう、
// キーの有無を別に確かめる(AGENTS §8-7)。
func missingKeys[T any](b []byte) []string {
	return checkKeys("", b, reflect.TypeFor[T]())
}

func checkKeys(path string, raw []byte, t reflect.Type) []string {
	trimmed := bytes.TrimSpace(raw)
	if bytes.Equal(trimmed, []byte("null")) {
		return nil // null の中身は見ない(null が許されるかは型と検査が決める)
	}
	switch t.Kind() {
	case reflect.Pointer:
		return checkKeys(path, raw, t.Elem())
	case reflect.Slice:
		var elems []json.RawMessage
		if err := json.Unmarshal(raw, &elems); err != nil {
			return []string{path + "(配列として読めない)"}
		}
		var out []string
		for i, e := range elems {
			out = append(out, checkKeys(fmt.Sprintf("%s[%d]", path, i), e, t.Elem())...)
		}
		return out
	case reflect.Struct:
		var got map[string]json.RawMessage
		if err := json.Unmarshal(raw, &got); err != nil {
			return []string{path + "(オブジェクトとして読めない)"}
		}
		var out []string
		for i := 0; i < t.NumField(); i++ {
			name, _, _ := strings.Cut(t.Field(i).Tag.Get("json"), ",")
			if name == "" || name == "-" {
				continue
			}
			p := name
			if path != "" {
				p = path + "." + name
			}
			v, ok := got[name]
			if !ok {
				out = append(out, p)
				continue
			}
			out = append(out, checkKeys(p, v, t.Field(i).Type)...)
		}
		return out
	}
	return nil
}
