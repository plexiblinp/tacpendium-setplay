import { describe, expect, it } from "vitest";

import { fetchVotes, postVote, scoresForEngine } from "./votes";

const BASE = "https://example.test/";
const ok = (body: unknown) => async () => new Response(JSON.stringify(body), { status: 200 });

describe("fetchVotes", () => {
  it("票があれば ok、空なら none", async () => {
    expect(await fetchVotes(ok({ votes: { "a|t": [2, 1] } }), BASE, "ryu", 40)).toEqual({ status: "ok", counts: { "a|t": [2, 1] } });
    expect(await fetchVotes(ok({ votes: {} }), BASE, "ryu", 40)).toEqual({ status: "none", counts: {} });
  });
  it("誤り・形の違い・例外・時間切れは unavailable(票なしで計算する)", async () => {
    const un = { status: "unavailable", counts: {} };
    expect(await fetchVotes(async () => new Response("", { status: 503 }), BASE, "ryu", 40)).toEqual(un);
    expect(await fetchVotes(ok({ votes: { "bad key": [1, 0] } }), BASE, "ryu", 40)).toEqual(un);
    expect(await fetchVotes(ok({ votes: { "a|t": [-1, 0] } }), BASE, "ryu", 40)).toEqual(un);
    expect(await fetchVotes(async () => Promise.reject(new Error("net")), BASE, "ryu", 40)).toEqual(un);
    const hang = (_u: string, init?: RequestInit) =>
      new Promise<Response>((_r, reject) => init?.signal?.addEventListener("abort", () => reject(new Error("aborted"))));
    expect(await fetchVotes(hang, BASE, "ryu", 40, 10)).toEqual(un);
  });
});

describe("scoresForEngine", () => {
  it("good − bad。0 は渡さない", () => {
    expect(scoresForEngine({ a: [3, 1], b: [1, 1], c: [0, 2] })).toEqual({ a: 2, c: -2 });
  });
});

describe("postVote", () => {
  it("成功は新しい数、誤り・形の違いは null", async () => {
    expect(await postVote(ok({ good: 1, bad: 0, value: 1 }), BASE, "ryu", 40, "a|t", 1)).toEqual({ good: 1, bad: 0, value: 1 });
    expect(await postVote(async () => new Response("", { status: 429 }), BASE, "ryu", 40, "a|t", 1)).toBeNull();
    expect(await postVote(ok({ nope: 1 }), BASE, "ryu", 40, "a|t", 1)).toBeNull();
    // 受け口が返した value を使う(送った値ではなく。再レビュー低-2)。
    expect(await postVote(ok({ good: 0, bad: 0, value: 0 }), BASE, "ryu", 40, "a|t", 1)).toEqual({ good: 0, bad: 0, value: 0 });
    expect(await postVote(ok({ good: 1, bad: 0, value: 2 }), BASE, "ryu", 40, "a|t", 1)).toBeNull();
  });
});
