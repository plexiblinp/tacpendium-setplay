#!/usr/bin/env python3
"""デプロイの workflow(.github/workflows/deploy.yml)の守りが残っていることを確かめる(S9 の対策 R-1・R-2)。

1. 起動の条件: workflow_run のとき「成功・main への push・同じリポジトリ」をすべて確かめる(フォークの PR の CI の完了で動かさない)
2. pull_request_target を使わない
3. secrets を参照するのは deploy job の「wrangler deploy」の段だけで、その job は Environment production を通る
4. build job は Environment を持たない(トークンを持たない)

使い方: python3 -I scripts/check-deploy-workflow.py [workflow のパス]
終了コード: 0 = 違反なし / 1 = 違反あり / 2 = 実行エラー
"""
import pathlib
import re
import sys

DEFAULT = pathlib.Path(__file__).resolve().parent.parent / ".github/workflows/deploy.yml"
REQUIRED_CONDITIONS = (
    "github.event.workflow_run.conclusion == 'success'",
    "github.event.workflow_run.event == 'push'",
    "github.event.workflow_run.head_branch == 'main'",
    "github.event.workflow_run.head_repository.full_name == github.repository",
)
DEPLOY_STEP = "- name: wrangler deploy"


def jobs(text):
    """トップレベルの jobs: の下の job ごとの本文(job 名 → 行の list)。"""
    out, cur, inside = {}, None, False
    for line in text.splitlines():
        if line.startswith("jobs:"):
            inside = True
            continue
        if inside and re.match(r"^\S", line):
            inside = False
        if not inside:
            continue
        m = re.match(r"^  ([A-Za-z0-9_-]+):\s*$", line)
        if m:
            cur = m.group(1)
            out[cur] = []
        elif cur is not None:
            out[cur].append(line)
    return out


def check(text):
    errs = []
    if "pull_request_target" in text:
        errs.append("pull_request_target を使っている")
    js = jobs(text)
    for name in ("build", "deploy"):
        if name not in js:
            errs.append(f"job {name} が無い")
    if errs:
        return errs
    build = "\n".join(js["build"])
    for cond in REQUIRED_CONDITIONS:
        if cond not in build:
            errs.append(f"build job の起動条件が無い: {cond}")
    if re.search(r"^\s+environment:", build, re.M):
        errs.append("build job が Environment を持っている(トークンを持たせない)")
    deploy = js["deploy"]
    if not any(re.match(r"^\s+environment:\s*production\s*$", l) for l in deploy):
        errs.append("deploy job が Environment production を通らない")
    # secrets の参照は deploy job の wrangler deploy の段の中だけ。
    step_start = next((i for i, l in enumerate(deploy) if l.strip() == DEPLOY_STEP), None)
    if step_start is None:
        errs.append(f"deploy job に段 {DEPLOY_STEP!r} が無い")
        return errs
    indent = len(deploy[step_start]) - len(deploy[step_start].lstrip())
    step_end = next(
        (i for i in range(step_start + 1, len(deploy)) if deploy[i].strip().startswith("- ") and len(deploy[i]) - len(deploy[i].lstrip()) == indent),
        len(deploy),
    )
    allowed = set(range(step_start, step_end))
    total = text.count("secrets.")
    inside = sum(deploy[i].count("secrets.") for i in allowed)
    if total != inside:
        errs.append(f"wrangler deploy の段の外で secrets を参照している({total - inside} 件)")
    return errs


def main():
    path = pathlib.Path(sys.argv[1]) if len(sys.argv) > 1 else DEFAULT
    try:
        text = path.read_text(encoding="utf-8")
    except OSError as e:
        print(f"ERROR: {e}", file=sys.stderr)
        return 2
    errs = check(text)
    for e in errs:
        print(f"ERROR: {e}")
    print(f"違反: {len(errs)} 件")
    return 1 if errs else 0


if __name__ == "__main__":
    sys.exit(main())
