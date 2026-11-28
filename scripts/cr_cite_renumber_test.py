#!/usr/bin/env python3
"""Tests for cr_cite_renumber.py. Stdlib only:

    python3 -m unittest scripts/cr_cite_renumber_test.py
"""

import io
import os
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import cr_cite_renumber  # noqa: E402

# Four keywords. Enlist has a subrule "a" and Deathtouch has none, so a
# citation of 702.160a renumbered to Enlist keeps its letter and one
# renumbered to Deathtouch drops it.
INDEX = "\n".join([
    "id\tkind\ttitle\tin_scope\texamples",
    "701\tsection\tKeyword Actions\t1\t0",
    "701.38\trule\tVote\t1\t0",
    "702.2\trule\tDeathtouch\t1\t0",
    "702.154\trule\tEnlist\t1\t0",
    "702.154a\tsubrule\t\t1\t0",
    "702.160\trule\tPrototype\t1\t0",
    "702.160a\tsubrule\t\t1\t0",
    "702.160b\tsubrule\t\t1\t0",
    "",
])


class RenumberTest(unittest.TestCase):
    def setUp(self):
        self.dir = tempfile.TemporaryDirectory()
        self.index = os.path.join(self.dir.name, "cr-index.tsv")
        with open(self.index, "w", encoding="utf-8") as f:
            f.write(INDEX)

    def tearDown(self):
        self.dir.cleanup()

    def run_on(self, name, text, write=False):
        path = os.path.join(self.dir.name, name)
        with open(path, "w", encoding="utf-8") as f:
            f.write(text)
        out = io.StringIO()
        args = (["--write"] if write else []) + [self.index, path]
        self.assertEqual(cr_cite_renumber.main(args, stdout=out), 0)
        with open(path, encoding="utf-8") as f:
            return f.read(), out.getvalue()

    def test_one_other_keyword_is_renumbered(self):
        src = "x := 1 // CR 702.160: enlist taps a creature\n"
        got, out = self.run_on("a.go", src, write=True)
        self.assertEqual(got, "x := 1 // CR 702.154: enlist taps a creature\n")
        self.assertEqual(out, "")

    def test_without_write_prints_a_diff_and_leaves_the_file(self):
        src = "// CR 702.160: enlist taps a creature\n"
        got, out = self.run_on("a.go", src)
        self.assertEqual(got, src)
        self.assertIn("-// CR 702.160: enlist taps a creature", out)
        self.assertIn("+// CR 702.154: enlist taps a creature", out)

    def test_two_other_keywords_are_skipped(self):
        src = "// CR 702.160: enlist a creature with deathtouch\n"
        got, out = self.run_on("a.go", src, write=True)
        self.assertEqual(got, src)
        self.assertTrue(out.startswith("SKIP "), out)
        self.assertIn("a.go:1:", out)

    def test_letter_kept_when_the_new_subrule_exists(self):
        got, _ = self.run_on("a.go", "// CR 702.160a: enlist\n", write=True)
        self.assertEqual(got, "// CR 702.154a: enlist\n")

    def test_letter_dropped_when_the_new_subrule_does_not_exist(self):
        got, _ = self.run_on("a.go", "// CR 702.160b: deathtouch\n", write=True)
        self.assertEqual(got, "// CR 702.2: deathtouch\n")

    def test_string_literal_in_a_non_test_file_is_left(self):
        src = 'log("CR 702.160: enlist") // CR 702.160: enlist\n'
        got, _ = self.run_on("a.go", src, write=True)
        self.assertEqual(got, 'log("CR 702.160: enlist") // CR 702.154: enlist\n')

    def test_string_literal_in_a_test_file_is_renumbered(self):
        src = 't.Error("CR 702.160: enlist")\n'
        got, _ = self.run_on("a_test.go", src, write=True)
        self.assertEqual(got, 't.Error("CR 702.154: enlist")\n')

    def test_clean_lines_are_left(self):
        src = "\n".join([
            "// CR 702.160: prototype",              # own title
            "// CR 702.160: prototype and enlist",   # own title beside another
            "// CR 702.154: players vote",           # Vote has four letters
            "// CR 702.160: enlisting",              # not a whole word
            "// CR 702.1: enlist",                   # 702.1 is not a keyword
            "x := enlist // CR 702.160",             # code text, comment edit only
            "",
        ])
        got, out = self.run_on("a.go", src, write=True)
        self.assertEqual(got, src.replace("x := enlist // CR 702.160", "x := enlist // CR 702.154"))
        self.assertEqual(out, "")


if __name__ == "__main__":
    unittest.main()
