# Runtime behavior notes

Load-bearing runtime and host behavior constraints; consult this page when working in the named areas.

## Trigger-relative filter arguments

`ControlledBy <ref>` and `OwnedBy <ref>` recognise exactly `TriggeredTarget`,
`TriggeredDefendingPlayer`, `TriggeredPlayer` and `TriggeredCard`, plus a
`Spawner> <known-inner-ref>` chain in that argument position (resolved
against the same riding TriggerContext); an absent binding fails closed
(also under `!`); the `Targeted*` family stays unknown, as does every
`Spawner>` chain outside that argument position. `effects.TriggerContext` carries event roles separately from the
resolving ability's source, targets and Remembered, survives suspension,
cloning and stack copying, and is rebuilt by replay without new events. Not
implemented: `TargetingPlayer$` (Magus of the Abyss asks the trigger
controller), LKI owner/control snapshots for a referent that changes before
resolution (live owner/controller is read), carrying these bindings into a
registered continuous effect, and the Self/Other target-source normalisation
(Flickerwisp's `Permanent.Other`).

## Cast-provenance filter arguments

The `Card.wasCast*` family is a rules-side split, not a filter predicate:
`castProvenanceAdmits` (`engine/rules/cast_provenance.go`) strips the tokens and reads
the log (latest PutOnStack cast wins; a copy was never cast; a never-cast card
reads false). Hand family: `wasCastFromYourHandByYou`, `wasCastByYou`,
`wasCastFromYourHand`. Origin-zone family: `wasCastFromExile`,
`wasCastFromYourGraveyard` and `...ByYou` (identical here: a cast's origin zone
is always the caster's), `wasCastFromTheirHand`. Bare `wasCastFromGraveyard`
is the effects-side CastFlags predicate (flashback/harmonize/escape) instead.
Wired at the trigger match walks, `Count$ThisTurnCast_<spec>`,
`Count$wasCastFromExile`, the target walks (offer and CR 608.2b recheck) and
the CantBeCast walk (`castOriginAdmitsAtZone` reads the pending cast's
origin). Still open: the tokens are not evaluated by effects' own
ConditionPresent/ConditionDefined evaluator (such a gate runs its sub
unconditionally), there is no `Count$wasCastFromYourGraveyard` head, and the
may-play provenance predicates (`MayPlaySource`/`CastSa`) stay fail-closed.

## Decision-arena lifetime contract

A fresh `rules.Engine` carves its decisions -- the posed `*decision.Decision`
and its `Options` -- out of a bump-allocated arena (engine/rules/decision_arena.go)
to keep them off the GC's plate (~34% of alloc_space on enginebench's play
rows). That storage is reclaimed by generation: a posed decision, its
`Options` slice and everything they reference are **valid only until the next
`Submit` (or `Advance`) on the engine that posed them**. A reader that needs
the decision past that point (the host's decision feed, the view projection,
a bot policy that stores offers, a bench trace) must copy it with
`decision.Decision.Clone()` (or an equivalent deep copy). A live engine keeps
two generations and retires one at each Submit boundary
(engine/rules/decision_arena_live.go), so the arena is bounded across a whole game;
a search simulation's engine uses `SetDecisionArena` and is Release-scoped
instead. The rules, host and view test binaries enable a poison verify mode
(`decisionArenaVerify`) that overwrites a retired generation with a sentinel,
so a reader that retains a decision fails loudly instead of reading a reused
slot.

## Host behaviour notes (embedder observer hooks)

`OnBurst` errors crash the match like a persist failure: the table
halts and the chain does not continue. `OnMatchEnd` errors are discarded
because the outcome is already recorded and an error cannot un-record it, so
an embedder that persists through `OnMatchEnd` must handle its own
persistence failures inside the callback.

## Payment plans for bot seats

A driver (`testkit/bench.PlayGame`, `server/host/match.go`, cmd tools) builds a
decision's mana-payment plans (`Engine.EnsurePaymentActions`) only for a
seat that implements `seat.PaymentPlanConsumer` and answers true. Bots
default to true; a wrapper forwards to what it wraps. Without plans the
auto-pay bot falls back to a manual mana tap, and once mana floats the
engine withholds plans for the rest of the window, so a search seat that
asks its bot first (`azmcts.Seat.DecideSearch`) never searched those
windows. The opt-outs are bots whose policy reads only the manual option
list (manual-mana `heuristic.Bot`, `heuristic.PolicyNetBot`, the search-teacher seat,
generation-0 `az-redeal`), each named with its reason in
`bots/all/payment_default_test.go`, which fails the build on any other
registered bot answering false. As a backstop, an AutoPayment az seat whose
bot still answers a mana tap searches the full payment vocabulary with no
bot candidate (`Root.NoBot`) and counts `Stats.BotTapRerooted`
(reported as `tap-rerooted` by a bench run). Human seats are unchanged: the host builds
their plans under `TableConfig.AutoMana` and the client only picks.

The same build also leaves an engine-side cast census on the decision
(`Decision.PotentialCasts`, `PotentialCastsKnown`, beside
`PotentialUnpayableCasts`): every object the offer builder's PotentialMana
walk offers a cast of. The bot's tap gate (`botpolicy.tapTarget`) never aims
a manual tap at a card absent from it or proved unpayable, which was two of
every three manual tap runs the auto-pay bot made. A
seat that never builds plans has no census, so the manual-mana bot and every
golden game are unchanged. The manual taps that remain are planner gaps the
V1 witness withholds by design ({X}, hybrid, convoke/delve/improvise,
target-dependent and optional costs, restricted or last-resort sources).

## Seat-privacy boundary

A seat credential names **both its table and seat**. Every current
seat-authorised HTTP route (`view`, `events`, `pending`, `intent`, `undo`)
goes through `server/host/httpapi`'s `claimForTable`; a mismatched or legacy unbound
claim is 403 and must never reach registry state. `cmd/gorged` mints startup
and vs-bot tokens with that pair, so after deployment old tokens stop working;
the web client returns a rejected stale join to the lobby, where it obtains a
fresh game claim. Stream session ids are random 128-bit values issued only in
`hello`; `host.Session.serial`, not the public id, preserves fan-out order.
