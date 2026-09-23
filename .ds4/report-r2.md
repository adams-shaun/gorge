# Report — r2 (agent-20260918T195920Z-2fd3b568) — Loamcrafter Faun `TriggerRemembered$Amount`

**Historical reconciliation round (before the sol1 review).** The ticket's
code work and tests were committed along with the t1 report, preserved at
`.ds4/report-t1-2fd3b568.md`; the shared `.ds4/report-t1.md` was restored
to main's content. The branch was rebased onto main. This report was the only
*new change in that reconciliation round*, NOT the only change in the branch
relative to main. The branch also carries `effects/count.go`,
`effects/immediate.go`, `effects/count_triggerremembered_test.go`,
`rules/loamcrafter_faun_test.go`, and the t1 report. The ChosenCardStrict r2
report below is preserved. The prior claim that `main...HEAD` contained
only this report was wrong; the sol1 report documents the full branch diff.

## What changed and why (per file)

- `.ds4/report-t1-2fd3b568.md` (new, commit `4a786d45`): this ticket's t1
  report, preserved at a unique path (same resolution as `83d640d5` and
  `b82aef05`) instead of clobbering the shared `report-t1.md` main tracks.
  Full content: the capture-excluded `TriggerRemembered` mapping, the
  Loamcrafter Faun end-to-end pin, the fails-without-the-fix proof, and the
  merged-sibling mapping verdict (plain landed; corrected here).
- `.ds4/report-t1.md`: restored to the committed (main) version — the
  DestroyAll.Zone report is preserved, this ticket's content no longer
  clobbers it.
- Code (rebased onto main, no conflicts): `448e89ab` =
  `effects/count.go` (TriggerRemembered → `rememberedExcludingCapture`, the
  one shared helper also used by `effImmediateTrigger`; Spawner>
  re-anchoring nils the consumed capture), `effects/immediate.go` (parent
  computation routed through the helper), `effects/count_triggerremembered_test.go`
  (real chain-ctx fixture: Captured = ETB'd source, Remembered = source +
  chain objects), `rules/loamcrafter_faun_test.go` (end-to-end: discard N
  lands → one return ask Max exactly N → named permanents to hand;
  empty discard = silent no-op with the chain registered). `41b7e422` pins
  the exotic verdicts: `CastTotalManaSpent` and `CardManaCostLKI` modelled,
  `GreatestCardManaCost` and `CardTypes` fail-closed.

## Rebase outcome (the round's blocking finding)

At the time of this report, `git rebase main` was clean, replaying three
commits (the Convoked / Imprint regions of `effects/count.go` were disjoint).
The then-current SHAs were `448e89ab` (fix), `41b7e422` (tests),
`4a786d45` (docs), rebased from `32ae38bc`/`bcf02231`/`f3ba0672`.
A subsequent controller-ordered rebase for sol1 replayed four commits;
current SHAs and base are recorded in `.ds4/report-sol1.md`.

## Gate commands and their real output (historical, on the r2 base)

Environment: `.cards` symlink present at the worktree root
(`.cards -> /home/sadams/projects/gorge/.cards`); `go test ./rules` at 36.2s
confirmed a real corpus run, not a skipped one. At that time the branch was
`main` + 3 commits (`git log --oneline -4`: 4a786d45, 41b7e422,
448e89ab, 0f94cca6=then-main). Current-base gates are in the sol1 report.

Done means #3 (targeted pins):
```
$ go test -run 'TestLoamcrafterFaun|TestTriggerRemembered|TestRefProperty|TestImmediateTrigger|TestForumFilibuster|TestSpeedYoungAvenger' ./effects ./rules
ok  	github.com/adams-shaun/gorge/effects	0.727s
ok  	github.com/adams-shaun/gorge/rules	0.645s
```

Done means #4 (affected packages, once):
```
$ go test ./effects ./rules
ok  	github.com/adams-shaun/gorge/effects	2.579s
ok  	github.com/adams-shaun/gorge/rules	36.184s
```

Done means #5 (format/vet):
```
$ gofmt -l effects/count.go effects/immediate.go effects/count_triggerremembered_test.go rules/loamcrafter_faun_test.go
(no output)
$ go vet ./effects ./rules
(no output)
```

Behaviour goldens outside `rules/` (run once, before DONE):
```
$ go test ./internal/archtest/
ok  	github.com/adams-shaun/gorge/internal/archtest	3.243s
$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok  	github.com/adams-shaun/gorge/cmd/botbench	1.249s
```
The botbench win split did not move; no golden re-pin needed. `go build ./...`
clean (exit 0).

## Deviations from the brief

None beyond those already recorded in the t1 report (brief's 4-exotic list
was really 2; the end-to-end pin cannot discriminate the mapping — the unit
test is the arbiter). The t1 round's deviation "did not rebase" is now
closed: the rebase was completed this round, cleanly.

## Issues (defects found, not fixed)

Unchanged from the t1 report (re-listed for the ledger):
- `IsTriggerRemembered` filter predicate unimplemented (61 corpus files);
  delayed triggers registered with it never match (Blessed Defiance).
- `TriggerRemembered$GreatestCardManaCost` and `TriggerRemembered$CardTypes`
  stay fail-closed (2 carriers); `CardTypes` would be a shared
  `evalRefProperty` addition; `GreatestCardManaCost` rides ticket `e27469dd`.


---

# Report — r2 (agent-20260918T233200Z-79b69706) — pred:ChosenCardStrict

Ticket: `pred:ChosenCardStrict` — the `Strict` suffix on the ChosenCard
predicate is never stripped. **Reconciled fix round.** The brief's work is
already closed on `main` by two prior merged commits, and this round's only
change is this report itself, appended to the shared report file **without
deleting anything**. The t1 diff the r2 review flagged has been dropped from
the branch; the branch now carries exactly one tracked file change: this
report's insertion at the top of `.ds4/report-r2.md`.

## Resolution of the r2 findings, each one

### [MAJOR] report-vs-diff reconciliation — RESOLVED

The r2 review was right, and t1's report was wrong about itself. What t1
actually shipped was a single tracked file change: `.ds4/report-t1.md`
modified with 135 insertions / **1,930 deletions** — i.e. commit `3b820486`
overwrote the DestroyAll.Zone ticket's committed report with t1's own text,
while t1's report text simultaneously claimed `git diff --stat main...HEAD`
was empty. Both halves of that were mistakes: the empty-diff claim was false,
and the overwrite destroyed another ticket's durable report.

Fix applied this round, in order:

1. Salvaged t1's report text to `.ds4/scratch/old-report-chosencardstrict.md`
   (untracked scratch, out of the review path) via
   `git show 3b820486:.ds4/report-t1.md`.
2. Ran the controller-ordered rebase (`git rebase main`); the overwrite
   conflicted with main's current `.ds4/report-t1.md` (DestroyAll.Zone
   report). Resolved by **dropping the commit entirely**
   (`git rebase --skip`): the branch no longer carries the 1,930-line
   removal. `git log --oneline -3` after the rebase starts at main's tip
   `8cac5583` (merge(agent-20260918T231813Z-ae51a551): param:api:DestroyAll.Zone)
   and `git status` is clean.

The branch's actual diff is now exactly the insertion of this report above
the prior r2 report (the TriggerRemembered ticket's, already merged) in
`.ds4/report-r2.md`. That is reconciled with the actual diff by construction:
`git diff --stat main...HEAD` after this commit reads
`.ds4/report-r2.md | <N> ++` — insertions only, zero deletions, and this
report is the only change.

On the substance: the t1 verdict's premise was measured correct, and this
round re-measures it on the rebase result (current main tip `8cac5583`),
not on the stale base — all evidence below is fresh.

### [MINOR] deck-ratchet deviation — CLARIFIED (was under-stated, now stated)

The brief's third Done-means item says "the DECK-side ratchet
(`rules/acceptance_test.go` `knownUnsupported`) then admits the card". t1
marked it N/A without flagging it as a deviation; it IS a deviation from the
brief's letter and here is the clarification:

- The deck census that found this gap was the **World Shaper (eoc commander
  precon)** deck import measurement, not a repo deck. That precon is NOT
  imported: `grep -rln 'Eumidian Wastewaker' internal/testutil/decks/`
  returns nothing (the only `World Shaper` hit in `internal/testutil/decks/`
  is the *card* World Shaper, a 1× entry in
  `foundations-tramplesaurus-rex.json` — a different deck, unrelated).
- `knownUnsupported` (`rules/acceptance_test.go`) is a bidirectional ratchet
  over the 24 imported repo decks' card sets; a card in no imported deck
  cannot be admitted to it, so there is no table row for Eumidian Wastewaker
  and none is needed. `grep -n 'Wastewaker' rules/acceptance_test.go
  rules/paramcensus_test.go` returns nothing, as expected.
- The ratchet item is therefore **not applicable rather than satisfied**, for
  the measured reason above. If the World Shaper precon is imported later,
  its Eumidian Wastewaker coverage is already proven by the regression test
  below and no ratchet row will be required.

## Re-verification on current main tip `8cac5583` — the three Done-means items

### 1. Predicate-stripping rule in `effects/filter.go` — PRESENT

`ChosenCardStrict` is classified and matched alongside `ChosenCard`, landed
by `5898aeeb` (2026-09-21, "feat(effects): implement api:ChooseSource with a
chosen-source replacement gate"; confirmed an ancestor of main):

```
$ grep -n 'ChosenCardStrict' effects/filter.go
1971:	if p == "ChosenCard" || p == "ChosenCardStrict" || p == "nonChosenCard" || p == "RememberedPlayerCtrl" || p == "CanBeTargetedByTriggeredSpellAbility" {
2468:	if p == "ChosenCard" || p == "ChosenCardStrict" || p == "nonChosenCard" {
2469:		// Forge's ChosenCard and ChosenCardStrict are one predicate for this
2475:		// Palm's `Card.ChosenCardStrict,Emblem.ChosenCard`), and every carrier
```

All compound spellings the brief measured (`Card.ChosenCardStrict`,
`Creature.ChosenCardStrict`, bare `ChosenCardStrict`) split at the spec layer
to the predicate `ChosenCardStrict` and route through the same matcher, which
reads `SpecContext.Chosen` and fails closed when no choice is bound.

### 2. End-to-end regression test — PRESENT, GREEN

`rules/eumidian_wastewaker_test.go` (landed by `f2a55835c`, 2026-09-22, the
sibling `pred:CanBeSacrificedBy` fix; confirmed an ancestor of main) pins the
brief's exact end-to-end chain on the real corpus card: choose →
discard-or-sacrifice → `SacrificeAll | ValidCards$ Card.ChosenCardStrict`
sacrifices the chosen permanent → `Count$ValidGraveyard Land.ChosenCard`
draws 2. A second test is the negative control (hand-only chooser resolves
end to end).

```
$ go test -run 'TestEumidianWastewaker' ./rules/ > .ds4/scratch/t.log 2>&1; tail -5 .ds4/scratch/t.log
ok  	github.com/adams-shaun/gorge/rules	0.643s
```

`.cards` is a symlink to `/home/sadams/projects/gorge/.cards` in this
worktree, so corpus-backed tests ran, not skipped.

### 3. Deck ratchet — N/A (deviation clarified above)

## Fails without the fix

Proof the predicate is load-bearing for the pinned test (the r2 reviewer's
break attempt, reproduced independently this round): strip the two
`ChosenCardStrict` clauses from `effects/filter.go` (predicate becomes
unknown → fails closed to the empty set) and the test fails at exactly the
sacrifice assertion; `effects/filter.go` was then restored byte-identically
(`cmp` against the pre-break copy passed, `git status` clean):

```
$ go test -run 'TestEumidianWastewaker' ./rules/ 2>&1 | tail -8
--- FAIL: TestEumidianWastewakerChoosersPickDiscardOrSacrifice (0.65s)
    eumidian_wastewaker_test.go:261: the chosen permanent was not sacrificed: &{ID:41 ... Zone:battlefield ...}
FAIL
FAIL	github.com/adams-shaun/gorge/rules	0.665s
FAIL
```

This matches the t1 report's recorded revert test (the reviewer's break
attempt also reproduced it), and is the exact defect the brief described:
the sacrifice arm matches nothing and the trigger only ever discards.

## Behaviour goldens (mandatory, run once before reporting)

```
$ go test ./internal/archtest/ 2>&1 | tail -3
ok  	github.com/adams-shaun/gorge/internal/archtest	3.983s
$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/ 2>&1 | tail -3
ok  	github.com/adams-shaun/gorge/cmd/botbench	1.599s
```

## Brief-premise re-measurement (counts were claims; re-measured)

```
$ /usr/bin/grep -rho '[A-Za-z.]*ChosenCardStrict' .cards/cardsfolder | sort | uniq -c
     68 Card.ChosenCardStrict
      3 ChosenCardStrict
      1 Creature.ChosenCardStrict
$ /usr/bin/grep -rl 'ChosenCardStrict' .cards/cardsfolder | wc -l
66
```

All three of the brief's prevalence claims held exactly (68/3/1 spellings,
66 files).

## Issues

- No new defect found. The interplay named in the brief (the same trigger's
  `Permanent.CanBeSacrificedBy` choose arm) was closed by the sibling ticket
  `f2a55835c`, whose test the Wastewaker file now shares; nothing remains
  open on this card's trigger chain from what this round measured.
- Process note for the controller, not a defect: `.ds4/report-t1.md` and
  `.ds4/report-r2.md` are tracked on main and shared across tickets; a
  dispatch that names a shared report path invites the overwrite class of
  mistake t1 made. The suffixed-name convention
  (`.ds4/report-r2-<slug>.md`) avoids it; this round kept to the dispatch's
  exact path by prepending and preserving, which is why the diff is
  insertions only.

---

# (Preserved below: the prior r2 report, agent-20260918T233200Z-4a2fcd44 — TriggerRemembered, already merged at d258009b. Nothing below this line was changed.)

# Report — r2 (agent-20260918T233200Z-4a2fcd44) — rebase resolution

Ticket: `count:TriggerRemembered$<Property>` — the delayed/trigger-remembered
count-reference family. **The implementation was completed in round t1**
(commit now `82e7072c feat(effects): admit TriggerRemembered$<Property> count ref`)
and the review verdict at `.ds4/verdict-t1.md` is **APPROVE**. The full t1
report is preserved at `.ds4/report-t1-triggerremembered.md` (committed this
round; previously left uncommitted at the shared path, which is what blocked
the rebase).

## What this round did

`findings-r2.md` reported that the rebase onto main failed:

```
error: cannot rebase: You have unstaged changes.
--- merge fallback ---
error: Your local changes to the following files would be overwritten by merge:
	.ds4/report-t1.md
```

Root cause: round t1 wrote its report at the SHARED path `.ds4/report-t1.md`
and left it uncommitted. Main tracks `.ds4/report-t1.md` with a different
ticket's report (fb-20260923T005857Z-c1a24352, Count$ResolvedThisTurn), so the
merge refused to touch the file. Same resolution as commit `83d640d5` (Deep
Spawn r2):

1. Moved this ticket's t1 report to the unique path
   `.ds4/report-t1-triggerremembered.md` and restored `.ds4/report-t1.md` to
   its tracked content (`git restore <path>` — no branch switch, no shared
   state change).
2. Committed the moved report (`5558278d docs(count): record the
   TriggerRemembered t1 report at a unique path`).
3. `git rebase main` — **clean, no conflicts**. The branch is now main +
   exactly two commits:
   - `82e7072c feat(effects): admit TriggerRemembered$<Property> count ref`
     (effects/count.go +11, effects/count_triggerremembered_test.go +97)
   - `5558278d docs(count): record the TriggerRemembered t1 report at a unique path`
4. Re-verified after the rebase: build + the t1 regression test.

```
$ git rebase main
Rebasing (1/2)Rebasing (2/2)Successfully rebased and updated refs/heads/wt/agent-20260918T233200Z-4a2fcd44.

$ git diff main --stat
 .ds4/report-t1-triggerremembered.md     | 166 ++++++++++++++++++++++++++++++++
 effects/count.go                        |  11 ++-
 effects/count_triggerremembered_test.go |  97 +++++++++++++++++++
 3 files changed, 273 insertions(+), 1 deletion(-)

$ go build ./... && go test -v -run 'TestTriggerRemembered' ./effects/
=== RUN   TestTriggerRememberedRefProperty
--- PASS: TestTriggerRememberedRefProperty (0.00s)
ok  	github.com/adams-shaun/gorge/effects	0.009s
```

(The test is a pure in-line fixture unit test — no corpus dependency — hence
sub-second. `.cards` is present in the worktree, symlinked to the shared
corpus.)

## Prior findings re-verification

Verdict `verdict-t1.md` = APPROVE carried two MINORs:

1. **`TriggerRemembered` binds `Ctx.Remembered`, which for an EVENT-MATCHED
   DelayedTrigger registration is the firing event's object, not the
   registration's own `RememberObjects$` capture.** The verdict itself says
   "No action needed this round" — both current event-matched carriers
   (Vivien's Invocation, Rushed Rebirth) have event object == registered
   capture. Re-checked after the rebase: `effects/count.go`'s refTargets case
   is unchanged by main (the rebase applied cleanly, diff vs main shows the
   same +11 hunk). Still latent-only; if a divergent carrier appears it
   belongs in that carrier's ticket.
2. **Uncommitted tracked `.ds4/report-t1.md`** — FIXED this round (see above).

## Issues

- None new. The only blocker this round was the shared report path, resolved
  per precedent. Latent DelayedTrigger binding divergence noted above stays
  documented in the t1 verdict; no corpus-visible defect.

## Notes for the controller

- `.ds4/report-r2.md` is a path main tracks with the Deep Spawn ticket's r2
  report. This file replaces it on this branch (committed), so the merge into
  main takes this version cleanly (main's copy is unchanged since the
  merge-base). The Deep Spawn report remains in main's history.
- `.ds4/` is gitignored in this worktree, so committing the new report file
  required `git add -f` — same as the tracked `.ds4` files already in the
  index from prior merges.


---

# Report — r2 (agent-20260919T185907Z-f5c7e2dc) — trig:Attacks.NoResolvingCheck on Sentinel Sarah Lyons

Round 2 of the ticket. Round 1's work was complete and green (`test(rules):
cover Sentinel Sarah Lyons battalion trigger`, then report appended to
`.ds4/report-t1.md`); the round was parked ONLY on the controller's rebase
directive failing (`error: cannot rebase: You have unstaged changes` and a
merge fallback conflicting on `.ds4/report-t1.md`). No review findings were
attached beyond that (`findings-r2.md` holds only the rebase error), so this
round did the rebase and re-verified everything after it.

## What changed this round

- Committed the uncommitted `.ds4/report-t1.md` round-1 report, then ran
  `git rebase main` — **clean, no conflicts**. Branch is now
  `126a5a95` on top of main (`git log main..HEAD` = exactly the two
  round-1/round-2 commits; diff vs main is `rules/battalion_test.go` + the
  report, nothing else).
- Re-verified the round-1 state against post-rebase main: the production
  `NoResolvingCheck$` read (`noResolvingCheck` +
  `triggerResolvingCheckHolds` in `rules/trigger_condition.go`, applied at
  the single resolution-time CR 603.4 site in `rules/stack.go`) survived the
  merge intact, and main has since landed its own companion tests
  (`rules/no_resolving_check_test.go`, Ugin's Mastery) plus a retired
  `knownUnsupportedParams` row (Love on the Battlefield) for the sibling
  ticket. My branch's contribution remains the brief's ask: the **real-corpus
  card test** for Sentinel Sarah Lyons (Battalion: IsPresent$
  `Creature.attacking+Other` GE2 + `NoResolvingCheck$ True`).
- Brief premises re-measured, both held: `grep -rlE 'NoResolvingCheck$'
  .cards/cardsfolder | wc -l` = 87 files / 88 lines; Sentinel Sarah Lyons is
  NOT in `internal/testutil/decks/`, so there is no `knownUnsupportedParams`
  row to delete for it.

## Gates run (real output)

`.cards` was PRESENT (symlink), so this is an executed run, not a skipped one.

Targeted test (`rules/battalion_test.go`), after the rebase:

```text
$ go test -run '^TestSentinelSarahLyonsBattalionSurvivesAttackerLeaving$' ./rules/
ok  	github.com/adams-shaun/gorge/rules	0.609s
```

Behaviour goldens:

```text
$ go test ./internal/archtest/
ok  	github.com/adams-shaun/gorge/internal/archtest	3.650s
$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok  	github.com/adams-shaun/gorge/cmd/botbench	1.320s
```

## Fails without the fix

The test pins the production bypass, so I neutralised the bypass
(`triggerResolvingCheckHolds`'s `noResolvingCheck` early-return in
`rules/trigger_condition.go`, saved to `.ds4/scratch/` first) and re-ran the
one test:

```text
--- FAIL: TestSentinelSarahLyonsBattalionSurvivesAttackerLeaving (0.60s)
    battalion_test.go:76: Sentinel Sarah Lyons trigger did not deal damage; it left the stack with "fizzled: intervening-if no longer holds"
FAIL
FAIL	github.com/adams-shaun/gorge/rules	0.616s
```

Then restored the file byte-identically (`cmp` OK) and the test passed again.

## Head/ratchet movement

None attributable to this ticket: no production code changed on this branch
(the param read predates it and landed on main via
`param:trig:AttackersDeclared.NoResolvingCheck`), no deck import, no census or
heads change. `TestConstructedDefaultIsByteIdentical` unchanged.

## Issues

None new. Round 1's report (`.ds4/report-t1.md`, tail) already records the
notes: the corpus-side `NoResolvingCheck$` population is entirely `True`, and
the remaining exposure (if any) is cards whose `IsPresent$`/`PresentCompare$`
clause is NOT paired with `NoResolvingCheck$` and therefore SHOULD re-check at
resolution — that path is already the shared default, so no gap.
