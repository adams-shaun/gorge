#!/usr/bin/env python3
"""Render `oraclediff status -all -json` output as the docs site's set table.

Reads the JSONL that gorge-compliance's

    go run ./cmd/oraclediff status -all -json -level A -procs 4 > status-A.jsonl

prints (one adopt.SetStatus object per line; any non-JSON line, such as the
ratchet summary the command also prints, is skipped) and writes a markdown
fragment that docs/site/support/sets.md includes through pymdownx.snippets.
The fragment is generated, never committed (.gitignore), and holds only
counts and percentages, never card text.

Per set:
  playable %   = 1 - (unsupported + not in corpus) / cards
  level-A pass = cards - (problem_cards - "no printed list"), i.e. cards with
                 no level-A problem. "no printed list" is a per-set pseudo
                 problem, not a card, so it is not subtracted. For a set
                 without a printed list this is a hard lower bound.
  level-B pass = the same over the level-B status (--level-b), which
                 gorge-compliance prints for the sets with a printed list:

    go run ./cmd/oraclediff status -all -json -level B -procs 4 \\
        -sets "$(ls oracle/compliance/printed | sed 's/\\.json$//' | paste -sd,)" > status-B.jsonl

Stdlib only. Usage:
  scripts/docs_sets.py [status-A.jsonl|-] [--level-b status-B.jsonl]
                       [-o docs/site/support/_generated_sets.md]
                       [--commit SHA] [--date YYYY-MM-DD]
"""

import argparse
import json
import sys
from collections import OrderedDict

NO_PRINTED = "no printed list"
NOT_IN_CORPUS = "not in corpus"
UNSUPPORTED = "unsupported"

DEFAULT_OUT = "docs/site/support/_generated_sets.md"


def load(stream):
    rows = []
    for line in stream:
        line = line.strip()
        if not line.startswith("{"):
            continue
        try:
            r = json.loads(line)
        except ValueError:
            continue
        if isinstance(r, dict) and r.get("set") and isinstance(r.get("cards"), int):
            rows.append(r)
    return rows


def measure(r):
    cards = r["cards"]
    b = r.get("buckets") or {}
    unsupported = int(b.get(UNSUPPORTED, 0))
    missing = int(b.get(NOT_IN_CORPUS, 0))
    problems = int(r.get("problem_cards", 0)) - int(b.get(NO_PRINTED, 0))
    a_pass = min(cards, max(0, cards - max(0, problems)))
    return {
        "cards": cards,
        "unsupported": unsupported,
        "missing": missing,
        "playable": max(0, cards - unsupported - missing),
        "a_pass": a_pass,
    }


def pct(n, d):
    return "n/a" if d <= 0 else "%.1f" % (100.0 * n / d)


def cell(s):
    return str(s).replace("|", "\\|").replace("\n", " ")


def set_type(r):
    return (r.get("set_type") or "UNKNOWN").replace("_", " ").title()


def buckets_text(r):
    b = dict(r.get("buckets") or {})
    b.pop(NO_PRINTED, None)
    if not b:
        return "none"
    return ", ".join("%d %s" % (v, k) for k, v in sorted(b.items(), key=lambda kv: (-kv[1], kv[0])))


def render(rows, commit, date, level_b=None):
    level_b = level_b or {}
    rows = sorted(rows, key=lambda r: (r.get("released") or "", r["set"]), reverse=True)
    out = []
    w = out.append
    w("## Automated measurement")
    w("")
    stamp = []
    if date:
        stamp.append("measured " + date)
    if commit:
        stamp.append("gorge-compliance @ `%s`" % commit[:12])
    w("*Generated from gorge-compliance's `oraclediff status -all -json -level A`%s%s. "
      "Counts and percentages only.*" % (
          " and `-level B`" if level_b else "",
          (", " + ", ".join(stamp)) if stamp else ""))
    w("")
    w("**Playable %** = 1 − (unsupported + not in corpus) / cards. "
      "**Level A %** counts cards with no Level A problem; for a set without a "
      "printed card list it is a lower bound.")
    w("")

    audited = [r for r in rows if r.get("declared") or r.get("printed")]
    if audited:
        w("### Audited sets (full printed card list)")
        w("")
        if level_b:
            w("| Set | Name | Released | Declared | Cards | Playable % | Level A % | Level B % | Level B problems |")
            w("|---|---|---|---|---:|---:|---:|---:|---|")
        else:
            w("| Set | Name | Released | Declared | Cards | Playable % | Level A % | Level A problems |")
            w("|---|---|---|---|---:|---:|---:|---:|")
        for r in audited:
            m = measure(r)
            head = "| %s | %s | %s | %s | %d | %s | %s |" % (
                cell(r["set"]), cell(r.get("name", "")), cell(r.get("released", "")),
                cell(r.get("declared") or "none"), m["cards"], pct(m["playable"], m["cards"]),
                pct(m["a_pass"], m["cards"]))
            b = level_b.get(r["set"])
            if level_b:
                if b is None:
                    w(head + " n/a | not measured |")
                else:
                    mb = measure(b)
                    w(head + " %s | %s |" % (pct(mb["a_pass"], mb["cards"]), cell(buckets_text(b))))
            else:
                w(head + " %d |" % int(r.get("outstanding", 0)))
        w("")

    groups = OrderedDict()
    for r in sorted(rows, key=set_type):
        groups.setdefault(set_type(r), []).append(r)
    w("### Roll-up by set type")
    w("")
    w("Per-set entries: a card reprinted in several sets is counted in each.")
    w("")
    w("| Set type | Sets | Entries | Unsupported | Not in corpus | Playable % | Level A % |")
    w("|---|---:|---:|---:|---:|---:|---:|")
    tot = {"sets": 0, "cards": 0, "unsupported": 0, "missing": 0, "playable": 0, "a_pass": 0}
    for name, rs in groups.items():
        acc = {"cards": 0, "unsupported": 0, "missing": 0, "playable": 0, "a_pass": 0}
        for r in rs:
            for k, v in measure(r).items():
                acc[k] += v
        w("| %s | %d | %d | %d | %d | %s | %s |" % (
            cell(name), len(rs), acc["cards"], acc["unsupported"], acc["missing"],
            pct(acc["playable"], acc["cards"]), pct(acc["a_pass"], acc["cards"])))
        tot["sets"] += len(rs)
        for k in acc:
            tot[k] += acc[k]
    w("| **All** | **%d** | **%d** | **%d** | **%d** | **%s** | **%s** |" % (
        tot["sets"], tot["cards"], tot["unsupported"], tot["missing"],
        pct(tot["playable"], tot["cards"]), pct(tot["a_pass"], tot["cards"])))
    w("")

    w("??? note \"Every set (%d), newest first\"" % len(rows))
    w("")
    w("    | Set | Name | Type | Released | Cards | Unsupported | Not in corpus | Playable % | Level A % |")
    w("    |---|---|---|---|---:|---:|---:|---:|---:|")
    for r in rows:
        m = measure(r)
        w("    | %s | %s | %s | %s | %d | %d | %d | %s | %s |" % (
            cell(r["set"]), cell(r.get("name", "")), cell(set_type(r)), cell(r.get("released", "")),
            m["cards"], m["unsupported"], m["missing"], pct(m["playable"], m["cards"]),
            pct(m["a_pass"], m["cards"])))
    w("")
    return "\n".join(out) + "\n"


def main(argv=None):
    ap = argparse.ArgumentParser(description=__doc__.split("\n\n")[0])
    ap.add_argument("input", nargs="?", default="-", help="status JSONL (default: stdin)")
    ap.add_argument("-o", "--out", default=DEFAULT_OUT, help="fragment path (default: %(default)s)")
    ap.add_argument("--commit", default="", help="measured commit, shown in the fragment")
    ap.add_argument("--date", default="", help="measurement date, shown in the fragment")
    ap.add_argument("--level-b", default="", help="level-B status JSONL (status -all -json -level B -sets ...)")
    a = ap.parse_args(argv)

    if a.input == "-":
        rows = load(sys.stdin)
    else:
        with open(a.input, encoding="utf-8") as f:
            rows = load(f)
    if not rows:
        print("docs_sets: no set rows in %s; fragment not written" % a.input, file=sys.stderr)
        return 1
    level_b = {}
    if a.level_b:
        with open(a.level_b, encoding="utf-8") as f:
            level_b = {r["set"]: r for r in load(f)}
    with open(a.out, "w", encoding="utf-8") as f:
        f.write(render(rows, a.commit, a.date, level_b))
    print("docs_sets: wrote %d sets to %s" % (len(rows), a.out), file=sys.stderr)
    return 0


if __name__ == "__main__":
    sys.exit(main())
