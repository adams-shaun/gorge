#!/usr/bin/env python3
"""Tests for docs_cert.py. Stdlib only (run the way docs_sets_test.py is run
in .github/workflows/docs.yml, or under pytest):

    python3 -m unittest discover -s scripts -p 'docs_cert_test.py'
    python3 -m pytest scripts/docs_cert_test.py
"""

import json
import os
import re
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import docs_cert  # noqa: E402

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
SUPPORT = os.path.join(ROOT, "docs", "site", "support")

META = {"gorge_sha": "a" * 40, "corpus_pin": "418ba5215f",
        "compliance_sha": "b" * 40, "measured": "2026-11-21T04:00:00Z"}


def set_row(code, released, **kw):
    s = {"set_code": code, "gorge_sha": "a" * 40, "corpus_pin": "418ba5215f",
         "released": released, "set_type": "expansion",
         "paper": 285, "new_paper": 285, "reprints": 0, "playable": 280,
         "excluded": 2, "c0": 200, "c1": 60, "c2": 20, "c3": 0, "open": 3,
         "level": "", "state": "triage", "cursor": "cert", "pages_cards": 285,
         "measured": "2026-11-21T04:00:00Z"}
    if code == "LEA":
        s["set_type"] = "core"
    s.update(kw)
    return s


def card(name, first_set, **kw):
    c = {"name": name, "first_set": first_set, "sets": [first_set],
         "level": "", "exclusion": "", "refs": "", "why": "", "tickets": "",
         "keys_total": 0, "keys_agreed": 0, "triggers_unmeasured": 0}
    c.update(kw)
    return c


# LEA (oldest, triage) and FDN (newest, certified at C3).
SETS = [
    set_row("LEA", "1993-08-05"),
    set_row("FDN", "2024-11-15", paper=517, new_paper=490, reprints=27,
            playable=517, excluded=0, c0=7, c1=10, c2=100, c3=400, open=0,
            level="C3", state="certified", cursor="", pages_cards=517),
]

CARDS = [
    card("Ancestral Recall", "LEA", level="C2", refs="FX",
         keys_total=1, keys_agreed=1),
    card("Black Lotus", "LEA", level="C2", refs="FX",
         keys_total=1, keys_agreed=1),
    card("Chaos Orb", "LEA", exclusion="dexterity",
         why="banned; dexterity (CR 702.24)"),
    card("Raging River", "LEA", why="gorge-unplayable: api:TwoPiles.OtherShape",
         tickets="gorge-1234"),
    # a reprint: first printed in LEA, also paper in FDN
    card("Lightning Bolt", "LEA", level="C1", refs="F",
         keys_total=2, keys_agreed=1, triggers_unmeasured=1, sets=["LEA", "FDN"]),
    card("Shieldline Sentinel", "FDN", level="C3", refs="FX",
         keys_total=3, keys_agreed=3),
]


def fixture(d):
    """SC13's cert-data export in a temporary directory."""
    cd = os.path.join(d, "cert-data")
    os.makedirs(cd)
    with open(os.path.join(cd, "cert-sets.jsonl"), "w") as f:
        for s in SETS:
            f.write(json.dumps(s) + "\n")
    with open(os.path.join(cd, "cert-cards.jsonl"), "w") as f:
        f.write("certloop: 6 cards over 2 sets\n")  # a summary line is skipped
        for c in CARDS:
            f.write(json.dumps(c) + "\n")
    with open(os.path.join(cd, "cert-meta.json"), "w") as f:
        json.dump(META, f)
    return cd


def cells(text):
    """Every table cell of every markdown table row in the text."""
    for line in text.splitlines():
        s = line.strip()
        if s.startswith("|") and s.endswith("|") and "---" not in s:
            for c in s.strip("|").split("|"):
                yield c.strip()


def row(text, code):
    for line in text.splitlines():
        if line.strip().startswith("| [%s]" % code):
            return [c.strip() for c in line.strip().strip("|").split("|")]
    return None


class RenderTest(unittest.TestCase):
    def setUp(self):
        self.page = docs_cert.render_cert(META, {s["set_code"]: s for s in SETS},
                                          {c["name"]: c for c in CARDS})

    def test_overview_in_release_order(self):
        self.assertLess(self.page.index("LEA"), self.page.index("FDN"))
        lea = row(self.page, "LEA")
        self.assertEqual(lea[0], "[LEA](cert/LEA.md)")
        self.assertEqual(lea[1], "1993-08-05")
        self.assertEqual(lea[2:10], ["285", "285", "200", "60", "20", "0", "2", "3"])
        self.assertEqual(lea[10], "triage")
        self.assertEqual(lea[11], "—")
        fdn = row(self.page, "FDN")
        self.assertEqual(fdn[2:10], ["517", "490", "7", "10", "100", "400", "0", "0"])
        self.assertEqual(fdn[10], "certified")
        self.assertEqual(fdn[11], "C3")

    def test_limits_box_has_six_bullets(self):
        self.assertEqual(len(docs_cert.LIMITS), 6)
        for b in docs_cert.LIMITS:
            self.assertIn(b, self.page)
        self.assertIn("What this evidence proves", self.page)

    def test_overview_names_the_levels(self):
        for name in ("playable", "exercised", "agreed", "certified"):
            self.assertIn(name, self.page)

    def test_meta_stamp(self):
        self.assertIn("2026-11-21", self.page)
        self.assertIn("`%s`" % ("a" * 9), self.page)
        self.assertIn("`418ba5215f`", self.page)
        self.assertIn("`%s`" % ("b" * 9), self.page)

    def test_without_cert_data_says_not_measured(self):
        page = docs_cert.render_cert({}, {}, {})
        self.assertIn("not measured", page)
        self.assertNotIn("cert/", page)  # no set links to dead pages
        self.assertEqual(len(docs_cert.LIMITS), 6)  # the limits box survives

    def test_set_page_lists_cards_with_level_refs_why_tickets(self):
        lea = docs_cert.render_set(SETS[0], docs_cert.cards_in_set(
            {c["name"]: c for c in CARDS}, "LEA"))
        rows = [[x.strip() for x in c.strip().strip("|").split("|")] for c in
                lea.splitlines() if c.strip().startswith("| ")][1:]
        names = [r[0] for r in rows]
        self.assertEqual(names, sorted(names))  # sorted by card name, deterministic
        bolt = next(r for r in rows if "Lightning Bolt" in r[0])
        self.assertEqual(bolt[0], "Lightning Bolt")
        self.assertEqual(bolt[1], "C1")
        self.assertEqual(bolt[2], "F")
        river = next(r for r in rows if "Raging River" in r[0])
        self.assertIn("api:TwoPiles.OtherShape", river[3])
        self.assertEqual(river[4], "gorge-1234")
        orb = next(r for r in rows if "Chaos Orb" in r[0])
        self.assertEqual(orb[1], "excluded:dexterity")
        self.assertIn("never counted as certified", lea)

    def test_reprint_shows_certified_at_first_set(self):
        fdn = docs_cert.render_set(SETS[1], docs_cert.cards_in_set(
            {c["name"]: c for c in CARDS}, "FDN"))
        rows = [[x.strip() for x in c.strip().strip("|").split("|")] for c in
                fdn.splitlines() if c.strip().startswith("| ")][1:]
        bolt = next(r for r in rows if "Lightning Bolt" in r[0])
        self.assertEqual(bolt[1], "certified at LEA (C1)")
        sent = next(r for r in rows if "Shieldline Sentinel" in r[0])
        self.assertEqual(sent[1], "C3")

    def test_set_page_summary(self):
        fdn = docs_cert.render_set(SETS[1], [])
        self.assertIn("certified at C3", fdn)
        self.assertIn("2024-11-15", fdn)
        self.assertIn("517", fdn)
        self.assertIn("No cards published", fdn)


class MainTest(unittest.TestCase):
    def test_renders_overview_and_set_pages(self):
        with tempfile.TemporaryDirectory() as d:
            out = os.path.join(d, "support")
            cd = fixture(d)
            self.assertEqual(docs_cert.main(["--cert-data", cd, "-o", out]), 0)
            page = open(os.path.join(out, "certification.md")).read()
            self.assertIn("[LEA](cert/LEA.md)", page)
            lea = open(os.path.join(out, "cert", "LEA.md")).read()
            self.assertIn("Black Lotus", lea)
            self.assertIn("[Overview](../certification.md)", lea)
            fdn = open(os.path.join(out, "cert", "FDN.md")).read()
            self.assertIn("certified at LEA (C1)", fdn)

    def test_no_field_over_200_chars(self):
        with tempfile.TemporaryDirectory() as d:
            out = os.path.join(d, "support")
            cd = fixture(d)
            docs_cert.main(["--cert-data", cd, "-o", out])
            for root, _, files in os.walk(out):
                for name in files:
                    text = open(os.path.join(root, name)).read()
                    long_cells = [c for c in cells(text) if len(c) > 200]
                    self.assertEqual([], long_cells, "%s: cell over 200 chars" % name)

    def test_missing_branch_renders_not_measured(self):
        with tempfile.TemporaryDirectory() as d:
            out = os.path.join(d, "support")
            self.assertEqual(docs_cert.main(["-o", out]), 0)
            page = open(os.path.join(out, "certification.md")).read()
            self.assertIn("not measured", page)
            self.assertFalse(os.path.exists(os.path.join(out, "cert")))

    def test_missing_branch_clears_stale_set_pages(self):
        with tempfile.TemporaryDirectory() as d:
            out = os.path.join(d, "support")
            cd = fixture(d)
            docs_cert.main(["--cert-data", cd, "-o", out])
            self.assertTrue(os.path.exists(os.path.join(out, "cert", "LEA.md")))
            docs_cert.main(["-o", out])
            self.assertFalse(os.path.exists(os.path.join(out, "cert")))

    def test_empty_data_dir_still_passes(self):
        with tempfile.TemporaryDirectory() as d:
            cd = os.path.join(d, "cert-data")
            os.makedirs(cd)
            out = os.path.join(d, "support")
            self.assertEqual(docs_cert.main(["--cert-data", cd, "-o", out]), 0)
            self.assertIn("not measured", open(os.path.join(out, "certification.md")).read())


if __name__ == "__main__":
    unittest.main()
