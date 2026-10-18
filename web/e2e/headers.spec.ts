// 実際に配ったときのセキュリティのヘッダ(S7。HANDOVER §7.4・§13)。scripts/serve.mjs は public/_headers を静的アセットに、
// /api は Worker の本物のコード(edge/。API_SECURITY_HEADERS)を通すので、両方の経路をここで 1 本ずつ確かめる(AGENTS §8-5)。
// ★外部のヘッダ診断(公開後)の代わりではない。手順は docs/release-checklist.md。
import { expect, test, type APIResponse } from "@playwright/test";

const COMMON: Record<string, string | RegExp> = {
  "x-content-type-options": "nosniff",
  "referrer-policy": "strict-origin-when-cross-origin",
  "x-frame-options": "DENY",
  "cross-origin-resource-policy": "same-origin",
  "strict-transport-security": /^max-age=\d{8,}$/,
  "content-security-policy": /frame-ancestors 'none'/,
};

function expectCommon(r: APIResponse, label: string): void {
  const h = r.headers();
  for (const [k, v] of Object.entries(COMMON)) {
    if (typeof v === "string") expect(h[k], `${label} ${k}`).toBe(v);
    else expect(h[k] ?? "", `${label} ${k}`).toMatch(v);
  }
}

test("静的アセット(画面・manifest・WASM)に CSP と診断のヘッダが付く", async ({ request }) => {
  for (const path of ["/", "/data/manifest.json", "/wasm/setplay.wasm"]) {
    const r = await request.get(path);
    expect(r.status(), path).toBe(200);
    expectCommon(r, path);
    expect(r.headers()["content-security-policy"], path).toContain("script-src 'self' 'wasm-unsafe-eval'");
    expect(r.headers()["permissions-policy"] ?? "", path).toContain("camera=()");
  }
});

test("/api(Worker)の応答にも付く(成功・204・誤り)", async ({ request }) => {
  const votes = await request.get("/api/votes?char=ryu&ka=1");
  const badBeacon = await request.post("/api/beacon", { data: "{", headers: { "content-type": "text/plain" } });
  const missing = await request.get("/api/nope");
  expect([votes.status(), badBeacon.status(), missing.status()]).toEqual([200, 400, 404]);
  for (const [label, r] of [["votes", votes], ["beacon 400", badBeacon], ["404", missing]] as const) {
    expectCommon(r, label);
    expect(r.headers()["content-security-policy"], label).toContain("default-src 'none'");
  }
});
