#!/usr/bin/env python3
"""scripts/check-private-refs.py の試験（標準ライブラリのみ）。

本物の禁止語は書けないので、試験用の語のハッシュで同じ照合を確かめる。
使い方: python3 -I scripts/check_private_refs_test.py
"""

import hashlib
import importlib.util
import pathlib
import unittest

_spec = importlib.util.spec_from_file_location(
    "check_private_refs", pathlib.Path(__file__).with_name("check-private-refs.py")
)
mod = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(mod)


def h(text: str) -> str:
    return hashlib.sha256(text.encode()).hexdigest()


DENY = frozenset({h("abc1234"), h("secret-note")})


class FindTest(unittest.TestCase):
    def test_short_and_long_sha(self):
        self.assertEqual(mod.find("根拠コミット: `abc1234`", DENY), [1])
        self.assertEqual(mod.find("x\nabc1234ffff0000aaaa (long)", DENY), [2])

    def test_sha_inside_longer_hex_run_is_not_a_prefix(self):
        self.assertEqual(mod.find("ffabc1234ff", DENY), [])

    def test_word_case_and_punctuation(self):
        self.assertEqual(mod.find("see Secret-Note.", DENY), [1])
        self.assertEqual(mod.find("secret-notes", DENY), [])

    def test_word_with_extension_path_or_separator_variants(self):
        # 文書の名前は、拡張子・パス・区切りの違う形で書かれやすい(S7 レビュー中-2)。
        self.assertEqual(mod.find("`secret-note.md`", DENY), [1])
        self.assertEqual(mod.find("docs/secret-note.md", DENY), [1])
        self.assertEqual(mod.find("secret_note を見る", DENY), [1])
        self.assertEqual(mod.find("old-secret-note-v2", DENY), [1])
        self.assertEqual(mod.find("secret.note", DENY), [1])

    def test_clean_text(self):
        self.assertEqual(mod.find("本家（非公開）の SHA は伏せた", DENY), [])

    def test_real_denylist_is_hashes_only(self):
        for d in mod.DENY:
            self.assertRegex(d, r"^[0-9a-f]{64}$")


if __name__ == "__main__":
    unittest.main()
