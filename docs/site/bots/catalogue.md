# Each bot in detail

The one-screen table is on [Bots](index.md). This page says what each registered bot does, then how its
fair-or-clairvoyant verdict was reached, with the code that decides it. The six below are `bots/all`'s, in every binary that links it;
[the policies `gorged` adds](#the-policies-gorged-adds) follow. To select or connect a bot, see
[Using and plugging in bots](using.md).

## The six registered bots

<div class="grid cards" markdown>

- **`bot`** <span class="gg-badge gg-badge--fair">fair</span>

    The default. Rules, a combat simulation, Forge's mulligan.

    <small>gorge, dirac. Browser, HTTP, SpellBench agent. 29.7 µs a decision.</small>

- **`forge-heuristic`** <span class="gg-badge gg-badge--fair">fair</span>

    `bot` plus rules re-expressed from Forge's AI.

    <small>gorge, dirac. Browser, HTTP, SpellBench agent.</small>

- **`forge-heuristic-hold`** <span class="gg-badge gg-badge--fair">fair</span>

    `forge-heuristic` that keeps instants back.

    <small>gorge, dirac. Browser, HTTP, SpellBench agent.</small>

- **`lethal-pressure`** <span class="gg-badge gg-badge--fair">fair</span>

    The older heuristic with an attack-into-lethal rule.

    <small>gorge, dirac. Browser, HTTP, SpellBench agent.</small>

- **`sb-tactical`** <span class="gg-badge gg-badge--fair">fair</span>

    SpellBench's scored tactical heuristic.

    <small>gorge, dirac, bot-only. Reads decklists. Not for people.</small>

- **`az-redeal`** <span class="gg-badge gg-badge--fair">fair</span>

    AlphaZero-style search, 100 simulations, no network.

    <small>gorge only, bot-only. Reads decklists. About 106 ms a searched decision.</small>

</div>

All six are fair about hidden cards. The two search bots read every seat's decklist, so they play only against other bots. The side-by-side table is on [Bot comparison](compare.md).

### What each does

Six policies register themselves in the bot registry
([`bots/registry.go`](https://github.com/adams-shaun/gorge/blob/main/bots/registry.go)) from the
packages [`bots/all`](https://github.com/adams-shaun/gorge/blob/main/bots/all/all.go) links, so every
binary that imports `bots/all` has them. Each entry's `bots.Info` carries its label, tier, formats, seat
limit, description and cost. [Bot speed](../performance/bots.md) measures them.

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

## The policies `gorged` adds

`cmd/gorged`, the server and the demo's binary, links further experimental policies beside `bots/all`
([`cmd/gorged/main.go`](https://github.com/adams-shaun/gorge/blob/main/cmd/gorged/main.go)). They stay out of `bots/all` so
`server/host`'s registry-wide gates keep the built-in set's cost; `bots/hosttest` runs their host-level gates instead.

- **`cast-profile`** is `bot` playing from a cast profile. The embedded default profile is intent-identical to `bot`, so the entry
  exists to serve a fitted profile beside it.
- **`sb-first`, `sb-uniform`, `sb-heuristic`** are SpellBench's three builtin policies behind one hosted adapter. Each answers from
  its seat's view; at priority it uses the honest root, when the host built one, as its payment planner.
- **`sb-search-lite-atk`** is sb-search ([`bots/spellbench/sbsearch`](https://github.com/adams-shaun/gorge/blob/main/bots/spellbench/sbsearch)):
  at priority it prices `sb-tactical`'s pick and its next best plays over four worlds redealt from its own observations, rolled two
  turns forward with `sb-tactical` on both sides, and plays an alternative only when it beats the pick by the margin. It searches an
  attack declaration the same way and answers everything else as plain `sb-tactical`.
- **`search`** is the search teacher ([`bots/searchseat`](https://github.com/adams-shaun/gorge/blob/main/bots/searchseat)): it
  replans the decisions it covers over sampled worlds rebuilt from its own observations and plays `bot` everywhere else.

## Fair or clairvoyant

A **fair** bot uses only what its seat may see. A **clairvoyant** bot reads hidden information: the opponent's hand, the
library order, or a clone of the real engine. **No registered bot is clairvoyant.** The two search bots are fair about hidden
cards but read every seat's decklist, so they never seat against a person. Verdicts, with where the code decides them:

- **The four heuristics are fair.** `Decide` takes the seat's redacted `view.View`, and `DecideBoard` takes a `Board` of public
  facts (`bots/heuristic/bot.go:616`, `:627`, `BoardFromView` at `:642`). They register without `Env` (`bots/bot/bot.go:12`).
- **`sb-tactical` is fair on hidden cards and reads decklists.** `Env: true, Search: false` (`bots/sbtactical/sbtactical.go:50`).
  Its planner root is redealt from the seat's own feed (`bots/searchseat/honestroot.go:30-53`), but the redeal draws from every seat's
  real decklist (`bots/registry.go:205`), so `-bot-policies` refuses it (`server/gorged/botpolicies.go:53`).
- **`az-redeal` is fair on hidden cards, determinised, with no belief model, and reads decklists.** `Env: true, Search: true`
  (`bots/azredeal/azredeal.go:52`); `Hosted` sets `WorldRedeal` (`azredeal.go:66-70`, `bots/azmcts/redeal.go:19-21`).
- **`cast-profile` is fair.** It is a `heuristic.Bot` registered without `Env` (`bots/castprofile/castprofile.go:15`).
- **The three SpellBench builtins are fair on hidden cards and read decklists.** `Env: true` (`bots/sbbuiltin/sbbuiltin.go:40`);
  they answer from `env.View` and take the honest root only as a payment planner (`sbbuiltin.go:75-81`).
- **`sb-search-lite-atk` and `search` are fair on hidden cards, determinised, and read decklists.** `Env: true, Search: true`
  (`bots/sbsearch/sbsearch.go:60`, `bots/search/search.go:44`). sb-search deals its worlds from the seat's own feed
  (`bots/spellbench/sbsearch/sbsearch.go:709`); the teacher's hosted config is not clairvoyant (`searchseat.Defaults`,
  pinned by `TestHostedSearchConfigIsHonest`).

Two further points.

- **The clairvoyant world exists, for benches only.** `azmcts.WorldClairvoyant` clones the real engine
  (`bots/azmcts/redeal.go:14-18`), and `searchseat.Options.Clairvoyant` searches a clone of the actual engine
  (`bots/searchseat/searchseat.go:149-153`). They are ceilings for measuring what hidden information is worth. The hosted
  `az-redeal` leaves `SeatConfig.Source` nil, which `NewSeat` refuses for a clairvoyant world (`bots/azredeal/azredeal.go:16-18`,
  `:64-65`), and a seat whose honest root is refused plays its fallback, never a clairvoyant one
  (`bots/searchseat/honestroot.go:49-52`).
- **A belief model exists.** `azmcts.WorldRedealClosed` closes the opponents' lists: their unknown
  cards are dealt from a list drawn from a belief corpus (`bots/azmcts/redeal.go:22-28`;
  `bots/azredeal/azredeal.go:100-105`, `ClosedFactory`). Such a seat declares `ClosedLists` and may sit against a person; the host
  checks the declaration at match start (`server/host/closedlist.go:23-38`). gorge ships no trained network and no belief corpus. A
  binary links a policy that needs a network with `gorged.RegisterNetBot` (`server/gorged/netbots.go:76`). Net runtime details are on
  [Search and nets](../engines/search-and-nets.md#the-net-runtime).

## Cost and engines

Costs are on [Bot speed](../performance/bots.md), measured 2026-10-31. The registry's own `Cost` field is a declared number, not a
measurement, for every bot but `az-redeal` (`bots/bot/bot.go:14`, `bots/sbtactical/sbtactical.go:45-50`,
`bots/azredeal/azredeal.go:47-52`). On dirac, `bot`, `lethal-pressure`, `forge-heuristic`, `forge-heuristic-hold` and, at a bot-only
table, `sb-tactical` take a seat. Searching policies are refused because dirac deals no search roots
(`server/host/table.go:340`, `engine/game/gorgegame/gorgegame.go:130`; [dirac server](../guide/dirac.md#what-it-refuses)). dirac's
`hbot` is a port of `bot` and `sbt` runs gorge's `sb-tactical` through `dirac/oracle` (`dirac/cmd/dplay/game.go:20-21`).

## Not registered

`bots/searchseat` (a determinized rollout seat, which `search` hosts), `bots/searchprobe` (the sampler, redealer and belief) and the
learned-policy runtime in `bots/policynet` are building blocks, not registered policies. `bots/explain` is the per-decision note a match record
carries, and `bots/sanity` is the guard `-bot-common-sense` wraps around a policy.

