# Task U3 — measure why Commander tables run slower than constructed

Branch `wt/u3`, worktree `.worktrees/u3`. This task ships **measurement and
findings only — no optimisation, no engine change**. Round 1 (this revision)
re-states the verdict against the observation that actually happened and adds
the one measurement round 0 never considered (decisions per STEP). It changes
no engine code and does not revert or delete the round-0 finding.

## Verdict up front (round 1, corrected)

**The observed ~2x is NOT explained by decision count, engine cost, or step
selection.** Round 0's verdict had to be retracted: it compared a 4-seat
Commander table against a 2-seat constructed table — a configuration the user
never watched (their server runs all tables at **4 seats**; `gorged`'s
`-seats` default is 4 and the demo used none of the other seats). At **equal
seat count** the plain numbers leave nothing to explain the 2x:

- **decisions/turn** — commander 33.43 vs constructed 33.86 (constructed asks
  *more*; commander asks ≈0 extra).
- **engine time/decision** — ~1.6x in commander, your own arithmetic shows is
  0.01% of wall time at `-pace 1.5s`.
- **decisions per STEP** (this round, the alternative the report never
  considered) — the user's two watched windows are 1.23x apart at most, and
  for the *same* window commander is 0.98x (slightly **faster**).

So none of the measured engine-side or pacing-side quantities reproduce a ~2x
at equal seats. Saying "the engine is innocent and the pace knob explains it"
was right that the engine is innocent, but wrong that the pace knob explains
it: after removing seat count (which does not vary on the user's server) the
pace term is the same for both formats, and only a decision-count difference
can make the pacing differ. There is no such difference at equal seats.

What this means, stated plainly and kept: the measurement does **not** explain
the report. It explains a different question. Per the round brief's point 3,
the correct deliverable is an honest "the observation remains unexplained by
decision count, engine cost, or step selection" rather than a verdict that
reads as solved — and a note on what would settle it (a direct wall-clock
tracing on the real `gorged` server, including rendering/SSE and the exact
steps the user timed). **No optimisation was made, and none should follow from
this result.**

### Round-0 verdict defect (recorded)

Round 0's headline ratio — the 1.59x "seat count" explanation — came from the
`constructed-2seat` row, which exists only because round 0 *assumed* command-
er tables had 4 seats and constructed demo tables had fewer. The running
server reports `t1..t4 all seats=4` (`gorged` `-seats` default 4,
`cmd/gorged/main.go:67`, demo launched `-format
commander,commander,constructed,constructed -tables 4` with no `-seats`), so
the 2-seat baseline does not exist in the observed configuration and the
1.59x was between two tables the user never compared. That defect is what this
round corrects. The seat-count/size finding itself is retained below, demoted
to what it is: an effect that **would** show up if the demo tables differed in
seats (or deck size or life), which on the reported server they do not.

## The measurement (round 0, unchanged)

Committed as `rules/format_cost_bench_test.go` — `BenchmarkFormatCost`, five
cases, all driven **directly on the engine** (never through `gorged`, whose
`-pace` sleep is the variable being removed; never through `mtgsim`/`botbench`,
which have no format flag). The first three rows were measured in round 0 and
are bit-identical after this round (decisions/turn, allocs/decision,
B/decision):

| case | decks | seats | life | decisions/turn | allocs/decision | B/decision | µs/decision |
|---|---|---|---|---|---|---|---|
| commander | 4× foundations-* (100-card) | 4 | 40 | **33.43** | **81.72** | 10612 | ~29–35 (load) |
| constructed | 4× legacy (60-card) | 4 | 20 | **33.86** | **51.35** | 9888 | ~15–18 (load) |
| constructed-2seat | 2× legacy (60-card) | 2 | 20 | **21.00** | **50.49** | 9968 | ~10–16 (load) |

Seed `0x55b00f`; the two 4-seat rows play **7 full turns** (the turn the user
was watching); the `-2seat` row plays the same 7 turns at 2 seats. `decisions/
turn`, `allocs/decision` and `B/decision` are **bit-identical across every
run** (they are the deterministic, load-free signal). The wall-clock
ns/decision column is load-sensitive (this box sat at load ~33 in round 0), so
it is reported as a range, not a gospel number — the honesty clause. The
engine-time ratio (~1.6x commander/constructed) is independently corroborated
by the deterministic allocation ratio 81.72/51.35 = 1.591x, so it is real even
though the clock is noisy.

The `constructed-2seat` row is now exposed for what it was: **round 0's
artifact**, kept so the record stays verifiable, but it models a table that
does not exist on the user's server and is excluded from every comparison in
round 1.

### What could NOT be held equal

Deck size (100 vs 60) and starting life (40 vs 20) are inherent to the
Commander format — a finding, not a flaw; they drive the engine-time component
below. Everything else was held equal across the two 4-seat rows: seat count,
seed, turn count, mulligan round (`Mulligans: 1`, the `gorged` default), token
corpus, and the same bot policy (`botpolicy.Decide` via the test bot).

## cpuprofile (the three suspects — round 0, retained)

Engine time per decision IS genuinely ~1.6x higher in Commander (real, just
sub-dominant), so per the round-0 brief it was profiled. `go tool pprof`
self-time top functions:

**Commander** — `checkTriggers.func1` 15.5%, GC `scanSpanPackedAVX512` 5.7%,
`memmove` 5.5%, `Object.Face` 4.4%, map-get 3.5%, `memclrNoHeapPointers` 3.5%,
`triggerMatches`/`forEachObject`/`zoneGate` smaller. **Constructed** has the
same functions in the same order (`checkTriggers.func1` 12.3%, GC 6.5%,
`memclr` 4.3%, …). The engine cost is spread across **object-walks + GC**, all
of which scale with object count.

Suspect verdicts (cum time, commander share vs constructed share):

| suspect | commander | constructed | verdict |
|---|---|---|---|
| `Game.Clone` (O(objects)) | **0%** | 0% | **KILLED** — never runs on the per-decision path; only `host` snapshot/viewat/restart at turn boundaries, amortised once a turn, dwarfed by pacing. |
| `legalActions` (m31 command-zone walk) | 0.28s · 6.18% | 0.22s · 6.77% | **KILLED as the format's driver** — identical share in both formats. |
| `BoardFromGameInto` (m37 per-decision commander scan) | 0.18s · 3.97% | 0.14s · 4.31% | **KILLED as the format's driver** — flat ~4% share. |

The real engine-cost mechanism is **object count**: 4×100 ≈ 400 vs 4×60 ≈ 240,
ratio 1.67x, matching the ~1.6x engine time and 1.59x allocation ratio — real,
measurable, but ~0.01% of the pace-dominated wall time. This finding stands.

## Decisions per STEP — the alternative round 0 never considered (round 1)

The user did not compare whole turns; they watched **`main1 → main2` on the
Commander table** and **`begin-combat → end` on the constructed table**, which
are different step windows. Pace is per decision, so a step window's wall time
is its own decision count × 1.5s. Round 1 buckets the **same two 4-seat
games** (same seed, same 7 turns) by `e.G.Step` read before each `Submit` — no
allocation, deterministic — and reports the average decisions per step per
turn. These are the two new sub-cases `commander-perstep` /
`constructed-perstep` (4 seats only; the 2-seat artifact is excluded).

**Per-step decisions per turn (4 seats, avg of 7 turns, seed `0x55b00f`):**

| step | commander | constructed |
|---|---|---|
| upkeep | 4.00 | 4.00 |
| draw | 4.00 | 4.00 |
| **main1** | **9.43** | **8.43** |
| begin-combat | 4.00 | 4.00 |
| declare-attackers | 0.00 | 0.29 |
| declare-blockers | 0.00 | 0.00 |
| combat-damage | 0.00 | 0.00 |
| end-combat | 4.00 | 5.14 |
| **main2** | **4.00** | **4.00** |
| end / cleanup / untap | 0.00 | 0.00 |
| **total** | **33.43** | **33.86** |

The distribution is essentially identical format-to-format. The only material
differences are main1 (9.43 vs 8.43 — commander is slightly more, its casting
density) and end-combat (4.00 vs 5.14 — constructed is slightly more). Nothing
is near a 2x gap at any step.

**The windows the user watched, at 1.5s per decision (`spendPace`):**

| window | commander | constructed | ratio |
|---|---|---|---|
| `main1 → main2` (decisions → wall @1.5s) | 21.43 → 32.1s/turn | 21.86 → 32.8s/turn | **0.98x** (commander *faster*) |
| `begin-combat → end` (→ wall @1.5s) | 16.00 → 24.0s/turn | 17.43 → 26.1s/turn | 0.92x |
| **the user's cross pairing**: commander `main1→main2` vs constructed `begin-combat→end` | 21.43 → 32.1s | — | 17.43 → 26.1s | **1.23x** |

`main1→main2` is the inclusive step-index range `[StepMain1, StepMain2]`
(main1, begin-combat, …, end-combat, main2); `begin-combat→end` is
`[StepBeginCombat, StepEnd]`. Both are read straight from the benchmark log.

**Step selection does not reproduce ~2x.** The best possible window pairing of
the two formats is 1.23x (commander main-phases against constructed
combat-to-end), and comparing the *same* window on both formats shows commander
is 0.98x — slightly faster. There simply is no per-step, per-turn, or per-
window decision-count gap of ~2x between the two formats at equal seat count.
The brief's point-3 branch applies: **the observation remains unexplained by
decision count, engine cost, or step selection.** We stop here, per the brief —
no third hypothesis was hunted and nothing was optimised.

### What would settle it (a note, not a new hypothesis to chase)

The measured engine-driven cost models what `gorged` would *sleep* between
decisions; it does not model what the user *perceived*. Settling the report
requires a direct wall-clock trace of the real server — the exact steps the
user timed on each table, through `host/httpapi` + the Svelte client, ideally
with SSE/rendering cost and the per-match goroutine included — rather than an
engine-direct decision census. That is app-side instrumentation (gorge
application never imports into the engine; the reverse holds), so it is a
different task and explicitly out of this report's scope.

## Gates run (round 1)

- `gofmt -l .` → clean (no files listed).
- `go vet ./...` → clean (exit 0).
- `go test ./rules/` → `ok github.com/adams-shaun/gorge/rules 4.239s`
- `go test ./rules/ -run '^TestHeads$' -v` → `--- PASS: TestHeads (0.61s)` —
  **heads did not move** (expected: benchmarks and measurement only).
- `go test -bench 'BenchmarkFormatCost' -benchmem -run '^$' ./rules/` → PASS;
  the three round-0 rows are bit-identical to round 0 and the two new per-step
  rows report the window table logged above:
  ```
  BenchmarkFormatCost/commander-32         148   7589689 ns/op   10612 B/decision  81.72 allocs/decision  33.43 decisions/turn  29205 ns/decision  976267 ns/turn
  BenchmarkFormatCost/constructed-32       289   4367371 ns/op    9888 B/decision  51.35 allocs/decision  33.86 decisions/turn  16483 ns/decision  558071 ns/turn
  BenchmarkFormatCost/constructed-2seat-32 615   2720027 ns/op    9968 B/decision  50.49 allocs/decision  21.00 decisions/turn  16403 ns/decision  344462 ns/turn
  BenchmarkFormatCost/commander-perstep-32    ... window main1->main2 21.43 -> 32.1s/turn; begin-combat->end 16.00 -> 24.0s/turn; total 33.43
  BenchmarkFormatCost/constructed-perstep-32  ... window main1->main2 21.86 -> 32.8s/turn; begin-combat->end 17.43 -> 26.1s/turn; total 33.86
  PASS
  ```
  The benchmark remains outside the ordinary `go test ./rules/` run (it has no
  `Test` functions; its corpus load happens only inside the `Benchmark*`
  body), so it adds no runtime to the test suite.

## Deviations from the brief

- **Kept the round-0 measurement sections** (the brief asked to keep them), and
  retained the `constructed-2seat` case in the file — not to defend it but so
  the record (and the round-0 report) remains reproducible. Round 1's
  comparisons all use the two 4-seat rows; the 2-seat case is labelled in the
  benchmark and the report as the artifact the defect was.
- **Step windows are the inclusive index ranges** `[main1, main2]` and
  `[begin-combat, end]`, matching the phase/step markers the user named. The
  benchmark logs the per-step table and both window wall-times directly, so
  the reader can re-assemble any other window; the two shown are the ones the
  brief named.

## Files

- `rules/format_cost_bench_test.go` (round 1) — adds `commander-perstep` and
  `constructed-perstep` sub-cases plus `perTurnPerStep`, `sumSteps`,
  `logStepWindows` and the `spendPace` constant; `perTurnPerStep` buckets
  decisions by `e.G.Step` (a struct-field read: no allocation, deterministic)
  over the same 7-turn/4-seat games and logs the per-step table and the two
  watched windows' decision counts and wall-times at 1.5s/decision. The round-0
  timing/allocation sub-cases are untouched (their three rows are bit-identical).

## Open concerns

- The ~2x the user saw is **not** accounted for by decision count, engine cost,
  or step selection at 4 seats. The honest gap is that the engine-direct census
  cannot model the served-game experience (rendering, SSE, per-match goroutine,
  which steps were actually timed). Closing it needs app-side wall-clock tracing
  on the real `gorged` server — explicitly deferred out of this task, which
  ships measurement only and must optimise nothing.
- Engine time per decision in Commander remains genuinely ~1.6x (object-count
  driven); invisible under 1.5s pacing but a real factor at `-pace 0` — a future
  optimisation decision, still out of scope.
