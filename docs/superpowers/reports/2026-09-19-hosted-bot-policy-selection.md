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

Post-implementation review found that an archived sidecar written before
`bot_policy` would list an empty match policy after restart. The follow-up
normalizes an absent sidecar value from the already-normalized table policy
in memory, without rewriting the archival sidecar or changing replay input.
`TestPrePolicyArchivedMatchReportsBotAndReplays` hand-authors both the old
table and old sidecar shape, then verifies the listed effective name and a
full event replay.

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
- `go test ./host -run 'TestPrePolicyArchivedMatchReportsBotAndReplays|TestBotPolicyPersistsItsNormalizedDefaultAndRejectsUnknownRestore|TestAFinishedMatchIsServedFromDiskAfterRestart' -count=1`

The broader focused-package command
`go test ./botpolicy ./seat ./host ./host/httpapi ./cmd/gorged -count=1`
still has two known unrelated failures: host's committed overshoot feedback
capture diverges at event 836 under the current corpus, and gorged's stale
deck-directory expectation assumes `death-n-taxes` sorts first despite the
new `avengers-assemble` deck. The policy tests, `botpolicy`, `seat`, and
`host/httpapi` portions passed. The wider Task 4 command similarly reports
only that stale gorged deck-list assertion; botbench passed.

The full `go test ./host -count=1` follow-up has the same documented,
pre-existing committed-overshoot replay divergence at event 836; the new
pre-policy migration regression and all other host tests passed.

## Scope and risks

No policy-strength claim was made and no botbench matrix was run. The prior
AR7 held-out evidence remains the only strength evidence for
`lethal-pressure`; it remains opt-in and production `bot` goldens were not
regenerated.

`Options.Seats` remains an explicit embedder controller override for backward
compatibility. The default hosted path is policy-aware; an embedder that
installs its own `Seats` function is intentionally responsible for its own
controller semantics.

The independent review's remaining minor coverage suggestions are deferred:
run whole-game board/view adapter parity for `lethal-pressure`, and add a
host-level two-run replay equality test for each named policy. The focused
factory/adaptor and current golden/replay tests cover the shipped migration.

Those review follow-ups are now covered: `TestHostedPoliciesReplayDeterministically`
plays two hosted matches for omitted/default `bot`, explicit `bot`, and
`lethal-pressure`, requiring a terminal win/draw, comparing every event and
intent, outcome/winner, chain head, and an independent replay; it also
compares omitted/default `bot` directly with explicit `bot`.
`TestBotAdaptersAgreeOverWholeGame` and
`TestBotAdaptersAgreeOverCommanderGame` run both variants through real
view- and game-shaped adapters, including stack/counter, attachment, and
Commander facts; each paired game terminates with identical adapter intent
streams and chain head.

## Final full-suite check

`go test ./... -count=1` was red only in known broad-suite areas: the stale
botbench/deck-pool expectations, host's committed overshoot capture at event
836, `internal/archtest`, and `internal/searchprobe`. Its one additional
gorged human-play exact-sequence failure passed on an immediate isolated rerun
and also passed at pre-feature commit `4afad4b`, so it is not evidence of a
hosted-policy regression. The focused host migration test and `TestHeads`
were green at `4fa8e00`.

## Next experiments

These are experiments, not production-policy changes. Each candidate remains
opt-in, uses only the deciding seat's legal view, records its reason through
the existing decision trace, and must leave `bot` heads untouched unless a
separate promotion and golden-update decision is approved.

1. **AR8 combined-attacker lethal pressure.** Replace AR7's independent
   attacker test with a deterministic subset search: evaluate one attacking
   set against a defender's minimum legal blocking set. Rank only public
   battlefield facts and use stable object-id tie breaks.
2. **Mana and tap sequencing.** Decide whether to tap a source by the
   best currently visible cast/activation it newly enables, preserving the
   existing pool-only and own-zone information boundary. Do not infer
   opponent hands, library order, or unprojected mana abilities.
3. **Combat block assignment.** Extend the defender-side policy from
   individual chump checks to legal blocker assignments, valuing survival,
   trades, and immediately visible lethal prevention without predicting
   hidden tricks.
4. **Target and removal evaluation.** Broaden the current literal-damage
   targeting heuristic only for effects whose public outcome is modelled;
   unknown prevention, replacement, X, and dynamic-effect shapes remain
   uninterpreted rather than guessed.
5. **Mulligan policy.** Add an opt-in opening-hand evaluator based only on
   the bot's own hand, its chosen deck's public identity, and visible format
   rules. Keep London keep/bottom choices traceable and deterministic.
6. **Trace-family diagnostics.** Turn decision-trace families into a
   candidate-comparison report: quantify which policy reasons changed,
   whether changes are concentrated in combat/casting/targeting, and whether
   any family correlates with stalls or replay divergence.
7. **Adverse-seed and watchdog search.** Sweep deterministic seeds and the
   approved deck pairs specifically for high decision counts, repeated state
   shapes, errors, and watchdog exits; a candidate with a reproducible stall
   is diagnostic-only, never a normal hosted policy.
8. **Promotion gate and rollback drill.** Before any default change, require
   development (`seed 0`, 100 games/pair) and held-out (`seed 1,000,000`,
   400 games/pair) matrices across all ten approved unordered pairs, with
   pair-level results, starting-player splits, CIs, same-policy controls,
   replay/stall/error counts, and trace diagnostics. A pooled win rate alone
   is insufficient; promotion additionally requires explicit authorization
   to update the three production heads. Verify that reverting the selected
   table policy to `bot` restores the old deterministic behavior.
