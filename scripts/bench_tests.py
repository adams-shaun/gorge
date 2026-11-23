#!/usr/bin/env python3
"""bench_tests.py: the pure parts of scripts/bench-tests.sh (per-package and per-test cost under the test budget).

The shell wrapper owns the systemd scope, the cgroup probes and running `go test`; everything that parses a
`go test -json` stream or a cgroup file, and everything that decides the shard split, lives here so
scripts/bench_tests_test.py can pin it. Stdlib only.

Output is TSV, one header and then these rows:

    kind	pkg	test	elapsed_s	wall_s	cpu_s	peak_rss_mb	tests	cached	reason
    pkg	<pkg>	-	-	<wall>	<cpu>	<rss>	<tests>	<0|1>	-
    test	<pkg>	<test>	<seconds>	-	-	-	-	-	-
    over	<pkg>	<test>	<seconds>	-	-	-	-	-	test_over_60s
    over	<pkg>	-	-	<wall>	<cpu>	<rss>	<tests>	<0|1>	rss_over_4g|timeout

A pkg row is one package run; a test row is a top-level test (a parent already includes its subtests), and an
over row names a test over the one-minute wall budget, a package over 4 GiB, or a package timeout. `cached` is
0 because the wrapper passes -count=1; a 1 means the flag was lost.

Exit codes: 0 measured, 1 a package failed, 2 usage, 3 an environment skip (the corpus or pools were missing),
5 a package timed out or went over 4 GiB. The ENV skip rule is scripts/test_targets.py's ENV_SKIP, so the two
wrappers cannot drift.
"""
from __future__ import annotations

import argparse
import json
import os
import re
import sys
from dataclasses import dataclass, field
from typing import Iterable, Optional

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from test_targets import ENV_SKIP  # noqa: E402  (the one ENV rule, shared with test-targets.sh)

TEST_BUDGET_S = 60.0
RSS_BUDGET_BYTES = 4 * 1024 * 1024 * 1024
HEADER = ["kind", "pkg", "test", "elapsed_s", "wall_s", "cpu_s", "peak_rss_mb", "tests", "cached", "reason"]

# The same signals test_targets.py treats as a budget failure: go's own timeout panic and a kill.
TIMEOUT_RE = re.compile(r"panic: test timed out|test timed out after|signal: (?:killed|terminated)")
_SHARD_RE = re.compile(r"^(\d+)/(\d+)$")


def parse_cpu_stat(text: str) -> float:
    """Seconds of CPU from a cgroup cpu.stat. Raises ValueError when the file is not one."""
    for line in text.splitlines():
        fields = line.split()
        if len(fields) == 2 and fields[0] == "usage_usec":
            try:
                return int(fields[1]) / 1_000_000.0
            except ValueError:
                break
    raise ValueError("cpu.stat has no usage_usec line")


def parse_memory_peak(text: str) -> int:
    """Bytes from a cgroup memory.peak (or memory.max_usage_in_bytes). Raises ValueError when unusable."""
    value = text.strip()
    if not value:
        raise ValueError("empty memory peak")
    try:
        peak = int(value)
    except ValueError:
        raise ValueError("memory peak is not a byte count: %r" % value) from None
    if peak < 0:
        raise ValueError("memory peak is negative: %r" % value)
    return peak


def peak_mb(peak_bytes: int) -> float:
    return peak_bytes / (1024.0 * 1024.0)


def parse_go_list(text: str) -> list[str]:
    """The non-blank, non-comment lines of `go list -f '{{.ImportPath}}'` output."""
    return [ln.strip() for ln in text.splitlines() if ln.strip() and not ln.lstrip().startswith("#")]


def parse_shard(spec: str) -> tuple[int, int]:
    """`i/n`, 1-based and in range. Raises ValueError with the accepted form."""
    m = _SHARD_RE.fullmatch((spec or "").strip())
    if not m:
        raise ValueError("SHARD must be i/n (e.g. 1/3), got %r" % spec)
    i, n = int(m.group(1)), int(m.group(2))
    if n < 1 or not 1 <= i <= n:
        raise ValueError("SHARD must have 1 <= i <= n and n >= 1, got %r" % spec)
    return i, n


def shard(pkgs: Iterable[str], i: int, n: int) -> list[str]:
    """The i'th of n shards of the sorted, deduplicated package list: every package in exactly one shard."""
    if n < 1 or not 1 <= i <= n:
        raise ValueError("need 1 <= i <= n and n >= 1, got %d/%d" % (i, n))
    unique = sorted(set(pkgs))
    return [pkg for idx, pkg in enumerate(unique) if idx % n == i - 1]


@dataclass(frozen=True)
class TestResult:
    name: str
    elapsed: float
    action: str


@dataclass
class Parsed:
    tests: list[TestResult] = field(default_factory=list)
    skipped: list[tuple[str, str, str]] = field(default_factory=list)  # (pkg, test, message)
    cached: bool = False
    failed: bool = False
    env: bool = False
    timeout: bool = False


def parse_events(lines: Iterable[str]) -> Parsed:
    """Parse one package's `go test -json` stream. Top-level tests only: a parent's Elapsed already includes
    its subtests, and measuring both would count the same seconds twice. Non-JSON lines (go's build noise) are
    ignored; a non-zero rc is the caller's failure signal for them."""
    parsed = Parsed()
    outputs: dict[tuple[str, str], list[str]] = {}
    seen: dict[tuple[str, str], TestResult] = {}
    for raw in lines:
        raw = raw.rstrip("\n")
        if not raw.strip():
            continue
        try:
            ev = json.loads(raw)
        except ValueError:
            continue
        if not isinstance(ev, dict):
            continue
        action = ev.get("Action", "")
        pkg = ev.get("Package", "")
        test = ev.get("Test", "")
        text = ev.get("Output", "")
        if action in ("output", "build-output"):
            outputs.setdefault((pkg, test), []).append(text.rstrip("\n"))
            if TIMEOUT_RE.search(text):
                parsed.timeout = True
            if not test and text.startswith("ok ") and "(cached)" in text:
                parsed.cached = True
            continue
        if action in ("pass", "fail") and test:
            if "/" in test:  # a subtest; its parent's Elapsed is the budget row
                continue
            elapsed = ev.get("Elapsed")
            if not isinstance(elapsed, (int, float)):
                elapsed = 0.0
            seen[(pkg, test)] = TestResult(test, float(elapsed), action)
            if action == "fail":
                parsed.failed = True
            continue
        if action in ("fail", "build-fail") and not test:
            parsed.failed = True
            continue
        if action == "skip" and test:
            message = " ".join(outputs.get((pkg, test), []))
            parsed.skipped.append((pkg, test, message))
            if ENV_SKIP.search(message):
                parsed.env = True
    parsed.tests = sorted(seen.values(), key=lambda t: t.name)
    return parsed


@dataclass
class Summary:
    pkg: str
    wall_s: float
    cpu_s: Optional[float]
    peak_bytes: Optional[int]
    parsed: Parsed
    rc: int
    rss_over: bool
    timeout: bool

    @property
    def env(self) -> bool:
        return self.parsed.env

    def peak_rss_mb(self) -> Optional[float]:
        return None if self.peak_bytes is None else peak_mb(self.peak_bytes)

    def exit_code(self) -> int:
        # A timeout or an over-RSS package is a budget failure; a real test failure outranks an environment
        # skip (the same precedence test_targets.py uses).
        if self.timeout or self.rss_over:
            return 5
        if self.parsed.failed or self.rc != 0:
            return 1
        if self.parsed.env:
            return 3
        return 0

    def rows(self) -> list[str]:
        cpu = "-" if self.cpu_s is None else "%.2f" % self.cpu_s
        rss = "-" if self.peak_bytes is None else "%.1f" % peak_mb(self.peak_bytes)
        cached = "1" if self.parsed.cached else "0"
        count = str(len(self.parsed.tests))
        out = [["pkg", self.pkg, "-", "-", "%.1f" % self.wall_s, cpu, rss, count, cached, "-"]]
        for test in self.parsed.tests:
            out.append(["test", self.pkg, test.name, "%.2f" % test.elapsed, "-", "-", "-", "-", "-", "-"])
        for test in sorted(self.parsed.tests, key=lambda t: (-t.elapsed, t.name)):
            if test.elapsed > TEST_BUDGET_S:
                out.append(["over", self.pkg, test.name, "%.1f" % test.elapsed,
                            "-", "-", "-", "-", "-", "test_over_60s"])
        if self.timeout:
            out.append(["over", self.pkg, "-", "-", "%.1f" % self.wall_s, cpu, rss, count, cached, "timeout"])
        elif self.rss_over:
            out.append(["over", self.pkg, "-", "-", "%.1f" % self.wall_s, cpu, rss, count, cached, "rss_over_4g"])
        return ["\t".join(row) for row in out]


def summarize(pkg: str, lines: Iterable[str], cpu_stat: str = "", memory_peak: str = "",
              wall_s: float = 0.0, rc: int = 0) -> Summary:
    """Measure one package from its -json stream and cgroup probes. Unreadable probes become '-' in the row;
    they never turn a real test result into a crash."""
    parsed = parse_events(lines)
    cpu_s: Optional[float] = None
    if cpu_stat.strip():
        try:
            cpu_s = parse_cpu_stat(cpu_stat)
        except ValueError:
            cpu_s = None
    peak_bytes: Optional[int] = None
    if memory_peak.strip():
        try:
            peak_bytes = parse_memory_peak(memory_peak)
        except ValueError:
            peak_bytes = None
    rss_over = peak_bytes is not None and peak_bytes > RSS_BUDGET_BYTES
    timeout = parsed.timeout or rc == 124
    # A kill at the memory cap can leave memory.peak exactly at 4 GiB rather than over it.
    if rc in (137, -9) and (peak_bytes is None or peak_bytes >= RSS_BUDGET_BYTES):
        rss_over = True
    return Summary(pkg, float(wall_s), cpu_s, peak_bytes, parsed, rc, rss_over, timeout)


def _read(path: str) -> str:
    if not path:
        return ""
    try:
        with open(path, errors="replace") as f:
            return f.read()
    except OSError:
        return ""


def main(argv: Optional[list[str]] = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    sub = parser.add_subparsers(dest="cmd", required=True)
    s = sub.add_parser("summarize", help="measure one package's -json stream and cgroup files")
    s.add_argument("--pkg", required=True)
    s.add_argument("--json", required=True)
    s.add_argument("--cpu-stat", default="")
    s.add_argument("--memory-peak", default="")
    s.add_argument("--wall-ns", default="0")
    s.add_argument("--rc", default="0")
    s.add_argument("--no-header", action="store_true", help="omit the TSV header (the shell prints it once)")
    sh = sub.add_parser("shard", help="select one shard of the package names on stdin")
    sh.add_argument("--spec", required=True)
    sub.add_parser("go-list", help="filter `go list -f {{.ImportPath}}` output to package lines on stdin")
    args = parser.parse_args(argv)

    if args.cmd == "go-list":
        for pkg in parse_go_list(sys.stdin.read()):
            print(pkg)
        return 0

    if args.cmd == "shard":
        try:
            i, n = parse_shard(args.spec)
        except ValueError as exc:
            print("bench-tests: %s" % exc, file=sys.stderr)
            return 2
        for pkg in shard([ln.strip() for ln in sys.stdin if ln.strip()], i, n):
            print(pkg)
        return 0

    try:
        wall_ns = int(args.wall_ns)
        rc = int(args.rc)
    except ValueError:
        print("bench-tests: --wall-ns and --rc must be integers", file=sys.stderr)
        return 2
    summary = summarize(args.pkg, _read(args.json).splitlines(), _read(args.cpu_stat),
                        _read(args.memory_peak), wall_ns / 1_000_000_000.0, rc)
    if not args.no_header:
        print("\t".join(HEADER))
    for row in summary.rows():
        print(row)
    return summary.exit_code()


if __name__ == "__main__":
    raise SystemExit(main())
