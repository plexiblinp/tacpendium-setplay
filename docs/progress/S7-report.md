# S7 公開前監査 — 完了報告

- 段階開始コミット: `52af7a19cd599460d5483278a8e7df6b3a08a285`
- ブランチ: `claude/relaxed-euler-t2z8gs`

## 計画

### Context
HANDOVER §9 S7「公開前監査: ヘッダ/CSP、依存、Actions 固定、`_reference/` 削除、private 由来の記述の除去 → public 化」。
完了条件: **外部ヘッダ診断で問題なし、§13 チェックリスト完**。
- 段階開始コミット: `52af7a19cd599460d5483278a8e7df6b3a08a285`（ブランチ `claude/relaxed-euler-t2z8gs`）
- public 化・デプロイ・GitHub 設定の変更・Cloudflare の作業は**開発者が行う**（AGENTS §3-3〜5）。Claude はリポジトリ内の整備と、手順・照合表の作成まで。
  → 「外部ヘッダ診断」はデプロイ後でないと回せないので、S7 では**手元の本物の配信（serve.mjs＋_headers＋Worker）でヘッダを機械検査**し、外部診断（securityheaders.com / Mozilla Observatory）の手順と期待値を書いて開発者に渡す。完了条件のこの項目は「一部（デプロイ待ち）」と正直に書く。

### 調査で分かったこと（読み取りのみ）
- `_reference/` を消して**壊れるのは 1 系統だけ**: `scripts/check-licenses.py:54` の EXPECT、`REUSE.toml` の 4 番目の表、`LICENSES/LicenseRef-Tacpendium-NotForDistribution.txt`。Go/WASM/web の試験は `_reference` を読まない（本家突き合わせは `engine/testdata` ＋公開版 tacpendium）。
- 利用者に見える文字列（5 言語・README・NOTICE）には非公開の名前は無い。
- 残っている場所: `AGENTS.md`（本家の名前・本家の内部文書のパス）、`.claude/commands/{review,sync_upstream}.md`、`upstream/README.md`、`ci.yml` のコメント（private）、hooks の `_reference` 除外、`internal/service/setplay/setplay.go:4`（本家の内部のパス）、Go のコメントに非公開の内部文書 ID（本家の設計書・調査報告・知識集の節番号など、約 20 ファイル）、`docs/handover/HANDOVER-001`（本家の SHA・非公開 DB の詳細・§13 の禁止語そのもの）、`docs/progress/S0〜S6`（grep 結果に禁止語）。
- Actions は全て SHA 固定・`permissions: contents: read`・`persist-credentials: false`。Dependabot は actions と npm のみ（**gomod なし**）。`ci.yml` に `pull_request` トリガー無し（public 化で足す予定とコメント）。
- ヘッダ: `_headers` に CSP 等あり。**HSTS 無し**（workers.dev は HTTPS 固定だが診断で減点される）。`/api` の応答は nosniff・no-store のみ。
- `wrangler.toml` の `database_id` は 0 埋めの仮置き（開発者がデプロイ時に入れる）。秘密情報のコミットは無し。
- 新キャラ 2 週間ポリシーは未記載。`PRIVACY_LAST_UPDATED` は 2026-10-17（公開日に揃える）。NOTICE §4 の公開 URL が空欄、README に「未公開」の帯。

### やること（順に。節目ごとに commit・push）
1. **計画のコミット**: `docs/progress/S7-report.md` に本計画節。
2. **`_reference/` 削除**（1 コミット）: `git rm -r _reference`、REUSE 表 4・LicenseRef ファイル・EXPECT 行の削除、`ci.yml`・hooks 3 本の `_reference` 除外を外す。`check-licenses.py` が緑のままを確認。
3. **private 由来の記述の除去**（現在のツリー）:
   - `AGENTS.md`・`.claude/commands/review.md`（観点 5 の比較先を「本家突き合わせ試験と公開版 tacpendium」に）・`sync_upstream.md`・`upstream/README.md`・`README.md`・`DATA-LICENSE.md`・`ci.yml` のコメント。
   - Go/TS のコメント: `.upstream`（公開版）を取得し、**公開版に同じ記述があるものは残す**（既に公開）。公開版に無い非公開文書 ID・本家の内部のパスは、ID を外して説明だけ残すか、公開版の対応箇所への参照に置き換える。説明の中身は変えない（挙動のコメント）。
   - `docs/handover`・`docs/progress` の扱いは下の Q3 の回答どおり。
   - **検査スクリプト** `scripts/check-private-refs.sh` を新設し CI の checks job で回す（禁止語はハッシュで持ち、語そのものを公開ツリーに書かない＝「何を伏せたか」の手がかりを残さない）。§8-3: 禁止語を 1 行入れて赤、を確認。
4. **ヘッダ/CSP**:
   - `_headers` に `Strict-Transport-Security: max-age=31536000`（preload は付けない＝workers.dev は自分のドメインでないため）と `Cross-Origin-Resource-Policy: same-origin` を追加（推測: 診断の減点項目。CSP は変えない）。
   - Worker の `/api` 応答にも `Referrer-Policy`・`X-Frame-Options`/CSP `frame-ancestors 'none'` 相当（`default-src 'none'`）を付ける。
   - E2E に「実配信のヘッダを全経路（`/`・`/data/manifest.json`・`/wasm/*`・`/api/votes`・`/api/beacon`）で検査」を 1 本追加。§8-3: ヘッダを 1 行消して赤を確認。
   - `docs/release-checklist.md` に外部診断の手順・期待値（A 以上）を書く。
5. **依存・CI**: `dependabot.yml` に gomod を追加。`ci.yml` に `pull_request` トリガー（public 化後のフォーク PR 用。権限は read のまま）。`pnpm audit --prod` と `govulncheck`（Go 公式・BSD）を回し、結果を報告に貼る（CI への常設は判断して報告）。`check-actions-pinned.sh`・`check-licenses.py` を回す。
6. **文言**:
   - 新キャラ 2 週間ポリシー（配信開始日起点・新キャラのみ・書くだけ）を `limitations` に 1 キー（5 言語。zh-Hant は生成）、README、`sync_upstream.md`（運用: 配信から 2 週間たってから取り込む）。
   - NOTICE §4・README の帯・`PRIVACY_LAST_UPDATED` は**公開日が決まってから**（S9）。S7 では「公開時に変える箇所」の一覧を checklist に書く。
   - 画面の項目（下の「開発者の決定」Q4）。
7. **プライバシーの法的照合**: `/research` 相当（読み取り・Web 調査）で、個人情報保護法の個人関連情報・GDPR の仮名化/ePrivacy・Cloudflare 規約・**D1 Time Travel で消したソルトが戻るか**（Cloudflare docs で確認）をまとめ `docs/progress/S7-privacy-research.md` へ。法的助言ではない。対策が要るなら開発者判断として報告。
8. **§13 チェックリスト照合表** `docs/release-checklist.md`: 各項目に 済／開発者作業／デプロイ後 と証拠。GitHub 設定（Dependabot alerts・secret scanning＋push protection・main 保護と必須 check・Actions 権限・2FA）は Q2 の方法で照合項目を作る。Cloudflare（カード未登録・回数制限）は開発者の確認項目。KA 0〜100 全キャラの生成物＋golden 緑は手元で回して証拠を貼る。
9. **レビュー**: 新しい general-purpose サブエージェントで `/review 7 52af7a1`（読み取り専用、`S7-review.md` のみ書く）。最大 2 往復で取り込み。
10. **完了報告**と再検査、push、SHA 一致を確認して停止。

### 回す検査（出力はファイル、`EXIT=` と件数）
`go test -count=1 ./...`・`go vet`・`gofmt -l`・`go mod tidy -diff`・WASM 試験・`node --test scripts/wasm/wasm.test.mjs`・`tsc --noEmit`・`vitest --run`・`pnpm build`（上限検査）・`playwright test`・`check-licenses.py`・`check-actions-pinned.sh`・`verify-plan.py check`・新設 `check-private-refs.sh`・`govulncheck`・`pnpm audit`。最後に `git diff --stat 52af7a1`。

### 開発者の決定（2026-10-17、S7 冒頭）
- Q1 履歴: **最終ツリーを 1 コミットにまとめて公開**。S7 では手順を `docs/release-checklist.md` に書くだけで、実行は開発者（S9）。
- Q2 GitHub 設定の照合: **本家を接続**（開発者の指示。理由: こちらの方が新しい可能性がある）。
  - プラン承認後に 本家を `add_repo`（read）でこのセッションに追加し、**作業ディレクトリの外**（スクラッチパッド）にクローンする。リポジトリにコミットされないようにするため。
  - 読むのは公開時チェックリストと GitHub 設定の記述だけ。中身は写さず、照合表にはこのリポジトリ向けの項目だけを書く（D2 は不変、§7）。
- Q3 docs: **伏せ字で残す**。本家の名前・SHA・内部文書名・非公開 DB の詳細を「本家（非公開）」等に置き換え、`check-private-refs.sh` で残数 0 を確かめる。
- Q4 画面（全部 S7 に入れる）:
  - 制約の説明を短くする
  - 絞り込みの文言の案
  - キャラ検索＋リセット
  - **絞り込みのデザインを分かりやすく変える**（開発者の追加要望）
  - 進め方: 先に文言とデザインの案（2〜3 案、画面の画像付き）を作り、AskUserQuestion で選んでもらってから 5 言語で実装する（推測: 見た目の好みは開発者が決めることなので）。
  - 試験: vitest と E2E を足し、§8-3 で赤を確かめる。

### 本家の公開時チェックリストから拾った GitHub 設定（照合表の入力）
本家の公開手順書（読み取りのみ。中身は写さず、このリポジトリに当てはまる項目だけ）:
- public 化の前に private のまま中身と設定を確かめる（1 コミットにまとめた公開用リポジトリ／ブランチを private で作り、確かめてから public）。
- main の保護（ruleset: 削除・force push 禁止）。**可視性を変えると ruleset が無効になりうるので、public 化の直後に再確認する。**
- Private vulnerability reporting・Dependency graph・Dependabot alerts は public でも既定 off（opt-in）。public 化と同時に有効化する。
- CodeQL（default setup・Go と JS/TS）・Secret scanning・Push protection は public なら無料。public 化の時点で有効化する。
- 公開物は CI から出す（手元から上げない＝AGENTS §8-12 と同じ）。
- 「忘れても何も赤くならない」型の項目なので、照合表で 1 項目ずつ済を付ける。

## やったこと

| 項目 | 成果物 | 確かめたこと |
|---|---|---|
| `_reference/` の削除 | `git rm -r _reference`（141 ファイル）、REUSE の表 4・`LICENSES/LicenseRef-Tacpendium-NotForDistribution.txt`・`check-licenses.py` の代表パス・`ci.yml` と hook 3 本の除外を外した | `check-licenses.py` 違反 0。Go・WASM・web の試験は `_reference` を読まない（探索で確認。本家との突き合わせは `engine/testdata` と公開版 tacpendium） |
| 非公開由来の記述 | `scripts/check-private-refs.py`（語は SHA-256 で持つ。16 進は先頭 7 文字で照合）＋試験 `scripts/check_private_refs_test.py`、CI の Go job に組み込み。HANDOVER の本家 SHA とメモ名の 2 か所を伏せた。`AGENTS.md`・`review.md`・`sync_upstream.md`・`upstream/README.md`・`README.md`・`DATA-LICENSE.md` の `_reference` 参照を公開版 tacpendium（`.upstream/tacpendium`）へ | 検査を入れる前の木で「2 件」と出ることを確認（§8-3）。直した後 0 件（走査 270 ファイル） |
| Actions の節約 | `ci.yml`: 起動を main 向け PR・main への push・手動に。6 job → 2 job（`Go・WASM・リポジトリ検査`・`Web`）。独立な段は `!cancelled()`。AGENTS §9 に運用 | YAML の構文・`check-actions-pinned.sh` 違反 0。CI 上の実測は PR を出したとき（下の残課題） |
| ヘッダ | `_headers` に `Cross-Origin-Resource-Policy`・`X-Frame-Options: DENY`・`Strict-Transport-Security: max-age=31536000`。Worker の `/api` 応答に `API_SECURITY_HEADERS`（同じ値＋`default-src 'none'; frame-ancestors 'none'` の CSP）。E2E `web/e2e/headers.spec.ts`（静的と `/api`）。Worker の単体試験に 1 本（期待値は定数と別に書く） | §8-3: 単体（Worker の 204 から外す・`_headers` から 1 行消す）で 2 本赤、E2E（ビルド済みの `_headers` と Worker から 1 つずつ消す）で 2 本赤 |
| 依存 | Go の toolchain を go1.26.8 → go1.26.9、`dependabot.yml` に gomod。`verify-plan.py ack --none`（理由に手元の全突き合わせ） | govulncheck v1.9.0（DB は `storage.googleapis.com/go-vulndb`。`vuln.go.dev` は proxy で遮断）: go1.26.8 は標準ライブラリ 13 件（コードからの到達 0、package 水準 1 件は Windows の `os.Root`）、go1.26.9 で 0 件。WASM（`GOOS=js GOARCH=wasm ./cmd/wasm/`）も 0 件。`pnpm audit`（`--prod` と全体）0 件・160 依存を走査 |
| 画面（A 案） | 「重ねる技」の枠に「選び方: 種類で選ぶ／1 つの技を指定」。キャラの名前検索（`web/src/lib/characterSearch.ts`。本家と同じ語の先頭一致）。「条件をリセット」。文言 5 言語、`docs/frontend.md` | vitest: `App.test.tsx` に 6 本（既存の「特定の技」の 1 本を書き換え、5 本を新規。うち 1 本はレビューの取り込みで足した「技が未選択なら押せない」）と `characterSearch.test.ts` に 5 本。E2E に 1 本。§8-3: 種類へ戻すときに技を外さない／URL の技で開かない／リセットで URL を戻さない／先頭一致を部分一致にする／Enter で選ばない、で狙いの試験が赤（7 本と 1 本）。E2E はリセットで URL を戻さないビルドで赤 |
| 新キャラの方針 | README「新キャラクターへの対応」、`/sync_upstream` の運用、画面の「提案の制約」の最後の項目（5 言語） | vitest 1 本（項目を外すと赤） |
| プライバシー | `docs/progress/S7-privacy-research.md`（サブエージェントの調査。法的助言ではない）。`web/wrangler.toml` に `[observability] enabled = false`。`docs/telemetry.md` の Time Travel の記述を事実に直した | Time Travel は常時有効・無料プランで 7 日（Cloudflare docs のソースで確認）。法令の一次資料は proxy で読めず「未確認」 |
| 公開の手順 | `docs/release-checklist.md`（§13 の照合・1 コミットでの公開・外部のヘッダ診断・公開時に直す箇所・GitHub 設定 G-1〜G-9） | 本家の公開手順書（非公開）を読み、当てはまる項目だけを書き直した |

## 判断と理由

- **伏せ字の範囲を「公開されていないものだけ」に縮めた**（開発者決定 2026-10-18）。公開版 tacpendium に本家の名前（358 ファイル）・内部文書 ID・知識集の本文そのものが既にあり、伏せても読めるため。公開されていないもので木にあったのは、本家の SHA 1 つ（HANDOVER）とメモ名 1 つ（同）だけだった（16 進の連なりを、このリポジトリの履歴・公開版の lock・外部の SHA と突き合わせて洗い出した）。検査の禁止語は、これに `_reference/` の元の SHA と S7 で読んだ本家の先端の SHA を足した 4 つ。
- **禁止語をハッシュで持つ**: 検査スクリプトが「何を伏せたか」の一覧にならないように。語を足す手順はスクリプトの先頭に書いた。
- **CI の job を 2 つにした**: 課金が job ごとに 1 分切り上げで、軽い変更でも約 9 分かかっていた。まとめると同じ job の中で 1 つの失敗が後の段を止めるので、独立な段は `!cancelled()` で回し、検査済みの記録（cache）は `success()` のまま（失敗の後に記録しない＝検査が抜けない）。
- **toolchain の更新を CI で回し直さない**（`ack --none`）: 手元の go1.26.9 で全 Go 試験と本家との突き合わせ（全キャラ 5,208 条件）を回して不一致 0 だったため。CI の分数を節約する。
- **HSTS に preload を付けない**: `workers.dev` は自分のドメインでない。**`X-Frame-Options` も付けた**: CSP の `frame-ancestors` を解さない古いブラウザ向けで、外部診断で減点されるため。
- **キャラの名前検索は検索欄＋ネイティブの選択**: 本家は検索欄付きのドロップダウン（Radix の Popover と独自のキーボード操作、300 行余り）。照合の規則は本家と同じにしたが、部品は依存を増やさずブラウザ任せの形にした（Enter で先頭を選ぶ）。選んでいるキャラは絞り込みで外れても一覧に残す（選択の表示が空にならないように）。推測: 本家の部品の見た目に揃える必要があれば後で替えられる。
- **「重ねる技」の切替は表示の状態**で、条件に効くのは `targetMove` だけ。URL に技があれば技の指定で開く（URL は欄が出た後に読まれるので、`form.targetMove` からも導く）。条件と URL の形は変えていない。汚連携のモードでも見出しは「重ねる技」（並び順の「重ねる技順」と揃えた。開発者の文言の決定どおり）。
- **リセット**は条件・結果・URL を最初に戻し、計算中なら中断する。言語と表記は利用者の設定なので戻さない（S5 の要望の推測どおり）。
- **Workers Logs を止めた**: 既定で有効になる可能性があり、記録に IP が含まれうる（調査 §2.3。未確認）。止めても費用・機能に影響せず、プライバシーポリシーの約束（生の IP を残さない）に沿う。
- **公開は新しいリポジトリで**（推奨。開発者が決める）: GitHub は PR の head を消さないので、このリポジトリの main を書き換えて public にしても `_reference` を含む古いコミットが読める。

## 逸脱

- 計画 3 の「Go/TS のコメントの非公開文書 ID を外す」は**行わなかった**（開発者決定で範囲を縮めたため。公開済みの ID なので残した）。
- 計画 4 の「外部のヘッダ診断で問題なし」は**まだ満たしていない**（デプロイ前のため。手元の本物の配信での検査で代えた。§13 の照合は `docs/release-checklist.md`）。
- 計画では検査スクリプトを `check-private-refs.sh` としたが、Python で書いた（ハッシュと 16 進の照合を標準ライブラリで書くため）。

## 残課題・既知の問題

- **提案の制約の書き直し**: 開発者が改善版を考え、Claude が整える（開発者決定 2026-10-18）。現行の全文はセッションで提示した。新キャラの方針は暫定で末尾の項目に置いた（改善版に入れたら消す）。
- **プライバシーポリシーの文と対策**（調査 §7 の表。開発者が決める）: (1) 「日をまたいで結び付けることはできません」は Time Travel（7 日）と遅延削除のため事実より強い。(2) 対策の案: beacon の `visitor` を 32 ビットに切る（C）・IP を丸める（D）・定時に消す（E）。(3) EU の訪問者（ePrivacy 5(3) で beacon に同意が要る余地）。法令の一次資料は proxy で読めず、調査の法令の部分は「未確認」。
- **CI の実測**: 2 job にした後の分数は、開発者が PR を作ったときに測る（この段階の push では CI は走らない）。S7 の最初の 2 回の push の CI は数秒で失敗した（推測: 無料枠の使い切り。開発者の連絡と一致）。
- **外部のヘッダ診断・Workers Logs の確認・D1 の作成**はデプロイ後（開発者。`docs/release-checklist.md`）。
- 手元の E2E は Chromium 141（`/opt/pw-browsers/chromium-1194`）。CI は `playwright install` の版。
- `go1.26.9` の取得は `GOTOOLCHAIN=auto` に頼る（手元の Go は 1.24.7。go.mod の toolchain 行で自動で取る）。

- `check-private-refs.py` の語の照合は、1 つの語の部分の数の 2 乗で増える（再レビュー低。今の木で全体 1.7 秒）。数千の部分を持つ語が入ったら上限を設ける。

## 開発者に決めてほしいこと（2026-10-18 その 3 で回答済み）

1. 提案の制約の改善版 → 開発者の案を要約し「使い方と注意」にした（下の「レビュー後の追加」）。
2. プライバシーポリシー → 文だけ直す（仕組みは変えない）。
3. 公開の形 → 新しいリポジトリに 1 コミットで出し、以後はそこで直接開発。今のリポジトリはほぼ凍結。
4. 公開時の設定 → CI の `claude/**` への push での起動と upstream-watch の定期実行を戻す（Claude の推奨を開発者に提示。`docs/release-checklist.md` §4）。
5. 公開後の受け付け → Google フォーム＋Issues。PR は受け付けない。

**Google フォーム**: 開発者が作った URL を `FEEDBACK_FORM_URL` に入れた（2026-10-18。追跡用の引数 `?usp=header` は外した）。

### 版の固定と解除条件（AGENTS §5）

- `go.mod` の `toolchain go1.26.9`: 標準ライブラリの脆弱性の修正版。解除条件は無し（次の修正版が出たら上げる。Dependabot は toolchain 行を上げないので、govulncheck で見る）。

## 次の段階への入力（S8 ローカル版）

- `go.mod` の toolchain は go1.26.9。ローカル版（単一バイナリ）は標準ライブラリの HTTP サーバを使うはずなので、govulncheck を `-db https://storage.googleapis.com/go-vulndb` で回す（`vuln.go.dev` はこのコンテナの proxy で遮断）。Windows の `os.Root` の件（GO-2026-6604）は go1.26.9 で直っている。
- CI は PR・main・手動でしか走らない。S8 の作業中の証拠は手元で回す。job は増やさない（AGENTS §9）。3 OS のビルドを CI に足すなら、既存の job の段にするか、手動起動の workflow にする。
- `_reference/` は無い。移植元は `bash scripts/fetch-upstream.sh` で `.upstream/tacpendium`。
- 公開してはいけない語の検査がある（`scripts/check-private-refs.py`）。本家の SHA を報告・コミットメッセージに書かない。

## 最終確認（レビュー 1 回と再レビュー 1 回の取り込み後、HEAD `c08042a` で回した結果）

出力はファイルへ書き、合否は `EXIT=` と件数で見た。手元の Go は `go.mod` の toolchain 行で go1.26.9 を使う。

```
$ go test -count=1 -v -timeout 35m ./... > go.txt 2>&1; echo EXIT=$?
EXIT=0
PASS(全階層 ^\s*--- PASS)=348 FAIL=0 SKIP=0
    conformance_test.go:332: 突き合わせた条件: 5208(キャラ 31 ・不一致 0)
$ GOOS=js GOARCH=wasm go test -count=1 -exec=scripts/wasm/go-test-exec.sh ./internal/service/setplay/ ./internal/repository/setplay/ ./engine/ → EXIT=0(3 パッケージ ok)
$ bash scripts/wasm/build.sh && node --test scripts/wasm/wasm.test.mjs → EXIT=0  # pass 6  # fail 0
$ go vet ./...                         → EXIT=0
$ gofmt -l(追跡中の .go)              → 出力なし
$ go mod tidy -diff                    → EXIT=0
$ govulncheck v1.9.0 -db https://storage.googleapis.com/go-vulndb ./... → EXIT=0  No vulnerabilities found.
$ cd web && pnpm exec tsc --noEmit     → EXIT=0
$ pnpm exec vitest --run > vitest.txt 2>&1; echo EXIT=$?
EXIT=0
 Test Files  15 passed (15)
      Tests  159 passed (159)
$ bash scripts/prepare-assets.sh       → EXIT=0(public/data・public/wasm を消してから)
$ pnpm build                           → EXIT=0  ファイル: 41(上限 20000) / 違反: 0 件
$ PW_CHROMIUM_PATH=/opt/pw-browsers/chromium-1194/chrome-linux/chrome pnpm exec playwright test > e2e.txt 2>&1; echo EXIT=$?
EXIT=0
  24 passed (41.1s)
$ pnpm audit --prod                    → EXIT=0  No known vulnerabilities found
$ python3 scripts/check-licenses.py          → EXIT=0  違反: 0 件
$ bash scripts/check-actions-pinned.sh       → EXIT=0  workflow: 3 本 / uses 参照: 14 件 / 違反: 0 件
$ python3 -I scripts/ci/verify-plan.py check → EXIT=0  違反: 0 件
$ python3 -I scripts/ci/verify_plan_test.py  → EXIT=0  OK
$ python3 -I scripts/check_private_refs_test.py → EXIT=0  Ran 6 tests / OK
$ python3 -I scripts/check-private-refs.py   → EXIT=0  走査: 271 ファイル / 違反: 0 件
$ bash -n(hooks・scripts・upstream・web/scripts の .sh) → 誤りなし
```

- **完了条件**:
  - 「チェックリスト完」: リポジトリの中で済ませられる項目は済（`docs/release-checklist.md` §1 の 1・3・4・5・6〔手元〕・8・10）。残りは開発者の作業（2 履歴を含めない公開・7 Cloudflare・9 GitHub 設定）で、手順と照合表を書いた。
  - 「外部ヘッダ診断で問題なし」: **未了**（デプロイ前のため。逸脱の節）。手元の本物の配信で、静的アセットと `/api` のヘッダを E2E で検査している。
- **試験の数**: Go の PASS は S6 と同じ 348（Go の試験は増減なし。`_reference/` の中の試験は Go のビルド対象外だったので数に入っていない）。vitest は S6 の最後の 147 → 159（+12: `characterSearch.test.ts` 5・`App.test.tsx` 新規 5〔既存 1 本は書き換え〕・新キャラの方針 1・Worker のヘッダ 1）。E2E は 21 → 24（`headers.spec.ts` 2・A 案の流れ 1）。Python の試験は `check_private_refs_test.py` 6 本が新規。消した・skip した試験は無い。
- **CI**: S7 の途中の push では CI は走らない（S7 で変えた）。S7 の最初の 2 回の push（`ab83814`・`31891c7`）の CI は数秒で失敗した（推測: Actions の無料枠の使い切り）。2 job にした後の CI と分数の実測は、開発者が PR を作ったときに確かめる。

### `git diff --stat 52af7a19cd599460d5483278a8e7df6b3a08a285 HEAD`（HEAD = c08042a。本節の追記を含まない）

`_reference/` の削除（141 ファイル・47,678 行の削除）は行が多いので 1 行にまとめ、それ以外を逐語で貼る。

```
 _reference/ 141 files changed, 47678 deletions(-)
 .claude/commands/review.md                         |   4 +-
 .claude/commands/sync_upstream.md                  |   3 +-
 .claude/hooks/post-edit-check.sh                   |   2 +-
 .claude/hooks/session-start.sh                     |   4 +-
 .claude/hooks/stop-test.sh                         |   6 +-
 .github/dependabot.yml                             |  16 +-
 .github/workflows/ci.yml                           | 370 +++++++++------------
 AGENTS.md                                          |  23 +-
 DATA-LICENSE.md                                    |   3 +-
 .../LicenseRef-Tacpendium-NotForDistribution.txt   |  48 ---
 README.md                                          |   7 +-
 REUSE.toml                                         |  11 +-
 docs/frontend.md                                   |  10 +-
 docs/handover/HANDOVER-001-setplay-standalone.md   |   8 +-
 docs/progress/S7-privacy-research.md               | 144 ++++++++
 docs/progress/S7-report.md                         | 124 +++++++
 docs/progress/S7-review.md                         | 198 +++++++++++
 docs/release-checklist.md                          |  89 +++++
 docs/telemetry.md                                  |   3 +-
 go.mod                                             |   2 +-
 scripts/check-licenses.py                          |   1 -
 scripts/check-private-refs.py                      |  94 ++++++
 scripts/check_private_refs_test.py                 |  56 ++++
 scripts/ci/logic-version.json                      |  12 +-
 upstream/README.md                                 |   2 +-
 web/e2e/headers.spec.ts                            |  42 +++
 web/e2e/setplay.spec.ts                            |  23 ++
 web/edge/index.test.ts                             |  28 +-
 web/edge/index.ts                                  |  18 +-
 web/public/_headers                                |   5 +
 web/scripts/scripts.test.mjs                       |   4 +
 web/src/components/App.test.tsx                    |  95 +++++-
 web/src/components/App.tsx                         |  10 +
 web/src/components/ConditionsPanel.tsx             | 163 ++++++---
 web/src/components/LimitationNotice.tsx            |   2 +-
 web/src/components/TargetPicker.tsx                |   2 +-
 web/src/lib/characterSearch.test.ts                |  44 +++
 web/src/lib/characterSearch.ts                     |  31 ++
 web/src/locales/en.json                            |  19 +-
 web/src/locales/ja.json                            |  21 +-
 web/src/locales/ko.json                            |  21 +-
 web/src/locales/zh-Hans.json                       |  21 +-
 web/src/locales/zh-Hant.json                       |  21 +-
 web/src/styles.css                                 |   5 +-
 web/wrangler.toml                                  |   5 +
 45 files changed, 1428 insertions(+), 392 deletions(-)
```

## レビュー後の追加（開発者の回答 2026-10-18 その 3）

| 項目 | 成果物 | 確かめたこと |
|---|---|---|
| 使い方と注意 | `web/src/components/Guide.tsx`（冒頭 2 行＋本家〔公開版 tacpendium〕への導線、折りたたみ 3 つ: 使い方・注意と対策・レシピのルール）。`LimitationNotice.tsx` と `limitations.*`・`app.description` を消した。5 言語（zh-Hant は生成）。`docs/frontend.md`「使い方と注意」に、載せなかった旧項目とエンジンのコメントとの対応 | 案の事実をデータで確かめた: ルークの連打キャンセルの対象はしゃがみ弱P・しゃがみ弱K だけ（moves DB の `chainCancelTotal`）、ラッシュ版の元技が必殺技の行は 31 キャラで 0 件。vitest 1 本（3 つの折りたたみ・導線のリンク・新キャラの文・ルークの例）。§8-3: 新キャラの項目を外すと赤 |
| プライバシー | `privacy.hashBody` を「通常は〜。ただし消したデータは Cloudflare の復旧機能で 7 日間は戻せる状態で残る」（5 言語）。`privacy.contactFormBody`（フォームがあるときだけ出す）。`PRIVACY_LAST_UPDATED` を 2026-10-18 に | vitest（7 日の文・フォームが無いときは出さない）。§8-3: フォームの文を常に出すと赤 |
| 受け付け | `FEEDBACK_FORM_URL`（空）・`FEEDBACK_URL`（フォームか Issues）。画面の「改善の提案を送る」（5 言語。「(GitHub Issues)」を外した）。`CONTRIBUTING.md`・`SECURITY.md`（日英） | vitest（フォームが空なら Issues へ）。§8-3: Issues へ戻さないと赤 |
| 公開の手順 | `docs/release-checklist.md` §2（公開リポジトリで直接開発）・§4（CI の push 起動・upstream-watch の定期実行を戻す・フォームの URL）・§5 G-10（Issues を有効、PR の機能は Dependabot のため残す） | — |

- エンジンのコメント（`internal/service/setplay/*.go` の「ヒント文面〔web i18n `setplay.limitations.*`〕」）は公開版 tacpendium から移したまま残した（本家の画面の文言を指す。直すと公開版との差が増え、見張り対象のコードの記録〔ack〕も要る）。対応は `docs/frontend.md` に書いた。
- **この追加はレビュー（2 往復）の後に入れたもので、独立のレビューは受けていない**（往復の上限のため）。自分で回した検査は下のとおり。

```
$ cd web && pnpm exec tsc --noEmit → EXIT=0
$ pnpm exec vitest --run → EXIT=0  Test Files 15 passed (15) / Tests 159 passed (159)
$ pnpm build → EXIT=0
$ PW_CHROMIUM_PATH=... pnpm exec playwright test → EXIT=0  24 passed (48.0s)
$ python3 scripts/check-licenses.py → 違反: 0 件
$ python3 -I scripts/check-private-refs.py → 違反: 0 件
```

### `git diff --stat 52af7a19cd599460d5483278a8e7df6b3a08a285 HEAD`（追加の後。HEAD = aa21f99。本節の追記を含まない。`_reference/` の削除 141 ファイル・47,678 行は除いた）

```
 .claude/commands/review.md                         |   4 +-
 .claude/commands/sync_upstream.md                  |   3 +-
 .claude/hooks/post-edit-check.sh                   |   2 +-
 .claude/hooks/session-start.sh                     |   4 +-
 .claude/hooks/stop-test.sh                         |   6 +-
 .github/dependabot.yml                             |  16 +-
 .github/workflows/ci.yml                           | 370 +++++++++------------
 AGENTS.md                                          |  23 +-
 CONTRIBUTING.md                                    |  25 ++
 DATA-LICENSE.md                                    |   3 +-
 .../LicenseRef-Tacpendium-NotForDistribution.txt   |  48 ---
 README.md                                          |   7 +-
 REUSE.toml                                         |  11 +-
 SECURITY.md                                        |  17 +
 docs/frontend.md                                   |  19 +-
 docs/handover/HANDOVER-001-setplay-standalone.md   |   8 +-
 docs/progress/S7-privacy-research.md               | 146 ++++++++
 docs/progress/S7-report.md                         | 228 +++++++++++++
 docs/progress/S7-review.md                         | 198 +++++++++++
 docs/release-checklist.md                          |  91 +++++
 docs/telemetry.md                                  |   3 +-
 go.mod                                             |   2 +-
 scripts/check-licenses.py                          |   1 -
 scripts/check-private-refs.py                      |  94 ++++++
 scripts/check_private_refs_test.py                 |  56 ++++
 scripts/ci/logic-version.json                      |  12 +-
 upstream/README.md                                 |   2 +-
 web/e2e/headers.spec.ts                            |  42 +++
 web/e2e/setplay.spec.ts                            |  23 ++
 web/edge/index.test.ts                             |  28 +-
 web/edge/index.ts                                  |  18 +-
 web/public/_headers                                |   5 +
 web/scripts/scripts.test.mjs                       |   4 +
 web/src/components/App.test.tsx                    | 108 +++++-
 web/src/components/App.tsx                         |  15 +-
 web/src/components/ConditionsPanel.tsx             | 163 ++++++---
 web/src/components/Guide.tsx                       |  69 ++++
 web/src/components/LimitationNotice.tsx            |  29 --
 web/src/components/PrivacyPolicy.tsx               |  12 +-
 web/src/components/SettingsBar.tsx                 |   4 +-
 web/src/components/TargetPicker.tsx                |   2 +-
 web/src/lib/characterSearch.test.ts                |  44 +++
 web/src/lib/characterSearch.ts                     |  31 ++
 web/src/lib/constants.ts                           |   7 +
 web/src/locales/en.json                            |  65 ++--
 web/src/locales/ja.json                            |  67 ++--
 web/src/locales/ko.json                            |  67 ++--
 web/src/locales/zh-Hans.json                       |  67 ++--
 web/src/locales/zh-Hant.json                       |  67 ++--
 web/src/styles.css                                 |  13 +-
 web/wrangler.toml                                  |   5 +
 51 files changed, 1837 insertions(+), 517 deletions(-)
```

## 追加 2: キャラの選択を本家と同じ形に（開発者の要望 2026-10-18）

- S7 の前半では「検索欄＋ネイティブの選択」の 2 段にしていたが、開発者の要望で本家の部品（`SearchableSelect`。公開版 tacpendium で確認）と同じ動きにした: 箱を押すと検索欄（自動でフォーカス）と候補の一覧が開き、語の先頭一致で絞り、↑↓（端で折り返す）・Enter・クリックで選ぶ、Esc か外側のクリックで閉じて検索語を消す。依存は増やしていない（本家の Radix の Popover は使わず `web/src/components/CharacterSelect.tsx` を自前で書いた）。照合の規則（`characterSearch.ts`）は変えていない。
- 試験: vitest の名前検索の 1 本を新しい形に書き直した（開く・絞る・Enter・↑↓・一致なし・Esc・開き直すと検索語が空）。ほかの試験と E2E はキャラを選ぶ手順を新しい部品に合わせた（`pickCharacter`・`character-option-<code>`、選択は `data-value` で見る）。§8-3: ↑↓ を効かなくすると赤、閉じたときに検索語を消さないと赤。
- これもレビュー（2 往復）の後の追加で、独立のレビューは受けていない。

```
$ cd web && pnpm exec tsc --noEmit → EXIT=0
$ pnpm exec vitest --run → EXIT=0  Test Files 15 passed (15) / Tests 159 passed (159)
$ pnpm build → EXIT=0
$ PW_CHROMIUM_PATH=... pnpm exec playwright test → EXIT=0  24 passed (45.0s)
```

## 追加 3: Google フォームの URL（2026-10-18）

- `FEEDBACK_FORM_URL` に開発者のフォームの URL を入れた。画面の「改善の提案を送る」とプライバシーポリシーの案内（回答は開発者の Google アカウントに保存される）がフォームを指す。空に戻すと Issues だけを案内する（`feedbackUrl`）。
- 試験: `web/src/lib/constants.test.ts`（2 本。空なら Issues・URL の形）、`App.test.tsx` の 2 か所をフォームを期待する形に。§8-3: 空のときに Issues へ戻さないと赤。

```
$ cd web && pnpm exec tsc --noEmit → EXIT=0
$ pnpm exec vitest --run → EXIT=0  Test Files 16 passed (16) / Tests 161 passed (161)
$ pnpm build → EXIT=0
$ PW_CHROMIUM_PATH=... pnpm exec playwright test → EXIT=0  24 passed (50.3s)
```

## 追加 4: アプリ名と本家の紹介（開発者の要望 2026-10-18）

- **アプリ名は `Tacpendium-Setplay`**（開発者決定。HANDOVER §12-1 のサイト名の決定に当たる）。訳さない名前なので locales ではなく定数 `APP_NAME`（`web/src/lib/constants.ts`）にし、見出しは「Tacpendium-Setplay ＋副題（`app.title`。ja「セットプレイ自動提案(非公式)」）」、タブの題は「Tacpendium-Setplay | 副題」。`web/index.html` の題も同じに。
- 本家の紹介を書き直した（`guide.upstreamAbout`・`upstreamMerits`・`upstreamLanguages`・`upstreamLink`。5 言語）: このサイトは Tacpendium からセットプレイ自動提案だけを切り出した Web 版であること／Tacpendium（PC で動く無料アプリ）ならコンボを保存してセットプレイ・確定反撃と紐付け、状況まで記録・比較し、CSV や画像に書き出せること（公開版 tacpendium の README の「主な機能」から選んだ）／Tacpendium の画面は日本語のみ（英語は一部、ほかは未対応）。
- 試験: `App.test.tsx` の使い方の 1 本に、見出し・タブの題・紹介の 3 文を足した。§8-3: 見出しからアプリ名を外すと赤。vitest 161 passed・E2E 24 passed（どちらも EXIT=0）。
- 公開時: README 冒頭の「サイト名は未定」の帯は、公開時に外すときに名前を書く（`docs/release-checklist.md` §4）。

## 追加 5: 言語ごとのフォントの並び（開発者の懸念 2026-10-18）

- スクリーンショットの日本語が中国語風の字形だったのは、クラウドの作業環境に日本語のフォントが無く、`lang=ja` の既定が中国語のフォント（WenQuanYi Zen Hei）になるため（`fc-match "sans-serif:lang=ja"` で確認）。通常の環境（Windows・Mac・iOS・Android）では起きない（開発者の devContainer でも起きなかった）。
- 予防として、1 列だった並び（JP・KR・SC が混在）を言語ごとに分けた（`web/src/styles.css` の `:root:lang(...)`）。日本語は Hiragino・Yu Gothic・Meiryo・BIZ UDPGothic・Noto Sans JP だけで、中国語・韓国語のフォントを混ぜない。Web フォントは同梱しない。
- 試験: E2E 1 本（言語を切り替えると計算されたフォントの並びが変わる・日本語に SC/TC/KR/YaHei/PingFang が入らない）。§8-3: 日本語の並びに Noto Sans SC を足すと赤。E2E 25 passed・vitest 161 passed（どちらも EXIT=0）。見た目の確認は、日本語のフォントがある開発者の手元で。

