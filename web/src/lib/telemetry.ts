// 実行計測(HANDOVER §7.1・D3): 「提案を出す」1 回ごとに beacon を 1 つ送る。中身は shared/api.ts の beaconSchema。
// ★cookie・ストレージは使わない(§7.3)。抽出率は静的な config.json(混雑時に下げる。§7.1)。
import { z } from "zod";

import {
  API_BEACON_PATH,
  BEACON_SCHEMA_VERSION,
  beaconSchema,
  type BeaconEvent,
  type Outcome,
  type VotesStatus,
} from "../shared/api";
import type { Conditions } from "./conditions";
import type { NotationKind } from "./data";
import type { Lang } from "./langs";
import type { FetchFn } from "./loader";

/** 配信の設定(public/config.json)。唯一の可変の設定ファイル(no-cache)。 */
export const CONFIG_PATH = "config.json";
/** config.json が読めないときの抽出率(全部送る。計測が止まっても困らないが、黙って止めない)。 */
export const DEFAULT_SAMPLE_RATE = 1;

const configSchema = z.object({
  schemaVersion: z.literal(1),
  beaconSampleRate: z.number().min(0).max(1),
});
export type SiteConfig = z.infer<typeof configSchema>;

export async function loadConfig(fetchFn: FetchFn, baseUrl: string): Promise<SiteConfig> {
  try {
    const res = await fetchFn(new URL(CONFIG_PATH, baseUrl).href, { cache: "no-cache" });
    if (!res.ok) throw new Error(String(res.status));
    return configSchema.parse(await res.json());
  } catch {
    return { schemaVersion: 1, beaconSampleRate: DEFAULT_SAMPLE_RATE };
  }
}

/** 1 回の実行の結果(beacon にする材料)。 */
export interface RunRecord {
  conditions: Conditions;
  notation: NotationKind;
  lang: Lang;
  outcome: Outcome;
  errorCode: string | null;
  totalFound: number | null;
  items: number | null;
  truncated: boolean | null;
  cacheHit: boolean;
  wallMs: number | null;
  computeMs: number | null;
  votesStatus: VotesStatus;
  votesApplied: number;
  dataVersion: string;
}

/** 実行の記録を beacon にする。当てはまらない項目は null(0 と区別する。AGENTS §8-7)。 */
export function buildBeacon(r: RunRecord, sampleRate: number): BeaconEvent {
  const c = r.conditions;
  return {
    v: BEACON_SCHEMA_VERSION,
    char: c.character,
    ka: c.ka,
    mode: c.mode,
    nMin: c.mode === "meaty" ? c.nMin : null,
    nMax: c.mode === "meaty" ? c.nMax : null,
    gMin: c.mode === "gap" ? c.gMin : null,
    gMax: c.mode === "gap" ? c.gMax : null,
    remain: c.mode === "filler" ? c.remain : null,
    safeJump: c.mode === "filler" ? c.safeJump : null,
    targetTypes: c.mode === "filler" || c.targetMove != null ? [] : [...c.targetTypes],
    targetMove: c.mode === "filler" ? null : c.targetMove,
    includeZeroDamage: c.includeZeroDamage,
    approachOnly: c.approachOnly,
    blockSafeOnly: c.blockSafeOnly,
    chainCancelOnly: c.chainCancelOnly,
    sort: c.sort,
    notation: r.notation,
    lang: r.lang,
    outcome: r.outcome,
    errorCode: r.errorCode,
    resultCount: r.totalFound,
    items: r.items,
    truncated: r.truncated,
    cacheHit: r.cacheHit,
    ms: r.wallMs == null ? null : Math.round(r.wallMs * 10) / 10,
    computeMs: r.computeMs == null ? null : Math.round(r.computeMs * 10) / 10,
    votesStatus: r.votesStatus,
    votesApplied: r.votesApplied,
    dataVersion: r.dataVersion,
    sampleRate,
  };
}

/** 送る口(navigator.sendBeacon。試験では差し替える)。 */
export type BeaconSender = (url: string, body: string) => boolean;

/**
 * 抽出率に当たれば beacon を送る。送った beacon を返す(送らなかったら null)。
 * 形が beaconSchema に合わないとき(画面の誤り)は送らない(受け口は同じ形で落とすので、送っても無駄)。
 */
export function sendRun(send: BeaconSender, baseUrl: string, r: RunRecord, sampleRate: number, random: () => number): BeaconEvent | null {
  if (sampleRate <= 0 || random() >= sampleRate) return null;
  const ev = buildBeacon(r, sampleRate);
  if (!beaconSchema.safeParse(ev).success) return null;
  try {
    send(new URL(API_BEACON_PATH, baseUrl).href, JSON.stringify(ev));
  } catch {
    return null;
  }
  return ev;
}
