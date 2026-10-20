#!/usr/bin/env python3
"""check-deploy-workflow.py の試験。今の deploy.yml は通り、守りを 1 つずつ外すと赤になる(AGENTS §8-3)。"""
import importlib.util
import pathlib
import unittest

ROOT = pathlib.Path(__file__).resolve().parent.parent
spec = importlib.util.spec_from_file_location("cdw", ROOT / "scripts/check-deploy-workflow.py")
cdw = importlib.util.module_from_spec(spec)
spec.loader.exec_module(cdw)
TEXT = (ROOT / ".github/workflows/deploy.yml").read_text(encoding="utf-8")


def broken(old, new):
    assert TEXT.count(old) == 1, old
    return TEXT.replace(old, new)


class CheckDeployWorkflowTest(unittest.TestCase):
    def test_current_passes(self):
        self.assertEqual(cdw.check(TEXT), [])

    def test_each_condition_is_required(self):
        for cond in cdw.REQUIRED_CONDITIONS:
            with self.subTest(cond=cond):
                self.assertEqual(len(cdw.check(broken(cond, "true"))), 1)

    def test_pull_request_target(self):
        self.assertEqual(len(cdw.check(broken("  workflow_dispatch:\n", "  workflow_dispatch:\n  pull_request_target:\n"))), 1)

    def test_environment_required(self):
        self.assertEqual(len(cdw.check(broken("    environment: production\n", ""))), 1)

    def test_build_job_without_environment(self):
        self.assertEqual(len(cdw.check(broken("    runs-on: ubuntu-latest\n    timeout-minutes: 20\n", "    runs-on: ubuntu-latest\n    environment: production\n    timeout-minutes: 20\n"))), 1)

    def test_secrets_outside_deploy_step(self):
        # pnpm install の段に secrets を足すと赤。
        text = broken("        run: pnpm install --frozen-lockfile --ignore-scripts\n",
                      "        env:\n          T: ${{ secrets.CLOUDFLARE_API_TOKEN }}\n        run: pnpm install --frozen-lockfile --ignore-scripts\n")
        self.assertEqual(len(cdw.check(text)), 1)
        # build job に足しても赤。
        text = broken("        run: pnpm install --frozen-lockfile\n",
                      "        env:\n          T: ${{ secrets.CLOUDFLARE_API_TOKEN }}\n        run: pnpm install --frozen-lockfile\n")
        self.assertEqual(len(cdw.check(text)), 1)


if __name__ == "__main__":
    unittest.main()
