#!/usr/bin/env python3
"""Flag parity between scripts/instr-measure.sh and dirac-gate.sh step 5.

The merge queue measures dirac's transition benches with dirac-gate's own step-5
method, copied into scripts/instr-measure.sh so the protected gate is not edited.
This test reads the method back out of both scripts, so a change to either one
that the other does not get is red.

    python3 -I -m unittest discover -s scripts -p 'instr_measure_test.py'
"""

import os
import re
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
GATE = os.path.join(HERE, "dirac-gate.sh")
MEASURE = os.path.join(HERE, "instr-measure.sh")


def read(path):
    with open(path) as f:
        return f.read()


def function_body(path, name):
    """The stripped, non-blank body lines of a bash `name() { ... }` function."""
    text = read(path)
    m = re.search(
        r"^[ \t]*%s\(\)[ \t]*\{\n(.*?)^[ \t]*\}" % re.escape(name),
        text,
        re.M | re.S,
    )
    assert m, "%s has no %s() function" % (path, name)
    return [line.strip() for line in m.group(1).splitlines() if line.strip()]


class FlagParity(unittest.TestCase):
    def test_instr_copies_step5s_command(self):
        gate = function_body(GATE, "instr")
        # Precondition: the gate's function really is the perf-stat measurement.
        self.assertTrue(
            any("perf stat -x, -e instructions:u" in line for line in gate),
            "dirac-gate.sh step 5 no longer runs perf stat: update this test",
        )
        self.assertEqual(function_body(MEASURE, "instr"), gate)

    def test_per_op_copies_step5s_counts_and_divisor(self):
        gate = function_body(GATE, "per_op")
        joined = " ".join(gate)
        # Precondition: the two bench times and the 400000 delta live in step 5.
        for token in ("100000", "500000", "/ 400000"):
            self.assertIn(token, joined)
        self.assertEqual(function_body(MEASURE, "per_op"), gate)

    def test_step5s_named_flags_are_present(self):
        gate = " ".join(function_body(GATE, "instr"))
        for flag in (
            "GOGC=off",
            "GOMAXPROCS=1",
            "taskset -c",
            "perf stat -x,",
            "-e instructions:u",
            '-test.run XXX',
            '-test.bench "^$3\\$"',
        ):
            self.assertIn(flag, gate)
        self.assertEqual(function_body(MEASURE, "instr"), function_body(GATE, "instr"))


if __name__ == "__main__":
    unittest.main()
