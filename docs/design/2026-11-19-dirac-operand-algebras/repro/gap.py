"""gap-plan analysis. usage: python3 -I gap.py <gapdata dir> <out dir>

Inputs (all under gapdata):
  gorge-play.tsv   name, types, playable(0/1), missing primitives   (forgecx list)
  dirac-status.tsv name, status, twoSeat, twoSeatWhy                (kgen.SupportFor over ALL.txt)
  legal.tsv        name, paper(0/1), formats, why-not               (legal.py)
  census-first.tsv kgen -census-std -tsv                            (first blocker per node)
  census-all.tsv   kgen -census-std -all-blockers -tsv              (every blocker)
"""
import collections
import os
import re
import sys

gd, out = sys.argv[1], sys.argv[2]
KEY = sys.argv[3] if len(sys.argv) > 3 else "label"


def rows(fn):
    with open(os.path.join(gd, fn), encoding="utf-8") as fh:
        for ln in fh:
            yield ln.rstrip("\n").split("\t")


play, ptypes, gmiss = {}, {}, {}
for r in rows("gorge-play.tsv"):
    play[r[0]] = r[2] == "1"
    ptypes[r[0]] = r[1]
    gmiss[r[0]] = [x for x in r[3].split(",") if x] if len(r) > 3 else []
status, twoseat, twowhy = {}, {}, {}
for r in rows("dirac-status.tsv"):
    status[r[0]] = r[1]
    twoseat[r[0]] = r[2] == "true"
    twowhy[r[0]] = r[3] if len(r) > 3 else ""
paper, pwhy, pfmts = {}, {}, {}
for r in rows("legal.tsv"):
    paper[r[0]] = r[1] == "1"
    pfmts[r[0]] = r[2]
    pwhy[r[0]] = r[3]


def blockers(fn):
    first = {}
    labels = collections.defaultdict(list)
    for r in rows(fn):
        if r[0] != "ALL" or len(r) < 7 or r[4] == "" or r[4].startswith("partial:"):
            continue
        card, via, node, cls, reason, label = r[1], r[2], r[3], r[4], r[5], r[6]
        if cls == "other:token refused" and via == "":
            lab = "other:token refused"
        else:
            lab = (label or cls) if KEY == "label" else cls
        if card not in first:
            first[card] = (lab, node, reason, via)
        if lab not in labels[card]:
            labels[card].append(lab)
    # drop the bare "token refused" when the token's own blockers are listed
    for c, ls in labels.items():
        if len(ls) > 1 and "other:token refused" in ls:
            ls.remove("other:token refused")
    return first, labels


first, _ = blockers("census-first.tsv")
_, allb = blockers("census-all.tsv")

FAMILIES = [
    ("F0 digital / non-paper mechanic", r"Perpetual|ChaosEnsues|PlaneswalkedTo|PlaneswalkedFrom|SetInMotion|Sticker|Contraption|Attraction|api:Draft|MakeCard|Specializ|Double team|\bSeek\b|Conjure|api:Abandon|PlanarDice|api:Planeswalk|kw:Visit|Hidden agenda|Vanguard|DeckLimit|\bante\b|api:Heist|NewGame|Subgame"),
    ("F1 token (its token lowers short)", r"^other:token refused$"),
    ("F2 object memory (Remembered/Imprinted/ExiledWith/Chosen/LKI)", r"Remember|Imprint|ExiledWith|ChosenCard|StoreSVar|LKI|TempRemember|Forget|ChosenNumber|ChosenColor|ChosenType|NamedCard"),
    ("F3 reference resolution (Defined$/Triggered*/Targeted)", r"Defined|Targeted|Triggered|ParentTarget|target slot|Spawner|no target"),
    ("F4 keyword", r"^kw:"),
    ("F5 effect objects and durations (Effect/AtEOT/delayed)", r"^other:Effect |Effect\.|Duration|AtEOT|delayed trigger|Until"),
    ("F6 replacement", r"^repl:|replacement|Replace|ETB counter"),
    ("F7 cost", r"[Cc]ost|^cost:|kicker|morph|cumulative"),
    ("F8 filter compiler", r"^prop:|filter|ValidSA|ValidCard|ValidTgts|ValidActivating|ValidTarget|^count:Valid$|shares|TgtZone|TargetType|ChangeType|ValidBlocker|ValidAttacker"),
    ("F9 static", r"^stat:|static|MayPlay|MayLookAt|CantBe|AffectedZone|Protection|^param:static"),
    ("F10 trigger", r"^trig:|trigger|TriggerZones|Mode=|ChangesZone |AttackersDeclared|AttackerBlocked|CounterRemoved|SpellCast"),
    ("F11 mana ability", r"[Mm]ana"),
    ("F12 count / SVar / condition / repeat", r"^count:|SVar|Condition|amount|Repeat|\bX="),
    ("F13 choice / vote / pile", r"Choose|Choice|Vote|NameCard|TwoPiles|api:Pair|Chooser"),
    ("F14 API (missing effect)", r"^api:"),
    ("F15 unread param", r"^param:"),
    ("F16 zone move / copy / animate / other effect shape", r"."),
]


def family(lab):
    for f, rx in FAMILIES:
        if re.search(rx, lab):
            return f
    return "other"


names = sorted(status)
gapA = [n for n in names if status[n] == "not lowered" and play.get(n)]
gapA_np = [n for n in names if status[n] == "not lowered" and n not in play]
neither = [n for n in names if status[n] == "not lowered" and play.get(n) is False]
dirac_only = [n for n in names if status[n] != "not lowered" and play.get(n) is False]
unver = [n for n in names if status[n] == "unverified"]
refused = [n for n in names if status[n] == "refused"]
gorge_unplay = [n for n in play if not play[n]]

lines = []
P = lines.append
P("# gap-plan measurements")
P("")
P("| set | cards | paper |")
P("|---|---:|---:|")


def cnt(ns):
    return len(ns), sum(1 for n in ns if paper.get(n))


for lab, ns in [
    ("corpus (ALL.txt)", names),
    ("gorge playable", [n for n in names if play.get(n)]),
    ("gorge unplayable", [n for n in names if play.get(n) is False]),
    ("dirac lowered", [n for n in names if status[n] != "not lowered"]),
    ("dirac supported", [n for n in names if status[n] == "supported"]),
    ("dirac supported, 4-seat", [n for n in names if status[n] == "supported" and not twoseat[n]]),
    ("dirac unverified", unver),
    ("dirac refused", refused),
    ("GAP A: gorge plays, dirac does not lower", gapA),
    ("dirac does not lower, gorge cannot play either", neither),
    ("dirac lowers, gorge cannot play (should be 0)", dirac_only),
    ("not lowered and absent from gorge list", gapA_np),
]:
    c, p = cnt(ns)
    P("| %s | %d | %d |" % (lab, c, p))
P("")
P("paper why-not over GAP A: " + str(collections.Counter(pwhy[n] or "paper" for n in gapA).most_common()))
P("")
P("paper why-not over corpus: " + str(collections.Counter(pwhy[n] or "paper" for n in names).most_common()))
P("")

# first-blocker class census on GAP A
fam_first = collections.Counter()
fam_first_p = collections.Counter()
lab_first = collections.Counter()
lab_first_p = collections.Counter()
for n in gapA:
    lab = first.get(n, ("(no row)",))[0]
    lab_first[lab] += 1
    fam_first[family(lab)] += 1
    if paper.get(n):
        lab_first_p[lab] += 1
        fam_first_p[family(lab)] += 1
P("## GAP A by first blocker family")
P("")
P("| family | cards | paper |")
P("|---|---:|---:|")
for f, c in fam_first.most_common():
    P("| %s | %d | %d |" % (f, c, fam_first_p[f]))
P("")

# all-blocker census
nblk = collections.Counter()
carry = collections.Counter()
carry_p = collections.Counter()
sole = collections.Counter()
sole_p = collections.Counter()
for n in gapA:
    ls = allb.get(n) or [first.get(n, ("(no row)",))[0]]
    allb[n] = ls
    nblk[min(len(ls), 6)] += 1
    for lab in ls:
        carry[lab] += 1
        if paper.get(n):
            carry_p[lab] += 1
    if len(ls) == 1:
        sole[ls[0]] += 1
        if paper.get(n):
            sole_p[ls[0]] += 1
P("blockers per GAP A card (6 = 6+): " + str(sorted(nblk.items())))
P("")
P("distinct labels over GAP A: %d; over paper GAP A: %d" % (len(carry), sum(1 for l in carry_p if carry_p[l])))
P("")

# family on all blockers: cards whose every blocker is in one family
famcards = collections.Counter()
famcards_p = collections.Counter()
for n in gapA:
    fs = {family(l) for l in allb[n]}
    key = fs.pop() if len(fs) == 1 else "(several families)"
    famcards[key] += 1
    if paper.get(n):
        famcards_p[key] += 1
P("## GAP A: cards whose blockers all sit in one family")
P("")
P("| family | cards | paper |")
P("|---|---:|---:|")
for f, c in famcards.most_common():
    P("| %s | %d | %d |" % (f, c, famcards_p[f]))
P("")

# greedy unlock curve over paper GAP A cards, by label
pg = [n for n in gapA if paper.get(n)]
need = {n: set(allb[n]) for n in pg}
fixed = set()
done = 0
order = []
total = len(pg)
marks = {0.5: None, 0.8: None, 0.95: None, 1.0: None}
rem = dict(need)
while rem:
    # label whose fixing completes most cards; tie -> most carriers among remaining
    comp = collections.Counter()
    car = collections.Counter()
    for n, s in rem.items():
        left = s - fixed
        if len(left) == 1:
            comp[next(iter(left))] += 1
        for l in left:
            car[l] += 1
    best = max(car, key=lambda l: (comp[l], car[l], l))
    fixed.add(best)
    newly = [n for n, s in rem.items() if s <= fixed]
    for n in newly:
        del rem[n]
    done += len(newly)
    order.append((best, family(best), len(newly), done, car[best]))
    for m in marks:
        if marks[m] is None and done >= m * total:
            marks[m] = len(order)
P("## Unlock curve (paper GAP A, %d cards; greedy by labels)" % total)
P("")
P("labels to reach 50%%: %s, 80%%: %s, 95%%: %s, 100%%: %s" % (marks[0.5], marks[0.8], marks[0.95], marks[1.0]))
P("")
P("| step | label | family | cards completed | running | running % | carriers left |")
P("|---:|---|---|---:|---:|---:|---:|")
for i, (l, f, k, d, c) in enumerate(order[:150]):
    P("| %d | %s | %s | %d | %d | %.1f | %d |" % (i + 1, l.replace("|", "\\|"), f, k, d, 100.0 * d / total, c))
P("")

# family unlock curve: fix whole families
famneed = {n: {family(l) for l in need[n]} for n in pg}
ff = set()
rem = dict(famneed)
forder = []
dn = 0
while rem:
    comp = collections.Counter()
    car = collections.Counter()
    for n, s in rem.items():
        left = s - ff
        if len(left) == 1:
            comp[next(iter(left))] += 1
        for l in left:
            car[l] += 1
    best = max(car, key=lambda l: (comp[l], car[l], l))
    ff.add(best)
    newly = [n for n, s in rem.items() if s <= ff]
    for n in newly:
        del rem[n]
    dn += len(newly)
    nl = len({l for n in pg for l in need[n] if family(l) == best})
    forder.append((best, len(newly), dn, nl))
P("## Family unlock order (paper GAP A)")
P("")
P("| step | family | labels in family | cards completed | running | running % |")
P("|---:|---|---:|---:|---:|---:|")
for i, (f, k, d, nl) in enumerate(forder):
    P("| %d | %s | %d | %d | %d | %.1f |" % (i + 1, f, nl, k, d, 100.0 * d / total))
P("")

P("## Top labels by paper carriers (GAP A)")
P("")
P("| label | family | carriers | paper carriers | sole | paper sole |")
P("|---|---|---:|---:|---:|---:|")
for l, c in sorted(carry_p.items(), key=lambda x: (-x[1], x[0]))[:120]:
    P("| %s | %s | %d | %d | %d | %d |" % (l.replace("|", "\\|"), family(l), carry[l], c, sole[l], sole_p[l]))
P("")

# gorge's own gap
gm = collections.Counter()
gm_p = collections.Counter()
gsole = collections.Counter()
gsole_p = collections.Counter()
for n in gorge_unplay:
    for m in gmiss[n]:
        gm[m] += 1
        if paper.get(n):
            gm_p[m] += 1
    if len(gmiss[n]) == 1:
        gsole[gmiss[n][0]] += 1
        if paper.get(n):
            gsole_p[gmiss[n][0]] += 1
P("## gorge unplayable: %d cards, %d paper" % (len(gorge_unplay), sum(1 for n in gorge_unplay if paper.get(n))))
P("")
P("| primitive | cards | paper | sole | paper sole |")
P("|---|---:|---:|---:|---:|")
for m, c in gm.most_common(70):
    P("| %s | %d | %d | %d | %d |" % (m, c, gm_p[m], gsole[m], gsole_p[m]))
P("")

# evidence side
P("## unverified: %d (paper %d); 2-seat-only among them %d" % (len(unver), sum(1 for n in unver if paper.get(n)), sum(1 for n in unver if twoseat[n])))
P("")
sup = [n for n in names if status[n] == "supported"]
ts = [n for n in sup if twoseat[n]]
P("supported but 2-seat only: %d (paper %d)" % (len(ts), sum(1 for n in ts if paper.get(n))))
why = collections.Counter()
for n in names:
    if status[n] in ("supported", "unverified") and twoseat[n]:
        for w in twowhy[n].split(";"):
            if w:
                why[w[:70]] += 1
P("")
P("two-seat reasons over lowered cards: " + str(why.most_common(20)))
P("")

with open(os.path.join(out, "gap-measure-" + KEY + ".md"), "w", encoding="utf-8") as fh:
    fh.write("\n".join(lines) + "\n")
with open(os.path.join(out, "gapA-" + KEY + ".tsv"), "w", encoding="utf-8") as fh:
    for n in gapA:
        f = first.get(n, ("(no row)", "", "", ""))
        fh.write("%s\t%d\t%s\t%s\t%s\t%s\t%s\n" % (n, 1 if paper.get(n) else 0, pwhy[n], f[0], family(f[0]), f[2][:160], " | ".join(allb[n])))
with open(os.path.join(out, "unverified.tsv"), "w", encoding="utf-8") as fh:
    for n in unver:
        fh.write("%s\t%d\t%s\t%s\n" % (n, 1 if paper.get(n) else 0, twoseat[n], pfmts[n]))
print("ok", len(gapA), total)
