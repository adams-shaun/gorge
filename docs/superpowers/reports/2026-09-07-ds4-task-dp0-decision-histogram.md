# Task dp0 — measure which decisions the bot actually faces

Branch `wt/dp0`, worktree `.worktrees/dp0`. This task ships **measurement and
instrumentation only — no policy behaviour changed**. `botpolicy/` is
untouched; the collector observes the Decision the engine offered and the
Intent the seat returned, and never calls into the policy, the engine or any
rng source, so enabling it cannot perturb a single decision (proved below).

## What changed and why

### `cmd/botbench/stats.go` (new)
The per-decision-kind histogram. `decisionStats` is a mutex-guarded
`map[string]*decisionStat`; every mutating method (`game`, `record`) takes the
mutex, because `bench()`/`playOnePairWithPool` run games in parallel and
several goroutines append to this one map. It is **pure observation**:

- `game()` counts one played match (the mean-per-game denominator), including
  stalled ones.
- `record(d, in)` runs after `Decide` returns and before `Submit`, reads only
  the offered options and the returned answer, and tallies `count`,
  `sumOpts` (mean option count), `single` (instances with `len(Options)==1`)
  and `first` (instances whose first chosen index is `Options[0].Index`).
- `statKey` gives each decision a row key. `KPriority` is broken down by the
  **branch the policy took** (`priorityBranch` recovers it from the chosen
  option's `Kind`: `activate`→tap, `play_land`→land, `cast`→cast,
  `ability`→ability, `pass`/`concede`→pass). `KChoose` is broken down by its
  option sub-kind (`d.Options[0].Kind`: x, exile, sacrifice, discard, name,
  type, number, yes/no). Every other kind maps to one row named by the kind.
- `write` emits rows in **sorted-key order**, never map-iteration order, so
  the report is byte-deterministic for identical inputs.

**Why no hook into `botpolicy/`:** the chosen answer is fully recoverable from
the Intent and the Decision the bench already has in hand. `in.Choices[0]`
names the option the policy selected, and that option's `Kind` is the branch.
So the narrowest possible mechanism was zero lines in `botpolicy/` — the
brief's "add the narrowest possible hook" case did not arise.

### `cmd/botbench/main.go`
- New package-level `decisionStatsEnabled bool` (set once in `main` from the
  flag; read-only thereafter), so the dozens of existing test call sites that
  exercise `run`/`runMatrix` keep their signature and behaviour — a disabled
  collector creates nothing, records nothing and appends nothing.
- `playMatch` gained a trailing `collect *decisionStats` parameter; it calls
  `collect.game()` at entry and `collect.record(d, in)` between `Decide` and
  `Submit`. Only the two internal call sites (in `run` and `runMatrix`) call
  it, so no test surface changed.
- `run` and `runMatrix` create `collect` only when the flag is on (nil
  otherwise, so the default report is byte-identical to a pre-flag build),
  pass it into the play closure, and call `collect.write(out)` after the
  normal report on success.
- `main` registers `-decision-stats` (default false) and sets the flag.

### `cmd/botbench/stats_test.go` (new)
Corpus-free unit tests so the new instrumentation itself is covered:

- `TestDecisionStatsAggregate` pins the column tallies and sorted-key output.
- `TestDecisionStatsConcurrent` bursts records from 40 goroutines and asserts
  no count is lost — the exact defect the task calls out.
- `TestPriorityBranch` pins the tap/land/cast/ability/pass/other mapping.

## Gate commands run (real output)

```sh
$ go build ./...                         # whole tree
# (no output — build succeeded)

$ go vet ./cmd/botbench/...              # affected package + its deps
# (no output — vet clean)

$ go test ./cmd/botbench/...             # affected package, once
ok  	github.com/adams-shaun/gorge/cmd/botbench	2.963s

$ go run ./cmd/testtime -changed         # budget measure (pre-commit hook)
testtime: cmd/botbench 2.9s 34 tests budget 5s
testtime: wrote cmd/botbench/TEST_HISTORY.md
```

Per the fleet rules I did **not** run `go test ./...` or `go test -race` (the
controller runs the full tree and the race detector once at the pre-push gate
on main). I did not run `make sim`/`make report`/heads: this change touches
only `cmd/botbench` and cannot move a chain head, a golden replay or the
ratchet (no `rules/`/`effects/`/`events/` code changed).

## Determinism proof — enabling `-decision-stats` does not perturb a decision

This is the single most important check. Same seed (0), same game count (200),
same format, flag off vs on. The **normal** bench output (everything before
the appended stats block) must be byte-identical.

Flag **off**:
```sh
$ /tmp/botbench -games 200 -format constructed            > off.txt
$ /tmp/botbench -games 200 -format commander              > cmd_off.txt
```
Flag **on** (adds the stats block):
```sh
$ /tmp/botbench -games 200 -format constructed -decision-stats > on.txt
$ /tmp/botbench -games 200 -format commander   -decision-stats > cmd.txt
```

Now diff the normal output (drop the appended stats block, marker at the
`decision stats` line):

```sh
$ # constructed: on.txt marker line 209, so take lines 1..207 (the normal report)
$ head -n 207 on.txt > on_normal.txt
$ diff off.txt on_normal.txt && echo "EMPTY"
EMPTY

$ # commander: cmd.txt marker line 209, take lines 1..207
$ head -n 207 cmd.txt > cmd_on_normal.txt
$ diff cmd_off.txt cmd_on_normal.txt && echo "EMPTY"
EMPTY
```

Both diffs are **empty**, verbatim. (The whole-file diff is just `207a208 >
(blank)` — the blank separator line that begins the appended stats block; it
is part of the new table, not the normal report.) Enabling the flag therefore
leaves every decision, every rng draw, every option order and every event
exactly as it was with the flag off. The collector is read-only observation.

## The histogram — constructed, 200 games

`bot bench: base seed 0, bot vs bot, 200 games, 2 seats, decks
death-n-taxes,dimir-tempo` (`-games 200 -format constructed -decision-stats`):

```
decision stats (200 games):
kind              count  mean/game  mean opts  singleton%  first-option%
attackers         1673   8.37       1.96       42.1%       78.7%
blockers          278    1.39       1.85       51.4%       15.1%
choose/discard    207    1.03       8.11       0.0%        100.0%
choose/exile      162    0.81       5.17       3.1%        100.0%
choose/name       105    0.53       8.14       0.0%        100.0%
choose/number     82     0.41       13.00      0.0%        100.0%
choose/sacrifice  84     0.42       1.71       59.5%       100.0%
choose/type       35     0.17       9.00       0.0%        100.0%
priority/ability  1791   8.96       5.84       0.0%        46.8%
priority/cast     3454   17.27      5.68       0.0%        70.5%
priority/land     2149   10.74      6.14       0.0%        55.6%
priority/pass     58355  291.77     2.95       0.0%        74.4%
priority/tap      7939   39.70      6.00       0.0%        45.9%
target            2074   10.37      5.82       11.2%       35.4%
trigger_optional  1165   5.83       2.00       0.0%        48.6%
trigger_order     219    1.09       2.14       0.0%        46.1%
```

(No `mulligan`, `modes`, `commander_zone` or `choose/x`/`choose/yes` rows in
constructed: `Config.Mulligans` is the zero value so no London round runs, no
mid-resolution modal/unless-pay ask arose in these games, and a constructed
game has no command zone or {X} casts in these lists.)

## The histogram — commander, 200 games

`bot bench: base seed 0, bot vs bot, 200 games, 2 seats, decks
foundations-calling-all-angels,foundations-keen-engineering (commander
format)` (`-games 200 -format commander -decision-stats`):

```
decision stats (200 games):
kind              count  mean/game  mean opts  singleton%  first-option%
attackers         3727   18.64      4.23       25.1%       46.8%
blockers          1547   7.74       5.61       28.0%       12.7%
choose/discard    214    1.07       8.22       0.0%        100.0%
choose/name       55     0.28       20.51      0.0%        100.0%
choose/sacrifice  318    1.59       5.42       0.0%        100.0%
choose/type       44     0.22       18.00      0.0%        100.0%
choose/x          143    0.71       1.68       67.8%       67.8%
commander_zone    234    1.17       2.00       0.0%        100.0%
modes             24     0.12       3.00       0.0%        100.0%
priority/ability  2305   11.53      6.82       0.0%        41.8%
priority/cast     6777   33.88      7.50       0.0%        54.6%
priority/land     3362   16.81      8.65       0.0%        46.3%
priority/pass     99748  498.74     4.50       0.0%        69.4%
priority/tap      24981  124.91     8.39       0.0%        50.8%
target            1970   9.85       11.29      7.1%        29.7%
trigger_optional  517    2.58       2.00       0.0%        48.2%
trigger_order     889    4.45       2.20       0.0%        49.4%
```

## The point of the table: which kinds are frequent AND flat

Total decisions/game: **~399 constructed**, **~734 commander**. The single
dominant row is **priority/pass** — 292/game (73%) constructed and 499/game
(68%) commander. It is frequent, and at 74%/69% first-option it is flat; but
most of it is the degenerate no-op (nothing legal to do, or a pass around a
resolution), so it is not the first place to spend policy effort. The 26–31%
of passes where "pass" was *not* `Options[0]` are the cases where the bot saw
a legal action and declined it.

The cleanest **frequent AND flat** pair is a real-decision one:

1. **attackers** — constructed **8.4/game**, **78.7% first-option**, mean
   options **1.96**. Over half the time it is a singleton (42%), but when it
   is not the bot takes `Options[0]` ~79% of the time. This is the largest
   volume of a *choice* answered flat in constructed. Commander is same shape
   (18.6/game) though less flat (46.8%).
2. **choose/\*** — flat **by construction**: the `x`/`exile`/`sacrifice`/
   `discard`/`name`/`type`/`number` branches all return the first `Min`
   options or the first option, so **every choose row is 100% first-option**.
   Individually small (≤1.6/game), but together ~3.4/game constructed and
   ~4.1/game commander, and the flatness is total — any quality work there is
   pure upside with zero risk of a wrong cost.
3. **priority/cast** — **17.3/game** constructed, **70.5% first-option** (and
   34/game / 55% commander). Frequent and flat-ish: the casting ranking exists
   but its output coincides with `Options[0]` ~70% of the time.
4. **priority/tap** — **39.7/game** constructed (125/game commander) at ~6
   options each, but only 45.9% first-option. This is *not* flat by the
   measure — but note the caveat below: the policy answers it position-first
   (always the first `activate`), and the measured share is dragged below
   100% because `play_land`/`cast` options are listed *before* the activates,
   so the first `activate` is rarely `Options[0]`. The flatness is real but
   under-reported by the first-option metric.

And the task's framing's "die roll" is visible too: **trigger_optional** is
**5.8/game** (constructed) at **48.6% first-option** with exactly 2 options —
a coin flip, not a ranking, and the one non-trivial frequent decision the
policy deliberately randomises (trigger_order at ~1/game is a shuffle, 46%
first). At ~6/game that is the highest-frequency *pure die roll* in the run.

## Verdict

The leverage, in order, is: **(a)** `attackers` (the frequent+flat combo with
real options in constructed), **(b)** the **choose/\*** sub-kinds (100% flat,
low but real volume), **(c)** `priority/cast` (17–34/game at 70/55% first),
and **(d)** `trigger_optional` (~6/game, a deliberate coin). `priority/pass`
and `priority/tap` dominate raw volume but pass is mostly forced and tap's
flatness is an artifact of option ordering the metric under-reports. A policy
change aimed at the next round should target attackers, the choose sub-kinds,
casting ranks and the trigger-optional coin before it spends effort on the
priority/pass sea.

## Deviations from the brief

- **No change to `botpolicy/`** was needed (and none was made): the chosen
  branch is recoverable from the Intent + Decision already in hand, so the
  "only if unavoidable" hook was not needed.
- The deliverable runs are single-pair (`-games 200 -format …`); I also wired
  the histogram into the `-pairs` matrix path (both text and JSON) so the flag
  works on a matrix run too, since it is the same "a run".

## Open concerns

- **`first-option` share and option position.** For `priority/tap` (and
  `priority/pass` where a declined action precedes it) the metric measures
  "was `Options[0]` chosen", which conflates *policy flatness* with *whether
  the chosen option is first in the list*. The tap policy always picks the
  first `activate`, but `play_land`/`cast`/`ability` options are offered
  before the activates, so the share lands at ~46–51% instead of ~100%. A
  flatness signal for priority sub-branches might better be "the chosen option
  was the first of its *kind*"; worth considering if this histogram becomes a
  ratchet.
- **No race detector run.** I could not run `go test -race` (fleet rule FL-56:
  the controller runs it once at the pre-push gate). The collector is fully
  mutex-guarded and `TestDecisionStatsConcurrent` asserts no lost counts
  functionally, but a real `-race` pass was not run by me.
- **Sample stability.** 200 games is stable to the precision shown for the
  high-frequency rows; the low-frequency rows (`modes`, `commander_zone`,
  `choose/type`) vary game to game and should be read as orders of magnitude,
  not exact rates.
