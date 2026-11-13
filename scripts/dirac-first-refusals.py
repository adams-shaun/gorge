#!/usr/bin/env python3
"""Refuse by name what the evidence corpus still shows diverging, and write the lists.

  dirac-first-refusals.py settle <out> [--rounds N] [--resume]  settle the refused list against the corpus
  dirac-first-refusals.py evidence <out> [--pools P ...]        write dirac/support/evidence from <out>
  dirac-first-refusals.py report <out>                          counts by run, surface and refused class

The caller caps the process: settle runs scripts/dirac-evidence.sh.

A game is a pure function of its pair: a refusal takes pairs out of a pool and changes none of the
others. So one run with the refused cards in (PLAY_REFUSED) names every bad game of the first 1500
pairs, and the list is worked out from those games alone, whatever list the tree held before:

  - each bad game holds a refused card, so none can be played; and
  - each refused card is implicated by a bad game and is the only refusal keeping some bad game out.

A bad game is answered by one of its implicated cards, else by the sources of its latest actions when
it names none (an unmodelled stall, a token). A card that plays clean all through the real deck pools
is taken as a victim while the game names any other: refusing it costs the decks that hold it, and if
it really is the cause it comes back once nothing else is left to blame. The card answering the most
open games goes first, the ranking run again until none is open; then a refusal the games do not need
is taken back, the least implicated first. Many lists satisfy the two rules, and this order picks one.

A deck pool reads on past the pairs it turns away, so a list can bring it games the first run did not
play: more still in the gate's slices, which turn away unverified cards too. A round plays the whole
corpus as it will be judged, refused cards out, writes the evidence lists, plays those slices, adds any
new bad game and works the list out again. Settled means a round found none and every run passed.

An entry whose pool is outside the corpus stays as it is. So does one whose game still implicates its
card, which keeps a hand-written reason.
"""
import argparse
import collections
import glob
import json
import os
import re
import subprocess
import sys

BASICS = {"Plains", "Island", "Swamp", "Mountain", "Forest", "Wastes"}
EVIDENCE_MIN_PLAYS = 3  # oracle.EvidenceMinPlays
DECK_POOLS = ("fdn", "blb", "pauper")
DECK_CLEARED_PLAYS = 50
REFUSED_DIR = "dirac/support/refused"
EVIDENCE_DIR = "dirac/support/evidence"
EVIDENCE_GAMES = "dirac/support/evidence-games.tsv"
EVIDENCE_FORMS = ("autopay.m0", "autopay.m1", "full.m0", "full.m1")  # the four runs a pool owes
VERDICT = re.compile(
    r"^(\S+): (\d+) games \((\d+) refused-card, (\d+) refused-pair, (\d+) error\), (\d+) clean, "
    r"(\d+) divergent, (\d+) unmodelled \[.*?\], (\d+) capacity \[.*?\], (\d+) hang"
)


def root():
    return subprocess.check_output(["git", "rev-parse", "--show-toplevel"], text=True).strip()


def run_files(out, suffix):
    return sorted(glob.glob(os.path.join(out, f"*.m[01].{suffix}")))


def run_parts(path):
    pool, surface, m = os.path.basename(path).split(".")[:3]
    return pool, surface, int(m[1:])


def bad_games(out):
    """Every bad game of the runs in out, as dicts."""
    games = []
    for path in run_files(out, "out"):
        pool, surface, m = run_parts(path)
        with open(path) as f:
            for line in f:
                if line.startswith("bad-game\t"):
                    _, pair, _, outcome, cls, impl, recent, held = line.rstrip("\n").split("\t")
                    games.append(dict(pool=pool, surface=surface, m=m, pair=int(pair), outcome=outcome, cls=cls,
                                      impl=names_of(impl), recent=names_of(recent), held=set(names_of(held))))
    return games


def names_of(field):
    return [n for n in field.split("|") if n and n not in BASICS]


def deck_cleared(out):
    """The cards the real deck pools play at least DECK_CLEARED_PLAYS times in clean games."""
    plays = collections.Counter()
    for pool in DECK_POOLS:
        for path in glob.glob(os.path.join(out, pool + ".*.m[01].coverage")):
            with open(path) as f:
                next(f)
                for line in f:
                    card, cast, land = line.split("\t")[:3]
                    plays[card] += int(cast) + int(land)
    return {c for c, n in plays.items() if n >= DECK_CLEARED_PLAYS}


def blame(games, cleared):
    """Set each game's cards: who may answer it, first choice first. Only a card in its decks can."""
    for g in games:
        impl, recent = [c for c in g["impl"] if c in g["held"]], [c for c in g["recent"] if c in g["held"]]
        options = [[c for c in impl if c not in cleared], [c for c in recent if c not in cleared], impl, recent]
        g["cards"] = set(next((o for o in options if o), []))
    return games


def field_class(cls):
    return re.sub(r"\[\d+\]", "[]", cls)


def class_tag(g):
    if g["outcome"] != "divergent":
        return f"{g['outcome']}-{g['cls']}"
    what, _, field = field_class(g["cls"]).partition(" ")
    if what == "kernel":
        return "kernel"
    if what in ("options", "decision"):
        return what
    return f"{what}-{field}".replace("[]", "")


def why_of(g, n):
    what = field_class(g["cls"])
    games = f"; {n} games in the corpus" if n > 1 else ""
    if g["outcome"] == "unmodelled":
        return f"gorge does something the kernel does not model here (stall {g['cls']}){games}"
    if g["outcome"] == "hang":
        return f"a lockstep game with it never finished{games}"
    if what.startswith("options"):
        return f"the engines offer different options{games}"
    if what.startswith("decision"):
        return f"the engines pose different decisions{games}"
    return f"the engines' state differs at {what}{games}"


def settled(games, kept):
    """The refused list that answers games, and the games with no card to refuse."""
    refused = set(kept)
    count = collections.Counter(c for g in games for c in g["cards"])
    while True:
        open_games = [g for g in games if not g["held"] & refused]
        answers = collections.Counter(c for g in open_games for c in g["cards"])
        if not answers:
            break
        refused.add(min(answers, key=lambda c: (-answers[c], c)))
    hits = [g["held"] & refused for g in games]
    for card in sorted(refused - kept, key=lambda c: (count[c], c)):
        if not any(h == {card} for h in hits):
            refused.discard(card)
            for h in hits:
                h.discard(card)
    return refused, open_games


def slug(card):
    """cardset.SupportSlug."""
    return re.sub(r"[^a-z0-9]+", "-", card.lower()).strip("-")


def entry_path(top, card):
    return os.path.join(top, REFUSED_DIR, slug(card) + ".json")


def write_refused(top, games, refused, kept):
    """Write the list. An entry whose game still implicates its card stands; a new one names the game
    implicating it that holds the fewest other refusals."""
    for card in refused_names(top) - refused:
        os.remove(entry_path(top, card))
    witness = {}
    count = collections.Counter()
    for g in sorted(games, key=lambda g: (len(g["held"] & refused), game_key(g))):
        for c in g["cards"]:
            count[c] += 1
            witness.setdefault(c, g)
    implicating = {(c, game_key(g)) for g in games for c in g["cards"]}
    for card in refused - kept:
        if os.path.exists(entry_path(top, card)) and (card, entry_key(top, card)) in implicating:
            continue
        g = witness[card]
        entry = dict(card=card, **{"class": class_tag(g)}, why=why_of(g, count[card]),
                     repro=dict(pool=g["pool"], surface=g["surface"], seed=1, pair=g["pair"], policy="random", mulligans=g["m"]))
        with open(entry_path(top, card), "w") as f:
            json.dump(entry, f, indent=2, ensure_ascii=False)
            f.write("\n")


def evidence_run(top, out, pools, **env):
    env = dict(os.environ, POOLS=" ".join(pools), **env)
    r = subprocess.run(["bash", os.path.join(top, "scripts/dirac-evidence.sh"), out], cwd=top, env=env)
    if r.returncode == 2:
        sys.exit("the evidence run could not build")
    return r.returncode


def game_key(g):
    return g["pool"], g["surface"], g["m"], g["pair"]


def entry_key(top, card):
    with open(entry_path(top, card)) as f:
        r = json.load(f)["repro"]
    return r["pool"], r["surface"], r["mulligans"], r["pair"]


def settle(top, out, rounds, resume):
    subprocess.run(["make", "dirac-tables"], cwd=top, check=True, capture_output=True)
    pools = subprocess.check_output(["go", "run", "-trimpath", "./dirac/oracle/cmd/kfuzz", "-list-synth"], cwd=top, text=True)
    corpus = (*DECK_POOLS, *(p for p in pools.split() if p.startswith(("std-", "dr-"))))
    played_in, gate = os.path.join(out, "played-in"), os.path.join(out, "gate")
    if not resume:
        print(f"playing {len(corpus)} pools with the refused cards in", flush=True)
        evidence_run(top, played_in, corpus, PLAY_REFUSED="1")
    cleared = deck_cleared(played_in)
    known = {game_key(g): g for g in bad_games(played_in)}
    kept = {c for c in refused_names(top) if entry_key(top, c)[0] not in corpus}
    print(f"{len(known)} bad games, {len(kept)} refusals outside the corpus", flush=True)
    for n in range(rounds):
        games = blame(list(known.values()), cleared)
        refused, orphans = settled(games, kept)
        for g in orphans[:20]:
            print(f"  no card: {g['pool']} {g['surface']} m{g['m']} pair {g['pair']}: {g['outcome']} {g['cls']}")
        write_refused(top, games, refused, kept)
        print(f"round {n}: {len(refused)} refused, {len(orphans)} bad games with no card to refuse", flush=True)
        rc = evidence_run(top, out, corpus)
        write_evidence(top, out)
        rc |= evidence_run(top, gate, DECK_POOLS, GATE_SLICE="1")
        new = [g for g in bad_games(out) + bad_games(gate) if game_key(g) not in known]
        print(f"round {n}: {len(new)} new bad games", flush=True)
        if not new:
            print("settled: every run passed" if rc == 0 else "settled, and a run failed: read its .out")
            sys.exit(rc)
        known.update((game_key(g), g) for g in new)
    sys.exit(f"not settled after {rounds} rounds")


def merged_coverage(out, only=None):
    """card -> [plays, unused keys in every run, pools it was played in]. A pool with a run
    that did not pass counts for nothing, so an unfinished corpus leaves cards unverified.
    With only, count the named pools' own runs alone."""
    dirty = set()
    for path in run_files(out, "rc"):
        with open(path) as f:
            if f.read().strip() != "0":
                dirty.add(run_parts(path)[0])
    cards = {}
    for path in run_files(out, "coverage"):
        pool = run_parts(path)[0]
        if pool in dirty or (only is not None and pool not in only):
            continue
        with open(path) as f:
            next(f)
            for line in f:
                card, cast, land, _, _, unused = line.rstrip("\n").split("\t")
                plays = int(cast) + int(land)
                unused = set(filter(None, unused.split(",")))
                c = cards.setdefault(card, [0, unused, set()])
                c[0] += plays
                c[1] &= unused
                if plays:
                    c[2].add(pool)
    return cards


def refused_names(top):
    names = set()
    for path in glob.glob(os.path.join(top, REFUSED_DIR, "*.json")):
        with open(path) as f:
            names.add(json.load(f)["card"])
    return names


def require_pool_runs(out, pools):
    """Refuse, writing nothing, unless every named pool has the four expected runs (autopay and
    full, mulligans 0 and 1), each passed."""
    for pool in pools:
        rc = {}
        for path in run_files(out, "rc"):
            p, surface, m = run_parts(path)
            if p == pool:
                rc[f"{surface}.m{m}"] = path
        missing = [form for form in EVIDENCE_FORMS if form not in rc]
        if missing:
            refuse(f"{pool} is missing runs {' '.join(missing)}")
        failed = []
        for form in EVIDENCE_FORMS:
            with open(rc[form]) as f:
                if f.read().strip() != "0":
                    failed.append(form)
        if failed:
            refuse(f"{pool} runs {' '.join(failed)} did not pass")


def refuse(why):
    print(f"evidence: {why}; writing nothing", file=sys.stderr)
    sys.exit(1)


def existing_list(top, pool):
    path = os.path.join(top, EVIDENCE_DIR, pool + ".txt")
    if not os.path.exists(path):
        return []
    with open(path) as f:
        return [n for n in f.read().splitlines() if n]


def ran_pools(out):
    """The pools out has a run of; only these may the writer judge."""
    return {run_parts(p)[0] for suffix in ("rc", "coverage", "out") for p in run_files(out, suffix)}


def passed_pools(out):
    """The pools whose every run in out exited 0."""
    ok = {}
    for path in run_files(out, "rc"):
        pool = run_parts(path)[0]
        with open(path) as f:
            ok[pool] = ok.get(pool, True) and f.read().strip() == "0"
    return {pool for pool, good in ok.items() if good}


def no_unrefused_drops(top, lists, pools, refused):
    """Refuse, writing nothing, when a pool's list would lose a card no refusal names.

    The dirac gate refuses a landing in which a card leaves dirac/support/evidence without an entry in
    dirac/support/refused. A pool that played clean but did not reach a card already evidenced (the
    card fell under the floor) must fail the round loudly, naming it, rather than commit the drop."""
    drops = {pool: sorted(set(existing_list(top, pool)) - set(lists.get(pool, [])) - refused)
             for pool in pools}
    drops = {pool: cards for pool, cards in drops.items() if cards}
    if drops:
        refuse("; ".join(f"{pool} would drop {', '.join(cards)} with no refusal"
                         for pool, cards in sorted(drops.items())))


def tsv_rows(top, rows):
    """The evidence-games lines with rows rewritten: a named pool's old row replaced in place, a new
    one inserted where the file stays sorted, every other line byte for byte as it was."""
    lines = []
    if os.path.exists(os.path.join(top, EVIDENCE_GAMES)):
        with open(os.path.join(top, EVIDENCE_GAMES)) as f:
            lines = f.read().splitlines()
    written = set()
    out = []
    for line in lines:
        pool = line.split("\t", 1)[0]
        if pool in rows:
            if pool not in written:
                out.append(f"{pool}\t{rows[pool]}")
                written.add(pool)
        else:
            out.append(line)
    for pool in sorted(set(rows) - written):
        i = next((i for i, l in enumerate(out) if l.split("\t", 1)[0] > pool), len(out))
        out.insert(i, f"{pool}\t{rows[pool]}")
    return out


def write_evidence(top, out, pools=None):
    """A list per pool, of the cards it played that meet the floor over the whole corpus's clean games.

    With pools, only those pools' lists and evidence-games rows are written, each list the union of
    the one in the tree and the cards meeting the floor over that pool's own clean games, and every
    expected run of every named pool must have passed first. A card is never removed: only a
    refusal does that. Everything else is left byte for byte as it was.

    Either way the write refuses, changing nothing, when a pool that passed would lose a card no
    refusal names: the dirac gate refuses that landing, so the round must fail here, loudly."""
    refused = refused_names(top)
    lists = collections.defaultdict(list)
    if pools is None:
        for card, (plays, unused, ps) in sorted(merged_coverage(out).items()):
            if plays >= EVIDENCE_MIN_PLAYS and not unused and card not in refused:
                for pool in ps:
                    lists[pool].append(card)
        no_unrefused_drops(top, lists, ran_pools(out) & passed_pools(out), refused)
        for path in glob.glob(os.path.join(top, EVIDENCE_DIR, "*.txt")):
            os.remove(path)
        rows = games_per_run(out)
        lines = [f"{pool}\t{games}" for pool, games in sorted(rows.items())]
    else:
        pools = list(pools)
        require_pool_runs(out, pools)
        games = games_per_run(out)
        for pool in pools:
            if pool not in games:
                refuse(f"{pool} has no verdict")
            gained = [card for card, (plays, unused, _) in sorted(merged_coverage(out, {pool}).items())
                      if plays >= EVIDENCE_MIN_PLAYS and not unused and card not in refused]
            lists[pool] = sorted((set(existing_list(top, pool)) | set(gained)) - refused)
        no_unrefused_drops(top, lists, pools, refused)
        lines = tsv_rows(top, {p: games[p] for p in pools})
    for pool, names in lists.items():
        with open(os.path.join(top, EVIDENCE_DIR, pool + ".txt"), "w") as f:
            f.writelines(n + "\n" for n in names)
    print(f"evidence: {len(set(n for v in lists.values() for n in v))} cards in {len(lists)} lists")
    with open(os.path.join(top, EVIDENCE_GAMES), "w") as f:
        f.writelines(l + "\n" for l in lines)


def games_per_run(out):
    """pool -> the games one of its runs started. Each form of a pool turns away the same pairs."""
    games = {}
    for path in run_files(out, "out"):
        pool = run_parts(path)[0]
        with open(path) as f:
            for line in f:
                v = VERDICT.match(line)
                if v and v.group(1) == pool:
                    games[pool] = int(v.group(2))
                    break
    return games


def report(top, out):
    totals = collections.defaultdict(collections.Counter)
    for path in run_files(out, "out"):
        pool, surface, m = run_parts(path)
        with open(path) as f:
            for line in f:
                v = VERDICT.match(line)
                if v and v.group(1) == pool:
                    keys = ("games", "refused_card", "refused_pair", "error", "clean", "divergent", "unmodelled", "capacity", "hang")
                    for k, x in zip(keys, v.groups()[1:]):
                        totals[f"{surface} m{m}"][k] += int(x)
                    break
    for form, c in sorted(totals.items()):
        print(form, " ".join(f"{k}={v}" for k, v in c.items()))
    classes = collections.Counter()
    for path in glob.glob(os.path.join(top, REFUSED_DIR, "*.json")):
        with open(path) as f:
            classes[json.load(f)["class"]] += 1
    print("refused by class:", sum(classes.values()))
    for k, v in classes.most_common():
        print(f"  {v} {k}")
    games = bad_games(out)
    if games:
        print(f"bad games left: {len(games)}")


def main():
    p = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    p.add_argument("command", choices=["settle", "evidence", "report"])
    p.add_argument("out")
    p.add_argument("--rounds", type=int, default=20)
    p.add_argument("--resume", action="store_true", help="settle: read the runs already in <out>/played-in")
    p.add_argument("--pools", nargs="+", metavar="P", help="evidence: write only these pools' lists and rows")
    a = p.parse_args()
    top, out = root(), os.path.abspath(a.out)
    if a.command == "settle":
        settle(top, out, a.rounds, a.resume)
    elif a.command == "evidence":
        write_evidence(top, out, a.pools)
    else:
        report(top, out)


if __name__ == "__main__":
    main()
