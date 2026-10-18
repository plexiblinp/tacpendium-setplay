import { describe, expect, it } from "vitest";

import { detectLang, LANGS, RESOURCES } from "./i18n";

function keys(obj: unknown, prefix = ""): string[] {
  if (obj === null || typeof obj !== "object") return [prefix];
  return Object.entries(obj as Record<string, unknown>).flatMap(([k, v]) => keys(v, prefix ? `${prefix}.${k}` : k));
}
function values(obj: unknown): string[] {
  if (typeof obj === "string") return [obj];
  if (obj === null || typeof obj !== "object") return [];
  return Object.values(obj as Record<string, unknown>).flatMap(values);
}

// かな(ひらがな・カタカナ・半角カナ。カタカナの中黒「・」も含む)。
const KANA = /[぀-ヿㇰ-ㇿｦ-ﾟ]/;
// 漢字(CJK 統合漢字)。中国語では使うので、en・ko だけで見る。
const HAN = /[㐀-䶿一-鿿]/;

describe("ロケール(AGENTS §8-9)", () => {
  it("全ロケールのキー集合が ja と一致する", () => {
    const ja = keys(RESOURCES.ja).sort();
    for (const l of LANGS) expect(keys(RESOURCES[l]).sort(), l).toEqual(ja);
  });
  it("空の値が無い", () => {
    for (const l of LANGS) for (const v of values(RESOURCES[l])) expect(v.trim(), l).not.toBe("");
  });
  it("差し込み({{x}})の名前が ja と同じ", () => {
    const vars = (s: string) => [...s.matchAll(/\{\{(\w+)\}\}/g)].map((m) => m[1]).sort();
    const ja = RESOURCES.ja as unknown as Record<string, unknown>;
    for (const l of LANGS) {
      const loc = RESOURCES[l] as unknown as Record<string, unknown>;
      for (const k of keys(ja)) {
        const get = (o: Record<string, unknown>) => k.split(".").reduce<unknown>((x, p) => (x as Record<string, unknown>)[p], o) as string;
        expect(vars(get(loc)), `${l} ${k}`).toEqual(vars(get(ja)));
      }
    }
  });
  it("日本語以外のロケールに日本語(かな)が混じっていない。en・ko には漢字も無い", () => {
    for (const l of LANGS.filter((x) => x !== "ja")) {
      for (const v of values(RESOURCES[l])) {
        expect(KANA.test(v), `${l}: ${v}`).toBe(false);
        if (l === "en" || l === "ko") expect(HAN.test(v), `${l}: ${v}`).toBe(false);
      }
    }
  });
  it("ja の値は en と別物(訳し忘れでキーだけ揃えていない)", () => {
    const ja = values(RESOURCES.ja);
    const en = values(RESOURCES.en);
    const same = ja.filter((v, i) => v === en[i] && /[a-z]{3,}/i.test(v) && !/^[A-Z0-9 .()+{}]+$/.test(v));
    expect(same).toEqual([]);
  });
});

describe("detectLang(HANDOVER §6)", () => {
  it.each([
    [["ja-JP"], "ja"],
    [["en-US"], "en"],
    [["ko-KR"], "ko"],
    [["zh-CN"], "zh-Hans"],
    [["zh-SG"], "zh-Hans"],
    [["zh"], "zh-Hans"],
    [["zh-TW"], "zh-Hant"],
    [["zh-HK"], "zh-Hant"],
    [["zh-MO"], "zh-Hant"],
    [["zh-Hant-TW"], "zh-Hant"],
    [["zh-Hans-HK"], "zh-Hans"],
    [["fr-FR", "ko"], "ko"],
    [["fr-FR"], "en"],
    [[], "en"],
  ])("%j → %s", (langs, want) => {
    expect(detectLang(langs)).toBe(want);
  });
});
