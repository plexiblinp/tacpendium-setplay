package engine

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/plexiblinp/tacpendium-setplay/internal/service/setplay"
)

func decodeSuggest(t *testing.T, b []byte) SuggestResult {
	t.Helper()
	var r SuggestResult
	if err := json.Unmarshal(b, &r); err != nil {
		t.Fatalf("%s: %v", b, err)
	}
	return r
}

func TestLoadJSON(t *testing.T) {
	e := New()
	b, err := os.ReadFile(filepath.Join(movesDir, "ryu.json"))
	if err != nil {
		t.Fatal(err)
	}
	var ok LoadResult
	if err := json.Unmarshal(e.LoadJSON(b), &ok); err != nil || ok.Character != "ryu" || ok.Error != nil {
		t.Fatalf("load ryu: %+v %v", ok, err)
	}
	var bad LoadResult
	if err := json.Unmarshal(e.LoadJSON([]byte(`{"schemaVersion":99}`)), &bad); err != nil || bad.Error == nil || bad.Error.Code != ErrCodeInvalidData {
		t.Fatalf("壊れたデータは invalid_moves_data: %+v %v", bad, err)
	}
}

func TestSuggestJSON(t *testing.T) {
	e := New()
	b, err := os.ReadFile(filepath.Join(movesDir, "ryu.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.Load(b); err != nil {
		t.Fatal(err)
	}

	r := decodeSuggest(t, e.SuggestJSON("ryu", 40, "target_types=unique"))
	if r.Error != nil || r.Result == nil || len(r.Result.Items) == 0 {
		t.Fatalf("成功のはず: %+v", r)
	}
	r = decodeSuggest(t, e.SuggestJSON("ryu", -1, ""))
	if r.Result == nil || r.Result.Reason != "knockdown_advantage_negative" || r.Result.Items == nil {
		t.Fatalf("負の KA は reason つきの空の結果(items は [] で null にしない): %+v", r.Result)
	}
	r = decodeSuggest(t, e.SuggestJSON("ken", 40, ""))
	if r.Error == nil || r.Error.Code != ErrCodeCharacterNotLoaded {
		t.Fatalf("読んでいないキャラ: %+v", r)
	}
	r = decodeSuggest(t, e.SuggestJSON("ryu", 40, "mode=xxx"))
	if r.Error == nil || r.Error.Code != ErrCodeInvalidQuery || r.Error.Param != QueryMode {
		t.Fatalf("クエリの誤り: %+v", r)
	}
	r = decodeSuggest(t, e.SuggestJSON("ryu", 40, "%zz"))
	if r.Error == nil || r.Error.Code != ErrCodeInvalidQuery {
		t.Fatalf("クエリ文字列の形の誤り: %+v", r)
	}
	r = decodeSuggest(t, InvalidArgumentsJSON("x"))
	if r.Error == nil || r.Error.Code != ErrCodeInvalidArguments {
		t.Fatalf("引数の誤り: %+v", r)
	}
}

// target_move がキャラに無い code のときは、本家(他キャラの target_move_id)と同じく誤りにせず 0 件。
func TestSuggest_UnknownTargetMoveIsEmpty(t *testing.T) {
	e := New()
	b, err := os.ReadFile(filepath.Join(movesDir, "ryu.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.Load(b); err != nil {
		t.Fatal(err)
	}
	r := decodeSuggest(t, e.SuggestJSON("ryu", 40, "target_move=no_such_move"))
	if r.Error != nil || r.Result == nil || len(r.Result.Items) != 0 || r.Result.TotalFound != 0 {
		t.Fatalf("0 件のはず: %+v", r)
	}
	r = decodeSuggest(t, e.SuggestJSON("ryu", 40, "target_move=collarbone_breaker"))
	if r.Error != nil || r.Result == nil || len(r.Result.Items) == 0 {
		t.Fatalf("実在の技は指定できる: %+v", r)
	}
	for _, it := range r.Result.Items {
		if last := it.Steps[len(it.Steps)-1]; last.Code != "collarbone_breaker" || last.Role != "target" {
			t.Fatalf("当てる技は指定の技だけ: %+v", it.Steps)
		}
	}
}

func TestKnockdownFromFloat(t *testing.T) {
	ok := map[float64]int{40: 40, 0: 0, -3: -3, 100: 100}
	for f, want := range ok {
		if got, valid := KnockdownFromFloat(f); !valid || got != want {
			t.Errorf("%v: got %d %v", f, got, valid)
		}
	}
	for _, f := range []float64{40.9, -0.5, math.NaN(), math.Inf(1), math.Inf(-1), 1e12} {
		if _, valid := KnockdownFromFloat(f); valid {
			t.Errorf("%v は誤りにするはず", f)
		}
	}
}

// 本家ハンドラは ErrInvalidRemain を 400 invalid_query_parameter にする。Query を直接組んだ経路でも同じ。
func TestApiErrorOf_InvalidRemain(t *testing.T) {
	e := New()
	b, err := os.ReadFile(filepath.Join(movesDir, "ryu.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.Load(b); err != nil {
		t.Fatal(err)
	}
	q := Query{Params: setplay.SuggestParams{Mode: setplay.ModeFiller, Remain: -1}}
	_, err = e.Suggest(context.Background(), "ryu", 40, q)
	if !errors.Is(err, setplay.ErrInvalidRemain) {
		t.Fatalf("want ErrInvalidRemain, got %v", err)
	}
	if ae := apiErrorOf(err); ae.Code != ErrCodeInvalidQuery || ae.Param != QueryRemain {
		t.Fatalf("got %+v", ae)
	}
}

// 安全飛びの値はキャラごとに違い、moves DB(本家 custom_states.safe_jump)から読む。
func TestSafeJump(t *testing.T) {
	e := New()
	for _, c := range []string{"ryu", "dhalsim"} {
		b, err := os.ReadFile(filepath.Join(movesDir, c+".json"))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := e.Load(b); err != nil {
			t.Fatal(err)
		}
	}
	// 期待値は 000010_data_seed_safe_jump の値を書き写したもの。
	want := map[string]SafeJump{"ryu": {True: 43, F5: 42, F6: 41}, "dhalsim": {True: 73, F5: 72, F6: 71}}
	for c, w := range want {
		got, err := e.SafeJump(c)
		if err != nil || got == nil || *got != w {
			t.Errorf("%s: got %+v %v, want %+v", c, got, err, w)
		}
	}
	if _, err := e.SafeJump("ken"); err == nil {
		t.Error("読んでいないキャラは誤り")
	}
}
