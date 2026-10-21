# Python and Java examples

This page walks through two complete bots that play gorge over the network,
one in Python and one in Java, for authors who want to wire their own
strategy to the engine in a language other than Go. Both are single files
with no dependencies beyond the language's standard library, and both are
structured so that you replace one function, `choose`, with your own
strategy. CI plays one complete game with each against a throwaway `gorged`
([`scripts/examples-smoke.sh`](https://github.com/adams-shaun/gorge/blob/main/scripts/examples-smoke.sh));
the transcripts below were captured on 2026-10-20 (`ad3777023`), and the
smoke script passed again at `513a2a0c3` on 2026-10-21.

Source:
[`examples/bots/python/gorge_agent.py`](https://github.com/adams-shaun/gorge/blob/main/examples/bots/python/gorge_agent.py)
(Python 3, `urllib` and `json` only) and
[`examples/bots/java/Agent.java`](https://github.com/adams-shaun/gorge/blob/main/examples/bots/java/Agent.java)
(Java 21, `java.net.http`, a 130-line embedded JSON reader and writer; runs
as `java Agent.java`, no build step, no Maven).

## Why HTTP

Both agents use the [HTTP seat](http.md), not the [SpellBench v2
protocol](spellbench.md). v2's transport is a child process's stdin and
stdout: the server must start the agent itself, on its own machine, once per
match. The HTTP seat works from any machine that can reach the server, keeps
one agent process across as many games as it likes, and is the same surface
the browser client plays through, so anything the agent does can be watched
in a browser. An agent that already speaks v2 should use `-sb-agent`
instead.

## Run them

Start a server with the play-vs-bot flow armed (see
[Start a server](http.md#start-a-server-that-has-a-seat-for-you)):

```console
$ ./gorged -addr 127.0.0.1:8092 -dir /tmp/gorge-bots -feedback /tmp/gorge-bots/feedback \
    -tables 0 -vsbot -max-on-demand-tables 0 -pace 0 -prewarm=false -bot-policy bot
```

Each run creates a fresh game against the hosted `bot`, plays it to the end
and prints a summary. `-v` prints every answer. `--policy random` or
`--policy simple` picks the strategy; `--deck`, `--bot-deck` and
`--bot-policy` pick the matchup (ids from `GET /api/decks` and
`GET /api/bot-policies`). To play a seat you already hold instead, pass
`--table`, `--match`, `--seat` and `--token`.

=== "Python"

    ```console
    $ python3 gorge_agent.py --server http://127.0.0.1:8092 --policy simple \
        --deck mono-green-stompy --bot-deck mono-red-goblins -v
    created table g209 match 1, seat 0 vs bot
    seq 22    mulligan         -> [0]
    seq 53    priority         -> [0]
    ...
    seq 2878  priority         -> [10]
    seq 2887  priority         -> pay plan
    seq 2897  priority         -> [8]
    seq 2907  priority         -> [8]
    seq 2916  priority         -> [8]
    seq 2924  attackers        -> [0, 1, 2, 3, 4, 5, 6]
    seq 2935  priority         -> [6]
    seq 2945  priority         -> [6]
    decisions answered: 276 (0 answers refused and retried)
    by kind: attackers=9, blockers=2, choose=1, mulligan=1, priority=259, target=4
    intent POST round trip: median 0.14 ms, p95 0.20 ms
    match finished after 24 turns in 0.7 s: winner us (seat 0, mono-green-stompy)

    $ python3 gorge_agent.py --server http://127.0.0.1:8092 --policy random --seed 7
    created table g210 match 1, seat 0 vs bot
    decisions answered: 130 (0 answers refused and retried)
    by kind: choose=3, mulligan=1, priority=126
    intent POST round trip: median 0.12 ms, p95 0.18 ms
    match finished after 13 turns in 0.4 s: winner opponent (seat 1, boros-moxite-burn)
    ```

=== "Java"

    ```console
    $ java Agent.java --server http://127.0.0.1:8093 --policy simple \
        --deck mono-green-stompy --bot-deck mono-red-goblins -v
    created table g1 match 1, seat 0 vs bot
    seq 18    starting_player  -> [0]
    seq 20    mulligan         -> [0]
    ...
    seq 3596  priority         -> [11]
    seq 3605  priority         -> [11]
    seq 3620  priority         -> [11]
    seq 3630  priority         -> [11]
    seq 3634  blockers         -> []
    seq 3642  priority         -> [11]
    decisions answered: 347 (0 answers refused and retried)
    by kind: attackers=14, blockers=4, choose=2, mulligan=1, priority=321, starting_player=1, target=4
    intent POST round trip: median 0.26 ms, p95 0.87 ms
    match finished after 30 turns in 1.1 s: winner opponent (seat 1, mono-red-goblins)

    $ java Agent.java --server http://127.0.0.1:8093 --policy random --seed 7
    created table g2 match 1, seat 0 vs bot
    decisions answered: 223 (0 answers refused and retried)
    by kind: attackers=1, blockers=1, choose=5, mulligan=1, priority=213, starting_player=1, target=1
    intent POST round trip: median 0.28 ms, p95 0.62 ms
    match finished after 18 turns in 0.6 s: winner opponent (seat 1, tron)
    ```

    `java Agent.java` compiles the file in memory (Java 11+ source-launcher;
    tested on OpenJDK 21.0.11). The Java default server is port 8093; pass
    `--server` for another.

## How they are built

Both files have the same four parts, in the same order, so they can be read
side by side.

### 1. The seat client: four calls

`pending()` returns the decision or nothing (on `409`), `view()` the seat's
redacted view, `submit()` posts an intent and returns the error body on a
refusal, `match_state()` reads the match row (state, winner, turns). Every
call sends the token as `Authorization: Bearer`.

=== "Python"

    ```python
    def pending(self):
        """The decision asked of this seat, or None (409: nothing pending)."""
        status, body = self._call("GET", self._match_path("pending?seat=%d" % self.seat))
        if status == 200:
            return body
        if status == 409:
            return None
        raise RuntimeError("pending: HTTP %d %s" % (status, body))

    def submit(self, decision, answer):
        intent = {"seq": decision["seq"], "player": decision["player"]}
        if isinstance(answer, dict):
            intent.update(answer)          # {"payment": {...}}
        else:
            intent["choices"] = answer     # [option indices]
        status, body = self._call("POST", self._match_path("intent"), intent)
        return None if status == 204 else (body or {"code": str(status)})
    ```

=== "Java"

    ```java
    Map<String, Object> pending() throws IOException, InterruptedException {
        Reply r = call("GET", matchPath("pending?seat=" + seat), null);
        if (r.status == 200) return Json.obj(r.body);
        if (r.status == 409) return null;
        throw new IOException("pending: HTTP " + r.status + " " + Json.write(r.body));
    }

    Map<String, Object> submit(Map<String, Object> d, Answer a) throws IOException, InterruptedException {
        Map<String, Object> intent = new LinkedHashMap<>();
        intent.put("seq", d.get("seq"));
        intent.put("player", d.get("player"));
        if (a.payment != null) intent.put("payment", a.payment);
        else intent.put("choices", a.choices);
        Reply r = call("POST", matchPath("intent"), intent);
        if (r.status == 204) return null;
        return r.body == null ? Map.of("code", String.valueOf(r.status)) : Json.obj(r.body);
    }
    ```

    The JSON reader keeps object key order (`LinkedHashMap`) and reads
    integers as `long` (a 64-bit seed as `BigInteger`), so a payment plan is
    echoed back exactly as the server sent it.

### 2. Answer helpers: the wire rules

`legal_random` builds a random answer that respects `min` and `max`,
includes every `required` option, and takes at most one option per
*exclusivity key*: the option's `group`, and on an `attackers` decision its
creature. `minimal` is the fallback that is almost always accepted (pass at
priority, otherwise the first `min` options). `pay` turns a payment action
into a `{"payment": …}` answer.

=== "Python"

    ```python
    def exclusive_keys(d, o):
        keys = []
        if o.get("group"):
            keys.append(("group", o["group"]))
        if d["kind"] == "attackers" and o.get("obj"):
            keys.append(("attacker", o["obj"]))   # one creature, one defender
        return keys

    def pay(action):
        return {"payment": {"action_id": action["id"], "plan": action["plans"][0]}}
    ```

=== "Java"

    ```java
    static List<String> exclusiveKeys(Map<String, Object> d, Map<String, Object> o) {
        List<String> keys = new ArrayList<>();
        if (o.get("group") != null) keys.add("group:" + o.get("group"));
        if ("attackers".equals(d.get("kind")) && o.get("obj") != null) keys.add("attacker:" + o.get("obj"));
        return keys;
    }

    /** Either option indices or a plan-paid cast. */
    record Answer(List<Integer> choices, Map<String, Object> payment) {
        static Answer of(List<Integer> c) { return new Answer(c, null); }
        static Answer pay(Map<String, Object> action) {
            Map<String, Object> p = new LinkedHashMap<>();
            p.put("action_id", action.get("id"));
            p.put("plan", Json.arr(action.get("plans")).get(0)); // echoed verbatim
            return new Answer(null, p);
        }
    }
    ```

### 3. The strategies

Two are included:

- **random**: uniform over legal answers, except that it never concedes and
  passes priority half the time (a uniformly random priority answer keeps
  activating repeatable abilities and never ends its turn).
- **simple**: keeps every hand and plays first; plays a land whenever it
  can, then the most expensive spell it can cast (through a payment plan
  when that is how the spell is offered); attacks with a creature only when
  its toughness exceeds the power of every untapped enemy creature; blocks
  only when the blocker survives; targets the opponent's things when it can;
  says yes to optional triggers. Everything else is `legal_random`.

### 4. The loop

Poll `/pending`, back off from 2 ms to 50 ms while nothing is pending, check
the match state every tenth empty poll, and on a decision: read the view if
the strategy wants it, then submit the candidates in order (the strategy's
answer, eight random legal answers, the minimal answer) until one is
accepted. A stale-`seq` refusal means the decision moved on, so it polls
again. It also remembers the last `seq` it answered and skips a pending
decision that is not newer: the server refuses a second answer to one
decision with a `409` anyway, so this only saves a round trip.

=== "Python"

    ```python
    answered = -1
    while True:
        d = client.pending()
        if d is not None and d["seq"] <= answered:
            d = None
        if d is None:
            ...                      # back off; every 10th poll, check match_state()
            continue
        view = client.view() if agent.needs_view else None
        accepted = False
        for choices in agent.candidates(d, view):
            err = client.submit(d, choices)
            if err is None:
                accepted = True
                break
            if "pending decision seq" in err.get("message", ""):
                break                # the decision moved on; poll again
        if not accepted:
            continue                 # nothing admitted: read the decision again
        answered = d["seq"]
    ```

=== "Java"

    ```java
    long answered = -1;
    while (true) {
        Map<String, Object> d = client.pending();
        if (d != null && Json.num(d.get("seq")) <= answered) d = null;
        if (d == null) { /* back off; every 10th poll, check matchState() */ continue; }
        Map<String, Object> view = policy.needsView() ? client.view() : null;
        List<Answer> tries = new ArrayList<>();
        tries.add(policy.choose(d, view, rng));
        for (int i = 0; i < 8; i++) tries.add(Answer.of(legalRandom(d, rng, null)));
        tries.add(Answer.of(minimal(d)));
        boolean accepted = false;
        for (Answer ans : tries) {
            Map<String, Object> err = client.submit(d, ans);
            if (err == null) { accepted = true; break; }
            if (String.valueOf(err.get("message")).contains("pending decision seq")) break;
        }
        if (!accepted) continue; // nothing admitted: read the decision again
        answered = Json.num(d.get("seq"));
    }
    ```

## Write your own strategy

Override one method. It gets the decision and (if you ask for it) the view,
and returns either a list of option indices or a payment answer. Returning
something illegal is safe: the server refuses it with a `409` and the loop
falls back.

=== "Python"

    ```python
    class BurnFace(ga.Agent):
        """Point every target at the opponent; otherwise play like SimpleAgent."""
        needs_view = True

        def __init__(self, seed=1):
            super().__init__(seed)
            self.simple = ga.SimpleAgent(seed)

        def choose(self, d, view):
            if d["kind"] == "target":
                face = [o for o in d["options"] if o["kind"] == "player" and o["player"] != d["player"]]
                if face:
                    return [face[0]["index"]]
            return self.simple.choose(d, view)
    ```

    Then pass an instance to `play(client, BurnFace())`, or add it to the
    `--policy` choices in `main()`. (Tested 2026-10-20: a Burn deck, 152
    decisions, none refused.)

=== "Java"

    ```java
    /** Point every target at the opponent; otherwise play like SimplePolicy. */
    static final class BurnFace implements Policy {
        final SimplePolicy simple = new SimplePolicy();
        public boolean needsView() { return true; }
        public Answer choose(Map<String, Object> d, Map<String, Object> view, Random rng) {
            if ("target".equals(d.get("kind"))) {
                for (Map<String, Object> o : options(d)) {
                    if ("player".equals(kind(o)) && Json.num(o.get("player")) != Json.num(d.get("player")))
                        return Answer.of(List.of(idx(o)));
                }
            }
            return simple.choose(d, view, rng);
        }
    }
    ```

    Then select it in `main()` where `policy` is chosen. (Tested
    2026-10-20 in a copy of `Agent.java` with a `--policy burn` branch: 164
    decisions, none refused.)

The decision kinds and what an answer to each means are on
[Playing a seat over HTTP](http.md#answering-each-decision-kind).

## How well they play

Neither is meant to be strong; they are scaffolding. Measured on 2026-10-20
against the hosted `bot` playing UW Control, our deck dealt at random, one
game per seed:

| Agent | Games | Wins | Answers refused |
|---|---:|---:|---:|
| Python `simple` | 40 | 20 | 0 |
| Python `random` | 40 | 3 | 2 |
| Java `simple` | 20 | 9 | 0 |
| Java `random` | 20 | 3 | 0 |

```console
$ for i in $(seq 1 40); do python3 gorge_agent.py --policy simple --seed $i --bot-deck uw-control | tail -1; done
```

All 120 games ran to a result. Over whole games the agents spend 1.6 ms
(random, no view) to 3.2 ms (simple, a view per decision) per decision,
mostly in polling back-off; [Latency](http.md#latency) breaks that down.
For fair comparisons between strategies (seats traded, seeded deck pairs,
enough games), see [Bot-vs-bot games](../guide/bot-games.md).
