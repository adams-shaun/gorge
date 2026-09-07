# rg1 — regeneration replacement

Base: `587f31da5883029fa834ee0f6de62d2ac5b537e7`. Corpus present throughout.

## Changes

- `effects/regeneration.go`: shared `ReplaceDestruction` consumes exactly one existing Shield counter, clears marked damage and its Deathtouched marker, taps, and removes the permanent from combat. Every mutation is an emitted existing event.
- `rules/sba.go`: invoke that helper only for lethal-damage/deathtouch destruction, after the indestructible check. Zero toughness bypasses it. Preserve the existing bounded SBA attempt accounting.
- `effects/zone.go`: Destroy and DestroyAll use the same helper, after indestructible, before emitting a graveyard move. Sacrifice and exile paths are untouched. Intercepting destruction here rather than generic MoveZone is what prevents regeneration from replacing all deaths.
- `effects/counters.go`: keep the existing grant; remove the obsolete placeholder Note/comment.
- `rules/combat.go`: cleanup consumes all unused Shield counters in existing deterministic battlefield order.
- `events/apply.go`, `events/event.go`: extend existing EndCombatReset semantics, without adding kinds or event fields. Obj=0 retains the original global reset; nonzero removes only that permanent. A removed blocker becomes a zero tombstone in the attacker's BlockedBy list. Existing liveBlockers ignores nonexistent IDs, while len(BlockedBy)>0 preserves CR 509.1h (an attacker stays blocked). This also stops a regenerated blocker fighting in the second damage step. No forbidden files edited.
- `rules/regeneration_test.go`: inline fixtures cover attacker/blocker combat, event-only replay of that replacement, consumed shields, deathtouch-marker removal, Destroy and DestroyAll, sacrifice, exile, zero toughness, cleanup expiry, two shields, and the compiled corpus Experiment One's actual Regenerate ability. The corpus test resolves the real ability; it does not test activation-cost payment.
- `AGENTS.md`: rewrite rather than delete the approximation row: the replacement is implemented, but regeneration restrictions remain unmodelled.

## Deviations / concerns

The engine has no modelled "can't be regenerated" destroy flag or keyword gate (searched effects/rules/cards Go sources for NoRegen/CantRegen). Per brief, none was invented. That limitation is explicit in AGENTS.md.

The three negative guard cases already PASS on the unmodified base. It is impossible to truthfully say those tests fail before the fix: sacrifice, exile and zero toughness already kill shielded creatures. They deliberately pin existing correct behaviour against an overbroad replacement. All positive regression cases FAIL on base and PASS after. No artificially unrelated assertion was added to force the negative guards red.

Combat removal uses zero tombstones, analogous to the existing stale blocker IDs after death. The view currently projects BlockedBy verbatim (it already can contain IDs no longer on the battlefield); no view changes were made, per scope.

## Before/after proof

Archived base with:

```sh
mkdir -p /tmp/rg1/base
git archive 587f31da5883029fa834ee0f6de62d2ac5b537e7 | tar -x -C /tmp/rg1/base
ln -s "$(readlink -f .cards)" /tmp/rg1/base/.cards
git -C /tmp/rg1/base init -q
cp rules/regeneration_test.go /tmp/rg1/base/rules/regeneration_test.go
GOMEMLIMIT=5GiB go -C /tmp/rg1/base test -p=2 ./rules -run TestRegeneration -v
```

`git init` is necessary because CorpusRegistry resolves the git root; the first archived TestHeads attempt without it failed with `testutil: could not resolve git repo root: exit status 128`. This is an independent archive, not shared worktree git configuration. Initial test-authoring compilation errors (enterStep and **SA) were corrected before recording red/green evidence.

Final base output:

```text
=== RUN   TestRegenerationCombat
=== RUN   TestRegenerationCombat/attacker
    regeneration_test.go:51: replacement incomplete: zone=graveyard tapped=false damage=0 shields=0 attacking=false
=== RUN   TestRegenerationCombat/blocker
    regeneration_test.go:51: replacement incomplete: zone=graveyard tapped=false damage=0 shields=0 attacking=false
--- FAIL: TestRegenerationCombat (0.00s)
    --- FAIL: TestRegenerationCombat/attacker (0.00s)
    --- FAIL: TestRegenerationCombat/blocker (0.00s)
=== RUN   TestRegenerationConsumed
    regeneration_test.go:80: replacement incomplete: zone=graveyard tapped=false damage=0 shields=0 attacking=false
--- FAIL: TestRegenerationConsumed (0.00s)
=== RUN   TestRegenerationDeathtouch
    regeneration_test.go:92: replacement incomplete: zone=graveyard tapped=false damage=0 shields=0 attacking=false
--- FAIL: TestRegenerationDeathtouch (0.00s)
=== RUN   TestRegenerationDestroy
=== RUN   TestRegenerationDestroy/Destroy
    regeneration_test.go:109: replacement incomplete: zone=graveyard tapped=false damage=0 shields=0 attacking=false
=== RUN   TestRegenerationDestroy/DestroyAll
    regeneration_test.go:109: replacement incomplete: zone=graveyard tapped=false damage=0 shields=0 attacking=false
--- FAIL: TestRegenerationDestroy (0.00s)
    --- FAIL: TestRegenerationDestroy/Destroy (0.00s)
    --- FAIL: TestRegenerationDestroy/DestroyAll (0.00s)
=== RUN   TestRegenerationDoesNotReplaceOtherMoves
=== RUN   TestRegenerationDoesNotReplaceOtherMoves/Sacrifice
=== RUN   TestRegenerationDoesNotReplaceOtherMoves/ChangeZone
=== RUN   TestRegenerationDoesNotReplaceOtherMoves/zero_toughness
--- PASS: TestRegenerationDoesNotReplaceOtherMoves (0.00s)
    --- PASS: TestRegenerationDoesNotReplaceOtherMoves/Sacrifice (0.00s)
    --- PASS: TestRegenerationDoesNotReplaceOtherMoves/ChangeZone (0.00s)
    --- PASS: TestRegenerationDoesNotReplaceOtherMoves/zero_toughness (0.00s)
=== RUN   TestRegenerationExpires
    regeneration_test.go:143: unused shields survived cleanup
--- FAIL: TestRegenerationExpires (0.00s)
=== RUN   TestRegenerationTwoShields
    regeneration_test.go:151: first destruction should consume exactly one shield
--- FAIL: TestRegenerationTwoShields (0.00s)
=== RUN   TestRegenerationExperimentOne
    regeneration_test.go:182: replacement incomplete: zone=graveyard tapped=false damage=0 shields=0 attacking=false
--- FAIL: TestRegenerationExperimentOne (0.25s)
FAIL
FAIL	github.com/adams-shaun/gorge/rules	0.258s
FAIL
```

After: `GOMEMLIMIT=5GiB go test -p=2 ./rules -run TestRegeneration -v`

```text
=== RUN   TestRegenerationCombat
=== RUN   TestRegenerationCombat/attacker
=== RUN   TestRegenerationCombat/blocker
--- PASS: TestRegenerationCombat (0.00s)
    --- PASS: TestRegenerationCombat/attacker (0.00s)
    --- PASS: TestRegenerationCombat/blocker (0.00s)
=== RUN   TestRegenerationConsumed
--- PASS: TestRegenerationConsumed (0.00s)
=== RUN   TestRegenerationDeathtouch
--- PASS: TestRegenerationDeathtouch (0.00s)
=== RUN   TestRegenerationDestroy
=== RUN   TestRegenerationDestroy/Destroy
=== RUN   TestRegenerationDestroy/DestroyAll
--- PASS: TestRegenerationDestroy (0.00s)
    --- PASS: TestRegenerationDestroy/Destroy (0.00s)
    --- PASS: TestRegenerationDestroy/DestroyAll (0.00s)
=== RUN   TestRegenerationDoesNotReplaceOtherMoves
=== RUN   TestRegenerationDoesNotReplaceOtherMoves/Sacrifice
=== RUN   TestRegenerationDoesNotReplaceOtherMoves/ChangeZone
=== RUN   TestRegenerationDoesNotReplaceOtherMoves/zero_toughness
--- PASS: TestRegenerationDoesNotReplaceOtherMoves (0.00s)
    --- PASS: TestRegenerationDoesNotReplaceOtherMoves/Sacrifice (0.00s)
    --- PASS: TestRegenerationDoesNotReplaceOtherMoves/ChangeZone (0.00s)
    --- PASS: TestRegenerationDoesNotReplaceOtherMoves/zero_toughness (0.00s)
=== RUN   TestRegenerationExpires
--- PASS: TestRegenerationExpires (0.00s)
=== RUN   TestRegenerationTwoShields
--- PASS: TestRegenerationTwoShields (0.00s)
=== RUN   TestRegenerationExperimentOne
--- PASS: TestRegenerationExperimentOne (0.25s)
PASS
ok  	github.com/adams-shaun/gorge/rules	0.255s
```

Eight top-level tests pass after; base has seven failures and one passing negative-guard group. Twelve leaf cases pass after; nine fail and three pass before.

## Required gates (verbatim final output)

`GOMEMLIMIT=5GiB go test -p=2 ./rules ./effects`

```text
ok  	github.com/adams-shaun/gorge/rules	4.331s
ok  	github.com/adams-shaun/gorge/effects	(cached)
```

`GOMEMLIMIT=5GiB go test -p=2 ./rules -run 'TestHeads|TestRepoDeckGamesReplayExactly|TestEveryRepoDeckIsFullySupported' -v`

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
ok  	github.com/adams-shaun/gorge/rules	1.238s
```

`GOMEMLIMIT=5GiB go vet -p=2 ./rules ./effects`: exit 0, no output.

Additional affected package: `GOMEMLIMIT=5GiB go test -p=2 ./events`

```text
ok  	github.com/adams-shaun/gorge/events	0.007s
```

`git diff --check`: exit 0, no output. Changed Go files formatted with gofmt.

## Commit hook

Implementation commit: `60e111e`. The pre-commit hook passed without a budget override and automatically updated/staged `effects/TEST_HISTORY.md`, `events/TEST_HISTORY.md`, and `rules/TEST_HISTORY.md`:

```text
pre-commit: measuring test time for changed packages
testtime: effects 0.0s 119 tests budget 5s
testtime: events 0.0s 61 tests budget 5s
testtime: rules 1.6s 337 tests budget 10s
pre-commit: staging TEST_HISTORY.md:
  effects/TEST_HISTORY.md
  events/TEST_HISTORY.md
  rules/TEST_HISTORY.md
```

`.ds4` is ignored, so the first normal `git add` omitted this report; the report is force-added in a separate documentation commit. No overrides/trailers invented.

## Chain-head attribution

All FOUR are unchanged, a stronger result than expected. Measured reason: none of the four head games invokes Regenerate at all. A temporary test replaced that primitive with a callback that fails the test on invocation, then called the same acceptanceHead helper for 2/4/6/8. Both base and new trees passed. Probe removed from both trees afterward. Therefore the fixed Experiment One/shield destruction behaviour is exercised by the focused regression tests, not these particular head games. No head or ratchet goldens edited.

Base gate: `GOMEMLIMIT=5GiB go -C /tmp/rg1/base test -p=2 ./rules -run TestHeads -v`

```text
=== RUN   TestHeads
--- PASS: TestHeads (0.60s)
PASS
ok  	github.com/adams-shaun/gorge/rules	0.607s
```

Current probe: `GOMEMLIMIT=5GiB go test -p=2 ./rules -run TestRG1HeadPairs -v`

```text
=== RUN   TestRG1HeadPairs
    rg1_probe_test.go:16: 2 seats: chain head e6cfa5853c674b1b, golden e6cfa5853c674b1b
    rg1_probe_test.go:16: 4 seats: chain head 082262b548424173, golden 082262b548424173
    rg1_probe_test.go:16: 6 seats: chain head 761486a7d9f76753, golden 761486a7d9f76753
    rg1_probe_test.go:16: 8 seats: chain head 968be0bdc1f6a43b, golden 968be0bdc1f6a43b
--- PASS: TestRG1HeadPairs (0.60s)
PASS
ok  	github.com/adams-shaun/gorge/rules	0.608s
```

Base probe: `GOMEMLIMIT=5GiB go -C /tmp/rg1/base test -p=2 ./rules -run TestRG1HeadPairs -v`

```text
=== RUN   TestRG1HeadPairs
    rg1_probe_test.go:16: 2 seats: chain head e6cfa5853c674b1b, golden e6cfa5853c674b1b
    rg1_probe_test.go:16: 4 seats: chain head 082262b548424173, golden 082262b548424173
    rg1_probe_test.go:16: 6 seats: chain head 761486a7d9f76753, golden 761486a7d9f76753
    rg1_probe_test.go:16: 8 seats: chain head 968be0bdc1f6a43b, golden 968be0bdc1f6a43b
--- PASS: TestRG1HeadPairs (0.60s)
PASS
ok  	github.com/adams-shaun/gorge/rules	0.605s
```
