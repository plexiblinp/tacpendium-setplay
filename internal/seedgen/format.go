package seedgen

// 生成 SQL の形式切り替え(M20-03)。
//
// ★なぜ切り替えが要るのか
//   M20-03 が preset_aliases へ character_id を足したため、以後に生成する
//   INSERT は character_id を出さなければならない(出さないと NULL 行が入り、
//   UNIQUE 索引をすり抜ける。★落ちないため気づけない = 指示書 M20-03 §4.5)。
//   一方、既に適用済みのマイグレは改変できない(同 §2.2)。そして golden テストは
//   「生成器の出力がコミット済みマイグレと byte 一致すること」を主張しており、
//   これが手編集ドリフトの唯一の検出経路である。
//   ⇒ 旧形式を名前付きで残し、golden だけがそれを指定する。

// SQLFormat は preset_aliases への INSERT が character_id を含むかを表す。
type SQLFormat int

const (
	// FormatCurrent は character_id を含む現行形式(M20-03 以降)。既定値である。
	//
	// 新しい seed 波(character_data/seed-progress.md の手順)は必ずこちらを使う。
	// cmd/seedgen はオプションを渡さないため自動的に本形式になる。
	// ★M48-08 以降、表記の規則は「派生技も層 A ・被ったら優先の 1 つだけ残す」である
	// (旧規則は FormatPreM4808)。
	// ★M43-05 以降、ターゲットコンボは段を `~` でつなぎ末尾に `(TC)` を付け、空中から出す技には
	// `j.`(垂直ジャンプ `nj.` ／ ハイジャンプ `hj.`)を付ける(旧規則は FormatPreM4305)。
	FormatCurrent SQLFormat = iota

	// FormatPreM2003 は character_id を含まない旧形式。
	//
	// ★後方互換専用であり、対象は閉じた集合である——M20-03 より前に生成・適用済みの
	// 次の 6 ファイルだけを指す。今後この集合が増えることはない。
	//
	//	000026_seed_moves_first_wave
	//	000030_seed_moves_ryu
	//	000045_seed_moves_manon
	//	000055_seed_moves_third_wave
	//	000072_m20_seed_aliases_numeric
	//	000073_m20_seed_aliases_srk
	//
	// ★★M33-03: この 6 つは *凍結 golden の名前* である(internal/seedgen/testdata/)。
	// ⇒ M33-02 が旧 111 本を新系列 9 本へ潰したため、同名のマイグレは migrations/ に無い。
	// ★綴りは変えていない。由来を辿れるようにするためである。
	//
	// ★新規生成では使わないこと。使うと character_id が NULL の行が入り、
	// UNIQUE(preset_id, character_id, alias_text) をすり抜ける。
	// 指定してよいのは、上記 6 ファイルとの byte 一致を主張する golden テストだけである。
	FormatPreM2003

	// FormatPreM4808 は M48-08 より前の表記の規則(★SQL の形は FormatCurrent と同じ)。
	//
	// M48-08(CHANGE-256・開発者決定 D-1037)が 2 つの規則を変えた:
	//   (1) 派生技(is_derived=true かつ command あり)も層 A の対象にする(裁定 U-2 の撤回)
	//   (2) 同一キャラ内で表記が被ったら、全部落とすのではなく優先の 1 つだけ残す
	//       (優先 = 派生でない技 → 派生技、同順位は CSV の行順＝moves.id の小さい順)
	// ⇒ 規則を変えれば適用済みの生成物の再生成結果も変わる。FormatPreM2003 と同じく、
	//   旧規則を「当時の生成器の挙動」として名前付きで凍結する。
	//
	// ★対象は閉じた集合である:
	//
	//	000090_seed_aliases_numeric_fourth_wave / 000091_seed_aliases_srk_fourth_wave
	//	  … 凍結 golden(internal/seedgen/testdata/)との byte 一致
	//	migrations/000008 の numeric / srk の行 … M48-08 の差分マイグレ(000012)の引き算の基準
	//
	// ★新規生成では使わないこと。使うと派生技の表記が作られず、被った表記が両方とも落ちる。
	FormatPreM4808

	// FormatPreM4305 は M43-05 より前の表記の規則(＝M48-08 の規則。★SQL の形は FormatCurrent と同じ)。
	//
	// M43-05(CHANGE-257・開発者決定 D-1051 と Plan §0)が 2 つの規則を変えた:
	//   (1) ターゲットコンボは段ごとに通常技の表記を書き `~` でつなぎ、末尾に `(TC)` を付ける
	//       (numeric `5MP~5HP(TC)` ／ srk `st.MP~st.HP (TC)`。旧＝command の正規化キー `MP>HP`)
	//   (2) 空中から出す技に `j.` を付ける(垂直ジャンプ `nj.` ／ ハイジャンプ `hj.`)
	// ⇒ FormatPreM4808 と同じく、旧規則を「当時の生成器の挙動」として名前付きで凍結する。
	//
	// ★対象は閉じた集合である:
	//
	//	migrations/000012 の差分(FormatPreM4305 − FormatPreM4808)… 000012 を生成したときの規則
	//	  (★M43-07 以降、000012 は固定済みであり、今の CSV で作り直して照らさない＝internal/migrationpin)
	//	M43-05 の差分マイグレ(000014)の引き算の基準(＝000008 ＋ 000012 の numeric / srk の行)
	//
	// ★新規生成では使わないこと。使うとターゲットコンボが `>` の形になり、空中の技に `j.` が付かない。
	FormatPreM4305
)

// includesCharacterID は本形式が preset_aliases へ character_id を出すかを返す。
func (f SQLFormat) includesCharacterID() bool { return f != FormatPreM2003 }

// includesNoInputDerived は本形式が層 C-3(P-34 の 13 件・M20-06)を出すかを返す。
//
// ★FormatPreM2003 で false にする理由は character_id と同じである——上記 6 ファイルは
// 層 C-3 が存在しない時点で生成・適用済みであり、出すと golden テストが主張する
// byte 一致が崩れる。★「規則を足したら適用済みマイグレの再生成結果が変わる」のは
// 生成器を持つ以上つねに起きるため、旧形式を「当時の生成器の挙動」として凍結してある。
func (f SQLFormat) includesNoInputDerived() bool { return f != FormatPreM2003 }

// resolvesDerivedWithPriority は本形式が M48-08 の規則(派生技も層 A ・被ったら優先の
// 1 つだけ残す)で表記を解決するかを返す(FormatPreM4808 の注記)。
func (f SQLFormat) resolvesDerivedWithPriority() bool {
	return f == FormatCurrent || f == FormatPreM4305
}

// notatesTargetComboAndAerial は本形式が M43-05 の規則(ターゲットコンボの `~`/`(TC)` ・空中の技の
// `j.`)で表記を作るかを返す(FormatPreM4305 の注記)。
func (f SQLFormat) notatesTargetComboAndAerial() bool { return f == FormatCurrent }

// Option は生成の任意設定。可変長で受けるため、既存の呼び出しは無改変で
// 既定(FormatCurrent)になる。
type Option func(*options)

type options struct {
	format      SQLFormat
	noInputOnly bool
}

// WithNoInputOnly は層 C-3(P-34 の 13 件)だけを SQL へ出す(M20-06)。
//
// ★用途は「既に seed 済みのキャラへ層 C-3 だけを後から入れる」一度きりの投入である。
// 既存の -mode aliases をそのまま再実行すると、up は NOT EXISTS で skip する一方
// down は無条件 DELETE であるため、旧 000072 / 000073 が入れた行まで消す down ができる。
//
// ★解決そのものは全層で回る(衝突判定の母数を保つため)。絞るのは出力だけである。
func WithNoInputOnly() Option {
	return func(o *options) { o.noInputOnly = true }
}

// WithFormat は生成 SQL の形式を指定する。
//
// ★FormatPreM2003 を渡してよいのは golden テストだけである(理由は同定数の注記)。
func WithFormat(f SQLFormat) Option {
	return func(o *options) { o.format = f }
}

// resolveOptions は可変長オプションを畳む(未指定なら FormatCurrent)。
func resolveOptions(opts []Option) options {
	o := options{format: FormatCurrent}
	for _, apply := range opts {
		apply(&o)
	}
	return o
}
