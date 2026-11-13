# How dirac is held to gorge

dirac is a second Magic engine. gorge (`engine/` in this repository) is the
reference engine and dirac's **oracle**: wherever both engines can play the same game, dirac must
pose the same choices and reach the same state. This page maps the tools that check that, explains the
**lockstep** check they share, and covers following a gorge change and the parity tests.

Related pages: [dirac/README.md](../README.md) (what dirac is and its Go API),
[cards.md](cards.md) (how cards enter dirac), [running.md](running.md) (running games, bots and
measurements).

## Which tool answers which question

| Question | Tool | What it does |
|---|---|---|
| Does dirac play this game exactly like gorge? | `dirac/oracle` lockstep, driven by `dirac/oracle/cmd/kfuzz` | Plays the same decks and seed on both engines and compares the canonical option list at every decision and the state projection after every committed action. |
| Does a recorded game replay? | `dirac/oracle` `Repro`/`Replay`; `kfuzz -repro`, `kfuzz -replay` | `Repro` stores a divergence's action prefix; `Replay` re-executes a kernel game in gorge with dirac's chance draws forced. |
| Does the heuristic bot choose the same? | `dirac/oracle/cmd/khbot`, `dirac/oracle/hbot_parity.go` | Gorge's auto-pay heuristic bot plays gorge; dirac's port (`dirac/search/hbot`) answers the same decisions on dirac state; prints agreement per decision kind. |
| SpellBench-transcript parity | `dirac/oracle/cmd/ksbt`, `dirac/oracle/sbt*.go`, `v2bridge.go`; `kfuzz -v2 transcript` | Gorge's sb-tactical answers through a view built from dirac state (`ksbt`); the v2 transcript mode diffs the same seeded games played by `dirac/cmd/sbv2dirac` and gorge's environment. |
| Which Comprehensive Rules are covered? | `dirac/crcoverage` | One Markdown file per CR chapter, one row per rule, with status and the code that decides it. |
| Did a change cost speed? | `BenchmarkTransitionFDN`, `BenchmarkTransitionBLB`, `BenchmarkTransitionFDNPool` in `./dirac` | Single-core transition benchmarks on real decks; see [running.md](running.md#performance). |

## Quick start: one lockstep run

Build the driver, then play a couple of FDN pool pairings in lockstep:

```sh
go build -trimpath -o bin/kfuzz ./dirac/oracle/cmd/kfuzz
bin/kfuzz -kernel fdn -first-pair 0 -pairs 2 -seed 1 -keep-going -surface autopay -shrink 0
```

```
fdn: 2 games (0 skipped as unsupported), 2 without divergence, 0 kernel capacity stalls [] (lead gorge); first divergences:
fdn: 2 games (0 refused-card, 0 refused-pair, 0 error), 2 clean, 0 divergent, 0 unmodelled [], 0 capacity [], 0 hang (lead gorge, surface autopay)
fdn-classes-json: {}
```

The second `fdn:` line is the verdict. Games are the pairs that started a game, each of them clean,
divergent, stalled (`unmodelled` or `capacity`, by reason) or a `hang`; a pair that could not be built
is outside the count, in the bracket. The first `fdn:` line is a shorter form of the same run. The
`fdn-classes-json:` line repeats the divergence classes as JSON. After a divergence an `implicates:`
line ranks the cards named most often, the top 10 (`oracle.Divergence.Implicates`: the card in an
option only one side offers, else the first differing `Cards[]` field other than `Pos`, else the source
of the last action that has one). It is an upper bound: a shared mechanism blames its victims.

On a pool (`-kernel fdn`, `blb` or any registered pool) `kfuzz` exits like this:

| Outcome | Plain | `-absolute` |
|---|---|---|
| clean, turn or decision cap | 0 | 0 |
| capacity stall | 0 | 0 |
| `refused-card`, `refused-pair` skip | 0 | 0 |
| divergence (a panic in a game counts as one) | 1 | 1 |
| hang | 1 | 1 |
| unmodelled stall | 0 | 1 |
| non-empty `unknown:` line | 0 | 1 |
| `error` skip | 0 | 1 |
| no game played | 0 | 1 |

Bad flags exit 2. `-absolute` prints `absolute: OK` or `absolute: FAIL: <what>`. It, `-pairs-file` and
`-evidence` belong to pool runs and exit 2 anywhere else. The `vanilla` and generic `gorge`/`gorge-api`
paths also exit 1 on a divergence or a hang.

`-game-timeout` (default 30 s, 0 turns it off, on every path) records a game with no result in that
time as a `hang` with its decks and seed, and the run moves on — by replacing its own process image:
a hung game's goroutine cannot be stopped and would hold a core until the process exits, so the first
hang ends the segment (the verdict lines cover the pairs it played) and kfuzz execs itself for the
remaining pairs (same pid, new image — the hung goroutine dies with the old one), which fails the run
even when its own segment is clean; the hang line's `rerun with: <cmd>` names the command that replays
one pair, and the segment-level file flags
(`-coverage`, `-evidence`, `-reach`, `-out/repro.json`, `-cpuprofile`, `-memprofile`) are inherited
unchanged, so the last segment's file wins (a `-cpuprofile`/`-memprofile` is not flushed past the
exec) — a hang already fails the run, and the per-pair files
under `-out` are pair-numbered and survive. `-surface autopay|full` picks the option
surface (default `full`; `-autopay-surface` means `-surface autopay`). `-pairs-file F` plays
`deckA<TAB>deckB[<TAB>seed]` lines instead of drawing from `pool.tsv`. `-coverage F` and `-evidence F`
count clean games only: `-evidence` writes the cards with at least `oracle.EvidenceMinPlays` plays and
every own ability key used, leaving out cards refused by name. It plays unverified cards, so that a card can
earn evidence, and not refused ones; `-allow-unverified` plays them the same way without writing a list. A
pair holding a refused card is skipped and counted. A synthetic pool deals every lowered card, so a refusal
takes games out of it and leaves its other pairs as they were. `scripts/dirac-evidence.sh <out>` plays the whole
evidence corpus and `scripts/dirac-first-refusals.py evidence <out>` writes `dirac/support/evidence/`, the floor
met across all of a card's runs. `-implicates-all` names every implicated card and prints one `bad-game` line per
game that fails `-absolute`; `scripts/dirac-first-refusals.py settle <out>` reads them to find the refused list
that leaves the corpus clean, from a run with `-play-refused`. On a deck pool's 1500-pair slice, `-absolute` also fails under `oracle.GateMinGames` started
games (`oracle.SliceMinGames` on a committed slice, which has no further pairs), so refusals cannot shrink a gate without
an edit saying so.

`-mulligans N` is the London mulligans each player may take, on every path; the default 0 skips the
round. It changes every game's stream, so a slice at `-mulligans 1` is not the same games as at 0.

A verdict's stalls are split by `dirac.StallClassOf`. `capacity`: a fixed array or budget ran out and
gorge has no such limit; the game was correct up to there. These are declared limits, always counted,
never a failure, and such a game still earns coverage. `unmodelled`: gorge does something at this point
that dirac does not; these fail under `-absolute`. A reason nobody classed counts as unmodelled.
A rule gap must not stop under a capacity reason's name: `TestRuleGapSitesKeepTheirOwnReason`.

A pair whose cards dirac cannot play together is refused before the game, as `refused-pair`, when
`Matchup` builds its table (`cardset.InteractionRefusal`, naming the carriers): the table flags that used
to stall at the start of the game (`dirac`'s `genPrep.initStall`). The `refused-pair:` line counts them by
reason. `Init` keeps the same check as a guard for a hand-built table, and
`TestInteractionRefusalMatchesInitStall` holds the two together.

### Synthetic evidence pools

Two real pools leave most of what kgen claims unplayed, so `kfuzz -synth <pool>` plays a card list
instead of a deck list. `make dirac-tables` also writes one table per list under
`<corpus>/kernel-synth-tables/<key>/`, and nothing about these pools is committed: `kgen -synth-pools`
derives each list from the set lists and the corpus on every build, and kfuzz deals the decks in memory.

- `std-<set>` is one set list of `dirac/cardset/standard.txt`, front-face names, basics added.
- `tail-NN` is 250 lowered cards outside those lists, cut in FNV-1a order. `tail-unlowered.txt` beside
  the tables names the rest of the corpus, so every card is accounted for.
- A list too big for one table (over the 192 subtypes a table's masks hold) becomes `<pool>-a` and `<pool>-b`, split
  on the lowest bit of the name's hash that lets both halves build. Cards on opposite sides never meet.

`oracle.SynthPool` deals `oracle.SynthDecks` 40-card decks from `-seed`: two colours, 23 spells from a
shuffled queue at most two of each, up to 4 nonbasic lands, basics to 40. A card of three or more colours
at the head of the queue makes the deck take its colours, or that card would never be dealt. The deal is from the cards the run plays: the evidence
run leaves out the cards refused by name, so no pair is lost to one, and the run that settles
the refused list (`-play-refused`) deals every lowered card, which is what keeps a refused
entry's repro pair fixed. Within the cards dealt, which ones a deck holds is the queue's and
never depends on how a card plays. A deck name
(`std-dsk_00017_WU`) and `-seed` reproduce a game: `kfuzz -synth std-dsk -first-pair N -pairs 1 -seed 1`.

```sh
bin/kfuzz -synth std-dsk -first-pair 0 -pairs 1500 -seed 1 -keep-going -surface autopay -shrink 0 -workers 4 -reach reach.tsv
```

Evidence is a claim about games played, so a synthetic run always prints how far its list got:

```
reach std-dsk: 271 cards in the list, 225 lowered, 225 dealt, 225 appeared in a game, 216 met 3 plays in clean games, 191 evidence (70.5% of the list, over 1500 games)
```

Basics are not counted. `appeared` counts every game that started, whatever its end; the last two count
clean games only, and `evidence` also needs every own ability key used. `-reach F` writes one row per card
saying where it stopped: `not-lowered`, `refused`, `not-dealt`, `no-game`, `few-plays`, `keys-unused` or `evidence`.
Synthetic decks are not human decks: per-game rates overstate what a real pool sees, while a finding about
a card holds either way. A game-start stall spends the whole game, so a list with such an interaction reaches
less: TDM's three-colour `{2/C}` hybrids beside any cost modifier stall about 70 % of its games.

`kfuzz` needs the gorge card corpus: run it where `.cards` is linked or
pass `-cards DIR`.

## Lockstep, the core check

`oracle.Lockstep` (`dirac/oracle/lockstep.go`) starts a fresh gorge side and plays one game with
gorge leading. `oracle.LockstepLed` (`lockstep.go`) allows either side to lead. Both sides get the
same decks and seed; a policy answers the leader's canonical decision, and the follower mirrors it.
Every side implements the same `Side` surface (`dirac/oracle/canon.go`): `Step`, `Apply`,
`Project`, `Terminal`, `Decided`, `Dirty`, `Turn`, `Err`.

What is compared, in order:

1. The canonical **option list** at every decision. Options are sorted `Canon` values (the action
   grammar is documented in `dirac/oracle/doc.go`), so the comparison is order-independent and
   catches a missing, extra or differently-priced play.
2. The **decision kind and player** (a priority decision must not become a declaration, and the wrong
   seat must not act).
3. The **Projection** after every action gorge commits. A Projection
   (`dirac/oracle/project.go`) is the whole canonical game state: per card instance its card,
   owner, controller, zone, position, flags, damage, counters and attachments; then life, poison,
   speed, the stack bottom to top, turn, step, active player, priority holder, RNG draw count, lands
   played, mana pools and the game result. `dirac/doc.go` defines dirac's side of the same
   shape. Both are fixed arrays, so equality is a value comparison.
4. The **terminal result** (`over`, `winner`) when both sides stop.

A game that hits `-max-turns` or `-max-decisions` is a **stall**, not a divergence. A **dirac
capacity stall** is different: dirac has fixed capacity (stack, board, instances) and ends the
game when it runs out (`oracle.KernelStallPrefix`, the `kernel ` prefix in the verdict). gorge has no
such cap, so a capacity stall is reported separately and is a known dirac limit, not a mismatch. A
game that hits a turn or decision cap agreed for its whole length and counts as clean. A game stopped at
a dirac capacity agreed up to there and earns coverage for that much; one stopped at an unmodelled
reason earns none. Lockstep records which in `Result.StallUnmodelled`.

One capacity stall is gorge's: a trigger that reached the engine's hang guard (`maxTriggerFires`, 256
fires of one source and trigger over a match) stops matching, and dirac does not count fires. When a
game would be reported divergent and `rules.Engine.TriggerCapReached` holds, it ends as a stall named
`oracle.GorgeTriggerCapStall`. It agreed up to the decision where the difference appeared, counts under
the capacity reasons of the verdict, and is printed with `-v` and `-absolute`.

### How a divergence is reported

`oracle.Divergence` (`lockstep.go`) records the decision number (`At`), what disagreed (`What`, usually
`options`, `projection`, `decision`, `terminal`, `gorge` or `kernel`), the first mismatching field
(`Field`, for example `Cards[12].Zone`), every mismatching field with both values (`Diff`), both
option lists, and the side that led. `kfuzz -v` prints the last actions before the divergence and the
field diff. This run of 2026-10-30 shows the shape; the difference it found has since been
fixed, and the same command plays clean:

```
$ bin/kfuzz -kernel blb -first-pair 8 -pairs 1 -seed 1 -surface full -mulligans 1 -shrink 0 -v
pair 8 (BLB_top_19581_WBpR vs BLB_top_28414_WB, seed 10215742739101889101) after 157 decisions:
    after pass(-,-,0)
    after pass(-,-,0)
    after pass(-,-,0)
    after done(-,-,0)
    after activate(#4{Cindering Cutthroat},-,0)
divergence at decision 157: options options choose decision for p0
  gorge  options: [choose:pay(-,-,2)]
  kernel options: [choose:pay(-,-,2) choose:pay(-,-,3)]
  implicates: Cindering Cutthroat
```

### Reproducing one

The engines are deterministic: the same `-seed`, `-first-pair` and `-pairs` replay the same games
byte for byte, so the command that found a divergence reproduces it. Add `-v` to see the last actions
and the field diff, and `-shrink N` (default 200) to shorten the action prefix while it still
diverges; `-shrink 0` turns shrinking off.

`oracle.Repro` (`dirac/oracle/repro.go`) is the portable form: the two decks by card name,
the seed, the run knobs and the canonical action prefix. Every path writes it with `-out DIR` and
replays it with `-repro FILE`: the generic `gorge`/`gorge-api` path and the pool paths write the first
divergence as `DIR/repro.json` (a pool repro needs no deck files or `pool.tsv`), and the `vanilla` path
writes up to three files per divergence class under `DIR`. A clean run writes nothing. `-repro` takes
the surface from the command line, since a repro does not record it.

For a game dirac played on its own, `oracle.RecordKernelGame` makes a `koracle-game-2` record, and
`kfuzz -replay FILE` re-executes one saved as JSON in gorge with
its chance draws forced, and verifies gorge's log. `-replay` is routed before dirac switch, so it
works under any `-kernel`.

## The oracle import boundary

Only `dirac/oracle` and its commands, `dirac/kgen`, `dirac/cmd/dplay` and `dirac/cmd/sbv2dirac` may
import gorge's packages. The core (`dirac`, `dirac/kop`, `dirac/vanilla`, `dirac/cardset`, `dirac/search/...`,
`dirac/feat`) must not. The boundary keeps dirac a standalone engine: its `State` cannot
accidentally hold a gorge pointer or map, and dirac builds and tests without the gorge engine
except through the oracle. The oracle is the one place both engines meet, which is why every comparison
lives there.

Check the boundary with:

```sh
go list -f '{{.ImportPath}}: {{join .Imports " "}}' ./dirac/... | grep -E ' github.com/adams-shaun/gorge/(engine|bots|server)' | cut -d: -f1
```

```
github.com/adams-shaun/gorge/dirac/cmd/dplay
github.com/adams-shaun/gorge/dirac/cmd/sbv2dirac
github.com/adams-shaun/gorge/dirac/cmd/sbv2dirac/sbv2
github.com/adams-shaun/gorge/dirac/kgen
github.com/adams-shaun/gorge/dirac/oracle
github.com/adams-shaun/gorge/dirac/oracle/cmd/kfuzz
```

## Following a gorge change

gorge and dirac are one Go module, so `dirac/oracle` always plays the gorge in the same tree. gorge
leads: new behaviour lands there first. A change that alters what a supported card does fixes dirac in
the same change, or makes kgen refuse the affected cards by name (`kgen -refuse`, see the
[glossary](glossary.md)), which leaves the catch-up visible. After such a change:

- re-run lockstep, because the oracle's behaviour may have changed;
- update `dirac/docs/GORGE-FINDINGS.md` where a copied behaviour is now fixed or newly required;
- update `dirac/crcoverage` rows that say dirac inherits gorge's behaviour, when gorge's row
  moved;
- rebuild the tables with `make dirac-tables`, since a change in the gorge packages kgen builds against
  moves the table key. If the accepted counts moved, rewrite `dirac/kgen`'s acceptance pins:
  `KGEN_REPIN=1 DIRAC_UPDATE_SUPPORT=1 go test -v -timeout 5m -run 'TestCorpusAcceptance|TestSetAcceptance|TestCorpusBuildAcceptance|TestSupportDocIsCurrent' ./dirac/kgen`.
  That is not a parity fix and should only follow a real count change. A lower accepted count fails
  unless `KGEN_REPIN_FALL=<reason>` is also set, and the reason is printed.

## Bot and transcript parity

`khbot` (`dirac/oracle/cmd/khbot`) compares dirac's port of gorge's auto-pay heuristic bot with
the bot itself. Gorge's bot chooses every action on gorge; at each decision dirac bot is asked
the same decision on dirac's state (`oracle.HBotParityGame`, `dirac/oracle/hbot_parity.go`).
It prints agreement per decision kind and one example of each disagreement class (2026-10-29):

```sh
go run ./dirac/oracle/cmd/khbot -pairs 1 -workers 1
```

```
khbot hbot: 1 games (0 diverged before the end), 77 decisions compared, 0 not comparable (gorge's answer not on the canonical surface; 0 picks followed another option), wall 38ms
(0 priority decisions where gorge's bot tapped mana by hand: compared as the cast it taps for)
kind                     n    agree same-card
attackers                8  100.00%  100.00%
blockers                 5  100.00%  100.00%
choose                   2  100.00%  100.00%
priority                54  100.00%  100.00%
target                   4  100.00%  100.00%
trigger_order            4  100.00%  100.00%
disagreement classes:
```

`ksbt` (`dirac/oracle/cmd/ksbt`) does the same for SpellBench's sb-tactical: gorge's own decision
core answers a gorge view that the adapter builds from dirac position
(`oracle.SBTParityGame`, `dirac/oracle/sbt_parity.go`; the view is `sbt_view.go`, the agent
`sbt.go`). It is an agent, not a hot-loop policy, because the gorge view allocates.

```sh
go run ./dirac/oracle/cmd/ksbt -pairs 1 -workers 1
```

```
ksbt: 1 games (0 diverged before the end), 84 decisions compared, 0 not comparable (gorge's answer not on the canonical surface; 0 picks followed another option), wall 42ms
(0 priority decisions where gorge's top play is not on the canonical surface: compared as its best offered play)
kind                     n    agree same-card
attackers                9  100.00%  100.00%
blockers                 3  100.00%  100.00%
choose                   2  100.00%  100.00%
priority                57  100.00%  100.00%
target                  10  100.00%  100.00%
trigger_order            3  100.00%  100.00%
disagreement classes:
```

`kfuzz -v2 transcript` is a different seam: it plays the same seeded games on `dirac/cmd/sbv2dirac`
(dirac hosted as a SpellBench v2 engine) and gorge's environment and diffs their transcripts.
`-v2-no-chance` is the negative control: the gorge host
keeps its own shuffles instead of dealing dirac's chance, and the transcripts must differ.
`dirac/oracle/v2bridge.go` maps an agent's native v2 option back to the canonical dirac action.

## Adding a card: the parity test shape

A new card or mechanic gets a pool (`dirac/cardset/pools/fixtures/<name>/cards.txt`, built by `make dirac-tables`)
and a lockstep test in `dirac/oracle`. The smallest complete model is
`dirac/oracle/altcost_test.go` (`TestAltcostLockstep`): a two-deck, 40-card-each fixture in
`kaltcostDecks`, and one call to `lockstepSet(t, "altcost", kaltcostDecks, 1000,
"KALTCOST_SEEDS")`. The shared helper is `dirac/oracle/ward_test.go`.

The shape:

- add the pool list so `make dirac-tables` builds it and `cardset.ByName` finds it;
- write a comment naming the mechanic and the CR rules the fixture is meant to reach;
- give both decks 40 cards, chosen so the mechanic actually occurs (the comment says how);
- call `lockstepSet`, which plays `seeds` games, mixes policies by seed (`seed%4 == 0` on gorge's
  auto-pay surface, `seed%8 == 0` random, otherwise cast-first) and fails on every divergence; the
  environment variable named by the last argument overrides the seed count for a quick run;
- add a mutation test (`TestProwessLockstepMutation`, `dirac/oracle/prowess_test.go`) that flips
  a dirac debug knob and proves some seed diverges, so the fixture is known to reach the mechanic.
  Without the mutation proof a green lockstep can mean the card was never exercised.

Run one focused set with a small seed count:

```sh
systemd-run --user --scope -q -p MemoryMax=4G -p CPUQuota=400% env GOMAXPROCS=4 GOMEMLIMIT=3GiB KALTCOST_SEEDS=8 go test -timeout 2m -run 'TestAltcostLockstep' ./dirac/oracle/
```

```
ok  	github.com/adams-shaun/gorge/dirac/oracle	0.615s
```

## Comprehensive Rules coverage

`dirac/crcoverage` is the rule-by-rule audit: one Markdown file per CR chapter, one row per numbered
rule, with a status (`follows`, `partial`, `deviates`, `not-impl`, `definition`, `n/a`, `unknown`),
the code that decides it, the tests that exercise it and a note. dirac is two-player and plays
fixed pools, so many rows are `n/a` with the reason stated. The package README explains how to read
and update a row: [dirac/crcoverage/README.md](../crcoverage/README.md).

```sh
systemd-run --user --scope -q -p MemoryMax=4G -p CPUQuota=400% env GOMAXPROCS=4 GOMEMLIMIT=3GiB go test -timeout 2m -run 'TestCRCoverageIsCurrent' ./dirac/crcoverage
```

```
ok  	github.com/adams-shaun/gorge/dirac/crcoverage	0.127s
```

`TestCRCoverageIsCurrent` fails when a row stops matching the code. A behaviour change updates its
row in the same commit; `KERNEL_UPDATE_CRCOVERAGE=1` regenerates the README's summary table.

## Speed

`BenchmarkTransitionFDN`, `BenchmarkTransitionBLB` and `BenchmarkTransitionFDNPool` in `./dirac` play
real decks on one core and report no allocations. [running.md](running.md#performance) has the command
and how to compare two commits; compare instructions per operation, not absolute time across days.

## Differences from gorge

[`DEVIATIONS.md`](DEVIATIONS.md) lists the differences that are not divergences, each classed as canonical
surface or declared capacity. [`GORGE-FINDINGS.md`](GORGE-FINDINGS.md) lists the gorge behaviour dirac
copies and the differences still open.

## Where next

- [dirac/README.md](../README.md) — dirac's state, actions, decisions and projection.
- [dirac/docs/README.md](README.md) — the documentation index.
- [cards.md](cards.md) — kgen, the binary card tables and `cardset`.
- [running.md](running.md) — playing games, bots and the SpellBench v2 engine.
- [dirac/docs/kgen-params.md](kgen-params.md) — kgen's parameters.
