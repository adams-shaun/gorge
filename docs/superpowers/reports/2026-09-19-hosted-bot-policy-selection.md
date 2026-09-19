# Hosted bot policy selection — verification report

## Delivered behavior

Hosted `TableConfig` now persists `bot_policy`. An omitted value normalizes to
`bot`; `lethal-pressure` is an explicit experimental selection; `legacy` and
all unknown names are rejected. The host builds every default bot seat and
every human-seat timeout caretaker through the same per-seat seeded policy
factory. `legacy` remains a botbench-only diagnostic adapter.

The effective policy is safe public metadata on table, match-start, match
listing, sidecar, and feedback-snapshot data. It is not an event, rules
configuration field, hash input, or source of hidden-information access.

Commits:

- `9a98601` — closed hosted policy factory and adapter parity
- `54d3e47` — durable table validation/default normalization
- `919d0ce` — controller/caretaker wiring and public metadata
- `7531287` — play-vs-bot API selection and shared botbench constructors

## Verification

Passed:

- `go test ./host ./seat -run 'TestNormalizeBotPolicy|TestNewBotPolicySeatIsDeterministic|TestBotAdaptersAgree' -count=1`
- `go test ./host ./protocol -run 'TestTableBotPolicy|TestConfigurationIsValidated|Test.*BotPolicy.*Persist|Test.*BotPolicy.*Restore|TestGoldens' -count=1`
- `go test ./host ./protocol ./seat -run 'Test(FeedbackMatchMarshalsTheSidecarsKeys|HumanCaretakerUsesConfiguredPolicy|DefaultSeatsConstructTheConfiguredPolicy|BotPolicyMetadataIsCarriedToMatch|Goldens|BotAdaptersAgree)' -count=1`
- `go test ./host/httpapi ./cmd/gorged ./cmd/botbench -run 'TestCreateGame.*BotPolicy|Test.*Legacy.*Policy' -count=1`
- `go test ./cmd/botbench -count=1 -skip 'TestFullPairsIteratesSorted|TestConstructedDefaultIsByteIdentical'`
- `go test ./rules -run TestHeads -count=1` — existing heads unchanged
- `make sim` — 20/20 four-seat games replayed successfully
- `go vet ./...`
- `go run ./cmd/gentypes -check`
- `git diff --check`

The broader focused-package command
`go test ./botpolicy ./seat ./host ./host/httpapi ./cmd/gorged -count=1`
still has two known unrelated failures: host's committed overshoot feedback
capture diverges at event 836 under the current corpus, and gorged's stale
deck-directory expectation assumes `death-n-taxes` sorts first despite the
new `avengers-assemble` deck. The policy tests, `botpolicy`, `seat`, and
`host/httpapi` portions passed. The wider Task 4 command similarly reports
only that stale gorged deck-list assertion; botbench passed.

## Scope and risks

No policy-strength claim was made and no botbench matrix was run. The prior
AR7 held-out evidence remains the only strength evidence for
`lethal-pressure`; it remains opt-in and production `bot` goldens were not
regenerated.

`Options.Seats` remains an explicit embedder controller override for backward
compatibility. The default hosted path is policy-aware; an embedder that
installs its own `Seats` function is intentionally responsible for its own
controller semantics.

## Next experiment

AR8 combined-attacker lethal pressure should remain a separate opt-in policy
experiment. Evaluate it with the approved ten-pair development and held-out
botbench matrices, pair-level intervals, starting-player splits, replay and
stall/error status, and trace-family diagnostics before requesting any
production promotion or golden update.
