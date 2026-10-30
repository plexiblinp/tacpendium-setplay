import { describe, expect, it } from "vitest";

import { buildQueries, col, query, report, run, toMarkdown } from "./report.mjs";

describe("集計スクリプト", () => {
  it("項目名を layout の列に写す(書く側と同じ edge/beacon-layout.json)", () => {
    expect(col("char")).toBe("blob1");
    expect(col("ka")).toBe("double2");
    expect(col("visitor")).toMatch(/^blob\d+$/);
    expect(() => col("nope")).toThrow();
  });
  it("問い合わせは期間・版で絞り、抽出の重みで割り戻す", () => {
    const qs = buildQueries("setplay_runs", 7);
    expect(qs.length).toBeGreaterThan(5);
    for (const q of qs) {
      expect(q.sql).toContain("INTERVAL '7' DAY");
      expect(q.sql).toContain(`${col("v")} = 1`);
    }
    expect(qs.find((q) => q.title.startsWith("KA 80")).sql).toContain(`${col("ka")} >= 80`);
    expect(qs[0].sql).toContain(`_sample_interval / ${col("sampleRate")}`);
    // 分位点も抽出率で割り戻した整数の重みを使う(再レビュー低-3)。
    expect(qs.find((q) => q.title.startsWith("KA の帯の待ち時間")).sql).toContain(`quantileExactWeighted(0.5)(${col("ms")}, toUInt32(round(_sample_interval / ${col("sampleRate")})))`);
  });
  it("条件つきの和は sumIf で書き、if を使わない(本物の SQL API は if の 2・3 番目の型が揃わないと HTTP 422。2026-10-30 に実際に出た)", () => {
    for (const q of buildQueries("setplay_runs", 7)) {
      // sumIf・countIf・avgIf は対象外(名前の途中の if)。
      expect(q.sql, q.title).not.toMatch(/(?<![A-Za-z])if\s*\(/i);
    }
    const heavy = buildQueries("setplay_runs", 7).find((q) => q.title.startsWith("KA 80")).sql;
    expect(heavy).toContain(`sumIf(_sample_interval / ${col("sampleRate")}, ${col("ka")} >= 80)`);
    const zero = buildQueries("setplay_runs", 7).find((q) => q.title.startsWith("結果が 0 件")).sql;
    expect(zero).toContain(`sumIf(_sample_interval / ${col("sampleRate")}, ${col("resultCount")} = 0)`);
  });
  it("dataset・日数の不正は誤り(SQL に埋め込まない)", () => {
    expect(() => buildQueries("x; DROP", 7)).toThrow();
    expect(() => buildQueries("ok", 0)).toThrow();
    expect(() => buildQueries("ok", 93)).toThrow();
  });
  it("表にする(0 行は「該当なし」)", () => {
    expect(toMarkdown("t", [{ a: 1, b: "x|y" }])).toBe("### t\n\n| a | b |\n|---|---|\n| 1 | x\\|y |\n");
    expect(toMarkdown("t", [])).toContain("該当なし");
  });
  it("SQL API を叩き、誤りにトークンを出さない", async () => {
    const seen = [];
    const ok = async (url, init) => {
      seen.push({ url, init });
      return new Response(JSON.stringify({ data: [{ runs: 3 }] }), { status: 200 });
    };
    expect(await query(ok, "acc", "secret-token", "SELECT 1")).toEqual([{ runs: 3 }]);
    expect(seen[0].url).toBe("https://api.cloudflare.com/client/v4/accounts/acc/analytics_engine/sql");
    expect(seen[0].init.body).toBe("SELECT 1 FORMAT JSON");
    expect(seen[0].init.headers.Authorization).toBe("Bearer secret-token");
    const bad = async () => new Response("denied", { status: 403 });
    await expect(query(bad, "acc", "secret-token", "SELECT 1")).rejects.toThrow(/HTTP 403/);
    await expect(query(bad, "acc", "secret-token", "SELECT 1")).rejects.not.toThrow(/secret-token/);
    const { markdown: md, failures } = await report(ok, "acc", "t", "setplay_runs", 3);
    expect(failures).toBe(0);
    expect(md).toContain("直近 3 日");
    expect(md.match(/^### /gm).length).toBe(buildQueries("setplay_runs", 3).length);
  });
  // 本物の SQL API が sumIf を含む問い合わせだけ 422 で返す、という形の偽物(2026-10-30 の失敗の形)。
  const partly = async (_url, init) =>
    init.body.includes("sumIf")
      ? new Response(`Input was invalid: ${"x".repeat(400)}`, { status: 422 })
      : new Response(JSON.stringify({ data: [{ runs: 3 }] }), { status: 200 });
  it("1 つの表が失敗しても残りの表は出し、失敗した表には誤りの先頭を出す(トークンは出さない)", async () => {
    const qs = buildQueries("setplay_runs", 7);
    const { markdown, failures } = await report(partly, "acc", "secret-token", "setplay_runs", 7);
    expect(failures).toBe(qs.filter((q) => q.sql.includes("sumIf")).length);
    expect(failures).toBe(2);
    expect(markdown.match(/^### /gm).length).toBe(qs.length);
    expect(markdown.match(/^失敗: SQL API: HTTP 422: Input was invalid/gm).length).toBe(2);
    expect(markdown.match(/^\| 3 \|$/gm).length).toBe(qs.length - 2);
    // 誤りの文は先頭 300 字まで。
    expect(markdown).not.toContain("x".repeat(300));
    expect(markdown).not.toContain("secret-token");
  });
  it("コマンドは、失敗した表があれば表を出したうえで終了コード 1", async () => {
    const out = [];
    const err = [];
    const env = { CLOUDFLARE_ACCOUNT_ID: "acc", CLOUDFLARE_API_TOKEN: "secret-token" };
    expect(await run(env, ["7"], partly, (s) => out.push(s), (s) => err.push(s))).toBe(1);
    expect(out.join("\n")).toContain("### キャラ別の実行数");
    expect(err.join("\n")).toContain("失敗した表: 2 件");
    expect([...out, ...err].join("\n")).not.toContain("secret-token");
    const ok = async () => new Response(JSON.stringify({ data: [] }), { status: 200 });
    expect(await run(env, [], ok, () => {}, () => {})).toBe(0);
    expect(await run({}, [], ok, () => {}, (s) => err.push(s))).toBe(2);
    expect(await run(env, ["0"], ok, () => {}, () => {})).toBe(1);
  });
});
