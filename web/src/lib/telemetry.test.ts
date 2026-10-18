import { describe, expect, it, vi } from "vitest";

import { beaconSchema } from "../shared/api";
import { defaultConditions } from "./conditions";
import { buildBeacon, loadConfig, sendRun, type RunRecord } from "./telemetry";

const rec = (over: Partial<RunRecord> = {}): RunRecord => ({
  conditions: defaultConditions("ryu", 40),
  notation: "srk",
  lang: "en",
  outcome: "result",
  errorCode: null,
  totalFound: 10,
  items: 10,
  truncated: false,
  cacheHit: false,
  wallMs: 12.345,
  computeMs: 10,
  votesStatus: "none",
  votesApplied: 0,
  dataVersion: "ffdb69e",
  ...over,
});

describe("buildBeacon", () => {
  it("モードで当てはまらない項目は null(0 と区別する)", () => {
    const meaty = buildBeacon(rec(), 1);
    expect(meaty).toMatchObject({ nMin: 1, nMax: 0, gMin: null, gMax: null, remain: null, safeJump: null, ms: 12.3 });
    const gap = buildBeacon(rec({ conditions: { ...defaultConditions("ryu", 40), mode: "gap" } }), 1);
    expect(gap).toMatchObject({ nMin: null, nMax: null, gMin: 1, gMax: 13, remain: null });
    const filler = buildBeacon(rec({ conditions: { ...defaultConditions("ryu", 40), mode: "filler", remain: 42, safeJump: "f5" } }), 1);
    expect(filler).toMatchObject({ remain: 42, safeJump: "f5", targetTypes: [], targetMove: null });
    for (const b of [meaty, gap, filler]) expect(beaconSchema.safeParse(b).success).toBe(true);
  });
});

describe("sendRun", () => {
  it("抽出率に当たったときだけ送る", () => {
    const send = vi.fn(() => true);
    expect(sendRun(send, "https://x.test/", rec(), 0.5, () => 0.49)).not.toBeNull();
    expect(sendRun(send, "https://x.test/", rec(), 0.5, () => 0.5)).toBeNull();
    expect(sendRun(send, "https://x.test/", rec(), 0, () => 0)).toBeNull();
    expect(send).toHaveBeenCalledTimes(1);
    expect(send.mock.calls[0]).toEqual(["https://x.test/api/beacon", expect.stringContaining('"char":"ryu"')]);
  });
  it("形に合わない記録は送らない・送る口の例外で落ちない", () => {
    const send = vi.fn(() => true);
    expect(sendRun(send, "https://x.test/", rec({ wallMs: -5 }), 1, () => 0)).toBeNull();
    expect(send).not.toHaveBeenCalled();
    expect(sendRun(() => { throw new Error("blocked"); }, "https://x.test/", rec(), 1, () => 0)).toBeNull();
  });
});

describe("loadConfig", () => {
  it("読めれば値、読めなければ全部送る(1)", async () => {
    expect((await loadConfig(async () => new Response(JSON.stringify({ schemaVersion: 1, beaconSampleRate: 0.2 })), "https://x.test/")).beaconSampleRate).toBe(0.2);
    expect((await loadConfig(async () => new Response("", { status: 404 }), "https://x.test/")).beaconSampleRate).toBe(1);
    expect((await loadConfig(async () => new Response(JSON.stringify({ schemaVersion: 1, beaconSampleRate: 3 })), "https://x.test/")).beaconSampleRate).toBe(1);
  });
});
