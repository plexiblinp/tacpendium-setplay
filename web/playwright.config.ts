import { defineConfig, devices } from "@playwright/test";

// E2E: ビルド済みの dist を _headers どおりのヘッダで配り(scripts/serve.mjs)、実 WASM・実データで画面を通す。
// 先に pnpm assets と pnpm build を済ませておくこと(CI の Web job と同じ)。
// AGENTS §8-10: retries は 0。不安定なら単独実行と直列実行で切り分ける。
const PORT = 4173;

export default defineConfig({
  testDir: "e2e",
  retries: 0,
  workers: 1,
  fullyParallel: false,
  timeout: 120_000,
  expect: { timeout: 30_000 },
  reporter: [["list"]],
  use: {
    baseURL: `http://127.0.0.1:${PORT}/`,
    trace: "retain-on-failure",
    ...devices["Desktop Chrome"],
    // 手元の環境では Playwright の版と合う Chromium が無いことがある。PW_CHROMIUM_PATH で差し替える(CI は playwright install)。
    launchOptions: process.env.PW_CHROMIUM_PATH ? { executablePath: process.env.PW_CHROMIUM_PATH } : {},
  },
  webServer: {
    command: `node scripts/serve.mjs dist ${PORT}`,
    url: `http://127.0.0.1:${PORT}/`,
    reuseExistingServer: false,
    timeout: 30_000,
  },
});
