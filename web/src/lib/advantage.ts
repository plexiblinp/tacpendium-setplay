// 重ねたときの有利フレーム(開発者の要望 2026-10-25)。moves DB の onHit・onBlock は持続 1F 目を当てたときの値
// (docs/data/moves-db.md)なので、持続 N F目を重ねたら N − 1 を足す。
// ★計算上の値。多段技(値は最後のヒット)・持続のどこで当てても有利が変わらない技・飛び道具の距離による差は考えない
//   (開発者決定 2026-10-25。画面の「注意と対策」に書く)。
import type { Move } from "./data";
import type { Item } from "./engineTypes";
import type { MovesByCode } from "./notation";

/** 有利を出さない区分(投げは重ねる打撃ではない)。 */
const NO_ADVANTAGE_CATEGORY: Move["category"] = "throw";

/** 重ねたときの有利。null は「データに値が無い」(ヒットでダウンする技のヒット側など)。 */
export interface MeatyAdvantage {
  block: number | null;
  hit: number | null;
}

/** 重ねる提案(mode=meaty)の当てる技の、持続 N F目を重ねたときの有利。出せない行は null。 */
export function meatyAdvantage(item: Pick<Item, "mode" | "n" | "steps">, moves: MovesByCode): MeatyAdvantage | null {
  if (item.mode !== "meaty") return null;
  const target = item.steps.find((s) => s.role === "target");
  const move = target == null ? undefined : moves.get(target.code);
  if (move == null || move.category === NO_ADVANTAGE_CATEGORY) return null;
  if (move.onBlock == null && move.onHit == null) return null;
  const shift = item.n - 1;
  return {
    block: move.onBlock == null ? null : move.onBlock + shift,
    hit: move.onHit == null ? null : move.onHit + shift,
  };
}

/** 有利の表示(+3・-2・±0。値が無ければ —)。 */
export function formatAdvantage(v: number | null): string {
  if (v == null) return "—";
  if (v === 0) return "±0";
  return v > 0 ? `+${v}` : `${v}`;
}
