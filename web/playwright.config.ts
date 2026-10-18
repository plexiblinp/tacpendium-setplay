import path from "node:path";

import { defineConfig, devices } from "@playwright/test";

// E2E: ビルド済みの dist を _headers どおりのヘッダで配り(scripts/serve.mjs)、実 WASM・実データで画面を通す。
// 先に pnpm assets と pnpm build を済ませておくこと(CI の Web job と同じ)。
// project local(S8)はローカル版の実バイナリ(scripts/local/build.sh が作る。LOCAL_BIN で差し替え)を起動して通す。
// AGENTS §8-10: retries は 0。不安定なら単独実行と直列実行で切り分ける。
const PORT = 4173;
const LOCAL_PORT = 4174;

const GOOS: Record<string, string> = { linux: "linux", darwin: "darwin", win32: "windows" };
const GOARCH: Record<string, string> = { x64: "amd64", arm64: "arm64" };
const localBin =
  process.env.LOCAL_BIN ??
  path.join(
    "..",
    "build",
    "local",
    `tacpendium-setplay-local_${GOOS[process.platform] ?? process.platform}_${GOARCH[process.arch] ?? process.arch}${process.platform === "win32" ? ".exe" : ""}`,
  );

export default defineConfig({
  testDir: "e2e",
  retries: 0,
  workers: 1,
  fullyParallel: false,
  timeout: 120_000,
  expect: { timeout: 30_000 },
  reporter: [["list"]],
  use: {
    trace: "retain-on-failure",
    ...devices["Desktop Chrome"],
    // 手元の環境では Playwright の版と合う Chromium が無いことがある。PW_CHROMIUM_PATH で差し替える(CI は playwright install)。
    launchOptions: process.env.PW_CHROMIUM_PATH ? { executablePath: process.env.PW_CHROMIUM_PATH } : {},
  },
  projects: [
    { name: "site", testIgnore: /local\.spec\.ts$/, use: { baseURL: `http://127.0.0.1:${PORT}/` } },
    { name: "local", testMatch: /local\.spec\.ts$/, use: { baseURL: `http://127.0.0.1:${LOCAL_PORT}/` } },
  ],
  webServer: [
    {
      command: `node scripts/serve.mjs dist ${PORT}`,
      url: `http://127.0.0.1:${PORT}/`,
      reuseExistingServer: false,
      timeout: 30_000,
    },
    {
      command: `${JSON.stringify(localBin)} -open=false -port ${LOCAL_PORT}`,
      url: `http://127.0.0.1:${LOCAL_PORT}/`,
      reuseExistingServer: false,
      timeout: 30_000,
    },
  ],
});
