// 条件の欄(未適用)。数値の欄は文字列のまま持つ(本家 M47-04 追補②: 数値で持つと空にした瞬間に最小値へ戻る)。
// 「提案を出す」で validateForm を通った条件だけを計算に渡す。
import {
  FILLER_REMAIN_MIN,
  GAP_G_MAX,
  GAP_G_MIN,
  KA_MIN,
  N_MIN_LOWER,
} from "./constants";
import { KA_MAX } from "./flavor";
import {
  defaultConditions,
  parseIntInRange,
  type Conditions,
  type SafeJumpKind,
  type Sort,
  type TargetType,
} from "./conditions";
import type { Mode } from "./engineTypes";

export interface FormState {
  character: string;
  kaText: string;
  mode: Mode;
  nMinText: string;
  nMaxText: string;
  gMinText: string;
  gMaxText: string;
  remainText: string;
  targetTypes: TargetType[];
  includeZeroDamage: boolean;
  targetMove: string | null;
  sort: Sort;
  approachOnly: boolean;
  blockSafeOnly: boolean;
  chainCancelOnly: boolean;
}

/** 欄の誤り(i18n のキーの末尾に当たる名前)。 */
export type FormError =
  | "characterRequired"
  | "kaInvalid"
  | "nMinInvalid"
  | "nMaxInvalid"
  | "nMaxBelowMin"
  | "gapRangeInvalid"
  | "gapMaxBelowMin"
  | "fillerRemainRequired"
  | "targetTypesRequired";

export function formFromConditions(c: Conditions): FormState {
  return {
    character: c.character,
    kaText: String(c.ka),
    mode: c.mode,
    nMinText: String(c.nMin),
    nMaxText: c.nMax > 0 ? String(c.nMax) : "",
    gMinText: String(c.gMin),
    gMaxText: String(c.gMax),
    remainText: String(c.remain),
    targetTypes: [...c.targetTypes],
    includeZeroDamage: c.includeZeroDamage,
    targetMove: c.targetMove,
    sort: c.sort,
    approachOnly: c.approachOnly,
    blockSafeOnly: c.blockSafeOnly,
    chainCancelOnly: c.chainCancelOnly,
  };
}

/** 最初の欄(キャラ未選択・KA は空)。 */
export function initialForm(): FormState {
  return { ...formFromConditions(defaultConditions("", KA_MIN)), kaText: "" };
}

export interface Validated {
  /** 誤りが無ければ計算に渡す条件。 */
  conditions: Conditions | null;
  errors: FormError[];
}

/**
 * 欄を検査して条件にする。いまのモードが使う欄だけを検査する(使わない欄が誤っていても押せる。本家と同じ)。
 * 使わない欄の値は既定値で埋める(エンジンには送らない＝toEngineQuery)。
 * safeJump はここでは null(安全飛びのボタンは validateForm の後で remain と種類を差し替える)。
 */
/** kaMax は KA の上限(サイト 100・ローカル版は外す。既定はこのビルドの上限)。 */
export function validateForm(f: FormState, kaMax: number = KA_MAX): Validated {
  const errors: FormError[] = [];
  const d = defaultConditions(f.character, KA_MIN);
  if (f.character === "") errors.push("characterRequired");
  const ka = parseIntInRange(f.kaText, KA_MIN, kaMax);
  if (ka == null) errors.push("kaInvalid");

  const nMin = parseIntInRange(f.nMinText, N_MIN_LOWER);
  // 空欄は 0(= 上限なし)。
  const nMax = f.nMaxText.trim() === "" ? 0 : parseIntInRange(f.nMaxText, 0);
  const gMin = parseIntInRange(f.gMinText, GAP_G_MIN, GAP_G_MAX);
  const gMax = parseIntInRange(f.gMaxText, GAP_G_MIN, GAP_G_MAX);
  const remain = parseIntInRange(f.remainText, FILLER_REMAIN_MIN);

  if (f.mode === "meaty") {
    if (nMin == null) errors.push("nMinInvalid");
    if (nMax == null) errors.push("nMaxInvalid");
    // 最小 > 最大は受理帯が空になり必ず 0 件のため押せなくする(本家 M47-04 追補④)。
    if (nMin != null && nMax != null && nMax > 0 && nMax < nMin) errors.push("nMaxBelowMin");
  } else if (f.mode === "gap") {
    if (gMin == null || gMax == null) errors.push("gapRangeInvalid");
    if (gMin != null && gMax != null && gMax < gMin) errors.push("gapMaxBelowMin");
  } else if (remain == null) {
    errors.push("fillerRemainRequired");
  }
  // ★単体版の変更: 種別が 0 個だと本家のクエリは target_types が空になり、エンジンが既定の種別に戻す
  //   (押した条件と違う結果が出る)。特定の技を選んでいないときは押せなくする(S5 報告)。
  if (f.mode !== "filler" && f.targetMove == null && f.targetTypes.length === 0) {
    errors.push("targetTypesRequired");
  }

  if (errors.length > 0 || ka == null) return { conditions: null, errors };
  return {
    conditions: {
      character: f.character,
      ka,
      mode: f.mode,
      nMin: nMin ?? d.nMin,
      nMax: nMax ?? d.nMax,
      gMin: gMin ?? d.gMin,
      gMax: gMax ?? d.gMax,
      remain: remain ?? d.remain,
      safeJump: null,
      targetTypes: [...f.targetTypes],
      includeZeroDamage: f.includeZeroDamage,
      targetMove: f.mode === "filler" ? null : f.targetMove,
      sort: f.sort,
      approachOnly: f.approachOnly,
      blockSafeOnly: f.blockSafeOnly,
      chainCancelOnly: f.chainCancelOnly,
    },
    errors,
  };
}

/** 安全飛びのボタン: 有利調整を remain = そのキャラの値で出す(本家と同じ計算)。KA・キャラの誤りは validateForm と同じ。 */
export function safeJumpConditions(f: FormState, kind: SafeJumpKind, value: number, kaMax: number = KA_MAX): Validated {
  const v = validateForm({ ...f, mode: "filler", remainText: String(value) }, kaMax);
  if (v.conditions == null) return v;
  return { conditions: { ...v.conditions, safeJump: kind, targetMove: null }, errors: [] };
}
