// 「重ねる技」を「1 つの技を指定」で選ぶときの技の選択(本家 SetplayTargetPicker の作り直し)。区分で絞って選ぶ。system(非技)は出さない。
import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";

import { MOVE_CATEGORY_ORDER, PICKER_EXCLUDED_CATEGORY, type Move, type NotationKind } from "../lib/data";

interface Props {
  moves: readonly Move[];
  notation: NotationKind;
  value: string | null;
  onChange: (code: string | null) => void;
}

const ALL = "all";

export function TargetPicker({ moves, notation, value, onChange }: Props) {
  const { t } = useTranslation();
  const [category, setCategory] = useState<string>(ALL);

  const grouped = useMemo(() => {
    const map = new Map<string, Move[]>();
    for (const m of moves) {
      if (m.category === PICKER_EXCLUDED_CATEGORY) continue;
      const arr = map.get(m.category) ?? [];
      arr.push(m);
      map.set(m.category, arr);
    }
    return map;
  }, [moves]);
  // 表の順に無い区分(データに新しい区分が足されたとき)も末尾に出す(黙って消さない)。
  const categories = [
    ...MOVE_CATEGORY_ORDER.filter((c) => grouped.has(c)),
    ...[...grouped.keys()].filter((c) => !(MOVE_CATEGORY_ORDER as readonly string[]).includes(c)),
  ];
  const categoryLabel = (c: string) => t(`category.${c}`, { defaultValue: c });

  return (
    <div className="picker" data-testid="picker">
      <label>
        {t("setplay.picker.category")}
        <select value={category} onChange={(e) => setCategory(e.target.value)} data-testid="picker-category">
          <option value={ALL}>{t("setplay.picker.allCategories")}</option>
          {categories.map((c) => (
            <option key={c} value={c}>
              {categoryLabel(c)}
            </option>
          ))}
        </select>
      </label>
      <label>
        {t("setplay.picker.move")}
        <select
          value={value ?? ""}
          onChange={(e) => onChange(e.target.value === "" ? null : e.target.value)}
          data-testid="picker-move"
        >
          <option value="">{t("setplay.picker.placeholder")}</option>
          {categories
            .filter((c) => category === ALL || c === category)
            .map((c) => (
              <optgroup key={c} label={categoryLabel(c)}>
                {(grouped.get(c) ?? []).map((m) => (
                  <option key={m.code} value={m.code}>
                    {m.notation[notation]}
                  </option>
                ))}
              </optgroup>
            ))}
        </select>
      </label>
    </div>
  );
}
