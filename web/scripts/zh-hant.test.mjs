import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

import { describe, expect, it } from "vitest";

import { renderLocale, toTraditional } from "./zh-hant.mjs";

const dir = path.join(path.dirname(fileURLToPath(import.meta.url)), "../src/locales");

describe("zh-Hant(OpenCC s2twp で zh-Hans から生成。HANDOVER §6)", () => {
  it("コミット済みの zh-Hant.json が zh-Hans.json からの再生成と一致する(手で直していない・生成し忘れていない)", () => {
    const hans = JSON.parse(fs.readFileSync(path.join(dir, "zh-Hans.json"), "utf8"));
    expect(fs.readFileSync(path.join(dir, "zh-Hant.json"), "utf8")).toBe(renderLocale(toTraditional(hans)));
  });
  it("台湾の語彙に変える(s2twp)・差し込みの名前は変えない", () => {
    expect(toTraditional("加载数据")).toBe("載入資料");
    expect(toTraditional({ a: ["软件 {{设置}}"] })).toEqual({ a: ["軟體 {{设置}}"] });
  });
});
