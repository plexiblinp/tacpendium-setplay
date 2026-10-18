// 画面(React)が動き出す前の誤りを画面に出す(真っ白にしない)。依存なし・CSP の script-src 'self' で読める普通のスクリプト。
// React が描画を始めると #boot-status は消えるので、ここで書くのは「描画の前に止まった」ときだけ。
(function () {
  function show(message) {
    var el = document.getElementById("boot-status");
    if (!el) return;
    el.textContent = "";
    var head = document.createElement("strong");
    head.textContent = "画面を表示できませんでした / The page failed to start";
    var body = document.createElement("pre");
    body.textContent = String(message);
    el.appendChild(head);
    el.appendChild(body);
  }
  // 第 3 引数 true(捕捉の段階)にすると、スクリプトの読み込みの失敗(泡立たない)も拾える。
  window.addEventListener(
    "error",
    function (e) {
      var t = e.target;
      if (t && t !== window && (t.src || t.href)) {
        show("読み込めなかった / failed to load: " + (t.src || t.href));
        return;
      }
      show(e.error && e.error.stack ? e.error.stack : e.message || "script error");
    },
    true,
  );
  window.addEventListener("unhandledrejection", function (e) {
    var r = e.reason;
    show(r && r.stack ? r.stack : r);
  });
})();
