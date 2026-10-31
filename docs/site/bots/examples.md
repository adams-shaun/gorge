# Python and Java examples

This page walks through two complete bots that play gorge over the network,
one in Python and one in Java, for authors who want to wire their own
strategy to the engine in a language other than Go. Both are single files
with no dependencies beyond the language's standard library, and both are
structured so that you replace one function, `choose`, with your own
strategy. CI plays one complete game with each against a throwaway `gorged`
([`scripts/examples-smoke.sh`](https://github.com/adams-shaun/gorge/blob/main/scripts/examples-smoke.sh)).
The transcripts below are from 2026-10-31.

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
one agent process across as many games as it likes, and is the surface the
browser client plays through, so anything the agent does can be watched in a
browser. An agent that already speaks v2 should use `-sb-agent` instead.

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
    $ cd examples/bots/python
    $ python3 gorge_agent.py --server http://127.0.0.1:8092 --policy simple \
        --deck mono-green-stompy --bot-deck mono-red-goblins -v
    created table g1 match 1, seat 0 vs bot
    seq 18    starting_player  -> [0]
    seq 20    mulligan         -> [0]
    ...
    seq 1345  priority         -> [3]
    seq 1355  priority         -> [3]
    decisions answered: 126 (0 answers refused and retried)
    by kind: attackers=5, mulligan=1, priority=118, starting_player=1, target=1
    intent POST round trip: median 0.30 ms, p95 0.54 ms
    match finished after 11 turns in 0.5 s: winner us (seat 0, mono-green-stompy)
    ```

=== "Java"

    ```console
    $ cd examples/bots/java
    $ java Agent.java --server http://127.0.0.1:8092 --policy simple \
        --deck mono-green-stompy --bot-deck mono-red-goblins -v
    created table g3 match 1, seat 0 vs bot
    seq 18    starting_player  -> [0]
    seq 20    mulligan         -> [0]
    ...
    seq 2052  priority         -> [10]
    decisions answered: 193 (0 answers refused and retried)
    by kind: attackers=6, blockers=4, choose=1, mulligan=1, priority=178, starting_player=1, target=2
    intent POST round trip: median 0.49 ms, p95 1.31 ms
    match finished after 16 turns in 0.8 s: winner opponent (seat 1, mono-red-goblins)
    ```

    `java Agent.java` compiles the file in memory; the example uses Java 21 features, tested on OpenJDK 21.0.11.
    The Java default server is port 8093; pass `--server` for another.

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
    `--policy` choices in `main()`.

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

    Then select it in `main()` where `policy` is chosen.

The decision kinds and what an answer to each means are on
[Playing a seat over HTTP](http.md#answering-each-decision-kind).

Neither strategy is meant to be strong; they are scaffolding. For fair comparisons between strategies, with seats traded
and enough games, see [Bot-vs-bot games](../guide/bot-games.md). Over whole games the agents spend a few milliseconds per
decision, mostly in polling back-off; [Latency](http.md#latency) breaks that down.
