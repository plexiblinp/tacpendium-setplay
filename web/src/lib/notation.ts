// 技の表記(HANDOVER D6)とレシピの文字列化(画面の行とコピー(D16)の中身)。
import type { Move, NotationKind } from "./data";
import type { Item, Step } from "./engineTypes";

/** UI 言語ごとの既定の表記。ja は日本語技名、それ以外は SRK(HANDOVER §4.4)。 */
export function defaultNotation(lang: string): NotationKind {
  return lang === "ja" ? "nameJa" : "srk";
}

/** レシピの区切り(本家の画面と同じ)。 */
export const RECIPE_SEPARATOR = " > ";

export type MovesByCode = ReadonlyMap<string, Move>;

export function indexMoves(moves: readonly Move[]): Map<string, Move> {
  return new Map(moves.map((m) => [m.code, m]));
}

/** 技 1 つの表記。moves DB に無い code は code のまま(壊れたデータで画面を落とさない)。 */
export function moveLabel(code: string, moves: MovesByCode, kind: NotationKind): string {
  return moves.get(code)?.notation[kind] ?? code;
}

/** 手順の並びを選択中の表記で 1 行にする。 */
export function recipeText(steps: readonly Step[], moves: MovesByCode, kind: NotationKind): string {
  return steps.map((s) => moveLabel(s.code, moves, kind)).join(RECIPE_SEPARATOR);
}

/** 行の安定した鍵(React の key)。手順の code 列 + N(+ 安全飛びの種類)。本家の suggestionKey と同じ考え方。 */
export function itemKey(item: Item, safeJump: string | null): string {
  return `${safeJump ? `${safeJump}:` : ""}${item.steps.map((s) => s.code).join("-")}#${item.n}`;
}

/** コピーするテキスト: レシピ(選択中の表記)と、行のフレームの説明(画面と同じ文言)。 */
export function copyText(recipe: string, frameLabel: string): string {
  return `${recipe} (${frameLabel})`;
}
