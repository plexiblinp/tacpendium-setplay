---
description: 規模・複雑度の定点観測（health）。scripts/observe-scale.sh を回し、系列表 docs/audits/scale-baseline.md に 1 列足して報告を書く（調査だけ。コードは直さない）
argument-hint: <前回の commit。省略時は系列表の最新列>
---

規模・複雑度の定点観測を行う。手順と判定は `docs/maintenance.md`「health」が正本（本書と食い違えばそちらを取る）。規約は `AGENTS.md`（§8 検証と報告）。

**調査だけ**。アプリのコード・設定・依存は変えない。書いてよいのは、系列表 `docs/audits/scale-baseline.md` への 1 列と、報告 `docs/progress/<YYYY-MM-DD>-health-scale-report.md` だけ。リファクタリングは報告で候補を挙げ、開発者の指示で `/maint health <内容>` として別に行う。

1. 読む: `docs/maintenance.md`「health」、系列表、直近の `health` の報告。`$ARGUMENTS` が空なら系列表の最新列の commit を前回とする。
2. 道具: `gocyclo`・`scc` が `$(go env GOPATH)/bin` に無ければ入れる（版は `docs/maintenance.md`「health」）。
3. 測る（出力は scratchpad へ）:
   - `OBSERVE_OUT=<scratchpad>/obs-prev bash scripts/observe-scale.sh <前回>`
   - `OBSERVE_OUT=<scratchpad>/obs-now bash scripts/observe-scale.sh HEAD`（作業ツリーに未コミットの変更があれば先に片付ける。`dirty=1` の列は系列に置かない）
   - 公開版 tacpendium を取得しておく（`bash scripts/fetch-upstream.sh`）。上位の表の「上流由来」の列はそれを見る。
4. 突き合わせる（動いた行だけを読む）:
   - `== COUNTS ==` を系列表の前の列と比べる。`-1` は未測定として理由を書く。
   - 関数ごとの増減は、2 回の `cyclo-sorted.txt` を「ファイルの位置 + 関数名」で突き合わせる（`main.run` のように関数名だけでは重なる）。
   - ファイルごとの増減は、2 回の `scc.csv` の Complexity（8 列目）を突き合わせる（TS は関数ごとの値が無い）。
   - 増えたもの・閾値を超えたものを、**単体版だけのコード**と**上流由来**（同じパスが公開版 tacpendium にある）に分ける。上流由来は直さない（直すなら本家で直し、取り込みで受ける）。
5. 書く: 系列表に 1 列（所見は書かない）。報告に、表の差・リファクタリングの候補（実査した位置と理由。数字だけを根拠にしない）・測れなかったもの。
6. 検査とコミット: 報告を書いた後にもう一度 `observe-scale.sh HEAD` を回して COUNTS が報告と一致することを確かめ、`docs(health): ...` でコミット・push。
