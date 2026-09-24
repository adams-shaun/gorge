# scryrepl-order — VERIFY AND PIN: CR 616.1 order choice for competing Scry replacements

Branch `wt/scryrepl-order`, worktree `.worktrees/scryrepl-order`, base `main`.

**Outcome: verify-and-pin only. No production change. No new test (the one named
gap is unbuildable with real corpus cards — proven below).** A durable,
task-unique copy of this report is committed (see Rules below); the tree carries
no non-test change and no test change.

## Workspace / premises re-measured

- `.cards` **is present** in this worktree as a symlink to
  `/home/sadams/projects/gorge/.cards` (not the vacuous-skip signature).
  `ls -la .cards` → `cards -> /home/sadams/projects/gorge/.cards`.
- Fix commit `8d1151fb` ("fix(rules): let affected player order competing scry
  replacements") is on the branch: `git merge-base --is-ancestor 8d1151fb HEAD`
  succeeded. Its test file is `rules/scry_replacement_order_test.go` (added in
  that commit). Pin tests present and green at branch tip.
- Corpus prevalence held exactly: `R:Event$ Scry` carriers are **2** files, both
  printed `R:` lines:
  ```
  $ /usr/bin/grep -rl 'R:Event\$ Scry' .cards/cardsfolder
  .cards/cardsfolder/k/kenessos_priest_of_thassa.txt
  .cards/cardsfolder/e/eligeth_crossroads_augur.txt
  $ /usr/bin/grep -rh 'Event\$ Scry' .cards/cardsfolder
  .../kenessos_priest_of_thassa.txt:R:Event$ Scry | ActiveZones$ Battlefield | ValidPlayer$ You | ReplaceWith$ ScryP1 | ...
  .../eligeth_crossroads_augur.txt:R:Event$ Scry | ActiveZones$ Battlefield | ValidPlayer$ You | ReplaceWith$ Draw | ...
  ```
  Both carry `ValidPlayer$ You`, so the competition is reachable exactly when one
  player controls both (the Partner-commander board the existing tests build).

## What main already does (spot-checked, unchanged)

`rules/replacement.go`: `applyReplacementsDispatch` routes `events.Scry` to
`continueScryReplacements` (line 375). `continueScryReplacements` (line 2880)
rechecks applicability per iteration, parks a `replChoiceScry` when
`len(applicable) > 1` and the scrying player is live (line ~2896), and applies
each survivor once through the `used []bool` slice. `Engine.Scry` (line ~2843)
returns `(0, false, true)` while the proposal is parked, so no library is
inspected. The ask is `scryReplacementDecision`, resumed by the
`"scry_replacement"` case in `rules/resolution.go`. No change made.

## Fails without the fix

Method (as the brief directed): a disposable export of the fix's parent, with
`.cards` symlinked in and the three test files copied verbatim. No `git stash`,
no branch move.

```
$ mkdir -p .ds4/scratch/prefix && git archive 8d1151fb^ | tar -x -C .ds4/scratch/prefix
$ ln -sfn /home/sadams/projects/gorge/.cards .ds4/scratch/prefix/.cards
$ cp rules/scry_replacement_order_test.go rules/scry_replacement_test.go \
     rules/scry_replacement_dredge_test.go .ds4/scratch/prefix/rules/
$ cd .ds4/scratch/prefix
$ go test -v -run 'TestScryReplacementOrderChoiceAndResume|TestScryReplacementOrderDrawDredgeResumesSpell|TestKenessosReplacesScryCountBeforeLooking|TestKenessosDoesNotReplaceTheCompletedScryRecord|TestEligethDrawsInsteadOfScrying' ./rules/
```

Result — the three single-replacement tests PASS (the fix they need predates
8d1151fb), while **both table entries** of the order pin and the Dredge pin
FAIL:

```
=== RUN   TestScryReplacementOrderDrawDredgeResumesSpell
--- FAIL: TestScryReplacementOrderDrawDredgeResumesSpell (0.41s)
    scry_replacement_dredge_test.go:78: precondition: Scry spell must be parked on affected player's order choice: &{Seq:63 Player:0 Kind:modes Prompt:Replace draw with Dredge? ... ResumeKind:dredge ...}
=== RUN   TestScryReplacementOrderChoiceAndResume
=== RUN   TestScryReplacementOrderChoiceAndResume/kenessos_first
=== RUN   TestScryReplacementOrderChoiceAndResume/eligeth_first
--- FAIL: TestScryReplacementOrderChoiceAndResume (0.00s)
    --- FAIL: TestScryReplacementOrderChoiceAndResume/kenessos_first (0.00s)
        scry_replacement_order_test.go:68: missing affected-player Scry replacement-order choice: pending=priority options=13 spell zone=graveyard
    --- FAIL: TestScryReplacementOrderChoiceAndResume/eligeth_first (0.00s)
        scry_replacement_order_test.go:68: missing affected-player Scry replacement-order choice: pending=priority options=13 spell zone=graveyard
=== RUN   TestKenessosReplacesScryCountBeforeLooking
--- PASS: TestKenessosReplacesScryCountBeforeLooking (0.00s)
=== RUN   TestKenessosDoesNotReplaceTheCompletedScryRecord
--- PASS: TestKenessosDoesNotReplaceTheCompletedScryRecord (0.00s)
=== RUN   TestEligethDrawsInsteadOfScrying
--- PASS: TestEligethDrawsInsteadOfScrying (0.00s)
FAIL
FAIL	github.com/adams-shaun/gorge/rules	0.477s
FAIL
exit=1
```

The pre-fix code applied the matches in scan order (the spell resolved straight
to the graveyard, `pending=priority`, so no `decision.KReplacement` was ever
posed). This is exactly the reported failure mode and it reproduces on **both**
order subtests. The Dredge pin's precondition (`Spell must be parked on affected
player's order choice`) catches the same missing ask before reaching Dredge.

At the branch tip the same command is green:

```
$ go test -run 'TestScryReplacementOrderChoiceAndResume|TestScryReplacementOrderDrawDredgeResumesSpell|TestKenessosReplacesScryCountBeforeLooking|TestKenessosDoesNotReplaceTheCompletedScryRecord|TestEligethDrawsInsteadOfScrying' ./rules/
ok  	github.com/adams-shaun/gorge/rules	0.471s
exit=0
```

## Gap check (brief item 2) — named gap is unbuildable with real cards

The brief's only named gap is a competition where one source arrives by the
ordinary printed-face scan and the other by the Effect-created scan
(`scryReplacementMatches`, `rules/replacement.go:2947`, `m.key != ""` branch at
2948). I measured that this board **cannot be built from the corpus**:

1. **No corpus card has an Effect-created Scry replacement.** The only two
   `Event$ Scry` occurrences in the whole corpus are the printed `R:` lines on
   Kenessos and Eligeth (grep above). There is no `ReplacementEffects$ <SVar>`
   whose SVar carries `Event$ Scry`.
2. **The Effect-created Scry shape is not even registrable.** `effects/misc.go`
   (~line 555) only registers a `ReplacementEffects$` SVar as a live continuous
   replacement when `event == "DamageDone"` or a `Moved`/`CreateToken` body
   matches; any other event (including `Scry`) falls to the
   `"continuous replacement unimplemented (...)"` Note and is never added to
   `e.active()`. So the `m.key != ""` arm of `scryReplacementMatches` has zero
   writers for the Scry event, today or after any corpus move that leaves the
   registration gate unchanged.

Per the brief ("if you cannot build it with real cards, say so in the report
instead of inventing a synthetic face"), **no synthetic Effect-created Scry
match was invented and no table entry was added.** The existing table already
pins the two reachable properties that gap would test: the ask is posed
(`len(d.Options) == 2`), the source scanned **second** can be chosen first
(`eligeth_first` selects `el`, which is appended after `k`), and each survivor
applies exactly once (net draw 4 vs 3).

## Behaviour goldens and gates

```
$ go test ./internal/archtest/
ok  	github.com/adams-shaun/gorge/internal/archtest	3.499s

$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok  	github.com/adams-shaun/gorge/cmd/botbench	0.648s
```

Botbench split did not move (expected: no engine behaviour change).

```
$ git status --short      # (empty)
$ git diff --stat         # (empty)
$ gofmt -l rules/         # (empty)
```

No non-test file modified.

## Rules / commits

`.ds4/` is gitignored. The dispatch-mandated reviewer handoff is
`.ds4/report-t1.md` (working copy only — that path is a *tracked* report for an
older ticket, `agent-20260919T181318Z-86535368`; its historical HEAD content is
left unmodified and is not part of this branch's commit). The durable,
task-unique committed copy is `.ds4/report-t1-scryrepl-order.md`, following the
`report-sol1-scry-repl.md` precedent.

No production or test file changed on this branch; the only committed file is
the durable report, so the branch tip equals `main` plus that report. Fix commit
`8d1151fb` is an ancestor of `main` and of this branch; nothing here re-applies
it.

## Issues

- **Non-Scry competing-replacement pair that skips the CR 616.1 order ask
  (NOT FIXED, outside this brief).** `rules/replacement.go:2781`
  `continueExploreReplacements` takes `matches[0]` and applies it
  deterministically — there is no `replChoice` parking and no
  `decision.KReplacement` for the `Explore` event. The corpus has exactly two
  competing `R:Event$ Explore` carriers, both `ValidExplorer$ Creature.YouCtrl`,
  so one player controlling both reaches the competition:
  - `Topography Tracker` — "If a creature you control would explore, instead it
    explores, then it explores again" (`ReplaceWith$ Explore1`, `DB$ Explore |
    Num$ 2`), `.cards/cardsfolder/t/topography_tracker.txt`
  - `Twists and Turns` — "If a creature you control would explore, instead you
    scry 1, then that creature explores" (`ReplaceWith$ DBScry` → `DB$ Scry |
    SubAbility$ DBExplore`), `.cards/cardsfolder/t/twists_and_turns_mycoid_maze.txt`

  The outcomes do not commute (explore-twice vs scry-then-explore), so CR 616.1
  says the exploring creature's controller chooses. **Expected decision kind:
  `decision.KReplacement`, owned by the explorer's controller.** Measured with a
  disposable probe (both sources on the battlefield, `Engine.ExploreReplaced`
  driven directly): `replChoices=0` and no `KReplacement` was pending — the
  scan-first body (Topography Tracker) applied and the explore proceeded to its
  own keep/graveyard ask. Corpus prevalence:
  `/usr/bin/grep -rlE 'R:Event\$ Explore' .cards/cardsfolder | wc -l` → `2`.
  This is the generalize-from-Scry sibling the brief asked me to name; a CR-lane
  test citing CR 616.1 would make it visible to the ledger. Not fixed here.
- **The `m.key != ""` (Effect-created) arm of `scryReplacementMatches` has no
  Scry writer.** It is correct by inspection and unreachable in the current
  corpus (see Gap check). Worth a note only so a future ticket adding an
  Effect-created `R:Event$ Scry` knows the ordering park in
  `continueScryReplacements` is the shared path it must go through.
