package engine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	setplayrepo "github.com/plexiblinp/tacpendium-setplay/internal/repository/setplay"
)

// 本家との適合性テスト。
//
// 正解(testdata/upstream_oracle.json)は scripts/upstream-oracle/run.sh が本家の実装そのもの
// (HTTP ハンドラ → サービス → SQL のリポジトリ。migrations を流した本物の DB)で作る。
// ここでは同じ条件(testdata/oracle_conditions.json)を単体版のエンジンで moves DB(data/moves)から回し、
// 条件ごとに 件数 ・truncated ・reason ・誤り ・行の正準形の sha256 が一致することを確かめる。
// 正解の作り直しと一致は CI が見る(手元で作ったものが本家の出力と違えば赤)。

const (
	oracleConditionsPath = "testdata/oracle_conditions.json"
	oraclePath           = "testdata/upstream_oracle.json"
	movesDir             = "../data/moves"
)

type oracleConditions struct {
	KAs     []int    `json:"kas"`
	Queries []string `json:"queries"`
}

type oracleCell struct {
	Character  string `json:"character"`
	KA         int    `json:"ka"`
	Query      string `json:"query"`
	Status     int    `json:"status"`
	Error      string `json:"error,omitempty"`
	TotalFound int    `json:"totalFound"`
	Truncated  bool   `json:"truncated"`
	Reason     string `json:"reason,omitempty"`
	Items      int    `json:"items"`
	SHA256     string `json:"sha256"`
}

type oracleCandidates struct {
	Character   string `json:"character"`
	Moves       int    `json:"moves"`
	Derivations int    `json:"derivations"`
	SHA256      string `json:"sha256"`
}

type oracleFile struct {
	Candidates []oracleCandidates `json:"candidates"`
	Cells      []oracleCell       `json:"cells"`
}

func intp(p *int) string {
	if p == nil {
		return "null"
	}
	return strconv.Itoa(*p)
}

// canonicalCandidates は候補行と親子関係の正準形(scripts/upstream-oracle の standaloneOracleCandidateLines と同じ書式)。
func canonicalCandidates(c *setplayrepo.Character) string {
	codeByID := make(map[int64]string, len(c.Candidates))
	for _, m := range c.Candidates {
		codeByID[m.ID] = m.Code
	}
	var b strings.Builder
	for _, m := range c.Candidates {
		orig := "null"
		if m.OriginalMoveID != nil {
			orig = codeByID[*m.OriginalMoveID]
		}
		fmt.Fprintf(&b, "M|%s|%s|%s|%s|%s|%s|%s|%t|%t|%t|%t|%s|%s|%t|%s\n", m.Code, m.Category, orig,
			intp(m.Startup), intp(m.Active), intp(m.Total), intp(m.Damage),
			m.IsDerived, m.IsAerial, m.IsProjectile, m.SetupOnly, m.StartupBasis,
			intp(m.ChainCancelTotal), m.FastestUnreachable, intp(m.OnBlock))
	}
	for _, d := range c.Derivations {
		fmt.Fprintf(&b, "D|%s|%s\n", codeByID[d.ChildMoveID], codeByID[d.ParentMoveID])
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

// 読み口の同一性: moves DB から読んだ候補行 ・親子関係(並び順を含む)が、本家の repository が
// DB から読むものと同じであること。数値 ID は違うので code で比べる。
func TestConformance_CandidatesMatchUpstreamRepository(t *testing.T) {
	var oracle oracleFile
	readJSON(t, oraclePath, &oracle)
	chars := loadAllCharacters(t)
	if len(oracle.Candidates) != len(chars) {
		t.Fatalf("正解のキャラ数 %d と moves DB のキャラ数 %d が違う", len(oracle.Candidates), len(chars))
	}
	for _, want := range oracle.Candidates {
		c, ok := chars[want.Character]
		if !ok {
			t.Errorf("%s: moves DB に無い", want.Character)
			continue
		}
		got := oracleCandidates{Character: c.Code, Moves: len(c.Candidates), Derivations: len(c.Derivations), SHA256: canonicalCandidates(c)}
		if got != want {
			t.Errorf("%s: 本家の読み口と違う\n  本家   %+v\n  単体版 %+v", want.Character, want, got)
		}
	}
}

// statusOK ・statusBadRequest は正解の Status(本家ハンドラの HTTP 状態)。
const (
	statusOK         = 200
	statusBadRequest = 400
)

// canonicalItems は行の正準形の sha256(scripts/upstream-oracle の standaloneOracleCanonical と同じ書式)。
func canonicalItems(items []Item) string {
	var b strings.Builder
	for _, it := range items {
		fmt.Fprintf(&b, "%s|%d|%d|%d|%d|%d|%d|", it.Mode, it.S, it.N, it.Landing, it.TargetActive, it.G, it.Remain)
		for j, st := range it.Steps {
			if j > 0 {
				b.WriteByte(',')
			}
			fmt.Fprintf(&b, "%s:%s:%t", st.Code, st.Role, st.Counted)
		}
		b.WriteByte('\n')
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

// runCell は 1 条件を単体版で回し、正解と同じ形の要約にする。
// resolveSafeJump は条件のクエリの {safe_jump.true} ・{safe_jump.f5} ・{safe_jump.f6} を、
// そのキャラの安全飛びの値に置き換える(本家側は scripts/upstream-oracle が本家の DB から同じことをする)。
func resolveSafeJump(e *Engine, character, query string) (string, error) {
	if !strings.Contains(query, "{safe_jump.") {
		return query, nil
	}
	sj, err := e.SafeJump(character)
	if err != nil {
		return "", err
	}
	if sj == nil {
		return "", fmt.Errorf("%s: 安全飛びの値が無い", character)
	}
	return strings.NewReplacer(
		"{safe_jump.true}", strconv.Itoa(sj.True),
		"{safe_jump.f5}", strconv.Itoa(sj.F5),
		"{safe_jump.f6}", strconv.Itoa(sj.F6),
	).Replace(query), nil
}

func runCell(e *Engine, character string, ka int, query string) (oracleCell, error) {
	cell := oracleCell{Character: character, KA: ka, Query: query}
	resolved, err := resolveSafeJump(e, character, query)
	if err != nil {
		return cell, err
	}
	v, err := url.ParseQuery(resolved)
	if err != nil {
		return cell, err
	}
	q, err := ParseQuery(v)
	if err != nil {
		var qe *QueryError
		if !errors.As(err, &qe) {
			return cell, err
		}
		cell.Status = statusBadRequest
		cell.Error = ErrCodeInvalidQuery + ": " + qe.Message
		return cell, nil
	}
	res, err := e.Suggest(context.Background(), character, ka, q)
	if err != nil {
		return cell, err
	}
	cell.Status = statusOK
	cell.TotalFound, cell.Truncated, cell.Reason = res.TotalFound, res.Truncated, res.Reason
	cell.Items = len(res.Items)
	cell.SHA256 = canonicalItems(res.Items)
	return cell, nil
}

func readJSON(t *testing.T, path string, v any) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
}

// wasmConformanceCharacters は WASM で回すときに突き合わせるキャラ。
// WASM は遅く(実測は完了報告)全キャラは CI の時間に収まらないため、連打版の型 A(ザンギエフ ・ダルシム)・
// 状態の多いキャラ(ジェイミー)・既定のキャラ(リュウ)・単位上限に早く達するキャラ(リリー)に絞る。
// ネイティブでは全キャラを回す。
// oracleCharsEnv は本家の正解と突き合わせるキャラを絞る環境変数(TestConformance_UpstreamOracle)。
const oracleCharsEnv = "SETPLAY_ORACLE_CHARS"

// oracleSmokeEnv は全キャラで回す KA の上限(安全網。TestConformance_UpstreamOracle)。
const oracleSmokeEnv = "SETPLAY_ORACLE_SMOKE_MAX_KA"

// wasmConformanceMaxKA は WASM で突き合わせる KA の上限。KA 100 の重い条件は、WASM(wasm32 の 4 GiB)で
// メモリの断片化により out of memory になることがある(S4。GOMEMLIMIT を置いても再現した)。KA 100 は
// ネイティブ(全キャラ)と Node の通し(scripts/wasm/wasm.test.mjs。WASM 経由)が見る。
const wasmConformanceMaxKA = 79

var wasmConformanceCharacters = map[string]bool{"ryu": true, "zangief": true, "dhalsim": true, "jamie": true, "lily": true}

func TestConformance_UpstreamOracle(t *testing.T) {
	var conds oracleConditions
	readJSON(t, oracleConditionsPath, &conds)
	var oracle oracleFile
	readJSON(t, oraclePath, &oracle)

	// 正解が条件のとおりに作られていること(条件を変えて正解を作り直し忘れたら赤)。
	var idx struct {
		Characters []struct {
			Code string `json:"code"`
			File string `json:"file"`
		} `json:"characters"`
	}
	readJSON(t, filepath.Join(movesDir, "index.json"), &idx)
	if want := len(idx.Characters) * len(conds.KAs) * len(conds.Queries); len(oracle.Cells) != want {
		t.Fatalf("正解の条件数 %d が 条件ファイル(キャラ %d × KA %d × クエリ %d = %d)と合わない。scripts/upstream-oracle/run.sh で作り直すこと",
			len(oracle.Cells), len(idx.Characters), len(conds.KAs), len(conds.Queries), want)
	}

	files := map[string]string{}
	for _, c := range idx.Characters {
		files[c.Code] = c.File
	}
	// oracleCharsEnv にキャラの code をカンマ区切りで入れると、そのキャラだけ突き合わせる(CI が検査済みでない
	// キャラだけを回すため。scripts/ci/verify-plan.py)。未設定は全キャラ。知らない code は誤り。
	// oracleSmokeEnv に KA を入れると、その KA 以下の条件は全キャラで回す(CI の安全網。検査済みで省いたキャラも
	// 軽く見る)。oracleCharsEnv と一緒に使える(指定のキャラは全 KA、それ以外は低い KA だけ)。
	smokeMaxKA, hasSmoke := 0, false
	if v := os.Getenv(oracleSmokeEnv); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			t.Fatalf("%s=%q: 整数でない", oracleSmokeEnv, v)
		}
		smokeMaxKA, hasSmoke = n, true
	}
	var onlyChars map[string]bool
	if spec := os.Getenv(oracleCharsEnv); spec != "" {
		onlyChars = map[string]bool{}
		for _, c := range strings.Split(spec, ",") {
			if c == "" {
				continue
			}
			if _, ok := files[c]; !ok {
				t.Fatalf("%s=%q: 知らないキャラ %q", oracleCharsEnv, spec, c)
			}
			onlyChars[c] = true
		}
	}
	byChar := map[string][]oracleCell{}
	var order []string
	for i, want := range oracle.Cells {
		if want.Character != idx.Characters[i/(len(conds.KAs)*len(conds.Queries))].Code ||
			want.KA != conds.KAs[i/len(conds.Queries)%len(conds.KAs)] || want.Query != conds.Queries[i%len(conds.Queries)] {
			t.Fatalf("正解 %d 件目の条件 (%s, %d, %q) が条件ファイルの並びと合わない", i, want.Character, want.KA, want.Query)
		}
		if runtime.GOARCH == "wasm" && (!wasmConformanceCharacters[want.Character] || want.KA > wasmConformanceMaxKA) {
			continue
		}
		if (onlyChars != nil || hasSmoke) && !onlyChars[want.Character] && !(hasSmoke && want.KA <= smokeMaxKA) {
			continue
		}
		if _, ok := byChar[want.Character]; !ok {
			order = append(order, want.Character)
		}
		byChar[want.Character] = append(byChar[want.Character], want)
	}

	// キャラごとに別の Engine で並行に回す(エンジンは読むだけの共有状態しか持たない)。
	var checked, mismatched atomic.Int64
	t.Run("characters", func(t *testing.T) {
		for _, code := range order {
			cells := byChar[code]
			t.Run(code, func(t *testing.T) {
				// WASM は単一スレッドで並行にしても速くならず、キャラごとの結果が同時にメモリに載って
				// 4 GiB の上限で落ちる(S4 の CI で実際に out of memory)。ネイティブだけ並行に回す。
				if runtime.GOARCH != "wasm" {
					t.Parallel()
				}
				e := New()
				b, err := os.ReadFile(filepath.Join(movesDir, files[code]))
				if err != nil {
					t.Fatal(err)
				}
				if _, err := e.Load(b); err != nil {
					t.Fatal(err)
				}
				bad := 0
				for _, want := range cells {
					got, err := runCell(e, want.Character, want.KA, want.Query)
					if err != nil {
						t.Fatalf("%s KA %d %q: %v", want.Character, want.KA, want.Query, err)
					}
					checked.Add(1)
					if got != want {
						mismatched.Add(1)
						bad++
						if bad <= 5 {
							t.Errorf("本家と違う: %s KA %d %q\n  本家   %+v\n  単体版 %+v", want.Character, want.KA, want.Query, want, got)
						}
					}
				}
				if bad > 0 {
					t.Errorf("%s: 不一致 %d / %d 条件", code, bad, len(cells))
				}
			})
		}
	})
	if checked.Load() == 0 {
		t.Fatal("突き合わせた条件が 0 件")
	}
	t.Logf("突き合わせた条件: %d(キャラ %d ・不一致 %d)", checked.Load(), len(order), mismatched.Load())
}
