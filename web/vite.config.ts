import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  build: {
    // 配信の上限(1 ファイル 25 MiB)とは別に、警告の閾値。wasm は public/ からそのまま置くので対象外。
    chunkSizeWarningLimit: 600,
    // ソースマップは配らない(公開物を小さく保つ。ソースはリポジトリで公開する＝AGPL)。
    sourcemap: false,
  },
  worker: {
    format: "es",
  },
  test: {
    environment: "jsdom",
    include: ["src/**/*.test.{ts,tsx}", "edge/**/*.test.ts", "scripts/**/*.test.{ts,mjs}"],
    setupFiles: ["src/test/setup.ts"],
  },
});
