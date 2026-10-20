# Bot player training and adoption guidelines

*Moved from the repository README on 2026-10-16, then trimmed the same day to what gorge ships. Measurements are as they were recorded.*

gorge ships the engine-side interfaces, the hosted bots (`bot`,
`lethal-pressure`, `sb-tactical`, `az-redeal`) and their inference code; the
research bots, the training tools and the benchmark harness live in a private
companion repository.

This section is for anyone building, searching with, training or shipping a
bot policy. It covers:

- the interfaces the engine offers a bot;
- what has been tried and measured so far;
- the gate a policy must pass before it plays real users.

## The decision contract

Everything a bot does is answering a `decision.Decision`:

- The engine lists **every** legal `Option`. The bot replies with a
  `decision.Intent` naming option indices.
- `(*Decision).Validate` rejects anything the engine did not offer, so a bot
  can never make an illegal move. It can only choose badly.
- `decision.Kinds` is a closed set: priority, target, attackers, blockers,
  mulligan, modes, trigger order/optional, choose, commander zone,
  replacement, arrange, starting player. A policy must answer all of them,
  usually by delegating the kinds it does not specialise in to the default
  bot.

A bot implements `seat.Seat`:

```go
Decide(ctx context.Context, v view.View, d decision.Decision) (decision.Intent, error)
```

It receives the seat's redacted `view.View`, never the engine. There are two
opt-in extensions:

- `heuristic.BoardSeat` (`bots/heuristic`) takes the cheaper `botpolicy.Board` instead of a projected
  view.
- `seat.PaymentPlanConsumer` asks for the engine's mana-payment plans.

External processes can play a seat in two ways:

- over HTTP, through `server/host/httpapi`: `…/pending`, `…/intent`, `…/view`,
  `…/events`, SSE `/api/stream`;
- **(manabrew)** as a ManaBrew protocol client, over SSE + POST
  (`server/host/manabrewhttp`, `server/manabrew`). Opt-in and off by default;
  `gorged -manabrew` mounts it (`-manabrew-addr` for a separate listener),
  and it requires a seat gate (`-humans` or `-vsbot`).

## Engine capabilities for search and reinforcement learning

| Capability | API | Notes |
|---|---|---|
| Step environment | `rules.New(Config)`, `Pending()`, `Submit(Intent)`, `Advance()` | Deterministic: same `Config.Seed` plus same intents gives the same events. |
| Legal actions | `Decision.Options`; `decision.PotentialAction` | The action space *is* the option list. Mana taps are options too, unless auto-pay is on. `PotentialAction` lists the plays that would become castable after floating mana. |
| Intent-level mana | bot auto-pay (`EnableAutoPayMana`); `rules.(*Engine).PotentialPaymentPlans` plus `bots/spellbench/payexec` | Plan "cast X / activate Y" and lower the payment witness to taps afterwards, so taps never enter a search tree. Naive manual-tap bots lose about 190–230 Elo. |
| Fork / simulate | `(*Engine).Clone()`, `CloneInto(*Spare)` / `Release()` | Clone at an intent boundary only. Recycle spares on hot paths. |
| Chance-honest worlds | `rules.NewHypothetical`, `(*Engine).CloneHypothetical(seed)`, `SubmitHypothetical`, `AdvanceHypothetical` | A search clone gets a fresh chance stream, so it cannot peek at the real game's future shuffles and draws. |
| Hidden-information sampling | `bots/searchprobe`: `NewCollector` / `Capture`, `Sample`, `Redealer`, known-card tracking | Determinization. `Sample` never sees the real engine, log, seed or hidden hash. `azmcts.NewRedeal` wraps it for search. |
| Observation | `view.Project` / `ProjectFor` (`Seat` / `Public` / `Omniscient`); `policynet.EncodeState` / `EncodeOption` | Hidden zones are counts. Omniscient never exposes library order. |
| Terminal / reward | `View.Over`, `View.Winner`; `searchprobe.LeafValue` (heuristic leaf); `policynet` value head | No shaped reward is built in. Win/loss is the ground truth. |
| Replay and audit | `events.Log` (sha256 chain, `Head()`), `replay.Replay` / `ReplayTo` | Any game a policy plays can be re-run and checked event for event. |
| Game runner | `testkit/bench`: `PlayGame`, `RunPairs` | Watchdogs, livelock recovery, no clock. |
| Throughput | measured | About 17.5 ms per bot-vs-bot game on one core (`archived 2026-09-06-gorge-learned-policy-research.md (docs/design/ARCHIVE.md)`). About 20–23 duel games/s/core after the hotspot work (`archived 2026-09-17-hotspot-resume.md (docs/design/ARCHIVE.md)`). |

## Engine speed

Measured with `cmd/enginebench` on the rows of draft-zero's
`docs/015-rules-engine-comparison.md` §1 (method in §6): random play is
turns/s, the heuristic bot is games/s, and the rest are per-operation costs
and per-thread search rates. *subset corpus* is an `-tags enginebench_subset`
build that opens only the workload decks' cards rather than the whole registry
— the measuring vehicle for the pointer-free corpus work; every other column
opens the shared imaged registry.

| docs/015 row | a4af596 (docs/015 pin) | old main (e0496f062) | today's main | today, subset corpus | today vs old main |
|---|---|---|---|---|---|
| Random play FDN A, turns/s | 933 | 807 | 3,956 | 3,922 | 4.9x |
| Random play FDN B, turns/s | 1,722 | 1,266 | 3,653 | 3,867 | 2.9x |
| Random play Burn, turns/s | 3,093 | 2,389 | 6,224 | 6,754 | 2.6x |
| Built-in bot FDN A / B, games/s | 39.6 / 68.8 | 38.4 / 50.5 | 148.6 / 150.1 | 148.8 / 163.3 | 3.9x / 3.0x |
| Auto-pay bot FDN A / B, games/s | 31.5 / 45.0 | 30.9 / 32.7 | 83.0 / 92.2 | 104.8 / 101.3 | 2.7x / 2.8x |
| Copy a mid-game state, GC excluded | 14.3 µs | 12.0 µs | 10.7 µs | 7.8 µs | 1.1x |
| Same copies with GC paid | 50.3 µs | 51.3 µs | 53.9 µs | 24.4 µs | 0.95x |
| One search step (copy + submit) | 59.9 µs | 61.4 µs | 31.7 µs | 30.0 µs | 1.9x |
| Pass-priority step only | 39.8 µs | 42.1 µs | 25.9 µs | 26.9 µs | 1.6x |
| Old rollout sampler, rollouts/s | 40.0 | 28.3 | 94.3 | 103.3 | 3.3x |
| azmcts + honest redeal, 100 sims, sims/s | n/a | 336 | 1,099 | 1,294 | 3.3x |
| azmcts + honest redeal, 1,000 sims, sims/s | n/a | 203 | 954 | 1,012 | 4.7x |

## Search

- **PIMC seat.** `bots/searchseat` and `bots/searchprobe`. The method is
  ensemble determinization:
  - sample K hidden-information worlds;
  - roll every candidate out with the default policy;
  - pick the best mean.

  Candidate 0 is always the bot's own answer and wins ties. The L10 seat
  measured **53.7% [52.2, 55.2] vs a 50.4% control (+3.3pp)** on the held-out
  gate. It costs **664 ms mean / 2.2 s p95 per asked decision**, and its
  coverage falls to 4.8% after turn 13
  (`archived 2026-09-24-training-approaches-summary.md (docs/design/ARCHIVE.md)`). It
  is not a hosted policy in gorge.
- **AlphaZero-style PUCT.** `bots/azmcts`, hosted as `az-redeal`
  (`bots/azredeal`):
  - The search is single-perspective: tree nodes are only the seat's own
    searched decisions, and `botpolicy.Decide` plays everything else.
  - There are no rollouts. A leaf is scored by a value function on the
    *redacted* view.
  - The environment seam is `azmcts.Env` (`Root`, `Play(Key)`, `Leaf`). The
    entry point is `azmcts.Search(root, worldSource, net, opts)`.
  - Defaults: 100 sims, c_puct 1.5, first-play urgency 0.1. Exploration noise
    (Dirichlet α 0.3, ε 0.25) applies only during generation.
  - Clairvoyant worlds are refused unless the driving command opts in, and
    no hosted path does; gorge ships no clairvoyant world source.
  - Measured with honest redeal worlds at generation 0 (uniform prior,
    heuristic leaf): az25 **64.7% [63.2, 66.2] vs `bot`, +14.7pp, 45 ms mean
    per searched decision**, and az100 +20.5pp at about 250 ms
    (`docs/design/2026-09-28-spellbench-agent-design.md` §12).
  - Caveat: `bot` is both the opponent and the search's environment model in
    those runs, which flatters every search arm.

## Learned policies

`bots/policynet` is a pure-Go learned policy:

- hashed FNV features, an MLP scorer and an optional value head;
- hand-derived gradients, no autodiff;
- binary `.gpol` checkpoints that refuse to load on an encoder-hash mismatch.

`heuristic.NewPolicyNetBot` (`bots/heuristic`) plays a checkpoint. Any kind
the net does not answer goes to the default bot. The trainer and the label
generators around it are not part of gorge.

What has been measured (all against `bot`; details in
`archived 2026-09-24-training-approaches-summary.md (docs/design/ARCHIVE.md)`):

| Approach | Result |
|---|---|
| AR7 heuristic (lethal pressure) | +1.65pp held-out. **Promoted.** |
| Auto-pay mana for the hosted bot | +3.17pp constructed, +3.69pp Commander. **Promoted** (66aa546dd). |
| SPSA cast profile | 50.92% [49.38, 52.47]. Failed the +3pp gate. |
| PIMC search seat (L10) | +3.3pp held-out, slow. Opt-in. |
| Distilled net / ExIt / PPO / VDWM | 44.9–51.5%. Reproduces the bot, no better. |
| Value head | Log loss 0.244 vs 0.460 base rate, but no win-rate gain on its own. |

The durable lesson: **decision-time search beats the bot, and compressing
search into a cheap policy has so far only reproduced the bot.** The
direction that follows: the network *serves search*, a value head first and
then an entity encoder, trained on search visit-count targets.

This is staged: clairvoyant AZ loop, then honest sampled worlds, with kill
criteria at each stage (`docs/design/2026-09-27-alphazero-mcts-design.md`,
`archived 2026-09-28-spellbench-policy-networks.md (docs/design/ARCHIVE.md)`).

## Evaluating a policy

`testkit/bench` is gorge's game runner: `PlayGame` plays one seeded game
between any `seat.Seat` implementations, and `RunPairs` runs a deck-pair
matrix with seats traded.

- Game *i* is seeded `base + i`, and seats trade policies every game.
- Always run `bot` against `bot` on the same seeds as the control.

Sample size decides what a result can show:

| Games | Smallest effect detectable |
|---:|---|
| 95 | about +100 Elo |
| 379 | about +50 Elo |
| 1,792 | about +23 Elo (3.3pp) |

A 200-game run is about ±7pp: a smoke test, not a verdict
(`archived 2026-09-28-spellbench-theory-and-references.md (docs/design/ARCHIVE.md)` §7).

## Adoption ladder

A policy moves up one rung at a time. Each rung has a gate.

1. **Bench-only.** A `seat.Seat` you drive through `testkit/bench`. No gate
   beyond the rules below. Research modes (clairvoyant, oracle, learned
   policies) stay here.
2. **Hosted, opt-in.** Register it in the `bots` registry (`bots.Register`
   from an `init`, `bots/registry.go`) and link its package from `bots/all`,
   so that `POST /api/games` can select it with `bot_policy` (today: `bot`,
   `lethal-pressure`, `sb-tactical`, `az-redeal`). Requirements:
   - It must be seat-honest (no clairvoyance, nothing the view does not
     show).
   - It must be deterministic from `(match seed, seat)`.
   - It must fit the hosted latency budget: L10's 664 ms mean / 2.2 s p95 is
     the reference for "slow".
   - A host test must pin it.
   - An operator decision is required.
3. **Default `bot`.** Requirements:
   - It passes the **held-out gate**: seed 1,000,000, 400 games per deck
     pair, seats traded.
   - It gains **≥ +3pp pooled** over the current `bot`.
   - **No pair falls below −5pp.**
   - **Zero new stalls or errors.**
   - The same merge regenerates the `TestHeads` goldens, naming the policy
     change as the cause.
   - Development iterations are gated at +1.96σ against the current best
     before they spend a held-out run.
   - Promotion is the operator's decision
     (`archived 2026-09-19-learned-cast-profile.md (docs/design/ARCHIVE.md)`,
     "Operator decisions").

## Native bots

A bot policy linked into a server declares itself through `bots.Register`. The
entry's `Info` names the policy, its tier, formats, seat bound and — for any
knob the factory reads — its `Options []bots.OptionSpec`. Each spec names a
key, a type (`string`, `int`, `float`, `bool` or `path`), an optional default,
help text and whether the option is required. `Register` rejects an invalid
key, an unknown type, a duplicate key or a default that does not parse.

The factory receives the resolved values in `bots.Options.Config`; `bots.New`
fills every declared default before calling it. Read them with
`Options.String`, `Int`, `Float` and `Bool`, which panic on an undeclared key
(a programming error, not a user error).

`gorged` is the reference host:

- `-bot-opt <policy>.<key>=<value>` sets one value and is repeatable.
- `-bot-config <policy>=<file.json>` reads a flat JSON object; repeatable.
- `GORGED_BOT_<POLICY>_<KEY>` (upper case, `-` becomes `_`) supplies a value
  without a flag.

Precedence is flag, then environment, then config file, then the declared
default. Every value is validated before a listener opens: an unknown policy
or key, a value that does not parse as its declared type, or a missing
required option exits non-zero with the flag and key named. Startup validates
every registered policy, not only the offered set, so a linked policy that
declares a required option must be configurable on every server that links it.
`gorged -help` prints every registered policy's options, and the option values
never appear in `/api/bot-policies`.

## Rules for bot code

- **Determinism.** Seeded `math/rand/v2` only, and no map order in anything
  that reaches a decision or a checkpoint. No `time` in library code: inject a
  `Millis` hook the way `searchseat` and `azmcts` do. Only allowlisted
  commands may read a clock, and only for reporting.
- **Pure Go, in-repo.** No cgo, no dependencies, and no PyTorch or other
  external runtime for anything that ships.
- **Honesty.** A policy or value model that will play must read the redacted
  view only. The `mz-oracle` / `mz-opphand` feature sets and clairvoyant
  worlds are diagnostic ceilings. `policynet.WriteCheckpoint` refuses the
  oracle feature sets. The one exception, `WriteOracleCheckpoint`, exists only
  for a search leaf scored inside an already-sampled world.
- **Artifacts.**
  - Corpora, checkpoints and `GOTMPDIR` live under `/mnt/sata/gorge-training`,
    never `/tmp`.
  - Small JSON weight profiles may be committed.
  - `.gpol` neural checkpoints may not be committed without an operator
    decision.
  - Never commit anything derived from Forge script text.
- **Resources.** Run one heavy job at a time, under `systemd-run --user
  --scope -p MemoryMax=…` with `GOMEMLIMIT` set. This is a shared box.
- **Report what you measured.** Record results in
  `docs/design/`: the command, the seeds, the game count, and the
  CI against a same-seed control.

