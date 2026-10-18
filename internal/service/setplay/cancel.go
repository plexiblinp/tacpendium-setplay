package setplay

import "context"

// cancelCheckInterval は、計算の繰り返しの内側で要求の取り消し(ctx)を見る間隔(回)である(M47-03・A2)。
//
// ctx.Err() は mutex を取るため、最内の再帰で毎回呼ぶと計算そのものが遅くなる。
// 1024 回に 1 回だけ見ることで、止まるまでの遅れを探索のごく短い区間に抑えつつ、
// 取り消さない要求の所要時間を変えない(実測は docs/progress/M47-03-completion-report.md §4)。
const cancelCheckInterval = 1024

// cancelProbe は計算の繰り返しの内側で要求の取り消しを見る小さな部品である(M47-03)。
//
// 取り消しを一度観測したら err に保持し、以降の tick は必ず true を返す
// (呼び出し側はそれを見て繰り返しを抜け、最後に err を返す)。
// ゼロ値ではなく newCancelProbe で作ること。
type cancelProbe struct {
	ctx context.Context
	n   int
	err error
}

// newCancelProbe は ctx を見る cancelProbe を返す。ctx が nil なら取り消しを見ない。
func newCancelProbe(ctx context.Context) *cancelProbe {
	return &cancelProbe{ctx: ctx}
}

// tick は繰り返し 1 回ごとに呼ぶ。cancelCheckInterval 回に 1 回 ctx を見て、
// 取り消されていれば(以後ずっと)true を返す。
func (p *cancelProbe) tick() bool {
	if p.err != nil {
		return true
	}
	p.n++
	if p.n < cancelCheckInterval {
		return false
	}
	p.n = 0
	return p.check()
}

// check は間隔に関係なく ctx を今すぐ見る。繰り返しの節目(当てる技 1 つごと等)で使う。
func (p *cancelProbe) check() bool {
	if p.err != nil {
		return true
	}
	if p.ctx == nil {
		return false
	}
	if err := p.ctx.Err(); err != nil {
		p.err = err
		return true
	}
	return false
}
