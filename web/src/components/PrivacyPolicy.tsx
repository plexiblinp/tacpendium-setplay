// プライバシーポリシー(HANDOVER §7.3・D14)。開発者が依頼したレビューの修正案(2026-10-17)の文面。
// 位置づけは信頼のための説明(レビューの結論: 法的には問題なし)。全項目の一覧は docs/telemetry.md。
import { useTranslation } from "react-i18next";

import { FEEDBACK_FORM_URL, ISSUES_URL } from "../lib/constants";
import type { Lang } from "../lib/langs";

/** 直接開くための URL の #(フッターのリンク先。ストレージを使わずに共有できる)。 */
export const PRIVACY_HASH = "#privacy";

/** 最終更新日(UTC の日付)。文面を変えたら更新する。★公開時(S7〜S9)に公開日へ揃える。 */
export const PRIVACY_LAST_UPDATED = "2026-10-18";

const SECTIONS = ["hash", "retention", "thirdParty", "contact"] as const;

export function PrivacyPolicy({ onClose, lang }: { onClose: () => void; lang: Lang }) {
  const { t } = useTranslation();
  const date = new Intl.DateTimeFormat(lang, { dateStyle: "long", timeZone: "UTC" }).format(new Date(`${PRIVACY_LAST_UPDATED}T00:00:00Z`));
  return (
    <section className="privacy" id="privacy" aria-labelledby="privacy-title" data-testid="privacy">
      <h2 id="privacy-title">{t("privacy.title")}</h2>
      <p>{t("privacy.intro")}</p>
      <h3>{t("privacy.collectTitle")}</h3>
      <ul>
        <li>{t("privacy.usageItem")}</li>
        <li>{t("privacy.votesItem")}</li>
      </ul>
      {SECTIONS.map((k) => (
        <div key={k}>
          <h3>{t(`privacy.${k}Title`)}</h3>
          <p>{t(`privacy.${k}Body`)}</p>
        </div>
      ))}
      <p>
        <a href={ISSUES_URL} target="_blank" rel="noopener noreferrer">
          {ISSUES_URL}
        </a>
      </p>
      {FEEDBACK_FORM_URL !== "" && (
        <p data-testid="privacy-form">
          {t("privacy.contactFormBody")}{" "}
          <a href={FEEDBACK_FORM_URL} target="_blank" rel="noopener noreferrer">
            {FEEDBACK_FORM_URL}
          </a>
        </p>
      )}
      <p className="muted small" data-testid="privacy-updated">
        {t("privacy.lastUpdated", { date })}
      </p>
      <button type="button" className="secondary" onClick={onClose} data-testid="privacy-close">
        {t("privacy.close")}
      </button>
    </section>
  );
}
