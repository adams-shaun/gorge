# The hosted az policies: az-c3, az-c4 and az-c5

Status: hosted, opt-in, experimental. None is promoted or a default, and none
has been measured against `bot` on the held-out gate. This page gathers the
three policies' design and measurements, which were first written up while the
policies lived in the private companion module (2026-10-09, 10-14 and 10-15).

## What they are

Each policy is the az search seat (`bots/azredeal`, `bots/azmcts`) in its
honest redeal world, with a small trained network in place of the heuristic
prior and leaf. The search spec is the one the self-play loop trained and
evaluated:

```
az:sims=100:autopay:oppnodes:oppfull:morekinds:topk=6         (az-c3)
az:sims=100:autopay:oppnodes:oppfull:morekinds:closedlist:topk=6 (az-c4, az-c5)
```

with the London mulligan by land count (`botpolicy.MulliganLands`). The network
is an `mlp` of width 64, about 0.6M parameters (601,346 for C4 and C5), most of
them a 16,384-row hashed table, evaluated in process from a `.dzgw` file by
`bots/gonet`.

| Policy | Package | Opponent's list | May play a human |
|---|---|---|---|
| az-c3 | `bots/azc3` | read (open) | no: bot-only demo table, needs `-allow-decklist-bots` |
| az-c4 | `bots/azc4` | guessed from a belief (closed) | yes |
| az-c5 | `bots/azc5` | guessed from a belief (closed) | yes |

`azc3.Hosted` is the one implementation: a hosted az policy over one network,
parameterised by its `bots.Info` and whether it is closed-list only. az-c4 and
az-c5 are two more instances of it. `azc3.Config()` is the seat config
(`azredeal.Hosted` plus the keys above) and `azc3.ClosedConfig(b)` closes it
under a belief; `TestConfigMatchesTheMeasuredSpec` and
`TestConfigIsTheClosedMeasuredSpec` pin every field.

**Closed lists.** az-c4 and az-c5 know their own list and never the
opponent's. At every searched decision the opponent's unseen hand, library and
sideboard are re-identified from a belief: a list drawn uniformly from the
28,366 train-split 17lands FDN decks whose colour identity covers the colours of
the opponent's seen cards, minus what the seat has seen (`searchprobe.Belief`,
`azmcts.WorldRedealClosed`).

**Trained on one distribution.** Every network saw only 17lands FDN limited
decks. The server keeps each policy to them (the deck pool below); what it does
on a constructed deck is unmeasured.

## The networks

C3 came from the open-list self-play loop. C4 came from the closed-list loop with
the engine's paid-cost mana census on. C5 continued C4's loop for four
generations, warm-started from C4, on a build carrying the payment-plan fixes
(bots consume the engine's payment plans by default, the manual tap gate, the
flashback and discard plans).

The networks are training output, about 2.4 MB each, and are not in the
repository. They are produced by the private companion module's training loop
and handed to a server by path (`-az-c3-net` and friends, or the deploy's
`AZ_C3_NET` and friends in `deploy/demo.env`). `GORGE_DZGW` points the tests
that read a real network at one; without it they use the synthetic network
`bots/gonet/gonettest` writes.

## What was measured

Every row is 600 paired games (300 deck pairs, seats swapped) at 100
simulations on the FDN **eval** decks (never trained on), two mulligans with the
land-count rule, seed 78. Intervals are Wilson 95%. Strength is bot against bot
throughout.

az-c3, open lists:

| C3 against | wins - losses | score | 95% interval | ms per searched decision |
|---|---|---|---|---|
| sb-tactical | 378 - 222 | 63.0% | [59.1, 66.8] | 314 |
| the same search, no net | 335 - 265 | 55.8% | [51.8, 59.8] | 202 |
| C2 | 307 - 293 | 51.2% | [47.2, 55.1] | 344 |

az-c4, closed lists, census on:

| C4 against | wins - losses | score | 95% interval | ms per searched decision |
|---|---|---|---|---|
| sb-tactical | 407 - 193 | 67.8% | [64.0, 71.4] | 134 |
| the same closed search, no net | 365 - 235 | 60.8% | [56.9, 64.7] | 110 |
| C3 of the same loop | 322 - 278 | 53.7% | [49.7, 57.6] | 134 |

az-c5, closed lists, census on, payment-plan build:

| Net | vs az-c4 | vs sb-tactical |
|---|---|---|
| az-c4 on this build | n/a | 69.0% (414 - 186) [65.2, 72.6] |
| C5 | 52.8% (317 - 283) [48.8, 56.8] | 72.3% (434 - 166) [68.6, 75.8] |

C5's edge over C4 is small (the head-to-head interval includes 50%); most of
C4's gain on the newer build came from the payment fixes. A 250-game gauntlet of
az-c3 against sb-tactical on the hosted deck pool (each deck a seat-swapped
mirror) scored 64.0% [57.9, 69.7], agreeing with the loop's 63.0%.

**Hosted latency** (the `Cost` each entry quotes) is a smoke, not a load test:
the wall time from a passing human's accepted intent to that human's next
decision, polled every 3 ms. az-c3: 4 games, 32 waits that included bot search,
mean 185 ms, p95 259. az-c4: 5 games, one at a time on an idle server, the 73
waits of 20 ms or more, mean 91 ms, p95 136. az-c5: 5 games with an az-c5 demo
table playing on the same 4-CPU server, the 266 waits of 20 ms or more, mean
97 ms, p95 173.

## Why the closed-list policies may play humans

An opponent's decklist is private, so the host refuses every `bots.Info.Env`
policy at a table with a human unless the policy closes the lists:

- `bots.Info.ClosedLists` declares an Env policy whose every seat closes its
  opponents' lists, and `Info.ReadsDecklists()` is `Env && !ClosedLists`. Host
  table validation, gorged's offered policies, the vs-bot create flow and draft
  events all gate on that one predicate.
- The host does not trust the declaration (`server/host/closedlist.go`). At a
  table with a human it checks the seats it built before the first decision:
  every non-human `bots.EnvSeat` must be a `searchseat.ClosedListSeat` with a
  belief, or the match stops with nothing handed out. At every decision an open
  setup is cut to the actor's own list, and no root is built at a decision posed
  inside a resolution, whose checkpoint holds the real hidden cards (2 of 271
  searched decisions in the privacy test play the unsearched answer instead).
- The belief comes from the server's `-az-c4-belief` (`-az-c5-belief`) flag.
  Nothing a client sends selects or changes it.

A leak audit of everything the host hands the seat found and fixed: the
opponent's sideboard objects keeping their real cards, clones sharing the real
deck manifests, a stale offer-walk class cache, the feed collector keeping a
pointer to the live engine, and the host's reused `Board` carrying its row
cache. What remains is listed as residual: the engine's compiled-text cache
(a speed cache keyed by card, never an answer), three boolean pool gates, and
object-id adjacency, which nothing in the search reads.

`bots/hosttest` holds the proofs, run through the real host:
`TestAZC4NeverSeesTheHumansHiddenCards` and `TestAZC5NeverSeesTheHumansHiddenCards`
play whole games against a human seat and walk, by reflection, everything handed
to the bot at every decision, failing on any secret card not yet revealed;
`TestClosedListGuardRefusesAnOpenSeatAtAHumanTable` and
`TestDecklistGateAtHumanTables` pin the gate.

## Hosting

```
gorged -vsbot \
  -az-c3-net c3.dzgw -az-c3-decks DECKS -allow-decklist-bots \
  -az-c4-net c4.dzgw -az-c4-decks DECKS -az-c4-belief BELIEF [-az-c4-belief-pool pool.tsv] \
  -az-c5-net c5.dzgw -az-c5-decks DECKS -az-c5-belief BELIEF [-az-c5-belief-pool pool.tsv]
```

Each policy registers only when its `-<policy>-net` is given: a server that
never names a network cannot offer the policy, and a bad file stops the server
at startup. Each needs `-bot-auto-mana` (on by default), because it pays its
mana through the engine's payment plans as it did in training.

- **Deck pool.** `-<policy>-decks DIR` names the decks the policy may play, and
  they are reserved for it both ways: a game against it deals both seats from
  the pool, and a game against any other policy never deals a reserved deck.
  `GET /api/decks?policy=az-c4` lists the pool. Policies may share a directory;
  a shared deck is reserved for each and listed once. The pool used so far is
  the top three decks by rank in each colour pair with at least 300 training
  decks, 36 in all, from the train split.
- **Belief.** `-<policy>-belief DIR` is the deck corpus the policy guesses
  from; `-<policy>-belief-pool` names a `pool.tsv` whose train split is read.
- **Demo table.** Each enabled policy adds one two-seat bot-only table of itself
  (`-<policy>-demo-table=false` omits it).

The decks and the belief corpus are 17lands data (CC BY 4.0, `NOTICE`) and are
not in the repository. `make deploy-demo` passes all of this from the
`AZ_C3_*`, `AZ_C4_*` and `AZ_C5_*` variables (`deploy/demo.env.example`).

## What was not measured

- Against `bot` on the held-out gate, and against a human who plays.
- On any constructed or Commander deck.
- Hosted latency under load.
- The strength cost of the posed-resolution fallback.
