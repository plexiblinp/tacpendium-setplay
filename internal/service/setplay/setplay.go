// Package setplay は、ダウン中無敵の終わり際に技をぴったり重ねる
// 「セットプレイ(起き攻め)」の提案を、純粋なフレーム計算問題として解く。
//
// 別リポ autopilot-combomgr/projects/setplay-suggestion の setplay パッケージを
// Tacpendium 本体へ移植したもの(M19-01)。移植にあたり、受理条件を「完全一致」から
// 「受理帯(窓方式)」へ変更した。標準ライブラリのみに依存する純ロジックであり、
// 特定ゲームや Tacpendium のデータモデルには依存しない(フレーム値だけを扱う)。
//
// # 確定した意味論(DES-003 §3.4 errata②・M19-01 §1.1)
//
//   - KA(knockdownAdvantage) = ダウン中の無敵時間。相手の起き上がりは KA+1 フレーム目。
//   - S = Σ(filler.total) + target.startup = target の第 1 active の絶対フレーム。
//   - N = KA + 2 − S = 起き上がりに重なる持続フレーム番号。
//   - 成立条件(窓方式) = NMin ≤ N ≤ target.active
//     ⇔ KA + 2 − active ≤ S ≤ KA + 2 − NMin。
//
// # 受理帯方式(エンジン実装。上記と代数的に等価)
//
// 予算 budgetMax = KA + 2 − NMin − target.Startup を空振り全体フレームの合計
// sumF で埋める。remaining = budgetMax − sumF とすると、
//
//	受理: 0 ≤ remaining ≤ target.Active − NMin
//	N    = remaining + NMin      (= KA + 2 − S と恒等)
//	S    = KA + 2 − NMin − remaining
//
// 探索は subset-sum(反復深化・少手数優先・枝刈り・sanitize)で、移植元の構造を温存する。
package setplay

import (
	"context"
	"errors"
	"sort"
)

// Move は空振りでフレームを消費する1つの行動(技、またはステップ・ジャンプ
// 等の固定フレーム移動)。
//
// 【M19-05】グルーは単発技だけでなく「合成 filler 単位」(チェーン列をまとめた疑似 filler)も
// この型で渡すようになった。エンジンから見た意味は変わらない——1 単位が消費するフレーム数を
// 1 つの数として受け取るだけである(エンジンは無改修)。
type Move struct {
	// Name は表示・識別用の文字列。Tacpendium 本体では moves.code を渡す想定。
	// 合成単位のときはグルーが組み立てた合成キー(code を ">" で連結したもの)が入る。
	Name string `json:"name"`
	// TotalFrames は 1 単位が空振りで消費する実フレーム。
	// 単発技なら moves.total、合成単位なら Σ(非末端の chain_cancel_total) ＋ 末端の total。
	// 非正(0 以下)の値は「未設定/NULL」とみなし、候補から黙って除外する。
	TotalFrames int `json:"totalFrames"`
}

// Target は最後に当てたい技。
type Target struct {
	// Name は表示・識別用の文字列。
	Name string `json:"name"`
	// Startup は発生フレーム(正の値であること)。
	Startup int `json:"startup"`
	// Active は持続フレーム数(正の値であること)。受理帯の幅を決める。
	Active int `json:"active"`
}

// Suggestion は1つのセットプレイ提案。Fillers を順に空振りしてから Target を出すと、
// target の第 1 active が起き上がり(KA+1)に対して HitActiveFrame 目で重なる。
type Suggestion struct {
	// Fillers は空振りする技列。全体フレーム昇順の正準順で格納する
	// (フレーム合計は順序に依存しないため、重複組合せを避ける目的)。
	Fillers []Move `json:"fillers"`
	// Target は当てたい技。
	Target Target `json:"target"`
	// TotalFrames は S = Σ Fillers.TotalFrames + Target.Startup
	// (= target の第 1 active の絶対フレーム)。
	//
	// 【破壊的変更・M19-01】移植元では常に KA に等しい値だったが、窓方式では
	// S の意味に変更した。移植元テスト TestExactMatchNoFiller / TestSingleFiller の
	// 期待値はこの変更に合わせて再計算済み(2 件)。
	TotalFrames int `json:"totalFrames"`
	// HitActiveFrame は N(起き上がりに重なる持続フレーム番号)。1 ≤ N ≤ Active。
	HitActiveFrame int `json:"hitActiveFrame"`
}

// Options は探索の挙動を制御する。ゼロ値は「同一技の再利用なし・上限なし・NMin=0」
// となるため、通常は DefaultOptions を起点に調整することを推奨する。
type Options struct {
	// AllowRepeat が true のとき、同じ技を複数回空振りに使える。
	AllowRepeat bool `json:"allowRepeat"`
	// MaxFillers は1提案あたりの空振り技数の上限。0 以下は「上限なし」だが、
	// その場合も合計が budgetMax を超えない範囲で自然に収束する。
	MaxFillers int `json:"maxFillers"`
	// MaxResults は返す提案数の上限。0 以下は「上限なし」。
	// 提案は空振り技数の少ない順に生成されるため、上限到達時は
	// 入力に近い(少手数の)提案が優先的に残る。
	MaxResults int `json:"maxResults"`
	// NMin は最小持続番号(N の下限)。1 未満は 1 に丸める。
	// N_min は絞り込み設定であり、不正入力ではない(NMin > Active は空結果)。
	NMin int `json:"nMin"`
	// Band は受理帯 [Lo, Hi] を N について直接指定する(M19-02 gap モード)。
	//
	// nil のとき(既定・meaty)は受理帯を [NMin, target.Active] とみなし、
	// 一般化前とまったく同じ挙動になる(既存呼び出し・既存テストは無改修)。
	//
	// 非 nil のときは NMin と target.Active を帯には用いず、Lo/Hi を直接使う。
	// 「あえて重ねない(gap)」モードは Lo = active + Gmin・Hi = active + Gmax を渡す(gapBand。G = N − active)。
	// Hi = 0 は gap の正当値(Gmin=1)であるため、0 を「未指定」の代替にはしない
	// (未指定は nil ポインタで表す)。target.Active ≥ 1 の妥当性検査は帯に依らず維持する。
	Band *NBand `json:"band,omitempty"`
}

// NBand は N の受理帯 [Lo, Hi](両端含む)を表す。Options.Band で用いる。
// エンジンは Lo が負の帯も受理する(一般の帯として。gap モードの帯は gapBand が返す [active + Gmin, active + Gmax] で正)。
type NBand struct {
	Lo int `json:"lo"`
	Hi int `json:"hi"`
}

// DefaultOptions は推奨既定値を返す。同一技の再利用を許可し、
// 組み合わせ爆発を抑えるための穏当な上限を設定する。NMin は 1(最速重ねから許可)。
func DefaultOptions() Options {
	return Options{
		AllowRepeat: true,
		MaxFillers:  8,
		MaxResults:  200,
		NMin:        1,
	}
}

// 入力エラー。
var (
	// ErrNegativeAdvantage は knockdownAdvantage が負の場合に返る。
	ErrNegativeAdvantage = errors.New("setplay: knockdown advantage must be non-negative")
	// ErrInvalidTargetStartup は当てたい技の発生が正でない場合に返る。
	ErrInvalidTargetStartup = errors.New("setplay: target startup must be positive")
	// ErrInvalidTargetActive は当てたい技の持続が正でない場合に返る(M19-01 で追加)。
	// グルー層が事前に active ≥ 1 を保証する想定だが、エンジンは防御的に検査する。
	ErrInvalidTargetActive = errors.New("setplay: target active must be positive")
)

// Suggest は knockdownAdvantage・空振り候補 moves・当てたい技 target を受け取り、
// 窓条件 NMin ≤ N ≤ target.Active を満たす提案を返す。
//
// 動作:
//   - 受理帯 0 ≤ remaining ≤ target.Active − NMin を満たす空振り組み合わせを列挙する。
//   - 解が存在しないときは空スライスを返す(エラーではない)。
//   - moves のうち TotalFrames が非正のものは候補から除外する。
//   - NMin < 1 は 1 に丸める。NMin > target.Active は空結果(エラーにしない)。
//   - budgetMax < 0 は空結果(エラーにしない)。
//
// 入力エラー(負の knockdownAdvantage、非正の target.Startup / target.Active)の
// ときのみ error を返す。取り消しを見ない(要求の取り消しで止めたいときは SuggestContext)。
func Suggest(knockdownAdvantage int, moves []Move, target Target, opt Options) ([]Suggestion, error) {
	return SuggestContext(context.Background(), knockdownAdvantage, moves, target, opt)
}

// SuggestContext は Suggest と同じ計算を、ctx の取り消しを見ながら行う(M47-03・A2)。
//
// 探索の再帰の内側で cancelCheckInterval 回に 1 回 ctx を見て、取り消されていれば
// 途中で止まり、ctx.Err() をそのまま返す(errors.Is(err, context.Canceled) が成り立つ)。
// 取り消されない限り、結果は Suggest と同じ行が同じ順で出る。
func SuggestContext(ctx context.Context, knockdownAdvantage int, moves []Move, target Target, opt Options) ([]Suggestion, error) {
	if knockdownAdvantage < 0 {
		return nil, ErrNegativeAdvantage
	}
	if target.Startup <= 0 {
		return nil, ErrInvalidTargetStartup
	}
	if target.Active <= 0 {
		return nil, ErrInvalidTargetActive
	}

	// NMin < 1 は 1 に丸める(絞り込み設定であり不正入力ではない)。
	nMin := opt.NMin
	if nMin < 1 {
		nMin = 1
	}

	// 受理帯 [nLo, nHi] を決める。Band 未指定(nil)なら meaty で [NMin, Active]
	// に写像し、一般化前と完全一致させる。指定時(gap)は Lo/Hi を直接使う。
	nLo, nHi := nMin, target.Active
	if opt.Band != nil {
		nLo, nHi = opt.Band.Lo, opt.Band.Hi
	}

	results := []Suggestion{}

	// 受理帯の幅 band = nHi − nLo。nLo > nHi なら band < 0 で解なし。
	band := nHi - nLo
	if band < 0 {
		return results, nil
	}

	// 予算 budgetMax = KA + 2 − nLo − 発生。これを空振り全体フレーム合計 sumF で埋め、
	// 残り remaining が [0, band] に収まれば受理。nLo が負の帯では budgetMax が
	// 増えるが、枝刈り・恒等式は remaining 基準のため不変(下限側・上限側とも列挙される)。
	budgetMax := knockdownAdvantage + 2 - nLo - target.Startup
	if budgetMax < 0 {
		// 起き上がり窓より手前にしか当たらない。解なし。
		return results, nil
	}

	candidates := sanitize(moves)

	// 反復深化: 空振り技数 k を 0 から増やしながら探索する。
	// これにより「少手数の提案を先に」生成でき、MaxResults 到達時も
	// 実用的な提案が優先して残る。終端は budgetMax と最小全体フレームで決まる。
	maxDepth := computeMaxDepth(budgetMax, candidates, opt.MaxFillers)

	probe := newCancelProbe(ctx)
	for k := 0; k <= maxDepth; k++ {
		if probe.check() {
			break
		}
		cont := searchDepth(candidates, 0, budgetMax, k, nil, knockdownAdvantage, target, nLo, band, opt, &results, probe)
		if !cont {
			break
		}
	}
	if probe.err != nil {
		return nil, probe.err
	}

	return results, nil
}

// sanitize は非正フレームの技を除去し、完全重複(同一 Name かつ同一 TotalFrames)を
// 1 件にまとめ、全体フレーム昇順(同値は Name 昇順)に整列した候補を返す。
// 昇順整列は探索の枝刈り(残予算を超えた時点で打ち切り)と
// 提案内 Fillers の正準順を成立させるために必須。
func sanitize(moves []Move) []Move {
	seen := make(map[Move]struct{}, len(moves))
	out := make([]Move, 0, len(moves))
	for _, m := range moves {
		if m.TotalFrames <= 0 {
			continue // 未設定/NULL とみなし黙って除外
		}
		if _, dup := seen[m]; dup {
			continue
		}
		seen[m] = struct{}{}
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].TotalFrames != out[j].TotalFrames {
			return out[i].TotalFrames < out[j].TotalFrames
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// computeMaxDepth は探索する空振り技数の上限を求める。
// sumF は budgetMax を超えられないため、自然上限 = budgetMax / 最小全体フレーム。
// 受理帯(下限 remaining ≥ 0 は budgetMax 基準)でも最大手数の根拠は budgetMax であり、
// 帯の下限側(sumF が大きい解)は手数が同じか少ないため取りこぼさない。
// 利用者指定の MaxFillers があればその小さい方。
func computeMaxDepth(budgetMax int, candidates []Move, maxFillers int) int {
	if budgetMax == 0 || len(candidates) == 0 {
		return 0
	}
	minTotal := candidates[0].TotalFrames // 昇順整列済み
	natural := budgetMax / minTotal
	if maxFillers > 0 && maxFillers < natural {
		return maxFillers
	}
	return natural
}

// searchDepth はちょうど depth 手の空振りで remaining を受理帯 [0, band] に収める
// 組み合わせを深さ優先で探索し、見つかるたび results へ追加する。
// 戻り値 false は「MaxResults 到達、または要求の取り消し(probe.err に保持)につき
// 探索を打ち切るべき」を表す(取り消しは M47-03)。
func searchDepth(candidates []Move, start, remaining, depth int, current []Move, ka int, target Target, nLo, band int, opt Options, results *[]Suggestion, probe *cancelProbe) bool {
	if opt.MaxResults > 0 && len(*results) >= opt.MaxResults {
		return false
	}
	if depth == 0 {
		// 受理帯: 0 ≤ remaining ≤ band(= nHi − nLo)。
		// break 条件により remaining は常に 0 以上に保たれるため、実質は上限判定。
		if remaining >= 0 && remaining <= band {
			// make で複製(空でも nil でなく [] になり、JSON 出力が安定する)。
			fillers := make([]Move, len(current))
			copy(fillers, current)
			// remaining から S と N を直接復元(減算をやり直さない)。
			// nLo が負の帯では N ≤ 0 になり得るが、恒等式どおりで正しい。
			*results = append(*results, Suggestion{
				Fillers:        fillers,
				Target:         target,
				TotalFrames:    ka + 2 - nLo - remaining, // = S
				HitActiveFrame: remaining + nLo,          // = N
			})
		}
		return !(opt.MaxResults > 0 && len(*results) >= opt.MaxResults)
	}
	// 下限枝刈り: 最小全体フレームを depth 回重ねても remaining を超過する
	// (final remaining < 0 = 帯の下限 0 未満)なら無駄。受理帯の下限が 0 のため
	// この枝刈りは帯内の解を切り落とさない。
	// (候補はグローバル昇順整列済み + start は再帰で単調増加のため、
	//  candidates[start] が残り範囲での真の最小となる)
	if start < len(candidates) && remaining < candidates[start].TotalFrames*depth {
		return true
	}
	for i := start; i < len(candidates); i++ {
		if candidates[i].TotalFrames > remaining {
			break // 昇順整列のため、以降はすべて残予算を超える
		}
		// 要求が取り消されたら、探索の途中で止まる(M47-03・A2)。
		if probe.tick() {
			return false
		}
		current = append(current, candidates[i])
		next := i
		if !opt.AllowRepeat {
			next = i + 1
		}
		cont := searchDepth(candidates, next, remaining-candidates[i].TotalFrames, depth-1, current, ka, target, nLo, band, opt, results, probe)
		current = current[:len(current)-1]
		if !cont {
			return false
		}
	}
	return true
}
