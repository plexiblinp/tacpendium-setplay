// 画面の最上位(App)が起動時に読むもの(manifest・抽出率)の Hooks。App から切り出した(2026-10-31 の health)。
import { useEffect, useState } from "react";

import { loadManifest, type FetchFn, type LoadedManifest } from "../lib/loader";
import { DEFAULT_SAMPLE_RATE, loadConfig } from "../lib/telemetry";

export type ManifestState = { status: "loading" } | { status: "error"; message: string } | { status: "ready"; data: LoadedManifest };

/** 配信データの manifest を読む。 */
export function useManifest(fetchFn: FetchFn, href: string): ManifestState {
  const [manifest, setManifest] = useState<ManifestState>({ status: "loading" });
  useEffect(() => {
    let alive = true;
    loadManifest(fetchFn, href).then(
      (data) => alive && setManifest({ status: "ready", data }),
      (e: unknown) => alive && setManifest({ status: "error", message: e instanceof Error ? e.message : String(e) }),
    );
    return () => {
      alive = false;
    };
  }, [fetchFn, href]);
  return manifest;
}

/** 計測の抽出率。サイトだけが読み、ローカル版(online が false)は既定のまま(通信しない)。 */
export function useSampleRate(online: boolean, fetchFn: FetchFn, href: string): number {
  const [sampleRate, setSampleRate] = useState(DEFAULT_SAMPLE_RATE);
  useEffect(() => {
    if (!online) return;
    let alive = true;
    void loadConfig(fetchFn, href).then((c) => alive && setSampleRate(c.beaconSampleRate));
    return () => {
      alive = false;
    };
  }, [online, fetchFn, href]);
  return sampleRate;
}
