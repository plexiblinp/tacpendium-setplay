// 配信物全体(フロントのバンドル + WASM + 事前計算のデータ)が Cloudflare 無料の静的アセットの上限に収まるかを数える
// (S1 ⑥: 20,000 ファイル・1 ファイル 25 MiB。cmd/precompute はデータだけを数えるので、ここで全体を数え直す。S4 レビュー低-3)。
import fs from "node:fs";
import path from "node:path";

export const MAX_FILES = 20000;
export const MAX_FILE_BYTES = 25 * 1024 * 1024;
/** Cloudflare が配信のファイルとして数えないもの(設定ファイル)。 */
export const CONFIG_FILES = new Set(["_headers", "_redirects"]);

/** dir 以下の配るファイルを [相対パス, バイト数] で返す。 */
export function listFiles(dir) {
  const out = [];
  const walk = (d) => {
    for (const e of fs.readdirSync(d, { withFileTypes: true })) {
      const p = path.join(d, e.name);
      if (e.isDirectory()) walk(p);
      else if (e.isFile()) out.push([path.relative(dir, p).split(path.sep).join("/"), fs.statSync(p).size]);
    }
  };
  walk(dir);
  return out.filter(([rel]) => !CONFIG_FILES.has(rel));
}

/** 上限の違反を文字列の配列で返す(空なら違反なし)。 */
export function checkLimits(files, maxFiles = MAX_FILES, maxBytes = MAX_FILE_BYTES) {
  const v = [];
  if (files.length > maxFiles) v.push(`ファイル数 ${files.length} が上限 ${maxFiles} を超える`);
  for (const [rel, size] of files) if (size > maxBytes) v.push(`${rel}: ${size} bytes が上限 ${maxBytes} を超える`);
  return v;
}
