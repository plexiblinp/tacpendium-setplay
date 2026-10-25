# 公開後の運用の手引き

2026-10-25 の一般公開の後に、サイトとローカル版を直し続けるための手引き。進め方の規約は `AGENTS.md` §3「公開後の進め方」で、ここは手順を書く。
作業は 3 種類ある。セキュリティ（`security`）、使いやすさ（`ux`）、SF6 の更新への対応（`data`）。Claude Code では `/maint <種類> <内容>` で始める。

## 役割

| 誰 | すること |
|---|---|
| 自動（GitHub） | 見張りと知らせ（下の表）。直すことはしない |
| Claude（セッション） | 調べる・直す・検査する・報告を書く・作業ブランチに push する。PR は頼まれたときに作る |
| 開発者 | 作業を頼む・PR をマージする・「Cloudflare へ出す」を承認する・ローカル版の Release を公開する・アカウントとトークンを管理する |

## 自動の見張り

| 見張り | 頻度 | 知らせ方 | 受けたら |
|---|---|---|---|
| Dependabot（Actions・npm・Go の依存） | 週 1 回。公開 7 日たった版だけ。security update は即時 | PR | `security`。CI が緑ならマージ（AGENTS §5） |
| `govulncheck.yml`（Go と標準ライブラリ） | 週 1 回（月曜 09:41 JST）と、CI の Go の job で毎回 | Actions の失敗通知 | `security` |
| CodeQL | PR と main への push | PR の指摘・Security タブ | `security` |
| Secret scanning・Push protection | 常時 | GitHub の通知・push の拒否 | `docs/incident.md` |
| `upstream-watch.yml`（公開版 tacpendium の変化） | 日 1 回（09:13 JST） | Actions の失敗通知（変化があると赤） | `data` |
| Issues・Google フォーム | 随時 | GitHub の通知・フォームの回答 | `ux`（プライバシーの問い合わせは開発者が答える） |

Actions の失敗通知は GitHub の Settings → Notifications → Actions で「Only notify for failed workflows」をメールか GitHub Mobile で受ける（推測: 既定で有効。開発者が一度確かめる）。

## デプロイ

1. PR をマージすると main の CI が走る。Go・Web の job が緑になると「Cloudflare へ出す」が承認待ちになる。
2. Actions の該当の run で **Review deployments → production → Approve and deploy**。
3. 出たら、job の Summary の版の ID を確かめ、サイトを開いて変えたところを見る。

- **承認しないまま次の PR をマージしない**。main の実行は同時に 1 本で、途中で打ち切らない（`ci.yml` の `concurrency`）。承認待ちが 1 本残っていると、後の実行は job 0 件の pending のまま止まる。待てるのも 1 本だけなので、さらに後の実行が来ると、待っていた実行は cancelled になる（S9 で実際に起きた）。
  - 止まったら、古い承認待ちの run を **Cancel workflow**（または Review deployments → Reject）。すると最新の実行が走る。最新の main には前の変更も入っているので、出すのは最新の 1 回でよい。
  - 出さなくてよい変更（文書だけ・ビルドの道具だけ）も、承認待ちは Cancel で片付ける。
- 出し直しは Actions →「CI」→ Run workflow（main）。検査をすべて回し直してから出す。
- 前の版に戻す・止めるのは `docs/incident.md`。

## security — セキュリティの更新

### Dependabot の PR

- CI が緑なら開発者がマージする。メジャー版だけは、Claude がリリースノートで入力・挙動の変更を確かめる（AGENTS §5）。
- 何本か溜まったら、1 本をマージする → 残りを「Update branch」→ CI → マージ、を繰り返す（ruleset が main に追いついていることを求める）。デプロイはまとめて最後の 1 回でよい（上の「デプロイ」）。
- `pnpm audit` の high（とくに実行時の依存＝`dependencies`）は、Dependabot の PR を待たずに `security` の作業にしてよい。

### Dependabot が上げないもの（月に 1 回、Claude が点検する）

| もの | 版を書いている場所 | 調べるところ |
|---|---|---|
| Node | `.github/workflows/ci.yml`・`local-release.yml` の `node-version`、`web/package.json` の `engines.node` | nodejs.org のセキュリティリリース |
| Go の toolchain | `go.mod` の `go` の行 | govulncheck・go.dev の security の告知 |
| pnpm 本体 | `web/package.json` の `packageManager` と `engines.pnpm` | pnpm の Security Advisories（全件）と CHANGELOG |
| wrangler | `web/package.json`（Dependabot が上げる。メジャー版は手で） | |

- どれも、公開から 7 日たった修正版を選ぶ（依存の `minimumReleaseAge` と同じ考え方）。急ぐ修正で 7 日を待てないときは、報告に理由を書く。
- 依存を足す・上げるのは Node ≥ 22.23.4 の環境で（AGENTS §5）。
- 点検の結果は、変えるものが無くても `docs/progress/<日付>-security-toolchain-report.md` に 5 行ほどで残す（調べた版と出典）。

### Go の修正（標準ライブラリを含む）のとき

- サイトは WASM を CI で作り直すので、マージしてデプロイすれば直る。
- **ローカル版**（`go:embed` の単一バイナリ）は作り直さないと直らない。修正がローカル版に効くとき（`net/http`・`crypto/*` など。govulncheck の到達性で判断）は、main に `v<版>` のタグ → draft の Release → 開発者が照合して公開（`docs/local.md`「配布」。S9 の v1.0.1 と同じ）。

### 弱点を見つけた・知らされたとき

- public のブランチで直すと、直す前に弱点が見えてしまう。利用者に害が出うるもの（票の受け口・ヘッダ・配信）は、GitHub の **Security → Advisories → New draft security advisory** の非公開の fork（temporary private fork）で直し、マージと公開を同時に行う。
- 害が出ないもの（テストの中だけの依存など）は普通の PR でよい。
- 差し替え・漏えいが起きたら `docs/incident.md`。

### 期限のあるもの

| もの | 期限 | すること |
|---|---|---|
| Cloudflare のデプロイ用トークン | **2027-01-22** | 1 週間前に作り直し、Environment `production` の secret を上書き（`docs/telemetry.md`「デプロイ用の API トークン」）。以後も 3 か月ごと |
| GitHub・Cloudflare の回復コード | 無し | 保管場所を年 1 回確かめる |

## ux — 使いやすさの改善・不具合の修正

1. Issues・フォームの内容を、再現の手順（キャラ・KA・条件・URL・ブラウザ）に起こす。URL を共有してもらえば条件が再現できる。
2. 直す前に、その不具合で赤くなるテスト（単体か E2E）を足して赤を確かめる（AGENTS §8-3）。
3. 文言を変えたら全ロケール（ja・en・ko・zh-Hans・zh-Hant）と docs を grep する（AGENTS §8-8・§8-9）。
4. 画面の構成・既定値・票の扱いなど、利用者に見える仕様を変えるときは、実装の前に案を開発者に出す（AGENTS §3）。
5. 計測・票・ログで扱うデータが変わるなら、プライバシーポリシー（全ロケール）と `PRIVACY_LAST_UPDATED` も変える。
6. 主要フローの E2E と、サイトの配信の上限（`pnpm build` の検査）を通す。

## data — SF6 の更新への対応

更新は次の順に流れてくる。単体版は**公開版 tacpendium だけ**を取込元にする（HANDOVER D2。本家 combomgr は読まない）。

```
SF6 の更新（新キャラ・技の性能の変更）
  → 本家でデータと計算を直す
  → 公開版 tacpendium のリリース
  → upstream-watch.yml が赤（日 1 回）
  → セッションで /sync_upstream <SHA>（取り込み・検査・upstream/tacpendium.lock の更新・報告）
  → PR → マージ → 「Cloudflare へ出す」を承認
  → （データが変わったら）ローカル版の新しいタグ
```

- **新キャラクターは、ゲームでの配信開始日から 2 週間たってから**取り込む（README「新キャラクターへの対応」）。既存キャラの調整はすぐ取り込んでよい。分けられない版は開発者に聞く（`/sync_upstream` の手順）。
- 本家で扱える技が増えた・計算が変わったときも同じ流れになる。計算に効くコードの変更は `scripts/ci/verify-plan.py ack` で影響を記録する（`/sync_upstream` §3）。
- 新キャラでは、データの外（全ロケールの文言・`web/index.html` の OGP・README・docs）にキャラの数や名前を書いた箇所が無いかも grep で確かめる。
- データが変わるとローカル版の中身（埋め込みのデータ）も古くなる。大きな更新（新キャラ・バランスパッチ）のときは、ローカル版の新しいタグを出す。
- SF6 の大きな更新は、デプロイ用トークンの作り直し（3 か月）と時期が重なることが多い。同じ日にまとめてよい。

## 更新履歴（画面の「更新 …」）

利用者に見える変更を出すデプロイごとに、**1 件・1 行**を足す（開発者の要望 2026-10-25。長くなるなら目玉だけ）。画面は新しい 1 件だけを薄く出し、押すと直近 10 件（`CHANGELOG_SHOWN`）を開く。

1. `web/src/lib/changelog.ts` の `CHANGELOG` の**先頭**に `{ date: "YYYY-MM-DD", key: "<名前>" }`。日付はサイトに出る日（マージしてデプロイする日の見込み。ずれたらデプロイの前に直す）。
2. 全ロケールの `changelog.items.<名前>` に文（ja・en・ko・zh-Hans は手で、zh-Hant は `pnpm gen:locales`）。
3. 10 件を超えた古いものは、`CHANGELOG` とロケールの両方から消してよい（画面には出ないので残しても害は無い。試験はキーの過不足を見る）。
4. 文書だけ・ビルドの道具だけ・依存の更新だけのデプロイには足さない。セキュリティの修正は、利用者に関係があるときだけ「セキュリティの修正」と短く書く（中身は書かない）。

## 記録

- 作業の報告: `docs/progress/<YYYY-MM-DD>-<種類>-<短い名前>-report.md`（公開版の取り込みは `upstream-<短縮SHA>-report.md`）。
- 事故: `docs/incident.md` の手順で対応し、報告を `docs/progress/<日付>-incident-<名前>-report.md` に書く。
