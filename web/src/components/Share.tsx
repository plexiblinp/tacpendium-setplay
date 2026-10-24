// この条件を共有(開発者決定 2026-10-20)。端末の共有メニュー(Web Share API)があればそれを開き、
// 無ければ「URL をコピー」と「X に投稿」(新しいタブで投稿画面を開くリンク)を出す。
// ★外部の JS・SNS のウィジェットは使わず、押した回数も計測しない(CSP・プライバシーポリシーを変えないため)。
import { useEffect, useId, useState } from "react";
import { useTranslation } from "react-i18next";

import { APP_NAME } from "../lib/constants";
import { xIntentUrl } from "../lib/share";

interface Props {
  /** 共有する URL(条件のクエリ込み)。 */
  url: string;
  /** 共有の文(選んでいる言語。アプリ名込み)。 */
  text: string;
}

/** コピー・共有の結果の表示を元に戻すまでの時間(ms)。 */
const FEEDBACK_MS = 2000;

type Feedback = "idle" | "copied" | "copyFailed" | "shareFailed";

export function Share({ url, text }: Props) {
  const { t } = useTranslation();
  const labelId = useId();
  const [feedback, setFeedback] = useState<Feedback>("idle");
  useEffect(() => {
    if (feedback === "idle") return;
    const id = setTimeout(() => setFeedback("idle"), FEEDBACK_MS);
    return () => clearTimeout(id);
  }, [feedback]);

  const canShare = typeof navigator.share === "function";

  const onShare = () => {
    // title はアプリ名(text と同じ文を渡すと、共有先によって 2 回入る。レビュー低-1)。
    navigator.share({ title: APP_NAME, text, url }).then(
      () => undefined,
      // 利用者が共有メニューを閉じた(AbortError)ときは何も出さない。
      (e: unknown) => {
        if (!(e instanceof DOMException && e.name === "AbortError")) setFeedback("shareFailed");
      },
    );
  };

  const onCopy = () => {
    // clipboard が無い(安全でない接続・権限の拒否)ときも例外で落とさず、失敗を出す。
    const write = navigator.clipboard?.writeText(url) ?? Promise.reject(new Error("clipboard unavailable"));
    write.then(
      () => setFeedback("copied"),
      () => setFeedback("copyFailed"),
    );
  };

  return (
    <div className="share inline" role="group" aria-labelledby={labelId} data-testid="share" data-share-url={url}>
      {canShare ? (
        <button type="button" className="secondary" id={labelId} onClick={onShare} data-testid="share-native">
          {t("share.button")}
        </button>
      ) : (
        <>
          {/* 読み上げの名前にコロンを入れない(再レビュー低-8)。 */}
          <span className="muted small">
            <span id={labelId}>{t("share.button")}</span>
            <span aria-hidden="true">:</span>
          </span>
          <button type="button" className="secondary" onClick={onCopy} data-testid="share-copy">
            {feedback === "copied" ? t("share.copied") : feedback === "copyFailed" ? t("share.copyFailed") : t("share.copyUrl")}
          </button>
          <a
            className="share-x"
            href={xIntentUrl(text, url)}
            target="_blank"
            rel="noopener noreferrer"
            aria-label={t("share.postXAria")}
            data-testid="share-x"
          >
            {t("share.postX")}
          </a>
        </>
      )}
      {feedback === "shareFailed" && (
        <span className="error small" role="status" data-testid="share-failed">
          {t("share.shareFailed")}
        </span>
      )}
    </div>
  );
}
