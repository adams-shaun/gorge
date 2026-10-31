#!/usr/bin/env python3
"""Generate the inline-SVG figures for docs/site/bots/magezero-replication.md.

The numbers are inline (they are the measured results the page reports); the
UWTempo training curve also reads an optional TSV of dirac generations
(columns: arm gen opp games wins wr ci_lo ci_hi). Output: one .svg per figure
under docs/site/bots/magezero/, included by the page through snippets. The
SVGs carry class names only, so docs/site/stylesheets/diagrams.css (.mzc)
themes them for light and dark mode; every mark has a <title> tooltip.

  python3 scripts/docs_magezero_figs.py [--curve curve.tsv]
"""
import argparse
import math
import os

OUT = os.path.join(os.path.dirname(__file__), "..", "docs", "site", "bots", "magezero")


def ci(p, n):
    h = 1.96 * math.sqrt(max(p * (1 - p), 1e-9) / n)
    return max(0.0, p - h), min(1.0, p + h)


def esc(s):
    return s.replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;")


class SVG:
    def __init__(self, w, h, label):
        self.w, self.h = w, h
        self.parts = [f'<svg class="mzc" viewBox="0 0 {w} {h}" role="img" aria-label="{esc(label)}" xmlns="http://www.w3.org/2000/svg">']

    def add(self, s):
        self.parts.append(s)

    def text(self, x, y, s, cls="", anchor="start"):
        c = f' class="{cls}"' if cls else ""
        self.add(f'<text x="{x:.1f}" y="{y:.1f}"{c} text-anchor="{anchor}">{esc(s)}</text>')

    def dot(self, x, y, cls, tip, r=5, shape="circle"):
        # A larger transparent hit target carries the tooltip.
        self.add(f'<g><title>{esc(tip)}</title><circle cx="{x:.1f}" cy="{y:.1f}" r="11" fill="transparent"/>')
        if shape == "circle":
            self.add(f'<circle class="dot {cls}" cx="{x:.1f}" cy="{y:.1f}" r="{r}"/></g>')
        else:
            self.add(f'<rect class="dot {cls}" x="{x - r:.1f}" y="{y - r:.1f}" width="{2 * r}" height="{2 * r}" rx="1.5"/></g>')

    def done(self):
        # No blank lines: the SVG is inlined into Markdown.
        return "\n".join(self.parts + ["</svg>"]) + "\n"


def legend(svg, x, y, items):
    for cls, label, shape in items:
        if shape == "band":
            svg.add(f'<rect class="mz-band" x="{x}" y="{y - 7}" width="18" height="10" rx="2"/>')
        elif shape == "grey":
            svg.add(f'<circle class="mz-grey" cx="{x + 6}" cy="{y - 2}" r="4.5"/>')
        elif shape == "line":
            svg.add(f'<line class="{cls} ln" x1="{x}" y1="{y - 2}" x2="{x + 18}" y2="{y - 2}"/>')
        elif shape == "faint":
            svg.add(f'<line class="mz-faint" x1="{x}" y1="{y - 2}" x2="{x + 18}" y2="{y - 2}"/>')
        else:
            svg.add(f'<circle class="dot {cls}" cx="{x + 6}" cy="{y - 2}" r="5"/>')
        svg.text(x + 24, y + 2, label)
        x += 30 + 7 * len(label)


def xaxis(svg, x0, x1, y0, y1, lo, hi, step, fmt=lambda v: f"{v:.0%}"):
    def X(v):
        return x0 + (v - lo) / (hi - lo) * (x1 - x0)
    v = lo
    while v <= hi + 1e-9:
        svg.add(f'<line class="mz-grid" x1="{X(v):.1f}" y1="{y0}" x2="{X(v):.1f}" y2="{y1}"/>')
        svg.text(X(v), y1 + 16, fmt(v), "mz-muted", "middle")
        v += step
    return X


def fig_faeries():
    # Faeries (seat A) gen0 win rate, no network on either side.
    rows = [  # opp, ver4 (p, n), jar (p, n), kernel base (p, n), kernel matched (p, n)
        ("Spy", (0.467, 394), (0.36, 50), (0.677, 400), (0.537, 400)),
        ("Affinity", (0.400, 100), (0.48, 50), (0.590, 400), (0.575, 400)),
        ("Elves", (0.355, 386), (0.51, 49), (0.460, 400), (0.443, 400)),
        ("Wildfire", (0.577, 397), (0.46, 50), (0.645, 400), (0.665, 400)),
        ("Burn", (0.560, 100), (0.52, 50), (0.615, 400), (0.635, 400)),
        ("mirror", (0.509, 399), (0.54, 50), (0.495, 400), (0.505, 400)),
    ]
    W, top, rh = 760, 64, 38
    H = top + rh * len(rows) + 40
    s = SVG(W, H, "Faeries gen0 win rate per opponent: XMage published and rerun band against dirac before and after matching the search")
    s.text(16, 20, "Faeries gen0 win rate (no network), per opponent", "mz-title")
    legend(s, 16, 44, [("", "XMage: published ↔ jar rerun", "band"), ("s2", "dirac, default search", "dot"),
                       ("s1", "dirac, search matched to MCTS2", "dot")])
    x0, x1 = 110, W - 24
    X = xaxis(s, x0, x1, top - 6, top + rh * len(rows), 0.30, 0.75, 0.05)
    s.add(f'<line class="mz-ref" x1="{X(0.5):.1f}" y1="{top - 6}" x2="{X(0.5):.1f}" y2="{top + rh * len(rows)}"/>')
    for i, (opp, v4, jar, kb, km) in enumerate(rows):
        y = top + rh * i + rh / 2
        s.text(x0 - 12, y + 4, opp, "", "end")
        a, b = sorted([v4[0], jar[0]])
        s.add(f'<g><title>{esc(f"XMage: published ver4 {v4[0]:.3f} ({v4[1]} games), jar rerun {jar[0]:.2f} ({jar[1]} games)")}</title>'
              f'<rect class="mz-band" x="{X(a) - 3:.1f}" y="{y - 11:.1f}" width="{X(b) - X(a) + 6:.1f}" height="22" rx="4"/></g>')
        for p, n, cls, lab in ((kb[0], kb[1], "s2", "default search"), (km[0], km[1], "s1", "matched search")):
            lo, hi = ci(p, n)
            off = -5 if cls == "s2" else 5
            s.add(f'<line class="{cls} ci" x1="{X(lo):.1f}" y1="{y + off:.1f}" x2="{X(hi):.1f}" y2="{y + off:.1f}"/>')
            s.dot(X(p), y + off, cls, f"Faeries vs {opp}, dirac {lab}: {p:.3f} [{lo:.3f}, {hi:.3f}], {n} games")
    s.text(X(0.5), H - 4, "Faeries win rate (95% CI on dirac points)", "mz-muted", "middle")
    return s.done()


def fig_gen1():
    rows = [("MonoB", 0.074, (0.316, 171), (0.300, 100)), ("MonoG", 0.075, (0.160, 200), (0.240, 100)),
            ("MonoR", 0.102, (0.175, 200), (0.270, 100)), ("MonoU", 0.536, (0.754, 199), (0.760, 100)),
            ("MonoW", 0.210, (0.575, 186), (0.480, 100))]
    W, top, rh = 760, 64, 38
    H = top + rh * len(rows) + 40
    s = SVG(W, H, "Faeries gen1 win rate vs the Standard monos: MageZero's published gen1 against a rerun of their model and jar")
    s.text(16, 20, "Their gen1 model on their jar: published vs rerun here", "mz-title")
    legend(s, 16, 44, [("", "their gen0", "grey"), ("s1", "their gen1 (published)", "dot"), ("s2", "gen1 rerun here", "dot")])
    x0, x1 = 110, W - 24
    X = xaxis(s, x0, x1, top - 6, top + rh * len(rows), 0.0, 0.9, 0.1)
    for i, (opp, g0, g1, rerun) in enumerate(rows):
        y = top + rh * i + rh / 2
        s.text(x0 - 12, y + 4, opp, "", "end")
        s.add(f'<g><title>Their gen0 vs {opp}: {g0:.3f} (about 2,000 games)</title><circle class="mz-grey" cx="{X(g0):.1f}" cy="{y:.1f}" r="4.5"/></g>')
        for (p, n), cls, lab, off in ((g1, "s1", "their gen1", -5), (rerun, "s2", "rerun here", 5)):
            lo, hi = ci(p, n)
            s.add(f'<line class="{cls} ci" x1="{X(lo):.1f}" y1="{y + off:.1f}" x2="{X(hi):.1f}" y2="{y + off:.1f}"/>')
            s.dot(X(p), y + off, cls, f"Faeries vs {opp}, {lab}: {p:.3f} [{lo:.3f}, {hi:.3f}], {n} games")
    s.text((x0 + x1) / 2, H - 4, "Faeries win rate (95% CI)", "mz-muted", "middle")
    return s.done()


def fig_decisions():
    cats = [("priority", "s1"), ("combat picks", "s2"), ("other targets", "s3"), ("yes/no asks", "seg4"),
            ("choose / modes", "seg5")]
    rows = [("MageZero ver4", [45.4, 24.76, 9.04, 2.48, 0.0]), ("XMage jar rerun", [50.3, 24.48, 9.62, 2.40, 0.0]),
            ("dirac", [35.2, 9.66, 2.94, 1.0, 3.56])]
    W, top, rh = 760, 70, 40
    H = top + rh * len(rows) + 40
    s = SVG(W, H, "Recorded decisions per seat-game by kind: XMage 82-87, dirac 52")
    s.text(16, 20, "Recorded decisions per seat-game, by kind (Faeries pool, gen0)", "mz-title")
    lx = 16
    for lab, cls in cats:
        s.add(f'<rect class="{cls}" x="{lx}" y="{37}" width="12" height="12" rx="2" style="stroke:none"/>')
        s.text(lx + 18, 47, lab)
        lx += 30 + 7 * len(lab)
    x0, x1 = 130, W - 60
    X = xaxis(s, x0, x1, top - 6, top + rh * len(rows), 0, 90, 10, fmt=lambda v: f"{v:.0f}")
    for i, (name, vals) in enumerate(rows):
        y = top + rh * i + 8
        s.text(x0 - 12, y + 16, name, "", "end")
        acc = 0.0
        for (lab, cls), v in zip(cats, vals):
            if v <= 0:
                continue
            xa, xb = X(acc), X(acc + v)
            s.add(f'<g><title>{esc(f"{name}: {lab} {v:.1f} per seat-game")}</title>'
                  f'<rect class="seg {cls}" x="{xa:.1f}" y="{y}" width="{xb - xa:.1f}" height="22" rx="3"/></g>')
            acc += v
        s.text(X(acc) + 8, y + 16, f"{acc:.1f}")
    s.text((x0 + x1) / 2, H - 4, "decisions per seat-game", "mz-muted", "middle")
    return s.done()


THEIRS = {  # their UWTempo main line (roundrobin-trainlog.txt), per opponent B G R U W, 200 games each
    0: (.415, .415, .335, .270, .300), 1: (.405, .410, .425, .285, .280), 2: (.470, .465, .490, .345, .425),
    3: (.565, .520, .485, .455, .485), 4: (.560, .585, .525, .475, .390), 5: (.600, .540, .600, .470, .410),
    6: (.655, .550, .595, .480, .435), 7: (.575, .550, .490, .455, .375), 8: (.597, .600, .505, .480, .465), 9: (.595, .615, .480, .480, .510),
    10: (.610, .640, .575, .460, .535), 11: (.590, .670, .595, .565, .575), 12: (.745, .625, .620, .530, .520),
    13: (.595, .665, .590, .530, .490), 14: (.705, .620, .555, .560, .550), 15: (.620, .680, .620, .520, .585),
    16: (.705, .700, .685, .545, .575), 17: (.655, .680, .645, .650, .600),
}


def read_curve(path):
    arms = {}
    if not path or not os.path.exists(path):
        return arms
    for ln in open(path):
        f = ln.rstrip("\n").split("\t")
        if not f or f[0] in ("arm", "") or len(f) < 5:
            continue
        arm, gen, opp, games, wins = f[0], int(f[1]), f[2], int(f[3]), float(f[4])
        if not opp.startswith("MTGA_"):  # skip the TSV's pooled AVG rows
            continue
        g = arms.setdefault(arm, {}).setdefault(gen, [0, 0])
        g[0] += games
        g[1] += wins
    return {a: {g: (w / n, n) for g, (n, w) in gens.items() if n} for a, gens in arms.items()}


def fig_curve(curve):
    W, H = 760, 380
    s = SVG(W, H, "UWTempo average win rate vs the five MTGA mono decks by training generation")
    s.text(16, 20, "UWTempo vs the five MTGA monos: average win rate per generation", "mz-title")
    items = [("", "MageZero per opponent", "faint"), ("s1", "MageZero average", "line")]
    labels = {"L": "dirac, calibrated opponents", "L2": "same, second seed",
              "K": "dirac, heuristic opponents"}
    cls_of = {"L": "s2", "L2": "s2", "K": "s3"}
    for arm in ("L", "L2", "K"):
        if arm in curve:
            items.append((cls_of[arm] + (" dash" if arm == "L2" else ""), labels[arm], "line"))
    legend(s, 16, 44, items[:3])
    legend(s, 16, 64, items[3:])
    x0, x1, y0, y1 = 56, W - 24, 80, H - 40
    gmax = max([17] + [max(g) for g in curve.values() if g])

    def X(g):
        return x0 + g / gmax * (x1 - x0)

    def Y(v):
        return y1 - (v - 0.2) / (0.9 - 0.2) * (y1 - y0)
    for v in (0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9):
        s.add(f'<line class="{"mz-ref" if v == 0.5 else "mz-grid"}" x1="{x0}" y1="{Y(v):.1f}" x2="{x1}" y2="{Y(v):.1f}"/>')
        s.text(x0 - 8, Y(v) + 4, f"{v:.0%}", "mz-muted", "end")
    for g in range(0, gmax + 1, 2 if gmax <= 24 else 4):
        s.text(X(g), y1 + 16, str(g), "mz-muted", "middle")
    s.text((x0 + x1) / 2, H - 4, "training generation (1 generation = 1,000 games)", "mz-muted", "middle")
    gens = sorted(THEIRS)
    for k in range(5):
        pts = " ".join(f"{X(g):.1f},{Y(THEIRS[g][k]):.1f}" for g in gens)
        s.add(f'<polyline class="mz-faint" points="{pts}"/>')
    avg = {g: sum(v) / 5 for g, v in THEIRS.items()}
    s.add(f'<polyline class="s1 ln" points="{" ".join(f"{X(g):.1f},{Y(avg[g]):.1f}" for g in gens)}"/>')
    for g in gens:
        lo, hi = ci(avg[g], 1000)
        s.dot(X(g), Y(avg[g]), "s1", f"MageZero gen{g}: {avg[g]:.3f} [{lo:.3f}, {hi:.3f}] over 1,000 games", r=4)
    for arm, gd in sorted(curve.items(), key=lambda kv: {"K": 0, "L2": 1, "L": 2}.get(kv[0], 3)):
        cls = cls_of.get(arm, "s3")
        gs = sorted(gd)
        dash = " dash" if arm == "L2" else ""
        s.add(f'<polyline class="{cls} ln{dash}" points="{" ".join(f"{X(g):.1f},{Y(gd[g][0]):.1f}" for g in gs)}"/>')
        for g in gs:
            p, n = gd[g]
            lo, hi = ci(p, n)
            if arm != "L2":
                s.dot(X(g), Y(p), cls, f"{labels.get(arm, arm)} gen{g}: {p:.3f} [{lo:.3f}, {hi:.3f}] over {n} games", r=4)
    return s.done()


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--curve", help="dirac generations TSV (arm gen opp games wins ...)")
    a = ap.parse_args()
    os.makedirs(OUT, exist_ok=True)
    figs = {"faeries-gen0.svg": fig_faeries(), "gen1-rerun.svg": fig_gen1(), "decisions.svg": fig_decisions(),
            "uwtempo-curve.svg": fig_curve(read_curve(a.curve))}
    for name, body in figs.items():
        with open(os.path.join(OUT, name), "w") as f:
            f.write(body)
        print(name)


if __name__ == "__main__":
    main()
