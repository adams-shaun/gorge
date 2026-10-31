#!/usr/bin/env python3
"""A gorge seat played over HTTP, standard library only.

The engine asks a decision; the agent answers with option indices. That is
the whole contract: gorge lists every legal option, validates every answer,
and refuses (HTTP 409) anything it did not offer, so a bot can choose badly
but never illegally.

Run it against a gorged started with -vsbot (it creates its own game):

    python3 gorge_agent.py --server http://127.0.0.1:8080 --policy simple

or join a seat you already hold (gorged -humans 0 -seat-token TOKEN):

    python3 gorge_agent.py --server http://127.0.0.1:8080 \
        --table t1 --match 1 --seat 0 --token TOKEN

To write your own strategy, subclass Agent and override choose(); see
RandomAgent and SimpleAgent below.
"""

import argparse
import json
import random
import sys
import time
import urllib.error
import urllib.request


# --------------------------------------------------------------------------
# The HTTP seat client: four endpoints.
# --------------------------------------------------------------------------

class SeatClient:
    """One seat at one match. The token is the only credential: the server
    derives the seat from it and never trusts a seat number the client sends."""

    def __init__(self, server, table, match, seat, token):
        self.server = server.rstrip("/")
        self.table, self.match, self.seat, self.token = table, match, seat, token

    def _call(self, method, path, body=None):
        data = None if body is None else json.dumps(body).encode()
        req = urllib.request.Request(self.server + path, data=data, method=method)
        req.add_header("Authorization", "Bearer " + self.token)
        if data is not None:
            req.add_header("Content-Type", "application/json")
        try:
            with urllib.request.urlopen(req, timeout=30) as resp:
                raw = resp.read()
                return resp.status, (json.loads(raw) if raw else None)
        except urllib.error.HTTPError as e:
            raw = e.read()
            try:
                return e.code, json.loads(raw)
            except ValueError:
                return e.code, {"code": "http", "message": raw.decode(errors="replace")}

    def _match_path(self, tail):
        return "/api/tables/%s/matches/%d/%s" % (self.table, self.match, tail)

    def pending(self):
        """The decision asked of this seat, or None (409: nothing pending)."""
        status, body = self._call("GET", self._match_path("pending?seat=%d" % self.seat))
        if status == 200:
            return body
        if status == 409:
            return None
        raise RuntimeError("pending: HTTP %d %s" % (status, body))

    def view(self):
        """This seat's redacted view of the game (own hand, public board)."""
        status, body = self._call("GET", self._match_path("view?seat=%d" % self.seat))
        if status != 200:
            raise RuntimeError("view: HTTP %d %s" % (status, body))
        return body

    def submit(self, decision, answer):
        """POST an intent. answer is a list of option indices, or a dict such
        as {"payment": {...}} for a cast paid by an offered plan. Returns None
        on success (204), else the error body."""
        intent = {"seq": decision["seq"], "player": decision["player"]}
        if isinstance(answer, dict):
            intent.update(answer)
        else:
            intent["choices"] = answer
        status, body = self._call("POST", self._match_path("intent"), intent)
        return None if status == 204 else (body or {"code": str(status)})

    def match_state(self):
        """This match's row of GET /api/tables/{t}/matches (state, winner...)."""
        status, body = self._call("GET", "/api/tables/%s/matches" % self.table)
        if status == 404:
            return None
        if status != 200:
            raise RuntimeError("matches: HTTP %d %s" % (status, body))
        for row in body:
            if row["match"] == self.match:
                return row
        return None


def create_game(server, fmt="constructed", bot_policy="", human_deck="", bot_deck="",
                mulligans=None):
    """POST /api/games: a fresh private table, us against a hosted bot."""
    body = {"format": fmt}
    for key, val in (("bot_policy", bot_policy), ("human_deck", human_deck),
                     ("bot_deck", bot_deck)):
        if val:
            body[key] = val
    if mulligans is not None:
        body["mulligans"] = mulligans
    req = urllib.request.Request(server.rstrip("/") + "/api/games",
                                 data=json.dumps(body).encode(), method="POST")
    req.add_header("Content-Type", "application/json")
    try:
        with urllib.request.urlopen(req, timeout=30) as resp:
            return json.loads(resp.read())
    except urllib.error.HTTPError as e:
        sys.exit("create game: HTTP %d %s" % (e.code, e.read().decode(errors="replace")))


# --------------------------------------------------------------------------
# Answer helpers: the wire rules every answer must meet.
# --------------------------------------------------------------------------

def exclusive_keys(d, o):
    """The keys of which an answer may hold at most one option: the option's
    Group, and on an attackers decision its creature (one creature attacks
    one defender; the wire does not mark that with a Group, and an answer
    that breaks it is accepted by the HTTP layer but refused by the rules,
    which ends the match, so a client must enforce it itself)."""
    keys = []
    if o.get("group"):
        keys.append(("group", o["group"]))
    if d["kind"] == "attackers" and o.get("obj"):
        keys.append(("attacker", o["obj"]))
    return keys


def legal_random(d, rng, pool=None):
    """A random answer that respects Min/Max, required options and the
    exclusivity keys above. Budgets (maxSum/minSum) and a few whole-declaration
    rules are not modelled here; the server refuses those answers with a 409
    and the caller tries the next candidate."""
    options = [o for o in (d["options"] if pool is None else pool) if o["kind"] != "concede"]
    picked, taken = [], set()
    for o in options:
        if o.get("required"):
            picked.append(o["index"])
            taken.update(exclusive_keys(d, o))
    rest = [o for o in options if o["index"] not in picked]
    rng.shuffle(rest)
    want = rng.randint(d["min"], max(d["min"], d["max"]))
    for o in rest:
        if len(picked) >= want:
            break
        keys = exclusive_keys(d, o)
        if any(k in taken for k in keys):
            continue
        picked.append(o["index"])
        taken.update(keys)
    if d["kind"] in ("trigger_order", "arrange"):
        rng.shuffle(picked)  # an order: any permutation of the picks is legal
    return picked


def minimal(d):
    """The least committal answer: pass at priority, else the first Min."""
    if d["kind"] == "priority":
        for o in d["options"]:
            if o["kind"] == "pass":
                return [o["index"]]
    return [o["index"] for o in d["options"][: d["min"]]]


def by_kind(d, kind):
    return [o for o in d["options"] if o["kind"] == kind]


def pay(action):
    """Cast through an offered payment plan: the engine taps the lands. On an
    auto-mana table a spell you cannot pay from the floating pool is offered
    ONLY this way (a payment action), not as a "cast" option."""
    return {"payment": {"action_id": action["id"], "plan": action["plans"][0]}}


def plays(d):
    """Every priority play as (answer, object id): plan-paid casts first,
    then the option list's non-pass, non-concede entries."""
    out = [(pay(a), a["cast"]["object"]) for a in d.get("payment_actions") or [] if a["plans"]]
    out += [([o["index"]], o.get("obj")) for o in d["options"]
            if o["kind"] not in ("pass", "concede")]
    return out


def cmc(card):
    """Mana value from the view's cost string, e.g. '1 W W' -> 3."""
    total = 0
    for sym in (card.get("mana_cost") or "").split():
        if sym.isdigit():
            total += int(sym)
        elif sym not in ("no", "cost", "X"):
            total += 1
    return total


# --------------------------------------------------------------------------
# Strategies. Override choose(); return a list of option indices.
# --------------------------------------------------------------------------

class Agent:
    needs_view = False  # set True to receive the seat's view in choose()

    def __init__(self, seed=1):
        self.rng = random.Random(seed)

    def choose(self, d, view):
        raise NotImplementedError

    def candidates(self, d, view):
        """Answers to try in order: the policy's, a few random legal ones,
        then the minimal one. The server is the judge; a 409 moves on."""
        yield self.choose(d, view)
        for _ in range(8):
            yield legal_random(d, self.rng)
        yield minimal(d)


class RandomAgent(Agent):
    """Uniform over legal answers, except it never concedes and passes
    priority half the time so it does not spin on repeatable abilities."""

    def choose(self, d, view):
        if d["kind"] == "priority":
            options = plays(d)
            if not options or self.rng.random() < 0.5:
                return minimal(d)
            return self.rng.choice(options)[0]
        return legal_random(d, self.rng)


class SimpleAgent(Agent):
    """Land first, then the most expensive spell it can cast; attacks with a
    creature only when no untapped enemy creature could kill it; blocks only
    when the blocker survives; keeps every hand and goes first."""

    needs_view = True

    def choose(self, d, view):
        me = d["player"]
        cards = {}  # object id -> card view, every zone we can see
        for p in view["players"]:
            for zone in ("hand", "battlefield"):
                for c in p.get(zone) or []:
                    cards[c["id"]] = c
        kind = d["kind"]
        if kind == "mulligan":
            keep = by_kind(d, "keep")
            if keep:
                return [keep[0]["index"]]
            return legal_random(d, self.rng)  # bottoming: any cards
        if kind == "starting_player":
            return [o["index"] for o in d["options"] if o["player"] == me][:1]
        if kind == "priority":
            lands = by_kind(d, "play_land")
            if lands:
                return [lands[0]["index"]]
            casts = [(pay(a), a["cast"]["object"]) for a in d.get("payment_actions") or []
                     if a["plans"]]
            casts += [([o["index"]], o.get("obj")) for o in by_kind(d, "cast")]
            if casts:
                return max(casts, key=lambda c: cmc(cards.get(c[1], {})))[0]
            return minimal(d)
        if kind == "attackers":
            enemy = [c for p in view["players"] if p["seat"] != me
                     for c in p.get("battlefield") or []
                     if "Creature" in c["types"] and not c["tapped"]]
            biggest = max([c["power"] for c in enemy], default=0)
            picked, taken = [], set()
            for o in d["options"]:
                c = cards.get(o.get("obj"), {})
                safe = c.get("toughness", 0) > biggest
                keys = exclusive_keys(d, o)
                if (o.get("required") or safe) and not any(k in taken for k in keys):
                    picked.append(o["index"])
                    taken.update(keys)
            return picked
        if kind == "blockers":
            picked, groups, used = [], set(), set()
            for o in d["options"]:
                blocker = cards.get(o.get("obj"), {})
                attacker = cards.get(o.get("attacker"), {})
                g = o.get("group")
                if (blocker.get("toughness", 0) > attacker.get("power", 99)
                        and o.get("attacker") not in used and not (g and g in groups)):
                    picked.append(o["index"])
                    used.add(o.get("attacker"))
                    if g:
                        groups.add(g)
            return picked
        if kind == "target":
            theirs = [o for o in d["options"] if o["player"] != me]
            pool = theirs if len(theirs) >= d["min"] else d["options"]
            return legal_random(d, self.rng, pool)
        if kind == "trigger_optional":
            yes = by_kind(d, "yes")
            return [yes[0]["index"]] if yes else minimal(d)
        return legal_random(d, self.rng)


# --------------------------------------------------------------------------
# The loop: poll, decide, submit, repeat until the match is over.
# --------------------------------------------------------------------------

def wait_for_match(client, seconds=10):
    """A fresh table's match goes live a moment after POST /api/games returns."""
    deadline = time.monotonic() + seconds
    while client.match_state() is None:
        if time.monotonic() > deadline:
            sys.exit("match %s/%d never appeared" % (client.table, client.match))
        time.sleep(0.02)


def play(client, agent, verbose=False):
    wait_for_match(client)
    decisions, refused, idle, stats = 0, 0, 0, {}
    rtt = []
    sleep = 0.002
    answered = -1  # seq of the last decision we answered
    while True:
        d = client.pending()
        # Right after a 204 the server can still show the decision just
        # answered for a moment; never answer the same seq twice.
        if d is not None and d["seq"] <= answered:
            d = None
        if d is None:
            idle += 1
            if idle % 10 == 0:
                row = client.match_state()
                if row and row["state"] != "live":
                    return row, decisions, refused, stats, rtt
            time.sleep(sleep)
            sleep = min(sleep * 2, 0.05)
            continue
        idle, sleep = 0, 0.002
        view = client.view() if agent.needs_view else None
        accepted = False
        for choices in agent.candidates(d, view):
            t0 = time.perf_counter()
            err = client.submit(d, choices)
            rtt.append(time.perf_counter() - t0)
            if err is None:
                accepted = True
                break
            refused += 1
            if "pending decision seq" in err.get("message", ""):
                break  # the decision moved on; poll again
        if not accepted:
            continue  # nothing was admitted: read the decision again
        answered = d["seq"]
        decisions += 1
        stats[d["kind"]] = stats.get(d["kind"], 0) + 1
        if verbose:
            shown = "pay plan" if isinstance(choices, dict) else choices
            print("seq %-5d %-16s -> %s" % (d["seq"], d["kind"], shown))


def main():
    ap = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    ap.add_argument("--server", default="http://127.0.0.1:8080")
    ap.add_argument("--policy", choices=("random", "simple"), default="simple")
    ap.add_argument("--seed", type=int, default=1)
    ap.add_argument("--format", default="constructed")
    ap.add_argument("--bot-policy", default="", help="hosted opponent (GET /api/bot-policies)")
    ap.add_argument("--deck", default="", help="our deck id (GET /api/decks)")
    ap.add_argument("--bot-deck", default="")
    ap.add_argument("--table", help="join an existing seat instead of creating a game")
    ap.add_argument("--match", type=int, default=1)
    ap.add_argument("--seat", type=int, default=0)
    ap.add_argument("--token")
    ap.add_argument("-v", "--verbose", action="store_true")
    a = ap.parse_args()

    if a.table:
        client = SeatClient(a.server, a.table, a.match, a.seat, a.token)
    else:
        g = create_game(a.server, a.format, a.bot_policy, a.deck, a.bot_deck)
        print("created table %s match %d, seat %d vs %s" %
              (g["table"], g["match"], g["seat"], g["bot_policy"]))
        client = SeatClient(a.server, g["table"], g["match"], g["seat"], g["token"])
    agent = (RandomAgent if a.policy == "random" else SimpleAgent)(a.seed)

    t0 = time.perf_counter()
    row, n, refused, stats, rtt = play(client, agent, a.verbose)
    wall = time.perf_counter() - t0
    rtt.sort()
    print("decisions answered: %d (%d answers refused and retried)" % (n, refused))
    print("by kind: " + ", ".join("%s=%d" % kv for kv in sorted(stats.items())))
    if rtt:
        print("intent POST round trip: median %.2f ms, p95 %.2f ms" %
              (1000 * rtt[len(rtt) // 2], 1000 * rtt[int(len(rtt) * 0.95)]))
    seats = row["seats"]
    winner = row.get("winner")
    who = "draw" if winner is None else "%s (seat %d, %s)" % (
        "us" if winner == client.seat else "opponent", winner, seats[winner]["deck"])
    print("match %s after %d turns in %.1f s: winner %s" % (row["state"], row["turns"], wall, who))


if __name__ == "__main__":
    main()
