// dist を _headers どおりのヘッダで配る静的サーバ(E2E 用。本番の配信は Cloudflare)。
// 使い方: node scripts/serve.mjs [dist] [port]
// /api/* は Worker の本物のコード(edge/。node scripts/build-edge.mjs が束ねた dist-edge/local.mjs)へ渡す。
// D1 は node:sqlite、Analytics Engine はメモリの偽物。E2E が確かめるための口 /__e2e/points・/__e2e/votes を足す
// (本番の Worker には無い。dist-edge が無ければ /api は 404 で、画面は票なし・計測なしで動く)。
import fs from "node:fs";
import http from "node:http";
import path from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

import { headersFor, parseHeaders } from "./headers.mjs";

const localApiPath = path.join(path.dirname(fileURLToPath(import.meta.url)), "..", "dist-edge", "local.mjs");

async function toRequest(req) {
  const chunks = [];
  for await (const c of req) chunks.push(c);
  const headers = new Headers();
  for (const [k, v] of Object.entries(req.headers)) if (typeof v === "string") headers.set(k, v);
  const body = chunks.length > 0 && req.method !== "GET" && req.method !== "HEAD" ? Buffer.concat(chunks) : undefined;
  return new Request(`http://${req.headers.host}${req.url}`, { method: req.method, headers, body });
}

async function sendResponse(res, r) {
  const headers = {};
  r.headers.forEach((v, k) => (headers[k] = v));
  res.writeHead(r.status, headers);
  res.end(Buffer.from(await r.arrayBuffer()));
}

const root = path.resolve(process.argv[2] ?? "dist");
const port = Number(process.argv[3] ?? 4173);

/** Worker の env.ASSETS の代わり(票の受け口が manifest・moves DB を読む)。dist のファイルをそのまま返す。 */
const assets = {
  fetch: async (request) => {
    const file = path.join(root, decodeURIComponent(new URL(request.url).pathname));
    if (!file.startsWith(root + path.sep) || !fs.existsSync(file) || !fs.statSync(file).isFile()) return new Response("not found", { status: 404 });
    return new Response(fs.readFileSync(file), { status: 200 });
  },
};
const api = fs.existsSync(localApiPath) ? (await import(pathToFileURL(localApiPath).href)).createLocalApi(assets) : null;
const rules = parseHeaders(fs.readFileSync(path.join(root, "_headers"), "utf8"));

const TYPES = {
  ".html": "text/html; charset=utf-8",
  ".js": "text/javascript; charset=utf-8",
  ".css": "text/css; charset=utf-8",
  ".json": "application/json; charset=utf-8",
  ".wasm": "application/wasm",
  ".bin": "application/octet-stream",
  ".svg": "image/svg+xml",
  ".png": "image/png",
};

http
  .createServer(async (req, res) => {
    const url = new URL(req.url ?? "/", "http://localhost");
    if (api != null && url.pathname.startsWith("/api/")) {
      await sendResponse(res, await api.handle(await toRequest(req)));
      return;
    }
    if (api != null && (url.pathname === "/__e2e/points" || url.pathname === "/__e2e/votes")) {
      res.writeHead(200, { "content-type": "application/json; charset=utf-8" });
      res.end(JSON.stringify(url.pathname === "/__e2e/points" ? api.points() : api.votes()));
      return;
    }
    let pathname = decodeURIComponent(url.pathname);
    if (pathname.endsWith("/")) pathname += "index.html";
    const file = path.join(root, pathname);
    // root の外・_headers 自体は配らない。
    if (!file.startsWith(root + path.sep) || path.basename(file) === "_headers" || !fs.existsSync(file) || !fs.statSync(file).isFile()) {
      res.writeHead(404, { "content-type": "text/plain; charset=utf-8" });
      res.end("not found");
      return;
    }
    const headers = { "content-type": TYPES[path.extname(file)] ?? "application/octet-stream" };
    for (const [k, v] of headersFor(rules, url.pathname)) headers[k] = v;
    res.writeHead(200, headers);
    fs.createReadStream(file).pipe(res);
  })
  .listen(port, "127.0.0.1", () => console.log(`serving ${root} on http://127.0.0.1:${port}${api ? " (+ /api: dist-edge/local.mjs)" : ""}`));
