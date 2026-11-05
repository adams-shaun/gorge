# How cards get into dirac

**dirac** is the second rules engine in this repository. It does not read Forge
scripts at run time: `dirac/kgen` (the card generator) compiles each supported card once into a binary
table, and dirac plays that data through the vocabulary in `dirac/kop` (**kop** is dirac's
op/instruction package). **gorge** is the rules engine in `engine/`; it leads and dirac follows:
`dirac/oracle` runs them in **lockstep** (the same game and seed, the same decision, every step compared)
— see [`dirac/docs/verification.md`](verification.md). This page explains that path, how to tell whether
a card is playable, and what adding support costs.

For dirac API itself see [`dirac/README.md`](../README.md) and the package docs in
[`dirac/doc.go`](../doc.go). For per-parameter lowering detail see
[`dirac/docs/kgen-params.md`](kgen-params.md). For how the result is checked against gorge see
[`dirac/docs/verification.md`](verification.md).

## Quick start: is card X supported?

A card is in one or more pools. `kgen -supported` answers from the pool's table, and the pool's
coverage report says why a card is refused. Tables are not committed, so build them first:

```sh
$ make dirac-tables
kgen: <n> tables, <size> bytes, key <key> in .../.cards/kernel-tables/<key>
$ go run ./dirac/cmd/kgen -supported fdn | grep Abrade
Abrade	true
$ go run ./dirac/cmd/kgen -supported blb | grep 'Sword of Fire'
Sword of Fire and Ice	false
$ grep -n '^| Sword of Fire and Ice ' <tables>/blb.coverage.md
45:| Sword of Fire and Ice | emit static[0] Continuous | keyword "ProtectionFromBlue" has no kop bit |
```

`<tables>` is the directory `make dirac-tables` prints. In `blb`, kgen refused `Sword of Fire and Ice` at
the static-ability emit step, because kop has no "protection from blue" keyword bit. "Supported" means
the table implements every ability; a refused card is still present in the pool as an inert vanilla
fallback for `Pool()` searches, but `Matchup` refuses a deck that contains it.

The concrete recipe:

1. **Find the pool.** `grep -rlx --include=cards.txt 'Sword of Fire and Ice' dirac/cardset/pools` prints
   `dirac/cardset/pools/blb/cards.txt`; the same for `Abrade` prints the `fdn` and `exileany` lists (the
   same card can be in several pools). If no list names the card, it has never been lowered into a
   pool; go to step 4.
2. **Check the refusal table.** `grep -n '^| <name> ' <tables>/<pool>.coverage.md`. A row is
   `| card | IR node | reason |`. No row means the card is supported. `Set.Supported()[poolIndex]` is the
   same answer in code; `Set.Lookup(name)` returns the pool index, and `Set.PoolID(index)` gives the
   stable CardID recorded in `dirac/cardset/pools/<pool>/ids.tsv`.
3. **Read the reason.** The IR-node column names where lowering stopped: `A[0]` is the first ability,
   `T[1]` the second trigger, `S[0]` a static, `R[0]` a replacement, `emit static[0]` the emit/build
   step, `ab[0]` an origin/ability link. The reason is the first unread Forge parameter, unknown filter
   property, kop shape or guard the lowerer hit. The report also groups refusals by reason and by
   class/tier (G generator, K keyword, O op/predicate, M mechanism, `?` unclassified).
4. **See what kgen saw, or lower a card that is in no pool.** `-dump` writes the compiled IR, `-lir`
   writes the lowered form plus refusals:

   ```sh
   $ printf 'Abrade\nSword of Fire and Ice\n' >/tmp/cards.txt
   $ go run ./dirac/cmd/kgen -card-list /tmp/cards.txt -lir
   CARD Abrade token=false cost={1R} types=0x20 sub=[] colors="R" ...
     ab[0] Spell  cost={0} sorcery=false zone=0 loyalty=false limit=0 modes=2
       target[0] mode=0 zone=1 min=1 max=1 F={types=0x2} ...
        0 Charm skip=1
        1 DealDamage obj=Target0 A=3
        2 Charm C=1 skip=1
        3 Destroy obj=Target1
   CARD Sword of Fire and Ice token=false cost={3} types=0x4 sub=[Equipment] ...
     static Continuous F={types=0x2 EquippedBy} ... kw=[ProtectionFromRed ProtectionFromBlue] ...
   # kgen coverage: pool fdn
   - cards supported: 1 of 2
   - refused cards: 1
   ```

   The last block is the same coverage report a table build writes. `-lir` prints each card's lowered
   IR, then runs build/emit and prints the coverage report, so refusals raised at that later step appear
   too: Sword's program lowers, but `emit static[0]` refuses its protection keyword. The card corpus is
   read from `.cards` by default (`-cards`), and kgen errors on a card name the compiled corpus lacks.

`-census-std` moves the question from one pool to a whole set list. It reports per-set acceptance, the
refusal classes ranked by how many Standard-set cards they block, per-set and joint class orders, and
optional TSV rows. A **ratchet** here is a pinned minimum acceptance count: it fails if acceptance falls
and must be re-pinned when it rises (`dirac/cardset/setlists/README.md`):

```sh
$ go run ./dirac/cmd/kgen -census-std -sets FDN
# excerpt: the per-set table starts with BIG, BLB, ... and reaches FDN
## Per set (lower+build, the ratchet basis)

| set | std | accepted | listed | % | short of 90 % |
|---|---|---:|---:|---:|---:|
| BIG | non-std | 23 | 30 | 76.7 | 4 |
...
| FDN | std | 505 | 517 | 97.7 | 0 |
...
## Class ranking (Standard sets only, tokens unpacked)

| rank | class | tier | std cards | std sole | sets (cards) | corpus cards |
| 1 | other:mana "_" | G | 1 | 1 | FDN 1 | 86 |
...
## Per set: classes to 90 % accepted (card completion)
...
## Bundles
```

`-sets` picks the codes treated as “Standard” for the ranking (default: `dirac/cardset/standard.txt`);
`-tsv FILE` writes one row per refusal. Counts here are from 2026-10-30 and a local `.cards`; they move
with the corpus, so rerun the command rather than quoting this paragraph.

## The pipeline

```
Forge script (.cards/cardsfolder/*.txt)
        │  gorge: make fetch-cards; make compile-cards
        ▼
gorge card IR (engine/cards: Card, Face, SA)
        │  dirac/kgen: lower_*.go (IR → LIR) and build.go (LIR → kop)
        │  input: dirac/cardset/pools/<pool>/{cards.txt,ids.tsv}
        ▼
binary tables, gitignored, under the corpus: .cards/kernel-tables/<key>/
  manifest.tsv  <pool>.ktab  <pool>.coverage.md
        │  loaded on first use
        ▼
cardset.ByName("fdn") → a game (dirac.NewConfig)
```

1. **Forge script.** Forge's card scripts are GPL-3.0 and never committed. `make fetch-cards` fetches the
   cardsfolder at the Makefile's `FORGE_REF`; `make compile-cards` compiles it into the IR cache in `.cards`
   (or `$GORGE_CARDS`). `.cards` is gitignored.
2. **gorge IR.** kgen opens a subset of the compiled corpus through `cards.OpenCorpusSubset` in
   `engine/cards`. A `Card` has `Faces`; each `Face` carries `Abilities`, `Triggers`, `Statics`, `Repls` and
   `SVars`, and each ability is an `SA` tree with `Params map[string]string`.
3. **Lowering.** `dirac/kgen/lower*.go` walks the IR and turns it into an internal LIR. Anything it does
   not understand becomes a `refusal{Card, Node, Reason}` instead of a guess. A card with any refusal is
   unsupported; the reason string is what `coverage.md` prints.
4. **Build and encode.** `dirac/kgen/build.go` turns the LIR into `kop.CardDef`, `kop.Ability` and
   `[]kop.Instr` programs; values that kop cannot carry refuse here too. `dirac/kgen/tables.go` renders
   each pool as a `cardset.PoolData` and encodes it (`dirac/cardset/ktab.go`) with its coverage report.
5. **The table.** `PoolData` holds the name table, `Supported`, `CreatureTypes`, `Subtypes`, the per-card
   `Defs`, and the `Filters`/`Exprs`/`Conds`/`Overrides`/`Emblems` tables. Its `Set`
   (`dirac/cardset/pooldata.go`) is the pool API (`Lookup`, `Def`, `Pool`, `PoolID`, `InertID`, `Inert`,
   `AbilityOrigin`, `Matchup`) and builds a `kop.Table` for two decks: only the cards those decks need,
   plus their tokens.
6. **A game.** `dirac.NewConfig(tbl, deckA, deckB, seed, cfg)` takes that `kop.Table`; dirac never
   reads the tables or gorge. `dirac/cardset` is the engine-neutral seam: `Set` exposes the pool, `ByName`
   resolves one.

## Where tables live

`make dirac-tables` writes `<corpus>/kernel-tables/<key>/`. The key is 16 hex digits of the corpus's
`cards.lock` digest and 16 of a hash of the sources that decide a table's bytes, which the binary embeds
(`tablesrc.go`: kgen, `kop`, `cardset`, the pool lists and the gorge packages kgen builds against). So a binary knows which tables
match it without a source tree, the bytes under a key never change, and worktrees sharing a corpus share
tables. Publishing keeps the 64 newest key directories.

`cardset.TablesDir` looks in `$DIRAC_TABLES/<key>`, then `$GORGE_CARDS/kernel-tables/<key>`, then
`<first .cards walking up from the cwd>/kernel-tables/<key>`. A deployed binary sets
`DIRAC_TABLES` to a copy of a `kernel-tables` directory. With a corpus but no tables for this binary,
`ByName` returns `ErrNoTables`, whose text names `make dirac-tables`. With no corpus at all it returns
`ErrNoCorpus`, and tests skip: a fresh clone builds and vets.
`TestTablesUpToDate` rebuilds every pool and compares it with the published directory, byte for byte.

## kgen modes and flags

The header of `dirac/kgen/main.go` names the main modes; `go run ./dirac/cmd/kgen -h` prints every flag. Verified
on 2026-10-30:

| flag | default | meaning |
|---|---|---|
| `-tables` | off | write every pool's table under the corpus; exit 3 means an `ids.tsv` was rewritten, so rebuild kgen and rerun (`make dirac-tables` does) |
| `-synth-pools` | off | write the synthetic evidence pools' tables under the corpus |
| `-supported POOL` | — | print `name<TAB>true\|false` for every entry of the pool's table, tokens included |
| `-show POOL` | — | print the `kop.CardDef` literal of the card or token named by the argument |
| `-ops POOL` | — | write the pool's `OPS.md` to `-out` |
| `-pool NAME` | `fdn` | the name of an ad-hoc pool |
| `-decks DIR` | — | deck-pool input: every `*.json` deck in DIR |
| `-card-list FILE` | — | card-list input (one name per line) instead of `-decks` |
| `-cards DIR` | `.cards` | compiled card corpus |
| `-out DIR` | — | output directory of an ad-hoc pool; with `-ops`, the output file |
| `-dump DIR` | — | write per-card compiled IR to DIR instead of generating (never commit it) |
| `-census` | off | print the (API/trigger/static/replacement, parameter) census |
| `-lir` | off | print every card's lowered IR and refusals |
| `-whatif LIST` | — | with `-lir`, additionally refuse these cards and report coverage |
| `-layer-census` | off | print the interim layer guard's reach |
| `-census-std` | off | print refusal classes ranked by Standard-set carry |
| `-sets LIST` | `standard.txt` | with `-census-std`, comma-separated set codes |
| `-tsv FILE` | — | with `-census-std`, write a per-refusal TSV |

`-card-list` and `-decks` choose the cards of an ad-hoc pool, which `-out` writes outside the tables
directory. `-dump` is the raw-IR study aid; `-census` is the data source for
[`dirac/docs/kgen-params.md`](kgen-params.md); `-lir` is the lowering/debug view; `-census-std` is the
set-wide acceptance view above. A build preserves `ids.tsv`: a CardID never moves, a new card takes
the next free ID, and a card that leaves the pool keeps an inert placeholder.

An ad-hoc pool and a dump, run on 2026-10-30:

```sh
$ go run ./dirac/cmd/kgen -pool kdocsample -card-list /tmp/cards.txt -out /tmp/kdocgen
kgen: 1 of 2 cards supported
$ ls /tmp/kdocgen
ids.tsv
kdocsample.coverage.md
kdocsample.ktab
$ go run ./dirac/cmd/kgen -card-list /tmp/cards.txt -dump /tmp/kdocdump
$ ls /tmp/kdocdump
abrade.txt
sword_of_fire_and_ice.txt
```

## What a pool consists of

Committed, under `dirac/cardset/pools/<pool>/`:

| file | contents |
|---|---|
| `cards.txt` | the pool's cards, one name per line; `#` starts a comment, and a list's opening comment says what the pool is for |
| `ids.tsv` | the CardID index: `<id>\t<card\|token>\t<name>`; append-only, an ID never moves |

Generated, in the key directory:

| file | contents |
|---|---|
| `<pool>.ktab` | the binary table, one `cardset.PoolData`: static records plus `Abilities`, each with targets, modes, cost/trigger/static/replacement and a `Prog []kop.Instr`; token `kop.CardDef`s, addressed by pool index from token-making instructions; the name table and pool-wide tables; and the layer guard's per-card classification and conflict list |
| `<pool>.coverage.md` | per-pool acceptance: cards supported, every refusal `(card, IR node, reason)`, refusals by reason, and by class/tier |
| `manifest.tsv` | one line per pool: name, card count, token count, sha256 of the table's payload |

`OPS.md`, the interpreter contract for every op, ext, counter, field and predicate a pool uses, is on demand:
`go run ./dirac/cmd/kgen -ops <pool> -out <file>`.

`go run ./dirac/cmd/kgen -show fdn Abrade` prints a compact example. The static record carries the mana cost,
type and colour; the one spell ability has two charm modes, each with its own target slot, and the
program is four instructions:

```go
kop.CardDef{
	Cost:   kop.ManaCost{Generic: 1, Pips: [6]uint8{0, 0, 0, 1}},
	Types:  kop.TInstant,
	Colors: kop.ColR,
	Timing: kop.TimingInstant,
	Abilities: []kop.Ability{{
		Targets: [kop.MaxTargets]kop.TargetSpec{ /* creature, then artifact/enchantment */ },
		NModes: 2, ModesPick: 1,
		Prog: []kop.Instr{
			{Op: kop.OpCharm, A: kop.Const(1)},
			{Op: kop.OpDealDamage, Obj: kop.RefTarget0, A: kop.Const(3)},
			{Op: kop.OpCharm, A: kop.Const(1), C: 1},
			{Op: kop.OpDestroy, Obj: kop.RefTarget1},
		},
	}},
}
```

`Matchup` copies the needed `CardDef`s into a `Table` and renumbers their `CardID`s; `Pool()` builds a table
over the whole pool, keeping `PoolID(i)` stable so a CardID in one Pool table means the same card in
another.

## Which pools matter

`dirac/cardset/pools` holds three set pools (`fdn`, `blb`, `pauper`) and many small pools under `fixtures/`.
They are all card lists:

- **Set pools.** `fdn` is the union of the cards of its deck pool, not every name in
  `dirac/cardset/setlists/FDN.txt`. That is why `fdn.coverage.md` can report every card of the pool while
  `-census-std` reports fewer for the whole FDN list. `blb` and `pauper` are the other set pools.
- **Fixture pools.** Small, purpose-built pools: a mechanic or parameter family being brought up, a
  constructed lockstep scenario, or a batch acceptance slice. Examples: `bargain`, `warp`,
  `mayplaykind`, `enterstapped`. A newcomer can ignore them unless working on that mechanic.

`cardset.ByName` and `cardset.Names` serve every pool in this binary's manifest; nothing needs linking.

`dirac/cardset/setlists/*.txt` are not pools. They are the acceptance lists `dirac/kgen/ratchet_test.go`
lowers and pins, and the lists `-census-std` ranks. `dirac/cardset/standard.txt` is the default set-code
list for `-census-std`.

## Adding support for a new card or mechanic

Most support work is a lowering, not a new engine primitive. The shape of the work:

1. **Reproduce the refusal.** Put the card in a one-name list and run
   `go run ./dirac/cmd/kgen -card-list /tmp/card.txt -lir`; read the refusal node. Then dump the input with
   `-dump` and look at that `A[n]`/`T[n]`/`S[n]`/`R[n]` and its parameters. The coverage report's reason and
   class/tier are the short version; `-dump` is the full one.
2. **Lower it.** Add or extend a `dirac/kgen/lower_<name>.go` for the IR parameter/node, and a `build.go`
   arm if the LIR shape needs a new kop value. Follow the existing files: a refusal is removed only when
   the lowering handles the real parameter, not by accepting it silently. Descriptive Forge parameters
   (prompts, AI hints, descriptions) are ignored by construction, as listed at the top of
   [`dirac/docs/kgen-params.md`](kgen-params.md).
3. **Extend kop only if needed.** If kop cannot express the effect, add an op or an `ExtOp` sub-op in
   `dirac/kop` and implement it in dirac. [`dirac/README.md`](../README.md) is the
   registration table: append-only ordinals, the name-registry tests, the `execExt` case, the new
   `opsdoc_<name>.go` row, and where a new stall reason goes. Do not renumber. A **stall** is a guard
   that stops the game when dirac cannot represent an ordering or a choice; it is a deliberate
   failure, not a silent wrong answer (see [`GORGE-FINDINGS.md`](GORGE-FINDINGS.md)).
4. **Rebuild the tables.** `make dirac-tables` rebuilds every pool and extends each `ids.tsv`; commit a
   changed `ids.tsv`. For a new pool, create `dirac/cardset/pools/fixtures/<pool>/cards.txt` first, opening with a
   comment that says what it is for. If the lowering changes acceptance counts in the set-list ratchets, `KGEN_REPIN=1`
   rewrites the pins in `dirac/kgen/ratchet_test.go` (see
   [`verification.md`](verification.md#following-a-gorge-change)).
5. **Prove it against gorge.** The generated table is dirac's claim; gorge is the arbiter. The
   lockstep harness in `dirac/oracle` and its `kfuzz` lockstep fuzz driver compare the same game action
   by action. The commands, the census and the expected baseline live in
   [`dirac/docs/verification.md`](verification.md); do not invent a new measurement here.

Two failure modes are easy to confuse. A **lowering** refusal means kgen cannot represent the card, so
it never enters a playable table. A supported card can still hit a **dirac** stall if the interpreter
meets a state it cannot order or choose; that is an engine fix, not a kgen fix, and the stall name in
`GORGE-FINDINGS.md` is the pointer.

## kop and OPS.md, at the level a card author needs

`kop` is `dirac/kop`: the engine-neutral vocabulary the generated data targets. The important types are
in `dirac/kop` (`table.go` for the card/table shapes, `ops.go` and `ext.go` for the op enums):

- `CardDef` — printed cost, types, subtypes, keywords, colours, P/T/loyalty, timing, and the card's
  `Abilities`.
- `Ability` — one spell/activated/triggered/static/replacement ability: targets, modes, cost, trigger or
  static/replacement data, and `Prog`.
- `Instr` — one instruction: `Op`, an `Obj` ref, two value operands `A`/`B`, an integer `C`, and a
  filter `F`. `OpExt` plus an `ExtOp` sub-op is the extension space when the op enum is full.
- `Table` — the matchup's whole card universe plus side tables (`Filters`, `Exprs`, `Conds`,
  `Overrides`, `Subtypes`, …).

The interpreter contract is in [`dirac/kop/doc.go`](../kop/doc.go): programs run front to
back; an illegal target makes that instruction a no-op and an all-illegal targeted ability fizzles
(CR 608.2b); branch ops are structured; decision ops suspend and resume at the same instruction; an
unimplemented op panics rather than silently doing nothing. Read it before writing a new op.

A pool's `OPS.md` (`kgen -ops`) is the pool-local reference for what those instructions mean. A row
names the op or extension (and its operand layout), gives the gorge rule/parameter it mirrors, then
lists the cards that use it. The prose is written in `dirac/kgen/opsdoc.go` and the per-pool
usage is computed from the built table, so the file cannot drift from the pool. Use it to answer
“what does this op do for these cards?”; use `dirac/README.md` to answer “where do I register a new
one?”.

## Related pages

- [`dirac/docs/README.md`](README.md) — the documentation index.
- [`dirac/docs/kgen-params.md`](kgen-params.md) — the per-API/parameter census and lowering table.
- [`dirac/README.md`](../README.md) — where to register a new op, stall or ext sub-op.
- [`dirac/docs/verification.md`](verification.md) — lockstep, `kfuzz`, parity and following a gorge change.
- [`dirac/docs/running.md`](running.md) — playing games on dirac.
- [`DEVIATIONS.md`](DEVIATIONS.md) and [`GORGE-FINDINGS.md`](GORGE-FINDINGS.md) — the declared
  differences from gorge, and the gorge behaviour dirac copies.
