// src/locales/zh-Hans.json から src/locales/zh-Hant.json を作る(生成物はコミットし、試験が再生成と一致を見る)。
// 使い方: pnpm gen:locales   (zh-Hans.json を直したら回してコミットする。zh-Hant.json は手で直さない)
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

import { renderLocale, toTraditional } from "./zh-hant.mjs";

const dir = path.join(path.dirname(fileURLToPath(import.meta.url)), "../src/locales");
const hans = JSON.parse(fs.readFileSync(path.join(dir, "zh-Hans.json"), "utf8"));
fs.writeFileSync(path.join(dir, "zh-Hant.json"), renderLocale(toTraditional(hans)));
console.log("zh-Hant.json を書いた");
