// キャラの選択(S7。本家の SearchableSelect〔単一選択・キャラ用〕と同じ動き。本家は Radix の Popover を使うが、
// 依存を増やさないよう自前で作った)。箱を押すと検索欄と候補の一覧が開き、打つと語の先頭一致で絞られる
// (`ry` → リュウ。terry は出ない＝characterSearch.ts)。↑↓ で候補を移り、Enter かクリックで選ぶ。
// 検索欄で Enter を押すと先頭の候補を選ぶ。Esc か外側のクリックで閉じる(閉じたら検索語は消す。本家と同じ)。
import { useEffect, useRef, useState, type KeyboardEvent } from "react";
import { useTranslation } from "react-i18next";

import { filterCharacters } from "../lib/characterSearch";
import type { ManifestCharacter } from "../lib/data";
import type { Lang } from "../lib/i18n";

interface Props {
  characters: readonly ManifestCharacter[];
  /** 選んでいるキャラの code。未選択は "" 。 */
  value: string;
  onChange: (code: string) => void;
  lang: Lang;
  labelId: string;
}

export function CharacterSelect({ characters, value, onChange, lang, labelId }: Props) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const rootRef = useRef<HTMLDivElement>(null);
  const listRef = useRef<HTMLUListElement>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);

  const nameOf = (c: ManifestCharacter) => (lang === "ja" ? c.name.ja : c.name.en);
  const current = characters.find((c) => c.code === value) ?? null;
  const matched = filterCharacters(characters, query);

  const close = (focusTrigger: boolean) => {
    setOpen(false);
    setQuery("");
    if (focusTrigger) triggerRef.current?.focus();
  };
  const pick = (code: string) => {
    onChange(code);
    close(true);
  };

  // 外側のクリックで閉じる。
  useEffect(() => {
    if (!open) return;
    const onDown = (e: MouseEvent) => {
      if (rootRef.current != null && !rootRef.current.contains(e.target as Node)) close(false);
    };
    document.addEventListener("mousedown", onDown);
    return () => document.removeEventListener("mousedown", onDown);
  }, [open]);

  const options = () => Array.from(listRef.current?.querySelectorAll<HTMLElement>('[role="option"]') ?? []);
  /** ↑↓ で候補のフォーカスを移す(端で折り返す。本家の useListboxKeyNav と同じ)。 */
  const moveFocus = (delta: number) => {
    const items = options();
    if (items.length === 0) return;
    const at = items.indexOf(document.activeElement as HTMLElement);
    const next = at === -1 ? (delta > 0 ? 0 : items.length - 1) : (at + delta + items.length) % items.length;
    items[next]?.focus();
  };
  const onKey = (e: KeyboardEvent<HTMLElement>) => {
    if (e.key === "ArrowDown" || e.key === "ArrowUp") {
      e.preventDefault();
      moveFocus(e.key === "ArrowDown" ? 1 : -1);
    } else if (e.key === "Escape") {
      e.preventDefault();
      close(true);
    }
  };
  const onSearchKey = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Enter") {
      e.preventDefault();
      if (matched.length > 0) pick(matched[0]!.code);
      return;
    }
    onKey(e);
  };

  return (
    <div className="combo" ref={rootRef}>
      <button
        type="button"
        ref={triggerRef}
        role="combobox"
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-labelledby={labelId}
        className="combo-trigger"
        onClick={() => (open ? close(false) : setOpen(true))}
        data-testid="character-select"
        data-value={value}
      >
        <span className={current == null ? "muted" : undefined}>
          {current == null ? t("form.characterPlaceholder") : nameOf(current)}
        </span>
        <span aria-hidden="true">▾</span>
      </button>
      {open && (
        <div className="combo-pop">
          <input
            type="text"
            autoFocus
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyDown={onSearchKey}
            placeholder={t("form.characterSearch")}
            aria-label={t("form.characterSearchAria")}
            data-testid="character-search"
          />
          {matched.length === 0 ? (
            <p className="help" data-testid="character-no-match">
              {t("form.characterNoMatch")}
            </p>
          ) : (
            <ul className="combo-list" role="listbox" aria-labelledby={labelId} ref={listRef} onKeyDown={onKey}>
              {matched.map((c) => (
                <li key={c.code}>
                  <button
                    type="button"
                    role="option"
                    aria-selected={c.code === value}
                    onClick={() => pick(c.code)}
                    data-testid={`character-option-${c.code}`}
                  >
                    {nameOf(c)}
                  </button>
                </li>
              ))}
            </ul>
          )}
        </div>
      )}
    </div>
  );
}
