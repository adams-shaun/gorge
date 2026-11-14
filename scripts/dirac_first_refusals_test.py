#!/usr/bin/env python3
"""Tests for the --pools mode of `dirac-first-refusals.py evidence`. Stdlib only:

    python3 -I -m unittest discover -s scripts -p 'dirac_first_refusals_test.py'

write_evidence works over any module root it is handed, so these build a temporary one with two
evidence lists, a refused entry and a fake <out>, and check that naming one pool leaves the other
pool's list and evidence-games row byte for byte as they were.
"""

import importlib.util
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


class TestWriteEvidencePools(unittest.TestCase):
    def setUp(self):
        self.mod = load_module()
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.top = os.path.join(self.tmp.name, "root")
        self.out = os.path.join(self.tmp.name, "out")
        os.makedirs(os.path.join(self.top, "dirac/support/evidence"))
        os.makedirs(os.path.join(self.top, "dirac/support/refused"))
        os.makedirs(self.out)
        with open(os.path.join(self.top, "dirac/support/evidence/std-alpha.txt"), "w") as f:
            f.write("AlphaOld\n")
        with open(os.path.join(self.top, "dirac/support/evidence/std-beta.txt"), "w") as f:
            f.write("BetaOld\n")
        with open(os.path.join(self.top, "dirac/support/evidence-games.tsv"), "w") as f:
            f.write("std-alpha\t18\nstd-beta\t24\n")
        with open(os.path.join(self.top, "dirac/support/refused/refused-card.json"), "w") as f:
            json.dump({"card": "RefusedCard"}, f)
        played = {
            "std-alpha": {"AlphaOld": (1, 0), "AlphaNew": (1, 0), "RefusedCard": (1, 0)},
            "std-beta": {"BetaOld": (1, 0), "BetaNew": (1, 0)},
        }
        for pool, cards in played.items():
            for surface in ("autopay", "full"):
                for m in (0, 1):
                    form = f"{surface}.m{m}"
                    with open(os.path.join(self.out, f"{pool}.{form}.rc"), "w") as f:
                        f.write("0\n")
                    with open(os.path.join(self.out, f"{pool}.{form}.coverage"), "w") as f:
                        f.write("\n".join(coverage_lines(cards)) + "\n")
                    with open(os.path.join(self.out, f"{pool}.{form}.out"), "w") as f:
                        f.write(verdict_line(pool, 20 if pool == "std-alpha" else 30) + "\n")

    def write(self, pools=None):
        self.mod.write_evidence(self.top, self.out, pools)

    def list_bytes(self, pool):
        with open(os.path.join(self.top, "dirac/support/evidence", pool + ".txt"), "rb") as f:
            return f.read()

    def tsv_bytes(self):
        with open(os.path.join(self.top, "dirac/support/evidence-games.tsv"), "rb") as f:
            return f.read()

    def snapshot(self):
        return {"alpha": self.list_bytes("std-alpha"), "beta": self.list_bytes("std-beta"),
                "tsv": self.tsv_bytes()}

    def test_failed_run_refuses_and_writes_nothing(self):
        before = self.snapshot()
        with open(os.path.join(self.out, "std-alpha.autopay.m1.rc"), "w") as f:
            f.write("1\n")
        with self.assertRaises(SystemExit) as cm:
            self.write(["std-alpha"])
        self.assertEqual(cm.exception.code, 1)
        self.assertEqual(self.snapshot(), before, "a failed run must change nothing")

    def test_passing_pool_gains_new_keeps_old(self):
        self.write(["std-alpha"])
        self.assertEqual(self.list_bytes("std-alpha"), b"AlphaNew\nAlphaOld\n")
        self.assertEqual(self.list_bytes("std-beta"), b"BetaOld\n")
        self.assertEqual(self.tsv_bytes(), b"std-alpha\t20\nstd-beta\t24\n")

    def test_refused_card_never_written(self):
        self.write(["std-alpha"])
        self.assertNotIn(b"RefusedCard", self.list_bytes("std-alpha"))

    def test_no_runs_refuses(self):
        before = self.snapshot()
        with self.assertRaises(SystemExit) as cm:
            self.write(["std-gamma"])
        self.assertEqual(cm.exception.code, 1)
        self.assertEqual(self.snapshot(), before, "a pool with no runs must change nothing")

    def test_missing_form_refuses(self):
        before = self.snapshot()
        os.remove(os.path.join(self.out, "std-alpha.full.m0.rc"))
        with self.assertRaises(SystemExit) as cm:
            self.write(["std-alpha"])
        self.assertEqual(cm.exception.code, 1)
        self.assertEqual(self.snapshot(), before, "an unfinished pool must change nothing")

    def test_without_pools_rewrites_everything(self):
        """The old behaviour, kept for settle: dirty pools lose their list, clean ones are rebuilt
        from the whole corpus, every tsv row is rewritten."""
        with open(os.path.join(self.out, "std-beta.full.m0.rc"), "w") as f:
            f.write("1\n")
        self.write()
        self.assertEqual(self.list_bytes("std-alpha"), b"AlphaNew\nAlphaOld\n")
        self.assertFalse(os.path.exists(os.path.join(self.top, "dirac/support/evidence/std-beta.txt")))
        self.assertEqual(self.tsv_bytes(), b"std-alpha\t20\nstd-beta\t30\n")


def bad_game_line(pair, impl, held):
    return f"bad-game\t{pair}\t{pair + 100}\tdivergent\tstate life\t{impl}\t{impl}\t{held}"


class TestCleanGameCredit(TestWriteEvidencePools):
    """A synthetic pool whose runs finished with bad games credits, over its clean games, the cards
    no bad game holds, and nothing a bad game holds."""

    def fail_with_bad_game(self, form="full.m0", extra=""):
        with open(os.path.join(self.out, f"std-alpha.{form}.rc"), "w") as f:
            f.write("1\n")
        verdict = ("std-alpha: 20 games (0 refused-card, 0 refused-pair, 0 error), 19 clean, "
                   "1 divergent, 0 unmodelled [], 0 capacity [], 0 hang")
        with open(os.path.join(self.out, f"std-alpha.{form}.out"), "w") as f:
            f.write(verdict + "\n" + bad_game_line(7, "AlphaNew", "AlphaNew|Island|Plains") + "\n" + extra)

    def test_clean_game_credit(self):
        played = {"AlphaOld": (1, 0), "AlphaNew": (1, 0), "AlphaClean": (1, 0)}
        for surface in ("autopay", "full"):
            for m in (0, 1):
                with open(os.path.join(self.out, f"std-alpha.{surface}.m{m}.coverage"), "w") as f:
                    f.write("\n".join(coverage_lines(played)) + "\n")
        self.fail_with_bad_game()
        self.write(["std-alpha"])
        self.assertEqual(self.list_bytes("std-alpha"), b"AlphaClean\nAlphaOld\n",
                         "the card the bad game holds stays unverified; the clean one is credited")
        self.assertEqual(self.tsv_bytes(), b"std-alpha\t20\nstd-beta\t24\n")

    def test_unnamed_failure_still_refuses(self):
        before = self.snapshot()
        self.fail_with_bad_game(extra="unknown-game\t9\t109\tchoose/x\n")
        with self.assertRaises(SystemExit):
            self.write(["std-alpha"])
        self.assertEqual(self.snapshot(), before, "an unknown option kind must change nothing")

    def test_deck_pool_keeps_the_all_clean_rule(self):
        self.assertIsNone(self.mod.run_finished(self.out, "fdn", "full.m0"))


if __name__ == "__main__":
    unittest.main()
