import { describe, expect, it } from "vitest";

import { CLIENT_ERROR_WORKER, EngineClient, type WorkerLike } from "./engineClient";
import type { SuggestReply, SuggestRequest } from "../worker/protocol";

class FakeWorker implements WorkerLike {
  onmessage: ((ev: MessageEvent<SuggestReply>) => void) | null = null;
  onerror: ((ev: ErrorEvent) => void) | null = null;
  sent: SuggestRequest[] = [];
  terminated = false;
  postMessage(msg: SuggestRequest) {
    this.sent.push(msg);
  }
  terminate() {
    this.terminated = true;
  }
  reply(r: SuggestReply) {
    this.onmessage?.({ data: r } as MessageEvent<SuggestReply>);
  }
}

const REQ = { character: "ryu", movesUrl: "m", ka: 40, query: "" };
const TIMINGS = { boot: 0, load: 0, compute: 5, wasmMemoryBytes: 0 };

function setup(memoryReleaseBytes?: number) {
  const workers: FakeWorker[] = [];
  let t = 0;
  const client = new EngineClient(
    () => {
      const w = new FakeWorker();
      workers.push(w);
      return w;
    },
    () => (t += 10),
    memoryReleaseBytes,
  );
  return { client, workers };
}

describe("EngineClient", () => {
  it("結果を返し、Worker を使い回す", async () => {
    const { client, workers } = setup();
    const p = client.suggest(REQ);
    expect(client.busy).toBe(true);
    const id = workers[0]!.sent[0]!.id;
    workers[0]!.reply({ type: "result", id, result: { items: [], truncated: false, totalFound: 0 }, timings: TIMINGS });
    expect(await p).toMatchObject({ kind: "result", wallMs: 10 });
    expect(client.busy).toBe(false);
    const p2 = client.suggest(REQ);
    workers[0]!.reply({ type: "error", id: workers[0]!.sent[1]!.id, error: { code: "x", message: "m" }, timings: TIMINGS });
    expect(await p2).toMatchObject({ kind: "error", error: { code: "x" } });
    expect(workers).toHaveLength(1);
  });
  it("中断は Worker ごと止め、次の計算で作り直す", async () => {
    const { client, workers } = setup();
    const p = client.suggest(REQ);
    client.cancel();
    expect(await p).toEqual({ kind: "canceled" });
    expect(workers[0]!.terminated).toBe(true);
    void client.suggest(REQ);
    expect(workers).toHaveLength(2);
  });
  it("計算中に次を頼むと前は canceled", async () => {
    const { client, workers } = setup();
    const p1 = client.suggest(REQ);
    const p2 = client.suggest({ ...REQ, ka: 41 });
    expect(await p1).toEqual({ kind: "canceled" });
    const w = workers[1]!;
    w.reply({ type: "result", id: w.sent[0]!.id, result: { items: [], truncated: false, totalFound: 0 }, timings: TIMINGS });
    expect(await p2).toMatchObject({ kind: "result" });
  });
  it("中断の後に届いた古い返事は捨てる", async () => {
    const { client, workers } = setup();
    const p1 = client.suggest(REQ);
    const old = workers[0]!;
    const oldId = old.sent[0]!.id;
    client.cancel();
    await p1;
    const p2 = client.suggest(REQ);
    old.reply({ type: "result", id: oldId, result: { items: [], truncated: true, totalFound: 9 }, timings: TIMINGS });
    expect(client.busy).toBe(true);
    const w = workers[1]!;
    w.reply({ type: "result", id: w.sent[0]!.id, result: { items: [], truncated: false, totalFound: 0 }, timings: TIMINGS });
    expect(await p2).toMatchObject({ result: { totalFound: 0 } });
  });
  it("Worker が落ちたら誤りにし、次は作り直す", async () => {
    const { client, workers } = setup();
    const p = client.suggest(REQ);
    workers[0]!.onerror?.({ message: "out of memory", preventDefault() {} } as ErrorEvent);
    expect(await p).toMatchObject({ kind: "error", error: { code: CLIENT_ERROR_WORKER, message: "out of memory" } });
    expect(workers[0]!.terminated).toBe(true);
    void client.suggest(REQ);
    expect(workers).toHaveLength(2);
  });
  it("計算の後に WASM のメモリが上限を超えていたら Worker を捨てる(結果は返す)", async () => {
    const { client, workers } = setup(100);
    const reply = (w: FakeWorker, mem: number) =>
      w.reply({ type: "result", id: w.sent[w.sent.length - 1]!.id, result: { items: [], truncated: false, totalFound: 0 }, timings: { ...TIMINGS, wasmMemoryBytes: mem } });
    const p1 = client.suggest(REQ);
    reply(workers[0]!, 100); // ちょうどは捨てない
    expect(await p1).toMatchObject({ kind: "result" });
    expect(workers[0]!.terminated).toBe(false);
    const p2 = client.suggest(REQ);
    reply(workers[0]!, 101);
    expect(await p2).toMatchObject({ kind: "result" });
    expect(workers[0]!.terminated).toBe(true);
    void client.suggest(REQ);
    expect(workers).toHaveLength(2);
  });
  it("fatal の誤り(WASM が落ちた)なら Worker を捨て、次は作り直す。ふつうの誤りでは捨てない", async () => {
    const { client, workers } = setup();
    const p1 = client.suggest(REQ);
    workers[0]!.reply({ type: "error", id: workers[0]!.sent[0]!.id, error: { code: "invalid_query_parameter", message: "m" }, timings: TIMINGS });
    await p1;
    expect(workers[0]!.terminated).toBe(false);
    const p2 = client.suggest(REQ);
    workers[0]!.reply({ type: "error", id: workers[0]!.sent[1]!.id, error: { code: "wasm_crashed", message: "m" }, timings: TIMINGS, fatal: true });
    expect(await p2).toMatchObject({ kind: "error", error: { code: "wasm_crashed" } });
    expect(workers[0]!.terminated).toBe(true);
    void client.suggest(REQ);
    expect(workers).toHaveLength(2);
  });
});
