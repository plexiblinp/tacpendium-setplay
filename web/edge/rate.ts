// isolate の中だけの簡易な回数制限(HANDOVER §7.2。無料ドメインでは WAF を置けないので Worker のコードで行う)。
// isolate は Cloudflare の拠点・時間ごとに別なので、厳密な上限ではない(連打・1 本のスクリプトの水増しを抑える程度)。

/** 覚えておく鍵の数の上限。超えたら古い窓の鍵から捨てる(メモリを食い続けないように)。 */
const MAX_KEYS = 10_000;

export class WindowLimiter {
  private readonly hits = new Map<string, { window: number; count: number }>();

  constructor(
    private readonly limit: number,
    private readonly windowMs: number,
  ) {}

  /** 1 回数える。窓の中で limit を超えたら false。 */
  take(key: string, nowMs: number): boolean {
    const window = Math.floor(nowMs / this.windowMs);
    const h = this.hits.get(key);
    if (h == null || h.window !== window) {
      if (this.hits.size >= MAX_KEYS) this.prune(window);
      this.hits.set(key, { window, count: 1 });
      return true;
    }
    h.count++;
    return h.count <= this.limit;
  }

  private prune(window: number): void {
    for (const [k, h] of this.hits) if (h.window !== window) this.hits.delete(k);
    if (this.hits.size >= MAX_KEYS) this.hits.clear();
  }
}
