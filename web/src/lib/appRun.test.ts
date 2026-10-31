import { describe, expect, it } from "vitest";

import type { MyVote } from "../components/Results";
import { forgetVoteCounts, isPlainLeftClick, myVoteKey, NO_RESULT_METRICS, resultMetrics, withSharedSafeJump } from "./appRun";
import { defaultConditions } from "./conditions";
import type { SuggestResponse } from "./engineTypes";

const vote = (good: number | null, bad: number | null): MyVote => ({ good, bad, value: 1, state: "idle" });

describe("自分の票の表", () => {
  it("鍵はキャラ|KA|レシピ", () => {
    expect(myVoteKey("ken", 30, "r1")).toBe("ken|30|r1");
  });
  it("そのキャラ・KA の数だけを忘れ、押した状態と、ほかのキャラ・KA は残す", () => {
    const m = { [myVoteKey("ken", 30, "a")]: vote(3, 1), [myVoteKey("ken", 3, "a")]: vote(5, 0), [myVoteKey("ryu", 30, "a")]: vote(2, 2) };
    const next = forgetVoteCounts(m, "ken", 30);
    expect(next[myVoteKey("ken", 30, "a")]).toEqual({ good: null, bad: null, value: 1, state: "idle" });
    expect(next[myVoteKey("ken", 3, "a")]).toEqual(vote(5, 0));
    expect(next[myVoteKey("ryu", 30, "a")]).toEqual(vote(2, 2));
    expect(m[myVoteKey("ken", 30, "a")]).toEqual(vote(3, 1)); // 元の表は変えない
    // 逆向き: KA 3 を忘れても、KA 30 は残す(鍵の区切りまで含めて前方一致を取る)。
    expect(forgetVoteCounts(m, "ken", 3)[myVoteKey("ken", 30, "a")]).toEqual(vote(3, 1));
  });
});

describe("計測に載せる結果の数", () => {
  it("結果からは件数・行数・打ち切りを取る。結果の無い実行は null", () => {
    const result = { totalFound: 12, items: [{}, {}], truncated: true } as unknown as SuggestResponse;
    expect(resultMetrics(result)).toEqual({ totalFound: 12, items: 2, truncated: true });
    expect(NO_RESULT_METRICS).toEqual({ totalFound: null, items: null, truncated: null });
  });
});

describe("共有 URL の安全飛び", () => {
  const table = { true: 40, f5: 41, f6: 42 };
  it("種類から値を引き直す(URL の remain は信じない)", () => {
    const c = { ...defaultConditions("ken", 30), safeJump: "f5" as const, remain: 99 };
    expect(withSharedSafeJump(c, table)).toMatchObject({ safeJump: "f5", remain: 41 });
    expect(c.remain).toBe(99); // 元の条件は変えない
  });
  it("値の無いキャラでは安全飛びを外す。安全飛びの無い条件はそのまま", () => {
    const c = { ...defaultConditions("ken", 30), safeJump: "true" as const, remain: 7 };
    expect(withSharedSafeJump(c, null)).toMatchObject({ safeJump: null, remain: 7 });
    expect(withSharedSafeJump(c, undefined)).toMatchObject({ safeJump: null, remain: 7 });
    const plain = defaultConditions("ken", 30);
    expect(withSharedSafeJump(plain, table)).toBe(plain);
  });
});

describe("押し方", () => {
  const click = { button: 0, metaKey: false, ctrlKey: false, shiftKey: false, altKey: false };
  it("修飾キーなしの左クリックだけ", () => {
    expect(isPlainLeftClick(click)).toBe(true);
    expect(isPlainLeftClick({ ...click, button: 1 })).toBe(false);
    for (const k of ["metaKey", "ctrlKey", "shiftKey", "altKey"] as const) expect(isPlainLeftClick({ ...click, [k]: true })).toBe(false);
  });
});
