#!/usr/bin/env python3
"""Tests for scripts/dirac-scope-list.py. Stdlib only:

    python3 -I scripts/dirac_scope_list_test.py

Each builds a tiny carddb sqlite (the columns the script reads) and a tiny corpus of made-up
scripts, runs the script's main and reads the list it writes.
"""

import contextlib
import importlib.util
import io
import os
import sqlite3
import tempfile
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))


def load_module():
    spec = importlib.util.spec_from_file_location("dirac_scope_list", os.path.join(HERE, "dirac-scope-list.py"))
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


SCHEMA = """
CREATE TABLE cards (oracle_id TEXT PRIMARY KEY, name TEXT NOT NULL, type_line TEXT NOT NULL, paper INTEGER);
CREATE TABLE legality (oracle_id TEXT NOT NULL, format TEXT NOT NULL, status TEXT NOT NULL);
CREATE TABLE printings (printing_id TEXT PRIMARY KEY, oracle_id TEXT NOT NULL, set_code TEXT NOT NULL);
CREATE TABLE sets (set_code TEXT PRIMARY KEY, set_type TEXT NOT NULL);
INSERT INTO sets VALUES ('xex', 'expansion'), ('xfn', 'funny'), ('xar', 'archenemy'), ('xme', 'memorabilia');
"""

# name -> (type line, paper, {format: status}, [set codes], script lines after Name:)
CARDS = {
    "Legal Paper Bear": ("Creature — Bear", 1, {"commander": "legal", "legacy": "legal"}, ["xex"], ["Types:Creature Bear"]),
    "Banned Paper Ape": ("Creature — Ape", 1, {"commander": "banned", "legacy": "legal"}, ["xex"], ["Types:Creature Ape"]),
    "Screen Only Wisp": ("Creature — Spirit", 0, {"alchemy": "legal"}, [], ["Types:Creature Spirit"]),
    "Doom Plot": ("Scheme", 1, {}, ["xar"], ["Types:Scheme"]),
    "Joke Goat": ("Creature — Goat", 1, {}, ["xfn"], ["Types:Creature Goat"]),
    "Sticky Imp": ("Creature — Imp", 1, {}, ["xfn"], ["Types:Creature Imp", "A:AB$ AddSticker"]),
    "Old Gold Lotus": ("Artifact", 1, {}, ["xex", "xme"], ["Types:Artifact"]),
    "Coming Soon Elk": ("Creature — Elk", 1, {"future": "legal"}, ["xex"], ["Types:Creature Elk"]),
    "Hybrid Beast": ("Creature — Beast", 1, {"commander": "legal"}, ["xex"], ["Types:Creature Beast", "K:Mutate:2"]),
    "Arcane Bolt": ("Instant — Arcane", 1, {"commander": "legal"}, ["xex"], ["Types:Instant Arcane", "K:Splice:Arcane:1"]),
    "Front Face // Back Face": ("Instant // Sorcery", 1, {"commander": "legal"}, ["xex"], None),
}


class ScopeListTest(unittest.TestCase):
    def setUp(self):
        self.mod = load_module()
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        d = self.tmp.name
        self.db = os.path.join(d, "carddb.sqlite")
        con = sqlite3.connect(self.db)
        con.executescript(SCHEMA)
        for i, (name, (tl, paper, legal, sets, _)) in enumerate(CARDS.items()):
            oid = f"oid-{i}"
            con.execute("INSERT INTO cards VALUES (?, ?, ?, ?)", (oid, name, tl, paper))
            con.executemany("INSERT INTO legality VALUES (?, ?, ?)", [(oid, f, s) for f, s in legal.items()])
            con.executemany("INSERT INTO printings VALUES (?, ?, ?)", [(f"{oid}-{s}", oid, s) for s in sets])
        con.commit()
        con.close()
        self.cards = os.path.join(d, "cards")
        folder = os.path.join(self.cards, "cardsfolder")
        scripts = {name: lines for name, (_, _, _, _, lines) in CARDS.items() if lines is not None}
        scripts["Front Face"] = ["Types:Instant", "ALTERNATE", "Name:Back Face", "Types:Sorcery"]
        scripts["Unknown Newcomer"] = ["Types:Creature Elf"]
        for name, lines in scripts.items():
            self.write_script(os.path.join(folder, "x"), name, lines)
        self.write_script(os.path.join(folder, "rebalanced"), "A-Legal Paper Bear", ["Types:Creature Bear"])
        self.out = os.path.join(d, "scope", "out-of-scope.txt")

    def write_script(self, d, name, lines):
        os.makedirs(d, exist_ok=True)
        stem = "".join(ch if ch.isalnum() else "_" for ch in name.lower())
        with open(os.path.join(d, stem + ".txt"), "w", encoding="utf-8") as fh:
            fh.write("\n".join([f"Name:{name}"] + lines) + "\n")

    def run_script(self):
        stdout, stderr = io.StringIO(), io.StringIO()
        with contextlib.redirect_stdout(stdout), contextlib.redirect_stderr(stderr):
            rc = self.mod.main(["--carddb", self.db, "--cards", self.cards, "--out", self.out])
        self.assertEqual(rc, 0)
        with open(self.out, encoding="utf-8") as fh:
            body = fh.read()
        rows = dict(line.split("\t") for line in body.splitlines())
        return body, rows, stdout.getvalue(), stderr.getvalue()

    def test_reasons(self):
        body, rows, stdout, stderr = self.run_script()
        self.assertIn("Legal Paper Bear", CARDS)  # the precondition: the corpus holds it
        self.assertNotIn("Legal Paper Bear", rows, "a Commander-legal paper card is in scope")
        self.assertNotIn("Banned Paper Ape", rows, "a banned Commander card is in scope")
        self.assertEqual(rows.get("Screen Only Wisp"), "digital-only")
        self.assertEqual(rows.get("Doom Plot"), "non-tournament-type")
        self.assertEqual(rows.get("Joke Goat"), "not-commander-legal")
        self.assertEqual(rows.get("Sticky Imp"), "non-tournament-mechanic")
        self.assertEqual(rows.get("A-Legal Paper Bear"), "alchemy-rebalanced")
        self.assertEqual(rows.get("Hybrid Beast"), "deferred-mutate")
        self.assertEqual(rows.get("Arcane Bolt"), "deferred-splice")
        self.assertNotIn("Front Face", rows, "a split card is found by its front face")
        self.assertNotIn("Back Face", rows, "a card is named by its first Name: line")
        self.assertEqual(set(rows.values()) - set(self.mod.REASONS), set())
        self.assertEqual(body.splitlines(), sorted(body.splitlines()))
        self.assertIn("digital-only\t1\n", stdout)

    def test_unknown_stays_in_scope(self):
        _, rows, _, stderr = self.run_script()
        self.assertNotIn("Unknown Newcomer", rows, "a card carddb does not know is in scope")
        self.assertIn("Unknown Newcomer: not in carddb", stderr)
        self.assertNotIn("Old Gold Lotus", rows, "no legality row at all and a tournament printing: unknown, in scope")
        self.assertIn("Old Gold Lotus: no legality row", stderr)
        self.assertNotIn("Coming Soon Elk", rows, "an unreleased card's 'future' row says nothing of Commander")
        self.assertIn("Coming Soon Elk: no legality row", stderr)


if __name__ == "__main__":
    unittest.main()
