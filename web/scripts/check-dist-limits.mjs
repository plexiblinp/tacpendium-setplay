// 使い方: node scripts/check-dist-limits.mjs <dist>   違反があれば exit 1。最後の行は「違反: N 件」。
import { checkLimits, listFiles, MAX_FILE_BYTES, MAX_FILES } from "./dist-limits.mjs";

const dir = process.argv[2];
if (!dir) {
  console.error("使い方: node scripts/check-dist-limits.mjs <dist>");
  process.exit(2);
}
const files = listFiles(dir);
if (files.length === 0) {
  console.error(`${dir}: 配るファイルが 0 件(ビルドされていない)`);
  process.exit(2);
}
const total = files.reduce((s, [, n]) => s + n, 0);
const [maxRel, maxSize] = files.reduce((a, b) => (b[1] > a[1] ? b : a));
console.log(`ファイル: ${files.length}(上限 ${MAX_FILES})`);
console.log(`最大: ${maxRel} ${maxSize} bytes(上限 ${MAX_FILE_BYTES})`);
console.log(`合計: ${total} bytes`);
const v = checkLimits(files);
for (const x of v) console.log(x);
console.log(`違反: ${v.length} 件`);
process.exit(v.length === 0 ? 0 : 1);
