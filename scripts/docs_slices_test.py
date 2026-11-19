#!/usr/bin/env python3
"""Tests for the lockstep slice table: dirac_slices_summary.py (kfuzz lines to
slices-summary.json) and docs_sets.py --slices (the table and the lockstep
column). Stdlib only:

    python3 -m unittest discover -s scripts -p 'docs_slices_test.py'
"""

import json
import os
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import dirac_slices_summary  # noqa: E402
import docs_sets  # noqa: E402

KFUZZ = """\
fdn: 1500 games (0 skipped as unsupported), 1497 without divergence, 3 kernel capacity stalls [x] (lead gorge); first divergences:
fdn: 1500 games (2 refused-card, 0 refused-pair, 0 error), 1497 clean, 0 divergent, 0 unmodelled [], 3 capacity [stack=3], 0 hang (lead gorge, surface autopay)
fdn-classes-json: {}
absolute: OK
fdn: 1498 games (0 refused-card, 0 refused-pair, 0 error), 1490 clean, 0 divergent, 0 unmodelled [], 8 capacity [stack=8], 0 hang (lead gorge, surface full)
blb: 1500 games (0 refused-card, 0 refused-pair, 0 error), 1500 clean, 0 divergent, 0 unmodelled [], 0 capacity [], 0 hang (lead gorge, surface full)
cmd5: 100 games (0 refused-card, 0 refused-pair, 0 error), 90 clean, 1 divergent, 0 unmodelled [], 9 capacity [two=6], 0 hang (lead gorge, surface full)
cmd5 pods (4 seats): 100 games, 90 clean, 1 divergent, 3 capacity, 6 twoseat, 0 unmodelled, 0 hang
edh: 0 games (100 refused-card, 0 refused-pair, 0 error), 0 clean, 0 divergent, 0 unmodelled [], 0 capacity [], 0 hang (lead gorge, surface full)
edh pods (4 seats): 0 games, 0 clean, 0 divergent, 0 capacity, 0 twoseat, 0 unmodelled, 0 hang
"""

PAGE = ("| list | cards | lowered | refused | unverified | supported | 4-seat |\n|---|---:|---:|---:|---:|---:|---:|\n"
        "| FDN | 517 | 505 | 1 | 4 | 500 | 498 |\n| ALL | 900 | 800 | 2 | 8 | 790 | 780 |\n")


def table_rows(text):
    return [[c.strip() for c in ln.strip().strip("|").split("|")] for ln in text.splitlines() if ln.startswith("| ")]


def section(text, name):
    start = "<!-- --8<-- [start:%s] -->" % name
    return text[text.index(start) + len(start):text.index("<!-- --8<-- [end:%s] -->" % name)]


class LockstepSummaryTest(unittest.TestCase):
    def test_lockstep_parse_rows(self):
        d = dirac_slices_summary.summarize(KFUZZ.splitlines(), "abcdef0123456789", "2026-11-19")
        got = [(r["slice"], r["surface"], r["seats"], r["games"], r["clean"], r["divergent"], r["capacity"], r["twoseat"])
               for r in d["slices"]]
        self.assertEqual(got, [
            ("fdn", "autopay", 2, 1500, 1497, 0, 3, 0),
            ("fdn", "full", 2, 1498, 1490, 0, 8, 0),
            ("blb", "full", 2, 1500, 1500, 0, 0, 0),
            ("cmd5", "full", 4, 100, 90, 1, 3, 6),
            ("edh", "full", 4, 0, 0, 0, 0, 0),
        ])
        self.assertEqual(d["commit"], "abcdef0123456789")
        self.assertTrue(all(r["commit"] == "abcdef0123456789" and "unmodelled" in r and "hang" in r for r in d["slices"]))

    def test_lockstep_parse_ignores_noise(self):
        self.assertEqual(dirac_slices_summary.parse(["fdn-classes-json: {}", "absolute: OK", "pair 3: x"]), [])


class LockstepTableTest(unittest.TestCase):
    def render(self, sl):
        with tempfile.TemporaryDirectory() as d:
            page = os.path.join(d, "support.md")
            with open(page, "w", encoding="utf-8") as f:
                f.write(PAGE)
            out = os.path.join(d, "out.md")
            args = ["--dirac-support", page, "-o", out]
            if sl is not None:
                args += ["--slices", sl]
            self.assertEqual(docs_sets.main(args), 0)
            with open(out, encoding="utf-8") as f:
                return f.read()

    def test_lockstep_fixture_renders_table_and_column(self):
        with tempfile.TemporaryDirectory() as d:
            path = os.path.join(d, "slices-summary.json")
            with open(path, "w", encoding="utf-8") as f:
                json.dump(dirac_slices_summary.summarize(KFUZZ.splitlines(), "abcdef0123456789", "2026-11-19"), f)
            out = self.render(path)
        rows = table_rows(section(out, "slices"))
        self.assertEqual(rows[0][:4], ["Slice", "Seats", "Surface", "Games"])
        self.assertIn(["cmd5", "4", "full", "100", "90", "1", "3", "6", "0", "0"], rows)
        self.assertIn(["fdn", "2", "autopay", "1,500", "1,497", "0", "3", "0", "0", "0"], rows)
        self.assertEqual(len(rows), 1 + 5)
        self.assertIn("`abcdef012`", section(out, "slices"))
        self.assertNotIn("not measured", section(out, "slices"))
        # the sets table's lockstep column: clean / games on the full surface, the capacity stalls noted
        fdn = next(r for r in table_rows(section(out, "sets")) if r[0] == "FDN")
        self.assertEqual(fdn[9], "1,490 / 1,498 (8 stalled)")
        self.assertIn("FDN: 1,490 of 1,498 games agree, 8 stalled, 0 disagree", section(out, "headline"))

    def test_lockstep_missing_file_renders_not_measured(self):
        for path in (None, "/nonexistent/slices-summary.json"):
            out = self.render(path)
            self.assertIn("not measured", section(out, "slices"))
            self.assertEqual(table_rows(section(out, "slices")), [])
            fdn = next(r for r in table_rows(section(out, "sets")) if r[0] == "FDN")
            self.assertEqual(fdn[9], "—")
            self.assertIn("not measured", section(out, "headline"))

    def test_lockstep_bad_file_renders_not_measured(self):
        with tempfile.TemporaryDirectory() as d:
            path = os.path.join(d, "slices-summary.json")
            with open(path, "w", encoding="utf-8") as f:
                f.write('{"slices": [{"slice": "fdn"}]}')
            self.assertIn("not measured", section(self.render(path), "slices"))


if __name__ == "__main__":
    unittest.main()
