# セキュリティ監査の指摘への対応（F-01〜F-08） — 完了報告

- 日付: 2026-10-22
- 対象: 監査の指摘（HEAD 999925d 時点）。作業ブランチ `claude/s9-web-deployment-t0w876`（S9 のデプロイ準備〔PR #6〕の上に積んだ。開始は `920a91d`）
- ファイル名: 段階の外の単発の作業は `docs/progress/<種類>-<主題>-report.md`（`feat-share-report.md` と同じ形。Go 1.26.10 のホットフィックスは報告を作らなかった）

## 開発者がやること（デプロイの前に）

画面の項目名は 2026-10 時点の見込み。違っていたら見えた名前で読み替える（要確認）。

1. **本番 D1 に新しい表 `quotas` を作る（このコードをデプロイする前に必須）**
   - Cloudflare → 左のメニュー **Storage & Databases** → **D1 SQL Database** → **tacpendium-setplay** → **Console** タブ。
   - 次を実行する（`web/edge/schema.sql` の末尾と同じ。何度流しても同じ）:
     ```sql
     CREATE TABLE IF NOT EXISTS quotas (day TEXT NOT NULL, net TEXT NOT NULL, n INTEGER NOT NULL, PRIMARY KEY (day, net)) WITHOUT ROWID;
     ```
   - 確認: `SELECT name FROM sqlite_master WHERE type = 'table' ORDER BY name;` に **`ballots`・`quotas`・`salts`・`votes`** が出る（`_cf_KV` は Cloudflare の内部の表）。
   - 作らずにデプロイすると、日替わりのソルトの処理（`DELETE FROM quotas`）が誤りになり、投票と票の配信が 503、計測は利用者のハッシュなし（`unavailable`）になる（提案そのものは止まらない）。
2. **デプロイ方式の確認**: デプロイは GitHub Actions の CI の `deploy` job（「Cloudflare へ出す」）だけ。Cloudflare の Git 連携（Workers Builds）は**つながない**。
   - 最初のデプロイの後、Cloudflare → **Workers & Pages** → **tacpendium-setplay** → **Settings** → **Build** の **Git repository** が「未接続（Connect の表示）」であること。
3. **プレビュー URL と非本番ブランチのビルド**（最初のデプロイの後）
   - 同じ Worker の **Settings** → **Domains & Routes** の **Preview URLs** が **Disabled**（`wrangler.toml` の `preview_urls = false` がデプロイで反映される見込み）。Enabled なら Disable にする。
   - Git 連携をつないでいないので「非本番ブランチのビルド」は無い。もしつないだ場合は **Settings** → **Build** → **Branch control** の **Enable Preview Builds**（非本番ブランチのビルド）の印を外す。
4. **GitHub の設定の再確認**（2026-10-21 に点検済み。念のため）: リポジトリの **Settings** → **Advanced Security** で、次がすべて **Enabled**（無効なら Enable）。
   - **Dependabot alerts**・**Dependabot security updates**
   - **Secret scanning** と **Push protection**（Secret Protection の欄。赤い「Disable」の表示は「有効」の意味）
   - **Private vulnerability reporting**

## やったこと

| ID | 重大度 | 結果 | コミット |
|---|---|---|---|
| F-01 | Medium | **直した**: 回線（IPv4 のアドレス・IPv6 の /64）のハッシュで 1 分の上限と 1 日の票の上限を数える | `7ae9aa5` |
| F-02 | Low | **直した**: 票の配信に回線単位の回数制限（1 分 120 回） | `ea836c8` |
| F-03 | Low | **直した（キャラ・技 code まで）**: 配信していないキャラ・moves DB に無い技の票は 400。取りうる鍵の集合での検証は**しない**（見積もりは下） | `ff43c4c` |
| F-04 | Low | **直した**: 本文を読みながら数え、上限を超えたら読むのを止める | `734a9ff` |
| F-05 | Low | **直していない（案だけ）**: 依存の凍結の指示のため工程を足さなかった。案は `docs/release-checklist.md` §7 | `b77d574` |
| F-06 | Info | **直した（設定）**: `wrangler.toml` に `preview_urls = false`。出典は §7 | `b77d574` |
| F-07 | Info | **直した**: 静的アセットに無いパスの誤りの応答（400 以上）にも `/api` と同じヘッダ | `1b8cd60` |
| F-08 | Info | **確かめた**: D1 Time Travel は無料 7 日・有料 30 日（記述と一致）。出典を足した | `b77d574` |
| 文書 | — | プライバシーポリシー（5 言語）・`docs/telemetry.md`・チェックリスト | `3cf02b1`・`b77d574` |

### 変えた中身

- **F-01**（`web/edge/visitor.ts`・`index.ts`・`votes.ts`・`schema.sql`）
  - `netOf(ip)`: IPv4 はそのまま、IPv6 は先頭 /64（`::` の展開・末尾の IPv4 表記・ゾーン ID・`::ffff:a.b.c.d` は IPv4）。形の分からない値は 1 つの回線。
  - `netHash = sha256("net\n" + 回線 + "\n" + 日替わりソルト)` の先頭 32 桁。生の IP も回線の値も保存しない。
  - 1 分の上限の鍵を回線のハッシュに: beacon **120**、投票 **60**（定数 `BEACON_PER_NET_PER_MINUTE`・`VOTE_PER_NET_PER_MINUTE`）。
  - 1 日の上限: 利用者 200 は残し、回線 **1,000**（`VOTES_PER_NET_PER_DAY`）。新しい票のたびに `quotas` を UPSERT で数える（取り消しても戻らない）。`quotas` は `SaltStore` の日替わりの batch で前日以前を消す。
  - E2E 用の Worker（`edge/testing/local.ts`）にも新しい limiter を緩い値で足した。
- **F-02**: `handleVotes` を回線単位の limiter（**120/分**。`VOTES_LIST_PER_NET_PER_MINUTE`）に通す。
- **F-03**（`web/edge/catalog.ts` 新規）
  - Worker が `env.ASSETS.fetch("/data/manifest.json")` を `manifestSchema` で検証し、isolate の中で **5 分**覚える。無いキャラは投票・配信とも **400**（`unknown_character`）、読めないときは **503**（`data_unavailable`。読めなかったことは覚えない）。
  - 投票の鍵の技 code が、そのキャラの moves DB（manifest が指すファイル。技の `code` だけを検証して読む）に全部あるか。無ければ **400**（`unknown_move`）。
  - 手元・E2E の配信（`web/scripts/serve.mjs`）は、dist のファイルを返す `ASSETS` を Worker に渡すようにした（無いと票の E2E が 503 になる）。
  - `docs/telemetry.md` の「並びに効かない」を事実（配信の上限 10,000 件から本物の票を落としうる・回線ごとの上限で抑える）に直した。
- **F-04**: `readBody` は `Content-Length` の先の検査を残し、`req.body` を reader で読んで数え、上限を超えた時点で `reader.cancel()` して 413。`TextDecoder` の `stream: true` で chunk の境目のマルチバイト文字を繋ぐ。
- **F-07**: `withErrorSecurityHeaders`: Worker の最後の `env.ASSETS.fetch` の応答が 400 以上なら、無いヘッダだけ `API_SECURITY_HEADERS` を足す（アセットの 2xx・3xx は `_headers` に任せる）。
- **文書**: プライバシーポリシーの「利用者の区別」に回線のハッシュ（目的・IPv6 は前半 64 ビット・毎日消す）を 5 言語で足した（zh-Hant は `pnpm gen:locales`）。`PRIVACY_LAST_UPDATED` を 2026-10-22 に（試験の固定値も直した）。

## 判断と理由

- **IP だけにしなかった理由（F-01）**: 利用者のハッシュに UA を混ぜるのは、同じ回線（家庭の Wi-Fi・携帯の CGNAT）の別の人を、推定利用者数と票の重複防止で別の人として数えるため（D14・D17 の形）。IP だけにすると 1 回線の全員が 1 人になり、家族や同じ学校の人が票を付けられなくなる。そこで「1 人」は今の `hash(IP + UA + ソルト)` のまま残し、**水増しの上限だけ**を回線に掛けた（UA を変えても回線は変わらない）。D14 の「IP + 日替わりソルトのハッシュ（生 IP 非保存）」の範囲で、D14・D17 の決定は変えていない。
- **上限の値**: 1 人の普通の使い方（「提案を出す」は計算を待つので 1 分に十数回、👍 / 👎 の付け替えは 1 分に 20 回ほど）を、共有 IP の数人分（4〜6 倍）上回る値にした。beacon 120・投票 60・配信 120/分、1 日の票は 1 人 200 の 5 倍の 1,000。推測: このサイトの規模では、同じ IP から同時に数人を超えて使う場面はまれ。外れたら定数 1 か所で直せる。
- **IPv6 を /64 に丸めた理由**: 1 つの契約に /64 以上が配られ、端末は /64 の中でアドレスを自由に変える（プライバシー拡張）。/128 のままだと 1 台で無数の回線になれる。
- **manifest を Worker で読む方式（F-03）**: 案は (1) 束ねる時に manifest を Worker に埋め込む、(2) KV 等に置く、(3) 静的アセットを `env.ASSETS.fetch` で読む。(1) は Worker の束ねがデータの生成に縛られ、データだけの更新でも Worker を作り直す必要が出る。(2) は保存先が増え、無料枠と設定が増える。(3) は配信している中身そのものを見るので食い違いが無く、静的アセットの読み出しは無料・無制限。isolate の中で 5 分覚えて読み出しを抑える。採ったのは (3)。
- **鍵の検証の判断（F-03）**: 取りうる鍵（RecipeKey）の集合をエンジンで数えた（一時的なプログラムで `engine.Suggest` を KA 0〜100 × モード 2 つ〔meaty・gap〕、当てる技の種類はすべて、`limit` 3000 で回し、`totalFound` の合計と鍵の平均長を取った。モード間の重なりを含むので上限寄り、safe jump 等のモードは含まない）:

  | キャラ | 鍵の数（KA 0〜100 の合計） | 1 KA の最大 | 鍵の平均長 | 推定の大きさ |
  |---|---|---|---|---|
  | ryu | 7,227,862 | 308,497 | 90 B | 約 708 MB |
  | ken | 6,758,216 | 295,291 | 89 B | 約 656 MB |
  | jp | 5,623,265 | 361,291 | 72 B | 約 450 MB |
  | manon | 4,993,508 | 319,265 | 81 B | 約 444 MB |

  1 キャラで数百 MB、31 キャラで 10 GB を超える見込みで、配信の上限（`scripts/dist-limits.mjs`: 1 ファイル 25 MiB・20,000 ファイル）に収まらない。KA ごとに分けても 1 KA で最大 36 万件（約 30 MB）。**実装しない**。代わりに小さく済む「鍵の技 code がそのキャラの moves DB にあるか」を入れた（moves DB は 1 キャラ 60〜90 KB で、もう配っている）。これでも実在する技の組み合わせは通るので、配信の上限を埋める水増しは回線ごとの上限で抑える（本物のスコア 1 の票を落とすには約 2 万票＝約 20 回線・1 日）。
- **F-05 を足さなかったこと**: 依頼の指示（「凍結中は工程を足さない」）に従った。ただし、このブランチでは Node 22.23.4 への更新とともに Node・Go の凍結を 2026-10-22 に解いている（AGENTS §5）。頼まれれば、案のとおり Web の job に `pnpm audit` を足す（依存の追加ではない）。
- **F-07 を Worker で付けた理由**: Cloudflare の docs は「`_headers` は静的アセットの応答に掛かり、Worker のコードが作る応答には掛からない」までで、`env.ASSETS.fetch` の 404 は書かれていない。確かめられないので、付いていなければ付ける形にした（付いていれば上書きしない）。
- **F-06 の出典**: Version URLs は「その版の設定と資源をそのまま使う」（https://developers.cloudflare.com/workers/versions-and-deployments/version-urls/ ）。Previews は本番の設定を受け継がない（https://developers.cloudflare.com/workers/previews/ ）。`preview_urls` は省くと既存の設定を変えない（https://developers.cloudflare.com/workers/wrangler/configuration/ ）。docs は cloudflare-docs のソース（GitHub）で読んだ（このコンテナから developers.cloudflare.com は開けない）。

### 逸脱・範囲を広げたこと

- 依頼の「作業ブランチ」は S9 のデプロイ準備（PR #6。未マージ）のブランチで、その上に積んだ。PR #6 をマージすると、この修正も一緒に main に入る。
- `web/scripts/serve.mjs`（E2E の配信）と `web/src/components/App.test.tsx`（プライバシーポリシーの日付の固定値）を変えた（F-03・文書の変更に伴う）。
- 鍵の技 code の検証は、依頼の「調べてから判断」の範囲で入れた。

## 残課題・既知の問題

- **本物の Cloudflare では動かしていない**（まだデプロイしていない）。目録の読み出し（`env.ASSETS.fetch`）・`preview_urls`・404 のヘッダは、デプロイ後に確かめる（ヘッダ診断のときに `/no-such-page` を見る）。
- **無料プランの CPU 時間（1 要求 10 ms）**: 目録を覚えていない isolate の最初の投票は、manifest（約 10 KB）と moves DB（60〜90 KB）を読んで JSON を解く。手元では測っていない（未計測）。超えると 1 回だけ誤りになりうる（推測）。
- 1 分の上限は isolate の中だけ（拠点ごと）。1 日の上限は D1 で数える。
- 大きな CGNAT・会社の出口では、正当な利用者どうしで回線の上限を分け合う。多数の IP（多数の /64）を持つ相手には回線の数だけ上限を与える。
- F-05（npm の脆弱性検査）は案のまま。

## 次への入力

- デプロイ前に「開発者がやること」1（`quotas`）が必須。
- PR #6 の CI（push で走る）でこの修正の検査も回る。

## 検証（手元。Node v22.23.4・Go 1.26.10。出力はファイルへ書き、終了コードと件数で判定）

```
$ pnpm exec tsc --noEmit                         → EXIT=0
$ pnpm test                                      → EXIT=0  Test Files 18 passed (18) / Tests 197 passed (197)
$ bash scripts/prepare-assets.sh                 → EXIT=0
$ pnpm build                                     → EXIT=0  ファイル: 41(上限 20000) / 違反: 0 件
$ SKIP_ASSETS=1 bash scripts/local/build.sh      → EXIT=0
$ PW_CHROMIUM_PATH=/opt/pw-browsers/chromium-1194/chrome-linux/chrome pnpm e2e   → EXIT=0  28 passed (49.8s)  (retries: 0)
$ gofmt -l(追跡している .go 全部)                → 出力 0 バイト
$ go vet ./...                                   → EXIT=0
$ go test -count=1 ./...                         → EXIT=0  PASS 359 / FAIL 0 / SKIP 0(-v の行数)。本家の正解との突き合わせ 5208 条件(キャラ 31)・不一致 0
$ python3 scripts/check-licenses.py              → EXIT=0 違反: 0 件
$ python3 -I scripts/check-private-refs.py       → EXIT=0 違反: 0 件
$ bash scripts/check-actions-pinned.sh           → EXIT=0 違反: 0 件
$ python3 -I scripts/check-deploy-workflow.py    → EXIT=0 違反: 0 件
$ python3 -I scripts/ci/verify-plan.py check     → EXIT=0 違反: 0 件
```

手元の E2E は、このコンテナに入っている Chromium の版に合わせて `PW_CHROMIUM_PATH` を付けた（S8 と同じ）。

**報告を書いた後の再確認**（AGENTS §8-11）: `tsc` EXIT=0、`pnpm test` EXIT=0・Tests 197 passed、`pnpm build` EXIT=0・違反 0 件、`pnpm e2e` EXIT=0・28 passed（47.2s）、`gofmt -l` 0 バイト、`go vet` EXIT=0、リポジトリの検査 5 つ いずれも違反 0 件で、上と一致した。`go test` は回し直していない（この修正は Go のファイルを変えていない: `git diff --name-only 920a91d -- '*.go' go.mod go.sum` は 0 件。上の結果はこの修正を積んだ後の木で回したもの）。

### 足した試験と、わざと壊して赤を確かめた結果（AGENTS §8-3。壊し方はどれも型・ビルドが通る形）

| 試験（`web/edge/index.test.ts`） | 壊し方 | 結果 |
|---|---|---|
| UA を変えても同じ回線は 1 分の上限で 429（beacon・投票）／IPv6 の同じ /64 | 上限の鍵を利用者のハッシュに戻す | 3 件が赤 |
| UA を変えても 1 回線 1 日の上限で 429（IPv6 /64・別の回線は独立） | 回線の 1 日の上限を 1000 倍に | 1 件が赤 |
| 回線ごとの数は日替わりで消え、生の IP も回線も残らない | 日替わりで `quotas` を消さない／回線の値をそのまま保存 | それぞれ 1 件が赤 |
| 回線の鍵（IPv4・写した IPv6・/64・ゾーン ID・形の分からない値） | /64 に丸めない | 7 件が赤 |
| 票の配信の連打で 429 | 配信を limiter に通さない | 1 件が赤 |
| 配信していないキャラは 400 | キャラを確かめない | 1 件が赤 |
| moves DB に無い技 code は 400 | 技を確かめない | 1 件が赤 |
| 目録は覚え、時間が経ったら読み直す | 期限なしで覚える | 1 件が赤 |
| manifest を読めないときは 503（読めなかったことは覚えない） | — | 修正前のコードでは 200 になる形 |
| Content-Length の無い長いストリームは 413 で、上限の少し先で読むのを止める（pull の回数・cancel） | **修正前のコードで回した** | 赤（101 回読み切っていた。`expected 101 to be less than or equal to 13`） |
| 上限ちょうどの本文は受ける | 上限を 1 バイト厳しく | 1 件が赤 |
| chunk の境目で割れたマルチバイト文字 | `stream: true` を外す | 1 件が赤 |
| 静的アセットに無いパスの 404 にもヘッダ（既にあるものは上書きしない・200 は触らない） | **修正前のコードで回した**／上書きする／200 にも付ける | それぞれ 1 件が赤 |

言語の試験（キーの一致・非日本語への日本語の混入・zh-Hant の再生成との一致）は `pnpm test` の中で通った。

### `git diff --stat 999925d`（この報告のファイルを除く。S9 のデプロイ準備〔PR #6〕の差分を含む）

```
 .github/workflows/analytics.yml       |  49 ---
 .github/workflows/ci.yml              |  87 ++++-
 .github/workflows/local-release.yml   |   2 +-
 AGENTS.md                             |  11 +-
 docs/incident.md                      |  46 +++
 docs/local.md                         |   3 +
 docs/release-checklist.md             |  47 ++-
 docs/telemetry.md                     |  74 +++--
 scripts/check-deploy-workflow.py      | 135 ++++++++
 scripts/check_deploy_workflow_test.py |  67 ++++
 web/edge/catalog.ts                   |  86 +++++
 web/edge/index.test.ts                | 256 ++++++++++++++-
 web/edge/index.ts                     | 133 ++++++--
 web/edge/schema.sql                   |   9 +
 web/edge/testing/local.ts             |  11 +-
 web/edge/visitor.ts                   |  56 +++-
 web/edge/votes.ts                     |  23 +-
 web/package.json                      |   5 +-
 web/pnpm-lock.yaml                    | 588 +++++++++++++++++++++++++++++++++-
 web/pnpm-workspace.yaml               |   6 +
 web/scripts/analytics/report.mjs      |   2 +-
 web/scripts/serve.mjs                 |  11 +-
 web/src/components/App.test.tsx       |   2 +-
 web/src/components/PrivacyPolicy.tsx  |   2 +-
 web/src/locales/en.json               |   2 +-
 web/src/locales/ja.json               |   2 +-
 web/src/locales/ko.json               |   2 +-
 web/src/locales/zh-Hans.json          |   2 +-
 web/src/locales/zh-Hant.json          |   2 +-
 web/wrangler.toml                     |   9 +-
 30 files changed, 1550 insertions(+), 180 deletions(-)
```

試験のファイルは追加・増加のみ（`web/edge/index.test.ts` +243 −13。−13 は import・試験の世界〔偽の ASSETS・limiter・UA の指定〕の作り直しと、試験 1 件の名前の付け替え〔「1 人 1 分」→「1 回線 1 分」〕で、`it(` の削除は 0 件。`App.test.tsx` 1 行の置き換え、`scripts/check_deploy_workflow_test.py` は S9 で新規）。

### このセキュリティ修正だけの差分（`git diff --stat 920a91d`。この報告のファイルを除く）

```
 docs/release-checklist.md            |  18 ++-
 docs/telemetry.md                    |  26 ++--
 web/edge/catalog.ts                  |  86 ++++++++++++
 web/edge/index.test.ts               | 256 +++++++++++++++++++++++++++++++++--
 web/edge/index.ts                    | 133 ++++++++++++++----
 web/edge/schema.sql                  |   9 ++
 web/edge/testing/local.ts            |  11 +-
 web/edge/visitor.ts                  |  56 +++++++-
 web/edge/votes.ts                    |  23 +++-
 web/scripts/serve.mjs                |  11 +-
 web/src/components/App.test.tsx      |   2 +-
 web/src/components/PrivacyPolicy.tsx |   2 +-
 web/src/locales/en.json              |   2 +-
 web/src/locales/ja.json              |   2 +-
 web/src/locales/ko.json              |   2 +-
 web/src/locales/zh-Hans.json         |   2 +-
 web/src/locales/zh-Hant.json         |   2 +-
 web/wrangler.toml                    |   5 +
 18 files changed, 579 insertions(+), 69 deletions(-)
```
