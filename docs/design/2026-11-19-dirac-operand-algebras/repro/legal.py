"""gap-plan: per-card paper-format legality from Forge editions + formats at the corpus pin.

usage: python3 -I legal.py <res dir> <ALL.txt> <gorge-play.tsv> > legal.tsv
Output: name<TAB>paper(0/1)<TAB>formats(csv)<TAB>why-not
"""
import os
import re
import sys

res, alllist, playtsv = sys.argv[1], sys.argv[2], sys.argv[3]
EXCL_SECTIONS = {"tokens", "other", "rebalanced", "conjured"}
PAPER_TYPES = {"Core", "Expansion", "Commander", "Reprint", "Starter", "Duel_Deck", "Draft",
               "Collector_Edition", "Boxed_Set", "Multiplayer", "Promo"}
NONTOURN_TYPES = ("Plane", "Phenomenon", "Scheme", "Vanguard", "Conspiracy", "Attraction",
                  "Contraption", "Dungeon", "Stickers")
line_re = re.compile(r"^(\S+)\s+([CURMSLT])\s+(.+?)(\s+@.*)?$")

printed = {}  # name -> list of (code, type, rarity)
ed_dir = os.path.join(res, "editions")
for fn in sorted(os.listdir(ed_dir)):
    with open(os.path.join(ed_dir, fn), encoding="utf-8", errors="replace") as fh:
        code, etype, sect = None, None, None
        for raw in fh:
            ln = raw.strip()
            if not ln:
                continue
            if ln.startswith("[") and ln.endswith("]"):
                sect = ln[1:-1].lower()
                continue
            if sect == "metadata":
                if ln.startswith("Code="):
                    code = ln[5:].strip()
                elif ln.startswith("Type="):
                    etype = ln[5:].strip()
                continue
            if sect is None or sect in EXCL_SECTIONS or "token" in sect:
                continue
            m = line_re.match(ln)
            if not m:
                continue
            name = m.group(3).strip()
            name = re.sub(r"\s+\$.*$", "", name)
            printed.setdefault(name, []).append((code, etype, m.group(2)))


def readfmt(path):
    d = {}
    with open(path, encoding="utf-8") as fh:
        for raw in fh:
            if ":" in raw:
                k, v = raw.split(":", 1)
                d[k.strip()] = v.strip()
    sets = set(s.strip() for s in d.get("Sets", "").split(",") if s.strip())
    banned = set(s.strip() for s in d.get("Banned", "").split(";") if s.strip())
    return sets, banned


fmts = {}
for nm in ("Standard", "Pioneer", "Modern", "Legacy", "Vintage", "Pauper"):
    fmts[nm] = readfmt(os.path.join(res, "formats", "Sanctioned", nm + ".txt"))
fmts["Commander"] = readfmt(os.path.join(res, "formats", "Casual", "Commander.txt"))

types = {}
with open(playtsv, encoding="utf-8") as fh:
    for raw in fh:
        p = raw.rstrip("\n").split("\t")
        types[p[0]] = p[1]


def lookup(name):
    if name in printed:
        return printed[name]
    # split/aftermath cards: editions print "A // B"; ALL.txt carries the first face
    for k in (name,):
        pass
    return None


split_index = {}
for k in printed:
    if " // " in k:
        split_index.setdefault(k.split(" // ")[0], k)

with open(alllist, encoding="utf-8") as fh:
    names = [l.rstrip("\n") for l in fh if l.strip()]

for name in names:
    prints = printed.get(name) or printed.get(split_index.get(name, "\0"))
    why = ""
    ok = []
    tl = types.get(name, "")
    if name.startswith("A-"):
        why = "alchemy-rebalanced"
    elif set(tl.strip("[]").split()) & set(NONTOURN_TYPES):
        why = "nontournament-type"
    elif not prints:
        why = "no-printing"
    else:
        codes = {c for c, _, _ in prints}
        paper = [p for p in prints if p[1] in PAPER_TYPES]
        for f in ("Standard", "Pioneer", "Modern", "Legacy", "Vintage"):
            sets, banned = fmts[f]
            if codes & sets and name not in banned:
                ok.append(f)
        sets, banned = fmts["Pauper"]
        if any(r == "C" and t in PAPER_TYPES for _, t, r in prints) and name not in banned:
            ok.append("Pauper")
        sets, banned = fmts["Commander"]
        if paper and name not in banned and "Vintage" in ok:
            ok.append("Commander")
        if not ok:
            ts = sorted({t for _, t, _ in prints})
            if all(t in ("Funny",) for t in ts):
                why = "un-set"
            elif all(t in ("Online", "Funny") for t in ts):
                why = "digital-only"
            else:
                why = "banned-or-other:" + "/".join(ts)
    print("%s\t%d\t%s\t%s" % (name, 1 if ok else 0, ",".join(ok), why))
