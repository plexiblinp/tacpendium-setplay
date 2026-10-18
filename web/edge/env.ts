// Cloudflare Worker の束縛(bindings)の型。使う機能だけを最小限に書く(@cloudflare/workers-types を足さない)。
// 名前は wrangler.toml の binding と同じ。

/** D1(SQLite)。https://developers.cloudflare.com/d1/worker-api/ の一部。 */
export interface D1PreparedStatement {
  bind(...values: unknown[]): D1PreparedStatement;
  first<T = Record<string, unknown>>(): Promise<T | null>;
  all<T = Record<string, unknown>>(): Promise<{ results: T[] }>;
  run(): Promise<unknown>;
}

export interface D1Database {
  prepare(query: string): D1PreparedStatement;
  /** 文を 1 つのトランザクションで順に流す(途中で失敗したら全部戻る)。 */
  batch(statements: D1PreparedStatement[]): Promise<unknown[]>;
}

/** Workers Analytics Engine。1 回の書き込みは blob 20・double 20・index 1 まで(S1 ⑥)。 */
export interface AnalyticsEngineDataPoint {
  blobs?: string[];
  doubles?: number[];
  indexes?: string[];
}

export interface AnalyticsEngineDataset {
  writeDataPoint(point: AnalyticsEngineDataPoint): void;
}

/** 静的アセット(web/dist)。 */
export interface AssetsFetcher {
  fetch(request: Request): Promise<Response>;
}

export interface Env {
  ASSETS: AssetsFetcher;
  /** 票と日替わりソルト(schema.sql)。無ければ投票は 503、計測は利用者のハッシュなし(visitor 空文字)で書く。
   * 誤りを返すとき(無料枠の超過等)も計測はハッシュなし(visitor "unavailable")で書く。 */
  DB?: D1Database;
  /** 実行計測。無ければ書かない(204 は返す。計測が止まっても画面は困らない)。 */
  TELEMETRY?: AnalyticsEngineDataset;
}
