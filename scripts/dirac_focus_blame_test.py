#!/usr/bin/env python3
"""Tests for the focus-pool blame rule in `dirac-first-refusals.py`.

    python3 -I -m unittest discover -s scripts -p 'dirac_focus_blame_test.py'

A focus pool deals its focus cards into every game, so a diverging game there
must be answered by one of them before any other card it holds. Blaming a
bystander instead refuses a card that plays clean and leaves the card under
test credited from its clean games.
"""

import importlib.util
import os
import tempfile
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))


def load_module():
    spec = importlib.util.spec_from_file_location("dirac_first_refusals", os.path.join(HERE, "dirac-first-refusals.py"))
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


def game(pool, held, impl=(), recent=()):
    return dict(pool=pool, surface="autopay", m=0, pair=7, outcome="divergent",
                cls="projection Cards[].Pos", impl=list(impl), recent=list(recent), held=set(held))


class TestFocusBlame(unittest.TestCase):
    def setUp(self):
        self.mod = load_module()

    def test_focus_card_answers_its_pool_before_a_bystander(self):
        # The game implicates the focus card first.
        g = game("ev-cmdr-focus", {"Focus Card", "Innocent", "Other"}, impl=["Focus Card", "Innocent", "Other"])
        # The precondition of the rule: without the focus list every uncleared card answers.
        self.assertEqual(self.mod.blame([dict(g)], set())[0]["cards"], {"Focus Card", "Innocent", "Other"})
        blamed = self.mod.blame([dict(g)], set(), {"ev-cmdr-focus": {"Focus Card"}})
        self.assertEqual(blamed[0]["cards"], {"Focus Card"})

    def test_focus_card_answers_from_recent_when_nothing_is_implicated(self):
        g = game("ev-cmdr-focus", {"Focus Card", "Innocent"}, recent=["Focus Card", "Innocent"])
        blamed = self.mod.blame([dict(g)], set(), {"ev-cmdr-focus": {"Focus Card"}})
        self.assertEqual(blamed[0]["cards"], {"Focus Card"})

    def test_focus_card_held_but_not_implicated_first_does_not_answer(self):
        # The game holds the focus card, but implicates a bystander first: the hand rule of
        # round cs6, which left Braids, Dragonhawk and four more unverified, not refused.
        g = game("ev-cmdr-focus", {"Focus Card", "Innocent"}, impl=["Innocent", "Focus Card"], recent=["Focus Card"])
        blamed = self.mod.blame([dict(g)], set(), {"ev-cmdr-focus": {"Focus Card"}})
        self.assertEqual(blamed[0]["cards"], {"Innocent", "Focus Card"})
        g = game("ev-cmdr-focus", {"Focus Card", "Innocent"}, impl=["Innocent"], recent=["Focus Card"])
        blamed = self.mod.blame([dict(g)], set(), {"ev-cmdr-focus": {"Focus Card"}})
        self.assertEqual(blamed[0]["cards"], {"Innocent"})

    def test_focus_list_does_not_reach_another_pool(self):
        g = game("hosted-cmdr", {"Focus Card", "Innocent"}, impl=["Innocent"])
        blamed = self.mod.blame([dict(g)], set(), {"ev-cmdr-focus": {"Focus Card"}})
        self.assertEqual(blamed[0]["cards"], {"Innocent"})

    def test_a_focus_card_not_in_the_game_does_not_answer_it(self):
        g = game("ev-cmdr-focus", {"Innocent"}, impl=["Innocent"])
        blamed = self.mod.blame([dict(g)], set(), {"ev-cmdr-focus": {"Focus Card"}})
        self.assertEqual(blamed[0]["cards"], {"Innocent"})

    def test_focus_file_names_its_card_before_the_enablers(self):
        with tempfile.TemporaryDirectory() as tmp:
            directory = os.path.join(tmp, "dirac/support/focus")
            os.makedirs(directory)
            with open(os.path.join(directory, "ev-cmdr-x.txt"), "w") as f:
                f.write("# a comment\nFocus Card\tEnabler One;Enabler Two\nPartner\n")
            self.assertEqual(self.mod.focus_cards(tmp), {"ev-cmdr-x": {"Focus Card", "Partner"}})


if __name__ == "__main__":
    unittest.main()
