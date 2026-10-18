// 配信データの読み込み(manifest と moves DB)。形は zod で検証する。
import { MANIFEST_PATH } from "./constants";
import { manifestSchema, movesDbSchema, resolveDataUrl, type Manifest, type MovesDb } from "./data";

export type FetchFn = (url: string, init?: RequestInit) => Promise<Response>;

export interface LoadedManifest {
  manifest: Manifest;
  /** manifest の絶対 URL(moves の相対パスの基準)。 */
  url: string;
}

async function fetchJson(fetchFn: FetchFn, url: string, init?: RequestInit): Promise<unknown> {
  const res = await fetchFn(url, init);
  if (!res.ok) throw new Error(`${url}: HTTP ${res.status}`);
  return res.json();
}

/** manifest を読む。baseUrl はページの URL(Vite の base を含む)。 */
export async function loadManifest(fetchFn: FetchFn, baseUrl: string): Promise<LoadedManifest> {
  const url = new URL(MANIFEST_PATH, baseUrl).href;
  // manifest は唯一の可変ファイル(docs/precompute.md)。キャッシュがあってもサーバに確かめる。
  const raw = await fetchJson(fetchFn, url, { cache: "no-cache" });
  const r = manifestSchema.safeParse(raw);
  if (!r.success) throw new Error(`manifest.json の形が違う: ${r.error.message}`);
  return { manifest: r.data, url };
}

/** キャラの moves DB を読む(表示と特定の技の選択に使う。計算は Worker が同じ URL を読む)。 */
export async function loadMoves(fetchFn: FetchFn, manifestUrl: string, relative: string): Promise<MovesDb> {
  const raw = await fetchJson(fetchFn, resolveDataUrl(manifestUrl, relative));
  const r = movesDbSchema.safeParse(raw);
  if (!r.success) throw new Error(`${relative} の形が違う: ${r.error.message}`);
  return r.data;
}
