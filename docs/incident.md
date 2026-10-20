# 事故のときの手順（S9）

サイトの差し替え・トークンの漏れ・GitHub アカウントの乗っ取りのとき、開発者が上から順に行う。**早く止めて、早く知らせる**のを優先する（原因の調べは止めた後）。
画面の項目名は 2026-10 時点の見込みで、Cloudflare・GitHub の画面が変わっていることがある（要確認と書いたものは S9 で実物を見て直す）。

## 兆候

- サイトの見た目・動きが変わった、見覚えの無いスクリプト・外部への通信がある（CSP は外部への通信を断るので、ブラウザの開発者ツールに CSP の違反が出ることがある）。
- Cloudflare から見覚えの無いデプロイ・トークンの作成の通知、監査ログ（Audit Log）に見覚えの無い操作。
- GitHub の Actions に、承認していない「Deploy」の実行・見覚えの無い workflow の変更。
- GitHub から見覚えの無いログイン・2FA の変更の通知。

## 1. 止める

1. **デプロイ用のトークンを消す**: Cloudflare → Account API tokens → 該当のトークン → Delete（Roll でもよい）。その時点で無効になる。
2. **サイトを止める**（中身を差し替えられたとき）: Workers & Pages → `tacpendium-setplay` → Settings → Domains & Routes → `workers.dev` を無効にする（要確認）。止めている間は誰も開けない。
3. **GitHub からのデプロイを止める**: Actions →「Deploy」→「…」→ Disable workflow。

## 2. 確かめる

- Cloudflare の監査ログ: 何が・いつ・どのトークンで行われたか。ほかの Worker が作られていないか（作られていたら消す）。
- Worker の版の履歴（Deployments）: 見覚えの無い版がいつ出たか。
- GitHub: main と各ブランチに見覚えの無いコミット・タグ・Release・workflow の変更が無いか。Settings → Environments・Secrets が変わっていないか。

## 3. 戻す

1. 新しいトークンを作る（`docs/telemetry.md`「デプロイ用の API トークン」）→ Environment `production` の secret を上書き。
2. 正しい版に戻す: Worker の Deployments から前の版へ rollback（要確認）、または main の正しいコミットから Actions →「Deploy」→ Run workflow。Disable した workflow は Enable に戻す。
3. `workers.dev` を有効に戻す。
4. 票（D1）を消された・書き換えられたとき: D1 の Time Travel で前の時点へ戻す（無料プランは 7 日。`npx wrangler d1 time-travel restore tacpendium-setplay --timestamp=<時刻>`。要確認）。

## 4. 知らせる

- 利用者へ: README の冒頭と、固定した Issue に「いつからいつまで、何が起きたか・利用者がすべきこと（なければ無い）」を書く。
- 本サービスが持つのは IP そのものではなく日替わりのハッシュ（`docs/telemetry.md`）。それでも、利用者の情報に触れられたおそれがあるときは、個人情報保護法の報告・通知が要るかを専門家に相談する（法的な判断は Claude はしない。`docs/progress/S7-privacy-research.md`）。

## GitHub アカウントが乗っ取られたとき

- ログインできる: パスワードを変える → Settings → Sessions で他のセッションを切る → SSH 鍵・Personal access token・OAuth アプリ・GitHub Apps・メールアドレス・2FA の方法に見覚えの無いものがあれば消す → Settings → Security log を見る。
- ログインできない: GitHub Support のアカウント復旧に申し出る（リカバリーコード・登録済みの端末・SSH 鍵などで本人確認）。
- その後、上の 1〜4 を行う（乗っ取った人が workflow を書き換えてトークンを使った可能性があるので、トークンは必ず作り直す）。

## 普段の備え

- GitHub: リカバリーコードを保管する。2FA は 2 つ以上（GitHub Mobile ＋ パスキーか認証アプリ）。メールのアカウントにも 2FA。
- Cloudflare: 2FA。トークンは期限つき（3 か月）。期限の 1 週間前をカレンダーに入れる。
