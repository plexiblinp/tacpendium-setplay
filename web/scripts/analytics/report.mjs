// 実行計測の集計(S6・HANDOVER §12-4 の開発者決定: 集計スクリプト)。Analytics Engine の SQL API に問い合わせ、Markdown の表を出す。
//
// 使い方(手元): CLOUDFLARE_ACCOUNT_ID=… CLOUDFLARE_API_TOKEN=… node scripts/analytics/report.mjs [日数(既定 7)]
//         手元でだけ回す(S9。public の Actions のログ・summary は誰でも読めるため、Actions の集計は削除した)。
// トークンは「Account Analytics: Read」だけの読み取り専用にする(docs/telemetry.md)。トークンは表示しない。
//
// 列の対応(blobN・doubleN)は書く側と同じ edge/beacon-layout.json から引く。
// 重み: Analytics Engine 自身の抽出(_sample_interval)と、画面の抽出率(sampleRate)の両方で割り戻す。
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const LAYOUT = JSON.parse(fs.readFileSync(path.join(path.dirname(fileURLToPath(import.meta.url)), "../../edge/beacon-layout.json"), "utf8"));

export const DEFAULT_DATASET = "setplay_runs";
export const DEFAULT_DAYS = 7;
/** 集計できる日数の上限(Analytics Engine の保存は 3 か月。S1 ⑥)。 */
export const MAX_DAYS = 92;
/** 「重い KA」の境目(S5 後の開発者決定 1: KA 80 以上の割合と待ち時間で D を決める)。 */
export const HEAVY_KA = 80;
const API = "https://api.cloudflare.com/client/v4/accounts";

/** beacon の項目名 → SQL の列名。 */
export function col(name) {
  const b = LAYOUT.blobs.indexOf(name);
  if (b >= 0) return `blob${b + 1}`;
  const d = LAYOUT.doubles.indexOf(name);
  if (d >= 0) return `double${d + 1}`;
  throw new Error(`layout に無い項目: ${name}`);
}

/** 1 行の重み(推定の実行回数)。 */
const W = () => `_sample_interval / ${col("sampleRate")}`;
/** 分位点の重み(quantileExactWeighted は整数の重みを取るので丸める。件数と同じく抽出率でも割り戻す。レビュー低-5)。 */
const WI = () => `toUInt32(round(${W()}))`;

/**
 * 集計の問い合わせの一覧。dataset は英数字と _ だけ(SQL に埋め込むため検査する)。
 * 条件つきの和は sumIf で書く。if(条件, 重み, 0) は本物の SQL API で HTTP 422 になる(if の 2・3 番目の型が Double と Integer で揃わない。2026-10-30)。
 */
export function buildQueries(dataset, days) {
  if (!/^[A-Za-z0-9_]+$/.test(dataset)) throw new Error(`dataset の名前が不正: ${dataset}`);
  if (!Number.isInteger(days) || days < 1 || days > MAX_DAYS) throw new Error(`日数は 1〜${MAX_DAYS}: ${days}`);
  const from = `${dataset} WHERE timestamp > NOW() - INTERVAL '${days}' DAY AND ${col("v")} = 1`;
  const fresh = `${col("outcome")} = 'result' AND ${col("cacheHit")} = 0`;
  const ms = col("ms");
  return [
    {
      title: "日ごとの実行数と推定利用者数(利用者は日替わりのハッシュの種類数。日をまたいでは数えられない。抽出率 1 未満の日は過小。ハッシュを作れなかった実行〔空文字・unavailable〕もそれぞれ 1 人として数える)",
      sql: `SELECT toStartOfDay(timestamp) AS day, round(SUM(${W()})) AS runs, count(DISTINCT ${col("visitor")}) AS visitors FROM ${from} GROUP BY day ORDER BY day`,
    },
    {
      title: `KA ${HEAVY_KA} 以上の実行の割合`,
      sql: `SELECT round(sumIf(${W()}, ${col("ka")} >= ${HEAVY_KA})) AS heavyRuns, round(SUM(${W()})) AS runs, round(100 * sumIf(${W()}, ${col("ka")} >= ${HEAVY_KA}) / SUM(${W()}), 1) AS heavyPercent FROM ${from}`,
    },
    {
      title: "KA の帯(10 ごと)の実行数",
      sql: `SELECT intDiv(${col("ka")}, 10) * 10 AS kaBand, round(SUM(${W()})) AS runs FROM ${from} GROUP BY kaBand ORDER BY kaBand`,
    },
    {
      title: "KA の帯の待ち時間(ms。計算した実行だけ。キャッシュ・誤り・中断を除く)",
      sql: `SELECT intDiv(${col("ka")}, 10) * 10 AS kaBand, round(quantileExactWeighted(0.5)(${ms}, ${WI()})) AS p50, round(quantileExactWeighted(0.95)(${ms}, ${WI()})) AS p95, round(max(${ms})) AS maxMs FROM ${from} AND ${fresh} GROUP BY kaBand ORDER BY kaBand`,
    },
    {
      title: "キャラ別の実行数",
      sql: `SELECT ${col("char")} AS char, round(SUM(${W()})) AS runs FROM ${from} GROUP BY char ORDER BY runs DESC`,
    },
    {
      title: "モード別",
      sql: `SELECT ${col("mode")} AS mode, round(SUM(${W()})) AS runs FROM ${from} GROUP BY mode ORDER BY runs DESC`,
    },
    {
      title: "言語・表記別",
      sql: `SELECT ${col("lang")} AS lang, ${col("notation")} AS notation, round(SUM(${W()})) AS runs FROM ${from} GROUP BY lang, notation ORDER BY runs DESC`,
    },
    {
      title: "結果(result / error / canceled)と誤りの code",
      sql: `SELECT ${col("outcome")} AS outcome, ${col("errorCode")} AS errorCode, round(SUM(${W()})) AS runs FROM ${from} GROUP BY outcome, errorCode ORDER BY runs DESC`,
    },
    {
      title: "結果が 0 件だった実行の割合",
      sql: `SELECT round(100 * sumIf(${W()}, ${col("resultCount")} = 0) / SUM(${W()}), 1) AS zeroPercent, round(SUM(${W()})) AS resultRuns FROM ${from} AND ${col("outcome")} = 'result'`,
    },
    {
      title: "票の読み込み(ok / none / unavailable)",
      sql: `SELECT ${col("votesStatus")} AS votesStatus, round(SUM(${W()})) AS runs FROM ${from} GROUP BY votesStatus ORDER BY runs DESC`,
    },
  ];
}

/** SQL API に 1 つ問い合わせる。誤りの文にトークンは入れない。 */
export async function query(fetchFn, accountId, token, sql) {
  const res = await fetchFn(`${API}/${encodeURIComponent(accountId)}/analytics_engine/sql`, {
    method: "POST",
    headers: { Authorization: `Bearer ${token}` },
    body: `${sql} FORMAT JSON`,
  });
  const text = await res.text();
  if (!res.ok) throw new Error(`SQL API: HTTP ${res.status}: ${text.slice(0, 300)}`);
  const body = JSON.parse(text);
  if (!Array.isArray(body.data)) throw new Error("SQL API: data が無い");
  return body.data;
}

/** 行の配列を Markdown の表にする。0 行は「該当なし」。 */
export function toMarkdown(title, rows) {
  if (rows.length === 0) return `### ${title}\n\n該当なし(0 行)\n`;
  const keys = Object.keys(rows[0]);
  const esc = (v) => String(v ?? "").replaceAll("|", "\\|");
  const lines = [`| ${keys.join(" | ")} |`, `|${keys.map(() => "---").join("|")}|`, ...rows.map((r) => `| ${keys.map((k) => esc(r[k])).join(" | ")} |`)];
  return `### ${title}\n\n${lines.join("\n")}\n`;
}

/** 失敗した表の本文。誤りの文は query が先頭 300 字に切ったもの(トークンは入らない)。 */
export function failureMarkdown(title, message) {
  return `### ${title}\n\n失敗: ${message}\n`;
}

/** すべての表を問い合わせる。1 つが失敗しても残りの表は出し、失敗した数を返す。 */
export async function report(fetchFn, accountId, token, dataset, days) {
  const parts = [`## 実行計測の集計(直近 ${days} 日・dataset ${dataset})\n`];
  let failures = 0;
  for (const q of buildQueries(dataset, days)) {
    try {
      parts.push(toMarkdown(q.title, await query(fetchFn, accountId, token, q.sql)));
    } catch (e) {
      failures++;
      parts.push(failureMarkdown(q.title, e instanceof Error ? e.message : String(e)));
    }
  }
  return { markdown: parts.join("\n"), failures };
}

/** コマンドの本体。終了コード(0 成功・1 失敗した表がある・2 使い方の誤り)を返す。 */
export async function run(env, argv, fetchFn, out, err) {
  const accountId = env.CLOUDFLARE_ACCOUNT_ID ?? "";
  const token = env.CLOUDFLARE_API_TOKEN ?? "";
  const dataset = env.ANALYTICS_DATASET || DEFAULT_DATASET;
  const days = Number(argv[0] ?? DEFAULT_DAYS);
  if (accountId === "" || token === "") {
    err("CLOUDFLARE_ACCOUNT_ID と CLOUDFLARE_API_TOKEN が要る(docs/telemetry.md)");
    return 2;
  }
  try {
    const { markdown, failures } = await report(fetchFn, accountId, token, dataset, days);
    out(markdown);
    if (failures > 0) {
      err(`失敗した表: ${failures} 件(表の「失敗:」を見る)`);
      return 1;
    }
    return 0;
  } catch (e) {
    err(e instanceof Error ? e.message : String(e));
    return 1;
  }
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  process.exitCode = await run(process.env, process.argv.slice(2), fetch, console.log, console.error);
}
