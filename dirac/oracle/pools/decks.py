"""Build a lockstep deck pool for kfuzz -kernel <set> from 17lands (or from gorge deck JSONs).

The FDN pool (<pools>/fdn, where <pools> is $GORGE_POOLS or the module's .pools) is DraftZero's recipe: every
17lands FDN PremierDraft maindeck whose player is in a win-rate bucket >= 0.60, one deck per
(draft_id, build_index), sorted by (draft_time, draft_id, build_index), stem
FDN_top_<1-based index>_<main colours>[p<splash>]. This applies the same recipe to
game_data_public.<SET>.PremierDraft.csv.gz (17lands public data, CC BY 4.0).

A set's kernel table rarely supports every card (BLB: 104 of 276; every one of the 37,839 decks
holds at least two refused cards), so a strict pool is empty. The pool this writes keeps a deck
only when at most --max-refused of its --deck-size (default 40) maindeck copies are refused cards, and replaces each of
those copies with a basic land in proportion to the deck's own basics (largest remainder, WUBRG
on ties). Basic lands are free in limited, so the result is a deck the drafter could have
registered from the same draft. A deck's notes record what was replaced. --max-refused 0 is the
strict pool.

  curl -o <dir>/game_data_public.<SET>.PremierDraft.csv.gz \
      https://17lands-public.s3.amazonaws.com/analysis_data/game_data/game_data_public.{SET}.PremierDraft.csv.gz
  python3 dirac/oracle/pools/decks.py --set <SET> <dir>/game_data_public.<SET>.PremierDraft.csv.gz \
      <pools>/<set>

--from-decks keeps a Commander deck's commanders (a draftsim -decks-out deck's "commanders" list; a deck whose
commander is refused is skipped). --four-seat also replaces the cards the kernel plays at two seats only, which
stall a four-seat pod at its start (the mbc pool: draftsim -set MBC -collation draft -format commander
-deck-size 50 -structure none -decks-out DIR, then --from-decks DIR --deck-size 50 --allow-unverified --four-seat).

--emit-cards PATH writes the sorted card names of the CSV header (the kgen -card-list input for a
new set) and exits. --from-decks DIR applies the same rule to gorge deck JSONs in DIR instead of a
CSV (sim pools); pass --label sim, or --label mtggoldfish for the scraped Standard decks. --min-deck-size N
keeps a deck of at least N cards instead of exactly --deck-size (Standard's 60 minimum). With --emit-cards,
--from-decks writes the sorted union of the decks' card names.

Writes <out>/decks/*.json (gorge deck JSON), <out>/pool.tsv (every tenth deck is eval),
<out>/summary.txt (the refusal census over every extracted deck) and <out>/pool.manifest (the
recipe and the tree digest dirac/oracle/pools.go pins).
"""
import argparse
import collections
import csv
import gzip
import hashlib
import json
import os
import re
import subprocess
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
REPO = os.path.join(HERE, "..", "..", "..")
BASICS = ["Plains", "Island", "Swamp", "Mountain", "Forest"]
BASIC_OF = dict(zip("WUBRG", BASICS))


def tree_digest(root):
    """The digest dirac/oracle/pools.go pins: `sha256sum` lines over decks/** and pool.tsv, byte-sorted.

    Equals: find ./decks ./pool.tsv -type f | LC_ALL=C sort | xargs -d '\\n' sha256sum | sha256sum"""
    return digest_of(root, ["pool.tsv"] + files_under(root, "decks"))


def dir_digest(root):
    """The same lines over every file under root except a top-level pool.manifest (a --from-decks source)."""
    return digest_of(root, [r for r in files_under(root, ".") if r != "pool.manifest"])


def files_under(root, sub):
    rels = []
    for dp, _, fs in os.walk(os.path.join(root, sub)):
        for f in fs:
            full = os.path.join(dp, f)
            if os.path.islink(full) or not os.path.isfile(full):
                continue
            rel = os.path.relpath(full, root).replace(os.sep, "/")
            if "\\" in rel or "\n" in rel:
                sys.exit(f"{rel!r}: backslash or newline in a path")
            rels.append(rel)
    return rels


def digest_of(root, rels):
    out = hashlib.sha256()
    for rel in sorted(rels, key=lambda r: r.encode()):
        h = hashlib.sha256(open(os.path.join(root, rel), "rb").read()).hexdigest()
        out.update(f"{h}  ./{rel}\n".encode())
    return out.hexdigest()


def file_sha256(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for b in iter(lambda: f.read(1 << 20), b""):
            h.update(b)
    return h.hexdigest()


def supported(pool, unverified=False, four_seat=False):
    """card name -> supported, from the pool's binary table (make dirac-tables)."""
    env = dict(os.environ, GOWORK="off")
    out = subprocess.run(["go", "run", "-trimpath", "./dirac/cmd/kgen", "-supported", pool] + (["-unverified"] if unverified else [])
                         + (["-four-seat"] if four_seat else []), cwd=REPO, env=env, capture_output=True, text=True)
    if out.returncode != 0:
        sys.exit(f"kgen -supported {pool}: {out.stderr.strip()}")
    return {n: s == "true" for n, s in (line.split("\t") for line in out.stdout.splitlines())}


def bucket(v):
    try:
        return float(v)
    except (TypeError, ValueError):
        return None


def emit_deck_cards(dirpath, dest):
    names = sorted({c for d in from_decks(dirpath) for c in d["cards"]}, key=lambda n: n.encode())
    with open(dest, "w") as g:
        g.write("".join(n + "\n" for n in names))
    print(f"{len(names)} cards -> {dest}")


def emit_cards(path, dest):
    with gzip.open(path, "rt", newline="") as f:
        header = next(csv.reader(f))
    names = sorted((n[len("deck_"):] for n in header if n.startswith("deck_")), key=lambda n: n.encode())
    with open(dest, "w") as g:
        g.write("".join(n + "\n" for n in names))
    print(f"{len(names)} cards -> {dest}")


def from_decks(dirpath):
    """Gorge deck JSONs in dirpath as extract()-shaped decks, in file-name order."""
    out = []
    for fn in sorted(os.listdir(dirpath), key=lambda n: n.encode()):
        if not fn.endswith(".json"):
            continue
        j = json.load(open(os.path.join(dirpath, fn)))
        cards = {}
        for c in j["cards"]:
            cards[c["name"]] = cards.get(c["name"], 0) + c["count"]
        out.append({"name": j["name"], "notes": j.get("notes", ""), "main_colors": j.get("main_colors", "C"),
                    "splash_colors": "", "format": j.get("format", "limited"), "commander": j.get("commander", ""),
                    "commanders": j.get("commanders") or [], "maindeck_size": sum(cards.values()), "cards": dict(sorted(cards.items()))})
    return out


def extract(path, min_winrate):
    """DraftZero's tools/extract_decks.py extract() (d17ad0a), min-player-games 0."""
    csv.field_size_limit(sys.maxsize)
    decks = {}
    with gzip.open(path, "rt", newline="") as f:
        reader = csv.reader(f)
        header = next(reader)
        col = {name: i for i, name in enumerate(header)}
        deck_cols = [(i, name[len("deck_"):]) for i, name in enumerate(header) if name.startswith("deck_")]
        for row in reader:
            wr = bucket(row[col["user_game_win_rate_bucket"]])
            if wr is None or wr < min_winrate:
                continue
            key = (row[col["draft_id"]], row[col["build_index"]])
            if key in decks:
                continue
            cards = {name: int(row[i]) for i, name in deck_cols if row[i] not in ("", "0")}
            decks[key] = {
                "draft_id": key[0], "build_index": int(key[1]), "draft_time": row[col["draft_time"]],
                "main_colors": row[col["main_colors"]], "splash_colors": row[col["splash_colors"]],
                "user_game_win_rate_bucket": wr, "maindeck_size": sum(cards.values()),
                "cards": dict(sorted(cards.items())),
            }
    out = list(decks.values())
    out.sort(key=lambda d: (d["draft_time"], d["draft_id"], d["build_index"]))
    return out


def stem(SET, i, d):
    colors = d["main_colors"] + (f"+{d['splash_colors']}" if d["splash_colors"] else "")
    return f"{SET}_top_{i:05d}_{(colors or 'C').replace('+', 'p')}"


def fill_basics(cards, n):
    """n basic lands split as the deck's own basics (largest remainder, WUBRG on ties)."""
    have = {b: cards.get(b, 0) for b in BASICS}
    total = sum(have.values())
    if total == 0:
        return None
    exact = {b: n * have[b] / total for b in BASICS}
    add = {b: int(exact[b]) for b in BASICS}
    rest = sorted(BASICS, key=lambda b: (-(exact[b] - add[b]), BASICS.index(b)))
    for b in rest[: n - sum(add.values())]:
        add[b] += 1
    return add


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("paths", nargs="*", help="<csv> <out>; with --emit-cards <csv>; with --from-decks <out>")
    ap.add_argument("--set", required=True, help="upper-case set code, e.g. BLB")
    ap.add_argument("--min-winrate", type=float, default=0.60)
    ap.add_argument("--max-refused", type=int, default=6, help="refused maindeck copies a deck may replace with basics")
    ap.add_argument("--deck-size", type=int, default=40, help="maindeck cards a deck must hold (100 for Commander)")
    ap.add_argument("--min-deck-size", type=int, default=0, help="maindeck cards a deck must hold at least (60 for Standard); replaces the exact --deck-size check")
    ap.add_argument("--allow-unverified", action="store_true", help="count cards without lockstep evidence as playable (kfuzz -allow-unverified); a refused card stays refused")
    ap.add_argument("--four-seat", action="store_true", help="count a card the kernel plays at two seats only as refused (kgen -supported -four-seat): a pool for kfuzz -seats 4")
    ap.add_argument("--emit-cards", metavar="PATH", help="write the CSV's card names (kgen -card-list input) and exit")
    ap.add_argument("--from-decks", metavar="DIR", help="gorge deck JSONs instead of a CSV")
    ap.add_argument("--label", choices=["17lands", "sim", "mtggoldfish"], default="17lands")
    args = ap.parse_args()
    SET = args.set
    if SET != SET.upper() or not SET:
        ap.error("--set must be upper-case")
    if args.emit_cards:
        if args.from_decks:
            if args.paths:
                ap.error("--emit-cards with --from-decks takes no other path")
            emit_deck_cards(args.from_decks, args.emit_cards)
            return
        if len(args.paths) != 1:
            ap.error("--emit-cards takes the CSV only")
        emit_cards(args.paths[0], args.emit_cards)
        return
    if args.from_decks:
        if len(args.paths) != 1:
            ap.error("--from-decks takes the output directory only")
        source, outdir = args.from_decks, args.paths[0]
    else:
        if len(args.paths) != 2:
            ap.error("want <csv> <out>")
        source, outdir = args.paths

    sup = supported(SET.lower(), args.allow_unverified, args.four_seat)
    decks = from_decks(args.from_decks) if args.from_decks else extract(source, args.min_winrate)
    os.makedirs(os.path.join(outdir, "decks"), exist_ok=True)
    in_decks = collections.Counter()
    strict = 0
    by_refused = collections.Counter()
    skipped = collections.Counter()
    rows = []
    for i, d in enumerate(decks, 1):
        unknown = [c for c in d["cards"] if c not in sup]
        if unknown:
            skipped[f"card not in the {SET} pool"] += 1
            continue
        bad = {c: n for c, n in d["cards"].items() if not sup[c]}
        for c in bad:
            in_decks[c] += 1
        k = sum(bad.values())
        by_refused[k] += 1
        if k == 0:
            strict += 1
        if args.min_deck_size:
            if d["maindeck_size"] < args.min_deck_size:
                skipped[f"maindeck under {args.min_deck_size} cards"] += 1
                continue
        elif d["maindeck_size"] != args.deck_size:
            skipped[f"maindeck not {args.deck_size} cards"] += 1
            continue
        if k > args.max_refused:
            skipped[f"more than {args.max_refused} refused copies"] += 1
            continue
        if any(not sup.get(c, False) for c in [d.get("commander")] + d.get("commanders", []) if c):
            skipped["commander refused"] += 1
            continue
        cards = {c: n for c, n in d["cards"].items() if sup[c]}
        add = fill_basics(cards, k)
        if add is None:
            skipped["refused copies and no basics to replace them"] += 1
            continue
        for b, n in add.items():
            if n:
                cards[b] = cards.get(b, 0) + n
        if args.from_decks:
            s = d["name"]
            note = d["notes"]
        else:
            s = stem(SET, i, d)
            note = f"17lands {SET} PremierDraft deck {d['draft_id']}:{d['build_index']}, player win-rate bucket " \
                   f"{d['user_game_win_rate_bucket']:.2f} (CC BY 4.0)"
        if bad:
            note += ("; " if note else "") + "kernel-refused copies replaced by basics: " + ", ".join(f"{n} {c}" for c, n in sorted(bad.items())) \
                + " -> " + ", ".join(f"{n} {b}" for b, n in add.items() if n)
        deck = {"name": s, "format": d.get("format", "limited"), "notes": note}
        if d.get("commander"):
            deck["commander"] = d["commander"]
        if d.get("commanders"):
            deck["commanders"] = d["commanders"]  # a Commander draft's commander (and partner), draftsim -decks-out
        deck.update({"cards": [{"name": c, "count": n} for c, n in sorted(cards.items())]})
        with open(os.path.join(outdir, "decks", s + ".json"), "w") as g:
            json.dump(deck, g, indent=0)
        rows.append((s, d["main_colors"] + d["splash_colors"], d["main_colors"], sum(cards.values())))
    with open(os.path.join(outdir, "pool.tsv"), "w") as f:
        f.write("deck\tsplit\tcolors\tmain_colors\tcards\n")
        for j, (s, c, m, n) in enumerate(rows):
            f.write(f"{s}\t{'eval' if j % 10 == 9 else 'train'}\t{c}\t{m}\t{n}\n")
    refused = sorted(c for c, ok in sup.items() if not ok)
    lines = [
        f"{SET} PremierDraft, win-rate bucket >= {args.min_winrate:.2f}: {len(decks):,} decks",
        f"{SET.lower()} table: {sum(sup.values())} of {len(sup)} pool entries supported (tokens included); {len(refused)} refused",
        f"fully supported decks (strict pool): {strict:,}",
        f"pool written: {len(rows):,} decks (at most {args.max_refused} refused copies, replaced by basics)",
        "skipped: " + ", ".join(f"{r}: {n:,}" for r, n in sorted(skipped.items())),
        "decks by refused maindeck copies: " + ", ".join(f"{k}: {n:,}" for k, n in sorted(by_refused.items())),
        "", "refused cards by the number of decks holding them (a deck holding any is not strict):",
    ]
    lines += [f"  {in_decks[c]:7,}  {c}" for c in sorted(refused, key=lambda c: (-in_decks[c], c))]
    with open(os.path.join(outdir, "summary.txt"), "w") as f:
        f.write("\n".join(lines) + "\n")
    src_sha = dir_digest(source) if args.from_decks else file_sha256(source)
    manifest = [
        ("set", SET), ("label", args.label),
        ("source", source), ("source_sha256", src_sha),
        ("recipe", f"min_winrate={args.min_winrate:.2f} max_refused={args.max_refused}" + (" four_seat" if args.four_seat else "")),
        ("pool_cards_sha256", file_sha256(os.path.join(REPO, "dirac", "cardset", "pools", SET.lower(), "cards.txt"))),
        ("pool_supported", f"{sum(sup.values())} of {len(sup)}"),
        ("decks", str(len(rows))), ("tree_sha256", tree_digest(outdir)),
    ]
    with open(os.path.join(outdir, "pool.manifest"), "w") as f:
        f.write("".join(f"{k}: {v}\n" for k, v in manifest))
    print("\n".join(lines[:6]))


if __name__ == "__main__":
    main()
