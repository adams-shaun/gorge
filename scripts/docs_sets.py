#!/usr/bin/env python3
"""Render the docs site's card-support tables, by engine and by set.

Two sources, joined by set code:

1. gorge-compliance's per-set status (gorge: playable, certified Level A and
   Level B). Its nightly CI publishes the output of

       go run ./cmd/oraclediff status -all -json -level A -procs 4 > status-A.jsonl
       go run ./cmd/oraclediff status -all -json -level B -procs 4 \\
           -sets <every set whose Level A row has "printed": true> > status-B.jsonl

   with a meta.json ({"commit", "gorge_pin", "forge_ref", "date",
   "ratchet_failures"}) to its orphan docs-data branch. Each line is one
   adopt.SetStatus object; any non-JSON line is skipped. The job publishes
   whenever its measurement finishes, so the data can be older than the build
   only when one didn't. Each table's caption gives the data's date and corpus,
   says so when gorge's FORGE_REF has moved past that corpus, and counts the
   ratchet checks that measurement failed. A meta.json without the count reads
   as none.

2. dirac's per-set support, a committed snapshot
   (docs/site/support/dirac-sets.json). Its counts are the table in
   dirac/docs/support.md, which needs the card corpus to generate and so can't
   be generated here. docs_sets_test.py holds the snapshot to that page, and
   --refresh-dirac copies the page into it. Its lockstep rows are read from
   make dirac-slices.

Either source may be missing; the tables then say so. The output is one
markdown fragment, docs/site/support/_generated_sets.md (gitignored, never
committed), holding only counts and percentages, never card text. sets.md
includes its sections through pymdownx.snippets:

    headline  one row per engine and measure
    sets      the by-engine table for every set either source audits
    rollup    gorge-compliance's roll-up by set type, then every set
    charts    HTML bars (gg-bars) per engine over the sets on dirac's support
              page (the Standard sets): dirac supported / unverified / refused,
              and gorge playable / unsupported / not in corpus

Per set:
  playable %   = 1 - (unsupported + not in corpus) / cards
  Level A %    = cards with no Level A problem / cards. "no printed list" is a
                 per-set pseudo problem, not a card, so it is not subtracted.
                 For a set without a printed list this is a hard lower bound.
  Level B %    = the same over the Level B status.
  dirac supported, lowered = counts / cards from the snapshot, over dirac's
                 own card list for the set (distinct card names in XMage's
                 manifest that exist in the corpus), which can differ by a
                 few cards from gorge-compliance's count.

Stdlib only. Usage:
  scripts/docs_sets.py [status-A.jsonl|-] [--level-b status-B.jsonl]
                       [--meta meta.json] [--dirac dirac-sets.json]
                       [-o docs/site/support/_generated_sets.md]
                       [--commit SHA] [--date YYYY-MM-DD]
  scripts/docs_sets.py --refresh-dirac docs/site/support/dirac-sets.json \\
                       --commit SHA --date YYYY-MM-DD
"""

import argparse
import json
import os
import re
import sys
from collections import OrderedDict

NO_PRINTED = "no printed list"
NOT_IN_CORPUS = "not in corpus"
UNSUPPORTED = "unsupported"

DEFAULT_OUT = "docs/site/support/_generated_sets.md"
ROOT = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..")
MAKEFILE = os.path.join(ROOT, "Makefile")
SUPPORT_PAGE = os.path.join(ROOT, "dirac", "docs", "support.md")
SUPPORT_COUNTS = ("cards", "lowered", "refused", "unverified", "supported")
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


def load_dirac(path):
    """Read and check the dirac snapshot; raise ValueError on a bad file."""
    with open(path, encoding="utf-8") as f:
        d = json.load(f)
    for k in ("date", "commit", "corpus", "sets"):
        if k not in d:
            raise ValueError("%s: missing %r" % (path, k))
    for code, s in list(d["sets"].items()) + [("corpus", d["corpus"])]:
        if not all(isinstance(s.get(k), int) and s[k] >= 0 for k in SUPPORT_COUNTS):
            raise ValueError("%s: %s: need integer %s" % (path, code, ", ".join(SUPPORT_COUNTS)))
        if s["refused"] + s["unverified"] + s["supported"] != s["lowered"] or not 0 < s["cards"] or s["lowered"] > s["cards"]:
            raise ValueError("%s: %s: need refused + unverified + supported = lowered <= cards, cards > 0" % (path, code))
    for code, s in (d.get("lockstep") or {}).items():
        if not all(isinstance(s.get(k), int) for k in ("games", "agree", "stalls", "disagree")):
            raise ValueError("%s: lockstep %s: need integer games/agree/stalls/disagree" % (path, code))
        if s["agree"] + s["stalls"] + s["disagree"] != s["games"]:
            raise ValueError("%s: lockstep %s: agree + stalls + disagree != games" % (path, code))
    return d


def read_support_page(path=SUPPORT_PAGE):
    """The rows of dirac/docs/support.md's table: {list: {count: n}}; columns past SUPPORT_COUNTS (the 4-seat ceiling) are not part of the snapshot."""
    rows = {}
    with open(path, encoding="utf-8") as f:
        for line in f:
            cells = [c.strip() for c in line.strip().strip("|").split("|")]
            if line.startswith("|") and len(cells) >= 1 + len(SUPPORT_COUNTS) and cells[1].isdigit():
                rows[cells[0]] = dict(zip(SUPPORT_COUNTS, map(int, cells[1:1 + len(SUPPORT_COUNTS)])))
    return rows


def refresh_dirac(path, commit, date, support=SUPPORT_PAGE):
    """Copy dirac/docs/support.md's counts into the snapshot, stamped commit and date."""
    with open(path, encoding="utf-8") as f:
        d = json.load(f)
    rows = read_support_page(support)
    d["date"], d["commit"] = date, commit[:9]
    d["corpus"] = rows.pop("ALL")
    d["sets"] = dict(sorted(rows.items()))
    with open(path, "w", encoding="utf-8") as f:
        f.write(dump_dirac(d))


def dump_dirac(d):
    """The snapshot's committed form: one line per set, so a diff reads count by count."""
    keys = ["about", "date", "commit", "corpus", "lockstep", "sets"]
    parts = []
    for k in keys:
        if k in ("lockstep", "sets"):
            rows = ",\n".join("    %s: %s" % (json.dumps(c), json.dumps(v)) for c, v in sorted(d[k].items()))
            parts.append('  "%s": {\n%s\n  }' % (k, rows))
        else:
            parts.append('  "%s": %s' % (k, json.dumps(d[k], ensure_ascii=False)))
    return "{\n" + ",\n".join(parts) + "\n}\n"


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


def dd_cell(dd, code, count):
    s = (dd or {}).get("sets", {}).get(code)
    if not s:
        return DASH
    return "%d / %d (%s%%)" % (s[count], s["cards"], pct(s[count], s["cards"]))


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


def forge_pin(path=MAKEFILE):
    """gorge's current corpus pin, FORGE_REF in the Makefile, or "" without one."""
    try:
        with open(path, encoding="utf-8") as f:
            m = re.search(r"^FORGE_REF\s*\??=\s*(\S+)", f.read(), re.M)
    except OSError:
        return ""
    return m.group(1) if m else ""


def corpus_text(meta):
    """The data's corpus, and gorge's current pin when that has moved on."""
    ref = meta.get("forge_ref")
    if not ref:
        return ""
    text = "Forge corpus `%s`" % ref[:9]
    pin = meta.get("current_forge_ref")
    if pin and pin != ref:
        text += " (gorge's `FORGE_REF` is now `%s`)" % pin[:9]
    return text


def stamp_text(meta, level_b):
    if not meta:
        return ""
    parts = []
    if meta.get("date"):
        parts.append("measured " + meta["date"])
    if corpus_text(meta):
        parts.append(corpus_text(meta))
    if meta.get("commit"):
        parts.append("gorge-compliance @ `%s`" % meta["commit"][:9])
    if meta.get("gorge_pin"):
        parts.append("measuring gorge `%s`" % meta["gorge_pin"][:9])
    return "gorge-compliance's `oraclediff status -all -json -level A`%s, %s" % (
        " and `-level B`" if level_b else "", ", ".join(parts))


def data_caption(meta):
    """One line under a table with gorge columns: the data's date and corpus."""
    parts = []
    if meta and meta.get("date"):
        parts.append("measured " + meta["date"])
    if meta and corpus_text(meta):
        parts.append("on " + corpus_text(meta))
    failures = meta.get("ratchet_failures") if meta else 0
    if failures:
        parts.append("with %d compliance ratchet check%s failing" % (failures, "" if failures == 1 else "s"))
    return "*gorge figures %s.*" % ", ".join(parts) if parts else ""


def dd_stamp(dd):
    return "the dirac snapshot of %s at gorge `%s`" % (dd["date"], dd["commit"][:9])


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
        w("| dirac | supported, whole corpus | **%s of %s** cards (%s%%) |" % (
            num(c["supported"]), num(c["cards"]), pct(c["supported"], c["cards"])))
        w("| dirac | lowered, whole corpus (weaker: not yet checked) | %s of %s cards (%s%%) |" % (
            num(c["lowered"]), num(c["cards"]), pct(c["lowered"], c["cards"])))
        w("| dirac | sets on its support page | **%d** |" % len(dd["sets"]))
        ls = dd.get("lockstep") or {}
        if ls:
            parts = []
            for code in sorted(ls):
                s = ls[code]
                parts.append("%s: %s of %s games agree, %d stalled, %d disagree" % (
                    code, num(s["agree"]), num(s["games"]), s["stalls"], s["disagree"]))
            w("| dirac | lockstep slices against gorge | **%s** (%s) |" % (
                ", ".join(sorted(ls)), "; ".join(parts)))
    else:
        w("| dirac | supported | not available to this build |")
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
    w("| Set | Name | Declared | Cards | Playable % | Level A % | Level B % | dirac supported | dirac lowered | Lockstep games |")
    w("|---|---|---|---:|---:|---:|---:|---:|---:|---:|")
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
        w("| %s | %s | %s | %d | %s | %s | %s | %s | %s | %s |" % (
            cell(c), cell(r.get("name", "")), cell(r.get("declared") or DASH), m["cards"],
            pct(m["playable"], m["cards"]), a, bcell,
            dd_cell(dd, c, "supported"), dd_cell(dd, c, "lowered"), lockstep_cell(dd, c)))
    for c in unknown:
        w("| %s | | %s | %s | %s | %s | %s | %s | %s | %s |" % (
            cell(c), DASH, DASH, DASH, DASH, DASH,
            dd_cell(dd, c, "supported"), dd_cell(dd, c, "lowered"), lockstep_cell(dd, c)))
    w("")
    if not rows:
        w("*gorge-compliance's status was not available to this build, so the gorge columns are empty.*")
        w("")
    elif data_caption(meta):
        w(data_caption(meta))
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
    if data_caption(meta):
        w(data_caption(meta))
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
    w("    | Set | Name | Type | Released | Cards | Unsupported | Not in corpus | Playable % | Level A % | dirac supported |")
    w("    |---|---|---|---|---:|---:|---:|---:|---:|---:|")
    for r in rows:
        m = measure(r)
        w("    | %s | %s | %s | %s | %d | %d | %d | %s | %s | %s |" % (
            cell(r["set"]), cell(r.get("name", "")), cell(set_type(r)), cell(r.get("released", "")),
            m["cards"], m["unsupported"], m["missing"], pct(m["playable"], m["cards"]),
            pct(m["a_pass"], m["cards"]) if audited(r) else "≥ " + pct(m["a_pass"], m["cards"]),
            dd_cell(dd, r["set"], "supported")))
    w("")
    return "\n".join(out) + "\n"


def bars(rows, names=("supported", "unverified", "refused")):
    """Generated coverage bars. rows: [(label, ok, unv, ref, total)]; widths are percentages of total."""
    out = ['<div class="gg-bars">']
    for label, ok, unv, ref, total in rows:
        w = lambda n: "%.1f%%" % (100.0 * n / total if total else 0)
        desc = "%s: %d %s, %d %s, %d %s" % (label, ok, names[0], unv, names[1], ref, names[2])
        out.append('<div class="gg-bar" role="img" title="%s" aria-label="%s"><span class="gg-bar__label" aria-hidden="true">%s</span>'
                   '<span class="gg-seg gg-seg--ok" style="--w:%s"></span><span class="gg-seg gg-seg--unv" style="--w:%s"></span>'
                   '<span class="gg-seg gg-seg--ref" style="--w:%s"></span></div>' % (
                       desc, desc, label, w(ok), w(unv), w(ref)))
    out.append("</div>")
    # The key: each swatch is a segment of the bar it names.
    out.append('<ul class="gg-legend gg-bars__key">%s</ul>' % "".join(
        '<li><i class="gg-seg gg-seg--%s"></i>%s</li>' % (k, n) for k, n in zip(("ok", "unv", "ref"), names)))
    return "\n".join(out) + "\n"


def render_charts(rows, level_b, meta, dd):
    out = []
    w = out.append
    sets = (dd or {}).get("sets", {})
    if dd:
        w("**dirac.** Each bar is a set's cards: supported, then unverified (lowered, not yet evidenced), then refused.")
        w("")
        w(bars([(c, sets[c]["supported"], sets[c]["unverified"], sets[c]["refused"], sets[c]["cards"]) for c in sorted(sets)]))
        w("*From " + dd_stamp(dd) + ".*")
        w("")
    else:
        w("*dirac's snapshot was not available to this build, so there is no dirac chart.*")
        w("")
    by = {r["set"]: r for r in rows}
    codes = [c for c in sorted(sets) if c in by]
    if codes:
        w("**gorge.** Each bar is a set's entries: playable, then not in the corpus, then unsupported.")
        w("")
        w(bars([(c, measure(by[c])["playable"], measure(by[c])["missing"], measure(by[c])["unsupported"], measure(by[c])["cards"])
                for c in codes], ("playable", "not in corpus", "unsupported")))
        if data_caption(meta):
            w(data_caption(meta))
            w("")
    else:
        w("*gorge's per-set figures come from gorge-compliance's published status, "
          "which this build did not have, so there is no gorge chart. The figures are "
          "in the by-set table below.*")
        w("")
    return "\n".join(out) + "\n"


def render(rows, meta=None, level_b=None, dd=None):
    level_b = level_b or {}
    return "".join([
        "<!-- Generated by scripts/docs_sets.py; never edit or commit. -->\n",
        section("headline", render_headline(rows, level_b, meta, dd)),
        section("sets", render_sets(rows, level_b, meta, dd)),
        section("rollup", render_rollup(rows, level_b, meta, dd)),
        section("charts", render_charts(rows, level_b, meta, dd)),
    ])


def main(argv=None):
    ap = argparse.ArgumentParser(description=__doc__.split("\n\n")[0])
    ap.add_argument("input", nargs="?", default=None, help="Level A status JSONL ('-' for stdin)")
    ap.add_argument("-o", "--out", default=DEFAULT_OUT, help="fragment path (default: %(default)s)")
    ap.add_argument("--level-b", default="", help="Level B status JSONL")
    ap.add_argument("--meta", default="", help="docs-data meta.json (commit, gorge_pin, forge_ref, date, ratchet_failures)")
    ap.add_argument("--commit", default="", help="gorge-compliance commit (overrides --meta)")
    ap.add_argument("--date", default="", help="measurement date (overrides --meta)")
    ap.add_argument("--dirac", default="", help="dirac per-set snapshot JSON")
    ap.add_argument("--refresh-dirac", default="", metavar="SNAPSHOT",
                    help="copy dirac/docs/support.md's counts into this snapshot, with --commit and --date, and exit")
    a = ap.parse_args(argv)

    if a.refresh_dirac:
        if not a.commit or not a.date:
            print("docs_sets: --refresh-dirac needs --commit and --date", file=sys.stderr)
            return 1
        refresh_dirac(a.refresh_dirac, a.commit, a.date)
        return 0

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
    if meta:
        meta["current_forge_ref"] = forge_pin()
    dd = load_dirac(a.dirac) if a.dirac else None
    if not rows and not dd:
        print("docs_sets: neither status rows nor a dirac snapshot; fragment not written", file=sys.stderr)
        return 1
    with open(a.out, "w", encoding="utf-8") as f:
        f.write(render(rows, meta, level_b, dd))
    print("docs_sets: wrote %d status sets, %d dirac sets to %s" % (
        len(rows), len((dd or {}).get("sets", {})), a.out), file=sys.stderr)
    return 0


if __name__ == "__main__":
    sys.exit(main())
