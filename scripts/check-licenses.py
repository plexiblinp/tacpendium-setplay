#!/usr/bin/env python3
"""REUSE.toml と LICENSES/ の整合を検査する（標準ライブラリのみ）。

検査:
  1. REUSE.toml が使う SPDX-License-Identifier ごとに LICENSES/<id>.txt がある
  2. LICENSES/ に、どこからも使われていないライセンス本文が無い
  3. 各 [[annotations]] の path グロブが追跡ファイルに 1 件以上当たる
     （グロブはファイルを動かすと黙って外れ、外れた先が既定なのでほかの検査は緑のままになる）。
     まだファイルの無い予定パスは PENDING に段階付きで載せる。当たり始めたら PENDING から消すこと。
  4. 追跡ファイルがすべて、いずれかの表で解決される
  5. 各表の各グロブについて、その表が「勝つ」（最後に当たる）ファイルに 1 件以上当たる。0 件なら、後ろの表に覆い隠されているか
     並びが崩れている（例: 既定の `**` を末尾へ動かすと、ほかの表がすべて 0 件になる）
  6. EXPECT に挙げた代表パスが、期待どおりのライセンスに解決される
     （例: `.claude/**` を `.claude/*` にすると、hooks/ 配下が黙って既定へ落ちる）

グロブの規則は REUSE 仕様 3.3 に従う: `*` は `/` を跨がない、`**` は跨ぐ、`\\` で次の文字をそのまま扱う。
同じファイルに複数の表が当たるときは最後の表が使われる。

使い方: python3 scripts/check-licenses.py
終了コード: 0 = 違反なし / 1 = 違反あり / 2 = 実行エラー
"""

import pathlib
import re
import subprocess
import sys
import tomllib

ROOT = pathlib.Path(__file__).resolve().parent.parent

# まだ追跡ファイルが無くてよいグロブ → それを作る段階。
PENDING: dict[str, str] = {}

# 代表パス → 期待するライセンス。各層・各表から、ディレクトリの深いところを 1 本ずつ選ぶ。
# パスを動かしたら本表も直す（存在しないパスは違反として出る）。
EXPECT = {
    "README.md": "AGPL-3.0-or-later",
    "NOTICE": "AGPL-3.0-or-later",
    "AGENTS.md": "MIT",
    ".claude/hooks/session-start.sh": "MIT",
    ".github/workflows/ci.yml": "MIT",
    "scripts/check-licenses.py": "MIT",
    "upstream/watch.sh": "MIT",
    "docs/handover/HANDOVER-001-setplay-standalone.md": "MIT",
    "docs/progress/S0-report.md": "MIT",
    "docs/upstream-diff.md": "MIT",
    "internal/movesdb/build.go": "AGPL-3.0-or-later",
    "web/src/components/App.tsx": "AGPL-3.0-or-later",
    "data/moves/ryu.json": "CC-BY-SA-4.0",
    "engine/testdata/upstream_oracle.json": "CC-BY-SA-4.0",
    "engine/engine.go": "AGPL-3.0-or-later",
    "scripts/upstream-oracle/standalone_oracle_test.go.txt": "MIT",
    "data/overrides/name_en.json": "CC-BY-SA-4.0",
}

# REUSE 仕様 3.3 §Covered Files で対象外になるもの。
NOT_COVERED = re.compile(r"^(LICENSES/|LICENSE$|REUSE\.toml$|\.git/)")


def glob_to_regex(pattern: str) -> re.Pattern[str]:
    out = []
    i = 0
    while i < len(pattern):
        c = pattern[i]
        if c == "\\" and i + 1 < len(pattern):
            out.append(re.escape(pattern[i + 1]))
            i += 2
        elif pattern.startswith("**", i):
            out.append(".*")
            i += 2
        elif c == "*":
            out.append("[^/]*")
            i += 1
        else:
            out.append(re.escape(c))
            i += 1
    return re.compile("".join(out) + r"\Z")


def tracked_files() -> list[str]:
    res = subprocess.run(
        ["git", "-c", "core.quotePath=false", "ls-files", "-z"],
        cwd=ROOT, capture_output=True, check=True,
    )
    files = [f for f in res.stdout.decode("utf-8").split("\0") if f]
    return [f for f in files if not NOT_COVERED.match(f)]


def as_list(v: object) -> list[str]:
    return list(v) if isinstance(v, list) else [v]  # type: ignore[list-item]


def main() -> int:
    try:
        conf = tomllib.loads((ROOT / "REUSE.toml").read_text(encoding="utf-8"))
        files = tracked_files()
    except (OSError, tomllib.TOMLDecodeError, subprocess.CalledProcessError) as e:
        print(f"ERROR: 読み込みに失敗しました: {e}", file=sys.stderr)
        return 2

    errors: list[str] = []
    notes: list[str] = []
    annotations = conf.get("annotations", [])
    if not annotations:
        errors.append("REUSE.toml に [[annotations]] がありません")

    used_ids: set[str] = set()
    tables: list[tuple[list[tuple[str, re.Pattern[str]]], str]] = []
    for n, ann in enumerate(annotations, start=1):
        lic = ann.get("SPDX-License-Identifier")
        if not lic:
            errors.append(f"表 {n}: SPDX-License-Identifier がありません")
            tables.append(([], ""))  # 表の番号と tables の添字をそろえておく
            continue
        if not ann.get("SPDX-FileCopyrightText"):
            errors.append(f"表 {n}: SPDX-FileCopyrightText がありません")
        if ann.get("precedence") != "override":
            errors.append(f"表 {n}: precedence が override ではありません")
        used_ids.update(t for t in re.split(r"\s+(?:AND|OR|WITH)\s+|[()\s]", lic) if t)
        regs = []
        for pat in as_list(ann.get("path", [])):
            reg = glob_to_regex(pat)
            regs.append((pat, reg))
            hits = sum(1 for f in files if reg.match(f))
            if pat in PENDING:
                if hits:
                    errors.append(f"表 {n}: '{pat}' は PENDING ({PENDING[pat]}) だが {hits} 件当たる。PENDING から消すこと")
                else:
                    notes.append(f"表 {n}: '{pat}' は 0 件（PENDING: {PENDING[pat]} で作る）")
            elif hits == 0:
                errors.append(f"表 {n}: '{pat}' が追跡ファイルに 1 件も当たりません")
        tables.append((regs, lic))

    for pat in PENDING:
        if not any(pat in as_list(a.get("path", [])) for a in annotations):
            errors.append(f"PENDING の '{pat}' が REUSE.toml のどの表にもありません")

    lic_dir = ROOT / "LICENSES"
    present = {p.stem for p in lic_dir.glob("*.txt")} if lic_dir.is_dir() else set()
    for lid in sorted(used_ids - present):
        errors.append(f"LICENSES/{lid}.txt がありません（REUSE.toml で使用）")
    for lid in sorted(present - used_ids):
        errors.append(f"LICENSES/{lid}.txt はどの表からも使われていません")

    resolved: dict[str, int] = {}
    file_lic: dict[str, str] = {}
    wins: dict[tuple[int, str], int] = {}
    for f in files:
        winner = None
        for idx, (regs, _) in enumerate(tables):
            if any(r.match(f) for _, r in regs):
                winner = idx
        if winner is None:
            errors.append(f"{f}: どの表にも当たりません")
            continue
        for pat, r in tables[winner][0]:
            if r.match(f):
                wins[(winner, pat)] = wins.get((winner, pat), 0) + 1
        lic = tables[winner][1]
        file_lic[f] = lic
        resolved[lic] = resolved.get(lic, 0) + 1

    for idx, (regs, _) in enumerate(tables):
        for pat, _ in regs:
            if pat in PENDING or wins.get((idx, pat)):
                continue
            errors.append(f"表 {idx + 1}: '{pat}' が勝つファイルが 0 件（後ろの表に覆い隠されているか、並びが崩れている）")

    for path, want in EXPECT.items():
        got = file_lic.get(path)
        if got is None:
            errors.append(f"EXPECT: {path} が追跡ファイルにありません")
        elif got != want:
            errors.append(f"EXPECT: {path} は {want} のはずが {got} に解決されました")

    print(f"追跡ファイル: {len(files)} 件（LICENSES/・LICENSE・REUSE.toml を除く）")
    for lic, cnt in sorted(resolved.items()):
        print(f"  {lic}: {cnt} 件")
    for m in notes:
        print(f"NOTE: {m}")
    for m in errors:
        print(f"ERROR: {m}")
    print(f"違反: {len(errors)} 件")
    return 1 if errors else 0


if __name__ == "__main__":
    sys.exit(main())
