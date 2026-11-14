#!/usr/bin/env python3
"""oracle_fasttier.py: build or check the ./dirac/oracle fast tier for dirac-gate.sh step 8b.

The train runs the whole oracle with -v. A top-level test's median total PASS time across those logs
(its own line plus every subtest line) says what it costs; the tests at or under --threshold
(default 0.5 s) form the fast tier. A test that ever skipped is left out: the gate fails a skip, and
a skipped run says nothing about the cost. The list is committed at
dirac/oracle/testdata/fasttier.txt and the landing gate runs exactly it, so an oracle regression is
caught at the unit that caused it instead of the next train.

    python3 scripts/oracle_fasttier.py --logs '/path/to/train-*.oracle.v' > dirac/oracle/testdata/fasttier.txt
    python3 scripts/oracle_fasttier.py --logs '/path/to/train-*.oracle.v' --write dirac/oracle/testdata/fasttier.txt
    python3 scripts/oracle_fasttier.py --check dirac/oracle/testdata/fasttier.txt

The generator needs the train's -v logs: --logs defaults to $GORGE_ORACLE_LOGS, so set
either one. A green promote PR's oracle part logs serve (ci-oracle-shard.sh prints every PASS and
SKIP line), as fetched: `gh api repos/<owner>/<repo>/actions/jobs/<id>/logs > part-<k>.log`. --check needs no logs.

--check asks `go test -list` which tests exist and fails, loudly, for every listed name that
does not; a clean run also proves the list is sorted and has no duplicates. The gate repeats the
existence check itself, since a list is only useful while every name still runs.
"""

import argparse
import glob
import os
import re
import subprocess
import sys
from collections import Counter

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
DEFAULT_LOGS = os.environ.get("GORGE_ORACLE_LOGS", "")
DEFAULT_THRESHOLD = 0.5
PASS_RE = re.compile(r"^\s*--- PASS: (\S+) \(([0-9.]+)s\)$")
SKIP_RE = re.compile(r"^\s*--- SKIP: (\S+)")
# A GitHub job log (the promote PR's oracle parts, scripts/ci-oracle-shard.sh) starts each line with a timestamp.
GH_TS = re.compile(r"^\ufeff?\d{4}-\d\d-\d\dT[\d:.]+Z ")

HEADER = """\
# The oracle fast tier: ./dirac/oracle top-level tests whose median total time (own line plus every
# subtest) in the train's -v logs is 0.5 s or less. scripts/dirac-gate.sh step 8b runs exactly these
# at every landing, capped and fail-fast. A test that ever skipped is absent. Refresh after a full
# train with:
#   python3 scripts/oracle_fasttier.py --logs '/path/to/train-*.oracle.v' --write dirac/oracle/testdata/fasttier.txt
# and check every name still exists with:
#   python3 scripts/oracle_fasttier.py --check dirac/oracle/testdata/fasttier.txt
"""


def median(values):
    ordered = sorted(values)
    n = len(ordered)
    if n % 2:
        return ordered[n // 2]
    return (ordered[n // 2 - 1] + ordered[n // 2]) / 2


def parse_logs(paths):
    """Per log, sum a top-level test's own and subtest PASS durations.

    Returns (observations, ever_skipped): observations maps a top-level test to the total seconds
    of each log it passed, and ever_skipped holds every top-level test with a SKIP line.
    """
    observations = {}
    ever_skipped = set()
    for path in paths:
        with open(path, errors="replace") as f:
            per = {}
            tops = set()
            skipped = set()
            for line in f:
                line = GH_TS.sub("", line)
                m = SKIP_RE.match(line.rstrip())
                if m:
                    skipped.add(m.group(1).split("/")[0])
                    continue
                m = PASS_RE.match(line.rstrip())
                if not m:
                    continue
                name, secs = m.group(1), float(m.group(2))
                top = name.split("/")[0]
                per[top] = per.get(top, 0.0) + secs
                if "/" not in name:
                    tops.add(top)
        for top in tops:
            if top not in skipped:
                observations.setdefault(top, []).append(per[top])
        ever_skipped.update(skipped)
    return observations, ever_skipped


def generated(paths, threshold):
    if not paths:
        sys.exit("oracle_fasttier: no logs matched (pass --logs GLOB)")
    observations, ever_skipped = parse_logs(paths)
    if not observations:
        sys.exit("oracle_fasttier: no '--- PASS: <name> (<s>s)' lines in %d logs" % len(paths))
    times = {
        name: median(durations)
        for name, durations in observations.items()
        if name not in ever_skipped
    }
    names = sorted(name for name, secs in times.items() if secs <= threshold + 1e-9)
    total = sum(times[name] for name in names)
    print(
        "oracle_fasttier: %d of %d top-level tests <= %.2f s including subtests "
        "(%d logs, %.1f test-seconds, %d ever skipped)"
        % (len(names), len(times), threshold, len(paths), total, len(ever_skipped)),
        file=sys.stderr,
    )
    return names


def read_list(path):
    names = []
    with open(path) as f:
        for line in f:
            line = line.strip()
            if line and not line.startswith("#"):
                names.append(line)
    return names


def existing_tests(pkg):
    proc = subprocess.run(
        ["go", "test", "-list", ".*", pkg],
        cwd=ROOT,
        capture_output=True,
        text=True,
    )
    if proc.returncode != 0:
        print(
            "oracle_fasttier: FAIL: `go test -list .* %s` failed:\n%s"
            % (pkg, (proc.stdout + proc.stderr).rstrip()),
            file=sys.stderr,
        )
        return None
    return {line.strip() for line in proc.stdout.splitlines() if line.startswith("Test")}


def check(path, pkg):
    try:
        names = read_list(path)
    except OSError as e:
        print("oracle_fasttier: FAIL: %s" % e, file=sys.stderr)
        return 1
    have = existing_tests(pkg)
    if have is None:
        return 1
    problems = []
    missing = sorted(set(names) - have)
    if missing:
        problems.append(
            "%d name(s) no longer exist in %s:\n  %s"
            % (len(missing), pkg, "\n  ".join(missing))
        )
    duplicates = sorted(name for name, count in Counter(names).items() if count > 1)
    if duplicates:
        problems.append("duplicate name(s): %s" % " ".join(duplicates))
    if names != sorted(names):
        problems.append("names are not sorted")
    if not names:
        problems.append("the list is empty")
    if problems:
        print(
            "oracle_fasttier: FAIL: %s\n%s" % (path, "\n".join(problems)),
            file=sys.stderr,
        )
        return 1
    print("oracle_fasttier: ok %d names in %s" % (len(names), path))
    return 0


def main(argv):
    p = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    p.add_argument(
        "--logs",
        default=DEFAULT_LOGS,
        help="glob of train -v logs (default: $GORGE_ORACLE_LOGS)",
    )
    p.add_argument("--threshold", type=float, default=DEFAULT_THRESHOLD, help="seconds (default: %(default)s)")
    p.add_argument("--write", metavar="FILE", help="write the generated list to FILE")
    p.add_argument("--check", metavar="FILE", help="check that every name in FILE exists")
    p.add_argument("--pkg", default="./dirac/oracle", help="package to list (default: %(default)s)")
    args = p.parse_args(argv)
    if args.write and args.check:
        p.error("--write and --check are mutually exclusive")
    if args.check:
        return check(args.check, args.pkg)
    paths = sorted(glob.glob(args.logs))
    names = generated(paths, args.threshold)
    # A test deleted since the logs were written cannot run, so the refreshed list drops it here.
    # --check and the gate still fail on a committed name that goes away later.
    have = existing_tests(args.pkg)
    if have is None:
        return 1
    dropped = [name for name in names if name not in have]
    if dropped:
        print(
            "oracle_fasttier: dropping %d name(s) no longer in %s: %s"
            % (len(dropped), args.pkg, " ".join(dropped)),
            file=sys.stderr,
        )
    names = [name for name in names if name in have]
    text = HEADER + "".join(name + "\n" for name in names)
    if args.write:
        with open(args.write, "w") as f:
            f.write(text)
        print("oracle_fasttier: wrote %d names to %s" % (len(names), args.write))
    else:
        sys.stdout.write(text)
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
