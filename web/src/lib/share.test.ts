import { describe, expect, it } from "vitest";

import { defaultConditions, parseSearch } from "./conditions";
import { LANGS, RESOURCES } from "./i18n";
import { characterDisplayName, shareModeKey, shareText, shareUrl, siteUrl, xIntentUrl } from "./share";

describe("shareUrl", () => {
  it("origin と path に条件のクエリを付ける(元のクエリ・ハッシュは捨てる)", () => {
    const c = { ...defaultConditions("ryu", 40), mode: "gap" as const };
    expect(shareUrl("https://example.test/app/?c=ken&ka=1#privacy", c)).toBe("https://example.test/app/?c=ryu&ka=40&mode=gap");
  });
  it("共有した URL を開くと同じ条件になる(往復)", () => {
    const c = { ...defaultConditions("ryu", 40), chainCancelOnly: true, sort: "target" as const };
    const u = new URL(shareUrl("https://example.test/", c));
    expect(parseSearch(u.search, new Set(["ryu"])).conditions).toEqual(c);
  });
});

describe("shareText・characterDisplayName", () => {
  it("文の後ろにアプリ名を付ける", () => {
    expect(shareText("リュウ KA 43 の重ね")).toBe("リュウ KA 43 の重ね — Tacpendium-Setplay");
  });
  it("ja は日本語名、ほかは英語名", () => {
    const name = { ja: "リュウ", en: "Ryu" };
    expect(characterDisplayName(name, "ja")).toBe("リュウ");
    for (const l of ["en", "ko", "zh-Hans", "zh-Hant"] as const) expect(characterDisplayName(name, l)).toBe("Ryu");
  });
});

describe("xIntentUrl", () => {
  it("x.com の投稿画面に text と url を符号化して渡す", () => {
    const url = "https://example.test/?c=ryu&ka=40";
    const s = xIntentUrl("リュウ KA 40 & more | Tacpendium-Setplay", url);
    const u = new URL(s);
    expect(u.origin + u.pathname).toBe("https://x.com/intent/post");
    expect(u.searchParams.get("text")).toBe("リュウ KA 40 & more | Tacpendium-Setplay");
    expect(u.searchParams.get("url")).toBe(url);
    // 共有する URL の & がクエリを割らない(text・url の 2 つだけ)。
    expect([...u.searchParams.keys()]).toEqual(["text", "url"]);
  });
});

describe("shareModeKey・siteUrl", () => {
  it("安全飛びは種類、それ以外はモードのキー", () => {
    expect(shareModeKey({ mode: "meaty", safeJump: null })).toBe("share.mode.meaty");
    expect(shareModeKey({ mode: "gap", safeJump: null })).toBe("share.mode.gap");
    expect(shareModeKey({ mode: "filler", safeJump: null })).toBe("share.mode.filler");
    expect(shareModeKey({ mode: "filler", safeJump: "f5" })).toBe("setplay.safeJumpKind.f5");
  });
  it("サイトのトップは origin と path だけ(条件・ハッシュを落とす)", () => {
    expect(siteUrl("https://example.test/app/?c=ryu&ka=40#privacy")).toBe("https://example.test/app/");
  });
});

// X の文字数の数え方(weighted length): URL は 23、CJK 等は 2、ほかは 1。上限 280。
const X_MAX_WEIGHT = 280;
const X_URL_WEIGHT = 23;
function xWeight(text: string): number {
  let n = 0;
  for (const ch of text) n += /[\u1100-\u11ff\u2e80-\u9fff\uac00-\ud7af\uf900-\ufaff\uff00-\uffef]/.test(ch) ? 2 : 1;
  return n + 1 + X_URL_WEIGHT;
}
const get = (o: unknown, key: string): string => key.split(".").reduce<unknown>((x, p) => (x as Record<string, unknown>)[p], o) as string;
const fill = (s: string, v: Record<string, string>) => s.replace(/\{\{(\w+)\}\}/g, (_, k: string) => v[k] ?? "");

describe("X の文字数(全ロケール・長い条件)", () => {
  it("1 件だけ長すぎる文を数えると超える(数え方の確かめ)", () => {
    expect(xWeight("あ".repeat(130))).toBeGreaterThan(X_MAX_WEIGHT);
  });
  it("結果の共有・サイトの共有とも、URL を含めて 280 以内", () => {
    const longName = "M".repeat(20); // いまのキャラ名より長い名前で見る
    const modes = ["share.mode.meaty", "share.mode.gap", "share.mode.filler", "setplay.safeJumpKind.true", "setplay.safeJumpKind.f5"];
    for (const l of LANGS) {
      const r = RESOURCES[l];
      for (const m of modes) {
        const text = shareText(fill(get(r, "share.text"), { character: longName, ka: "100", mode: get(r, m) }));
        expect(xWeight(text), `${l} ${m}`).toBeLessThanOrEqual(X_MAX_WEIGHT);
      }
      expect(xWeight(shareText(get(r, "share.appText"))), l).toBeLessThanOrEqual(X_MAX_WEIGHT);
    }
  });
});
