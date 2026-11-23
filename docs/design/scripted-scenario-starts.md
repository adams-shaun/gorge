# Scripted scenario starts for the dirac lockstep

Status: design, not implemented. Base `origin/main` `b527652de9` (2026-11-23); every `path:line` is at
that commit unless marked fleet-repo. Anything not read from code is marked INFERRED.

## Why

evloop credits 5.3 cards/h against a 100/h target (`fleet/evloop/status.json` at 2026-11-23T10:47:
`credited_per_h` 5.3, `credited_24h` 127, `games_24h` 1,264,594, `core_min_per_card` 26.5, `need` 252,
`blocked` 176; the brief's 244/172 were an earlier read). Its 11-19 census of 218 stuck cards (fleet-repo
`ops/fleet/evloop/README.md:143`, commit `734f5a0a6d`) puts 110 in "cast, but an ability key never used". A random
deal cannot fix that: 64 need an enabler, 14 a board or combat state, 12 a morph turned face up, 5 a threshold
and 10 commander play. The scripted decks (`scripted/*.txt`, README:126) only shape the deck list. Games still
start from a shuffle (`engine/rules/genesis.go:526`, `dirac/engine.go:123-131`), and
`dirac/oracle/featscen.go:15` says it directly: "dirac has no position builder".

What already exists and is reused:

- gorge's oracle scenario runner already builds positions through events: `oracleScenario`/`oracleSeat`
  (`engine/rules/oracle_run.go:48,73`), `build` (`:699`), placement (`oracle_setup_attached.go:131`,
  `oracle_setup_triggers.go:124`). It returns the hands to the library, moves the named cards out, then emits
  `LibraryOrder` (`oracle_run.go:852`), `LifeChange`, `Tap`, `CounterChange`, `FlipFace` and `SetupSettled`
  (`engine/events/event.go:1235-1246`, folded at `apply.go:374`). It runs *outside* `rules.New`, so
  `engine/replay` (which replays from `rules.New(cfg)`, `engine/replay/replay.go:193-195`) cannot rebuild it.
- dirac's test-only builder (`dirac/scenario_test.go:32-58`) uses the internal moves `enter`/`place`
  (`dirac/zones.go:466,164`), clears `pfSick` and drops the pending queue (`npend = 0`).
- The lockstep projection holds library position (`dirac/oracle/project.go:78`, `Pos`), so comparing at
  decision 0 checks library order too.

## Decisions

| # | Decision | Rejected alternative |
|---|---|---|
| D1 | The start is **genesis configuration**: `rules.Config.Start *rules.StartSpec`, applied inside `genesisDeal`. Replay from `(Config, intents)` reproduces it, and the setup events sit in the hash chain. A nil `Start` is byte-identical genesis, so no golden head moves. | The oracle runner's out-of-`New` setup. `engine/replay` cannot rebuild it. |
| D2 | **No new event kind.** Setup uses the kinds already listed above. `events.Kind` stays as it is. | A `ScenarioPlace` kind. Not needed. |
| D3 | **Ordinary shuffle + deal, then setup.** Each seat's final library order follows one closed rule that both engines implement: the listed `library` cards top first, then the post-shuffle order minus every placed object, then the returned unlisted opening-hand cards in hand order. gorge emits it as one complete `LibraryOrder`. The RNG stream is unchanged until play begins, and the unlisted remainder varies with the seed. | A stacked-deck genesis that skips the shuffle. It changes the RNG stream and adds a second genesis path. |
| D4 | **Every trigger queued by setup is dropped on both sides.** A placed permanent counts as if it had been there since before the game (CR 302.6, `SetupSettled`). The XMage-parity rule in `placeSetupCard`, which keeps CounterAdded triggers, is not copied. gorge truncates `e.pendingTriggers`, dirac sets `npend = 0`. | Sharing `placeSetupCard`. That couples the gorge-compliance runner to this feature and keeps a trigger asymmetry dirac would have to mirror. |
| D5 | **No shared code with the compliance runner in v1.** One JSON vocabulary only: the scenario's `setup` block uses `oracleSeat`'s field names (`hand`, `battlefield`, `tapped`, `back_face`, `graveyard`, `exile`, `library_top`, `counters`, `life`, `attached`, `commanders`). | Refactoring `oracleRun` into Engine methods. That risks the byte-identity of gorge-compliance's scenarios. |
| D6 | **Turn and phase come from play, not from setup.** The start is always the beginning of turn 1 with the chosen seat active. A later `turn`/`phase` is reached by the focus policy passing through, in lockstep, so every step is compared. Draws on the way come from `library`. | Writing `Turn`/`Step` on both sides. That would need dirac step-entry bookkeeping and adds risk. |
| D7 | **The active player is chosen by seed.** The toss is the first draw, `IntN(seats)`, in both engines (`genesis.go:390-401`, `dirac/engine.go:15-17,123`). The loader takes the first seed ≥ the run seed whose toss is the wanted seat (the `seatZeroStartSpare` pattern, `oracle_run.go:2884`), then checks `StartingPlayer` after `New`. | A starting-player field. Not needed. |
| D8 | **dirac: no new State bytes.** Setup writes only existing fields: objects, perms (`pfSick`, tapped, counters, face), the players' life, `pl.lib`. The start ops are an `Init`-time argument and are never stored. `TestStateSize` (`dirac/kernel_test.go:48`, 49 KiB) and `TestColdArraysHoldNoPointers` stay as they are. | |
| D9 | **No manual mana activations in v1.** Mana keys are credited when the payer taps the source (`ArmCoverage`, `dirac/oracle/coverage.go:141-157`). A scenario forces this by making the focus card the only source of a colour a spell in hand needs. A key the planner can never pick is a surface gap and is filed, not scripted. | A new canonical action kind in both engines. |
| D10 | **Scripted games credit ability keys, never plays** (see Evidence). | Counting them like organic games. |

## 1. Scenario format

The file is JSON, one scenario per file, stored as `<card-slug>.<n>.json`:

```json
{"format":"gorge-start-1","focus":"Helix Pinnacle","keys":["f0/a0","f0/t0"],
 "players":2,"game":"constructed","active":0,"turn":1,"phase":"main1","max_turns":12,
 "filler":"basics","opponent":"explore",
 "setup":{"p0":{"hand":["Helix Pinnacle"],"battlefield":["Helix Pinnacle","Forest","Forest","Forest"],
                "tapped":[],"counters":{"Helix Pinnacle":{"TOWER":99}},
                "library":["Forest","Forest"],"graveyard":[],"exile":[],"life":20},
          "p1":{"battlefield":["Grizzly Bears"]}}}
```

- `keys` are coverage inventory keys `f<face>/a<i>` and `f<face>/t<i>` (`coverage.go:68`). The loader rejects a
  key the card does not have.
- `library` is the top of the library, top first (D3). The rest of a 40-card deck is `filler`: `basics` (basics in
  the focus card's colours) or `deck:<name>` (a pool deck file).
- `phase` is one of `upkeep|main1|combat|main2|end`. `opponent` is one of `explore` (`ExplorePolicy`,
  `dirac/oracle/policy.go:96`), `heuristic` or `passive` (pass whenever legal).
- `game: commander` requires `players: 4` (operator rule: Commander is four players, never fewer).
  `commanders` names a seat's commander.
- Deferred to v2 (T11): `face_down`, `tokens` and an Equipment `attached`. The morph class does not need
  `face_down` (§6).
- The loader refuses, naming the card:
  - a card the scenario's table cannot hold;
  - a battlefield card with an as-enters choice. gorge would park it (`landEntryParked`,
    `oracle_setup_triggers.go:164`) where dirac has no matching state;
  - a focus key whose ability reads history that placement cannot fake (cast-from, kicked, entered-this-turn).
    The kgen IR flags for this are INFERRED, and T2 finds them;
  - a seat with more than `MaxDeck` (100) cards (`dirac/state.go:14`).

## 2. gorge: building the start (`engine/rules/start.go`, new)

- `type StartSpec struct { Seats []StartSeat }`. A `StartSeat` holds slices only, never a map:
  `Hand, Battlefield, Graveyard, Exile, Library []int` (indices into `Config.Decks[p]`), `Tapped, BackFace []int`,
  `Counters []StartCounter{Card int; Kind string; N int32}` sorted by (Card, Kind), `Attach []StartAttach`, and
  `Life *int32`. Slices keep the order deterministic (invariant 3) with no map range.
- `Config.Start` sits beside `Sideboards` (`engine/rules/engine.go:24-40`). The hook is in `genesisDeal`, after
  the starting-player fold (`genesis.go:595`) and before `startPostDealSetup` (`:612`). It runs only when
  `cfg.Start != nil` and `Mulligans == 0` (the loader enforces mulligans 0). The opening-hand round (Leylines)
  then reads the scripted hand in both engines.
- `applyStart` emits, per seat:
  1. `MoveZone` hand→library for the dealt hand.
  2. A `MoveZone` from the library to each placement's zone. `back_face` puts a `FlipFace` first, as
     `placeSetupEntry` does (`oracle_setup_attached.go:136-156`).
  3. `Tap`, then `CounterChange`, then `SetupSettled` for each battlefield card.
  4. One complete `LibraryOrder` (D3).
  5. `LifeChange`.

  Then it truncates `e.pendingTriggers` and `orderedTriggers` (D4) and sets `e.cmdZoneScan` as `build` does
  (`oracle_run.go:873`).
- `rules.SeedForStarter(seed uint64, seats, want int) uint64` (D7).

## 3. dirac: the same start (`dirac/start.go`, new, out of line)

- `func (s *State) InitNStart(tbl *Table, decks [MaxSeats][]CardID, n uint8, seed uint64, cfg Config, st *Start)`
  is `InitN` (`dirac/engine.go:34`) with one call added between the per-seat deal loop (`:124-135`) and
  `s.startPl`/`beginTurn` (`:136-147`). `Start` uses the same indices as `StartSpec` and is built from the
  same scenario by the loader (§T2). It is never stored in `State`.
- The ops are `toLibrary`, `place(o, zone, tapped, backFace)` (through `enter`/`place`, `zones.go:466,164`;
  `enterTransformed`, `face_transform.go:34`), `counters` (the existing counter writer), `settle` (clear `pfSick`),
  `libOrder(p, objs)` and `life`. After them, `npend = 0`.
- Object numbering matches with no extra work. Both sides lay out the decks the loader built, in seat order
  (`Matchup.InstanceCard`, `dirac/oracle/lockstep.go:267-277`), so a scenario ref is the same instance on both.
- Wiring: `Matchup.Start` (a new field, `dirac/oracle/setup.go:20-71`) is copied into `rules.Config.Start` by
  `Matchup.Config` (`setup.go:497`), which `NewGorgeSide` builds from (`gorge.go:274-291`). `RealKernel`
  (`realkernel.go:132`) calls `InitNStart` when `m.Start != nil`. Every lockstep path then starts from the
  same state: `Lockstep`, `LockstepLed` (`lockstep.go:305,318`), replay and repro.
- Parity is checked at decision 0. The first `Project` compare already covers the board, library `Pos`, life
  and counters (`project.go:78`). A mismatch at decision 0 is classed `setup`. It is filed against the harness,
  never the card, and it stops the scenario, not the card.

## 4. Focus policy (`dirac/oracle/focus_policy.go`, new)

`FocusPolicy(base Policy, f FocusSpec) Policy` keeps the contract in `policy.go:19-24`: it is pure over its own
counters and the context, and a run replays from its action list. `FocusSpec` holds the focus seat, the focus
instance refs, the wanted keys, and the pass-through `(turn, phase)`.

At a focus-seat decision, the policy picks the first of these that is offered:

1. While `(turn, phase)` has not been reached: `ActPass`, or `ActDone`/the first option when passing is not offered.
2. `ActActivate` with a focus source and `mode` = the ability index of a wanted `a` key (`dirac/canon/canon.go:37`).
3. `ActSpecial` with a focus source (turn face up).
4. `ActCast`/`ActCastFace` of a focus card from hand, including a face-down morph cast.
5. `ActPlayLand` of a focus land.
6. At `attackers`: every focus creature (attack triggers).
7. `ActYes` with a focus subject (optional triggers).

Otherwise it defers to `base`. Each wanted key is picked at most 3 times a game, then `base` takes over, which
prevents activation loops. Wanted `t` keys are reached through scenario design (the phase, attacking, a board
that meets the condition), not picked directly. The opponent seat plays `opponent`.

## 5. Evidence semantics

Today a card is evidence when it reaches ≥3 casts or land plays and uses every own key in clean organic
games (`dirac/oracle/evidence.go:11,14,74`). gorge's writer counts only those (`scripts/dirac-first-refusals.py:43`,
`merged_coverage` `:302`).

Proposed rule, for runs named `sc-<pool>-NNN`:

- A scripted clean game adds key uses (`Used`) and never adds `Cast` or `Land`. Setup itself credits nothing:
  `CoverageOf` credits casts from `PutOnStack` and land plays from `LandPlayed` (`coverage.go:215,230`), and
  setup emits neither. Setup also leaves no ability objects behind (D4).
- A card is credited when:
  - (a) its plays floor (3) is met by organic clean games (its existing `evidence-keys` row or this cycle's
    organic runs);
  - (b) every key that organic games never used was used in at least `EvidenceScriptedMinGames` = 3 clean
    scripted games over at least 2 distinct seeds;
  - (c) no bad game in any of its scenarios implicates it (the existing hold rule).
- Credits land as plain rows in `dirac/support/evidence/sc-<pool>.txt`, which supports them like any list
  (`dirac/kgen/support.go:38`). The sidecar `evidence-keys/sc-<pool>.tsv` adds two columns: the `scenario`
  sha and `organic_plays`.
- Soundness: evidence claims that both engines agreed on every decision and board across games that reached
  the card's behaviour. A scripted game still meets that claim from decision 0 to the end, and decision 0 is
  itself compared. Two things are given up:
  - Organic games prove the position was reachable through play. A placed board is legal but may be
    unreachable in 40-card play (99 tower counters).
  - A placed permanent has no history. Keys that read history are refused in §1.

  Requiring the plays from organic games keeps "this card is cast and resolves identically" on organic ground.
  Scripted games only fill in keys.
- Pod scenarios (commander) write pod rows, which are report-only today (README:31-33, OD-4 stage 1). They
  credit nothing until `EvidencePodMinPlays` lands.

## 6. evloop integration (fleet-repo `ops/fleet/evloop`)

A stuck card's class comes from its pulled coverage, reach and bad-game rows, the same way the 11-19 census
did. Each class has its own generator in `scenarios.py` (new):

| Class (census count) | Generated start | Source of the board |
|---|---|---|
| enabler-needed (64: 20 snow, 22 named card/type, 22 token/mechanic maker) | focus in hand and one on the battlefield, the enablers the existing `scripted/*.txt` selectors pick placed (permanents) or in hand (spells), 6 untapped lands of its colours | selectors over every pool (`pool_index`). The table is widened with `synthWithSet` (`dirac/oracle/cmd/kfuzz/synth.go:53`), so the "22 of 74 spec cards resolve" limit (README:136) goes away |
| board/combat (14) | hand-written, `scenarios/<slug>.json` (T10) | per card |
| morph face-up (12) | focus in hand; lands for {3} plus its turn-up cost; opponent creatures none | policy rules 4 then 3 |
| threshold (5) | focus on the battlefield with counters one short (Helix Pinnacle 99 TOWER) | hand-written. Battle of Wits needs 200 library cards > `MaxDeck` 100: declared out of scope, not counted |
| commander-only (10) | 4-seat commander pod, focus as commander | report-only until OD-4 stage 2 |
| forecast (5) | none: "0 of 9 played forecast cards ever used it" (README:153) is an option-surface gap; file it | |

- Runs: per scenario, surfaces autopay and full × `EVL_SC_SEEDS` (32) seeds × mulligans 0, with `max_turns` 12.
  That is `kfuzz -scenario <dir> -coverage -branches -evidence` (T7). A stuck card re-enters once when its
  scenario file is newer than its verdict, which is the existing `retry-scripted` rule (README:132).
- Throughput (INFERRED from today's mean, since no scripted game has been timed):
  - Cost: 3,362 core-min / 1,264,594 games (status.json) = 0.16 core-s a game. 64 games a scenario × up to
    3 scenarios = 192 games ≈ 31 core-s ≈ 0.5 core-min a card, against 26.5 today.
  - Compute is not the limit: at `EVL_MAX_WORKERS` 24, about 2,800 cards/h.
  - The limit is the addressable set. 90 of the 218 census cards (64 + 14 + 12) fit; 4 of the 5 thresholds fit
    after T10. If the census's divergence share holds (32/218 = 15%), about 75 are credited.
  - They arrive as one burst. The first cycle after T9 plays them, and the hourly land PR merges them, so the
    burst arrives within about 2 h. After that the rate falls to the inflow of new stuck cards in these classes.
  - Scenarios also turn hidden divergences into tickets. They count as `blocked`, not credited.

## 7. Tickets (Sonnet implementers; in order)

Every gorge ticket rebases and runs `scripts/pr-precheck.sh origin/main` before it pushes. A worktree links
`.cards` and checks it resolves (AGENTS.md).

| # | Ticket | Files | Done means | Cx | Deps |
|---|---|---|---|---|---|
| T1 | gorge `StartSpec`, `Config.Start`, `applyStart`, `SeedForStarter` | `engine/rules/start.go`, `start_test.go`, `engine.go`, `genesis.go` | `make test-one PKG=./engine/rules RUN='^(TestScriptedStart.*\|TestHeads\|TestSetHeads)$'` PASS with heads unmoved. `make test-one PKG=./engine/replay RUN='^TestScriptedStartReplay$'` PASS (replays a 3-turn game exactly) | high | - |
| T2 | scenario loader and validator, `Matchup.Start`, deck layout, refusals in §1 | `dirac/oracle/scenario.go`, `scenario_test.go`, `setup.go` | `make test-one PKG=./dirac/oracle RUN='^TestScenario(Parse\|Validate\|Layout)'` PASS | medium | T1 |
| T3 | dirac `InitNStart`, start ops | `dirac/start.go`, `start_test.go` | `make test-one PKG=./dirac RUN='^(TestInitNStart.*\|TestStateSize\|TestColdArraysHoldNoPointers)$'` PASS with the State size unchanged | high | - |
| T4 | `RealKernel` wiring plus a decision-0 parity test over 6 fixture scenarios on the FDN table (placement, tapped, counters, back face, Aura attached, library order, active seat 1) | `realkernel.go`, `dirac/oracle/testdata/scenarios/*.json`, `scripted_start_lockstep_test.go` | `make dirac-tables` then `make test-one PKG=./dirac/oracle RUN='^TestScriptedStartLockstep$'` PASS (every fixture clean over 8 seeds) | high | T2, T3 |
| T5 | `FocusPolicy` | `focus_policy.go`, `focus_policy_test.go` | `make test-one PKG=./dirac/oracle RUN='^TestFocusPolicy'` PASS (picks the key when offered, passes through to (turn, phase), defers, and is deterministic over a replayed action list) | medium | T2 |
| T6 | repro and replay carry the start (`Repro.Start` inline; format `koracle-repro-3` only when present) | `repro.go`, `replay.go`, `repro_wire.go` | `make test-one PKG=./dirac/oracle RUN='^TestScriptedStartRepro'` PASS (write, read, replay, same verdict) | medium | T4 |
| T7 | `kfuzz -scenario <dir>` (runs `sc-<name>`, `.coverage`, `.evidence`, `.verdict`) | `dirac/oracle/cmd/kfuzz/{main.go,scenario.go}` | `make test-one PKG=./dirac/oracle/cmd/kfuzz RUN='^TestScenarioRun'` PASS | medium | T4, T5, T6 |
| T8 | evidence rule §5: `EvidenceScriptedMinGames`, writer support for `sc-` runs, sidecar columns. **Operator sign-off before merge** | `dirac/oracle/evidence.go`, `scripts/dirac-first-refusals.py`, `scripts/dirac_evidence_scripted_test.py`, `dirac/kgen/support_test.go` | `python3 -m unittest scripts/dirac_evidence_scripted_test.py` OK. `make test-one PKG=./dirac/kgen RUN='^TestSupport'` PASS | medium | T7 |
| T9 | fleet-repo evloop: class generators, `sc-` runs, `status.json` `scripted_credited_24h` | fleet-repo `ops/fleet/evloop/{scenarios.py,evloop_box.py,evloop.py,test_scenarios.py}` | `python3 -m unittest ops/fleet/evloop/test_scenarios.py` OK. One dry cycle on the box shows `sc-` runs in `scripted.tsv` | medium | T7 (credit: T8) |
| T10 | hand-written scenarios: 14 board/combat, 4 threshold | fleet-repo `ops/fleet/evloop/scenarios/*.json` | each loads (`kfuzz -scenario <file> -pairs 1`), 0 `setup` divergences | low | T7 |
| T11 | v2 placement: `face_down`, `tokens`, Equipment attach. Only if T9's `scripted.tsv` lists cards that need them | `start.go` (both), `scenario.go` | the T4 test grows fixtures for each, PASS | medium | T4 |
| T12 | commander pod scenarios, 4 seats | `scenario.go`, the T9 generator | the T4 test covers a 4-seat commander fixture, PASS. Credits wait on OD-4 stage 2 | medium | T4, OD-4 |

T1 ∥ T3 can run in parallel. Then T2, T4, then T5 ∥ T6, then T7, T8 and T9 in that order. T10 needs only T7.
That is 12 tickets: 10 core and 2 optional (T11, T12).

## 8. Risks

- **Placement semantics drift** between gorge's `MoveZone` fold and dirac's `enter`: entry replacements, ETB
  counters, static registration. The decision-0 compare catches it as `setup`. The T4 fixtures pin the common
  shapes, and the §1 refusals keep out the shapes known to differ.
- **History-less permanents** could make a key look agreed while history-reading code goes untested. §1
  refuses those keys, and the refusal predicate is INFERRED until T2.
- **Coverage leakage**: setup must credit nothing (§5). The T7 test asserts zero `Used`/`Cast` for a game that
  ends at decision 0.
- **gorge-compliance**: D5 leaves `oracle_run.go` untouched, so the compliance scenarios' bytes stay the same.
- **The rule change itself** (§5) weakens "organic only". It is bounded by keeping plays organic and requiring
  3 clean scripted games over 2 seeds.

## Operator sign-off

1. The §5 evidence rule: scripted clean games credit keys only, organic plays still required, floor 3 games
   over 2 seeds. T8 waits on this.
2. `Config.Start` as a new genesis input in the rules core. It leaves heads unmoved and adds no event kind,
   but it is a new way into `rules.New`.
3. No State bytes are requested. Battle of Wits (needs `MaxDeck` > 100) is declared out of scope rather than
   raising the bar.
4. Commander scenarios stay report-only until OD-4 stage 2.
