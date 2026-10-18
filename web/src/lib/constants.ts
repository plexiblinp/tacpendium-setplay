// 画面とエンジンの境目の定数。値の出どころはコメントのとおり(マジックナンバーを散らさない。AGENTS §4)。
// ★edge/(Worker)も読むので、ビルドで変わる値(import.meta.env)はここに置かない(画面だけが flavor.ts を読む)。
import type { Flavor } from "./flavor";

/** KA(ダウン後の有利フレーム)の下限。 */
export const KA_MIN = 0;
/** サイトの KA の上限(HANDOVER D10。engine.SiteMaxKnockdownAdvantage と同じ)。計測・票の受け口(edge/)も常にこの値で検査する。 */
export const KA_MAX_SITE = 100;
/**
 * ローカル版の KA の上限。アプリでは数で切らない(D10「KA 無制限」)ので、エンジンが受ける最大(engine.KnockdownFromFloat の
 * int32 の上限)にする。重い条件は「時間がかかる」の注意と中断で扱う(S8)。
 */
export const KA_MAX_LOCAL = 2_147_483_647;

/** ビルドの種類ごとの KA の上限。 */
export function kaMaxFor(flavor: Flavor): number {
  return flavor === "local" ? KA_MAX_LOCAL : KA_MAX_SITE;
}

/** エンジンに渡す limit。本家の maxLimit(3000)。limit は「並べ替えの後で先頭を切る」だけなので、
 * 上限で 1 回数えて画面で PAGE_SIZE 件ずつ出す(「さらに表示」で計算し直さない。docs/frontend.md)。 */
export const ENGINE_LIMIT = 3000;

/** 「さらに表示」1 回で増やす件数。本家の defaultLimit / SETPLAY_LIMIT_INCREMENT(200)と同じ。 */
export const PAGE_SIZE = 200;

/** 汚連携(gap)の G の範囲。本家と同じ(1〜13)。 */
export const GAP_G_MIN = 1;
export const GAP_G_MAX = 13;

/** 重ねる(meaty)の N の下限。 */
export const N_MIN_LOWER = 1;

/** 有利調整(filler)の「調整後の有利」の既定値と下限。本家の FILLER_REMAIN_DEFAULT / MIN と同じ。 */
export const FILLER_REMAIN_DEFAULT = 4;
export const FILLER_REMAIN_MIN = 0;

/** KA がこの値以上なら、押す前に「時間がかかることがあります」を出す(本家の SETPLAY_SLOW_KA_THRESHOLD)。 */
export const SLOW_KA_THRESHOLD = 80;

/** 結果のメモリキャッシュに残す条件の数(同一条件の連打・行き来を即表示する。HANDOVER §7.2)。 */
export const RESULT_CACHE_ENTRIES = 30;

/** 結果が無いときの理由コード(engine の reason)。 */
export const REASON_KNOCKDOWN_NEGATIVE = "knockdown_advantage_negative";
export const REASON_REMAIN_NOT_BELOW_KNOCKDOWN = "remain_not_below_knockdown";

/** アプリの名前(訳さない。副題は locales の app.title)。S7 で開発者が決めた。 */
export const APP_NAME = "Tacpendium-Setplay";

/** ソースと Issue の置き場(AGPL のソースへのリンク・翻訳の改善提案。HANDOVER §6・§8)。 */
export const SOURCE_URL = "https://github.com/plexiblinp/tacpendium-setplay";
export const ISSUES_URL = `${SOURCE_URL}/issues`;
/** ローカル版の配布ページ(KA が 100 を超えるときの案内。HANDOVER D10)。 */
export const RELEASES_URL = `${SOURCE_URL}/releases`;
/**
 * 意見・不具合・翻訳の改善を送る Google フォーム(ログイン不要・日英併記・URL は書けない。S7 の開発者決定)。
 * 本家用の Google アカウントで作ったもの。空にするとフォームへのリンクを出さず、Issues だけを案内する。
 */
export const FEEDBACK_FORM_URL: string = "https://docs.google.com/forms/d/e/1FAIpQLSfsII4FX9522zD6ClH8oQ1mkNhgWd2quQvMzla3RxPeONykcQ/viewform";

/** 意見を送る先(フォームがあればフォーム、無ければ Issues)。 */
export function feedbackUrl(formUrl: string): string {
  return formUrl !== "" ? formUrl : ISSUES_URL;
}
export const FEEDBACK_URL = feedbackUrl(FEEDBACK_FORM_URL);
/** データの取込元とライセンス(NOTICE §2 のクレジット)。 */
export const DATA_SOURCE_URL = "https://github.com/plexiblinp/tacpendium";
export const DATA_LICENSE_URL = "https://creativecommons.org/licenses/by-sa/4.0/";

/** 配信の場所(Vite の base からの相対)。 */
export const MANIFEST_PATH = "data/manifest.json";
export const DATA_DIR = "data/";
export const WASM_PATH = "wasm/setplay.wasm";
export const WASM_EXEC_PATH = "wasm/wasm_exec.js";

/**
 * 計算の後、WASM の線形メモリがこれを超えていたら Worker を捨ててメモリを返す(次の計算で作り直す。起動は 0.1〜0.2 秒)。
 * Go の WASM は広げたメモリを返さないため、KA 100 の重い条件(実測で最大 833 MiB)の後に大きなメモリを持ち続けない。
 * 256 MiB は実測で KA 60 以下(最大 97 MiB)では捨てず、KA 80 以上の重い条件の後で捨てる値(S5 報告・開発者決定)。
 */
export const WASM_MEMORY_RELEASE_BYTES = 256 * 1024 * 1024;
