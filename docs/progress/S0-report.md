# S0 リポジトリ整備 — 完了報告

- 段階開始コミット: `2eaca647e64c0481194f5c7a085db93a8675b801`
- ブランチ: `claude/stage-0-zn02w6`

## 計画

### Context
HANDOVER §9 の S0「リポジトリ整備: AGENTS.md/CLAUDE.md、ライセンス一式、CI 骨格、.gitignore」。完了条件は **CI が空で緑**。
開始コミット: `2eaca647e64c0481194f5c7a085db93a8675b801`（ブランチ `claude/stage-0-zn02w6`）。

現状の確認結果:
- `AGENTS.md` / `CLAUDE.md` / `.claude/`（settings + hook 3 本、settings が参照する hook は全部実在）は既存コミットで済。§11・§12-2 も AGENTS.md で確定済み → S0 では変更しない（報告に「済」と根拠だけ書く）。
- ライセンス一式（`NOTICE` / `DATA-LICENSE.md` / `REUSE.toml` / `LICENSES/LicenseRef-...`）は**本家のコピーのまま**。本リポジトリに無いもの（`character_data/`・`migrations/`・shadcn・`web/src/components/ui`・`SUPPORT.md`/`SECURITY.md`・`docs/progress/M26-01-report.md`・`scripts/check-*.sh`）を参照し、§13 で禁止の `autopilot-combomgr` 言及もある → 単体版向けに書き直す。
- CI は `upstream-watch.yml`（手動のみ）だけ。通常の CI が無い。
- `.gitignore` は 6 行のみ。README は暫定。
- 環境: go1.24.7 / node 22 / pnpm / python3.13 あり。github.com への ls-remote 可（Action の SHA を実測できる）。

### 作る・変えるファイル
1. `docs/progress/S0-report.md` — 冒頭に「計画」節（完了条件・ファイル・検査・未決と扱い・開始コミット）。最初にコミット・push。
2. **ライセンス一式の単体版化**
   - `REUSE.toml`: 三層を維持（A=AGPL 既定 / C=MIT: `AGENTS.md` `CLAUDE.md` `.claude/**` `docs/handover/**` `docs/progress/**` `upstream/**` `scripts/**` / B=CC-BY-SA: S2 で作るデータ置き場 `data/**`）。`_reference/**` は `LicenseRef-Tacpendium-NotForDistribution`（配布しない・S7 で削除）。本家の経緯コメント（D-xxx, M40 等）は削る。
     - 推測: データ置き場のパス名は `data/`（S2 で変えるなら REUSE.toml も同時に直す）。
     - 推測: 層 C を MIT に保つのは本家方針の踏襲（HANDOVER §10「本家からコピーし単体版向けに修正」の範囲内と判断。§8 の AGPL/CC-BY-SA は変えない）。
   - `NOTICE`: 非公式／ゲームアセット不含／データは公開版 tacpendium（`plexiblinp/tacpendium`）由来の手入力事実情報・CC-BY-SA・クレジット／撤退ポリシー（窓口メールは本家と同じものを引き継ぐ）／ソースの所在。第三者ライブラリ節は「まだ依存なし。S3/S5 で追記」。shadcn・autopilot・M26 参照は削除。
   - `DATA-LICENSE.md`: 層 B＝取込データ（moves DB JSON）と事前計算結果（派生データ）。利用者生成データ節は「本サービスは利用者データを保存しない」に置換。正本は公開版 tacpendium の `character_data/`。
   - `LICENSES/LicenseRef-Tacpendium-NotForDistribution.txt`: 対象を `_reference/**` に書き換え。
   - `LICENSE`・`LICENSES/{AGPL,CC-BY-SA,MIT}.txt` は本文なので触らない。
3. **検査スクリプト**（層 C、stdlib のみ）
   - `scripts/check-licenses.py`: (a) REUSE.toml の各 `SPDX-License-Identifier` に `LICENSES/<id>.txt` があり、逆に未使用の LICENSES ファイルが無い (b) 各 annotation の path グロブが追跡ファイルに 1 件以上当たる（本家 D-777 の教訓。S2 まで空でよいグロブは `PENDING = {"data/**": "S2"}` で明示し、当たり始めたら PENDING からの削除を促す） (c) 追跡ファイルの各々が最後に当たる表で解決できる。REUSE 3.3 のグロブ（`*` は `/` を跨がない、`**` は跨ぐ）を実装。
   - `scripts/check-actions-pinned.sh`: `.github/workflows/*.yml` の `uses:` が 40 桁 SHA 固定であることと、各 workflow に `permissions:` があることを検査。
   - 両方とも §8-3 に従い、わざと壊して赤を確認（例: LICENSES の 1 本を一時退避／`uses: actions/checkout@v7` を一時記述／annotation に当たらないグロブを追加）し、1 件の違反で 1 件と出ることを確かめる。
4. **CI 骨格** `.github/workflows/ci.yml`
   - `on: push`（全ブランチ。`claude/...` の `/` 入りでも動く）＋ `pull_request` ＋ `workflow_dispatch`。`paths` フィルタなし。`permissions: contents: read`、`concurrency: ci-${{ github.ref }}` cancel-in-progress。
   - job `checks`: checkout（`3d3c42e5…` v7.0.1、`persist-credentials: false`）→ `check-licenses.py` → `check-actions-pinned.sh` → hook の `bash -n`。
   - job `go`: setup-go（`b7ad1dad…` v7.0.0、`cache: false`、版は go.mod 不在時 1.24.x）を常に実行（SHA を今から検証するため）。`go.mod` が無ければ summary に「go.mod なし: スキップ」と出して終了。あれば `test -z "$(gofmt -l .)"`、`go vet ./...`、`set -o pipefail; go test -count=1 ./... | tee`、件数を summary へ（読めなければ「読めなかった」）。
   - job `web`: setup-node（`82076278…` v7.0.0）を常に実行。`package.json` が無ければスキップを summary。あれば pnpm/action-setup（`ea17c68d…` v6.1.0 の commit）→ `pnpm install --frozen-lockfile` → `tsc --noEmit` → `vitest --run`（ANSI を落として件数抽出）。
     - 推測: 段 S3/S5 でディレクトリ構成が決まったら `working-directory` を合わせる。今は「ルートかサブディレクトリの最初の 1 つ」ではなく、ルート直下の `go.mod` / `web/package.json` を見る（HANDOVER §10 の構成と本家の `web/` に合わせる）。
   - SHA はすべて `git ls-remote --tags` で実測した値（タグ注釈付きは `^{}` の commit）。
5. `.github/dependabot.yml`: `github-actions` を週 1・グループ化（§7.4 の Dependabot）。gomod/npm は S3/S5 で足す（報告の次段入力に書く）。
6. `.gitignore`: Go（`/bin/` `*.test` `coverage.out`）、フロント（`coverage/` `playwright-report/` `test-results/` `*.tsbuildinfo` `.vite/`）、`*.log`、エディタ（`.idea/` `.vscode/`）、生成物（事前計算出力は S4 で決めるので今は `dist/` のまま）。
7. `README.md`: 概要（非公式・KA→レシピ）・状態（準備中）・ライセンス三層の表・データの出所・リポジトリ構成・開発の入口（AGENTS.md）。サイト名は §12-1 未決のためリポジトリ名を仮名として使い、「SF6」をサイト名には使わない。

### 未決事項と扱い
- サイト名・商標（§12-1）: 聞かない（S0 では決めずに仮名で進める。S9 までに必要）。
- `_reference/**` を NotForDistribution と宣言: 配布しない範囲の状態表示であり権利付与ではないので自分で決める。報告に理由。
- 層 C=MIT の継続: 本家方針の踏襲として自分で決める。開発者に異論があれば差し戻し可能と報告に書く（まだ配布していないので取り消しは可能）。
- Dependabot の PR が main 宛てに来るようになる: 報告に明記。
- 止めて聞くもの（AGENTS §3）に当たる事項は現時点で無い見込み。

### 検証
- ローカル: `python3 scripts/check-licenses.py > /tmp/s0-lic.txt 2>&1; echo EXIT=$?`、`bash scripts/check-actions-pinned.sh > /tmp/s0-pin.txt 2>&1; echo EXIT=$?`、各々わざと壊して EXIT≠0 と違反件数 1 を確認、`bash -n .claude/hooks/*.sh`。
- 旧記述の残数: `grep -rn -e shadcn -e autopilot -e M26 -e SUPPORT.md -e SECURITY.md -e 'character_data/' NOTICE DATA-LICENSE.md REUSE.toml LICENSES/LicenseRef* README.md`（意図した残りのみ）。
- CI: push 後に GitHub MCP（`actions_list` / `get_job_logs`）で `ci.yml` の run が全 job success であることを確認し、run ID と結論を報告へ。赤なら直して再 push。
- レビュー: general-purpose サブエージェントを新規起動（fork しない）→ `.claude/commands/review.md` の手順で `docs/progress/S0-review.md`、取り込み節を追記、最大 2 往復。
- 完了報告: 4 項目＋`EXIT=` 逐語＋`git diff --stat 2eaca64`。書いた後に検査を再実行して一致確認。push 後 `git rev-parse HEAD` と `git ls-remote origin claude/stage-0-zn02w6` の一致を確認して停止（S1 へ進まない）。

---

## やったこと

完了条件（HANDOVER §9 S0）: **CI が空で緑** → 満たした（下の CI 結果）。

| 項目 | 成果物 | 状態 |
|---|---|---|
| AGENTS.md / CLAUDE.md（§11） | 既存（`4aa0583`〜`2eaca64`）。settings.json が参照する hook 3 本は実在し、CI で `bash -n` を通す | S0 開始前に済。変更なし |
| ライセンス一式 | `REUSE.toml`・`NOTICE`・`DATA-LICENSE.md`・`LICENSES/LicenseRef-Tacpendium-NotForDistribution.txt` を単体版向けに書き直し。`LICENSE` と `LICENSES/{AGPL,CC-BY-SA,MIT}.txt` は本文なので不変 | 済 |
| 検査スクリプト | `scripts/check-licenses.py`（識別子↔LICENSES の 1:1・グロブの空振り・グロブ単位の覆い隠し・未解決ファイル・代表パスの期待ライセンス）、`scripts/check-actions-pinned.sh`（SHA 固定・トップレベル permissions・write-all 禁止） | 済 |
| CI 骨格 | `.github/workflows/ci.yml`: job「リポジトリ検査」（上 2 本・go.mod/package.json の置き場所・シェル構文）、job「Go」「Web」（go.mod / web/package.json が無ければ summary に「スキップ」を出して緑） | 済 |
| Dependabot | `.github/dependabot.yml`（github-actions を週 1・グループ化・`chore(deps)`） | 済 |
| `.gitignore` | Go / フロント / テスト出力 / 秘密情報 / エディタ / `__pycache__` | 済 |
| README | 概要・データの出所・ライセンス表・開発者向けの入口 | 済 |

### 検査結果（最終・報告を書いた後に再実行したもの）

```
$ python3 scripts/check-licenses.py > /tmp/s0-lic.txt 2>&1; echo LIC_EXIT=$?
LIC_EXIT=0
追跡ファイル: 167 件（LICENSES/・LICENSE・REUSE.toml を除く）
  AGPL-3.0-or-later: 4 件
  LicenseRef-Tacpendium-NotForDistribution: 141 件
  MIT: 22 件
NOTE: 表 3: 'data/**' は 0 件（PENDING: S2 で作る）
違反: 0 件
$ bash scripts/check-actions-pinned.sh > /tmp/s0-pin.txt 2>&1; echo PIN_EXIT=$?
PIN_EXIT=0
workflow: 2 本 / uses 参照: 7 件
違反: 0 件
$ for f in .claude/hooks/*.sh scripts/*.sh upstream/*.sh; do bash -n "$f" || exit 9; done; echo SYNTAX_EXIT=$?
SYNTAX_EXIT=0
```
（本報告のコミットで追跡ファイルは増えない＝S0-report.md は既存のため、件数は報告後も同じはず。最後に再実行して確かめる。）

### わざと壊して赤を確認（AGENTS §8-3）

`check-licenses.py`（リポジトリの写し /tmp/s0-break/repo または作業ツリーで一時的に壊し、直後に戻した）:

| 壊し方 | EXIT | 違反 |
|---|---|---|
| `LICENSES/MIT.txt` を退避 | 1 | 1（`LICENSES/MIT.txt がありません`） |
| 未使用の `LICENSES/BSD-3-Clause.txt` を追加 | 1 | 1 |
| `"upstream/**"` → `"upstreams/**"`（グロブの空振り） | 1 | 1 |
| PENDING の `data/**` にファイルを追加 | 1 | 1（PENDING から消すよう促す） |
| 既定の表 `**` を削除 | 1 | 8（未解決 7 件＋AGPL 本文が未使用） |
| 既定の表を末尾へ移動（レビュー中-1） | 1 | 17 |
| `.claude/**` → `.claude/*`（レビュー中-1） | 1 | 1（EXPECT） |
| 層 C に `data/README.md` を足し `data/**` に覆い隠させる（再-低-1） | 1 | 2（うち 1 件が `'data/README.md' が勝つファイルが 0 件`） |
| グロブ単体: `a/*` は `a/b` に当たり `a/b/c` に当たらない、`a/**` は `a/b/c` に当たる、`*.md` は `d/x.md` に当たらない | — | 期待どおり（True False True False） |

`check-actions-pinned.sh`:

| 壊し方 | EXIT | 違反 |
|---|---|---|
| `actions/checkout@v7`（タグ参照） | 1 | 1 |
| トップレベルの `permissions:` を消す | 1 | 1 |
| `permissions: write-all` / `"write-all"` / `'write-all'` | 1 | 1 |

CI の置き場所検査: `engine/go.mod` を追跡させると FAIL、ルートの `go.mod` は OK（同じコマンドを手元で実行）。

### CI（GitHub Actions、`ci.yml`）

| run | コミット | 結果 |
|---|---|---|
| 37410593105 | `a09689c`（実装） | 3 job とも success。ログに `違反: 0 件` を 2 本とも確認。Go / Web は検出 step 以降 skipped |
| 37411112725 | `dad3680`（1 往復目の取り込み） | 3 job とも success（再レビューが確認） |
| 37411499686 | `82f2c07`（2 往復目の取り込み） | 下の「最終確認」に記す |

### `git diff --stat 2eaca647e64c0481194f5c7a085db93a8675b801`（本報告の完了節を書く前）

```
 .github/dependabot.yml                             |  12 +
 .github/workflows/ci.yml                           | 147 ++++++++++
 .gitignore                                         |  21 ++
 DATA-LICENSE.md                                    |  79 ++---
 .../LicenseRef-Tacpendium-NotForDistribution.txt   |  67 ++---
 NOTICE                                             | 158 +++-------
 README.md                                          |  40 ++-
 REUSE.toml                                         | 249 +++-------------
 docs/progress/S0-report.md                         |  56 ++++
 docs/progress/S0-review.md                         | 326 +++++++++++++++++++++
 scripts/check-actions-pinned.sh                    |  41 +++
 scripts/check-licenses.py                          | 183 ++++++++++++
 12 files changed, 950 insertions(+), 429 deletions(-)
```
`_reference/` の差分: 0 行。テストは S0 開始時点で 0 本で、減ったものは無い（新規の検査 2 本を追加）。

旧記述の残数: `grep -rn -e shadcn -e autopilot -e M26 -e M40 -e 'D-7' -e SUPPORT.md -e SECURITY.md -e seed-data -e public-snapshot -e combomgr NOTICE DATA-LICENSE.md REUSE.toml LICENSES/LicenseRef-Tacpendium-NotForDistribution.txt README.md .github scripts` → 実装時点で 0 件（grep EXIT=1）。その後の取り込みで `scripts/check-licenses.py` の EXPECT に `_reference/combomgr/SOURCE_COMMIT` が入った（意図的。次段入力参照）。

## 判断と理由

- **三層を維持し、層 C（MIT）に `.github/**` も含めた。** 本家方針の踏襲（HANDOVER §10「本家からコピーし単体版向けに修正」）。§8 の AGPL / CC-BY-SA は変えていない。まだ配布していないので、開発者が異論なら取り消せる。
- **`_reference/**` は `LicenseRef-Tacpendium-NotForDistribution`。** 配らない範囲の状態表示であって権利付与ではないため自分で決めた。案「層 A のまま」は配る予定の無い私的資料に AGPL の許諾を付けることになるので不採用。
- **層 B の置き場所は `data/**`（推測）。** S2 で変えるなら REUSE.toml と `check-licenses.py` の PENDING を同時に直す。PENDING にファイルが入ると検査が赤になって削除を促す。
- **公開版 tacpendium のデータが CC-BY-SA であることは実物で確認した。** 公開版の `REUSE.toml` で `character_data/**`・`migrations/*_data_*.sql` が CC-BY-SA-4.0、帰属が `2026 plexiblinp and the Tacpendium contributors`（`git clone --filter=blob:none` で取得）。帰属表示はこの値にそろえた。
- **クレジットの表示例は CC BY-SA 4.0 §3(a)(1)（帰属・ライセンス URI・改変の表示）に合わせた（レビュー中-3）。** ライセンスの選択ではなく、既定ライセンスの要件への適合なので開発者には聞かなかった。
- **CI の Go / Web は setup-go / setup-node を常に実行する。** 中身が無い今のうちに Action の SHA が実在することを CI で確かめるため。pnpm/action-setup は `web/package.json` ができるまで走らない（SHA は `git ls-remote` で実測のみ）。
- **CI の検出はルートの `go.mod` と `web/package.json` に固定し、それ以外の場所に置かれたら検査 job を落とす（レビュー中-2）。** 「最初に見つかったもの」を自動で拾う案は、複数できたときに黙って片方を見落とすので不採用。
- **`pull_request` トリガーを外した（レビュー低-5）。** push が全ブランチを覆い、同一コミットへの二重実行で private 期間の分数を食うため。フォーク PR を受ける public 化後に戻す。
- **Python（標準ライブラリのみ）で検査を書いた。** `tomllib` で REUSE.toml を正しく読めるため。`reuse` ツール（GPL）は導入の可否を聞く必要があるので使わなかった。Go のモジュールは S3 でモジュールパスを決める段なので S0 では作らない。

### 計画から変えた点（再-低-6）
- 計画の「DATA-LICENSE: 利用者生成データ節を『保存しない』に置換」→ 計測（§7.1・§7.3）と矛盾するため、「入力に権利を主張しない」＋計測は S6 のプライバシーポリシーへ、に変えた（レビュー高-1・再-中-1）。
- 計画の `on: pull_request` → 外した（上記）。
- 計画の層 C に `.github/**` を追加（レビュー低-4）。
- 計画の `check-licenses.py` (a)〜(c) に、グロブ単位の覆い隠し検出と EXPECT を追加（レビュー中-1・再-低-1）。
- 計画に無かった CI の「置き場所」検査と `write-all` 禁止を追加（レビュー中-2・低-1）。

## 残課題・既知の問題

- `check-actions-pinned.sh` は、job 単位の `permissions` に書いた個別の `: write`（フロー形式を含む）を止めない。必要な job が出うるため、レビューで見る（レビュー低-1・再-低-3 の一部不採用）。
- 引用符で囲んだキー `"uses":` は検出しない（本リポジトリでは書かない）。
- CI の Summary には置き場所検査の結果を出していない（落ちれば job が赤になり、理由はログに出る）。
- `reuse lint`（公式ツール）とは突き合わせていない。グロブ解釈は REUSE 3.3 の記述に沿って自前実装した。
- Dependabot は main へのマージ後に有効になり、Action 更新の PR を週 1 で作る（公開から 7 日の cooldown 付き）。開発者の判断で残し、「CI が緑なら受け入れ、メジャー版だけ確認する」方針を AGENTS.md §5 に書いた（報告後の追加変更）。`pnpm/action-setup` は S5 まで CI で動かないため、その更新は緑でも検証になっていない。
- サイト名・商標（HANDOVER §12-1）は未決のまま。NOTICE・README はリポジトリ名を仮名として使い、サイト名に「SF6」を使っていない。

## 次の段階への入力

- **S1**: 本段で作ったのは整備だけで、実測は何もしていない。S1 は AGENTS.md §2 の表どおりに読む。**HANDOVER §9 の S1 行に ⑦⑧ を追加した（2026-10-06、新機能 D17 グッド/バッドに伴う D1 無料枠と打ち切りへの影響）**。D16（コピー）・D17 は §1 に追記済み。
- **S2**: 層 B の置き場所を `data/` にするなら、ファイルを置いた時点で `check-licenses.py` が赤になる（`PENDING` から `data/**` を消すこと）。別の場所にするなら REUSE.toml の (3) と PENDING を同時に直す。英語技名の上書き表（D7）も層 B に入れる。
- **S3**: `go.mod` はルートに置く前提で CI を書いた（別の場所なら検査 job が落ちる）。置いたら `ci.yml` の setup-go を `go-version-file: go.mod`・`cache: true` に、Dependabot に `gomod` を足す。
- **S5**: フロントは `web/package.json`（`packageManager` に pnpm の版を書く。pnpm/action-setup が読む）。置いたら setup-node に `cache: pnpm` を、Dependabot に `npm` を足す。依存を入れたら NOTICE §5 に直接依存とライセンスを書く。
- **S7**: `_reference/` を消したら、REUSE.toml の (4)、`LICENSES/LicenseRef-Tacpendium-NotForDistribution.txt`、`check-licenses.py` の EXPECT の `_reference/combomgr/SOURCE_COMMIT` を消す（消さないと検査が赤になる＝消し忘れが残らない）。§13 の grep では `ci.yml` 冒頭コメントの `private` も当たる（Actions の分数の注記。public 化で不要になる）。NOTICE §4 にリポジトリの公開 URL を書く。public 化後に `pull_request` トリガーを戻す。

## 最終確認

- 報告を書いた後の再実行: `LIC_EXIT=0` / `PIN_EXIT=0` / `SYNTAX_EXIT=0`。`check-licenses.py` と `check-actions-pinned.sh` の出力は、上の「検査結果」と `diff` で一致した（差分なし）。
- CI run 37411499686（`82f2c07`）: success。run 37411573221（`22c513c`、完了報告のコミット）: success（3 job とも success。Go / Web は検出 step 以降 skipped）。
- 本節を足したコミットは docs だけの変更。CI はこのコミットでも走るが、本報告には結果を書かない（書くと、書いたコミットでまた CI が走る）。
