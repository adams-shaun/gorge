"""Bucket the paper gap-A refusal census by mechanism family (the kernel capability that retires a class).
usage: python3 -I mech.py <census dir>   (reads out/gapA-class.tsv; writes mech-*.tsv beside it)"""
import collections, re, sys, os
G = sys.argv[1]
MECH = [
 ("M0 non-paper (park)", r"Perpetual|ChaosEnsues|PlaneswalkedTo|PlaneswalkedFrom|SetInMotion|Sticker|Contraption|Attraction|api:Draft|MakeCard|Specializ|Double team|\bSeek\b|Conjure|api:Abandon|PlanarDice|api:Planeswalk|kw:Visit|Hidden agenda|Vanguard|DeckLimit|\bante\b|api:Heist|NewGame|Subgame|DBPlaneswalk"),
 ("M1 token (transitive)", r"^other:token refused$"),
 ("M2 object memory store", r"Remember|Imprint|ExiledWith|Forget|Note[dC]|NoteCards|StoreSVar|ChosenCard|TempRemember|Pair|Soulbond|Haunt|Champion|ExileOnMoved|RememberedLKI|LKI\$|Clear[A-Z]|Exiled\$|Tribut|Mentor|Conspire|Replicate|Squad|SetChosen"),
 ("M3 reference algebra (SetExpr)", r"Defined|Targeted|Triggered|ParentTarget|target slot|no target|Spawner|unknown filter head|DefinedPlayer|DefinedCards|RepeatPlayers|Chooser|Choser|EffectOwner|Activator|ActivatingPlayer|AttachedTo\$|DamageSource|TargetedController|value head [A-Z][A-Za-z]*\$|Controller \"|Caster \"|referent"),
 ("M4 duration / Effect object", r"Duration|AtEOT|Until|^other:Effect |^param:Effect\.|delayed trigger|emblem|Effect static|Effect replacement|Effect trigger"),
 ("M5 filter program", r"^prop:|filter|ValidSA|Valid[A-Z][A-Za-z]*\$? \"|^count:Valid|shares|TgtZone|TargetType|ChangeType|compar|quality|^param:[A-Za-z]+\.Valid|ValidCard|ValidBlocker|ValidAttacker|ValidTgts|CardChoices"),
 ("M6 value / condition expr", r"^count:|SVar|amount|Amount|Condition|Compare|\bX=|IsPresent|CheckSVar|Repeat"),
 ("M7 keyword desugar", r"^kw:"),
 ("M8 ability-shape tables (trig/static/repl)", r"^trig:|^stat:|^repl:|^param:trigger\.|^param:static\.|replacement|trigger|TriggerZones|Mode=|AffectedZone|MayPlay|MayLook|CantBe|CantAttack|CantBlock|MustBlock|MustAttack|Taps |Attacks |Blocks |ChangesZone|AttackersDeclared|AttackerBlocked|CounterRemoved|SpellCast|AbilityCast|static|Drawn |grant"),
 ("M9 cost program", r"[Cc]ost|^cost:|kicker|morph|cumulative|alternat"),
 ("M10 mana", r"[Mm]ana"),
 ("M11 choice vocabulary", r"Choose|Choice|Vote|NameCard|TwoPiles|Piles|GenericChoice|ChooseEach"),
 ("M12 API / effect atom", r"^api:|^param:"),
 ("M13 effect shape (other)", r"."),
]
GUARD = re.compile(r"outside|shape|beside|without|with no|with an? |\bon an? |\bonly\b|other than|not an? |not the|names no|over an? |beyond|the kernel|in an? |of an? |gorge |\(|->|to an? ")
def kind(lab):
    if not lab.startswith("other:"):
        return "atom"
    if GUARD.search(lab):
        return "guard"
    if '"_"' in lab:
        return "value"
    return "guard"
def mech(lab):
    for m, rx in MECH:
        if re.search(rx, lab):
            return m
    return "?"
cards = []
for ln in open(os.path.join(G, "out/gapA-class.tsv"), encoding="utf-8"):
    r = ln.rstrip("\n").split("\t")
    if r[1] != "1":
        continue
    cards.append((r[0], r[6].split(" | ")))
car = collections.Counter(); sole = collections.Counter(); ncls = collections.defaultdict(set)
kw = collections.Counter()  # (mech, kind) carriers
classes = {}
for n, labs in cards:
    ms = set()
    for l in labs:
        m = mech(l); classes[l] = (m, kind(l))
        car[m] += 1; ncls[m].add(l); kw[(m, kind(l))] += 1; ms.add(m)
    if len(ms) == 1:
        sole[next(iter(ms))] += 1
print(f"paper gap A cards {len(cards)}, classes {len(classes)}")
print("| family | classes | carriers | sole-family cards | guard% | value% | atom% |")
for m, _ in MECH:
    c = car[m] or 1
    print(f"| {m} | {len(ncls[m])} | {car[m]} | {sole[m]} | {100*kw[(m,'guard')]//c} | {100*kw[(m,'value')]//c} | {100*kw[(m,'atom')]//c} |")
multi = sum(1 for n, labs in cards if len({mech(l) for l in labs}) > 1)
print("multi-family cards", multi)
tot = collections.Counter(k for (m,k),v in kw.items() for _ in range(v))
print("carrier kind share", {k: v for k, v in tot.items()})
with open(os.path.join(G, "mech-classes.tsv"), "w") as f:
    cc = collections.Counter(l for _, labs in cards for l in labs)
    for l, v in cc.most_common():
        f.write(f"{v}\t{classes[l][0]}\t{classes[l][1]}\t{l}\n")
with open(os.path.join(G, "mech-cards.tsv"), "w") as f:
    for n, labs in cards:
        f.write(n + "\t" + ",".join(sorted({mech(l)[:3].strip() for l in labs})) + "\t" + " | ".join(labs) + "\n")
