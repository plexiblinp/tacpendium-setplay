// Cloudflare の _headers 形式を読む(E2E の静的サーバと試験で使う)。対応するのは、このリポジトリで使う形だけ:
//   行頭が空白でない行 = パスの規則(* は任意の文字列)、続く字下げの行 = "Name: value"、# で始まる行は注記。
// 同じパスに当たる規則はすべて足し合わせ、同じ名前のヘッダは ", " で連結する(Cloudflare の _headers の説明に合わせた。
// 推測: 実配信での確認は S7 のヘッダ診断で行う。今のファイルに同じ名前の重複は無い)。

/** _headers の本文を [{ pattern: RegExp, source: string, headers: [name, value][] }] にする。 */
export function parseHeaders(text) {
  const rules = [];
  let cur = null;
  for (const [i, raw] of text.split(/\r?\n/).entries()) {
    const line = raw.replace(/\s+$/, "");
    if (line.trim() === "" || line.trim().startsWith("#")) continue;
    if (!/^\s/.test(line)) {
      const source = line.trim();
      if (!source.startsWith("/")) throw new Error(`_headers ${i + 1} 行: パスは / で始めること: ${source}`);
      const re = new RegExp(`^${source.split("*").map(escapeRegExp).join(".*")}$`);
      cur = { pattern: re, source, headers: [] };
      rules.push(cur);
      continue;
    }
    if (cur == null) throw new Error(`_headers ${i + 1} 行: パスの前にヘッダがある`);
    const m = /^\s+([A-Za-z0-9-]+):\s*(.+)$/.exec(line);
    if (m == null) throw new Error(`_headers ${i + 1} 行: "Name: value" の形でない: ${line}`);
    cur.headers.push([m[1], m[2]]);
  }
  return rules;
}

/** パスに当たるヘッダ(規則の順に足し合わせ。同じ名前は ", " で連結)。 */
export function headersFor(rules, pathname) {
  const out = new Map();
  for (const r of rules) {
    if (!r.pattern.test(pathname)) continue;
    for (const [k, v] of r.headers) {
      const prev = out.get(k.toLowerCase());
      out.set(k.toLowerCase(), prev ? [prev[0], `${prev[1]}, ${v}`] : [k, v]);
    }
  }
  return [...out.values()];
}

function escapeRegExp(s) {
  return s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}
