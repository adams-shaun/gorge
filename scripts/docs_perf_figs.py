#!/usr/bin/env python3
"""Generate the inline-SVG figures for the performance pages (docs/site/performance/).

Every plotted value comes from docs/site/performance/data/perf.json, the
measured results the pages report. Output: one .svg per figure under
docs/site/performance/figs/, included by the pages through snippets. The
figures reuse the MageZero page's drawing helpers and the .mzc classes in
docs/site/stylesheets/diagrams.css, so they follow the light/dark toggle and
carry no background; every mark has a <title> tooltip. Stdlib only, and the
output is byte-stable for a given data file (docs_perf_figs_test.py).

  python3 scripts/docs_perf_figs.py [--check]
"""
import argparse
import json
import math
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from docs_magezero_figs import SVG, esc, legend  # noqa: E402

ROOT = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..")
DATA = os.path.join(ROOT, "docs", "site", "performance", "data", "perf.json")
OUT = os.path.join(ROOT, "docs", "site", "performance", "figs")

# One colour per engine on every figure.
ENGINE = {"gorge": "s1", "XMage": "s2", "dirac": "s3", "Forge": "c4", "mtg-kernel": "c5"}


def num(v):
    """A value as the pages print it."""
    if v >= 1000:
        return f"{v:,.0f}"
    if v >= 100:
        return f"{v:.0f}"
    if v >= 10:
        return f"{v:.1f}".rstrip("0").rstrip(".")
    if v >= 1:
        return f"{v:.2f}".rstrip("0").rstrip(".")
    return f"{v:.3g}"


class Scale:
    def __init__(self, x0, x1, lo, hi, log):
        self.x0, self.x1, self.log = x0, x1, log
        if log:
            # A little headroom past the largest value rather than a whole empty decade.
            self.lo, self.hi = math.floor(math.log10(lo)), math.log10(hi) + 0.15
        else:
            st = nice_step(hi)
            self.lo, self.hi = 0.0, math.ceil(hi / st - 1e-9) * st
        if self.hi == self.lo:
            self.hi += 1

    def __call__(self, v):
        t = (math.log10(v) - self.lo) / (self.hi - self.lo) if self.log else (v - self.lo) / (self.hi - self.lo)
        return self.x0 + t * (self.x1 - self.x0)

    def ticks(self, step=None):
        if self.log:
            return [10 ** e for e in range(self.lo, math.floor(self.hi) + 1)]
        step = step or nice_step(self.hi)
        out, v = [], 0.0
        while v <= self.hi + 1e-9:
            out.append(v)
            v += step
        return out


def nice_step(hi):
    raw = hi / 5
    mag = 10 ** math.floor(math.log10(raw))
    for m in (1, 2, 2.5, 5, 10):
        if m * mag >= raw:
            return m * mag
    return 10 * mag


def ticklabel(v):
    return f"{v:,.0f}" if v >= 1 else f"{v:g}"


def hbars(title, label, rows, xlabel, log=True, W=760, lw=210, rh=26, ref=None, items=None, hi=None, step=None):
    """rows: (label, value, lo, hi, cls, tip) or ("#", heading) for a group heading.
    Bars start at the axis minimum; a thin line marks lo-hi when given; the value is printed at the bar's end."""
    top = 44 + (22 if items else 0)
    H = top + rh * len(rows) + 40
    s = SVG(W, H, label)
    s.text(16, 20, title, "mz-title")
    if items:
        legend(s, 16, 46, items)
    vals = [r[1] for r in rows if r[0] != "#"] + [r[3] for r in rows if r[0] != "#" and r[3] is not None]
    x0, x1 = lw, W - 70
    lo_v = min(r[1] for r in rows if r[0] != "#")
    X = Scale(x0, x1, lo_v, hi or max(vals), log)
    y0, y1 = top - 4, top + rh * len(rows)
    for t in X.ticks(step):
        s.add(f'<line class="mz-grid" x1="{X(t):.1f}" y1="{y0}" x2="{X(t):.1f}" y2="{y1}"/>')
        s.text(X(t), y1 + 15, ticklabel(t), "mz-muted", "middle")
    if ref is not None:
        s.add(f'<line class="mz-ref" x1="{X(ref):.1f}" y1="{y0}" x2="{X(ref):.1f}" y2="{y1}"/>')
    base = X(10 ** X.lo) if log else X(0)
    for i, r in enumerate(rows):
        y = top + rh * i
        if r[0] == "#":
            s.text(16, y + rh / 2 + 4, r[1], "mz-title")
            continue
        lab, v, lo, h, cls, tip = r
        s.text(x0 - 8, y + rh / 2 + 4, lab, "", "end")
        s.add(f'<g><title>{esc(tip)}</title><rect class="{cls}" x="{base:.1f}" y="{y + 4:.1f}" '
              f'width="{max(X(v) - base, 1.5):.1f}" height="{rh - 8}" rx="2" style="stroke:none"/>')
        end = X(v)
        if lo is not None:
            s.add(f'<line class="mz-axis" x1="{X(lo):.1f}" y1="{y + rh / 2:.1f}" x2="{X(h):.1f}" y2="{y + rh / 2:.1f}" style="stroke-width:2"/>'
                  f'<line class="mz-axis" x1="{X(h):.1f}" y1="{y + 7:.1f}" x2="{X(h):.1f}" y2="{y + rh - 7:.1f}"/>')
            end = max(end, X(h))
        s.add("</g>")
        s.text(end + 6, y + rh / 2 + 4, num(v))
    s.add(f'<line class="mz-axis" x1="{base:.1f}" y1="{y0}" x2="{base:.1f}" y2="{y1}"/>')
    s.text((x0 + x1) / 2, H - 6, xlabel, "mz-muted", "middle")
    return s.done()


def grouped(title, label, cats, series, xlabel, log=False, W=760, lw=170, unit=""):
    """cats: labels; series: (cls, legend, [value or None per cat], [tip per cat])."""
    bh, gap = 12, 12
    gh = bh * len(series) + gap
    top = 70
    H = top + gh * len(cats) + 36
    s = SVG(W, H, label)
    s.text(16, 20, title, "mz-title")
    legend(s, 16, 46, [(c, lg, "dot") for c, lg, _, _ in series])
    vals = [v for _, _, vs, _ in series for v in vs if v]
    x0, x1 = lw, W - 70
    X = Scale(x0, x1, min(vals), max(vals), log)
    y0, y1 = top - 6, top + gh * len(cats)
    for t in X.ticks():
        s.add(f'<line class="mz-grid" x1="{X(t):.1f}" y1="{y0}" x2="{X(t):.1f}" y2="{y1}"/>')
        s.text(X(t), y1 + 15, ticklabel(t), "mz-muted", "middle")
    base = X(10 ** X.lo) if log else X(0)
    for i, c in enumerate(cats):
        y = top + gh * i
        s.text(x0 - 8, y + bh * len(series) / 2 + 4, c, "", "end")
        for j, (cls, _, vs, tips) in enumerate(series):
            v = vs[i]
            if not v:
                continue
            yy = y + bh * j
            s.add(f'<g><title>{esc(tips[i])}</title><rect class="{cls}" x="{base:.1f}" y="{yy + 1:.1f}" '
                  f'width="{max(X(v) - base, 1.5):.1f}" height="{bh - 2}" rx="1.5" style="stroke:none"/></g>')
            s.text(X(v) + 5, yy + bh - 2, num(v) + unit, "mz-muted")
    s.add(f'<line class="mz-axis" x1="{base:.1f}" y1="{y0}" x2="{base:.1f}" y2="{y1}"/>')
    s.text((x0 + x1) / 2, H - 6, xlabel, "mz-muted", "middle")
    return s.done()


def name(r):
    return f'{r["engine"]} · {r["policy"]}'


# ---- index.md


def engine_rows(d, key, unit, groups=True):
    rows = []
    for kind, head in (("random", "Random legal action: the engine almost alone"), ("ai", "Each engine's own AI")):
        if groups:
            rows.append(("#", head))
        for r in d["engines"]["rows"]:
            if r["kind"] != kind:
                continue
            v = r[key]
            med, lo, hi = (v if isinstance(v, list) else (v, None, None))
            tip = f"{name(r)}: {num(med)} {unit}" + (f" (min {num(lo)}, max {num(hi)})" if lo is not None else "") + f", measured {r['date']}"
            rows.append((name(r), med, lo, hi, ENGINE[r["engine"]], tip))
    return rows


def fig_us_turn(d):
    return hbars("CPU per turn, by engine and policy (lower is cheaper)",
                 "Bar chart, log scale: CPU microseconds per turn for each engine and policy",
                 engine_rows(d, "us_turn", "µs/turn"), "CPU µs per turn (log scale); whisker = slowest to fastest run")


def fig_games(d):
    return hbars("Games per core-second, by engine and policy (higher is faster)",
                 "Bar chart, log scale: whole games played per CPU-second for each engine and policy",
                 engine_rows(d, "games", "games/core-s"), "games per core-second (log scale); whisker = slowest to fastest run")


def fig_rss(d):
    rows = []
    seen = set()
    for r in d["engines"]["rows"] + d["kernel_workloads"]["rows"]:
        if r["engine"] in seen:
            continue
        seen.add(r["engine"])
        same = [x["rss"] for x in d["engines"]["rows"] + d["kernel_workloads"]["rows"] if x["engine"] == r["engine"]]
        lo, hi = min(same), max(same)
        med = sorted(same)[len(same) // 2]
        rows.append((r["engine"], med, lo, hi, ENGINE[r["engine"]],
                     f"{r['engine']}: peak RSS {num(lo)}-{num(hi)} MB over its rows"))
    rows.sort(key=lambda r: r[1])
    return hbars("Peak memory of a game-playing process", "Bar chart, log scale: peak resident memory per engine",
                 rows, "peak RSS, MB (log scale); bar = middle row, whisker = range over every row the engine has", lw=120)


def fig_workloads(d):
    rows = []
    for wl in ("Pair A", "Burn"):
        rows.append(("#", f"{wl}" + (" (FDN, mtg-kernel's FDN build)" if wl == "Pair A" else " mirror (mono-red Pauper)")))
        for r in d["kernel_workloads"]["rows"]:
            if r["workload"] == wl:
                rows.append((name(r), r["us_turn"], None, None, ENGINE[r["engine"]],
                             f"{wl} · {name(r)}: {num(r['us_turn'])} µs/turn, {r['turns']} turns/game, measured {r['date']}"))
    return hbars("CPU per turn on the decks mtg-kernel plays", "Bar chart, log scale: microseconds per turn on FDN pair A and the Burn mirror",
                 rows, "CPU µs per turn, random policy (log scale)")


def fig_dirac_ops(d):
    rows = [(r["op"], r["instr"], None, None, "s3", f"{r['op']}: {r['instr']:,} instructions, {num(r['ns'])} ns per op, 0 allocations")
            for r in d["dirac_ops"]["rows"]]
    return hbars("dirac: instructions per operation", "Bar chart, log scale: instructions per dirac transition and copy",
                 rows, "user-space instructions per operation (log scale), median of 5; every row allocates nothing", lw=230)


def fig_copy_vs_step(d):
    g = d["gorge_ops"]["rows"]
    di = {r["op"]: r for r in d["dirac_ops"]["rows"]}
    cats = ["copy a position", "one transition"]
    gorge_gc = [g[0]["us_gc"], g[1]["us_gc"]]
    gorge_nogc = [g[0]["us_nogc"], g[1]["us_nogc"]]
    dirac = [di["CopyFrom, mid-game FDN"]["ns"] / 1000, di["Transition, FDN"]["ns"] / 1000]
    return grouped("What a search pays per copy and per step", "Grouped bars, log scale: microseconds per copy and per transition, gorge against dirac",
                   cats, [("s1", "gorge, with GC", gorge_gc, [f"gorge {c}: {num(v)} µs with GC" for c, v in zip(cats, gorge_gc)]),
                          ("s1 soft", "gorge, GC off", gorge_nogc, [f"gorge {c}: {num(v)} µs, GC off" for c, v in zip(cats, gorge_nogc)]),
                          ("s3", "dirac", dirac, [f"dirac {c}: {num(v)} µs, 0 allocations" for c, v in zip(cats, dirac)])],
                   "µs per operation (log scale); gorge 2026-11-05 (clone, step), dirac 2026-11-02 (CopyFrom, Transition FDN)", log=True, unit=" µs")


# ---- engine.md


def fig_gorge_rows(d):
    gr = d["gorge_rows"]
    cats = [r["row"] for r in gr["rows"]]
    series = []
    for j, (pair, cls) in enumerate(zip(gr["pairs"], ("s1", "s2", "s3"))):
        vs = [r["games"][j][0] for r in gr["rows"]]
        tips = [f"{r['row']}, {pair}: {num(r['games'][j][0])} games/core-s ({num(r['games'][j][1])}-{num(r['games'][j][2])}), "
                f"{num(r['us_turn'][j])} µs/turn, {r['turns'][j]} turns/game" for r in gr["rows"]]
        series.append((cls, pair, vs, tips))
    return grouped("gorge benchmark rows: games per core-second", "Grouped bars: gorge enginebench games per core-second by row and workload",
                   cats, series, "games per core-second, median of 3 reps (b78e4c8b1 baseline, re-measured 2026-11-05)")


def fig_gorge_turn(d):
    gr = d["gorge_rows"]
    cats = [r["row"] for r in gr["rows"]]
    series = [(cls, pair, [r["us_turn"][j] for r in gr["rows"]],
               [f"{r['row']}, {pair}: {num(r['us_turn'][j])} µs/turn, {num(r['us_decision'][j])} µs/decision" for r in gr["rows"]])
              for j, (pair, cls) in enumerate(zip(gr["pairs"], ("s1", "s2", "s3")))]
    return grouped("gorge benchmark rows: CPU per turn", "Grouped bars: gorge CPU microseconds per turn by row and workload",
                   cats, series, "CPU µs per turn (lower is cheaper)", unit=" µs")


def fig_regression(d):
    rows = d["regression"]["rows"]
    W, top, rh = 760, 50, 30
    H = top + rh * len(rows) + 40
    s = SVG(W, H, "Range plot: HEAD's CPU per turn as a multiple of the b78e4c8b1 baseline, per row")
    s.text(16, 20, "HEAD against the baseline: CPU per turn, HEAD ÷ b78e4c8b1", "mz-title")
    x0, x1 = 160, W - 40

    def P(v):
        return x0 + (v - 0.95) / 0.35 * (x1 - x0)
    y0, y1 = top - 6, top + rh * len(rows)
    for t in (0.95, 1.0, 1.05, 1.1, 1.15, 1.2, 1.25, 1.3):
        s.add(f'<line class="{"mz-ref" if t == 1.0 else "mz-grid"}" x1="{P(t):.1f}" y1="{y0}" x2="{P(t):.1f}" y2="{y1}"/>')
        s.text(P(t), y1 + 15, f"×{t:.2f}", "mz-muted", "middle")
    s.text(P(1.0) + 4, y0 + 10, "no change", "mz-muted")
    for i, r in enumerate(rows):
        y = top + rh * i + rh / 2
        s.text(x0 - 8, y + 4, r["row"], "", "end")
        tip = f"{r['row']}: HEAD costs ×{r['lo']:.2f}-{r['hi']:.2f} the baseline's CPU per turn"
        s.add(f'<g><title>{esc(tip)}</title><rect class="s2" x="{P(r["lo"]) - 4:.1f}" y="{y - 6:.1f}" '
              f'width="{P(r["hi"]) - P(r["lo"]) + 8:.1f}" height="12" rx="6" style="stroke:none"/></g>')
        s.text(P(r["hi"]) + 10, y + 4, f"×{r['lo']:.2f}-{r['hi']:.2f}")
    s.text((x0 + x1) / 2, H - 6, "ratio of CPU per turn, three ABBA reps per row, pinned core, 2026-11-05 (right of the dashed line = slower)",
           "mz-muted", "middle")
    return s.done()


def fig_profile(d):
    p = d["profile"]
    cats = p["packages"]
    series = [(cls, run, [v or None for v in p["runs"][run]], [f"{run}: {pkg} {v}% of flat CPU" for pkg, v in zip(cats, p["runs"][run])])
              for run, cls in (("random -autopay", "s1"), ("bot -autopay", "s2"))]
    return grouped("Where the CPU goes: flat time by package", "Grouped bars: share of flat CPU time by package, random and bot runs",
                   cats, series, "% of flat CPU, one 20 s pair-A profile per run (0ec285f67, 2026-11-05)", unit="%")


def fig_calltree(d):
    rows = [(n, v, None, None, "s1", f"{n}: {v}% cumulative CPU") for n, v in d["profile"]["cumulative"]["random -autopay"]]
    return hbars("The hot path, random auto-pay run: cumulative CPU by call", "Bar chart: cumulative share of CPU of the main call-tree nodes",
                 rows, "% of CPU, cumulative (nodes nest, so the bars don't sum)", log=False, hi=100, step=20, lw=230)


# ---- bots.md


def fig_bot_games(d):
    rows = d["bot_games"]["rows"]
    cats = [r["policy"] for r in rows]
    series = [(cls, pair, [r["ms_game"][j] for r in rows], [f"{r['policy']}, {pair}: {num(r['ms_game'][j])} ms a game, {num(r['us_decision'][j])} µs a decision" for r in rows])
              for j, (pair, cls) in enumerate((("Pair A", "s1"), ("Pair B", "s2")))]
    return grouped("A whole game with the heuristic bot on both seats", "Grouped bars: milliseconds of CPU per gorge game by policy and pair",
                   cats, series, "CPU ms per game, gorge, both seats the same policy, 2026-10-31", unit=" ms")


def loglog(title, label, d, key, ylabel, rng=True):
    W, H = 760, 380
    s = SVG(W, H, label)
    s.text(16, 20, title, "mz-title")
    x0, x1, y0, y1 = 80, W - 110, 40, H - 50
    rows = d["search"]["rows"]
    ys = [r[key][0] if isinstance(r[key], list) else r[key] for r in rows]
    if rng:
        ys += [r[key][2] for r in rows] + [r[key][1] for r in rows]
    elo, ehi = math.floor(math.log10(min(ys))), math.ceil(math.log10(max(ys)))

    def X(n):
        return x0 + (math.log10(n) - math.log10(25)) / (math.log10(1000) - math.log10(25)) * (x1 - x0)

    def Y(v):
        return y1 - (math.log10(v) - elo) / (ehi - elo) * (y1 - y0)
    for e in range(elo, ehi + 1):
        s.add(f'<line class="mz-grid" x1="{x0}" y1="{Y(10 ** e):.1f}" x2="{x1}" y2="{Y(10 ** e):.1f}"/>')
        s.text(x0 - 8, Y(10 ** e) + 4, ticklabel(10 ** e), "mz-muted", "end")
    for n in (25, 100, 1000):
        s.add(f'<line class="mz-grid" x1="{X(n):.1f}" y1="{y0}" x2="{X(n):.1f}" y2="{y1}"/>')
        s.text(X(n), y1 + 16, f"{n:,}", "mz-muted", "middle")
    s.text((x0 + x1) / 2, H - 14, "simulations per searched decision (log scale)", "mz-muted", "middle")
    s.add(f'<text x="18" y="{(y0 + y1) / 2:.1f}" class="mz-muted" text-anchor="middle" transform="rotate(-90 18 {(y0 + y1) / 2:.1f})">{esc(ylabel)}</text>')
    for eng in ("gorge", "dirac"):
        cls = ENGINE[eng]
        pts = [r for r in rows if r["engine"] == eng]

        def val(r):
            return r[key][0] if isinstance(r[key], list) else r[key]
        s.add(f'<polyline class="{cls} ln" points="{" ".join(f"{X(r["sims"]):.1f},{Y(val(r)):.1f}" for r in pts)}"/>')
        for r in pts:
            if rng:
                s.add(f'<line class="{cls} ci" x1="{X(r["sims"]):.1f}" y1="{Y(r[key][1]):.1f}" x2="{X(r["sims"]):.1f}" y2="{Y(r[key][2]):.1f}"/>')
            tip = f"{eng}, {r['sims']:,} sims: {num(r['us_sim'][0])} µs/sim, {r['sims_per_s']:,} sims/core-s, {num(r['ms_decision'])} ms/decision"
            s.dot(X(r["sims"]), Y(val(r)), cls, tip, r=4.5)
            s.text(X(r["sims"]) + 8, Y(val(r)) - 8, num(val(r)), "mz-muted")
        last = pts[-1]
        s.text(X(last["sims"]) + 14, Y(val(last)) + 14, eng, "mz-title")
    return s.done()


def fig_search_sim(d):
    return loglog("Cost of one simulation grows with the budget", "Line chart, log-log: microseconds per simulation against simulations per decision, gorge and dirac",
                  d, "us_sim", "CPU µs per simulation (log)")


def fig_search_decision(d):
    return loglog("What a searched decision costs", "Line chart, log-log: milliseconds per searched decision against simulations, gorge and dirac",
                  d, "ms_decision", "CPU ms per searched decision (log)", rng=False)


# ---- methodology.md


def fig_decisions_per_turn(d):
    rows = []
    for r in d["engines"]["rows"]:
        if r["us_decision"] is None:
            continue
        v = r["us_turn"][0] / r["us_decision"]
        rows.append((name(r), round(v, 1), None, None, ENGINE[r["engine"]],
                     f"{name(r)}: {num(r['us_turn'][0])} µs/turn ÷ {num(r['us_decision'])} µs/decision = {v:.1f} decisions per turn"))
    return hbars("Decisions per turn, as each engine counts them", "Bar chart: decisions per turn by engine and policy, derived from µs per turn and µs per decision",
                 rows, "decisions per turn = µs per turn ÷ µs per decision (Forge has no decision count)", log=False)


FIGS = {
    "us-per-turn.svg": fig_us_turn,
    "games-per-core-s.svg": fig_games,
    "peak-rss.svg": fig_rss,
    "kernel-workloads.svg": fig_workloads,
    "dirac-ops.svg": fig_dirac_ops,
    "copy-vs-step.svg": fig_copy_vs_step,
    "gorge-rows.svg": fig_gorge_rows,
    "gorge-turn.svg": fig_gorge_turn,
    "regression.svg": fig_regression,
    "profile.svg": fig_profile,
    "calltree.svg": fig_calltree,
    "bot-games.svg": fig_bot_games,
    "search-per-sim.svg": fig_search_sim,
    "search-per-decision.svg": fig_search_decision,
    "decisions-per-turn.svg": fig_decisions_per_turn,
}


def render(data_path=DATA):
    with open(data_path) as f:
        d = json.load(f)
    return {n: fn(d) for n, fn in FIGS.items()}


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--check", action="store_true", help="fail if a committed figure differs from a fresh render")
    a = ap.parse_args()
    figs = render()
    os.makedirs(OUT, exist_ok=True)
    stale = []
    for n, body in figs.items():
        p = os.path.join(OUT, n)
        if a.check:
            try:
                with open(p) as f:
                    if f.read() != body:
                        stale.append(n)
            except FileNotFoundError:
                stale.append(n)
            continue
        with open(p, "w") as f:
            f.write(body)
        print(n)
    if stale:
        sys.exit("stale figures (run scripts/docs_perf_figs.py): " + " ".join(stale))


if __name__ == "__main__":
    main()
