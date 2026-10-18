import { describe, expect, it } from "vitest";

import { filterCharacters, matchesWordPrefix, normalizeForSearch } from "./characterSearch";
import type { ManifestCharacter } from "./data";

function ch(code: string, ja: string, en: string): ManifestCharacter {
  return { code, name: { ja, en }, dataVersion: "v", calcFingerprint: "f", safeJump: null, movesBytes: 1, moves: "m" } as unknown as ManifestCharacter;
}

const CHARS = [
  ch("ryu", "リュウ", "Ryu"),
  ch("terry", "テリー", "Terry"),
  ch("c_viper", "C.ヴァイパー", "C. Viper"),
  ch("e_honda", "E.本田", "E. Honda"),
  ch("rashid", "ラシード", "Rashid"),
];
const codes = (q: string) => filterCharacters(CHARS, q).map((c) => c.code);

describe("キャラの名前検索(本家の規則)", () => {
  it("英字・かな(カタカナ)・code の語の先頭一致", () => {
    expect(codes("ry")).toEqual(["ryu"]);
    expect(codes("リ")).toEqual(["ryu"]);
    expect(codes("r")).toEqual(["ryu", "rashid"]);
  });
  it("途中一致は当てない(ry で terry を出さない・リ で テリー を出さない)", () => {
    expect(codes("ry")).not.toContain("terry");
    expect(codes("リ")).not.toContain("terry");
  });
  it("区切り(. _ 空白 ・)の後の語にも当たる", () => {
    expect(codes("viper")).toEqual(["c_viper"]);
    expect(codes("ヴァイパー")).toEqual(["c_viper"]);
    expect(codes("本田")).toEqual(["e_honda"]);
    expect(codes("honda")).toEqual(["e_honda"]);
  });
  it("全角・大文字・前後の空白は正規化する。空は全件", () => {
    expect(codes(" ＲＹ ")).toEqual(["ryu"]);
    expect(codes("")).toEqual(CHARS.map((c) => c.code));
    expect(normalizeForSearch(" ＡＢ ")).toBe("ab");
  });
  it("一致なしは空", () => {
    expect(codes("zzz")).toEqual([]);
    expect(matchesWordPrefix("Ryu", "yu")).toBe(false);
  });
});
