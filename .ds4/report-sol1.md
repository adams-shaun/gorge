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
