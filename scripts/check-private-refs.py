#!/usr/bin/env python3
"""公開してはいけない語（本家＝非公開リポジトリのコミット SHA と、公開されていない文書の名前）が追跡ファイルに無いかを検査する
（標準ライブラリのみ。S7）。

語そのものは本ファイルに書かない（書くと本ファイルが「何を伏せたか」の手がかりになる）。SHA-256 だけを持つ。
- 16 進の連なり（7〜40 文字）は先頭 7 文字のハッシュで照合する（短縮 SHA でも長い SHA でも当たる）。
- それ以外の語（英数字・`_`・`-`・`.` の連なり）は、小文字にし、`_`・`.`・`-` で部分に分け、連続する部分の並びを
  すべて `-` でつないだ形のハッシュで照合する（`<語>.md`・`docs/<語>.md`・`<語>_v2`・`-` を `_` に替えた形も当たる。S7 レビュー中-2）。
本家の名前や、公開版 tacpendium で既に公開されている文書 ID は対象にしない（S7 の開発者決定 2026-10-18）。

語を足すとき: `python3 -c "import hashlib;print(hashlib.sha256('<語>'.encode()).hexdigest())"` の値を DENY に足す
（SHA は先頭 7 文字を小文字で。語は小文字にして区切りを `-` にした形で）。語は報告・コミットメッセージにも書かない。

使い方: python3 scripts/check-private-refs.py
終了コード: 0 = 違反なし / 1 = 違反あり / 2 = 実行エラー
"""

import hashlib
import pathlib
import re
import subprocess
import sys

ROOT = pathlib.Path(__file__).resolve().parent.parent

# 本家のコミット SHA（先頭 7 文字）3 件と、公開されていない文書の名前 1 件。どれも今の木には無い。SHA は、HANDOVER から
# 伏せたもの・消した `_reference/` の元のもの・S7 で読んだ本家の先端。報告・コミットメッセージに書かれないよう見張る。
DENY = frozenset(
    {
        "465ed12a6529c5c7b9ab892554b9f66f100bc93845c6413f98a204ce4597aa4b",
        "e0908ce95d5336ecf5bb2f5fedc1e8579b6ca22cc5e0035e03e22a4ff35692a7",
        "39151fd4ccb88bb3794754ad7ada333b129e859fbb75de0d31413364ce490ef2",
        "d0273c657c2419460950b58d3d354f6922c60f64be19219a272385127169f931",
    }
)

HEX_PREFIX_LEN = 7
HEX_RE = re.compile(r"(?<![0-9a-f])[0-9a-f]{7,40}(?![0-9a-f])")
WORD_RE = re.compile(r"[A-Za-z0-9_.-]+")
PART_SEPARATOR = re.compile(r"[_.-]+")


def _digest(text: str) -> str:
    return hashlib.sha256(text.encode()).hexdigest()


def _word_forms(word: str) -> set[str]:
    """語の部分(区切り `_` `.` `-` で分けたもの)の連続する並びを、`-` でつないだ形の集合。"""
    parts = [p for p in PART_SEPARATOR.split(word) if p]
    return {"-".join(parts[i:j]) for i in range(len(parts)) for j in range(i + 1, len(parts) + 1)}


def find(text: str, deny: frozenset[str] = DENY) -> list[int]:
    """deny に当たる語がある行番号（1 始まり）を返す。"""
    hits = []
    for lineno, line in enumerate(text.splitlines(), 1):
        lower = line.lower()
        hexes = (m.group(0)[:HEX_PREFIX_LEN] for m in HEX_RE.finditer(lower))
        forms = (f for m in WORD_RE.finditer(lower) for f in _word_forms(m.group(0)))
        if any(_digest(t) in deny for t in hexes) or any(_digest(f) in deny for f in forms):
            hits.append(lineno)
    return hits


def main() -> int:
    try:
        out = subprocess.run(
            ["git", "ls-files", "-z"], cwd=ROOT, check=True, capture_output=True
        ).stdout
    except (OSError, subprocess.CalledProcessError) as err:
        print(f"ERROR: git ls-files に失敗しました: {err}", file=sys.stderr)
        return 2
    files = [p for p in out.decode().split("\0") if p]
    violations = 0
    scanned = 0
    for rel in files:
        path = ROOT / rel
        try:
            data = path.read_bytes()
        except OSError:
            continue
        if b"\0" in data:
            continue
        scanned += 1
        for lineno in find(data.decode("utf-8", errors="replace")):
            print(f"{rel}:{lineno}: 公開してはいけない語")
            violations += 1
    print(f"走査: {scanned} ファイル")
    print(f"違反: {violations} 件")
    return 1 if violations else 0


if __name__ == "__main__":
    sys.exit(main())
