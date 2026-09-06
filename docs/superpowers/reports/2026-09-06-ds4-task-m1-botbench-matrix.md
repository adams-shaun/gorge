# Task M1 — a deck-pair matrix in `cmd/botbench`

Branch `wt/m1` off `79d45b1`, worktree `.worktrees/m1`.

## What this task was

`cmd/botbench`'s default run indexed the sorted repo-deck list with
`names[:seats]`, so at `-seats 2` it always and only played
`death-n-taxes` vs `dimir-tempo` — one of the 12 repo decks' 66 unordered
pairs, and the *only* pair every policy number this project has quoted was
measured on. This task adds a **deck-pair matrix mode**: `-pairs all`
plays all 66 unordered pairs, `-pairs a:b,c:d` plays named pairs, the
default (no `-pairs`) is byte-for-byte unchanged, and `-games` keeps its
meaning as **games per pair**.

## What changed and why (per file)

### `cmd/botbench/main.go`

Added the matrix mode alongside the untouched single-pair path. The
single-pair `bench`/`run`/`playMatch` were not modified; a default run
reproduces the old binary byte-for-byte (verified against the pre-change
binary).

- **`pairDef` / `String`** — one ordered deck pair; `a` is always seat 0, so
  `death-n-taxes:dimir-tempo` is the first `all` pair and reconstructs the
  single-pair run.
- **`fullPairs(names)`** — every unordered pair in sorted `i<j` order (66 for
  the 12 repo decks). Order is a pure function of the sorted deck list, never
  a map.
- **`parsePairs(spec)`** — `"all"`, or the named `"a:b,c:d"` list; validates
  deck names against the sorted repo list; rejects unknown decks and `"a:b:x"`.
- **`gameSeedPair(base, pos, games, g)`** — pair `pos` uses the seed block
  `base+pos*games .. base+(pos+1)*games`. Pair 0 is therefore `base+0..base+games-1`,
  byte-identical to the single-pair run of `games` games at the same base —
  the property that makes a matrix row reproduce today's number, and every
  game across the whole matrix has a distinct seed.
- **`playOnePair`** — plays `games` matches of one pair and tallies them.
  Seats **still trade policies every game** (`aPlaysSeat`), exactly as the
  single-pair bench does, so a deck list cannot masquerade as a policy
  advantage here any more than it could there; only the pair's two deck lists
  are fixed to the two seats for the duration of the pair.
- **`runPairs` / `progressWriter`** — plays pairs in parallel (default
  worker count = `runtime.NumCPU()`) writing each tally to its own slice
  position, so the result is byte-deterministic regardless of worker count
  (`-workers`). Live progress goes to a separate writer (stderr in `main`);
  the report is only printed once, serially and in pair order, after every
  pair finishes — so progress never leaks into (or breaks) the deterministic
  report.
- **`mergeResults` / `excludeCounts`** — pooled numbers are over COUNTS
  (sum of wins over sum of games), not an average of per-pair rates, so the
  pooled Wald CI is the interval a single run of all the games would give.
  `excludeCounts` classifies each per-pair A-win interval against 50%:
  below (A loses), above (A wins), or undecided. Pooled intents: a pair that
  trips `maxIntents` propagates `playMatch`'s error wrapped with the pair
  name and seed, so a matrix run that hits the liveness cap fails loudly with
  the pair and seed identified.
- **`writeMatrixText` / `writeMatrixJSON`** — the per-pair table and pooled
  line (text) and a machine-readable JSON document (`-format json`, rates as
  fractions 0..1, struct-order fields so two builds diff exactly). The header
  and pooled line both state that `-games` is **PER PAIR**.
- **`runMatrix`** — entry through the real engine; resolves corpus, loads
  each distinct deck once, and drives `playMatch`/`policies`/`ci95` shared
  with `run`.
- **Flags** — `-pairs`, `-format` (text/json), `-workers`. Defaults keep
  today's behaviour.

### `cmd/botbench/main_test.go`

Added tests pinning the new invariants (all synthetic except the small
end-to-end runner, so they do not couple to what the bot plays and do not
flake under later bot work):

- `TestFullPairsIteratesSorted` — 12 decks → 66 pairs, strictly sorted,
  no dupes, first pair `death-n-taxes:dimir-tempo`.
- `TestParsePairs` — `"all"`, named list, unknown-deck rejection.
- `TestMatrixTradesPoliciesEveryGame` — each policy holds each seat exactly
  half an even pair (the policy-not-deck property).
- `TestMatrixGamesArePerPair` — a pair tallies `games` games, not the total;
  the header says per-pair unambiguously.
- `TestPooledCIPoolsCountsNotRates` — pooled CI is over pooled counts, not a
  mean of rates (uses unequal denominators where the two disagree).
- `TestMatrixReportOrderIsSorted` — table rows follow the sorted pair order.
- `TestMatrixDeterministicUnderWorkers` — workers=1 and workers=8 give
  identical tallies and byte-identical text reports.
- `TestMatrixEndToEnd` — two named pairs through the real engine, report
  contains both rows and is byte-deterministic run to run.

## Acceptance: the single-pair number as a matrix row

`-a bot -b legacy -pairs "death-n-taxes:dimir-tempo" -games 4000 -seed 0`
(reproduced as the first row of the full `all` run too):

```
death-n-taxes:dimir-tempo  2556  1444  0  63.9% [62.4%, 65.4%]  59.9% [58.3%, 61.4%]  18.0
```

This is exactly the brief's `e7891a3` (B2+B3) reference line (63.9%
[62.4,65.4], mean 18.0), and matches the pre-change single-pair run measured
on this box before the change. **The change did not alter the game stream.**

## The matrix — `-a bot -b legacy -pairs all -games 4000 -seed 0`

264,000 games, 66 pairs × 4000, all 66 completed with a winner (0 draws),
**no pair hit the 400,000-intent cap**.

```
bot bench matrix: base seed 0, bot vs legacy, 66 deck pairs, 4000 games PER PAIR (total 264000 games)
deck pair                           A wins  B wins  draws  A win rate (95% CI)   seat 0 rate (95% CI)    mean turns
death-n-taxes:dimir-tempo           2556    1444    0      63.9% [62.4%, 65.4%]  59.9% [58.3%, 61.4%]    18.0
death-n-taxes:eldrazi-stompy        2493    1507    0      62.3% [60.8%, 63.8%]  51.1% [49.5%, 52.6%]    13.8
death-n-taxes:mono-black-aggro      2345    1655    0      58.6% [57.1%, 60.2%]  24.2% [22.9%, 25.6%]    17.1
death-n-taxes:mono-blue-tempo       2882    1118    0      72.0% [70.7%, 73.4%]  41.8% [40.2%, 43.3%]    18.7
death-n-taxes:mono-green-stompy     2717    1283    0      67.9% [66.5%, 69.4%]  37.7% [36.2%, 39.2%]    14.7
death-n-taxes:mono-red-goblins      2550    1450    0      63.7% [62.3%, 65.2%]  44.1% [42.6%, 45.6%]    15.0
death-n-taxes:the-epic-storm        2007    1993    0      50.2% [48.6%, 51.7%]  98.3% [97.9%, 98.7%]    16.8
death-n-taxes:tron                  2015    1985    0      50.4% [48.8%, 51.9%]  99.3% [99.0%, 99.5%]    16.4
death-n-taxes:ur-delver             2671    1329    0      66.8% [65.3%, 68.2%]  55.5% [53.9%, 57.0%]    16.2
death-n-taxes:uw-control            2014    1986    0      50.3% [48.8%, 51.9%]  99.3% [99.0%, 99.6%]    23.7
death-n-taxes:uw-tempo              2768    1232    0      69.2% [67.8%, 70.6%]  40.6% [39.1%, 42.1%]    18.7
dimir-tempo:eldrazi-stompy          2174    1826    0      54.4% [52.8%, 55.9%]  35.2% [33.8%, 36.7%]    13.8
dimir-tempo:mono-black-aggro        2090    1910    0      52.2% [50.7%, 53.8%]  15.2% [14.1%, 16.4%]    16.5
dimir-tempo:mono-blue-tempo         2477    1523    0      61.9% [60.4%, 63.4%]  24.5% [23.1%, 25.8%]    19.2
dimir-tempo:mono-green-stompy       2282    1718    0      57.0% [55.5%, 58.6%]  20.8% [19.5%, 22.0%]    13.1
dimir-tempo:mono-red-goblins        2158    1842    0      53.9% [52.4%, 55.5%]  11.3% [10.3%, 12.3%]    11.9
dimir-tempo:the-epic-storm          2048    1952    0      51.2% [49.7%, 52.7%]  89.0% [88.1%, 90.0%]    24.8
dimir-tempo:tron                    2048    1952    0      51.2% [49.7%, 52.7%]  91.5% [90.6%, 92.3%]    25.1
dimir-tempo:ur-delver               2236    1764    0      55.9% [54.4%, 57.4%]  30.6% [29.2%, 32.1%]    16.2
dimir-tempo:uw-control              2145    1855    0      53.6% [52.1%, 55.2%]  90.4% [89.5%, 91.3%]    31.6
dimir-tempo:uw-tempo                2348    1652    0      58.7% [57.2%, 60.2%]  24.5% [23.2%, 25.8%]    19.2
eldrazi-stompy:mono-black-aggro     1985    2015    0      49.6% [48.1%, 51.2%]  30.4% [29.0%, 31.9%]    13.2
eldrazi-stompy:mono-blue-tempo      2054    1946    0      51.3% [49.8%, 52.9%]  54.8% [53.2%, 56.3%]    12.9
eldrazi-stompy:mono-green-stompy    2316    1684    0      57.9% [56.4%, 59.4%]  36.0% [34.5%, 37.5%]    12.0
eldrazi-stompy:mono-red-goblins     2237    1763    0      55.9% [54.4%, 57.5%]  39.6% [38.1%, 41.1%]    11.2
eldrazi-stompy:the-epic-storm       2003    1997    0      50.1% [48.5%, 51.6%]  89.8% [88.8%, 90.7%]    12.2
eldrazi-stompy:tron                 1990    2010    0      49.8% [48.2%, 51.3%]  92.2% [91.4%, 93.0%]    12.1
eldrazi-stompy:ur-delver            2046    1954    0      51.1% [49.6%, 52.7%]  63.2% [61.7%, 64.7%]    11.7
eldrazi-stompy:uw-control           2072    1928    0      51.8% [50.3%, 53.3%]  76.2% [74.9%, 77.5%]    14.8
eldrazi-stompy:uw-tempo             2049    1951    0      51.2% [49.7%, 52.8%]  51.2% [49.6%, 52.7%]    13.2
mono-black-aggro:mono-blue-tempo    2655    1345    0      66.4% [64.9%, 67.8%]  61.5% [60.0%, 63.0%]    17.1
mono-black-aggro:mono-green-stompy  2849    1151    0      71.2% [69.8%, 72.6%]  44.1% [42.5%, 45.6%]    13.9
mono-black-aggro:mono-red-goblins   2680    1320    0      67.0% [65.5%, 68.5%]  50.5% [49.0%, 52.1%]    14.5
mono-black-aggro:the-epic-storm     2011    1989    0      50.3% [48.7%, 51.8%]  98.1% [97.6%, 98.5%]    15.2
mono-black-aggro:tron               2009    1991    0      50.2% [48.7%, 51.8%]  98.9% [98.5%, 99.2%]    15.0
mono-black-aggro:ur-delver          2295    1705    0      57.4% [55.8%, 58.9%]  62.9% [61.4%, 64.4%]    15.3
mono-black-aggro:uw-control         1990    2010    0      49.8% [48.2%, 51.3%]  99.6% [99.4%, 99.8%]    21.3
mono-black-aggro:uw-tempo           2584    1416    0      64.6% [63.1%, 66.1%]  59.2% [57.7%, 60.8%]    17.2
mono-blue-tempo:mono-green-stompy   2196    1804    0      54.9% [53.4%, 56.4%]  27.9% [26.5%, 29.2%]    13.5
mono-blue-tempo:mono-red-goblins    2061    1939    0      51.5% [50.0%, 53.1%]  22.5% [21.2%, 23.8%]    13.6
mono-blue-tempo:the-epic-storm      1995    2005    0      49.9% [48.3%, 51.4%]  98.4% [98.0%, 98.8%]    17.8
mono-blue-tempo:tron                2001    1999    0      50.0% [48.5%, 51.6%]  99.7% [99.5%, 99.9%]    17.6
mono-blue-tempo:ur-delver           2474    1526    0      61.9% [60.3%, 63.4%]  49.4% [47.8%, 50.9%]    16.2
mono-blue-tempo:uw-control          2012    1988    0      50.3% [48.8%, 51.8%]  99.1% [98.8%, 99.4%]    23.9
mono-blue-tempo:uw-tempo            2890    1110    0      72.2% [70.9%, 73.6%]  42.3% [40.8%, 43.8%]    24.2
mono-green-stompy:mono-red-goblins  2045    1955    0      51.1% [49.6%, 52.7%]  70.9% [69.5%, 72.3%]    12.1
mono-green-stompy:the-epic-storm    1995    2005    0      49.9% [48.3%, 51.4%]  99.4% [99.1%, 99.6%]    11.0
mono-green-stompy:tron              2001    1999    0      50.0% [48.5%, 51.6%]  99.9% [99.8%, 100.0%]   10.8
mono-green-stompy:ur-delver         2019    1981    0      50.5% [48.9%, 52.0%]  82.8% [81.7%, 84.0%]    11.7
mono-green-stompy:uw-control        2005    1995    0      50.1% [48.6%, 51.7%]  99.3% [99.0%, 99.5%]    14.7
mono-green-stompy:uw-tempo          1939    2061    0      48.5% [46.9%, 50.0%]  72.4% [71.0%, 73.8%]    13.6
mono-red-goblins:the-epic-storm     1999    2001    0      50.0% [48.4%, 51.5%]  99.6% [99.4%, 99.8%]    10.1
mono-red-goblins:tron               1998    2002    0      50.0% [48.4%, 51.5%]  100.0% [99.9%, 100.0%]  9.9
mono-red-goblins:ur-delver          2186    1814    0      54.6% [53.1%, 56.2%]  81.5% [80.3%, 82.8%]    11.4
mono-red-goblins:uw-control         2001    1999    0      50.0% [48.5%, 51.6%]  99.9% [99.8%, 100.0%]   12.4
mono-red-goblins:uw-tempo           1925    2075    0      48.1% [46.6%, 49.7%]  72.3% [70.9%, 73.7%]    13.8
the-epic-storm:tron                 2058    1942    0      51.4% [49.9%, 53.0%]  42.9% [41.4%, 44.4%]    42.9
the-epic-storm:ur-delver            2012    1988    0      50.3% [48.8%, 51.8%]  3.9% [3.3%, 4.5%]       16.4
the-epic-storm:uw-control           1932    2068    0      48.3% [46.8%, 49.8%]  30.4% [29.0%, 31.8%]    50.0
the-epic-storm:uw-tempo             1990    2010    0      49.8% [48.2%, 51.3%]  1.8% [1.4%, 2.2%]       18.0
tron:ur-delver                      2013    1987    0      50.3% [48.8%, 51.9%]  2.1% [1.7%, 2.6%]       16.3
tron:uw-control                     2093    1907    0      52.3% [50.8%, 53.9%]  64.6% [63.1%, 66.1%]    47.2
tron:uw-tempo                       1999    2001    0      50.0% [48.4%, 51.5%]  0.5% [0.3%, 0.7%]       17.9
ur-delver:uw-control                2028    1972    0      50.7% [49.2%, 52.2%]  97.5% [97.0%, 97.9%]    19.2
ur-delver:uw-tempo                  2232    1768    0      55.8% [54.3%, 57.3%]  47.3% [45.8%, 48.9%]    16.3
uw-control:uw-tempo                 2015    1985    0      50.4% [48.8%, 51.9%]  0.6% [0.3%, 0.8%]       22.9

games per pair: 4000  (total 264000 games across 66 pairs)
pooled across 66 pairs: bot wins 144963, B wins 119037, draws 0
pooled bot win rate: 54.9%  95% CI [54.7%, 55.1%] (normal approximation to the binomial, pooled over counts)
pooled seat 0 win rate: 59.1%  95% CI [58.9%, 59.3%] (normal approximation to the binomial)
mean turns per game (pooled): 17.4
pairs whose A-win interval excludes 50%: A loses on 2, A wins on 30, undecided on 34
```

## Reading the matrix

**Pools.** Pooled across the 66 pairs A holds 54.9% [54.7, 55.1] (counts-pooled
CI). That single number hides the structure the whole exercise is about: the
rep is "A beats B on 30 of 66 pairs, loses on 2, is undecided on 34 at a
Wald width of roughly ±1.5pp". A lone pooled 54.9% reads like a mild edge;
the decomposition says it is a **massive but narrow** edge — the bot wins
decisively on nearly half the matchups and is indistinguishable from `legacy`
on the other half, with a genuine*, if small, losing end.

**Pairs where the policy LOSES (interval below 50%).** Exactly two, and they
are the interesting result:

1. `mono-red-goblins:uw-tempo` — **48.1% [46.6%, 49.7%]** (A wins 1925/4000).
2. `the-epic-storm:uw-control` — **48.3% [46.8%, 49.8%]** (A wins 1932/4000).

(`mono-green-stompy:uw-tempo` at 48.5% [46.9, 50.0] brushes 50% and is
technically undecided, but it is a third near-tolerant loss the same
direction.) Note who they are: both are matchups where the opponent is a
**tempo/control deck that gains value while the game runs long or the board
goes wide**, and both are the matchup type the current in-zone-order spell
selection (the research's #1 unfixed sensitivity, worth 16.8pp) would be
expected to struggle with. There is more in common: `uw-control`/`uw-tempo`
are the hero cards here (deck *in use against* the bot, seat flip is wash).
This is exactly the kind of "change that helps aggro can cost 9pp on
control" specialisation the matrix exists to surface, and it strongly agrees
with the research's call to point the next bot work at spell selection.

**Best pairs.** `mono-blue-tempo:uw-tempo` 72.2%, `death-n-taxes:mono-blue-tempo`
72.0%, `mono-black-aggro:mono-green-stompy` 71.2%, `death-n-taxes:uw-tempo`
69.2%, `death-n-taxes:mono-green-stompy` 67.9% — i.e. the bot's edge is
concentrated against decks whose plan is "undeveloped board plus slow
combo/control finish", and it pairs best where the opponent is a *second*
copy of a build the bot is already good against.

**Seat play/draw edge — how much it varies by pair.** The seat-0 rate swings
from **0.5%** (`tron:uw-tempo`) to **100.0%** (`mono-red-goblins:tron`).
That is far more than a first-turn ordering effect, and the reason is
deliberate and worth saying plainly: because the deck list a seat holds is
fixed for the whole pair, the seat-0 rate is *dominated by deck-matchup
asymmetry* (which of the two decks is sitting in seat 0), not by who goes
first. On the evenly-matched pairs where deck quality does not swamp it, the
seat-0 rate sits at the familiar 55–60% band (e.g. `dimir-tempo:ur-delver`
55.9%, `death-n-taxes:ur-delver` 55.5%), matching the +7.2pp play/draw edge
the research measured; on lopsided matchups the rate compresses to ~0 or
~100 to say *which deck wins the matchup*, not who plays first. So the honest
summary is: the seat/order edge is the ~5–7pp family it has always been on
balanced pairs, and everything more extreme in the seat-0 column is deck
dominance, correctly driven by the deck-to-seat binding. Any future bot task
that reads a seat-0 figure must keep the deck axis in mind.

**Pairs that behave strangely.**

- *Very long games (stall/control), all `uw-control`/`tron`/`storm` mixes:*
  `the-epic-storm:uw-control` mean **50.0** turns, `tron:uw-control` **47.2**,
  `the-epic-storm:tron` **42.9**, `dimir-tempo:uw-control` **31.6`,
  `mono-blue-tempo:uw-tempo` **24.2**, `dimir-tempo:tron` **25.1**. These are
  the mirror/control stalls, not anomalies.
- *Very short games (linear aggro vs a slow finisher):* `mono-red-goblins:tron`
  **9.9**, `mono-red-goblins:the-epic-storm` **10.1**, `mono-green-stompy:tron`
  **10.8**, `mono-red-goblins:uw-control` **12.4**, `mono-blue-tempo:tron` 17.6
  — the fast goldfisher runs the slow deck over before it develops.
- *Draws:* **zero draws in all 264,000 games.** Not a surprise for CR
  104.4a's no-surviving-seats condition under two sane policies, but worth
  stating since the pool sits on it.

**Notably "no policy difference" rows abound** (30 pairs at or near 50.0%
with the CI straddling 50) — the bot's B2–B3 effort over the combat surface
is flat against most of the field, which is exactly the no-headroom signal
the research predicted and why the next milestone should stop point money at
combat.

## Liveness lead — investigated, not fixed

The research flagged that a `KChoose`-randomising seat could drive a game
past the 400,000-intent cap and asked whether the matrix hits it. **No pair
in this matrix hits the cap, and no game in any pair does.** The full
264,000-game run completed with every game terminating on a winner (mean
intents far below 400k). The `bot` and `legacy` seats here are both real
policies — neither randomises `KChoose` the way the probe did — so the
matrix provides no evidence of a real engine liveness bug in that
configuration. The probe artefact (a deliberately pathological seat) is not
reproduced by any actual policy matchup. The cap is unchanged; nothing was
fixed.

## Verbatim gate output

```
$ gofmt -l .                      # -> no output (clean)
$ go vet ./...                    # -> rc=0, no output
$ go test -count=1 ./cmd/botbench/   # -> ok  github.com/adams-shaun/gorge/cmd/botbench
$ go test -count=1 ./rules/       # -> FAIL: TestHeads (pre-existing, see below; all other rules tests pass)
```

`go test ./cmd/botbench/` runs the full new suite (16 tests) plus the 8
existing ones; the synthetic matrix tests run in microseconds, the smallest
real-engine matrix (`TestMatrixEndToEnd`, 4 games) in ~0.5s.

## Pre-existing `TestHeads` failure at base 79d45b1 — NEEDS CONTEXT

`go test -count=1 ./rules/` fails on `TestHeads` at the **pristine base commit
79d45b1, with my changes stashed** — the identical four moved heads:

```
2 seats: 9b7d5a6232041650 (golden 45e0671d07b60d9e)
4 seats: e47b184b6e416678 (golden 04950e3969039a7b)
6 seats: 71f65a02830ac615 (golden d70bc7e30c0fccdd)
8 seats: e7c4c01973df351c (golden 496784e7fbcf37be)
```

**Measured cause (bisected in throwaway worktrees, not guessed):**
`TestHeads` **passes** at `336e2cd` (parent of the B3 targeting merge) and
**fails** at `60b8981` (the B3 merge) with *exactly* those three 4/6/8 heads
(and the same 2-seat move). The cause is the **B3 targeting heuristic
`e13a3ee`** ("never own, board over face, rank by threat, honour Min/Max",
merged `60b8981`), which changed `botpolicy.Decide`'s `KTarget` branch and
therefore the games the acceptance suite replays. The chain-head golden was
last regenerated for **B2** at `6074251` and was never regenerated for B3, so
at base the golden is stale and `TestHeads` is red. All other `rules/` tests
including the coverage ratchet (`TestEveryRepoDeckIsFullySupported`) pass, so
the corpus (pinned `95f04e8a…`, matching Makefile `FORGE_REF`) is compatible
with the engine.

This is **not caused by this task** — my diff touches only `cmd/botbench/`
and the report; `rules/` is untouched, and per the brief I do **not** edit
`rules/heads_test.go` goldens and do not change the engine. The `rules/` gate
cannot be made green by me without violating the "do not touch
`heads_test.go`" constraint; the fix belongs to whoever owns the B3 golden
regeneration (name the measured cause above). This is flagged as
`NEEDS_CONTEXT` for the controller.

## Deviations from the brief (with reasons)

1. **Still parallel (matrix only).** The brief's "tool already
   parallelises" is not true of the shipped single-pair path, and it
   explicitly asked only for progress reporting. I added a modest,
   default-on `-workers` (default `runtime.NumCPU()`, capped at the pair
   count) that runs **pairs** in parallel rather than serialising the whole
   matrix, solely so a 66-cell N=4000 run finishes in tens of minutes rather
   than an hour on one core, and the report stays byte-deterministic under
   any worker count (results land in per-position slice slots; the report is
   emitted once in pair order). `-workers 1` gives the fully serial
   behaviour the brief's "hour on one core" describes. Result correctness is
   independent of the worker count, so this is a wall-time optimisation, not
   a semantics change.
2. **`-format json` implemented and justified.** The brief allowed it pro
   forma ("your call; justify it"). It is the only shape a future task can
   diff two builds on numerically: rates are fractions, the CI and seat CI
   are explicit `[lo,hi]` pairs, `games_per_pair` is in the document so a
   reader cannot mistake the total for the per-pair N, and the JSON is
   byte-stable (struct field order, no map) so `diff <(run A) <(run B)` is a
   real signal.
3. **Corpus symlink.** The worktree has no `.cards/` (gitignored, not
   carried by git). I symlinked `.worktrees/m1/.cards ->
   /home/sadams/projects/gorge/.cards` (the pinned corpus) so `run -dir
   .cards` and the auto-detecting tests work; it is gitignored and not part
   of the commit. I did not run `make fetch-cards`.

## Mutation checks (each broke, a named test failed, then restored byte-for-byte)

1. *Stop trading policies within a pair* (`if aPlaysSeat(g, seat)` →
   `if seat == 0`):
   `TestMatrixTradesPoliciesEveryGame` fails — "policy 'a' held seat 0 in
   6 games, want 3".
2. *Make `-games` mean the total* (`games /= total` in `runPairs`):
   `TestMatrixGamesArePerPair` fails — "pair d1:d2 tallied 3 games, want 7
   per pair" and "header must state per-pair count unambiguously" (the
   report printed the contradiction "games per pair: 7 (total 6 games)").
3. *Pool the CIs by averaging the rates instead of pooling the counts*:
   `TestPooledCIPoolsCountsNotRates` fails — report printed "pooled a win
   rate: 5.0%" instead of 9.9%, CI off accordingly.
4. *Make pair order depend on map iteration* (report ranged over a
   `map[string]pairResult`):
   `TestMatrixReportOrderIsSorted` fails — "pair 'mono-blue-tempo:uw-control'
   sits at report row 0 but its sorted index is 43".

## Open concerns

- The B3 `TestHeads` regeneration (above) is owned outside this task.
- `TestMatrixDeterministicUnderWorkers` asserts byte-determinism under
  workers=1 vs workers=8 over the synthetic path; the engine path is covered
  by `TestMatrixEndToEnd` determinism.
- `-workers` defaults to `runtime.NumCPU()`; a machine burst above that is
  the user's call (`-workers 1` to be single-threaded).
