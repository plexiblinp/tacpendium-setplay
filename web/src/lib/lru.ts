/** 件数で古いものから捨てるメモリキャッシュ(同一条件の再計算をしない。HANDOVER §7.2・D11)。 */
export class LruCache<K, V> {
  private readonly map = new Map<K, V>();

  constructor(private readonly capacity: number) {
    if (!Number.isInteger(capacity) || capacity < 1) {
      throw new Error(`LruCache: capacity は 1 以上の整数(${capacity})`);
    }
  }

  get(key: K): V | undefined {
    const v = this.map.get(key);
    if (v === undefined) return undefined;
    // 使ったものを新しい側へ動かす。
    this.map.delete(key);
    this.map.set(key, v);
    return v;
  }

  set(key: K, value: V): void {
    this.map.delete(key);
    this.map.set(key, value);
    while (this.map.size > this.capacity) {
      const oldest = this.map.keys().next();
      if (oldest.done) break;
      this.map.delete(oldest.value);
    }
  }

  /** 全部捨てる(票が変わり、覚えた並びが古くなったとき)。 */
  clear(): void {
    this.map.clear();
  }

  get size(): number {
    return this.map.size;
  }
}
