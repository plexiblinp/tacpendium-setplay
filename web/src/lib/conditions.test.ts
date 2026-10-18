import { describe, expect, it } from "vitest";

import {
  cacheKey,
  conditionsToSearch,
  hasAppliedFilters,
  defaultConditions,
  parseIntInRange,
  parseSearch,
  toEngineQuery,
  type Conditions,
} from "./conditions";
import { KA_MAX_LOCAL } from "./constants";
import { formFromConditions, validateForm, type FormState } from "./formState";

const KNOWN = new Set(["ryu", "ken", "c_viper"]);
const q = (c: Conditions) => Object.fromEntries(new URLSearchParams(toEngineQuery(c)));

describe("toEngineQuery(本家の画面と同じ組み方)", () => {
  it("既定(重ねる)", () => {
    expect(q(defaultConditions("ryu", 40))).toEqual({
      n_min: "1",
      sort: "n",
      target_types: "normal,unique,special_projectile",
      include_zero_damage: "false",
      mode: "meaty",
      limit: "3000",
    });
  });
  it("n_max は重ねるで 0 以外のときだけ。g_* は汚連携だけ。remain は有利調整だけ", () => {
    const c = { ...defaultConditions("ryu", 40), nMax: 5, gMin: 2, gMax: 4, remain: 7 };
    expect(q(c).n_max).toBe("5");
    expect(q(c).g_min).toBeUndefined();
    expect(q(c).remain).toBeUndefined();
    const gap = q({ ...c, mode: "gap" });
    expect([gap.n_max, gap.g_min, gap.g_max, gap.remain]).toEqual([undefined, "2", "4", undefined]);
    const filler = q({ ...c, mode: "filler" });
    expect([filler.n_max, filler.g_min, filler.remain]).toEqual([undefined, undefined, "7"]);
  });
  it("絞り込みは true のときだけ。ガード時 0F 以上は重ねるだけ。安全飛びでは接近を掛けない", () => {
    const c = { ...defaultConditions("ryu", 40), approachOnly: true, blockSafeOnly: true, chainCancelOnly: true };
    expect(q(c)).toMatchObject({ approach_only: "true", block_safe_only: "true", chain_cancel_only: "true" });
    expect(q({ ...c, mode: "gap" }).block_safe_only).toBeUndefined();
    const sj = q({ ...c, mode: "filler", safeJump: "f5", remain: 42 });
    expect(sj.approach_only).toBeUndefined();
    expect(sj.chain_cancel_only).toBe("true");
  });
  it("特定の技は target_move(code)", () => {
    expect(q({ ...defaultConditions("ryu", 40), targetMove: "hadoken_light" }).target_move).toBe("hadoken_light");
  });
  it("同じ計算になる条件は同じ鍵、違えば違う鍵", () => {
    const a = defaultConditions("ryu", 40);
    expect(cacheKey(a)).toBe(cacheKey({ ...a, gMin: 3 })); // 重ねるでは g は送らない
    expect(cacheKey(a)).not.toBe(cacheKey({ ...a, ka: 41 }));
    expect(cacheKey(a)).not.toBe(cacheKey({ ...a, character: "ken" }));
  });
});

describe("URL(共有)", () => {
  it("往復で同じ条件に戻る", () => {
    const c: Conditions = {
      ...defaultConditions("c_viper", 63),
      mode: "gap",
      gMin: 2,
      gMax: 5,
      targetTypes: ["normal", "throw"],
      includeZeroDamage: true,
      targetMove: "standing_light_punch",
      sort: "target",
      approachOnly: true,
      chainCancelOnly: true,
      nMin: 2,
      nMax: 9,
      remain: 10,
    };
    expect(parseSearch(conditionsToSearch(c), KNOWN)).toEqual({ conditions: c, rejected: [] });
  });
  it("既定値は書かない", () => {
    expect(conditionsToSearch(defaultConditions("ryu", 40))).toBe("c=ryu&ka=40");
  });
  it("安全飛びは有利調整のときだけ読む", () => {
    expect(parseSearch("c=ryu&ka=60&mode=filler&sj=true", KNOWN).conditions?.safeJump).toBe("true");
    expect(parseSearch("c=ryu&ka=60&sj=true", KNOWN).conditions?.safeJump).toBeNull();
  });
  it("キャラ・KA が無い・誤り・知らないキャラなら条件にしない", () => {
    expect(parseSearch("", KNOWN).conditions).toBeNull();
    expect(parseSearch("c=ryu", KNOWN).conditions).toBeNull();
    expect(parseSearch("c=ryu&ka=101", KNOWN)).toEqual({ conditions: null, rejected: ["ka"] });
    expect(parseSearch("c=ryu&ka=-1", KNOWN).conditions).toBeNull();
    expect(parseSearch("c=ryu&ka=4.5", KNOWN).conditions).toBeNull();
    expect(parseSearch("c=nobody&ka=40", KNOWN)).toEqual({ conditions: null, rejected: ["c"] });
  });
  it("ほかのキーの誤りは、そのキーだけ既定値に戻して挙げる", () => {
    const r = parseSearch("c=ryu&ka=40&mode=xxx&g_min=0&types=normal,bogus&n_min=abc&move=../x&sort=n", KNOWN);
    expect(r.conditions).toEqual(defaultConditions("ryu", 40));
    expect(r.rejected.sort()).toEqual(["g_min", "mode", "move", "n_min", "types"]);
  });
  it("KA の境界(0 と 100)は受ける", () => {
    expect(parseSearch("c=ryu&ka=0", KNOWN).conditions?.ka).toBe(0);
    expect(parseSearch("c=ryu&ka=100", KNOWN).conditions?.ka).toBe(100);
  });
});

describe("parseIntInRange", () => {
  it("範囲内の整数だけ", () => {
    expect(parseIntInRange("3", 1)).toBe(3);
    expect(parseIntInRange(" 13 ", 1, 13)).toBe(13);
    for (const s of ["", "0", "14", "1.5", "-1", "a", "1e1"]) expect(parseIntInRange(s, 1, 13)).toBeNull();
  });
});

describe("URL の検査(画面の欄と揃える)", () => {
  it("最大 < 最小は最大を既定に戻して挙げる", () => {
    const r = parseSearch("c=ryu&ka=40&n_min=5&n_max=3", KNOWN);
    expect([r.conditions?.nMin, r.conditions?.nMax, r.rejected]).toEqual([5, 0, ["n_max"]]);
    const g = parseSearch("c=ryu&ka=40&mode=gap&g_min=5&g_max=3", KNOWN);
    expect([g.conditions?.gMin, g.conditions?.gMax, g.rejected]).toEqual([1, 13, ["g_min", "g_max"]]);
  });
  it("有利調整でない sj・重ねるでない block_safe は捨てて挙げる", () => {
    expect(parseSearch("c=ryu&ka=40&sj=f5", KNOWN).rejected).toEqual(["sj"]);
    const r = parseSearch("c=ryu&ka=40&mode=gap&block_safe=1", KNOWN);
    expect([r.conditions?.blockSafeOnly, r.rejected]).toEqual([false, ["block_safe"]]);
  });
  it("安全な整数を超える数は受けない(画面の欄と同じ)", () => {
    expect(parseSearch("c=ryu&ka=40&n_max=99999999999999999999", KNOWN).rejected).toEqual(["n_max"]);
    expect(parseSearch("c=ryu&ka=40&n_min=1000", KNOWN).rejected).toEqual([]);
  });
  it("種別 0 個は、特定の技か有利調整のときだけ受ける", () => {
    expect(parseSearch("c=ryu&ka=40&types=&move=x", KNOWN).conditions?.targetTypes).toEqual([]);
    expect(parseSearch("c=ryu&ka=40&types=&mode=filler", KNOWN).rejected).toEqual([]);
    const r = parseSearch("c=ryu&ka=40&types=", KNOWN);
    expect([r.conditions?.targetTypes, r.rejected]).toEqual([["normal", "unique", "special_projectile"], ["types"]]);
  });
  it("使わないモードの欄の前後関係は問わない(画面の欄と同じ)", () => {
    expect(parseSearch("c=ryu&ka=40&g_min=5&g_max=3", KNOWN).rejected).toEqual([]);
    expect(parseSearch("c=ryu&ka=40&mode=gap&n_min=5&n_max=3", KNOWN).rejected).toEqual([]);
  });
  it("画面で押せる条件は、共有して開いても同じ条件になる(往復・読めなかったキー無し)", () => {
    const base = formFromConditions(defaultConditions("ryu", 40));
    const forms: FormState[] = [
      { ...base, nMinText: "1000" },
      { ...base, mode: "filler", remainText: "150" },
      { ...base, gMinText: "5", gMaxText: "3" }, // 重ねるで、隠れた汚連携の欄が 5〜3
      { ...base, mode: "gap", nMinText: "5", nMaxText: "3" }, // 汚連携で、隠れた重ねるの欄が 5〜3
      { ...base, mode: "gap", blockSafeOnly: true, approachOnly: true },
      { ...base, mode: "filler", targetMove: "x", targetTypes: [] },
      { ...base, targetMove: "x", targetTypes: [] },
      { ...base, nMinText: "3", nMaxText: "", sort: "target", chainCancelOnly: true, includeZeroDamage: true },
    ];
    for (const f of forms) {
      const c = validateForm(f).conditions;
      expect(c, JSON.stringify(f)).not.toBeNull();
      if (c == null) continue;
      const r = parseSearch(conditionsToSearch(c), KNOWN);
      expect(r.rejected, JSON.stringify(f)).toEqual([]);
      // 計算に効く部分(エンジンのクエリ)が同じ。
      expect(r.conditions && toEngineQuery(r.conditions), JSON.stringify(f)).toBe(toEngineQuery(c));
    }
  });
  it("重ねるでないときは block_safe を URL に書かない", () => {
    expect(conditionsToSearch({ ...defaultConditions("ryu", 40), mode: "gap", blockSafeOnly: true })).toBe("c=ryu&ka=40&mode=gap");
  });
});

describe("hasAppliedFilters(エンジンに実際に送る絞り込み)", () => {
  const d = defaultConditions("ryu", 40);
  it("送らない絞り込みは数えない", () => {
    expect(hasAppliedFilters(d)).toBe(false);
    expect(hasAppliedFilters({ ...d, blockSafeOnly: true })).toBe(true);
    expect(hasAppliedFilters({ ...d, mode: "gap", blockSafeOnly: true })).toBe(false);
    expect(hasAppliedFilters({ ...d, mode: "filler", approachOnly: true })).toBe(true);
    expect(hasAppliedFilters({ ...d, mode: "filler", safeJump: "f5", approachOnly: true })).toBe(false);
    expect(hasAppliedFilters({ ...d, mode: "gap", chainCancelOnly: true })).toBe(true);
  });
  it("toEngineQuery と食い違わない", () => {
    for (const mode of ["meaty", "gap", "filler"] as const)
      for (const safeJump of [null, "f5"] as const)
        for (const [a, b, ch] of [[true, false, false], [false, true, false], [false, false, true], [false, false, false]]) {
          const c = { ...d, mode, safeJump: mode === "filler" ? safeJump : null, approachOnly: a!, blockSafeOnly: b!, chainCancelOnly: ch! };
          expect(hasAppliedFilters(c), JSON.stringify(c)).toBe(/approach_only|block_safe_only|chain_cancel_only/.test(toEngineQuery(c)));
        }
  });
});

describe("parseSearch の KA の上限(S8)", () => {
  it("サイト(既定)は 100 まで。超えたら共有の条件ではない(ka を捨てる)", () => {
    expect(parseSearch("c=ryu&ka=100", KNOWN).conditions?.ka).toBe(100);
    expect(parseSearch("c=ryu&ka=101", KNOWN)).toEqual({ conditions: null, rejected: ["ka"] });
  });
  it("ローカル版はエンジンの受ける最大まで", () => {
    expect(parseSearch("c=ryu&ka=101", KNOWN, KA_MAX_LOCAL).conditions?.ka).toBe(101);
    expect(parseSearch(`c=ryu&ka=${KA_MAX_LOCAL}`, KNOWN, KA_MAX_LOCAL).conditions?.ka).toBe(KA_MAX_LOCAL);
    expect(parseSearch(`c=ryu&ka=${KA_MAX_LOCAL + 1}`, KNOWN, KA_MAX_LOCAL).rejected).toEqual(["ka"]);
  });
});
