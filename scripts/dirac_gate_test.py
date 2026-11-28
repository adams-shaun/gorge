#!/usr/bin/env python3
"""Tests for the step-4 failing-pair selection in dirac-gate.sh. Stdlib only:

    python3 -I -m unittest discover -s scripts -p 'dirac_gate_test.py'

The pattern is read back out of the script (the `failing='...'` line), so a change to the
script's pattern changes what these tests see.
"""

import os
import re
import subprocess
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
SCRIPT = os.path.join(HERE, "dirac-gate.sh")


def failing_pattern():
    with open(SCRIPT) as f:
        m = re.search(r"^\s*failing='(.*)'$", f.read(), re.M)
    assert m, "dirac-gate.sh has no failing='...' line"
    return m.group(1)


def run_step4(log):
    """Run the script's two step-4 greps over log; returns (echoed lines, pair)."""
    sh = (
        'failing="$1"\n'
        'grep -E "$failing|^ *(divergence at decision|options:|implicates:)" "$2" | head -6\n'
        'echo "@@pair"\n'
        'grep -E "$failing" "$2" | sed -nE \'s/^pair ([0-9]+) .*/\\1/p;q\'\n'
    )
    import tempfile

    with tempfile.NamedTemporaryFile("w", suffix=".log", delete=False) as f:
        f.write(log)
        path = f.name
    try:
        out = subprocess.run(
            ["bash", "-c", sh, "bash", failing_pattern(), path],
            capture_output=True, text=True, check=False,
        ).stdout
    finally:
        os.unlink(path)
    echoed, _, pair = out.partition("@@pair\n")
    return echoed.splitlines(), pair.strip()


REFUSED_CARD = 'pair 46 (BLB_top_1_W vs BLB_top_2_U, seed 1): refused-card: card "Baylen, the Haymaker" is not supported'
REFUSED_PAIR = "pair 47 (BLB_top_3_W vs BLB_top_4_U, seed 1): refused-pair: pair has a refused card"
CAPACITY = "pair 48 (BLB_top_5_W vs BLB_top_6_U, seed 1): kernel capacity stall after 12 decisions"
DIV_HEAD = "pair 612 (BLB_top_28298_WBpR vs BLB_top_37648_WU, seed 1) after 7 decisions:"
DIV_BODY = "divergence at decision 7: projection Cards[34].Flags differs"
SKIPS = [REFUSED_CARD, REFUSED_PAIR, CAPACITY]


class FailingPair(unittest.TestCase):
    def test_divergence_after_skips(self):
        log = "\n".join(SKIPS + [DIV_HEAD, DIV_BODY, "blb: 1 divergent"]) + "\n"
        first = re.search(r"^pair ([0-9]+) ", log, re.M).group(1)
        echoed, pair = run_step4(log)
        self.assertEqual(first, "46")  # precondition: the old grep would pick the skip
        self.assertEqual(pair, "612")
        self.assertNotEqual(first, pair)
        self.assertIn(DIV_HEAD, echoed)
        self.assertIn(DIV_BODY, echoed)
        for skip in SKIPS:
            self.assertNotIn(skip, echoed)

    def test_only_skips_gives_no_pair(self):
        echoed, pair = run_step4("\n".join(SKIPS) + "\n")
        self.assertEqual(pair, "")
        self.assertEqual(echoed, [])

    def test_hang_before_divergence(self):
        hang = "pair 9 (A vs B, seed 1): hang: no result after 1m0s"
        _, pair = run_step4("\n".join(SKIPS + [hang, DIV_HEAD, DIV_BODY]) + "\n")
        self.assertEqual(pair, "9")

    def test_unmodelled_stall(self):
        stall = "pair 20 (A vs B, seed 1): kernel unmodelled stall after 3 decisions"
        _, pair = run_step4("\n".join(SKIPS + [stall, DIV_HEAD, DIV_BODY]) + "\n")
        self.assertEqual(pair, "20")

    def test_error_skip(self):
        err = "pair 30 (A vs B, seed 1): error: boom"
        _, pair = run_step4("\n".join(SKIPS + [err, DIV_HEAD, DIV_BODY]) + "\n")
        self.assertEqual(pair, "30")


class Step8(unittest.TestCase):
    """Step 8 runs internal/archtest without its 386 cross-build (coverage audit 2026-11-14, item 6): the
    91 s build moved to the affected side, off the gate's critical path."""

    def test_archtest_runs_without_the_386_build(self):
        with open(SCRIPT) as f:
            lines = [ln for ln in f if re.search(r"\bgo test\b.*\./internal/archtest", ln)]
        self.assertEqual(len(lines), 1, lines)
        self.assertIn("-skip '^TestEngineCompilesFor32Bit$'", lines[0])


class Step0(unittest.TestCase):
    """Step 0 refuses an ordinary change to the batch's EDH4 instruction check or its base: only the lane that
    regenerates pinned files moves scripts/dirac-instr-base.tsv, in a change of its own."""

    def run_gate(self, path):
        import shutil
        import tempfile

        tmp = tempfile.mkdtemp(prefix="gate-step0-")
        try:
            git = lambda *a: subprocess.run(["git", "-C", tmp, *a], capture_output=True, text=True, check=True)
            git("init", "-q")
            os.makedirs(os.path.join(tmp, "scripts"))
            for p in ("scripts/dirac-instr-base.tsv", "scripts/dirac-instr-batch.sh", "README"):
                with open(os.path.join(tmp, p), "w") as f:
                    f.write("one\n")
            git("add", ".")
            git("-c", "user.name=t", "-c", "user.email=t@t", "commit", "-qm", "base")
            base = git("rev-parse", "HEAD").stdout.strip()
            with open(os.path.join(tmp, path), "a") as f:
                f.write("two\n")
            git("-c", "user.name=t", "-c", "user.email=t@t", "commit", "-qam", "change")
            p = subprocess.run(["bash", SCRIPT, base], cwd=tmp, capture_output=True, text=True, check=False, timeout=60)
            return p.returncode, p.stdout + p.stderr
        finally:
            shutil.rmtree(tmp)

    def test_editing_the_instr_base_fails_step0(self):
        rc, out = self.run_gate("scripts/dirac-instr-base.tsv")
        self.assertEqual(rc, 1, out)
        self.assertIn("edits what only the maintainers change:  scripts/dirac-instr-base.tsv", out)

    def test_editing_the_instr_batch_script_fails_step0(self):
        rc, out = self.run_gate("scripts/dirac-instr-batch.sh")
        self.assertEqual(rc, 1, out)
        self.assertIn("scripts/dirac-instr-batch.sh", out)

    def test_an_ordinary_change_passes_step0(self):
        rc, out = self.run_gate("README")
        self.assertIn("== 0 protected", out)
        self.assertNotIn("edits what only the maintainers change", out)
        self.assertIn("== 1 path filter", out)


if __name__ == "__main__":
    unittest.main()
