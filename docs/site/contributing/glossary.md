# Glossary

The words gorge's code, commits, reviews and this site use, for a Go
developer who knows neither Magic engine internals nor Forge. The first
group is about cards and the engine; the [second](#repositories-kernels-and-process)
about the repositories, the second kernel and how changes land.

**Card script.** One text file per card in Forge's corpus,
`.cards/cardsfolder/<letter>/<name>.txt` (token definitions are under
`.cards/tokenscripts/`). Each line is a characteristic (`Name:`, `ManaCost:`,
`Types:`, `PT:`, `Oracle:`) or an ability: `A:` (spell or activated ability),
`T:` (trigger), `S:` (static), `R:` (replacement), `K:` (keyword), `SVar:`.
The scripts are GPL-3.0. They are fetched, never committed. See
[Card pipeline](card-pipeline.md).

**`$` parameter.** An ability line is a `|`-separated list of `Key$ Value`
pairs. Examples: `ValidTgts$ Creature` (what may be targeted), `NumDmg$ 3`,
`AffectedZone$ Hand` (where a static's affected cards are), `EffectZone$
Graveyard` (where the static's *source* must be for it to function; absent
means the battlefield, CR 113.6). In the IR they are the `Params` map on each
ability (`cards.SA`, `cards.Trigger`, `cards.Static`, `cards.Repl`). A
parameter no code reads is an "unread param", which the param census tracks.

**SVar.** A named value on a card (`SVar:<Name>:<body>`), stored in
`cards.Face.SVars`. The body is either another ability (a sub-ability that a
trigger's `Execute$` or an ability's `SubAbility$` names) or a value
expression (`Count$…`) that a parameter refers to by name.

**IR (intermediate representation).** What `engine/cards` compiles a script
into: a `cards.Card` holding one or more `cards.Face`s, each with its
`Abilities`, `Triggers`, `Statics`, `Repls`, `Keywords` and `SVars` (types in
`engine/cards/ir.go`). The whole corpus is cached as
`.cards/ir-<fingerprint>.gob.gz`.

**Face.** One printed face of a card (`cards.Face`). Most cards have one.
Split, adventure, modal double-faced and transforming cards have more
(`ALTERNATE` in the script starts the next one). `Object.Face()` is the face
an object currently shows.

**Intrinsic.** An ability the rules grant rather than the script prints. The
script for a basic Forest has no mana ability; `Face.ApplyIntrinsics`
(`engine/cards/intrinsic.go`) adds "{T}: Add {G}" from the subtype (CR 305.6).

**Primitive.** One named engine capability a card needs. `Face.Primitives()`
(`engine/cards/primitive.go`) lists a face's primitives as prefixed strings,
and a card is *playable* when every one is implemented:

| Head | Means | Implemented by |
|---|---|---|
| `api:<Name>` | an effect, from `SP$`/`AB$`/`DB$ <Name>` (`api:DealDamage`) | `effects.Register` in `engine/effects` |
| `trig:<Mode>` | a trigger mode, from `T:Mode$ <Mode>` (`trig:ChangesZone`) | `engine/rules`, declared via `effects.RegisterNonAPI` |
| `stat:<Mode>` | a static ability mode, from `S:Mode$ <Mode>` (`stat:Continuous`) | `engine/rules`, `RegisterNonAPI` |
| `repl:<Event>` | a replacement effect, from `R:Event$ <Event>` (`repl:Moved`) | `engine/rules`, `RegisterNonAPI` |
| `kw:<Keyword>` | a keyword (`kw:Flying`) | `engine/rules` (`RegisterNonAPI`), or an `engine/cards/kw_*.go` expander that rewrites it into ordinary lines |
| `count:<Head>` | a value expression an SVar reads (`count:Domain` for `Count$Domain`) | the evaluator in `engine/effects/count*.go` plus `modelledValueHeads` |

`effects.Supported()` returns the implemented set. `make report` compares it
with the corpus.

**Registry.** `cards.Registry`: every compiled card and token, looked up by
name (`Registry.Lookup`). Tests get the shared one from
`testutil.CorpusRegistry(t)`. Not to be confused with the *effect registry*
in `engine/effects/registry.go` (primitive name → implementation) or the bot
registry in `bots/`.

**Event.** One entry in the game's log (`events.Event`, kinds in
`engine/events/event.go`): a zone move, a life change, damage, a tap, a
choice made. The only way state changes is `events.Apply`. Engine code
proposes events with `e.emit` (rules) or `Host.Emit` (effects). `events.Kind`
is append-only.

**Chain head.** The event log is a SHA-256 hash chain: each event's hash
covers the previous one. `Log.Head()` (`engine/events/log.go`) is the latest
hash, printed as 16 hex digits. Two runs with the same head did exactly the
same thing. `TestHeads`
pins the heads of the golden acceptance games in
`engine/rules/testdata/heads/<seats>.txt`, so "the head moved" means your
change altered what those games do.

**Decision, intent and decision kinds.** When the engine needs a player's
input it poses a `decision.Decision` listing *every* legal `Option`. The
player answers with a `decision.Intent` naming option indices, and anything
not offered is rejected. The kinds are a closed set
(`engine/decision/decision.go`): `priority`, `target`, `attackers`,
`blockers`, `trigger_order`, `trigger_optional`, `choose`, `mulligan`,
`modes`, `commander_zone`, `replacement`, `arrange`, `starting_player`.
Conceding is an option on a priority decision, not a kind.

**Seat.** Whoever answers a seat's decisions: a bot, a test script, a human
over HTTP. The contract is `seat.Seat` (`engine/seat/seat.go`), whose one
method is `Decide(ctx, view.View, decision.Decision) (decision.Intent, error)`.
A seat never sees the engine.

**View.** One seat's redacted picture of the game (`view.View`, built by
`view.Project` in `engine/view`): hidden zones are counts, other players'
decisions are hidden. Clients and bots read state only through a view.

**Projection.** Building a seat's `view.View` from the game state
(`view.Project`, `view.ProjectFor`, and `view.RedactEvents` for the log).
"Change a projection" means changing what some seat is shown, which lives in
`engine/view/card.go` or `engine/view/stack.go`.

**Replay.** Re-running a match from its `Config` plus recorded intents and
checking the rebuilt event stream event for event (`engine/replay`,
`make sim`'s `-verify`, `cmd/repro`). Because all mutation is in the log,
replay is also how a server resumes after a restart.

**Ratchet.** A test whose expected value is a committed table and which fails
in both directions. A new gap fails, and a listed gap that is now closed
(stale) fails too. Tables only shrink. Examples:
`engine/rules/testdata/known-unsupported/`, `engine/rules/testdata/paramcensus/`,
the oracle audit's `known-divergent/` files. See [Testing](testing.md).

**Census.** A test or one-off measurement that walks the whole corpus (or
every card with a given shape) and counts how many behave a certain way.
"Measure the class, not the card": a fix is checked against every card
sharing the bug's shape, not just the reported one. The param census
(`engine/rules/paramcensus_test.go`) is a committed one.

**Oracle text / oracle scenario.** A card's printed rules text (the script's
`Oracle:` line). An *oracle scenario* is a JSON test, played by gorge's oracle runner, whose
expectations come from the printed text and the CR, not from the script or
the engine. (Separately, *compliance oracles* compare gorge with other
engines, XMage and Forge.) Both live in [gorge-compliance](https://github.com/adams-shaun/gorge-compliance); gorge
keeps the runner and a few residual scenarios under
`engine/rules/testdata/oracle/`.

**CR citation.** A reference to the
[Magic: The Gathering Comprehensive Rules](https://magic.wizards.com/en/rules)
by rule number: "CR 113.6" (where abilities function), "CR 704.5a"
(a state-based action), "CR 613" (the layer system). Code comments and commit
messages cite the rule a line enforces.

**Known approximation.** A place where a registered primitive behaves more
narrowly than the card text, listed in `AGENTS.md`'s *Known approximations*
table. The table only shrinks (`TestKnownApproximationsOnlyShrinks`; see
[Invariants](../architecture/invariants.md#ratchets-only-shrink)).
Deliberate behaviour that only looks like an approximation is an *engine
contract*, recorded in `docs/design/2026-09-22-engine-contracts.md`.

**Forge, `FORGE_REF`, the fork.** [Forge](https://github.com/Card-Forge/forge)
is an open-source Java Magic engine whose card scripts gorge compiles. gorge
reads them from its fork [adams-shaun/forge](https://github.com/adams-shaun/forge)
at the commit pinned by `FORGE_REF` in the `Makefile`.

## Repositories, kernels and process

**gorge-compliance.** The public companion repository that checks gorge card
by card against XMage, Forge and hand-written Oracle-text scenarios, and
holds the certification ratchet. It imports gorge; gorge never imports it.
See [The three repositories](../repos/index.md).

**Private companion.** The maintainers' private companion module: draft
events, the dirty-dirac kernel and the demo server. It imports gorge at a
pinned commit; nothing in gorge needs it.

**`GORGE_PIN`.** A file at the root of each companion repository holding the
full gorge commit SHA it builds against, mirrored by the gorge
pseudo-version in its `go.mod`. Moving it is a pull request in the companion
([Pinning](../repos/index.md#pinning)).

**`develop` / `main`.** Changes land on `develop` by pull request. The
maintainers' pipeline periodically opens a `develop → main` snapshot pull
request and merges it, when its checks are green, with a merge commit. `main` is what this site and
the published coverage are built from ([Branch flow](../repos/index.md#branch-flow)).

**dirty-dirac.** A second, much faster rules kernel, in the private
companion module. Its whole game is one fixed-size value (no pointers, maps
or slices, so a copy is a clone) driven as a `Step`/`Apply` state machine,
and its cards are tables of *kop* instructions, mostly generated ahead of
time from gorge's compiled cards. It began as a two-player microkernel for
two fixed card pools and is growing toward a full engine: `kgen` now lowers
cards from the whole corpus, and play is checked against gorge by a
*lockstep oracle*. Work in progress. See
[the dirty-dirac kernel](../architecture/dirty-dirac/index.md).

**kop.** dirty-dirac's card-effect vocabulary: a closed, append-only set of
ops, and the tables (`kop.Table`, one entry per card) its interpreter runs.
The generator that lowers a card's gorge IR into kop tables is `kgen`.

**Lockstep oracle.** A check that plays the same game in two engines at once,
feeding both the same answers, and at every decision compares their
projected state and the options each offers; the first difference is
reported as a divergence. dirty-dirac is
held to gorge this way. (Distinct from the *compliance oracles*, which
compare gorge with XMage and Forge scenario by scenario.)

**Canonical projection.** The engine-neutral snapshot both sides of a
lockstep run are reduced to before comparison: per object its card, zone,
position, flags, counters and derived power/toughness, then life, the stack,
turn, step and mana. Two engines agree at a decision when their projections
are equal (`==` on fixed arrays) and they offer the same canonical options.
See [Verification](../architecture/dirty-dirac/oracle.md).

**Capacity stall.** A lockstep game dirty-dirac stops on purpose because the
position no longer fits its fixed-size state (more objects, board slots or
stack entries than the arrays hold). It is reported separately from a
divergence: a stall is a known limit, not a rules disagreement. A few stall
reasons mark interactions whose ordering dirty-dirac does not model yet,
rather than capacity.

**kgen.** The generator that lowers gorge's compiled card IR into
dirty-dirac's kop tables, as generated Go card packages. It reads every
parameter of a card or refuses the card. A refused card stays out of every
deck dirty-dirac accepts. A card kgen *accepts* has lowered without a
refusal; that is weaker than a card verified in lockstep. See [Cards](../architecture/dirty-dirac/cards.md).

**SpellBench v2.** An external, engine-neutral benchmark protocol for Magic
agents ([jackmaiorino/spellbench](https://github.com/jackmaiorino/spellbench)).
gorge serves it in both roles: as an environment
(`server/spellbench`, `cmd/spellbench-gorge-env`) and as an agent wrapping any
registered gorge bot (`cmd/spellbench-gorge-agent`). `gorged` and `mtgsim`
can also seat a v2 agent as a bot (`-sb-agent`).

**Seam.** An exported extension point other modules plug into without gorge
importing them: `bots.Register`, `server/gorged.RegisterNetBot`,
`server/gorged.Extensions`, and the exported oracle runner.

**Golden / goldens.** Committed expected outputs a test compares against
byte for byte: the chain heads in `engine/rules/testdata/heads/`, wire
frames, replay fixtures. A golden moves only with a named cause.
