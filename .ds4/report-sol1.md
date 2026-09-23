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
