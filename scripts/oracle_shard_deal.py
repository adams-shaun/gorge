#!/usr/bin/env python3
"""oracle_shard_deal.py: the cost-balanced deal of ./dirac/oracle's tests (stdlib only).

The package plays its lockstep games serially, so one `go test` process runs past any sensible wall.
Two callers split it by measured cost, and this is their one deal: scripts/ci-oracle-shard.sh deals the
package into CI's matrix parts, and scripts/test_targets.py deals the precheck's replay slice into
concurrent lanes. Each test, heaviest first (ties by name, byte order), goes to the part carrying the
least so far. The deal is a pure function of the test list and the weights, so every caller computes
the same one.

Weights are dirac/oracle/testdata/ci-shard-seconds.txt ("<test> <seconds>", a test's leaf subtests
summed: its cost, not its wall). A test the table does not list weighs `default`: the table's median
unless the caller passes one. The table lists only tests of >= 1 s, so neither real caller uses the
median: test_targets.py passes this module's UNLISTED_SECONDS and ci-oracle-shard.sh its own
DEFAULT_SECONDS (the same 0.08, the default in that file's header). A name in `zero`
weighs 0: on a clone with no deck pool the pool-only tests (dirac/oracle/testdata/poolskips.txt) skip.

Usage (the parts, as "<part> <seconds> <test>"; part indices are 0-based, as CI's matrix is):
  oracle_shard_deal.py --parts N [--part K] [--weights FILE] [--default SECONDS] [--zero FILE] < tests
"""
import argparse
import sys

DEFAULT_WEIGHTS = "dirac/oracle/testdata/ci-shard-seconds.txt"
DEFAULT_POOLSKIPS = "dirac/oracle/testdata/poolskips.txt"
# What a test the table does not list weighs. The table lists only the tests of >= 1 s, so its median
# (~14 s) is the wrong stand-in for an unlisted one: the precheck's lanes used it and weighed every
# sub-second test like a heavy one (scripts/ci-oracle-shard.sh's DEFAULT_SECONDS is this number too).
UNLISTED_SECONDS = 0.08


def read_weights(path=DEFAULT_WEIGHTS):
    """{test: seconds} from a "<test> <seconds>" table; {} when the file is absent or unreadable."""
    weights = {}
    try:
        with open(path, errors="replace") as f:
            for ln in f:
                if ln.lstrip().startswith("#"):
                    continue
                fields = ln.split()
                if len(fields) != 2:
                    continue
                try:
                    weights[fields[0]] = float(fields[1])
                except ValueError:
                    continue
    except OSError:
        return {}
    return weights


def median_weight(weights):
    """The table's median weight: the conservative default for a test it does not list (1.0 with no table)."""
    vals = sorted(weights.values())
    if not vals:
        return 1.0
    n = len(vals)
    return vals[n // 2] if n % 2 else (vals[n // 2 - 1] + vals[n // 2]) / 2.0


def read_zero(path=DEFAULT_POOLSKIPS):
    """The top-level test names that skip on a clone with no deck pool (a poolskips line's first '/'
    component: the file lists subtest paths too)."""
    names = set()
    try:
        with open(path, errors="replace") as f:
            for ln in f:
                ln = ln.strip()
                if ln and not ln.startswith("#"):
                    names.add(ln.split("/", 1)[0])
    except OSError:
        return set()
    return names


def deal(tests, weights, parts, default=None, zero=frozenset()):
    """[(part, test, seconds)] for every test in `tests`: part is 0-based (CI's matrix), heaviest first,
    each test dealt to the part carrying the least so far (the first such part on a tie, as the shell
    deal always has). A test missing from `weights` weighs `default` (the table's median when None);
    a test in `zero` weighs 0. Every test appears exactly once."""
    if parts < 1:
        raise ValueError("parts must be >= 1")
    if default is None:
        default = median_weight(weights)

    def weigh(name):
        if name in zero:
            return 0.0
        return round(weights.get(name, default), 3)  # the shell deal printed %.3f before summing

    loads = [0.0] * parts
    dealt = []
    for name in sorted(tests, key=lambda n: (-weigh(n), n)):
        secs = weigh(name)
        best = 0
        for k in range(1, parts):
            if loads[k] < loads[best]:
                best = k
        loads[best] += secs
        dealt.append((best, name, secs))
    return dealt


def main(argv=None):
    ap = argparse.ArgumentParser(prog="oracle_shard_deal.py")
    ap.add_argument("--parts", type=int, required=True, help="how many parts to deal into")
    ap.add_argument("--part", type=int, default=None, help="print only this part's tests (0-based)")
    ap.add_argument("--weights", default=DEFAULT_WEIGHTS)
    ap.add_argument("--default", type=float, default=None,
                    help="seconds for a test the table does not list (default: the table's median)")
    ap.add_argument("--zero", default="", help="a poolskips file: its top-level names weigh 0")
    ap.add_argument("--tests", default="-", help="the test list, one name a line (- is stdin)")
    a = ap.parse_args(argv)
    if a.parts < 1:
        ap.error("--parts must be >= 1")
    if a.part is not None and not 0 <= a.part < a.parts:
        ap.error("--part is 0..parts-1")
    if a.tests == "-":
        names = [ln.strip() for ln in sys.stdin if ln.strip()]
    else:
        with open(a.tests, errors="replace") as f:
            names = [ln.strip() for ln in f if ln.strip()]
    weights = read_weights(a.weights)
    zero = read_zero(a.zero) if a.zero else set()
    for part, name, secs in deal(names, weights, a.parts, a.default, zero):
        if a.part is None or part == a.part:
            print("%d %.3f %s" % (part, secs, name))
    return 0


if __name__ == "__main__":
    sys.exit(main())
