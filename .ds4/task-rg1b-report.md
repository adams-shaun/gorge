# rg1b — NoRegen destruction regression

Base: `wt/rg1` at `7f300ba`. Corpus cache present; no scripts tracked.

## Changes

- `effects/zone.go`: both `effDestroy` and `effDestroyAll` call `ReplaceDestruction` only when `sa.Params["NoRegen"]` is empty. Indestructible remains checked first.
- `effects/regeneration.go`: skip `Tap` when already tapped; document the caller's NoRegen precondition.
- `rules/regeneration_test.go`: `TestRegenerationCannotReplaceNoRegen` resolves the actual compiled Terror and Wrath of God abilities and Nekrataal trigger Effect from `CorpusRegistry`. It asserts the real SA has NoRegen=True, the shielded green nonartifact creature dies, and no shield-consumption event occurs. No synthetic destruction SA is used. Extended the two-shield test to reject redundant Tap on the second replacement.
- `AGENTS.md`: corrected the ledger: destroy-side NoRegen is represented and honoured; continuous Mode$ CantRegenerate remains deferred under Effect, explicitly including Incinerate.

Confirmed NoRegen in the actual local Terror, Wrath of God and Nekrataal script data as well as compiled parameters. Absence of Go references does not imply absence of a modelled script parameter.

## Before gate (production code still at 7f300ba)

Command: `GOMEMLIMIT=5GiB go test -p=2 ./rules -run '^TestRegenerationCannotReplaceNoRegen$' -v`

```text
=== RUN   TestRegenerationCannotReplaceNoRegen
=== RUN   TestRegenerationCannotReplaceNoRegen/Terror
    regeneration_test.go:140: shielded creature survived; zone=battlefield
=== RUN   TestRegenerationCannotReplaceNoRegen/Wrath_of_God
    regeneration_test.go:140: shielded creature survived; zone=battlefield
=== RUN   TestRegenerationCannotReplaceNoRegen/Nekrataal
    regeneration_test.go:140: shielded creature survived; zone=battlefield
--- FAIL: TestRegenerationCannotReplaceNoRegen (0.24s)
    --- FAIL: TestRegenerationCannotReplaceNoRegen/Terror (0.00s)
    --- FAIL: TestRegenerationCannotReplaceNoRegen/Wrath_of_God (0.00s)
    --- FAIL: TestRegenerationCannotReplaceNoRegen/Nekrataal (0.00s)
FAIL
FAIL	github.com/adams-shaun/gorge/rules	0.249s
FAIL
```

## After gate

Command: `GOMEMLIMIT=5GiB go test -p=2 ./rules ./effects ./events`

```text
ok  	github.com/adams-shaun/gorge/rules	4.472s
ok  	github.com/adams-shaun/gorge/effects	0.006s
ok  	github.com/adams-shaun/gorge/events	(cached)
```

The full rules suite includes the previously failing three-card test and the redundant-Tap assertion.

Command: `GOMEMLIMIT=5GiB go test -p=2 ./rules -run 'TestHeads|TestRepoDeckGamesReplayExactly|TestEveryRepoDeckIsFullySupported' -v`

```text
=== RUN   TestEveryRepoDeckIsFullySupported
    acceptance_test.go:113: ratchet: 0 of 423 distinct cards across the repo decks are not fully supported
--- PASS: TestEveryRepoDeckIsFullySupported (0.25s)
=== RUN   TestRepoDeckGamesReplayExactly
    acceptance_test.go:316: seed 0: 1236 intents, chain 6913c09872d8879a, replay OK
    acceptance_test.go:316: seed 1: 798 intents, chain 2eaee7ed26b40264, replay OK
    acceptance_test.go:316: seed 2: 836 intents, chain 05fc584050816e7c, replay OK
    acceptance_test.go:316: seed 3: 978 intents, chain f72b7803e60aac3a, replay OK
    acceptance_test.go:316: seed 4: 785 intents, chain ebb36203ddb18233, replay OK
--- PASS: TestRepoDeckGamesReplayExactly (0.40s)
=== RUN   TestHeads
--- PASS: TestHeads (0.58s)
PASS
ok  	github.com/adams-shaun/gorge/rules	1.243s
```

Command: `GOMEMLIMIT=5GiB go vet -p=2 ./rules ./effects ./events`

Output: none; exit 0.

`gofmt -w effects/zone.go effects/regeneration.go rules/regeneration_test.go` and `git diff --check`: no output, exit 0.

## Heads, deviations, open concerns

All heads remain unmoved; `rules/heads_test.go` untouched. Replay is byte-identical for all five tested seeds and the measured ratchet is 0/423. Followed the task-specific requirement that heads PASS rather than the generic wave note predicting movement. No base attribution run needed because there is no movement. No push, merge, rebase or amend.

As explicitly requested, left the `BlockedBy` zero tombstone wire issue untouched: `events/apply.go` can retain a zero entry and `view/view.go` projects `[0]`. Controller is tracking this separately. Continuous CantRegenerate remains the existing Effect deferral, not an additional destruction-flag gap.
