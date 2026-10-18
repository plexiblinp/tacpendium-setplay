import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";

import { describe, expect, it } from "vitest";

import { checkLimits, listFiles, MAX_FILE_BYTES, MAX_FILES } from "./dist-limits.mjs";
import { headersFor, parseHeaders } from "./headers.mjs";

const WEB = path.join(path.dirname(fileURLToPath(import.meta.url)), "..");

describe("配信の上限(dist 全体)", () => {
  it("1 件だけ超える入力で 1 件と出る・ちょうどは 0 件", () => {
    expect(checkLimits([["a", MAX_FILE_BYTES]])).toEqual([]);
    expect(checkLimits([["a", MAX_FILE_BYTES + 1], ["b", 1]])).toHaveLength(1);
    const many = Array.from({ length: MAX_FILES + 1 }, (_, i) => [`f${i}`, 1]);
    expect(checkLimits(many)).toHaveLength(1);
    expect(checkLimits(many.slice(1))).toEqual([]);
  });
  it("入れ子のファイルを数え、_headers は数えない", () => {
    const d = fs.mkdtempSync(path.join(os.tmpdir(), "dist-"));
    try {
      fs.mkdirSync(path.join(d, "data/moves"), { recursive: true });
      fs.writeFileSync(path.join(d, "index.html"), "x");
      fs.writeFileSync(path.join(d, "data/moves/a.json"), "xyz");
      fs.writeFileSync(path.join(d, "_headers"), "/*\n  X: y\n");
      expect(listFiles(d).sort()).toEqual([["data/moves/a.json", 3], ["index.html", 1]]);
    } finally {
      fs.rmSync(d, { recursive: true, force: true });
    }
  });
});

describe("_headers", () => {
  const rules = parseHeaders(fs.readFileSync(path.join(WEB, "public/_headers"), "utf8"));
  const get = (p, name) => headersFor(rules, p).find(([k]) => k.toLowerCase() === name.toLowerCase())?.[1];

  it("全パスに HANDOVER §7.4 のヘッダ", () => {
    for (const p of ["/", "/index.html", "/assets/index-x.js", "/data/moves/ryu.x.json", "/wasm/setplay.wasm"]) {
      const csp = get(p, "Content-Security-Policy") ?? "";
      expect(csp, p).toContain("default-src 'self'");
      expect(csp, p).toContain("script-src 'self' 'wasm-unsafe-eval'");
      expect(csp, p).toContain("connect-src 'self'");
      expect(csp, p).toContain("frame-ancestors 'none'");
      expect(csp, p).not.toContain("unsafe-inline");
      expect(csp, p).not.toMatch(/'unsafe-eval'/);
      expect(get(p, "X-Content-Type-Options"), p).toBe("nosniff");
      expect(get(p, "Referrer-Policy"), p).toBe("strict-origin-when-cross-origin");
      // S7: 外部のヘッダ診断で減点される項目。
      expect(get(p, "X-Frame-Options"), p).toBe("DENY");
      expect(get(p, "Cross-Origin-Resource-Policy"), p).toBe("same-origin");
      expect(get(p, "Strict-Transport-Security"), p).toMatch(/^max-age=\d{8,}$/);
    }
  });
  it("Permissions-Policy は clipboard-write=(self) 以外を拒否", () => {
    const pp = get("/", "Permissions-Policy") ?? "";
    const allowed = pp.split(",").map((s) => s.trim()).filter((s) => !s.endsWith("=()"));
    expect(allowed).toEqual(["clipboard-write=(self)"]);
  });
  it("ハッシュ入りの名前だけ immutable。manifest と wasm は再検証", () => {
    expect(get("/data/moves/ryu.0123456789abcdef.json", "Cache-Control")).toContain("immutable");
    expect(get("/assets/index-abc.js", "Cache-Control")).toContain("immutable");
    expect(get("/data/manifest.json", "Cache-Control")).toBe("no-cache");
    expect(get("/wasm/setplay.wasm", "Cache-Control")).toBe("no-cache");
    expect(get("/index.html", "Cache-Control")).toBeUndefined();
  });
  it("同じ名前のヘッダは連結する", () => {
    const r = parseHeaders("/*\n  X-A: 1\n/x\n  x-a: 2\n");
    expect(headersFor(r, "/x")).toEqual([["X-A", "1, 2"]]);
    expect(headersFor(r, "/y")).toEqual([["X-A", "1"]]);
  });
  it("壊れた形は誤り", () => {
    expect(() => parseHeaders("  X: y\n")).toThrow();
    expect(() => parseHeaders("nopath\n  X: y\n")).toThrow();
    expect(() => parseHeaders("/*\n  no colon\n")).toThrow();
  });
});
