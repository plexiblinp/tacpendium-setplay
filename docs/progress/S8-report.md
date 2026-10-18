# S8 ローカル版 — 完了報告

- 段階開始コミット: `c6d5ded15d9784c860425ed41dcf7c77da3193bf`
- ブランチ: `claude/optimistic-faraday-0kbp3t`

## 計画

### 完了条件（HANDOVER §9 S8）
「ローカル版(KA 無制限・単一バイナリ・Releases)」— **3 OS で起動確認**。
加えて開発者の追加指示（2026-10-18）:
- govulncheck の常設: CI の Go job に step を 1 つ（job は増やさない）。対象は `./...`・WASM（`GOOS=js GOARCH=wasm ./cmd/wasm/`）・ローカル版。`golang.org/x/vuln/cmd/govulncheck` は版を固定。週 1 回＋手動起動で govulncheck だけを回す小さな workflow。検出したら赤。わざと起こして赤を確かめる（AGENTS §8-3）。
- Cloudflare のデプロイ・ヘッダ診断・S9 の報告・チェックリスト §5 の「済」付けは S9（S8 ではやらない）。
- Cloudflare のデプロイ後に GitHub リポジトリの About の Website に URL を入れることを、S9 の手番として書いておく。

### 前提（読んで分かったこと）
- 計算は既にブラウザ内の WASM（S5 の計算方式 C）。サイトとローカル版の違いは「KA の上限」と「通信（計測 beacon・票）」だけ。
- KA の上限 100 は `web/src/lib/constants.ts` の `KA_MAX`（URL の検証・欄の検査・入力欄の max・文言）と、beacon の形（`web/src/shared/api.ts`）・Worker（`web/edge/index.ts`）で掛かる。エンジンは掛けない（`engine.KnockdownFromFloat` は int32 の範囲だけ）。
- 中断は既にある（Worker ごと terminate）。HANDOVER §5 の「Worker と中断ボタン付き」は満たしている。
- 予備計測（Node 上の WASM、ryu 重ねる・既定のクエリ）: KA 100 = 8.0 秒・428 MiB、KA 150 = 5.7 秒・646 MiB、KA 200 = 33.3 秒・605 MiB、KA 400 は 77 秒で打ち切り。ingrid 汚連携 KA 100 は 3 分を超えて打ち切った（推測: 既定のクエリが画面と違う。画面の条件での計測は実装後にやり直す）。→ KA を上げると時間は大きく伸びるが、中断で止められる。

### 作るもの
1. **フロントのローカル版ビルド**: `vite build --mode local`（`web/src/lib/flavor.ts` で判定）。
   - KA の上限を外す（推測: エンジンが受ける int32 の上限 2147483647 を `KA_MAX_LOCAL` にする。数で切らない＝「無制限」。重い条件は既存の「時間がかかる」注意と中断で扱う）。サイトは 100 のまま（`KA_MAX_SITE`。beacon と Worker は常に `KA_MAX_SITE`）。
   - 通信しない: 計測（config.json・beacon）と票（取得・投票ボタン・票の案内）を出さない。プライバシーポリシーの代わりに「ローカル版は計測・投票の通信をしない」と出す（5 言語）。
   - サイト側: KA の説明に「100 を超えるときはローカル版（Releases）」の案内を足す（D10）。
2. **Go の単一バイナリ** `cmd/local`（標準ライブラリのみ）: ローカル版の dist（データ・WASM 込み）を `embed` し、`127.0.0.1` だけで配る。`_headers` を同じ規則で読んでヘッダを付ける（CSP 下で WASM・Worker が動くこと）。Content-Type は固定の表（Windows の registry の MIME に頼らない）。起動したらブラウザを開く（`-open=false` で止める）。Host の検査・タイムアウト・Ctrl+C で終了。配信の処理は `internal/localserver`（`fstest.MapFS` で単体試験）。
   - 埋め込む中身は `scripts/local/build.sh` が `cmd/local/site/` に置く（gitignore。プレースホルダ 1 つだけコミット。中身が無いバイナリは起動時に理由を出して終わる）。
3. **Releases の workflow** `.github/workflows/local-release.yml`: ubuntu で 1 回ビルド（5 種: linux amd64/arm64・darwin amd64/arm64・windows amd64）→ ubuntu・macos・windows で**起動確認**（実バイナリを起動し、index・manifest・WASM・ヘッダを HTTP で確かめる）→ タグ（`v*`）の push のときだけ **draft の Release** を作る（公開は開発者が押す＝AGENTS §3-3）。PR・`claude/**` への push では関係するパスが変わったときだけ走る（必須 check ではないので paths フィルタ可）。
4. **govulncheck**: `scripts/govulncheck.sh`（版 v1.9.0 固定。対象は `./...`・WASM・ローカル版を windows/darwin/linux で）。CI の Go job に step 1 つ＋summary、`.github/workflows/govulncheck.yml`（週 1・手動）。
5. **E2E**: Playwright に project `local` を足し、実バイナリを起動して KA 120 で提案が出る・計測と票の通信が無いことを確かめる。CI の Web job にローカル版のビルドの段を足す（job は増やさない）。
6. docs: `docs/local.md`（使い方・ビルド・配布・署名なしの警告）、README、`docs/release-checklist.md`（S9 の手番: Release のタグ、About の Website）。

### 回す検査
- `go test -count=1 ./...`・`go vet`・`gofmt -l`、`GOOS=js GOARCH=wasm` のテスト（変更が無ければ省略しない範囲で）
- `pnpm exec tsc --noEmit`・`vitest --run`・`pnpm build`・ローカル版のビルド・Playwright（サイト＋local）
- `scripts/govulncheck.sh`（正常と、古い toolchain でわざと赤）
- `check-licenses.py`・`check-private-refs.py`・`check-actions-pinned.sh`
- 3 OS の起動確認は local-release.yml の結果（push 後に GitHub で確かめる）

### 未決事項と扱い
- KA の上限の値（無制限の表し方）: 自分で決める（上の推測）。
- Release の作成（タグの push）: 公開に関わるので開発者（S9 の手番として書く）。workflow は draft を作るまで。
- コード署名（macOS の公証・Windows の署名）: 費用がかかるのでやらない。警告の回避手順を docs に書く。
