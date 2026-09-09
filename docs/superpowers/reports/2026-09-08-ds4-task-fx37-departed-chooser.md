# Task fx37 — pin the departed-chooser resumption event stream

Branch `wt/fx37`, worktree `.worktrees/fx37`, base current `main`. One commit:
`test(rules): pin the departed-chooser resumption event stream`.

## What changed and why

One new test file, `rules/departed_chooser_test.go`, containing
`TestDepartedChooserResumptionEventStreamIsDeterministic` and a small
`kindRuns` helper. No engine code, no existing test oracle, no
`acceptanceHeads` change. The task brief is a **test-only** deliverable, and
this is exactly that.

The test drives a genuine CR 800.4f elimination with a decision outstanding
and pins the **event stream** of `rules/sba.go`'s
`releasePendingDecisionOfDepartedPlayer`, not merely the end state:

- **exact event count** of the path (64);
- **kinds and order** of the resumption tail, run-length encoded so a change
  in order fails as well as a change in count;
- the **CR 608.2n completion tail** (the suspended Chain Lightning moving from
  the stack to the graveyard, with no second `Resolve`);
- the **chain head** asserted as a constant (`08a5b034057d2b7b`);
- a **log-only replay** (`replayFromLog`) that folds the recorded events
  straight through `events.Apply` and must reconstruct the identical Game
  (`diffGames` == "").

## Why this card and this departure

**Card: Chain Lightning** (real corpus card, `ur-delver` repo deck). Its
`DealDamage -> CopySpellAbility` chain poses a genuine mid-resolution
`UnlessCost$` paid-or-copy ask, the `ResumeKind "unless_pay"` shape — precisely
the suspended-resolution arm of `releasePendingDecisionOfDepartedPlayer`. A
synthetic fixture could manufacture the same ask, but Chain Lightning reaches
it through the real cast/resolve path with no manufactured SAs, and it is the
repo's own established departed-payer probe
(`TestCR608CompletedSpellLeavesStackAfterDepartedPayer`).

**Departure: a concession** (`e.emit(PlayerLost, "conceded")`, CR 104.3a). It is
the only lawful way to leave while your own resolution is suspended on the
stack. A concession is normally a **priority** option, but the suspended ask is
not a priority decision, so no `Submit` intent can express it at this point —
which is exactly why the existing CR fixture emits it directly. With three
seats the game correctly keeps running after the loss (that is what makes the
departed-chooser path reachable rather than a game-over).

## Measure first: where the asserted stream came from

**Measured-and-accepted.** I instrumented the path before writing the
assertions and read off the real log (the same fixture, `start` recorded just
before the `PlayerLost`, events printed through `checkStateBased`). The tail is:

```
seq 84   player_lost   player 1  "conceded"        (the departure record)
seq 85..144  move_zone  ×60  player 0  "player left the game"  (CR 800.4a sweep)
seq 145  move_zone  Chain Lightning  stack -> graveyard   (CR 608.2n completion)
seq 146  priority
seq 147  decision_ask  "priority"
```

That is 64 events: `player_lost ×1, move_zone ×61, priority ×1, decision_ask ×1`.

I checked the three "is this wrong" signals the brief names before pinning:

- **A state change with no event?** The `replayFromLog` comparison passes
  (identical Game), so every state change on this path is carried by an event.
- **A missing completion tail?** No — the Chain Lightning leaves the stack for
  the graveyard (seq 145), and no second `Resolve` re-fires (the `Resolve` was
  emitted once, before the ask suspended the resolution). This is the correct
  CR 608.2n tail.
- **An event attributed to the departed player?** No. After the departure
  `PlayerLost` (which is necessarily seat 1 as the record of the loss), every
  event in the resumption tail is attributed to a living seat (the `move_zone`
  sweep carries the default player 0 controller in its zero-value field; the
  resumed `priority`/`decision_ask` are for seat 0). The test asserts exactly
  this: no post-departure event with `Player == 1` other than the
  `PlayerLost` itself.

So the stream is correct as produced — I pinned it rather than freezing a
divergence.

## Sensitivity check (not committed)

To prove the test is not vacuous I temporarily neutralised the resume block in
`releasePendingDecisionOfDepartedPlayer` (`if e.resume != nil && false`), ran
the test, then restored the file. The test failed exactly as it should, and the
restored tree passes:

```
--- FAIL: TestDepartedChooserResumptionEventStreamIsDeterministic
    departed_chooser_test.go:79: departed-chooser path emitted 63 events, want 64
```

The `&& false` variant (no resume) drops the completion tail → 63 events → the
count assertion catches it. I confirmed `git diff` on `rules/sba.go` is empty
after restore.

## Deviation from the brief

The brief says to drive the replay "through the `replay` package." I used
`replayFromLog` (the rules-internal reconstruction in `rules/trigger_test.go`)
instead. Reason: the `replay` package re-executes a match **from its recorded
Intents**, and this fixture's setup (the Chain Lightning hand move, the life
change, the mana add) and the departure (a concession at a suspended,
non-priority ask) are direct event emits that no `Submit` intent ever
recorded — so an intent-driven replay cannot reproduce them and would report a
spurious divergence the moment it reaches them. `replayFromLog` folds the
recorded events straight through `events.Apply`, which is precisely the T21-e
"if any state change bypassed an event emit, a log-only replay cannot know
about it" property the brief asks the assertion to catch, and it is what the
repo's own established replay-fidelity tests (`cast_test.go`, `etb_test.go`,
`delayed_test.go`) use for fixtures with direct emits. This is the correct tool
for "log alone reconstructs the state"; the `replay` package is for
intent-driven fixtures.

## Gates (real output)

Format / diff / types / vet:

```sh
$ gofmt -l .
(no output)
$ git diff --check
(no output)
$ go run ./cmd/gentypes -check
(no output)
$ GOMEMLIMIT=5GiB go vet ./...
(no output)
```

Affected + named-dependent packages:

```sh
$ GOMEMLIMIT=5GiB go test -p=2 -count=1 ./rules ./effects ./host ./events ./state ./replay
ok  	github.com/adams-shaun/gorge/rules	19.231s
ok  	github.com/adams-shaun/gorge/effects	5.587s
ok  	github.com/adams-shaun/gorge/host	17.600s
ok  	github.com/adams-shaun/gorge/events	0.008s
ok  	github.com/adams-shaun/gorge/state	0.006s
ok  	github.com/adams-shaun/gorge/replay	5.883s
```

New test, focused and repeated:

```sh
$ go test -run 'TestDepartedChooserResumptionEventStreamIsDeterministic$' -count=1 ./rules/
ok  	github.com/adams-shaun/gorge/rules
(stable across 3 consecutive runs)
```

CR conformance lane (85 leaves):

```sh
$ GORGE_CR_CONFORMANCE=1 GOMEMLIMIT=5GiB go test -p=2 -count=1 ./rules -run TestCR -v
FAIL
FAIL	github.com/adams-shaun/gorge/rules	18.231s
```

## Leaf counting

Counting method (the brief's definition, which two previous seats got wrong):
a leaf is a `--- PASS|FAIL|SKIP: <name>` entry (leading whitespace stripped)
that no other name has as a `/` prefix — subtests count, their parents do
not. Counting only the un-indented top-level `---` lines gives 61, which is
the wrong number; the correct count needs the indented subtest `---` lines.

Result: **85 leaves, 45 FAIL / 40 PASS** — identical to the brief's stated
baseline for current `main`. The new test
(`TestDepartedChooserResumptionEventStreamIsDeterministic`) does not match
`-run TestCR` (it does not start with `TestCR`), so it does not appear in the
lane and **no leaf moved; zero green-to-red**. No engine or existing-test
change was made, so the lane is byte-for-byte unchanged from baseline.

## Open concerns

None that block. Two notes:

- The pinned event count (64) and its 60-event departure sweep depend on the
  fixed `death-n-taxes` repo deck composition (the sweep is the departed
  player's owned objects). That coupling is inherent to "exact event count"
  and matches the coupling the acceptance/heads goldens already rely on; a
  repo-deck change that alters the sweep count would intentionally surface here.
- The `move_zone "player left the game"` events carry the zero-value
  controller (seat 0) in their `Player` field rather than naming the departed
  owner. That is not a departed-player attribution (seat 0 is alive) and is
  not part of this task's scope; recorded here only so it is not later
  mistaken for one.
