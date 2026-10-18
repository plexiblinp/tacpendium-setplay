#!/usr/bin/env python3
"""重い検査を「入力が変わった分だけ」回すための判定(S4 後の開発者決定 3・4。2026-10-11 の見直しで案 A)。

検査済みの組を鍵の一覧ファイル(CI のキャッシュに置く)に残し、同じ組は二度と回さない。
鍵 = sha256(検査の種類, キャラ, そのキャラの calcFingerprint, その種類の計算の版, そのキャラの追加の版)。

- calcFingerprint は moves DB(data/moves/index.json)のキャラ単位の指紋で、セットプレイの計算に効く列だけから作る
  (表記 ・名前 ・メモ ・ヒット時の有利は含まない)。だから計算に関係ない列だけの変更では鍵が変わらない。
- **コードの変更は鍵に直接は入れない。** 見張るコード(logic-version.json の watched)が変わったら、`check` が
  CI を赤にして止める。変更した人(取り込み担当)が影響を判断して `ack` で記録する:
    --none                      影響なし(何も回さない)
    --kinds K --chars C1,C2     その種類 ・キャラだけ回す(キャラの追加の版を上げる)
    --kinds K --all             その種類の全キャラを回す(種類の計算の版を上げる。--kinds 省略で全種類)
  記録には理由が要り、履歴(log)に残る。判断の誤りに備え、Go job は毎回軽い突き合わせ(全キャラ ・低い KA)を回す。

使い方:
  verify-plan.py check
  verify-plan.py ack --reason "…" --none | --all [--kinds oracle,wasm] | --kinds oracle --chars ryu,ken
  verify-plan.py plan   --kind oracle --verified .verify/oracle.txt   → 標準出力の最後の行が「chars=ryu,ken」(空なら回さない)
  verify-plan.py record --kind oracle --verified .verify/oracle.txt --chars ryu,ken
  (--all を付けた plan は、検査済みを無視して全部を出す。手動実行用)
"""
import argparse
import datetime
import hashlib
import json
import os
import subprocess
import sys

# 検査の種類 → 対象のキャラ(None は全キャラ)。
KINDS = {
    # 本家の正解との突き合わせ(ネイティブ。engine の TestConformance_UpstreamOracle)
    "oracle": None,
    # WASM の突き合わせと Node の通し(engine/conformance_test.go の wasmConformanceCharacters と
    # scripts/wasm/wasm.test.mjs のキャラ)。キャラ数が少ないので、どれか 1 つでも未検査なら全部回す。
    "wasm": ["ryu", "zangief", "dhalsim", "jamie", "lily"],
}

LOGIC_FILE = os.path.join(os.path.dirname(os.path.abspath(__file__)), "logic-version.json")
MAX_LINES = 5000  # 一覧ファイルの上限(古い鍵から捨てる)

# go.mod は丸ごとではなく、計算に効く部分だけを見る(2026-10-18。Dependabot の取込 CLI 向けの依存の更新で
# 毎回 check が赤になっていたため): go ・toolchain の行と、計算のパッケージ(テストを含む。ネイティブと WASM)が
# 実際に使うモジュールの版。取込 CLI(cmd/movesdb)だけが使う依存の更新はここに出ない(その出力は CI の
# 「moves DB の再生成と一致」と calcFingerprint で見ている)。
GO_MOD = "go.mod"
GO_MOD_KEYWORDS = ("go", "toolchain")
CALC_PACKAGES = ["./engine/...", "./internal/service/...", "./internal/repository/...", "./cmd/precompute"]
CALC_WASM_PACKAGES = ["./cmd/wasm", "./engine/", "./internal/service/setplay/", "./internal/repository/setplay/"]
GO_LIST_FORMAT = "{{with .Module}}{{.Path}}@{{.Version}}{{with .Replace}} => {{.Path}}@{{.Version}}{{end}}{{end}}"


# ---- 見張るコードのダイジェスト(作業ツリーの中身から。コミット前でも CI でも同じ値になる)

def tracked_files(path):
    """path 以下の、git が扱う(追跡中 ・未追跡で無視されていない)ファイル。"""
    out = subprocess.run(["git", "ls-files", "-co", "--exclude-standard", "--", path],
                         capture_output=True, text=True, check=True)
    return sorted(p for p in out.stdout.splitlines() if os.path.isfile(p))


def digest(path, files=tracked_files):
    h = hashlib.sha256()
    for p in files(path):
        with open(p, "rb") as f:
            h.update(p.encode() + b"\0" + hashlib.sha256(f.read()).digest())
    return h.hexdigest()


def go_list_modules(env_extra):
    """計算のパッケージが使うモジュール(go list の 1 行 1 モジュール)。失敗したら止める(見張りを黙って外さない)。"""
    packages = CALC_WASM_PACKAGES if env_extra else CALC_PACKAGES
    out = subprocess.run(["go", "list", "-deps", "-test", "-f", GO_LIST_FORMAT, *packages],
                         capture_output=True, text=True, env={**os.environ, **env_extra})
    if out.returncode != 0:
        raise SystemExit(f"ERROR: go list に失敗した({GO_MOD} の見張り): {out.stderr.strip()}")
    return out.stdout.splitlines()


def go_mod_calc_lines(go_mod_text, list_modules=go_list_modules):
    """go.mod のうち計算に効く部分(並べ替え済みの行)。本体のモジュール(版も置き換えも無い行)は除く。"""
    lines = sorted(ln.strip() for ln in go_mod_text.splitlines() if (ln.split() or [""])[0] in GO_MOD_KEYWORDS)
    modules = set()
    for env_extra in ({}, {"GOOS": "js", "GOARCH": "wasm"}):
        modules.update(m for m in list_modules(env_extra) if m and not is_main_module(m))
    return lines + sorted(modules)


def is_main_module(line):
    """本体のモジュールは版が空(「path@」)で、置き換えも無い。"""
    return line.endswith("@") and " => " not in line


def go_mod_digest(path=GO_MOD, list_modules=go_list_modules):
    with open(path, encoding="utf-8") as f:
        lines = go_mod_calc_lines(f.read(), list_modules)
    return hashlib.sha256("\n".join(lines).encode()).hexdigest()


def watched_digest(path):
    return go_mod_digest(path) if path == GO_MOD else digest(path)


def current_trees(logic, dig=watched_digest):
    return {p: dig(p) for p in logic["watched"]}


def changed_paths(logic, dig=watched_digest):
    cur = current_trees(logic, dig)
    return [p for p in logic["watched"] if logic["trees"].get(p) != cur[p]]


# ---- 鍵

def keys_for(kind, index, logic):
    """キャラ → 鍵。index は data/moves/index.json の中身、logic は logic-version.json の中身。"""
    only = KINDS[kind]
    k = logic["kinds"][kind]
    fps = {c["code"]: c["calcFingerprint"] for c in index["characters"]}
    chars = only if only is not None else [c["code"] for c in index["characters"]]
    out = {}
    for c in chars:
        if c not in fps:
            raise SystemExit(f"ERROR: {kind}: キャラ {c} が index.json に無い")
        extra = k.get("chars", {}).get(c, 0)
        out[c] = hashlib.sha256(f"{kind}\n{c}\n{fps[c]}\n{k['version']}\n{extra}".encode()).hexdigest()
    return out


def read_verified(path):
    if not os.path.exists(path):
        return []
    with open(path, encoding="utf-8") as f:
        return [line.strip() for line in f if line.strip()]


def plan(kind, index, logic, verified, force=False):
    """回すキャラ(index の順)。wasm は 1 つでも未検査なら全部。"""
    keys = keys_for(kind, index, logic)
    done = set(verified)
    todo = [c for c, k in keys.items() if force or k not in done]
    if kind == "wasm" and todo:
        todo = list(keys)
    return todo


def record(kind, index, logic, verified, chars):
    keys = keys_for(kind, index, logic)
    unknown = [c for c in chars if c not in keys]
    if unknown:
        raise SystemExit(f"ERROR: {kind}: 対象外のキャラ {unknown}")
    lines = list(verified)
    have = set(lines)
    for c in chars:
        if keys[c] not in have:
            lines.append(keys[c])
            have.add(keys[c])
    return lines[-MAX_LINES:]


def ack(logic, index, reason, mode, kinds, chars, dig=watched_digest, now=None):
    """見張るコードの変更の影響を記録した新しい logic を返す。mode は none / all / chars。"""
    if not reason.strip():
        raise SystemExit("ERROR: --reason が要る(なぜその判断か)")
    kinds = kinds or sorted(KINDS)
    for k in kinds:
        if k not in KINDS:
            raise SystemExit(f"ERROR: 知らない検査の種類 {k}")
    new = json.loads(json.dumps(logic))
    if mode == "all":
        for k in kinds:
            new["kinds"][k]["version"] += 1
    elif mode == "chars":
        if not chars:
            raise SystemExit("ERROR: --chars が空")
        codes = {c["code"] for c in index["characters"]}
        for c in chars:
            if c not in codes:
                raise SystemExit(f"ERROR: 知らないキャラ {c}")
        for k in kinds:
            extra = new["kinds"][k].setdefault("chars", {})
            for c in chars:
                extra[c] = extra.get(c, 0) + 1
    elif mode != "none":
        raise SystemExit(f"ERROR: 知らない判断 {mode}")
    before = changed_paths(logic, dig)
    new["trees"] = current_trees(logic, dig)
    when = (now or datetime.datetime.now(datetime.timezone.utc)).strftime("%Y-%m-%dT%H:%M:%SZ")
    new.setdefault("log", []).append({
        "at": when, "decision": mode, "kinds": kinds if mode != "none" else [],
        "chars": chars if mode == "chars" else [], "changed": before, "reason": reason.strip(),
    })
    return new


def load_json(path):
    with open(path, encoding="utf-8") as f:
        return json.load(f)


def write_json(path, v):
    with open(path, "w", encoding="utf-8") as f:
        json.dump(v, f, ensure_ascii=False, indent=2)
        f.write("\n")


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("cmd", choices=["check", "ack", "plan", "record"])
    ap.add_argument("--kind", choices=sorted(KINDS))
    ap.add_argument("--kinds", default="")
    ap.add_argument("--verified")
    ap.add_argument("--index", default="data/moves/index.json")
    ap.add_argument("--logic", default=LOGIC_FILE)
    ap.add_argument("--chars", default="")
    ap.add_argument("--reason", default="")
    ap.add_argument("--all", action="store_true")
    ap.add_argument("--none", action="store_true")
    a = ap.parse_args()
    logic = load_json(a.logic)
    index = load_json(a.index)
    chars = [c for c in a.chars.split(",") if c]

    if a.cmd == "check":
        ch = changed_paths(logic)
        if ch:
            print("見張るコードが記録(scripts/ci/logic-version.json)と違う: " + ", ".join(ch))
            print("影響を判断して記録すること(コミットに含める):")
            print('  影響なし:              python3 scripts/ci/verify-plan.py ack --none --reason "…"')
            print('  一部のキャラだけ回す:  python3 scripts/ci/verify-plan.py ack --kinds oracle,wasm --chars ryu,ken --reason "…"')
            print('  全部回す:              python3 scripts/ci/verify-plan.py ack --all --reason "…"')
            print("違反: 1 件")
            return 1
        print(f"見張るコード {len(logic['watched'])} か所: 記録と一致")
        print("違反: 0 件")
        return 0
    if a.cmd == "ack":
        modes = [m for m, on in (("none", a.none), ("all", a.all), ("chars", bool(chars))) if on]
        if len(modes) != 1:
            raise SystemExit("ERROR: --none / --all / --chars のどれか 1 つを指定する")
        kinds = [k for k in a.kinds.split(",") if k]
        new = ack(logic, index, a.reason, modes[0], kinds, chars)
        write_json(a.logic, new)
        e = new["log"][-1]
        print(f"記録した: {e['decision']} kinds={e['kinds']} chars={e['chars']} 変わったコード={e['changed']}")
        return 0
    if not a.kind or not a.verified:
        raise SystemExit("ERROR: plan / record には --kind と --verified が要る")
    verified = read_verified(a.verified)
    if a.cmd == "plan":
        todo = plan(a.kind, index, logic, verified, force=a.all)
        total = len(keys_for(a.kind, index, logic))
        print(f"{a.kind}: 対象 {total} キャラ / 検査済みの鍵 {len(verified)} 件 / 回す {len(todo)} キャラ" + (" (手動実行: 全部)" if a.all else ""))
        print("chars=" + ",".join(todo))
        return 0
    lines = record(a.kind, index, logic, verified, chars)
    os.makedirs(os.path.dirname(a.verified) or ".", exist_ok=True)
    with open(a.verified, "w", encoding="utf-8") as f:
        f.write("".join(line + "\n" for line in lines))
    print(f"{a.kind}: 記録 {len(chars)} キャラ / 鍵 {len(lines)} 件")
    return 0


if __name__ == "__main__":
    sys.exit(main())
