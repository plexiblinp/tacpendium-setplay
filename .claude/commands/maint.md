---
description: 公開後の作業を 1 つ進める（security / ux / data / health。読む → 直す → 検査 → 報告）
argument-hint: <security|ux|data|health> <内容（例: "Node の修正版" "結果の表の並びが崩れる Issue #12"）>
---

公開後の作業 **$ARGUMENTS** を進める。規約は `AGENTS.md`（とくに §3「公開後の進め方」・§8）、手順は `docs/maintenance.md`。

- 引数の先頭の語が種類（`security`・`ux`・`data`・`health`）。無い・ほかの語なら、内容から決めて報告に書く。
- 承認は待たない。**AGENTS §3「止めて開発者に聞くもの」と、利用者に見える仕様の変更のときだけ** AskUserQuestion で止まる（選択肢と推奨を添える）。

## 1. 読む
- HANDOVER §0・§1、`docs/maintenance.md` の種類の節、`docs/progress/` の新しい報告を 1〜2 本。
- 触る場所の資料は AGENTS §2 の表を目安に読む（エンジン → S3 の行、画面 → S5 の行、データ → S2 の行）。
- 開始コミットを記録する: `git rev-parse HEAD`。
- `data` は `/sync_upstream` の手順に従う（報告もそちらの形）。
- `health` のリファクタリングは、`docs/maintenance.md`「health」の守り（挙動を変えない・上流由来は直さない）に従う。観測だけなら `/audit_scale`。

## 2. 直す
- 不具合なら、直す前に赤くなるテストを足して赤を確かめる（AGENTS §8-3）。
- 節目ごとにコミット・push（Conventional Commits。スコープは種類。例 `fix(ux): ...`・`chore(security): ...`）。
- 利用者に見える変更なら、更新履歴に 1 行足す（`docs/maintenance.md`「更新履歴」）。

## 3. 検査
- 変えた側のテスト・検査を、出力をファイルへ書いて `EXIT=` と件数で判定する（AGENTS §8-1）。Go を変えたら `go test -count=1 ./...`、Web を変えたら `tsc`・`vitest --run`・`pnpm build`・E2E。
- 文言を変えたら全ロケールと docs を grep する（AGENTS §8-8）。

## 4. 報告
- `docs/progress/<YYYY-MM-DD>-<種類>-<短い名前>-report.md` に、やったこと・判断と理由・検査の逐語・`git diff --stat <開始コミット>`・残課題を書く。
- 報告を書いた後に検査をもう一度回し、報告と一致することを確かめてからコミット・push する。
- 開発者への返事に、PR が要るか（頼まれたら作る）・デプロイが要るか（配信物が変わるか）・ローカル版のタグが要るかを書く。
