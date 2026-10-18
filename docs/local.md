# Tacpendium-Setplay ローカル版 / Local version

サイトと同じ画面を、**ダウン後の有利フレーム（KA）の上限 100 を外して**手元の PC で開くための配布版です（HANDOVER D10・§5）。
計算はサイトと同じく、ブラウザの中の WebAssembly が行います。サーバ（このプログラム）は画面とデータを `127.0.0.1` で配るだけで、
**外へは通信しません**（実行の計測・グッド/バッドの票は送りません。並びは本家の順です）。

The same app as the website, with **no upper limit on knockdown advantage (KA)**. Everything runs in your browser on your PC;
the program only serves the page on `127.0.0.1` and **sends nothing anywhere** (no usage data, no votes).

## 使い方 / How to use

1. GitHub の [Releases](https://github.com/plexiblinp/tacpendium-setplay/releases) から OS に合う圧縮を落として展開する。
   `SHA256SUMS` で中身を確かめられる（例 `sha256sum -c SHA256SUMS --ignore-missing`）。

   | OS | ファイル |
   |---|---|
   | Windows（64 bit） | `tacpendium-setplay-local_windows_amd64.zip` |
   | macOS（Apple シリコン / Intel） | `tacpendium-setplay-local_darwin_arm64.tar.gz` / `…_darwin_amd64.tar.gz` |
   | Linux（x86-64 / ARM64） | `tacpendium-setplay-local_linux_amd64.tar.gz` / `…_linux_arm64.tar.gz` |

2. 中のプログラムを起動する。既定のブラウザで `http://127.0.0.1:8787/` が開く（8787 が使われていれば空いているポート。表示された URL を開く）。
3. 終わるときは、起動した窓で **Ctrl+C**（Windows は窓を閉じてもよい）。

起動の引数 / Options:

| 引数 | 意味 |
|---|---|
| `-port 8787` | 待ち受けのポート（`0` なら空いているポート。指定したポートが使われていれば誤りで終わる） |
| `-open=false` | ブラウザを開かない |
| `-version` | 版を出して終わる |

### 署名についての注意 / Unsigned binaries

配布物にはコード署名をしていません（費用がかかるため）。初回の起動で OS が警告します。

- **Windows**: 「Windows によって PC が保護されました」→「詳細情報」→「実行」。
- **macOS**: 開けないときは、ターミナルで `xattr -d com.apple.quarantine ./tacpendium-setplay-local_darwin_arm64` を実行してから起動する（または Finder で右クリック →「開く」）。
- **Linux**: `chmod +x` が要ることがある（圧縮の中では実行権が付いている）。

### 重い条件について / Heavy conditions

KA が大きいほど計算に時間がかかります（目安: リュウの重ねる・既定の条件で KA 100 は約 8 秒、KA 200 は約 33 秒。手元の Node での測定）。
計算中は「中断」で止められます。ブラウザのタブ 1 つで 1 GB 近いメモリを使うことがあります。

## 開発者向け / For developers

### ビルド

```
bash scripts/local/build.sh [置き場(既定 build/local)] [GOOS/GOARCH ...]
# 例: bash scripts/local/build.sh build/local linux/amd64 windows/amd64 darwin/arm64
```

1. `web/scripts/prepare-assets.sh` でデータ（`cmd/precompute`）と WASM（`cmd/wasm`）を `web/public/` に作る（`SKIP_ASSETS=1` なら今あるものを使う）。
2. フロントを `vite build --mode standalone`（`web/src/lib/flavor.ts`。Vite は mode 名 `local` を `.env.local` と紛れるため受け付けない）で `web/dist-local/` に作る。
3. それを `cmd/local/site/` に置き、`go:embed` で埋め込んで `go build`（`CGO_ENABLED=0`・`-trimpath`・版は `-X main.version`）。

`cmd/local/site/` はプレースホルダ（`.gitkeep`）だけをコミットしている。ビルドの手順を飛ばしたバイナリは起動時に理由を出して終わる。

### サイトとの違い（`web/src/lib/flavor.ts` の `Flavor`）

| | サイト（`site`） | ローカル版（`local`） |
|---|---|---|
| KA の上限 | 100（`KA_MAX_SITE`） | エンジンが受ける最大 2147483647（`KA_MAX_LOCAL`。アプリでは数で切らない） |
| 計測（beacon・`config.json`） | 送る | 送らない・読まない |
| 票（グッド/バッド） | 読む・送る・ボタンを出す | 使わない（並びは本家の順。ボタン・案内を出さない） |
| フッター | プライバシーポリシー | 「通信しない」の注記 |
| KA の説明 | 100 を超えるならローカル版（Releases）への案内 | 上限なしの説明・見出しに「ローカル版」 |

配信（`internal/localserver`）:

- `127.0.0.1` だけで待ち受ける。`Host` が `127.0.0.1` / `localhost` / `[::1]` と同じポートでなければ 403（DNS rebinding で別の名前から読まれないように）。
- サイトと同じ `_headers`（`web/public/_headers`）を同じ規則（`web/scripts/headers.mjs`）で読み、同じヘッダを付ける（CSP の下で WASM・Worker が動くこと）。`Strict-Transport-Security` も付くが、HTTP の応答ではブラウザが無視する（RFC 6797 §8.1）。
- Content-Type は固定の表（Windows の registry の MIME で `.js` が `text/plain` になる環境がある）。`_headers`・ドットで始まる名前・ディレクトリは配らない。GET / HEAD のみ。

### 検査

- 単体: `go test ./cmd/local/ ./internal/localserver/`、`web/` の vitest（`flavor.test.ts`・`App.test.tsx` の「ローカル版」）。
- E2E: Playwright の project `local`（`web/e2e/local.spec.ts`）。実バイナリ（`build/local/tacpendium-setplay-local_<os>_<arch>`。`LOCAL_BIN` で差し替え）を起動して通す。CI の Web job が回す。
- 起動確認（3 OS）: `.github/workflows/local-release.yml` の smoke（`scripts/local/smoke.sh`）。ubuntu（linux/amd64）・macos（darwin/arm64）・windows（windows/amd64）で、配る圧縮を展開して起動し、HTTP で画面・データ・WASM・ヘッダを確かめる。linux/arm64 と darwin/amd64 はビルドだけ（起動は確かめていない）。
- govulncheck: `scripts/govulncheck.sh`（ローカル版は windows・darwin・linux の 3 つで見る）。

### 配布（Releases）

1. main で `v<版>` のタグを push する（開発者。例 `git tag v1.0.0 && git push origin v1.0.0`）。
2. `local-release.yml` がビルド → 3 OS の起動確認 → **draft** の Release（5 種の圧縮と `SHA256SUMS`）を作る。
3. 開発者が GitHub の画面で中身を確かめて公開する。

手元で作ったバイナリは配らない（公開物は CI からだけ出す＝AGENTS §8-12）。
