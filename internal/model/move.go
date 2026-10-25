package model

// 単体版: 本家の internal/model/move.go のうち、セットプレイの計算が使う定数だけを移植した
// (技カテゴリ ・名指しの技 code ・startup_basis)。Move 構造体などは持たない。docs/upstream-diff.md。

// 技カテゴリの代表的な値(DES-003 §3.3、DES-004 §2.1)。
const (
	MoveCategoryNormal      = "normal"
	MoveCategorySpecial     = "special"
	MoveCategoryUnique      = "unique"
	MoveCategorySuperArt    = "super_art"
	MoveCategoryThrow       = "throw"
	MoveCategorySystem      = "system"
	MoveCategoryTargetCombo = "target_combo"
	MoveCategoryRushVariant = "rush_variant"
	MoveCategoryDriveImpact = "drive_impact"
	// MoveCategoryCriticalArt はクリティカルアーツ(CA)。SA3 とは別行で出現し、
	// code は `ca_` 接頭辞を持つ(DES-003 §3.3 L285 enum、CHANGE-025、M9-02 取込で活性化)。
	MoveCategoryCriticalArt = "critical_art"
)

// 技 code のうち、実装が名指しで参照するもの。
//
// ★リテラル文字列を実装各所に散在させない(CLAUDE.md §4)。フロントエンド側の対応定数は
// web/src/constants/move-code.ts に同期する(新規値の追加時は両側を同時に更新すること)。
const (
	// MoveCodeDriveReversal はドライブリバーサル(M31-04 / SM-098。投入元は 000004_data_seed_moves)。
	//
	// ★category は system であり、全キャラに 1 行ずつ在る(移動 system 技と同じ全キャラ行の形。
	//   投入元は 000004_data_seed_moves)。
	// ★確定反撃の走査では「相手技」として扱う。ただしジャストパリィタブからは除外する
	//   (開発者の逐語「この技をジャストパリィすることはないので取り扱わなくてもいい」。
	//    除外の実体は service/punishfinder の justParryExcludedCodes)。
	// ★入力面(仮想コントローラ)には出さない —— 防御リバーサルでありコンボ部品ではない
	//   (DES-002 §4)。FE 側の除外は web/src/features/combo/moveSurfacing.ts が持つ。
	// ★★ただし他から引っ越し(intake)は「入力面」に含めない(2026-09-10 開発者判断)。
	//   別名照合(internal/service/intake)は本 code を category で絞らず素通しする。
	//   ⇒ これは塞ぎ忘れではなく対象外と決めた結果である。勝手に塞がないこと。
	MoveCodeDriveReversal = "drive_reversal"

	// MoveCodeJumpForward は前ジャンプ(移動 system 技。全キャラに 1 行ずつ在る)。
	//
	// ★M47-04: 安全飛びの提案を採用するとき、画面が「繋ぎ ＞ 前ジャンプ ＞ 選んだ技」の
	//   前ジャンプとして名指しする(開発者の決定＝指示書 §9.4)。参照は FE 側
	//   (web/src/features/setplay/safeJump.ts)。
	// ★M47-05: BE の提案の絞り込み「接近を含む提案だけ」が、接近の手の 1 つとして名指しする
	//   (service/setplay の isApproachMove)。M47-04 の時点では BE は名指ししていなかった。
	MoveCodeJumpForward = "jump_forward"

	// MoveCodeDashForward は前ステップ(移動 system 技。全キャラに 1 行ずつ在る)。
	//
	// ★M47-05: 提案の絞り込み「接近を含む提案だけ」の接近の手の 1 つ(service/setplay の
	//   isApproachMove。開発者の回答＝前ステップ ／ 前ジャンプ ／ ラッシュから出す技＝D-1005)。
	//   後ろステップ(dash_back)は接近に数えない(D-1007)。
	MoveCodeDashForward = "dash_forward"

	// MoveCodeJumpNeutral は垂直ジャンプ(移動 system 技。全キャラに 1 行ずつ在る)。
	//
	// ★M43-13: 確定反撃サーチの jump レーンから新規登録するとき、垂直ジャンプの攻撃
	//   (code が neutral_jumping_ で始まる始動技)の前へ置く移動として、画面が名指しする
	//   (FE 側 web/src/features/combo/lockedLead.ts)。BE は名指ししない(FE と 1:1 で同期するために置く)。
	MoveCodeJumpNeutral = "jump_neutral"
)

// startup_basis の値域(DES-003 §3.3、M19-DESIGN-07 §3)。
// 「格納されている startup が何を起点に測った値か」を表す。NOT NULL・既存行は unknown 起点。
//
//   - standalone: 単独で出したときの発生。
//   - through:    親からの通し値(親の入力からの経過フレーム)。親計上ゼロの規約が効く。
//   - unknown:    由来が未確定。unknown を暗黙に単発扱いしない(R4 の三値)。
//
// 本定数は BE 内の消費側でのみ参照する。GET /api/moves には露出しないため、
// 対応するフロントエンド定数は作らない(CLAUDE.md §4 の同期規約は API 露出列が対象)。
const (
	MoveStartupBasisStandalone = "standalone"
	MoveStartupBasisThrough    = "through"
	MoveStartupBasisUnknown    = "unknown"
)
