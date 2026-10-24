# 計測と投票（S6）

HANDOVER §7.1〜§7.3・D3・D14・D17。開発者決定（2026-10-12）: ダッシュボードは**集計スクリプト**、票は **D1 ＋ Worker から配信**、スコアは **good − bad**、票の単位は **キャラ＋KA**。

提案の計算は利用者のブラウザ（WASM）で行う。ここに書く受け口（動くコード）が止まっても、提案は止まらない（票なしの並び＝本家の順、計測なし）。

## 構成

```
画面(web/src)
  「提案を出す」→ GET /api/votes?char=&ka=（1.5 秒で打ち切り）→ WASM のエンジン(votes を渡す) → 結果
              → navigator.sendBeacon(POST /api/beacon)        … 実行ごとに 1 回（抽出率は /config.json）
  行の 👍 / 👎 → POST /api/vote
        ▼
Cloudflare Worker(web/edge/index.ts。/api/* だけが来る。ほかは静的アセット web/dist)
  /api/beacon → 検証(zod・1KB 以下) → Analytics Engine に 1 点（dataset setplay_runs）
  /api/vote   → 検証(形・配信しているキャラと技)→ D1(votes・ballots・quotas)
  /api/votes  → D1(votes) の (char, ka) の行
        ▼
集計: web/scripts/analytics/report.mjs（SQL API → Markdown の表）。手元でだけ回す（S9。public の Actions のログ・summary は誰でも読めるため）
```

| ファイル | 中身 |
|---|---|
| `web/src/shared/api.ts` | 受け口の形（zod）。画面は送る前、Worker は受けた後に同じ形で検証する |
| `web/edge/index.ts` | Worker の入口・経路・回数制限・Origin の検査・本文を上限つきで読む |
| `web/edge/catalog.ts` | 配信しているキャラ・技の目録（静的アセットの manifest・moves DB。票の受け口の検証。S9 のセキュリティ修正 F-03） |
| `web/edge/beacon.ts`・`beacon-layout.json` | beacon → Analytics Engine の 1 点。列の順の正本は `beacon-layout.json`（集計も同じファイルを読む） |
| `web/edge/votes.ts`・`schema.sql` | 票の保存・配信・D1 のスキーマ |
| `web/edge/visitor.ts` | 日替わりソルト・利用者のハッシュ・回線のハッシュ |
| `web/edge/testing/` | 試験・E2E 用の偽物（D1 = `node:sqlite`、Analytics Engine = メモリ）と、Worker を Node で動かす `local.ts` |
| `web/wrangler.toml` | Worker の設定（`database_id` は開発者が作った D1 のもの。S9） |
| `web/src/lib/telemetry.ts`・`votes.ts` | 画面の beacon・票 |
| `web/src/components/PrivacyPolicy.tsx` | プライバシーポリシー（5 言語。フッターと `#privacy`） |

## 実行計測（beacon）

- 送るとき: 「提案を出す」ごとに 1 回。結果・誤り・中断・キャッシュからの表示のどれも送る（`outcome`・`cacheHit`）。
- 中身（`beaconSchema`）: キャラ・KA・モード・N/G/remain（モードで当てはまらないものは null）・安全飛びの種類・当てる技の種別・特定の技・ダメージ 0・絞り込み 3 つ・並び順・表記・言語・結果（`outcome`・誤りの code）・総数・受け取った件数・打ち切り・キャッシュ・所要（押してから結果まで `ms`、Worker の計算だけ `computeMs`）・票の読み込み（`votesStatus`・`votesApplied`）・データ版・抽出率。
- **null と 0 を区別する**（AGENTS §8-7）: Analytics Engine の double では null を **-1**、blob では **空文字**にする。`nMax` の 0 は「上限なし」（本家と同じ）で -1 とは別。当てる技の種別の空（blob が空文字）は「当てはまらない（有利調整・特定の技）」（画面は 0 個では押せない）。
- プライバシーポリシーの「実行の計測」は**要約**（開発者の目視の要望 2026-10-17: 長いと読まれない）で、全項目の一覧は本書を指す。項目を足したら下の対応も直し、要約に収まらない種類の項目（例: 新しい識別子）なら 5 言語の `privacy.usageItem` も直し、`PRIVACY_LAST_UPDATED`（`web/src/components/PrivacyPolicy.tsx`）を更新する。文面は開発者が依頼したレビューの修正案（2026-10-17。法的には問題なし・信頼のための説明という位置づけ）。対応: キャラ=`char`／有利フレーム=`ka`／モードと絞り込みの条件=`mode`〜`sort`／表記と言語=`notation`・`lang`／実行の結果=`outcome`・`errorCode`／件数と打ち切り=`resultCount`・`items`・`truncated`／計算済みの結果=`cacheHit`／時間=`ms`・`computeMs`／グッド/バッドの読み込み=`votesStatus`・`votesApplied`／データ版=`dataVersion`／送る割合=`sampleRate`／利用者の区別のハッシュ=`visitor`（Worker が付ける）。
- 抽出率: `web/public/config.json` の `beaconSampleRate`（0〜1）。混雑時に下げる（静的ファイルの書き換え＝再デプロイ）。読めなければ 1（全部送る）。集計は `_sample_interval / sampleRate` で割り戻す。
- Worker: 本文 1KB 以下・JSON・`beaconSchema` に合う（知らない項目は落とす）・別のオリジンの POST は 403・**1 回線 1 分 120 回**（鍵は回線のハッシュ。isolate の中だけの簡易な制限。ハッシュを作れないときは isolate の全員で 1 分 600 回）。本文は読みながら数え、1KB を超えた時点で読むのをやめて 413（Content-Length が無くても全部は読まない。S9 のセキュリティ修正 F-04）。Analytics Engine の束縛が無ければ書かずに 204。
- 列の対応（`beacon-layout.json`。blob と double の番号は配列の位置 + 1。index は `char`）: 列を足すときは末尾に足し、順を変えない（過去の点の読み方が変わる）。形を変えたら `BEACON_SCHEMA_VERSION` を上げる（集計は `double1 = 1` で絞っている）。

## 票（グッド/バッド。D17）

- 票の鍵: エンジンの `RecipeKey`（`internal/service/setplay/votes.go`）= 「当てる技以外の手の技 code を `,` で連結」`|`「当てる技の code」（有利調整は空）。KA・N・G は入れない。データ版が変わっても同じ技の同じ手順なら同じ鍵。エンジンが各行に `recipeKey` を付け、画面はそれで送り・引く（鍵の作り方は Go の 1 か所）。
- 単位: キャラ＋KA ごと（同じ手順でも KA が違えば別の票）。
- スコア: good − bad（`voteScore`）。票なし＝0。
- 並べ替え: エンジンが本家の並べ替えの**後**にスコアの降順で**安定に**並べ直し、その後で `limit` を切る（AGENTS §4「並べ替えの後で切る」）。票の無い行・同点の行は本家の順のまま。票が無ければ並びも中身も本家と同じ（本家との突き合わせ・golden が見る経路）。
- 画面: 計算の前に `GET /api/votes` を読む（1.5 秒で打ち切り。誤り・時間切れは票なし＝`votesStatus: unavailable` で計算し、その旨を出す）。スコア 0 の鍵はエンジンに渡さない。押した票はその場では行を動かさず、次の検索から効く（覚えた結果は投票で捨てる）。
- 重複防止: `hash(IP + UA + 日替わりソルト)` × レシピ × 日で 1 票（同じ日は付け替え・取り消し可）。1 人 1 日 200 レシピまで。日をまたぐと同じ人がもう 1 票付けられる（§7.2 の案のとおり）。
- 回線ごとの上限（S9 のセキュリティ修正 F-01・F-02）: UA は送る側が自由に変えられるので、上の「1 人」だけでは UA を変えるたびに別の人になる。回線のハッシュ（下）で、**投票は 1 回線 1 分 60 回・1 日 1,000 票**（新しい票の数。`quotas` に数え、取り消しても戻らない）、**票の配信は 1 回線 1 分 120 回**に抑える。値は共有 IP（家庭・学校・携帯の CGNAT）で数人が同時に使っても当たらない大きさにした。
- 受ける票は、配信しているキャラ・そのキャラの moves DB にある技 code の鍵だけ（`web/edge/catalog.ts`。無いキャラ・技は 400、目録を読めないときは 503。F-03）。
- 配信: `GET /api/votes?char=&ka=` → `{"votes": {"<鍵>": [good, bad]}}`。主キー `(char, ka, recipe)` の前方一致で、そのキャラ・KA の行だけ読む（D1 の「読んだ行」を抑える。S1 ⑦）。0・0 の行は出さない。上限 10,000 件（`|good − bad|` の大きい順）。**キャッシュさせない（`no-store`）**: 投票の直後の検索がブラウザに残った古い票で並ぶ不具合を E2E で見つけたため。

## 利用者のハッシュと日替わりソルト（D14）

- `visitor = sha256(IP + "\n" + UA + "\n" + ソルト)` の先頭 32 桁。IP は `CF-Connecting-IP`。**生の IP は保存しない**（試験で、点・投票の記録に IP の文字列が入らないことを見る）。
  - UA を混ぜる理由: 同じ回線（家庭の Wi-Fi・CGNAT）の別の人を、推定利用者数と票の重複防止で別の人として数えるため（IP だけでは 1 回線を 1 人にまとめてしまう）。
- 回線のハッシュ `net = sha256("net" + "\n" + 回線 + "\n" + ソルト)` の先頭 32 桁（S9 のセキュリティ修正 F-01）。回線は IPv4 ならアドレス、IPv6 なら先頭 /64（1 つの契約に配られる単位）、IPv4 を写した IPv6 は IPv4。回数制限と回線ごとの 1 日の票の数（`quotas`）の鍵で、Analytics Engine には書かない。生の IP も回線の値も保存しない（試験で見る）。
  - IP 単位の上限を置く理由: UA を変えるだけで `visitor` が変わり、利用者単位の上限を逃れられたため（監査の指摘 F-01）。IP だけにしなかった理由は上の「UA を混ぜる理由」。
- ソルトは D1 の `salts` に UTC の日付ごとに 1 つ（32 バイトの乱数）。**日が変わった後の最初の要求で**（遅延削除）、前日以前のソルト・投票の記録（`ballots`）・回線ごとの票の数（`quotas`）を消す。要求が来なければ残る。秘密情報（環境変数）は要らない。
- **D1 の Time Travel は常に有効で、無料プランでは過去 7 日の任意の時点にデータベースを戻せる**（S7 で Cloudflare の docs で確認。`docs/progress/S7-privacy-research.md` §2.1。S9 のセキュリティ修正 F-08 で 2026-10-22 に再確認: 無料 7 日・有料 30 日。https://developers.cloudflare.com/d1/reference/time-travel/ ）。つまり消したソルトは 7 日の間、アカウントの持ち主が戻せ、戻せば Analytics Engine に 3 か月残る `visitor` から総当たりで IP を当てうる。画面のプライバシーポリシーは「通常は日をまたいで結び付けられない。ただし消したデータは Cloudflare の復旧機能で 7 日間は戻せる状態で残る」と書く（S7。開発者とレビュー担当の判断で、仕組みは変えず文を直した）。
- Workers Logs（要求の記録。IP を含みうる）は `web/wrangler.toml` の `[observability] enabled = false` で止めている（S7）。デプロイ後に、ダッシュボードで記録が残っていないことを確かめる。
- D1 が誤りを返すとき（無料枠の超過等）も beacon は書く（`visitor` は `unavailable`。D1 の束縛が無いときは空文字）。票の障害で計測を止めない（レビュー中-1）。
- beacon の `visitor` は「日ごとの推定利用者数」（その日のハッシュの種類数）に使う。日をまたいでは数えられない（ソルトが変わるため）。

## 集計（ダッシュボードの代わり）

```sh
cd web
CLOUDFLARE_ACCOUNT_ID=… CLOUDFLARE_API_TOKEN=… node scripts/analytics/report.mjs 7   # 直近 7 日
```

**手元でだけ回す**（S9 の開発者決定 2026-10-20）。以前の `.github/workflows/analytics.yml` は削除した。public のリポジトリでは Actions のログと job summary を誰でも読めるので、集計を出すとプライバシーポリシーの「第三者に渡さない」と利用者の期待からずれるため。集計用のトークンも GitHub に置かずに済む。

トークンをコマンドの文字列に書くと shell の履歴に残る。打ち込んで見えない形で読む（例 bash `read -rs CLOUDFLARE_API_TOKEN && export CLOUDFLARE_API_TOKEN`、PowerShell 7 `$env:CLOUDFLARE_API_TOKEN = Read-Host -MaskInput`）。依存は Node の組み込みだけなので、`pnpm install` は要らない。

出す表: 日ごとの実行数と推定利用者数／**KA 80 以上の実行の割合**（S5 後の開発者決定 1: D を作るかの判断材料）／KA の帯ごとの実行数と待ち時間（中央値・95%・最大）／キャラ別／モード別／言語・表記別／結果と誤りの code／0 件の割合／票の読み込み。

## デプロイの手順（開発者が行う。S9）

公開・アカウント・秘密情報（AGENTS §3 の 3〜5）なので、Cloudflare と GitHub の操作は開発者が行う。デプロイは **GitHub Actions の `ci.yml` の `deploy` job からだけ**（AGENTS §8-12。S9 の開発者決定 2026-10-20: 方式 (a)＝Actions ＋ スコープを絞った API トークン ＋ 承認者付きの Environment）。

1. Cloudflare のアカウント（**カードを登録しない**。無料プラン）と 2FA。
2. D1 を作る（Cloudflare の画面の Storage & Databases → D1 SQL Database。S9 は画面で作った）→ `database_id` を `web/wrangler.toml` に入れる（秘密情報ではない）。
3. `edge/schema.sql` を D1 に流す（画面の D1 の Console。表を足したら、その `CREATE TABLE` をデプロイの前に流す。S9 では `quotas` を後から足した）。
4. **Analytics Engine を有効にする**（アカウントで 1 回。画面の Analytics Engine で **Create Blank Dataset** → 名前 `setplay_runs`・Dataset Binding `TELEMETRY`。`wrangler.toml` と同じ名前にする）。有効にしていないと `wrangler deploy` が「You need to enable Analytics Engine」（code 10089）で止まる（S9 の初回で実際に止まった）。無料プランは 1 日書き込み 10 万件・読み取り 1 万回で、カード未登録なら超えても止まるだけ（https://developers.cloudflare.com/analytics/analytics-engine/pricing/ ）。
5. デプロイ用のトークン（下の「デプロイ用の API トークン」）を作り、GitHub の Environment `production` の secrets に `CLOUDFLARE_API_TOKEN`・`CLOUDFLARE_ACCOUNT_ID` を置く。
6. main に入った変更は、CI（`ci.yml`）の Go・Web の job が緑になると `deploy` job が承認待ちになり、承認の後に出る。出し直しは Actions →「CI」→ Run workflow（main。検査をすべて回し直してから出す）。

`deploy` job の作り（`.github/workflows/ci.yml` の冒頭と job のコメントが正本）:

- 起動は main への push と手動起動（main）だけ。Go・Web の job の成功を待ち（`needs`）、Web の job で作り E2E を通した `dist`（成果物 `site-dist`。同じ条件のときだけ作り 1 日で消える）を出す。ビルドはしない。
- Environment `production`（承認・main だけ）を通る。トークンは `wrangler deploy` の 1 段だけに渡し、install のスクリプトは動かさない。
- 別の workflow（`workflow_run`）にしなかった理由: フォークの PR の CI の完了でも起動し main の扱いで動く。CodeQL がキャッシュ汚染（actions/cache-poisoning）として指摘した（S9。PR #6）。守りが消えたら `scripts/check-deploy-workflow.py`（CI のリポジトリ検査）が赤になる。
- `wrangler` は `web/package.json` の devDependency（lockfile で固定。`pnpm-workspace.yaml` の `minimumReleaseAge` で公開 7 日未満の版は入らない）。

無料枠を超えたとき: Worker は 1 日 10 万リクエストを超えると Error 1027（S1 ⑥。静的アセットへの要求は Worker を通らず無料・無制限なので、止まるのは `/api` だけの見込み）。D1 は UTC 0 時まで誤り（票は読めず `unavailable`、投票は 503）。Analytics Engine の超過時の挙動は docs に記載が無い（S1 ⑥。実アカウントで確かめる）。どれも課金には移らない（カード未登録）。

### デプロイ用の API トークン

- 作り方: Cloudflare の画面の **Manage Account → Account API Tokens**（アカウント所有のトークン）→ Create Token。名前は `github-actions-deploy-tacpendium-setplay`。**有効期限は 3 か月**（開発者決定 2026-10-20。SF6 の大きな更新の周期に合わせ、更新のたびに作り直す）。接続元の IP の制限は付けない（GitHub Actions の IP は変わる）。
- 権限（2026-10 の新しい画面の「Permission policies」。S9 で実際に通した形。Legacy の「Workers Scripts」は使わない＝開発者決定 2026-10-22）:
  - **Worker を新しく作るとき（初回だけ）**: Developer Platform の **Workers: Admin**、範囲は Entire Account。`Workers: Editor` では作成できず、`assets-upload-session` が「No access to the specified resource」で止まる（S9 の初回で実際に止まった）。Cloudflare の docs も「Editor は既存の Worker の更新・デプロイだけで作成・削除はできない。作成には Workers 全体の Admin が要る」と書く（https://developers.cloudflare.com/workers/authorization/workers/ ）。
  - **作成の後（普段）**: 範囲を **Specified Workers → tacpendium-setplay**、方針を **Individual Workers: Editor** に絞る（2026-10-24 に絞り、その状態で CI からのデプロイが通ることを確かめた。漏れたときの被害がこの Worker の差し替えまでに縮む）。
  - **D1・Analytics Engine の権限は付けない**（束縛を持つ Worker のデプロイに、束縛先の権限は要らない。上の docs。S9 で実際に要らなかった）。
  - Worker を作り直す・別の Worker を作る必要が出たら、そのときだけ Workers: Admin に戻し、済んだら絞り直す。
- 置き場所: GitHub の **Environment `production` の secrets だけ**（Repository secret・Variables には置かない。Variables は秘密ではない）。Environment には必須の承認者（開発者）と「main だけ」を付け、Prevent self-review は付けない（一人では承認できなくなる）。
- 期限が切れたとき: デプロイが認証の誤りで止まるだけで、サイト・票・計測は残る。新しいトークンを作って secret を上書きする。
- 漏れたとき: `docs/incident.md`。

### 集計用の API トークンの扱い

**作り方**: Cloudflare の画面の Account API tokens → Create Token → Custom。権限は **Account → Account Analytics → Read の 1 つだけ**、対象のアカウントを 1 つに絞り、有効期限を付ける。デプロイ用とは別のトークンにする。

**置き場所**: 開発者の手元だけ（S9 で集計を手元に移した。GitHub には置かない）。使うときに環境変数で渡し（上の「集計」）、ファイル・チャット・コマンドの文字列には書かない。

**漏れた場合に起きること（Account Analytics: Read だけのトークン）**

- できること: そのアカウントの分析データを**読む**こと（本サービスの計測＝キャラ・KA・条件・件数・時間・日替わりのハッシュ。推測: Account Analytics の権限は同じアカウントのほかの分析〔Workers のリクエスト数等〕も読める）。計測に生の IP は入れていない。ハッシュは、その日のソルトが消えた後は IP を当てにくくなる（ソルトの削除は日付が変わった後の最初の要求で行う遅延削除で、D1 の復旧用の記録に残る可能性がある＝上の「利用者のハッシュと日替わりソルト」）。
- できないこと: サイトの書き換え・デプロイ、D1（票）の読み書き・削除、設定・課金の変更。
- 起きうる実害: 読み取りの問い合わせを大量に投げられ、無料枠（Analytics Engine の読み取り 1 日 1 万回。S1 ⑥）を使い切られ、その日の自分の集計が失敗する。課金はされない（カード未登録）。
- 対処: Cloudflare の画面でそのトークンを **Roll（再発行）か Delete** すれば即座に無効になる。

デプロイ用のトークン（上の「デプロイ用の API トークン」）は権限が強い（Worker を差し替えられ、差し替えた Worker から D1 の票も読み書きできる）。漏れたときの手順は `docs/incident.md`。

## 検査

| 検査 | どこで |
|---|---|
| 票が第一キー・limit の前・票なしは不変・鍵の重なりなし・繋ぎだけ | `engine/votes_test.go`、`internal/service/setplay/votes_test.go` |
| Go → WASM → JS（4 番目の引数） | `scripts/wasm/wasm.test.mjs` |
| Worker（検証・Analytics Engine の列・null の扱い・回数制限・Origin・票の付け替え／取り消し／日替わり／上限・IP が残らない・D1 の誤りで 503・索引） | `web/edge/index.test.ts`（D1 は `node:sqlite` で同じ SQL） |
| 画面（beacon の中身・抽出率・誤りと中断・票の取得と渡し方・投票・失敗・キャッシュを捨てる・プライバシーポリシー） | `web/src/components/App.test.tsx`、`web/src/lib/telemetry.test.ts`・`votes.test.ts` |
| 集計スクリプト | `web/scripts/analytics/report.test.mjs` |
| 通し（画面 → sendBeacon → Worker → Analytics Engine／画面 → D1 → WASM → 画面） | `web/e2e/telemetry.spec.ts`（`scripts/serve.mjs` が Worker の本物のコードを `dist-edge/local.mjs` で動かす） |
| Worker が束ねられる（Node の組み込みに頼らない） | `pnpm build` の最後の `scripts/build-edge.mjs`（`dist-edge/worker.mjs`） |

## 限界

- **本物の Cloudflare では動かしていない**（アカウントが無い）。D1 は `node:sqlite`、Analytics Engine はメモリの偽物で、D1 の制限・遅延・無料枠、Analytics Engine の SQL の方言は再現しない。集計の SQL は docs（cloudflare-docs の SQL reference）の関数だけを使ったが、実アカウントで流していない。
- 回数制限（1 分）は isolate の中だけ（拠点・時間で別の isolate になる）。厳密な上限ではない。1 日の上限（利用者 200・回線 1,000）は D1 で数えるので isolate をまたいで効く。
- 回線ごとの上限は、多くの人が 1 つの IP を共有する回線（大きな CGNAT・会社の出口）では、正当な利用者どうしで上限を分け合う。逆に、多数の IP を持つ相手（ボットネット・IPv6 の /64 を多数持つ相手）は回線の数だけ上限を得る。どちらも「水増しを 1 回線分に抑える」以上の保証ではない。
- 同じ人の同時の投票が重なると数がずれうる（読んでから書く 2 段。排他はしない。開発者注: 正確さは要らない）。
- 票の受け口が確かめるのは「配信しているキャラか」と「鍵の技 code がそのキャラの moves DB にあるか」まで（`web/edge/catalog.ts`。静的アセットの manifest・moves DB を Worker が読み、isolate の中で 5 分覚える。読めないときは 503）。**技の組み合わせが実際の提案に出るか**は確かめない（取りうる鍵は 1 キャラ・KA 0〜100 で数百万件・数百 MB の見込みで、配信の上限に収まらない。`docs/progress/fix-security-report.md`）。
  - 実在しない組み合わせの票は、どの行にも当たらないので、それ自体で並びは変わらない。ただし配信は 1 キャラ・KA あたり |good − bad| の大きい順に 10,000 件までなので、**その数を超える偽の鍵に本物より大きいスコアを付けると、本物の票が配信から落ちうる**（＝並びに間接的に効く）。これは回線ごとの上限（1 回線 1 日 1,000 票）で抑える: 本物のスコア 1 の票を落とすには、偽の鍵 10,001 件に 2 票ずつ（約 2 万票）＝約 20 回線・1 日が要る。
- エンジンの安全上限（target あたり 6000 件・単位 2000・連鎖 8）は票より前に掛かる。KA 64 以上で上限に落ちた行は、票があっても出ない（S1 ⑧。上限は本家と同じ）。
- 日をまたぐと同じ人が同じレシピにもう 1 票付けられる（「× 日で 1 票」の設計どおり）。
