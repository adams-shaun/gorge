#!/usr/bin/env python3
"""Tests for edh_decks.py's normaliser. Stdlib only, no network:

    python3 -m unittest discover -s scripts -p 'edh_decks_test.py'
"""

import io
import json
import os
import sys
import tempfile
import unittest
import urllib.error

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import edh_decks  # noqa: E402


class PreconTest(unittest.TestCase):
    def test_entries_keep_commander_decks_and_collections(self):
        dl = {"data": [
            {"type": "Commander Deck", "code": "C13", "fileName": "B_C13", "name": "B", "releaseDate": "2013-11-01"},
            {"type": "Commander Deck", "code": "CMD", "fileName": "A_CMD", "name": "A", "releaseDate": "2011-06-17"},
            {"type": "Box Set", "code": "CM1", "fileName": "Ars_CM1", "name": "Commander's Arsenal", "releaseDate": "2012-10-23"},
            {"type": "Box Set", "code": "CC1", "fileName": "P_CC1", "name": "Commander Collection: Green - Premium", "releaseDate": "2020-12-04"},
            {"type": "Box Set", "code": "OCM1", "fileName": "O_OCM1", "name": "Commander's Arsenal Oversized", "releaseDate": "2012-10-23"},
            {"type": "Theme Deck", "code": "10E", "fileName": "T_10E", "name": "T", "releaseDate": "2007-07-13"},
        ]}
        got = [(r["fileName"], k) for r, k in edh_decks.precon_entries(dl)]
        self.assertEqual(got, [("A_CMD", "precon"), ("Ars_CM1", "collection"), ("B_C13", "precon")])

    def test_mtgjson_deck_puts_commander_in_cards(self):
        data = {"data": {
            "commander": [{"name": "Kaalia of the Vast", "count": 1}],
            "mainBoard": [{"name": "Sol Ring", "count": 1}, {"name": "Plains", "count": 5},
                          {"name": "Plains", "count": 2},
                          {"name": "Delver of Secrets // Insectile Aberration", "count": 1}],
            "sideBoard": [{"name": "Not Played", "count": 1}],
        }}
        row = {"name": "Heavenly Inferno", "fileName": "HI_CMD", "code": "CMD", "releaseDate": "2011-06-17"}
        d = edh_decks.mtgjson_deck(data, row, "precon")
        self.assertEqual(d["commander"], "Kaalia of the Vast")
        self.assertNotIn("commanders", d)
        self.assertEqual(d["format"], "commander")
        self.assertEqual(d["cards"], [
            {"name": "Kaalia of the Vast", "count": 1}, {"name": "Sol Ring", "count": 1},
            {"name": "Plains", "count": 7},
            {"name": "Delver of Secrets // Insectile Aberration", "count": 1}])

    def test_display_commander_fallback(self):
        data = {"data": {"commander": [], "displayCommander": [{"name": "X"}], "mainBoard": [{"name": "Y", "count": 1}]}}
        row = {"name": "n", "fileName": "f", "code": "c", "releaseDate": "d"}
        self.assertEqual(edh_decks.mtgjson_deck(data, row, "precon")["commander"], "X")
        self.assertNotIn("commander", edh_decks.mtgjson_deck(data, row, "collection"))


class ArchidektTest(unittest.TestCase):
    RAW = {
        "id": 7, "name": "Deck", "edhBracket": 3, "viewCount": 10, "deckFormat": 3, "owner": {"username": "dropped"},
        "categories": [
            {"name": "Commander", "includedInDeck": True, "isPremier": True},
            {"name": "Ramp", "includedInDeck": True, "isPremier": False},
            {"name": "Maybeboard", "includedInDeck": False, "isPremier": False},
            {"name": "Sideboard", "includedInDeck": False, "isPremier": False},
        ],
        "cards": [
            {"quantity": 1, "categories": ["Commander"], "card": {"oracleCard": {"name": "Tymna the Weaver"}}},
            {"quantity": 1, "categories": ["Commander"], "card": {"oracleCard": {"name": "Thrasios, Triton Hero"}}},
            {"quantity": 1, "categories": ["Ramp"], "card": {"oracleCard": {"name": "Sol Ring"}}},
            {"quantity": 1, "categories": ["Maybeboard", "Ramp"], "card": {"oracleCard": {"name": "Mana Vault"}}},
            {"quantity": 1, "categories": ["Ramp", "Sideboard"], "card": {"oracleCard": {"name": "Mana Crypt"}}},
            {"quantity": 3, "categories": None, "card": {"oracleCard": {"name": "Forest"}}},
            {"quantity": 1, "categories": [], "card": {"oracleCard": {"name": "Forest"}}},
        ],
    }

    def test_trim_drops_owner_and_keeps_names(self):
        t = edh_decks.trim_archidekt(self.RAW)
        self.assertNotIn("owner", json.dumps(t))
        self.assertEqual([c["name"] for c in t["cards"]][:3], ["Tymna the Weaver", "Thrasios, Triton Hero", "Sol Ring"])

    def test_deck_drops_excluded_categories_and_finds_partners(self):
        d = edh_decks.archidekt_deck(edh_decks.trim_archidekt(self.RAW))
        self.assertEqual(d["commander"], "Tymna the Weaver")
        self.assertEqual(d["commanders"], ["Tymna the Weaver", "Thrasios, Triton Hero"])
        self.assertEqual([(c["name"], c["count"]) for c in d["cards"]],
                         [("Tymna the Weaver", 1), ("Thrasios, Triton Hero", 1), ("Sol Ring", 1), ("Forest", 4)])
        self.assertIn("https://archidekt.com/decks/7", d["notes"])

    def test_search_url(self):
        self.assertEqual(edh_decks.archidekt_search_url(2, 4),
                         "https://archidekt.com/api/decks/v3/?deckFormat=3&orderBy=-viewCount&pageSize=60&page=2&edhBracket=4")


class FetcherTest(unittest.TestCase):
    def test_backoff_on_429_then_success(self):
        calls, sleeps = [], []

        def opener(req, timeout):
            calls.append(req.get_header("User-agent"))
            if len(calls) == 1:
                raise urllib.error.HTTPError(req.full_url, 429, "slow down", {"Retry-After": "7"}, None)
            return io.BytesIO(b'{"ok": 1}')

        f = edh_decks.Fetcher(interval=0, opener=opener, sleep=sleeps.append)
        self.assertEqual(f.get_json("https://example.invalid/x"), {"ok": 1})
        self.assertEqual(calls, [edh_decks.USER_AGENT] * 2)
        self.assertEqual(sleeps, [7.0])

    def test_403_stops(self):
        def opener(req, timeout):
            raise urllib.error.HTTPError(req.full_url, 403, "no", {}, None)
        with self.assertRaises(SystemExit):
            edh_decks.Fetcher(interval=0, opener=opener, sleep=lambda s: None).get_json("https://example.invalid/x")


class ManifestTest(unittest.TestCase):
    def test_normalise_writes_decks_and_manifest(self):
        with tempfile.TemporaryDirectory() as d:
            raw = os.path.join(d, "raw")
            dl = {"meta": {"date": "2099-01-01", "version": "v"}, "data": [
                {"type": "Commander Deck", "code": "C99", "fileName": "D_C99", "name": "D\tTab", "releaseDate": "2099-01-01"}]}
            edh_decks.write_json(os.path.join(raw, "mtgjson", "DeckList.json"), dl)
            edh_decks.write_json(os.path.join(raw, "mtgjson", "decks", "D_C99.json"),
                                 {"data": {"commander": [{"name": "C", "count": 1}], "mainBoard": [{"name": "M", "count": 99}]}})
            edh_decks.write_json(os.path.join(raw, "archidekt", "ranks.json"), {
                "fetched": "2099-01-02", "overall": [{"id": 7, "name": "Deck", "views": 10, "bracket": 3}],
                "brackets": {"3": [{"id": 7, "name": "Deck", "views": 10, "bracket": 3}]}})
            edh_decks.write_json(os.path.join(raw, "archidekt", "decks", "7.json"),
                                 edh_decks.trim_archidekt(ArchidektTest.RAW))
            man = os.path.join(d, "m.tsv")
            dls = os.path.join(d, "decks.jsonl")
            edh_decks.normalise(type("A", (), {"out": d, "manifest": man, "decklists": dls})())
            with open(dls) as f:
                lists = [json.loads(ln) for ln in f]
            self.assertEqual(len(lists), 2)
            self.assertEqual((lists[0]["source"], lists[0]["id"], lists[0]["commanders"]), ("mtgjson", "D_C99", ["C"]))
            self.assertEqual(lists[0]["cards"], ["M"] * 99)  # the commander is not among the 99
            self.assertEqual(lists[1]["source"], "archidekt")
            self.assertTrue(os.path.exists(os.path.join(d, "precon", "D_C99.json")))
            self.assertTrue(os.path.exists(os.path.join(d, "popular", "archidekt-7.json")))
            with open(man) as f:
                lines = f.read().splitlines()
            self.assertEqual(lines[0].split("\t"), edh_decks.MANIFEST_HEADER)
            self.assertEqual(len(lines), 3)
            for ln in lines:
                self.assertEqual(len(ln.split("\t")), len(edh_decks.MANIFEST_HEADER))
            pre = dict(zip(edh_decks.MANIFEST_HEADER, lines[1].split("\t")))
            self.assertEqual((pre["name"], pre["cards"], pre["kind"]), ("D Tab", "100", "precon"))
            pop = dict(zip(edh_decks.MANIFEST_HEADER, lines[2].split("\t")))
            self.assertEqual((pop["bracket"], pop["overall_rank"], pop["bracket_rank"], pop["cards"]), ("3", "1", "1", "7"))
            self.assertEqual(pop["commander"], "Tymna the Weaver / Thrasios, Triton Hero")


if __name__ == "__main__":
    unittest.main()
