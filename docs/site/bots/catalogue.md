# Each bot in detail

What each registered bot does and how its fair-or-clairvoyant verdict was reached, with the code that decides it. The one-screen
table is [Bot comparison](compare.md). To select or connect a bot, see [Using and plugging in bots](using.md).

## The registered bots { #the-six-registered-bots }

`bots/all` links seven policies, so every binary that imports it has them. [`cmd/gorged`](https://github.com/adams-shaun/gorge/blob/main/cmd/gorged/main.go)
links [more](#the-policies-gorged-adds).

| Bot | Verdict | Engines | Notes |
|---|---|---|---|
| `bot` | <span class="gg-badge gg-badge--fair">fair</span> | gorge, dirac | Rules, a combat simulation, Forge's mulligan. 29.7 µs a decision ([measured](../performance/bots.md)) |
| `forge-heuristic` | <span class="gg-badge gg-badge--fair">fair</span> | gorge, dirac | `bot` plus rules re-expressed from Forge's AI |
| `forge-heuristic-hold` | <span class="gg-badge gg-badge--fair">fair</span> | gorge, dirac | `forge-heuristic` that keeps instants back |
| `lethal-pressure` | <span class="gg-badge gg-badge--fair">fair</span> | gorge, dirac | The older heuristic with an attack-into-lethal rule |
| `sb-tactical` | <span class="gg-badge gg-badge--fair">fair</span> | gorge, dirac | SpellBench's scored tactical heuristic; bot-only, reads decklists |
| `az-redeal` | <span class="gg-badge gg-badge--fair">fair</span> | gorge only | AlphaZero-style search, 100 simulations, no network; bot-only, reads decklists; 106 ms a searched decision (2-seat) |
| `az-redeal-maxn` | <span class="gg-badge gg-badge--fair">fair</span> | gorge only | `az-redeal` with max^n opponents, for Commander pods; bot-only, reads decklists |

All seven are fair about hidden cards. The three search-or-planner bots that read every seat's decklist (`sb-tactical`, `az-redeal`,
`az-redeal-maxn`) play only against other bots. Each entry's `bots.Info` ([`bots/registry.go`](https://github.com/adams-shaun/gorge/blob/main/bots/registry.go))
carries its label, tier, formats, seat limit, description and cost.

### What each does

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
| Removal timing | hold a one-shot destroy, damage or bounce spell until the best opposing creature is a real threat, it is lethal to a face, or it could never be cast later |
| Targeting | prefer a land the opponent's colours depend on and their prominent colour; skip a strictly worse copy of another of their creatures |
| Forge AI hints | `PlayMain1` earns a first-main bonus; a `RemoveDeck` card is never chosen |
| Deck-shape posture | an aggro deck releases its last held-back blocker; the simulation weighs life, and the mana reserve scales, by shape |
| Attachments | aim an Aura by `AttachAILogic`, rank own creatures as carriers, hold a boon Aura while nothing can carry it |
| Timing | counter only the top foreign spell; hold draw and token spells for Forge's window |
| Combat | gang blocks, a gang-aware predicted reply, Double Strike in blocked combat |

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
freshly redealt world built from its own observations; `bot` answers every other decision. It is `Env` like `sb-tactical`.
A hosted search seat has a 2-second budget per decision (`hostedDecisionDeadlineMS` in `server/host/bot_policy.go`); a search
that runs out plays `bot`'s answer.

**`az-redeal-maxn`** is `az-redeal` with the opponents' decision points in its tree, each opponent maximising its own value
(max^n) instead of all ganging up on the searcher (`azredeal.PolicyMaxN`). It is meant for Commander pods; at two seats it plays
as `az-redeal`.

## The policies `gorged` adds

[`cmd/gorged`](https://github.com/adams-shaun/gorge/blob/main/cmd/gorged/main.go) links further experimental policies beside
`bots/all`. They stay out of `bots/all` so `server/host`'s registry-wide gates keep the built-in set's cost; `bots/hosttest` runs
their host-level gates instead.

- **`cast-profile`**: `bot` playing from a cast profile. The embedded default is intent-identical to `bot`; the entry exists to
  serve a fitted profile beside it.
- **`sb-first`, `sb-uniform`, `sb-heuristic`**: SpellBench's three builtin policies behind one hosted adapter
  (`bots/sbbuiltin`). Each answers from its seat's view; at priority it uses the honest root, when the host built one, as its payment planner.
- **`sb-search-lite-atk`**: the sb-search search ([`bots/spellbench/sbsearch`](https://github.com/adams-shaun/gorge/blob/main/bots/spellbench/sbsearch), hosted by `bots/sbsearch`).
  At priority it prices `sb-tactical`'s pick and its next best plays over four worlds redealt from its own observations, rolled two
  turns forward with `sb-tactical` on both sides, and plays an alternative only when it beats the pick by the margin. It searches an
  attack declaration the same way and answers everything else as `sb-tactical`.
- **`search`** ([`bots/search`](https://github.com/adams-shaun/gorge/blob/main/bots/search) hosting [`bots/searchseat`](https://github.com/adams-shaun/gorge/blob/main/bots/searchseat)):
  the search teacher. It replans the decisions it covers over sampled worlds rebuilt from its own observations and plays `bot` elsewhere.
- **`az-closed-maxn`** registers only with `gorged -pod-bot-decks DIR` (`server/gorged/podbot.go`): a closed-list max^n search for
  Commander pods of up to four seats. Its opponents' cards are dealt from a belief of drafted decks, never their real lists, so it
  declares `ClosedLists` and may sit against a person.

## Fair or clairvoyant

A **fair** bot uses only what its seat may see. A **clairvoyant** bot reads hidden information: the opponent's hand, the
library order, or a clone of the real engine. **No registered bot is clairvoyant.** Where the code decides each verdict:

- **The four heuristics and `cast-profile` are fair.** `heuristic.Bot.Decide` takes the seat's redacted `view.View`;
  `DecideBoard` takes a `botpolicy.Board` of public facts (`heuristic.BoardFromView`). They register without `Env`.
- **`sb-tactical` is fair on hidden cards and reads decklists** (`Env: true, Search: false`). Its planner root is redealt from the
  seat's own feed (`searchseat.HonestRoot`), but the redeal draws from every seat's real decklist (`bots.ReadsDecklists`), so
  `-bot-policies` refuses it (`server/gorged/botpolicies.go`).
- **`az-redeal` and `az-redeal-maxn` are fair on hidden cards, determinised, with no belief model, and read decklists**
  (`Env: true, Search: true`). `Hosted` sets `azmcts.WorldRedeal`.
- **The three SpellBench builtins are fair on hidden cards and read decklists** (`Env: true`). They answer from `env.View` and take
  the honest root only as a payment planner.
- **`sb-search-lite-atk` and `search` are fair on hidden cards, determinised, and read decklists** (`Env: true, Search: true`).
  sb-search deals its worlds from the seat's own feed; the teacher's hosted config is not clairvoyant (`TestHostedSearchConfigIsHonest`
  in `bots/search`).

Two further points.

- **The clairvoyant world exists, for benches only.** `azmcts.WorldClairvoyant` clones the real engine, and
  `searchseat.Options.Clairvoyant` searches a clone of the actual engine. They are ceilings for measuring what hidden information
  is worth. Hosted `az-redeal` leaves `SeatConfig.Source` nil, which `NewSeat` refuses for a clairvoyant world, and a seat whose
  honest root is refused plays its fallback, never a clairvoyant one.
- **A belief model exists.** `azmcts.WorldRedealClosed` closes the opponents' lists: their unknown cards are dealt from a list drawn
  from a belief corpus (`azredeal.ClosedFactory`, `ClosedMaxNFactory`). Such a seat declares `ClosedLists`, and the host checks the
  declaration at match start (`server/host/closedlist.go`). gorge ships no trained network and no belief corpus. A binary links a policy
  that needs a network with `gorged.RegisterNetBot`; `cmd/gorged` registers none. Net runtime details are on
  [Search and nets](../engines/search-and-nets.md#the-net-runtime).

## Cost and engines

Measured costs are on [Bot speed](../performance/bots.md) (2026-10-31). The registry's `Cost` field is declared, not measured, for
every bot but `az-redeal`'s 106 ms row (see each `Cost.Note` in `bots/*/`). On dirac, `bot`, `lethal-pressure`, `forge-heuristic`,
`forge-heuristic-hold` and, at a bot-only table, `sb-tactical` take a seat. Searching policies are refused because dirac deals no search
roots (`Caps.SearchRoots` in `engine/game/gorgegame`, checked in `server/host/table.go`; [dirac server](../guide/dirac.md#what-it-refuses)).
dirac's `hbot` is a port of `bot`, and `sbt` runs gorge's `sb-tactical` through `dirac/oracle` (`dirac/cmd/dplay`).

## Not registered

`bots/searchseat` (a determinized rollout seat, which `search` hosts), `bots/searchprobe` (the sampler, redealer and belief) and the
learned-policy runtime in `bots/policynet` are building blocks. `bots/explain` is the per-decision note a match record carries;
`bots/sanity` is the guard `-bot-common-sense` wraps around a policy.
