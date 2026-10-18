package seedgen

import (
	"path/filepath"
	"testing"
)

// M20-02 の golden テスト。生成物(000072 / 000073)が CSV から再生成した SQL と
// byte-identical であることを保証する(手編集ドリフト検出。M14-03c の定石)。
//
// ★落ちたときの正しい対応は「期待値を緩める」ではなく「再生成する」である。
// 再生成コマンドは各テストの直上に書いてある。

// m2002CharOrder は本サブの投入順(character_data/*.csv に実在する 17 キャラの辞書順)。
//
// ★移動系 9 code は本定数ではなく m2002MovementChars で絞る。詳細は同定数の注記。
var m2002CharOrder = []string{
	"guile", "ingrid", "jamie", "jp", "juri", "ken", "kimberly", "lily", "luke",
	"m_bison", "mai", "manon", "marisa", "rashid", "ryu", "terry", "zangief",
}

// m2002MovementChars は移動系 9 code を投入する 19 キャラ(000072/000073 適用時点の全 characters)。
//
// ★charOrder(CSV を持つ 17 キャラ)と別に持つ理由(M20-02 レビュー H-2)。
// 移動系 9 code は CSV に存在せず(000025 系マイグレが投入)、投入先は「その波で characters 行が
// 増えたキャラ」である。★M20-02 当時は c_viper / dhalsim が攻撃技 CSV を持たなかったため
// 「CSV を持たないキャラにも投入先がある」と説明されていたが、M14-03f で両者の CSV を
// 投入したため、その理由は失効した(分離の必要は残る)。
// かつ ★up と down を同じ集合で絞る必要がある——
// 絞らないと、次の seed 波で作るマイグレの down が本マイグレの投入分まで消す。
//
// ★次の seed 波では、この定数ではなく「その波で増えたキャラ」を渡すこと。
// 手順は character_data/seed-progress.md。
var m2002MovementChars = []string{
	"c_viper", "dhalsim", "guile", "ingrid", "jamie", "jp", "juri", "ken", "kimberly",
	"lily", "luke", "m_bison", "mai", "manon", "marisa", "rashid", "ryu", "terry", "zangief",
}

const (
	numericAliasNote = "M20-02: 表記プリセット numeric のエイリアス投入(D-311 / D-314 / D-315 / D-321)"
	srkAliasNote     = "M20-02: 表記プリセット srk のエイリアス投入(D-311 / D-314 / D-315 / D-321)"
)

func readM2002Rows(t *testing.T) map[string][]MoveRow {
	t.Helper()
	root := repoRoot(t)
	rowsByChar := make(map[string][]MoveRow, len(m2002CharOrder))
	next := 0
	for _, code := range m2002CharOrder {
		rows, err := ReadFile(filepath.Join(root, "character_data", code+".csv"), next)
		if err != nil {
			t.Fatalf("ReadFile %s: %v", code, err)
		}
		rowsByChar[code] = rows
		next += len(rows)
	}
	return rowsByChar
}

// 再生成は TACPENDIUM_UPDATE_GOLDEN=1 go test ./internal/seedgen/ で行う
// (★M33-03。下の CLI は生成に使った引数の記録であり、出力先は migrations/ ではなく
//
//	 internal/seedgen/testdata/ である):
//
//		go run ./cmd/seedgen -mode aliases -preset numeric \
//		  -chars guile,ingrid,jamie,jp,juri,ken,kimberly,lily,luke,m_bison,mai,manon,marisa,rashid,ryu,terry,zangief \
//		  -movement-chars c_viper,dhalsim,guile,ingrid,jamie,jp,juri,ken,kimberly,lily,luke,m_bison,mai,manon,marisa,rashid,ryu,terry,zangief \
//		  -out 000072_m20_seed_aliases_numeric -note "<numericAliasNote の文字列>"
func TestGolden_M2002NumericAliasesMatchesRegeneration(t *testing.T) {
	// ★旧形式を指定する(format.go の FormatPreM2003)。000072 は M20-03 より前に
	//   生成・適用済みであり、character_id を含まない。改変できないため形式を固定する。
	res, err := GenerateAliases(NumericRule, m2002CharOrder, readM2002Rows(t), m2002MovementChars,
		AliasHeader("000072_m20_seed_aliases_numeric", numericAliasNote, WithFormat(FormatPreM2003)), WithFormat(FormatPreM2003))
	if err != nil {
		t.Fatalf("GenerateAliases(numeric): %v", err)
	}
	assertGolden(t, "000072_m20_seed_aliases_numeric", res.UpSQL, res.DownSQL)
}

// 再生成は TACPENDIUM_UPDATE_GOLDEN=1 go test ./internal/seedgen/ で行う
// (★M33-03。下の CLI は生成に使った引数の記録であり、出力先は migrations/ ではなく
//
//	 internal/seedgen/testdata/ である):
//
//		go run ./cmd/seedgen -mode aliases -preset srk \
//		  -chars guile,ingrid,jamie,jp,juri,ken,kimberly,lily,luke,m_bison,mai,manon,marisa,rashid,ryu,terry,zangief \
//		  -movement-chars c_viper,dhalsim,guile,ingrid,jamie,jp,juri,ken,kimberly,lily,luke,m_bison,mai,manon,marisa,rashid,ryu,terry,zangief \
//		  -out 000073_m20_seed_aliases_srk -note "<srkAliasNote の文字列>"
func TestGolden_M2002SRKAliasesMatchesRegeneration(t *testing.T) {
	// ★旧形式を指定する(000072 と同じ理由)。
	res, err := GenerateAliases(SRKRule, m2002CharOrder, readM2002Rows(t), m2002MovementChars,
		AliasHeader("000073_m20_seed_aliases_srk", srkAliasNote, WithFormat(FormatPreM2003)), WithFormat(FormatPreM2003))
	if err != nil {
		t.Fatalf("GenerateAliases(srk): %v", err)
	}
	assertGolden(t, "000073_m20_seed_aliases_srk", res.UpSQL, res.DownSQL)
}

// TestM2002_GenerationMeasurements は生成の実測値を固定する。
//
// ★件数を DoD に置くときは確定値か見込みかを書き分ける(SUPP-001 の実務知見)。
// 本サブの投入件数は生成規則の結果であり着手前には確定しない。⇒ 実測値を入れる。
//
// ★投入行と非投入行を対で固定する(SUPP-001 §5.5.2 (3))。「投入した件数」だけでは
// 規則を広く当てすぎても検出できない(件数が増えるだけでエラーにならない)。
func TestM2002_GenerationMeasurements(t *testing.T) {
	rows := readM2002Rows(t)

	for _, tc := range []struct {
		rule AliasPresetRule
		// 投入(キャラ別)
		layerB, layerA, layerRush, layerNoInput int
		// 非投入
		derived, derivedNoCommand        int
		noCommand, indexSkip             int
		rushNoOriginal, rushOrigUnfilled int
		collision                        int
		// 層 B が勝ったことで捨てた層 A の値の件数(D-2 (b))
		discarded int
	}{
		{
			rule: NumericRule,
			// 層 B は 3 接頭辞 + 強度 + ボタンの完全形。srk との差 5 件は
			// numeric だけで起きる「空中版 vs しゃがみ版」の衝突による(下記 collision 参照)。
			// ★★layerRush が 260 → 264 に増えたのは M35-03 である(SUPP-001 §5.5.4 規約 (16))。
			//   CSV の original_move_code の dangling 4 件を是正したため、元技が解決できる
			//   ようになり 4 件とも合成できた。⇒ rushNoOriginal の 4 → 0 と対である。
			//   ★衝突は起きていない(collision は不変)。⇒ 増分は素直に +4 である。
			layerB: 301, layerA: 513, layerRush: 264, layerNoInput: 13,
			// ★derivedNoCommand が 27 → 14 に減ったのは M20-06(層 C-3)である。
			//   27 件のうち値が決まった 13 件を投入するようになった(D-367 / D-373)。
			//   残る 14 件は値が未決のままであり、投入しない。
			derived: 282, derivedNoCommand: 14, noCommand: 0, indexSkip: 0,
			// ★★rushNoOriginal は「元技を解決できず合成しなかった行」である。
			//   D-305 の「推測で元技を当てない」は 1 文字も変わっていない——
			//   変わったのは、推測しなくても解決できるようになったことである(M35-03)。
			//   ⇒ 0 を主張し続けることが、dangling の再発を検出する床になる。
			rushNoOriginal: 0, rushOrigUnfilled: 8,
			collision: 84,
			discarded: 204,
		},
		{
			rule: SRKRule,
			// ★srk は cr. 接頭辞のおかげで「空中版 vs しゃがみ版」が衝突しない
			//   (crouching_heavy_punch = cr.HP / flying_body_press = 2HP)。
			//   ⇒ numeric より衝突が 10 件少なく、その分 rush も 5 件多く合成できる。
			// ★★layerRush の 265 → 269 は numeric 側と同じ理由である(M35-03)。
			layerB: 306, layerA: 518, layerRush: 269, layerNoInput: 13,
			// ★層 C-3 は記法に依らず同じ 13 件である(値の規則が name_ja の複製と
			//   move_code の整形であり、プリセットの語彙を使わないため)。
			derived: 282, derivedNoCommand: 14, noCommand: 0, indexSkip: 0,
			rushNoOriginal: 0, rushOrigUnfilled: 3,
			collision: 74,
			// ★srk は crouching_* も層 A(2LP)と別値(cr.LP)になるため、3 群すべてが記録される。
			//   numeric は crouching_* が層 A と同値(2LP)なので standing_/jumping_ の 204 件のみ。
			discarded: 306,
		},
	} {
		t.Run(tc.rule.PresetCode, func(t *testing.T) {
			// ★M48-08: 本表は M20-02 の as-built(旧規則)の実測である。⇒ FormatPreM4808 で測る。
			//   新しい規則の実測は generate_m4808_test.go の TestM4808_GenerationMeasurements。
			res, err := GenerateAliases(tc.rule, m2002CharOrder, rows, m2002MovementChars,
				AliasHeader("measure", "measure"), WithFormat(FormatPreM4808))
			if err != nil {
				t.Fatalf("GenerateAliases: %v", err)
			}

			for _, c := range []struct {
				name      string
				got, want int
			}{
				{"層 B", res.LayerStats[LayerB], tc.layerB},
				{"層 A", res.LayerStats[LayerA], tc.layerA},
				{"層 C-rush", res.LayerStats[LayerCRush], tc.layerRush},
				{"層 C-noinput", res.LayerStats[LayerCNoInput], tc.layerNoInput},
				{"キャラ別 投入合計", len(res.Rows), tc.layerB + tc.layerA + tc.layerRush + tc.layerNoInput},
				{"SQL へ出した行", len(res.Emitted), tc.layerB + tc.layerA + tc.layerRush + tc.layerNoInput},
				{"移動系 code 数", len(res.Movement), 9},
			} {
				if c.got != c.want {
					t.Errorf("%s = %d, want %d", c.name, c.got, c.want)
				}
			}

			byReason := map[unfilledReason]int{}
			for _, u := range res.Unfilled {
				byReason[u.Reason]++
			}
			for _, c := range []struct {
				reason unfilledReason
				want   int
			}{
				{ReasonDerived, tc.derived},
				{ReasonDerivedNoCommand, tc.derivedNoCommand},
				{ReasonNoCommand, tc.noCommand},
				{ReasonIndexSkip, tc.indexSkip},
				{ReasonRushNoOriginal, tc.rushNoOriginal},
				{ReasonRushOriginalUnfilled, tc.rushOrigUnfilled},
				{ReasonCollision, tc.collision},
			} {
				if got := byReason[c.reason]; got != c.want {
					t.Errorf("非投入 %s = %d, want %d", c.reason, got, c.want)
				}
			}
			if got, want := len(res.Unfilled), tc.derived+tc.derivedNoCommand+tc.noCommand+tc.indexSkip+
				tc.rushNoOriginal+tc.rushOrigUnfilled+tc.collision; got != want {
				t.Errorf("非投入 合計 = %d, want %d(理由の内訳と合わない)", got, want)
			}

			// ★捨てた層 A の値(D-2 (b)「黙って捨てない」)。
			//   層 B が勝った行のうち、層 A なら別の値になっていたものを記録する。
			//   numeric では crouching_* が層 A も層 B も同じ値(2LP 等)になるため記録されず、
			//   standing_* / jumping_* の 204 件だけが載る。srk は cr. 接頭辞で全 3 群が別値に
			//   なるため 306 件すべてが載る。
			if got := len(res.DiscardedLayerA); got != tc.discarded {
				t.Errorf("捨てた層 A の値 = %d 件, want %d", got, tc.discarded)
			}
			// ★情報が減る行は 0 件である。
			//   計画段階では marisa/zangief の *_holding 系 8 件で (hold) が落ちると
			//   見込んでいたが、実測では 0 件だった——*_holding は 4 パーツ code
			//   (standing_heavy_punch_holding)であり層 B の「接頭辞 + 強度 + ボタン」の
			//   完全形に当たらないため、層 A のまま (hold) を保持する。
			//   ⇒ 層 B 優先による情報損失は無い。ここが 0 でなくなったら層 B の
			//     適用範囲が広がったということなので、規則を見直すこと。
			lost := 0
			for _, d := range res.DiscardedLayerA {
				if d.LostInfo != "" {
					lost++
				}
			}
			if lost != 0 {
				t.Errorf("層 B 採用で情報が減る行 = %d, want 0(層 B は完全形にのみ当たるため損失は出ない)", lost)
			}

			// ★SA 注記が付かなかった super_art / critical_art は 0 件であること
			//   (M20-02 レビュー M-5)。96 行全数で saPattern の適合を実査したが、
			//   将来キャラが sa4 や別形の code を持ち込むと静かに注記なしになる。
			//   衝突しない限りエラーにもならないため、ここで数えて止める。
			if n := len(res.SAWithoutAnnotation); n != 0 {
				t.Errorf("SA 注記が付かなかった super_art / critical_art = %d 件, want 0。全件: %+v",
					n, res.SAWithoutAnnotation)
			}
		})
	}
}

// TestM2002_NoUnfilledNonDerivedNonRush は「非派生・非 rush の穴が 0 件」を固定する。
//
// ★OD 技 4 件の command 補記(000071)で穴は解消した。ここが 0 でなくなったら、
// 新しい穴ができたということである(規則の穴であり、推測で埋めてはならない)。
//
// ★M48-08 以降は派生技も層 A の対象になり、派生技の穴(例 guile の cond{ を含む OD ソニッククロス 3 件
// ＝index-skip)も Unfilled に現れる。⇒ 本テストの主張は「非派生」の穴であるため、派生技を除いて数える。
func TestM2002_NoUnfilledNonDerivedNonRush(t *testing.T) {
	rows := readM2002Rows(t)
	res, err := GenerateAliases(NumericRule, m2002CharOrder, rows, m2002MovementChars,
		AliasHeader("measure", "measure"))
	if err != nil {
		t.Fatalf("GenerateAliases: %v", err)
	}
	derived := map[string]bool{}
	for char, rs := range rows {
		for _, r := range rs {
			if r.IsDerived {
				derived[char+"/"+r.MoveCode] = true
			}
		}
	}
	var holes []UnfilledRow
	for _, u := range res.Unfilled {
		if derived[u.CharCode+"/"+u.MoveCode] {
			continue
		}
		switch u.Reason {
		case ReasonNoCommand, ReasonIndexSkip:
			holes = append(holes, u)
		}
	}
	if len(holes) != 0 {
		t.Errorf("非派生の穴 = %d 件, want 0。全件: %+v", len(holes), holes)
	}
}
