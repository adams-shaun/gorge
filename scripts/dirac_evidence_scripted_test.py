#!/usr/bin/env python3
"""Tests for the scripted-start evidence rule (design section 5, T8). Stdlib only:

    python3 -m unittest scripts/dirac_evidence_scripted_test.py

A scripted run is a scenario's game at one seed, named sc-<pool>-<seed>. Its coverage
credits key uses and never plays (D10), so a card with an existing organic evidence
row can be credited by scripted runs only when its organic plays floor is still met
and every key organic games never used was used in `EvidenceScriptedMinGames` clean
scripted games over at least two distinct seeds. These build a temporary module root
and a fake <out>, and check the list, the sidecar's two new columns, the seed and
game floors, the organic-play floor, the bad-game hold and refusal.
"""

import hashlib
import importlib.util
import json
import os
import tempfile
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
EVIDENCE = "dirac/support/evidence"
KEYS = "dirac/support/evidence-keys"
ORGANIC_HEADER = "card\tlir16\tplays\tkeys\tunused\tbranches\tmissing_branches"
SCRIPTED_HEADER = ORGANIC_HEADER + "\tscenario\torganic_plays"


def load_module():
    spec = importlib.util.spec_from_file_location("dirac_first_refusals", os.path.join(HERE, "dirac-first-refusals.py"))
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


def coverage_lines(cards):
    """cards: card -> (cast, land, keys_used, keys_total, unused)."""
    return ["card\tcast\tland\tkeys_used\tkeys_total\tunused_keys"] + [
        f"{card}\t{cast}\t{land}\t{used}\t{total}\t{unused}"
        for card, (cast, land, used, total, unused) in cards.items()
    ]


def verdict_line(run, clean=1):
    return (f"{run}: {clean} games (0 refused-card, 0 refused-pair, 0 error), {clean} clean, "
            "0 divergent, 0 unmodelled [], 0 capacity [], 0 hang")


class TestScriptedEvidence(unittest.TestCase):
    def setUp(self):
        self.mod = load_module()
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.top = os.path.join(self.tmp.name, "root")
        self.out = os.path.join(self.tmp.name, "out")
        for d in (EVIDENCE, KEYS, "dirac/support/refused"):
            os.makedirs(os.path.join(self.top, d))
        os.makedirs(self.out)
        self.scenario = os.path.join(self.out, "sc-hello.json")
        with open(self.scenario, "w") as f:
            f.write('{"format":"gorge-start-1","focus":"Card"}')
        # The organic evidence row: five organic plays, one key (f0/a1) organic
        # games never used. The scripted runs supply it.
        with open(os.path.join(self.top, KEYS, "std-org.tsv"), "w") as f:
            f.write(ORGANIC_HEADER + "\n")
            f.write("Card\t\t5\t1/2\tf0/a1\t0/0\t\n")

    def scripted(self, seed, cards, clean=1, bad=None, name=None):
        """One sc- run at seed, one game, as kfuzz -scenario -pairs 1 writes it."""
        run = name or f"sc-hello-{seed}"
        with open(os.path.join(self.out, run + ".coverage"), "w") as f:
            f.write("\n".join(coverage_lines(cards)) + "\n")
        text = ""
        if bad:
            text += bad + "\n"
        text += verdict_line(run, clean) + "\n"
        with open(os.path.join(self.out, run + ".verdict"), "w") as f:
            f.write(text)

    def write(self, pools=("sc-hello",)):
        try:
            self.mod.write_evidence(self.top, self.out, list(pools))
        except SystemExit as e:
            return e.code
        return 0

    def list_text(self, pool="sc-hello"):
        path = os.path.join(self.top, EVIDENCE, pool + ".txt")
        if not os.path.exists(path):
            return None
        with open(path) as f:
            return f.read()

    def sidecar_text(self, pool="sc-hello"):
        path = os.path.join(self.top, KEYS, pool + ".tsv")
        if not os.path.exists(path):
            return None
        with open(path) as f:
            return f.read()

    def used_card(self):
        return {"Card": (0, 0, 2, 2, "")}

    def test_three_games_over_two_seeds_credit_the_key(self):
        # Seeds 1 and 2: three clean scripted games in all, two distinct seeds.
        self.scripted(1, self.used_card())
        self.scripted(1, self.used_card(), name="sc-hello-01")
        self.scripted(2, self.used_card())
        self.assertEqual(self.write(), 0)
        self.assertEqual(self.list_text(), "Card\n")
        side = self.sidecar_text().splitlines()
        self.assertEqual(side[0], SCRIPTED_HEADER)
        with open(self.scenario, "rb") as f:
            sha = hashlib.sha1(f.read()).hexdigest()
        self.assertEqual(side[1], f"Card\t\t0\t2/2\t\t0/0\t\t{sha}\t5")

    def test_two_games_do_not_credit(self):
        # Two runs are two games, under EvidenceScriptedMinGames.
        self.scripted(1, self.used_card())
        self.scripted(2, self.used_card())
        self.assertEqual(self.write(), 0)
        self.assertEqual(self.list_text(), "")
        self.assertEqual(self.sidecar_text(), SCRIPTED_HEADER + "\n")

    def test_one_seed_does_not_credit(self):
        # Three games but one distinct seed: the rule needs two.
        for name in ("sc-hello-1", "sc-hello-01", "sc-hello-001"):
            self.scripted(1, self.used_card(), name=name)
        self.assertEqual(self.write(), 0)
        self.assertEqual(self.list_text(), "")

    def test_organic_play_floor_is_required(self):
        # Nine scripted seeds cannot replace the organic plays entirely (D10).
        with open(os.path.join(self.top, KEYS, "std-org.tsv"), "w") as f:
            f.write(ORGANIC_HEADER + "\n")
            f.write("Card\t\t2\t1/2\tf0/a1\t0/0\t\n")
        for seed in range(1, 10):
            self.scripted(seed, self.used_card())
        self.assertEqual(self.write(), 0)
        self.assertEqual(self.list_text(), "")

    def test_scripted_plays_never_reach_the_organic_floor(self):
        # A scripted game credits key uses only: even a coverage row claiming
        # casts and land plays cannot meet the organic plays floor.
        with open(os.path.join(self.top, KEYS, "std-org.tsv"), "w") as f:
            f.write(ORGANIC_HEADER + "\n")
            f.write("Card\t\t0\t1/2\tf0/a1\t0/0\t\n")
        for seed in range(1, 4):
            self.scripted(seed, {"Card": (5, 5, 2, 2, "")})
        self.assertEqual(self.write(), 0)
        self.assertEqual(self.list_text(), "")

    def test_all_keys_used_organically_needs_no_scripted_games(self):
        # (b) is vacuous when organic games used every key; the organic floor
        # (a) alone credits the card, and the scripted coverage supplies its row.
        with open(os.path.join(self.top, KEYS, "std-org.tsv"), "w") as f:
            f.write(ORGANIC_HEADER + "\n")
            f.write("Card\t\t5\t2/2\t\t0/0\t\n")
        self.scripted(1, {"Card": (0, 0, 2, 2, "")})
        self.assertEqual(self.write(), 0)
        self.assertEqual(self.list_text(), "Card\n")

    def test_a_bad_game_holds_the_card(self):
        # (c): a bad game in any of the card's scenarios keeps it out, while
        # another card of the same game's deck table (not implicated) still
        # credits: the last field is the table, not the implication.
        bad = "bad-game\t0\t1\tdivergent\tstate X\tCard\t\tCard|Other"
        self.scripted(1, {"Card": (0, 0, 2, 2, ""), "Other": (0, 0, 2, 2, "")})
        self.scripted(2, {"Card": (0, 0, 2, 2, ""), "Other": (0, 0, 2, 2, "")})
        self.scripted(3, {"Card": (0, 0, 2, 2, ""), "Other": (0, 0, 2, 2, "")}, bad=bad)
        with open(os.path.join(self.top, KEYS, "std-org.tsv"), "a") as f:
            f.write("Other\t\t5\t1/2\tf0/a1\t0/0\t\n")
        self.assertEqual(self.write(), 0)
        self.assertEqual(self.list_text(), "Other\n")

    def test_existing_scripted_list_is_kept_and_refused_leaves(self):
        with open(os.path.join(self.top, EVIDENCE, "sc-hello.txt"), "w") as f:
            f.write("Card\nOld\nRefusedCard\n")
        with open(os.path.join(self.top, "dirac/support/refused/r.json"), "w") as f:
            json.dump({"card": "RefusedCard"}, f)
        self.scripted(1, {"Card": (0, 0, 2, 2, "")})
        self.assertEqual(self.write(), 0)
        self.assertEqual(self.list_text(), "Card\nOld\n")

    def test_no_scripted_runs_refuses_and_writes_nothing(self):
        before = (self.list_text(), self.sidecar_text())
        self.assertEqual(self.write(("sc-missing",)), 1)
        self.assertEqual((self.list_text(), self.sidecar_text()), before)

    def test_this_cycles_organic_runs_supply_the_row(self):
        # No evidence-keys row: this cycle's organic clean runs give both the
        # plays floor and the key that scripted games must fill in.
        os.remove(os.path.join(self.top, KEYS, "std-org.tsv"))
        for surface in ("autopay", "full"):
            for m in (0, 1):
                form = f"{surface}.m{m}"
                with open(os.path.join(self.out, f"std-org.{form}.rc"), "w") as f:
                    f.write("0\n")
                with open(os.path.join(self.out, f"std-org.{form}.coverage"), "w") as f:
                    f.write("\n".join(coverage_lines({"Card": (1, 0, 1, 2, "f0/a1")})) + "\n")
                with open(os.path.join(self.out, f"std-org.{form}.out"), "w") as f:
                    f.write(verdict_line("std-org", 20) + "\n")
        self.scripted(1, self.used_card())
        self.scripted(2, self.used_card())
        self.scripted(3, self.used_card())
        self.assertEqual(self.write(), 0)
        self.assertEqual(self.list_text(), "Card\n")
        side = self.sidecar_text().splitlines()
        self.assertEqual(side[1].split("\t")[-1], "4")

    def test_organic_write_ignores_scripted_coverage(self):
        # A scripted run's zero plays must not disturb the organic merged
        # coverage, and an organic-list write must not touch a sc- list.
        with open(os.path.join(self.top, EVIDENCE, "std-org.txt"), "w") as f:
            f.write("Organic\n")
        with open(os.path.join(self.top, EVIDENCE, "sc-hello.txt"), "w") as f:
            f.write("Card\n")
        for surface in ("autopay", "full"):
            for m in (0, 1):
                form = f"{surface}.m{m}"
                with open(os.path.join(self.out, f"std-org.{form}.rc"), "w") as f:
                    f.write("0\n")
                with open(os.path.join(self.out, f"std-org.{form}.coverage"), "w") as f:
                    f.write("\n".join(coverage_lines({"Organic": (1, 0, 1, 1, "")})) + "\n")
                with open(os.path.join(self.out, f"std-org.{form}.out"), "w") as f:
                    f.write(verdict_line("std-org", 20) + "\n")
        self.scripted(1, self.used_card())
        self.assertEqual(self.write(("std-org",)), 0)
        self.assertEqual(self.list_text("std-org"), "Organic\n")
        self.assertEqual(self.list_text("sc-hello"), "Card\n")
        # An organic sidecar keeps its seven columns: only an sc- sidecar adds
        # the scenario sha and organic plays.
        with open(os.path.join(self.top, KEYS, "std-org.tsv")) as f:
            self.assertEqual(f.readline().rstrip("\n"), ORGANIC_HEADER)


if __name__ == "__main__":
    unittest.main()
