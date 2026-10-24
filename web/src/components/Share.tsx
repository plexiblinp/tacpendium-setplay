// 共有の操作(開発者決定 2026-10-20)。「この条件を共有」(結果の下)と「このサイトを共有」(フッター)で使う。
// 端末の共有メニュー(Web Share API)があればそれを開き、無ければ「URL をコピー」と「X に投稿」(新しいタブで投稿画面を開くリンク)を出す。
// ★URL のコピーは URL だけ(アドレス欄にも貼れる。SNS・チャットでは OGP のカードが出る。開発者への提案 2026-10-24)。
// ★外部の JS・SNS のウィジェットは使わず、押した回数も計測しない(CSP・プライバシーポリシーを変えないため)。
import { useEffect, useId, useState } from "react";
import { useTranslation } from "react-i18next";

import { APP_NAME } from "../lib/constants";
import { xIntentUrl } from "../lib/share";

interface Props {
  /** 共有する URL。 */
  url: string;
  /** 共有の文(選んでいる言語。アプリ名込み)。 */
  text: string;
  /** 操作の名前(「この条件を共有」等)。 */
  label: string;
  /** data-testid の頭(share・app-share)。 */
  testId: string;
}

/** コピー・共有の結果の表示を元に戻すまでの時間(ms)。 */
const FEEDBACK_MS = 2000;

type Feedback = "idle" | "copied" | "copyFailed" | "shareFailed";

export function Share({ url, text, label, testId }: Props) {
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
    <div className="share inline" role="group" aria-labelledby={labelId} data-testid={testId} data-share-url={url} data-share-text={text}>
      {canShare ? (
        <button type="button" className="primary" id={labelId} onClick={onShare} data-testid={`${testId}-native`}>
          {label}
        </button>
      ) : (
        <>
          {/* 読み上げの名前にコロンを入れない(再レビュー低-8)。 */}
          <span className="muted small">
            <span id={labelId}>{label}</span>
            <span aria-hidden="true">:</span>
          </span>
          <button type="button" className="primary" onClick={onCopy} data-testid={`${testId}-copy`}>
            {feedback === "copied" ? t("share.copied") : feedback === "copyFailed" ? t("share.copyFailed") : t("share.copyUrl")}
          </button>
          <a
            className="share-x button-like"
            href={xIntentUrl(text, url)}
            target="_blank"
            rel="noopener noreferrer"
            aria-label={t("share.postXAria")}
            data-testid={`${testId}-x`}
          >
            {t("share.postX")}
          </a>
        </>
      )}
      {feedback === "shareFailed" && (
        <span className="error small" role="status" data-testid={`${testId}-failed`}>
          {t("share.shareFailed")}
        </span>
      )}
    </div>
  );
}
