#!/usr/bin/env python3
"""デプロイ(.github/workflows/ci.yml の deploy job)の守りが残っていることを確かめる(S9 の対策 R-1・R-2)。

1. deploy job は main への push・手動起動(main)のときだけ走り、Go と Web の job の成功を待つ(needs)
2. workflow_run・pull_request_target を使わない(フォークの PR の完了で起動されうる・CodeQL のキャッシュ汚染の指摘)
3. secrets を参照するのは deploy job の「wrangler deploy」の段だけで、その job は Environment production を通る
4. deploy 以外の job は Environment を持たない(トークンを持たない)
5. 出す dist の成果物は、deploy job と同じ条件のときだけ作る

★独立監査 SP-02(2026-11-10)で、文字列の一致だけでは次を見逃すと分かったので、形で見るようにした:
  条件の && を || に変える(→ 条件は許した形のどれかと完全に一致することを見る)、
  secrets['…']・toJSON(secrets)(→ ${{ }} の中の語 secrets を数える)、
  on: [push, pull_request_target] の 1 行の形(→ コメントを除いた全文で語を探す)。

使い方: python3 -I scripts/check-deploy-workflow.py [workflow のパス]
終了コード: 0 = 違反なし / 1 = 違反あり / 2 = 実行エラー
"""
import pathlib
import re
import sys

DEFAULT = pathlib.Path(__file__).resolve().parent.parent / ".github/workflows/ci.yml"
DEPLOY_JOB = "deploy"
REQUIRED_NEEDS = ("go", "web")
# deploy job と成果物の段の条件として許す形(空白を除き、${{ }} を外した形で比べる)。main への push か手動起動だけ。
_MAIN = "github.ref=='refs/heads/main'"
_EVENTS = (
    "(github.event_name=='push'||github.event_name=='workflow_dispatch')",
    "(github.event_name=='workflow_dispatch'||github.event_name=='push')",
)
DEPLOY_CONDITIONS = frozenset([f"{_MAIN}&&{e}" for e in _EVENTS] + [f"{e}&&{_MAIN}" for e in _EVENTS])
UPLOAD_CONDITIONS = frozenset(f"success()&&{c}" for c in DEPLOY_CONDITIONS)
FORBIDDEN_TRIGGERS = ("workflow_run", "pull_request_target")
DEPLOY_STEP = "- name: wrangler deploy"
ARTIFACT_NAME = "name: site-dist"


def strip_comments(text):
    """YAML のコメント(行頭の # と、空白の後の #)を除く。説明の文の語を数えないため。"""
    out = []
    for line in text.splitlines():
        if line.lstrip().startswith("#"):
            out.append("")
        else:
            out.append(re.sub(r"\s+#.*$", "", line))
    return "\n".join(out)


def normalize_condition(expr):
    """if: の値を比べる形にする(${{ }} を外し、空白を除く)。"""
    e = re.sub(r"\s+", "", expr or "")
    if e.startswith("${{") and e.endswith("}}"):
        e = e[3:-2]
    return e


def secret_refs(text):
    """${{ }} の式のうち、語 secrets を含むものの数(secrets.X・secrets['X']・toJSON(secrets) のどれも)。"""
    return sum(1 for expr in re.findall(r"\$\{\{(.*?)\}\}", strip_comments(text)) if re.search(r"\bsecrets\b", expr))


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


def job_key(lines, key):
    """job 直下(4 字下げ)の key の値。無ければ None。"""
    for line in lines:
        m = re.match(rf"^    {key}:\s*(.*)$", line)
        if m:
            return m.group(1).strip()
    return None


def step_range(lines, start_text):
    """start_text の段の行番号の範囲。無ければ None。"""
    start = next((i for i, l in enumerate(lines) if l.strip() == start_text), None)
    if start is None:
        return None
    indent = len(lines[start]) - len(lines[start].lstrip())
    end = next(
        (i for i in range(start + 1, len(lines)) if lines[i].strip().startswith("- ") and len(lines[i]) - len(lines[i].lstrip()) == indent),
        len(lines),
    )
    return range(start, end)


def check(text):
    errs = []
    code = strip_comments(text)
    for trig in FORBIDDEN_TRIGGERS:
        if re.search(rf"\b{trig}\b", code):
            errs.append(f"{trig} を使っている")
    js = jobs(text)
    if DEPLOY_JOB not in js:
        errs.append(f"job {DEPLOY_JOB} が無い")
        return errs
    deploy = js[DEPLOY_JOB]
    cond = job_key(deploy, "if") or ""
    if normalize_condition(cond) not in DEPLOY_CONDITIONS:
        errs.append(f"deploy job の起動条件が「main への push か手動起動だけ」の形でない: {cond!r}")
    needs = job_key(deploy, "needs") or ""
    for n in REQUIRED_NEEDS:
        if not re.search(rf"\b{n}\b", needs):
            errs.append(f"deploy job が {n} の job を待たない(needs)")
    if job_key(deploy, "environment") != "production":
        errs.append("deploy job が Environment production を通らない")
    for name, lines in js.items():
        if name != DEPLOY_JOB and job_key(lines, "environment") is not None:
            errs.append(f"job {name} が Environment を持っている(トークンを持たせない)")
    # secrets の参照は deploy job の wrangler deploy の段の中だけ。
    rng = step_range(deploy, DEPLOY_STEP)
    if rng is None:
        errs.append(f"deploy job に段 {DEPLOY_STEP!r} が無い")
        return errs
    total = secret_refs(text)
    inside = secret_refs("\n".join(deploy[i] for i in rng))
    if total != inside:
        errs.append(f"wrangler deploy の段の外で secrets を参照している({total - inside} 件)")
    # 出す dist の成果物は deploy job と同じ条件のときだけ作る。
    uploads = [(name, i) for name, lines in js.items() for i, l in enumerate(lines) if l.strip() == ARTIFACT_NAME]
    if not uploads:
        errs.append(f"デプロイする成果物({ARTIFACT_NAME})を作る段が無い")
    for name, i in uploads:
        if name == DEPLOY_JOB:
            continue
        lines = js[name]
        start = max(j for j in range(i + 1) if lines[j].strip().startswith("- "))
        conds = [m.group(1) for l in lines[start:i] if (m := re.match(r"^\s*(?:- )?if:\s*(.*)$", l))]
        if not conds or normalize_condition(conds[0]) not in UPLOAD_CONDITIONS:
            errs.append(f"job {name} の成果物 site-dist が deploy job と同じ条件で絞られていない")
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
