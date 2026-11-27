#!/usr/bin/env python3
"""dirac-xproof-evidence.py credits a card from xproof's lockstep rows only when dirac agreed with
gorge and Forge agreed with gorge's key. Stdlib only:

    python3 -m unittest scripts/dirac_xproof_evidence_test.py

Each test builds a temporary module root (a pool's card names, empty evidence and refusal
directories) and a cycle dir holding a dirac-lockstep.tsv and keyproof-rows.tsv of a few rows.
"""

import importlib.util
import io
import os
import tempfile
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
LOCK_HEADER = "batch\tid\tanswer\tpattern\txmage_class\tforge_class\tfield"
KP_HEADER = "oracle_id\tkey\tdesc\tstate\treason\trefs\tsource\tagree_items\tdiff_items\twitness"
POOL = "pod-xproof"
EVIDENCE = "dirac/support/evidence"


def load_module():
    spec = importlib.util.spec_from_file_location("dirac_xproof_evidence", os.path.join(HERE, "dirac-xproof-evidence.py"))
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


def item(slug, key="f0t0", seed=1):
    return f"xs:XP:{slug}:{key}:s{seed}"


def lock_row(i, answer):
    return f"b00\t{i}\t{answer}\tAGREE3 {answer}\tagree\tagree\t-"


def kp_row(i, state="agreed", refs="FX", oid="o-1", key="f0t0"):
    return f"{oid}\t{key}\t{key}\t{state}\t-\t{refs}\tstart\t2\t0\tx:{i},f:{i}"


class Fixture(unittest.TestCase):
    def setUp(self):
        self.mod = load_module()
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.top = os.path.join(self.tmp.name, "root")
        for d in (EVIDENCE, "dirac/support/evidence-keys", "dirac/support/refused", "dirac/cardset/pools/p"):
            os.makedirs(os.path.join(self.top, d))
        self.write("dirac/cardset/pools/p/cards.txt", "Agatha's Champion\nAbrade\nDismember\nPlains\n")
        self.cycle = os.path.join(self.tmp.name, "pulled", "box", "xproof", "out", "c1")
        os.makedirs(self.cycle)

    def write(self, rel, text):
        with open(os.path.join(self.top, rel), "w") as f:
            f.write(text)

    def cycle_files(self, lock, kp, cycle=None):
        cycle = cycle or self.cycle
        os.makedirs(cycle, exist_ok=True)
        with open(os.path.join(cycle, "dirac-lockstep.tsv"), "w") as f:
            f.write("\n".join([LOCK_HEADER] + lock) + "\n")
        with open(os.path.join(cycle, "keyproof-rows.tsv"), "w") as f:
            f.write("\n".join([KP_HEADER] + kp) + "\n")

    def run_import(self, paths=None, **kw):
        out = io.StringIO()
        credited = self.mod.run(paths or [os.path.dirname(self.cycle)], self.top, out=out, **kw)
        self.out = out.getvalue()
        return credited

    def pool_text(self):
        try:
            with open(os.path.join(self.top, EVIDENCE, POOL + ".txt")) as f:
                return f.read()
        except OSError:
            return None


class TestAgreeAgreeCredits(Fixture):
    def test_row_records_seats_and_source(self):
        i = item("agatha-s-champion")
        self.cycle_files([lock_row(i, "D=G")], [kp_row(i)])
        # Precondition: the pool names the card and nothing evidences it yet.
        self.assertEqual(self.mod.card_slug("Agatha's Champion"), "agatha-s-champion")
        self.assertIsNone(self.pool_text())
        self.assertEqual(self.run_import(), ["Agatha's Champion"])
        self.assertEqual(self.pool_text(), "Agatha's Champion\t2\txproof\n")
        self.assertIn("cards credited: 1", self.out)

    def test_forge_alone_credits_and_seats_are_the_given_count(self):
        i = item("abrade")
        self.cycle_files([lock_row(i, "D=G")], [kp_row(i, refs="F+P")])
        self.assertEqual(self.run_import(seats=4), ["Abrade"])
        self.assertEqual(self.pool_text(), "Abrade\t4\txproof\n")

    def test_credited_card_joins_legacy_and_a_sidecar_row_keeps_it_out(self):
        a, b = item("abrade"), item("dismember")
        self.write("dirac/support/evidence-keys/p.tsv", "card\tlir16\tplays\tkeys\tunused\tbranches\tmissing_branches\nDismember\tx\t1\t1\t\t1\t\n")
        self.cycle_files([lock_row(a, "D=G"), lock_row(b, "D=G")], [kp_row(a), kp_row(b)])
        self.run_import()
        with open(os.path.join(self.top, "dirac/support/evidence-keys/LEGACY.txt")) as f:
            self.assertEqual(f.read(), "Abrade\n")

    def test_the_kgen_reader_rule_is_mirrored(self):
        self.write(f"{EVIDENCE}/a.txt", "Abrade\t2\txproof\nDismember\t2\nPlains\t4\n")
        self.assertEqual(self.mod.evidenced_names(self.mod.load_refusals_module(), self.top), {"Abrade", "Plains"})


class TestForgeDisagreeSkips(Fixture):
    def test_forge_did_not_agree_the_key(self):
        i = item("abrade")
        for state, refs in (("owed", "-"), ("diverged", "F"), ("agreed", "-")):
            with self.subTest(state=state, refs=refs):
                self.cycle_files([lock_row(i, "D=G")], [kp_row(i, state=state, refs=refs)])
                self.assertEqual(self.run_import(), [])
                self.assertIsNone(self.pool_text())
                self.assertIn("forge-not-agreed=1", self.out)


class TestDivergeSkips(Fixture):
    def test_diverge_row_does_not_credit(self):
        i = item("abrade")
        self.cycle_files([lock_row(i, "D!=G")], [kp_row(i)])
        self.assertEqual(self.run_import(), [])
        self.assertIsNone(self.pool_text())
        self.assertIn("diverged Abrade", self.out)

    def test_a_later_diverge_blocks_the_card_and_removes_nothing_kfuzz_credited(self):
        a, b = item("abrade"), item("abrade", key="a0", seed=2)
        self.write(f"{EVIDENCE}/fdn.txt", "Abrade\nDismember\n")
        self.cycle_files([lock_row(a, "D=G")], [kp_row(a)])
        self.cycle_files([lock_row(b, "D!=G")], [kp_row(b, key="a0")], cycle=os.path.join(os.path.dirname(self.cycle), "c2"))
        self.assertEqual(self.run_import(), [])
        self.assertIsNone(self.pool_text())
        with open(os.path.join(self.top, EVIDENCE, "fdn.txt")) as f:
            self.assertEqual(f.read(), "Abrade\nDismember\n")
        self.assertIn("diverged Abrade (an evidence list keeps it)", self.out)

    def test_diverge_in_another_cycle_blocks_an_agree(self):
        a, b = item("dismember"), item("dismember", key="a0", seed=2)
        self.cycle_files([lock_row(a, "D=G")], [kp_row(a)])
        self.cycle_files([lock_row(b, "D!=G")], [kp_row(b, key="a0")], cycle=os.path.join(os.path.dirname(self.cycle), "c2"))
        self.assertEqual(self.run_import(), [])
        # control: the agree alone credits
        self.cycle_files([lock_row(a, "D=G")], [kp_row(a)], cycle=os.path.join(os.path.dirname(self.cycle), "c2"))
        self.assertEqual(self.run_import(), ["Dismember"])


class TestRefusedSkips(Fixture):
    def test_no_answer_row_does_not_credit(self):
        i = item("abrade")
        self.cycle_files([lock_row(i, "D-")], [kp_row(i)])
        self.assertEqual(self.run_import(), [])
        self.assertIsNone(self.pool_text())
        self.assertIn("refused=1", self.out)

    def test_key_without_a_keyproof_row_does_not_credit(self):
        i = item("abrade")
        self.cycle_files([lock_row(i, "D=G")], [kp_row(item("dismember"))])
        self.assertEqual(self.run_import(), [])
        self.assertIn("no-keyproof=1", self.out)

    def test_a_card_refused_by_name_is_not_credited(self):
        i = item("abrade")
        with open(os.path.join(self.top, "dirac/support/refused/abrade.json"), "w") as f:
            f.write('{"card": "Abrade"}')
        self.cycle_files([lock_row(i, "D=G")], [kp_row(i)])
        self.assertEqual(self.run_import(), [])
        self.assertIn("refused-by-name=1", self.out)

    def test_an_xpa_style_log_is_skipped_not_misread(self):
        with open(os.path.join(self.cycle, "dirac-lockstep.tsv"), "w") as f:
            f.write("cycle\toracle_id\tcard\tkey\toutcome\tstep\tkind\tgames\tclean\tpool\tdetail\tgorge_sha\n"
                    "c1\to\tAbrade\tcast\tagree\t-\tkey-used\t1\t1\tp\tAbrade\tabc\n")
        self.assertEqual(self.run_import(), [])
        self.assertIn("1 in a foreign format", self.out)


class TestXmageOnlySkips(Fixture):
    def test_xmage_only_agreement_does_not_credit(self):
        i = item("abrade")
        self.cycle_files([lock_row(i, "D=G")], [kp_row(i, refs="X")])
        self.assertEqual(self.run_import(), [])
        self.assertIsNone(self.pool_text())
        self.assertIn("xmage-only=1", self.out)


class TestReimportIdempotent(Fixture):
    def test_second_run_changes_nothing(self):
        a, b = item("dismember"), item("abrade")
        self.cycle_files([lock_row(a, "D=G"), lock_row(b, "D=G")], [kp_row(a), kp_row(b)])
        self.run_import()
        first = self.pool_text()
        legacy = os.path.join(self.top, "dirac/support/evidence-keys/LEGACY.txt")
        with open(legacy) as f:
            legacy_first = f.read()
        # Precondition: the first run wrote both rows, sorted.
        self.assertEqual(first, "Abrade\t2\txproof\nDismember\t2\txproof\n")
        self.assertEqual(self.run_import(), [])
        self.assertEqual(self.pool_text(), first)
        with open(legacy) as f:
            self.assertEqual(f.read(), legacy_first)
        self.assertIn("already-listed=2", self.out)

    def test_output_is_sorted_whatever_the_input_order(self):
        a, b = item("dismember"), item("abrade")
        self.cycle_files([lock_row(a, "D=G"), lock_row(b, "D=G")], [kp_row(b), kp_row(a)])
        self.run_import()
        one = self.pool_text()
        os.remove(os.path.join(self.top, EVIDENCE, POOL + ".txt"))
        self.cycle_files([lock_row(b, "D=G"), lock_row(a, "D=G")], [kp_row(a), kp_row(b)])
        self.run_import()
        self.assertEqual(self.pool_text(), one)

    def test_a_card_evidenced_elsewhere_is_left_alone(self):
        i = item("abrade")
        self.write(f"{EVIDENCE}/fdn.txt", "Abrade\n")
        self.cycle_files([lock_row(i, "D=G")], [kp_row(i)])
        self.assertEqual(self.run_import(), [])
        self.assertIsNone(self.pool_text())
        self.assertIn("already-evidenced=1", self.out)


if __name__ == "__main__":
    unittest.main()
