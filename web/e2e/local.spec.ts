// ローカル版(HANDOVER D10・§5・S8)の E2E。実バイナリ(cmd/local。埋め込んだローカル版のビルド)を起動して通す
// (playwright.config.ts の project local)。バイナリ → 埋め込み → _headers のヘッダ → Worker の WASM → 画面の通し(AGENTS §8-5)。
import { expect, test, type Page } from "@playwright/test";

function watch(page: Page): { errors: string[]; requests: string[] } {
  const errors: string[] = [];
  const requests: string[] = [];
  page.on("pageerror", (e) => errors.push(`pageerror: ${e.message}`));
  page.on("console", (m) => {
    if (m.type() === "error") errors.push(`console: ${m.text()}`);
  });
  page.on("request", (r) => requests.push(r.url()));
  return { errors, requests };
}

async function choose(page: Page, character: string, ka: number) {
  await page.getByTestId("character-select").click();
  await page.getByTestId(`character-option-${character}`).click();
  await page.getByTestId("ka-input").fill(String(ka));
}

test("KA 100 を超えて提案が出る。KA 40 はサイトと同じ件数。外へは通信しない", async ({ page, baseURL }) => {
  const w = watch(page);
  await page.goto("/");
  await expect(page.getByTestId("local-badge")).toBeVisible();
  await expect(page.getByTestId("local-notice")).toBeVisible();
  await expect(page.getByTestId("privacy-link")).toHaveCount(0);

  // サイトの E2E と同じ条件・同じ件数(engine/testdata の正解 ryu KA 40 meaty 既定: totalFound 321)。
  await choose(page, "ryu", 40);
  await page.getByTestId("generate").click();
  await expect(page.getByTestId("count")).toHaveText(/321/);

  await choose(page, "ryu", 120);
  await page.getByTestId("generate").click();
  await expect(page.getByTestId("result-row").first()).toBeVisible({ timeout: 90_000 });
  await expect(page.getByTestId("form-errors")).toHaveCount(0);
  await expect(page).toHaveURL(/\?c=ryu&ka=120$/);
  await expect(page.getByTestId("vote-good")).toHaveCount(0);
  await expect(page.getByTestId("votes-note")).toHaveCount(0);

  const origin = new URL(baseURL!).origin;
  expect(w.requests.filter((u) => !u.startsWith(origin) && !u.startsWith("blob:") && !u.startsWith("data:"))).toEqual([]);
  expect(w.requests.filter((u) => /\/api\/|config\.json/.test(new URL(u).pathname))).toEqual([]);
  expect(w.errors).toEqual([]);
});

test("共有 URL の KA 100 超えで自動で計算する", async ({ page }) => {
  const w = watch(page);
  await page.goto("/?c=ken&ka=110&mode=gap");
  await expect(page.getByTestId("result-row").first()).toBeVisible({ timeout: 90_000 });
  await expect(page.getByTestId("url-rejected")).toHaveCount(0);
  await expect(page.getByTestId("ka-input")).toHaveValue("110");
  // 共有の操作は出さない(127.0.0.1 の URL は相手が開けない。開発者決定 2026-10-20)。
  await expect(page.getByTestId("share")).toHaveCount(0);
  await expect(page.getByTestId("app-share")).toHaveCount(0);
  expect(w.errors).toEqual([]);
});

test("重い条件(KA 400)は中断で止められ、続けて計算できる", async ({ page }) => {
  const w = watch(page);
  await page.goto("/");
  await choose(page, "ryu", 400);
  await expect(page.getByTestId("slow-warning")).toBeVisible();
  await page.getByTestId("generate").click();
  await expect(page.getByTestId("loading")).toBeVisible();
  await page.getByTestId("cancel").click();
  await expect(page.getByTestId("canceled")).toBeVisible();
  await choose(page, "ryu", 40);
  await page.getByTestId("generate").click();
  await expect(page.getByTestId("count")).toHaveText(/321/);
  expect(w.errors).toEqual([]);
});
