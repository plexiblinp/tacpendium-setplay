// 共有(この条件を共有。開発者決定 2026-10-20)。共有する URL・文・X の投稿画面の URL を組む純粋関数。
// ★外部の JS・SNS のウィジェットは使わない(CSP・プライバシーポリシーを変えないため)。X へは利用者が押したリンクで移るだけ。
import { conditionsToSearch, type Conditions } from "./conditions";
import { APP_NAME } from "./constants";
import type { Lang } from "./langs";

/** X の投稿画面(intent)。text と url をクエリで渡す。 */
export const X_INTENT_URL = "https://x.com/intent/post";

/** キャラの表示名(ja は日本語名、ほかの言語は英語名。キャラ選択と同じ規則)。 */
export function characterDisplayName(name: { ja: string; en: string }, lang: Lang): string {
  return lang === "ja" ? name.ja : name.en;
}

/**
 * 共有する URL。ページの URL(href)の origin と path に、結果の条件のクエリを付ける。
 * 画面が replaceState で書く URL と同じ形(条件は conditionsToSearch)。ハッシュ(#privacy 等)は付けない。
 */
export function shareUrl(href: string, c: Conditions): string {
  const u = new URL(href);
  return `${u.origin}${u.pathname}?${conditionsToSearch(c)}`;
}

/** 共有の文の「モード」の i18n キー。安全飛びの検索では安全飛びの種類(例: 真安全飛び)、それ以外はモードの名前。 */
export function shareModeKey(c: Pick<Conditions, "mode" | "safeJump">): string {
  return c.safeJump != null ? `setplay.safeJumpKind.${c.safeJump}` : `share.mode.${c.mode}`;
}

/** 共有の文とアプリ名の間の区切り。 */
export const SHARE_TEXT_SEPARATOR = " — ";

/** 共有の文(「リュウ KA 43 の重ね — Tacpendium-Setplay」)。subject は選んでいる言語で訳した前半。 */
export function shareText(subject: string): string {
  return `${subject}${SHARE_TEXT_SEPARATOR}${APP_NAME}`;
}

/** サイトのトップの URL(アプリの紹介の共有。ページの origin と path。クエリ・ハッシュは付けない)。 */
export function siteUrl(href: string): string {
  const u = new URL(href);
  return `${u.origin}${u.pathname}`;
}

/** X の投稿画面の URL(新しいタブで開く)。 */
export function xIntentUrl(text: string, url: string): string {
  return `${X_INTENT_URL}?${new URLSearchParams({ text, url }).toString()}`;
}
