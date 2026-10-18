// 条件の欄(本家 SetplaySuggestionSection の条件パネルの作り直し)。「提案を出す」まで計算しない。
import { useEffect, useState, type FocusEvent } from "react";
import { useTranslation } from "react-i18next";

import {
  SAFE_JUMP_KINDS,
  TARGET_TYPE_ORDER,
  type Conditions,
  type TargetType,
} from "../lib/conditions";
import {
  FILLER_REMAIN_MIN,
  GAP_G_MAX,
  GAP_G_MIN,
  KA_MIN,
  kaMaxFor,
  N_MIN_LOWER,
  RELEASES_URL,
  SLOW_KA_THRESHOLD,
} from "../lib/constants";
import type { Flavor } from "../lib/flavor";
import type { ManifestCharacter, Move, NotationKind } from "../lib/data";
import type { Mode } from "../lib/engineTypes";
import { parseIntInRange } from "../lib/conditions";
import { safeJumpConditions, validateForm, type FormError, type FormState } from "../lib/formState";
import type { Lang } from "../lib/i18n";
import { CharacterSelect } from "./CharacterSelect";
import { TargetPicker } from "./TargetPicker";

interface Props {
  form: FormState;
  onChange: (f: FormState) => void;
  characters: readonly ManifestCharacter[];
  lang: Lang;
  moves: readonly Move[];
  notation: NotationKind;
  running: boolean;
  onRun: (c: Conditions) => void;
  /** 条件・結果・URL を最初の状態に戻す(言語と表記は戻さない＝利用者の設定)。 */
  onReset: () => void;
  /** ビルドの種類(KA の上限と説明が変わる。HANDOVER D10)。 */
  flavor: Flavor;
}

const MODES: readonly Mode[] = ["meaty", "gap", "filler"];
const MODE_LABEL = { meaty: "setplay.modeMeaty", gap: "setplay.modeGap", filler: "setplay.modeFiller" } as const;

/** 欄の誤り → 文言のキー。 */
const ERROR_KEY: Record<FormError, string> = {
  characterRequired: "form.characterRequired",
  kaInvalid: "form.kaInvalid", // ローカル版は form.kaInvalidLocal(上限なし)
  nMinInvalid: "setplay.nMinInvalid",
  nMaxInvalid: "setplay.nMaxInvalid",
  nMaxBelowMin: "setplay.nMaxBelowMinHint",
  gapRangeInvalid: "setplay.gapRangeInvalid",
  gapMaxBelowMin: "setplay.gapMaxBelowMinHint",
  fillerRemainRequired: "setplay.fillerRemainRequired",
  targetTypesRequired: "setplay.targetTypesRequired",
};

/** 数値の欄をクリックしたら中の数を全選択する(そのまま打てば置き換わる。本家と同じ)。 */
function selectAllOnFocus(e: FocusEvent<HTMLInputElement>) {
  e.currentTarget.select();
}

export function ConditionsPanel({ form, onChange, characters, lang, moves, notation, running, onRun, onReset, flavor }: Props) {
  const { t } = useTranslation();
  // 「重ねる技」の選び方(種類で選ぶ／1 つの技を指定)。表示の状態で、条件に効くのは targetMove だけ。
  const [byMove, setByMove] = useState(form.targetMove != null);
  const set = <K extends keyof FormState>(k: K, v: FormState[K]) => onChange({ ...form, [k]: v });

  const kaMax = kaMaxFor(flavor);
  const local = flavor === "local";
  const validated = validateForm(form, kaMax);
  const character = characters.find((c) => c.code === form.character) ?? null;
  const ka = parseIntInRange(form.kaText, KA_MIN, kaMax);
  const remain = parseIntInRange(form.remainText, FILLER_REMAIN_MIN);
  const isFiller = form.mode === "filler";
  // URL から読んだ条件に技があれば、技の指定として出す(欄は表示の後に URL から埋まるため、初期値だけでは足りない)。
  // 一度技が入ったら選び方を「1 つの技を指定」に固定する(技の欄を空にしても種類の欄へ勝手に変わらない)。
  useEffect(() => {
    if (form.targetMove != null) setByMove(true);
  }, [form.targetMove]);
  const showMove = !isFiller && (byMove || form.targetMove != null);
  // 「1 つの技を指定」で技が未選択なら押せない(隠れた種類の条件で計算しない。S7 レビュー中-1)。
  const moveRequired = showMove && form.targetMove == null;
  // 誤りの文は、書き始めた欄(キャラ・KA 以外)と、書いた KA だけに出す(開いた直後から赤くしない)。
  // 種類の誤りは、種類で選んでいるときだけ(隠れた欄の誤りは直せないため)。
  const shownErrors = validated.errors.filter(
    (e) =>
      e !== "characterRequired" &&
      !(e === "kaInvalid" && form.kaText.trim() === "") &&
      !(e === "targetTypesRequired" && showMove),
  );

  const pickCharacter = (code: string) => onChange({ ...form, character: code, targetMove: null });
  const chooseByMove = (v: boolean) => {
    setByMove(v);
    // 種類で選ぶに戻したら、選んでいた技を外す(条件に残すと種類の欄が効かないため)。
    if (!v && form.targetMove != null) set("targetMove", null);
  };

  const toggleType = (type: TargetType) => {
    const has = form.targetTypes.includes(type);
    const next = TARGET_TYPE_ORDER.filter((x) => (x === type ? !has : form.targetTypes.includes(x)));
    set("targetTypes", next);
  };


  return (
    <section className="panel" aria-label={t("setplay.generate")}>
      <div className="row">
        <div className="field">
          <span className="label" id="character-label">
            {t("form.character")}
          </span>
          <CharacterSelect
            characters={characters}
            value={form.character}
            onChange={pickCharacter}
            lang={lang}
            labelId="character-label"
          />
        </div>
        <label className="field">
          <span>{t("form.ka")}</span>
          <span className="inline">
            <input
              type="number"
              inputMode="numeric"
              min={KA_MIN}
              max={kaMax}
              value={form.kaText}
              onChange={(e) => set("kaText", e.target.value)}
              onFocus={selectAllOnFocus}
              className="num"
              data-testid="ka-input"
            />
            {t("form.kaUnit")}
          </span>
        </label>
      </div>
      {local ? (
        <p className="help" data-testid="ka-help">
          {t("form.kaHelpLocal", { min: KA_MIN })}
        </p>
      ) : (
        <p className="help" data-testid="ka-help">
          {t("form.kaHelp", { min: KA_MIN, max: kaMax })}{" "}
          <a href={RELEASES_URL} target="_blank" rel="noopener noreferrer" data-testid="local-version-link">
            {t("form.kaOverLimit", { max: kaMax })}
          </a>
        </p>
      )}

      <div className="row" role="group" aria-label={t("setplay.modeLabel")} data-testid="mode">
        <span className="label">{t("setplay.modeLabel")}</span>
        <span className="segmented">
          {MODES.map((m) => (
            <button
              key={m}
              type="button"
              aria-pressed={form.mode === m}
              onClick={() => set("mode", m)}
              data-testid={`mode-${m}`}
            >
              {t(MODE_LABEL[m])}
            </button>
          ))}
        </span>
      </div>

      {!isFiller && (
        <fieldset data-testid="target-fieldset">
          <legend>{t("setplay.targetTypeLegend")}</legend>
          <div className="row" role="group" aria-label={t("setplay.targetChooseLabel")}>
            <span className="label">{t("setplay.targetChooseLabel")}</span>
            <span className="segmented">
              <button type="button" aria-pressed={!showMove} onClick={() => chooseByMove(false)} data-testid="target-by-type">
                {t("setplay.targetByType")}
              </button>
              <button type="button" aria-pressed={showMove} onClick={() => chooseByMove(true)} data-testid="target-by-move">
                {t("setplay.targetByMove")}
              </button>
            </span>
          </div>
          {showMove ? (
            <TargetPicker
              moves={moves}
              notation={notation}
              value={form.targetMove}
              onChange={(code) => set("targetMove", code)}
            />
          ) : (
            <div className="target-types" data-testid="type-fieldset">
              <div className="checks">
                {TARGET_TYPE_ORDER.map((type) => (
                  <label key={type}>
                    <input
                      type="checkbox"
                      checked={form.targetTypes.includes(type)}
                      onChange={() => toggleType(type)}
                      data-testid={`type-${type}`}
                    />
                    {t(`setplay.targetType.${type}`)}
                  </label>
                ))}
              </div>
              <label>
                <input
                  type="checkbox"
                  checked={form.includeZeroDamage}
                  onChange={(e) => set("includeZeroDamage", e.target.checked)}
                  data-testid="include-zero-damage"
                />
                {t("setplay.includeZeroDamage")}
              </label>
            </div>
          )}
        </fieldset>
      )}

      {isFiller ? (
        <div className="group" data-testid="filler-panel">
          <label className="inline">
            <span>{t("setplay.fillerRemainLabel")}</span>
            <input
              type="number"
              min={FILLER_REMAIN_MIN}
              value={form.remainText}
              onChange={(e) => set("remainText", e.target.value)}
              onFocus={selectAllOnFocus}
              className="num"
              aria-label={t("setplay.fillerRemainAria")}
              data-testid="filler-remain"
            />
            {t("setplay.fillerRemainUnit")}
          </label>
          <p className="help">{t("setplay.fillerRemainHelp")}</p>
          {/* 調整後の有利 ≥ KA は繋ぎを置けない。押す前に理由を出す(エンジンも 0 件＋理由で返す)。 */}
          {remain != null && ka != null && remain >= ka && (
            <p className="warn" data-testid="filler-remain-hint">
              {t("setplay.fillerRemainNotBelowKnockdown")}
            </p>
          )}
          <div className="inline" data-testid="safe-jump">
            <span>{t("setplay.safeJumpLegend")}</span>
            {SAFE_JUMP_KINDS.map((kind) => {
              const v = character?.safeJump?.[kind];
              const label = t(`setplay.safeJumpKind.${kind}`);
              const sj = v == null ? null : safeJumpConditions(form, kind, v, kaMax);
              return (
                <button
                  key={kind}
                  type="button"
                  className="secondary"
                  disabled={sj?.conditions == null || running}
                  onClick={() => sj?.conditions && onRun(sj.conditions)}
                  data-testid={`safe-jump-${kind}`}
                >
                  {v == null ? label : t("setplay.safeJumpButton", { kind: label, value: v })}
                </button>
              );
            })}
          </div>
          {character != null && character.safeJump == null && (
            <p className="help" data-testid="safe-jump-unavailable">
              {t("setplay.safeJumpUnavailable")}
            </p>
          )}
          {form.approachOnly && (
            <p className="help" data-testid="safe-jump-approach-note">
              {t("setplay.safeJumpApproachNote")}
            </p>
          )}
        </div>
      ) : form.mode === "gap" ? (
        <div className="group" data-testid="gap-range">
          <span className="inline">
            <span>{t("setplay.gapRangeLabel")}</span>
            <span>{t("setplay.gapRangePrefix")}</span>
            <input
              type="number"
              min={GAP_G_MIN}
              max={GAP_G_MAX}
              value={form.gMinText}
              onChange={(e) => set("gMinText", e.target.value)}
              onFocus={selectAllOnFocus}
              className="num"
              aria-label={t("setplay.gapMinLabel")}
              data-testid="g-min"
            />
            <span>{t("setplay.rangeSeparator")}</span>
            <input
              type="number"
              min={GAP_G_MIN}
              max={GAP_G_MAX}
              value={form.gMaxText}
              onChange={(e) => set("gMaxText", e.target.value)}
              onFocus={selectAllOnFocus}
              className="num"
              aria-label={t("setplay.gapMaxLabel")}
              data-testid="g-max"
            />
            <span>{t("setplay.gapRangeUnit")}</span>
          </span>
          <p className="help">{t("setplay.gapRangeHelp")}</p>
        </div>
      ) : (
        <div className="group" data-testid="n-range">
          <span className="inline">
            <span>{t("setplay.nRangeLabel")}</span>
            <input
              type="number"
              min={N_MIN_LOWER}
              value={form.nMinText}
              onChange={(e) => set("nMinText", e.target.value)}
              onFocus={selectAllOnFocus}
              className="num"
              aria-label={t("setplay.nMinLabel")}
              data-testid="n-min"
            />
            <span>{t("setplay.rangeSeparator")}</span>
            <input
              type="number"
              min={0}
              value={form.nMaxText}
              onChange={(e) => set("nMaxText", e.target.value)}
              onFocus={selectAllOnFocus}
              className="num wide"
              aria-label={t("setplay.nMaxLabel")}
              placeholder={t("setplay.nMaxPlaceholder")}
              data-testid="n-max"
            />
            <span>{t("setplay.nRangeUnit")}</span>
          </span>
        </div>
      )}

      {!isFiller && (
        <div className="row" role="group" aria-label={t("setplay.sortLabel")}>
          <span className="label">{t("setplay.sortLabel")}</span>
          <span className="segmented">
            <button type="button" aria-pressed={form.sort === "n"} onClick={() => set("sort", "n")} data-testid="sort-n">
              {t("setplay.sortByN")}
            </button>
            <button
              type="button"
              aria-pressed={form.sort === "target"}
              onClick={() => set("sort", "target")}
              data-testid="sort-target"
            >
              {t("setplay.sortByTarget")}
            </button>
          </span>
        </div>
      )}

      {/* 結果の行の絞り込み(既定 OFF・AND)。ガード時 0F 以上は重ねるモードでだけ効くため、そのときだけ出す。 */}
      <fieldset data-testid="filter-fieldset">
        <legend>{t("setplay.filterLegend")}</legend>
        <div className="checks">
          <label>
            <input
              type="checkbox"
              checked={form.approachOnly}
              onChange={(e) => set("approachOnly", e.target.checked)}
              data-testid="filter-approach"
            />
            {t("setplay.filterApproach")}
          </label>
          {form.mode === "meaty" && (
            <label>
              <input
                type="checkbox"
                checked={form.blockSafeOnly}
                onChange={(e) => set("blockSafeOnly", e.target.checked)}
                data-testid="filter-block-safe"
              />
              {t("setplay.filterBlockSafe")}
            </label>
          )}
          <label>
            <input
              type="checkbox"
              checked={form.chainCancelOnly}
              onChange={(e) => set("chainCancelOnly", e.target.checked)}
              data-testid="filter-chain-cancel"
            />
            {t("setplay.filterChainCancel")}
          </label>
        </div>
      </fieldset>

      {moveRequired && form.character !== "" && (
        <p className="warn" data-testid="target-move-required">
          {t("setplay.targetMoveRequired")}
        </p>
      )}
      {shownErrors.length > 0 && (
        <ul className="warn" data-testid="form-errors">
          {shownErrors.map((e) => (
            <li key={e} data-testid={`form-error-${e}`}>
              {t(e === "kaInvalid" && local ? "form.kaInvalidLocal" : ERROR_KEY[e], { min: KA_MIN, max: kaMax })}
            </li>
          ))}
        </ul>
      )}

      <div className="actions">
        <button
          type="button"
          className="primary"
          disabled={validated.conditions == null || running || moveRequired}
          onClick={() => validated.conditions && onRun(validated.conditions)}
          data-testid="generate"
        >
          {t("setplay.generate")}
        </button>
        <button
          type="button"
          className="secondary reset"
          onClick={() => {
            setByMove(false);
            onReset();
          }}
          data-testid="reset"
        >
          {t("setplay.reset")}
        </button>
      </div>
      {ka != null && ka >= SLOW_KA_THRESHOLD && (
        <p className="warn" data-testid="slow-warning">
          {t("setplay.slowWarning")}
        </p>
      )}
    </section>
  );
}
