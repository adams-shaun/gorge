#!/usr/bin/env python3
"""Generate the inline-SVG figures for the research record (docs/site/research/).

Every number is inline: each is a measured result its page reports, in the
page's own table. Output: one .svg per figure under docs/site/research/figs/,
included by the pages through snippets. The figures reuse the MageZero page's
drawing helpers and its .mzc classes in docs/site/stylesheets/diagrams.css, so
they follow the light/dark toggle; every mark has a <title> tooltip.

  python3 scripts/docs_research_figs.py
"""
import math
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from docs_magezero_figs import SVG, ci, esc, legend, xaxis  # noqa: E402

OUT = os.path.join(os.path.dirname(__file__), "..", "docs", "site", "research", "figs")

# Verdict colours: the categorical slots, in the validated order.
MEASURE, ADOPTED, REJECTED = "s1", "s3", "s2"


def half(p, n):
    """The 95% half-width of a score over n games."""
    return 1.96 * math.sqrt(p * (1 - p) / n)


def fig_gates():
    # (label, score, lo, hi, games, verdict class)
    def pm(p, h):
        return p, p - h, p + h
    rows = [
        ("az-c4 vs az-c3", 0.537, 0.497, 0.576, 600, ADOPTED),
        ("az-c5 vs az-c4", 0.528, 0.488, 0.568, 600, REJECTED),
        ("LLM blend vs search", 0.5048, 0.4932, 0.5164, 7100, REJECTED),
        ("az-c6 g7 vs g6", *pm(0.500, half(0.5, 300)), 300, MEASURE),
        ("kernel-hour net vs g7", *pm(0.536, 0.018), 3000, MEASURE),
        ("loop2 k10 vs k9", *pm(0.500, 0.018), 3000, MEASURE),
        ("M_CTL 400 vs 100 sims", *pm(0.5384, 0.0154), 4000, MEASURE),
        ("M_CTL 1600 vs 400 sims", *pm(0.5375, 0.022), 2000, MEASURE),
        ("M_CTL 3200 vs 1600 sims", *pm(0.499, 0.022), 2000, MEASURE),
        ("W64 from scratch vs g26", *pm(0.4723, 0.0155), 4000, MEASURE),
        ("W128 from scratch vs g26", *pm(0.4514, 0.0154), 4000, REJECTED),
        ("GNN G_M vs MLP, equal sims", *pm(0.5302, 0.0155), 4000, REJECTED),
        ("GNN G_M vs MLP, equal time", *pm(0.4855, 0.0155), 4000, REJECTED),
        ("cqraw target vs cq, g9", *pm(0.50475, 0.0155), 4000, REJECTED),
        ("PCR vs control, g9", *pm(0.501625, 0.0155), 4000, REJECTED),
        ("react + held vs control, g6", *pm(0.4965, 0.0155), 4000, ADOPTED),
        ("Pauper g1 vs M_CTLX", 0.5630, 0.5412, 0.5846, 2000, ADOPTED),
        ("Pauper pooled retrain vs g6", 0.4840, 0.4621, 0.5059, 2000, REJECTED),
        ("Pauper g8 vs g6", 0.5292, 0.5073, 0.5510, 2000, ADOPTED),
    ]
    W, top, rh = 760, 70, 25
    H = top + rh * len(rows) + 40
    s = SVG(W, H, "Head-to-head play gates from the research record, each a score with its 95% interval around 0.5")
    s.text(16, 20, "Head-to-head play gates, score of the first-named side", "mz-title")
    legend(s, 16, 46, [(ADOPTED, "adopted", "dot"), (REJECTED, "not adopted", "dot"), (MEASURE, "measurement", "dot")])
    x0, x1 = 210, W - 24
    X = xaxis(s, x0, x1, top - 6, top + rh * len(rows), 0.40, 0.62, 0.02, fmt=lambda v: f"{v:.2f}")
    s.add(f'<line class="mz-ref" x1="{X(0.5):.1f}" y1="{top - 6}" x2="{X(0.5):.1f}" y2="{top + rh * len(rows)}"/>')
    names = {ADOPTED: "adopted", REJECTED: "not adopted", MEASURE: "measurement"}
    for i, (lab, p, lo, hi, n, cls) in enumerate(rows):
        y = top + rh * i + rh / 2
        s.text(x0 - 12, y + 4, lab, "", "end")
        s.add(f'<line class="{cls} ci" x1="{X(lo):.1f}" y1="{y:.1f}" x2="{X(hi):.1f}" y2="{y:.1f}"/>')
        s.dot(X(p), y, cls, f"{lab}: {p:.4f} [{lo:.4f}, {hi:.4f}], {n:,} games, {names[cls]}", r=4.5)
    s.text((x0 + x1) / 2, H - 4, "score (95% interval); the dashed line is an even match", "mz-muted", "middle")
    return s.done()


def fig_search_scaling():
    sims = [25, 100, 400, 1600]
    standin = [(0.564, 0.018), (0.618, 0.017), (0.681, 0.017), (0.702, 0.028)]
    native = [0.658, 0.708, 0.754, 0.793]
    W, H = 760, 360
    s = SVG(W, H, "The g7 net against sb-tactical on the dirac kernel, by simulations per decision")
    s.text(16, 20, "g7 against sb-tactical on the dirac kernel, by simulations", "mz-title")
    legend(s, 16, 44, [("s1", "sb-tactical's stand-in fallback (10-18)", "line"),
                       ("s2", "gorge's heuristic bot ported (xp)", "line")])
    legend(s, 16, 64, [("s3", "sims 400, cpuct 0.5 + root LCB", "dot")])
    x0, x1, y0, y1 = 60, W - 30, 90, H - 40

    def X(n):
        return x0 + (math.log(n / 25, 4)) / 3 * (x1 - x0)

    def Y(v):
        return y1 - (v - 0.5) / (0.85 - 0.5) * (y1 - y0)
    for v in (0.5, 0.55, 0.6, 0.65, 0.7, 0.75, 0.8, 0.85):
        s.add(f'<line class="{"mz-ref" if v == 0.5 else "mz-grid"}" x1="{x0}" y1="{Y(v):.1f}" x2="{x1}" y2="{Y(v):.1f}"/>')
        s.text(x0 - 8, Y(v) + 4, f"{v:.2f}", "mz-muted", "end")
    for n in sims:
        s.text(X(n), y1 + 16, str(n), "mz-muted", "middle")
    s.text((x0 + x1) / 2, H - 4, "simulations per searched decision (log scale)", "mz-muted", "middle")
    s.add(f'<polyline class="s1 ln" points="{" ".join(f"{X(n):.1f},{Y(p):.1f}" for n, (p, _) in zip(sims, standin))}"/>')
    for n, (p, h) in zip(sims, standin):
        games = 1000 if n == 1600 else 3000
        s.add(f'<line class="s1 ci" x1="{X(n):.1f}" y1="{Y(p - h):.1f}" x2="{X(n):.1f}" y2="{Y(p + h):.1f}"/>')
        s.dot(X(n), Y(p), "s1", f"stand-in fallback, sims {n}: {p:.3f} ±{h:.3f}, {games:,} games", r=4.5)
    s.add(f'<polyline class="s2 ln" points="{" ".join(f"{X(n):.1f},{Y(p):.1f}" for n, p in zip(sims, native))}"/>')
    for n, p in zip(sims, native):
        lo, hi = ci(p, 3000)
        s.add(f'<line class="s2 ci" x1="{X(n):.1f}" y1="{Y(lo):.1f}" x2="{X(n):.1f}" y2="{Y(hi):.1f}"/>')
        s.dot(X(n), Y(p), "s2", f"heuristic bot ported, sims {n}: {p:.3f} [{lo:.3f}, {hi:.3f}], 3,000 games", r=4.5)
    s.dot(X(400) + 10, Y(0.784), "s3", "sims 400 with cpuct 0.5 and root LCB: 0.784 ±0.015", r=5)
    return s.done()


def line_chart(title, label, series, xs, ylo, yhi, ystep, xlabel, refs=(), W=760, H=360):
    """series: (class, legend, {x: (p, lo, hi, tip)}, dashed)."""
    s = SVG(W, H, label)
    s.text(16, 20, title, "mz-title")
    items = [(c + (" dash" if d else ""), lg, "line") for c, lg, _, d in series]
    for row in range(0, len(items), 2):
        legend(s, 16, 44 + 20 * (row // 2), items[row:row + 2])
    x0, x1, y0, y1 = 60, W - 30, 70 + 20 * ((len(items) - 1) // 2), H - 40
    lo_x, hi_x = min(xs), max(xs)

    def X(x):
        return x0 + (x - lo_x) / (hi_x - lo_x) * (x1 - x0)

    def Y(v):
        return y1 - (v - ylo) / (yhi - ylo) * (y1 - y0)
    v = ylo
    while v <= yhi + 1e-9:
        s.add(f'<line class="{"mz-ref" if abs(v - 0.5) < 1e-9 else "mz-grid"}" x1="{x0}" y1="{Y(v):.1f}" x2="{x1}" y2="{Y(v):.1f}"/>')
        s.text(x0 - 8, Y(v) + 4, f"{v:.2f}", "mz-muted", "end")
        v += ystep
    for x in xs:
        s.text(X(x), y1 + 16, f"g{x}", "mz-muted", "middle")
    for val, tip in refs:
        s.add(f'<g><title>{esc(tip)}</title><line class="mz-faint dash" x1="{x0}" y1="{Y(val):.1f}" x2="{x1}" y2="{Y(val):.1f}"/></g>')
    s.text((x0 + x1) / 2, H - 4, xlabel, "mz-muted", "middle")
    for cls, _, pts, dashed in series:
        ks = sorted(pts)
        dash = " dash" if dashed else ""
        s.add(f'<polyline class="{cls} ln{dash}" points="{" ".join(f"{X(k):.1f},{Y(pts[k][0]):.1f}" for k in ks)}"/>')
        for k in ks:
            p, lo, hi, tip = pts[k]
            if lo is not None:
                s.add(f'<line class="{cls} ci" x1="{X(k):.1f}" y1="{Y(lo):.1f}" x2="{X(k):.1f}" y2="{Y(hi):.1f}"/>')
            s.dot(X(k), Y(p), cls, tip, r=4)
    return s.done()


def scored(p, n, what):
    lo, hi = ci(p, n)
    return (p, lo, hi, f"{what}: {p:.3f} [{lo:.3f}, {hi:.3f}], {n:,} games")


def fig_c6_loop():
    cseries = {1: 0.598, 2: 0.638, 3: 0.658, 4: 0.678, 5: 0.675, 6: 0.667}
    c6_bot = {1: 0.600, 2: 0.657, 3: 0.680, 4: 0.673, 5: 0.687, 6: 0.727, 7: 0.720}
    c6_prev = {2: 0.560, 3: 0.520, 4: 0.510, 5: 0.527, 6: 0.490, 7: 0.500}
    series = [
        ("s1", "census-on c-series vs sb-tactical (600 games)",
         {g: scored(p, 600, f"C{g} vs sb-tactical") for g, p in cseries.items()}, False),
        ("s2", "az-c6 vs sb-tactical (300 games)",
         {g: scored(p, 300, f"az-c6 g{g} vs sb-tactical") for g, p in c6_bot.items()}, False),
        ("s3", "az-c6 vs the previous generation (300 games)",
         {g: scored(p, 300, f"az-c6 g{g} vs g{g - 1}") for g, p in c6_prev.items()}, True),
    ]
    return line_chart("The FDN self-play loop by generation", "Gate scores of the census-on c-series and the az-c6 loop by generation",
                      series, list(range(1, 8)), 0.40, 0.80, 0.05, "generation")


def fig_pauper():
    champ = {1: (0.5630, 0.5412, 0.5846, "M_CTLX"), 2: (0.5530, 0.5311, 0.5747, "g1"), 3: (0.5415, 0.5196, 0.5632, "g2"),
             4: (0.5517, 0.5299, 0.5734, "g3"), 5: (0.5467, 0.5249, 0.5685, "g4"), 6: (0.5363, 0.5143, 0.5580, "g5"),
             7: (0.5208, 0.4988, 0.5426, "g6"), 8: (0.5292, 0.5073, 0.5510, "g6")}
    bot = {1: 0.6040, 2: 0.6640, 3: 0.6960, 4: 0.7340, 5: 0.7620, 6: 0.8020, 7: 0.8080, 8: 0.8060}
    series = [
        ("s1", "vs the champion (2,000 games)",
         {g: (p, lo, hi, f"g{g} vs {c}: {p:.4f} [{lo:.4f}, {hi:.4f}], 2,000 games") for g, (p, lo, hi, c) in champ.items()}, False),
        ("s2", "vs sb-tactical (500 games)", {g: scored(p, 500, f"g{g} vs sb-tactical") for g, p in bot.items()}, False),
    ]
    return line_chart("Pauper retrain: every generation's gates", "Pauper retrain gate scores by generation, against the champion and against sb-tactical",
                      series, list(range(1, 9)), 0.45, 0.85, 0.05, "generation (g1–g6 run 1, g7–g8 run 2; the pooled retrain is left out)",
                      refs=[(0.5807, "M_CTLX vs sb-tactical: 0.5807 [0.5590, 0.6022], 2,000 games")])


def fig_card_bias():
    groups = [("Text mentions sacrifice", 25, (-20.3, -12.2, -15.7)), ("Mana value 1-2", 90, (-8.9, -4.8, -5.8)),
              ("Mana value 3", 70, (2.3, 4.6, 3.1)), ("Mana value 4", 44, (4.6, 4.3, 6.3)),
              ("Mana value 5", 28, (5.0, 1.3, 1.1)), ("Mana value 6+", 30, (10.0, -3.9, 0.0)),
              ("Enchantments", 18, (9.1, 4.7, 10.3)), ("Creatures", 162, (6.2, 4.0, 4.5)),
              ("Instants", 36, (-19.4, -13.3, -14.7)), ("Sorceries", 29, (-10.3, -5.6, -1.7)),
              ("Artifacts", 12, (-2.7, -2.7, -17.6))]
    lists = [("s1", "fdn-bot-80k"), ("s2", "fdn-azc6g3-20k"), ("s3", "fdn-nonet-3k")]
    W, top, rh = 760, 70, 34
    H = top + rh * len(groups) + 40
    s = SVG(W, H, "Mean simulated percentile minus 17Lands percentile, by card group, for the three FDN lists")
    s.text(16, 20, "Where simulation disagrees with 17Lands (FDN, percentile points)", "mz-title")
    legend(s, 16, 46, [(c, n, "dot") for c, n in lists])
    x0, x1 = 200, W - 24
    X = xaxis(s, x0, x1, top - 6, top + rh * len(groups), -25, 15, 5, fmt=lambda v: f"{v:+.0f}" if v else "0")
    s.add(f'<line class="mz-axis" x1="{X(0):.1f}" y1="{top - 6}" x2="{X(0):.1f}" y2="{top + rh * len(groups)}"/>')
    for i, (g, n, vals) in enumerate(groups):
        y = top + rh * i + 4
        s.text(x0 - 12, y + 16, f"{g} ({n})", "", "end")
        for k, ((cls, name), v) in enumerate(zip(lists, vals)):
            yy = y + 2 + k * 9
            a, b = sorted([X(0), X(v)])
            s.add(f'<g><title>{esc(f"{g}, {name}: {v:+.1f}")}</title>'
                  f'<rect class="{cls}" x="{a:.1f}" y="{yy}" width="{max(b - a, 1.5):.1f}" height="7" rx="1.5" style="stroke:none"/></g>')
    s.text((x0 + x1) / 2, H - 4, "simulation rates the group lower ← 0 → higher than human play", "mz-muted", "middle")
    return s.done()


def fig_instant_timing():
    rows = [("all instant-speed casts", 20.9, 35.2), ("Stab", 30.1, 35.2), ("Luminous Rebuke", 15.7, 35.0),
            ("Faebloom Trick", 5.1, 29.4), ("Goblin Surprise", 7.1, 22.0), ("Giant Growth", 25.6, 32.5),
            ("Refute", 96.6, 89.2)]
    W, top, rh = 760, 70, 32
    H = top + rh * len(rows) + 40
    s = SVG(W, H, "Share of instant-speed casts made on the opponent's turn, control C6 against react R6, with the human reference")
    s.text(16, 20, "Casts on the opponent's turn: control C6 → react R6", "mz-title")
    legend(s, 16, 46, [("mz-grey", "C6 (control)", "grey"), ("s3", "R6 (react + held)", "dot")])
    x0, x1 = 190, W - 24
    X = xaxis(s, x0, x1, top - 6, top + rh * len(rows), 0.0, 1.0, 0.1)
    s.add(f'<g><title>17Lands FDN humans: 49.6% of instant-speed casts on the opponent\'s turn</title>'
          f'<line class="mz-ref" x1="{X(0.496):.1f}" y1="{top - 6}" x2="{X(0.496):.1f}" y2="{top + rh * len(rows)}"/></g>')
    s.text(X(0.496) + 6, top - 10, "humans 49.6%", "mz-muted")
    for i, (lab, c, r) in enumerate(rows):
        y = top + rh * i + rh / 2
        s.text(x0 - 12, y + 4, lab, "mz-title" if i == 0 else "", "end")
        s.add(f'<line class="mz-faint" x1="{X(c / 100):.1f}" y1="{y:.1f}" x2="{X(r / 100):.1f}" y2="{y:.1f}"/>')
        s.add(f'<g><title>{esc(f"{lab}, C6: {c:.1f}%")}</title><circle class="mz-grey" cx="{X(c / 100):.1f}" cy="{y:.1f}" r="4.5"/></g>')
        s.dot(X(r / 100), y, "s3", f"{lab}, R6: {r:.1f}%", r=5)
    s.text((x0 + x1) / 2, H - 4, "share of the card's instant-speed casts made on the opponent's turn", "mz-muted", "middle")
    return s.done()


def main():
    os.makedirs(OUT, exist_ok=True)
    figs = {"gates.svg": fig_gates(), "search-scaling.svg": fig_search_scaling(), "c6-loop.svg": fig_c6_loop(),
            "pauper.svg": fig_pauper(), "card-bias.svg": fig_card_bias(), "instant-timing.svg": fig_instant_timing()}
    for name, body in figs.items():
        with open(os.path.join(OUT, name), "w") as f:
            f.write(body)
        print(name)


if __name__ == "__main__":
    main()
