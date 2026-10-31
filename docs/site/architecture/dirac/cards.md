# Cards

dirty-dirac does not parse card text and does not hand-write cards. Its
generator, `kgen`, reads **gorge's** compiled card IR and lowers each card into
programs over a small integer op vocabulary, `kop`; a card pool becomes a
generated Go package. This page explains that vocabulary, the lowering and its
refusal contract, how the card grammar is being closed across the whole
corpus, the layer system, what is generated and what is written by hand, and
how pool packages are found at run time. It is for readers who know gorge's
[card pipeline](../../contributing/card-pipeline.md) and want to see how the
same cards reach the fast kernel.

*Counts on this page were read on 2026-10-25 from the dirty-dirac source of
that day: the `kop` enums, and the acceptance ratchet's pinned counts.*

## The pipeline

<figure markdown>
<svg viewBox="0 0 760 420" width="100%" role="img" aria-label="Card pipeline: a Forge card script is compiled by gorge into IR; kgen loads a pool's cards from the registry, lowers each one, refusing any unread parameter, builds side tables and emits a Go package per set; the cardset registry and a matchup builder turn that into a kop.Table the hand-written kernel interpreter runs">
  <defs>
    <marker id="cp-a" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse"><path d="M0,0 L10,5 L0,10 z" class="dg-arrow"/></marker>
    <marker id="cp-b" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse"><path d="M0,0 L10,5 L0,10 z" class="dg-arrow-accent"/></marker>
  </defs>
  <text x="12" y="20" class="dg-text-muted">gorge (public)</text>
  <rect x="10" y="28" width="220" height="70" rx="8" class="dg-box"/>
  <text x="120" y="50" text-anchor="middle" class="dg-text-bold">Forge card script</text>
  <text x="120" y="68" text-anchor="middle" class="dg-text-muted">.cards/, fetched at FORGE_REF</text>
  <text x="120" y="84" text-anchor="middle" class="dg-text-muted">GPL-3.0, never committed</text>
  <line x1="232" y1="63" x2="266" y2="63" class="dg-line" marker-end="url(#cp-a)"/>
  <rect x="270" y="28" width="220" height="70" rx="8" class="dg-box"/>
  <text x="380" y="50" text-anchor="middle" class="dg-text-bold">engine/cards compiler</text>
  <text x="380" y="68" text-anchor="middle" class="dg-text-muted">parse, expand keywords,</text>
  <text x="380" y="84" text-anchor="middle" class="dg-text-muted">IR cache</text>
  <line x1="492" y1="63" x2="526" y2="63" class="dg-line" marker-end="url(#cp-a)"/>
  <rect x="530" y="28" width="220" height="70" rx="8" class="dg-box"/>
  <text x="640" y="50" text-anchor="middle" class="dg-text-bold">cards.Registry</text>
  <text x="640" y="68" text-anchor="middle" class="dg-text-muted">faces, abilities, triggers,</text>
  <text x="640" y="84" text-anchor="middle" class="dg-text-muted">statics, SVars, $ params</text>

  <rect x="10" y="118" width="740" height="132" rx="10" class="dg-box-muted"/>
  <text x="22" y="136" class="dg-text-muted">kgen (dirty-dirac's generator, run by hand; output checked in)</text>
  <line x1="640" y1="100" x2="640" y2="144" class="dg-line-accent" marker-end="url(#cp-b)"/>
  <rect x="575" y="148" width="165" height="62" rx="8" class="dg-box-accent"/>
  <text x="657" y="170" text-anchor="middle" class="dg-text-bold">load the set</text>
  <text x="657" y="188" text-anchor="middle" class="dg-text-muted">a deck pool, a card</text>
  <text x="657" y="202" text-anchor="middle" class="dg-text-muted">list or the corpus</text>
  <line x1="573" y1="179" x2="537" y2="179" class="dg-line-accent" marker-end="url(#cp-b)"/>
  <rect x="390" y="148" width="145" height="62" rx="8" class="dg-box-accent"/>
  <text x="462" y="170" text-anchor="middle" class="dg-text-bold">lower</text>
  <text x="462" y="188" text-anchor="middle" class="dg-text-muted">every $ param read,</text>
  <text x="462" y="202" text-anchor="middle" class="dg-text-muted">or the card is refused</text>
  <line x1="388" y1="179" x2="352" y2="179" class="dg-line-accent" marker-end="url(#cp-b)"/>
  <rect x="205" y="148" width="145" height="62" rx="8" class="dg-box-accent"/>
  <text x="277" y="170" text-anchor="middle" class="dg-text-bold">build</text>
  <text x="277" y="188" text-anchor="middle" class="dg-text-muted">intern filters, values,</text>
  <text x="277" y="202" text-anchor="middle" class="dg-text-muted">conditions, overrides</text>
  <line x1="203" y1="179" x2="167" y2="179" class="dg-line-accent" marker-end="url(#cp-b)"/>
  <rect x="20" y="148" width="145" height="62" rx="8" class="dg-box-accent"/>
  <text x="92" y="170" text-anchor="middle" class="dg-text-bold">emit</text>
  <text x="92" y="188" text-anchor="middle" class="dg-text-muted">gofmt'd Go, one file</text>
  <text x="92" y="202" text-anchor="middle" class="dg-text-muted">per card and token</text>
  <line x1="462" y1="212" x2="462" y2="230" class="dg-line-dashed"/>
  <text x="472" y="242" class="dg-text-muted">refused: listed in the coverage report, kept as an inert fallback</text>

  <line x1="92" y1="212" x2="92" y2="276" class="dg-line-accent" marker-end="url(#cp-b)"/>
  <rect x="10" y="280" width="230" height="96" rx="8" class="dg-box"/>
  <text x="125" y="302" text-anchor="middle" class="dg-text-bold">cards_&lt;pool&gt; package</text>
  <text x="22" y="322" class="dg-text-mono">card_*.go, token_*.go</text>
  <text x="22" y="338" class="dg-text-mono">pool.go, matchup.go</text>
  <text x="22" y="354" class="dg-text-mono">ids.tsv  (stable CardIDs)</text>
  <text x="22" y="370" class="dg-text-mono">coverage.md, OPS.md</text>
  <line x1="242" y1="328" x2="268" y2="328" class="dg-line" marker-end="url(#cp-a)"/>
  <rect x="272" y="296" width="140" height="64" rx="8" class="dg-box"/>
  <text x="342" y="320" text-anchor="middle" class="dg-text-bold">cardset</text>
  <text x="342" y="338" text-anchor="middle" class="dg-text-muted">registry by pool name</text>
  <line x1="414" y1="328" x2="440" y2="328" class="dg-line" marker-end="url(#cp-a)"/>
  <rect x="444" y="296" width="140" height="64" rx="8" class="dg-box"/>
  <text x="514" y="320" text-anchor="middle" class="dg-text-bold">Matchup(a, b)</text>
  <text x="514" y="338" text-anchor="middle" class="dg-text-muted">→ *kop.Table + decks</text>
  <line x1="586" y1="328" x2="612" y2="328" class="dg-line" marker-end="url(#cp-a)"/>
  <rect x="616" y="296" width="134" height="64" rx="8" class="dg-box-accent"/>
  <text x="683" y="320" text-anchor="middle" class="dg-text-bold">kernel</text>
  <text x="683" y="338" text-anchor="middle" class="dg-text-muted">hand-written</text>
  <text x="683" y="352" text-anchor="middle" class="dg-text-muted">interpreter</text>
  <text x="12" y="404" class="dg-text-muted">Only kgen reads gorge. The generated packages and the kernel import nothing from gorge.</text>
</svg>
<figcaption>From Forge script to a playable table. The top row is gorge's own
pipeline; the accented boxes are dirty-dirac's.</figcaption>
</figure>

The key decision is the second step's input: **kgen lowers gorge's compiled
IR, not the Forge script**. The kernel therefore inherits gorge's reading of
every card: keyword expansion, SVar resolution and parameter meaning are
gorge's. That keeps the two engines comparable (the oracle can demand equal
behaviour) and avoids a second script parser. It also means a misreading in
gorge is a misreading in dirty-dirac; see the
[independence caveat](oracle.md#what-lockstep-cannot-see).

kgen runs in two modes. Given a deck pool or a card list it generates a
`cards_<pool>` package that a kernel game can play. Given any card list, the
whole corpus included, it can also lower every card without writing a
package and report which ones it would accept: that is how corpus and per-set
acceptance are measured.

## The kop vocabulary

A card is a `kop.CardDef`: printed cost, types, subtypes, keywords, colours,
power, toughness and loyalty, a few native fields (kicker and flashback costs,
ward, equip cost, the Aura's enchant target, protection, landwalk, the faces
of a multi-face card) and a list of `kop.Ability`. An ability is one of seven
kinds (spell, activated, mana, triggered, static, replacement, alternative
cost) and carries its cost, its trigger subscription or static record, up to
three target slots, modes, an optional threshold gate (a Class level band) and
a **program**: a flat slice of instructions.

```go
type Instr struct {
    Op  Op        // one of 148 ops
    Obj Ref       // who or what it acts on: a target slot, the source, the controller, each opponent, ...
    A   ValueRef  // an amount: a constant, X, a count of a filter, last damage, a power, a side-table expression
    B   ValueRef  // a second amount
    C   int16     // an op-specific constant: a keyword mask, a counter kind, a token, a zone, a skip count
    F   FilterID  // a filter in the table, for the "all", search and choice ops
}
```

Two cards as kgen emits them, abridged:

```go
var shivanDragon = kop.CardDef{               // {4}{R}{R}, 5/5 flying
    Cost: kop.ManaCost{Generic: 4, Pips: [6]uint8{0, 0, 0, 2}},
    Types: kop.TCreature, Keywords: kop.KwFlying, Power: 5, Toughness: 5,
    Abilities: []kop.Ability{{
        Kind: kop.AbActivated,                   // {R}: +1/+0 until end of turn
        Cost: kop.Cost{Mana: kop.ManaCost{Pips: [6]uint8{0, 0, 0, 1}}},
        Prog: []kop.Instr{{Op: kop.OpPumpUntilEOT, Obj: kop.RefSelf, A: kop.Const(1)}},
    }},
}

var burstLightning = kop.CardDef{             // {R} instant, kicker {4}
    Types: kop.TInstant, Keywords: kop.KwKicker, Kicker: kop.ManaCost{Generic: 4},
    Abilities: []kop.Ability{{
        Targets: [3]kop.TargetSpec{{Kind: kop.TgtAny}}, NTargets: 1,
        // 2 damage, or 4 if kicked: an expression in the table's side tables
        Prog: []kop.Instr{{Op: kop.OpDealDamage, Obj: kop.RefTarget0, A: kop.ValueRef{K: kop.VExpr, N: 3}}},
    }},
}
```

What is in the vocabulary:

| Enum | Values | Examples |
|---|---|---|
| `Op` (a `uint8`) | 148: 61 in the original vocabulary, the rest appended by kgen | `DealDamage`, `Draw`, `Destroy`, `CreateToken`, `Scry`, `Fight`, `Counter`, `Charm`, `Proliferate`; appended: `UnlessPaid`, `SearchEx`, `Override`, `CopySpell`, `GiveGift`, `ClassLevelUp`, `Explore`, `Saddle` |
| `TrigEvent` | 30 | enters, dies, leaves, to graveyard, attacks, phase, spell cast, life gained, damage dealt, becomes target, drawn, discarded, counters added, blocks, becomes blocked, sacrificed, taps, untaps, mana expended, gift given, token created, becomes monstrous, turned face up, transformed, crime committed, door unlocked |
| `StaticKind` | 17 | continuous P/T and keywords, cost changes, can't block, may play from a zone, override, player hexproof, can't be cast, control (layer 2) |
| `ReplKind` | 21 | enters tapped, enters with counters, as-enters choice, double damage, damage plus N, prevent noncombat damage, double tokens, double counters, life doubled, life plus N, graveyard-to-library, exiled instead of dying, counters of one kind doubled or increased |
| `KwMask` (a `uint32`) | 32 of 32 bits used | flying, first strike, deathtouch, ward, protection from black or red, fear, shadow, ... |
| `CounterKind` | 38 counter kinds plus a Class level | +1/+1, -1/-1, loyalty, stun, shield, flying, lore, defense, ...; keyword counters (first strike, deathtouch, trample, ...); named kinds with no rules meaning of their own (charge, oil, quest, ...) |

Every enum is **append-only**: generated tables encode values by ordinal, so a
new op goes immediately above the count constant, never in the middle. The
keyword mask is full and is not widened (that would grow every permanent):
further keywords are either lowered into abilities, statics and effect
records, or carried as card fields. The interpreter contract is in `kop`'s
package doc:

- A resolving ability runs its program front to back against the stack entry's
  source, controller, targets, modes, X and kicked flag.
- An instruction whose target has become illegal is a no-op; if every target is
  illegal the whole program is skipped (CR 608.2b).
- Branch ops (`Charm`, `IfKicked`, `SkipIf`, `Repeat`, ...) are structured: they
  skip or loop over the instructions immediately following them.
- Decision-raising ops (choose a colour, a card, scry, search, dig, ...)
  suspend the resolution; the kernel poses a decision and resumes at the same
  instruction ([Step and Apply](step-apply.md)).
- **An op the interpreter does not implement panics, naming itself. It is
  never a silent no-op.**

The variable-length parts live in the table, not in the state: a
`kop.Table` holds the cards, the filters, the subtype names, and four side
tables kgen fills (value expressions, conditions, override records, emblems),
plus the mapping from gorge's 362-word creature-type vocabulary to the table's
subtypes.

## Lowering: read every parameter or refuse

kgen walks each card's IR: its abilities and their `SubAbility` chains, its
triggers, statics and replacement effects, its keywords and SVars. Each node's
`$` parameters are wrapped in a tracker that records which ones the lowering
read. When a node is done, any rules parameter left unread **refuses the
card**, with the node and the reason. Only descriptive and AI-hint parameters
(`SpellDescription`, `AILogic`, `TgtPrompt`, ...) are ignored by name.

The same rule holds inside a parameter's value. A filter property, a value
head, a value modifier (`/Plus.Y`, `/Times.X`) or a cost token that kgen
cannot parse is a refusal, never a default: kgen never lowers an unparsed
amount as 0 or an unknown property as "anything". A test feeds every modifier,
parameter and value-head switch an unknown token and asserts a refusal. This
rule exists because the one class of silent mis-lowering found so far (a
modifier that named an SVar and was read as +0) produced games that agreed
with gorge until the missing amount mattered.

Typical refusals, from the BLB coverage report: an unknown filter property
(`TopLibrary`, `greatestPowerControlledByTargeted`), an unread parameter (`Optional` on a mill, a
`ClassBand` on a static), a value modifier it cannot evaluate, a replacement
shape it does not model.

A refused card is not dropped from the package. It is emitted as an **inert
fallback**: printed cost, types, subtypes, colours and P/T, no abilities. Two
consequences:

- `Matchup` refuses a deck that contains one, so a kernel game never plays a
  card as vanilla by accident.
- The whole-pool table (`Pool`) still has a row for it, so a search world
  that deals hidden cards from a belief over the pool can deal it, as a
  placeholder.

The generator also writes `OPS.md`, the interpreter contract for every op,
enum value and field it appended, with the cards that use each one, and a test
fails if an appended op has no entry. Another test regenerates every pool into
a temporary directory and diffs it against the checked-in package, so the
checked-in tables can never drift from the generator.

**Stable card IDs.** `ids.tsv` is a checked-in index of every `CardID` ever
assigned in the pool. An ID never moves: a new card takes the next free ID, and
a card that leaves the pool keeps its ID as an inert placeholder. Recorded
games and caches keyed by `CardID` stay valid across regenerations.

## Growing the card grammar

The kernel was built on FDN, and the first measurement over the whole corpus
showed what that meant: most cards in other sets were refused, and what
refused them was overwhelmingly **shared grammar** (filter properties,
`Defined$` forms, parameters, value heads, common effects and trigger modes),
not each set's named mechanics. Fixing a set at a time would re-pay that
grammar for every set. So the grammar is closed once, corpus-wide:

1. **Classify.** kgen normalises every refusal reason into a class (one per
   filter property, parameter, effect, keyword, trigger mode, ...) and tags
   each class with the kind of work it needs: generator-only, a keyword
   desugaring, a new op or predicate arm in the interpreter, or a new rules
   mechanism with its own `State` block. The coverage report prints
   "Refusals by class and tier".
2. **Close in greedy order.** Classes are closed in order of how many corpus
   cards each unlocks. Most of the top classes are generator-only work or a
   single new op; only a handful (multi-face cards, face-down cards,
   Crew-style aggregate tap costs) need a mechanism.
3. **Ratchet.** An acceptance test lowers the whole corpus and the card list
   of each registered set, and pins each accepted count exactly. A change
   that lowers any count fails; a rise is re-pinned in the same change, so
   the pins are always the live numbers. Today (2026-10-25) they stand at **17,126 of
   34,072** corpus cards; the per-set pins are on
   [Card support by set](../../support/sets.md#per-set-support).
4. **Keep the verified pools still.** A lowering change must leave the FDN
   and BLB lockstep results unchanged. A change that moves them is a defect
   in the lowering, not a new behaviour.

An accepted card is lowered, not verified: it plays in lockstep only once it
is in a pool that is fuzzed against gorge ([Verification](oracle.md)). When a
card that is accepted turns out to differ from gorge in a later lockstep, the
fix is a refusal or a corrected lowering; the ratchet may fall when a class is
found to have been accepted too eagerly, and that fall is recorded rather than
hidden.

## Generated and hand-written

| Part | Generated by kgen | Written by hand |
|---|---|---|
| Card definitions, ability programs, filters, expressions, conditions, override records | yes, one Go file per card and token | |
| Pool index, name table, `Matchup` and `Pool` builders, pool registration | yes (`pool.go`, `matchup.go`) | |
| `ids.tsv`, coverage report, `OPS.md` | yes | |
| The `kop` types and enums | kgen appends values and fields (append-only) | the original vocabulary and the types |
| The interpreter and every rule (turns, priority, casting, combat, state-based actions, layers) | | yes, in `kernel` |
| The generated-table extension: the code behind kgen's ops, statics, replacements and filter predicates | | yes, in `kernel`'s `*_gen.go` files (named for the tables they serve, not generated) |
| A tiny test table (basic lands, vanilla and keyword creatures, a few spells) | | yes, package `vanilla` |

The `*_gen.go` files are worth naming because the suffix misleads: they are
hand-written kernel code that only runs on a generated table. Examples:

- **Characteristics**: computing a permanent's types, keywords and P/T
  through the layer system (below).
- **Spell copies**: a copy is a new card object on the stack with the
  original's targets, modes, X and kicked status; it is not cast, and it
  ceases to exist when it leaves the stack (CR 707.10, CR 704.5e).
- Replacement effects, triggers, statics, legality of plays, target slots
  taking several objects.

On a hand-written table every one of those hooks is skipped. Each newer rules
block sits behind its own per-table gate in the same way: a table with no card
that needs it pays one predictable branch.

## Layers: the effect pool and the CR 613 walk

dirty-dirac derives a permanent's characteristics with a small, fixed-size
version of gorge's CR 613 layer system. The derivation applies layers 1 to 7
in order. Within each layer it applies effects in timestamp order and uses
gorge's dependency rules. It allocates nothing, and every mutable byte lives
in `State`.

**The effect pool.** An ability that resolved (CR 611.2) records its
continuous effect as a 12-byte `fxRec` in a fixed pool of 16 in
`State`. A record names its object, because the affected set is fixed at
resolution (CR 611.2c).

- Records are appended, and `State.tick()` is monotonic, so pool order is
  timestamp order and nothing is ever sorted.
- Cleanup drops until-end-of-turn records. The controller's next cleanup
  drops "until your next turn" records.
- A full pool stalls the game (`StallTables`). It never drops an effect
  silently.

The record kinds are:

| kind | layer | what it holds |
|---|---|---|
| `fxCopyExc` | 1 | a copy's exceptions (Offspring's "except it's 1/1", an added subtype or keyword), copiable by a later copy (CR 707.9b) |
| `fxControl` | 2 | a control change and its duration (end of turn, your next turn, while the source stays, permanent), settled eagerly into the controller |
| `fxOverride` | 3-7b | a `kop.Override`: set or add types, subtypes and colours, a name, lose all abilities, add or remove keywords, set base P/T |
| `fxLoseAll` | 6 | a one-shot "loses all abilities" and its timestamp |
| `fxKwGrant` | 6 | an until-end-of-turn keyword grant's timestamp, kept only when the table has a static removal (Starforged Sword) or a global lose-all |

**Statics stay live.** Battlefield statics are not copied into the pool.
The walk reads them in place and stamps each one with its source's layer
timestamp. An Aura or Equipment is restamped when it becomes attached (CR
613.7e), and an emblem's timestamp is 0. Commutative effects keep their
cheap accumulators: +N/+N pumps, counters and anthems (7c) add in any order.

**The walk.** `chars(k, slot, upTo)` builds a stack-local `dchars`, one
layer at a time:

| layer | applied |
|---|---|
| 1 | the card or its copy-of, plus the copy exceptions |
| 2 | control, already settled from the `fxControl` records |
| 3 | the newest rename whose filter matches the finished layer-4 types (Witness Protection's name) |
| 4 | the changeling CDA first (CR 613.3), then override records, override statics and static subtype grants in gorge's CR 613.8 dependency order (timestamp order unless one effect changes what another applies to; a cycle falls back to timestamp order). Each static's filter is matched against the types built so far. |
| 5 | colour setters by timestamp |
| 6 | printed keywords, keyword counters and copy keywords as the base; then losses, removals and grants by timestamp, with a removal before an addition on a tie, reordered by gorge's `abilityDependencyOrder` |
| 7a / 7b | the CDA P/T, the copy's set P/T, then base-P/T setters by timestamp |
| 7c | counters, pumps and anthems |

An override static that applied in layer 4 keeps applying in the later
layers (CR 613.6). A permanent with at most one order-sensitive effect, or
with records only, takes a fixed-order fast path, which gives the same
answer. A table with nothing order-sensitive pays one predictable branch
per read.

**What kgen refuses instead.** A layer shape the walk does not implement is
refused per card when kgen lowers it, never silently mis-ordered: P/T
switching (7d, Twisted Image), layer-3 text changes (Artificial Evolution),
P/T set from a dynamic value (so Humility never meets Opalescence), and a
few override shapes that could form a 613.8 cycle. Every pair of
order-sensitive effects the kernel does accept is ordered by the walk, and
each ordering has a scenario whose expected answer comes from gorge's own
oracle runner.

Cards that need behaviour no generic op expresses get it in one of three
ways, in order of preference:

1. a new append-only op or enum value, with its contract in `OPS.md`;
2. an override record;
3. kernel-native code keyed by a `kop` kind. Ward and granted prowess are
   kernel-native triggers. Twinflame Tyrant, Crystal Barricade, Doubling
   Season and Progenitus are fixed replacement kinds.

No kernel code path branches on a card's name. A card's name is in its
`CardDef` for reports only.

## Pool packages and the cardset registry

Each generated pool is its own package, `cards_<pool>`. A pool is generated
either from a deck pool (every distinct card in a directory of deck lists;
FDN) or from a card list (BLB's is every distinct card name on the set's
booster sheets). Besides FDN and BLB there are 74 small constructed pools, each
chosen so that the cards a piece of grammar work unlocked come up in play
under lockstep ([Verification](oracle.md#agreement-today)).

Each package implements `cardset.Set` and registers itself under its pool name
in `init`:

| Method | Returns |
|---|---|
| `Name()` | the pool name, the registry key |
| `Lookup(name)` | a card's pool index |
| `Supported()` | per pool index (cards, then tokens), whether the generated table implements it |
| `Matchup(a, b)` | a `kop.Table` for two decks plus each deck as `CardID`s; fails on an unsupported card |
| `Pool()`, `PoolID(i)`, `InertID(id)`, `NumIDs()` | the whole-pool table and its stable IDs |
| `AbilityOrigin(i, ab)` | which gorge ability (face ability or trigger index) a kop ability came from, for the oracle |
| `Names()`, `Subtypes()` | the name and subtype vocabularies |

The registry is a slice, not a map, so nothing can depend on map order. Tools
select a pool by name (`-set fdn`).

## Sizing the vocabulary: gorge's kcensus

Before the vocabulary existed, gorge's
[`cmd/kcensus`](https://github.com/adams-shaun/gorge/blob/main/cmd/kcensus/main.go)
measured what it would need. It loads a deck pool, resolves every card through
the compiled corpus, lists the mechanics each card reaches (API codes along
every sub-ability, trigger and SVar chain; trigger, static and replacement
modes; keyword heads; cost tokens), and prints the smallest mechanic set that
fully covers 50, 80, 95 and 100 % of the decks, plus how many decks need
general layer, replacement, copy or control machinery. Its labels use the same
spelling as gorge's coverage ratchet (`api:`, `trig:`, `stat:`, `repl:`, `kw:`),
so its output lines up with gorge's own support reports.
