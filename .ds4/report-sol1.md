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
