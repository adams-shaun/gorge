"""Add hidden (unread-after-refusal) params as pseudo-classes by family; rerun the phase sim conservatively."""
import collections, re, sys, os
G = sys.argv[1]
PFAM = [
 ("M2", r"Remember|Imprint|Forget|ExileOnMoved|Note|Chosen|Pair|Haunt|LKI"),
 ("M3", r"^Defined|Targeted|Triggered|Chooser|Choser|Activator|DefinedPlayer|DefinedCards|RepeatPlayers|TgtPrompt|TargetMin|TargetMax|TargetUnique|MayChooseTarget|TargetsWith"),
 ("M4", r"Duration|AtEOT|Until|Static|Triggers|ReplacementEffects|Effect"),
 ("M5", r"Valid|Type|ChangeType|TgtZone|Shares|Restriction"),
 ("M6", r"Condition|Amount|Num|SVar|Count|Check|IsPresent|PresentCompare|X|Repeat"),
 ("M8", r"Mode|Trigger|Replace|Affected|CharacteristicDefining|Mana"),
 ("M9", r"Cost|Kicker|Optional"),
 ("M11", r"Choice|Choose|Vote|Random"),
 ("M13", r"."),
]
paper = set()
for ln in open(os.path.join(G, "out/gapA-class.tsv")):
    r = ln.rstrip("\n").split("\t")
    if r[1] == "1": paper.add(r[0])
hidden = collections.defaultdict(set)
for ln in open(os.path.join(G, "census-all.tsv")):
    r = ln.rstrip("\n").split("\t")
    if r[0] != "ALL" or not r[4].startswith("partial:") or r[1] not in paper: continue
    m = re.search(r"\[(.*)\]", r[5])
    for p in (m.group(1).split() if m else []):
        for f, rx in PFAM:
            if re.search(rx, p):
                hidden[r[1]].add(f); break
cnt = collections.Counter(f for fs in hidden.values() for f in fs)
print("hidden families on paper gap cards:", dict(cnt))
with open(os.path.join(G, "hidden.tsv"), "w") as fh:
    for c, fs in hidden.items():
        fh.write(c + "\t" + ",".join(sorted(fs)) + "\n")
