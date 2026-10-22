// @vitest-environment node
// Worker(edge/)の試験。D1 は node:sqlite の上で同じ SQL を流し、Analytics Engine は書いた点を覚える偽物。
import { beforeEach, describe, expect, it } from "vitest";

import { API_BODY_MAX_BYTES, BEACON_SCHEMA_VERSION, type BeaconEvent } from "../src/shared/api";
import layout from "./beacon-layout.json";
import { AE_MAX_BLOBS, AE_MAX_DOUBLES, toDataPoint } from "./beacon";
import type { Env } from "./env";
import {
  API_SECURITY_HEADERS,
  BEACON_PER_NET_PER_MINUTE,
  createHandler,
  VOTE_PER_NET_PER_MINUTE,
  VOTES_LIST_PER_NET_PER_MINUTE,
  type Deps,
} from "./index";
import { WindowLimiter } from "./rate";
import { MemoryAnalytics, SqliteD1 } from "./testing/sqliteD1";
import { VOTES_PER_NET_PER_DAY, VOTES_PER_VOTER_PER_DAY } from "./votes";
import { netOf, SaltStore } from "./visitor";

const ORIGIN = "https://setplay.example";

export function sampleBeacon(over: Partial<BeaconEvent> = {}): BeaconEvent {
  return {
    v: BEACON_SCHEMA_VERSION,
    char: "ryu",
    ka: 43,
    mode: "meaty",
    nMin: 1,
    nMax: 0,
    gMin: null,
    gMax: null,
    remain: null,
    safeJump: null,
    targetTypes: ["normal", "unique", "special_projectile"],
    targetMove: null,
    includeZeroDamage: false,
    approachOnly: false,
    blockSafeOnly: true,
    chainCancelOnly: false,
    sort: "n",
    notation: "srk",
    lang: "en",
    outcome: "result",
    errorCode: null,
    resultCount: 1234,
    items: 1234,
    truncated: false,
    cacheHit: false,
    ms: 321.5,
    computeMs: 300,
    votesStatus: "none",
    votesApplied: 0,
    dataVersion: "ffdb69e468a3f5c0b0e16255e68b6baa3266796d",
    sampleRate: 1,
    ...over,
  };
}

type CallInit = RequestInit & { ip?: string; ua?: string };

interface World {
  env: Env & { DB: SqliteD1; TELEMETRY: MemoryAnalytics };
  deps: Deps;
  call: (path: string, init?: CallInit) => Promise<Response>;
  clock: { t: Date };
}

function world(over: Partial<Deps> = {}): World {
  const clock = { t: new Date("2026-10-12T10:00:00Z") };
  const env = {
    DB: new SqliteD1(),
    TELEMETRY: new MemoryAnalytics(),
    ASSETS: { fetch: async () => new Response("asset", { status: 200 }) },
  };
  const deps: Deps = {
    now: () => clock.t,
    salts: new SaltStore(),
    beaconLimiter: new WindowLimiter(BEACON_PER_NET_PER_MINUTE, 60_000),
    anonBeaconLimiter: new WindowLimiter(1000, 60_000),
    voteLimiter: new WindowLimiter(100_000, 60_000),
    votesLimiter: new WindowLimiter(100_000, 60_000),
    ...over,
  };
  const h = createHandler(deps);
  const call = (path: string, init: CallInit = {}) => {
    const headers = new Headers(init.headers);
    headers.set("cf-connecting-ip", init.ip ?? "203.0.113.7");
    headers.set("user-agent", init.ua ?? "test-agent");
    if (init.method === "POST" && !headers.has("origin")) headers.set("origin", ORIGIN);
    return h(new Request(ORIGIN + path, { ...init, headers }), env);
  };
  return { env, deps, call, clock };
}

const post = (body: unknown, extra: RequestInit = {}): RequestInit => ({
  method: "POST",
  body: typeof body === "string" ? body : JSON.stringify(body),
  headers: { "content-type": "text/plain;charset=UTF-8" },
  ...extra,
});

describe("beacon", () => {
  let w: World;
  beforeEach(() => {
    w = world();
  });

  it("正しい beacon は 204 で、Analytics Engine に 1 点(layout の順)", async () => {
    const r = await w.call("/api/beacon", post(sampleBeacon()));
    expect(r.status).toBe(204);
    expect(w.env.TELEMETRY.points).toHaveLength(1);
    const p = w.env.TELEMETRY.points[0]!;
    const blob = (k: string) => p.blobs![layout.blobs.indexOf(k)];
    const dbl = (k: string) => p.doubles![layout.doubles.indexOf(k)];
    expect(blob("char")).toBe("ryu");
    expect(blob("targetTypes")).toBe("normal,special_projectile,unique");
    expect(blob("targetMove")).toBe(""); // null は空文字
    expect(blob("visitor")).toMatch(/^[0-9a-f]{32}$/);
    expect(dbl("ka")).toBe(43);
    expect(dbl("gMin")).toBe(-1); // null(当てはまらない)は -1
    expect(dbl("nMax")).toBe(0); // 0(上限なし)は 0 のまま
    expect(dbl("blockSafeOnly")).toBe(1);
    expect(dbl("ms")).toBe(321.5);
    expect(p.indexes).toEqual(["ryu"]);
  });

  it("生の IP は点に入らない(ハッシュだけ)", async () => {
    await w.call("/api/beacon", post(sampleBeacon(), { ip: "198.51.100.23" } as RequestInit));
    expect(JSON.stringify(w.env.TELEMETRY.points)).not.toContain("198.51.100.23");
  });

  it.each([
    ["範囲外の KA", { ka: 101 }],
    ["知らないモード", { mode: "xxx" }],
    ["知らない項目", { extra: 1 }],
    ["技 code の形でない", { targetMove: "<script>" }],
    ["抽出率 0", { sampleRate: 0 }],
    ["所要が負", { ms: -1 }],
  ])("検証で落とす: %s", async (_name, over) => {
    const r = await w.call("/api/beacon", post({ ...sampleBeacon(), ...over }));
    expect(r.status).toBe(400);
    expect(w.env.TELEMETRY.points).toHaveLength(0);
  });

  it("JSON でない・大きすぎる・GET・別のオリジンは書かない", async () => {
    expect((await w.call("/api/beacon", post("{"))).status).toBe(400);
    expect((await w.call("/api/beacon", post("x".repeat(API_BODY_MAX_BYTES + 1)))).status).toBe(413);
    expect((await w.call("/api/beacon")).status).toBe(405);
    expect((await w.call("/api/beacon", post(sampleBeacon(), { headers: { origin: "https://evil.example" } }))).status).toBe(403);
    expect(w.env.TELEMETRY.points).toHaveLength(0);
  });

  it("1 回線 1 分の上限を超えたら 429(別の回線は通る)", async () => {
    for (let i = 0; i < BEACON_PER_NET_PER_MINUTE; i++) expect((await w.call("/api/beacon", post(sampleBeacon()))).status).toBe(204);
    expect((await w.call("/api/beacon", post(sampleBeacon()))).status).toBe(429);
    expect((await w.call("/api/beacon", post(sampleBeacon(), { ip: "192.0.2.1" } as RequestInit))).status).toBe(204);
    w.clock.t = new Date(w.clock.t.getTime() + 60_000);
    expect((await w.call("/api/beacon", post(sampleBeacon()))).status).toBe(204);
  });

  it("UA を変えても同じ回線は 1 分の上限で 429(F-01)", async () => {
    for (let i = 0; i < BEACON_PER_NET_PER_MINUTE; i++) {
      expect((await w.call("/api/beacon", { ...post(sampleBeacon()), ua: `agent-${i}` })).status).toBe(204);
    }
    expect((await w.call("/api/beacon", { ...post(sampleBeacon()), ua: "agent-new" })).status).toBe(429);
    expect((await w.call("/api/beacon", { ...post(sampleBeacon()), ip: "192.0.2.1", ua: "agent-new" })).status).toBe(204);
  });

  it("IPv6 は同じ /64 を 1 つの回線として数える(F-01)", async () => {
    const same = ["2001:db8:1:2::1", "2001:0db8:0001:0002:ffff::9"];
    for (let i = 0; i < BEACON_PER_NET_PER_MINUTE; i++) {
      expect((await w.call("/api/beacon", { ...post(sampleBeacon()), ip: same[i % 2], ua: `a${i}` })).status).toBe(204);
    }
    expect((await w.call("/api/beacon", { ...post(sampleBeacon()), ip: same[0] })).status).toBe(429);
    expect((await w.call("/api/beacon", { ...post(sampleBeacon()), ip: "2001:db8:1:3::1" })).status).toBe(204); // 別の /64
  });

  it("D1 が誤りを返しても beacon は 204 で 1 点書く(visitor は unavailable。票の障害で計測を止めない)", async () => {
    const broken = { ...w.env, DB: { prepare: () => { throw new Error("D1 free tier exceeded"); }, batch: async () => { throw new Error("D1 free tier exceeded"); } } };
    const r = await createHandler({ ...w.deps, salts: new SaltStore() })(new Request(ORIGIN + "/api/beacon", post(sampleBeacon())), broken as unknown as Env);
    expect(r.status).toBe(204);
    expect(w.env.TELEMETRY.points).toHaveLength(1);
    expect(w.env.TELEMETRY.points[0]!.blobs![layout.blobs.indexOf("visitor")]).toBe("unavailable");
  });

  it("ハッシュを作れない beacon は別の(緩い)上限で数える(再レビュー低-3)", async () => {
    const deps = { ...w.deps, anonBeaconLimiter: new WindowLimiter(2, 60_000) };
    const env = { ...w.env, DB: undefined };
    const h = createHandler(deps);
    const send = () => h(new Request(ORIGIN + "/api/beacon", post(sampleBeacon())), env);
    expect((await send()).status).toBe(204);
    expect((await send()).status).toBe(204);
    expect((await send()).status).toBe(429);
    expect((await w.call("/api/beacon", post(sampleBeacon()))).status).toBe(204); // ハッシュのある人は別の上限
  });

  it("Analytics Engine が無くても 204(計測が止まっても画面は困らない)", async () => {
    const env = { ...w.env, TELEMETRY: undefined };
    const r = await createHandler(w.deps)(new Request(ORIGIN + "/api/beacon", post(sampleBeacon())), env);
    expect(r.status).toBe(204);
  });

  it("layout は beacon の全項目＋visitor を 1 回ずつ持ち、Analytics Engine の上限に収まる", () => {
    const keys = [...Object.keys(sampleBeacon()), "visitor"].sort();
    expect([...layout.blobs, ...layout.doubles].sort()).toEqual(keys);
    expect(layout.blobs.length).toBeLessThanOrEqual(AE_MAX_BLOBS);
    expect(layout.doubles.length).toBeLessThanOrEqual(AE_MAX_DOUBLES);
    const p = toDataPoint({ ...sampleBeacon(), visitor: "v" });
    expect(p.blobs).toHaveLength(layout.blobs.length);
    expect(p.doubles!.every((d) => Number.isFinite(d))).toBe(true);
  });
});

describe("票", () => {
  let w: World;
  beforeEach(() => {
    w = world();
  });
  const vote = (recipe: string, value: number, ip?: string) =>
    w.call("/api/vote", post({ char: "ryu", ka: 43, recipe, value }, ip ? ({ ip } as RequestInit) : {}));
  const votes = async (ka = 43) => (await (await w.call(`/api/votes?char=ryu&ka=${ka}`)).json()) as { votes: Record<string, [number, number]> };

  it("付ける・付け替える・取り消す(同じ人・同じ日は 1 票)", async () => {
    expect(await (await vote("a,b|t", 1)).json()).toEqual({ good: 1, bad: 0, value: 1 });
    expect(await (await vote("a,b|t", 1)).json()).toEqual({ good: 1, bad: 0, value: 1 }); // 二重に数えない
    expect(await (await vote("a,b|t", -1)).json()).toEqual({ good: 0, bad: 1, value: -1 }); // 付け替え
    expect(await (await vote("a,b|t", 1, "192.0.2.9")).json()).toEqual({ good: 1, bad: 1, value: 1 }); // 別の人
    expect(await (await vote("a,b|t", 0)).json()).toEqual({ good: 1, bad: 0, value: 0 }); // 取り消し
    expect(await votes()).toEqual({ votes: { "a,b|t": [1, 0] } });
    expect(await votes(44)).toEqual({ votes: {} }); // 票は KA ごと
  });

  it("日が変わると前日の記録とソルトを消し、同じ人がもう 1 票付けられる", async () => {
    await vote("x|", 1);
    w.clock.t = new Date("2026-10-13T00:00:01Z");
    expect(await (await vote("x|", 1)).json()).toEqual({ good: 2, bad: 0, value: 1 });
    const days = w.env.DB.db.prepare("SELECT DISTINCT day FROM ballots").all();
    expect(days).toEqual([{ day: "2026-10-13" }]);
    expect(w.env.DB.db.prepare("SELECT day FROM salts").all()).toEqual([{ day: "2026-10-13" }]);
  });

  it("投票の記録に生の IP は残らない", async () => {
    await vote("a|t", 1, "198.51.100.23");
    const dump = JSON.stringify(w.env.DB.db.prepare("SELECT * FROM ballots").all());
    expect(dump).not.toContain("198.51.100.23");
  });

  it("1 人 1 日の上限(付け替えは上限の後もできる)", async () => {
    for (let i = 0; i < VOTES_PER_VOTER_PER_DAY; i++) expect((await vote(`m${i}|`, 1)).status).toBe(200);
    expect((await vote("over|", 1)).status).toBe(429);
    expect((await vote("m0|", -1)).status).toBe(200);
  });

  it("UA を変えても同じ回線は 1 分の上限で 429(F-01)", async () => {
    const v = world({ voteLimiter: new WindowLimiter(VOTE_PER_NET_PER_MINUTE, 60_000) });
    const cast = (i: number, ip?: string) => v.call("/api/vote", { ...post({ char: "ryu", ka: 43, recipe: `r${i}|`, value: 1 }), ip, ua: `agent-${i}` });
    for (let i = 0; i < VOTE_PER_NET_PER_MINUTE; i++) expect((await cast(i)).status).toBe(200);
    expect((await cast(VOTE_PER_NET_PER_MINUTE)).status).toBe(429);
    expect((await cast(VOTE_PER_NET_PER_MINUTE, "192.0.2.1")).status).toBe(200); // 別の回線は独立
  });

  it("UA を変えても 1 回線 1 日の上限で 429(IPv6 は /64 で数え、別の回線は独立。F-01)", async () => {
    const cast = (i: number, ip: string) => w.call("/api/vote", { ...post({ char: "ryu", ka: 43, recipe: `q${i}|`, value: 1 }), ip, ua: `agent-${i}` });
    const net = ["2001:db8:aa:bb::1", "2001:db8:aa:bb:1:2:3:4"];
    for (let i = 0; i < VOTES_PER_NET_PER_DAY; i++) expect((await cast(i, net[i % 2]!)).status).toBe(200);
    const over = await cast(VOTES_PER_NET_PER_DAY, net[0]!);
    expect(over.status).toBe(429);
    expect(await over.json()).toEqual({ error: "daily_limit" });
    expect((await cast(VOTES_PER_NET_PER_DAY, "2001:db8:aa:bc::1")).status).toBe(200); // 別の /64
    expect((await cast(VOTES_PER_NET_PER_DAY + 1, "198.51.100.30")).status).toBe(200); // 別の IPv4
  });

  it("票の配信の連打は 1 回線 1 分の上限で 429(UA を変えても。別の回線は独立。F-02)", async () => {
    const v = world({ votesLimiter: new WindowLimiter(VOTES_LIST_PER_NET_PER_MINUTE, 60_000) });
    const get = (i: number, ip?: string) => v.call(`/api/votes?char=ryu&ka=${i % 101}`, { ip, ua: `agent-${i}` });
    for (let i = 0; i < VOTES_LIST_PER_NET_PER_MINUTE; i++) expect((await get(i)).status).toBe(200);
    expect((await get(VOTES_LIST_PER_NET_PER_MINUTE)).status).toBe(429);
    expect((await get(0, "192.0.2.1")).status).toBe(200);
    v.clock.t = new Date(v.clock.t.getTime() + 60_000);
    expect((await get(0)).status).toBe(200);
  });

  it("回線ごとの数は日替わりで消え、生の IP も回線も残らない(F-01)", async () => {
    const ip = "198.51.100.23";
    await vote("a|t", 1, ip);
    const dump = JSON.stringify(w.env.DB.db.prepare("SELECT * FROM quotas").all());
    expect(dump).toMatch(/[0-9a-f]{32}/);
    expect(dump).not.toContain(ip);
    expect(dump).not.toContain(netOf(ip));
    w.clock.t = new Date("2026-10-13T00:00:01Z");
    await vote("b|t", 1, ip);
    expect(w.env.DB.db.prepare("SELECT day, n FROM quotas").all()).toEqual([{ day: "2026-10-13", n: 1 }]);
  });

  it.each([
    ["鍵の形でない", { recipe: "a|b|c" }],
    ["空の鍵", { recipe: "|" }],
    ["値が 2", { value: 2 }],
    ["KA が範囲外", { ka: -1 }],
    ["知らない項目", { x: 1 }],
  ])("検証で落とす: %s", async (_n, over) => {
    const r = await w.call("/api/vote", post({ char: "ryu", ka: 43, recipe: "a|t", value: 1, ...over }));
    expect(r.status).toBe(400);
  });

  it("配信の誤り・D1 が無いとき", async () => {
    expect((await w.call("/api/votes?char=ryu&ka=101")).status).toBe(400);
    expect((await w.call("/api/votes?char=RYU&ka=1")).status).toBe(400);
    expect((await w.call("/api/votes?char=ryu&ka=1.5")).status).toBe(400);
    const env = { ...w.env, DB: undefined };
    const h = createHandler(w.deps);
    expect((await h(new Request(ORIGIN + "/api/votes?char=ryu&ka=1"), env)).status).toBe(503);
    expect((await h(new Request(ORIGIN + "/api/vote", post({ char: "ryu", ka: 1, recipe: "a|", value: 1 })), env)).status).toBe(503);
  });

  it("D1 が誤りを返したら 503(中身を返さない)", async () => {
    const broken = { ...w.env, DB: { prepare: () => { throw new Error("D1 free tier exceeded"); }, batch: async () => [] } };
    const r = await createHandler({ ...w.deps, salts: new SaltStore() })(new Request(ORIGIN + "/api/votes?char=ryu&ka=1"), broken as unknown as Env);
    expect(r.status).toBe(503);
    expect(await r.text()).not.toContain("free tier");
  });

  it("配信の行は主キーの前方一致で、票が 0・0 の行は出さない", async () => {
    await vote("a|t", 1);
    await vote("a|t", 0);
    expect(await votes()).toEqual({ votes: {} });
    const plan = JSON.stringify(
      w.env.DB.db.prepare("EXPLAIN QUERY PLAN SELECT recipe, good, bad FROM votes WHERE char = ? AND ka = ?").all("ryu", 43),
    );
    expect(plan).toMatch(/USING PRIMARY KEY|USING INDEX/);
  });
});

describe("そのほか", () => {
  it("/api 以外は静的アセット、知らない /api は 404", async () => {
    const w = world();
    expect(await (await w.call("/index.html")).text()).toBe("asset");
    expect((await w.call("/api/nope")).status).toBe(404);
  });
});

describe("キャッシュ", () => {
  it("票の配信はキャッシュさせない(投票の直後の検索で古い票を使わない)", async () => {
    const w = world();
    const r = await w.call("/api/votes?char=ryu&ka=1");
    expect(r.headers.get("cache-control")).toBe("no-store");
    expect(r.headers.get("x-content-type-options")).toBe("nosniff");
  });
});

describe("セキュリティのヘッダ(S7)", () => {
  it("/api のどの応答にも付く(200・204・4xx)", async () => {
    const w = world();
    const responses = [
      await w.call("/api/votes?char=ryu&ka=1"),
      await w.call("/api/beacon", post(sampleBeacon())),
      await w.call("/api/beacon", post("{")),
      await w.call("/api/nope"),
    ];
    expect(responses.map((r) => r.status)).toEqual([200, 204, 400, 404]);
    // 期待値は定数と別に書く(定数から項目を消しても赤になるように。S7 レビュー低-8)。
    const expected: Record<string, string> = {
      "content-security-policy": "default-src 'none'; frame-ancestors 'none'",
      "x-content-type-options": "nosniff",
      "referrer-policy": "strict-origin-when-cross-origin",
      "x-frame-options": "DENY",
      "cross-origin-resource-policy": "same-origin",
      "strict-transport-security": "max-age=31536000",
    };
    expect(API_SECURITY_HEADERS).toEqual(expected);
    for (const r of responses) {
      for (const [k, v] of Object.entries(expected)) expect(r.headers.get(k), `${r.status} ${k}`).toBe(v);
    }
  });
});

describe("回線の鍵(F-01)", () => {
  it.each([
    ["203.0.113.7", "4:203.0.113.7"],
    ["::ffff:203.0.113.7", "4:203.0.113.7"],
    ["2001:db8::1", "6:2001:db8:0:0::/64"],
    ["2001:0DB8:0000:0000:ffff:1:2:3", "6:2001:db8:0:0::/64"],
    ["2001:db8:1:2:3:4:5:6", "6:2001:db8:1:2::/64"],
    ["::1", "6:0:0:0:0::/64"],
    ["fe80::1%eth0", "6:fe80:0:0:0::/64"],
    ["", "?:"],
    ["not-an-ip", "?:not-an-ip"],
  ])("%s → %s", (ip, net) => {
    expect(netOf(ip)).toBe(net);
  });
});
