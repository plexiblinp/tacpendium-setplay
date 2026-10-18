// 画面の言語の一覧(HANDOVER D5・D15)。i18next を読み込まない葉のモジュール(Cloudflare Worker の検証も使う)。
export const LANGS = ["ja", "en", "zh-Hans", "zh-Hant", "ko"] as const;
export type Lang = (typeof LANGS)[number];
