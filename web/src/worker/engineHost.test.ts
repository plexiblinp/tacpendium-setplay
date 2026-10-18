import { describe, expect, it, vi } from "vitest";

import { EngineHost, HOST_ERROR_BOOT, HOST_ERROR_CRASHED, HOST_ERROR_FETCH, HOST_ERROR_RESPONSE, type HostDeps, type WasmApi } from "./engineHost";
import type { SuggestRequest } from "./protocol";

const RESULT = { items: [], truncated: false, totalFound: 0 };

function fakeApi(over: Partial<WasmApi> = {}): WasmApi {
  return {
    load: vi.fn((json: string) => JSON.stringify({ character: (JSON.parse(json) as { code: string }).code })),
    suggest: vi.fn(() => JSON.stringify({ result: RESULT })),
    ...over,
  };
}

function deps(api: WasmApi, over: Partial<HostDeps> = {}): HostDeps {
  return {
    boot: vi.fn(async () => api),
    fetchText: vi.fn(async (url: string) => JSON.stringify({ code: url.includes("ken") ? "ken" : "ryu" })),
    now: () => 0,
    memoryBytes: () => 1024,
    ...over,
  };
}

const req = (over: Partial<SuggestRequest> = {}): SuggestRequest => ({
  type: "suggest",
  id: 1,
  character: "ryu",
  movesUrl: "https://x/data/moves/ryu.1.json",
  ka: 40,
  query: "mode=meaty",
  ...over,
});

describe("EngineHost", () => {
  it("初回は起動・moves を読み、suggest の結果を返す", async () => {
    const api = fakeApi();
    const d = deps(api);
    const r = await new EngineHost(d).handle(req());
    expect(r).toMatchObject({ type: "result", id: 1, result: RESULT, timings: { wasmMemoryBytes: 1024 } });
    expect(api.load).toHaveBeenCalledTimes(1);
    expect(api.suggest).toHaveBeenCalledWith("ryu", 40, "mode=meaty");
  });
  it("同じキャラ・同じ URL なら 2 回目は読み直さない。起動も 1 回", async () => {
    const api = fakeApi();
    const d = deps(api);
    const h = new EngineHost(d);
    await h.handle(req());
    await h.handle(req({ id: 2, ka: 41 }));
    expect(d.boot).toHaveBeenCalledTimes(1);
    expect(api.load).toHaveBeenCalledTimes(1);
    expect(api.suggest).toHaveBeenCalledTimes(2);
  });
  it("URL が変われば(データが更新された)読み直す", async () => {
    const api = fakeApi();
    const h = new EngineHost(deps(api));
    await h.handle(req());
    await h.handle(req({ movesUrl: "https://x/data/moves/ryu.2.json" }));
    expect(api.load).toHaveBeenCalledTimes(2);
  });
  it("エンジンの誤りはそのまま返す", async () => {
    const api = fakeApi({ suggest: () => JSON.stringify({ error: { code: "invalid_query_parameter", message: "m", param: "mode" } }) });
    expect(await new EngineHost(deps(api)).handle(req())).toMatchObject({ type: "error", error: { code: "invalid_query_parameter", param: "mode" } });
  });
  it("load の誤り・違うキャラの moves は誤り(読み込み済みにしない)", async () => {
    const bad = fakeApi({ load: () => JSON.stringify({ error: { code: "invalid_moves_data", message: "m" } }) });
    expect(await new EngineHost(deps(bad)).handle(req())).toMatchObject({ error: { code: "invalid_moves_data" } });
    const api = fakeApi();
    const h = new EngineHost(deps(api));
    expect(await h.handle(req({ movesUrl: "https://x/data/moves/ken.1.json" }))).toMatchObject({ error: { code: HOST_ERROR_RESPONSE } });
    expect(api.suggest).not.toHaveBeenCalled();
  });
  it("取得の失敗・起動の失敗・JSON でない応答", async () => {
    const api = fakeApi();
    const fetchFail = deps(api, { fetchText: async () => Promise.reject(new Error("404")) });
    expect(await new EngineHost(fetchFail).handle(req())).toMatchObject({ error: { code: HOST_ERROR_FETCH, message: "404" } });
    const bootFail = deps(api, { boot: vi.fn(async () => Promise.reject(new Error("no wasm"))) });
    const h = new EngineHost(bootFail);
    expect(await h.handle(req())).toMatchObject({ error: { code: HOST_ERROR_BOOT } });
    // 起動の失敗は覚えず、次の要求でもう一度起動する。
    await h.handle(req());
    expect(bootFail.boot).toHaveBeenCalledTimes(2);
    const junk = fakeApi({ suggest: () => "not json" });
    expect(await new EngineHost(deps(junk)).handle(req())).toMatchObject({ error: { code: HOST_ERROR_RESPONSE } });
  });
  it("WASM が落ちたら(undefined を返す・例外を投げる)fatal で返し、次の要求で起動し直して読み直す", async () => {
    let calls = 0;
    const api = fakeApi({
      suggest: vi.fn(() => {
        calls++;
        if (calls === 1) return undefined as unknown as string; // panic した呼び出し(cmd/wasm は recover しない)
        if (calls === 2) throw new Error("Go program has already exited");
        return JSON.stringify({ result: RESULT });
      }),
    });
    const d = deps(api);
    const h = new EngineHost(d);
    expect(await h.handle(req())).toMatchObject({ type: "error", fatal: true, error: { code: HOST_ERROR_CRASHED } });
    expect(await h.handle(req({ id: 2 }))).toMatchObject({ type: "error", fatal: true, error: { code: HOST_ERROR_CRASHED, message: "Go program has already exited" } });
    expect(await h.handle(req({ id: 3 }))).toMatchObject({ type: "result", id: 3 });
    expect(d.boot).toHaveBeenCalledTimes(3);
    expect(api.load).toHaveBeenCalledTimes(3);
  });
  it("load で落ちても fatal", async () => {
    const api = fakeApi({ load: () => { throw new Error("boom"); } });
    expect(await new EngineHost(deps(api)).handle(req())).toMatchObject({ fatal: true, error: { code: HOST_ERROR_CRASHED } });
  });
});
