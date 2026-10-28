#!/usr/bin/env python3
"""Render the docs site's card-support tables, by engine and by set.

Two sources, joined by set code:

1. gorge-compliance's per-set status (gorge: playable, certified Level A and
   Level B). Its nightly CI publishes the output of

       go run ./cmd/oraclediff status -all -json -level A -procs 4 > status-A.jsonl
       go run ./cmd/oraclediff status -all -json -level B -procs 4 \\
           -sets <every set whose Level A row has "printed": true> > status-B.jsonl

   with a meta.json ({"commit", "gorge_pin", "forge_ref", "date"}) to its
   orphan docs-data branch. Each line is one adopt.SetStatus object; any
   non-JSON line (the ratchet summary the command also prints) is skipped.

2. dirty-dirac's per-set acceptance, a committed snapshot
   (docs/site/support/dirty-dirac-sets.json). dirty-dirac's source is
   private, so a maintainer refreshes that file by hand from its acceptance
   ratchet and lockstep runs.

Either source may be missing; the tables then say so. The output is one
markdown fragment, docs/site/support/_generated_sets.md (gitignored, never
committed), holding only counts and percentages, never card text. sets.md
includes its sections through pymdownx.snippets:

    headline  one row per engine and measure
    sets      the by-engine table for every set either source audits
    rollup    gorge-compliance's roll-up by set type, then every set

Per set:
  playable %   = 1 - (unsupported + not in corpus) / cards
  Level A %    = cards with no Level A problem / cards. "no printed list" is a
                 per-set pseudo problem, not a card, so it is not subtracted.
                 For a set without a printed list this is a hard lower bound.
  Level B %    = the same over the Level B status.
  dirty-dirac  = accepted / cards from the snapshot, over dirty-dirac's own
                 card list for the set (distinct card names in XMage's
                 manifest that exist in the corpus), which can differ by a
                 few cards from gorge-compliance's count.

Stdlib only. Usage:
  scripts/docs_sets.py [status-A.jsonl|-] [--level-b status-B.jsonl]
                       [--meta meta.json] [--dirty-dirac dirty-dirac-sets.json]
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
DASH = "—"


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


def load_dirty_dirac(path):
    """Read and check the dirty-dirac snapshot; raise ValueError on a bad file."""
    with open(path, encoding="utf-8") as f:
        d = json.load(f)
    for k in ("date", "corpus", "sets"):
        if k not in d:
            raise ValueError("%s: missing %r" % (path, k))
    for code, s in list(d["sets"].items()) + [("corpus", d["corpus"])]:
        a, n = s.get("accepted"), s.get("cards")
        if not (isinstance(a, int) and isinstance(n, int) and 0 <= a <= n and n > 0):
            raise ValueError("%s: %s: need 0 <= accepted <= cards, cards > 0" % (path, code))
    for code, s in (d.get("lockstep") or {}).items():
        if not all(isinstance(s.get(k), int) for k in ("games", "agree", "stalls", "disagree")):
            raise ValueError("%s: lockstep %s: need integer games/agree/stalls/disagree" % (path, code))
        if s["agree"] + s["stalls"] + s["disagree"] != s["games"]:
            raise ValueError("%s: lockstep %s: agree + stalls + disagree != games" % (path, code))
    return d


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


def num(n):
    return "{:,}".format(n)


def cell(s):
    return str(s).replace("|", "\\|").replace("\n", " ")


def set_type(r):
    return (r.get("set_type") or "UNKNOWN").replace("_", " ").title()


def audited(r):
    return bool(r.get("declared") or r.get("printed"))


def buckets_text(r):
    b = dict(r.get("buckets") or {})
    b.pop(NO_PRINTED, None)
    if not b:
        return "none"
    return ", ".join("%d %s" % (v, k) for k, v in sorted(b.items(), key=lambda kv: (-kv[1], kv[0])))


def dd_cell(dd, code):
    s = (dd or {}).get("sets", {}).get(code)
    if not s:
        return DASH
    return "%d / %d (%s%%)" % (s["accepted"], s["cards"], pct(s["accepted"], s["cards"]))


def lockstep_cell(dd, code):
    s = (dd or {}).get("lockstep", {}).get(code)
    if not s:
        return DASH
    text = "%s / %s" % (num(s["agree"]), num(s["games"]))
    extra = []
    if s["stalls"]:
        extra.append("%d stalled" % s["stalls"])
    if s["disagree"]:
        extra.append("%d disagree" % s["disagree"])
    return text + (" (%s)" % ", ".join(extra) if extra else "")


def section(name, body):
    return "<!-- --8<-- [start:%s] -->\n%s<!-- --8<-- [end:%s] -->\n" % (name, body, name)


def stamp_text(meta, level_b):
    if not meta:
        return ""
    parts = []
    if meta.get("date"):
        parts.append("measured " + meta["date"])
    if meta.get("commit"):
        parts.append("gorge-compliance @ `%s`" % meta["commit"][:9])
    if meta.get("gorge_pin"):
        parts.append("measuring gorge `%s`" % meta["gorge_pin"][:9])
    return "gorge-compliance's `oraclediff status -all -json -level A`%s, %s" % (
        " and `-level B`" if level_b else "", ", ".join(parts))


def dd_stamp(dd):
    # The private module's commit is never published (privacy rule); the date
    # identifies the snapshot.
    return "the dirty-dirac snapshot of %s" % dd["date"]


def render_headline(rows, level_b, meta, dd):
    out = []
    w = out.append
    w("| Engine | Measure | Figure |")
    w("|---|---|---|")
    if rows:
        aud = [r for r in rows if audited(r)]
        decl_a = [r for r in rows if r.get("declared") in ("A", "B")]
        decl_b = [r for r in rows if r.get("declared") == "B"]
        w("| gorge | sets with a printed card list (audited) | **%d** |" % len(aud))
        if decl_a:
            cards = sum(r["cards"] for r in decl_a)
            passed = sum(measure(r)["a_pass"] for r in decl_a)
            w("| gorge | certified **Level A** or better | **%d** sets declared; %s of their %s set entries pass (%s%%) |" % (
                len(decl_a), num(passed), num(cards), pct(passed, cards)))
        if level_b:
            # Over the same sets as the Level A row, so the two compare.
            bs = [level_b[r["set"]] for r in decl_a if r["set"] in level_b]
            cards = sum(b["cards"] for b in bs)
            passed = sum(measure(b)["a_pass"] for b in bs)
            w("| gorge | certified **Level B** | **%d** sets declared; over those %d sets, %s of %s entries pass (%s%%) |" % (
                len(decl_b), len(bs), num(passed), num(cards), pct(passed, cards)))
        tot = {"cards": 0, "playable": 0}
        for r in rows:
            m = measure(r)
            tot["cards"] += m["cards"]
            tot["playable"] += m["playable"]
        w("| gorge | playable, every printing in all %d XMage set manifests | %s of %s entries (%s%%) |" % (
            len(rows), num(tot["playable"]), num(tot["cards"]), pct(tot["playable"], tot["cards"])))
    else:
        w("| gorge | certified Level A / B | not available to this build (gorge-compliance's docs-data was not fetched) |")
    if dd:
        c = dd["corpus"]
        w("| dirty-dirac | lowered, whole corpus | **%s of %s** cards (%s%%) |" % (
            num(c["accepted"]), num(c["cards"]), pct(c["accepted"], c["cards"])))
        w("| dirty-dirac | sets in its acceptance ratchet | **%d** |" % len(dd["sets"]))
        ls = dd.get("lockstep") or {}
        if ls:
            parts = []
            for code in sorted(ls):
                s = ls[code]
                parts.append("%s: %s of %s games agree, %d stalled, %d disagree" % (
                    code, num(s["agree"]), num(s["games"]), s["stalls"], s["disagree"]))
            w("| dirty-dirac | lockstep-verified against gorge | **%s** (%s) |" % (
                ", ".join(sorted(ls)), "; ".join(parts)))
    else:
        w("| dirty-dirac | lowered | not available to this build |")
    w("")
    notes = []
    if rows and meta:
        notes.append(stamp_text(meta, level_b))
    if dd:
        notes.append(dd_stamp(dd))
    if notes:
        w("*Generated at build time from " + "; ".join(notes) + ".*")
        w("")
    return "\n".join(out) + "\n"


def render_sets(rows, level_b, meta, dd):
    by = {r["set"]: r for r in rows}
    dd_sets = (dd or {}).get("sets", {})
    codes = {c for c, r in by.items() if audited(r)} | set(dd_sets)
    known = sorted((c for c in codes if c in by),
                   key=lambda c: (by[c].get("released") or "", c), reverse=True)
    unknown = sorted(c for c in codes if c not in by)
    out = []
    w = out.append
    w("| Set | Name | Declared | Cards | Playable % | Level A % | Level B % | dirty-dirac lowered | Lockstep games |")
    w("|---|---|---|---:|---:|---:|---:|---:|---:|")
    problems = []
    for c in known:
        r = by[c]
        m = measure(r)
        if audited(r):
            a = pct(m["a_pass"], m["cards"])
        else:
            a = "≥ " + pct(m["a_pass"], m["cards"])
        b = level_b.get(c) if level_b else None
        if b is not None:
            bm = measure(b)
            bcell = pct(bm["a_pass"], bm["cards"])
            if buckets_text(b) != "none":
                problems.append("%s: %s" % (c, buckets_text(b)))
        else:
            bcell = DASH
        w("| %s | %s | %s | %d | %s | %s | %s | %s | %s |" % (
            cell(c), cell(r.get("name", "")), cell(r.get("declared") or DASH), m["cards"],
            pct(m["playable"], m["cards"]), a, bcell, dd_cell(dd, c), lockstep_cell(dd, c)))
    for c in unknown:
        w("| %s | | %s | %s | %s | %s | %s | %s | %s |" % (
            cell(c), DASH, DASH, DASH, DASH, DASH, dd_cell(dd, c), lockstep_cell(dd, c)))
    w("")
    if not rows:
        w("*gorge-compliance's status was not available to this build, so the gorge columns are empty.*")
        w("")
    if problems:
        w("**Level B problems:** " + "; ".join(problems) + ".")
        w("")
    return "\n".join(out) + "\n"


def render_rollup(rows, level_b, meta, dd):
    out = []
    w = out.append
    if not rows:
        w("*gorge-compliance's status was not available to this build.*")
        w("")
        return "\n".join(out) + "\n"
    rows = sorted(rows, key=lambda r: (r.get("released") or "", r["set"]), reverse=True)
    groups = OrderedDict()
    for r in sorted(rows, key=set_type):
        groups.setdefault(set_type(r), []).append(r)
    w("A card reprinted in several sets is counted once per set.")
    w("")
    w("| Set type | Sets | Entries | Unsupported | Not in corpus | Playable % | Level A % |")
    w("|---|---:|---:|---:|---:|---:|---:|")
    tot = {"sets": 0, "cards": 0, "unsupported": 0, "missing": 0, "playable": 0, "a_pass": 0}
    for name, rs in groups.items():
        acc = {"cards": 0, "unsupported": 0, "missing": 0, "playable": 0, "a_pass": 0}
        for r in rs:
            for k, v in measure(r).items():
                acc[k] += v
        w("| %s | %d | %s | %s | %s | %s | %s |" % (
            cell(name), len(rs), num(acc["cards"]), num(acc["unsupported"]), num(acc["missing"]),
            pct(acc["playable"], acc["cards"]), pct(acc["a_pass"], acc["cards"])))
        tot["sets"] += len(rs)
        for k in acc:
            tot[k] += acc[k]
    w("| **All** | **%d** | **%s** | **%s** | **%s** | **%s** | **%s** |" % (
        tot["sets"], num(tot["cards"]), num(tot["unsupported"]), num(tot["missing"]),
        pct(tot["playable"], tot["cards"]), pct(tot["a_pass"], tot["cards"])))
    w("")
    w("??? note \"Every set (%d), newest first\"" % len(rows))
    w("")
    w("    | Set | Name | Type | Released | Cards | Unsupported | Not in corpus | Playable % | Level A % | dirty-dirac lowered |")
    w("    |---|---|---|---|---:|---:|---:|---:|---:|---:|")
    for r in rows:
        m = measure(r)
        w("    | %s | %s | %s | %s | %d | %d | %d | %s | %s | %s |" % (
            cell(r["set"]), cell(r.get("name", "")), cell(set_type(r)), cell(r.get("released", "")),
            m["cards"], m["unsupported"], m["missing"], pct(m["playable"], m["cards"]),
            pct(m["a_pass"], m["cards"]) if audited(r) else "≥ " + pct(m["a_pass"], m["cards"]),
            dd_cell(dd, r["set"])))
    w("")
    return "\n".join(out) + "\n"


def render(rows, meta=None, level_b=None, dd=None):
    level_b = level_b or {}
    return "".join([
        "<!-- Generated by scripts/docs_sets.py; never edit or commit. -->\n",
        section("headline", render_headline(rows, level_b, meta, dd)),
        section("sets", render_sets(rows, level_b, meta, dd)),
        section("rollup", render_rollup(rows, level_b, meta, dd)),
    ])


def main(argv=None):
    ap = argparse.ArgumentParser(description=__doc__.split("\n\n")[0])
    ap.add_argument("input", nargs="?", default=None, help="Level A status JSONL ('-' for stdin)")
    ap.add_argument("-o", "--out", default=DEFAULT_OUT, help="fragment path (default: %(default)s)")
    ap.add_argument("--level-b", default="", help="Level B status JSONL")
    ap.add_argument("--meta", default="", help="docs-data meta.json (commit, gorge_pin, date)")
    ap.add_argument("--commit", default="", help="gorge-compliance commit (overrides --meta)")
    ap.add_argument("--date", default="", help="measurement date (overrides --meta)")
    ap.add_argument("--dirty-dirac", default="", help="dirty-dirac per-set snapshot JSON")
    a = ap.parse_args(argv)

    rows = []
    if a.input == "-":
        rows = load(sys.stdin)
    elif a.input:
        with open(a.input, encoding="utf-8") as f:
            rows = load(f)
    if a.input and not rows:
        print("docs_sets: no set rows in %s" % a.input, file=sys.stderr)
    level_b = {}
    if a.level_b:
        with open(a.level_b, encoding="utf-8") as f:
            level_b = {r["set"]: r for r in load(f)}
    meta = {}
    if a.meta:
        with open(a.meta, encoding="utf-8") as f:
            meta = json.load(f)
    if a.commit:
        meta["commit"] = a.commit
    if a.date:
        meta["date"] = a.date
    dd = load_dirty_dirac(a.dirty_dirac) if a.dirty_dirac else None
    if not rows and not dd:
        print("docs_sets: neither status rows nor a dirty-dirac snapshot; fragment not written", file=sys.stderr)
        return 1
    with open(a.out, "w", encoding="utf-8") as f:
        f.write(render(rows, meta, level_b, dd))
    print("docs_sets: wrote %d status sets, %d dirty-dirac sets to %s" % (
        len(rows), len((dd or {}).get("sets", {})), a.out), file=sys.stderr)
    return 0


if __name__ == "__main__":
    sys.exit(main())
