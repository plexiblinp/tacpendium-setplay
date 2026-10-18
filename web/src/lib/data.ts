// 配信データ(事前計算の manifest と moves DB)の形。正本は docs/precompute.md・docs/data/moves-db.md。
// 読み込み時に zod で検証する(形が変わったら黙って壊れず、読み込みの誤りとして出す)。
import { z } from "zod";

const localizedName = z.object({ ja: z.string(), en: z.string() });

export const safeJumpSchema = z.object({
  true: z.number().int().nonnegative(),
  f5: z.number().int().nonnegative(),
  f6: z.number().int().nonnegative(),
});
export type SafeJump = z.infer<typeof safeJumpSchema>;

export const manifestCharacterSchema = z.object({
  code: z.string().min(1),
  name: localizedName,
  dataVersion: z.string(),
  safeJump: safeJumpSchema.nullable(),
  moves: z.string().min(1),
});
export type ManifestCharacter = z.infer<typeof manifestCharacterSchema>;

/** manifest の形の版。cmd/precompute の schemaVersion と揃える。 */
export const MANIFEST_SCHEMA_VERSION = 1;

export const manifestSchema = z.object({
  schemaVersion: z.literal(MANIFEST_SCHEMA_VERSION),
  dataVersion: z.string(),
  maxKnockdownAdvantage: z.number().int(),
  characters: z.array(manifestCharacterSchema).min(1),
});
export type Manifest = z.infer<typeof manifestSchema>;

export const notationSchema = z.object({
  nameJa: z.string(),
  nameEn: z.string(),
  srk: z.string(),
  numeric: z.string(),
});

export const moveSchema = z.object({
  code: z.string().min(1),
  category: z.string(),
  damage: z.number().nullable(),
  notation: notationSchema,
});
export type Move = z.infer<typeof moveSchema>;

export const movesDbSchema = z.object({
  character: z.object({ code: z.string(), name: localizedName }),
  moves: z.array(moveSchema),
});
export type MovesDb = z.infer<typeof movesDbSchema>;

/** 技の 4 表記(HANDOVER D6)。キーは moves DB の notation と同じ。 */
export const NOTATION_KINDS = ["nameJa", "nameEn", "srk", "numeric"] as const;
export type NotationKind = (typeof NOTATION_KINDS)[number];

/** 技の区分(moves DB の category)。画面に出す順。特定の技の選択では system を出さない(本家と同じ)。 */
export const MOVE_CATEGORY_ORDER = [
  "normal",
  "unique",
  "target_combo",
  "special",
  "super_art",
  "critical_art",
  "throw",
  "drive_impact",
  "rush_variant",
  "system",
] as const;
export const PICKER_EXCLUDED_CATEGORY = "system";

/** 相対パス(manifest からの)を、manifest の URL を基準に絶対 URL へ。 */
export function resolveDataUrl(manifestUrl: string, relative: string): string {
  return new URL(relative, manifestUrl).href;
}
