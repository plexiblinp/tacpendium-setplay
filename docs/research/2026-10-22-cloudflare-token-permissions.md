> **外部 AI の出力（原文のまま）**
> - 取得日: 2026-10-22（出力の中の「調査実施」は 2026-10-23 日本時間。開発者が外部の生成 AI に依頼し、結果のファイルをセッションに渡した。使った外部 AI の種類は記録していない）
> - 問い: Cloudflare の新しい API トークンの権限方針で、Legacy を使わずに wrangler deploy を通す最小の方針（`docs/telemetry.md`「デプロイ用の API トークン」の根拠）
> - 裏取り: **中心の主張は済み**。cloudflare-docs のソース（`src/content/docs/workers/authorization/workers.mdx`、GitHub 上。2026-10-22 取得）で、(1) Editor は既存の Worker の更新・デプロイだけで作成・削除はできない、(2) 新しい Worker の作成には Workers 全体の Admin が要る、(3) 束縛を持つ Worker のデプロイに束縛先の権限は要らない、(4) Legacy の廃止日は今のところ無い、を確かめた。実際の動作も S9 で確かめた（初回は Workers: Admin で作成でき、その後 Individual Workers: Editor に絞った状態でデプロイが通った。ただし絞った後のデプロイはアセットの変更が無く、ファイルの上げ直しは未確認）。API リファレンスの各エンドポイントの記載は、この出力の記述のまま。

---

# Cloudflare Workers の非 Legacy API トークン権限調査

**基準日:** 2026-10-22（日本時間）  
**調査実施:** 2026-10-23（日本時間）。2026-10-22以前に公開・更新された情報を判断材料とする。  
**対象:** Account API token / GitHub Actions / Wrangler 4.150系 / 静的アセット付き Worker の初回デプロイ / D1・Workers Analytics Engine 束縛 / `workers.dev` / `preview_urls = false`。

## 結論（要約）

1. **公式の権限モデルで初回作成に必要な最小の新方針は、`Developer Platform > Workers: Admin` を「Workers プロダクト全体（対象アカウント内の全 Worker）」に設定すること（1 方針）。** `Workers: Editor` は既存 Worker の更新用であり、新規作成はできない。［一次：2026-09-15、[S1](https://developers.cloudflare.com/workers/authorization/workers/)］
2. **ただし、この 1 方針で「静的アセット付き `wrangler deploy` が Legacy なしで必ず通る」とは未確認。** 問題の `assets-upload-session` API は、現在公開されている API リファレンスでは受け付ける権限に **`Workers Scripts Write` のみ**を掲載しており、`Workers Admin` が明示されていない。したがって、**実際に動作が確認された「Legacy 不使用の最小構成」は不明**。［一次：API リファレンス更新日記載なし、[S4](https://developers.cloudflare.com/api/resources/workers/subresources/scripts/subresources/assets/subresources/upload/methods/create/)］
3. **既存 D1 への束縛のためだけに D1 の Read / Edit を追加する必要はない。** Analytics Engine についても、束縛定義時に独立した権限を要求する公式記載は確認できず、追加権限を安易に与える根拠はない。ただし同構成での成功確認はない。［一次：2026-09-15、[S1](https://developers.cloudflare.com/workers/authorization/workers/)；2026-04-23、[S10](https://developers.cloudflare.com/analytics/analytics-engine/get-started/)］
4. **推奨運用:** 初回だけ短期の Workers 全体 Admin で動作を検証し、作成できたらそのトークンを失効させ、`Specified Workers` + `Workers: Editor` の専用 CI トークンに切り替える。ただし、切り替え後のアセットアップロードにも同じ API の不確実性が残るため、**切り替え後の `wrangler deploy` 成功を必ず検証する**。［一次：2026-09-15、[S1](https://developers.cloudflare.com/workers/authorization/workers/)・[S2](https://blog.cloudflare.com/workers-granular-authorization/)］

> **確度の区分:** **確定（公式の明文）**・**未確認（公式 API 記載と矛盾／不足）**・**推論（権限モデルからの運用上の判断）**を区別する。本調査では実アカウント、API トークン、`wrangler 4.150.x` を使った実デプロイを行っていない。

## 1. 新「Workers」方針 4 段階の操作・API と公式説明

- **Metadata Read-only:** Worker の一覧・メタデータ・設定・メトリクス・ログ・トレース等を閲覧可能。スクリプトのソース内容と secret の値は閲覧不可、変更不可。`wrangler tail` は対象 Worker のこの権限が最小と明記される。**確定。**［一次：2026-09-15、[S1](https://developers.cloudflare.com/workers/authorization/workers/)；2026-09-15、[S2](https://blog.cloudflare.com/workers-granular-authorization/)］
- **Content Read-only:** 上記に加え、Worker ソースコード・コンテンツを閲覧可能。デプロイ・更新・作成・削除は不可。例: `GET /accounts/{account_id}/workers/scripts/{script_name}/content/v2` は `Workers Content Read-Only` と `Individual Workers Content Read-Only` を受け付けると API リファレンスに記載。**確定。**［一次：2026-09-15、[S1](https://developers.cloudflare.com/workers/authorization/workers/)；API 公開・更新日記載なし、[S5](https://developers.cloudflare.com/api/resources/workers/subresources/scripts/subresources/content/methods/get/)］
- **Editor:** **既存** Worker の読み取り、ソースと設定の更新、バージョンのアップロード・デプロイ、ロールバック、スケジュール・Observability 設定、secret の追加・削除、Worker 名の変更が可能。Worker **新規作成と削除は不可**。API 例: `POST .../versions`、`POST .../deployments` は `Workers Editor` および `Individual Workers Editor` を受け付ける。**確定。**［一次：2026-09-15、[S1](https://developers.cloudflare.com/workers/authorization/workers/)；API 更新日記載なし、[S6](https://developers.cloudflare.com/api/resources/workers/subresources/scripts/subresources/versions/methods/create/)・[S7](https://developers.cloudflare.com/api/resources/workers/subresources/scripts/subresources/deployments/methods/create/)］
- **Admin:** Editor の操作に加え、Worker の**新規作成と削除**が可能。新規作成には **Workers プロダクト全体の Admin** が必要。個別 Worker の Admin でも当該 Worker の削除は可能だが、未作成 Worker を新規作成するための権限にはならない。API 例: `DELETE .../workers/scripts/{script_name}` は `Workers Admin` と `Individual Workers Admin` を受け付ける。**確定。**［一次：2026-09-15、[S1](https://developers.cloudflare.com/workers/authorization/workers/)；API 更新日記載なし、[S8](https://developers.cloudflare.com/api/resources/workers/subresources/scripts/methods/delete/)］
- **網羅的な API ごとの許可表は不明。** 公式の「Workers roles and permissions」は代表的な操作と Wrangler コマンドを説明するが、全 API の完全な 4 ロール対応表ではない。また、個別 API の `Accepted Permissions` は新旧表記が不統一である。上記 API は「確認できた例」であり、網羅リストではない。［一次：2026-09-15、[S1](https://developers.cloudflare.com/workers/authorization/workers/)；API 更新日記載なし、[S4](https://developers.cloudflare.com/api/resources/workers/subresources/scripts/subresources/assets/subresources/upload/methods/create/)・[S6](https://developers.cloudflare.com/api/resources/workers/subresources/scripts/subresources/versions/methods/create/)］

## 2. Editor で新規作成・`assets-upload-session` が拒否されるのは仕様か

- **新規作成が Editor では許されないのは明示的な仕様。** Cloudflare は「Editor は既存 Worker を更新・デプロイできるが、作成・削除はできない」「新規作成時の `wrangler deploy` は Workers プロダクト全体の Admin が必要」と明記。つまり、Editor は「既存 Worker の編集」に限定されるという理解は **作成権限の意味では正しい**。［一次：2026-09-15、[S1](https://developers.cloudflare.com/workers/authorization/workers/)］
- **ただし `assets-upload-session` 自体が『新規作成だから Editor 拒否』なのかは不明。** `POST /accounts/{account_id}/workers/scripts/{script_name}/assets-upload-session` の `Accepted Permissions` は **`Workers Scripts Write` だけ**であり、新 `Workers Editor` も新 `Workers Admin` も掲載されていない。したがって、今回の `No access to the specified resource.` は（a）新規作成不可、（b）アセット API への新権限対応の不足／リファレンス未更新、またはその両方、のどれかと断定できない。**原因の特定は未確認。**［一次：API 更新日記載なし、[S4](https://developers.cloudflare.com/api/resources/workers/subresources/scripts/subresources/assets/subresources/upload/methods/create/)；2026-09-15、[S1](https://developers.cloudflare.com/workers/authorization/workers/)］
- 比較すると `POST .../versions` には新 `Workers Admin` / `Workers Editor` が掲載されている。**API ごとに新権限の掲載状況が異なる**こと自体は直接確認できるが、それが実 API の権限制御の未対応を意味するか、ドキュメントの更新漏れかは不明。［一次：API 更新日記載なし、[S4](https://developers.cloudflare.com/api/resources/workers/subresources/scripts/subresources/assets/subresources/upload/methods/create/)・[S6](https://developers.cloudflare.com/api/resources/workers/subresources/scripts/subresources/versions/methods/create/)］
- **類似エンドポイントの別障害と混同しないこと。** GitHub issue #13118（2026-03-30）は同 API で HTTP 500 / コード 10013 が発生した障害報告で、Cloudflare 側がインシデントに案内している。今回の `No access ...` を説明する権限情報では**ない**。［二次：2026-03-30、[S15](https://github.com/cloudflare/workers-sdk/issues/13118)］

## 3. Legacy 不使用で静的アセット付き初回 `wrangler deploy` を通す最小方針

- **公式ロール定義からの最小「候補」:** `Permission policies` に **Workers プロダクト全体の `Developer Platform > Workers: Admin` を 1 件**。UI で `Entire Account` を範囲として選ぶ場合も、**付与する権限の対象は Workers 方針だけ**とし、アカウント全サービスの Administrator 権限や Developer Platform 全体 Admin と混同しない。新規作成が必要なので `Workers: Editor` 単独や `Specified Workers` は不適切。［一次：2026-09-15、[S1](https://developers.cloudflare.com/workers/authorization/workers/)・[S3](https://developers.cloudflare.com/workers/authorization/)］
- **追加方針:** この前提では独自ドメインや Routes を変更しないので `Zone > Workers Routes > Write` は不要。**既存の** D1 / KV / R2 の権限も「束縛だけ」では不要。ただし D1 の新規自動プロビジョニングを行う構成は例外（質問 4 参照）。`Workers: Admin` に Editor が内包されるので、別に `Workers: Editor` を重ねる理由もない。［一次：2026-09-15、[S1](https://developers.cloudflare.com/workers/authorization/workers/)］
- **「これで必ず通る」という判定は保留。** 静的アセットの最初のステップは `assets-upload-session` であり、公式 API リファレンスは Legacy `Workers Scripts Write` のみを掲載。さらに `POST /accounts/{account_id}/workers/workers`（Worker 作成）も API リファレンス上は `Workers Scripts Write` のみと記載されるため、**上位のロールガイド（Admin 必須）と API ごとの表記が一致しない**。実際の認可実装がどうなっているかを、公開文書だけで確証できない。［一次：API 更新日記載なし、[S4](https://developers.cloudflare.com/api/resources/workers/subresources/scripts/subresources/assets/subresources/upload/methods/create/)・[S9](https://developers.cloudflare.com/api/resources/workers/subresources/beta/subresources/workers/methods/create/)；2026-09-15、[S1](https://developers.cloudflare.com/workers/authorization/workers/)］
- **本番環境を危険にさらさずに検証する手順（推奨・検証未実施）:** (1) 対象アカウントの `workers.dev` サブドメインが既に設定済みか確認。(2) **Legacy なし・Workers プロダクト全体 Admin だけ**の短期テスト用 Account API token を作り、通常の `CLOUDFLARE_API_TOKEN` / `CLOUDFLARE_ACCOUNT_ID` で本番とは別名の最小 Worker + 少量アセットを初回 deploy。(3) 成功時は初回作成権限が不要になるので Admin トークンを失効させ、対象 Worker の `Specified Workers` + `Editor` トークンを作る。(4) **同じトークンでアセットに変更を加えた再デプロイを検証**。(5) どちらかで拒否された場合、そこで停止し、レスポンスの HTTP ステータス・Cloudflare エラーコード・Ray ID・エンドポイント・Wrangler 正確な版・トークンのスコープ（値は秘匿）を付けて Cloudflare へ確認。**検証前に Legacy を追加したり、D1 権限を増やして推測で回避しない。**［手順の根拠：一次 2026-09-15、[S1](https://developers.cloudflare.com/workers/authorization/workers/)；一次 2026-09-28、[S13](https://developers.cloudflare.com/fundamentals/api/get-started/account-owned-tokens/)；一次 2026-10-01、[S14](https://developers.cloudflare.com/workers/wrangler/system-environment-variables/)］
- **未実施:** 特定の `wrangler 4.150.x` ビルドにおける、上記の Legacy 不使用・新 Admin 限定トークンでの `assets-upload-session` および後続 API の実成功例は確認できなかった。したがって、実証済み最小構成としては報告しない。

## 4. D1 と Analytics Engine の束縛に追加権限が必要か

- **D1: 既存データベースを Worker に束縛して deploy するだけなら不要。** Cloudflare は「KV、R2、D1 等への束縛を持つ Worker をデプロイするには Worker の Editor 権限で足りる。束縛先の個別権限は不要」と明記している。`wrangler d1 execute`、D1 自体の作成・クエリ等を CI で別途行う場合は話が異なる。［一次：2026-09-15、[S1](https://developers.cloudflare.com/workers/authorization/workers/)］
- **例外：D1 が未作成で Wrangler の自動プロビジョニングを利用する場合。** Cloudflare の Wrangler 設定資料（2026-10-20）は、リソース ID が未指定なら deploy 時に D1 等を自動作成し得ると説明する。これは単なる束縛ではなく **D1 の新規作成** なので、「D1 権限は不要」という結論の範囲外である。新規作成が必要な場合は D1 側の作成権限を別途検討するか、管理者が先に DB を作成し `database_id` を固定する。**自動作成時に必要な新 UI の正確なロール組合せは今回未検証。**［一次：2026-10-20、[S19](https://developers.cloudflare.com/workers/wrangler/configuration/)；2026-09-15、[S3](https://developers.cloudflare.com/workers/authorization/)］
- **Workers Analytics Engine: 束縛だけなら追加権限を要求する根拠は見当たらない。** 公式手順は `analytics_engine_datasets` を Wrangler 設定に記述するだけで、データセットは Worker が最初にデータを書いた際に自動作成される、と説明する。**ただし、Workers Analytics Engine 束縛に関する「別の API token 権限は絶対不要」という明示的な権限表は未確認**。結論の確度は D1 より低い。Analytics Engine の GraphQL 等を CI から直接呼ぶケースは本件の対象外。［一次：2026-04-23、[S10](https://developers.cloudflare.com/analytics/analytics-engine/get-started/)；2026-09-15、[S1](https://developers.cloudflare.com/workers/authorization/workers/)］
- **セキュリティ上の重要な留意点:** 「トークンに D1 の API 権限が不要」でも「Worker のコードから D1 にアクセスできない」わけではない。攻撃者が Worker のコードを差し替えられる **Editor でも**、その Worker に与えられた D1 束縛を悪用してデータの読み出し・改変・外部送信を行える可能性がある。従って **Editor は低リスクな『静的ファイルアップロード権限だけ』ではない**。これは Worker 実行権限と束縛の構造に基づく**脅威モデル上の推論**。［一次：2026-09-15、[S1](https://developers.cloudflare.com/workers/authorization/workers/)；2026-04-23、[S10](https://developers.cloudflare.com/analytics/analytics-engine/get-started/)］

## 5. `workers.dev` 公開と `preview_urls` 変更に別の権限が必要か

- **既存アカウントで、既存 Worker の `workers.dev` 公開を変更する操作:** `POST /accounts/{account_id}/workers/scripts/{script_name}/subdomain` は `enabled` と `previews_enabled` を受け取る。Cloudflare の **TypeScript / Go API リファレンスは** `Workers Admin`、`Workers Editor`、`Individual Workers Admin`、`Individual Workers Editor` を受け付けると明記している。**専用の Zone Workers Routes 権限は、この API の列挙には現れない。**［一次：API 更新日記載なし、[S11](https://developers.cloudflare.com/api/typescript/resources/workers/subresources/scripts/subresources/subdomain/methods/create/)；2026-09-15、[S1](https://developers.cloudflare.com/workers/authorization/workers/)］
- **ただし API ドキュメント内に差がある:** 同じエンドポイントの **HTTP 版リファレンスは `Workers Scripts Write` のみを掲載**している。新権限による成功可否は SDK 版記載が支持するが、HTTP 版との不一致は解消されていない。［一次：API 更新日記載なし、[S12](https://developers.cloudflare.com/api/resources/workers/subresources/scripts/subresources/subdomain/methods/create/)・[S11](https://developers.cloudflare.com/api/typescript/resources/workers/subresources/scripts/subresources/subdomain/methods/create/)］
- **`preview_urls = false`:** Wrangler 設定は Version URLs（以前の Preview URLs）を無効化するための正式な設定。既存 Worker のデプロイに必要な Workers Editor / Admin 以外の **Version URLs 専用追加ロールは確認できない**。独自ドメイン・Routes は使わないので `Zone > Workers Routes > Write` は不要。［一次：2026-09-22、[S16](https://developers.cloudflare.com/workers/versions-and-deployments/version-urls/)；2026-09-15、[S1](https://developers.cloudflare.com/workers/authorization/workers/)］
- **`workers.dev` アカウントのサブドメインを初めて「確保・設定」する場合は別問題。** `PUT /accounts/{account_id}/workers/subdomain` というアカウント単位 API が存在し、HTTP API リファレンスは `Workers Scripts Write` のみを掲載する。**新 `Workers: Admin` だけでこの初期作業まで通るかは不明**。CI 前に管理者がダッシュボードでサブドメインの初期設定を完了しておくのが確実。［一次：API 更新日記載なし、[S17](https://developers.cloudflare.com/api/resources/workers/subresources/subdomains/methods/update/)；2026-09-22、[S18](https://developers.cloudflare.com/workers/configuration/routing/workers-dev/)］
- **`workers.dev` 本番 URL と Version URLs は別の公開経路。** `preview_urls = false` だけでは `workers.dev` の本番 URL を遮断しない。実際の公開予定に沿って `workers_dev = true` と `preview_urls = false` の組合せを明示する。［一次：2026-09-22、[S16](https://developers.cloudflare.com/workers/versions-and-deployments/version-urls/)・[S18](https://developers.cloudflare.com/workers/configuration/routing/workers-dev/)；Wrangler 設定 2026-10-20 更新、[S19](https://developers.cloudflare.com/workers/wrangler/configuration/)］

## 6. `Specified Workers` に限定したトークンで初回作成できるか

- **できない（公式の明文）。** `Specified Workers` は存在する Worker を選択するスコープ。未作成の Worker は選択できず、作成には Workers **プロダクト全体**の Admin が必須。［一次：2026-09-15、[S1](https://developers.cloudflare.com/workers/authorization/workers/)］
- **権限の分割が正解:** 初回デプロイ用は Workers プロダクト全体 Admin の一時トークン、以後の CI 用はその **既存 Worker 1 個のみを選ぶ `Specified Workers` + `Workers: Editor`**。ただしアセット API の新権限との整合性は検証が必要。［一次：2026-09-15、[S1](https://developers.cloudflare.com/workers/authorization/workers/)・[S2](https://blog.cloudflare.com/workers-granular-authorization/)・[S20](https://developers.cloudflare.com/changelog/post/2026-09-15-granular-worker-permissions/)］
- **範囲の解釈:** `Entire Account` を選んでも `Workers: Admin` 方針だけなら D1 や R2 にまで Admin が広がるわけではない。しかし、**そのアカウントの既存・将来のすべての Worker への Admin** となるので強い権限である。［一次：2026-09-15、[S1](https://developers.cloudflare.com/workers/authorization/workers/)］

## 7. Legacy `Workers Scripts` の廃止日・移行案内

- **廃止予定の具体的な日付・期限は未公表。** Cloudflare 公式ブログ（2026-09-15）および公式 Workers 権限文書（2026-09-15）は、Legacy 権限は引き続き動作し、廃止する場合は事前告知すると明言している。従って「Legacy と書いてあるので直ちに使えなくなる」は誤り。［一次：2026-09-15、[S2](https://blog.cloudflare.com/workers-granular-authorization/)・[S1](https://developers.cloudflare.com/workers/authorization/workers/)］
- **公式の移行先案内はある。** `Workers Scripts Read` → 新 `Workers: Content Read-only`、`Workers Scripts Edit` → 新 `Workers: Editor`（Workers プロダクト全体スコープ）。`Workers CI Edit` → 同 `Editor`、`Workers Tail Read` → `Metadata Read-only` 等。これは**既存 Worker を扱う通常のロール対応**であり、静的アセット付き**初回作成の全エンドポイント互換性まで保証するものではない**。［一次：2026-09-15、[S1](https://developers.cloudflare.com/workers/authorization/workers/)・[S2](https://blog.cloudflare.com/workers-granular-authorization/)］
- **`Workers Scripts Edit`（UI の Legacy 権限名）と `Workers Scripts Write`（API リファレンス上の名称）は表記が異なる。** 本レポートでは、それらが内部的に完全に同一の権限 ID と断定せず、API 上の `Write` は原文のまま記載している。［一次：2026-09-15、[S2](https://blog.cloudflare.com/workers-granular-authorization/)；API 更新日記載なし、[S4](https://developers.cloudflare.com/api/resources/workers/subresources/scripts/subresources/assets/subresources/upload/methods/create/)］

## 8. `Workers: Admin` が Editor より余分に許す操作と漏えい影響

- **明確な追加権限:** Worker の **新規作成・削除**。削除は `DELETE /accounts/{account_id}/workers/scripts/{script_name}` で行え、`Workers Admin` または `Individual Workers Admin` を受け付ける。Editor は削除不可。［一次：2026-09-15、[S1](https://developers.cloudflare.com/workers/authorization/workers/)；API 更新日記載なし、[S8](https://developers.cloudflare.com/api/resources/workers/subresources/scripts/methods/delete/)］
- **全 Workers 対象 Admin が漏れた場合:** 新しい悪意ある Worker の作成、既存 Worker のコードや公開設定の変更、**任意の既存 Worker の削除**を含む広範な障害が起こり得る。`Specified Workers` + Editor なら、他の Worker の作成・削除・変更は本来許されない。［一次：2026-09-15、[S1](https://developers.cloudflare.com/workers/authorization/workers/)・[S2](https://blog.cloudflare.com/workers-granular-authorization/)］
- **Editor でも被害は大きい:** 対象 Worker のコード・実行動作・設定を変更でき、D1 等のバインディングからアプリ権限でデータを操作できる可能性がある。Admin と Editor の差は主に **作成／削除と操作対象のスコープ**であり、Editor だけならデータの機密性・完全性を守れる、という意味ではない。後半は**脅威モデル上の推論**。［一次：2026-09-15、[S1](https://developers.cloudflare.com/workers/authorization/workers/)・[S2](https://blog.cloudflare.com/workers-granular-authorization/)］
- **API トークン管理権限は自動では含まれない。** `Workers: Admin` が Cloudflare アカウントの管理者権限や `Account API Tokens > Write` まで持つとは公式文書から読み取れない。Account API token の作成は別の専用権限の操作。［一次：2026-09-28、[S21](https://developers.cloudflare.com/fundamentals/api/how-to/create-via-api/)；2026-09-15、[S1](https://developers.cloudflare.com/workers/authorization/workers/)］
- **漏えい対策の推奨:** Admin は初回の短期利用に限定し、実運用は Worker 単位 Editor。トークンの有効期限設定・ローテーションも検討する。Cloudflare はトークンの TTL / IP フィルタを漏えいリスクの緩和策として案内しているが、GitHub ホステッドランナーは送信元 IP 固定の可否に別途注意が必要。［一次：2026-09-28、[S13](https://developers.cloudflare.com/fundamentals/api/get-started/account-owned-tokens/)・[S21](https://developers.cloudflare.com/fundamentals/api/how-to/create-via-api/)；2026-09-15、[S2](https://blog.cloudflare.com/workers-granular-authorization/)］

## 不明だった点の一覧

1. **最重要:** `wrangler 4.150.x` で、**Legacy を 1 件も付けない `Workers: Admin`（Workers プロダクト全体）だけ**の Account API token が、静的アセットの `POST .../assets-upload-session` を本当に通過できるか。公式ロールガイドは Admin を必要とするが、この API の権限一覧には新権限が掲載されていない。［[S1](https://developers.cloudflare.com/workers/authorization/workers/)・[S4](https://developers.cloudflare.com/api/resources/workers/subresources/scripts/subresources/assets/subresources/upload/methods/create/)］
2. **同様に重要:** Worker 作成後、`Specified Workers` + 新 `Workers: Editor` **だけ**で、変更された静的アセットを含む `wrangler deploy` が成功するか。通常の既存 Worker デプロイは Editor が公式要件だが、アセット API での実運用可否は未確認。［[S1](https://developers.cloudflare.com/workers/authorization/workers/)・[S4](https://developers.cloudflare.com/api/resources/workers/subresources/scripts/subresources/assets/subresources/upload/methods/create/)］
3. `assets-upload-session` と Worker 作成 API が新権限を列挙しないのは **ドキュメントの更新遅延**か、**API 側の認可未対応**か。また、今回の `No access to the specified resource.` が「未作成 Worker だから」なのか「エンドポイントの権限不足」なのかは切り分け不能。［[S4](https://developers.cloudflare.com/api/resources/workers/subresources/scripts/subresources/assets/subresources/upload/methods/create/)・[S9](https://developers.cloudflare.com/api/resources/workers/subresources/beta/subresources/workers/methods/create/)］
4. D1 の Wrangler 自動プロビジョニング時に必要な権限の正確な組合せと、Analytics Engine の束縛 **のみ**について、他の個別 API 権限が不要であると明記した最新の個別権限表は未確認。公式の Analytics Engine セットアップは追加権限を指定していない。［[S10](https://developers.cloudflare.com/analytics/analytics-engine/get-started/)］
5. `workers.dev` アカウント全体の初回サブドメイン作成 API と、Worker ごとの公開設定 API における、**新 Workers 権限の完全な互換性**。後者は SDK 版 API リファレンスに新権限が掲載されている一方、HTTP 版では Legacy のみ。［[S11](https://developers.cloudflare.com/api/typescript/resources/workers/subresources/scripts/subresources/subdomain/methods/create/)・[S12](https://developers.cloudflare.com/api/resources/workers/subresources/scripts/subresources/subdomain/methods/create/)・[S17](https://developers.cloudflare.com/api/resources/workers/subresources/subdomains/methods/update/)］
6. Cloudflare の **全 Workers API × 新 4 ロール × スコープ**を網羅した、更新日付きの機械可読な権限対応表は本調査では見つからなかった。［[S1](https://developers.cloudflare.com/workers/authorization/workers/)］

## 情報源の一覧

**日付の扱い:** Docs は「Last updated」、ブログ・Changelog・GitHub issue は公開日を掲載。Cloudflare API リファレンスの個別エンドポイントは**ページ内に公開日・更新日が見つからないため「日付記載なし」**とし、2026-10-23 に公開ページを確認した事実と区別する。すべて 2026-10-22 時点の公開資料の検証に使用。

| ID | 情報源 | 日付（公開日／更新日） | 区分 | URL |
|---|---|---|---|---|
| S1 | Workers roles and permissions | **更新 2026-09-15** | 一次（Cloudflare公式 docs） | https://developers.cloudflare.com/workers/authorization/workers/ |
| S2 | Give every teammate and agent the right level of access to your Workers | **公開 2026-09-15** | 一次（Cloudflare公式ブログ） | https://blog.cloudflare.com/workers-granular-authorization/ |
| S3 | Roles and permissions (Developer Platform) | **更新 2026-09-15** | 一次（Cloudflare公式 docs） | https://developers.cloudflare.com/workers/authorization/ |
| S4 | Create Worker Assets Upload Session | **日付記載なし**（2026-10-23 確認） | 一次（Cloudflare公式 HTTP API） | https://developers.cloudflare.com/api/resources/workers/subresources/scripts/subresources/assets/subresources/upload/methods/create/ |
| S5 | Get Worker Script Content | **日付記載なし**（2026-10-23 確認） | 一次（Cloudflare公式 HTTP API） | https://developers.cloudflare.com/api/resources/workers/subresources/scripts/subresources/content/methods/get/ |
| S6 | Upload Version | **日付記載なし**（2026-10-23 確認） | 一次（Cloudflare公式 HTTP API） | https://developers.cloudflare.com/api/resources/workers/subresources/scripts/subresources/versions/methods/create/ |
| S7 | Create Worker Deployment | **日付記載なし**（2026-10-23 確認） | 一次（Cloudflare公式 HTTP API） | https://developers.cloudflare.com/api/resources/workers/subresources/scripts/subresources/deployments/methods/create/ |
| S8 | Delete Worker | **日付記載なし**（2026-10-23 確認） | 一次（Cloudflare公式 HTTP API） | https://developers.cloudflare.com/api/resources/workers/subresources/scripts/methods/delete/ |
| S9 | Create Worker | **日付記載なし**（2026-10-23 確認） | 一次（Cloudflare公式 HTTP API） | https://developers.cloudflare.com/api/resources/workers/subresources/beta/subresources/workers/methods/create/ |
| S10 | Get started with Workers Analytics Engine | **更新 2026-04-23** | 一次（Cloudflare公式 docs） | https://developers.cloudflare.com/analytics/analytics-engine/get-started/ |
| S11 | Update Worker Script Subdomain (TypeScript) | **日付記載なし**（2026-10-23 確認） | 一次（Cloudflare公式 SDK API） | https://developers.cloudflare.com/api/typescript/resources/workers/subresources/scripts/subresources/subdomain/methods/create/ |
| S12 | Post Worker subdomain (HTTP) | **日付記載なし**（2026-10-23 確認） | 一次（Cloudflare公式 HTTP API） | https://developers.cloudflare.com/api/resources/workers/subresources/scripts/subresources/subdomain/methods/create/ |
| S13 | Account API tokens | **更新 2026-09-28** | 一次（Cloudflare公式 docs） | https://developers.cloudflare.com/fundamentals/api/get-started/account-owned-tokens/ |
| S14 | System environment variables | **更新 2026-10-01** | 一次（Cloudflare公式 docs） | https://developers.cloudflare.com/workers/wrangler/system-environment-variables/ |
| S15 | workers-sdk issue #13118 | **報告 2026-03-30**（同日 close） | **二次（GitHub ユーザー報告。Cloudflare担当者の障害回答を含む）** | https://github.com/cloudflare/workers-sdk/issues/13118 |
| S16 | Version URLs | **更新 2026-09-22** | 一次（Cloudflare公式 docs） | https://developers.cloudflare.com/workers/versions-and-deployments/version-urls/ |
| S17 | Create a Workers Subdomain | **日付記載なし**（2026-10-23 確認） | 一次（Cloudflare公式 HTTP API） | https://developers.cloudflare.com/api/resources/workers/subresources/subdomains/methods/update/ |
| S18 | workers.dev | **更新 2026-09-22** | 一次（Cloudflare公式 docs） | https://developers.cloudflare.com/workers/configuration/routing/workers-dev/ |
| S19 | Wrangler Configuration | **更新 2026-10-20**（正規 URL のページ上の表示） | 一次（Cloudflare公式 docs） | https://developers.cloudflare.com/workers/wrangler/configuration/ |
| S20 | Grant teammates and agents access to specific Workers | **公開 2026-09-15** | 一次（Cloudflare公式 Changelog） | https://developers.cloudflare.com/changelog/post/2026-09-15-granular-worker-permissions/ |
| S21 | Create tokens via API | **更新 2026-09-28** | 一次（Cloudflare公式 docs） | https://developers.cloudflare.com/fundamentals/api/how-to/create-via-api/ |

**最終判断:** まず `Workers: Admin`（Workers プロダクト全体）**だけ**を Legacy 不使用で試験するのは公式の作成権限モデルに合致する。**しかし「アセット付き初回 deploy まで動く最小権限が確定した」とは言えない**。試験成功後に `Specified Workers` + `Workers: Editor` へ縮小し、**アセット更新を含む再デプロイまで成功確認**して、初めてその CI 構成を採用する。
