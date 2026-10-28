#!/usr/bin/env python3
"""Tests for docs_sets.py. Stdlib only:

    python3 -m unittest discover -s scripts -p 'docs_sets_test.py'
"""

import json
import os
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import docs_sets  # noqa: E402

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
SNAPSHOT = os.path.join(ROOT, "docs", "site", "support", "dirty-dirac-sets.json")

A_ROWS = [
    # audited, declared B, also in dirty-dirac
    {"set": "FDN", "name": "Foundations", "set_type": "EXPANSION", "released": "2024-11-15",
     "level": "A", "declared": "B", "printed": True, "cards": 517, "outstanding": 0},
    # audited, gorge only
    {"set": "BIG", "name": "The Big Score", "set_type": "SUPPLEMENTAL_STANDARD_LEGAL",
     "released": "2024-04-19", "level": "A", "declared": "B", "printed": True, "cards": 30,
     "outstanding": 0},
    # not audited, but dirty-dirac has it
    {"set": "BRO", "name": "The Brothers' War", "set_type": "EXPANSION", "released": "2022-11-18",
     "level": "A", "printed": False, "cards": 280, "outstanding": 263, "problem_cards": 263,
     "buckets": {"no printed list": 1, "no verdict": 241, "unsupported": 21}},
    # not audited, no dirty-dirac: only in the roll-up
    {"set": "LEA", "name": "Limited Edition Alpha", "set_type": "CORE", "released": "1993-08-05",
     "level": "A", "printed": False, "cards": 284, "outstanding": 256, "problem_cards": 256,
     "buckets": {"no printed list": 1, "unsupported": 17}},
]
B_ROWS = {
    "FDN": {"set": "FDN", "level": "B", "declared": "B", "printed": True, "cards": 517, "outstanding": 0},
    "BIG": {"set": "BIG", "level": "B", "declared": "B", "printed": True, "cards": 30,
            "outstanding": 1, "problem_cards": 1, "buckets": {"harness": 1}},
}
DD = {
    "date": "2026-10-21", "commit": "0123456789ab",
    "corpus": {"accepted": 50, "cards": 100},
    "lockstep": {"FDN": {"games": 10, "agree": 9, "stalls": 1, "disagree": 0}},
    "sets": {"FDN": {"accepted": 471, "cards": 517}, "BRO": {"accepted": 144, "cards": 280},
             "ZZZ": {"accepted": 1, "cards": 2}},
}
META = {"commit": "671efbb30abc", "gorge_pin": "40aee00fabfd", "date": "2026-10-21"}


def section(text, name):
    start = "<!-- --8<-- [start:%s] -->" % name
    end = "<!-- --8<-- [end:%s] -->" % name
    return text[text.index(start) + len(start):text.index(end)]


def row(text, code):
    for line in text.splitlines():
        if line.strip().startswith("| %s |" % code):
            return [c.strip() for c in line.strip().strip("|").split("|")]
    return None


class RenderTest(unittest.TestCase):
    def setUp(self):
        self.out = docs_sets.render(A_ROWS, META, B_ROWS, DD)

    def test_sections(self):
        for name in ("headline", "sets", "rollup"):
            section(self.out, name)

    def test_join_by_set_code(self):
        sets = section(self.out, "sets")
        fdn = row(sets, "FDN")
        self.assertEqual(fdn[2], "B")
        self.assertEqual(fdn[7], "471 / 517 (91.1%)")
        self.assertEqual(fdn[8], "9 / 10 (1 stalled)")
        # gorge only
        self.assertEqual(row(sets, "BIG")[7], "—")
        self.assertEqual(row(sets, "BIG")[6], "96.7")
        # dirty-dirac set without a printed list: Level A is a lower bound
        bro = row(sets, "BRO")
        self.assertEqual(bro[5], "≥ 6.4")
        self.assertEqual(bro[6], "—")
        # a code only dirty-dirac has still renders
        self.assertEqual(row(sets, "ZZZ")[7], "1 / 2 (50.0%)")
        # an unaudited set dirty-dirac lacks stays out of the by-engine table
        self.assertIsNone(row(sets, "LEA"))
        self.assertIn("BIG: 1 harness", sets)

    def test_rollup_carries_dirty_dirac(self):
        rollup = section(self.out, "rollup")
        self.assertIn("| BRO | The Brothers' War |", rollup)
        self.assertIn("144 / 280 (51.4%) |", rollup)
        self.assertIn("| **All** | **4** |", rollup)

    def test_headline(self):
        head = section(self.out, "headline")
        self.assertIn("**50 of 100** cards (50.0%)", head)
        self.assertIn("`671efbb30`", head)
        # The private module's commit is never published.
        self.assertNotIn("`012345678`", head)
        self.assertIn("the dirty-dirac snapshot of 2026-10-21", head)

    def test_without_status(self):
        out = docs_sets.render([], {}, {}, DD)
        self.assertIn("not available to this build", section(out, "headline"))
        self.assertEqual(row(section(out, "sets"), "FDN")[7], "471 / 517 (91.1%)")

    def test_without_dirty_dirac(self):
        out = docs_sets.render(A_ROWS, META, B_ROWS, None)
        self.assertEqual(row(section(out, "sets"), "FDN")[7], "—")
        self.assertIsNone(row(section(out, "sets"), "BRO"))


class MainTest(unittest.TestCase):
    def test_needs_some_input(self):
        with tempfile.TemporaryDirectory() as d:
            self.assertEqual(docs_sets.main(["-o", os.path.join(d, "x.md")]), 1)

    def test_files(self):
        with tempfile.TemporaryDirectory() as d:
            a = os.path.join(d, "status-A.jsonl")
            with open(a, "w") as f:
                f.write("ratchet: summary line\n")
                for r in A_ROWS:
                    f.write(json.dumps(r) + "\n")
            dd = os.path.join(d, "dd.json")
            with open(dd, "w") as f:
                json.dump(DD, f)
            meta = os.path.join(d, "meta.json")
            with open(meta, "w") as f:
                json.dump(META, f)
            out = os.path.join(d, "out.md")
            self.assertEqual(docs_sets.main([a, "--meta", meta, "--dirty-dirac", dd, "-o", out]), 0)
            with open(out) as f:
                self.assertIn("471 / 517", f.read())

    def test_bad_snapshot(self):
        bad = dict(DD, sets={"FDN": {"accepted": 600, "cards": 517}})
        with tempfile.TemporaryDirectory() as d:
            p = os.path.join(d, "dd.json")
            with open(p, "w") as f:
                json.dump(bad, f)
            with self.assertRaises(ValueError):
                docs_sets.load_dirty_dirac(p)

    def test_committed_snapshot(self):
        dd = docs_sets.load_dirty_dirac(SNAPSHOT)
        for code in dd["sets"]:
            self.assertRegex(code, r"^[0-9A-Z]{3,5}$")
        for code in dd.get("lockstep", {}):
            self.assertIn(code, dd["sets"])


if __name__ == "__main__":
    unittest.main()
