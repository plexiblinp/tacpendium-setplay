// 更新履歴(開発者の要望 2026-10-25)。いちばん新しい 1 件だけを薄く出し、押すと直近 CHANGELOG_SHOWN 件を開く。
import { useTranslation } from "react-i18next";

import { CHANGELOG_SHOWN, recentChanges } from "../lib/changelog";

export function Changelog() {
  const { t } = useTranslation();
  const entries = recentChanges();
  const latest = entries[0];
  if (latest == null) return null;
  const line = (key: string) => t(`changelog.items.${key}`);
  return (
    <details className="changelog muted small" data-testid="changelog">
      <summary data-testid="changelog-latest">{t("changelog.latest", { date: latest.date, text: line(latest.key) })}</summary>
      <p className="changelog-title">{t("changelog.title", { n: CHANGELOG_SHOWN })}</p>
      <ul>
        {entries.map((e) => (
          <li key={e.key} data-testid="changelog-item">
            <time dateTime={e.date}>{e.date}</time> {line(e.key)}
          </li>
        ))}
      </ul>
    </details>
  );
}
