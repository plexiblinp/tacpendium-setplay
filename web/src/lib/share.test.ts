import { describe, expect, it } from "vitest";

import { defaultConditions, parseSearch } from "./conditions";
import { characterDisplayName, shareText, shareUrl, xIntentUrl } from "./share";

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
    expect(shareText("リュウ KA 40 のセットプレイ")).toBe("リュウ KA 40 のセットプレイ | Tacpendium-Setplay");
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
