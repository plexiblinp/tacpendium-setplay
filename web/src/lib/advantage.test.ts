import { describe, expect, it } from "vitest";

import { formatAdvantage, meatyAdvantage } from "./advantage";
import type { Move } from "./data";
import type { Item } from "./engineTypes";
import { indexMoves } from "./notation";

const mv = (code: string, category: string, onHit: number | null, onBlock: number | null): Move => ({
  code,
  category,
  damage: 300,
  onHit,
  onBlock,
  notation: { nameJa: code, nameEn: code, srk: code, numeric: code },
});
const moves = indexMoves([
  mv("lp", "normal", 4, -1),
  mv("hp", "normal", 4, -2),
  mv("hado", "special", null, -6),
  mv("dp", "special", null, null),
  mv("throw", "throw", 20, null),
]);
const item = (target: string, n: number, mode: Item["mode"] = "meaty"): Pick<Item, "mode" | "n" | "steps"> => ({
  mode,
  n,
  steps: [
    { code: "lp", role: "filler", counted: true },
    { code: target, role: "target", counted: true },
  ],
});

describe("重ねたときの有利", () => {
  it("持続 1F 目はデータの値のまま", () => {
    expect(meatyAdvantage(item("hp", 1), moves)).toEqual({ block: -2, hit: 4 });
  });
  it("持続 N F目は N − 1 を足す", () => {
    expect(meatyAdvantage(item("hp", 3), moves)).toEqual({ block: 0, hit: 6 });
  });
  it("片方だけ値が無ければ、その側は null(ヒットでダウンする技など)", () => {
    expect(meatyAdvantage(item("hado", 4), moves)).toEqual({ block: -3, hit: null });
  });
  it("両方とも値が無い・投げ・当てる技が無い・知らない技は出さない", () => {
    expect(meatyAdvantage(item("dp", 2), moves)).toBeNull();
    expect(meatyAdvantage(item("throw", 1), moves)).toBeNull();
    expect(meatyAdvantage(item("nope", 1), moves)).toBeNull();
    expect(meatyAdvantage({ mode: "meaty", n: 1, steps: [{ code: "lp", role: "filler", counted: true }] }, moves)).toBeNull();
  });
  it("重ねない提案(gap・filler)には出さない", () => {
    expect(meatyAdvantage(item("hp", 3, "gap"), moves)).toBeNull();
    expect(meatyAdvantage(item("hp", 3, "filler"), moves)).toBeNull();
  });
  it("表示は符号つき、0 は ±0、値が無ければ —", () => {
    expect([3, -2, 0, null].map(formatAdvantage)).toEqual(["+3", "-2", "±0", "—"]);
  });
});
