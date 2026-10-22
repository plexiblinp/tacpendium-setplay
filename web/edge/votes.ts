// 票(HANDOVER D17・開発者決定 2026-10-12): D1 に保存し、キャラ・KA ごとに配る。
// 重複防止は「利用者のハッシュ × レシピ × 日」で 1 票(同じ日は付け替え・取り消しができる)。1 人 1 日の票数に上限。
// 回線(IPv4 のアドレス・IPv6 の /64)ごとにも 1 日の新しい票の数に上限を置く(S9 のセキュリティ修正 F-01。UA を変えて
// 利用者のハッシュを変えれば 1 人の上限は逃れられるため)。数は quotas に UPSERT で数え、日替わりで消す(visitor.ts)。
// ★同じ人の同時の投票が重なると数がずれうる(読んでから書く 2 段のため)。利用者が少なく正確さは要らない
//   (開発者注)ので排他はしない。docs/telemetry.md の限界。
import { VOTES_MAX_ENTRIES, type VoteReply, type VoteRequest, type VoteValue, type VotesReply } from "../src/shared/api";
import type { D1Database } from "./env";

/** 1 人(利用者のハッシュ)が 1 日に票を付けられるレシピの数。 */
export const VOTES_PER_VOTER_PER_DAY = 200;
/**
 * 1 回線(回線のハッシュ)が 1 日に付けられる新しい票の数。1 人の上限の 5 倍: 家庭・学校・携帯の共有 IP(CGNAT)で
 * 数人が同じ日に使っても当たらず、UA を変えた水増しは 1 回線 1 日この数で止まる。取り消しても戻らない(数え直さない)。
 */
export const VOTES_PER_NET_PER_DAY = 1000;

export type CastResult = { kind: "ok"; reply: VoteReply } | { kind: "limited" };

/** 票を付ける・付け替える・取り消す(value 0)。voter は利用者のハッシュ、net は回線のハッシュ(visitor.ts)。 */
export async function castVote(db: D1Database, day: string, voter: string, net: string, req: VoteRequest): Promise<CastResult> {
  const prev = await db
    .prepare("SELECT value FROM ballots WHERE day = ? AND voter = ? AND char = ? AND ka = ? AND recipe = ?")
    .bind(day, voter, req.char, req.ka, req.recipe)
    .first<{ value: number }>();
  const before: VoteValue = prev == null ? 0 : prev.value === 1 ? 1 : -1;
  const isNew = prev == null && req.value !== 0;
  if (isNew) {
    const used = await db
      .prepare("SELECT COUNT(*) AS n FROM ballots WHERE day = ? AND voter = ?")
      .bind(day, voter)
      .first<{ n: number }>();
    if ((used?.n ?? 0) >= VOTES_PER_VOTER_PER_DAY) return { kind: "limited" };
    const netUsed = await db.prepare("SELECT n FROM quotas WHERE day = ? AND net = ?").bind(day, net).first<{ n: number }>();
    if ((netUsed?.n ?? 0) >= VOTES_PER_NET_PER_DAY) return { kind: "limited" };
  }
  const dGood = (req.value === 1 ? 1 : 0) - (before === 1 ? 1 : 0);
  const dBad = (req.value === -1 ? 1 : 0) - (before === -1 ? 1 : 0);
  const stmts = [];
  if (isNew) {
    stmts.push(
      db
        .prepare("INSERT INTO quotas (day, net, n) VALUES (?, ?, 1) ON CONFLICT (day, net) DO UPDATE SET n = n + 1")
        .bind(day, net),
    );
  }
  if (req.value === 0) {
    stmts.push(
      db
        .prepare("DELETE FROM ballots WHERE day = ? AND voter = ? AND char = ? AND ka = ? AND recipe = ?")
        .bind(day, voter, req.char, req.ka, req.recipe),
    );
  } else {
    stmts.push(
      db
        .prepare(
          "INSERT INTO ballots (day, voter, char, ka, recipe, value) VALUES (?, ?, ?, ?, ?, ?) " +
            "ON CONFLICT (day, voter, char, ka, recipe) DO UPDATE SET value = excluded.value",
        )
        .bind(day, voter, req.char, req.ka, req.recipe, req.value),
    );
  }
  if (dGood !== 0 || dBad !== 0) {
    stmts.push(
      db
        .prepare(
          "INSERT INTO votes (char, ka, recipe, good, bad) VALUES (?, ?, ?, MAX(?, 0), MAX(?, 0)) " +
            "ON CONFLICT (char, ka, recipe) DO UPDATE SET good = MAX(good + ?, 0), bad = MAX(bad + ?, 0)",
        )
        .bind(req.char, req.ka, req.recipe, dGood, dBad, dGood, dBad),
    );
  }
  await db.batch(stmts);
  const now = await db
    .prepare("SELECT good, bad FROM votes WHERE char = ? AND ka = ? AND recipe = ?")
    .bind(req.char, req.ka, req.recipe)
    .first<{ good: number; bad: number }>();
  return { kind: "ok", reply: { good: now?.good ?? 0, bad: now?.bad ?? 0, value: req.value } };
}

/**
 * キャラ・KA の票を配る(主キーの前方一致で、そのキャラ・KA の行だけ読む＝D1 の読んだ行数を抑える。S1 ⑦)。
 * 票が 0・0 に戻った行は配らない。上限を超えたら、|good − bad| の大きい順に VOTES_MAX_ENTRIES 件。
 */
export async function listVotes(db: D1Database, char: string, ka: number): Promise<VotesReply> {
  const { results } = await db
    .prepare(
      "SELECT recipe, good, bad FROM votes WHERE char = ? AND ka = ? AND (good > 0 OR bad > 0) " +
        "ORDER BY ABS(good - bad) DESC, recipe LIMIT ?",
    )
    .bind(char, ka, VOTES_MAX_ENTRIES)
    .all<{ recipe: string; good: number; bad: number }>();
  const votes: VotesReply["votes"] = {};
  for (const r of results) votes[r.recipe] = [r.good, r.bad];
  return { votes };
}
