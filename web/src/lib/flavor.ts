// ビルドの種類(サイト / ローカル版。HANDOVER D10・§5)。ローカル版は `vite build --mode standalone` で作る(scripts/local/build.sh)。
// 違いは KA の上限と、外への通信(計測・票)をしないことだけ。計算はどちらもブラウザの WASM。
// ★画面(components/)だけが読む。lib/ の純粋な関数・edge/(Worker)からは読まない(wrangler の束ねには import.meta.env が無い。
//   読んでもサイトとして動くよう ?. で読むが、依存は edge.test の「import の経路」の試験が止める)。

/** ローカル版の Vite の mode(「local」は Vite が .env.local と紛れるので使えない)。 */
export const LOCAL_BUILD_MODE = "standalone";

export const FLAVORS = ["site", "local"] as const;
export type Flavor = (typeof FLAVORS)[number];

/** mode からビルドの種類を決める(standalone 以外はサイト)。 */
export function flavorFromMode(mode: string | undefined): Flavor {
  return mode === LOCAL_BUILD_MODE ? "local" : "site";
}

/** このビルドの種類。 */
export const FLAVOR: Flavor = flavorFromMode(import.meta.env?.MODE);
