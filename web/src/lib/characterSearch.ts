// キャラの名前検索(S5 目視の要望・S7。本家に揃える＝公開版 tacpendium の web/src/lib/text-filter.ts の
// normalizeForSearch・matchesWordPrefix・filterByWordPrefix を移植)。
// 規則: 正規化は NFKC → trim → 小文字。一致は「語の先頭一致」(全体の先頭か、区切りで分けた各語の先頭)。
// ⇒ 「ry」は ryu に当たり terry に当たらない。「viper」「ヴァイパー」は C.ヴァイパー に当たる。
import type { ManifestCharacter } from "./data";

/** 検索語・被検索文字列に共通で掛ける正規化。 */
export function normalizeForSearch(value: string): string {
  return value.normalize("NFKC").trim().toLowerCase();
}

/** 語の区切り。空白・ピリオド・下線・ハイフン・中黒。 */
const WORD_SEPARATOR = /[\s._\-・]+/u;

/** text が query の「語の先頭一致」か。空の query は常に真。 */
export function matchesWordPrefix(text: string, query: string): boolean {
  const q = normalizeForSearch(query);
  if (q === "") return true;
  const t = normalizeForSearch(text);
  if (t.startsWith(q)) return true;
  return t.split(WORD_SEPARATOR).some((word) => word.startsWith(q));
}

/** キャラを日本語名・英語名・code の語の先頭一致で絞り込む。並び順は入力のまま。 */
export function filterCharacters(
  characters: readonly ManifestCharacter[],
  query: string,
): ManifestCharacter[] {
  if (normalizeForSearch(query) === "") return [...characters];
  return characters.filter((c) => [c.name.ja, c.name.en, c.code].some((text) => matchesWordPrefix(text, query)));
}
