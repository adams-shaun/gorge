# How dirac is held to gorge

gorge (`engine/`) is the reference engine and dirac's **oracle**: wherever both engines can play the same game, dirac must
pose the same choices and reach the same state. This page maps the tools that check that, explains **lockstep**, and
covers following a gorge change and the parity tests.

Related: [dirac/README.md](../README.md) (the Go API), [cards.md](cards.md) (how cards enter dirac),
[running.md](running.md) (running games, bots and measurements).

## Which tool answers which question

| Question | Tool | What it does |
|---|---|---|
| Does dirac play this game exactly like gorge? | `dirac/oracle` lockstep, driven by `dirac/oracle/cmd/kfuzz` | Plays the same decks and seed on both engines; compares the canonical option list at every decision and the state projection after every committed action. |
| Does a recorded game replay? | `oracle.Repro`, `oracle.Replay`; `kfuzz -repro`, `kfuzz -replay` | `Repro` stores a divergence's action prefix; `Replay` re-executes a dirac game in gorge with dirac's chance draws forced. |
| Does the heuristic bot choose the same? | `dirac/oracle/cmd/khbot`, `oracle/hbot_parity.go` | gorge's auto-pay heuristic bot plays gorge; its dirac port (`dirac/search/hbot`) answers the same decisions on dirac state; prints agreement per decision kind. |
| SpellBench-transcript parity | `dirac/oracle/cmd/ksbt` (`sbt*.go`); `kfuzz -v2 transcript` (`v2bridge.go`) | sb-tactical answers through a view built from dirac state (`ksbt`); the v2 mode diffs seeded games played by `dirac/cmd/sbv2dirac` and gorge's environment. |
| Which rules are covered? | `dirac/crcoverage` | One Markdown file per CR chapter, one row per rule, with status and the code that decides it. |
| Did a change cost speed? | `BenchmarkTransitionFDN`, `BenchmarkTransitionBLB`, `BenchmarkTransitionFDNPool` in `./dirac` | Single-core transition benchmarks; see [running.md](running.md#performance). |

## One lockstep run

```sh
go build -trimpath -o bin/kfuzz ./dirac/oracle/cmd/kfuzz
bin/kfuzz -kernel fdn -first-pair 0 -pairs 2 -seed 1 -keep-going -surface autopay -shrink 0
```

```text
fdn: 2 games (0 refused-card, 0 refused-pair, 0 error), 2 clean, 0 divergent, 0 unmodelled [], 0 capacity [], 0 hang (lead gorge, surface autopay)
```

That is the verdict line (`kfuzz` prints a shorter form of the same run and a `fdn-classes-json:` line beside it). Games
are the pairs that started a game, each clean, divergent, stalled (`unmodelled` or `capacity`, by reason) or a `hang`; a
pair that could not be built is outside the count, in the bracket. After a divergence an `implicates:` line ranks the top
10 cards named (`oracle.Divergence.Implicates`: the card in an option only one side offers, else the first differing
`Cards[]` field other than `Pos`, else the source of the last action that has one). It is an upper bound: a shared
mechanism blames its victims. `kfuzz` needs the corpus: run it where `.cards` is linked or pass `-cards DIR`.

On a pool (`-kernel fdn`, `blb` or any registered pool) `kfuzz` exits:

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

Bad flags exit 2. `-absolute` prints `absolute: OK` or `absolute: FAIL: <what>`; it, `-pairs-file` and `-evidence`
belong to pool runs and exit 2 elsewhere. The `vanilla` and generic `gorge`/`gorge-api` paths also exit 1 on a divergence
or a hang.

Flags worth knowing (`kfuzz -h` has the rest):

- `-game-timeout` (default 30 s, 0 is off) records a game with no result in that time as a `hang` with its decks and
  seed. A hung goroutine cannot be stopped, so the first hang ends the segment and kfuzz re-executes itself for the
  remaining pairs; this fails the run even when the segment is clean. The hang line's `rerun with: <cmd>` replays one
  pair. File flags (`-coverage`, `-evidence`, `-reach`, `-out`, profiles) are inherited, so the last segment's file wins.
- `-surface autopay|full` picks the option surface (default `full`; `-autopay-surface` means `-surface autopay`).
- `-pairs-file F` plays `deckA<TAB>deckB[<TAB>seed]` lines instead of drawing from `pool.tsv`.
- `-mulligans N` is the London mulligans each player may take (default 0 skips the round); it changes every game's stream.
- `-coverage F` and `-evidence F` count clean games only. `-evidence` writes the cards with at least
  `oracle.EvidenceMinPlays` plays and every own ability key used, leaving out cards refused by name; it plays unverified
  cards so they can earn evidence. `-allow-unverified` plays them without writing a list.
- `-implicates-all` names every implicated card and prints one `bad-game` line per game that fails `-absolute`;
  `scripts/dirac-first-refusals.py settle <out>` reads them, from a `-play-refused` run, to find the refused list that
  leaves the corpus clean.
- On a deck pool's 1500-pair slice, `-absolute` also fails under `oracle.GateMinGames` started games
  (`oracle.SliceMinGames` on a committed slice), so refusals cannot shrink a gate without an edit saying so.

`scripts/dirac-evidence.sh <out>` plays the whole evidence corpus and `scripts/dirac-first-refusals.py evidence <out>`
writes `dirac/support/evidence/`, the floor met across all of a card's runs.

**Stall classes.** `dirac.StallClassOf` splits a verdict's stalls. `capacity`: a fixed array or budget ran out and gorge
has no such limit; the game was correct up to there, is always counted, never fails, and earns coverage.
`unmodelled`: gorge does something dirac does not; these fail `-absolute`, as does a reason nobody classed.
`TestRuleGapSitesKeepTheirOwnReason` keeps a rule gap from stopping under a capacity reason's name.

**Refused pairs.** A pair whose cards dirac cannot play together is refused before the game as `refused-pair`, when
`Matchup` builds its table (`cardset.InteractionRefusal`, naming the carriers); the `refused-pair:` line counts them by
reason. `Init` keeps the same check as a guard for a hand-built table, and `TestInteractionRefusalMatchesInitStall` holds
the two together.

### Synthetic evidence pools

The two real deck pools leave most of what kgen claims unplayed, so `kfuzz -synth <pool>` plays a card list instead.
`make dirac-tables` writes one table per list under `<corpus>/kernel-synth-tables/<key>/` (`kgen -synth-pools`); nothing
is committed, and kfuzz deals the decks in memory.

- `std-<set>`: one set list of `dirac/cardset/standard.txt`, front-face names, basics added.
- `tail-NN`: lowered cards outside those lists, `cardset.TailBucket` to a pool in FNV-1a order. `tail-unlowered.txt`
  beside the tables names the rest of the corpus.
- `dr-<code>`: a draft pool of `dirac/cardset/draftpools.txt`.
- `fx-<set>`: a focus pool, `kfuzz -focus` over a list in `dirac/support/focus`, dealt around the cards that list names.
- A list too big for one table (the subtype masks hold 192) or over `cardset.DraftSplitCards` is split in two by name hash
  (`<pool>-a`, `<pool>-b`; drafts split into `-aaaa` ... `-bbbb`). Cards on opposite sides never meet, and a split's
  evidence counts only when every one of its games is clean.

`oracle.SynthPool` deals `oracle.SynthDecks` 40-card decks from `-seed`: two colours, 23 spells from a shuffled queue
(at most two of each), up to 4 nonbasic lands, basics to 40. A card of three or more colours at the head of the queue
makes the deck take its colours, or it would never be dealt. The evidence run leaves out the cards refused by name, so no
pair is lost to one; the run that settles the refused list (`-play-refused`) deals every lowered card, which keeps a
refused entry's repro pair fixed. A deck name (`std-dsk_00017_WU`) and `-seed` reproduce a game:
`kfuzz -synth std-dsk -first-pair N -pairs 1 -seed 1`.

```sh
bin/kfuzz -synth std-dsk -first-pair 0 -pairs 1500 -seed 1 -keep-going -surface autopay -shrink 0 -workers 4 -reach reach.tsv
```

A synthetic run prints how far its list got (`reach <pool>: N cards in the list, ... evidence (P% of the list, over G games)`).
Basics are not counted; `appeared` counts every game that started; the last two figures count clean games only, and
evidence also needs every own ability key used. `-reach F` writes one row per card: `not-lowered`, `refused`,
`not-dealt`, `no-game`, `few-plays`, `keys-unused` or `evidence`. Synthetic decks are not human decks: per-game rates
overstate what a real pool sees, while a finding about a card holds either way. A game-start stall spends the whole game.

### Four-seat pods

dirac plays pods of three or four seats in lockstep with gorge under the same comparison. `-seats 3` or `-seats 4`
plays pods of distinct decks (`oracle.PodAt`), `-pods N` of them, on `-kernel vanilla`, a `-synth` pool or a Commander
pool cut with `-cmd-mini`:

```sh
bin/kfuzz -kernel cmd5 -seats 4 -cmd-mini 60 -pods 100 -seed 1 -keep-going -shrink 0
```

A pod run prints a pod line after the verdict. The `TestFourSeat*` tests in `dirac/oracle` hold pod play in lockstep on
fixtures, one mechanic at a time. `make dirac-pods` runs the `cmd5` and `edh` pods as a census, not a gate.

A card whose lowering reads the one opponent of a two-seat game (`kop.CardTwoSeat`) plays two seats only: a game of three
or more holding it is refused at its start under `StallTwoSeat` (`dirac/stallclass.go`; `MatchupN` refuses the pod,
`InitN` stalls a hand-built table). That is a declared limit, counted, not a divergence. `TestCommanderPodTwoSeatCensus`
counts which Commander precons such cards hold to two seats, by class, against the goldens in
`dirac/oracle/testdata/twoseat-census/`. `kgen -supported <pool> -four-seat` answers false for such a card.

## Lockstep, the core check

`oracle.Lockstep` (`dirac/oracle/lockstep.go`) plays one game with gorge leading; `oracle.LockstepLed` lets either side
lead. Both sides get the same decks and seed; a policy answers the leader's canonical decision and the follower mirrors
it. Every side implements `Side` (`dirac/oracle/canon.go`): `Step`, `Apply`, `Project`, `Terminal`, `Decided`, `Dirty`,
`Turn`, `Err`.

Compared, in order:

1. **The canonical option list** at every decision. Options are sorted `Canon` values (grammar in
   `dirac/oracle/doc.go`), so the comparison catches a missing, extra or differently priced play.
2. **The decision kind and player**: a priority decision must not become a declaration, and the wrong seat must not act.
3. **The Projection** after every action gorge commits (`dirac/oracle/project.go`): per instance its card, owner,
   controller, zone, position, flags, damage, counters and attachments; then life, poison, speed, the stack, turn, step,
   active player, priority holder, RNG draw count, lands played, mana pools and the result. Fixed arrays, so equality is
   a value comparison.
4. **The terminal result** (`over`, `winner`) when both sides stop.

A game that hits `-max-turns` or `-max-decisions` is a stall, not a divergence, and counts as clean. A **dirac capacity
stall** (`oracle.KernelStallPrefix`, the `kernel ` prefix in the verdict) agreed up to where it stopped and earns coverage
for that much; one stopped at an unmodelled reason earns none (`Result.StallUnmodelled`).

One capacity stall is gorge's: a trigger that reached the engine's hang guard (`maxTriggerFires`,
`engine/rules/trigger_match.go`) stops matching, and dirac does not count fires. When a game would be reported
divergent and `rules.Engine.TriggerCapReached` holds, it ends as a stall named `oracle.GorgeTriggerCapStall`, counted
under the capacity reasons and printed with `-v` and `-absolute`.

### How a divergence is reported

`oracle.Divergence` (`lockstep.go`) records the decision number (`At`), what disagreed (`What`: `options`, `projection`,
`decision`, `terminal`, `gorge` or `kernel`), the first mismatching field (`Field`, e.g. `Cards[12].Zone`), every
mismatching field with both values (`Diff`), both option lists and the side that led. `kfuzz -v` prints the last actions
before it and the field diff.

### Reproducing one

The engines are deterministic: the same `-seed`, `-first-pair` and `-pairs` replay the same games byte for byte.
`-shrink N` (default 200; 0 is off) shortens the action prefix while it still diverges.

`oracle.Repro` (`dirac/oracle/repro.go`) is the portable form: the two decks by card name, the seed, the run knobs and
the canonical action prefix. `-out DIR` writes it and `-repro FILE` replays it with no deck files. The pool paths write
the first divergence as `DIR/repro.json`; the `vanilla` path writes up to three files per divergence class; a clean run
writes nothing. `-repro` takes the surface from the command line, since a repro does not record it.

For a game dirac played alone, `oracle.RecordKernelGame` makes a `koracle-game-2` record, and `kfuzz -replay FILE`
re-executes it in gorge with its chance draws forced and verifies gorge's log. `-replay` is routed before the `-kernel`
switch, so it works under any `-kernel`.

## The oracle import boundary

Only `dirac/oracle` and its commands, `dirac/kgen`, `dirac/adapter` (view, decision and card types only), `dirac/shadow`
(`engine/view` only), `dirac/lab` (`engine/deck` only), `dirac/neteval` (gorge's network packages only), `dirac/cmd/dplay`
and `dirac/cmd/sbv2dirac` may import gorge's packages; the table is in [dirac/README.md](../README.md#import-boundary).
The core (`dirac`, `dirac/kop`, `dirac/vanilla`, `dirac/cardset`, `dirac/search/...`, `dirac/feat`) must not, so its
`State` cannot hold a gorge pointer or map and dirac builds without the gorge engine except through the oracle.
`internal/archtest` holds this (`TestDiracCoreImportsNoGorge`, `TestAdapterLinksNoRulesEngine`).

## Following a gorge change

gorge and dirac are one Go module, so `dirac/oracle` always plays the gorge in the same tree. gorge leads. A change that
alters what a supported card does fixes dirac in the same change, or makes kgen refuse the affected cards by name
(`kgen -refuse`, see the [glossary](glossary.md)), which leaves the catch-up visible. `scripts/dirac-gate.sh <base>` holds
this. After such a change:

- re-run lockstep: the oracle's behaviour may have changed;
- update [`GORGE-FINDINGS.md`](GORGE-FINDINGS.md) where a copied behaviour is now fixed or newly required;
- update `dirac/crcoverage` rows that say dirac inherits gorge's behaviour, when gorge's row moved;
- rebuild the tables with `make dirac-tables`: a change in the gorge packages kgen builds against moves the table key.
  If the accepted counts moved, re-pin `dirac/kgen`'s acceptance rows
  (`dirac/kgen/testdata/acceptance.tsv`, `build_acceptance.tsv`) and the support doc:

  ```sh
  make test-one PKG=./dirac/kgen RUN='TestCorpusAcceptance|TestSetAcceptance|TestCorpusBuildAcceptance|TestSupportDocIsCurrent' TESTENV='KGEN_REPIN=1 DIRAC_UPDATE_SUPPORT=1'
  ```

  A re-pin is not a parity fix and should follow a real count change. A lower accepted count fails unless
  `KGEN_REPIN_FALL=<reason>` is also set. By default the ratchets fail only on a fall; `DIRAC_PIN_STRICT=1` (set in the
  nightly run in `.github/workflows/ci.yml` and in main's regeneration scripts) holds every pin exact.

## Bot and transcript parity

`khbot` (`dirac/oracle/cmd/khbot`) compares dirac's port of gorge's auto-pay heuristic bot with the bot itself. gorge's
bot chooses every action on gorge; at each decision dirac's bot is asked the same decision on dirac's state
(`oracle.HBotParityGame`, `dirac/oracle/hbot_parity.go`). It prints agreement per decision kind and one example of each
disagreement class:

```sh
go run ./dirac/oracle/cmd/khbot -pairs 1 -workers 1
```

`ksbt` (`dirac/oracle/cmd/ksbt`) does the same for SpellBench's sb-tactical: gorge's decision core answers a gorge view
built from the dirac position (`oracle.SBTParityGame`, `sbt_parity.go`; the view is `sbt_view.go`, the agent `sbt.go`).
It is an agent, not a hot-loop policy, because the gorge view allocates.

`kfuzz -v2 transcript` plays the same seeded games on `dirac/cmd/sbv2dirac` (dirac hosted as a SpellBench v2 engine) and
gorge's environment and diffs the transcripts. `-v2-no-chance` is the negative control: the gorge host keeps its own
shuffles instead of dealing dirac's chance, and the transcripts must differ. `v2bridge.go` maps an agent's native v2
option back to the canonical dirac action.

## Adding a card: the parity test shape

A new card or mechanic gets a pool (`dirac/cardset/pools/fixtures/<name>/cards.txt`, built by `make dirac-tables`) and a
lockstep test in `dirac/oracle`. The smallest complete model is `TestAltcostLockstep` (`dirac/oracle/altcost_test.go`):
two 40-card decks (`kaltcostDecks`) and one call to `lockstepSet(t, "altcost", kaltcostDecks, 1000, "KALTCOST_SEEDS")`
(helper in `dirac/oracle/ward_test.go`). The shape:

- add the pool list so `cardset.ByName` finds it, with a comment naming the mechanic and the CR rules the fixture reaches;
- give both decks 40 cards, chosen so the mechanic occurs (the comment says how);
- call `lockstepSet`: it plays `seeds` games, mixes policies by seed (`seed%4 == 0` on gorge's auto-pay surface,
  `seed%8 == 0` random, otherwise cast-first) and fails on every divergence; the environment variable named by the last
  argument overrides the seed count for a quick run;
- add a mutation test (`TestProwessLockstepMutation`, `dirac/oracle/prowess_test.go`) that flips a dirac debug knob and
  proves some seed diverges. Without it a green lockstep can mean the card was never exercised.

Run one focused set with a small seed count:

```sh
make test-one PKG=./dirac/oracle RUN='^TestAltcostLockstep$' TESTENV='KALTCOST_SEEDS=8'
```

## Comprehensive Rules coverage

`dirac/crcoverage` is the rule-by-rule audit: one Markdown file per CR chapter, one row per numbered rule, with a status
(`follows`, `partial`, `deviates`, `not-impl`, `definition`, `n/a`, `unknown`), the code that decides it, the tests that
exercise it and a note. dirac plays at most `MaxSeats` seats and plays fixed pools, so many rows are `n/a` with the reason
stated. [dirac/crcoverage/README.md](../crcoverage/README.md) explains how to read and update a row and holds the
current count of each status.

`TestCRCoverageIsCurrent` fails when a row stops matching the code, so a behaviour change updates its row in the same
commit. `KERNEL_UPDATE_CRCOVERAGE=1` regenerates the README's summary table.

## Speed

`BenchmarkTransitionFDN`, `BenchmarkTransitionBLB` and `BenchmarkTransitionFDNPool` in `./dirac` play real decks on one
core and report no allocations. [running.md](running.md#performance) has the command and how to compare two commits.

## Differences from gorge

[`DEVIATIONS.md`](DEVIATIONS.md) lists the differences that are not divergences, each classed as canonical surface or
declared capacity. [`GORGE-FINDINGS.md`](GORGE-FINDINGS.md) lists the gorge behaviour dirac copies and the differences
still open. Both are generated or test-held.
