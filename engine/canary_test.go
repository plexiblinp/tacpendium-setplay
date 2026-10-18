package engine

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	setplayrepo "github.com/plexiblinp/tacpendium-setplay/internal/repository/setplay"
	"github.com/plexiblinp/tacpendium-setplay/internal/service/setplay"
)

// 本家の internal/infra/migration の canary 2 本(TestCanary_ApproachMovesExistInSeed ・
// TestCanary_RapidVariantsNamedAfterBaseMove)を、moves DB(data/moves)に対して移植したもの。
// 本家は migrations を流した DB を SQL で見る。単体版は同じことを moves DB で見る。

// loadAllCharacters は data/moves の全キャラを code → Character で返す。
func loadAllCharacters(t *testing.T) map[string]*setplayrepo.Character {
	t.Helper()
	var idx struct {
		Characters []struct {
			Code string `json:"code"`
			File string `json:"file"`
		} `json:"characters"`
	}
	readJSON(t, filepath.Join(movesDir, "index.json"), &idx)
	out := make(map[string]*setplayrepo.Character, len(idx.Characters))
	for _, c := range idx.Characters {
		b, err := os.ReadFile(filepath.Join(movesDir, c.File))
		if err != nil {
			t.Fatal(err)
		}
		ch, err := setplayrepo.ParseCharacter(b)
		if err != nil {
			t.Fatal(err)
		}
		out[c.Code] = ch
	}
	if len(out) == 0 {
		t.Fatal("data/moves にキャラが無い")
	}
	return out
}

// missingApproachCodes は辞書のうち moves DB に無い「キャラ/code」を返す(本家の同名関数に当たる)。
func missingApproachCodes(chars map[string]*setplayrepo.Character, dict map[string][]string) []string {
	var missing []string
	for ch, codes := range dict {
		have := map[string]bool{}
		if c, ok := chars[ch]; ok {
			for _, m := range c.Candidates {
				have[m.Code] = true
			}
		}
		for _, code := range codes {
			if !have[code] {
				missing = append(missing, ch+"/"+code)
			}
		}
	}
	sort.Strings(missing)
	return missing
}

func TestCanary_ApproachMovesExistInMovesDB(t *testing.T) {
	chars := loadAllCharacters(t)

	// 対照: 書き間違いを拾えること(本家と同じ対照)。
	bad := map[string][]string{"ryu": {"hadoken_light", "no_such_move"}, "no_such_char": {"hadoken_light"}}
	if got := strings.Join(missingApproachCodes(chars, bad), ","); got != "no_such_char/hadoken_light,ryu/no_such_move" {
		t.Fatalf("対照: 書き間違いを拾えていない: %s", got)
	}

	if missing := missingApproachCodes(chars, setplay.CharacterApproachMoves()); len(missing) != 0 {
		t.Errorf("接近技の辞書に moves DB に無い code がある:\n  %s", strings.Join(missing, "\n  "))
	}
}

func TestCanary_RapidVariantsNamedAfterBaseMove(t *testing.T) {
	chars := loadAllCharacters(t)
	var got []string
	for chCode, c := range chars {
		hasParent := map[int64]bool{}
		for _, d := range c.Derivations {
			hasParent[d.ChildMoveID] = true
		}
		chainByCode := map[string]bool{}
		for _, m := range c.Candidates {
			if m.ChainCancelTotal != nil {
				chainByCode[m.Code] = true
			}
		}
		for _, m := range c.Candidates {
			// 本家の SQL と同じ条件: チェーングループ員 ・total ≥ 1 ・startup_basis が standalone か unknown ・親が在る。
			if m.ChainCancelTotal == nil || m.Total == nil || *m.Total < 1 {
				continue
			}
			if m.StartupBasis != "standalone" && m.StartupBasis != "unknown" {
				continue
			}
			if !hasParent[m.ID] {
				continue
			}
			got = append(got, chCode+"/"+m.Code)
			if !strings.HasSuffix(m.Code, "_rapid") {
				t.Errorf("(1) 連打版の code が _rapid で終わらない: %s/%s", chCode, m.Code)
				continue
			}
			if base := strings.TrimSuffix(m.Code, "_rapid"); !chainByCode[base] {
				t.Errorf("(2) 連打版 %s/%s の元技 %s がチェーングループ員として在らない", chCode, m.Code, base)
			}
		}
	}
	sort.Strings(got)
	want := "dhalsim/crouching_light_punch_rapid,zangief/crouching_light_kick_rapid,zangief/crouching_light_punch_rapid,zangief/standing_light_punch_rapid"
	if strings.Join(got, ",") != want {
		t.Errorf("(3) 連打版の行 = %s, want %s", strings.Join(got, ","), want)
	}
}
