// KA(ダウン後の有利フレーム)の欄の「これは何？」: 押すと測り方・図・例と「例で試す」を開く(開発者 2026-11-07)。
// KA は当て方で変わるので技ごとの値は持たない。入れるのは利用者で、ここは測り方を説明するだけ。
import { useId, useState } from "react";
import { useTranslation } from "react-i18next";

import { KA_FIGURE_EXAMPLE } from "../lib/constants";
import { KaFigure } from "./KaFigure";

const STEPS = ["step1", "step2", "step3"] as const;

interface Props {
  /** 「例で試す」。例のキャラが配信に無いときは undefined(ボタンを出さない)。 */
  onTrial?: () => void;
  /** 計算中は「例で試す」を押せない。 */
  running: boolean;
}

export function KaGuide({ onTrial, running }: Props) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const panelId = useId();
  const example = { combo: KA_FIGURE_EXAMPLE.combo, ka: KA_FIGURE_EXAMPLE.ka, meaty: KA_FIGURE_EXAMPLE.meaty };
  return (
    <div className="ka-guide">
      <button type="button" className="ka-what" aria-expanded={open} aria-controls={panelId} onClick={() => setOpen((o) => !o)} data-testid="ka-what">
        {t("kaGuide.button")}
      </button>
      {open && (
        <div id={panelId} className="ka-guide-panel" data-testid="ka-guide">
          <ol>
            {STEPS.map((k) => (
              <li key={k}>{t(`kaGuide.${k}`)}</li>
            ))}
          </ol>
          <KaFigure
            labels={{
              self: t("kaGuide.figureSelf"),
              opponent: t("kaGuide.figureOpponent"),
              here: t("kaGuide.figureHere"),
              alt: t("kaGuide.figureAlt", { ka: KA_FIGURE_EXAMPLE.ka }),
            }}
          />
          <p className="ka-example" data-testid="ka-example">
            {t("kaGuide.example", example)}
          </p>
          {onTrial != null && (
            <button type="button" className="secondary" onClick={onTrial} disabled={running} data-testid="ka-trial">
              {t("kaGuide.trial")}
            </button>
          )}
        </div>
      )}
    </div>
  );
}
