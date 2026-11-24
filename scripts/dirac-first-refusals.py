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
import hashlib
import json
import os
import re
import subprocess
import sys
import tempfile

BASICS = {"Plains", "Island", "Swamp", "Mountain", "Forest", "Wastes"}
EVIDENCE_MIN_PLAYS = 3  # oracle.EvidenceMinPlays
EVIDENCE_SCRIPTED_MIN_GAMES = 3  # oracle.EvidenceScriptedMinGames
SCRIPTED_MIN_SEEDS = 2  # design section 5: over at least two distinct seeds
SCRIPTED_PREFIX = "sc-"  # a scripted run's name: sc-<pool>-<seed>
DECK_POOLS = ("fdn", "blb", "pauper")
DECK_CLEARED_PLAYS = 50
REFUSED_DIR = "dirac/support/refused"
EVIDENCE_DIR = "dirac/support/evidence"
EVIDENCE_GAMES = "dirac/support/evidence-games.tsv"
KEYS_DIR = "dirac/support/evidence-keys"  # one <pool>.tsv sidecar per evidence list, and LEGACY.txt
KEYS_HEADER = "card\tlir16\tplays\tkeys\tunused\tbranches\tmissing_branches"
# A scripted list's sidecar adds its scenario's sha and the organic plays that met
# the plays floor (design section 5). The other columns describe the scripted
# runs' keys; `plays` stays 0, since a scripted game never credits one (D10).
KEYS_SCRIPTED_HEADER = KEYS_HEADER + "\tscenario\torganic_plays"
EVIDENCE_FORMS = ("autopay.m0", "autopay.m1", "full.m0", "full.m1")  # the four runs a pool owes
VERDICT = re.compile(
    r"^(\S+): (\d+) games \((\d+) refused-card, (\d+) refused-pair, (\d+) error\), (\d+) clean, "
    r"(\d+) divergent, (\d+) unmodelled \[.*?\], (\d+) capacity \[.*?\], (\d+) hang"
)


def root():
    return subprocess.check_output(["git", "rev-parse", "--show-toplevel"], text=True).strip()


def path_pool(path):
    return os.path.splitext(os.path.basename(path))[0]


def run_files(out, suffix):
    files = glob.glob(os.path.join(out, f"*.m[01].{suffix}"))
    # A scripted run (kfuzz -scenario) writes one sc-<name>.<kind> file, with no
    # surface or mulligans column in its name: the run is one scenario.
    if suffix in ("coverage", "branches", "evidence", "verdict", "rc", "out"):
        files += glob.glob(os.path.join(out, f"{SCRIPTED_PREFIX}*.{suffix}"))
    return sorted(set(files))


SCRIPTED_FORM = re.compile(r"\.(?:autopay|full)\.m[01]$")
SCRIPTED_RUN = re.compile(r"^(.*?)[.-](\d+)$")


def run_parts(path):
    base = os.path.basename(path)
    stem = base.rsplit(".", 1)[0]
    if stem.startswith(SCRIPTED_PREFIX) and not SCRIPTED_FORM.search(stem):
        return stem, "scripted", 0
    pool, surface, m = base.split(".")[:3]
    return pool, surface, int(m[1:])


def is_scripted(pool):
    return pool.startswith(SCRIPTED_PREFIX)


def scripted_group_seed(run):
    """A scripted run's pool and seed: `sc-<pool>-<NNN>` (also accepts the
    `<card-slug>.<n>` spelling kfuzz derives from a scenario file). A name with
    no trailing number is its own group, at seed 0."""
    m = SCRIPTED_RUN.match(run)
    if not m:
        return run, 0
    return m.group(1), int(m.group(2))


def report_files(out):
    """One report per run: its .out when a pool run wrote one, else its .verdict
    (a scripted run's report, kfuzz -scenario)."""
    paths = {}
    for suffix in ("out", "verdict"):
        for path in run_files(out, suffix):
            paths.setdefault(run_parts(path), path)
    return [paths[k] for k in sorted(paths)]


def bad_games(out):
    """Every bad game of the runs in out, as dicts."""
    games = []
    for path in report_files(out):
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


SYNTH_POOLS = ("std-", "dr-", "tail-", "fx-", "ev-")  # pools dealt by kfuzz -synth, credited from clean games


def run_finished(out, pool, form):
    """For one run of a synthetic pool whose rc is not 0: the cards of its bad games when the run
    finished and failed only on bad games it names (a verdict line, no error skip, no unknown option
    kind, at least one bad game and a bad-game line for every divergent, unmodelled and hung game),
    else None: a run that failed with no bad game to name failed for some other reason."""
    if not pool.startswith(SYNTH_POOLS):
        return None
    path = os.path.join(out, f"{pool}.{form}.out")
    if not os.path.exists(path):
        return None
    v, held, bad = None, set(), 0
    with open(path) as f:
        for line in f:
            m = VERDICT.match(line)
            if m and m.group(1) == pool and v is None:
                v = m
            elif line.startswith("bad-game\t"):
                bad += 1
                held |= set(names_of(line.rstrip("\n").split("\t")[7]))
            elif line.startswith("unknown-game\t"):
                return None
    if v is None or int(v.group(5)) or not bad or bad != int(v.group(7)) + int(v.group(8)) + int(v.group(10)):
        return None
    return held


def run_rcs(out):
    """pool -> form -> the run's rc, as a string. A scripted run's verdict is its
    report, not a pool run's rc: it is judged by the scripted rule instead."""
    rcs = collections.defaultdict(dict)
    for path in run_files(out, "rc"):
        pool, surface, m = run_parts(path)
        if is_scripted(pool):
            continue
        with open(path) as f:
            rcs[pool][f"{surface}.m{m}"] = f.read().strip()
    return rcs


def pool_credit(out):
    """(pools that count for nothing, cards a counted pool's bad games hold). A run that passed counts;
    so does a synthetic pool's run that finished with bad games only (run_finished): its coverage holds
    clean games alone (oracle.Coverage.MergeClean), and every card of its bad games is held back, in
    every pool, so a card is credited only when no bad game holds it. Any other run that did not pass
    makes its pool count for nothing, as an unfinished corpus leaves cards unverified."""
    dirty, held = set(), set()
    for pool, forms in run_rcs(out).items():
        for form, rc in forms.items():
            if rc == "0":
                continue
            h = run_finished(out, pool, form)
            if h is None:
                dirty.add(pool)
                break
            held |= h
    return dirty, held


def merged_coverage(out, only=None):
    """card -> [plays, unused keys in every run, pools it was played in, keys_total, branch uses, branch
    owed], over clean games only. keys_total is the largest the runs report; uses and owed map a branch
    key to its sum over the same runs (the run's .branches file, when it has one). A pool with a run that
    did not pass, other than a synthetic one that finished with bad games only, counts for nothing, and
    a card any counted pool's bad game holds is left out (pool_credit). With only, count the named
    pools' own runs alone."""
    dirty, held = pool_credit(out)
    cards = {}
    for path in run_files(out, "coverage"):
        pool = run_parts(path)[0]
        if is_scripted(pool) or pool in dirty or (only is not None and pool not in only):
            continue
        with open(path) as f:
            next(f)
            for line in f:
                card, cast, land, _, total, unused = line.rstrip("\n").split("\t")
                if card in held:
                    continue
                plays = int(cast) + int(land)
                unused = set(filter(None, unused.split(",")))
                c = cards.setdefault(card, [0, unused, set(), 0, collections.Counter(), collections.Counter()])
                c[0] += plays
                c[1] &= unused
                c[3] = max(c[3], int(total))
                if plays:
                    c[2].add(pool)
        branches = path[:-len("coverage")] + "branches"
        if not os.path.exists(branches):
            continue
        with open(branches) as f:
            next(f)
            for line in f:
                card, key, uses, owed = line.rstrip("\n").split("\t")
                if card in cards and card not in held:
                    cards[card][4][key] += int(uses)
                    cards[card][5][key] += int(owed)
    return cards


def refused_names(top):
    names = set()
    for path in glob.glob(os.path.join(top, REFUSED_DIR, "*.json")):
        with open(path) as f:
            names.add(json.load(f)["card"])
    return names


def require_pool_runs(out, pools):
    """Refuse, writing nothing, unless every named pool has the four expected runs (autopay and
    full, mulligans 0 and 1), each passed, or, for a synthetic pool, finished with bad games only."""
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
                if f.read().strip() != "0" and run_finished(out, pool, form) is None:
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
    card fell under the floor) must fail the round loudly, naming it, rather than commit the drop.
    An evidence row carries its card name before the first tab (a pod row's seats column), so the
    comparison is by name: a refused pod row leaving is not an unrefused drop."""
    drops = {pool: sorted({row_name(r) for r in existing_list(top, pool)} -
                          {row_name(r) for r in lists.get(pool, [])} - refused)
             for pool in pools}
    drops = {pool: cards for pool, cards in drops.items() if cards}
    if drops:
        refuse("; ".join(f"{pool} would drop {', '.join(cards)} with no refusal"
                         for pool, cards in sorted(drops.items())))


def game_row(pool, games, pods):
    """One evidence-games row: a pod run's carries its clean pod games as a third column."""
    if pods:
        return f"{pool}\t{games}\t{pods}"
    return f"{pool}\t{games}"


def tsv_rows(top, rows):
    """The evidence-games lines with rows rewritten: a named pool's old row replaced in place, a new
    one inserted where the file stays sorted, every other line byte for byte as it was. A value is
    the run's (games, pods)."""
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
                out.append(game_row(pool, *rows[pool]))
                written.add(pool)
        else:
            out.append(line)
    for pool in sorted(set(rows) - written):
        i = next((i for i, l in enumerate(out) if l.split("\t", 1)[0] > pool), len(out))
        out.insert(i, game_row(pool, *rows[pool]))
    return out


def lir_keys(top, cards):
    """card -> the 16-hex evidence key of its -lir CARD block, from one `kgen -lir-keys` call over cards.
    DFR_LIR_KEYS names a card<TAB>key file that stands in for the call, for tests."""
    cards = sorted(set(cards))
    path = os.environ.get("DFR_LIR_KEYS")
    if path:
        with open(path) as f:
            text = f.read()
    elif not cards:
        return {}
    elif not os.access(os.path.join(top, "bin/kgen"), os.X_OK):
        print("evidence: bin/kgen is not built; the sidecar's lir16 column stays empty", file=sys.stderr)
        return {}
    else:
        with tempfile.TemporaryDirectory() as tmp:
            lst = os.path.join(tmp, "cards.txt")
            with open(lst, "w") as f:
                f.writelines(c + "\n" for c in cards)
            r = subprocess.run([os.path.join(top, "bin/kgen"), "-lir-keys", "-card-list", lst, "-cards", ".cards"],
                               cwd=top, capture_output=True, text=True)
        if r.returncode:
            refuse(f"kgen -lir-keys failed: {r.stderr.strip()}")
        text = r.stdout
    keys = {}
    for line in text.splitlines():
        card, _, key = line.partition("\t")
        if key:
            keys[card] = key
    return keys


def sidecar_rows(top, pool):
    """card -> the sidecar's line for it, as the file has it now."""
    path = os.path.join(top, KEYS_DIR, pool + ".tsv")
    rows = {}
    if os.path.exists(path):
        with open(path) as f:
            for line in f.read().splitlines()[1:]:
                if line:
                    rows[line.split("\t", 1)[0]] = line
    return rows


def sidecar_row(card, lir, cov):
    plays, unused, _, total, uses, owed = cov
    keys = f"{total - len(unused)}/{total}"
    wanted = sorted(k for k in owed if owed[k] > 0)
    missing = [k for k in wanted if not uses[k]]
    return "\t".join([card, lir, str(plays), keys, ",".join(sorted(unused)),
                      f"{len(wanted) - len(missing)}/{len(wanted)}", ",".join(missing)])


def sidecars(top, lists, before, covs):
    """pool -> the sidecar's lines for each written list: a card that was already on its list before
    this write keeps the row it has (a card with no row stays legacy), and a card the list gains, or
    regains after its list was removed, gets a current row."""
    lir = lir_keys(top, {c for pool, names in lists.items() for c in names
                         if c not in before.get(pool, ()) and c in covs[pool]})
    files = {}
    for pool, names in lists.items():
        old = sidecar_rows(top, pool)
        kept = before.get(pool, ())
        rows = {c: old[c] for c in names if c in old and c in kept}
        for c in names:
            if c not in rows and c not in before.get(pool, ()) and c in covs[pool]:
                rows[c] = sidecar_row(c, lir.get(c, ""), covs[pool][c])
        files[pool] = [KEYS_HEADER] + [rows[c] for c in sorted(rows)]
    return files


def row_name(row):
    """An evidence row's card name: its part before the first tab (a pod row's seats column)."""
    return row.split("\t", 1)[0]


def merge_rows(existing, run):
    """One row per card name, with a pod row (one carrying trailing columns) winning over a plain
    one. Unrefusing a card restores it plain; without this its plain row would shadow the pod run
    that re-evidences it (readSupport takes the first row of a name, and sorted order puts the plain
    row first), so the card would never read as pod-evidenced again."""
    rows = {}
    for row in sorted(existing):
        name = row_name(row)
        if name not in rows or "\t" in row:
            rows[name] = row
    for row in sorted(run):
        name = row_name(row)
        if name not in rows or "\t" in row:
            rows[name] = row
    return list(rows.values())


def write_legacy(top):
    """LEGACY.txt: the evidenced names no sidecar has a row for, sorted. An evidence row may carry
    trailing columns (a pod run's seats field); the name is the part before the first tab."""
    names, keyed = set(), set()
    for path in glob.glob(os.path.join(top, EVIDENCE_DIR, "*.txt")):
        with open(path) as f:
            names |= {row_name(n) for n in f.read().splitlines() if n}
    for path in glob.glob(os.path.join(top, KEYS_DIR, "*.tsv")):
        keyed |= set(sidecar_rows(top, os.path.basename(path)[:-len(".tsv")]))
    os.makedirs(os.path.join(top, KEYS_DIR), exist_ok=True)
    with open(os.path.join(top, KEYS_DIR, "LEGACY.txt"), "w") as f:
        f.writelines(n + "\n" for n in sorted(names - keyed))
    print(f"evidence: {len(names - keyed)} legacy names")


def evidence_rows(out, pool):
    """The rows a pod run wrote for pool (Name, or Name<TAB><seats>), unioned over its forms: a pod
    run writes no coverage, so these .evidence rows are the card list it lands."""
    rows = set()
    for path in run_files(out, "evidence"):
        if run_parts(path)[0] != pool:
            continue
        with open(path) as f:
            rows |= {line.rstrip("\n") for line in f if line.strip()}
    return rows


def without_refused(rows, refused):
    """The rows whose name no refusal names; an evidence row's trailing columns stay with it."""
    return [r for r in rows if row_name(r) not in refused]


def scripted_groups(out):
    """The scripted pools out has a coverage file of, in name order."""
    return sorted({scripted_group_seed(run_parts(p)[0])[0]
                   for p in run_files(out, "coverage") if is_scripted(run_parts(p)[0])})


def scripted_coverage(out):
    """group -> card -> the group's scripted runs, merged for the evidence rule:
    the card's keys_total, the keys no run used (intersection), per run its unused
    keys and key total, its seed, and the branches the runs took.

    A scripted coverage file holds clean games only (`Coverage.MergeClean`), and
    its `plays` are 0 by construction (D10): this rule reads keys, never plays."""
    groups = {}
    for path in run_files(out, "coverage"):
        run = run_parts(path)[0]
        if not is_scripted(run):
            continue
        group, seed = scripted_group_seed(run)
        with open(path) as f:
            next(f)
            for line in f:
                card, _, _, _, total, unused = line.rstrip("\n").split("\t")
                c = groups.setdefault(group, {}).setdefault(card, {
                    "total": 0, "unused": None, "runs": {}, "run_unused": {}, "run_total": {},
                    "uses": collections.Counter(), "owed": collections.Counter()})
                c["total"] = max(c["total"], int(total))
                u = set(filter(None, unused.split(",")))
                c["runs"][run] = seed
                c["run_unused"][run] = u
                c["run_total"][run] = int(total)
                c["unused"] = u if c["unused"] is None else c["unused"] & u
        branches = path[:-len("coverage")] + "branches"
        if not os.path.exists(branches):
            continue
        with open(branches) as f:
            next(f)
            for line in f:
                card, key, uses, owed = line.rstrip("\n").split("\t")
                c = groups.get(group, {}).get(card)
                if c:
                    c["uses"][key] += int(uses)
                    c["owed"][key] += int(owed)
    return groups


def scripted_held(out):
    """The cards a scripted run's bad game implicates (design section 5(c)).

    A bad-game line's last field is the game's whole deck table, which the
    organic hold rule uses to answer a failed run; here the column is the
    divergence's implicated cards, or, when it names none (an unmodelled
    stall), the sources of its latest actions — the same fallback blame uses."""
    held = set()
    for g in bad_games(out):
        if is_scripted(g["pool"]):
            held |= set(g["impl"] or g["recent"])
    return held


def frac_total(s):
    _, _, d = s.partition("/")
    try:
        return int(d)
    except ValueError:
        return 0


def organic_sidecars(top):
    """card -> (plays, unused keys, keys_total) from its existing evidence-keys row.

    The design's rule (a) reads an existing row's organic plays; (b) reads the
    keys the row records as never used. There is one row per card (the first
    sidecar in name order wins, as dirac/kgen/coverage_ratchet.go reads them)."""
    plays, unused, total = {}, {}, {}
    for path in sorted(glob.glob(os.path.join(top, KEYS_DIR, "*.tsv"))):
        pool = os.path.basename(path)[:-len(".tsv")]
        if is_scripted(pool):
            continue
        for card, line in sidecar_rows(top, pool).items():
            if card in plays:
                continue
            col = line.split("\t")
            if len(col) < 6:
                continue
            try:
                plays[card] = int(col[2])
            except ValueError:
                plays[card] = 0
            unused[card] = {k for k in col[4].split(",") if k}
            total[card] = frac_total(col[3])
    return plays, unused, total


def scripted_key_enough(c, key, total):
    """The key was used in EVIDENCE_SCRIPTED_MIN_GAMES clean scripted games over
    at least SCRIPTED_MIN_SEEDS distinct seeds. A run is one game: the design
    names them sc-<pool>-<NNN>."""
    games, seeds = 0, set()
    for run, seed in c["runs"].items():
        if key not in c["run_unused"][run] and c["run_total"][run] >= total:
            games += 1
            seeds.add(seed)
    return games >= EVIDENCE_SCRIPTED_MIN_GAMES and len(seeds) >= SCRIPTED_MIN_SEEDS


def scripted_scenario_sha(out, group, cards):
    """The scenario file's sha1, when it sits beside the runs (or in scenarios/)."""
    runs = sorted({run for c in cards.values() for run in c["runs"]})
    stems = []
    for run in runs:
        stems.append(run)
        if run.startswith(SCRIPTED_PREFIX):
            stems.append(run[len(SCRIPTED_PREFIX):])
    stems.append(group)
    if group.startswith(SCRIPTED_PREFIX):
        stems.append(group[len(SCRIPTED_PREFIX):])
    for stem in stems:
        for path in (os.path.join(out, stem + ".json"), os.path.join(out, "scenarios", stem + ".json")):
            if os.path.exists(path):
                with open(path, "rb") as f:
                    return hashlib.sha1(f.read()).hexdigest()
    return ""


def scripted_evidence(top, out, groups, refused):
    """The scripted lists and sidecar data for groups: lists, covs, meta.

    A card is credited when (a) its organic plays (an existing evidence-keys row
    or this cycle's organic runs) reach EVIDENCE_MIN_PLAYS; (b) every key those
    organic games never used was used in EVIDENCE_SCRIPTED_MIN_GAMES clean
    scripted games over at least two seeds; and (c) no bad game in any of its
    scenarios implicates it. A card is never removed: only a refusal removes it."""
    organic_plays, organic_unused, organic_total = organic_sidecars(top)
    cycle = merged_coverage(out)  # organic only: merged_coverage skips sc- runs
    coverage = scripted_coverage(out)
    held = scripted_held(out)
    reported = {run_parts(p)[0] for p in report_files(out)}
    lists, covs, meta = {}, {}, {}
    for group in sorted(groups):
        cards = coverage.get(group)
        if not cards:
            refuse(f"{group} has no scripted coverage")
        if not any(run in reported for c in cards.values() for run in c["runs"]):
            refuse(f"{group} has no verdict")
        credited = []
        plays_of = {}
        for card, c in sorted(cards.items()):
            if card in refused or card in held:
                continue
            plays = organic_plays.get(card, 0)
            unused = set(organic_unused.get(card, ()))
            total = max(c["total"], organic_total.get(card, 0))
            if card in cycle:
                cyc_plays, cyc_unused, _, cyc_total, *_ = cycle[card]
                plays = max(plays, cyc_plays)
                total = max(total, cyc_total)
                # A key this cycle's organic games used is no longer missing. With
                # no sidecar row, the cycle's own unused set is the missing set.
                unused = unused & cyc_unused if card in organic_unused else set(cyc_unused)
            plays_of[card] = plays
            if plays < EVIDENCE_MIN_PLAYS:
                continue
            if any(not scripted_key_enough(c, key, total) for key in unused):
                continue
            credited.append(card)
        names = sorted((set(existing_list(top, group)) | set(credited)) - refused)
        lists[group] = names
        covs[group] = {card: cards[card] for card in names if card in cards}
        meta[group] = (scripted_scenario_sha(out, group, cards),
                       {card: plays_of.get(card, organic_plays.get(card, 0)) for card in names})
    return lists, covs, meta


def scripted_sidecars(top, lists, before, covs, meta):
    """The sidecar files of the scripted lists: the seven organic columns over the
    scripted runs' keys (plays 0, D10), then the scenario sha and organic plays."""
    files = {}
    for pool, names in lists.items():
        if not is_scripted(pool):
            continue
        sha, plays = meta.get(pool, ("", {}))
        have = covs.get(pool, {})
        lir = lir_keys(top, {c for c in names if c in have})
        old = sidecar_rows(top, pool)
        rows = {}
        for card in names:
            c = have.get(card)
            if card in old and card in before.get(pool, ()):
                row = old[card]
                if len(row.split("\t")) == 7:
                    row = "\t".join([row, sha, str(plays.get(card, 0))])
                rows[card] = row
                continue
            if c is None:
                cov = (0, set(), None, 0, collections.Counter(), collections.Counter())
            else:
                cov = (0, c["unused"] or set(), None, c["total"], c["uses"], c["owed"])
            rows[card] = "\t".join([sidecar_row(card, lir.get(card, ""), cov), sha, str(plays.get(card, 0))])
        files[pool] = [KEYS_SCRIPTED_HEADER] + [rows[c] for c in sorted(rows)]
    return files


def write_evidence(top, out, pools=None):
    """A list per pool, of the cards it played that meet the floor over the whole corpus's clean games.

    With pools, only those pools' lists and evidence-games rows are written, each list the union of
    the one in the tree and the cards meeting the floor over that pool's own clean games, and every
    expected run of every named pool must have passed first; a synthetic pool's run may instead have
    finished with bad games only, and then no card those games hold is credited. A pod pool (name
    pod-*) writes no coverage files: its run's .evidence rows are the list's source, seats column and
    all. A scripted pool (name sc-*, one per scenario) credits key uses only, under the section-5
    rule: its list and sidecar are written from scripted_evidence, which keeps a card the tree
    already lists and takes one out only when a refusal names it. A card is never removed: only a
    refusal does that. Everything else is left byte for byte as it was.

    Either way the write refuses, changing nothing, when a pool that passed would lose a card no
    refusal names: the dirac gate refuses that landing, so the round must fail here, loudly."""
    refused = refused_names(top)
    lists = collections.defaultdict(list)
    covs = {}
    meta = {}
    remove = []
    if pools is None:
        cov = merged_coverage(out)
        before = {path_pool(p): set(existing_list(top, path_pool(p)))
                  for p in glob.glob(os.path.join(top, EVIDENCE_DIR, "*.txt"))}
        for card, (plays, unused, ps, *_) in sorted(cov.items()):
            if plays >= EVIDENCE_MIN_PLAYS and not unused and card not in refused:
                for pool in ps:
                    lists[pool].append(card)
        covs = {pool: cov for pool in lists}
        passed = ran_pools(out) & passed_pools(out)
        # A pod run has no coverage, so the coverage loop above never sees its pool: land it from
        # its .evidence rows when its runs passed, and leave a landed pod list alone when they are
        # not in this out dir (the pod writer owns it; settle must not delete it).
        for pool in sorted(passed):
            if pool.startswith("pod-"):
                rows = merge_rows(existing_list(top, pool), evidence_rows(out, pool))
                lists[pool] = sorted(without_refused(rows, refused))
                covs[pool] = {}
        # A scripted run writes no evidence rows (D10), so its list comes from the key rule. A
        # settle out dir has no sc- files: leave the landed scripted lists alone then.
        sc_groups = scripted_groups(out)
        sc_lists, sc_covs, sc_meta = scripted_evidence(top, out, sc_groups, refused)
        lists.update(sc_lists)
        covs.update(sc_covs)
        meta.update(sc_meta)
        no_unrefused_drops(top, lists, passed | set(sc_groups), refused)
        remove = [path for path in glob.glob(os.path.join(top, EVIDENCE_DIR, "*.txt"))
                  if not path_pool(path).startswith("pod-") and not is_scripted(path_pool(path))]
        rows, pods = games_per_run(out)
        # Rewrite the runs' own rows in place and keep the rest: a settle out dir
        # holds no scripted (or pod) runs, and their rows are not its to drop.
        lines = tsv_rows(top, {pool: (games, pods.get(pool, 0)) for pool, games in rows.items()})
    else:
        pools = list(pools)
        organic = [pool for pool in pools if not is_scripted(pool)]
        scripted = [pool for pool in pools if is_scripted(pool)]
        before = {pool: set(existing_list(top, pool)) for pool in pools}
        require_pool_runs(out, organic)
        games, pods = games_per_run(out)
        for pool in organic:
            if pool not in games:
                refuse(f"{pool} has no verdict")
            covs[pool] = merged_coverage(out, {pool})
            gained = [card for card, (plays, unused, *_) in sorted(covs[pool].items())
                      if plays >= EVIDENCE_MIN_PLAYS and not unused and card not in refused]
            rows = set(existing_list(top, pool)) | set(gained)
            if pool.startswith("pod-"):
                rows = merge_rows(rows, evidence_rows(out, pool))
                if not rows:
                    refuse(f"{pool} has no evidence rows")
            lists[pool] = sorted(without_refused(rows, refused))
        sc_lists, sc_covs, sc_meta = scripted_evidence(top, out, scripted, refused)
        lists.update(sc_lists)
        covs.update(sc_covs)
        meta.update(sc_meta)
        no_unrefused_drops(top, lists, pools, refused)
        lines = tsv_rows(top, {p: (games.get(p, 0), pods.get(p, 0)) for p in pools})
    side = sidecars(top, {p: v for p, v in lists.items() if not is_scripted(p)},
                    before, {p: v for p, v in covs.items() if not is_scripted(p)})
    side.update(scripted_sidecars(top, lists, before, covs, meta))
    os.makedirs(os.path.join(top, KEYS_DIR), exist_ok=True)
    for path in remove:
        os.remove(path)
    for pool, names in lists.items():
        with open(os.path.join(top, EVIDENCE_DIR, pool + ".txt"), "w") as f:
            f.writelines(n + "\n" for n in names)
        if pool.startswith("pod-"):
            # A pod-evidenced card is supported already, so a two-seat list gave it its sidecar;
            # a pod list needs none of its own.
            continue
        with open(os.path.join(top, KEYS_DIR, pool + ".tsv"), "w") as f:
            f.writelines(l + "\n" for l in side[pool])
    print(f"evidence: {len(set(n for v in lists.values() for n in v))} cards in {len(lists)} lists")
    with open(os.path.join(top, EVIDENCE_GAMES), "w") as f:
        f.writelines(l + "\n" for l in lines)
    # A card this write gave a sidecar row is no longer legacy: rewrite LEGACY.txt with the rows (11-18: two
    # evidence-loop PRs, #1450 and #1556, keyed hundreds of LEGACY.txt cards and left them listed, and
    # TestEvidenceCoverageRatchet failed "stale legacy entry" in their merge-queue batches).
    prune_legacy(top, {c for rows in side.values() for c in (r.split("\t", 1)[0] for r in rows[1:])})


def prune_legacy(top, keyed):
    """Drop from LEGACY.txt the names that now have a sidecar row; never add one (a card with no row stays as listed)."""
    path = os.path.join(top, KEYS_DIR, "LEGACY.txt")
    if not os.path.exists(path):
        return
    with open(path) as f:
        names = [n for n in f.read().splitlines() if n]
    kept = [n for n in names if n not in keyed]
    if len(kept) != len(names):
        with open(path, "w") as f:
            f.writelines(n + "\n" for n in kept)
        print(f"evidence: {len(names) - len(kept)} names left LEGACY.txt")


def run_kernel(pool):
    """The name a run's verdict line carries. A pod run is dealt as -kernel <pool without pod->
    (scripts/dirac-evidence.sh's pod case), so its out file names the set (cmd5), not the run
    (pod-cmd5); every other run names its own pool."""
    return pool[len("pod-"):] if pool.startswith("pod-") else pool


def games_per_run(out):
    """pool -> (games, pods): the games one of its runs started, and, when the run is a pod
    run (its name begins pod-, which scripts/dirac-evidence.sh's pod case deals), the clean
    pod games among them: every clean game of a pod run is one. Each form of a pool turns
    away the same pairs. A scripted run is one scenario at one seed (sc-<pool>-<NNN>): its
    games add up under that pool. A run file contributes only its own verdict: the file's
    stem names the pool, and the verdict inside it must name that pool's kernel."""
    games, pods = {}, {}
    for path in report_files(out):
        run = run_parts(path)[0]
        kernel = run_kernel(run)
        verdict = None
        with open(path) as f:
            for line in f:
                v = VERDICT.match(line)
                if v and v.group(1) == kernel:
                    verdict = v
                    break
        if verdict is None:
            continue
        if is_scripted(run):
            pool, _ = scripted_group_seed(run)
            games[pool] = games.get(pool, 0) + int(verdict.group(2))
            continue
        games[run] = int(verdict.group(2))
        if run.startswith("pod-"):
            pods[run] = int(verdict.group(6))
    return games, pods


def report(top, out):
    totals = collections.defaultdict(collections.Counter)
    for path in report_files(out):
        pool, surface, m = run_parts(path)
        kernel = run_kernel(pool)
        if is_scripted(pool):
            pool, _ = scripted_group_seed(pool)
            surface, m = "scripted", 0
        with open(path) as f:
            for line in f:
                v = VERDICT.match(line)
                if v and v.group(1) == kernel:
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
    p.add_argument("command", choices=["settle", "evidence", "report", "legacy-keys"])
    p.add_argument("out", nargs="?", default=".", help="the run directory (not read by legacy-keys)")
    p.add_argument("--rounds", type=int, default=20)
    p.add_argument("--resume", action="store_true", help="settle: read the runs already in <out>/played-in")
    p.add_argument("--pools", nargs="+", metavar="P", help="evidence: write only these pools' lists and rows")
    a = p.parse_args()
    top, out = root(), os.path.abspath(a.out)
    if a.command == "settle":
        settle(top, out, a.rounds, a.resume)
    elif a.command == "evidence":
        write_evidence(top, out, a.pools)
    elif a.command == "legacy-keys":
        write_legacy(top)
    else:
        report(top, out)


if __name__ == "__main__":
    main()
