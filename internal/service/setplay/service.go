package setplay

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/plexiblinp/tacpendium-setplay/internal/model"
	setplayrepo "github.com/plexiblinp/tacpendium-setplay/internal/repository/setplay"
)

// ステップの役割。M19-02 で値域に parent を用意し、M19-05 の表記展開で実際に出力を始めた。
const (
	RoleFiller = "filler"
	RoleTarget = "target"
	// RoleParent は表記用親(through の技の直前に前置する行)。
	// 費用規則の分類 1 により計上ゼロ = ProposalStep.Counted が false になる(M19-05)。
	RoleParent = "parent"
)

// 打ち切り関連(§4.3)。「エンジンは安全上限まで全列挙 → グルーがランキング →
// 上位 limit 件を返す」構造。
//
// ★truncated(=「数え切れていない」)を立てる上限は 3 つ(M47-03 で 2 つ足した)——
// engineSafetyCap ／ maxChainUnits ／ maxChainLen。どれかに到達したら truncated=true。
// 立てないもの＝limit 切り(defaultLimit / maxLimit。totalFound と「さらに表示」で示す)／
// MaxFillers(1 提案の繋ぎの数の上限＝提案の形の定義。条件を絞っても変わらないため
// 「数え切れていない」には含めない＝M47-03 の Plan で開発者が選んだ)。
//
//   - engineSafetyCap: エンジン 1 target あたりの全列挙の安全上限。ここに到達した
//     ときに truncated=true。テストで打ち切り挙動を確認するため var。
//     ★実データでも発火する——M47-RESEARCH-01 B-1 の実測で、KA 80 では既定の条件で
//     62 件中 37 件が到達した(KA 40 以下では到達しない)。
//   - defaultLimit: limit 省略時の返却件数(§11-2 暫定=現行表示件数と同じ 200)。
//   - maxLimit: limit の上限。超過指定はここへ丸める(エラーにしない=n_min の流儀)。
//     実測の meaty 全種別最大 677・gap 既定種別最大 2875 を包含する 3000 とし、
//     既定運用(meaty・gap 既定種別)は全件到達可能にする。全種別 gap の 4884 は上位
//     優先で表示(ランキング後に切るため隠れるのは価値の低い分=§4.3 設計意図)。
var engineSafetyCap = 6000

// runEngine はサービスが計算の本体(エンジン)を呼ぶ入口である(M47-03)。
// ★要求の ctx を必ずエンジンへ渡すため、サービスの経路(meaty ／ gap ／ filler)はすべてここを通す。
// テストで「すべての経路が要求の ctx をエンジンへ渡している」ことを確かめるため var とする。
var runEngine = SuggestContext

const (
	defaultLimit = 200
	maxLimit     = 3000
)

// gap(汚連携)モードの G 範囲(開発者確定 2026-07-26)。G は 1〜13 で丸める。
const (
	gapGMinFloor   = 1  // g_min の下限(1 未満は 1 へ)。
	gapGMaxDefault = 13 // g_max の既定かつ上限(未指定・超過は 13 へ)。
)

// 提案モード(§4.4)。
const (
	ModeMeaty = "meaty" // 重ねる(既存挙動)。既定。
	ModeGap   = "gap"   // あえて重ねない(汚連携=完全空振り)。受理帯を [active+Gmin, active+Gmax] に差し替える。
	// ModeFiller は繋ぎだけ(M47-04・A4)。当てる技を持たず、繋ぎの全体フレームの合計＝KA − Remain
	// になるレシピを返す。安全飛び(A5)も同じ計算を Remain＝安全飛びの値で呼ぶ(値は画面が
	// characters.custom_states から読む)。
	ModeFiller = "filler"
)

// fillerOnlyTargetCode は繋ぎだけモードでエンジンへ渡すダミーの当てる技の名前(M47-04)。
// 行には出さない(buildFillerOnlyProposal は当てる技のステップを積まない)。
const fillerOnlyTargetCode = "__filler_only__"

// ReasonKnockdownNegative は負 KA のとき提案 0 件の理由としてレスポンスに載せる
// 専用コード(§4.2)。NULL の ErrKnockdownNotSet(400)とは別で、これは 200 で返す。
const ReasonKnockdownNegative = "knockdown_advantage_negative"

// ReasonRemainNotBelowKnockdown は繋ぎだけモードで Remain ≥ KA のとき、提案 0 件の理由として
// 返す(M47-04)。繋ぎを 1 つも置けない(Remain＝KA は繋ぎ 0 個、Remain＞KA は予算が負)。
const ReasonRemainNotBelowKnockdown = "remain_not_below_knockdown"

// エラーセンチネル。
//
// 単体版: 本家の ErrComboNotFound ・ErrKnockdownNotSet は持たない(コンボが無く、KA は必ず入力として渡る)。
var (
	// ErrInvalidRemain は繋ぎだけモードの Remain が負の場合に返る(M47-04)。
	// ハンドラが先に 400 にするため通常は到達しない(防御)。
	ErrInvalidRemain = errors.New("setplay: remain must be non-negative")
)

// 対象種別(target type)キー。category + is_projectile(+ rush は元技 category)から算出する(§4.2 拡張)。
const (
	TargetTypeNormal            = "normal"             // 通常技
	TargetTypeUnique            = "unique"             // 特殊技
	TargetTypeSpecial           = "special"            // 必殺技(非弾)
	TargetTypeSpecialProjectile = "special_projectile" // 必殺技(弾)
	TargetTypeThrow             = "throw"              // 投げ
	TargetTypeNormalRush        = "normal_rush"        // 通常技(ラッシュ版)
	TargetTypeUniqueRush        = "unique_rush"        // 特殊技(ラッシュ版)
	// 元技が special 等のラッシュ版・その他の派生種別は提供しない("" 扱い)。
)

// isRushTargetType は rush 種別か。rush は is_derived=true が正常なため列挙時に除外しない。
func isRushTargetType(t string) bool {
	return t == TargetTypeNormalRush || t == TargetTypeUniqueRush
}

// SuggestSort は並べ替えモード。
type SuggestSort string

const (
	// SortByN は持続が深い順(N 降順)。既定。
	SortByN SuggestSort = "n"
	// SortByTarget は重ねる技(target)順。
	SortByTarget SuggestSort = "target"
)

// SuggestParams は提案生成の絞り込み・並べ替え指定。
type SuggestParams struct {
	// NMin は最小持続番号。0(未指定)は既定 1。エンジンが 1 未満を 1 に丸める。
	NMin int
	// NMax は最大持続番号(N の上限)。meaty モードでのみ有効。0/未指定は上限なし
	// (=target.Active まで)。持続の長い技(波動拳など、画面に残る弾の硬直を持続扱い
	// している target)で深い N を切り落とし候補を絞るために使う。NMax<NMin は空結果。
	NMax int
	// TargetTypes は自動列挙で許可する対象種別キー集合。空は「該当なし=結果なし」。
	// TargetMoveID 指定時は無視される。
	TargetTypes []string
	// IncludeZeroDamage が true のとき damage 0/NULL の技も target 対象にする。
	// 既定 false = damage>0 のみ(ドライブパリィ等のダメージ0技を除外)。
	// system は type="" のため元から自動対象外。
	IncludeZeroDamage bool
	// Sort は並べ替えモード。"" は SortByN 扱い。
	Sort SuggestSort
	// TargetMoveID は明示指定 target。nil で自動列挙(§4.2 の種別/damage/is_derived/is_aerial を適用)。
	TargetMoveID *int64
	// Mode は提案モード(§4.4)。"" / ModeMeaty は重ねる(既存)。ModeGap はあえて重ねない。
	Mode string
	// GMin/GMax は gap モードの G 範囲(§4.4)。Mode!=gap のとき無視。
	// GMin<1 は 1、GMax>13 または未指定(0)は 13 に丸める。GMin>GMax は空結果。
	GMin int
	GMax int
	// Limit は返却件数の上限(§4.3)。0/未指定は defaultLimit、maxLimit 超過は丸める。
	Limit int
	// Remain は繋ぎだけモード(ModeFiller)の「繋ぎの後に残るフレーム」X(M47-04)。
	// 繋ぎの全体フレームの合計＝KA − Remain の組を返す。Mode!=filler のとき無視。負はエラー。
	Remain int

	// 結果の行の絞り込み(M47-05・CHANGE-254)。どれも既定 false(＝絞らない)。true のものを AND で掛ける。
	// 掛けるのは行を全部作った後・並べ替えと totalFound の前である(＝totalFound は絞った後の総数)。

	// ApproachOnly が true のとき、接近の手(isApproachMove)を含む行だけを返す。
	// 繋ぎ・表記用親・当てる技のすべてのステップを見る(当てる技がラッシュの技の行も数える＝開発者の回答)。
	ApproachOnly bool
	// BlockSafeOnly が true のとき、当てる技の on_block + N − 1 ≥ 0 の行だけを返す(0 を含む)。
	// on_block が NULL の技の行は返さない(出さなかった件数は返さない＝D-1007)。
	// ★重ねるモード(meaty)だけで効く。gap ・filler では無視する(当てないため／当てる技が無いため)。
	BlockSafeOnly bool
	// ChainCancelOnly が true のとき、連打キャンセル(長さ 2 以上の合成の繋ぎ単位)を含む行だけを返す。
	// 型 A の連打版の行は合成単位の 2 打目以降にしか入らないため、この 1 つの判定で型 A ・型 B の両方を拾う。
	ChainCancelOnly bool

	// Votes は票のスコア(単体版だけ。S6・HANDOVER D17)。鍵は RecipeKey、値は good − bad。
	// 空なら並びは本家と同じ。空でなければ本家の並べ替えの後にスコアの降順で安定に並べ直し、その後で limit を切る(votes.go)。
	Votes map[string]int
}

// moveTargetType は category + is_projectile(+ rush は元技 category)から対象種別キーを返す。
// 提供しない種別(super_art / system / 元 special のラッシュ等)は "" を返す。
// catByID は同キャラ move の id→category マップ(rush_variant の元技分類に使う)。
func moveTargetType(mc setplayrepo.MoveCandidate, catByID map[int64]string) string {
	switch mc.Category {
	case model.MoveCategoryNormal:
		return TargetTypeNormal
	case model.MoveCategoryUnique:
		return TargetTypeUnique
	case model.MoveCategoryThrow:
		return TargetTypeThrow
	case model.MoveCategorySpecial:
		if mc.IsProjectile {
			return TargetTypeSpecialProjectile
		}
		return TargetTypeSpecial
	case model.MoveCategoryRushVariant:
		// 元技のカテゴリで rush 種別を判定する(元 normal→normal_rush、元 unique→unique_rush)。
		if mc.OriginalMoveID == nil {
			return ""
		}
		switch catByID[*mc.OriginalMoveID] {
		case model.MoveCategoryNormal:
			return TargetTypeNormalRush
		case model.MoveCategoryUnique:
			return TargetTypeUniqueRush
		default:
			return "" // 元 special 等のラッシュは提供しない
		}
	default:
		return ""
	}
}

// ProposalStep は提案の 1 ステップ(表示順)。
type ProposalStep struct {
	MoveID  int64
	Code    string
	Role    string
	Counted bool
}

// Proposal は 1 件の提案。
type Proposal struct {
	Steps        []ProposalStep
	S            int    // 第 1 active の絶対フレーム。filler(当てる技を持たない)では 0(M47-04)
	N            int    // 起き上がりに重なる持続フレーム番号。filler では 0(M47-04)
	Landing      int    // = KA + 1
	TargetActive int    // 当てる技の持続。filler では 0(M47-04)
	Mode         string // "meaty" | "gap" | "filler"(§4.4・M47-04)
	G            int    // gap のとき = N − active(最終 active が起き上がりの G フレーム前＝完全空振り)。meaty / filler のとき 0
	// Remain は繋ぎだけモードのとき、繋ぎの後に残るフレーム(＝KA − Σ繋ぎ)。他のモードでは 0(M47-04)。
	Remain int

	// ランキング・内部処理用の非公開フィールド(JSON 非公開)。
	targetCode string // 重ねる技(target)の code。sort=target の第 1 キー
	sumFiller  int    // Σ filler.total
	sortName   string // 決定論的タイブレーク用(全ステップ code の連結)
	recipeHash string // 型 B の重複の判定に使う(M47-05)。単体版は alreadyAdopted を持たないので、この用途だけ

	// M47-05: 絞り込み(refineProposals)と隣り合いの並べ替えのための非公開の印。
	hasApproach    bool           // 接近の手(isApproachMove)を含む
	hasChainCancel bool           // 長さ 2 以上の合成の繋ぎ単位を含む
	targetOnBlock  *int           // 当てる技の on_block。filler(当てる技を持たない)では nil
	parts          []proposalPart // 繋ぎの単位ごとのステップ(表記順)。隣り合いの並べ替え(追補④で全キャラ)は、この順だけを入れ替える
	targetPart     *proposalPart  // 当てる技のステップ(表記用親を含む)。filler では nil
}

// proposalPart は提案の 1 区切り(繋ぎの 1 単位、または当てる技)のステップ群である(M47-05)。
// 「別々の区切りに置いた技が、最速で押すと連打版に化ける」判定と並べ替えに使う(M47-05 追補②)。
type proposalPart struct {
	steps []ProposalStep
	// first / last は、区切りの最初 ・最後のステップの技である。表記用親が前置されていれば、最初は親である。
	first setplayrepo.MoveCandidate
	last  setplayrepo.MoveCandidate
}

// SuggestResult は提案生成の結果。
type SuggestResult struct {
	// Proposals はランキング後に上位 limit 件へ絞った提案。
	Proposals []Proposal
	// Truncated は安全上限に到達し「数え切れていない」ときのみ true。安全上限は 3 つ——
	// エンジンの engineSafetyCap ／ 繋ぎの単位の構築の maxChainUnits ・maxChainLen(後 2 つは M47-03)。
	// limit で切っただけの状態は Truncated にしない(§4.3.2-3)。MaxFillers(提案の形の定義)も含めない。
	Truncated bool
	// TotalFound はランキング対象となった総解数(limit で切る前)。UI の件数表示・
	// 「さらに表示」の要否判定に使う(§4.3)。
	TotalFound int
	// Reason は提案 0 件を返す際の理由コード。負 KA＝ReasonKnockdownNegative ／
	// 繋ぎだけモードで Remain ≥ KA＝ReasonRemainNotBelowKnockdown(M47-04)の 2 つ。
	// 正常に提案がある場合は空。
	Reason string
}

// MoveCandidateReader はキャラの moves 候補と親子関係を読む。
//
// 単体版: キャラは数値 ID ではなく code(例 "ryu")で指す。本家の CharacterCode(ID → code)は要らないので持たない。
type MoveCandidateReader interface {
	ListCharacterMoveCandidates(ctx context.Context, characterCode string) ([]setplayrepo.MoveCandidate, error)
	ListCharacterMoveDerivations(ctx context.Context, characterCode string) ([]setplayrepo.MoveDerivation, error)
}

// isSoloUnavailable は「単独では出せない(親からの派生でのみ出る)技」を判定する。
// 正本は DES-003 §3.3 / M19-DESIGN-07 §5-4(CHANGE-093・D-227 で 'unknown' を含めた)。
//
// ★この述語が答えるのは「自分がこの技を単独で入力できるか」である。
// 「相手がこの技を出しうるか」(確定反撃サーチの相手技候補)には転用しないこと——
// 親から出せば実機で成立する技を落とす偽陰性になる(D-260)。
//
// hasParent は move_derivations に子として存在するか。同じ概念の判定をここ 1 本に集約し、
// filler ゲート・target ゲート・合成 filler 単位の 3 箇所がすべてこれを呼ぶ(第 2 の述語を作らない)。
func isSoloUnavailable(m setplayrepo.MoveCandidate, hasParent bool) bool {
	if !hasParent {
		return false
	}
	return m.StartupBasis == model.MoveStartupBasisStandalone ||
		m.StartupBasis == model.MoveStartupBasisUnknown
}

// Service はセットプレイ自動提案のグルー層。
type Service interface {
	// SuggestForKnockdown はキャラ characterCode と KA から成立するセットプレイ提案を返す(副作用なし)。
	//
	// 単体版: 本家の SuggestForCombo(comboID)に当たる。本家はコンボから KA とキャラを引くが、
	// 単体版はどちらも入力として受け取る(HANDOVER §2.2)。
	SuggestForKnockdown(ctx context.Context, characterCode string, ka int, params SuggestParams) (*SuggestResult, error)
}

type service struct {
	moveReader MoveCandidateReader
}

// NewService はグルー層 Service を構築する。
func NewService(moveReader MoveCandidateReader) Service {
	return &service{moveReader: moveReader}
}

func (s *service) SuggestForKnockdown(ctx context.Context, characterCode string, ka int, params SuggestParams) (*SuggestResult, error) {
	landing := ka + 1

	// M19-LIMITATION-NOTICE (e): 負 KA(有利フレームがマイナス)のコンボは提案不可。
	//   NULL(400)とは別で、保存が許される正当値(VAL-C10)のため 200 + 専用理由コードで返す。
	//   この扱いを変更したら、ヒント文面(web i18n `setplay.limitations.*`)を同時に更新すること。
	if ka < 0 {
		return &SuggestResult{Proposals: []Proposal{}, Reason: ReasonKnockdownNegative}, nil
	}

	mode := params.Mode
	if mode == "" {
		mode = ModeMeaty
	}
	if mode == ModeFiller {
		if params.Remain < 0 {
			return nil, ErrInvalidRemain
		}
		// Remain ≥ KA は繋ぎを 1 つも置けない(= KA は繋ぎ 0 個の行だけ、> KA は予算が負)。
		if params.Remain >= ka {
			return &SuggestResult{Proposals: []Proposal{}, Reason: ReasonRemainNotBelowKnockdown}, nil
		}
	}

	cands, err := s.moveReader.ListCharacterMoveCandidates(ctx, characterCode)
	if err != nil {
		return nil, fmt.Errorf("list move candidates: %w", err)
	}

	derivations, err := s.moveReader.ListCharacterMoveDerivations(ctx, characterCode)
	if err != nil {
		return nil, fmt.Errorf("list move derivations: %w", err)
	}
	parentsByChild := make(map[int64][]int64, len(derivations))
	for _, d := range derivations {
		parentsByChild[d.ChildMoveID] = append(parentsByChild[d.ChildMoveID], d.ParentMoveID)
	}

	byID := make(map[int64]setplayrepo.MoveCandidate, len(cands))
	for _, m := range cands {
		byID[m.ID] = m
	}

	// M47-05: 接近の判定に使うキャラ固有の接近技(辞書はキャラの code で引く)。
	marks := proposalMarks{approach: approachCodesFor(characterCode)}
	// 表記用親の解決器。through の技の直前に前置する親を決める(費用規則 分類 1 = 計上ゼロ)。
	parentOf := newParentResolver(byID, parentsByChild)

	singles := collectFillerSingles(cands, parentsByChild)
	// 要求の取り消しを、以降の計算の繰り返しの内側で見る(M47-03・A2)。
	probe := newCancelProbe(ctx)
	units, unitsTruncated := buildFillerUnits(singles, cands, parentsByChild, ka+2, probe)
	if probe.err != nil {
		return nil, fmt.Errorf("build filler units: %w", probe.err)
	}
	unitByName := make(map[string]fillerUnit, len(units))
	fillers := make([]Move, 0, len(units))
	for _, u := range units {
		unitByName[u.Name] = u
		fillers = append(fillers, Move{Name: u.Name, TotalFrames: u.Total})
	}

	if mode == ModeFiller {
		return s.suggestFillerOnly(ctx, ka, params, fillers, unitByName, parentOf, marks, unitsTruncated)
	}

	targets := collectTargets(cands, parentsByChild, params)

	opt := DefaultOptions()
	if params.NMin != 0 {
		opt.NMin = params.NMin // NMin < 1 はエンジンが 1 へ丸める。gap では帯に用いず無視される。
	}
	// エンジンは安全上限まで全列挙する(打ち切りはランキング後にグルーで行う=§4.3.2)。
	opt.MaxResults = engineSafetyCap

	// 安全上限に到達した(=数え切れていない)ときのみ truncated=true。
	// 繋ぎの単位の構築で上限(maxChainUnits / maxChainLen)に達したときも同じ(M47-03)。
	truncated := unitsTruncated
	proposals := []Proposal{}
	for _, tc := range targets {
		if probe.check() {
			return nil, fmt.Errorf("suggest: %w", probe.err)
		}
		target := Target{Name: tc.Code, Startup: *tc.Startup, Active: *tc.Active}
		// M19-LIMITATION-NOTICE (f): 受理帯の差し替えだけでモード/上限を実現する
		//   (探索構造・恒等式は不変)。この扱いを変更したら、ヒント文面を同時に更新すること。
		optT := opt
		if mode == ModeGap {
			// M19-02-GAP-DEF: gap は N を [active+Gmin, active+Gmax] へ(完全空振り)。ロールバック点。
			optT.Band = gapBand(params.GMin, params.GMax, target.Active)
		} else {
			optT.Band = meatyBand(opt.NMin, params.NMax, target.Active) // meaty は n_max 指定時のみ上限を絞る。
		}
		suggestions, serr := runEngine(ctx, ka, fillers, target, optT)
		if serr != nil {
			if errors.Is(serr, context.Canceled) || errors.Is(serr, context.DeadlineExceeded) {
				return nil, fmt.Errorf("suggest: %w", serr)
			}
			// target は事前フィルタ済みのため通常到達しないが、防御的にスキップ。
			continue
		}
		if opt.MaxResults > 0 && len(suggestions) >= opt.MaxResults {
			truncated = true
		}
		for _, sg := range suggestions {
			if probe.tick() {
				return nil, fmt.Errorf("suggest: %w", probe.err)
			}
			proposals = append(proposals, buildProposal(landing, mode, tc, sg, unitByName, parentOf, marks))
		}
	}
	if probe.check() {
		return nil, fmt.Errorf("suggest: %w", probe.err)
	}

	// M47-05: 連打キャンセルになる隣り合い ・同じレシピの重複 ・3 つの絞り込みを、並べ替えと totalFound の前に掛ける
	//   (totalFound を「絞った後の総数」にし、画面の件数表示と「さらに表示」を食い違わせないため)。
	proposals = refineProposals(proposals, params, mode)

	// M19-LIMITATION-NOTICE (g): 打ち切りはここ(ランキング後)で行う。エンジンの安全上限
	//   より前ではない。この扱いを変更したら、ヒント文面を同時に更新すること。
	rank(proposals, params.Sort, mode == ModeGap)
	applyVotes(proposals, params.Votes) // 単体版(S6): 票を第一キーに。limit で切る前に掛ける
	totalFound := len(proposals)

	limit := params.Limit
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit // 超過指定は丸める(エラーにしない=n_min の流儀)。
	}
	if len(proposals) > limit {
		proposals = proposals[:limit] // limit 切りは truncated にしない(§4.3.2-3)。
	}

	return &SuggestResult{Proposals: proposals, Truncated: truncated, TotalFound: totalFound}, nil
}

// suggestFillerOnly は繋ぎだけモード(M47-04・A4 ／ A5 の安全飛び)の提案を返す。
//
// ★計算の本体(エンジン Suggest)は変えない。発生 Remain+1・持続 1 のダミーの当てる技を
// 1 回だけ渡す(開発者の決定＝指示書 §9.4-1 案 (a))。このとき受理帯は N＝1 の 1 点になり、
// 予算 = KA + 2 − 1 − (Remain + 1) = KA − Remain をちょうど使い切る組だけが受理される
// ＝**繋ぎの全体フレームの合計 = KA − Remain**(M47-RESEARCH-01 D-1)。
//
// ★繋ぎ 0 個の行(Remain = KA のときだけ出る)は捨てる。呼び出し側が Remain ≥ KA を先に
// 0 件で返すため通常は出ないが、防御として残す。
//
// meaty / gap の経路(target のループ・buildProposal・rank)には触れない。
//
// unitsTruncated は繋ぎの単位の構築で上限(maxChainUnits / maxChainLen)に達したかである(M47-03)。
// 要求が取り消されたら計算の途中で止まり、取り消しの誤りを返す(M47-03・A2)。
func (s *service) suggestFillerOnly(ctx context.Context, ka int, params SuggestParams, fillers []Move, unitByName map[string]fillerUnit, parentOf parentResolver, marks proposalMarks, unitsTruncated bool) (*SuggestResult, error) {
	opt := DefaultOptions()
	opt.NMin = 1
	opt.MaxResults = engineSafetyCap
	dummy := Target{Name: fillerOnlyTargetCode, Startup: params.Remain + 1, Active: 1}
	suggestions, err := runEngine(ctx, ka, fillers, dummy, opt)
	if err != nil {
		return nil, fmt.Errorf("suggest filler only: %w", err)
	}
	truncated := unitsTruncated || (opt.MaxResults > 0 && len(suggestions) >= opt.MaxResults)

	probe := newCancelProbe(ctx)
	proposals := make([]Proposal, 0, len(suggestions))
	for _, sg := range suggestions {
		if probe.tick() {
			return nil, fmt.Errorf("suggest filler only: %w", probe.err)
		}
		if len(sg.Fillers) == 0 {
			continue
		}
		proposals = append(proposals, buildFillerOnlyProposal(ka+1, params.Remain, sg, unitByName, parentOf, marks))
	}
	if probe.check() {
		return nil, fmt.Errorf("suggest filler only: %w", probe.err)
	}

	// M47-05: 重ねる ・汚連携と同じ絞り込みを掛ける(BlockSafeOnly は当てる技が無いため効かない)。
	proposals = refineProposals(proposals, params, ModeFiller)

	rankFillerOnly(proposals)
	applyVotes(proposals, params.Votes) // 単体版(S6): 票を第一キーに。limit で切る前に掛ける
	totalFound := len(proposals)

	limit := params.Limit
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	if len(proposals) > limit {
		proposals = proposals[:limit]
	}

	return &SuggestResult{Proposals: proposals, Truncated: truncated, TotalFound: totalFound}, nil
}

// buildFillerOnlyProposal は繋ぎだけの Suggestion を Proposal へ変換する(M47-04)。
// 当てる技のステップは積まない。繋ぎの展開(合成単位の構成技・through の表記用親)は
// buildProposal と同じ規則である。S / N / TargetActive / G は意味を持たないため 0。
func buildFillerOnlyProposal(landing, remain int, sg Suggestion, unitByName map[string]fillerUnit, parentOf parentResolver, marks proposalMarks) Proposal {
	p := Proposal{
		Landing: landing,
		Mode:    ModeFiller,
		Remain:  remain,
	}
	for _, f := range sg.Fillers {
		p.addFillerUnit(unitByName[f.Name], parentOf, marks)
		p.sumFiller += f.TotalFrames
	}
	p.assemble()
	return p
}

// proposalMarks は行を作るときに印を立てるための材料である(M47-05)。
type proposalMarks struct {
	// approach はそのキャラの固有の接近技の code 集合(characterApproachMoves)。
	approach map[string]bool
}

// appendMoveSteps は 1 つの技(through なら表記用親を前置)のステップを steps へ積む。
// 返すのは積んだ後のステップ列 ・接近の手を含むか ・積んだ最初のステップの技(親を前置したなら親)。
func appendMoveSteps(steps []ProposalStep, m setplayrepo.MoveCandidate, role string, parentOf parentResolver, marks proposalMarks) ([]ProposalStep, bool, setplayrepo.MoveCandidate) {
	approach := false
	first := m
	if p := parentOf(m); p != nil {
		steps = append(steps, ProposalStep{MoveID: p.ID, Code: p.Code, Role: RoleParent, Counted: false}) // 分類 1: 計上ゼロ
		approach = approach || isApproachMove(*p, marks.approach)
		first = *p
	}
	steps = append(steps, ProposalStep{MoveID: m.ID, Code: m.Code, Role: role, Counted: true})
	approach = approach || isApproachMove(m, marks.approach)
	return steps, approach, first
}

// addFillerUnit は繋ぎの 1 単位(合成単位は構成技へ展開)を区切りとして積み、印を立てる。
func (p *Proposal) addFillerUnit(u fillerUnit, parentOf parentResolver, marks proposalMarks) {
	var part proposalPart
	for i, m := range u.Members {
		var approach bool
		var first setplayrepo.MoveCandidate
		part.steps, approach, first = appendMoveSteps(part.steps, m, RoleFiller, parentOf, marks)
		if i == 0 {
			part.first = first
		}
		p.hasApproach = p.hasApproach || approach
	}
	if n := len(u.Members); n > 0 {
		part.last = u.Members[n-1]
	}
	if len(u.Members) >= 2 {
		p.hasChainCancel = true // 合成単位＝連打キャンセル(型 A の連打版も合成単位の中にだけ入る)
	}
	p.parts = append(p.parts, part)
}

// setTarget は当てる技を区切りとして積み、印を立てる。
func (p *Proposal) setTarget(tc setplayrepo.MoveCandidate, parentOf parentResolver, marks proposalMarks) {
	var part proposalPart
	var approach bool
	part.steps, approach, part.first = appendMoveSteps(nil, tc, RoleTarget, parentOf, marks)
	part.last = tc
	p.hasApproach = p.hasApproach || approach
	p.targetPart = &part
	p.targetOnBlock = tc.OnBlock
}

// assemble は区切り(parts ・targetPart)から Steps ・sortName ・recipeHash を作り直す。
// 隣り合いの並べ替えで区切りの順を変えた後にも呼ぶ(M47-05)。
func (p *Proposal) assemble() {
	n := 0
	for _, pt := range p.parts {
		n += len(pt.steps)
	}
	if p.targetPart != nil {
		n += len(p.targetPart.steps)
	}
	steps := make([]ProposalStep, 0, n)
	for _, pt := range p.parts {
		steps = append(steps, pt.steps...)
	}
	if p.targetPart != nil {
		steps = append(steps, p.targetPart.steps...)
	}
	idParts := make([]string, len(steps))
	codeParts := make([]string, len(steps))
	for i, st := range steps {
		// 単体版: 本家は setup_steps の正準形の sha256(setupsvc.CalcSetupRecipeHash)を入れる。
		// 単体版での用途は「同じ move ID 列か」の判定(dedupeByRecipe)だけなので、ID 列そのものを鍵にする。
		// 本家の正準形は modifiers が常に null のため「ID 列が同じ ⇔ ハッシュが同じ」で、等しさの意味は変わらない。
		idParts[i] = strconv.FormatInt(st.MoveID, 10)
		codeParts[i] = st.Code
	}
	p.Steps = steps
	p.sortName = strings.Join(codeParts, ">")
	p.recipeHash = strings.Join(idParts, ",")
}

// rankFillerOnly は繋ぎだけモードの並び(M47-04)。手数の少ない順 → 表記の昇順。
// 推測: 短いレシピほど入力しやすい(指示書 §4.1・§9.2 が推測で既定にすることを許している)。
// Σ繋ぎはどの行も KA − Remain で等しいため、キーに使わない。
func rankFillerOnly(props []Proposal) {
	sort.SliceStable(props, func(i, j int) bool {
		a, b := &props[i], &props[j]
		if len(a.Steps) != len(b.Steps) {
			return len(a.Steps) < len(b.Steps)
		}
		return a.sortName < b.sortName
	})
}

// meatyBand は meaty モードで n_max(N の上限)が指定されたときだけ受理帯を
// [max(1,nMin), min(active, nMax)] に絞る。nMax<=0 は絞りなし(nil=既定挙動＝
// 一般化前と完全一致)。nMax<nMin のときは Lo>Hi となりエンジンが空結果を返す。
func meatyBand(nMin, nMax, active int) *NBand {
	if nMax <= 0 {
		return nil
	}
	lo := nMin
	if lo < 1 {
		lo = 1
	}
	hi := active
	if nMax < hi {
		hi = nMax
	}
	return &NBand{Lo: lo, Hi: hi}
}

// M19-02-GAP-DEF: gap(あえて重ねない=完全空振り)モードの受理帯を返す。ロールバック点。
//
// gap の隙間 G ＝「target の**最終** active が起き上がり(KA+1)より何フレーム前に出るか」
// (＝技を丸ごと空振りさせ、起き上がりに一切重ねない)。
//
//	最終 active = (KA+1) − G、最終 active = S + active − 1、N = KA+2 − S ⟹ **G = N − active**。
//
// よって N の受理帯は [active + Gmin, active + Gmax]。G≥1 で最終 active ≤ KA(起き上がり前)
// ＝完全空振りが保証され、N > active ゆえ meaty(N ≤ active)と**完全に排他**になる。
//
// 【経緯】当てる意図のセットプレイは meaty で実現済みのため、gap は「当てない」専用。
// 定義は 2 度是正した(後→第1active前→最終active前=完全空振り。開発者確認 2026-07-26)。
// 実機フィードバックで覆る可能性があるため本関数に隔離し、単一コミット revert で戻せる。
//
// GMin<1→1、GMax>13 または未指定(0)→13 に丸める。GMin>GMax のときは Lo>Hi となり
// エンジンが空結果を返す。active は target ごとに異なるため per-target で渡す。
func gapBand(gMin, gMax, active int) *NBand {
	if gMin < gapGMinFloor {
		gMin = gapGMinFloor
	}
	if gMax <= 0 || gMax > gapGMaxDefault {
		gMax = gapGMaxDefault
	}
	return &NBand{Lo: active + gMin, Hi: active + gMax}
}

// collectFillerSingles は単発 filler の候補集合を返す(M19-05 段階 A)。
//
// 述語(M19-DESIGN-07 §5-3/§5-4・M19-DESIGN-08 §1):
//
//	total >= 1
//	  AND NOT (category = 'target_combo' AND is_derived = 1)   -- ゲート 1
//	  AND NOT 単独入力不可                                       -- ゲート 2
//
// ★ゲート 1 は「category を外す」ではなく「category を is_derived へ置き換える」ものである。
// 外すと篩が全部落ちる(D-148)。空振りでも出るパターン 3 だけが新たに候補へ入り、
// 前段のヒット/ガードを要するパターン 1/2 は引き続き除外される。
//
// ★ゲート 2 は設計過程で発見された既存の穴の是正(単独入力不可の除外が無かった)。
// 該当行はチェーン合成・表記展開の経由でのみ登場させる——除外して終わりにしない。
//
// ★is_derived / rush_variant / setup_only 単独では絞らない(M19-02 §4.1「除外しないもの」)。
//
// M19-LIMITATION-NOTICE (c): この filler 規則を変更したら、ヒント文面
// (web i18n `setplay.limitations.*`)を同時に更新すること。
func collectFillerSingles(cands []setplayrepo.MoveCandidate, parentsByChild map[int64][]int64) []setplayrepo.MoveCandidate {
	out := make([]setplayrepo.MoveCandidate, 0, len(cands))
	for _, m := range cands {
		if m.Total == nil || *m.Total < 1 {
			continue
		}
		if m.Category == model.MoveCategoryTargetCombo && m.IsDerived {
			continue // ゲート 1
		}
		if isSoloUnavailable(m, len(parentsByChild[m.ID]) > 0) {
			continue // ゲート 2
		}
		out = append(out, m)
	}
	return out
}

// maxChainLen は合成 filler 単位に組む連鎖の最大長。
// k は本来 KA 上限で自然に有限になる(実用上 KA ≦ 60 程度なら高々 5〜6 連＝M19-DESIGN-07 §2-2)。
// ★到達して「もう 1 段伸ばせる単位」が残っていたら truncated=true にする(M47-03)。
// ★実データでも到達する——M47-03 の実測(実 seed・31 キャラ)で、KA 80 以下では 0 キャラ、
// KA 100 で 28 キャラ(うち maxChainUnits には達さず本上限だけに達するのが 5 キャラ)、
// KA 120 以上で全 31 キャラが到達した。予算側の枝刈りが先に効くのは KA 80 以下である。
// テストで到達を確認するため var とする。
var maxChainLen = 8

// maxChainUnits は 1 キャラあたりに作る合成単位の個数の上限。
//
// ★長さの上限(maxChainLen)だけでは個数を縛れない。budgetCap = KA + 2 は KA に比例して
// 緩むため、KA が大きいほど |グループ員|^k が素直に立ち上がる。**KA は VAL-C10 が
// WARNING のみで −600〜+600 が警告なしに保存できる**ため、極端な KA のコンボでは
// 「エンジンの枝刈りが効く前に、確定的に大量の単位を構築する」経路になりうる
// (KA=600・3 員グループで Σ3^k ≒ 9,840)。
//
// engineSafetyCap(エンジン側の全列挙の安全上限)と同じ役割を単位構築側に置いたものである。
// ★到達したら truncated=true にする(M47-03 ＝ D-1003。それまでは黙って打ち切っていた)。
// KA 60 までは到達しない(最大 1217＝lily)が、KA 80 では 31 キャラ中 20 キャラが到達する
// (M47-RESEARCH-01 B-2 の実測)。
// テストで到達を確認するため var とする。
var maxChainUnits = 2000

// fillerUnit は探索へ渡す空振り 1 単位。単発技も長さ 1 の単位として扱う。
//
// 合成単位(長さ 2 以上)は「チェーングループから作った疑似 filler」であり、
// 単位 total ＝ Σ(非末端の chain_cancel_total) ＋ 末端の total(M19-DESIGN-07 §2-2)。
// 費用は単位ごとに独立なので、エンジン・受理帯・恒等式は不変(区分 2)。
type fillerUnit struct {
	// Name はエンジンへ渡す識別子。単発は move_code、合成は code を ">" で連結したもの。
	Name string
	// Total は単位全体の実消費フレーム。
	Total int
	// Members は表記順の構成技。
	Members []setplayrepo.MoveCandidate
}

// buildFillerUnits は単発 filler と合成 filler 単位を組み立てる(M19-05 段階 B)。
//
// # 合成の規則
//
//   - チェーングループ員 ＝ chain_cancel_total が非 NULL の行。
//     非 NULL であること自体がグループ員であることを兼ねる(DES-003 §3.3)。
//   - 先頭位置には単独入力可の員だけを置ける(単独で出せない技から連携は始められない)。
//   - 2 番目以降には、単独で出せるグループ員 X を押したときに**直前の技の後に実際に出る技**を置く
//     (M47-05 追補②で是正＝M19-DESIGN-07 §9 項目 8 の「親集合駆動の差し込み規則」。rapidTable.next)。
//     X に連打版 R が在り、直前の技の元技(直前が連打版ならその元技)が R の親(move_derivations)なら R、
//     そうでなければ X(ふつうの連打キャンセル)。
//     ★キャラ名でなく性質(親集合)で書いている。
//     ・ザンギエフ: 連打版の親はグループの通常版 3 技 ⇒ 2 打目以降は常に連打版、3 打目以降も連打版(§9-1)。
//     ・ダルシム: 連打版しゃがみ弱P の親は立ち弱P ・アジャイルキックだけ ⇒ 直前がしゃがみ弱P(素・連打版)なら
//     素のしゃがみ弱P が出る(ドメイン知識集成 §3-5)。
//     ・通常のキャラ: 連打版が無い ⇒ グループ全員がそのまま続く(M47-05 追補②の前と同じ)。
//     ★【旧規則・失効】M47-05 追補②の前は「単独入力不可の員があればその集合だけを 2 番目以降に置く」だった。
//     ザンギエフには合うが、ダルシムでは「しゃがみ弱P＞連打版しゃがみ弱P」を作り、
//     「立ち弱P＞立ち弱P」等のふつうの連打キャンセルを作っていなかった。
//   - 差し込んだ行自身の値を費用規則が読む(末端の total も連打版の値になる)。
//
// cands は同キャラの全候補。2 番目以降の位置には単発 filler として落ちた行(単独入力不可の
// 連打版など)も登場するため、singles だけでなく全候補から員を拾う必要がある。
// budgetCap は S の上限(= KA + 2)。単位 total がこれを超える組は作らない。
//
// ★合成単位を**追加**するが、チェーングループ員を単発として複数含む解を**除去しない**
// (M19-05 時点・開発者裁定 2026-08-09 で「除去せず残件として報告」)。そのため同じ表記
// (例: リュウ 立弱P > 立弱P)が、合成単位経由の S(実機 22F)と単発 2 つ分の S(全 total 計上 26F)の
// 2 通りでエンジンから出る。
// ⇒ **M47-05 で、行を作った後に費用の小さい 1 行へまとめるようにした**(refineProposals の
// dedupeByRecipe・D-1003)。同じ表記の 2 行が両方とも受理帯に入れば 22F の 1 行だけが残る。
// ⇒ **M47-05 追補④で、単発のチェーン員が隣り合う行は全キャラで並べ替えるか落とす**ようにした
// (adjustChainAdjacency・M19-DESIGN-07 §9-3 の最速原則)。26F の側だけが受理帯に入る KA でも、
// 単発ずつの行(実機では入力できない)は残らない。
// 本関数は引き続き除去しない——除去は結果の行の側で行う。
//
// # 打ち切りと取り消し(M47-03)
//
// 2 つ目の戻り値は、上限に達して単位を作り切れなかったか(＝truncated)である。
// maxChainUnits に達したとき、または maxChainLen まで伸ばした後に「もう 1 段伸ばせる単位」が
// 残っていたときに true。probe が取り消しを観測したら途中で止まる(呼び出し側が probe.err を見る)。
func buildFillerUnits(singles, cands []setplayrepo.MoveCandidate, parentsByChild map[int64][]int64, budgetCap int, probe *cancelProbe) ([]fillerUnit, bool) {
	units := make([]fillerUnit, 0, len(singles))
	for _, m := range singles {
		units = append(units, fillerUnit{Name: m.Code, Total: *m.Total, Members: []setplayrepo.MoveCandidate{m}})
	}

	// チェーングループを組む。先頭候補は「単発 filler として通った員」に限る
	// (ゲート 1/2 で落ちた技から連携を始めることはできない)。
	heads := make([]setplayrepo.MoveCandidate, 0, len(singles))
	for _, m := range singles {
		if m.ChainCancelTotal != nil {
			heads = append(heads, m)
		}
	}
	if len(heads) == 0 {
		return units, false
	}
	// 2 番目以降は、押す技(単独で出せるグループ員＝heads)ごとに「直前の技の後に実際に出る技」を
	// 連打版の表で決める(M47-05 追補②・親集合駆動の差し込み規則)。通常のキャラは表が空で、押した技がそのまま続く。
	rapids := newRapidTable(cands, parentsByChild)
	conts := func(last setplayrepo.MoveCandidate) []setplayrepo.MoveCandidate {
		out := make([]setplayrepo.MoveCandidate, len(heads))
		for i, x := range heads {
			out[i], _ = rapids.next(last, x)
		}
		return out
	}

	// 長さの短い順に幅優先で伸ばす。
	// ★深さ優先ではなく幅優先にするのは、maxChainUnits に到達したときに残るのが
	//   「短い連鎖」＝実用的な単位になるようにするためである(engineSafetyCap 到達時に
	//   少手数の提案が優先して残るのと同じ考え方)。
	//
	// prefix は「連鎖列」と「非末端の chain_cancel_total の累計」の対。
	type prefix struct {
		seq     []setplayrepo.MoveCandidate
		partial int
	}
	frontier := make([]prefix, 0, len(heads))
	for _, h := range heads {
		frontier = append(frontier, prefix{seq: []setplayrepo.MoveCandidate{h}, partial: *h.ChainCancelTotal})
	}
	capped := false
	for length := 2; length <= maxChainLen && len(frontier) > 0 && !capped; length++ {
		next := make([]prefix, 0, len(frontier)*len(heads))
		for _, p := range frontier {
			for _, c := range conts(p.seq[len(p.seq)-1]) {
				if probe.tick() {
					return units, capped
				}
				if c.Total == nil {
					continue // 末端になれない行は単位に組めない
				}
				// ★末端として置けるか(total)と、非末端として更に伸ばせるか(chain_cancel_total)は
				//   別の判定である。chain_cancel_total < total であるため、
				//   「末端にすると予算超過だが、非末端として繋げば収まる」組が原理的にありうる。
				//   2 つを 1 つの continue にまとめると、その経路ごと落としてしまう。
				if p.partial+*c.Total <= budgetCap {
					if len(units) >= maxChainUnits {
						capped = true
						break
					}
					units = append(units, newChainUnit(appendSeq(p.seq, c), p.partial+*c.Total))
				}
				if c.ChainCancelTotal != nil && p.partial+*c.ChainCancelTotal <= budgetCap {
					next = append(next, prefix{seq: appendSeq(p.seq, c), partial: p.partial + *c.ChainCancelTotal})
				}
			}
			if capped {
				break
			}
		}
		frontier = next
	}
	if capped {
		return units, true
	}
	// 長さの上限(maxChainLen)まで伸ばし切った後、もう 1 段伸ばせば末端として置ける
	// 単位が残っていたら、数え切れていない(M47-03)。1 段分を見るだけで、単位は作らない。
	for _, p := range frontier {
		for _, c := range conts(p.seq[len(p.seq)-1]) {
			if c.Total != nil && p.partial+*c.Total <= budgetCap {
				return units, true
			}
		}
	}
	return units, false
}

// appendSeq は seq の複製へ m を足した新しいスライスを返す(共有バッキング配列を作らない)。
func appendSeq(seq []setplayrepo.MoveCandidate, m setplayrepo.MoveCandidate) []setplayrepo.MoveCandidate {
	out := make([]setplayrepo.MoveCandidate, len(seq), len(seq)+1)
	copy(out, seq)
	return append(out, m)
}

// newChainUnit は連鎖列から合成単位を作る。
// 単位 total は呼び出し側が算出済み(Σ非末端の chain_cancel_total ＋ 末端の total)。
func newChainUnit(seq []setplayrepo.MoveCandidate, total int) fillerUnit {
	codes := make([]string, len(seq))
	for i, m := range seq {
		codes[i] = m.Code
	}
	return fillerUnit{Name: strings.Join(codes, ">"), Total: total, Members: seq}
}

// collectTargets は §4.2(拡張)の列挙規則で target 候補を絞り込む。
func collectTargets(cands []setplayrepo.MoveCandidate, parentsByChild map[int64][]int64, params SuggestParams) []setplayrepo.MoveCandidate {
	typeSet := make(map[string]bool, len(params.TargetTypes))
	for _, t := range params.TargetTypes {
		typeSet[t] = true
	}
	// rush_variant の元技カテゴリ解決用に id→category を作る。
	catByID := make(map[int64]string, len(cands))
	for _, m := range cands {
		catByID[m.ID] = m.Category
	}
	out := make([]setplayrepo.MoveCandidate, 0, len(cands))
	for _, m := range cands {
		// 条件 1: startup ≥ 1 かつ active ≥ 1(いずれか NULL の技は除外)。
		if m.Startup == nil || *m.Startup < 1 || m.Active == nil || *m.Active < 1 {
			continue
		}
		if params.TargetMoveID != nil {
			// 明示指定モード: 指定 move のみ。種別/damage/is_derived/is_aerial は適用しない
			// (ユーザーが技を直接選んだため。§4.2 明示指定)。ただし system は常に対象外
			// (§4.6-2・DES-005 §5.6 項目12「category=system は常に対象外」。BE 側でも除外)。
			if m.ID == *params.TargetMoveID && m.Category != model.MoveCategorySystem {
				out = append(out, m)
			}
			continue
		}
		// 自動列挙。
		// M19-LIMITATION-NOTICE (b): この target 列挙規則(種別 type・fastest_unreachable 除外・
		//   単独入力不可の除外・非 through の is_derived 除外〔rush 種別は除外しない〕・
		//   damage>0 既定)を変更したら、ヒント文面(web i18n `setplay.limitations.*`)を
		//   同時に更新すること。
		ttype := moveTargetType(m, catByID)
		if ttype == "" || !typeSet[ttype] {
			continue // 選択された種別以外(system/SA/元 special ラッシュ等の "" を含む)は対象外
		}
		// M19-05: target ゲートを is_derived / is_aerial 依存から
		// basis ＋ 親参照 ＋ air フラグ導出へ切り替えた(M19-DESIGN-07 §3)。
		//
		// is_aerial → fastest_unreachable。is_aerial の実セマンティクスは「空中技か」ではなく
		// 「通常技と特殊技でラッシュ版を作るか否か」であり(D-222)、target 可否の軸ではない。
		// 「単独で最速入力しても地上の相手に当てられない」= スカラー S の target にできない、
		// という事実そのものを持つのが fastest_unreachable である。
		if m.FastestUnreachable {
			continue
		}
		if !isRushTargetType(ttype) {
			// 単独では出せない技は target にできない(親から出す前提の技であるため)。
			if isSoloUnavailable(m, len(parentsByChild[m.ID]) > 0) {
				continue
			}
			// is_derived → basis。through は通し値なので S の意味が確定し、target を解禁する。
			// ★through 以外の派生技(状態変種 standalone・親参照なしで is_derived=1 のもの)は
			//   引き続き除外する——M19-DESIGN-07 §9-4 が「暫定 is_derived ゲート継続」と明記して
			//   いる範囲である。ここを外すと denjin_charge_* / flame_* / mine_set_* 等の
			//   状態前提技が一斉に target へ入る(実測 +172 行)。
			if m.IsDerived && m.StartupBasis != model.MoveStartupBasisThrough {
				continue
			}
		}
		if !params.IncludeZeroDamage && (m.Damage == nil || *m.Damage <= 0) {
			continue // ダメージ 0/NULL(ドライブパリィ等)は既定で target にしない(重ねる概念がない)
		}
		out = append(out, m)
	}
	return out
}

// CandidateProjection は候補集合の射影(測定用)。D-237 の版固定測定手段が使う。
type CandidateProjection struct {
	// FillerCodes は単発 filler 候補の move_code(昇順)。
	FillerCodes []string
	// TargetCodes は target 候補の move_code(昇順)。
	TargetCodes []string
}

// ProjectCandidates は候補集合を射影する。**測定専用**であり提案生成の経路では使わない。
//
// 提案生成が使うのと同じ collectFillerSingles / collectTargets を呼ぶため、規則を二重に持たない
// (SQL で規則を書き写すと、本番の述語を直したときに測定側が静かに古くなる)。
//
// 使い方は internal/infra/migration/canary_setplay_projection_test.go を参照。
func ProjectCandidates(cands []setplayrepo.MoveCandidate, derivations []setplayrepo.MoveDerivation, params SuggestParams) CandidateProjection {
	parentsByChild := make(map[int64][]int64, len(derivations))
	for _, d := range derivations {
		parentsByChild[d.ChildMoveID] = append(parentsByChild[d.ChildMoveID], d.ParentMoveID)
	}
	p := CandidateProjection{FillerCodes: []string{}, TargetCodes: []string{}}
	for _, m := range collectFillerSingles(cands, parentsByChild) {
		p.FillerCodes = append(p.FillerCodes, m.Code)
	}
	for _, m := range collectTargets(cands, parentsByChild, params) {
		p.TargetCodes = append(p.TargetCodes, m.Code)
	}
	sort.Strings(p.FillerCodes)
	sort.Strings(p.TargetCodes)
	return p
}

// parentResolver は through の技に前置する表記用親を返す。親を持たない技には nil を返す。
type parentResolver func(m setplayrepo.MoveCandidate) *setplayrepo.MoveCandidate

// newParentResolver は表記展開(M19-DESIGN-07 §2-1 分類 1)の親解決器を作る。
//
// ★親を前置するのは startup_basis='through' の技だけである。through の startup は
// 「親の入力を起点に測った通し値」であり、親の消費は既にその値へ織り込まれている。
// したがって前置した親ステップの費用は 0(計上ゼロ)——これが費用規則の分類 1 である。
// standalone / unknown の技に親を前置すると二重計上になる。
//
// ★親が複数あるときは move_code 昇順の先頭を採る。費用には影響しない(分類 1 の cost は
// 親が誰でも 0)ため表示上の決定にすぎないが、決定論にするために順序を固定する。
// 意味的に正しい親を選ぶ規則は本サブでは持たない(followup notation-expansion-multiple-parents)。
func newParentResolver(byID map[int64]setplayrepo.MoveCandidate, parentsByChild map[int64][]int64) parentResolver {
	chosen := make(map[int64]setplayrepo.MoveCandidate, len(parentsByChild))
	for childID, parentIDs := range parentsByChild {
		var best *setplayrepo.MoveCandidate
		for _, pid := range parentIDs {
			p, ok := byID[pid]
			if !ok {
				continue // 別キャラ等で解決できない親は前置しない
			}
			if best == nil || p.Code < best.Code {
				pc := p
				best = &pc
			}
		}
		if best != nil {
			chosen[childID] = *best
		}
	}
	return func(m setplayrepo.MoveCandidate) *setplayrepo.MoveCandidate {
		if m.StartupBasis != model.MoveStartupBasisThrough {
			return nil
		}
		p, ok := chosen[m.ID]
		if !ok {
			return nil
		}
		return &p
	}
}

// buildProposal はエンジンの Suggestion を Proposal(DTO 素材)へ変換する。
//
// # 費用規則 4 分類の現れ方(M19-DESIGN-07 §2-1)
//
//   - 分類 1(表記用親 → 0): 前置した親ステップを Counted=false とし S に足さない。
//   - 分類 2(チェーン隣接 → chain_cancel_total): 合成 filler 単位の Total に内包済み。
//   - 分類 3(target → startup): エンジンが S = Σ単位 total + target.Startup で算出済み。
//   - 分類 4(その他 → total): 単発 filler・チェーン末端。単位 Total に内包済み。
//
// ★恒等式 S = Σcost(i) / N = KA + 2 − S / G = N − active は不変である。
// 前置親は計上ゼロなので S は動かない。
//
// ★表記展開は role 非依存(M19-DESIGN-02)。through の技を filler に使う解でも同じ規則で
// 親ステップを前置し、計上は通し total 1 つ(前置親は計上ゼロ)。
func buildProposal(landing int, mode string, tc setplayrepo.MoveCandidate, sg Suggestion, unitByName map[string]fillerUnit, parentOf parentResolver, marks proposalMarks) Proposal {
	p := Proposal{
		S:            sg.TotalFrames,
		N:            sg.HitActiveFrame,
		Landing:      landing,
		TargetActive: *tc.Active,
		Mode:         mode,
		targetCode:   tc.Code,
	}
	for _, f := range sg.Fillers {
		// 合成単位は構成技へ展開する。単発は長さ 1 の単位として同じ経路を通る。
		p.addFillerUnit(unitByName[f.Name], parentOf, marks)
		p.sumFiller += f.TotalFrames
	}
	p.setTarget(tc, parentOf, marks)

	// M19-02-GAP-DEF: gap のとき G = N − active(＝最終 active が起き上がりの G フレーム前＝
	// 完全空振り。恒等式より)。meaty のとき 0。ロールバック点。
	if mode == ModeGap {
		p.G = sg.HitActiveFrame - *tc.Active
	}
	p.assemble()
	return p
}

// rank はグルー側のランキング(§4.4 拡張)。
// meaty(gap=false): 第 1 キー = N 降順(持続が深い順)。
// gap(gap=true): 第 1 キー = **G 昇順**(隙間が小さい順)。G=N−active は target ごとに
//
//	active が異なるため N ではなく Proposal.G を直接比較する(M19-02-GAP-DEF・ロールバック点)。
//
// sort=n: 第 1 キー → target code 昇順 → 手数昇順 → Σtotal 昇順 → Name 昇順。
// sort=target: target code 昇順 → 第 1 キー → 手数昇順 → Σtotal 昇順 → Name 昇順。
// 手数は主キーから外れタイブレークのみ(手数ソートは廃止)。
func rank(props []Proposal, mode SuggestSort, gap bool) {
	// primaryBefore: 第 1 キー(meaty=N 降順 / gap=G 昇順)。equal のとき次キーへ委ねる。
	primaryBefore := func(a, b *Proposal) (before, equal bool) {
		if gap {
			if a.G != b.G {
				return a.G < b.G, false
			}
			return false, true
		}
		if a.N != b.N {
			return a.N > b.N, false
		}
		return false, true
	}
	sort.SliceStable(props, func(i, j int) bool {
		a, b := &props[i], &props[j]
		if mode == SortByTarget {
			if a.targetCode != b.targetCode {
				return a.targetCode < b.targetCode
			}
			if bfr, eq := primaryBefore(a, b); !eq {
				return bfr
			}
		} else { // SortByN(既定・"" 含む)
			if bfr, eq := primaryBefore(a, b); !eq {
				return bfr
			}
			if a.targetCode != b.targetCode {
				return a.targetCode < b.targetCode
			}
		}
		// 手数(タイブレーク)= target 以外のステップ数。M19-05 以降は合成 filler 単位の構成技と
		// 表記用親も 1 ステップとして数える(いずれも実際に入力する手であるため)。
		// 計上ゼロの親も「手」ではあるので除かない。
		fa, fb := len(a.Steps)-1, len(b.Steps)-1
		if fa != fb {
			return fa < fb
		}
		if a.sumFiller != b.sumFiller {
			return a.sumFiller < b.sumFiller
		}
		return a.sortName < b.sortName
	})
}
