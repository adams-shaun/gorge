#!/usr/bin/env python3
"""Turn the dirac-slices job's kfuzz output into slices-summary.json.

`make dirac-slices dirac-pods` prints, per run, kfuzz's summary line

    <pool>: N games (R refused-card, P refused-pair, E error), C clean, D divergent, U unmodelled [...], K capacity [...], H hang (lead L, surface S)

and, for a pod run (-seats above 2), the pod line

    <pool> pods (4 seats): N games, C clean, D divergent, K capacity, T twoseat, U unmodelled, H hang

This script reads those lines (kfuzz's format is not changed) from the files
named, or stdin, and writes one row per run:

    {"commit": "<gorge sha>", "date": "YYYY-MM-DD", "slices": [
        {"slice", "surface", "seats", "games", "clean", "divergent", "capacity",
         "twoseat", "unmodelled", "hang", "commit"}]}

A pod line sets the row's seats and moves its twoseat stalls out of capacity (the
verdict line counts them in it). The docs build (scripts/docs_sets.py --slices)
renders the file. Stdlib only.

Usage: scripts/dirac_slices_summary.py [log ...] [-o slices-summary.json] [--commit SHA] [--date YYYY-MM-DD]
"""

import argparse
import datetime
import json
import re
import sys

VERDICT = re.compile(
    r"^(?P<slice>[\w.-]+): (?P<games>\d+) games \(\d+ refused-card, \d+ refused-pair, \d+ error\), "
    r"(?P<clean>\d+) clean, (?P<divergent>\d+) divergent, (?P<unmodelled>\d+) unmodelled \[[^\]]*\], "
    r"(?P<capacity>\d+) capacity \[[^\]]*\], (?P<hang>\d+) hang \(lead [^,)]+, surface (?P<surface>[^)\s]+)\)")
PODS = re.compile(
    r"^(?P<slice>[\w.-]+) pods \((?P<seats>\d+) seats\): (?P<games>\d+) games, (?P<clean>\d+) clean, "
    r"(?P<divergent>\d+) divergent, (?P<capacity>\d+) capacity, (?P<twoseat>\d+) twoseat, "
    r"(?P<unmodelled>\d+) unmodelled, (?P<hang>\d+) hang")
COUNTS = ("games", "clean", "divergent", "capacity", "twoseat", "unmodelled", "hang")


def parse(lines, commit=""):
    """The rows the kfuzz lines hold, in order; a line of any other shape is skipped."""
    rows = []
    for line in lines:
        line = line.strip()
        m = VERDICT.match(line)
        if m:
            r = {"slice": m["slice"], "surface": m["surface"], "seats": 2}
            r.update({k: int(m[k]) for k in COUNTS if k != "twoseat"})
            r["twoseat"] = 0
            r["commit"] = commit
            rows.append(r)
            continue
        m = PODS.match(line)
        if m:
            last = next((r for r in reversed(rows) if r["slice"] == m["slice"]), None)
            if last is None:
                continue
            last["seats"] = int(m["seats"])
            for k in ("capacity", "twoseat"):
                last[k] = int(m[k])
    return rows


def summarize(lines, commit="", date=""):
    return {"commit": commit, "date": date, "slices": parse(lines, commit)}


def main(argv=None):
    ap = argparse.ArgumentParser(description=__doc__.split("\n\n")[0])
    ap.add_argument("logs", nargs="*", help="kfuzz output files (default: stdin)")
    ap.add_argument("-o", "--out", default="slices-summary.json")
    ap.add_argument("--commit", default="", help="the gorge commit the slices ran at")
    ap.add_argument("--date", default="", help="YYYY-MM-DD (default: today, UTC)")
    a = ap.parse_args(argv)
    lines = []
    for p in a.logs:
        with open(p, encoding="utf-8", errors="replace") as f:
            lines += f.read().splitlines()
    if not a.logs:
        lines = sys.stdin.read().splitlines()
    date = a.date or datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%d")
    out = summarize(lines, a.commit, date)
    if not out["slices"]:
        print("dirac_slices_summary: no kfuzz summary line in the input; nothing written", file=sys.stderr)
        return 1
    with open(a.out, "w", encoding="utf-8") as f:
        json.dump(out, f, indent=2)
        f.write("\n")
    print("dirac_slices_summary: wrote %d runs to %s" % (len(out["slices"]), a.out), file=sys.stderr)
    return 0


if __name__ == "__main__":
    sys.exit(main())
