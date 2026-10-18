// フッター: データ版・ソースへのリンク(AGPL)・データのクレジット(CC BY-SA)・非公式の明記(HANDOVER §8)。
import { useTranslation } from "react-i18next";

import { DATA_LICENSE_URL, DATA_SOURCE_URL, SOURCE_URL } from "../lib/constants";
import { PRIVACY_HASH } from "./PrivacyPolicy";

/** データ版の表示の長さ(取込元の SHA の先頭)。 */
const SHORT_SHA = 7;

export function Footer({ dataVersion, onPrivacy }: { dataVersion: string; onPrivacy: () => void }) {
  const { t } = useTranslation();
  return (
    <footer className="site-footer">
      <p data-testid="data-version">{t("footer.dataVersion", { version: dataVersion.slice(0, SHORT_SHA) })}</p>
      <p>
        <a href={SOURCE_URL} target="_blank" rel="noopener noreferrer">
          {t("footer.source")}
        </a>
      </p>
      <p>
        <a href={DATA_SOURCE_URL} target="_blank" rel="noopener noreferrer">
          {t("footer.dataCredit")}
        </a>{" "}
        <a href={DATA_LICENSE_URL} target="_blank" rel="noopener noreferrer">
          CC BY-SA 4.0
        </a>
      </p>
      <p>{t("footer.unofficial")}</p>
      <p>
        <a href={PRIVACY_HASH} onClick={onPrivacy} data-testid="privacy-link">
          {t("footer.privacy")}
        </a>
      </p>
    </footer>
  );
}
