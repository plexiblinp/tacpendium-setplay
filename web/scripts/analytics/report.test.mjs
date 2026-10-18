import { describe, expect, it } from "vitest";

import { buildQueries, col, query, report, toMarkdown } from "./report.mjs";

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
    const md = await report(ok, "acc", "t", "setplay_runs", 3);
    expect(md).toContain("直近 3 日");
    expect(md.match(/^### /gm).length).toBe(buildQueries("setplay_runs", 3).length);
  });
});
