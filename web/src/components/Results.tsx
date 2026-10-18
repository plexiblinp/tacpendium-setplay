// 結果一覧(件数・さらに表示・打ち切り・理由・行ごとのコピー(D16))。
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";

import { hasAppliedFilters, type Conditions } from "../lib/conditions";
import { PAGE_SIZE, REASON_KNOCKDOWN_NEGATIVE, REASON_REMAIN_NOT_BELOW_KNOCKDOWN } from "../lib/constants";
import type { NotationKind } from "../lib/data";
import type { SuggestOutcome } from "../lib/engineClient";
import type { Item } from "../lib/engineTypes";
import { copyText, itemKey, recipeText, type MovesByCode } from "../lib/notation";
import type { LoadedVotes } from "../lib/votes";
import type { VoteValue } from "../shared/api";

export type RunState =
  | { status: "idle" }
  | { status: "running"; conditions: Conditions }
  | { status: "done"; conditions: Conditions; outcome: Extract<SuggestOutcome, { kind: "result" }>; fromCache: boolean; votes: LoadedVotes }
  | { status: "error"; conditions: Conditions; code: string }
  | { status: "canceled" };

/** 自分の票(D17)。good・bad は送った後に受け口が返した数(まだ送っていなければ null = 計算に使った票の数を出す)。 */
export interface MyVote {
  good: number | null;
  bad: number | null;
  value: VoteValue;
  state: "idle" | "sending" | "failed";
}

interface Props {
  run: RunState;
  onCancel: () => void;
  movesByCode: MovesByCode;
  notation: NotationKind;
  /** キャラ|KA|鍵 → 自分の票。 */
  myVotes: Record<string, MyVote>;
  onVote: (c: Conditions, recipe: string, value: VoteValue) => void;
}

/** コピーの表示を元に戻すまでの時間(ms)。 */
const COPY_FEEDBACK_MS = 2000;

export function Results({ run, onCancel, movesByCode, notation, myVotes, onVote }: Props) {
  const { t } = useTranslation();
  const [shown, setShown] = useState(PAGE_SIZE);
  // 隠したレシピ(鍵)。その回の結果の間だけ。もう一度検索すると元に戻る(開発者の要望 2026-10-17)。
  const [hidden, setHidden] = useState<ReadonlySet<string>>(() => new Set());
  // 新しい計算(キャッシュからの表示を含む)では先頭の PAGE_SIZE 件に戻し、隠したものも戻す。
  useEffect(() => {
    setShown(PAGE_SIZE);
    setHidden(new Set());
  }, [run]);

  if (run.status === "idle") {
    return (
      <section className="results" aria-live="polite">
        <p className="muted">{t("setplay.notSearched")}</p>
      </section>
    );
  }
  if (run.status === "canceled") {
    return (
      <section className="results" aria-live="polite">
        <p className="muted" data-testid="canceled">
          {t("setplay.canceled")}
        </p>
      </section>
    );
  }
  if (run.status === "running") {
    return (
      <section className="results" aria-live="polite" aria-busy="true">
        <div className="inline">
          <p className="muted" data-testid="loading">
            {t("setplay.loading")}
          </p>
          <button type="button" className="secondary" onClick={onCancel} data-testid="cancel">
            {t("setplay.cancel")}
          </button>
        </div>
      </section>
    );
  }
  if (run.status === "error") {
    return (
      <section className="results" aria-live="polite">
        <p className="error" role="alert" data-testid="run-error">
          {t("setplay.error", { code: run.code })}
        </p>
      </section>
    );
  }

  const { conditions: c, outcome, fromCache, votes } = run;
  const { items, totalFound, truncated, reason } = outcome.result;
  const filtered = hasAppliedFilters(c);
  // 隠したものを除いてから PAGE_SIZE 件ずつ出す(隠すと次の行が繰り上がる)。
  const remaining = hidden.size === 0 ? items : items.filter((it) => !hidden.has(it.recipeKey));
  const visible = remaining.slice(0, shown);

  const frameLabel = (it: Item): string => {
    if (it.mode === "filler") {
      return c.safeJump != null
        ? t("setplay.safeJumpRowLabel", { kind: t(`setplay.safeJumpKind.${c.safeJump}`), remain: it.remain })
        : t("setplay.remainAfterFiller", { remain: it.remain });
    }
    return it.mode === "gap" ? t("setplay.gapBeforeWakeup", { g: it.g }) : t("setplay.hitOnActive", { n: it.n });
  };

  return (
    <section className="results" aria-live="polite">
      {reason === REASON_KNOCKDOWN_NEGATIVE ? (
        <p className="muted" data-testid="negative-knockdown">
          {t("setplay.negativeKnockdown")}
        </p>
      ) : items.length === 0 ? (
        <p className="muted" data-testid="empty">
          {reason === REASON_REMAIN_NOT_BELOW_KNOCKDOWN
            ? t("setplay.fillerRemainNotBelowKnockdown")
            : filtered
              ? t("setplay.filteredEmpty")
              : t("setplay.empty")}
        </p>
      ) : (
        <>
          <p className="muted small" data-testid="count">
            {t("setplay.countLabel", { total: totalFound, shown: visible.length })}
            {" · "}
            <span data-testid="timing" data-timings={fromCache ? undefined : JSON.stringify({ wall: outcome.wallMs, ...outcome.timings })}>
              {fromCache ? t("setplay.cached") : t("setplay.timing", { ms: Math.round(outcome.wallMs) })}
            </span>
          </p>
          <p className="muted small" data-testid="votes-note" data-votes-status={votes.status}>
            {votes.status === "unavailable" ? t("setplay.votesUnavailable") : t("setplay.votesOrderNote")}
          </p>
          {hidden.size > 0 && (
            <p className="muted small inline" data-testid="hidden-note">
              {t("setplay.hiddenCount", { count: hidden.size })}{" "}
              <button type="button" className="link" onClick={() => setHidden(new Set())} data-testid="unhide-all">
                {t("setplay.unhideAll")}
              </button>
            </p>
          )}
          <ul className="rows">
            {visible.map((it) => {
              const mine = myVotes[`${c.character}|${c.ka}|${it.recipeKey}`];
              const counted = votes.counts[it.recipeKey];
              return (
                <Row
                  key={itemKey(it, c.safeJump)}
                  canonical={canonicalLine(it)}
                  recipeKey={it.recipeKey}
                  recipe={recipeText(it.steps, movesByCode, notation)}
                  frameLabel={frameLabel(it)}
                  good={mine?.good ?? counted?.[0] ?? 0}
                  bad={mine?.bad ?? counted?.[1] ?? 0}
                  mine={mine}
                  onVote={(v) => onVote(c, it.recipeKey, v)}
                  onHide={() => setHidden((h) => new Set(h).add(it.recipeKey))}
                />
              );
            })}
          </ul>
          {/* 計算は上限 3000 件で 1 回だけ行い、ここでは表示の件数だけを増やす(docs/frontend.md)。 */}
          {visible.length < remaining.length && (
            <button type="button" className="secondary" onClick={() => setShown((n) => n + PAGE_SIZE)} data-testid="show-more">
              {t("setplay.showMore")}
            </button>
          )}
        </>
      )}
      {/* 安全上限に達した(数え切れていない)ときだけ。limit で切っただけの状態とは区別する。 */}
      {truncated && (
        <p className="warn" data-testid="truncated">
          {t("setplay.truncated")}
        </p>
      )}
    </section>
  );
}

/**
 * 行の正準形(engine/conformance_test.go の canonicalItems と同じ書式の 1 行)。E2E が画面に出た結果を
 * 本家の正解(sha256)と突き合わせるために data 属性に置く(Go → WASM → Worker → 画面の通し。AGENTS §8-5)。
 */
export function canonicalLine(it: Item): string {
  const steps = it.steps.map((st) => `${st.code}:${st.role}:${st.counted}`).join(",");
  return `${it.mode}|${it.s}|${it.n}|${it.landing}|${it.targetActive}|${it.g}|${it.remain}|${steps}`;
}

interface RowProps {
  recipe: string;
  frameLabel: string;
  canonical: string;
  recipeKey: string;
  good: number;
  bad: number;
  mine: MyVote | undefined;
  onVote: (v: VoteValue) => void;
  onHide: () => void;
}

/** グッド/バッドのアイコン(色は CSS の currentColor。絵文字は色を変えられないため SVG で描く)。 */
function ThumbIcon({ down }: { down?: boolean }) {
  return (
    <svg className={down ? "thumb down" : "thumb"} viewBox="0 0 24 24" width="16" height="16" aria-hidden="true" focusable="false">
      <path d="M2 10h4v11H2zM8 21h9.3a2 2 0 0 0 2-1.6l1.6-7.4A2 2 0 0 0 19 9.6h-5.4l.9-4.3a1.6 1.6 0 0 0-2.8-1.4L8 9z" fill="currentColor" />
    </svg>
  );
}

function Row({ recipe, frameLabel, canonical, recipeKey, good, bad, mine, onVote, onHide }: RowProps) {
  const { t } = useTranslation();
  const [copy, setCopy] = useState<"idle" | "copied" | "failed">("idle");
  useEffect(() => {
    if (copy === "idle") return;
    const id = setTimeout(() => setCopy("idle"), COPY_FEEDBACK_MS);
    return () => clearTimeout(id);
  }, [copy]);

  const onCopy = () => {
    const text = copyText(recipe, frameLabel);
    // clipboard が無い(安全でない接続・権限の拒否)ときも例外で落とさず、失敗を出す。
    const write = navigator.clipboard?.writeText(text) ?? Promise.reject(new Error("clipboard unavailable"));
    write.then(
      () => setCopy("copied"),
      () => setCopy("failed"),
    );
  };

  return (
    <li className="row-item" data-testid="result-row" data-canonical={canonical} data-recipe-key={recipeKey}>
      <span className="recipe" data-testid="recipe">
        {recipe}
      </span>
      <span className="frame" data-testid="frame-label">
        {frameLabel}
      </span>
      <button
        type="button"
        className="copy"
        onClick={onCopy}
        aria-label={t("setplay.copyAria", { recipe })}
        data-testid="copy"
      >
        {copy === "copied" ? t("setplay.copied") : copy === "failed" ? t("setplay.copyFailed") : t("setplay.copy")}
      </button>
      {/* グッド/バッド(D17)。同じ票をもう一度押すと取り消す。 */}
      <span className="votes">
        <button
          type="button"
          className="vote good"
          aria-pressed={mine?.value === 1}
          aria-label={t("setplay.voteGoodAria", { recipe })}
          disabled={mine?.state === "sending"}
          onClick={() => onVote(1)}
          data-testid="vote-good"
        >
          <ThumbIcon /> <span data-testid="good-count">{good}</span>
        </button>
        <button
          type="button"
          className="vote bad"
          aria-pressed={mine?.value === -1}
          aria-label={t("setplay.voteBadAria", { recipe })}
          disabled={mine?.state === "sending"}
          onClick={() => onVote(-1)}
          data-testid="vote-bad"
        >
          <ThumbIcon down /> <span data-testid="bad-count">{bad}</span>
        </button>
        <button type="button" className="hide" onClick={onHide} aria-label={t("setplay.hideAria", { recipe })} data-testid="hide">
          {t("setplay.hide")}
        </button>
        {mine?.state === "failed" && (
          <span className="error small" role="status" data-testid="vote-failed">
            {t("setplay.voteFailed")}
          </span>
        )}
      </span>
    </li>
  );
}
