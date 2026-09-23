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

# Task fb-20260923T020152Z — attacker radial picker

## Summary

Committed the board UI fix in `a1d21db9` (`fix(web): keep the attacker picker open across selections`). The picker identifies the narrow `attacker` wire-option kind as a selection, keeps it open after that pick, and ignores bubbled clicks originating inside the body-ported picker. The explicit outside-click and Escape dismissals remain intact; ordinary action choices still close explicitly.

## Changes

- `web/src/lib/cardoptions.ts` — added `isSelectionOption`, based only on the option's wire kind (`attacker`). This keeps the behavior narrow and puts the classification in one helper.
- `web/src/components/OptionPicker.svelte` — closes after ordinary choices but not attacker selections; the window click handler ignores clicks within `[data-option-picker]`, accounting for the portaled wheel's bubbling clicks while preserving outside-click dismissal.
- `web/src/components/CardMenu.fixture.html` and `web/src/components/CardMenu.fixture.ts` — added a real mounted attacker tile with three distinct wire indices and a recording post callback.
- `web/src/components/CardMenu.test.ts` — added mounted checks for three successive attacker picks without reopening, plus outside-click and Escape dismissal. The existing ordinary-action mounted test still exercises close/reopen behavior.

Structural approach: classify by the existing wire-option role, not by labels, object identity, or inferred combat state. A later option using the same attacker wire kind follows the same path; unrelated multi-pick kinds are deliberately not widened.

## Fails without the fix

Saved the committed fixed `OptionPicker.svelte`, restored the pre-fix file from `HEAD^`, and ran the mounted suite. The new regression failed because the radial was detached immediately after the first attacker click. Restored the fixed source and verified it byte-identically against the saved copy (`cmp` passed).

Command: `cd web && npm_config_cache=/tmp/gorge-fb-npm-cache npx vitest run src/components/CardMenu.test.ts`

```text
 RUN  v5.0.0 /home/sadams/projects/gorge/.worktrees/fb-20260923T020152Z-694613d1/web

9:06:30 AM [vite-plugin-svelte] src/components/ResolvedCard.svelte:97:41 This reference only captures the initial value of `anchorProp`. Did you mean to reference it inside a derived instead?
https://svelte.dev/e/state_referenced_locally
 ❯ src/components/CardMenu.test.ts (6 tests | 1 failed) 992ms
   ❯ declaring attackers keeps the radial picker open (fb-20260923T020152Z) (3)
     × a click on an attacker option posts its wire index and leaves the radial attached for the next attacker 138ms

⎯⎯⎯⎯⎯⎯⎯ Failed Tests 1 ⎯⎯⎯⎯⎯⎯⎯

 FAIL  src/components/CardMenu.test.ts > declaring attackers keeps the radial picker open (fb-20260923T020152Z) > a click on an attacker option posts its wire index and leaves the radial attached for the next attacker
AssertionError: expected +0 to be 1 // Object.is equality

- Expected
+ Received

- 1
+ 0

 ❯ src/components/CardMenu.test.ts:115:33
    113|     // anchor after a successful attacker pick (no badge re-open), and…
    114|     // next attacker posts its own wire index (R-E4-1, never list posi…
    115|     expect(await wheel.count()).toBe(1);
       |                                 ^
    116|     await wheel.locator('button[data-wire-index="41"]').waitFor();
    117|     await wheel.locator('button[data-wire-index="41"]').click();

⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯[1/1]⎯

 Test Files  1 failed (1)
      Tests  1 failed | 5 passed (6)
   Start at  09:06:30
   Duration  2.18s (tests 82%, import 17%, transform 1%)
```

The tests assert their setup: each dismissal/selection case waits for the expected wire-index button and checks that the picker is attached before interaction; the multi-selection case checks the wheel remains attached after each click and checks the exact posted index sequence.

## Gates run

Corpus and web test dependencies were present (`.cards` exists; `web/node_modules/.bin/vitest` exists). The first bare `npx` attempt could not write npm's shared cache (`EROFS`); setting a worktree-safe cache resolved it. Successful targeted run:

`cd web && npm_config_cache=/tmp/gorge-fb-npm-cache npx vitest run src/components/CardMenu.test.ts`

```text
 RUN  v5.0.0 /home/sadams/projects/gorge/.worktrees/fb-20260923T020152Z-694613d1/web

9:06:13 AM [vite-plugin-svelte] src/components/ResolvedCard.svelte:97:41 This reference only captures the initial value of `anchorProp`. Did you mean to reference it inside a derived instead?
https://svelte.dev/e/state_referenced_locally

 Test Files  1 passed (1)
      Tests  6 passed (6)
   Start at  09:06:12
   Duration  2.26s (tests 83%, import 16%, transform 1%)
```

`go test ./internal/archtest/`

```text
ok  	github.com/adams-shaun/gorge/internal/archtest	(cached)
```

`go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/`

```text
ok  	github.com/adams-shaun/gorge/cmd/botbench	(cached)
```

## Issues

No additional unfixed defects found. This UI change closes no Known-approximations row and changes no engine behavior; no ledger entry applies. `cmd/botbench` remained byte-identical (test passed).

## Review-round cleanup

The first integration attempt was blocked by unstaged `.ds4/report-t1.md` and
`.ds4/report-t2.md` rewrites belonging to other tracked reports. Both were
preserved under `.ds4/scratch/fb-report-{t1,t2}-preserved.md` (ignored), then
restored byte-for-byte from this branch's HEAD. This task's report is appended
to the designated `.ds4/report-sol1.md` rather than overwriting or committing
changes to historical report-t1/report-t2.

The original implementation and targeted gates were run before the code
commit `a1d21db9`. No implementation changes were needed this round.
