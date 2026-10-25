import { describe, expect, it } from "vitest";

import { CHANGELOG, CHANGELOG_SHOWN, recentChanges } from "./changelog";
import { LANGS, RESOURCES } from "./i18n";

describe("更新履歴", () => {
  it("日付は YYYY-MM-DD・キーは重ならない・全ロケールに文がある", () => {
    expect(CHANGELOG.length).toBeGreaterThan(0);
    expect(new Set(CHANGELOG.map((e) => e.key)).size).toBe(CHANGELOG.length);
    for (const e of CHANGELOG) {
      expect(e.date).toMatch(/^\d{4}-\d{2}-\d{2}$/);
      expect(Number.isNaN(Date.parse(`${e.date}T00:00:00Z`)), e.date).toBe(false);
      for (const l of LANGS) {
        const items = (RESOURCES[l] as unknown as { changelog: { items: Record<string, string> } }).changelog.items;
        expect(items[e.key], `${l} ${e.key}`).toBeTruthy();
      }
    }
  });
  it("ロケールの文は CHANGELOG のキーと過不足なく一致する(消し忘れ・足し忘れ)", () => {
    const items = (RESOURCES.ja as unknown as { changelog: { items: Record<string, string> } }).changelog.items;
    expect(Object.keys(items).sort()).toEqual(CHANGELOG.map((e) => e.key).sort());
  });
  it("新しい順に並べてから切る(同じ日は書いた順)", () => {
    const entries = [
      { date: "2026-01-01", key: "a" },
      { date: "2026-03-01", key: "b" },
      { date: "2026-02-01", key: "c" },
      { date: "2026-03-01", key: "d" },
    ];
    expect(recentChanges(entries, 3).map((e) => e.key)).toEqual(["b", "d", "c"]);
  });
  it(`出すのは ${CHANGELOG_SHOWN} 件まで`, () => {
    const many = Array.from({ length: 15 }, (_, i) => ({ date: `2026-01-${String(i + 1).padStart(2, "0")}`, key: `k${i}` }));
    const shown = recentChanges(many);
    expect(shown).toHaveLength(CHANGELOG_SHOWN);
    expect(shown[0]!.key).toBe("k14");
  });
});
