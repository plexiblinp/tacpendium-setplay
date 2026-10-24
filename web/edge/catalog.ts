// 配信データの目録(S9 のセキュリティ修正 F-03)。票の受け口が、実在するキャラ・技だけを受けるために使う。
// 静的アセット(env.ASSETS)から manifest と moves DB を読み、isolate の中で CATALOG_TTL_MS のあいだ覚える
// (データの更新〔デプロイ〕から最大この時間は古い目録で判定する)。読めないときは覚えず、呼び出し側が 503 にする。
//
// ★鍵の検証は「技 code がそのキャラの moves DB にある」までで、組み合わせが実際の提案に出るかは見ない。
//   取りうる鍵の集合は 1 キャラ・KA 0〜100 で数百万件・数百 MB の見込みで、配信の上限に収まらないため
//   (docs/progress/fix-security-report.md)。実在する技の組み合わせの水増しは、回線ごとの上限(F-01)で抑える。
import { z } from "zod";

import { manifestSchema } from "../src/lib/data";
import type { AssetsFetcher } from "./env";

/** 目録を覚える時間。 */
export const CATALOG_TTL_MS = 5 * 60_000;
/** manifest の配信のパス(web/public/data/manifest.json)。 */
export const MANIFEST_PATH = "/data/manifest.json";

/** 鍵の検証に要る moves DB の部分(技の code だけ。読み込みの CPU を抑えるため、表記などは検証しない)。 */
const moveCodesSchema = z.object({ moves: z.array(z.object({ code: z.string().min(1) })) });

/** 静的アセットを読めない・形が誤り(呼び出し側は 503)。 */
export class CatalogUnavailable extends Error {}

interface Cached<T> {
  at: number;
  value: T;
}

export class Catalog {
  /** キャラの code → moves DB の URL。 */
  private manifest: Cached<Map<string, string>> | null = null;
  /** キャラの code → 技の code の集合。 */
  private readonly codes = new Map<string, Cached<Set<string>>>();

  private static fresh<T>(c: Cached<T> | null | undefined, nowMs: number): c is Cached<T> {
    return c != null && nowMs - c.at < CATALOG_TTL_MS;
  }

  private static async readJson(assets: AssetsFetcher, url: string): Promise<unknown> {
    let res: Response;
    try {
      res = await assets.fetch(new Request(url));
    } catch {
      throw new CatalogUnavailable(`catalog: ${url} を読めない`);
    }
    if (!res.ok) throw new CatalogUnavailable(`catalog: ${url}: HTTP ${res.status}`);
    try {
      return await res.json();
    } catch {
      throw new CatalogUnavailable(`catalog: ${url} が JSON でない`);
    }
  }

  private async characters(assets: AssetsFetcher, origin: string, nowMs: number): Promise<Map<string, string>> {
    if (Catalog.fresh(this.manifest, nowMs)) return this.manifest.value;
    const manifestUrl = new URL(MANIFEST_PATH, origin).href;
    const parsed = manifestSchema.safeParse(await Catalog.readJson(assets, manifestUrl));
    if (!parsed.success) throw new CatalogUnavailable("catalog: manifest の形が誤り");
    const map = new Map(parsed.data.characters.map((c) => [c.code, new URL(c.moves, manifestUrl).href]));
    this.manifest = { at: nowMs, value: map };
    return map;
  }

  /** 配信しているキャラか。目録を読めなければ CatalogUnavailable。 */
  async hasCharacter(assets: AssetsFetcher, origin: string, char: string, nowMs: number): Promise<boolean> {
    return (await this.characters(assets, origin, nowMs)).has(char);
  }

  /** そのキャラの技の code の集合。配信していないキャラは null。目録を読めなければ CatalogUnavailable。 */
  async moveCodes(assets: AssetsFetcher, origin: string, char: string, nowMs: number): Promise<Set<string> | null> {
    const url = (await this.characters(assets, origin, nowMs)).get(char);
    if (url == null) return null;
    const hit = this.codes.get(char);
    if (Catalog.fresh(hit, nowMs)) return hit.value;
    const parsed = moveCodesSchema.safeParse(await Catalog.readJson(assets, url));
    if (!parsed.success) throw new CatalogUnavailable(`catalog: ${char} の moves DB の形が誤り`);
    const set = new Set(parsed.data.moves.map((m) => m.code));
    this.codes.set(char, { at: nowMs, value: set });
    return set;
  }
}

/** 鍵(engine の RecipeKey: 「技 code を , で連結」|「当てる技の code」)の技 code の一覧。形は voteRequestSchema で検査済み。 */
export function recipeCodes(recipe: string): string[] {
  return recipe.split(/[|,]/).filter((c) => c !== "");
}
