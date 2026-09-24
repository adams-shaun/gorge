# Report — target-specific Equip reduction window

## Changes

- `rules/cast.go`: widened only `castWindowUnits` via `castWindowPaidUnits`, the shared cast-only probe used by target discount and Strive. It prices provable dynamic amounts using the effects count evaluator and supported paid activation costs (PayLife, covered literal generic, deterministic self-sacrifice); rejects unsupported or nondeterministic forms. The probe's candidates come from the mana window source walk and filters sources committed to Convoke/Conspire. Generic activation cost additionally requires unrestricted floating mana. `windowManaUnits` was not changed.
- `rules/equip_reduce_window_dynamic_test.go`: new inline-fixture/regression tests for the real-corpus Belt target with a power-count Joiner, paid-cost source shapes, completed exact payment, and probe subset of mana-window offers.
- `rules/cast_window_restricted_pool_test.go`: verifies restricted floating mana cannot certify generic activation payment.

`.cards` was present as a symlink to `/home/sadams/projects/gorge/.cards`; the real Belt corpus fixture ran. The menu filters KTarget options; no Clamp/botpolicy coupling is needed because the bot selects only offered options.

## Verification

Targeted command:

```text
go test -run 'TestBeltOfGiantStrength|TestCastWindowProvable|TestEquipReduceWindowFunded' ./rules/
ok   github.com/adams-shaun/gorge/rules  0.480s
```

Fail-without-fix proof (temporarily removed the `castWindowPaidUnits` call, restored `rules/cast.go`, and verified byte-identical with `cmp`):

```text
--- FAIL: TestEquipReduceWindowFundedTargetIsOffered
    equip_reduce_window_dynamic_test.go:149: 2-power target withheld although its repriced {8} is fundable from {6} plus the untapped Joiner's {G}{G}
--- FAIL: TestCastWindowProvableSubsetOfManaWindowOffers
    equip_reduce_window_dynamic_test.go:212: castWindowUnits priced no alternative for Joiner (4): the widened layer is absent
FAIL
```

Required gates:

```text
go test ./internal/archtest/
ok   github.com/adams-shaun/gorge/internal/archtest  1.437s

go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok   github.com/adams-shaun/gorge/cmd/botbench  0.676s

gofmt -l rules/cast.go rules/equip_reduce_window_dynamic_test.go rules/cast_window_restricted_pool_test.go
(no output)
go run ./cmd/gentypes -check
(no output)
```

No allowlist edits; no heads or ratchet edits. The three unchanged Belt regressions were included in the targeted command and passed.

## Issues

- `RestrictValid$` producers remain excluded fail-closed; the matcher grammar is not sufficient to safely certify arbitrary restricted production. Dynamic choice-shaped production, multi-colour generic-cost producers, generic-cost activations requiring other window taps first, and unsupported Amount$ bodies are likewise excluded. `tapXType`, SubCounter, Mill, UnlessCost$, Return<>, colored activation costs, multi-part sacrifice and loyalty producers remain out of scope. Follow-up ticket is recorded in `.ds4/new-tickets/castwindow-probe-remaining-shapes.md`.
- No Known-approximations row was closed or changed.
- No CR-lane test is proposed: this is a cast-window reachability defect already pinned by end-to-end rules tests.

## Existing workspace changes

At task start `rules/cast.go` and `.ds4/report-t1.md` were already modified and `rules/cast_window_restricted_pool_test.go` already untracked. The production implementation, new test, and follow-up were retained after review. `.ds4/report-t1.md` was not staged or included in this ticket commit.
