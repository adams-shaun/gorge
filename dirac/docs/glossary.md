# Kernel glossary

Terms you meet reading the kernel's code and docs.

## Checking against gorge

**Lockstep.** `dirac/oracle` plays the same decks and seed on gorge and on the kernel. It compares the
option list at every decision and the state projection after every committed action. The first
difference is a **divergence**. A game with no divergence and no unmodelled stall (below) is **clean**
(`oracle.Result.Clean`). See [verification.md](verification.md).

**Stall.** The kernel stops a game it cannot continue correctly. The game ends as a draw,
`Terminal()` returns `(true, -1)` and `StallReason()` names the reason. Each reason has a class, read
with `dirac.StallClassOf` and defined in `dirac/stallclass.go`:

- **Capacity**: a fixed array or budget ran out, and gorge has no such limit. The game was correct up
  to there. These are declared limits: always counted, never a failure.
- **Unmodelled**: gorge does something at this point that the kernel does not. These are rule gaps, and
  they fail a `kfuzz -absolute` run. A reason nobody classed counts as unmodelled.

## Cards

**Lowered.** Lowering is `dirac/kgen` turning a card's compiled script into table data. A card kgen
cannot represent, at lowering or at table build, does not lower, and the reason is recorded in its
pool's `coverage.md`. It stays in the table as an inert stand-in, and `Matchup` refuses a deck holding it.

**Refused.** A card that lowers and must not be played, because it is known to diverge. It has a file
in `dirac/support/refused/`, written by `kgen -refuse`, naming a game that shows it.
`TestRefusedEntriesStillDiverge` fails an entry once no game implicates the card. A refused card still
counts as "accepted" in the `dirac/kgen` ratchets, which count lowering, and `Matchup` refuses a deck
holding it.

**Evidenced.** Played in clean lockstep games: at least `oracle.EvidenceMinPlays` (3) casts and land
plays, with every one of the card's own ability keys used. `kfuzz -evidence` writes these cards, and
the lists in `dirac/support/evidence/` hold them. `kfuzz -reach` says where any other card of a
synthetic pool stopped.

**Supported.** Lowered, not refused, and evidenced: `Set.Status(i) == cardset.StatusSupported`, computed
by kgen when it writes a table. A lowered card with no evidence is **unverified**, and `Matchup` refuses a
deck holding it, as it does a refused card, unless a caller asked to play it
(`cardset.AllowUnverified`, `kfuzz -allow-unverified`).
`Set.Supported()` and `kgen -supported <pool>` report what a game may play. [support.md](support.md) has
the counts.

**Raw table.** A set that plays every lowered card, refused and unverified alike (`cardset.RawByName`).
Lockstep fixtures and refused entries' repros play these. Nothing else may read
them (`TestOnlyTheOracleReadsRawTables`).

**Pool.** Either of two lists:

- A **card pool** is the card list in `dirac/cardset/pools/<pool>/cards.txt`, with stable card IDs in
  `ids.tsv`.
- A **deck pool** is the list of decks in a `pool.tsv` that lockstep draws its pairs from, kept outside
  the repository and linked as `.pools` or named by `$GORGE_POOLS`. A clone without one plays the committed
  slices in `dirac/oracle/testdata/slices`: the first 1500 pairs of each pool at seed 1.

**Table.** kgen's binary output for one card pool, `<pool>.ktab`, with `<pool>.coverage.md` beside it.
`make dirac-tables` writes them under `<corpus>/kernel-tables/<key>/`, where the key comes from the
corpus and kgen's sources. Tables are not committed.
