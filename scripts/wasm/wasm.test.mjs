// Go → WASM → JS の配線の通し検査(AGENTS §8-5)。依存なし(node:test)。
//
// 使い方: bash scripts/wasm/build.sh && node --test scripts/wasm/wasm.test.mjs
//   (置き場を変えたときは WASM_DIR=<置き場> を渡す)
//
// scripts/wasm/build.sh が作った setplay.wasm を Go 付属の wasm_exec.js で起動し、JS から
// globalThis.tacpendiumSetplay.load / suggest を呼ぶ。結果を本家の実装で作った正解
// (engine/testdata/upstream_oracle.json)と、Go のテストと同じ正準形の sha256 で突き合わせる。
// ★ここで突き合わせるのは一部のキャラだけ(WASM_ORACLE_CHARACTERS)。全キャラは Go のテスト
//   (engine/conformance_test.go)がネイティブで見る。ここの目的は配線(JSON の受け渡し・引数・誤りの形)である。

import { test } from "node:test";
import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import fs from "node:fs";
import path from "node:path";
import { createRequire } from "node:module";
import { fileURLToPath } from "node:url";

const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../..");
const WASM_DIR = process.env.WASM_DIR ? path.resolve(process.env.WASM_DIR) : path.join(ROOT, "build/wasm");
const MOVES_DIR = path.join(ROOT, "data/moves");
const ORACLE = path.join(ROOT, "engine/testdata/upstream_oracle.json");
const WASM_ORACLE_CHARACTERS = ["ryu", "zangief", "dhalsim"];
const GLOBAL_NAME = "tacpendiumSetplay";
const STATUS_OK = 200;

// wasm_exec.js はファイルの読み書きに globalThis.fs を使う(無いと Go の標準出力への書き込みが失敗する)。
globalThis.fs ??= fs;
createRequire(import.meta.url)(path.join(WASM_DIR, "wasm_exec.js"));

async function startEngine() {
  const go = new globalThis.Go();
  const bytes = fs.readFileSync(path.join(WASM_DIR, "setplay.wasm"));
  const { instance } = await WebAssembly.instantiate(bytes, go.importObject);
  // main は関数を登録した後 select{} で待つので、run の Promise は解決しない(待たない)。
  void go.run(instance);
  const api = globalThis[GLOBAL_NAME];
  assert.ok(api, `globalThis.${GLOBAL_NAME} が無い(cmd/wasm の登録)`);
  return api;
}

// engine/conformance_test.go の canonicalItems と同じ書式。
function canonicalItems(items) {
  let s = "";
  for (const it of items) {
    s += `${it.mode}|${it.s}|${it.n}|${it.landing}|${it.targetActive}|${it.g}|${it.remain}|`;
    s += it.steps.map((st) => `${st.code}:${st.role}:${st.counted}`).join(",");
    s += "\n";
  }
  return createHash("sha256").update(s, "utf8").digest("hex");
}

// 条件のクエリの {safe_jump.true|f5|f6} を、そのキャラの安全飛びの値(moves DB の safeJump)に置き換える
// (engine/conformance_test.go の resolveSafeJump と同じ)。
function resolveSafeJump(character, query) {
  if (!query.includes("{safe_jump.")) return query;
  const sj = JSON.parse(fs.readFileSync(path.join(MOVES_DIR, `${character}.json`), "utf8")).safeJump;
  assert.ok(sj, `${character}: 安全飛びの値が無い`);
  return query
    .replaceAll("{safe_jump.true}", String(sj.true))
    .replaceAll("{safe_jump.f5}", String(sj.f5))
    .replaceAll("{safe_jump.f6}", String(sj.f6));
}

const api = await startEngine();

test("load: moves DB を読み、キャラの code を返す", () => {
  for (const c of WASM_ORACLE_CHARACTERS) {
    const r = JSON.parse(api.load(fs.readFileSync(path.join(MOVES_DIR, `${c}.json`), "utf8")));
    assert.deepEqual(r, { character: c });
  }
});

test("load: 壊れたデータは invalid_moves_data(例外にしない)", () => {
  const r = JSON.parse(api.load('{"schemaVersion":99}'));
  assert.equal(r.error.code, "invalid_moves_data");
});

test("引数の数・型の誤りは invalid_arguments", () => {
  assert.equal(JSON.parse(api.load()).error.code, "invalid_arguments");
  assert.equal(JSON.parse(api.suggest("ryu", "40", "")).error.code, "invalid_arguments");
  assert.equal(JSON.parse(api.suggest("ryu", 40)).error.code, "invalid_arguments");
  // KA は整数だけ(小数・NaN・Infinity を黙って丸めると「該当なし」と区別できない)。
  for (const ka of [40.9, -0.5, NaN, Infinity]) {
    assert.equal(JSON.parse(api.suggest("ryu", ka, "")).error.code, "invalid_arguments", `ka=${ka}`);
  }
});

test("suggest: 読んでいないキャラ・クエリの誤り", () => {
  assert.equal(JSON.parse(api.suggest("no_such_char", 40, "")).error.code, "character_not_loaded");
  const r = JSON.parse(api.suggest("ryu", 40, "mode=xxx"));
  assert.equal(r.error.code, "invalid_query_parameter");
  assert.equal(r.error.param, "mode");
});

// 本家の正解(WASM_ORACLE_CHARACTERS の分)を suggest で回し、[突き合わせた件数, 不一致] を返す。
function compareWithOracle() {
  const oracle = JSON.parse(fs.readFileSync(ORACLE, "utf8"));
  let checked = 0;
  const mismatches = [];
  for (const cell of oracle.cells) {
    if (!WASM_ORACLE_CHARACTERS.includes(cell.character)) continue;
    const r = JSON.parse(api.suggest(cell.character, cell.ka, resolveSafeJump(cell.character, cell.query)));
    const got = { status: r.error ? 400 : STATUS_OK };
    if (r.error) {
      got.error = `${r.error.code}: ${r.error.message}`;
    } else {
      const res = r.result;
      Object.assign(got, {
        totalFound: res.totalFound,
        truncated: res.truncated,
        reason: res.reason ?? "",
        items: res.items.length,
        sha256: canonicalItems(res.items),
      });
    }
    const want =
      cell.status === STATUS_OK
        ? {
            status: cell.status,
            totalFound: cell.totalFound,
            truncated: cell.truncated,
            reason: cell.reason ?? "",
            items: cell.items,
            sha256: cell.sha256,
          }
        : { status: cell.status, error: cell.error };
    checked++;
    try {
      assert.deepEqual(got, want);
    } catch {
      mismatches.push({ cell: `${cell.character} KA ${cell.ka} ${JSON.stringify(cell.query)}`, want, got });
    }
  }
  return [checked, mismatches];
}

test("suggest: 本家の正解と一致する(WASM 経由)", () => {
  const [checked, mismatches] = compareWithOracle();
  assert.ok(checked > 0, "突き合わせた条件が 0 件");
  assert.deepEqual(mismatches.slice(0, 5), [], `不一致 ${mismatches.length} / ${checked}`);
  console.log(`WASM 経由で突き合わせた条件: ${checked}(不一致 ${mismatches.length})`);
});

// 票(S6・HANDOVER D17)。4 番目の引数に {"<recipeKey>": good − bad} の JSON 文字列を渡すと、票が第一キーになる
// (並べ替えの後・limit で切る前)。省略・空文字は票なし(本家と同じ並び)。
test("suggest: 票(4 番目の引数)が第一キーに効く・票なしは本家と同じ", () => {
  const base = JSON.parse(api.suggest("ryu", 43, "limit=3000"));
  assert.equal(api.suggest("ryu", 43, "limit=3000", ""), JSON.stringify(base));
  const items = base.result.items;
  assert.ok(items.length > 200, `件数 ${items.length}`);
  for (const it of items) assert.match(it.recipeKey, /\|/);
  const far = items[items.length - 1].recipeKey;
  const voted = JSON.parse(api.suggest("ryu", 43, "limit=200", JSON.stringify({ [far]: 1 })));
  assert.equal(voted.result.items[0].recipeKey, far);
  assert.equal(voted.result.items.length, 200);
  assert.equal(JSON.parse(api.suggest("ryu", 43, "", 5)).error.code, "invalid_arguments");
  assert.equal(JSON.parse(api.suggest("ryu", 43, "", '{"a":"x"}')).error.code, "invalid_arguments");
});
