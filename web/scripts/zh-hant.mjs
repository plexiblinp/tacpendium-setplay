// zh-Hans の文言から zh-Hant を作る(HANDOVER §6: OpenCC の s2twp = 台湾の語彙にも変換)。
// opencc-js の { from: "cn", to: "twp" } が OpenCC の s2twp に当たる。値だけを変換し、キーと {{..}} の差し込みはそのまま。
import * as OpenCC from "opencc-js";

const convert = OpenCC.Converter({ from: "cn", to: "twp" });

/** 値の文字列だけを変換した新しいオブジェクトを返す(入れ子を保つ)。 */
export function toTraditional(value) {
  if (typeof value === "string") {
    // {{name}} の中身(変数名)は変換しない。
    return value
      .split(/(\{\{[^}]*\}\})/)
      .map((part) => (part.startsWith("{{") ? part : convert(part)))
      .join("");
  }
  if (Array.isArray(value)) return value.map(toTraditional);
  if (value !== null && typeof value === "object") {
    return Object.fromEntries(Object.entries(value).map(([k, v]) => [k, toTraditional(v)]));
  }
  return value;
}

/** ファイルに書く形(2 字下げ・末尾改行)。 */
export function renderLocale(obj) {
  return `${JSON.stringify(obj, null, 2)}\n`;
}
