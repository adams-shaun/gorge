#!/usr/bin/env python3
"""Tests for cr_comment_only.py. Stdlib only:

    python3 -m unittest scripts/cr_comment_only_test.py
"""

import os
import sys
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import cr_comment_only  # noqa: E402


def diff(path, hunks):
    """A `git diff -U0` text for one file; hunks is a list of
    (start line, removed lines, added lines)."""
    out = [
        "diff --git a/%s b/%s" % (path, path),
        "index 1111111..2222222 100644",
        "--- a/" + path,
        "+++ b/" + path,
    ]
    for start, removed, added in hunks:
        out.append("@@ -%d,%d +%d,%d @@ func f() {" % (start, len(removed), start, len(added)))
        out += ["-" + l for l in removed]
        out += ["+" + l for l in added]
    return "\n".join(out) + "\n"


class CheckDiffTest(unittest.TestCase):
    def check(self, text):
        return cr_comment_only.check_diff(text)

    def test_comment_only_rule_number_change_passes(self):
        text = diff("engine/effects/a.go", [
            (3, ["\tx := 1 // CR 702.160: Enlist"], ["\tx := 1 // CR 702.154: Enlist"]),
            (9, ["// CR 701.13a exile it"], ["// CR 701.14a exile it"]),
        ])
        problems, files, lines = self.check(text)
        self.assertEqual(problems, [])
        self.assertEqual((files, lines), (1, 2))

    def test_change_before_comment_in_non_test_file_fails(self):
        text = diff("engine/effects/a.go", [
            (3, ["\tr := 702.160 // CR 702.160"], ["\tr := 702.154 // CR 702.160"]),
        ])
        problems, _, _ = self.check(text)
        self.assertEqual(problems, [
            "engine/effects/a.go:3: edit outside a comment in a non-test file"])

    def test_change_inside_string_with_slashes_in_non_test_file_fails(self):
        text = diff("engine/effects/a.go", [
            (5, ['\tnote("see http://x CR 702.2")'], ['\tnote("see http://x CR 702.3")']),
        ])
        problems, _, _ = self.check(text)
        self.assertEqual(problems, [
            "engine/effects/a.go:5: edit outside a comment in a non-test file"])

    def test_same_string_change_in_test_file_passes(self):
        text = diff("engine/effects/a_test.go", [
            (5, ['\tnote("see http://x CR 702.2")'], ['\tnote("see http://x CR 702.3")']),
        ])
        problems, files, lines = self.check(text)
        self.assertEqual(problems, [])
        self.assertEqual((files, lines), (1, 1))

    def test_comment_after_rune_and_raw_string_passes(self):
        text = diff("engine/effects/a.go", [
            (2, ["\tc := '/'; s := `//` // CR 702.2"], ["\tc := '/'; s := `//` // CR 702.3"]),
        ])
        problems, _, _ = self.check(text)
        self.assertEqual(problems, [])

    def test_hunk_with_unequal_line_counts_fails(self):
        text = diff("engine/effects/a.go", [
            (7, ["// CR 702.2", "// more"], ["// CR 702.3"]),
        ])
        problems, _, _ = self.check(text)
        self.assertEqual(problems, ["engine/effects/a.go:7: hunk changes line count"])

    def test_word_change_beside_number_fails(self):
        text = diff("engine/effects/a.go", [
            (4, ["// CR 702.2: deathtouch"], ["// CR 702.3: Deathtouch"]),
        ])
        problems, _, _ = self.check(text)
        self.assertEqual(problems, ["engine/effects/a.go:4: changed more than a rule number"])

    def test_non_go_file_fails(self):
        text = diff("docs/a.md", [(1, ["CR 702.2"], ["CR 702.3"])])
        problems, files, _ = self.check(text)
        self.assertEqual(problems, ["docs/a.md:0: not a Go file"])
        self.assertEqual(files, 1)


if __name__ == "__main__":
    unittest.main()
