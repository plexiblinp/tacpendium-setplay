/// <reference lib="webworker" />
// エンジンの WASM を動かす Web Worker(メインスレッドを固めない。HANDOVER §5)。
// 中断はメインスレッドが Worker ごと terminate する(cmd/wasm は取り消しを受けない)。
import { WASM_EXEC_PATH, WASM_PATH } from "../lib/constants";
import { CLIENT_ERROR_WORKER } from "../lib/engineClient";
import { EngineHost, type WasmApi } from "./engineHost";
import type { SuggestRequest } from "./protocol";

declare const self: DedicatedWorkerGlobalScope;

/** cmd/wasm の globalName。 */
const WASM_GLOBAL = "tacpendiumSetplay";

interface GoInstance {
  importObject: WebAssembly.Imports;
  run(instance: WebAssembly.Instance): Promise<void>;
}

function assetUrl(path: string): string {
  // ★完全な URL にする。開発サーバ(Vite)は相対・絶対パスの動的 import に ?import を付け、public/ の
  //   ファイルを読ませない。完全な URL はそのまま通る。
  return new URL(`${import.meta.env.BASE_URL}${path}`, self.location.origin).href;
}

/** WASM の線形メモリ(Go の WASM は "mem" で出す)。 */
let memory: WebAssembly.Memory | null = null;

async function boot(): Promise<WasmApi> {
  // Go 付属の wasm_exec.js(globalThis.Go を置く)。WASM と同じ Go で作って並べてある(web/scripts/prepare-assets.sh)。
  await import(/* @vite-ignore */ assetUrl(WASM_EXEC_PATH));
  const GoCtor = (globalThis as unknown as { Go?: new () => GoInstance }).Go;
  if (GoCtor == null) throw new Error("wasm_exec.js が Go を置かなかった");
  const go = new GoCtor();
  const res = await fetch(assetUrl(WASM_PATH));
  if (!res.ok) throw new Error(`${WASM_PATH}: HTTP ${res.status}`);
  const { instance } = await WebAssembly.instantiate(await res.arrayBuffer(), go.importObject);
  const mem = instance.exports.mem;
  memory = mem instanceof WebAssembly.Memory ? mem : null;
  // main は関数を登録した後 select{} で待つので、run の Promise は解決しない(待たない)。
  void go.run(instance);
  const api = (globalThis as unknown as Record<string, WasmApi | undefined>)[WASM_GLOBAL];
  if (api == null) throw new Error(`globalThis.${WASM_GLOBAL} が無い`);
  return api;
}

async function fetchOk(url: string): Promise<Response> {
  const res = await fetch(url);
  if (!res.ok) throw new Error(`${url}: HTTP ${res.status}`);
  return res;
}

const host = new EngineHost({
  boot,
  fetchText: async (url) => (await fetchOk(url)).text(),
  now: () => performance.now(),
  memoryBytes: () => memory?.buffer.byteLength ?? 0,
});

// 要求は 1 つずつ順に処理する(WASM は 1 スレッド。画面は計算中にボタンを押せなくしている)。
let queue: Promise<void> = Promise.resolve();
self.onmessage = (ev: MessageEvent<SuggestRequest>) => {
  const req = ev.data;
  queue = queue
    .then(async () => {
      self.postMessage(await host.handle(req));
    })
    // handle は誤りを返事にして返すが、万一 reject しても返事をして鎖を切らない(固まらない)。
    .catch((e: unknown) => {
      const message = e instanceof Error ? e.message : String(e);
      const timings = { boot: 0, load: 0, compute: 0, wasmMemoryBytes: 0 };
      self.postMessage({ type: "error", id: req.id, error: { code: CLIENT_ERROR_WORKER, message }, timings, fatal: true });
    });
};
