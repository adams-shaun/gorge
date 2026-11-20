"""Phase simulation over paper gap A by mechanism family. usage: python3 -I sim3.py <census dir>"""
import collections, sys, os
G = sys.argv[1]
exp = set(l.strip() for l in open(os.path.join(G, "gorge-kw-expanders.txt")))
cls = {}
for ln in open(os.path.join(G, "mech-classes.tsv")):
    v, m, k, l = ln.rstrip("\n").split("\t")
    cls[l] = (m.split()[0], k, int(v))
cards = []
for ln in open(os.path.join(G, "mech-cards.tsv")):
    n, ms, labs = ln.rstrip("\n").split("\t")
    cards.append((n, labs.split(" | ")))
def fam(l): return cls[l][0]
PH = [
 ("D1 keyword pass-through", lambda l: fam(l) == "M7" and l[3:] in exp, lambda l: fam(l) == "M7" and l[3:] in exp),
 ("D2 SetExpr + Defined bases", lambda l: fam(l) == "M3" and cls[l][1] != "atom", lambda l: fam(l) == "M3"),
 ("D3 object-set store", lambda l: fam(l) == "M2" and cls[l][1] != "atom", lambda l: fam(l) == "M2"),
 ("D4 filter programs + word port", lambda l: fam(l) == "M5" and cls[l][1] != "atom", lambda l: fam(l) == "M5"),
 ("D5 durations + Effect containers", lambda l: fam(l) == "M4" and cls[l][1] != "atom", lambda l: fam(l) == "M4"),
 ("D6 Expr/condition vocab", lambda l: fam(l) == "M6" and cls[l][1] != "atom", lambda l: fam(l) == "M6"),
 ("D7 trig/static/repl mode tables", lambda l: fam(l) == "M8" and cls[l][1] != "atom", lambda l: fam(l) == "M8"),
 ("D8 cost program + mana", lambda l: fam(l) in ("M9", "M10") and cls[l][1] != "atom", lambda l: fam(l) in ("M9", "M10")),
 ("D9 choice vocabulary", lambda l: fam(l) == "M11" and cls[l][1] != "atom", lambda l: fam(l) == "M11"),
 ("W keywords (rest)", lambda l: False, lambda l: fam(l) == "M7"),
 ("W API atoms + effect shapes", lambda l: False, lambda l: fam(l) in ("M12", "M13")),
]
for mode, idx in (("lower: mechanism only (guard+value classes)", 1), ("upper: mechanism + family vocabulary port", 2)):
    print("##", mode)
    done = set(); prev = 0
    print("| phase | classes retired | carrier-rows | paper cards unlocked | cumulative | supported paper % |")
    for name, lo, up in PH:
        f = lo if idx == 1 else up
        new = {l for l in cls if f(l)} - done
        done |= new
        ok = sum(1 for n, labs in cards if all(l in done for l in labs))
        cr = sum(cls[l][2] for l in new)
        print(f"| {name} | {len(new)} | {cr} | {ok-prev} | {ok} | {100*(24574+ok)/32530:.1f} |")
        prev = ok

# conservative: a card also needs every family its hidden (unread-after-refusal) params name
hid = {}
for ln in open(os.path.join(G, "hidden.tsv")):
    c, fs = ln.rstrip("\n").split("\t"); hid[c] = set(fs.split(","))
PFAMOF = {"D1": set(), "D2": {"M3"}, "D3": {"M2"}, "D4": {"M5"}, "D5": {"M4"}, "D6": {"M6"}, "D7": {"M8"}, "D8": {"M9", "M10"}, "D9": {"M11"}, "W": set()}
print("## conservative upper: hidden params must also be in a done family")
done = set(); fams = set(); prev = 0
print("| phase | paper cards unlocked | cumulative | supported paper % |")
for name, lo, up in PH:
    new = {l for l in cls if up(l)} - done; done |= new
    key = name.split()[0]
    fams |= PFAMOF.get(key, set())
    if name.startswith("W API"): fams |= {"M12", "M13"}
    ok = sum(1 for n, labs in cards if all(l in done for l in labs) and hid.get(n, set()) <= fams)
    print(f"| {name} | {ok-prev} | {ok} | {100*(24574+ok)/32530:.1f} |"); prev = ok
