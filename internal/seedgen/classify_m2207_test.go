package seedgen

import (
	"sort"
	"strings"
	"testing"

	"github.com/plexiblinp/tacpendium-setplay/internal/sanumber"
)

// 本ファイルは M22-07 §4.2 / §3.3-4 の 2 分類を実データ全数で固定する。
//
//	(a) 機械的に導ける別名 … SA 番号(SA1 / SA2 / SA3 / CA)。move_code から導ける
//	(b) ドメイン知識が要る別名 … 技の通称(「ライズ」「フレア」等)。導けない
//
// ★導出は internal/sanumber(D-5)を使い回す。再実装しない——写しを作ると、片方だけ直した
// ときに「生成器は拾えているのに分類は落としている」という食い違いが黙って成立する。
// ★M22-07b で規則の置き場を seedgen の非公開変数から internal/sanumber へ移した
// (消費者が逆引きの第 3 段と 2 つになったため)。本テストの主張は移設前と同じである。
//
// ★★本サブは (a) を 1 行も投入していない。理由は 2 段ある。
//
//	(1) 制約が塞ぐぶん … preset_aliases は UNIQUE (preset_id, move_id)
//	    (DES-003 §3.9 制約①「1 技 1 表記」)を持つ。既に別名を持つ技へ 2 本目を足せない。
//	(2) 規約が禁じるぶん … 表記プリセットに別名を持たない技は在るが、それらは
//	    いずれも「同じ SA 番号を持つ技が同一キャラに複数ある」組に属する。
//	    1 技だけへ素の SA 番号を与えると、DES-004 §5.5 が「両方落とす」と定めた組を
//	    1 件に確定させてしまう(=別の技として確定する)。制約②は 1 組 1 行なら通るため、
//	    ★塞いでいるのは制約ではなく規約である。
//
// ★★(2) は M22-07 当時の規約である。M48-08(CHANGE-256)で投入側は「被ったら優先の 1 つだけ
// 残す」へ変わり、落ちた側は取込で候補として足される(internal/aliascollision)。⇒ 素の SA 番号を
// 1 技だけへ与えても今は「黙って確定」にはならないが、(1) の制約①は変わらず、本サブの分類
// (何組になるか)の主張も変わらない。
//
// 本ファイルが固定するのは「投入した結果」ではなく「投入するとしたら何組になるか」である。
// 詳細と裁定の選択肢は docs/progress/m22-07-completion-report.md。
//
// ★件数は本コメントに書かない。正本は下の各テストの want 定数である(E-136 / SUPP-001 §5.5.3.1)。

// saCategories は SA 番号を持つ category(層 A の saAnnotation と同じ境界)。
var saCategories = map[string]bool{"super_art": true, "critical_art": true}

// m2207DerivablePairs は (キャラ code, SA 番号) → 属する move_code(昇順)を返す。
//
// ★単位は「組」であって「技」ではない。同一キャラで 2 技が同じ SA 番号を持つ組があるため、
// 技単位で数えると投入できる行数を過大に見積もる。
func m2207DerivablePairs(t *testing.T, rowsByChar map[string][]MoveRow) map[string][]string {
	t.Helper()

	pairs := make(map[string][]string)
	var scanned, unmatched int
	for _, rows := range rowsByChar {
		for _, r := range rows {
			if !saCategories[r.Category] {
				continue
			}
			scanned++
			num, ok := sanumber.Extract(r.MoveCode)
			if !ok {
				// ★落とさずに落ちたことを主張する(D-357。件数を合わせるために行を落とさない)。
				unmatched++
				t.Errorf("move_code から SA / CA 番号を導けなかった: char=%s move=%s",
					r.CharacterCode, r.MoveCode)
				continue
			}
			key := r.CharacterCode + "\t" + num
			pairs[key] = append(pairs[key], r.MoveCode)
		}
	}
	for k := range pairs {
		sort.Strings(pairs[k])
	}

	const wantScanned = 96
	if scanned != wantScanned {
		t.Errorf("super_art + critical_art の行数 = %d, want %d", scanned, wantScanned)
	}
	if unmatched != 0 {
		t.Errorf("SA 番号を導けなかった行 = %d 件(0 件であること)", unmatched)
	}
	return pairs
}

// TestM2207_DerivableAliasesAreFullyClassified は (a) の全数分類を固定する。
//
// ★「導ける」は仮説であり、実査で確かめる(指示書 §4.2 の但し書き / §9.1-2)。本テストが
// その実査である——96 行全数から SA 番号が取れることを主張し、取れない行が出たら赤くなる。
func TestM2207_DerivableAliasesAreFullyClassified(t *testing.T) {
	pairs := m2207DerivablePairs(t, readM2002Rows(t))

	var singleton, collided int
	for _, codes := range pairs {
		if len(codes) == 1 {
			singleton++
			continue
		}
		collided++
	}

	// ★実測値で固定する。次のキャラ波で赤くなるのは正しい——増えたぶんを測り直す合図である。
	const (
		wantPairs     = 68 // (キャラ, SA 番号) の相異なる組
		wantSingleton = 53 // 1 組 1 技 = 別名を与えても同一キャラ内で衝突しない組
		wantCollided  = 15 // 1 組 2 技以上 = 同じ SA 番号を持つ技が複数ある組
	)
	if len(pairs) != wantPairs {
		t.Errorf("(キャラ, SA 番号) の組数 = %d, want %d"+
			"(internal/aliasindex の基準値テストと食い違っている)", len(pairs), wantPairs)
	}
	if singleton != wantSingleton {
		t.Errorf("1 組 1 技の組数 = %d, want %d", singleton, wantSingleton)
	}
	if collided != wantCollided {
		t.Errorf("1 組 2 技以上の組数 = %d, want %d", collided, wantCollided)
	}

	// ★衝突組へ SA 番号を与えると、DES-004 §5.5 により両方落ちる(★M22-07 当時の規約)。
	// ⇒ 投入できる行数の上限は singleton であって、96 でも 68 でもない。
	// ★M48-08(CHANGE-256)以降は被ったら優先の 1 つが残るため、「両方落ちる」は成り立たない。
	//   ⇒ 本 Logf の「上限」は M22-07 当時の読みである。組数そのもの(want 定数)は変わらない。
	t.Logf("(a) 導ける別名: (キャラ, SA 番号) %d 組 / 衝突しない %d 組 / 同一キャラ内で衝突 %d 組",
		len(pairs), singleton, collided)
}

// TestM2207_StatePrefixedSuperArtsAreNotDropped は、状態接頭辞つきの SA が
// 分類から落ちていないことを主張する。
//
// ★これが D-357 の型そのものである——`sa1_` で始まる行だけを拾う実装にすると、
// 接頭辞つきの 10 行が黙って落ち、組数が 68 から減って「きれいな数字」になる。
// 落ちた 10 行は同一キャラの素の SA と同じ番号を持つため、落とすと衝突組が
// 衝突していないように見え、★投入できない組を投入できると誤判定する。
func TestM2207_StatePrefixedSuperArtsAreNotDropped(t *testing.T) {
	// 実データに在る状態接頭辞つきの SA(2026-08-18 実査・10 行)。
	want := []string{
		"denjin_charge_sa1_shinku_hadoken",
		"denjin_charge_sa2_shin_hashogeki_lv1",
		"denjin_charge_sa2_shin_hashogeki_lv2",
		"denjin_charge_sa2_shin_hashogeki_lv3",
		"flame_sa1_kagerou_no_mai",
		"flame_sa2_air_chou_hissatsu_shinobi_bachi",
		"flame_sa2_chou_hissatsu_shinobi_bachi",
		"fuha_sa1_sakkai_fuhazan",
		"windclad_sa2_soaring_thunderbird",
		"windclad_sa2_thunderbird",
	}

	pairs := m2207DerivablePairs(t, readM2002Rows(t))

	seen := make(map[string]bool)
	for _, codes := range pairs {
		for _, c := range codes {
			seen[c] = true
		}
	}
	for _, code := range want {
		if !seen[code] {
			t.Errorf("状態接頭辞つきの SA が分類から落ちた: %s", code)
		}
	}

	// ★対照: 接頭辞つきの行は「素の SA と同じ番号」を持つため、必ず衝突組に居る。
	// 居なければ、番号の取り違え(接頭辞側を番号として読んだ等)を疑う。
	for _, code := range want {
		var found bool
		for _, codes := range pairs {
			if len(codes) < 2 {
				continue
			}
			for _, c := range codes {
				if c == code {
					found = true
				}
			}
		}
		if !found {
			t.Errorf("状態接頭辞つきの SA が衝突組に居ない: %s"+
				"(同一キャラの素の SA と同じ番号を持つはずである)", code)
		}
	}

	t.Logf("状態接頭辞つきの SA %d 行はすべて分類され、すべて衝突組に居る", len(want))
}

// TestM2207_NonDerivableCandidatesAreEnumerable は (b) の候補母集合を固定する。
//
// ★(b) は「値が導けない」のであって「対象が分からない」のではない。⇒ 対象は数えられる。
// 数えられるものを数えずに「ドメイン知識が要る」で済ませると、設計卓が受け取るのは
// 「何件あるか分からない宿題」になる(P-34 は 27 件と分かっていたから 13 件へ絞れた)。
//
// 候補の規則: 固有名を持つ 5 category。通称は固有名の短縮形であるため、系統的な表記しか
// 持たない category(normal / throw / system / target_combo / rush_variant)は対象外である。
// ★rush_variant を外す理由は「元技のエイリアス + (ラッシュ)」という派生形だからであり
// (DES-004 §3.2.1)、元技側に通称が付けばそちらへ従う。
func TestM2207_NonDerivableCandidatesAreEnumerable(t *testing.T) {
	properNameCategories := map[string]bool{
		"special": true, "super_art": true, "critical_art": true,
		"unique": true, "drive_impact": true,
	}

	rowsByChar := readM2002Rows(t)
	byCategory := make(map[string]int)
	var candidates int
	for _, rows := range rowsByChar {
		for _, r := range rows {
			if properNameCategories[r.Category] {
				candidates++
				byCategory[r.Category]++
			}
		}
	}

	const wantCandidates = 747
	if candidates != wantCandidates {
		t.Errorf("(b) 候補の技数 = %d, want %d", candidates, wantCandidates)
	}

	cats := make([]string, 0, len(byCategory))
	for c := range byCategory {
		cats = append(cats, c)
	}
	sort.Strings(cats)
	for _, c := range cats {
		t.Logf("(b) 候補: %-13s %d 技", c, byCategory[c])
	}
	t.Logf("(b) 導けない別名の候補 合計 %d 技 / ★製造が値を持つのは 0 技"+
		"(値決めは設計卓・開発者の手番。指示書 §4.2)", candidates)
}

// TestM2207_EmptySlotsAreNotUsableForSANumbers は「表記プリセットに空きがある技へ
// 素の SA 番号を入れてよいか」を実データで判定する。
//
// ★★本テストは M22-07 レビュー(高-1)の是正として置いた。当初の完了報告は
// 「空き枠は制約②で再衝突するので投入可能な行は 0 行」と書いていたが、それは誤りである——
// 制約②は (preset_id, character_id, alias_text) であり、1 組につき 1 行だけ入れる限り違反しない。
//
// ★塞いでいるのは制約ではなく規約である。空き枠はすべて「同じ SA 番号を持つ技が
// 同一キャラに複数ある」組に属するため、1 技だけへ素の SA 番号を与えると、
// DES-004 §5.5 が「両方落とす」と定めた組を 1 件に確定させてしまう。
// ⇒ 入れられるかどうか(制約)と、入れてよいかどうか(規約)は別である。
// ★「両方落とす」は M22-07 当時の規約である(M48-08 以降は優先の 1 つを残す。ファイル冒頭の注記)。
//
// ★誤りの発生経路は「件数をコメントにだけ書き、assert していなかった」ことである(E-136)。
// 本テストが空き枠の性質を assert する正本になる。
func TestM2207_EmptySlotsAreNotUsableForSANumbers(t *testing.T) {
	// 適用済み 000072(numeric)と同じ引数・同じ形式で再生成し、生成器自身の判定を使う。
	res, err := GenerateAliases(NumericRule, m2002CharOrder, readM2002Rows(t), m2002MovementChars,
		AliasHeader("000072_m20_seed_aliases_numeric", numericAliasNote, WithFormat(FormatPreM2003)),
		WithFormat(FormatPreM2003))
	if err != nil {
		t.Fatalf("GenerateAliases(numeric): %v", err)
	}

	pairs := m2207DerivablePairs(t, readM2002Rows(t))

	// 別名が出た SA / CA の move_code。
	emitted := make(map[string]bool)
	for _, r := range res.Emitted {
		emitted[r.MoveCode] = true
	}

	// 空き枠 = SA / CA でありながら別名が出なかった move。
	type slot struct{ char, num, move string }
	var empties []slot
	for key, codes := range pairs {
		charCode, num, _ := strings.Cut(key, "\t")
		for _, mc := range codes {
			if !emitted[mc] {
				empties = append(empties, slot{charCode, num, mc})
			}
		}
	}

	// ★核心の主張: 空き枠はすべて「組サイズ 2 以上」の組に属する。
	// これが成り立つ限り、空き枠へ素の SA 番号を入れると必ず §5.5 の組を 1 件に確定させる。
	inSingletonGroup := 0
	emptiesPerGroup := make(map[string]int)
	for _, e := range empties {
		key := e.char + "\t" + e.num
		emptiesPerGroup[key]++
		if len(pairs[key]) == 1 {
			inSingletonGroup++
			t.Errorf("★空き枠が組サイズ 1 の組に在る: char=%s %s move=%s"+
				"(この組は素の SA 番号を安全に与えられる。結論を見直すこと)", e.char, e.num, e.move)
		}
	}

	// ★制約②では塞がれないことも明示する: 空きが 1 技だけの組は、1 行なら UNIQUE に当たらない。
	singleEmptyGroups := 0
	for _, n := range emptiesPerGroup {
		if n == 1 {
			singleEmptyGroups++
		}
	}

	const (
		wantEmpties           = 30 // 別名が出なかった SA / CA の move
		wantGroups            = 14 // 空き枠が属する (キャラ, SA 番号) 組
		wantSingleEmptyGroups = 6  // うち「空きが 1 技だけ」= 制約②では塞がれない組
	)
	if len(empties) != wantEmpties {
		t.Errorf("空き枠の技数 = %d, want %d", len(empties), wantEmpties)
	}
	if len(emptiesPerGroup) != wantGroups {
		t.Errorf("空き枠が属する組数 = %d, want %d", len(emptiesPerGroup), wantGroups)
	}
	if singleEmptyGroups != wantSingleEmptyGroups {
		t.Errorf("空きが 1 技だけの組数 = %d, want %d", singleEmptyGroups, wantSingleEmptyGroups)
	}
	if inSingletonGroup != 0 {
		t.Errorf("組サイズ 1 の空き枠 = %d 件, want 0", inSingletonGroup)
	}

	t.Logf("空き枠 %d 技 / 属する組 %d / 空きが 1 技だけの組 %d(★制約②では塞がれない)"+
		" / 組サイズ 1 の空き枠 %d(★0 であることが「入れてよい行が無い」の根拠)",
		len(empties), len(emptiesPerGroup), singleEmptyGroups, inSingletonGroup)
}

// TestM2207_EmptySlotReasonsAreNotSingleCause は空き枠の理由が単一原因でないことを固定する。
//
// ★当初の完了報告は空き枠を「同一コマンドの衝突で両方落ちた」一本で説明していたが、
// DES-004 §5.7-4 自身が「衝突組」と「is_derived=true かつ command 空欄」を別の理由として
// 列挙しており、生成器も別の unfilledReason を持つ。⇒ 単一原因として書いてはならない。
func TestM2207_EmptySlotReasonsAreNotSingleCause(t *testing.T) {
	res, err := GenerateAliases(NumericRule, m2002CharOrder, readM2002Rows(t), m2002MovementChars,
		AliasHeader("000072_m20_seed_aliases_numeric", numericAliasNote, WithFormat(FormatPreM2003)),
		WithFormat(FormatPreM2003))
	if err != nil {
		t.Fatalf("GenerateAliases(numeric): %v", err)
	}

	saCode := make(map[string]bool)
	for key, codes := range m2207DerivablePairs(t, readM2002Rows(t)) {
		_ = key
		for _, mc := range codes {
			saCode[mc] = true
		}
	}

	byReason := make(map[unfilledReason]int)
	for _, u := range res.Unfilled {
		if saCode[u.MoveCode] {
			byReason[u.Reason]++
		}
	}

	if len(byReason) < 2 {
		t.Errorf("空き枠の理由が %d 種類しかない(単一原因として説明してはならない): %v",
			len(byReason), byReason)
	}
	reasons := make([]string, 0, len(byReason))
	for r := range byReason {
		reasons = append(reasons, string(r))
	}
	sort.Strings(reasons)
	for _, r := range reasons {
		t.Logf("空き枠の理由: %-20s %d 件", r, byReason[unfilledReason(r)])
	}
}
