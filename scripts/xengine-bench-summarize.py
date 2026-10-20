#!/usr/bin/env python3
"""Helpers and the summary table for scripts/xengine-bench.sh (stdlib only).

  xengine-bench-summarize.py RUNS.jsonl
      print the per-row table: median and min-max over reps
  xengine-bench-summarize.py --record ENGINE ROW PAIR REP CORE LOAD0 TIMEFILE RESULT KIND LOAD1
      print one normalised run line (KIND: enginebench, xmage, kplay, forge:WARM:SECS)
  xengine-bench-summarize.py --forge-games DECKS DECK_A DECK_B SEED N
      print N forge.oracle.GameReplay request lines, seats alternating
  xengine-bench-summarize.py --kplay-decks DECKS DECK_A DECK_B DECKFILE PAIRSFILE SEED
      write a kplay -deckfile and a -pairs-file of 200000 games, seats alternating
"""
import json
import os
import re
import statistics
import sys


def dck_cards(decks, name):
    """A .dck deck list ("N [SET:NUM] Name") as [{"name", "count"}]."""
    cards = []
    with open(os.path.join(decks, name + ".dck")) as f:
        for line in f:
            m = re.match(r"^(\d+)\s+(?:\[[^\]]*\]\s+)?(.+?)\s*$", line)
            if m:
                cards.append({"name": m.group(2), "count": int(m.group(1))})
    return cards


def forge_games(decks, a, b, seed, n):
    ca, cb = dck_cards(decks, a), dck_cards(decks, b)
    for g in range(int(n)):
        seats = [{"name": a, "cards": ca}, {"name": b, "cards": cb}]
        if g % 2:
            seats.reverse()
        print(json.dumps({"id": "%s-%s-%d" % (a, b, g), "seed": int(seed) + g,
                          "max_turns": 100, "max_ms": 600000, "seats": seats}))


def kplay_decks(decks, a, b, deckfile, pairsfile, seed):
    with open(deckfile, "w") as f:
        for name in (a, b):
            f.write(json.dumps({"name": name, "cards": dck_cards(decks, name)}) + "\n")
    with open(pairsfile, "w") as f:
        for g in range(200000):
            x, y = (a, b) if g % 2 == 0 else (b, a)
            f.write("%s\t%s\t%d\n" % (x, y, int(seed) + g))


def record(engine, row, pair, rep, core, load0, timefile, result, kind, load1):
    # GNU time prepends "Command exited with non-zero status N" (the Forge run
    # is stopped with a signal); the JSON is the last line.
    tl = [l for l in open(timefile)] if os.path.exists(timefile) else []
    t = json.loads(tl[-1]) if tl else {}
    games = turns = decisions = None
    cpu_s = None
    warm = 0
    errors = 0
    if kind == "enginebench":
        r = json.loads(open(result).read().strip().splitlines()[-1])
        games, turns, decisions, cpu_s = r["games"], r["turns"], r["decisions"], r["cpu_secs"]
        errors = r["detail"].get("Stalls", 0)
    elif kind == "xmage":
        s = [json.loads(l) for l in open(result) if '"summary"' in l][-1]
        games, turns, decisions, cpu_s = s["games"], s["turns"], s["decisions"], s["cpu_s"]
        warm, errors = s["warmup_games"], s["no_winner"]
    elif kind == "kplay":
        s = json.load(open(result))
        rows = [json.loads(l) for l in open(result[: -len(".summary.json")]) if l.strip()]
        games, cpu_s = s["games"], s["cpu_s"]
        turns = sum(r["turns"] for r in rows)
        decisions = sum(r["intents"] for r in rows)
        errors = s.get("stalls", 0) + s.get("errors", 0)
    elif kind.startswith("forge:"):
        _, w, m = kind.split(":")
        rows = [json.loads(l) for l in open(result) if l.strip()]
        acc, i = 0, 0
        while i < len(rows) and acc < float(w) * 1000:
            acc += rows[i].get("ms", 0)
            i += 1
        warm = i
        meas = []
        acc = 0
        for r in rows[i:]:
            if acc >= float(m) * 1000:
                break
            meas.append(r)
            acc += r.get("ms", 0)
        games = len(meas)
        turns = sum(r.get("turns", 0) for r in meas)
        errors = sum(1 for r in meas if "harness" in r)
        cpu_s = acc / 1000.0  # wall per game on one pinned core
    other = None
    base = timefile[: -len(".time")]
    if os.path.exists(base + ".stat0") and os.path.exists(base + ".stat1"):
        a = [int(x) for x in open(base + ".stat0").read().split()[1:]]
        b = [int(x) for x in open(base + ".stat1").read().split()[1:]]
        d = [y - x for x, y in zip(a, b)]
        total, idle = sum(d[:8]), d[3] + d[4]
        hz = os.sysconf("SC_CLK_TCK")
        own = (t.get("user_s") or 0) + (t.get("sys_s") or 0)
        if total:
            # The core's busy time beyond the run's own CPU, as a share of the window.
            other = max(0.0, ((total - idle) / hz - own) / (total / hz))
    out = {"engine": engine, "row": row, "pair": pair, "rep": int(rep), "core": int(core),
           "load1_before": float(load0), "load1_after": float(load1),
           "maxrss_mb": round(t.get("maxrss_kb", 0) / 1024.0, 1),
           "proc_user_s": t.get("user_s"), "proc_sys_s": t.get("sys_s"), "proc_wall_s": t.get("wall_s"),
           "warmup_games": warm, "games": games, "turns": turns, "decisions": decisions,
           "cpu_s": cpu_s, "errors": errors,
           "other_busy": None if other is None else round(other, 3)}
    if games and cpu_s:
        out["games_per_cpu_s"] = games / cpu_s
        out["ms_per_game"] = 1000.0 * cpu_s / games
        out["us_per_turn"] = 1e6 * cpu_s / turns if turns else None
        out["us_per_decision"] = 1e6 * cpu_s / decisions if decisions else None
    print(json.dumps(out))


def fmt(v):
    if v is None:
        return "n/a"
    if v >= 100:
        return "{:,.0f}".format(v)
    if v >= 10:
        return "{:.1f}".format(v)
    return "{:.2f}".format(v)


def spread(vals):
    vals = [v for v in vals if v is not None]
    if not vals:
        return "n/a"
    return "%s (%s-%s)" % (fmt(statistics.median(vals)), fmt(min(vals)), fmt(max(vals)))


def summary(path):
    runs = [json.loads(l) for l in open(path) if l.strip()]
    groups = {}
    for r in runs:
        groups.setdefault((r["engine"], r["row"], r["pair"]), []).append(r)
    print("| engine | policy | pair | reps | games/core-s | ms/game | µs/turn | µs/decision | turns/game | peak RSS MB | load1 | other busy |")
    print("|---|---|---|---|---|---|---|---|---|---|---|---|")
    for (e, row, pair), rs in groups.items():
        loads = [x for r in rs for x in (r["load1_before"], r["load1_after"])]
        tpg = [r["turns"] / r["games"] for r in rs if r.get("games") and r.get("turns")]
        ob = [r["other_busy"] for r in rs if r.get("other_busy") is not None]
        print("| %s | %s | %s | %d | %s | %s | %s | %s | %s | %s | %.0f-%.0f | %s |" % (
            e, row, pair, len(rs), spread([r.get("games_per_cpu_s") for r in rs]),
            spread([r.get("ms_per_game") for r in rs]), spread([r.get("us_per_turn") for r in rs]),
            spread([r.get("us_per_decision") for r in rs]), fmt(statistics.median(tpg)) if tpg else "n/a",
            fmt(statistics.median([r["maxrss_mb"] for r in rs])), min(loads), max(loads),
            "max %.0f%%" % (100 * max(ob)) if ob else "n/a"))


def main(argv):
    if len(argv) >= 2 and argv[1] == "--record":
        record(*argv[2:12])
    elif len(argv) >= 2 and argv[1] == "--forge-games":
        forge_games(*argv[2:7])
    elif len(argv) >= 2 and argv[1] == "--kplay-decks":
        kplay_decks(*argv[2:8])
    elif len(argv) == 2:
        summary(argv[1])
    else:
        sys.stderr.write(__doc__)
        return 2
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
