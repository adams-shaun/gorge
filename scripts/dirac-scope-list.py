#!/usr/bin/env python3
"""Write dirac's out-of-scope card list: the corpus cards its coverage does not divide by.

  dirac-scope-list.py --carddb FILE --cards DIR --out dirac/support/scope/out-of-scope.txt

dirac's "100%" is over Commander-legal paper cards. This reads the public carddb sqlite (cards,
their paper flag and type line, `legality` rows with format 'commander', the printings' set types)
and the corpus at DIR (DIR/cardsfolder, or DIR itself) and writes one line per corpus card that is
out of scope, `name<TAB>reason`, sorted by name (bytewise), the name as the corpus spells it (its
script's first Name: line). Names only: nothing of a script is copied. Run it by hand when the corpus
pin moves and commit the file; dirac/kgen's support page reads it.

A reason is one of:

  alchemy-rebalanced       a script under the corpus's rebalanced/ directory (an A- card)
  digital-only             carddb has no paper printing of it
  non-tournament-type      a Scheme, Plane, Phenomenon, Vanguard, Conspiracy, Attraction,
                           Contraption or Sticker that is not Commander-legal
  non-tournament-mechanic  a paper card, not Commander-legal, whose script carries stickers,
                           attractions, contraptions or the planar die
  not-commander-legal      any other paper card with no Commander legality row
  deferred-mutate          an in-scope card with K:Mutate (deferred until 2027-01-15)
  deferred-splice          an in-scope card with K:Splice (deferred until 2027-01-15)

A card with a Commander legality row, legal or banned, is in scope (bar the two deferrals). So is a
corpus card carddb does not know, and a paper card carddb holds no legality row of any format for
(a 'future' row aside) when one of its printings is in a tournament set: the unknown stays in the
denominator. Both are listed on stderr. (carddb drops every legality row of some cards with a
memorabilia printing, so a missing Commander row alone does not mean not legal.)
"""

import argparse
import collections
import os
import re
import sqlite3
import sys

REASONS = (
    "alchemy-rebalanced",
    "digital-only",
    "non-tournament-type",
    "non-tournament-mechanic",
    "not-commander-legal",
    "deferred-mutate",
    "deferred-splice",
)

NON_TOURNAMENT_TYPES = frozenset(
    ("Scheme", "Plane", "Phenomenon", "Vanguard", "Conspiracy", "Attraction", "Contraption", "Sticker"))

# Set types no tournament-legal card is printed in only: a card whose every printing is in one of
# these is not Commander-legal even when carddb holds no legality row for it.
NON_TOURNAMENT_SET_TYPES = frozenset(("funny", "memorabilia", "vanguard", "token", "minigame", "alchemy"))

MECHANIC = re.compile(r"Sticker|Attraction|Contraption|PlanarDi(?:ce|e)")


class CorpusCard:
    __slots__ = ("name", "rebalanced", "mutate", "splice", "mechanic")

    def __init__(self, name):
        self.name = name
        self.rebalanced = self.mutate = self.splice = self.mechanic = False


def corpus_cards(cards_dir):
    """name -> CorpusCard over every script under cards_dir (its cardsfolder when it has one)."""
    root = os.path.join(cards_dir, "cardsfolder")
    if not os.path.isdir(root):
        root = cards_dir
    out = {}
    for d, dirs, files in os.walk(root):
        dirs.sort()
        rebalanced = "rebalanced" in os.path.relpath(d, root).split(os.sep)
        for f in sorted(files):
            if not f.endswith(".txt"):
                continue
            with open(os.path.join(d, f), encoding="utf-8") as fh:
                lines = fh.read().splitlines()
            name = next((ln[len("Name:"):] for ln in lines if ln.startswith("Name:")), None)
            if name is None:
                continue
            c = out.setdefault(name, CorpusCard(name))
            c.rebalanced |= rebalanced
            for ln in lines:
                if ln.startswith("Name:"):
                    continue
                c.mutate |= ln.startswith("K:Mutate")
                c.splice |= ln.startswith("K:Splice")
                c.mechanic |= bool(MECHANIC.search(ln))
    return out


class CardDB:
    def __init__(self, path):
        db = sqlite3.connect(f"file:{path}?mode=ro", uri=True)
        try:
            self.by_name = collections.defaultdict(list)
            self.by_front = collections.defaultdict(list)
            for oid, name, type_line, paper in db.execute("SELECT oracle_id, name, type_line, paper FROM cards"):
                row = (oid, type_line or "", paper)
                self.by_name[name].append(row)
                if " // " in name:
                    self.by_front[name.split(" // ", 1)[0]].append(row)
            self.commander = {r[0] for r in db.execute("SELECT oracle_id FROM legality WHERE format = 'commander'")}
            # A 'future' row is an unreleased card's: it says nothing yet of Commander.
            self.any_legality = {
                r[0] for r in db.execute("SELECT DISTINCT oracle_id FROM legality WHERE format != 'future'")}
            self.set_types = collections.defaultdict(set)
            for oid, st in db.execute(
                    "SELECT p.oracle_id, s.set_type FROM printings p JOIN sets s ON s.set_code = p.set_code"):
                self.set_types[oid].add(st)
        finally:
            db.close()

    def lookup(self, name):
        return self.by_name.get(name) or self.by_front.get(name) or []


def non_tournament_type(type_line):
    for face in type_line.split(" // "):
        if NON_TOURNAMENT_TYPES & set(face.split("—", 1)[0].split()):
            return True
    return False


def classify(card, db):
    """(reason, unknown): reason is None for an in-scope card; unknown names why carddb could not
    place it, for stderr."""
    if card.rebalanced:
        return "alchemy-rebalanced", None
    rows = db.lookup(card.name)
    unknown = None
    if not rows:
        unknown = "not in carddb"
    elif not any(oid in db.commander for oid, _, _ in rows):
        if all(paper == 0 for _, _, paper in rows):
            return "digital-only", None
        paper_rows = [r for r in rows if r[2] != 0]
        if any(non_tournament_type(tl) for _, tl, _ in paper_rows):
            return "non-tournament-type", None
        known = any(oid in db.any_legality for oid, _, _ in paper_rows)
        sets = set().union(*(db.set_types.get(oid, set()) for oid, _, _ in paper_rows))
        if known or (sets and sets <= NON_TOURNAMENT_SET_TYPES):
            return ("non-tournament-mechanic" if card.mechanic else "not-commander-legal"), None
        unknown = "no legality row in carddb, printed in a tournament set"
    if card.mutate:
        return "deferred-mutate", unknown
    if card.splice:
        return "deferred-splice", unknown
    return None, unknown


def build(carddb, cards_dir):
    """(out-of-scope rows sorted by name, [(name, why unknown)])."""
    db = CardDB(carddb)
    rows, unknown = [], []
    for name, card in corpus_cards(cards_dir).items():
        reason, why = classify(card, db)
        if why:
            unknown.append((name, why))
        if reason:
            rows.append((name, reason))
    # Python orders str by code point, which is UTF-8 byte order: the order Go's sort gives.
    rows.sort()
    unknown.sort()
    return rows, unknown


def main(argv=None):
    ap = argparse.ArgumentParser(description=__doc__.split("\n", 1)[0])
    ap.add_argument("--carddb", required=True, help="the public carddb sqlite")
    ap.add_argument("--cards", required=True, help="the corpus directory (holding cardsfolder)")
    ap.add_argument("--out", required=True, help="the list to write")
    a = ap.parse_args(argv)
    rows, unknown = build(a.carddb, a.cards)
    for name, why in unknown:
        print(f"in scope, unknown: {name}: {why}", file=sys.stderr)
    os.makedirs(os.path.dirname(os.path.abspath(a.out)), exist_ok=True)
    with open(a.out, "w", encoding="utf-8", newline="\n") as fh:
        fh.writelines(f"{name}\t{reason}\n" for name, reason in rows)
    counts = collections.Counter(reason for _, reason in rows)
    for reason in REASONS:
        print(f"{reason}\t{counts[reason]}")
    print(f"total\t{len(rows)}")
    print(f"unknown (in scope)\t{len(unknown)}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
