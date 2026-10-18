# upstream — 公開版 tacpendium への追従

- `tacpendium.lock`: 追従の基準とする公開版 tacpendium（`plexiblinp/tacpendium`）のコミット SHA。**最後に取り込みを済ませた版**を指す。
- `watch.sh`: 基準から公開版の見張るパス（セットプレイのロジック・モデル・UI・文言・データ・migrations）が変わったかを調べる。変化があれば exit 1。
  - ローカル: `bash upstream/watch.sh`（比較先を変えるなら `bash upstream/watch.sh <ref>`）
  - CI: `.github/workflows/upstream-watch.yml`（日 1 回の定期実行と手動起動）
- 変化があったら Claude Code で `/sync_upstream <新しい SHA>`。取り込みが済んだら `tacpendium.lock` を更新する。
- 公開版はリリース時だけ更新される。
