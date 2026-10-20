import { act, cleanup, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import type { SuggestOutcome } from "../lib/engineClient";
import type { SuggestResponse } from "../lib/engineTypes";
import { KA_MAX_LOCAL, RELEASES_URL } from "../lib/constants";
import type { Flavor } from "../lib/flavor";
import { initI18n } from "../lib/i18n";
import { beaconSchema, type BeaconEvent } from "../shared/api";
import type { SuggestRequest } from "../worker/protocol";
import { App, type SuggestEngine } from "./App";

const MANIFEST = {
  schemaVersion: 1,
  dataVersion: "ffdb69e468a3f5c0b0e16255e68b6baa3266796d",
  maxKnockdownAdvantage: 100,
  characters: [
    { code: "ryu", name: { ja: "リュウ", en: "Ryu" }, dataVersion: "x", calcFingerprint: "f", safeJump: { true: 43, f5: 42, f6: 41 }, moves: "moves/ryu.1.json" },
    { code: "ken", name: { ja: "ケン", en: "Ken" }, dataVersion: "x", calcFingerprint: "f", safeJump: null, moves: "moves/ken.1.json" },
  ],
};
const mv = (code: string, nameJa: string, srk: string, category = "normal") => ({
  code,
  category,
  damage: 300,
  notation: { nameJa, nameEn: `${code} en`, srk, numeric: `${srk}#` },
});
const MOVES = {
  ryu: { character: { code: "ryu", name: MANIFEST.characters[0]!.name }, moves: [mv("lp", "弱P", "cr.LP"), mv("hado", "波動拳", "236P", "special"), mv("dash", "前ステップ", "66", "system")] },
  ken: { character: { code: "ken", name: MANIFEST.characters[1]!.name }, moves: [mv("lk", "弱K", "cr.LK")] },
};

const RESULT: SuggestResponse = {
  items: Array.from({ length: 250 }, (_, i) => ({
    steps: [
      { code: "lp", role: "filler", counted: true },
      { code: "hado", role: "target", counted: true },
    ],
    s: 30,
    n: i + 1,
    landing: 41,
    targetActive: 300,
    mode: "meaty",
    g: 0,
    remain: 0,
    recipeKey: `lp${i}|hado`,
  })),
  truncated: false,
  totalFound: 250,
};

const sendBeacon = vi.fn((_url: string, _body: string) => true);

const fetchFn = vi.fn(async (url: string) => {
  const u = new URL(url);
  let body: unknown;
  if (u.pathname.endsWith("manifest.json")) body = MANIFEST;
  else if (u.pathname.includes("/moves/ryu")) body = MOVES.ryu;
  else if (u.pathname.includes("/moves/ken")) body = MOVES.ken;
  else return new Response("", { status: 404 });
  return new Response(JSON.stringify(body), { status: 200 });
});

class FakeEngine implements SuggestEngine {
  calls: Omit<SuggestRequest, "type" | "id">[] = [];
  private resolve: ((o: SuggestOutcome) => void) | null = null;
  canceled = 0;
  suggest(req: Omit<SuggestRequest, "type" | "id">) {
    this.calls.push(req);
    return new Promise<SuggestOutcome>((r) => (this.resolve = r));
  }
  cancel() {
    this.canceled++;
    this.resolve?.({ kind: "canceled" });
    this.resolve = null;
  }
  /** 計算の要求が来るのを待って(票の取得の後に来る)、結果を返す。 */
  async finish(o: SuggestOutcome = { kind: "result", result: RESULT, timings: { boot: 0, load: 0, compute: 1, wasmMemoryBytes: 0 }, wallMs: 12 }) {
    await waitFor(() => expect(this.resolve).not.toBeNull());
    act(() => {
      this.resolve?.(o);
    });
    this.resolve = null;
  }
}

function renderApp(search = "", languages = ["ja"], fetchImpl: typeof fetchFn = fetchFn, flavor?: Flavor) {
  const engine = new FakeEngine();
  const replaceSearch = vi.fn();
  const view = render(
    <App
      engine={engine}
      fetchFn={fetchImpl}
      location={{ href: `https://example.test/${search ? `?${search}` : ""}`, search }}
      replaceSearch={replaceSearch}
      languages={languages}
      onLangChange={() => undefined}
      sendBeacon={sendBeacon}
      random={() => 0}
      flavor={flavor}
    />,
  );
  return { engine, replaceSearch, view };
}

beforeAll(() => {
  initI18n("ja");
});
afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

/** キャラを選ぶ(箱を押して、一覧から選ぶ。CharacterSelect)。 */
async function pickCharacter(user: ReturnType<typeof userEvent.setup>, code: string) {
  await user.click(await screen.findByTestId("character-select"));
  await user.click(await screen.findByTestId(`character-option-${code}`));
}

async function search(user: ReturnType<typeof userEvent.setup>, ch = "ryu", ka = "40") {
  await pickCharacter(user, ch);
  await user.clear(screen.getByTestId("ka-input"));
  await user.type(screen.getByTestId("ka-input"), ka);
  await user.click(screen.getByTestId("generate"));
}

describe("App", () => {
  it("キャラと KA を入れて提案を出す → 計算中は押せず中断が出る → 結果を 200 件ずつ出す", async () => {
    const user = userEvent.setup();
    const { engine, replaceSearch } = renderApp();
    expect(await screen.findByTestId("generate")).toBeDisabled();
    await search(user);
    expect(screen.getByTestId("loading")).toBeInTheDocument();
    expect(screen.getByTestId("generate")).toBeDisabled();
    expect(engine.calls[0]).toMatchObject({ character: "ryu", ka: 40, movesUrl: "https://example.test/data/moves/ryu.1.json" });
    expect(engine.calls[0]!.query).toContain("limit=3000");
    expect(replaceSearch).toHaveBeenCalledWith("c=ryu&ka=40");
    await engine.finish();
    expect(await screen.findAllByTestId("result-row")).toHaveLength(200);
    expect(screen.getByTestId("count")).toHaveTextContent("全250件中200件を表示");
    await user.click(screen.getByTestId("show-more"));
    expect(screen.getAllByTestId("result-row")).toHaveLength(250);
    expect(screen.queryByTestId("show-more")).toBeNull();
    expect(engine.calls).toHaveLength(1); // 「さらに表示」で計算し直さない
  });

  it("表記を切り替えると行の技名が変わる(既定は ja = 日本語技名)", async () => {
    const user = userEvent.setup();
    const { engine } = renderApp();
    await search(user);
    await engine.finish();
    const first = (await screen.findAllByTestId("recipe"))[0]!;
    expect(first).toHaveTextContent("弱P > 波動拳");
    await user.click(screen.getByTestId("notation-srk"));
    expect(screen.getAllByTestId("recipe")[0]).toHaveTextContent("cr.LP > 236P");
    await user.click(screen.getByTestId("notation-numeric"));
    expect(screen.getAllByTestId("recipe")[0]).toHaveTextContent("cr.LP# > 236P#");
  });

  it("英語 UI の既定の表記は SRK", async () => {
    const user = userEvent.setup();
    const { engine } = renderApp("", ["en-US"]);
    await search(user);
    await engine.finish();
    expect((await screen.findAllByTestId("recipe"))[0]).toHaveTextContent("cr.LP > 236P");
    expect(screen.getByTestId("count")).toHaveTextContent("Showing 200 of 250");
  });

  it("コピーは選択中の表記のレシピとフレームの説明を書く", async () => {
    const user = userEvent.setup();
    const writeText = vi.fn(async () => undefined);
    Object.defineProperty(navigator, "clipboard", { value: { writeText }, configurable: true });
    const { engine } = renderApp();
    await search(user);
    await engine.finish();
    await user.click((await screen.findAllByTestId("copy"))[2]!);
    expect(writeText).toHaveBeenCalledWith("弱P > 波動拳 (持続3F目ヒット)");
    expect(await screen.findByText("コピーしました")).toBeInTheDocument();
  });

  it("コピーに失敗したら失敗を出す", async () => {
    const user = userEvent.setup();
    Object.defineProperty(navigator, "clipboard", { value: { writeText: async () => Promise.reject(new Error("denied")) }, configurable: true });
    const { engine } = renderApp();
    await search(user);
    await engine.finish();
    await user.click((await screen.findAllByTestId("copy"))[0]!);
    expect(await screen.findByText("コピーできませんでした")).toBeInTheDocument();
  });

  it("同じ条件は計算せずキャッシュから出す", async () => {
    const user = userEvent.setup();
    const { engine } = renderApp();
    await search(user);
    await engine.finish();
    await screen.findAllByTestId("result-row");
    await user.click(screen.getByTestId("generate"));
    expect(engine.calls).toHaveLength(1);
    expect(screen.getByTestId("timing")).toHaveTextContent("計算済みの結果");
  });

  it("中断すると中断の表示になり、Worker を止める", async () => {
    const user = userEvent.setup();
    const { engine } = renderApp();
    await search(user);
    await user.click(screen.getByTestId("cancel"));
    expect(engine.canceled).toBe(1);
    expect(screen.getByTestId("canceled")).toBeInTheDocument();
    expect(screen.getByTestId("generate")).toBeEnabled();
  });

  it("共有 URL の条件で自動で計算し、欄にも入れる", async () => {
    const { engine } = renderApp("c=ryu&ka=63&mode=gap&g_min=2&chain=1");
    await waitFor(() => expect(engine.calls).toHaveLength(1));
    expect(engine.calls[0]).toMatchObject({ character: "ryu", ka: 63 });
    expect(engine.calls[0]!.query).toContain("mode=gap");
    expect(engine.calls[0]!.query).toContain("g_min=2");
    expect(engine.calls[0]!.query).toContain("chain_cancel_only=true");
    expect(screen.getByTestId("ka-input")).toHaveValue(63);
    expect(screen.getByTestId("mode-gap")).toHaveAttribute("aria-pressed", "true");
  });

  it("共有 URL の安全飛びは、URL の remain ではなくキャラの値で計算する", async () => {
    const { engine } = renderApp("c=ryu&ka=63&mode=filler&sj=f5&remain=7");
    await waitFor(() => expect(engine.calls).toHaveLength(1));
    expect(engine.calls[0]!.query).toContain("remain=42");
    await engine.finish({ kind: "result", result: { ...RESULT, items: [{ ...RESULT.items[0]!, mode: "filler", remain: 42 }] }, timings: { boot: 0, load: 0, compute: 1, wasmMemoryBytes: 0 }, wallMs: 1 });
    expect(await screen.findByTestId("frame-label")).toHaveTextContent("5F安全飛び(残り42F)");
  });

  it("共有 URL の誤ったキーは既定値に戻して知らせる", async () => {
    renderApp("c=ryu&ka=40&mode=bogus");
    expect(await screen.findByTestId("url-rejected")).toHaveTextContent("mode");
  });

  it("安全飛びのボタンは値で押せ、値の無いキャラでは押せない", async () => {
    const user = userEvent.setup();
    const { engine } = renderApp();
    await pickCharacter(user, "ryu");
    await user.type(screen.getByTestId("ka-input"), "60");
    await user.click(screen.getByTestId("mode-filler"));
    const sj = within(screen.getByTestId("safe-jump"));
    await user.click(sj.getByText("6F安全飛び(+41)"));
    expect(engine.calls[0]!.query).toContain("remain=41");
    engine.cancel();
    await pickCharacter(user, "ken");
    expect(screen.getByTestId("safe-jump-f5")).toBeDisabled();
    expect(screen.getByTestId("safe-jump-unavailable")).toBeInTheDocument();
  });

  it("欄の誤りは理由を出して押せない", async () => {
    const user = userEvent.setup();
    renderApp();
    await pickCharacter(user, "ryu");
    await user.type(screen.getByTestId("ka-input"), "101");
    expect(screen.getByTestId("form-error-kaInvalid")).toHaveTextContent("0〜100 の整数");
    expect(screen.getByTestId("generate")).toBeDisabled();
    await user.clear(screen.getByTestId("ka-input"));
    await user.type(screen.getByTestId("ka-input"), "80");
    expect(screen.getByTestId("slow-warning")).toBeInTheDocument();
    await user.type(screen.getByTestId("n-max"), "0");
    await user.clear(screen.getByTestId("n-min"));
    await user.type(screen.getByTestId("n-min"), "3");
    await user.clear(screen.getByTestId("n-max"));
    await user.type(screen.getByTestId("n-max"), "2");
    expect(screen.getByTestId("form-error-nMaxBelowMin")).toBeInTheDocument();
    expect(screen.getByTestId("generate")).toBeDisabled();
  });

  it("重ねる技を「1 つの技を指定」にすると、種類の欄が消え、system を出さず target_move で送る", async () => {
    const user = userEvent.setup();
    const { engine } = renderApp();
    await pickCharacter(user, "ryu");
    await user.type(screen.getByTestId("ka-input"), "40");
    expect(screen.getByTestId("target-fieldset")).toHaveTextContent("重ねる技");
    await user.click(screen.getByTestId("target-by-move"));
    expect(screen.queryByTestId("type-fieldset")).toBeNull();
    const picker = await screen.findByTestId("picker-move");
    await waitFor(() => expect(within(picker).getByText("波動拳")).toBeInTheDocument());
    expect(within(picker).queryByText("前ステップ")).toBeNull();
    await user.selectOptions(picker, "hado");
    await user.click(screen.getByTestId("generate"));
    expect(engine.calls[0]!.query).toContain("target_move=hado");
  });

  it("「種類で選ぶ」に戻すと選んでいた技を外し、種類で送る", async () => {
    const user = userEvent.setup();
    const { engine } = renderApp();
    await pickCharacter(user, "ryu");
    await user.type(screen.getByTestId("ka-input"), "40");
    await user.click(screen.getByTestId("target-by-move"));
    const picker = await screen.findByTestId("picker-move");
    await waitFor(() => expect(within(picker).getByText("波動拳")).toBeInTheDocument());
    await user.selectOptions(picker, "hado");
    await user.click(screen.getByTestId("target-by-type"));
    expect(screen.getByTestId("type-fieldset")).toBeInTheDocument();
    expect(screen.queryByTestId("picker-move")).toBeNull();
    await user.click(screen.getByTestId("generate"));
    expect(engine.calls[0]!.query).not.toContain("target_move");
  });

  it("URL の条件に技があれば「1 つの技を指定」で開き、技の欄を空にしても種類の欄へ変わらない", async () => {
    const user = userEvent.setup();
    renderApp("?c=ryu&ka=40&move=hado");
    const picker = await screen.findByTestId("picker-move");
    expect(screen.getByTestId("target-by-move")).toHaveAttribute("aria-pressed", "true");
    await waitFor(() => expect(within(picker).getByText("波動拳")).toBeInTheDocument());
    await user.selectOptions(picker, "");
    expect(screen.getByTestId("picker-move")).toBeInTheDocument();
    expect(screen.queryByTestId("type-fieldset")).toBeNull();
  });

  it("「1 つの技を指定」で技が未選択なら押せない。隠れた種類の誤りは出さない", async () => {
    const user = userEvent.setup();
    renderApp();
    await pickCharacter(user, "ryu");
    await user.type(screen.getByTestId("ka-input"), "40");
    // 種類は既定(3 種類)のまま。技を選ばずに押せると、隠れた種類で計算してしまう。
    await user.click(screen.getByTestId("target-by-move"));
    expect(screen.getByTestId("generate")).toBeDisabled();
    expect(screen.getByTestId("target-move-required")).toBeInTheDocument();
    // 種類を 0 個にしてから切り替えても、直せない隠れた欄の誤りは出さない。
    await user.click(screen.getByTestId("target-by-type"));
    for (const type of ["normal", "unique", "special_projectile"]) await user.click(screen.getByTestId(`type-${type}`));
    expect(screen.getByTestId("form-error-targetTypesRequired")).toBeInTheDocument();
    await user.click(screen.getByTestId("target-by-move"));
    expect(screen.queryByTestId("form-error-targetTypesRequired")).toBeNull();
    const picker = await screen.findByTestId("picker-move");
    await waitFor(() => expect(within(picker).getByText("波動拳")).toBeInTheDocument());
    await user.selectOptions(picker, "hado");
    expect(screen.getByTestId("generate")).toBeEnabled();
    expect(screen.queryByTestId("target-move-required")).toBeNull();
  });

  it("使い方と注意: 冒頭の説明・本家への導線・3 つの折りたたみ(使い方・注意と対策・レシピのルール)", async () => {
    renderApp();
    expect(await screen.findByTestId("guide")).toHaveTextContent("ゲーム内で確かめてください");
    // アプリ名(訳さない)と副題(S7)。本家から切り出したことと、本家を使う利点。
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent("Tacpendium-Setplay");
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent("セットプレイ自動提案");
    expect(document.title).toBe("Tacpendium-Setplay | セットプレイ自動提案(非公式)");
    expect(screen.getByTestId("guide-upstream")).toHaveTextContent("切り出した Web 版");
    expect(screen.getByTestId("guide-upstream")).toHaveTextContent("確定反撃と紐付け");
    expect(screen.getByTestId("guide-upstream")).toHaveTextContent("日本語のみ対応です(英語は一部、ほかの言語は未対応)");
    const upstream = within(screen.getByTestId("guide-upstream")).getByRole("link");
    expect(upstream).toHaveAttribute("href", "https://github.com/plexiblinp/tacpendium");
    expect(screen.getByTestId("guide-howto")).toHaveTextContent("棒立ちで有利フレームを見る");
    expect(screen.getByTestId("guide-cautions")).toHaveTextContent("→ 「接近を含む提案だけ」にチェック");
    expect(screen.getByTestId("guide-rules")).toHaveTextContent("配信開始から2週間後");
    expect(screen.getByTestId("guide-rules")).toHaveTextContent("しゃがみ弱P > しゃがみ弱P");
  });

  it("キャラの選択(本家と同じ形): 箱を押すと検索欄と一覧が開き、語の先頭一致で絞り、Enter・↑↓・Esc が効く", async () => {
    const user = userEvent.setup();
    renderApp();
    const trigger = await screen.findByTestId("character-select");
    expect(trigger).toHaveAttribute("aria-expanded", "false");
    expect(screen.queryByTestId("character-search")).toBeNull();
    await user.click(trigger);
    expect(trigger).toHaveAttribute("aria-expanded", "true");
    expect(screen.getByTestId("character-search")).toHaveFocus();
    await user.keyboard("ケ");
    expect(screen.queryByTestId("character-option-ryu")).toBeNull();
    expect(screen.getByTestId("character-option-ken")).toBeInTheDocument();
    await user.keyboard("{Enter}");
    expect(trigger).toHaveAttribute("data-value", "ken");
    expect(trigger).toHaveTextContent("ケン");
    expect(screen.queryByTestId("character-search")).toBeNull();
    // 開き直すと検索語は消えている。↓ で先頭、もう一度 ↓ で次の候補へ移り、Enter で選ぶ。
    await user.click(trigger);
    expect(screen.getByTestId("character-search")).toHaveValue("");
    await user.keyboard("{ArrowDown}");
    expect(screen.getByTestId("character-option-ryu")).toHaveFocus();
    await user.keyboard("{ArrowDown}{Enter}");
    expect(trigger).toHaveAttribute("data-value", "ken");
    // 一致なし・Esc で閉じる(選択は変わらない)。
    await user.click(trigger);
    await user.keyboard("zzz");
    expect(screen.getByTestId("character-no-match")).toBeInTheDocument();
    await user.keyboard("{Escape}");
    expect(screen.queryByTestId("character-search")).toBeNull();
    expect(trigger).toHaveAttribute("data-value", "ken");
  });

  it("条件をリセット: 欄・結果・URL を最初に戻す(言語は戻さない)", async () => {
    const user = userEvent.setup();
    const { engine, replaceSearch } = renderApp("", ["en"]);
    await search(user);
    await user.click(screen.getByTestId("filter-approach"));
    await user.click(screen.getByTestId("mode-gap"));
    await user.click(screen.getByTestId("reset"));
    expect(screen.getByTestId("character-select")).toHaveAttribute("data-value", "");
    expect(screen.getByTestId("ka-input")).toHaveValue(null);
    expect(screen.getByTestId("mode-meaty")).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByTestId("filter-approach")).not.toBeChecked();
    expect(screen.getByText(/Set conditions and press/)).toBeInTheDocument();
    expect(replaceSearch).toHaveBeenLastCalledWith("");
    expect(engine.canceled).toBe(1);
    expect(document.documentElement.lang).toBe("en");
  });

  it("エンジンの誤り・理由・0 件・打ち切りの表示", async () => {
    const user = userEvent.setup();
    const { engine } = renderApp();
    await search(user);
    await engine.finish({ kind: "error", error: { code: "worker_failed", message: "oom" }, wallMs: 1 });
    expect(await screen.findByTestId("run-error")).toHaveTextContent("worker_failed");
    await user.clear(screen.getByTestId("ka-input"));
    await user.type(screen.getByTestId("ka-input"), "41");
    await user.click(screen.getByTestId("filter-approach"));
    await user.click(screen.getByTestId("generate"));
    await engine.finish({ kind: "result", result: { items: [], truncated: true, totalFound: 0 }, timings: { boot: 0, load: 0, compute: 1, wasmMemoryBytes: 0 }, wallMs: 1 });
    expect(await screen.findByTestId("empty")).toHaveTextContent("絞り込みを外すと");
    expect(screen.getByTestId("truncated")).toBeInTheDocument();
  });

  it("0 件のとき、エンジンに送っていない絞り込み(汚連携のガード時 0F 以上)では「絞り込みを外すと」を出さない", async () => {
    const user = userEvent.setup();
    const { engine } = renderApp();
    await pickCharacter(user, "ryu");
    await user.type(screen.getByTestId("ka-input"), "40");
    await user.click(screen.getByTestId("filter-block-safe"));
    await user.click(screen.getByTestId("mode-gap"));
    await user.click(screen.getByTestId("generate"));
    expect(engine.calls[0]!.query).not.toContain("block_safe_only");
    await engine.finish({ kind: "result", result: { items: [], truncated: false, totalFound: 0 }, timings: { boot: 0, load: 0, compute: 1, wasmMemoryBytes: 0 }, wallMs: 1 });
    expect(await screen.findByTestId("empty")).toHaveTextContent("提案が見つかりませんでした");
  });

  it("言語を切り替えると文言と既定の表記が変わり、機械翻訳の注記が出る", async () => {
    const user = userEvent.setup();
    const { engine } = renderApp();
    await search(user);
    await engine.finish();
    await user.selectOptions(screen.getByTestId("lang-select"), "ko");
    expect(await screen.findByText("제안 생성")).toBeInTheDocument();
    expect(screen.getByTestId("machine-translation")).toBeInTheDocument();
    // 改善の提案は Google フォームへ(S7)。
    expect(within(screen.getByTestId("machine-translation")).getByRole("link")).toHaveAttribute("href", expect.stringMatching(/^https:\/\/docs\.google\.com\/forms\//));
    expect(screen.getAllByTestId("recipe")[0]).toHaveTextContent("cr.LP > 236P");
    await user.selectOptions(screen.getByTestId("lang-select"), "zh-Hant");
    expect(await screen.findByText("生成推薦")).toBeInTheDocument();
    await user.selectOptions(screen.getByTestId("lang-select"), "ja");
    expect(screen.queryByTestId("machine-translation")).toBeNull();
  });

  it("データ版を出す・manifest が読めなければ誤り", async () => {
    renderApp();
    expect(await screen.findByTestId("data-version")).toHaveTextContent("データ版 ffdb69e");
    // 非公式・非営利の明記(開発者の目視の要望 2026-10-17)。
    expect(screen.getByText(/非公式・非営利のツールです/)).toBeInTheDocument();
    cleanup();
    render(
      <App
        engine={new FakeEngine()}
        fetchFn={async () => new Response("", { status: 500 })}
        location={{ href: "https://example.test/", search: "" }}
        replaceSearch={() => undefined}
        languages={["ja"]}
        onLangChange={() => undefined}
        sendBeacon={() => true}
      />,
    );
    expect(await screen.findByTestId("data-error")).toBeInTheDocument();
  });
});

/** fetchFn に票の受け口と config.json を足す。 */
function withApi(opts: { votes?: Record<string, [number, number]>; sampleRate?: number; voteReply?: (body: unknown) => Response }) {
  const posted: unknown[] = [];
  const f = vi.fn(async (url: string, init?: RequestInit) => {
    const u = new URL(url);
    if (u.pathname === "/api/votes") {
      return opts.votes ? new Response(JSON.stringify({ votes: opts.votes }), { status: 200 }) : new Response("", { status: 503 });
    }
    if (u.pathname === "/api/vote") {
      const body = JSON.parse(String(init?.body));
      posted.push(body);
      return opts.voteReply ? opts.voteReply(body) : new Response(JSON.stringify({ good: 8, bad: 1, value: body.value }), { status: 200 });
    }
    if (u.pathname === "/config.json" && opts.sampleRate != null) {
      return new Response(JSON.stringify({ schemaVersion: 1, beaconSampleRate: opts.sampleRate }), { status: 200 });
    }
    return fetchFn(url);
  });
  return { f, posted };
}

function beacons(): BeaconEvent[] {
  return sendBeacon.mock.calls.map(([url, body]) => {
    expect(url).toBe("https://example.test/api/beacon");
    return beaconSchema.parse(JSON.parse(body));
  });
}

describe("計測(beacon。HANDOVER §7.1)", () => {
  it("実行ごとに 1 つ送る(結果・キャッシュ)。中身は受け口の形に合う", async () => {
    const user = userEvent.setup();
    const { engine } = renderApp();
    await search(user);
    await engine.finish();
    await screen.findAllByTestId("result-row");
    await waitFor(() => expect(beacons()).toHaveLength(1));
    expect(beacons()[0]).toMatchObject({
      char: "ryu",
      ka: 40,
      mode: "meaty",
      nMin: 1,
      gMin: null,
      remain: null,
      outcome: "result",
      resultCount: 250,
      items: 250,
      cacheHit: false,
      computeMs: 1,
      votesStatus: "unavailable", // 票の受け口が無い(404)
      votesApplied: 0,
      notation: "nameJa",
      lang: "ja",
      dataVersion: MANIFEST.dataVersion,
      sampleRate: 1,
    });
    await user.click(screen.getByTestId("generate"));
    await waitFor(() => expect(beacons()).toHaveLength(2));
    expect(beacons()[1]).toMatchObject({ cacheHit: true, computeMs: null, outcome: "result" });
  });

  it("誤りと中断も送る", async () => {
    const user = userEvent.setup();
    const { engine } = renderApp();
    await search(user);
    await engine.finish({ kind: "error", error: { code: "worker_failed", message: "oom" }, wallMs: 1 });
    await screen.findByTestId("run-error");
    await user.clear(screen.getByTestId("ka-input"));
    await user.type(screen.getByTestId("ka-input"), "41");
    await user.click(screen.getByTestId("generate"));
    await waitFor(() => expect(engine.calls).toHaveLength(2));
    await user.click(screen.getByTestId("cancel"));
    await waitFor(() => expect(beacons()).toHaveLength(2));
    expect(beacons()[0]).toMatchObject({ outcome: "error", errorCode: "worker_failed", resultCount: null, items: null });
    expect(beacons()[1]).toMatchObject({ outcome: "canceled", ka: 41, resultCount: null });
  });

  it("config.json の抽出率 0 なら送らない", async () => {
    const user = userEvent.setup();
    const { f } = withApi({ sampleRate: 0 });
    const { engine } = renderApp("", ["ja"], f);
    await waitFor(() => expect(f.mock.calls.some(([u]) => String(u).endsWith("/config.json"))).toBe(true));
    await search(user);
    await engine.finish();
    await screen.findAllByTestId("result-row");
    expect(sendBeacon).not.toHaveBeenCalled();
  });
});

describe("グッド/バッド(D17)", () => {
  it("票を読み、スコア(good − bad)が 0 でない鍵だけをエンジンに渡し、行に数を出す", async () => {
    const user = userEvent.setup();
    const { f } = withApi({ votes: { "lp2|hado": [5, 1], "lp3|hado": [2, 2], "zzz|": [0, 3] } });
    const { engine } = renderApp("", ["ja"], f);
    await search(user);
    await waitFor(() => expect(engine.calls).toHaveLength(1));
    expect(JSON.parse(engine.calls[0]!.votes!)).toEqual({ "lp2|hado": 4, "zzz|": -3 });
    expect(f.mock.calls.some(([u]) => String(u) === "https://example.test/api/votes?char=ryu&ka=40")).toBe(true);
    await engine.finish();
    const rows = await screen.findAllByTestId("result-row");
    expect(within(rows[2]!).getByTestId("good-count")).toHaveTextContent("5");
    expect(within(rows[2]!).getByTestId("bad-count")).toHaveTextContent("1");
    expect(within(rows[0]!).getByTestId("good-count")).toHaveTextContent("0");
    expect(screen.getByTestId("votes-note")).toHaveAttribute("data-votes-status", "ok");
    await waitFor(() => expect(beacons()[0]).toMatchObject({ votesStatus: "ok", votesApplied: 2 }));
  });

  it("票が読めなければ票なしで計算し(votes を渡さない)、その旨を出す", async () => {
    const user = userEvent.setup();
    const { f } = withApi({});
    const { engine } = renderApp("", ["ja"], f);
    await search(user);
    await waitFor(() => expect(engine.calls).toHaveLength(1));
    expect(engine.calls[0]!.votes).toBeUndefined();
    await engine.finish();
    expect(await screen.findByTestId("votes-note")).toHaveTextContent("票を読み込めなかった");
  });

  it("押すと送り、数と押した状態を出す。同じ票をもう一度押すと取り消す。失敗は出す", async () => {
    const user = userEvent.setup();
    let fail = false;
    const { f, posted } = withApi({
      votes: {},
      voteReply: (b) => (fail ? new Response("", { status: 429 }) : new Response(JSON.stringify({ good: 8, bad: 1, value: (b as { value: number }).value }), { status: 200 })),
    });
    const { engine } = renderApp("", ["ja"], f);
    await search(user);
    await engine.finish();
    const row = (await screen.findAllByTestId("result-row"))[1]!;
    await user.click(within(row).getByTestId("vote-good"));
    await waitFor(() => expect(within(row).getByTestId("good-count")).toHaveTextContent("8"));
    expect(within(row).getByTestId("vote-good")).toHaveAttribute("aria-pressed", "true");
    expect(posted[0]).toEqual({ char: "ryu", ka: 40, recipe: "lp1|hado", value: 1 });
    await user.click(within(row).getByTestId("vote-good"));
    await waitFor(() => expect(posted[1]).toEqual({ char: "ryu", ka: 40, recipe: "lp1|hado", value: 0 }));
    await waitFor(() => expect(within(row).getByTestId("vote-good")).toHaveAttribute("aria-pressed", "false"));
    fail = true;
    await user.click(within(row).getByTestId("vote-bad"));
    expect(await within(row).findByTestId("vote-failed")).toHaveTextContent("票を送れませんでした");
  });

  it("投票の後は覚えた結果を捨て、同じ条件でも票を読み直して計算する", async () => {
    const user = userEvent.setup();
    const { f } = withApi({ votes: {} });
    const { engine } = renderApp("", ["ja"], f);
    await search(user);
    await engine.finish();
    const row = (await screen.findAllByTestId("result-row"))[0]!;
    await user.click(within(row).getByTestId("vote-good"));
    await waitFor(() => expect(within(row).getByTestId("good-count")).toHaveTextContent("8"));
    await user.click(screen.getByTestId("generate"));
    await waitFor(() => expect(engine.calls).toHaveLength(2));
    // 次の計算の後は、読み直した票の数を出す(投票の時点の数 8 を残さない。押した状態は残す)。
    await engine.finish();
    await waitFor(() => expect(within(screen.getAllByTestId("result-row")[0]!).getByTestId("good-count")).toHaveTextContent("0"));
    expect(within(screen.getAllByTestId("result-row")[0]!).getByTestId("vote-good")).toHaveAttribute("aria-pressed", "true");
  });
});

describe("プライバシーポリシー(HANDOVER §7.3)", () => {
  it("フッターのリンクで開き、閉じられる。#privacy の URL では最初から開く", async () => {
    const user = userEvent.setup();
    renderApp();
    await user.click(await screen.findByTestId("privacy-link"));
    expect(screen.getByTestId("privacy")).toHaveTextContent("IP アドレスそのものは保存せず");
    // 消したデータが復旧機能で 7 日間戻せること(S7 のプライバシーの調査)。フォームの案内と保存先の文を出す。
    expect(screen.getByTestId("privacy")).toHaveTextContent("7 日間は元に戻せる状態で残ります");
    expect(screen.getByTestId("privacy-form")).toHaveTextContent("開発者の Google アカウントに保存されます");
    expect(within(screen.getByTestId("privacy-form")).getByRole("link")).toHaveAttribute("href", expect.stringMatching(/^https:\/\/docs\.google\.com\/forms\//));
    await user.click(screen.getByTestId("privacy-close"));
    expect(screen.queryByTestId("privacy")).toBeNull();
    cleanup();
    render(
      <App
        engine={new FakeEngine()}
        fetchFn={fetchFn}
        location={{ href: "https://example.test/#privacy", search: "" }}
        replaceSearch={() => undefined}
        languages={["en"]}
        onLangChange={() => undefined}
        sendBeacon={() => true}
      />,
    );
    expect(await screen.findByTestId("privacy")).toHaveTextContent("The IP address itself is not stored");
    expect(screen.getByTestId("privacy-updated")).toHaveTextContent("Last updated: October 18, 2026");
  });
});

describe("隠す(開発者の要望 2026-10-17)", () => {
  it("隠した行はその回の結果から消え、次の行が繰り上がる。すべて戻せる。もう一度検索すると元に戻る", async () => {
    const user = userEvent.setup();
    const { engine } = renderApp();
    await search(user);
    await engine.finish();
    const rows = await screen.findAllByTestId("result-row");
    expect(rows).toHaveLength(200);
    const key = rows[0]!.getAttribute("data-recipe-key");
    await user.click(within(rows[0]!).getByTestId("hide"));
    const after = screen.getAllByTestId("result-row");
    expect(after).toHaveLength(200); // 201 件目が繰り上がる
    expect(after.map((r) => r.getAttribute("data-recipe-key"))).not.toContain(key);
    expect(screen.getByTestId("hidden-note")).toHaveTextContent("1 件を隠しています");
    await user.click(screen.getByTestId("unhide-all"));
    expect(screen.getAllByTestId("result-row")[0]).toHaveAttribute("data-recipe-key", key!);
    await user.click(within(screen.getAllByTestId("result-row")[0]!).getByTestId("hide"));
    await user.click(screen.getByTestId("generate")); // キャッシュから出す新しい結果でも元に戻る
    expect(await screen.findByTestId("timing")).toHaveTextContent("計算済みの結果");
    await waitFor(() => expect(screen.getAllByTestId("result-row")[0]).toHaveAttribute("data-recipe-key", key!));
    expect(screen.queryByTestId("hidden-note")).toBeNull();
  });
});

describe("ローカル版(HANDOVER D10・S8)", () => {
  it("KA 100 を超えても押せ、エンジンにそのまま渡す。説明は上限なしの文", async () => {
    const user = userEvent.setup();
    const { f } = withApi({ votes: { "lp2|hado": [5, 1] }, sampleRate: 1 });
    const { engine } = renderApp("", ["ja"], f, "local");
    expect(await screen.findByTestId("local-badge")).toHaveTextContent("ローカル版");
    expect(screen.getByTestId("ka-help")).toHaveTextContent("0 以上。ローカル版は上限なし");
    expect(screen.queryByTestId("local-version-link")).toBeNull();
    expect(screen.getByTestId("ka-input")).toHaveAttribute("max", String(KA_MAX_LOCAL));
    await search(user, "ryu", "250");
    expect(screen.queryByTestId("form-errors")).toBeNull();
    await engine.finish();
    expect(engine.calls[0]).toMatchObject({ character: "ryu", ka: 250 });
    expect(await screen.findAllByTestId("result-row")).toHaveLength(200);
  });

  it("計測・票の通信をしない(config.json・/api を読まない・beacon を送らない)。票のボタンと案内を出さない", async () => {
    const user = userEvent.setup();
    const { f } = withApi({ votes: { "lp2|hado": [5, 1] }, sampleRate: 1 });
    const { engine } = renderApp("", ["ja"], f, "local");
    await search(user);
    await engine.finish();
    await screen.findAllByTestId("result-row");
    // 2 回目(キャッシュ)と中断も送らない。
    await user.click(screen.getByTestId("generate"));
    await search(user, "ryu", "41");
    await user.click(await screen.findByTestId("cancel"));
    expect(engine.calls[0]!.votes).toBeUndefined();
    expect(sendBeacon).not.toHaveBeenCalled();
    const urls = f.mock.calls.map(([u]) => new URL(String(u)).pathname);
    expect(urls.filter((p) => p.startsWith("/api/") || p.endsWith("config.json"))).toEqual([]);
    expect(urls.some((p) => p.endsWith("manifest.json"))).toBe(true);
    expect(screen.queryByTestId("vote-good")).toBeNull();
    expect(screen.queryByTestId("votes-note")).toBeNull();
    expect(screen.getByTestId("guide-howto")).not.toHaveTextContent("グッド/バッド");
    // プライバシーポリシーの代わりに「通信しない」を出す。#privacy でも開かない。
    expect(screen.queryByTestId("privacy-link")).toBeNull();
    expect(screen.getByTestId("local-notice")).toHaveTextContent("計測・投票の通信はしません");
  });

  it("共有 URL の KA 100 超えを受ける(サイトは誤りとして捨てる)", async () => {
    const local = renderApp("c=ryu&ka=150", ["ja"], fetchFn, "local");
    await waitFor(() => expect(local.engine.calls).toHaveLength(1));
    expect(local.engine.calls[0]).toMatchObject({ ka: 150 });
    cleanup();
    const site = renderApp("c=ryu&ka=150", ["ja"], fetchFn, "site");
    expect(await site.view.findByTestId("url-rejected")).toHaveTextContent("ka");
    expect(site.engine.calls).toHaveLength(0);
  });

  it("サイトは KA 100 を超えると押せず、ローカル版への案内を出す", async () => {
    const user = userEvent.setup();
    const { engine } = renderApp("", ["ja"], fetchFn, "site");
    const link = await screen.findByTestId("local-version-link");
    expect(link).toHaveAttribute("href", RELEASES_URL);
    expect(link).toHaveTextContent("100 を超える場合はローカル版");
    expect(screen.queryByTestId("local-badge")).toBeNull();
    await search(user, "ryu", "101");
    expect(screen.getByTestId("form-error-kaInvalid")).toHaveTextContent("0〜100 の整数");
    expect(engine.calls).toHaveLength(0);
  });

  it("ローカル版の KA の誤りは上限なしの文", async () => {
    const user = userEvent.setup();
    renderApp("", ["ja"], fetchFn, "local");
    await search(user, "ryu", "-1");
    expect(await screen.findByTestId("form-error-kaInvalid")).toHaveTextContent("0 以上の整数を入れてください");
  });
});

describe("この条件を共有(開発者決定 2026-10-20)", () => {
  afterEach(() => {
    Reflect.deleteProperty(navigator, "share");
  });

  it("共有の仕組みが無い(PC)ときは URL のコピーと X への投稿リンクを出す。結果が出るまでは出さない", async () => {
    const user = userEvent.setup();
    const writeText = vi.fn(async () => undefined);
    Object.defineProperty(navigator, "clipboard", { value: { writeText }, configurable: true });
    const { engine } = renderApp();
    await search(user);
    expect(screen.queryByTestId("share")).toBeNull();
    await engine.finish();
    await screen.findAllByTestId("result-row");
    expect(screen.queryByTestId("share-native")).toBeNull();
    const x = screen.getByTestId("share-x");
    expect(x).toHaveAttribute("target", "_blank");
    expect(x).toHaveAttribute("rel", "noopener noreferrer");
    const href = new URL(x.getAttribute("href")!);
    expect(href.origin + href.pathname).toBe("https://x.com/intent/post");
    expect(href.searchParams.get("text")).toBe("リュウ KA 40 のセットプレイ | Tacpendium-Setplay");
    expect(href.searchParams.get("url")).toBe("https://example.test/?c=ryu&ka=40");
    await user.click(screen.getByTestId("share-copy"));
    expect(writeText).toHaveBeenCalledWith("https://example.test/?c=ryu&ka=40");
    expect(await screen.findByText("URL をコピーしました")).toBeInTheDocument();
    // 操作のまとまりに「この条件を共有」の名前が付いている。
    expect(screen.getByRole("group", { name: "この条件を共有:" })).toBe(screen.getByTestId("share"));
    // 条件を変えたら(キャッシュからすぐ出ても)前の条件の「コピーしました」を残さない。
    await search(user, "ryu", "41");
    await engine.finish();
    await waitFor(() => expect(screen.getByTestId("share")).toHaveAttribute("data-share-url", "https://example.test/?c=ryu&ka=41"));
    await user.clear(screen.getByTestId("ka-input"));
    await user.type(screen.getByTestId("ka-input"), "40");
    await user.click(screen.getByTestId("generate"));
    expect(screen.getByTestId("timing")).toHaveTextContent("計算済みの結果");
    await user.click(screen.getByTestId("share-copy"));
    expect(await screen.findByText("URL をコピーしました")).toBeInTheDocument();
    await user.clear(screen.getByTestId("ka-input"));
    await user.type(screen.getByTestId("ka-input"), "41");
    await user.click(screen.getByTestId("generate"));
    expect(screen.getByTestId("share")).toHaveAttribute("data-share-url", "https://example.test/?c=ryu&ka=41");
    expect(screen.getByTestId("share-copy")).toHaveTextContent(/^URL をコピー$/);
  });

  it("結果が 0 件のときは出さない", async () => {
    const user = userEvent.setup();
    const { engine } = renderApp();
    await search(user);
    await engine.finish({ kind: "result", result: { items: [], truncated: false, totalFound: 0 }, timings: { boot: 0, load: 0, compute: 1, wasmMemoryBytes: 0 }, wallMs: 1 });
    expect(await screen.findByTestId("empty")).toBeInTheDocument();
    expect(screen.queryByTestId("share")).toBeNull();
  });

  it("共有の仕組みがある(スマホ)ときは端末の共有メニューを開く。閉じただけなら何も出さず、失敗は出す", async () => {
    const user = userEvent.setup();
    const share = vi.fn(async (_data: ShareData) => Promise.reject(new DOMException("canceled", "AbortError")));
    Object.defineProperty(navigator, "share", { value: share, configurable: true });
    const { engine } = renderApp("", ["en-US"]);
    await search(user, "ryu", "55");
    await engine.finish();
    await screen.findAllByTestId("result-row");
    expect(screen.queryByTestId("share-x")).toBeNull();
    expect(screen.queryByTestId("share-copy")).toBeNull();
    await user.click(screen.getByTestId("share-native"));
    expect(share).toHaveBeenCalledWith({
      title: "Tacpendium-Setplay",
      text: "Ryu KA 55 setplays | Tacpendium-Setplay",
      url: "https://example.test/?c=ryu&ka=55",
    });
    await act(async () => undefined);
    expect(screen.queryByTestId("share-failed")).toBeNull();
    share.mockImplementation(async () => Promise.reject(new Error("NotAllowedError")));
    await user.click(screen.getByTestId("share-native"));
    expect(await screen.findByTestId("share-failed")).toHaveTextContent("Could not share");
  });

  it("共有する URL は画面の条件(既定でない条件も入る)", async () => {
    const { engine } = renderApp("c=ken&ka=30&mode=gap&chain=1");
    await engine.finish();
    await screen.findAllByTestId("result-row");
    expect(screen.getByTestId("share")).toHaveAttribute("data-share-url", "https://example.test/?c=ken&ka=30&mode=gap&chain=1");
  });

  it("ローカル版では出さない(URL が 127.0.0.1 で相手が開けない)", async () => {
    const user = userEvent.setup();
    Object.defineProperty(navigator, "share", { value: vi.fn(), configurable: true });
    const { engine } = renderApp("", ["ja"], fetchFn, "local");
    await search(user);
    await engine.finish();
    await screen.findAllByTestId("result-row");
    expect(screen.queryByTestId("share")).toBeNull();
    expect(screen.queryByTestId("share-native")).toBeNull();
  });
});
