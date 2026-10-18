// edge/(Worker)の import の経路に src/lib/flavor.ts が入らないこと(S8 レビュー高-1)。
// flavor.ts は import.meta.env を読む。wrangler の束ねには import.meta.env が無いので、Worker からは読まない。
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

import { describe, expect, it } from "vitest";

const WEB = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const FLAVOR = path.join(WEB, "src/lib/flavor.ts");

/** 相対の import(型だけの import を除く)を辿り、届くファイルの集合を返す。 */
export function reachable(entry: string): Set<string> {
  const seen = new Set<string>();
  const stack = [entry];
  while (stack.length > 0) {
    const file = stack.pop()!;
    if (seen.has(file)) continue;
    seen.add(file);
    const src = fs.readFileSync(file, "utf8");
    const specs: string[] = [];
    for (const m of src.matchAll(/^\s*(import|export)\s+(type\s+)?[^;]*?from\s+"(\.[^"]+)"/gms)) {
      if (!m[2]) specs.push(m[3]!); // import type は実行時に読まない
    }
    // 副作用だけの import "./x" と、動的な import("./x")(再レビュー低-B)。
    for (const m of src.matchAll(/^\s*import\s+"(\.[^"]+)"/gm)) specs.push(m[1]!);
    for (const m of src.matchAll(/\bimport\(\s*"(\.[^"]+)"\s*\)/g)) specs.push(m[1]!);
    for (const spec of specs) {
      const base = path.resolve(path.dirname(file), spec);
      const cand = [base, `${base}.ts`, `${base}.tsx`, path.join(base, "index.ts")].find((c) => fs.existsSync(c) && fs.statSync(c).isFile());
      if (cand != null && /\.tsx?$/.test(cand)) stack.push(cand);
    }
  }
  return seen;
}

describe("edge の import の経路", () => {
  it("Worker の入口から flavor.ts に届かない", () => {
    const files = reachable(path.join(WEB, "edge/index.ts"));
    expect(files.has(path.join(WEB, "src/shared/api.ts"))).toBe(true); // 経路を辿れていること
    expect([...files].filter((f) => f === FLAVOR)).toEqual([]);
  });
  it("画面の入口からは flavor.ts に届く(辿り方の確認)", () => {
    expect(reachable(path.join(WEB, "src/main.tsx")).has(FLAVOR)).toBe(true);
  });
});
