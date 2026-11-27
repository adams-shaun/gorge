#!/usr/bin/env python3
"""hosted-cmdr is an evidence deck pool: dirac-evidence.sh plays it by default and
dirac-first-refusals.py counts its clean plays as a real deck pool's. Stdlib only:

    python3 scripts/dirac_evidence_hosted_cmdr_test.py
"""

import importlib.util
import os
import re
import tempfile
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))


def load_module():
    spec = importlib.util.spec_from_file_location("dirac_first_refusals", os.path.join(HERE, "dirac-first-refusals.py"))
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


class TestHostedCmdrIsAnEvidencePool(unittest.TestCase):
    def test_deck_pools_hold_it(self):
        self.assertIn("hosted-cmdr", load_module().DECK_POOLS)

    def test_evidence_sh_plays_it_by_default(self):
        with open(os.path.join(HERE, "dirac-evidence.sh")) as f:
            text = f.read()
        default = re.search(r'^\tpools="([^$"]*)', text, re.M)
        self.assertIsNotNone(default, "dirac-evidence.sh has no default pool list")
        self.assertIn("hosted-cmdr", default.group(1).split())

    def test_its_clean_plays_clear_a_card(self):
        """deck_cleared reads hosted-cmdr's coverage like fdn's: a card it plays
        DECK_CLEARED_PLAYS times is cleared, one it plays fewer times is not."""
        mod = load_module()
        with tempfile.TemporaryDirectory() as out:
            with open(os.path.join(out, "hosted-cmdr.full.m0.coverage"), "w") as f:
                f.write("card\tcast\tland\tpairs\twasted\tunused\n")
                f.write(f"Command Tower\t0\t{mod.DECK_CLEARED_PLAYS}\t0\t0\t\n")
                f.write("Path of Ancestry\t0\t1\t0\t0\t\n")
            self.assertEqual(mod.deck_cleared(out), {"Command Tower"})


if __name__ == "__main__":
    unittest.main()
