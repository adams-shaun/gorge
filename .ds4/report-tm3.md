# Report tm3 — Clone pump riders

Implemented in commit `b266861e4` (`effects/clone.go`, `rules/clone_pump_test.go`): Clone now reads `PumpKeywords$`, parses with `cards.SplitKeywordList`, and registers its grant separately from the clone's own copy effect. An explicit EOT/next-turn `PumpDuration$` is honored; absent duration follows the copy lifetime. Added tests for both independent lifetime shapes with checks that Haste is absent from the copied face beforehand and no Clone fallback/unread-note occurs.

Workspace: `.cards` was present.

## Gates and output

`go test -run 'TestClonePumpKeywordsRideTheCopyLifetime|TestClonePumpDurationEOtExpiresWhileThePermanentCopySurvives$' ./rules/`

```text
ok   github.com/adams-shaun/gorge/rules 0.078s
```

`go test ./internal/archtest/`

```text
ok   github.com/adams-shaun/gorge/internal/archtest (cached)
```

`go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/`

```text
ok   github.com/adams-shaun/gorge/cmd/botbench (cached)
```

## Fails without the fix

Temporarily removed the rider registration and restored the file byte-identically afterward (verified with `cmp`). The requested targeted test command failed at compilation because the now-unused duration values are part of that fix:

```text
# github.com/adams-shaun/gorge/effects
effects/clone.go:421:2: declared and not used: pumpDur
effects/clone.go:421:11: declared and not used: pumpEOT
effects/clone.go:421:20: declared and not used: pumpTurn
FAIL github.com/adams-shaun/gorge/rules [build failed]
FAIL
```

This establishes that removing the registration makes the test command fail, although the failure is a compiler failure rather than a behavioral assertion.

## Issues

No additional unfixed issues identified within this task. No `knownUnsupportedParams` entries were affected: neither real carrier is currently in a repo deck.
