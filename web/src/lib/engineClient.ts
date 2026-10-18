// メインスレッド側の Worker の窓口。1 度に 1 つの計算だけを持ち、中断は Worker ごと止めて作り直す。
import { WASM_MEMORY_RELEASE_BYTES } from "./constants";
import type { EngineError, SuggestResponse } from "./engineTypes";
import type { SuggestReply, SuggestRequest, WorkerTimings } from "../worker/protocol";

export type SuggestOutcome =
  | { kind: "result"; result: SuggestResponse; timings: WorkerTimings; wallMs: number }
  | { kind: "error"; error: EngineError; wallMs: number }
  | { kind: "canceled" };

/** Worker の誤り(WASM が落ちた等)で返す code。 */
export const CLIENT_ERROR_WORKER = "worker_failed";

/** Worker のうち、ここで使う部分(試験で差し替える)。 */
export interface WorkerLike {
  onmessage: ((ev: MessageEvent<SuggestReply>) => void) | null;
  onerror: ((ev: ErrorEvent) => void) | null;
  postMessage(msg: SuggestRequest): void;
  terminate(): void;
}

interface Pending {
  id: number;
  started: number;
  resolve: (o: SuggestOutcome) => void;
}

export class EngineClient {
  private worker: WorkerLike | null = null;
  private pending: Pending | null = null;
  private nextId = 1;

  constructor(
    private readonly createWorker: () => WorkerLike,
    private readonly now: () => number = () => performance.now(),
    private readonly memoryReleaseBytes: number = WASM_MEMORY_RELEASE_BYTES,
  ) {}

  /** 計算中か。 */
  get busy(): boolean {
    return this.pending != null;
  }

  /** 計算する。計算中に呼ぶと前の計算を中断してから始める(結果は canceled)。 */
  suggest(req: Omit<SuggestRequest, "type" | "id">): Promise<SuggestOutcome> {
    if (this.pending != null) this.cancel();
    const w = this.ensureWorker();
    const id = this.nextId++;
    return new Promise<SuggestOutcome>((resolve) => {
      this.pending = { id, started: this.now(), resolve };
      w.postMessage({ type: "suggest", id, ...req });
    });
  }

  /** 計算中なら Worker ごと止める(次の計算で作り直す。読み込み済みのキャラはそのとき読み直す)。 */
  cancel(): void {
    const p = this.pending;
    if (p == null) return;
    this.pending = null;
    this.dropWorker();
    p.resolve({ kind: "canceled" });
  }

  dispose(): void {
    this.cancel();
    this.dropWorker();
  }

  private ensureWorker(): WorkerLike {
    if (this.worker != null) return this.worker;
    const w = this.createWorker();
    w.onmessage = (ev) => this.onReply(ev.data);
    w.onerror = (ev) => {
      // Worker 自体が落ちた(読み込みの失敗など)。Worker を捨て、次の計算で作り直す。
      // ★Go の WASM のメモリ不足は Go の終了(panic と同じ経路)になり、Worker 側が wasm_crashed(fatal)で返す(推測。実ブラウザの OOM は未確認)。
      ev.preventDefault?.();
      const p = this.pending;
      this.pending = null;
      this.dropWorker();
      p?.resolve({ kind: "error", error: { code: CLIENT_ERROR_WORKER, message: ev.message || "worker error" }, wallMs: p ? this.now() - p.started : 0 });
    };
    this.worker = w;
    return w;
  }

  private dropWorker(): void {
    if (this.worker == null) return;
    this.worker.onmessage = null;
    this.worker.onerror = null;
    this.worker.terminate();
    this.worker = null;
  }

  private onReply(r: SuggestReply): void {
    const p = this.pending;
    if (p == null || p.id !== r.id) return; // 中断した後に届いた古い返事は捨てる。
    this.pending = null;
    const wallMs = this.now() - p.started;
    // Go の WASM は広げたメモリを返さない。大きくなったら Worker ごと捨てて返す(次の計算で作り直す)。
    if (r.timings.wasmMemoryBytes > this.memoryReleaseBytes) this.dropWorker();
    if (r.type === "result") p.resolve({ kind: "result", result: r.result, timings: r.timings, wallMs });
    else {
      // WASM が落ちた Worker は使えない。捨てて、次の計算で作り直す。
      if (r.fatal) this.dropWorker();
      p.resolve({ kind: "error", error: r.error, wallMs });
    }
  }
}
