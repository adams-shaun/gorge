"""gap-plan: farm ability ledger x dirac status. usage: python3 -I proven.py <ledger.csv> <dirac-status.tsv> <legal.tsv>"""
import collections
import csv
import sys

led, st, lg = sys.argv[1], sys.argv[2], sys.argv[3]
status = {}
with open(st, encoding="utf-8") as fh:
    for ln in fh:
        p = ln.rstrip("\n").split("\t")
        status[p[0]] = (p[1], p[2] == "true")
rows = []
with open(led, encoding="utf-8", newline="") as fh:
    for r in csv.DictReader(fh):
        rows.append(r)
cards = {}
for r in rows:
    cards.setdefault(r["card"], []).append(r)
print("ledger rows", len(rows), "distinct cards", len(cards))
b = collections.Counter()
bs = collections.Counter()
for c, rs in cards.items():
    # a card in several sets: best bucket wins in this order
    order = ["AGREEING-EVIDENCE", "DIAGNOSED-DIFFERENCE", "EXPLAINED-BY-OTHER-CARD", "BLOCKED-BY-OTHER-CARD", "HAS-DIFFERENCES", "NO-CROSS-ENGINE-EVIDENCE"]
    bk = min((r["bucket"] for r in rs), key=lambda x: order.index(x) if x in order else 99)
    s = status.get(c, ("absent", False))[0]
    b[bk] += 1
    bs[(bk, s)] += 1
print()
print("| bucket | cards | dirac supported | unverified | not lowered | refused | other |")
print("|---|---:|---:|---:|---:|---:|---:|")
for bk, n in b.most_common():
    print("| %s | %d | %d | %d | %d | %d | %d |" % (bk, n, bs[(bk, "supported")], bs[(bk, "unverified")], bs[(bk, "not lowered")], bs[(bk, "refused")],
          n - bs[(bk, "supported")] - bs[(bk, "unverified")] - bs[(bk, "not lowered")] - bs[(bk, "refused")]))
diag_cited = 0
for c, rs in cards.items():
    for r in rs:
        if r["bucket"] == "DIAGNOSED-DIFFERENCE" and r.get("diagnosed"):
            diag_cited += 1
            break
print()
print("diagnosed rows with a diagnosed citation:", diag_cited)
prov = [c for c, rs in cards.items() if status.get(c, ("",))[0] == "supported" and any(r["bucket"] in ("AGREEING-EVIDENCE", "DIAGNOSED-DIFFERENCE") for r in rs)]
print("proven (dirac supported + AGREEING or DIAGNOSED):", len(prov))
prov4 = [c for c in prov if not status[c][1]]
print("proven and 4-seat capable:", len(prov4))
nce = collections.Counter()
for c, rs in cards.items():
    for r in rs:
        if r["bucket"] == "NO-CROSS-ENGINE-EVIDENCE":
            nce[(r.get("cnc_why") or r.get("no_dirac_why") or "-")[:60]] += 1
            break
print()
print("NO-CROSS-ENGINE-EVIDENCE why:", nce.most_common(12))
hd = collections.Counter()
for c, rs in cards.items():
    for r in rs:
        if r["bucket"] in ("HAS-DIFFERENCES", "BLOCKED-BY-OTHER-CARD", "EXPLAINED-BY-OTHER-CARD"):
            hd[(r["bucket"], (r.get("primary_classes") or r.get("blocked_by") or "-").split(";")[0][:50])] += 1
            break
print()
print("difference heads:", hd.most_common(20))
