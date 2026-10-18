// 画面(メインスレッド)と Worker の間のメッセージ。
import type { EngineError, SuggestResponse } from "../lib/engineTypes";

export interface SuggestRequest {
  type: "suggest";
  id: number;
  character: string;
  /** moves DB の URL(manifest の moves を絶対 URL にしたもの)。 */
  movesUrl: string;
  ka: number;
  query: string;
  /** 票(S6・D17)の JSON({"<recipeKey>": good − bad})。無ければ票なし(本家と同じ並び)。 */
  votes?: string;
}

/** Worker 側の所要(ミリ秒)。boot は WASM の起動(初回だけ)、load は moves の取得と読み込み(キャラの初回だけ)。 */
export interface WorkerTimings {
  boot: number;
  load: number;
  compute: number;
  /** 計算の後の WASM の線形メモリの大きさ(バイト)。Go は広げたメモリを返さないので、これまでの最大に当たる。 */
  wasmMemoryBytes: number;
}

export type SuggestReply =
  | { type: "result"; id: number; result: SuggestResponse; timings: WorkerTimings }
  /** fatal: WASM が落ちた(panic・Go の終了)。この Worker は使えないので、受け手は捨てて作り直す。 */
  | { type: "error"; id: number; error: EngineError; timings: WorkerTimings; fatal?: boolean };
