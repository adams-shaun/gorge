#!/usr/bin/env python3
"""Tests for the --pools mode of `dirac-first-refusals.py evidence`. Stdlib only:

    python3 -I -m unittest discover -s scripts -p 'dirac_first_refusals_test.py'

write_evidence works over any module root it is handed, so these build a temporary one with two
evidence lists, a refused entry and a fake <out>, and check that naming one pool leaves the other
pool's list and evidence-games row byte for byte as they were.
"""

import contextlib
import glob
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

    def pool_p(self, branch_rows, casts=(3, 0, 0, 0)):
        """A pool "p" with four passing runs; run i's coverage holds X with casts[i] casts (zero
        leaves it out), and only the first run's branches hold branch_rows."""
        i = 0
        for surface in ("autopay", "full"):
            for m in (0, 1):
                form = f"{surface}.m{m}"
                stem = os.path.join(self.out, f"p.{form}")
                with open(stem + ".rc", "w") as f:
                    f.write("0\n")
                with open(stem + ".coverage", "w") as f:
                    f.write("card\tcast\tland\tkeys_used\tkeys_total\tunused_keys\n")
                    if casts[i]:
                        f.write(f"X\t{casts[i]}\t0\t2\t2\t\n")
                with open(stem + ".branches", "w") as f:
                    f.write("card\tkey\tuses\towed\n")
                    if i == 0:
                        f.write("".join(f"X\t{r}\n" for r in branch_rows))
                with open(stem + ".out", "w") as f:
                    f.write(verdict_line("p", 20) + "\n")
                i += 1

    def keys_file(self, pool):
        with open(os.path.join(self.top, "dirac/support/evidence-keys", pool + ".tsv")) as f:
            return f.read()

    def test_sidecar_written_beside_list(self):
        self.pool_p(["f0/s\t3\t1", "ix:resp\t0\t1"])
        lir = os.path.join(self.tmp.name, "lir.tsv")
        with open(lir, "w") as f:
            f.write("X\t0123456789abcdef\n")
        os.environ["DFR_LIR_KEYS"] = lir
        self.addCleanup(os.environ.pop, "DFR_LIR_KEYS", None)
        self.write(["p"])
        self.assertEqual(self.list_bytes("p"), b"X\n")
        self.assertEqual(self.keys_file("p"),
                         "card\tlir16\tplays\tkeys\tunused\tbranches\tmissing_branches\n"
                         "X\t0123456789abcdef\t3\t2/2\t\t1/2\tix:resp\n")
        # A card already in the sidecar keeps its row byte for byte, whatever a later run says.
        with open(lir, "w") as f:
            f.write("X\tfedcba9876543210\n")
        self.write(["p"])
        self.assertIn("0123456789abcdef", self.keys_file("p"))
        self.assertNotIn("fedcba9876543210", self.keys_file("p"))

    def test_a_keyed_card_leaves_legacy(self):
        """A card the write gives a sidecar row is dropped from LEGACY.txt; other legacy names stay."""
        self.pool_p(["f0/s\t3\t1"])
        lir = os.path.join(self.tmp.name, "lir.tsv")
        with open(lir, "w") as f:
            f.write("X\t0123456789abcdef\n")
        os.environ["DFR_LIR_KEYS"] = lir
        self.addCleanup(os.environ.pop, "DFR_LIR_KEYS", None)
        keys = os.path.join(self.top, "dirac/support/evidence-keys")
        os.makedirs(keys, exist_ok=True)
        with open(os.path.join(self.top, "dirac/support/evidence/q.txt"), "w") as f:
            f.write("Old\n")
        with open(os.path.join(keys, "LEGACY.txt"), "w") as f:
            f.write("Old\nX\n")
        self.write(["p"])
        self.assertIn("X\t0123456789abcdef", self.keys_file("p"))
        with open(os.path.join(keys, "LEGACY.txt")) as f:
            self.assertEqual(f.read(), "Old\n")

    def test_sidecar_row_refreshed_when_a_card_is_regained(self):
        """A dirty whole-corpus round deletes the pool's list but leaves its sidecar; the next clean
        round re-gains the card, which must get the current row, not the pre-deletion one."""
        lir = os.path.join(self.tmp.name, "lir.tsv")
        with open(lir, "w") as f:
            f.write("X\t0123456789abcdef\n")
        os.environ["DFR_LIR_KEYS"] = lir
        self.addCleanup(os.environ.pop, "DFR_LIR_KEYS", None)
        self.pool_p(["f0/s\t3\t1", "ix:resp\t0\t1"], casts=(3, 3, 3, 3))
        self.write(["p"])
        self.assertIn("\t12\t", self.keys_file("p"))
        with open(os.path.join(self.out, "p.full.m0.rc"), "w") as f:
            f.write("1\n")
        self.write()
        self.assertFalse(os.path.exists(os.path.join(self.top, "dirac/support/evidence/p.txt")),
                         "the dirty round removes the pool's list")
        self.assertIn("\t12\t", self.keys_file("p"), "the dirty round leaves the old sidecar")
        with open(os.path.join(self.out, "p.full.m0.rc"), "w") as f:
            f.write("0\n")
        for surface in ("autopay", "full"):
            for m in (0, 1):
                with open(os.path.join(self.out, f"p.{surface}.m{m}.coverage"), "w") as f:
                    f.write("card\tcast\tland\tkeys_used\tkeys_total\tunused_keys\nX\t5\t0\t2\t2\t\n")
        with open(lir, "w") as f:
            f.write("X\tfedcba9876543210\n")
        self.write()
        self.assertEqual(self.keys_file("p"),
                         "card\tlir16\tplays\tkeys\tunused\tbranches\tmissing_branches\n"
                         "X\tfedcba9876543210\t20\t2/2\t\t1/2\tix:resp\n")

    def test_lir_failure_without_pools_changes_nothing(self):
        """The whole-corpus write computes the sidecars before removing any list: a key lookup that
        fails must leave the evidence corpus alone, as the docstring promises."""
        os.environ.pop("DFR_LIR_KEYS", None)
        os.makedirs(os.path.join(self.top, "bin"))
        kgen = os.path.join(self.top, "bin", "kgen")
        with open(kgen, "w") as f:
            f.write("#!/bin/sh\nexit 1\n")
        os.chmod(kgen, 0o755)
        before = self.snapshot()
        with self.assertRaises(SystemExit) as cm:
            self.write()
        self.assertEqual(cm.exception.code, 1)
        self.assertEqual(self.snapshot(), before, "a failed key lookup must change nothing")

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


def pod_verdict(games, clean):
    return (f"cmd5: {games} games (0 refused-card, 0 refused-pair, 0 error), {clean} clean, "
            "0 divergent, 0 unmodelled [], 0 capacity [], 0 hang")


class TestPodEvidence(unittest.TestCase):
    """A pod run writes -evidence rows (Name<TAB><seats>) and no coverage, so the lander takes its
    list from those rows; its verdict names the set (cmd5), not the run (pod-cmd5)."""

    def setUp(self):
        self.mod = load_module()
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.top = os.path.join(self.tmp.name, "root")
        self.out = os.path.join(self.tmp.name, "out")
        os.makedirs(os.path.join(self.top, "dirac/support/evidence"))
        os.makedirs(os.path.join(self.top, "dirac/support/refused"))
        os.makedirs(self.out)

    def pod_run(self, pool="pod-cmd5", games=12, clean=10, rows=("Forest\t4", "Mountain\t4")):
        for surface in ("autopay", "full"):
            for m in (0, 1):
                stem = os.path.join(self.out, f"{pool}.{surface}.m{m}")
                with open(stem + ".rc", "w") as f:
                    f.write("0\n")
                with open(stem + ".evidence", "w") as f:
                    f.write("".join(r + "\n" for r in rows))
                with open(stem + ".out", "w") as f:
                    f.write(pod_verdict(games, clean) + "\n")

    def two_seat_run(self, pool="std-alpha"):
        for surface in ("autopay", "full"):
            for m in (0, 1):
                stem = os.path.join(self.out, f"{pool}.{surface}.m{m}")
                with open(stem + ".rc", "w") as f:
                    f.write("0\n")
                with open(stem + ".coverage", "w") as f:
                    f.write("card\tcast\tland\tpairs\twasted\tunused\nAlpha\t3\t0\t0\t0\t\n")
                with open(stem + ".out", "w") as f:
                    f.write(verdict_line(pool, 20) + "\n")

    def list_text(self, pool="pod-cmd5"):
        with open(os.path.join(self.top, "dirac/support/evidence", pool + ".txt")) as f:
            return f.read()

    def tsv_text(self):
        with open(os.path.join(self.top, "dirac/support/evidence-games.tsv")) as f:
            return f.read()

    def test_pod_run_lands_its_evidence_rows(self):
        self.pod_run()
        # Precondition: a pod run writes no coverage, and its .evidence rows carry the seats column.
        self.assertFalse(glob.glob(os.path.join(self.out, "*.coverage")))
        self.assertEqual(self.mod.evidence_rows(self.out, "pod-cmd5"), {"Forest\t4", "Mountain\t4"})
        with open(os.path.join(self.top, "dirac/support/evidence-games.tsv"), "w") as f:
            f.write("cmd5\t500\n")
        self.mod.write_evidence(self.top, self.out, ["pod-cmd5"])
        self.assertEqual(self.list_text(), "Forest\t4\nMountain\t4\n")
        self.assertEqual(self.tsv_text(), "cmd5\t500\npod-cmd5\t12\t10\n")

    def test_unrefuse_then_land_keeps_one_pod_row(self):
        # unrefuseCard restores a card as a plain row; the next pod run re-evidences it. The
        # lander must collapse the two rows by name and keep the tabbed one, or readSupport
        # takes the plain row first and the card never reads as pod-evidenced again.
        with open(os.path.join(self.top, "dirac/support/evidence/pod-cmd5.txt"), "w") as f:
            f.write("Forest\nMountain\t4\n")
        self.pod_run(rows=("Forest\t4", "Mountain\t4"))
        # Precondition: both the restored plain row and the run's tabbed row are present.
        self.assertEqual(self.list_text(), "Forest\nMountain\t4\n")
        self.mod.write_evidence(self.top, self.out, ["pod-cmd5"])
        self.assertEqual(self.list_text(), "Forest\t4\nMountain\t4\n",
                         "the plain restored row must not shadow the pod run's row")

    def test_settle_keeps_a_landed_pod_list(self):
        with open(os.path.join(self.top, "dirac/support/evidence/pod-cmd5.txt"), "w") as f:
            f.write("Forest\t4\nMountain\t4\n")
        self.two_seat_run()
        self.mod.write_evidence(self.top, self.out)
        self.assertEqual(self.list_text(), "Forest\t4\nMountain\t4\n",
                         "settle has no pod coverage run, so it must not delete a landed pod list")
        self.assertEqual(self.list_text("std-alpha"), "Alpha\n")

    def test_settle_lands_a_pod_run_in_its_out(self):
        with open(os.path.join(self.top, "dirac/support/evidence/pod-cmd5.txt"), "w") as f:
            f.write("Forest\t4\n")
        self.pod_run(rows=("Forest\t4", "Mountain\t4"))
        self.two_seat_run()
        self.mod.write_evidence(self.top, self.out)
        self.assertEqual(self.list_text(), "Forest\t4\nMountain\t4\n")
        self.assertIn("pod-cmd5\t12\t10\n", self.tsv_text())

    def test_pod_refused_card_leaves_without_a_drop(self):
        with open(os.path.join(self.top, "dirac/support/evidence/pod-cmd5.txt"), "w") as f:
            f.write("Forest\t4\nMountain\t4\n")
        with open(os.path.join(self.top, "dirac/support/refused/mountain.json"), "w") as f:
            json.dump({"card": "Mountain"}, f)
        self.pod_run()
        self.mod.write_evidence(self.top, self.out, ["pod-cmd5"])
        self.assertEqual(self.list_text(), "Forest\t4\n")

    def test_legacy_takes_the_name_from_a_tabbed_row(self):
        with open(os.path.join(self.top, "dirac/support/evidence/pod-cmd5.txt"), "w") as f:
            f.write("Forest\t4\nMountain\n")
        self.mod.write_legacy(self.top)
        with open(os.path.join(self.top, "dirac/support/evidence-keys/LEGACY.txt")) as f:
            self.assertEqual(f.read(), "Forest\nMountain\n")

    def test_report_counts_a_pod_run(self):
        self.pod_run()
        buf = io.StringIO()
        with contextlib.redirect_stdout(buf):
            self.mod.report(self.top, self.out)
        line = next(l for l in buf.getvalue().splitlines() if l.startswith("autopay m0"))
        self.assertIn("clean=10", line, "the pod run's clean games count in the report")

    def test_games_per_run_keeps_pod_and_plain_runs_apart(self):
        with open(os.path.join(self.out, "cmd5.autopay.m0.out"), "w") as f:
            f.write(verdict_line("cmd5", 7) + "\n")
        self.pod_run()
        games, pods = self.mod.games_per_run(self.out)
        self.assertEqual(games, {"cmd5": 7, "pod-cmd5": 12})
        self.assertEqual(pods, {"pod-cmd5": 10})


if __name__ == "__main__":
    unittest.main()
