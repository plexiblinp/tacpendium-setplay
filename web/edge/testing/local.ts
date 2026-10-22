// 手元・E2E 用に Worker の本物のコード(createHandler)を Node で動かす(scripts/serve.mjs が読む)。
// D1 は node:sqlite、Analytics Engine は書いた点を覚える偽物(testing/sqliteD1.ts)。本番の Cloudflare とは別物。
import { createHandler } from "../index";
import { WindowLimiter } from "../rate";
import { SaltStore } from "../visitor";
import { MemoryAnalytics, SqliteD1 } from "./sqliteD1";

const MINUTE_MS = 60_000;
/** E2E は全部の試験が同じ利用者・同じ回線(同じ IP・UA)なので、回数制限を緩める(本番の値は edge/index.ts・votes.ts。レビュー低-4)。 */
const E2E_PER_MINUTE = 10_000;

export function createLocalApi() {
  const env = {
    DB: new SqliteD1(),
    TELEMETRY: new MemoryAnalytics(),
    ASSETS: { fetch: async () => new Response("not found", { status: 404 }) },
  };
  const handler = createHandler({
    now: () => new Date(),
    salts: new SaltStore(),
    beaconLimiter: new WindowLimiter(E2E_PER_MINUTE, MINUTE_MS),
    anonBeaconLimiter: new WindowLimiter(E2E_PER_MINUTE, MINUTE_MS),
    voteLimiter: new WindowLimiter(E2E_PER_MINUTE, MINUTE_MS),
    votesLimiter: new WindowLimiter(E2E_PER_MINUTE, MINUTE_MS),
  });
  return {
    handle: (req: Request) => handler(req, env),
    /** Analytics Engine に書いた点(E2E が「実行ごとのイベントが見える」を確かめる)。 */
    points: () => env.TELEMETRY.points,
    /** D1 の票の表(E2E が「票が保存された」を確かめる)。 */
    votes: () => env.DB.db.prepare("SELECT char, ka, recipe, good, bad FROM votes ORDER BY recipe").all(),
  };
}
