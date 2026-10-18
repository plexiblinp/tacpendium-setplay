// 試験用の D1: node:sqlite(Node 22 の組み込み)の上に D1 の使う部分だけを載せる。
// 同じ SQL(schema.sql と votes.ts・visitor.ts の文)を本物の SQLite で流すため、SQL の誤りを試験で見つけられる。
// ★本番の D1 とは別物(D1 の制限・無料枠・遅延は再現しない)。docs/telemetry.md の限界。
import { DatabaseSync, type SQLInputValue } from "node:sqlite";

import type { D1Database, D1PreparedStatement } from "../env";
import schemaSql from "../schema.sql?raw";

class Stmt implements D1PreparedStatement {
  constructor(
    readonly db: DatabaseSync,
    readonly sql: string,
    readonly params: SQLInputValue[] = [],
  ) {}

  bind(...values: unknown[]): D1PreparedStatement {
    return new Stmt(this.db, this.sql, values as SQLInputValue[]);
  }

  async first<T>(): Promise<T | null> {
    return (this.db.prepare(this.sql).get(...this.params) as T | undefined) ?? null;
  }

  async all<T>(): Promise<{ results: T[] }> {
    return { results: this.db.prepare(this.sql).all(...this.params) as T[] };
  }

  async run(): Promise<unknown> {
    return this.db.prepare(this.sql).run(...this.params);
  }
}

export class SqliteD1 implements D1Database {
  readonly db: DatabaseSync;
  /** 流した文の数(試験で「読まなかった」ことを見る)。 */
  statements = 0;

  constructor(file = ":memory:") {
    this.db = new DatabaseSync(file);
    this.db.exec(schemaSql);
  }

  prepare(query: string): D1PreparedStatement {
    this.statements++;
    return new Stmt(this.db, query);
  }

  async batch(statements: D1PreparedStatement[]): Promise<unknown[]> {
    this.db.exec("BEGIN");
    try {
      const out: unknown[] = [];
      for (const s of statements) out.push(await s.run());
      this.db.exec("COMMIT");
      return out;
    } catch (e) {
      this.db.exec("ROLLBACK");
      throw e;
    }
  }
}

/** 試験・E2E 用の Analytics Engine: 書いた点を覚える。 */
export class MemoryAnalytics {
  readonly points: { blobs?: string[]; doubles?: number[]; indexes?: string[] }[] = [];
  writeDataPoint(p: { blobs?: string[]; doubles?: number[]; indexes?: string[] }): void {
    this.points.push(p);
  }
}
