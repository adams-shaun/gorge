# Python and Java examples

Two complete bots that play gorge over the network, one in Python and one in Java. Each is a single file using only the standard
library; replace one function, `choose`, with your own strategy. The `examples` job of `.github/workflows/ci.yml` plays one game with each
against a throwaway `gorged` ([`scripts/examples-smoke.sh`](https://github.com/adams-shaun/gorge/blob/main/scripts/examples-smoke.sh)).
The transcripts below are from 2026-10-31.

Source:
[`examples/bots/python/gorge_agent.py`](https://github.com/adams-shaun/gorge/blob/main/examples/bots/python/gorge_agent.py)
(Python 3, `urllib` and `json` only) and
[`examples/bots/java/Agent.java`](https://github.com/adams-shaun/gorge/blob/main/examples/bots/java/Agent.java)
(Java 21, `java.net.http`, an embedded JSON reader and writer; runs
as `java Agent.java`, no build step, no Maven).

## Why HTTP

Both agents use the [HTTP seat](http.md), not [SpellBench v2](spellbench.md). v2 runs over a child process's stdin and stdout, so the server
must start the agent itself, on its own machine, once per match. The HTTP seat works from any machine, keeps one agent across many
games, and is the surface the browser plays through, so you can watch it. An agent that already speaks v2 should use `-sb-agent`.

## Run them

Start a server with the play-vs-bot flow armed (see
[Start a server](http.md#start-a-server-that-has-a-seat-for-you)):

```console
$ ./gorged -addr 127.0.0.1:8080 -dir /tmp/gorge-bots -feedback /tmp/gorge-bots/feedback \
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
    $ python3 gorge_agent.py --server http://127.0.0.1:8080 --policy simple \
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
    $ java Agent.java --server http://127.0.0.1:8080 --policy simple \
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
    Both agents default to `http://127.0.0.1:8080`, `gorged`'s default address; pass `--server` for another.

## How they are built

Both files have the same four parts, in the same order.

1. **Seat client.** `pending()` returns the decision, or nothing on `409`; `view()` the redacted view; `submit()` posts an intent and
   returns the error body on a refusal; `match_state()` reads state, winner and turns. Every call sends `Authorization: Bearer`.
2. **Answer helpers.** `legal_random` builds a random answer that respects `min` and `max`, includes every `required` option, and takes
   at most one option per exclusivity key (the option's `group`, and on `attackers` its creature). `minimal` is the fallback almost
   always accepted (pass at priority, else the first `min` options). `pay` turns a payment action into `{"payment": {"action_id", "plan"}}`,
   echoing the first plan verbatim.
3. **Strategies.** `random` is uniform over legal answers, but never concedes and passes priority half the time. `simple` keeps every
   hand, plays first, plays a land whenever it can, then the most expensive spell it can cast (through a plan when offered); attacks
   only when its toughness exceeds the power of every untapped enemy creature; blocks only when the blocker survives; targets the
   opponent's things; says yes to optional triggers. Everything else is `legal_random`.
4. **Loop.** Poll `/pending`, back off from 2 ms to 50 ms while idle, check the match state every tenth empty poll. On a decision, read
   the view if the strategy wants it, then submit in order: the strategy's answer, eight random legal answers, the minimal answer, until
   one is accepted. A stale-`seq` refusal means the decision moved on, so poll again. It skips a pending decision whose `seq` it has
   already answered.

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

Neither strategy is meant to be strong. For fair comparisons see [Bot-vs-bot games](../guide/bot-games.md); latency is on [Latency](http.md#latency).
