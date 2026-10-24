// 画面の最上位: 配信データの読み込み・言語と表記・URL(共有)・計算の実行。
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";

import { cacheKey, conditionsToSearch, parseSearch, toEngineQuery, type Conditions } from "../lib/conditions";
import { APP_NAME, kaMaxFor, RESULT_CACHE_ENTRIES } from "../lib/constants";
import { resolveDataUrl, type MovesDb, type NotationKind } from "../lib/data";
import { FLAVOR, type Flavor } from "../lib/flavor";
import type { SuggestOutcome } from "../lib/engineClient";
import { formFromConditions, initialForm, type FormState } from "../lib/formState";
import { detectLang, type Lang } from "../lib/i18n";
import { loadManifest, loadMoves, type FetchFn, type LoadedManifest } from "../lib/loader";
import { LruCache } from "../lib/lru";
import { characterDisplayName, shareText, shareUrl } from "../lib/share";
import { defaultNotation, indexMoves } from "../lib/notation";
import { DEFAULT_SAMPLE_RATE, loadConfig, sendRun, type BeaconSender, type RunRecord } from "../lib/telemetry";
import { fetchVotes, postVote, scoresForEngine, type LoadedVotes } from "../lib/votes";
import type { VoteValue } from "../shared/api";
import type { SuggestRequest } from "../worker/protocol";
import { ConditionsPanel } from "./ConditionsPanel";
import { Guide } from "./Guide";
import { Footer } from "./Footer";
import { PrivacyPolicy, PRIVACY_HASH } from "./PrivacyPolicy";
import { Results, type MyVote, type RunState } from "./Results";
import { Share } from "./Share";
import { SettingsBar } from "./SettingsBar";

/** 計算の窓口(EngineClient。試験では差し替える)。 */
export interface SuggestEngine {
  suggest(req: Omit<SuggestRequest, "type" | "id">): Promise<SuggestOutcome>;
  cancel(): void;
}

export interface AppProps {
  engine: SuggestEngine;
  fetchFn: FetchFn;
  /** ページの URL(manifest の基準・共有の URL の読み取り)。 */
  location: { href: string; search: string };
  /** URL を書き換える(共有)。 */
  replaceSearch: (search: string) => void;
  /** 言語の候補(navigator.languages)。 */
  languages: readonly string[];
  onLangChange: (lang: Lang) => void;
  /** 実行計測の送り口(navigator.sendBeacon。HANDOVER §7.1)。 */
  sendBeacon: BeaconSender;
  /** 抽出の乱数(0〜1)。試験では固定する。 */
  random?: () => number;
  /** 経過時間の時計(ms)。 */
  now?: () => number;
  /** ビルドの種類。ローカル版は計測・票の通信をしない(既定はこのビルドの種類。試験では差し替える)。 */
  flavor?: Flavor;
}

/** 結果のキャッシュに置くもの(計算の結果と、そのとき使った票)。 */
interface CachedRun {
  outcome: Extract<SuggestOutcome, { kind: "result" }>;
  /** 票を使わない版(ローカル版)では null。 */
  votes: LoadedVotes | null;
  votesApplied: number;
}

/** 計算中の実行(中断の計測に使う)。 */
interface Pending {
  token: number;
  conditions: Conditions;
  started: number;
  /** まだ読んでいない・票を使わない版では null。 */
  votes: LoadedVotes | null;
  votesApplied: number;
}

type ManifestState = { status: "loading" } | { status: "error"; message: string } | { status: "ready"; data: LoadedManifest };

export function App({
  engine,
  fetchFn,
  location,
  replaceSearch,
  languages,
  onLangChange,
  sendBeacon,
  random = Math.random,
  now = () => performance.now(),
  flavor = FLAVOR,
}: AppProps) {
  const online = flavor === "site";
  const { t, i18n } = useTranslation();
  const [lang, setLang] = useState<Lang>(() => detectLang(languages));
  // 表記は、利用者が選ぶまで言語の既定に従う(HANDOVER §4.4)。
  const [notationChoice, setNotationChoice] = useState<NotationKind | null>(null);
  const notation = notationChoice ?? defaultNotation(lang);

  const [manifest, setManifest] = useState<ManifestState>({ status: "loading" });
  const [form, setForm] = useState<FormState>(initialForm);
  const [moves, setMoves] = useState<Record<string, MovesDb>>({});
  const [run, setRun] = useState<RunState>({ status: "idle" });
  const [urlRejected, setUrlRejected] = useState<string[]>([]);
  const cache = useRef(new LruCache<string, CachedRun>(RESULT_CACHE_ENTRIES));
  const autoRunDone = useRef(false);
  const [sampleRate, setSampleRate] = useState(DEFAULT_SAMPLE_RATE);
  const pending = useRef<Pending | null>(null);
  const runToken = useRef(0);
  // 自分の票(キャラ|KA|鍵 → 票と数)。メモリだけ(§7.3)。
  const [myVotes, setMyVotes] = useState<Record<string, MyVote>>({});
  const [showPrivacy, setShowPrivacy] = useState(() => online && new URL(location.href).hash === PRIVACY_HASH);

  useEffect(() => {
    if (i18n.language !== lang) void i18n.changeLanguage(lang);
    document.documentElement.lang = lang;
    document.title = `${APP_NAME} | ${t("app.title")}`;
    onLangChange(lang);
  }, [lang, i18n, t, onLangChange]);

  useEffect(() => {
    if (!online) return;
    let alive = true;
    void loadConfig(fetchFn, location.href).then((c) => alive && setSampleRate(c.beaconSampleRate));
    return () => {
      alive = false;
    };
  }, [online, fetchFn, location.href]);

  useEffect(() => {
    let alive = true;
    loadManifest(fetchFn, location.href).then(
      (data) => alive && setManifest({ status: "ready", data }),
      (e: unknown) => alive && setManifest({ status: "error", message: e instanceof Error ? e.message : String(e) }),
    );
    return () => {
      alive = false;
    };
  }, [fetchFn, location.href]);

  const characters = manifest.status === "ready" ? manifest.data.manifest.characters : [];
  const character = characters.find((c) => c.code === form.character) ?? null;
  const movesDb = character ? moves[character.code] : undefined;
  const movesByCode = useMemo(() => indexMoves(movesDb?.moves ?? []), [movesDb]);

  // 選んだキャラの moves DB(表記・特定の技の選択に使う)。
  useEffect(() => {
    if (manifest.status !== "ready" || character == null || moves[character.code] != null) return;
    let alive = true;
    loadMoves(fetchFn, manifest.data.url, character.moves).then(
      (db) => alive && setMoves((m) => ({ ...m, [character.code]: db })),
      () => undefined, // 表示用の読み込みの失敗は、計算(Worker)の誤りとして出る。
    );
    return () => {
      alive = false;
    };
  }, [manifest, character, moves, fetchFn]);

  /** 実行 1 回の計測を送る(HANDOVER §7.1)。票を使わない版(votes が null)では送らない(ローカル版は通信しない)。 */
  const report = useCallback(
    ({ votes, ...r }: Omit<RunRecord, "notation" | "lang" | "dataVersion" | "votesStatus"> & { votes: LoadedVotes | null }) => {
      if (!online || votes == null || manifest.status !== "ready") return;
      const record = { ...r, votesStatus: votes.status, notation, lang, dataVersion: manifest.data.manifest.dataVersion };
      sendRun(sendBeacon, location.href, record, sampleRate, random);
    },
    [online, manifest, sendBeacon, location.href, notation, lang, sampleRate, random],
  );

  const execute = useCallback(
    async (c: Conditions) => {
      if (manifest.status !== "ready") return;
      const ch = manifest.data.manifest.characters.find((x) => x.code === c.character);
      if (ch == null) return;
      replaceSearch(conditionsToSearch(c));
      const started = now();
      const key = cacheKey(c);
      const hit = cache.current.get(key);
      if (hit != null) {
        setRun({ status: "done", conditions: c, outcome: hit.outcome, fromCache: true, votes: hit.votes });
        report({
          conditions: c,
          outcome: "result",
          errorCode: null,
          totalFound: hit.outcome.result.totalFound,
          items: hit.outcome.result.items.length,
          truncated: hit.outcome.result.truncated,
          cacheHit: true,
          wallMs: now() - started,
          computeMs: null,
          votes: hit.votes,
          votesApplied: hit.votesApplied,
        });
        return;
      }
      const token = ++runToken.current;
      pending.current = { token, conditions: c, started, votes: null, votesApplied: 0 };
      setRun({ status: "running", conditions: c });
      // 票を先に読む(HANDOVER D17: 票は並べ替えの第一キー)。読めなければ票なし(本家の順)で計算する。
      // ローカル版は票を使わない(通信しない。並びは本家の順)。
      const votes = online ? await fetchVotes(fetchFn, location.href, c.character, c.ka) : null;
      if (runToken.current !== token) return; // 票を待つ間に中断された
      // 読み直した票の数を表示に使う(投票の時点の数を残さない。押した状態 value は残す。レビュー低-1)。
      const prefix = `${c.character}|${c.ka}|`;
      // 票を読めなかったときは、押した時点の数を残す(0/0 に見せない。再レビュー低-1)。
      if (votes != null && votes.status !== "unavailable") setMyVotes((m) => {
        const next: Record<string, MyVote> = {};
        for (const [k, v] of Object.entries(m)) next[k] = k.startsWith(prefix) ? { ...v, good: null, bad: null } : v;
        return next;
      });
      const scores = scoresForEngine(votes?.counts ?? {});
      const votesApplied = Object.keys(scores).length;
      pending.current = { token, conditions: c, started, votes, votesApplied };
      const outcome = await engine.suggest({
        character: c.character,
        movesUrl: resolveDataUrl(manifest.data.url, ch.moves),
        ka: c.ka,
        query: toEngineQuery(c),
        ...(votesApplied > 0 ? { votes: JSON.stringify(scores) } : {}),
      });
      if (outcome.kind === "canceled") return; // 中断は cancel 側が状態を変え、計測を送る。
      pending.current = null;
      const base = { conditions: c, cacheHit: false, wallMs: now() - started, votes, votesApplied };
      if (outcome.kind === "result") {
        cache.current.set(key, { outcome, votes, votesApplied });
        setRun({ status: "done", conditions: c, outcome, fromCache: false, votes });
        report({
          ...base,
          outcome: "result",
          errorCode: null,
          totalFound: outcome.result.totalFound,
          items: outcome.result.items.length,
          truncated: outcome.result.truncated,
          computeMs: outcome.timings.compute,
        });
      } else {
        setRun({ status: "error", conditions: c, code: outcome.error.code });
        report({ ...base, outcome: "error", errorCode: outcome.error.code, totalFound: null, items: null, truncated: null, computeMs: null });
      }
    },
    [manifest, engine, replaceSearch, fetchFn, location.href, now, report],
  );

  const cancel = useCallback(() => {
    engine.cancel();
    runToken.current++;
    const p = pending.current;
    pending.current = null;
    setRun({ status: "canceled" });
    if (p != null) {
      report({
        conditions: p.conditions,
        outcome: "canceled",
        errorCode: null,
        totalFound: null,
        items: null,
        truncated: null,
        cacheHit: false,
        wallMs: now() - p.started,
        computeMs: null,
        // 票を待つ間の中断は、票を読めていない(unavailable)として送る(S6 からの扱い)。ローカル版は送らない。
        votes: online ? (p.votes ?? { status: "unavailable", counts: {} }) : null,
        votesApplied: p.votesApplied,
      });
    }
  }, [online, engine, report, now]);

  /** グッド/バッド(D17)。同じ票をもう一度押すと取り消す。並びは次の検索から変わる(押した行を動かさない)。 */
  const vote = useCallback(
    async (c: Conditions, recipe: string, value: VoteValue) => {
      const k = `${c.character}|${c.ka}|${recipe}`;
      const prev = myVotes[k];
      const next: VoteValue = prev?.value === value ? 0 : value;
      setMyVotes((m) => ({ ...m, [k]: { ...(prev ?? { good: null, bad: null }), value: prev?.value ?? 0, state: "sending" } }));
      const r = await postVote(fetchFn, location.href, c.character, c.ka, recipe, next);
      setMyVotes((m) => ({
        ...m,
        [k]: r == null ? { ...(m[k] ?? { good: null, bad: null, value: 0 }), state: "failed" } : { good: r.good, bad: r.bad, value: r.value, state: "idle" },
      }));
      // 覚えた結果は古い票で並べたものなので捨てる(次の検索で票を読み直す)。
      cache.current.clear();
    },
    [myVotes, fetchFn, location.href],
  );

  // 共有された URL の条件で、データの読み込み後に 1 度だけ自動で計算する。
  useEffect(() => {
    if (manifest.status !== "ready" || autoRunDone.current) return;
    autoRunDone.current = true;
    const known = new Set(manifest.data.manifest.characters.map((c) => c.code));
    const parsed = parseSearch(location.search, known, kaMaxFor(flavor));
    setUrlRejected(parsed.rejected);
    const c = parsed.conditions;
    if (c == null) return;
    const ch = manifest.data.manifest.characters.find((x) => x.code === c.character);
    // 安全飛びは種類から値を引き直す(URL の remain は信じない)。値の無いキャラでは安全飛びを外す。
    if (c.safeJump != null) {
      const v = ch?.safeJump?.[c.safeJump];
      if (v == null) c.safeJump = null;
      else c.remain = v;
    }
    setForm(formFromConditions(c));
    void execute(c);
  }, [manifest, location.search, execute, flavor]);

  /** 条件・結果・URL を最初の状態に戻す(S5 目視の要望)。言語と表記は利用者の設定なので戻さない。 */
  const reset = () => {
    if (run.status === "running") cancel();
    setForm(initialForm());
    setRun({ status: "idle" });
    setUrlRejected([]);
    replaceSearch("");
  };

  // 共有の操作(結果があるときだけ)。ローカル版は URL(127.0.0.1)を相手が開けないので出さない(開発者決定 2026-10-20)。
  const sharedCharacter = run.status === "done" ? characters.find((x) => x.code === run.conditions.character) : undefined;
  const sharedUrl = run.status === "done" ? shareUrl(location.href, run.conditions) : "";
  const share =
    online && run.status === "done" && sharedCharacter != null ? (
      <Share
        // 条件が変わったら作り直す(前の条件の「コピーしました」を残さない。レビュー低-2)。
        key={sharedUrl}
        url={sharedUrl}
        text={shareText(t("share.text", { character: characterDisplayName(sharedCharacter.name, lang), ka: run.conditions.ka }))}
      />
    ) : undefined;

  if (manifest.status === "loading") {
    return (
      <main className="page">
        <p data-testid="data-loading">{t("app.loadingData")}</p>
      </main>
    );
  }
  if (manifest.status === "error") {
    return (
      <main className="page">
        <p className="error" role="alert" data-testid="data-error">
          {t("app.loadError")}
        </p>
      </main>
    );
  }

  return (
    <>
      <header className="site-header">
        <h1>
          {APP_NAME} <span className="subtitle">{t("app.title")}</span>
          {!online && (
            <span className="badge" data-testid="local-badge">
              {t("app.localBadge")}
            </span>
          )}
        </h1>
        <SettingsBar
          lang={lang}
          onLang={setLang}
          notation={notation}
          onNotation={setNotationChoice}
        />
      </header>
      <main className="page">
        <Guide voting={online} />
        {urlRejected.length > 0 && (
          <p className="notice" role="status" data-testid="url-rejected">
            {t("app.urlRejected", { keys: urlRejected.join(", ") })}
          </p>
        )}
        <ConditionsPanel
          form={form}
          onChange={setForm}
          characters={characters}
          lang={lang}
          moves={movesDb?.moves ?? []}
          notation={notation}
          running={run.status === "running"}
          flavor={flavor}
          onRun={(c) => void execute(c)}
          onReset={reset}
        />
        <Results
          run={run}
          onCancel={cancel}
          movesByCode={movesByCode}
          notation={notation}
          myVotes={myVotes}
          onVote={(c, recipe, value) => void vote(c, recipe, value)}
          share={share}
        />
        {showPrivacy && <PrivacyPolicy onClose={() => setShowPrivacy(false)} lang={lang} />}
      </main>
      <Footer dataVersion={manifest.data.manifest.dataVersion} online={online} onPrivacy={() => setShowPrivacy(true)} />
    </>
  );
}
