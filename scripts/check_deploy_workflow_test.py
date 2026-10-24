#!/usr/bin/env python3
"""check-deploy-workflow.py の試験。今の ci.yml は通り、守りを 1 つずつ外すと赤になる(AGENTS §8-3)。"""
import importlib.util
import pathlib
import unittest

ROOT = pathlib.Path(__file__).resolve().parent.parent
spec = importlib.util.spec_from_file_location("cdw", ROOT / "scripts/check-deploy-workflow.py")
cdw = importlib.util.module_from_spec(spec)
spec.loader.exec_module(cdw)
TEXT = (ROOT / ".github/workflows/ci.yml").read_text(encoding="utf-8")
DEPLOY_IF = "    if: ${{ github.ref == 'refs/heads/main' && (github.event_name == 'push' || github.event_name == 'workflow_dispatch') }}\n    runs-on"
UPLOAD_IF = "        if: ${{ success() && github.ref == 'refs/heads/main' && (github.event_name == 'push' || github.event_name == 'workflow_dispatch') }}\n"


def broken(old, new):
    assert TEXT.count(old) == 1, old
    return TEXT.replace(old, new)


class CheckDeployWorkflowTest(unittest.TestCase):
    def test_current_passes(self):
        self.assertEqual(cdw.check(TEXT), [])

    def test_main_condition_required(self):
        self.assertEqual(len(cdw.check(broken(DEPLOY_IF, "    if: ${{ github.event_name == 'push' }}\n    runs-on"))), 1)

    def test_push_condition_required(self):
        self.assertEqual(len(cdw.check(broken(DEPLOY_IF, "    if: ${{ github.ref == 'refs/heads/main' }}\n    runs-on"))), 1)

    def test_pull_request_in_condition(self):
        text = broken(DEPLOY_IF, DEPLOY_IF.replace("github.event_name == 'workflow_dispatch'", "github.event_name == 'pull_request'"))
        self.assertEqual(len(cdw.check(text)), 1)

    def test_needs_required(self):
        self.assertEqual(len(cdw.check(broken("    needs: [go, web]\n", "    needs: [go]\n"))), 1)

    def test_forbidden_triggers(self):
        for trig in cdw.FORBIDDEN_TRIGGERS:
            with self.subTest(trig=trig):
                self.assertEqual(len(cdw.check(broken("  workflow_dispatch:\n", f"  workflow_dispatch:\n  {trig}:\n"))), 1)

    def test_environment_required(self):
        self.assertEqual(len(cdw.check(broken("    environment: production\n", ""))), 1)

    def test_other_job_without_environment(self):
        text = broken("    name: Web\n    runs-on: ubuntu-latest\n", "    name: Web\n    environment: production\n    runs-on: ubuntu-latest\n")
        self.assertEqual(len(cdw.check(text)), 1)

    def test_secrets_outside_deploy_step(self):
        # deploy job の pnpm install の段に secrets を足すと赤。
        text = broken("        run: pnpm install --frozen-lockfile --ignore-scripts\n",
                      "        env:\n          T: ${{ secrets.CLOUDFLARE_API_TOKEN }}\n        run: pnpm install --frozen-lockfile --ignore-scripts\n")
        self.assertEqual(len(cdw.check(text)), 1)
        # Web の job に足しても赤。
        text = broken("        run: pnpm install --frozen-lockfile\n",
                      "        env:\n          T: ${{ secrets.CLOUDFLARE_API_TOKEN }}\n        run: pnpm install --frozen-lockfile\n")
        self.assertEqual(len(cdw.check(text)), 1)

    def test_artifact_condition(self):
        # 成果物をいつでも作るようにすると赤。
        self.assertEqual(len(cdw.check(broken(UPLOAD_IF, "        if: ${{ success() }}\n"))), 1)
        self.assertEqual(len(cdw.check(broken(UPLOAD_IF, ""))), 1)


if __name__ == "__main__":
    unittest.main()
