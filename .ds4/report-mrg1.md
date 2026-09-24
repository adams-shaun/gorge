# Merge-conflict resolution — task agent-20260923T065617Z-9b7a6efa

Ticket: `agent-20260923T065617Z-9b7a6efa` (CR 704.5k world-rule SBA)
Branch: `wt/agent-20260923T065617Z-9b7a6efa`
Merge commit: `1ec547623` (Merge branch 'main' into wt/agent-20260923T065617Z-9b7a6efa)
Pre-merge branch tip: `1c2409916`
Merged main tip: `747871c7a`
Merge base: `374a568b1`

## What was in flight

`git status` on entry was **clean**, on branch `wt/agent-20260923T065617Z-9b7a6efa`,
with **no in-flight rebase or merge** — the daemon's earlier integration attempt
had failed without leaving a partial state. I re-ran `git merge main` from the
clean tree.

## Conflicts

`git merge main` auto-merged every code path and produced exactly two conflicts,
both on tracked `.ds4/` shared report files (no source, test or spec file
conflicted):

### 1. `.ds4/report-t1.md` — CONFLICT (content)

- **Branch side (HEAD):** prepends this ticket's round-1 world-rule report
  (257 lines) above the accumulated shared history. Established by the branch's
  own `41c1348fe` ("restore shared report-t1 history") after a reviewer MAJOR
  found an earlier round had replaced the shared file wholesale.
- **Main side:** prepends the `agent-20260923T114033Z-a57ee463` damage-by-source
  provenance report (`# Report — game-long damage-by-source provenance …`) above
  the same shared history.
- Both sides converge byte-exactly at the `---` + `# Reports appended below …`
  separator (HEAD line 258 ≡ main line 544). Verified:
  `cmp` of the two shared-history tails → identical.

**Resolution:** union of both additions — HEAD's world-rule report, then main's
damage-provenance report, then the byte-exact shared history once. No lines from
either side were dropped.

### 2. `.ds4/report-t3.md` — CONFLICT (add/add)

- The file does not exist at the merge base; both sides created it independently.
- **Branch side:** this ticket's round-3 fix-round report (159 lines).
- **Main side:** the damage-provenance ticket's round-3 report (113 lines).
- Neither side carries a real shared-history body (the one `# Reports appended
  below` line on main is inside a fenced code block, a quoted example, not a
  separator).

**Resolution:** union — branch's t3 report, a `---` separator, then main's t3
report.

## Additive-only verification

Both resolved files are insertions-only against **both** parents, satisfying the
review convention (no shared history deleted):

```
$ git diff --numstat main -- .ds4/report-t1.md .ds4/report-t3.md
257	0	.ds4/report-t1.md
162	0	.ds4/report-t3.md

$ git diff --numstat HEAD^1 -- .ds4/report-t1.md .ds4/report-t3.md
543	0	.ds4/report-t1.md
116	0	.ds4/report-t3.md
```

(`HEAD^1` = branch tip `1c2409916`; the second column is deletions, always `0`.)

## Commands run

```
$ git status
On branch wt/agent-20260923T065617Z-9b7a6efa
nothing to commit, working tree clean

$ git merge main
Auto-merging .ds4/report-t1.md
CONFLICT (content): Merge conflict in .ds4/report-t1.md
Auto-merging .ds4/report-t2.md
Auto-merging .ds4/report-t3.md
CONFLICT (add/add): Merge conflict in .ds4/report-t3.md
Automatic merge failed; fix conflicts and then commit the result.

$ git add .ds4/report-t1.md .ds4/report-t3.md
$ git commit --no-edit
[wt/agent-20260923T065617Z-9b7a6efa 1ec547623] Merge branch 'main' into wt/agent-20260923T065617Z-9b7a6efa

$ git status
On branch wt/agent-20260923T065617Z-9b7a6efa
nothing to commit, working tree clean

$ git rev-list --parents -1 HEAD
1ec547623f110281adfde0380231b99780ce8d15 \
  1c240991639a0f9683ebd5d690b46390839969f7 \
  747871c7ab648fe2b3d028686fab22df9251d5f5
```

## Post-merge gates

The branch's fix (the global "newest-wins" CR 704.5k world-rule SBA, reworked in
`26f2e09d0`/`b902ed0d`/`482486c80`/`1c2409916`) is intact in the merged tree
(`rules/sba.go` `worldRule`/`worldPermanents`/`worldUnderLayers`, `cards/face.go`
`IsWorld`) alongside main's new files (`rules/morph_turnup.go`, `cmd/exitloop/…`).
`go build ./rules/ ./cards/ ./effects/` and `go vet ./rules/ ./cards/` are clean.

Conflicted packages (targeted):

```
$ go test -v -run 'TestWorldRule|TestLegend' ./rules/
=== RUN   TestLegendRuleAsksControllerWhichDuplicateToKeep
...
=== RUN   TestWorldRuleTieSendsAllUsesPreDepartureBoard
=== RUN   TestWorldRuleNewestSurvives
=== RUN   TestWorldRuleGlobalAcrossControllers
=== RUN   TestWorldRuleTieSendsAll
=== RUN   TestWorldRuleSinglePermanentIsUntouched
=== RUN   TestWorldRuleReadsDerivedSupertype
ok  	github.com/adams-shaun/gorge/rules	0.857s
```

16 tests ran, 0 failures, 0 skips (`.cards` symlink present).

Post-merge ratchets (main carries tests a branch cut before them has not met):

```
$ go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'
ok  	github.com/adams-shaun/gorge/rules	0.796s
```

5 tests ran, 0 skips:
`TestEveryRepoDeckIsFullySupported`, `TestEveryRepoDeckCountHeadResolves`,
`TestEveryRepoDeckParamsAreRead`, `TestEveryDispatchedTriggerModeHasAMatcher`,
`TestNoTriggerModeIsRegisteredThatTheSwitchNeverDispatched`. No ratchet table
edit was needed — this ticket registers no new `Mode$` matcher and closes no
`knownUnsupported`/`knownUnsupportedParams`/`knownUnmodelledCountHeads` entry
(it adds a new primitive, the world-rule SBA, not a matcher).

Behaviour goldens:

```
$ go test ./internal/archtest/
ok  	github.com/adams-shaun/gorge/internal/archtest	8.135s

$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok  	github.com/adams-shaun/gorge/cmd/botbench	1.269s
```

`TestHeads`/`rules/heads_test.go` were NOT edited by this resolution; no repo
deck carries a World card, so no chain head moved.

## Open concerns

None. Both conflicts were documentation-file unions with byte-exact shared
history preserved; no source behaviour was changed by the resolution, and every
targeted check passed on the merged tree.
# Merge-conflict resolution (round 2) — task agent-20260923T114033Z-a57ee463

Ticket: `agent-20260923T114033Z-a57ee463`
Branch: `wt/agent-20260923T114033Z-a57ee463`
Merge commit: `ab4ecf06e` (Merge branch 'main' into wt/agent-20260923T114033Z-a57ee463)
Pre-merge branch tip: `e60f9037e`
Merged main tip: `bb92e11d5`
Merge base: `0e7a6c220`

## What was in flight

`git status` at entry showed a **clean** tree on the branch tip: no
`MERGE_HEAD`, no rebase directory. The daemon's reflog shows it attempted
`git rebase main` twice and **aborted both** cleanly (a rebase replays the
branch's own merge commits and re-hits the conflict). So no operation was in
flight and I started the integration fresh with `git merge main`, which is the
integration shape this branch already uses (its history has two prior merge
commits from main).

## Conflicted files

Exactly three files conflicted, and all three are tracked `.ds4` report files
(`.ds4/` is gitignored, but these paths were force-added historically and so
are tracked):

- `.ds4/report-mrg1.md`
- `.ds4/report-t1.md`
- `.ds4/report-t2.md`

No Go source file conflicted. The daemon's rebase had named
`rules/player_target_qualifier_test.go` as the conflict; that does **not**
reproduce under a merge, because this branch already merged main once
(`0d9ee9802`) and therefore already carries the resolution of that file (both
the branch's bound source `src` and main's after-the-dot negation). The daemon's
rebase conflict is a rebase artifact, not a real merge conflict.

## What each side wanted, and how I resolved it

All three files are the shared append-only report history. Both sides had
rewritten their head of the file:

- **HEAD (branch)** carries this ticket's reports and, below them, main's
  reports appended verbatim under the header
  `# Reports appended below are from other tickets on the shared report file
  (preserved verbatim from main):`.
- **main** carries only main's most recent report for the file.

I verified mechanically that **main's entire content is an exact contiguous
suffix of HEAD's** for every one of the three files:

```
report-mrg1.md : main lines=138   ours-tail(suffix) md5 == main md5  -> EXACT SUFFIX
report-t1.md   : main lines=3750  ours-tail(suffix) md5 == main md5  -> EXACT SUFFIX
report-t2.md   : main lines=1405  ours-tail(suffix) md5 == main md5  -> EXACT SUFFIX
```

A unified diff of ours vs theirs printed only `<` (ours-only) lines and no `>`
(theirs-only) lines, confirming main adds nothing HEAD lacks. So the resolution
is **take HEAD's version** for all three: it preserves the branch's reports AND
main's (which HEAD already contains verbatim). I used
`git checkout --ours -- <file>` then `git add`. No content was hand-authored or
lost; no marker remains.

This is the same "union by taking the side that contains the other" resolution
the previous round used for reader-facing report files.

## Auto-merged source (checked, no conflict)

The merge auto-merged 39 source files carrying main's 50 newer commits (the
policynet pn06–pn11 series, the DigUntil withheld-rider change, searchteacher/
policytrain/exitloop, botbench) alongside the branch's provenance fix. I did not
hand-edit any of them. I confirmed the branch's fix survived intact in the
merged tree:

```
events/event.go:862   DamageProvenance (appended above NumKinds)
events/event.go:870   NumKinds = int(DamageProvenance) + 1
rules/engine.go:2548  e.emit(events.Event{Kind: events.DamageProvenance, Obj: src, ...})
rules/trigger_eligibility.go:87  events.DamageProvenance (zero-interest arm)
view/describe.go:175  case events.DamageProvenance
```

`git diff --stat HEAD^2 HEAD` shows exactly the branch's fix files added on top
of main (`effects/damage.go`, `effects/filter.go`, `events/*`, `rules/engine.go`,
`rules/heads_test.go`, `state/game.go`, `state/object.go`, `view/describe.go`,
plus the branch's tests), and `git diff --stat HEAD^1 HEAD` shows only main's
newer work coming in. No conflict marker exists anywhere in the tree
(`grep -rln '^<<<<<<< ' --include='*.go' .` -> none).

## Commands run and real output

Merge:

```
$ git merge main --no-edit
Auto-merging .ds4/report-mrg1.md
CONFLICT (content): Merge conflict in .ds4/report-mrg1.md
Auto-merging .ds4/report-t1.md
CONFLICT (content): Merge conflict in .ds4/report-t1.md
Auto-merging .ds4/report-t2.md
CONFLICT (content): Merge conflict in .ds4/report-t2.md
Automatic merge failed; fix conflicts and then commit the result.
```

Only three `UU` paths; no source conflict:

```
$ git status --short | grep -E '^(UU|AA|DU|UD|AU|UA|DD)'
UU .ds4/report-mrg1.md
UU .ds4/report-t1.md
UU .ds4/report-t2.md
```

Suffix proof (main is an exact suffix of HEAD for each):

```
$ for f in .ds4/report-mrg1.md .ds4/report-t1.md .ds4/report-t2.md; do
    n=$(git show :3:$f | wc -l)
    test "$(git show :2:$f | tail -n "$n" | md5sum)" = "$(git show :3:$f | md5sum)" \
      && echo "$f EXACT SUFFIX"; done
.ds4/report-mrg1.md EXACT SUFFIX
.ds4/report-t1.md EXACT SUFFIX
.ds4/report-t2.md EXACT SUFFIX
```

Resolution + commit:

```
$ git checkout --ours -- .ds4/report-mrg1.md .ds4/report-t1.md .ds4/report-t2.md
$ git add .ds4/report-mrg1.md .ds4/report-t1.md .ds4/report-t2.md
$ git commit --no-edit
[wt/agent-20260923T114033Z-a57ee463 ab4ecf06e] Merge branch 'main' into wt/agent-20260923T114033Z-a57ee463
$ git status
On branch wt/agent-20260923T114033Z-a57ee463
nothing to commit, working tree clean
```

Post-merge state:

```
$ git rev-list --left-right --count main...HEAD
0       8
$ git log -1 --format='%H %s' HEAD
ab4ecf06ef284be5e66c5d00451283c0d43fd6cf Merge branch 'main' into wt/agent-20260923T114033Z-a57ee463
```

Build + sanity checks (`.cards` present as a symlink to the shared corpus, so
the corpus-backed tests actually ran; the `rules` runs came back in the
0.6–1.6 s range, not a sub-5 s skip):

```
$ go build ./...
(no output)

$ go test ./rules/ -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'
ok  	github.com/adams-shaun/gorge/rules	0.609s

$ go test ./rules/ -run 'TestHeads$'
ok  	github.com/adams-shaun/gorge/rules	1.625s

$ go test ./rules/ -run 'TestEmitRecordsGameLongDamageProvenance|TestEmitRecordsObjectDamageProvenance|TestDiseasedVerminAskOffersOnlyPreviouslyDamagedOpponents|TestValidTgtsPurePlayerCensusPinsThePlayerQualifierSets|TestTriggerEventInterestMapping|TestDepartedChooserResumptionEventStreamIsDeterministic|TestLifelinkAbilityDamageToCreatureGainsLife|TestLegalTargetsRecheck|TestPlayerTarget|TestSleeperAgent'
ok  	github.com/adams-shaun/gorge/rules	0.748s
```

`TestHeads` passing on the merged tree confirms main's 50 newer commits did not
move the acceptance stream (the branch's measured head values from the previous
round still hold), so no golden was edited or re-pinned by this resolution.

## Ratchets

The brief-mandated post-merge ratchet run passes without any table edit: no new
`Mode$` matcher and no `knownUnsupported` / `knownUnsupportedParams` /
`knownUnmodelledCountHeads` entry was left behind by the merge (green above).

## Deviations / uncertainties

- No deviations from the conflict scope: I touched only the three conflicted
  `.ds4` files and made no behavioural change.
- The one judgement call is the report-file resolution, resolved by theorem
  (main is an exact suffix of HEAD) rather than by hand-merging prose. If the
  daemon instead wants the branch report to *precede* main's appended reports,
  the current file already satisfies that order.
- I did not run the full daemon gate suite (whole-module, CR conformance,
  `make sim`, `go vet ./...`) — those are the daemon's to run after DONE.

## Issues

- None from this resolution. The only non-conflict observation is that `.ds4/`
  report files are tracked while the rest of `.ds4/` is gitignored, which makes
  every round of this shared, ever-growing report history conflict on merge.
  That is a repo-process wart, not an engine defect; no CR rule applies.

---

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

---

# Reports appended below are from other tickets on the shared report file (preserved verbatim from main):

# Merge-conflict resolution report — agent-20260923T042552Z-0936e140

## What was in flight

`git status` at start: **clean, no merge/rebase in flight**. The daemon's
failed attempt had aborted cleanly. Branch was 125 commits behind `main` and 6
ahead. I re-ran `git merge main` to reproduce, which opened the conflict.

## Conflicted files

Exactly one file was left unmerged: `effects/cardflow.go`.

`rules/paramcensus_test.go` produced a git "Auto-merging ... CONFLICT (content)"
line during the merge attempt, but it auto-resolved cleanly (the branch deleted
the `digUntilWithheldRange` exemption helper while `main`'s changes there were
elsewhere in the file); it never appeared as an unmerged path.

## The two sides

Both sides touched `effDigUntil`'s revealed-rest move block:

- **Branch (`c749b700a`, "feat(effects): implement DigUntil withheld rider
  semantics")** implemented the *other* withheld riders: non-literal `Amount$`
  via `digUntilAmountSVar`, `Shuffle$`/`ShuffleCondition$`, `NoMoveFound$`,
  `FoundLibraryPosition$`, `ImprintFound$`/`ImprintRevealed$`, and
  `NoneFoundDestination$`/`NoneFoundLibraryPosition$`. To support the NoneFound
  branch it introduced local `restDest, restPos := revDest, revPos` and wrapped
  the rest-move loop in `if !noMoveRevealed {`. Its doc comment claimed
  "RevealRandomOrder$ remains a deterministic existing-order stand-in because
  ambient randomness is forbidden."

- **Main (`3b8f333c2`, "feat(effects): DigUntil RevealRandomOrder shuffles the
  bottom return")** implemented `RevealRandomOrder$ True` with a seeded
  Fisher-Yates (`h.Rand`) over a `toReturn` slice, applied when the revealed
  destination is the library and position is `-1`; a stay-in-place placement
  keeps existing order behind one loud Note. It also deleted the corresponding
  AGENTS.md Known-approximations row and updated the doc comment.

These are complementary, not contradictory: the branch's stand-in sentence was
made stale by main's later, deliberate change to exactly that line.

## How I resolved it

**Doc comment** (region 1): kept the branch's full "riders implemented" text
and replaced only the stale final sentence with main's
"RevealRandomOrder$ True is implemented for the library-bottom return
(h.Rand, seeded and replay-exact); a stay-in-place placement keeps the existing
order behind one loud Note."

**Code** (region 2): took the branch's `restDest`/`restPos` + `!noMoveRevealed`
structure and folded main's `toReturn` construction and `revealRandomOrder`
shuffle *inside* the `if !noMoveRevealed` block. The shuffle now gates on
`restDest == state.ZLibrary` and `restPos` (the branch's rest-placement
variables) rather than main's `revDest`/`revPos`, so a NoneFound-library
placement would also get the random bottom return — the natural integration of
the two features. The remainder of the loop keeps the branch's `restDest`/
`restPos` moves, including the `RevealedLibraryPosition$` Note text (restPos).

One stray `}` remained from main's hunk boundary after the fold; removed it.
No conflict markers remain; `gofmt -l` is clean; `go build ./effects/` passes.

I did **not** touch any file outside `effects/cardflow.go`, and made no
behavioural change beyond joining the two sides.

## Commands and output

```
$ git status
On branch wt/agent-20260923T042552Z-0936e140
nothing to commit, working tree clean
$ git rev-list --left-right --count main...HEAD
125     6

$ git merge main
Auto-merging effects/cardflow.go
CONFLICT (content): Merge conflict in effects/cardflow.go
Auto-merging rules/paramcensus_test.go
Automatic merge failed; fix conflicts and then commit the result.

$ grep -n '<<<<<<<\|=======\|>>>>>>>' effects/cardflow.go   # after resolution
(none)

$ gofmt -l effects/cardflow.go
(clean)
$ go build ./effects/
(ok)

$ go test -run 'DigUntil|RevealRandom' ./effects/
ok  github.com/adams-shaun/gorge/effects  0.426s

$ go test -run 'DigUntil|RevealRandom' ./rules/
ok  github.com/adams-shaun/gorge/rules  0.684s

$ go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'
ok  github.com/adams-shaun/gorge/rules  1.226s

$ go test ./internal/testutil/ -run 'TestKnownApproximation|TestAgentsDoc'
ok  github.com/adams-shaun/gorge/internal/testutil  0.002s

$ git commit --no-edit
[wt/agent-20260923T042552Z-0936e140 091247850] Merge branch 'main' into wt/agent-20260923T042552Z-0936e140

$ git status
nothing to commit, working tree clean
$ git log --oneline -1 --parents
091247850 f415551e0 76dd677b3 Merge branch 'main' into wt/agent-20260923T042552Z-0936e140
$ git rev-list --left-right --count main...HEAD
0       7
```

`.cards` was present as a valid symlink to the shared corpus
(`.cards -> /home/sadams/projects/gorge/.cards`), so the corpus-backed DigUntil
tests actually ran (effects 0.426s, rules 0.684s — not a sub-5s skip).

## Ratchets

All four post-merge ratchets pass (no new `Mode$` matcher and no ratchet entry
moved by this integration), `TestKnownApproximationsOnlyShrinks` passes (main's
deletion of the RevealRandomOrder row is consistent with the constant), and
`TestEveryRepoDeck`/`TestEveryRepoDeckParams` are green — the merge did not move
coverage.

## Uncertainties

None material. The one judgement call is using `restDest`/`restPos` (rather than
`revDest`/`revPos`) as the shuffle's gate, so a `NoneFoundDestination$ Library`
scan that finds nothing also randomises its bottom return. That is the only
reading that keeps both features coherent; a corpus card combining
`RevealRandomOrder$` with `NoneFound*` would exercise it, but no test asserts it
either way.

## Issues

No new defects found. This ticket was integration-only.

STATUS=DONE
COMMITS=09124785037bb0af2b9494993a0912bc38668f7d
TESTS=go test -run 'DigUntil|RevealRandom' ./effects/ ./rules/ (ok); post-merge ratchets + TestKnownApproximation (ok)
