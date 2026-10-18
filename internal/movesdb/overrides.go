package movesdb

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// 上書き表(data/overrides/)。規則で表せない少数を人が補う(HANDOVER D7・§4.2)。
// ★公式サイト等から転記しない(NOTICE §2)。各項目に理由(reason)を必ず書く(検査が見る)。

// NameENOverride は英語技名の上書き 1 件。
type NameENOverride struct {
	NameEN string `json:"nameEn"`
	Reason string `json:"reason"`
}

// NotationOverride は numeric / SRK の上書き 1 件(空の側は上書きしない)。
type NotationOverride struct {
	Numeric string `json:"numeric,omitempty"`
	SRK     string `json:"srk,omitempty"`
	Reason  string `json:"reason"`
}

// Overrides は上書き表の全体。キーはキャラ code → move_code。
type Overrides struct {
	NameEN   map[string]map[string]NameENOverride
	Notation map[string]map[string]NotationOverride
}

// Override ファイル名(data/overrides/ の下)。
const (
	NameENOverrideFile   = "name_en.json"
	NotationOverrideFile = "notation.json"
)

// LoadOverrides は dir の上書き表を読む。未知のキーは誤りにする(綴り違いで黙って効かないのを防ぐ)。
func LoadOverrides(dir string) (*Overrides, error) {
	ov := &Overrides{}
	if err := readStrictJSON(filepath.Join(dir, NameENOverrideFile), &ov.NameEN); err != nil {
		return nil, err
	}
	if err := readStrictJSON(filepath.Join(dir, NotationOverrideFile), &ov.Notation); err != nil {
		return nil, err
	}
	for ch, m := range ov.NameEN {
		for code, o := range m {
			if strings.TrimSpace(o.NameEN) == "" || strings.TrimSpace(o.Reason) == "" {
				return nil, fmt.Errorf("%s: %s/%s の nameEn か reason が空", NameENOverrideFile, ch, code)
			}
		}
	}
	for ch, m := range ov.Notation {
		for code, o := range m {
			if (o.Numeric == "" && o.SRK == "") || strings.TrimSpace(o.Reason) == "" {
				return nil, fmt.Errorf("%s: %s/%s の値か reason が空", NotationOverrideFile, ch, code)
			}
		}
	}
	return ov, nil
}

func readStrictJSON(path string, v any) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	dec := json.NewDecoder(f)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}
