package setplay

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

const sampleMovesDB = `{
  "schemaVersion": 1, "dataVersion": "abc",
  "character": {"code": "t", "name": {"ja": "", "en": ""}},
  "states": [], "safeJump": {"true": 43, "f5": 42, "f6": 41}, "calcFingerprint": "",
  "moves": [
    {"code": "c", "category": "unique", "startup": 29, "active": 3, "total": 51, "onBlock": -4, "damage": 1000,
     "isDerived": true, "isAerial": false, "isProjectile": false, "setupOnly": false, "startupBasis": "through",
     "chainCancelTotal": null, "fastestUnreachable": false, "originalMove": null, "parents": ["b", "a"]},
    {"code": "a", "category": "normal", "startup": 4, "active": 3, "total": 13, "onBlock": null, "damage": null,
     "isDerived": false, "isAerial": true, "isProjectile": true, "setupOnly": true, "startupBasis": "standalone",
     "chainCancelTotal": 9, "fastestUnreachable": true, "originalMove": null, "parents": []},
    {"code": "b", "category": "rush_variant", "startup": null, "active": null, "total": null, "onBlock": 2, "damage": 0,
     "isDerived": true, "isAerial": false, "isProjectile": false, "setupOnly": false, "startupBasis": "unknown",
     "chainCancelTotal": null, "fastestUnreachable": false, "originalMove": "a", "parents": ["a"]}
  ]
}`

func ip(n int) *int { return &n }

func TestParseCharacter(t *testing.T) {
	c, err := ParseCharacter([]byte(sampleMovesDB))
	if err != nil {
		t.Fatal(err)
	}
	if c.Code != "t" || c.DataVersion != "abc" || c.SafeJump == nil || *c.SafeJump != (SafeJump{True: 43, F5: 42, F6: 41}) {
		t.Fatalf("キャラ: %+v", c)
	}
	// ID は配列の位置 + 1。null は nil のまま(0 にしない)。
	orig := int64(2)
	want := []MoveCandidate{
		{ID: 1, Code: "c", Category: "unique", Startup: ip(29), Active: ip(3), Total: ip(51), Damage: ip(1000), IsDerived: true,
			StartupBasis: "through", OnBlock: ip(-4)},
		{ID: 2, Code: "a", Category: "normal", Startup: ip(4), Active: ip(3), Total: ip(13), IsAerial: true, IsProjectile: true,
			SetupOnly: true, StartupBasis: "standalone", ChainCancelTotal: ip(9), FastestUnreachable: true},
		{ID: 3, Code: "b", Category: "rush_variant", OriginalMoveID: &orig, Damage: ip(0), IsDerived: true, StartupBasis: "unknown", OnBlock: ip(2)},
	}
	if !reflect.DeepEqual(c.Candidates, want) {
		t.Errorf("候補:\n got  %+v\n want %+v", c.Candidates, want)
	}
	// 本家の ORDER BY child_move_id, parent_move_id と同じ並び(parents の code 順ではない)。
	wantD := []MoveDerivation{{ChildMoveID: 1, ParentMoveID: 2}, {ChildMoveID: 1, ParentMoveID: 3}, {ChildMoveID: 3, ParentMoveID: 2}}
	if !reflect.DeepEqual(c.Derivations, wantD) {
		t.Errorf("親子: got %+v want %+v", c.Derivations, wantD)
	}
}

func TestParseCharacter_Errors(t *testing.T) {
	cases := map[string]string{
		"版違い":        strings.Replace(sampleMovesDB, `"schemaVersion": 1`, `"schemaVersion": 2`, 1),
		"キャラ code 空": strings.Replace(sampleMovesDB, `"code": "t"`, `"code": ""`, 1),
		"技 code 空":   strings.Replace(sampleMovesDB, `"code": "c"`, `"code": ""`, 1),
		"技 code 重複":  strings.Replace(sampleMovesDB, `"code": "c"`, `"code": "a"`, 1),
		"basis 空":    strings.Replace(sampleMovesDB, `"startupBasis": "through"`, `"startupBasis": ""`, 1),
		"元技が無い":      strings.Replace(sampleMovesDB, `"originalMove": "a"`, `"originalMove": "zz"`, 1),
		"親が無い":       strings.Replace(sampleMovesDB, `"parents": ["b", "a"]`, `"parents": ["b", "zz"]`, 1),
		"JSON でない":   "{",
	}
	for name, data := range cases {
		if data == sampleMovesDB {
			t.Fatalf("%s: 置き換えが効いていない", name)
		}
		if _, err := ParseCharacter([]byte(data)); err == nil {
			t.Errorf("%s: 誤りにならない", name)
		}
	}
}

func TestStore(t *testing.T) {
	c, err := ParseCharacter([]byte(sampleMovesDB))
	if err != nil {
		t.Fatal(err)
	}
	s := NewStore()
	s.Add(c)
	ctx := context.Background()
	if got, err := s.ListCharacterMoveCandidates(ctx, "t"); err != nil || len(got) != 3 {
		t.Errorf("候補: %d %v", len(got), err)
	}
	if got, err := s.ListCharacterMoveDerivations(ctx, "t"); err != nil || len(got) != 3 {
		t.Errorf("親子: %d %v", len(got), err)
	}
	if _, err := s.ListCharacterMoveCandidates(ctx, "x"); !errors.Is(err, ErrCharacterNotFound) {
		t.Errorf("無いキャラ: %v", err)
	}
	if _, err := s.ListCharacterMoveDerivations(ctx, "x"); !errors.Is(err, ErrCharacterNotFound) {
		t.Errorf("無いキャラ: %v", err)
	}
}
