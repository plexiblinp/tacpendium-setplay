// 画面が動き出す前・描画中に止まったとき、真っ白にせず理由を出す(index.html の #boot-status と public/boot.js・main.tsx の onUncaughtError)。
import { expect, test } from "@playwright/test";

test("画面のスクリプトが読めないと、真っ白でなく理由を出す", async ({ page }) => {
  await page.route(/\/assets\/index-[^/]+\.js$/, (r) => r.fulfill({ status: 404, body: "" }));
  await page.goto("/");
  await expect(page.locator("#boot-status")).toContainText("画面を表示できませんでした");
  await expect(page.locator("#boot-status")).toContainText("failed to load");
});

test("画面のスクリプトが動く前に例外を投げると、その内容を出す", async ({ page }) => {
  await page.route(/\/assets\/index-[^/]+\.js$/, (r) =>
    r.fulfill({ status: 200, contentType: "text/javascript", body: 'throw new Error("boom-for-test");' }),
  );
  await page.goto("/");
  await expect(page.locator("#boot-status")).toContainText("boom-for-test");
});

test("ふだんは読み込み中の表示が消え、画面が出る", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByTestId("character-select")).toBeVisible();
  await expect(page.locator("#boot-status")).toHaveCount(0);
});
