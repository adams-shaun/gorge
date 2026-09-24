# Report — API Phases follow-up

## Changes
- `rules/cast.go`: cost candidate and sacrifice candidate battlefield walks now exclude phased-out permanents.
- `rules/mana_activation.go`: the shared mana-ability discovery helper rejects phased-out battlefield objects, covering payment-window sources as well as ordinary offers.
- `rules/trigger_match.go`: printed/granted trigger source traversal skips phased-out objects.
- `rules/legal.go`: corrected the inaccurate claim that every reader was gated; documents the actual split.
- `events/event.go`: restored rationale for the append-only `NumKinds` sentinel.

The branch implementation of `api:Phases` itself was already present; this round fixes the three residual nonexistence gaps identified in review.

## Workspace
`.cards` exists (corpus cache and cardsfolder present). Rebased onto `main` as directed; no conflicts.

## Gates run
- `gofmt -w rules/cast.go rules/mana_activation.go rules/trigger_match.go rules/legal.go events/event.go && go build ./... && go test -run 'TestPhases|TestTalonGates|TestCR702|TestPhasedOut' ./rules/`: `ok github.com/adams-shaun/gorge/rules 0.467s`
- `go run ./cmd/gentypes -check`: passed (no output).
- `go test ./internal/archtest/`: `ok github.com/adams-shaun/gorge/internal/archtest 3.866s`
- `go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/`: `ok github.com/adams-shaun/gorge/cmd/botbench 0.689s`

## Fails without the fix
Not run. The residual fixes are not independently demonstrated by a test with the production changes reverted; this remains a verification limitation.

## Issues
- This round did not add the review-requested direct regression tests for cost candidate enumeration, payment-window mana discovery, and trigger-source suppression. The code paths are gated, but these omissions should be pinned with dedicated tests.
- Brief's request for a second carrier card / next-untap card test and a CR conformance leaf was already represented in the existing phased-out implementation and targeted test run; no new tests were added here.
