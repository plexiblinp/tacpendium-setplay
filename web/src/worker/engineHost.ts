// Worker の中身(WASM の API を呼ぶ側)。WASM や fetch を外から渡せるようにして、単体で試験できる形にした。
import type { EngineError, SuggestResponse } from "../lib/engineTypes";
import type { SuggestReply, SuggestRequest, WorkerTimings } from "./protocol";

/** cmd/wasm が globalThis.tacpendiumSetplay に置く関数(どれも JSON 文字列を返し、例外を投げない)。 */
export interface WasmApi {
  load(movesJson: string): string;
  suggest(character: string, ka: number, query: string, votes?: string): string;
}

export interface HostDeps {
  /** WASM を起動して API を返す(初回の要求で 1 度だけ呼ぶ)。 */
  boot(): Promise<WasmApi>;
  fetchText(url: string): Promise<string>;
  now(): number;
  /** WASM の線形メモリの大きさ(バイト。起動前は 0)。 */
  memoryBytes(): number;
}

/** 誤りの code(engine の code と重ならない、Worker 側だけのもの)。 */
export const HOST_ERROR_BOOT = "wasm_boot_failed";
export const HOST_ERROR_FETCH = "data_fetch_failed";
export const HOST_ERROR_RESPONSE = "invalid_engine_response";
/** WASM が落ちた(cmd/wasm は panic を recover しない。panic した呼び出しは undefined を返し、以後は例外を投げる)。 */
export const HOST_ERROR_CRASHED = "wasm_crashed";

interface LoadReply {
  character?: string;
  error?: EngineError;
}

interface SuggestJson {
  result?: SuggestResponse;
  error?: EngineError;
}

function messageOf(e: unknown): string {
  return e instanceof Error ? e.message : String(e);
}

/**
 * 要求を 1 つずつ処理する。WASM の起動とキャラの読み込みは初回だけ(読んだ moves の URL を覚える。
 * URL は中身のハッシュ入りなので、同じ URL なら同じ中身)。
 */
export class EngineHost {
  private api: Promise<WasmApi> | null = null;
  /** キャラ → 読み込み済みの moves の URL。 */
  private readonly loaded = new Map<string, string>();

  constructor(private readonly deps: HostDeps) {}

  async handle(req: SuggestRequest): Promise<SuggestReply> {
    const timings: WorkerTimings = { boot: 0, load: 0, compute: 0, wasmMemoryBytes: 0 };
    const fail = (error: EngineError): SuggestReply => ({ type: "error", id: req.id, error, timings });
    // WASM の関数を呼ぶ。例外か文字列でない応答は WASM が落ちたしるしなので、起動と読み込みの記録を捨てて fatal で返す。
    const crashed = (e: unknown): SuggestReply => {
      this.api = null;
      this.loaded.clear();
      return { type: "error", id: req.id, error: { code: HOST_ERROR_CRASHED, message: messageOf(e) }, timings, fatal: true };
    };
    const call = (f: () => unknown): string | SuggestReply => {
      try {
        const v = f();
        return typeof v === "string" ? v : crashed(new Error(`WASM の応答が文字列でない(${typeof v})`));
      } catch (e) {
        return crashed(e);
      }
    };

    let t = this.deps.now();
    let api: WasmApi;
    try {
      if (this.api == null) this.api = this.deps.boot();
      api = await this.api;
    } catch (e) {
      this.api = null;
      return fail({ code: HOST_ERROR_BOOT, message: messageOf(e) });
    }
    timings.boot = this.deps.now() - t;

    t = this.deps.now();
    if (this.loaded.get(req.character) !== req.movesUrl) {
      this.loaded.delete(req.character);
      let movesText: string;
      try {
        movesText = await this.deps.fetchText(req.movesUrl);
      } catch (e) {
        return fail({ code: HOST_ERROR_FETCH, message: messageOf(e) });
      }
      const raw = call(() => api.load(movesText));
      if (typeof raw !== "string") return raw;
      const lr = parseJson<LoadReply>(raw);
      if (lr == null) return fail({ code: HOST_ERROR_RESPONSE, message: "load の応答が JSON でない" });
      if (lr.error) return fail(lr.error);
      if (lr.character !== req.character) {
        return fail({ code: HOST_ERROR_RESPONSE, message: `moves のキャラ ${lr.character ?? "?"} が要求 ${req.character} と違う` });
      }
      this.loaded.set(req.character, req.movesUrl);
    }
    timings.load = this.deps.now() - t;

    t = this.deps.now();
    // 票(S6)があれば 4 番目の引数で渡す。無ければ 3 引数(本家と同じ並び)。
    const raw = call(() => (req.votes ? api.suggest(req.character, req.ka, req.query, req.votes) : api.suggest(req.character, req.ka, req.query)));
    timings.compute = this.deps.now() - t;
    if (typeof raw !== "string") return raw;
    const sr = parseJson<SuggestJson>(raw);
    timings.wasmMemoryBytes = this.deps.memoryBytes();
    if (sr == null) return fail({ code: HOST_ERROR_RESPONSE, message: "suggest の応答が JSON でない" });
    if (sr.error) return fail(sr.error);
    if (sr.result == null) return fail({ code: HOST_ERROR_RESPONSE, message: "suggest の応答に result も error も無い" });
    return { type: "result", id: req.id, result: sr.result, timings };
  }
}

function parseJson<T>(s: string): T | null {
  try {
    return JSON.parse(s) as T;
  } catch {
    return null;
  }
}
