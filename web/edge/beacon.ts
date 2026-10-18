// POST /api/beacon: 実行計測(HANDOVER §7.1・D3)。厳格に検証してから Analytics Engine に 1 点書く。
import { beaconSchema, type BeaconEvent } from "../src/shared/api";
import layout from "./beacon-layout.json";
import type { AnalyticsEngineDataPoint } from "./env";

/** 1 点に入れる値(beacon の項目と利用者のハッシュ)。 */
export type BeaconRecord = BeaconEvent & { visitor: string };

/** Analytics Engine の 1 点の上限(S1 ⑥)。 */
export const AE_MAX_BLOBS = 20;
export const AE_MAX_DOUBLES = 20;

/** 本文(JSON の文字列)を検証する。誤りなら null。 */
export function parseBeacon(text: string): BeaconEvent | null {
  let raw: unknown;
  try {
    raw = JSON.parse(text);
  } catch {
    return null;
  }
  const r = beaconSchema.safeParse(raw);
  return r.success ? r.data : null;
}

function blobOf(v: unknown): string {
  if (v == null) return layout.nullBlob;
  if (Array.isArray(v)) return [...v].map(String).sort().join(",");
  return String(v);
}

function doubleOf(v: unknown): number {
  if (v == null) return layout.nullDouble;
  if (typeof v === "boolean") return v ? 1 : 0;
  if (typeof v === "number") return v;
  throw new Error(`beacon: double に数でない値 ${String(v)}`);
}

/** beacon-layout.json の順に 1 点へ並べる。 */
export function toDataPoint(rec: BeaconRecord): AnalyticsEngineDataPoint {
  const r = rec as unknown as Record<string, unknown>;
  return {
    blobs: layout.blobs.map((k) => blobOf(r[k])),
    doubles: layout.doubles.map((k) => doubleOf(r[k])),
    indexes: [blobOf(r[layout.index])],
  };
}
