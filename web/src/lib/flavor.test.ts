import { describe, expect, it } from "vitest";

import { KA_MAX_LOCAL, KA_MAX_SITE, kaMaxFor } from "./constants";
import { FLAVOR, flavorFromMode, KA_MAX, LOCAL_BUILD_MODE } from "./flavor";

describe("ビルドの種類(S8)", () => {
  it("mode が standalone のときだけローカル版。mode が無い(import.meta.env の無い束ね)ときはサイト", () => {
    expect(flavorFromMode(LOCAL_BUILD_MODE)).toBe("local");
    expect(flavorFromMode("production")).toBe("site");
    expect(flavorFromMode("test")).toBe("site");
    expect(flavorFromMode(undefined)).toBe("site");
  });

  it("KA の上限: サイト 100、ローカル版はエンジンの受ける最大(int32)", () => {
    expect(kaMaxFor("site")).toBe(KA_MAX_SITE);
    expect(KA_MAX_SITE).toBe(100);
    expect(kaMaxFor("local")).toBe(KA_MAX_LOCAL);
    expect(KA_MAX_LOCAL).toBe(2 ** 31 - 1);
  });

  it("試験(mode test)のビルドはサイト", () => {
    expect(FLAVOR).toBe("site");
    expect(KA_MAX).toBe(KA_MAX_SITE);
  });
});
