// 利用者の推定(HANDOVER D14・§7.3): hash(IP + UA + 日替わりソルト)。生の IP は保存しない。
// ソルトは D1 に日付ごとに 1 つ置き、日付が変わった後の最初の要求で前日以前のソルトと投票の記録(ballots)を消す(遅延削除)。
// ソルトが消えれば、前日のハッシュから IP を当てることも、日をまたいで同じ人を結ぶことも難しくなる。ただし D1 の
// 復旧用の記録(Time Travel。推測・未確認)に一定期間残りうる(docs/telemetry.md)。
import type { D1Database } from "./env";

/** ハッシュの桁数(16 進)。128 ビット。 */
const VISITOR_HEX_LEN = 32;
/** ソルトのバイト数。 */
const SALT_BYTES = 32;

/** UTC の日付(YYYY-MM-DD)。 */
export function utcDay(now: Date): string {
  return now.toISOString().slice(0, 10);
}

function toHex(buf: ArrayBuffer | Uint8Array): string {
  const b = buf instanceof Uint8Array ? buf : new Uint8Array(buf);
  return Array.from(b, (x) => x.toString(16).padStart(2, "0")).join("");
}

export async function sha256Hex(text: string): Promise<string> {
  return toHex(await crypto.subtle.digest("SHA-256", new TextEncoder().encode(text)));
}

/** 利用者のハッシュ。IP・UA が無いときは空文字として混ぜる。 */
export async function visitorHash(ip: string, ua: string, salt: string): Promise<string> {
  return (await sha256Hex(`${ip}\n${ua}\n${salt}`)).slice(0, VISITOR_HEX_LEN);
}

/**
 * 今日のソルトを D1 から引く(無ければ作る)。isolate の中では日付ごとに覚えて D1 を読み直さない。
 * 日が変わって初めて引いたときに、前日以前のソルトと投票の記録を消す。
 */
export class SaltStore {
  private cached: { day: string; salt: string } | null = null;

  async today(db: D1Database, now: Date): Promise<string> {
    const day = utcDay(now);
    if (this.cached?.day === day) return this.cached.salt;
    const fresh = toHex(crypto.getRandomValues(new Uint8Array(SALT_BYTES)));
    await db.batch([
      db.prepare("INSERT OR IGNORE INTO salts (day, salt) VALUES (?, ?)").bind(day, fresh),
      db.prepare("DELETE FROM salts WHERE day < ?").bind(day),
      db.prepare("DELETE FROM ballots WHERE day < ?").bind(day),
    ]);
    const row = await db.prepare("SELECT salt FROM salts WHERE day = ?").bind(day).first<{ salt: string }>();
    if (row == null) throw new Error("salt: 今日のソルトを読めない");
    this.cached = { day, salt: row.salt };
    return row.salt;
  }
}
