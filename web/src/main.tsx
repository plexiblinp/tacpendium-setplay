import { StrictMode } from "react";
import { createRoot } from "react-dom/client";

import { App } from "./components/App";
import { EngineClient } from "./lib/engineClient";
import { detectLang, initI18n } from "./lib/i18n";
import "./styles.css";

const languages = navigator.languages.length > 0 ? navigator.languages : [navigator.language];
initI18n(detectLang(languages));

const engine = new EngineClient(
  () => new Worker(new URL("./worker/setplay.worker.ts", import.meta.url), { type: "module" }),
);

const root = document.getElementById("root");
if (root == null) throw new Error("#root が無い");

// 描画中の例外で画面ごと消えたら、理由を出す(関数コンポーネントだけで書くため、React 19 の onUncaughtError を使う)。
function showFatal(error: unknown) {
  const el = document.getElementById("root");
  if (el == null) return;
  const box = document.createElement("div");
  box.setAttribute("role", "alert");
  const head = document.createElement("strong");
  head.textContent = "画面を表示できませんでした / The page failed to render";
  const body = document.createElement("pre");
  body.textContent = error instanceof Error ? (error.stack ?? error.message) : String(error);
  box.append(head, body);
  el.replaceChildren(box);
}

createRoot(root, { onUncaughtError: (error) => showFatal(error) }).render(
  <StrictMode>
    <App
      engine={engine}
      fetchFn={(url, init) => fetch(url, init)}
      location={{ href: window.location.href, search: window.location.search }}
      replaceSearch={(search) => {
        const url = `${window.location.pathname}${search ? `?${search}` : ""}`;
        window.history.replaceState(null, "", url);
      }}
      languages={languages}
      onLangChange={() => undefined}
      // 実行計測(HANDOVER §7.1)。ページを閉じる途中でも届く sendBeacon。無いブラウザでは fetch の keepalive。
      sendBeacon={(url, body) =>
        typeof navigator.sendBeacon === "function"
          ? navigator.sendBeacon(url, body)
          : (void fetch(url, { method: "POST", body, keepalive: true }).catch(() => undefined), true)
      }
    />
  </StrictMode>,
);
