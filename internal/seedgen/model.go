// Package seedgen は手入力 CSV(character_data/*.csv) を seed SQL マイグレへ変換する
// dev/build 時専用の変換インフラである(M14-03b)。本体ランタイムには一切露出しない
// (取込 FR704 の復活禁止・指示書 §4.1)。
//
// 変換規約(指示書 §4.1):
//   - 列 remap → moves 現行スキーマ。move_code は CSV を正とし再採番しない。
//   - total は CSV 値を式 startup+active-1+recovery で検算し、不一致は fail(黙って上書きしない)。
//   - recovery は整数前提。非整数は手入力ミスとして fail。
//   - raw_data は notes/notes_tool のみ(空→NULL)。command/condition_* は列が無く投入しない
//     (CSV 原本で保全)。is_projectile の列は 000001_init_schema が宣言するが seedgen は SQL 非投入のまま
//     ＝初期値は seed が持つ(is_derived と同型・M18-01 §4.4。旧系列では 000039 の backfill)。
//     startup_basis / chain_cancel_total / fastest_unreachable の列も 000001_init_schema が宣言するが
//     同じく SQL 非投入＝機械で決まる分は seed が持つ(M19-04 §4.2。旧系列では 000050 の backfill)。
//     first_hit_startup(列 000114・M37-04)も同型で 4 つ目の SQL 非投入列である。
//     ★★値は開発者の手入力(D-857)であり、CSV を埋めたあと DB へ入れる backfill マイグレが
//     別途要る(番号の請求を含む)。⇒ 埋めただけでは csv_db_sync_test が赤になる。
//   - 移動 system move 9 種(forward〜jump_back)は drop(投入元は §4.2 の静的 seed)。drive_parry は通過。
//   - target_combo・rush_variant 行は無改変で通過投入。
//   - dup(同一キャラ内 code 衝突・alias_text 衝突)検出時は SQL を生成せず fail・一覧報告。
package seedgen

import "strings"

// CSV 列順(25 列・character_data/*.csv の現行契約)。
//
// ★数え違いの履歴: 「23 列」のまま 000104(last_changed_game_version)で 24 列へ、
//
//	000114(first_hit_startup)で 25 列へ広がっていた。★触ったブロックの数え違いは
//	触った手番で直すこと —— 見出しは最初に読まれ、誰も再カウントしない。
var csvColumns = []string{
	"character_code", "move_code", "category", "name_ja",
	"startup", "active", "recovery", "total",
	"on_hit", "on_block", "damage",
	"is_aerial", "is_projectile", "is_derived",
	"notes", "notes_tool", "original_move_code",
	"command", "condition_ja", "condition_en",
	"startup_basis", "chain_cancel_total", "fastest_unreachable",
	// M28-02a: FR702 のマーカー(その技が最後に変わったゲームバージョン)。
	// ★★上の 3 列と違い「SQL へ投入する」。理由は §2.2-4 の罠そのものである ——
	//   投入しない列は seed 再生成で黙って消え、DB だけが進む。
	"last_changed_game_version",
	// M37-04(B06): その技の初段が当たるまでのフレーム(列は 000114 で追加)。
	// ★保全のみ・SQL 非投入。startup_basis 等 3 列と同じ流儀である。
	// ★★末尾へ足した。parseRow は位置引き(g(N))であり、途中へ入れると既存 24 列の
	//   インデックスが全部ずれる。⇒ 位置依存を増やさない。
	"first_hit_startup",
}

// validStartupBases は moves.startup_basis の enum(DES-003 §3.3・000001_init_schema)。
// 空欄は未記入(＝DDL の DEFAULT 'unknown' に委ねる)として許容する。
var validStartupBases = map[string]bool{
	"": true, "standalone": true, "through": true, "unknown": true,
}

// movementSystemCodes は移動 system move 9 種(DES-004 §2.1)。変換系では drop する。
var movementSystemCodes = map[string]bool{
	"forward": true, "back": true,
	"micro_forward": true, "micro_back": true,
	"dash_forward": true, "dash_back": true,
	"jump_neutral": true, "jump_forward": true, "jump_back": true,
}

// validCategories は moves.category の enum(DES-003 §3.3・000001 コメント)。
var validCategories = map[string]bool{
	"normal": true, "special": true, "unique": true,
	"super_art": true, "critical_art": true, "throw": true,
	"system": true, "target_combo": true,
	"rush_variant": true, "drive_impact": true,
}

// MoveRow は CSV 1 行を表す。*int は NULL(空欄)を表現する。
type MoveRow struct {
	CharacterCode    string
	MoveCode         string
	Category         string
	NameJA           string
	Startup          *int
	Active           *int
	Recovery         *int
	Total            *int
	OnHit            *int
	OnBlock          *int
	Damage           *int
	IsAerial         bool
	IsProjectile     bool // 保全のみ・SQL 非投入。列は 000001_init_schema・初期値は seed が持つ(M18-01 G-b)
	IsDerived        bool // 索引フィルタ用・SQL 非投入(恒久配置は M17-02 G-k)
	Notes            string
	NotesTool        string
	OriginalMoveCode string
	Command          string // 索引源・SQL 非投入
	ConditionJA      string // 保全のみ・SQL 非投入
	ConditionEN      string // 保全のみ・SQL 非投入
	// 以下 3 列は保全のみ・SQL 非投入。列は 000001_init_schema・機械で決まる分の初期値は
	// seed が持つ(is_projectile と同型・M19-04 §4.2)。人手判断分は Phase 2 で
	// 本 CSV から専用モードが生成する。
	StartupBasis       string // 保全のみ・SQL 非投入。値域は validStartupBases(空欄=未記入)
	ChainCancelTotal   *int   // 保全のみ・SQL 非投入。値の正本は character_data/chain-cancel-measurements.md
	FastestUnreachable bool   // 保全のみ・SQL 非投入
	// LastChangedGameVersion は FR702 のマーカー(M28-02a・列は 000104 で追加)。
	//
	// ★★上の「保全のみ」列と違い SQL へ投入する。⇒ writeGameVersionUpdate が
	//   INSERT の直後へ UPDATE を出力する。空欄(未記入)なら 1 バイトも出力しない。
	// ★★持ち場を CSV にした理由 —— マーカーを立てるのは配信者(開発者)であり
	//   (checklist §5.1)、開発者が実際に move データを保守している場所がここだからである。
	//   マイグレの DML だけで管理すると、次の seed 波が DELETE → INSERT で消す。
	// ★値域は `YYYY.MM.DD.NN`(internal/gameversion)。検証は validate 側で行う
	//   (parseRow は型エラーのみを見る分業。startup_basis と同じ)。
	LastChangedGameVersion string
	// FirstHitStartup はその技の初段が当たるまでのフレーム(M37-04・列は 000114 で追加)。
	//
	// ★保全のみ・SQL 非投入。⇒ 値の手入力は開発者の手番であり(D-857)、本列を読むのは
	//   確定反撃の候補判定である(internal/service/punishfinder)。
	// ★NULL(空欄)の意味は「不明」で統一する。「1 段技だから不要」という第 2 の意味を
	//   持たせてはならない —— 2 つの意味を 1 つの空欄に畳むと、後から区別できない。
	// ★CSV と DB の一致は internal/infra/migration/csv_db_sync_test.go が固定する
	//   (本列は seedgen が SQL へ出さないため golden が守ってくれない)。
	FirstHitStartup *int
	RowIndex        int // 索引タイブレーク用の投入順(ファイル横断の通し番号)
}

// isMovementSystem は本行が drop 対象の移動 system move かを返す。
func (r MoveRow) isMovementSystem() bool {
	return r.Category == "system" && movementSystemCodes[r.MoveCode]
}

// isRush はラッシュ版行(original_move_code を持つ)かを返す。
func (r MoveRow) isRush() bool {
	return strings.TrimSpace(r.OriginalMoveCode) != ""
}
