# Merge-conflict resolution — task agent-20260923T114033Z-a57ee463

Ticket: `agent-20260923T114033Z-a57ee463`
Branch: `wt/agent-20260923T114033Z-a57ee463`
Merge commit: `ea4b5b32` (Merge branch 'main' into wt/agent-20260923T114033Z-a57ee463)
Pre-merge branch tip: `102a9345`
Merged main tip: `e46b273d`
Merge base: `87901df9`

## What was in flight

`git status` at entry showed a **clean** tree except for an unrelated
working-tree edit to `.ds4/report-t1.md`; no `MERGE_HEAD`, no rebase dirs. The
previous resolver run had not started the merge. So I ran `git merge main`
myself. Two files conflicted:

- `rules/heads_test.go` (2 hunks)
- `rules/player_target_qualifier_test.go` (1 hunk)

Everything else auto-merged cleanly, including the other eight files both sides
had touched.

## The branch's fix

Game-long damage-by-source provenance (The Fallen, Diseased Vermin) — commits
`c2f63152` (implementation) and `102a9345` (review-gate head re-pin). Every
landed `Damage` event now emits a `DamageProvenance` fact through
`Engine.emit`'s one post-fold tail, backing the
`wasDealtDamageThisGameBy <ref>` player qualifier and the
`wasDealtDamageByThisGame` object predicate.

## Main's competing change

`6ef0129a` ("feat(botpolicy): promote explore X1 — A1's attach no-op applies to
attach abilities only", the "bot-x1" change) is on main but not on the branch.
It is a **bot-policy** change: it changes the deterministic bot's legal-answer
choices, so it moves chain heads for the same seats the provenance change does.

## Conflict 1 — `rules/heads_test.go`

### What each side wanted

Both sides appended to the `acceptanceHeads` map, and the goldens contradict
because both changes move the acceptance stream.

- **HEAD (branch)** wanted, from its provenance change:
  - `2: f107be40dc2792c6`
  - `4: 9b3aab4e0336ba8c`
  - `6: c4ce39421c473963`
  - `8: 3d1974a1859d9676`
- **main** wanted, from bot-x1:
  - `2: 3ddcc4e3ba5bb799`
  - `4: b3bf75f0c56512ae` (unchanged by bot-x1)
  - `6: a93593d452866261` (unchanged by bot-x1)
  - `8: beac5729b0e63dcc`

### How I resolved it

Neither side's numbers are correct for the merged tree: **both** changes are now
present, so the correct goldens are a fourth set that neither side carries. I
measured them rather than picking a side.

First resolution (branch values as a compiling placeholder), then
`go test -run 'TestHeads$' -v ./rules/` reported:

```
2 seats: chain head 9b4759db7fe10fdd, golden f107be40dc2792c6
8 seats: chain head d2ebb7cbf40ccd12, golden 3d1974a1859d9676
```

4 and 6 seats passed at the branch's values (`9b3aab4e0336ba8c` /
`c4ce39421c473963`).

To **attribute** the move (not guess it), I ran a contained probe: I backed up
`rules/engine.go`, disabled only the provenance emission guard
(`if stored.Kind == events.Damage && stored.Amount > 0` →
`if false && …`), re-ran `TestHeads`, and restored `engine.go` byte-identically
(`git diff --stat rules/engine.go` printed nothing afterwards). The probe
produced:

```
2 seats: chain head 3ddcc4e3ba5bb799   (main's bot-x1 value)
4 seats: chain head b3bf75f0c56512ae   (main's value)
6 seats: chain head a93593d452866261   (main's value)
8 seats: chain head beac5729b0e63dcc   (main's bot-x1 value)
```

This proves the composition precisely:

| seats | main bot-x1 alone | provenance alone | merged (both) |
|---|---|---|---|
| 2 | `3ddcc4e3ba5bb799` | `f107be40dc2792c6` | `9b4759db7fe10fdd` |
| 4 | `b3bf75f0c56512ae` | `9b3aab4e0336ba8c` | `9b3aab4e0336ba8c` |
| 6 | `a93593d452866261` | `c4ce39421c473963` | `c4ce39421c473963` |
| 8 | `beac5729b0e63dcc` | `3d1974a1859d9676` | `d2ebb7cbf40ccd12` |

At 4 and 6 seats bot-x1 does not move the stream, so the merged head equals the
provenance-only value (and that is why those two entries, auto-merged to the
branch's values, were already correct). At 2 and 8 seats both changes move the
stream, so the merged head is a new value. I updated the 2- and 8-seat entries
and rewrote their attribution comments (and tightened the 4- and 6-seat comments
to say the merged value equals the provenance-only value because bot-x1 is
head-neutral for those streams). `TestHeads` now passes.

## Conflict 2 — `rules/player_target_qualifier_test.go`

The census `TestValidTgtsPurePlayerCensusPinsThePlayerQualifierSets` pins which
pure-player `ValidTgts` values offer seats, over a `SpecContext` with role
bindings. The branch had added a bound damage **source** (`src`) and a landed
hit in seat 1's record so `Opponent.wasDealtDamageThisGameBy Self` is judged
with a real binding. Main had added fuzz-cov3's after-the-dot negation handling.

- **HEAD (branch)** put `Opponent.wasDealtDamageThisGameBy Self` in
  `wantOffered`; listed `Player.!CardOwner` and `Player.!EnchantedBy` in
  `wantFailClosed`.
- **main** put `Player.!EnchantedBy` in `wantOffered` and
  `Opponent.wasDealtDamageThisGameBy Self` in `wantFailClosed` (on main there
  was no bound source, so the game-long qualifier failed closed).

### How I resolved it

The two changes are additive but interact: main's after-the-dot negation makes
`Player.!EnchantedBy` offer, and — in the branch's census, which **does** bind a
source — main's negation makes `Player.!CardOwner` offer too (main's own comment
already said "Crown of Doom's real offer binds one", i.e. it offers when a
source is bound).

My first resolution took the union of both sides' `wantOffered`, which the test
then failed against:

```
got  [... Opponent.wasDealtDamageThisGameBy Self Player Player.!CardOwner Player.!EnchantedBy ...]
want [... Opponent.wasDealtDamageThisGameBy Self Player Player.!EnchantedBy ...]
```

So I moved `Player.!CardOwner` to `wantOffered` as well and rewrote the comment
to explain that it offers here because this census binds a source (unbound it
fails closed). Final lists:

- `wantOffered`: `Any`, `Any.NotDefinedParentTarget,Player`, `Opponent`,
  `Opponent.wasDealtDamageThisGameBy Self`, `Player`, `Player.!CardOwner`,
  `Player.!EnchantedBy`, `Player.!TriggeredActivator`,
  `Player.!TriggeredCardController`, `Player.Opponent`, `Player.Other`, `You`
- `wantFailClosed`: the `Any.!*` trio plus `Player.LostLifeThisTurn` and the
  rest of the unreached qualifier family.

The test now passes.

## Auto-merged overlap files (no conflict, checked)

Both sides touched eight files; six had no conflict and auto-merged. I checked
the highest-risk one by hand, `events/event.go`: main appended `TurnFaceDown`
and `CloneStatic` above the old `NumKinds`, and the branch appended
`DamageProvenance` after `CloneStatic`, with `NumKinds = int(DamageProvenance) +
1`. The merged enum orders both sides' kinds and keeps the hash chain intact
(append-only invariant held). `kindNames` is sized `NumKinds` and the build
confirms the length matches. The other five (`effects/filter.go`,
`events/apply.go`, `rules/engine.go`, `state/game.go`, `state/object.go`)
contain both sides' additions; I ran tests that exercise each side's new
behaviour (see below) and they pass.

## Commands run and real output

Merge:

```
$ git merge main --no-edit
Auto-merging effects/filter.go
Auto-merging events/apply.go
Auto-merging events/event.go
Auto-merging rules/engine.go
Auto-merging rules/heads_test.go
CONFLICT (content): Merge conflict in rules/heads_test.go
Auto-merging rules/player_target_qualifier_test.go
CONFLICT (content): Merge conflict in rules/player_target_qualifier_test.go
Automatic merge failed; fix conflicts and then commit the result.
```

Compile check after resolving (before commit):

```
$ go build ./...
(no output)
```

Heads probe (provenance emission off) — attribution evidence:

```
2 seats: chain head 3ddcc4e3ba5bb799
4 seats: chain head b3bf75f0c56512ae
6 seats: chain head a93593d452866261
8 seats: chain head beac5729b0e63dcc
```

Final heads (provenance on, merged tree):

```
$ go test -run 'TestHeads$' ./rules/
ok  	github.com/adams-shaun/gorge/rules	2.182s
```

Conflicted-file focused tests:

```
$ go test -run 'TestValidTgtsPurePlayerCensusPinsThePlayerQualifierSets$' ./rules/
ok  	github.com/adams-shaun/gorge/rules	0.582s

$ go test -run 'TestEmitRecordsGameLongDamageProvenance|TestEmitRecordsObjectDamageProvenance|TestDiseasedVerminAskOffersOnlyPreviouslyDamagedOpponents|TestValidTgtsPurePlayerCensusPinsThePlayerQualifierSets|TestPlayerTarget|TestSleeperAgent|TestLegalTargetsRecheck' ./rules/
ok  	github.com/adams-shaun/gorge/rules	0.605s

$ go test -run 'TestDamageAllValidPlayersTheFallenResolves|TestPlayerDamageByRefThisGameFailsClosedOnUnboundRef|TestDamageProvenanceWordsAreClassified' ./effects/
ok  	github.com/adams-shaun/gorge/effects	0.579s
```

Post-merge ratchets (brief-mandated):

```
$ go test -count=1 -v ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'
--- PASS: TestEveryRepoDeckCountHeadResolves (0.00s)
--- PASS: TestEveryRepoDeckIsFullySupported (0.63s)
--- PASS: TestEveryDispatchedTriggerModeHasAMatcher (0.00s)
--- PASS: TestNoTriggerModeIsRegisteredThatTheSwitchNeverDispatched (0.00s)
--- PASS: TestEveryRepoDeckParamsAreRead (0.25s)
ok  	github.com/adams-shaun/gorge/rules	0.952s
```

No test skipped (`.cards` present as a symlink to the shared corpus; the 0.63 s
`TestEveryRepoDeckIsFullySupported` is the tell that the corpus was read, not
skipped).

Main's auto-merged additions still hold:

```
$ go test -run 'TestAshNodsAltar|TestSacrificed|TestFirstTurnControlled|TestCopyWasCastFromGraveyard|TestWasCastFromGraveyard|TestCloneTrigger|TestPlayerCount' ./rules/ ./events/ ./effects/
ok  	github.com/adams-shaun/gorge/rules	1.024s
ok  	github.com/adams-shaun/gorge/events	0.009s [no tests to run]
ok  	github.com/adams-shaun/gorge/effects	0.059s
```

Format:

```
$ gofmt -l rules/heads_test.go rules/player_target_qualifier_test.go
(no output)
```

## The in-flight operation

`git merge --no-edit` completed as `ea4b5b32`
("Merge branch 'main' into wt/agent-20260923T114033Z-a57ee463") with git's
default merge message. `MERGE_HEAD` is gone. After the merge the only remaining
working-tree change was the pre-existing `.ds4/report-t1.md` edit; I committed
that together with this report in the follow-up `docs(mrg1):` commit to leave
the tree clean (matching the repo's established merge-report convention).

## Deviations / uncertainties

- I included the pre-existing `.ds4/report-t1.md` working-tree edit in the
  follow-up docs commit rather than discarding it, so the tree is clean. It is
  the t1 seat's own report for this ticket and is unrelated to the conflict;
  discarding it would have destroyed that round's report, and the repo history
  already commits `.ds4` merge reports.
- The 2/8-seat merged head values are measured, not derivable from either side
  alone; the probe above is the evidence. If the daemon's heads gate disagrees,
  the probe can be re-run with the exact steps above.

## Issues

- None found beyond the conflict itself. The merge resolution did not surface a
  new engine defect. The two conflicted tests were both goldens that every
  head-moving change must update; no code defect was hidden by resolving them
  (the attribution probe shows the merged heads are exactly the superposition
  of the two documented changes, with no third behaviour moving).
