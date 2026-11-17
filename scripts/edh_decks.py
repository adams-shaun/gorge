#!/usr/bin/env python3
"""edh_decks.py -- fetch Commander decklists into gorge's repo-deck JSON shape.

Two corpora, both written under a gitignored directory (default
.edh-decks), never committed: the decklists are third-party content.

  precon/   every official Commander precon MTGJSON lists (DeckList.json type
            "Commander Deck", every year), plus Commander's Arsenal and the
            Commander Collection box sets (kind "collection": card pools, not
            100-card decks). One file per MTGJSON deck file.
  popular/  Archidekt's public Commander decks by view count: the overall top
            1000 (so top 100/500/1000 are prefixes) and the top 100 of each
            Commander bracket 1-5 (Archidekt's edhBracket field).

Usage (stdlib only, needs network):

    python3 -I scripts/edh_decks.py fetch [-out .edh-decks] [-top 1000] [-per-bracket 100]
    python3 -I scripts/edh_decks.py normalise [-out .edh-decks]

`fetch` caches every raw response under <out>/raw/ (a deck already cached is
never fetched again) and then runs `normalise`, which rewrites precon/,
popular/ and the committed manifest (cmd/forgec/testdata/edh-manifest.tsv by
default: ids, names, bracket, rank, source URL and fetch date, no decklists).
Requests are paced (-interval seconds apart) with exponential backoff on 429
and 5xx, honouring Retry-After. The User-Agent is generic.

`go run ./cmd/forgec edhcover` measures gorge's coverage over the output.

Tests: python3 -m unittest discover -s scripts -p 'edh_decks_test.py'
"""

import argparse
import datetime
import json
import os
import re
import sys
import time
import urllib.error
import urllib.request

USER_AGENT = "gorge-deckfetch/1.0"
MTGJSON = "https://mtgjson.com/api/v5"
ARCHIDEKT = "https://archidekt.com/api"
ARCHIDEKT_EDH_FORMAT = 3

# Box sets that are Commander products but not "Commander Deck" in MTGJSON.
COLLECTION_CODES = {"CM1", "CC1", "CC2"}

MANIFEST_HEADER = ["source", "id", "kind", "name", "commander", "bracket",
                   "overall_rank", "bracket_rank", "views", "cards", "released",
                   "url", "fetched"]


# ---- network -----------------------------------------------------------------

class Fetcher:
    def __init__(self, interval=1.0, tries=6, opener=None, sleep=time.sleep):
        self.interval = interval
        self.tries = tries
        self.opener = opener or urllib.request.urlopen
        self.sleep = sleep
        self.last = 0.0
        self.requests = 0

    def get_json(self, url):
        delay = 2.0
        for attempt in range(self.tries):
            wait = self.last + self.interval - time.monotonic()
            if wait > 0:
                self.sleep(wait)
            self.last = time.monotonic()
            self.requests += 1
            req = urllib.request.Request(url, headers={"User-Agent": USER_AGENT,
                                                       "Accept": "application/json"})
            try:
                with self.opener(req, timeout=60) as resp:
                    return json.load(resp)
            except urllib.error.HTTPError as e:
                if e.code == 404:
                    return None
                if e.code in (401, 403):
                    raise SystemExit(f"{url}: HTTP {e.code} -- the source refuses automated "
                                     "access; not working around it")
                if e.code != 429 and e.code < 500:
                    raise
                ra = e.headers.get("Retry-After") if e.headers else None
                back = float(ra) if ra and ra.isdigit() else delay
            except (urllib.error.URLError, TimeoutError, ConnectionError):
                back = delay
            print(f"  retry {attempt + 1}/{self.tries} in {back:.0f}s: {url}", file=sys.stderr)
            self.sleep(back)
            delay = min(delay * 2, 120)
        raise SystemExit(f"{url}: gave up after {self.tries} tries")


def cached(path, fetch):
    if os.path.exists(path):
        with open(path) as f:
            return json.load(f)
    data = fetch()
    if data is None:
        return None
    os.makedirs(os.path.dirname(path), exist_ok=True)
    tmp = path + ".tmp"
    with open(tmp, "w") as f:
        json.dump(data, f, separators=(",", ":"), sort_keys=True)
    os.replace(tmp, path)
    return data


# ---- MTGJSON precons -------------------------------------------------------------

def precon_entries(decklist):
    """The DeckList.json rows this corpus keeps, as (row, kind)."""
    out = []
    for row in decklist["data"]:
        if row["type"] == "Commander Deck":
            out.append((row, "precon"))
        elif (row["type"] == "Box Set" and row["code"] in COLLECTION_CODES
              and not re.search(r"Premium|Oversized", row["name"])):
            out.append((row, "collection"))
    out.sort(key=lambda rk: (rk[0]["releaseDate"], rk[0]["fileName"]))
    return out


def mtgjson_deck(data, row, kind):
    """One MTGJSON deck file as a repo deck dict."""
    d = data["data"]
    commanders = [c["name"] for c in d.get("commander") or []]
    if not commanders and kind == "precon":
        commanders = [c["name"] for c in d.get("displayCommander") or []]
    cards = merge([(c["name"], c.get("count", 1))
                   for c in (d.get("commander") or []) + (d.get("mainBoard") or [])])
    return deck_file(row["name"], commanders, cards,
                     f"MTGJSON {kind} {row['fileName']} ({row['code']}, {row['releaseDate']}), "
                     f"{MTGJSON}/decks/{row['fileName']}.json")


# ---- Archidekt popular decks -------------------------------------------------------

def archidekt_search_url(page, bracket=None, page_size=60):
    u = (f"{ARCHIDEKT}/decks/v3/?deckFormat={ARCHIDEKT_EDH_FORMAT}&orderBy=-viewCount"
         f"&pageSize={page_size}&page={page}")
    if bracket is not None:
        u += f"&edhBracket={bracket}"
    return u


def archidekt_ranked(fetcher, want, bracket=None):
    """The first `want` distinct public deck ids by view count, with summaries."""
    out, seen, page = [], set(), 1
    while len(out) < want:
        data = fetcher.get_json(archidekt_search_url(page, bracket))
        results = (data or {}).get("results") or []
        if not results:
            break
        for r in results:
            if r["id"] in seen or r.get("private") or r.get("unlisted"):
                continue
            if bracket is not None and r.get("edhBracket") != bracket:
                continue
            seen.add(r["id"])
            out.append({"id": r["id"], "name": r["name"], "views": r.get("viewCount", 0),
                        "bracket": r.get("edhBracket")})
            if len(out) == want:
                break
        if not data.get("next"):
            break
        page += 1
    return out


def trim_archidekt(d):
    """The fields normalise reads; raw decks are ~200 KB, this keeps ~10."""
    return {
        "id": d["id"], "name": d["name"], "edhBracket": d.get("edhBracket"),
        "viewCount": d.get("viewCount"), "deckFormat": d.get("deckFormat"),
        "categories": [{"name": c["name"], "includedInDeck": c.get("includedInDeck", True),
                        "isPremier": c.get("isPremier", False)} for c in d.get("categories") or []],
        "cards": [{"name": c["card"]["oracleCard"]["name"], "quantity": c.get("quantity", 1),
                   "categories": c.get("categories") or []} for c in d.get("cards") or []],
    }


def archidekt_deck(t):
    """A trimmed Archidekt deck as a repo deck dict (maybeboard/sideboard dropped)."""
    excluded = {c["name"] for c in t["categories"] if not c["includedInDeck"]}
    premier = {c["name"] for c in t["categories"] if c["isPremier"]}
    pairs, commanders = [], []
    for c in t["cards"]:
        cats = c["categories"]
        if any(x in excluded for x in cats):
            continue
        pairs.append((c["name"], c["quantity"]))
        if any(x in premier for x in cats):
            commanders.append(c["name"])
    return deck_file(t["name"], commanders, merge(pairs),
                     f"Archidekt deck {t['id']}, https://archidekt.com/decks/{t['id']}")


# ---- shared normalisation -------------------------------------------------------------

def merge(pairs):
    """Merge (name, count) pairs by name, keeping first-seen order."""
    idx, out = {}, []
    for name, n in pairs:
        name = " ".join(name.split())
        if name in idx:
            out[idx[name]][1] += n
        else:
            idx[name] = len(out)
            out.append([name, n])
    return out


def deck_file(name, commanders, cards, notes):
    d = {"name": name, "format": "commander", "archetype": "", "notes": notes}
    commanders = list(dict.fromkeys(commanders))
    if len(commanders) == 1:
        d["commander"] = commanders[0]
    elif len(commanders) > 1:
        d["commander"] = commanders[0]
        d["commanders"] = commanders
    d["cards"] = [{"name": n, "count": c} for n, c in cards]
    return d


def slug(s):
    return re.sub(r"[^a-z0-9]+", "-", s.lower()).strip("-") or "deck"


def tsv(v):
    return "" if v is None else re.sub(r"[\t\r\n]+", " ", str(v))


def load(path):
    with open(path) as f:
        return json.load(f)


def write_json(path, obj):
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w") as f:
        json.dump(obj, f, indent=1, ensure_ascii=False)
        f.write("\n")


# ---- commands --------------------------------------------------------------------------

def fetch(args):
    f = Fetcher(interval=args.interval)
    raw = os.path.join(args.out, "raw")
    today = datetime.date.today().isoformat()
    dl = cached(os.path.join(raw, "mtgjson", "DeckList.json"),
                lambda: f.get_json(f"{MTGJSON}/DeckList.json"))
    entries = precon_entries(dl)
    print(f"mtgjson: {len(entries)} commander decks listed ({dl['meta']['version']})")
    for row, _ in entries:
        cached(os.path.join(raw, "mtgjson", "decks", row["fileName"] + ".json"),
               lambda row=row: f.get_json(f"{MTGJSON}/decks/{row['fileName']}.json"))
    ranks = {"fetched": today, "overall": archidekt_ranked(f, args.top), "brackets": {}}
    for b in range(1, 6):
        ranks["brackets"][str(b)] = archidekt_ranked(f, args.per_bracket, b)
    print(f"archidekt: overall {len(ranks['overall'])}, brackets " +
          " ".join(f"{b}:{len(v)}" for b, v in ranks["brackets"].items()))
    write_json(os.path.join(raw, "archidekt", "ranks.json"), ranks)
    ids = {r["id"] for r in ranks["overall"]}
    for v in ranks["brackets"].values():
        ids |= {r["id"] for r in v}
    for n, i in enumerate(sorted(ids)):
        cached(os.path.join(raw, "archidekt", "decks", f"{i}.json"),
               lambda i=i: (lambda d: d and trim_archidekt(d))(f.get_json(f"{ARCHIDEKT}/decks/{i}/")))
        if n % 100 == 99:
            print(f"  archidekt decks {n + 1}/{len(ids)} ({f.requests} requests)")
    print(f"fetch: {f.requests} requests")
    normalise(args)


def normalise(args):
    raw = os.path.join(args.out, "raw")
    rows = []
    dl = load(os.path.join(raw, "mtgjson", "DeckList.json"))
    fetched = dl["meta"]["date"]
    entries = precon_entries(dl)
    missing = []
    for row, kind in entries:
        p = os.path.join(raw, "mtgjson", "decks", row["fileName"] + ".json")
        if not os.path.exists(p):
            missing.append(row["fileName"])
            continue
        d = mtgjson_deck(load(p), row, kind)
        write_json(os.path.join(args.out, "precon", row["fileName"] + ".json"), d)
        rows.append(["mtgjson", row["fileName"], kind, row["name"], " / ".join(d.get("commanders") or [d.get("commander", "")]),
                     "", "", "", "", sum(c["count"] for c in d["cards"]), row["releaseDate"],
                     f"{MTGJSON}/decks/{row['fileName']}.json", fetched])
    print(f"precon: {len(entries) - len(missing)} of {len(entries)} listed deck files normalised")
    if missing:
        print("  missing: " + ", ".join(missing))
    rp = os.path.join(raw, "archidekt", "ranks.json")
    if os.path.exists(rp):
        ranks = load(rp)
        overall = {r["id"]: n + 1 for n, r in enumerate(ranks["overall"])}
        bracket_rank = {}
        for v in ranks["brackets"].values():
            for n, r in enumerate(v):
                bracket_rank[r["id"]] = n + 1
        summary = {r["id"]: r for r in ranks["overall"]}
        for v in ranks["brackets"].values():
            summary.update({r["id"]: r for r in v})
        for i in sorted(summary, key=lambda i: (overall.get(i, 10**9), bracket_rank.get(i, 0), i)):
            p = os.path.join(raw, "archidekt", "decks", f"{i}.json")
            if not os.path.exists(p):
                continue
            t = load(p)
            d = archidekt_deck(t)
            write_json(os.path.join(args.out, "popular", f"archidekt-{i}.json"), d)
            rows.append(["archidekt", i, "popular", t["name"], " / ".join(d.get("commanders") or [d.get("commander", "")]),
                         t.get("edhBracket") or "", overall.get(i, ""), bracket_rank.get(i, ""),
                         t.get("viewCount") or summary[i]["views"], sum(c["count"] for c in d["cards"]), "",
                         f"https://archidekt.com/decks/{i}", ranks["fetched"]])
    with open(args.manifest, "w") as f:
        f.write("\t".join(MANIFEST_HEADER) + "\n")
        for r in rows:
            f.write("\t".join(tsv(v) for v in r) + "\n")
    print(f"manifest: {len(rows)} decks -> {args.manifest}")


def main(argv=None):
    ap = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    ap.add_argument("cmd", choices=["fetch", "normalise"])
    ap.add_argument("-out", default=".edh-decks")
    ap.add_argument("-manifest", default="cmd/forgec/testdata/edh-manifest.tsv")
    ap.add_argument("-top", type=int, default=1000)
    ap.add_argument("-per-bracket", dest="per_bracket", type=int, default=100)
    ap.add_argument("-interval", type=float, default=1.0)
    args = ap.parse_args(argv)
    (fetch if args.cmd == "fetch" else normalise)(args)


if __name__ == "__main__":
    main()
