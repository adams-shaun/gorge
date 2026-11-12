# Card support by set

This page says which *Magic* cards and sets each engine in the gorge family
supports, and what "supported" means for each. The word means something
different at every tier. The page is for players choosing a format, bot
authors choosing a card pool, and contributors looking for the card work with
the most leverage.

!!! info "Where the numbers come from"
    - **Generated at every docs build:** the engine headline table, the per-set
      table and the roll-ups. They join gorge-compliance's nightly per-set
      status with dirac's per-set support page. Each table's
      caption gives the date and Forge corpus it measured, which can trail the
      corpus pin below.
    - **Measured by hand:** the corpus figures, the leverage ranking and the
      repo-deck ratchets. Each names its date and revision.

    [Regenerating the tables](regenerating.md) gives the command behind each
    number.

## Headline

**gorge, playable:** 32,992 of
34,444 Forge card scripts at the corpus pin. All of them compile into gorge's IR
(`make report`, 2026-11-11, `a99704ac7`).

--8<-- "support/_generated_sets.md:headline"

## At a glance

One tile per Standard set (the sets on dirac's support page). The tint and the
large figure are the share of the set's cards dirac supports; the rest are
unverified (lowered, not yet evidenced by lockstep games) or refused. When this
build has gorge-compliance's status, each tile also gives gorge's playable share.

--8<-- "support/_generated_sets.md:glance"

## What "supported" means, engine by engine

Each tier certifies more than the one before it, and each has its own test.
Don't read a number from one tier as a claim at another. For example, *playable*
in gorge says which primitives are implemented. It doesn't say the cards behave
correctly.

| Engine | A card is supported when | Checked by |
|---|---|---|
| gorge, **playable** | every primitive its compiled script references is implemented | `make report` (`Registry.Coverage`) |
| gorge, **certified Level A** | it is playable and its cast-and-resolve (or play-land, or counter-spell) scenario agrees with XMage, carries a ruled divergence, or passes a hand-written Oracle scenario | gorge-compliance's gate |
| gorge, **certified Level B** | Level A plus one XMage-compared scenario per activated ability, trigger mode, static ability and attack/block | gorge-compliance's gate |
| dirac, **lowered** | its generator (`kgen`) can represent every ability in the engine's op tables | dirac's build ratchet |
| dirac, **supported** | it is lowered, not refused by name, and clean in the lockstep games of the evidence corpus, compared with gorge at every decision | dirac's lockstep oracle, [`dirac/docs/support.md`](https://github.com/adams-shaun/gorge/blob/main/dirac/docs/support.md) |

```mermaid
flowchart LR
  F["Forge card script<br/>(GPL, fetched at FORGE_REF)"] -->|forgec compile| IR["gorge IR"]
  IR -->|every primitive implemented| P["gorge: playable"]
  P -->|"scenario agrees with XMage<br/>(or ruled / hand scenario)"| A["certified Level A"]
  A -->|"one scenario per ability,<br/>trigger, static, attack/block"| B["certified Level B"]
  IR -->|"kgen lowers into op tables"| K["dirac: lowered"]
  K -->|"not refused; clean lockstep games<br/>of the evidence corpus"| L["dirac: supported"]
```

### gorge: playable

A card is **playable** when every [primitive](../contributing/glossary.md) its
compiled script references is implemented. That covers each `api:` effect,
`trig:` trigger mode, `kw:` keyword, `stat:` static mode, `repl:` replacement
and the `count:` value heads its SVars read. Clone-style enters-the-battlefield
replacements also have to pass a shape gate. The check is
[`Registry.Coverage`](https://github.com/adams-shaun/gorge/blob/main/engine/cards/validate.go)
against `effects.Supported()`, run by `make report` over the whole corpus.

Playable is necessary, not sufficient. A card can be playable and still
misbehave. The effect may not read one of its `$` parameters, or a primitive
may handle the common shape but not this card's. Two ratchets watch the cards
that matter most, the ones in the repository's own decks (see
[The repo-deck ratchet](#the-repo-deck-ratchet)).

### gorge certified: Levels A and B

[gorge-compliance](https://github.com/adams-shaun/gorge-compliance) is the
public compliance repository. It pins one gorge commit and checks it card by
card against independent references. One reference is hand-written scenarios
derived from the card's Oracle text and the Comprehensive Rules. The other is
**XMage**, an independent Java engine. For each card, gorge-compliance
generates scenarios, replays them in both engines, diffs the two engines'
snapshots at each checkpoint, and records a **verdict** per (card, template,
scenario hash). **Forge** runs as a third leg to adjudicate disagreements.

| Level | Not covered |
|---|---|
| **A** | activated abilities, trigger modes, attacks and blocks, statics. Basic lands are skipped. A card XMage does not implement needs a hand scenario |
| **B** | a requirement no scenario template serves yet is reported as a *template gap* and waits for its template |

The definitions are the gate's own wording,
[`gate.LevelMeaning`](https://github.com/adams-shaun/gorge-compliance/blob/main/oracle/compliance/gate/gate.go).
A verdict stays valid only while gorge still meets its frozen expectation and
the scenario's hash is unchanged. A moved scenario shows as a *stale verdict*
until XMage replays it. A *harness* problem means the test rig, not the
engine, could not express the scenario.

**Declared sets and the ratchet.** A set is *declared* at a level in
[`declared.json`](https://github.com/adams-shaun/gorge-compliance/blob/main/oracle/compliance/declared.json),
and CI fails if a declared set has any problem at that level. Only a set with
a full printed card list can be declared, because XMage's set manifests omit
the cards XMage lacks. The certification ratchet,
[`ratchet.json`](https://github.com/adams-shaun/gorge-compliance/blob/main/oracle/compliance/ratchet.json),
holds every other set's problem count, and that count may only fall. All of
these targets describe the pinned gorge commit. Bumping the pin is a pull
request that re-measures them.

The committed verdicts are counted by status (`agree`, `xmage_wrong`, `harness`, `diverge`, `gorge_wrong`)
with [the count's command](regenerating.md); the totals change with every pin bump, so none are written here. `xmage_wrong` is a ruled XMage error with a CR citation.

### dirac: lowered and supported

dirac is a second rules engine, and gorge is the reference it is checked against. It doesn't interpret
card scripts. Instead, a generator (`kgen`) **lowers** each card's gorge IR into fixed op tables, over the
whole Forge corpus. A ratchet pins the number of cards that lower, corpus-wide and per set. A fall fails,
and a rise is re-pinned.

Being lowered is much weaker than being **supported**. A supported card also isn't refused by name, and it
has evidence: **lockstep** games, which run dirac beside gorge on the same decks and seed and compare the
options offered at every decision and the state after every action, played it cleanly in the evidence corpus.
A game plays supported cards only. The counts come from
[`dirac/docs/support.md`](https://github.com/adams-shaun/gorge/blob/main/dirac/docs/support.md), which is generated.

Supported doesn't mean right or free of every difference. dirac shares gorge's bugs, because lockstep can't see
a mistake both engines make, and a sweep outside the evidence corpus can still find a difference on a pair of
supported cards. The FDN and BLB slices are the committed games a clone can replay, 1,500 seeded pairs each. A game
that hits a capacity limit, with no disagreement up to that point, counts as *stalled*. See
[dirac](../architecture/dirac/index.md#what-it-supports).

dirac's per-set card list is the distinct card names in XMage's manifest
for the set that exist in the Forge corpus. That list can differ by a few cards
from gorge-compliance's count, which uses the printed card list where one
exists. Each engine's column uses its own denominator. dirac's
corpus-wide count is distinct card names, 34,442, and `make report`'s
34,444 counts script files.

### XMage and Forge, as reference points

These are the two engines gorge measures against. Their numbers are their own
claims or a plain count, not certifications.

| Engine | Claim | Source |
|---|---|---|
| XMage | "over 32 000 unique cards and more than 91 000 reprints across official sets" | the [XMage README](https://github.com/magefree/mage), read 2026-10-20 |
| Forge | no card-count claim in its README; its card corpus at gorge's pin holds 34,444 card scripts and 864 token scripts | `.cards/cards.lock` and `make report` |

Forge is where gorge's card scripts come from, so Forge's corpus is gorge's
denominator. XMage supplies the set manifests (591 sets, built from its set
classes) and the second opinion behind every verdict.

## Charts by engine

One chart per engine over the Standard sets (the sets on dirac's support
page). The card-by-card detail is in the tables below and in
[`dirac/docs/support.md`](https://github.com/adams-shaun/gorge/blob/main/dirac/docs/support.md).

--8<-- "support/_generated_sets.md:charts"

## Per-set support

The table has one row for every set that gorge-compliance audits against a
full printed card list or that dirac's support page lists. It is sorted
newest first.

--8<-- "support/_generated_sets.md:sets"

- **Cards** is gorge-compliance's count: the printed card list for an audited
  set, otherwise XMage's manifest for the set.
- **Playable %** = 1 − (unsupported + not in corpus) / cards.
- **Level A %** and **Level B %** give the share of cards with no problem at
  that level. A set without a printed list can't be certified. Its Level A
  figure is marked ≥ because it is a hard lower bound: it counts the cards
  that already pass through a same-name verdict from an audited set, or as a
  basic land.
- **dirac supported** and **dirac lowered** are counts / cards over dirac's own
  card list for the set.
- **Lockstep games** is agreeing games / games in the set's committed slice, on
  `-surface full`. The support page carries no slice results, so the column is
  "—" until a source for them is wired in.

## Every set

gorge-compliance measures all 591 XMage set manifests, not only the audited
ones. For every set without a printed list, the figures are estimates, not
certifications.

--8<-- "support/_generated_sets.md:rollup"

### Method and error bars

1. **Denominator.** XMage's manifests omit cards XMage has not implemented.
   Across the audited sets, the manifest differs from the printed list by −12
   to +5 cards (gorge-compliance `6108eb1`, 2026-11-04). Reality Fracture is 2.5%
   short and The Hobbit 6.2% short. So the newest unaudited sets may be
   overstated by a few points, and older sets by under one point.
2. **Name folding** between XMage and Forge names: about ±1 card per set.
3. **Playable is not correct.** An audit finds problems that playable can't
   see. *Special Guests*, the one audited set not yet declared, sits below its
   playable figure at Levels A and B. Expect an unaudited set to land below
   its playable figure in the same way.
4. **Corpus-wide versus per-set.** The corpus-wide share counts distinct
   cards, including Un-sets and Alchemy. The roll-up counts each printing in
   each set. Both are correct. They answer different questions.

## Corpus coverage at the pin

[Getting started](../getting-started.md#1-fetch-and-compile-the-card-corpus) shows `make report`'s first lines. Every
script compiles. `forgec compile` reports 2 diagnostics, both an unsupported Specialize rider. Here is the breakdown
by card type (`make coverage`, at gorge `0ec285f67`, 2026-11-05):

| Card type | Cards | Playable | % |
|---|---:|---:|---:|
| Creature | 18,973 | 18,195 | 95.9 |
| Instant | 3,830 | 3,715 | 97.0 |
| Sorcery | 3,581 | 3,441 | 96.1 |
| Enchantment | 3,497 | 3,399 | 97.2 |
| Artifact | 2,543 | 2,398 | 94.3 |
| Land | 1,193 | 1,189 | 99.7 |
| Planeswalker | 336 | 324 | 96.4 |
| Battle | 37 | 35 | 94.6 |
| Other (planes, schemes, …) | 454 | 103 | 22.7 |

### Where the leverage is

The report ranks each missing primitive by the number of cards it alone would
unlock. The top of the list is mostly non-tournament play:

- Alchemy's `api:MakeCard` and `api:Draft`
- Planechase: `trig:ChaosEnsues` and `trig:PlaneswalkedTo`
- Archenemy: `trig:SetInMotion` and `api:Abandon`
- Unfinity's stickers and attractions, and Unstable's Contraptions

For cards that see tournament play, the leverage is in the older keywords
further down.

| Rank | Primitive | Cards unlocked | Notes |
|---:|---|---:|---|
| 1 | `api:MakeCard` | 186 | Alchemy |
| 2 | `trig:ChaosEnsues` | 158 | Planechase |
| 3 | `trig:SetInMotion` | 84 | Archenemy |
| 4 | `trig:PlaneswalkedTo` | 62 | Planechase |
| 5 | `kw:StickerSheet` | 48 | Unfinity |
| 6 | `trig:CrankContraption` | 45 | Unstable / Unfinity |
| 7 | `api:Draft` | 42 | Alchemy, Conspiracy |
| 8 | `kw:Bushido` | 37 | Kamigawa block |
| 9 | `api:PutSticker` | 36 | Unfinity |
| 10 | `kw:Choose a Background` | 32 | Commander Legends: Baldur's Gate |
| 11 | `kw:Disturb` | 32 | Innistrad |
| 12 | `kw:Splice` | 30 | Kamigawa block |
| 13 | `kw:Soulshift` | 26 | Kamigawa block |
| 14 | `stat:AssignNoCombatDamage` | 26 | |
| 15 | `kw:Modular` | 24 | |
| 16 | `api:Learn` | 23 | Strixhaven |
| 17 | `kw:Visit` | 23 | Unfinity attractions |
| 18 | `api:AssembleContraption` | 22 | Unstable |
| 19 | `api:OpenAttraction` | 22 | Unfinity attractions |
| 20 | `api:Abandon` | 20 | Archenemy |
| 21 | `api:Intensify` | 20 | Alchemy |
| 22 | `kw:Starting intensity` | 20 | Alchemy |
| 23 | `shape:ETBReplacement Clone` | 20 | clone-style "enter as a copy" |

A card can miss several primitives, so the counts don't add up. Implementing
one primitive unlocks only the cards that miss nothing else.

## The repo-deck ratchet

Two [ratchets](../contributing/glossary.md) hold the 34 decks in
[`testkit/testutil/decks/`](https://github.com/adams-shaun/gorge/tree/main/testkit/testutil/decks)
(15 60-card constructed, 19 Commander) to full support. Each ratchet keeps one
file per card:

```text
$ go test ./engine/rules/ -run 'TestEveryRepoDeckIsFullySupported|TestEveryRepoDeckParamsAreRead' -v
acceptance_test.go:70: ratchet: 0 of 1338 distinct cards across the repo decks are not fully supported
paramcensus_test.go:3674: param census: 0 of 1338 distinct repo-deck cards carry at least one unread param or unmodelled cost token; 0 distinct param labels, 0 distinct cost labels
```

- **Not fully supported** (0 on 2026-11-08). One file per card goes in
  `engine/rules/testdata/known-unsupported/`, a directory that doesn't exist while nothing is outstanding.
- **Parameter census** (0 on 2026-11-08). A listed card would be playable with at least one `$` parameter
  or cost token that isn't read yet, one file per card in `engine/rules/testdata/paramcensus/`, a directory
  that doesn't exist while nothing is outstanding.

Both tests fail in **both** directions. A card that newly loses support fails,
and so does a table entry the build now supports. To fix a card, delete its
file.
