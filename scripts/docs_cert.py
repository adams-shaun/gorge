#!/usr/bin/env python3
"""Render the docs site's set-certification pages: support/certification.md
and one page per set, support/cert/<SET>.md.

Source: gorge-compliance's orphan cert-data branch, published by four-yay's
`ops/fleet/discovery/certloop/publish.py` (SC13 of the set-certification
design, `.worktrees/set-cert-design/.workspaces/set-certification/DESIGN.md`).
A card is certified once, as a card name, at its first printing; a set's
reprints inherit that level. The export is counts, card names, levels,
reasons and ticket ids only — never card or script text (the GPL corpus),
never anything private — and no field is published over 200 characters
except `tickets`. `.github/workflows/docs.yml` fetches the branch the same
way it fetches `docs-data`; without it the overview page says "not measured"
and the build still passes. The pages are build artifacts, gitignored, and
regenerated at every docs build.

The three files, one JSON object per line (non-JSON lines are skipped):

1. `cert-sets.jsonl`: one row per set, the deckdb `cert_set` columns plus
   `pages_cards`: `set_code`, `released`, `set_type`, `paper` (every paper
   printing, reprints included), `new_paper`, `reprints`, `playable`,
   `excluded`, `c0`-`c3`, `open`, `level` (the highest Ck the set is
   certified at, or ""), `state` (queued, dealing, triage, certified,
   blocked, regressed), `cursor`, `measured`.
2. `cert-cards.jsonl`: one row per card: `name`, `first_set`, `sets` (the
   codes with a paper printing of it; without it the card sits only in
   `first_set`), `level` (C0-C3, or "" below C0), `exclusion` (ante,
   dexterity, subgame, nonpaper, ...), `refs` (FX | X | F | ""), `why`
   (the reason it is below the next level), `tickets`, `keys_total`,
   `keys_agreed`, `triggers_unmeasured`.
3. `cert-meta.json`: `gorge_sha`, `corpus_pin`, `compliance_sha`, `measured`.

Levels (design section 3): each is an all-abilities claim, never a fraction.

    C0  playable   gorge compiles the card with no missing primitive; it is paper
    C1  exercised  every ability key was used in a clean cross-engine game
    C2  agreed     every key used in a compared window where gorge = XMage
                   (and Forge when it answered); no open difference
    C3  certified  C2 plus compliance Level B and a passing Oracle-text audit

Exclusions are not levels: an `excluded:<cause>` card is listed on its set
page and never counted as certified.

The limits box condenses section 2 of the design ("What 'agree with Forge
and XMage' proves, and what it does not") to six bullets.

Stdlib only. Usage:
  scripts/docs_cert.py [--cert-data DIR] [-o docs/site/support]
"""

import argparse
import json
import os
import shutil
import sys

DEFAULT_OUT = "docs/site/support"
DASH = "—"
NOT_MEASURED = ("gorge's set certification is **not measured** for this build: "
                "gorge-compliance's `cert-data` was not available, so no set has "
                "figures or a page yet. [The support matrices](sets.md) cover the "
                "playable and supported shares.")
LEVEL_NAMES = ("C0 playable", "C1 exercised", "C2 agreed", "C3 certified")

# Condensed from the set-certification design, section 2 ("What 'agree with
# Forge and XMage' proves, and what it does not"), 2026-11-20.
LIMITS = [
    "gorge and Forge read the same script, so a gorge = Forge window only shows "
    "that gorge reads Forge's script the way Forge does. XMage is the only "
    "independently written reference: C2 needs an XMage agreement wherever XMage "
    "has the card, and a Forge-only agreement is counted separately as `refs=F`.",
    "All three engines can share a bug. Agreement is a vote, never a ruling: a card "
    "is wrong only on the printed card (current Oracle) and the CR. C3's "
    "Oracle-text audit and CR-cited scenarios are the defence.",
    "Random play does not reach rare abilities. Conditional modes, X = 0, "
    "alternative costs and interactions with specific other cards are hit by chance "
    "or not at all; certification decks with enablers and the per-ability Level B "
    "scenarios are the fix.",
    "Only some abilities can be counted. Statics and replacements have no coverage "
    "key: they show only as board state in compared windows, and in Level B "
    "scenarios.",
    "Agreement is window-local. Some windows cannot be compared, and a card's "
    "agreeing window may not include the ability that matters: a card's evidence is "
    "one agreeing window, not every ability.",
    "Scenario certification is not game certification. Single-card scenarios miss "
    "interactions, and the games are two-seat; multiplayer rules are certified by "
    "Forge-only 4-seat pods plus scenarios.",
]


def load_rows(path, key):
    """cert-data JSONL as {key: row}, or {} when the file is missing; a
    non-JSON line (a summary) is skipped, a row without the key too."""
    rows = {}
    try:
        with open(path, encoding="utf-8") as f:
            text = f.read()
    except OSError:
        return rows
    for line in text.splitlines():
        line = line.strip()
        if not line.startswith("{"):
            continue
        try:
            r = json.loads(line)
        except ValueError:
            continue
        if isinstance(r, dict) and r.get(key):
            rows[r[key]] = r
    return rows


def load_meta(path):
    try:
        with open(path, encoding="utf-8") as f:
            return json.load(f)
    except (OSError, ValueError):
        return {}


def cell(s):
    return str(s).replace("|", "\\|").replace("\n", " ")


def short(sha):
    return (sha or "")[:9]


def stamp(meta, link_back=False):
    parts = []
    if meta.get("measured"):
        parts.append("measured " + str(meta["measured"])[:10])
    if meta.get("gorge_sha"):
        parts.append("gorge `%s`" % short(meta["gorge_sha"]))
    if meta.get("corpus_pin"):
        parts.append("corpus `%s`" % meta["corpus_pin"])
    if meta.get("compliance_sha"):
        parts.append("gorge-compliance `%s`" % short(meta["compliance_sha"]))
    text = "From the `cert-data` export%s" % (", " + ", ".join(parts) if parts else "")
    if link_back:
        text += ". [Overview](../certification.md) · [Support matrices](../sets.md)"
    return "*%s.*" % text


def n(s, k):
    return int(s.get(k) or 0)


def in_release_order(sets):
    """The set rows, oldest first: certification walks the paper corpus in
    release order. A row without a release date sorts last, by code."""
    rows = list(sets.values()) if isinstance(sets, dict) else list(sets)
    return sorted(rows, key=lambda s: (s.get("released") or "9999-99-99", s["set_code"]))


def card_sets(c):
    """The set codes the card has a paper printing in."""
    s = c.get("sets")
    codes = {c for c in s if isinstance(c, str)} if isinstance(s, list) else set()
    codes.add(c["first_set"])
    return codes


def cards_in_set(cards, code):
    rows = list(cards.values()) if isinstance(cards, dict) else list(cards)
    return sorted((c for c in rows if code in card_sets(c)), key=lambda c: c["name"])


def level_cell(c, code):
    """The Level column: the card's level, `excluded:<cause>`, or — below C0.
    A reprint's level is its first printing's, so it shows that."""
    exclusion = c.get("exclusion") or ""
    level = c.get("level") or ""
    first = c.get("first_set") or ""
    if first and first != code:
        if level:
            return "certified at %s (%s)" % (first, level)
        return level_cell(c, first)
    if exclusion:
        return "excluded:%s" % exclusion
    return level or DASH


OVERVIEW_HEAD = "| Set | Released | Paper | New | C0 | C1 | C2 | C3 | Excluded | Open | State | Level |"
OVERVIEW_SEP = "|---|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|"


def render_overview(sets):
    out = []
    w = out.append
    if not sets:
        w(NOT_MEASURED)
        w("")
        return "\n".join(out) + "\n"
    w(OVERVIEW_HEAD)
    w(OVERVIEW_SEP)
    for s in in_release_order(sets):
        w("| [%s](cert/%s.md) | %s | %d | %d | %d | %d | %d | %d | %d | %d | %s | %s |" % (
            cell(s["set_code"]), cell(s["set_code"]), cell(s.get("released") or DASH),
            n(s, "paper"), n(s, "new_paper"),
            n(s, "c0"), n(s, "c1"), n(s, "c2"), n(s, "c3"),
            n(s, "excluded"), n(s, "open"),
            cell(s.get("state") or DASH), cell(s.get("level") or DASH)))
    w("")
    w("Every paper card is at a level, `excluded:<cause>` (listed on its set page, "
      "never counted as certified), or named with a reason it falls short, so the "
      "four level columns, Excluded and Open account for the set's paper cards. "
      "**Paper** counts every paper printing, reprints included; **New** the cards "
      "first printed here. A card is certified once, at its first printing: a set's "
      "reprints inherit that level, and its page shows where the certification came "
      "from. **State** is the certloop state machine (queued, dealing, triage, "
      "certified, blocked, regressed); **Level** the highest Ck the set is certified "
      "at, — when it is not.")
    w("")
    w("Each set links to its own page, which lists every paper card in it with its "
      "level, the references that agreed, the reason it falls short and its tickets.")
    w("")
    return "\n".join(out) + "\n"


def render_cert(meta, sets, cards):
    out = []
    w = out.append
    w("# Set certification")
    w("")
    w("How far gorge's card behaviour is certified, set by set, in release order. "
      "Each level is an **all-abilities** claim about a card, never a fraction: "
      "%s. A card below a level is always listed with the reason it falls "
      "short, on its set's page." % ", ".join(LEVEL_NAMES))
    w("")
    w("This is one of three measures and does not replace the others: the "
      "[support matrices](sets.md) cover what each engine plays and supports, and "
      "compliance's scenario levels. Certification is measured by gorge's "
      "`certloop` against Forge and XMage play, gorge-compliance's scenarios and "
      "the Oracle text, and published through gorge-compliance's `cert-data` "
      "branch.")
    w("")
    w('!!! warning "What this evidence proves, and what it does not"')
    w("")
    for b in LIMITS:
        w("    - %s" % b)
    w("")
    w("## Every set, in release order")
    w("")
    w(render_overview(sets))
    if sets:
        w(stamp(meta))
        w("")
    return "\n".join(out) + "\n"


SET_HEAD = "| Card | Level | Refs | Why | Tickets |"
SET_SEP = "|---|---|---|---|---|"


def set_summary(s):
    level = s.get("level") or ""
    level_text = "not certified" if not level else "certified at %s" % level
    return ("**Released** %s · **Type** %s · **State** %s · **%s**\n\n"
            "**Paper** %d (%d new, %d reprints) · **Playable** %d · **Excluded** %d · "
            "**Open** %d" % (
                cell(s.get("released") or DASH), cell(s.get("set_type") or DASH),
                cell(s.get("state") or DASH), cell(level_text),
                n(s, "paper"), n(s, "new_paper"), n(s, "reprints"),
                n(s, "playable"), n(s, "excluded"), n(s, "open")))


def render_set(s, cards):
    out = []
    w = out.append
    w("# Set certification: %s" % cell(s["set_code"]))
    w("")
    w(set_summary(s))
    w("")
    w("A card's level is an all-abilities claim made once, at its first printing; "
      "a row that says `certified at <first set>` is a reprint inheriting it. "
      "`excluded:<cause>` cards are never counted as certified. Below the levels, "
      "a card is listed with the reason it falls short and its tickets.")
    w("")
    w(SET_HEAD)
    w(SET_SEP)
    for c in cards:
        w("| %s | %s | %s | %s | %s |" % (
            cell(c["name"]), cell(level_cell(c, s["set_code"])),
            cell(c.get("refs") or DASH), cell(c.get("why") or DASH),
            cell(c.get("tickets") or DASH)))
    w("")
    if not cards:
        w("No cards published for this set yet.")
        w("")
    triggers = sum(n(c, "triggers_unmeasured") for c in cards)
    if triggers:
        w("%d trigger key%s on this set's card%s the farm does not credit yet, so "
          "the cards carrying them wait below C1." % (
              triggers, "" if triggers == 1 else "s",
              "" if len(cards) == 1 else "s"))
        w("")
    w(stamp({"gorge_sha": s.get("gorge_sha"), "corpus_pin": s.get("corpus_pin"),
             "measured": s.get("measured")}, link_back=True))
    return "\n".join(out) + "\n"


def main(argv=None):
    ap = argparse.ArgumentParser(description=__doc__.split("\n\n")[0])
    ap.add_argument("--cert-data", default="", metavar="DIR",
                    help="directory holding SC13's cert-sets.jsonl, cert-cards.jsonl "
                         "and cert-meta.json (gorge-compliance's cert-data branch)")
    ap.add_argument("-o", "--out", default=DEFAULT_OUT,
                    help="docs support directory (default: %(default)s)")
    a = ap.parse_args(argv)
    d = a.cert_data
    sets = load_rows(os.path.join(d, "cert-sets.jsonl"), "set_code") if d else {}
    cards = load_rows(os.path.join(d, "cert-cards.jsonl"), "name") if d else {}
    meta = load_meta(os.path.join(d, "cert-meta.json")) if d else {}
    if d and not sets:
        print("docs_cert: no cert-sets rows in %s; the certification pages render "
              "as not measured" % d, file=sys.stderr)
    os.makedirs(a.out, exist_ok=True)
    with open(os.path.join(a.out, "certification.md"), "w", encoding="utf-8") as f:
        f.write(render_cert(meta, sets, cards))
    certdir = os.path.join(a.out, "cert")
    if not sets:
        # a build without cert-data must not leave the last data build's pages
        if os.path.isdir(certdir):
            shutil.rmtree(certdir)
    else:
        os.makedirs(certdir, exist_ok=True)
        for s in in_release_order(list(sets.values())):
            with open(os.path.join(certdir, "%s.md" % s["set_code"]), "w",
                      encoding="utf-8") as f:
                f.write(render_set(s, cards_in_set(list(cards.values()), s["set_code"])))
    print("docs_cert: %d sets, %d cards -> %s" % (len(sets), len(cards), a.out),
          file=sys.stderr)
    return 0


if __name__ == "__main__":
    sys.exit(main())
