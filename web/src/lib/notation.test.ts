import { describe, expect, it } from "vitest";

import type { Move } from "./data";
import type { Item } from "./engineTypes";
import { copyText, defaultNotation, indexMoves, itemKey, recipeText } from "./notation";

const mv = (code: string, nameJa: string, srk: string): Move => ({
  code,
  category: "normal",
  damage: 300,
  onHit: null,
  onBlock: null,
  notation: { nameJa, nameEn: code.toUpperCase(), srk, numeric: `n:${code}` },
});
const moves = indexMoves([mv("a", "弱P", "cr.LP"), mv("b", "波動拳", "236P")]);
const steps = [
  { code: "a", role: "filler" as const, counted: true },
  { code: "b", role: "target" as const, counted: true },
];

describe("表記", () => {
  it("既定は ja だけ日本語技名、ほかは SRK", () => {
    expect(defaultNotation("ja")).toBe("nameJa");
    for (const l of ["en", "zh-Hans", "zh-Hant", "ko"]) expect(defaultNotation(l)).toBe("srk");
  });
  it("選んだ表記で手順を並べる", () => {
    expect(recipeText(steps, moves, "nameJa")).toBe("弱P > 波動拳");
    expect(recipeText(steps, moves, "srk")).toBe("cr.LP > 236P");
    expect(recipeText(steps, moves, "numeric")).toBe("n:a > n:b");
    expect(recipeText(steps, moves, "nameEn")).toBe("A > B");
  });
  it("moves DB に無い code は code のまま", () => {
    expect(recipeText([{ code: "zzz", role: "filler", counted: true }], moves, "srk")).toBe("zzz");
  });
  it("コピーの中身はレシピとフレームの説明", () => {
    expect(copyText("cr.LP > 236P", "Hits on active frame 3")).toBe("cr.LP > 236P (Hits on active frame 3)");
  });
  it("行の鍵は手順・N・安全飛びの種類で決まる", () => {
    const it0: Item = { steps, s: 1, n: 3, landing: 41, targetActive: 5, mode: "meaty", g: 0, remain: 0, recipeKey: "a|b" };
    expect(itemKey(it0, null)).toBe("a-b#3");
    expect(itemKey(it0, "f5")).toBe("f5:a-b#3");
    expect(itemKey({ ...it0, n: 4 }, null)).not.toBe(itemKey(it0, null));
  });
});
