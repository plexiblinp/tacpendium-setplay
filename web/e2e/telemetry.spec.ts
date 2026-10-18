// 計測と投票の通し(S6 の完了条件「実行ごとのイベントが見える」「票が保存され並べ替えの第一キーに効く」)。
// 画面 → sendBeacon → Worker の本物のコード(edge/。scripts/serve.mjs が dist-edge/local.mjs で動かす)→ Analytics Engine(偽物)、
// 画面 → POST /api/vote → D1(node:sqlite)→ GET /api/votes → WASM のエンジン(票が第一キー)→ 画面、を通す(AGENTS §8-5)。
// ★本家との突き合わせ(conformance.spec.ts)が使う条件(ryu・ken KA 43、zangief KA 79)には票を付けない(サーバの D1 は試験の間で共有)。
import { expect, test, type Page } from "@playwright/test";

import layout from "../edge/beacon-layout.json" with { type: "json" };

interface Point {
  blobs: string[];
  doubles: number[];
  indexes: string[];
}

async function points(page: Page): Promise<Point[]> {
  return (await (await page.request.get("/__e2e/points")).json()) as Point[];
}

const blob = (p: Point, k: string) => p.blobs[layout.blobs.indexOf(k)];
const dbl = (p: Point, k: string) => p.doubles[layout.doubles.indexOf(k)];

test("「提案を出す」1 回ごとに計測が 1 点届く(画面 → beacon → Worker → Analytics Engine)", async ({ page }) => {
  const before = (await points(page)).length;
  await page.goto("/");
  await page.getByTestId("character-select").click();
  await page.getByTestId("character-option-ryu").click();
  await page.getByTestId("ka-input").fill("40");
  await page.getByTestId("generate").click();
  await expect(page.getByTestId("count")).toHaveText(/321/);
  await expect.poll(async () => (await points(page)).length).toBe(before + 1);
  const p = (await points(page)).at(-1)!;
  expect(blob(p, "char")).toBe("ryu");
  expect(blob(p, "outcome")).toBe("result");
  expect(blob(p, "visitor")).toMatch(/^[0-9a-f]{32}$/);
  expect(dbl(p, "ka")).toBe(40);
  expect(dbl(p, "resultCount")).toBe(321);
  expect(dbl(p, "cacheHit")).toBe(0);
  expect(dbl(p, "gMin")).toBe(-1);
  expect(p.indexes).toEqual(["ryu"]);
  // 同じ条件をもう一度(キャッシュから出す)も 1 点。
  await page.getByTestId("generate").click();
  await expect.poll(async () => (await points(page)).length).toBe(before + 2);
  expect(dbl((await points(page)).at(-1)!, "cacheHit")).toBe(1);
});

test("票が保存され、次の検索で並べ替えの第一キーに効く(画面 → D1 → WASM → 画面)", async ({ page }) => {
  await page.goto("/?c=ryu&ka=50");
  await expect(page.getByTestId("result-row").first()).toBeVisible();
  await expect(page.getByTestId("votes-note")).toHaveAttribute("data-votes-status", "none");
  await page.getByTestId("show-more").click();
  const rows = page.getByTestId("result-row");
  const target = rows.nth(250);
  const key = await target.getAttribute("data-recipe-key");
  expect(key).toMatch(/\|/);
  const firstBefore = await rows.first().getAttribute("data-recipe-key");
  expect(firstBefore).not.toBe(key);

  await target.getByTestId("vote-good").click();
  await expect(target.getByTestId("good-count")).toHaveText("1");
  await expect(target.getByTestId("vote-good")).toHaveAttribute("aria-pressed", "true");
  const saved = (await (await page.request.get("/__e2e/votes")).json()) as { char: string; ka: number; recipe: string; good: number }[];
  expect(saved).toContainEqual({ char: "ryu", ka: 50, recipe: key, good: 1, bad: 0 });
  // 押した行はその場では動かない(次の検索から)。
  expect(await rows.first().getAttribute("data-recipe-key")).toBe(firstBefore);

  await page.getByTestId("generate").click();
  await expect(page.getByTestId("votes-note")).toHaveAttribute("data-votes-status", "ok");
  await expect(rows.first()).toHaveAttribute("data-recipe-key", key!);
  await expect(rows.first().getByTestId("good-count")).toHaveText("1");
  // 2 番目は票なしの並びの先頭(票の無い行は本家の順のまま)。
  await expect(rows.nth(1)).toHaveAttribute("data-recipe-key", firstBefore!);
});
