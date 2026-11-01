#!/usr/bin/env python3
"""A minimal gorge-agent/1 external agent: uniform-random picks, a random
23-non-land deck with the reference pair rule, and random legal decide
answers. Standard library only (json, random, sys); it writes nothing but
protocol lines to stdout and logs diagnostics to stderr.

The wire protocol of record is docs/site/api/gorge-agent.md. This is the
Python twin of draft/draftlab/extern/refagent (it need not agree with it bit for
bit): a starting point, not a good player.

Run it under gorge's lab with:

    go run ./cmd/labdemo -player ext:exec:examples/extagent/random_agent.py -seed 1

or serve it yourself over stdio, or (with cmd/extagent) on a Unix socket.
"""

import json
import random
import sys

PROTOCOL = "gorge-agent/1"
WUBRG = "WUBRG"
# Every view.Omit name (wire.ViewOmitNames): the policies read only the
# decision, so the leanest projection is enough.
VIEW_OMIT = [
    "library",
    "available",
    "archetype",
    "ability_costs",
    "effective_cost",
    "own_deck",
    "potential",
    "card_lists",
    "derived_chars",
]


def log(msg):
    """Diagnostics go to stderr; stdout carries protocol lines only."""
    print(msg, file=sys.stderr)


def send(obj):
    """Write one JSON line to stdout and flush it (unbuffered by design)."""
    sys.stdout.write(json.dumps(obj, separators=(",", ":")) + "\n")
    sys.stdout.flush()


def answer_pick(item, rng):
    """A uniform pick from the pack: an id that is in the pack."""
    pack = item.get("pack") or []
    if not pack:
        return {"id": item["id"], "error": {"code": "refused", "message": "empty pack"}}
    return {"id": item["id"], "pick": rng.choice(pack)["id"]}


def answer_build(item, rng):
    """The reference agent's build: the two most frequent non-land colours
    (WUBRG order breaks ties), up to 23 in-pair non-land cards (shuffled,
    topped up from the rest), the remaining slots split between the pair
    (the larger share to the first). Always build.Validate-legal."""
    pool = item.get("pool") or []
    deck_size = item.get("deck_size") or 40
    non_land = [c for c in pool if "Land" not in (c.get("type_line") or "")]
    counts = dict.fromkeys(WUBRG, 0)
    for c in non_land:
        for ch in c.get("colors") or "":
            if ch in counts:
                counts[ch] += 1
    # Stable sort keeps WUBRG order on ties, matching refagent's SliceStable.
    order = sorted(WUBRG, key=lambda l: -counts[l])
    pair = [l for l in order[:2] if counts[l] > 0]
    for l in "WU":
        if len(pair) < 2 and l not in pair:
            pair.append(l)

    def in_pair(colors):
        return all(ch in pair for ch in colors)

    matched = [c for c in non_land if in_pair(c.get("colors") or "")]
    rest = [c for c in non_land if not in_pair(c.get("colors") or "")]
    rng.shuffle(matched)
    main = matched[:23]
    need = 23 - len(main)
    if need > 0 and rest:
        rng.shuffle(rest)
        main += rest[:need]

    basics = dict.fromkeys(WUBRG, 0)
    n = deck_size - len(main)
    first = (n + 1) // 2
    basics[pair[0]] = first
    basics[pair[1]] = n - first
    return {
        "id": item["id"],
        "deck": {
            "main": [c["id"] for c in main],
            "basics": [basics[l] for l in WUBRG],
        },
    }


def answer_decide(item, rng):
    """Drop concede, then choose k uniform in [Min, min(Max, n)] distinct
    options in random order -- the reference agent's rule."""
    view = item.get("view") or {}
    decision = view.get("decision")
    if not decision:
        return {"id": item["id"], "error": {"code": "refused", "message": "no decision in view"}}
    options = [o for o in (decision.get("options") or []) if o.get("kind") != "concede"]
    n = len(options)
    hi = decision.get("max") or 0
    if hi < 0:
        hi = 0
    hi = min(hi, n)
    lo = decision.get("min") or 0
    if lo > hi:
        lo = hi
    k = lo
    if hi > lo:
        k += rng.randrange(hi - lo + 1)
    chosen = rng.sample(options, k) if k else []
    return {"id": item["id"], "intent": {"choices": [o["index"] for o in chosen]}}


def answer_item(item):
    """Answer one item; the rng is seeded from the item's own seed, so an
    answer is a pure function of the item (batch composition and order can
    never change it)."""
    rng = random.Random(item.get("seed"))
    kind = item.get("kind")
    if kind == "pick":
        return answer_pick(item, rng)
    if kind == "build":
        return answer_build(item, rng)
    if kind == "decide":
        return answer_decide(item, rng)
    return {"id": item["id"], "error": {"code": "refused", "message": "kind not served"}}


def handle_batch(msg):
    answers = [answer_item(item) for item in (msg.get("items") or [])]
    send({"v": 1, "type": "result", "batch": msg["batch"], "answers": answers})


def main():
    # gorge writes its hello first; answer with ours, then serve batches.
    line = sys.stdin.readline()
    if not line:
        return 0
    try:
        hello = json.loads(line)
    except ValueError as exc:
        log("random_agent: bad gorge hello: %s" % exc)
        return 1
    if hello.get("type") != "hello" or hello.get("protocol") != PROTOCOL:
        log("random_agent: unexpected first message: %r" % hello)
        return 1
    send(
        {
            "v": 1,
            "type": "hello",
            "protocol": PROTOCOL,
            "agent": "py-random",
            "model_version": "1",
            "kinds": ["pick", "build", "decide"],
            "view_omit": VIEW_OMIT,
        }
    )
    for line in sys.stdin:
        line = line.strip()
        if not line:
            continue
        try:
            msg = json.loads(line)
        except ValueError as exc:
            log("random_agent: bad message: %s" % exc)
            return 1
        mtype = msg.get("type")
        if mtype == "batch":
            handle_batch(msg)
        elif mtype == "stream_end":
            continue  # this agent keeps no per-stream state
        elif mtype == "bye":
            return 0
        else:
            log("random_agent: unexpected message type %r" % mtype)
            return 1
    return 0  # EOF: a clean end of stream


if __name__ == "__main__":
    sys.exit(main())
