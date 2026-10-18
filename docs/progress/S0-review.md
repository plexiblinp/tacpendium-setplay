# S0 レビュー

- 対象: ブランチ `claude/stage-0-zn02w6`、HEAD `a09689cc456bf8f3046af360c45e036c63f6e189`（origin と一致を `git ls-remote` で確認）
- 基準コミット: `2eaca647e64c0481194f5c7a085db93a8675b801`（`S0-report.md` 計画節の記載と一致）
- レビュー時点の `S0-report.md` は「計画」節のみ。完了節（やったこと・証拠・diff --stat）はまだ無いので、**計画と実際の差分**で判定した。

## 1. 要約

- 完了条件「CI が空で緑」は満たしている。CI run `37410593105`（`a09689c`、push）は 3 job（リポジトリ検査 / Go / Web）とも `success`。Go・Web は `go.mod` / `web/package.json` が無いので中身の step は skipped（設計どおり）。
- ライセンス一式の単体版化は計画どおり。旧記述（shadcn・autopilot・M26・SUPPORT/SECURITY・D-xxx・combomgr・private・非公開）の残りは 0 件。`_reference/` に変更は無い。Action の SHA 4 種は実在のタグと一致した。
- **高 1 件**: `DATA-LICENSE.md` §0 の「入力はメモリと URL にだけある」が、HANDOVER §7.1・§7.3 の計測設計（入力をビーコンで送って保存する）と食い違う。
- **中 4 件**: ライセンス検査が表の並べ替え・覆い隠しを検出しない（変異で確認）／CI が想定外の場所の `go.mod`・`package.json` を黙ってスキップして緑になる／CC BY-SA のクレジット例に必要な要素が欠けている／NOTICE の英語技名の説明が D7 の上書き表と合わない。

## 2. 完了条件への適合

| 項目 | 判定 | 根拠 |
|---|---|---|
| HANDOVER §9 S0「CI が空で緑」 | 済 | run 37410593105: conclusion=success。job: リポジトリ検査=success（ライセンス・SHA 固定・構文の各 step が success）、Go=success（gofmt/vet/test/Summary は skipped）、Web=success（pnpm 以降は skipped） |
| AGENTS.md / CLAUDE.md（§11・§12-2） | 済（既存） | 既存コミットにある。settings.json が参照する hook 3 本（`session-start.sh` `post-edit-check.sh` `stop-test.sh`）は全部実在 |
| ライセンス一式の単体版化 | 済（指摘あり） | REUSE.toml の四表（A=`**` / C / B=`data/**` / `_reference/**`）、NOTICE・DATA-LICENSE・LicenseRef を書き換え済み。`LICENSE`・`LICENSES/{AGPL,CC-BY-SA,MIT}.txt` は公開版 tacpendium の同名ファイルと `cmp` で一致し、`LICENSE` と `LICENSES/AGPL-3.0-or-later.txt` も一致。内容の指摘は 高-1、中-3、中-4、低-2〜4 |
| 検査スクリプト 2 本 | 済（指摘あり） | どちらも EXIT=0 で違反 0 件。計画にある壊し方では赤くなる（§5）。検出できない壊し方は 中-1・低-1 |
| CI 骨格 ci.yml | 済（指摘あり） | `permissions: contents: read`、concurrency、`paths` フィルタ無し、`persist-credentials: false`、`pipefail`、`-count=1`、`--run`、ANSI 除去、件数が読めないときは「読めなかった」。中-2・低-5 |
| Dependabot | 済 | github-actions を週 1 回、グループ化。低-6 |
| .gitignore | 済 | 計画の項目に加えて `__pycache__/`。計画から広げた分は完了報告に書くこと |
| README | 済 | 非公式と書いてある。サイト名に「SF6」を使っていない（仮の呼称と明記）。三層の表がある |
| 計画節の検証項目（わざと壊して赤を確認、`EXIT=` の逐語、`git diff --stat`） | 未（報告前） | 完了節がまだ無い。計画とのずれ（gofmt を `gofmt -l .` から `git ls-files ':!:_reference/**'` に変えた点、`go test -v` にした点）も完了報告に書くこと |

## 3. 指摘

### 高

**高-1　`DATA-LICENSE.md:5-7` の「保存しない」「メモリと URL にだけある」が、計測の設計（HANDOVER §7.1・§7.3、D3・D14）と食い違う**
- 内容: §0 には「本サービスは利用者のデータを保存しない。入力（キャラ・KA・絞り込み）はブラウザのメモリと URL にだけあり」とある。しかし HANDOVER §7.1 では、提案を実行するたびに `{char, ka, mode, …, filters, notation, lang, resultCount, …}` を `POST /api/beacon` で送り、Workers Analytics Engine に書く。§7.3 では `hash(IP + UA + 日替わりソルト)` を記録する。S6 が実装されると、この文は事実と合わなくなる。
- なぜ問題か: 利用者に見える権利・プライバシーの文書が、決定済みの仕様と反対のことを言っている。テストでは気づけない。AGENTS.md §7.3 の「ブラウザストレージを使わない」と「サーバへ送らない」が混ざっている。
- 直し方の案: §0 を「本サービスは利用者アカウントを持たず、利用者が作るデータ（コンボ・メモ等）も扱わない。入力はブラウザのメモリと URL で扱い、cookie やブラウザストレージは使わない。実行の計測（入力条件と結果件数）は送るが、その内容はプライバシーポリシーで説明する（S6）」のように書き分ける。権利を主張しない旨は残してよい。S6 で計測を実装するとき、プライバシーポリシーと一緒にこの節を直すことを完了報告の次段入力に書く。

### 中

**中-1　`scripts/check-licenses.py:95-129` は、表を並べ替えたり、ほかの表に覆い隠されたりしても検出しない**
- 内容: 表ごとの検査は「グロブが追跡ファイルに 1 件以上当たるか」だけで、「その表が最後に当たる表として、1 件以上のファイルを実際に決めているか」は見ていない。写しで試した結果:
  - M6: 既定の表 `**`（AGPL）を末尾へ移す → 166 件すべてが AGPL に解決される（`_reference/**` も層 C も消える）のに `EXIT=0 違反: 0 件`
  - M7: 層 C の `.claude/**` を `.claude/*` に変える → `.claude/hooks/*`・`.claude/commands/*` の 8 件が黙って AGPL に落ちるのに `EXIT=0`
- なぜ問題か: REUSE.toml の冒頭には「並び順が意味を持つ。並べ替えないこと」とあるが、それを守らせる検査が無い。本家 D-777 の「外れた先が既定なので緑のまま」という状態が、並べ替えや一部の外れでは今も起きる。docstring の「黙って外れる」問題への対策としては、半分しか効いていない。
- 直し方の案: (a) PENDING 以外の各表について、勝った件数が 0 なら違反にする（M6 を検出できる）。(b) 代表パスと期待するライセンスの表を置き、一致しなければ違反にする。例: `_reference/combomgr/SOURCE_COMMIT`→NotForDistribution、`.claude/hooks/stop-test.sh`・`scripts/check-licenses.py`・`docs/progress/S0-report.md`→MIT、`.github/workflows/ci.yml`→AGPL。これで M7 も検出できる。追加したら、M6・M7 で 1 件ずつ赤くなることを確かめる（§8-3）。

**中-2　`.github/workflows/ci.yml:62-70, 108-116` は、ルートの `go.mod` と `web/package.json` しか見ない。ほかの場所にできても黙ってスキップして緑になる**
- 内容: 判定は `[ -f go.mod ]` と `[ -f web/package.json ]` だけ。HANDOVER §5 にはエンジンを `engine/` パッケージに置くとあり、`engine/go.mod` のような構成になれば、Go の job は「go.mod なし: スキップ」で success になる。hook（`session-start.sh` と `stop-test.sh`）は `find` で全ディレクトリの `go.mod` / `package.json` を探すので、hook と CI で見る範囲がずれる。
- なぜ問題か: AGENTS.md §9 が戒める「失敗しているのに緑」の形そのもの。必須 check を登録しても、スキップは緑として通ってしまう。
- 直し方の案: 検出の step で `git ls-files -- '*go.mod' ':!:_reference/**'` と `'*package.json'` を数える。想定外の場所にあれば「CI が見ていない go.mod: <path>」と出して `exit 1` にする。S3・S5 でディレクトリが決まったら、想定の場所を書き換える。

**中-3　`DATA-LICENSE.md:69`、`NOTICE:39-40`、`README.md:168` のクレジット例が、CC BY-SA 4.0 §3(a)(1) の要素を満たしていない**
- 内容: 例は「データ © Tacpendium contributors, CC BY-SA 4.0 — <リポジトリ URL>」。ライセンスへの URI（https://creativecommons.org/licenses/by-sa/4.0/）が無い。DATA-LICENSE §2 で自ら「改変（表記の追加・形式の変換）し、派生データを作る」と書いているのに、改変したことの表示も無い（§3(a)(1)(B)）。
- なぜ問題か: この例は S5 の画面フッターと再配布者の表示にそのまま写される見込み。推測: 公開時に帰属の条件を欠く。HANDOVER §8 には「クレジット表示を付ける」としか書かれておらず、表示の中身はライセンス判断に当たる（AGENTS §3-6）。
- 直し方の案: 「データ: Tacpendium contributors（https://github.com/plexiblinp/tacpendium）、CC BY-SA 4.0（https://creativecommons.org/licenses/by-sa/4.0/）。本サービスで表記の追加・形式の変換・事前計算を行った」のように TASL と改変の表示に揃える。文言を確定する前に、開発者へ確認事項として挙げる。

**中-4　`NOTICE:31-33` の「英語技名・SRK 表記・ナンバリング表記は…規則で生成したものである」が、D7 の「+少数の上書き表」と合わない**
- 内容: D7 では、英語技名は `move_code` から規則で生成し、少数を上書き表で補う。上書き表は手で書くもので、規則による生成ではない。出所（誰の手入力か、どこから取ったか）を説明する節に、上書き表のことが書かれていない。また、まだ無いデータを「取り込んだもの」「生成したもの」と過去形で書いている（AGENTS §8-2）。
- なぜ問題か: NOTICE §2 の論点は「どこから取得したか」。上書き表は、公式の英語技名を転記する入口になりうる（§8「公式データの混入禁止」）。出所の宣言から漏れると、S2 で上書き表を作るときに根拠が無くなる。
- 直し方の案: 「規則で生成し、規則で表せない少数は開発者が手入力した上書き表で補う（公式サイト等からは転記しない）」と書き、時制を「取り込む」「生成する」にする。S2 の次段入力に「上書き表の出所の規則」を書く。

### 低

**低-1　`scripts/check-actions-pinned.sh:27-30` は `permissions:` があるかどうかしか見ない**
- 写しで試した結果、`contents: write` にしても（P3）、`permissions: write-all` にしても（P4）`EXIT=0`。`"uses": x@v4` のように key を引用符で囲んだ形も見逃す（P6）。AGENTS §9・HANDOVER §7.4 は「最小」を求めている。スクリプトの説明（「持つ」）とは合っているが、README と ci.yml の step 名「Action の SHA 固定と permissions」からは、権限の中身まで見ているように読める。
- 案: トップレベルは `contents: read` または `{}` に限る。job 単位の `write` は許可リスト（ファイル:job）で明示させる。

**低-2　`NOTICE:26` の「データ SQL」の範囲が書かれていない**
- 公開版 tacpendium（`ffdb69e`）の REUSE.toml では、層 B は `character_data/**`（`seed-progress.md` を除く）と `migrations/*_data_*.sql` だけ。`000002_seed_games`・`000007_seed_presets` などの `_data_` の付かない SQL は AGPL。S2 でこれらを取り込むと、層 B（CC-BY-SA）として配るものの中に上流では AGPL のものが混ざる。
- 案: 「データ SQL（公開版の `migrations/*_data_*.sql`）」と明記し、S2 の次段入力にも書く。

**低-3　`DATA-LICENSE.md:63-65` は、帰属表示の正本を REUSE.toml だとしているのに、値が REUSE.toml と違う**
- 本文の値は「`Tacpendium contributors`」、`REUSE.toml:52`（層 B）の値は「`2026 plexiblinp and the Tacpendium contributors`」。公開版の書き方を受け継いだもの。
- 案: どちらかに揃える。または「表示に使う名義は Tacpendium contributors。REUSE.toml の値はファイル単位の著作権表示」と役割を書き分ける。

**低-4　DATA-LICENSE §1 の層の説明と、REUSE.toml による実際の解決が一部合わない**
- `check-licenses.py` の出力では AGPL が 7 件: `.github/dependabot.yml`、`ci.yml`、`upstream-watch.yml`、`.gitignore`、`README.md`、`NOTICE`、`DATA-LICENSE.md`。このうち `upstream-watch.yml` は §1 で層 C と書かれた「上流追従の道具」なのに層 A になっている。README・NOTICE・DATA-LICENSE は文書なのに、層 A の説明は「コード、スキーマ、設定」。
- 案: `.github/workflows/upstream-watch.yml` を層 C に足すか、§1 の層 A の説明に「リポジトリの案内文書・CI 設定」を書き加える。

**低-5　`ci.yml:7-10` の `push`（全ブランチ）と `pull_request` が重なり、PR の付いたブランチでは同じコミットに対して 2 回走る**
- private の間は Actions の分数が月 2,000 分に限られる（HANDOVER §7.5）。今は 1 回 10 秒程度なので影響は小さいが、S4 の事前計算が入ると効いてくる。
- 案: `pull_request: branches: [main]` に絞るか、push と PR の重複を S4 で見直すと次段入力に書く。

**低-6　`.github/dependabot.yml` に `commit-message` の指定が無い**
- Dependabot の PR のタイトルとコミットメッセージが Conventional Commits（AGENTS §6）の形にならない。
- 案: `commit-message: { prefix: "chore(deps)" }`。

## 4. 良かった点

- Action の SHA 4 種（checkout `3d3c42e5…`=v7.0.1、setup-go `b7ad1dad…`=v7.0.0、setup-node `82076278…`=v7.0.0、pnpm/action-setup `ea17c68d…`=v6.1.0 の注釈付きタグの `^{}`）は、`git ls-remote --tags` の結果とすべて一致した。注釈付きタグはタグオブジェクトではなくコミットの側を選んでいる。
- gofmt は `gofmt -l` の出力が空でなければ落とす形で、`_reference/` を除外している。Summary は件数を読めないとき「件数を読めなかった」と出す。AGENTS §9 の教訓が具体的に反映されている。
- 検査スクリプトは計画にある壊し方（LICENSES の欠落・未使用、当たらないグロブ、PENDING が当たり始めた、ディレクトリの移動、タグ参照、短縮 SHA、permissions の欠落）でそれぞれ違反 1 件・EXIT=1 になった。「1 件ある入力で 1 件と出る」を満たしている。
- PENDING に段階を付け、当たり始めたら違反にする設計で、「まだ無い」と「外れた」を区別している（§8-7）。
- 旧記述の残りは 0 件。NOTICE・LicenseRef から本家の内部記号（D-xxx・M番号）がきれいに消えている。

## 5. 自分で回したコマンドと結果

作業ツリーでは読み取りだけを行った。変異は `/tmp/rv-mut`（リポジトリの写し）で行った。

```
$ python3 scripts/check-licenses.py > /tmp/rv-lic.txt 2>&1; echo EXIT=$?
EXIT=0
追跡ファイル: 166 件（LICENSES/・LICENSE・REUSE.toml を除く）
  AGPL-3.0-or-later: 7 件
  LicenseRef-Tacpendium-NotForDistribution: 141 件
  MIT: 18 件
NOTE: 表 3: 'data/**' は 0 件（PENDING: S2 で作る）
違反: 0 件

$ bash scripts/check-actions-pinned.sh > /tmp/rv-pin.txt 2>&1; echo EXIT=$?
EXIT=0
workflow: 2 本 / uses 参照: 7 件
違反: 0 件

$ for f in .claude/hooks/*.sh scripts/*.sh upstream/*.sh; do bash -n "$f" || echo BAD $f; done
（出力なし＝全部通過）

$ git diff --stat 2eaca647e64c0481194f5c7a085db93a8675b801
 11 files changed, 554 insertions(+), 428 deletions(-)   ※ _reference/ の変更 0、削除されたテストなし（基準にテストは無い）

$ grep -rn -i -e shadcn -e autopilot -e M26 -e SUPPORT.md -e SECURITY.md -e 'D-[0-9]' -e M40 -e combomgr -e private -e 非公開 \
    NOTICE DATA-LICENSE.md REUSE.toml LICENSES/LicenseRef* README.md .gitignore .github scripts; echo GREP_EXIT=$?
GREP_EXIT=1（0 件）
```

変異テスト（`/tmp/rv-mut`）:

```
[M1-missing-MIT]      EXIT=1 ERROR: LICENSES/MIT.txt がありません（REUSE.toml で使用） 違反: 1 件
[M2-unused]           EXIT=1 ERROR: LICENSES/BSD-3-Clause.txt はどの表からも使われていません 違反: 1 件
[M3-nohit]            EXIT=1 ERROR: 表 2: 'tools/**' が追跡ファイルに 1 件も当たりません 違反: 1 件
[M4-pending-hit]      EXIT=1 ERROR: 表 3: 'data/**' は PENDING (S2) だが 1 件当たる。PENDING から消すこと 違反: 1 件
[M5-moved-upstream]   EXIT=1 ERROR: 表 2: 'upstream/**' が追跡ファイルに 1 件も当たりません 違反: 1 件
[M6-default-last]     EXIT=0 違反: 0 件      ← 中-1（既定の表を末尾へ移すと全件 AGPL になるのに緑）
[M7-claude-star]      EXIT=0 違反: 0 件      ← 中-1（.claude/** を .claude/* にすると 8 件が AGPL に落ちるのに緑）
[P1-tag]              EXIT=1 ERROR: ci.yml:25: SHA 固定でない参照: actions/checkout@v7 違反: 1 件
[P2-noperm]           EXIT=1 ERROR: ci.yml: トップレベルの permissions: がありません 違反: 1 件
[P3-write]            EXIT=0 違反: 0 件      ← 低-1
[P4-writeall]         EXIT=0 違反: 0 件      ← 低-1
[P5-shortsha]         EXIT=1 ERROR: ci.yml:25: SHA 固定でない参照: actions/checkout@3d3c42e5 違反: 1 件
[P6-quotedkey]        EXIT=0 違反: 0 件      ← 低-1
```

Action の SHA（`git ls-remote --tags https://github.com/<repo>`）:

```
actions/checkout    3d3c42e5aac5ba805825da76410c181273ba90b1  refs/tags/v7.0.1
actions/setup-go    b7ad1dad31e06c5925ef5d2fc7ad053ef454303e  refs/tags/v7.0.0
actions/setup-node  820762786026740c76f36085b0efc47a31fe5020  refs/tags/v7.0.0
pnpm/action-setup   ea17c68df8912ef543352723c149a84f56e3d413  refs/tags/v6.1.0^{}
```

CI（GitHub MCP `actions_list`）: run `37410593105`、head_sha `a09689c…`、event=push、conclusion=success。job: リポジトリ検査=success、Go=success、Web=success。

公開版 tacpendium（`/tmp/rv-up/up` に読み取り専用で clone、HEAD `ffdb69e Release 0.2.0`。`upstream/tacpendium.lock` と一致）:
- `LICENSE`・`LICENSES/{AGPL-3.0-or-later,CC-BY-SA-4.0,MIT}.txt` は本リポジトリと `cmp` で一致（4 件とも exit 0）。
- 層 B は `character_data/**`・`migrations/*_data_*.sql`・`internal/seedgen/testdata/**`、SPDX-FileCopyrightText は「2026 plexiblinp and the Tacpendium contributors」。NOTICE §3 に撤退ポリシーと窓口 `plexiblinp.contact@gmail.com` がある。本リポジトリの NOTICE の「撤退ポリシーを引き継ぐ」「窓口は同じ」と合っている。

## 取り込み（実装担当が追記）

| ID | 採否 | 理由・対応 |
|---|---|---|
| 高-1 | 採用 | `DATA-LICENSE.md` §0 を「入力に権利を主張しない」に改め、計測（HANDOVER §7.1・§7.3）を予定として明記し、扱いは S6 のプライバシーポリシーへ送った。「保存しない」「メモリと URL にだけ」の記述は削除。 |
| 中-1 | 採用 | `check-licenses.py` に (5)「各表が勝つファイル 1 件以上」(6)「代表パスの期待ライセンス（EXPECT）」を追加。指摘の 2 変異（既定を末尾へ／`.claude/*`）がともに EXIT=1 になることを写しで確認（報告 §証拠）。 |
| 中-2 | 採用 | `ci.yml` の検査 job に「go.mod / package.json の置き場所」step を追加。ルートの `go.mod` と `web/package.json` 以外に追跡されていたら落とす（`engine/go.mod` を置くと FAIL、ルートの `go.mod` は OK を確認）。 |
| 中-3 | 採用（聞かない） | CC BY-SA 4.0 §3(a)(1) の要件（帰属・ライセンス URI・改変の表示）を満たす表示例へ直した。ライセンス自体の選択（HANDOVER §8）は変えておらず、要件への適合なので開発者確認は不要と判断。 |
| 中-4 | 採用 | NOTICE §2 を「生成し、少数を上書き表で人が補う（S2 で作る）」と未来形にし、上書き表も公式から転記しないと明記。 |
| 低-1 | 一部採用 | `write-all` を違反にした（変異で EXIT=1 確認）。個別の `: write` は将来の job で必要になりうるので止めない。引用符付き `"uses":` は本リポジトリで書かないため見送り。 |
| 低-2 | 採用 | NOTICE に `migrations/*_data_*.sql`（公開版で CC-BY-SA と宣言された範囲）と範囲を明記。 |
| 低-3 | 採用 | 帰属表示を REUSE.toml・公開版と同じ `plexiblinp and the Tacpendium contributors` に揃えた（DATA-LICENSE・NOTICE・README）。 |
| 低-4 | 採用 | `.github/**` を層 C（MIT）へ。層 A の説明に「利用者向けの文書（README・NOTICE・DATA-LICENSE）」を足した（REUSE.toml・DATA-LICENSE §1）。 |
| 低-5 | 採用 | `pull_request` トリガーを外した（push が全ブランチを覆う。フォーク PR を受ける public 化後に戻す旨を ci.yml に記載）。 |
| 低-6 | 採用 | dependabot に `commit-message.prefix: "chore(deps)"`。 |

## 再レビュー（2 往復目）

- 対象: HEAD `dad3680e7f82c36b13f97704c25be67d2174429b`（`git ls-remote origin claude/stage-0-zn02w6` と一致）。基準 `2eaca647e64c0481194f5c7a085db93a8675b801`。
- 見たもの: `git diff a09689c dad3680`（取り込み差分）、現在の `REUSE.toml`・`NOTICE`・`DATA-LICENSE.md`・`README.md`・`LICENSES/LicenseRef-*`・`.github/**`・`scripts/**`・`.gitignore`、HANDOVER §1・§7・§8・§9・§10・§11・§13、公開版 tacpendium `ffdb69e` の `REUSE.toml`・`DATA-LICENSE.md`。
- `S0-report.md` の完了節が未作成である点は指摘しない（依頼どおり）。

### 要約

- 取り込み 11 件はすべて差分に反映されていた。変異で確かめた限り、中-1・中-2 の検査は狙いどおり赤くなる。CI run `37411112725`（`dad3680`、push）は 3 job とも success、追加の「go.mod / package.json の置き場所」step も success。
- NG は無し。一部 OK が 2 件（中-4・低-1。残りは下の新指摘に回した）。
- 新しい指摘: 高 0、**中 1**（再-中-1: `DATA-LICENSE.md` の「匿名の」が HANDOVER §7.3 の仮名化の設計と合わない）、低 6。

### 取り込みの検証

| ID | 判定 | 根拠 |
|---|---|---|
| 高-1 | OK | `DATA-LICENSE.md:5-10`。「保存しない」「メモリと URL にだけ」は消えた（grep で 0 件。`AGENTS.md:90` のヒットはフロント規約で別の話）。計測を予定として書き、扱いを S6 のプライバシーポリシーへ送っている。ただし、新しく入った「匿名の」の語が別の食い違いを生んでいる → 再-中-1 |
| 中-1 | OK | (5) 勝つファイル 0 件、(6) EXPECT を追加（`scripts/check-licenses.py:36-49, 140-168`）。写し `/tmp/rv2-mut` で試した: M6（既定を末尾へ）EXIT=1・違反 10 件、M7（`.claude/*`）EXIT=1・1 件、M8（`.github/**` を削除）EXIT=1・1 件、M10（`_reference` の識別子を変える）EXIT=1・2 件、M13（`data/` にファイルを足す）EXIT=1・1 件。表の添字ずれ対策（`tables.append(([], ""))`）も正しい。表の中の 1 グロブだけが覆い隠される形は検出しない → 再-低-1 |
| 中-2 | OK | `ci.yml:39-42`。写しで試した: 何も無い／ルート `go.mod`／`web/package.json` → OK、`engine/go.mod`・ルート `package.json`・`web/e2e/package.json` → FAIL、`_reference/x/go.mod`（`-f` で追跡）→ 除外されて OK。`:(glob)**/go.mod` はルートの `go.mod` にも当たる（OK 側で確認）。CI でも当該 step は success |
| 中-3 | OK | `DATA-LICENSE.md:62-67`、`NOTICE:41-44`、`README.md:24`。作成者の表示・素材への URI・ライセンスの URI・改変の表示がそろった（CC BY-SA 4.0 §3(a)(1)(A)(i)(iii)(v)、(B)、(C)）。「開発者に聞かない」は判断として理由が書かれており妥当だと考える。ただし完了報告の「判断と理由」に、開発者確認を省いたことを書くこと |
| 中-4 | 一部 OK | NOTICE §2（`NOTICE:32-36`）は上書き表と未来形を書き、公式から転記しないと明記した。一方、同じ時制の問題が `DATA-LICENSE.md:54` に残っている → 再-低-2 |
| 低-1 | 一部 OK（取り込み表の「一部採用」と合う） | 写しで試した: トップレベル `permissions: write-all` → EXIT=1、job 単位の `write-all` → EXIT=1、`contents: write` → EXIT=0（意図どおり）。`permissions: "write-all"`（引用符付き）と `permissions: { contents: write, actions: write }` は EXIT=0 → 再-低-3 |
| 低-2 | OK | `NOTICE:26-28` に `migrations/*_data_*.sql` と書いた。公開版 `ffdb69e` の REUSE.toml 層 B のグロブと一致。ただし `character_data/` にも同じ限定が要る → 再-低-4 |
| 低-3 | OK | 「`Tacpendium contributors`」を含み「plexiblinp and the」の付かない行は、NOTICE・DATA-LICENSE・REUSE・README・`.github`・`scripts`・`upstream`・AGENTS・`.claude`・S0-report で 0 件。REUSE.toml 層 B の値と一致する。推測: 公開版の DATA-LICENSE §5 は今も `Tacpendium contributors` なので、「取込元と同じ値」は REUSE.toml の値の話として正しい |
| 低-4 | OK | `REUSE.toml:39` に `.github/**` を足した。解決は AGPL 4 件（`.gitignore`・README・NOTICE・DATA-LICENSE）、MIT 22 件。DATA-LICENSE §1 の説明（層 A に文書と設定、層 C に CI 設定）と合う。README の表の書き方は → 再-低-5 |
| 低-5 | OK | `ci.yml:5-11`。同じリポジトリのブランチから出す PR では、push の run がそのコミットの check として PR に付くので、必須 check の運用は壊れない（推測: マージ結果のコミットは試されなくなるが、private の間は許容範囲） |
| 低-6 | OK | `.github/dependabot.yml:8-9`。 |

### 新しい指摘

#### 中

**再-中-1　`DATA-LICENSE.md:8-9` の「匿名の集計用に送る」が、HANDOVER §7.3・D14 の設計（仮名化）と合わない**
- 内容: 高-1 を直した文に「匿名」の語が入った。HANDOVER §7.3 では `hash(IP + UA + 日替わりソルト)` を記録する。照合先には「GDPR の仮名化データの扱い」「個人情報保護法の個人関連情報」を挙げ、「法的助言ではないため公開前に照合する」としている。日替わりソルトのハッシュは、その日のうちは同じ利用者を結び付けられる仮名識別子で、一般に「匿名」とは区別される。
- なぜ問題か: 利用者に見える文書が、設計が公開前の法的照合に回しているプライバシー上の性質を、先に「匿名」と言い切っている。高-1 と同じ種類の問題（docs と決定済みの仕様の食い違い）で、テストでは気づけない。中にした理由は、文が「予定」で、詳細を S6 に送っているため。
- 直し方の案: 「匿名の」を削る。たとえば「実行ごとの条件と結果件数などを集計用に送る。利用者数の推定のため IP 等を日替わりソルトでハッシュした値を使い、生 IP は保存しない」と事実だけを書く。性質の評価（匿名か仮名か）はプライバシーポリシー（S6）と §7.3 の照合に任せる。

#### 低

**再-低-1　`scripts/check-licenses.py:156-161` の「勝つファイル」は表の単位で数えるので、表の中の 1 グロブが丸ごと覆い隠されても検出しない**
- 写しで試した M14: S2 を想定して `data/a.json`・`data/README.md` を足し、PENDING を外し、層 C（表 2）に `"data/README.md"` を足した → `EXIT=0 違反: 0 件`。`data/README.md` は後ろの層 B に覆い隠され、CC-BY-SA に解決される（MIT のつもりで書いた行が効いていない）。M12（表 2 に `_reference/combomgr/claude/**` を足す）も EXIT=0。docstring 5 と `REUSE.toml:19` の「並べ替え・覆い隠しの検出」は、表の単位の検出として読めば正しいが、グロブの単位では効かない。
- 案: 勝ちをグロブの単位で数え、「当たるが 1 件も勝たないグロブ」を違反にする。S2 でデータ置き場に MIT/AGPL の例外を書く可能性が高いので、そのときまでに入れる。追加したら M14 で赤くなることを確かめる。あわせて、`data/**` に代表パスを EXPECT へ足すことを、PENDING を外すときの手順として PENDING のコメントに書く。

**再-低-2　`DATA-LICENSE.md:54` の「そこから取り込み・生成したものである」が、まだ無いデータを過去形で書いている（中-4 の直しが届いていない）**
- 層 B のファイルはまだ 0 件（check-licenses の NOTE で確認）。AGENTS §8-2。
- 案: 「取り込み・生成する」とし、「取り込んだ版は `upstream/tacpendium.lock` のコミットで追える」を「追えるようにする（S2）」にする。

**再-低-3　`scripts/check-actions-pinned.sh:33` は引用符付きの `"write-all"` と、フロー形式で書いた write の権限を見逃す**
- 写しで試した: `permissions: "write-all"` → EXIT=0、`permissions: { contents: write, actions: write }` → EXIT=0。
- 案: 正規表現を `permissions:[[:space:]]*["']?write-all` にする。フロー形式はスクリプトの説明（個別の write は止めない）の範囲内なので、説明に「フロー形式は見ない」と書くだけでもよい。

**再-低-4　`NOTICE:26-28` の範囲の限定が SQL にしか掛かっていない。公開版では `character_data/seed-progress.md` が層 A（AGPL）**
- 公開版 `ffdb69e` の `REUSE.toml` (5) は `character_data/seed-progress.md` を AGPL に戻している。NOTICE は `character_data/` 全体を CC-BY-SA の取込元と読める書き方をしている。
- 案: 括弧書きを両方に掛ける。たとえば「`character_data/` とデータ SQL（`migrations/*_data_*.sql`）のうち、同プロジェクトで CC-BY-SA-4.0 と宣言された範囲」とする。S2 の次段入力に「取込は上流 REUSE.toml の層 B 解決に従う」と書く。

**再-低-5　取り込みで、S7 の grep（HANDOVER §13）に掛かる語が 2 か所に入った。あわせて README の表の書き方が層 A と合っていない**
- 1 回目は 0 件だった grep（`combomgr`・`private` ほか、NOTICE・DATA-LICENSE・REUSE・LicenseRef・README・.gitignore・.github・scripts）が、今は 2 件ヒットする。`scripts/check-licenses.py:48`（`_reference/combomgr/SOURCE_COMMIT`）と、`.github/workflows/ci.yml:5`（「private の間」）。どちらも今は必要な記述で、S7 で `_reference/` を消せば EXPECT と表 4 が赤くなるので気づける。ただし完了報告の「次の段階への入力」に、S7 で直す箇所として書いておくこと。
- `README.md:24` の「アプリ本体（コード）」は、README・NOTICE・DATA-LICENSE と `.gitignore` も層 A である（DATA-LICENSE §1）ことと合わない。「アプリ本体（コード・設定・利用者向け文書）」とする。

**再-低-6　計画節と実装のずれ、CI の Summary に新しい step が出ない**
- `docs/progress/S0-report.md:26`（「利用者データを保存しない」に置換）と `:34`（`pull_request` を入れる）は、取り込み後の実装と違う。層 C に `.github/**` を足したこと、`write-all` の検査、置き場所の step も計画に無い。計画節は経緯として残してよいが、完了報告で「計画から変えた点」として列挙すること（AGENTS §8-2「範囲を広げたら、広げた事実を書く」）。
- `ci.yml:46-54` の Summary は、ライセンスと Action 固定の 2 行だけを出す。置き場所の step の結果は出ない。失敗すれば job は赤になるので実害は小さい。案: `- 置き場所: OK / 想定外 <path>` の 1 行を足す。

### 良かった点（2 往復目）

- 中-1 の直しは、指摘した M6・M7 に加え、`.github/**` の削除（M8）や識別子のすり替え（M10）にも効いた。EXPECT は「存在しないパスも違反」としてあるので、代表パスを動かしたときに黙って外れることも無い。
- 中-2 の除外 `:!:_reference/**` は、実際に `_reference` 下へ `go.mod` を置いて効くことを確かめた。
- 帰属表示の値が REUSE.toml・NOTICE・DATA-LICENSE・README の 4 か所でそろった。

### 自分で回したコマンドと結果（2 往復目）

作業ツリーでは読み取りだけを行った（本ファイルの追記を除く）。変異は `/tmp/rv2-mut`（`dad3680` の clone）で行った。

```
$ python3 scripts/check-licenses.py > /tmp/rv2-lic.txt 2>&1; echo EXIT=$?
EXIT=0
追跡ファイル: 167 件（LICENSES/・LICENSE・REUSE.toml を除く）
  AGPL-3.0-or-later: 4 件
  LicenseRef-Tacpendium-NotForDistribution: 141 件
  MIT: 22 件
NOTE: 表 3: 'data/**' は 0 件（PENDING: S2 で作る）
違反: 0 件

$ bash scripts/check-actions-pinned.sh > /tmp/rv2-pin.txt 2>&1; echo EXIT=$?
EXIT=0
workflow: 2 本 / uses 参照: 7 件
違反: 0 件

$ for f in .claude/hooks/*.sh scripts/*.sh upstream/*.sh; do bash -n "$f" || echo BAD $f; done > /tmp/rv2-syn.txt 2>&1; echo EXIT=$?
EXIT=0（出力なし）

$ git diff --stat 2eaca647e64c0481194f5c7a085db93a8675b801 -- _reference | wc -l
0
$ git diff --stat 2eaca647e64c0481194f5c7a085db93a8675b801 HEAD
 12 files changed, 794 insertions(+), 429 deletions(-)

$ grep -rn 'Tacpendium contributors' <NOTICE DATA-LICENSE.md REUSE.toml LicenseRef README .github scripts upstream AGENTS.md .claude S0-report> | grep -v 'plexiblinp and the Tacpendium contributors'
G1 EXIT=1（0 件）
$ grep -rn -i -e shadcn -e autopilot -e M26 -e SUPPORT.md -e SECURITY.md -e 'D-[0-9]' -e M40 -e combomgr -e private -e 非公開 NOTICE DATA-LICENSE.md REUSE.toml LICENSES/LicenseRef* README.md .gitignore .github scripts
G4 EXIT=0（2 件: ci.yml:5 "private"、check-licenses.py:48 "combomgr"）← 再-低-5
```

変異（`/tmp/rv2-mut`。check-licenses.py）:

```
[M6  既定を末尾へ]                 EXIT=1 違反: 10 件（表 1・表 3 の勝ち 0 件 + EXPECT 8 件）
[M7  .claude/** → .claude/*]       EXIT=1 ERROR: EXPECT: .claude/hooks/session-start.sh は MIT のはずが AGPL-3.0-or-later 違反: 1 件
[M8  .github/** を削除]            EXIT=1 ERROR: EXPECT: .github/workflows/ci.yml は MIT のはずが AGPL-3.0-or-later 違反: 1 件
[M9  docs/progress/** → *.txt]     EXIT=1 違反: 2 件（当たらないグロブ + EXPECT）
[M10 _reference の識別子 → CC-BY-SA] EXIT=1 違反: 2 件（未使用 LICENSES + EXPECT）
[M11 tools/gen.sh を追加]           EXIT=0 違反: 0 件（既定 AGPL へ。設計どおり）
[M12 表 2 に _reference/combomgr/claude/** を追加] EXIT=0 違反: 0 件 ← 再-低-1
[M13 data/a.json を追加]            EXIT=1 ERROR: 表 3: 'data/**' は PENDING (S2) だが 1 件当たる 違反: 1 件
[M14 data/ 作成・PENDING 除去・表 2 に data/README.md] EXIT=0 違反: 0 件（data/README.md は CC-BY-SA に解決）← 再-低-1
```

変異（ci.yml の置き場所 step の式をそのまま実行）:

```
[none] OK  [root go.mod] OK  [web/package.json] OK
[engine/go.mod] FAIL: engine/go.mod  [package.json] FAIL: package.json  [web/e2e/package.json] FAIL: web/e2e/package.json
[_reference/x/go.mod] 出力なし（除外される）
```

変異（check-actions-pinned.sh）:

```
[P4  top write-all]               EXIT=1 ERROR: ci.yml: permissions: write-all は使わない 違反: 1 件
[P4b job write-all]               EXIT=1 ERROR: ci.yml: permissions: write-all は使わない 違反: 1 件
[P4c "write-all" 引用符付き]      EXIT=0 違反: 0 件 ← 再-低-3
[P3  contents: write]             EXIT=0 違反: 0 件（意図どおり）
[P7  { contents: write, actions: write }] EXIT=0 違反: 0 件 ← 再-低-3
```

公開版 tacpendium（`/tmp/rv2-up/up`、`git clone --depth 1` EXIT=0、HEAD `ffdb69e`。`upstream/tacpendium.lock` と一致）:
- `cmp` で `LICENSE`・`LICENSES/{AGPL-3.0-or-later,CC-BY-SA-4.0,MIT}.txt` を比べた: 4 件とも EXIT=0
- 層 B の SPDX-FileCopyrightText は「2026 plexiblinp and the Tacpendium contributors」。`migrations/*_data_*.sql` は層 B、`character_data/seed-progress.md` は (5) で AGPL に戻されている

CI（GitHub MCP `actions_list`）: run `37411112725`、head_sha `dad3680…`、event=push、conclusion=success。job: リポジトリ検査=success（「go.mod / package.json の置き場所」step も success）、Go=success（gofmt 以降は skipped）、Web=success（pnpm 以降は skipped）。

## 取り込み（2 往復目・実装担当が追記）

| ID | 採否 | 理由・対応 |
|---|---|---|
| 中-4（残り） / 再-低-2 | 採用 | `DATA-LICENSE.md` §4 を「取り込み・生成するもの（S2・S4 で作る）」と未来形に。 |
| 低-1（残り） / 再-低-3 | 一部採用 | 引用符付き `"write-all"` / `'write-all'` を検出（変異 2 種で EXIT=1 確認）。フロー形式の個別 write は止めない方針のまま（1 往復目と同じ理由）。 |
| 再-中-1 | 採用 | 「匿名の」を削り、「IP・UA・日替わりソルトのハッシュを用い、生 IP は保存しない」と HANDOVER §7.3 の事実だけを書いた。 |
| 再-低-1 | 採用 | 「勝つファイル」をグロブ単位で数えるよう変更。指摘の再現（層 C に `data/README.md`）で `'data/README.md' が勝つファイルが 0 件` が出て EXIT=1、既定を末尾へ移す変異も EXIT=1 を確認。 |
| 再-低-4 | 採用 | NOTICE §2 を「`character_data/` とデータ SQL のうち、同プロジェクトの REUSE.toml で CC-BY-SA-4.0 と宣言された範囲」と、限定が両方に掛かる文に直した。 |
| 再-低-5 | 一部採用 | README の表を「アプリ本体（コード・利用者向けの文書）」に。`check-licenses.py` の `combomgr`（EXPECT のパス）と `ci.yml` の `private` は今は必要なので残し、S7 の除去対象として完了報告の次段入力に書く。 |
| 再-低-6 | 採用（報告側） | 計画から変えた点を完了報告に列挙する。置き場所検査の結果を Summary に出す件は、step が落ちれば job が赤になり理由はログに出るため見送り。 |

再レビューは 2 往復で打ち切り（/stage の上限）。残りは完了報告の「残課題」へ。
