#!/usr/bin/env python3
"""Credit dirac evidence from xproof's lockstep rows. Stdlib only.

    scripts/dirac-xproof-evidence.py <pulled-root-or-cycle-dir>... [--pool P] [--seats N] [--dry-run]

xproof plays each scripted start (one card, one key) in gorge and, since `xengine play
-start -dirac`, in dirac beside it. A cycle's out/<cycle>/ holds

    dirac-lockstep.tsv   batch id answer pattern xmage_class forge_class field
                         one row per start item; answer D=G (dirac reached what gorge did),
                         D!=G (it differs) or D- (no answer: dirac could not play the game)
    keyproof-rows.tsv    one row per key; state agreed with refs F (Forge), X (XMage) or FX,
                         and a witness naming the item ids that proved it (x:<id>,f:<id>)

A card is credited when a lockstep row answers D=G and its id is a witness of a key that is
`agreed` with Forge (refs holds F). Forge outranks XMage: a key only XMage agreed (refs X) does
not credit. A row that diverged or has no answer, or whose id no keyproof row names, never
credits, and a card any ingested row shows diverging is not credited at all (and is reported;
nothing already evidenced is removed). The card's name comes from the item id's slug
(xs:<set>:<card slug>:<key>:s<seed>), matched against the names of dirac's pools.

A credited card gets the row `Name<TAB>seats<TAB>xproof` in --pool's list (default pod-xproof),
which dirac/kgen's readSupport reads: an xproof row evidences from two seats. --seats is the
seat count of the games the lockstep played (dirac plays two). A card the lists already
evidence is left alone; a credited card with no evidence-keys sidecar row joins LEGACY.txt, as
every evidenced card must. Re-importing a cycle changes nothing; rows are written sorted.
"""

import argparse
import collections
import glob
import importlib.util
import os
import re
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
SOURCE = "xproof"
DEFAULT_POOL = "pod-xproof"
LOCKSTEP = "dirac-lockstep.tsv"
KEYPROOF = "keyproof-rows.tsv"

# xengine's dirac answers (oracle/xengine/dside), and the words a brief may use for them.
AGREE = ("D=G", "agree")
DIVERGE = ("D!=G", "diverge")

# The reasons an item or a card is skipped, in the order a card reports the first of them.
REASONS = ("diverge", "refused", "no-keyproof", "forge-not-agreed", "xmage-only", "unnamed-id",
           "unknown-name", "ambiguous-name", "refused-by-name", "already-evidenced", "already-listed")


def load_refusals_module():
    spec = importlib.util.spec_from_file_location("dirac_first_refusals", os.path.join(HERE, "dirac-first-refusals.py"))
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


def card_slug(name):
    """A card name as xproof's item ids spell it (xproof.card_slug): "Katara, Water Tribe's Hope" -> katara-water-tribe-s-hope."""
    return re.sub(r"[^a-z0-9]+", "-", (name or "").lower()).strip("-")


def id_slug(item_id):
    """The card slug of a start item id xs:<set>:<slug>:<key>:s<seed>, or None for any other id."""
    p = item_id.split(":")
    return p[2] if len(p) >= 5 and p[0] == "xs" and p[2] else None


def refs_of(refs):
    """The letters of a keyproof refs cell: FX, F, X, with a third opinion's +P dropped."""
    return set(refs.replace("+P", "")) - {"-"}


def judge(answer, kp):
    """(True, "") when a lockstep row of this answer and the keyproof row its id joins to (None for
    none) credit the card, else (False, reason). The one place the crediting rule lives."""
    if answer in DIVERGE:
        return False, "diverge"
    if answer not in AGREE:
        return False, "refused"
    if kp is None:
        return False, "no-keyproof"
    if kp.get("state") != "agreed":
        return False, "forge-not-agreed"
    refs = refs_of(kp.get("refs", "-"))
    if "F" in refs:
        return True, ""
    return False, "xmage-only" if "X" in refs else "forge-not-agreed"


def read_tsv(path):
    """The rows of a tsv as header-keyed dicts; a missing file has none."""
    try:
        with open(path) as f:
            lines = f.read().splitlines()
    except OSError:
        return []
    if not lines:
        return []
    cols = lines[0].split("\t")
    return [dict(zip(cols, ln.split("\t"))) for ln in lines[1:] if ln.strip()]


def find_cycles(paths):
    """Every directory under (or being) a path that holds a dirac-lockstep.tsv, sorted."""
    found = set()
    for path in paths:
        path = os.path.abspath(path)
        if os.path.isfile(os.path.join(path, LOCKSTEP)):
            found.add(path)
            continue
        for d, dirs, files in os.walk(path):
            if LOCKSTEP in files:
                found.add(d)
                dirs[:] = []  # a cycle's subdirs are not cycles
    return sorted(found)


def witness_index(rows):
    """item id -> the keyproof row whose witness names it (x:<id> or f:<id>)."""
    idx = {}
    for r in rows:
        for tok in r.get("witness", "-").split(","):
            m = re.match(r"^[xf]:(.+)$", tok.strip())
            if m:
                idx.setdefault(m.group(1), r)
    return idx


def first_face(name):
    return name.split(" // ", 1)[0] if " // " in name else ""


def name_index(top, extra):
    """slug -> the card names it may be: every name dirac's pools and evidence lists hold, and those
    in extra files (one per line)."""
    names = set()
    files = glob.glob(os.path.join(top, "dirac/cardset/pools/*/cards.txt"))
    files += glob.glob(os.path.join(top, "dirac/support/evidence/*.txt")) + list(extra)
    for path in files:
        with open(path) as f:
            for ln in f.read().splitlines():
                n = ln.split("\t", 1)[0].strip()
                if n and not n.startswith("#"):
                    names.add(n)
    idx = collections.defaultdict(set)
    for n in names:
        idx[card_slug(n)].add(n)
        if first_face(n):
            idx[card_slug(first_face(n))].add(n)
    return idx


def evidenced_names(mod, top):
    """The names the evidence lists evidence, by the reader's rule (kgen readSupport): a plain row, a
    row of four or more seats, or an xproof row of two or more. A split card counts under its first face too."""
    out = set()
    for path in glob.glob(os.path.join(top, mod.EVIDENCE_DIR, "*.txt")):
        with open(path) as f:
            rows = [r for r in f.read().splitlines() if r.strip()]
        for r in rows:
            name, _, col = r.partition("\t")
            seats, _, source = col.partition("\t")
            try:
                n = int(seats.strip()) if col else None
            except ValueError:
                continue
            if n is None or n >= 4 or (n >= 2 and source.strip() == SOURCE):
                out.add(name)
                if first_face(name):
                    out.add(first_face(name))
    return out


def ingest(cycles):
    """(per, tally): per[slug] = the items read for a card slug, each (cycle, id, credit, reason);
    tally = rows by answer and item skip reasons, over every cycle."""
    per = collections.defaultdict(list)
    tally = {"cycles": 0, "foreign": 0, "answers": collections.Counter(), "items": collections.Counter()}
    for cyc in cycles:
        rows = read_tsv(os.path.join(cyc, LOCKSTEP))
        tally["cycles"] += 1
        if rows and not ("answer" in rows[0] and "id" in rows[0]):
            tally["foreign"] += 1  # an xpa-style log (outcome column): not xproof's start items
            continue
        idx = witness_index(read_tsv(os.path.join(cyc, KEYPROOF)))
        for r in rows:
            answer = r.get("answer", "-")
            tally["answers"][answer] += 1
            ok, why = judge(answer, idx.get(r.get("id", "")))
            slug = id_slug(r.get("id", ""))
            tally["items"]["credit" if ok else why] += 1
            per[slug].append((os.path.basename(cyc), r.get("id", ""), ok, why))
    return per, tally


def decide(per, names, listed, evidenced, refused):
    """(credited, skipped): credited = names to write; skipped = name-or-slug -> reason, for every card
    slug the rows named. A card any row shows diverging is never credited."""
    credited, skipped = [], {}
    for slug in sorted(per, key=lambda s: s or ""):
        items = per[slug]
        label = slug or "(unnamed ids)"
        whys = [w for _, _, ok, w in items if not ok]
        if slug is None:
            skipped[label] = "unnamed-id"
            continue
        cands = sorted(names.get(slug, ()))
        if "diverge" in whys:
            skipped[cands[0] if len(cands) == 1 else label] = "diverge"
            continue
        if not any(ok for _, _, ok, _ in items):
            first = min(whys, key=REASONS.index)
            skipped[cands[0] if len(cands) == 1 else label] = first
            continue
        if not cands:
            skipped[label] = "unknown-name"
        elif len(cands) > 1:
            skipped[label] = "ambiguous-name"
        elif cands[0] in refused or first_face(cands[0]) in refused:
            skipped[cands[0]] = "refused-by-name"
        elif cands[0] in listed:
            skipped[cands[0]] = "already-listed"
        elif cands[0] in evidenced:
            skipped[cands[0]] = "already-evidenced"
        else:
            credited.append(cands[0])
    return credited, skipped


def add_legacy(mod, top, names):
    """Add to LEGACY.txt (kept sorted) the names no sidecar has a row for, as legacyKeyIfNoRow does."""
    keyed = set()
    for path in glob.glob(os.path.join(top, mod.KEYS_DIR, "*.tsv")):
        keyed |= set(mod.sidecar_rows(top, os.path.basename(path)[:-len(".tsv")]))
    path = os.path.join(top, mod.KEYS_DIR, "LEGACY.txt")
    have = []
    if os.path.exists(path):
        with open(path) as f:
            have = [n for n in f.read().splitlines() if n]
    want = sorted(set(have) | {n for n in names if n not in keyed})
    if want != have:
        os.makedirs(os.path.dirname(path), exist_ok=True)
        with open(path, "w") as f:
            f.writelines(n + "\n" for n in want)
    return len(want) - len(have)


def run(paths, top, pool=DEFAULT_POOL, seats=2, extra_names=(), dry=False, out=sys.stdout):
    """Import the cycles under paths into top's pool list; returns the credited names."""
    mod = load_refusals_module()
    cycles = find_cycles(paths)
    per, tally = ingest(cycles)
    existing = mod.existing_list(top, pool)
    listed = {mod.row_name(r) for r in existing}
    evidenced = evidenced_names(mod, top)
    credited, skipped = decide(per, name_index(top, extra_names), listed, evidenced, mod.refused_names(top))
    rows = mod.merge_rows(existing, [f"{n}\t{seats}\t{SOURCE}" for n in credited])
    rows = sorted(mod.without_refused(rows, mod.refused_names(top)))
    changed = rows != existing and bool(credited)
    legacy = 0
    if changed and not dry:
        os.makedirs(os.path.join(top, mod.EVIDENCE_DIR), exist_ok=True)
        with open(os.path.join(top, mod.EVIDENCE_DIR, pool + ".txt"), "w") as f:
            f.writelines(r + "\n" for r in rows)
        legacy = add_legacy(mod, top, credited)
    by_reason = collections.Counter(skipped.values())
    print(f"xproof evidence: {tally['cycles']} cycles read ({tally['foreign']} in a foreign format, skipped)", file=out)
    print("rows by answer: " + (", ".join(f"{a}={n}" for a, n in sorted(tally["answers"].items())) or "none"), file=out)
    print("items: " + (", ".join(f"{a}={n}" for a, n in sorted(tally["items"].items())) or "none"), file=out)
    print(f"cards credited: {len(credited)}" + (" (dry run, nothing written)" if dry else f" into {pool} ({legacy} joined LEGACY.txt)"), file=out)
    for n in credited:
        print(f"  credited {n}", file=out)
    print("cards skipped: " + (", ".join(f"{r}={by_reason[r]}" for r in REASONS if by_reason[r]) or "none"), file=out)
    for n, why in sorted(skipped.items()):
        if why == "diverge":
            kept = " (an evidence list keeps it)" if n in evidenced else ""
            print(f"  diverged {n}{kept}", file=out)
    return credited


def main(argv=None):
    p = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    p.add_argument("paths", nargs="+", metavar="PATH", help="a pulled root or a cycle dir (one holding dirac-lockstep.tsv)")
    p.add_argument("--pool", default=DEFAULT_POOL, help="the evidence list to append to (default %(default)s)")
    p.add_argument("--seats", type=int, default=2, help="the seat count of the lockstep games (default 2: dirac plays two)")
    p.add_argument("--names", action="append", default=[], metavar="FILE", help="more card names, one per line")
    p.add_argument("--root", default=os.path.dirname(HERE), help="the module root (default: this script's)")
    p.add_argument("--dry-run", action="store_true", help="report, write nothing")
    a = p.parse_args(argv)
    run(a.paths, a.root, a.pool, a.seats, a.names, a.dry_run)


if __name__ == "__main__":
    main()
