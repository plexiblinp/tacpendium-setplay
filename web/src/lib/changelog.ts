// 更新履歴(開発者の要望 2026-10-25)。1 回のデプロイにつき 1 件・1 行の要約(長くなるなら目玉だけ)。
// 文は全ロケールの `changelog.items.<key>`(AGENTS §8-9)。足し方は docs/maintenance.md「更新履歴」。
// ★日付はサイトに出る日(マージしてデプロイする日の見込み)。新しいものを先頭に足す。

export interface ChangelogEntry {
  /** YYYY-MM-DD */
  date: string;
  /** 文言のキー(`changelog.items.<key>`)。 */
  key: string;
}

/** 画面に出す件数(これより古いものは出さない)。 */
export const CHANGELOG_SHOWN = 10;

export const CHANGELOG: readonly ChangelogEntry[] = [{ date: "2026-10-25", key: "meatyAdvantage" }];

/** 新しい順に並べてから CHANGELOG_SHOWN 件で切る(AGENTS §4。並べ替えの後で切る)。同じ日は書いた順。 */
export function recentChanges(entries: readonly ChangelogEntry[] = CHANGELOG, limit = CHANGELOG_SHOWN): ChangelogEntry[] {
  return entries
    .map((e, i) => ({ e, i }))
    .sort((a, b) => (a.e.date === b.e.date ? a.i - b.i : a.e.date < b.e.date ? 1 : -1))
    .slice(0, limit)
    .map(({ e }) => e);
}
