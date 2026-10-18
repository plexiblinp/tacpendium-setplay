# 事前計算（配信する静的データ）

S4 で作った。HANDOVER §3 ②。CLI は `cmd/precompute`。

```sh
go run ./cmd/precompute -out build/site-data   # -chars ryu,ken で一部だけ
```

### 計算はブラウザの WASM（S5 の開発者決定: 計算方式 C、2026-10-11）

S5 でブラウザ（Chromium・WASM・Worker）の所要を測ったところ、S4 の繋ぎ表（案 B）ありと表なしで速さは変わらなかった
（KA 100 の最大: 表あり 7.9 秒・表なし 6.9 秒。`docs/progress/S5-report.md`）。そこでサイトは表を配らず、WASM のエンジンだけで数える。
繋ぎ表の仕組み（作成・結合・`LoadTable`・`-tables`・CI の全 KA の突き合わせ）は **S6 で消した**（S5 後の開発者決定 2。D〔重い条件の結果を前もって配る〕を足すとしても、作るのは最終結果でエンジンそのものを使う）。
出力は `manifest.json` と `moves/` だけ。

出力先は「無いか空のディレクトリ」でなければ止まる（前回の出力が混ざらないように）。出力は決定的（時刻を入れない。同じ入力から同じバイト列）。
生成は全 31 キャラで数秒。公開に使う生成物は CI からだけ出す（AGENTS §8-12）。デプロイは S7 以降。

## 出力

| パス | 中身 | キャッシュ |
|---|---|---|
| `manifest.json` | キャラ一覧と各ファイルの場所（下） | **唯一の可変ファイル**（短く・再検証させる） |
| `moves/<code>.<hash>.json` | moves DB（`data/moves/<code>.json` のバイト列そのまま。表記・名前の表示と WASM の `load` に使う） | immutable |

`<hash>` はそのファイルの**中身**の sha256 の先頭 16 桁。中身の変わらないキャラは同じ名前のままなので、ブラウザのキャッシュが効く
（S1 後の開発者決定 3。HANDOVER §3 の `/data/<dataHash>/...` 全体ハッシュは使わない）。

- `moves`（1 キャラ 60〜100 KB）は、そのキャラのデータ（表記・名前を含む）が変わらない限り同じ名前（中身のハッシュ）。各ファイルの `dataVersion` は**キャラごとの版**で、計算に効く列（`calcFingerprint`）が変わったときだけ上がる（表示だけの変更では上がらないが、ファイルの中身は変わるので名前は変わる。`docs/data/moves-db.md`。S4 後の開発者決定 2）。

### `manifest.json`

| キー | 意味 |
|---|---|
| `schemaVersion` | manifest の形の版（1） |
| `dataVersion` | 全体の版: 取込元の公開版 tacpendium の SHA（`data/moves/index.json` の先頭と同じ。画面にデータ版として出す） |
| `maxKnockdownAdvantage` | サイトで受け付ける KA の上限（100 = HANDOVER D10） |
| `characters[]` | `index.json` の順。`code`・`name{ja,en}`・`dataVersion`（キャラごとの版）・`calcFingerprint`・`safeJump`（無ければ null）・`moves`（manifest からの相対パス）・`movesBytes` |

## 検査

| 検査 | どこで |
|---|---|
| CLI → ファイル → engine | `cmd/precompute` の `TestRun_OutputServesSameResults` |
| 表を作らない（`sp/`・表の項目が無い） | `TestRun_NoTables` |
| 決定性・配信上限（20,000 ファイル・1 ファイル 25 MiB） | `TestRun_Deterministic`・`TestCheckLimits`、CLI の `違反: N 件` |
| 本家の正解との一致 | `engine/conformance_test.go`（`docs/engine.md`） |

## CI での差分実行（S4 後の開発者決定 3・4。2026-10-11 の見直しで案 A）

重い検査は、検査済みの組を CI のキャッシュ（`actions/cache`。`.verify/<種類>.txt`）に残し、同じ組は二度と回さない。判定は `scripts/ci/verify-plan.py`（試験は `scripts/ci/verify_plan_test.py`）。

- 鍵 = sha256(検査の種類, キャラ, そのキャラの `calcFingerprint`, その種類の計算の版, そのキャラの追加の版)。`calcFingerprint` はセットプレイの計算に効く列だけの指紋（表記・名前・メモ・ヒット時の有利・硬直は含まない）なので、それ以外の列だけの変更では回らない。
- **コードの変更は鍵に直接入れない。** 計算に効くコード（`scripts/ci/logic-version.json` の `watched`。今は `internal/`・`engine/`・`cmd/`・`go.mod`・`upstream/tacpendium.lock`・`scripts/wasm/`）の中身が記録と違うと、リポジトリ検査 job の `verify-plan.py check` が赤になる。変更した人が影響を判断し、`verify-plan.py ack` で理由つきで記録する（`--none` 何も回さない／`--kinds … --chars …` そのキャラ・検査だけ／`--all` 全部）。記録は `logic-version.json` の `log` に残る。手順は `.claude/commands/sync_upstream.md`。
- ★計算に効くコードを `watched` の外（新しいトップレベルのディレクトリ等）に置いたら、`watched` に足すこと（足さないと変更に気づかない）。今は計算に関わるコードが `watched` の中に全部ある。
- **安全網**: 「影響なし」の判断の誤りに備え、Go job は本家の正解との突き合わせを KA 61 以下だけ全キャラで毎回回す（`SETPLAY_ORACLE_SMOKE_MAX_KA=61`。手元 4 コアで約 14 秒・7,812 回）。

| 種類 | 検査 | job |
|---|---|---|
| `oracle` | 本家の正解（`TestConformance_UpstreamOracle`、`SETPLAY_ORACLE_CHARS` で絞る。安全網は全キャラ・KA 61 以下） | Go |
| `wasm` | WASM の本家の突き合わせと Node の通し（5 キャラ。1 つでも未検査なら全部） | WASM |

- 検査済みの記録は**通ったときだけ**残す（打ち切り・失敗では残さない）。手動実行（workflow_dispatch）は全部回す。
- 手元の `go test ./...` は全部回る（環境変数を付けなければ絞らない）。
- S6 で繋ぎ表の全 KA の突き合わせ（種類 `sweep`）を消した。
- 限界: キャッシュはブランチ単位で、ブランチは main のキャッシュを読めるが、main はブランチのキャッシュを読めない。マージ後の main で、変わったキャラを 1 回回し直す。7 日使われないキャッシュは消え、そのときは全部回し直す。
