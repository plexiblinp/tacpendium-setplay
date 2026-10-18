// Worker(edge/)を束ねる(S6)。
//   dist-edge/worker.mjs … 本番の Worker と同じ入口(edge/index.ts)を webworker 向けに 1 ファイルへ。Node の組み込みに
//                          頼っていないこと・束ねられることの検査(デプロイは wrangler が自分で束ねる。S7 以降)
//   dist-edge/local.mjs  … 手元・E2E 用(edge/testing/local.ts。scripts/serve.mjs が読む)
// 使い方: node scripts/build-edge.mjs
import path from "node:path";
import { fileURLToPath } from "node:url";

import { build } from "vite";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const out = path.join(root, "dist-edge");

await build({
  root,
  configFile: false,
  publicDir: false,
  logLevel: "warn",
  ssr: { noExternal: true, target: "webworker" },
  build: { ssr: "edge/index.ts", outDir: out, emptyOutDir: true, minify: false, rollupOptions: { output: { entryFileNames: "worker.mjs" } } },
});
await build({
  root,
  configFile: false,
  publicDir: false,
  logLevel: "warn",
  build: { ssr: "edge/testing/local.ts", outDir: out, emptyOutDir: false, minify: false, rollupOptions: { output: { entryFileNames: "local.mjs" } } },
});
console.log(`束ねた: ${path.relative(root, out)}/worker.mjs, local.mjs`);
