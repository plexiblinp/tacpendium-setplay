#!/usr/bin/env python3
"""verify-plan.py の試験(標準ライブラリの unittest)。

  python3 -I scripts/ci/verify_plan_test.py
"""
import datetime
import importlib.util
import os
import unittest
import unittest.mock

_spec = importlib.util.spec_from_file_location(
    "verify_plan", os.path.join(os.path.dirname(os.path.abspath(__file__)), "verify-plan.py"))
vp = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(vp)

NOW = datetime.datetime(2026, 10, 11, tzinfo=datetime.timezone.utc)
ALL5 = ["ryu", "zangief", "dhalsim", "jamie", "lily"]

# 全キャラが対象の種類どうしの独立を見るため、試験の間だけ全キャラ対象の種類 "other" を足す
# (本物の種類は oracle(全キャラ)と wasm(5 キャラ)。S6 で繋ぎ表の sweep を消した)。
_kinds_patch = unittest.mock.patch.dict(vp.KINDS, {"other": None})


def setUpModule():
    _kinds_patch.start()


def tearDownModule():
    _kinds_patch.stop()


def index(**fps):
    return {"characters": [{"code": c, "calcFingerprint": fp} for c, fp in fps.items()]}


def digests(**override):
    def dig(path):
        return override.get(path, "d-" + path)
    return dig


def logic(dig=None):
    lg = {"watched": ["internal", "engine"], "trees": {},
          "kinds": {k: {"version": 1, "chars": {}} for k in vp.KINDS}, "log": []}
    lg["trees"] = vp.current_trees(lg, dig or digests())
    return lg


class PlanTest(unittest.TestCase):
    def setUp(self):
        self.idx = index(ryu="f1", ken="f2", lily="f3")
        self.lg = logic()

    def verified(self, kind, idx=None, lg=None, chars=None):
        idx = idx or self.idx
        return vp.record(kind, idx, lg or self.lg, [], chars or list(vp.keys_for(kind, idx, lg or self.lg)))

    def test_nothing_verified_runs_all(self):
        self.assertEqual(vp.plan("other", self.idx, self.lg, []), ["ryu", "ken", "lily"])

    def test_all_verified_runs_none(self):
        self.assertEqual(vp.plan("other", self.idx, self.lg, self.verified("other")), [])

    def test_changed_fingerprint_runs_only_that_character(self):
        v = self.verified("other")
        self.assertEqual(vp.plan("other", index(ryu="f1", ken="f2-new", lily="f3"), self.lg, v), ["ken"])

    def test_code_change_alone_does_not_change_keys(self):
        # コードの変更は鍵に入れない(check が止め、ack が判断を記録する)。
        v = self.verified("other")
        lg = logic(digests(internal="changed"))
        self.assertEqual(vp.plan("other", self.idx, lg, v), [])
        self.assertEqual(vp.changed_paths(self.lg, digests(internal="changed")), ["internal"])
        self.assertEqual(vp.changed_paths(self.lg, digests()), [])

    def test_kinds_do_not_share_keys(self):
        self.assertEqual(vp.plan("oracle", self.idx, self.lg, self.verified("other")), ["ryu", "ken", "lily"])

    def test_force_runs_all(self):
        self.assertEqual(vp.plan("other", self.idx, self.lg, self.verified("other"), force=True), ["ryu", "ken", "lily"])

    def test_wasm_runs_all_its_characters_if_any_changed(self):
        idx = index(ryu="a", zangief="b", dhalsim="c", jamie="d", lily="e", ken="f")
        v = self.verified("wasm", idx)
        self.assertEqual(vp.plan("wasm", idx, self.lg, v), [])
        self.assertEqual(vp.plan("wasm", index(ryu="a", zangief="b", dhalsim="c", jamie="d", lily="e", ken="z"), self.lg, v), [])
        changed = index(ryu="a", zangief="b2", dhalsim="c", jamie="d", lily="e", ken="f")
        self.assertEqual(vp.plan("wasm", changed, self.lg, v), ALL5)

    def test_record_rejects_unknown_character(self):
        with self.assertRaises(SystemExit):
            vp.record("other", self.idx, self.lg, [], ["nobody"])

    def test_record_keeps_old_keys_and_caps(self):
        old = [f"k{i}" for i in range(vp.MAX_LINES)]
        lines = vp.record("other", self.idx, self.lg, old, ["ryu"])
        self.assertEqual(len(lines), vp.MAX_LINES)
        self.assertEqual(lines[-1], vp.keys_for("other", self.idx, self.lg)["ryu"])


class AckTest(unittest.TestCase):
    def setUp(self):
        self.idx = index(ryu="f1", ken="f2", lily="f3")
        self.lg = logic()
        self.v = {k: vp.record(k, self.idx, self.lg, [], ["ryu", "ken", "lily"]) for k in ("other", "oracle")}
        self.changed = digests(internal="changed")

    def test_none_records_trees_and_runs_nothing(self):
        new = vp.ack(self.lg, self.idx, "表示だけの修正", "none", [], [], self.changed, NOW)
        self.assertEqual(vp.changed_paths(new, self.changed), [])
        for k in ("other", "oracle"):
            self.assertEqual(vp.plan(k, self.idx, new, self.v[k]), [])
        e = new["log"][-1]
        self.assertEqual((e["decision"], e["changed"], e["reason"]), ("none", ["internal"], "表示だけの修正"))

    def test_chars_reruns_only_those_characters_and_kinds(self):
        new = vp.ack(self.lg, self.idx, "連打の修正", "chars", ["other"], ["ken"], self.changed, NOW)
        self.assertEqual(vp.plan("other", self.idx, new, self.v["other"]), ["ken"])
        self.assertEqual(vp.plan("oracle", self.idx, new, self.v["oracle"]), [])

    def test_all_reruns_every_character_of_the_kinds(self):
        new = vp.ack(self.lg, self.idx, "計算の変更", "all", ["oracle"], [], self.changed, NOW)
        self.assertEqual(vp.plan("oracle", self.idx, new, self.v["oracle"]), ["ryu", "ken", "lily"])
        self.assertEqual(vp.plan("other", self.idx, new, self.v["other"]), [])
        every = vp.ack(self.lg, self.idx, "計算の変更", "all", [], [], self.changed, NOW)
        self.assertEqual(vp.plan("other", self.idx, every, self.v["other"]), ["ryu", "ken", "lily"])

    def test_ack_does_not_mutate_input_and_requires_reason(self):
        before = repr(self.lg)
        vp.ack(self.lg, self.idx, "x", "all", [], [], self.changed, NOW)
        self.assertEqual(repr(self.lg), before)
        with self.assertRaises(SystemExit):
            vp.ack(self.lg, self.idx, "  ", "none", [], [], self.changed, NOW)
        with self.assertRaises(SystemExit):
            vp.ack(self.lg, self.idx, "x", "chars", ["other"], ["nobody"], self.changed, NOW)
        with self.assertRaises(SystemExit):
            vp.ack(self.lg, self.idx, "x", "all", ["nokind"], [], self.changed, NOW)


class DigestTest(unittest.TestCase):
    def test_digest_depends_on_contents_and_names(self):
        import tempfile
        with tempfile.TemporaryDirectory() as d:
            a, b = os.path.join(d, "a.go"), os.path.join(d, "b.go")
            for p, s in ((a, "x"), (b, "y")):
                with open(p, "w") as f:
                    f.write(s)
            files = lambda path: sorted([a, b])  # noqa: E731
            d1 = vp.digest(d, files)
            with open(b, "w") as f:
                f.write("y2")
            self.assertNotEqual(d1, vp.digest(d, files))


GO_MOD_V1 = """module example.com/m

go 1.26.4

toolchain go1.26.9

require modernc.org/sqlite v1.58.0
"""


def modules(native=(), wasm=()):
    """go list の代わり。本体のモジュール(「path@」)は常に混ぜる。"""
    def run(env_extra):
        return ["example.com/m@", *(wasm if env_extra else native)]
    return run


class GoModTest(unittest.TestCase):
    def test_only_go_and_toolchain_lines_and_calc_modules_count(self):
        self.assertEqual(vp.go_mod_calc_lines(GO_MOD_V1, modules()), ["go 1.26.4", "toolchain go1.26.9"])

    def test_dependency_used_only_outside_calc_is_ignored(self):
        bumped = GO_MOD_V1.replace("sqlite v1.58.0", "sqlite v1.60.1")
        self.assertEqual(vp.go_mod_calc_lines(GO_MOD_V1, modules()), vp.go_mod_calc_lines(bumped, modules()))

    def test_toolchain_change_counts(self):
        older = GO_MOD_V1.replace("toolchain go1.26.9", "toolchain go1.26.8")
        self.assertNotEqual(vp.go_mod_calc_lines(GO_MOD_V1, modules()), vp.go_mod_calc_lines(older, modules()))

    def test_calc_module_versions_count_native_and_wasm(self):
        base = vp.go_mod_calc_lines(GO_MOD_V1, modules())
        native = vp.go_mod_calc_lines(GO_MOD_V1, modules(native=["golang.org/x/sys@v0.48.0"]))
        wasm = vp.go_mod_calc_lines(GO_MOD_V1, modules(wasm=["golang.org/x/sys@v0.48.0"]))
        self.assertIn("golang.org/x/sys@v0.48.0", native)
        self.assertIn("golang.org/x/sys@v0.48.0", wasm)
        self.assertNotEqual(base, native)
        bumped = vp.go_mod_calc_lines(GO_MOD_V1, modules(native=["golang.org/x/sys@v0.49.0"]))
        self.assertNotEqual(native, bumped)

    def test_main_module_is_dropped_but_its_replacement_is_kept(self):
        self.assertTrue(vp.is_main_module("example.com/m@"))
        self.assertFalse(vp.is_main_module("example.com/x@ => ../x@"))
        self.assertIn("example.com/x@ => ../x@",
                      vp.go_mod_calc_lines(GO_MOD_V1, modules(native=["example.com/x@ => ../x@"])))


if __name__ == "__main__":
    unittest.main(verbosity=2)
