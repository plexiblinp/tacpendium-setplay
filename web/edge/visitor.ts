// 利用者の推定(HANDOVER D14・§7.3): hash(IP + UA + 日替わりソルト)。生の IP は保存しない。
// 回線の鍵(S9 のセキュリティ修正 F-01): hash("net" + 回線 + 日替わりソルト)。回線は IPv4 ならアドレス、IPv6 なら先頭 /64。
//   UA は送る側が自由に変えられるので、利用者のハッシュだけを鍵にすると UA を変えるだけで上限を逃れられる。
//   回数制限と 1 日の票の上限は、回線の鍵でも数える(共有 IP を考えて値は緩め。edge/index.ts・votes.ts)。
// ソルトは D1 に日付ごとに 1 つ置き、日付が変わった後の最初の要求で前日以前のソルト・投票の記録(ballots)・
// 回線ごとの票の数(quotas)を消す(遅延削除)。
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

/** IPv6 の回線として扱う先頭の 16 ビットの区切りの数(/64)。 */
const IPV6_NET_GROUPS = 4;
const IPV4_RE = /^(\d{1,3})\.(\d{1,3})\.(\d{1,3})\.(\d{1,3})$/;
const IPV4_MAPPED_RE = /^::ffff:(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})$/i;

/** IPv6 を 8 つの 16 進の区切りに広げる(:: を埋める・末尾の IPv4 表記を直す)。形が誤りなら null。 */
function expandIPv6(ip: string): number[] | null {
  let text = ip.toLowerCase();
  const zone = text.indexOf("%");
  if (zone >= 0) text = text.slice(0, zone);
  const v4 = text.match(/^(.*:)(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})$/);
  if (v4 != null) {
    const parts = v4[2]!.split(".").map(Number);
    if (parts.some((x) => x > 255)) return null;
    text = `${v4[1]}${((parts[0]! << 8) | parts[1]!).toString(16)}:${((parts[2]! << 8) | parts[3]!).toString(16)}`;
  }
  const halves = text.split("::");
  if (halves.length > 2) return null;
  const parse = (h: string) => (h === "" ? [] : h.split(":"));
  const head = parse(halves[0]!);
  const tail = halves.length === 2 ? parse(halves[1]!) : [];
  const fill = 8 - head.length - tail.length;
  if ((halves.length === 2 && fill < 1) || (halves.length === 1 && fill !== 0)) return null;
  const groups = [...head, ...Array<string>(halves.length === 2 ? fill : 0).fill("0"), ...tail];
  if (groups.some((g) => !/^[0-9a-f]{1,4}$/.test(g))) return null;
  return groups.map((g) => parseInt(g, 16));
}

/**
 * 回線の鍵(ハッシュの前の値。保存しない)。IPv4 はアドレスそのまま、IPv6 は先頭 /64(1 つの契約に配られる単位)。
 * IPv4 を写した IPv6(::ffff:a.b.c.d)は IPv4 として扱う。形が分からない値は、そのまま 1 つの回線とする。
 */
export function netOf(ip: string): string {
  const mapped = ip.match(IPV4_MAPPED_RE);
  const v4 = mapped != null ? mapped[1]! : ip;
  const m = v4.match(IPV4_RE);
  if (m != null && m.slice(1).every((x) => Number(x) <= 255)) return `4:${m.slice(1).map(Number).join(".")}`;
  const g = ip.includes(":") ? expandIPv6(ip) : null;
  if (g != null) return `6:${g.slice(0, IPV6_NET_GROUPS).map((x) => x.toString(16)).join(":")}::/64`;
  return `?:${ip}`;
}

/** 回線のハッシュ(回数制限・回線ごとの 1 日の票の上限の鍵)。利用者のハッシュとは別の文字列から作る。 */
export async function netHash(ip: string, salt: string): Promise<string> {
  return (await sha256Hex(`net\n${netOf(ip)}\n${salt}`)).slice(0, VISITOR_HEX_LEN);
}

/**
 * 今日のソルトを D1 から引く(無ければ作る)。isolate の中では日付ごとに覚えて D1 を読み直さない。
 * 日が変わって初めて引いたときに、前日以前のソルト・投票の記録・回線ごとの票の数を消す。
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
      db.prepare("DELETE FROM quotas WHERE day < ?").bind(day),
    ]);
    const row = await db.prepare("SELECT salt FROM salts WHERE day = ?").bind(day).first<{ salt: string }>();
    if (row == null) throw new Error("salt: 今日のソルトを読めない");
    this.cached = { day, salt: row.salt };
    return row.salt;
  }
}
