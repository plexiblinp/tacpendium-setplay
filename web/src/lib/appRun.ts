// 画面の最上位(App)の、React に依らない部分: 計測に載せる結果の数・自分の票の表・共有 URL の安全飛び・押し方の判定。
// App の複雑度を下げ、単体で試験できるように切り出した(2026-10-31 の health。docs/audits/scale-baseline.md)。
import type { MyVote } from "../components/Results";
import type { Conditions } from "./conditions";
import type { SafeJump } from "./data";
import type { SuggestResponse } from "./engineTypes";
import type { RunRecord } from "./telemetry";

/** 自分の票の表の鍵(キャラ|KA|レシピの鍵)。画面の表示(Results)と投票(App)で同じ鍵を使う。 */
export function myVoteKey(character: string, ka: number, recipe: string): string {
  return `${character}|${ka}|${recipe}`;
}

/**
 * そのキャラ・KA の自分の票から、数(good・bad)だけを忘れる。押した状態(value)と送信の状態は残す。
 * 票を読み直したら、表示の数は読み直した数を使う(投票の時点の数を残さない。レビュー低-1)。
 */
export function forgetVoteCounts(m: Record<string, MyVote>, character: string, ka: number): Record<string, MyVote> {
  const prefix = myVoteKey(character, ka, "");
  const next: Record<string, MyVote> = {};
  for (const [k, v] of Object.entries(m)) next[k] = k.startsWith(prefix) ? { ...v, good: null, bad: null } : v;
  return next;
}

/** 計測に載せる、結果の数の項目。 */
export type ResultMetrics = Pick<RunRecord, "totalFound" | "items" | "truncated">;

/** 計算の結果から、計測に載せる数を取る。 */
export function resultMetrics(result: SuggestResponse): ResultMetrics {
  return { totalFound: result.totalFound, items: result.items.length, truncated: result.truncated };
}

/** 結果が無い実行(誤り・中断)の数の項目。当てはまらないので null(0 と区別する。AGENTS §8-7)。 */
export const NO_RESULT_METRICS: ResultMetrics = { totalFound: null, items: null, truncated: null };

/**
 * 共有 URL の条件の安全飛びを、キャラの値から引き直す(URL の remain は信じない)。
 * 値の無いキャラ・種類では安全飛びを外す。安全飛びの無い条件はそのまま返す。
 */
export function withSharedSafeJump(c: Conditions, table: SafeJump | null | undefined): Conditions {
  if (c.safeJump == null) return c;
  const v = table?.[c.safeJump];
  return v == null ? { ...c, safeJump: null } : { ...c, remain: v };
}

/** 修飾キーなしの左クリックか(リンクの既定の動き〔新しいタブ等〕を奪わないため)。 */
export function isPlainLeftClick(e: { button: number; metaKey: boolean; ctrlKey: boolean; shiftKey: boolean; altKey: boolean }): boolean {
  return e.button === 0 && !e.metaKey && !e.ctrlKey && !e.shiftKey && !e.altKey;
}
