// KA(ダウン後の有利フレーム)の測り方の図。開発者の下書き(2026-11-07)を SVG に起こした模式図。
// ゲームの画面は写さない(形だけを借りた棒人間と枠)。言語で変わる文字は labels で受け、数字と技の表記(テンキー)は全言語で同じ。
import { KA_FIGURE_EXAMPLE } from "../lib/constants";

export interface KaFigureLabels {
  /** 上の段(自分)のフレームメーターの見出し。 */
  self: string;
  /** 下の段(相手)のフレームメーターの見出し。 */
  opponent: string;
  /** 入れる数字を指す吹き出し。 */
  here: string;
  /** 図の代わりの文(読み上げ用)。 */
  alt: string;
}

const W = 800;
const H = 480;
/** フレームメーターの 1 マスの幅。 */
const CELL = 20;
const METER_X = 20;
const METER_W = 760;
const DRIVE_STOCKS = 6;

function StickFigure({ x, headband }: { x: number; headband: boolean }) {
  const headY = 150;
  return (
    <g className="ka-figure-line" fill="none" strokeWidth={4} strokeLinecap="round" strokeLinejoin="round">
      <circle cx={x} cy={headY} r={30} />
      {headband && (
        <>
          <line x1={x - 29} y1={headY - 10} x2={x + 29} y2={headY - 10} />
          <polyline points={`${x - 20},${headY - 26} ${x - 14},${headY - 46} ${x - 6},${headY - 28} ${x},${headY - 50} ${x + 6},${headY - 28} ${x + 14},${headY - 46} ${x + 20},${headY - 26}`} />
        </>
      )}
      <line x1={x} y1={headY + 30} x2={x} y2={headY + 95} />
      <line x1={x} y1={headY + 55} x2={x - 50} y2={headY + 20} />
      <line x1={x} y1={headY + 55} x2={x + 50} y2={headY + 20} />
      <line x1={x} y1={headY + 95} x2={x - 35} y2={headY + 160} />
      <line x1={x} y1={headY + 95} x2={x + 35} y2={headY + 160} />
    </g>
  );
}

function Meter({ y }: { y: number }) {
  const ticks = [];
  for (let x = METER_X + CELL; x < METER_X + METER_W; x += CELL) ticks.push(<line key={x} x1={x} y1={y} x2={x} y2={y + 24} />);
  return (
    <g className="ka-figure-line" fill="none" strokeWidth={2}>
      <rect x={METER_X} y={y} width={METER_W} height={24} strokeWidth={3} />
      {ticks}
    </g>
  );
}

function DriveStocks({ x0, dir }: { x0: number; dir: 1 | -1 }) {
  const cells = [];
  for (let i = 0; i < DRIVE_STOCKS; i++) {
    const x = x0 + dir * i * 42;
    cells.push(<polygon key={i} points={`${x},${70} ${x + 30},${70} ${x + 36},${84} ${x + 6},${84}`} />);
  }
  return (
    <g className="ka-figure-line" fill="none" strokeWidth={2.5}>
      {cells}
    </g>
  );
}

/** KA の測り方の図(トレーニングモードの画面の模式図)。 */
export function KaFigure({ labels }: { labels: KaFigureLabels }) {
  const ka = `${KA_FIGURE_EXAMPLE.ka}F`;
  return (
    <svg className="ka-figure" viewBox={`0 0 ${W} ${H}`} role="img" aria-label={labels.alt} data-testid="ka-figure">
      {/* 上: 体力・タイマー・ドライブゲージ */}
      <g className="ka-figure-line" fill="none" strokeWidth={3}>
        <rect x={12} y={14} width={52} height={40} rx={4} />
        <rect x={74} y={20} width={290} height={28} />
        <rect x={372} y={12} width={56} height={44} rx={10} />
        <rect x={436} y={20} width={290} height={28} />
        <rect x={736} y={14} width={52} height={40} rx={4} />
      </g>
      <text x={400} y={44} textAnchor="middle" className="ka-figure-text" fontSize={24} fontWeight={700}>
        99
      </text>
      <DriveStocks x0={110} dir={1} />
      <DriveStocks x0={654} dir={-1} />

      {/* 中: 自分(はちまき)と相手 */}
      <StickFigure x={230} headband />
      <StickFigure x={570} headband={false} />

      {/* 下: フレームメーター(上の段が自分、下の段が相手) */}
      <text x={METER_X} y={352} className="ka-figure-text" fontSize={24}>
        {labels.self}
      </text>
      <text x={250} y={353} className="ka-figure-plus" fontSize={30} fontWeight={700} data-testid="ka-figure-value">
        {ka}
      </text>
      <ellipse cx={278} cy={343} rx={52} ry={24} className="ka-figure-mark" fill="none" strokeWidth={4} />
      <line x1={328} y1={336} x2={372} y2={318} className="ka-figure-mark" strokeWidth={4} strokeLinecap="round" />
      <text x={378} y={326} className="ka-figure-mark-text" fontSize={30} fontWeight={700}>
        {labels.here}
      </text>
      <Meter y={362} />
      <Meter y={394} />
      <text x={METER_X} y={446} className="ka-figure-text" fontSize={24}>
        {labels.opponent}
      </text>
      <text x={250} y={448} className="ka-figure-minus" fontSize={30} fontWeight={700}>
        {`-${ka}`}
      </text>

      {/* 下の端: SA ゲージ */}
      <g className="ka-figure-line" fill="none" strokeWidth={3}>
        <rect x={40} y={456} width={250} height={14} />
        <rect x={510} y={456} width={250} height={14} />
      </g>
      <text x={14} y={472} className="ka-figure-text" fontSize={22} fontWeight={700}>
        3
      </text>
      <text x={772} y={472} className="ka-figure-text" fontSize={22} fontWeight={700}>
        3
      </text>
    </svg>
  );
}
