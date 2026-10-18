import { describe, expect, it } from "vitest";

import { LruCache } from "./lru";

describe("LruCache", () => {
  it("上限を超えたら最も使っていないものを捨てる", () => {
    const c = new LruCache<string, number>(2);
    c.set("a", 1);
    c.set("b", 2);
    expect(c.get("a")).toBe(1); // a を新しくする
    c.set("c", 3);
    expect(c.get("b")).toBeUndefined();
    expect(c.get("a")).toBe(1);
    expect(c.get("c")).toBe(3);
    expect(c.size).toBe(2);
  });
  it("上限は 1 以上の整数", () => {
    expect(() => new LruCache(0)).toThrow();
    expect(() => new LruCache(1.5)).toThrow();
  });
});
