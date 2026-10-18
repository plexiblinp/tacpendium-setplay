// 画面冒頭の説明と、使い方・注意と対策・レシピのルール(S7。開発者の案を要約。旧「提案の制約について」を置き換えた)。
// ★「注意と対策」「レシピのルール」はエンジンの規則(連打キャンセル・ラッシュ・target の列挙)と対応している。
//   規則を変えたら文も直す(載せなかった旧項目と規則の対応は docs/frontend.md「使い方と注意」)。
import { useTranslation } from "react-i18next";

import { DATA_SOURCE_URL } from "../lib/constants";

const STEPS = ["step1", "step2", "step3"] as const;
const TIPS = ["tipVote", "tipHide"] as const;
/** 注意と、その対策(「→」で続ける)。 */
const CAUTIONS = [
  ["distance", "distanceFix"],
  ["rush", "rushFix"],
  ["multiHit", "ignoreFix"],
  ["projectile", "projectileFix"],
  ["notOverlapping", "ignoreFix"],
] as const;
const RULES = ["ruleFastest", "ruleChain", "ruleExcluded", "ruleStance", "ruleNewCharacters"] as const;

export function Guide() {
  const { t } = useTranslation();
  return (
    <div className="guide" data-testid="guide">
      <p className="lead">
        {t("guide.lead1")}
        <br />
        {t("guide.lead2")}
      </p>
      <p className="lead small" data-testid="guide-upstream">
        {t("guide.upstreamAbout")}
        <br />
        {t("guide.upstreamMerits")}{" "}
        <a href={DATA_SOURCE_URL} target="_blank" rel="noopener noreferrer">
          {t("guide.upstreamLink")}
        </a>
        <br />
        {t("guide.upstreamLanguages")}
      </p>
      <details data-testid="guide-howto">
        <summary>{t("guide.howToTitle")}</summary>
        <ol>
          {STEPS.map((k) => (
            <li key={k}>{t(`guide.${k}`)}</li>
          ))}
        </ol>
        <ul>
          {TIPS.map((k) => (
            <li key={k}>{t(`guide.${k}`)}</li>
          ))}
        </ul>
      </details>
      <details data-testid="guide-cautions">
        <summary>{t("guide.cautionsTitle")}</summary>
        <ul>
          {CAUTIONS.map(([issue, fix]) => (
            <li key={issue}>
              {t(`guide.${issue}`)}
              <br />→ {t(`guide.${fix}`)}
            </li>
          ))}
        </ul>
      </details>
      <details data-testid="guide-rules">
        <summary>{t("guide.rulesTitle")}</summary>
        <ul>
          {RULES.map((k) => (
            <li key={k}>{t(`guide.${k}`)}</li>
          ))}
        </ul>
      </details>
    </div>
  );
}
