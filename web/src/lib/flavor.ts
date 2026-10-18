// ビルドの種類(サイト / ローカル版。HANDOVER D10・§5)。ローカル版は `vite build --mode local` で作る(scripts/local/build.sh)。
// 違いは KA の上限と、外への通信(計測・票)をしないことだけ。計算はどちらもブラウザの WASM。
// ★edge/(Worker)はこのモジュールを読まないこと(wrangler の束ねでは import.meta.env が無い)。
import { kaMaxFor } from "./constants";

/** ローカル版の Vite の mode。 */
export const LOCAL_BUILD_MODE = "local";

export const FLAVORS = ["site", "local"] as const;
export type Flavor = (typeof FLAVORS)[number];

/** mode からビルドの種類を決める(local 以外はサイト)。 */
export function flavorFromMode(mode: string | undefined): Flavor {
  return mode === LOCAL_BUILD_MODE ? "local" : "site";
}

/** このビルドの種類。 */
export const FLAVOR: Flavor = flavorFromMode(import.meta.env?.MODE);

/** このビルドの KA の上限(画面の欄・共有 URL の検証)。 */
export const KA_MAX = kaMaxFor(FLAVOR);
