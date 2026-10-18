package seedgen

import (
	"fmt"
	"sort"
	"strings"

	"github.com/plexiblinp/tacpendium-setplay/internal/moveindex"
	"github.com/plexiblinp/tacpendium-setplay/internal/sanumber"
)

// 本ファイルは M20-02 の生成系を提供する——表記プリセット(numeric / srk)の
// preset_aliases を character_data/*.csv から生成する「規則」の実体である。
//
// ★本サブが作るのは「17 キャラ分を入れるマイグレ」ではなく「規則」である(D-181)。
// 1 回きりの backfill は、適用時点の行だけを対象にして以後 INSERT される行を静かに
// 取り残す。character_data は全キャラ分が揃っておらず後続の seed 波が確実に来るため、
// 新キャラ CSV を投入するたびに本生成器を再適用して新しいマイグレを起こす運用とする
// (手順は character_data/seed-progress.md)。
//
// 生成物は手編集しない。golden テストで drift を検出する(M14-03c の定石)。

// ---- 層順の原理(★設計卓裁定 D-1・2026-08-13) --------------------------------
//
// 「command が表記を決める行は層 A、move_code の構造が決める行は層 B」。
//
// 3 接頭辞(standing_ / crouching_ / jumping_)を層 B が先に取るのは例外規定ではなく、
// この原理の帰結である。実データでは立ち技と J 攻撃の command が同一であり
// (standing_light_punch / jumping_light_punch とも "p_l")、command は地上/空中を
// 表現していない。その情報は move_code 側が持つ。⇒ この 3 群は move_code の構造が
// 表記を決める行であり、層 B が正しい。
//
// 層 A を先に当てると (1) DES-004 §3.4 の 5LP に一致しない (2) 同 j.HP に一致しない
// (3) 立ち技と J 攻撃が同表記になり同一キャラ内の衝突が 6 キー x 17 キャラ増える、
// の 3 つが同時に起きる。
//
// 判断に迷う行はこの 1 文で決め、根拠を完了報告に書くこと。

// aliasLayer は当該行の表記をどの層が決めたかを表す。
type aliasLayer string

const (
	// LayerA は command(moveindex 正規化キー)が表記を決めた行。
	LayerA aliasLayer = "A"
	// LayerB は move_code の構造(3 接頭辞 + 強度 + ボタン)が表記を決めた行。
	LayerB aliasLayer = "B"
	// LayerCMovement は移動系 9 code(§4.6 の固定表)。
	LayerCMovement aliasLayer = "C-movement"
	// LayerCRush は rush_variant(元技の表記との合成)。
	LayerCRush aliasLayer = "C-rush"
	// LayerCNoInput は「何も押さずに派生する技」(P-34・M20-06)。
	//
	// DES-004 §3.4.1 の層 C が挙げる 3 群のうちの 1 つである。command が空で
	// is_derived=true のため層 A も層 B も当たらず、規則をどう広げても埋まらない。
	// ⇒ 対象の集合だけを固定表で持ち、値は規則で作る(noInputDerivedTargets の注記)。
	LayerCNoInput aliasLayer = "C-noinput"
)

// unfilledReason は行が埋まらなかった理由。
type unfilledReason string

const (
	// ReasonDerived は is_derived=true のため層 A の対象外だった行(★設計卓裁定 U-2)。
	//
	// ★★【2026-09-27 = M48-08 / CHANGE-256】U-2 は開発者決定 D-1037 で撤回された。
	// ⇒ FormatCurrent では本理由は 1 件も出ない(派生技も層 A の対象である)。
	// 本理由が出るのは旧規則を凍結した FormatPreM4808 / FormatPreM2003 だけであり、
	// 凍結 golden と差分マイグレの引き算の基準のために残してある。下の段落は当時の理由である。
	//
	// move_commands は is_derived=true の行を載せない(moveindex の skip 規則。逆引きの
	// 決定論を守るため——派生技は元技と同じ command を持つことが多く、載せると 1:N になる)。
	// 本生成器も同じ境界を採る。実測では、派生を層 A へ広げると正味 +40 行のために
	// 元技 68 行が衝突で落ちる(中ソニックブーム等)ため、割に合わない。
	ReasonDerived unfilledReason = "derived"
	// ReasonDerivedNoCommand は is_derived=true かつ command が空欄の行(★D-315)。
	//
	// ReasonDerived から切り出してあるのは、この類だけ扱いが違うためである——
	// 他の派生行は command を持っており、規則を広げれば埋められる(U-2 で広げないと決めた)。
	// 本類は command 自体が無いので、規則をどう広げても埋まらない。
	//
	// ★母数は 27 件である(D-315 が起票した時点では 28 件だった)。
	// lily/windclad_od_condor_spire の 1 件は「入力が無いから空欄」ではなく
	// 「入力があるのに空欄」= 入力漏れだったため、開発者確認のうえ CSV へ補記した
	// (d dr r plus k k)。
	//
	// ★【2026-08-14 更新 = M20-06 / D-373】27 件のうち 13 件は値が決まり、層 C-3 として
	// 投入するようになった(noInputDerivedTargets を参照)。⇒ 本理由に残るのは 14 件である。
	ReasonDerivedNoCommand unfilledReason = "derived-no-command"
	// ReasonNoCommand は command が空欄で層 A が成立しなかった行(非派生)。
	ReasonNoCommand unfilledReason = "no-command"
	// ReasonIndexSkip は cond{ / raw{ / 語彙外トークンで正規化できなかった行。
	ReasonIndexSkip unfilledReason = "index-skip"
	// ReasonRushNoOriginal は rush_variant だが元技を解決できなかった行(D-305)。
	ReasonRushNoOriginal unfilledReason = "rush-no-original"
	// ReasonRushOriginalUnfilled は元技側の表記が無く合成できなかった rush 行。
	ReasonRushOriginalUnfilled unfilledReason = "rush-original-unfilled"
	// ReasonCollision は同一キャラ内で表記が衝突したため投入しない行。
	//
	// ★FormatCurrent(M48-08 以降)では衝突した組のうち優先の 1 つだけが残り、本理由は
	// 「残らなかった側」に付く(Detail に残った側を書く)。旧規則(FormatPreM4808 以前)では
	// 衝突した組の全員に付いていた(§9.3-5)。
	ReasonCollision unfilledReason = "collision"
)

// AliasRow は投入する 1 行(キャラ別。移動系 9 code は別経路)。
type AliasRow struct {
	CharCode    string
	MoveCode    string
	AliasText   string
	AliasTextEn string // "" は NULL
	Layer       aliasLayer
}

// UnfilledRow は投入しない 1 行(穴・衝突・対象外)。完了報告の全件列挙に使う。
type UnfilledRow struct {
	CharCode string
	MoveCode string
	Category string
	NameJA   string
	Reason   unfilledReason
	// Detail は衝突相手の move_code など、報告に要る補足。
	Detail string
}

// AliasResult は生成の成果物。
type AliasResult struct {
	UpSQL   string
	DownSQL string
	// Rows はキャラ別に解決できた行の全件(層 A / B / C-rush / C-noinput)。
	// CharCode→MoveCode 昇順。★衝突判定の母数であり、SQL へ出た行とは限らない。
	Rows []AliasRow
	// Emitted は実際に SQL へ出た行。WithNoInputOnly を使わない限り Rows と一致する。
	Emitted []AliasRow
	// Movement は移動系 9 code(全キャラへ 1 文で投入するため行に展開しない)。
	Movement []AliasRow
	// Unfilled は投入しない行の全件。
	Unfilled []UnfilledRow
	// LayerStats は層別の投入行数。
	LayerStats map[aliasLayer]int
	// DiscardedLayerA は層 B が勝ったときに捨てた層 A の値(★D-2 (b)「黙って捨てない」)。
	DiscardedLayerA []DiscardedLayerAValue
	// SAWithoutAnnotation は super_art / critical_art でありながら saPattern が当たらず
	// 注記が付かなかった行(★M20-02 レビュー M-5)。
	//
	// ★0 件であることを機械で守るための経路である。将来キャラが sa4 や別形の code を
	// 持ち込むと、衝突しない限り静かに注記なしで投入され、表記が不揃いになる。
	SAWithoutAnnotation []UnfilledRow
	// Collisions は同一キャラ内で表記が被った組のうち、技が残った組の全件(M48-08)。
	//
	// ★他から引っ越し(取込)の逆引きが、被った表記を黙って残った技へ決めないための材料である
	// (internal/aliascollision の生成元)。★移動系 9 code の表記と被って落ちた行は含めない——
	// その表記は移動系として 1 件で確定させる(M48-08 Plan 未決 5 の開発者決定)。
	// ★旧規則(FormatPreM4808 以前)では常に空である(被った組は全員落ちていた)。
	Collisions []AliasCollision
}

// AliasCollision は同一キャラ内で表記が被った 1 組(M48-08)。
type AliasCollision struct {
	CharCode  string
	AliasText string
	// Kept は表記を残した技の move_code。
	Kept string
	// Dropped は表記を作らなかった技の move_code(昇順)。
	Dropped []string
}

// DiscardedLayerAValue は層 B が優先されたことで使われなかった層 A の値。
type DiscardedLayerAValue struct {
	CharCode  string
	MoveCode  string
	LayerAKey string // command を正規化した値
	LayerBKey string // 実際に採用した値
	// LostInfo は層 A にあって層 B に無い情報(例 "(hold)")。空なら情報損失なし。
	LostInfo string
}

// AliasPresetRule は 1 プリセット分の生成規則。
type AliasPresetRule struct {
	// PresetCode は presets.code(投入先の解決に使う)。
	PresetCode string
	// 層 B の接頭辞写像(DES-004 §3.4 のサンプル表が固定点)。
	StandingPrefix  string
	CrouchingPrefix string
	JumpingPrefix   string
	// Movement は移動系 9 code の表(§4.6)。key = move_code。
	Movement map[string]movementAlias
}

type movementAlias struct {
	text   string
	textEn string
}

// movementCodeOrder は移動系 9 code の出力順(§4.6 の表の並び。決定論のため固定)。
var movementCodeOrder = []string{
	"forward", "back", "dash_forward", "dash_back",
	"jump_neutral", "jump_forward", "jump_back", "micro_forward", "micro_back",
}

// NumericRule は numeric プリセットの生成規則(DES-004 §3.4)。
var NumericRule = AliasPresetRule{
	PresetCode:      "numeric",
	StandingPrefix:  "5",
	CrouchingPrefix: "2",
	JumpingPrefix:   "j.",
	Movement: map[string]movementAlias{
		"forward":      {text: "6"},
		"back":         {text: "4"},
		"dash_forward": {text: "66"},
		"dash_back":    {text: "44"},
		"jump_neutral": {text: "8"},
		"jump_forward": {text: "9"},
		"jump_back":    {text: "7"},
		// ★この 2 code だけ alias_text が日本語になる(D-314 / D-321)。
		// 両記法に「微歩き」「微下がり」を表す表記が存在しないためである。素直に書くと
		// forward と同じ 6 になり、DES-004 §2.1 が 1 入力 = 1 move の原則でわざわざ
		// 別 move にした区別が表示から消え、逆引きも「複数件 → 未解決」に倒れる。
		// ⇒ 元の名前を明示的に持たせ、英語表示のための値を alias_text_en に添える。
		"micro_forward": {text: "微歩き", textEn: "microwalk"},
		"micro_back":    {text: "微下がり", textEn: "back microwalk"},
	},
}

// SRKRule は srk プリセットの生成規則(DES-004 §3.4)。
//
// ★srk は numeric の機械変換ではない。通常技は接頭辞が変わり(5→st. / 2→cr. /
// j. は同じ)、必殺技は数字のまま、SA は括弧付きの注記が入り、移動は別語彙である。
// ★DES-004 §3.3 が「表記ゆれは 1 つに決め打つ」と定めており、s. ではなく st. を採る。
var SRKRule = AliasPresetRule{
	PresetCode:      "srk",
	StandingPrefix:  "st.",
	CrouchingPrefix: "cr.",
	JumpingPrefix:   "j.",
	Movement: map[string]movementAlias{
		"forward":       {text: "f"},
		"back":          {text: "b"},
		"dash_forward":  {text: "dash"},
		"dash_back":     {text: "backdash"},
		"jump_neutral":  {text: "nj"},
		"jump_forward":  {text: "fj"},
		"jump_back":     {text: "bj"},
		"micro_forward": {text: "微歩き", textEn: "microwalk"},
		"micro_back":    {text: "微下がり", textEn: "back microwalk"},
	},
}

// ---- 層 C-3: 何も押さずに派生する技(P-34・D-367 / D-373) --------------------

// noInputDerivedTargets は表記プリセットへエイリアスを投入する
// 「何も押さずに派生する技」の集合(★13 move_code。D-367 で確定)。
//
// ★本表が持つのは *集合* だけであり、*値* は持たない。値は 2 本の規則で作る(§4.2)。
//
//	alias_text    … 実データの name_ja をそのまま複製する(括弧の種類・記号も書き換えない)
//	alias_text_en … move_code を humanizeMoveCode で人間可読へ整形する
//
// ⇒ 「新しい情報を作らない」がコード上でも成立する。値を表へ書くと、CSV の name_ja が
// 直ったときに表だけが古くなる(E-76 と同型)。
//
// ★既 seed 19 キャラでは、同分類は実データに 27 件あるが、投入するのはそのうち 13 件である
// (D-367 の開発者判断)。とくに jp/departure_window_double_warp_od は「エッジケースであり
// フォールバックでよい」として意図的に外してある。★19 キャラ側へ 14 件目を足さないこと
// (生成器テストが行の不在を主張している)。
//
// ★★【2026-09-02 更新 = M14-03f / 開発者判断】第四波 14 キャラぶんの 5 件を足した(下記)。
// 同分類は第四波の CSV に 11 件あるが、投入するのは「特定の技を入力したあと、何も押さずに
// 待っていたら出る技」に逐語で当たる 5 件だけである(開発者判断 2026-09-02)。
// 外した 6 件と理由:
// ・akuma/gou_hadoken_max_holding_{light,medium,heavy} … ボタンを押し続けている(何も押さずに待つのとは違う)
// ・aki/sinister_slide_cancel ／ chun_li/serenity_stream_cancel … 上入力が要る
// ・alex/prowler_stance_cancel … 解除操作であり、notes_tool に入力の記述が無い
// ⇒ 外した 6 件は numeric / srk で official_ja_move(日本語技名)へフォールバックする。
// 画面は壊れず、テストも緑である(DES-004 §5.3)。
//
// ★第四波の 5 件は 000090 / 000091(通常の -mode aliases)に含まれる。旧 000076 / 000077 と同型の
// -noinput-only マイグレは起こさない——あの 2 本は旧 000072 / 000073 が M20-06 より前に
// 生成されたことの追補であり、M20-06 以降に起こす波では層 C-3 が通常実行に含まれるためである。
//
// ★本表は「行を作るための入力」であって「表示時に引かれるもの」ではない(§4.3-3)。
// 表示・逆引きの経路から本表を参照してはならない——参照すると、カスタムプリセットを
// 選んでいても組み込みの値が出る。
var noInputDerivedTargets = map[string]map[string]bool{
	"kimberly": {"arc_step": true, "arc_step_od": true},
	"lily":     {"condor_dive_follow_up": true, "windclad_od_condor_dive_follow_up": true},
	"jp":       {"departure_shadow_od": true},
	"marisa":   {"scutum_counterattack": true, "scutum_counterattack_od": true},
	"m_bison":  {"psycho_mine_auto_detonation": true},
	"rashid": {
		"buffed_dash_forward": true, "buffed_dash_back": true,
		"buffed_jump_forward": true, "buffed_jump_neutral": true, "buffed_jump_back": true,
	},

	// ── 第四波(M14-03f・2026-09-02 開発者判断)────────────────────────────────
	// cammy: notes_tool の逐語「フーリガンから何もしなかったら出る技。フーリガン強度不問」。
	"cammy": {
		"razors_edge_slicer": true, "razors_edge_slicer_od": true,
		"razors_edge_slicer_holding": true,
	},
	// akuma: notes_tool の逐語「弱中強百鬼襲から派生」/「OD百鬼襲から派生」。
	"akuma": {"demon_low_slash": true, "demon_low_slash_od": true},
}

// isNoInputDerivedTarget は当該行が層 C-3 の投入対象かを返す。
func isNoInputDerivedTarget(charCode, moveCode string) bool {
	return noInputDerivedTargets[charCode][moveCode]
}

// noInputDerivedTargetCount は層 C-3 の対象 move_code の総数を返す(テスト・報告用)。
func noInputDerivedTargetCount() int {
	n := 0
	for _, codes := range noInputDerivedTargets {
		n += len(codes)
	}
	return n
}

// humanizeMoveCode は move_code を人間可読な英語表記へ整形する(§4.2 の規則 2)。
//
// 規則は 3 つだけである。
//   - `_` を半角スペースへ
//   - トークン od は OD へ大文字化
//   - 他のトークンは入力のまま
//
// ★3 つ目を「小文字のまま」ではなく「入力のまま」と書いてある。実装は小文字化を
// 行わない——move_code は全て小文字であり(DES-004 §2.1)、結果は同じだが、
// 「小文字化する」と読むと実装と食い違う。
//
// ★意訳しない。move_code に無い語を足した時点で、それは新しい情報である(D-289)。
// 既にある識別子の区切りを変えるだけなので「英語の公式技名を持たない」に触れない。
//
// ★od の判定はトークン完全一致で行う。部分一致にすると condor / product のような
// 語中の od まで大文字化される。語中に od トークンを持つ実例が
// windclad_od_condor_dive_follow_up にあり、これは末尾の _od と同じ規則で OD になる。
func humanizeMoveCode(moveCode string) string {
	tokens := strings.Split(moveCode, "_")
	for i, t := range tokens {
		if t == "od" {
			tokens[i] = "OD"
		}
	}
	return strings.Join(tokens, " ")
}

// ---- 層 B: move_code の構造引き -------------------------------------------

// layerBPrefixes は層 B が受け持つ接頭辞。
// ★jumping_(空中攻撃)と jump_(移動 system move)を取り違えないこと
// (DES-004 §2.1 の 2026-07-23 errata)。取り違えると走査が移動 move を巻き込む。
var layerBPrefixes = []string{"standing_", "crouching_", "jumping_"}

// strengthAbbrev は強度の略記(DES-004 §3.4)。
var strengthAbbrev = map[string]string{"light": "L", "medium": "M", "heavy": "H"}

// buttonAbbrev はボタンの略記(同)。
var buttonAbbrev = map[string]string{"punch": "P", "kick": "K"}

// layerB は move_code の構造から表記を作る。不成立なら ok=false。
//
// ★「接頭辞 + 強度 + ボタン」の完全形にのみ当てる。接頭辞だけで判定しない——
// 実データには terry/jumping_knee・jumping_lariat(target_combo)や
// zangief/standing_light_punch_rapid のように、接頭辞を持つが強度・ボタン語彙に
// 当たらない行が実在する。接頭辞だけで判定すると、それらに誤った表記が付く。
func layerB(rule AliasPresetRule, moveCode string) (string, bool) {
	for _, p := range layerBPrefixes {
		rest, found := strings.CutPrefix(moveCode, p)
		if !found {
			continue
		}
		parts := strings.Split(rest, "_")
		if len(parts) != 2 {
			return "", false // 例 jumping_knee / standing_light_punch_rapid
		}
		s, okS := strengthAbbrev[parts[0]]
		b, okB := buttonAbbrev[parts[1]]
		if !okS || !okB {
			return "", false
		}
		var prefix string
		switch p {
		case "standing_":
			prefix = rule.StandingPrefix
		case "crouching_":
			prefix = rule.CrouchingPrefix
		case "jumping_":
			prefix = rule.JumpingPrefix
		}
		return prefix + s + b, true
	}
	return "", false
}

// ---- 層 A: command の正規化キー + SA 注記 -----------------------------------

// ★SA / CA 番号の抽出規則は internal/sanumber が持つ(M22-07b で移設)。
//
// 移設した理由は、消費者が 2 つになったためである——本ファイル(層 A の注記)と、
// 逆引きの第 3 段(internal/aliasindex)。★規則を 2 つ持つと、片方だけ直したときに
// 静かにずれる(指示書 M22-07b §4.2)。挙動は移設前と同一であり、本ファイルの
// golden テストが生成 SQL の byte 一致でそれを主張している。

// saAnnotation は super_art / critical_art に付ける注記を返す(例 " (SA1)")。
// 対象外・抽出不能なら空文字。
//
// ★書式は「半角スペース 1 個 + 半角丸括弧」(D-5)。DES-004 §3.4 の srk 欄
// 「236236P (SA1)」に合わせている。
//
// ★適用範囲は super_art / critical_art 一律である(衝突組だけに付けない)。理由:
//
//	(1) 衝突組だけに付けると同じ SA でもキャラによって注記の有無が変わり表記が不揃いになる。
//	(2) 衝突の有無を見てから注記すると「衝突判定 → 注記 → 再判定」のループになり、
//	    生成器の決定論が崩れる。
//	(3) 残存衝突キー数は一律でも衝突組限定でも同じである(注記は衝突組にしか効かない)。
//	    ⇒ 差分は表記の一貫性だけであり、一貫性を採る。
func saAnnotation(category, moveCode string) string {
	if category != "super_art" && category != "critical_art" {
		return ""
	}
	num, ok := sanumber.Extract(moveCode)
	if !ok {
		return ""
	}
	return " (" + num + ")"
}

// ---- 生成本体 --------------------------------------------------------------

// GenerateAliases は CSV 行から 1 プリセット分の preset_aliases seed マイグレを生成する。
//
// charOrder は投入するキャラ code の順序、rowsByChar は各 code の CSV 行。
// header は方向("up"/"down")を受けて生成 SQL 冒頭のコメントヘッダを返す。
//
// movementChars は移動系 9 code を投入するキャラ code の集合である。
//
// ★movementChars を charOrder と分けている理由(M20-02 レビュー H-2)。
// 移動系 9 code は CSV に 1 行も存在せず(31 CSV すべてで 0 件。seed マイグレが投入する)、
// 投入先は「その波で characters 行が新しく増えたキャラ」である。⇒ charOrder(攻撃技を入れる
// キャラ)とは一致しない。
//
// ★★M14-03f(第四波・2026-09-02)がその実例である——-chars は 14 キャラだが
// -movement-chars は 12 キャラであった。c_viper / dhalsim は移動 9 code と
// そのエイリアスを旧 000025 / 000072 / 000073 で既に持っているため、混ぜると本波の down が
// 前の波の投入分まで消す。
//
// ★旧文面は「CSV を持たないキャラ(c_viper / dhalsim)にも投入先がある」と理由を書いていたが、
// M14-03f で両者の CSV を投入したため失効した。分離の必要は残るが、理由が違う。
//
// ★★up と down を必ず同じ集合でスコープすること。
// 旧実装は「characters を CROSS JOIN して全キャラへ当てる」形にしていたが、down 側に
// スコープが無く、次の seed 波で生成したマイグレを down すると
// 「前の波が入れた全キャラ分」まで消えた(SUPP-001 §5.5.2 (5)「down は up の反転ではない」の
// 一形)。up が NOT EXISTS で skip する一方 down は無条件に消すため、非対称が事故になる。
// ⇒ 明示的なキャラ集合で両側を絞る。先例(旧 000025)の down も同じく明示的に絞っている。
//
// ★正規化は再実装しない(「二度作らない」)。internal/moveindex に通して token_key を得る。
//
// ★opts は省略可(既定 = FormatCurrent。詳細は format.go)。
func GenerateAliases(rule AliasPresetRule, charOrder []string, rowsByChar map[string][]MoveRow,
	movementChars []string, header func(dir string) string, opts ...Option) (*AliasResult, error) {
	if err := validate(charOrder, rowsByChar); err != nil {
		return nil, err
	}
	opt := resolveOptions(opts)

	res := &AliasResult{LayerStats: map[aliasLayer]int{}}

	// 移動系 9 code は CSV に無い(seedgen が drop し、seed マイグレが投入している)。
	// ⇒ movementChars で明示的に絞る。★up と down を同じ集合にするためであり、
	//   「CSV を持たないキャラにも当てる」ためではない(旧文面。M14-03f で失効)。
	for _, code := range movementCodeOrder {
		ma, ok := rule.Movement[code]
		if !ok {
			return nil, fmt.Errorf("preset %q: movement code %q の表記が未定義", rule.PresetCode, code)
		}
		res.Movement = append(res.Movement, AliasRow{
			MoveCode: code, AliasText: ma.text, AliasTextEn: ma.textEn, Layer: LayerCMovement,
		})
	}

	var up, down strings.Builder
	up.WriteString(header("up"))
	down.WriteString(header("down"))

	var downBlocks []string
	// ★-noinput-only では移動系を出さない。既に旧 000072 / 000073 が投入済みであり、
	//   出すと down が前の投入分まで消す(H-2 と同型)。
	if !opt.noInputOnly {
		if len(movementChars) == 0 {
			return nil, fmt.Errorf("preset %q: movementChars が空(移動系 9 code の投入先が決まらない)", rule.PresetCode)
		}
		writeMovementAliasInsert(&up, rule, movementChars, res.Movement, opt.format)
		downBlocks = append(downBlocks, buildMovementAliasDown(rule, movementChars, res.Movement))
	}

	for _, charCode := range charOrder {
		rows := rowsByChar[charCode]
		resolved, unfilled, discarded, saNoAnn, collisions := resolveCharacter(rule, charCode, rows, opt.format)
		res.Unfilled = append(res.Unfilled, unfilled...)
		res.Collisions = append(res.Collisions, collisions...)
		res.DiscardedLayerA = append(res.DiscardedLayerA, discarded...)
		res.SAWithoutAnnotation = append(res.SAWithoutAnnotation, saNoAnn...)
		for _, r := range resolved {
			res.LayerStats[r.Layer]++
		}
		res.Rows = append(res.Rows, resolved...)

		// ★出力だけを絞る。解決そのものは全層で回す——層 C-3 だけを解決すると、
		//   衝突判定の母数から他層の表記が落ち、UNIQUE(preset_id, character_id,
		//   alias_text) に触れる行を「衝突なし」と誤判定する。
		emit := resolved
		if opt.noInputOnly {
			emit = filterLayer(resolved, LayerCNoInput)
		}
		res.Emitted = append(res.Emitted, emit...)
		if len(emit) == 0 {
			continue
		}
		writeCharAliasInsert(&up, rule, charCode, emit, opt.format)
		downBlocks = append(downBlocks, buildCharAliasDown(rule, charCode, emit))
	}

	// down はキャラ逆順で結合(投入と逆順。buildCharDown と同じ運び)。
	for i := len(downBlocks) - 1; i >= 0; i-- {
		down.WriteString(downBlocks[i])
	}

	res.UpSQL = up.String()
	res.DownSQL = down.String()
	return res, nil
}

// resolveCharacter は 1 キャラ分の表記を解決する。
//
// 手順:
//  1. 層 B / 層 A で基底の表記を作る(rush_variant は対象外。派生技は旧規則でだけ対象外)。
//  2. 移動系 9 code を衝突判定の母数へ入れる(投入は別経路だが、同一キャラ内で
//     ぶつかれば逆引きが壊れるため判定には含める)。
//  3. 同一キャラ内で表記が衝突した組を解く。FormatCurrent では優先の 1 つだけ残し
//     (keepPriorCollisions)、旧規則では全部落とす(dropCollisions)。
//  4. 生き残った基底の表記から rush_variant を合成する。
//  5. rush 同士の衝突も同じ関数で解く。
//
// format は規則の版を決める(format.go)。層 C-3(P-34 の 13 件)は旧形式
// FormatPreM2003 では当てない(理由は includesNoInputDerived)。
func resolveCharacter(rule AliasPresetRule, charCode string, rows []MoveRow, format SQLFormat) (
	[]AliasRow, []UnfilledRow, []DiscardedLayerAValue, []UnfilledRow, []AliasCollision) {
	return resolveCharacterWith(rule, charCode, rows, format, nil)
}

// collisionResolver は同一キャラ内の表記の衝突を解く関数(dropCollisions / keepPriorCollisions の形)。
type collisionResolver func(map[string]AliasRow, map[string]string) (map[string]AliasRow, []collision, []AliasCollision)

// resolveCharacterWith は resolveCharacter の本体である。resolve が nil なら format が決める
// 本家の規則で衝突を解き、nil でなければそれを使う。
//
// ★単体版での変更(docs/upstream-diff.md): 本家の resolveCharacter をこの関数へ切り出し、
// 衝突の解き方だけを差し替えられるようにした。単体版は逆引きしないため被りを落とさない(HANDOVER §2.6)。
func resolveCharacterWith(rule AliasPresetRule, charCode string, rows []MoveRow, format SQLFormat, resolve collisionResolver) (
	[]AliasRow, []UnfilledRow, []DiscardedLayerAValue, []UnfilledRow, []AliasCollision) {

	noInput := format.includesNoInputDerived()
	priority := format.resolvesDerivedWithPriority()
	var unfilled []UnfilledRow
	var discarded []DiscardedLayerAValue
	var saNoAnnotation []UnfilledRow

	// 正規化キーを得るための索引。★派生行も正規化できるよう isDerived=false を渡す
	// (層 A の対象にするかどうかの判定は下で別に行う。ここでは「捨てた層 A の値」を
	//  報告するために全行の正規化結果が要る = D-2 (b))。
	ix := moveindex.New()
	for i, r := range rows {
		if r.isMovementSystem() || strings.TrimSpace(r.Command) == "" {
			continue
		}
		ix.Add(r.CharacterCode, r.MoveCode, r.Command, false, i)
	}
	tokenOf := map[string]string{}
	for _, e := range ix.Entries() {
		tokenOf[e.MoveCode] = e.TokenKey
	}
	indexSkipped := map[string]bool{}
	for _, s := range ix.Skipped() {
		indexSkipped[s.MoveCode] = true
	}

	base := map[string]AliasRow{} // move_code -> 表記
	byMove := map[string]MoveRow{}
	var rushRows []MoveRow

	for _, r := range rows {
		byMove[r.MoveCode] = r
		if r.isMovementSystem() {
			continue // 移動系は別経路(そもそも CSV から drop される)
		}
		if r.Category == "rush_variant" {
			rushRows = append(rushRows, r)
			continue
		}

		// ── 層 B(move_code の構造が表記を決める行) ───────────────────────
		if text, ok := layerB(rule, r.MoveCode); ok {
			base[r.MoveCode] = AliasRow{CharCode: charCode, MoveCode: r.MoveCode, AliasText: text, Layer: LayerB}
			// ★捨てた層 A の値を記録する(D-2 (b)「黙って捨てない」)。
			if a, has := tokenOf[r.MoveCode]; has && a != text {
				discarded = append(discarded, DiscardedLayerAValue{
					CharCode: charCode, MoveCode: r.MoveCode,
					LayerAKey: a, LayerBKey: text, LostInfo: lostInfo(a, text),
				})
			}
			continue
		}

		// ── 層 A(command が表記を決める行) ──────────────────────────────
		// ★旧規則では is_derived=true は対象外(U-2)。M48-08(D-1037)で U-2 を撤回し、
		//   FormatCurrent では command を持つ派生技も層 A の対象にする。
		//   ★command が空の派生技(層 C-3 ・derived-no-command)の扱いは変えていない。
		if r.IsDerived && (!priority || strings.TrimSpace(r.Command) == "") {
			// ★command が無い派生行(D-315 の 28 件)は別立てで記録する。
			//   規則をどう広げても埋まらない類であり、値は設計卓が決めた(P-34 → D-367)。
			if strings.TrimSpace(r.Command) == "" {
				// ── 層 C-3: 値が決まった 13 件だけを投入する(M20-06) ─────────
				if noInput && isNoInputDerivedTarget(charCode, r.MoveCode) {
					base[r.MoveCode] = AliasRow{
						CharCode: charCode, MoveCode: r.MoveCode,
						AliasText:   r.NameJA,
						AliasTextEn: humanizeMoveCode(r.MoveCode),
						Layer:       LayerCNoInput,
					}
					continue
				}
				unfilled = append(unfilled, mkUnfilled(r, ReasonDerivedNoCommand, ""))
			} else {
				unfilled = append(unfilled, mkUnfilled(r, ReasonDerived, ""))
			}
			continue
		}
		if strings.TrimSpace(r.Command) == "" {
			unfilled = append(unfilled, mkUnfilled(r, ReasonNoCommand, ""))
			continue
		}
		if indexSkipped[r.MoveCode] {
			unfilled = append(unfilled, mkUnfilled(r, ReasonIndexSkip, r.Command))
			continue
		}
		text, ok := tokenOf[r.MoveCode]
		if !ok {
			unfilled = append(unfilled, mkUnfilled(r, ReasonIndexSkip, r.Command))
			continue
		}
		ann := saAnnotation(r.Category, r.MoveCode)
		if ann == "" && (r.Category == "super_art" || r.Category == "critical_art") {
			// ★注記が付かなかった SA / CA を記録する(M20-05)。投入は止めない——
			//   表記自体は正しく、止めると穴が増えるだけである。報告で気づけるようにする。
			saNoAnnotation = append(saNoAnnotation, mkUnfilled(r, "sa-no-annotation",
				"move_code から SA/CA 番号を抽出できない"))
		}
		base[r.MoveCode] = AliasRow{
			CharCode: charCode, MoveCode: r.MoveCode,
			AliasText: text + ann, Layer: LayerA,
		}
	}

	// ── 衝突: 移動系 9 code も母数に入れて判定する ──────────────────────
	reserved := map[string]string{} // 表記 -> 出所(報告用)
	for _, code := range movementCodeOrder {
		reserved[rule.Movement[code].text] = code
	}
	if priority {
		// ★M48-08(Plan 未決 5 の開発者決定): 他のプリセットの移動系の表記も予約する。
		//   逆引きの辞書は全プリセット横断である(DES-004 §5.5 の注記)。予約しないと、
		//   srk の派生技(alex/low_retreat 等)が numeric の移動系の表記 `4` を取り、
		//   取込で `4` が「歩き」と「その技」の 2 候補に割れる。
		for _, other := range []AliasPresetRule{NumericRule, SRKRule} {
			for _, code := range movementCodeOrder {
				if _, taken := reserved[other.Movement[code].text]; !taken {
					reserved[other.Movement[code].text] = other.PresetCode + ":" + code
				}
			}
		}
	}
	var resolveCollisions collisionResolver = dropCollisions
	if priority {
		resolveCollisions = keepPriorCollisions(byMove)
	}
	if resolve != nil {
		resolveCollisions = resolve
	}
	var collisions []AliasCollision
	base, collided, kept := resolveCollisions(base, reserved)
	collisions = append(collisions, kept...)
	for _, c := range collided {
		unfilled = append(unfilled, mkUnfilled(byMove[c.moveCode], ReasonCollision, c.detail))
	}

	// ── 層 C-2: rush_variant を合成する ────────────────────────────────
	rushBase := map[string]AliasRow{}
	for _, r := range rushRows {
		orig := strings.TrimSpace(r.OriginalMoveCode)
		if orig == "" {
			unfilled = append(unfilled, mkUnfilled(r, ReasonRushNoOriginal, "original_move_code が空"))
			continue
		}
		// ★元技が CSV に実在するかを確かめる。実在しない場合は当てない(D-305)。
		//
		// ★★【2026-09-11 更新 = M35-03】実データの食い違いは 0 件になった。
		// かつて original_move_code が glowing_touch_1 のように実体(glowing_touch_1hits)と
		// 食い違う入力ミスが 4 件あり(ingrid 2 / lily 1 / mai 1)、DB 側では
		// original_move_id が NULL になっていた。⇒ M35-03 が CSV 正本を是正し、
		// golden 000026 / 000072 / 000073 を再生成して解消した(実測 0 件)。
		//
		// ★★★それでも本ガードは残す。⇒ D-305 の「推測で元技を当てない」は 1 文字も
		// 変わっていない。変わったのは「推測しなくても解決できるようになった」ことだけである。
		// ★同型の入力ミスは CSV の手入力でいつでも再発しうる。再発を検出する床は 2 本:
		//   - internal/seedgen/generate_m2002_golden_test.go の rushNoOriginal: 0
		//   - internal/infra/migration/migrate_m2002_test.go の
		//     「v73 時点で original_move_id が NULL の rush_variant が 0 行」(母数付き)
		// ⇒ 本ガードを外すと、その 2 本は緑のまま「推測で当てた別名」が投入される。
		if _, exists := byMove[orig]; !exists {
			unfilled = append(unfilled, mkUnfilled(r, ReasonRushNoOriginal,
				fmt.Sprintf("original_move_code=%q が実在しない", orig)))
			continue
		}
		ob, ok := base[orig]
		if !ok {
			unfilled = append(unfilled, mkUnfilled(r, ReasonRushOriginalUnfilled,
				fmt.Sprintf("元技 %s の表記が無い", orig)))
			continue
		}
		rushBase[r.MoveCode] = AliasRow{
			CharCode: charCode, MoveCode: r.MoveCode,
			AliasText: rushPrefix + rushJoiner + ob.AliasText, Layer: LayerCRush,
		}
	}
	// rush の表記が基底・移動系とぶつかっていないかも見る
	// (reserved は移動系 9 code で初期化済み。そこへ生き残った基底の表記を足す)。
	for code, row := range base {
		reserved[row.AliasText] = code
	}
	rushBase, rushCollided, rushKept := resolveCollisions(rushBase, reserved)
	collisions = append(collisions, rushKept...)
	for _, c := range rushCollided {
		unfilled = append(unfilled, mkUnfilled(byMove[c.moveCode], ReasonCollision, c.detail))
	}

	out := make([]AliasRow, 0, len(base)+len(rushBase))
	for _, m := range []map[string]AliasRow{base, rushBase} {
		for _, row := range m {
			out = append(out, row)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].MoveCode < out[j].MoveCode })
	sort.Slice(unfilled, func(i, j int) bool {
		if unfilled[i].MoveCode != unfilled[j].MoveCode {
			return unfilled[i].MoveCode < unfilled[j].MoveCode
		}
		return unfilled[i].Reason < unfilled[j].Reason
	})
	sort.Slice(collisions, func(i, j int) bool { return collisions[i].AliasText < collisions[j].AliasText })
	return out, unfilled, discarded, saNoAnnotation, collisions
}

const (
	// rushPrefix / rushJoiner は「DR + 連結子 + 元技の表記」の合成形(D-311)。
	// ★DES-004 §4.2 は numeric / srk とも連結子を '>' と定めている。表示はそれでよいが、
	//   1 ステップの表示文字列の中に '>' が入りうる点は逆引き側の論点になる(M20-07)。
	rushPrefix = "DR"
	rushJoiner = " > "
)

type collision struct {
	moveCode string
	detail   string
}

// dropCollisions は同一キャラ内で表記が衝突した組を全部落とす。
//
// ★★【2026-09-27 = M48-08 / CHANGE-256】これは旧規則(FormatPreM4808 以前)である。
// 開発者決定 D-1037 で「被ったら優先の 1 つだけ残す」へ改めた(keepPriorCollisions)。
// 本関数は凍結 golden と差分マイグレの引き算の基準を再現するために残してある。
// 下の段落は当時の理由である。
//
// ★製造は独断で片方を捨てない(§9.3-5)。衝突した組は両方(3 件以上なら全部)落とし、
// 全件を報告する。規則を調整して衝突を解くか、その行だけ外すかは設計卓が決める。
// 投入してしまうと逆引きが「複数件 → 未解決」に倒れ、G-11 / G-14b の決定論が働かなくなる。
//
// ★★ただし reserved 側は落ちない——ここだけ「片方を捨てる」形になっている
// (M20-02 レビュー M-2 で明示化)。
//
//	rows     … 判定対象(落ちうる)
//	reserved … 既に確定した表記(落ちない特権を持つ)
//
// reserved に入るのは 2 つ。
//  1. 移動系 9 code の固定表記(§4.6)——記法自身の語彙であり、技の側に譲らせる。
//  2. rush の合成時に生き残った基底の表記——rush は元技の表記から合成されるため、
//     元技より後に決まる。基底を落として rush を残すと合成元が消える。
//
// ⇒ この 2 つは「先に確定しているものが勝つ」という順序規則であり、
// §9.3-5 が禁じる「衝突した 2 技のどちらを残すかを製造が選ぶ」ではない。
// 実データでは reserved との衝突は 0 件である(移動系の値 6 / 4 / 66 / 44 / 8 / 9 / 7 /
// 微歩き / 微下がり と一致する技の表記が無い)。発生したら報告に現れる。
func dropCollisions(rows map[string]AliasRow, reserved map[string]string) (map[string]AliasRow, []collision, []AliasCollision) {
	byText := map[string][]string{}
	for code, r := range rows {
		byText[r.AliasText] = append(byText[r.AliasText], code)
	}
	var out []collision
	kept := map[string]AliasRow{}
	for code, r := range rows {
		peers := byText[r.AliasText]
		if len(peers) > 1 {
			sorted := append([]string(nil), peers...)
			sort.Strings(sorted)
			out = append(out, collision{moveCode: code,
				detail: fmt.Sprintf("表記 %q が %s と衝突", r.AliasText, strings.Join(sorted, ", "))})
			continue
		}
		if src, taken := reserved[r.AliasText]; taken {
			out = append(out, collision{moveCode: code,
				detail: fmt.Sprintf("表記 %q が %s と衝突", r.AliasText, src)})
			continue
		}
		kept[code] = r
	}
	sort.Slice(out, func(i, j int) bool { return out[i].moveCode < out[j].moveCode })
	return kept, out, nil
}

// keepPriorCollisions は同一キャラ内で表記が衝突した組のうち、優先の 1 つだけを残す
// 関数を返す(M48-08・CHANGE-256・開発者決定 D-1037 と Plan §9.4-1)。
//
// 優先の順:
//  1. 派生でない技 → 派生技(★派生技で層 A が作った行だけを後ろへ回す)
//  2. 同じ順位の中は CSV の行の順(RowIndex)。★キャラ内では moves.id の小さい順と同じ
//     である(000004 が CSV の行順に id を振る。31 キャラ全数で逆転 0＝M48-08 §3.3 の実査)。
//
// ★1 を置いた理由(開発者決定)——派生技が元技より先に登録されている組が 2 つあり
// (alex `6MP`・ken `214K`)、id だけで決めると今出ている元技の表記が派生技へ移る。
// ⇒ 1 を置くと「今ある行は 1 行も変わらない」が成り立ち、差分マイグレは追加だけになる。
//
// ★reserved(移動系 9 code・rush の合成時の基底)と被った行は、今までどおり reserved が
// 残る(dropCollisions の注記と同じ順序規則)。
//
// ★落とした側は理由 collision で記録し、Detail に残った側を書く。残った組は
// AliasCollision として返す(取込の逆引きが候補として扱うため。reserved と被った組は返さない)。
func keepPriorCollisions(byMove map[string]MoveRow) func(map[string]AliasRow, map[string]string) (
	map[string]AliasRow, []collision, []AliasCollision) {
	rank := func(r AliasRow) (int, int) {
		m := byMove[r.MoveCode]
		tier := 0
		if m.IsDerived && r.Layer == LayerA {
			tier = 1
		}
		return tier, m.RowIndex
	}
	before := func(a, b AliasRow) bool {
		ta, ia := rank(a)
		tb, ib := rank(b)
		if ta != tb {
			return ta < tb
		}
		return ia < ib
	}
	return func(rows map[string]AliasRow, reserved map[string]string) (
		map[string]AliasRow, []collision, []AliasCollision) {
		byText := map[string][]string{}
		for code, r := range rows {
			byText[r.AliasText] = append(byText[r.AliasText], code)
		}
		var out []collision
		var groups []AliasCollision
		kept := map[string]AliasRow{}
		for text, codes := range byText {
			sort.Strings(codes)
			if src, taken := reserved[text]; taken {
				for _, code := range codes {
					out = append(out, collision{moveCode: code,
						detail: fmt.Sprintf("表記 %q が %s と衝突", text, src)})
				}
				continue
			}
			best := codes[0]
			for _, code := range codes[1:] {
				if before(rows[code], rows[best]) {
					best = code
				}
			}
			kept[best] = rows[best]
			if len(codes) == 1 {
				continue
			}
			g := AliasCollision{CharCode: rows[best].CharCode, AliasText: text, Kept: best}
			for _, code := range codes {
				if code == best {
					continue
				}
				g.Dropped = append(g.Dropped, code)
				out = append(out, collision{moveCode: code,
					detail: fmt.Sprintf("表記 %q が %s と衝突(残る側: %s)", text, strings.Join(codes, ", "), best)})
			}
			groups = append(groups, g)
		}
		sort.Slice(out, func(i, j int) bool { return out[i].moveCode < out[j].moveCode })
		return kept, out, groups
	}
}

// filterLayer は指定した層の行だけを返す(順序は保つ)。
func filterLayer(rows []AliasRow, layer aliasLayer) []AliasRow {
	var out []AliasRow
	for _, r := range rows {
		if r.Layer == layer {
			out = append(out, r)
		}
	}
	return out
}

// lostInfo は層 A にあって層 B に無い情報を返す(報告用)。空なら損失なし。
func lostInfo(layerA, layerB string) string {
	if strings.Contains(layerA, "(hold)") && !strings.Contains(layerB, "(hold)") {
		return "(hold)"
	}
	return ""
}

func mkUnfilled(r MoveRow, reason unfilledReason, detail string) UnfilledRow {
	return UnfilledRow{
		CharCode: r.CharacterCode, MoveCode: r.MoveCode,
		Category: r.Category, NameJA: r.NameJA, Reason: reason, Detail: detail,
	}
}

// ---- SQL 出力 --------------------------------------------------------------

// presetIDSubquery は投入先プリセットの id を code から引く副問い合わせ。
// ★id をリテラルで埋め込まない。code が canonical な識別子であり(DES-004 §3.1)、
// id は投入順の産物にすぎない(旧 000069 の up が同じ理由で code を条件にしている)。
func presetIDSubquery(rule AliasPresetRule) string {
	return "(SELECT id FROM presets WHERE code = " + sqlStr(rule.PresetCode) + ")"
}

// writeMovementAliasInsert は移動系 9 code を movementChars のキャラへ投入する 1 文を書く。
//
// ★キャラを明示的に列挙する(M20-02 レビュー H-2 の是正)。down と同じ集合で絞るためである。
// 旧実装は characters を CROSS JOIN して全キャラへ当てていたが、down 側を同じ形で
// 絞れず(「このマイグレが入れた行」を down 時点で判別できない)、次の seed 波の down が
// 前の波の分まで消す事故になった。⇒ 両側を同じ明示リストで絞る。
//
// ★移動系は CSV に 1 行も無いため charOrder では表せない。投入先は「その波で characters 行が
// 新しく増えたキャラ」であり、攻撃技を入れるキャラ集合とは一致しない。⇒ 専用の集合を受け取る。
//
// ★character_id を出すのは M20-03 以降の必須事項である(同 §4.5)。出さないと NULL 行が
// 入り UNIQUE(preset_id, character_id, alias_text) をすり抜ける——落ちないため気づけない。
// format が旧形式なのは golden テストのときだけである(format.go の FormatPreM2003)。
func writeMovementAliasInsert(b *strings.Builder, rule AliasPresetRule, movementChars []string,
	rows []AliasRow, format SQLFormat) {
	fmt.Fprintf(b, "-- ===== 層 C-1: 移動系 %d code x %d キャラ =====\n", len(rows), len(movementChars))
	if format.includesCharacterID() {
		b.WriteString("INSERT INTO preset_aliases (preset_id, move_id, character_id, alias_text, alias_text_en)\n")
		fmt.Fprintf(b, "SELECT %s, m.id, m.character_id, v.alias_text, v.alias_text_en\n", presetIDSubquery(rule))
	} else {
		b.WriteString("INSERT INTO preset_aliases (preset_id, move_id, alias_text, alias_text_en)\n")
		fmt.Fprintf(b, "SELECT %s, m.id, v.alias_text, v.alias_text_en\n", presetIDSubquery(rule))
	}
	b.WriteString("FROM moves m\n")
	b.WriteString("JOIN characters c ON c.id = m.character_id\n")
	b.WriteString("JOIN games g ON g.id = c.game_id AND g.code = 'sf6'\n")
	b.WriteString("CROSS JOIN (\n")
	for i, r := range rows {
		prefix := "  UNION ALL SELECT "
		if i == 0 {
			prefix = "        SELECT "
		}
		b.WriteString(prefix)
		b.WriteString(sqlStr(r.MoveCode))
		if i == 0 {
			b.WriteString(" AS code")
		}
		b.WriteString(", ")
		b.WriteString(sqlStr(r.AliasText))
		if i == 0 {
			b.WriteString(" AS alias_text")
		}
		b.WriteString(", ")
		b.WriteString(nullableSQLStr(r.AliasTextEn))
		if i == 0 {
			b.WriteString(" AS alias_text_en")
		}
		b.WriteString("\n")
	}
	b.WriteString(") AS v\n")
	fmt.Fprintf(b, "WHERE %s\n", charInFilter("c", movementChars))
	b.WriteString("  AND m.code = v.code AND m.category = 'system'\n")
	// ★再適用時は skip する(upsert にしない = 既存行を書き換えない。§4.9)。
	//   UNIQUE (preset_id, move_id) があるため、ガードが無いと再適用で落ちる。
	fmt.Fprintf(b, "  AND NOT EXISTS (\n"+
		"      SELECT 1 FROM preset_aliases pa\n"+
		"      WHERE pa.preset_id = %s AND pa.move_id = m.id\n"+
		"  );\n\n", presetIDSubquery(rule))
}

// buildMovementAliasDown は移動系 9 code の down(投入 code・投入キャラのみの精密 DELETE)。
//
// ★up と同じ movementChars で絞ること(M20-02 レビュー H-2)。絞らないと、次の seed 波で
// 生成したマイグレの down が前の波の投入分まで消す。up は NOT EXISTS で skip するため
// 「このマイグレが実際に入れた行」は down 時点では判別できず、スコープでしか守れない。
// ★games.code='sf6' フィルタも up と揃える(同 L-1)。
func buildMovementAliasDown(rule AliasPresetRule, movementChars []string, rows []AliasRow) string {
	var b strings.Builder
	fmt.Fprintf(&b, "-- ===== 層 C-1: 移動系(%d キャラ) =====\n", len(movementChars))
	fmt.Fprintf(&b, "DELETE FROM preset_aliases WHERE preset_id = %s AND move_id IN (\n", presetIDSubquery(rule))
	b.WriteString("    SELECT m.id FROM moves m\n")
	b.WriteString("    JOIN characters c ON c.id = m.character_id\n")
	b.WriteString("    JOIN games g ON g.id = c.game_id AND g.code = 'sf6'\n")
	fmt.Fprintf(&b, "    WHERE %s AND m.category = 'system' AND m.code IN (", charInFilter("c", movementChars))
	for i, r := range rows {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(sqlStr(r.MoveCode))
	}
	b.WriteString("));\n\n")
	return b.String()
}

// writeCharAliasInsert は 1 キャラ分の INSERT 文を書く(writeAliasInsert と同型)。
//
// ★character_id については writeMovementAliasInsert の注記と同じ(M20-03 §4.5)。
//
// ★alias_text_en は「その塊に値を持つ行が 1 つでもあるとき」だけ列に出す(M20-06)。
// 層 C-3 が入るまで、キャラ別の塊は 1 行も英語表記を持たなかった(値を持つのは移動系の
// 2 code だけ = D-317)。無条件に出すと、既存の全キャラ分の塊に NULL だけの列が増える。
func writeCharAliasInsert(b *strings.Builder, rule AliasPresetRule, charCode string,
	rows []AliasRow, format SQLFormat) {
	withEn := false
	for _, r := range rows {
		if r.AliasTextEn != "" {
			withEn = true
			break
		}
	}
	fmt.Fprintf(b, "-- ===== %s (%d aliases) =====\n", charCode, len(rows))
	cols := "preset_id, move_id, alias_text"
	sel := "m.id, v.alias_text"
	if format.includesCharacterID() {
		cols = "preset_id, move_id, character_id, alias_text"
		sel = "m.id, m.character_id, v.alias_text"
	}
	if withEn {
		cols += ", alias_text_en"
		sel += ", v.alias_text_en"
	}
	fmt.Fprintf(b, "INSERT INTO preset_aliases (%s)\n", cols)
	fmt.Fprintf(b, "SELECT %s, %s\n", presetIDSubquery(rule), sel)
	b.WriteString("FROM moves m\nJOIN characters c ON c.id = m.character_id\n")
	b.WriteString("CROSS JOIN (\n")
	for i, r := range rows {
		prefix := "  UNION ALL SELECT "
		if i == 0 {
			prefix = "        SELECT "
		}
		b.WriteString(prefix)
		b.WriteString(sqlStr(r.MoveCode))
		if i == 0 {
			b.WriteString(" AS code")
		}
		b.WriteString(", ")
		b.WriteString(sqlStr(r.AliasText))
		if i == 0 {
			b.WriteString(" AS alias_text")
		}
		if withEn {
			b.WriteString(", ")
			b.WriteString(nullableSQLStr(r.AliasTextEn))
			if i == 0 {
				b.WriteString(" AS alias_text_en")
			}
		}
		b.WriteString("\n")
	}
	fmt.Fprintf(b, ") AS v\nWHERE %s AND m.code = v.code\n", charFilter("c", charCode))
	fmt.Fprintf(b, "  AND NOT EXISTS (\n"+
		"      SELECT 1 FROM preset_aliases pa\n"+
		"      WHERE pa.preset_id = %s AND pa.move_id = m.id\n"+
		"  );\n\n", presetIDSubquery(rule))
}

// buildCharAliasDown は 1 キャラ分の down(投入 code のみの精密 DELETE)。
func buildCharAliasDown(rule AliasPresetRule, charCode string, rows []AliasRow) string {
	var b strings.Builder
	fmt.Fprintf(&b, "-- ===== %s =====\n", charCode)
	fmt.Fprintf(&b, "DELETE FROM preset_aliases WHERE preset_id = %s AND move_id IN (\n", presetIDSubquery(rule))
	fmt.Fprintf(&b, "    SELECT m.id FROM moves m JOIN characters c ON c.id = m.character_id WHERE %s AND m.code IN (",
		charFilter("c", charCode))
	for i, r := range rows {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(sqlStr(r.MoveCode))
	}
	b.WriteString("));\n\n")
	return b.String()
}

// nullableSQLStr は空文字を NULL として書く(alias_text_en 用)。
// ★「英語表記を持たない」と「空文字である」を区別する(旧 000070 のコメント参照)。
func nullableSQLStr(s string) string {
	if s == "" {
		return "NULL"
	}
	return sqlStr(s)
}

// AliasHeader は preset_aliases seed マイグレ用の生成 SQL ヘッダを組み立てる
// (cmd/seedgen の -mode aliases と golden テストが共用する)。
//
// ★opts には GenerateAliases へ渡すものと同じ Option を渡すこと。ヘッダの文面が
// 生成物の中身と食い違わないようにするためである(下記の移動系の行が実例)。
func AliasHeader(stem, note string, opts ...Option) func(dir string) string {
	opt := resolveOptions(opts)
	return func(dir string) string {
		h := "-- " + stem + "." + dir + ".sql\n"
		if note != "" {
			h += "-- " + note + "\n"
		}
		h += "-- 本ファイルは cmd/seedgen が character_data/*.csv から生成した成果物(手編集しない)。\n" +
			"-- ★1 回きりの backfill ではなく「規則」の出力である(D-181)。新キャラ CSV を投入する\n" +
			"--   seed 波では、本規則を再適用して新しい連番のマイグレを起こすこと\n" +
			"--   (手順は character_data/seed-progress.md)。\n" +
			"-- 層順の原理: command が表記を決める行は層 A、move_code の構造が決める行は層 B。\n"
		h += movementHeaderLine(opt)
		h += "-- 再適用時は NOT EXISTS ガードで skip する(upsert にしない = 既存行を書き換えない)。\n" +
			"-- 既存マイグレは非改変(新規連番で追加)。FK 依存順 characters→moves→preset_aliases。\n\n"
		return h
	}
}

// movementHeaderLine は移動系 9 code についてのヘッダ 1 行を返す。
//
// ★旧文面「移動系 9 code は characters を CROSS JOIN するため、CSV を持たないキャラにも
// 当たる」は失効している。M20-02 レビュー H-2 が CROSS JOIN をキャラの明示列挙へ是正した
// (down を up と同じ集合で絞れず、次の波の down が前の波の投入分まで消したため)。
// ⇒ 生成物の本体は旧 000072 の時点で既にキャラを列挙しているのに、ヘッダだけが旧のまま
// 残り、以後のすべての波へ複製される状態だった(D-361 の (b) 前提を述べている散文)。
//
// ★旧文面は FormatPreM2003 でのみ再現する。凍結 golden 000072 / 000073 との byte 一致を
// 主張する golden テストが唯一の手編集ドリフト検出経路であり、そこを崩さないためである
// (適用済みマイグレは改変しない = M20-03 §2.2)。
func movementHeaderLine(opt options) string {
	if !opt.format.includesCharacterID() {
		return "-- 移動系 9 code は characters を CROSS JOIN するため、CSV を持たないキャラにも当たる。\n"
	}
	if opt.noInputOnly {
		// 層 C-3 だけを出す生成物には移動系のブロックが無い。
		return "-- ★本ファイルは層 C-3(何も押さずに派生する技)だけを投入する。移動系 9 code は含まない。\n"
	}
	return "-- 移動系 9 code は投入先キャラを明示列挙する(up と down を同じ集合で絞るため = H-2)。\n"
}

// RenderAliasReport は生成結果の内訳を Markdown で返す(完了報告の全件列挙用)。
func RenderAliasReport(rule AliasPresetRule, res *AliasResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# preset_aliases 生成レポート(%s)\n\n", rule.PresetCode)

	fmt.Fprintf(&b, "## 層別の投入行数\n\n| 層 | 行数 |\n|---|---|\n")
	fmt.Fprintf(&b, "| C-1 移動系(9 code x 全キャラ) | SQL 1 文(キャラ列挙なし) |\n")
	for _, l := range []aliasLayer{LayerB, LayerA, LayerCRush, LayerCNoInput} {
		fmt.Fprintf(&b, "| %s | %d |\n", l, res.LayerStats[l])
	}
	fmt.Fprintf(&b, "| **キャラ別 合計** | **%d** |\n", len(res.Rows))
	fmt.Fprintf(&b, "| **うち SQL へ出した行** | **%d** |\n\n", len(res.Emitted))

	byReason := map[unfilledReason][]UnfilledRow{}
	for _, u := range res.Unfilled {
		byReason[u.Reason] = append(byReason[u.Reason], u)
	}
	fmt.Fprintf(&b, "## 投入しない行 %d 件\n\n| 理由 | 件数 |\n|---|---|\n", len(res.Unfilled))
	reasons := []unfilledReason{
		ReasonDerived, ReasonDerivedNoCommand, ReasonNoCommand, ReasonIndexSkip,
		ReasonRushNoOriginal, ReasonRushOriginalUnfilled, ReasonCollision,
	}
	for _, r := range reasons {
		fmt.Fprintf(&b, "| %s | %d |\n", r, len(byReason[r]))
	}
	b.WriteString("\n")

	for _, r := range reasons {
		list := byReason[r]
		if len(list) == 0 {
			continue
		}
		fmt.Fprintf(&b, "### %s(%d 件)\n\n", r, len(list))
		b.WriteString("| character | move_code | category | name_ja | 補足 |\n|---|---|---|---|---|\n")
		sorted := append([]UnfilledRow(nil), list...)
		sort.Slice(sorted, func(i, j int) bool {
			if sorted[i].CharCode != sorted[j].CharCode {
				return sorted[i].CharCode < sorted[j].CharCode
			}
			return sorted[i].MoveCode < sorted[j].MoveCode
		})
		for _, u := range sorted {
			fmt.Fprintf(&b, "| %s | `%s` | %s | %s | %s |\n",
				u.CharCode, u.MoveCode, u.Category, u.NameJA, u.Detail)
		}
		b.WriteString("\n")
	}

	fmt.Fprintf(&b, "## ★SA 注記が付かなかった super_art / critical_art %d 件\n\n", len(res.SAWithoutAnnotation))
	if len(res.SAWithoutAnnotation) == 0 {
		b.WriteString("なし(全行で move_code から SA/CA 番号を機械抽出できた)。\n\n")
	} else {
		b.WriteString("| character | move_code | category | name_ja |\n|---|---|---|---|\n")
		sorted := append([]UnfilledRow(nil), res.SAWithoutAnnotation...)
		sort.Slice(sorted, func(i, j int) bool {
			if sorted[i].CharCode != sorted[j].CharCode {
				return sorted[i].CharCode < sorted[j].CharCode
			}
			return sorted[i].MoveCode < sorted[j].MoveCode
		})
		for _, u := range sorted {
			fmt.Fprintf(&b, "| %s | `%s` | %s | %s |\n", u.CharCode, u.MoveCode, u.Category, u.NameJA)
		}
		b.WriteString("\n")
	}

	fmt.Fprintf(&b, "## 層 B が優先されて捨てた層 A の値 %d 件(★黙って捨てない)\n\n", len(res.DiscardedLayerA))
	var lost []DiscardedLayerAValue
	for _, d := range res.DiscardedLayerA {
		if d.LostInfo != "" {
			lost = append(lost, d)
		}
	}
	fmt.Fprintf(&b, "うち**情報が減る行 = %d 件**(層 A にあって層 B に無い情報を持つ)。\n\n", len(lost))
	if len(lost) > 0 {
		b.WriteString("| character | move_code | 層 A なら | 層 B(採用) | 落ちた情報 |\n|---|---|---|---|---|\n")
		sort.Slice(lost, func(i, j int) bool {
			if lost[i].CharCode != lost[j].CharCode {
				return lost[i].CharCode < lost[j].CharCode
			}
			return lost[i].MoveCode < lost[j].MoveCode
		})
		for _, d := range lost {
			fmt.Fprintf(&b, "| %s | `%s` | `%s` | `%s` | `%s` |\n",
				d.CharCode, d.MoveCode, d.LayerAKey, d.LayerBKey, d.LostInfo)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// charInFilter は複数キャラ code の IN 条件を組み立てる(charFilter の複数版)。
// ★games の絞りは呼び出し側が JOIN で行う(charFilter は game_id の副問い合わせを使うが、
// 移動系は既に games を JOIN しているため二重にしない)。
func charInFilter(alias string, codes []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s.code IN (", alias)
	for i, c := range codes {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(sqlStr(c))
	}
	b.WriteString(")")
	return b.String()
}
