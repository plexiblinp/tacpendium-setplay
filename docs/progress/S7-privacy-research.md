# S7 プライバシーの調査（D1 Time Travel・個人情報保護法・外部送信規律・GDPR/ePrivacy・Cloudflare 規約）

> **法的助言ではない。一次資料の引用と論点の整理**（調査日 2026-10-18。読み取りのみ。コード・ポリシーは変えていない）

## 0. 調査の条件と限界（先に読む）

- このセッションのネットワークでは、`developers.cloudflare.com`・`www.ppc.go.jp`・`laws.e-gov.go.jp`・`www.soumu.go.jp`・`edpb.europa.eu`・`eur-lex.europa.eu`・`www.cloudflare.com` が**プロキシに拒否された**（CONNECT 403。WebFetch も名前解決できない）。
- **Cloudflare の docs は一次資料を読めた**: docs の正本リポジトリ `cloudflare/cloudflare-docs` の `production` ブランチを `raw.githubusercontent.com` から取得（2026-10-18。コミット SHA は GitHub API が使えず未取得）。下では `CF-docs:<パス>` と書く（= `https://raw.githubusercontent.com/cloudflare/cloudflare-docs/production/src/content/docs/<パス>`）。
- **法令・当局の資料（日本・EU）は本文を読めていない。** URL は WebSearch の結果で存在と題名を確かめただけで、引用の文言は検索の抜粋か、私の記憶にもとづく。**ここは「未確認」と書く。** 開発者が判断する前に、§6 の URL を人の手で開いて確かめること。
- 対象のコード: `web/edge/visitor.ts`・`votes.ts`・`index.ts`・`schema.sql`・`beacon.ts`・`web/wrangler.toml`、`web/src/locales/ja.json` の `privacy.*`、`docs/telemetry.md`。

## 1. 何を集めているか（コードで確かめた内容）

| データ | 置き場 | 中身 | 残る期間 |
|---|---|---|---|
| beacon（実行計測） | Analytics Engine `setplay_runs` | キャラ・KA・条件・言語・件数・時間・`visitor` | 3 か月（下 §2.2） |
| `visitor` | 上の beacon の列、D1 `ballots.voter` | `sha256(IP + "\n" + UA + "\n" + salt)` の先頭 32 桁（128 ビット）。IP は `cf-connecting-ip`（`index.ts:86`） | beacon は 3 か月、`ballots` は翌日の最初の要求まで |
| salt | D1 `salts` | 32 バイトの乱数。日付（UTC）ごとに 1 行 | **日が変わった後の最初の要求まで**（`visitor.ts` の遅延削除。要求が来なければ残る） |
| 票の合計 | D1 `votes` | キャラ・KA・レシピごとの good/bad の数（誰の票かは無い） | サイトを続ける間 |

ポリシーの該当文（`ja.json` `privacy.hashBody`）:「IP アドレスそのものは保存せず、乱数は毎日消すため、日をまたいで同じ人を結び付けることはできません。」

**食い違いを 1 つ見つけた**: `docs/telemetry.md` の「利用者のハッシュと日替わりソルト」には「プライバシーポリシーは『消したデータが基盤の復旧用の記録に一定期間残ることがある』と書いた」とある。しかし、いまの `privacy.*`（5 言語）にこの文は**無い**（`web/src/locales/` 全体で「復旧」を grep して 0 件。ほかの 4 言語の訳語では探していない）。どちらかを直す必要がある。

## 2. 問 1・問 5: Cloudflare 側の事実

### 2.1 D1 Time Travel（CF-docs: `d1/reference/time-travel.mdx`・`d1/platform/limits.mdx`）

逐語の引用:

- "You do not need to enable Time Travel. It is always on."
- "Time Travel automatically creates bookmarks on your behalf."
- "You can restore a database back to a point in time up to 30 days in the past (Workers Paid plan) or 7 days (Workers Free plan)."
- limits 表: "Time Travel duration (point-in-time recovery) | 30 days (Workers Paid) / 7 days (Free)"
- "Restoring a database to a specific point-in-time is a _destructive_ operation, and overwrites the database in place."
- 対象: "Databases with `version: production` support the new Time Travel API."（新しく作る DB はこちらのはず。推測: `wrangler d1 info` で確かめる）

ここから言えること:

1. **常に有効。** 無効にする設定は、読んだ 2 ファイルに書かれていない（「無い」と言い切れるのはこの 2 ファイルの範囲だけ。`wrangler d1` の他のページは見ていない）。
2. **無料プランでは 7 日前まで、DB 全体をその時点へ戻せる。** 戻せば、消したはずの `salts` の行（と `ballots`）も戻る。戻す権限はアカウントの持ち主（＝運営者）にある。
3. 戻すと DB 全体が上書きされる（票の合計も過去に戻る）。ただし "To undo this operation, you can restore to the previous bookmark" とあるので、**戻す → salt を書き写す → 元に戻す、の往復が運営者にはできる**（推測: 手順から読める。試していない）。
4. 過去の salt が手に入ると、Analytics Engine に 3 か月残る `visitor` から IP を総当たりで当てうる。推測: IPv4 は 2^32 通りしかなく、UA はよくある文字列の候補で足りるので、1 日分なら GPU 1 台で現実的な時間に収まる。**128 ビットのハッシュは、候補が当たったとき 1 つに決まる**（偶然の一致はほぼ起きない）。

### 2.2 Analytics Engine の保存期間（CF-docs: `analytics/analytics-engine/limits.mdx`）

- "Data written to Workers Analytics Engine is stored for three months."（ポリシーの「最長3か月」と合う。プランによる違いは書かれていない）

### 2.3 Workers Logs（新しく分かったリスク。CF-docs: `workers/observability/logs/workers-logs.mdx`）

- "All newly created Workers will come with the observability setting enabled by default."
- "Each Workers invocation returns a single invocation log that contains details such as the Request, Response, and related metadata. … Each invocation log is enriched with information available to Cloudflare in the context of the invocation."
- "Maximum log retention period | 7 Days"
- "Invocation logs can be disabled in wrangler by adding the `invocation_logs = false` configuration."

同じページに "You must add the observability setting for your Worker to write logs" ともあり、**既定で有効かどうかは docs の中で食い違っている（未確認）**。`web/wrangler.toml` には `[observability]` が無い。推測（未確認）: 有効なら、`/api/beacon`・`/api/votes` への要求ごとに、IP を含む要求のヘッダが 7 日間、運営者のアカウントに残る。そうなら「IP アドレスそのものは保存せず」は**運営者のアカウントの中では成り立たない**。デプロイ後にダッシュボードで実物を見て確かめること。

### 2.4 Cloudflare の規約・DPA（問 5）

- Self-Serve Subscription Agreement: `https://www.cloudflare.com/terms/`。**本文は読めていない（未確認）**。検索の抜粋で読めたのは §2.5.1「You and your End Users … will retain all right, title and interest in and to any data …」だけ。**利用者向けのプライバシー告知を顧客が出す義務の条項は、確かめられなかった。**
- DPA: `https://www.cloudflare.com/cloudflare-customer-dpa/`（未到達・未確認）。推測: 顧客が controller、Cloudflare が processor という形で、セルフサーブの契約に組み込まれる（第三者の要約サイトにそう書かれていたが、一次資料ではない）。
- 推測: 日本法では Cloudflare への送信は「委託」（個人情報保護法 27 条 5 項 1 号）か、「クラウドサービスの事業者が中身を取り扱わない場合は提供に当たらない」という PPC の Q&A の整理のどちらかになり、「第三者提供」とはされない見込み（Q&A の番号・文言は未確認）。

## 3. 問 1 の結論: ポリシーの文と対策

「乱数は毎日消すため、日をまたいで同じ人を結び付けることはできません」は、**実装と Cloudflare の仕様に照らすと強すぎる**。

1. 消すのは「日が変わった後の**最初の要求**」で、毎日決まった時刻ではない（`visitor.ts`）。
2. 消した後も、**Time Travel で 7 日（無料プラン）戻せる**（§2.1）。
3. 同じ日のうちは、運営者が salt を持っているので結び付けられる（これは設計どおりで、ポリシーも「日をまたいで」に絞っている）。
4. Workers Logs が有効なら、生の IP そのものが 7 日残る可能性がある（§2.3・未確認）。

結び付けられないと言えるのは、**消してから 7 日を過ぎた後**（Logs も 7 日で消える前提）。

### 対策の案

| 案 | 中身 | 利点 | 欠点 |
|---|---|---|---|
| A 文を弱める | 「乱数は日付が変わると消します。消した後も Cloudflare の復旧用の記録に最長 7 日残るため、その間は運営者が技術的に結び付けられますが、そうした操作はしません」 | コード変更なし。事実どおり | 「できない」から「しない」に変わり、約束に頼る |
| B `secret + 日付` から salt を作る（HMAC） | D1 に salt を置かない | Time Travel の問題は消える | **悪化する。** 秘密を持つ人は過去の salt をいつでも作り直せるので、Analytics Engine の 3 か月分すべてが当てられる状態になる。秘密を毎日替えるなら、その秘密をどこかに置くことになり元に戻る。**採らない** |
| C ハッシュを短くして Analytics Engine に書く | beacon の `visitor` を例えば 32 ビット（16 進 8 桁）に切る。`ballots` の重複防止は 128 ビットのまま | salt が戻っても、1 つのハッシュに当てはまる (IP, UA) の組が多数残り、人を 1 人に決めにくい（推測: IPv4 × UA の候補は 2^32 を大きく超えるため） | 日ごとの利用者数の推定に偶然の一致が混ざる（推測: 1 日数千人なら誤差は 0.1% 程度。計算していない） |
| D IP を丸めてから混ぜる | IPv4 は /24、IPv6 は /48 にしてからハッシュ | 戻されても、当てられるのは回線の範囲まで | 同じ範囲・同じ UA の人が 1 人に数えられ、重複防止も巻き込む |
| E 定時に消す | Cron Trigger（UTC 0 時過ぎ）で `DELETE FROM salts/ballots` | 「毎日消す」が文字どおりになる | Time Travel は残る。無料プランで Cron が使えるかは未確認 |
| F Workers Logs を止める | `wrangler.toml` に `[observability]` と `invocation_logs = false`（または `enabled = false`） | IP が 7 日残る可能性を断てる | 障害の調査がしにくくなる |

**推奨: A + C + F（E は余裕があれば）。** C と F はそれぞれ数行の変更で、「戻せても人を決めにくい」「IP がログに残らない」を仕組みで支える。そのうえで A の文で残る 7 日を正直に書く。B は採らない。

## 4. 問 2: 個人情報保護法（APPI）

一次資料（本文は未到達。所在は検索で確認）: 法律 `https://laws.e-gov.go.jp/law/415AC0000000057`、通則編ガイドライン `https://www.ppc.go.jp/personalinfo/legal/guidelines_tsusoku/`（検索の結果では「令和8年6月一部改正」と出ている）、Q&A `https://www.ppc.go.jp/all_faq_index/`。

- **個人情報（法 2 条 1 項）**: 生存する個人に関する情報で、特定の個人を識別できるもの（他の情報と容易に照合でき、それで識別できるものを含む）または個人識別符号を含むもの（記憶にもとづく要約。逐語は未確認）。本サービスは氏名・メール・アカウントを持たず、照合に使える台帳も無い。推測: **運営者にとって個人情報には当たらない**見込み。IP アドレスだけで個人情報になるかは、PPC が「個人識別符号」に IP を挙げていない（検索で読めた 2016 年のパブコメ資料 `https://www.ppc.go.jp/files/pdf/280916_siryou1-1_bessi2.pdf` の抜粋。結論の文言は未確認）という理解にとどまる。
- **個人関連情報（法 2 条 7 項）**: 生存する個人に関する情報で、個人情報・仮名加工情報・匿名加工情報のどれにも当たらないもの。通則編 2-8 の例に「Cookie 等の端末識別子を通じて収集された、ある個人のウェブサイトの閲覧履歴」があると検索の抜粋で確かめた（`www.issoh.co.jp` 等の二次資料の引用。通則編の本文では未確認）。推測: 日替わりハッシュに紐付いた実行の記録は、これに近く**個人関連情報に当たりうる**。一方で、ガイドラインは「特定の個人との対応関係が排斥されている統計情報」は当たらないとしている（同じく二次資料の引用）。票の合計（`votes`）はこちら。
- **個人関連情報の義務（法 31 条）**は、受け取る第三者が「個人データとして取得することが想定される」ときの提供の場面にかかる（記憶にもとづく。未確認）。本サービスは第三者に渡さない（Cloudflare は §2.4 のとおり委託等）。推測: **かからない**。
- **利用目的の公表（法 21 条）・個人情報取扱事業者（法 16 条 2 項）**: 2017 年の改正で 5,000 件の要件はなくなり、営利・非営利を問わない（国交省の広報資料の抜粋 `https://www.mlit.go.jp/kankocho/content/810002310.pdf` で確認。PPC の本文は未確認）。ただし義務がかかるのは「個人情報データベース等を事業の用に供している者」で、扱うものが**個人情報**の場合。推測: 上のとおり個人情報を持たないなら、法 21 条の公表義務はかからない。個人の趣味のサイトが「事業」に当たるか（通則編 2-5 の「事業」の解釈）は未確認。
- 推測の結論: **法の義務としてのポリシーはおそらく要らないが、出しておくのが信頼のうえで良い**（いまの扱い＝`docs/telemetry.md` の「法的には問題なし・信頼のための説明」と同じ）。GitHub の Issue で問い合わせを受けるとき、書き込まれた内容は GitHub の側で扱われる。

## 5. 問 3: 電気通信事業法の外部送信規律（2023 年 6 月施行、法 27 条の 12）

一次資料: 総務省「外部送信規律 FAQ」`https://www.soumu.go.jp/main_sosiki/joho_tsusin/d_syohi/gaibusoushin_kiritsu_00002.html`（存在は検索で確認。本文は未到達）。

- 対象は「電気通信事業者」または「第三号事業を営む者」のうち、利用者の利益に及ぼす影響が少なくない役務を提供する者（FAQ 問 1-8 の要旨として検索で確認）。
- 「営む」は、反復継続して、電気通信事業そのもので利益を上げる目的と解されている（法律事務所の解説の要旨。総務省の本文では未確認）。推測: **広告も課金も無い個人の無料サイトは「営む」に当たらず、規律の対象外の見込み**。
- 推測（未確認）: Cookie を使わなくても、利用者の端末に外部へ送らせる指令（JavaScript 等）であれば対象になりうる（FAQ 問 1-7 の要旨として検索で確認）。また、送信先が運営者自身のサーバで役務の提供に必要な情報などは適用除外とされている（施行規則 22 条の 2 の 28 の各号。号の文言は未確認）。本サービスの beacon は**同じオリジン（自分の Worker）へ送り、第三者のタグは無い**ので、対象だったとしても適用除外の議論ができる。
- 結論（推測）: **対応は要らない見込み**。将来、広告・寄付の仕組み・第三者の解析タグを入れるなら見直す。

## 6. 問 4: GDPR・ePrivacy（EU からの訪問者）

一次資料（どれも本文は未到達）: GDPR `https://eur-lex.europa.eu/eli/reg/2016/679/oj`、EDPB Guidelines 2/2023 v2.0（2024-10-07 採択・10-16 公表）`https://www.edpb.europa.eu/documents/guideline/guidelines-22023-on-technical-scope-of-art-53-of-eprivacy-directive_en`、CJEU C-413/23 P EDPS v SRB（2025-09-04）`https://curia.europa.eu/`。

- **適用されるか（GDPR 3 条 2 項）**: EU の外の管理者でも、EU の人へ役務を「提供」するか、EU の中での行動を「監視」するなら適用される（記憶にもとづく。条文の文言は未確認）。無料でも「提供」になりうる。推測: 日本語・英語・中国語・韓国語の 5 言語で、EU の言語・通貨・ドメインに向けた工夫は無いので、「EU に向けた提供」は弱い。日替わりのハッシュによる集計は、行動の追跡やプロファイリングというより利用統計に近い。**適用はグレーで、低リスク寄り。**
- **個人データか**: 推測: IP は個人データになりうる（CJEU Breyer C-582/14。未確認）。salt を持つ当日の運営者にとって、ハッシュは**仮名化データ＝個人データ**（GDPR 前文 26）。SRB 判決は、再識別できない受け手にとっては個人データでない場合があるとした（法律事務所の要約で確認。判決本文は未読）が、本件の運営者は salt を持つ側なので、当日・Time Travel の 7 日・Logs の間は個人データとみるのが安全。
- **適法の根拠**: 推測: 正当な利益（6 条 1 項 f）。告知（13 条）はいまのポリシーでおおむね足りるが、根拠・保存期間・権利の窓口を書いていない。
- **ePrivacy 5 条 3 項**: 端末に情報を保存すること、または端末の中の情報へアクセスすることには、厳密に必要なものを除いて同意が要る。検索の抜粋によれば、EDPB 2/2023 は、トラッキングピクセル・トラッキングリンクを「アクセス」に当たるとし、端末の中のコードがネットワーク越しに情報を送り返させることもアクセスに当たるとしている。IP だけによる追跡にも節（3.3）がある。推測: この解釈では、**画面の JavaScript が条件・結果を `sendBeacon` で送る行為は 5 条 3 項の範囲に入りうる**。利用統計は「利用者が求めた役務に厳密に必要」とはされにくい。cookie を使わないことは、それだけでは逃げ道にならない。
  - ただし 5 条 3 項は各国法で実施され、日本の運営者にどこまで及ぶかは別の論点（未調査）。フランスの CNIL のように、条件付きで利用統計を同意不要とする国がある（記憶にもとづく。未確認）。
- 推測の結論: **EU への露出は小さく、実害のリスクは低い。ただし EDPB の解釈に沿うと、厳密には EU からの beacon に同意が要りうる。** 取れる策は (a) 何もしないで告知を厚くする、(b) Worker で `request.cf.country` が EEA の要求の beacon を書かずに捨てる（推測: Cloudflare の `cf` オブジェクトの国コードで判定できる。CF-docs は未確認）、(c) beacon をオプトインにする（計測の数が大きく減る）。

## 7. 開発者が決めること

**決定（2026-10-18）**: 文の修正だけを行い、仕組みは変えない（開発者とレビュー担当の判断。非営利のファンサイトでは十分）。画面の文は「通常は日をまたいで結び付けられない。ただし消したデータは Cloudflare の復旧機能で 7 日間は戻せる状態で残る」（5 言語）。Workers Logs は止めた（S7）。EU の訪問者・定時の削除・ハッシュの短縮は行わない。

| # | 決めること | 選択肢 | 推奨 |
|---|---|---|---|
| 1 | ハッシュの文の書き方 | 今のまま ／ A（7 日の復旧用の記録を書く） | **A**（今の文は事実より強い） |
| 2 | Time Travel への対策 | なし ／ C（beacon の `visitor` を 32 ビットに切る） ／ D（IP を丸める） ／ B（HMAC） | **C**。B は採らない（悪化する） |
| 3 | Workers Logs | 既定のまま ／ F（`invocation_logs = false` か `enabled = false` を書く） | **F**。デプロイ後に Logs の画面で IP が残っていないか確かめる |
| 4 | 消す時刻 | 遅延削除のまま ／ E（Cron で UTC 0 時過ぎ） | 余裕があれば E。しないなら文を「日付が変わると」に |
| 5 | EU の訪問者 | 何もしない＋告知を足す ／ EEA の beacon を捨てる ／ オプトイン | **何もしない＋告知を足す**（露出が小さい）。迷うなら「捨てる」も数行でできる |
| 6 | `docs/telemetry.md` と画面の食い違い（§1） | docs を直す ／ 画面に文を足す | 1 の A で画面に足せば両方そろう |

### ポリシーの文の変更案（5 言語とも。`PRIVACY_LAST_UPDATED` を更新。本調査では編集していない）

1. `privacy.hashBody` →「利用者数の推定と重複投票の防止のため、IP アドレスとブラウザの種類に日替わりの乱数を混ぜたハッシュ値を使います。IP アドレスそのものは保存しません。乱数は日付（UTC）が変わると消します。消した後も Cloudflare の復旧用の記録に最長 7 日残るため、その間は運営者が技術的に結び付けられますが、そうした操作はしません。7 日を過ぎると、日をまたいで同じ人を結び付けることはできなくなります。」（案 C を入れたら「記録するハッシュは短く切り、元の IP を 1 つに決められないようにしています」を足す）
2. `privacy.retentionBody` に足す:「重複投票の防止のための記録は翌日に消します（復旧用の記録に最長 7 日残ります）。」
3. （EU 向け・任意）「利用状況の記録は、サービスの改善という正当な利益にもとづいて行います。」を `usageItem` の後に足す。
4. 案 F をしない場合に限り:「Cloudflare の動作ログに、要求の IP アドレスが最長 7 日残ることがあります。」（§2.3 の確認の後に決める）
5. 文言を変えたら、AGENTS §8-8 のとおり `docs/telemetry.md`・`visitor.ts` のコメント（「Time Travel。推測・未確認」→ 確認済み・7 日）・5 言語を grep し、古い記述を 0 件にする。

## 8. 未確認の一覧（人の手で確かめるもの）

- 法令・PPC 通則編 2-1・2-5・2-8、Q&A（クラウド例外・個人関連情報）の本文の逐語
- 総務省 外部送信規律 FAQ 問 1-7・1-8、施行規則 22 条の 2 の 28 の号
- EDPB 2/2023 v2.0 の該当段落（2.5 節・3.3 節・ピクセル）
- Cloudflare Self-Serve Subscription Agreement と Customer DPA の、エンドユーザーへの告知・processor の条項
- Workers Logs が既定で有効か、invocation log に IP が入るか（実アカウントで見る）
- D1 Time Travel を無効にする手段が無いこと（読んだのは `time-travel.mdx`・`limits.mdx` の 2 ファイルだけ）
