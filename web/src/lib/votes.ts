// 票(HANDOVER D17)の取得と投票。受け口は同じオリジンの Worker(CSP の connect-src 'self')。
// ★票が読めなくても提案は止めない: 時間切れ・誤りは「票なし」(本家の順)で計算する(HANDOVER §3)。
import {
  API_VOTE_PATH,
  API_VOTES_PATH,
  voteScore,
  voteReplySchema,
  votesReplySchema,
  type VoteReply,
  type VotesStatus,
  type VoteValue,
} from "../shared/api";
import type { FetchFn } from "./loader";

/** 票の取得を待つ上限(ms)。超えたら票なしで計算する(提案を待たせない)。 */
export const VOTES_TIMEOUT_MS = 1500;

/** 鍵 → [good, bad]。 */
export type VoteCounts = Record<string, readonly [number, number]>;

export interface LoadedVotes {
  status: VotesStatus;
  counts: VoteCounts;
}

/** キャラ・KA の票を読む。誤り・時間切れ・形の違いは status: "unavailable"(counts は空)。 */
export async function fetchVotes(fetchFn: FetchFn, baseUrl: string, char: string, ka: number, timeoutMs = VOTES_TIMEOUT_MS): Promise<LoadedVotes> {
  const ctrl = new AbortController();
  const timer = setTimeout(() => ctrl.abort(), timeoutMs);
  try {
    const url = new URL(API_VOTES_PATH, baseUrl);
    url.searchParams.set("char", char);
    url.searchParams.set("ka", String(ka));
    const res = await fetchFn(url.href, { signal: ctrl.signal });
    if (!res.ok) return { status: "unavailable", counts: {} };
    const r = votesReplySchema.safeParse(await res.json());
    if (!r.success) return { status: "unavailable", counts: {} };
    const counts = r.data.votes;
    return { status: Object.keys(counts).length > 0 ? "ok" : "none", counts };
  } catch {
    return { status: "unavailable", counts: {} };
  } finally {
    clearTimeout(timer);
  }
}

/** エンジンに渡す票(鍵 → good − bad)。スコア 0 の鍵は並びを変えないので渡さない。 */
export function scoresForEngine(counts: VoteCounts): Record<string, number> {
  const out: Record<string, number> = {};
  for (const [k, [good, bad]] of Object.entries(counts)) {
    const s = voteScore({ good, bad });
    if (s !== 0) out[k] = s;
  }
  return out;
}

/** 票を送る。成功ならそのレシピの新しい数とこの人の票を返す。誤りは null(画面は「送れなかった」を出す)。 */
export async function postVote(fetchFn: FetchFn, baseUrl: string, char: string, ka: number, recipe: string, value: VoteValue): Promise<VoteReply | null> {
  try {
    const res = await fetchFn(new URL(API_VOTE_PATH, baseUrl).href, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ char, ka, recipe, value }),
    });
    if (!res.ok) return null;
    const r = voteReplySchema.safeParse(await res.json());
    return r.success ? r.data : null;
  } catch {
    return null;
  }
}
