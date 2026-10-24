// SNS のカードの画像(OGP の og:image。1200×630)を作り、public/og.png に書く(共有ボタンの作業 2026-10-20)。
// 使い方: node scripts/og-image.mjs   (PW_CHROMIUM_PATH で Chromium を差し替えられる。できた PNG はコミットする)
// ★自作の図形と文字だけ。ゲームのロゴ・画像は使わない(HANDOVER §8)。文字は英語だけ(字形のフォントが環境で変わらないように)。
import path from "node:path";
import { fileURLToPath } from "node:url";

import { chromium } from "@playwright/test";

const WIDTH = 1200;
const HEIGHT = 630;
const out = path.join(path.dirname(fileURLToPath(import.meta.url)), "../public/og.png");

// 色は画面(src/styles.css の --accent 等)と favicon.svg の形に揃える。
const html = `<!doctype html>
<html><head><meta charset="utf-8"><style>
  html, body { margin: 0; width: ${WIDTH}px; height: ${HEIGHT}px; }
  body { background: #f7f8fa; color: #1d2330; font-family: "DejaVu Sans", Arial, sans-serif; display: flex; align-items: center; }
  .card { margin: 0 80px; display: flex; gap: 48px; align-items: center; }
  .mark { width: 180px; height: 180px; flex: none; }
  h1 { font-size: 64px; margin: 0 0 20px; letter-spacing: -1px; white-space: nowrap; }
  p { font-size: 32px; margin: 0 0 12px; color: #5b6475; white-space: nowrap; }
  .bar { position: absolute; left: 0; right: 0; bottom: 0; height: 24px; background: #2357d6; }
</style></head><body>
  <div class="card">
    <svg class="mark" viewBox="0 0 16 16"><rect width="16" height="16" rx="3" fill="#2357d6"/><path d="M4 11h8M4 8h5M4 5h8" stroke="#fff" stroke-width="1.6"/></svg>
    <div>
      <h1>Tacpendium-Setplay</h1>
      <p>SF6 setplay suggestions (unofficial)</p>
      <p>Knockdown advantage &#8594; okizeme recipes</p>
    </div>
  </div>
  <div class="bar"></div>
</body></html>`;

const browser = await chromium.launch(process.env.PW_CHROMIUM_PATH ? { executablePath: process.env.PW_CHROMIUM_PATH } : {});
try {
  const page = await browser.newPage({ viewport: { width: WIDTH, height: HEIGHT } });
  await page.setContent(html);
  await page.screenshot({ path: out, type: "png" });
  console.log(`書いた: ${out}`);
} finally {
  await browser.close();
}
