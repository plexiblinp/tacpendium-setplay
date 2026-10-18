// 画面に出た結果が本家の正解(engine/testdata/upstream_oracle.json)と順序まで一致することを確かめる
// (Go → WASM → Worker → 画面の通し。AGENTS §8-5)。共有 URL で条件を渡し、行の data-canonical を集めて sha256 を取る。
// ★本家の正解は limit つき(既定 200 または 3000)。画面は 3000 件で 1 回数えて先頭から出すので、
//   既定(limit 200)の条件は「画面の先頭 200 行 = 本家の limit 200 の結果」を見る(並べ替えの後で切る、の確認)。
import { createHash } from "node:crypto";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

import { expect, test, type Page } from "@playwright/test";

interface Cell {
  character: string;
  ka: number;
  query: string;
  status: number;
  totalFound: number;
  truncated: boolean;
  items: number;
  sha256: string;
}

const ORACLE = path.join(path.dirname(fileURLToPath(import.meta.url)), "../../engine/testdata/upstream_oracle.json");
const cells = (JSON.parse(fs.readFileSync(ORACLE, "utf8")) as { cells: Cell[] }).cells;

// 本家のクエリ → 画面の共有 URL のクエリ。
const CASES: { character: string; ka: number; query: string; url: string }[] = [
  { character: "ryu", ka: 43, query: "", url: "" },
  { character: "ryu", ka: 43, query: "sort=target&n_min=2&n_max=4&limit=3000", url: "sort=target&n_min=2&n_max=4" },
  { character: "ryu", ka: 43, query: "mode=gap&g_min=2&g_max=5&sort=target&limit=3000", url: "mode=gap&g_min=2&g_max=5&sort=target" },
  { character: "ryu", ka: 43, query: "mode=filler&remain=4&limit=3000", url: "mode=filler&remain=4" },
  { character: "ryu", ka: 43, query: "approach_only=true&block_safe_only=true&limit=3000", url: "approach=1&block_safe=1" },
  { character: "ryu", ka: 43, query: "mode=gap&chain_cancel_only=true&limit=3000", url: "mode=gap&chain=1" },
  { character: "ryu", ka: 43, query: "target_move=standing_medium_punch&limit=3000", url: "move=standing_medium_punch" },
  {
    character: "ken",
    ka: 43,
    query: "limit=3000&target_types=normal,unique,special,special_projectile,throw,normal_rush,unique_rush&include_zero_damage=true",
    url: "types=normal,unique,special_projectile,special,throw,normal_rush,unique_rush&zero=1",
  },
  { character: "zangief", ka: 79, query: "", url: "" },
];

async function collect(page: Page): Promise<string[]> {
  await expect(page.getByTestId("count").or(page.getByTestId("empty"))).toBeVisible({ timeout: 90_000 });
  // さらに表示を押し切る。
  while ((await page.getByTestId("show-more").count()) > 0) {
    const before = await page.getByTestId("result-row").count();
    await page.getByTestId("show-more").click();
    await expect(page.getByTestId("result-row")).not.toHaveCount(before);
  }
  return page.getByTestId("result-row").evaluateAll((els) => els.map((e) => e.getAttribute("data-canonical") ?? ""));
}

for (const c of CASES) {
  test(`本家と一致: ${c.character} KA ${c.ka} ${JSON.stringify(c.query)}`, async ({ page }) => {
    const want = cells.find((x) => x.character === c.character && x.ka === c.ka && x.query === c.query);
    expect(want, "本家の正解に条件がある").toBeDefined();
    if (want == null) return;
    await page.goto(`/?c=${c.character}&ka=${c.ka}${c.url ? `&${c.url}` : ""}`);
    const lines = (await collect(page)).slice(0, want.items);
    expect(lines).toHaveLength(want.items);
    const sha = createHash("sha256").update(lines.map((l) => `${l}\n`).join(""), "utf8").digest("hex");
    expect(sha).toBe(want.sha256);
    if (want.items > 0) await expect(page.getByTestId("count")).toContainText(String(want.totalFound));
    await expect(page.getByTestId("truncated")).toHaveCount(want.truncated ? 1 : 0);
  });
}
