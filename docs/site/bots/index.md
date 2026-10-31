# Bots

This section is for anyone who wants a program, not a person, to play gorge: a Go developer
adding a policy, a Python or Java author wiring a strategy to a server, or a benchmark harness.
This page surveys the bots gorge ships and the ways to plug one in. The pages after it walk
through each with working code.

## The one contract every bot answers

Whatever the transport, a bot does one thing: the engine poses a **decision** listing every legal
option, and the bot answers with an **intent** naming option indices
([glossary](../contributing/glossary.md)). The engine validates the answer against what it
offered, so a bot can choose badly but can't make an illegal move. The decision kinds are a closed
set of thirteen (`decision.Kinds` in
[`engine/decision/decision.go`](https://github.com/adams-shaun/gorge/blob/main/engine/decision/decision.go)):
`priority`, `target`, `attackers`, `blockers`, `mulligan`, `modes`, `trigger_order`,
`trigger_optional`, `commander_zone`, `choose`, `replacement`, `arrange` and `starting_player`.
Conceding is an option on a priority decision, not a kind.
[Playing a seat over HTTP](http.md#answering-each-decision-kind) explains how to answer each one.

A bot sees one seat's redacted **view** of the game: its own hand, the public board and counts for
hidden zones, never the opponent's hand or the library order.

## The bots gorge ships

Six policies register themselves in the bot registry
([`bots/registry.go`](https://github.com/adams-shaun/gorge/blob/main/bots/registry.go)) from the
packages [`bots/all`](https://github.com/adams-shaun/gorge/blob/main/bots/all/all.go) links, so every
binary that imports `bots/all` has them. Each entry's `bots.Info` carries its label, tier, formats, seat
limit, description and cost. [Bot speed](../performance/bots.md) measures them.

| Name | Formats, seats | Offered by `gorged` | Package |
|---|---|---|---|
| `bot` | constructed and Commander, any | yes | [`bots/bot`](https://github.com/adams-shaun/gorge/blob/main/bots/bot/bot.go) |
| `forge-heuristic` | constructed and Commander, any | yes | [`bots/forgeheuristic`](https://github.com/adams-shaun/gorge/blob/main/bots/forgeheuristic/forgeheuristic.go) |
| `forge-heuristic-hold` | constructed and Commander, any | yes | [`bots/forgeheuristic`](https://github.com/adams-shaun/gorge/blob/main/bots/forgeheuristic/forgeheuristic.go) |
| `lethal-pressure` | constructed and Commander, any | yes; `gorged`'s default `-bot-policy` | [`bots/lethalpressure`](https://github.com/adams-shaun/gorge/blob/main/bots/lethalpressure/lethalpressure.go) |
| `sb-tactical` | constructed, 2 | no: reads every seat's decklist | [`bots/sbtactical`](https://github.com/adams-shaun/gorge/blob/main/bots/sbtactical/sbtactical.go) |
| `az-redeal` | constructed, 2 | no: reads every seat's decklist | [`bots/azredeal`](https://github.com/adams-shaun/gorge/blob/main/bots/azredeal/azredeal.go) |

**`bot`** is the production heuristic: hand-written rules
([`bots/botpolicy`](https://github.com/adams-shaun/gorge/blob/main/bots/botpolicy), driven by
[`bots/heuristic`](https://github.com/adams-shaun/gorge/blob/main/bots/heuristic)) that score each offered
option. On a table with automatic mana payment, the default for hosted bots, it casts through the engine's
payment plans and picks attacks and blocks with a two-ply combat simulation; on a manual-tap table it is the
plain heuristic. It answers the London mulligan with a re-expression of Forge's keep and bottom rule
(`botpolicy.MulliganForge`).

**`forge-heuristic`** is `bot`, built by the same constructor (`bot.BaseWithSim`), with every opt-in rule
family re-expressed from Forge's AI switched on. Each family is a clean-room rule in gorge's own vocabulary,
not translated Forge code:

| Family | Rules armed |
|---|---|
| Removal timing | hold a one-shot destroy, damage or bounce spell until the best opposing creature is a real threat, the spell is lethal to a face, or it could never be cast later |
| Targeting | prefer a land the opponent's colours depend on and the opponent's prominent colour; skip a target that is a strictly worse copy of another of their creatures |
| Forge AI hints | a card's `PlayMain1` hint earns a first-main-phase bonus; a `RemoveDeck` card is never chosen |
| Deck-shape posture | an aggro deck releases its last held-back blocker, the combat simulation weighs life by deck shape, and the mana reserve scales by shape |
| Attachments | aim an Aura by its `AttachAILogic` hint, rank its own creatures as carriers, hold a boon Aura while nothing can carry it |
| Timing | counter only the top foreign spell; hold draw and token spells for Forge's window |
| Combat, inside the simulation | gang blocks, a gang-aware predicted reply, Double Strike in blocked combat |

The rules live in `bots/botpolicy` (`TargetRules`, `RemovalRules`, `TimingRules`, `AIHints`, `AttachRules`,
`PostureRules`, `CombatRules`), and `bots/forgeheuristic.New` shows how to arm any subset on a policy of your own.

**`forge-heuristic-hold`** is `forge-heuristic` with its one option, `hold-instant`, switched on: it keeps instants
back outside the respond, opponent post-combat and combat-trick windows.

**`lethal-pressure`** is the older configuration: the plain heuristic plus an attack rule (a creature whose unblocked
power reaches the defender's life attacks even into a cheaper blocker). It has no combat simulation on any table, and it
keeps the heuristic's original mulligan, a one-in-three coin whatever the hand.

**`sb-tactical`** is a scored tactical heuristic ported from the SpellBench project. It values creatures, timing, the
damage race and removal over what its seat can see, reads printed card facts from the card registry, and at priority
prices its plays through a payment planner built on an *honest root*: a copy of the game with every hidden card redealt
from what the seat has seen. That redeal draws from each seat's real decklist, which is why the policy is marked `Env`
and is never seated against a person.

**`az-redeal`** is an AlphaZero-style tree search
([`bots/azmcts`](https://github.com/adams-shaun/gorge/blob/main/bots/azmcts)) with no network: a uniform prior and a
heuristic leaf value. At each priority, attackers, blockers and target decision it runs 100 simulations, each over a
freshly redealt world built from its own observations, and `bot` answers every other decision. It is `Env` like
`sb-tactical`. A hosted search seat has a 2-second budget per decision (`hostedDecisionDeadlineMS`); a search that runs
out plays `bot`'s answer.

### How to select one

| Where | How |
|---|---|
| `gorged`, play-vs-bot | `POST /api/games` with `"bot_policy": "<name>"`; omit it to get the server's `-bot-policy`. `-bot-policies a,b,…` limits what the server offers |
| `gorged`, bot-only tables | `-bot-policy <name>` seats that policy in every bot seat of the startup tables |
| `mtgsim` | `-a <name>` on seat 0, `-b <name>` on seat 1; every other seat plays the plain heuristic |
| A Go program | `bots.New(name, bots.Options{Seed, AutoPayMana, Deps: bots.Deps{Cards: reg}})` after a blank import of `bots/all`; [`testkit/bench`](https://github.com/adams-shaun/gorge/blob/main/testkit/bench/bench.go) drives it |
| SpellBench v2 | `spellbench-gorge-agent -policy <name>` serves it as a v2 agent ([SpellBench v2 agents](spellbench.md)) |

`-bot-common-sense` (default off) wraps `bot`, `forge-heuristic` and `sb-tactical` in the `bots/sanity` guard, which drops
hostile removal that could only hit the bot's own permanents.

`sb-tactical` and `az-redeal` need a driver that supplies their honest roots and the card registry.
`testkit/bench.PlayGame` with `Hooks{EnvSeats: true, HonestRoot: true}` does. `mtgsim` doesn't: `-a sb-tactical` fails with
`Deps.Cards is nil`, and `-a az-redeal` runs but never searches.

As the opponent for an HTTP bot, or the baseline for your own policy, use `bot`, with a `bot`-against-`bot` control on the
same seeds. Start `gorged` with `-bot-policy bot` to make it the default.

### Other policies at runtime

- **A SpellBench v2 agent** registered with `-sb-agent name=/path/to/agent` becomes a policy like any other.
- **Network-backed search bots.** A binary links a search bot that needs a trained network through `gorged.RegisterNetBot`
  ([`server/gorged/netbots.go`](https://github.com/adams-shaun/gorge/blob/main/server/gorged/netbots.go)), which gives it its
  `-<policy>-*` flags. gorge ships the inference code but no trained networks, so a plain `gorged` has no such flags.
- **Your own Go policy**, linked into a binary of your own ([Writing a native Go bot](native-go.md)).

`bots/searchseat` (a determinized rollout seat), `bots/searchprobe` and the learned-policy runtime in `bots/policynet` are
building blocks for benches, not registered policies.

## The ways to plug a bot in

<figure markdown>
<svg viewBox="0 0 780 400" width="100%" role="img" aria-label="Inside one gorged process the rules engine and the host match loop drive three kinds of seat: native Go seats called in-process, a HumanSeat that parks a decision for the HTTP API, and a SpellBench seat adapter. Across the process boundary, HTTP clients in any language answer the HumanSeat over HTTP and JSON with a bearer token, and a SpellBench v2 agent process answers the adapter over stdin and stdout NDJSON.">
  <defs>
    <marker id="bt-arr" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse"><path d="M0,0 L10,5 L0,10 z" class="dg-arrow"/></marker>
    <marker id="bt-arr-acc" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse"><path d="M0,0 L10,5 L0,10 z" class="dg-arrow-accent"/></marker>
  </defs>
  <rect x="16" y="30" width="474" height="340" rx="8" class="dg-box-muted"/>
  <text x="30" y="52" class="dg-text-bold">one process: gorged, mtgsim or testkit/bench</text>
  <rect x="36" y="74" width="150" height="62" rx="6" class="dg-box"/>
  <text x="111" y="100" text-anchor="middle" class="dg-text-bold">rules engine</text>
  <text x="111" y="120" text-anchor="middle" class="dg-text-muted">Decision → Intent</text>
  <rect x="36" y="176" width="150" height="170" rx="6" class="dg-box"/>
  <text x="111" y="250" text-anchor="middle" class="dg-text-bold">match loop</text>
  <text x="111" y="270" text-anchor="middle" class="dg-text-muted">server/host or bench</text>
  <line x1="111" y1="138" x2="111" y2="174" class="dg-line" marker-start="url(#bt-arr)" marker-end="url(#bt-arr)"/>
  <rect x="236" y="74" width="234" height="72" rx="6" class="dg-box-accent"/>
  <text x="353" y="98" text-anchor="middle" class="dg-text-bold">① native Go seats</text>
  <text x="353" y="117" text-anchor="middle" class="dg-text-muted">bot, forge-heuristic, az-redeal, yours</text>
  <text x="353" y="134" text-anchor="middle" class="dg-text-mono">Decide(ctx, View, Decision)</text>
  <rect x="236" y="176" width="234" height="72" rx="6" class="dg-box"/>
  <text x="353" y="200" text-anchor="middle" class="dg-text-bold">② HumanSeat + httpapi</text>
  <text x="353" y="219" text-anchor="middle" class="dg-text-muted">parks the decision until an intent</text>
  <text x="353" y="236" text-anchor="middle" class="dg-text-mono">/pending /intent /view</text>
  <rect x="236" y="276" width="234" height="72" rx="6" class="dg-box"/>
  <text x="353" y="300" text-anchor="middle" class="dg-text-bold">③ SpellBench seat adapter</text>
  <text x="353" y="319" text-anchor="middle" class="dg-text-muted">server/spellbench/seat (-sb-agent)</text>
  <text x="353" y="336" text-anchor="middle" class="dg-text-mono">hello · game_start · choose</text>
  <path d="M188,200 C212,200 212,110 234,110" class="dg-line" marker-end="url(#bt-arr)"/>
  <line x1="188" y1="212" x2="234" y2="212" class="dg-line" marker-end="url(#bt-arr)"/>
  <path d="M188,224 C212,224 212,312 234,312" class="dg-line" marker-end="url(#bt-arr)"/>
  <line x1="500" y1="20" x2="500" y2="380" class="dg-line-dashed"/>
  <text x="506" y="36" class="dg-text-muted">process boundary</text>
  <rect x="580" y="176" width="190" height="72" rx="6" class="dg-box"/>
  <text x="675" y="202" text-anchor="middle" class="dg-text-bold">HTTP client</text>
  <text x="675" y="222" text-anchor="middle" class="dg-text-muted">Python, Java, a browser…</text>
  <text x="675" y="239" text-anchor="middle" class="dg-text-muted">any machine that can reach it</text>
  <rect x="580" y="276" width="190" height="72" rx="6" class="dg-box"/>
  <text x="675" y="302" text-anchor="middle" class="dg-text-bold">v2 agent process</text>
  <text x="675" y="322" text-anchor="middle" class="dg-text-muted">spawned per match, any language</text>
  <text x="675" y="339" text-anchor="middle" class="dg-text-muted">same machine only</text>
  <line x1="472" y1="212" x2="578" y2="212" class="dg-line-accent" marker-start="url(#bt-arr-acc)" marker-end="url(#bt-arr-acc)"/>
  <text x="508" y="204" class="dg-text-muted">HTTP + JSON</text>
  <text x="508" y="230" class="dg-text-muted">bearer token</text>
  <line x1="472" y1="312" x2="578" y2="312" class="dg-line" marker-start="url(#bt-arr)" marker-end="url(#bt-arr)"/>
  <text x="508" y="304" class="dg-text-muted">stdin/stdout</text>
  <text x="508" y="330" class="dg-text-muted">NDJSON</text>
  <text x="580" y="102" class="dg-text-muted">① has nothing outside:</text>
  <text x="580" y="120" class="dg-text-muted">the seat is a Go value</text>
  <text x="580" y="138" class="dg-text-muted">in the same address space</text>
</svg>
<figcaption>The integration surfaces. Native seats are called directly; an HTTP client answers a parked HumanSeat across the network; a SpellBench v2 agent is a child process on the same machine.</figcaption>
</figure>

1. **Native Go** ([guide](native-go.md)). Implement `seat.Seat`, register it with `bots.Register`, link it into `gorged` or a
   bench. It is the fastest path and the only one that can search: a Go seat can clone the engine or ask the host for an honest
   redealt world.
2. **HTTP seat** ([guide](http.md), [examples](examples.md)). A server started with `-vsbot` or `-humans` parks a seat's decision
   behind `GET .../pending`, and any program that speaks HTTP and JSON answers it with `POST .../intent`. The Python and Java examples
   use this.
3. **SpellBench v2** ([guide](spellbench.md)). gorge speaks the SpellBench benchmark protocol in both directions over a child process's
   stdin and stdout: an external agent can take a gorge seat (`-sb-agent`), and any gorge bot can be served as a v2 agent. With `-sbv2`
   a server also offers the same decisions over HTTP.
4. **gorge-agent/1** ([reference](../api/gorge-agent.md)). A batch protocol for drafters, builders and players in the
   [draft tools](../guide/draft.md#compose-a-workflow-with-labdemo).

| | Native Go seat | HTTP seat | SpellBench v2 agent seat |
|---|---|---|---|
| **Language** | Go | anything with HTTP and JSON | anything that reads and writes lines on stdio |
| **Transport** | a function call | HTTP/1.1 and JSON; SSE for push | stdin and stdout, one JSON object per line |
| **Where the bot runs** | inside the host process | anywhere that can reach the server | a child process the host spawns, one per match |
| **Hidden information** | the seat is handed a redacted view; the code is trusted, since it runs in the server | enforced by the server: the bearer token names the one seat it may read and act for | the host sends an engine-neutral observation for the agent's seat only |
| **Can it search or clone the engine?** | yes: `Engine.Clone`, `CloneHypothetical`, or an honest root via `bots.EnvSeat` | no engine access; bring your own simulator | no, unless the agent embeds one |
| **Determinism** | a pure function of the seed | the server records every intent, so a finished game replays; the bot's own randomness is yours to seed | the host passes an `agent_seed` |
| **Many games at once** | goroutines (`testkit/bench.RunPairs`) | one token per seat, up to `-max-on-demand-tables` | one process per seat per match |
| **On a timeout** | hosted search seats fall back to their heuristic after 2 s | none: `gorged` waits, and the seat's caretaker bot answers only when the table shuts down | forfeit: the seat concedes at its next priority, with the default bot answering until then |

Per-decision costs: about 30 µs for a two-seat game step in process (`examples/bots/go/cmd/duel`, on
[Writing a native Go bot](native-go.md#play-it-against-the-default-bot)), a few tenths of a millisecond over HTTP on loopback
([Latency](http.md#latency)), and about the same over stdio ([SpellBench latency](spellbench.md#latency)).
