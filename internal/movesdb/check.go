package movesdb

import (
	"fmt"
	"sort"
	"strings"

	"github.com/plexiblinp/tacpendium-setplay/internal/seedgen"
)

// 検査(HANDOVER §4.3)。違反は文字列の一覧で返す(空なら合格)。

// ExpectedCharacterCount は moves DB のキャラ数(本家 characters の行数。キャラが増えたら直す)。
const ExpectedCharacterCount = 31

// notationKinds は 4 表記の名前と取り出し方。
var notationKinds = []struct {
	name string
	get  func(Notation) string
}{
	{"nameJa", func(n Notation) string { return n.NameJA }},
	{"nameEn", func(n Notation) string { return n.NameEN }},
	{"srk", func(n Notation) string { return n.SRK }},
	{"numeric", func(n Notation) string { return n.Numeric }},
}

// CheckFile は 1 キャラ分の moves DB を検査する。
//   - 形: schemaVersion・dataVersion(40 桁の 16 進。キャラごとの版＝そのキャラが最後に変わった上流の版)・キャラ code・技 code の一意
//   - 全技が 4 表記を持つ(空白だけも不可)
//   - 同一キャラ × 同一表記種で、表示文字列が完全一致する技が 0 件
//   - originalMove・parents が同じキャラの実在の技を指す
//   - 指紋が中身と一致する
func CheckFile(f CharacterFile) []string {
	var errs []string
	add := func(format string, a ...any) {
		errs = append(errs, f.Character.Code+": "+fmt.Sprintf(format, a...))
	}
	if f.SchemaVersion != SchemaVersion {
		add("schemaVersion %d != %d", f.SchemaVersion, SchemaVersion)
	}
	if !isSHA(f.DataVersion) {
		add("dataVersion %q が 40 桁の 16 進でない", f.DataVersion)
	}
	if f.Character.Code == "" || f.Character.Name.JA == "" || f.Character.Name.EN == "" {
		add("キャラの code / 名前が空")
	}
	if len(f.Moves) == 0 {
		add("技が 0 件")
	}
	stateCodes := map[string]bool{}
	for _, s := range f.States {
		stateCodes[s.Code] = true
	}
	known := map[string]bool{}
	for _, m := range f.Moves {
		if m.State != nil && !stateCodes[*m.State] {
			add("%s の state %s が states に無い", m.Code, *m.State)
		}
		if known[m.Code] {
			add("技 code が重複: %s", m.Code)
		}
		known[m.Code] = true
	}
	for _, k := range notationKinds {
		seen := map[string][]string{}
		for _, m := range f.Moves {
			v := k.get(m.Notation)
			if strings.TrimSpace(v) == "" {
				add("%s の %s が空", m.Code, k.name)
				continue
			}
			seen[v] = append(seen[v], m.Code)
		}
		keys := make([]string, 0, len(seen))
		for v := range seen {
			keys = append(keys, v)
		}
		sort.Strings(keys)
		for _, v := range keys {
			if len(seen[v]) > 1 {
				add("%s %q が %d 技で同じ: %s", k.name, v, len(seen[v]), strings.Join(seen[v], ", "))
			}
		}
	}
	for _, m := range f.Moves {
		if m.OriginalMove != nil && !known[*m.OriginalMove] {
			add("%s の originalMove %s が無い", m.Code, *m.OriginalMove)
		}
		if m.Parents == nil {
			add("%s の parents が null(無いときは [])", m.Code)
		}
		for _, p := range m.Parents {
			if !known[p] {
				add("%s の親 %s が無い", m.Code, p)
			}
		}
		if !sort.StringsAreSorted(m.Parents) {
			add("%s の parents が昇順でない", m.Code)
		}
		fp, err := moveFingerprint(m)
		if err != nil {
			add("%v", err)
		} else if fp != m.CalcFingerprint {
			add("%s の calcFingerprint が中身と合わない", m.Code)
		}
	}
	if fp, err := characterFingerprint(f); err != nil {
		add("%v", err)
	} else if fp != f.CalcFingerprint {
		add("calcFingerprint が中身と合わない")
	}
	return errs
}

// OracleStats はオラクル突合(本家 DB の preset_aliases との比較)の件数。
type OracleStats struct {
	Kind string
	// Same は基底の表記が本家 DB と同じ技。
	Same int
	// Marked は Same のうち、被りの区別印が付いて表示が本家と違う技。
	Marked int
	// DroppedByUpstream は本家 DB に表記が無く、本家の規則で回すと被り(移動系の予約表記との被りを含む)で
	// 落ちる技(seedgen の理由 collision)。
	DroppedByUpstream int
	// RushOriginalDropped は本家 DB に表記が無く、本家の規則で回すと元技の表記が無いために落ちるラッシュ版
	// (理由 rush-original-unfilled)。
	RushOriginalDropped int
	// Override は上書き表で値を決めた技。
	Override int
	// NoUpstreamAlias は本家 DB にも生成器にも表記が無く、名前フォールバックにした技。
	NoUpstreamAlias int
}

// CheckOracle は生成した numeric / SRK の基底の表記を、本家 DB の preset_aliases と突き合わせる
// (HANDOVER §4.2「本家 preset_aliases は照合用オラクル」)。説明のつかない差を違反として返す。
//
// 説明のつく差(どれも本家の規則で回し直して確かめる。「被りがある」だけでは通さない):
//   - 本家 DB に表記が無く、本家の規則で回すと被り(移動系の予約表記を含む)で落ちる技
//   - 本家 DB に表記が無く、本家の規則で回すと元技の表記が無いために落ちるラッシュ版
//   - 本家 DB にも表記が無く、生成器も作れなかった技(名前フォールバック)
//   - 上書き表(data/overrides/notation.json)で決めた技
func CheckOracle(ds *Dataset, reports []CharacterReport) ([]OracleStats, []string) {
	var errs []string
	byCode := map[string]CharacterReport{}
	for _, r := range reports {
		byCode[r.Code] = r
	}
	var stats []OracleStats
	for _, kind := range []struct {
		name   string
		preset string
		get    func(CharacterReport) *NotationResult
		rule   seedgen.AliasPresetRule
	}{
		{"numeric", PresetNumeric, func(r CharacterReport) *NotationResult { return r.Numeric }, seedgen.NumericRule},
		{"srk", PresetSRK, func(r CharacterReport) *NotationResult { return r.SRK }, seedgen.SRKRule},
	} {
		st := OracleStats{Kind: kind.name}
		for _, c := range ds.Characters {
			res := kind.get(byCode[c.Code])
			if res == nil {
				errs = append(errs, fmt.Sprintf("%s/%s: 生成の記録が無い", kind.name, c.Code))
				continue
			}
			upstreamUnfilled := seedgen.UpstreamUnfilled(kind.rule, c.Code, ds.CSV[c.Code])
			for _, m := range ds.Moves[c.ID] {
				src := res.Source[m.Code]
				base := res.Base[m.Code]
				a, has := ds.Aliases[kind.preset][m.ID]
				up := a.Text
				if kind.name == "srk" && a.TextEn != "" {
					up = a.TextEn
				}
				switch {
				case src == SourceOverride:
					st.Override++
				case has && base == up:
					st.Same++
					if res.Text[m.Code] != base {
						st.Marked++
					}
				case has:
					errs = append(errs, fmt.Sprintf("%s/%s/%s: 基底 %q が本家 %q と違う(出所 %s)", kind.name, c.Code, m.Code, base, up, src))
				case src == SourceName:
					st.NoUpstreamAlias++
				case upstreamUnfilled[m.Code] == "collision":
					st.DroppedByUpstream++
				case upstreamUnfilled[m.Code] == "rush-original-unfilled":
					st.RushOriginalDropped++
				default:
					errs = append(errs, fmt.Sprintf("%s/%s/%s: 本家 DB に表記が無いのに、本家の規則では落ちない(基底 %q・出所 %s・本家の規則での理由 %q)",
						kind.name, c.Code, m.Code, base, src, upstreamUnfilled[m.Code]))
				}
			}
		}
		stats = append(stats, st)
	}
	return stats, errs
}

// CheckNameJA は CSV の name_ja と本家 DB の official_ja_move(日本語技名)を比べ、違う技を返す。
// ★単体版の nameJa は CSV の name_ja を正とする(HANDOVER §4.2)。差は報告のための材料で、違反ではない。
func CheckNameJA(ds *Dataset) []string {
	var diffs []string
	for _, c := range ds.Characters {
		ids := map[string]int64{}
		for _, m := range ds.Moves[c.ID] {
			ids[m.Code] = m.ID
		}
		for _, r := range ds.CSV[c.Code] {
			id, ok := ids[r.MoveCode]
			if !ok {
				diffs = append(diffs, fmt.Sprintf("%s/%s: CSV の技が DB に無い", c.Code, r.MoveCode))
				continue
			}
			if a := ds.Aliases[PresetOfficialJA][id].Text; a != r.NameJA {
				diffs = append(diffs, fmt.Sprintf("%s/%s: name_ja %q / official_ja_move %q", c.Code, r.MoveCode, r.NameJA, a))
			}
		}
	}
	return diffs
}

// CheckCSVAgainstDB は CSV と DB の計算列が一致するかを見る(本家の TestCSVAndDBAgreeOnFrameCostColumns と
// 同じ趣旨。データ修正の migration が CSV と DB の片方だけを直していないか)。
func CheckCSVAgainstDB(ds *Dataset) []string {
	var errs []string
	eq := func(a, b *int) bool {
		if a == nil || b == nil {
			return a == nil && b == nil
		}
		return *a == *b
	}
	for _, c := range ds.Characters {
		byCode := map[string]DBMove{}
		for _, m := range ds.Moves[c.ID] {
			byCode[m.Code] = m
		}
		for _, r := range ds.CSV[c.Code] {
			m, ok := byCode[r.MoveCode]
			if !ok {
				errs = append(errs, fmt.Sprintf("%s/%s: CSV の技が DB に無い", c.Code, r.MoveCode))
				continue
			}
			var bad []string
			if m.Category != r.Category {
				bad = append(bad, "category")
			}
			if !eq(m.Startup, r.Startup) {
				bad = append(bad, "startup")
			}
			if !eq(m.Active, r.Active) {
				bad = append(bad, "active")
			}
			if !eq(m.Total, r.Total) {
				bad = append(bad, "total")
			}
			if !eq(m.OnBlock, r.OnBlock) {
				bad = append(bad, "on_block")
			}
			if !eq(m.Damage, r.Damage) {
				bad = append(bad, "damage")
			}
			if !eq(m.ChainCancelTotal, r.ChainCancelTotal) {
				bad = append(bad, "chain_cancel_total")
			}
			if m.IsDerived != r.IsDerived {
				bad = append(bad, "is_derived")
			}
			if m.IsAerial != r.IsAerial {
				bad = append(bad, "is_aerial")
			}
			if m.IsProjectile != r.IsProjectile {
				bad = append(bad, "is_projectile")
			}
			if m.FastestUnreachable != r.FastestUnreachable {
				bad = append(bad, "fastest_unreachable")
			}
			if m.StartupBasis != r.StartupBasis && !(r.StartupBasis == "" && m.StartupBasis == "unknown") {
				bad = append(bad, "startup_basis")
			}
			if len(bad) > 0 {
				errs = append(errs, fmt.Sprintf("%s/%s: CSV と DB が違う列 %s", c.Code, r.MoveCode, strings.Join(bad, ",")))
			}
		}
	}
	return errs
}

// isSHA は s が git のコミット SHA(40 桁の小文字 16 進)の形か。
func isSHA(s string) bool {
	if len(s) != 40 {
		return false
	}
	for _, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}
