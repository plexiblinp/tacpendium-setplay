// Cloudflare Worker の入口(S6)。/api/* だけを動くコードで受け、それ以外は静的アセット(web/dist)を返す。
// 提案の計算は利用者のブラウザ(WASM)で行う。ここが止まっても提案は止まらない(HANDOVER §3・§7.5)。
//
//   POST /api/beacon        実行計測 → Analytics Engine(§7.1)
//   POST /api/vote          グッド/バッド → D1(D17)。配信しているキャラ・技の票だけ(catalog.ts)
//   GET  /api/votes?char=&ka=  キャラ・KA の票。配信しているキャラだけ
import {
  API_BEACON_PATH,
  API_BODY_MAX_BYTES,
  API_VOTE_PATH,
  API_VOTES_PATH,
  voteRequestSchema,
  type ApiErrorReply,
} from "../src/shared/api";
import { KA_MAX_SITE, KA_MIN } from "../src/lib/constants";
import { parseBeacon, toDataPoint } from "./beacon";
import { Catalog, CatalogUnavailable, recipeCodes } from "./catalog";
import type { Env } from "./env";
import { WindowLimiter } from "./rate";
import { castVote, listVotes } from "./votes";
import { netHash, SaltStore, utcDay, visitorHash } from "./visitor";

/**
 * beacon の回数制限(1 回線・1 分。鍵は回線のハッシュ＝UA を変えても同じ。S9 のセキュリティ修正 F-01)。
 * 画面は「提案を出す」ごとに 1 回しか送らず、1 人が 1 分に押せるのは多くて十数回。共有 IP(CGNAT)の数人分を見込んで 120。
 */
export const BEACON_PER_NET_PER_MINUTE = 120;
/** 利用者のハッシュを作れないとき(D1 が無い・誤り)の beacon の回数制限(isolate の全員で共有するので緩くする)。 */
export const ANON_BEACON_PER_MINUTE = 600;
/** 利用者のハッシュを作れなかったときの visitor の値(空文字 = D1 の束縛が無い と区別する。AGENTS §8-7)。 */
export const VISITOR_UNAVAILABLE = "unavailable";
/**
 * 投票の回数制限(1 回線・1 分。F-01)。1 日の上限(votes.ts)とは別に、連打を抑える。👍 / 👎 の付け替えを
 * 1 人が素早く続けても 1 分に 20 回ほど。共有 IP の数人分を見込んで 60。
 */
export const VOTE_PER_NET_PER_MINUTE = 60;
/**
 * 票の配信(GET /api/votes)の回数制限(1 回線・1 分。S9 のセキュリティ修正 F-02)。画面は「提案を出す」ごとに 1 回読み、
 * 同じ条件の再表示はメモリのキャッシュが受ける。条件を変えて検索し直しても 1 人 1 分に十数回なので、共有 IP の数人分を見込んで 120。
 * 連打で D1 の読み取りの無料枠(全員の票)を使い切らせないため。
 */
export const VOTES_LIST_PER_NET_PER_MINUTE = 120;
const MINUTE_MS = 60_000;

const CODE_RE = /^[a-z0-9_]{1,64}$/;

export interface Deps {
  now: () => Date;
  salts: SaltStore;
  /** beacon の回数制限(鍵は回線のハッシュ)。 */
  beaconLimiter: WindowLimiter;
  /** ハッシュを作れない(D1 が無い・誤り)beacon の回数制限(isolate の全員で 1 つの鍵)。 */
  anonBeaconLimiter: WindowLimiter;
  /** 投票の回数制限(鍵は回線のハッシュ)。 */
  voteLimiter: WindowLimiter;
  /** 票の配信の回数制限(鍵は回線のハッシュ)。 */
  votesLimiter: WindowLimiter;
  /** 配信しているキャラ・技の目録(静的アセットの manifest・moves DB。F-03)。 */
  catalog: Catalog;
}

/** 目録の問い合わせ。読めなければ 503 の応答(Response)を返す。 */
async function lookup<T>(f: () => Promise<T>): Promise<T | Response> {
  try {
    return await f();
  } catch (e) {
    if (e instanceof CatalogUnavailable) return fail(503, "data_unavailable");
    throw e;
  }
}

/**
 * /api の応答に付けるセキュリティのヘッダ。_headers は静的アセットにだけ掛かる(Worker の応答には掛からない)ので、ここで付ける。
 * JSON しか返さないので CSP は何も読ませない形。HSTS は public/_headers と同じ値(S7。preload は付けない＝workers.dev は自分のドメインでない)。
 */
export const API_SECURITY_HEADERS: Readonly<Record<string, string>> = {
  "content-security-policy": "default-src 'none'; frame-ancestors 'none'",
  "x-content-type-options": "nosniff",
  "referrer-policy": "strict-origin-when-cross-origin",
  "x-frame-options": "DENY",
  "cross-origin-resource-policy": "same-origin",
  "strict-transport-security": "max-age=31536000",
};

/**
 * 静的アセットに無い要求(Worker の最後の env.ASSETS.fetch)の誤りの応答に、/api と同じセキュリティのヘッダを付ける
 * (S9 のセキュリティ修正 F-07)。_headers が env.ASSETS.fetch の 404 に掛かるかは Cloudflare の docs に書かれておらず
 * 確かめられないため(docs は「Worker のコードが作る応答には掛からない」まで)。中身はページではないので CSP は何も読ませない形でよい。
 * 2xx・3xx(アセットそのもの・転送)は _headers に任せて触らない。既にあるヘッダは上書きしない。
 */
export function withErrorSecurityHeaders(res: Response): Response {
  if (res.status < 400) return res;
  const headers = new Headers(res.headers);
  for (const [k, v] of Object.entries(API_SECURITY_HEADERS)) if (!headers.has(k)) headers.set(k, v);
  return new Response(res.body, { status: res.status, statusText: res.statusText, headers });
}

function json(status: number, body: unknown, cacheControl = "no-store"): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { ...API_SECURITY_HEADERS, "content-type": "application/json; charset=utf-8", "cache-control": cacheControl },
  });
}

function fail(status: number, error: string): Response {
  const body: ApiErrorReply = { error };
  return json(status, body);
}

/**
 * 本文を上限つきで読む。超えたら null。Content-Length で先に断り、無い(chunked)ときも読みながら数えて、
 * 上限を超えた時点で読むのを止める(全部を読んでから比べない。S9 のセキュリティ修正 F-04)。
 * chunk の境目で割れたマルチバイト文字は、TextDecoder の stream で繋ぐ。
 */
export async function readBody(req: Request): Promise<string | null> {
  const declared = Number(req.headers.get("content-length") ?? "0");
  if (declared > API_BODY_MAX_BYTES) return null;
  if (req.body == null) return "";
  const reader = req.body.getReader();
  const decoder = new TextDecoder();
  let total = 0;
  let text = "";
  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;
    total += value.byteLength;
    if (total > API_BODY_MAX_BYTES) {
      await reader.cancel().catch(() => undefined);
      return null;
    }
    text += decoder.decode(value, { stream: true });
  }
  return text + decoder.decode();
}

/** 別のサイトからの POST を断る(ブラウザは同じオリジンの POST にも Origin を付ける)。Origin が無い要求は通す。 */
function sameOrigin(req: Request): boolean {
  const origin = req.headers.get("origin");
  return origin == null || origin === new URL(req.url).origin;
}

/** 利用者のハッシュ(票の重複防止・計測の推定利用者数)と回線のハッシュ(回数制限・回線ごとの上限)。D1 が無ければ null。 */
async function identityOf(req: Request, env: Env, deps: Deps): Promise<{ visitor: string; net: string } | null> {
  if (env.DB == null) return null;
  const salt = await deps.salts.today(env.DB, deps.now());
  const ip = req.headers.get("cf-connecting-ip") ?? "";
  return { visitor: await visitorHash(ip, req.headers.get("user-agent") ?? "", salt), net: await netHash(ip, salt) };
}

async function handleBeacon(req: Request, env: Env, deps: Deps): Promise<Response> {
  if (req.method !== "POST") return fail(405, "method_not_allowed");
  if (!sameOrigin(req)) return fail(403, "forbidden_origin");
  const text = await readBody(req);
  if (text == null) return fail(413, "too_large");
  const ev = parseBeacon(text);
  if (ev == null) return fail(400, "invalid_beacon");
  // ★D1(票・ソルト)が誤りを返しても計測は止めない(無料枠は別。票の障害で計測まで落とさない)。ハッシュなしで書く。
  let id: { visitor: string; net: string } | null;
  let visitor: string;
  try {
    id = await identityOf(req, env, deps);
    visitor = id?.visitor ?? "";
  } catch {
    id = null;
    visitor = VISITOR_UNAVAILABLE;
  }
  const limiter = id != null ? deps.beaconLimiter : deps.anonBeaconLimiter;
  if (!limiter.take(id != null ? id.net : "anonymous", deps.now().getTime())) return fail(429, "rate_limited");
  env.TELEMETRY?.writeDataPoint(toDataPoint({ ...ev, visitor }));
  return new Response(null, { status: 204, headers: { ...API_SECURITY_HEADERS, "cache-control": "no-store" } });
}

async function handleVote(req: Request, env: Env, deps: Deps): Promise<Response> {
  if (req.method !== "POST") return fail(405, "method_not_allowed");
  if (!sameOrigin(req)) return fail(403, "forbidden_origin");
  if (env.DB == null) return fail(503, "votes_unavailable");
  const text = await readBody(req);
  if (text == null) return fail(413, "too_large");
  let raw: unknown;
  try {
    raw = JSON.parse(text);
  } catch {
    return fail(400, "invalid_vote");
  }
  const parsed = voteRequestSchema.safeParse(raw);
  if (!parsed.success) return fail(400, "invalid_vote");
  // 配信していないキャラ・技の票は受けない(F-03。存在しない鍵で配信の上限 VOTES_MAX_ENTRIES を埋めさせない)。
  const codes = await lookup(() => deps.catalog.moveCodes(env.ASSETS, new URL(req.url).origin, parsed.data.char, deps.now().getTime()));
  if (codes instanceof Response) return codes;
  if (codes == null) return fail(400, "unknown_character");
  if (!recipeCodes(parsed.data.recipe).every((c) => codes.has(c))) return fail(400, "unknown_move");
  const id = await identityOf(req, env, deps);
  if (id == null) return fail(503, "votes_unavailable");
  if (!deps.voteLimiter.take(id.net, deps.now().getTime())) return fail(429, "rate_limited");
  const r = await castVote(env.DB, utcDay(deps.now()), id.visitor, id.net, parsed.data);
  if (r.kind === "limited") return fail(429, "daily_limit");
  return json(200, r.reply);
}

async function handleVotes(req: Request, env: Env, deps: Deps): Promise<Response> {
  if (req.method !== "GET") return fail(405, "method_not_allowed");
  if (env.DB == null) return fail(503, "votes_unavailable");
  const q = new URL(req.url).searchParams;
  const char = q.get("char") ?? "";
  const kaText = q.get("ka") ?? "";
  const ka = Number(kaText);
  if (!CODE_RE.test(char) || !/^\d{1,3}$/.test(kaText) || ka < KA_MIN || ka > KA_MAX_SITE) return fail(400, "invalid_query");
  const known = await lookup(() => deps.catalog.hasCharacter(env.ASSETS, new URL(req.url).origin, char, deps.now().getTime()));
  if (known instanceof Response) return known;
  if (!known) return fail(400, "unknown_character");
  const id = await identityOf(req, env, deps);
  if (id == null) return fail(503, "votes_unavailable");
  if (!deps.votesLimiter.take(id.net, deps.now().getTime())) return fail(429, "rate_limited");
  // ★キャッシュさせない(no-store): 投票した直後の検索で、ブラウザに残った古い票で並べないため(S6 の E2E で見つけた)。
  //   読む行はそのキャラ・KA の票の付いたレシピだけで、同じ条件の再表示は画面のメモリのキャッシュが受けるので、D1 の読みは少ない。
  return json(200, await listVotes(env.DB, char, ka));
}

/** 試験のために依存を差し替えられる形で作る。 */
export function createHandler(deps: Deps) {
  return async (req: Request, env: Env): Promise<Response> => {
    const { pathname } = new URL(req.url);
    try {
      if (pathname === API_BEACON_PATH) return await handleBeacon(req, env, deps);
      if (pathname === API_VOTE_PATH) return await handleVote(req, env, deps);
      if (pathname === API_VOTES_PATH) return await handleVotes(req, env, deps);
    } catch {
      // D1 の無料枠を超えた(UTC 0 時まで誤り。S1 ⑦)等。中身は返さない(画面は票なし・計測なしで動く)。
      return fail(503, "unavailable");
    }
    if (pathname.startsWith("/api/")) return fail(404, "not_found");
    return withErrorSecurityHeaders(await env.ASSETS.fetch(req));
  };
}

const handler = createHandler({
  now: () => new Date(),
  salts: new SaltStore(),
  beaconLimiter: new WindowLimiter(BEACON_PER_NET_PER_MINUTE, MINUTE_MS),
  anonBeaconLimiter: new WindowLimiter(ANON_BEACON_PER_MINUTE, MINUTE_MS),
  voteLimiter: new WindowLimiter(VOTE_PER_NET_PER_MINUTE, MINUTE_MS),
  votesLimiter: new WindowLimiter(VOTES_LIST_PER_NET_PER_MINUTE, MINUTE_MS),
  catalog: new Catalog(),
});

export default {
  fetch: (req: Request, env: Env): Promise<Response> => handler(req, env),
};
