// 検索の条件。画面の状態・URL クエリ(共有)・エンジンのクエリ文字列の 3 つの形の間を写す純粋関数。
// ★URL は外から来る入力なので zod で検証し、誤った値は既定値に戻す(読み込みで落とさない)。
import { z } from "zod";

import {
  ENGINE_LIMIT,
  FILLER_REMAIN_DEFAULT,
  FILLER_REMAIN_MIN,
  GAP_G_MAX,
  GAP_G_MIN,
  KA_MAX_SITE,
  KA_MIN,
  N_MIN_LOWER,
} from "./constants";
import type { Mode } from "./engineTypes";

export type Sort = "n" | "target";

/** 当てる技の種別(engine の target_types)。画面に並べる順。 */
export const TARGET_TYPE_ORDER = [
  "normal",
  "unique",
  "special_projectile",
  "special",
  "throw",
  "normal_rush",
  "unique_rush",
] as const;
export type TargetType = (typeof TARGET_TYPE_ORDER)[number];

/** 既定で ON の種別(engine.DefaultTargetTypes と同じ)。 */
export const DEFAULT_TARGET_TYPES: readonly TargetType[] = ["normal", "unique", "special_projectile"];

export const SAFE_JUMP_KINDS = ["f5", "f6", "true"] as const;
export type SafeJumpKind = (typeof SAFE_JUMP_KINDS)[number];

/**
 * 計算に渡す条件(「提案を出す」で確定したもの)。
 * nMax の 0 は「上限なし」(本家と同じ)。safeJump は安全飛びのボタンで出した検索のときだけ種類が入る
 * (計算は remain = そのキャラの値の有利調整と同じ。行の表示だけが変わる)。
 */
export interface Conditions {
  character: string;
  ka: number;
  mode: Mode;
  nMin: number;
  nMax: number;
  gMin: number;
  gMax: number;
  remain: number;
  safeJump: SafeJumpKind | null;
  targetTypes: TargetType[];
  includeZeroDamage: boolean;
  targetMove: string | null;
  sort: Sort;
  approachOnly: boolean;
  blockSafeOnly: boolean;
  chainCancelOnly: boolean;
}

/** キャラと KA 以外の既定値。 */
export function defaultConditions(character: string, ka: number): Conditions {
  return {
    character,
    ka,
    mode: "meaty",
    nMin: N_MIN_LOWER,
    nMax: 0,
    gMin: GAP_G_MIN,
    gMax: GAP_G_MAX,
    remain: FILLER_REMAIN_DEFAULT,
    safeJump: null,
    targetTypes: [...DEFAULT_TARGET_TYPES],
    includeZeroDamage: false,
    targetMove: null,
    sort: "n",
    approachOnly: false,
    blockSafeOnly: false,
    chainCancelOnly: false,
  };
}

/**
 * エンジン(WASM の suggest)に渡すクエリ文字列。本家の画面(setplayApi.getSuggestions)と同じ規則で組む:
 * n_max は meaty で 0 以外のときだけ、g_* は gap だけ、remain は filler だけ、block_safe_only は meaty だけ、
 * 絞り込みは true のときだけ送る。limit は常に上限(ENGINE_LIMIT。画面で PAGE_SIZE 件ずつ出す)。
 * ★安全飛びの検索では「接近を含む提案だけ」を掛けない(本家 M47-05 追補②。前ジャンプ自体が接近)。
 */
export function toEngineQuery(c: Conditions): string {
  const p = new URLSearchParams();
  p.set("n_min", String(c.nMin));
  if (c.mode === "meaty" && c.nMax > 0) p.set("n_max", String(c.nMax));
  p.set("sort", c.sort);
  p.set("target_types", c.targetTypes.join(","));
  p.set("include_zero_damage", String(c.includeZeroDamage));
  if (c.targetMove != null) p.set("target_move", c.targetMove);
  p.set("mode", c.mode);
  if (c.mode === "gap") {
    p.set("g_min", String(c.gMin));
    p.set("g_max", String(c.gMax));
  }
  if (c.mode === "filler") p.set("remain", String(c.remain));
  p.set("limit", String(ENGINE_LIMIT));
  if (c.approachOnly && c.safeJump == null) p.set("approach_only", "true");
  if (c.blockSafeOnly && c.mode === "meaty") p.set("block_safe_only", "true");
  if (c.chainCancelOnly) p.set("chain_cancel_only", "true");
  return p.toString();
}

/**
 * エンジンに実際に送る絞り込みがあるか(toEngineQuery と同じ規則)。0 件のときに「絞り込みを外すと…」を出すかに使う
 * (本家も送ったものだけで判定する。汚連携・有利調整のガード時 0F 以上、安全飛びの接近は送らない)。
 */
export function hasAppliedFilters(c: Conditions): boolean {
  return (c.approachOnly && c.safeJump == null) || (c.blockSafeOnly && c.mode === "meaty") || c.chainCancelOnly;
}

/** 結果のキャッシュの鍵。同じ計算になる条件は同じ鍵(エンジンのクエリで決まる)。 */
export function cacheKey(c: Conditions): string {
  return `${c.character}|${c.ka}|${toEngineQuery(c)}`;
}

// ---- URL クエリ(共有) ----
// キーは短く、既定値と同じものは書かない(共有する URL を短く保つ)。言語・表記は入れない(受け手の設定を上書きしない)。
const URL_KEYS = {
  character: "c",
  ka: "ka",
  mode: "mode",
  nMin: "n_min",
  nMax: "n_max",
  gMin: "g_min",
  gMax: "g_max",
  remain: "remain",
  safeJump: "sj",
  targetTypes: "types",
  includeZeroDamage: "zero",
  targetMove: "move",
  sort: "sort",
  approachOnly: "approach",
  blockSafeOnly: "block_safe",
  chainCancelOnly: "chain",
} as const satisfies Record<keyof Conditions, string>;

// ★URL の数は画面の欄(parseIntInRange)と同じ規則で受ける: 0 以上の整数で、安全な整数(Number.MAX_SAFE_INTEGER)まで。
//   画面で押せる条件を共有したら、開いた側でも同じ条件になる(往復の試験 conditions.test.ts)。
const intString = z.string().regex(/^\d+$/).transform(Number).pipe(z.number().max(Number.MAX_SAFE_INTEGER));
const flag = z.literal("1").transform(() => true);
const urlSchema = z.object({
  c: z.string().regex(/^[a-z0-9_]+$/),
  // 上限は parseSearch の kaMax で見る(サイトとローカル版で違う)。
  ka: intString.pipe(z.number().int().min(KA_MIN)),
  mode: z.enum(["meaty", "gap", "filler"]).optional(),
  n_min: intString.pipe(z.number().int().min(N_MIN_LOWER)).optional(),
  n_max: intString.pipe(z.number().int().min(0)).optional(),
  g_min: intString.pipe(z.number().int().min(GAP_G_MIN).max(GAP_G_MAX)).optional(),
  g_max: intString.pipe(z.number().int().min(GAP_G_MIN).max(GAP_G_MAX)).optional(),
  remain: intString.pipe(z.number().int().min(FILLER_REMAIN_MIN)).optional(),
  sj: z.enum(SAFE_JUMP_KINDS).optional(),
  // 空は「種別 0 個」(特定の技を選んだとき・有利調整では画面で押せる)。それ以外での空は parseSearch が捨てる。
  types: z
    .string()
    .transform((s) => (s === "" ? [] : s.split(",")))
    .pipe(z.array(z.enum(TARGET_TYPE_ORDER)))
    .optional(),
  zero: flag.optional(),
  move: z.string().regex(/^[a-z0-9_]+$/).optional(),
  sort: z.enum(["n", "target"]).optional(),
  approach: flag.optional(),
  block_safe: flag.optional(),
  chain: flag.optional(),
});

/** 条件を URL クエリにする(既定値と同じものは書かない)。 */
export function conditionsToSearch(c: Conditions): string {
  const d = defaultConditions(c.character, c.ka);
  const p = new URLSearchParams();
  p.set(URL_KEYS.character, c.character);
  p.set(URL_KEYS.ka, String(c.ka));
  if (c.mode !== d.mode) p.set(URL_KEYS.mode, c.mode);
  if (c.nMin !== d.nMin) p.set(URL_KEYS.nMin, String(c.nMin));
  if (c.nMax !== d.nMax) p.set(URL_KEYS.nMax, String(c.nMax));
  if (c.gMin !== d.gMin) p.set(URL_KEYS.gMin, String(c.gMin));
  if (c.gMax !== d.gMax) p.set(URL_KEYS.gMax, String(c.gMax));
  if (c.remain !== d.remain) p.set(URL_KEYS.remain, String(c.remain));
  if (c.safeJump != null) p.set(URL_KEYS.safeJump, c.safeJump);
  if (c.targetTypes.join(",") !== d.targetTypes.join(",")) p.set(URL_KEYS.targetTypes, c.targetTypes.join(","));
  if (c.includeZeroDamage) p.set(URL_KEYS.includeZeroDamage, "1");
  if (c.targetMove != null) p.set(URL_KEYS.targetMove, c.targetMove);
  if (c.sort !== d.sort) p.set(URL_KEYS.sort, c.sort);
  if (c.approachOnly) p.set(URL_KEYS.approachOnly, "1");
  // ガード時 0F 以上は重ねるだけで効く(ほかのモードでは送らない)ので、URL にも重ねるのときだけ書く。
  if (c.blockSafeOnly && c.mode === "meaty") p.set(URL_KEYS.blockSafeOnly, "1");
  if (c.chainCancelOnly) p.set(URL_KEYS.chainCancelOnly, "1");
  return p.toString();
}

/** URL クエリの読み取り結果。conditions が null なら共有の条件ではない(キャラ・KA が無いか誤り)。 */
export interface ParsedSearch {
  conditions: Conditions | null;
  /** 誤りで捨てたキー(既定値に戻した)。 */
  rejected: string[];
}

/**
 * URL クエリを条件にする。キャラ・KA が無いか誤りなら conditions は null。それ以外のキーの誤りは、
 * そのキーだけ既定値に戻して rejected に挙げる(共有された URL が少し壊れていても開けるように)。
 * knownCharacters に無いキャラは誤り。kaMax を超える KA も誤り(既定はサイトの上限。ローカル版は呼び出し側が渡す)。
 */
export function parseSearch(search: string, knownCharacters: ReadonlySet<string>, kaMax: number = KA_MAX_SITE): ParsedSearch {
  const raw = Object.fromEntries(new URLSearchParams(search));
  const rejected: string[] = [];
  const shape = urlSchema.shape;
  const ok: Record<string, unknown> = {};
  for (const key of Object.keys(shape) as (keyof typeof shape)[]) {
    if (!(key in raw)) continue;
    const r = shape[key].safeParse(raw[key]);
    if (r.success && !(key === "ka" && (r.data as number) > kaMax)) ok[key] = r.data;
    else rejected.push(key);
  }
  const character = ok.c as string | undefined;
  const ka = ok.ka as number | undefined;
  if (character == null || ka == null || !knownCharacters.has(character)) {
    return { conditions: null, rejected: character != null && !knownCharacters.has(character) ? [...rejected, "c"] : rejected };
  }
  const c = defaultConditions(character, ka);
  const v = ok as Partial<z.infer<typeof urlSchema>>;
  if (v.mode != null) c.mode = v.mode;
  if (v.n_min != null) c.nMin = v.n_min;
  if (v.n_max != null) c.nMax = v.n_max;
  if (v.g_min != null) c.gMin = v.g_min;
  if (v.g_max != null) c.gMax = v.g_max;
  if (v.remain != null) c.remain = v.remain;
  if (v.sj != null) {
    if (c.mode === "filler") c.safeJump = v.sj;
    else rejected.push("sj"); // 安全飛びは有利調整だけ
  }
  if (v.types != null) c.targetTypes = TARGET_TYPE_ORDER.filter((t) => v.types?.includes(t));
  if (v.zero) c.includeZeroDamage = true;
  if (v.move != null) c.targetMove = v.move;
  if (v.sort != null) c.sort = v.sort;
  if (v.approach) c.approachOnly = true;
  if (v.block_safe) {
    if (c.mode === "meaty") c.blockSafeOnly = true;
    else rejected.push("block_safe"); // 重ねるだけ
  }
  if (v.chain) c.chainCancelOnly = true;
  // 種別 0 個は、特定の技を選んでいないとき・有利調整でないときは画面で押せない(validateForm)ので既定に戻す。
  if (c.targetTypes.length === 0 && c.mode !== "filler" && c.targetMove == null) {
    c.targetTypes = [...DEFAULT_TARGET_TYPES];
    rejected.push("types");
  }
  // 最大 < 最小は画面では押せない条件(必ず 0 件)なので、既定に戻す。画面の欄の検査(validateForm)と同じく、
  // いまのモードが使う欄だけを見る(使わない欄の値はエンジンに送らない)。
  if (c.mode === "meaty" && c.nMax > 0 && c.nMax < c.nMin) {
    c.nMax = 0;
    rejected.push("n_max");
  }
  if (c.mode === "gap" && c.gMax < c.gMin) {
    c.gMin = GAP_G_MIN;
    c.gMax = GAP_G_MAX;
    rejected.push("g_min", "g_max");
  }
  return { conditions: c, rejected };
}

/** 数値の欄の文字列を整数にする。[min, max] の整数だけを採り、空欄・小数・負・数字以外・範囲外は null(本家と同じ)。 */
export function parseIntInRange(text: string, min: number, max = Number.MAX_SAFE_INTEGER): number | null {
  const v = text.trim();
  if (!/^\d+$/.test(v)) return null;
  const n = Number(v);
  return n >= min && n <= max ? n : null;
}
