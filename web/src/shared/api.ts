// 画面と Cloudflare Worker(web/edge/)の間の受け口の形(S6・HANDOVER §7.1・D17)。
// 画面は送る前に、Worker は受けた後に、同じ zod の形で検証する(形の正本はここ 1 か所)。
// ★Worker も読むので、i18next・React を読み込むモジュールを import しないこと。
import { z } from "zod";

import { SAFE_JUMP_KINDS, TARGET_TYPE_ORDER } from "../lib/conditions";
import { GAP_G_MAX, GAP_G_MIN, KA_MAX, KA_MIN, N_MIN_LOWER } from "../lib/constants";
import { NOTATION_KINDS } from "../lib/data";
import { LANGS } from "../lib/langs";

export const API_BEACON_PATH = "/api/beacon";
export const API_VOTE_PATH = "/api/vote";
export const API_VOTES_PATH = "/api/votes";

/** 受け口の本文の上限(HANDOVER §7.1「1KB 以下」)。投票も同じ上限。 */
export const API_BODY_MAX_BYTES = 1024;

/** beacon の形の版。形を変えたら上げる(集計は版で分ける)。 */
export const BEACON_SCHEMA_VERSION = 1;

/** 技 code・キャラ code・誤りの code の形(英小文字・数字・_)。 */
const CODE_RE = /^[a-z0-9_]{1,64}$/;
const code = z.string().regex(CODE_RE);

/** 所要(ミリ秒)の上限。これを超える値は壊れた入力として捨てる(計算の実測の最大は 8 秒台。S5)。 */
export const BEACON_MS_MAX = 600_000;
/** 件数の上限(エンジンの結果の総数。KA 100 の上限ありの最大は数十万。S1 ⑧)。 */
export const BEACON_COUNT_MAX = 10_000_000;
/** N の上限(持続の長い技の弾の持続でも 3 桁に収まる)。 */
export const N_MAX_LIMIT = 999;

export const OUTCOMES = ["result", "error", "canceled"] as const;
export type Outcome = (typeof OUTCOMES)[number];

/** 票の読み込みの結果(計算に票を使えたか)。none = 票の付いたレシピが無かった、unavailable = 読めなかった(本家の順で出した)。 */
export const VOTES_STATUSES = ["ok", "none", "unavailable"] as const;
export type VotesStatus = (typeof VOTES_STATUSES)[number];

const ms = z.number().finite().nonnegative().max(BEACON_MS_MAX);
const count = z.number().int().nonnegative().max(BEACON_COUNT_MAX);

/**
 * 「提案を出す」1 回分の計測(HANDOVER §7.1)。null は「その実行では当てはまらない」(例: 重ねる以外の nMin、
 * 結果が出なかった実行の件数)で、0 とは区別する(AGENTS §8-7)。
 */
export const beaconSchema = z.strictObject({
  v: z.literal(BEACON_SCHEMA_VERSION),
  char: code,
  ka: z.number().int().min(KA_MIN).max(KA_MAX),
  mode: z.enum(["meaty", "gap", "filler"]),
  nMin: z.number().int().min(N_MIN_LOWER).max(N_MAX_LIMIT).nullable(),
  /** 0 は「上限なし」(本家と同じ)。 */
  nMax: z.number().int().min(0).max(N_MAX_LIMIT).nullable(),
  gMin: z.number().int().min(GAP_G_MIN).max(GAP_G_MAX).nullable(),
  gMax: z.number().int().min(GAP_G_MIN).max(GAP_G_MAX).nullable(),
  remain: z.number().int().min(0).max(KA_MAX).nullable(),
  safeJump: z.enum(SAFE_JUMP_KINDS).nullable(),
  targetTypes: z.array(z.enum(TARGET_TYPE_ORDER)).max(TARGET_TYPE_ORDER.length),
  targetMove: code.nullable(),
  includeZeroDamage: z.boolean(),
  approachOnly: z.boolean(),
  blockSafeOnly: z.boolean(),
  chainCancelOnly: z.boolean(),
  sort: z.enum(["n", "target"]),
  notation: z.enum(NOTATION_KINDS),
  lang: z.enum(LANGS),
  outcome: z.enum(OUTCOMES),
  errorCode: code.nullable(),
  /** 結果の総数(engine の totalFound)。 */
  resultCount: count.nullable(),
  /** 画面が受け取った件数(上限 3000)。 */
  items: count.nullable(),
  truncated: z.boolean().nullable(),
  cacheHit: z.boolean(),
  /** 押してから結果が出るまで(メインスレッドの経過)。キャッシュから出したときも測る。 */
  ms: ms.nullable(),
  /** Worker の中の計算だけ(キャッシュから出したときは null)。 */
  computeMs: ms.nullable(),
  votesStatus: z.enum(VOTES_STATUSES),
  /** 計算に渡した票の付いたレシピの数。 */
  votesApplied: count,
  dataVersion: z.string().regex(/^[A-Za-z0-9._-]{1,64}$/),
  /** 送った時の抽出率(0〜1]。集計は 1 / sampleRate で重み付けする。 */
  sampleRate: z.number().gt(0).max(1),
});
export type BeaconEvent = z.infer<typeof beaconSchema>;

/** レシピの鍵(engine の RecipeKey): 「当てる技以外の技 code を , で連結」|「当てる技の code」。どちらかは空でない。 */
export const RECIPE_KEY_RE = /^(?=.*[a-z0-9_])([a-z0-9_]+(,[a-z0-9_]+)*)?\|([a-z0-9_]+)?$/;
/** 鍵の長さの上限(engine.MaxRecipeKeyLen と同じ)。 */
export const RECIPE_KEY_MAX = 1024;
/** 1 キャラ・1 KA で配る票の付いたレシピの上限(engine.MaxVoteEntries と同じ)。 */
export const VOTES_MAX_ENTRIES = 10_000;

export const recipeKeySchema = z.string().max(RECIPE_KEY_MAX).regex(RECIPE_KEY_RE);

/** 票の値。1 = グッド、-1 = バッド、0 = 取り消し。 */
export const VOTE_VALUES = [1, -1, 0] as const;
export type VoteValue = (typeof VOTE_VALUES)[number];

export const voteRequestSchema = z.strictObject({
  char: code,
  ka: z.number().int().min(KA_MIN).max(KA_MAX),
  recipe: recipeKeySchema,
  value: z.union([z.literal(1), z.literal(-1), z.literal(0)]),
});
export type VoteRequest = z.infer<typeof voteRequestSchema>;

/** 1 つのレシピの票の数。 */
export interface VoteCount {
  good: number;
  bad: number;
}

/** POST /api/vote の応答。value はこの人の今日の票(付け替え・取り消しの後)。画面は応答をこの形で検証する。 */
export const voteReplySchema = z.strictObject({
  good: z.number().int().nonnegative(),
  bad: z.number().int().nonnegative(),
  value: z.union([z.literal(1), z.literal(-1), z.literal(0)]),
});
export type VoteReply = z.infer<typeof voteReplySchema>;

/** GET /api/votes?char=&ka= の応答。鍵 → [good, bad]。 */
export const votesReplySchema = z.strictObject({
  votes: z.record(recipeKeySchema, z.tuple([count, count])),
});
export type VotesReply = z.infer<typeof votesReplySchema>;

/** 票のスコア(並べ替えの第一キー。開発者決定 2026-10-12: good − bad)。 */
export function voteScore(c: VoteCount): number {
  return c.good - c.bad;
}

/** 誤りの応答の形。 */
export interface ApiErrorReply {
  error: string;
}
