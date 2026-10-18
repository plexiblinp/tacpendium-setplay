import { describe, expect, it } from "vitest";

import { defaultConditions } from "./conditions";
import { KA_MAX_LOCAL } from "./constants";
import { formFromConditions, initialForm, safeJumpConditions, validateForm, type FormState } from "./formState";

const base = (): FormState => ({ ...formFromConditions(defaultConditions("ryu", 40)) });

describe("validateForm", () => {
  it("既定の欄はそのまま条件になる", () => {
    expect(validateForm(base())).toEqual({ conditions: defaultConditions("ryu", 40), errors: [] });
  });
  it("最初の欄はキャラ・KA が無いので押せない", () => {
    expect(validateForm(initialForm()).errors).toEqual(["characterRequired", "kaInvalid"]);
  });
  it("KA は 0〜100 の整数", () => {
    for (const k of ["-1", "101", "4.5", "", "abc"]) expect(validateForm({ ...base(), kaText: k }).errors).toEqual(["kaInvalid"]);
    expect(validateForm({ ...base(), kaText: "0" }).conditions?.ka).toBe(0);
    expect(validateForm({ ...base(), kaText: "100" }).conditions?.ka).toBe(100);
  });
  it("ローカル版は KA の上限を外す(エンジンの受ける最大まで)", () => {
    expect(validateForm({ ...base(), kaText: "101" }, KA_MAX_LOCAL).conditions?.ka).toBe(101);
    expect(validateForm({ ...base(), kaText: String(KA_MAX_LOCAL) }, KA_MAX_LOCAL).conditions?.ka).toBe(KA_MAX_LOCAL);
    expect(validateForm({ ...base(), kaText: String(KA_MAX_LOCAL + 1) }, KA_MAX_LOCAL).errors).toEqual(["kaInvalid"]);
    expect(validateForm({ ...base(), kaText: "-1" }, KA_MAX_LOCAL).errors).toEqual(["kaInvalid"]);
    expect(safeJumpConditions({ ...base(), kaText: "150" }, "f5", 42, KA_MAX_LOCAL).conditions?.ka).toBe(150);
    expect(safeJumpConditions({ ...base(), kaText: "150" }, "f5", 42).conditions).toBeNull();
  });
  it("いまのモードの欄だけを検査する", () => {
    const f = { ...base(), gMinText: "x", remainText: "" };
    expect(validateForm(f).errors).toEqual([]);
    expect(validateForm({ ...f, mode: "gap" }).errors).toEqual(["gapRangeInvalid"]);
    expect(validateForm({ ...f, mode: "filler" }).errors).toEqual(["fillerRemainRequired"]);
  });
  it("最大 < 最小は押せない(重ねる: 空欄の最大は上限なし)", () => {
    expect(validateForm({ ...base(), nMinText: "3", nMaxText: "2" }).errors).toEqual(["nMaxBelowMin"]);
    expect(validateForm({ ...base(), nMinText: "3", nMaxText: "" }).conditions?.nMax).toBe(0);
    expect(validateForm({ ...base(), mode: "gap", gMinText: "5", gMaxText: "4" }).errors).toEqual(["gapMaxBelowMin"]);
  });
  it("種別が 0 個なら押せない(特定の技を選んでいるとき・有利調整では問わない)", () => {
    const f = { ...base(), targetTypes: [] };
    expect(validateForm(f).errors).toEqual(["targetTypesRequired"]);
    expect(validateForm({ ...f, targetMove: "hadoken_light" }).errors).toEqual([]);
    expect(validateForm({ ...f, mode: "filler" }).errors).toEqual([]);
  });
  it("有利調整では特定の技を渡さない", () => {
    expect(validateForm({ ...base(), mode: "filler", targetMove: "hadoken_light" }).conditions?.targetMove).toBeNull();
  });
});

describe("safeJumpConditions", () => {
  it("有利調整を remain = 値で、種類つきで出す(欄の remain は使わない)", () => {
    const r = safeJumpConditions({ ...base(), remainText: "" }, "f5", 42);
    expect(r.conditions).toMatchObject({ mode: "filler", remain: 42, safeJump: "f5", targetMove: null });
  });
  it("KA が誤りなら出さない", () => {
    expect(safeJumpConditions({ ...base(), kaText: "" }, "f5", 42).conditions).toBeNull();
  });
});
