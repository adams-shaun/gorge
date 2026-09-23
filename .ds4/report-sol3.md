# Discard approximation — rebase and verification (sol3)

## Changes and integration

The controller ordered a rebase before continuing. Staged only `.ds4/report-mrg1.md` (already tracked, though under an ignored directory), committed it as `f865930d`, then ran `git rebase main`. Resolved the conflict in `internal/testutil/agentsdoc_test.go`: main had 73 rows and this ticket deletes Discard, so the ratchet is now **72**. The rebased commit initially reintroduced main's already-closed CopySpellAbility row in `AGENTS.md`; removed that row while retaining the Discard deletion. Measured 72 table rows. At the later `.ds4/report-mrg1.md` conflict, preserved both historical report sections. Rebase completed, working tree clean before this report. Removed the trailing blank line introduced in the merged historical report as a follow-up formatting change.

The rebased commits implement: `effects/cardflow.go` and `effects/registry.go` — seeded random discard with the chosen index and object in the applied move, chooser-specific private asks, DefinedCards resolution, optional whole-hand election and target-index continuation; `rules/resolution.go` — resume routing; `events/actions.go` — move payload documentation; `rules/cr704_state_based_conformance_test.go` — wait until the Smallpox resolution is complete before asserting deferred SBA. Tests in `effects/cardflow_discard_modes_test.go`, `effects/cardflow_discard_hand_test.go`, `rules/discard_multi_target_test.go`, and `rules/discard_random_replay_test.go` cover the changes. The original row is deleted, with no new approximation row. The corpus is present (`.cards` symlink, `cardsfolder`, IR cache).

Prior review MAJORs: the Smallpox CR 704 test now passes at the actual completion boundary (including every seat's discard); the random discard's `Host.Rand` is the engine's seeded RNG, and its pick index plus Obj are logged in the canonical event folded by `events.Apply`. The corpus Burning Inquiry replay test verifies both event-only state reconstruction and a repeatable draw sequence.

## Fails without the fix

Copied `effects/cardflow.go` to `.ds4/scratch/cardflow-sol3-fixed.go`; temporarily removed the random-choice `ev.Amount = int32(j + 1)` assignment; ran the focused test; restored the file with `cp` and verified `cmp` was identical (`restore_cmp=0`):

```
without=1 restore_cmp=0
--- FAIL: TestBurningInquiryRandomDiscardIsLoggedAndReplayable (0.58s)
    discard_random_replay_test.go:44: random choice index does not identify the applied discard: {Seq:43 Kind:move_zone Player:0 Obj:57 From:hand To:graveyard Amount:0 Step:untap Counter: Text:discarded IDs:[] Pairs:[] Secret:false} hand=[27 38 18 31 57 55 42 61]
FAIL
FAIL github.com/adams-shaun/gorge/rules 0.597s
```

The preceding round's `.ds4/report-sol2.md` also records a failing pre-fix run and a byte-identical restore; this round repeated the proof against the rebased code.

## Gates (actual output)

```
$ go test -run 'TestDiscard|TestBurningInquiryRandomDiscardIsLoggedAndReplayable|TestCR704NoLifeSBAInsideSmallpoxDiscard' ./effects/ ./rules/
ok   github.com/adams-shaun/gorge/effects 0.683s
ok   github.com/adams-shaun/gorge/rules 0.635s
$ go test -run 'TestKnownApproximationsOnlyShrinks|TestKnownApproximationRowsAreShort' ./internal/testutil/
ok   github.com/adams-shaun/gorge/internal/testutil 0.003s
$ go test ./internal/archtest/
ok   github.com/adams-shaun/gorge/internal/archtest 2.944s
$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok   github.com/adams-shaun/gorge/cmd/botbench 1.079s
$ go run ./cmd/gentypes -check
(exit 0, no output)
$ gofmt -l effects/cardflow.go effects/registry.go events/actions.go rules/resolution.go rules/cr704_state_based_conformance_test.go rules/discard_multi_target_test.go rules/discard_random_replay_test.go effects/cardflow_discard_modes_test.go internal/testutil/agentsdoc_test.go
(no output)
```

No TestHeads or botbench re-pin was needed from the targeted evidence; broad goldens belong to the merge gate. No new ratchet entries closed beyond the approximation row.

## Issues

- `UnlessCost$` / `UnlessPayer$` remain unread in `effects/cardflow.go` (`effDiscard`): 26 actual `$ Discard |` lines in 25 corpus files contain either rider; the alternative payment is not offered. Completing it requires an engine-backed payment continuation in `effects/unless.go` / `rules/unless_payment.go`. This deviation is in the feature commit message and filed as `.ds4/new-tickets/discard-unless-payment.md`, not a replacement approximation row. A CR 701.8 corpus test should pin the payment alternative. Other named families: Condition* uses the common resolution gate; `AILogic$` is an AI-only hint; Mandatory and TargetMin/Max/Unique are handled by mandatory resolution and cast-time targeting, respectively.
