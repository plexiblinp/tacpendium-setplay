// 言語と技の表記の切替(HANDOVER D5・D6)。
import { useTranslation } from "react-i18next";

import { FEEDBACK_URL } from "../lib/constants";
import { NOTATION_KINDS, type NotationKind } from "../lib/data";
import { LANG_NAMES, LANGS, MACHINE_TRANSLATED, type Lang } from "../lib/i18n";

interface Props {
  lang: Lang;
  onLang: (lang: Lang) => void;
  notation: NotationKind;
  onNotation: (n: NotationKind) => void;
}

export function SettingsBar({ lang, onLang, notation, onNotation }: Props) {
  const { t } = useTranslation();
  return (
    <div className="settings">
      <label>
        {t("settings.language")}
        <select value={lang} onChange={(e) => onLang(e.target.value as Lang)} data-testid="lang-select">
          {LANGS.map((l) => (
            <option key={l} value={l}>
              {LANG_NAMES[l]}
            </option>
          ))}
        </select>
      </label>
      <fieldset className="segmented" data-testid="notation">
        <legend>{t("settings.notation")}</legend>
        {NOTATION_KINDS.map((k) => (
          <button
            key={k}
            type="button"
            aria-pressed={notation === k}
            onClick={() => onNotation(k)}
            data-testid={`notation-${k}`}
          >
            {t(`settings.notationKind.${k}`)}
          </button>
        ))}
      </fieldset>
      {MACHINE_TRANSLATED.has(lang) && (
        <p className="machine-translation" data-testid="machine-translation">
          {t("settings.machineTranslation")}{" "}
          <a href={FEEDBACK_URL} target="_blank" rel="noopener noreferrer">
            {t("settings.machineTranslationLink")}
          </a>
        </p>
      )}
    </div>
  );
}
