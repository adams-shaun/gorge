#!/usr/bin/env python3
"""Tests for the evidence writer's refusal to drop a card without a refusal. Stdlib only:

    python3 -I -m unittest discover -s scripts -p 'dirac_evidence_drop_test.py'

An evhunt round sweeps one pool clean but the random 1500 pairs may not reach a card the list already
holds. The old writer rewrote that pool's list from the sweep alone, so the card left the evidence
lists with no entry in dirac/support/refused and scripts/dirac-gate.sh refused the landing. These
build a temporary module root and a fake <out>, and check that the write fails loudly, naming the
card and changing nothing; a refused card may still leave, and --pools keeps an unreached card.
"""

import contextlib
import importlib.util
import io
import json
import os
import tempfile
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))


def load_module():
    spec = importlib.util.spec_from_file_location("dirac_first_refusals", os.path.join(HERE, "dirac-first-refusals.py"))
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


def verdict_line(pool, games):
    return (f"{pool}: {games} games (0 refused-card, 0 refused-pair, 0 error), {games} clean, "
            "0 divergent, 0 unmodelled [], 0 capacity [], 0 hang")


def coverage_lines(card_plays):
    """card_plays: card -> (cast, land) per run."""
    return ["card\tcast\tland\tpairs\twasted\tunused"] + [
        f"{card}\t{cast}\t{land}\t0\t0\t" for card, (cast, land) in card_plays.items()
    ]


class TestEvidenceDrop(unittest.TestCase):
    def setUp(self):
        self.mod = load_module()
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.top = os.path.join(self.tmp.name, "root")
        self.out = os.path.join(self.tmp.name, "out")
        os.makedirs(os.path.join(self.top, "dirac/support/evidence"))
        os.makedirs(os.path.join(self.top, "dirac/support/refused"))
        os.makedirs(self.out)
        with open(os.path.join(self.top, "dirac/support/evidence-games.tsv"), "w") as f:
            f.write("std-tla\t18\n")
        # The sweep's four clean runs reached Reached, and Unreached only once on two surfaces: two
        # plays, under the three-play floor, exactly as Giant Koi fell out of the stranded TLA sweep.
        for surface in ("autopay", "full"):
            for m in (0, 1):
                form = f"{surface}.m{m}"
                played = {"Reached": (3, 0)}
                if (surface, m) in (("autopay", 0), ("full", 1)):
                    played["Unreached"] = (1, 0)
                with open(os.path.join(self.out, f"std-tla.{form}.rc"), "w") as f:
                    f.write("0\n")
                with open(os.path.join(self.out, f"std-tla.{form}.coverage"), "w") as f:
                    f.write("\n".join(coverage_lines(played)) + "\n")
                with open(os.path.join(self.out, f"std-tla.{form}.out"), "w") as f:
                    f.write(verdict_line("std-tla", 20) + "\n")

    def write(self, pools=None):
        with contextlib.redirect_stderr(io.StringIO()) as err:
            try:
                self.mod.write_evidence(self.top, self.out, pools)
            except SystemExit as e:
                return e.code, err.getvalue()
        return 0, err.getvalue()

    def list_text(self, pool="std-tla"):
        with open(os.path.join(self.top, "dirac/support/evidence", pool + ".txt")) as f:
            return f.read()

    def tsv_text(self):
        with open(os.path.join(self.top, "dirac/support/evidence-games.tsv")) as f:
            return f.read()

    def test_unreached_card_refuses_and_writes_nothing(self):
        with open(os.path.join(self.top, "dirac/support/evidence/std-tla.txt"), "w") as f:
            f.write("Unreached\nReached\n")
        before = (self.list_text(), self.tsv_text())
        # The precondition the refusal depends on: the card is in the list, the sweep reached it on
        # only two plays, under the floor, and no refusal names it.
        self.assertIn("Unreached", before[0].splitlines())
        self.assertLess(self.mod.merged_coverage(self.out)["Unreached"][0], self.mod.EVIDENCE_MIN_PLAYS)
        self.assertNotIn("Unreached", self.mod.refused_names(self.top))
        code, err = self.write()
        self.assertEqual(code, 1)
        self.assertEqual((self.list_text(), self.tsv_text()), before, "a refused write must change nothing")
        self.assertIn("Unreached", err)
        self.assertIn("writing nothing", err)

    def test_refused_card_may_leave_the_list(self):
        with open(os.path.join(self.top, "dirac/support/evidence/std-tla.txt"), "w") as f:
            f.write("RefusedCard\nReached\n")
        with open(os.path.join(self.top, "dirac/support/refused/refused-card.json"), "w") as f:
            json.dump({"card": "RefusedCard"}, f)
        code, err = self.write()
        self.assertEqual(code, 0, err)
        self.assertEqual(self.list_text(), "Reached\n")
        self.assertEqual(self.tsv_text(), "std-tla\t20\n")

    def test_pools_mode_keeps_an_unreached_card(self):
        with open(os.path.join(self.top, "dirac/support/evidence/std-tla.txt"), "w") as f:
            f.write("Unreached\nReached\n")
        code, err = self.write(["std-tla"])
        self.assertEqual(code, 0, err)
        self.assertEqual(self.list_text(), "Reached\nUnreached\n")
        self.assertEqual(self.tsv_text(), "std-tla\t20\n")

    def test_pools_mode_drops_a_refused_card(self):
        with open(os.path.join(self.top, "dirac/support/evidence/std-tla.txt"), "w") as f:
            f.write("RefusedCard\nReached\n")
        with open(os.path.join(self.top, "dirac/support/refused/refused-card.json"), "w") as f:
            json.dump({"card": "RefusedCard"}, f)
        code, err = self.write(["std-tla"])
        self.assertEqual(code, 0, err)
        self.assertEqual(self.list_text(), "Reached\n")


if __name__ == "__main__":
    unittest.main()
