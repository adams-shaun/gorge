# Glossary

Words gorge's code, commits and reviews use. The first group is cards and the engine; the
[second](#repositories-engines-and-process) is repositories, the second engine and process.

**Card script.** One text file per card in Forge's corpus,
`.cards/cardsfolder/<letter>/<name>.txt` (tokens: `.cards/tokenscripts/`). Lines are
characteristics (`Name:`, `Types:`, `PT:`, `Oracle:`) or abilities: `A:` spell or activated,
`T:` trigger, `S:` static, `R:` replacement, `K:` keyword, `SVar:`. GPL-3.0, fetched, never
committed. See [Card pipeline](card-pipeline.md).

**`$` parameter.** A `Key$ Value` pair on an ability line: `ValidTgts$ Creature`, `NumDmg$ 3`,
`AffectedZone$ Hand` (where a static's affected cards are), `EffectZone$ Graveyard` (where its
*source* must be to function; absent means the battlefield, CR 113.6). In the IR they are the
`Params` map on `cards.SA`, `cards.Trigger`, `cards.Static` and `cards.Repl`. One no code reads
is an "unread param", tracked by the param census.

**SVar.** A named value (`SVar:<Name>:<body>`) in `cards.Face.SVars`: a sub-ability (named by
`Execute$` or `SubAbility$`) or a value expression (`Count$…`) a parameter refers to.

**IR (intermediate representation).** What `engine/cards` compiles a script
into: a `cards.Card` holding one or more `cards.Face`s, each with its
`Abilities`, `Triggers`, `Statics`, `Repls`, `Keywords` and `SVars` (types in
`engine/cards/ir.go`). The whole corpus is cached as
`.cards/ir-<fingerprint>.gob.gz`.

**Face.** One printed face (`cards.Face`); split, adventure, modal and transforming cards have
more (`ALTERNATE` starts the next). `Object.Face()` is the face an object shows.

**Intrinsic.** An ability the rules grant, not the script: a basic Forest's "{T}: Add {G}" comes
from `Face.ApplyIntrinsics` (`engine/cards/intrinsic.go`, CR 305.6).

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

**Registry.** `cards.Registry`: every compiled card and token (`Registry.Lookup`); tests get the
shared one from `testutil.CorpusRegistry(t)`. Distinct from the *effect registry*
(`engine/effects/registry.go`) and the bot registry (`bots/`).

**Event.** One entry in the game's log (`events.Event`, kinds in `engine/events/event.go`). State
changes only through `events.Apply`; engine code proposes events with `e.emit` (rules) or
`Host.Emit` (effects). `events.Kind` is append-only.

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

**Seat.** Whoever answers a seat's decisions (bot, test script, human over HTTP): `seat.Seat`
(`engine/seat/seat.go`), one method `Decide(ctx, view.View, decision.Decision) (decision.Intent, error)`.
It never sees the engine.

**View.** One seat's redacted picture of the game (`view.View`, built by
`view.Project` in `engine/view`): hidden zones are counts, other players'
decisions are hidden. Clients and bots read state only through a view.

**Projection.** Building a seat's `view.View` (`view.Project`, `view.ProjectFor`;
`view.RedactEvents` for the log). Lives in `engine/view/card.go` and `stack.go`.

**Replay.** Re-running a match from its `Config` plus recorded intents and comparing the rebuilt
event stream event for event (`engine/replay`, `make sim`'s `-verify`, `cmd/repro`); also how a
server serves finished matches.

**Ratchet.** A test whose expected value is a committed table and which fails both when a gap
appears and when a listed gap closes (stale). Tables only shrink, e.g.
`engine/rules/testdata/known-unsupported/`, `paramcensus/`, the audit's `known-divergent/`.
See [Testing](testing.md).

**Census.** A walk over the corpus (or every card of one shape) counting how many behave a
certain way; a fix is checked against the whole class. The param census
(`engine/rules/paramcensus_test.go`) is a committed one.

**Oracle text / oracle scenario.** Oracle text is the printed rules text (the `Oracle:` line). An
*oracle scenario* is a JSON test whose expectations come from it and the CR, played by gorge's
runner. (*Compliance oracles* instead compare gorge with XMage and Forge.) Both live in
[gorge-compliance](https://github.com/adams-shaun/gorge-compliance); gorge keeps the runner and
residual scenarios under `engine/rules/testdata/oracle/`.

**CR citation.** A reference to the
[Magic: The Gathering Comprehensive Rules](https://magic.wizards.com/en/rules)
by rule number: "CR 113.6" (where abilities function), "CR 704.5a"
(a state-based action), "CR 613" (the layer system). Code comments and commit
messages cite the rule a line enforces.

**Known approximation.** A place where a registered primitive behaves more
narrowly than the card text, listed in `docs/agents/known-approximations/`.
The list only shrinks (`TestKnownApproximationsOnlyShrinks`; see
[Invariants](../architecture/invariants.md#ratchets-only-shrink)).
Deliberate behaviour that only looks like an approximation is an *engine
contract*, recorded in `docs/design/2026-09-22-engine-contracts.md`.

**Forge, `FORGE_REF`, the fork.** [Forge](https://github.com/Card-Forge/forge)
is an open-source Java Magic engine whose card scripts gorge compiles. gorge
reads them from its fork [adams-shaun/forge](https://github.com/adams-shaun/forge)
at the commit pinned by `FORGE_REF` in the `Makefile`.

## Repositories, engines and process

**gorge-compliance.** The public companion repository that checks gorge card
by card against XMage, Forge and hand-written Oracle-text scenarios, and
holds the certification ratchet. It imports gorge; gorge never imports it.
See [The three repositories](../repos/index.md).

**Private companion.** The maintainers' private companion module: machine
learning and private experiments. It imports gorge at
a pinned commit; nothing in gorge needs it.

**`GORGE_PIN`.** A file at the root of each companion repository holding the
full gorge commit SHA it builds against, mirrored by the gorge
pseudo-version in its `go.mod`. Moving it is a pull request in the companion
([Pinning](../repos/index.md#pinning)).

**`main` / merge queue.** Every change is a pull request to `main`. Mergify queues each non-draft PR once its
`fast` and `lint` checks pass, batches up to 10 on a `mergify/merge-queue/*` branch, and merges the batch with a
merge commit when `ci-ok` is green there. `hotfix/*` branches jump the queue. This site and the published coverage
are built from `main` ([Branch flow](../repos/index.md#branch-flow)).

**dirac.** The second rules engine, for up to four seats (`MaxSeats`), built for search and
self-play: a flat state in one buffer, driven as a `Step`/`Apply` machine, with cards as *kop*
tables generated from gorge's IR and play checked against gorge by a *lockstep oracle*. See
[dirac](../architecture/dirac/index.md).

**kop.** dirac's card-effect vocabulary: a closed, append-only set of
ops, and the tables (`kop.Table`, one entry per card) its interpreter runs.
The generator that lowers a card's gorge IR into kop tables is `kgen`.

**Lockstep oracle.** Plays one game in two engines with the same answers and compares projected
state and offered options at every decision; the first difference is a divergence. How dirac is
held to gorge (not the *compliance oracles*).

**Canonical projection.** The engine-neutral snapshot lockstep compares (per object card, zone,
position, flags, counters, derived P/T; then life, stack, turn, step, mana). See
[Verification](../architecture/dirac/oracle.md).

**Stall.** A game dirac ends as a draw with a named reason because it can't continue
correctly. A *capacity* stall means the position no longer fits a fixed array, and it is a declared
limit. An *unmodelled* stall means gorge does something there that dirac doesn't, and a strict
lockstep run fails on it. Neither is reported as a divergence.

**kgen.** Lowers gorge's IR into dirac's kop tables (binary, never committed); it reads every
parameter of a card or refuses it. Lowered is not supported: that also takes clean lockstep games
of the evidence corpus and no refusal by name. See [Cards](../architecture/dirac/cards.md).

**SpellBench v2.** An external, engine-neutral benchmark protocol for Magic
agents ([jackmaiorino/spellbench](https://github.com/jackmaiorino/spellbench)).
gorge serves it in both roles: as an environment
(`server/spellbench`, `cmd/spellbench-gorge-env`) and as an agent wrapping any
registered gorge bot (`cmd/spellbench-gorge-agent`). `gorged` and `mtgsim`
can also seat a v2 agent as a bot (`-sb-agent`).

**Draft event, pod, persona.** A draft event is a whole limited tournament,
not to be confused with an event in a game's log. A pod is its seats. A persona
is a bot entrant: a drafter, a builder and a game policy. See
[Draft events](../guide/draft.md).

**Seam.** An exported extension point other modules plug into without gorge
importing them: `bots.Register`, `server/gorged.RegisterNetBot`,
`server/gorged.Extensions`, and the exported oracle runner. The *engine seam*,
`engine/game`, is what a host needs from a running game; each engine implements it.

**Wire.** A client protocol a game can be played through: gorge's native HTTP API,
ManaBrew or SpellBench v2. See [Wires](../architecture/wires.md).

**Repo decks.** The deck files in `testkit/testutil/decks/`: 60-card constructed and
Commander. The *pinned* constructed list (`testutil.LegacyDeckNames`, after the Legacy
format) is what the golden games and `mtgsim` seat from.

**Honest root, `Env` policy.** An honest root is a copy of the game with every hidden card
redealt from what the seat has seen, for a search to read. An `Env` policy (`bots.Info.Env`)
builds one from every seat's real decklist, so it is never seated against a person.

**Caretaker.** The bot that answers for a human seat when nobody will: at table shutdown, or
after `-manabrew-think`. It is the table's own bot policy.

**Golden / goldens.** Committed expected outputs a test compares against
byte for byte: the chain heads in `engine/rules/testdata/heads/`, wire
frames, replay fixtures. A golden moves only with a named cause.
