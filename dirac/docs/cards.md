# How cards get into dirac

dirac does not read Forge scripts at run time. `dirac/kgen` compiles each supported card once into a binary table, and
dirac plays that data through the vocabulary in `dirac/kop` (**kop**, dirac's op and instruction package). gorge leads
and dirac follows: `dirac/oracle` runs them in **lockstep** (same game, same seed, every decision compared; see
[verification](verification.md)). This page covers that path, how to tell whether a card is playable and what adding
support costs.

Related: [`dirac/README.md`](../README.md) and [`dirac/doc.go`](../doc.go) for the API,
[`kgen-params.md`](kgen-params.md) for per-parameter lowering (generated).

## Is card X supported?

Tables are not committed, so build them first (`make dirac-tables`). Then, per pool:

```sh
make dirac-tables
go run ./dirac/cmd/kgen -supported fdn | grep Abrade     # name<TAB>true|false
go run ./dirac/cmd/kgen -show fdn Abrade                 # the card's kop.CardDef
```

1. **Find the pool.** `grep -rlx --include=cards.txt 'Card Name' dirac/cardset/pools`. A card can be in several pools.
   In none: it was never lowered into a pool; use step 4.
2. **Check the refusal table.** `grep -n '^| Card Name ' <tables>/<pool>.coverage.md` (`<tables>` is the directory
   `make dirac-tables` prints). A row is `| card | IR node | reason |`; no row means supported. In code,
   `Set.Supported()[poolIndex]` gives the same answer, `Set.Lookup(name)` the index and `Set.PoolID(index)` the stable
   `CardID` in `dirac/cardset/pools/<pool>/ids.tsv`.
3. **Read the reason.** The node column says where lowering stopped: `A[0]` the first ability, `T[1]` the second trigger,
   `S[0]` a static, `R[0]` a replacement, `emit static[0]` the emit step, `ab[0]` an origin link. The report also groups
   refusals by reason and by class and tier (`dirac/kgen/classes.go`: G generator, K keyword, O op or predicate,
   M mechanism, `?` unclassified).
4. **See what kgen saw.** For a card in no pool, put its name in a file and run
   `go run ./dirac/cmd/kgen -card-list FILE -lir`: it prints each card's lowered IR and refusals, then the coverage
   report. `-dump DIR` writes the raw compiled IR instead.

A refused card stays in its pool as an inert stand-in (cost, types, P/T and no abilities) so a search can deal it
from a pool, but `Matchup` refuses a deck holding it.

`-census-std` asks the question of a whole set list: per-set acceptance and the refusal classes ranked by how many
Standard cards each blocks. A **ratchet** is a pinned minimum acceptance count: it fails when acceptance falls and is
re-pinned when it rises (`dirac/cardset/setlists/README.md`). Counts move with the corpus, so rerun
`go run ./dirac/cmd/kgen -census-std -sets FDN` rather than quote them.

## The pipeline

```text
Forge script (.cards/cardsfolder/*.txt)
   | make fetch-cards; make compile-cards
   v
gorge card IR (engine/cards: Card, Face, SA)
   | dirac/kgen: lower*.go (IR -> LIR), build.go (LIR -> kop)
   |   input: dirac/cardset/pools/<pool>/{cards.txt,ids.tsv}
   v
binary tables, gitignored: .cards/kernel-tables/<key>/{manifest.tsv,<pool>.ktab,<pool>.coverage.md}
   | loaded on first use
   v
cardset.ByName("fdn") -> a game (dirac.NewConfig)
```

1. **Script.** GPL-3.0, never committed. `make fetch-cards` fetches the cardsfolder at the Makefile's `FORGE_REF`;
   `make compile-cards` compiles it into the IR cache under `.cards` (or `$GORGE_CARDS`).
2. **IR.** kgen opens a subset of the corpus with `cards.OpenCorpusSubset` (`engine/cards`). A `Card` has `Faces`; a
   `Face` carries `Abilities`, `Triggers`, `Statics`, `Repls` and `SVars`; an ability is an `SA` tree of `Params`.
3. **Lowering.** `dirac/kgen/lower*.go` turns the IR into an internal LIR. Anything it does not understand becomes a
   `refusal{Card, Node, Reason}`, never a guess; a card with any refusal is unsupported.
4. **Build and encode.** `build.go` turns the LIR into `kop.CardDef`, `kop.Ability` and `[]kop.Instr`; values kop
   cannot carry refuse here too. `tables.go` renders each pool as a `cardset.PoolData` and encodes it
   (`dirac/cardset/ktab.go`) with its coverage report.
5. **Table.** `PoolData` holds the name table, `Supported`, creature types, subtypes, per-card `Defs` and the
   `Filters`/`Exprs`/`Conds`/`Overrides`/`Emblems` tables. Its `Set` (`dirac/cardset/pooldata.go`) is the pool API
   (`Lookup`, `Def`, `Pool`, `PoolID`, `InertID`, `Inert`, `AbilityOrigin`, `Matchup`). `Matchup` builds a `kop.Table`
   of only the cards the decks need, plus their tokens, renumbering `CardID`s; `Pool()` builds one over the whole pool
   with `PoolID(i)` stable.
6. **Game.** `dirac.NewConfig(tbl, deckA, deckB, seed, cfg)` takes that `kop.Table`. dirac never reads the tables or
   gorge; `dirac/cardset` is the engine-neutral seam.

## Where tables live

`make dirac-tables` writes `<corpus>/kernel-tables/<key>/`, and the synthetic evidence pools under
`<corpus>/kernel-synth-tables/<key>/`. The key is 16 hex digits of the corpus's `cards.lock` digest and 16 of
`gorge.TableSourceHash` (`tablesrc.go` at the module root: the sources that decide a table's bytes, which the binary
embeds). A binary therefore knows which tables match it without a source tree, the bytes under a key never change, and
worktrees sharing a corpus share tables. Publishing keeps the newest `keepKeyDirs` key directories
(`dirac/kgen/tables.go`).

`cardset.TablesDir` looks in `$DIRAC_TABLES/<key>`, then `$GORGE_CARDS/kernel-tables/<key>`, then the first `.cards`
walking up from the cwd. A deployed binary sets `DIRAC_TABLES` to a copy of a `kernel-tables` directory and needs no
corpus. A corpus with no tables for this binary gives `ErrNoTables` (naming `make dirac-tables`); no corpus gives
`ErrNoCorpus`, and tests skip, so a fresh clone builds and vets. `TestTablesUpToDate` rebuilds every pool and compares
it byte for byte with the published directory.

## kgen modes and flags

`go run ./dirac/cmd/kgen -h` prints every flag. The main ones:

| Flag | Meaning |
|---|---|
| `-tables` | write every pool's table under the corpus; exit 3 means an `ids.tsv` was rewritten, so rebuild kgen and rerun (`make dirac-tables` does) |
| `-synth-pools` | write the synthetic evidence pools' tables under the corpus |
| `-supported POOL` | print `name<TAB>true\|false` per entry; `-unverified` counts cards without evidence as playable, `-four-seat` makes a pod-limited card false |
| `-show POOL CARD` | print the `kop.CardDef` literal of a card or token |
| `-ops POOL` | write the pool's `OPS.md` to `-out` |
| `-pool NAME` (default `fdn`), `-decks DIR`, `-card-list FILE`, `-cards DIR` (default `.cards`), `-out DIR` | an ad-hoc pool: its cards from `*.json` decks or a name list, written outside the tables directory |
| `-dump DIR` | write per-card compiled IR (never commit it) |
| `-census` | the (API/trigger/static/replacement, parameter) census behind [`kgen-params.md`](kgen-params.md) |
| `-lir`, `-whatif LIST`, `-lir-keys` | lowered IR and refusals; extra refusals; each card's evidence key |
| `-layer-census` | the layer guard's reach |
| `-census-std`, `-sets LIST`, `-all-blockers`, `-tsv FILE` | refusal classes ranked by Standard-set carry, over `-sets` (default `dirac/cardset/standard.txt`); every blocker of a card; a per-refusal TSV |
| `-refuse CARD -why TEXT -class C -repro POOL:SURFACE:PAIR[:POLICY[:MULLIGANS]]`, `-unrefuse CARD` | write or delete a `dirac/support/refused` entry |

A build preserves `ids.tsv`: a `CardID` never moves, a new card takes the next free ID, and a card that leaves the pool
keeps an inert placeholder.

## What a pool consists of

Committed, in `dirac/cardset/pools/<pool>/`: `cards.txt` (one name per line, `#` comments; the opening comment says what
the pool is for) and `ids.tsv` (`<id>\t<card|token>\t<name>`, append-only).

Generated, in the key directory:

| File | Contents |
|---|---|
| `<pool>.ktab` | the binary table, one `cardset.PoolData`: static records, abilities with their `Prog []kop.Instr`, token defs, the name table, pool-wide tables and the layer guard's per-card classification |
| `<pool>.coverage.md` | cards supported, every refusal `(card, IR node, reason)`, refusals by reason and by class and tier |
| `manifest.tsv` | one line per pool: name, card count, token count, sha256 of the payload |

`OPS.md`, the interpreter contract for every op, ext, counter, field and predicate a pool uses, is on demand:
`go run ./dirac/cmd/kgen -ops <pool> -out <file>`. Its prose is in `dirac/kgen/opsdoc.go` and the per-pool usage is
computed from the built table, so it cannot drift.

## Which pools matter

`dirac/cardset/pools` holds the set and deck pools and, under `fixtures/`, many small pools.
`cardset.ByName` and `cardset.Names` serve every pool in this binary's manifest.

- **Deck and set pools** (`fdn`, `blb`, `pauper`, `mbc`, `cmd5`, `edh`, `standard-meta`, ...): `fdn` is the union of the cards of
  its deck pool, not every name in `dirac/cardset/setlists/FDN.txt`, which is why `fdn.coverage.md` can report every card
  of the pool while `-census-std` reports fewer for the whole FDN list.
- **Fixture pools:** small, purpose-built pools for one mechanic or parameter family, a lockstep scenario or an
  acceptance slice. Ignore them unless working on that mechanic.
- `dirac/cardset/setlists/*.txt` are not pools: they are the acceptance lists `dirac/kgen/ratchet_test.go` lowers and
  pins, and the lists `-census-std` ranks.

## Adding support for a card or mechanic

Most work is a lowering, not a new engine primitive.

1. **Reproduce the refusal.** One-name list, `go run ./dirac/cmd/kgen -card-list FILE -lir`; then `-dump` and read the
   `A[n]`/`T[n]`/`S[n]`/`R[n]` node and its parameters.
2. **Lower it.** Add or extend `dirac/kgen/lower_<name>.go`, and a `build.go` arm if the LIR needs a new kop value. A
   refusal is removed only when the lowering handles the real parameter, never by accepting it silently. Descriptive
   Forge parameters (prompts, AI hints) are ignored by construction, as listed at the top of
   [`kgen-params.md`](kgen-params.md).
3. **Extend kop only if needed.** Add an op or `ExtOp` sub-op in `dirac/kop` and implement it in dirac, following the
   registration rules in [`dirac/README.md`](../README.md#registering-an-op-stall-or-ext-sub-op-contributors). A **stall**
   is a guard that stops the game when dirac cannot represent an ordering or choice: a deliberate failure, not a silent
   wrong answer ([`GORGE-FINDINGS.md`](GORGE-FINDINGS.md)).
4. **Rebuild the tables.** `make dirac-tables` extends each `ids.tsv`; commit a changed one. A new pool starts as
   `dirac/cardset/pools/fixtures/<pool>/cards.txt` with an opening comment. If acceptance counts change, `KGEN_REPIN=1`
   rewrites the pins in `dirac/kgen/testdata/acceptance.tsv` and `build_acceptance.tsv` (see
   [verification](verification.md#following-a-gorge-change)).
5. **Prove it against gorge** with `dirac/oracle` and `kfuzz` ([verification](verification.md)).

A **lowering** refusal means kgen cannot represent the card, so it never enters a playable table. A supported card can
still hit a **dirac** stall when the interpreter meets a state it cannot order or choose; that is an engine fix, and the
stall name in [`GORGE-FINDINGS.md`](GORGE-FINDINGS.md) is the pointer.

## kop at the level a card author needs

`dirac/kop` is the engine-neutral vocabulary the tables target (`table.go` for shapes, `ops.go` and `ext.go` for the
enums):

- `CardDef`: printed cost, types, subtypes, keywords, colours, P/T/loyalty, timing and `Abilities`.
- `Ability`: one spell, activated, triggered, static or replacement ability: targets, modes, cost, trigger or static data
  and `Prog`.
- `Instr`: `Op`, an `Obj` ref, value operands `A`/`B`, an integer `C` and a filter `F`. `OpExt` plus an `ExtOp`
  sub-op is the extension space.
- `Table`: the matchup's card universe plus side tables (`Filters`, `Exprs`, `Conds`, `Overrides`, `Subtypes`, ...).

The interpreter contract is in [`dirac/kop/doc.go`](../kop/doc.go): programs run front to back; an illegal target makes
that instruction a no-op and an all-illegal targeted ability fizzles (CR 608.2b); branch ops are structured; decision
ops suspend and resume at the same instruction; an unimplemented op panics. Read it before writing a new op.

## Related pages

[`README.md`](README.md) (index), [`kgen-params.md`](kgen-params.md), [`dirac/README.md`](../README.md),
[`verification.md`](verification.md), [`running.md`](running.md), [`DEVIATIONS.md`](DEVIATIONS.md) and
[`GORGE-FINDINGS.md`](GORGE-FINDINGS.md).
