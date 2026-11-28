#!/usr/bin/env python3
"""Tests for cr_index.py. Stdlib only:

    python3 -m unittest scripts/cr_index_test.py
"""

import os
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import cr_index  # noqa: E402

# Twelve lines: a contents block, then the body. The second section follows a
# form feed, a wrapped line starts with an already-seen rule number, and one
# Example: paragraph belongs to 100.1a.
SYNTHETIC = "\r\n".join([
    "Contents",
    "1. Game Concepts",
    "Glossary",
    "1. Game Concepts",
    "100. General",
    "   100.1. These rules apply to any game. See",
    "      100.1a A two-player game has two players.",
    "         Example: A game between two players.",
    "\f701. Keyword Actions",
    "   701.2. Activate",
    "100.1. for the scope of these rules.",
    "Glossary",
])


class ParseTest(unittest.TestCase):
    def test_synthetic_text_is_twelve_lines(self):
        self.assertEqual(len(SYNTHETIC.split("\n")), 12)

    def test_rows(self):
        rows = cr_index.parse(SYNTHETIC)
        self.assertEqual(rows, [
            ["100", "section", "General", 0],
            ["100.1", "rule", "", 0],
            ["100.1a", "subrule", "", 1],
            ["701", "section", "Keyword Actions", 0],
            ["701.2", "rule", "Activate", 0],
        ])

    def test_render_applies_scope(self):
        rows = cr_index.parse(SYNTHETIC)
        got = cr_index.render(rows, {"100.1"})
        self.assertEqual(got, "\n".join([
            "id\tkind\ttitle\tin_scope\texamples",
            "100\tsection\tGeneral\t1\t0",
            "100.1\trule\t\t0\t0",
            "100.1a\tsubrule\t\t0\t1",
            "701\tsection\tKeyword Actions\t1\t0",
            "701.2\trule\tActivate\t1\t0",
        ]) + "\n")
        got = cr_index.render(rows, {"100"})
        self.assertIn("100.1a\tsubrule\t\t0\t1", got)
        self.assertIn("701.2\trule\tActivate\t1\t0", got)

    def test_markers_must_appear_twice(self):
        with self.assertRaises(cr_index.CRIndexError):
            cr_index.parse(SYNTHETIC.replace("Contents\r\n1. Game Concepts", "Contents"))
        with self.assertRaises(cr_index.CRIndexError):
            cr_index.parse(SYNTHETIC + "\nGlossary")

    def test_main_writes_the_index(self):
        with tempfile.TemporaryDirectory() as d:
            text = os.path.join(d, "cr.txt")
            scope = os.path.join(d, "scope.tsv")
            out = os.path.join(d, "out.tsv")
            with open(text, "w", encoding="utf-8", newline="") as f:
                f.write(SYNTHETIC)
            with open(scope, "w", encoding="utf-8") as f:
                f.write("id\treason\n701\tkeyword actions\n")
            old = os.environ.get("CR_SCOPE_EXCLUSIONS")
            os.environ["CR_SCOPE_EXCLUSIONS"] = scope
            try:
                self.assertEqual(cr_index.main(["cr_index.py", text, out]), 0)
            finally:
                if old is None:
                    del os.environ["CR_SCOPE_EXCLUSIONS"]
                else:
                    os.environ["CR_SCOPE_EXCLUSIONS"] = old
            with open(out, encoding="utf-8") as f:
                lines = f.read().split("\n")
            self.assertEqual(lines[0], cr_index.HEADER)
            self.assertIn("701.2\trule\tActivate\t0\t0", lines)


if __name__ == "__main__":
    unittest.main()
