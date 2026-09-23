# Convoked$Amount — fix-round report

STATUS: DONE. Commit `88d159e4` (following initial implementation `9e650da1`). `.cards` was present as a symlink to the shared corpus; `/usr/bin/grep -rlE 'Convoked\$Amount' .cards/cardsfolder | wc -l` returned **2**.

## Changes

- `effects/count.go`: only the new Convoked head rejects unknown/invalid `/Op` suffixes for both bare and `Count$` forms; known operators still compose via the existing arithmetic. No changes to the behavior of other count heads.
- `effects/convoked_amount_test.go`: assert `(0,true)` for empty and absent provenance and `(0,false)` for unsupported operators (both spellings), rather than losing evaluability through `Num`.
- `rules/convoked_amount_test.go`: the real cast of Imperiosaur captures exactly two selected bears and excludes a third untapped battlefield bear, then ETB adds four counters. The real Knight ETB Dig is driven to its actual ask with a replay-visible library order and MV 0/2/4/5 fixture preconditions; only 0 and 2 are offered, and the MV-2 bear is selected and reaches hand. The prior post-resolution count assertion remains. This proves the engine consumer, rather than only a manually seeded SVar read.

The structural fix uses `Object.Convoked` already captured by the cast and the existing `/Op` evaluator; no cast-time gating or event changes. No count-head ratchet entry or Known-approximations row corresponded to this head; neither was changed. Botbench split did not move.

## Fails without the fix

Copied `effects/count.go` to `.ds4/scratch/count-fixed.go`, replaced it temporarily with `git show 9e650da1^:effects/count.go`, ran `go test -run 'TestConvokedAmount|TestAncientImperiosaurEntersWithTwoCountersPerConvoker|TestKnightErrantOfEosXCountsConvokers' ./effects ./rules`, then restored the exact copy (`cmp` exit 0). Exit 1:

```
--- FAIL: TestConvokedAmountReadsTheCorpusHeads (0.59s)
    convoked_amount_test.go:75: Num Amount$ X (Knight-Errant SVar) = 0, want 2
--- FAIL: TestConvokedAmountEmptyAndAbsentAreEvaluatedZero (0.00s)
    convoked_amount_test.go:111: empty provenance = (0,false), want (0,true)
FAIL github.com/adams-shaun/gorge/effects
--- FAIL: TestAncientImperiosaurEntersWithTwoCountersPerConvoker (0.57s)
    convoked_amount_test.go:135: Ancient Imperiosaur P1P1 counters = 0, want 4 (2 convokers x /Twice)
--- FAIL: TestKnightErrantOfEosXCountsConvokers (0.00s)
    convoked_amount_test.go:216: Knight Dig never offered a choice
FAIL github.com/adams-shaun/gorge/rules
```

Separately removed just the two operator-validation checks temporarily, ran `go test -run '^TestConvokedAmountEmptyAndAbsentAreEvaluatedZero$' ./effects` and restored the byte-identical copy (`cmp` exit 0). Exit 1:

```
--- FAIL: TestConvokedAmountEmptyAndAbsentAreEvaluatedZero (0.58s)
    convoked_amount_test.go:124: unknown operator "Convoked$Amount/Unmodelled" = (0,true), want unresolved
FAIL github.com/adams-shaun/gorge/effects
```

## Gates

`go test -run 'TestConvokedAmount|TestAncientImperiosaurEntersWithTwoCountersPerConvoker|TestKnightErrantOfEosXCountsConvokers|TestEveryRepoDeckCountHeadResolves' ./effects ./rules` (exit 0):

```
ok   github.com/adams-shaun/gorge/effects (cached)
ok   github.com/adams-shaun/gorge/rules 0.701s
```

`go test ./internal/archtest/` (exit 0):

```
ok   github.com/adams-shaun/gorge/internal/archtest 3.086s
```

`go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/` (exit 0):

```
ok   github.com/adams-shaun/gorge/cmd/botbench 1.191s
```

`gofmt -l effects/count.go effects/convoked_amount_test.go rules/convoked_amount_test.go` (exit 0): no output.
`go run ./cmd/gentypes -check` (exit 0): no output.
`git diff --check` (exit 0): no output.

## Issues

None found outside scope. No ledger entry closed; no CR lane test warranted for this card-specific count head.

---

Historical reports preserved verbatim below from the main lineage (Attached predicates, Deep Spawn UnlessCost Mill, RollDice, Gitaxian Probe verification, CopySpellAbility.Optional); they belong to separate tasks and are not findings of the Convoked$Amount ticket.
---

# Attached predicates — agent-20260922T210645Z-27e19c88

## Changes and review finding

The earlier commits `60976e28` (bare `Attached`, four context referents, Arna) and `53403389` (real Stangg trigger) implemented the brief; `82db540a` made plural bindings unbound. This fix-round commit `6b7f8116` closes the remaining MAJOR from `findings-sol1.md`: `effects/filter.go` now passes the game through `attachedToReferentObjects` and `contextPredicateBound`, rejecting any nonexistent object ID in a target or remembered binding before evaluating the positive OR its negation. The single-binding, literal/dotted, player-only, and plural paths remain unchanged. `effects/attachedto_stale_binding_test.go` is a new test file: it checks all four referents, missing IDs, mixed live/stale bindings, both polarities, grammar recognition, and battlefield/live-ID preconditions. This uses the shared resolver, so the next context-bound caller cannot forget the liveness check. `.cards` was already symlinked to `/home/sadams/projects/gorge/.cards`; corpus-backed runs did not skip. No Known-approximations row closed; no head golden or ratchet edited.

## Fails without the fix

New test was added before editing `filter.go`; this is the exact pre-fix run (`go test -run 'TestAttachedToStaleReferentFailsClosed' ./effects/`, exit 1):

```
--- FAIL: TestAttachedToStaleReferentFailsClosed (0.65s)
    attachedto_stale_binding_test.go:38: AttachedTo Targeted: stale binding returned (true, true), want (false, false)
    attachedto_stale_binding_test.go:41: Aura.AttachedTo Targeted: stale binding must match nothing
    attachedto_stale_binding_test.go:38: !AttachedTo Targeted: stale binding returned (false, true), want (false, false)
    attachedto_stale_binding_test.go:38: AttachedTo ParentTarget: stale binding returned (true, true), want (false, false)
    attachedto_stale_binding_test.go:41: Aura.AttachedTo ParentTarget: stale binding must match nothing
    attachedto_stale_binding_test.go:38: !AttachedTo ParentTarget: stale binding returned (false, true), want (false, false)
    attachedto_stale_binding_test.go:38: AttachedTo TriggeredCardLKICopy: stale binding returned (true, true), want (false, false)
    attachedto_stale_binding_test.go:41: Aura.AttachedTo TriggeredCardLKICopy: stale binding must match nothing
    attachedto_stale_binding_test.go:38: !AttachedTo TriggeredCardLKICopy: stale binding returned (false, true), want (false, false)
    attachedto_stale_binding_test.go:38: AttachedTo TriggeredAttackerLKICopy: stale binding returned (true, true), want (false, false)
    attachedto_stale_binding_test.go:41: Aura.AttachedTo TriggeredAttackerLKICopy: stale binding must match nothing
    attachedto_stale_binding_test.go:38: !AttachedTo TriggeredAttackerLKICopy: stale binding returned (false, true), want (false, false)
FAIL
FAIL github.com/adams-shaun/gorge/effects 0.662s
FAIL
```

Earlier fix-reverted evidence for bare Attached/context referents is in `.ds4/scratch/fails-effects.log` (TestAttachedPredicate / TestAttachedToContextReferents failed); real Arna and Stangg carrier failures are in `.ds4/scratch/fails-rules2.log` (both lacked a Bonesplitter token copy). The plural-binding test's pre-fix failures are documented in the earlier round's report. All files are corpus-backed; carrier tests assert the source is attached, the trigger resolves and the copied object differs from the original.

## Gates run (exact commands, actual output)

```
$ gofmt -l effects/filter.go effects/attachedto_stale_binding_test.go
$ go run ./cmd/gentypes -check
(exit 0, no output)
$ go test -run 'TestAttachedPredicate|TestAttachedToContextReferents|TestAttachedToReferentPluralBindingFailsClosed|TestAttachedToStaleReferentFailsClosed|TestAttachedToLiteralPredicate|TestAttachedToTargetedBoundFromContext|TestAttachedToPlayerWordStaysUnknown|TestAttachedToPredicateUnlocksCorpusTargeting|TestArnaRealSourceFilterReachesCopyRider|TestStanggRealTriggerCopiesAttachedPermanents' ./effects ./rules/
ok   github.com/adams-shaun/gorge/effects  0.747s
ok   github.com/adams-shaun/gorge/rules    0.704s
$ go test ./internal/archtest/
ok   github.com/adams-shaun/gorge/internal/archtest (cached)
$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok   github.com/adams-shaun/gorge/cmd/botbench 1.248s
```

## Issues

No new unresolved defect in this fix round. The pre-existing Silence the Believers plural-target limitation is fail-closed by design in `effects/filter.go:attachedToReferentObjects`: at 2+ object targets its `Aura.AttachedTo Targeted` rider does not apply; the earlier commit `82db540a` documents this remainder, rather than enlarging the frozen Known-approximations register. No head/ratchet movement measured; the daemon owns full game/acceptance gates.

---

Historical Gitaxian Probe report preserved verbatim below; it belongs to a separate task and is not a finding of this round.

---

# Deep Spawn UnlessCost Mill — sol1 review response

## Finding resolved

Removed `.ds4/report-r2-split-cast.md`, an unrelated split-cast report
mistakenly introduced by the merge-resolution report commit. Updated
`.ds4/report-r2.md` to remove references to it. The Deep Spawn implementation
is unchanged: `rules/mana.go` strictly accepts fixed `Mill<N>`,
`rules/stack.go` settles it through `payMillCost` after all checks,
`rules/deep_spawn_mill_unless_test.go` exercises the real corpus upkeep and
short/empty libraries, and `rules/unless_unpriceable_test.go` removes Deep
Spawn from the bidirectional strict-unpriceable census. The full initial
implementation report, including its proof and verification, is
`.ds4/report-t1-mill-unless.md`. `.cards` is present as a symlink to the real
corpus; the initial real-card test ran, not skipped. No Known-approximations
row was involved, no golden/ratchet was changed, and no engine change was
made in this review round.

## Gates (this review round; exact commands and output)

```
$ go test -run 'TestDeepSpawnUnlessMillCost|TestUnlessCostStrictParsePopulation' ./rules/ > .ds4/scratch/sol1-deep-rules.log 2>&1; tail -30 .ds4/scratch/sol1-deep-rules.log
ok  github.com/adams-shaun/gorge/rules (cached)
$ go test ./internal/archtest/ > .ds4/scratch/sol1-deep-arch.log 2>&1; tail -30 .ds4/scratch/sol1-deep-arch.log
ok  github.com/adams-shaun/gorge/internal/archtest (cached)
$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/ > .ds4/scratch/sol1-deep-botbench.log 2>&1; tail -30 .ds4/scratch/sol1-deep-botbench.log
ok  github.com/adams-shaun/gorge/cmd/botbench (cached)
$ gofmt -l rules/mana.go rules/stack.go rules/unless_unpriceable_test.go rules/deep_spawn_mill_unless_test.go
[no output]
$ go run ./cmd/gentypes -check
[no output; exit 0]
```

## Fails without the fix

Initial round restored `rules/mana.go` and `rules/stack.go` byte-identically
after temporarily reverting their Mill additions. Its exact targeted test
failure (full proof in `.ds4/report-t1-mill-unless.md`):

```
--- FAIL: TestParseUnlessCostMill (0.00s)
    deep_spawn_mill_unless_test.go:25: ParseUnlessCost(Mill<2>) = !ok, want the fixed mill cost accepted
--- FAIL: TestDeepSpawnUnlessMillCost (0.62s)
    deep_spawn_mill_unless_test.go:130: expected a 2-option Pay/Don't pay ask, got [{Index:0 Kind:mode Label:Don't pay Obj:81 ...}]
--- FAIL: TestDeepSpawnUnlessMillCostShortLibrary (0.00s)
    deep_spawn_mill_unless_test.go:165: short library exposed 1 options, want the full Pay/Don't pay pair
--- FAIL: TestDeepSpawnUnlessMillCostEmptyLibrary (0.00s)
    deep_spawn_mill_unless_test.go:187: empty library exposed 1 options, want the full Pay/Don't pay pair
FAIL
FAIL    github.com/adams-shaun/gorge/rules    0.643s
FAIL
```

## Issues

No additional unresolved engine defect found in this round.

---

# RollDice cost implementation report — agent-20260922T120916Z-0a3043f3

## Review finding resolved

The previous integration overwrote the tracked `.ds4/report-t1.md` from the unrelated `playerspec-life-svar-threshold` ticket. Restored that report byte-for-byte from `main` (verified with `cmp`); this RollDice report is now stored separately in `.ds4/report-sol1.md`. No engine code changed in this review round. The new report path is force-staged so it remains durable across the merge.

## Changes

- `rules/mana.go`: added a `Cost.RollDice` part and narrow `RollDice<N/Sides/XVar>` parser. The supported `X` form is recorded as `N`, `Spec` (sides), and `Dyn`; malformed/unsupported instances degrade to one generic mana and `Unknown: [RollDice]`.
- `rules/cast.go`: ability payment now uses the seeded engine RNG for each cost die, emits the canonical `effects.DieRollNote`, and binds the last result to `pc.x` before the ability's existing pay-time `CastInfo`.
- `rules/rolldice_cost_test.go`: added parser/malformed-input checks and a real Clay Golem end-to-end pin. It confirms the object starts as an unmarked battlefield permanent, resolves a d8 cost roll, gains the matching counters and Monstrous mark, and pushes its BecomeMonstrous trigger. The test checks the canonical per-die Note directly as the cost-side `RolledDie`/`RolledDieOnce` trigger encoding rather than staging a second carrier.
- `rules/monstrosity_test.go`: removed the now-false zero-amount Clay Golem pin; its replacement is the end-to-end test above.

`.cards` was present in this worktree. Re-measured corpus prevalence: `/usr/bin/grep -rlE 'Cost\\$[^|]*RollDice' .cards/cardsfolder` found exactly one card, `Clay Golem`.

## Fails without the fix

Saved `rules/mana.go`, temporarily removed only its RollDice parser branch (leaving the field/payment code intact), ran the Clay Golem test, then restored and `cmp`-verified the file. Output:

```text
without_fix_exit=1 restore_cmp=0
--- FAIL: TestClayGolemRollDiceCostAndMonstrosity (0.00s)
    rolldice_cost_test.go:15: RollDice cost parse = {Colored:[0 0 0 0 0 0] Generic:7 Life:0 X:0 Hybrid:[] Phyrexian:[] Twobrid:[] HybridPhyrexian:[] Snow:0 Tap:false Sac:[] Discard:[] SubCounter:[] AddCounter:[] Exile:[] Reveal:[] RevealChosen:[] Behold:[] TapPermanent:[] Blight:[] Forage:false Draw:[] Energy:[] LifeX:[] LifeHalfUp:false DamageYou:[] Return:[] PutToLib:[] MoveToGrave:[] Mill:[] Evidence:[] RollDice:[] Unknown:[RollDice]}, want {6} and one modelled roll
FAIL
FAIL    github.com/adams-shaun/gorge/rules    0.002s
FAIL
```

## Verification

Targeted suite (only the brief's permitted targeted command):

```text
$ go test -run 'TestClayGolem|TestMonstrosity|TestRolledDie' ./rules/
ok  github.com/adams-shaun/gorge/rules  0.645s
```

Required gates:

```text
$ go test ./internal/archtest/
ok  github.com/adams-shaun/gorge/internal/archtest  3.660s

$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok  github.com/adams-shaun/gorge/cmd/botbench  1.366s

$ gofmt -l rules/mana.go rules/cast.go rules/rolldice_cost_test.go rules/monstrosity_test.go
[no output]
$ go run ./cmd/gentypes -check
[no output]
```

## Verification on merged HEAD `32029f5c` after report restoration

```text
$ go test -run 'TestClayGolem|TestMonstrosity|TestRolledDie' ./rules/ > .ds4/scratch/sol1-rules.log 2>&1; tail -30 .ds4/scratch/sol1-rules.log
ok  github.com/adams-shaun/gorge/rules (cached)
$ go test ./internal/archtest/ > .ds4/scratch/sol1-arch.log 2>&1; tail -15 .ds4/scratch/sol1-arch.log
ok  github.com/adams-shaun/gorge/internal/archtest (cached)
$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/ > .ds4/scratch/sol1-bot.log 2>&1; tail -5 .ds4/scratch/sol1-bot.log
ok  github.com/adams-shaun/gorge/cmd/botbench (cached)
$ gofmt -l rules/mana.go rules/cast.go rules/monstrosity_test.go rules/rolldice_cost_test.go
[no output]
$ go run ./cmd/gentypes -check
[no output; exit 0]
$ cmp .ds4/report-t1.md <(git show main:.ds4/report-t1.md)
[no output; exit 0]
```

The tests above were redirected to `.ds4/scratch/sol1-{rules,arch,bot}.log` and tailed once, with exit status 0 for each. Prior uncached gate output and the parser-revert proof are retained above from round 1. `.cards` is present as a symlink; no corpus-dependent tests skipped. No known-approximations row names RollDice, so no row or constant changed. No heads/ratchet movement observed; no full suite was run in this seat.

## Issues

No additional defects found or left open in the requested cost-token scope. The cost-side roll uses the existing canonical roll Note, so existing `RolledDie` and `RolledDieOnce` matching consumes it without new trigger registration. `XVar` other than `X` intentionally degrades loudly as specified by the brief; supporting arbitrary XVar names would require additional grammar and binding work.

---

# Second report stored at this shared path (from main): Gitaxian Probe verification — fb-20260923T015847Z-fad49275

# Gitaxian Probe verification — fb-20260923T015847Z-fad49275

## Conclusion

**Original reported match not reproduced.** No feedback snapshot was supplied, so its exact state and failure point cannot be established. Current real-corpus Probe tests pass and the already-landed fix `868d7c6b` makes `RevealHand` without `NumCards$` reveal the whole hand (`c7f54854` merged the earlier report). No new behavior or test is warranted.

## Prior finding resolution and changes

- **MAJOR (destructive report overwrite):** Restored `.ds4/report-t1.md` byte-for-byte from the parent of `435ea8a2`, preserving the unrelated restricted-mana and count-head records (verified with `git show HEAD^:.ds4/report-t1.md | cmp - .ds4/report-t1.md`). This ticket's report is instead `.ds4/report-sol1.md`, its designated round-specific path. No other historical report was edited.
- **MINOR (false clean-tree claim):** The overwritten report's claim of a clean working tree was incorrect. The correction is this committed restore and separate report; before this round's edit the working tree was clean *because the overwrite had already been committed*, not because no change was made.
- No production or test Go files changed. `.cards` was present as a symlink to `/home/sadams/projects/gorge/.cards`, with `ir.gob.gz` available; corpus tests did not vacuously skip.

## Path inspection

`effects/cardflow.go:2072` has `wholeHand := sa.API == "RevealHand" && !hasNum`; `effects/revealhand_test.go` verifies the actual compiled Probe SA has `Look$ True` and no `NumCards$`, and that after the look acknowledgment its secret Note carries all target hand IDs, scoped to the activator. `view/look_redaction_test.go` drives the real card through the engine and checks all IDs, redaction across seats/spectators, transcript names, and replay. `host/fanout.go:eventBodiesFor` calls `view.RedactEventFor` then `view.Describe` on the redacted event; `host/viewat.go` uses the same function for Events/EventsSeat. `web/src/components/Transcript.svelte` takes `e.line`, filters via `visibleLog`, then renders `parseLogLine(e.line)`; `web/src/lib/logfilter.ts` does not hide `note` events. No distinct, reproducible downstream omission was found. This is code-path verification, not a replay of the player's missing snapshot or a live-demo test.

## Gates (exact commands and output)

```
$ go test -run 'TestGitaxianProbeLookIsAPrivateLookScopedToTheActivator|TestThoughtKnotSeerRevealHandRevealsTheWholeHand' ./effects/ 2>&1 | tail -30
ok  	github.com/adams-shaun/gorge/effects	(cached)
$ go test -run 'TestGitaxianProbeLookStaysPrivateFromEveryOtherViewer|TestGitaxianProbeLookDescribeLines|TestGitaxianProbeGameReplaysAndDescribesIdentically' ./view/ 2>&1 | tail -30
ok  	github.com/adams-shaun/gorge/view	(cached)
$ go test ./internal/archtest/ 2>&1 | tail -15
ok  	github.com/adams-shaun/gorge/internal/archtest	(cached)
$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/ 2>&1 | tail -15
ok  	github.com/adams-shaun/gorge/cmd/botbench	(cached)
```

No new tests: `## Fails without the fix` and new-test preconditions are inapplicable. No Go files changed: `gofmt -l <changed Go files>` and `go run ./cmd/gentypes -check` are not required. No head, ratchet or Known-approximations row changed.

## Issues

No new defect identified. Without the missing feedback capture the original live game's point of failure remains unverifiable; do not infer that the reported historical symptom did not occur.

---

# CopySpellAbility.Optional — sol1 rebase and report-conflict resolution

## Finding resolved

The MAJOR in `.ds4/findings-sol1.md` was a destructive overwrite of an unrelated ticket's `.ds4/report-t1.md`. Rebased onto `main` as directed and resolved both report conflicts by retaining **main's exact versions** of `.ds4/report-t1.md` and `.ds4/report-r2.md`; `cmp` verified both byte-for-byte against `main`. The CopySpellAbility reports from earlier rounds are preserved in the branch's historical commits; this ticket's current report is this section of `.ds4/report-sol1.md`, appended without replacing the other tickets' existing content at that path. No Go changes were needed in this review round. The prior findings (no-host AskOutcome and Optional+UnlessCost composition) were already fixed in `8054eb71` (rebased SHA), as confirmed by the reviewer and the focused tests below. No other MAJOR was raised.

## Implementation already committed

- `effects/copy.go`, `effects/registry.go`: Optional$ True asks the copy controller for a yes/no through `copy_optional`, pauses only on `AskAsked`, resumes with the chosen answer, and composes AFTER the shared UnlessCost gate. A no-host ask preserves the historical copy.
- `rules/resolution.go`: returns the election answer through the existing mid-resolution resume machinery.
- `rules/copy_spell_ability_optional_test.go`, `rules/copy_optional_unless_test.go`, `effects/copy_optional_no_host_test.go`, `effects/copy_test.go`: real-corpus Sevinne accept/decline, no-host fallback, switched and unswitched UnlessCost composition. `rules/was_cast_from_zone_test.go` answers Sevinne's newly-real election; `effects/context_test.go` updates the test double's comment.
- `.cards` is present as a symlink to the real corpus. Prior measurement found 11 corpus files with CopySpellAbility and Optional$; no known-approximations row, chain-head golden, or acceptance ratchet was changed.

## Gates on rebased branch (exact commands and real output)

```
$ go test -run 'TestCopySpellAbilityOptional|TestCopySpellAbilityUnswitchedShapePayingStopsTheCopies|TestOptionalUnless|TestSevinnesReclamationMayCopyElection' ./rules/ ./effects/ > .ds4/scratch/copy-sol1-focused.log 2>&1; rc=$?; tail -30 .ds4/scratch/copy-sol1-focused.log; echo focused_exit=$rc
ok   github.com/adams-shaun/gorge/rules 0.663s
ok   github.com/adams-shaun/gorge/effects 0.624s
focused_exit=0

$ go test ./internal/archtest/ > .ds4/scratch/copy-sol1-arch.log 2>&1; rc=$?; tail -15 .ds4/scratch/copy-sol1-arch.log; echo arch_exit=$rc
ok   github.com/adams-shaun/gorge/internal/archtest 3.538s
arch_exit=0

$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/ > .ds4/scratch/copy-sol1-bot.log 2>&1; rc=$?; tail -5 .ds4/scratch/copy-sol1-bot.log; echo bot_exit=$rc
ok   github.com/adams-shaun/gorge/cmd/botbench 1.391s
bot_exit=0

$ gofmt -l effects/copy.go effects/copy_optional_no_host_test.go effects/context_test.go effects/copy_test.go effects/registry.go rules/copy_optional_unless_test.go rules/copy_spell_ability_optional_test.go rules/resolution.go rules/was_cast_from_zone_test.go
(no output)
$ go run ./cmd/gentypes -check; echo gentypes_exit=$?
gentypes_exit=0
$ cmp .ds4/report-t1.md <(git show main:.ds4/report-t1.md) && echo report_t1_matches_main
report_t1_matches_main
$ cmp .ds4/report-r2.md <(git show main:.ds4/report-r2.md) && echo report_r2_matches_main
report_r2_matches_main
```

## Fails without the fix

No new tests or Go edits in this round. Previous rounds proved the tests fail with the corresponding production hunks removed and restored the files byte-identically. Recorded failing output (from the original round-1/round-2 reports):

```
--- FAIL: TestSevinnesReclamationMayCopyElectionDeclineMakesNoCopy (0.59s)
    copy_spell_ability_optional_test.go:178: expected the Optional$ True may-copy election (KChoose copy_optional), got ... Kind:target ... ResumeKind:copy_targets ...
--- FAIL: TestCopySpellAbilityOptionalNoHostMakesTheCopy (0.00s)
    copy_optional_no_host_test.go:33: 0 StackCopy events on a no-host election, want 1 (the R-9 stand-in makes the copy)
--- FAIL: TestOptionalUnlessCopyDeclinePosesElectionToCopyController (0.00s)
    copy_optional_unless_test.go:163: expected the may-copy election (KChoose copy_optional) after the declined gate, got ... priority ...
```

## Issues

Existing, separate issue: a copied Sevinne's Reclamation inherits graveyard-cast flags in `events/apply.go`'s `StackCopy`, so `effects/filter.go`'s `wasCastFromGraveyard` can admit the copy's conditional copy clause even though the copy was not cast (CR 707.10). Earlier measurement: 29 corpus files mention `wasCastFromGraveyard`, six use `ConditionPresent$ Card.wasCastFromGraveyard`. Already filed in `.ds4/new-tickets/copy-inherits-cast-provenance.md`; not expanded in this ticket. No new defect found during the report-conflict fix.

---

# Second report stored at this shared path (from branch): Cascade free cast — CR 702.85a / CR 107.3b

# Cascade free cast — CR 702.85a / CR 107.3b

## Outcome

The brief's premise is not a legal Magic play: when casting Villainous Wealth *without paying its mana cost*, CR 107.3b fixes its mana-cost X at **0**, not an announced 1. The library scan compares its printed MV 3 with Bloodbraid Elf's on-stack MV 4; the free cast remains MV 3. There is no legal at-or-above-4 free-cast X choice to reject. An earlier attempt on this branch added such an X choice (`a1b718ea`), but review identified the CR violation; `f0ae814e` reverted that code and `cfc32dfd` pinned the legal behavior in `rules/cascade_resulting_mv_test.go`. No new cast-flow restriction is warranted. The source's own announced X remains covered by `TestCascadeXSpellUsesAnnouncedManaValue`.

This round strengthened `rules/cascade_resulting_mv_test.go`: asserts the library card *actually has an X mana-cost symbol*, its printed 3 is exactly one below the source's 4, and both cards are on the stack after the free cast with candidate X=0 and resulting MV strictly below source MV. The pre-existing test in that file checks that no X announcement occurs at any stage of the free cast, that it resolves and is never bottomed, and that replay agrees. I removed its redundant non-X companion (it passed even with the illegal-X change reverted, so it could not serve as a regression for this defect). The card scripts were read from the existing `.cards` symlink, not committed. No Known-approximations row was changed; the existing-order bottom stand-in is unchanged.

The required brief test name `TestCascadeFreeCastXMustRemainBelowCascadeManaValue` is intentionally replaced by `TestCascadeFreeCastAnnouncesNoX`: an X=1 reject test would enforce a choice that the rules do not permit. This is also why there is no separate legal X>=1 boundary test.

## Fails without the fix

Proof: copied `effects/cascade.go`, `rules/cast.go`, `rules/resolution.go`, and `rules/play_cost_test.go` to `.ds4/scratch/cascade-sol1/`; temporarily reinstated the prior illegal-X implementation from `a1b718ea`, ran the legal-case test, restored the four files from their copies and verified `cmp` on every file. Real command output:

```text
$ go test -run 'TestCascadeFreeCastAnnouncesNoX$' ./rules/
--- FAIL: TestCascadeFreeCastAnnouncesNoX (0.58s)
    cascade_resulting_mv_test.go:85: after the election want CR 601.2c's target ask, got &{Seq:78 Player:0 Kind:choose Prompt:Choose a value for X Min:1 Max:1 Options:[{Index:0 Kind:x Label:X = 0 Obj:0 Counter: Player:0 Attacker:0 Battle:0 Required:false BlockMust:false MinBlockers:0 MaxBlockers:0 Controller:0 Group: AltCostIndex:0 CostLife:0 CostTaps:0 Mode: Amount:0 Ability:0 SVar: Keyword: Cost: Grant:<nil> GrantSource:0 GainedSource:0 GainedIdx:0 Value:0} {Index:1 Kind:x Label:X = 1 Obj:0 Counter: Player:0 Attacker:0 Battle:0 Required:false BlockMust:false MinBlockers:0 MaxBlockers:0 Controller:0 Group: AltCostIndex:0 CostLife:0 CostTaps:0 Mode: Amount:1 Ability:0 SVar: Keyword: Cost: Grant:<nil> GrantSource:0 GainedSource:0 GainedIdx:0 Value:0} {Index:2 Kind:x Label:X = 2 Obj:0 Counter: Player:0 Attacker:0 Battle:0 Required:false BlockMust:false MinBlockers:0 MaxBlockers:0 Controller:0 Group: AltCostIndex:0 CostLife:0 CostTaps:0 Mode: Amount:2 Ability:0 SVar: Keyword: Cost: Grant:<nil> GrantSource:0 GainedSource:0 GainedIdx:0 Value:0} {Index:3 Kind:x Label:X = 3 Obj:0 Counter: Player:0 Attacker:0 Battle:0 Required:false BlockMust:false MinBlockers:0 MaxBlockers:0 Controller:0 Group: AltCostIndex:0 CostLife:0 CostTaps:0 Mode: Amount:3 Ability:0 SVar: Keyword: Cost: Grant:<nil> GrantSource:0 GainedSource:0 GainedIdx:0 Value:0} {Index:4 Kind:x Label:X = 4 Obj:0 Counter: Player:0 Attacker:0 Battle:0 Required:false BlockMust:false MinBlockers:0 MaxBlockers:0 Controller:0 Group: AltCostIndex:0 CostLife:0 CostTaps:0 Mode: Amount:4 Ability:0 SVar: Keyword: Cost: Grant:<nil> GrantSource:0 GainedSource:0 GainedIdx:0 Value:0}] MaxSum:0 Budgeted:false GroupLimit:0 Repeatable:false Source:3 TargetsWithSameController:false TargetEffect:<nil> Restable:false ResumeKind: ResumeSA:<nil> ResumeModes:[] ResumeTarget:0 Rolls:[] ResumeChoices:[] ResumeChosenValid:false ResumeRemembered:[] ResumeDigUntilMove: ResumeDigUntilMoveDone:false ResumeTargetsUnique:[] ResumeMoved:[] ResumeDigPrimary:[] ResumeObjects:[] ResumeRound:0 ResumeRepeatNext:0 ResumeUptoIdx:0 ResumeUptoCount:0 ResumeVillainousVictims:[] ResumeVillainousIndex:0}
FAIL
FAIL	github.com/adams-shaun/gorge/rules	0.599s
FAIL
reverted-fix exit=1; restored files byte-identical
```

The full unabridged failure is `.ds4/scratch/cascade-sol1/fails.log` (git-excluded). X=1 would make the candidate MV 4, equal to the source; the faulty version wrongly offered that choice at all. This is the only new test retained; it fails with the illegal-X behavior restored.

## Gates (real output)

```text
$ go test -run 'TestCascadeFreeCastAnnouncesNoX|TestCascadeXSpellUsesAnnouncedManaValue' ./rules/
ok   github.com/adams-shaun/gorge/rules 0.721s

$ go test ./internal/archtest/
ok   github.com/adams-shaun/gorge/internal/archtest 1.570s

$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok   github.com/adams-shaun/gorge/cmd/botbench (cached)

$ gofmt -l rules/cascade_resulting_mv_test.go
(no output)
$ go run ./cmd/gentypes -check
(no output; exit 0)
$ git diff --check
(no output)
```

Botbench golden unchanged (cached result valid for this test-only amendment); no split re-pin or attribution needed. The `.cards` symlink was present and the real-corpus test executed, not skipped. The prior round's proof and gate output are in `.ds4/scratch/cascade-sol1/` and the earlier committed test; the branch history records the reviewed revert explicitly.

## Merge hygiene

Before this round, tracked `.ds4/report-r2.md` and `.ds4/report-t1.md` had uncommitted *unrelated report overwrites*, blocking the controller's rebase/merge (`findings-sol1.md`). Preserved both to `.ds4/scratch/cascade-sol1/report-*-uncommitted.md`, then restored their tracked HEAD content byte-for-byte; neither was staged. Did not rebase, switch branches, or touch shared git settings. Only this task's test and report are being committed.

## Issues

No new out-of-scope defect verified in this round. The brief's X=1 counterexample is ruled out by CR 107.3b, not an outstanding implementation bug. The existing-order cascade bottoming approximation is unchanged. A prior report notes a possible `rules/cast.go:recheckIllegal` mismatch for an X in an *additional cost* on a free cast, but reachability and card prevalence have not been established; no general-X changes are attempted here.

## Commits

`f0ae814e` (review-requested revert of illegal free-cast X choice), `cfc32dfd` (new legal-path regression), `6bcfe1fc` (strict-MV/asserted-X preconditions).
