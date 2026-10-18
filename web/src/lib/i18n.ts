// 画面の言語(HANDOVER §6・D5・D15)。翻訳は UI ラベル・操作説明だけで、技名は表記(D6)で切り替える。
import i18n from "i18next";
import { initReactI18next } from "react-i18next";

import en from "../locales/en.json";
import ja from "../locales/ja.json";
import ko from "../locales/ko.json";
import zhHans from "../locales/zh-Hans.json";
import zhHant from "../locales/zh-Hant.json";

import { LANGS, type Lang } from "./langs";

export { LANGS, type Lang };

export const RESOURCES: Record<Lang, typeof ja> = { ja, en, "zh-Hans": zhHans, "zh-Hant": zhHant, ko };

/** 言語の名前(その言語自身の表記。翻訳しない)。 */
export const LANG_NAMES: Record<Lang, string> = {
  ja: "日本語",
  en: "English",
  "zh-Hans": "简体中文",
  "zh-Hant": "繁體中文",
  ko: "한국어",
};

/** 機械翻訳の注記を出す言語(HANDOVER §6)。 */
export const MACHINE_TRANSLATED: ReadonlySet<Lang> = new Set<Lang>(["zh-Hans", "zh-Hant", "ko"]);

/** 言語が決まらないときの既定。 */
export const FALLBACK_LANG: Lang = "en";

const TRADITIONAL_REGIONS = new Set(["tw", "hk", "mo"]);

/**
 * navigator.languages から言語を選ぶ。zh-CN / zh-SG は簡体、zh-TW / zh-HK / zh-MO は繁体(HANDOVER §6)。
 * zh-Hant / zh-Hans の書き方(文字体系付き)も受け、地域より文字体系を優先する。地域の無い zh は簡体。どれにも当たらなければ en。
 */
export function detectLang(languages: readonly string[]): Lang {
  for (const raw of languages) {
    const parts = raw.toLowerCase().split(/[-_]/);
    const base = parts[0];
    if (base === "ja") return "ja";
    if (base === "en") return "en";
    if (base === "ko") return "ko";
    if (base === "zh") {
      // 文字体系(Hans / Hant)が書いてあれば地域より優先する。
      if (parts.includes("hans")) return "zh-Hans";
      if (parts.includes("hant") || parts.some((p) => TRADITIONAL_REGIONS.has(p))) return "zh-Hant";
      return "zh-Hans";
    }
  }
  return FALLBACK_LANG;
}

export function initI18n(lang: Lang): typeof i18n {
  void i18n.use(initReactI18next).init({
    resources: Object.fromEntries(LANGS.map((l) => [l, { translation: RESOURCES[l] }])),
    lng: lang,
    fallbackLng: FALLBACK_LANG,
    interpolation: { escapeValue: false }, // React が逃がす(dangerouslySetInnerHTML は使わない)。
    returnNull: false,
  });
  return i18n;
}
