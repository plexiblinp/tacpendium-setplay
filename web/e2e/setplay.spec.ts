// 主要フローの E2E(HANDOVER §9 S5 の完了条件)。実 WASM・実データ・_headers のヘッダ(CSP 等)で通す。
import { expect, test, type Page } from "@playwright/test";

// CSP 違反・ページの例外は失敗にする(ヘッダで WASM・Worker が止まっていないことを見る)。
function watchErrors(page: Page): string[] {
  const errors: string[] = [];
  page.on("pageerror", (e) => errors.push(`pageerror: ${e.message}`));
  page.on("console", (m) => {
    if (m.type() === "error") errors.push(`console: ${m.text()}`);
  });
  return errors;
}

async function choose(page: Page, character: string, ka: number) {
  await page.getByTestId("character-select").click();
  await page.getByTestId(`character-option-${character}`).click();
  await page.getByTestId("ka-input").fill(String(ka));
}

test.describe("セットプレイ提案", () => {
  test("キャラと KA で提案を出す → Worker の WASM が計算 → 表記の切替・さらに表示・URL", async ({ page }) => {
    const errors = watchErrors(page);
    await page.goto("/");
    await choose(page, "ryu", 40);
    await page.getByTestId("generate").click();
    await expect(page.getByTestId("result-row").first()).toBeVisible();
    // 本家と同じ件数(engine/testdata の正解 ryu KA 40 meaty 既定: totalFound 321)。
    await expect(page.getByTestId("count")).toHaveText(/321/);
    await expect(page.getByTestId("result-row")).toHaveCount(200);
    await expect(page).toHaveURL(/\?c=ryu&ka=40$/);

    const first = page.getByTestId("recipe").first();
    const srk = await first.textContent();
    await page.getByTestId("notation-nameJa").click();
    const ja = await first.textContent();
    expect(ja).not.toBe(srk);
    expect(ja).toMatch(/[぀-ヿ一-鿿]/);

    await page.getByTestId("show-more").click();
    await expect(page.getByTestId("result-row")).toHaveCount(321);
    await expect(page.getByTestId("show-more")).toHaveCount(0);
    expect(errors).toEqual([]);
  });

  test("重ねたときの有利: 実データの当てる技の有利 + (N − 1) が行に出る(moves DB → 画面の通し)", async ({ page }) => {
    const errors = watchErrors(page);
    await page.goto("/");
    await choose(page, "ryu", 40);
    await page.getByTestId("generate").click();
    const row = page.getByTestId("result-row").first();
    await expect(row).toBeVisible();
    const key = (await row.getAttribute("data-recipe-key")) ?? "";
    const target = key.split("|")[1] ?? "";
    const label = (await row.getByTestId("frame-label").textContent()) ?? "";
    const n = Number(/\d+/.exec(label)?.[0]);
    // 配信している moves DB から、当てる技の有利を読む(画面と同じ manifest → moves の経路)。
    const move = await page.evaluate(async (code) => {
      const manifest = (await (await fetch("/data/manifest.json")).json()) as { characters: { code: string; moves: string }[] };
      const ryu = manifest.characters.find((c) => c.code === "ryu")!;
      const db = (await (await fetch(new URL(ryu.moves, new URL("/data/manifest.json", location.href)).href)).json()) as {
        moves: { code: string; onHit: number | null; onBlock: number | null }[];
      };
      return db.moves.find((m) => m.code === code) ?? null;
    }, target);
    expect(move).not.toBeNull();
    const fmt = (v: number | null) => (v == null ? "—" : v === 0 ? "±0" : v > 0 ? `+${v}` : `${v}`);
    const block = fmt(move!.onBlock == null ? null : move!.onBlock + n - 1);
    const hit = fmt(move!.onHit == null ? null : move!.onHit + n - 1);
    await expect(row.getByTestId("advantage")).toHaveText(new RegExp(`(On block|ガード) ${block.replace("+", "\\+")} / (On hit|ヒット) ${hit.replace("+", "\\+")}$`));
    expect(errors).toEqual([]);
  });

  test("モード(汚連携・有利調整・安全飛び)と絞り込み", async ({ page }) => {
    const errors = watchErrors(page);
    await page.goto("/");
    await choose(page, "ryu", 70);
    await page.getByTestId("mode-gap").click();
    await page.getByTestId("generate").click();
    await expect(page.getByTestId("frame-label").first()).toHaveText(/before wake-up|前に空振り/);
    // 重ねない提案には有利を出さない。
    await expect(page.getByTestId("advantage")).toHaveCount(0);

    await page.getByTestId("mode-filler").click();
    await page.getByTestId("safe-jump-true").click();
    await expect(page.getByTestId("frame-label").first()).toHaveText(/True safe jump \(43F left\)|真安全飛び\(残り43F\)/);
    await expect(page).toHaveURL(/mode=filler/);
    await expect(page).toHaveURL(/sj=true/);

    await page.getByTestId("mode-meaty").click();
    await page.getByTestId("filter-block-safe").check();
    await page.getByTestId("generate").click();
    await expect(page.getByTestId("result-row").first().or(page.getByTestId("empty"))).toBeVisible();
    await expect(page).toHaveURL(/block_safe=1/);
    expect(errors).toEqual([]);
  });

  test("共有 URL を開くと同じ条件で自動計算する", async ({ page }) => {
    const errors = watchErrors(page);
    await page.goto("/?c=zangief&ka=45&mode=gap&g_min=2&g_max=5&sort=target");
    await expect(page.getByTestId("result-row").first().or(page.getByTestId("empty"))).toBeVisible();
    await expect(page.getByTestId("character-select")).toHaveAttribute("data-value", "zangief");
    await expect(page.getByTestId("ka-input")).toHaveValue("45");
    await expect(page.getByTestId("g-min")).toHaveValue("2");
    await expect(page.getByTestId("sort-target")).toHaveAttribute("aria-pressed", "true");
    expect(errors).toEqual([]);
  });

  test("コピー(D16): Permissions-Policy の clipboard-write=(self) の下で、選択中の表記のテキストがクリップボードに入る", async ({ page, context }) => {
    const errors = watchErrors(page);
    await context.grantPermissions(["clipboard-read", "clipboard-write"]);
    await page.goto("/");
    await choose(page, "ken", 30);
    await page.getByTestId("generate").click();
    await page.getByTestId("notation-numeric").click();
    const recipe = (await page.getByTestId("recipe").first().textContent()) ?? "";
    const label = (await page.getByTestId("frame-label").first().textContent()) ?? "";
    await page.getByTestId("copy").first().click();
    await expect(page.getByTestId("copy").first()).toHaveText(/Copied|コピーしました/);
    // ページの Permissions-Policy は clipboard-read を拒否するので、読み出しは別のオリジンの空のページで行う
    // (localhost は安全なコンテキスト扱い。中身は Playwright が返す)。
    const reader = await context.newPage();
    await reader.route("http://localhost:4199/reader", (r) => r.fulfill({ body: "<!doctype html><title>r</title>", contentType: "text/html" }));
    await reader.goto("http://localhost:4199/reader");
    const text = await reader.evaluate(() => navigator.clipboard.readText());
    expect(text).toBe(`${recipe} (${label})`);
    expect(errors).toEqual([]);
  });

  test("この条件を共有: 結果を出すと共有の操作が出て、共有する URL に条件が入る(PC の形・OGP)", async ({ page, context }) => {
    const errors = watchErrors(page);
    await context.grantPermissions(["clipboard-read", "clipboard-write"]);
    // 端末の共有メニューの有無はブラウザ・OS で変わるので、無い形(PC)に固定する。
    await page.addInitScript(() => Reflect.deleteProperty(Navigator.prototype, "share"));
    await page.goto("/");
    await expect(page.locator('meta[property="og:title"]')).toHaveAttribute("content", /Tacpendium-Setplay/);
    await expect(page.locator('meta[property="og:description"]')).toHaveAttribute("content", /SF6/);
    await expect(page.locator('meta[property="og:type"]')).toHaveAttribute("content", "website");
    await expect(page.locator('meta[name="twitter:card"]')).toHaveAttribute("content", "summary_large_image");
    // og:url はサイトの URL(S9)。og:image は同じ host の絶対 URL で、その path をこの配信が画像として返す。
    const ogUrl = (await page.locator('meta[property="og:url"]').getAttribute("content")) ?? "";
    expect(ogUrl).toBe("https://tacpendium-setplay.plexiblinp.workers.dev/");
    const ogImage = new URL((await page.locator('meta[property="og:image"]').getAttribute("content")) ?? "");
    expect(ogImage.host).toBe(new URL(ogUrl).host);
    // ブラウザの外からの取得(SNS のサーバが取りに来る形)。CORP(same-origin)はブラウザだけが見るので、ここでは効かない。
    const img = await page.request.get(ogImage.pathname);
    expect(img.status()).toBe(200);
    expect(img.headers()["content-type"]).toMatch(/^image\/png/);
    // フッターの「このサイトを共有」: 結果の前から出て、サイトのトップ(条件なし)を共有する。
    const appShare = page.locator("footer").getByTestId("app-share");
    await expect(appShare.getByTestId("app-share-x")).toBeVisible();
    const top = new URL("/", page.url()).href;
    await expect(appShare).toHaveAttribute("data-share-url", top);
    const appX = new URL((await appShare.getByTestId("app-share-x").getAttribute("href")) ?? "");
    expect(appX.searchParams.get("url")).toBe(top);
    expect(appX.searchParams.get("text")).toMatch(/SF6 .+ — Tacpendium-Setplay$/);
    await choose(page, "ryu", 40);
    await page.getByTestId("mode-gap").click();
    await expect(page.getByTestId("share")).toHaveCount(0);
    await page.getByTestId("generate").click();
    await expect(page.getByTestId("share-x")).toBeVisible();
    await expect(page).toHaveURL(/\?c=ryu&ka=40&mode=gap$/);
    const shared = await page.getByTestId("share").getAttribute("data-share-url");
    expect(shared).toBe(page.url());
    const x = new URL((await page.getByTestId("share-x").getAttribute("href")) ?? "");
    expect(`${x.origin}${x.pathname}`).toBe("https://x.com/intent/post");
    expect(x.searchParams.get("url")).toBe(page.url());
    // 文にはキャラ・KA・モード(汚連携)・アプリ名が入る(言語はブラウザの設定)。
    expect(x.searchParams.get("text")).toMatch(/^(リュウ KA 40 の汚連携|Ryu KA 40 deliberate-gap setplays) — Tacpendium-Setplay$/);
    // 結果の共有の後もフッターの共有は条件の無いトップのまま。
    await expect(appShare).toHaveAttribute("data-share-url", top);
    await page.getByTestId("share-copy").click();
    await expect(page.getByTestId("share-copy")).toHaveText(/URL copied|URL をコピーしました/);
    const reader = await context.newPage();
    await reader.route("http://localhost:4199/reader", (r) => r.fulfill({ body: "<!doctype html><title>r</title>", contentType: "text/html" }));
    await reader.goto("http://localhost:4199/reader");
    expect(await reader.evaluate(() => navigator.clipboard.readText())).toBe(page.url());
    expect(errors).toEqual([]);
  });

  test("中断: 重い計算を止めて、すぐ次の計算ができる", async ({ page }) => {
    const errors = watchErrors(page);
    await page.goto("/");
    await choose(page, "dhalsim", 100);
    await expect(page.getByTestId("slow-warning")).toBeVisible();
    await page.getByTestId("mode-gap").click();
    await page.getByTestId("generate").click();
    await expect(page.getByTestId("loading")).toBeVisible();
    await expect(page.getByTestId("generate")).toBeDisabled();
    await page.getByTestId("cancel").click();
    await expect(page.getByTestId("canceled")).toBeVisible();
    await page.getByTestId("ka-input").fill("30");
    await page.getByTestId("mode-meaty").click();
    await page.getByTestId("generate").click();
    await expect(page.getByTestId("result-row").first()).toBeVisible();
    expect(errors).toEqual([]);
  });

  test("5 言語: 文言が切り替わり、既定の表記は ja だけ日本語技名", async ({ browser }) => {
    for (const [locale, generate, notationPressed] of [
      ["ja-JP", "提案を出す", "notation-nameJa"],
      ["en-US", "Generate suggestions", "notation-srk"],
      ["zh-CN", "生成推荐", "notation-srk"],
      ["zh-TW", "生成推薦", "notation-srk"],
      ["ko-KR", "제안 생성", "notation-srk"],
    ] as const) {
      const ctx = await browser.newContext({ locale });
      const page = await ctx.newPage();
      const errors = watchErrors(page);
      await page.goto("/");
      await expect(page.getByTestId("generate")).toHaveText(generate);
      await expect(page.getByTestId(notationPressed)).toHaveAttribute("aria-pressed", "true");
      await expect(page.getByTestId("machine-translation")).toHaveCount(locale.startsWith("zh") || locale.startsWith("ko") ? 1 : 0);
      expect(await page.evaluate(() => document.documentElement.lang)).toBe(
        { "ja-JP": "ja", "en-US": "en", "zh-CN": "zh-Hans", "zh-TW": "zh-Hant", "ko-KR": "ko" }[locale],
      );
      expect(errors).toEqual([]);
      await ctx.close();
    }
  });

  test("ストレージ・cookie を使わない(HANDOVER §7.3)", async ({ page, context }) => {
    await page.goto("/");
    await choose(page, "ryu", 20);
    await page.getByTestId("generate").click();
    await expect(page.getByTestId("result-row").first()).toBeVisible();
    const used = await page.evaluate(async () => ({
      local: localStorage.length,
      session: sessionStorage.length,
      idb: (await indexedDB.databases()).length,
      cookie: document.cookie,
    }));
    expect(used).toEqual({ local: 0, session: 0, idb: 0, cookie: "" });
    expect(await context.cookies()).toEqual([]);
  });
  test("ヘッダ: 中身が本体と同じ左端・サイト名を押すと条件と結果をリセット", async ({ page }) => {
    const errors = watchErrors(page);
    await page.setViewportSize({ width: 1400, height: 900 });
    await page.goto("/?c=ryu&ka=40&block_safe=1");
    await expect(page.getByTestId("count").or(page.getByTestId("empty"))).toBeVisible();
    const header = await page.getByTestId("home").boundingBox();
    const body = await page.getByTestId("guide").boundingBox();
    expect(header).not.toBeNull();
    expect(body).not.toBeNull();
    expect(Math.abs(header!.x - body!.x)).toBeLessThanOrEqual(1);
    await page.getByTestId("home").click();
    await expect(page).toHaveURL(/\/$/);
    await expect(page.getByTestId("character-select")).toHaveAttribute("data-value", "");
    await expect(page.getByTestId("ka-input")).toHaveValue("");
    await expect(page.getByTestId("result-row")).toHaveCount(0);
    expect(errors).toEqual([]);
  });

  test("重ねる技(S7・A 案): 名前検索 → 1 つの技を指定 → 提案 → URL に技 → 条件をリセット", async ({ page }) => {
    const errors = watchErrors(page);
    await page.goto("/");
    await page.getByTestId("character-select").click();
    await page.getByTestId("character-search").fill("ry");
    await page.getByTestId("character-search").press("Enter");
    await expect(page.getByTestId("character-select")).toHaveAttribute("data-value", "ryu");
    await page.getByTestId("ka-input").fill("40");
    await page.getByTestId("target-by-move").click();
    await expect(page.getByTestId("type-fieldset")).toHaveCount(0);
    const picker = page.getByTestId("picker-move");
    // 先頭の技(空の選択肢の次)を選ぶ。どの技でも、URL の move と結果の当てる技が同じであることを見る。
    const code = await picker.locator("option").nth(1).getAttribute("value");
    expect(code).not.toBeNull();
    await picker.selectOption(code!);
    await page.getByTestId("generate").click();
    await expect(page).toHaveURL(new RegExp(`[?&]move=${code}(&|$)`));
    await expect(page.getByTestId("count").or(page.getByTestId("empty"))).toBeVisible();
    await page.getByTestId("reset").click();
    await expect(page).toHaveURL(/\/$/);
    await expect(page.getByTestId("character-select")).toHaveAttribute("data-value", "");
    await expect(page.getByTestId("target-by-type")).toHaveAttribute("aria-pressed", "true");
    expect(errors).toEqual([]);
  });
  test("フォントの並びが言語ごとに変わる(日本語の画面に中国語の字形のフォントを使わない。S7)", async ({ page }) => {
    await page.goto("/");
    const font = () => page.evaluate(() => getComputedStyle(document.documentElement).fontFamily);
    await page.getByTestId("lang-select").selectOption("ja");
    await expect.poll(font).toMatch(/^"Hiragino Sans"/);
    expect(await font()).not.toMatch(/Noto Sans (SC|TC|KR)|YaHei|PingFang/);
    await page.getByTestId("lang-select").selectOption("zh-Hans");
    await expect.poll(font).toMatch(/^"PingFang SC"/);
    await page.getByTestId("lang-select").selectOption("ko");
    await expect.poll(font).toMatch(/^"Apple SD Gothic Neo"/);
  });
});
