// ブラウザ(Chromium)で提案の計算の所要を測る(S5: 計算方式 C/D の判断材料。S4 後の開発者決定 1)。
// 使い方: node scripts/measure/browser-timing.mjs <base URL> <出力 JSON> [chars] [kas]
//   例: node scripts/measure/browser-timing.mjs http://127.0.0.1:4173/ /tmp/t.json ryu,cammy 20,40,60,80,100
// キャラごとに新しいページを開き(WASM の起動と moves・表の読み込みはキャラの最初の 1 回)、条件を画面から順に回す。
// 1 条件の所要は画面の data-timings(wall = 押してから結果まで・compute = WASM の suggest)。
import fs from "node:fs";

import { chromium } from "@playwright/test";

const [base, out, charsArg, kasArg] = process.argv.slice(2);
if (!base || !out) throw new Error("使い方: browser-timing.mjs <base URL> <出力 JSON> [chars] [kas]");
const chars = (charsArg ?? "ryu,kimberly,cammy,zangief,dhalsim").split(",");
const kas = (kasArg ?? "20,40,60,80,100").split(",").map(Number);
const MODES = (process.env.MODES ?? "meaty,gap,filler").split(",");
const TIMEOUT_MS = 10 * 60 * 1000;

const browser = await chromium.launch({ executablePath: process.env.PW_CHROMIUM_PATH || undefined });
const rows = [];
for (const c of chars) {
  const page = await browser.newPage();
  page.on("pageerror", (e) => console.error(`pageerror(${c}):`, e.message));
  await page.goto(base);
  await page.getByTestId("character-select").selectOption(c);
  for (const ka of kas) {
    for (const mode of MODES) {
      await page.getByTestId("ka-input").fill(String(ka));
      await page.getByTestId(`mode-${mode}`).click();
      const gen = page.getByTestId("generate");
      if (!(await gen.isEnabled())) continue;
      await gen.click();
      const done = page.locator('[data-testid="count"], [data-testid="empty"], [data-testid="run-error"], [data-testid="negative-knockdown"]');
      await done.first().waitFor({ timeout: TIMEOUT_MS });
      const timing = page.getByTestId("timing");
      const t = (await timing.count()) > 0 ? JSON.parse((await timing.getAttribute("data-timings")) ?? "null") : null;
      const count = (await page.getByTestId("count").count()) > 0 ? await page.getByTestId("count").textContent() : "(0 件)";
      const err = (await page.getByTestId("run-error").count()) > 0 ? await page.getByTestId("run-error").textContent() : null;
      const truncated = (await page.getByTestId("truncated").count()) > 0;
      const row = { character: c, ka, mode, ...t, count, truncated, error: err };
      rows.push(row);
      console.log(JSON.stringify(row));
    }
  }
  await page.close();
}
await browser.close();
fs.writeFileSync(out, `${JSON.stringify({ base, ua: "chromium", rows }, null, 1)}\n`);
