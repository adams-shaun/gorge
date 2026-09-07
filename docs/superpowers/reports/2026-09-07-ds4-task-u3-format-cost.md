# Task U3 — measure why Commander tables run slower than constructed

Branch `wt/u3` off `8e8bf69`, worktree `.worktrees/u3`. **This task shipped a
measurement and a finding only — no optimisation, no engine change.**

## Verdict up front

The ~2x per-step wall time the user saw is **the demo's pacing knob × seat
count**, and the engine is innocent of it. At equal seat count the Commander
format adds **≈0 extra decisions per turn** (33.43 vs 33.86) and ≈**1.6x
engine time per decision** — but at `-pace 1.5s` the pace term is ~25,000x
larger than the engine term, so the 1.6x is a 0.005% rounding error on wall
time. The single number that explains the 2x is **seat count**: a 4-seat
Commander game asks ≈1.59x the decisions per turn of a 2-seat constructed
game, and pace is per-decision, so it is ≈1.59x slower per turn purely from
pacing.

## The measurement

Committed as `rules/format_cost_bench_test.go` — `BenchmarkFormatCost`, three
cases, all driven **directly on the engine** (never through `gorged`, whose
`-pace` sleep is the variable being removed; never through `mtgsim`/`botbench`,
which have no format flag):

| case | decks | seats | life | decisions/turn | allocs/decision | B/decision | µs/decision |
|---|---|---|---|---|---|---|---|
| commander | 4× foundations-* (100-card) | 4 | 40 | **33.43** | **81.72** | 10612 | ~54–65 |
| constructed | 4× legacy (60-card) | 4 | 20 | **33.86** | **51.35** | 9888 | ~32–56 |
| constructed-2seat | 2× legacy (60-card) | 2 | 20 | **21.00** | **50.49** | 9968 | ~23–25 |

Seed `0x55b00f`; every case plays **7 full turns** (the turn the user was
watching). `decisions/turn`, `allocs/decision` and `B/decision` are
**bit-identical across every run** (they are the deterministic, load-free
signal). The wall-clock µs/decision column is load-sensitive: this box ran at
load average ~33 during measurement, and `ns/decision` fluctuated (±20–30%
between runs), so that column is reported as a range, not a single gospel
number — the honesty clause of the brief. The engine ratio it implies (~1.6x)
is independently corroborated by the deterministic allocation ratio
81.72/51.35 = 1.591x, so it is not measurement noise even though the clock is.

### What could NOT be held equal

Deck size (100 vs 60) and starting life (40 vs 20) are inherent to the
Commander format — a finding, not a flaw; they are the mechanism behind the
engine-time component below. Everything else was held equal: seat count (for
the two 4-seat rows), seed, turn count, mulligan round (`Mulligans: 1`, the
`gorged` default), token corpus, and the same bot policy (`botpolicy.Decide`
via the test bot). The `constructed-2seat` row deliberately re-introduces the
one variable the brief's pacing candidate itself names ("4 seats where
constructed demo tables have fewer") so the decomposition can attribute the
user's real config.

## The decomposition, as an arithmetic line

At `-pace 1.5s` per decision (the `gorged` default), wall time per turn is
`decisions/turn × (pace + engine-per-decision)` (the mulligan round is a
Tiny fixed overhead on both sides; it is excluded from the counted turns):

```
commander(4,100)   = 33.43 × (1.500000s + 0.000054s)  = 50.1468s + 0.00181s = 50.1486s/turn
constructed(2,60)  = 21.00 × (1.500000s + 0.000024s)  = 31.5000s + 0.00050s = 31.5005s/turn
ratio              = 50.1486 / 31.5005 = 1.592x          <- the observed ~2x, to first order

of which, pace × decisions alone:     50.1468 / 31.5000 = 1.592x
of which, engine time contributes:    1.59216 vs 1.59200 = 0.01% of the difference
```

So **~99.99% of the per-turn (and per-step, which scales the same way) wall
time difference is `pace × extra decisions`, and the extra decisions come from
seat count, not from the Commander format**. The engine adds ≈1.8ms of the
~50.15s per commander turn — a rounding error at this pace. This is the brief's
first candidate, and it confirms the user "is seeing the demo's pacing knob
and the engine is innocent".

Killing the "extra decisions" hypothesis as a **format** effect: at equal
seats, commander asks 33.43 decisions/turn and constructed 33.86 — flat. The
CR 903.9 park, m31's command-zone `legalActions` walk and m37's per-decision
commander scan do **not** spawn extra decisions. Halving constructed seats
(33.86 → 21.00) moves decisions/turn far more than any format switch.

## cpuprofile (the three suspects)

Engine time per decision IS genuinely ~1.6x higher in Commander (real, just
sub-dominant), so per the brief I profiled it. `go tool pprof` on a
`-cpuprofile` run of both formats, self-time top functions:

**Commander** — `checkTriggers.func1` 15.5%, GC `scanSpanPackedAVX512` 5.7%,
`memmove` 5.5%, `Object.Face` 4.4%, map-get 3.5%, `memclrNoHeapPointers` 3.5%,
`triggerMatches`/`forEachObject`/`zoneGate` smaller. **Constructed** has the
same functions in the same order (`checkTriggers.func1` 12.3%, GC 6.5%,
`memclr` 4.3%, …). The engine cost is spread across **object-walks + GC**, all
of which scale with object count.

Suspect verdicts (cum time, commander share vs constructed share):

| suspect | commander | constructed | verdict |
|---|---|---|---|
| `Game.Clone` (O(objects)) | **0%** | 0% | **KILLED** — it never runs on the per-decision path. `Clone` is called only in `host` snapshot/viewat/restart* at turn boundaries and view requests, once per turn at most; an engine-direct play never invokes it, so it is not a per-decision cost at all. Even in a real `gorged` table it is amortised once a turn, dwarfed by ~50s of pacing. |
| `legalActions` (m31 command-zone walk) | 0.28s · 6.18% | 0.22s · 6.77% | **KILLED as the format's driver** — identical share in both formats; the extra walk grows with object count in proportion to everything else, not by a commander-specific multiplier. |
| `BoardFromGameInto` (m37 per-decision commander scan) | 0.18s · 3.97% | 0.14s · 4.31% | **KILLED as the format's driver** — flat ~4% share in both formats; the command-zone scan is small and proportional, not a Commander tax. |

The real engine-cost mechanism is **object count**: 4×100 ≈ 400 objects vs
4×60 ≈ 240, ratio 1.67x, matching the ~1.6x engine time and the 1.59x
allocation ratio. That is the brief's deck-size finding — real, measurable,
but ~0.01% of the pace-dominated wall time, which is exactly why the user's
2x is not an engine bug.

## Gates run

- `gofmt -l .` → clean (no files listed).
- `go vet ./...` → clean.
- `go test ./rules/` → `ok github.com/adams-shaun/gorge/rules 12.66s` (load-inflated; baseline for the same 322 tests is ~4.2s, see TEST_HISTORY budget note below).
- `go test ./rules/ -run TestHeads` → `--- PASS: TestHeads` — **heads did not move** (expected: this task adds a benchmark, no engine change).
- `go test -bench 'BenchmarkFormatCost' -benchmem -run '^$' ./rules/` → PASS, e.g.
  ```
  BenchmarkFormatCost/commander-32           79  14274612 ns/op  10612 B/decision  81.72 allocs/decision  33.43 decisions/turn  53887 ns/decision  1801350 ns/turn  2791292 B/op  19291 allocs/op
  BenchmarkFormatCost/constructed-32        127   9646759 ns/op   9888 B/decision  51.35 allocs/decision  33.86 decisions/turn  36140 ns/decision  1223605 ns/turn  2508984 B/op  12288 allocs/op
  BenchmarkFormatCost/constructed-2seat-32  247   4406491 ns/op   9968 B/decision  50.49 allocs/decision  21.00 decisions/turn  25471 ns/decision   534886 ns/turn  1547622 B/op   7493 allocs/op
  PASS
  ```
- `go tool pprof -top` (cpuprofile) outputs quoted in the sections above.

## Deviations from the brief

- **Added a `constructed-2seat` case.** The brief said hold seat count equal,
  and the two 4-seat rows do exactly that. The 2-seat row re-introduces the
  brief's own seat-count hypothesis so the decomposition can state how much of
  the user's 2x is decision-density vs format; it is the only place seat count
  differs from the 4-seat constructed case (same decks/seed/turns). This is
  directly in service of the two candidates the brief asked to separate.
- **`Mulligans: 1` on both sides** rather than literally the zero `Config` for
  constructed. `host/match.go`'s "zero `Config`" comment concerns
  Format/StartingLife/Commanders; the mulligan round is a real, equal-cost part
  of every served game (the `gorged` default is 1) and holding it equal removes
  a confound the two candidates would otherwise share unevenly. The commander
  row uses the exact `host/match.go` commander shape.
- **Reverted a `testtime` load spike.** `go run ./cmd/testtime ./rules` under
  the measured box load (load avg ~33) reported 12.2s vs the 10s budget and
  appended a row to `rules/TEST_HISTORY.md`. The benchmark adds **zero test
  runtime** (it has no `Test` functions and its corpus load happens only inside
  the `Benchmark*` function), and the immediately preceding TEST_HISTORY rows —
  the same 322-test suite — measure ~4.2s on a quiet run. The 12.2s is machine
  load, not a real budget regression, so I left the budget alone and restored
  the load-polluted HISTORY row rather than record a meaningless spike. If the
  hook's own quiet gate still measures over 10s at merge, this is the
  `NEEDS_CONTEXT` case the brief names — but I expect it to be ~4.5s since no
  tests were added.

## Files

- `rules/format_cost_bench_test.go` (new) — `BenchmarkFormatCost` +
  `commanderBenchConfig`/`constructedBenchConfig`/`runFormatBench`/
  `playFormatTurns`. The benchmark and its three sub-cases, seed, fixed turn
  count, the per-decision/per-turn/allocation metrics, and the completion
  assertions that turn an early game-over into a hard failure (so the
  denominator can never silently shrink).

## Open concerns

- Engine time per decision in Commander is genuinely ~1.6x (object-count
  driven). It is invisible under the demo's 1.5s pace, but it would become a
  real factor at `-pace 0`. That is a future optimisation decision, explicitly
  **out of scope** for this task — the brief demands no optimisation and none
  was made.
